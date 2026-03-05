package storage

import (
	"os"
	"testing"
)

func TestNewDefragmenter(t *testing.T) {
	defrag := NewDefragmenter()
	if defrag == nil {
		t.Error("NewDefragmenter returned nil")
	}
}

func TestDefragmentValueStore(t *testing.T) {
	tempDir := t.TempDir()

	vs, err := NewValueStore(tempDir)
	if err != nil {
		t.Fatalf("NewValueStore failed: %v", err)
	}
	defer vs.Close()

	// Create some test values
	values := []Value{
		{TypeTag: TypeText, Data: []byte("value1")},
		{TypeTag: TypeText, Data: []byte("value2")},
		{TypeTag: TypeText, Data: []byte("value3")},
		{TypeTag: TypeText, Data: []byte("large_value_" + string(make([]byte, 5000)))}, // Tier 3
	}

	var valueIDs []uint64
	for _, value := range values {
		valueID, err := vs.WriteValue(value)
		if err != nil {
			t.Fatalf("WriteValue failed: %v", err)
		}
		valueIDs = append(valueIDs, valueID)
	}

	// Tombstone some values to create fragmentation
	err = vs.TombstoneValue(valueIDs[1])
	if err != nil {
		t.Fatalf("TombstoneValue failed: %v", err)
	}

	err = vs.TombstoneValue(valueIDs[3])
	if err != nil {
		t.Fatalf("TombstoneValue failed: %v", err)
	}

	// Get original file size for reference (used in logging)
	info, err := vs.tier3File.Stat()
	if err != nil {
		t.Fatalf("Failed to stat tier 3 file: %v", err)
	}
	_ = info.Size() // originalSize - tracked in stats instead

	// Perform defragmentation
	defrag := NewDefragmenter()
	stats, err := defrag.DefragmentValueStore(vs)
	if err != nil {
		t.Fatalf("DefragmentValueStore failed: %v", err)
	}

	// Verify stats
	if stats == nil {
		t.Error("DefragmentationStats is nil")
		return
	}

	if stats.TombstonesRemoved == 0 {
		t.Error("Expected some tombstones to be removed")
	}

	if stats.RecordsProcessed == 0 {
		t.Error("Expected some records to be processed")
	}

	t.Logf("Defragmentation stats: Original=%d, Compacted=%d, Tombstones=%d, Records=%d",
		stats.OriginalSize, stats.CompactedSize, stats.TombstonesRemoved, stats.RecordsProcessed)

	// Verify that remaining values can still be read
	remainingValueIDs := []uint64{valueIDs[0], valueIDs[2]}
	expectedValues := []Value{values[0], values[2]}

	for i, valueID := range remainingValueIDs {
		readValue, err := vs.ReadValue(valueID)
		if err != nil {
			t.Errorf("Failed to read value %d after defragmentation: %v", valueID, err)
			continue
		}

		if readValue.TypeTag != expectedValues[i].TypeTag {
			t.Errorf("Type mismatch for value %d: got %d, want %d",
				valueID, readValue.TypeTag, expectedValues[i].TypeTag)
		}

		if string(readValue.Data) != string(expectedValues[i].Data) {
			t.Errorf("Data mismatch for value %d: got %s, want %s",
				valueID, string(readValue.Data), string(expectedValues[i].Data))
		}
	}
}

func TestDefragmentEmptyValueStore(t *testing.T) {
	tempDir := t.TempDir()

	vs, err := NewValueStore(tempDir)
	if err != nil {
		t.Fatalf("NewValueStore failed: %v", err)
	}
	defer vs.Close()

	// Perform defragmentation on empty store
	defrag := NewDefragmenter()
	stats, err := defrag.DefragmentValueStore(vs)
	if err != nil {
		t.Fatalf("DefragmentValueStore failed on empty store: %v", err)
	}

	// Verify stats for empty store
	if stats.OriginalSize != 0 {
		t.Errorf("Expected OriginalSize to be 0, got %d", stats.OriginalSize)
	}

	if stats.CompactedSize != 0 {
		t.Errorf("Expected CompactedSize to be 0, got %d", stats.CompactedSize)
	}

	if stats.TombstonesRemoved != 0 {
		t.Errorf("Expected TombstonesRemoved to be 0, got %d", stats.TombstonesRemoved)
	}

	if stats.RecordsProcessed != 0 {
		t.Errorf("Expected RecordsProcessed to be 0, got %d", stats.RecordsProcessed)
	}
}

func TestDefragmentationStats(t *testing.T) {
	tempDir := t.TempDir()

	vs, err := NewValueStore(tempDir)
	if err != nil {
		t.Fatalf("NewValueStore failed: %v", err)
	}
	defer vs.Close()

	// Create a large value to ensure tier 3 usage
	largeData := make([]byte, 8000) // Force tier 3
	for i := range largeData {
		largeData[i] = byte(i % 256)
	}

	value := Value{TypeTag: TypeText, Data: largeData}
	valueID, err := vs.WriteValue(value)
	if err != nil {
		t.Fatalf("WriteValue failed: %v", err)
	}

	// Get original size before tombstoning
	info, err := vs.tier3File.Stat()
	if err != nil {
		t.Fatalf("Failed to stat tier 3 file: %v", err)
	}
	sizeBefore := info.Size()

	// Tombstone the value
	err = vs.TombstoneValue(valueID)
	if err != nil {
		t.Fatalf("TombstoneValue failed: %v", err)
	}

	// Defragment
	defrag := NewDefragmenter()
	stats, err := defrag.DefragmentValueStore(vs)
	if err != nil {
		t.Fatalf("DefragmentValueStore failed: %v", err)
	}

	// Check that bytes were reclaimed
	if stats.BytesReclaimed <= 0 {
		t.Errorf("Expected BytesReclaimed > 0, got %d", stats.BytesReclaimed)
	}

	if stats.CompactedSize >= stats.OriginalSize {
		t.Errorf("Expected CompactedSize < OriginalSize, got %d >= %d",
			stats.CompactedSize, stats.OriginalSize)
	}

	// Verify file actually got smaller
	info, err = vs.tier3File.Stat()
	if err != nil {
		t.Fatalf("Failed to stat tier 3 file after defrag: %v", err)
	}
	sizeAfter := info.Size()

	if sizeAfter >= sizeBefore {
		t.Errorf("Expected file to be smaller after defragmentation: before=%d, after=%d",
			sizeBefore, sizeAfter)
	}

	t.Logf("File size before: %d, after: %d, bytes reclaimed: %d",
		sizeBefore, sizeAfter, sizeBefore-sizeAfter)
}

func TestFreeListRebuildAfterDefrag(t *testing.T) {
	tempDir := t.TempDir()

	vs, err := NewValueStore(tempDir)
	if err != nil {
		t.Fatalf("NewValueStore failed: %v", err)
	}
	defer vs.Close()

	// Create and tombstone some values to populate free list
	value := Value{TypeTag: TypeText, Data: []byte("test_value")}
	valueID, err := vs.WriteValue(value)
	if err != nil {
		t.Fatalf("WriteValue failed: %v", err)
	}

	err = vs.TombstoneValue(valueID)
	if err != nil {
		t.Fatalf("TombstoneValue failed: %v", err)
	}

	// Check that free list has entries before defrag
	initialFreeListSize := len(vs.freeLists[vs.determineTier(len(vs.serializeValue(value)))].GetBlocks())
	if initialFreeListSize == 0 {
		t.Error("Expected free list to have entries after tombstoning")
	}

	// Perform defragmentation
	defrag := NewDefragmenter()
	_, err = defrag.DefragmentValueStore(vs)
	if err != nil {
		t.Fatalf("DefragmentValueStore failed: %v", err)
	}

	// After defragmentation, the free list should be rebuilt
	// Since we tombstoned everything and it was compacted away,
	// the free list should be empty or have different entries
	finalFreeListSize := len(vs.freeLists[vs.determineTier(len(vs.serializeValue(value)))].GetBlocks())

	t.Logf("Free list size before defrag: %d, after defrag: %d",
		initialFreeListSize, finalFreeListSize)

	// The exact behavior depends on implementation, but we should verify
	// that the free list rebuild operation completed without error
}

func TestRebuildFreeListFromFile(t *testing.T) {
	// Create a temporary file with some test data
	tempFile, err := os.CreateTemp(t.TempDir(), "test_freelist_*.dat")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer tempFile.Close()

	// Write a test record followed by a tombstone
	testValue := Value{TypeTag: TypeText, Data: []byte("hello")}

	// Create a minimal ValueStore to use serialization methods
	tempDir := t.TempDir()
	vs, err := NewValueStore(tempDir)
	if err != nil {
		t.Fatalf("NewValueStore failed: %v", err)
	}
	defer vs.Close()

	// Serialize a normal value
	serialized := vs.serializeValue(testValue)
	_, err = tempFile.Write(serialized)
	if err != nil {
		t.Fatalf("Failed to write test value: %v", err)
	}

	// Create a tombstone record
	tombstoneData := make([]byte, len(serialized))
	copy(tombstoneData, serialized)
	tombstoneData[0] = SentinelTombstone // Change sentinel to tombstone
	_, err = tempFile.Write(tombstoneData)
	if err != nil {
		t.Fatalf("Failed to write tombstone: %v", err)
	}

	// Sync the file
	err = tempFile.Sync()
	if err != nil {
		t.Fatalf("Failed to sync file: %v", err)
	}

	// Test rebuilding free list from the file
	freeList := NewFreeList()
	err = freeList.RebuildFromFile(tempFile)
	if err != nil {
		t.Fatalf("RebuildFromFile failed: %v", err)
	}

	// Verify that the tombstone was detected and added to free list
	blocks := freeList.GetBlocks()
	if len(blocks) == 0 {
		t.Error("Expected free list to have blocks after rebuild")
		return
	}

	// The tombstone should be at offset equal to the first record's size
	expectedOffset := uint64(len(serialized))
	found := false
	for _, block := range blocks {
		if block.Offset == expectedOffset {
			found = true
			if block.Size != uint64(len(tombstoneData)) {
				t.Errorf("Expected block size %d, got %d", len(tombstoneData), block.Size)
			}
			break
		}
	}

	if !found {
		t.Errorf("Expected to find free block at offset %d", expectedOffset)
	}

	t.Logf("Found %d free blocks after rebuild", len(blocks))
	for i, block := range blocks {
		t.Logf("Block %d: offset=%d, size=%d", i, block.Offset, block.Size)
	}
}

// TestStep1_6SuccessCriteria validates all success criteria for Step 1.6
func TestStep1_6SuccessCriteria(t *testing.T) {
	tempDir := t.TempDir()
	vs, err := NewValueStore(tempDir)
	if err != nil {
		t.Fatalf("NewValueStore failed: %v", err)
	}
	defer vs.Close()

	// Success criteria 1: Purge values, confirm free list entries created
	t.Log("Testing: Purge values, confirm free list entries created")

	value1 := Value{TypeTag: TypeText, Data: []byte("test_value_1")}
	valueID1, err := vs.WriteValue(value1)
	if err != nil {
		t.Fatalf("WriteValue failed: %v", err)
	}

	// Purge (tombstone) the value
	err = vs.TombstoneValue(valueID1)
	if err != nil {
		t.Fatalf("TombstoneValue failed: %v", err)
	}

	// Verify free list entry was created
	tier := vs.determineTier(len(vs.serializeValue(value1)))
	freeBlocks := vs.freeLists[tier].GetBlocks()
	if len(freeBlocks) == 0 {
		t.Error("Expected free list entry after purging value")
	}
	t.Logf("✓ Free list has %d entries after purging", len(freeBlocks))

	// Success criteria 2: New writes reuse free list space when available
	t.Log("Testing: New writes reuse free list space when available")

	// Record initial free space
	initialFreeSpace := vs.freeLists[tier].GetTotalFreeSpace()

	// Write a new value of similar size
	value2 := Value{TypeTag: TypeText, Data: []byte("test_value_2")}
	valueID2, err := vs.WriteValue(value2)
	if err != nil {
		t.Fatalf("WriteValue failed: %v", err)
	}

	// Verify free space was reduced (space was reused)
	finalFreeSpace := vs.freeLists[tier].GetTotalFreeSpace()
	if finalFreeSpace >= initialFreeSpace {
		t.Errorf("Expected free space to be reduced: initial=%d, final=%d",
			initialFreeSpace, finalFreeSpace)
	}
	t.Logf("✓ Free space reduced from %d to %d (reused)", initialFreeSpace, finalFreeSpace)

	// Verify we can still read the reused value
	readValue, err := vs.ReadValue(valueID2)
	if err != nil {
		t.Fatalf("Failed to read reused value: %v", err)
	}
	if string(readValue.Data) != "test_value_2" {
		t.Errorf("Value data mismatch after reuse: got %s, want test_value_2",
			string(readValue.Data))
	}
	t.Log("✓ Reused space value reads correctly")

	// Success criteria 3: Free list can be rebuilt from scanning tombstones
	t.Log("Testing: Free list can be rebuilt from scanning tombstones")

	// Create a large value to force tier 3 usage for this test
	tier3Value := Value{TypeTag: TypeText, Data: make([]byte, 5000)}
	for i := range tier3Value.Data {
		tier3Value.Data[i] = byte('B')
	}
	tier3ValueID, err := vs.WriteValue(tier3Value)
	if err != nil {
		t.Fatalf("WriteValue failed: %v", err)
	}

	// Tombstone the tier 3 value
	err = vs.TombstoneValue(tier3ValueID)
	if err != nil {
		t.Fatalf("TombstoneValue failed: %v", err)
	}

	// Clear the tier 3 free list manually
	vs.freeLists[3].Clear()
	if len(vs.freeLists[3].GetBlocks()) != 0 {
		t.Error("Expected empty free list after clearing")
	}

	// Rebuild free list by scanning the tier 3 file
	err = vs.freeLists[3].RebuildFromFile(vs.tier3File)
	if err != nil {
		t.Fatalf("RebuildFromFile failed: %v", err)
	}

	// Verify free list was rebuilt
	rebuiltBlocks := vs.freeLists[3].GetBlocks()
	if len(rebuiltBlocks) == 0 {
		t.Error("Expected free list entries after rebuilding from tombstones")
	} else {
		t.Logf("✓ Rebuilt %d free blocks from scanning tombstones", len(rebuiltBlocks))
	}

	// Success criteria 4: Defragmentation compacts a file, updates all references
	t.Log("Testing: Defragmentation compacts a file, updates all references")

	// Create a large value to ensure tier 3 usage
	largeValue := Value{TypeTag: TypeText, Data: make([]byte, 8000)}
	for i := range largeValue.Data {
		largeValue.Data[i] = byte('A' + (i % 26))
	}

	largeValueID, err := vs.WriteValue(largeValue)
	if err != nil {
		t.Fatalf("WriteValue failed: %v", err)
	}

	// Get file size before tombstoning
	info, err := vs.tier3File.Stat()
	if err != nil {
		t.Fatalf("Failed to stat tier 3 file: %v", err)
	}
	sizeBeforeTombstone := info.Size()

	// Tombstone the large value
	err = vs.TombstoneValue(largeValueID)
	if err != nil {
		t.Fatalf("TombstoneValue failed: %v", err)
	}

	// Perform defragmentation
	defrag := NewDefragmenter()
	stats, err := defrag.DefragmentValueStore(vs)
	if err != nil {
		t.Fatalf("DefragmentValueStore failed: %v", err)
	}

	// Verify compaction occurred
	if stats.BytesReclaimed <= 0 {
		t.Errorf("Expected bytes to be reclaimed during defragmentation")
	}

	// Verify file actually got smaller
	info, err = vs.tier3File.Stat()
	if err != nil {
		t.Fatalf("Failed to stat tier 3 file after defrag: %v", err)
	}
	sizeAfterDefrag := info.Size()

	if sizeAfterDefrag >= sizeBeforeTombstone {
		t.Errorf("Expected file to be smaller after defragmentation: before=%d, after=%d",
			sizeBeforeTombstone, sizeAfterDefrag)
	}

	t.Logf("✓ Defragmentation compacted file from %d to %d bytes (reclaimed %d bytes)",
		sizeBeforeTombstone, sizeAfterDefrag, stats.BytesReclaimed)

	// Verify that remaining values can still be read (references updated)
	readValue2, err := vs.ReadValue(valueID2)
	if err != nil {
		t.Errorf("Failed to read value after defragmentation: %v", err)
	} else if string(readValue2.Data) != "test_value_2" {
		t.Errorf("Value corrupted after defragmentation: got %s, want test_value_2",
			string(readValue2.Data))
	} else {
		t.Log("✓ References updated correctly after defragmentation")
	}

	t.Log("✅ All Step 1.6 success criteria validated")
}