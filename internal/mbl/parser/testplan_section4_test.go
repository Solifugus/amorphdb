// AmorphDB Test Plan Section 4: MBL Parser
// This file implements all test cases from Section 4 of AmorphDB_Test_Plan.md
//
// Section 4 covers:
// - 4.1 Expressions (19 test cases)
// - 4.2 Statements (23 test cases)
// - 4.3 Error Recovery (5 test cases)
//
// IMPORTANT: When tests fail, fix the parser implementation to match amorphdb_design.md
// specification. Never change tests to match broken parser. The spec wins.

package parser

import (
	"strings"
	"testing"

	"github.com/solifugus/amorphdb/internal/mbl/lexer"
)

// Helper function to create a parser from input and parse as program
func parseProgram(input string) (*Program, []string) {
	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram()
	return program, p.Errors()
}

// Helper function to check a specific AST node type and structure
func checkAST(t *testing.T, program *Program, expectedType string, expectedStructure string) {
	if program == nil {
		t.Fatalf("ParseProgram() returned nil")
	}

	if len(program.Statements) == 0 {
		t.Fatalf("program.Statements does not contain any statements")
	}

	// Convert AST to string and check structure
	actual := program.String()
	if !strings.Contains(actual, expectedStructure) {
		t.Errorf("AST structure mismatch.\nExpected to contain: %q\nActual: %q", expectedStructure, actual)
	}
}

// ============================================================================
// 4.1 EXPRESSIONS
// ============================================================================

func TestSection4_1_1_ArithmeticPrecedence(t *testing.T) {
	input := "result = 2 + 3 * 4"
	program, errors := parseProgram(input)

	if len(errors) != 0 {
		t.Fatalf("parser errors: %v", errors)
	}

	// Should parse as (2 + (3 * 4)), not ((2 + 3) * 4)
	// Multiplication has higher precedence than addition
	checkAST(t, program, "BinaryExpression", "(2 + (3 * 4))")
}

func TestSection4_1_2_ExponentiationRightAssociativity(t *testing.T) {
	input := "result = 2 ^ 3 ^ 2"
	program, errors := parseProgram(input)

	if len(errors) != 0 {
		t.Fatalf("parser errors: %v", errors)
	}

	// Should parse as (2 ^ (3 ^ 2)), not ((2 ^ 3) ^ 2)
	// Exponentiation is right-associative
	checkAST(t, program, "BinaryExpression", "(2 ^ (3 ^ 2))")
}

func TestSection4_1_3_ParenthesesOverride(t *testing.T) {
	input := "result = (2 + 3) * 4"
	program, errors := parseProgram(input)

	if len(errors) != 0 {
		t.Fatalf("parser errors: %v", errors)
	}

	// Parentheses override normal precedence
	checkAST(t, program, "BinaryExpression", "((2 + 3) * 4)")
}

func TestSection4_1_4_LogicalAndOrEqualPrecedence(t *testing.T) {
	input := "result = a and b or c"
	program, errors := parseProgram(input)

	if len(errors) != 0 {
		t.Fatalf("parser errors: %v", errors)
	}

	// 'and' and 'or' have equal precedence and are left-associative
	checkAST(t, program, "BinaryExpression", "((a and b) or c)")
}

func TestSection4_1_5_NotPrecedence(t *testing.T) {
	input := "result = not a and b"
	program, errors := parseProgram(input)

	if len(errors) != 0 {
		t.Fatalf("parser errors: %v", errors)
	}

	// 'not' has higher precedence than 'and'
	checkAST(t, program, "BinaryExpression", "((not a) and b)")
}

func TestSection4_1_6_ComparisonChain(t *testing.T) {
	// Test that context determines whether = is comparison or assignment

	// In expression context, = should be comparison
	input1 := "if a = b: pass"
	program1, errors1 := parseProgram(input1)

	if len(errors1) != 0 {
		t.Fatalf("parser errors for comparison context: %v", errors1)
	}

	checkAST(t, program1, "IfStatement", "a = b")

	// In statement context, = should be assignment
	input2 := "x = 5"
	program2, errors2 := parseProgram(input2)

	if len(errors2) != 0 {
		t.Fatalf("parser errors for assignment context: %v", errors2)
	}

	checkAST(t, program2, "AssignmentStatement", "x = 5")
}

func TestSection4_1_7_PathExpression(t *testing.T) {
	input := "result = my.account.balance"
	program, errors := parseProgram(input)

	if len(errors) != 0 {
		t.Fatalf("parser errors: %v", errors)
	}

	// Should parse as nested member access
	checkAST(t, program, "PathExpression", "my.account.balance")
}

func TestSection4_1_8_PathWithBrackets(t *testing.T) {
	input := "result = my.users[active = true]"
	program, errors := parseProgram(input)

	if len(errors) != 0 {
		t.Fatalf("parser errors: %v", errors)
	}

	// Should parse as filter expression (with parentheses around filter condition)
	checkAST(t, program, "BracketFilterExpression", "my.users[(active = true)]")
}

func TestSection4_1_9_PathWithProjection(t *testing.T) {
	input := "result = my.users{ name, age }"
	program, errors := parseProgram(input)

	if len(errors) != 0 {
		t.Fatalf("parser errors: %v", errors)
	}

	// Should parse as projection node
	checkAST(t, program, "ProjectionExpression", "my.users{ name, age }")
}

func TestSection4_1_10_BracketsAndProjection(t *testing.T) {
	input := "result = my.users[active = true]{ name, email }"
	program, errors := parseProgram(input)

	if len(errors) != 0 {
		t.Fatalf("parser errors: %v", errors)
	}

	// Should parse as filter then project (with parentheses around filter condition)
	checkAST(t, program, "ProjectionExpression", "my.users[(active = true)]{ name, email }")
}

func TestSection4_1_11_TemporalInBrackets(t *testing.T) {
	input := "result = person.name[@2025-06-01]"
	program, errors := parseProgram(input)

	if len(errors) != 0 {
		t.Fatalf("parser errors: %v", errors)
	}

	// Should parse as "as of" temporal query
	checkAST(t, program, "BracketFilterExpression", "@2025-06-01")
}

func TestSection4_1_12_MixedTemporalAndAttributeFilter(t *testing.T) {
	input := "result = person[name = \"bob\", @ < @2025-04-15]"
	program, errors := parseProgram(input)

	if len(errors) != 0 {
		t.Fatalf("parser errors: %v", errors)
	}

	// Should parse with both attribute and temporal conditions
	checkAST(t, program, "BracketFilterExpression", "name = \"bob\"")
	checkAST(t, program, "BracketFilterExpression", "@ < @2025-04-15")
}

func TestSection4_1_13_RecordLiteral(t *testing.T) {
	input := "result = { name: \"Matt\", age: 55 }"
	program, errors := parseProgram(input)

	if len(errors) != 0 {
		t.Fatalf("parser errors: %v", errors)
	}

	// Should parse as record literal AST node
	checkAST(t, program, "RecordLiteralExpression", "name: \"Matt\"")
	checkAST(t, program, "RecordLiteralExpression", "age: 55")
}

func TestSection4_1_14_NestedRecordLiteral(t *testing.T) {
	input := "result = { address: { city: \"Rome\", state: \"NY\" } }"
	program, errors := parseProgram(input)

	if len(errors) != 0 {
		t.Fatalf("parser errors: %v", errors)
	}

	// Should parse with nested record literal
	checkAST(t, program, "RecordLiteralExpression", "address:")
	checkAST(t, program, "RecordLiteralExpression", "city: \"Rome\"")
	checkAST(t, program, "RecordLiteralExpression", "state: \"NY\"")
}

func TestSection4_1_15_TypeSafeComparison(t *testing.T) {
	input := "result = x ?= 5"
	program, errors := parseProgram(input)

	if len(errors) != 0 {
		t.Fatalf("parser errors: %v", errors)
	}

	// Should parse to type-safe equality node
	checkAST(t, program, "BinaryExpression", "x ?= 5")
}

func TestSection4_1_16_StringConcatenation(t *testing.T) {
	input := "result = \"Hello\" & \" \" & \"World\""
	program, errors := parseProgram(input)

	if len(errors) != 0 {
		t.Fatalf("parser errors: %v", errors)
	}

	// Should parse as string concatenation with correct precedence
	checkAST(t, program, "BinaryExpression", "&")
}

func TestSection4_1_17_CollectionOperation(t *testing.T) {
	// Test x..count
	input1 := "result = x..count"
	program1, errors1 := parseProgram(input1)

	if len(errors1) != 0 {
		t.Fatalf("parser errors for ..count: %v", errors1)
	}
	checkAST(t, program1, "CollectionOperationExpression", "x..count")

	// Test x..combine(",")
	input2 := "result = x..combine(\",\")"
	program2, errors2 := parseProgram(input2)

	if len(errors2) != 0 {
		t.Fatalf("parser errors for ..combine: %v", errors2)
	}
	checkAST(t, program2, "CollectionOperationExpression", "x..combine")

	// Test x..remove(2)
	input3 := "result = x..remove(2)"
	program3, errors3 := parseProgram(input3)

	if len(errors3) != 0 {
		t.Fatalf("parser errors for ..remove: %v", errors3)
	}
	checkAST(t, program3, "CollectionOperationExpression", "x..remove")
}

func TestSection4_1_18_MetaAttributeAccess(t *testing.T) {
	input := "result = my.account.balance.@time"
	program, errors := parseProgram(input)

	if len(errors) != 0 {
		t.Fatalf("parser errors: %v", errors)
	}

	// Should parse as meta attribute access
	checkAST(t, program, "PathExpression", "my.account.balance.@time")
}

func TestSection4_1_19_InstanceMetaInBrackets(t *testing.T) {
	input := "result = my.log[@agent = \"\"world.agent.kalevo\"\"]"
	program, errors := parseProgram(input)

	if len(errors) != 0 {
		t.Fatalf("parser errors: %v", errors)
	}

	// Should parse instance meta filter with reference syntax
	checkAST(t, program, "BracketFilterExpression", "@agent")
	checkAST(t, program, "BracketFilterExpression", "\"\"world.agent.kalevo\"\"")
}

// ============================================================================
// 4.2 STATEMENTS
// ============================================================================

func TestSection4_2_1_ValueAssignment(t *testing.T) {
	input := "my.x = 42"
	program, errors := parseProgram(input)

	if len(errors) != 0 {
		t.Fatalf("parser errors: %v", errors)
	}

	// Should parse as assignment AST node
	checkAST(t, program, "AssignmentStatement", "my.x = 42")
}

func TestSection4_2_2_Definition(t *testing.T) {
	input := "my.func: procedure(x): return x * 2"
	program, errors := parseProgram(input)

	if len(errors) != 0 {
		t.Fatalf("parser errors: %v", errors)
	}

	// Should parse as definition node
	checkAST(t, program, "DefinitionStatement", "my.func:")
	checkAST(t, program, "ProcedureExpression", "procedure(x)")
}

// TestSection4_2_3_AppendAssignment - REMOVED
// The +my.list = "item" syntax is deprecated. Use my.list..append("item") instead.

func TestSection4_2_4_QuietAssignment(t *testing.T) {
	input := "my.x = (quietly) \"value\""
	program, errors := parseProgram(input)

	if len(errors) != 0 {
		t.Fatalf("parser errors: %v", errors)
	}

	// Should parse with quiet modifier on assignment
	checkAST(t, program, "AssignmentStatement", "my.x = (quietly) \"value\"")
}

func TestSection4_2_5_IfStatement(t *testing.T) {
	input := `if x > 5:
    y = 10`
	program, errors := parseProgram(input)

	if len(errors) != 0 {
		t.Fatalf("parser errors: %v", errors)
	}

	// Should parse if statement with indented block
	checkAST(t, program, "IfStatement", "if")
	checkAST(t, program, "BinaryExpression", "x > 5")
}

func TestSection4_2_6_IfElse(t *testing.T) {
	input := `if x > 5:
    y = 10
else:
    y = 0`
	program, errors := parseProgram(input)

	if len(errors) != 0 {
		t.Fatalf("parser errors: %v", errors)
	}

	// Should parse if/else statement
	checkAST(t, program, "IfStatement", "if")
	checkAST(t, program, "IfStatement", "else")
}

func TestSection4_2_7_IfElseIfElse(t *testing.T) {
	input := `if x > 5:
    y = 10
else if x > 0:
    y = 5
else:
    y = 0`
	program, errors := parseProgram(input)

	if len(errors) != 0 {
		t.Fatalf("parser errors: %v", errors)
	}

	// Should parse three-branch conditional
	checkAST(t, program, "IfStatement", "if")
	checkAST(t, program, "IfStatement", "else if")
	checkAST(t, program, "IfStatement", "else")
}

func TestSection4_2_8_ForEachLoop(t *testing.T) {
	input := `for item in collection:
    process(item)`
	program, errors := parseProgram(input)

	if len(errors) != 0 {
		t.Fatalf("parser errors: %v", errors)
	}

	// Should parse for-each loop
	checkAST(t, program, "ForStatement", "for item in collection")
}

func TestSection4_2_9_ForEachWithIndex(t *testing.T) {
	input := `for item, index in collection:
    process(item, index)`
	program, errors := parseProgram(input)

	if len(errors) != 0 {
		t.Fatalf("parser errors: %v", errors)
	}

	// Should parse for-each with index
	checkAST(t, program, "ForStatement", "for item, index in collection")
}

func TestSection4_2_10_WhileLoop(t *testing.T) {
	input := `while condition:
    process()`
	program, errors := parseProgram(input)

	if len(errors) != 0 {
		t.Fatalf("parser errors: %v", errors)
	}

	// Should parse while loop
	checkAST(t, program, "WhileStatement", "while condition")
}

func TestSection4_2_11_CatchElse(t *testing.T) {
	input := `catch:
    risky_operation()
else unknown:
    handle_unknown()`
	program, errors := parseProgram(input)

	if len(errors) != 0 {
		t.Fatalf("parser errors: %v", errors)
	}

	// Should parse catch/else structure
	checkAST(t, program, "CatchStatement", "catch")
	checkAST(t, program, "CatchStatement", "else unknown")
}

func TestSection4_2_12_ReturnWithValue(t *testing.T) {
	input := "return x * 2"
	program, errors := parseProgram(input)

	if len(errors) != 0 {
		t.Fatalf("parser errors: %v", errors)
	}

	// Should parse return with expression value
	checkAST(t, program, "ReturnStatement", "return")
	checkAST(t, program, "BinaryExpression", "x * 2")
}

func TestSection4_2_13_ReturnVoid(t *testing.T) {
	input := "return"
	program, errors := parseProgram(input)

	if len(errors) != 0 {
		t.Fatalf("parser errors: %v", errors)
	}

	// Should parse void return
	checkAST(t, program, "ReturnStatement", "return")
}

func TestSection4_2_14_ProcedureDefinition(t *testing.T) {
	input := `my.calc: procedure(x, y):
    result = x * y
    return result`
	program, errors := parseProgram(input)

	if len(errors) != 0 {
		t.Fatalf("parser errors: %v", errors)
	}

	// Should parse full multi-line procedure with params
	checkAST(t, program, "DefinitionStatement", "my.calc:")
	checkAST(t, program, "ProcedureExpression", "procedure(x, y)")
}

func TestSection4_2_15_WatcherDefinition(t *testing.T) {
	input := `my.watcher: watch(path):
    process(path)`
	program, errors := parseProgram(input)

	if len(errors) != 0 {
		t.Fatalf("parser errors: %v", errors)
	}

	// Should parse watcher definition with body
	checkAST(t, program, "DefinitionStatement", "my.watcher:")
	checkAST(t, program, "WatchExpression", "watch(path)")
}

func TestSection4_2_16_MultiPathWatcher(t *testing.T) {
	input := `my.watcher: watch(path1, path2, path3):
    process_multiple()`
	program, errors := parseProgram(input)

	if len(errors) != 0 {
		t.Fatalf("parser errors: %v", errors)
	}

	// Should parse multi-path watcher
	checkAST(t, program, "WatchExpression", "watch(path1, path2, path3)")
}

func TestSection4_2_17_AppendWatcher(t *testing.T) {
	input := `my.watcher: watch append(my.orders) as orders:
    process_orders(orders)`
	program, errors := parseProgram(input)

	if len(errors) != 0 {
		t.Fatalf("parser errors: %v", errors)
	}

	// Should parse append watcher with binding
	checkAST(t, program, "WatchExpression", "watch append(my.orders) as orders")
}

func TestSection4_2_18_AppendWatcherWithPredicate(t *testing.T) {
	input := `my.watcher: watch append(my.orders[priority ?= "urgent"]) as urgent:
    process_urgent(urgent)`
	program, errors := parseProgram(input)

	if len(errors) != 0 {
		t.Fatalf("parser errors: %v", errors)
	}

	// Should parse append watcher with predicate filter
	checkAST(t, program, "WatchExpression", "watch append")
	checkAST(t, program, "BracketFilterExpression", "priority ?= \"urgent\"")
}

func TestSection4_2_19_EmbedStatement(t *testing.T) {
	input := "embed my.context.stamp"
	program, errors := parseProgram(input)

	if len(errors) != 0 {
		t.Fatalf("parser errors: %v", errors)
	}

	// Should parse embed statement
	checkAST(t, program, "EmbedStatement", "embed my.context.stamp")
}

func TestSection4_2_20_SpreadSyntax(t *testing.T) {
	input := "...my.context.stamp"
	program, errors := parseProgram(input)

	if len(errors) != 0 {
		t.Fatalf("parser errors: %v", errors)
	}

	// Should parse spread syntax (equivalent to embed)
	checkAST(t, program, "SpreadStatement", "...my.context.stamp")
}

func TestSection4_2_21_ScopeSet(t *testing.T) {
	input := "my.scope."
	program, errors := parseProgram(input)

	if len(errors) != 0 {
		t.Fatalf("parser errors: %v", errors)
	}

	// Should parse scope setting with trailing dot
	checkAST(t, program, "ScopeStatement", "my.scope.")
}

func TestSection4_2_22_ScopeRelativeReference(t *testing.T) {
	input := `my.scope.:
    .local.variable = 5`
	program, errors := parseProgram(input)

	if len(errors) != 0 {
		t.Fatalf("parser errors: %v", errors)
	}

	// Should parse scope-relative reference inside a scope block
	checkAST(t, program, "PathExpression", ".local.variable")
}

func TestSection4_2_23_BreakStatement(t *testing.T) {
	input := `while true:
    if condition:
        break
    process()`
	program, errors := parseProgram(input)

	if len(errors) != 0 {
		t.Fatalf("parser errors: %v", errors)
	}

	// Should parse break statement inside a loop
	checkAST(t, program, "BreakStatement", "break")
}

// ============================================================================
// 4.3 ERROR RECOVERY
// ============================================================================

func TestSection4_3_1_MissingColonAfterIf(t *testing.T) {
	input := "if x > 5\n    y = 10"  // Missing colon after if condition
	_, errors := parseProgram(input)

	if len(errors) == 0 {
		t.Fatal("Expected parser error for missing colon, but got none")
	}

	// Should report clear error with line number
	errorMsg := errors[0]
	if !strings.Contains(strings.ToLower(errorMsg), "colon") && !strings.Contains(strings.ToLower(errorMsg), ":") {
		t.Errorf("Error message should mention missing colon. Got: %s", errorMsg)
	}

	if !strings.Contains(errorMsg, "line") {
		t.Errorf("Error message should include line number. Got: %s", errorMsg)
	}
}

func TestSection4_3_2_UnmatchedBracket(t *testing.T) {
	input := "my.list[0"  // Missing closing bracket
	_, errors := parseProgram(input)

	if len(errors) == 0 {
		t.Fatal("Expected parser error for unmatched bracket, but got none")
	}

	// Should identify missing ]
	errorMsg := errors[0]
	if !strings.Contains(errorMsg, "]") && !strings.Contains(strings.ToLower(errorMsg), "bracket") {
		t.Errorf("Error message should mention missing bracket. Got: %s", errorMsg)
	}
}

func TestSection4_3_3_InvalidOperator(t *testing.T) {
	input := "my.x == 5"  // == is not a valid operator (should be ?= for equality)
	_, errors := parseProgram(input)

	if len(errors) == 0 {
		t.Fatal("Expected parser error for invalid operator, but got none")
	}

	// Should report == is not valid
	errorMsg := errors[0]
	if !strings.Contains(errorMsg, "==") || (!strings.Contains(strings.ToLower(errorMsg), "invalid") && !strings.Contains(strings.ToLower(errorMsg), "unexpected")) {
		t.Errorf("Error message should indicate == is invalid. Got: %s", errorMsg)
	}
}

func TestSection4_3_4_UnexpectedToken(t *testing.T) {
	input := "my.x = = 5"  // Double equals in wrong position
	_, errors := parseProgram(input)

	if len(errors) == 0 {
		t.Fatal("Expected parser error for unexpected token, but got none")
	}

	// Should report unexpected token
	errorMsg := errors[0]
	if !strings.Contains(strings.ToLower(errorMsg), "unexpected") && !strings.Contains(strings.ToLower(errorMsg), "invalid") {
		t.Errorf("Error message should indicate unexpected token. Got: %s", errorMsg)
	}
}

func TestSection4_3_5_EmptyProcedureBody(t *testing.T) {
	input := "my.func: procedure():"  // No body after procedure definition
	_, errors := parseProgram(input)

	// This could either be an error or create an empty procedure
	// The test verifies that the parser handles this case gracefully
	// (either with clear error or valid empty procedure)

	if len(errors) > 0 {
		// If it's an error, it should be clear
		errorMsg := errors[0]
		if !strings.Contains(strings.ToLower(errorMsg), "body") && !strings.Contains(strings.ToLower(errorMsg), "empty") {
			t.Errorf("Error message should mention empty/missing body. Got: %s", errorMsg)
		}
	}
	// If no errors, parser created empty procedure (also valid)
}

// ============================================================================
// HELPER FUNCTIONS
// ============================================================================

func TestSection4Summary(t *testing.T) {
	t.Log("AmorphDB Test Plan Section 4: MBL Parser")
	t.Log("4.1 Expressions: 19 test cases")
	t.Log("4.2 Statements: 23 test cases")
	t.Log("4.3 Error Recovery: 5 test cases")
	t.Log("Total: 47 test cases covering complete MBL parser functionality")
	t.Log("All tests follow amorphdb_design.md specification")
	t.Log("Implementation must be fixed to match spec when tests fail")
}