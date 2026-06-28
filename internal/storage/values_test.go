package storage

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestNewValueStore(t *testing.T) {
	tempDir := t.TempDir()

	vs, err := NewValueStore(tempDir)
	if err != nil {
		t.Fatalf("NewValueStore failed: %v", err)
	}
	defer vs.Close()

	// Check that the base directory was created
	if _, err := os.Stat(tempDir); os.IsNotExist(err) {
		t.Errorf("Base directory was not created")
	}

	// Check that tier 3 file was created
	tier3Path := filepath.Join(tempDir, "large.values")
	if _, err := os.Stat(tier3Path); os.IsNotExist(err) {
		t.Errorf("Tier 3 file was not created")
	}
}

func TestSerializeDeserialize(t *testing.T) {
	tempDir := t.TempDir()
	vs, err := NewValueStore(tempDir)
	if err != nil {
		t.Fatalf("NewValueStore failed: %v", err)
	}
	defer vs.Close()

	testCases := []struct {
		name  string
		value Value
	}{
		{
			name: "text value",
			value: Value{
				TypeTag: TypeText,
				Data:    []byte("hello world"),
			},
		},
		{
			name: "number value",
			value: Value{
				TypeTag: TypeNumber,
				Data:    []byte{0x40, 0x45, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}, // float64(42.0)
			},
		},
		{
			name: "empty text",
			value: Value{
				TypeTag: TypeText,
				Data:    []byte{},
			},
		},
		{
			name: "nothing value",
			value: Value{
				TypeTag: TypeNothing,
				Data:    []byte{},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Serialize
			serialized := vs.serializeValue(tc.value)

			// Deserialize
			deserialized, err := vs.deserializeValue(serialized)
			if err != nil {
				t.Errorf("deserializeValue failed: %v", err)
				return
			}

			// Compare
			if deserialized.TypeTag != tc.value.TypeTag {
				t.Errorf("TypeTag mismatch: got %x, want %x", deserialized.TypeTag, tc.value.TypeTag)
			}

			if !bytes.Equal(deserialized.Data, tc.value.Data) {
				t.Errorf("Data mismatch: got %v, want %v", deserialized.Data, tc.value.Data)
			}
		})
	}
}

func TestDetermineTier(t *testing.T) {
	vs := &ValueStore{}

	testCases := []struct {
		size         int
		expectedTier int
	}{
		{10, 0},     // Small value -> tier 0
		{64, 0},     // Boundary value -> tier 0
		{65, 1},     // Just over boundary -> tier 1
		{512, 1},    // Tier 1 boundary
		{513, 2},    // Just over tier 1 -> tier 2
		{4096, 2},   // Tier 2 boundary
		{4097, 3},   // Large value -> tier 3
		{100000, 3}, // Very large value -> tier 3
	}

	for _, tc := range testCases {
		tier := vs.determineTier(tc.size)
		if tier != tc.expectedTier {
			t.Errorf("determineTier(%d) = %d, want %d", tc.size, tier, tc.expectedTier)
		}
	}
}

func TestValueIDEncoding(t *testing.T) {
	vs := &ValueStore{}

	testCases := []struct {
		tier   int
		bucket uint64
		offset uint64
	}{
		{0, 0, 0},
		{0, 1, 100},
		{1, 0, 1000},
		{2, 5, 2000},
		{3, 0, 500000},
		{3, 1000, 999999},
	}

	for _, tc := range testCases {
		// Encode
		valueID := vs.encodeValueID(tc.tier, tc.bucket, tc.offset)

		// Decode and verify
		extractedTier := vs.extractTier(valueID)
		extractedBucket := vs.extractBucket(valueID)
		extractedOffset := vs.extractOffset(valueID)

		if extractedTier != tc.tier {
			t.Errorf("Tier mismatch: got %d, want %d", extractedTier, tc.tier)
		}

		if extractedBucket != tc.bucket {
			t.Errorf("Bucket mismatch: got %d, want %d", extractedBucket, tc.bucket)
		}

		if extractedOffset != tc.offset {
			t.Errorf("Offset mismatch: got %d, want %d", extractedOffset, tc.offset)
		}
	}
}

func TestHashValue(t *testing.T) {
	vs := &ValueStore{}

	value1 := Value{TypeTag: TypeText, Data: []byte("hello")}
	value2 := Value{TypeTag: TypeText, Data: []byte("hello")}
	value3 := Value{TypeTag: TypeText, Data: []byte("world")}

	hash1 := vs.hashValue(value1)
	hash2 := vs.hashValue(value2)
	hash3 := vs.hashValue(value3)

	// Same values should have same hash
	if hash1 != hash2 {
		t.Errorf("Same values have different hashes")
	}

	// Different values should have different hashes
	if hash1 == hash3 {
		t.Errorf("Different values have same hash")
	}
}

func TestIsVariableLength(t *testing.T) {
	vs := &ValueStore{}

	testCases := []struct {
		typeTag  byte
		expected bool
	}{
		{TypeText, true},
		{TypeNumber, false},
		{TypeTime, false},
		{TypeMoney, true}, // arbitrary-precision, stored as variable-length data
		{TypePicture, true},
		{TypeReference, true}, // a reference is a path; length varies
		{TypeProcedure, true},
		{TypeWatcher, true}, // a watcher carries procedure source; length varies
		{TypeEmbed, true},
		{TypeNothing, false},
		{TypeUnknown, true},
		{TypeAnything, false},
	}

	for _, tc := range testCases {
		result := vs.isVariableLength(tc.typeTag)
		if result != tc.expected {
			t.Errorf("isVariableLength(%x) = %v, want %v", tc.typeTag, result, tc.expected)
		}
	}
}

func TestDeserializeTombstone(t *testing.T) {
	vs := &ValueStore{}

	// Create tombstoned data
	data := []byte{SentinelTombstone, TypeText, 0x00, 0x00, 0x00, 0x05, 'h', 'e', 'l', 'l', 'o'}

	_, err := vs.deserializeValue(data)
	if err == nil {
		t.Errorf("Expected error for tombstoned value")
	}

	expectedError := "value is tombstoned"
	if err.Error() != expectedError {
		t.Errorf("Got error '%v', want '%v'", err.Error(), expectedError)
	}
}

func TestDeserializeInvalidSentinel(t *testing.T) {
	vs := &ValueStore{}

	// Create data with invalid sentinel
	data := []byte{0xFF, TypeText, 0x00, 0x00, 0x00, 0x05, 'h', 'e', 'l', 'l', 'o'}

	_, err := vs.deserializeValue(data)
	if err == nil {
		t.Errorf("Expected error for invalid sentinel")
	}
}

func TestDeserializeTooShort(t *testing.T) {
	vs := &ValueStore{}

	// Data too short
	data := []byte{SentinelValue}

	_, err := vs.deserializeValue(data)
	if err == nil {
		t.Errorf("Expected error for too-short data")
	}
}

// Test success criteria from development plan

func TestWriteReadTextValue(t *testing.T) {
	tempDir := t.TempDir()
	vs, err := NewValueStore(tempDir)
	if err != nil {
		t.Fatalf("NewValueStore failed: %v", err)
	}
	defer vs.Close()

	// Write a Text value, read it back, confirm exact match
	original := Value{
		TypeTag: TypeText,
		Data:    []byte("Hello, World!"),
	}

	valueID, err := vs.WriteValue(original)
	if err != nil {
		t.Fatalf("WriteValue failed: %v", err)
	}

	read, err := vs.ReadValue(valueID)
	if err != nil {
		t.Fatalf("ReadValue failed: %v", err)
	}

	if read.TypeTag != original.TypeTag {
		t.Errorf("TypeTag mismatch: got %x, want %x", read.TypeTag, original.TypeTag)
	}

	if !bytes.Equal(read.Data, original.Data) {
		t.Errorf("Data mismatch: got %q, want %q", read.Data, original.Data)
	}
}

func TestDeduplication(t *testing.T) {
	tempDir := t.TempDir()
	vs, err := NewValueStore(tempDir)
	if err != nil {
		t.Fatalf("NewValueStore failed: %v", err)
	}
	defer vs.Close()

	// Write same value twice, confirm same value_id returned
	value := Value{
		TypeTag: TypeText,
		Data:    []byte("duplicate test"),
	}

	valueID1, err := vs.WriteValue(value)
	if err != nil {
		t.Fatalf("First WriteValue failed: %v", err)
	}

	valueID2, err := vs.WriteValue(value)
	if err != nil {
		t.Fatalf("Second WriteValue failed: %v", err)
	}

	if valueID1 != valueID2 {
		t.Errorf("Deduplication failed: got different IDs %d and %d", valueID1, valueID2)
	}
}

func TestBucketAssignment(t *testing.T) {
	tempDir := t.TempDir()
	vs, err := NewValueStore(tempDir)
	if err != nil {
		t.Fatalf("NewValueStore failed: %v", err)
	}
	defer vs.Close()

	testCases := []struct {
		name         string
		value        Value
		expectedTier int
	}{
		{
			name: "small value tier 0",
			value: Value{
				TypeTag: TypeNumber,
				Data:    []byte{0x40, 0x45, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}, // 8 bytes
			},
			expectedTier: 0,
		},
		{
			name: "medium value tier 1",
			value: Value{
				TypeTag: TypeText,
				Data:    make([]byte, 100), // ~100 bytes total with header
			},
			expectedTier: 1,
		},
		{
			name: "large value tier 2",
			value: Value{
				TypeTag: TypeText,
				Data:    make([]byte, 1000), // ~1000 bytes total with header
			},
			expectedTier: 2,
		},
		{
			name: "very large value tier 3",
			value: Value{
				TypeTag: TypeText,
				Data:    make([]byte, 5000), // >4096 bytes
			},
			expectedTier: 3,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			valueID, err := vs.WriteValue(tc.value)
			if err != nil {
				t.Fatalf("WriteValue failed: %v", err)
			}

			tier := vs.extractTier(valueID)
			if tier != tc.expectedTier {
				t.Errorf("Expected tier %d, got %d", tc.expectedTier, tier)
			}

			// Verify we can read it back
			read, err := vs.ReadValue(valueID)
			if err != nil {
				t.Fatalf("ReadValue failed: %v", err)
			}

			if !bytes.Equal(read.Data, tc.value.Data) {
				t.Errorf("Data mismatch after read")
			}
		})
	}
}

func TestTombstoneAndFreeList(t *testing.T) {
	tempDir := t.TempDir()
	vs, err := NewValueStore(tempDir)
	if err != nil {
		t.Fatalf("NewValueStore failed: %v", err)
	}
	defer vs.Close()

	// Write a value
	value := Value{
		TypeTag: TypeText,
		Data:    []byte("to be deleted"),
	}

	valueID, err := vs.WriteValue(value)
	if err != nil {
		t.Fatalf("WriteValue failed: %v", err)
	}

	// Verify we can read it
	_, err = vs.ReadValue(valueID)
	if err != nil {
		t.Fatalf("ReadValue failed before tombstone: %v", err)
	}

	// Tombstone the value
	err = vs.TombstoneValue(valueID)
	if err != nil {
		t.Fatalf("TombstoneValue failed: %v", err)
	}

	// Verify reading tombstoned value returns error
	_, err = vs.ReadValue(valueID)
	if err == nil {
		t.Errorf("Expected error when reading tombstoned value")
	}

	// Check that free list has an entry
	tier := vs.extractTier(valueID)
	if len(vs.freeLists[tier].GetBlocks()) == 0 {
		t.Errorf("Free list should have entry after tombstone")
	}
}

func TestFreeListReuse(t *testing.T) {
	tempDir := t.TempDir()
	vs, err := NewValueStore(tempDir)
	if err != nil {
		t.Fatalf("NewValueStore failed: %v", err)
	}
	defer vs.Close()

	// Write a value
	value1 := Value{
		TypeTag: TypeText,
		Data:    []byte("first value"),
	}

	valueID1, err := vs.WriteValue(value1)
	if err != nil {
		t.Fatalf("WriteValue failed: %v", err)
	}

	// Tombstone it
	err = vs.TombstoneValue(valueID1)
	if err != nil {
		t.Fatalf("TombstoneValue failed: %v", err)
	}

	// Write a value of similar size
	value2 := Value{
		TypeTag: TypeText,
		Data:    []byte("second valu"), // Similar size to trigger reuse
	}

	valueID2, err := vs.WriteValue(value2)
	if err != nil {
		t.Fatalf("Second WriteValue failed: %v", err)
	}

	// Verify we can read the new value
	read, err := vs.ReadValue(valueID2)
	if err != nil {
		t.Fatalf("ReadValue failed: %v", err)
	}

	if !bytes.Equal(read.Data, value2.Data) {
		t.Errorf("Data mismatch: got %q, want %q", read.Data, value2.Data)
	}

	// Check that free list is reduced or empty
	tier := vs.extractTier(valueID2)
	initialFreeListSize := 1 // We had one tombstoned block
	currentFreeListSize := len(vs.freeLists[tier].GetBlocks())

	if currentFreeListSize >= initialFreeListSize {
		t.Errorf("Free list should be reduced after reuse")
	}
}
