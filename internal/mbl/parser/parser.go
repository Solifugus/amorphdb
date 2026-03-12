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
		lexer.RANGE:    RANGE,
		lexer.DOT:      CALL,
		lexer.LBRACKET: CALL,
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
	p.registerPrefix(lexer.LPAREN, p.parseGroupedExpression)
	p.registerPrefix(lexer.LBRACE, p.parseRecordLiteral)
	p.registerPrefix(lexer.LBRACKET, p.parseListLiteral)

	// Initialize infix parse functions
	p.infixParseFns = make(map[lexer.TokenType]infixParseFn)
	p.registerInfix(lexer.OR, p.parseBinaryExpression)
	p.registerInfix(lexer.AND, p.parseBinaryExpression)
	p.registerInfix(lexer.EQUAL, p.parseBinaryExpression)
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
	p.registerInfix(lexer.RANGE, p.parseBinaryExpression)
	p.registerInfix(lexer.DOT, p.parseCallExpression)
	p.registerInfix(lexer.LBRACKET, p.parseBracketFilterExpression)
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
	msg := fmt.Sprintf("no prefix parse function for %s found at line %d, column %d",
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
	case lexer.RETURN:
		return p.parseReturnStatement()
	case lexer.NEW:
		return p.parseInstantiationStatement()
	default:
		// Check if this might be an assignment or procedure definition
		if p.currentToken.Type == lexer.IDENT ||
			p.currentToken.Type == lexer.MY ||
			p.currentToken.Type == lexer.WORLD {

			if p.isSimpleAssignment() {
				return p.parseAssignmentStatement()
			} else if p.isProcedureDefinition() {
				return p.parseProcedureStatement()
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

	// Complex case: modifier syntax (x:(copy) = ...)
	if p.peekToken.Type == lexer.DEFINE {
		return true
	}

	// Check for path pattern (identifier.identifier...)
	if p.peekToken.Type == lexer.DOT {
		// For storage keywords (my, world), always use assignment parser
		// It will handle both read and write cases
		if p.currentToken.Type == lexer.MY || p.currentToken.Type == lexer.WORLD {
			return true
		}
		// Regular identifiers with dots are local record field access
		return false
	}

	return false
}

// looksLikePathAssignment does limited lookahead to detect path assignments
func (p *Parser) looksLikePathAssignment() bool {
	// This is called when currentToken is MY or WORLD
	// We need to check if the pattern is: MY/WORLD [.ident]* = value

	// If next token is immediate assignment, it's definitely assignment
	if p.peekToken.Type == lexer.ASSIGN {
		return true
	}

	// For now, let's be conservative and only detect simple assignments
	// More complex paths will be handled by trying expression parsing first
	return false
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
		"upper", "lower", "trim",
		// Math operations
		"abs", "floor", "ceil", "max", "min",
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

// parseAssignmentStatement parses variable assignments
func (p *Parser) parseAssignmentStatement() Statement {
	stmt := &AssignmentStatement{Token: p.currentToken}

	// Parse the left-hand side path (my.data.value)
	pathExpr := p.parsePathExpression()
	stmt.Name = pathExpr

	// Check for modifier like :(copy)
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

// parsePathExpression parses path expressions like my.data.value
func (p *Parser) parsePathExpression() *PathExpression {
	expr := &PathExpression{
		Token: p.currentToken,
		Parts: []string{p.currentToken.Literal},
	}

	for p.peekToken.Type == lexer.DOT {
		p.nextToken() // consume current
		p.nextToken() // consume DOT

		if p.currentToken.Type != lexer.IDENT {
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

	stmt.Consequence = p.parseBlockStatement()

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

	stmt.Variable = p.currentToken.Literal

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

// parseMyExpression parses 'my' path expressions
func (p *Parser) parseMyExpression() Expression {
	expr := &PathExpression{
		Token: p.currentToken,
		Parts: []string{"my"},
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
	p.nextToken()
	expression.Right = p.parseExpression(precedence)

	return expression
}

// parseCallExpression parses function calls and method chains
func (p *Parser) parseCallExpression(left Expression) Expression {
	if p.currentToken.Type == lexer.DOT {
		// Handle dot notation for method calls or property access
		if !p.expectPeek(lexer.IDENT) {
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

	exp.Filters = append(exp.Filters, p.parseExpression(LOWEST))

	for p.peekToken.Type == lexer.COMMA {
		p.nextToken()
		p.nextToken()
		exp.Filters = append(exp.Filters, p.parseExpression(LOWEST))
	}

	if !p.expectPeek(lexer.RBRACKET) {
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
