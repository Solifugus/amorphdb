package zone

import (
	"fmt"
	"strings"
	"sync"
	"time"

)

// SplitManager handles autonomous zone splitting when load thresholds are exceeded
type SplitManager struct {
	mu                sync.RWMutex
	nodeIdentity      string
	hashRing          *HashRing
	monitoredZones    map[string]*ZoneMonitoring // Zones we're monitoring for splitting
	splitThresholds   *SplitThresholds          // Thresholds that trigger splits
	onZoneSplit       func(*SplitResult) error  // Callback for announcing splits
	monitoringEnabled bool
	stopChannel       chan struct{}
}

// ZoneMonitoring tracks load metrics for a zone
type ZoneMonitoring struct {
	ZoneID              string                  // Zone identifier
	PathPrefix          string                  // Path prefix for the zone
	DataSize            int64                   // Total data size in bytes
	QueryRate           float64                 // Queries per second
	WriteRate           float64                 // Writes per second
	SubtreeCount        int                     // Number of immediate children
	InstanceAge         map[string]time.Time    // Age of instances by path
	LastSizeCheck       time.Time               // Last time data size was measured
	LastQueryMeasure    time.Time               // Last query rate measurement
	LoadHistory         []LoadSnapshot          // Historical load data
	SplitAttempts       int                     // Number of split attempts
	LastSplitAttempt    time.Time               // Last split attempt time
}

// SplitThresholds defines when zone splitting should occur
type SplitThresholds struct {
	MaxDataSize       int64   // Maximum data size before split (bytes)
	MaxQueryRate      float64 // Maximum queries per second
	MaxWriteRate      float64 // Maximum writes per second
	MaxSubtrees       int     // Maximum immediate children
	MinInstanceAge    time.Duration // Minimum age before temporal split
	CooldownPeriod    time.Duration // Time between split attempts
}

// LoadSnapshot represents load metrics at a point in time
type LoadSnapshot struct {
	Timestamp  time.Time // When the measurement was taken
	DataSize   int64     // Data size at this time
	QueryRate  float64   // Query rate at this time
	WriteRate  float64   // Write rate at this time
}

// SplitResult represents the result of a zone split
type SplitResult struct {
	OriginalZoneID  string       // Original zone that was split
	NewZones        []*NewZone   // New zones created from the split
	SplitType       string       // "spatial" or "temporal"
	SplitCriteria   string       // Description of why split occurred
	DataMigration   []*DataMove  // Data that needs to be moved
	AnnounceTime    time.Time    // When to announce to gossip
}

// NewZone represents a newly created zone from a split
type NewZone struct {
	ZoneID      string   // New zone identifier
	PathPrefix  string   // Path prefix for new zone
	Authority   string   // Authority node for new zone
	Replicas    []string // Replica nodes for new zone
	DataSize    int64    // Estimated data size
	PathPattern string   // Pattern for paths in this zone
}

// DataMove represents data that needs to migrate to a new zone
type DataMove struct {
	FromZone    string   // Source zone
	ToZone      string   // Destination zone
	PathPrefix  string   // Path prefix to move
	DataSize    int64    // Size of data to move
	Priority    int      // Migration priority (1=high, 10=low)
}

// NewSplitManager creates a new zone split manager
func NewSplitManager(nodeIdentity string, hashRing *HashRing) *SplitManager {
	return &SplitManager{
		nodeIdentity:      nodeIdentity,
		hashRing:          hashRing,
		monitoredZones:    make(map[string]*ZoneMonitoring),
		splitThresholds:   NewDefaultSplitThresholds(),
		monitoringEnabled: false,
		stopChannel:       make(chan struct{}),
	}
}

// NewDefaultSplitThresholds creates default splitting thresholds
func NewDefaultSplitThresholds() *SplitThresholds {
	return &SplitThresholds{
		MaxDataSize:    100 * 1024 * 1024, // 100 MB
		MaxQueryRate:   1000,               // 1000 queries per second
		MaxWriteRate:   500,                // 500 writes per second
		MaxSubtrees:    1000,               // 1000 immediate children
		MinInstanceAge: time.Hour * 24,     // 1 day for temporal split
		CooldownPeriod: time.Minute * 10,   // 10 minutes between attempts
	}
}

// StartMonitoring begins zone monitoring for split decisions
func (sm *SplitManager) StartMonitoring() {
	sm.mu.Lock()
	if sm.monitoringEnabled {
		sm.mu.Unlock()
		return
	}
	sm.monitoringEnabled = true
	sm.mu.Unlock()

	go sm.monitoringLoop()
}

// StopMonitoring stops zone monitoring
func (sm *SplitManager) StopMonitoring() {
	sm.mu.Lock()
	if !sm.monitoringEnabled {
		sm.mu.Unlock()
		return
	}
	sm.monitoringEnabled = false
	sm.mu.Unlock()

	close(sm.stopChannel)
}

// AddZoneForMonitoring adds a zone to monitor for splitting
func (sm *SplitManager) AddZoneForMonitoring(zoneID, pathPrefix string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	monitoring := &ZoneMonitoring{
		ZoneID:           zoneID,
		PathPrefix:       pathPrefix,
		DataSize:         0,
		QueryRate:        0,
		WriteRate:        0,
		SubtreeCount:     0,
		InstanceAge:      make(map[string]time.Time),
		LastSizeCheck:    time.Now(),
		LastQueryMeasure: time.Now(),
		LoadHistory:      make([]LoadSnapshot, 0),
		SplitAttempts:    0,
		LastSplitAttempt: time.Time{},
	}

	sm.monitoredZones[zoneID] = monitoring
}

// RemoveZoneFromMonitoring removes a zone from monitoring
func (sm *SplitManager) RemoveZoneFromMonitoring(zoneID string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	delete(sm.monitoredZones, zoneID)
}

// UpdateZoneMetrics updates load metrics for a zone
func (sm *SplitManager) UpdateZoneMetrics(zoneID string, dataSize int64, queryRate, writeRate float64, subtreeCount int) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	monitoring, exists := sm.monitoredZones[zoneID]
	if !exists {
		return
	}

	now := time.Now()

	// Update current metrics
	monitoring.DataSize = dataSize
	monitoring.QueryRate = queryRate
	monitoring.WriteRate = writeRate
	monitoring.SubtreeCount = subtreeCount
	monitoring.LastSizeCheck = now
	monitoring.LastQueryMeasure = now

	// Add to load history
	snapshot := LoadSnapshot{
		Timestamp: now,
		DataSize:  dataSize,
		QueryRate: queryRate,
		WriteRate: writeRate,
	}
	monitoring.LoadHistory = append(monitoring.LoadHistory, snapshot)

	// Keep only last 100 snapshots
	if len(monitoring.LoadHistory) > 100 {
		monitoring.LoadHistory = monitoring.LoadHistory[len(monitoring.LoadHistory)-100:]
	}
}

// UpdateInstanceAge updates the age information for instances in a zone
func (sm *SplitManager) UpdateInstanceAge(zoneID, path string, timestamp time.Time) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	monitoring, exists := sm.monitoredZones[zoneID]
	if !exists {
		return
	}

	monitoring.InstanceAge[path] = timestamp
}

// SetSplitCallback sets the callback for zone split announcements
func (sm *SplitManager) SetSplitCallback(callback func(*SplitResult) error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.onZoneSplit = callback
}

// monitoringLoop is the main monitoring and decision loop
func (sm *SplitManager) monitoringLoop() {
	ticker := time.NewTicker(time.Second * 30) // Check every 30 seconds
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			sm.evaluateZones()
		case <-sm.stopChannel:
			return
		}
	}
}

// evaluateZones evaluates all monitored zones for splitting
func (sm *SplitManager) evaluateZones() {
	sm.mu.RLock()
	zones := make([]*ZoneMonitoring, 0, len(sm.monitoredZones))
	for _, zone := range sm.monitoredZones {
		zones = append(zones, zone)
	}
	sm.mu.RUnlock()

	for _, zone := range zones {
		sm.evaluateZoneForSplit(zone)
	}
}

// evaluateZoneForSplit evaluates a single zone for splitting
func (sm *SplitManager) evaluateZoneForSplit(zone *ZoneMonitoring) {
	// Check cooldown period
	if time.Since(zone.LastSplitAttempt) < sm.splitThresholds.CooldownPeriod {
		return
	}

	// Determine if split is needed and what type
	splitNeeded, splitType, criteria := sm.shouldSplitZone(zone)
	if !splitNeeded {
		return
	}

	// Perform the split
	splitResult, err := sm.performZoneSplit(zone, splitType, criteria)
	if err != nil {
		// Mark split attempt and continue
		sm.mu.Lock()
		zone.SplitAttempts++
		zone.LastSplitAttempt = time.Now()
		sm.mu.Unlock()
		return
	}

	// Announce split via callback
	if sm.onZoneSplit != nil {
		go sm.onZoneSplit(splitResult)
	}
}

// shouldSplitZone determines if a zone should be split and how
func (sm *SplitManager) shouldSplitZone(zone *ZoneMonitoring) (bool, string, string) {
	thresholds := sm.splitThresholds

	// Check size threshold
	if zone.DataSize > thresholds.MaxDataSize {
		if zone.SubtreeCount >= 2 {
			return true, "spatial", fmt.Sprintf("data size %d bytes exceeds threshold %d", zone.DataSize, thresholds.MaxDataSize)
		} else {
			return true, "temporal", fmt.Sprintf("data size %d bytes exceeds threshold, single subtree", zone.DataSize)
		}
	}

	// Check query rate threshold
	if zone.QueryRate > thresholds.MaxQueryRate {
		if zone.SubtreeCount >= 2 {
			return true, "spatial", fmt.Sprintf("query rate %.2f/s exceeds threshold %.2f", zone.QueryRate, thresholds.MaxQueryRate)
		} else {
			return true, "temporal", fmt.Sprintf("query rate %.2f/s exceeds threshold, single subtree", zone.QueryRate)
		}
	}

	// Check write rate threshold
	if zone.WriteRate > thresholds.MaxWriteRate {
		if zone.SubtreeCount >= 2 {
			return true, "spatial", fmt.Sprintf("write rate %.2f/s exceeds threshold %.2f", zone.WriteRate, thresholds.MaxWriteRate)
		} else {
			return true, "temporal", fmt.Sprintf("write rate %.2f/s exceeds threshold, single subtree", zone.WriteRate)
		}
	}

	// Check subtree count threshold
	if zone.SubtreeCount > thresholds.MaxSubtrees {
		return true, "spatial", fmt.Sprintf("subtree count %d exceeds threshold %d", zone.SubtreeCount, thresholds.MaxSubtrees)
	}

	return false, "", ""
}

// performZoneSplit performs the actual zone split
func (sm *SplitManager) performZoneSplit(zone *ZoneMonitoring, splitType, criteria string) (*SplitResult, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	// Mark split attempt
	zone.SplitAttempts++
	zone.LastSplitAttempt = time.Now()

	var newZones []*NewZone
	var dataMoves []*DataMove
	var err error

	switch splitType {
	case "spatial":
		newZones, dataMoves, err = sm.performSpatialSplit(zone)
	case "temporal":
		newZones, dataMoves, err = sm.performTemporalSplit(zone)
	default:
		return nil, fmt.Errorf("unknown split type: %s", splitType)
	}

	if err != nil {
		return nil, err
	}

	splitResult := &SplitResult{
		OriginalZoneID: zone.ZoneID,
		NewZones:       newZones,
		SplitType:      splitType,
		SplitCriteria:  criteria,
		DataMigration:  dataMoves,
		AnnounceTime:   time.Now(),
	}

	return splitResult, nil
}

// performSpatialSplit performs spatial zone splitting by subtrees
func (sm *SplitManager) performSpatialSplit(zone *ZoneMonitoring) ([]*NewZone, []*DataMove, error) {
	// Simulate spatial split by dividing path space
	// In real implementation, would analyze actual data distribution

	pathParts := strings.Split(zone.PathPrefix, ".")
	basePrefix := strings.Join(pathParts, ".")

	// Create two new zones by splitting alphabetically
	newZones := []*NewZone{
		{
			ZoneID:      generateZoneID(basePrefix + ".a"),
			PathPrefix:  basePrefix + ".a",
			Authority:   sm.nodeIdentity, // Simplified - would use hash ring
			Replicas:    []string{},      // Would be assigned from hash ring
			DataSize:    zone.DataSize / 2,
			PathPattern: basePrefix + ".a*",
		},
		{
			ZoneID:      generateZoneID(basePrefix + ".m"),
			PathPrefix:  basePrefix + ".m",
			Authority:   sm.nodeIdentity,
			Replicas:    []string{},
			DataSize:    zone.DataSize / 2,
			PathPattern: basePrefix + ".m*",
		},
	}

	// Create data migration tasks
	dataMoves := []*DataMove{
		{
			FromZone:   zone.ZoneID,
			ToZone:     newZones[0].ZoneID,
			PathPrefix: basePrefix + ".a",
			DataSize:   zone.DataSize / 2,
			Priority:   5,
		},
		{
			FromZone:   zone.ZoneID,
			ToZone:     newZones[1].ZoneID,
			PathPrefix: basePrefix + ".m",
			DataSize:   zone.DataSize / 2,
			Priority:   5,
		},
	}

	return newZones, dataMoves, nil
}

// performTemporalSplit performs temporal zone splitting by instance age
func (sm *SplitManager) performTemporalSplit(zone *ZoneMonitoring) ([]*NewZone, []*DataMove, error) {
	// Split by age - recent vs historical data
	cutoffTime := time.Now().Add(-sm.splitThresholds.MinInstanceAge)

	// Count instances by age
	recentSize := int64(0)
	historicalSize := int64(0)

	for _, timestamp := range zone.InstanceAge {
		if timestamp.After(cutoffTime) {
			recentSize += 1024 // Estimate size
		} else {
			historicalSize += 1024
		}
	}

	// Create zones for recent and historical data
	newZones := []*NewZone{
		{
			ZoneID:      generateZoneID(zone.PathPrefix + ".recent"),
			PathPrefix:  zone.PathPrefix,
			Authority:   sm.nodeIdentity,
			Replicas:    []string{},
			DataSize:    recentSize,
			PathPattern: zone.PathPrefix + ".*@recent",
		},
		{
			ZoneID:      generateZoneID(zone.PathPrefix + ".historical"),
			PathPrefix:  zone.PathPrefix,
			Authority:   sm.nodeIdentity,
			Replicas:    []string{},
			DataSize:    historicalSize,
			PathPattern: zone.PathPrefix + ".*@historical",
		},
	}

	// Create data migration tasks
	dataMoves := []*DataMove{
		{
			FromZone:   zone.ZoneID,
			ToZone:     newZones[0].ZoneID,
			PathPrefix: zone.PathPrefix + "@recent",
			DataSize:   recentSize,
			Priority:   1, // High priority for recent data
		},
		{
			FromZone:   zone.ZoneID,
			ToZone:     newZones[1].ZoneID,
			PathPrefix: zone.PathPrefix + "@historical",
			DataSize:   historicalSize,
			Priority:   8, // Lower priority for historical data
		},
	}

	return newZones, dataMoves, nil
}

// generateZoneID generates a unique zone identifier
func generateZoneID(pathPrefix string) string {
	// Simple zone ID generation based on path and timestamp
	timestamp := time.Now().Unix()
	return fmt.Sprintf("zone_%x_%d", hash32(pathPrefix), timestamp)
}

// hash32 creates a simple 32-bit hash of a string
func hash32(s string) uint32 {
	hash := uint32(0)
	for _, c := range s {
		hash = hash*31 + uint32(c)
	}
	return hash
}

// GetMonitoredZones returns information about all monitored zones
func (sm *SplitManager) GetMonitoredZones() map[string]*ZoneMonitoring {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	result := make(map[string]*ZoneMonitoring)
	for zoneID, monitoring := range sm.monitoredZones {
		result[zoneID] = &ZoneMonitoring{
			ZoneID:           monitoring.ZoneID,
			PathPrefix:       monitoring.PathPrefix,
			DataSize:         monitoring.DataSize,
			QueryRate:        monitoring.QueryRate,
			WriteRate:        monitoring.WriteRate,
			SubtreeCount:     monitoring.SubtreeCount,
			InstanceAge:      nil, // Don't expose internal age tracking
			LastSizeCheck:    monitoring.LastSizeCheck,
			LastQueryMeasure: monitoring.LastQueryMeasure,
			LoadHistory:      append([]LoadSnapshot{}, monitoring.LoadHistory...), // Copy
			SplitAttempts:    monitoring.SplitAttempts,
			LastSplitAttempt: monitoring.LastSplitAttempt,
		}
	}

	return result
}

// GetSplitThresholds returns the current split thresholds
func (sm *SplitManager) GetSplitThresholds() *SplitThresholds {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	return &SplitThresholds{
		MaxDataSize:    sm.splitThresholds.MaxDataSize,
		MaxQueryRate:   sm.splitThresholds.MaxQueryRate,
		MaxWriteRate:   sm.splitThresholds.MaxWriteRate,
		MaxSubtrees:    sm.splitThresholds.MaxSubtrees,
		MinInstanceAge: sm.splitThresholds.MinInstanceAge,
		CooldownPeriod: sm.splitThresholds.CooldownPeriod,
	}
}

// SetSplitThresholds updates the split thresholds
func (sm *SplitManager) SetSplitThresholds(thresholds *SplitThresholds) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.splitThresholds = thresholds
}

// GetSplitStats returns statistics about zone splitting activity
func (sm *SplitManager) GetSplitStats() map[string]interface{} {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	stats := make(map[string]interface{})
	stats["monitored_zones"] = len(sm.monitoredZones)
	stats["monitoring_enabled"] = sm.monitoringEnabled

	totalAttempts := 0
	zonesNearThreshold := 0

	for _, zone := range sm.monitoredZones {
		totalAttempts += zone.SplitAttempts

		// Check if zone is approaching thresholds (80% of limit)
		if zone.DataSize > int64(float64(sm.splitThresholds.MaxDataSize)*0.8) ||
			zone.QueryRate > sm.splitThresholds.MaxQueryRate*0.8 ||
			zone.WriteRate > sm.splitThresholds.MaxWriteRate*0.8 {
			zonesNearThreshold++
		}
	}

	stats["total_split_attempts"] = totalAttempts
	stats["zones_near_threshold"] = zonesNearThreshold

	return stats
}

// PredictSplitTime estimates when a zone will need to be split
func (sm *SplitManager) PredictSplitTime(zoneID string) (time.Duration, string, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	monitoring, exists := sm.monitoredZones[zoneID]
	if !exists {
		return 0, "", fmt.Errorf("zone %s not monitored", zoneID)
	}

	// Analyze load history for trends
	if len(monitoring.LoadHistory) < 3 {
		return 0, "insufficient_data", nil
	}

	// Calculate growth rates
	recent := monitoring.LoadHistory[len(monitoring.LoadHistory)-1]
	older := monitoring.LoadHistory[len(monitoring.LoadHistory)-3]

	timeDiff := recent.Timestamp.Sub(older.Timestamp).Seconds()
	if timeDiff <= 0 {
		return 0, "no_time_difference", nil
	}

	// Calculate data growth rate (bytes per second)
	dataGrowthRate := float64(recent.DataSize-older.DataSize) / timeDiff

	// Predict when data size threshold will be reached
	if dataGrowthRate > 0 {
		remainingCapacity := float64(sm.splitThresholds.MaxDataSize - recent.DataSize)
		secondsToThreshold := remainingCapacity / dataGrowthRate
		return time.Duration(secondsToThreshold) * time.Second, "data_size", nil
	}

	// Similar analysis for query rate and write rate could be added

	return 0, "no_growth_detected", nil
}
