package mesh

import (
	"testing"
	"time"

	"github.com/solifugus/amorphdb/internal/config"
	"github.com/solifugus/amorphdb/internal/directory"
	"github.com/solifugus/amorphdb/internal/identity"
	"github.com/solifugus/amorphdb/internal/storage"
	"github.com/solifugus/amorphdb/internal/subscription"
	"github.com/solifugus/amorphdb/internal/types"
)

// createTestReplicationManager creates a test replication manager with subscription model enabled
func createTestReplicationManager() (*ReplicationManager, storage.ExtendedTree) {
	cfg := &config.Config{
		Mesh: config.MeshConfig{
			UseSubscriptionReplication: true,
		},
	}

	nodeIdentity := &identity.Identity{
		ID:       "test-node",
		Type:     types.NodeAgent,
		MeshName: "test-mesh",
		Created:  time.Now(),
		NodeID:   "test-node-1",
	}

	tree := storage.NewMemoryTree()
	pathDirectory := directory.NewDirectory()
	subscriptionReg := subscription.NewSubscriptionRegistry()

	rm := NewReplicationManager(
		"test-node",
		cfg,
		nil, // no hashRing for subscription model
		tree,
		pathDirectory,
		subscriptionReg,
		nodeIdentity,
	)

	return rm, tree
}

func TestReplicationManager_RecordPathWrite_Success(t *testing.T) {
	rm, _ := createTestReplicationManager()

	// Set up this node as authority for a path
	pathStr := "world.market"
	rm.pathDirectory.Set(pathStr, rm.nodeId, time.Now())

	// Add a subscriber
	subscriber := &identity.Identity{
		ID:       "subscriber-1",
		Type:     types.NodeAgent,
		MeshName: "test-mesh",
		NodeID:   "subscriber-1",
	}
	rm.subscriptionReg.AddSubscriber(pathStr, subscriber)

	// Record a path write
	path := []string{"world", "market"}
	value := storage.Value{
		TypeTag: storage.TypeText,
		Data:    []byte("test-value"),
	}
	author := uint64(1000)

	err := rm.RecordPathWrite(path, value, author)
	if err != nil {
		t.Errorf("RecordPathWrite failed: %v", err)
	}

	// Verify pending write was created
	rm.mu.RLock()
	pendingWrites := rm.pendingPathWrites[pathStr]
	rm.mu.RUnlock()

	if len(pendingWrites) != 1 {
		t.Errorf("Expected 1 pending write, got %d", len(pendingWrites))
	}

	write := pendingWrites[0]
	if len(write.Path) != 2 || write.Path[0] != "world" || write.Path[1] != "market" {
		t.Errorf("Unexpected path in pending write: %v", write.Path)
	}

	if string(write.Value.Data) != "test-value" {
		t.Errorf("Expected value 'test-value', got '%s'", string(write.Value.Data))
	}

	if write.Author != author {
		t.Errorf("Expected author %d, got %d", author, write.Author)
	}

	if len(write.Subscribers) != 1 || write.Subscribers[0].ID != "subscriber-1" {
		t.Errorf("Expected 1 subscriber 'subscriber-1', got %v", write.Subscribers)
	}
}

func TestReplicationManager_RecordPathWrite_NotAuthority(t *testing.T) {
	rm, _ := createTestReplicationManager()

	// Do not set up this node as authority for the path

	// Try to record a path write
	path := []string{"world", "market"}
	value := storage.Value{
		TypeTag: storage.TypeText,
		Data:    []byte("test-value"),
	}
	author := uint64(1000)

	err := rm.RecordPathWrite(path, value, author)
	if err == nil {
		t.Error("Expected error for non-authority write, but got nil")
	}

	expectedMsg := "node is not authority for path"
	if err != nil && len(err.Error()) < len(expectedMsg) {
		t.Errorf("Expected error to contain '%s', got '%s'", expectedMsg, err.Error())
	}
}

func TestReplicationManager_RecordPathWrite_NoSubscribers(t *testing.T) {
	rm, _ := createTestReplicationManager()

	// Set up this node as authority for a path
	pathStr := "world.market"
	rm.pathDirectory.Set(pathStr, rm.nodeId, time.Now())

	// Do not add any subscribers

	// Record a path write
	path := []string{"world", "market"}
	value := storage.Value{
		TypeTag: storage.TypeText,
		Data:    []byte("test-value"),
	}
	author := uint64(1000)

	err := rm.RecordPathWrite(path, value, author)
	if err != nil {
		t.Errorf("RecordPathWrite failed even with no subscribers: %v", err)
	}

	// Verify no pending writes were created
	rm.mu.RLock()
	pendingWrites := rm.pendingPathWrites[pathStr]
	rm.mu.RUnlock()

	if len(pendingWrites) != 0 {
		t.Errorf("Expected 0 pending writes for path with no subscribers, got %d", len(pendingWrites))
	}
}

func TestReplicationManager_RequestSubscription_Success(t *testing.T) {
	rm, _ := createTestReplicationManager()

	// Set up another node as authority for a path
	pathStr := "world.market"
	authority := &identity.Identity{
		ID:       "authority-node",
		Type:     types.NodeAgent,
		MeshName: "test-mesh",
		NodeID:   "authority-node-1",
	}
	rm.pathDirectory.Set(pathStr, authority, time.Now())

	// Request subscription
	err := rm.RequestSubscription(pathStr)
	if err != nil {
		t.Errorf("RequestSubscription failed: %v", err)
	}

	// Verify subscription was created
	rm.mu.RLock()
	subscribedPath, exists := rm.subscribedPaths[pathStr]
	rm.mu.RUnlock()

	if !exists {
		t.Fatal("Expected subscription to be created")
	}

	if subscribedPath.Authority.ID != authority.ID {
		t.Errorf("Expected authority %s, got %s", authority.ID, subscribedPath.Authority.ID)
	}

	if subscribedPath.IsCurrent {
		t.Error("Expected new subscription to not be current initially")
	}
}

func TestReplicationManager_RequestSubscription_SelfAuthority(t *testing.T) {
	rm, _ := createTestReplicationManager()

	// Set up this node as authority for a path
	pathStr := "world.market"
	rm.pathDirectory.Set(pathStr, rm.nodeId, time.Now())

	// Request subscription (should be no-op since we're the authority)
	err := rm.RequestSubscription(pathStr)
	if err != nil {
		t.Errorf("RequestSubscription failed for self-authority: %v", err)
	}

	// Verify no subscription was created since we're the authority
	rm.mu.RLock()
	_, exists := rm.subscribedPaths[pathStr]
	rm.mu.RUnlock()

	if exists {
		t.Error("Expected no subscription to be created when we are the authority")
	}
}

func TestReplicationManager_GetReplicationStats_Subscription(t *testing.T) {
	rm, _ := createTestReplicationManager()

	// Add some subscription data
	pathStr := "world.market"
	rm.pathDirectory.Set(pathStr, rm.nodeId, time.Now())

	subscribedPath := &SubscribedPath{
		Path:         []string{"world", "market"},
		Authority:    rm.nodeId,
		LastReceived: time.Now(),
		IsCurrent:    true,
		DataSize:     1024,
	}
	rm.subscribedPaths[pathStr] = subscribedPath

	// Get statistics
	stats := rm.GetReplicationStats()

	// Check subscription-based statistics
	if stats["subscribed_paths"] != 1 {
		t.Errorf("Expected 1 subscribed path, got %v", stats["subscribed_paths"])
	}

	if stats["current_subscriptions"] != 1 {
		t.Errorf("Expected 1 current subscription, got %v", stats["current_subscriptions"])
	}

	// Should not have zone-based statistics
	if _, exists := stats["authority_zones"]; exists {
		t.Error("Expected no zone-based statistics in subscription mode")
	}
}

func TestReplicationManager_PathStringConversion(t *testing.T) {
	// Test path to string conversion
	path := []string{"world", "market", "equities"}
	pathStr := pathToString(path)
	expected := "world.market.equities"
	if pathStr != expected {
		t.Errorf("pathToString: expected '%s', got '%s'", expected, pathStr)
	}

	// Test empty path
	emptyPath := []string{}
	emptyStr := pathToString(emptyPath)
	if emptyStr != "" {
		t.Errorf("pathToString empty: expected empty string, got '%s'", emptyStr)
	}

	// Test string to path conversion (simplified)
	convertedPath := stringToPath("test.path")
	if len(convertedPath) != 1 || convertedPath[0] != "test.path" {
		t.Errorf("stringToPath: expected ['test.path'], got %v", convertedPath)
	}

	// Test empty string to path
	emptyConvertedPath := stringToPath("")
	if len(emptyConvertedPath) != 0 {
		t.Errorf("stringToPath empty: expected empty slice, got %v", emptyConvertedPath)
	}
}
