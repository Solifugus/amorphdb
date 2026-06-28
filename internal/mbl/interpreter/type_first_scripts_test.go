package interpreter

import (
	"io/ioutil"
	"path/filepath"
	"testing"

	"github.com/solifugus/amorphdb/internal/storage"
	"github.com/solifugus/amorphdb/internal/types"
)

// TestTypeFirstProcedureScript loads the type-first procedure
// demonstration script (Step 2.6 of docs/syntax_changes_development_plan.md)
// from tests/unit/ and runs it end-to-end through the interpreter.
// The script exercises type-first, path-first, and anonymous-assignment
// procedure definitions; this test verifies that calls into each
// resolve to the expected values, confirming the .mbl script works.
func TestTypeFirstProcedureScript(t *testing.T) {
	scriptPath := filepath.Join("..", "..", "..", "tests", "unit", "type_first_procedure_test.mbl")
	body, err := ioutil.ReadFile(scriptPath)
	if err != nil {
		t.Fatalf("failed to read %s: %v", scriptPath, err)
	}

	tree := &MockTree{data: make(map[string]storage.Value)}
	interp := New(tree, 1001)

	evalCode(t, interp, string(body))

	// After running the script, all procedures should be bound; verify
	// each is callable and returns the expected value.
	cases := []struct {
		expr string
		want interface{}
	}{
		{"double(21)", types.Number{Value: 42}},
		{"calculate_tax(100, 0.25)", types.Number{Value: 25}},
		{"ping()", types.Text{Value: "pong"}},
		{`greet("World")`, types.Text{Value: "hello, World"}},
		{"plus_one(square(4))", types.Number{Value: 17}},
		{"my.functions.triple(7)", types.Number{Value: 21}},
		{"my.handlers.negate(5)", types.Number{Value: -5}},
	}
	for _, c := range cases {
		got := evalCode(t, interp, c.expr)
		if !compareValues(got, c.want) {
			t.Errorf("%s: expected %v (%T), got %v (%T)", c.expr, c.want, c.want, got, got)
		}
	}
}

// TestTypeFirstWatcherScript loads the type-first watcher demonstration
// script (Step 2.6) and verifies that each watcher definition binds in
// the interpreter's watcher registry at the expected storage path.
// Firing behavior is covered by TestTypeFirstWatcherEvaluation; this
// test asserts the script-level definitions land where Step 2.5 says
// they should.
func TestTypeFirstWatcherScript(t *testing.T) {
	scriptPath := filepath.Join("..", "..", "..", "tests", "unit", "type_first_watcher_test.mbl")
	body, err := ioutil.ReadFile(scriptPath)
	if err != nil {
		t.Fatalf("failed to read %s: %v", scriptPath, err)
	}

	tree := &MockTree{data: make(map[string]storage.Value)}
	interp := New(tree, 1001)

	evalCode(t, interp, string(body))

	expectedKeys := []string{
		"world.agent.1001.balance_check",
		"world.agent.1001.multi_check",
		"world.agent.1001.new_orders",
		"world.agent.1001.urgent_orders",
		"world.agent.1001.handlers.legacy",
		"world.agent.1001.handlers.assigned",
	}
	for _, key := range expectedKeys {
		if _, found := interp.watchers[key]; !found {
			t.Errorf("expected watcher to be bound at %q; registry has %v", key, watcherKeys(interp))
		}
	}
}
