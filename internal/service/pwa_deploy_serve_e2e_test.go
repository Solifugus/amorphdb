package service

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/solifugus/amorphdb/internal/mbl/interpreter"
	"github.com/solifugus/amorphdb/internal/mbl/lexer"
	"github.com/solifugus/amorphdb/internal/mbl/parser"
	"github.com/solifugus/amorphdb/internal/storage"
	"github.com/solifugus/amorphdb/internal/types"
)

// TestPWADeployServeEndToEnd is the full-chain guard for PWA publishing:
//
//	directory on disk
//	  --(real deploy_pwa MBL via the interpreter)-->
//	StorageTree (assets serialized with types.Record.Serialize)
//	  --(TreeAdapter + AssetCacheManager enumeration + parse)-->
//	HTTP GET response bytes
//
// This is the test that would have caught the serialization mismatch: the
// asset cache previously parsed a fictional JSON record format and silently
// dropped every asset that deploy_pwa() actually wrote. It exercises the real
// interpreter deploy path, the real serialized byte format, nested (slashed)
// asset paths, and the real HTTP handler — no mock storage values anywhere.
func TestPWADeployServeEndToEnd(t *testing.T) {
	const domain = "app.test"

	// 1. A small PWA on disk, including a nested-directory asset.
	srcDir := t.TempDir()
	files := map[string]string{
		"index.html":  "<!DOCTYPE html><html><body>home</body></html>",
		"css/app.css": "body{color:rebeccapurple}",
		"app.js":      "console.log('amorphdb pwa');",
	}
	for rel, content := range files {
		full := filepath.Join(srcDir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", full, err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", full, err)
		}
	}

	// 2. Real StorageTree shared by the deploying interpreter and the server.
	tree, err := storage.NewStorageTree(filepath.Join(t.TempDir(), "data"))
	if err != nil {
		t.Fatalf("NewStorageTree: %v", err)
	}

	// 3. Deploy via the real MBL path. Interpret() flushes the staged writes
	//    to the tree at the end, exactly as the daemon does on a heartbeat.
	interp := interpreter.New(tree, 1)
	src := `my.computer.network.web.deploy_pwa("` + domain + `", "` + srcDir + `")`
	prog := parser.New(lexer.New(src)).ParseProgram()
	if _, err := interp.Interpret(prog); err != nil {
		t.Fatalf("deploy_pwa interpret: %v", err)
	}

	// 4. Enable the domain. The spec's documented mechanism is
	//    `my.computer.network.web.pwa["app.test"].enabled = true`, but the
	//    bracket/quoted path-segment assignment syntax is not yet implemented
	//    (tracked 🚧). Until it is, write the enable flags directly to storage
	//    so the serve half can be exercised end-to-end.
	//    TODO(devplan): drive enable via MBL once bracketed path-segment
	//    assignment lands; see mbl_reference.md and amorphdb_design.md §PWA.
	base := []string{"my", "computer", "network", "web", "pwa", domain}
	boolTrue := storage.Value{TypeTag: types.TypeBoolean, Data: []byte{1}}
	if err := tree.Write(append(append([]string{}, base...), "enabled"), boolTrue, 1); err != nil {
		t.Fatalf("write enabled: %v", err)
	}
	if err := tree.Write(append(append([]string{}, base...), "spa_mode"), boolTrue, 1); err != nil {
		t.Fatalf("write spa_mode: %v", err)
	}

	// 5. Stand up the serving side over the SAME storage via TreeAdapter.
	adapter := storage.NewTreeAdapter(tree)
	watcher := NewMockWatcherEngine()
	assetCache := NewAssetCacheManager(adapter, watcher, 1)
	if err := assetCache.RefreshCache(domain); err != nil {
		t.Fatalf("RefreshCache: %v", err)
	}
	sse := NewSSEManager(adapter, watcher, 1)
	hs := NewHTTPServer("", "", nil, assetCache, sse, nil, adapter, 1)

	server := httptest.NewServer(http.HandlerFunc(hs.handleRequest))
	defer server.Close()

	// 6. Each deployed file must serve verbatim, including the nested one.
	for rel, want := range files {
		req, err := http.NewRequest(http.MethodGet, server.URL+"/"+rel, nil)
		if err != nil {
			t.Fatalf("new request %s: %v", rel, err)
		}
		req.Host = domain // route by Host, as a browser would
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("GET %s: %v", rel, err)
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("GET /%s status = %d, want 200 (body %q)", rel, resp.StatusCode, string(body))
			continue
		}
		if string(body) != want {
			t.Errorf("GET /%s body = %q, want %q", rel, string(body), want)
		}
	}
}
