// Tests for bracket key-selectors in READ expressions: path[key] descends into
// the child named by the key's value, e.g.
//
//	creds = world.apps.myapp.auth.credentials[username]
//	host  = world.config.hosts["alpha"]
//
// A key-selector read resolves to exactly the same path a direct dotted read
// would, so it round-trips with the assignment side. It must not disturb the
// existing list-index and attribute-filter semantics of brackets.
package interpreter

import (
	"path/filepath"
	"testing"

	"github.com/solifugus/amorphdb/internal/mbl/lexer"
	"github.com/solifugus/amorphdb/internal/mbl/parser"
	"github.com/solifugus/amorphdb/internal/storage"
	"github.com/solifugus/amorphdb/internal/types"
)

func readInterp(t *testing.T, name string) *Interpreter {
	t.Helper()
	tree, err := storage.NewStorageTree(filepath.Join(t.TempDir(), name))
	if err != nil {
		t.Fatalf("NewStorageTree: %v", err)
	}
	return New(tree, 1)
}

func interpret(t *testing.T, in *Interpreter, src string) interface{} {
	t.Helper()
	p := parser.New(lexer.New(src))
	prog := p.ParseProgram()
	if errs := p.Errors(); len(errs) != 0 {
		t.Fatalf("parse errors for %q: %v", src, errs)
	}
	res, err := in.Interpret(prog)
	if err != nil {
		t.Fatalf("interpret %q: %v", src, err)
	}
	return res
}

func TestReadKeySelector_DynamicKeyLeaf(t *testing.T) {
	in := readInterp(t, "dyn")
	interpret(t, in, `world.config.hosts.alpha = "10.0.0.1"`)
	got := interpret(t, in, "k = \"alpha\"\nworld.config.hosts[k]")
	text, ok := got.(types.Text)
	if !ok || text.Value != "10.0.0.1" {
		t.Fatalf("world.config.hosts[k] = %v (%T), want Text 10.0.0.1", got, got)
	}
}

func TestReadKeySelector_StaticStringKeyLeaf(t *testing.T) {
	in := readInterp(t, "static")
	interpret(t, in, `world.config.hosts.alpha = "10.0.0.1"`)
	got := interpret(t, in, `world.config.hosts["alpha"]`)
	text, ok := got.(types.Text)
	if !ok || text.Value != "10.0.0.1" {
		t.Fatalf(`world.config.hosts["alpha"] = %v (%T), want Text 10.0.0.1`, got, got)
	}
}

func TestReadKeySelector_LeafMatchesDirectRead(t *testing.T) {
	// A terminal key-selector read of a leaf resolves to the same value as the
	// equivalent dotted read of that leaf.
	in := readInterp(t, "match")
	interpret(t, in, `world.store.widget = 42`)

	direct := interpret(t, in, `world.store.widget`)
	viaKey := interpret(t, in, "item = \"widget\"\nworld.store[item]")

	dnum, dok := direct.(types.Number)
	knum, kok := viaKey.(types.Number)
	if !dok || !kok || dnum.Value != 42 || knum.Value != dnum.Value {
		t.Fatalf("direct=%v keyselector=%v, want both Number 42", direct, viaKey)
	}
}

func TestReadKeySelector_ListIndexStillWorks(t *testing.T) {
	// An in-memory list variable must still index by position, not be treated
	// as a key-selector path descent.
	in := readInterp(t, "list")
	got := interpret(t, in, "xs = [\"apple\", \"banana\", \"cherry\"]\nxs[1]")
	text, ok := got.(types.Text)
	if !ok || text.Value != "banana" {
		t.Fatalf("xs[1] = %v (%T), want Text banana", got, got)
	}
}

func TestReadKeySelector_ComparisonFilterStillWorks(t *testing.T) {
	// A comparison inside the brackets is a filter, never a key-selector: it
	// must still return a Record (the existing filter behavior), not attempt a
	// path descent into a child named after the condition.
	in := readInterp(t, "filter")
	interpret(t, in, "world.staff.a.age = 25\nworld.staff.b.age = 40")
	got := interpret(t, in, `world.staff[age > 30]`)
	if _, ok := got.(types.Record); !ok {
		t.Fatalf("filter read = %v (%T), want Record", got, got)
	}
}

func TestReadKeySelector_UnknownKeyPropagates(t *testing.T) {
	in := readInterp(t, "unk")
	got := interpret(t, in, `world.config.hosts[missing]`)
	if _, ok := got.(types.Unknown); !ok {
		t.Fatalf("read with unresolved key = %v (%T), want Unknown", got, got)
	}
}
