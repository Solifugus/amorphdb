// Package lexer implements MBL lexical analysis and tokenization tests
// This file contains comprehensive Section 3 tests from AmorphDB_Test_Plan.md
package lexer

import (
	"testing"
)

// Section 3.1: Token Recognition Tests

func TestSection3_1_1_Identifiers(t *testing.T) {
	// Test case 3.1.1: Identifiers - account, _private, Account, café
	input := `account _private Account café`

	tests := []struct {
		expectedType    TokenType
		expectedLiteral string
	}{
		{IDENT, "account"},
		{IDENT, "_private"},
		{IDENT, "Account"},
		{IDENT, "café"},
		{EOF, ""},
	}

	l := New(input)
	for i, tt := range tests {
		tok := l.NextToken()
		if tok.Type != tt.expectedType {
			t.Fatalf("test 3.1.1[%d] - token type wrong. expected=%v, got=%v, literal=%q",
				i, tt.expectedType, tok.Type, tok.Literal)
		}
		if tok.Literal != tt.expectedLiteral {
			t.Fatalf("test 3.1.1[%d] - literal wrong. expected=%q, got=%q",
				i, tt.expectedLiteral, tok.Literal)
		}
	}
}

func TestSection3_1_2_ReservedWords(t *testing.T) {
	// Test case 3.1.2: All 30+ reserved words tokenize as specific token types, not identifiers
	reservedWords := map[string]TokenType{
		"and":       AND,
		"or":        OR,
		"not":       NOT,
		"if":        IF,
		"else":      ELSE,
		"while":     WHILE,
		"for":       FOR,
		"in":        IN,
		"return":    RETURN,
		"new":       NEW,
		"quietly":   QUIETLY,
		"watch":     WATCH,
		"my":        MY,
		"world":     WORLD,
		"true":      TRUE,
		"false":     FALSE,
		"embed":     EMBED,
		"nothing":   NOTHING,
		"unknown":   UNKNOWN,
		"anything":  ANYTHING,
		"as":        AS,
		"append":    APPEND,
		"consider":  CONSIDER,
		"pass":      PASS,
		"quote":     QUOTE,
		"tab":       TAB,
		"newline":   NEWLINE_LITERAL,
		"empty":     EMPTY,
		"pi":        PI,
		"euler":     EULER,
	}

	for word, expectedType := range reservedWords {
		l := New(word)
		tok := l.NextToken()

		if tok.Type != expectedType {
			t.Fatalf("test 3.1.2 reserved word %q - token type wrong. expected=%v, got=%v",
				word, expectedType, tok.Type)
		}
		if tok.Literal != word {
			t.Fatalf("test 3.1.2 reserved word %q - literal wrong. expected=%q, got=%q",
				word, word, tok.Literal)
		}
	}
}

func TestSection3_1_3_Numbers_Integer(t *testing.T) {
	// Test case 3.1.3: Numbers - integer - 42, 1_000_000 (underscore stripped, value correct)
	tests := []struct {
		input           string
		expectedLiteral string
	}{
		{"42", "42"},
		{"1_000_000", "1000000"}, // Underscores should be stripped
	}

	for _, tt := range tests {
		l := New(tt.input)
		tok := l.NextToken()

		if tok.Type != NUMBER {
			t.Fatalf("test 3.1.3 input %q - token type wrong. expected=NUMBER, got=%v",
				tt.input, tok.Type)
		}
		if tok.Literal != tt.expectedLiteral {
			t.Fatalf("test 3.1.3 input %q - literal wrong. expected=%q, got=%q",
				tt.input, tt.expectedLiteral, tok.Literal)
		}
	}
}

func TestSection3_1_4_Numbers_Float(t *testing.T) {
	// Test case 3.1.4: Numbers - float - 3.14159, 1_000.50
	tests := []struct {
		input           string
		expectedLiteral string
	}{
		{"3.14159", "3.14159"},
		{"1_000.50", "1000.50"}, // Underscores should be stripped
	}

	for _, tt := range tests {
		l := New(tt.input)
		tok := l.NextToken()

		if tok.Type != NUMBER {
			t.Fatalf("test 3.1.4 input %q - token type wrong. expected=NUMBER, got=%v",
				tt.input, tok.Type)
		}
		if tok.Literal != tt.expectedLiteral {
			t.Fatalf("test 3.1.4 input %q - literal wrong. expected=%q, got=%q",
				tt.input, tt.expectedLiteral, tok.Literal)
		}
	}
}

func TestSection3_1_5_Numbers_Invalid(t *testing.T) {
	// Test case 3.1.5: Numbers - invalid - .5 (starts with period), 5. (ends with period)
	tests := []struct {
		input        string
		expectedType TokenType
		description  string
	}{
		{".5", DOT, "period followed by number should be DOT token"},
		{"5.", ILLEGAL, "number ending with period should be ILLEGAL"},
	}

	for _, tt := range tests {
		l := New(tt.input)
		tok := l.NextToken()

		if tok.Type != tt.expectedType {
			t.Fatalf("test 3.1.5 input %q - %s. expected=%v, got=%v with literal %q",
				tt.input, tt.description, tt.expectedType, tok.Type, tok.Literal)
		}
	}
}

func TestSection3_1_6_Numbers_Negative(t *testing.T) {
	// Test case 3.1.6: Numbers - negative - -42, -3.14 (negative sign part of number or unary operator)
	tests := []struct {
		input           string
		expectMinus     bool
		expectedLiteral string
	}{
		{"-42", true, "42"},   // Should be MINUS token followed by NUMBER
		{"-3.14", true, "3.14"}, // Should be MINUS token followed by NUMBER
	}

	for _, tt := range tests {
		l := New(tt.input)

		// First token should be MINUS
		tok1 := l.NextToken()
		if tt.expectMinus && tok1.Type != MINUS {
			t.Fatalf("test 3.1.6 input %q - first token should be MINUS, got %v",
				tt.input, tok1.Type)
		}

		// Second token should be NUMBER
		tok2 := l.NextToken()
		if tok2.Type != NUMBER {
			t.Fatalf("test 3.1.6 input %q - second token should be NUMBER, got %v",
				tt.input, tok2.Type)
		}
		if tok2.Literal != tt.expectedLiteral {
			t.Fatalf("test 3.1.6 input %q - number literal wrong. expected=%q, got=%q",
				tt.input, tt.expectedLiteral, tok2.Literal)
		}
	}
}

func TestSection3_1_7_TimeLiterals(t *testing.T) {
	// Test case 3.1.7: Time literals - @2026-01-15, @2026-01-15 14:30, etc.
	tests := []struct {
		input           string
		expectedLiteral string
	}{
		{"@2026-01-15", "@2026-01-15"},
		{"@2026-01-15 14:30", "@2026-01-15 14:30"},
		{"@2026-01-15 14:30:22", "@2026-01-15 14:30:22"},
		{"@2026-01-15 14:30:22.500", "@2026-01-15 14:30:22.500"},
	}

	for _, tt := range tests {
		l := New(tt.input)
		tok := l.NextToken()

		if tok.Type != TIME {
			t.Fatalf("test 3.1.7 input %q - token type wrong. expected=TIME, got=%v",
				tt.input, tok.Type)
		}
		if tok.Literal != tt.expectedLiteral {
			t.Fatalf("test 3.1.7 input %q - literal wrong. expected=%q, got=%q",
				tt.input, tt.expectedLiteral, tok.Literal)
		}
	}
}

func TestSection3_1_8_Text_Simple(t *testing.T) {
	// Test case 3.1.8: Text - simple - "Hello"
	input := `"Hello"`

	l := New(input)
	tok := l.NextToken()

	if tok.Type != TEXT {
		t.Fatalf("test 3.1.8 - token type wrong. expected=TEXT, got=%v", tok.Type)
	}
	if tok.Literal != `"Hello"` {
		t.Fatalf("test 3.1.8 - literal wrong. expected=%q, got=%q", `"Hello"`, tok.Literal)
	}
}

func TestSection3_1_9_Text_MultiQuote(t *testing.T) {
	// Test case 3.1.9: Text embedding quotes - _"He said "Hello""_
	input := `_"He said "Hello""_`

	l := New(input)
	tok := l.NextToken()

	if tok.Type != TEXT {
		t.Fatalf("test 3.1.9 - token type wrong. expected=TEXT, got=%v", tok.Type)
	}
	if tok.Literal != input {
		t.Fatalf("test 3.1.9 - literal wrong. expected=%q, got=%q", input, tok.Literal)
	}
	value, ok := UnquoteText(tok.Literal)
	if !ok {
		t.Fatalf("test 3.1.9 - UnquoteText did not recognise %q", tok.Literal)
	}
	if want := `He said "Hello"`; value != want {
		t.Fatalf("test 3.1.9 - value wrong. expected=%q, got=%q", want, value)
	}
}

func TestSection3_1_10_Text_TripleQuote(t *testing.T) {
	// Test case 3.1.10: Text with a doubled delimiter run - _""Complex "quoted" text""_
	input := `_""Complex "quoted" text""_`

	l := New(input)
	tok := l.NextToken()

	if tok.Type != TEXT {
		t.Fatalf("test 3.1.10 - token type wrong. expected=TEXT, got=%v", tok.Type)
	}
	if tok.Literal != input {
		t.Fatalf("test 3.1.10 - literal wrong. expected=%q, got=%q", input, tok.Literal)
	}
	value, ok := UnquoteText(tok.Literal)
	if !ok {
		t.Fatalf("test 3.1.10 - UnquoteText did not recognise %q", tok.Literal)
	}
	if want := `Complex "quoted" text`; value != want {
		t.Fatalf("test 3.1.10 - value wrong. expected=%q, got=%q", want, value)
	}
}

func TestSection3_1_11_MoneyLiterals(t *testing.T) {
	// Test case 3.1.11: Money literals - $143.68, €21.80
	tests := []struct {
		input           string
		expectedLiteral string
	}{
		{"$143.68", "$143.68"},
		{"€21.80", "€21.80"},
	}

	for _, tt := range tests {
		l := New(tt.input)
		tok := l.NextToken()

		if tok.Type != MONEY {
			t.Fatalf("test 3.1.11 input %q - token type wrong. expected=MONEY, got=%v",
				tt.input, tok.Type)
		}
		if tok.Literal != tt.expectedLiteral {
			t.Fatalf("test 3.1.11 input %q - literal wrong. expected=%q, got=%q",
				tt.input, tt.expectedLiteral, tok.Literal)
		}
	}
}

func TestSection3_1_12_Operators_Arithmetic(t *testing.T) {
	// Test case 3.1.12: Operators - arithmetic - +, -, *, /, %, ^
	tests := []struct {
		input        string
		expectedType TokenType
	}{
		{"+", PLUS},
		{"-", MINUS},
		{"*", MULTIPLY},
		{"/", DIVIDE},
		{"%", MODULO},
		{"^", POWER}, // ^ is exponentiation operator per spec
	}

	for _, tt := range tests {
		l := New(tt.input)
		tok := l.NextToken()

		if tok.Type != tt.expectedType {
			t.Fatalf("test 3.1.12 input %q - token type wrong. expected=%v, got=%v",
				tt.input, tt.expectedType, tok.Type)
		}
	}
}

func TestSection3_1_13_Operators_Comparison(t *testing.T) {
	// Test case 3.1.13: Operators - comparison - =, !=, <, >, <=, >=
	// Note: In MBL, = is assignment, ?= is comparison
	tests := []struct {
		input        string
		expectedType TokenType
	}{
		{"=", ASSIGN},     // Assignment, not comparison
		{"!=", NOT_EQUAL}, // Not equal comparison operator
		{"<", LT},
		{">", GT},
		{"<=", LTE},
		{">=", GTE},
	}

	for _, tt := range tests {
		l := New(tt.input)
		tok := l.NextToken()

		if tok.Type != tt.expectedType {
			t.Fatalf("test 3.1.13 input %q - token type wrong. expected=%v, got=%v",
				tt.input, tt.expectedType, tok.Type)
		}
	}
}

func TestSection3_1_14_Operators_Logical(t *testing.T) {
	// Test case 3.1.14: Operators - logical - and, or, not
	tests := []struct {
		input        string
		expectedType TokenType
	}{
		{"and", AND},
		{"or", OR},
		{"not", NOT},
	}

	for _, tt := range tests {
		l := New(tt.input)
		tok := l.NextToken()

		if tok.Type != tt.expectedType {
			t.Fatalf("test 3.1.14 input %q - token type wrong. expected=%v, got=%v",
				tt.input, tt.expectedType, tok.Type)
		}
		if tok.Literal != tt.input {
			t.Fatalf("test 3.1.14 input %q - literal wrong. expected=%q, got=%q",
				tt.input, tt.input, tok.Literal)
		}
	}
}

func TestSection3_1_15_Operators_TypeSafe(t *testing.T) {
	// Test case 3.1.15: Definite-equality operator `?=`.
	//
	// Per the updated spec (see docs/syntax_changes_development_plan.md
	// Phase 1 and docs/amorphdb_design.md), the type-safe family was
	// reduced to a single operator: `?=`. The former operators
	// `?!=`, `?<`, `?>`, `?<=`, `?>=` are no longer part of the
	// language — `?` followed by anything other than `=` is a
	// lexical error (ILLEGAL token on the bare `?`). Inequality of
	// potentially-Unknown values is written `not (x ?= y)`; ordering
	// uses standard operators after an explicit Unknown check.
	tests := []struct {
		input        string
		expectedType TokenType
		description  string
	}{
		{"?=", EQUAL, "definite-equality operator"},
		// `?` not followed by `=` is rejected at the lexical level.
		{"?!", ILLEGAL, "bare ? is illegal"},
		{"?<", ILLEGAL, "?< removed from language"},
		{"?>", ILLEGAL, "?> removed from language"},
	}

	for _, tt := range tests {
		l := New(tt.input)
		tok := l.NextToken()

		if tok.Type != tt.expectedType {
			t.Fatalf("test 3.1.15 input %q (%s) - token type wrong. expected=%v, got=%v",
				tt.input, tt.description, tt.expectedType, tok.Type)
		}
	}
}

func TestSection3_1_16_Operators_Assignment(t *testing.T) {
	// Test case 3.1.16: Operators - assignment - =, := (distinguish from comparison =)
	tests := []struct {
		input        string
		expectedType TokenType
	}{
		{"=", ASSIGN},  // Value assignment
		{":", DEFINE},  // Definition (without =)
	}

	for _, tt := range tests {
		l := New(tt.input)
		tok := l.NextToken()

		if tok.Type != tt.expectedType {
			t.Fatalf("test 3.1.16 input %q - token type wrong. expected=%v, got=%v",
				tt.input, tt.expectedType, tok.Type)
		}
	}
}

func TestSection3_1_17_PathPrefixes(t *testing.T) {
	// Test case 3.1.17: Path prefixes - ., +, bare. The `~` home sigil was
	// removed from the language (agent home is reached via `my`), so `~` is no
	// longer a valid token and now lexes as ILLEGAL.
	tests := []struct {
		input        string
		expectedType TokenType
	}{
		{"~", ILLEGAL},
		{".", DOT},
		{"+", PLUS},
	}

	for _, tt := range tests {
		l := New(tt.input)
		tok := l.NextToken()

		if tok.Type != tt.expectedType {
			t.Fatalf("test 3.1.17 input %q - token type wrong. expected=%v, got=%v",
				tt.input, tt.expectedType, tok.Type)
		}
	}
}

func TestSection3_1_18_Brackets(t *testing.T) {
	// Test case 3.1.18: Brackets - [, ], {, }, (, )
	tests := []struct {
		input        string
		expectedType TokenType
	}{
		{"[", LBRACKET},
		{"]", RBRACKET},
		{"{", LBRACE},
		{"}", RBRACE},
		{"(", LPAREN},
		{")", RPAREN},
	}

	for _, tt := range tests {
		l := New(tt.input)
		tok := l.NextToken()

		if tok.Type != tt.expectedType {
			t.Fatalf("test 3.1.18 input %q - token type wrong. expected=%v, got=%v",
				tt.input, tt.expectedType, tok.Type)
		}
	}
}

func TestSection3_1_19_Comments(t *testing.T) {
	// Test case 3.1.19: Comments - # This is a comment (everything after # to EOL discarded)
	input := "# This is a comment"

	l := New(input)
	tok := l.NextToken()

	if tok.Type != COMMENT {
		t.Fatalf("test 3.1.19 - token type wrong. expected=COMMENT, got=%v", tok.Type)
	}
	if tok.Literal != "# This is a comment" {
		t.Fatalf("test 3.1.19 - literal wrong. expected=%q, got=%q",
			"# This is a comment", tok.Literal)
	}
}

func TestSection3_1_20_Comment_vs_Unknown(t *testing.T) {
	// Test case 3.1.20: Comments - all # patterns are comments (no more #reason Unknown syntax)
	tests := []struct {
		input           string
		expectedType    TokenType
		expectedLiteral string
		description     string
	}{
		{"#offline", COMMENT, "#offline", "Comment (# is always comment)"},
		{"# comment", COMMENT, "# comment", "Comment (space after #)"},
	}

	for _, tt := range tests {
		l := New(tt.input)
		tok := l.NextToken()

		if tok.Type != tt.expectedType {
			t.Fatalf("test 3.1.20 %s - token type wrong. expected=%v, got=%v",
				tt.description, tt.expectedType, tok.Type)
		}
		if tok.Literal != tt.expectedLiteral {
			t.Fatalf("test 3.1.20 %s - literal wrong. expected=%q, got=%q",
				tt.description, tt.expectedLiteral, tok.Literal)
		}
	}
}

func TestSection3_1_21_References(t *testing.T) {
	// Test case 3.1.21: References - a quoted path is text, not a reference.
	input := `"world.agent.kalevo"`

	l := New(input)
	tok := l.NextToken()

	// A quoted path tokenizes as TEXT — quoting does not create a reference.
	if tok.Type != TEXT {
		t.Fatalf("test 3.1.21 - token type wrong. expected=TEXT, got=%v", tok.Type)
	}
	if tok.Literal != input {
		t.Fatalf("test 3.1.21 - literal wrong. expected=%q, got=%q", input, tok.Literal)
	}
	value, ok := UnquoteText(tok.Literal)
	if !ok {
		t.Fatalf("test 3.1.21 - UnquoteText did not recognise %q", tok.Literal)
	}
	if want := `world.agent.kalevo`; value != want {
		t.Fatalf("test 3.1.21 - value wrong. expected=%q, got=%q", want, value)
	}
}

func TestSection3_1_22_Dot_and_DoubleDot(t *testing.T) {
	// Test case 3.1.22: Dot and double-dot - . for path, .. for collection operations
	tests := []struct {
		input        string
		expectedType TokenType
	}{
		{".", DOT},
		{"..", RANGE}, // Double-dot for collection operations
	}

	for _, tt := range tests {
		l := New(tt.input)
		tok := l.NextToken()

		if tok.Type != tt.expectedType {
			t.Fatalf("test 3.1.22 input %q - token type wrong. expected=%v, got=%v",
				tt.input, tt.expectedType, tok.Type)
		}
	}
}

func TestSection3_1_23_At_Disambiguation(t *testing.T) {
	// Test case 3.1.23: @ disambiguation - @2026 → time literal; @time → meta attribute; bare @ → instance timestamp reference
	tests := []struct {
		input           string
		expectedType    TokenType
		expectedLiteral string
		description     string
	}{
		{"@2026", TIME, "@2026", "Time literal"},
		{"@time", TIME, "@time", "Meta attribute (parsed as time for now)"},
		{"@", TIME, "@", "Bare @ for instance timestamp"},
	}

	for _, tt := range tests {
		l := New(tt.input)
		tok := l.NextToken()

		if tok.Type != tt.expectedType {
			t.Fatalf("test 3.1.23 %s - token type wrong. expected=%v, got=%v",
				tt.description, tt.expectedType, tok.Type)
		}
		if tok.Literal != tt.expectedLiteral {
			t.Fatalf("test 3.1.23 %s - literal wrong. expected=%q, got=%q",
				tt.description, tt.expectedLiteral, tok.Literal)
		}
	}
}

func TestSection3_1_24_Whitespace_Handling(t *testing.T) {
	// Test case 3.1.24: Whitespace handling - tabs, spaces, newlines, indentation level tracking
	input := "x\n    y"  // Simple test: identifier, newline, indented identifier

	l := New(input)

	// Should get identifier, newline, then indentation, then identifier
	tok1 := l.NextToken() // x
	if tok1.Type != IDENT || tok1.Literal != "x" {
		t.Fatalf("test 3.1.24 - first token should be IDENT 'x', got %v %q", tok1.Type, tok1.Literal)
	}

	tok2 := l.NextToken() // newline
	if tok2.Type != NEWLINE {
		t.Fatalf("test 3.1.24 - second token should be NEWLINE, got %v", tok2.Type)
	}

	tok3 := l.NextToken() // indent (because of the 4 spaces)
	if tok3.Type != INDENT {
		t.Fatalf("test 3.1.24 - third token should be INDENT, got %v", tok3.Type)
	}

	tok4 := l.NextToken() // identifier
	if tok4.Type != IDENT || tok4.Literal != "y" {
		t.Fatalf("test 3.1.24 - fourth token should be IDENT 'y', got %v %q", tok4.Type, tok4.Literal)
	}
}

func TestSection3_1_25_String_Concatenation_Operator(t *testing.T) {
	// Test case 3.1.25: String concatenation operator - & for text join
	input := "&"

	l := New(input)
	tok := l.NextToken()

	if tok.Type != CONCAT {
		t.Fatalf("test 3.1.25 - token type wrong. expected=CONCAT, got=%v", tok.Type)
	}
	if tok.Literal != "&" {
		t.Fatalf("test 3.1.25 - literal wrong. expected=%q, got=%q", "&", tok.Literal)
	}
}

func TestSection3_1_26_Unicode_Identifiers(t *testing.T) {
	// Test case 3.1.26: Identifiers with Unicode letters
	tests := []string{
		"café",
		"变量",  // Chinese
		"متغير", // Arabic
		"переменная", // Russian
	}

	for _, input := range tests {
		l := New(input)
		tok := l.NextToken()

		if tok.Type != IDENT {
			t.Fatalf("test 3.1.26 input %q - token type wrong. expected=IDENT, got=%v",
				input, tok.Type)
		}
		if tok.Literal != input {
			t.Fatalf("test 3.1.26 input %q - literal wrong. expected=%q, got=%q",
				input, input, tok.Literal)
		}
	}
}

func TestSection3_1_27_Empty_Input(t *testing.T) {
	// Test case 3.1.27: Empty input produces only EOF token
	input := ""

	l := New(input)
	tok := l.NextToken()

	if tok.Type != EOF {
		t.Fatalf("test 3.1.27 - token type wrong. expected=EOF, got=%v", tok.Type)
	}
	if tok.Literal != "" {
		t.Fatalf("test 3.1.27 - literal wrong. expected=%q, got=%q", "", tok.Literal)
	}
}

func TestSection3_1_28_Unterminated_String(t *testing.T) {
	// Test case 3.1.28: Unterminated string produces error token with clear message
	input := `"Hello`  // No closing quote

	l := New(input)
	tok := l.NextToken()

	// Should produce an ILLEGAL token or handle the error appropriately
	if tok.Type == TEXT {
		t.Fatalf("test 3.1.28 - should not produce valid TEXT token for unterminated string")
	}

	// Should be ILLEGAL with some error indication
	if tok.Type != ILLEGAL {
		t.Fatalf("test 3.1.28 - token type should be ILLEGAL for unterminated string, got=%v", tok.Type)
	}
}

// Section 3.2: Indentation / Scope Tests

func TestSection3_2_1_Indent_Produces_INDENT_Token(t *testing.T) {
	// Test case 3.2.1: Going from 0 to 4 spaces emits INDENT
	input := "\n    x"  // newline then 4 spaces then identifier

	l := New(input)

	tok1 := l.NextToken() // NEWLINE
	if tok1.Type != NEWLINE {
		t.Fatalf("test 3.2.1 - first token should be NEWLINE, got %v", tok1.Type)
	}

	tok2 := l.NextToken() // INDENT
	if tok2.Type != INDENT {
		t.Fatalf("test 3.2.1 - second token should be INDENT, got %v", tok2.Type)
	}

	tok3 := l.NextToken() // IDENT
	if tok3.Type != IDENT || tok3.Literal != "x" {
		t.Fatalf("test 3.2.1 - third token should be IDENT 'x', got %v %q", tok3.Type, tok3.Literal)
	}
}

func TestSection3_2_2_Dedent_Produces_DEDENT_Token(t *testing.T) {
	// Test case 3.2.2: Going from 4 to 0 spaces emits DEDENT
	input := "    x\ny"  // 4 spaces, identifier, newline, identifier at column 0

	l := New(input)

	// Skip to the interesting part - after the indented line
	for {
		tok := l.NextToken()
		if tok.Type == NEWLINE {
			break
		}
		if tok.Type == EOF {
			t.Fatal("test 3.2.2 - unexpected EOF before newline")
		}
	}

	// Next token should be DEDENT
	tok := l.NextToken()
	if tok.Type != DEDENT {
		t.Fatalf("test 3.2.2 - token should be DEDENT, got %v", tok.Type)
	}
}

func TestSection3_2_3_Multiple_Dedent_Levels(t *testing.T) {
	// Test case 3.2.3: Collapsing two nested levels at once emits 2 DEDENTs.
	//
	// MBL indents blocks with tabs only (one tab = one level); spaces are not
	// used for block structure. A block is reached by progressively nesting one
	// level at a time, so being two levels deep means two INDENTs were emitted.
	// Returning straight to level 0 must therefore emit two matching DEDENTs.
	input := "a\n\tb\n\t\tc\nd" // level 0, 1 tab, 2 tabs, back to level 0

	l := New(input)

	// Walk to the token immediately before the dedents: the deepest identifier 'c'.
	for {
		tok := l.NextToken()
		if tok.Type == IDENT && tok.Literal == "c" {
			break
		}
		if tok.Type == EOF {
			t.Fatal("test 3.2.3 - unexpected EOF before deepest line")
		}
	}

	// Consume the newline ending the deepest line.
	if tok := l.NextToken(); tok.Type != NEWLINE {
		t.Fatalf("test 3.2.3 - expected NEWLINE after deepest line, got %v", tok.Type)
	}

	// Collapsing from level 2 to level 0 must emit exactly two DEDENTs.
	tok1 := l.NextToken()
	if tok1.Type != DEDENT {
		t.Fatalf("test 3.2.3 - first token should be DEDENT, got %v", tok1.Type)
	}

	tok2 := l.NextToken()
	if tok2.Type != DEDENT {
		t.Fatalf("test 3.2.3 - second token should be DEDENT, got %v", tok2.Type)
	}

	// And then the level-0 identifier 'd' (no extra DEDENTs).
	tok3 := l.NextToken()
	if tok3.Type != IDENT || tok3.Literal != "d" {
		t.Fatalf("test 3.2.3 - expected IDENT 'd' after dedents, got %v %q", tok3.Type, tok3.Literal)
	}
}

func TestSection3_2_4_Inconsistent_Indentation(t *testing.T) {
	// Test case 3.2.4: Mixing tabs and spaces produces a clear error
	input := "x\n\t y"  // identifier, newline, then tab and space (mixed)

	l := New(input)

	// Skip identifier and newline
	tok1 := l.NextToken() // x
	if tok1.Type != IDENT {
		t.Fatalf("test 3.2.4 - expected IDENT, got %v", tok1.Type)
	}

	tok2 := l.NextToken() // newline
	if tok2.Type != NEWLINE {
		t.Fatalf("test 3.2.4 - expected NEWLINE, got %v", tok2.Type)
	}

	// Should produce an ILLEGAL token for mixed indentation
	tok3 := l.NextToken()
	if tok3.Type != ILLEGAL {
		t.Fatalf("test 3.2.4 - should produce ILLEGAL token for mixed indentation, got %v", tok3.Type)
	}
}

func TestSection3_2_5_Blank_Lines_Ignored(t *testing.T) {
	// Test case 3.2.5: Blank lines between indented blocks do not emit spurious INDENT/DEDENT
	input := "    x\n\n    y"  // indented, blank line, indented again

	l := New(input)

	// First indented identifier
	tok1 := l.NextToken() // INDENT
	if tok1.Type != INDENT {
		t.Fatalf("test 3.2.5 - first token should be INDENT, got %v", tok1.Type)
	}

	tok2 := l.NextToken() // x
	if tok2.Type != IDENT || tok2.Literal != "x" {
		t.Fatalf("test 3.2.5 - second token should be IDENT 'x', got %v %q", tok2.Type, tok2.Literal)
	}

	tok3 := l.NextToken() // NEWLINE
	if tok3.Type != NEWLINE {
		t.Fatalf("test 3.2.5 - third token should be NEWLINE, got %v", tok3.Type)
	}

	tok4 := l.NextToken() // NEWLINE (blank line)
	if tok4.Type != NEWLINE {
		t.Fatalf("test 3.2.5 - fourth token should be NEWLINE, got %v", tok4.Type)
	}

	// The next identifier should NOT have another INDENT token
	tok5 := l.NextToken() // y (should NOT be preceded by INDENT)
	if tok5.Type != IDENT || tok5.Literal != "y" {
		t.Fatalf("test 3.2.5 - fifth token should be IDENT 'y', got %v %q", tok5.Type, tok5.Literal)
	}
}