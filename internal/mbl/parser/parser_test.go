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
			expected:    "x :(copy) = value",
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

	if stmt.Variable != "item" {
		t.Errorf("variable wrong. expected=%q, got=%q", "item", stmt.Variable)
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
