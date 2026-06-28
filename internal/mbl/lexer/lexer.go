// Package lexer implements MBL lexical analysis and tokenization
package lexer

import (
	"strings"
	"unicode"
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

	// Context tracking for collection operations
	previousToken TokenType // track previous token for contextual parsing
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
		// All # characters start comments (single-line or block)
		if l.peekChar() == '#' {
			// It's a block comment (multiple #)
			return l.readComment()
		} else {
			// It's a single-line comment
			return l.readComment()
		}

	case '"':
		stringLiteral := l.readString()
		// Check if string is properly terminated by checking if it ends with quote(s)
		if !strings.HasSuffix(stringLiteral, "\"") {
			// Unterminated string
			tok.Type = ILLEGAL
			tok.Literal = stringLiteral
		} else {
			tok.Type = TEXT
			tok.Literal = stringLiteral
		}

	case '@':
		tok.Type = TIME
		tok.Literal = l.readTimeLiteral()

	case '¤', '$', '€', '£', '¥':
		tok.Type = MONEY
		tok.Literal = l.readMoneyLiteral()

	case '(':
		// Check if this looks like a standalone modifier (not a function call)
		if l.isStandaloneModifier() {
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
			// Check if it's ... (spread) or .. (range)
			if l.peekChar() == '.' {
				// It's ... (spread)
				l.readChar() // consume the third dot
				tok.Type = SPREAD
				tok.Literal = "..."
				l.readChar()
			} else {
				// It's .. (range)
				tok.Type = RANGE
				tok.Literal = string(ch) + string(l.ch)
				l.readChar()
			}
		} else {
			tok.Type = DOT
			tok.Literal = string(l.ch)
			l.readChar()
		}

	case '~':
		tok.Type = TILDE
		tok.Literal = string(l.ch)
		l.readChar()

	case ';':
		tok.Type = SEMICOLON
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

	case '^':
		tok.Type = POWER
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

	case '!':
		if l.peekChar() == '=' {
			ch := l.ch
			l.readChar()
			tok.Type = NOT_EQUAL
			tok.Literal = string(ch) + string(l.ch)
			l.readChar()
		} else {
			tok.Type = ILLEGAL
			tok.Literal = string(l.ch)
			l.readChar()
		}

	case '?':
		if l.peekChar() == '=' {
			ch := l.ch
			l.readChar()
			tok.Type = EQUAL
			tok.Literal = string(ch) + string(l.ch)
			l.readChar()
		} else {
			// Unknown character starting with ?
			tok.Type = ILLEGAL
			tok.Literal = string(l.ch)
			l.readChar() // Advance past the illegal character
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
			tok.Type = l.lookupIdentWithContext(tok.Literal)
			return tok
		} else if isDigit(l.ch) {
			// Check for invalid number format before parsing
			startPos := l.position
			numberLiteral := l.readNumber()

			// Check for period after the number (like "5.")
			if l.ch == '.' {
				// Invalid: number followed by period without digits
				l.readChar() // consume the period
				tok.Type = ILLEGAL
				tok.Literal = l.input[startPos:l.position]
			} else {
				tok.Type = NUMBER
				tok.Literal = numberLiteral
			}
			return tok
		} else {
			tok.Type = ILLEGAL
			tok.Literal = string(l.ch)
			l.readChar()
		}
	}

	// Update previous token for contextual parsing
	l.previousToken = tok.Type
	return tok
}

// lookupIdentWithContext performs context-aware identifier lookup
// After .., treat "append" and "prepend" as regular identifiers, not keywords
func (l *Lexer) lookupIdentWithContext(ident string) TokenType {
	// If previous token was RANGE (..) and identifier is append/prepend,
	// treat as regular identifier, not keyword
	if l.previousToken == RANGE && (ident == "append" || ident == "prepend") {
		return IDENT
	}
	// Otherwise use standard keyword lookup
	return LookupIdent(ident)
}

// handleIndentation processes indentation at the beginning of a line
func (l *Lexer) handleIndentation() Token {
	// Count indentation level (tabs and spaces)
	spacesCount := 0
	tabsCount := 0
	startPos := l.position

	for l.ch == '\t' || l.ch == ' ' {
		if l.ch == '\t' {
			tabsCount++
		} else {
			spacesCount++
		}
		l.readChar()
	}

	// Check for mixed indentation (tabs and spaces together)
	if tabsCount > 0 && spacesCount > 0 {
		return Token{Type: ILLEGAL, Literal: "mixed tabs and spaces in indentation", Line: l.line, Column: l.column, Position: l.position}
	}

	// Convert to indentation levels (4 spaces = 1 level, 1 tab = 1 level)
	var indentLevel int
	if tabsCount > 0 {
		indentLevel = tabsCount
	} else {
		indentLevel = spacesCount / 4 // 4 spaces = 1 level
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

	if indentLevel > currentLevel {
		// Increased indentation - push exactly one new level and emit a single
		// INDENT. A jump of several columns (e.g. 0 -> 2 tabs) is still one block
		// opening, so it must produce one INDENT matched by one DEDENT on close.
		// Pushing intermediate levels here would emit one INDENT but multiple
		// DEDENTs, unbalancing the token stream and breaking block parsing.
		l.indentStack = append(l.indentStack, indentLevel)
		return Token{Type: INDENT, Literal: "", Line: l.line, Column: l.column, Position: startPos}
	} else if indentLevel < currentLevel {
		// Decreased indentation - may need multiple DEDENTs
		dedentCount := 0
		for len(l.indentStack) > 1 && l.indentStack[len(l.indentStack)-1] > indentLevel {
			l.indentStack = l.indentStack[:len(l.indentStack)-1]
			dedentCount++
		}

		// Check if we have a valid indentation level
		if len(l.indentStack) > 0 && l.indentStack[len(l.indentStack)-1] != indentLevel {
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

// readNumber reads a numeric literal with underscore support
func (l *Lexer) readNumber() string {
	const MAX_NUMBER_LENGTH = 100 // Reasonable limit for numeric literals

	digitCount := 0
	var result strings.Builder

	// Read integer part with underscore support
	for (isDigit(l.ch) || l.ch == '_') && digitCount < MAX_NUMBER_LENGTH {
		if l.ch == '_' {
			// Skip underscores, don't add to result
			l.readChar()
			continue
		}
		result.WriteRune(l.ch)
		l.readChar()
		digitCount++
	}

	// Handle decimal point - must be followed by digit
	if l.ch == '.' && isDigit(l.peekChar()) && digitCount < MAX_NUMBER_LENGTH {
		result.WriteRune(l.ch)
		l.readChar() // consume '.'
		digitCount++

		for (isDigit(l.ch) || l.ch == '_') && digitCount < MAX_NUMBER_LENGTH {
			if l.ch == '_' {
				// Skip underscores, don't add to result
				l.readChar()
				continue
			}
			result.WriteRune(l.ch)
			l.readChar()
			digitCount++
		}
	}

	// Handle scientific notation
	if (l.ch == 'e' || l.ch == 'E') && digitCount < MAX_NUMBER_LENGTH {
		result.WriteRune(l.ch)
		l.readChar() // consume 'e' or 'E'
		digitCount++

		if (l.ch == '+' || l.ch == '-') && digitCount < MAX_NUMBER_LENGTH {
			result.WriteRune(l.ch)
			l.readChar() // consume sign
			digitCount++
		}

		for (isDigit(l.ch) || l.ch == '_') && digitCount < MAX_NUMBER_LENGTH {
			if l.ch == '_' {
				// Skip underscores, don't add to result
				l.readChar()
				continue
			}
			result.WriteRune(l.ch)
			l.readChar()
			digitCount++
		}
	}

	return result.String()
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
				// Found enough closing quotes - consume all available quotes
				for i := 0; i < closeQuotes; i++ {
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

	// Handle identifier-like time references (e.g., @time, @agent)
	if isLetter(l.ch) {
		for isLetter(l.ch) || isDigit(l.ch) || l.ch == '_' {
			l.readChar()
		}
		return l.input[position:l.position]
	}

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

// readComment reads a comment according to the spec:
//   - Single # comments to end of line or to a closing # on the same line
//   - Two or more adjacent # characters open a block comment that terminates
//     at the next occurrence of the same number of adjacent # characters
func (l *Lexer) readComment() Token {
	position := l.position

	// Count opening # characters
	openHashCount := 0
	for l.ch == '#' {
		openHashCount++
		l.readChar()
	}

	if openHashCount == 1 {
		// Single # comment - look for closing # on same line or end of line
		for l.ch != '\n' && l.ch != 0 {
			if l.ch == '#' {
				// Found potential closing #
				l.readChar() // consume the #
				break
			}
			l.readChar()
		}

		return Token{
			Type:     COMMENT,
			Literal:  l.input[position:l.position],
			Line:     l.line,
			Column:   l.column,
			Position: position,
		}
	} else {
		// Block comment - look for matching number of # characters
		for {
			if l.ch == 0 {
				break // EOF - unterminated block comment
			}

			if l.ch == '#' {
				// Count consecutive # characters
				hashCount := 0
				tempPos := l.position

				for l.ch == '#' && hashCount < openHashCount {
					hashCount++
					l.readChar()
				}

				if hashCount == openHashCount {
					// Found matching closing sequence
					break
				} else {
					// Not enough # characters, backtrack and continue
					l.position = tempPos
					l.readPosition = tempPos + 1
					if tempPos < len(l.input) {
						var size int
						l.ch, size = utf8.DecodeRuneInString(l.input[tempPos:])
						l.readPosition = tempPos + size
					}
					l.readChar() // consume the # and continue
				}
			} else {
				l.readChar()
			}
		}

		return Token{
			Type:     BLOCK_COMMENT,
			Literal:  l.input[position:l.position],
			Line:     l.line,
			Column:   l.column,
			Position: position,
		}
	}
}

// isStandaloneModifier checks if this looks like a standalone modifier (word) not a function call
func (l *Lexer) isStandaloneModifier() bool {
	if l.ch != '(' {
		return false
	}

	// Look ahead to find the content and closing paren
	tempPos := l.readPosition
	wordStart := tempPos
	for tempPos < len(l.input) && l.input[tempPos] != ')' && l.input[tempPos] != '\n' {
		tempPos++
	}

	// Must have closing parenthesis
	if tempPos >= len(l.input) || l.input[tempPos] != ')' {
		return false
	}

	// Check what follows the closing parenthesis - function definitions have ':'
	afterClosing := tempPos + 1
	if afterClosing < len(l.input) && l.input[afterClosing] == ':' {
		return false // This looks like a function definition: identifier(params):
	}

	// Check if the content between parentheses is a known modifier
	word := l.input[wordStart:tempPos]
	if len(word) == 0 {
		return false
	}

	// Check if it's a known single-word modifier
	if _, isKnownModifier := modifiers[word]; isKnownModifier {
		return true
	}

	// Check for reset expressions like "reset 0"
	wordTrimmed := strings.TrimSpace(word)
	if strings.HasPrefix(wordTrimmed, "reset ") {
		return true
	}

	// A '(' that immediately follows an identifier character is a function
	// call's argument list (e.g. foo(x)), not a standalone modifier. Known
	// modifiers were already handled above (so new(copy) still works); only
	// the extensible "unknown modifier" path below must avoid swallowing a
	// single-identifier call argument like (x) into one IDENT token.
	if l.position > 0 {
		prev := l.input[l.position-1]
		if prev == '_' || unicode.IsLetter(rune(prev)) || unicode.IsDigit(rune(prev)) {
			return false
		}
	}

	// Allow unknown modifiers if they are valid identifier words (no dots, spaces, quotes, or numbers)
	// This enables extensible modifier syntax like (unknown_modifier)
	// but rejects complex expressions like (my.template), literals like (4), or strings like ("text")
	if len(wordTrimmed) > 0 &&
		!strings.Contains(wordTrimmed, ".") &&
		!strings.Contains(wordTrimmed, " ") &&
		!strings.Contains(wordTrimmed, `"`) &&
		!strings.ContainsAny(wordTrimmed, "0123456789") {
		// Must start with a letter or underscore to be a valid identifier
		if len(wordTrimmed) > 0 && (unicode.IsLetter([]rune(wordTrimmed)[0]) || []rune(wordTrimmed)[0] == '_') {
			return true
		}
	}

	return false
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

	// Determine token type
	var tokenType TokenType
	if modifierType, ok := modifiers[word]; ok {
		tokenType = modifierType
	} else if strings.HasPrefix(strings.TrimSpace(word), "reset ") {
		tokenType = RESET
	} else {
		tokenType = IDENT
	}

	return Token{
		Type:     tokenType,
		Literal:  l.input[position:l.position],
		Line:     l.line,
		Column:   l.column,
		Position: position,
	}
}

// ContainsPattern checks if the remaining input contains a specific pattern
func (l *Lexer) ContainsPattern(pattern string) bool {
	remaining := l.input[l.position:]
	for i := 0; i <= len(remaining)-len(pattern); i++ {
		if remaining[i:i+len(pattern)] == pattern {
			return true
		}
		// Stop at newline (end of current statement)
		if remaining[i] == '\n' {
			break
		}
	}
	return false
}

// EndsWithPattern checks if the remaining input ends with a specific pattern
func (l *Lexer) EndsWithPattern(pattern string) bool {
	remaining := l.input[l.position:]

	// Find the end of the current statement (newline or EOF)
	statementEnd := len(remaining)
	for i, ch := range remaining {
		if ch == '\n' {
			statementEnd = i
			break
		}
	}

	// Check if the statement ends with the pattern
	statement := remaining[:statementEnd]
	return len(statement) >= len(pattern) && statement[len(statement)-len(pattern):] == pattern
}

// isLetter checks if a character is a letter (including Unicode letters)
func isLetter(ch rune) bool {
	return unicode.IsLetter(ch) || ch == '_'
}

// isDigit checks if a character is a digit
func isDigit(ch rune) bool {
	return ch >= '0' && ch <= '9'
}
