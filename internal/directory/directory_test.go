package directory

import (
	"testing"
	"time"

	"github.com/solifugus/amorphdb/internal/identity"
	"github.com/solifugus/amorphdb/internal/types"
)

// createTestIdentity creates a test identity for testing
func createTestIdentity(id, meshName, nodeID string) *identity.Identity {
	return &identity.Identity{
		ID:       id,
		Type:     types.NodeAgent,
		MeshName: meshName,
		Created:  time.Now(),
		NodeID:   nodeID,
	}
}

func TestDirectory_SetAndGetBasicPaths(t *testing.T) {
	dir := NewDirectory()

	// Create test identities
	authority1 := createTestIdentity("node-1", "test-mesh", "test-node-1")
	authority2 := createTestIdentity("node-2", "test-mesh", "test-node-2")

	timestamp1 := time.Now()
	timestamp2 := timestamp1.Add(time.Second)

	// Test basic set and get
	dir.Set("world.market", authority1, timestamp1)
	dir.Set("world.users", authority2, timestamp2)

	// Test exact matches
	auth, ts, ok := dir.Get("world.market")
	if !ok {
		t.Fatal("Expected to find authority for world.market")
	}
	if auth.ID != authority1.ID {
		t.Errorf("Expected authority %s, got %s", authority1.ID, auth.ID)
	}
	if !ts.Equal(timestamp1) {
		t.Errorf("Expected timestamp %v, got %v", timestamp1, ts)
	}

	auth, ts, ok = dir.Get("world.users")
	if !ok {
		t.Fatal("Expected to find authority for world.users")
	}
	if auth.ID != authority2.ID {
		t.Errorf("Expected authority %s, got %s", authority2.ID, auth.ID)
	}
	if !ts.Equal(timestamp2) {
		t.Errorf("Expected timestamp %v, got %v", timestamp2, ts)
	}

	// Test non-existent path
	_, _, ok = dir.Get("world.nonexistent")
	if ok {
		t.Error("Expected not to find authority for non-existent path")
	}
}

func TestDirectory_LongestPrefixMatching(t *testing.T) {
	dir := NewDirectory()

	// Create test identities
	marketAuth := createTestIdentity("market-auth", "test-mesh", "market-node")
	equitiesAuth := createTestIdentity("equities-auth", "test-mesh", "equities-node")

	baseTime := time.Now()

	// Set up hierarchy: world.market has one authority, world.market.equities has another
	dir.Set("world.market", marketAuth, baseTime)
	dir.Set("world.market.equities", equitiesAuth, baseTime.Add(time.Second))

	// Test exact match for more specific path
	auth, _, ok := dir.Get("world.market.equities")
	if !ok {
		t.Fatal("Expected to find authority for world.market.equities")
	}
	if auth.ID != equitiesAuth.ID {
		t.Errorf("Expected equities authority %s, got %s", equitiesAuth.ID, auth.ID)
	}

	// Test fallback to parent for non-specific path
	auth, _, ok = dir.Get("world.market.bonds")
	if !ok {
		t.Fatal("Expected to find authority for world.market.bonds (should fallback to world.market)")
	}
	if auth.ID != marketAuth.ID {
		t.Errorf("Expected market authority %s, got %s", marketAuth.ID, auth.ID)
	}

	// Test fallback to parent for deeply nested path
	auth, _, ok = dir.Get("world.market.bonds.corporate.high_yield")
	if !ok {
		t.Fatal("Expected to find authority for deeply nested path (should fallback to world.market)")
	}
	if auth.ID != marketAuth.ID {
		t.Errorf("Expected market authority %s, got %s", marketAuth.ID, auth.ID)
	}

	// Test no match when no parent exists
	_, _, ok = dir.Get("world.users.alice")
	if ok {
		t.Error("Expected not to find authority for path with no parent authority")
	}
}

func TestDirectory_Delete(t *testing.T) {
	dir := NewDirectory()

	authority := createTestIdentity("test-auth", "test-mesh", "test-node")
	timestamp := time.Now()

	// Set a path
	dir.Set("world.market", authority, timestamp)

	// Verify it exists
	_, _, ok := dir.Get("world.market")
	if !ok {
		t.Fatal("Expected to find authority after setting")
	}

	// Delete it
	dir.Delete("world.market")

	// Verify it's gone
	_, _, ok = dir.Get("world.market")
	if ok {
		t.Error("Expected not to find authority after deletion")
	}

	// Verify size is correct
	if dir.Size() != 0 {
		t.Errorf("Expected directory size 0 after deletion, got %d", dir.Size())
	}
}

func TestDirectory_SnapshotRoundTrip(t *testing.T) {
	// Create source directory with multiple entries
	sourceDir := NewDirectory()

	auth1 := createTestIdentity("auth-1", "test-mesh", "node-1")
	auth2 := createTestIdentity("auth-2", "test-mesh", "node-2")
	auth3 := createTestIdentity("auth-3", "test-mesh", "node-3")

	baseTime := time.Now()

	sourceDir.Set("world.market", auth1, baseTime)
	sourceDir.Set("world.users", auth2, baseTime.Add(time.Second))
	sourceDir.Set("world.market.equities", auth3, baseTime.Add(2*time.Second))

	// Take snapshot
	snapshot := sourceDir.Snapshot()

	// Verify snapshot has correct number of entries
	if len(snapshot) != 3 {
		t.Errorf("Expected snapshot length 3, got %d", len(snapshot))
	}

	// Create new directory and merge snapshot
	targetDir := NewDirectory()
	targetDir.Merge(snapshot)

	// Verify all entries were merged correctly
	auth, _, ok := targetDir.Get("world.market")
	if !ok || auth.ID != auth1.ID {
		t.Error("Failed to merge world.market entry")
	}

	auth, _, ok = targetDir.Get("world.users")
	if !ok || auth.ID != auth2.ID {
		t.Error("Failed to merge world.users entry")
	}

	auth, _, ok = targetDir.Get("world.market.equities")
	if !ok || auth.ID != auth3.ID {
		t.Error("Failed to merge world.market.equities entry")
	}

	// Verify target directory size
	if targetDir.Size() != 3 {
		t.Errorf("Expected target directory size 3, got %d", targetDir.Size())
	}
}

func TestDirectory_MergeWithLastWriteWins(t *testing.T) {
	dir := NewDirectory()

	oldAuth := createTestIdentity("old-auth", "test-mesh", "old-node")
	newAuth := createTestIdentity("new-auth", "test-mesh", "new-node")

	oldTime := time.Now()
	newTime := oldTime.Add(time.Hour)

	// Set initial entry
	dir.Set("world.market", oldAuth, oldTime)

	// Verify initial state
	auth, ts, ok := dir.Get("world.market")
	if !ok || auth.ID != oldAuth.ID {
		t.Fatal("Failed to set initial entry")
	}
	if !ts.Equal(oldTime) {
		t.Fatalf("Expected initial timestamp %v, got %v", oldTime, ts)
	}

	// Merge newer entry (should win)
	newerEntries := []DirectoryEntry{
		{
			Path:      "world.market",
			Authority: newAuth,
			Timestamp: newTime,
		},
	}
	dir.Merge(newerEntries)

	// Verify newer entry won
	auth, ts, ok = dir.Get("world.market")
	if !ok || auth.ID != newAuth.ID {
		t.Error("Expected newer entry to win last-write-wins conflict")
	}
	if !ts.Equal(newTime) {
		t.Errorf("Expected newer timestamp %v, got %v", newTime, ts)
	}

	// Merge older entry (should not win)
	olderEntries := []DirectoryEntry{
		{
			Path:      "world.market",
			Authority: oldAuth,
			Timestamp: oldTime,
		},
	}
	dir.Merge(olderEntries)

	// Verify newer entry still wins
	auth, ts, ok = dir.Get("world.market")
	if !ok || auth.ID != newAuth.ID {
		t.Error("Expected newer entry to remain after merging older entry")
	}
	if !ts.Equal(newTime) {
		t.Errorf("Expected newer timestamp %v to remain, got %v", newTime, ts)
	}
}

func TestDirectory_ConcurrentOperations(t *testing.T) {
	dir := NewDirectory()

	auth1 := createTestIdentity("auth-1", "test-mesh", "node-1")
	// auth2 := createTestIdentity("auth-2", "test-mesh", "node-2") // Removed unused variable

	timestamp := time.Now()

	// Test concurrent reads and writes
	done := make(chan bool, 2)

	// Writer goroutine
	go func() {
		for i := 0; i < 100; i++ {
			path := "world.test" + string(rune('a'+i%26))
			dir.Set(path, auth1, timestamp)
		}
		done <- true
	}()

	// Reader goroutine
	go func() {
		for i := 0; i < 100; i++ {
			path := "world.test" + string(rune('a'+i%26))
			dir.Get(path)
		}
		done <- true
	}()

	// Wait for both goroutines to complete
	<-done
	<-done

	// Verify final state
	if dir.Size() > 26 {
		t.Errorf("Expected directory size <= 26, got %d", dir.Size())
	}
}

func TestDirectory_PrefixMatchingEdgeCases(t *testing.T) {
	dir := NewDirectory()

	auth1 := createTestIdentity("auth-1", "test-mesh", "node-1")
	auth2 := createTestIdentity("auth-2", "test-mesh", "node-2")

	timestamp := time.Now()

	// Set up confusing paths that could cause false matches
	dir.Set("world.market", auth1, timestamp)
	dir.Set("world.marketplace", auth2, timestamp.Add(time.Second))

	// Test that "world.market.bonds" matches "world.market", not "world.marketplace"
	auth, _, ok := dir.Get("world.market.bonds")
	if !ok {
		t.Fatal("Expected to find authority for world.market.bonds")
	}
	if auth.ID != auth1.ID {
		t.Errorf("Expected world.market authority %s, got %s", auth1.ID, auth.ID)
	}

	// Test that "world.marketplace.items" matches "world.marketplace"
	auth, _, ok = dir.Get("world.marketplace.items")
	if !ok {
		t.Fatal("Expected to find authority for world.marketplace.items")
	}
	if auth.ID != auth2.ID {
		t.Errorf("Expected world.marketplace authority %s, got %s", auth2.ID, auth.ID)
	}

	// Test that "world.marketdata" doesn't match either (no prefix match)
	_, _, ok = dir.Get("world.marketdata")
	if ok {
		t.Error("Expected not to find authority for world.marketdata (should not prefix match)")
	}
}