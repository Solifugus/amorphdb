package interpreter

import (
	"strings"
	"testing"

	"github.com/solifugus/amorphdb/internal/storage"
	"github.com/solifugus/amorphdb/internal/types"
)

// TestRecursionDepthGuard verifies that unbounded MBL recursion is caught by
// the call-depth guard and reported as a recoverable Unknown, rather than
// exhausting the Go goroutine stack and aborting the process with an
// uncatchable `fatal error: stack overflow`. Before the guard existed, this
// exact script crashed the whole daemon.
func TestRecursionDepthGuard(t *testing.T) {
	tree := &MockTree{data: make(map[string]storage.Value)}
	interp := New(tree, 1001)

	evalCode(t, interp, "procedure loop(n): return loop(n)")
	got := evalCode(t, interp, "loop(1)")

	unknown, ok := got.(types.Unknown)
	if !ok {
		t.Fatalf("infinite recursion should yield Unknown, got %v (%T)", got, got)
	}
	if !strings.Contains(unknown.Reason, "call depth exceeded") {
		t.Errorf("Unknown reason should mention the depth limit, got %q", unknown.Reason)
	}
}

// TestRecursionWithinLimitStillWorks verifies the guard does not break
// legitimate bounded recursion: a terminating recursive procedure must still
// run to completion and return its result rather than tripping the depth
// guard. Termination here comes from guarding the recursive *call* with an
// `if` (so the deepest frame makes no further call), which does not rely on
// early-return-from-a-block semantics.
func TestRecursionWithinLimitStillWorks(t *testing.T) {
	tree := &MockTree{data: make(map[string]storage.Value)}
	interp := New(tree, 1001)

	// countdown(n) recurses n levels deep and unwinds, returning n at the
	// top frame. If the guard mis-fired on a terminating recursion this would
	// come back as an Unknown instead of the expected number.
	evalCode(t, interp, "procedure countdown(n):\n    if n > 0:\n        countdown(n - 1)\n    return n")
	got := evalCode(t, interp, "countdown(5)")

	if !compareValues(got, types.Number{Value: 5}) {
		t.Errorf("countdown(5): expected 5, got %v (%T)", got, got)
	}
}
