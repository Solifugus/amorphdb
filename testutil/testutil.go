// Package testutil provides shared test infrastructure for AmorphDB testing
package testutil

import (
	"fmt"
	"log"
	"os"
	"testing"
	"time"

	"github.com/solifugus/amorphdb/internal/mbl/lexer"
	"github.com/solifugus/amorphdb/internal/mbl/parser"
	"github.com/solifugus/amorphdb/internal/storage"
	"github.com/solifugus/amorphdb/pkg/client"
)

// TestNode represents a running AmorphDB instance for testing
type TestNode struct {
	DataDir string
	Storage storage.Tree
	Client  *client.Client
	cleanup func()
}

// StartNode starts a fresh amorphd instance with temp data directory
// Returns a connected client and cleanup function
func StartNode(t *testing.T) *TestNode {
	t.Helper()

	// Create temp directory
	tempDir := t.TempDir()

	// Create storage instance
	tree, err := storage.NewStorageTree(tempDir)
	if err != nil {
		t.Fatalf("Failed to create storage tree: %v", err)
	}

	// For now, we'll just return the storage tree
	// TODO: When daemon implementation is ready, start actual amorphd process
	return &TestNode{
		DataDir: tempDir,
		Storage: tree,
		Client:  nil, // TODO: Connect actual client when daemon is ready
		cleanup: func() {
			if tree != nil {
				tree.Close()
			}
		},
	}
}

// StartMesh starts n nodes and connects them into a mesh
// Returns slice of test nodes
func StartMesh(t *testing.T, n int) []*TestNode {
	t.Helper()

	nodes := make([]*TestNode, n)
	for i := 0; i < n; i++ {
		nodes[i] = StartNode(t)
	}

	// TODO: When mesh implementation is ready, connect nodes into mesh
	return nodes
}

// Close cleans up the test node
func (tn *TestNode) Close() {
	if tn.cleanup != nil {
		tn.cleanup()
	}
}

// WaitForHeartbeat blocks until at least one heartbeat cycle completes
func WaitForHeartbeat(t *testing.T) {
	t.Helper()
	// TODO: When heartbeat system is implemented, wait for actual heartbeat
	// For now, just a short sleep to simulate heartbeat timing
	time.Sleep(10 * time.Millisecond)
}

// CreateAgent creates a mobile agent with the given name
// Returns auth credentials
func CreateAgent(t *testing.T, client *client.Client, name string) string {
	t.Helper()
	// TODO: Implement when agent system is ready
	return fmt.Sprintf("agent-%s-credentials", name)
}

// RunMBL sends an MBL program to the client for execution
// Returns output and error
func RunMBL(t *testing.T, client *client.Client, program string) (interface{}, error) {
	t.Helper()
	// TODO: Implement when client-server communication is ready

	// For now, create a simple interpreter for direct execution
	l := lexer.New(program)
	p := parser.New(l)
	ast := p.ParseProgram()

	if len(p.Errors()) > 0 {
		return nil, fmt.Errorf("parse errors: %v", p.Errors())
	}

	// Create minimal interpreter setup
	// TODO: Use proper interpreter when available
	return ast, nil
}

// AssertEventually polls fn until true or timeout (for async mesh operations)
func AssertEventually(t *testing.T, timeout time.Duration, fn func() bool, msg string) {
	t.Helper()

	start := time.Now()
	for time.Since(start) < timeout {
		if fn() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}

	t.Fatalf("Timeout after %v: %s", timeout, msg)
}

// CreateTestStorage creates a storage tree for testing with temp directory
func CreateTestStorage(t *testing.T) (storage.Tree, func()) {
	t.Helper()

	tempDir := t.TempDir()
	tree, err := storage.NewStorageTree(tempDir)
	if err != nil {
		t.Fatalf("Failed to create test storage: %v", err)
	}

	return tree, func() {
		tree.Close()
	}
}

// ExecuteProgram executes MBL code using a storage tree directly
func ExecuteProgram(t *testing.T, tree storage.Tree, code string) (interface{}, error) {
	t.Helper()

	// Parse the code
	l := lexer.New(code)
	p := parser.New(l)
	program := p.ParseProgram()

	// Check for parser errors
	if len(p.Errors()) > 0 {
		return nil, fmt.Errorf("parse errors: %v", p.Errors())
	}

	// Create minimal coordinator for testing (when available)
	// For now just return parsed AST
	// TODO: Use actual interpreter when coordinator is stable
	return program, nil
}

// CreateTestLogger creates a logger for testing
func CreateTestLogger() *log.Logger {
	return log.New(os.Stdout, "[TEST] ", log.LstdFlags)
}