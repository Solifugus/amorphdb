package mesh

import (
	"fmt"
	"sync"
	"time"

	"github.com/solifugus/amorphdb/internal/directory"
	"github.com/solifugus/amorphdb/internal/identity"
	"github.com/solifugus/amorphdb/internal/protocol"
	"github.com/solifugus/amorphdb/internal/subscription"
)

// AuthorityManager manages write authority assignment and promotion for the subscription model
type AuthorityManager struct {
	mu                sync.RWMutex
	nodeIdentity     *identity.Identity
	directory        *directory.Directory
	subscriptions    *subscription.SubscriptionRegistry
	gossipManager    *GossipManager
	heartbeatManager *HeartbeatManager

	// Configuration
	missedHeartbeatThreshold int           // Number of missed heartbeats before promotion
	writeRateThreshold       int           // Writes/second threshold for splitting
	promotionTimeout         time.Duration // Timeout for promotion election

	// Authority tracking
	missedHeartbeats map[string]int    // Path authority -> missed heartbeat count
	writeRates       map[string]int    // Path -> current write rate
	lastWriteTime    map[string]time.Time // Path -> last write timestamp

	// Election state
	activeElections map[string]*AuthorityElection // Path -> ongoing election
}

// AuthorityElection represents an ongoing authority promotion election
type AuthorityElection struct {
	Path             string
	FormerAuthority  *identity.Identity
	Candidates       []*identity.Identity
	StartTime        time.Time
	Votes           map[string]*identity.Identity // Voter -> chosen candidate
	Completed        bool
	Winner          *identity.Identity
}

// AuthorityAnnouncement represents an authority change announcement
type AuthorityAnnouncement struct {
	Path      string             `json:"path"`
	Authority *identity.Identity `json:"authority"`
	Reason    string             `json:"reason"`    // "promotion", "delegation", "split", "initial"
	Timestamp time.Time          `json:"timestamp"`
}

// NewAuthorityManager creates a new authority manager
func NewAuthorityManager(
	nodeIdentity *identity.Identity,
	directory *directory.Directory,
	subscriptions *subscription.SubscriptionRegistry,
) *AuthorityManager {
	return &AuthorityManager{
		nodeIdentity:             nodeIdentity,
		directory:                directory,
		subscriptions:            subscriptions,
		missedHeartbeatThreshold: 3,
		writeRateThreshold:       100, // 100 writes/second threshold
		promotionTimeout:         time.Second * 5,
		missedHeartbeats:         make(map[string]int),
		writeRates:               make(map[string]int),
		lastWriteTime:            make(map[string]time.Time),
		activeElections:          make(map[string]*AuthorityElection),
	}
}

// SetGossipManager sets the gossip manager for authority announcements
func (am *AuthorityManager) SetGossipManager(gossipManager *GossipManager) {
	am.mu.Lock()
	defer am.mu.Unlock()
	am.gossipManager = gossipManager
}

// SetHeartbeatManager sets the heartbeat manager for failure detection
func (am *AuthorityManager) SetHeartbeatManager(heartbeatManager *HeartbeatManager) {
	am.mu.Lock()
	defer am.mu.Unlock()
	am.heartbeatManager = heartbeatManager

	// Register callback for node failures
	heartbeatManager.SetFailureCallback(func(nodeID string) {
		am.onNodeFailed(nodeID)
	})
}

// AnnounceAuthority announces this node as authority for a path
func (am *AuthorityManager) AnnounceAuthority(path, reason string) error {
	am.mu.Lock()
	defer am.mu.Unlock()

	timestamp := time.Now()

	// Update local directory
	am.directory.Set(path, am.nodeIdentity, timestamp)

	// Create announcement
	announcement := &AuthorityAnnouncement{
		Path:      path,
		Authority: am.nodeIdentity,
		Reason:    reason,
		Timestamp: timestamp,
	}

	// Propagate via gossip
	return am.propagateAnnouncement(announcement)
}

// DelegateAuthority voluntarily delegates authority for a sub-path to a subscriber
func (am *AuthorityManager) DelegateAuthority(path string, newAuthority *identity.Identity) error {
	am.mu.Lock()
	defer am.mu.Unlock()

	// Verify we currently hold authority for this path
	currentAuthority, _, ok := am.directory.Get(path)
	if !ok || currentAuthority.ID != am.nodeIdentity.ID {
		return fmt.Errorf("cannot delegate path '%s': not current authority", path)
	}

	// Verify the new authority is a subscriber
	subscribers := am.subscriptions.Subscribers(path)
	found := false
	for _, subscriber := range subscribers {
		if subscriber.ID == newAuthority.ID {
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("cannot delegate to node '%s': not a subscriber to path '%s'",
			newAuthority.ID, path)
	}

	// Update local directory
	timestamp := time.Now()
	am.directory.Set(path, newAuthority, timestamp)

	// Create announcement
	announcement := &AuthorityAnnouncement{
		Path:      path,
		Authority: newAuthority,
		Reason:    "delegation",
		Timestamp: timestamp,
	}

	// Propagate via gossip
	return am.propagateAnnouncement(announcement)
}

// SplitAuthority splits authority for a sub-path when overloaded
func (am *AuthorityManager) SplitAuthority(parentPath, subPath string, newAuthority *identity.Identity) error {
	am.mu.Lock()
	defer am.mu.Unlock()

	// Verify we hold authority for the parent path
	currentAuthority, _, ok := am.directory.Get(parentPath)
	if !ok || currentAuthority.ID != am.nodeIdentity.ID {
		return fmt.Errorf("cannot split from path '%s': not current authority", parentPath)
	}

	// Verify the new authority is a subscriber
	subscribers := am.subscriptions.Subscribers(parentPath)
	found := false
	for _, subscriber := range subscribers {
		if subscriber.ID == newAuthority.ID {
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("cannot split to node '%s': not a subscriber to path '%s'",
			newAuthority.ID, parentPath)
	}

	// Update local directory with new authority for sub-path
	timestamp := time.Now()
	am.directory.Set(subPath, newAuthority, timestamp)

	// Create announcement
	announcement := &AuthorityAnnouncement{
		Path:      subPath,
		Authority: newAuthority,
		Reason:    "split",
		Timestamp: timestamp,
	}

	// Propagate via gossip
	return am.propagateAnnouncement(announcement)
}

// OnHeartbeatMissed tracks missed heartbeats for authorities and triggers promotions
func (am *AuthorityManager) OnHeartbeatMissed(nodeID string) {
	am.mu.Lock()
	defer am.mu.Unlock()

	// Find all paths where this node is authority
	snapshot := am.directory.Snapshot()
	for _, entry := range snapshot {
		if entry.Authority.NodeID == nodeID {
			// Track missed heartbeat
			am.missedHeartbeats[entry.Path]++

			// Check if threshold reached
			if am.missedHeartbeats[entry.Path] >= am.missedHeartbeatThreshold {
				// Trigger promotion election
				go am.startPromotionElection(entry.Path, entry.Authority)
			}
		}
	}
}

// RecordWrite records a write operation for authority splitting decisions
func (am *AuthorityManager) RecordWrite(path string) {
	am.mu.Lock()
	defer am.mu.Unlock()

	now := time.Now()
	am.lastWriteTime[path] = now

	// Update write rate counter (simple sliding window)
	if lastWrite, exists := am.lastWriteTime[path]; exists {
		timeDiff := now.Sub(lastWrite)
		if timeDiff < time.Second {
			am.writeRates[path]++
		} else {
			am.writeRates[path] = 1
		}
	} else {
		am.writeRates[path] = 1
	}

	// Check if splitting is needed
	if am.writeRates[path] > am.writeRateThreshold {
		go am.considerSplitting(path)
	}
}

// ProcessAuthorityAnnouncement processes incoming authority announcements from gossip
func (am *AuthorityManager) ProcessAuthorityAnnouncement(announcement *AuthorityAnnouncement) {
	am.mu.Lock()
	defer am.mu.Unlock()

	// Update local directory with last-write-wins
	_, currentTime, exists := am.directory.Get(announcement.Path)
	if !exists || announcement.Timestamp.After(currentTime) {
		am.directory.Set(announcement.Path, announcement.Authority, announcement.Timestamp)

		// Reset missed heartbeat counter for this path
		delete(am.missedHeartbeats, announcement.Path)

		// Cancel any active election for this path
		if election, exists := am.activeElections[announcement.Path]; exists {
			election.Completed = true
			delete(am.activeElections, announcement.Path)
		}
	}
}

// onNodeFailed is called when heartbeat manager detects node failure
func (am *AuthorityManager) onNodeFailed(nodeID string) {
	am.mu.Lock()
	defer am.mu.Unlock()

	// Find all paths where this node is authority
	snapshot := am.directory.Snapshot()
	for _, entry := range snapshot {
		if entry.Authority.NodeID == nodeID {
			// Immediately trigger promotion election
			go am.startPromotionElection(entry.Path, entry.Authority)
		}
	}
}

// startPromotionElection begins an election to promote a new authority
func (am *AuthorityManager) startPromotionElection(path string, formerAuthority *identity.Identity) {
	am.mu.Lock()

	// Check if election is already in progress
	if election, exists := am.activeElections[path]; exists && !election.Completed {
		am.mu.Unlock()
		return
	}

	// Get all subscribers who can become authority
	candidates := am.subscriptions.Subscribers(path)
	if len(candidates) == 0 {
		am.mu.Unlock()
		return
	}

	// Create election
	election := &AuthorityElection{
		Path:            path,
		FormerAuthority: formerAuthority,
		Candidates:      candidates,
		StartTime:       time.Now(),
		Votes:          make(map[string]*identity.Identity),
		Completed:      false,
	}

	am.activeElections[path] = election
	am.mu.Unlock()

	// Conduct election (simplified algorithm: longest uptime wins)
	winner := am.electByUptime(candidates)

	am.mu.Lock()
	election.Winner = winner
	election.Completed = true
	am.mu.Unlock()

	// If we are the winner, announce ourselves as authority
	if winner.ID == am.nodeIdentity.ID {
		am.AnnounceAuthority(path, "promotion")
	}
}

// electByUptime selects candidate with longest uptime
func (am *AuthorityManager) electByUptime(candidates []*identity.Identity) *identity.Identity {
	if len(candidates) == 0 {
		return nil
	}

	// For simplicity, use creation time as proxy for uptime
	// In production, this would track actual node uptime
	var winner *identity.Identity

	for _, candidate := range candidates {
		if winner == nil {
			winner = candidate
			continue
		}

		// Primary criterion: earliest creation time (longest uptime)
		if candidate.Created.Before(winner.Created) {
			winner = candidate
		} else if candidate.Created.Equal(winner.Created) {
			// Deterministic tiebreaker: lexicographically smaller node identity
			if candidate.ID < winner.ID {
				winner = candidate
			}
		}
	}

	return winner
}

// considerSplitting evaluates whether to split authority for high-write paths
func (am *AuthorityManager) considerSplitting(path string) {
	am.mu.RLock()
	writeRate := am.writeRates[path]
	am.mu.RUnlock()

	if writeRate <= am.writeRateThreshold {
		return
	}

	// Find suitable candidate for splitting
	subscribers := am.subscriptions.Subscribers(path)
	if len(subscribers) == 0 {
		return
	}

	// Simple heuristic: pick first subscriber
	// In production, this would consider load, capability, etc.
	newAuthority := subscribers[0]

	// Create a sub-path for splitting (simplified approach)
	subPath := path + ".partition1"

	// Perform the split
	err := am.SplitAuthority(path, subPath, newAuthority)
	if err != nil {
		// TODO: Log error
		return
	}
}

// propagateAnnouncement sends authority announcement via gossip
func (am *AuthorityManager) propagateAnnouncement(announcement *AuthorityAnnouncement) error {
	if am.gossipManager == nil {
		return fmt.Errorf("gossip manager not set")
	}

	// Create a custom node update for authority announcements
	// We'll extend the protocol to support authority updates
	update := &protocol.NodeUpdate{
		Identity: am.nodeIdentity.ID,
		Status:   "authority_change",
		Address:  "", // Not relevant for authority updates
		ZoneInfo: am.encodeAuthorityAnnouncement(announcement),
	}

	// Add to gossip manager's pending updates
	// Note: This is a simplified approach. In production, we'd have dedicated
	// authority announcement message types in the protocol
	am.gossipManager.UpdateZoneInfo(am.nodeIdentity.ID, update.ZoneInfo)

	return nil
}

// encodeAuthorityAnnouncement encodes announcement for transport
func (am *AuthorityManager) encodeAuthorityAnnouncement(announcement *AuthorityAnnouncement) []byte {
	// Simple encoding for demonstration
	// In production, this would use proper serialization (JSON, protobuf, etc.)

	data := fmt.Sprintf("AUTH:%s:%s:%s:%d",
		announcement.Path,
		announcement.Authority.ID,
		announcement.Reason,
		announcement.Timestamp.Unix())

	return []byte(data)
}

// decodeAuthorityAnnouncement decodes announcement from transport
func (am *AuthorityManager) decodeAuthorityAnnouncement(data []byte) *AuthorityAnnouncement {
	// Simple decoding for demonstration
	// In production, this would use proper deserialization

	// Parse the encoded format: "AUTH:path:authorityID:reason:timestamp"
	str := string(data)
	if len(str) < 5 || str[:5] != "AUTH:" {
		return nil
	}

	// This is a simplified parser - production would be more robust
	// For now, just return nil to indicate this is a placeholder
	return nil
}

// GetAuthorityForPath returns the current authority for a path
func (am *AuthorityManager) GetAuthorityForPath(path string) (*identity.Identity, bool) {
	am.mu.RLock()
	defer am.mu.RUnlock()

	authority, _, ok := am.directory.Get(path)
	return authority, ok
}

// IsAuthorityFor checks if this node is authority for the given path
func (am *AuthorityManager) IsAuthorityFor(path string) bool {
	authority, ok := am.GetAuthorityForPath(path)
	return ok && authority.ID == am.nodeIdentity.ID
}

// GetAuthorityStats returns statistics about authority management
func (am *AuthorityManager) GetAuthorityStats() map[string]interface{} {
	am.mu.RLock()
	defer am.mu.RUnlock()

	stats := make(map[string]interface{})
	stats["active_elections"] = len(am.activeElections)
	stats["tracked_paths"] = len(am.writeRates)

	// Count paths where we are authority
	snapshot := am.directory.Snapshot()
	authorityCount := 0
	for _, entry := range snapshot {
		if entry.Authority.ID == am.nodeIdentity.ID {
			authorityCount++
		}
	}
	stats["authority_count"] = authorityCount

	return stats
}