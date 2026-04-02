package ari

import (
	"fmt"
	"strconv"
	"strings"
)

// Parser parses ARI specification tokens into an AST
type Parser struct {
	lexer *Lexer

	curToken  Token
	peekToken Token

	errors []string
}

// NewParser creates a new parser instance
func NewParser(lexer *Lexer) *Parser {
	p := &Parser{
		lexer:  lexer,
		errors: []string{},
	}

	// Read two tokens, so curToken and peekToken are both set
	p.nextToken()
	p.nextToken()

	return p
}

// nextToken advances tokens
func (p *Parser) nextToken() {
	p.curToken = p.peekToken
	p.peekToken = p.lexer.NextToken()
}

// ParseARISpec parses a complete ARI specification
func (p *Parser) ParseARISpec() *ARISpec {
	spec := &ARISpec{
		CustomTypes: []CustomType{},
		Sections:    []Section{},
	}

	// Skip any leading newlines
	p.skipNewlines()

	for p.curToken.Type != EOF {
		if p.curToken.Type == TYPE {
			customType := p.parseCustomType()
			if customType != nil {
				spec.CustomTypes = append(spec.CustomTypes, *customType)
			}
		} else if p.curToken.Type == SECTION {
			section := p.parseSection()
			if section != nil {
				spec.Sections = append(spec.Sections, *section)
			}
		} else {
			p.addError(fmt.Sprintf("unexpected token at top level: %s", p.curToken.Type))
			p.nextToken()
		}
		p.skipNewlines()
	}

	return spec
}

// parseCustomType parses a custom type definition
func (p *Parser) parseCustomType() *CustomType {
	if !p.expectToken(TYPE) {
		return nil
	}

	if p.curToken.Type != IDENTIFIER {
		p.addError("expected type name identifier")
		return nil
	}

	typeName := p.curToken.Literal
	p.nextToken()

	if !p.expectToken(COLON) {
		return nil
	}

	p.skipNewlines()

	customType := &CustomType{
		Name:     typeName,
		Patterns: []CustomPattern{},
	}

	// Parse pattern mappings (indented block)
	for p.isIndented() {
		// Skip indentation
		p.skipIndentation()

		// Check if this is an output declaration
		if p.curToken.Type == OUTPUT {
			p.nextToken()
			if !p.expectToken(COLON) {
				return nil
			}
			// Accept either IDENTIFIER or built-in type tokens as output types
			if p.curToken.Type != IDENTIFIER &&
			   p.curToken.Type != DATE && p.curToken.Type != MONEY &&
			   p.curToken.Type != INTEGER && p.curToken.Type != DECIMAL &&
			   p.curToken.Type != SSN && p.curToken.Type != ISO_DATE {
				p.addError("expected output type identifier")
				return nil
			}
			customType.OutputType = p.curToken.Literal
			p.nextToken()
		} else {
			// Parse as a normal pattern
			pattern := p.parseCustomPattern()
			if pattern != nil {
				customType.Patterns = append(customType.Patterns, *pattern)
			}
		}
		p.skipNewlines()
	}

	return customType
}

// parseCustomPattern parses a pattern mapping within a custom type
func (p *Parser) parseCustomPattern() *CustomPattern {
	// Skip indentation
	p.skipIndentation()

	pattern := p.parsePattern()
	if pattern == nil {
		return nil
	}

	customPattern := &CustomPattern{
		Pattern:    *pattern,
		Modifiers:  []string{},
	}

	// Look for arrow and transformation
	if p.curToken.Type == ARROW {
		p.nextToken()
		if p.curToken.Type == STRING {
			customPattern.Transform = p.curToken.Literal
			p.nextToken()
			// Update pattern type to indicate it has a transformation
			if customPattern.Pattern.Type == PatternRegex {
				customPattern.Pattern.Type = PatternRegexWithTransform
			}
		}
	}

	// Parse modifiers (strip, negate, etc.)
	for p.curToken.Type == IDENTIFIER {
		modifier := p.curToken.Literal
		if modifier == "strip" || modifier == "negate" {
			customPattern.Modifiers = append(customPattern.Modifiers, modifier)
			p.nextToken()
			if p.curToken.Type == LPAREN {
				p.nextToken()
				if p.curToken.Type == STRING {
					// strip("char")
					modifier += "(" + p.curToken.Literal + ")"
					customPattern.Modifiers[len(customPattern.Modifiers)-1] = modifier
					p.nextToken()
				}
				p.expectToken(RPAREN)
			}
		} else if modifier == "as" {
			p.nextToken()
			if p.curToken.Type == IDENTIFIER {
				customPattern.OutputType = p.curToken.Literal
				p.nextToken()
				// Handle DECIMAL(12,2) syntax
				if p.curToken.Type == LPAREN {
					p.nextToken()
					typeSpec := customPattern.OutputType + "("
					for p.curToken.Type != RPAREN && p.curToken.Type != EOF {
						typeSpec += p.curToken.Literal
						p.nextToken()
					}
					typeSpec += ")"
					customPattern.OutputType = typeSpec
					p.expectToken(RPAREN)
				}
			}
		} else {
			break
		}
	}

	return customPattern
}

// parseSection parses a section definition
func (p *Parser) parseSection() *Section {
	if !p.expectToken(SECTION) {
		return nil
	}

	if p.curToken.Type != IDENTIFIER {
		p.addError("expected section name")
		return nil
	}

	section := &Section{
		Name:     p.curToken.Literal,
		Fields:   []Field{},
		Sections: []Section{},
	}

	p.nextToken()

	// Parse optional starts/ends
	for p.curToken.Type == STARTS || p.curToken.Type == ENDS {
		if p.curToken.Type == STARTS {
			p.nextToken()
			if !p.expectToken(LPAREN) {
				return nil
			}
			pattern := p.parsePattern()
			if pattern != nil {
				section.StartsWith = pattern
			}
			p.expectToken(RPAREN)
		} else if p.curToken.Type == ENDS {
			p.nextToken()
			if !p.expectToken(LPAREN) {
				return nil
			}
			pattern := p.parsePattern()
			if pattern != nil {
				section.EndsWith = pattern
			}
			p.expectToken(RPAREN)
		}
	}

	if !p.expectToken(COLON) {
		return nil
	}

	p.skipNewlines()

	// Parse section body (indented block)
	// Track the expected indentation level for this section
	var sectionIndentLevel int
	if p.isIndented() {
		sectionIndentLevel = p.curToken.Column
	}

	for p.isIndented() && (sectionIndentLevel == 0 || p.curToken.Column >= sectionIndentLevel) {
		// If this line is less indented than our section level, stop parsing
		if sectionIndentLevel > 0 && p.curToken.Column < sectionIndentLevel {
			break
		}

		p.skipIndentation()

		if p.curToken.Type == FIELD {
			field := p.parseField()
			if field != nil {
				section.Fields = append(section.Fields, *field)
			}
		} else if p.curToken.Type == BREAK {
			breakRule := p.parseBreak()
			if breakRule != nil {
				section.Break = breakRule
			}
		} else if p.curToken.Type == SECTION {
			nestedSection := p.parseSection()
			if nestedSection != nil {
				section.Sections = append(section.Sections, *nestedSection)
			}
		} else {
			p.addError(fmt.Sprintf("unexpected token in section body: %s", p.curToken.Type))
			p.nextToken()
		}

		p.skipNewlines()
	}

	return section
}

// parseField parses a field definition using the correct grammar:
// field field_name: anchor [anchor ...]: pattern [, pattern ...]
func (p *Parser) parseField() *Field {
	if !p.expectToken(FIELD) {
		return nil
	}

	if p.curToken.Type != IDENTIFIER {
		p.addError("expected field name")
		return nil
	}

	field := &Field{
		Name:     p.curToken.Literal,
		Anchors:  []Anchor{},
		Patterns: []Pattern{},
	}

	p.nextToken()

	if !p.expectToken(COLON) {
		return nil
	}

	// Parse space-separated anchors until we hit a colon
	for {
		// Check if we've reached the pattern separator colon
		if p.curToken.Type == COLON {
			p.nextToken()
			break
		}

		// Parse anchor (direction [distance])
		anchor := p.parseAnchor()
		if anchor != nil {
			field.Anchors = append(field.Anchors, *anchor)
		} else {
			// If anchor parsing failed, try to recover by advancing
			if p.curToken.Type != COLON && p.curToken.Type != NEWLINE && p.curToken.Type != EOF {
				p.nextToken()
			}
		}

		// If we hit end of line or file, assume missing colon
		if p.curToken.Type == NEWLINE || p.curToken.Type == EOF {
			p.addError("expected colon separator between anchors and patterns")
			return field
		}
	}

	// Parse comma-separated patterns
	for {
		pattern := p.parsePattern()
		if pattern != nil {
			field.Patterns = append(field.Patterns, *pattern)
		}

		// Check for more patterns
		if p.curToken.Type == COMMA {
			p.nextToken()
			continue
		}
		break
	}

	return field
}

// parseAnchor parses an anchor: direction [distance]
// Some directions like "same" don't require a distance
func (p *Parser) parseAnchor() *Anchor {
	// Parse direction
	var direction Direction
	switch p.curToken.Type {
	case LEFT:
		direction = DirectionLeft
	case RIGHT:
		direction = DirectionRight
	case UP:
		direction = DirectionUp
	case DOWN:
		direction = DirectionDown
	case SAME:
		direction = DirectionSame
	case FLUSH:
		direction = DirectionFlush
	default:
		p.addError(fmt.Sprintf("expected direction, got %s", p.curToken.Type))
		p.nextToken() // Advance past problematic token to prevent infinite loop
		return nil
	}

	p.nextToken()

	// Check if distance is present
	// Some directions like "same" may not have an explicit distance
	var distance *Distance

	if p.isDistanceToken() {
		distance = p.parseDistance()
		if distance == nil {
			return nil
		}
	} else {
		// Default distance for directions that don't specify one
		distance = &Distance{Type: DistanceExact, Min: 0}
	}

	// Create anchor without requiring a pattern (patterns come after the colon)
	return &Anchor{
		Direction: direction,
		Distance:  *distance,
		Pattern:   Pattern{Type: PatternText, Text: ""}, // placeholder pattern
	}
}

// isDistanceToken checks if the current token represents a distance
func (p *Parser) isDistanceToken() bool {
	return p.curToken.Type == NUMBER ||
		p.curToken.Type == RANGE ||
		p.curToken.Type == OPEN_MIN ||
		p.curToken.Type == OPEN_MAX ||
		p.curToken.Type == FLUSH
}

// parseDistance parses a distance specification
func (p *Parser) parseDistance() *Distance {
	switch p.curToken.Type {
	case FLUSH:
		p.nextToken()
		return &Distance{Type: DistanceFlush}
	case NUMBER:
		num, err := strconv.Atoi(p.curToken.Literal)
		if err != nil {
			p.addError(fmt.Sprintf("invalid number: %s", p.curToken.Literal))
			return nil
		}
		p.nextToken()
		return &Distance{Type: DistanceExact, Min: num}
	case RANGE:
		parts := strings.Split(p.curToken.Literal, "-")
		if len(parts) != 2 {
			p.addError(fmt.Sprintf("invalid range: %s", p.curToken.Literal))
			return nil
		}
		min, err1 := strconv.Atoi(parts[0])
		max, err2 := strconv.Atoi(parts[1])
		if err1 != nil || err2 != nil {
			p.addError(fmt.Sprintf("invalid range numbers: %s", p.curToken.Literal))
			return nil
		}
		p.nextToken()
		return &Distance{Type: DistanceRange, Min: min, Max: max}
	case OPEN_MIN:
		numStr := strings.TrimSuffix(p.curToken.Literal, "-")
		num, err := strconv.Atoi(numStr)
		if err != nil {
			p.addError(fmt.Sprintf("invalid open minimum: %s", p.curToken.Literal))
			return nil
		}
		p.nextToken()
		return &Distance{Type: DistanceOpenMin, Min: num, Max: -1}
	case OPEN_MAX:
		numStr := strings.TrimPrefix(p.curToken.Literal, "-")
		num, err := strconv.Atoi(numStr)
		if err != nil {
			p.addError(fmt.Sprintf("invalid open maximum: %s", p.curToken.Literal))
			return nil
		}
		p.nextToken()
		return &Distance{Type: DistanceOpenMax, Min: 0, Max: num}
	default:
		p.addError(fmt.Sprintf("expected distance specification, got %s", p.curToken.Type))
		p.nextToken() // Advance past problematic token to prevent infinite loop
		return nil
	}
}

// parsePattern parses a pattern
func (p *Parser) parsePattern() *Pattern {
	switch p.curToken.Type {
	case STRING:
		pattern := &Pattern{Type: PatternText, Text: p.curToken.Literal}
		p.nextToken()
		return pattern
	case REGEX:
		pattern := &Pattern{Type: PatternRegex, Text: p.curToken.Literal}
		p.nextToken()
		return pattern
	case REGEX_WITH_TRANSFORM:
		pattern := &Pattern{Type: PatternRegexWithTransform, Text: p.curToken.Literal}
		p.nextToken()
		return pattern
	case DATE:
		pattern := &Pattern{Type: PatternBuiltinDate}
		p.nextToken()
		return pattern
	case MONEY:
		pattern := &Pattern{Type: PatternBuiltinMoney}
		p.nextToken()
		return pattern
	case INTEGER:
		pattern := &Pattern{Type: PatternBuiltinInteger}
		p.nextToken()
		return pattern
	case DECIMAL:
		pattern := &Pattern{Type: PatternBuiltinDecimal}
		p.nextToken()
		return pattern
	case SSN:
		pattern := &Pattern{Type: PatternBuiltinSSN}
		p.nextToken()
		return pattern
	case ISO_DATE:
		pattern := &Pattern{Type: PatternBuiltinISODate}
		p.nextToken()
		return pattern
	case IDENTIFIER:
		// This could be a custom type reference
		pattern := &Pattern{Type: PatternCustomType, Text: p.curToken.Literal}
		p.nextToken()
		return pattern
	default:
		p.addError(fmt.Sprintf("expected pattern, got %s", p.curToken.Type))
		p.nextToken() // Advance past problematic token to prevent infinite loop
		return nil
	}
}

// parseBreak parses a break rule
func (p *Parser) parseBreak() *Break {
	if !p.expectToken(BREAK) {
		return nil
	}

	breakRule := &Break{}

	if p.curToken.Type == ON {
		p.nextToken()
		pattern := p.parsePattern()
		if pattern == nil {
			return nil
		}
		breakRule.Type = BreakOnPattern
		breakRule.Pattern = pattern
	} else {
		breakRule.Type = BreakOnReMatch
	}

	return breakRule
}

// Helper methods

func (p *Parser) expectToken(expected TokenType) bool {
	if p.curToken.Type == expected {
		p.nextToken()
		return true
	}
	p.addError(fmt.Sprintf("expected %s, got %s", expected, p.curToken.Type))
	return false
}

func (p *Parser) addError(msg string) {
	p.errors = append(p.errors, fmt.Sprintf("line %d, col %d: %s", p.curToken.Line, p.curToken.Column, msg))
}

func (p *Parser) skipNewlines() {
	for p.curToken.Type == NEWLINE {
		p.nextToken()
	}
}

func (p *Parser) isIndented() bool {
	return p.curToken.Column > 1 && p.curToken.Type != EOF && p.curToken.Type != NEWLINE
}

func (p *Parser) skipIndentation() {
	// Skip whitespace at beginning of line
	// The lexer already handles this, so we don't need to do anything special
}

// Errors returns parsing errors
func (p *Parser) Errors() []string {
	return p.errors
}