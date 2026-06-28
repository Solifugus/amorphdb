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

func TestReplicationManager_SubscriptionBasedReplication_Integration(t *testing.T) {
	// Create configuration with subscription replication enabled
	cfg := &config.Config{
		Mesh: config.MeshConfig{
			UseSubscriptionReplication: true,
		},
	}

	// Create authority node
	authorityIdentity := &identity.Identity{
		ID:       "authority-node",
		Type:     types.NodeAgent,
		MeshName: "test-mesh",
		Created:  time.Now(),
		NodeID:   "authority-1",
	}

	authorityTree := storage.NewMemoryTree()
	authorityDirectory := directory.NewDirectory()
	authoritySubscriptions := subscription.NewSubscriptionRegistry()

	authorityRM := NewReplicationManager(
		"authority-node",
		cfg,
		nil,
		authorityTree,
		authorityDirectory,
		authoritySubscriptions,
		authorityIdentity,
	)

	// Create subscriber node
	subscriberIdentity := &identity.Identity{
		ID:       "subscriber-node",
		Type:     types.NodeAgent,
		MeshName: "test-mesh",
		Created:  time.Now(),
		NodeID:   "subscriber-1",
	}

	subscriberTree := storage.NewMemoryTree()
	subscriberDirectory := directory.NewDirectory()
	subscriberSubscriptions := subscription.NewSubscriptionRegistry()

	subscriberRM := NewReplicationManager(
		"subscriber-node",
		cfg,
		nil,
		subscriberTree,
		subscriberDirectory,
		subscriberSubscriptions,
		subscriberIdentity,
	)

	// Test scenario: End-to-end subscription-based replication
	pathStr := "world.market.prices"

	// Step 1: Set up authority for the path
	authorityRM.pathDirectory.Set(pathStr, authorityIdentity, time.Now())
	subscriberRM.pathDirectory.Set(pathStr, authorityIdentity, time.Now()) // Both nodes know the authority

	// Step 2: Subscriber subscribes to the path
	err := subscriberRM.RequestSubscription(pathStr)
	if err != nil {
		t.Fatalf("Failed to request subscription: %v", err)
	}

	// Step 3: Authority adds subscriber to its registry
	authorityRM.subscriptionReg.AddSubscriber(pathStr, subscriberIdentity)

	// Step 4: Authority performs a write
	path := []string{"world", "market", "prices"}
	value := storage.Value{
		TypeTag: storage.TypeText,
		Data:    []byte("BTC: $45000, ETH: $3000"),
	}
	author := uint64(1001)

	err = authorityRM.RecordPathWrite(path, value, author)
	if err != nil {
		t.Fatalf("Failed to record path write: %v", err)
	}

	// Step 5: Verify pending write was created on authority
	authorityRM.mu.RLock()
	pendingWrites := authorityRM.pendingPathWrites[pathStr]
	authorityRM.mu.RUnlock()

	if len(pendingWrites) != 1 {
		t.Fatalf("Expected 1 pending write on authority, got %d", len(pendingWrites))
	}

	write := pendingWrites[0]
	if len(write.Subscribers) != 1 || write.Subscribers[0].ID != subscriberIdentity.ID {
		t.Errorf("Expected write to have subscriber %s, got %v", subscriberIdentity.ID, write.Subscribers)
	}

	// Step 6: Simulate replication process
	authorityRM.processSubscriptionReplication()

	// Step 7: Verify subscription tracking on subscriber
	subscriberRM.mu.RLock()
	subscribedPath, exists := subscriberRM.subscribedPaths[pathStr]
	subscriberRM.mu.RUnlock()

	if !exists {
		t.Fatal("Expected subscriber to have subscription record")
	}

	if subscribedPath.Authority.ID != authorityIdentity.ID {
		t.Errorf("Expected subscription authority %s, got %s", authorityIdentity.ID, subscribedPath.Authority.ID)
	}
}

func TestReplicationManager_WriteRouting_Integration(t *testing.T) {
	// Test write routing: writes to non-authority nodes should be forwarded

	cfg := &config.Config{
		Mesh: config.MeshConfig{
			UseSubscriptionReplication: true,
		},
	}

	// Create authority node
	authorityIdentity := &identity.Identity{
		ID:       "authority-node",
		Type:     types.NodeAgent,
		MeshName: "test-mesh",
		Created:  time.Now(),
		NodeID:   "authority-1",
	}

	// Create non-authority node
	nodeIdentity := &identity.Identity{
		ID:       "regular-node",
		Type:     types.NodeAgent,
		MeshName: "test-mesh",
		Created:  time.Now(),
		NodeID:   "regular-1",
	}

	nodeTree := storage.NewMemoryTree()
	nodeDirectory := directory.NewDirectory()
	nodeSubscriptions := subscription.NewSubscriptionRegistry()

	nodeRM := NewReplicationManager(
		"regular-node",
		cfg,
		nil,
		nodeTree,
		nodeDirectory,
		nodeSubscriptions,
		nodeIdentity,
	)

	// Set up authority in directory (not this node)
	pathStr := "world.orders"
	nodeRM.pathDirectory.Set(pathStr, authorityIdentity, time.Now())

	// Test write forwarding
	path := []string{"world", "orders"}
	value := storage.Value{
		TypeTag: storage.TypeText,
		Data:    []byte("order-12345"),
	}
	author := uint64(2001)

	err := nodeRM.ForwardWriteToAuthority(path, value, author)

	// This should not fail (the actual network call is mocked/stubbed)
	if err != nil {
		t.Fatalf("ForwardWriteToAuthority failed: %v", err)
	}

	// Verify this node didn't try to handle the write locally
	nodeRM.mu.RLock()
	pendingWrites := nodeRM.pendingPathWrites[pathStr]
	nodeRM.mu.RUnlock()

	// Should be empty since we're not the authority
	if len(pendingWrites) != 0 {
		t.Errorf("Expected 0 pending writes on non-authority node, got %d", len(pendingWrites))
	}
}

func TestReplicationManager_BootstrapSync_Integration(t *testing.T) {
	// Test bootstrap synchronization when a new subscriber joins

	cfg := &config.Config{
		Mesh: config.MeshConfig{
			UseSubscriptionReplication: true,
		},
	}

	// Create authority with existing data
	authorityIdentity := &identity.Identity{
		ID:       "authority-node",
		Type:     types.NodeAgent,
		MeshName: "test-mesh",
		Created:  time.Now(),
		NodeID:   "authority-1",
	}

	authorityTree := storage.NewMemoryTree()
	authorityDirectory := directory.NewDirectory()
	authoritySubscriptions := subscription.NewSubscriptionRegistry()

	authorityRM := NewReplicationManager(
		"authority-node",
		cfg,
		nil,
		authorityTree,
		authorityDirectory,
		authoritySubscriptions,
		authorityIdentity,
	)

	// Set up authority and write some initial data
	pathStr := "world.system.status"
	authorityRM.pathDirectory.Set(pathStr, authorityIdentity, time.Now())

	// Write initial data to storage
	path := []string{"world", "system", "status"}
	value := storage.Value{
		TypeTag: storage.TypeText,
		Data:    []byte("system-online"),
	}
	author := uint64(4001)

	err := authorityTree.Write(path, value, author)
	if err != nil {
		t.Fatalf("Failed to write initial data: %v", err)
	}

	// Create new subscriber
	subscriberIdentity := &identity.Identity{
		ID:       "new-subscriber",
		Type:     types.NodeAgent,
		MeshName: "test-mesh",
		Created:  time.Now(),
		NodeID:   "subscriber-1",
	}

	subscriberTree := storage.NewMemoryTree()
	subscriberDirectory := directory.NewDirectory()
	subscriberSubscriptions := subscription.NewSubscriptionRegistry()

	subscriberRM := NewReplicationManager(
		"new-subscriber",
		cfg,
		nil,
		subscriberTree,
		subscriberDirectory,
		subscriberSubscriptions,
		subscriberIdentity,
	)

	// Set up directory on subscriber
	subscriberRM.pathDirectory.Set(pathStr, authorityIdentity, time.Now())

	// Request subscription (should trigger bootstrap sync)
	err = subscriberRM.RequestSubscription(pathStr)
	if err != nil {
		t.Fatalf("Failed to request subscription for bootstrap: %v", err)
	}

	// Verify subscription was established
	subscriberRM.mu.RLock()
	subscribedPath, exists := subscriberRM.subscribedPaths[pathStr]
	subscriberRM.mu.RUnlock()

	if !exists {
		t.Fatal("Expected subscription to be established")
	}

	if subscribedPath.Authority.ID != authorityIdentity.ID {
		t.Errorf("Expected subscription authority %s, got %s", authorityIdentity.ID, subscribedPath.Authority.ID)
	}

	// Note: In a full implementation, the bootstrap sync would copy existing data
	// from authority to subscriber. Here we verify the subscription mechanism works.
}
