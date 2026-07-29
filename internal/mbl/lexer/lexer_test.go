package lexer

import (
	"testing"
)

func TestBasicTokenization(t *testing.T) {
	input := `my.contacts.joe.name = "Hello"`

	tests := []struct {
		expectedType    TokenType
		expectedLiteral string
	}{
		{MY, "my"},
		{DOT, "."},
		{IDENT, "contacts"},
		{DOT, "."},
		{IDENT, "joe"},
		{DOT, "."},
		{IDENT, "name"},
		{ASSIGN, "="},
		{TEXT, `"Hello"`},
		{EOF, ""},
	}

	l := New(input)

	for i, tt := range tests {
		tok := l.NextToken()

		if tok.Type != tt.expectedType {
			t.Fatalf("tests[%d] - tokentype wrong. expected=%q, got=%q",
				i, tt.expectedType, tok.Type)
		}

		if tok.Literal != tt.expectedLiteral {
			t.Fatalf("tests[%d] - literal wrong. expected=%q, got=%q",
				i, tt.expectedLiteral, tok.Literal)
		}
	}
}

func TestIndentationTracking(t *testing.T) {
	input := `if true:
	x = 1
	if y:
		z = 2
	a = 3
b = 4`

	tests := []struct {
		expectedType    TokenType
		expectedLiteral string
	}{
		{IF, "if"},
		{TRUE, "true"},
		{DEFINE, ":"},
		{NEWLINE, "\n"},
		{INDENT, ""},
		{IDENT, "x"},
		{ASSIGN, "="},
		{NUMBER, "1"},
		{NEWLINE, "\n"},
		{IF, "if"},
		{IDENT, "y"},
		{DEFINE, ":"},
		{NEWLINE, "\n"},
		{INDENT, ""},
		{IDENT, "z"},
		{ASSIGN, "="},
		{NUMBER, "2"},
		{NEWLINE, "\n"},
		{DEDENT, ""},
		{IDENT, "a"},
		{ASSIGN, "="},
		{NUMBER, "3"},
		{NEWLINE, "\n"},
		{DEDENT, ""},
		{IDENT, "b"},
		{ASSIGN, "="},
		{NUMBER, "4"},
		{EOF, ""},
	}

	l := New(input)

	for i, tt := range tests {
		tok := l.NextToken()

		if tok.Type != tt.expectedType {
			t.Fatalf("tests[%d] - tokentype wrong. expected=%q, got=%q",
				i, tt.expectedType, tok.Type)
		}

		if tok.Literal != tt.expectedLiteral {
			t.Fatalf("tests[%d] - literal wrong. expected=%q, got=%q",
				i, tt.expectedLiteral, tok.Literal)
		}
	}
}

func TestTimeLiterals(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"@2026-02-01", "@2026-02-01"},
		{"@2026-02-01 15:30:00", "@2026-02-01 15:30:00"},
		{"@2026-12-31 23:59:59.999", "@2026-12-31 23:59:59.999"},
	}

	for _, tt := range tests {
		l := New(tt.input)
		tok := l.NextToken()

		if tok.Type != TIME {
			t.Fatalf("expected TIME token, got %q", tok.Type)
		}

		if tok.Literal != tt.expected {
			t.Fatalf("expected %q, got %q", tt.expected, tok.Literal)
		}
	}
}

func TestMoneyLiterals(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"¤19.95 USD", "¤19.95 USD"},
		{"¤100 EUR", "¤100 EUR"},
		{"¤0.50 GBP", "¤0.50 GBP"},
	}

	for _, tt := range tests {
		l := New(tt.input)
		tok := l.NextToken()

		if tok.Type != MONEY {
			t.Fatalf("expected MONEY token, got %q", tok.Type)
		}

		if tok.Literal != tt.expected {
			t.Fatalf("expected %q, got %q", tt.expected, tok.Literal)
		}
	}
}

func TestRangeOperator(t *testing.T) {
	input := "x..trim..upper"

	tests := []struct {
		expectedType    TokenType
		expectedLiteral string
	}{
		{IDENT, "x"},
		{RANGE, ".."},
		{IDENT, "trim"},
		{RANGE, ".."},
		{IDENT, "upper"},
		{EOF, ""},
	}

	l := New(input)

	for i, tt := range tests {
		tok := l.NextToken()

		if tok.Type != tt.expectedType {
			t.Fatalf("tests[%d] - tokentype wrong. expected=%q, got=%q",
				i, tt.expectedType, tok.Type)
		}

		if tok.Literal != tt.expectedLiteral {
			t.Fatalf("tests[%d] - literal wrong. expected=%q, got=%q",
				i, tt.expectedLiteral, tok.Literal)
		}
	}
}

func TestEqualityOperator(t *testing.T) {
	input := "x ?= y"

	tests := []struct {
		expectedType    TokenType
		expectedLiteral string
	}{
		{IDENT, "x"},
		{EQUAL, "?="},
		{IDENT, "y"},
		{EOF, ""},
	}

	l := New(input)

	for i, tt := range tests {
		tok := l.NextToken()

		if tok.Type != tt.expectedType {
			t.Fatalf("tests[%d] - tokentype wrong. expected=%q, got=%q",
				i, tt.expectedType, tok.Type)
		}

		if tok.Literal != tt.expectedLiteral {
			t.Fatalf("tests[%d] - literal wrong. expected=%q, got=%q",
				i, tt.expectedLiteral, tok.Literal)
		}
	}
}

func TestQuotedStrings(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{`"hello"`, `"hello"`},
		// Text containing quotes uses the extended form _"…"_ ; the bare
		// multi-quote form ("" … "") is no longer a single literal.
		{`_"she said "hi""_`, `_"she said "hi""_`},
		{`_""triple quoted""_`, `_""triple quoted""_`},
		{`"multi
line
string"`, `"multi
line
string"`},
	}

	for _, tt := range tests {
		l := New(tt.input)
		tok := l.NextToken()

		if tok.Type != TEXT {
			t.Fatalf("expected TEXT token, got %q", tok.Type)
		}

		if tok.Literal != tt.expected {
			t.Fatalf("expected %q, got %q", tt.expected, tok.Literal)
		}
	}
}

func TestComments(t *testing.T) {
	input := `x = 1 # this is a comment
y = 2
## block comment
multiple lines
##
z = 3`

	tests := []struct {
		expectedType    TokenType
		expectedLiteral string
	}{
		{IDENT, "x"},
		{ASSIGN, "="},
		{NUMBER, "1"},
		{COMMENT, "# this is a comment"},
		{NEWLINE, "\n"},
		{IDENT, "y"},
		{ASSIGN, "="},
		{NUMBER, "2"},
		{NEWLINE, "\n"},
		{BLOCK_COMMENT, "## block comment\nmultiple lines\n##"},
		{NEWLINE, "\n"},
		{IDENT, "z"},
		{ASSIGN, "="},
		{NUMBER, "3"},
		{EOF, ""},
	}

	l := New(input)

	for i, tt := range tests {
		tok := l.NextToken()

		if tok.Type != tt.expectedType {
			t.Fatalf("tests[%d] - tokentype wrong. expected=%q, got=%q",
				i, tt.expectedType, tok.Type)
		}

		if tok.Literal != tt.expectedLiteral {
			t.Fatalf("tests[%d] - literal wrong. expected=%q, got=%q",
				i, tt.expectedLiteral, tok.Literal)
		}
	}
}

func TestEnhancedBlockComments(t *testing.T) {
	// Test all spec examples for comments
	testCases := []struct {
		name     string
		input    string
		expected []struct {
			tokenType TokenType
			literal   string
		}
	}{
		{
			name:  "single line comment",
			input: `# single line comment`,
			expected: []struct {
				tokenType TokenType
				literal   string
			}{
				{COMMENT, "# single line comment"},
				{EOF, ""},
			},
		},
		{
			name:  "code with inline comment and more code",
			input: `code # inline comment # more code`,
			expected: []struct {
				tokenType TokenType
				literal   string
			}{
				{IDENT, "code"},
				{COMMENT, "# inline comment #"},
				{IDENT, "more"},
				{IDENT, "code"},
				{EOF, ""},
			},
		},
		{
			name:  "double hash block comment",
			input: `## block comment spanning lines ##`,
			expected: []struct {
				tokenType TokenType
				literal   string
			}{
				{BLOCK_COMMENT, "## block comment spanning lines ##"},
				{EOF, ""},
			},
		},
		{
			name:  "block comment with single hash inside",
			input: `## block containing # single hash ##`,
			expected: []struct {
				tokenType TokenType
				literal   string
			}{
				{BLOCK_COMMENT, "## block containing # single hash ##"},
				{EOF, ""},
			},
		},
		{
			name:  "triple hash block comment with double hash inside",
			input: `### block containing ## double hash ###`,
			expected: []struct {
				tokenType TokenType
				literal   string
			}{
				{BLOCK_COMMENT, "### block containing ## double hash ###"},
				{EOF, ""},
			},
		},
		{
			name: "multiline block comment",
			input: `## block comment
spanning multiple
lines ##`,
			expected: []struct {
				tokenType TokenType
				literal   string
			}{
				{BLOCK_COMMENT, "## block comment\nspanning multiple\nlines ##"},
				{EOF, ""},
			},
		},
		{
			name:  "quadruple hash block comment",
			input: `#### four hash block ####`,
			expected: []struct {
				tokenType TokenType
				literal   string
			}{
				{BLOCK_COMMENT, "#### four hash block ####"},
				{EOF, ""},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			l := New(tc.input)

			for i, expected := range tc.expected {
				tok := l.NextToken()

				if tok.Type != expected.tokenType {
					t.Errorf("test %q - token[%d] type wrong. expected=%q, got=%q",
						tc.name, i, expected.tokenType, tok.Type)
				}

				if tok.Literal != expected.literal {
					t.Errorf("test %q - token[%d] literal wrong. expected=%q, got=%q",
						tc.name, i, expected.literal, tok.Literal)
				}
			}
		})
	}
}

func TestSpaceIndentationError(t *testing.T) {
	input := `if true:
    x = 1`  // Using spaces instead of tabs

	l := New(input)

	// Skip to the line with space indentation
	l.NextToken() // IF
	l.NextToken() // TRUE
	l.NextToken() // DEFINE
	l.NextToken() // NEWLINE

	tok := l.NextToken()

	// Spaces are now accepted for indentation (4 spaces = 1 level)
	if tok.Type != INDENT {
		t.Fatalf("expected INDENT token for space indentation, got %q", tok.Type)
	}
}

func TestKeywords(t *testing.T) {
	tests := []struct {
		input    string
		expected TokenType
	}{
		{"if", IF},
		{"else", ELSE},
		{"while", WHILE},
		{"for", FOR},
		{"in", IN},
		{"consider", CONSIDER},
		{"watch", WATCH},
		{"append", APPEND},
		{"as", AS},
		{"return", RETURN},
		{"pass", PASS},
		{"new", NEW},
		{"and", AND},
		{"or", OR},
		{"not", NOT},
		{"true", TRUE},
		{"false", FALSE},
		{"my", MY},
		{"world", WORLD},
		{"nothing", NOTHING},
		{"unknown", UNKNOWN},
		{"anything", ANYTHING},
	}

	for _, tt := range tests {
		l := New(tt.input)
		tok := l.NextToken()

		if tok.Type != tt.expected {
			t.Fatalf("expected %q, got %q for input %q", tt.expected, tok.Type, tt.input)
		}
	}
}

func TestModifiers(t *testing.T) {
	input := "(copy) (link) (quietly) (unknown_modifier)"

	tests := []struct {
		expectedType    TokenType
		expectedLiteral string
	}{
		{COPY, "(copy)"},
		{LINK, "(link)"},
		{QUIETLY, "(quietly)"},
		{IDENT, "(unknown_modifier)"}, // Unknown modifiers become IDENT
		{EOF, ""},
	}

	l := New(input)

	for i, tt := range tests {
		tok := l.NextToken()

		if tok.Type != tt.expectedType {
			t.Fatalf("tests[%d] - tokentype wrong. expected=%q, got=%q",
				i, tt.expectedType, tok.Type)
		}

		if tok.Literal != tt.expectedLiteral {
			t.Fatalf("tests[%d] - literal wrong. expected=%q, got=%q",
				i, tt.expectedLiteral, tok.Literal)
		}
	}
}

func TestNumbers(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"123", "123"},
		{"123.456", "123.456"},
		{"1.23e-4", "1.23e-4"},
		{"1.23E+4", "1.23E+4"},
		{"0.5", "0.5"},
	}

	for _, tt := range tests {
		l := New(tt.input)
		tok := l.NextToken()

		if tok.Type != NUMBER {
			t.Fatalf("expected NUMBER token, got %q for input %q", tok.Type, tt.input)
		}

		if tok.Literal != tt.expected {
			t.Fatalf("expected %q, got %q", tt.expected, tok.Literal)
		}
	}
}

func TestOperators(t *testing.T) {
	input := "+ - * / % & ?= > < >= <= = .. :"

	tests := []struct {
		expectedType    TokenType
		expectedLiteral string
	}{
		{PLUS, "+"},
		{MINUS, "-"},
		{MULTIPLY, "*"},
		{DIVIDE, "/"},
		{MODULO, "%"},
		{CONCAT, "&"},
		{EQUAL, "?="},
		{GT, ">"},
		{LT, "<"},
		{GTE, ">="},
		{LTE, "<="},
		{ASSIGN, "="},
		{RANGE, ".."},
		{DEFINE, ":"},
		{EOF, ""},
	}

	l := New(input)

	for i, tt := range tests {
		tok := l.NextToken()

		if tok.Type != tt.expectedType {
			t.Fatalf("tests[%d] - tokentype wrong. expected=%q, got=%q",
				i, tt.expectedType, tok.Type)
		}

		if tok.Literal != tt.expectedLiteral {
			t.Fatalf("tests[%d] - literal wrong. expected=%q, got=%q",
				i, tt.expectedLiteral, tok.Literal)
		}
	}
}

func TestDelimiters(t *testing.T) {
	input := "( ) [ ] { } , ."

	tests := []struct {
		expectedType    TokenType
		expectedLiteral string
	}{
		{LPAREN, "("},
		{RPAREN, ")"},
		{LBRACKET, "["},
		{RBRACKET, "]"},
		{LBRACE, "{"},
		{RBRACE, "}"},
		{COMMA, ","},
		{DOT, "."},
		{EOF, ""},
	}

	l := New(input)

	for i, tt := range tests {
		tok := l.NextToken()

		if tok.Type != tt.expectedType {
			t.Fatalf("tests[%d] - tokentype wrong. expected=%q, got=%q",
				i, tt.expectedType, tok.Type)
		}

		if tok.Literal != tt.expectedLiteral {
			t.Fatalf("tests[%d] - literal wrong. expected=%q, got=%q",
				i, tt.expectedLiteral, tok.Literal)
		}
	}
}

func TestComplexIndentationExample(t *testing.T) {
	input := `def function():
	if condition:
		do_something()
		if nested:
			deeper()
	else:
		other_thing()
	final_statement()`

	// This tests multiple levels of indentation and dedentation
	l := New(input)
	var tokens []Token

	for {
		tok := l.NextToken()
		tokens = append(tokens, tok)
		if tok.Type == EOF {
			break
		}
	}

	// Verify we have the right number of INDENT and DEDENT tokens
	indentCount := 0
	dedentCount := 0
	for _, tok := range tokens {
		if tok.Type == INDENT {
			indentCount++
		} else if tok.Type == DEDENT {
			dedentCount++
		}
	}

	if indentCount != 4 {
		t.Fatalf("expected 4 INDENT tokens, got %d", indentCount)
	}
	if dedentCount != 4 {
		t.Fatalf("expected 4 DEDENT tokens, got %d", dedentCount)
	}
}

func TestEmbedAndSpreadTokens(t *testing.T) {
	input := `embed my.stamp
...my.other.stamp
.. range syntax
... spread syntax`

	tests := []struct {
		expectedType    TokenType
		expectedLiteral string
	}{
		{EMBED, "embed"},
		{MY, "my"},
		{DOT, "."},
		{IDENT, "stamp"},
		{NEWLINE, "\n"},
		{SPREAD, "..."},
		{MY, "my"},
		{DOT, "."},
		{IDENT, "other"},
		{DOT, "."},
		{IDENT, "stamp"},
		{NEWLINE, "\n"},
		{RANGE, ".."},
		{IDENT, "range"},
		{IDENT, "syntax"},
		{NEWLINE, "\n"},
		{SPREAD, "..."},
		{IDENT, "spread"},
		{IDENT, "syntax"},
		{EOF, ""},
	}

	l := New(input)

	for i, tt := range tests {
		tok := l.NextToken()

		if tok.Type != tt.expectedType {
			t.Fatalf("tests[%d] - tokentype wrong. expected=%q, got=%q",
				i, tt.expectedType, tok.Type)
		}

		if tok.Literal != tt.expectedLiteral {
			t.Fatalf("tests[%d] - literal wrong. expected=%q, got=%q",
				i, tt.expectedLiteral, tok.Literal)
		}
	}
}