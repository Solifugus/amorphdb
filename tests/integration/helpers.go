package integration

import (
	"fmt"
	"log"
	"os"

	"github.com/solifugus/amorphdb/internal/mbl/interpreter"
	"github.com/solifugus/amorphdb/internal/mbl/lexer"
	"github.com/solifugus/amorphdb/internal/mbl/parser"
	"github.com/solifugus/amorphdb/internal/storage"
)

// executeProgram executes MBL code using the given interpreter
func executeProgram(interp *interpreter.Interpreter, code string) (interface{}, error) {
	// Parse the code
	l := lexer.New(code)
	p := parser.New(l)
	program := p.ParseProgram()

	// Check for parser errors
	if len(p.Errors()) > 0 {
		return nil, fmt.Errorf("parse errors: %v", p.Errors())
	}

	// Execute the program
	return interp.Interpret(program)
}

// createTestCoordinator creates a coordinator for testing
func createTestCoordinator(storage storage.Tree) *interpreter.CommitCoordinator {
	logger := log.New(os.Stdout, "[Coordinator] ", log.LstdFlags)

	// Note: Using nil for hashRing since zone model is deprecated
	// This is acceptable for testing as coordinator will use subscription model
	return interpreter.NewCommitCoordinator(nil, nil, storage, "test-node", logger)
}