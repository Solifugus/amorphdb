// Package subscription manages subscription registry for distributed AmorphDB mesh networks
package subscription

import (
	"sync"

	"github.com/solifugus/amorphdb/internal/identity"
)

// Subscription represents a path this node subscribes to
type Subscription struct {
	Path      string             `json:"path"`      // Tree path (e.g., "world.market.equities")
	Authority *identity.Identity `json:"authority"` // Node holding write authority for this path
}

// SubscriptionRegistry manages subscription state for a node
type SubscriptionRegistry struct {
	mu sync.RWMutex

	// Paths this node subscribes to (and their authorities)
	subscriptions map[string]*identity.Identity // path -> authority

	// Remote nodes that subscribe to paths we have authority for
	subscribers map[string]map[string]*identity.Identity // path -> (subscriberNodeID -> subscriber)
}

// NewSubscriptionRegistry creates a new subscription registry
func NewSubscriptionRegistry() *SubscriptionRegistry {
	return &SubscriptionRegistry{
		subscriptions: make(map[string]*identity.Identity),
		subscribers:   make(map[string]map[string]*identity.Identity),
	}
}

// Subscribe registers this node as a subscriber to a path from an authority node
func (sr *SubscriptionRegistry) Subscribe(path string, authorityNode *identity.Identity) {
	sr.mu.Lock()
	defer sr.mu.Unlock()

	sr.subscriptions[path] = authorityNode
}

// Unsubscribe removes subscription to a path
func (sr *SubscriptionRegistry) Unsubscribe(path string) {
	sr.mu.Lock()
	defer sr.mu.Unlock()

	delete(sr.subscriptions, path)
}

// AddSubscriber records that a remote node subscribes to a path we own authority for
func (sr *SubscriptionRegistry) AddSubscriber(path string, subscriberNode *identity.Identity) {
	sr.mu.Lock()
	defer sr.mu.Unlock()

	if sr.subscribers[path] == nil {
		sr.subscribers[path] = make(map[string]*identity.Identity)
	}

	sr.subscribers[path][subscriberNode.ID] = subscriberNode
}

// RemoveSubscriber removes a subscriber record for a path
func (sr *SubscriptionRegistry) RemoveSubscriber(path string, subscriberNode *identity.Identity) {
	sr.mu.Lock()
	defer sr.mu.Unlock()

	if pathSubscribers, exists := sr.subscribers[path]; exists {
		delete(pathSubscribers, subscriberNode.ID)

		// Clean up empty path entry
		if len(pathSubscribers) == 0 {
			delete(sr.subscribers, path)
		}
	}
}

// Subscribers returns all subscribers for a given path
func (sr *SubscriptionRegistry) Subscribers(path string) []*identity.Identity {
	sr.mu.RLock()
	defer sr.mu.RUnlock()

	pathSubscribers, exists := sr.subscribers[path]
	if !exists {
		return nil
	}

	subscribers := make([]*identity.Identity, 0, len(pathSubscribers))
	for _, subscriber := range pathSubscribers {
		subscribers = append(subscribers, subscriber)
	}

	return subscribers
}

// Subscriptions returns all paths this node subscribes to
func (sr *SubscriptionRegistry) Subscriptions() []Subscription {
	sr.mu.RLock()
	defer sr.mu.RUnlock()

	subscriptions := make([]Subscription, 0, len(sr.subscriptions))
	for path, authority := range sr.subscriptions {
		subscriptions = append(subscriptions, Subscription{
			Path:      path,
			Authority: authority,
		})
	}

	return subscriptions
}

// GetSubscriptionAuthority returns the authority for a subscribed path, if subscribed
func (sr *SubscriptionRegistry) GetSubscriptionAuthority(path string) (*identity.Identity, bool) {
	sr.mu.RLock()
	defer sr.mu.RUnlock()

	authority, exists := sr.subscriptions[path]
	return authority, exists
}

// IsSubscribed returns true if this node subscribes to the given path
func (sr *SubscriptionRegistry) IsSubscribed(path string) bool {
	sr.mu.RLock()
	defer sr.mu.RUnlock()

	_, exists := sr.subscriptions[path]
	return exists
}

// HasSubscribers returns true if the given path has any subscribers
func (sr *SubscriptionRegistry) HasSubscribers(path string) bool {
	sr.mu.RLock()
	defer sr.mu.RUnlock()

	pathSubscribers, exists := sr.subscribers[path]
	return exists && len(pathSubscribers) > 0
}

// SubscriptionCount returns the number of paths this node subscribes to
func (sr *SubscriptionRegistry) SubscriptionCount() int {
	sr.mu.RLock()
	defer sr.mu.RUnlock()
	return len(sr.subscriptions)
}

// SubscriberCount returns the total number of subscribers across all paths we have authority for
func (sr *SubscriptionRegistry) SubscriberCount() int {
	sr.mu.RLock()
	defer sr.mu.RUnlock()

	count := 0
	for _, pathSubscribers := range sr.subscribers {
		count += len(pathSubscribers)
	}
	return count
}

// PathsWithSubscribers returns all paths that have subscribers
func (sr *SubscriptionRegistry) PathsWithSubscribers() []string {
	sr.mu.RLock()
	defer sr.mu.RUnlock()

	paths := make([]string, 0, len(sr.subscribers))
	for path := range sr.subscribers {
		paths = append(paths, path)
	}
	return paths
}