package service

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/solifugus/amorphdb/internal/config"
	"github.com/solifugus/amorphdb/internal/storage"
	"github.com/solifugus/amorphdb/internal/types"
	"github.com/solifugus/amorphdb/web/boilerplate"
)

// TestHTTPServer_ServePWABoilerplate verifies the embedded client framework is
// served at /amorphdb/pwa.js, ahead of any per-domain asset/SPA handling, so an
// app can load it with a single <script src="/amorphdb/pwa.js"> tag.
func TestHTTPServer_ServePWABoilerplate(t *testing.T) {
	tree := NewAssetMockTree()
	watcherEngine := NewMockWatcherEngine()
	assetCache := NewAssetCacheManager(tree, watcherEngine, 1)
	sseManager := NewSSEManager(tree, watcherEngine, 1)
	pwaBridge := NewPWABridge(tree, sseManager, watcherEngine, 1, "testapp")
	httpServer := NewHTTPServer(":8080", "", nil, assetCache, sseManager, pwaBridge, tree, 1)

	req := httptest.NewRequest("GET", "/amorphdb/pwa.js", nil)
	req.Host = "app.example.com"
	w := httptest.NewRecorder()
	httpServer.handleRequest(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	if ct := w.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/javascript") {
		t.Errorf("expected JavaScript Content-Type, got %q", ct)
	}

	body := w.Body.Bytes()
	if len(body) == 0 {
		t.Fatalf("served boilerplate is empty")
	}
	if !bytes.Equal(body, boilerplate.PWAJavaScript) {
		t.Errorf("served body (%d bytes) does not match the embedded boilerplate (%d bytes)", len(body), len(boilerplate.PWAJavaScript))
	}
	if !bytes.Contains(body, []byte("AmorphDB PWA Boilerplate")) {
		t.Errorf("served boilerplate is missing its expected header banner")
	}
}

// TestHTTPServer_ServesNestedAsset proves assets in subdirectories are served at
// their nested URL. Under the Option A convention (matching production
// deploy_pwa), a file like css/app.css is stored as a SINGLE path component
// named "css/app.css"; enumeration returns that key intact and the server
// serves it at GET /css/app.css.
func TestHTTPServer_ServesNestedAsset(t *testing.T) {
	tree := NewAssetMockTree()
	watcherEngine := NewMockWatcherEngine()
	assetCache := NewAssetCacheManager(tree, watcherEngine, 1)

	domain := "app.example.com"
	tree.SetupTestPWA(domain, true, false, map[string]*Asset{
		"index.html":        {Data: []byte("<html/>"), MimeType: "text/html", Path: "index.html"},
		"css/app.css":       {Data: []byte("body{color:red}"), MimeType: "text/css", Path: "css/app.css"},
		"js/main.bundle.js": {Data: []byte("//bundle"), MimeType: "application/javascript", Path: "js/main.bundle.js"},
	})
	if err := assetCache.RefreshCache(domain); err != nil {
		t.Fatalf("RefreshCache failed: %v", err)
	}

	sseManager := NewSSEManager(tree, watcherEngine, 1)
	pwaBridge := NewPWABridge(tree, sseManager, watcherEngine, 1, "testapp")
	httpServer := NewHTTPServer(":8080", "", nil, assetCache, sseManager, pwaBridge, tree, 1)

	cases := []struct{ url, wantCT, wantBody string }{
		{"/css/app.css", "text/css", "body{color:red}"},
		{"/js/main.bundle.js", "application/javascript", "//bundle"},
	}
	for _, tc := range cases {
		req := httptest.NewRequest("GET", tc.url, nil)
		req.Host = domain
		w := httptest.NewRecorder()
		httpServer.handleRequest(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("GET %s: status = %d, want 200", tc.url, w.Code)
			continue
		}
		if got := w.Body.String(); got != tc.wantBody {
			t.Errorf("GET %s: body = %q, want %q", tc.url, got, tc.wantBody)
		}
		if ct := w.Header().Get("Content-Type"); ct != tc.wantCT {
			t.Errorf("GET %s: Content-Type = %q, want %q", tc.url, ct, tc.wantCT)
		}
	}
}

func TestHTTPServer_ServeStaticAsset(t *testing.T) {
	// Setup
	tree := NewAssetMockTree()
	watcherEngine := NewMockWatcherEngine()
	assetCache := NewAssetCacheManager(tree, watcherEngine, 1)

	domain := "app.example.com"

	// Setup PWA with test assets
	testAssets := map[string]*Asset{
		"index.html": {
			Data:     []byte("<html><head><title>Test App</title></head><body><h1>Hello World</h1></body></html>"),
			MimeType: "text/html",
			Path:     "index.html",
		},
		"style.css": {
			Data:     []byte("body { font-family: Arial, sans-serif; color: #333; }"),
			MimeType: "text/css",
			Path:     "style.css",
		},
		"app.js": {
			Data:     []byte("console.log('Application loaded');"),
			MimeType: "application/javascript",
			Path:     "app.js",
		},
	}

	tree.SetupTestPWA(domain, true, false, testAssets) // SPA mode disabled for this test
	err := assetCache.RefreshCache(domain)
	if err != nil {
		t.Fatalf("Failed to refresh cache: %v", err)
	}

	// Create SSE manager, PWA bridge and HTTP server
	sseManager := NewSSEManager(tree, watcherEngine, 1)
	pwaBridge := NewPWABridge(tree, sseManager, watcherEngine, 1, "testapp")
	httpServer := NewHTTPServer(":8080", "", nil, assetCache, sseManager, pwaBridge, tree, 1)

	// Test 1: Serve index.html and verify Content-Type
	req := httptest.NewRequest("GET", "/", nil)
	req.Host = domain
	w := httptest.NewRecorder()
	httpServer.handleRequest(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}

	expectedContentType := "text/html"
	contentType := w.Header().Get("Content-Type")
	if contentType != expectedContentType {
		t.Errorf("Expected Content-Type %s, got %s", expectedContentType, contentType)
	}

	expectedBody := string(testAssets["index.html"].Data)
	if w.Body.String() != expectedBody {
		t.Errorf("Expected body %s, got %s", expectedBody, w.Body.String())
	}

	// Test 2: Serve CSS file and verify Content-Type
	req = httptest.NewRequest("GET", "/style.css", nil)
	req.Host = domain
	w = httptest.NewRecorder()
	httpServer.handleRequest(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d for CSS, got %d", http.StatusOK, w.Code)
	}

	expectedContentType = "text/css"
	contentType = w.Header().Get("Content-Type")
	if contentType != expectedContentType {
		t.Errorf("Expected Content-Type %s for CSS, got %s", expectedContentType, contentType)
	}

	// Test 3: Serve JavaScript file and verify Content-Type
	req = httptest.NewRequest("GET", "/app.js", nil)
	req.Host = domain
	w = httptest.NewRecorder()
	httpServer.handleRequest(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d for JS, got %d", http.StatusOK, w.Code)
	}

	expectedContentType = "application/javascript"
	contentType = w.Header().Get("Content-Type")
	if contentType != expectedContentType {
		t.Errorf("Expected Content-Type %s for JS, got %s", expectedContentType, contentType)
	}

	// Test 4: Verify cache headers are set
	cacheControl := w.Header().Get("Cache-Control")
	if !strings.Contains(cacheControl, "public") {
		t.Errorf("Expected cache control to be public, got %s", cacheControl)
	}

	lastModified := w.Header().Get("Last-Modified")
	if lastModified == "" {
		t.Error("Expected Last-Modified header to be set")
	}
}

func TestHTTPServer_SPAFallbackEnabled(t *testing.T) {
	// Setup
	tree := NewAssetMockTree()
	watcherEngine := NewMockWatcherEngine()
	assetCache := NewAssetCacheManager(tree, watcherEngine, 1)

	domain := "spa.example.com"

	// Setup PWA with SPA mode enabled
	testAssets := map[string]*Asset{
		"index.html": {
			Data:     []byte("<html><head><title>SPA App</title></head><body><div id='root'></div></body></html>"),
			MimeType: "text/html",
			Path:     "index.html",
		},
		"app.js": {
			Data:     []byte("// SPA application bundle"),
			MimeType: "application/javascript",
			Path:     "app.js",
		},
	}

	tree.SetupTestPWA(domain, true, true, testAssets) // SPA mode enabled
	err := assetCache.RefreshCache(domain)
	if err != nil {
		t.Fatalf("Failed to refresh cache: %v", err)
	}

	// Create SSE manager, PWA bridge and HTTP server
	sseManager := NewSSEManager(tree, watcherEngine, 1)
	pwaBridge := NewPWABridge(tree, sseManager, watcherEngine, 1, "testapp")
	httpServer := NewHTTPServer(":8080", "", nil, assetCache, sseManager, pwaBridge, tree, 1)

	// Test 1: Request existing asset - should serve normally
	req := httptest.NewRequest("GET", "/app.js", nil)
	req.Host = domain
	w := httptest.NewRecorder()
	httpServer.handleRequest(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d for existing asset, got %d", http.StatusOK, w.Code)
	}

	expectedBody := string(testAssets["app.js"].Data)
	if w.Body.String() != expectedBody {
		t.Errorf("Expected body %s, got %s", expectedBody, w.Body.String())
	}

	// Test 2: Request non-existent path with SPA mode on - should return index.html
	req = httptest.NewRequest("GET", "/user/profile", nil)
	req.Host = domain
	w = httptest.NewRecorder()
	httpServer.handleRequest(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d for SPA fallback, got %d", http.StatusOK, w.Code)
	}

	expectedContentType := "text/html"
	contentType := w.Header().Get("Content-Type")
	if contentType != expectedContentType {
		t.Errorf("Expected Content-Type %s for SPA fallback, got %s", expectedContentType, contentType)
	}

	expectedBody = string(testAssets["index.html"].Data)
	if w.Body.String() != expectedBody {
		t.Errorf("Expected index.html content for SPA fallback, got %s", w.Body.String())
	}

	// Test 3: Request another non-existent path - should also return index.html
	req = httptest.NewRequest("GET", "/admin/dashboard", nil)
	req.Host = domain
	w = httptest.NewRecorder()
	httpServer.handleRequest(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d for SPA fallback, got %d", http.StatusOK, w.Code)
	}

	if w.Body.String() != expectedBody {
		t.Errorf("Expected index.html content for SPA fallback, got %s", w.Body.String())
	}
}

func TestHTTPServer_SPAFallbackDisabled(t *testing.T) {
	// Setup
	tree := NewAssetMockTree()
	watcherEngine := NewMockWatcherEngine()
	assetCache := NewAssetCacheManager(tree, watcherEngine, 1)

	domain := "api.example.com"

	// Setup PWA with SPA mode disabled
	testAssets := map[string]*Asset{
		"index.html": {
			Data:     []byte("<html><body>Static Site</body></html>"),
			MimeType: "text/html",
			Path:     "index.html",
		},
	}

	tree.SetupTestPWA(domain, true, false, testAssets) // SPA mode disabled
	err := assetCache.RefreshCache(domain)
	if err != nil {
		t.Fatalf("Failed to refresh cache: %v", err)
	}

	// Create SSE manager, PWA bridge and HTTP server
	sseManager := NewSSEManager(tree, watcherEngine, 1)
	pwaBridge := NewPWABridge(tree, sseManager, watcherEngine, 1, "testapp")
	httpServer := NewHTTPServer(":8080", "", nil, assetCache, sseManager, pwaBridge, tree, 1)

	// Test 1: Request existing asset - should serve normally
	req := httptest.NewRequest("GET", "/", nil)
	req.Host = domain
	w := httptest.NewRecorder()
	httpServer.handleRequest(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d for existing asset, got %d", http.StatusOK, w.Code)
	}

	// Test 2: Request non-existent path with SPA mode off - should return 503
	req = httptest.NewRequest("GET", "/nonexistent", nil)
	req.Host = domain
	w = httptest.NewRecorder()
	httpServer.handleRequest(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status %d for non-existent path with SPA off, got %d",
			http.StatusServiceUnavailable, w.Code)
	}

	expectedBody := "Service Unavailable\n"
	if w.Body.String() != expectedBody {
		t.Errorf("Expected body %s, got %s", expectedBody, w.Body.String())
	}

	// Test 3: Request another non-existent path - should also return 503
	req = httptest.NewRequest("GET", "/api/users", nil)
	req.Host = domain
	w = httptest.NewRecorder()
	httpServer.handleRequest(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status %d for API path with SPA off, got %d",
			http.StatusServiceUnavailable, w.Code)
	}
}

func TestHTTPServer_PWANotEnabled(t *testing.T) {
	// Setup
	tree := NewAssetMockTree()
	watcherEngine := NewMockWatcherEngine()
	assetCache := NewAssetCacheManager(tree, watcherEngine, 1)

	domain := "disabled.example.com"

	// Setup PWA but keep it disabled
	testAssets := map[string]*Asset{
		"index.html": {
			Data:     []byte("<html><body>Disabled</body></html>"),
			MimeType: "text/html",
			Path:     "index.html",
		},
	}

	tree.SetupTestPWA(domain, false, false, testAssets) // PWA disabled
	err := assetCache.RefreshCache(domain)
	if err != nil {
		t.Fatalf("Failed to refresh cache: %v", err)
	}

	// Create SSE manager, PWA bridge and HTTP server
	sseManager := NewSSEManager(tree, watcherEngine, 1)
	pwaBridge := NewPWABridge(tree, sseManager, watcherEngine, 1, "testapp")
	httpServer := NewHTTPServer(":8080", "", nil, assetCache, sseManager, pwaBridge, tree, 1)

	// Test 1: Request any path when PWA is disabled - should return 503
	req := httptest.NewRequest("GET", "/", nil)
	req.Host = domain
	w := httptest.NewRecorder()
	httpServer.handleRequest(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status %d for disabled PWA, got %d",
			http.StatusServiceUnavailable, w.Code)
	}

	// Test 2: Request specific asset when PWA is disabled - should return 503
	req = httptest.NewRequest("GET", "/index.html", nil)
	req.Host = domain
	w = httptest.NewRecorder()
	httpServer.handleRequest(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status %d for disabled PWA asset request, got %d",
			http.StatusServiceUnavailable, w.Code)
	}
}

func TestHTTPServer_InvalidHost(t *testing.T) {
	// Setup
	tree := NewAssetMockTree()
	watcherEngine := NewMockWatcherEngine()
	assetCache := NewAssetCacheManager(tree, watcherEngine, 1)

	// Create SSE manager, PWA bridge and HTTP server
	sseManager := NewSSEManager(tree, watcherEngine, 1)
	pwaBridge := NewPWABridge(tree, sseManager, watcherEngine, 1, "testapp")
	httpServer := NewHTTPServer(":8080", "", nil, assetCache, sseManager, pwaBridge, tree, 1)

	// Test 1: Request with empty host
	req := httptest.NewRequest("GET", "/", nil)
	req.Host = ""
	w := httptest.NewRecorder()
	httpServer.handleRequest(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d for empty host, got %d", http.StatusBadRequest, w.Code)
	}

	// Test 2: Request with invalid host format
	req = httptest.NewRequest("GET", "/", nil)
	req.Host = "invalid..domain"
	w = httptest.NewRecorder()
	httpServer.handleRequest(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d for invalid host, got %d", http.StatusBadRequest, w.Code)
	}
}

func TestExtractDomain(t *testing.T) {
	tests := []struct {
		host     string
		expected string
	}{
		{"example.com", "example.com"},
		{"example.com:8080", "example.com"},
		{"localhost", "localhost"},
		{"localhost:3000", "localhost"},
		{"127.0.0.1", "127.0.0.1"},
		{"127.0.0.1:8080", "127.0.0.1"},
		{"app.example.com", "app.example.com"},
		{"app.example.com:443", "app.example.com"},
		{"", ""},
		{"invalid", ""},
	}

	for _, test := range tests {
		result := extractDomain(test.host)
		if result != test.expected {
			t.Errorf("extractDomain(%s) = %s, expected %s", test.host, result, test.expected)
		}
	}
}

func TestHTTPServer_MCPRequestRouting(t *testing.T) {
	// Setup
	tree := NewAssetMockTree()
	watcherEngine := NewMockWatcherEngine()
	assetCache := NewAssetCacheManager(tree, watcherEngine, 1)
	sseManager := NewSSEManager(tree, watcherEngine, 1)
	pwaBridge := NewPWABridge(tree, sseManager, watcherEngine, 1, "testapp")
	httpServer := NewHTTPServer(":8080", "", nil, assetCache, sseManager, pwaBridge, tree, 1)

	// Create MCP message — pre-auth, type must be login or signup.
	mcpMsg := MCPMessage{
		Type:      "login",
		RequestID: "req-123",
		Payload:   "test_payload",
		Sent:      time.Now().UnixMilli(),
	}

	body, err := json.Marshal(mcpMsg)
	if err != nil {
		t.Fatalf("Failed to marshal MCP message: %v", err)
	}

	// Test 1: MCP request to /mcp should be routed to PWA bridge
	req := httptest.NewRequest("POST", "/mcp", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(HeaderDevice, "test-device")
	req.Host = "app.example.com"
	w := httptest.NewRecorder()

	httpServer.handleRequest(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d for MCP request, got %d", http.StatusOK, w.Code)
	}

	// Parse response
	var response map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode MCP response: %v", err)
	}

	if response["status"] != "success" {
		t.Errorf("Expected status 'success', got %s", response["status"])
	}

	if response["request_id"] != mcpMsg.RequestID {
		t.Errorf("Expected request_id %s, got %s", mcpMsg.RequestID, response["request_id"])
	}

	// Verify data was written to tree via PWA bridge
	expectedPath := []string{"world", "apps", "testapp", "devices", "test-device", "login"}
	value, err := tree.Read(expectedPath)
	if err != nil {
		t.Errorf("Failed to read login data from tree: %v", err)
	}

	if string(value.Data) != "test_payload" {
		t.Errorf("Expected 'test_payload', got %s", string(value.Data))
	}

	// Test 2: Non-MCP request should not be routed to PWA bridge
	req2 := httptest.NewRequest("GET", "/style.css", nil)
	req2.Host = "app.example.com"
	w2 := httptest.NewRecorder()

	httpServer.handleRequest(w2, req2)

	// Should get 503 since no assets are set up for this domain
	if w2.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status %d for non-MCP request, got %d", http.StatusServiceUnavailable, w2.Code)
	}

	// Test 3: Verify MCP path detection works correctly
	if !pwaBridge.IsMCPPath("mcp") {
		t.Error("Expected 'mcp' to be detected as MCP path")
	}

	if pwaBridge.IsMCPPath("style.css") {
		t.Error("Expected 'style.css' to not be detected as MCP path")
	}
}

func TestHTTPServer_PWABridgeOutbound_Step9(t *testing.T) {
	// Setup
	tree := NewAssetMockTree()
	watcherEngine := NewMockWatcherEngine()
	assetCache := NewAssetCacheManager(tree, watcherEngine, 1)
	sseManager := NewSSEManager(tree, watcherEngine, 1)
	pwaBridge := NewPWABridge(tree, sseManager, watcherEngine, 1, "testapp")
	httpServer := NewHTTPServer(":8080", "", nil, assetCache, sseManager, pwaBridge, tree, 1)

	deviceID := "step9-test-device"

	// Test 1: Touch device via MCP login message
	mcpMsg := MCPMessage{
		Type:      "login",
		RequestID: "req-step9",
		Payload:   nil,
		Sent:      time.Now().UnixMilli(),
	}

	body, err := json.Marshal(mcpMsg)
	if err != nil {
		t.Fatalf("Failed to marshal MCP message: %v", err)
	}

	req := httptest.NewRequest("POST", "/mcp", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(HeaderDevice, deviceID)
	req.Host = "app.example.com"
	w := httptest.NewRecorder()

	httpServer.handleRequest(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d for MCP message, got %d", http.StatusOK, w.Code)
	}

	// Test 2: Subscribe to a data path via HTTP endpoint
	subscribeReq := map[string]string{
		"action": "subscribe",
		"path":   "world.apps.testapp.shared.notifications",
	}
	subscribeBody, _ := json.Marshal(subscribeReq)

	req = httptest.NewRequest("POST", "/subscribe", bytes.NewReader(subscribeBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(HeaderDevice, deviceID)
	req.Host = "app.example.com"
	w = httptest.NewRecorder()

	httpServer.handleRequest(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d for subscription, got %d", http.StatusOK, w.Code)
	}

	// Parse response
	var response map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode subscription response: %v", err)
	}

	if response["status"] != "success" {
		t.Errorf("Expected subscription success, got %v", response["status"])
	}

	// Test 3: Verify client SSE path routing
	req = httptest.NewRequest("GET", "/client-sse/"+deviceID, nil)
	req.Host = "app.example.com"
	w = httptest.NewRecorder()

	// Start SSE connection in a goroutine (since it blocks)
	done := make(chan bool)
	go func() {
		httpServer.handleRequest(w, req)
		done <- true
	}()

	// Give time for SSE headers to be set
	time.Sleep(50 * time.Millisecond)

	// Verify SSE headers were set
	contentType := w.Header().Get("Content-Type")
	if contentType != "text/event-stream" {
		t.Errorf("Expected Content-Type 'text/event-stream', got '%s'", contentType)
	}

	// Test 4: Write data to subscribed path and verify outbound processing
	subscribedPath := []string{"world", "apps", "testapp", "shared", "notifications"}
	testValue := storage.Value{
		TypeTag: types.TypeText,
		Data:    []byte("New notification message"),
	}
	tree.Write(subscribedPath, testValue, 1)

	// Force processing of data changes (simulates watcher trigger)
	pwaBridge.ForceProcessDataChanges()

	// Test 5: Verify subscription management through HTTP server
	unsubscribeReq := map[string]string{
		"action": "unsubscribe",
		"path":   "world.apps.testapp.shared.notifications",
	}
	unsubscribeBody, _ := json.Marshal(unsubscribeReq)

	req2 := httptest.NewRequest("POST", "/subscribe", bytes.NewReader(unsubscribeBody))
	req2.Header.Set("Content-Type", "application/json")
	req2.Header.Set(HeaderDevice, deviceID)
	req2.Host = "app.example.com"
	w2 := httptest.NewRecorder()

	httpServer.handleRequest(w2, req2)

	if w2.Code != http.StatusOK {
		t.Errorf("Expected status %d for unsubscription, got %d", http.StatusOK, w2.Code)
	}

	// Test 6: Verify path detection methods work correctly through HTTP server
	isClientSSE, detectedDeviceID := pwaBridge.IsClientSSEPath("client-sse/" + deviceID)
	if !isClientSSE {
		t.Error("Expected client SSE path to be detected")
	}
	if detectedDeviceID != deviceID {
		t.Errorf("Expected device ID %s, got %s", deviceID, detectedDeviceID)
	}

	if !pwaBridge.IsSubscriptionPath("subscribe") {
		t.Error("Expected subscription path to be detected")
	}

	// Test 7: Verify device has subscription state
	subscriptions, exists := pwaBridge.GetClientSubscriptions(deviceID)
	if !exists {
		t.Error("Expected device to exist")
	}

	// Should be empty after unsubscription
	if len(subscriptions) != 0 {
		t.Errorf("Expected 0 subscriptions after unsubscribe, got %d", len(subscriptions))
	}

	// Clean up the SSE request by closing it
	select {
	case <-done:
		// Request completed
	case <-time.After(100 * time.Millisecond):
		// Timeout is acceptable for SSE connections in tests
	}
}

// =====================
// Step 10: Inbound Request Queue Tests
// =====================

func TestHTTPServer_InboundRequestQueue_BasicRequest(t *testing.T) {
	// Setup
	tree := NewAssetMockTree()
	watcherEngine := NewMockWatcherEngine()
	assetCache := NewAssetCacheManager(tree, watcherEngine, 1)
	sseManager := NewSSEManager(tree, watcherEngine, 1)
	pwaBridge := NewPWABridge(tree, sseManager, watcherEngine, 1, "testapp")
	httpServer := NewHTTPServer(":8080", "", nil, assetCache, sseManager, pwaBridge, tree, 1)

	// Override request TTL for faster testing
	httpServer.requestTTL = 100 * time.Millisecond

	// Test 1: Send external API request and verify request node is created
	reqBody := `{"order_id": "12345", "amount": 100.50}`
	req := httptest.NewRequest("POST", "/api/orders", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer token123")
	req.Header.Set("X-Custom-Header", "custom-value")
	req.Host = "api.example.com"
	w := httptest.NewRecorder()

	// Handle request in goroutine since it blocks waiting for response
	done := make(chan bool)
	go func() {
		httpServer.handleRequest(w, req)
		done <- true
	}()

	// Give time for request node to be written
	time.Sleep(50 * time.Millisecond)

	// Verify request node was created in storage tree
	requestFound := false
	requestPath := []string{"my", "computer", "network", "web", "requests"}

	// Check if any request nodes were created
	requests, err := tree.ReadAllChildren(requestPath)

	// Check if any request nodes were created

	if err == nil && len(requests) > 0 {
		requestFound = true

		// Verify the request has correct structure
		for _, requestValue := range requests {
			if requestValue.TypeTag == types.TypeRecord {
				// Parse the JSON data to verify contents
				var requestData map[string]interface{}
				if err := json.Unmarshal(requestValue.Data, &requestData); err == nil {
					// Verify required fields
					if requestData["domain"] == "api.example.com" &&
						requestData["method"] == "POST" &&
						requestData["path"] == "/api/orders" &&
						requestData["body"] == reqBody {
						// Verify headers
						if headers, ok := requestData["headers"].(map[string]interface{}); ok {
							if headers["Content-Type"] == "application/json" &&
								headers["Authorization"] == "Bearer token123" &&
								headers["X-Custom-Header"] == "custom-value" {
								// Request structure is correct
								break
							}
						}
					}
				}
			}
		}
	}

	if !requestFound {
		t.Error("Expected request node to be created in storage tree")
	}

	// Wait for TTL to expire
	select {
	case <-done:
		// Request completed
	case <-time.After(200 * time.Millisecond):
		t.Error("Request handling did not complete within timeout")
	}

	// Verify 503 response due to TTL expiry
	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status %d for TTL expiry, got %d", http.StatusServiceUnavailable, w.Code)
	}
}

func TestHTTPServer_InboundRequestQueue_WatcherResponse(t *testing.T) {
	// Setup
	tree := NewAssetMockTree()
	watcherEngine := NewMockWatcherEngine()
	assetCache := NewAssetCacheManager(tree, watcherEngine, 1)
	sseManager := NewSSEManager(tree, watcherEngine, 1)
	pwaBridge := NewPWABridge(tree, sseManager, watcherEngine, 1, "testapp")
	httpServer := NewHTTPServer(":8080", "", nil, assetCache, sseManager, pwaBridge, tree, 1)

	// Override request TTL for faster testing
	httpServer.requestTTL = 2 * time.Second

	// Test 2: Send request and have a simulated watcher respond
	reqBody := `{"product_id": "ABC123"}`
	req := httptest.NewRequest("GET", "/api/product/ABC123", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	req.Host = "api.example.com"
	w := httptest.NewRecorder()

	// Handle request in goroutine
	done := make(chan bool)
	requestID := ""
	go func() {
		httpServer.handleRequest(w, req)
		done <- true
	}()

	// Give time for request to be processed and get the request ID
	time.Sleep(50 * time.Millisecond)

	// Find the request ID from pending requests
	httpServer.pendingMutex.RLock()
	for id := range httpServer.pendingRequests {
		requestID = id
		break
	}
	httpServer.pendingMutex.RUnlock()

	if requestID == "" {
		t.Fatal("No pending request found")
	}

	// Simulate watcher response after 100ms
	time.Sleep(50 * time.Millisecond)
	response := &RequestResponse{
		StatusCode: 200,
		Body:       `{"name": "Widget", "price": 29.99, "in_stock": true}`,
		Headers: map[string]string{
			"Content-Type":  "application/json",
			"X-API-Version": "1.0",
		},
	}

	// Send response
	if !httpServer.SendRequestResponse(requestID, response) {
		t.Error("Failed to send response to pending request")
	}

	// Wait for request completion
	select {
	case <-done:
		// Request completed
	case <-time.After(500 * time.Millisecond):
		t.Error("Request handling did not complete within timeout")
	}

	// Verify response
	if w.Code != 200 {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	expectedBody := `{"name": "Widget", "price": 29.99, "in_stock": true}`
	if strings.TrimSpace(w.Body.String()) != expectedBody {
		t.Errorf("Expected body %q, got %q", expectedBody, w.Body.String())
	}

	// Verify headers
	if w.Header().Get("Content-Type") != "application/json" {
		t.Errorf("Expected Content-Type header to be set")
	}

	if w.Header().Get("X-API-Version") != "1.0" {
		t.Errorf("Expected custom header to be set")
	}
}

func TestHTTPServer_InboundRequestQueue_RequestAttributes(t *testing.T) {
	// Setup
	tree := NewAssetMockTree()
	watcherEngine := NewMockWatcherEngine()
	assetCache := NewAssetCacheManager(tree, watcherEngine, 1)
	sseManager := NewSSEManager(tree, watcherEngine, 1)
	pwaBridge := NewPWABridge(tree, sseManager, watcherEngine, 1, "testapp")
	httpServer := NewHTTPServer(":8080", "", nil, assetCache, sseManager, pwaBridge, tree, 1)

	// Override request TTL for faster testing
	httpServer.requestTTL = 100 * time.Millisecond

	// Test 3: Verify all request attributes are correctly captured
	reqBody := `{"webhook": "test", "timestamp": 1234567890}`
	req := httptest.NewRequest("PUT", "/webhook/github?repo=test&event=push", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-GitHub-Event", "push")
	req.Header.Set("X-GitHub-Delivery", "12345-67890")
	req.Header.Set("X-Forwarded-For", "192.168.1.100")
	req.Host = "webhooks.example.com"
	w := httptest.NewRecorder()

	// Handle request in goroutine
	done := make(chan bool)
	go func() {
		httpServer.handleRequest(w, req)
		done <- true
	}()

	// Give time for request node to be written
	time.Sleep(50 * time.Millisecond)

	// Verify request node attributes
	requestPath := []string{"my", "computer", "network", "web", "requests"}
	requests, err := tree.ReadAllChildren(requestPath)
	if err != nil || len(requests) == 0 {
		t.Fatal("Expected at least one request node")
	}

	// Find and verify the request
	requestFound := false
	for _, requestValue := range requests {
		if requestValue.TypeTag == types.TypeRecord {
			var requestData map[string]interface{}
			if err := json.Unmarshal(requestValue.Data, &requestData); err == nil {
				// Check domain
				if requestData["domain"] != "webhooks.example.com" {
					continue
				}

				// Verify all required attributes exist
				requiredFields := []string{"id", "domain", "method", "path", "query", "headers", "body", "remote_ip", "time"}
				for _, field := range requiredFields {
					if _, exists := requestData[field]; !exists {
						t.Errorf("Missing required field: %s", field)
					}
				}

				// Verify specific values
				if requestData["method"] != "PUT" {
					t.Errorf("Expected method PUT, got %v", requestData["method"])
				}

				if requestData["path"] != "/webhook/github" {
					t.Errorf("Expected path /webhook/github, got %v", requestData["path"])
				}

				if requestData["body"] != reqBody {
					t.Errorf("Expected body %q, got %v", reqBody, requestData["body"])
				}

				// Verify query parameters
				if query, ok := requestData["query"].(map[string]interface{}); ok {
					if query["repo"] != "test" || query["event"] != "push" {
						t.Error("Query parameters not correctly parsed")
					}
				} else {
					t.Error("Query should be a record/map")
				}

				// Verify headers
				if headers, ok := requestData["headers"].(map[string]interface{}); ok {
					expectedHeaders := map[string]string{
						"Content-Type":      "application/json",
						"X-Github-Event":    "push",
						"X-Github-Delivery": "12345-67890",
					}

					for key, expectedValue := range expectedHeaders {
						if headers[key] != expectedValue {
							t.Errorf("Expected header %s=%s, got %v", key, expectedValue, headers[key])
						}
					}
				} else {
					t.Error("Headers should be a record/map")
				}

				// Verify remote IP (should use X-Forwarded-For if present)
				if requestData["remote_ip"] != "192.168.1.100" {
					t.Errorf("Expected remote_ip to use X-Forwarded-For value, got %v", requestData["remote_ip"])
				}

				// Verify time is a valid timestamp
				if timeStr, ok := requestData["time"].(string); ok {
					if _, err := time.Parse(time.RFC3339, timeStr); err != nil {
						t.Errorf("Invalid time format: %s", timeStr)
					}
				} else {
					t.Error("Time should be a string in RFC3339 format")
				}

				requestFound = true
				break
			}
		}
	}

	if !requestFound {
		t.Error("Could not find and verify request node with correct attributes")
	}

	// Wait for TTL expiry
	select {
	case <-done:
		// Request completed
	case <-time.After(200 * time.Millisecond):
		t.Error("Request handling did not complete within timeout")
	}

	// Should get 503 due to no response
	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status %d for no response, got %d", http.StatusServiceUnavailable, w.Code)
	}
}

func TestHTTPServer_InboundRequestQueue_TTLExpiry(t *testing.T) {
	// Setup
	tree := NewAssetMockTree()
	watcherEngine := NewMockWatcherEngine()
	assetCache := NewAssetCacheManager(tree, watcherEngine, 1)
	sseManager := NewSSEManager(tree, watcherEngine, 1)
	pwaBridge := NewPWABridge(tree, sseManager, watcherEngine, 1, "testapp")
	httpServer := NewHTTPServer(":8080", "", nil, assetCache, sseManager, pwaBridge, tree, 1)

	// Set very short TTL for this test
	httpServer.requestTTL = 50 * time.Millisecond

	// Test 4: Verify TTL expiry behavior
	req := httptest.NewRequest("DELETE", "/api/users/123", nil)
	req.Host = "api.example.com"
	w := httptest.NewRecorder()

	startTime := time.Now()

	// Handle request in goroutine
	done := make(chan bool)
	go func() {
		httpServer.handleRequest(w, req)
		done <- true
	}()

	// Wait for completion
	select {
	case <-done:
		// Request completed
	case <-time.After(200 * time.Millisecond):
		t.Error("Request handling did not complete within timeout")
	}

	duration := time.Since(startTime)

	// Verify request took approximately the TTL time
	if duration < 40*time.Millisecond || duration > 100*time.Millisecond {
		t.Errorf("Expected request to take ~50ms TTL time, took %v", duration)
	}

	// Verify 503 response
	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status %d for TTL expiry, got %d", http.StatusServiceUnavailable, w.Code)
	}

	expectedBody := "Service Unavailable\n"
	if w.Body.String() != expectedBody {
		t.Errorf("Expected body %q, got %q", expectedBody, w.Body.String())
	}
}

// generateSelfSignedCert creates a self-signed certificate for testing
func generateSelfSignedCert(hosts []string) (certFile, keyFile string, err error) {
	// Generate unique file names to avoid conflicts
	timestamp := time.Now().UnixNano()
	// Generate private key
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return "", "", err
	}

	// Create certificate template
	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization:  []string{"Test"},
			Country:       []string{"US"},
			Province:      []string{""},
			Locality:      []string{"Test"},
			StreetAddress: []string{""},
			PostalCode:    []string{""},
		},
		NotBefore:   time.Now(),
		NotAfter:    time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:    x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IPAddresses: nil,
		DNSNames:    hosts,
	}

	// Create certificate
	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		return "", "", err
	}

	// Create temporary cert file with unique name
	certFile = fmt.Sprintf("/tmp/test_cert_%d.pem", timestamp)
	certOut, err := os.Create(certFile)
	if err != nil {
		return "", "", err
	}
	defer certOut.Close()

	err = pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	if err != nil {
		return "", "", err
	}

	// Create temporary key file with unique name
	keyFile = fmt.Sprintf("/tmp/test_key_%d.pem", timestamp)
	keyOut, err := os.Create(keyFile)
	if err != nil {
		return "", "", err
	}
	defer keyOut.Close()

	privKeyBytes := x509.MarshalPKCS1PrivateKey(priv)
	err = pem.Encode(keyOut, &pem.Block{Type: "RSA PRIVATE KEY", Bytes: privKeyBytes})
	if err != nil {
		return "", "", err
	}

	return certFile, keyFile, nil
}

// TestHTTPServer_TLS_BasicHTTPS tests basic HTTPS functionality with a self-signed cert
func TestHTTPServer_TLS_BasicHTTPS(t *testing.T) {
	// Create self-signed certificate for localhost
	certFile, keyFile, err := generateSelfSignedCert([]string{"localhost", "127.0.0.1"})
	if err != nil {
		t.Fatalf("Failed to generate test certificate: %v", err)
	}
	defer os.Remove(certFile)
	defer os.Remove(keyFile)

	// Create asset cache, SSE manager, and PWA bridge
	tree := NewAssetMockTree()
	watcherEngine := NewMockWatcherEngine()
	assetCache := NewAssetCacheManager(tree, watcherEngine, 1)
	sseManager := NewSSEManager(tree, watcherEngine, 1)
	pwaBridge := NewPWABridge(tree, sseManager, watcherEngine, 1, "testapp")

	// Set up TLS configuration
	tlsConfigs := map[string]config.TLSConfig{
		"localhost": {
			CertFile: certFile,
			KeyFile:  keyFile,
		},
	}

	// Create HTTP server with TLS
	httpServer := NewHTTPServer(":8080", ":8443", tlsConfigs, assetCache, sseManager, pwaBridge, tree, 1)

	// Test that TLS configuration was set up
	if httpServer.tlsConfig == nil {
		t.Fatal("Expected TLS configuration to be set up")
	}

	if httpServer.httpsServer == nil {
		t.Fatal("Expected HTTPS server to be created")
	}

	// Test that the certificate can be loaded via GetCertificate
	clientHello := &tls.ClientHelloInfo{
		ServerName: "localhost",
	}
	cert, err := httpServer.tlsConfig.GetCertificate(clientHello)
	if err != nil {
		t.Fatalf("Failed to get certificate for localhost: %v", err)
	}

	if cert == nil {
		t.Fatal("Expected certificate to be returned")
	}

	// Verify that the certificate contains localhost in its DNS names
	x509Cert, err := x509.ParseCertificate(cert.Certificate[0])
	if err != nil {
		t.Fatalf("Failed to parse certificate: %v", err)
	}

	found := false
	for _, name := range x509Cert.DNSNames {
		if name == "localhost" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected certificate to contain localhost in DNS names")
	}
}

// TestHTTPServer_TLS_SNIRouting tests SNI routing with multiple domains
func TestHTTPServer_TLS_SNIRouting(t *testing.T) {
	// Create self-signed certificates for two different domains
	cert1File, key1File, err := generateSelfSignedCert([]string{"example.com"})
	if err != nil {
		t.Fatalf("Failed to generate certificate for example.com: %v", err)
	}
	defer os.Remove(cert1File)
	defer os.Remove(key1File)

	cert2File, key2File, err := generateSelfSignedCert([]string{"test.com"})
	if err != nil {
		t.Fatalf("Failed to generate certificate for test.com: %v", err)
	}
	defer os.Remove(cert2File)
	defer os.Remove(key2File)

	// Create asset cache, SSE manager, and PWA bridge
	tree := NewAssetMockTree()
	watcherEngine := NewMockWatcherEngine()
	assetCache := NewAssetCacheManager(tree, watcherEngine, 1)
	sseManager := NewSSEManager(tree, watcherEngine, 1)
	pwaBridge := NewPWABridge(tree, sseManager, watcherEngine, 1, "testapp")

	// Set up TLS configuration with multiple domains
	tlsConfigs := map[string]config.TLSConfig{
		"example.com": {
			CertFile: cert1File,
			KeyFile:  key1File,
		},
		"test.com": {
			CertFile: cert2File,
			KeyFile:  key2File,
		},
	}

	// Create HTTP server with TLS
	httpServer := NewHTTPServer(":8080", ":8443", tlsConfigs, assetCache, sseManager, pwaBridge, tree, 1)

	// Test certificate retrieval for example.com
	clientHello1 := &tls.ClientHelloInfo{
		ServerName: "example.com",
	}
	cert1, err := httpServer.tlsConfig.GetCertificate(clientHello1)
	if err != nil {
		t.Fatalf("Failed to get certificate for example.com: %v", err)
	}

	// Test certificate retrieval for test.com
	clientHello2 := &tls.ClientHelloInfo{
		ServerName: "test.com",
	}
	cert2, err := httpServer.tlsConfig.GetCertificate(clientHello2)
	if err != nil {
		t.Fatalf("Failed to get certificate for test.com: %v", err)
	}

	// Verify that different certificates were returned
	if len(cert1.Certificate) == 0 || len(cert2.Certificate) == 0 {
		t.Fatal("Expected certificates to have at least one certificate in the chain")
	}

	if bytes.Equal(cert1.Certificate[0], cert2.Certificate[0]) {
		// Debug: let's see what's actually in the certificates
		x509Cert1Debug, _ := x509.ParseCertificate(cert1.Certificate[0])
		x509Cert2Debug, _ := x509.ParseCertificate(cert2.Certificate[0])
		t.Errorf("Expected different certificates for different domains. Cert1 DNS: %v, Cert2 DNS: %v",
			x509Cert1Debug.DNSNames, x509Cert2Debug.DNSNames)
	}

	// Parse and verify certificate domains
	x509Cert1, err := x509.ParseCertificate(cert1.Certificate[0])
	if err != nil {
		t.Fatalf("Failed to parse certificate 1: %v", err)
	}

	x509Cert2, err := x509.ParseCertificate(cert2.Certificate[0])
	if err != nil {
		t.Fatalf("Failed to parse certificate 2: %v", err)
	}

	// Check that example.com certificate contains example.com
	found := false
	for _, name := range x509Cert1.DNSNames {
		if name == "example.com" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected certificate 1 to contain example.com in DNS names")
	}

	// Check that test.com certificate contains test.com
	found = false
	for _, name := range x509Cert2.DNSNames {
		if name == "test.com" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected certificate 2 to contain test.com in DNS names")
	}
}

// TestHTTPServer_TLS_NoSNI tests fallback behavior when no SNI is provided
func TestHTTPServer_TLS_NoSNI(t *testing.T) {
	// Create self-signed certificate for localhost
	certFile, keyFile, err := generateSelfSignedCert([]string{"localhost"})
	if err != nil {
		t.Fatalf("Failed to generate test certificate: %v", err)
	}
	defer os.Remove(certFile)
	defer os.Remove(keyFile)

	// Create asset cache, SSE manager, and PWA bridge
	tree := NewAssetMockTree()
	watcherEngine := NewMockWatcherEngine()
	assetCache := NewAssetCacheManager(tree, watcherEngine, 1)
	sseManager := NewSSEManager(tree, watcherEngine, 1)
	pwaBridge := NewPWABridge(tree, sseManager, watcherEngine, 1, "testapp")

	// Set up TLS configuration
	tlsConfigs := map[string]config.TLSConfig{
		"localhost": {
			CertFile: certFile,
			KeyFile:  keyFile,
		},
	}

	// Create HTTP server with TLS
	httpServer := NewHTTPServer(":8080", ":8443", tlsConfigs, assetCache, sseManager, pwaBridge, tree, 1)

	// Test certificate retrieval without SNI (empty ServerName)
	clientHello := &tls.ClientHelloInfo{
		ServerName: "", // No SNI
	}
	cert, err := httpServer.tlsConfig.GetCertificate(clientHello)
	if err != nil {
		t.Fatalf("Failed to get fallback certificate: %v", err)
	}

	if cert == nil {
		t.Fatal("Expected fallback certificate to be returned")
	}

	// Should fallback to the first configured domain (localhost)
	x509Cert, err := x509.ParseCertificate(cert.Certificate[0])
	if err != nil {
		t.Fatalf("Failed to parse certificate: %v", err)
	}

	found := false
	for _, name := range x509Cert.DNSNames {
		if name == "localhost" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected fallback certificate to contain localhost in DNS names")
	}
}

// TestHTTPServer_TLS_UnknownDomain tests behavior with unknown domain in SNI
func TestHTTPServer_TLS_UnknownDomain(t *testing.T) {
	// Create self-signed certificate for localhost
	certFile, keyFile, err := generateSelfSignedCert([]string{"localhost"})
	if err != nil {
		t.Fatalf("Failed to generate test certificate: %v", err)
	}
	defer os.Remove(certFile)
	defer os.Remove(keyFile)

	// Create asset cache, SSE manager, and PWA bridge
	tree := NewAssetMockTree()
	watcherEngine := NewMockWatcherEngine()
	assetCache := NewAssetCacheManager(tree, watcherEngine, 1)
	sseManager := NewSSEManager(tree, watcherEngine, 1)
	pwaBridge := NewPWABridge(tree, sseManager, watcherEngine, 1, "testapp")

	// Set up TLS configuration
	tlsConfigs := map[string]config.TLSConfig{
		"localhost": {
			CertFile: certFile,
			KeyFile:  keyFile,
		},
	}

	// Create HTTP server with TLS
	httpServer := NewHTTPServer(":8080", ":8443", tlsConfigs, assetCache, sseManager, pwaBridge, tree, 1)

	// Test certificate retrieval for unknown domain
	clientHello := &tls.ClientHelloInfo{
		ServerName: "unknown.example.com",
	}
	cert, err := httpServer.tlsConfig.GetCertificate(clientHello)
	if err == nil {
		t.Error("Expected error for unknown domain")
	}

	if cert != nil {
		t.Error("Expected no certificate for unknown domain")
	}
}

// TestHTTPServer_HTTPSRedirect tests HTTP to HTTPS redirect functionality
func TestHTTPServer_HTTPSRedirect(t *testing.T) {
	// Create asset cache, SSE manager, and PWA bridge
	tree := NewAssetMockTree()
	watcherEngine := NewMockWatcherEngine()
	assetCache := NewAssetCacheManager(tree, watcherEngine, 1)
	sseManager := NewSSEManager(tree, watcherEngine, 1)
	pwaBridge := NewPWABridge(tree, sseManager, watcherEngine, 1, "testapp")

	// Create self-signed certificate for testing
	certFile, keyFile, err := generateSelfSignedCert([]string{"localhost"})
	if err != nil {
		t.Fatalf("Failed to generate test certificate: %v", err)
	}
	defer os.Remove(certFile)
	defer os.Remove(keyFile)

	// Set up TLS configuration
	tlsConfigs := map[string]config.TLSConfig{
		"localhost": {
			CertFile: certFile,
			KeyFile:  keyFile,
		},
	}

	// Create HTTP server with TLS
	httpServer := NewHTTPServer(":8080", ":8443", tlsConfigs, assetCache, sseManager, pwaBridge, tree, 1)

	// Test HTTP redirect to HTTPS
	req := httptest.NewRequest("GET", "http://localhost:8080/test", nil)
	req.Host = "localhost:8080"
	w := httptest.NewRecorder()

	httpServer.redirectToHTTPS(w, req)

	// Should return 301 redirect
	if w.Code != http.StatusPermanentRedirect {
		t.Errorf("Expected status %d for HTTPS redirect, got %d", http.StatusPermanentRedirect, w.Code)
	}

	location := w.Header().Get("Location")
	expectedLocation := "https://localhost:8080/test"
	if location != expectedLocation {
		t.Errorf("Expected Location header %q, got %q", expectedLocation, location)
	}
}
