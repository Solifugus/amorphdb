package subscription

import (
	"fmt"
	"testing"
	"time"

	"github.com/solifugus/amorphdb/internal/identity"
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

func TestSubscriptionRegistry_SubscribeAndListSubscriptions(t *testing.T) {
	registry := NewSubscriptionRegistry()

	// Create test identities
	authority1 := createTestIdentity("auth-1", "test-mesh", "auth-node-1")
	authority2 := createTestIdentity("auth-2", "test-mesh", "auth-node-2")

	// Test initial state
	subscriptions := registry.Subscriptions()
	if len(subscriptions) != 0 {
		t.Errorf("Expected 0 initial subscriptions, got %d", len(subscriptions))
	}

	// Subscribe to first path
	registry.Subscribe("world.market", authority1)

	// Verify subscription
	if !registry.IsSubscribed("world.market") {
		t.Error("Expected to be subscribed to world.market")
	}

	auth, ok := registry.GetSubscriptionAuthority("world.market")
	if !ok {
		t.Fatal("Expected to find authority for world.market")
	}
	if auth.ID != authority1.ID {
		t.Errorf("Expected authority %s, got %s", authority1.ID, auth.ID)
	}

	// Subscribe to second path
	registry.Subscribe("world.users", authority2)

	// Verify both subscriptions
	subscriptions = registry.Subscriptions()
	if len(subscriptions) != 2 {
		t.Errorf("Expected 2 subscriptions, got %d", len(subscriptions))
	}

	if registry.SubscriptionCount() != 2 {
		t.Errorf("Expected subscription count 2, got %d", registry.SubscriptionCount())
	}

	// Verify subscription details
	pathFound := make(map[string]bool)
	for _, sub := range subscriptions {
		pathFound[sub.Path] = true
		if sub.Path == "world.market" && sub.Authority.ID != authority1.ID {
			t.Errorf("Expected world.market authority %s, got %s", authority1.ID, sub.Authority.ID)
		}
		if sub.Path == "world.users" && sub.Authority.ID != authority2.ID {
			t.Errorf("Expected world.users authority %s, got %s", authority2.ID, sub.Authority.ID)
		}
	}

	if !pathFound["world.market"] || !pathFound["world.users"] {
		t.Error("Expected to find both world.market and world.users in subscriptions")
	}
}

func TestSubscriptionRegistry_Unsubscribe(t *testing.T) {
	registry := NewSubscriptionRegistry()
	authority := createTestIdentity("auth-1", "test-mesh", "auth-node-1")

	// Subscribe to a path
	registry.Subscribe("world.market", authority)

	// Verify subscription
	if !registry.IsSubscribed("world.market") {
		t.Fatal("Expected to be subscribed to world.market")
	}

	// Unsubscribe
	registry.Unsubscribe("world.market")

	// Verify unsubscription
	if registry.IsSubscribed("world.market") {
		t.Error("Expected not to be subscribed to world.market after unsubscribe")
	}

	_, ok := registry.GetSubscriptionAuthority("world.market")
	if ok {
		t.Error("Expected not to find authority for world.market after unsubscribe")
	}

	if registry.SubscriptionCount() != 0 {
		t.Errorf("Expected subscription count 0, got %d", registry.SubscriptionCount())
	}
}

func TestSubscriptionRegistry_AddAndRemoveSubscribers(t *testing.T) {
	registry := NewSubscriptionRegistry()

	// Create test subscriber identities
	subscriber1 := createTestIdentity("sub-1", "test-mesh", "sub-node-1")
	subscriber2 := createTestIdentity("sub-2", "test-mesh", "sub-node-2")

	// Test initial state
	subscribers := registry.Subscribers("world.market")
	if len(subscribers) != 0 {
		t.Errorf("Expected 0 initial subscribers, got %d", len(subscribers))
	}

	if registry.HasSubscribers("world.market") {
		t.Error("Expected no subscribers initially")
	}

	// Add first subscriber
	registry.AddSubscriber("world.market", subscriber1)

	// Verify subscriber was added
	if !registry.HasSubscribers("world.market") {
		t.Error("Expected to have subscribers for world.market")
	}

	subscribers = registry.Subscribers("world.market")
	if len(subscribers) != 1 {
		t.Errorf("Expected 1 subscriber, got %d", len(subscribers))
	}

	if subscribers[0].ID != subscriber1.ID {
		t.Errorf("Expected subscriber %s, got %s", subscriber1.ID, subscribers[0].ID)
	}

	// Add second subscriber
	registry.AddSubscriber("world.market", subscriber2)

	// Verify both subscribers
	subscribers = registry.Subscribers("world.market")
	if len(subscribers) != 2 {
		t.Errorf("Expected 2 subscribers, got %d", len(subscribers))
	}

	subscriberIDs := make(map[string]bool)
	for _, sub := range subscribers {
		subscriberIDs[sub.ID] = true
	}

	if !subscriberIDs[subscriber1.ID] || !subscriberIDs[subscriber2.ID] {
		t.Error("Expected to find both subscribers")
	}

	if registry.SubscriberCount() != 2 {
		t.Errorf("Expected subscriber count 2, got %d", registry.SubscriberCount())
	}

	// Remove first subscriber
	registry.RemoveSubscriber("world.market", subscriber1)

	// Verify removal
	subscribers = registry.Subscribers("world.market")
	if len(subscribers) != 1 {
		t.Errorf("Expected 1 subscriber after removal, got %d", len(subscribers))
	}

	if subscribers[0].ID != subscriber2.ID {
		t.Errorf("Expected remaining subscriber %s, got %s", subscriber2.ID, subscribers[0].ID)
	}

	// Remove second subscriber
	registry.RemoveSubscriber("world.market", subscriber2)

	// Verify all subscribers removed
	if registry.HasSubscribers("world.market") {
		t.Error("Expected no subscribers after removing all")
	}

	subscribers = registry.Subscribers("world.market")
	if len(subscribers) != 0 {
		t.Errorf("Expected 0 subscribers after removing all, got %d", len(subscribers))
	}

	if registry.SubscriberCount() != 0 {
		t.Errorf("Expected subscriber count 0, got %d", registry.SubscriberCount())
	}
}

func TestSubscriptionRegistry_MultipleSubscribersPerPath(t *testing.T) {
	registry := NewSubscriptionRegistry()

	// Create multiple subscriber identities
	subscribers := make([]*identity.Identity, 5)
	for i := 0; i < 5; i++ {
		subscribers[i] = createTestIdentity(
			fmt.Sprintf("sub-%d", i+1),
			"test-mesh",
			fmt.Sprintf("sub-node-%d", i+1),
		)
	}

	// Add all subscribers to the same path
	for _, subscriber := range subscribers {
		registry.AddSubscriber("world.market.equities", subscriber)
	}

	// Verify all subscribers are tracked
	actualSubscribers := registry.Subscribers("world.market.equities")
	if len(actualSubscribers) != 5 {
		t.Errorf("Expected 5 subscribers, got %d", len(actualSubscribers))
	}

	// Verify all subscriber IDs are present
	expectedIDs := make(map[string]bool)
	for _, subscriber := range subscribers {
		expectedIDs[subscriber.ID] = true
	}

	for _, actualSubscriber := range actualSubscribers {
		if !expectedIDs[actualSubscriber.ID] {
			t.Errorf("Found unexpected subscriber %s", actualSubscriber.ID)
		}
		delete(expectedIDs, actualSubscriber.ID)
	}

	if len(expectedIDs) != 0 {
		t.Errorf("Missing subscribers: %v", expectedIDs)
	}

	// Test paths with subscribers
	pathsWithSubs := registry.PathsWithSubscribers()
	if len(pathsWithSubs) != 1 {
		t.Errorf("Expected 1 path with subscribers, got %d", len(pathsWithSubs))
	}
	if pathsWithSubs[0] != "world.market.equities" {
		t.Errorf("Expected path world.market.equities, got %s", pathsWithSubs[0])
	}
}

func TestSubscriptionRegistry_MultiplePathsMultipleSubscribers(t *testing.T) {
	registry := NewSubscriptionRegistry()

	// Create subscribers and authorities
	authority1 := createTestIdentity("auth-1", "test-mesh", "auth-node-1")
	authority2 := createTestIdentity("auth-2", "test-mesh", "auth-node-2")

	subscriber1 := createTestIdentity("sub-1", "test-mesh", "sub-node-1")
	subscriber2 := createTestIdentity("sub-2", "test-mesh", "sub-node-2")
	subscriber3 := createTestIdentity("sub-3", "test-mesh", "sub-node-3")

	// This node subscribes to some paths
	registry.Subscribe("world.external.api", authority1)
	registry.Subscribe("world.external.feed", authority2)

	// Remote nodes subscribe to paths we have authority for
	registry.AddSubscriber("world.market", subscriber1)
	registry.AddSubscriber("world.market", subscriber2)
	registry.AddSubscriber("world.users", subscriber2)
	registry.AddSubscriber("world.users", subscriber3)

	// Verify subscription state
	if registry.SubscriptionCount() != 2 {
		t.Errorf("Expected 2 subscriptions, got %d", registry.SubscriptionCount())
	}

	if registry.SubscriberCount() != 4 {
		t.Errorf("Expected 4 total subscribers, got %d", registry.SubscriberCount())
	}

	// Verify specific path subscribers
	marketSubscribers := registry.Subscribers("world.market")
	if len(marketSubscribers) != 2 {
		t.Errorf("Expected 2 subscribers for world.market, got %d", len(marketSubscribers))
	}

	userSubscribers := registry.Subscribers("world.users")
	if len(userSubscribers) != 2 {
		t.Errorf("Expected 2 subscribers for world.users, got %d", len(userSubscribers))
	}

	// Verify paths with subscribers
	pathsWithSubs := registry.PathsWithSubscribers()
	if len(pathsWithSubs) != 2 {
		t.Errorf("Expected 2 paths with subscribers, got %d", len(pathsWithSubs))
	}
}

func TestSubscriptionRegistry_ConcurrentOperations(t *testing.T) {
	registry := NewSubscriptionRegistry()

	authority := createTestIdentity("auth-1", "test-mesh", "auth-node-1")
	subscriber := createTestIdentity("sub-1", "test-mesh", "sub-node-1")

	done := make(chan bool, 4)

	// Concurrent subscription operations
	go func() {
		for i := 0; i < 100; i++ {
			path := fmt.Sprintf("world.test.%d", i%10)
			registry.Subscribe(path, authority)
		}
		done <- true
	}()

	// Concurrent unsubscription operations
	go func() {
		for i := 0; i < 100; i++ {
			path := fmt.Sprintf("world.test.%d", i%10)
			registry.Unsubscribe(path)
		}
		done <- true
	}()

	// Concurrent subscriber addition
	go func() {
		for i := 0; i < 100; i++ {
			path := fmt.Sprintf("world.sub.%d", i%5)
			registry.AddSubscriber(path, subscriber)
		}
		done <- true
	}()

	// Concurrent subscriber removal
	go func() {
		for i := 0; i < 100; i++ {
			path := fmt.Sprintf("world.sub.%d", i%5)
			registry.RemoveSubscriber(path, subscriber)
		}
		done <- true
	}()

	// Wait for all goroutines to complete
	for i := 0; i < 4; i++ {
		<-done
	}

	// Verify registry is in a valid state (no crashes)
	subscriptions := registry.Subscriptions()
	if subscriptions == nil {
		t.Error("Expected valid subscriptions slice")
	}

	pathsWithSubs := registry.PathsWithSubscribers()
	if pathsWithSubs == nil {
		t.Error("Expected valid paths with subscribers slice")
	}
}

func TestSubscriptionRegistry_EdgeCases(t *testing.T) {
	registry := NewSubscriptionRegistry()

	authority := createTestIdentity("auth-1", "test-mesh", "auth-node-1")
	subscriber := createTestIdentity("sub-1", "test-mesh", "sub-node-1")

	// Test unsubscribe from non-existent subscription
	registry.Unsubscribe("world.nonexistent")

	// Test remove subscriber from non-existent path
	registry.RemoveSubscriber("world.nonexistent", subscriber)

	// Test remove non-existent subscriber from existing path
	registry.AddSubscriber("world.market", subscriber)
	nonExistentSubscriber := createTestIdentity("non-existent", "test-mesh", "non-node")
	registry.RemoveSubscriber("world.market", nonExistentSubscriber)

	// Verify the existing subscriber is still there
	subscribers := registry.Subscribers("world.market")
	if len(subscribers) != 1 {
		t.Errorf("Expected 1 subscriber, got %d", len(subscribers))
	}

	// Test duplicate subscription (should replace)
	newAuthority := createTestIdentity("new-auth", "test-mesh", "new-auth-node")
	registry.Subscribe("world.test", authority)
	registry.Subscribe("world.test", newAuthority)

	auth, ok := registry.GetSubscriptionAuthority("world.test")
	if !ok {
		t.Fatal("Expected to find subscription for world.test")
	}
	if auth.ID != newAuthority.ID {
		t.Errorf("Expected new authority %s, got %s", newAuthority.ID, auth.ID)
	}

	// Test duplicate subscriber (should not create duplicates)
	registry.AddSubscriber("world.duplicate", subscriber)
	registry.AddSubscriber("world.duplicate", subscriber)

	subscribers = registry.Subscribers("world.duplicate")
	if len(subscribers) != 1 {
		t.Errorf("Expected 1 subscriber after adding duplicate, got %d", len(subscribers))
	}
}