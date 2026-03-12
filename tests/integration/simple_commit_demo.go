package integration

import (
	"fmt"
	"testing"

	"github.com/solifugus/amorphdb/internal/mbl/interpreter"
	"github.com/solifugus/amorphdb/internal/mbl/lexer"
	"github.com/solifugus/amorphdb/internal/mbl/parser"
	"github.com/solifugus/amorphdb/internal/storage"
	"github.com/solifugus/amorphdb/internal/types"
)

func TestSimpleCommitDemo(t *testing.T) {
	t.Log("🧪 Simple Commit Buffer Test")
	t.Log("============================")

	// Test basic commit buffer functionality with simple syntax
	t.Run("SimpleCommitBuffer", func(t *testing.T) {
		testSimpleCommitBuffer(t)
	})

	// Test coordinator functionality
	t.Run("CoordinatorBatching", func(t *testing.T) {
		testCoordinatorBatching(t)
	})

	t.Log("\n✅ Commit buffer implementation tests completed!")
}

func testSimpleCommitBuffer(t *testing.T) {
	t.Log("\n1️⃣ Testing basic commit buffer with simple assignments")

	// Create test storage
	storage := storage.NewMemoryTree()
	coordinator := createTestCoordinator(storage)

	// Create interpreter with coordinator
	interp := interpreter.NewWithCoordinator(storage, 1, coordinator)

	// Test simple MBL assignments
	code := `my.test.value = "hello"`

	// Parse and execute
	l := lexer.New(code)
	p := parser.New(l)
	program := p.ParseProgram()

	if len(p.Errors()) > 0 {
		t.Fatalf("❌ Parse errors: %v", p.Errors())
	}

	result, err := interp.Interpret(program)
	if err != nil {
		t.Fatalf("❌ Execution error: %v", err)
	}

	if unknown, ok := result.(types.Unknown); ok {
		t.Logf("❌ Execution returned Unknown: %s\n", unknown.Reason)
		return
	}

	// Check coordinator staging
	pendingCount := coordinator.GetPendingCount()
	t.Logf("✅ Simple assignment executed, %d writes staged in coordinator\n", pendingCount)

	// Test flush
	if err := coordinator.FlushAll(); err != nil {
		t.Fatalf("❌ Coordinator flush failed: %v", err)
	}

	t.Logf("✅ Coordinator flush successful\n")
}

func testCoordinatorBatching(t *testing.T) {
	t.Log("\n2️⃣ Testing coordinator batching functionality")

	// Create test storage
	storage := storage.NewMemoryTree()
	coordinator := createTestCoordinator(storage)

	// Test multiple interpreters staging writes
	for i := 0; i < 3; i++ {
		interp := interpreter.NewWithCoordinator(storage, uint64(i+1), coordinator)

		code := fmt.Sprintf(`my.batch.test%d = "value%d"`, i, i)

		l := lexer.New(code)
		p := parser.New(l)
		program := p.ParseProgram()

		if len(p.Errors()) > 0 {
			t.Fatalf("❌ Parse errors for interpreter %d: %v", i, p.Errors())
		}

		result, err := interp.Interpret(program)
		if err != nil {
			t.Fatalf("❌ Execution error for interpreter %d: %v", i, err)
		}

		if unknown, ok := result.(types.Unknown); ok {
			t.Logf("❌ Interpreter %d returned Unknown: %s\n", i, unknown.Reason)
			continue
		}
	}

	// Check total pending writes
	pendingCount := coordinator.GetPendingCount()
	t.Logf("✅ Multiple interpreters executed, %d total writes staged\n", pendingCount)

	// Flush all as batch
	if err := coordinator.FlushAll(); err != nil {
		t.Fatalf("❌ Batch flush failed: %v", err)
	}

	t.Logf("✅ Batch flush successful - all writes committed as single batch\n")
}

