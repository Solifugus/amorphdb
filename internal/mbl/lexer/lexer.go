// Package lexer implements MBL lexical analysis and tokenization
package lexer

import (
	"unicode/utf8"
)

// Lexer represents the lexical analyzer
type Lexer struct {
	input        string
	position     int  // current position in input (points to current char)
	readPosition int  // current reading position in input (after current char)
	ch           rune // current char under examination
	line         int  // current line number
	column       int  // current column number

	// Indentation tracking
	indentStack    []int // stack of indentation levels
	pendingDedents int   // number of DEDENT tokens to emit
	atLineStart    bool  // true if we're at the start of a line
}

// New creates a new lexer instance
func New(input string) *Lexer {
	l := &Lexer{
		input:       input,
		line:        1,
		column:      0,
		indentStack: []int{0}, // Start with base indentation level 0
		atLineStart: true,
	}
	l.readChar() // Initialize with first character
	return l
}

// readChar reads the next character and advances position
func (l *Lexer) readChar() {
	if l.readPosition >= len(l.input) {
		l.ch = 0 // represents EOF
		l.position = l.readPosition
	} else {
		var size int
		l.ch, size = utf8.DecodeRuneInString(l.input[l.readPosition:])
		l.position = l.readPosition
		l.readPosition += size
	}

	// Track line and column numbers
	if l.ch == '\n' {
		l.line++
		l.column = 0
		l.atLineStart = true
	} else if l.ch != '\t' || !l.atLineStart {
		l.column++
		// Only reset atLineStart if we're not at a line boundary
		// This prevents readChar from resetting atLineStart before
		// NextToken gets a chance to call handleIndentation
		if l.ch != ' ' && l.ch != '\t' && l.column > 1 {
			l.atLineStart = false
		}
	}
}

// peekChar returns the next character without advancing position
func (l *Lexer) peekChar() rune {
	if l.readPosition >= len(l.input) {
		return 0
	}
	ch, _ := utf8.DecodeRuneInString(l.input[l.readPosition:])
	return ch
}

// NextToken scans and returns the next token
func (l *Lexer) NextToken() Token {
	var tok Token

	// Handle pending DEDENT tokens first
	if l.pendingDedents > 0 {
		l.pendingDedents--
		return Token{Type: DEDENT, Literal: "", Line: l.line, Column: l.column, Position: l.position}
	}

	// Handle indentation at the start of a line
	if l.atLineStart {
		return l.handleIndentation()
	}

	// Skip whitespace (except newlines)
	l.skipWhitespace()

	tok.Line = l.line
	tok.Column = l.column
	tok.Position = l.position

	switch l.ch {
	case 0:
		// Handle final DEDENT tokens at EOF
		if len(l.indentStack) > 1 {
			l.pendingDedents = len(l.indentStack) - 1
			l.indentStack = []int{0}
			if l.pendingDedents > 0 {
				l.pendingDedents--
				return Token{Type: DEDENT, Literal: "", Line: l.line, Column: l.column, Position: l.position}
			}
		}
		tok.Type = EOF
		tok.Literal = ""

	case '\n':
		tok.Type = NEWLINE
		tok.Literal = string(l.ch)
		l.readChar()

	case '#':
		return l.readComment()

	case '"':
		tok.Type = TEXT
		tok.Literal = l.readString()

	case '@':
		tok.Type = TIME
		tok.Literal = l.readTimeLiteral()

	case '¤':
		tok.Type = MONEY
		tok.Literal = l.readMoneyLiteral()

	case '(':
		// Check for modifiers
		if l.isModifier() {
			return l.readModifier()
		}
		tok.Type = LPAREN
		tok.Literal = string(l.ch)
		l.readChar()

	case ')':
		tok.Type = RPAREN
		tok.Literal = string(l.ch)
		l.readChar()

	case '[':
		tok.Type = LBRACKET
		tok.Literal = string(l.ch)
		l.readChar()

	case ']':
		tok.Type = RBRACKET
		tok.Literal = string(l.ch)
		l.readChar()

	case '{':
		tok.Type = LBRACE
		tok.Literal = string(l.ch)
		l.readChar()

	case '}':
		tok.Type = RBRACE
		tok.Literal = string(l.ch)
		l.readChar()

	case ',':
		tok.Type = COMMA
		tok.Literal = string(l.ch)
		l.readChar()

	case '.':
		if l.peekChar() == '.' {
			ch := l.ch
			l.readChar()
			tok.Type = RANGE
			tok.Literal = string(ch) + string(l.ch)
			l.readChar()
		} else {
			tok.Type = DOT
			tok.Literal = string(l.ch)
			l.readChar()
		}

	case '~':
		tok.Type = TILDE
		tok.Literal = string(l.ch)
		l.readChar()

	case ':':
		tok.Type = DEFINE
		tok.Literal = string(l.ch)
		l.readChar()

	case '+':
		tok.Type = PLUS
		tok.Literal = string(l.ch)
		l.readChar()

	case '-':
		tok.Type = MINUS
		tok.Literal = string(l.ch)
		l.readChar()

	case '*':
		tok.Type = MULTIPLY
		tok.Literal = string(l.ch)
		l.readChar()

	case '/':
		tok.Type = DIVIDE
		tok.Literal = string(l.ch)
		l.readChar()

	case '%':
		tok.Type = MODULO
		tok.Literal = string(l.ch)
		l.readChar()

	case '&':
		tok.Type = CONCAT
		tok.Literal = string(l.ch)
		l.readChar()

	case '=':
		tok.Type = ASSIGN
		tok.Literal = string(l.ch)
		l.readChar()

	case '?':
		if l.peekChar() == '=' {
			ch := l.ch
			l.readChar()
			tok.Type = EQUAL
			tok.Literal = string(ch) + string(l.ch)
			l.readChar()
		} else {
			tok.Type = ILLEGAL
			tok.Literal = string(l.ch)
			l.readChar()
		}

	case '>':
		if l.peekChar() == '=' {
			ch := l.ch
			l.readChar()
			tok.Type = GTE
			tok.Literal = string(ch) + string(l.ch)
			l.readChar()
		} else {
			tok.Type = GT
			tok.Literal = string(l.ch)
			l.readChar()
		}

	case '<':
		if l.peekChar() == '=' {
			ch := l.ch
			l.readChar()
			tok.Type = LTE
			tok.Literal = string(ch) + string(l.ch)
			l.readChar()
		} else {
			tok.Type = LT
			tok.Literal = string(l.ch)
			l.readChar()
		}

	default:
		if isLetter(l.ch) {
			tok.Literal = l.readIdentifier()
			tok.Type = LookupIdent(tok.Literal)
			return tok
		} else if isDigit(l.ch) {
			tok.Type = NUMBER
			tok.Literal = l.readNumber()
			return tok
		} else {
			tok.Type = ILLEGAL
			tok.Literal = string(l.ch)
			l.readChar()
		}
	}

	return tok
}

// handleIndentation processes indentation at the beginning of a line
func (l *Lexer) handleIndentation() Token {
	// Count tabs at the beginning of the line
	tabCount := 0
	startPos := l.position

	for l.ch == '\t' {
		tabCount++
		l.readChar()
	}

	// Check for spaces in indentation (error)
	if l.ch == ' ' {
		return Token{Type: ILLEGAL, Literal: "spaces not allowed in indentation, use tabs", Line: l.line, Column: l.column, Position: l.position}
	}

	// Skip empty lines and comments
	if l.ch == '\n' || l.ch == 0 || l.ch == '#' {
		if l.ch == '\n' {
			l.readChar() // Advance past the newline
			return Token{Type: NEWLINE, Literal: "\n", Line: l.line, Column: l.column, Position: l.position}
		}
		// For comments and EOF, continue with normal tokenization
		l.atLineStart = false
		return l.NextToken()
	}

	// We have content on this line, so process indentation
	l.atLineStart = false
	currentLevel := l.indentStack[len(l.indentStack)-1]

	if tabCount > currentLevel {
		// Increased indentation
		l.indentStack = append(l.indentStack, tabCount)
		return Token{Type: INDENT, Literal: "", Line: l.line, Column: l.column, Position: startPos}
	} else if tabCount < currentLevel {
		// Decreased indentation - may need multiple DEDENTs
		dedentCount := 0
		for len(l.indentStack) > 1 && l.indentStack[len(l.indentStack)-1] > tabCount {
			l.indentStack = l.indentStack[:len(l.indentStack)-1]
			dedentCount++
		}

		// Check if we have a valid indentation level
		if len(l.indentStack) > 0 && l.indentStack[len(l.indentStack)-1] != tabCount {
			return Token{Type: ILLEGAL, Literal: "invalid indentation level", Line: l.line, Column: l.column, Position: startPos}
		}

		if dedentCount > 0 {
			l.pendingDedents = dedentCount - 1 // First DEDENT returned immediately
			return Token{Type: DEDENT, Literal: "", Line: l.line, Column: l.column, Position: startPos}
		}
	}

	// Same indentation level, continue with next token
	return l.NextToken()
}

// skipWhitespace skips whitespace characters (except newlines and tabs at line start)
func (l *Lexer) skipWhitespace() {
	for l.ch == ' ' || l.ch == '\r' {
		l.readChar()
	}
}

// readIdentifier reads an identifier or keyword
func (l *Lexer) readIdentifier() string {
	const MAX_IDENTIFIER_LENGTH = 255 // Reasonable limit for identifiers

	position := l.position
	charCount := 0

	for (isLetter(l.ch) || isDigit(l.ch) || l.ch == '_') && charCount < MAX_IDENTIFIER_LENGTH {
		l.readChar()
		charCount++
	}

	return l.input[position:l.position]
}

// readNumber reads a numeric literal
func (l *Lexer) readNumber() string {
	const MAX_NUMBER_LENGTH = 100 // Reasonable limit for numeric literals

	position := l.position
	digitCount := 0

	// Read integer part with bounds checking
	for isDigit(l.ch) && digitCount < MAX_NUMBER_LENGTH {
		l.readChar()
		digitCount++
	}

	// Handle decimal point
	if l.ch == '.' && isDigit(l.peekChar()) && digitCount < MAX_NUMBER_LENGTH {
		l.readChar() // consume '.'
		digitCount++

		for isDigit(l.ch) && digitCount < MAX_NUMBER_LENGTH {
			l.readChar()
			digitCount++
		}
	}

	// Handle scientific notation
	if (l.ch == 'e' || l.ch == 'E') && digitCount < MAX_NUMBER_LENGTH {
		l.readChar() // consume 'e' or 'E'
		digitCount++

		if (l.ch == '+' || l.ch == '-') && digitCount < MAX_NUMBER_LENGTH {
			l.readChar() // consume sign
			digitCount++
		}

		for isDigit(l.ch) && digitCount < MAX_NUMBER_LENGTH {
			l.readChar()
			digitCount++
		}
	}

	return l.input[position:l.position]
}

// readString reads a quoted string with matching delimiter counting
func (l *Lexer) readString() string {
	const MAX_CONSECUTIVE_QUOTES = 10 // Reasonable limit for quote nesting

	startPos := l.position

	// Count opening quotes with safety limit
	openQuotes := 0
	for l.ch == '"' && openQuotes < MAX_CONSECUTIVE_QUOTES {
		openQuotes++
		l.readChar()
	}

	// If we hit the limit, consume remaining quotes as regular content
	if l.ch == '"' {
		// Too many quotes - treat as malformed, consume one quote and return
		l.readChar()
		return l.input[startPos:l.position]
	}

	if openQuotes == 0 {
		return `"`
	}

	// Read until we find matching closing quotes or EOF
	for l.ch != 0 {
		if l.ch == '"' {
			// Look ahead to count consecutive closing quotes with safety limit
			lookAheadPos := l.position
			closeQuotes := 0

			// Count quotes from current position with bounds checking
			for lookAheadPos < len(l.input) && l.input[lookAheadPos] == '"' && closeQuotes < MAX_CONSECUTIVE_QUOTES {
				closeQuotes++
				lookAheadPos++
			}

			if closeQuotes >= openQuotes {
				// Found enough closing quotes - consume exactly what we need
				for i := 0; i < openQuotes; i++ {
					l.readChar()
				}
				return l.input[startPos:l.position]
			} else {
				// Not enough quotes, consume one as content
				l.readChar()
			}
		} else {
			l.readChar()
		}
	}

	// EOF reached - return what we have
	return l.input[startPos:l.position]
}

// readTimeLiteral reads a time literal starting with @
func (l *Lexer) readTimeLiteral() string {
	position := l.position
	l.readChar() // consume '@'

	// Read date part (YYYY-MM-DD format expected)
	for isDigit(l.ch) || l.ch == '-' {
		l.readChar()
	}

	// Check for time part (space followed by HH:MM:SS)
	if l.ch == ' ' {
		l.readChar() // consume space
		for isDigit(l.ch) || l.ch == ':' || l.ch == '.' {
			l.readChar()
		}
	}

	return l.input[position:l.position]
}

// readMoneyLiteral reads a money literal starting with ¤
func (l *Lexer) readMoneyLiteral() string {
	position := l.position
	l.readChar() // consume '¤'

	// Read amount (number)
	for isDigit(l.ch) || l.ch == '.' {
		l.readChar()
	}

	// Skip whitespace before currency code
	for l.ch == ' ' {
		l.readChar()
	}

	// Read currency code (typically 3 letters)
	for isLetter(l.ch) {
		l.readChar()
	}

	return l.input[position:l.position]
}

// readComment reads a comment (# to end of line or ##...##)
func (l *Lexer) readComment() Token {
	position := l.position

	if l.ch == '#' && l.peekChar() == '#' {
		// Block comment ##...##
		l.readChar() // consume first #
		l.readChar() // consume second #

		// Look for closing ##
		for {
			if l.ch == 0 {
				break // EOF
			}
			if l.ch == '#' && l.peekChar() == '#' {
				l.readChar() // consume first #
				l.readChar() // consume second #
				break
			}
			l.readChar()
		}

		return Token{
			Type:     BLOCK_COMMENT,
			Literal:  l.input[position:l.position],
			Line:     l.line,
			Column:   l.column,
			Position: position,
		}
	} else {
		// Single line comment
		for l.ch != '\n' && l.ch != 0 {
			l.readChar()
		}

		return Token{
			Type:     COMMENT,
			Literal:  l.input[position:l.position],
			Line:     l.line,
			Column:   l.column,
			Position: position,
		}
	}
}

// isModifier checks if the current position starts a modifier like (copy)
func (l *Lexer) isModifier() bool {
	if l.ch != '(' {
		return false
	}

	// Look ahead to see if this looks like a modifier
	tempPos := l.readPosition
	for tempPos < len(l.input) && l.input[tempPos] != ')' && l.input[tempPos] != '\n' {
		tempPos++
	}

	if tempPos >= len(l.input) || l.input[tempPos] != ')' {
		return false
	}

	// Extract the word inside parentheses
	word := l.input[l.readPosition:tempPos]
	_, isModifier := modifiers[word]
	return isModifier
}

// readModifier reads a modifier token like (copy)
func (l *Lexer) readModifier() Token {
	position := l.position
	l.readChar() // consume '('

	wordStart := l.position
	for l.ch != ')' && l.ch != 0 && l.ch != '\n' {
		l.readChar()
	}

	word := l.input[wordStart:l.position]

	if l.ch == ')' {
		l.readChar() // consume ')'
	}

	tokenType := LookupModifier(word)

	return Token{
		Type:     tokenType,
		Literal:  l.input[position:l.position],
		Line:     l.line,
		Column:   l.column,
		Position: position,
	}
}

// isLetter checks if a character is a letter
func isLetter(ch rune) bool {
	return (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || ch == '_'
}

// isDigit checks if a character is a digit
func isDigit(ch rune) bool {
	return ch >= '0' && ch <= '9'
}