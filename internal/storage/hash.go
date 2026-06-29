package storage

import (
	"crypto/sha256"
	"fmt"
	"sync"
	"time"
)

// Hash represents a cryptographic hash value
type Hash [32]byte

// String returns a hexadecimal representation of the hash
func (h Hash) String() string {
	return fmt.Sprintf("%x", h[:])
}

// HashPath generates a hash for a path slice
func HashPath(path []string) Hash {
	// Create a deterministic string representation of the path
	pathStr := ""
	for i, component := range path {
		if i > 0 {
			pathStr += "."
		}
		pathStr += component
	}

	return sha256.Sum256([]byte(pathStr))
}

// HashValue generates a hash for a byte slice value
func HashValue(data []byte) Hash {
	return sha256.Sum256(data)
}

// GetCurrentTimestamp returns the current timestamp in microseconds since Unix epoch
func GetCurrentTimestamp() int64 {
	return time.Now().UnixMicro()
}

// Instance represents a storage instance with extended fields for security package
type SecurityInstance struct {
	AttributeHash Hash
	ValueHash     Hash
	Agent         uint64
	Timestamp     int64
	PreviousHash  Hash // Links to previous instance
}

// ExtendedTree provides additional methods needed by security package
type ExtendedTree interface {
	Tree
	GetInstance(attrHash Hash, agentID uint64) (SecurityInstance, error)
	GetValue(valueHash Hash) ([]byte, error)
	PutValue(valueHash Hash, value []byte) error
	PutInstance(attrHash Hash, instance SecurityInstance) error
	ResolveAttributePath(attributeID uint64) (string, error) // For extract operations
}

// TreeAdapter adapts a regular Tree to ExtendedTree interface.
//
// Path-based data (Read/Write/ReadAt/Children/Purge) is delegated to the
// underlying Tree. The hash-keyed security metadata (instances and values used
// by permissions, stamps, and filters) is held in dedicated maps so that
// PutInstance/GetInstance and PutValue/GetValue round-trip faithfully — a value
// stored by hash can be read back by that same hash, and an instance stored for
// a (attribute, agent) pair can be retrieved for that same pair. Earlier this
// metadata was faked with dummy paths and never round-tripped, which silently
// dropped every permission grant, stamp, and filter the daemon stored.
type TreeAdapter struct {
	tree      Tree
	instances map[InstanceKey]SecurityInstance // Agent-scoped instance metadata
	values    map[Hash][]byte                  // Hash-keyed value metadata
	mu        sync.RWMutex                     // Protects instances and values
}

// NewTreeAdapter creates a new tree adapter
func NewTreeAdapter(tree Tree) ExtendedTree {
	return &TreeAdapter{
		tree:      tree,
		instances: make(map[InstanceKey]SecurityInstance),
		values:    make(map[Hash][]byte),
	}
}

// Delegate Tree interface methods to underlying tree
func (ta *TreeAdapter) Read(path []string) (Value, error) {
	return ta.tree.Read(path)
}

func (ta *TreeAdapter) Write(path []string, value Value, author uint64) error {
	return ta.tree.Write(path, value, author)
}

func (ta *TreeAdapter) ReadAt(path []string, timestamp int64) (Value, error) {
	return ta.tree.ReadAt(path, timestamp)
}

func (ta *TreeAdapter) Children(path []string) ([]Attribute, error) {
	return ta.tree.Children(path)
}

func (ta *TreeAdapter) Purge(path []string, from int64, to int64, author uint64) error {
	return ta.tree.Purge(path, from, to, author)
}

// Extended methods for security package

// GetInstance retrieves the security instance previously stored for the given
// attribute hash and agent. It returns an error (not a zero value) when no
// instance has been stored, so callers correctly fall back to default
// permission/stamp/filter behavior when nothing is set.
func (ta *TreeAdapter) GetInstance(attrHash Hash, agentID uint64) (SecurityInstance, error) {
	ta.mu.RLock()
	defer ta.mu.RUnlock()

	key := InstanceKey{Hash: attrHash, AgentID: agentID}
	if instance, exists := ta.instances[key]; exists {
		return instance, nil
	}
	return SecurityInstance{}, fmt.Errorf("instance not found for hash %x and agent %d", attrHash[:8], agentID)
}

// GetValue retrieves the value bytes previously stored under the given value
// hash, returning an error when none exists.
func (ta *TreeAdapter) GetValue(valueHash Hash) ([]byte, error) {
	ta.mu.RLock()
	defer ta.mu.RUnlock()

	if value, exists := ta.values[valueHash]; exists {
		// Return a copy so callers cannot mutate the stored bytes.
		out := make([]byte, len(value))
		copy(out, value)
		return out, nil
	}
	return nil, fmt.Errorf("value not found for hash %x", valueHash[:8])
}

// PutValue stores value bytes keyed by their hash so they can be retrieved by
// GetValue with the same hash.
func (ta *TreeAdapter) PutValue(valueHash Hash, value []byte) error {
	ta.mu.Lock()
	defer ta.mu.Unlock()

	stored := make([]byte, len(value))
	copy(stored, value)
	ta.values[valueHash] = stored
	return nil
}

// PutInstance stores instance metadata keyed by (attribute hash, agent) so it
// can be retrieved by GetInstance with the same pair. The agent component comes
// from instance.Agent — permissions and hierarchical stamps store under agent 0
// (global), while personal stamps and filters store under the owning agent.
func (ta *TreeAdapter) PutInstance(attrHash Hash, instance SecurityInstance) error {
	ta.mu.Lock()
	defer ta.mu.Unlock()

	key := InstanceKey{Hash: attrHash, AgentID: instance.Agent}
	ta.instances[key] = instance
	return nil
}

func (ta *TreeAdapter) ResolveAttributePath(attributeID uint64) (string, error) {
	// Cast the underlying tree to StorageTree to access the method
	if storageTree, ok := ta.tree.(*StorageTree); ok {
		return storageTree.ResolveAttributePath(attributeID)
	}
	return "", fmt.Errorf("underlying tree does not support attribute path resolution")
}

// ListLeafPaths delegates leaf-path enumeration to the underlying tree when it
// supports it (e.g. *StorageTree), so callers holding only the adapter — such as
// PWA asset loading — can enumerate stored leaves under a prefix.
func (ta *TreeAdapter) ListLeafPaths(prefix []string) ([]string, error) {
	if lister, ok := ta.tree.(interface {
		ListLeafPaths(prefix []string) ([]string, error)
	}); ok {
		return lister.ListLeafPaths(prefix)
	}
	return nil, fmt.Errorf("underlying tree does not support leaf path listing")
}

// InstanceKey represents a composite key for agent-scoped instances
type InstanceKey struct {
	Hash    Hash
	AgentID uint64
}

// MemoryTree is an in-memory implementation of ExtendedTree for testing
type MemoryTree struct {
	data      map[string]Value                 // Path-based data storage
	instances map[InstanceKey]SecurityInstance // Agent-scoped instance storage
	values    map[Hash][]byte                  // Hash-based value storage
	mu        sync.RWMutex                     // Thread safety
}

// NewMemoryTree creates a new in-memory tree for testing
func NewMemoryTree() ExtendedTree {
	return &MemoryTree{
		data:      make(map[string]Value),
		instances: make(map[InstanceKey]SecurityInstance),
		values:    make(map[Hash][]byte),
	}
}

// Read implements Tree interface
func (mt *MemoryTree) Read(path []string) (Value, error) {
	mt.mu.RLock()
	defer mt.mu.RUnlock()

	pathStr := joinPath(path)
	if value, exists := mt.data[pathStr]; exists {
		return value, nil
	}
	return Value{}, fmt.Errorf("path not found: %s", pathStr)
}

// Write implements Tree interface
func (mt *MemoryTree) Write(path []string, value Value, author uint64) error {
	mt.mu.Lock()
	defer mt.mu.Unlock()

	pathStr := joinPath(path)
	mt.data[pathStr] = value
	return nil
}

// ReadAt implements Tree interface
func (mt *MemoryTree) ReadAt(path []string, timestamp int64) (Value, error) {
	// For simplicity, just return current value
	return mt.Read(path)
}

// Children implements Tree interface
func (mt *MemoryTree) Children(path []string) ([]Attribute, error) {
	mt.mu.RLock()
	defer mt.mu.RUnlock()

	// Simple implementation - return empty for now
	return []Attribute{}, nil
}

// Purge implements Tree interface
func (mt *MemoryTree) Purge(path []string, from int64, to int64, author uint64) error {
	mt.mu.Lock()
	defer mt.mu.Unlock()

	pathStr := joinPath(path)
	delete(mt.data, pathStr)
	return nil
}

// GetInstance implements ExtendedTree interface
func (mt *MemoryTree) GetInstance(attrHash Hash, agentID uint64) (SecurityInstance, error) {
	mt.mu.RLock()
	defer mt.mu.RUnlock()

	key := InstanceKey{Hash: attrHash, AgentID: agentID}
	if instance, exists := mt.instances[key]; exists {
		return instance, nil
	}
	return SecurityInstance{}, fmt.Errorf("instance not found for hash %x and agent %d", attrHash[:8], agentID)
}

// GetValue implements ExtendedTree interface
func (mt *MemoryTree) GetValue(valueHash Hash) ([]byte, error) {
	mt.mu.RLock()
	defer mt.mu.RUnlock()

	if value, exists := mt.values[valueHash]; exists {
		return value, nil
	}
	return nil, fmt.Errorf("value not found for hash %x", valueHash[:8])
}

// PutValue implements ExtendedTree interface
func (mt *MemoryTree) PutValue(valueHash Hash, value []byte) error {
	mt.mu.Lock()
	defer mt.mu.Unlock()

	mt.values[valueHash] = value
	return nil
}

// PutInstance implements ExtendedTree interface
func (mt *MemoryTree) PutInstance(attrHash Hash, instance SecurityInstance) error {
	mt.mu.Lock()
	defer mt.mu.Unlock()

	key := InstanceKey{Hash: attrHash, AgentID: instance.Agent}
	mt.instances[key] = instance
	return nil
}

// ResolveAttributePath implements ExtendedTree interface
func (mt *MemoryTree) ResolveAttributePath(attributeID uint64) (string, error) {
	// For MemoryTree (used in testing), return a simple path
	return fmt.Sprintf("memory.attr_%d", attributeID), nil
}

// Helper function to join path components
func joinPath(path []string) string {
	if len(path) == 0 {
		return ""
	}
	result := path[0]
	for i := 1; i < len(path); i++ {
		result += "." + path[i]
	}
	return result
}
