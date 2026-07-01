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
	Hash       storage.Hash           // Hash of this snapshot for caching
}

// NewStampManager creates a new stamp manager
func NewStampManager(tree storage.ExtendedTree) *StampManager {
	return &StampManager{
		tree:          tree,
		stampCache:    make(map[string]*StampSnapshot),
		snapshotCache: make(map[string]storage.Hash),
	}
}

// CollectStamps gathers stamps from write point up to the agent's home and creates effective stamp
func (sm *StampManager) CollectStamps(writePath []string, agentID uint64) (*StampSnapshot, error) {
	// Start with system-injected author
	effectiveStamp := map[string]interface{}{
		"@author": agentID, // System-level guarantee
	}

	// Add personal stamp from the agent's home stamp
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

// SetPersonalStamp sets the agent's personal stamp at the agent's home stamp
func (sm *StampManager) SetPersonalStamp(stampFields map[string]interface{}, agentID uint64) error {
	// Create a record with the stamp fields
	stampRecord := types.Record{
		Fields: stampFields,
	}

	// Store at the agent's home stamp (per-agent, keyed by agentID)
	stampPath := []string{"stamp"}
	attrHash := storage.HashPath(stampPath)

	// Serialize the stamp record
	value := types.SerializeValue(stampRecord)
	valueHash := storage.HashValue(value)

	// Create the instance
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

	return nil
}

// SetHierarchicalStamp sets a stamp at a specific hierarchical level
func (sm *StampManager) SetHierarchicalStamp(path []string, stampFields map[string]interface{}, agentID uint64) error {
	// Create a record with the stamp fields
	stampRecord := types.Record{
		Fields: stampFields,
	}

	// Store at path/@stamp
	stampPath := append(path, "@stamp")
	attrHash := storage.HashPath(stampPath)

	// Serialize the stamp record
	value := types.SerializeValue(stampRecord)
	valueHash := storage.HashValue(value)

	// A hierarchical @stamp is a property of the path itself: it applies to data
	// written at or under this path by any agent. Like permissions, it is stored
	// globally (agent 0) rather than under the setting agent, so every agent's
	// CollectStamps can find it. The agentID parameter records intent only.
	_ = agentID
	instance := storage.SecurityInstance{
		AttributeHash: attrHash,
		ValueHash:     valueHash,
		Agent:         0,
		Timestamp:     storage.GetCurrentTimestamp(),
	}

	// Store value and instance
	err := sm.tree.PutValue(valueHash, value)
	if err != nil {
		return fmt.Errorf("failed to store hierarchical stamp value: %v", err)
	}

	err = sm.tree.PutInstance(attrHash, instance)
	if err != nil {
		return fmt.Errorf("failed to store hierarchical stamp instance: %v", err)
	}

	return nil
}

// getPersonalStamp retrieves the agent's personal stamp from the agent's home stamp
func (sm *StampManager) getPersonalStamp(agentID uint64) (map[string]interface{}, error) {
	// Get the home stamp for this agent
	stampPath := []string{"stamp"}
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
	valueStruct := types.SerializedValue{TypeTag: types.TypeRecord, Data: value}
	stampValue, _ := types.DeserializeValue(valueStruct)
	if record, ok := stampValue.(types.Record); ok {
		return record.Fields, nil
	}

	return nil, fmt.Errorf("personal stamp is not a record")
}

// getHierarchicalStamp retrieves @stamp at a specific path level
func (sm *StampManager) getHierarchicalStamp(path []string, agentID uint64) (map[string]interface{}, error) {
	// Look for @stamp meta-attribute at this path. Hierarchical stamps are
	// stored globally (agent 0), so query there regardless of the active agent.
	_ = agentID
	stampPath := append(path, "@stamp")
	attrHash := storage.HashPath(stampPath)

	instance, err := sm.tree.GetInstance(attrHash, 0)
	if err != nil {
		return nil, err // No stamp at this level
	}

	value, err := sm.tree.GetValue(instance.ValueHash)
	if err != nil {
		return nil, err
	}

	// Deserialize the stamp
	valueStruct := types.SerializedValue{TypeTag: types.TypeRecord, Data: value}
	stampValue, _ := types.DeserializeValue(valueStruct)
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

// EmbedStamp applies stamp attributes to an instance, respecting collision rules:
// - Host attributes win over embed attributes
// - Later embed attributes win over earlier embed attributes
func (sm *StampManager) EmbedStamp(instance map[string]interface{}, stamp *StampSnapshot) map[string]interface{} {
	result := make(map[string]interface{})

	// Track which attributes are embedded vs original host
	embeddedAttrs := make(map[string]bool)
	hadPreviousEmbeds := false

	// Check if instance already has embedded attribute metadata
	if embeddedMeta, exists := instance["@embedded_attrs"]; exists {
		if embeddedMap, ok := embeddedMeta.(map[string]bool); ok {
			embeddedAttrs = embeddedMap
			hadPreviousEmbeds = true
		}
	}

	// First, copy all original instance attributes (except metadata)
	for key, value := range instance {
		if key != "@embedded_attrs" {
			result[key] = value
		}
	}

	// Then, add stamp attributes with proper collision handling
	for key, value := range stamp.Attributes {
		// Check if instance already has this attribute
		if existingValue, exists := result[key]; exists {
			// Check if it's marked as protected
			if sm.isProtectedAttribute(existingValue) {
				// Protected attribute cannot be overridden by stamp
				continue
			}

			// Check if existing attribute was embedded (not original host)
			if embeddedAttrs[key] {
				// Later embed wins over earlier embed
				result[key] = value
				embeddedAttrs[key] = true
			} else {
				// Existing is a host attribute - host wins, don't replace
				continue
			}
		} else {
			// No conflict, add stamp attribute as embedded
			result[key] = value
			embeddedAttrs[key] = true
		}
	}

	// Store metadata about which attributes are embedded
	// Only add metadata if this is a subsequent embed (there were previous embeds)
	// or if we're adding embeds that will be used in future embed calls
	if hadPreviousEmbeds {
		result["@embedded_attrs"] = embeddedAttrs
	} else {
		// First embed - only add metadata if there were actual embedded attributes added
		hasNewEmbedded := false
		for _, isEmbedded := range embeddedAttrs {
			if isEmbedded {
				hasNewEmbedded = true
				break
			}
		}
		if hasNewEmbedded {
			result["@embedded_attrs"] = embeddedAttrs
		}
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

// QueryByStamp searches for data that matches stamp criteria
// This implements stamp queries like my.projects[@stamp.department = "engineering"]
func (sm *StampManager) QueryByStamp(data map[string]interface{}, stampKey string, stampValue interface{}) map[string]interface{} {
	result := make(map[string]interface{})

	for key, value := range data {
		// Check if this item has the matching stamp
		if instanceWithStamp, ok := value.(map[string]interface{}); ok {
			// Look for the stamp attribute
			if actualStampValue, exists := instanceWithStamp[stampKey]; exists {
				// Check if stamp values match
				if sm.stampValuesEqual(actualStampValue, stampValue) {
					result[key] = value
				}
			}
		}
	}

	return result
}

// stampValuesEqual compares stamp values for equality
func (sm *StampManager) stampValuesEqual(a, b interface{}) bool {
	switch va := a.(type) {
	case types.Text:
		if vb, ok := b.(types.Text); ok {
			return va.Value == vb.Value
		}
	case types.Number:
		if vb, ok := b.(types.Number); ok {
			return va.Value == vb.Value
		}
	case types.Boolean:
		if vb, ok := b.(types.Boolean); ok {
			return va.Value == vb.Value
		}
	case uint64:
		if vb, ok := b.(uint64); ok {
			return va == vb
		}
	case string:
		if vb, ok := b.(string); ok {
			return va == vb
		}
		if vb, ok := b.(types.Text); ok {
			return va == vb.Value
		}
	}
	return false
}
