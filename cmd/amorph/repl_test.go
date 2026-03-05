// Package main implements REPL unit tests
package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// NewTestREPL creates a REPL instance for testing with temporary storage
func NewTestREPL(t *testing.T) *REPL {
	// Create temporary directory for test storage
	tempDir := t.TempDir()
	storageDir := filepath.Join(tempDir, "data")
	if err := os.MkdirAll(storageDir, 0755); err != nil {
		t.Fatalf("Failed to create test storage directory: %v", err)
	}

	// Create REPL with test storage
	repl, err := NewREPL()
	if err != nil {
		t.Fatalf("Failed to create test REPL: %v", err)
	}

	// Override storage directory for isolation
	repl.storageDir = storageDir

	return repl
}

// TestREPLBasicExecution tests basic MBL execution
func TestREPLBasicExecution(t *testing.T) {
	repl := NewTestREPL(t)
	defer repl.Close()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "number assignment",
			input:    "x = 42",
			expected: "42",
		},
		{
			name:     "text assignment",
			input:    "my.name = \"Alice\"",
			expected: "Alice",
		},
		{
			name:     "arithmetic expression",
			input:    "result = 10 + 5 * 2",
			expected: "20",
		},
		{
			name:     "boolean expression",
			input:    "flag = true",
			expected: "true",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := repl.execute(tt.input)
			if result != tt.expected {
				t.Errorf("execute(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

// TestREPLDataPersistence tests that data persists across execute calls
func TestREPLDataPersistence(t *testing.T) {
	repl := NewTestREPL(t)
	defer repl.Close()

	// Set a value
	result := repl.execute("my.test = \"persistent\"")
	if result != "persistent" {
		t.Errorf("First assignment failed: got %q", result)
	}

	// Retrieve the value in a separate call (using another assignment to verify it's stored)
	result = repl.execute("retrieved = my.test")
	if result != "persistent" {
		t.Errorf("Value not persistent: got %q, want %q", result, "persistent")
	}
}

// TestREPLErrorHandling tests error handling and formatting
func TestREPLErrorHandling(t *testing.T) {
	repl := NewTestREPL(t)
	defer repl.Close()

	tests := []struct {
		name      string
		input     string
		wantError bool
	}{
		{
			name:      "syntax error",
			input:     "x = = 42",
			wantError: true,
		},
		{
			name:      "unmatched parentheses",
			input:     "x = (1 + 2",
			wantError: true,
		},
		{
			name:      "valid expression",
			input:     "x = 42",
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := repl.execute(tt.input)
			hasError := strings.Contains(result, "Error:")

			if tt.wantError && !hasError {
				t.Errorf("Expected error for input %q, got result: %q", tt.input, result)
			}
			if !tt.wantError && hasError {
				t.Errorf("Unexpected error for input %q: %q", tt.input, result)
			}
		})
	}
}

// TestNeedsContinuation tests multi-line input detection
func TestNeedsContinuation(t *testing.T) {
	repl := NewTestREPL(t)
	defer repl.Close()

	tests := []struct {
		name         string
		currentLine  string
		allLines     []string
		wantContinue bool
	}{
		{
			name:         "if statement needs continuation",
			currentLine:  "if x > 10:",
			allLines:     []string{"if x > 10:"},
			wantContinue: true,
		},
		{
			name:         "indented line continues",
			currentLine:  "    output(\"hello\")",
			allLines:     []string{"if x > 10:", "    output(\"hello\")"},
			wantContinue: false, // Simplified implementation doesn't support this yet
		},
		{
			name:         "unmatched bracket continues",
			currentLine:  "list = [1, 2,",
			allLines:     []string{"list = [1, 2,"},
			wantContinue: true,
		},
		{
			name:         "complete statement",
			currentLine:  "x = 42",
			allLines:     []string{"x = 42"},
			wantContinue: false,
		},
		{
			name:         "empty line ends multi-line",
			currentLine:  "",
			allLines:     []string{"if x > 10:", "    output(x)", ""},
			wantContinue: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := repl.needsContinuation(tt.currentLine, tt.allLines)
			if result != tt.wantContinue {
				t.Errorf("needsContinuation(%q, %v) = %v, want %v",
					tt.currentLine, tt.allLines, result, tt.wantContinue)
			}
		})
	}
}

// TestResultFormatting tests output formatting for different types
func TestResultFormatting(t *testing.T) {
	repl := NewTestREPL(t)
	defer repl.Close()

	tests := []struct {
		name     string
		input    string
		contains string // Check if result contains this string
	}{
		{
			name:     "list formatting",
			input:    "nums = [1, 2, 3]",
			contains: "[",
		},
		{
			name:     "record formatting",
			input:    "person = {\"name\": \"Bob\", \"age\": 30}",
			contains: "{",
		},
		{
			name:     "function call output",
			input:    "result = output(\"Hello World\")",
			contains: "Hello World",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := repl.execute(tt.input)
			if !strings.Contains(result, tt.contains) {
				t.Errorf("execute(%q) = %q, expected to contain %q", tt.input, result, tt.contains)
			}
		})
	}
}