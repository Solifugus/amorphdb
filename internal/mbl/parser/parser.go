// Package parser implements MBL syntax analysis and AST generation
package parser

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/solifugus/amorphdb/internal/mbl/lexer"
)

// Operator precedence levels (lowest to highest)
const (
	_ int = iota
	LOWEST
	OR      // or
	AND     // and
	NOT     // not
	EQUALS  // ?=, >, <, >=, <=
	CONCAT  // &
	SUM     // +, -
	PRODUCT // *, /, %
	POWER   // ^ (exponentiation - higher than multiplication)
	UNARY   // -x, +x, not x
	RANGE   // ..
	CALL    // myFunction(x), a.b, a[b]
)

// Parser represents the syntax analyzer
type Parser struct {
	lexer *lexer.Lexer

	currentToken lexer.Token
	peekToken    lexer.Token

	errors []string

	// Memory safety
	recursionDepth    int
	maxRecursionDepth int // 1000
	nodeCount         int
	maxNodes          int // 100,000

	// Operator precedence map
	precedences map[lexer.TokenType]int

	// Pratt parser functions
	prefixParseFns map[lexer.TokenType]prefixParseFn
	infixParseFns  map[lexer.TokenType]infixParseFn
}

type (
	prefixParseFn func() Expression
	infixParseFn  func(Expression) Expression
)

// New creates a new parser instance
func New(l *lexer.Lexer) *Parser {
	p := &Parser{
		lexer:             l,
		errors:            []string{},
		maxRecursionDepth: 1000,
		maxNodes:          100000,
	}

	// Initialize precedence map
	p.precedences = map[lexer.TokenType]int{
		lexer.OR:       OR,
		lexer.AND:      AND,
		lexer.NOT:      NOT,
		lexer.EQUAL:    EQUALS,
		lexer.ASSIGN:   EQUALS, // = can be comparison in expression context
		lexer.GT:       EQUALS,
		lexer.LT:       EQUALS,
		lexer.GTE:      EQUALS,
		lexer.LTE:      EQUALS,
		lexer.CONCAT:   CONCAT,
		lexer.PLUS:     SUM,
		lexer.MINUS:    SUM,
		lexer.MULTIPLY: PRODUCT,
		lexer.DIVIDE:   PRODUCT,
		lexer.MODULO:   PRODUCT,
		lexer.POWER:    POWER,
		lexer.RANGE:    RANGE,
		lexer.DOT:      CALL,
		lexer.LBRACKET: CALL,
		lexer.LBRACE:   CALL, // projection syntax
		lexer.LPAREN:   CALL,
	}

	// Initialize prefix parse functions
	p.prefixParseFns = make(map[lexer.TokenType]prefixParseFn)
	p.registerPrefix(lexer.IDENT, p.parseIdentifier)
	p.registerPrefix(lexer.MY, p.parseMyExpression)
	p.registerPrefix(lexer.WORLD, p.parseWorldExpression)
	p.registerPrefix(lexer.TEXT, p.parseStringLiteral)
	p.registerPrefix(lexer.NUMBER, p.parseNumberLiteral)
	p.registerPrefix(lexer.TIME, p.parseTimeLiteral)
	p.registerPrefix(lexer.MONEY, p.parseMoneyLiteral)
	p.registerPrefix(lexer.TRUE, p.parseBooleanLiteral)
	p.registerPrefix(lexer.FALSE, p.parseBooleanLiteral)
	p.registerPrefix(lexer.NOTHING, p.parseNothingLiteral)
	p.registerPrefix(lexer.UNKNOWN, p.parseUnknownLiteral)
	p.registerPrefix(lexer.QUEUED, p.parseQueuedLiteral)
	p.registerPrefix(lexer.ANYTHING, p.parseAnythingLiteral)
	p.registerPrefix(lexer.PI, p.parsePiLiteral)
	p.registerPrefix(lexer.EULER, p.parseEulerLiteral)
	p.registerPrefix(lexer.EMPTY, p.parseEmptyLiteral)
	p.registerPrefix(lexer.QUOTE, p.parseQuoteLiteral)
	p.registerPrefix(lexer.TAB, p.parseTabLiteral)
	p.registerPrefix(lexer.NEWLINE_LITERAL, p.parseNewlineLiteral)
	p.registerPrefix(lexer.MINUS, p.parseUnaryExpression)
	p.registerPrefix(lexer.PLUS, p.parseUnaryExpression)
	p.registerPrefix(lexer.NOT, p.parseUnaryExpression)
	p.registerPrefix(lexer.QUIETLY, p.parseQuietlyExpression)
	p.registerPrefix(lexer.LPAREN, p.parseGroupedExpression)
	p.registerPrefix(lexer.LBRACE, p.parseRecordLiteral)
	p.registerPrefix(lexer.LBRACKET, p.parseListLiteral)
	p.registerPrefix(lexer.DEFINE, p.parseDefinitionExpression)
	p.registerPrefix(lexer.WATCH, p.parseWatchExpression)
	p.registerPrefix(lexer.PROCEDURE, p.parseProcedureExpression)

	// Initialize infix parse functions
	p.infixParseFns = make(map[lexer.TokenType]infixParseFn)
	p.registerInfix(lexer.OR, p.parseBinaryExpression)
	p.registerInfix(lexer.AND, p.parseBinaryExpression)
	p.registerInfix(lexer.EQUAL, p.parseBinaryExpression)
	p.registerInfix(lexer.ASSIGN, p.parseBinaryExpression) // = as comparison in expression context
	p.registerInfix(lexer.GT, p.parseBinaryExpression)
	p.registerInfix(lexer.LT, p.parseBinaryExpression)
	p.registerInfix(lexer.GTE, p.parseBinaryExpression)
	p.registerInfix(lexer.LTE, p.parseBinaryExpression)
	p.registerInfix(lexer.CONCAT, p.parseBinaryExpression)
	p.registerInfix(lexer.PLUS, p.parseBinaryExpression)
	p.registerInfix(lexer.MINUS, p.parseBinaryExpression)
	p.registerInfix(lexer.MULTIPLY, p.parseBinaryExpression)
	p.registerInfix(lexer.DIVIDE, p.parseBinaryExpression)
	p.registerInfix(lexer.MODULO, p.parseBinaryExpression)
	p.registerInfix(lexer.POWER, p.parseBinaryExpression)
	p.registerInfix(lexer.RANGE, p.parseCollectionOperation)
	p.registerInfix(lexer.DOT, p.parseCallExpression)
	p.registerInfix(lexer.LBRACKET, p.parseBracketFilterExpression)
	p.registerInfix(lexer.LBRACE, p.parseProjectionExpression)
	p.registerInfix(lexer.LPAREN, p.parseCallExpression)

	// Read two tokens, so currentToken and peekToken are both set
	p.nextToken()
	p.nextToken()

	return p
}

// registerPrefix associates a token type with a prefix parse function
func (p *Parser) registerPrefix(tokenType lexer.TokenType, fn prefixParseFn) {
	p.prefixParseFns[tokenType] = fn
}

// registerInfix associates a token type with an infix parse function
func (p *Parser) registerInfix(tokenType lexer.TokenType, fn infixParseFn) {
	p.infixParseFns[tokenType] = fn
}

// nextToken advances both currentToken and peekToken
func (p *Parser) nextToken() {
	p.currentToken = p.peekToken
	p.peekToken = p.lexer.NextToken()
}

// skipComments advances past comment tokens
func (p *Parser) skipComments() {
	for p.currentToken.Type == lexer.COMMENT || p.currentToken.Type == lexer.BLOCK_COMMENT {
		p.nextToken()
	}
}

// Errors returns all parsing errors encountered
func (p *Parser) Errors() []string {
	return p.errors
}

// peekError adds an error for an unexpected peek token
func (p *Parser) peekError(t lexer.TokenType) {
	msg := fmt.Sprintf("expected next token to be %s, got %s instead at line %d, column %d",
		t, p.peekToken.Type, p.peekToken.Line, p.peekToken.Column)
	p.errors = append(p.errors, msg)
}

// noPrefixParseFnError adds an error when no prefix parse function is found
func (p *Parser) noPrefixParseFnError(t lexer.TokenType) {
	// Special case: consecutive ASSIGN tokens (==) - provide better error
	if t == lexer.ASSIGN && p.peekToken.Type == lexer.ASSIGN {
		msg := fmt.Sprintf("invalid operator '==' at line %d, column %d. Use '?=' for equality comparison",
			p.currentToken.Line, p.currentToken.Column)
		p.errors = append(p.errors, msg)
		return
	}

	// Special case: single ASSIGN in unexpected position
	if t == lexer.ASSIGN {
		// Check if this might be part of == pattern by looking at previous parsing context
		msg := fmt.Sprintf("unexpected '=' at line %d, column %d. If using '==' for comparison, use '?=' instead",
			p.currentToken.Line, p.currentToken.Column)
		p.errors = append(p.errors, msg)
		return
	}

	// Generic error for other tokens
	msg := fmt.Sprintf("unexpected token %s at line %d, column %d",
		t, p.currentToken.Line, p.currentToken.Column)
	p.errors = append(p.errors, msg)
}

// expectPeek checks if the next token is of the expected type and advances
func (p *Parser) expectPeek(t lexer.TokenType) bool {
	if p.peekToken.Type == t {
		p.nextToken()
		return true
	}
	p.peekError(t)
	return false
}

// currentTokenIs checks if the current token matches the given type
func (p *Parser) currentTokenIs(t lexer.TokenType) bool {
	return p.currentToken.Type == t
}

// peekTokenIs checks if the peek token matches the given type
func (p *Parser) peekTokenIs(t lexer.TokenType) bool {
	return p.peekToken.Type == t
}

// peekPrecedence returns the precedence of the peek token
func (p *Parser) peekPrecedence() int {
	if p, ok := p.precedences[p.peekToken.Type]; ok {
		return p
	}
	return LOWEST
}

// currentPrecedence returns the precedence of the current token
func (p *Parser) currentPrecedence() int {
	if p, ok := p.precedences[p.currentToken.Type]; ok {
		return p
	}
	return LOWEST
}

// incrementRecursionDepth safely increments recursion depth
func (p *Parser) incrementRecursionDepth() bool {
	p.recursionDepth++
	if p.recursionDepth > p.maxRecursionDepth {
		p.errors = append(p.errors, fmt.Sprintf("maximum recursion depth (%d) exceeded", p.maxRecursionDepth))
		return false
	}
	return true
}

// decrementRecursionDepth safely decrements recursion depth
func (p *Parser) decrementRecursionDepth() {
	if p.recursionDepth > 0 {
		p.recursionDepth--
	}
}

// hasCriticalError checks if the parser has hit errors that should stop parsing
func (p *Parser) hasCriticalError() bool {
	for _, err := range p.errors {
		if strings.Contains(err, "maximum recursion depth") ||
		   strings.Contains(err, "maximum node count") {
			return true
		}
	}
	return false
}

// incrementNodeCount safely increments node count
func (p *Parser) incrementNodeCount() bool {
	p.nodeCount++
	if p.nodeCount > p.maxNodes {
		p.errors = append(p.errors, fmt.Sprintf("maximum node count (%d) exceeded", p.maxNodes))
		return false
	}
	return true
}

// ParseProgram parses the entire program and returns the AST
func (p *Parser) ParseProgram() *Program {
	program := &Program{
		Statements: []Statement{},
		Token:      p.currentToken,
	}

	for p.currentToken.Type != lexer.EOF {
		p.skipComments()
		if p.currentToken.Type == lexer.EOF {
			break
		}

		// Skip newlines at the top level
		if p.currentToken.Type == lexer.NEWLINE {
			p.nextToken()
			continue
		}

		stmt := p.parseStatement()
		if stmt != nil {
			program.Statements = append(program.Statements, stmt)
		}

		// Check for critical errors that should stop parsing
		if p.hasCriticalError() {
			break
		}

		// Handle semicolon separation
		if p.peekToken.Type == lexer.SEMICOLON {
			p.nextToken() // move to semicolon
			p.nextToken() // consume semicolon, move to next statement
			// Continue parsing next statement on the same line
			continue
		}

		p.nextToken()
	}

	return program
}

// parseStatement parses individual statements
func (p *Parser) parseStatement() Statement {
	if !p.incrementRecursionDepth() {
		return nil
	}
	defer p.decrementRecursionDepth()

	if !p.incrementNodeCount() {
		return nil
	}

	p.skipComments()

	switch p.currentToken.Type {
	case lexer.IF:
		return p.parseIfStatement()
	case lexer.WHILE:
		return p.parseWhileStatement()
	case lexer.FOR:
		return p.parseForStatement()
	case lexer.CONSIDER:
		return p.parseConsiderStatement()
	case lexer.WATCH:
		return p.parseWatchStatement()
	case lexer.CATCH:
		return p.parseCatchStatement()
	case lexer.RETURN:
		return p.parseReturnStatement()
	case lexer.PASS:
		return p.parsePassStatement()
	case lexer.NEW:
		return p.parseInstantiationStatement()
	case lexer.EMBED:
		return p.parseEmbedDirectiveStatement()
	case lexer.SPREAD:
		return p.parseEmbedDirectiveStatement()
	case lexer.DOT:
		// Scope-relative assignment (e.g., .local.variable = 5)
		return p.parseScopeRelativeAssignment()
	default:
		// Check if this might be an assignment or procedure definition
		if p.currentToken.Type == lexer.IDENT ||
			p.currentToken.Type == lexer.MY ||
			p.currentToken.Type == lexer.WORLD {

			if p.isDefinition() {
				return p.parseDefinitionStatement()
			} else if p.isSimpleAssignment() {
				return p.parseAssignmentStatement()
			} else if p.isProcedureDefinition() {
				return p.parseProcedureStatement()
			} else if p.isWatchStatement() {
				return p.parseWatchStatement()
			} else if p.isScopeStatement() {
				return p.parseScopeStatement()
			}
		} else if p.currentToken.Type == lexer.PLUS {
			// Check for append assignment (+my.path = value)
			if p.isAppendAssignment() {
				return p.parseAppendAssignmentStatement()
			}
		}
		// Fall back to expression statement
		return p.parseExpressionStatement()
	}
}

// isSimpleAssignment checks for obvious assignment patterns only
func (p *Parser) isSimpleAssignment() bool {
	// Check for identifier or path-starting keywords first
	if p.currentToken.Type != lexer.IDENT &&
		p.currentToken.Type != lexer.MY &&
		p.currentToken.Type != lexer.WORLD {
		return false
	}

	// Simple case: single identifier followed by ASSIGN
	if p.peekToken.Type == lexer.ASSIGN {
		return true
	}

	// Complex case: modifier syntax (x:(copy) = ...) but not watch statements
	if p.peekToken.Type == lexer.DEFINE {
		// Check if this is a watch statement first
		if p.lexer.ContainsPattern("watch") {
			return false // Let watch statement parser handle this
		}
		return true
	}

	// Check for path pattern (identifier.identifier...)
	if p.peekToken.Type == lexer.DOT {
		// For storage keywords (my, world), check if there's actually an assignment
		if p.currentToken.Type == lexer.MY || p.currentToken.Type == lexer.WORLD {
			return p.looksLikePathAssignment()
		}
		// Regular identifiers with dots are local record field access
		return false
	}

	return false
}

// looksLikePathAssignment does limited lookahead to detect path assignments
func (p *Parser) looksLikePathAssignment() bool {
	// This is called when currentToken is MY or WORLD
	// We need to check if the pattern is: MY/WORLD [.ident]* (=|:) value

	// If next token is immediate assignment, it's definitely assignment
	if p.peekToken.Type == lexer.ASSIGN {
		return true
	}

	// If next token is define (colon), it could be assignment too
	if p.peekToken.Type == lexer.DEFINE {
		return true
	}

	// Use the existing ContainsPattern method to look for assignment patterns
	// Check for = operators after a path pattern (: is for definitions, not assignments)
	return p.lexer.ContainsPattern(" = ") ||
		   p.lexer.ContainsPattern("= ")
}

// isProcedureDefinition checks if the current statement is a procedure definition
func (p *Parser) isProcedureDefinition() bool {
	if p.currentToken.Type != lexer.IDENT || p.peekToken.Type != lexer.LPAREN {
		return false
	}

	// Key insight: procedure definitions are at column 1 and have the pattern:
	// identifier(params): body
	// Function calls either:
	// 1. Are not at column 1 (indented), OR
	// 2. Don't have a colon after the closing parenthesis

	if p.currentToken.Column != 1 {
		return false // Indented - definitely a function call
	}

	// At column 1 - could be either procedure definition or function call.
	// Use a simple heuristic: check if the input line contains "):" pattern

	// Known built-in function names that should never be procedures
	tokenLiteral := p.currentToken.Literal
	knownFunctions := []string{
		// Core utility functions
		"len", "text", "number", "add", "output",
		// String operations
		"upper", "lower", "trim", "substring", "find", "replace", "split",
		// Math operations
		"abs", "floor", "ceil", "max", "min", "round",
		// Legacy test functions
		"multiply", "process", "call",
	}
	for _, fn := range knownFunctions {
		if tokenLiteral == fn {
			return false
		}
	}

	// Now we can properly check if the remaining input contains "):" pattern
	return p.lexer.ContainsPattern("):")
}

// isWatchStatement checks if the current statement is a watch statement
func (p *Parser) isWatchStatement() bool {
	// Watch statements have the pattern: name: watch(...)
	// Check for identifier followed by DEFINE then WATCH
	if p.currentToken.Type != lexer.IDENT || p.peekToken.Type != lexer.DEFINE {
		return false
	}

	// Use simple pattern check for watch keyword after colon
	return p.lexer.ContainsPattern("watch")
}

// isScopeStatement checks for scope setting patterns (path ending with . or .:)
func (p *Parser) isScopeStatement() bool {
	// Quick check: if this isn't a path start, not a scope statement
	if p.currentToken.Type != lexer.IDENT &&
		p.currentToken.Type != lexer.MY &&
		p.currentToken.Type != lexer.WORLD {
		return false
	}

	// Scope statements must contain ".:" pattern (with body) or end with "." (simple scope set)
	// Use lexer lookahead to find these patterns
	return p.lexer.ContainsPattern(".:") ||      // scope with body
		p.lexer.ContainsPattern(".\n:") ||    // multi-line scope with body
		p.lexer.ContainsPattern(".") && !p.lexer.ContainsPattern("[") // scope set (but not bracket filters)
}

// parseScopeStatement parses scope setting statements like my.path.
func (p *Parser) parseScopeStatement() Statement {
	stmt := &ScopeStatement{Token: p.currentToken}

	// Parse the path manually to ensure we capture everything
	pathStr := ""

	// Add the first token (MY/WORLD/IDENT)
	pathStr += p.currentToken.Literal
	p.nextToken()

	// Continue while we see DOT followed by another token
	for p.currentToken.Type == lexer.DOT {
		pathStr += "."
		p.nextToken()

		// Check if this is the trailing dot (followed by EOF/newline/semicolon)
		if p.currentToken.Type == lexer.EOF ||
			p.currentToken.Type == lexer.NEWLINE ||
			p.currentToken.Type == lexer.SEMICOLON {
			// This is a trailing dot, we're done
			break
		}

		// Otherwise, there should be another identifier
		if p.currentToken.Type == lexer.IDENT {
			pathStr += p.currentToken.Literal
			p.nextToken()
		} else {
			// Unexpected token, stop parsing
			break
		}
	}

	stmt.Path = pathStr

	// Check for optional body after scope path
	// If we see a DEFINE token (colon), expect a body
	if p.currentToken.Type == lexer.DEFINE {
		p.nextToken() // consume ":"

		// Skip NEWLINE tokens before INDENT
		for p.peekToken.Type == lexer.NEWLINE {
			p.nextToken()
		}

		if p.peekToken.Type == lexer.INDENT {
			p.nextToken() // consume NEWLINE (now at INDENT)
			stmt.Body = p.parseBlockStatement()
		}
	}

	return stmt
}

// isAppendAssignment checks for append assignment patterns (+my.path = value)
func (p *Parser) isAppendAssignment() bool {
	// Should start with + followed by path
	if p.currentToken.Type != lexer.PLUS {
		return false
	}
	// Check if next token starts a path
	return p.peekToken.Type == lexer.IDENT ||
		p.peekToken.Type == lexer.MY ||
		p.peekToken.Type == lexer.WORLD
}

// isDefinition checks if the current statement is a definition (using : colon)
func (p *Parser) isDefinition() bool {
	// Check for identifier or path-starting keywords first
	if p.currentToken.Type != lexer.IDENT &&
		p.currentToken.Type != lexer.MY &&
		p.currentToken.Type != lexer.WORLD {
		return false
	}

	// Only check immediate next token to avoid confusion with record literals
	// If next token is immediate assignment, it's definitely assignment (not definition)
	if p.peekToken.Type == lexer.ASSIGN {
		return false
	}

	// If next token is immediate definition, check if this looks like a path definition
	if p.peekToken.Type == lexer.DEFINE {
		return true
	}

	// For paths like my.var: value, we need to scan ahead carefully
	// Look for colon followed by space (definition syntax)
	return p.lexer.ContainsPattern(": ") && !p.lexer.ContainsPattern("= ")
}

// parseAppendAssignmentStatement parses append assignments (+my.list = item)
func (p *Parser) parseAppendAssignmentStatement() Statement {
	stmt := &AppendAssignmentStatement{Token: p.currentToken} // PLUS token

	// Move past the PLUS to parse the path
	p.nextToken()
	pathExpr := p.parsePathExpression()
	stmt.Name = pathExpr

	// Expect assignment operator
	if !p.expectPeek(lexer.ASSIGN) {
		return nil
	}

	p.nextToken()
	stmt.Value = p.parseExpression(LOWEST)

	// Consume optional semicolon or newline
	if p.peekToken.Type == lexer.NEWLINE {
		p.nextToken()
	}

	return stmt
}

// parseAssignmentStatement parses variable assignments
func (p *Parser) parseAssignmentStatement() Statement {
	stmt := &AssignmentStatement{Token: p.currentToken}

	// Parse the left-hand side path (my.data.value)
	pathExpr := p.parsePathExpression()
	stmt.Name = pathExpr

	// Check for modifier like :(copy) or same-line definition like : value
	var modifier *string
	if p.peekToken.Type == lexer.DEFINE {
		p.nextToken() // consume DEFINE ":"

		// Check for modifier tokens (COPY, LINK, RESET, etc.)
		if p.peekToken.Type == lexer.COPY ||
			p.peekToken.Type == lexer.LINK ||
			p.peekToken.Type == lexer.RESET ||
			p.peekToken.Type == lexer.EXCLUDE ||
			p.peekToken.Type == lexer.PROTECTED ||
			p.peekToken.Type == lexer.CASCADE ||
			p.peekToken.Type == lexer.QUIETLY {
			p.nextToken() // consume the modifier token
			// Extract modifier name and handle reset expressions
			modifierLiteral := p.currentToken.Literal
			if len(modifierLiteral) > 2 && modifierLiteral[0] == '(' && modifierLiteral[len(modifierLiteral)-1] == ')' {
				modifierValue := modifierLiteral[1 : len(modifierLiteral)-1]

				// Handle reset expressions in assignments
				if strings.HasPrefix(modifierValue, "reset ") {
					resetStr := "reset"
					modifier = &resetStr
					stmt.Modifier = modifier

					// TODO: Handle reset expressions in assignments - for now store as-is
				} else {
					modifier = &modifierValue
					stmt.Modifier = modifier
				}
			}
		} else if p.peekToken.Type != lexer.NEWLINE {
			// Same-line definition: path: value (no assignment operator needed)
			p.nextToken() // move to the value expression
			stmt.Value = p.parseExpression(LOWEST)

			// Consume optional semicolon or newline
			if p.peekToken.Type == lexer.SEMICOLON {
				p.nextToken()
			} else if p.peekToken.Type == lexer.NEWLINE {
				p.nextToken()
			}

			return stmt
		}
	}

	// Check if there's an assignment operator
	if p.peekToken.Type != lexer.ASSIGN {
		// No assignment - this is a read operation, return as expression statement
		exprStmt := &ExpressionStatement{Token: stmt.Token}
		exprStmt.Expression = pathExpr
		return exprStmt
	}

	// There is an assignment - continue as normal assignment statement
	p.nextToken() // consume the ASSIGN token
	p.nextToken() // move to the value expression

	stmt.Value = p.parseExpression(LOWEST)

	// Consume optional semicolon or newline
	if p.peekToken.Type == lexer.NEWLINE {
		p.nextToken()
	}

	return stmt
}

// parseEmbedDirectiveStatement parses embed directives like "embed my.stamp" or "...my.stamp"
func (p *Parser) parseEmbedDirectiveStatement() Statement {
	stmt := &EmbedDirectiveStatement{Token: p.currentToken}

	// Move to the path expression
	p.nextToken()

	// Parse the path being embedded
	stmt.Path = p.parseExpression(LOWEST)

	// Consume optional semicolon or newline
	if p.peekToken.Type == lexer.SEMICOLON {
		p.nextToken()
	} else if p.peekToken.Type == lexer.NEWLINE {
		p.nextToken()
	}

	return stmt
}

// parsePathExpression parses path expressions like my.data.value and my.data.@time
func (p *Parser) parsePathExpression() *PathExpression {
	expr := &PathExpression{
		Token: p.currentToken,
		Parts: []string{p.currentToken.Literal},
	}

	for p.peekToken.Type == lexer.DOT {
		p.nextToken() // consume current
		p.nextToken() // consume DOT

		// Accept both IDENT and TIME tokens (for meta attributes like @time)
		if p.currentToken.Type != lexer.IDENT && p.currentToken.Type != lexer.TIME {
			return expr
		}

		expr.Parts = append(expr.Parts, p.currentToken.Literal)
	}

	return expr
}

// parseIfStatement parses if/else statements
func (p *Parser) parseIfStatement() *IfStatement {
	stmt := &IfStatement{Token: p.currentToken}

	if !p.expectPeek(lexer.IDENT) && !p.currentTokenIs(lexer.LPAREN) {
		p.nextToken()
	}

	stmt.Condition = p.parseExpression(LOWEST)

	if p.peekToken.Type != lexer.DEFINE {
		// Provide more helpful error for missing colon after if condition
		msg := fmt.Sprintf("missing colon (:) after if condition at line %d, column %d",
			p.peekToken.Line, p.peekToken.Column)
		p.errors = append(p.errors, msg)
		return nil
	}
	p.nextToken() // consume the DEFINE token

	// Check for single-line vs block statement
	if p.peekToken.Type == lexer.NEWLINE {
		// Multi-line format: skip NEWLINE and expect INDENT
		for p.peekToken.Type == lexer.NEWLINE {
			p.nextToken()
		}

		if !p.expectPeek(lexer.INDENT) {
			return nil
		}

		stmt.Consequence = p.parseBlockStatement()
	} else {
		// Single-line format: parse one statement directly
		p.nextToken() // Move to the statement after ':'
		singleStmt := p.parseStatement()
		if singleStmt != nil {
			stmt.Consequence = &BlockStatement{
				Token:      p.currentToken,
				Statements: []Statement{singleStmt},
			}
		}
	}

	// Check for else
	if p.peekToken.Type == lexer.ELSE {
		p.nextToken()

		if p.peekToken.Type == lexer.IF {
			p.nextToken()
			stmt.Alternative = p.parseIfStatement() // Elif
		} else {
			if !p.expectPeek(lexer.DEFINE) {
				return nil
			}

			// Skip NEWLINE tokens before INDENT
			for p.peekToken.Type == lexer.NEWLINE {
				p.nextToken()
			}

			if !p.expectPeek(lexer.INDENT) {
				return nil
			}
			stmt.Alternative = p.parseBlockStatement()
		}
	}

	return stmt
}

// parseWhileStatement parses while loops
func (p *Parser) parseWhileStatement() *WhileStatement {
	stmt := &WhileStatement{Token: p.currentToken}

	p.nextToken()
	stmt.Condition = p.parseExpression(LOWEST)

	if !p.expectPeek(lexer.DEFINE) {
		return nil
	}

	// Skip NEWLINE tokens before INDENT
	for p.peekToken.Type == lexer.NEWLINE {
		p.nextToken()
	}

	if !p.expectPeek(lexer.INDENT) {
		return nil
	}

	stmt.Body = p.parseBlockStatement()

	return stmt
}

// parseForStatement parses for loops
func (p *Parser) parseForStatement() *ForStatement {
	stmt := &ForStatement{Token: p.currentToken}

	if !p.expectPeek(lexer.IDENT) {
		return nil
	}

	// Parse first variable
	stmt.Variables = []string{p.currentToken.Literal}

	// Check for additional variables (comma-separated)
	for p.peekToken.Type == lexer.COMMA {
		p.nextToken() // consume comma
		if !p.expectPeek(lexer.IDENT) {
			return nil
		}
		stmt.Variables = append(stmt.Variables, p.currentToken.Literal)
	}

	if !p.expectPeek(lexer.IN) {
		return nil
	}

	p.nextToken()
	stmt.Iterable = p.parseExpression(LOWEST)

	if !p.expectPeek(lexer.DEFINE) {
		return nil
	}

	// Skip NEWLINE tokens before INDENT
	for p.peekToken.Type == lexer.NEWLINE {
		p.nextToken()
	}

	if !p.expectPeek(lexer.INDENT) {
		return nil
	}

	stmt.Body = p.parseBlockStatement()

	return stmt
}

// parseConsiderStatement parses consider/case statements
func (p *Parser) parseConsiderStatement() *ConsiderStatement {
	stmt := &ConsiderStatement{Token: p.currentToken}

	p.nextToken()
	stmt.Value = p.parseExpression(LOWEST)

	if !p.expectPeek(lexer.DEFINE) {
		return nil
	}

	if !p.expectPeek(lexer.INDENT) {
		return nil
	}

	// Parse cases within the block
	for p.currentToken.Type != lexer.DEDENT && p.currentToken.Type != lexer.EOF {
		if p.currentToken.Type == lexer.IDENT && p.currentToken.Literal == "case" {
			p.nextToken()
			caseValue := p.parseExpression(LOWEST)

			if !p.expectPeek(lexer.DEFINE) {
				return nil
			}

			if !p.expectPeek(lexer.INDENT) {
				return nil
			}

			caseBody := p.parseBlockStatement()

			stmt.Cases = append(stmt.Cases, &ConsiderCase{
				Token: p.currentToken,
				Value: caseValue,
				Body:  caseBody,
			})
		} else if p.currentToken.Type == lexer.IDENT && p.currentToken.Literal == "default" {
			p.nextToken()

			if !p.expectPeek(lexer.DEFINE) {
				return nil
			}

			if !p.expectPeek(lexer.INDENT) {
				return nil
			}

			stmt.Default = p.parseBlockStatement()
		} else {
			p.nextToken()
		}
	}

	return stmt
}

// parseReturnStatement parses return statements
func (p *Parser) parseReturnStatement() *ReturnStatement {
	stmt := &ReturnStatement{Token: p.currentToken}

	if p.peekToken.Type == lexer.NEWLINE || p.peekToken.Type == lexer.EOF {
		return stmt
	}

	p.nextToken()
	stmt.ReturnValue = p.parseExpression(LOWEST)

	if p.peekToken.Type == lexer.NEWLINE {
		p.nextToken()
	}

	return stmt
}

// parsePassStatement parses pass statements
func (p *Parser) parsePassStatement() *PassStatement {
	stmt := &PassStatement{Token: p.currentToken}

	// Consume optional semicolon or newline
	if p.peekToken.Type == lexer.SEMICOLON {
		p.nextToken()
	} else if p.peekToken.Type == lexer.NEWLINE {
		p.nextToken()
	}

	return stmt
}

// parseWatchStatement parses watch statements
func (p *Parser) parseWatchStatement() Statement {
	stmt := &WatchStatement{Token: p.currentToken}

	// Check if we're parsing from WATCH token (old style) or from identifier (new style)
	if p.currentToken.Type == lexer.WATCH {
		// Old style: just a standalone watch token
		// For now, return a simple expression statement to maintain compatibility
		return p.parseExpressionStatement()
	}

	// New style: name: watch(...) syntax
	// Parse the name (identifier before the colon)
	stmt.Name = p.currentToken.Literal

	// Expect DEFINE ":"
	if !p.expectPeek(lexer.DEFINE) {
		return nil
	}

	// Expect WATCH keyword
	if !p.expectPeek(lexer.WATCH) {
		return nil
	}

	stmt.Token = p.currentToken // Set token to WATCH

	// Check if this is an append watcher
	if p.peekToken.Type == lexer.APPEND {
		p.nextToken() // consume APPEND token
		stmt.IsAppend = true

		// Expect opening parenthesis
		if !p.expectPeek(lexer.LPAREN) {
			return nil
		}

		// Parse the path expression (including any bracket filters)
		p.nextToken()
		pathExpr := p.parseExpression(LOWEST)

		// Extract filters from BracketFilterExpression if present
		if bracketExpr, ok := pathExpr.(*BracketFilterExpression); ok {
			stmt.Paths = []Expression{bracketExpr.Left}
			stmt.Filters = bracketExpr.Filters
		} else {
			stmt.Paths = []Expression{pathExpr}
		}

		// Expect closing parenthesis
		if !p.expectPeek(lexer.RPAREN) {
			return nil
		}

		// Expect AS keyword
		if !p.expectPeek(lexer.AS) {
			return nil
		}

		// Expect binding name
		if !p.expectPeek(lexer.IDENT) {
			return nil
		}

		stmt.BindingName = p.currentToken.Literal

	} else {
		// Regular watcher: watch(path)
		stmt.IsAppend = false

		// Expect opening parenthesis
		if !p.expectPeek(lexer.LPAREN) {
			return nil
		}

		// Parse comma-separated path expressions
		p.nextToken()
		stmt.Paths = []Expression{}
		stmt.Paths = append(stmt.Paths, p.parseExpression(LOWEST))

		// Handle additional comma-separated paths
		for p.peekToken.Type == lexer.COMMA {
			p.nextToken() // consume COMMA
			p.nextToken() // move to next expression
			stmt.Paths = append(stmt.Paths, p.parseExpression(LOWEST))
		}

		// Expect closing parenthesis
		if !p.expectPeek(lexer.RPAREN) {
			return nil
		}
	}

	// Expect DEFINE ":" for the body
	if !p.expectPeek(lexer.DEFINE) {
		return nil
	}

	// Skip NEWLINE tokens before INDENT
	for p.peekToken.Type == lexer.NEWLINE {
		p.nextToken()
	}

	// Expect INDENT for the body
	if !p.expectPeek(lexer.INDENT) {
		return nil
	}

	// Parse the watcher body
	stmt.Body = p.parseBlockStatement()

	return stmt
}

// parseInstantiationStatement parses object instantiation with heritability
func (p *Parser) parseInstantiationStatement() *InstantiationStatement {
	stmt := &InstantiationStatement{Token: p.currentToken}

	if !p.expectPeek(lexer.IDENT) {
		return nil
	}

	stmt.Type = p.currentToken.Literal

	// Check for heritability modifier like :(copy)
	if p.peekToken.Type == lexer.DEFINE {
		p.nextToken() // consume DEFINE ":"
		// Check for modifier tokens (COPY, LINK, RESET, EXCLUDE)
		if p.peekToken.Type == lexer.COPY ||
			p.peekToken.Type == lexer.LINK ||
			p.peekToken.Type == lexer.RESET ||
			p.peekToken.Type == lexer.EXCLUDE {
			p.nextToken() // consume the modifier token
			// Extract just the modifier name from the token literal (remove parentheses)
			modifierLiteral := p.currentToken.Literal
			if len(modifierLiteral) > 2 && modifierLiteral[0] == '(' && modifierLiteral[len(modifierLiteral)-1] == ')' {
				modifierValue := modifierLiteral[1 : len(modifierLiteral)-1]
				stmt.Modifier = &modifierValue
			}
		}
	}

	// Parse mixed sources: template names and inline records
	stmt.Sources = []InstantiationSource{}

	for p.peekToken.Type == lexer.IDENT || p.peekToken.Type == lexer.LBRACE {
		p.nextToken() // consume the source token

		var source InstantiationSource

		if p.currentToken.Type == lexer.IDENT {
			// Template name source
			templateName := p.currentToken.Literal
			source.TemplateName = &templateName
		} else if p.currentToken.Type == lexer.LBRACE {
			// Inline record source
			record := p.parseRecordLiteral().(*RecordExpression)
			source.InlineRecord = record
		}

		stmt.Sources = append(stmt.Sources, source)

		// Check for comma to continue with more sources
		if p.peekToken.Type != lexer.COMMA {
			break
		}
		p.nextToken() // consume COMMA
	}

	// Check for optional property initialization
	if p.peekToken.Type == lexer.LBRACE {
		p.nextToken()
		stmt.Properties = p.parseRecordLiteral().(*RecordExpression)
	}

	return stmt
}

// parseProcedureStatement parses function definitions
func (p *Parser) parseProcedureStatement() *ProcedureStatement {
	stmt := &ProcedureStatement{
		Token: p.currentToken,
		Name:  p.currentToken.Literal,
	}

	if !p.expectPeek(lexer.LPAREN) {
		return nil
	}

	// Parse parameters
	if p.peekToken.Type != lexer.RPAREN {
		p.nextToken()

		stmt.Parameters = append(stmt.Parameters, p.currentToken.Literal)

		for p.peekToken.Type == lexer.COMMA {
			p.nextToken()
			p.nextToken()
			stmt.Parameters = append(stmt.Parameters, p.currentToken.Literal)
		}
	}

	if !p.expectPeek(lexer.RPAREN) {
		return nil
	}

	if !p.expectPeek(lexer.DEFINE) {
		return nil
	}

	// Check for same-line vs multi-line definition
	if p.peekToken.Type == lexer.NEWLINE {
		// Multi-line form: skip NEWLINE tokens before INDENT
		for p.peekToken.Type == lexer.NEWLINE {
			p.nextToken()
		}

		if !p.expectPeek(lexer.INDENT) {
			return nil
		}

		stmt.Body = p.parseBlockStatement()
	} else {
		// Same-line form: parse statements until SEMICOLON or EOF
		stmt.Body = p.parseSameLineBlock()
	}

	return stmt
}

// parseExpressionStatement parses standalone expressions
func (p *Parser) parseExpressionStatement() *ExpressionStatement {
	stmt := &ExpressionStatement{Token: p.currentToken}
	stmt.Expression = p.parseExpression(LOWEST)

	if p.peekToken.Type == lexer.NEWLINE {
		p.nextToken()
	}

	return stmt
}

// parseBlockStatement parses indented blocks of statements
func (p *Parser) parseBlockStatement() *BlockStatement {
	block := &BlockStatement{
		Token:      p.currentToken, // INDENT token
		Statements: []Statement{},
	}

	p.nextToken() // consume INDENT

	for p.currentToken.Type != lexer.DEDENT && p.currentToken.Type != lexer.EOF {
		p.skipComments()

		if p.currentToken.Type == lexer.NEWLINE {
			p.nextToken()
			continue
		}

		stmt := p.parseStatement()
		if stmt != nil {
			block.Statements = append(block.Statements, stmt)
		}

		p.nextToken()
	}

	// Current token should be DEDENT
	return block
}

// parseSameLineBlock parses statements on the same line, separated by semicolons
func (p *Parser) parseSameLineBlock() *BlockStatement {
	block := &BlockStatement{
		Token:      p.currentToken,
		Statements: []Statement{},
	}

	// Parse first statement (already at the correct position)
	p.nextToken() // move to the first token of the statement

	for p.currentToken.Type != lexer.EOF &&
		p.currentToken.Type != lexer.NEWLINE &&
		p.currentToken.Type != lexer.DEDENT {

		p.skipComments()

		stmt := p.parseStatement()
		if stmt != nil {
			block.Statements = append(block.Statements, stmt)
		}

		// Check for semicolon separator
		if p.peekToken.Type == lexer.SEMICOLON {
			p.nextToken() // consume current token
			p.nextToken() // consume semicolon, move to next statement
		} else {
			// No semicolon, should be end of line or block
			break
		}
	}

	return block
}

// ============================================================================
// EXPRESSION PARSING (Pratt parser)
// ============================================================================

// parseExpression is the main entry point for expression parsing using Pratt parsing
func (p *Parser) parseExpression(precedence int) Expression {
	if !p.incrementRecursionDepth() {
		return nil
	}
	defer p.decrementRecursionDepth()

	if !p.incrementNodeCount() {
		return nil
	}

	prefix := p.prefixParseFns[p.currentToken.Type]
	if prefix == nil {
		p.noPrefixParseFnError(p.currentToken.Type)
		return nil
	}

	leftExp := prefix()

	for p.peekToken.Type != lexer.NEWLINE && precedence < p.peekPrecedence() {
		infix := p.infixParseFns[p.peekToken.Type]
		if infix == nil {
			return leftExp
		}

		p.nextToken()
		leftExp = infix(leftExp)
	}

	return leftExp
}

// ============================================================================
// PREFIX PARSE FUNCTIONS
// ============================================================================

// parseIdentifier parses simple identifiers
func (p *Parser) parseIdentifier() Expression {
	return &PathExpression{
		Token: p.currentToken,
		Parts: []string{p.currentToken.Literal},
	}
}

// parseMyExpression parses 'my' path expressions including meta attributes
func (p *Parser) parseMyExpression() Expression {
	expr := &PathExpression{
		Token: p.currentToken,
		Parts: []string{"my"},
	}

	for p.peekToken.Type == lexer.DOT {
		p.nextToken() // consume current
		p.nextToken() // consume DOT

		// Accept both IDENT and TIME tokens (for meta attributes like @time)
		if p.currentToken.Type != lexer.IDENT && p.currentToken.Type != lexer.TIME {
			break
		}

		expr.Parts = append(expr.Parts, p.currentToken.Literal)
	}

	return expr
}

// parseWorldExpression parses 'world' path expressions
func (p *Parser) parseWorldExpression() Expression {
	expr := &PathExpression{
		Token: p.currentToken,
		Parts: []string{"world"},
	}

	for p.peekToken.Type == lexer.DOT {
		p.nextToken() // consume current
		p.nextToken() // consume DOT

		if p.currentToken.Type != lexer.IDENT {
			break
		}

		expr.Parts = append(expr.Parts, p.currentToken.Literal)
	}

	return expr
}

// parseStringLiteral parses string literals
func (p *Parser) parseStringLiteral() Expression {
	return &LiteralExpression{
		Token: p.currentToken,
		Value: p.currentToken.Literal,
	}
}

// parseNumberLiteral parses numeric literals
func (p *Parser) parseNumberLiteral() Expression {
	lit := &LiteralExpression{Token: p.currentToken}

	// Try to parse as integer first
	if value, err := strconv.ParseInt(p.currentToken.Literal, 0, 64); err == nil {
		lit.Value = value
	} else {
		// Try to parse as float
		if value, err := strconv.ParseFloat(p.currentToken.Literal, 64); err == nil {
			lit.Value = value
		} else {
			msg := fmt.Sprintf("could not parse %q as number", p.currentToken.Literal)
			p.errors = append(p.errors, msg)
			return nil
		}
	}

	return lit
}

// parseTimeLiteral parses time literals
func (p *Parser) parseTimeLiteral() Expression {
	// Handle bare @ as instance timestamp reference
	if p.currentToken.Literal == "@" || strings.TrimSpace(p.currentToken.Literal) == "@" {
		// Create a path expression for instance timestamp reference
		return &PathExpression{
			Token: p.currentToken,
			Parts: []string{"@"}, // Bare @ refers to instance timestamp
		}
	}

	// Handle meta attributes like @agent, @time, etc.
	if strings.HasPrefix(p.currentToken.Literal, "@") && !strings.Contains(p.currentToken.Literal, "-") {
		timeStr := strings.TrimPrefix(p.currentToken.Literal, "@")
		// If it's a letter after @, treat as meta attribute
		if len(timeStr) > 0 && ((timeStr[0] >= 'a' && timeStr[0] <= 'z') || (timeStr[0] >= 'A' && timeStr[0] <= 'Z')) {
			return &PathExpression{
				Token: p.currentToken,
				Parts: []string{p.currentToken.Literal}, // @agent, @time, etc.
			}
		}
	}

	lit := &LiteralExpression{Token: p.currentToken}

	// Parse time literal (removing @ prefix)
	timeStr := strings.TrimPrefix(p.currentToken.Literal, "@")

	// Try different time formats
	formats := []string{
		"2006-01-02",
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05",
		"2006-01-02T15:04:05Z07:00",
	}

	var parsedTime time.Time
	var err error

	for _, format := range formats {
		if parsedTime, err = time.Parse(format, timeStr); err == nil {
			lit.Value = parsedTime
			return lit
		}
	}

	msg := fmt.Sprintf("could not parse %q as time", p.currentToken.Literal)
	p.errors = append(p.errors, msg)
	return nil
}

// parseMoneyLiteral parses money literals
func (p *Parser) parseMoneyLiteral() Expression {
	return &LiteralExpression{
		Token: p.currentToken,
		Value: p.currentToken.Literal, // Store as string for now
	}
}

// parseBooleanLiteral parses true/false literals
func (p *Parser) parseBooleanLiteral() Expression {
	return &LiteralExpression{
		Token: p.currentToken,
		Value: p.currentTokenIs(lexer.TRUE),
	}
}

// parseNothingLiteral parses Nothing literal
func (p *Parser) parseNothingLiteral() Expression {
	return &LiteralExpression{
		Token: p.currentToken,
		Value: nil, // Nothing represented as nil
	}
}

// parseUnknownLiteral parses Unknown literal
func (p *Parser) parseUnknownLiteral() Expression {
	return &LiteralExpression{
		Token: p.currentToken,
		Value: "Unknown",
	}
}

// parseQueuedLiteral parses Queued literal
func (p *Parser) parseQueuedLiteral() Expression {
	return &LiteralExpression{
		Token: p.currentToken,
		Value: p.currentToken.Literal, // Use the actual token literal (e.g., "?tomorrow")
	}
}

// parseAnythingLiteral parses Anything literal
func (p *Parser) parseAnythingLiteral() Expression {
	return &LiteralExpression{
		Token: p.currentToken,
		Value: "Anything",
	}
}

// parsePiLiteral parses pi literal
func (p *Parser) parsePiLiteral() Expression {
	return &LiteralExpression{
		Token: p.currentToken,
		Value: 3.141592653589793,
	}
}

// parseEulerLiteral parses euler literal
func (p *Parser) parseEulerLiteral() Expression {
	return &LiteralExpression{
		Token: p.currentToken,
		Value: 2.718281828459045,
	}
}

// parseEmptyLiteral parses empty literal
func (p *Parser) parseEmptyLiteral() Expression {
	return &LiteralExpression{
		Token: p.currentToken,
		Value: "",
	}
}

// parseQuoteLiteral parses quote literal
func (p *Parser) parseQuoteLiteral() Expression {
	return &LiteralExpression{
		Token: p.currentToken,
		Value: "\"",
	}
}

// parseTabLiteral parses tab literal
func (p *Parser) parseTabLiteral() Expression {
	return &LiteralExpression{
		Token: p.currentToken,
		Value: "\t",
	}
}

// parseNewlineLiteral parses newline literal
func (p *Parser) parseNewlineLiteral() Expression {
	return &LiteralExpression{
		Token: p.currentToken,
		Value: "\n",
	}
}

// parseUnaryExpression parses unary expressions like -x, +x, not x
func (p *Parser) parseUnaryExpression() Expression {
	expression := &UnaryExpression{
		Token:    p.currentToken,
		Operator: p.currentToken.Literal,
	}

	p.nextToken()

	expression.Right = p.parseExpression(UNARY)

	return expression
}

// parseQuietlyExpression parses quietly modifier expressions
func (p *Parser) parseQuietlyExpression() Expression {
	quietlyToken := p.currentToken

	// Check if there's a following expression (like a string literal)
	if p.peekToken.Type == lexer.TEXT || p.peekToken.Type == lexer.NUMBER ||
		p.peekToken.Type == lexer.IDENT || p.peekToken.Type == lexer.MY {

		p.nextToken() // consume the quietly token
		rightExpr := p.parseExpression(LOWEST)

		// Return a binary expression representing (quietly) followed by the expression
		return &BinaryExpression{
			Token:    quietlyToken,
			Left:     &LiteralExpression{Token: quietlyToken, Value: quietlyToken.Literal},
			Operator: " ", // Space operator for concatenation
			Right:    rightExpr,
		}
	}

	// If no following expression, just return the quietly modifier as is
	return &LiteralExpression{
		Token: quietlyToken,
		Value: "quietly",
	}
}

// parseGroupedExpression parses expressions in parentheses
func (p *Parser) parseGroupedExpression() Expression {
	p.nextToken()

	exp := p.parseExpression(LOWEST)

	if !p.expectPeek(lexer.RPAREN) {
		return nil
	}

	return exp
}

// parseRecordLiteral parses record literals like {x: 1, y: 2} with heritability support
func (p *Parser) parseRecordLiteral() Expression {
	record := &RecordExpression{
		Token:  p.currentToken,
		Fields: []RecordField{},
		Pairs:  make(map[Expression]Expression), // Keep for compatibility
	}

	if p.peekToken.Type == lexer.RBRACE {
		p.nextToken()
		return record
	}

	p.nextToken()

	// Parse the first field
	field := p.parseRecordField()
	if field != nil {
		record.Fields = append(record.Fields, *field)
		// Also add to legacy Pairs for compatibility
		record.Pairs[field.Key] = field.Value
	}

	for p.peekToken.Type == lexer.COMMA {
		p.nextToken()
		p.nextToken()

		field = p.parseRecordField()
		if field != nil {
			record.Fields = append(record.Fields, *field)
			// Also add to legacy Pairs for compatibility
			record.Pairs[field.Key] = field.Value
		}
	}

	if !p.expectPeek(lexer.RBRACE) {
		return nil
	}

	return record
}

// parseRecordField parses a single record field with optional heritability modifier
func (p *Parser) parseRecordField() *RecordField {
	field := &RecordField{}

	// Parse the key (identifier, no quotes needed)
	field.Key = p.parseExpression(LOWEST)

	if !p.expectPeek(lexer.DEFINE) {
		return nil
	}

	// Check for heritability modifier after the colon
	if p.peekToken.Type == lexer.COPY ||
		p.peekToken.Type == lexer.LINK ||
		p.peekToken.Type == lexer.RESET ||
		p.peekToken.Type == lexer.EXCLUDE {

		p.nextToken() // consume the modifier token

		// Extract modifier name (remove parentheses)
		modifierLiteral := p.currentToken.Literal
		if len(modifierLiteral) > 2 && modifierLiteral[0] == '(' && modifierLiteral[len(modifierLiteral)-1] == ')' {
			modifierValue := modifierLiteral[1 : len(modifierLiteral)-1]
			field.Modifier = &modifierValue

			// For reset modifier, parse the reset expression
			if strings.HasPrefix(modifierValue, "reset ") {
				resetStr := "reset"
				field.Modifier = &resetStr

				// Extract and parse the reset expression
				resetExprStr := strings.TrimSpace(modifierValue[6:]) // Remove "reset "
				if resetExprStr != "" {
					field.ResetExpr = p.parseResetExpression(resetExprStr)
				}
			} else if modifierValue == "reset" {
				// Simple reset without expression - use default value
				resetStr := "reset"
				field.Modifier = &resetStr
			}
		}

		// Parse the value after the modifier
		p.nextToken()
		field.Value = p.parseExpression(LOWEST)
	} else {
		// No modifier - use default behavior (copy semantics, but don't store explicit modifier)
		field.Modifier = nil

		// Parse the value directly
		p.nextToken()
		field.Value = p.parseExpression(LOWEST)
	}

	return field
}

// parseResetExpression parses expressions within reset modifiers
// Examples: "random(100, 999)", "now()", "42", "uuid()"
func (p *Parser) parseResetExpression(exprStr string) Expression {
	// Create a temporary lexer for the expression string
	tempLexer := lexer.New(exprStr)
	tempParser := New(tempLexer)

	// Parse the expression
	expr := tempParser.parseExpression(LOWEST)

	// If parsing failed, return as literal
	if expr == nil || len(tempParser.Errors()) > 0 {
		return &LiteralExpression{
			Token: lexer.Token{Type: lexer.TEXT, Literal: exprStr},
			Value: exprStr,
		}
	}

	return expr
}

// parseListLiteral parses list literals like ["a", "b"]
func (p *Parser) parseListLiteral() Expression {
	list := &ListExpression{
		Token:    p.currentToken,
		Elements: []Expression{},
	}

	if p.peekToken.Type == lexer.RBRACKET {
		p.nextToken()
		return list
	}

	p.nextToken()

	list.Elements = append(list.Elements, p.parseExpression(LOWEST))

	for p.peekToken.Type == lexer.COMMA {
		p.nextToken()
		p.nextToken()
		list.Elements = append(list.Elements, p.parseExpression(LOWEST))
	}

	if !p.expectPeek(lexer.RBRACKET) {
		return nil
	}

	return list
}

// ============================================================================
// INFIX PARSE FUNCTIONS
// ============================================================================

// parseBinaryExpression parses binary expressions with correct precedence
func (p *Parser) parseBinaryExpression(left Expression) Expression {
	expression := &BinaryExpression{
		Token:    p.currentToken,
		Left:     left,
		Operator: p.currentToken.Literal,
	}

	precedence := p.currentPrecedence()

	// Handle right-associativity for exponentiation
	if p.currentToken.Type == lexer.POWER {
		p.nextToken()
		expression.Right = p.parseExpression(precedence - 1) // Right-associative
	} else {
		p.nextToken()
		expression.Right = p.parseExpression(precedence) // Left-associative
	}

	return expression
}

// parseCollectionOperation parses collection operations like x..count, x..remove(), x..combine()
func (p *Parser) parseCollectionOperation(left Expression) Expression {
	// Check if this is a collection operation (..count, ..remove, ..combine)
	if p.peekToken.Type == lexer.IDENT {
		nextToken := p.peekToken.Literal
		if nextToken == "count" || nextToken == "remove" || nextToken == "combine" {
			// This is a collection operation
			p.nextToken() // consume RANGE token (..)
			methodName := p.currentToken.Literal

			expr := &CollectionOperationExpression{
				Token:      p.currentToken,
				Object:     left,
				Method:     methodName,
				Arguments:  []Expression{},
			}

			// Check if this method has arguments (remove, combine do; count doesn't)
			if p.peekToken.Type == lexer.LPAREN {
				p.nextToken() // consume method name, move to LPAREN
				expr.Arguments = p.parseExpressionList(lexer.RPAREN)
			}

			return expr
		}
	}

	// Not a collection operation, fall back to binary expression
	return p.parseBinaryExpression(left)
}

// parseCallExpression parses function calls and method chains
func (p *Parser) parseCallExpression(left Expression) Expression {
	if p.currentToken.Type == lexer.DOT {
		// Handle dot notation for method calls or property access
		// Accept both IDENT and TIME tokens (for meta attributes like @time)
		if p.peekToken.Type == lexer.IDENT {
			p.nextToken()
		} else if p.peekToken.Type == lexer.TIME {
			p.nextToken()
		} else {
			p.peekError(lexer.IDENT)
			return nil
		}

		// Create a new path expression or extend existing one
		if pathExpr, ok := left.(*PathExpression); ok {
			pathExpr.Parts = append(pathExpr.Parts, p.currentToken.Literal)
			return pathExpr
		} else {
			// Convert to a call expression for method chaining
			call := &CallExpression{
				Token:    p.currentToken,
				Function: left,
			}

			// If followed by parentheses, parse arguments
			if p.peekToken.Type == lexer.LPAREN {
				p.nextToken() // consume LPAREN
				call.Arguments = p.parseExpressionList(lexer.RPAREN)
			}

			return call
		}
	} else if p.currentToken.Type == lexer.LPAREN {
		// Handle function calls
		call := &CallExpression{
			Token:     p.currentToken,
			Function:  left,
			Arguments: p.parseExpressionList(lexer.RPAREN),
		}
		return call
	}

	return left
}

// parseBracketFilterExpression parses bracket filter expressions like person[name = "bob"]
func (p *Parser) parseBracketFilterExpression(left Expression) Expression {
	exp := &BracketFilterExpression{
		Token: p.currentToken,
		Left:  left,
	}

	p.nextToken()

	// Parse filter expressions inside brackets - in this context, ASSIGN should be treated as equality
	exp.Filters = append(exp.Filters, p.parseFilterExpression())

	for p.peekToken.Type == lexer.COMMA {
		p.nextToken()
		p.nextToken()
		exp.Filters = append(exp.Filters, p.parseFilterExpression())
	}

	if p.peekToken.Type != lexer.RBRACKET {
		// Provide more helpful error for missing closing bracket
		msg := fmt.Sprintf("missing closing bracket (]) at line %d, column %d",
			p.peekToken.Line, p.peekToken.Column)
		p.errors = append(p.errors, msg)
		return nil
	}
	p.nextToken() // consume the RBRACKET token

	return exp
}

// parseFilterExpression parses expressions inside bracket filters, treating ASSIGN as equality
func (p *Parser) parseFilterExpression() Expression {
	// Parse left side of the filter expression
	left := p.parseExpression(LOWEST)

	// Handle ASSIGN tokens as equality in filter context
	if p.peekToken.Type == lexer.ASSIGN {
		p.nextToken() // move to ASSIGN token

		// Create a binary expression with "=" as equality
		exp := &BinaryExpression{
			Token:    p.currentToken,
			Left:     left,
			Operator: "=", // Treat as equality operator in filter context
		}

		precedence := EQUALS // Use equality precedence
		p.nextToken()
		exp.Right = p.parseExpression(precedence)

		return exp
	}

	// For other operators (>, <, etc.), use normal parsing
	return left
}

// parseProjectionExpression parses projection expressions like path{ name, age, job }
func (p *Parser) parseProjectionExpression(left Expression) Expression {
	exp := &ProjectionExpression{
		Token: p.currentToken, // LBRACE token
		Left:  left,
		Fields: []string{},
	}

	// Advance past the opening brace
	p.nextToken()

	// Parse the first field name (must be an identifier)
	if !p.currentTokenIs(lexer.IDENT) {
		p.errors = append(p.errors, fmt.Sprintf("expected identifier in projection, got %s at line %d, column %d",
			p.currentToken.Type, p.currentToken.Line, p.currentToken.Column))
		return nil
	}

	exp.Fields = append(exp.Fields, p.currentToken.Literal)

	// Parse remaining comma-separated field names
	for p.peekToken.Type == lexer.COMMA {
		p.nextToken() // consume current identifier
		p.nextToken() // consume comma

		if !p.currentTokenIs(lexer.IDENT) {
			p.errors = append(p.errors, fmt.Sprintf("expected identifier in projection, got %s at line %d, column %d",
				p.currentToken.Type, p.currentToken.Line, p.currentToken.Column))
			return nil
		}

		exp.Fields = append(exp.Fields, p.currentToken.Literal)
	}

	// Expect closing brace
	if !p.expectPeek(lexer.RBRACE) {
		return nil
	}

	return exp
}

// parseExpressionList parses a list of expressions separated by commas
func (p *Parser) parseExpressionList(end lexer.TokenType) []Expression {
	args := []Expression{}

	if p.peekToken.Type == end {
		p.nextToken()
		return args
	}

	p.nextToken()
	args = append(args, p.parseExpression(LOWEST))

	for p.peekToken.Type == lexer.COMMA {
		p.nextToken()
		p.nextToken()
		args = append(args, p.parseExpression(LOWEST))
	}

	if !p.expectPeek(end) {
		return nil
	}

	return args
}

// parseDefinitionExpression parses definition syntax that starts with DEFINE (colon)
func (p *Parser) parseDefinitionExpression() Expression {
	p.errors = append(p.errors, fmt.Sprintf("unexpected definition at line %d, column %d - definitions must start with a name", p.currentToken.Line, p.currentToken.Column))
	return nil
}

// parseDefinitionStatement parses definition statements like "my.func: procedure(x): return x"
func (p *Parser) parseDefinitionStatement() Statement {
	ds := &DefinitionStatement{Token: p.currentToken}

	// Parse the name (left-hand side path like my.func)
	pathExpr := p.parsePathExpression()
	ds.Name = pathExpr

	if !p.expectPeek(lexer.DEFINE) {
		return nil
	}

	// Parse the value (right-hand side like procedure(x): return x)
	p.nextToken()
	value := p.parseExpression(LOWEST)
	if value == nil {
		return nil
	}
	ds.Value = value

	return ds
}

// parseWatchExpression parses watch expressions like "watch(path)" or "watch append(path) as var"
func (p *Parser) parseWatchExpression() Expression {
	we := &WatchExpression{Token: p.currentToken}

	// Check for append modifier
	if p.peekToken.Type == lexer.APPEND {
		p.nextToken()
		we.IsAppend = true
	}

	if !p.expectPeek(lexer.LPAREN) {
		return nil
	}

	// Parse first path
	p.nextToken()
	firstPath := p.parseExpression(LOWEST)
	if firstPath == nil {
		return nil
	}
	we.Paths = []Expression{firstPath}

	// Parse additional paths (comma-separated)
	for p.peekToken.Type == lexer.COMMA {
		p.nextToken() // consume comma
		p.nextToken() // move to next expression
		path := p.parseExpression(LOWEST)
		if path == nil {
			return nil
		}
		we.Paths = append(we.Paths, path)
	}

	if !p.expectPeek(lexer.RPAREN) {
		return nil
	}

	// Check for "as" alias
	if p.peekToken.Type == lexer.AS {
		p.nextToken() // consume "as"
		if !p.expectPeek(lexer.IDENT) {
			return nil
		}
		we.Alias = p.currentToken.Literal
	}

	// Check for optional body with ":"
	if p.peekToken.Type == lexer.DEFINE {
		p.nextToken() // consume ":"

		// Check if this is a single-line or multi-line body
		if p.peekToken.Type == lexer.NEWLINE {
			// Multi-line body - skip NEWLINE and expect INDENT
			for p.peekToken.Type == lexer.NEWLINE {
				p.nextToken()
			}

			if !p.expectPeek(lexer.INDENT) {
				return nil
			}

			we.Body = p.parseBlockStatement()
		} else {
			// Single-line body - parse single statement
			p.nextToken()
			stmt := p.parseStatement()
			if stmt == nil {
				return nil
			}

			// Create a block with the single statement
			we.Body = &BlockStatement{
				Token:      p.currentToken,
				Statements: []Statement{stmt},
			}
		}
	}

	return we
}

// parseScopeRelativeExpression parses expressions starting with DOT for scope-relative references
func (p *Parser) parseScopeRelativeExpression() Expression {
	if p.peekToken.Type != lexer.IDENT {
		p.errors = append(p.errors, fmt.Sprintf("expected identifier after '.' at line %d, column %d", p.peekToken.Line, p.peekToken.Column))
		return nil
	}

	// Build a path expression starting with "."
	pe := &PathExpression{Token: p.currentToken}
	pe.Parts = []string{""}  // Empty string represents the leading dot

	p.nextToken() // move to identifier
	pe.Parts = append(pe.Parts, p.currentToken.Literal)

	// Continue parsing path segments
	for p.peekToken.Type == lexer.DOT && p.peekToken.Literal != "" {
		p.nextToken() // consume dot
		if p.peekToken.Type != lexer.IDENT {
			break
		}
		p.nextToken() // move to identifier
		pe.Parts = append(pe.Parts, p.currentToken.Literal)
	}

	return pe
}

// parseCatchStatement parses catch-else exception handling statements
func (p *Parser) parseCatchStatement() Statement {
	cs := &CatchStatement{Token: p.currentToken}

	if !p.expectPeek(lexer.DEFINE) {
		return nil
	}

	// Skip NEWLINE tokens before INDENT
	for p.peekToken.Type == lexer.NEWLINE {
		p.nextToken()
	}

	if !p.expectPeek(lexer.INDENT) {
		return nil
	}

	cs.TryBody = p.parseBlockStatement()

	// Parse else clauses
	for p.peekToken.Type == lexer.ELSE {
		p.nextToken() // consume ELSE

		clause := &ElseClause{Token: p.currentToken}

		// Check for error type (can be IDENT or special keywords like UNKNOWN, QUEUED)
		if p.peekToken.Type == lexer.IDENT || p.peekToken.Type == lexer.UNKNOWN || p.peekToken.Type == lexer.QUEUED {
			p.nextToken()
			clause.ErrorType = p.currentToken.Literal
		}

		if !p.expectPeek(lexer.DEFINE) {
			return cs
		}

		// Skip NEWLINE tokens before INDENT
		for p.peekToken.Type == lexer.NEWLINE {
			p.nextToken()
		}

		if p.peekToken.Type == lexer.INDENT {
			p.nextToken()
			clause.Body = p.parseBlockStatement()
		}

		cs.ElseClauses = append(cs.ElseClauses, clause)
	}

	return cs
}

// parseProcedureExpression parses procedure expressions like "procedure(x, y): body"
func (p *Parser) parseProcedureExpression() Expression {
	pe := &ProcedureExpression{Token: p.currentToken}

	if !p.expectPeek(lexer.LPAREN) {
		return nil
	}

	// Parse parameters
	if p.peekToken.Type != lexer.RPAREN {
		p.nextToken()
		pe.Parameters = []string{p.currentToken.Literal}

		for p.peekToken.Type == lexer.COMMA {
			p.nextToken() // consume comma
			if !p.expectPeek(lexer.IDENT) {
				return nil
			}
			pe.Parameters = append(pe.Parameters, p.currentToken.Literal)
		}
	}

	if !p.expectPeek(lexer.RPAREN) {
		return nil
	}

	if !p.expectPeek(lexer.DEFINE) {
		return nil
	}

	// Check if this is a single-line procedure or multi-line
	if p.peekToken.Type == lexer.NEWLINE {
		// Multi-line procedure - skip NEWLINE and expect INDENT
		for p.peekToken.Type == lexer.NEWLINE {
			p.nextToken()
		}

		if !p.expectPeek(lexer.INDENT) {
			return nil
		}

		pe.Body = p.parseBlockStatement()
	} else {
		// Single-line procedure - parse single statement
		p.nextToken()

		// Check for empty procedure body (EOF immediately after :)
		if p.currentToken.Type == lexer.EOF {
			msg := fmt.Sprintf("empty procedure body at line %d, column %d. Procedures must have a body",
				p.currentToken.Line, p.currentToken.Column)
			p.errors = append(p.errors, msg)
			return nil
		}

		stmt := p.parseStatement()
		if stmt == nil {
			return nil
		}

		// Create a block with the single statement
		pe.Body = &BlockStatement{
			Token:      p.currentToken,
			Statements: []Statement{stmt},
		}
	}

	return pe
}


// parseScopeRelativeAssignment parses assignments to scope-relative references like ".local.variable = 5"
func (p *Parser) parseScopeRelativeAssignment() Statement {
	stmt := &AssignmentStatement{Token: p.currentToken}

	// Parse the scope-relative path (.local.variable)
	pathExpr := p.parseScopeRelativeExpression()
	if pathExpr == nil {
		return nil
	}

	// Type assert to PathExpression
	if pathExp, ok := pathExpr.(*PathExpression); ok {
		stmt.Name = pathExp
	} else {
		p.errors = append(p.errors, "expected path expression for scope-relative assignment")
		return nil
	}

	// Expect assignment operator
	if !p.expectPeek(lexer.ASSIGN) {
		return nil
	}

	// Parse the value
	p.nextToken()
	value := p.parseExpression(LOWEST)
	if value == nil {
		return nil
	}
	stmt.Value = value

	return stmt
}
