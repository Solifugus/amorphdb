package service

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/solifugus/amorphdb/internal/storage"
	"github.com/solifugus/amorphdb/internal/types"
)

var ErrNotFound = errors.New("not found")

// MockWatcherEngine implements a minimal watcher engine for testing
type MockWatcherEngine struct {
	watchers map[string]bool
}

func NewMockWatcherEngine() *MockWatcherEngine {
	return &MockWatcherEngine{
		watchers: make(map[string]bool),
	}
}

func (mwe *MockWatcherEngine) RegisterWatcher(name string, watching []string, code string) error {
	mwe.watchers[name] = true
	return nil
}

// AssetMockTree implements a memory-based storage tree for testing
type AssetMockTree struct {
	data map[string]storage.Value
}

func NewAssetMockTree() *AssetMockTree {
	return &AssetMockTree{
		data: make(map[string]storage.Value),
	}
}

func (amt *AssetMockTree) pathKey(path []string) string {
	return strings.Join(path, ".")
}

func (amt *AssetMockTree) Read(path []string) (storage.Value, error) {
	key := amt.pathKey(path)
	if value, exists := amt.data[key]; exists {
		return value, nil
	}
	return storage.Value{}, ErrNotFound
}

func (amt *AssetMockTree) Write(path []string, value storage.Value, author uint64) error {
	key := amt.pathKey(path)
	amt.data[key] = value
	return nil
}

func (amt *AssetMockTree) ReadAt(path []string, timestamp int64) (storage.Value, error) {
	return amt.Read(path)
}

func (amt *AssetMockTree) Children(path []string) ([]storage.Attribute, error) {
	prefix := amt.pathKey(path) + "."
	var children []storage.Attribute
	childNames := make(map[string]bool)

	for key := range amt.data {
		if strings.HasPrefix(key, prefix) {
			// Extract immediate child name
			remaining := key[len(prefix):]
			parts := strings.Split(remaining, ".")
			if len(parts) > 0 {
				childName := parts[0]
				if !childNames[childName] {
					childNames[childName] = true
					children = append(children, storage.Attribute{
						ID:              uint64(len(children) + 1),
						LabelValueID:    uint64(len(children) + 1), // Points to child name
						FirstInstanceID: uint64(len(children) + 1), // Simplified for testing
						NextAttributeID: 0,
					})
				}
			}
		}
	}

	return children, nil
}

func (amt *AssetMockTree) Purge(path []string, from int64, to int64, author uint64) error {
	return nil
}

// ReadAllChildren reads all direct child values under the given path
func (amt *AssetMockTree) ReadAllChildren(path []string) (map[string]storage.Value, error) {
	prefix := amt.pathKey(path)
	if prefix != "" {
		prefix += "."
	}

	children := make(map[string]storage.Value)

	for key, value := range amt.data {
		if strings.HasPrefix(key, prefix) {
			// Extract immediate child name
			remaining := key[len(prefix):]
			parts := strings.Split(remaining, ".")
			if len(parts) > 0 {
				childName := parts[0]
				// Only include direct children (not nested grandchildren)
				if len(parts) == 1 {
					// This is a direct child
					children[childName] = value
				}
			}
		}
	}

	return children, nil
}

// ListLeafPaths enumerates stored leaf keys under the given prefix, mirroring
// storage.StorageTree.ListLeafPaths so asset loading exercises the same
// enumeration path it uses in production.
func (amt *AssetMockTree) ListLeafPaths(prefix []string) ([]string, error) {
	prefixStr := amt.pathKey(prefix) + "."
	seen := make(map[string]bool)
	var leaves []string

	for key := range amt.data {
		if !strings.HasPrefix(key, prefixStr) {
			continue
		}
		remainder := key[len(prefixStr):]
		if remainder == "" || seen[remainder] {
			continue
		}
		seen[remainder] = true
		leaves = append(leaves, remainder)
	}

	return leaves, nil
}

// GetAllKeys returns all keys in the tree (for debugging)
func (amt *AssetMockTree) GetAllKeys() []string {
	keys := make([]string, 0, len(amt.data))
	for key := range amt.data {
		keys = append(keys, key)
	}
	return keys
}

// Extended methods for ExtendedTree interface
func (amt *AssetMockTree) GetInstance(attrHash storage.Hash, agentID uint64) (storage.SecurityInstance, error) {
	return storage.SecurityInstance{}, nil
}

func (amt *AssetMockTree) GetValue(valueHash storage.Hash) ([]byte, error) {
	return nil, nil
}

func (amt *AssetMockTree) PutValue(valueHash storage.Hash, value []byte) error {
	return nil
}

func (amt *AssetMockTree) PutInstance(attrHash storage.Hash, instance storage.SecurityInstance) error {
	return nil
}

func (amt *AssetMockTree) ResolveAttributePath(attributeID uint64) (string, error) {
	// For testing purposes, return a simple mock path
	return fmt.Sprintf("test.attr_%d", attributeID), nil
}

// Helper method to set up test data
func (amt *AssetMockTree) SetupTestPWA(domain string, enabled bool, spaMode bool, assets map[string]*Asset) {
	// Set PWA configuration
	enabledPath := []string{"my", "computer", "network", "web", "pwa", domain, "enabled"}
	enabledValue := storage.Value{
		TypeTag: types.TypeBoolean,
		Data:    []byte{0},
	}
	if enabled {
		enabledValue.Data = []byte{1}
	}
	amt.Write(enabledPath, enabledValue, 1)

	spaPath := []string{"my", "computer", "network", "web", "pwa", domain, "spa_mode"}
	spaValue := storage.Value{
		TypeTag: types.TypeBoolean,
		Data:    []byte{0},
	}
	if spaMode {
		spaValue.Data = []byte{1}
	}
	amt.Write(spaPath, spaValue, 1)

	// Set assets
	for assetPath, asset := range assets {
		pathParts := strings.Split(assetPath, "/")
		fullPath := append([]string{"my", "computer", "network", "web", "pwa", domain, "assets"}, pathParts...)

		// Create a simplified record representation
		recordData := fmt.Sprintf(`{"data":"%s","mime_type":"%s"}`, string(asset.Data), asset.MimeType)
		assetValue := storage.Value{
			TypeTag: types.TypeRecord,
			Data:    []byte(recordData),
		}
		amt.Write(fullPath, assetValue, 1)
	}
}

// parseRecordFromValue extracts record data from storage value for testing
func parseRecordFromValue(value storage.Value) (*types.Record, error) {
	if value.TypeTag != types.TypeRecord {
		return nil, fmt.Errorf("not a record")
	}

	// Simple JSON parsing for test data
	recordStr := string(value.Data)

	// Extract data field
	dataStart := strings.Index(recordStr, `"data":"`)
	if dataStart == -1 {
		return nil, fmt.Errorf("no data field")
	}
	dataStart += 8 // Skip `"data":"`
	dataEnd := strings.Index(recordStr[dataStart:], `"`)
	if dataEnd == -1 {
		return nil, fmt.Errorf("malformed data field")
	}
	data := recordStr[dataStart : dataStart+dataEnd]

	// Extract mime_type field
	mimeStart := strings.Index(recordStr, `"mime_type":"`)
	if mimeStart == -1 {
		return nil, fmt.Errorf("no mime_type field")
	}
	mimeStart += 12 // Skip `"mime_type":"`
	mimeEnd := strings.Index(recordStr[mimeStart:], `"`)
	if mimeEnd == -1 {
		return nil, fmt.Errorf("malformed mime_type field")
	}
	mimeType := recordStr[mimeStart : mimeStart+mimeEnd]

	return &types.Record{
		Fields: map[string]interface{}{
			"data":      types.Text{Value: data},
			"mime_type": types.Text{Value: mimeType},
		},
	}, nil
}

// TestAssetCacheManager_LoadsArbitraryFilenames proves the enumeration-based
// loader serves assets whose names are NOT in the old hardcoded common-paths
// list (index.html/app.js/style.css/...). Before enumeration, files like
// dashboard.html or vendor.bundle.js were silently never loaded.
func TestAssetCacheManager_LoadsArbitraryFilenames(t *testing.T) {
	tree := NewAssetMockTree()
	watcherEngine := NewMockWatcherEngine()
	manager := NewAssetCacheManager(tree, watcherEngine, 1)

	domain := "app.example.com"

	// None of these filenames are in the legacy hardcoded common-paths list.
	testAssets := map[string]*Asset{
		"dashboard.html":   {Data: []byte("<html>dash</html>"), MimeType: "text/html", Path: "dashboard.html"},
		"vendor.bundle.js": {Data: []byte("/*vendor*/"), MimeType: "application/javascript", Path: "vendor.bundle.js"},
		"data.json":        {Data: []byte(`k=v`), MimeType: "application/json", Path: "data.json"},
		"logo.svg":         {Data: []byte("<svg/>"), MimeType: "image/svg+xml", Path: "logo.svg"},
	}

	tree.SetupTestPWA(domain, true, false, testAssets)
	if err := manager.RefreshCache(domain); err != nil {
		t.Fatalf("RefreshCache failed: %v", err)
	}

	for name, want := range testAssets {
		got := manager.GetAsset(domain, name)
		if got == nil {
			t.Errorf("expected arbitrary-named asset %q to load, got nil", name)
			continue
		}
		if string(got.Data) != string(want.Data) {
			t.Errorf("asset %q data = %q, want %q", name, got.Data, want.Data)
		}
		if got.MimeType != want.MimeType {
			t.Errorf("asset %q mime = %q, want %q", name, got.MimeType, want.MimeType)
		}
	}
}

// TestAssetCacheManager_DiscoversConfiguredDomains proves PWA domains are found
// by enumerating storage rather than matching a hardcoded list — including
// domains with dots in their names — and that asset files are not mistaken for
// domains. scanForPWADomains then loads each discovered domain into the cache.
func TestAssetCacheManager_DiscoversConfiguredDomains(t *testing.T) {
	tree := NewAssetMockTree()
	watcherEngine := NewMockWatcherEngine()
	manager := NewAssetCacheManager(tree, watcherEngine, 1)

	// Two real configured domains, neither in the legacy hardcoded scan list.
	configured := map[string]*Asset{
		"index.html": {Data: []byte("<html/>"), MimeType: "text/html", Path: "index.html"},
		// An asset literally named "enabled" must NOT be discovered as a domain.
		"enabled": {Data: []byte("x"), MimeType: "text/plain", Path: "enabled"},
	}
	tree.SetupTestPWA("shop.acme.io", true, true, configured)
	tree.SetupTestPWA("admin.acme.io", true, false, map[string]*Asset{
		"index.html": {Data: []byte("<html/>"), MimeType: "text/html", Path: "index.html"},
	})

	domains, ok := manager.discoverPWADomains()
	if !ok {
		t.Fatalf("discoverPWADomains returned ok=false; mock should support enumeration")
	}
	set := make(map[string]bool)
	for _, d := range domains {
		set[d] = true
	}
	if !set["shop.acme.io"] || !set["admin.acme.io"] {
		t.Errorf("expected both dotted domains discovered, got %v", domains)
	}
	for _, d := range domains {
		if strings.Contains(d, ".assets") || d == "shop.acme.io.assets" {
			t.Errorf("asset path leaked as a domain: %q", d)
		}
	}

	// scanForPWADomains should load the discovered domains into the cache.
	manager.scanForPWADomains()
	if enabled, spa := manager.IsEnabled("shop.acme.io"); !enabled || !spa {
		t.Errorf("shop.acme.io not loaded/enabled (enabled=%v spa=%v)", enabled, spa)
	}
	if enabled, _ := manager.IsEnabled("admin.acme.io"); !enabled {
		t.Errorf("admin.acme.io not loaded/enabled")
	}
}

func TestAssetCacheManager_BasicOperations(t *testing.T) {
	// Setup
	tree := NewAssetMockTree()
	watcherEngine := NewMockWatcherEngine()
	manager := NewAssetCacheManager(tree, watcherEngine, 1)

	domain := "app.example.com"

	// Test 1: PWA not enabled initially
	enabled, spaMode := manager.IsEnabled(domain)
	if enabled || spaMode {
		t.Errorf("Expected PWA to be disabled initially, got enabled=%v, spaMode=%v", enabled, spaMode)
	}

	// Test 2: Asset lookup when PWA not enabled
	asset := manager.GetAsset(domain, "index.html")
	if asset != nil {
		t.Errorf("Expected no asset when PWA disabled, got %v", asset)
	}

	// Test 3: Setup PWA with assets
	testAssets := map[string]*Asset{
		"index.html": {
			Data:     []byte("<html><body>Hello World</body></html>"),
			MimeType: "text/html",
			Path:     "index.html",
		},
		"style.css": {
			Data:     []byte("body { color: blue; }"),
			MimeType: "text/css",
			Path:     "style.css",
		},
		"app.js": {
			Data:     []byte("console.log('Hello');"),
			MimeType: "application/javascript",
			Path:     "app.js",
		},
	}

	tree.SetupTestPWA(domain, true, true, testAssets)

	// Test 4: Refresh cache and verify PWA is enabled
	err := manager.RefreshCache(domain)
	if err != nil {
		t.Fatalf("Failed to refresh cache: %v", err)
	}

	enabled, spaMode = manager.IsEnabled(domain)
	if !enabled || !spaMode {
		t.Errorf("Expected PWA to be enabled with SPA mode, got enabled=%v, spaMode=%v", enabled, spaMode)
	}

	// Test 5: Verify assets are loaded (O(1) lookup test)
	for expectedPath, expectedAsset := range testAssets {
		asset := manager.GetAsset(domain, expectedPath)
		if asset == nil {
			t.Errorf("Expected to find asset at path %s, got nil", expectedPath)
			continue
		}

		if asset.Path != expectedAsset.Path {
			t.Errorf("Expected asset path %s, got %s", expectedAsset.Path, asset.Path)
		}

		if asset.MimeType != expectedAsset.MimeType {
			t.Errorf("Expected MIME type %s, got %s", expectedAsset.MimeType, asset.MimeType)
		}

		if string(asset.Data) != string(expectedAsset.Data) {
			t.Errorf("Expected asset data %s, got %s", string(expectedAsset.Data), string(asset.Data))
		}

		// Verify load time is recent
		if time.Since(asset.LoadTime) > time.Minute {
			t.Errorf("Expected recent load time, got %v", asset.LoadTime)
		}
	}
}

func TestAssetCacheManager_AssetUpdate(t *testing.T) {
	// Setup
	tree := NewAssetMockTree()
	watcherEngine := NewMockWatcherEngine()
	manager := NewAssetCacheManager(tree, watcherEngine, 1)

	domain := "app.example.com"

	// Setup initial assets
	initialAssets := map[string]*Asset{
		"index.html": {
			Data:     []byte("<html><body>Version 1</body></html>"),
			MimeType: "text/html",
			Path:     "index.html",
		},
	}

	tree.SetupTestPWA(domain, true, false, initialAssets)
	err := manager.RefreshCache(domain)
	if err != nil {
		t.Fatalf("Failed to refresh cache: %v", err)
	}

	// Verify initial asset
	asset := manager.GetAsset(domain, "index.html")
	if asset == nil {
		t.Fatal("Expected to find initial asset")
	}

	if string(asset.Data) != "<html><body>Version 1</body></html>" {
		t.Errorf("Expected initial content, got %s", string(asset.Data))
	}

	initialLoadTime := asset.LoadTime

	// Wait a moment to ensure different timestamps
	time.Sleep(10 * time.Millisecond)

	// Update the asset in storage
	updatedAssets := map[string]*Asset{
		"index.html": {
			Data:     []byte("<html><body>Version 2</body></html>"),
			MimeType: "text/html",
			Path:     "index.html",
		},
	}

	tree.SetupTestPWA(domain, true, false, updatedAssets)

	// Refresh cache to simulate watcher trigger
	err = manager.RefreshCache(domain)
	if err != nil {
		t.Fatalf("Failed to refresh cache after update: %v", err)
	}

	// Verify asset was updated
	updatedAsset := manager.GetAsset(domain, "index.html")
	if updatedAsset == nil {
		t.Fatal("Expected to find updated asset")
	}

	if string(updatedAsset.Data) != "<html><body>Version 2</body></html>" {
		t.Errorf("Expected updated content, got %s", string(updatedAsset.Data))
	}

	// Verify load time was updated (cache refresh happened)
	if !updatedAsset.LoadTime.After(initialLoadTime) {
		t.Errorf("Expected updated load time %v to be after initial time %v", updatedAsset.LoadTime, initialLoadTime)
	}
}

func TestAssetCacheManager_DisablePWA(t *testing.T) {
	// Setup
	tree := NewAssetMockTree()
	watcherEngine := NewMockWatcherEngine()
	manager := NewAssetCacheManager(tree, watcherEngine, 1)

	domain := "app.example.com"

	// Setup PWA with assets
	testAssets := map[string]*Asset{
		"index.html": {
			Data:     []byte("<html><body>Test</body></html>"),
			MimeType: "text/html",
			Path:     "index.html",
		},
	}

	tree.SetupTestPWA(domain, true, false, testAssets)
	err := manager.RefreshCache(domain)
	if err != nil {
		t.Fatalf("Failed to refresh cache: %v", err)
	}

	// Verify asset is available
	asset := manager.GetAsset(domain, "index.html")
	if asset == nil {
		t.Fatal("Expected to find asset when PWA enabled")
	}

	// Disable PWA
	tree.SetupTestPWA(domain, false, false, testAssets)
	err = manager.RefreshCache(domain)
	if err != nil {
		t.Fatalf("Failed to refresh cache after disable: %v", err)
	}

	// Verify PWA is disabled
	enabled, _ := manager.IsEnabled(domain)
	if enabled {
		t.Error("Expected PWA to be disabled")
	}

	// Verify asset is no longer accessible
	asset = manager.GetAsset(domain, "index.html")
	if asset != nil {
		t.Error("Expected no asset when PWA disabled")
	}
}

func BenchmarkAssetCacheManager_GetAsset(b *testing.B) {
	// Setup
	tree := NewAssetMockTree()
	watcherEngine := NewMockWatcherEngine()
	manager := NewAssetCacheManager(tree, watcherEngine, 1)

	domain := "app.example.com"

	// Setup many assets to test O(1) lookup performance
	testAssets := make(map[string]*Asset)
	for i := 0; i < 1000; i++ {
		path := fmt.Sprintf("file%d.js", i)
		testAssets[path] = &Asset{
			Data:     []byte(fmt.Sprintf("console.log('File %d');", i)),
			MimeType: "application/javascript",
			Path:     path,
		}
	}

	tree.SetupTestPWA(domain, true, false, testAssets)
	err := manager.RefreshCache(domain)
	if err != nil {
		b.Fatalf("Failed to refresh cache: %v", err)
	}

	// Benchmark O(1) lookups
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		path := fmt.Sprintf("file%d.js", i%1000)
		asset := manager.GetAsset(domain, path)
		if asset == nil {
			b.Errorf("Expected to find asset at path %s", path)
		}
	}
}
