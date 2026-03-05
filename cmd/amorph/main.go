// Package main implements the AmorphDB REPL (Read-Eval-Print Loop)
package main

import (
	"fmt"
	"os"
)

func main() {
	// Create REPL instance
	repl, err := NewREPL()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error initializing AmorphDB: %v\n", err)
		os.Exit(1)
	}

	// Ensure proper cleanup on exit
	defer repl.Close()

	// Start interactive REPL
	repl.Start()
}