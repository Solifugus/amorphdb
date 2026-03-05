package storage

import (
	"encoding/binary"
	"fmt"
	"os"
	"sync"
	"time"
)

// Instance represents a temporal record in the instance chain
type Instance struct {
	ID              uint64 // Instance ID (position in file / record size)
	ValueID         uint64 // Points to value in the value store
	OlderInstanceID uint64 // Previous instance in chain (0 if first)
	Timestamp       int64  // UNIX time in microseconds (UTC)
	AuthorID        uint64 // Value ID of the author identity
}

// InstanceStore manages the storage of instances
type InstanceStore struct {
	file   *os.File
	mu     sync.RWMutex
	nextID uint64
}

// Instance record size in bytes (all fixed-width fields)
const InstanceRecordSize = 32 // 4 * 8 bytes

// NewInstanceStore creates a new instance store at the given file path
func NewInstanceStore(filePath string) (*InstanceStore, error) {
	file, err := os.OpenFile(filePath, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open instance file: %w", err)
	}

	is := &InstanceStore{
		file: file,
	}

	// Determine next available ID by checking file size
	// Instance IDs start from 1 so that 0 can mean "no older instance"
	info, err := file.Stat()
	if err != nil {
		file.Close()
		return nil, fmt.Errorf("failed to stat instance file: %w", err)
	}

	is.nextID = uint64(info.Size())/InstanceRecordSize + 1

	return is, nil
}

// Close closes the instance store
func (is *InstanceStore) Close() error {
	is.mu.Lock()
	defer is.mu.Unlock()

	if is.file != nil {
		return is.file.Close()
	}
	return nil
}

// WriteInstance writes a new instance and returns its ID
func (is *InstanceStore) WriteInstance(valueID uint64, olderInstanceID uint64, authorID uint64) (uint64, error) {
	is.mu.Lock()
	defer is.mu.Unlock()

	// Generate timestamp in microseconds UTC
	timestamp := time.Now().UTC().UnixMicro()

	instance := Instance{
		ID:              is.nextID,
		ValueID:         valueID,
		OlderInstanceID: olderInstanceID,
		Timestamp:       timestamp,
		AuthorID:        authorID,
	}

	// Serialize instance
	data := is.serializeInstance(instance)

	// Seek to end of file
	_, err := is.file.Seek(0, 2)
	if err != nil {
		return 0, fmt.Errorf("seek to end failed: %w", err)
	}

	// Write instance data
	_, err = is.file.Write(data)
	if err != nil {
		return 0, fmt.Errorf("write instance failed: %w", err)
	}

	// Sync to ensure data is written
	err = is.file.Sync()
	if err != nil {
		return 0, fmt.Errorf("sync failed: %w", err)
	}

	instanceID := is.nextID
	is.nextID++

	return instanceID, nil
}

// ReadInstance reads an instance by its ID
func (is *InstanceStore) ReadInstance(instanceID uint64) (Instance, error) {
	is.mu.RLock()
	defer is.mu.RUnlock()

	if instanceID == 0 {
		return Instance{}, fmt.Errorf("invalid instance ID 0")
	}

	// Calculate file offset (instance IDs start from 1)
	offset := int64((instanceID - 1) * InstanceRecordSize)

	// Seek to instance position
	_, err := is.file.Seek(offset, 0)
	if err != nil {
		return Instance{}, fmt.Errorf("seek failed: %w", err)
	}

	// Read instance data
	data := make([]byte, InstanceRecordSize)
	n, err := is.file.Read(data)
	if err != nil {
		return Instance{}, fmt.Errorf("read instance failed: %w", err)
	}

	if n != InstanceRecordSize {
		return Instance{}, fmt.Errorf("incomplete read: got %d bytes, expected %d", n, InstanceRecordSize)
	}

	return is.deserializeInstance(data, instanceID), nil
}

// GetInstanceChain returns all instances in a chain, from newest to oldest
func (is *InstanceStore) GetInstanceChain(newestInstanceID uint64) ([]Instance, error) {
	if newestInstanceID == 0 {
		return []Instance{}, nil
	}

	var chain []Instance
	currentID := newestInstanceID

	for currentID != 0 {
		instance, err := is.ReadInstance(currentID)
		if err != nil {
			return nil, fmt.Errorf("failed to read instance %d: %w", currentID, err)
		}

		chain = append(chain, instance)
		currentID = instance.OlderInstanceID
	}

	return chain, nil
}

// ValidateChainOrder verifies that timestamps are monotonically increasing when walking backwards
func (is *InstanceStore) ValidateChainOrder(chain []Instance) error {
	for i := 0; i < len(chain)-1; i++ {
		newer := chain[i]
		older := chain[i+1]

		if newer.Timestamp <= older.Timestamp {
			return fmt.Errorf("timestamp order violation: instance %d (ts=%d) should be newer than instance %d (ts=%d)",
				newer.ID, newer.Timestamp, older.ID, older.Timestamp)
		}
	}
	return nil
}

// serializeInstance converts an instance to its binary representation
func (is *InstanceStore) serializeInstance(instance Instance) []byte {
	data := make([]byte, InstanceRecordSize)

	// All fields are uint64, use little-endian encoding
	binary.LittleEndian.PutUint64(data[0:8], instance.ValueID)
	binary.LittleEndian.PutUint64(data[8:16], instance.OlderInstanceID)
	binary.LittleEndian.PutUint64(data[16:24], uint64(instance.Timestamp))
	binary.LittleEndian.PutUint64(data[24:32], instance.AuthorID)

	return data
}

// deserializeInstance converts binary data back to an instance
func (is *InstanceStore) deserializeInstance(data []byte, instanceID uint64) Instance {
	return Instance{
		ID:              instanceID,
		ValueID:         binary.LittleEndian.Uint64(data[0:8]),
		OlderInstanceID: binary.LittleEndian.Uint64(data[8:16]),
		Timestamp:       int64(binary.LittleEndian.Uint64(data[16:24])),
		AuthorID:        binary.LittleEndian.Uint64(data[24:32]),
	}
}

// FindInstanceAtTime finds the instance that was active at the given timestamp
// by walking the instance chain backwards in time
func (is *InstanceStore) FindInstanceAtTime(firstInstanceID uint64, timestamp int64) (uint64, error) {
	if firstInstanceID == 0 {
		return 0, nil
	}

	currentInstanceID := firstInstanceID

	for currentInstanceID != 0 {
		instance, err := is.ReadInstance(currentInstanceID)
		if err != nil {
			return 0, fmt.Errorf("failed to read instance %d: %w", currentInstanceID, err)
		}

		// If this instance's timestamp is <= our target timestamp, it's the one we want
		if instance.Timestamp <= timestamp {
			return currentInstanceID, nil
		}

		// Move to the older instance
		currentInstanceID = instance.OlderInstanceID
	}

	// No instance found at or before the target timestamp
	return 0, nil
}

// PurgeInstancesInTimeRange tombstones instances in the given time range
// Returns the number of instances purged
func (is *InstanceStore) PurgeInstancesInTimeRange(firstInstanceID uint64, from int64, to int64, author uint64) (int, error) {
	if firstInstanceID == 0 {
		return 0, nil
	}

	purgedCount := 0
	currentInstanceID := firstInstanceID

	for currentInstanceID != 0 {
		instance, err := is.ReadInstance(currentInstanceID)
		if err != nil {
			return purgedCount, fmt.Errorf("failed to read instance %d: %w", currentInstanceID, err)
		}

		// Check if this instance falls within the purge range
		if instance.Timestamp >= from && instance.Timestamp <= to {
			// Tombstone this instance by setting its author to 0
			err = is.TombstoneInstance(currentInstanceID, author)
			if err != nil {
				return purgedCount, fmt.Errorf("failed to tombstone instance %d: %w", currentInstanceID, err)
			}
			purgedCount++
		}

		// Move to the older instance
		currentInstanceID = instance.OlderInstanceID
	}

	return purgedCount, nil
}

// TombstoneInstance marks an instance as deleted by setting a tombstone marker
func (is *InstanceStore) TombstoneInstance(instanceID uint64, deletedBy uint64) error {
	is.mu.Lock()
	defer is.mu.Unlock()

	if instanceID == 0 {
		return fmt.Errorf("invalid instance ID 0")
	}

	// Calculate file offset for the AuthorID field (we'll use this to mark as tombstone)
	// Layout: ValueID (8) + AuthorID (8) + Timestamp (8) + OlderInstanceID (8)
	// We'll set AuthorID to a special tombstone value (max uint64)
	offset := int64((instanceID-1)*InstanceRecordSize + 8)

	// Seek to AuthorID position
	_, err := is.file.Seek(offset, 0)
	if err != nil {
		return fmt.Errorf("seek failed: %w", err)
	}

	// Write the tombstone marker (use max uint64 as tombstone indicator)
	tombstoneBytes := make([]byte, 8)
	binary.LittleEndian.PutUint64(tombstoneBytes, uint64(0xFFFFFFFFFFFFFFFF)) // Tombstone marker

	_, err = is.file.Write(tombstoneBytes)
	if err != nil {
		return fmt.Errorf("write tombstone failed: %w", err)
	}

	// Sync to ensure data is written
	err = is.file.Sync()
	if err != nil {
		return fmt.Errorf("sync failed: %w", err)
	}

	return nil
}

// GetInstanceCount returns the number of instances in the store
func (is *InstanceStore) GetInstanceCount() uint64 {
	is.mu.RLock()
	defer is.mu.RUnlock()
	// Since IDs start from 1, count is nextID - 1
	return is.nextID - 1
}