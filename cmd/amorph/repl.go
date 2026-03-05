// Package main implements AmorphDB REPL core functionality
package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/solifugus/amorphdb/internal/mbl/interpreter"
	"github.com/solifugus/amorphdb/internal/mbl/lexer"
	"github.com/solifugus/amorphdb/internal/mbl/parser"
	"github.com/solifugus/amorphdb/internal/storage"
)

// REPL represents the Read-Eval-Print Loop implementation
type REPL struct {
	tree        storage.Tree              // Persistent storage
	interpreter *interpreter.Interpreter // MBL execution engine
	scanner     *bufio.Scanner            // Input reading
	agentID     uint64                    // Default agent identity
	storageDir  string                    // Storage location
	homeDir     string                    // User's AmorphDB directory
}

// NewREPL creates a new REPL instance with initialized storage and interpreter
func NewREPL() (*REPL, error) {
	// Get user's home directory
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get user home directory: %w", err)
	}

	// Create ~/.amorph directory
	amorphDir := filepath.Join(homeDir, ".amorph")
	if err := os.MkdirAll(amorphDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create AmorphDB directory: %w", err)
	}

	// Create data subdirectory for storage
	storageDir := filepath.Join(amorphDir, "data")
	if err := os.MkdirAll(storageDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create storage directory: %w", err)
	}

	// Initialize storage tree
	tree, err := storage.NewStorageTree(storageDir)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize storage: %w", err)
	}

	// Default agent ID for REPL sessions
	const defaultAgentID uint64 = 1000

	// Initialize MBL interpreter
	interp := interpreter.New(tree, defaultAgentID)

	// Create scanner for input reading
	scanner := bufio.NewScanner(os.Stdin)

	return &REPL{
		tree:        tree,
		interpreter: interp,
		scanner:     scanner,
		agentID:     defaultAgentID,
		storageDir:  storageDir,
		homeDir:     amorphDir,
	}, nil
}

// Start begins the interactive REPL session
func (r *REPL) Start() {
	// Welcome message
	fmt.Println("AmorphDB REPL - MBL Interactive Shell")
	fmt.Printf("Storage: %s\n", r.storageDir)
	fmt.Printf("Agent ID: %d\n", r.agentID)
	fmt.Println()
	fmt.Println("Type 'exit' to quit, 'help' for help.")
	fmt.Println()

	// Main REPL loop
	for {
		// Read input (handles multi-line)
		input := r.readInput()

		// Handle special commands
		switch strings.TrimSpace(input) {
		case "exit", "quit", ":q":
			fmt.Println("Goodbye!")
			return
		case "help", "?":
			r.showHelp()
			continue
		case "":
			continue // Empty input, show prompt again
		}

		// Execute MBL statement
		result := r.execute(input)
		if result != "" {
			fmt.Println(result)
		}
	}
}

// execute processes MBL input through the lexer→parser→interpreter pipeline
func (r *REPL) execute(input string) string {
	// 1. Tokenize input with MBL lexer
	l := lexer.New(input)

	// 2. Parse tokens into AST with MBL parser
	p := parser.New(l)
	program := p.ParseProgram()

	// Check for parse errors
	if len(p.Errors()) > 0 {
		return r.formatParseErrors(p.Errors())
	}

	// 3. Interpret AST against storage
	result, err := r.interpreter.Interpret(program)
	if err != nil {
		return fmt.Sprintf("Runtime Error: %v", err)
	}

	// 4. Format result for user display
	return r.formatResult(result)
}

// formatParseErrors formats parser errors for user display
func (r *REPL) formatParseErrors(errors []string) string {
	if len(errors) == 1 {
		return fmt.Sprintf("Syntax Error: %s", errors[0])
	}

	var builder strings.Builder
	builder.WriteString("Syntax Errors:\n")
	for i, err := range errors {
		builder.WriteString(fmt.Sprintf("  %d. %s\n", i+1, err))
	}
	return strings.TrimSpace(builder.String())
}

// showHelp displays REPL help information
func (r *REPL) showHelp() {
	fmt.Println("AmorphDB REPL Help:")
	fmt.Println("  exit, quit, :q    Exit the REPL")
	fmt.Println("  help, ?           Show this help")
	fmt.Println()
	fmt.Println("MBL Examples:")
	fmt.Println("  x = 42             Assign a number")
	fmt.Println("  my.name = \"Alice\"   Assign to a path")
	fmt.Println("  output(my.name)    Call built-in function")
	fmt.Println("  if x > 40:         Multi-line if statement")
	fmt.Println("      output(\"big\")    (continue with indentation)")
	fmt.Println()
	fmt.Println("  list = [1, 2, 3]   Create a list")
	fmt.Println("  data = {name: \"Bob\", age: 25}  Create a record")
	fmt.Println()
}

// Close cleans up REPL resources
func (r *REPL) Close() error {
	// The storage tree should handle its own cleanup
	// when it goes out of scope, but we can add explicit
	// cleanup here if needed in the future
	return nil
}