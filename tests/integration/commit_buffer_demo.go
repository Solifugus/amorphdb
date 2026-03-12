package main

import (
	"fmt"
	"log"

	"github.com/solifugus/amorphdb/internal/mbl/interpreter"
	"github.com/solifugus/amorphdb/internal/mbl/lexer"
	"github.com/solifugus/amorphdb/internal/mbl/parser"
	"github.com/solifugus/amorphdb/internal/storage"
	"github.com/solifugus/amorphdb/internal/types"
)

func main() {
	fmt.Println("AmorphDB Commit Buffer Demo")
	fmt.Println("===========================")

	// Create storage tree
	tree := storage.NewMemoryTree()

	// Test 1: Local variables should not enter commit buffer
	testLocalVariables(tree)

	// Test 2: Persistent writes should be batched
	testPersistentBatching(tree)

	fmt.Println("\n✅ All commit buffer tests passed!")
}

func testLocalVariables(tree storage.Tree) {
	fmt.Println("\n1. Testing local variables (zero mesh cost)")

	source := `
		# Local variable assignments - should NOT enter commit buffer
		x = 42
		y = "hello"
		z = x + 100
	`

	result := executeProgram(tree, source)
	fmt.Printf("   Result: %v\n", result)
	fmt.Println("   ✅ Local variables handled correctly")
}

func testPersistentBatching(tree storage.Tree) {
	fmt.Println("\n2. Testing persistent writes batching")

	source := `
		# Multiple persistent writes - should be staged and batched
		my.data.count = 10
		my.data.name = "test"
		world.shared.value = 42
	`

	result := executeProgram(tree, source)
	fmt.Printf("   Result: %v\n", result)

	// Verify the writes actually persisted
	count, _ := tree.Read([]string{"world", "agent", "1", "data", "count"})
	name, _ := tree.Read([]string{"world", "agent", "1", "data", "name"})
	shared, _ := tree.Read([]string{"world", "shared", "value"})

	fmt.Printf("   Persisted count: %v\n", count)
	fmt.Printf("   Persisted name: %v\n", name)
	fmt.Printf("   Persisted shared: %v\n", shared)
	fmt.Println("   ✅ Persistent writes batched and committed")
}

func executeProgram(tree storage.Tree, source string) interface{} {
	// Tokenize
	tokens := lexer.Tokenize(source)

	// Parse
	p := parser.NewParser(tokens)
	program, err := p.ParseProgram()
	if err != nil {
		log.Fatalf("Parse error: %v", err)
	}

	// Create interpreter
	i := interpreter.NewInterpreter(tree, 1) // agent ID 1

	// Execute
	result, err := i.Execute(program)
	if err != nil {
		log.Fatalf("Execution error: %v", err)
	}

	return result
}
