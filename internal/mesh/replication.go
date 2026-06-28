package mesh

import (
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/solifugus/amorphdb/internal/config"
	"github.com/solifugus/amorphdb/internal/directory"
	"github.com/solifugus/amorphdb/internal/identity"
	"github.com/solifugus/amorphdb/internal/protocol"
	"github.com/solifugus/amorphdb/internal/storage"
	"github.com/solifugus/amorphdb/internal/subscription"
	"github.com/solifugus/amorphdb/internal/zone_deprecated" // TODO: Remove legacy zone references
)

// ReplicationManager handles data replication between nodes using subscription-based model
type ReplicationManager struct {
	mu              sync.RWMutex
	nodeIdentity    string
	config          *config.Config

	// Legacy zone-based components (TODO: Remove after cleanup)
	hashRing        *zone.HashRing
	authorityZones  map[string]*AuthorityZone  // Zones where this node is authority
	replicaZones    map[string]*ReplicaZone    // Zones where this node is replica
	pendingWrites   map[string][]*PendingWrite // Writes waiting for replication (zone-based)
	onZoneTransfer  func(string, []byte) error // Callback for receiving zone data

	// Subscription-based replication components
	pathDirectory   *directory.Directory               // Path-to-authority mapping
	subscriptionReg *subscription.SubscriptionRegistry // Subscription tracking
	nodeId          *identity.Identity                 // Full node identity
	pendingPathWrites map[string][]*PendingPathWrite   // Writes waiting for replication
	subscribedPaths map[string]*SubscribedPath         // Local subscribed data

	// Common components
	replicationRate time.Duration              // How often to sync with replicas/subscribers
	isRunning       bool
	stopChannel     chan struct{}
	storageTree     storage.Tree // Local storage
}

// AuthorityZone represents a zone where this node is the authority
type AuthorityZone struct {
	ZoneID      string    // Zone identifier
	PathPrefix  string    // Path prefix for this zone
	Replicas    []string  // List of replica node identities
	LastSync    time.Time // Last successful sync with all replicas
	WriteCount  int64     // Number of writes since last sync
	DataSize    int64     // Approximate data size in bytes
	SyncStatus  map[string]*ReplicaStatus // Per-replica sync status
}

// ReplicaZone represents a zone where this node is a replica
type ReplicaZone struct {
	ZoneID       string    // Zone identifier
	PathPrefix   string    // Path prefix for this zone
	Authority    string    // Authority node identity
	LastReceived time.Time // Last successful data receive from authority
	DataSize     int64     // Local replica data size
	IsCurrent    bool      // Whether replica is up-to-date
}

// ReplicaStatus tracks synchronization status with a specific replica
type ReplicaStatus struct {
	NodeIdentity    string    // Replica node identity
	LastAckTime     time.Time // Last acknowledgment received
	LastSyncAttempt time.Time // Last sync attempt
	ConsecutiveFailures int   // Failed sync attempts in a row
	IsHealthy       bool      // Whether replica is responding
	PendingWrites   int       // Number of unacknowledged writes
}

// PendingWrite represents a write operation waiting for replication (zone-based)
type PendingWrite struct {
	Path      []string      // Storage path
	Value     storage.Value // Value to write
	Author    uint64        // Agent who performed the write
	Timestamp int64         // Write timestamp
	ZoneID    string        // Target zone
	Attempts  int           // Replication attempts
}

// PendingPathWrite represents a write operation waiting for subscription-based replication
type PendingPathWrite struct {
	Path        []string          // Storage path
	Value       storage.Value     // Value to write
	Author      uint64            // Agent who performed the write
	Timestamp   int64             // Write timestamp
	Subscribers []*identity.Identity // Subscriber nodes to push to
	Attempts    int               // Replication attempts
}

// SubscribedPath represents a path this node subscribes to
type SubscribedPath struct {
	Path          []string          // Subscribed path
	Authority     *identity.Identity // Current authority for this path
	LastReceived  time.Time         // Last update received
	IsCurrent     bool              // Whether local copy is up-to-date
	DataSize      int64             // Local subscribed data size
}

// NewReplicationManager creates a new replication manager
func NewReplicationManager(
	nodeIdentity string,
	cfg *config.Config,
	hashRing *zone.HashRing,
	storageTree storage.Tree,
	pathDirectory *directory.Directory,
	subscriptionReg *subscription.SubscriptionRegistry,
	nodeId *identity.Identity,
) *ReplicationManager {
	rm := &ReplicationManager{
		nodeIdentity:      nodeIdentity,
		config:            cfg,
		storageTree:       storageTree,
		replicationRate:   time.Second,
		stopChannel:       make(chan struct{}),

		// Legacy zone-based components (TODO: Remove after cleanup)
		hashRing:          hashRing,
		authorityZones:    make(map[string]*AuthorityZone),
		replicaZones:      make(map[string]*ReplicaZone),
		pendingWrites:     make(map[string][]*PendingWrite),

		// Subscription-based components
		pathDirectory:     pathDirectory,
		subscriptionReg:   subscriptionReg,
		nodeId:            nodeId,
		pendingPathWrites: make(map[string][]*PendingPathWrite),
		subscribedPaths:   make(map[string]*SubscribedPath),
	}

	return rm
}


// Start begins the replication process
func (rm *ReplicationManager) Start() {
	rm.mu.Lock()
	if rm.isRunning {
		rm.mu.Unlock()
		return
	}
	rm.isRunning = true
	rm.mu.Unlock()

	go rm.replicationLoop()
}

// Stop stops the replication process
func (rm *ReplicationManager) Stop() {
	rm.mu.Lock()
	if !rm.isRunning {
		rm.mu.Unlock()
		return
	}
	rm.isRunning = false
	rm.mu.Unlock()

	close(rm.stopChannel)
}

// AddAuthorityZone adds a zone where this node is the authority
func (rm *ReplicationManager) AddAuthorityZone(zoneID, pathPrefix string, replicas []string) {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	syncStatus := make(map[string]*ReplicaStatus)
	for _, replica := range replicas {
		syncStatus[replica] = &ReplicaStatus{
			NodeIdentity:        replica,
			LastAckTime:         time.Time{},
			LastSyncAttempt:     time.Time{},
			ConsecutiveFailures: 0,
			IsHealthy:          true,
			PendingWrites:      0,
		}
	}

	zone := &AuthorityZone{
		ZoneID:     zoneID,
		PathPrefix: pathPrefix,
		Replicas:   replicas,
		LastSync:   time.Time{},
		WriteCount: 0,
		DataSize:   0,
		SyncStatus: syncStatus,
	}

	rm.authorityZones[zoneID] = zone
	rm.pendingWrites[zoneID] = make([]*PendingWrite, 0)
}

// AddReplicaZone adds a zone where this node is a replica
func (rm *ReplicationManager) AddReplicaZone(zoneID, pathPrefix, authority string) {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	zone := &ReplicaZone{
		ZoneID:       zoneID,
		PathPrefix:   pathPrefix,
		Authority:    authority,
		LastReceived: time.Time{},
		DataSize:     0,
		IsCurrent:    false,
	}

	rm.replicaZones[zoneID] = zone
}

// RemoveZone removes a zone from authority or replica management
func (rm *ReplicationManager) RemoveZone(zoneID string) {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	delete(rm.authorityZones, zoneID)
	delete(rm.replicaZones, zoneID)
	delete(rm.pendingWrites, zoneID)
}

// RecordWrite records a write operation for replication
func (rm *ReplicationManager) RecordWrite(path []string, value storage.Value, author uint64, zoneID string) error {
	// Use subscription-based replication (only model supported)
	return rm.RecordPathWrite(path, value, author)
}

// HandleReplicationRequest processes a replication request from authority
func (rm *ReplicationManager) HandleReplicationRequest(data []byte) error {
	// Decode replication data
	replicationData, err := rm.decodeReplicationData(data)
	if err != nil {
		return fmt.Errorf("failed to decode replication data: %w", err)
	}

	rm.mu.Lock()
	defer rm.mu.Unlock()

	// Check if we're a replica for this zone
	replicaZone, isReplica := rm.replicaZones[replicationData.ZoneID]
	if !isReplica {
		return fmt.Errorf("not a replica for zone %s", replicationData.ZoneID)
	}

	// Apply writes to local storage
	for _, write := range replicationData.Writes {
		err := rm.storageTree.Write(write.Path, write.Value, write.Author)
		if err != nil {
			return fmt.Errorf("failed to apply write to path %v: %w", write.Path, err)
		}
	}

	// Update replica status
	replicaZone.LastReceived = time.Now()
	replicaZone.DataSize = replicationData.DataSize
	replicaZone.IsCurrent = true

	return nil
}

// replicationLoop is the main replication processing loop
func (rm *ReplicationManager) replicationLoop() {
	ticker := time.NewTicker(rm.replicationRate)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			rm.processReplicationRound()
		case <-rm.stopChannel:
			return
		}
	}
}

// processReplicationRound handles one round of replication
func (rm *ReplicationManager) processReplicationRound() {
	// Use subscription-based replication (only model supported)
	rm.processSubscriptionReplication()
}

// processZoneReplication handles zone-based replication (legacy)
func (rm *ReplicationManager) processZoneReplication() {
	rm.mu.RLock()
	zones := make([]*AuthorityZone, 0, len(rm.authorityZones))
	for _, zone := range rm.authorityZones {
		zones = append(zones, zone)
	}
	rm.mu.RUnlock()

	// Process each authority zone
	for _, zone := range zones {
		rm.replicateZone(zone)
	}
}

// replicateZone replicates data for a specific authority zone
func (rm *ReplicationManager) replicateZone(zone *AuthorityZone) {
	rm.mu.Lock()
	pendingWrites := rm.pendingWrites[zone.ZoneID]
	if len(pendingWrites) == 0 {
		rm.mu.Unlock()
		return
	}

	// Prepare replication data
	replicationData := &ReplicationData{
		ZoneID:      zone.ZoneID,
		PathPrefix:  zone.PathPrefix,
		Writes:      pendingWrites,
		DataSize:    zone.DataSize,
		Timestamp:   time.Now().UnixNano(),
		SourceNode:  rm.nodeIdentity,
	}

	replicaList := make([]string, len(zone.Replicas))
	copy(replicaList, zone.Replicas)
	rm.mu.Unlock()

	// Send to each replica
	successCount := 0
	for _, replica := range replicaList {
		if rm.sendReplicationData(replica, replicationData) {
			successCount++
		}
	}

	// Update zone status based on success
	rm.mu.Lock()
	if successCount > 0 {
		// At least one replica succeeded - remove replicated writes
		zone.LastSync = time.Now()
		rm.pendingWrites[zone.ZoneID] = rm.pendingWrites[zone.ZoneID][len(pendingWrites):]

		// Update replica statuses
		for _, replica := range replicaList {
			status := zone.SyncStatus[replica]
			if status != nil {
				status.LastSyncAttempt = time.Now()
				// Success/failure tracking would be updated based on individual responses
			}
		}
	}
	rm.mu.Unlock()
}

// sendReplicationData sends replication data to a specific replica
func (rm *ReplicationManager) sendReplicationData(replicaIdentity string, data *ReplicationData) bool {
	// Get replica address from hash ring
	nodes := rm.hashRing.GetAllNodes()
	replicaNode, exists := nodes[replicaIdentity]
	if !exists || !replicaNode.IsOnline {
		return false
	}

	// Connect to replica
	conn, err := net.DialTimeout("tcp", replicaNode.Address, time.Second*3)
	if err != nil {
		rm.markReplicaUnhealthy(data.ZoneID, replicaIdentity)
		return false
	}
	defer conn.Close()

	// Encode and send replication data
	payload := rm.encodeReplicationData(data)

	replicationMsg := &protocol.Message{
		Version:  protocol.Version,
		Type:     protocol.ZONE_TRANSFER, // Reuse zone transfer type for replication
		Sequence: uint32(time.Now().Unix()),
		Payload:  payload,
	}

	msgData, err := protocol.EncodeMessage(replicationMsg)
	if err != nil {
		return false
	}

	conn.SetWriteDeadline(time.Now().Add(time.Second * 5))
	_, err = conn.Write(msgData)
	if err != nil {
		rm.markReplicaUnhealthy(data.ZoneID, replicaIdentity)
		return false
	}

	// Read acknowledgment
	conn.SetReadDeadline(time.Now().Add(time.Second * 5))
	response, err := rm.readReplicationResponse(conn)
	if err != nil {
		rm.markReplicaUnhealthy(data.ZoneID, replicaIdentity)
		return false
	}

	// Check acknowledgment
	if response.Type == protocol.ZONE_TRANSFER { // Acknowledgment uses same type
		rm.markReplicaHealthy(data.ZoneID, replicaIdentity)
		return true
	}

	return false
}

// markReplicaUnhealthy marks a replica as unhealthy
func (rm *ReplicationManager) markReplicaUnhealthy(zoneID, replicaIdentity string) {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	zone := rm.authorityZones[zoneID]
	if zone == nil {
		return
	}

	status := zone.SyncStatus[replicaIdentity]
	if status != nil {
		status.ConsecutiveFailures++
		if status.ConsecutiveFailures >= 3 {
			status.IsHealthy = false
		}
	}
}

// markReplicaHealthy marks a replica as healthy
func (rm *ReplicationManager) markReplicaHealthy(zoneID, replicaIdentity string) {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	zone := rm.authorityZones[zoneID]
	if zone == nil {
		return
	}

	status := zone.SyncStatus[replicaIdentity]
	if status != nil {
		status.ConsecutiveFailures = 0
		status.IsHealthy = true
		status.LastAckTime = time.Now()
	}
}

// readReplicationResponse reads a replication response message
func (rm *ReplicationManager) readReplicationResponse(conn net.Conn) (*protocol.Message, error) {
	// Read header (10 bytes)
	headerBytes := make([]byte, 10)
	n := 0
	for n < len(headerBytes) {
		read, err := conn.Read(headerBytes[n:])
		if err != nil {
			return nil, fmt.Errorf("read header: %w", err)
		}
		n += read
	}

	// Parse payload length
	payloadLength := uint32(headerBytes[6])<<24 | uint32(headerBytes[7])<<16 |
		uint32(headerBytes[8])<<8 | uint32(headerBytes[9])

	// Read payload
	payload := make([]byte, payloadLength)
	n = 0
	for n < len(payload) {
		read, err := conn.Read(payload[n:])
		if err != nil {
			return nil, fmt.Errorf("read payload: %w", err)
		}
		n += read
	}

	// Read checksum
	checksumBytes := make([]byte, 4)
	n = 0
	for n < len(checksumBytes) {
		read, err := conn.Read(checksumBytes[n:])
		if err != nil {
			return nil, fmt.Errorf("read checksum: %w", err)
		}
		n += read
	}

	// Reconstruct and decode message
	fullMessage := make([]byte, 0, 14+payloadLength)
	fullMessage = append(fullMessage, headerBytes...)
	fullMessage = append(fullMessage, payload...)
	fullMessage = append(fullMessage, checksumBytes...)

	return protocol.DecodeMessage(fullMessage)
}

// GetAuthorityZones returns zones where this node is authority
func (rm *ReplicationManager) GetAuthorityZones() map[string]*AuthorityZone {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	result := make(map[string]*AuthorityZone)
	for zoneID, zone := range rm.authorityZones {
		result[zoneID] = &AuthorityZone{
			ZoneID:     zone.ZoneID,
			PathPrefix: zone.PathPrefix,
			Replicas:   append([]string{}, zone.Replicas...),
			LastSync:   zone.LastSync,
			WriteCount: zone.WriteCount,
			DataSize:   zone.DataSize,
			SyncStatus: nil, // Don't expose internal sync status
		}
	}

	return result
}

// GetReplicaZones returns zones where this node is a replica
func (rm *ReplicationManager) GetReplicaZones() map[string]*ReplicaZone {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	result := make(map[string]*ReplicaZone)
	for zoneID, zone := range rm.replicaZones {
		result[zoneID] = &ReplicaZone{
			ZoneID:       zone.ZoneID,
			PathPrefix:   zone.PathPrefix,
			Authority:    zone.Authority,
			LastReceived: zone.LastReceived,
			DataSize:     zone.DataSize,
			IsCurrent:    zone.IsCurrent,
		}
	}

	return result
}

// SetZoneTransferCallback sets the callback for handling zone transfers
func (rm *ReplicationManager) SetZoneTransferCallback(callback func(string, []byte) error) {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	rm.onZoneTransfer = callback
}

// ReplicationData represents data to be replicated
type ReplicationData struct {
	ZoneID     string          // Zone identifier
	PathPrefix string          // Path prefix for zone
	Writes     []*PendingWrite // Write operations to replicate
	DataSize   int64           // Total zone data size
	Timestamp  int64           // Replication timestamp
	SourceNode string          // Source authority node
}

// encodeReplicationData encodes replication data into bytes
func (rm *ReplicationManager) encodeReplicationData(data *ReplicationData) []byte {
	// Simple encoding for replication data
	// Format: zoneID_len + zoneID + pathPrefix_len + pathPrefix + timestamp + dataSize + writeCount + [writes...]

	// Calculate size
	size := 1 + len(data.ZoneID) + 1 + len(data.PathPrefix) + 8 + 8 + 4 + 1 + len(data.SourceNode)
	for _, write := range data.Writes {
		size += 4 + // path length
			len(write.Path)*50 + // approximate path size
			8 + // author
			8 + // timestamp
			100 // approximate value size
	}

	payload := make([]byte, 0, size)

	// Encode zone ID
	payload = append(payload, byte(len(data.ZoneID)))
	payload = append(payload, []byte(data.ZoneID)...)

	// Encode path prefix
	payload = append(payload, byte(len(data.PathPrefix)))
	payload = append(payload, []byte(data.PathPrefix)...)

	// Encode timestamp and data size
	for i := 0; i < 8; i++ {
		payload = append(payload, byte(data.Timestamp>>(56-i*8)))
	}

	for i := 0; i < 8; i++ {
		payload = append(payload, byte(data.DataSize>>(56-i*8)))
	}

	// Encode write count
	writeCount := len(data.Writes)
	for i := 0; i < 4; i++ {
		payload = append(payload, byte(writeCount>>(24-i*8)))
	}

	// Encode source node
	payload = append(payload, byte(len(data.SourceNode)))
	payload = append(payload, []byte(data.SourceNode)...)

	// For now, just encode basic metadata
	// In a full implementation, would include serialized write operations

	return payload
}

// decodeReplicationData decodes replication data from bytes
func (rm *ReplicationManager) decodeReplicationData(payload []byte) (*ReplicationData, error) {
	if len(payload) < 20 {
		return nil, fmt.Errorf("payload too short")
	}

	offset := 0

	// Decode zone ID
	zoneIDLen := int(payload[offset])
	offset++
	if offset+zoneIDLen > len(payload) {
		return nil, fmt.Errorf("invalid zone ID length")
	}
	zoneID := string(payload[offset : offset+zoneIDLen])
	offset += zoneIDLen

	// Decode path prefix
	if offset >= len(payload) {
		return nil, fmt.Errorf("missing path prefix")
	}
	pathPrefixLen := int(payload[offset])
	offset++
	if offset+pathPrefixLen > len(payload) {
		return nil, fmt.Errorf("invalid path prefix length")
	}
	pathPrefix := string(payload[offset : offset+pathPrefixLen])
	offset += pathPrefixLen

	// Decode timestamp
	if offset+8 > len(payload) {
		return nil, fmt.Errorf("missing timestamp")
	}
	var timestamp int64
	for i := 0; i < 8; i++ {
		timestamp = (timestamp << 8) | int64(payload[offset])
		offset++
	}

	// Decode data size
	if offset+8 > len(payload) {
		return nil, fmt.Errorf("missing data size")
	}
	var dataSize int64
	for i := 0; i < 8; i++ {
		dataSize = (dataSize << 8) | int64(payload[offset])
		offset++
	}

	// For now, return basic structure
	// In full implementation, would decode write operations

	return &ReplicationData{
		ZoneID:     zoneID,
		PathPrefix: pathPrefix,
		Writes:     []*PendingWrite{}, // Would decode actual writes
		DataSize:   dataSize,
		Timestamp:  timestamp,
		SourceNode: "", // Would decode from payload
	}, nil
}

// GetReplicationStats returns statistics about replication status
func (rm *ReplicationManager) GetReplicationStats() map[string]interface{} {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	stats := make(map[string]interface{})

	// Subscription-based statistics (only model supported)
	stats["subscribed_paths"] = len(rm.subscribedPaths)

	totalPendingPathWrites := 0
	for _, writes := range rm.pendingPathWrites {
		totalPendingPathWrites += len(writes)
	}
	stats["pending_path_writes"] = totalPendingPathWrites

	currentSubscriptions := 0
	for _, path := range rm.subscribedPaths {
		if path.IsCurrent {
			currentSubscriptions++
		}
	}
	stats["current_subscriptions"] = currentSubscriptions

	return stats
}

// RecordPathWrite records a write operation for subscription-based replication
func (rm *ReplicationManager) RecordPathWrite(path []string, value storage.Value, author uint64) error {

	rm.mu.Lock()
	defer rm.mu.Unlock()

	pathStr := pathToString(path)

	// Check if this node is authority for the path
	authority, _, ok := rm.pathDirectory.Get(pathStr)
	if !ok || authority.ID != rm.nodeId.ID {
		return fmt.Errorf("node is not authority for path %s", pathStr)
	}

	// Get all subscribers for this path
	subscribers := rm.subscriptionReg.Subscribers(pathStr)
	if len(subscribers) == 0 {
		// No subscribers, write completes immediately
		return nil
	}

	// Create pending write
	pendingWrite := &PendingPathWrite{
		Path:        path,
		Value:       value,
		Author:      author,
		Timestamp:   time.Now().UnixNano(),
		Subscribers: subscribers,
		Attempts:    0,
	}

	// Add to pending writes
	rm.pendingPathWrites[pathStr] = append(rm.pendingPathWrites[pathStr], pendingWrite)

	return nil
}

// HandleSubscriptionUpdate processes a subscription update from authority
func (rm *ReplicationManager) HandleSubscriptionUpdate(data []byte) error {

	// Decode subscription update data
	updateData, err := rm.decodeSubscriptionUpdate(data)
	if err != nil {
		return fmt.Errorf("failed to decode subscription update: %w", err)
	}

	rm.mu.Lock()
	defer rm.mu.Unlock()

	// Check if we're subscribed to this path
	pathStr := pathToString(updateData.Path)
	subscribedPath, isSubscribed := rm.subscribedPaths[pathStr]
	if !isSubscribed {
		return fmt.Errorf("not subscribed to path %s", pathStr)
	}

	// Apply write to local storage
	err = rm.storageTree.Write(updateData.Path, updateData.Value, updateData.Author)
	if err != nil {
		return fmt.Errorf("failed to apply write to path %v: %w", updateData.Path, err)
	}

	// Update subscription status
	subscribedPath.LastReceived = time.Now()
	subscribedPath.IsCurrent = true

	return nil
}

// ForwardWriteToAuthority forwards a write to the authority node for a path
func (rm *ReplicationManager) ForwardWriteToAuthority(path []string, value storage.Value, author uint64) error {

	pathStr := pathToString(path)

	// Look up authority for this path
	authority, _, ok := rm.pathDirectory.Get(pathStr)
	if !ok {
		return fmt.Errorf("no authority found for path %s", pathStr)
	}

	// If we are the authority, handle locally
	if authority.ID == rm.nodeId.ID {
		return rm.RecordPathWrite(path, value, author)
	}

	// Forward to authority node
	return rm.sendWriteToAuthority(authority, path, value, author)
}

// RequestSubscription requests subscription to a path from its authority
func (rm *ReplicationManager) RequestSubscription(pathStr string) error {

	// Look up authority for this path
	authority, _, ok := rm.pathDirectory.Get(pathStr)
	if !ok {
		return fmt.Errorf("no authority found for path %s", pathStr)
	}

	// If we are the authority, no subscription needed
	if authority.ID == rm.nodeId.ID {
		return nil
	}

	rm.mu.Lock()
	defer rm.mu.Unlock()

	// Check if already subscribed
	if _, exists := rm.subscribedPaths[pathStr]; exists {
		return nil // Already subscribed
	}

	// Create subscription record
	subscribedPath := &SubscribedPath{
		Path:         stringToPath(pathStr),
		Authority:    authority,
		LastReceived: time.Time{},
		IsCurrent:    false,
		DataSize:     0,
	}

	rm.subscribedPaths[pathStr] = subscribedPath

	// Register subscription with local registry
	rm.subscriptionReg.Subscribe(pathStr, authority)

	// Send subscription request to authority
	return rm.sendSubscriptionRequest(authority, pathStr)
}

// ProcessSubscriptionReplication handles one round of subscription-based replication
func (rm *ReplicationManager) processSubscriptionReplication() {

	rm.mu.RLock()
	pathWrites := make(map[string][]*PendingPathWrite)
	for pathStr, writes := range rm.pendingPathWrites {
		if len(writes) > 0 {
			pathWrites[pathStr] = append([]*PendingPathWrite{}, writes...)
		}
	}
	rm.mu.RUnlock()

	// Process each path with pending writes
	for pathStr, writes := range pathWrites {
		rm.replicatePathWrites(pathStr, writes)
	}
}

// replicatePathWrites replicates writes for a specific path to its subscribers
func (rm *ReplicationManager) replicatePathWrites(pathStr string, writes []*PendingPathWrite) {
	if len(writes) == 0 {
		return
	}

	// Prepare subscription update data
	updateData := &SubscriptionUpdateData{
		Path:      writes[0].Path, // All writes are for the same path
		Writes:    writes,
		Timestamp: time.Now().UnixNano(),
		Authority: rm.nodeId,
	}

	// Send to each subscriber
	successCount := 0
	for _, write := range writes {
		for _, subscriber := range write.Subscribers {
			if rm.sendSubscriptionUpdate(subscriber, updateData) {
				successCount++
			}
		}
	}

	// Update status based on success
	rm.mu.Lock()
	defer rm.mu.Unlock()

	if successCount > 0 {
		// At least one subscriber succeeded - remove replicated writes
		rm.pendingPathWrites[pathStr] = rm.pendingPathWrites[pathStr][len(writes):]
	}
}

// sendWriteToAuthority sends a write request to the authority node
func (rm *ReplicationManager) sendWriteToAuthority(authority *identity.Identity, path []string, value storage.Value, author uint64) error {
	// Create write message
	writeMsg := &protocol.WriteMessage{
		Path:   path,
		Value:  value,
		Author: author,
	}

	// Send to authority node (implementation would use actual network protocol)
	// For now, this is a placeholder
	_ = writeMsg
	_ = authority

	// TODO: Implement actual network communication to authority node
	return nil
}

// sendSubscriptionRequest sends a subscription request to an authority node
func (rm *ReplicationManager) sendSubscriptionRequest(authority *identity.Identity, pathStr string) error {
	// Create subscription request message
	// TODO: Add subscription request message type to protocol if not exists

	// For now, this is a placeholder
	_ = authority
	_ = pathStr

	// TODO: Implement actual network communication for subscription requests
	return nil
}

// sendSubscriptionUpdate sends subscription update to a subscriber node
func (rm *ReplicationManager) sendSubscriptionUpdate(subscriber *identity.Identity, data *SubscriptionUpdateData) bool {
	// Encode and send subscription update
	payload := rm.encodeSubscriptionUpdate(data)

	// Create protocol message for subscription update
	// For now, reuse existing message types
	updateMsg := &protocol.Message{
		Version:  protocol.Version,
		Type:     protocol.HEARTBEAT, // Would use dedicated subscription update type in production
		Sequence: uint32(time.Now().Unix()),
		Payload:  payload,
	}

	// TODO: Implement actual network communication
	_ = updateMsg
	_ = subscriber

	// Placeholder - in reality would send over network and return actual result
	return true
}

// SubscriptionUpdateData represents data for subscription-based replication
type SubscriptionUpdateData struct {
	Path      []string             // Path being updated
	Writes    []*PendingPathWrite  // Write operations
	Value     storage.Value        // Value being written (simplified single value)
	Author    uint64               // Author of the write
	Timestamp int64                // Update timestamp
	Authority *identity.Identity   // Authority node identity
}

// encodeSubscriptionUpdate encodes subscription update data
func (rm *ReplicationManager) encodeSubscriptionUpdate(data *SubscriptionUpdateData) []byte {
	// Simple encoding for subscription update
	// In production, would use proper serialization

	pathStr := pathToString(data.Path)
	return []byte(fmt.Sprintf("SUB_UPDATE:%s:%d:%s", pathStr, data.Timestamp, data.Authority.ID))
}

// decodeSubscriptionUpdate decodes subscription update data
func (rm *ReplicationManager) decodeSubscriptionUpdate(payload []byte) (*SubscriptionUpdateData, error) {
	// Simple decoding for subscription update
	// In production, would use proper deserialization

	// For now, return minimal structure
	return &SubscriptionUpdateData{
		Path:      []string{"placeholder"},
		Writes:    []*PendingPathWrite{},
		Timestamp: time.Now().UnixNano(),
		Authority: &identity.Identity{ID: "placeholder"},
	}, nil
}

// pathToString converts a path slice to a string representation
func pathToString(path []string) string {
	if len(path) == 0 {
		return ""
	}
	result := ""
	for i, segment := range path {
		if i > 0 {
			result += "."
		}
		result += segment
	}
	return result
}

// stringToPath converts a string representation back to a path slice
func stringToPath(pathStr string) []string {
	if pathStr == "" {
		return []string{}
	}
	return []string{pathStr} // Simplified - would split on "." in production
}
