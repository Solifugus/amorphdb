package interpreter

import (
	"strings"
	"testing"

	"github.com/solifugus/amorphdb/internal/types"
)

func TestCatchElseExceptionHandling(t *testing.T) {
	tree := &MockTree{}
	interpreter := New(tree, 1001)

	t.Run("Successful execution - no else taken", func(t *testing.T) {
		input := `catch:
    my.result = "success"
    my.result
else unknown:
    "Operation failed: " & unknown
else queued:
    "Operation delayed: " & queued`

		result, err := interpreter.EvaluateExpression(input)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}

		// Should return "success" since no exception occurred
		if text, ok := result.(types.Text); ok {
			if text.Value != "success" {
				t.Errorf("Expected 'success', got %s", text.Value)
			}
		} else {
			t.Errorf("Expected Text result, got %T: %v", result, result)
		}
	})

	t.Run("Unknown handler - catches Unknown result", func(t *testing.T) {
		input := `catch:
    undefined_function()
else unknown:
    "caught unknown: " & unknown
else queued:
    "caught queued: " & queued`

		result, err := interpreter.EvaluateExpression(input)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}

		// Should catch the Unknown from undefined_function() and return the handler result
		if text, ok := result.(types.Text); ok {
			if !strings.Contains(text.Value, "caught unknown:") {
				t.Errorf("Expected 'caught unknown:' in result, got %s", text.Value)
			}
		} else {
			t.Errorf("Expected Text result from handler, got %T: %v", result, result)
		}
	})

	t.Run("Unknown not caught - propagates when no handler", func(t *testing.T) {
		input := `catch:
    undefined_function()
else queued:
    "caught queued: " & queued`

		result, err := interpreter.EvaluateExpression(input)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}

		// Should return the Unknown since no else unknown handler
		if unknown, ok := result.(types.Unknown); ok {
			if !strings.Contains(unknown.Reason, "undefined_function") {
				t.Errorf("Expected undefined_function error, got: %s", unknown.Reason)
			}
		} else {
			t.Errorf("Expected Unknown result, got %T: %v", result, result)
		}
	})

	t.Run("Multiple expression catch body", func(t *testing.T) {
		input := `catch:
    my.step1 = "first"
    my.step2 = "second"
    undefined_function()
    my.step3 = "third"
else unknown:
    "handled error after: " & my.step2`

		result, err := interpreter.EvaluateExpression(input)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}

		// Should catch Unknown and execute handler
		if text, ok := result.(types.Text); ok {
			if !strings.Contains(text.Value, "handled error after: second") {
				t.Errorf("Expected 'handled error after: second', got %s", text.Value)
			}
		} else {
			t.Errorf("Expected Text result from handler, got %T: %v", result, result)
		}
	})

	t.Run("Unknown variable available in handler", func(t *testing.T) {
		input := `catch:
    undefined_function()
else unknown:
    "Error: " & unknown`

		result, err := interpreter.EvaluateExpression(input)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}

		// Should return concatenated result using the Unknown value
		if text, ok := result.(types.Text); ok {
			if !strings.Contains(text.Value, "Error:") && !strings.Contains(text.Value, "undefined_function") {
				t.Errorf("Expected 'Error:' and 'undefined_function' in result, got: %s", text.Value)
			}
		} else {
			t.Errorf("Expected Text result from concatenation, got %T: %v", result, result)
		}
	})
}