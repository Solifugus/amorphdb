package ari

import (
	"strings"
	"testing"
)

func TestEngine_SimpleFile(t *testing.T) {
	// Simple 5-line test file
	input := `ID: 12345
NAME: Alice
TOTAL: $500.00`

	// Minimal ARI spec that works with current parser - just test basic structure
	ariSpec := `section report:`

	// Parse ARI specification
	lexer := NewLexer(ariSpec)
	parser := NewParser(lexer)
	spec := parser.ParseARISpec()

	if len(parser.Errors()) != 0 {
		t.Fatalf("ARI parser errors: %v", parser.Errors())
	}

	// Process the input
	engine := NewEngine(spec)
	reader := strings.NewReader(input)

	result, err := engine.ProcessFile(reader, int64(len(input)))
	if err != nil {
		t.Fatalf("Engine processing failed: %v", err)
	}

	// Verify result is not nil
	if result == nil {
		t.Fatalf("Expected result, got nil")
	}

	t.Logf("Processed result: %+v", result)
}

func TestEngine_FileSizeLimit(t *testing.T) {
	// Test file size limit enforcement
	engine := NewEngine(&ARISpec{})
	reader := strings.NewReader("test")

	// Try with size exceeding limit
	_, err := engine.ProcessFile(reader, MaxFileSize+1)
	if err == nil {
		t.Fatalf("Expected file size error, got none")
	}

	if !strings.Contains(err.Error(), "#file_too_large") {
		t.Fatalf("Expected file_too_large error, got: %v", err)
	}
}

func TestEngine_EmptyFile(t *testing.T) {
	// Test empty file handling
	ariSpec := `section empty:`

	lexer := NewLexer(ariSpec)
	parser := NewParser(lexer)
	spec := parser.ParseARISpec()

	if len(parser.Errors()) != 0 {
		t.Fatalf("ARI parser errors: %v", parser.Errors())
	}

	engine := NewEngine(spec)
	reader := strings.NewReader("")

	_, err := engine.ProcessFile(reader, 0)
	if err == nil {
		t.Fatalf("Expected error for empty file, got none")
	}

	// Engine should return invalid_range error for empty files
	if !strings.Contains(err.Error(), "#invalid_range") {
		t.Fatalf("Expected invalid_range error, got: %v", err)
	}
}

func TestEngine_LongLineDetection(t *testing.T) {
	// Test long line detection
	ariSpec := `section test:`

	lexer := NewLexer(ariSpec)
	parser := NewParser(lexer)
	spec := parser.ParseARISpec()

	engine := NewEngine(spec)

	// Create a line that exceeds MaxLineLength
	longLine := strings.Repeat("x", MaxLineLength+1)
	reader := strings.NewReader(longLine)

	_, err := engine.ProcessFile(reader, int64(len(longLine)))
	if err == nil {
		t.Fatalf("Expected read_error error, got none")
	}

	// Engine returns bufio.Scanner error for overly long lines
	if !strings.Contains(err.Error(), "#read_error: bufio.Scanner: token too long") {
		t.Fatalf("Expected read_error: bufio.Scanner: token too long error, got: %v", err)
	}
}

func TestEngine_TooManyLines(t *testing.T) {
	// Test too many lines detection
	ariSpec := `section test:`

	lexer := NewLexer(ariSpec)
	parser := NewParser(lexer)
	spec := parser.ParseARISpec()

	engine := NewEngine(spec)

	// Create input with more than MaxLines
	lines := make([]string, MaxLines+1)
	for i := range lines {
		lines[i] = "test line"
	}
	input := strings.Join(lines, "\n")
	reader := strings.NewReader(input)

	_, err := engine.ProcessFile(reader, int64(len(input)))
	if err == nil {
		t.Fatalf("Expected too_many_lines error, got none")
	}

	if !strings.Contains(err.Error(), "#too_many_lines") {
		t.Fatalf("Expected too_many_lines error, got: %v", err)
	}
}