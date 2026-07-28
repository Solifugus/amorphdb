// return_test.go — early-return control flow.
//
// A `return` must exit its enclosing procedure immediately, skipping every
// statement after it in the body and unwinding out of any conditionals and
// loops it sits inside. Before the returnSignal sentinel existed, a return
// only produced the value of its own statement and execution fell through,
// so the LAST return in a body won rather than the first one reached — which
// silently broke guard clauses and base-case recursion.

package interpreter

import (
	"testing"

	"github.com/solifugus/amorphdb/internal/storage"
	"github.com/solifugus/amorphdb/internal/types"
)

// TestReturnFromGuardClauseWins is the minimal reproduction of the defect: a
// guard clause that fires must supply the procedure's result, not the return
// that trails it.
func TestReturnFromGuardClauseWins(t *testing.T) {
	interp := newTestInterpreter(t)

	evalCode(t, interp, "procedure classify(n):\n\tif n <= 0:\n\t\treturn 100\n\treturn 200")

	if got := evalCode(t, interp, "classify(0)"); !compareValues(got, types.Number{Value: 100}) {
		t.Errorf("classify(0): guard clause should win, expected 100, got %v (%T)", got, got)
	}
	// The fall-through path must still reach the trailing return.
	if got := evalCode(t, interp, "classify(5)"); !compareValues(got, types.Number{Value: 200}) {
		t.Errorf("classify(5): expected fall-through to 200, got %v (%T)", got, got)
	}
}

// TestReturnSkipsLaterStatements verifies the statements after a taken return
// genuinely do not execute, rather than merely being overridden by its value.
// The trailing assignment would change the reported value if it ran.
func TestReturnSkipsLaterStatements(t *testing.T) {
	interp := newTestInterpreter(t)

	evalCode(t, interp, "procedure early():\n\tmark = \"first\"\n\treturn mark\n\tmark = \"second\"\n\treturn mark")

	if got := evalCode(t, interp, "early()"); !compareValues(got, types.Text{Value: "first"}) {
		t.Errorf("early(): statements after return must not run, expected \"first\", got %v (%T)", got, got)
	}
}

// TestReturnExitsWhileLoop verifies a return inside a while body unwinds past
// the loop instead of being consumed by it or letting the loop keep spinning.
func TestReturnExitsWhileLoop(t *testing.T) {
	interp := newTestInterpreter(t)

	evalCode(t, interp, "procedure countTo(limit):\n\ti = 0\n\twhile i < 100:\n\t\ti = i + 1\n\t\tif i >= limit:\n\t\t\treturn i\n\treturn -1")

	if got := evalCode(t, interp, "countTo(3)"); !compareValues(got, types.Number{Value: 3}) {
		t.Errorf("countTo(3): expected return from inside while to yield 3, got %v (%T)", got, got)
	}
}

// TestReturnExitsForLoop verifies a return inside a for body exits both the
// loop and the procedure, yielding the FIRST match rather than the last.
func TestReturnExitsForLoop(t *testing.T) {
	interp := newTestInterpreter(t)

	evalCode(t, interp, "procedure firstOver(items):\n\tfor item in items:\n\t\tif item > 10:\n\t\t\treturn item\n\treturn 0")

	// 20 and 30 both qualify; only the first may come back.
	if got := evalCode(t, interp, "firstOver([5, 20, 30])"); !compareValues(got, types.Number{Value: 20}) {
		t.Errorf("firstOver: expected first match 20, got %v (%T)", got, got)
	}
	// No element qualifies — the loop finishes and the trailing return runs.
	if got := evalCode(t, interp, "firstOver([1, 2, 3])"); !compareValues(got, types.Number{Value: 0}) {
		t.Errorf("firstOver: expected 0 when nothing matches, got %v (%T)", got, got)
	}
}

// TestBaseCaseRecursionTerminates covers the practical consequence of the
// defect: the natural recursion shape — base case guarded by an early return,
// recursive call after it — previously never terminated on its own and only
// stopped when the call-depth guard tripped. It must now compute normally.
func TestBaseCaseRecursionTerminates(t *testing.T) {
	interp := newTestInterpreter(t)

	evalCode(t, interp, "procedure factorial(n):\n\tif n <= 1:\n\t\treturn 1\n\treturn n * factorial(n - 1)")

	got := evalCode(t, interp, "factorial(5)")
	if unknown, ok := got.(types.Unknown); ok {
		t.Fatalf("factorial(5) should terminate on its own base case, got Unknown: %s", unknown.Reason)
	}
	if !compareValues(got, types.Number{Value: 120}) {
		t.Errorf("factorial(5): expected 120, got %v (%T)", got, got)
	}
}

// TestBareReturnStopsExecution verifies a valueless `return` still terminates
// the body (yielding Nothing) instead of falling through to what follows.
func TestBareReturnStopsExecution(t *testing.T) {
	interp := newTestInterpreter(t)

	evalCode(t, interp, "procedure stop():\n\treturn\n\treturn \"unreachable\"")

	got := evalCode(t, interp, "stop()")
	if _, ok := got.(types.Nothing); !ok {
		t.Errorf("stop(): bare return should yield Nothing and stop, got %v (%T)", got, got)
	}
}

// TestReturnedUnknownStillPropagates is the failure-path case: returning an
// Unknown must keep surfacing as a bare Unknown so the rollback and catch/else
// machinery still recognises it, rather than being hidden inside the sentinel.
func TestReturnedUnknownStillPropagates(t *testing.T) {
	interp := newTestInterpreter(t)

	evalCode(t, interp, "procedure failing():\n\treturn unknown(\"deliberate\")")

	got := evalCode(t, interp, "failing()")
	unknown, ok := got.(types.Unknown)
	if !ok {
		t.Fatalf("failing(): expected Unknown to propagate, got %v (%T)", got, got)
	}
	if unknown.Reason != "deliberate" {
		t.Errorf("failing(): expected reason %q, got %q", "deliberate", unknown.Reason)
	}
}

// TestTopLevelReturnEndsProgram verifies the program itself is a return
// boundary: a top-level return yields its value as the program result and the
// sentinel never leaks out to the caller as an opaque value.
func TestTopLevelReturnEndsProgram(t *testing.T) {
	tree := &MockTree{data: make(map[string]storage.Value)}
	interp := New(tree, 1001)

	got := evalCode(t, interp, "x = 1\nreturn \"done\"\nx = 2")
	if !compareValues(got, types.Text{Value: "done"}) {
		t.Errorf("top-level return: expected \"done\", got %v (%T)", got, got)
	}
}
