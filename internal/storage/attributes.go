package storage

import (
	"encoding/binary"
	"fmt"
	"os"
	"sync"
)

// Attribute represents a named or numbered entry that points to its first instance
type Attribute struct {
	ID              uint64 // Attribute ID (position in file / record size)
	LabelValueID    uint64 // Points to the attribute name/number in values
	FirstInstanceID uint64 // Points to the most recent instance
	NextAttributeID uint64 // Next sibling attribute (0 if last)
}

// AttributeStore manages the storage of attributes
type AttributeStore struct {
	file   *os.File
	mu     sync.RWMutex
	nextID uint64
}

// Attribute record size in bytes (all fixed-width fields)
const AttributeRecordSize = 24 // 3 * 8 bytes

// NewAttributeStore creates a new attribute store at the given file path
func NewAttributeStore(filePath string) (*AttributeStore, error) {
	file, err := os.OpenFile(filePath, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open attribute file: %w", err)
	}

	as := &AttributeStore{
		file: file,
	}

	// Determine next available ID by checking file size
	// Attribute IDs start from 1 so that 0 can mean "no next attribute"
	info, err := file.Stat()
	if err != nil {
		file.Close()
		return nil, fmt.Errorf("failed to stat attribute file: %w", err)
	}

	as.nextID = uint64(info.Size())/AttributeRecordSize + 1

	return as, nil
}

// Close closes the attribute store
func (as *AttributeStore) Close() error {
	as.mu.Lock()
	defer as.mu.Unlock()

	if as.file != nil {
		return as.file.Close()
	}
	return nil
}

// WriteAttribute writes a new attribute and returns its ID
func (as *AttributeStore) WriteAttribute(labelValueID uint64, firstInstanceID uint64, nextAttributeID uint64) (uint64, error) {
	as.mu.Lock()
	defer as.mu.Unlock()

	attribute := Attribute{
		ID:              as.nextID,
		LabelValueID:    labelValueID,
		FirstInstanceID: firstInstanceID,
		NextAttributeID: nextAttributeID,
	}

	// Serialize attribute
	data := as.serializeAttribute(attribute)

	// Seek to end of file
	_, err := as.file.Seek(0, 2)
	if err != nil {
		return 0, fmt.Errorf("seek to end failed: %w", err)
	}

	// Write attribute data
	_, err = as.file.Write(data)
	if err != nil {
		return 0, fmt.Errorf("write attribute failed: %w", err)
	}

	// Sync to ensure data is written
	err = as.file.Sync()
	if err != nil {
		return 0, fmt.Errorf("sync failed: %w", err)
	}

	attributeID := as.nextID
	as.nextID++

	return attributeID, nil
}

// ReadAttribute reads an attribute by its ID
func (as *AttributeStore) ReadAttribute(attributeID uint64) (Attribute, error) {
	as.mu.RLock()
	defer as.mu.RUnlock()

	if attributeID == 0 {
		return Attribute{}, fmt.Errorf("invalid attribute ID 0")
	}

	// Calculate file offset (attribute IDs start from 1)
	offset := int64((attributeID - 1) * AttributeRecordSize)

	// Seek to attribute position
	_, err := as.file.Seek(offset, 0)
	if err != nil {
		return Attribute{}, fmt.Errorf("seek failed: %w", err)
	}

	// Read attribute data
	data := make([]byte, AttributeRecordSize)
	n, err := as.file.Read(data)
	if err != nil {
		return Attribute{}, fmt.Errorf("read attribute failed: %w", err)
	}

	if n != AttributeRecordSize {
		return Attribute{}, fmt.Errorf("incomplete read: got %d bytes, expected %d", n, AttributeRecordSize)
	}

	return as.deserializeAttribute(data, attributeID), nil
}

// UpdateAttributeNextID updates the NextAttributeID field of an existing attribute
func (as *AttributeStore) UpdateAttributeNextID(attributeID uint64, nextAttributeID uint64) error {
	as.mu.Lock()
	defer as.mu.Unlock()

	if attributeID == 0 {
		return fmt.Errorf("invalid attribute ID 0")
	}

	// Calculate file offset for the NextAttributeID field
	// Layout: LabelValueID (8) + FirstInstanceID (8) + NextAttributeID (8)
	offset := int64((attributeID-1)*AttributeRecordSize + 16)

	// Seek to NextAttributeID position
	_, err := as.file.Seek(offset, 0)
	if err != nil {
		return fmt.Errorf("seek failed: %w", err)
	}

	// Write the new NextAttributeID
	nextIDBytes := make([]byte, 8)
	binary.LittleEndian.PutUint64(nextIDBytes, nextAttributeID)

	_, err = as.file.Write(nextIDBytes)
	if err != nil {
		return fmt.Errorf("write next attribute ID failed: %w", err)
	}

	// Sync to ensure data is written
	err = as.file.Sync()
	if err != nil {
		return fmt.Errorf("sync failed: %w", err)
	}

	return nil
}

// GetSiblingChain returns all sibling attributes starting from the given attribute ID
func (as *AttributeStore) GetSiblingChain(firstAttributeID uint64) ([]Attribute, error) {
	if firstAttributeID == 0 {
		return []Attribute{}, nil
	}

	var siblings []Attribute
	currentID := firstAttributeID

	for currentID != 0 {
		attribute, err := as.ReadAttribute(currentID)
		if err != nil {
			return nil, fmt.Errorf("failed to read attribute %d: %w", currentID, err)
		}

		siblings = append(siblings, attribute)
		currentID = attribute.NextAttributeID
	}

	return siblings, nil
}

// FindAttributeByLabel searches the sibling chain for an attribute with the given label value ID
func (as *AttributeStore) FindAttributeByLabel(firstAttributeID uint64, labelValueID uint64) (*Attribute, error) {
	siblings, err := as.GetSiblingChain(firstAttributeID)
	if err != nil {
		return nil, fmt.Errorf("failed to get sibling chain: %w", err)
	}

	for _, attr := range siblings {
		if attr.LabelValueID == labelValueID {
			return &attr, nil
		}
	}

	return nil, nil // Not found
}

// serializeAttribute converts an attribute to its binary representation
func (as *AttributeStore) serializeAttribute(attribute Attribute) []byte {
	data := make([]byte, AttributeRecordSize)

	// All fields are uint64, use little-endian encoding
	binary.LittleEndian.PutUint64(data[0:8], attribute.LabelValueID)
	binary.LittleEndian.PutUint64(data[8:16], attribute.FirstInstanceID)
	binary.LittleEndian.PutUint64(data[16:24], attribute.NextAttributeID)

	return data
}

// deserializeAttribute converts binary data back to an attribute
func (as *AttributeStore) deserializeAttribute(data []byte, attributeID uint64) Attribute {
	return Attribute{
		ID:              attributeID,
		LabelValueID:    binary.LittleEndian.Uint64(data[0:8]),
		FirstInstanceID: binary.LittleEndian.Uint64(data[8:16]),
		NextAttributeID: binary.LittleEndian.Uint64(data[16:24]),
	}
}

// UpdateAttributeFirstInstance updates the FirstInstanceID field of an existing attribute
func (as *AttributeStore) UpdateAttributeFirstInstance(attributeID uint64, firstInstanceID uint64) error {
	as.mu.Lock()
	defer as.mu.Unlock()

	if attributeID == 0 {
		return fmt.Errorf("invalid attribute ID 0")
	}

	// Calculate file offset for the FirstInstanceID field
	// Layout: LabelValueID (8) + FirstInstanceID (8) + NextAttributeID (8)
	offset := int64((attributeID-1)*AttributeRecordSize + 8)

	// Seek to FirstInstanceID position
	_, err := as.file.Seek(offset, 0)
	if err != nil {
		return fmt.Errorf("seek failed: %w", err)
	}

	// Write the new FirstInstanceID
	firstInstanceBytes := make([]byte, 8)
	binary.LittleEndian.PutUint64(firstInstanceBytes, firstInstanceID)

	_, err = as.file.Write(firstInstanceBytes)
	if err != nil {
		return fmt.Errorf("write first instance ID failed: %w", err)
	}

	// Sync to ensure data is written
	err = as.file.Sync()
	if err != nil {
		return fmt.Errorf("sync failed: %w", err)
	}

	return nil
}

// GetAttributeCount returns the number of attributes in the store
func (as *AttributeStore) GetAttributeCount() uint64 {
	as.mu.RLock()
	defer as.mu.RUnlock()
	// Since IDs start from 1, count is nextID - 1
	return as.nextID - 1
}