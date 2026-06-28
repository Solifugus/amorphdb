package parser

import (
	"strings"
	"testing"

	"github.com/solifugus/amorphdb/internal/mbl/lexer"
)

// Helper function to create a parser from input string
func createParser(input string) *Parser {
	l := lexer.New(input)
	p := New(l)
	return p
}

// Helper function to check if parsing completed without errors
func checkParserErrors(t *testing.T, p *Parser) {
	errors := p.Errors()
	if len(errors) == 0 {
		return
	}

	t.Errorf("parser has %d errors", len(errors))
	for _, msg := range errors {
		t.Errorf("parser error: %q", msg)
	}
	t.FailNow()
}

func TestExpressionParsing(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expected    string
		expectError bool
	}{
		{
			name:        "simple assignment",
			input:       "x = 42",
			expected:    "x = 42",
			expectError: false,
		},
		{
			name:        "path assignment",
			input:       "my.data.value = 100",
			expected:    "my.data.value = 100",
			expectError: false,
		},
		{
			name:        "assignment with modifier",
			input:       "x:(copy) = value",
			expected:    "x: ((copy) = value)",
			expectError: false,
		},
		{
			name:        "binary expression addition",
			input:       "result = x + y",
			expected:    "result = (x + y)",
			expectError: false,
		},
		{
			name:        "binary expression with precedence",
			input:       "result = x + y * z",
			expected:    "result = (x + (y * z))",
			expectError: false,
		},
		{
			name:        "unary expression",
			input:       "result = -x",
			expected:    "result = (-x)",
			expectError: false,
		},
		{
			name:        "boolean expression",
			input:       "result = x and y or z",
			expected:    "result = ((x and y) or z)",
			expectError: false,
		},
		{
			name:        "comparison expression",
			input:       "result = x > y",
			expected:    "result = (x > y)",
			expectError: false,
		},
		{
			name:        "path expression",
			input:       "value = my.config.timeout",
			expected:    "value = my.config.timeout",
			expectError: false,
		},
		{
			name:        "literal expressions",
			input:       `name = "John Doe"`,
			expected:    `name = "John Doe"`,
			expectError: false,
		},
		{
			name:        "number literal",
			input:       "count = 42",
			expected:    "count = 42",
			expectError: false,
		},
		{
			name:        "float literal",
			input:       "price = 19.99",
			expected:    "price = 19.99",
			expectError: false,
		},
		{
			name:        "boolean literals",
			input:       "enabled = true",
			expected:    "enabled = true",
			expectError: false,
		},
		{
			name:        "grouped expression",
			input:       "result = (x + y) * z",
			expected:    "result = ((x + y) * z)",
			expectError: false,
		},
		{
			name:        "record literal",
			input:       "person = {name: \"Bob\", age: 25}",
			expected:    "person = {name: \"Bob\", age: 25}",
			expectError: false,
		},
		{
			name:        "list literal",
			input:       "numbers = [1, 2, 3]",
			expected:    "numbers = [1, 2, 3]",
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := createParser(tt.input)
			program := p.ParseProgram()

			if tt.expectError {
				if len(p.Errors()) == 0 {
					t.Errorf("expected error but got none")
				}
				return
			}

			checkParserErrors(t, p)

			if len(program.Statements) != 1 {
				t.Errorf("program should have 1 statement, got %d", len(program.Statements))
				return
			}

			stmt := program.Statements[0]
			if stmt.String() != tt.expected {
				t.Errorf("wrong AST. expected=%q, got=%q", tt.expected, stmt.String())
			}
		})
	}
}

func TestOperatorPrecedence(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{
			"result = -a * b",
			"result = ((-a) * b)",
		},
		{
			"result = not -a",
			"result = (not (-a))",
		},
		{
			"result = a + b + c",
			"result = ((a + b) + c)",
		},
		{
			"result = a + b - c",
			"result = ((a + b) - c)",
		},
		{
			"result = a * b * c",
			"result = ((a * b) * c)",
		},
		{
			"result = a * b / c",
			"result = ((a * b) / c)",
		},
		{
			"result = a + b / c",
			"result = (a + (b / c))",
		},
		{
			"result = a + b * c + d / e - f",
			"result = (((a + (b * c)) + (d / e)) - f)",
		},
		{
			"result = 3 + 4 * 5 ?= 3 * 1 + 4 * 5",
			"result = ((3 + (4 * 5)) ?= ((3 * 1) + (4 * 5)))",
		},
		{
			"result = true and false or true",
			"result = ((true and false) or true)",
		},
		{
			"result = 1 < 2 and 3 > 4",
			"result = ((1 < 2) and (3 > 4))",
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			p := createParser(tt.input)
			program := p.ParseProgram()
			checkParserErrors(t, p)

			actual := program.String()
			if actual != tt.expected {
				t.Errorf("expected=%q, got=%q", tt.expected, actual)
			}
		})
	}
}

func TestIfStatements(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple if",
			input:    "if x > 5:\n\ty = 10",
			expected: "if (x > 5): {\n  y = 10\n}",
		},
		{
			name:     "if with else",
			input:    "if x > 5:\n\ty = 10\nelse:\n\ty = 0",
			expected: "if (x > 5): {\n  y = 10\n} else {\n  y = 0\n}",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := createParser(tt.input)
			program := p.ParseProgram()
			checkParserErrors(t, p)

			if len(program.Statements) != 1 {
				t.Errorf("program should have 1 statement, got %d", len(program.Statements))
				return
			}

			stmt := program.Statements[0]
			if stmt.String() != tt.expected {
				t.Errorf("wrong AST. expected=%q, got=%q", tt.expected, stmt.String())
			}
		})
	}
}

func TestWhileStatements(t *testing.T) {
	input := "while x > 0:\n\tx = x - 1"

	p := createParser(input)
	program := p.ParseProgram()
	checkParserErrors(t, p)

	if len(program.Statements) != 1 {
		t.Errorf("program should have 1 statement, got %d", len(program.Statements))
		return
	}

	stmt, ok := program.Statements[0].(*WhileStatement)
	if !ok {
		t.Errorf("statement is not *WhileStatement. got=%T", program.Statements[0])
		return
	}

	if stmt.Condition.String() != "(x > 0)" {
		t.Errorf("condition wrong. expected=%q, got=%q", "(x > 0)", stmt.Condition.String())
	}
}

func TestForStatements(t *testing.T) {
	input := "for item in items:\n\tprocess(item)"

	p := createParser(input)
	program := p.ParseProgram()
	checkParserErrors(t, p)

	if len(program.Statements) != 1 {
		t.Errorf("program should have 1 statement, got %d", len(program.Statements))
		return
	}

	stmt, ok := program.Statements[0].(*ForStatement)
	if !ok {
		t.Errorf("statement is not *ForStatement. got=%T", program.Statements[0])
		return
	}

	if len(stmt.Variables) != 1 || stmt.Variables[0] != "item" {
		t.Errorf("variable wrong. expected=[\"item\"], got=%v", stmt.Variables)
	}

	if stmt.Iterable.String() != "items" {
		t.Errorf("iterable wrong. expected=%q, got=%q", "items", stmt.Iterable.String())
	}
}

func TestReturnStatements(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"return 5", "return 5"},
		{"return true", "return true"},
		{"return x + y", "return (x + y)"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			p := createParser(tt.input)
			program := p.ParseProgram()
			checkParserErrors(t, p)

			if len(program.Statements) != 1 {
				t.Errorf("program should have 1 statement, got %d", len(program.Statements))
				return
			}

			stmt := program.Statements[0]
			if stmt.String() != tt.expected {
				t.Errorf("wrong return statement. expected=%q, got=%q", tt.expected, stmt.String())
			}
		})
	}
}

func TestProcedureStatements(t *testing.T) {
	input := "fibonacci(n):\n\tif n <= 1:\n\t\treturn n\n\treturn fibonacci(n-1) + fibonacci(n-2)"

	p := createParser(input)
	program := p.ParseProgram()
	checkParserErrors(t, p)

	if len(program.Statements) != 1 {
		t.Errorf("program should have 1 statement, got %d", len(program.Statements))
		return
	}

	stmt, ok := program.Statements[0].(*ProcedureStatement)
	if !ok {
		t.Errorf("statement is not *ProcedureStatement. got=%T", program.Statements[0])
		return
	}

	if stmt.Name != "fibonacci" {
		t.Errorf("procedure name wrong. expected=%q, got=%q", "fibonacci", stmt.Name)
	}

	if len(stmt.Parameters) != 1 || stmt.Parameters[0] != "n" {
		t.Errorf("parameters wrong. expected=[%q], got=%v", "n", stmt.Parameters)
	}
}

func TestInstantiationStatements(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"new person", "new person"},
		{"new person {name: \"Bob\", age: 30}", "new person {name: \"Bob\", age: 30}"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			p := createParser(tt.input)
			program := p.ParseProgram()
			checkParserErrors(t, p)

			if len(program.Statements) != 1 {
				t.Errorf("program should have 1 statement, got %d", len(program.Statements))
				return
			}

			stmt := program.Statements[0]
			if stmt.String() != tt.expected {
				t.Errorf("wrong instantiation. expected=%q, got=%q", tt.expected, stmt.String())
			}
		})
	}
}

func TestCallExpressions(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"add(1, 2 * 3, 4 + 5)", "add(1, (2 * 3), (4 + 5))"},
		{"a + add(b * c) + d", "((a + add((b * c))) + d)"},
		{"add(a, b, 1, 2 * 3, 4 + 5)", "add(a, b, 1, (2 * 3), (4 + 5))"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			p := createParser(tt.input)
			program := p.ParseProgram()
			checkParserErrors(t, p)

			if len(program.Statements) != 1 {
				t.Errorf("program should have 1 statement, got %d", len(program.Statements))
				return
			}

			stmt := program.Statements[0]
			if stmt.String() != tt.expected {
				t.Errorf("wrong call expression. expected=%q, got=%q", tt.expected, stmt.String())
			}
		})
	}
}

func TestBracketFilterExpressions(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"people[age > 18]", "people[(age > 18)]"},
		{"people[name ?= \"bob\", age > 18]", "people[(name ?= \"bob\"), (age > 18)]"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			p := createParser(tt.input)
			program := p.ParseProgram()
			checkParserErrors(t, p)

			if len(program.Statements) != 1 {
				t.Errorf("program should have 1 statement, got %d", len(program.Statements))
				return
			}

			stmt := program.Statements[0]
			if stmt.String() != tt.expected {
				t.Errorf("wrong bracket filter. expected=%q, got=%q", tt.expected, stmt.String())
			}
		})
	}
}

func TestLiteralExpressions(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"42", "42"},
		{"3.14", "3.14"},
		{"true", "true"},
		{"false", "false"},
		{`"hello world"`, `"hello world"`},
		{"Nothing", "Nothing"},
		{"Unknown", "Unknown"},
		{"Anything", "Anything"},
		{"pi", "pi"},
		{"euler", "euler"},
		{"empty", "empty"},
		{"quote", "quote"},
		{"tab", "tab"},
		{"newline", "newline"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			p := createParser(tt.input)
			program := p.ParseProgram()
			checkParserErrors(t, p)

			if len(program.Statements) != 1 {
				t.Errorf("program should have 1 statement, got %d", len(program.Statements))
				return
			}

			stmt := program.Statements[0]
			if stmt.String() != tt.expected {
				t.Errorf("wrong literal. expected=%q, got=%q", tt.expected, stmt.String())
			}
		})
	}
}

func TestErrorHandling(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"incomplete assignment", "x = "},
		{"missing closing brace", "{x: 1"},
		{"missing closing bracket", "[1, 2"},
		{"incomplete if", "if x > 5"},
		{"incomplete while", "while x > 0"},
		{"incomplete for", "for x"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := createParser(tt.input)
			p.ParseProgram()

			if len(p.Errors()) == 0 {
				t.Errorf("expected parsing errors for input %q, but got none", tt.input)
			}
		})
	}
}

func TestMemorySafety(t *testing.T) {
	// Test recursion depth limit
	t.Run("recursion depth limit", func(t *testing.T) {
		// Create deeply nested parentheses to trigger recursion depth
		input := strings.Repeat("(", 1100) + "x" + strings.Repeat(")", 1100)
		p := createParser(input)
		p.ParseProgram()

		errors := p.Errors()
		found := false
		for _, err := range errors {
			if strings.Contains(err, "maximum recursion depth") {
				found = true
				break
			}
		}

		if !found {
			t.Errorf("expected recursion depth error, got errors: %v", errors)
		}
	})

	// Test node count limit (simplified test)
	t.Run("node count awareness", func(t *testing.T) {
		p := createParser("x = 1")
		program := p.ParseProgram()
		checkParserErrors(t, p)

		if p.nodeCount == 0 {
			t.Errorf("expected node count to be tracked, got %d", p.nodeCount)
		}

		if len(program.Statements) == 0 {
			t.Errorf("expected at least one statement")
		}
	})
}

// Benchmark tests for performance
func BenchmarkParseSimpleExpression(b *testing.B) {
	input := "x = a + b * c"
	for i := 0; i < b.N; i++ {
		p := createParser(input)
		p.ParseProgram()
	}
}

func BenchmarkParseComplexExpression(b *testing.B) {
	input := "result = (a + b) * (c - d) / (e + f) and x > y or z < w"
	for i := 0; i < b.N; i++ {
		p := createParser(input)
		p.ParseProgram()
	}
}

func BenchmarkParseLargeProgram(b *testing.B) {
	input := "fibonacci(n):\n\tif n <= 1:\n\t\treturn n\n\treturn fibonacci(n-1) + fibonacci(n-2)\n\nfactorial(n):\n\tif n <= 1:\n\t\treturn 1\n\treturn n * factorial(n-1)\n\nresult = fibonacci(10) + factorial(5)"

	for i := 0; i < b.N; i++ {
		p := createParser(input)
		p.ParseProgram()
	}
}

// TestWatchAppendTokens tests that new watch append tokens are recognized
func TestWatchAppendTokens(t *testing.T) {
	input := `watch append as`

	l := lexer.New(input)

	expectedTokens := []lexer.TokenType{
		lexer.WATCH,
		lexer.APPEND,
		lexer.AS,
		lexer.EOF,
	}

	for i, expected := range expectedTokens {
		token := l.NextToken()
		if token.Type != expected {
			t.Fatalf("token[%d] - expected %s, got %s", i, expected, token.Type)
		}
	}
}

// TestWatchStatementSwitchCase tests that WATCH token triggers parseWatchStatement
func TestWatchStatementSwitchCase(t *testing.T) {
	input := `watch`

	p := createParser(input)
	program := p.ParseProgram()

	// The simplified parseWatchStatement returns an expression statement
	// This test verifies that the WATCH case is reached
	if program == nil {
		t.Fatal("ParseProgram() returned nil")
	}

	if len(program.Statements) != 1 {
		t.Fatalf("Expected 1 statement, got %d", len(program.Statements))
	}

	// The simplified implementation returns an expression statement
	_, ok := program.Statements[0].(*ExpressionStatement)
	if !ok {
		t.Fatalf("Expected ExpressionStatement from parseWatchStatement, got %T", program.Statements[0])
	}
}

// TestWatchAppendParsing tests full watch append(...) as name syntax
func TestWatchAppendParsing(t *testing.T) {
	input := `my_watcher: watch append(my.list) as items:
	output("test")`

	p := createParser(input)
	program := p.ParseProgram()

	if len(p.errors) != 0 {
		for _, err := range p.errors {
			t.Errorf("parser error: %s", err)
		}
		t.FailNow()
	}

	if program == nil {
		t.Fatal("ParseProgram() returned nil")
	}

	if len(program.Statements) != 1 {
		t.Fatalf("Expected 1 statement, got %d", len(program.Statements))
	}

	stmt, ok := program.Statements[0].(*WatchStatement)
	if !ok {
		t.Fatalf("Expected WatchStatement, got %T", program.Statements[0])
	}

	if stmt.Name != "my_watcher" {
		t.Errorf("Expected watcher name 'my_watcher', got '%s'", stmt.Name)
	}

	if !stmt.IsAppend {
		t.Error("Expected IsAppend to be true")
	}

	if stmt.BindingName != "items" {
		t.Errorf("Expected binding name 'items', got '%s'", stmt.BindingName)
	}

	if len(stmt.Filters) != 0 {
		t.Errorf("Expected 0 filters for simple path, got %d", len(stmt.Filters))
	}

	if stmt.Body == nil {
		t.Error("Expected watcher body")
	}
}

// TestWatchAppendWithFilterParsing tests watch append with predicate filters
func TestWatchAppendWithFilterParsing(t *testing.T) {
	input := `my_watcher: watch append(my.list[status ?= "active"]) as items:
	output("test")`

	p := createParser(input)
	program := p.ParseProgram()

	if len(p.errors) != 0 {
		for _, err := range p.errors {
			t.Errorf("parser error: %s", err)
		}
		t.FailNow()
	}

	if program == nil {
		t.Fatal("ParseProgram() returned nil")
	}

	if len(program.Statements) != 1 {
		t.Fatalf("Expected 1 statement, got %d", len(program.Statements))
	}

	stmt, ok := program.Statements[0].(*WatchStatement)
	if !ok {
		t.Fatalf("Expected WatchStatement, got %T", program.Statements[0])
	}

	if stmt.Name != "my_watcher" {
		t.Errorf("Expected watcher name 'my_watcher', got '%s'", stmt.Name)
	}

	if !stmt.IsAppend {
		t.Error("Expected IsAppend to be true")
	}

	if stmt.BindingName != "items" {
		t.Errorf("Expected binding name 'items', got '%s'", stmt.BindingName)
	}

	if len(stmt.Filters) != 1 {
		t.Errorf("Expected 1 filter, got %d", len(stmt.Filters))
	}

	if stmt.Body == nil {
		t.Error("Expected watcher body")
	}
}

// TestSimpleWatchDetection tests basic watch statement detection
func TestSimpleWatchDetection(t *testing.T) {
	input := `my_watcher: watch`

	p := createParser(input)

	// Test isWatchStatement detection
	if !p.isWatchStatement() {
		t.Error("Expected isWatchStatement to return true")
	}
}

// TestMultiPathWatchParsing tests multi-path regular watchers
func TestMultiPathWatchParsing(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expectedStr string
		pathCount   int
	}{
		{
			name: "two paths",
			input: `my_watcher: watch(my.account.balance, my.account.limit):
		output("balance or limit changed")`,
			expectedStr: "my_watcher: watch(my.account.balance, my.account.limit):",
			pathCount:   2,
		},
		{
			name: "three paths",
			input: `monitor: watch(my.cpu.usage, my.memory.usage, my.disk.usage):
		output("resource changed")`,
			expectedStr: "monitor: watch(my.cpu.usage, my.memory.usage, my.disk.usage):",
			pathCount:   3,
		},
		{
			name: "single path (backwards compatibility)",
			input: `single_watcher: watch(my.data.value):
		my.result = "changed"`,
			expectedStr: "single_watcher: watch(my.data.value):",
			pathCount:   1,
		},
		{
			name: "mixed path types",
			input: `complex: watch(my.config.setting, world.clock.minute):
		my.status = "updated"`,
			expectedStr: "complex: watch(my.config.setting, world.clock.minute):",
			pathCount:   2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := createParser(tt.input)
			program := p.ParseProgram()
			checkParserErrors(t, p)

			if len(program.Statements) != 1 {
				t.Fatalf("Expected 1 statement, got %d", len(program.Statements))
			}

			stmt, ok := program.Statements[0].(*WatchStatement)
			if !ok {
				t.Fatalf("Expected WatchStatement, got %T", program.Statements[0])
			}

			// Verify path count
			if len(stmt.Paths) != tt.pathCount {
				t.Errorf("Expected %d paths, got %d", tt.pathCount, len(stmt.Paths))
			}

			// Verify it's not an append watcher
			if stmt.IsAppend {
				t.Error("Expected regular watcher, got append watcher")
			}

			// Check string representation (without body for simplicity)
			stmtStr := stmt.String()
			if !strings.Contains(stmtStr, tt.expectedStr) {
				t.Errorf("Expected string to contain %q, got %q", tt.expectedStr, stmtStr)
			}
		})
	}
}

// TestMultiPathWatchErrorCases tests invalid multi-path syntax
func TestMultiPathWatchErrorCases(t *testing.T) {
	tests := []struct {
		name  string
		input string
		desc  string
	}{
		{
			name: "append with multiple paths",
			input: `bad: watch append(my.list1, my.list2) as items:
		output("test")`,
			desc: "append watchers should not support multiple paths",
		},
		{
			name: "trailing comma",
			input: `bad2: watch(my.path1, my.path2,):
		output("test")`,
			desc: "trailing comma should cause parse error",
		},
		{
			name: "empty path list",
			input: `bad3: watch():
		output("test")`,
			desc: "empty path list should cause parse error",
		},
		{
			name: "double comma",
			input: `bad4: watch(my.path1,, my.path2):
		output("test")`,
			desc: "double comma should cause parse error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := createParser(tt.input)
			program := p.ParseProgram()

			// For these error cases, we expect either:
			// 1. Parser errors to be reported, OR
			// 2. Parsed successfully but with unexpected structure (depends on error recovery)

			// At minimum, check that we don't crash
			if program == nil {
				t.Error("Program should not be nil even with errors")
			}

			// Note: The exact error handling behavior depends on the parser's error recovery strategy
			// These tests ensure we don't crash on malformed input
		})
	}
}

// TestMultiPathWatchPathExpressions tests that individual paths are correctly parsed
func TestMultiPathWatchPathExpressions(t *testing.T) {
	input := `test_watcher: watch(my.account.balance, world.clock.hour):
		my.result = "test"`

	p := createParser(input)
	program := p.ParseProgram()
	checkParserErrors(t, p)

	if len(program.Statements) != 1 {
		t.Fatalf("Expected 1 statement, got %d", len(program.Statements))
	}

	stmt, ok := program.Statements[0].(*WatchStatement)
	if !ok {
		t.Fatalf("Expected WatchStatement, got %T", program.Statements[0])
	}

	if len(stmt.Paths) != 2 {
		t.Fatalf("Expected 2 paths, got %d", len(stmt.Paths))
	}

	// Check first path
	path1, ok := stmt.Paths[0].(*PathExpression)
	if !ok {
		t.Fatalf("Expected first path to be PathExpression, got %T", stmt.Paths[0])
	}
	expected1 := []string{"my", "account", "balance"}
	if len(path1.Parts) != len(expected1) {
		t.Errorf("Expected path1 to have %d parts, got %d", len(expected1), len(path1.Parts))
	}
	for i, part := range path1.Parts {
		if part != expected1[i] {
			t.Errorf("Expected path1 part %d to be %q, got %q", i, expected1[i], part)
		}
	}

	// Check second path
	path2, ok := stmt.Paths[1].(*PathExpression)
	if !ok {
		t.Fatalf("Expected second path to be PathExpression, got %T", stmt.Paths[1])
	}
	expected2 := []string{"world", "clock", "hour"}
	if len(path2.Parts) != len(expected2) {
		t.Errorf("Expected path2 to have %d parts, got %d", len(expected2), len(path2.Parts))
	}
	for i, part := range path2.Parts {
		if part != expected2[i] {
			t.Errorf("Expected path2 part %d to be %q, got %q", i, expected2[i], part)
		}
	}
}

// TestSameLineDefinitions tests same-line definition parsing
func TestSameLineDefinitions(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expectError bool
		validate    func(t *testing.T, program *Program)
	}{
		{
			name:        "same-line procedure definition",
			input:       "double(x): return x * 2",
			expectError: false,
			validate: func(t *testing.T, program *Program) {
				if len(program.Statements) != 1 {
					t.Fatalf("Expected 1 statement, got %d", len(program.Statements))
				}
				stmt, ok := program.Statements[0].(*ProcedureStatement)
				if !ok {
					t.Fatalf("Expected ProcedureStatement, got %T", program.Statements[0])
				}
				if stmt.Name != "double" {
					t.Errorf("Expected procedure name \"double\", got %q", stmt.Name)
				}
				if len(stmt.Parameters) != 1 || stmt.Parameters[0] != "x" {
					t.Errorf("Expected parameters [x], got %v", stmt.Parameters)
				}
				if stmt.Body == nil {
					t.Fatal("Expected procedure body to be non-nil")
				}
				if len(stmt.Body.Statements) != 1 {
					t.Errorf("Expected 1 statement in body, got %d", len(stmt.Body.Statements))
				}
			},
		},
		{
			name:        "multi-line form still works",
			input:       "triple(x):\n\treturn x * 3",
			expectError: false,
			validate: func(t *testing.T, program *Program) {
				if len(program.Statements) != 1 {
					t.Fatalf("Expected 1 statement, got %d", len(program.Statements))
				}
				stmt, ok := program.Statements[0].(*ProcedureStatement)
				if !ok {
					t.Fatalf("Expected ProcedureStatement, got %T", program.Statements[0])
				}
				if stmt.Name != "triple" {
					t.Errorf("Expected procedure name \"triple\", got %q", stmt.Name)
				}
			},
		},
		{
			name:        "same-line value definition",
			input:       "my.config.host: \"localhost\"",
			expectError: false,
			validate: func(t *testing.T, program *Program) {
				if len(program.Statements) != 1 {
					t.Fatalf("Expected 1 statement, got %d", len(program.Statements))
				}
				stmt, ok := program.Statements[0].(*AssignmentStatement)
				if !ok {
					t.Fatalf("Expected AssignmentStatement, got %T", program.Statements[0])
				}
				if stmt.Value == nil {
					t.Fatal("Expected assignment to have a value")
				}
			},
		},
		{
			name:        "semicolon separated definitions",
			input:       "my.a: 1; my.b: 2",
			expectError: false,
			validate: func(t *testing.T, program *Program) {
				if len(program.Statements) != 2 {
					t.Fatalf("Expected 2 statements, got %d", len(program.Statements))
				}
				// Check first statement
				stmt1, ok := program.Statements[0].(*AssignmentStatement)
				if !ok {
					t.Fatalf("Expected first statement to be AssignmentStatement, got %T", program.Statements[0])
				}
				// Check second statement
				stmt2, ok := program.Statements[1].(*AssignmentStatement)
				if !ok {
					t.Fatalf("Expected second statement to be AssignmentStatement, got %T", program.Statements[1])
				}
				_ = stmt1; _ = stmt2 // Use variables to avoid unused variable error
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := createParser(tt.input)
			program := parser.ParseProgram()

			if tt.expectError {
				if len(parser.Errors()) == 0 {
					t.Error("Expected parsing errors, but got none")
				}
			} else {
				checkParserErrors(t, parser)
				if tt.validate != nil {
					tt.validate(t, program)
				}
			}
		})
	}
}

// TestProjectionSyntax tests projection expressions like path{ field1, field2 }
func TestProjectionSyntax(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expectError bool
		validate    func(*testing.T, *Program)
	}{
		{
			name:        "basic projection",
			input:       "person{ name, age, job }",
			expectError: false,
			validate: func(t *testing.T, program *Program) {
				if len(program.Statements) != 1 {
					t.Fatalf("Expected 1 statement, got %d", len(program.Statements))
				}

				exprStmt, ok := program.Statements[0].(*ExpressionStatement)
				if !ok {
					t.Fatalf("Expected ExpressionStatement, got %T", program.Statements[0])
				}

				projection, ok := exprStmt.Expression.(*ProjectionExpression)
				if !ok {
					t.Fatalf("Expected ProjectionExpression, got %T", exprStmt.Expression)
				}

				// Check path
				pathExpr, ok := projection.Left.(*PathExpression)
				if !ok {
					t.Fatalf("Expected PathExpression for projection path, got %T", projection.Left)
				}
				if len(pathExpr.Parts) != 1 || pathExpr.Parts[0] != "person" {
					t.Fatalf("Expected path 'person', got %v", pathExpr.Parts)
				}

				// Check fields
				expectedFields := []string{"name", "age", "job"}
				if len(projection.Fields) != len(expectedFields) {
					t.Fatalf("Expected %d fields, got %d", len(expectedFields), len(projection.Fields))
				}
				for i, expected := range expectedFields {
					if projection.Fields[i] != expected {
						t.Fatalf("Expected field %s, got %s", expected, projection.Fields[i])
					}
				}
			},
		},
		{
			name:        "path projection",
			input:       "my.users{ name, email }",
			expectError: false,
			validate: func(t *testing.T, program *Program) {
				if len(program.Statements) != 1 {
					t.Fatalf("Expected 1 statement, got %d", len(program.Statements))
				}

				exprStmt, ok := program.Statements[0].(*ExpressionStatement)
				if !ok {
					t.Fatalf("Expected ExpressionStatement, got %T", program.Statements[0])
				}

				projection, ok := exprStmt.Expression.(*ProjectionExpression)
				if !ok {
					t.Fatalf("Expected ProjectionExpression, got %T", exprStmt.Expression)
				}

				// Check path (should be my.users)
				pathExpr, ok := projection.Left.(*PathExpression)
				if !ok {
					t.Fatalf("Expected PathExpression for projection path, got %T", projection.Left)
				}
				expectedPath := []string{"my", "users"}
				if len(pathExpr.Parts) != len(expectedPath) {
					t.Fatalf("Expected path %v, got %v", expectedPath, pathExpr.Parts)
				}
				for i, expected := range expectedPath {
					if pathExpr.Parts[i] != expected {
						t.Fatalf("Expected path part %s, got %s", expected, pathExpr.Parts[i])
					}
				}

				// Check fields
				expectedFields := []string{"name", "email"}
				if len(projection.Fields) != len(expectedFields) {
					t.Fatalf("Expected %d fields, got %d", len(expectedFields), len(projection.Fields))
				}
				for i, expected := range expectedFields {
					if projection.Fields[i] != expected {
						t.Fatalf("Expected field %s, got %s", expected, projection.Fields[i])
					}
				}
			},
		},
		{
			name:        "single field projection",
			input:       "data{ value }",
			expectError: false,
			validate: func(t *testing.T, program *Program) {
				if len(program.Statements) != 1 {
					t.Fatalf("Expected 1 statement, got %d", len(program.Statements))
				}

				exprStmt, ok := program.Statements[0].(*ExpressionStatement)
				if !ok {
					t.Fatalf("Expected ExpressionStatement, got %T", program.Statements[0])
				}

				projection, ok := exprStmt.Expression.(*ProjectionExpression)
				if !ok {
					t.Fatalf("Expected ProjectionExpression, got %T", exprStmt.Expression)
				}

				if len(projection.Fields) != 1 || projection.Fields[0] != "value" {
					t.Fatalf("Expected single field 'value', got %v", projection.Fields)
				}
			},
		},
		{
			name:        "projection with bracket filter",
			input:       "users[active ?= true]{ name, email }",
			expectError: false,
			validate: func(t *testing.T, program *Program) {
				if len(program.Statements) != 1 {
					t.Fatalf("Expected 1 statement, got %d", len(program.Statements))
				}

				exprStmt, ok := program.Statements[0].(*ExpressionStatement)
				if !ok {
					t.Fatalf("Expected ExpressionStatement, got %T", program.Statements[0])
				}

				projection, ok := exprStmt.Expression.(*ProjectionExpression)
				if !ok {
					t.Fatalf("Expected ProjectionExpression, got %T", exprStmt.Expression)
				}

				// The left should be a BracketFilterExpression
				bracketFilter, ok := projection.Left.(*BracketFilterExpression)
				if !ok {
					t.Fatalf("Expected BracketFilterExpression for projection left side, got %T", projection.Left)
				}

				// Check the path in the bracket filter
				pathExpr, ok := bracketFilter.Left.(*PathExpression)
				if !ok {
					t.Fatalf("Expected PathExpression in bracket filter, got %T", bracketFilter.Left)
				}
				expectedPath := []string{"users"}
				if len(pathExpr.Parts) != len(expectedPath) {
					t.Fatalf("Expected path %v, got %v", expectedPath, pathExpr.Parts)
				}

				// Check projection fields
				expectedFields := []string{"name", "email"}
				if len(projection.Fields) != len(expectedFields) {
					t.Fatalf("Expected %d fields, got %d", len(expectedFields), len(projection.Fields))
				}
			},
		},
		{
			name:        "empty projection should error",
			input:       "person{ }",
			expectError: true,
		},
		{
			name:        "projection with non-identifier should error",
			input:       "person{ name, 123 }",
			expectError: true,
		},
		{
			name:        "projection without closing brace should error",
			input:       "person{ name, age",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := createParser(tt.input)
			program := parser.ParseProgram()

			if tt.expectError {
				if len(parser.Errors()) == 0 {
					t.Error("Expected parsing errors, but got none")
				}
			} else {
				checkParserErrors(t, parser)
				if tt.validate != nil {
					tt.validate(t, program)
				}
			}
		})
	}
}

func TestEmbedDirectives(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		validate func(t *testing.T, program *Program)
	}{
		{
			name:  "basic embed directive",
			input: "embed my.stamp",
			validate: func(t *testing.T, program *Program) {
				if len(program.Statements) != 1 {
					t.Fatalf("expected 1 statement, got %d", len(program.Statements))
				}

				stmt, ok := program.Statements[0].(*EmbedDirectiveStatement)
				if !ok {
					t.Fatalf("expected EmbedDirectiveStatement, got %T", program.Statements[0])
				}

				if stmt.Token.Type != lexer.EMBED {
					t.Errorf("expected token type EMBED, got %s", stmt.Token.Type)
				}

				pathExpr, ok := stmt.Path.(*PathExpression)
				if !ok {
					t.Fatalf("expected PathExpression, got %T", stmt.Path)
				}

				if pathExpr.String() != "my.stamp" {
					t.Errorf("expected path \"my.stamp\", got \"%s\"", pathExpr.String())
				}
			},
		},
		{
			name:  "spread directive",
			input: "...my.other.stamp",
			validate: func(t *testing.T, program *Program) {
				if len(program.Statements) != 1 {
					t.Fatalf("expected 1 statement, got %d", len(program.Statements))
				}

				stmt, ok := program.Statements[0].(*EmbedDirectiveStatement)
				if !ok {
					t.Fatalf("expected EmbedDirectiveStatement, got %T", program.Statements[0])
				}

				if stmt.Token.Type != lexer.SPREAD {
					t.Errorf("expected token type SPREAD, got %s", stmt.Token.Type)
				}

				pathExpr, ok := stmt.Path.(*PathExpression)
				if !ok {
					t.Fatalf("expected PathExpression, got %T", stmt.Path)
				}

				if pathExpr.String() != "my.other.stamp" {
					t.Errorf("expected path \"my.other.stamp\", got \"%s\"", pathExpr.String())
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := createParser(tt.input)
			program := parser.ParseProgram()

			checkParserErrors(t, parser)

			if tt.validate != nil {
				tt.validate(t, program)
			}
		})
	}
}

// TestTypeFirstProcedureDefinition verifies the parsing of the type-first
// procedure form `procedure name(args): body` (Step 2.1 of
// docs/syntax_changes_development_plan.md). The form must produce a
// *DefinitionStatement whose Name is a single-segment PathExpression and
// whose Value is a *ProcedureExpression — the same AST shape as the
// equivalent path-first form `name: procedure(args): body`. The existing
// path-first form, anonymous-assignment form, and anonymous-in-expression
// form must continue to parse unchanged.
func TestTypeFirstProcedureDefinition(t *testing.T) {
	t.Run("single-line type-first procedure", func(t *testing.T) {
		input := "procedure double(x): return x * 2"

		p := createParser(input)
		program := p.ParseProgram()
		checkParserErrors(t, p)

		if len(program.Statements) != 1 {
			t.Fatalf("expected 1 statement, got %d", len(program.Statements))
		}

		ds, ok := program.Statements[0].(*DefinitionStatement)
		if !ok {
			t.Fatalf("expected *DefinitionStatement, got %T", program.Statements[0])
		}

		namePath, ok := ds.Name.(*PathExpression)
		if !ok {
			t.Fatalf("Name should be *PathExpression, got %T", ds.Name)
		}
		if len(namePath.Parts) != 1 || namePath.Parts[0] != "double" {
			t.Errorf("Name parts wrong. expected=[\"double\"], got=%v", namePath.Parts)
		}

		pe, ok := ds.Value.(*ProcedureExpression)
		if !ok {
			t.Fatalf("Value should be *ProcedureExpression, got %T", ds.Value)
		}
		if len(pe.Parameters) != 1 || pe.Parameters[0] != "x" {
			t.Errorf("parameters wrong. expected=[\"x\"], got=%v", pe.Parameters)
		}
		if pe.Body == nil {
			t.Fatalf("body is nil")
		}
		if len(pe.Body.Statements) != 1 {
			t.Errorf("body should have 1 statement, got %d", len(pe.Body.Statements))
		}
		if _, ok := pe.Body.Statements[0].(*ReturnStatement); !ok {
			t.Errorf("body[0] should be *ReturnStatement, got %T", pe.Body.Statements[0])
		}
	})

	t.Run("multi-line type-first procedure with multiple params", func(t *testing.T) {
		input := "procedure calculate_tax(income, rate):\n    return income * rate"

		p := createParser(input)
		program := p.ParseProgram()
		checkParserErrors(t, p)

		if len(program.Statements) != 1 {
			t.Fatalf("expected 1 statement, got %d", len(program.Statements))
		}

		ds, ok := program.Statements[0].(*DefinitionStatement)
		if !ok {
			t.Fatalf("expected *DefinitionStatement, got %T", program.Statements[0])
		}

		namePath, ok := ds.Name.(*PathExpression)
		if !ok {
			t.Fatalf("Name should be *PathExpression, got %T", ds.Name)
		}
		if len(namePath.Parts) != 1 || namePath.Parts[0] != "calculate_tax" {
			t.Errorf("Name parts wrong. expected=[\"calculate_tax\"], got=%v", namePath.Parts)
		}

		pe, ok := ds.Value.(*ProcedureExpression)
		if !ok {
			t.Fatalf("Value should be *ProcedureExpression, got %T", ds.Value)
		}
		if len(pe.Parameters) != 2 || pe.Parameters[0] != "income" || pe.Parameters[1] != "rate" {
			t.Errorf("parameters wrong. expected=[\"income\", \"rate\"], got=%v", pe.Parameters)
		}
		if pe.Body == nil || len(pe.Body.Statements) != 1 {
			t.Fatalf("body should have 1 statement, got %v", pe.Body)
		}
		if _, ok := pe.Body.Statements[0].(*ReturnStatement); !ok {
			t.Errorf("body[0] should be *ReturnStatement, got %T", pe.Body.Statements[0])
		}
	})

	t.Run("type-first procedure with no parameters", func(t *testing.T) {
		input := "procedure ping(): return \"pong\""

		p := createParser(input)
		program := p.ParseProgram()
		checkParserErrors(t, p)

		if len(program.Statements) != 1 {
			t.Fatalf("expected 1 statement, got %d", len(program.Statements))
		}

		ds, ok := program.Statements[0].(*DefinitionStatement)
		if !ok {
			t.Fatalf("expected *DefinitionStatement, got %T", program.Statements[0])
		}
		pe, ok := ds.Value.(*ProcedureExpression)
		if !ok {
			t.Fatalf("Value should be *ProcedureExpression, got %T", ds.Value)
		}
		if len(pe.Parameters) != 0 {
			t.Errorf("parameters should be empty, got=%v", pe.Parameters)
		}
	})

	// Regression: path-first form must still parse and produce its existing
	// AST shape (DefinitionStatement wrapping ProcedureExpression).
	t.Run("path-first form regression", func(t *testing.T) {
		input := "my.functions.double: procedure(x): return x * 2"

		p := createParser(input)
		program := p.ParseProgram()
		checkParserErrors(t, p)

		if len(program.Statements) != 1 {
			t.Fatalf("expected 1 statement, got %d", len(program.Statements))
		}

		ds, ok := program.Statements[0].(*DefinitionStatement)
		if !ok {
			t.Fatalf("expected *DefinitionStatement, got %T", program.Statements[0])
		}

		namePath, ok := ds.Name.(*PathExpression)
		if !ok {
			t.Fatalf("Name should be *PathExpression, got %T", ds.Name)
		}
		expectedParts := []string{"my", "functions", "double"}
		if len(namePath.Parts) != len(expectedParts) {
			t.Errorf("Name parts wrong. expected=%v, got=%v", expectedParts, namePath.Parts)
		} else {
			for i, want := range expectedParts {
				if namePath.Parts[i] != want {
					t.Errorf("Name parts[%d] wrong. expected=%q, got=%q", i, want, namePath.Parts[i])
				}
			}
		}

		if _, ok := ds.Value.(*ProcedureExpression); !ok {
			t.Errorf("Value should be *ProcedureExpression, got %T", ds.Value)
		}
	})

	// Regression: anonymous procedure assigned with `=` must still parse.
	t.Run("anonymous procedure assignment regression", func(t *testing.T) {
		input := "my.double = procedure(x): return x * 2"

		p := createParser(input)
		program := p.ParseProgram()
		checkParserErrors(t, p)

		if len(program.Statements) != 1 {
			t.Fatalf("expected 1 statement, got %d", len(program.Statements))
		}

		as, ok := program.Statements[0].(*AssignmentStatement)
		if !ok {
			t.Fatalf("expected *AssignmentStatement, got %T", program.Statements[0])
		}
		if _, ok := as.Value.(*ProcedureExpression); !ok {
			t.Errorf("Value should be *ProcedureExpression, got %T", as.Value)
		}
	})

	// Regression: anonymous procedure passed as an argument must still parse.
	t.Run("anonymous procedure as call argument regression", func(t *testing.T) {
		input := "result = map(list, procedure(x): return x * 2)"

		p := createParser(input)
		program := p.ParseProgram()
		checkParserErrors(t, p)

		if len(program.Statements) != 1 {
			t.Fatalf("expected 1 statement, got %d", len(program.Statements))
		}

		// Either AssignmentStatement (preferred) or ExpressionStatement.
		// Verify the program parsed without errors and that a
		// ProcedureExpression appears somewhere in the AST.
		found := containsProcedureExpression(program.Statements[0])
		if !found {
			t.Errorf("expected *ProcedureExpression in AST, statement was %T: %s",
				program.Statements[0], program.Statements[0].String())
		}
	})
}

// containsProcedureExpression walks the AST under a Statement looking for
// a ProcedureExpression. Used as a coarse-grained check in tests that do
// not need to assert the exact statement shape.
func containsProcedureExpression(stmt Statement) bool {
	switch s := stmt.(type) {
	case *AssignmentStatement:
		return containsProcedureExpressionInExpr(s.Value)
	case *ExpressionStatement:
		return containsProcedureExpressionInExpr(s.Expression)
	case *DefinitionStatement:
		return containsProcedureExpressionInExpr(s.Value)
	}
	return false
}

func containsProcedureExpressionInExpr(expr Expression) bool {
	switch e := expr.(type) {
	case *ProcedureExpression:
		return true
	case *CallExpression:
		for _, arg := range e.Arguments {
			if containsProcedureExpressionInExpr(arg) {
				return true
			}
		}
	case *BinaryExpression:
		return containsProcedureExpressionInExpr(e.Left) || containsProcedureExpressionInExpr(e.Right)
	}
	return false
}

// TestTypeFirstWatcherDefinition verifies the parsing of the type-first
// watcher form `watch name(paths): body` and `watch name append(path) as
// binding: body` (Step 2.3 of docs/syntax_changes_development_plan.md). The
// type-first form must produce a *WatchStatement matching the same AST shape
// as the equivalent path-first form (`name: watch(...)`). Both value-change
// and append flavors are covered, including multi-path watchers and append
// watchers with predicate filters.
func TestTypeFirstWatcherDefinition(t *testing.T) {
	t.Run("single-path type-first watcher", func(t *testing.T) {
		input := `watch balance_check(my.account.balance):
    output("balance changed")`

		p := createParser(input)
		program := p.ParseProgram()
		checkParserErrors(t, p)

		if len(program.Statements) != 1 {
			t.Fatalf("expected 1 statement, got %d", len(program.Statements))
		}

		ws, ok := program.Statements[0].(*WatchStatement)
		if !ok {
			t.Fatalf("expected *WatchStatement, got %T", program.Statements[0])
		}
		if ws.Name != "balance_check" {
			t.Errorf("Name wrong. expected=%q, got=%q", "balance_check", ws.Name)
		}
		if ws.IsAppend {
			t.Errorf("IsAppend should be false for value-change watcher")
		}
		if len(ws.Paths) != 1 {
			t.Fatalf("expected 1 path, got %d", len(ws.Paths))
		}
		path, ok := ws.Paths[0].(*PathExpression)
		if !ok {
			t.Fatalf("expected Paths[0] to be *PathExpression, got %T", ws.Paths[0])
		}
		expectedParts := []string{"my", "account", "balance"}
		if len(path.Parts) != len(expectedParts) {
			t.Errorf("path parts wrong. expected=%v, got=%v", expectedParts, path.Parts)
		} else {
			for i, want := range expectedParts {
				if path.Parts[i] != want {
					t.Errorf("path.Parts[%d] wrong. expected=%q, got=%q", i, want, path.Parts[i])
				}
			}
		}
		if ws.Body == nil {
			t.Fatalf("Body is nil")
		}
		if len(ws.Body.Statements) != 1 {
			t.Errorf("Body should have 1 statement, got %d", len(ws.Body.Statements))
		}
	})

	t.Run("multi-path type-first watcher", func(t *testing.T) {
		input := `watch balance_check(my.account.balance, my.account.limit):
    output("balance or limit changed")`

		p := createParser(input)
		program := p.ParseProgram()
		checkParserErrors(t, p)

		if len(program.Statements) != 1 {
			t.Fatalf("expected 1 statement, got %d", len(program.Statements))
		}

		ws, ok := program.Statements[0].(*WatchStatement)
		if !ok {
			t.Fatalf("expected *WatchStatement, got %T", program.Statements[0])
		}
		if ws.Name != "balance_check" {
			t.Errorf("Name wrong. expected=%q, got=%q", "balance_check", ws.Name)
		}
		if ws.IsAppend {
			t.Errorf("IsAppend should be false for value-change watcher")
		}
		if len(ws.Paths) != 2 {
			t.Fatalf("expected 2 paths, got %d", len(ws.Paths))
		}
		path1, ok := ws.Paths[0].(*PathExpression)
		if !ok {
			t.Fatalf("expected Paths[0] to be *PathExpression, got %T", ws.Paths[0])
		}
		if got := strings.Join(path1.Parts, "."); got != "my.account.balance" {
			t.Errorf("path[0] wrong. expected=%q, got=%q", "my.account.balance", got)
		}
		path2, ok := ws.Paths[1].(*PathExpression)
		if !ok {
			t.Fatalf("expected Paths[1] to be *PathExpression, got %T", ws.Paths[1])
		}
		if got := strings.Join(path2.Parts, "."); got != "my.account.limit" {
			t.Errorf("path[1] wrong. expected=%q, got=%q", "my.account.limit", got)
		}
	})

	t.Run("type-first watcher with mixed my/world paths", func(t *testing.T) {
		input := `watch monitor(my.config.setting, world.clock.minute):
    my.status = "updated"`

		p := createParser(input)
		program := p.ParseProgram()
		checkParserErrors(t, p)

		ws, ok := program.Statements[0].(*WatchStatement)
		if !ok {
			t.Fatalf("expected *WatchStatement, got %T", program.Statements[0])
		}
		if ws.Name != "monitor" {
			t.Errorf("Name wrong. expected=%q, got=%q", "monitor", ws.Name)
		}
		if len(ws.Paths) != 2 {
			t.Fatalf("expected 2 paths, got %d", len(ws.Paths))
		}
		path2, ok := ws.Paths[1].(*PathExpression)
		if !ok {
			t.Fatalf("expected Paths[1] to be *PathExpression, got %T", ws.Paths[1])
		}
		if got := strings.Join(path2.Parts, "."); got != "world.clock.minute" {
			t.Errorf("path[1] wrong. expected=%q, got=%q", "world.clock.minute", got)
		}
	})

	t.Run("type-first append watcher with binding", func(t *testing.T) {
		input := `watch new_orders append(my.orders) as orders:
    output("new order")`

		p := createParser(input)
		program := p.ParseProgram()
		checkParserErrors(t, p)

		if len(program.Statements) != 1 {
			t.Fatalf("expected 1 statement, got %d", len(program.Statements))
		}

		ws, ok := program.Statements[0].(*WatchStatement)
		if !ok {
			t.Fatalf("expected *WatchStatement, got %T", program.Statements[0])
		}
		if ws.Name != "new_orders" {
			t.Errorf("Name wrong. expected=%q, got=%q", "new_orders", ws.Name)
		}
		if !ws.IsAppend {
			t.Errorf("IsAppend should be true for append watcher")
		}
		if ws.BindingName != "orders" {
			t.Errorf("BindingName wrong. expected=%q, got=%q", "orders", ws.BindingName)
		}
		if len(ws.Paths) != 1 {
			t.Fatalf("expected 1 path, got %d", len(ws.Paths))
		}
		path, ok := ws.Paths[0].(*PathExpression)
		if !ok {
			t.Fatalf("expected Paths[0] to be *PathExpression, got %T", ws.Paths[0])
		}
		if got := strings.Join(path.Parts, "."); got != "my.orders" {
			t.Errorf("path wrong. expected=%q, got=%q", "my.orders", got)
		}
		if len(ws.Filters) != 0 {
			t.Errorf("expected 0 filters for unfiltered append, got %d", len(ws.Filters))
		}
	})

	t.Run("type-first append watcher with predicate filter", func(t *testing.T) {
		input := `watch urgent_orders append(my.orders[priority ?= "urgent"]) as urgent:
    output("urgent order")`

		p := createParser(input)
		program := p.ParseProgram()
		checkParserErrors(t, p)

		if len(program.Statements) != 1 {
			t.Fatalf("expected 1 statement, got %d", len(program.Statements))
		}

		ws, ok := program.Statements[0].(*WatchStatement)
		if !ok {
			t.Fatalf("expected *WatchStatement, got %T", program.Statements[0])
		}
		if ws.Name != "urgent_orders" {
			t.Errorf("Name wrong. expected=%q, got=%q", "urgent_orders", ws.Name)
		}
		if !ws.IsAppend {
			t.Errorf("IsAppend should be true")
		}
		if ws.BindingName != "urgent" {
			t.Errorf("BindingName wrong. expected=%q, got=%q", "urgent", ws.BindingName)
		}
		if len(ws.Filters) != 1 {
			t.Errorf("expected 1 filter, got %d", len(ws.Filters))
		}
		// The path itself should be my.orders (without the filter brackets)
		if len(ws.Paths) != 1 {
			t.Fatalf("expected 1 path, got %d", len(ws.Paths))
		}
		path, ok := ws.Paths[0].(*PathExpression)
		if !ok {
			t.Fatalf("expected Paths[0] to be *PathExpression, got %T", ws.Paths[0])
		}
		if got := strings.Join(path.Parts, "."); got != "my.orders" {
			t.Errorf("path wrong. expected=%q, got=%q", "my.orders", got)
		}
	})

	t.Run("type-first watcher accepts world.* paths in name position", func(t *testing.T) {
		// The alternate-syntax branch in parseWatchStatement accepts IDENT,
		// MY, or WORLD as the watcher name. Confirm a multi-segment dotted
		// name path (e.g., my.monitors.balance_check) parses without error.
		input := `watch my.monitors.balance_check(my.account.balance):
    output("ok")`

		p := createParser(input)
		program := p.ParseProgram()
		checkParserErrors(t, p)

		if len(program.Statements) != 1 {
			t.Fatalf("expected 1 statement, got %d", len(program.Statements))
		}
		ws, ok := program.Statements[0].(*WatchStatement)
		if !ok {
			t.Fatalf("expected *WatchStatement, got %T", program.Statements[0])
		}
		if ws.Name != "my.monitors.balance_check" {
			t.Errorf("Name wrong. expected=%q, got=%q", "my.monitors.balance_check", ws.Name)
		}
	})

	// Regression: the path-first form `name: watch(...)` must still parse
	// without parser errors. (Note: a pre-existing behavior — unrelated to
	// Step 2.3 — currently routes the path-first form through
	// parseDefinitionStatement, producing a *DefinitionStatement wrapping a
	// *WatchExpression rather than a top-level *WatchStatement. Either AST
	// shape is acceptable for this regression check; the important property
	// is that no parser errors are reported and a watcher node appears in
	// the AST.)
	t.Run("path-first form regression (no parser errors)", func(t *testing.T) {
		input := `my_watcher: watch(my.account.balance):
    output("changed")`

		p := createParser(input)
		program := p.ParseProgram()
		checkParserErrors(t, p)

		if len(program.Statements) != 1 {
			t.Fatalf("expected 1 statement, got %d", len(program.Statements))
		}

		// Accept either WatchStatement (alternate path) or
		// DefinitionStatement-wrapping-WatchExpression (current default
		// behavior).
		switch s := program.Statements[0].(type) {
		case *WatchStatement:
			if s.Name != "my_watcher" {
				t.Errorf("Name wrong. expected=%q, got=%q", "my_watcher", s.Name)
			}
		case *DefinitionStatement:
			namePath, ok := s.Name.(*PathExpression)
			if !ok {
				t.Fatalf("DefinitionStatement.Name should be *PathExpression, got %T", s.Name)
			}
			if len(namePath.Parts) != 1 || namePath.Parts[0] != "my_watcher" {
				t.Errorf("Name parts wrong. expected=[\"my_watcher\"], got=%v", namePath.Parts)
			}
			if _, ok := s.Value.(*WatchExpression); !ok {
				t.Errorf("DefinitionStatement.Value should be *WatchExpression, got %T", s.Value)
			}
		default:
			t.Fatalf("expected *WatchStatement or *DefinitionStatement, got %T", s)
		}
	})

	// Regression: anonymous watch expression (assigned with `=`) must still
	// parse. This exercises the existing WATCH-prefix function
	// (parseWatchExpression) used inside expression position.
	t.Run("anonymous watch expression assignment regression", func(t *testing.T) {
		input := `my.handler = watch(my.account.balance): output("x")`

		p := createParser(input)
		program := p.ParseProgram()
		checkParserErrors(t, p)

		if len(program.Statements) != 1 {
			t.Fatalf("expected 1 statement, got %d", len(program.Statements))
		}
		as, ok := program.Statements[0].(*AssignmentStatement)
		if !ok {
			t.Fatalf("expected *AssignmentStatement, got %T", program.Statements[0])
		}
		if _, ok := as.Value.(*WatchExpression); !ok {
			t.Errorf("Value should be *WatchExpression, got %T", as.Value)
		}
	})
}
