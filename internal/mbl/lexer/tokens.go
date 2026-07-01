// Package lexer implements MBL lexical analysis and tokenization
package lexer

import "fmt"

// TokenType represents the type of a token
type TokenType int

const (
	// Special tokens
	ILLEGAL TokenType = iota
	EOF
	NEWLINE

	// Identifiers and literals
	IDENT  // bare words
	TEXT   // "quoted strings"
	NUMBER // 123, 123.456, 1.23e-4
	TIME   // @2026-02-01, @2026-02-01 15:30:00
	MONEY  // ¤19.95 USD

	// Keywords
	IF
	ELSE
	WHILE
	FOR
	IN
	CONSIDER
	WATCH
	APPEND
	AS
	RETURN
	BREAK
	PASS
	NEW
	CATCH
	PROCEDURE
	AND
	OR
	NOT
	TRUE
	FALSE
	MY
	WORLD
	QUOTE
	TAB
	NEWLINE_LITERAL // 'newline' keyword
	EMPTY
	PI
	EULER
	NOTHING
	UNKNOWN
	ANYTHING
	EMBED

	// Operators
	SPREAD    // ...
	PLUS      // +
	MINUS     // -
	MULTIPLY  // *
	DIVIDE    // /
	MODULO    // %
	POWER     // ^
	CONCAT    // &
	EQUAL     // ?=
	NOT_EQUAL // !=
	GT        // >
	LT        // <
	GTE       // >=
	LTE       // <=
	ASSIGN    // =
	RANGE     // ..
	DEFINE    // :

	// Delimiters
	LPAREN    // (
	RPAREN    // )
	LBRACKET  // [
	RBRACKET  // ]
	LBRACE    // {
	RBRACE    // }
	COMMA     // ,
	DOT       // .
	SEMICOLON // ;

	// Indentation
	INDENT
	DEDENT

	// Modifiers
	COPY      // (copy)
	LINK      // (link)
	RESET     // (reset ...)
	EXCLUDE   // (exclude)
	PROTECTED // (protected)
	CASCADE   // (cascade)
	QUIETLY   // (quietly)

	// Comments
	COMMENT       // # comment
	BLOCK_COMMENT // ##...##
)

// Token represents a lexical token
type Token struct {
	Type     TokenType
	Literal  string
	Line     int
	Column   int
	Position int // Byte position in source
}

// String returns a string representation of the token type
func (t TokenType) String() string {
	switch t {
	case ILLEGAL:
		return "ILLEGAL"
	case EOF:
		return "EOF"
	case NEWLINE:
		return "NEWLINE"
	case IDENT:
		return "IDENT"
	case TEXT:
		return "TEXT"
	case NUMBER:
		return "NUMBER"
	case TIME:
		return "TIME"
	case MONEY:
		return "MONEY"
	case IF:
		return "IF"
	case ELSE:
		return "ELSE"
	case WHILE:
		return "WHILE"
	case FOR:
		return "FOR"
	case IN:
		return "IN"
	case CONSIDER:
		return "CONSIDER"
	case WATCH:
		return "WATCH"
	case APPEND:
		return "APPEND"
	case AS:
		return "AS"
	case RETURN:
		return "RETURN"
	case BREAK:
		return "BREAK"
	case PASS:
		return "PASS"
	case NEW:
		return "NEW"
	case CATCH:
		return "CATCH"
	case PROCEDURE:
		return "PROCEDURE"
	case AND:
		return "AND"
	case OR:
		return "OR"
	case NOT:
		return "NOT"
	case TRUE:
		return "TRUE"
	case FALSE:
		return "FALSE"
	case MY:
		return "MY"
	case WORLD:
		return "WORLD"
	case QUOTE:
		return "QUOTE"
	case TAB:
		return "TAB"
	case NEWLINE_LITERAL:
		return "NEWLINE_LITERAL"
	case EMPTY:
		return "EMPTY"
	case PI:
		return "PI"
	case EULER:
		return "EULER"
	case NOTHING:
		return "NOTHING"
	case UNKNOWN:
		return "UNKNOWN"
	case ANYTHING:
		return "ANYTHING"
	case EMBED:
		return "EMBED"
	case SPREAD:
		return "SPREAD"
	case PLUS:
		return "PLUS"
	case MINUS:
		return "MINUS"
	case MULTIPLY:
		return "MULTIPLY"
	case DIVIDE:
		return "DIVIDE"
	case MODULO:
		return "MODULO"
	case POWER:
		return "POWER"
	case CONCAT:
		return "CONCAT"
	case EQUAL:
		return "EQUAL"
	case NOT_EQUAL:
		return "NOT_EQUAL"
	case GT:
		return "GT"
	case LT:
		return "LT"
	case GTE:
		return "GTE"
	case LTE:
		return "LTE"
	case ASSIGN:
		return "ASSIGN"
	case RANGE:
		return "RANGE"
	case DEFINE:
		return "DEFINE"
	case LPAREN:
		return "LPAREN"
	case RPAREN:
		return "RPAREN"
	case LBRACKET:
		return "LBRACKET"
	case RBRACKET:
		return "RBRACKET"
	case LBRACE:
		return "LBRACE"
	case RBRACE:
		return "RBRACE"
	case COMMA:
		return "COMMA"
	case DOT:
		return "DOT"
	case SEMICOLON:
		return "SEMICOLON"
	case INDENT:
		return "INDENT"
	case DEDENT:
		return "DEDENT"
	case COPY:
		return "COPY"
	case LINK:
		return "LINK"
	case RESET:
		return "RESET"
	case EXCLUDE:
		return "EXCLUDE"
	case PROTECTED:
		return "PROTECTED"
	case CASCADE:
		return "CASCADE"
	case QUIETLY:
		return "QUIETLY"
	case COMMENT:
		return "COMMENT"
	case BLOCK_COMMENT:
		return "BLOCK_COMMENT"
	default:
		return "UNKNOWN"
	}
}

// String returns a string representation of the token
func (t Token) String() string {
	return fmt.Sprintf("{Type: %s, Literal: %q, Line: %d, Column: %d}",
		t.Type, t.Literal, t.Line, t.Column)
}

// Keywords map for identifying reserved words
var keywords = map[string]TokenType{
	"if":        IF,
	"else":      ELSE,
	"while":     WHILE,
	"for":       FOR,
	"in":        IN,
	"consider":  CONSIDER,
	"watch":     WATCH,
	"append":    APPEND,
	"as":        AS,
	"return":    RETURN,
	"break":     BREAK,
	"pass":      PASS,
	"new":       NEW,
	"catch":     CATCH,
	"procedure": PROCEDURE,
	"and":       AND,
	"or":        OR,
	"not":       NOT,
	"true":      TRUE,
	"false":     FALSE,
	"my":        MY,
	"world":     WORLD,
	"quote":     QUOTE,
	"tab":       TAB,
	"newline":   NEWLINE_LITERAL,
	"empty":     EMPTY,
	"pi":        PI,
	"euler":     EULER,
	"nothing":   NOTHING,  // Fixed: lowercase
	"unknown":   UNKNOWN,  // Fixed: lowercase
	"anything":  ANYTHING, // Fixed: lowercase
	"embed":     EMBED,
	"quietly":   QUIETLY, // Added: quietly should be a keyword
}

// LookupIdent checks if an identifier is a keyword and returns the appropriate token type
func LookupIdent(ident string) TokenType {
	if tokenType, ok := keywords[ident]; ok {
		return tokenType
	}
	return IDENT
}

// Modifier keywords map for modifiers in parentheses
var modifiers = map[string]TokenType{
	"copy":      COPY,
	"link":      LINK,
	"reset":     RESET,
	"exclude":   EXCLUDE,
	"protected": PROTECTED,
	"cascade":   CASCADE,
	"quietly":   QUIETLY,
}

// LookupModifier checks if a parenthesized word is a modifier
func LookupModifier(modifier string) TokenType {
	if tokenType, ok := modifiers[modifier]; ok {
		return tokenType
	}
	return IDENT // Unknown modifier treated as identifier
}
