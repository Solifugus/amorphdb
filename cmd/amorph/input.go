// Package main implements REPL input handling with multi-line support
package main

import (
	"fmt"
	"strings"
)

// readInput handles input with full multi-line support
func (r *REPL) readInput() string {
	var lines []string
	prompt := "amorph> "

	for {
		fmt.Print(prompt)

		// Read line from scanner
		if !r.scanner.Scan() {
			// EOF or error - return what we have so far
			break
		}

		currentLine := r.scanner.Text()
		lines = append(lines, currentLine)

		// Check if we need continuation
		if !r.needsContinuation(currentLine, lines) {
			break
		}

		// Use continuation prompt for next line
		prompt = "     | "
	}

	// Join all lines and return
	return strings.Join(lines, "\n")
}

// needsContinuation determines if input requires more lines
func (r *REPL) needsContinuation(currentLine string, allLines []string) bool {
	trimmed := strings.TrimSpace(currentLine)

	// Empty line ends multi-line input
	if trimmed == "" && len(allLines) > 1 {
		return false
	}

	// Check for unmatched brackets across all lines
	if r.hasUnmatchedBrackets(allLines) {
		return true
	}

	// Check for control flow statements ending with ":"
	if strings.HasSuffix(trimmed, ":") {
		keywords := []string{"if", "elif", "else", "while", "for", "consider", "procedure", "watch"}
		for _, keyword := range keywords {
			if strings.HasPrefix(trimmed, keyword+" ") ||
			   strings.HasPrefix(trimmed, keyword+"(") ||
			   trimmed == keyword+":" {
				return true
			}
		}

		// Also handle procedure definitions (identifier:) and other colon endings
		if len(trimmed) > 1 {
			return true
		}
	}

	// Check for indented continuation
	if len(allLines) >= 2 {
		// If we're in a multi-line block, check indentation
		return r.isIndentedContinuation(currentLine, allLines)
	}

	return false
}

// isIndentedContinuation checks if current line continues an indented block
func (r *REPL) isIndentedContinuation(currentLine string, allLines []string) bool {
	if len(allLines) < 2 {
		return false
	}

	// Get the indentation level of the current line
	currentIndent := r.getIndentationLevel(currentLine)

	// Find the base indentation level (first line that's not empty and not a header)
	baseIndent := -1
	blockIndent := -1

	for i, line := range allLines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue // Skip empty lines
		}

		indent := r.getIndentationLevel(line)

		if i == 0 || (baseIndent == -1 && !strings.HasSuffix(trimmed, ":")) {
			// First non-empty line or first non-header line sets base indentation
			baseIndent = indent
		} else if blockIndent == -1 && indent > baseIndent {
			// First indented line sets block indentation
			blockIndent = indent
		}
	}

	// If we haven't established a block indentation yet, check if current line starts one
	if blockIndent == -1 {
		// If current line is indented relative to base, continue
		return currentIndent > baseIndent
	}

	// If current line is at block level or deeper, continue
	// If current line is back to base level or less, stop
	return currentIndent >= blockIndent
}

// getIndentationLevel returns the number of leading tabs in a line
func (r *REPL) getIndentationLevel(line string) int {
	count := 0
	for _, ch := range line {
		if ch == '\t' {
			count++
		} else {
			break
		}
	}
	return count
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