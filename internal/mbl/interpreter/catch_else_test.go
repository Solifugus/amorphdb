package interpreter

import (
	"testing"

	"github.com/solifugus/amorphdb/internal/storage"
	"github.com/solifugus/amorphdb/internal/types"
)

func TestCatchElse_CatchBodySucceeds(t *testing.T) {
	// Test 1: catch body succeeds — else block skipped
	program := `catch:
    x = 5`

	result, err := parseAndEvaluate(program)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	// Result should be 5 (the value assigned), else block never runs
	if numValue, ok := result.(types.Number); ok {
		if numValue.Value != 5 {
			t.Errorf("Expected result 5, got %f", numValue.Value)
		}
	} else {
		t.Errorf("Expected types.Number(5), got %T: %v", result, result)
	}
}

func TestCatchElse_CatchBodyProducesUnknown(t *testing.T) {
	// Test 2: catch body produces Unknown — else block runs
	program := `catch:
    unknown("something_broke")
else unknown:
    "caught: " & unknown`

	result, err := parseAndEvaluate(program)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	// Result should be "caught: something_broke"
	if textValue, ok := result.(types.Text); ok {
		expected := "caught: something_broke"
		if textValue.Value != expected {
			t.Errorf("Expected result '%s', got '%s'", expected, textValue.Value)
		}
	} else {
		t.Errorf("Expected types.Text('caught: something_broke'), got %T: %v", result, result)
	}
}

func TestCatchElse_CatchBodyMultipleStatementsLastUnknown(t *testing.T) {
	// Test 3: catch body has multiple statements, last one is Unknown
	program := `catch:
    x = 5
    unknown("fail")
else unknown:
    "error was: " & unknown`

	result, err := parseAndEvaluate(program)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	// Result should be "error was: fail"
	if textValue, ok := result.(types.Text); ok {
		expected := "error was: fail"
		if textValue.Value != expected {
			t.Errorf("Expected result '%s', got '%s'", expected, textValue.Value)
		}
	} else {
		t.Errorf("Expected types.Text('error was: fail'), got %T: %v", result, result)
	}
}

// Helper function to parse and evaluate MBL code
func parseAndEvaluate(program string) (interface{}, error) {
	tree := storage.NewMemoryTree()
	interpreter := New(tree, 1) // agent 1

	return interpreter.EvaluateExpression(program)
}