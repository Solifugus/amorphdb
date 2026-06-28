package mesh

import (
	"testing"
	"time"

	"github.com/solifugus/amorphdb/internal/directory"
	"github.com/solifugus/amorphdb/internal/identity"
	"github.com/solifugus/amorphdb/internal/subscription"
	"github.com/solifugus/amorphdb/internal/types"
)

// createTestIdentity creates a test identity for testing
func createTestIdentity(id, meshName, nodeID string) *identity.Identity {
	return &identity.Identity{
		ID:       id,
		Type:     types.NodeAgent,
		MeshName: meshName,
		Created:  time.Now(),
		NodeID:   nodeID,
	}
}

func TestAuthorityManager_AnnounceAuthority(t *testing.T) {
	// Create test components
	nodeIdentity := createTestIdentity("test-node", "test-mesh", "test-node-1")
	dir := directory.NewDirectory()
	subscriptions := subscription.NewSubscriptionRegistry()

	// Create authority manager
	am := NewAuthorityManager(nodeIdentity, dir, subscriptions)

	// Set up gossip manager for testing
	gossipMgr := NewGossipManager(nodeIdentity.ID)
	am.SetGossipManager(gossipMgr)

	// Test authority announcement
	err := am.AnnounceAuthority("world.market", "initial")
	if err != nil {
		t.Errorf("AnnounceAuthority failed: %v", err)
	}

	// Verify authority was set in directory
	authority, ok := am.GetAuthorityForPath("world.market")
	if !ok {
		t.Fatal("Expected to find authority for world.market")
	}
	if authority.ID != nodeIdentity.ID {
		t.Errorf("Expected authority %s, got %s", nodeIdentity.ID, authority.ID)
	}

	// Verify we recognize ourselves as authority
	if !am.IsAuthorityFor("world.market") {
		t.Error("Expected to be authority for world.market")
	}
}

func TestAuthorityManager_DelegateAuthority(t *testing.T) {
	// Create test components
	nodeIdentity := createTestIdentity("test-node", "test-mesh", "test-node-1")
	subscriber := createTestIdentity("subscriber-node", "test-mesh", "subscriber-node-1")
	dir := directory.NewDirectory()
	subscriptions := subscription.NewSubscriptionRegistry()

	// Create authority manager
	am := NewAuthorityManager(nodeIdentity, dir, subscriptions)

	// Set up gossip manager for testing
	gossipMgr := NewGossipManager(nodeIdentity.ID)
	am.SetGossipManager(gossipMgr)

	// Set up initial authority
	am.AnnounceAuthority("world.market", "initial")

	// Add subscriber
	subscriptions.AddSubscriber("world.market", subscriber)

	// Test delegation
	err := am.DelegateAuthority("world.market", subscriber)
	if err != nil {
		t.Errorf("DelegateAuthority failed: %v", err)
	}

	// Verify authority was transferred
	authority, ok := am.GetAuthorityForPath("world.market")
	if !ok {
		t.Fatal("Expected to find authority for world.market")
	}
	if authority.ID != subscriber.ID {
		t.Errorf("Expected authority %s, got %s", subscriber.ID, authority.ID)
	}

	// Verify we no longer consider ourselves authority
	if am.IsAuthorityFor("world.market") {
		t.Error("Expected not to be authority for world.market after delegation")
	}
}

func TestAuthorityManager_DelegateAuthority_NotSubscriber(t *testing.T) {
	// Create test components
	nodeIdentity := createTestIdentity("test-node", "test-mesh", "test-node-1")
	nonSubscriber := createTestIdentity("non-subscriber", "test-mesh", "non-subscriber-1")
	dir := directory.NewDirectory()
	subscriptions := subscription.NewSubscriptionRegistry()

	// Create authority manager
	am := NewAuthorityManager(nodeIdentity, dir, subscriptions)

	// Set up initial authority
	am.AnnounceAuthority("world.market", "initial")

	// Test delegation to non-subscriber (should fail)
	err := am.DelegateAuthority("world.market", nonSubscriber)
	if err == nil {
		t.Error("Expected DelegateAuthority to fail for non-subscriber")
	}

	// Verify authority was not changed
	authority, ok := am.GetAuthorityForPath("world.market")
	if !ok {
		t.Fatal("Expected to find authority for world.market")
	}
	if authority.ID != nodeIdentity.ID {
		t.Errorf("Expected authority to remain %s, got %s", nodeIdentity.ID, authority.ID)
	}
}

func TestAuthorityManager_SplitAuthority(t *testing.T) {
	// Create test components
	nodeIdentity := createTestIdentity("test-node", "test-mesh", "test-node-1")
	subscriber := createTestIdentity("subscriber-node", "test-mesh", "subscriber-node-1")
	dir := directory.NewDirectory()
	subscriptions := subscription.NewSubscriptionRegistry()

	// Create authority manager
	am := NewAuthorityManager(nodeIdentity, dir, subscriptions)

	// Set up gossip manager for testing
	gossipMgr := NewGossipManager(nodeIdentity.ID)
	am.SetGossipManager(gossipMgr)

	// Set up initial authority for parent path
	am.AnnounceAuthority("world.market", "initial")

	// Add subscriber
	subscriptions.AddSubscriber("world.market", subscriber)

	// Test authority splitting
	err := am.SplitAuthority("world.market", "world.market.equities", subscriber)
	if err != nil {
		t.Errorf("SplitAuthority failed: %v", err)
	}

	// Verify parent path authority unchanged
	parentAuth, ok := am.GetAuthorityForPath("world.market")
	if !ok {
		t.Fatal("Expected to find authority for world.market")
	}
	if parentAuth.ID != nodeIdentity.ID {
		t.Errorf("Expected parent authority %s, got %s", nodeIdentity.ID, parentAuth.ID)
	}

	// Verify sub-path has new authority
	subAuth, ok := am.GetAuthorityForPath("world.market.equities")
	if !ok {
		t.Fatal("Expected to find authority for world.market.equities")
	}
	if subAuth.ID != subscriber.ID {
		t.Errorf("Expected sub-path authority %s, got %s", subscriber.ID, subAuth.ID)
	}
}

func TestAuthorityManager_RecordWrite_SplittingTrigger(t *testing.T) {
	// Create test components
	nodeIdentity := createTestIdentity("test-node", "test-mesh", "test-node-1")
	subscriber := createTestIdentity("subscriber-node", "test-mesh", "subscriber-node-1")
	dir := directory.NewDirectory()
	subscriptions := subscription.NewSubscriptionRegistry()

	// Create authority manager with low threshold for testing
	am := NewAuthorityManager(nodeIdentity, dir, subscriptions)
	am.writeRateThreshold = 3 // Low threshold for testing

	// Set up initial authority
	am.AnnounceAuthority("world.market", "initial")

	// Add subscriber
	subscriptions.AddSubscriber("world.market", subscriber)

	// Record writes to trigger splitting threshold
	for i := 0; i < 5; i++ {
		am.RecordWrite("world.market")
	}

	// Give time for async splitting consideration
	time.Sleep(time.Millisecond * 100)

	// Verify write rate was recorded
	am.mu.RLock()
	writeRate := am.writeRates["world.market"]
	am.mu.RUnlock()

	if writeRate == 0 {
		t.Error("Expected write rate to be recorded")
	}
}

func TestAuthorityManager_OnHeartbeatMissed(t *testing.T) {
	// Create test components
	nodeIdentity := createTestIdentity("test-node", "test-mesh", "test-node-1")
	// subscriber := createTestIdentity("subscriber-node", "test-mesh", "subscriber-node-1")
	authorityNode := createTestIdentity("authority-node", "test-mesh", "authority-node-1")
	dir := directory.NewDirectory()
	subscriptions := subscription.NewSubscriptionRegistry()

	// Create authority manager
	am := NewAuthorityManager(nodeIdentity, dir, subscriptions)

	// Set up authority for another node
	dir.Set("world.market", authorityNode, time.Now())

	// Add our node as a subscriber (so we can participate in elections)
	subscriptions.AddSubscriber("world.market", nodeIdentity)

	// Simulate missed heartbeats
	for i := 0; i < am.missedHeartbeatThreshold; i++ {
		am.OnHeartbeatMissed("authority-node-1")
	}

	// Give time for election processing
	time.Sleep(time.Millisecond * 100)

	// Verify missed heartbeats were tracked
	am.mu.RLock()
	missed := am.missedHeartbeats["world.market"]
	am.mu.RUnlock()

	if missed < am.missedHeartbeatThreshold {
		t.Errorf("Expected %d missed heartbeats, got %d", am.missedHeartbeatThreshold, missed)
	}
}

func TestAuthorityManager_ProcessAuthorityAnnouncement(t *testing.T) {
	// Create test components
	nodeIdentity := createTestIdentity("test-node", "test-mesh", "test-node-1")
	newAuthority := createTestIdentity("new-authority", "test-mesh", "new-authority-1")
	dir := directory.NewDirectory()
	subscriptions := subscription.NewSubscriptionRegistry()

	// Create authority manager
	am := NewAuthorityManager(nodeIdentity, dir, subscriptions)

	// Set up initial authority
	am.AnnounceAuthority("world.market", "initial")

	// Create announcement from another node
	announcement := &AuthorityAnnouncement{
		Path:      "world.market",
		Authority: newAuthority,
		Reason:    "promotion",
		Timestamp: time.Now().Add(time.Second), // Later timestamp
	}

	// Process the announcement
	am.ProcessAuthorityAnnouncement(announcement)

	// Verify authority was updated
	authority, ok := am.GetAuthorityForPath("world.market")
	if !ok {
		t.Fatal("Expected to find authority for world.market")
	}
	if authority.ID != newAuthority.ID {
		t.Errorf("Expected authority %s, got %s", newAuthority.ID, authority.ID)
	}
}

func TestAuthorityManager_ProcessAuthorityAnnouncement_OlderTimestamp(t *testing.T) {
	// Create test components
	nodeIdentity := createTestIdentity("test-node", "test-mesh", "test-node-1")
	newAuthority := createTestIdentity("new-authority", "test-mesh", "new-authority-1")
	dir := directory.NewDirectory()
	subscriptions := subscription.NewSubscriptionRegistry()

	// Create authority manager
	am := NewAuthorityManager(nodeIdentity, dir, subscriptions)

	// Set up initial authority
	am.AnnounceAuthority("world.market", "initial")

	// Create announcement with older timestamp
	announcement := &AuthorityAnnouncement{
		Path:      "world.market",
		Authority: newAuthority,
		Reason:    "promotion",
		Timestamp: time.Now().Add(-time.Hour), // Earlier timestamp
	}

	// Process the announcement
	am.ProcessAuthorityAnnouncement(announcement)

	// Verify authority was NOT updated (last-write-wins)
	authority, ok := am.GetAuthorityForPath("world.market")
	if !ok {
		t.Fatal("Expected to find authority for world.market")
	}
	if authority.ID != nodeIdentity.ID {
		t.Errorf("Expected authority to remain %s, got %s", nodeIdentity.ID, authority.ID)
	}
}

func TestAuthorityManager_ElectByUptime(t *testing.T) {
	// Create test components
	nodeIdentity := createTestIdentity("test-node", "test-mesh", "test-node-1")
	dir := directory.NewDirectory()
	subscriptions := subscription.NewSubscriptionRegistry()

	// Create authority manager
	am := NewAuthorityManager(nodeIdentity, dir, subscriptions)

	// Create candidates with different creation times
	older := createTestIdentity("older-node", "test-mesh", "older-node-1")
	older.Created = time.Now().Add(-time.Hour) // Created earlier

	newer := createTestIdentity("newer-node", "test-mesh", "newer-node-1")
	newer.Created = time.Now() // Created later

	candidates := []*identity.Identity{newer, older}

	// Test election
	winner := am.electByUptime(candidates)

	// Verify older node wins (longest uptime)
	if winner == nil {
		t.Fatal("Expected a winner to be elected")
	}
	if winner.ID != older.ID {
		t.Errorf("Expected older node %s to win, got %s", older.ID, winner.ID)
	}
}

func TestAuthorityManager_ElectByUptime_DeterministicTiebreaker(t *testing.T) {
	// Create test components
	nodeIdentity := createTestIdentity("test-node", "test-mesh", "test-node-1")
	dir := directory.NewDirectory()
	subscriptions := subscription.NewSubscriptionRegistry()

	// Create authority manager
	am := NewAuthorityManager(nodeIdentity, dir, subscriptions)

	// Create candidates with identical creation times but different IDs
	sameTime := time.Now()

	// Note: "alice" < "bob" lexicographically, so alice should win the tiebreaker
	alice := createTestIdentity("alice", "test-mesh", "alice-node-1")
	alice.Created = sameTime

	bob := createTestIdentity("bob", "test-mesh", "bob-node-1")
	bob.Created = sameTime

	charlie := createTestIdentity("charlie", "test-mesh", "charlie-node-1")
	charlie.Created = sameTime

	// Test election with identical creation times (different orders)
	candidates1 := []*identity.Identity{bob, alice, charlie}
	candidates2 := []*identity.Identity{charlie, bob, alice}
	candidates3 := []*identity.Identity{alice, charlie, bob}

	// All elections should return alice (lexicographically first)
	winner1 := am.electByUptime(candidates1)
	winner2 := am.electByUptime(candidates2)
	winner3 := am.electByUptime(candidates3)

	if winner1 == nil || winner1.ID != "alice" {
		t.Errorf("Expected alice to win first election, got %v", winner1)
	}
	if winner2 == nil || winner2.ID != "alice" {
		t.Errorf("Expected alice to win second election, got %v", winner2)
	}
	if winner3 == nil || winner3.ID != "alice" {
		t.Errorf("Expected alice to win third election, got %v", winner3)
	}

	// Verify determinism: all elections return the same winner
	if winner1.ID != winner2.ID || winner2.ID != winner3.ID {
		t.Errorf("Elections are not deterministic: %s, %s, %s",
			winner1.ID, winner2.ID, winner3.ID)
	}
}

func TestAuthorityManager_ElectByUptime_UptimeOverridesTiebreaker(t *testing.T) {
	// Create test components
	nodeIdentity := createTestIdentity("test-node", "test-mesh", "test-node-1")
	dir := directory.NewDirectory()
	subscriptions := subscription.NewSubscriptionRegistry()

	// Create authority manager
	am := NewAuthorityManager(nodeIdentity, dir, subscriptions)

	now := time.Now()

	// Create candidates where the lexicographically later node has longer uptime
	alice := createTestIdentity("alice", "test-mesh", "alice-node-1")
	alice.Created = now // More recent (shorter uptime)

	zulu := createTestIdentity("zulu", "test-mesh", "zulu-node-1")
	zulu.Created = now.Add(-time.Hour) // Earlier (longer uptime)

	candidates := []*identity.Identity{alice, zulu}

	// Zulu should win due to longer uptime, despite being lexicographically later
	winner := am.electByUptime(candidates)

	if winner == nil || winner.ID != "zulu" {
		t.Errorf("Expected zulu to win due to longer uptime, got %v", winner)
	}
}

func TestAuthorityManager_GetAuthorityStats(t *testing.T) {
	// Create test components
	nodeIdentity := createTestIdentity("test-node", "test-mesh", "test-node-1")
	dir := directory.NewDirectory()
	subscriptions := subscription.NewSubscriptionRegistry()

	// Create authority manager
	am := NewAuthorityManager(nodeIdentity, dir, subscriptions)

	// Set up some authority and write tracking
	am.AnnounceAuthority("world.market", "initial")
	am.AnnounceAuthority("world.users", "initial")
	am.RecordWrite("world.market")

	// Get stats
	stats := am.GetAuthorityStats()

	// Verify expected stats are present
	if authorityCount, ok := stats["authority_count"]; !ok {
		t.Error("Expected authority_count in stats")
	} else if count, ok := authorityCount.(int); !ok || count != 2 {
		t.Errorf("Expected authority_count 2, got %v", authorityCount)
	}

	if trackedPaths, ok := stats["tracked_paths"]; !ok {
		t.Error("Expected tracked_paths in stats")
	} else if count, ok := trackedPaths.(int); !ok || count != 1 {
		t.Errorf("Expected tracked_paths 1, got %v", trackedPaths)
	}

	if activeElections, ok := stats["active_elections"]; !ok {
		t.Error("Expected active_elections in stats")
	} else if count, ok := activeElections.(int); !ok || count != 0 {
		t.Errorf("Expected active_elections 0, got %v", activeElections)
	}
}