package ari

import (
	"testing"
)

func TestLexer_BasicTokens(t *testing.T) {
	input := `section field break : , - ( ) [ ] 5 2-10 3- -8`

	tests := []struct {
		expectedType    TokenType
		expectedLiteral string
	}{
		{SECTION, "section"},
		{FIELD, "field"},
		{BREAK, "break"},
		{COLON, ":"},
		{COMMA, ","},
		{DASH, "-"},
		{LPAREN, "("},
		{RPAREN, ")"},
		{LBRACKET, "["},
		{RBRACKET, "]"},
		{NUMBER, "5"},
		{RANGE, "2-10"},
		{OPEN_MIN, "3-"},
		{OPEN_MAX, "-8"},
		{EOF, ""},
	}

	lexer := NewLexer(input)

	for i, tt := range tests {
		tok := lexer.NextToken()

		if tok.Type != tt.expectedType {
			t.Fatalf("test[%d] - tokentype wrong. expected=%q, got=%q", i, tt.expectedType, tok.Type)
		}

		if tok.Literal != tt.expectedLiteral {
			t.Fatalf("test[%d] - literal wrong. expected=%q, got=%q", i, tt.expectedLiteral, tok.Literal)
		}
	}
}

func TestLexer_Keywords(t *testing.T) {
	input := `left right up down same flush starts ends on type output date money integer decimal ssn ISO_DATE`

	tests := []struct {
		expectedType    TokenType
		expectedLiteral string
	}{
		{LEFT, "left"},
		{RIGHT, "right"},
		{UP, "up"},
		{DOWN, "down"},
		{SAME, "same"},
		{FLUSH, "flush"},
		{STARTS, "starts"},
		{ENDS, "ends"},
		{ON, "on"},
		{TYPE, "type"},
		{OUTPUT, "output"},
		{DATE, "date"},
		{MONEY, "money"},
		{INTEGER, "integer"},
		{DECIMAL, "decimal"},
		{SSN, "ssn"},
		{ISO_DATE, "ISO_DATE"},
		{EOF, ""},
	}

	lexer := NewLexer(input)

	for i, tt := range tests {
		tok := lexer.NextToken()

		if tok.Type != tt.expectedType {
			t.Fatalf("test[%d] - tokentype wrong. expected=%q, got=%q", i, tt.expectedType, tok.Type)
		}

		if tok.Literal != tt.expectedLiteral {
			t.Fatalf("test[%d] - literal wrong. expected=%q, got=%q", i, tt.expectedLiteral, tok.Literal)
		}
	}
}

func TestLexer_StringLiterals(t *testing.T) {
	input := `"hello world" "ID" "TOTAL"`

	tests := []struct {
		expectedType    TokenType
		expectedLiteral string
	}{
		{STRING, "hello world"},
		{STRING, "ID"},
		{STRING, "TOTAL"},
		{EOF, ""},
	}

	lexer := NewLexer(input)

	for i, tt := range tests {
		tok := lexer.NextToken()

		if tok.Type != tt.expectedType {
			t.Fatalf("test[%d] - tokentype wrong. expected=%q, got=%q", i, tt.expectedType, tok.Type)
		}

		if tok.Literal != tt.expectedLiteral {
			t.Fatalf("test[%d] - literal wrong. expected=%q, got=%q", i, tt.expectedLiteral, tok.Literal)
		}
	}
}

func TestLexer_RegexPatterns(t *testing.T) {
	input := `/\d{4}-\d{2}-\d{2}/ /(\d{2})\/(\d{2})\/(\d{4})/$3-$1-$2/`

	tests := []struct {
		expectedType    TokenType
		expectedLiteral string
	}{
		{REGEX, `/\d{4}-\d{2}-\d{2}/`},
		{REGEX_WITH_TRANSFORM, `/(\d{2})\/(\d{2})\/(\d{4})/$3-$1-$2/`},
		{EOF, ""},
	}

	lexer := NewLexer(input)

	for i, tt := range tests {
		tok := lexer.NextToken()

		if tok.Type != tt.expectedType {
			t.Fatalf("test[%d] - tokentype wrong. expected=%q, got=%q", i, tt.expectedType, tok.Type)
		}

		if tok.Literal != tt.expectedLiteral {
			t.Fatalf("test[%d] - literal wrong. expected=%q, got=%q", i, tt.expectedLiteral, tok.Literal)
		}
	}
}

func TestLexer_Arrow(t *testing.T) {
	input := `->`

	lexer := NewLexer(input)
	tok := lexer.NextToken()

	if tok.Type != ARROW {
		t.Fatalf("tokentype wrong. expected=%q, got=%q", ARROW, tok.Type)
	}

	if tok.Literal != "->" {
		t.Fatalf("literal wrong. expected=%q, got=%q", "->", tok.Literal)
	}
}

func TestLexer_Identifiers(t *testing.T) {
	input := `bank_report report_date transaction_type`

	tests := []struct {
		expectedType    TokenType
		expectedLiteral string
	}{
		{IDENTIFIER, "bank_report"},
		{IDENTIFIER, "report_date"},
		{IDENTIFIER, "transaction_type"},
		{EOF, ""},
	}

	lexer := NewLexer(input)

	for i, tt := range tests {
		tok := lexer.NextToken()

		if tok.Type != tt.expectedType {
			t.Fatalf("test[%d] - tokentype wrong. expected=%q, got=%q", i, tt.expectedType, tok.Type)
		}

		if tok.Literal != tt.expectedLiteral {
			t.Fatalf("test[%d] - literal wrong. expected=%q, got=%q", i, tt.expectedLiteral, tok.Literal)
		}
	}
}

func TestLexer_LineTracking(t *testing.T) {
	input := `section bank_report:
    field report_date: right 10 up 1: date`

	lexer := NewLexer(input)

	// First token should be on line 1
	tok := lexer.NextToken()
	if tok.Line != 1 {
		t.Fatalf("first token line wrong. expected=1, got=%d", tok.Line)
	}

	// Skip to newline
	for tok.Type != NEWLINE && tok.Type != EOF {
		tok = lexer.NextToken()
	}

	// Next content token should be on line 2
	tok = lexer.NextToken() // skip newline
	if tok.Type != EOF {
		// The next significant token (field) should be on line 2
		for tok.Type == WHITESPACE {
			tok = lexer.NextToken()
		}
		if tok.Line != 2 {
			t.Fatalf("second line token wrong. expected=2, got=%d", tok.Line)
		}
	}
}