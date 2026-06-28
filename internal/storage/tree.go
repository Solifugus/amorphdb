package storage

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"
)

// Tree interface provides a high-level API for tree operations combining
// attributes, instances, and values
type Tree interface {
	Read(path []string) (Value, error)
	Write(path []string, value Value, author uint64) error
	ReadAt(path []string, timestamp int64) (Value, error)
	Children(path []string) ([]Attribute, error)
	Purge(path []string, from int64, to int64, author uint64) error
}

// StorageTree implements the Tree interface using the underlying storage layers
type StorageTree struct {
	valueStore     *ValueStore
	instanceStore  *InstanceStore
	attributeStore *AttributeStore
	hashIndex      *HashIndex
}

// NewStorageTree creates a new storage tree with the given directory for storage files
func NewStorageTree(storageDir string) (*StorageTree, error) {
	// Create storage directory if it doesn't exist
	// Initialize value store
	valueStore, err := NewValueStore(filepath.Join(storageDir, "values.dat"))
	if err != nil {
		return nil, fmt.Errorf("failed to create value store: %w", err)
	}

	// Initialize instance store
	instanceStore, err := NewInstanceStore(filepath.Join(storageDir, "instances.dat"))
	if err != nil {
		valueStore.Close()
		return nil, fmt.Errorf("failed to create instance store: %w", err)
	}

	// Initialize attribute store
	attributeStore, err := NewAttributeStore(filepath.Join(storageDir, "attributes.dat"))
	if err != nil {
		valueStore.Close()
		instanceStore.Close()
		return nil, fmt.Errorf("failed to create attribute store: %w", err)
	}

	// Initialize hash index and rebuild from attribute store
	hashIndex := NewHashIndex(0) // Use default bucket count
	err = hashIndex.RebuildFromAttributeStore(attributeStore)
	if err != nil {
		valueStore.Close()
		instanceStore.Close()
		attributeStore.Close()
		return nil, fmt.Errorf("failed to rebuild hash index: %w", err)
	}

	return &StorageTree{
		valueStore:     valueStore,
		instanceStore:  instanceStore,
		attributeStore: attributeStore,
		hashIndex:      hashIndex,
	}, nil
}

// Close closes all underlying storage components
func (st *StorageTree) Close() error {
	var errs []error

	if err := st.valueStore.Close(); err != nil {
		errs = append(errs, fmt.Errorf("value store close: %w", err))
	}

	if err := st.instanceStore.Close(); err != nil {
		errs = append(errs, fmt.Errorf("instance store close: %w", err))
	}

	if err := st.attributeStore.Close(); err != nil {
		errs = append(errs, fmt.Errorf("attribute store close: %w", err))
	}

	if len(errs) > 0 {
		return fmt.Errorf("close errors: %v", errs)
	}

	return nil
}

// Read reads the current value at the given path
func (st *StorageTree) Read(path []string) (Value, error) {
	if len(path) == 0 {
		return Value{}, fmt.Errorf("empty path")
	}

	// Navigate to the attribute for this path
	attributeID, err := st.findAttributeForPath(path)
	if err != nil {
		return Value{}, fmt.Errorf("failed to find attribute for path: %w", err)
	}

	if attributeID == 0 {
		return Value{}, fmt.Errorf("path not found: %v", path)
	}

	// Get the attribute to find the first (most recent) instance
	attribute, err := st.attributeStore.ReadAttribute(attributeID)
	if err != nil {
		return Value{}, fmt.Errorf("failed to read attribute: %w", err)
	}

	if attribute.FirstInstanceID == 0 {
		return Value{}, fmt.Errorf("no instances for path: %v", path)
	}

	// Get the most recent instance
	instance, err := st.instanceStore.ReadInstance(attribute.FirstInstanceID)
	if err != nil {
		return Value{}, fmt.Errorf("failed to read instance: %w", err)
	}

	// Read the value
	value, err := st.valueStore.ReadValue(instance.ValueID)
	if err != nil {
		return Value{}, fmt.Errorf("failed to read value: %w", err)
	}

	return value, nil
}

// Write writes a value at the given path, creating a new instance
func (st *StorageTree) Write(path []string, value Value, author uint64) error {
	if len(path) == 0 {
		return fmt.Errorf("empty path")
	}

	// Store the value first to get its ID
	valueID, err := st.valueStore.WriteValue(value)
	if err != nil {
		return fmt.Errorf("failed to write value: %w", err)
	}

	// Find or create the attribute chain for this path
	attributeID, err := st.findOrCreateAttributeForPath(path)
	if err != nil {
		return fmt.Errorf("failed to find or create attribute for path: %w", err)
	}

	// Get the current attribute to find the previous instance
	attribute, err := st.attributeStore.ReadAttribute(attributeID)
	if err != nil {
		return fmt.Errorf("failed to read attribute: %w", err)
	}

	// Check if the value is the same as the current one (no new instance needed)
	if attribute.FirstInstanceID != 0 {
		currentInstance, err := st.instanceStore.ReadInstance(attribute.FirstInstanceID)
		if err != nil {
			return fmt.Errorf("failed to read current instance: %w", err)
		}

		if currentInstance.ValueID == valueID {
			// Same value, no new instance needed
			return nil
		}
	}

	// Create a new instance
	instanceID, err := st.instanceStore.WriteInstance(valueID, attribute.FirstInstanceID, author)
	if err != nil {
		return fmt.Errorf("failed to write instance: %w", err)
	}

	// Update the attribute to point to the new instance
	err = st.attributeStore.UpdateAttributeFirstInstance(attributeID, instanceID)
	if err != nil {
		return fmt.Errorf("failed to update attribute first instance: %w", err)
	}

	return nil
}

// ReadAt reads the value at the given path at a specific timestamp
func (st *StorageTree) ReadAt(path []string, timestamp int64) (Value, error) {
	if len(path) == 0 {
		return Value{}, fmt.Errorf("empty path")
	}

	// Navigate to the attribute for this path
	attributeID, err := st.findAttributeForPath(path)
	if err != nil {
		return Value{}, fmt.Errorf("failed to find attribute for path: %w", err)
	}

	if attributeID == 0 {
		return Value{}, fmt.Errorf("path not found: %v", path)
	}

	// Get the attribute to find the first instance
	attribute, err := st.attributeStore.ReadAttribute(attributeID)
	if err != nil {
		return Value{}, fmt.Errorf("failed to read attribute: %w", err)
	}

	if attribute.FirstInstanceID == 0 {
		return Value{}, fmt.Errorf("no instances for path: %v", path)
	}

	// Walk through the instance chain to find the one that was active at the given timestamp
	instanceID, err := st.instanceStore.FindInstanceAtTime(attribute.FirstInstanceID, timestamp)
	if err != nil {
		return Value{}, fmt.Errorf("failed to find instance at time: %w", err)
	}

	if instanceID == 0 {
		return Value{}, fmt.Errorf("no instance found at timestamp %d for path %v", timestamp, path)
	}

	// Get the instance
	instance, err := st.instanceStore.ReadInstance(instanceID)
	if err != nil {
		return Value{}, fmt.Errorf("failed to read instance: %w", err)
	}

	// Read the value
	value, err := st.valueStore.ReadValue(instance.ValueID)
	if err != nil {
		return Value{}, fmt.Errorf("failed to read value: %w", err)
	}

	return value, nil
}

// Children returns all child attributes at the given path
func (st *StorageTree) Children(path []string) ([]Attribute, error) {
	// If path is empty, return root attributes
	if len(path) == 0 {
		return st.getRootAttributes()
	}

	// Build the parent path
	parentPath := ""
	for i, component := range path {
		if i > 0 {
			parentPath += "."
		}
		parentPath += component
	}

	// Search for child attributes by finding all paths that start with parentPath + "."
	// We don't require the parent path to exist as an attribute itself
	return st.findChildrenByPrefix(parentPath)
}

// Purge tombstones instances in a time range and creates audit records
func (st *StorageTree) Purge(path []string, from int64, to int64, author uint64) error {
	if len(path) == 0 {
		return fmt.Errorf("empty path")
	}

	// Find the attribute for this path
	attributeID, err := st.findAttributeForPath(path)
	if err != nil {
		return fmt.Errorf("failed to find attribute for path: %w", err)
	}

	if attributeID == 0 {
		return fmt.Errorf("path not found: %v", path)
	}

	// Get the attribute
	attribute, err := st.attributeStore.ReadAttribute(attributeID)
	if err != nil {
		return fmt.Errorf("failed to read attribute: %w", err)
	}

	if attribute.FirstInstanceID == 0 {
		return nil // No instances to purge
	}

	// Purge instances in the time range
	purgedCount, err := st.instanceStore.PurgeInstancesInTimeRange(attribute.FirstInstanceID, from, to, author)
	if err != nil {
		return fmt.Errorf("failed to purge instances: %w", err)
	}

	// Create audit record for the purge operation
	auditValue := Value{
		Data:    []byte(fmt.Sprintf("Purged %d instances from %d to %d by author %d", purgedCount, from, to, author)),
		TypeTag: TypeText,
	}

	auditPath := append(path, "_audit", fmt.Sprintf("purge_%d", time.Now().Unix()))
	err = st.Write(auditPath, auditValue, author)
	if err != nil {
		return fmt.Errorf("failed to create audit record: %w", err)
	}

	return nil
}

// findChildrenByPrefix finds child attributes by searching for paths with the given prefix
func (st *StorageTree) findChildrenByPrefix(parentPath string) ([]Attribute, error) {
	var children []Attribute
	childPrefix := parentPath + "."

	// We need to search through attributes to find ones with paths starting with our prefix
	// This is not efficient but works for the current composite path implementation

	// Get the maximum attribute ID to know how many to check
	maxAttributeID := st.attributeStore.GetAttributeCount()

	seenChildren := make(map[string]bool) // To avoid duplicates

	for attributeID := uint64(1); attributeID <= uint64(maxAttributeID); attributeID++ {
		attribute, err := st.attributeStore.ReadAttribute(attributeID)
		if err != nil {
			continue // Skip invalid attributes
		}

		// Get the path string for this attribute
		pathValue, err := st.valueStore.ReadValue(attribute.LabelValueID)
		if err != nil {
			continue // Skip if we can't read the path
		}

		pathStr := string(pathValue.Data)

		// Check if this path starts with our child prefix
		if strings.HasPrefix(pathStr, childPrefix) {
			// Extract the immediate child name (the part after the prefix, before the next dot)
			remainder := pathStr[len(childPrefix):]

			// Find the first dot in the remainder to get just the immediate child
			var childName string
			if dotIndex := strings.Index(remainder, "."); dotIndex != -1 {
				childName = remainder[:dotIndex]
			} else {
				childName = remainder
			}

			// Skip if we've already seen this child
			if seenChildren[childName] {
				continue
			}
			seenChildren[childName] = true

			// Create a pseudo-attribute for this child
			// In a proper implementation, we'd have separate attributes for each level
			childPathValue := Value{
				TypeTag: TypeText,
				Data:    []byte(childName),
			}
			childValueID, err := st.valueStore.WriteValue(childPathValue)
			if err != nil {
				continue // Skip on error
			}

			childAttribute := Attribute{
				ID:              attributeID, // Use the original attribute ID for now
				LabelValueID:    childValueID,
				FirstInstanceID: attribute.FirstInstanceID,
				NextAttributeID: 0,
			}

			children = append(children, childAttribute)
		}
	}

	return children, nil
}

// findAttributeForPath navigates the attribute hierarchy to find the attribute for a given path
func (st *StorageTree) findAttributeForPath(path []string) (uint64, error) {
	if len(path) == 0 {
		return 0, nil
	}

	// For simplicity in this implementation, always use composite path approach
	// which matches what findOrCreateAttributeForPath does
	compositePath := ""
	for i, component := range path {
		if i > 0 {
			compositePath += "."
		}
		compositePath += component
	}

	// Find value ID for the composite path
	compositeValueID, err := st.findValueIDForString(compositePath)
	if err != nil || compositeValueID == 0 {
		return 0, nil
	}

	// Look up in hash index
	attributeID, found := st.hashIndex.Lookup(compositeValueID)
	if !found {
		return 0, nil
	}

	return attributeID, nil
}

// findOrCreateAttributeForPath finds or creates the attribute chain for a given path
func (st *StorageTree) findOrCreateAttributeForPath(path []string) (uint64, error) {
	if len(path) == 0 {
		return 0, fmt.Errorf("empty path")
	}

	// For simplicity, create a composite path as a single attribute
	compositePath := ""
	for i, component := range path {
		if i > 0 {
			compositePath += "."
		}
		compositePath += component
	}

	// Store the path as a value
	pathValue := Value{
		Data:    []byte(compositePath),
		TypeTag: TypeText,
	}

	pathValueID, err := st.valueStore.WriteValue(pathValue)
	if err != nil {
		return 0, fmt.Errorf("failed to write path value: %w", err)
	}

	// Check if attribute already exists
	attributeID, found := st.hashIndex.Lookup(pathValueID)
	if found {
		return attributeID, nil
	}

	// Create new attribute
	attributeID, err = st.attributeStore.WriteAttribute(pathValueID, 0, 0)
	if err != nil {
		return 0, fmt.Errorf("failed to write attribute: %w", err)
	}

	// Add to hash index
	st.hashIndex.Insert(pathValueID, attributeID)

	return attributeID, nil
}

// findValueIDForString finds the value ID for a string, returns 0 if not found
func (st *StorageTree) findValueIDForString(str string) (uint64, error) {
	// Look for existing value without creating a new one
	valueID := st.valueStore.FindValueIDForData([]byte(str), TypeText)
	if valueID != 0 {
		return valueID, nil
	}

	// If not found in hash map (which might be empty after restart),
	// fall back to WriteValue which will properly deduplicate
	tempValue := Value{
		Data:    []byte(str),
		TypeTag: TypeText,
	}

	valueID, err := st.valueStore.WriteValue(tempValue)
	if err != nil {
		return 0, fmt.Errorf("failed to find/create value ID for string: %w", err)
	}

	return valueID, nil
}

// getRootAttributes returns all root level attributes
func (st *StorageTree) getRootAttributes() ([]Attribute, error) {
	// For now, return all attributes as roots
	// In a full implementation, we'd distinguish between root and child attributes
	var rootAttributes []Attribute

	attributeCount := st.attributeStore.GetAttributeCount()
	for id := uint64(1); id <= attributeCount; id++ {
		attribute, err := st.attributeStore.ReadAttribute(id)
		if err != nil {
			continue // Skip invalid attributes
		}
		rootAttributes = append(rootAttributes, attribute)
	}

	return rootAttributes, nil
}

// ResolveAttributePath resolves an attribute ID to its path string for extract operations
func (st *StorageTree) ResolveAttributePath(attributeID uint64) (string, error) {
	if attributeID == 0 {
		return "", fmt.Errorf("invalid attribute ID 0")
	}

	// Read the attribute
	attribute, err := st.attributeStore.ReadAttribute(attributeID)
	if err != nil {
		return "", fmt.Errorf("failed to read attribute %d: %w", attributeID, err)
	}

	// Read the path value from the label
	pathValue, err := st.valueStore.ReadValue(attribute.LabelValueID)
	if err != nil {
		return "", fmt.Errorf("failed to read path value for attribute %d: %w", attributeID, err)
	}

	// The path is stored as a text value
	if pathValue.TypeTag != TypeText {
		return "", fmt.Errorf("attribute %d has non-text label (type 0x%02x)", attributeID, pathValue.TypeTag)
	}

	pathStr := string(pathValue.Data)
	return pathStr, nil
}