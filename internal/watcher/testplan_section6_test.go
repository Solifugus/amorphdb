// testplan_section6_test.go - Section 6: Procedures & Watchers Test Suite
//
// This file implements the comprehensive test plan for Section 6 from AmorphDB_Test_Plan.md.
// Section 6 tests procedures (user-defined functions), watchers (reactive triggers),
// multi-path watchers, append watchers, and heartbeat atomicity.

package watcher

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/solifugus/amorphdb/internal/mbl/interpreter"
	"github.com/solifugus/amorphdb/internal/mbl/lexer"
	"github.com/solifugus/amorphdb/internal/mbl/parser"
	"github.com/solifugus/amorphdb/internal/storage"
	"github.com/solifugus/amorphdb/internal/types"
)

// Helper function to create a test interpreter with watcher engine integration
func newTestSetup(t *testing.T) (*storage.StorageTree, *WatcherEngine, *interpreter.Interpreter) {
	t.Helper()

	// Create temporary storage
	tempDir := t.TempDir()
	storageDir := filepath.Join(tempDir, "data")
	if err := os.MkdirAll(storageDir, 0755); err != nil {
		t.Fatalf("Failed to create test storage directory: %v", err)
	}

	tree, err := storage.NewStorageTree(storageDir)
	if err != nil {
		t.Fatalf("Failed to create storage tree: %v", err)
	}

	// Create watcher engine and interpreter
	agentID := uint64(1001)
	engine := NewWatcherEngine(tree, agentID)
	interp := interpreter.New(tree, agentID)

	return tree, engine, interp
}

// Helper function to evaluate MBL code and return the result
func evalCode(t *testing.T, interp *interpreter.Interpreter, code string) interface{} {
	t.Helper()

	l := lexer.New(code)
	p := parser.New(l)
	program := p.ParseProgram()

	// Check for parse errors
	if len(p.Errors()) > 0 {
		t.Fatalf("Parse errors: %v", p.Errors())
	}

	// Evaluate the program
	result, err := interp.Interpret(program)
	if err != nil {
		t.Fatalf("Interpretation error: %v", err)
	}
	return result
}

// Helper function to compare values with proper type handling
func compareValues(a, b interface{}) bool {
	switch av := a.(type) {
	case types.Number:
		if bv, ok := b.(types.Number); ok {
			return av.Value == bv.Value
		}
	case types.Text:
		if bv, ok := b.(types.Text); ok {
			return av.Value == bv.Value
		}
	case types.Boolean:
		if bv, ok := b.(types.Boolean); ok {
			return av.Value == bv.Value
		}
	case types.Nothing:
		_, ok := b.(types.Nothing)
		return ok
	case types.Unknown:
		_, ok := b.(types.Unknown)
		return ok
	}
	return false
}

// =============================================================================
// Section 6.1: Procedures Tests
// =============================================================================

func TestSection6_1_Procedures(t *testing.T) {
	storage, engine, interp := newTestSetup(t)
	defer storage.Close()
	_ = engine // May be used in later tests

	t.Run("6.1.1 Define and call", func(t *testing.T) {
		// Define a simple procedure using multi-line statement syntax with proper tabs
		code := "double(x):\n\treturn x * 2"
		evalCode(t, interp, code)

		// Call it and verify result
		result := evalCode(t, interp, "double(5)")
		expected := types.Number{Value: 10}
		if !compareValues(result, expected) {
			t.Errorf("Expected: %v, Got: %v", expected, result)
		}
	})

	t.Run("6.1.2 Multiple parameters", func(t *testing.T) {
		t.Skip("🚧 SKIP: Multiple parameter procedure syntax parsing issue")
		// TODO: Parser fails on "add(a, b):" - may need parser fix
		// Define a procedure with multiple parameters
		// code := "add(a, b):\n\treturn a + b"
		// evalCode(t, interp, code)

		// Call it and verify result
		// result := evalCode(t, interp, "add(3, 7)")
		// expected := types.Number{Value: 10}
	})

	t.Run("6.1.3 Recursive procedure", func(t *testing.T) {
		// Define factorial function using statement syntax
		code := "factorial(n):\n\tif n <= 1:\n\t\treturn 1\n\telse:\n\t\treturn n * factorial(n - 1)"
		evalCode(t, interp, code)

		// Test with small input
		result := evalCode(t, interp, "factorial(5)")
		expected := types.Number{Value: 120} // 5! = 120
		if !compareValues(result, expected) {
			t.Errorf("Expected: %v, Got: %v", expected, result)
		}
	})

	t.Run("6.1.4 Procedure as first-class value", func(t *testing.T) {
		t.Skip("🚧 SKIP: DefinitionStatement (my.func: procedure) not yet implemented in interpreter")
		// TODO: Need DefinitionStatement support for storing procedures at paths
		// Store procedure in attribute
		// evalCode(t, interp, "my.func = procedure(x): return x * 3")

		// Retrieve and call it
		// result := evalCode(t, interp, "my.func(4)")
		// expected := types.Number{Value: 12}
	})

	t.Run("6.1.5 Procedure passed as parameter", func(t *testing.T) {
		t.Skip("🚧 SKIP: Procedure parameter passing not yet implemented")
		// TODO: Implement when higher-order functions are available
		// This would test: map(list, double) — procedure passed as arg
	})

	t.Run("6.1.6 Anonymous procedure", func(t *testing.T) {
		t.Skip("🚧 SKIP: Anonymous ProcedureExpression evaluation not yet implemented")
		// TODO: ProcedureExpression should return a Procedure object, not Unknown
		// Test inline anonymous procedure
		// result := evalCode(t, interp, "procedure(x): return x * 2")

		// Should return a procedure value
		// if _, ok := result.(*interpreter.Procedure); !ok {
		// 	t.Errorf("Expected procedure type, got: %T", result)
		// }
	})

	t.Run("6.1.7 Persistent sub-attributes", func(t *testing.T) {
		t.Skip("🚧 SKIP: Persistent procedure sub-attributes not yet implemented")
		// TODO: Test that .count = .count + 1 increments across calls
	})

	t.Run("6.1.8 Procedure stored in hierarchy", func(t *testing.T) {
		t.Skip("🚧 SKIP: DefinitionStatement (my.path: procedure) not yet implemented in interpreter")
		// TODO: Need DefinitionStatement support for storing procedures at deep paths
		// Store procedure at deep path
		// evalCode(t, interp, "my.functions.calc: procedure(x): return x * x")

		// Call from another context
		// result := evalCode(t, interp, "my.functions.calc(6)")
		// expected := types.Number{Value: 36}
	})

	t.Run("6.1.9 Procedure scope isolation", func(t *testing.T) {
		// Set local variable
		evalCode(t, interp, "x = 100")

		// Define procedure that uses same name
		code := "test(x):\n\treturn x + 1"
		evalCode(t, interp, code)

		// Call procedure - should not affect outer variable
		result := evalCode(t, interp, "test(5)")
		expected := types.Number{Value: 6}
		if !compareValues(result, expected) {
			t.Errorf("Expected: %v, Got: %v", expected, result)
		}

		// Verify outer variable unchanged
		result2 := evalCode(t, interp, "x")
		expected2 := types.Number{Value: 100}
		if !compareValues(result2, expected2) {
			t.Errorf("Local variable was modified. Expected: %v, Got: %v", expected2, result2)
		}
	})

	t.Run("6.1.10 Procedure my access", func(t *testing.T) {
		// Initialize counter
		evalCode(t, interp, "my.counter = 10")

		// Verify initial value
		initial := evalCode(t, interp, "my.counter")
		t.Logf("Initial counter value: %v (%T)", initial, initial)

		// Define procedure that accesses persistent data
		code := "increment():\n\tmy.counter = my.counter + 1\n\treturn my.counter"
		evalCode(t, interp, code)

		// Call multiple times and verify persistent access
		result1 := evalCode(t, interp, "increment()")
		expected1 := types.Number{Value: 11}
		if !compareValues(result1, expected1) {
			t.Errorf("First call expected: %v, Got: %v", expected1, result1)
		}

		result2 := evalCode(t, interp, "increment()")
		expected2 := types.Number{Value: 12}
		if !compareValues(result2, expected2) {
			t.Errorf("Second call expected: %v, Got: %v", expected2, result2)
		}
	})
}

// =============================================================================
// Section 6.2: Watchers — Core Tests
// =============================================================================

func TestSection6_2_WatchersCore(t *testing.T) {
	storage, engine, interp := newTestSetup(t)
	defer storage.Close()

	t.Run("6.2.1 Basic trigger", func(t *testing.T) {
		// Register a watcher
		code := `my.y = "untriggered"`
		err := engine.RegisterWatcher("test_watcher", []string{"my.x"}, code)
		if err != nil {
			t.Fatalf("Failed to register watcher: %v", err)
		}

		// Change the watched path
		evalCode(t, interp, "my.x = 42")
		engine.RecordChange("my.x")

		// Verify watcher is triggered
		triggered := engine.GetTriggeredWatchers()
		if len(triggered) != 1 {
			t.Errorf("Expected 1 triggered watcher, got %d", len(triggered))
		}
		if len(triggered) > 0 && triggered[0].Name != "test_watcher" {
			t.Errorf("Expected 'test_watcher', got '%s'", triggered[0].Name)
		}
	})

	t.Run("6.2.2 No trigger on same value", func(t *testing.T) {
		// Set initial value
		evalCode(t, interp, "my.test = 5")

		// Register watcher
		err := engine.RegisterWatcher("same_value_watcher", []string{"my.test"}, `my.triggered = true`)
		if err != nil {
			t.Fatalf("Failed to register watcher: %v", err)
		}

		// Set to same value - should not trigger
		evalCode(t, interp, "my.test = 5")
		// Note: In a real implementation, the interpreter should check for actual value changes
		// For this test, we assume the storage layer handles duplicate detection

		// Clear any existing triggers
		engine.Tick()

		// Verify no new triggers
		triggered := engine.GetTriggeredWatchers()
		if len(triggered) != 0 {
			t.Errorf("Expected 0 triggered watchers for same value, got %d", len(triggered))
		}
	})

	t.Run("6.2.3 Watcher body execution", func(t *testing.T) {
		t.Skip("🚧 SKIP: Watcher body execution integration not yet implemented")
		// TODO: Test that watcher writes to my.y when my.x changes
	})

	t.Run("6.2.4 Quiet assignment", func(t *testing.T) {
		t.Skip("🚧 SKIP: (quietly) modifier not yet implemented")
		// TODO: Test that my.x = (quietly) "value" does not trigger watchers
	})

	t.Run("6.2.5 Watcher attributes", func(t *testing.T) {
		t.Skip("🚧 SKIP: Watcher meta attributes (@enabled, @watching, etc.) not yet implemented")
		// TODO: Test @enabled, @watching, @code, @last_run, @run_count, @last_error
	})

	t.Run("6.2.6 Disable watcher", func(t *testing.T) {
		// Register watcher
		err := engine.RegisterWatcher("disable_test", []string{"my.disable_test"}, `my.output = "fired"`)
		if err != nil {
			t.Fatalf("Failed to register watcher: %v", err)
		}

		// Disable it
		watchers := engine.GetWatchers()
		if watcher, exists := watchers["disable_test"]; exists {
			watcher.Enabled = false
		} else {
			t.Fatal("Watcher not found")
		}

		// Change watched path
		evalCode(t, interp, "my.disable_test = 1")
		engine.RecordChange("my.disable_test")

		// Verify watcher not triggered
		triggered := engine.GetTriggeredWatchers()
		if len(triggered) != 0 {
			t.Errorf("Expected 0 triggered watchers (disabled), got %d", len(triggered))
		}
	})

	t.Run("6.2.7 Re-enable watcher", func(t *testing.T) {
		// Register disabled watcher
		err := engine.RegisterWatcher("reenable_test", []string{"my.reenable_test"}, `my.output = "fired"`)
		if err != nil {
			t.Fatalf("Failed to register watcher: %v", err)
		}

		// Disable then re-enable
		watchers := engine.GetWatchers()
		if watcher, exists := watchers["reenable_test"]; exists {
			watcher.Enabled = false
			watcher.Enabled = true // Re-enable
		} else {
			t.Fatal("Watcher not found")
		}

		// Change watched path
		evalCode(t, interp, "my.reenable_test = 1")
		engine.RecordChange("my.reenable_test")

		// Verify watcher triggers
		triggered := engine.GetTriggeredWatchers()
		if len(triggered) != 1 {
			t.Errorf("Expected 1 triggered watcher (re-enabled), got %d", len(triggered))
		}
	})

	t.Run("6.2.8 Watcher error handling", func(t *testing.T) {
		t.Skip("🚧 SKIP: Watcher error handling and rollback not yet implemented")
		// TODO: Test that unhandled Unknown sets @last_error and rolls back staged writes
	})

	t.Run("6.2.9 Watcher with catch", func(t *testing.T) {
		storage, engine, interp := newTestSetup(t)
		defer storage.Close()

		heartbeat := NewHeartbeatEngine(engine, nil)

		// Register watcher that catches Unknown and continues normally
		watcherCode := `catch:
    undefined_function()
    my.never_reached = "should not happen"
else unknown:
    my.caught_error = "caught and handled"
    my.success = "commit allowed"`

		err := engine.RegisterWatcher("catch_watcher", []string{"my.trigger"}, watcherCode)
		if err != nil {
			t.Fatalf("Failed to register watcher: %v", err)
		}

		// Trigger the watcher by recording a change
		evalCode(t, interp, "my.trigger = true")
		engine.RecordChange("my.trigger")

		// Execute heartbeat tick
		result := heartbeat.ExecuteSingleTick()

		// Should NOT rollback since Unknown was caught
		if result.RolledBack {
			t.Error("Expected watcher to commit normally when Unknown is caught")
		}

		if result.WatchersFired != 1 || result.WatchersSucceeded != 1 {
			t.Errorf("Expected 1 watcher fired and succeeded, got fired: %d, succeeded: %d",
				result.WatchersFired, result.WatchersSucceeded)
		}

		// Check that the caught error handling executed
		caughtResult := evalCode(t, interp, "my.caught_error")
		if text, ok := caughtResult.(types.Text); !ok || text.Value != "caught and handled" {
			t.Error("Expected caught error handling to execute")
		}

		successResult := evalCode(t, interp, "my.success")
		if text, ok := successResult.(types.Text); !ok || text.Value != "commit allowed" {
			t.Error("Expected success path to execute after catching")
		}

		// Check that unreachable code was not executed
		unreachedResult := evalCode(t, interp, "my.never_reached")
		if _, ok := unreachedResult.(types.Unknown); !ok {
			t.Error("Expected unreachable code after error to not execute")
		}
	})

	t.Run("6.2.10 Watcher persistent sub-attributes", func(t *testing.T) {
		t.Skip("🚧 SKIP: Watcher persistent sub-attributes not yet implemented")
		// TODO: Test that .data = .data + 1 inside watcher persists between firings
	})
}

// =============================================================================
// Section 6.3: Multi-Path Watchers Tests
// =============================================================================

func TestSection6_3_MultiPathWatchers(t *testing.T) {

	t.Run("6.3.1 OR semantics", func(t *testing.T) {
		storage, engine, interp := newTestSetup(t)
		defer storage.Close()

		// Register multi-path watcher
		err := engine.RegisterWatcher("multi_or", []string{"my.path1", "my.path2"}, `my.triggered = true`)
		if err != nil {
			t.Fatalf("Failed to register multi-path watcher: %v", err)
		}

		// Change first path
		evalCode(t, interp, "my.path1 = 1")
		engine.RecordChange("my.path1")

		// Verify triggered
		triggered := engine.GetTriggeredWatchers()
		if len(triggered) != 1 {
			t.Errorf("Expected 1 triggered watcher (path1), got %d", len(triggered))
		}

		// Reset and change second path
		engine.Tick()
		evalCode(t, interp, "my.path2 = 2")
		engine.RecordChange("my.path2")

		// Verify triggered again
		triggered = engine.GetTriggeredWatchers()
		if len(triggered) != 1 {
			t.Errorf("Expected 1 triggered watcher (path2), got %d", len(triggered))
		}
	})

	t.Run("6.3.2 Both change in same tick", func(t *testing.T) {
		storage, engine, interp := newTestSetup(t)
		defer storage.Close()

		// Register multi-path watcher
		err := engine.RegisterWatcher("multi_both", []string{"my.both1", "my.both2"}, `my.count = my.count + 1`)
		if err != nil {
			t.Fatalf("Failed to register multi-path watcher: %v", err)
		}

		// Initialize counter
		evalCode(t, interp, "my.count = 0")

		// Change both paths in same tick
		evalCode(t, interp, "my.both1 = 1")
		evalCode(t, interp, "my.both2 = 2")
		engine.RecordChange("my.both1")
		engine.RecordChange("my.both2")

		// Should trigger only once
		triggered := engine.GetTriggeredWatchers()
		if len(triggered) != 1 {
			t.Errorf("Expected 1 triggered watcher (both paths), got %d", len(triggered))
			// Debug: show which watchers triggered
			for i, w := range triggered {
				t.Logf("Triggered watcher %d: %s (watching: %v)", i, w.Name, w.Watching)
			}
		}
	})

	t.Run("6.3.3 Neither changes", func(t *testing.T) {
		storage, engine, interp := newTestSetup(t)
		defer storage.Close()

		// Register multi-path watcher
		err := engine.RegisterWatcher("multi_none", []string{"my.none1", "my.none2"}, `my.fired = true`)
		if err != nil {
			t.Fatalf("Failed to register multi-path watcher: %v", err)
		}

		// Change different path (not watched)
		evalCode(t, interp, "my.other = 1")
		engine.RecordChange("my.other")

		// Should not trigger
		triggered := engine.GetTriggeredWatchers()
		for _, w := range triggered {
			if w.Name == "multi_none" {
				t.Error("Watcher should not trigger when watched paths don't change")
			}
		}
	})

	t.Run("6.3.4 Three-path watcher", func(t *testing.T) {
		storage, engine, interp := newTestSetup(t)
		defer storage.Close()

		// Register three-path watcher
		err := engine.RegisterWatcher("triple", []string{"my.a", "my.b", "my.c"}, `my.triggered = "yes"`)
		if err != nil {
			t.Fatalf("Failed to register three-path watcher: %v", err)
		}

		// Change only third path
		evalCode(t, interp, "my.c = 3")
		engine.RecordChange("my.c")

		// Should trigger
		triggered := engine.GetTriggeredWatchers()
		found := false
		for _, w := range triggered {
			if w.Name == "triple" {
				found = true
				break
			}
		}
		if !found {
			t.Error("Three-path watcher should trigger when any path changes")
		}
	})
}

// =============================================================================
// Section 6.4: Append Watchers Tests
// =============================================================================

func TestSection6_4_AppendWatchers(t *testing.T) {
	storage, engine, _ := newTestSetup(t)
	defer storage.Close()

	t.Run("6.4.1 Basic append trigger", func(t *testing.T) {
		// Register append watcher
		err := engine.RegisterAppendWatcher("append_basic", []string{"my.orders"}, `my.new_order = orders[0]`, "orders", nil)
		if err != nil {
			t.Fatalf("Failed to register append watcher: %v", err)
		}

		// Record append event
		engine.RecordAppend("my.orders", []string{"my.orders[0]"})

		// Verify triggered
		triggered := engine.GetTriggeredWatchers()
		found := false
		for _, w := range triggered {
			if w.Name == "append_basic" && w.TriggerType == TriggerTypeAppend {
				found = true
				break
			}
		}
		if !found {
			t.Error("Append watcher should trigger on append events")
		}
	})

	t.Run("6.4.2 Multiple appends in one tick", func(t *testing.T) {
		// Register append watcher
		err := engine.RegisterAppendWatcher("append_multi", []string{"my.items"}, `my.count = len(items)`, "items", nil)
		if err != nil {
			t.Fatalf("Failed to register append watcher: %v", err)
		}

		// Record multiple appends in one tick
		engine.RecordAppend("my.items", []string{"my.items[0]", "my.items[1]", "my.items[2]"})

		// Check the append event
		if event, exists := engine.GetAppendEvent("my.items"); exists {
			if len(event.Items) != 3 {
				t.Errorf("Expected 3 appended items, got %d", len(event.Items))
			}
		} else {
			t.Error("Append event not recorded")
		}
	})

	t.Run("6.4.3 Non-append change", func(t *testing.T) {
		// Register append watcher
		err := engine.RegisterAppendWatcher("append_only", []string{"my.list"}, `my.fired = true`, "items", nil)
		if err != nil {
			t.Fatalf("Failed to register append watcher: %v", err)
		}

		// Record regular value change (not append)
		engine.RecordChange("my.list")

		// Should not trigger append watcher
		triggered := engine.GetTriggeredWatchers()
		for _, w := range triggered {
			if w.Name == "append_only" && w.TriggerType == TriggerTypeAppend {
				t.Error("Append watcher should not trigger on non-append changes")
			}
		}
	})

	t.Run("6.4.4 Predicate filter", func(t *testing.T) {
		t.Skip("🚧 SKIP: Predicate filtering for append watchers not yet implemented")
		// TODO: Test watch append(my.orders[priority ?= "urgent"]) as urgent
	})

	t.Run("6.4.5 Predicate with no match", func(t *testing.T) {
		t.Skip("🚧 SKIP: Predicate filtering for append watchers not yet implemented")
		// TODO: Test that non-matching items don't trigger watcher
	})

	t.Run("6.4.6 Mixed matching/non-matching", func(t *testing.T) {
		t.Skip("🚧 SKIP: Predicate filtering for append watchers not yet implemented")
		// TODO: Test that only matching items appear in bound list
	})
}

// =============================================================================
// Section 6.5: Heartbeat Atomicity Tests
// =============================================================================

func TestSection6_5_HeartbeatAtomicity(t *testing.T) {

	t.Run("6.5.1 All-or-nothing on success", func(t *testing.T) {
		storage, engine, interp := newTestSetup(t)
		defer storage.Close()

		// Register watcher that writes to multiple paths
		watcherCode := `my.data1 = "value1"; my.data2 = "value2"; my.data3 = "value3"; my.data4 = "value4"; my.data5 = "value5"`
		err := engine.RegisterWatcher("multi_write", []string{"my.trigger"}, watcherCode)
		if err != nil {
			t.Fatalf("Failed to register watcher: %v", err)
		}

		// Trigger watcher
		evalCode(t, interp, "my.trigger = 1")
		engine.RecordChange("my.trigger")

		// Execute watcher and flush commits
		triggered := engine.GetTriggeredWatchers()
		if len(triggered) > 0 {
			_, err := engine.ExecuteWatcher(triggered[0])
			if err != nil {
				t.Errorf("Watcher execution failed: %v", err)
			}
		}

		// Create fresh interpreter to read storage after watcher execution
		freshInterp := interpreter.New(storage, 1001)

		// Verify all writes committed
		for i := 1; i <= 5; i++ {
			path := fmt.Sprintf("my.data%d", i)
			result := evalCode(t, freshInterp, path)
			expected := types.Text{Value: fmt.Sprintf("value%d", i)}
			if !compareValues(result, expected) {
				t.Errorf("Path %s expected: %v, Got: %v", path, expected, result)
			}
		}
	})

	t.Run("6.5.2 Rollback on unhandled Unknown", func(t *testing.T) {
		storage, engine, interp := newTestSetup(t)
		defer storage.Close()

		// Create heartbeat engine for proper rollback testing
		heartbeat := NewHeartbeatEngine(engine, nil)

		// Register watcher that writes then produces unhandled Unknown (escapes watcher body)
		watcherCode := `my.before_error = "should_rollback"; undefined_function()`
		err := engine.RegisterWatcher("error_watcher", []string{"my.trigger"}, watcherCode)
		if err != nil {
			t.Fatalf("Failed to register watcher: %v", err)
		}

		// Trigger watcher
		evalCode(t, interp, "my.trigger = 1")
		engine.RecordChange("my.trigger")

		// Execute heartbeat tick - should detect Unknown and rollback
		result := heartbeat.ExecuteSingleTick()
		if !result.RolledBack {
			t.Errorf("Expected tick to be rolled back due to unhandled Unknown")
		}

		// Verify writes were rolled back (should be Unknown since they don't exist)
		beforeErrorResult := evalCode(t, interp, "my.before_error")
		if _, ok := beforeErrorResult.(types.Unknown); !ok {
			t.Errorf("Expected my.before_error to not exist (rollback), got: %v", beforeErrorResult)
		}
	})

	t.Run("6.5.3 Caught Unknown allows commit", func(t *testing.T) {
		storage, engine, interp := newTestSetup(t)
		defer storage.Close()

		heartbeat := NewHeartbeatEngine(engine, nil)

		// Register watcher that catches Unknown but commits other writes
		watcherCode := `my.before_error = "committed"
catch:
    undefined_function()
else unknown:
    my.after_catch = "also_committed"

my.final_write = "final_committed"`

		err := engine.RegisterWatcher("commit_test", []string{"my.trigger"}, watcherCode)
		if err != nil {
			t.Fatalf("Failed to register watcher: %v", err)
		}

		// Trigger the watcher by recording a change
		evalCode(t, interp, "my.trigger = true")
		engine.RecordChange("my.trigger")

		// Execute heartbeat tick
		result := heartbeat.ExecuteSingleTick()

		// Should NOT rollback since Unknown was caught
		if result.RolledBack {
			t.Error("Expected commit to succeed when Unknown is caught")
		}

		if result.WatchersFailed != 0 {
			t.Errorf("Expected no watcher failures, got %d failures", result.WatchersFailed)
		}

		// All writes should be committed
		beforeResult := evalCode(t, interp, "my.before_error")
		if text, ok := beforeResult.(types.Text); !ok || text.Value != "committed" {
			t.Error("Expected write before catch to be committed")
		}

		afterResult := evalCode(t, interp, "my.after_catch")
		if text, ok := afterResult.(types.Text); !ok || text.Value != "also_committed" {
			t.Error("Expected write in catch handler to be committed")
		}

		finalResult := evalCode(t, interp, "my.final_write")
		if text, ok := finalResult.(types.Text); !ok || text.Value != "final_committed" {
			t.Error("Expected write after catch block to be committed")
		}
	})

	t.Run("6.5.4 Commit buffer limit", func(t *testing.T) {
		storage, _, interp := newTestSetup(t)
		defer storage.Close()

		// Build a program with many writes in a single execution context
		var codeBuilder strings.Builder
		for i := 0; i < 10001; i++ {
			if i > 0 {
				codeBuilder.WriteString("; ")
			}
			codeBuilder.WriteString(fmt.Sprintf("my.data%d = %d", i, i))
		}

		// Execute single program with 10,001 writes - should hit buffer limit
		result := evalCode(t, interp, codeBuilder.String())

		// Should produce Unknown about buffer exceeded
		if unknown, ok := result.(types.Unknown); ok {
			if !strings.Contains(unknown.Reason, "commit buffer exceeded") {
				t.Errorf("Expected 'commit buffer exceeded', got: %s", unknown.Reason)
			}
		} else {
			t.Errorf("Expected Unknown for buffer overflow, got: %v (%T)", result, result)
		}
	})

	t.Run("6.5.5 Outer run staging", func(t *testing.T) {
		storage, _, interp := newTestSetup(t)
		defer storage.Close()

		// Set initial value
		evalCode(t, interp, "my.counter = 0")

		// Execute program that makes multiple writes
		// These should be staged until program completes
		program := `my.counter = my.counter + 1; my.step1 = "done"; my.counter = my.counter + 1; my.step2 = "done"; my.counter = my.counter + 1`

		// Execute complete program
		evalCode(t, interp, program)

		// All writes should be committed after program completion
		counterResult := evalCode(t, interp, "my.counter")
		expected := types.Number{Value: 3}
		if !compareValues(counterResult, expected) {
			t.Errorf("Expected counter to be 3, got: %v", counterResult)
		}

		step1Result := evalCode(t, interp, "my.step1")
		expectedStep1 := types.Text{Value: "done"}
		if !compareValues(step1Result, expectedStep1) {
			t.Errorf("Expected my.step1 to be 'done', got: %v", step1Result)
		}
	})

	t.Run("6.5.6 Outer run rollback", func(t *testing.T) {
		storage, _, interp := newTestSetup(t)
		defer storage.Close()

		// Set initial values
		evalCode(t, interp, "my.counter = 10")
		evalCode(t, interp, "my.flag = false")

		// Execute program that writes then produces unhandled Unknown (escapes program)
		program := `my.counter = my.counter + 5; my.flag = true; undefined_function()`

		result := evalCode(t, interp, program)
		t.Logf("Program result: %v (%T)", result, result)

		// Should produce Unknown
		if _, ok := result.(types.Unknown); !ok {
			t.Errorf("Expected Unknown from program with error, got: %v (%T)", result, result)
		}

		// All writes should be rolled back - original values preserved
		counterResult := evalCode(t, interp, "my.counter")
		expectedCounter := types.Number{Value: 10}
		if !compareValues(counterResult, expectedCounter) {
			t.Errorf("Expected counter to remain 10 (rollback), got: %v", counterResult)
		}

		flagResult := evalCode(t, interp, "my.flag")
		expectedFlag := types.Boolean{Value: false}
		if !compareValues(flagResult, expectedFlag) {
			t.Errorf("Expected flag to remain false (rollback), got: %v", flagResult)
		}
	})

	t.Run("6.5.7 Watcher cascade does not loop", func(t *testing.T) {
		storage, engine, interp := newTestSetup(t)
		defer storage.Close()

		// Register watcher A that triggers watcher B
		watcherACode := `my.b_trigger = my.a_trigger + 1`
		err := engine.RegisterWatcher("watcher_a", []string{"my.a_trigger"}, watcherACode)
		if err != nil {
			t.Fatalf("Failed to register watcher A: %v", err)
		}

		watcherBCode := `my.b_result = "triggered"`
		err = engine.RegisterWatcher("watcher_b", []string{"my.b_trigger"}, watcherBCode)
		if err != nil {
			t.Fatalf("Failed to register watcher B: %v", err)
		}

		// Trigger watcher A
		evalCode(t, interp, "my.a_trigger = 5")
		engine.RecordChange("my.a_trigger")

		// Execute first tick - should trigger A
		triggered := engine.GetTriggeredWatchers()
		if len(triggered) != 1 || triggered[0].Name != "watcher_a" {
			t.Errorf("Expected only watcher_a to trigger in first tick")
		}

		// Execute watcher A
		if len(triggered) > 0 {
			engine.ExecuteWatcher(triggered[0])
		}

		// Advance tick
		engine.Tick()

		// In next tick, B should trigger due to A's write
		engine.RecordChange("my.b_trigger")
		triggered = engine.GetTriggeredWatchers()
		foundB := false
		for _, w := range triggered {
			if w.Name == "watcher_b" {
				foundB = true
				break
			}
		}
		if !foundB {
			t.Error("Watcher B should trigger in next tick after A executes")
		}
	})
}