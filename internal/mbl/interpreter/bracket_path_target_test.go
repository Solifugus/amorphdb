// End-to-end tests that bracket key-selector assignment targets write to the
// correct storage paths, including the my.computer node-local routing the PWA
// asset cache depends on.
package interpreter

import (
	"path/filepath"
	"testing"

	"github.com/solifugus/amorphdb/internal/mbl/lexer"
	"github.com/solifugus/amorphdb/internal/mbl/parser"
	"github.com/solifugus/amorphdb/internal/storage"
	"github.com/solifugus/amorphdb/internal/types"
)

func runMBL(t *testing.T, interp *Interpreter, src string) interface{} {
	t.Helper()
	p := parser.New(lexer.New(src))
	prog := p.ParseProgram()
	if errs := p.Errors(); len(errs) != 0 {
		t.Fatalf("parse errors for %q: %v", src, errs)
	}
	res, err := interp.Interpret(prog)
	if err != nil {
		t.Fatalf("interpret %q: %v", src, err)
	}
	return res
}

func freshTreeInterp(t *testing.T, name string) (*storage.StorageTree, *Interpreter) {
	t.Helper()
	tree, err := storage.NewStorageTree(filepath.Join(t.TempDir(), name))
	if err != nil {
		t.Fatalf("NewStorageTree: %v", err)
	}
	return tree, New(tree, 1)
}

// TestBracketAssign_PWAEnableLandsNodeLocal is the headline case: enabling a PWA
// domain from MBL must write to the literal my.computer.network.web.pwa.<domain>
// subtree (a node-local virtual mount), exactly where the asset cache reads —
// not the per-agent home it would land in without the routing fix.
func TestBracketAssign_PWAEnableLandsNodeLocal(t *testing.T) {
	tree, interp := freshTreeInterp(t, "pwa")
	runMBL(t, interp, `my.computer.network.web.pwa["app.example.com"].enabled = true`)
	runMBL(t, interp, `my.computer.network.web.pwa["app.example.com"].spa_mode = true`)

	for _, leaf := range []string{"enabled", "spa_mode"} {
		path := []string{"my", "computer", "network", "web", "pwa", "app.example.com", leaf}
		v, err := tree.Read(path)
		if err != nil {
			t.Fatalf("read %v: %v", path, err)
		}
		if v.TypeTag != types.TypeBoolean || len(v.Data) == 0 || v.Data[0] != 1 {
			t.Fatalf("%s = %+v, want Boolean true", leaf, v)
		}
	}
}

// TestBracketAssign_NestedAssetKey writes a nested asset via a static bracket
// key; the slash must be preserved inside a single component, matching what
// deploy_pwa() stores.
func TestBracketAssign_NestedAssetKey(t *testing.T) {
	tree, interp := freshTreeInterp(t, "asset")
	runMBL(t, interp, `my.computer.network.web.pwa["app.example.com"].assets["css/app.css"] = "body{color:red}"`)

	path := []string{"my", "computer", "network", "web", "pwa", "app.example.com", "assets", "css/app.css"}
	v, err := tree.Read(path)
	if err != nil {
		t.Fatalf("read %v: %v", path, err)
	}
	if v.TypeTag != types.TypeText {
		t.Fatalf("asset typetag = %d, want Text", v.TypeTag)
	}
}

// TestBracketAssign_DynamicKeyFromVariable covers the dominant auth form:
// tokens[token].identity = username, where token/username are runtime values.
func TestBracketAssign_DynamicKeyFromVariable(t *testing.T) {
	tree, interp := freshTreeInterp(t, "tokens")
	runMBL(t, interp, "token = \"abc123\"\nusername = \"kalevo\"\nworld.apps.myapp.tokens[token].identity = username")

	path := []string{"world", "apps", "myapp", "tokens", "abc123", "identity"}
	v, err := tree.Read(path)
	if err != nil {
		t.Fatalf("read %v: %v", path, err)
	}
	mbl, err := storageToMBL(v)
	if err != nil {
		t.Fatalf("storageToMBL: %v", err)
	}
	text, ok := mbl.(types.Text)
	if !ok || text.Value != "kalevo" {
		t.Fatalf("identity = %v (%T), want Text kalevo", mbl, mbl)
	}
}

// TestBracketAssign_LoopIndexKey covers data[i] = ... with a numeric loop
// variable as the key (commit-buffer / cross-zone style scripts).
func TestBracketAssign_LoopIndexKey(t *testing.T) {
	tree, interp := freshTreeInterp(t, "loop")
	runMBL(t, interp, "for i in [0, 1, 2]:\n\tworld.zone.data[i] = i")

	for _, k := range []string{"0", "1", "2"} {
		path := []string{"world", "zone", "data", k}
		if _, err := tree.Read(path); err != nil {
			t.Fatalf("expected zone.data.%s written, read err: %v", k, err)
		}
	}
}

// TestBracketAssign_DynamicKeyUnknownPropagates is the primary failure case: a
// key that evaluates to Unknown must surface as Unknown rather than writing to
// a garbage path.
func TestBracketAssign_DynamicKeyUnknownPropagates(t *testing.T) {
	_, interp := freshTreeInterp(t, "unk")
	// `missing` is never assigned, so it reads back Unknown and the key cannot
	// resolve.
	res := runMBL(t, interp, `world.apps.myapp.tokens[missing].identity = "x"`)
	if _, ok := res.(types.Unknown); !ok {
		t.Fatalf("expected Unknown from unresolved key, got %v (%T)", res, res)
	}
}

// TestComputerRouting_DotlessStaysNodeLocal proves the routing fix independent
// of brackets: a plain my.computer.* assignment stays at its literal location
// instead of being rewritten to world.agent.<id>.*.
func TestComputerRouting_DotlessStaysNodeLocal(t *testing.T) {
	tree, interp := freshTreeInterp(t, "route")
	runMBL(t, interp, `my.computer.network.web.pwa.foo.enabled = true`)

	if _, err := tree.Read([]string{"my", "computer", "network", "web", "pwa", "foo", "enabled"}); err != nil {
		t.Fatalf("expected node-local my.computer path, read err: %v", err)
	}
	if _, err := tree.Read([]string{"world", "agent", "1", "computer", "network", "web", "pwa", "foo", "enabled"}); err == nil {
		t.Fatalf("my.computer.* must not be rewritten to the per-agent home")
	}
}

// TestComputerRouting_NonComputerMyStillResolves guards the routing change: an
// ordinary my.* data path is still rewritten to the per-agent home.
func TestComputerRouting_NonComputerMyStillResolves(t *testing.T) {
	tree, interp := freshTreeInterp(t, "myhome")
	runMBL(t, interp, `my.profile.name = "kalevo"`)

	if _, err := tree.Read([]string{"world", "agent", "1", "profile", "name"}); err != nil {
		t.Fatalf("expected my.profile.name at world.agent.1.profile.name, read err: %v", err)
	}
}
