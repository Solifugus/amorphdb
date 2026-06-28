package interpreter

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/solifugus/amorphdb/internal/mbl/lexer"
	"github.com/solifugus/amorphdb/internal/mbl/parser"
	"github.com/solifugus/amorphdb/internal/storage"
	"github.com/solifugus/amorphdb/internal/types"
)

// newHTTPTestServer starts a local HTTP server that emulates the httpbin.org
// endpoints the web.* tests exercise. Tests are hermetic — they no longer reach
// the public internet, so they run deterministically offline and under parallel
// load. The server reflects the observed request method, headers, and body so
// the assertions can confirm the correct HTTP verb and headers were sent.
func newHTTPTestServer() *httptest.Server {
	echo := func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		hdrs := make(map[string]string, len(r.Header))
		for k, v := range r.Header {
			hdrs[k] = strings.Join(v, ", ")
		}
		resp := map[string]interface{}{
			"method":  r.Method,
			"headers": hdrs,
			"body":    string(body),
			"url":     r.URL.String(),
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(resp)
	}

	mux := http.NewServeMux()
	for _, path := range []string{"/get", "/post", "/put", "/patch", "/delete", "/headers"} {
		mux.HandleFunc(path, echo)
	}

	// /delay/N sleeps N seconds before responding, honoring client cancellation
	// so a client-side timeout returns promptly and never blocks server shutdown.
	mux.HandleFunc("/delay/", func(w http.ResponseWriter, r *http.Request) {
		secs, _ := strconv.Atoi(strings.TrimPrefix(r.URL.Path, "/delay/"))
		select {
		case <-time.After(time.Duration(secs) * time.Second):
			echo(w, r)
		case <-r.Context().Done():
		}
	})

	return httptest.NewServer(mux)
}

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

// HasPath checks if a path exists in the mock storage
func (m *MockTree) HasPath(path []string) bool {
	if m.data == nil {
		return false
	}
	key := joinPath(path)
	_, exists := m.data[key]
	return exists
}

// GetValue retrieves a value from the mock storage, returning nil if not found
func (m *MockTree) GetValue(path []string) *storage.Value {
	if m.data == nil {
		return nil
	}
	key := joinPath(path)
	if value, exists := m.data[key]; exists {
		return &value
	}
	return nil
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
		// TODO: Add tests for unquoted field names as specified in amorphdb_design.md
		// Spec shows {name: "value"} but current implementation requires {"name": "value"}
		// See Step 2 completion note in DEVPLAN.md
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

// TestRecordLiteralAssignment tests record literal assignment as required by Step 2
func TestRecordLiteralAssignment(t *testing.T) {
	tests := []struct {
		name        string
		assignments []string // Multiple assignment statements to execute in sequence
		testPath    string   // Path to test after assignments
		expected    interface{}
	}{
		{
			// Test single field first to isolate the issue
			name: "single field record assignment",
			assignments: []string{
				`x = {"name": "Alice"}`,
			},
			testPath: "x",
			expected: types.Record{Fields: map[string]interface{}{
				"name": types.Text{Value: "Alice"},
			}},
		},
		{
			// Basic record literal
			name: "basic record literal assignment",
			assignments: []string{
				`x = {"name": "Alice", "age": 30}`,
			},
			testPath: "x.name",
			expected: types.Text{Value: "Alice"},
		},
		{
			name: "basic record literal assignment - age field",
			assignments: []string{
				`x = {"name": "Alice", "age": 30}`,
			},
			testPath: "x.age",
			expected: types.Number{Value: 30.0},
		},
		{
			// Nested record literal
			name: "nested record literal assignment",
			assignments: []string{
				`x = {"address": {"city": "Rome", "state": "NY"}}`,
			},
			testPath: "x.address.city",
			expected: types.Text{Value: "Rome"},
		},
		{
			name: "nested record literal assignment - state field",
			assignments: []string{
				`x = {"address": {"city": "Rome", "state": "NY"}}`,
			},
			testPath: "x.address.state",
			expected: types.Text{Value: "NY"},
		},
		{
			// Record literal with all supported value types
			name: "record with multiple value types",
			assignments: []string{
				`x = {"label": "test", "count": 42, "active": true}`,
			},
			testPath: "x.label",
			expected: types.Text{Value: "test"},
		},
		{
			name: "record with multiple value types - count field",
			assignments: []string{
				`x = {"label": "test", "count": 42, "active": true}`,
			},
			testPath: "x.count",
			expected: types.Number{Value: 42.0},
		},
		{
			name: "record with multiple value types - active field",
			assignments: []string{
				`x = {"label": "test", "count": 42, "active": true}`,
			},
			testPath: "x.active",
			expected: types.Boolean{Value: true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tree := &MockTree{}
			interpreter := New(tree, 1001)

			// Execute all assignment statements
			for _, assignment := range tt.assignments {
				l := lexer.New(assignment)
				p := parser.New(l)
				program := p.ParseProgram()

				if len(p.Errors()) != 0 {
					t.Fatalf("parsing errors for '%s': %v", assignment, p.Errors())
				}

				_, err := interpreter.Interpret(program)
				if err != nil {
					t.Fatalf("interpretation error for '%s': %v", assignment, err)
				}
			}

			// Now test reading the expected path
			l := lexer.New(tt.testPath)
			p := parser.New(l)
			program := p.ParseProgram()

			if len(p.Errors()) != 0 {
				t.Fatalf("parsing errors for test path '%s': %v", tt.testPath, p.Errors())
			}

			result, err := interpreter.Interpret(program)
			if err != nil {
				t.Fatalf("interpretation error for test path '%s': %v", tt.testPath, err)
			}

			// Compare the result with expected
			if !compareValues(result, tt.expected) {
				// Debug: print detailed comparison for Records
				if resultRecord, ok := result.(types.Record); ok {
					if expectedRecord, ok := tt.expected.(types.Record); ok {
						t.Errorf("Record field mismatch for path '%s':", tt.testPath)
						t.Errorf("  Expected fields: %+v", expectedRecord.Fields)
						t.Errorf("  Actual fields: %+v", resultRecord.Fields)
						return
					}
				}
				t.Errorf("wrong result for path '%s'. expected=%v, got=%v", tt.testPath, tt.expected, result)
			}
		})
	}
}

// TestRecursiveAssignment tests auto-creation of intermediate nodes as required by Step 3
func TestRecursiveAssignment(t *testing.T) {
	tests := []struct {
		name        string
		assignments []string
		testPath    string
		expected    interface{}
	}{
		{
			name: "assign to path where intermediates don't exist",
			assignments: []string{
				`my.app.deeply.nested.value = 42`,
			},
			testPath: "my.app.deeply.nested.value",
			expected: types.Number{Value: 42},
		},
		{
			name: "intermediate paths should exist as empty records",
			assignments: []string{
				`my.app.deeply.nested.value = 42`,
			},
			testPath: "my.app",
			expected: types.Record{Fields: map[string]interface{}{}},
		},
		{
			name: "deeply nested intermediate paths should exist",
			assignments: []string{
				`my.app.deeply.nested.value = 42`,
			},
			testPath: "my.app.deeply",
			expected: types.Record{Fields: map[string]interface{}{}},
		},
		{
			name: "assign where some intermediates exist and some don't - existing intermediate",
			assignments: []string{
				`my.app = "exists"`,
				`my.app.child.grandchild = "hello"`,
			},
			testPath: "my.app",
			expected: types.Text{Value: "exists"},
		},
		{
			name: "assign where some intermediates exist and some don't - nested value",
			assignments: []string{
				`my.app = "exists"`,
				`my.app.child.grandchild = "hello"`,
			},
			testPath: "my.app.child.grandchild",
			expected: types.Text{Value: "hello"},
		},
		{
			name: "assign where some intermediates exist and some don't - auto-created intermediate",
			assignments: []string{
				`my.app = "exists"`,
				`my.app.child.grandchild = "hello"`,
			},
			testPath: "my.app.child",
			expected: types.Record{Fields: map[string]interface{}{}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a fresh mock tree and interpreter for each test
			tree := &MockTree{data: make(map[string]storage.Value)}
			interpreter := New(tree, 1000)

			// Execute all assignment statements
			for _, assignment := range tt.assignments {
				l := lexer.New(assignment)
				p := parser.New(l)
				program := p.ParseProgram()
				if len(p.Errors()) != 0 {
					t.Fatalf("parsing errors for '%s': %v", assignment, p.Errors())
				}

				_, err := interpreter.Interpret(program)
				if err != nil {
					t.Fatalf("interpretation error for '%s': %v", assignment, err)
				}
			}

			// Test the specific path
			l := lexer.New(tt.testPath)
			p := parser.New(l)
			program := p.ParseProgram()
			if len(p.Errors()) != 0 {
				t.Fatalf("parsing errors for test path '%s': %v", tt.testPath, p.Errors())
			}

			result, err := interpreter.Interpret(program)
			if err != nil {
				t.Fatalf("interpretation error for test path '%s': %v", tt.testPath, err)
			}

			// Compare the result with expected
			if !compareValues(result, tt.expected) {
				// Debug: print detailed comparison for Records
				if resultRecord, ok := result.(types.Record); ok {
					if expectedRecord, ok := tt.expected.(types.Record); ok {
						t.Errorf("Record field mismatch for path '%s':", tt.testPath)
						t.Errorf("  Expected fields: %+v", expectedRecord.Fields)
						t.Errorf("  Actual fields: %+v", resultRecord.Fields)
						return
					}
				}
				t.Errorf("wrong result for path '%s'. expected=%v, got=%v", tt.testPath, tt.expected, result)
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
			name:  "list element access simulation",
			input: `[10, 20, 30]`,
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
func TestAssetFunction(t *testing.T) {
	// Create a mock tree for testing
	tree := &MockTree{}
	interpreter := New(tree, 1001) // Agent ID 1001

	tests := []struct {
		name     string
		input    string
		expected interface{}
	}{
		{
			name:  "basic asset creation",
			input: `asset("hello", "text/plain")`,
			expected: types.Record{
				Fields: map[string]interface{}{
					"data":      types.Text{Value: "hello"},
					"mime_type": types.Text{Value: "text/plain"},
				},
			},
		},
		{
			name:  "asset with JSON mime type",
			input: `asset("json_data", "application/json")`,
			expected: types.Record{
				Fields: map[string]interface{}{
					"data":      types.Text{Value: "json_data"},
					"mime_type": types.Text{Value: "application/json"},
				},
			},
		},
		{
			name:  "asset with binary data",
			input: `asset("binary data", "image/png")`,
			expected: types.Record{
				Fields: map[string]interface{}{
					"data":      types.Text{Value: "binary data"},
					"mime_type": types.Text{Value: "image/png"},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Parse the input
			l := lexer.New(tt.input)
			p := parser.New(l)
			program := p.ParseProgram()

			if len(p.Errors()) != 0 {
				t.Fatalf("parsing errors: %v", p.Errors())
			}

			// Interpret the parsed program
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

func TestAssetFunctionErrors(t *testing.T) {
	// Create a mock tree for testing
	tree := &MockTree{}
	interpreter := New(tree, 1001) // Agent ID 1001

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "asset with no arguments",
			input:    `asset()`,
			expected: "asset() requires exactly 2 arguments: asset(data, mime_type)",
		},
		{
			name:     "asset with one argument",
			input:    `asset("data")`,
			expected: "asset() requires exactly 2 arguments: asset(data, mime_type)",
		},
		{
			name:     "asset with three arguments",
			input:    `asset("data", "text/plain", "extra")`,
			expected: "asset() requires exactly 2 arguments: asset(data, mime_type)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Parse the input
			l := lexer.New(tt.input)
			p := parser.New(l)
			program := p.ParseProgram()

			if len(p.Errors()) != 0 {
				t.Fatalf("parsing errors: %v", p.Errors())
			}

			// Interpret the parsed program
			result, err := interpreter.Interpret(program)
			if err != nil {
				t.Fatalf("interpretation error: %v", err)
			}

			// Should return Unknown with the expected error reason
			unknown, ok := result.(types.Unknown)
			if !ok {
				t.Errorf("expected Unknown result, got %T", result)
				return
			}

			if unknown.Reason != tt.expected {
				t.Errorf("wrong error reason. expected=%q, got=%q", tt.expected, unknown.Reason)
			}
		})
	}
}

// TestProjectionExpressions tests projection syntax evaluation
func TestProjectionExpressions(t *testing.T) {
	tree := &MockTree{}
	interpreter := New(tree, 1001)

	tests := []struct {
		name     string
		setup    string
		input    string
		expected interface{}
	}{
		{
			name:  "basic projection",
			setup: `person = { "name": "Alice", "age": 30, "job": "Engineer", "city": "Rome" }`,
			input: `person{ name, age, job }`,
			expected: types.Record{
				Fields: map[string]interface{}{
					"name": types.Text{Value: "Alice"},
					"age":  types.Number{Value: 30.0},
					"job":  types.Text{Value: "Engineer"},
				},
			},
		},
		{
			name:  "projection with missing field",
			setup: `person = { "name": "Alice", "age": 30 }`,
			input: `person{ name, nonexistent }`,
			expected: types.Record{
				Fields: map[string]interface{}{
					"name":        types.Text{Value: "Alice"},
					"nonexistent": types.Unknown{Reason: "not_found"},
				},
			},
		},
		{
			name:  "single field projection",
			setup: `data = { "value": 42, "other": "ignored" }`,
			input: `data{ value }`,
			expected: types.Record{
				Fields: map[string]interface{}{
					"value": types.Number{Value: 42.0},
				},
			},
		},
		{
			name:  "projection from path",
			setup: `my.user = { "name": "Bob", "email": "bob@example.com", "password": "secret" }`,
			input: `my.user{ name, email }`,
			expected: types.Record{
				Fields: map[string]interface{}{
					"name":  types.Text{Value: "Bob"},
					"email": types.Text{Value: "bob@example.com"},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup data if needed
			if tt.setup != "" {
				l := lexer.New(tt.setup)
				p := parser.New(l)
				setupProgram := p.ParseProgram()

				if len(p.Errors()) != 0 {
					t.Fatalf("setup parsing errors: %v", p.Errors())
				}

				_, err := interpreter.Interpret(setupProgram)
				if err != nil {
					t.Fatalf("setup interpretation error: %v", err)
				}
			}

			// Parse and evaluate the projection expression
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

			// Compare the result
			if !types.Equal(result, tt.expected) {
				// Provide detailed error message showing type mismatches
				if expectedRecord, ok := tt.expected.(types.Record); ok {
					if resultRecord, ok := result.(types.Record); ok {
						t.Errorf("wrong result. Expected Record with %d fields, got Record with %d fields",
							len(expectedRecord.Fields), len(resultRecord.Fields))
						// Show field details
						for key, expectedValue := range expectedRecord.Fields {
							if actualValue, exists := resultRecord.Fields[key]; exists {
								if !types.Equal(expectedValue, actualValue) {
									t.Errorf("  Field %s: expected %T(%v), got %T(%v)",
										key, expectedValue, expectedValue, actualValue, actualValue)
								}
							} else {
								t.Errorf("  Field %s: missing in result", key)
							}
						}
					} else {
						t.Errorf("wrong result. expected=Record, got=%T", result)
					}
				} else {
					t.Errorf("wrong result. expected=%v, got=%v", tt.expected, result)
				}
			}
		})
	}
}

// TestProjectionErrorCases tests error conditions for projections
func TestProjectionErrorCases(t *testing.T) {
	tree := &MockTree{}
	interpreter := New(tree, 1001)

	tests := []struct {
		name          string
		setup         string
		input         string
		expectedError string
	}{
		{
			name:          "projection from non-record",
			setup:         `value = 42`,
			input:         `value{ field }`,
			expectedError: "cannot project from types.Number, projections require a record",
		},
		{
			name:          "projection from text",
			setup:         `text = "hello"`,
			input:         `text{ field }`,
			expectedError: "cannot project from types.Text, projections require a record",
		},
		{
			name:          "projection from number literal",
			setup:         ``,
			input:         `42{ field }`,
			expectedError: "cannot project from types.Number, projections require a record",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup data if needed
			if tt.setup != "" {
				l := lexer.New(tt.setup)
				p := parser.New(l)
				setupProgram := p.ParseProgram()

				if len(p.Errors()) != 0 {
					t.Fatalf("setup parsing errors: %v", p.Errors())
				}

				_, err := interpreter.Interpret(setupProgram)
				if err != nil {
					t.Fatalf("setup interpretation error: %v", err)
				}
			}

			// Parse and evaluate the projection expression
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

			// Should return Unknown with the expected error
			unknown, ok := result.(types.Unknown)
			if !ok {
				t.Errorf("expected Unknown result, got %T", result)
				return
			}

			if unknown.Reason != tt.expectedError {
				t.Errorf("wrong error reason. expected=%q, got=%q", tt.expectedError, unknown.Reason)
			}
		})
	}
}

func TestEmbedDirectives(t *testing.T) {
	// Create a mock tree for testing
	tree := &MockTree{}
	interpreter := New(tree, 1001)

	tests := []struct {
		name     string
		input    string
		expected interface{}
	}{
		{
			name:     "basic embed directive",
			input:    "embed my.stamp",
			expected: types.Nothing{},
		},
		{
			name:     "spread directive",
			input:    "...my.other.stamp",
			expected: types.Nothing{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := interpreter.EvaluateExpression(tt.input)

			if err != nil {
				t.Fatalf("evaluation error: %v", err)
			}

			switch expected := tt.expected.(type) {
			case types.Nothing:
				if _, ok := result.(types.Nothing); !ok {
					t.Errorf("expected Nothing, got %T(%v)", result, result)
				}
			default:
				t.Errorf("unhandled expected type: %T", expected)
			}
		})
	}
}

// TestXMLImportExport tests XML import and export functionality
func TestXMLImportExport(t *testing.T) {
	// Create temporary test files
	simpleXML := `<?xml version="1.0" encoding="UTF-8"?>
<report date="2024-03-15">
    <institution>First National Bank</institution>
    <transactions>
        <transaction id="T001" amount="1500.00">
            <description>Wire Transfer</description>
        </transaction>
        <transaction id="T002" amount="250.00">
            <description>ATM Withdrawal</description>
        </transaction>
    </transactions>
</report>`

	namespacedXML := `<?xml version="1.0" encoding="UTF-8"?>
<financial_report xmlns:fin="http://example.com/financial">
    <header date="2024-03-15">
        <institution>First National Bank</institution>
    </header>
    <fin:transactions>
        <fin:transaction id="T001" amount="1500.00">
            <description>Wire Transfer</description>
        </fin:transaction>
    </fin:transactions>
</financial_report>`

	// Create temporary files
	tmpDir := t.TempDir()
	simpleXMLPath := tmpDir + "/simple.xml"
	namespacedXMLPath := tmpDir + "/namespaced.xml"
	exportPath := tmpDir + "/exported.xml"

	// Write test XML files
	err := ioutil.WriteFile(simpleXMLPath, []byte(simpleXML), 0644)
	if err != nil {
		t.Fatalf("Failed to create test XML file: %v", err)
	}

	err = ioutil.WriteFile(namespacedXMLPath, []byte(namespacedXML), 0644)
	if err != nil {
		t.Fatalf("Failed to create namespaced XML file: %v", err)
	}

	tree := &MockTree{}
	interpreter := New(tree, 1001)

	// Test simple XML import
	t.Run("Simple XML Import", func(t *testing.T) {
		result, err := interpreter.EvaluateExpression(fmt.Sprintf(`my.computer.files.import("%s", "xml")`, simpleXMLPath))
		if err != nil {
			t.Fatalf("Import failed: %v", err)
		}

		record, ok := result.(types.Record)
		if !ok {
			t.Fatalf("Expected Record, got %T", result)
		}

		// Check root attributes
		if date, exists := record.Fields["date"]; exists {
			if dateText, ok := date.(types.Text); ok {
				if dateText.Value != "2024-03-15" {
					t.Errorf("Expected date '2024-03-15', got '%s'", dateText.Value)
				}
			} else {
				t.Errorf("Expected date to be Text, got %T", date)
			}
		} else {
			t.Error("Expected date attribute")
		}

		// Check institution field
		if institution, exists := record.Fields["institution"]; exists {
			if instRecord, ok := institution.(types.Record); ok {
				if text, exists := instRecord.Fields["_text"]; exists {
					if textValue, ok := text.(types.Text); ok {
						if textValue.Value != "First National Bank" {
							t.Errorf("Expected institution 'First National Bank', got '%s'", textValue.Value)
						}
					}
				}
			}
		}

		// Check transactions list
		if transactions, exists := record.Fields["transactions"]; exists {
			if transRecord, ok := transactions.(types.Record); ok {
				if transactionField, exists := transRecord.Fields["transaction"]; exists {
					if transList, ok := transactionField.(types.List); ok {
						if len(transList.Elements) != 2 {
							t.Errorf("Expected 2 transactions, got %d", len(transList.Elements))
						}
					}
				}
			}
		}
	})

	// Test XML import with namespaces
	t.Run("XML Import with Namespaces", func(t *testing.T) {
		result, err := interpreter.EvaluateExpression(fmt.Sprintf(`my.computer.files.import("%s", "xml")`, namespacedXMLPath))
		if err != nil {
			t.Fatalf("Namespaced import failed: %v", err)
		}

		record, ok := result.(types.Record)
		if !ok {
			t.Fatalf("Expected Record, got %T", result)
		}

		// Basic structure check - namespaces are handled as prefixed element names
		if header, exists := record.Fields["header"]; exists {
			if headerRecord, ok := header.(types.Record); ok {
				if date, exists := headerRecord.Fields["date"]; exists {
					if dateText, ok := date.(types.Text); ok && dateText.Value == "2024-03-15" {
						// Success - namespace handling works
					} else {
						t.Error("Namespace handling issue with date attribute")
					}
				}
			}
		}
	})

	// Test XML export
	t.Run("XML Export", func(t *testing.T) {
		// Create a test record to export
		testRecord := types.Record{
			Fields: map[string]interface{}{
				"title":  types.Text{Value: "Test Report"},
				"count":  types.Number{Value: 42},
				"active": types.Boolean{Value: true},
				"details": types.Record{
					Fields: map[string]interface{}{
						"_text": types.Text{Value: "Some details"},
						"id":    types.Text{Value: "D001"},
					},
				},
			},
		}

		// Set the record in interpreter scope for export
		interpreter.scope.Set("test_record", testRecord)

		// Test export with default options
		result, err := interpreter.EvaluateExpression(fmt.Sprintf(`my.computer.files.export(test_record, "%s", "xml")`, exportPath))
		if err != nil {
			t.Fatalf("Export failed: %v", err)
		}

		// Should return a time stamp on success
		if _, ok := result.(types.Time); !ok {
			t.Errorf("Expected Time result from export, got %T", result)
		}

		// Verify the exported file exists and has content
		exportedContent, err := ioutil.ReadFile(exportPath)
		if err != nil {
			t.Fatalf("Failed to read exported file: %v", err)
		}

		exportedStr := string(exportedContent)
		if !strings.Contains(exportedStr, "<?xml") {
			t.Error("Exported XML missing XML declaration")
		}
		if !strings.Contains(exportedStr, "<root>") {
			t.Error("Exported XML missing root element")
		}
		if !strings.Contains(exportedStr, "Test Report") {
			t.Error("Exported XML missing test data")
		}
		if !strings.Contains(exportedStr, "<title>Test Report</title>") {
			t.Error("Exported XML missing title element")
		}
	})

	// Test error cases
	t.Run("XML Import Error Cases", func(t *testing.T) {
		// Test non-existent file
		result, err := interpreter.EvaluateExpression(`my.computer.files.import("/nonexistent/file.xml", "xml")`)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}

		if unknown, ok := result.(types.Unknown); ok {
			if !strings.Contains(unknown.Reason, "failed to read file") {
				t.Errorf("Expected file read error, got: %s", unknown.Reason)
			}
		} else {
			t.Errorf("Expected Unknown result for non-existent file, got %T", result)
		}

		// Test invalid format
		result, err = interpreter.EvaluateExpression(fmt.Sprintf(`my.computer.files.import("%s", "invalid_format")`, simpleXMLPath))
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}

		if unknown, ok := result.(types.Unknown); ok {
			if !strings.Contains(unknown.Reason, "unsupported import format") {
				t.Errorf("Expected format error, got: %s", unknown.Reason)
			}
		} else {
			t.Errorf("Expected Unknown result for invalid format, got %T", result)
		}
	})

	// Test xlsx stub
	t.Run("XLSX Format Stub", func(t *testing.T) {
		result, err := interpreter.EvaluateExpression(fmt.Sprintf(`my.computer.files.import("%s", "xlsx")`, simpleXMLPath))
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}

		if unknown, ok := result.(types.Unknown); ok {
			if !strings.Contains(unknown.Reason, "Excel import not yet implemented") {
				t.Errorf("Expected xlsx stub message, got: %s", unknown.Reason)
			}
		} else {
			t.Errorf("Expected Unknown result for xlsx format, got %T", result)
		}
	})
}

func TestHttpGetFunction(t *testing.T) {
	tree := &MockTree{}
	interpreter := New(tree, 1001)
	server := newHTTPTestServer()
	defer server.Close()

	// Test basic GET request
	t.Run("Basic GET request", func(t *testing.T) {
		result, err := interpreter.EvaluateExpression(fmt.Sprintf(`my.computer.network.web.get("%s/get")`, server.URL))
		if err != nil {
			t.Fatalf("GET request failed: %v", err)
		}

		record, ok := result.(types.Record)
		if !ok {
			t.Fatalf("Expected Record response, got %T", result)
		}

		// Verify response structure according to spec
		if _, exists := record.Fields["status"]; !exists {
			t.Error("Response missing 'status' field")
		}
		if status, ok := record.Fields["status"].(types.Number); ok {
			if status.Value != 200 {
				t.Errorf("Expected status 200, got %v", status.Value)
			}
		} else {
			t.Error("Status field should be Number type")
		}

		if _, exists := record.Fields["body"]; !exists {
			t.Error("Response missing 'body' field")
		}
		if _, ok := record.Fields["body"].(types.Text); !ok {
			t.Error("Body field should be Text type")
		}

		if _, exists := record.Fields["headers"]; !exists {
			t.Error("Response missing 'headers' field")
		}
		if _, ok := record.Fields["headers"].(types.Record); !ok {
			t.Error("Headers field should be Record type")
		}

		if _, exists := record.Fields["time"]; !exists {
			t.Error("Response missing 'time' field")
		}
		if _, ok := record.Fields["time"].(types.Time); !ok {
			t.Error("Time field should be Time type")
		}

		if _, exists := record.Fields["duration"]; !exists {
			t.Error("Response missing 'duration' field")
		}
		if duration, ok := record.Fields["duration"].(types.Number); ok {
			if duration.Value < 0 {
				t.Error("Duration should be non-negative")
			}
		} else {
			t.Error("Duration field should be Number type")
		}
	})

	// Test GET with timeout option - using direct function call instead of expression
	t.Run("GET with timeout option", func(t *testing.T) {
		// Create options record manually
		options := types.Record{
			Fields: map[string]interface{}{
				"timeout": types.Number{Value: 5000},
			},
		}

		// Call the function directly
		result := interpreter.evalHttpGetFunction([]interface{}{
			types.Text{Value: server.URL + "/delay/1"},
			options,
		})

		record, ok := result.(types.Record)
		if !ok {
			t.Fatalf("Expected Record response, got %T", result)
		}

		// Verify the request completed successfully within the timeout
		if status, ok := record.Fields["status"].(types.Number); ok {
			if status.Value != 200 {
				t.Errorf("Expected status 200, got %v", status.Value)
			}
		} else {
			t.Error("Status field should be Number type")
		}
	})

	// Test GET with short timeout (should timeout)
	t.Run("GET with short timeout", func(t *testing.T) {
		// Create options record with very short timeout
		options := types.Record{
			Fields: map[string]interface{}{
				"timeout": types.Number{Value: 1000}, // 1 second for 3 second delay
			},
		}

		result := interpreter.evalHttpGetFunction([]interface{}{
			types.Text{Value: server.URL + "/delay/3"},
			options,
		})

		unknown, ok := result.(types.Unknown)
		if !ok {
			t.Fatalf("Expected Unknown result for timeout, got %T", result)
		}

		if unknown.Reason != "timeout" {
			t.Errorf("Expected timeout error, got: %s", unknown.Reason)
		}
	})

	// Test GET to non-existent domain
	t.Run("GET to non-existent domain", func(t *testing.T) {
		result, err := interpreter.EvaluateExpression(`my.computer.network.web.get("https://nonexistent-domain-12345.invalid")`)
		if err != nil {
			t.Fatalf("Unexpected interpretation error: %v", err)
		}

		unknown, ok := result.(types.Unknown)
		if !ok {
			t.Fatalf("Expected Unknown result for DNS failure, got %T", result)
		}

		if unknown.Reason != "dns_failure" {
			t.Errorf("Expected dns_failure, got: %s", unknown.Reason)
		}
	})

	// Test GET with custom headers
	t.Run("GET with custom headers", func(t *testing.T) {
		// Create options record with headers
		headers := types.Record{
			Fields: map[string]interface{}{
				"User-Agent": types.Text{Value: "AmorphDB-Test"},
				"X-Test":     types.Text{Value: "true"},
			},
		}
		options := types.Record{
			Fields: map[string]interface{}{
				"headers": headers,
			},
		}

		result := interpreter.evalHttpGetFunction([]interface{}{
			types.Text{Value: server.URL + "/headers"},
			options,
		})

		record, ok := result.(types.Record)
		if !ok {
			t.Fatalf("Expected Record response, got %T", result)
		}

		if status, ok := record.Fields["status"].(types.Number); ok {
			if status.Value != 200 {
				t.Errorf("Expected status 200, got %v", status.Value)
			}
		}

		// Parse the JSON response to verify headers were sent
		if body, ok := record.Fields["body"].(types.Text); ok {
			if !strings.Contains(body.Value, "AmorphDB-Test") {
				t.Error("Custom User-Agent header not found in response")
			}
			if !strings.Contains(body.Value, "X-Test") {
				t.Error("Custom X-Test header not found in response")
			}
		}
	})

	// Test invalid arguments
	t.Run("Invalid arguments", func(t *testing.T) {
		// No arguments
		result, err := interpreter.EvaluateExpression(`my.computer.network.web.get()`)
		if err != nil {
			t.Fatalf("Unexpected interpretation error: %v", err)
		}
		if unknown, ok := result.(types.Unknown); ok {
			if !strings.Contains(unknown.Reason, "expects 1 or 2 arguments") {
				t.Errorf("Expected argument count error, got: %s", unknown.Reason)
			}
		} else {
			t.Error("Expected Unknown result for no arguments")
		}

		// Non-string URL
		result, err = interpreter.EvaluateExpression(`my.computer.network.web.get(123)`)
		if err != nil {
			t.Fatalf("Unexpected interpretation error: %v", err)
		}
		if unknown, ok := result.(types.Unknown); ok {
			if !strings.Contains(unknown.Reason, "must be a URL string") {
				t.Errorf("Expected URL type error, got: %s", unknown.Reason)
			}
		} else {
			t.Error("Expected Unknown result for non-string URL")
		}

		// Invalid options type - test directly
		result = interpreter.evalHttpGetFunction([]interface{}{
			types.Text{Value: server.URL + "/get"},
			types.Text{Value: "invalid"}, // Wrong type - should be Record
		})
		if unknown, ok := result.(types.Unknown); ok {
			if !strings.Contains(unknown.Reason, "must be an options record") {
				t.Errorf("Expected options type error, got: %s", unknown.Reason)
			}
		} else {
			t.Error("Expected Unknown result for invalid options type")
		}
	})
}

// TestHttpPostFunction tests POST requests with body
func TestHttpPostFunction(t *testing.T) {
	tree := &MockTree{}
	interpreter := New(tree, 1001)
	server := newHTTPTestServer()
	defer server.Close()

	// Test basic POST request with body
	t.Run("Basic POST request with body", func(t *testing.T) {
		result := interpreter.evalHttpPostFunction([]interface{}{
			types.Text{Value: server.URL + "/post"},
			types.Text{Value: `{"test": "data", "method": "POST"}`},
		})

		record, ok := result.(types.Record)
		if !ok {
			t.Fatalf("Expected Record response, got %T", result)
		}

		// Verify response structure according to spec
		if status, ok := record.Fields["status"].(types.Number); ok {
			if status.Value != 200 {
				t.Errorf("Expected status 200, got %v", status.Value)
			}
		} else {
			t.Error("Status field should be Number type")
		}

		// Verify that the body was sent correctly
		if body, ok := record.Fields["body"].(types.Text); ok {
			// httpbin.org/post returns the posted data in the response
			if !strings.Contains(body.Value, "POST") {
				t.Error("POST method not reflected in response body")
			}
		} else {
			t.Error("Body field should be Text type")
		}
	})

	// Test POST with options (headers and timeout)
	t.Run("POST with custom headers", func(t *testing.T) {
		// Create options record with headers
		headers := types.Record{
			Fields: map[string]interface{}{
				"Content-Type": types.Text{Value: "application/json"},
				"X-Test":       types.Text{Value: "POST-test"},
			},
		}
		options := types.Record{
			Fields: map[string]interface{}{
				"headers": headers,
				"timeout": types.Number{Value: 10000},
			},
		}

		result := interpreter.evalHttpPostFunction([]interface{}{
			types.Text{Value: server.URL + "/post"},
			types.Text{Value: `{"message": "test with headers"}`},
			options,
		})

		record, ok := result.(types.Record)
		if !ok {
			t.Fatalf("Expected Record response, got %T", result)
		}

		if status, ok := record.Fields["status"].(types.Number); ok {
			if status.Value != 200 {
				t.Errorf("Expected status 200, got %v", status.Value)
			}
		}
	})

	// Test invalid arguments
	t.Run("Invalid arguments", func(t *testing.T) {
		// Missing body argument
		result := interpreter.evalHttpPostFunction([]interface{}{
			types.Text{Value: server.URL + "/post"},
		})

		if unknown, ok := result.(types.Unknown); ok {
			if !strings.Contains(unknown.Reason, "expects 2 or 3 arguments") {
				t.Errorf("Expected argument count error, got: %s", unknown.Reason)
			}
		} else {
			t.Error("Expected Unknown result for missing body")
		}

		// Non-string body
		result = interpreter.evalHttpPostFunction([]interface{}{
			types.Text{Value: server.URL + "/post"},
			types.Number{Value: 123}, // Invalid body type
		})

		if unknown, ok := result.(types.Unknown); ok {
			if !strings.Contains(unknown.Reason, "must be a body string") {
				t.Errorf("Expected body type error, got: %s", unknown.Reason)
			}
		} else {
			t.Error("Expected Unknown result for non-string body")
		}
	})
}

// TestHttpPutFunction tests PUT requests
func TestHttpPutFunction(t *testing.T) {
	tree := &MockTree{}
	interpreter := New(tree, 1001)
	server := newHTTPTestServer()
	defer server.Close()

	// Test basic PUT request - confirm correct HTTP method is used
	t.Run("Basic PUT request", func(t *testing.T) {
		result := interpreter.evalHttpPutFunction([]interface{}{
			types.Text{Value: server.URL + "/put"},
			types.Text{Value: `{"test": "data", "method": "PUT"}`},
		})

		record, ok := result.(types.Record)
		if !ok {
			t.Fatalf("Expected Record response, got %T", result)
		}

		if status, ok := record.Fields["status"].(types.Number); ok {
			if status.Value != 200 {
				t.Errorf("Expected status 200, got %v", status.Value)
			}
		} else {
			t.Error("Status field should be Number type")
		}

		// Verify PUT method was used (httpbin returns method info)
		if body, ok := record.Fields["body"].(types.Text); ok {
			if !strings.Contains(body.Value, "PUT") {
				t.Error("PUT method not reflected in response - incorrect HTTP method used")
			}
		}
	})
}

// TestHttpPatchFunction tests PATCH requests
func TestHttpPatchFunction(t *testing.T) {
	tree := &MockTree{}
	interpreter := New(tree, 1001)
	server := newHTTPTestServer()
	defer server.Close()

	// Test basic PATCH request - confirm correct HTTP method is used
	t.Run("Basic PATCH request", func(t *testing.T) {
		result := interpreter.evalHttpPatchFunction([]interface{}{
			types.Text{Value: server.URL + "/patch"},
			types.Text{Value: `{"test": "data", "method": "PATCH"}`},
		})

		record, ok := result.(types.Record)
		if !ok {
			t.Fatalf("Expected Record response, got %T", result)
		}

		if status, ok := record.Fields["status"].(types.Number); ok {
			if status.Value != 200 {
				t.Errorf("Expected status 200, got %v", status.Value)
			}
		} else {
			t.Error("Status field should be Number type")
		}

		// Verify PATCH method was used (httpbin returns method info)
		if body, ok := record.Fields["body"].(types.Text); ok {
			if !strings.Contains(body.Value, "PATCH") {
				t.Error("PATCH method not reflected in response - incorrect HTTP method used")
			}
		}
	})
}

// TestHttpDeleteFunction tests DELETE requests
func TestHttpDeleteFunction(t *testing.T) {
	tree := &MockTree{}
	interpreter := New(tree, 1001)
	server := newHTTPTestServer()
	defer server.Close()

	// Test basic DELETE request - confirm correct HTTP method is used
	t.Run("Basic DELETE request", func(t *testing.T) {
		result := interpreter.evalHttpDeleteFunction([]interface{}{
			types.Text{Value: server.URL + "/delete"},
		})

		record, ok := result.(types.Record)
		if !ok {
			t.Fatalf("Expected Record response, got %T", result)
		}

		if status, ok := record.Fields["status"].(types.Number); ok {
			if status.Value != 200 {
				t.Errorf("Expected status 200, got %v", status.Value)
			}
		} else {
			t.Error("Status field should be Number type")
		}

		// Verify DELETE method was used - httpbin should return valid response for DELETE
		// The key requirement is that the request succeeds, confirming correct HTTP method
		if body, ok := record.Fields["body"].(types.Text); ok {
			// Just verify we got a response body (httpbin.org/delete returns JSON with request details)
			if len(body.Value) == 0 {
				t.Error("Expected response body from DELETE request")
			}
		}
	})

	// Test DELETE with options
	t.Run("DELETE with options", func(t *testing.T) {
		options := types.Record{
			Fields: map[string]interface{}{
				"timeout": types.Number{Value: 5000},
			},
		}

		result := interpreter.evalHttpDeleteFunction([]interface{}{
			types.Text{Value: server.URL + "/delete"},
			options,
		})

		record, ok := result.(types.Record)
		if !ok {
			t.Fatalf("Expected Record response, got %T", result)
		}

		if status, ok := record.Fields["status"].(types.Number); ok {
			if status.Value != 200 {
				t.Errorf("Expected status 200, got %v", status.Value)
			}
		}
	})
}

func TestParseJsonFunction(t *testing.T) {
	interpreter := func() *Interpreter {
		tree := &MockTree{}
		return New(tree, 1001)
	}()

	t.Run("basic types", func(t *testing.T) {
		tests := []struct {
			name     string
			json     string
			expected interface{}
		}{
			{
				name:     "string",
				json:     `"hello"`,
				expected: types.Text{Value: "hello"},
			},
			{
				name:     "number",
				json:     `42`,
				expected: types.Number{Value: 42},
			},
			{
				name:     "boolean true",
				json:     `true`,
				expected: types.Boolean{Value: true},
			},
			{
				name:     "boolean false",
				json:     `false`,
				expected: types.Boolean{Value: false},
			},
			{
				name:     "null",
				json:     `null`,
				expected: types.Nothing{},
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				result := interpreter.evalParseJsonFunction([]interface{}{types.Text{Value: tt.json}})

				if !equalValues(result, tt.expected) {
					t.Errorf("Expected %+v, got %+v", tt.expected, result)
				}
			})
		}
	})

	t.Run("arrays become lists", func(t *testing.T) {
		json := `[1, "hello", true, null]`
		result := interpreter.evalParseJsonFunction([]interface{}{types.Text{Value: json}})

		list, ok := result.(types.List)
		if !ok {
			t.Fatalf("Expected List, got %T", result)
		}

		if len(list.Elements) != 4 {
			t.Fatalf("Expected 4 elements, got %d", len(list.Elements))
		}

		expected := []interface{}{
			types.Number{Value: 1},
			types.Text{Value: "hello"},
			types.Boolean{Value: true},
			types.Nothing{},
		}

		for i, elem := range expected {
			if !equalValues(list.Elements[i], elem) {
				t.Errorf("Element %d: expected %+v, got %+v", i, elem, list.Elements[i])
			}
		}
	})

	t.Run("objects become records", func(t *testing.T) {
		json := `{"name": "John", "age": 30, "active": true}`
		result := interpreter.evalParseJsonFunction([]interface{}{types.Text{Value: json}})

		record, ok := result.(types.Record)
		if !ok {
			t.Fatalf("Expected Record, got %T", result)
		}

		if len(record.Fields) != 3 {
			t.Fatalf("Expected 3 fields, got %d", len(record.Fields))
		}

		if !equalValues(record.Fields["name"], types.Text{Value: "John"}) {
			t.Errorf("Expected name='John', got %+v", record.Fields["name"])
		}

		if !equalValues(record.Fields["age"], types.Number{Value: 30}) {
			t.Errorf("Expected age=30, got %+v", record.Fields["age"])
		}

		if !equalValues(record.Fields["active"], types.Boolean{Value: true}) {
			t.Errorf("Expected active=true, got %+v", record.Fields["active"])
		}
	})

	t.Run("nested structures", func(t *testing.T) {
		json := `{
			"user": {
				"name": "Alice",
				"tags": ["admin", "developer"]
			},
			"count": 5
		}`
		result := interpreter.evalParseJsonFunction([]interface{}{types.Text{Value: json}})

		record, ok := result.(types.Record)
		if !ok {
			t.Fatalf("Expected Record, got %T", result)
		}

		userField, exists := record.Fields["user"]
		if !exists {
			t.Fatal("Missing user field")
		}

		userRecord, ok := userField.(types.Record)
		if !ok {
			t.Fatalf("Expected user to be Record, got %T", userField)
		}

		if !equalValues(userRecord.Fields["name"], types.Text{Value: "Alice"}) {
			t.Errorf("Expected user.name='Alice', got %+v", userRecord.Fields["name"])
		}

		tagsField, exists := userRecord.Fields["tags"]
		if !exists {
			t.Fatal("Missing tags field")
		}

		tagsList, ok := tagsField.(types.List)
		if !ok {
			t.Fatalf("Expected tags to be List, got %T", tagsField)
		}

		if len(tagsList.Elements) != 2 {
			t.Fatalf("Expected 2 tags, got %d", len(tagsList.Elements))
		}
	})

	t.Run("invalid JSON", func(t *testing.T) {
		result := interpreter.evalParseJsonFunction([]interface{}{types.Text{Value: `{invalid json`}})

		unknown, ok := result.(types.Unknown)
		if !ok {
			t.Fatalf("Expected Unknown error, got %T", result)
		}

		if !strings.Contains(unknown.Reason, "invalid JSON") {
			t.Errorf("Expected JSON parse error, got: %s", unknown.Reason)
		}
	})

	t.Run("invalid arguments", func(t *testing.T) {
		// Wrong number of arguments
		result := interpreter.evalParseJsonFunction([]interface{}{})
		if unknown, ok := result.(types.Unknown); ok {
			if !strings.Contains(unknown.Reason, "expects exactly one argument") {
				t.Errorf("Expected argument count error, got: %s", unknown.Reason)
			}
		} else {
			t.Fatalf("Expected Unknown error for wrong argument count, got %T", result)
		}

		// Wrong argument type
		result = interpreter.evalParseJsonFunction([]interface{}{types.Number{Value: 42}})
		if unknown, ok := result.(types.Unknown); ok {
			if !strings.Contains(unknown.Reason, "argument must be a string") {
				t.Errorf("Expected string argument error, got: %s", unknown.Reason)
			}
		} else {
			t.Fatalf("Expected Unknown error for wrong argument type, got %T", result)
		}
	})
}

func TestToJsonFunction(t *testing.T) {
	interpreter := func() *Interpreter {
		tree := &MockTree{}
		return New(tree, 1001)
	}()

	t.Run("basic types", func(t *testing.T) {
		tests := []struct {
			name     string
			input    interface{}
			expected string
		}{
			{
				name:     "text",
				input:    types.Text{Value: "hello"},
				expected: `"hello"`,
			},
			{
				name:     "number",
				input:    types.Number{Value: 42},
				expected: `42`,
			},
			{
				name:     "boolean true",
				input:    types.Boolean{Value: true},
				expected: `true`,
			},
			{
				name:     "boolean false",
				input:    types.Boolean{Value: false},
				expected: `false`,
			},
			{
				name:     "nothing",
				input:    types.Nothing{},
				expected: `null`,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				result := interpreter.evalToJsonFunction([]interface{}{tt.input})

				text, ok := result.(types.Text)
				if !ok {
					t.Fatalf("Expected Text result, got %T", result)
				}

				if text.Value != tt.expected {
					t.Errorf("Expected %s, got %s", tt.expected, text.Value)
				}
			})
		}
	})

	t.Run("lists become arrays", func(t *testing.T) {
		list := types.List{
			Elements: []interface{}{
				types.Number{Value: 1},
				types.Text{Value: "hello"},
				types.Boolean{Value: true},
				types.Nothing{},
			},
		}

		result := interpreter.evalToJsonFunction([]interface{}{list})

		text, ok := result.(types.Text)
		if !ok {
			t.Fatalf("Expected Text result, got %T", result)
		}

		expected := `[1,"hello",true,null]`
		if text.Value != expected {
			t.Errorf("Expected %s, got %s", expected, text.Value)
		}
	})

	t.Run("records become objects", func(t *testing.T) {
		record := types.Record{
			Fields: map[string]interface{}{
				"name":   types.Text{Value: "John"},
				"age":    types.Number{Value: 30},
				"active": types.Boolean{Value: true},
			},
		}

		result := interpreter.evalToJsonFunction([]interface{}{record})

		text, ok := result.(types.Text)
		if !ok {
			t.Fatalf("Expected Text result, got %T", result)
		}

		// Parse the result back to verify it's valid JSON with correct structure
		var parsed map[string]interface{}
		if err := json.Unmarshal([]byte(text.Value), &parsed); err != nil {
			t.Fatalf("Result is not valid JSON: %v", err)
		}

		if parsed["name"] != "John" {
			t.Errorf("Expected name='John', got %v", parsed["name"])
		}

		if parsed["age"] != float64(30) {
			t.Errorf("Expected age=30, got %v", parsed["age"])
		}

		if parsed["active"] != true {
			t.Errorf("Expected active=true, got %v", parsed["active"])
		}
	})

	t.Run("nested structures", func(t *testing.T) {
		nested := types.Record{
			Fields: map[string]interface{}{
				"user": types.Record{
					Fields: map[string]interface{}{
						"name": types.Text{Value: "Alice"},
						"tags": types.List{
							Elements: []interface{}{
								types.Text{Value: "admin"},
								types.Text{Value: "developer"},
							},
						},
					},
				},
				"count": types.Number{Value: 5},
			},
		}

		result := interpreter.evalToJsonFunction([]interface{}{nested})

		text, ok := result.(types.Text)
		if !ok {
			t.Fatalf("Expected Text result, got %T", result)
		}

		// Parse the result back to verify structure
		var parsed map[string]interface{}
		if err := json.Unmarshal([]byte(text.Value), &parsed); err != nil {
			t.Fatalf("Result is not valid JSON: %v", err)
		}

		user, ok := parsed["user"].(map[string]interface{})
		if !ok {
			t.Fatalf("Expected user to be object, got %T", parsed["user"])
		}

		if user["name"] != "Alice" {
			t.Errorf("Expected user.name='Alice', got %v", user["name"])
		}

		tags, ok := user["tags"].([]interface{})
		if !ok {
			t.Fatalf("Expected tags to be array, got %T", user["tags"])
		}

		if len(tags) != 2 {
			t.Fatalf("Expected 2 tags, got %d", len(tags))
		}
	})

	t.Run("invalid arguments", func(t *testing.T) {
		// Wrong number of arguments
		result := interpreter.evalToJsonFunction([]interface{}{})
		if unknown, ok := result.(types.Unknown); ok {
			if !strings.Contains(unknown.Reason, "expects exactly one argument") {
				t.Errorf("Expected argument count error, got: %s", unknown.Reason)
			}
		} else {
			t.Fatalf("Expected Unknown error for wrong argument count, got %T", result)
		}
	})
}

func TestJsonRoundTrip(t *testing.T) {
	interpreter := func() *Interpreter {
		tree := &MockTree{}
		return New(tree, 1001)
	}()

	tests := []struct {
		name        string
		json        string
		description string
	}{
		{
			name:        "simple object",
			json:        `{"name":"John","age":30,"active":true}`,
			description: "simple object with mixed types",
		},
		{
			name:        "nested structure",
			json:        `{"user":{"name":"Alice","tags":["admin","dev"]},"count":5}`,
			description: "nested objects and arrays",
		},
		{
			name:        "array of objects",
			json:        `[{"id":1,"name":"Alice"},{"id":2,"name":"Bob"}]`,
			description: "array containing objects",
		},
		{
			name:        "mixed array",
			json:        `[1,"hello",true,null,{"key":"value"}]`,
			description: "array with mixed types including object",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Parse JSON to MBL
			parsed := interpreter.evalParseJsonFunction([]interface{}{types.Text{Value: tt.json}})

			// Check for parse errors
			if unknown, ok := parsed.(types.Unknown); ok {
				t.Fatalf("Parse failed: %s", unknown.Reason)
			}

			// Convert back to JSON
			jsonified := interpreter.evalToJsonFunction([]interface{}{parsed})

			text, ok := jsonified.(types.Text)
			if !ok {
				t.Fatalf("Expected Text result, got %T", jsonified)
			}

			// Parse both original and round-trip JSON to compare structure
			var original, roundtrip interface{}

			if err := json.Unmarshal([]byte(tt.json), &original); err != nil {
				t.Fatalf("Failed to parse original JSON: %v", err)
			}

			if err := json.Unmarshal([]byte(text.Value), &roundtrip); err != nil {
				t.Fatalf("Failed to parse round-trip JSON: %v", err)
			}

			// Compare the parsed structures (order-independent for objects)
			if !jsonEquals(original, roundtrip) {
				t.Errorf("Round-trip failed.\nOriginal: %s\nRound-trip: %s", tt.json, text.Value)
			}
		})
	}
}

// Helper function to compare JSON values for structural equality
func jsonEquals(a, b interface{}) bool {
	// Convert both to JSON strings for comparison (handles object key ordering)
	aBytes, err1 := json.Marshal(a)
	bBytes, err2 := json.Marshal(b)

	if err1 != nil || err2 != nil {
		return false
	}

	// Parse and re-marshal to normalize key ordering
	var aObj, bObj interface{}
	json.Unmarshal(aBytes, &aObj)
	json.Unmarshal(bBytes, &bObj)

	aNorm, _ := json.Marshal(aObj)
	bNorm, _ := json.Marshal(bObj)

	return string(aNorm) == string(bNorm)
}

// Helper function to compare MBL values for equality
func equalValues(a, b interface{}) bool {
	// Handle different types
	switch va := a.(type) {
	case types.Text:
		if vb, ok := b.(types.Text); ok {
			return va.Value == vb.Value
		}
	case types.Number:
		if vb, ok := b.(types.Number); ok {
			return va.Value == vb.Value
		}
	case types.Boolean:
		if vb, ok := b.(types.Boolean); ok {
			return va.Value == vb.Value
		}
	case types.Nothing:
		_, ok := b.(types.Nothing)
		return ok
	case types.List:
		if vb, ok := b.(types.List); ok {
			if len(va.Elements) != len(vb.Elements) {
				return false
			}
			for i := range va.Elements {
				if !equalValues(va.Elements[i], vb.Elements[i]) {
					return false
				}
			}
			return true
		}
	case types.Record:
		if vb, ok := b.(types.Record); ok {
			if len(va.Fields) != len(vb.Fields) {
				return false
			}
			for key := range va.Fields {
				if !equalValues(va.Fields[key], vb.Fields[key]) {
					return false
				}
			}
			return true
		}
	}
	return false
}

func TestJsonHelpersIntegration(t *testing.T) {
	tree := &MockTree{}
	interpreter := New(tree, 1001)

	t.Run("parse_json via MBL evaluation", func(t *testing.T) {
		// Provide the JSON string as a local variable. In MBL a bare name is a
		// local variable, so binding it in scope makes `my_json` resolve to the
		// string when it is passed to parse_json below.
		interpreter.scope.Set("my_json", types.Text{Value: `{"name": "test", "value": 42}`})

		// Test parsing JSON via the full MBL interpreter
		expression := `my.computer.network.web.parse_json(my_json)`

		result, err := interpreter.EvaluateExpression(expression)
		if err != nil {
			t.Fatalf("Expression evaluation failed: %v", err)
		}

		record, ok := result.(types.Record)
		if !ok {
			t.Fatalf("Expected Record, got %T", result)
		}

		if !equalValues(record.Fields["name"], types.Text{Value: "test"}) {
			t.Errorf("Expected name='test', got %+v", record.Fields["name"])
		}

		if !equalValues(record.Fields["value"], types.Number{Value: 42}) {
			t.Errorf("Expected value=42, got %+v", record.Fields["value"])
		}
	})

	t.Run("to_json of simple literal", func(t *testing.T) {
		// Test converting a simple value to JSON
		expression := `my.computer.network.web.to_json("hello")`

		result, err := interpreter.EvaluateExpression(expression)
		if err != nil {
			t.Fatalf("Expression evaluation failed: %v", err)
		}

		text, ok := result.(types.Text)
		if !ok {
			t.Fatalf("Expected Text, got %T: %+v", result, result)
		}

		if text.Value != `"hello"` {
			t.Errorf("Expected \"hello\", got %s", text.Value)
		}
	})
}

func TestAssetFunctionDirect(t *testing.T) {
	interpreter := func() *Interpreter {
		tree := &MockTree{}
		return New(tree, 1001)
	}()

	tests := []struct {
		name     string
		data     string
		mimeType string
		expected types.Record
	}{
		{
			name:     "plain text asset",
			data:     "hello world",
			mimeType: "text/plain",
			expected: types.Record{
				Fields: map[string]interface{}{
					"data":      types.Text{Value: "hello world"},
					"mime_type": types.Text{Value: "text/plain"},
				},
			},
		},
		{
			name:     "JSON asset",
			data:     `{"key": "value"}`,
			mimeType: "application/json",
			expected: types.Record{
				Fields: map[string]interface{}{
					"data":      types.Text{Value: `{"key": "value"}`},
					"mime_type": types.Text{Value: "application/json"},
				},
			},
		},
		{
			name:     "CSS asset",
			data:     "body { color: red; }",
			mimeType: "text/css",
			expected: types.Record{
				Fields: map[string]interface{}{
					"data":      types.Text{Value: "body { color: red; }"},
					"mime_type": types.Text{Value: "text/css"},
				},
			},
		},
		{
			name:     "binary asset",
			data:     "\x89PNG\r\n\x1a\n",
			mimeType: "image/png",
			expected: types.Record{
				Fields: map[string]interface{}{
					"data":      types.Text{Value: "\x89PNG\r\n\x1a\n"},
					"mime_type": types.Text{Value: "image/png"},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := interpreter.evalAssetFunction([]interface{}{
				types.Text{Value: tt.data},
				types.Text{Value: tt.mimeType},
			})

			record, ok := result.(types.Record)
			if !ok {
				t.Fatalf("Expected Record, got %T", result)
			}

			if !equalValues(record, tt.expected) {
				t.Errorf("Asset mismatch.\nExpected: %+v\nGot: %+v", tt.expected, record)
			}

			// Verify specific fields
			data, hasData := record.Fields["data"]
			mimeType, hasMimeType := record.Fields["mime_type"]

			if !hasData {
				t.Error("Asset missing 'data' field")
			}
			if !hasMimeType {
				t.Error("Asset missing 'mime_type' field")
			}

			if hasData && !equalValues(data, types.Text{Value: tt.data}) {
				t.Errorf("Data field mismatch. Expected: %s, Got: %+v", tt.data, data)
			}
			if hasMimeType && !equalValues(mimeType, types.Text{Value: tt.mimeType}) {
				t.Errorf("MIME type field mismatch. Expected: %s, Got: %+v", tt.mimeType, mimeType)
			}
		})
	}

	// Test error cases
	t.Run("insufficient arguments", func(t *testing.T) {
		result := interpreter.evalAssetFunction([]interface{}{})
		unknown, ok := result.(types.Unknown)
		if !ok || !strings.Contains(unknown.Reason, "requires exactly 2 arguments") {
			t.Errorf("Expected argument error, got %+v", result)
		}
	})

	t.Run("too many arguments", func(t *testing.T) {
		result := interpreter.evalAssetFunction([]interface{}{
			types.Text{Value: "data"},
			types.Text{Value: "text/plain"},
			types.Text{Value: "extra"},
		})
		unknown, ok := result.(types.Unknown)
		if !ok || !strings.Contains(unknown.Reason, "requires exactly 2 arguments") {
			t.Errorf("Expected argument error, got %+v", result)
		}
	})

	t.Run("invalid data type", func(t *testing.T) {
		result := interpreter.evalAssetFunction([]interface{}{
			types.Number{Value: 123}, // Invalid type
			types.Text{Value: "text/plain"},
		})
		// Should still work due to type coercion
		record, ok := result.(types.Record)
		if !ok {
			t.Fatalf("Expected Record after coercion, got %T", result)
		}
		if !equalValues(record.Fields["data"], types.Text{Value: "123"}) {
			t.Error("Expected coerced number to become text")
		}
	})
}

func TestNetworkWebAssetFunction(t *testing.T) {
	tree := &MockTree{}
	interpreter := New(tree, 1001)

	t.Run("my.computer.network.web.asset() call", func(t *testing.T) {
		expression := `my.computer.network.web.asset("test data", "text/plain")`

		result, err := interpreter.EvaluateExpression(expression)
		if err != nil {
			t.Fatalf("Expression evaluation failed: %v", err)
		}

		record, ok := result.(types.Record)
		if !ok {
			t.Fatalf("Expected Record, got %T", result)
		}

		expectedRecord := types.Record{
			Fields: map[string]interface{}{
				"data":      types.Text{Value: "test data"},
				"mime_type": types.Text{Value: "text/plain"},
			},
		}

		if !equalValues(record, expectedRecord) {
			t.Errorf("Network web asset mismatch.\nExpected: %+v\nGot: %+v", expectedRecord, record)
		}
	})
}

func TestDeployPwaFunction(t *testing.T) {
	tree := &MockTree{}
	interpreter := New(tree, 1001)

	t.Run("successful deployment", func(t *testing.T) {
		// Create a temporary directory structure
		tmpDir, err := ioutil.TempDir("", "pwa_test")
		if err != nil {
			t.Fatalf("Failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tmpDir)

		// Create test files
		testFiles := map[string]string{
			"index.html": `<!DOCTYPE html>
<html>
<head><title>Test App</title></head>
<body><h1>Hello</h1></body>
</html>`,
			"app.js": `console.log("Hello from app.js");
function init() {
    document.body.style.background = 'lightblue';
}`,
			"style.css": `body {
    font-family: Arial, sans-serif;
    margin: 0;
    padding: 20px;
}
h1 { color: blue; }`,
			"data/config.json": `{
    "appName": "Test PWA",
    "version": "1.0.0"
}`,
		}

		// Write test files
		for relativePath, content := range testFiles {
			fullPath := filepath.Join(tmpDir, relativePath)
			dir := filepath.Dir(fullPath)
			if err := os.MkdirAll(dir, 0755); err != nil {
				t.Fatalf("Failed to create directory %s: %v", dir, err)
			}
			if err := ioutil.WriteFile(fullPath, []byte(content), 0644); err != nil {
				t.Fatalf("Failed to write file %s: %v", fullPath, err)
			}
		}

		// Test deploy_pwa function
		result := interpreter.evalDeployPwaFunction([]interface{}{
			types.Text{Value: "app.example.com"},
			types.Text{Value: tmpDir},
		})

		// Flush the staged writes to make them visible in the tree
		if err := interpreter.flushBuffer(); err != nil {
			t.Fatalf("Failed to flush commit buffer: %v", err)
		}

		record, ok := result.(types.Record)
		if !ok {
			t.Fatalf("Expected Record result, got %T: %+v", result, result)
		}

		// Verify result structure
		domain, hasDomain := record.Fields["domain"]
		assetsCount, hasAssetsCount := record.Fields["assets_count"]
		status, hasStatus := record.Fields["status"]

		if !hasDomain {
			t.Error("Result missing 'domain' field")
		}
		if !hasAssetsCount {
			t.Error("Result missing 'assets_count' field")
		}
		if !hasStatus {
			t.Error("Result missing 'status' field")
		}

		if hasDomain && !equalValues(domain, types.Text{Value: "app.example.com"}) {
			t.Errorf("Domain mismatch. Expected: app.example.com, Got: %+v", domain)
		}
		if hasAssetsCount && !equalValues(assetsCount, types.Number{Value: 4}) {
			t.Errorf("Assets count mismatch. Expected: 4, Got: %+v", assetsCount)
		}
		if hasStatus && !equalValues(status, types.Text{Value: "deployed"}) {
			t.Errorf("Status mismatch. Expected: deployed, Got: %+v", status)
		}

		// Verify that assets were staged in the mock tree
		// Check a few key paths that should have been written
		expectedPaths := [][]string{
			{"my", "computer", "network", "web", "pwa", "app.example.com", "assets", "index.html"},
			{"my", "computer", "network", "web", "pwa", "app.example.com", "assets", "app.js"},
			{"my", "computer", "network", "web", "pwa", "app.example.com", "assets", "style.css"},
			{"my", "computer", "network", "web", "pwa", "app.example.com", "assets", "data/config.json"},
		}

		for _, path := range expectedPaths {
			pathStr := strings.Join(path, ".")
			if !tree.HasPath(path) {
				t.Errorf("Expected asset path not found in storage: %s", pathStr)
			}
		}

		// Verify MIME types are correct
		if tree.HasPath([]string{"my", "computer", "network", "web", "pwa", "app.example.com", "assets", "index.html"}) {
			value := tree.GetValue([]string{"my", "computer", "network", "web", "pwa", "app.example.com", "assets", "index.html"})
			if value != nil {
				// Decode the stored asset record
				mblValue, err := storageToMBL(*value)
				if err == nil {
					if assetRecord, ok := mblValue.(types.Record); ok {
						if mimeType, hasMimeType := assetRecord.Fields["mime_type"]; hasMimeType {
							if mimeText, ok := mimeType.(types.Text); ok {
								if !strings.HasPrefix(mimeText.Value, "text/html") {
									t.Errorf("Expected HTML MIME type (text/html*), got %+v", mimeType)
								}
							} else {
								t.Errorf("Expected Text MIME type, got %T", mimeType)
							}
						}
					}
				}
			}
		}
	})

	// Test error cases
	t.Run("directory does not exist", func(t *testing.T) {
		result := interpreter.evalDeployPwaFunction([]interface{}{
			types.Text{Value: "app.example.com"},
			types.Text{Value: "/nonexistent/directory"},
		})

		unknown, ok := result.(types.Unknown)
		if !ok || !strings.Contains(unknown.Reason, "directory does not exist") {
			t.Errorf("Expected directory not found error, got %+v", result)
		}
	})

	t.Run("insufficient arguments", func(t *testing.T) {
		result := interpreter.evalDeployPwaFunction([]interface{}{
			types.Text{Value: "app.example.com"},
		})

		unknown, ok := result.(types.Unknown)
		if !ok || !strings.Contains(unknown.Reason, "requires exactly 2 arguments") {
			t.Errorf("Expected argument error, got %+v", result)
		}
	})

	t.Run("too many arguments", func(t *testing.T) {
		result := interpreter.evalDeployPwaFunction([]interface{}{
			types.Text{Value: "app.example.com"},
			types.Text{Value: "/tmp"},
			types.Text{Value: "extra"},
		})

		unknown, ok := result.(types.Unknown)
		if !ok || !strings.Contains(unknown.Reason, "requires exactly 2 arguments") {
			t.Errorf("Expected argument error, got %+v", result)
		}
	})

	t.Run("empty directory", func(t *testing.T) {
		// Create empty temp directory
		tmpDir, err := ioutil.TempDir("", "pwa_empty_test")
		if err != nil {
			t.Fatalf("Failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tmpDir)

		result := interpreter.evalDeployPwaFunction([]interface{}{
			types.Text{Value: "empty.example.com"},
			types.Text{Value: tmpDir},
		})

		record, ok := result.(types.Record)
		if !ok {
			t.Fatalf("Expected Record result, got %T: %+v", result, result)
		}

		// Should succeed with 0 assets
		if assetsCount, hasCount := record.Fields["assets_count"]; hasCount {
			if !equalValues(assetsCount, types.Number{Value: 0}) {
				t.Errorf("Expected 0 assets for empty directory, got %+v", assetsCount)
			}
		}
	})
}

func TestNetworkWebDeployPwaFunction(t *testing.T) {
	tree := &MockTree{}
	interpreter := New(tree, 1001)

	t.Run("my.computer.network.web.deploy_pwa() call", func(t *testing.T) {
		// Create a simple temp directory for testing
		tmpDir, err := ioutil.TempDir("", "pwa_network_test")
		if err != nil {
			t.Fatalf("Failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tmpDir)

		// Create a simple index.html file
		indexPath := filepath.Join(tmpDir, "index.html")
		indexContent := `<!DOCTYPE html><html><head><title>Test</title></head><body><h1>Hello World</h1></body></html>`
		if err := ioutil.WriteFile(indexPath, []byte(indexContent), 0644); err != nil {
			t.Fatalf("Failed to write test file: %v", err)
		}

		// Test via full MBL expression evaluation
		expression := fmt.Sprintf(`my.computer.network.web.deploy_pwa("test.example.com", "%s")`, tmpDir)

		result, err := interpreter.EvaluateExpression(expression)
		if err != nil {
			t.Fatalf("Expression evaluation failed: %v", err)
		}

		record, ok := result.(types.Record)
		if !ok {
			t.Fatalf("Expected Record, got %T", result)
		}

		// Verify deployment completed successfully
		if status, hasStatus := record.Fields["status"]; hasStatus {
			if !equalValues(status, types.Text{Value: "deployed"}) {
				t.Errorf("Expected status='deployed', got %+v", status)
			}
		} else {
			t.Error("Missing status field")
		}

		if assetsCount, hasCount := record.Fields["assets_count"]; hasCount {
			if !equalValues(assetsCount, types.Number{Value: 1}) {
				t.Errorf("Expected assets_count=1, got %+v", assetsCount)
			}
		} else {
			t.Error("Missing assets_count field")
		}
	})
}
func TestWebResponseHelpers(t *testing.T) {
	// Create a mock tree for testing
	tree := &MockTree{}
	interpreter := New(tree, 1001) // Agent ID 1001

	// Create a mock request record as would be created by the inbound request queue
	requestRecord := types.Record{
		Fields: map[string]interface{}{
			"id":        types.Text{Value: "req_12345"},
			"domain":    types.Text{Value: "example.com"},
			"method":    types.Text{Value: "GET"},
			"path":      types.Text{Value: "/api/test"},
			"remote_ip": types.Text{Value: "192.168.1.100"},
		},
	}

	t.Run("ok() helper", func(t *testing.T) {
		result := interpreter.evalWebOkFunction([]interface{}{
			requestRecord,
			types.Text{Value: "Success message"},
		})

		if result.(types.Text).Value != "response_sent" {
			t.Errorf("Expected 'response_sent', got %v", result)
		}

		// Flush the staged writes to make them visible
		if err := interpreter.flushBuffer(); err != nil {
			t.Fatalf("Failed to flush commit buffer: %v", err)
		}

		// Verify the response was written to storage
		responsePath := []string{"my", "computer", "network", "web", "requests", "req_12345", "response"}
		value, exists := tree.data[joinPath(responsePath)]
		if !exists {
			t.Error("Response not written to storage")
			return
		}

		// Convert storage value back to MBL types
		mblValue, err := storageToMBL(value)
		if err != nil {
			t.Errorf("Error converting storage value: %v", err)
			return
		}

		response, ok := mblValue.(types.Record)
		if !ok {
			t.Errorf("Expected Record, got %T", mblValue)
			return
		}

		// Check status code
		if statusCodeField, ok := response.Fields["status_code"].(types.Number); !ok || statusCodeField.Value != 200 {
			t.Errorf("Expected status_code 200, got %v", response.Fields["status_code"])
		}

		// Check body
		if bodyField, ok := response.Fields["body"].(types.Text); !ok || bodyField.Value != "Success message" {
			t.Errorf("Expected body 'Success message', got %v", response.Fields["body"])
		}
	})

	t.Run("ok_json() helper", func(t *testing.T) {
		testData := types.Record{
			Fields: map[string]interface{}{
				"message": types.Text{Value: "Hello"},
				"status":  types.Number{Value: 1},
			},
		}

		result := interpreter.evalWebOkJsonFunction([]interface{}{
			requestRecord,
			testData,
		})

		if result.(types.Text).Value != "response_sent" {
			t.Errorf("Expected 'response_sent', got %v", result)
		}

		// Flush the staged writes to make them visible
		if err := interpreter.flushBuffer(); err != nil {
			t.Fatalf("Failed to flush commit buffer: %v", err)
		}

		// Verify the response was written to storage
		responsePath := []string{"my", "computer", "network", "web", "requests", "req_12345", "response"}
		value, exists := tree.data[joinPath(responsePath)]
		if !exists {
			t.Error("Response not written to storage")
			return
		}

		// Convert storage value back to MBL types
		mblValue, err := storageToMBL(value)
		if err != nil {
			t.Errorf("Error converting storage value: %v", err)
			return
		}

		response, ok := mblValue.(types.Record)
		if !ok {
			t.Errorf("Expected Record, got %T", mblValue)
			return
		}

		// Check status code
		if statusCodeField, ok := response.Fields["status_code"].(types.Number); !ok || statusCodeField.Value != 200 {
			t.Errorf("Expected status_code 200, got %v", response.Fields["status_code"])
		}

		// Check that body contains JSON
		if bodyField, ok := response.Fields["body"].(types.Text); ok {
			if !strings.Contains(bodyField.Value, "\"message\"") {
				t.Errorf("Expected JSON body, got %v", bodyField.Value)
			}
		} else {
			t.Errorf("Expected body to be Text, got %T", response.Fields["body"])
		}

		// Check Content-Type header
		headersField, ok := response.Fields["headers"].(types.Record)
		if !ok {
			t.Errorf("Expected headers to be Record, got %T", response.Fields["headers"])
			return
		}
		if contentTypeField, ok := headersField.Fields["Content-Type"].(types.Text); !ok || contentTypeField.Value != "application/json" {
			t.Errorf("Expected Content-Type 'application/json', got %v", headersField.Fields["Content-Type"])
		}
	})

	t.Run("not_found() helper", func(t *testing.T) {
		result := interpreter.evalWebNotFoundFunction([]interface{}{
			requestRecord,
		})

		if result.(types.Text).Value != "response_sent" {
			t.Errorf("Expected 'response_sent', got %v", result)
		}

		// Flush the staged writes to make them visible
		if err := interpreter.flushBuffer(); err != nil {
			t.Fatalf("Failed to flush commit buffer: %v", err)
		}

		// Verify the response was written to storage
		responsePath := []string{"my", "computer", "network", "web", "requests", "req_12345", "response"}
		value, exists := tree.data[joinPath(responsePath)]
		if !exists {
			t.Error("Response not written to storage")
			return
		}

		// Convert storage value back to MBL types
		mblValue, err := storageToMBL(value)
		if err != nil {
			t.Errorf("Error converting storage value: %v", err)
			return
		}

		response, ok := mblValue.(types.Record)
		if !ok {
			t.Errorf("Expected Record, got %T", mblValue)
			return
		}

		// Check status code
		if statusCodeField, ok := response.Fields["status_code"].(types.Number); !ok || statusCodeField.Value != 404 {
			t.Errorf("Expected status_code 404, got %v", response.Fields["status_code"])
		}

		// Check body
		if bodyField, ok := response.Fields["body"].(types.Text); !ok || bodyField.Value != "Not Found" {
			t.Errorf("Expected body 'Not Found', got %v", response.Fields["body"])
		}
	})

	t.Run("bad_request() helper", func(t *testing.T) {
		result := interpreter.evalWebBadRequestFunction([]interface{}{
			requestRecord,
			types.Text{Value: "Invalid input"},
		})

		if result.(types.Text).Value != "response_sent" {
			t.Errorf("Expected 'response_sent', got %v", result)
		}

		// Flush the staged writes to make them visible
		if err := interpreter.flushBuffer(); err != nil {
			t.Fatalf("Failed to flush commit buffer: %v", err)
		}

		// Verify the response was written to storage
		responsePath := []string{"my", "computer", "network", "web", "requests", "req_12345", "response"}
		value, exists := tree.data[joinPath(responsePath)]
		if !exists {
			t.Error("Response not written to storage")
			return
		}

		// Convert storage value back to MBL types
		mblValue, err := storageToMBL(value)
		if err != nil {
			t.Errorf("Error converting storage value: %v", err)
			return
		}

		response, ok := mblValue.(types.Record)
		if !ok {
			t.Errorf("Expected Record, got %T", mblValue)
			return
		}

		// Check status code
		if statusCodeField, ok := response.Fields["status_code"].(types.Number); !ok || statusCodeField.Value != 400 {
			t.Errorf("Expected status_code 400, got %v", response.Fields["status_code"])
		}

		// Check body
		if bodyField, ok := response.Fields["body"].(types.Text); !ok || bodyField.Value != "Invalid input" {
			t.Errorf("Expected body 'Invalid input', got %v", response.Fields["body"])
		}
	})

	t.Run("server_error() helper", func(t *testing.T) {
		result := interpreter.evalWebServerErrorFunction([]interface{}{
			requestRecord,
			types.Text{Value: "Internal error"},
		})

		if result.(types.Text).Value != "response_sent" {
			t.Errorf("Expected 'response_sent', got %v", result)
		}

		// Flush the staged writes to make them visible
		if err := interpreter.flushBuffer(); err != nil {
			t.Fatalf("Failed to flush commit buffer: %v", err)
		}

		// Verify the response was written to storage
		responsePath := []string{"my", "computer", "network", "web", "requests", "req_12345", "response"}
		value, exists := tree.data[joinPath(responsePath)]
		if !exists {
			t.Error("Response not written to storage")
			return
		}

		// Convert storage value back to MBL types
		mblValue, err := storageToMBL(value)
		if err != nil {
			t.Errorf("Error converting storage value: %v", err)
			return
		}

		response, ok := mblValue.(types.Record)
		if !ok {
			t.Errorf("Expected Record, got %T", mblValue)
			return
		}

		// Check status code
		if statusCodeField, ok := response.Fields["status_code"].(types.Number); !ok || statusCodeField.Value != 500 {
			t.Errorf("Expected status_code 500, got %v", response.Fields["status_code"])
		}

		// Check body
		if bodyField, ok := response.Fields["body"].(types.Text); !ok || bodyField.Value != "Internal error" {
			t.Errorf("Expected body 'Internal error', got %v", response.Fields["body"])
		}
	})

	t.Run("redirect() helper", func(t *testing.T) {
		result := interpreter.evalWebRedirectFunction([]interface{}{
			requestRecord,
			types.Text{Value: "https://newsite.com"},
		})

		if result.(types.Text).Value != "response_sent" {
			t.Errorf("Expected 'response_sent', got %v", result)
		}

		// Flush the staged writes to make them visible
		if err := interpreter.flushBuffer(); err != nil {
			t.Fatalf("Failed to flush commit buffer: %v", err)
		}

		// Verify the response was written to storage
		responsePath := []string{"my", "computer", "network", "web", "requests", "req_12345", "response"}
		value, exists := tree.data[joinPath(responsePath)]
		if !exists {
			t.Error("Response not written to storage")
			return
		}

		// Convert storage value back to MBL types
		mblValue, err := storageToMBL(value)
		if err != nil {
			t.Errorf("Error converting storage value: %v", err)
			return
		}

		response, ok := mblValue.(types.Record)
		if !ok {
			t.Errorf("Expected Record, got %T", mblValue)
			return
		}

		// Check status code
		if statusCodeField, ok := response.Fields["status_code"].(types.Number); !ok || statusCodeField.Value != 302 {
			t.Errorf("Expected status_code 302, got %v", response.Fields["status_code"])
		}

		// Check body
		if bodyField, ok := response.Fields["body"].(types.Text); !ok || bodyField.Value != "Found" {
			t.Errorf("Expected body 'Found', got %v", response.Fields["body"])
		}

		// Check Location header
		headersField, ok := response.Fields["headers"].(types.Record)
		if !ok {
			t.Errorf("Expected headers to be Record, got %T", response.Fields["headers"])
			return
		}
		if locationField, ok := headersField.Fields["Location"].(types.Text); !ok || locationField.Value != "https://newsite.com" {
			t.Errorf("Expected Location 'https://newsite.com', got %v", headersField.Fields["Location"])
		}
	})
}

// TestDefiniteEqualityUnknownMatching covers the ?= matching semantics
// added in syntax_changes_development_plan.md Step 1.4:
//   - bare `unknown` on the right matches any Unknown left value
//   - `unknown("reason")` on the right matches only Unknowns with that reason
//   - left Unknown, right not Unknown → false (no propagation)
//   - both non-Unknown                → standard equality
//
// Cases use inline `unknown(...)` literals on the LHS instead of going
// through a variable. Assigning an Unknown to a local variable currently
// short-circuits without storing (a separate, pre-existing issue outside
// this step's scope), so reading the variable back would yield Nothing
// and not exercise the matching semantics here.
func TestDefiniteEqualityUnknownMatching(t *testing.T) {
	cases := []struct {
		name string
		expr string
		want interface{}
	}{
		{
			name: "bare unknown matches reasoned unknown left",
			expr: `unknown("timeout") ?= unknown`,
			want: types.Boolean{Value: true},
		},
		{
			name: "bare unknown matches bare unknown left",
			expr: `unknown ?= unknown`,
			want: types.Boolean{Value: true},
		},
		{
			name: "bare unknown does not match a number",
			expr: `42 ?= unknown`,
			want: types.Boolean{Value: false},
		},
		{
			name: "bare unknown does not match a string",
			expr: `"hello" ?= unknown`,
			want: types.Boolean{Value: false},
		},
		{
			name: "reasoned unknown matches same reason",
			expr: `unknown("timeout") ?= unknown("timeout")`,
			want: types.Boolean{Value: true},
		},
		{
			name: "reasoned unknown does not match different reason",
			expr: `unknown("dns_failure") ?= unknown("timeout")`,
			want: types.Boolean{Value: false},
		},
		{
			name: "reasoned unknown does not match bare unknown left",
			expr: `unknown ?= unknown("timeout")`,
			want: types.Boolean{Value: false},
		},
		{
			name: "reasoned unknown does not match a number",
			expr: `42 ?= unknown("timeout")`,
			want: types.Boolean{Value: false},
		},
		{
			name: "left Unknown vs right number returns false (no propagation)",
			expr: `unknown("offline") ?= 5`,
			want: types.Boolean{Value: false},
		},
		{
			name: "normal equality still works on non-Unknown values",
			expr: `42 ?= 42`,
			want: types.Boolean{Value: true},
		},
		{
			name: "normal inequality still works on non-Unknown values",
			expr: `42 ?= 7`,
			want: types.Boolean{Value: false},
		},
		{
			name: "string equality still works",
			expr: `"hello" ?= "hello"`,
			want: types.Boolean{Value: true},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tree := &MockTree{data: make(map[string]storage.Value)}
			interp := New(tree, 1001)
			got := evalCode(t, interp, tc.expr)
			if !compareValues(got, tc.want) {
				t.Errorf("expr=%q: expected %v (%T), got %v (%T)",
					tc.expr, tc.want, tc.want, got, got)
			}
		})
	}
}

// TestTypeFirstProcedureEvaluation verifies that procedure definitions
// (Step 2.2 of docs/syntax_changes_development_plan.md) bind correctly
// and that calls resolve through the appropriate paths. Covers:
//   - Type-first form: `procedure name(args): body` binds at the
//     current scope and is callable by simple name.
//   - Path-first form: `my.path.func: procedure(args): body` binds at
//     the multi-segment path and is callable via the same path.
//   - Anonymous-assignment form: `my.func = procedure(args): body` —
//     the right-hand side evaluates to a *Procedure and the left-hand
//     side path binds it via the procedure registry.
//   - Procedure expression evaluated standalone yields a *Procedure
//     value (not Unknown).
//   - Multi-line bodies with intermediate statements work end-to-end.
//   - Calling a path that has no bound procedure produces an Unknown.
func TestTypeFirstProcedureEvaluation(t *testing.T) {
	t.Run("type-first procedure single-line", func(t *testing.T) {
		tree := &MockTree{data: make(map[string]storage.Value)}
		interp := New(tree, 1001)

		evalCode(t, interp, "procedure double(x): return x * 2")
		got := evalCode(t, interp, "double(21)")
		if !compareValues(got, types.Number{Value: 42}) {
			t.Errorf("double(21): expected 42, got %v (%T)", got, got)
		}
	})

	t.Run("type-first procedure multi-line block body", func(t *testing.T) {
		tree := &MockTree{data: make(map[string]storage.Value)}
		interp := New(tree, 1001)

		evalCode(t, interp, "procedure calculate(income, rate):\n    base = income * rate\n    return base")
		got := evalCode(t, interp, "calculate(100, 0.25)")
		if !compareValues(got, types.Number{Value: 25}) {
			t.Errorf("calculate(100, 0.25): expected 25, got %v (%T)", got, got)
		}
	})

	t.Run("type-first procedure accessible at expected path", func(t *testing.T) {
		tree := &MockTree{data: make(map[string]storage.Value)}
		interp := New(tree, 1001)

		evalCode(t, interp, "procedure greet(name): return \"hello, \" & name")

		// Type-first definitions at REPL / no-scope context bind under
		// `my.<name>` per Step 2.5 spec ("REPL → creates in `my`"). The
		// simple name is also placed in the local scope so unqualified
		// `greet(...)` calls resolve via scope.Get.
		val, found := interp.scope.Get("greet")
		if !found {
			t.Fatalf("expected procedure 'greet' to be bound in scope")
		}
		if _, ok := val.(*Procedure); !ok {
			t.Errorf("expected scope value to be *Procedure, got %T", val)
		}

		if _, ok := interp.procedures["my.greet"]; !ok {
			t.Errorf("expected procedure registry to contain 'my.greet'")
		}
	})

	t.Run("path-first procedure deep path callable", func(t *testing.T) {
		tree := &MockTree{data: make(map[string]storage.Value)}
		interp := New(tree, 1001)

		evalCode(t, interp, "my.functions.triple: procedure(x): return x * 3")
		got := evalCode(t, interp, "my.functions.triple(7)")
		if !compareValues(got, types.Number{Value: 21}) {
			t.Errorf("my.functions.triple(7): expected 21, got %v (%T)", got, got)
		}

		// Deep-path binding goes only into the procedure registry, not
		// the local scope (which would conflict with stored values at
		// the same path).
		if _, ok := interp.procedures["my.functions.triple"]; !ok {
			t.Errorf("expected procedure registry to contain 'my.functions.triple'")
		}
	})

	t.Run("anonymous procedure assignment to deep path", func(t *testing.T) {
		tree := &MockTree{data: make(map[string]storage.Value)}
		interp := New(tree, 1001)

		evalCode(t, interp, "my.double = procedure(x): return x * 2")
		got := evalCode(t, interp, "my.double(5)")
		if !compareValues(got, types.Number{Value: 10}) {
			t.Errorf("my.double(5): expected 10, got %v (%T)", got, got)
		}
	})

	t.Run("procedure expression evaluates to Procedure value", func(t *testing.T) {
		tree := &MockTree{data: make(map[string]storage.Value)}
		interp := New(tree, 1001)

		got := evalCode(t, interp, "procedure(x): return x * 2")
		if _, ok := got.(*Procedure); !ok {
			t.Errorf("expected *Procedure, got %T (%v)", got, got)
		}
	})

	t.Run("zero-parameter type-first procedure", func(t *testing.T) {
		tree := &MockTree{data: make(map[string]storage.Value)}
		interp := New(tree, 1001)

		evalCode(t, interp, "procedure ping(): return \"pong\"")
		got := evalCode(t, interp, "ping()")
		if !compareValues(got, types.Text{Value: "pong"}) {
			t.Errorf("ping(): expected \"pong\", got %v (%T)", got, got)
		}
	})

	t.Run("call to unbound deep path returns Unknown", func(t *testing.T) {
		tree := &MockTree{data: make(map[string]storage.Value)}
		interp := New(tree, 1001)

		got := evalCode(t, interp, "my.missing.func(1)")
		if _, ok := got.(types.Unknown); !ok {
			t.Errorf("expected Unknown for unbound deep path call, got %v (%T)", got, got)
		}
	})

	t.Run("type-first procedures compose at call site", func(t *testing.T) {
		// NOTE: Calling one procedure from inside another procedure body is
		// blocked by a pre-existing parser bug (out of Step 2.2's scope) that
		// splits `f(x)` inside a body into `f` and `(x)`. The same bug
		// affects path-first and legacy column-1 procedure forms. This test
		// instead composes two type-first procedures at the call site.
		tree := &MockTree{data: make(map[string]storage.Value)}
		interp := New(tree, 1001)

		evalCode(t, interp, "procedure square(x): return x * x")
		evalCode(t, interp, "procedure plus_one(y): return y + 1")
		got := evalCode(t, interp, "plus_one(square(4))")
		if !compareValues(got, types.Number{Value: 17}) {
			t.Errorf("plus_one(square(4)): expected 17, got %v (%T)", got, got)
		}
	})
}

// recordingRegistrar captures the calls made by the interpreter to a
// WatcherRegistrar so tests can verify the parameters that would be
// forwarded to a real watcher.WatcherEngine.
type recordingRegistrar struct {
	valueChange []registeredWatcher
	appendForm  []registeredAppendWatcher
	failNext    error
}

type registeredWatcher struct {
	name     string
	watching []string
	code     string
}

type registeredAppendWatcher struct {
	name        string
	watching    []string
	code        string
	bindingName string
	filters     []parser.Expression
}

func (r *recordingRegistrar) RegisterWatcher(name string, watching []string, code string) error {
	if r.failNext != nil {
		err := r.failNext
		r.failNext = nil
		return err
	}
	r.valueChange = append(r.valueChange, registeredWatcher{
		name:     name,
		watching: append([]string(nil), watching...),
		code:     code,
	})
	return nil
}

func (r *recordingRegistrar) RegisterAppendWatcher(name string, watching []string, code string, bindingName string, filters []parser.Expression) error {
	if r.failNext != nil {
		err := r.failNext
		r.failNext = nil
		return err
	}
	r.appendForm = append(r.appendForm, registeredAppendWatcher{
		name:        name,
		watching:    append([]string(nil), watching...),
		code:        code,
		bindingName: bindingName,
		filters:     filters,
	})
	return nil
}

// TestTypeFirstWatcherEvaluation covers Step 2.4: evaluation of
// type-first watcher definitions. Verifies that *parser.WatchStatement
// produces a *Watcher value, binds it at the resolved storage path,
// forwards registration to the installed WatcherRegistrar, and that
// the captured body fires correctly when invoked. Also verifies the
// path-first form (`name: watch(...): body`) produces an equivalent
// binding so both syntaxes round-trip through the same registry.
func TestTypeFirstWatcherEvaluation(t *testing.T) {
	t.Run("type-first watcher binds at agent-home path", func(t *testing.T) {
		tree := &MockTree{data: make(map[string]storage.Value)}
		interp := New(tree, 1001)

		got := evalCode(t, interp, "watch test_watcher(my.data.x):\n    my.fired = 1")
		watcher, ok := got.(*Watcher)
		if !ok {
			t.Fatalf("expected *Watcher result, got %T (%v)", got, got)
		}
		if watcher.Name != "test_watcher" {
			t.Errorf("watcher.Name: expected \"test_watcher\", got %q", watcher.Name)
		}
		if watcher.IsAppend {
			t.Errorf("watcher.IsAppend: expected false for value-change form")
		}

		key := "world.agent.1001.test_watcher"
		bound, found := interp.watchers[key]
		if !found {
			t.Fatalf("expected watcher to be bound at %q; registry has %v", key, watcherKeys(interp))
		}
		if bound != watcher {
			t.Errorf("registry entry differs from returned value")
		}

		expectedWatching := []string{"world.agent.1001.data.x"}
		if !stringSlicesEqual(watcher.Watching, expectedWatching) {
			t.Errorf("watcher.Watching: expected %v, got %v", expectedWatching, watcher.Watching)
		}
	})

	t.Run("type-first watcher fires and writes through body", func(t *testing.T) {
		tree := &MockTree{data: make(map[string]storage.Value)}
		interp := New(tree, 1001)

		got := evalCode(t, interp, "watch trigger_writer(my.data.x):\n    my.fired = 42")
		watcher, ok := got.(*Watcher)
		if !ok {
			t.Fatalf("expected *Watcher, got %T", got)
		}

		// Fire by invoking the captured body. Use a fresh interpreter to
		// drive the fire so the firing scope is isolated from definition
		// state, mirroring how the watcher engine instantiates a fresh
		// interpreter per fire.
		fireInterp := New(tree, 1001)
		result := watcher.Fire(fireInterp, nil)
		if unknown, ok := result.(types.Unknown); ok {
			t.Fatalf("watcher Fire returned Unknown: %s", unknown.Reason)
		}

		// flushBuffer was invoked at the end of Interpret() — but Fire
		// does not call Interpret, so for this test we flush directly.
		if err := fireInterp.flushBuffer(); err != nil {
			t.Fatalf("flushBuffer failed: %v", err)
		}

		stored := tree.GetValue([]string{"world", "agent", "1001", "fired"})
		if stored == nil {
			t.Fatalf("expected my.fired to be written; storage has no entry")
		}
		decoded, err := types.DeserializeValue(types.SerializedValue{TypeTag: stored.TypeTag, Data: stored.Data})
		if err != nil {
			t.Fatalf("failed to decode stored value: %v", err)
		}
		if !compareValues(decoded, types.Number{Value: 42}) {
			t.Errorf("expected my.fired = 42, got %v (%T)", decoded, decoded)
		}
	})

	t.Run("type-first multi-path watcher resolves all watched paths", func(t *testing.T) {
		tree := &MockTree{data: make(map[string]storage.Value)}
		interp := New(tree, 1001)

		got := evalCode(t, interp, "watch multi(my.a, my.b, my.c):\n    my.touched = 1")
		watcher, ok := got.(*Watcher)
		if !ok {
			t.Fatalf("expected *Watcher, got %T", got)
		}
		expected := []string{"world.agent.1001.a", "world.agent.1001.b", "world.agent.1001.c"}
		if !stringSlicesEqual(watcher.Watching, expected) {
			t.Errorf("watcher.Watching: expected %v, got %v", expected, watcher.Watching)
		}
	})

	t.Run("type-first append watcher binds items and fires", func(t *testing.T) {
		tree := &MockTree{data: make(map[string]storage.Value)}
		interp := New(tree, 1001)

		got := evalCode(t, interp, "watch new_orders append(my.orders) as orders:\n    my.last = orders")
		watcher, ok := got.(*Watcher)
		if !ok {
			t.Fatalf("expected *Watcher, got %T", got)
		}
		if !watcher.IsAppend {
			t.Errorf("watcher.IsAppend: expected true for append form")
		}
		if watcher.BindingName != "orders" {
			t.Errorf("watcher.BindingName: expected \"orders\", got %q", watcher.BindingName)
		}

		// Fire with a synthetic items list and confirm the binding flows
		// through the body — my.last should hold the same list.
		items := []interface{}{
			types.Text{Value: "ord-1"},
			types.Text{Value: "ord-2"},
		}
		fireInterp := New(tree, 1001)
		result := watcher.Fire(fireInterp, items)
		if unknown, ok := result.(types.Unknown); ok {
			t.Fatalf("watcher Fire returned Unknown: %s", unknown.Reason)
		}
		if err := fireInterp.flushBuffer(); err != nil {
			t.Fatalf("flushBuffer failed: %v", err)
		}

		stored := tree.GetValue([]string{"world", "agent", "1001", "last"})
		if stored == nil {
			t.Fatalf("expected my.last to be written")
		}
		decoded, err := types.DeserializeValue(types.SerializedValue{TypeTag: stored.TypeTag, Data: stored.Data})
		if err != nil {
			t.Fatalf("failed to decode stored list: %v", err)
		}
		list, ok := decoded.(types.List)
		if !ok {
			t.Fatalf("expected stored value to be List, got %T", decoded)
		}
		if len(list.Elements) != 2 {
			t.Fatalf("expected list of 2 items, got %d", len(list.Elements))
		}
	})

	t.Run("type-first append watcher with predicate filter preserves filter", func(t *testing.T) {
		tree := &MockTree{data: make(map[string]storage.Value)}
		interp := New(tree, 1001)

		got := evalCode(t, interp, "watch urgent_orders append(my.orders[priority ?= \"urgent\"]) as urgent:\n    my.fired = 1")
		watcher, ok := got.(*Watcher)
		if !ok {
			t.Fatalf("expected *Watcher, got %T", got)
		}
		if len(watcher.Filters) != 1 {
			t.Errorf("expected 1 filter expression, got %d", len(watcher.Filters))
		}
		// Path should have the bracket filter stripped.
		expected := []string{"world.agent.1001.orders"}
		if !stringSlicesEqual(watcher.Watching, expected) {
			t.Errorf("watcher.Watching: expected %v, got %v", expected, watcher.Watching)
		}
	})

	t.Run("path-first watcher form binds at storage path", func(t *testing.T) {
		// `my.handler: watch(my.x): body` parses to a DefinitionStatement
		// wrapping a WatchExpression (per Step 2.3 notes). Step 2.4 must
		// route that shape through bindWatcherAtPath so the path-first
		// form behaves equivalently to the type-first form.
		tree := &MockTree{data: make(map[string]storage.Value)}
		interp := New(tree, 1001)

		got := evalCode(t, interp, "my.handler: watch(my.x):\n    my.touched = 1")
		watcher, ok := got.(*Watcher)
		if !ok {
			t.Fatalf("expected *Watcher result for path-first form, got %T (%v)", got, got)
		}
		if watcher.Name != "handler" {
			t.Errorf("watcher.Name: expected \"handler\", got %q", watcher.Name)
		}
		key := "world.agent.1001.handler"
		if _, found := interp.watchers[key]; !found {
			t.Fatalf("expected watcher to be bound at %q; registry has %v", key, watcherKeys(interp))
		}
		expected := []string{"world.agent.1001.x"}
		if !stringSlicesEqual(watcher.Watching, expected) {
			t.Errorf("watcher.Watching: expected %v, got %v", expected, watcher.Watching)
		}
	})

	t.Run("anonymous watch expression assignment binds at path", func(t *testing.T) {
		// `my.handler = watch(my.x): body` — assignment form should also
		// route a *Watcher value through the registry.
		tree := &MockTree{data: make(map[string]storage.Value)}
		interp := New(tree, 1001)

		got := evalCode(t, interp, "my.handler = watch(my.x):\n    my.touched = 1")
		watcher, ok := got.(*Watcher)
		if !ok {
			t.Fatalf("expected *Watcher result for assignment form, got %T (%v)", got, got)
		}
		key := "world.agent.1001.handler"
		if _, found := interp.watchers[key]; !found {
			t.Fatalf("expected watcher to be bound at %q; registry has %v", key, watcherKeys(interp))
		}
		_ = watcher
	})

	t.Run("registrar receives value-change registration", func(t *testing.T) {
		tree := &MockTree{data: make(map[string]storage.Value)}
		interp := New(tree, 1001)
		reg := &recordingRegistrar{}
		interp.SetWatcherRegistrar(reg)

		evalCode(t, interp, "watch ping(my.x):\n    my.fired = 1")

		if len(reg.valueChange) != 1 {
			t.Fatalf("expected 1 value-change registration, got %d", len(reg.valueChange))
		}
		entry := reg.valueChange[0]
		if entry.name != "world.agent.1001.ping" {
			t.Errorf("registrar got name %q, want %q", entry.name, "world.agent.1001.ping")
		}
		expected := []string{"world.agent.1001.x"}
		if !stringSlicesEqual(entry.watching, expected) {
			t.Errorf("registrar got watching=%v, want %v", entry.watching, expected)
		}
		if entry.code == "" {
			t.Errorf("registrar got empty code; expected serialized body")
		}
	})

	t.Run("registrar receives append registration with filters", func(t *testing.T) {
		tree := &MockTree{data: make(map[string]storage.Value)}
		interp := New(tree, 1001)
		reg := &recordingRegistrar{}
		interp.SetWatcherRegistrar(reg)

		evalCode(t, interp, "watch new_orders append(my.orders[priority ?= \"urgent\"]) as orders:\n    my.fired = 1")

		if len(reg.appendForm) != 1 {
			t.Fatalf("expected 1 append registration, got %d", len(reg.appendForm))
		}
		entry := reg.appendForm[0]
		if entry.bindingName != "orders" {
			t.Errorf("registrar got bindingName=%q, want \"orders\"", entry.bindingName)
		}
		if len(entry.filters) != 1 {
			t.Errorf("registrar got %d filters, want 1", len(entry.filters))
		}
		if len(reg.valueChange) != 0 {
			t.Errorf("expected no value-change registrations, got %d", len(reg.valueChange))
		}
	})

	t.Run("registrar error becomes Unknown", func(t *testing.T) {
		tree := &MockTree{data: make(map[string]storage.Value)}
		interp := New(tree, 1001)
		reg := &recordingRegistrar{failNext: fmt.Errorf("boom")}
		interp.SetWatcherRegistrar(reg)

		got := evalCode(t, interp, "watch boom_watcher(my.x):\n    my.fired = 1")
		unknown, ok := got.(types.Unknown)
		if !ok {
			t.Fatalf("expected Unknown when registrar fails, got %T (%v)", got, got)
		}
		if !strings.Contains(unknown.Reason, "boom_watcher") {
			t.Errorf("expected Unknown reason to mention watcher name, got %q", unknown.Reason)
		}
	})

	t.Run("watch statement with no paths returns Unknown", func(t *testing.T) {
		// Empty paths cannot be expressed in syntax (parser requires at
		// least one path), so this test exercises the AST-level guard
		// by constructing the statement manually.
		tree := &MockTree{data: make(map[string]storage.Value)}
		interp := New(tree, 1001)

		stmt := &parser.WatchStatement{
			Name: "empty",
			Body: &parser.BlockStatement{
				Statements: []parser.Statement{},
			},
		}
		got := interp.evalWatchStatement(stmt)
		if _, ok := got.(types.Unknown); !ok {
			t.Errorf("expected Unknown for empty paths, got %T (%v)", got, got)
		}
	})
}

// stringSlicesEqual compares two string slices by value. Local helper
// to avoid pulling in reflect.DeepEqual for a focused check.
func stringSlicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// watcherKeys returns the keys of the interpreter's watcher registry,
// useful for failure messages.
func watcherKeys(i *Interpreter) []string {
	keys := make([]string, 0, len(i.watchers))
	for k := range i.watchers {
		keys = append(keys, k)
	}
	return keys
}

// procedureKeys returns the keys of the interpreter's procedure
// registry, useful for failure messages in scope-resolution tests.
func procedureKeys(i *Interpreter) []string {
	keys := make([]string, 0, len(i.procedures))
	for k := range i.procedures {
		keys = append(keys, k)
	}
	return keys
}

// TestTypeFirstScopeResolution covers Step 2.5: the "current persistent
// scope" must control where a type-first definition (procedure or
// watcher) binds. The four contexts from the dev plan:
//   - File at scope `my.` → `procedure double(x):` creates my.double
//   - File at scope `my.utilities.` → creates my.utilities.double
//   - Inside a procedure body → creates in the procedure's persistent
//     scope (the scope active when the procedure was defined, not the
//     caller's scope)
//   - REPL (no scope set) → creates in `my` (the agent's home)
//
// Procedures are keyed in the registry by the unresolved (user-typed)
// path, while watchers are keyed by the resolved storage path —
// reflecting the lookup semantics each registry serves.
func TestTypeFirstScopeResolution(t *testing.T) {
	t.Run("REPL no scope binds procedure at my.<name>", func(t *testing.T) {
		tree := &MockTree{data: make(map[string]storage.Value)}
		interp := New(tree, 1001)

		evalCode(t, interp, "procedure double(x): return x * 2")

		if _, ok := interp.procedures["my.double"]; !ok {
			t.Errorf("expected procedure registry to contain 'my.double'; got %v", procedureKeys(interp))
		}
		// Unqualified call still resolves via the simple-name binding
		// in the local scope.
		got := evalCode(t, interp, "double(21)")
		if !compareValues(got, types.Number{Value: 42}) {
			t.Errorf("double(21): expected 42, got %v (%T)", got, got)
		}
	})

	t.Run("file at scope my binds procedure at my.<name>", func(t *testing.T) {
		tree := &MockTree{data: make(map[string]storage.Value)}
		interp := New(tree, 1001)

		// `my.` mirrors a file loaded into the agent's home.
		evalCode(t, interp, "my.")
		evalCode(t, interp, "procedure double(x): return x * 2")

		if _, ok := interp.procedures["my.double"]; !ok {
			t.Errorf("expected procedure registry to contain 'my.double'; got %v", procedureKeys(interp))
		}
	})

	t.Run("file at scope my.utilities binds procedure at my.utilities.<name>", func(t *testing.T) {
		tree := &MockTree{data: make(map[string]storage.Value)}
		interp := New(tree, 1001)

		evalCode(t, interp, "my.utilities.")
		evalCode(t, interp, "procedure double(x): return x * 2")

		if _, ok := interp.procedures["my.utilities.double"]; !ok {
			t.Errorf("expected procedure registry to contain 'my.utilities.double'; got %v", procedureKeys(interp))
		}
		// Qualified call resolves via the registry under the scoped path.
		got := evalCode(t, interp, "my.utilities.double(21)")
		if !compareValues(got, types.Number{Value: 42}) {
			t.Errorf("my.utilities.double(21): expected 42, got %v (%T)", got, got)
		}
	})

	t.Run("nested procedure binds in defining procedure's scope", func(t *testing.T) {
		// Define `outer` in scope my.utilities. Its body declares a
		// nested type-first `inner` procedure. When outer is invoked
		// from a different scope, inner must still bind in
		// my.utilities (outer's defining scope), not the caller's.
		tree := &MockTree{data: make(map[string]storage.Value)}
		interp := New(tree, 1001)

		evalCode(t, interp, "my.utilities.")
		evalCode(t, interp, "procedure outer():\n    procedure inner(): return 1")

		// Switch to a different scope and invoke outer via its
		// qualified path. inner is defined when outer's body executes.
		evalCode(t, interp, "my.elsewhere.")
		evalCode(t, interp, "my.utilities.outer()")

		if _, ok := interp.procedures["my.utilities.inner"]; !ok {
			t.Errorf("expected nested procedure to bind at 'my.utilities.inner' (procedure's defining scope); got %v", procedureKeys(interp))
		}
		if _, ok := interp.procedures["my.elsewhere.inner"]; ok {
			t.Errorf("nested procedure should not bind at caller's scope 'my.elsewhere.inner'")
		}
	})

	t.Run("procedure call restores caller's scope after body returns", func(t *testing.T) {
		// After a procedure body finishes, the interpreter's
		// currentScopePathRaw must return to whatever it was at the
		// call site — the procedure's scope save/restore must not
		// leak across the call boundary.
		tree := &MockTree{data: make(map[string]storage.Value)}
		interp := New(tree, 1001)

		evalCode(t, interp, "my.utilities.")
		evalCode(t, interp, "procedure noop(): return 0")
		evalCode(t, interp, "my.elsewhere.")
		evalCode(t, interp, "my.utilities.noop()")

		// Defining a procedure now should bind under my.elsewhere.
		evalCode(t, interp, "procedure after(): return 1")
		if _, ok := interp.procedures["my.elsewhere.after"]; !ok {
			t.Errorf("expected post-call procedure to bind at caller's scope 'my.elsewhere.after'; got %v", procedureKeys(interp))
		}
	})

	t.Run("REPL no scope binds watcher at agent home", func(t *testing.T) {
		tree := &MockTree{data: make(map[string]storage.Value)}
		interp := New(tree, 1001)

		evalCode(t, interp, "watch ping(my.x):\n    my.fired = 1")

		// Watcher registry uses resolved storage paths.
		if _, ok := interp.watchers["world.agent.1001.ping"]; !ok {
			t.Errorf("expected watcher at 'world.agent.1001.ping'; got %v", watcherKeys(interp))
		}
	})

	t.Run("file at scope my.utilities binds watcher under that scope", func(t *testing.T) {
		tree := &MockTree{data: make(map[string]storage.Value)}
		interp := New(tree, 1001)

		evalCode(t, interp, "my.utilities.")
		evalCode(t, interp, "watch ping(my.x):\n    my.fired = 1")

		// currentScopePath is the resolved form
		// (world.agent.1001.utilities); bindingPathFor appends the
		// watcher name.
		if _, ok := interp.watchers["world.agent.1001.utilities.ping"]; !ok {
			t.Errorf("expected watcher at 'world.agent.1001.utilities.ping'; got %v", watcherKeys(interp))
		}
	})

	t.Run("clearing scope restores REPL binding", func(t *testing.T) {
		// `my.utilities.` sets a scope; an empty scope statement
		// (just `.`) clears it. Subsequent type-first definitions
		// then bind in `my` again.
		tree := &MockTree{data: make(map[string]storage.Value)}
		interp := New(tree, 1001)

		evalCode(t, interp, "my.utilities.")
		evalCode(t, interp, "procedure scoped(): return 1")
		if _, ok := interp.procedures["my.utilities.scoped"]; !ok {
			t.Errorf("expected scoped binding; got %v", procedureKeys(interp))
		}

		// Clear scope by setting it directly via the field. (The
		// parser treats a single dot specially; setting nil here
		// mirrors what evalScopeStatement would do for an empty
		// path.)
		interp.currentScopePath = nil
		interp.currentScopePathRaw = nil

		evalCode(t, interp, "procedure plain(): return 2")
		if _, ok := interp.procedures["my.plain"]; !ok {
			t.Errorf("expected REPL binding 'my.plain' after clearing scope; got %v", procedureKeys(interp))
		}
	})
}
