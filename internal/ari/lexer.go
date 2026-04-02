package ari


// TokenType represents different types of tokens in ARI spec
type TokenType int

const (
	// Special tokens
	ILLEGAL TokenType = iota
	EOF
	NEWLINE
	WHITESPACE

	// Literals
	IDENTIFIER
	STRING
	NUMBER
	REGEX
	REGEX_WITH_TRANSFORM

	// Keywords
	SECTION
	FIELD
	BREAK
	STARTS
	ENDS
	ON
	TYPE
	OUTPUT

	// Direction keywords
	LEFT
	RIGHT
	UP
	DOWN
	SAME
	FLUSH

	// Built-in type keywords
	DATE
	MONEY
	INTEGER
	DECIMAL
	SSN
	ISO_DATE

	// Operators
	COLON     // :
	COMMA     // ,
	DASH      // -
	ARROW     // ->
	LPAREN    // (
	RPAREN    // )
	LBRACKET  // [
	RBRACKET  // ]

	// Distance modifiers
	RANGE      // 2-10
	OPEN_MIN   // 3-
	OPEN_MAX   // -8
)

// Token represents a single token
type Token struct {
	Type    TokenType
	Literal string
	Line    int
	Column  int
}

// Lexer tokenizes ARI specification text
type Lexer struct {
	input        string
	position     int  // current position (points to current char)
	readPosition int  // current reading position (after current char)
	ch           byte // current char under examination
	line         int
	column       int
}

// Keywords maps string literals to their token types
var keywords = map[string]TokenType{
	"section": SECTION,
	"field":   FIELD,
	"break":   BREAK,
	"starts":  STARTS,
	"ends":    ENDS,
	"on":      ON,
	"type":    TYPE,
	"output":  OUTPUT,
	"left":    LEFT,
	"right":   RIGHT,
	"up":      UP,
	"down":    DOWN,
	"same":    SAME,
	"flush":   FLUSH,
	"date":    DATE,
	"money":   MONEY,
	"integer": INTEGER,
	"decimal": DECIMAL,
	"ssn":     SSN,
	"ISO_DATE": ISO_DATE,
}

// NewLexer creates a new lexer instance
func NewLexer(input string) *Lexer {
	l := &Lexer{
		input:  input,
		line:   1,
		column: 0,
	}
	l.readChar()
	return l
}

// readChar reads the next character and advances position
func (l *Lexer) readChar() {
	if l.readPosition >= len(l.input) {
		l.ch = 0 // ASCII NUL character represents "EOF"
	} else {
		l.ch = l.input[l.readPosition]
	}
	l.position = l.readPosition
	l.readPosition++

	if l.ch == '\n' {
		l.line++
		l.column = 0
	} else {
		l.column++
	}
}

// peekChar returns the next character without advancing position
func (l *Lexer) peekChar() byte {
	if l.readPosition >= len(l.input) {
		return 0
	}
	return l.input[l.readPosition]
}

// skipWhitespace skips whitespace characters except newlines
func (l *Lexer) skipWhitespace() {
	for l.ch == ' ' || l.ch == '\t' || l.ch == '\r' {
		l.readChar()
	}
}

// readIdentifier reads an identifier or keyword
func (l *Lexer) readIdentifier() string {
	position := l.position
	for isLetter(l.ch) || isDigit(l.ch) || l.ch == '_' {
		l.readChar()
	}
	return l.input[position:l.position]
}

// readNumber reads a number or distance specification
func (l *Lexer) readNumber() (string, TokenType) {
	position := l.position

	// Read the number part
	for isDigit(l.ch) {
		l.readChar()
	}

	// Check for range specifications
	if l.ch == '-' {
		if isDigit(l.peekChar()) {
			// This is a range like "2-10"
			l.readChar() // consume the '-'
			for isDigit(l.ch) {
				l.readChar()
			}
			return l.input[position:l.position], RANGE
		} else {
			// This is an open minimum like "3-"
			l.readChar() // consume the '-'
			return l.input[position:l.position], OPEN_MIN
		}
	}

	return l.input[position:l.position], NUMBER
}

// readString reads a quoted string literal
func (l *Lexer) readString() string {
	position := l.position + 1 // skip opening quote
	for {
		l.readChar()
		if l.ch == '"' || l.ch == 0 {
			break
		}
		// Handle escape sequences
		if l.ch == '\\' {
			l.readChar()
		}
	}
	return l.input[position:l.position]
}

// readRegex reads a regex pattern /pattern/ or /pattern/replacement/
func (l *Lexer) readRegex() (string, TokenType) {
	position := l.position
	l.readChar() // skip opening '/'

	// Read until we find the closing '/'
	for l.ch != '/' && l.ch != 0 {
		if l.ch == '\\' {
			l.readChar() // skip escaped character
		}
		l.readChar()
	}

	if l.ch == '/' {
		l.readChar() // consume closing '/'

		// Check if this is a regex with transformation
		if l.ch == '/' {
			// This is a regex with replacement: /pattern/replacement/
			l.readChar() // skip second '/'
			for l.ch != '/' && l.ch != 0 {
				if l.ch == '\\' {
					l.readChar()
				}
				l.readChar()
			}
			if l.ch == '/' {
				l.readChar() // consume final '/'
			}
			return l.input[position:l.position], REGEX_WITH_TRANSFORM
		}
	}

	return l.input[position:l.position], REGEX
}

// NextToken scans and returns the next token
func (l *Lexer) NextToken() Token {
	var tok Token

	l.skipWhitespace()

	tok.Line = l.line
	tok.Column = l.column

	switch l.ch {
	case ':':
		tok = Token{Type: COLON, Literal: string(l.ch), Line: l.line, Column: l.column}
	case ',':
		tok = Token{Type: COMMA, Literal: string(l.ch), Line: l.line, Column: l.column}
	case '(':
		tok = Token{Type: LPAREN, Literal: string(l.ch), Line: l.line, Column: l.column}
	case ')':
		tok = Token{Type: RPAREN, Literal: string(l.ch), Line: l.line, Column: l.column}
	case '[':
		tok = Token{Type: LBRACKET, Literal: string(l.ch), Line: l.line, Column: l.column}
	case ']':
		tok = Token{Type: RBRACKET, Literal: string(l.ch), Line: l.line, Column: l.column}
	case '\n':
		tok = Token{Type: NEWLINE, Literal: "\\n", Line: l.line, Column: l.column}
	case '"':
		tok.Type = STRING
		tok.Literal = l.readString()
	case '/':
		literal, tokenType := l.readRegex()
		tok.Type = tokenType
		tok.Literal = literal
		return tok
	case '-':
		// Check if this is an open maximum range like "-8"
		if isDigit(l.peekChar()) {
			literal, _ := l.readNumber()
			tok = Token{Type: OPEN_MAX, Literal: literal, Line: l.line, Column: l.column}
			return tok
		} else if l.peekChar() == '>' {
			// This is an arrow ->
			l.readChar() // consume '-'
			l.readChar() // consume '>'
			tok = Token{Type: ARROW, Literal: "->", Line: l.line, Column: l.column}
			return tok
		} else {
			tok = Token{Type: DASH, Literal: string(l.ch), Line: l.line, Column: l.column}
		}
	case 0:
		tok.Literal = ""
		tok.Type = EOF
	default:
		if isLetter(l.ch) {
			tok.Literal = l.readIdentifier()
			tok.Type = lookupIdent(tok.Literal)
			return tok
		} else if isDigit(l.ch) {
			literal, tokenType := l.readNumber()
			tok.Literal = literal
			tok.Type = tokenType
			return tok
		} else {
			tok = Token{Type: ILLEGAL, Literal: string(l.ch), Line: l.line, Column: l.column}
		}
	}

	l.readChar()
	return tok
}

// lookupIdent checks if an identifier is a keyword
func lookupIdent(ident string) TokenType {
	if tok, ok := keywords[ident]; ok {
		return tok
	}
	return IDENTIFIER
}

// isLetter checks if character is a letter
func isLetter(ch byte) bool {
	return 'a' <= ch && ch <= 'z' || 'A' <= ch && ch <= 'Z' || ch == '_'
}

// isDigit checks if character is a digit
func isDigit(ch byte) bool {
	return '0' <= ch && ch <= '9'
}

// TokenString returns string representation of a token type
func (t TokenType) String() string {
	switch t {
	case ILLEGAL:
		return "ILLEGAL"
	case EOF:
		return "EOF"
	case NEWLINE:
		return "NEWLINE"
	case WHITESPACE:
		return "WHITESPACE"
	case IDENTIFIER:
		return "IDENTIFIER"
	case STRING:
		return "STRING"
	case NUMBER:
		return "NUMBER"
	case REGEX:
		return "REGEX"
	case REGEX_WITH_TRANSFORM:
		return "REGEX_WITH_TRANSFORM"
	case SECTION:
		return "SECTION"
	case FIELD:
		return "FIELD"
	case BREAK:
		return "BREAK"
	case STARTS:
		return "STARTS"
	case ENDS:
		return "ENDS"
	case ON:
		return "ON"
	case TYPE:
		return "TYPE"
	case OUTPUT:
		return "OUTPUT"
	case LEFT:
		return "LEFT"
	case RIGHT:
		return "RIGHT"
	case UP:
		return "UP"
	case DOWN:
		return "DOWN"
	case SAME:
		return "SAME"
	case FLUSH:
		return "FLUSH"
	case DATE:
		return "DATE"
	case MONEY:
		return "MONEY"
	case INTEGER:
		return "INTEGER"
	case DECIMAL:
		return "DECIMAL"
	case SSN:
		return "SSN"
	case ISO_DATE:
		return "ISO_DATE"
	case COLON:
		return "COLON"
	case COMMA:
		return "COMMA"
	case DASH:
		return "DASH"
	case ARROW:
		return "ARROW"
	case LPAREN:
		return "LPAREN"
	case RPAREN:
		return "RPAREN"
	case LBRACKET:
		return "LBRACKET"
	case RBRACKET:
		return "RBRACKET"
	case RANGE:
		return "RANGE"
	case OPEN_MIN:
		return "OPEN_MIN"
	case OPEN_MAX:
		return "OPEN_MAX"
	default:
		return "UNKNOWN"
	}
}