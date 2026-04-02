package ari

import (
	"bufio"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
)

const (
	// Maximum file size for import (10MB) to prevent OOM
	MaxFileSize = 10 * 1024 * 1024

	// Maximum search depth to prevent infinite recursion
	MaxSearchDepth = 1000

	// Maximum line length to prevent unbounded scanning
	MaxLineLength = 1024 * 1024

	// Maximum lines to process
	MaxLines = 100000
)

// Engine processes ARI specifications against input files
type Engine struct {
	spec      *ARISpec
	lines     []string
	maxLines  int
	maxCols   int
	processed map[string]interface{}
}

// NewEngine creates a new ARI processing engine with bounds checking
func NewEngine(spec *ARISpec) *Engine {
	return &Engine{
		spec:      spec,
		processed: make(map[string]interface{}),
	}
}

// ProcessFile applies the ARI specification to a file
func (e *Engine) ProcessFile(reader io.Reader, maxSize int64) (interface{}, error) {
	// Check file size limit
	if maxSize > MaxFileSize {
		return nil, fmt.Errorf("#file_too_large: file size %d exceeds maximum %d", maxSize, MaxFileSize)
	}

	// Read file with line count and size limits
	scanner := bufio.NewScanner(reader)
	lines := make([]string, 0)
	lineCount := 0
	totalSize := 0

	for scanner.Scan() {
		lineCount++
		line := scanner.Text()

		// Prevent OOM from extremely long lines
		if len(line) > MaxLineLength {
			return nil, fmt.Errorf("#line_too_long: line %d exceeds maximum length %d", lineCount, MaxLineLength)
		}

		// Prevent OOM from too many lines
		if lineCount > MaxLines {
			return nil, fmt.Errorf("#too_many_lines: file has more than %d lines", MaxLines)
		}

		totalSize += len(line)
		if totalSize > MaxFileSize {
			return nil, fmt.Errorf("#file_too_large: total content size exceeds maximum %d", MaxFileSize)
		}

		lines = append(lines, line)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("#read_error: %v", err)
	}

	e.lines = lines
	e.maxLines = len(lines)

	// Find maximum column width for bounds checking
	e.maxCols = 0
	for _, line := range lines {
		if len(line) > e.maxCols {
			e.maxCols = len(line)
		}
	}

	// Process all sections
	result := make(map[string]interface{})

	for _, section := range e.spec.Sections {
		sectionResult, err := e.processSection(&section, 0, len(lines)-1)
		if err != nil {
			return nil, err
		}
		result[section.Name] = sectionResult
	}

	return result, nil
}

// processSection processes a single section with bounds checking
func (e *Engine) processSection(section *Section, startLine, endLine int) (interface{}, error) {
	// Guard against invalid line ranges
	if startLine < 0 {
		startLine = 0
	}
	if endLine >= e.maxLines {
		endLine = e.maxLines - 1
	}
	if startLine > endLine {
		return nil, fmt.Errorf("#invalid_range: start line %d > end line %d", startLine, endLine)
	}

	// Find section boundaries if patterns are specified
	actualStart := startLine
	actualEnd := endLine

	if section.StartsWith != nil {
		foundStart, err := e.findPattern(section.StartsWith, startLine, endLine, true)
		if err != nil {
			return nil, err
		}
		if foundStart >= 0 {
			actualStart = foundStart
		}
	}

	if section.EndsWith != nil {
		foundEnd, err := e.findPattern(section.EndsWith, actualStart, endLine, false)
		if err != nil {
			return nil, err
		}
		if foundEnd >= 0 {
			actualEnd = foundEnd
		}
	}

	// Process fields within the section bounds
	var records []map[string]interface{}

	if section.Break != nil {
		// Section has break rules - handle repeating records
		records = e.processRepeatingRecords(section, actualStart, actualEnd)
	} else {
		// Section without break - create single record with all fields
		record := make(map[string]interface{})

		// Search for each field independently within the section bounds
		for _, field := range section.Fields {
			value, foundLine, err := e.processField(&field, actualStart, actualEnd)
			if err != nil {
				return nil, err
			}
			if foundLine >= 0 {
				record[field.Name] = value
			}
		}

		// Only add record if at least one field was found
		if len(record) > 0 {
			records = append(records, record)
		}
	}

	// Process nested sections
	for _, nestedSection := range section.Sections {
		nestedResult, err := e.processSection(&nestedSection, actualStart, actualEnd)
		if err != nil {
			return nil, err
		}

		// Add nested section result to the first record or create a new one
		if len(records) > 0 {
			records[0][nestedSection.Name] = nestedResult
		} else {
			records = append(records, map[string]interface{}{
				nestedSection.Name: nestedResult,
			})
		}
	}

	if len(records) == 0 {
		return nil, nil
	} else if len(records) == 1 {
		return records[0], nil
	} else {
		return records, nil
	}
}

// processRepeatingRecords handles sections with break rules that contain repeating records
func (e *Engine) processRepeatingRecords(section *Section, startLine, endLine int) []map[string]interface{} {
	records := make([]map[string]interface{}, 0)
	currentLine := startLine

	// Set reasonable iteration limits
	maxIterations := (endLine - startLine + 1) * 10
	if maxIterations < 100 {
		maxIterations = 100
	}
	if maxIterations > 10000 {
		maxIterations = 10000
	}
	iterations := 0

	for currentLine <= endLine && iterations < maxIterations {
		iterations++

		record := make(map[string]interface{})
		recordProcessed := false

		// Process all fields at current position
		for _, field := range section.Fields {
			value, foundLine, err := e.processField(&field, currentLine, endLine)
			if err != nil {
				continue // Skip field on error
			}
			if foundLine >= 0 {
				record[field.Name] = value
				recordProcessed = true
			}
		}

		// Add record if any fields were found
		if recordProcessed {
			records = append(records, record)
		}

		// Handle break conditions
		if section.Break != nil {
			nextLine, err := e.handleBreak(section.Break, currentLine, endLine)
			if err != nil {
				break
			}
			if nextLine > currentLine {
				currentLine = nextLine
			} else {
				currentLine++
			}
		} else {
			currentLine++
		}

		// Safety check
		if iterations >= maxIterations {
			break
		}
	}

	return records
}

// processField extracts a field value with directional anchoring and bounds checking
func (e *Engine) processField(field *Field, startLine, endLine int) (interface{}, int, error) {
	// Guard against invalid parameters
	if startLine < 0 || startLine >= e.maxLines {
		return nil, -1, nil
	}
	if endLine >= e.maxLines {
		endLine = e.maxLines - 1
	}

	// For fields with anchors, we need to search for anchor patterns first
	// Then apply the directional movements
	if len(field.Anchors) == 0 {
		// No anchors - try patterns at start position
		for _, pattern := range field.Patterns {
			value, found := e.extractPattern(&pattern, startLine, 0)
			if found {
				return value, startLine, nil
			}
		}
		return nil, -1, nil
	}

	// Search through the section for field patterns that could serve as anchors
	for searchLine := startLine; searchLine <= endLine; searchLine++ {
		if searchLine >= e.maxLines {
			break
		}

		currentLine := e.lines[searchLine]

		// Try to find field patterns in this line to use as reference points
		for _, pattern := range field.Patterns {
			// Search for pattern in this line
			for col := 0; col < len(currentLine); col++ {
				if e.matchesPatternAtPosition(&pattern, searchLine, col) {
					// Found a potential pattern match, now apply anchors
					finalLine, finalCol, err := e.applyAnchorsToPosition(field.Anchors, searchLine, col)
					if err != nil {
						continue // Try next position
					}

					// Bounds check
					if finalLine < 0 || finalLine >= e.maxLines {
						continue
					}
					if finalCol < 0 || (finalLine < len(e.lines) && finalCol >= len(e.lines[finalLine])) {
						continue
					}

					// Extract value at the final position - don't use anchor pattern
					value, found := e.extractValueAtPosition(finalLine, finalCol)
					if found {
						return value, finalLine, nil
					}
				}
			}
		}

		// Also try anchor-first approach: apply anchors from current position
		// This handles cases where we need to move to find the pattern
		finalLine, finalCol, err := e.applyAnchorsToPosition(field.Anchors, searchLine, 0)
		if err != nil {
			continue
		}

		// Bounds check
		if finalLine < 0 || finalLine >= e.maxLines {
			continue
		}
		if finalCol < 0 || (finalLine < len(e.lines) && finalCol >= len(e.lines[finalLine])) {
			continue
		}

		// Extract value at the anchored position
		value, found := e.extractValueAtPosition(finalLine, finalCol)
		if found {
			return value, finalLine, nil
		}
	}

	return nil, -1, nil
}

// matchesPatternAtPosition checks if a pattern matches at a specific position
func (e *Engine) matchesPatternAtPosition(pattern *Pattern, line, col int) bool {
	if line < 0 || line >= e.maxLines || col < 0 {
		return false
	}

	currentLine := e.lines[line]
	if col >= len(currentLine) {
		return false
	}

	switch pattern.Type {
	case PatternText:
		text := pattern.Text
		return col+len(text) <= len(currentLine) && currentLine[col:col+len(text)] == text

	case PatternBuiltinDate:
		// Check for date pattern at position
		datePattern := regexp.MustCompile(`\d{4}-\d{2}-\d{2}|\d{2}/\d{2}/\d{4}|\d{2}-\d{2}-\d{4}`)
		searchEnd := col + 50
		if searchEnd > len(currentLine) {
			searchEnd = len(currentLine)
		}
		return datePattern.MatchString(currentLine[col:searchEnd])

	case PatternBuiltinMoney:
		// Check for money pattern at position
		moneyPattern := regexp.MustCompile(`\$[\d,]+\.?\d*`)
		searchEnd := col + 30
		if searchEnd > len(currentLine) {
			searchEnd = len(currentLine)
		}
		return moneyPattern.MatchString(currentLine[col:searchEnd])

	case PatternBuiltinInteger:
		// Check for integer pattern at position
		intPattern := regexp.MustCompile(`\d+`)
		searchEnd := col + 20
		if searchEnd > len(currentLine) {
			searchEnd = len(currentLine)
		}
		return intPattern.MatchString(currentLine[col:searchEnd])

	default:
		return false
	}
}

// applyAnchorsToPosition applies a sequence of anchors to a starting position
func (e *Engine) applyAnchorsToPosition(anchors []Anchor, startLine, startCol int) (int, int, error) {
	currentLine := startLine
	currentCol := startCol

	for _, anchor := range anchors {
		newLine, newCol, err := e.applyAnchor(&anchor, currentLine, currentCol, e.maxLines-1, false)
		if err != nil {
			return currentLine, currentCol, err
		}
		currentLine = newLine
		currentCol = newCol
	}

	return currentLine, currentCol, nil
}

// applyAnchor applies a directional anchor with bounds checking
func (e *Engine) applyAnchor(anchor *Anchor, line, col, endLine int, isFirstAnchor bool) (int, int, error) {
	// Apply directional movement with distance and bounds checking
	switch anchor.Direction {
	case DirectionLeft:
		distance := e.getDistance(&anchor.Distance)
		newCol := col - distance
		if newCol < 0 {
			newCol = 0 // Clamp to line start
		}
		return line, newCol, nil

	case DirectionRight:
		distance := e.getDistance(&anchor.Distance)
		if line < len(e.lines) {
			currentLine := e.lines[line]
			newCol := col + distance
			if newCol >= len(currentLine) {
				newCol = len(currentLine) - 1 // Clamp to line end
			}
			if newCol < 0 {
				newCol = 0
			}
			return line, newCol, nil
		}
		return line, col, nil

	case DirectionUp:
		distance := e.getDistance(&anchor.Distance)
		newLine := line - distance
		if newLine < 0 {
			newLine = 0 // Clamp to file start
		}
		return newLine, col, nil

	case DirectionDown:
		distance := e.getDistance(&anchor.Distance)
		newLine := line + distance
		if newLine >= e.maxLines {
			newLine = e.maxLines - 1 // Clamp to file end
		}
		return newLine, col, nil

	case DirectionSame:
		return line, col, nil

	case DirectionFlush:
		// Flush to edge with bounds checking
		switch {
		case anchor.Distance.Type == DistanceFlush:
			// Flush left (to beginning of line)
			return line, 0, nil
		default:
			// Stay at current position
			return line, col, nil
		}
	}

	return line, col, fmt.Errorf("#invalid_direction: unsupported direction %v", anchor.Direction)
}

// findPattern finds a pattern within line bounds with search limits
func (e *Engine) findPattern(pattern *Pattern, startLine, endLine int, findFirst bool) (int, error) {
	// Bounds checking
	if startLine < 0 {
		startLine = 0
	}
	if endLine >= e.maxLines {
		endLine = e.maxLines - 1
	}

	searchCount := 0
	maxSearches := endLine - startLine + 1

	for line := startLine; line <= endLine && searchCount < MaxSearchDepth; line++ {
		searchCount++

		if e.matchesPattern(pattern, e.lines[line]) {
			return line, nil
		}

		if searchCount >= maxSearches {
			break
		}
	}

	return -1, nil // Not found
}

// extractPattern extracts value using pattern at specific position
func (e *Engine) extractPattern(pattern *Pattern, line, col int) (interface{}, bool) {
	// Bounds checking
	if line < 0 || line >= e.maxLines {
		return nil, false
	}

	currentLine := e.lines[line]
	if col < 0 || col >= len(currentLine) {
		return nil, false
	}

	// Extract based on pattern type
	switch pattern.Type {
	case PatternText:
		// Look for exact text match at position
		text := pattern.Text
		if col+len(text) <= len(currentLine) {
			if currentLine[col:col+len(text)] == text {
				return text, true
			}
		}

	case PatternRegex, PatternRegexWithTransform:
		// Apply regex pattern
		regex, err := regexp.Compile(pattern.Text)
		if err != nil {
			return nil, false
		}

		// Search from current position to end of line (bounded)
		searchEnd := col + 200 // Limit regex search scope
		if searchEnd > len(currentLine) {
			searchEnd = len(currentLine)
		}

		match := regex.FindString(currentLine[col:searchEnd])
		if match != "" {
			return match, true
		}

	case PatternBuiltinDate:
		// Simple date pattern matching (bounded)
		datePattern := regexp.MustCompile(`\d{4}-\d{2}-\d{2}|\d{2}/\d{2}/\d{4}|\d{2}-\d{2}-\d{4}`)
		searchEnd := col + 50 // Reasonable limit for date patterns
		if searchEnd > len(currentLine) {
			searchEnd = len(currentLine)
		}

		match := datePattern.FindString(currentLine[col:searchEnd])
		if match != "" {
			return match, true
		}

	case PatternBuiltinMoney:
		// Simple money pattern (bounded)
		moneyPattern := regexp.MustCompile(`\$[\d,]+\.?\d*`)
		searchEnd := col + 30 // Reasonable limit for money patterns
		if searchEnd > len(currentLine) {
			searchEnd = len(currentLine)
		}

		match := moneyPattern.FindString(currentLine[col:searchEnd])
		if match != "" {
			return match, true
		}

	case PatternBuiltinInteger:
		// Integer pattern (bounded)
		intPattern := regexp.MustCompile(`\d+`)
		searchEnd := col + 20 // Reasonable limit for integer patterns
		if searchEnd > len(currentLine) {
			searchEnd = len(currentLine)
		}

		match := intPattern.FindString(currentLine[col:searchEnd])
		if match != "" {
			if val, err := strconv.Atoi(match); err == nil {
				return val, true
			}
		}
	}

	return nil, false
}

// Helper methods with bounds checking

func (e *Engine) matchesPattern(pattern *Pattern, line string) bool {
	if len(line) > MaxLineLength {
		line = line[:MaxLineLength] // Truncate excessively long lines
	}

	switch pattern.Type {
	case PatternText:
		return strings.Contains(line, pattern.Text)
	case PatternRegex, PatternRegexWithTransform:
		regex, err := regexp.Compile(pattern.Text)
		if err != nil {
			return false
		}
		return regex.MatchString(line)
	default:
		return false
	}
}

func (e *Engine) getDistance(distance *Distance) int {
	switch distance.Type {
	case DistanceExact:
		return distance.Min
	case DistanceRange:
		// Use minimum for simplicity in this basic implementation
		return distance.Min
	case DistanceOpenMin:
		return distance.Min
	case DistanceOpenMax:
		return distance.Max
	default:
		return 1
	}
}

func (e *Engine) handleBreak(breakRule *Break, currentLine, endLine int) (int, error) {
	// Simple break handling with bounds checking
	switch breakRule.Type {
	case BreakOnReMatch:
		// Look for re-match of first field pattern in next lines
		return currentLine + 1, nil
	case BreakOnPattern:
		if breakRule.Pattern != nil {
			nextLine, err := e.findPattern(breakRule.Pattern, currentLine+1, endLine, true)
			if err != nil {
				return currentLine + 1, err
			}
			if nextLine >= 0 {
				return nextLine, nil
			}
		}
		return currentLine + 1, nil
	default:
		return currentLine + 1, nil
	}
}

// extractValueAtPosition extracts a value starting at the given position
// This method tries to intelligently extract various data types
func (e *Engine) extractValueAtPosition(line, col int) (interface{}, bool) {
	if line < 0 || line >= len(e.lines) {
		return nil, false
	}

	currentLine := e.lines[line]
	if col < 0 || col >= len(currentLine) {
		return nil, false
	}

	// Skip leading whitespace
	for col < len(currentLine) && (currentLine[col] == ' ' || currentLine[col] == '\t') {
		col++
	}

	if col >= len(currentLine) {
		return nil, false
	}

	// Try to extract different value types
	remaining := currentLine[col:]

	// Money pattern: $123.45 or $1,234.56
	moneyPattern := regexp.MustCompile(`^\$[\d,]+\.?\d*`)
	if match := moneyPattern.FindString(remaining); match != "" {
		return match, true
	}

	// Date patterns: 2024-03-15, 03/15/2024, 03-15-2024
	datePattern := regexp.MustCompile(`^\d{4}-\d{2}-\d{2}|\d{2}/\d{2}/\d{4}|\d{2}-\d{2}-\d{4}`)
	if match := datePattern.FindString(remaining); match != "" {
		return match, true
	}

	// Decimal number: 123.45
	decimalPattern := regexp.MustCompile(`^\d+\.\d+`)
	if match := decimalPattern.FindString(remaining); match != "" {
		return match, true
	}

	// Integer: 123
	intPattern := regexp.MustCompile(`^\d+`)
	if match := intPattern.FindString(remaining); match != "" {
		return match, true
	}

	// Quoted string: "text"
	if remaining[0] == '"' {
		for i := 1; i < len(remaining); i++ {
			if remaining[i] == '"' {
				return remaining[1:i], true
			}
		}
	}

	// Unquoted word (until whitespace or punctuation)
	wordPattern := regexp.MustCompile(`^[A-Za-z0-9_-]+`)
	if match := wordPattern.FindString(remaining); match != "" {
		return match, true
	}

	return nil, false
}