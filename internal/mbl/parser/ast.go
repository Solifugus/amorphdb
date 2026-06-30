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
	// Dynamic records bracket key-selector segments whose name must be computed
	// at runtime (e.g. tokens[token] or data[i]). The map key is the index into
	// Parts of the segment; the value is the expression to evaluate to that
	// segment's name. Static literal keys (pwa["host.example.com"]) are baked
	// directly into Parts at parse time and have no Dynamic entry. nil/empty for
	// a fully static path. Only assignment targets currently produce these (see
	// parsePathTarget); read-side bracket selectors still flow through
	// BracketFilterExpression.
	Dynamic map[int]Expression
}

func (pe *PathExpression) expressionNode() {}

func (pe *PathExpression) String() string {
	if len(pe.Dynamic) == 0 {
		return strings.Join(pe.Parts, ".")
	}
	// Render dynamic segments as [expr] so debug output round-trips intent.
	var b strings.Builder
	for idx, part := range pe.Parts {
		if expr, ok := pe.Dynamic[idx]; ok {
			b.WriteString("[")
			b.WriteString(expr.String())
			b.WriteString("]")
			continue
		}
		if idx > 0 {
			b.WriteString(".")
		}
		b.WriteString(part)
	}
	return b.String()
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
	// Special case for space operator (concatenation)
	if be.Operator == " " {
		return fmt.Sprintf("%s %s", be.Left.String(), be.Right.String())
	}
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

// ReferenceExpression represents a reference to a path, not its value (e.g., (link)world.market.price)
type ReferenceExpression struct {
	Token lexer.Token // The LINK token
	Path  Expression  // The path being referenced
}

func (re *ReferenceExpression) expressionNode() {}

func (re *ReferenceExpression) String() string {
	return fmt.Sprintf("(link)%s", re.Path.String())
}

func (re *ReferenceExpression) TokenLiteral() string { return re.Token.Literal }
func (re *ReferenceExpression) Position() (int, int) { return re.Token.Line, re.Token.Column }

// ModifierExpression represents heritability modifiers like (copy) value or (reset 0) value
type ModifierExpression struct {
	Token    lexer.Token // The modifier token (COPY, RESET, etc.)
	Modifier string      // The modifier text like "(copy)" or "(reset 0)"
	Value    Expression  // The value being modified
}

func (me *ModifierExpression) expressionNode() {}

func (me *ModifierExpression) String() string {
	return fmt.Sprintf("%s %s", me.Modifier, me.Value.String())
}

func (me *ModifierExpression) TokenLiteral() string { return me.Token.Literal }
func (me *ModifierExpression) Position() (int, int) { return me.Token.Line, me.Token.Column }

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

// CollectionOperationExpression represents collection operations like x..count, x..remove(), x..combine()
type CollectionOperationExpression struct {
	Token     lexer.Token  // The method name token
	Object    Expression   // The object being operated on
	Method    string       // count, remove, combine
	Arguments []Expression // Arguments (if any)
}

func (coe *CollectionOperationExpression) expressionNode() {}

func (coe *CollectionOperationExpression) String() string {
	if len(coe.Arguments) > 0 {
		var args []string
		for _, arg := range coe.Arguments {
			args = append(args, arg.String())
		}
		return fmt.Sprintf("%s..%s(%s)", coe.Object.String(), coe.Method, strings.Join(args, ", "))
	}
	return fmt.Sprintf("%s..%s", coe.Object.String(), coe.Method)
}

func (coe *CollectionOperationExpression) TokenLiteral() string { return coe.Token.Literal }
func (coe *CollectionOperationExpression) Position() (int, int) {
	return coe.Token.Line, coe.Token.Column
}

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

// ProjectionExpression represents projection syntax like path{ name, age, job }
type ProjectionExpression struct {
	Token  lexer.Token // The LBRACE token
	Left   Expression  // The expression being projected (path)
	Fields []string    // The field names to select
}

func (pe *ProjectionExpression) expressionNode() {}

func (pe *ProjectionExpression) String() string {
	return fmt.Sprintf("%s{ %s }", pe.Left.String(), strings.Join(pe.Fields, ", "))
}

func (pe *ProjectionExpression) TokenLiteral() string { return pe.Token.Literal }
func (pe *ProjectionExpression) Position() (int, int) { return pe.Token.Line, pe.Token.Column }

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
	Token     lexer.Token // The FOR token
	Variables []string    // The loop variables (e.g., ["item", "index"])
	Iterable  Expression  // What we're iterating over
	Body      *BlockStatement
}

func (fs *ForStatement) statementNode() {}

func (fs *ForStatement) String() string {
	variables := strings.Join(fs.Variables, ", ")
	return fmt.Sprintf("for %s in %s: %s", variables, fs.Iterable.String(), fs.Body.String())
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

// PassStatement represents pass statements (no-op statements)
type PassStatement struct {
	Token lexer.Token // The PASS token
}

func (ps *PassStatement) statementNode() {}

func (ps *PassStatement) String() string {
	return "pass"
}

func (ps *PassStatement) TokenLiteral() string { return ps.Token.Literal }
func (ps *PassStatement) Position() (int, int) { return ps.Token.Line, ps.Token.Column }

// BreakStatement represents a break statement that exits the nearest enclosing loop
type BreakStatement struct {
	Token lexer.Token // The BREAK token
}

func (bs *BreakStatement) statementNode() {}

func (bs *BreakStatement) String() string {
	return "break"
}

func (bs *BreakStatement) TokenLiteral() string { return bs.Token.Literal }
func (bs *BreakStatement) Position() (int, int) { return bs.Token.Line, bs.Token.Column }

// ScopeStatement represents scope setting statements like my.path.
type ScopeStatement struct {
	Token lexer.Token     // The first token of the path
	Path  string          // The scope path with trailing dot
	Body  *BlockStatement // Optional body for scope statements with ":"
}

func (ss *ScopeStatement) statementNode() {}

func (ss *ScopeStatement) String() string {
	if ss.Body != nil {
		return ss.Path + ": " + ss.Body.String()
	}
	return ss.Path
}

func (ss *ScopeStatement) TokenLiteral() string { return ss.Token.Literal }
func (ss *ScopeStatement) Position() (int, int) { return ss.Token.Line, ss.Token.Column }

// WatchStatement represents watch statements with optional append trigger
type WatchStatement struct {
	Token       lexer.Token     // The WATCH token
	Name        string          // Watcher identifier (left side of :)
	IsAppend    bool            // Whether this is an append watcher
	Paths       []Expression    // The paths being watched (MULTIPLE for regular, single for append)
	Filters     []Expression    // Optional predicate filters for append watchers
	BindingName string          // Variable name for 'as name' binding (append watchers only)
	Body        *BlockStatement // The watcher body
}

func (ws *WatchStatement) statementNode() {}

func (ws *WatchStatement) String() string {
	var out strings.Builder
	out.WriteString(ws.Name)
	out.WriteString(": watch")

	if ws.IsAppend {
		// Append watchers use single path (first in Paths array)
		out.WriteString(" append(")
		if len(ws.Paths) > 0 {
			out.WriteString(ws.Paths[0].String())
		}
		if len(ws.Filters) > 0 {
			out.WriteString("[")
			for i, filter := range ws.Filters {
				if i > 0 {
					out.WriteString(", ")
				}
				out.WriteString(filter.String())
			}
			out.WriteString("]")
		}
		out.WriteString(")")
		if ws.BindingName != "" {
			out.WriteString(" as ")
			out.WriteString(ws.BindingName)
		}
	} else {
		// Regular watchers support multiple comma-separated paths
		out.WriteString("(")
		for i, path := range ws.Paths {
			if i > 0 {
				out.WriteString(", ")
			}
			out.WriteString(path.String())
		}
		out.WriteString(")")
	}

	out.WriteString(": ")
	out.WriteString(ws.Body.String())
	return out.String()
}

func (ws *WatchStatement) TokenLiteral() string { return ws.Token.Literal }
func (ws *WatchStatement) Position() (int, int) { return ws.Token.Line, ws.Token.Column }

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

// EmbedDirectiveStatement represents embed directives in record bodies
type EmbedDirectiveStatement struct {
	Token lexer.Token // The EMBED or SPREAD token
	Path  Expression  // The path being embedded
}

func (eds *EmbedDirectiveStatement) statementNode() {}

func (eds *EmbedDirectiveStatement) String() string {
	if eds.Token.Type == lexer.SPREAD {
		return "..." + eds.Path.String()
	}
	return "embed " + eds.Path.String()
}

func (eds *EmbedDirectiveStatement) TokenLiteral() string { return eds.Token.Literal }
func (eds *EmbedDirectiveStatement) Position() (int, int) { return eds.Token.Line, eds.Token.Column }

// DefinitionStatement represents definitions like "my.var: value" or "my.func: procedure(x): return x"
type DefinitionStatement struct {
	Token lexer.Token // The identifier token
	Name  Expression  // The name being defined (path expression)
	Value Expression  // The value or expression being assigned
}

func (ds *DefinitionStatement) statementNode() {}

func (ds *DefinitionStatement) String() string {
	return fmt.Sprintf("%s: %s", ds.Name.String(), ds.Value.String())
}

func (ds *DefinitionStatement) TokenLiteral() string { return ds.Token.Literal }
func (ds *DefinitionStatement) Position() (int, int) { return ds.Token.Line, ds.Token.Column }

// WatchExpression represents watch expressions like "watch(path)" or "watch append(path) as var"
type WatchExpression struct {
	Token     lexer.Token     // The WATCH token
	Paths     []Expression    // The paths being watched
	IsAppend  bool            // True for "watch append(...)"
	Alias     string          // Optional alias from "as var"
	Predicate Expression      // Optional predicate for filtering
	Body      *BlockStatement // Optional body for watch expressions with ":"
}

func (we *WatchExpression) expressionNode() {}

func (we *WatchExpression) String() string {
	var out strings.Builder
	out.WriteString("watch")

	if we.IsAppend {
		out.WriteString(" append")
	}

	out.WriteString("(")
	for i, path := range we.Paths {
		if i > 0 {
			out.WriteString(", ")
		}
		out.WriteString(path.String())
	}
	out.WriteString(")")

	if we.Alias != "" {
		out.WriteString(" as ")
		out.WriteString(we.Alias)
	}

	if we.Body != nil {
		out.WriteString(": ")
		out.WriteString(we.Body.String())
	}

	return out.String()
}

func (we *WatchExpression) TokenLiteral() string { return we.Token.Literal }
func (we *WatchExpression) Position() (int, int) { return we.Token.Line, we.Token.Column }

// ProcedureExpression represents procedure expressions like "procedure(x, y): return x + y"
type ProcedureExpression struct {
	Token      lexer.Token     // The identifier token
	Parameters []string        // Parameter names
	Body       *BlockStatement // Procedure body
}

func (pe *ProcedureExpression) expressionNode() {}

func (pe *ProcedureExpression) String() string {
	params := strings.Join(pe.Parameters, ", ")
	return fmt.Sprintf("procedure(%s): %s", params, pe.Body.String())
}

func (pe *ProcedureExpression) TokenLiteral() string { return pe.Token.Literal }
func (pe *ProcedureExpression) Position() (int, int) { return pe.Token.Line, pe.Token.Column }

// CatchStatement represents catch-else exception handling
type CatchStatement struct {
	Token       lexer.Token     // The CATCH token
	TryBody     *BlockStatement // The catch block
	ElseClauses []*ElseClause   // else unknown, else error, etc.
	FinalElse   *BlockStatement // final else clause
}

type ElseClause struct {
	Token     lexer.Token // The ELSE token
	ErrorType string      // "unknown", "error", etc. (optional)
	Body      *BlockStatement
}

func (cs *CatchStatement) statementNode() {}

func (cs *CatchStatement) String() string {
	var out strings.Builder
	out.WriteString("catch: ")
	out.WriteString(cs.TryBody.String())

	for _, clause := range cs.ElseClauses {
		out.WriteString(" else")
		if clause.ErrorType != "" {
			out.WriteString(" ")
			out.WriteString(clause.ErrorType)
		}
		out.WriteString(": ")
		out.WriteString(clause.Body.String())
	}

	if cs.FinalElse != nil {
		out.WriteString(" else: ")
		out.WriteString(cs.FinalElse.String())
	}

	return out.String()
}

func (cs *CatchStatement) TokenLiteral() string { return cs.Token.Literal }
func (cs *CatchStatement) Position() (int, int) { return cs.Token.Line, cs.Token.Column }
