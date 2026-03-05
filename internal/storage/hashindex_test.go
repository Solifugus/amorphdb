package storage

import (
	"path/filepath"
	"testing"
	"time"
)

func TestNewHashIndex(t *testing.T) {
	// Test with default bucket count
	hi := NewHashIndex(0)
	if hi.bucketCount != DefaultBucketCount {
		t.Errorf("Expected bucket count %d, got %d", DefaultBucketCount, hi.bucketCount)
	}
	if hi.Size() != 0 {
		t.Errorf("New index should have size 0, got %d", hi.Size())
	}

	// Test with custom bucket count
	customCount := uint64(512)
	hi2 := NewHashIndex(customCount)
	if hi2.bucketCount != customCount {
		t.Errorf("Expected bucket count %d, got %d", customCount, hi2.bucketCount)
	}
}

func TestInsertAndLookup(t *testing.T) {
	hi := NewHashIndex(64) // Smaller bucket count for testing

	// Test basic insert and lookup
	labelValueID := uint64(12345)
	attributeID := uint64(67890)

	hi.Insert(labelValueID, attributeID)
	if hi.Size() != 1 {
		t.Errorf("Expected size 1 after insert, got %d", hi.Size())
	}

	foundID, found := hi.Lookup(labelValueID)
	if !found {
		t.Errorf("Lookup failed for inserted label")
	}
	if foundID != attributeID {
		t.Errorf("Lookup returned wrong attribute ID: got %d, want %d", foundID, attributeID)
	}

	// Test lookup of non-existent label
	_, found = hi.Lookup(99999)
	if found {
		t.Errorf("Lookup should have failed for non-existent label")
	}
}

func TestInsertUpdate(t *testing.T) {
	hi := NewHashIndex(64)

	labelValueID := uint64(12345)
	attributeID1 := uint64(100)
	attributeID2 := uint64(200)

	// Insert first mapping
	hi.Insert(labelValueID, attributeID1)
	foundID, found := hi.Lookup(labelValueID)
	if !found || foundID != attributeID1 {
		t.Errorf("First insert failed: found=%t, id=%d, want=%d", found, foundID, attributeID1)
	}

	// Update mapping (same label, different attribute ID)
	hi.Insert(labelValueID, attributeID2)
	if hi.Size() != 1 {
		t.Errorf("Update should not increase size: got %d, want 1", hi.Size())
	}

	foundID, found = hi.Lookup(labelValueID)
	if !found || foundID != attributeID2 {
		t.Errorf("Update failed: found=%t, id=%d, want=%d", found, foundID, attributeID2)
	}
}

func TestHashCollisions(t *testing.T) {
	// Use very small bucket count to force collisions
	hi := NewHashIndex(2)

	// Insert multiple entries that will likely collide
	entries := map[uint64]uint64{
		1000: 1,
		2000: 2,
		3000: 3,
		4000: 4,
		5000: 5,
	}

	// Insert all entries
	for labelID, attrID := range entries {
		hi.Insert(labelID, attrID)
	}

	if hi.Size() != uint64(len(entries)) {
		t.Errorf("Expected size %d, got %d", len(entries), hi.Size())
	}

	// Verify all entries can be found despite collisions
	for labelID, expectedAttrID := range entries {
		foundID, found := hi.Lookup(labelID)
		if !found {
			t.Errorf("Failed to find label %d", labelID)
		}
		if foundID != expectedAttrID {
			t.Errorf("Wrong attribute ID for label %d: got %d, want %d", labelID, foundID, expectedAttrID)
		}
	}

	// Check collision statistics
	usedBuckets, totalChainLength, maxChainLength := hi.GetCollisionStats()
	t.Logf("Collision stats: used buckets=%d, total chain length=%d, max chain length=%d",
		usedBuckets, totalChainLength, maxChainLength)

	// With 5 entries and 2 buckets, we should have collisions
	if maxChainLength < 2 {
		t.Errorf("Expected some collisions with small bucket count, max chain length=%d", maxChainLength)
	}
}

func TestRemove(t *testing.T) {
	hi := NewHashIndex(64)

	// Insert some entries
	entries := map[uint64]uint64{
		100: 1,
		200: 2,
		300: 3,
	}

	for labelID, attrID := range entries {
		hi.Insert(labelID, attrID)
	}

	// Remove middle entry
	removed := hi.Remove(200)
	if !removed {
		t.Errorf("Remove should have succeeded for existing entry")
	}

	if hi.Size() != 2 {
		t.Errorf("Size should be 2 after removal, got %d", hi.Size())
	}

	// Verify it's gone
	_, found := hi.Lookup(200)
	if found {
		t.Errorf("Removed entry should not be found")
	}

	// Verify others still exist
	for labelID, expectedAttrID := range entries {
		if labelID == 200 {
			continue // Skip the removed one
		}
		foundID, found := hi.Lookup(labelID)
		if !found || foundID != expectedAttrID {
			t.Errorf("Remaining entry %d should still exist: found=%t, id=%d", labelID, found, foundID)
		}
	}

	// Try to remove non-existent entry
	removed = hi.Remove(999)
	if removed {
		t.Errorf("Remove should have failed for non-existent entry")
	}
}

func TestLargeScalePerformance(t *testing.T) {
	// Insert 1000 attributes and confirm O(1) lookup performance
	hi := NewHashIndex(0) // Use default bucket count

	const numEntries = 1000
	labels := make([]uint64, numEntries)

	// Insert 1000 entries
	insertStart := time.Now()
	for i := 0; i < numEntries; i++ {
		labelID := uint64(10000 + i)
		attributeID := uint64(i + 1)
		labels[i] = labelID
		hi.Insert(labelID, attributeID)
	}
	insertDuration := time.Since(insertStart)

	if hi.Size() != numEntries {
		t.Errorf("Expected size %d after inserts, got %d", numEntries, hi.Size())
	}

	// Lookup all entries and measure performance
	lookupStart := time.Now()
	for i, labelID := range labels {
		expectedAttrID := uint64(i + 1)
		foundID, found := hi.Lookup(labelID)
		if !found {
			t.Errorf("Failed to find label %d", labelID)
		}
		if foundID != expectedAttrID {
			t.Errorf("Wrong attribute ID for label %d: got %d, want %d", labelID, foundID, expectedAttrID)
		}
	}
	lookupDuration := time.Since(lookupStart)

	t.Logf("Performance test: %d inserts took %v, %d lookups took %v",
		numEntries, insertDuration, numEntries, lookupDuration)

	// Calculate average times
	avgInsert := insertDuration.Nanoseconds() / numEntries
	avgLookup := lookupDuration.Nanoseconds() / numEntries

	t.Logf("Average times: insert=%dns, lookup=%dns", avgInsert, avgLookup)

	// Get collision statistics
	usedBuckets, totalChainLength, maxChainLength := hi.GetCollisionStats()
	avgChainLength := float64(totalChainLength) / float64(usedBuckets)

	t.Logf("Collision stats: used buckets=%d/%d, avg chain length=%.2f, max chain length=%d",
		usedBuckets, hi.bucketCount, avgChainLength, maxChainLength)

	// With good hash distribution, max chain length should be reasonable
	if maxChainLength > 10 {
		t.Logf("Warning: max chain length %d is quite high, may indicate poor hash distribution", maxChainLength)
	}
}

func TestRebuildFromAttributeStore(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "attributes.dat")

	// Create attribute store and add some attributes
	as, err := NewAttributeStore(filePath)
	if err != nil {
		t.Fatalf("NewAttributeStore failed: %v", err)
	}
	defer as.Close()

	// Add test attributes
	testAttributes := map[uint64]uint64{
		1001: 0, // Will get attribute ID 1
		2002: 0, // Will get attribute ID 2
		3003: 0, // Will get attribute ID 3
		4004: 0, // Will get attribute ID 4
	}

	for labelValueID := range testAttributes {
		attributeID, err := as.WriteAttribute(labelValueID, 9999, 0)
		if err != nil {
			t.Fatalf("WriteAttribute failed: %v", err)
		}
		testAttributes[labelValueID] = attributeID
	}

	// Create hash index and rebuild from store
	hi := NewHashIndex(64)
	err = hi.RebuildFromAttributeStore(as)
	if err != nil {
		t.Fatalf("RebuildFromAttributeStore failed: %v", err)
	}

	// Verify all attributes are in the index
	if hi.Size() != uint64(len(testAttributes)) {
		t.Errorf("Index size after rebuild: got %d, want %d", hi.Size(), len(testAttributes))
	}

	for labelValueID, expectedAttributeID := range testAttributes {
		foundID, found := hi.Lookup(labelValueID)
		if !found {
			t.Errorf("Failed to find rebuilt attribute with label %d", labelValueID)
		}
		if foundID != expectedAttributeID {
			t.Errorf("Wrong attribute ID after rebuild for label %d: got %d, want %d",
				labelValueID, foundID, expectedAttributeID)
		}
	}

	// Test rebuilding again (should produce identical results)
	hi2 := NewHashIndex(64)
	err = hi2.RebuildFromAttributeStore(as)
	if err != nil {
		t.Fatalf("Second RebuildFromAttributeStore failed: %v", err)
	}

	if hi.Size() != hi2.Size() {
		t.Errorf("Rebuild sizes don't match: first=%d, second=%d", hi.Size(), hi2.Size())
	}

	// Verify identical results
	for labelValueID := range testAttributes {
		id1, found1 := hi.Lookup(labelValueID)
		id2, found2 := hi2.Lookup(labelValueID)

		if found1 != found2 || id1 != id2 {
			t.Errorf("Rebuild results don't match for label %d: first=(%d,%t), second=(%d,%t)",
				labelValueID, id1, found1, id2, found2)
		}
	}
}

func TestClear(t *testing.T) {
	hi := NewHashIndex(64)

	// Add some entries
	for i := 0; i < 5; i++ {
		hi.Insert(uint64(1000+i), uint64(i+1))
	}

	if hi.Size() != 5 {
		t.Errorf("Expected size 5 before clear, got %d", hi.Size())
	}

	// Clear the index
	hi.Clear()

	if hi.Size() != 0 {
		t.Errorf("Expected size 0 after clear, got %d", hi.Size())
	}

	// Verify lookups fail
	_, found := hi.Lookup(1000)
	if found {
		t.Errorf("Lookup should fail after clear")
	}
}

func TestConcurrentAccess(t *testing.T) {
	hi := NewHashIndex(256)

	// Test concurrent inserts and lookups
	const numGoroutines = 10
	const numOpsPerGoroutine = 100

	done := make(chan bool, numGoroutines)

	// Start multiple goroutines doing inserts
	for g := 0; g < numGoroutines; g++ {
		go func(goroutineID int) {
			defer func() { done <- true }()

			base := uint64(goroutineID * numOpsPerGoroutine)

			// Insert entries
			for i := 0; i < numOpsPerGoroutine; i++ {
				labelID := base + uint64(i)
				attributeID := labelID * 2
				hi.Insert(labelID, attributeID)
			}

			// Lookup entries
			for i := 0; i < numOpsPerGoroutine; i++ {
				labelID := base + uint64(i)
				expectedAttrID := labelID * 2
				foundID, found := hi.Lookup(labelID)
				if found && foundID != expectedAttrID {
					t.Errorf("Concurrent lookup failed: label=%d, got=%d, want=%d",
						labelID, foundID, expectedAttrID)
				}
			}
		}(g)
	}

	// Wait for all goroutines to complete
	for g := 0; g < numGoroutines; g++ {
		<-done
	}

	expectedSize := uint64(numGoroutines * numOpsPerGoroutine)
	if hi.Size() != expectedSize {
		t.Errorf("Expected size %d after concurrent operations, got %d", expectedSize, hi.Size())
	}
}