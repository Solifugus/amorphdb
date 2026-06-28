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

// TreeAdapter adapts a regular Tree to ExtendedTree interface
type TreeAdapter struct {
	tree Tree
}

// NewTreeAdapter creates a new tree adapter
func NewTreeAdapter(tree Tree) ExtendedTree {
	return &TreeAdapter{tree: tree}
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
func (ta *TreeAdapter) GetInstance(attrHash Hash, agentID uint64) (SecurityInstance, error) {
	// For now, simulate getting an instance by converting hash to path
	// This is a simplified implementation - in a full implementation,
	// we would need to extend the storage engine to support hash-based lookups

	// Convert hash to a dummy path for lookup
	path := []string{fmt.Sprintf("hash_%x", attrHash[:8])}

	value, err := ta.tree.Read(path)
	if err != nil {
		return SecurityInstance{}, fmt.Errorf("instance not found: %w", err)
	}

	// Create a dummy instance structure
	return SecurityInstance{
		AttributeHash: attrHash,
		ValueHash:     HashValue(value.Data),
		Agent:         agentID,
		Timestamp:     GetCurrentTimestamp(),
	}, nil
}

func (ta *TreeAdapter) GetValue(valueHash Hash) ([]byte, error) {
	// In a simplified implementation, we'll return a dummy value
	// A full implementation would need hash-to-value lookup in the value store
	return []byte(fmt.Sprintf("value_for_hash_%x", valueHash[:8])), nil
}

func (ta *TreeAdapter) PutValue(valueHash Hash, value []byte) error {
	// Store the value using a hash-based path
	path := []string{"values", fmt.Sprintf("hash_%x", valueHash[:8])}
	storageValue := Value{
		Data:    value,
		TypeTag: TypeText, // Store as text for simplicity
	}
	return ta.tree.Write(path, storageValue, 1000) // Use system agent ID
}

func (ta *TreeAdapter) PutInstance(attrHash Hash, instance SecurityInstance) error {
	// Store the instance metadata using a hash-based path
	path := []string{"instances", fmt.Sprintf("hash_%x", attrHash[:8])}

	// Serialize instance data
	instanceData := fmt.Sprintf("agent=%d,timestamp=%d,value_hash=%x",
		instance.Agent, instance.Timestamp, instance.ValueHash[:8])

	storageValue := Value{
		Data:    []byte(instanceData),
		TypeTag: TypeText,
	}
	return ta.tree.Write(path, storageValue, instance.Agent)
}

func (ta *TreeAdapter) ResolveAttributePath(attributeID uint64) (string, error) {
	// Cast the underlying tree to StorageTree to access the method
	if storageTree, ok := ta.tree.(*StorageTree); ok {
		return storageTree.ResolveAttributePath(attributeID)
	}
	return "", fmt.Errorf("underlying tree does not support attribute path resolution")
}

// InstanceKey represents a composite key for agent-scoped instances
type InstanceKey struct {
	Hash    Hash
	AgentID uint64
}

// MemoryTree is an in-memory implementation of ExtendedTree for testing
type MemoryTree struct {
	data      map[string]Value                    // Path-based data storage
	instances map[InstanceKey]SecurityInstance   // Agent-scoped instance storage
	values    map[Hash][]byte                    // Hash-based value storage
	mu        sync.RWMutex                       // Thread safety
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
