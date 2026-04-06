// Package interpreter implements MBL program execution against the storage engine
package interpreter

import (
	"crypto/rand"
	"encoding/xml"
	"fmt"
	"io/ioutil"
	"math"
	"math/big"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/solifugus/amorphdb/internal/ari"
	"github.com/solifugus/amorphdb/internal/mbl/lexer"
	"github.com/solifugus/amorphdb/internal/mbl/parser"
	"github.com/solifugus/amorphdb/internal/storage"
	"github.com/solifugus/amorphdb/internal/types"
)

// CommitBuffer holds persistent writes to be flushed as a batch
type CommitBuffer struct {
	writes []PendingWrite
	limit  int
}

// PendingWrite represents a write operation staged for batched commit
type PendingWrite struct {
	path   []string
	value  storage.Value
	author uint64
}

// DefaultCommitBufferLimit is the default limit for writes per execution
const DefaultCommitBufferLimit = 10000

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

// Update updates a variable in the scope where it already exists, or creates it locally if not found
func (s *Scope) Update(name string, value interface{}) {
	if _, found := s.local[name]; found {
		// Variable exists in local scope, update it
		s.local[name] = value
		return
	}
	if s.parent != nil {
		// Check if variable exists in parent scope
		if _, found := s.parent.Get(name); found {
			// Variable exists in parent scope, update it there
			s.parent.Update(name, value)
			return
		}
	}
	// Variable doesn't exist anywhere, create it locally
	s.local[name] = value
}

// Interpreter executes MBL programs against the storage engine
type Interpreter struct {
	scope        *Scope
	errors       []string      // Accumulated errors during execution
	commitBuffer *CommitBuffer // Staged writes for batched commit
	coordinator  *CommitCoordinator // Optional coordinator for cross-execution batching
}

// New creates a new interpreter instance
func New(tree storage.Tree, agent uint64) *Interpreter {
	return &Interpreter{
		scope: NewScope(tree, agent),
		commitBuffer: &CommitBuffer{
			writes: make([]PendingWrite, 0),
			limit:  DefaultCommitBufferLimit,
		},
		coordinator: nil, // No coordinator by default
	}
}

// NewWithCoordinator creates an interpreter instance with a commit coordinator
func NewWithCoordinator(tree storage.Tree, agent uint64, coordinator *CommitCoordinator) *Interpreter {
	return &Interpreter{
		scope: NewScope(tree, agent),
		commitBuffer: &CommitBuffer{
			writes: make([]PendingWrite, 0),
			limit:  DefaultCommitBufferLimit,
		},
		coordinator: coordinator,
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
		// Rollback only occurs if the final result is an unhandled Unknown
	}

	// Check if final result should trigger rollback
	if unknown, ok := result.(types.Unknown); ok && i.shouldRollback(unknown) {
		i.discardBuffer()
		return result, nil
	}

	// Flush staged writes on normal completion
	if err := i.flushBuffer(); err != nil {
		// If flush fails, return Unknown with the error
		return types.Unknown{Reason: fmt.Sprintf("commit failed: %v", err)}, nil
	}

	return result, nil
}

// Errors returns accumulated interpretation errors
func (i *Interpreter) Errors() []string {
	return i.errors
}

// GetWrittenPaths returns the list of paths that have been written in the current commit buffer
func (i *Interpreter) GetWrittenPaths() [][]string {
	var paths [][]string
	for _, write := range i.commitBuffer.writes {
		paths = append(paths, write.path)
	}
	return paths
}

// EvaluateExpression parses and evaluates a single MBL expression
func (i *Interpreter) EvaluateExpression(input string) (interface{}, error) {
	// Parse as expression
	l := lexer.New(input)
	p := parser.New(l)

	// Parse as a single expression by creating a simple program
	program := p.ParseProgram()
	if len(p.Errors()) > 0 {
		return nil, fmt.Errorf("parse errors: %v", strings.Join(p.Errors(), "; "))
	}

	// Execute and return result
	result, err := i.Interpret(program)
	if err != nil {
		return nil, err
	}

	return result, nil
}

// ExecuteStatement parses and executes a single MBL statement
func (i *Interpreter) ExecuteStatement(input string) (interface{}, error) {
	// Parse as statement
	l := lexer.New(input)
	p := parser.New(l)

	// Parse as a program (which can contain statements)
	program := p.ParseProgram()
	if len(p.Errors()) > 0 {
		return nil, fmt.Errorf("parse errors: %v", strings.Join(p.Errors(), "; "))
	}

	// Execute and return result
	result, err := i.Interpret(program)
	if err != nil {
		return nil, err
	}

	return result, nil
}

// stageWrite adds a write operation to the commit buffer
func (i *Interpreter) stageWrite(path []string, value storage.Value, author uint64) interface{} {
	// Check buffer limit
	if len(i.commitBuffer.writes) >= i.commitBuffer.limit {
		return types.Unknown{Reason: "commit buffer exceeded"}
	}

	// Stage the write
	pendingWrite := PendingWrite{
		path:   path,
		value:  value,
		author: author,
	}

	i.commitBuffer.writes = append(i.commitBuffer.writes, pendingWrite)
	return nil
}

// flushBuffer commits all staged writes as a batch
func (i *Interpreter) flushBuffer() error {
	if len(i.commitBuffer.writes) == 0 {
		return nil // Nothing to flush
	}

	// If coordinator is available, stage writes for coordinated batching
	if i.coordinator != nil {
		err := i.coordinator.StageWrites(i.commitBuffer.writes)
		if err != nil {
			i.discardBuffer()
			return fmt.Errorf("failed to stage writes with coordinator: %w", err)
		}
		// Clear local buffer after staging with coordinator
		i.commitBuffer.writes = i.commitBuffer.writes[:0]
		return nil
	}

	// Fallback: direct flush without coordination (for standalone execution)
	return i.flushBufferDirect()
}

// flushBufferDirect commits staged writes directly without coordination
func (i *Interpreter) flushBufferDirect() error {
	// Apply all writes directly to local storage
	for _, write := range i.commitBuffer.writes {
		err := i.scope.tree.Write(write.path, write.value, write.author)
		if err != nil {
			// On write failure, discard remaining writes and return error
			i.discardBuffer()
			return fmt.Errorf("write failed for path %v: %w", write.path, err)
		}
	}

	// Clear buffer after successful flush
	i.commitBuffer.writes = i.commitBuffer.writes[:0]
	return nil
}

// discardBuffer clears all staged writes without committing them
func (i *Interpreter) discardBuffer() {
	i.commitBuffer.writes = i.commitBuffer.writes[:0]
}

// getStagedWrite checks if a path has a staged write in the commit buffer
func (i *Interpreter) getStagedWrite(path []string) *storage.Value {
	// Check writes in reverse order (most recent first)
	for idx := len(i.commitBuffer.writes) - 1; idx >= 0; idx-- {
		write := i.commitBuffer.writes[idx]
		if pathsEqual(write.path, path) {
			return &write.value
		}
	}
	return nil
}

// pathsEqual compares two path slices for equality
func pathsEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// shouldRollback determines if an Unknown should trigger buffer rollback
func (i *Interpreter) shouldRollback(unknown types.Unknown) bool {
	// Per heartbeat atomicity spec: rollback occurs when an Unknown
	// propagates out unhandled. For MBL, any Unknown that escapes
	// as the final result represents an unhandled error condition.
	// This includes both system errors and unhandled business logic errors.

	reason := unknown.Reason

	// System/infrastructure errors always trigger rollback
	if strings.Contains(reason, "commit buffer exceeded") ||
	   strings.Contains(reason, "commit failed") ||
	   strings.Contains(reason, "failed to read path") ||
	   strings.Contains(reason, "failed to write path") ||
	   strings.Contains(reason, "storage error") ||
	   strings.Contains(reason, "parse error") ||
	   strings.Contains(reason, "interpreter error") {
		return true
	}

	// Unhandled function calls and similar runtime errors should trigger rollback
	// when they escape as the final result (not when assigned/handled)
	if strings.Contains(reason, "unknown function") ||
	   strings.Contains(reason, "undefined_function") ||
	   strings.Contains(reason, "unsupported statement type") ||
	   strings.Contains(reason, "unsupported expression type") {
		return true
	}

	// Pure data/type errors should NOT trigger rollback (resilient computation)
	// Examples: undefined variable, field not found, division by zero, offline
	// These are expected in resilient computation and should not rollback transactions
	return false
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
	case *parser.EmbedDirectiveStatement:
		return i.evalEmbedDirectiveStatement(node)
	case *parser.CatchStatement:
		return i.evalCatchStatement(node)
	default:
		return types.Unknown{Reason: fmt.Sprintf("unsupported statement type: %T", node)}
	}
}

// evalEmbedDirectiveStatement evaluates embed directives like "embed my.stamp"
func (i *Interpreter) evalEmbedDirectiveStatement(node *parser.EmbedDirectiveStatement) interface{} {
	// For now, embed directives are stored as special metadata
	// The actual embedding logic happens during record resolution
	// TODO(spec): Implement full embed resolution in storage layer

	// Evaluate the path to ensure it's valid
	pathResult := i.evalExpression(node.Path)
	if unknown, ok := pathResult.(types.Unknown); ok {
		return unknown
	}

	// For now, embed directives don't produce a direct result
	// They affect record resolution which happens at read time
	return types.Nothing{}
}

// evalCatchStatement evaluates catch-else exception handling statements
func (i *Interpreter) evalCatchStatement(node *parser.CatchStatement) interface{} {
	// Execute the try body
	result := i.evalBlockStatement(node.TryBody)

	// If result is normal (not Unknown or Queued), return it directly
	if unknown, ok := result.(types.Unknown); !ok {
		if queued, ok := result.(types.Queued); !ok {
			return result
		} else {
			// Handle Queued result - find matching else queued clause
			for _, clause := range node.ElseClauses {
				if clause.ErrorType == "queued" {
					// Set queued value in scope for the handler to access
					i.scope.Set("queued", queued)
					return i.evalBlockStatement(clause.Body)
				}
			}
			// No else queued handler found, return the Queued value
			return queued
		}
	} else {
		// Handle Unknown result - find matching else unknown clause
		for _, clause := range node.ElseClauses {
			if clause.ErrorType == "unknown" {
				// Set unknown value in scope for the handler to access
				i.scope.Set("unknown", unknown)
				return i.evalBlockStatement(clause.Body)
			}
		}
		// No else unknown handler found, return the Unknown value
		return unknown
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
	case *parser.ProjectionExpression:
		return i.evalProjectionExpression(node)
	case *parser.RecordExpression:
		return i.evalRecordExpression(node)
	case *parser.ListExpression:
		return i.evalListExpression(node)
	case *parser.CollectionOperationExpression:
		return i.evalCollectionOperationExpression(node)
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

	// Queued literals (prefixed with ?)
	if strings.HasPrefix(literal, "?") {
		reason := strings.TrimPrefix(literal, "?")
		if reason == "" {
			reason = "unknown"
		}
		return types.Queued{Info: reason}
	}

	// Special values
	switch literal {
	case "Nothing":
		return types.Nothing{}
	case "Unknown":
		return types.Unknown{Reason: "explicit Unknown"}
	case "Queued":
		return types.Queued{Info: "explicit Queued"}
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

	// Check for local variables first (single part paths)
	if len(node.Parts) == 1 {
		if value, found := i.scope.Get(node.Parts[0]); found {
			return value
		}
	}

	// Check for local record field access (multi-part paths)
	if len(node.Parts) > 1 {
		if baseValue, found := i.scope.Get(node.Parts[0]); found {
			return i.accessRecordFields(baseValue, node.Parts[1:])
		}
	}

	// Resolve special path roots
	path := i.resolvePath(node.Parts)

	// Check staging buffer first for uncommitted writes
	if stagedValue := i.getStagedWrite(path); stagedValue != nil {
		// Convert storage.Value to MBL type
		mblValue, err := storageToMBL(*stagedValue)
		if err != nil {
			return types.Unknown{Reason: fmt.Sprintf("failed to convert staged value: %v", err)}
		}
		return mblValue
	}

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

// accessRecordFields recursively accesses fields from record values
func (i *Interpreter) accessRecordFields(baseValue interface{}, fieldNames []string) interface{} {
	currentValue := baseValue

	for _, fieldName := range fieldNames {
		record, ok := currentValue.(types.Record)
		if !ok {
			return types.Unknown{Reason: fmt.Sprintf("cannot access field '%s' on non-record type", fieldName)}
		}

		fieldValue, exists := record.Fields[fieldName]
		if !exists {
			return types.Unknown{Reason: fmt.Sprintf("field '%s' not found in record", fieldName)}
		}

		currentValue = fieldValue
	}

	return currentValue
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
	case "^":
		return types.Power(left, right)

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

	// Ensure value is properly converted to MBL type
	value = convertToMBLType(value)

	// For simple variable names, update in the scope where they exist
	if len(node.Name.Parts) == 1 && !i.isSpecialPath(node.Name.Parts[0]) {
		i.scope.Update(node.Name.Parts[0], value)
		return value
	}

	// For paths, write to storage
	path := i.resolvePath(node.Name.Parts)

	// Auto-create intermediate nodes if they don't exist (Step 3: Recursive assignment)
	if len(path) > 1 {
		err := i.ensureIntermediatePath(path[:len(path)-1])
		if err != nil {
			return types.Unknown{Reason: fmt.Sprintf("failed to create intermediate path: %v", err)}
		}
	}

	// Convert MBL type to storage.Value
	storageValue, err := mblToStorage(value)
	if err != nil {
		return types.Unknown{Reason: fmt.Sprintf("failed to convert value for storage: %v", err)}
	}

	// Stage write for batched commit instead of immediate write
	result := i.stageWrite(path, storageValue, i.scope.agent)
	if result != nil {
		if unknown, ok := result.(types.Unknown); ok {
			return unknown
		}
		return types.Unknown{Reason: fmt.Sprintf("failed to stage write to path %v", path)}
	}

	return value
}

// isSpecialPath checks if a path starts with special identifiers
func (i *Interpreter) isSpecialPath(name string) bool {
	return name == "my" || name == "world" || name == "~"
}

// ensureIntermediatePath ensures all intermediate nodes exist by creating empty records if needed
func (i *Interpreter) ensureIntermediatePath(path []string) error {
	if len(path) == 0 {
		return nil
	}

	// Walk through each segment of the path, creating intermediate nodes as needed
	for segmentCount := 1; segmentCount <= len(path); segmentCount++ {
		intermediatePath := path[:segmentCount]

		// Check if this intermediate path already exists
		value, err := i.scope.tree.Read(intermediatePath)
		if err == nil {
			// Check if the value is Nothing (which means path doesn't exist in MockTree)
			if value.TypeTag == types.TypeNothing {
				// Path returns Nothing, treat as non-existent
			} else {
				// Path exists with actual value, continue to next segment
				continue
			}
		}

		// Path doesn't exist, create an empty record node
		emptyRecord := types.Record{Fields: map[string]interface{}{}}

		// Convert MBL type to storage.Value
		storageValue, err := mblToStorage(emptyRecord)
		if err != nil {
			return fmt.Errorf("failed to convert empty record for intermediate path %v: %w", intermediatePath, err)
		}

		// Stage write for the intermediate path
		result := i.stageWrite(intermediatePath, storageValue, i.scope.agent)
		if result != nil {
			if unknown, ok := result.(types.Unknown); ok {
				return fmt.Errorf("failed to create intermediate path %v: %s", intermediatePath, unknown.Reason)
			}
			return fmt.Errorf("failed to create intermediate path %v", intermediatePath)
		}
	}

	return nil
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
	const maxIterations = 100000 // Safety limit to prevent infinite loops
	iterations := 0

	for {
		// Safety check: prevent runaway loops
		iterations++
		if iterations > maxIterations {
			return types.Unknown{Reason: fmt.Sprintf("while loop exceeded maximum iterations (%d)", maxIterations)}
		}

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

		// Safety check: if commit buffer is getting large, flush periodically
		if iterations%1000 == 0 && len(i.commitBuffer.writes) > 5000 {
			err := i.flushBuffer()
			if err != nil {
				return types.Unknown{Reason: fmt.Sprintf("failed to flush buffer during loop: %v", err)}
			}
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

	// Safety check: prevent iteration over extremely large lists
	const maxForLoopElements = 100000
	if len(list.Elements) > maxForLoopElements {
		return types.Unknown{Reason: fmt.Sprintf("for loop list too large (%d elements, max %d)", len(list.Elements), maxForLoopElements)}
	}

	// Iterate over elements
	for idx, element := range list.Elements {
		// Set the loop variables
		if len(node.Variables) >= 1 {
			i.scope.Set(node.Variables[0], element) // First variable is the element
		}
		if len(node.Variables) >= 2 {
			i.scope.Set(node.Variables[1], types.Number{Value: float64(idx)}) // Second variable is the index
		}

		// Execute the loop body
		result = i.evalStatement(node.Body)

		// Check for Unknown (error) - break on error
		if _, ok := result.(types.Unknown); ok {
			break
		}

		// Safety check: flush buffer periodically for large iterations
		if idx%1000 == 0 && len(i.commitBuffer.writes) > 5000 {
			err := i.flushBuffer()
			if err != nil {
				return types.Unknown{Reason: fmt.Sprintf("failed to flush buffer during for loop: %v", err)}
			}
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
	Body       *parser.BlockStatement
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

	// Execute procedure body as a block statement
	result := interpreter.evalBlockStatement(p.Body)

	// Restore original scope
	interpreter.scope = oldScope

	return result
}

func (i *Interpreter) evalInstantiationStatement(node *parser.InstantiationStatement) interface{} {
	// Create a new record with the specified type
	fields := make(map[string]interface{})

	// Add type information
	fields["_type"] = types.Text{Value: node.Type}

	// Handle inheritance from mixed sources
	if len(node.Sources) > 0 {
		// Get the global modifier (default to "copy" if not specified)
		globalModifier := "copy"
		if node.Modifier != nil {
			globalModifier = *node.Modifier
		}

		// Process each source (template or inline record)
		for _, source := range node.Sources {
			var sourceRecord types.Record

			if source.TemplateName != nil {
				// Template name source
				sourceValue, found := i.scope.Get(*source.TemplateName)
				if !found {
					return types.Unknown{Reason: fmt.Sprintf("template source '%s' not found", *source.TemplateName)}
				}

				// Source must be a Record
				var ok bool
				sourceRecord, ok = sourceValue.(types.Record)
				if !ok {
					return types.Unknown{Reason: fmt.Sprintf("template source '%s' is not a record", *source.TemplateName)}
				}
			} else if source.InlineRecord != nil {
				// Inline record source
				recordResult := i.evalRecordExpression(source.InlineRecord)

				// Unknown propagation
				if unknown, ok := recordResult.(types.Unknown); ok {
					return unknown
				}

				var ok bool
				sourceRecord, ok = recordResult.(types.Record)
				if !ok {
					return types.Unknown{Reason: "inline record evaluation failed"}
				}
			} else {
				return types.Unknown{Reason: "invalid instantiation source"}
			}

			// Apply inheritance with per-attribute modifier support
			err := i.applyInheritanceWithModifiers(fields, sourceRecord.Fields, globalModifier)
			if err != nil {
				return types.Unknown{Reason: fmt.Sprintf("inheritance failed: %v", err)}
			}
		}
	}

	// Evaluate the properties if present (these override inherited values)
	if node.Properties != nil {
		propertiesResult := i.evalRecordExpression(node.Properties)

		// Unknown propagation
		if unknown, ok := propertiesResult.(types.Unknown); ok {
			return unknown
		}

		// Add all properties to the fields (properties override inheritance)
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

	// Get function name - handle both simple and complex paths
	var functionName string
	var functionPath []string
	if pathExpr, ok := node.Function.(*parser.PathExpression); ok {
		functionPath = pathExpr.Parts
		if len(pathExpr.Parts) == 1 {
			functionName = pathExpr.Parts[0]
		} else {
			// Handle complex paths like my.computer.files.import
			functionName = strings.Join(pathExpr.Parts, ".")
		}
	} else {
		return types.Unknown{Reason: "function must be a path expression"}
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
	case "round":
		return i.evalRoundFunction(args)
	// Text operations
	case "substring":
		return i.evalSubstringFunction(args)
	case "find":
		return i.evalFindFunction(args)
	case "replace":
		return i.evalReplaceFunction(args)
	case "split":
		return i.evalSplitFunction(args)
	// Reset expression functions
	case "random":
		return i.evalRandomFunction(args)
	case "now":
		return i.evalNowFunction(args)
	case "uuid":
		return i.evalUuidFunction(args)
	case "asset":
		return i.evalAssetFunction(args)
	default:
		// Handle computer library procedures
		if len(functionPath) >= 3 && functionPath[0] == "my" && functionPath[1] == "computer" {
			return i.evalComputerLibraryProcedure(functionPath, args)
		}
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

// Reset expression built-in functions

func (i *Interpreter) evalRandomFunction(args []interface{}) interface{} {
	switch len(args) {
	case 0:
		// random() - return random float between 0 and 1
		randomFloat, err := rand.Int(rand.Reader, big.NewInt(1000000))
		if err != nil {
			return types.Unknown{Reason: "failed to generate random number"}
		}
		return types.Number{Value: float64(randomFloat.Int64()) / 1000000.0}

	case 1:
		// random(max) - return random integer from 0 to max-1
		maxResult := types.CoerceToNumber(args[0])
		if !maxResult.Ok {
			return types.Unknown{Reason: "random() max argument must be a number"}
		}
		max := int64(maxResult.Value.(types.Number).Value)
		if max <= 0 {
			return types.Unknown{Reason: "random() max must be positive"}
		}

		randomInt, err := rand.Int(rand.Reader, big.NewInt(max))
		if err != nil {
			return types.Unknown{Reason: "failed to generate random number"}
		}
		return types.Number{Value: float64(randomInt.Int64())}

	case 2:
		// random(min, max) - return random integer from min to max-1
		minResult := types.CoerceToNumber(args[0])
		maxResult := types.CoerceToNumber(args[1])
		if !minResult.Ok || !maxResult.Ok {
			return types.Unknown{Reason: "random() arguments must be numbers"}
		}

		min := int64(minResult.Value.(types.Number).Value)
		max := int64(maxResult.Value.(types.Number).Value)
		if min >= max {
			return types.Unknown{Reason: "random() min must be less than max"}
		}

		randomInt, err := rand.Int(rand.Reader, big.NewInt(max-min))
		if err != nil {
			return types.Unknown{Reason: "failed to generate random number"}
		}
		return types.Number{Value: float64(randomInt.Int64() + min)}

	default:
		return types.Unknown{Reason: "random() takes 0, 1, or 2 arguments"}
	}
}

func (i *Interpreter) evalNowFunction(args []interface{}) interface{} {
	if len(args) != 0 {
		return types.Unknown{Reason: "now() takes no arguments"}
	}

	return types.Time{Timestamp: time.Now()}
}

func (i *Interpreter) evalUuidFunction(args []interface{}) interface{} {
	if len(args) != 0 {
		return types.Unknown{Reason: "uuid() takes no arguments"}
	}

	id := uuid.New()
	return types.Text{Value: id.String()}
}

func (i *Interpreter) evalAssetFunction(args []interface{}) interface{} {
	if len(args) != 2 {
		return types.Unknown{Reason: "asset() requires exactly 2 arguments: asset(data, mime_type)"}
	}

	// Convert arguments to Text type
	dataResult := types.CoerceToText(args[0])
	if !dataResult.Ok {
		return dataResult.Value // Return the Unknown error
	}
	data, _ := dataResult.Value.(types.Text)

	mimeTypeResult := types.CoerceToText(args[1])
	if !mimeTypeResult.Ok {
		return mimeTypeResult.Value // Return the Unknown error
	}
	mimeType, _ := mimeTypeResult.Value.(types.Text)

	// Create a record with .data and .mime_type fields
	return types.Record{
		Fields: map[string]interface{}{
			"data":      data,
			"mime_type": mimeType,
		},
	}
}

// New math functions
func (i *Interpreter) evalRoundFunction(args []interface{}) interface{} {
	if len(args) != 1 {
		return types.Unknown{Reason: "round() requires exactly 1 argument"}
	}

	numberResult := types.CoerceToNumber(args[0])
	if !numberResult.Ok {
		return numberResult.Value
	}
	number, _ := numberResult.Value.(types.Number)

	return types.Number{Value: math.Round(number.Value)}
}

// New text functions
func (i *Interpreter) evalSubstringFunction(args []interface{}) interface{} {
	if len(args) != 3 {
		return types.Unknown{Reason: "substring() requires exactly 3 arguments: substring(text, start, length)"}
	}

	// Convert all arguments to proper types
	textResult := types.CoerceToText(args[0])
	if !textResult.Ok {
		return textResult.Value
	}
	text, _ := textResult.Value.(types.Text)

	startResult := types.CoerceToNumber(args[1])
	if !startResult.Ok {
		return startResult.Value
	}
	start := int(startResult.Value.(types.Number).Value)

	lengthResult := types.CoerceToNumber(args[2])
	if !lengthResult.Ok {
		return lengthResult.Value
	}
	length := int(lengthResult.Value.(types.Number).Value)

	// Validate bounds
	if start < 0 || length < 0 || start >= len(text.Value) {
		return types.Unknown{Reason: fmt.Sprintf("invalid substring bounds: start=%d, length=%d for string of length %d", start, length, len(text.Value))}
	}

	end := start + length
	if end > len(text.Value) {
		end = len(text.Value)
	}

	return types.Text{Value: text.Value[start:end]}
}

func (i *Interpreter) evalFindFunction(args []interface{}) interface{} {
	if len(args) != 2 {
		return types.Unknown{Reason: "find() requires exactly 2 arguments: find(text, pattern)"}
	}

	textResult := types.CoerceToText(args[0])
	if !textResult.Ok {
		return textResult.Value
	}
	text, _ := textResult.Value.(types.Text)

	patternResult := types.CoerceToText(args[1])
	if !patternResult.Ok {
		return patternResult.Value
	}
	pattern, _ := patternResult.Value.(types.Text)

	index := strings.Index(text.Value, pattern.Value)
	if index == -1 {
		return types.Number{Value: -1} // Not found
	}
	return types.Number{Value: float64(index)}
}

func (i *Interpreter) evalReplaceFunction(args []interface{}) interface{} {
	if len(args) != 3 {
		return types.Unknown{Reason: "replace() requires exactly 3 arguments: replace(text, old, new)"}
	}

	textResult := types.CoerceToText(args[0])
	if !textResult.Ok {
		return textResult.Value
	}
	text, _ := textResult.Value.(types.Text)

	oldResult := types.CoerceToText(args[1])
	if !oldResult.Ok {
		return oldResult.Value
	}
	old, _ := oldResult.Value.(types.Text)

	newResult := types.CoerceToText(args[2])
	if !newResult.Ok {
		return newResult.Value
	}
	newStr, _ := newResult.Value.(types.Text)

	result := strings.ReplaceAll(text.Value, old.Value, newStr.Value)
	return types.Text{Value: result}
}

func (i *Interpreter) evalSplitFunction(args []interface{}) interface{} {
	if len(args) != 2 {
		return types.Unknown{Reason: "split() requires exactly 2 arguments: split(text, separator)"}
	}

	textResult := types.CoerceToText(args[0])
	if !textResult.Ok {
		return textResult.Value
	}
	text, _ := textResult.Value.(types.Text)

	separatorResult := types.CoerceToText(args[1])
	if !separatorResult.Ok {
		return separatorResult.Value
	}
	separator, _ := separatorResult.Value.(types.Text)

	parts := strings.Split(text.Value, separator.Value)
	elements := make([]interface{}, len(parts))
	for i, part := range parts {
		elements[i] = types.Text{Value: part}
	}

	return types.List{Elements: elements}
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
		// Check if this is a simple index lookup (single numeric filter)
		if len(node.Filters) == 1 {
			if indexResult := i.tryIndexLookup(baseValue, node.Filters[0]); indexResult != nil {
				return indexResult
			}
		}
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

// tryIndexLookup attempts to perform index lookup for single numeric filters
func (i *Interpreter) tryIndexLookup(list types.List, filter parser.Expression) interface{} {
	// Check if the filter is a literal expression
	literal, ok := filter.(*parser.LiteralExpression)
	if !ok {
		return nil // Not a literal, continue with filtering
	}

	// Check if it's a numeric literal
	num, ok := literal.Value.(types.Number)
	if !ok {
		// Try to handle different numeric types
		switch v := literal.Value.(type) {
		case float64:
			index := int(v)
			if index < 0 || index >= len(list.Elements) {
				return types.Unknown{Reason: fmt.Sprintf("list index %d out of bounds (length %d)", index, len(list.Elements))}
			}
			return list.Elements[index]
		case int:
			if v < 0 || v >= len(list.Elements) {
				return types.Unknown{Reason: fmt.Sprintf("list index %d out of bounds (length %d)", v, len(list.Elements))}
			}
			return list.Elements[v]
		case int64:
			index := int(v)
			if index < 0 || index >= len(list.Elements) {
				return types.Unknown{Reason: fmt.Sprintf("list index %d out of bounds (length %d)", index, len(list.Elements))}
			}
			return list.Elements[index]
		}
		return nil // Not a number, continue with filtering
	}

	// Convert to integer index
	index := int(num.Value)

	// Check bounds
	if index < 0 || index >= len(list.Elements) {
		return types.Unknown{Reason: fmt.Sprintf("list index %d out of bounds (length %d)", index, len(list.Elements))}
	}

	// Return the element at the index
	return list.Elements[index]
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

// evalProjectionExpression evaluates projection expressions like path{ name, age, job }
func (i *Interpreter) evalProjectionExpression(node *parser.ProjectionExpression) interface{} {
	// Evaluate the left expression (the path being projected)
	base := i.evalExpression(node.Left)

	if unknown, ok := base.(types.Unknown); ok {
		return unknown
	}

	// Projections work on records - check if base is a record
	record, ok := base.(types.Record)
	if !ok {
		return types.Unknown{Reason: fmt.Sprintf("cannot project from %T, projections require a record", base)}
	}

	// Create new record with only the specified fields
	projectedFields := make(map[string]interface{})

	for _, fieldName := range node.Fields {
		if value, exists := record.Fields[fieldName]; exists {
			// Ensure value is properly converted to MBL type
			projectedFields[fieldName] = convertToMBLType(value)
		} else {
			// Field doesn't exist - include as Unknown (#not_found)
			projectedFields[fieldName] = types.Unknown{Reason: "#not_found"}
		}
	}

	return types.Record{Fields: projectedFields}
}

func (i *Interpreter) evalRecordExpression(node *parser.RecordExpression) interface{} {
	fields := make(map[string]interface{})

	// Handle new Fields structure if available
	if len(node.Fields) > 0 {
		for _, field := range node.Fields {
			// Evaluate the key (should be a string)
			keyResult := i.evalExpression(field.Key)
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

			// Handle heritability modifiers and reset expressions
			var value interface{}
			if field.Modifier != nil {
				// Apply per-attribute heritability modifier
				value = i.evalExpression(field.Value)
				if unknown, ok := value.(types.Unknown); ok {
					return unknown
				}

				// Store the modifier as a meta-attribute for later inheritance processing
				// For now, store both the value and the modifier information
				switch *field.Modifier {
				case "reset":
					if field.ResetExpr != nil {
						// Evaluate reset expression
						resetValue := i.evalExpression(field.ResetExpr)
						if unknown, ok := resetValue.(types.Unknown); ok {
							return unknown
						}
						// Store both the value and reset expression
						// This will be handled during inheritance
						value = map[string]interface{}{
							"value": value,
							"@reset": resetValue,
						}
					} else {
						// Simple reset without expression
						value = map[string]interface{}{
							"value": value,
							"@reset": true,
						}
					}
				case "exclude":
					// Exclude modifier - mark for exclusion from inheritance
					value = map[string]interface{}{
						"value": value,
						"@exclude": true,
					}
				case "link", "copy":
					// Store modifier for inheritance processing
					value = map[string]interface{}{
						"value": value,
						"@inherit": *field.Modifier,
					}
				default:
					// Default behavior
					value = i.evalExpression(field.Value)
					if unknown, ok := value.(types.Unknown); ok {
						return unknown
					}
				}
			} else {
				// No modifier, evaluate value normally
				value = i.evalExpression(field.Value)
				if unknown, ok := value.(types.Unknown); ok {
					return unknown
				}
				// Ensure value is properly converted to MBL type
				value = convertToMBLType(value)
			}

			fields[key] = value
		}
	} else {
		// Backward compatibility with old Pairs structure
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
			// Ensure value is properly converted to MBL type
			value = convertToMBLType(value)

			fields[key] = value
		}
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

// evalCollectionOperationExpression evaluates collection operations like list..count, list..remove(1)
func (i *Interpreter) evalCollectionOperationExpression(node *parser.CollectionOperationExpression) interface{} {
	// Evaluate the object being operated on
	object := i.evalExpression(node.Object)

	// Check for Unknown propagation
	if unknown, ok := object.(types.Unknown); ok {
		return unknown
	}

	switch node.Method {
	case "count":
		return i.evalCollectionCount(object)
	case "combine":
		if len(node.Arguments) != 1 {
			return types.Unknown{Reason: "combine operation requires exactly one argument (separator)"}
		}
		separator := i.evalExpression(node.Arguments[0])
		if unknown, ok := separator.(types.Unknown); ok {
			return unknown
		}
		return i.evalCollectionCombine(object, separator)
	case "remove":
		if len(node.Arguments) == 0 {
			return types.Unknown{Reason: "remove operation requires at least one argument"}
		}
		var args []interface{}
		for _, argExpr := range node.Arguments {
			arg := i.evalExpression(argExpr)
			if unknown, ok := arg.(types.Unknown); ok {
				return unknown
			}
			args = append(args, arg)
		}
		return i.evalCollectionRemove(object, args)
	default:
		return types.Unknown{Reason: fmt.Sprintf("unsupported collection operation: %s", node.Method)}
	}
}

// evalCollectionCount implements the ..count operation
func (i *Interpreter) evalCollectionCount(object interface{}) interface{} {
	switch obj := object.(type) {
	case types.List:
		return types.Number{Value: float64(len(obj.Elements))}
	case types.Record:
		return types.Number{Value: float64(len(obj.Fields))}
	default:
		return types.Unknown{Reason: fmt.Sprintf("cannot count elements of type %T", object)}
	}
}

// evalCollectionCombine implements the ..combine(separator) operation
func (i *Interpreter) evalCollectionCombine(object, separator interface{}) interface{} {
	list, ok := object.(types.List)
	if !ok {
		return types.Unknown{Reason: fmt.Sprintf("combine operation only works on lists, not %T", object)}
	}

	sepText, ok := separator.(types.Text)
	if !ok {
		return types.Unknown{Reason: fmt.Sprintf("combine separator must be text, not %T", separator)}
	}

	var parts []string
	for _, element := range list.Elements {
		// Convert each element to text
		textResult := types.CoerceToText(element)
		if !textResult.Ok {
			return textResult.Value // Return the error
		}
		parts = append(parts, textResult.Value.(types.Text).Value)
	}

	return types.Text{Value: strings.Join(parts, sepText.Value)}
}

// evalCollectionRemove implements the ..remove(...) operation
func (i *Interpreter) evalCollectionRemove(object interface{}, args []interface{}) interface{} {
	list, ok := object.(types.List)
	if !ok {
		return types.Unknown{Reason: fmt.Sprintf("remove operation only works on lists, not %T", object)}
	}

	if len(args) == 1 {
		// Remove by index or value
		if num, ok := args[0].(types.Number); ok {
			index := int(num.Value)
			// Only treat as index if it's a valid index AND the value at that index differs from the index value
			if index >= 0 && index < len(list.Elements) {
				// Check if the element at this index is different from the index value
				// If they're the same, it's ambiguous, so prefer remove by value
				elementAtIndex := list.Elements[index]
				if elemNum, ok := elementAtIndex.(types.Number); ok && elemNum.Value == num.Value {
					// Ambiguous case: the index and the value are the same, prefer remove by value
					goto removeByValue
				}
				// Remove by index (unambiguous case)
				newElements := make([]interface{}, 0, len(list.Elements)-1)
				for i, elem := range list.Elements {
					if i != index {
						newElements = append(newElements, elem)
					}
				}
				return types.List{Elements: newElements}
			}
		}

	removeByValue:
		// Remove by value
		newElements := make([]interface{}, 0, len(list.Elements))
		removed := false
		for _, elem := range list.Elements {
			if !removed && types.Equal(elem, args[0]) {
				removed = true // Skip first occurrence
			} else {
				newElements = append(newElements, elem)
			}
		}
		return types.List{Elements: newElements}
	} else if len(args) == 2 {
		// Remove range (start, end)
		start, ok1 := args[0].(types.Number)
		end, ok2 := args[1].(types.Number)
		if !ok1 || !ok2 {
			return types.Unknown{Reason: "remove range requires two numeric indices"}
		}
		startIdx := int(start.Value)
		endIdx := int(end.Value)
		if startIdx < 0 || endIdx < 0 || startIdx >= len(list.Elements) || endIdx >= len(list.Elements) || startIdx > endIdx {
			return types.Unknown{Reason: fmt.Sprintf("invalid range [%d, %d] for list of length %d", startIdx, endIdx, len(list.Elements))}
		}
		newElements := make([]interface{}, 0, len(list.Elements)-(endIdx-startIdx+1))
		for i, elem := range list.Elements {
			if i < startIdx || i > endIdx {
				newElements = append(newElements, elem)
			}
		}
		return types.List{Elements: newElements}
	} else {
		return types.Unknown{Reason: "remove operation accepts 1 or 2 arguments"}
	}
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

	// Convert types.Value interface to storage.Value struct
	return storage.Value{
		TypeTag: typesValue.TypeTag(),
		Data:    typesValue.Serialize(),
	}, nil
}

// storageToMBL converts storage.Value to MBL type
func storageToMBL(value storage.Value) (interface{}, error) {
	// Convert storage.Value to types.SerializedValue
	typesValue := types.SerializedValue{
		TypeTag: value.TypeTag,
		Data:    value.Data,
	}

	// Deserialize to MBL type
	return types.DeserializeValue(typesValue)
}

// applyInheritance applies inheritance rules based on modifier
// applyInheritanceWithModifiers applies inheritance with support for per-attribute modifiers
func (i *Interpreter) applyInheritanceWithModifiers(target map[string]interface{}, source map[string]interface{}, globalModifier string) error {
	for key, value := range source {
		// Check if this field has per-attribute modifiers
		if fieldMeta, ok := value.(map[string]interface{}); ok {
			// Check for meta-attributes
			if fieldValue, hasValue := fieldMeta["value"]; hasValue {
				// Extract per-attribute modifier information
				var effectiveModifier string

				if exclude, hasExclude := fieldMeta["@exclude"]; hasExclude && exclude.(bool) {
					// Field marked for exclusion - skip it
					continue
				} else if inherit, hasInherit := fieldMeta["@inherit"]; hasInherit {
					// Use per-attribute inheritance modifier
					effectiveModifier = inherit.(string)
				} else if reset, hasReset := fieldMeta["@reset"]; hasReset {
					if resetExpr, isExpr := reset.(bool); isExpr && resetExpr {
						// Simple reset - use default behavior
						effectiveModifier = "reset"
					} else {
						// Reset with expression - use the already evaluated reset value
						effectiveModifier = "reset"
						// The reset value was already evaluated and is stored in reset
						fieldValue = reset
					}
				} else {
					// Use global modifier
					effectiveModifier = globalModifier
				}

				// Apply the effective modifier
				err := i.applySingleFieldInheritance(target, key, fieldValue, effectiveModifier)
				if err != nil {
					return err
				}
			} else {
				// Not a meta-attribute structure, use global modifier
				err := i.applySingleFieldInheritance(target, key, value, globalModifier)
				if err != nil {
					return err
				}
			}
		} else {
			// Regular field without meta-attributes, use global modifier
			err := i.applySingleFieldInheritance(target, key, value, globalModifier)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// applySingleFieldInheritance applies inheritance for a single field
func (i *Interpreter) applySingleFieldInheritance(target map[string]interface{}, key string, value interface{}, modifier string) error {
	switch modifier {
	case "copy":
		// Copy field if not already present (first source wins for conflicts)
		if _, exists := target[key]; !exists {
			target[key] = i.deepCopyValue(value)
		}
	case "link":
		// Create reference link to source value (shared reference)
		if _, exists := target[key]; !exists {
			target[key] = value // Direct reference (shared)
		}
	case "reset":
		// Copy field, always overriding existing values
		target[key] = i.deepCopyValue(value)
	case "exclude":
		// Field is marked for exclusion - do not inherit it
		// This case is handled at the caller level, but included for completeness
		return nil
	default:
		return fmt.Errorf("unknown modifier: %s", modifier)
	}
	return nil
}

func (i *Interpreter) applyInheritance(target map[string]interface{}, source map[string]interface{}, modifier string) error {
	switch modifier {
	case "copy":
		// Copy all fields from source, can be overridden by later values
		for key, value := range source {
			// Only set if not already present (first source wins for conflicts)
			if _, exists := target[key]; !exists {
				target[key] = i.deepCopyValue(value)
			}
		}
	case "link":
		// Create reference links to source values (shared references)
		for key, value := range source {
			if _, exists := target[key]; !exists {
				target[key] = value // Direct reference (shared)
			}
		}
	case "reset":
		// Copy all fields, but allow complete override
		for key, value := range source {
			target[key] = i.deepCopyValue(value) // Always copy, even if exists
		}
	case "exclude":
		// Copy all fields except those marked for exclusion
		// For now, copy everything (full exclude logic would need additional syntax)
		for key, value := range source {
			if _, exists := target[key]; !exists {
				target[key] = i.deepCopyValue(value)
			}
		}
	default:
		return fmt.Errorf("unknown heritability modifier: %s", modifier)
	}
	return nil
}

// deepCopyValue creates a deep copy of MBL values
func (i *Interpreter) deepCopyValue(value interface{}) interface{} {
	switch v := value.(type) {
	case types.Record:
		// Deep copy the record
		newFields := make(map[string]interface{})
		for key, fieldValue := range v.Fields {
			newFields[key] = i.deepCopyValue(fieldValue)
		}
		return types.Record{Fields: newFields}
	case types.List:
		// Deep copy the list
		newElements := make([]interface{}, len(v.Elements))
		for idx, element := range v.Elements {
			newElements[idx] = i.deepCopyValue(element)
		}
		return types.List{Elements: newElements}
	default:
		// For primitive types (Number, Text, Boolean, etc.), return as-is
		// These are value types in Go, so they're automatically copied
		return v
	}
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
		// Check for Queued literals first
		if strings.HasPrefix(v, "?") {
			reason := strings.TrimPrefix(v, "?")
			if reason == "" {
				reason = "unknown"
			}
			return types.Queued{Info: reason}
		}
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

// evalComputerLibraryProcedure handles computer library procedures like my.computer.files.import
func (i *Interpreter) evalComputerLibraryProcedure(path []string, args []interface{}) interface{} {
	if len(path) < 3 {
		return types.Unknown{Reason: "invalid computer library path"}
	}

	// path[0] = "my", path[1] = "computer", path[2] = sub-library
	subLibrary := path[2]

	switch subLibrary {
	case "files":
		return i.evalFilesLibraryProcedure(path[3:], args)
	case "network":
		return types.Unknown{Reason: "network sub-library not yet implemented"}
	case "system":
		return types.Unknown{Reason: "system sub-library not yet implemented"}
	default:
		return types.Unknown{Reason: fmt.Sprintf("unknown computer sub-library: %s", subLibrary)}
	}
}

// evalFilesLibraryProcedure handles my.computer.files.* procedures
func (i *Interpreter) evalFilesLibraryProcedure(path []string, args []interface{}) interface{} {
	if len(path) == 0 {
		return types.Unknown{Reason: "incomplete files library path"}
	}

	procedure := path[0]

	switch procedure {
	case "import":
		return i.evalFilesImport(args)
	case "export":
		return i.evalFilesExport(args)
	case "import_fixed_width":
		return i.evalFilesImportFixedWidth(args)
	case "read":
		return i.evalFilesRead(args)
	case "write":
		return i.evalFilesWrite(args)
	case "exists":
		return i.evalFilesExists(args)
	case "delete":
		return i.evalFilesDelete(args)
	case "list":
		return i.evalFilesList(args)
	case "info":
		return i.evalFilesInfo(args)
	default:
		return types.Unknown{Reason: fmt.Sprintf("unknown files procedure: %s", procedure)}
	}
}

// evalFilesImport implements my.computer.files.import(path, format, options)
func (i *Interpreter) evalFilesImport(args []interface{}) interface{} {
	if len(args) < 2 {
		return types.Unknown{Reason: "import requires at least 2 arguments: path and format"}
	}

	// Extract path
	pathResult := types.CoerceToText(args[0])
	if !pathResult.Ok {
		return types.Unknown{Reason: "import path must be text"}
	}
	path := pathResult.Value.(types.Text).Value

	// Extract format
	formatResult := types.CoerceToText(args[1])
	if !formatResult.Ok {
		return types.Unknown{Reason: "import format must be text"}
	}
	format := formatResult.Value.(types.Text).Value

	// Extract options if provided
	var options types.Record
	if len(args) >= 3 {
		if record, ok := args[2].(types.Record); ok {
			options = record
		} else {
			return types.Unknown{Reason: "import options must be a record"}
		}
	} else {
		options = types.Record{Fields: make(map[string]interface{})}
	}

	switch format {
	case "xml":
		return i.evalXMLImport(path, options)
	case "json":
		return types.Unknown{Reason: "JSON import not yet implemented"}
	case "csv":
		return types.Unknown{Reason: "CSV import not yet implemented"}
	case "tsv":
		return types.Unknown{Reason: "TSV import not yet implemented"}
	case "toml":
		return types.Unknown{Reason: "TOML import not yet implemented"}
	case "xlsx":
		return types.Unknown{Reason: "Excel import not yet implemented - reserved for future implementation"}
	default:
		return types.Unknown{Reason: fmt.Sprintf("unsupported import format: %s", format)}
	}
}

// evalFilesExport implements my.computer.files.export(node, path, format, options)
func (i *Interpreter) evalFilesExport(args []interface{}) interface{} {
	if len(args) < 3 {
		return types.Unknown{Reason: "export requires at least 3 arguments: node, path, and format"}
	}

	// Extract node (the data to export)
	node := args[0]

	// Extract path
	pathResult := types.CoerceToText(args[1])
	if !pathResult.Ok {
		return types.Unknown{Reason: "export path must be text"}
	}
	path := pathResult.Value.(types.Text).Value

	// Extract format
	formatResult := types.CoerceToText(args[2])
	if !formatResult.Ok {
		return types.Unknown{Reason: "export format must be text"}
	}
	format := formatResult.Value.(types.Text).Value

	// Extract options if provided
	var options types.Record
	if len(args) >= 4 {
		if record, ok := args[3].(types.Record); ok {
			options = record
		} else {
			return types.Unknown{Reason: "export options must be a record"}
		}
	} else {
		options = types.Record{Fields: make(map[string]interface{})}
	}

	switch format {
	case "xml":
		return i.evalXMLExport(node, path, options)
	case "json":
		return types.Unknown{Reason: "JSON export not yet implemented"}
	case "csv":
		return types.Unknown{Reason: "CSV export not yet implemented"}
	case "tsv":
		return types.Unknown{Reason: "TSV export not yet implemented"}
	case "toml":
		return types.Unknown{Reason: "TOML export not yet implemented"}
	case "xlsx":
		return types.Unknown{Reason: "Excel export not yet implemented - reserved for future implementation"}
	default:
		return types.Unknown{Reason: fmt.Sprintf("unsupported export format: %s", format)}
	}
}

// Placeholder implementations for other files procedures
func (i *Interpreter) evalFilesRead(args []interface{}) interface{} {
	return types.Unknown{Reason: "files.read not yet implemented"}
}

func (i *Interpreter) evalFilesWrite(args []interface{}) interface{} {
	return types.Unknown{Reason: "files.write not yet implemented"}
}

func (i *Interpreter) evalFilesExists(args []interface{}) interface{} {
	return types.Unknown{Reason: "files.exists not yet implemented"}
}

func (i *Interpreter) evalFilesDelete(args []interface{}) interface{} {
	return types.Unknown{Reason: "files.delete not yet implemented"}
}

func (i *Interpreter) evalFilesList(args []interface{}) interface{} {
	return types.Unknown{Reason: "files.list not yet implemented"}
}

func (i *Interpreter) evalFilesInfo(args []interface{}) interface{} {
	return types.Unknown{Reason: "files.info not yet implemented"}
}

// evalXMLImport imports XML file and converts to AmorphDB record structure
func (i *Interpreter) evalXMLImport(path string, options types.Record) interface{} {
	// Read the XML file
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return types.Unknown{Reason: fmt.Sprintf("failed to read file %s: %v", path, err)}
	}

	// Parse XML into a generic structure
	var xmlData map[string]interface{}
	err = xml.Unmarshal(data, &xmlData)
	if err != nil {
		// Try a more flexible approach with tokens
		return i.parseXMLWithTokens(data, options)
	}

	// Convert to AmorphDB structure
	return i.convertXMLToRecord(xmlData)
}

// parseXMLWithTokens parses XML using token-based approach for more flexibility
func (i *Interpreter) parseXMLWithTokens(data []byte, options types.Record) interface{} {
	decoder := xml.NewDecoder(strings.NewReader(string(data)))

	// Build the tree structure
	root, err := i.buildXMLTree(decoder)
	if err != nil {
		return types.Unknown{Reason: fmt.Sprintf("failed to parse XML: %v", err)}
	}

	return root
}

// XMLNode represents a node in the XML tree
type XMLNode struct {
	Name       xml.Name
	Attributes map[string]string
	Text       string
	Children   []*XMLNode
}

// buildXMLTree builds a tree structure from XML tokens
func (i *Interpreter) buildXMLTree(decoder *xml.Decoder) (interface{}, error) {
	var root *XMLNode
	var stack []*XMLNode

	for {
		token, err := decoder.Token()
		if err != nil {
			break // End of document or error
		}

		switch t := token.(type) {
		case xml.StartElement:
			node := &XMLNode{
				Name:       t.Name,
				Attributes: make(map[string]string),
				Children:   make([]*XMLNode, 0),
			}

			// Process attributes
			for _, attr := range t.Attr {
				node.Attributes[attr.Name.Local] = attr.Value
			}

			// Add to parent or set as root
			if len(stack) > 0 {
				parent := stack[len(stack)-1]
				parent.Children = append(parent.Children, node)
			} else {
				root = node
			}

			// Push to stack
			stack = append(stack, node)

		case xml.CharData:
			if len(stack) > 0 {
				current := stack[len(stack)-1]
				text := strings.TrimSpace(string(t))
				if text != "" {
					current.Text = text
				}
			}

		case xml.EndElement:
			// Pop from stack
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}
		}
	}

	if root == nil {
		return types.Unknown{Reason: "no root element found in XML"}, nil
	}

	return i.xmlNodeToRecord(root), nil
}

// xmlNodeToRecord converts XMLNode to AmorphDB record
func (i *Interpreter) xmlNodeToRecord(node *XMLNode) interface{} {
	fields := make(map[string]interface{})

	// Add attributes as fields
	for name, value := range node.Attributes {
		fields[name] = types.Text{Value: value}
	}

	// Add text content if present
	if node.Text != "" {
		fields["_text"] = types.Text{Value: node.Text}
	}

	// Process child elements
	childGroups := make(map[string][]*XMLNode)
	for _, child := range node.Children {
		name := child.Name.Local
		childGroups[name] = append(childGroups[name], child)
	}

	// Convert child groups to fields or lists
	for name, children := range childGroups {
		if len(children) == 1 {
			// Single child becomes a field
			fields[name] = i.xmlNodeToRecord(children[0])
		} else {
			// Multiple children become a list
			elements := make([]interface{}, len(children))
			for idx, child := range children {
				elements[idx] = i.xmlNodeToRecord(child)
			}
			fields[name] = types.List{Elements: elements}
		}
	}

	return types.Record{Fields: fields}
}

// convertXMLToRecord converts generic XML data to AmorphDB record (fallback)
func (i *Interpreter) convertXMLToRecord(data map[string]interface{}) interface{} {
	fields := make(map[string]interface{})

	for key, value := range data {
		switch v := value.(type) {
		case string:
			fields[key] = types.Text{Value: v}
		case map[string]interface{}:
			fields[key] = i.convertXMLToRecord(v)
		case []interface{}:
			elements := make([]interface{}, len(v))
			for idx, item := range v {
				if itemMap, ok := item.(map[string]interface{}); ok {
					elements[idx] = i.convertXMLToRecord(itemMap)
				} else {
					elements[idx] = types.Text{Value: fmt.Sprintf("%v", item)}
				}
			}
			fields[key] = types.List{Elements: elements}
		default:
			fields[key] = types.Text{Value: fmt.Sprintf("%v", v)}
		}
	}

	return types.Record{Fields: fields}
}

// evalXMLExport exports AmorphDB record to XML file
func (i *Interpreter) evalXMLExport(node interface{}, path string, options types.Record) interface{} {
	// Convert AmorphDB node to XML
	xmlContent, err := i.recordToXML(node, options)
	if err != nil {
		return types.Unknown{Reason: fmt.Sprintf("failed to convert to XML: %v", err)}
	}

	// Add XML header
	if pretty, exists := options.Fields["pretty"]; exists {
		if prettyBool, ok := pretty.(types.Boolean); ok && prettyBool.Value {
			xmlContent = "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n" + xmlContent
		}
	} else {
		// Default to pretty formatting
		xmlContent = "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n" + xmlContent
	}

	// Write to file
	err = ioutil.WriteFile(path, []byte(xmlContent), 0644)
	if err != nil {
		return types.Unknown{Reason: fmt.Sprintf("failed to write file %s: %v", path, err)}
	}

	// Return success time
	return types.Time{Timestamp: time.Now(), Precision: types.PrecisionSecond}
}

// recordToXML converts AmorphDB record to XML string
func (i *Interpreter) recordToXML(node interface{}, options types.Record) (string, error) {
	record, ok := node.(types.Record)
	if !ok {
		return "", fmt.Errorf("can only export records to XML, got %T", node)
	}

	// Determine which fields should be attributes vs elements
	var attributeFields map[string]bool
	if attrs, exists := options.Fields["attributes"]; exists {
		if attrList, ok := attrs.(types.List); ok {
			attributeFields = make(map[string]bool)
			for _, attr := range attrList.Elements {
				if attrText, ok := attr.(types.Text); ok {
					attributeFields[attrText.Value] = true
				}
			}
		}
	}

	// Build XML string
	var result strings.Builder

	// For now, use a default root element name
	rootName := "root"
	if name, exists := options.Fields["root_element"]; exists {
		if nameText, ok := name.(types.Text); ok {
			rootName = nameText.Value
		}
	}

	result.WriteString(fmt.Sprintf("<%s", rootName))

	// Add attributes
	if attributeFields != nil {
		for field, value := range record.Fields {
			if attributeFields[field] {
				if textValue, ok := value.(types.Text); ok {
					result.WriteString(fmt.Sprintf(" %s=\"%s\"", field, textValue.Value))
				}
			}
		}
	}

	result.WriteString(">")

	// Add child elements
	for field, value := range record.Fields {
		if attributeFields != nil && attributeFields[field] {
			continue // Skip fields that are attributes
		}

		if field == "_text" {
			// Special handling for text content
			if textValue, ok := value.(types.Text); ok {
				result.WriteString(textValue.Value)
			}
			continue
		}

		// Convert field to XML element
		fieldXML, err := i.fieldToXMLElement(field, value, options)
		if err != nil {
			return "", err
		}
		result.WriteString(fieldXML)
	}

	result.WriteString(fmt.Sprintf("</%s>", rootName))

	return result.String(), nil
}

// fieldToXMLElement converts a field to an XML element
func (i *Interpreter) fieldToXMLElement(name string, value interface{}, options types.Record) (string, error) {
	switch v := value.(type) {
	case types.Text:
		return fmt.Sprintf("<%s>%s</%s>", name, v.Value, name), nil
	case types.Number:
		return fmt.Sprintf("<%s>%g</%s>", name, v.Value, name), nil
	case types.Boolean:
		return fmt.Sprintf("<%s>%t</%s>", name, v.Value, name), nil
	case types.Record:
		// Nested record becomes nested element
		nestedXML, err := i.recordToXML(v, options)
		if err != nil {
			return "", err
		}
		// Replace root element name with field name
		// This is simplified - a more robust implementation would use XML parsing
		nestedXML = strings.Replace(nestedXML, "<root", fmt.Sprintf("<%s", name), 1)
		nestedXML = strings.Replace(nestedXML, "</root>", fmt.Sprintf("</%s>", name), 1)
		return nestedXML, nil
	case types.List:
		// List becomes multiple elements with the same name
		var result strings.Builder
		for _, element := range v.Elements {
			elementXML, err := i.fieldToXMLElement(name, element, options)
			if err != nil {
				return "", err
			}
			result.WriteString(elementXML)
		}
		return result.String(), nil
	case types.Unknown:
		// Skip Unknown values
		return "", nil
	default:
		return fmt.Sprintf("<%s>%v</%s>", name, v, name), nil
	}
}

// evalFilesImportFixedWidth implements my.computer.files.import_fixed_width(path, ari_spec, options)
func (i *Interpreter) evalFilesImportFixedWidth(args []interface{}) interface{} {
	if len(args) < 2 {
		return types.Unknown{Reason: "import_fixed_width requires at least 2 arguments: path and ari_spec"}
	}

	// Extract path
	pathResult := types.CoerceToText(args[0])
	if !pathResult.Ok {
		return types.Unknown{Reason: "import_fixed_width path must be text"}
	}
	path := pathResult.Value.(types.Text).Value

	// Extract ARI spec
	ariSpecResult := types.CoerceToText(args[1])
	if !ariSpecResult.Ok {
		return types.Unknown{Reason: "import_fixed_width ari_spec must be text"}
	}
	ariSpecText := ariSpecResult.Value.(types.Text).Value

	// Extract options if provided (reserved for future use)
	if len(args) >= 3 {
		if _, ok := args[2].(types.Record); !ok {
			return types.Unknown{Reason: "import_fixed_width options must be a record"}
		}
		// TODO: Add options support when needed
	}

	// Check if file exists and get size for safety
	fileInfo, err := os.Stat(path)
	if err != nil {
		return types.Unknown{Reason: fmt.Sprintf("file not found or not accessible: %s", path)}
	}

	// Check file size limit (10MB max to prevent OOM)
	maxSize := int64(10 * 1024 * 1024)
	if fileInfo.Size() > maxSize {
		return types.Unknown{Reason: fmt.Sprintf("#file_too_large: file size %d exceeds maximum %d bytes", fileInfo.Size(), maxSize)}
	}

	// Parse ARI specification
	ariLexer := ari.NewLexer(ariSpecText)
	ariParser := ari.NewParser(ariLexer)
	ariSpec := ariParser.ParseARISpec()

	if len(ariParser.Errors()) > 0 {
		return types.Unknown{Reason: fmt.Sprintf("ARI specification parsing errors: %v", ariParser.Errors())}
	}

	// Open file for processing
	file, err := os.Open(path)
	if err != nil {
		return types.Unknown{Reason: fmt.Sprintf("failed to open file: %v", err)}
	}
	defer file.Close()

	// Process file with ARI engine
	engine := ari.NewEngine(ariSpec)
	result, err := engine.ProcessFile(file, fileInfo.Size())
	if err != nil {
		return types.Unknown{Reason: fmt.Sprintf("ARI processing failed: %v", err)}
	}

	// Convert result to MBL types
	return i.convertARIResult(result)
}

// convertARIResult converts ARI engine results to MBL types
func (i *Interpreter) convertARIResult(result interface{}) interface{} {
	switch v := result.(type) {
	case nil:
		return types.Unknown{Reason: "#not_found"}
	case string:
		return types.Text{Value: v}
	case int:
		return types.Number{Value: float64(v)}
	case float64:
		return types.Number{Value: v}
	case bool:
		return types.Boolean{Value: v}
	case map[string]interface{}:
		// Convert map to MBL Record
		record := types.Record{Fields: make(map[string]interface{})}
		for key, value := range v {
			record.Fields[key] = i.convertARIResult(value)
		}
		return record
	case []interface{}:
		// Convert slice to MBL List
		elements := make([]interface{}, len(v))
		for idx, element := range v {
			elements[idx] = i.convertARIResult(element)
		}
		return types.List{Elements: elements}
	case []map[string]interface{}:
		// Convert slice of maps to MBL List
		elements := make([]interface{}, len(v))
		for idx, element := range v {
			elements[idx] = i.convertARIResult(element)
		}
		return types.List{Elements: elements}
	default:
		// Fallback for unknown types
		return types.Text{Value: fmt.Sprintf("%v", v)}
	}
}

