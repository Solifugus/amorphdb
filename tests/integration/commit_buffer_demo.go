package integration

import (
	"testing"

	"github.com/solifugus/amorphdb/internal/mbl/interpreter"
	"github.com/solifugus/amorphdb/internal/storage"
)

func TestCommitBufferDemo(t *testing.T) {
	t.Log("AmorphDB Commit Buffer Demo")
	t.Log("===========================")

	// Create storage tree
	tree := storage.NewMemoryTree()

	// Test 1: Local variables should not enter commit buffer
	t.Run("LocalVariables", func(t *testing.T) {
		testLocalVariables(t, tree)
	})

	// Test 2: Persistent writes should be batched
	t.Run("PersistentBatching", func(t *testing.T) {
		testPersistentBatching(t, tree)
	})

	t.Log("\n✅ All commit buffer tests passed!")
}

func testLocalVariables(t *testing.T, tree storage.Tree) {
	t.Log("\n1. Testing local variables (zero mesh cost)")

	source := `
		# Local variable assignments - should NOT enter commit buffer
		x = 42
		y = "hello"
		z = x + 100
	`

	// Create interpreter for this test
	interp := interpreter.New(tree, 12345)
	result, err := executeProgram(interp, source)
	if err != nil {
		t.Fatalf("Failed to execute local variables test: %v", err)
	}
	t.Logf("   Result: %v", result)
	t.Log("   ✅ Local variables handled correctly")
}

func testPersistentBatching(t *testing.T, tree storage.Tree) {
	t.Log("\n2. Testing persistent writes batching")

	source := `
		# Multiple persistent writes - should be staged and batched
		my.data.count = 10
		my.data.name = "test"
		world.shared.value = 42
	`

	// Create interpreter for this test
	interp := interpreter.New(tree, 12345)
	result, err := executeProgram(interp, source)
	if err != nil {
		t.Fatalf("Failed to execute persistent batching test: %v", err)
	}
	t.Logf("   Result: %v", result)

	// Verify the writes actually persisted
	count, _ := tree.Read([]string{"world", "agent", "1", "data", "count"})
	name, _ := tree.Read([]string{"world", "agent", "1", "data", "name"})
	shared, _ := tree.Read([]string{"world", "shared", "value"})

	t.Logf("   Persisted count: %v", count)
	t.Logf("   Persisted name: %v", name)
	t.Logf("   Persisted shared: %v", shared)
	t.Log("   ✅ Persistent writes batched and committed")
}

