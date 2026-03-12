// Package security implements stamps - automatic meta-attribute injection system
package security

import (
	"crypto/sha256"
	"fmt"

	"github.com/solifugus/amorphdb/internal/storage"
	"github.com/solifugus/amorphdb/internal/types"
)

// StampManager handles stamp collection, merging, and injection
type StampManager struct {
	tree          storage.ExtendedTree
	stampCache    map[string]*StampSnapshot // Cache for stamp snapshots
	snapshotCache map[string]storage.Hash   // Cache mapping stamp hash to snapshot hash
}

// StampSnapshot represents a frozen stamp that can be embedded in instances
type StampSnapshot struct {
	Attributes map[string]interface{} // The effective stamp attributes
	Hash       storage.Hash          // Hash of this snapshot for caching
}

// NewStampManager creates a new stamp manager
func NewStampManager(tree storage.ExtendedTree) *StampManager {
	return &StampManager{
		tree:          tree,
		stampCache:    make(map[string]*StampSnapshot),
		snapshotCache: make(map[string]storage.Hash),
	}
}

// CollectStamps gathers stamps from write point up to ~ and creates effective stamp
func (sm *StampManager) CollectStamps(writePath []string, agentID uint64) (*StampSnapshot, error) {
	// Start with system-injected author
	effectiveStamp := map[string]interface{}{
		"@author": agentID, // System-level guarantee
	}

	// Add personal stamp from ~.stamp
	personalStamp, err := sm.getPersonalStamp(agentID)
	if err == nil && personalStamp != nil {
		// Personal stamp attributes go into effective stamp
		for key, value := range personalStamp {
			effectiveStamp[key] = value
		}
	}

	// Walk up the hierarchy collecting @stamp meta-attributes
	// Start from writePath and go up to ~
	for i := len(writePath); i >= 0; i-- {
		currentPath := writePath[:i]
		hierarchicalStamp, err := sm.getHierarchicalStamp(currentPath, agentID)
		if err == nil && hierarchicalStamp != nil {
			// Merge hierarchical stamp (deeper wins on collision)
			for key, value := range hierarchicalStamp {
				effectiveStamp[key] = value
			}
		}
	}

	// Create stamp hash for caching
	stampHash := sm.hashStamp(effectiveStamp)

	// Check if we already have a snapshot for this stamp
	if snapshotHash, exists := sm.snapshotCache[stampHash]; exists {
		// Reuse existing snapshot
		return sm.getSnapshot(snapshotHash)
	}

	// Create new snapshot
	snapshot := &StampSnapshot{
		Attributes: effectiveStamp,
		Hash:       storage.HashValue(types.SerializeValue(effectiveStamp)),
	}

	// Cache the snapshot
	sm.stampCache[stampHash] = snapshot
	sm.snapshotCache[stampHash] = snapshot.Hash

	return snapshot, nil
}

// getPersonalStamp retrieves the agent's personal stamp from ~.stamp
func (sm *StampManager) getPersonalStamp(agentID uint64) (map[string]interface{}, error) {
	// Get ~.stamp for this agent
	stampPath := []string{"~", "stamp"}
	attrHash := storage.HashPath(stampPath)

	instance, err := sm.tree.GetInstance(attrHash, agentID)
	if err != nil {
		return nil, err // No personal stamp
	}

	value, err := sm.tree.GetValue(instance.ValueHash)
	if err != nil {
		return nil, err
	}

	// Deserialize the stamp
	valueStruct := types.SerializedValue{TypeTag: storage.TypeText, Data: value}; stampValue, _ := types.DeserializeValue(valueStruct)
	if record, ok := stampValue.(types.Record); ok {
		return record.Fields, nil
	}

	return nil, fmt.Errorf("personal stamp is not a record")
}

// getHierarchicalStamp retrieves @stamp at a specific path level
func (sm *StampManager) getHierarchicalStamp(path []string, agentID uint64) (map[string]interface{}, error) {
	// Look for @stamp meta-attribute at this path
	stampPath := append(path, "@stamp")
	attrHash := storage.HashPath(stampPath)

	instance, err := sm.tree.GetInstance(attrHash, agentID)
	if err != nil {
		return nil, err // No stamp at this level
	}

	value, err := sm.tree.GetValue(instance.ValueHash)
	if err != nil {
		return nil, err
	}

	// Deserialize the stamp
	valueStruct := types.SerializedValue{TypeTag: storage.TypeText, Data: value}; stampValue, _ := types.DeserializeValue(valueStruct)
	if record, ok := stampValue.(types.Record); ok {
		return record.Fields, nil
	}

	return nil, fmt.Errorf("hierarchical stamp is not a record")
}

// hashStamp creates a consistent hash for a stamp map for caching
func (sm *StampManager) hashStamp(stamp map[string]interface{}) string {
	// Create a deterministic string representation of the stamp
	serialized := types.SerializeValue(stamp)
	hash := sha256.Sum256(serialized)
	return fmt.Sprintf("%x", hash)
}

// getSnapshot retrieves a cached snapshot
func (sm *StampManager) getSnapshot(snapshotHash storage.Hash) (*StampSnapshot, error) {
	// In a full implementation, this would load from storage
	// For now, assume it's in memory cache
	for _, snapshot := range sm.stampCache {
		if snapshot.Hash == snapshotHash {
			return snapshot, nil
		}
	}
	return nil, fmt.Errorf("snapshot not found")
}

// EmbedStamp applies stamp attributes to an instance, respecting (protected) attributes
func (sm *StampManager) EmbedStamp(instance map[string]interface{}, stamp *StampSnapshot) map[string]interface{} {
	result := make(map[string]interface{})

	// First, copy all original instance attributes
	for key, value := range instance {
		result[key] = value
	}

	// Then, add stamp attributes if they don't conflict or aren't protected
	for key, value := range stamp.Attributes {
		// Check if instance already has this attribute
		if existingValue, exists := result[key]; exists {
			// Instance attribute wins - check if it's marked as protected
			if sm.isProtectedAttribute(existingValue) {
				// Protected attribute cannot be overridden by stamp
				continue
			}
			// If not protected, instance attribute still wins over stamp
			continue
		}

		// No conflict, add stamp attribute
		result[key] = value
	}

	return result
}

// isProtectedAttribute checks if an attribute is marked as (protected)
func (sm *StampManager) isProtectedAttribute(value interface{}) bool {
	// Check if the value has protection metadata
	if valueMap, ok := value.(map[string]interface{}); ok {
		if protected, exists := valueMap["@protected"]; exists {
			if protectedBool, ok := protected.(bool); ok {
				return protectedBool
			}
		}
	}
	return false
}

// SetPersonalStamp sets an agent's personal stamp at ~.stamp
func (sm *StampManager) SetPersonalStamp(stamp map[string]interface{}, agentID uint64) error {
	stampPath := []string{"~", "stamp"}
	attrHash := storage.HashPath(stampPath)

	// Serialize the stamp
	stampRecord := types.Record{Fields: stamp}
	value := types.SerializeValue(stampRecord)
	valueHash := storage.HashValue(value)

	// Create instance
	instance := storage.SecurityInstance{
		AttributeHash: attrHash,
		ValueHash:     valueHash,
		Agent:         agentID,
		Timestamp:     storage.GetCurrentTimestamp(),
	}

	// Store value and instance
	err := sm.tree.PutValue(valueHash, value)
	if err != nil {
		return fmt.Errorf("failed to store stamp value: %v", err)
	}

	err = sm.tree.PutInstance(attrHash, instance)
	if err != nil {
		return fmt.Errorf("failed to store stamp instance: %v", err)
	}

	// Invalidate cache for this agent's stamps
	sm.invalidateStampCache()

	return nil
}

// SetHierarchicalStamp sets a hierarchical stamp at a specific path
func (sm *StampManager) SetHierarchicalStamp(path []string, stamp map[string]interface{}, agentID uint64) error {
	stampPath := append(path, "@stamp")
	attrHash := storage.HashPath(stampPath)

	// Serialize the stamp
	stampRecord := types.Record{Fields: stamp}
	value := types.SerializeValue(stampRecord)
	valueHash := storage.HashValue(value)

	// Create instance
	instance := storage.SecurityInstance{
		AttributeHash: attrHash,
		ValueHash:     valueHash,
		Agent:         agentID,
		Timestamp:     storage.GetCurrentTimestamp(),
	}

	// Store value and instance
	err := sm.tree.PutValue(valueHash, value)
	if err != nil {
		return fmt.Errorf("failed to store stamp value: %v", err)
	}

	err = sm.tree.PutInstance(attrHash, instance)
	if err != nil {
		return fmt.Errorf("failed to store stamp instance: %v", err)
	}

	// Invalidate cache since hierarchy changed
	sm.invalidateStampCache()

	return nil
}

// invalidateStampCache clears the stamp cache when stamps change
func (sm *StampManager) invalidateStampCache() {
	sm.stampCache = make(map[string]*StampSnapshot)
	sm.snapshotCache = make(map[string]storage.Hash)
}

// CreateProtectedAttribute creates an attribute with protection metadata
func CreateProtectedAttribute(value interface{}) map[string]interface{} {
	return map[string]interface{}{
		"value":      value,
		"@protected": true,
	}
}

// GetAttributeValue extracts the actual value from a potentially protected attribute
func GetAttributeValue(attr interface{}) interface{} {
	if attrMap, ok := attr.(map[string]interface{}); ok {
		if value, exists := attrMap["value"]; exists {
			return value
		}
	}
	return attr
}
