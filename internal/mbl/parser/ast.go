// Package parser implements MBL syntax analysis and AST generation
package parser

import (
	"fmt"
	"strings"

	"github.com/solifugus/amorphdb/internal/mbl/lexer"
)

// Node represents the base interface for all AST nodes
type Node interface {
	String() string       // Debug representation
	TokenLiteral() string // Source token literal
	Position() (int, int) // Line, column for error reporting
}

// Statement represents all statement nodes
type Statement interface {
	Node
	statementNode()
}

// Expression represents all expression nodes
type Expression interface {
	Node
	expressionNode()
}

// Program represents the root node containing all statements
type Program struct {
	Statements []Statement
	Token      lexer.Token // First token of the program
}

func (p *Program) String() string {
	var out strings.Builder
	for _, s := range p.Statements {
		out.WriteString(s.String())
	}
	return out.String()
}

func (p *Program) TokenLiteral() string {
	if len(p.Statements) > 0 {
		return p.Statements[0].TokenLiteral()
	}
	return ""
}

func (p *Program) Position() (int, int) {
	if p.Token.Line != 0 {
		return p.Token.Line, p.Token.Column
	}
	if len(p.Statements) > 0 {
		return p.Statements[0].Position()
	}
	return 0, 0
}

// BlockStatement represents a block of statements with indentation
type BlockStatement struct {
	Token      lexer.Token // The INDENT token
	Statements []Statement
}

func (bs *BlockStatement) statementNode() {}

func (bs *BlockStatement) String() string {
	var out strings.Builder
	out.WriteString("{\n")
	for _, s := range bs.Statements {
		out.WriteString("  " + strings.ReplaceAll(s.String(), "\n", "\n  "))
		out.WriteString("\n")
	}
	out.WriteString("}")
	return out.String()
}

func (bs *BlockStatement) TokenLiteral() string { return bs.Token.Literal }
func (bs *BlockStatement) Position() (int, int) { return bs.Token.Line, bs.Token.Column }

// ============================================================================
// EXPRESSION NODES
// ============================================================================

// PathExpression represents path expressions like my.data.value, world.config
type PathExpression struct {
	Token lexer.Token // First identifier token
	Parts []string    // The parts of the path
}

func (pe *PathExpression) expressionNode() {}

func (pe *PathExpression) String() string {
	return strings.Join(pe.Parts, ".")
}

func (pe *PathExpression) TokenLiteral() string { return pe.Token.Literal }
func (pe *PathExpression) Position() (int, int) { return pe.Token.Line, pe.Token.Column }

// BinaryExpression represents binary operations with precedence
type BinaryExpression struct {
	Token    lexer.Token // The operator token
	Left     Expression
	Operator string
	Right    Expression
}

func (be *BinaryExpression) expressionNode() {}

func (be *BinaryExpression) String() string {
	return fmt.Sprintf("(%s %s %s)", be.Left.String(), be.Operator, be.Right.String())
}

func (be *BinaryExpression) TokenLiteral() string { return be.Token.Literal }
func (be *BinaryExpression) Position() (int, int) { return be.Token.Line, be.Token.Column }

// UnaryExpression represents unary operations like not, -, +
type UnaryExpression struct {
	Token    lexer.Token // The operator token
	Operator string
	Right    Expression
}

func (ue *UnaryExpression) expressionNode() {}

func (ue *UnaryExpression) String() string {
	// Use space for word operators like "not", no space for symbol operators like "-", "+"
	if ue.Operator == "not" {
		return fmt.Sprintf("(%s %s)", ue.Operator, ue.Right.String())
	}
	return fmt.Sprintf("(%s%s)", ue.Operator, ue.Right.String())
}

func (ue *UnaryExpression) TokenLiteral() string { return ue.Token.Literal }
func (ue *UnaryExpression) Position() (int, int) { return ue.Token.Line, ue.Token.Column }

// LiteralExpression represents literal values (strings, numbers, time, money, booleans)
type LiteralExpression struct {
	Token lexer.Token
	Value interface{} // Parsed value
}

func (le *LiteralExpression) expressionNode() {}

func (le *LiteralExpression) String() string {
	return le.Token.Literal
}

func (le *LiteralExpression) TokenLiteral() string { return le.Token.Literal }
func (le *LiteralExpression) Position() (int, int) { return le.Token.Line, le.Token.Column }

// CallExpression represents system operations like text..trim()..upper
type CallExpression struct {
	Token     lexer.Token // The identifier or DOT token
	Function  Expression  // The function being called or expression being chained
	Arguments []Expression
}

func (ce *CallExpression) expressionNode() {}

func (ce *CallExpression) String() string {
	var args strings.Builder
	for i, arg := range ce.Arguments {
		if i > 0 {
			args.WriteString(", ")
		}
		args.WriteString(arg.String())
	}
	return fmt.Sprintf("%s(%s)", ce.Function.String(), args.String())
}

func (ce *CallExpression) TokenLiteral() string { return ce.Token.Literal }
func (ce *CallExpression) Position() (int, int) { return ce.Token.Line, ce.Token.Column }

// BracketFilterExpression represents bracket filters like person[name = "bob", age > 18]
type BracketFilterExpression struct {
	Token   lexer.Token  // The LBRACKET token
	Left    Expression   // The expression being filtered
	Filters []Expression // The filter conditions
}

func (bfe *BracketFilterExpression) expressionNode() {}

func (bfe *BracketFilterExpression) String() string {
	var filters strings.Builder
	for i, filter := range bfe.Filters {
		if i > 0 {
			filters.WriteString(", ")
		}
		filters.WriteString(filter.String())
	}
	return fmt.Sprintf("%s[%s]", bfe.Left.String(), filters.String())
}

func (bfe *BracketFilterExpression) TokenLiteral() string { return bfe.Token.Literal }
func (bfe *BracketFilterExpression) Position() (int, int) { return bfe.Token.Line, bfe.Token.Column }

// RecordField represents a single field in a record with optional heritability modifiers
type RecordField struct {
	Key       Expression // The field key
	Modifier  *string    // Optional heritability modifier like "copy", "link", "reset", "exclude"
	ResetExpr Expression // Optional reset expression for (reset expr) modifier
	Value     Expression // The field value
}

// RecordExpression represents record literals like {x: 1, y: 2} with heritability support
type RecordExpression struct {
	Token  lexer.Token   // The LBRACE token
	Fields []RecordField // Fields with optional modifiers
	// Pairs is kept for backward compatibility during transition
	Pairs map[Expression]Expression
}

func (re *RecordExpression) expressionNode() {}

func (re *RecordExpression) String() string {
	var pairs strings.Builder
	pairs.WriteString("{")

	// Handle new Fields structure
	if len(re.Fields) > 0 {
		for i, field := range re.Fields {
			if i > 0 {
				pairs.WriteString(", ")
			}

			// Key
			pairs.WriteString(field.Key.String())

			// Modifier if present
			if field.Modifier != nil {
				pairs.WriteString(":(")
				pairs.WriteString(*field.Modifier)
				if field.ResetExpr != nil {
					pairs.WriteString(" ")
					pairs.WriteString(field.ResetExpr.String())
				}
				pairs.WriteString(")")
			}

			pairs.WriteString(": ")
			pairs.WriteString(field.Value.String())
		}
	} else {
		// Backward compatibility with old Pairs
		i := 0
		for key, value := range re.Pairs {
			if i > 0 {
				pairs.WriteString(", ")
			}
			pairs.WriteString(fmt.Sprintf("%s: %s", key.String(), value.String()))
			i++
		}
	}

	pairs.WriteString("}")
	return pairs.String()
}

func (re *RecordExpression) TokenLiteral() string { return re.Token.Literal }
func (re *RecordExpression) Position() (int, int) { return re.Token.Line, re.Token.Column }

// ListExpression represents list literals like ["a", "b"]
type ListExpression struct {
	Token    lexer.Token // The LBRACKET token
	Elements []Expression
}

func (le *ListExpression) expressionNode() {}

func (le *ListExpression) String() string {
	var elements strings.Builder
	elements.WriteString("[")
	for i, el := range le.Elements {
		if i > 0 {
			elements.WriteString(", ")
		}
		elements.WriteString(el.String())
	}
	elements.WriteString("]")
	return elements.String()
}

func (le *ListExpression) TokenLiteral() string { return le.Token.Literal }
func (le *ListExpression) Position() (int, int) { return le.Token.Line, le.Token.Column }

// ============================================================================
// STATEMENT NODES
// ============================================================================

// AssignmentStatement represents assignments like x = value, x:(copy) value
type AssignmentStatement struct {
	Token    lexer.Token // The identifier token
	Name     *PathExpression
	Value    Expression
	Modifier *string // Optional modifier like "copy", "link", etc.
}

func (as *AssignmentStatement) statementNode() {}

func (as *AssignmentStatement) String() string {
	modifier := ""
	if as.Modifier != nil {
		modifier = fmt.Sprintf(":(%s) ", *as.Modifier)
	}
	return fmt.Sprintf("%s %s= %s", as.Name.String(), modifier, as.Value.String())
}

func (as *AssignmentStatement) TokenLiteral() string { return as.Token.Literal }
func (as *AssignmentStatement) Position() (int, int) { return as.Token.Line, as.Token.Column }

// IfStatement represents if/else constructs
type IfStatement struct {
	Token       lexer.Token // The IF token
	Condition   Expression
	Consequence *BlockStatement
	Alternative Statement // Can be another IfStatement (elif) or BlockStatement (else)
}

func (ifs *IfStatement) statementNode() {}

func (ifs *IfStatement) String() string {
	var out strings.Builder
	out.WriteString("if ")
	out.WriteString(ifs.Condition.String())
	out.WriteString(": ")
	out.WriteString(ifs.Consequence.String())
	if ifs.Alternative != nil {
		out.WriteString(" else ")
		out.WriteString(ifs.Alternative.String())
	}
	return out.String()
}

func (ifs *IfStatement) TokenLiteral() string { return ifs.Token.Literal }
func (ifs *IfStatement) Position() (int, int) { return ifs.Token.Line, ifs.Token.Column }

// WhileStatement represents while loops
type WhileStatement struct {
	Token     lexer.Token // The WHILE token
	Condition Expression
	Body      *BlockStatement
}

func (ws *WhileStatement) statementNode() {}

func (ws *WhileStatement) String() string {
	return fmt.Sprintf("while %s: %s", ws.Condition.String(), ws.Body.String())
}

func (ws *WhileStatement) TokenLiteral() string { return ws.Token.Literal }
func (ws *WhileStatement) Position() (int, int) { return ws.Token.Line, ws.Token.Column }

// ForStatement represents for loops
type ForStatement struct {
	Token    lexer.Token // The FOR token
	Variable string      // The loop variable
	Iterable Expression  // What we're iterating over
	Body     *BlockStatement
}

func (fs *ForStatement) statementNode() {}

func (fs *ForStatement) String() string {
	return fmt.Sprintf("for %s in %s: %s", fs.Variable, fs.Iterable.String(), fs.Body.String())
}

func (fs *ForStatement) TokenLiteral() string { return fs.Token.Literal }
func (fs *ForStatement) Position() (int, int) { return fs.Token.Line, fs.Token.Column }

// ConsiderStatement represents switch-like constructs
type ConsiderStatement struct {
	Token   lexer.Token // The CONSIDER token
	Value   Expression
	Cases   []*ConsiderCase
	Default *BlockStatement // Optional default case
}

type ConsiderCase struct {
	Token lexer.Token // The case value token
	Value Expression
	Body  *BlockStatement
}

func (cs *ConsiderStatement) statementNode() {}

func (cs *ConsiderStatement) String() string {
	var out strings.Builder
	out.WriteString("consider ")
	out.WriteString(cs.Value.String())
	out.WriteString(": ")
	for _, c := range cs.Cases {
		out.WriteString(fmt.Sprintf("case %s: %s ", c.Value.String(), c.Body.String()))
	}
	if cs.Default != nil {
		out.WriteString("default: ")
		out.WriteString(cs.Default.String())
	}
	return out.String()
}

func (cs *ConsiderStatement) TokenLiteral() string { return cs.Token.Literal }
func (cs *ConsiderStatement) Position() (int, int) { return cs.Token.Line, cs.Token.Column }

// ProcedureStatement represents function definitions
type ProcedureStatement struct {
	Token      lexer.Token // The procedure name token
	Name       string
	Parameters []string // Parameter names
	Body       *BlockStatement
}

func (ps *ProcedureStatement) statementNode() {}

func (ps *ProcedureStatement) String() string {
	params := strings.Join(ps.Parameters, ", ")
	return fmt.Sprintf("%s(%s): %s", ps.Name, params, ps.Body.String())
}

func (ps *ProcedureStatement) TokenLiteral() string { return ps.Token.Literal }
func (ps *ProcedureStatement) Position() (int, int) { return ps.Token.Line, ps.Token.Column }

// ReturnStatement represents return statements
type ReturnStatement struct {
	Token       lexer.Token // The RETURN token
	ReturnValue Expression
}

func (rs *ReturnStatement) statementNode() {}

func (rs *ReturnStatement) String() string {
	if rs.ReturnValue != nil {
		return fmt.Sprintf("return %s", rs.ReturnValue.String())
	}
	return "return"
}

func (rs *ReturnStatement) TokenLiteral() string { return rs.Token.Literal }
func (rs *ReturnStatement) Position() (int, int) { return rs.Token.Line, rs.Token.Column }

// ExpressionStatement represents standalone expressions
type ExpressionStatement struct {
	Token      lexer.Token // First token of the expression
	Expression Expression
}

func (es *ExpressionStatement) statementNode() {}

func (es *ExpressionStatement) String() string {
	if es.Expression != nil {
		return es.Expression.String()
	}
	return ""
}

func (es *ExpressionStatement) TokenLiteral() string { return es.Token.Literal }
func (es *ExpressionStatement) Position() (int, int) { return es.Token.Line, es.Token.Column }

// InstantiationSource represents a source for inheritance - can be template name or inline record
type InstantiationSource struct {
	TemplateName *string           // Template name like "person"
	InlineRecord *RecordExpression // Inline record like { name:(copy) "value" }
}

// InstantiationStatement represents object instantiation like new person { name: "Bob" }
type InstantiationStatement struct {
	Token      lexer.Token           // The NEW token
	Type       string                // The type being instantiated
	Modifier   *string               // Optional global heritability modifier like "copy", "link", etc.
	Sources    []InstantiationSource // Mixed template sources and inline records
	Properties *RecordExpression     // The final properties (for backward compatibility)
}

func (is *InstantiationStatement) statementNode() {}

func (is *InstantiationStatement) String() string {
	var out strings.Builder
	out.WriteString("new ")
	out.WriteString(is.Type)

	// Add modifier if present
	if is.Modifier != nil {
		out.WriteString(fmt.Sprintf(":(%s)", *is.Modifier))
	}

	// Add sources if present
	if len(is.Sources) > 0 {
		out.WriteString(" ")
		for i, source := range is.Sources {
			if i > 0 {
				out.WriteString(", ")
			}
			if source.TemplateName != nil {
				out.WriteString(*source.TemplateName)
			} else if source.InlineRecord != nil {
				out.WriteString(source.InlineRecord.String())
			}
		}
	}

	// Add properties if present
	if is.Properties != nil {
		out.WriteString(" ")
		out.WriteString(is.Properties.String())
	}

	return out.String()
}

func (is *InstantiationStatement) TokenLiteral() string { return is.Token.Literal }
func (is *InstantiationStatement) Position() (int, int) { return is.Token.Line, is.Token.Column }
