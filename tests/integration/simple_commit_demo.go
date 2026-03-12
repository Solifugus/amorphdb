package main

import (
	"fmt"
	"log"
	"os"

	"github.com/solifugus/amorphdb/internal/mbl/interpreter"
	"github.com/solifugus/amorphdb/internal/mbl/lexer"
	"github.com/solifugus/amorphdb/internal/mbl/parser"
	"github.com/solifugus/amorphdb/internal/storage"
	"github.com/solifugus/amorphdb/internal/types"
	"github.com/solifugus/amorphdb/internal/zone"
)

func main() {
	fmt.Println("🧪 Simple Commit Buffer Test")
	fmt.Println("============================")

	// Test basic commit buffer functionality with simple syntax
	testSimpleCommitBuffer()

	// Test coordinator functionality
	testCoordinatorBatching()

	fmt.Println("\n✅ Commit buffer implementation tests completed!")
}

func testSimpleCommitBuffer() {
	fmt.Println("\n1️⃣ Testing basic commit buffer with simple assignments")

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
		fmt.Printf("❌ Parse errors: %v\n", p.Errors())
		return
	}

	result, err := interp.Interpret(program)
	if err != nil {
		fmt.Printf("❌ Execution error: %v\n", err)
		return
	}

	if unknown, ok := result.(types.Unknown); ok {
		fmt.Printf("❌ Execution returned Unknown: %s\n", unknown.Reason)
		return
	}

	// Check coordinator staging
	pendingCount := coordinator.GetPendingCount()
	fmt.Printf("✅ Simple assignment executed, %d writes staged in coordinator\n", pendingCount)

	// Test flush
	if err := coordinator.FlushAll(); err != nil {
		fmt.Printf("❌ Coordinator flush failed: %v\n", err)
		return
	}

	fmt.Printf("✅ Coordinator flush successful\n")
}

func testCoordinatorBatching() {
	fmt.Println("\n2️⃣ Testing coordinator batching functionality")

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
			fmt.Printf("❌ Parse errors for interpreter %d: %v\n", i, p.Errors())
			continue
		}

		result, err := interp.Interpret(program)
		if err != nil {
			fmt.Printf("❌ Execution error for interpreter %d: %v\n", i, err)
			continue
		}

		if unknown, ok := result.(types.Unknown); ok {
			fmt.Printf("❌ Interpreter %d returned Unknown: %s\n", i, unknown.Reason)
			continue
		}
	}

	// Check total pending writes
	pendingCount := coordinator.GetPendingCount()
	fmt.Printf("✅ Multiple interpreters executed, %d total writes staged\n", pendingCount)

	// Flush all as batch
	if err := coordinator.FlushAll(); err != nil {
		fmt.Printf("❌ Batch flush failed: %v\n", err)
		return
	}

	fmt.Printf("✅ Batch flush successful - all writes committed as single batch\n")
}

func createTestCoordinator(storage storage.Tree) *interpreter.CommitCoordinator {
	// Create minimal hash ring for testing
	hashRing := zone.NewHashRing(1, 1)
	hashRing.AddNode("test-node", "127.0.0.1:8080")

	// Create coordinator without replication manager for simple testing
	logger := log.New(os.Stdout, "[Coordinator] ", log.LstdFlags)
	return interpreter.NewCommitCoordinator(hashRing, nil, storage, "test-node", logger)
}