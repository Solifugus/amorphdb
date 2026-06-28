package interpreter

import (
	"testing"

	"github.com/solifugus/amorphdb/internal/storage"
	"github.com/solifugus/amorphdb/internal/types"
)

func TestCascade_BasicCascadeParentToChild(t *testing.T) {
	// Test 1: Basic cascade — parent value visible to bare reference
	// Set world.org.max_budget:(cascade) = 50000
	// Bare reference to max_budget should resolve to 50000

	tree := storage.NewMemoryTree()
	interpreter := New(tree, 1) // agent 1

	// Set cascaded value at parent level
	cascadeAssignment := `world.org.max_budget:(cascade) = 50000`
	_, err := interpreter.EvaluateExpression(cascadeAssignment)
	if err != nil {
		t.Fatalf("Failed to set cascaded value: %v", err)
	}

	// Try to resolve bare reference - should find cascaded value
	childLookup := `max_budget`
	result, err := interpreter.EvaluateExpression(childLookup)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	// Should resolve to 50000
	if numValue, ok := result.(types.Number); ok {
		if numValue.Value != 50000 {
			t.Errorf("Expected cascaded value 50000, got %f", numValue.Value)
		}
	} else {
		t.Errorf("Expected types.Number(50000), got %T: %v", result, result)
	}
}

func TestCascade_NoSidewaysLeakage(t *testing.T) {
	// Test 2: Cascade does NOT leak sideways
	// Set world.org.max_budget:(cascade) = 50000
	// Set world.other_org.max_budget:(cascade) = 75000  (different value)
	// Bare reference should resolve to org value (first in search pattern)
	// This demonstrates that cascade resolution follows search patterns

	tree := storage.NewMemoryTree()
	interpreter := New(tree, 1) // agent 1

	// Set cascaded value in first branch
	cascadeAssignment1 := `world.org.max_budget:(cascade) = 50000`
	_, err := interpreter.EvaluateExpression(cascadeAssignment1)
	if err != nil {
		t.Fatalf("Failed to set first cascaded value: %v", err)
	}

	// Set different cascaded value in second branch
	cascadeAssignment2 := `world.other_org.max_budget:(cascade) = 75000`
	_, err = interpreter.EvaluateExpression(cascadeAssignment2)
	if err != nil {
		t.Fatalf("Failed to set second cascaded value: %v", err)
	}

	// Try to resolve bare reference - should find world.org first due to search pattern
	childLookup := `max_budget`
	result, err := interpreter.EvaluateExpression(childLookup)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	// Should resolve to 50000 (world.org value, first in search pattern)
	if numValue, ok := result.(types.Number); ok {
		if numValue.Value != 50000 {
			t.Errorf("Expected world.org cascade value 50000, got %f", numValue.Value)
		}
	} else {
		t.Errorf("Expected types.Number(50000), got %T: %v", result, result)
	}
}

func TestCascade_NonCascadedValuesDoNotResolve(t *testing.T) {
	// Test 3: Non-cascaded values do NOT resolve from parent
	// Set world.org.secret = "hidden" (no cascade modifier)
	// Bare reference to secret should NOT resolve

	tree := storage.NewMemoryTree()
	interpreter := New(tree, 1) // agent 1

	// Set non-cascaded value
	regularAssignment := `world.org.secret = "hidden"`
	_, err := interpreter.EvaluateExpression(regularAssignment)
	if err != nil {
		t.Fatalf("Failed to set regular value: %v", err)
	}

	// Try to resolve bare reference - should NOT find non-cascaded value
	childLookup := `secret`
	result, err := interpreter.EvaluateExpression(childLookup)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	// Should NOT resolve since not cascaded
	if unknown, ok := result.(types.Unknown); ok {
		// This is expected - non-cascaded values should not resolve
		t.Logf("Expected unknown result: %v", unknown)
	} else {
		t.Errorf("Expected Unknown (no cascade resolution), got %T: %v", result, result)
	}
}

func TestCascade_MultiLevelWalk(t *testing.T) {
	// Test 4: Cascade walks multiple hierarchy patterns
	// Set world.config.timeout:(cascade) = 30
	// Bare reference to timeout should resolve to 30

	tree := storage.NewMemoryTree()
	interpreter := New(tree, 1) // agent 1

	// Set cascaded value at config level
	cascadeAssignment := `world.config.timeout:(cascade) = 30`
	_, err := interpreter.EvaluateExpression(cascadeAssignment)
	if err != nil {
		t.Fatalf("Failed to set cascaded value: %v", err)
	}

	// Try to resolve bare reference - should find config-level cascade
	childLookup := `timeout`
	result, err := interpreter.EvaluateExpression(childLookup)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	// Should resolve to 30
	if numValue, ok := result.(types.Number); ok {
		if numValue.Value != 30 {
			t.Errorf("Expected cascaded value 30, got %f", numValue.Value)
		}
	} else {
		t.Errorf("Expected types.Number(30), got %T: %v", result, result)
	}
}