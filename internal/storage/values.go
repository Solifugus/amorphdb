package storage

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// Type tags for MBL data types
const (
	TypeText      = 0x01
	TypeNumber    = 0x02
	TypeTime      = 0x03
	TypeMoney     = 0x04
	TypePicture   = 0x05
	TypeReference = 0x06
	TypeProcedure = 0x07
	TypeWatcher   = 0x08
	TypeEmbed     = 0x09
	TypeNothing   = 0xF0
	TypeUnknown   = 0xF1
	TypeAnything  = 0xF2
)

// Sentinels for value records
const (
	SentinelValue    = 0x1E // Normal value record
	SentinelTombstone = 0x1F // Deleted/tombstoned value
)

// Bucket tier boundaries in bytes
const (
	Tier0MaxSize = 64    // Fixed-size: numbers, time, money, references
	Tier1MaxSize = 512   // Small: short text, small procedures
	Tier2MaxSize = 4096  // Medium: longer text, procedures
	// Tier 3: Large (>4096 bytes) — one file, offset-addressed
)

// Bits allocation for value ID encoding
const (
	OffsetBits = 40 // Lower 40 bits for offset within file
	TierBits   = 2  // Next 2 bits for tier (0-3)
	BucketBits = 22 // Upper 22 bits for bucket file number
)

// Masks for extracting components from value ID
const (
	OffsetMask = (1 << OffsetBits) - 1
	TierMask   = ((1 << TierBits) - 1) << OffsetBits
	BucketMask = ((1 << BucketBits) - 1) << (OffsetBits + TierBits)
)

// Value represents a stored data value
type Value struct {
	TypeTag byte
	Data    []byte
}

// ValueStore manages the storage of values in bucketed files
type ValueStore struct {
	basePath string
	mu       sync.RWMutex

	// File handles for each tier
	tier0Files map[uint64]*os.File // Bucket files for tier 0 (≤64 bytes)
	tier1Files map[uint64]*os.File // Bucket files for tier 1 (64-512 bytes)
	tier2Files map[uint64]*os.File // Bucket files for tier 2 (512-4096 bytes)
	tier3File  *os.File            // Single large file for tier 3 (>4096 bytes)

	// Current bucket numbers for each tier
	currentBucket [4]uint64

	// Free lists for reclaiming tombstoned space
	freeLists [4]*FreeList

	// Content hash to value ID mapping for deduplication
	hashToValueID map[[32]byte]uint64

	// Reference counting for proper deduplication cleanup
	referenceCount map[uint64]int
}

// Note: FreeBlock is now defined in freelist.go

// NewValueStore creates a new value store at the given base path
func NewValueStore(basePath string) (*ValueStore, error) {
	// Ensure directory exists
	if err := os.MkdirAll(basePath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create value store directory: %w", err)
	}

	vs := &ValueStore{
		basePath:       basePath,
		tier0Files:     make(map[uint64]*os.File),
		tier1Files:     make(map[uint64]*os.File),
		tier2Files:     make(map[uint64]*os.File),
		hashToValueID:  make(map[[32]byte]uint64),
		referenceCount: make(map[uint64]int),
		freeLists:      [4]*FreeList{NewFreeList(), NewFreeList(), NewFreeList(), NewFreeList()},
	}

	// Open or create the tier 3 large file
	tier3Path := filepath.Join(basePath, "large.values")
	f, err := os.OpenFile(tier3Path, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open tier 3 file: %w", err)
	}
	vs.tier3File = f

	// Rebuild hash map from existing values for proper deduplication across restarts
	err = vs.rebuildHashMap()
	if err != nil {
		f.Close()
		return nil, fmt.Errorf("failed to rebuild hash map: %w", err)
	}

	return vs, nil
}

// Close closes all open file handles
func (vs *ValueStore) Close() error {
	vs.mu.Lock()
	defer vs.mu.Unlock()

	var errs []error

	// Close all tier 0 files
	for _, f := range vs.tier0Files {
		if err := f.Close(); err != nil {
			errs = append(errs, err)
		}
	}

	// Close all tier 1 files
	for _, f := range vs.tier1Files {
		if err := f.Close(); err != nil {
			errs = append(errs, err)
		}
	}

	// Close all tier 2 files
	for _, f := range vs.tier2Files {
		if err := f.Close(); err != nil {
			errs = append(errs, err)
		}
	}

	// Close tier 3 file
	if vs.tier3File != nil {
		if err := vs.tier3File.Close(); err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("errors closing files: %v", errs)
	}

	return nil
}

// WriteValue writes a value and returns its value ID (public API)
func (vs *ValueStore) WriteValue(value Value) (uint64, error) {
	vs.mu.Lock()
	defer vs.mu.Unlock()
	return vs.writeValue(value)
}

// ReadValue reads a value by its value ID (public API)
func (vs *ValueStore) ReadValue(valueID uint64) (Value, error) {
	vs.mu.RLock()
	defer vs.mu.RUnlock()
	return vs.readValue(valueID)
}

// TombstoneValue marks a value as deleted (public API)
func (vs *ValueStore) TombstoneValue(valueID uint64) error {
	vs.mu.Lock()
	defer vs.mu.Unlock()
	return vs.tombstoneValue(valueID)
}

// writeValue writes a value and returns its value ID
func (vs *ValueStore) writeValue(value Value) (uint64, error) {
	// Check for deduplication first
	hash := vs.hashValue(value)
	if valueID, exists := vs.hashToValueID[hash]; exists {
		// Increment reference count for deduplicated value
		vs.referenceCount[valueID]++
		return valueID, nil
	}

	// Determine tier based on serialized size
	serialized := vs.serializeValue(value)
	tier := vs.determineTier(len(serialized))

	var valueID uint64
	var err error

	switch tier {
	case 0:
		valueID, err = vs.writeToTier0(serialized)
	case 1:
		valueID, err = vs.writeToTier1(serialized)
	case 2:
		valueID, err = vs.writeToTier2(serialized)
	case 3:
		valueID, err = vs.writeToTier3(serialized)
	default:
		return 0, fmt.Errorf("invalid tier: %d", tier)
	}

	if err != nil {
		return 0, err
	}

	// Store hash mapping for deduplication and set initial reference count
	vs.hashToValueID[hash] = valueID
	vs.referenceCount[valueID] = 1

	return valueID, nil
}

// readValue reads a value by its value ID
func (vs *ValueStore) readValue(valueID uint64) (Value, error) {
	tier := vs.extractTier(valueID)
	bucket := vs.extractBucket(valueID)
	offset := vs.extractOffset(valueID)

	var data []byte
	var err error

	switch tier {
	case 0:
		data, err = vs.readFromTier0(bucket, offset)
	case 1:
		data, err = vs.readFromTier1(bucket, offset)
	case 2:
		data, err = vs.readFromTier2(bucket, offset)
	case 3:
		data, err = vs.readFromTier3(offset)
	default:
		return Value{}, fmt.Errorf("invalid tier: %d", tier)
	}

	if err != nil {
		return Value{}, err
	}

	return vs.deserializeValue(data)
}

// tombstoneValue decrements reference count and marks value as deleted if count reaches zero
func (vs *ValueStore) tombstoneValue(valueID uint64) error {
	// Check current reference count
	refCount, exists := vs.referenceCount[valueID]
	if !exists {
		return fmt.Errorf("value ID %d not found in reference count", valueID)
	}

	// Decrement reference count
	refCount--

	// If still has references, just update the count and return
	if refCount > 0 {
		vs.referenceCount[valueID] = refCount
		return nil
	}

	// No more references, actually tombstone the value
	delete(vs.referenceCount, valueID)

	// Find and remove the hash mapping
	// We need to read the value to compute its hash for removal
	value, err := vs.readValue(valueID)
	if err != nil {
		// Value might already be tombstoned, try to tombstone anyway
	} else {
		hash := vs.hashValue(value)
		delete(vs.hashToValueID, hash)
	}

	tier := vs.extractTier(valueID)
	bucket := vs.extractBucket(valueID)
	offset := vs.extractOffset(valueID)

	switch tier {
	case 0:
		return vs.tombstoneTier0(bucket, offset)
	case 1:
		return vs.tombstoneTier1(bucket, offset)
	case 2:
		return vs.tombstoneTier2(bucket, offset)
	case 3:
		return vs.tombstoneTier3(offset)
	default:
		return fmt.Errorf("invalid tier: %d", tier)
	}
}

// serializeValue serializes a value into the wire format
func (vs *ValueStore) serializeValue(value Value) []byte {
	var buf bytes.Buffer

	// Write sentinel
	buf.WriteByte(SentinelValue)

	// Write type tag
	buf.WriteByte(value.TypeTag)

	// Write length for variable-length types
	if vs.isVariableLength(value.TypeTag) {
		lengthBytes := make([]byte, 4)
		binary.LittleEndian.PutUint32(lengthBytes, uint32(len(value.Data)))
		buf.Write(lengthBytes)
	}

	// Write data
	buf.Write(value.Data)

	return buf.Bytes()
}

// deserializeValue deserializes a value from the wire format
func (vs *ValueStore) deserializeValue(data []byte) (Value, error) {
	if len(data) < 2 {
		return Value{}, errors.New("data too short")
	}

	// Check sentinel
	if data[0] == SentinelTombstone {
		return Value{}, errors.New("value is tombstoned")
	}

	if data[0] != SentinelValue {
		return Value{}, fmt.Errorf("invalid sentinel: %x", data[0])
	}

	typeTag := data[1]
	offset := 2

	var valueData []byte

	if vs.isVariableLength(typeTag) {
		// Read length
		if len(data) < offset+4 {
			return Value{}, errors.New("missing length field")
		}
		length := binary.LittleEndian.Uint32(data[offset : offset+4])
		offset += 4

		// Read data
		if len(data) < offset+int(length) {
			return Value{}, errors.New("incomplete data")
		}
		valueData = data[offset : offset+int(length)]
	} else {
		// Fixed-length type, read expected size
		expectedSize := vs.getFixedTypeSize(typeTag)
		if len(data) < offset+expectedSize {
			return Value{}, fmt.Errorf("incomplete fixed-length data for type %x: need %d bytes, have %d", typeTag, expectedSize, len(data)-offset)
		}
		valueData = data[offset : offset+expectedSize]
	}

	return Value{
		TypeTag: typeTag,
		Data:    valueData,
	}, nil
}

// hashValue computes a hash of the value for deduplication
func (vs *ValueStore) hashValue(value Value) [32]byte {
	h := sha256.New()
	h.Write([]byte{value.TypeTag})
	h.Write(value.Data)
	return sha256.Sum256(h.Sum(nil))
}

// determineTier determines which tier a value should be stored in based on size
func (vs *ValueStore) determineTier(size int) int {
	if size <= Tier0MaxSize {
		return 0
	} else if size <= Tier1MaxSize {
		return 1
	} else if size <= Tier2MaxSize {
		return 2
	} else {
		return 3
	}
}

// isVariableLength returns true if the type tag represents a variable-length type
func (vs *ValueStore) isVariableLength(typeTag byte) bool {
	switch typeTag {
	case TypeText, TypeMoney, TypePicture, TypeReference, TypeProcedure, TypeWatcher, TypeEmbed, TypeUnknown:
		return true
	case TypeNumber, TypeTime, TypeNothing, TypeAnything:
		return false
	default:
		// Default to variable length for safety
		return true
	}
}

// encodeValueID creates a value ID from tier, bucket, and offset
func (vs *ValueStore) encodeValueID(tier int, bucket uint64, offset uint64) uint64 {
	return offset | (uint64(tier)<<OffsetBits) | (bucket<<(OffsetBits+TierBits))
}

// extractTier extracts the tier from a value ID
func (vs *ValueStore) extractTier(valueID uint64) int {
	return int((valueID & TierMask) >> OffsetBits)
}

// extractBucket extracts the bucket number from a value ID
func (vs *ValueStore) extractBucket(valueID uint64) uint64 {
	return (valueID & BucketMask) >> (OffsetBits + TierBits)
}

// extractOffset extracts the offset from a value ID
func (vs *ValueStore) extractOffset(valueID uint64) uint64 {
	return valueID & OffsetMask
}

// writeToTier3 writes data to the large file (tier 3)
func (vs *ValueStore) writeToTier3(data []byte) (uint64, error) {
	// Check free list first
	if offset, found := vs.freeLists[3].AllocateSpace(uint64(len(data))); found {
		// Write to the reused position
		_, err := vs.tier3File.Seek(int64(offset), 0)
		if err != nil {
			return 0, fmt.Errorf("seek failed: %w", err)
		}

		_, err = vs.tier3File.Write(data)
		if err != nil {
			return 0, fmt.Errorf("write failed: %w", err)
		}

		return vs.encodeValueID(3, 0, offset), nil
	}

	// No suitable free block, append to end
	info, err := vs.tier3File.Stat()
	if err != nil {
		return 0, fmt.Errorf("stat failed: %w", err)
	}

	offset := uint64(info.Size())

	_, err = vs.tier3File.Seek(0, 2) // Seek to end
	if err != nil {
		return 0, fmt.Errorf("seek to end failed: %w", err)
	}

	_, err = vs.tier3File.Write(data)
	if err != nil {
		return 0, fmt.Errorf("write failed: %w", err)
	}

	return vs.encodeValueID(3, 0, offset), nil
}

// readFromTier3 reads data from the large file (tier 3)
func (vs *ValueStore) readFromTier3(offset uint64) ([]byte, error) {
	_, err := vs.tier3File.Seek(int64(offset), 0)
	if err != nil {
		return nil, fmt.Errorf("seek failed: %w", err)
	}

	// Read sentinel and type tag first
	header := make([]byte, 2)
	_, err = vs.tier3File.Read(header)
	if err != nil {
		return nil, fmt.Errorf("read header failed: %w", err)
	}

	if header[0] == SentinelTombstone {
		return nil, errors.New("value is tombstoned")
	}

	if header[0] != SentinelValue {
		return nil, fmt.Errorf("invalid sentinel: %x", header[0])
	}

	typeTag := header[1]

	var dataSize int

	if vs.isVariableLength(typeTag) {
		// Read length field
		lengthBytes := make([]byte, 4)
		_, err = vs.tier3File.Read(lengthBytes)
		if err != nil {
			return nil, fmt.Errorf("read length failed: %w", err)
		}

		dataSize = int(binary.LittleEndian.Uint32(lengthBytes))

		// Total size is header + length + data
		totalSize := 2 + 4 + dataSize

		// Reset and read the complete record
		_, err = vs.tier3File.Seek(int64(offset), 0)
		if err != nil {
			return nil, fmt.Errorf("seek failed: %w", err)
		}

		result := make([]byte, totalSize)
		_, err = vs.tier3File.Read(result)
		if err != nil {
			return nil, fmt.Errorf("read complete record failed: %w", err)
		}

		return result, nil
	} else {
		// Fixed-length type, need to determine size based on type
		dataSize = vs.getFixedTypeSize(typeTag)
		totalSize := 2 + dataSize

		// Reset and read the complete record
		_, err = vs.tier3File.Seek(int64(offset), 0)
		if err != nil {
			return nil, fmt.Errorf("seek failed: %w", err)
		}

		result := make([]byte, totalSize)
		_, err = vs.tier3File.Read(result)
		if err != nil {
			return nil, fmt.Errorf("read complete record failed: %w", err)
		}

		return result, nil
	}
}

// tombstoneTier3 marks a value as deleted in tier 3
func (vs *ValueStore) tombstoneTier3(offset uint64) error {
	// Read the current record to determine its size
	data, err := vs.readFromTier3(offset)
	if err != nil {
		return fmt.Errorf("read before tombstone failed: %w", err)
	}

	// Seek to the beginning and overwrite the sentinel
	_, err = vs.tier3File.Seek(int64(offset), 0)
	if err != nil {
		return fmt.Errorf("seek failed: %w", err)
	}

	_, err = vs.tier3File.Write([]byte{SentinelTombstone})
	if err != nil {
		return fmt.Errorf("write tombstone failed: %w", err)
	}

	// Add to free list
	vs.freeLists[3].AddBlock(offset, uint64(len(data)))

	return nil
}

// getFixedTypeSize returns the data size for fixed-length types
func (vs *ValueStore) getFixedTypeSize(typeTag byte) int {
	switch typeTag {
	case TypeNumber:
		return 8 // float64
	case TypeTime:
		return 8 // int64 timestamp
	case TypeNothing, TypeAnything:
		return 0 // No data
	default:
		// This should never be called for variable-length types
		// Money, Reference, Watcher are all variable-length
		return 0
	}
}

// Bucket-based tier operations (0, 1, 2) - implementing a simpler version first
func (vs *ValueStore) writeToTier0(data []byte) (uint64, error) {
	return vs.writeToBucketTier(0, data)
}

func (vs *ValueStore) writeToTier1(data []byte) (uint64, error) {
	return vs.writeToBucketTier(1, data)
}

func (vs *ValueStore) writeToTier2(data []byte) (uint64, error) {
	return vs.writeToBucketTier(2, data)
}

func (vs *ValueStore) readFromTier0(bucket uint64, offset uint64) ([]byte, error) {
	return vs.readFromBucketTier(0, bucket, offset)
}

func (vs *ValueStore) readFromTier1(bucket uint64, offset uint64) ([]byte, error) {
	return vs.readFromBucketTier(1, bucket, offset)
}

func (vs *ValueStore) readFromTier2(bucket uint64, offset uint64) ([]byte, error) {
	return vs.readFromBucketTier(2, bucket, offset)
}

func (vs *ValueStore) tombstoneTier0(bucket uint64, offset uint64) error {
	return vs.tombstoneBucketTier(0, bucket, offset)
}

func (vs *ValueStore) tombstoneTier1(bucket uint64, offset uint64) error {
	return vs.tombstoneBucketTier(1, bucket, offset)
}

func (vs *ValueStore) tombstoneTier2(bucket uint64, offset uint64) error {
	return vs.tombstoneBucketTier(2, bucket, offset)
}

// writeToBucketTier writes data to a bucket-based tier
func (vs *ValueStore) writeToBucketTier(tier int, data []byte) (uint64, error) {
	bucket := vs.currentBucket[tier]

	// Get or create file for this bucket
	file, err := vs.getBucketFile(tier, bucket)
	if err != nil {
		return 0, fmt.Errorf("get bucket file failed: %w", err)
	}

	// Check free list first
	if offset, found := vs.freeLists[tier].AllocateSpace(uint64(len(data))); found {
		// Write to the reused position
		_, err := file.Seek(int64(offset), 0)
		if err != nil {
			return 0, fmt.Errorf("seek failed: %w", err)
		}

		_, err = file.Write(data)
		if err != nil {
			return 0, fmt.Errorf("write failed: %w", err)
		}

		return vs.encodeValueID(tier, bucket, offset), nil
	}

	// Append to end of current bucket file
	info, err := file.Stat()
	if err != nil {
		return 0, fmt.Errorf("stat failed: %w", err)
	}

	offset := uint64(info.Size())

	_, err = file.Seek(0, 2) // Seek to end
	if err != nil {
		return 0, fmt.Errorf("seek to end failed: %w", err)
	}

	_, err = file.Write(data)
	if err != nil {
		return 0, fmt.Errorf("write failed: %w", err)
	}

	return vs.encodeValueID(tier, bucket, offset), nil
}

// readFromBucketTier reads data from a bucket-based tier
func (vs *ValueStore) readFromBucketTier(tier int, bucket uint64, offset uint64) ([]byte, error) {
	file, err := vs.getBucketFile(tier, bucket)
	if err != nil {
		return nil, fmt.Errorf("get bucket file failed: %w", err)
	}

	_, err = file.Seek(int64(offset), 0)
	if err != nil {
		return nil, fmt.Errorf("seek failed: %w", err)
	}

	// Read sentinel and type tag first
	header := make([]byte, 2)
	_, err = file.Read(header)
	if err != nil {
		return nil, fmt.Errorf("read header failed: %w", err)
	}

	if header[0] == SentinelTombstone {
		return nil, errors.New("value is tombstoned")
	}

	if header[0] != SentinelValue {
		return nil, fmt.Errorf("invalid sentinel: %x", header[0])
	}

	typeTag := header[1]

	var dataSize int

	if vs.isVariableLength(typeTag) {
		// Read length field
		lengthBytes := make([]byte, 4)
		_, err = file.Read(lengthBytes)
		if err != nil {
			return nil, fmt.Errorf("read length failed: %w", err)
		}

		dataSize = int(binary.LittleEndian.Uint32(lengthBytes))
		totalSize := 2 + 4 + dataSize

		// Reset and read complete record
		_, err = file.Seek(int64(offset), 0)
		if err != nil {
			return nil, fmt.Errorf("seek failed: %w", err)
		}

		result := make([]byte, totalSize)
		_, err = file.Read(result)
		if err != nil {
			return nil, fmt.Errorf("read complete record failed: %w", err)
		}

		return result, nil
	} else {
		dataSize = vs.getFixedTypeSize(typeTag)
		totalSize := 2 + dataSize

		// Reset and read complete record
		_, err = file.Seek(int64(offset), 0)
		if err != nil {
			return nil, fmt.Errorf("seek failed: %w", err)
		}

		result := make([]byte, totalSize)
		_, err = file.Read(result)
		if err != nil {
			return nil, fmt.Errorf("read complete record failed: %w", err)
		}

		return result, nil
	}
}

// tombstoneBucketTier marks a value as deleted in a bucket-based tier
func (vs *ValueStore) tombstoneBucketTier(tier int, bucket uint64, offset uint64) error {
	// Read current record to determine size
	data, err := vs.readFromBucketTier(tier, bucket, offset)
	if err != nil {
		return fmt.Errorf("read before tombstone failed: %w", err)
	}

	file, err := vs.getBucketFile(tier, bucket)
	if err != nil {
		return fmt.Errorf("get bucket file failed: %w", err)
	}

	// Overwrite sentinel
	_, err = file.Seek(int64(offset), 0)
	if err != nil {
		return fmt.Errorf("seek failed: %w", err)
	}

	_, err = file.Write([]byte{SentinelTombstone})
	if err != nil {
		return fmt.Errorf("write tombstone failed: %w", err)
	}

	// Add to free list
	vs.freeLists[tier].AddBlock(offset, uint64(len(data)))

	return nil
}

// getBucketFile gets or creates a bucket file for the given tier and bucket
func (vs *ValueStore) getBucketFile(tier int, bucket uint64) (*os.File, error) {
	var fileMap map[uint64]*os.File
	var tierName string

	switch tier {
	case 0:
		fileMap = vs.tier0Files
		tierName = "tier0"
	case 1:
		fileMap = vs.tier1Files
		tierName = "tier1"
	case 2:
		fileMap = vs.tier2Files
		tierName = "tier2"
	default:
		return nil, fmt.Errorf("invalid tier: %d", tier)
	}

	if file, exists := fileMap[bucket]; exists {
		return file, nil
	}

	// Create new bucket file
	filename := fmt.Sprintf("%s_bucket_%d.values", tierName, bucket)
	path := filepath.Join(vs.basePath, filename)

	file, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to create bucket file: %w", err)
	}

	fileMap[bucket] = file
	return file, nil
}

// FindValueIDForData looks for an existing value with the given data
// Returns 0 if not found, does not create new values
func (vs *ValueStore) FindValueIDForData(data []byte, typeTag byte) uint64 {
	vs.mu.RLock()
	defer vs.mu.RUnlock()

	// Create temporary value for hashing
	tempValue := Value{
		TypeTag: typeTag,
		Data:    data,
	}

	// Check hash map first
	hash := vs.hashValue(tempValue)
	if valueID, exists := vs.hashToValueID[hash]; exists {
		return valueID
	}

	return 0 // Not found
}

// rebuildHashMap rebuilds the deduplication hash map from existing values on disk
func (vs *ValueStore) rebuildHashMap() error {
	// Get the highest value ID by checking what would be the next ID
	// This is a simplified approach - scan through possible value IDs and try to read them

	// Determine the range to scan by checking file sizes
	maxValueID := vs.determineMaxValueID()

	for valueID := uint64(1); valueID <= maxValueID; valueID++ {
		// Try to read this value
		value, err := vs.readValue(valueID)
		if err != nil {
			// Value doesn't exist or is corrupted, skip it
			continue
		}

		// Add to hash map and set initial reference count
		hash := vs.hashValue(value)
		vs.hashToValueID[hash] = valueID
		vs.referenceCount[valueID] = 1 // Start with 1 reference when rebuilding
	}

	return nil
}

// determineMaxValueID determines the maximum value ID by examining file sizes
func (vs *ValueStore) determineMaxValueID() uint64 {
	maxValueID := uint64(0)

	// Check tier 3 file size to estimate max value ID
	if vs.tier3File != nil {
		info, err := vs.tier3File.Stat()
		if err == nil && info.Size() > 0 {
			// Rough estimate: assume average value size of 100 bytes
			estimatedCount := uint64(info.Size()) / 100
			if estimatedCount > maxValueID {
				maxValueID = estimatedCount
			}
		}
	}

	// For a more thorough approach, we'd scan all bucket files in all tiers
	// For now, use a reasonable upper bound
	if maxValueID == 0 {
		maxValueID = 1000 // Default scan range for small installations
	}

	return maxValueID
}