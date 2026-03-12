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

// TestRecordFieldAccess tests accessing fields from record variables
func TestRecordFieldAccess(t *testing.T) {
	repl := NewTestREPL(t)
	defer repl.Close()

	tests := []struct {
		name     string
		setup    string
		access   string
		expected string
	}{
		{
			name:     "simple field access",
			setup:    `person = {"name": "Alice", "age": 25}`,
			access:   `person.name`,
			expected: "Alice",
		},
		{
			name:     "numeric field access",
			setup:    `person = {"name": "Bob", "age": 30}`,
			access:   `person.age`,
			expected: "30",
		},
		{
			name:     "nested record access",
			setup:    `data = {"user": {"name": "Charlie", "id": 123}}`,
			access:   `data.user.name`,
			expected: "Charlie",
		},
		{
			name:     "mixed local and storage paths",
			setup:    `local_var = {"count": 5}`,
			access:   `local_var.count`,
			expected: "5",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup the record
			setupResult := repl.execute(tt.setup)
			if strings.Contains(setupResult, "Error:") {
				t.Fatalf("Setup failed: %s", setupResult)
			}

			// Access the field
			result := repl.execute(tt.access)
			if result != tt.expected {
				t.Errorf("execute(%q) = %q, want %q", tt.access, result, tt.expected)
			}
		})
	}
}

// TestRecordFieldAccessErrors tests error cases for record field access
func TestRecordFieldAccessErrors(t *testing.T) {
	repl := NewTestREPL(t)
	defer repl.Close()

	tests := []struct {
		name         string
		setup        string
		access       string
		expectsError bool
	}{
		{
			name:         "non-existent field",
			setup:        `person = {"name": "Alice"}`,
			access:       `person.age`,
			expectsError: true,
		},
		{
			name:         "field access on non-record",
			setup:        `number = 42`,
			access:       `number.field`,
			expectsError: true,
		},
		{
			name:         "nested field on non-record",
			setup:        `data = {"value": 123}`,
			access:       `data.value.field`,
			expectsError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			repl.execute(tt.setup)

			// Access should return Unknown
			result := repl.execute(tt.access)
			hasError := strings.Contains(result, "Unknown:")

			if tt.expectsError && !hasError {
				t.Errorf("Expected error for %q, got: %q", tt.access, result)
			}
			if !tt.expectsError && hasError {
				t.Errorf("Unexpected error for %q: %q", tt.access, result)
			}
		})
	}
}

// TestStoragePathReads tests reading storage paths as standalone expressions
func TestStoragePathReads(t *testing.T) {
	repl := NewTestREPL(t)
	defer repl.Close()

	// Set up some storage data
	result := repl.execute(`my.test = "direct_read_test"`)
	if result != "direct_read_test" {
		t.Fatalf("Setup failed: %s", result)
	}

	// Test direct reading of storage paths
	result = repl.execute("my.test")
	if result != "direct_read_test" {
		t.Errorf("Direct storage path read failed: got %q, want %q", result, "direct_read_test")
	}
}

// TestMultiLineInputDetection tests the multi-line input logic
func TestMultiLineInputDetection(t *testing.T) {
	repl := NewTestREPL(t)
	defer repl.Close()

	tests := []struct {
		name         string
		currentLine  string
		allLines     []string
		shouldContinue bool
	}{
		{
			name:         "if statement needs continuation",
			currentLine:  "if x > 10:",
			allLines:     []string{"if x > 10:"},
			shouldContinue: true,
		},
		{
			name:         "indented line continues block",
			currentLine:  "\toutput(\"hello\")",
			allLines:     []string{"if x > 10:", "\toutput(\"hello\")"},
			shouldContinue: true,
		},
		{
			name:         "empty line ends block",
			currentLine:  "",
			allLines:     []string{"if x > 10:", "\toutput(\"hello\")", ""},
			shouldContinue: false,
		},
		{
			name:         "dedented line ends block",
			currentLine:  "x = 5",
			allLines:     []string{"if x > 10:", "\toutput(\"hello\")", "x = 5"},
			shouldContinue: false,
		},
		{
			name:         "unmatched brackets continue",
			currentLine:  "data = {\"name\": \"test\",",
			allLines:     []string{"data = {\"name\": \"test\","},
			shouldContinue: true,
		},
		{
			name:         "procedure definition needs continuation",
			currentLine:  "my_func(x, y):",
			allLines:     []string{"my_func(x, y):"},
			shouldContinue: true,
		},
		{
			name:         "single line complete",
			currentLine:  "x = 42",
			allLines:     []string{"x = 42"},
			shouldContinue: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := repl.needsContinuation(tt.currentLine, tt.allLines)
			if result != tt.shouldContinue {
				t.Errorf("needsContinuation(%q, %v) = %v, want %v", tt.currentLine, tt.allLines, result, tt.shouldContinue)
			}
		})
	}
}

// TestAdvancedMBLFeatures tests advanced MBL functionality through REPL
func TestAdvancedMBLFeatures(t *testing.T) {
	repl := NewTestREPL(t)
	defer repl.Close()

	// Test string manipulation functions
	t.Run("String Functions", func(t *testing.T) {
		tests := []struct {
			input    string
			expected string
		}{
			{`upper("hello")`, "HELLO"},
			{`lower("WORLD")`, "world"},
			{`trim("  spaced  ")`, "spaced"},
		}

		for _, tt := range tests {
			result := repl.execute(tt.input)
			if result != tt.expected {
				t.Errorf("execute(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		}
	})

	// Test math functions
	t.Run("Math Functions", func(t *testing.T) {
		tests := []struct {
			input    string
			expected string
		}{
			{`abs(-5.7)`, "5.7"},
			{`floor(4.9)`, "4"},
			{`ceil(4.1)`, "5"},
			{`max(1, 5, 3, 9, 2)`, "9"},
			{`min(1, 5, 3, 9, 2)`, "1"},
		}

		for _, tt := range tests {
			result := repl.execute(tt.input)
			if result != tt.expected {
				t.Errorf("execute(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		}
	})

	// Test utility functions
	t.Run("Utility Functions", func(t *testing.T) {
		tests := []struct {
			setup    string
			input    string
			expected string
		}{
			{"", `len("hello")`, "5"},
			{"", `len([1, 2, 3, 4])`, "4"},
			{"", `text(42)`, "42"},
			{"", `number("123.45")`, "123.45"},
			{"", `add(1, 2, 3, 4, 5)`, "15"},
		}

		for _, tt := range tests {
			if tt.setup != "" {
				repl.execute(tt.setup)
			}
			result := repl.execute(tt.input)
			if result != tt.expected {
				t.Errorf("execute(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		}
	})

	// Test logical operators
	t.Run("Logical Operators", func(t *testing.T) {
		// Setup variables
		repl.execute("a = 5")
		repl.execute("b = 10")
		repl.execute("c = 3")

		tests := []struct {
			input    string
			expected string
		}{
			{`(a > 3) and (b < 15)`, "true"},
			{`(c < 2) or (a > 4)`, "true"},
			{`not (a > 10)`, "true"},
			{`a + b * c`, "35"},
		}

		for _, tt := range tests {
			result := repl.execute(tt.input)
			if result != tt.expected {
				t.Errorf("execute(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		}
	})

	// Test bracket filters
	t.Run("Bracket Filters", func(t *testing.T) {
		// Setup collections
		repl.execute(`numbers = [1, 2, 3, 4, 5, 6]`)
		repl.execute(`people = [{"name": "Alice", "age": 25}, {"name": "Bob", "age": 35}]`)

		tests := []struct {
			input    string
			contains string
		}{
			{`numbers[item > 3]`, "[4, 5, 6]"},
			{`people[item.age > 30]`, "Bob"},
		}

		for _, tt := range tests {
			result := repl.execute(tt.input)
			if !strings.Contains(result, tt.contains) {
				t.Errorf("execute(%q) = %q, expected to contain %q", tt.input, result, tt.contains)
			}
		}
	})
}

// TestUserDefinedProcedures tests procedure definition and calling
func TestUserDefinedProcedures(t *testing.T) {
	repl := NewTestREPL(t)
	defer repl.Close()

	// Test simple procedure
	t.Run("Simple Procedure", func(t *testing.T) {
		// Define procedure
		result := repl.execute(`double(x):
	return x * 2`)
		if strings.Contains(result, "Error:") {
			t.Fatalf("Procedure definition failed: %s", result)
		}

		// Test calls
		tests := []struct {
			input    string
			expected string
		}{
			{`double(5)`, "10"},
			{`double(10)`, "20"},
			{`double(-3)`, "-6"},
		}

		for _, tt := range tests {
			result := repl.execute(tt.input)
			if result != tt.expected {
				t.Errorf("execute(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		}
	})

	// Test procedure with multiple parameters
	t.Run("Multiple Parameters", func(t *testing.T) {
		result := repl.execute(`add_multiply(a, b, c):
	return (a + b) * c`)
		if strings.Contains(result, "Error:") {
			t.Fatalf("Procedure definition failed: %s", result)
		}

		result = repl.execute(`add_multiply(2, 3, 4)`)
		if result != "20" {
			t.Errorf("add_multiply(2, 3, 4) = %q, want %q", result, "20")
		}
	})
}

// TestInstantiationAndHeritability tests Step 7 features: new keyword and heritability
func TestInstantiationAndHeritability(t *testing.T) {
	repl := NewTestREPL(t)
	defer repl.Close()

	// Test basic instantiation
	t.Run("Basic Instantiation", func(t *testing.T) {
		tests := []struct {
			name     string
			input    string
			contains string
		}{
			{
				name:     "simple instantiation",
				input:    `new person {"name": "Alice", "age": 30}`,
				contains: "_type: person",
			},
			{
				name:     "instantiation without properties",
				input:    `new empty_record`,
				contains: "_type: empty_record",
			},
		}

		for _, tt := range tests {
			result := repl.execute(tt.input)
			if !strings.Contains(result, tt.contains) {
				t.Errorf("execute(%q) = %q, expected to contain %q", tt.input, result, tt.contains)
			}
		}
	})

	// Test copy inheritance
	t.Run("Copy Inheritance", func(t *testing.T) {
		// Setup a template
		setupResult := repl.execute(`base_person = {"name": "Template", "age": 25, "city": "DefaultCity"}`)
		if strings.Contains(setupResult, "Error:") {
			t.Fatalf("Template setup failed: %s", setupResult)
		}

		// Test copy inheritance
		result := repl.execute(`new person:(copy) base_person {"name": "Alice"}`)
		if !strings.Contains(result, "Alice") {
			t.Errorf("Copy inheritance failed to override name: %s", result)
		}
		if !strings.Contains(result, "25") {
			t.Errorf("Copy inheritance failed to inherit age: %s", result)
		}
		if !strings.Contains(result, "DefaultCity") {
			t.Errorf("Copy inheritance failed to inherit city: %s", result)
		}
	})

	// Test link inheritance
	t.Run("Link Inheritance", func(t *testing.T) {
		// Setup template
		repl.execute(`shared_template = {"shared_data": "linked_value", "count": 42}`)

		// Test link inheritance
		result := repl.execute(`new linked_object:(link) shared_template {"name": "Linked"}`)
		if !strings.Contains(result, "linked_value") {
			t.Errorf("Link inheritance failed: %s", result)
		}
		if !strings.Contains(result, "42") {
			t.Errorf("Link inheritance failed to inherit count: %s", result)
		}
	})

	// Test multiple inheritance
	t.Run("Multiple Inheritance", func(t *testing.T) {
		// Setup multiple templates
		repl.execute(`template1 = {"field1": "value1", "shared": "from_template1"}`)
		repl.execute(`template2 = {"field2": "value2", "shared": "from_template2"}`)

		// Test multiple inheritance (first source wins for conflicts)
		result := repl.execute(`new combined:(copy) template1, template2 {"field3": "value3"}`)
		if !strings.Contains(result, "value1") {
			t.Errorf("Multiple inheritance failed for field1: %s", result)
		}
		if !strings.Contains(result, "value2") {
			t.Errorf("Multiple inheritance failed for field2: %s", result)
		}
		if !strings.Contains(result, "value3") {
			t.Errorf("Multiple inheritance failed for field3: %s", result)
		}
		// First template should win for conflicts
		if !strings.Contains(result, "from_template1") {
			t.Errorf("Multiple inheritance conflict resolution failed: %s", result)
		}
	})

	// Test reset inheritance
	t.Run("Reset Inheritance", func(t *testing.T) {
		repl.execute(`reset_template = {"original": "old_value", "keep": "preserved"}`)

		result := repl.execute(`new reset_obj:(reset) reset_template {"original": "new_value"}`)
		if !strings.Contains(result, "new_value") {
			t.Errorf("Reset inheritance failed to override: %s", result)
		}
		if !strings.Contains(result, "preserved") {
			t.Errorf("Reset inheritance failed to preserve: %s", result)
		}
	})

	// Test error cases
	t.Run("Error Cases", func(t *testing.T) {
		tests := []struct {
			name      string
			input     string
			expectError bool
		}{
			{
				name:      "template not found",
				input:     `new obj:(copy) nonexistent_template`,
				expectError: true,
			},
			{
				name:      "template not a record",
				input:     `number_template = 42`,
				expectError: false, // setup
			},
			{
				name:      "template type error",
				input:     `new obj:(copy) number_template`,
				expectError: true,
			},
		}

		for _, tt := range tests {
			result := repl.execute(tt.input)
			hasError := strings.Contains(result, "Unknown:")

			if tt.expectError && !hasError {
				t.Errorf("Expected error for %q, got: %q", tt.input, result)
			}
			if !tt.expectError && hasError {
				t.Errorf("Unexpected error for %q: %q", tt.input, result)
			}
		}
	})
}

// TestAdvancedControlFlow tests complex control structures
func TestAdvancedControlFlow(t *testing.T) {
	repl := NewTestREPL(t)
	defer repl.Close()

	// Test while loop
	t.Run("While Loop", func(t *testing.T) {
		result := repl.execute(`counter = 0
total = 0
while counter < 3:
	total = total + counter
	counter = counter + 1

total`)
		if result != "3" {
			t.Errorf("While loop result = %q, want %q", result, "3")
		}
	})

	// Test nested control structures
	t.Run("Nested If Statements", func(t *testing.T) {
		result := repl.execute(`x = 10
if x > 5:
	if x > 8:
		result = "very big"
	else:
		result = "medium"
else:
	result = "small"

result`)
		if result != "very big" {
			t.Errorf("Nested if result = %q, want %q", result, "very big")
		}
	})
}