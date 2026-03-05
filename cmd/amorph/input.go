// Package main implements REPL input handling with multi-line support
package main

import (
	"fmt"
	"strings"
)

// readInput handles input with basic continuation support
func (r *REPL) readInput() string {
	fmt.Print("amorph> ")

	// Read line from scanner
	if !r.scanner.Scan() {
		// EOF or error
		return ""
	}

	line := r.scanner.Text()

	// For now, just support single-line input
	// Multi-line support can be added later once basic functionality works
	return strings.TrimSpace(line)
}

// needsContinuation determines if input requires more lines
// Simplified for now - will be enhanced later
func (r *REPL) needsContinuation(currentLine string, allLines []string) bool {
	trimmed := strings.TrimSpace(currentLine)

	// Check for obvious continuation needs
	if strings.HasSuffix(trimmed, ":") {
		// Control statements like if:, while:, etc.
		keywords := []string{"if", "elif", "else", "while", "for", "consider", "procedure"}
		for _, keyword := range keywords {
			if strings.HasPrefix(trimmed, keyword+" ") || trimmed == keyword+":" {
				return true
			}
		}
	}

	// Check for unmatched brackets
	return r.hasUnmatchedBrackets([]string{currentLine})
}

// hasUnmatchedBrackets checks if there are unmatched brackets in the input
func (r *REPL) hasUnmatchedBrackets(lines []string) bool {
	input := strings.Join(lines, "\n")

	// Count different types of brackets
	counts := map[rune]int{
		'(': 0,
		'[': 0,
		'{': 0,
	}

	inString := false
	var stringChar rune

	for _, ch := range input {
		// Handle string literals
		if (ch == '"' || ch == '\'') && !inString {
			inString = true
			stringChar = ch
			continue
		}
		if inString && ch == stringChar {
			inString = false
			continue
		}
		if inString {
			continue
		}

		// Count brackets outside of strings
		switch ch {
		case '(':
			counts['(']++
		case ')':
			counts['(']--
		case '[':
			counts['[']++
		case ']':
			counts['[']--
		case '{':
			counts['{']++
		case '}':
			counts['{']--
		}
	}

	// Check if any bracket type is unmatched
	for _, count := range counts {
		if count != 0 {
			return true
		}
	}

	return false
}