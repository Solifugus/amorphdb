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
	tree        storage.Tree             // Persistent storage (local or remote)
	client      *ProtocolClient          // Protocol client for remote connections
	interpreter *interpreter.Interpreter // MBL execution engine
	scanner     *bufio.Scanner           // Input reading
	agentID     uint64                   // Default agent identity
	agentName   string                   // Agent identity name
	homeDir     string                   // User's AmorphDB directory
	isRemote    bool                     // Whether using remote connection
	address     string                   // Connection address
}

// NewREPL creates a new REPL instance with connection to AmorphDB service
func NewREPL(address, identity string) (*REPL, error) {
	// Connect to AmorphDB service via protocol
	client, err := NewProtocolClient(address)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to AmorphDB service: %w", err)
	}

	return NewREPLWithClient(client, identity)
}

// NewREPLWithClient creates a new REPL instance with an existing client
func NewREPLWithClient(client *ProtocolClient, identity string) (*REPL, error) {
	// Get user's home directory
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get user home directory: %w", err)
	}

	amorphDir := filepath.Join(homeDir, ".amorph")

	// Default agent configuration
	agentID := uint64(1000)
	agentName := identity
	if agentName == "" {
		agentName = "anonymous"
	}

	// Ask the service who this connection is authenticated as, so `my.*`
	// resolves under the correct home. Local socket connections are auto-authed
	// as the node owner; on any error (older service, network peer) we keep the
	// default anonymous identity.
	if id, name, err := client.WhoAmI(); err == nil && id != 0 {
		agentID = id
		if name != "" {
			agentName = name
		}
	}

	// Initialize MBL interpreter with protocol client as storage
	interp := interpreter.New(client, agentID)

	// Create scanner for input reading
	scanner := bufio.NewScanner(os.Stdin)

	return &REPL{
		tree:        client, // Protocol client implements storage.Tree interface
		client:      client,
		interpreter: interp,
		scanner:     scanner,
		agentID:     agentID,
		agentName:   agentName,
		homeDir:     amorphDir,
		isRemote:    true, // All connections now go through service
	}, nil
}

// Start begins the interactive REPL session
func (r *REPL) Start() {
	// Welcome message
	fmt.Println("AmorphDB REPL - MBL Interactive Shell")
	fmt.Printf("Agent: %s (ID: %d)\n", r.agentName, r.agentID)
	if r.isRemote {
		fmt.Printf("Connected to service via protocol\n")
	}
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

		// Check for display hints before executing
		displayHint, cleanInput := r.parseDisplayHint(input)

		// Execute MBL statement
		result := r.executeWithHint(cleanInput, displayHint)
		if result != "" {
			fmt.Println(result)
		}
	}
}

// DisplayHint represents formatting hints for REPL output
type DisplayHint string

const (
	DisplayAuto  DisplayHint = "auto"  // Auto-inferred based on data structure
	DisplayTree  DisplayHint = "tree"  // Force tree view (indented hierarchy)
	DisplayTable DisplayHint = "table" // Force table view
	DisplayList  DisplayHint = "list"  // Force list view (one item per line)
)

// parseDisplayHint extracts display hint from input and returns clean input
func (r *REPL) parseDisplayHint(input string) (DisplayHint, string) {
	trimmed := strings.TrimSpace(input)

	// Look for " :hint" pattern at the end of the input
	// Use regex or simple string matching to find " :tree", " :table", " :list"
	if strings.Contains(trimmed, " :tree") && strings.HasSuffix(trimmed, " :tree") {
		cleanInput := strings.TrimSpace(strings.TrimSuffix(trimmed, " :tree"))
		return DisplayTree, cleanInput
	}
	if strings.Contains(trimmed, " :table") && strings.HasSuffix(trimmed, " :table") {
		cleanInput := strings.TrimSpace(strings.TrimSuffix(trimmed, " :table"))
		return DisplayTable, cleanInput
	}
	if strings.Contains(trimmed, " :list") && strings.HasSuffix(trimmed, " :list") {
		cleanInput := strings.TrimSpace(strings.TrimSuffix(trimmed, " :list"))
		return DisplayList, cleanInput
	}

	// No display hint found - use auto-inferred formatting
	return DisplayAuto, input
}

// executeWithHint processes MBL input with a display hint
func (r *REPL) executeWithHint(input string, hint DisplayHint) string {
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

	// 4. Format result with display hint
	return r.formatResultWithHint(result, hint)
}

// execute processes MBL input through the lexer→parser→interpreter pipeline
func (r *REPL) execute(input string) string {
	// Check for display hints first
	displayHint, cleanInput := r.parseDisplayHint(input)

	// Use executeWithHint to handle the cleaned input
	return r.executeWithHint(cleanInput, displayHint)
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
	if r.client != nil {
		return r.client.Close()
	}
	return nil
}
