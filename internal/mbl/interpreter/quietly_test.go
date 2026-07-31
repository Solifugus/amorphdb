// Tests for the (quietly) assignment modifier.
//
// (quietly) has two jobs: the assignment must still write, and the write must
// not announce itself to the watcher engine. Before this was fixed the first job
// failed outright — the parser only recognised the modifier after ':' (the
// definition form), so `my.x = (quietly) 5` fell to the QUIETLY prefix parser,
// built a binary expression whose operator was a space, evaluated to
// Unknown("unsupported binary operator:  "), and silently wrote nothing.
package interpreter

import (
	"testing"

	"github.com/solifugus/amorphdb/internal/storage"
	"github.com/solifugus/amorphdb/internal/types"
)

// TestQuietlyAssignmentStillWrites is the correctness half: a quiet write is
// still a write. This is the regression test for silent data loss.
func TestQuietlyAssignmentStillWrites(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want interface{}
	}{
		{"text", `my.status = (quietly) "overlimit"`, types.Text{Value: "overlimit"}},
		{"number", `my.count = (quietly) 42`, types.Number{Value: 42}},
		{"boolean", `my.flag = (quietly) true`, types.Boolean{Value: true}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, interp := freshTreeInterp(t, "quietly")

			got := runMBL(t, interp, tt.src)
			if unknown, isUnknown := got.(types.Unknown); isUnknown {
				t.Fatalf("assignment evaluated to Unknown(%s) — the write was lost", unknown.Reason)
			}
			if got != tt.want {
				t.Errorf("assignment result = %#v, want %#v", got, tt.want)
			}

			// And it must actually be readable afterwards.
			parts := map[string]string{"text": "my.status", "number": "my.count", "boolean": "my.flag"}
			back := runMBL(t, interp, parts[tt.name])
			if back != tt.want {
				t.Errorf("read back = %#v, want %#v", back, tt.want)
			}
		})
	}
}

// TestQuietlyMatchesLoudValue confirms the modifier changes only the
// notification, never the stored value.
func TestQuietlyMatchesLoudValue(t *testing.T) {
	_, quiet := freshTreeInterp(t, "q-quiet")
	_, loud := freshTreeInterp(t, "q-loud")

	runMBL(t, quiet, `my.x = (quietly) "same"`)
	runMBL(t, loud, `my.x = "same"`)

	q := runMBL(t, quiet, `my.x`)
	l := runMBL(t, loud, `my.x`)
	if q != l {
		t.Errorf("quiet stored %#v, loud stored %#v — the modifier changed the value", q, l)
	}
}

// TestQuietWritesAreWithheldFromWatchers is the suppression half. GetWrittenPaths
// is what the watcher engine uses to decide what fires next tick, so a quiet
// write must not appear there while a loud one must.
//
// Staging is exercised directly because Interpret flushes and clears the commit
// buffer, which would empty the list before it could be inspected.
func TestQuietWritesAreWithheldFromWatchers(t *testing.T) {
	_, interp := freshTreeInterp(t, "q-paths")

	val := storage.Value{TypeTag: storage.TypeText, Data: []byte("v")}

	if res := interp.stageWriteQuietly([]string{"world", "agent", "1", "loud"}, val, 1, false); res != nil {
		t.Fatalf("staging loud write failed: %v", res)
	}
	if res := interp.stageWriteQuietly([]string{"world", "agent", "1", "quiet"}, val, 1, true); res != nil {
		t.Fatalf("staging quiet write failed: %v", res)
	}

	paths := interp.GetWrittenPaths()
	if len(paths) != 1 {
		t.Fatalf("GetWrittenPaths returned %d paths (%v), want only the loud one", len(paths), paths)
	}
	if last := paths[0][len(paths[0])-1]; last != "loud" {
		t.Errorf("GetWrittenPaths returned %q, want the loud write", last)
	}

	// Both writes must still commit — suppression is about notification only.
	if err := interp.flushBuffer(); err != nil {
		t.Fatalf("flush failed: %v", err)
	}
	for _, name := range []string{"loud", "quiet"} {
		if _, err := interp.scope.tree.Read([]string{"world", "agent", "1", name}); err != nil {
			t.Errorf("%s write did not commit: %v", name, err)
		}
	}
}

// TestPlainAssignmentIsNotQuiet guards the default: only (quietly) suppresses.
func TestPlainAssignmentIsNotQuiet(t *testing.T) {
	_, interp := freshTreeInterp(t, "q-default")

	val := storage.Value{TypeTag: storage.TypeText, Data: []byte("v")}
	if res := interp.stageWrite([]string{"world", "agent", "1", "x"}, val, 1); res != nil {
		t.Fatalf("staging failed: %v", res)
	}

	if got := len(interp.GetWrittenPaths()); got != 1 {
		t.Errorf("plain write produced %d trigger paths, want 1", got)
	}
}

// --- quietly: block form (DEVPLAN Step 29) ---

// TestQuietlyBlockSuppressesEveryWrite covers the block form. Per-assignment
// (quietly) gets noisy when a watcher body writes several paths.
func TestQuietlyBlockSuppressesEveryWrite(t *testing.T) {
	_, interp := freshTreeInterp(t, "quiet-block")

	runMBL(t, interp, "quietly:\n    my.a = 1\n    my.b = 2\n    my.c.nested = 3")

	if paths := interp.ChangedPaths(); len(paths) != 0 {
		t.Errorf("quiet block announced %v — nothing inside it should trigger watchers", paths)
	}

	// The writes must still have happened; suppression is about notification.
	if got := runMBL(t, interp, `my.a`); got != (types.Number{Value: 1}) {
		t.Errorf("my.a = %#v, want Number{1}", got)
	}
	if got := runMBL(t, interp, `my.b`); got != (types.Number{Value: 2}) {
		t.Errorf("my.b = %#v, want Number{2}", got)
	}
	if got := runMBL(t, interp, `my.c.nested`); got != (types.Number{Value: 3}) {
		t.Errorf("my.c.nested = %#v, want Number{3}", got)
	}
}

// TestQuietlyBlockDoesNotLeakIntermediates is the specific leak this had at
// first: a quiet write that auto-creates parent nodes was still announcing the
// parents, and with descendant matching a watcher on the parent would fire.
func TestQuietlyBlockDoesNotLeakIntermediates(t *testing.T) {
	_, interp := freshTreeInterp(t, "quiet-intermediates")

	// Nothing exists yet, so this creates every ancestor on the way down.
	runMBL(t, interp, "quietly:\n    my.deep.nested.value = 42")

	if paths := interp.ChangedPaths(); len(paths) != 0 {
		t.Errorf("quiet block announced %v — auto-created ancestors leaked", paths)
	}
}

// TestWritesAfterAQuietlyBlockAreLoudAgain guards the depth counter: a block
// that leaves the interpreter permanently quiet would silently stop every
// watcher in the process.
func TestWritesAfterAQuietlyBlockAreLoudAgain(t *testing.T) {
	_, interp := freshTreeInterp(t, "quiet-restore")

	runMBL(t, interp, "quietly:\n    my.a = 1")
	interp.ChangedPaths()

	runMBL(t, interp, `my.loud = 2`)
	if paths := interp.ChangedPaths(); len(paths) == 0 {
		t.Error("a write after the block announced nothing — quiet depth was not restored")
	}
}

// TestNestedQuietlyBlocksRestoreCorrectly covers the counter rather than a flag.
func TestNestedQuietlyBlocksRestoreCorrectly(t *testing.T) {
	_, interp := freshTreeInterp(t, "quiet-nested")

	runMBL(t, interp, "quietly:\n    my.a = 1\n    quietly:\n        my.b = 2\n    my.c = 3")
	if paths := interp.ChangedPaths(); len(paths) != 0 {
		t.Errorf("nested quiet blocks announced %v", paths)
	}

	runMBL(t, interp, `my.after = 9`)
	if paths := interp.ChangedPaths(); len(paths) == 0 {
		t.Error("nesting left the interpreter permanently quiet")
	}
}

// TestBareQuietlyStillParsesAsAModifier checks the block dispatch did not
// capture the per-assignment form, which shares the keyword.
func TestBareQuietlyStillParsesAsAModifier(t *testing.T) {
	_, interp := freshTreeInterp(t, "quiet-modifier")

	res := runMBL(t, interp, `my.x = (quietly) "still works"`)
	if got, ok := res.(types.Text); !ok || got.Value != "still works" {
		t.Errorf("per-assignment (quietly) = %#v, want Text{still works}", res)
	}
}
