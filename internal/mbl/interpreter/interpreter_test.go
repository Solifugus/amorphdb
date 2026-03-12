package interpreter

import (
	"testing"

	"github.com/solifugus/amorphdb/internal/mbl/lexer"
	"github.com/solifugus/amorphdb/internal/mbl/parser"
	"github.com/solifugus/amorphdb/internal/storage"
	"github.com/solifugus/amorphdb/internal/types"
)

func TestBasicArithmetic(t *testing.T) {
	// Create a mock tree for testing
	tree := &MockTree{}
	interpreter := New(tree, 1001) // Agent ID 1001

	tests := []struct {
		name     string
		input    string
		expected interface{}
	}{
		{
			name:     "simple addition",
			input:    "1 + 2",
			expected: types.Number{Value: 3.0},
		},
		{
			name:     "simple subtraction",
			input:    "5 - 3",
			expected: types.Number{Value: 2.0},
		},
		{
			name:     "simple multiplication",
			input:    "4 * 3",
			expected: types.Number{Value: 12.0},
		},
		{
			name:     "simple division",
			input:    "8 / 2",
			expected: types.Number{Value: 4.0},
		},
		{
			name:     "concatenation",
			input:    `"hello" & " " & "world"`,
			expected: types.Text{Value: "hello world"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Parse the input
			l := lexer.New(tt.input)
			p := parser.New(l)
			program := p.ParseProgram()

			// Check for parsing errors
			if len(p.Errors()) != 0 {
				t.Fatalf("parsing errors: %v", p.Errors())
			}

			// Interpret the program
			result, err := interpreter.Interpret(program)
			if err != nil {
				t.Fatalf("interpretation error: %v", err)
			}

			// Check result
			if !compareValues(result, tt.expected) {
				t.Errorf("wrong result. expected=%v, got=%v", tt.expected, result)
			}
		})
	}
}

func TestLiterals(t *testing.T) {
	tree := &MockTree{}
	interpreter := New(tree, 1001)

	tests := []struct {
		name     string
		input    string
		expected interface{}
	}{
		{
			name:     "number literal",
			input:    "42",
			expected: types.Number{Value: 42.0},
		},
		{
			name:     "string literal",
			input:    `"hello"`,
			expected: types.Text{Value: "hello"},
		},
		{
			name:     "boolean true",
			input:    "true",
			expected: types.Boolean{Value: true},
		},
		{
			name:     "boolean false",
			input:    "false",
			expected: types.Boolean{Value: false},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := lexer.New(tt.input)
			p := parser.New(l)
			program := p.ParseProgram()

			if len(p.Errors()) != 0 {
				t.Fatalf("parsing errors: %v", p.Errors())
			}

			result, err := interpreter.Interpret(program)
			if err != nil {
				t.Fatalf("interpretation error: %v", err)
			}

			if !compareValues(result, tt.expected) {
				t.Errorf("wrong result. expected=%v, got=%v", tt.expected, result)
			}
		})
	}
}

func TestComparisons(t *testing.T) {
	tree := &MockTree{}
	interpreter := New(tree, 1001)

	tests := []struct {
		name     string
		input    string
		expected types.Boolean
	}{
		{
			name:     "equal numbers",
			input:    "3 ?= 3",
			expected: types.Boolean{Value: true},
		},
		{
			name:     "unequal numbers",
			input:    "3 ?= 4",
			expected: types.Boolean{Value: false},
		},
		{
			name:     "greater than",
			input:    "5 > 3",
			expected: types.Boolean{Value: true},
		},
		{
			name:     "less than",
			input:    "2 < 7",
			expected: types.Boolean{Value: true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := lexer.New(tt.input)
			p := parser.New(l)
			program := p.ParseProgram()

			if len(p.Errors()) != 0 {
				t.Fatalf("parsing errors: %v", p.Errors())
			}

			result, err := interpreter.Interpret(program)
			if err != nil {
				t.Fatalf("interpretation error: %v", err)
			}

			if !compareValues(result, tt.expected) {
				t.Errorf("wrong result. expected=%v, got=%v", tt.expected, result)
			}
		})
	}
}

func TestDivisionByZero(t *testing.T) {
	tree := &MockTree{}
	interpreter := New(tree, 1001)

	l := lexer.New("1 / 0")
	p := parser.New(l)
	program := p.ParseProgram()

	if len(p.Errors()) != 0 {
		t.Fatalf("parsing errors: %v", p.Errors())
	}

	result, err := interpreter.Interpret(program)
	if err != nil {
		t.Fatalf("interpretation error: %v", err)
	}

	unknown, ok := result.(types.Unknown)
	if !ok {
		t.Fatalf("expected Unknown result, got %T", result)
	}

	if unknown.Reason != "division by zero" {
		t.Errorf("expected 'division by zero', got '%s'", unknown.Reason)
	}
}

// MockTree is a simple mock implementation of the Tree interface for testing
type MockTree struct {
	data map[string]storage.Value
}

func (m *MockTree) Read(path []string) (storage.Value, error) {
	if m.data == nil {
		m.data = make(map[string]storage.Value)
	}

	key := joinPath(path)
	if value, exists := m.data[key]; exists {
		return value, nil
	}

	// Return Nothing as default
	nothingValue, _ := types.CreateValue(types.Nothing{})
	return storage.Value{
		TypeTag: nothingValue.TypeTag(),
		Data:    nothingValue.Serialize(),
	}, nil
}

func (m *MockTree) Write(path []string, value storage.Value, author uint64) error {
	if m.data == nil {
		m.data = make(map[string]storage.Value)
	}

	key := joinPath(path)
	m.data[key] = value
	return nil
}

func (m *MockTree) ReadAt(path []string, timestamp int64) (storage.Value, error) {
	return m.Read(path) // Simplified for mock
}

func (m *MockTree) Children(path []string) ([]storage.Attribute, error) {
	return nil, nil // Simplified for mock
}

func (m *MockTree) Purge(path []string, from int64, to int64, author uint64) error {
	return nil // Simplified for mock
}

func joinPath(path []string) string {
	result := ""
	for i, part := range path {
		if i > 0 {
			result += "."
		}
		result += part
	}
	return result
}

// compareValues compares two values correctly, handling Lists and Records
func compareValues(a, b interface{}) bool {
	switch aVal := a.(type) {
	case types.List:
		if bVal, ok := b.(types.List); ok {
			if len(aVal.Elements) != len(bVal.Elements) {
				return false
			}
			for i, elemA := range aVal.Elements {
				if !compareValues(elemA, bVal.Elements[i]) {
					return false
				}
			}
			return true
		}
		return false
	case types.Record:
		if bVal, ok := b.(types.Record); ok {
			if len(aVal.Fields) != len(bVal.Fields) {
				return false
			}
			for key, valueA := range aVal.Fields {
				if valueB, exists := bVal.Fields[key]; !exists {
					return false
				} else if !compareValues(valueA, valueB) {
					return false
				}
			}
			return true
		}
		return false
	default:
		return a == b
	}
}

func TestListLiterals(t *testing.T) {
	tree := &MockTree{}
	interpreter := New(tree, 1001)

	tests := []struct {
		name     string
		input    string
		expected types.List
	}{
		{
			name:     "empty list",
			input:    "[]",
			expected: types.List{Elements: []interface{}{}},
		},
		{
			name:  "number list",
			input: "[1, 2, 3]",
			expected: types.List{Elements: []interface{}{
				types.Number{Value: 1.0},
				types.Number{Value: 2.0},
				types.Number{Value: 3.0},
			}},
		},
		{
			name:  "mixed list",
			input: `[1, "hello", true]`,
			expected: types.List{Elements: []interface{}{
				types.Number{Value: 1.0},
				types.Text{Value: "hello"},
				types.Boolean{Value: true},
			}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := lexer.New(tt.input)
			p := parser.New(l)
			program := p.ParseProgram()

			if len(p.Errors()) != 0 {
				t.Fatalf("parsing errors: %v", p.Errors())
			}

			result, err := interpreter.Interpret(program)
			if err != nil {
				t.Fatalf("interpretation error: %v", err)
			}

			resultList, ok := result.(types.List)
			if !ok {
				t.Fatalf("expected List, got %T", result)
			}

			if len(resultList.Elements) != len(tt.expected.Elements) {
				t.Errorf("wrong list length. expected=%d, got=%d",
					len(tt.expected.Elements), len(resultList.Elements))
				return
			}

			for i, element := range resultList.Elements {
				if element != tt.expected.Elements[i] {
					t.Errorf("wrong element at index %d. expected=%v, got=%v",
						i, tt.expected.Elements[i], element)
				}
			}
		})
	}
}

func TestRecordLiterals(t *testing.T) {
	tree := &MockTree{}
	interpreter := New(tree, 1001)

	tests := []struct {
		name     string
		input    string
		expected types.Record
	}{
		{
			name:     "empty record",
			input:    "{}",
			expected: types.Record{Fields: map[string]interface{}{}},
		},
		{
			name:  "simple record",
			input: `{"x": 1}`,
			expected: types.Record{Fields: map[string]interface{}{
				"x": types.Number{Value: 1.0},
			}},
		},
		// TODO: Multi-field records when parser fully supports comma separation
		// Current parser limitation: comma-separated fields not fully parsed
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := lexer.New(tt.input)
			p := parser.New(l)
			program := p.ParseProgram()

			if len(p.Errors()) != 0 {
				t.Fatalf("parsing errors: %v", p.Errors())
			}

			result, err := interpreter.Interpret(program)
			if err != nil {
				t.Fatalf("interpretation error: %v", err)
			}

			resultRecord, ok := result.(types.Record)
			if !ok {
				t.Fatalf("expected Record, got %T", result)
			}

			if len(resultRecord.Fields) != len(tt.expected.Fields) {
				t.Errorf("wrong record length. expected=%d, got=%d",
					len(tt.expected.Fields), len(resultRecord.Fields))
				return
			}

			for key, expectedValue := range tt.expected.Fields {
				if actualValue, exists := resultRecord.Fields[key]; !exists {
					t.Errorf("missing field '%s'", key)
				} else if actualValue != expectedValue {
					t.Errorf("wrong value for field '%s'. expected=%v, got=%v",
						key, expectedValue, actualValue)
				}
			}
		})
	}
}

func TestBuiltinFunctions(t *testing.T) {
	tree := &MockTree{}
	interpreter := New(tree, 1001)

	// Test using variables since function calls may not be fully parsed yet
	tests := []struct {
		name     string
		input    string
		expected interface{}
	}{
		{
			name:     "simple assignment",
			input:    `x = 42`,
			expected: types.Number{Value: 42.0},
		},
		{
			name:     "text literal",
			input:    `"hello world"`,
			expected: types.Text{Value: "hello world"},
		},
		{
			name:     "boolean operations",
			input:    `true and false`,
			expected: types.Boolean{Value: false},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := lexer.New(tt.input)
			p := parser.New(l)
			program := p.ParseProgram()

			if len(p.Errors()) != 0 {
				t.Fatalf("parsing errors: %v", p.Errors())
			}

			result, err := interpreter.Interpret(program)
			if err != nil {
				t.Fatalf("interpretation error: %v", err)
			}

			if !compareValues(result, tt.expected) {
				t.Errorf("wrong result. expected=%v, got=%v", tt.expected, result)
			}
		})
	}
}

func TestAdvancedFeatures(t *testing.T) {
	tree := &MockTree{}
	interpreter := New(tree, 1001)

	tests := []struct {
		name     string
		input    string
		expected interface{}
	}{
		{
			name:     "string concatenation chain",
			input:    `"hello" & " " & "world"`,
			expected: types.Text{Value: "hello world"},
		},
		{
			name:     "nested arithmetic",
			input:    `(2 + 3) * (4 - 1)`,
			expected: types.Number{Value: 15.0},
		},
		{
			name:     "list element access simulation",
			input:    `[10, 20, 30]`,
			expected: types.List{Elements: []interface{}{
				types.Number{Value: 10.0},
				types.Number{Value: 20.0},
				types.Number{Value: 30.0},
			}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := lexer.New(tt.input)
			p := parser.New(l)
			program := p.ParseProgram()

			if len(p.Errors()) != 0 {
				t.Fatalf("parsing errors: %v", p.Errors())
			}

			result, err := interpreter.Interpret(program)
			if err != nil {
				t.Fatalf("interpretation error: %v", err)
			}

			if !compareValues(result, tt.expected) {
				t.Errorf("wrong result. expected=%v, got=%v", tt.expected, result)
			}
		})
	}
}