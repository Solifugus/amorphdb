// Package interpreter implements MBL program execution against the storage engine
package interpreter

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/solifugus/amorphdb/internal/mbl/parser"
	"github.com/solifugus/amorphdb/internal/storage"
	"github.com/solifugus/amorphdb/internal/types"
)

// Scope represents an execution environment with variables and storage access
type Scope struct {
	local  map[string]interface{} // Local variables in memory
	parent *Scope                 // Enclosing scope for procedures
	tree   storage.Tree           // Persistent storage access
	agent  uint64                 // Current agent identity
}

// NewScope creates a new execution scope
func NewScope(tree storage.Tree, agent uint64) *Scope {
	return &Scope{
		local: make(map[string]interface{}),
		tree:  tree,
		agent: agent,
	}
}

// NewChildScope creates a child scope for procedures
func (s *Scope) NewChildScope() *Scope {
	return &Scope{
		local:  make(map[string]interface{}),
		parent: s,
		tree:   s.tree,
		agent:  s.agent,
	}
}

// Set sets a variable in the local scope
func (s *Scope) Set(name string, value interface{}) {
	s.local[name] = value
}

// Get retrieves a variable from local scope or parent scopes
func (s *Scope) Get(name string) (interface{}, bool) {
	if value, found := s.local[name]; found {
		return value, true
	}
	if s.parent != nil {
		return s.parent.Get(name)
	}
	return types.Unknown{Reason: fmt.Sprintf("undefined variable '%s'", name)}, false
}

// Interpreter executes MBL programs against the storage engine
type Interpreter struct {
	scope  *Scope
	errors []string // Accumulated errors during execution
}

// New creates a new interpreter instance
func New(tree storage.Tree, agent uint64) *Interpreter {
	return &Interpreter{
		scope: NewScope(tree, agent),
	}
}

// Interpret executes a program AST and returns the result
func (i *Interpreter) Interpret(program *parser.Program) (interface{}, error) {
	if len(i.errors) > 0 {
		i.errors = i.errors[:0] // Clear previous errors
	}

	var result interface{} = types.Nothing{}

	for _, statement := range program.Statements {
		result = i.evalStatement(statement)

		// Note: Unknown values are valid results in MBL, representing resilient computation
		// They should not halt execution unless they represent critical errors
		// For now, we allow all Unknown values to be normal results
	}

	return result, nil
}

// Errors returns accumulated interpretation errors
func (i *Interpreter) Errors() []string {
	return i.errors
}

// evalStatement evaluates a statement node
func (i *Interpreter) evalStatement(node parser.Statement) interface{} {
	switch node := node.(type) {
	case *parser.AssignmentStatement:
		return i.evalAssignment(node)
	case *parser.ExpressionStatement:
		return i.evalExpression(node.Expression)
	case *parser.IfStatement:
		return i.evalIfStatement(node)
	case *parser.WhileStatement:
		return i.evalWhileStatement(node)
	case *parser.ForStatement:
		return i.evalForStatement(node)
	case *parser.ConsiderStatement:
		return i.evalConsiderStatement(node)
	case *parser.ReturnStatement:
		return i.evalReturnStatement(node)
	case *parser.ProcedureStatement:
		return i.evalProcedureStatement(node)
	case *parser.InstantiationStatement:
		return i.evalInstantiationStatement(node)
	case *parser.BlockStatement:
		return i.evalBlockStatement(node)
	default:
		return types.Unknown{Reason: fmt.Sprintf("unsupported statement type: %T", node)}
	}
}

// evalExpression evaluates an expression node
func (i *Interpreter) evalExpression(node parser.Expression) interface{} {
	switch node := node.(type) {
	case *parser.LiteralExpression:
		return i.evalLiteral(node)
	case *parser.PathExpression:
		return i.evalPath(node)
	case *parser.BinaryExpression:
		return i.evalBinaryExpression(node)
	case *parser.UnaryExpression:
		return i.evalUnaryExpression(node)
	case *parser.CallExpression:
		return i.evalCallExpression(node)
	case *parser.BracketFilterExpression:
		return i.evalBracketFilter(node)
	case *parser.RecordExpression:
		return i.evalRecordExpression(node)
	case *parser.ListExpression:
		return i.evalListExpression(node)
	default:
		return types.Unknown{Reason: fmt.Sprintf("unsupported expression type: %T", node)}
	}
}

// evalLiteral evaluates literal values
func (i *Interpreter) evalLiteral(node *parser.LiteralExpression) interface{} {
	if node.Value != nil {
		// If the parser already parsed the value, convert to MBL type
		return convertToMBLType(node.Value)
	}

	// Parse the literal from token literal
	literal := node.Token.Literal

	// Try to parse as different types based on token content
	// Numbers
	if num, err := strconv.ParseFloat(literal, 64); err == nil {
		return types.Number{Value: num}
	}

	// Strings (remove quotes)
	if len(literal) >= 2 && literal[0] == '"' && literal[len(literal)-1] == '"' {
		return types.Text{Value: literal[1 : len(literal)-1]}
	}

	// Booleans
	if literal == "true" {
		return types.Boolean{Value: true}
	}
	if literal == "false" {
		return types.Boolean{Value: false}
	}

	// Time literals (prefixed with @)
	if len(literal) > 1 && literal[0] == '@' {
		if time, err := parseTimeString(literal); err == nil {
			return time
		}
		return types.Unknown{Reason: fmt.Sprintf("invalid time literal: %s", literal)}
	}

	// Money literals (prefixed with ¤)
	if len(literal) > 1 && literal[0] == '¤' {
		if money, err := parseMoneyString(literal); err == nil {
			return money
		}
		return types.Unknown{Reason: fmt.Sprintf("invalid money literal: %s", literal)}
	}

	// Special values
	switch literal {
	case "Nothing":
		return types.Nothing{}
	case "Unknown":
		return types.Unknown{Reason: "explicit Unknown"}
	case "Anything":
		return types.Anything{}
	default:
		return types.Unknown{Reason: fmt.Sprintf("unknown literal: %s", literal)}
	}
}

// evalPath evaluates path expressions like my.data.value
func (i *Interpreter) evalPath(node *parser.PathExpression) interface{} {
	if len(node.Parts) == 0 {
		return types.Unknown{Reason: "empty path"}
	}

	// Check for local variables first
	if len(node.Parts) == 1 {
		if value, found := i.scope.Get(node.Parts[0]); found {
			return value
		}
	}

	// Resolve special path roots
	path := i.resolvePath(node.Parts)

	// Read from storage
	storageValue, err := i.scope.tree.Read(path)
	if err != nil {
		return types.Unknown{Reason: fmt.Sprintf("failed to read path %v: %v", path, err)}
	}

	// Convert storage.Value to MBL type
	mblValue, err := storageToMBL(storageValue)
	if err != nil {
		return types.Unknown{Reason: fmt.Sprintf("failed to convert storage value: %v", err)}
	}

	return mblValue
}

// resolvePath resolves special path prefixes like 'my', 'world', '~'
func (i *Interpreter) resolvePath(parts []string) []string {
	if len(parts) == 0 {
		return parts
	}

	switch parts[0] {
	case "my":
		// my.* -> world.agent.{identity}.*
		agentPath := []string{"world", "agent", fmt.Sprintf("%d", i.scope.agent)}
		return append(agentPath, parts[1:]...)
	case "world":
		// world.* -> world.*
		return parts
	case "~":
		// ~.* -> world.agent.{identity}.home.*
		homePath := []string{"world", "agent", fmt.Sprintf("%d", i.scope.agent), "home"}
		return append(homePath, parts[1:]...)
	default:
		// Relative paths resolve to current agent context
		agentPath := []string{"world", "agent", fmt.Sprintf("%d", i.scope.agent)}
		return append(agentPath, parts...)
	}
}

// evalBinaryExpression evaluates binary operations
func (i *Interpreter) evalBinaryExpression(node *parser.BinaryExpression) interface{} {
	left := i.evalExpression(node.Left)
	right := i.evalExpression(node.Right)

	// Unknown propagation - any Unknown input produces Unknown output
	if unknown, ok := left.(types.Unknown); ok {
		return unknown
	}
	if unknown, ok := right.(types.Unknown); ok {
		return unknown
	}

	switch node.Operator {
	// Arithmetic operators
	case "+":
		return types.Add(left, right)
	case "-":
		return types.Subtract(left, right)
	case "*":
		return types.Multiply(left, right)
	case "/":
		return types.Divide(left, right)
	case "%":
		return types.Modulo(left, right)

	// Concatenation
	case "&":
		return types.Concatenate(left, right)

	// Comparison operators
	case "?=":
		return types.Boolean{Value: types.Equal(left, right)}
	case ">":
		return types.Boolean{Value: types.Greater(left, right)}
	case "<":
		return types.Boolean{Value: types.Less(left, right)}
	case ">=":
		return types.Boolean{Value: types.GreaterEqual(left, right)}
	case "<=":
		return types.Boolean{Value: types.LessEqual(left, right)}

	// Logical operators
	case "and":
		return i.evalLogicalAnd(left, right)
	case "or":
		return i.evalLogicalOr(left, right)

	default:
		return types.Unknown{Reason: fmt.Sprintf("unsupported binary operator: %s", node.Operator)}
	}
}

// evalLogicalAnd implements logical AND with short-circuiting
func (i *Interpreter) evalLogicalAnd(left, right interface{}) interface{} {
	leftBool := isTruthy(left)
	if !leftBool {
		return types.Boolean{Value: false}
	}
	rightBool := isTruthy(right)
	return types.Boolean{Value: rightBool}
}

// evalLogicalOr implements logical OR with short-circuiting
func (i *Interpreter) evalLogicalOr(left, right interface{}) interface{} {
	leftBool := isTruthy(left)
	if leftBool {
		return types.Boolean{Value: true}
	}
	rightBool := isTruthy(right)
	return types.Boolean{Value: rightBool}
}

// evalUnaryExpression evaluates unary operations
func (i *Interpreter) evalUnaryExpression(node *parser.UnaryExpression) interface{} {
	operand := i.evalExpression(node.Right)

	// Unknown propagation
	if unknown, ok := operand.(types.Unknown); ok {
		return unknown
	}

	switch node.Operator {
	case "-":
		return negate(operand)
	case "+":
		// Unary plus - ensure numeric type
		if isNumeric(operand) {
			return operand
		}
		return types.Unknown{Reason: "unary plus requires numeric operand"}
	case "not":
		return types.Boolean{Value: !isTruthy(operand)}
	default:
		return types.Unknown{Reason: fmt.Sprintf("unsupported unary operator: %s", node.Operator)}
	}
}

// evalAssignment evaluates assignment statements
func (i *Interpreter) evalAssignment(node *parser.AssignmentStatement) interface{} {
	value := i.evalExpression(node.Value)

	// Check for Unknown result
	if unknown, ok := value.(types.Unknown); ok {
		return unknown
	}

	// For simple variable names, store locally
	if len(node.Name.Parts) == 1 && !i.isSpecialPath(node.Name.Parts[0]) {
		i.scope.Set(node.Name.Parts[0], value)
		return value
	}

	// For paths, write to storage
	path := i.resolvePath(node.Name.Parts)

	// Convert MBL type to storage.Value
	storageValue, err := mblToStorage(value)
	if err != nil {
		return types.Unknown{Reason: fmt.Sprintf("failed to convert value for storage: %v", err)}
	}

	err = i.scope.tree.Write(path, storageValue, i.scope.agent)
	if err != nil {
		return types.Unknown{Reason: fmt.Sprintf("failed to write to path %v: %v", path, err)}
	}

	return value
}

// isSpecialPath checks if a path starts with special identifiers
func (i *Interpreter) isSpecialPath(name string) bool {
	return name == "my" || name == "world" || name == "~"
}

// evalBlockStatement evaluates a block of statements
func (i *Interpreter) evalBlockStatement(node *parser.BlockStatement) interface{} {
	var result interface{} = types.Nothing{}

	for _, statement := range node.Statements {
		result = i.evalStatement(statement)

		// Stop on Unknown (error)
		if unknown, ok := result.(types.Unknown); ok && unknown.Reason != "" {
			return unknown
		}
	}

	return result
}

// Placeholder implementations for complex statements
func (i *Interpreter) evalIfStatement(node *parser.IfStatement) interface{} {
	condition := i.evalExpression(node.Condition)

	if unknown, ok := condition.(types.Unknown); ok {
		return unknown
	}

	if isTruthy(condition) {
		return i.evalStatement(node.Consequence)
	} else if node.Alternative != nil {
		return i.evalStatement(node.Alternative)
	}

	return types.Nothing{}
}

func (i *Interpreter) evalWhileStatement(node *parser.WhileStatement) interface{} {
	for {
		condition := i.evalExpression(node.Condition)

		if unknown, ok := condition.(types.Unknown); ok {
			return unknown
		}

		if !isTruthy(condition) {
			break
		}

		result := i.evalStatement(node.Body)
		if unknown, ok := result.(types.Unknown); ok {
			return unknown
		}
	}

	return types.Nothing{}
}

func (i *Interpreter) evalForStatement(node *parser.ForStatement) interface{} {
	// Evaluate the iterable expression (should be a List)
	iterable := i.evalExpression(node.Iterable)

	if unknown, ok := iterable.(types.Unknown); ok {
		return unknown
	}

	// Check if it's a List
	list, ok := iterable.(types.List)
	if !ok {
		return types.Unknown{Reason: fmt.Sprintf("cannot iterate over %T", iterable)}
	}

	// Create a child scope for the loop variable
	childScope := i.scope.NewChildScope()
	oldScope := i.scope
	i.scope = childScope

	var result interface{} = types.Nothing{}

	// Iterate over elements
	for _, element := range list.Elements {
		// Set the loop variable
		i.scope.Set(node.Variable, element)

		// Execute the loop body
		result = i.evalStatement(node.Body)

		// Check for Unknown (error) - break on error
		if _, ok := result.(types.Unknown); ok {
			break
		}
	}

	// Restore original scope
	i.scope = oldScope

	return result
}

func (i *Interpreter) evalConsiderStatement(node *parser.ConsiderStatement) interface{} {
	// Evaluate the value to consider
	value := i.evalExpression(node.Value)

	if unknown, ok := value.(types.Unknown); ok {
		return unknown
	}

	// Try each case
	for _, caseNode := range node.Cases {
		// Evaluate the case condition
		caseValue := i.evalExpression(caseNode.Value)

		if unknown, ok := caseValue.(types.Unknown); ok {
			return unknown
		}

		// Check if values are equal using MBL equality rules
		if types.Equal(value, caseValue) {
			// Execute this case body (Body is a *BlockStatement)
			return i.evalBlockStatement(caseNode.Body)
		}
	}

	// If no cases match, execute the default case if present
	if node.Default != nil {
		return i.evalBlockStatement(node.Default)
	}

	// No match and no default
	return types.Nothing{}
}

func (i *Interpreter) evalReturnStatement(node *parser.ReturnStatement) interface{} {
	if node.ReturnValue != nil {
		return i.evalExpression(node.ReturnValue)
	}
	return types.Nothing{}
}

func (i *Interpreter) evalProcedureStatement(node *parser.ProcedureStatement) interface{} {
	// Create a procedure value and store it in the current scope
	procedure := &Procedure{
		Name:       node.Name,
		Parameters: node.Parameters,
		Body:       node.Body,
		Closure:    i.scope, // Capture current scope as closure
	}

	// Store the procedure in the current scope
	i.scope.Set(node.Name, procedure)

	return types.Nothing{} // Procedure definition returns Nothing
}

// Procedure represents a user-defined procedure
type Procedure struct {
	Name       string
	Parameters []string
	Body       parser.Statement
	Closure    *Scope // Captured scope for closures
}

// Call invokes a procedure with given arguments
func (p *Procedure) Call(interpreter *Interpreter, args []interface{}) interface{} {
	// Check argument count
	if len(args) != len(p.Parameters) {
		return types.Unknown{
			Reason: fmt.Sprintf("procedure '%s' expects %d arguments, got %d",
				p.Name, len(p.Parameters), len(args)),
		}
	}

	// Create new scope for procedure execution
	procScope := p.Closure.NewChildScope()

	// Bind parameters to arguments
	for i, param := range p.Parameters {
		procScope.Set(param, args[i])
	}

	// Save current scope and switch to procedure scope
	oldScope := interpreter.scope
	interpreter.scope = procScope

	// Execute procedure body
	result := interpreter.evalStatement(p.Body)

	// Restore original scope
	interpreter.scope = oldScope

	return result
}

func (i *Interpreter) evalInstantiationStatement(node *parser.InstantiationStatement) interface{} {
	// Create a new record with the specified type
	fields := make(map[string]interface{})

	// Add type information
	fields["_type"] = types.Text{Value: node.Type}

	// Evaluate the properties if present
	if node.Properties != nil {
		propertiesResult := i.evalRecordExpression(node.Properties)

		// Unknown propagation
		if unknown, ok := propertiesResult.(types.Unknown); ok {
			return unknown
		}

		// Add all properties to the fields
		if record, ok := propertiesResult.(types.Record); ok {
			for key, value := range record.Fields {
				fields[key] = value
			}
		}
	}

	return types.Record{Fields: fields}
}

func (i *Interpreter) evalCallExpression(node *parser.CallExpression) interface{} {
	// Evaluate arguments first
	args := make([]interface{}, len(node.Arguments))
	for idx, arg := range node.Arguments {
		args[idx] = i.evalExpression(arg)

		// Unknown propagation
		if unknown, ok := args[idx].(types.Unknown); ok {
			return unknown
		}
	}

	// Get function name - for now assume it's a PathExpression
	var functionName string
	if pathExpr, ok := node.Function.(*parser.PathExpression); ok && len(pathExpr.Parts) == 1 {
		functionName = pathExpr.Parts[0]
	} else {
		return types.Unknown{Reason: "complex function expressions not yet supported"}
	}

	// Check for built-in functions first
	switch functionName {
	case "len":
		return i.evalLenFunction(args)
	case "text":
		return i.evalTextFunction(args)
	case "number":
		return i.evalNumberFunction(args)
	case "add":
		return i.evalAddFunction(args)
	case "output":
		return i.evalOutputFunction(args)
	// String operations
	case "upper":
		return i.evalUpperFunction(args)
	case "lower":
		return i.evalLowerFunction(args)
	case "trim":
		return i.evalTrimFunction(args)
	// Math operations
	case "abs":
		return i.evalAbsFunction(args)
	case "floor":
		return i.evalFloorFunction(args)
	case "ceil":
		return i.evalCeilFunction(args)
	case "max":
		return i.evalMaxFunction(args)
	case "min":
		return i.evalMinFunction(args)
	default:
		// Look up user-defined procedures
		if value, found := i.scope.Get(functionName); found {
			if procedure, ok := value.(*Procedure); ok {
				return procedure.Call(i, args)
			} else {
				return types.Unknown{Reason: fmt.Sprintf("'%s' is not a procedure", functionName)}
			}
		}
		return types.Unknown{Reason: fmt.Sprintf("unknown function: %s", functionName)}
	}
}

// Built-in function implementations

func (i *Interpreter) evalLenFunction(args []interface{}) interface{} {
	if len(args) != 1 {
		return types.Unknown{Reason: "len() requires exactly 1 argument"}
	}

	switch v := args[0].(type) {
	case types.Text:
		return types.Number{Value: float64(len(v.Value))}
	case types.List:
		return types.Number{Value: float64(len(v.Elements))}
	case types.Record:
		return types.Number{Value: float64(len(v.Fields))}
	default:
		return types.Unknown{Reason: fmt.Sprintf("len() not supported for %T", v)}
	}
}

func (i *Interpreter) evalTextFunction(args []interface{}) interface{} {
	if len(args) != 1 {
		return types.Unknown{Reason: "text() requires exactly 1 argument"}
	}

	result := types.CoerceToText(args[0])
	if !result.Ok {
		return result.Value // Return the Unknown error
	}

	return result.Value
}

func (i *Interpreter) evalNumberFunction(args []interface{}) interface{} {
	if len(args) != 1 {
		return types.Unknown{Reason: "number() requires exactly 1 argument"}
	}

	result := types.CoerceToNumber(args[0])
	if !result.Ok {
		return result.Value // Return the Unknown error
	}

	return result.Value
}

func (i *Interpreter) evalAddFunction(args []interface{}) interface{} {
	if len(args) == 0 {
		return types.Number{Value: 0.0}
	}

	// Sum all arguments
	total := types.Number{Value: 0.0}
	for _, arg := range args {
		result := types.Add(total, arg)
		if unknown, ok := result.(types.Unknown); ok {
			return unknown
		}
		total = result.(types.Number)
	}

	return total
}

func (i *Interpreter) evalOutputFunction(args []interface{}) interface{} {
	// Output function prints values (for debugging/logging)
	// In a real implementation, this might write to a log or stdout
	// For now, just return the first argument
	if len(args) > 0 {
		return args[0]
	}
	return types.Nothing{}
}

// String operation functions

func (i *Interpreter) evalUpperFunction(args []interface{}) interface{} {
	if len(args) != 1 {
		return types.Unknown{Reason: "upper() requires exactly 1 argument"}
	}

	textResult := types.CoerceToText(args[0])
	if !textResult.Ok {
		return textResult.Value
	}

	text := textResult.Value.(types.Text)
	return types.Text{Value: strings.ToUpper(text.Value)}
}

func (i *Interpreter) evalLowerFunction(args []interface{}) interface{} {
	if len(args) != 1 {
		return types.Unknown{Reason: "lower() requires exactly 1 argument"}
	}

	textResult := types.CoerceToText(args[0])
	if !textResult.Ok {
		return textResult.Value
	}

	text := textResult.Value.(types.Text)
	return types.Text{Value: strings.ToLower(text.Value)}
}

func (i *Interpreter) evalTrimFunction(args []interface{}) interface{} {
	if len(args) != 1 {
		return types.Unknown{Reason: "trim() requires exactly 1 argument"}
	}

	textResult := types.CoerceToText(args[0])
	if !textResult.Ok {
		return textResult.Value
	}

	text := textResult.Value.(types.Text)
	return types.Text{Value: strings.TrimSpace(text.Value)}
}

// Math operation functions

func (i *Interpreter) evalAbsFunction(args []interface{}) interface{} {
	if len(args) != 1 {
		return types.Unknown{Reason: "abs() requires exactly 1 argument"}
	}

	numberResult := types.CoerceToNumber(args[0])
	if !numberResult.Ok {
		return numberResult.Value
	}

	number := numberResult.Value.(types.Number)
	if number.Value < 0 {
		return types.Number{Value: -number.Value}
	}
	return number
}

func (i *Interpreter) evalFloorFunction(args []interface{}) interface{} {
	if len(args) != 1 {
		return types.Unknown{Reason: "floor() requires exactly 1 argument"}
	}

	numberResult := types.CoerceToNumber(args[0])
	if !numberResult.Ok {
		return numberResult.Value
	}

	number := numberResult.Value.(types.Number)
	return types.Number{Value: math.Floor(number.Value)}
}

func (i *Interpreter) evalCeilFunction(args []interface{}) interface{} {
	if len(args) != 1 {
		return types.Unknown{Reason: "ceil() requires exactly 1 argument"}
	}

	numberResult := types.CoerceToNumber(args[0])
	if !numberResult.Ok {
		return numberResult.Value
	}

	number := numberResult.Value.(types.Number)
	return types.Number{Value: math.Ceil(number.Value)}
}

func (i *Interpreter) evalMaxFunction(args []interface{}) interface{} {
	if len(args) == 0 {
		return types.Unknown{Reason: "max() requires at least 1 argument"}
	}

	// Convert all arguments to numbers
	var max float64
	first := true

	for i, arg := range args {
		numberResult := types.CoerceToNumber(arg)
		if !numberResult.Ok {
			return types.Unknown{Reason: fmt.Sprintf("max() argument %d cannot be converted to number", i+1)}
		}

		value := numberResult.Value.(types.Number).Value
		if first || value > max {
			max = value
			first = false
		}
	}

	return types.Number{Value: max}
}

func (i *Interpreter) evalMinFunction(args []interface{}) interface{} {
	if len(args) == 0 {
		return types.Unknown{Reason: "min() requires at least 1 argument"}
	}

	// Convert all arguments to numbers
	var min float64
	first := true

	for i, arg := range args {
		numberResult := types.CoerceToNumber(arg)
		if !numberResult.Ok {
			return types.Unknown{Reason: fmt.Sprintf("min() argument %d cannot be converted to number", i+1)}
		}

		value := numberResult.Value.(types.Number).Value
		if first || value < min {
			min = value
			first = false
		}
	}

	return types.Number{Value: min}
}

func (i *Interpreter) evalBracketFilter(node *parser.BracketFilterExpression) interface{} {
	// Evaluate the base expression (should be a List or Record)
	base := i.evalExpression(node.Left)

	if unknown, ok := base.(types.Unknown); ok {
		return unknown
	}

	// Handle different base types
	switch baseValue := base.(type) {
	case types.List:
		return i.filterList(baseValue, node.Filters)
	case types.Record:
		return i.filterRecord(baseValue, node.Filters)
	default:
		return types.Unknown{Reason: fmt.Sprintf("cannot apply bracket filter to %T", baseValue)}
	}
}

// filterList filters a list based on filter conditions
func (i *Interpreter) filterList(list types.List, filters []parser.Expression) interface{} {
	var filtered []interface{}

	// For each element in the list
	for _, element := range list.Elements {
		// Create a temporary scope with 'item' pointing to current element
		childScope := i.scope.NewChildScope()
		childScope.Set("item", element)
		oldScope := i.scope
		i.scope = childScope

		matches := true

		// Check all filter conditions
		for _, filter := range filters {
			result := i.evalExpression(filter)

			// Unknown propagation
			if unknown, ok := result.(types.Unknown); ok {
				i.scope = oldScope
				return unknown
			}

			// Check if result is truthy
			if !isTruthy(result) {
				matches = false
				break
			}
		}

		// Restore scope
		i.scope = oldScope

		if matches {
			filtered = append(filtered, element)
		}
	}

	return types.List{Elements: filtered}
}

// filterRecord filters a record based on field conditions
func (i *Interpreter) filterRecord(record types.Record, filters []parser.Expression) interface{} {
	filtered := make(map[string]interface{})

	// For each field in the record
	for key, value := range record.Fields {
		// Create a temporary scope with field access
		childScope := i.scope.NewChildScope()
		childScope.Set("field", value)
		childScope.Set("key", types.Text{Value: key})
		oldScope := i.scope
		i.scope = childScope

		matches := true

		// Check all filter conditions
		for _, filter := range filters {
			result := i.evalExpression(filter)

			// Unknown propagation
			if unknown, ok := result.(types.Unknown); ok {
				i.scope = oldScope
				return unknown
			}

			// Check if result is truthy
			if !isTruthy(result) {
				matches = false
				break
			}
		}

		// Restore scope
		i.scope = oldScope

		if matches {
			filtered[key] = value
		}
	}

	return types.Record{Fields: filtered}
}

func (i *Interpreter) evalRecordExpression(node *parser.RecordExpression) interface{} {
	fields := make(map[string]interface{})

	for keyExpr, valueExpr := range node.Pairs {
		// Evaluate the key (should be a string)
		keyResult := i.evalExpression(keyExpr)
		if unknown, ok := keyResult.(types.Unknown); ok {
			return unknown
		}

		// Convert key to string
		var key string
		if text, ok := keyResult.(types.Text); ok {
			key = text.Value
		} else {
			// Coerce key to text
			textResult := types.CoerceToText(keyResult)
			if !textResult.Ok {
				return textResult.Value
			}
			key = textResult.Value.(types.Text).Value
		}

		// Evaluate the value
		value := i.evalExpression(valueExpr)
		if unknown, ok := value.(types.Unknown); ok {
			return unknown
		}

		fields[key] = value
	}

	return types.Record{Fields: fields}
}

func (i *Interpreter) evalListExpression(node *parser.ListExpression) interface{} {
	var elements []interface{}

	for _, element := range node.Elements {
		value := i.evalExpression(element)

		// Unknown propagation - if any element is Unknown, return it
		if unknown, ok := value.(types.Unknown); ok {
			return unknown
		}

		elements = append(elements, value)
	}

	return types.List{Elements: elements}
}

// Utility functions

// isTruthy determines if a value is "truthy" in MBL (copied from coerce.go for local use)
func isTruthy(value interface{}) bool {
	switch v := value.(type) {
	case types.Number:
		return v.Value != 0.0
	case types.Text:
		return v.Value != ""
	case types.Nothing:
		return false
	case types.Unknown:
		return false
	case types.Anything:
		return true
	case types.Boolean:
		return v.Value
	case types.List:
		return len(v.Elements) > 0
	case types.Record:
		return len(v.Fields) > 0
	case *Procedure:
		return true // Procedures are truthy
	default:
		return true // Most values are truthy
	}
}

// isNumeric checks if a value is numeric
func isNumeric(value interface{}) bool {
	switch value.(type) {
	case types.Number:
		return true
	case types.Money:
		return true // Money can be used in numeric contexts
	default:
		return false
	}
}

// negate performs unary negation on a numeric value
func negate(value interface{}) interface{} {
	switch v := value.(type) {
	case types.Number:
		return types.Number{Value: -v.Value}
	case types.Money:
		return types.Money{Amount: -v.Amount, CurrencyCode: v.CurrencyCode}
	default:
		return types.Unknown{Reason: fmt.Sprintf("cannot negate non-numeric value: %T", value)}
	}
}

// parseTimeString parses a time string from MBL format
func parseTimeString(timeStr string) (types.Time, error) {
	// Remove @ prefix if present
	if strings.HasPrefix(timeStr, "@") {
		timeStr = timeStr[1:]
	}

	// Try different time formats based on length and content
	layouts := []struct {
		layout    string
		precision byte
	}{
		{"2006", types.PrecisionYear},
		{"2006-01", types.PrecisionMonth},
		{"2006-01-02", types.PrecisionDay},
		{"2006-01-02 15", types.PrecisionHour},
		{"2006-01-02 15:04", types.PrecisionMinute},
		{"2006-01-02 15:04:05", types.PrecisionSecond},
		{"2006-01-02 15:04:05.000000", types.PrecisionSubsecond},
	}

	for _, layout := range layouts {
		if t, err := time.ParseInLocation(layout.layout, timeStr, time.UTC); err == nil {
			return types.Time{
				Timestamp: t,
				Precision: layout.precision,
			}, nil
		}
	}

	return types.Time{}, fmt.Errorf("cannot parse time: %s", timeStr)
}

// parseMoneyString parses a money string from MBL format
func parseMoneyString(moneyStr string) (types.Money, error) {
	// Remove ¤ prefix if present
	if strings.HasPrefix(moneyStr, "¤") {
		moneyStr = moneyStr[1:]
	}

	// Split on space to separate amount and currency
	parts := strings.Fields(moneyStr)
	if len(parts) != 2 {
		return types.Money{}, fmt.Errorf("money format must be 'amount currency': %s", moneyStr)
	}

	amount, err := strconv.ParseFloat(parts[0], 64)
	if err != nil {
		return types.Money{}, fmt.Errorf("invalid money amount: %s", parts[0])
	}

	currency := parts[1]
	if len(currency) != 3 {
		return types.Money{}, fmt.Errorf("currency code must be 3 characters: %s", currency)
	}

	return types.Money{
		Amount:       amount,
		CurrencyCode: currency,
	}, nil
}

// mblToStorage converts an MBL type to storage.Value
func mblToStorage(value interface{}) (storage.Value, error) {
	typesValue, err := types.CreateValue(value)
	if err != nil {
		return storage.Value{}, err
	}

	// storage.Value and types.Value have the same structure
	return storage.Value{
		TypeTag: typesValue.TypeTag,
		Data:    typesValue.Data,
	}, nil
}

// storageToMBL converts storage.Value to MBL type
func storageToMBL(value storage.Value) (interface{}, error) {
	// Convert storage.Value to types.Value
	typesValue := types.Value{
		TypeTag: value.TypeTag,
		Data:    value.Data,
	}

	// Deserialize to MBL type
	return types.DeserializeValue(typesValue)
}

// convertToMBLType converts raw values from parser to proper MBL types
func convertToMBLType(value interface{}) interface{} {
	switch v := value.(type) {
	case int:
		return types.Number{Value: float64(v)}
	case int64:
		return types.Number{Value: float64(v)}
	case float64:
		return types.Number{Value: v}
	case string:
		// Remove quotes if they exist (in case parser stored raw token)
		s := v
		if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
			s = s[1 : len(s)-1]
		}
		return types.Text{Value: s}
	case bool:
		return types.Boolean{Value: v}
	case types.Text, types.Number, types.Boolean, types.Time, types.Money, types.Nothing, types.Unknown, types.Anything, types.List, types.Record:
		// Already proper MBL type
		return v
	default:
		// Unknown type, wrap as text
		return types.Text{Value: fmt.Sprintf("%v", v)}
	}
}

