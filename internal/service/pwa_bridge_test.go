package service

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/solifugus/amorphdb/internal/storage"
	"github.com/solifugus/amorphdb/internal/types"
)

// newTestBridge constructs a PWABridge backed by mock storage and watcher
// for unit tests. The login-result wait is disabled by default so legacy
// tests that do not exercise the result flow do not pay a polling penalty;
// Step A4 tests opt back in by setting bridge.loginResultTimeout directly.
func newTestBridge(t *testing.T) *PWABridge {
	t.Helper()
	tree := NewAssetMockTree()
	sseManager := NewSSEManager(tree, NewMockWatcherEngine(), 1)
	bridge := NewPWABridge(tree, sseManager, NewMockWatcherEngine(), 1, "testapp")
	bridge.loginResultTimeout = 0
	return bridge
}

func mcpRequest(t *testing.T, msg MCPMessage, deviceID string) *http.Request {
	t.Helper()
	body, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("marshal MCP message: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if deviceID != "" {
		req.Header.Set(HeaderDevice, deviceID)
	}
	return req
}

// mcpRequestAuth is mcpRequest plus an X-AmorphDB-Token header.
func mcpRequestAuth(t *testing.T, msg MCPMessage, deviceID, token string) *http.Request {
	req := mcpRequest(t, msg, deviceID)
	if token != "" {
		req.Header.Set(HeaderToken, token)
	}
	return req
}

// writeTokenEntry seeds world.apps.testapp.tokens.<token>.{identity,device}
// in the underlying mock tree, mirroring what a login watcher would write.
func writeTokenEntry(t *testing.T, bridge *PWABridge, token, identity, device string) {
	t.Helper()
	tree := bridge.tree
	identityPath := []string{"world", "apps", "testapp", "tokens", token, "identity"}
	devicePath := []string{"world", "apps", "testapp", "tokens", token, "device"}
	if err := tree.Write(identityPath, storage.Value{TypeTag: types.TypeText, Data: []byte(identity)}, 1); err != nil {
		t.Fatalf("seed identity: %v", err)
	}
	if err := tree.Write(devicePath, storage.Value{TypeTag: types.TypeText, Data: []byte(device)}, 1); err != nil {
		t.Fatalf("seed device: %v", err)
	}
}

// expireTokenEntry simulates @ttl-driven purge by removing the seeded token
// entries from the underlying mock tree.
func expireTokenEntry(t *testing.T, bridge *PWABridge, token string) {
	t.Helper()
	mock, ok := bridge.tree.(*AssetMockTree)
	if !ok {
		t.Fatalf("bridge.tree is not *AssetMockTree (got %T)", bridge.tree)
	}
	prefix := strings.Join([]string{"world", "apps", "testapp", "tokens", token}, ".")
	for k := range mock.data {
		if k == prefix || strings.HasPrefix(k, prefix+".") {
			delete(mock.data, k)
		}
	}
}

// Step A1 Test 1: device header present, type=login → write to
// world.apps.<app>.devices.<deviceId>.login
func TestPWABridge_DeviceLoginWrite(t *testing.T) {
	bridge := newTestBridge(t)
	tree := bridge.tree

	deviceID := "d8f2a1b3c4d5e6f7"
	payload := `{"username":"kalevo","password":"secret"}`

	msg := MCPMessage{
		Type:      "login",
		RequestID: "req-1",
		Payload:   payload,
		Sent:      time.Now().UnixMilli(),
	}

	w := httptest.NewRecorder()
	bridge.HandleMCPRequest(w, mcpRequest(t, msg, deviceID))

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d (body=%s)", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp["status"] != "success" {
		t.Errorf("expected status=success, got %v", resp["status"])
	}
	if resp["device_id"] != deviceID {
		t.Errorf("expected device_id=%s, got %v", deviceID, resp["device_id"])
	}

	// Verify the payload landed at world.apps.testapp.devices.<id>.login
	path := []string{"world", "apps", "testapp", "devices", deviceID, "login"}
	value, err := tree.Read(path)
	if err != nil {
		t.Fatalf("read login path: %v", err)
	}
	if value.TypeTag != types.TypeText {
		t.Errorf("expected TypeText, got %v", value.TypeTag)
	}
	if string(value.Data) != payload {
		t.Errorf("expected payload %q, got %q", payload, string(value.Data))
	}
}

// Step A1 Test 1b: type=signup is also allowed pre-auth.
func TestPWABridge_DeviceSignupWrite(t *testing.T) {
	bridge := newTestBridge(t)
	tree := bridge.tree

	deviceID := "device-signup-1"
	msg := MCPMessage{
		Type:      "signup",
		RequestID: "req-signup",
		Payload:   "registration_data",
		Sent:      time.Now().UnixMilli(),
	}

	w := httptest.NewRecorder()
	bridge.HandleMCPRequest(w, mcpRequest(t, msg, deviceID))

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	path := []string{"world", "apps", "testapp", "devices", deviceID, "signup"}
	value, err := tree.Read(path)
	if err != nil {
		t.Fatalf("read signup path: %v", err)
	}
	if string(value.Data) != "registration_data" {
		t.Errorf("expected 'registration_data', got %s", string(value.Data))
	}
}

// Step A1 Test 2: missing X-AmorphDB-Device header → 400.
func TestPWABridge_MissingDeviceHeaderRejected(t *testing.T) {
	bridge := newTestBridge(t)

	msg := MCPMessage{
		Type:      "login",
		RequestID: "req-2",
		Payload:   "ignored",
		Sent:      time.Now().UnixMilli(),
	}

	w := httptest.NewRecorder()
	bridge.HandleMCPRequest(w, mcpRequest(t, msg, ""))

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d (body=%s)", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), HeaderDevice) {
		t.Errorf("expected error to mention %s header, got %q", HeaderDevice, w.Body.String())
	}
}

// Step A1 Test 3: pre-auth write to a non-login/signup path → 401.
func TestPWABridge_NonAuthSubpathRejected(t *testing.T) {
	bridge := newTestBridge(t)
	tree := bridge.tree

	deviceID := "device-orders-1"
	msg := MCPMessage{
		Type:      "orders",
		RequestID: "req-3",
		Payload:   "should_not_persist",
		Sent:      time.Now().UnixMilli(),
	}

	w := httptest.NewRecorder()
	bridge.HandleMCPRequest(w, mcpRequest(t, msg, deviceID))

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d (body=%s)", w.Code, w.Body.String())
	}

	// Verify nothing was written.
	path := []string{"world", "apps", "testapp", "devices", deviceID, "orders"}
	if _, err := tree.Read(path); err == nil {
		t.Error("expected no data at orders path, but read succeeded")
	}
}

// Step A1 Test 4: two device IDs are isolated from each other.
func TestPWABridge_TwoDeviceIsolation(t *testing.T) {
	bridge := newTestBridge(t)
	tree := bridge.tree

	dev1 := "device-A"
	dev2 := "device-B"

	msg1 := MCPMessage{
		Type:      "login",
		RequestID: "req-A",
		Payload:   "alice_credentials",
		Sent:      time.Now().UnixMilli(),
	}
	msg2 := MCPMessage{
		Type:      "login",
		RequestID: "req-B",
		Payload:   "bob_credentials",
		Sent:      time.Now().UnixMilli(),
	}

	w1 := httptest.NewRecorder()
	bridge.HandleMCPRequest(w1, mcpRequest(t, msg1, dev1))
	if w1.Code != http.StatusOK {
		t.Fatalf("device A request failed: %d", w1.Code)
	}

	w2 := httptest.NewRecorder()
	bridge.HandleMCPRequest(w2, mcpRequest(t, msg2, dev2))
	if w2.Code != http.StatusOK {
		t.Fatalf("device B request failed: %d", w2.Code)
	}

	v1, err := tree.Read([]string{"world", "apps", "testapp", "devices", dev1, "login"})
	if err != nil {
		t.Fatalf("read device A: %v", err)
	}
	if string(v1.Data) != "alice_credentials" {
		t.Errorf("device A: expected alice_credentials, got %s", string(v1.Data))
	}

	v2, err := tree.Read([]string{"world", "apps", "testapp", "devices", dev2, "login"})
	if err != nil {
		t.Fatalf("read device B: %v", err)
	}
	if string(v2.Data) != "bob_credentials" {
		t.Errorf("device B: expected bob_credentials, got %s", string(v2.Data))
	}

	// Cross-checks — neither device sees the other's tree slot.
	if _, err := tree.Read([]string{"world", "apps", "testapp", "devices", dev1, "signup"}); err == nil {
		t.Error("device A signup should be empty")
	}
	if _, err := tree.Read([]string{"world", "apps", "testapp", "devices", dev2, "signup"}); err == nil {
		t.Error("device B signup should be empty")
	}

	if bridge.GetActiveDevices() != 2 {
		t.Errorf("expected 2 tracked devices, got %d", bridge.GetActiveDevices())
	}
}

func TestPWABridge_PayloadTypes(t *testing.T) {
	bridge := newTestBridge(t)
	tree := bridge.tree

	deviceID := "payload-device"

	cases := []struct {
		name         string
		payload      interface{}
		expectedType byte
		expectedData string
	}{
		{"string", "hello", types.TypeText, "hello"},
		{"number", 42.5, types.TypeNumber, "42.5"},
		{"boolean_true", true, types.TypeBoolean, string([]byte{1})},
		{"boolean_false", false, types.TypeBoolean, string([]byte{0})},
		{"nil", nil, types.TypeBoolean, string([]byte{1})},
		{"object", map[string]interface{}{"a": 1}, types.TypeText, `{"a":1}`},
	}

	for i, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			msg := MCPMessage{
				Type:      "login",
				RequestID: "req-" + strconv.Itoa(i),
				Payload:   tc.payload,
				Sent:      time.Now().UnixMilli(),
			}

			w := httptest.NewRecorder()
			bridge.HandleMCPRequest(w, mcpRequest(t, msg, deviceID))
			if w.Code != http.StatusOK {
				t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
			}

			value, err := tree.Read([]string{"world", "apps", "testapp", "devices", deviceID, "login"})
			if err != nil {
				t.Fatalf("read: %v", err)
			}
			if value.TypeTag != tc.expectedType {
				t.Errorf("expected type %v, got %v", tc.expectedType, value.TypeTag)
			}
			if string(value.Data) != tc.expectedData {
				t.Errorf("expected data %q, got %q", tc.expectedData, string(value.Data))
			}
		})
	}
}

func TestPWABridge_InvalidRequests(t *testing.T) {
	bridge := newTestBridge(t)

	cases := []struct {
		name        string
		method      string
		contentType string
		body        string
		device      string
		want        int
		wantSubstr  string
	}{
		{"wrong_method", "GET", "application/json", `{"type":"login","request_id":"r"}`, "dev", http.StatusMethodNotAllowed, "Method not allowed"},
		{"wrong_content_type", "POST", "text/plain", `{"type":"login","request_id":"r"}`, "dev", http.StatusBadRequest, "Content-Type"},
		{"missing_device", "POST", "application/json", `{"type":"login","request_id":"r"}`, "", http.StatusBadRequest, HeaderDevice},
		{"invalid_json", "POST", "application/json", `{not json}`, "dev", http.StatusBadRequest, "Invalid JSON"},
		{"missing_type", "POST", "application/json", `{"request_id":"r"}`, "dev", http.StatusBadRequest, "Missing 'type'"},
		{"missing_request_id", "POST", "application/json", `{"type":"login"}`, "dev", http.StatusBadRequest, "Missing 'request_id'"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, "/mcp", strings.NewReader(tc.body))
			req.Header.Set("Content-Type", tc.contentType)
			if tc.device != "" {
				req.Header.Set(HeaderDevice, tc.device)
			}
			w := httptest.NewRecorder()
			bridge.HandleMCPRequest(w, req)

			if w.Code != tc.want {
				t.Errorf("expected %d, got %d (body=%s)", tc.want, w.Code, w.Body.String())
			}
			if !strings.Contains(w.Body.String(), tc.wantSubstr) {
				t.Errorf("expected body to contain %q, got %q", tc.wantSubstr, w.Body.String())
			}
		})
	}
}

func TestPWABridge_IsMCPPath(t *testing.T) {
	bridge := newTestBridge(t)

	cases := []struct {
		path string
		want bool
	}{
		{"mcp", true},
		{"mcp/", true},
		{"mcp/foo", true},
		{"mcptest", false},
		{"index.html", false},
		{"", false},
	}
	for _, tc := range cases {
		if got := bridge.IsMCPPath(tc.path); got != tc.want {
			t.Errorf("IsMCPPath(%q) = %v, want %v", tc.path, got, tc.want)
		}
	}
}

func TestPWABridge_ClientSubscription(t *testing.T) {
	bridge := newTestBridge(t)
	deviceID := "sub-device-1"

	if err := bridge.SubscribeClientToPath(deviceID, "world.apps.testapp.devices."+deviceID+".login"); err == nil {
		t.Error("expected subscription to fail before device is known")
	}

	// Touch the device by sending a login message.
	msg := MCPMessage{Type: "login", RequestID: "r", Payload: nil}
	w := httptest.NewRecorder()
	bridge.HandleMCPRequest(w, mcpRequest(t, msg, deviceID))

	if err := bridge.SubscribeClientToPath(deviceID, "world.apps.testapp.devices."+deviceID+".login"); err != nil {
		t.Fatalf("subscribe failed: %v", err)
	}

	subs, ok := bridge.GetClientSubscriptions(deviceID)
	if !ok {
		t.Fatal("expected device to be tracked")
	}
	if len(subs) != 1 {
		t.Errorf("expected 1 subscription, got %d", len(subs))
	}

	if err := bridge.UnsubscribeClientFromPath(deviceID, "world.apps.testapp.devices."+deviceID+".login"); err != nil {
		t.Fatalf("unsubscribe failed: %v", err)
	}
	subs, _ = bridge.GetClientSubscriptions(deviceID)
	if len(subs) != 0 {
		t.Errorf("expected 0 subscriptions after unsubscribe, got %d", len(subs))
	}
}

func TestPWABridge_SubscriptionHTTPEndpoint(t *testing.T) {
	bridge := newTestBridge(t)
	deviceID := "sub-device-2"

	// Touch device first
	msg := MCPMessage{Type: "login", RequestID: "r", Payload: nil}
	bridge.HandleMCPRequest(httptest.NewRecorder(), mcpRequest(t, msg, deviceID))

	subscribeBody, _ := json.Marshal(map[string]string{
		"action": "subscribe",
		"path":   "world.apps.testapp.devices." + deviceID + ".login",
	})
	req := httptest.NewRequest(http.MethodPost, "/subscribe", bytes.NewReader(subscribeBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(HeaderDevice, deviceID)
	w := httptest.NewRecorder()

	bridge.HandleSubscriptionRequest(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("subscribe via HTTP failed: %d (body=%s)", w.Code, w.Body.String())
	}

	subs, _ := bridge.GetClientSubscriptions(deviceID)
	if len(subs) != 1 {
		t.Errorf("expected 1 subscription, got %d", len(subs))
	}

	// Missing device header should be rejected.
	reqNoDev := httptest.NewRequest(http.MethodPost, "/subscribe", bytes.NewReader(subscribeBody))
	reqNoDev.Header.Set("Content-Type", "application/json")
	wNoDev := httptest.NewRecorder()
	bridge.HandleSubscriptionRequest(wNoDev, reqNoDev)
	if wNoDev.Code != http.StatusBadRequest {
		t.Errorf("expected 400 without device header, got %d", wNoDev.Code)
	}

	// Invalid action.
	invalidBody, _ := json.Marshal(map[string]string{"action": "bogus", "path": "x"})
	reqInvalid := httptest.NewRequest(http.MethodPost, "/subscribe", bytes.NewReader(invalidBody))
	reqInvalid.Header.Set("Content-Type", "application/json")
	reqInvalid.Header.Set(HeaderDevice, deviceID)
	wInvalid := httptest.NewRecorder()
	bridge.HandleSubscriptionRequest(wInvalid, reqInvalid)
	if wInvalid.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid action, got %d", wInvalid.Code)
	}
}

func TestPWABridge_ClientSSEPath(t *testing.T) {
	bridge := newTestBridge(t)

	cases := []struct {
		path     string
		isSSE    bool
		deviceID string
	}{
		{"client-sse/dev123", true, "dev123"},
		{"client-sse/abc-def", true, "abc-def"},
		{"client-sse/", false, ""},
		{"sse/stream", false, ""},
		{"static/file.css", false, ""},
	}

	for _, tc := range cases {
		isSSE, deviceID := bridge.IsClientSSEPath(tc.path)
		if isSSE != tc.isSSE {
			t.Errorf("IsClientSSEPath(%q) isSSE=%v, want %v", tc.path, isSSE, tc.isSSE)
		}
		if deviceID != tc.deviceID {
			t.Errorf("IsClientSSEPath(%q) deviceID=%q, want %q", tc.path, deviceID, tc.deviceID)
		}
	}
}

func TestPWABridge_DataChangeProcessing(t *testing.T) {
	bridge := newTestBridge(t)
	tree := bridge.tree
	deviceID := "data-change-device"

	// Touch device, subscribe to a path
	bridge.HandleMCPRequest(httptest.NewRecorder(), mcpRequest(t, MCPMessage{
		Type: "login", RequestID: "r",
	}, deviceID))

	if err := bridge.SubscribeClientToPath(deviceID, "world.apps.testapp.shared.price"); err != nil {
		t.Fatalf("subscribe: %v", err)
	}

	tree.Write([]string{"world", "apps", "testapp", "shared", "price"}, storage.Value{
		TypeTag: types.TypeText,
		Data:    []byte("$99"),
	}, 1)

	bridge.ForceProcessDataChanges()

	subs, exists := bridge.GetClientSubscriptions(deviceID)
	if !exists {
		t.Error("device should still be tracked")
	}
	if !subs["world.apps.testapp.shared.price"] {
		t.Error("subscription should still exist")
	}
}

func TestPWABridge_DeviceTracking(t *testing.T) {
	bridge := newTestBridge(t)

	if bridge.GetActiveDevices() != 0 {
		t.Errorf("expected 0 devices, got %d", bridge.GetActiveDevices())
	}

	deviceID := "track-device-1"
	msg := MCPMessage{Type: "login", RequestID: "r"}
	req := mcpRequest(t, msg, deviceID)
	req.Header.Set("User-Agent", "TestUA/1.0")
	req.RemoteAddr = "10.0.0.1:1234"

	w := httptest.NewRecorder()
	bridge.HandleMCPRequest(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d", w.Code)
	}

	if bridge.GetActiveDevices() != 1 {
		t.Errorf("expected 1 device, got %d", bridge.GetActiveDevices())
	}

	device, ok := bridge.GetDeviceInfo(deviceID)
	if !ok {
		t.Fatal("device not tracked")
	}
	if device.UserAgent != "TestUA/1.0" {
		t.Errorf("UserAgent=%q", device.UserAgent)
	}
	if device.RemoteAddr != "10.0.0.1:1234" {
		t.Errorf("RemoteAddr=%q", device.RemoteAddr)
	}

	// Reusing the same device ID does not create another entry.
	bridge.HandleMCPRequest(httptest.NewRecorder(), mcpRequest(t, msg, deviceID))
	if bridge.GetActiveDevices() != 1 {
		t.Errorf("expected 1 device after reuse, got %d", bridge.GetActiveDevices())
	}
}

// Step A2 Test 1: a valid token + matching device routes writes to
// world.apps.<app>.users.<identity>.<type>.
func TestPWABridge_TokenAuthRoutesToUserSubtree(t *testing.T) {
	bridge := newTestBridge(t)
	tree := bridge.tree

	deviceID := "dev-auth-1"
	token := "token-abc-123"
	identity := "kalevo"
	writeTokenEntry(t, bridge, token, identity, deviceID)

	msg := MCPMessage{
		Type:      "submit_order",
		RequestID: "req-auth-1",
		Payload:   `{"item":"widget","qty":3}`,
		Sent:      time.Now().UnixMilli(),
	}

	w := httptest.NewRecorder()
	bridge.HandleMCPRequest(w, mcpRequestAuth(t, msg, deviceID, token))

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body=%s)", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp["status"] != "success" {
		t.Errorf("expected status=success, got %v", resp["status"])
	}
	if resp["identity"] != identity {
		t.Errorf("expected identity=%q in response, got %v", identity, resp["identity"])
	}

	// Verify the payload landed at world.apps.testapp.users.<identity>.submit_order
	userPath := []string{"world", "apps", "testapp", "users", identity, "submit_order"}
	value, err := tree.Read(userPath)
	if err != nil {
		t.Fatalf("read user path: %v", err)
	}
	if string(value.Data) != msg.Payload {
		t.Errorf("expected %q at user path, got %q", msg.Payload, string(value.Data))
	}

	// And NOT at the device subtree (post-auth must not double-write).
	devicePath := []string{"world", "apps", "testapp", "devices", deviceID, "submit_order"}
	if _, err := tree.Read(devicePath); err == nil {
		t.Error("authenticated write must not land at devices.<id>.<type>")
	}
}

// Step A2 Test 2: a token whose entry has been purged (simulating @ttl
// expiry) is rejected with 401, and no write occurs.
func TestPWABridge_ExpiredTokenRejected(t *testing.T) {
	bridge := newTestBridge(t)
	tree := bridge.tree

	deviceID := "dev-expired-1"
	token := "token-expired"
	identity := "miratu"
	writeTokenEntry(t, bridge, token, identity, deviceID)
	expireTokenEntry(t, bridge, token) // purge → token gone

	msg := MCPMessage{
		Type:      "submit_order",
		RequestID: "req-exp-1",
		Payload:   "should_not_persist",
		Sent:      time.Now().UnixMilli(),
	}

	w := httptest.NewRecorder()
	bridge.HandleMCPRequest(w, mcpRequestAuth(t, msg, deviceID, token))

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d (body=%s)", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "expired or unknown") {
		t.Errorf("expected error to mention 'expired or unknown', got %q", w.Body.String())
	}

	// No write to the (former) user's subtree.
	userPath := []string{"world", "apps", "testapp", "users", identity, "submit_order"}
	if _, err := tree.Read(userPath); err == nil {
		t.Error("expected no write under expired-token user, but read succeeded")
	}
}

// Step A2 Test 3: a valid token whose stored .device does not match the
// presenting X-AmorphDB-Device header is rejected with 401.
func TestPWABridge_TokenDeviceMismatchRejected(t *testing.T) {
	bridge := newTestBridge(t)
	tree := bridge.tree

	storedDevice := "dev-A"
	presentingDevice := "dev-B"
	token := "token-mismatch"
	identity := "alice"
	writeTokenEntry(t, bridge, token, identity, storedDevice)

	msg := MCPMessage{
		Type:      "submit_order",
		RequestID: "req-mm-1",
		Payload:   "should_not_persist",
		Sent:      time.Now().UnixMilli(),
	}

	w := httptest.NewRecorder()
	bridge.HandleMCPRequest(w, mcpRequestAuth(t, msg, presentingDevice, token))

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d (body=%s)", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "device mismatch") {
		t.Errorf("expected error to mention 'device mismatch', got %q", w.Body.String())
	}

	// No write to the user's subtree under either device.
	userPath := []string{"world", "apps", "testapp", "users", identity, "submit_order"}
	if _, err := tree.Read(userPath); err == nil {
		t.Error("device-mismatch request must not write to user subtree")
	}
}

// Step A2 Test 4: an unknown token (never seeded) is rejected with 401.
func TestPWABridge_UnknownTokenRejected(t *testing.T) {
	bridge := newTestBridge(t)
	tree := bridge.tree

	deviceID := "dev-unknown-1"

	msg := MCPMessage{
		Type:      "submit_order",
		RequestID: "req-unk-1",
		Payload:   "should_not_persist",
		Sent:      time.Now().UnixMilli(),
	}

	w := httptest.NewRecorder()
	bridge.HandleMCPRequest(w, mcpRequestAuth(t, msg, deviceID, "no-such-token"))

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d (body=%s)", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "expired or unknown") {
		t.Errorf("expected error to mention 'expired or unknown', got %q", w.Body.String())
	}

	// Nothing in users subtree.
	if _, err := tree.Read([]string{"world", "apps", "testapp", "users", "anyone", "submit_order"}); err == nil {
		t.Error("unknown-token request must not write to user subtree")
	}
}

// Step A2 supporting test: an authenticated request whose type is a non-auth
// path (e.g. orders) is permitted (post-auth boundary differs from pre-auth).
// This complements Test 1 and prevents accidental regression of the
// pre-auth-only filter to authenticated requests.
func TestPWABridge_AuthenticatedNonAuthTypeAllowed(t *testing.T) {
	bridge := newTestBridge(t)
	tree := bridge.tree

	deviceID := "dev-ok"
	token := "tok-ok"
	identity := "carol"
	writeTokenEntry(t, bridge, token, identity, deviceID)

	for _, mtype := range []string{"orders", "settings", "intent"} {
		msg := MCPMessage{
			Type:      mtype,
			RequestID: "req-" + mtype,
			Payload:   "ok-" + mtype,
			Sent:      time.Now().UnixMilli(),
		}
		w := httptest.NewRecorder()
		bridge.HandleMCPRequest(w, mcpRequestAuth(t, msg, deviceID, token))
		if w.Code != http.StatusOK {
			t.Fatalf("type=%s: expected 200, got %d (body=%s)", mtype, w.Code, w.Body.String())
		}
		val, err := tree.Read([]string{"world", "apps", "testapp", "users", identity, mtype})
		if err != nil {
			t.Fatalf("type=%s: read user subtree: %v", mtype, err)
		}
		if string(val.Data) != "ok-"+mtype {
			t.Errorf("type=%s: expected %q, got %q", mtype, "ok-"+mtype, string(val.Data))
		}
	}
}

func TestPWABridge_IsSubscriptionPath(t *testing.T) {
	bridge := newTestBridge(t)

	cases := []struct {
		path string
		want bool
	}{
		{"subscribe", true},
		{"subscribe/", true},
		{"subscribe/foo", true},
		{"unsubscribe", false},
		{"mcp", false},
		{"sse/stream", false},
	}
	for _, tc := range cases {
		if got := bridge.IsSubscriptionPath(tc.path); got != tc.want {
			t.Errorf("IsSubscriptionPath(%q)=%v, want %v", tc.path, got, tc.want)
		}
	}
}

// startSSEConn opens a client SSE connection in a goroutine (HandleClientSSERequest
// blocks for the lifetime of the stream). It returns the response recorder and a
// cancel function that closes the connection. The caller must invoke cancel and
// wait briefly for cleanup before inspecting the recorder body.
func startSSEConn(t *testing.T, bridge *PWABridge, deviceID, token string) (*httptest.ResponseRecorder, context.CancelFunc, <-chan struct{}) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	url := "/client-sse/" + deviceID
	if token != "" {
		url += "?token=" + token
	}
	req := httptest.NewRequest(http.MethodGet, url, nil).WithContext(ctx)
	w := httptest.NewRecorder()

	done := make(chan struct{})
	go func() {
		bridge.HandleClientSSERequest(w, req, deviceID)
		close(done)
	}()
	// Give the goroutine time to register its SSE connection.
	time.Sleep(50 * time.Millisecond)
	return w, cancel, done
}

// closeSSEConn cancels the connection and waits for its goroutine to finish.
func closeSSEConn(t *testing.T, cancel context.CancelFunc, done <-chan struct{}) {
	t.Helper()
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Error("SSE connection did not close within 1s")
	}
}

// Step A3 Test 1: an SSE connection opened with only a device ID receives
// events for paths under world.apps.<app>.devices.<deviceId>.*.
func TestPWABridge_SSEPreAuthReceivesDeviceEvents(t *testing.T) {
	bridge := newTestBridge(t)
	tree := bridge.tree
	deviceID := "dev-pre-auth"

	// Touch the device first so subscribe() finds it (HandleClientSSERequest
	// also touches, but we need to subscribe before opening the SSE so the
	// later ForceProcessDataChanges call fires the broadcast).
	bridge.HandleMCPRequest(httptest.NewRecorder(), mcpRequest(t, MCPMessage{
		Type: "login", RequestID: "r-pre", Payload: "creds",
	}, deviceID))

	devicePath := "world.apps.testapp.devices." + deviceID + ".login.result"
	if err := bridge.SubscribeClientToPath(deviceID, devicePath); err != nil {
		t.Fatalf("subscribe: %v", err)
	}

	w, cancel, done := startSSEConn(t, bridge, deviceID, "")

	// Confirm pre-auth: identity is empty.
	dev, ok := bridge.GetDeviceInfo(deviceID)
	if !ok {
		t.Fatal("device not tracked")
	}
	if dev.Identity != "" {
		t.Errorf("expected pre-auth (Identity=\"\"), got %q", dev.Identity)
	}

	// Simulate a login result write that the boilerplate would produce.
	tree.Write(strings.Split(devicePath, "."), storage.Value{
		TypeTag: types.TypeText,
		Data:    []byte(`{"status":"ok"}`),
	}, 1)

	bridge.ForceProcessDataChanges()
	time.Sleep(80 * time.Millisecond)

	closeSSEConn(t, cancel, done)
	time.Sleep(30 * time.Millisecond)

	body := w.Body.String()
	if !strings.Contains(body, "data_change") {
		t.Errorf("expected data_change event, body=%q", body)
	}
	if !strings.Contains(body, devicePath) {
		t.Errorf("expected device path %q in body, got %q", devicePath, body)
	}
}

// Step A3 Test 2: when a token is created and the device is promoted, the
// existing SSE connection now receives events for paths under
// world.apps.<app>.users.<identity>.*.
func TestPWABridge_SSETransitionsToUserEvents(t *testing.T) {
	bridge := newTestBridge(t)
	tree := bridge.tree

	deviceID := "dev-transition"
	identity := "kalevo"
	token := "token-transition"

	// Open connection pre-auth.
	bridge.HandleMCPRequest(httptest.NewRecorder(), mcpRequest(t, MCPMessage{
		Type: "login", RequestID: "r-tx", Payload: "creds",
	}, deviceID))
	w, cancel, done := startSSEConn(t, bridge, deviceID, "")

	dev, _ := bridge.GetDeviceInfo(deviceID)
	if dev.Identity != "" {
		t.Fatalf("expected pre-auth Identity=\"\", got %q", dev.Identity)
	}

	// Login watcher (simulated): create the token entry, then promote.
	writeTokenEntry(t, bridge, token, identity, deviceID)
	if err := bridge.PromoteDeviceToUser(deviceID, identity); err != nil {
		t.Fatalf("promote: %v", err)
	}
	dev, _ = bridge.GetDeviceInfo(deviceID)
	if dev.Identity != identity {
		t.Errorf("expected Identity=%q after promotion, got %q", identity, dev.Identity)
	}

	// Subscribe to a user-subtree path and write a value there.
	userPath := "world.apps.testapp.users." + identity + ".profile"
	if err := bridge.SubscribeClientToPath(deviceID, userPath); err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	tree.Write(strings.Split(userPath, "."), storage.Value{
		TypeTag: types.TypeText,
		Data:    []byte(`{"name":"Kalevo"}`),
	}, 1)

	bridge.ForceProcessDataChanges()
	time.Sleep(80 * time.Millisecond)

	closeSSEConn(t, cancel, done)
	time.Sleep(30 * time.Millisecond)

	body := w.Body.String()
	if !strings.Contains(body, userPath) {
		t.Errorf("expected user path %q in SSE body, got %q", userPath, body)
	}
}

// Step A3 Test 3: when two devices are promoted to the same identity, both
// SSE connections receive events for the user subtree.
func TestPWABridge_SSEMultipleDevicesSameUser(t *testing.T) {
	bridge := newTestBridge(t)
	tree := bridge.tree

	devA := "dev-A-shared"
	devB := "dev-B-shared"
	identity := "shared-user"
	tokenA := "token-A"
	tokenB := "token-B"

	// Touch + register tokens for both devices.
	bridge.HandleMCPRequest(httptest.NewRecorder(), mcpRequest(t, MCPMessage{
		Type: "login", RequestID: "r-A", Payload: "creds",
	}, devA))
	bridge.HandleMCPRequest(httptest.NewRecorder(), mcpRequest(t, MCPMessage{
		Type: "login", RequestID: "r-B", Payload: "creds",
	}, devB))
	writeTokenEntry(t, bridge, tokenA, identity, devA)
	writeTokenEntry(t, bridge, tokenB, identity, devB)

	// Open SSE for each device with token (will auto-promote).
	wA, cancelA, doneA := startSSEConn(t, bridge, devA, tokenA)
	wB, cancelB, doneB := startSSEConn(t, bridge, devB, tokenB)

	for _, dev := range []string{devA, devB} {
		info, ok := bridge.GetDeviceInfo(dev)
		if !ok {
			t.Fatalf("%s not tracked", dev)
		}
		if info.Identity != identity {
			t.Errorf("%s: expected Identity=%q, got %q", dev, identity, info.Identity)
		}
	}

	// Both subscribe to the same user-scoped path.
	userPath := "world.apps.testapp.users." + identity + ".notification"
	if err := bridge.SubscribeClientToPath(devA, userPath); err != nil {
		t.Fatalf("subscribe A: %v", err)
	}
	if err := bridge.SubscribeClientToPath(devB, userPath); err != nil {
		t.Fatalf("subscribe B: %v", err)
	}

	tree.Write(strings.Split(userPath, "."), storage.Value{
		TypeTag: types.TypeText,
		Data:    []byte("hello-team"),
	}, 1)

	bridge.ForceProcessDataChanges()
	time.Sleep(80 * time.Millisecond)

	closeSSEConn(t, cancelA, doneA)
	closeSSEConn(t, cancelB, doneB)
	time.Sleep(30 * time.Millisecond)

	if !strings.Contains(wA.Body.String(), userPath) {
		t.Errorf("device A did not receive user event: body=%q", wA.Body.String())
	}
	if !strings.Contains(wB.Body.String(), userPath) {
		t.Errorf("device B did not receive user event: body=%q", wB.Body.String())
	}
}

// Step A3 Test 4: a write under user A's subtree does not reach a connection
// promoted to a different user B.
func TestPWABridge_SSEDifferentUsersIsolated(t *testing.T) {
	bridge := newTestBridge(t)
	tree := bridge.tree

	devA := "dev-A-iso"
	devB := "dev-B-iso"
	idA := "alice"
	idB := "bob"
	tokA := "tok-A-iso"
	tokB := "tok-B-iso"

	bridge.HandleMCPRequest(httptest.NewRecorder(), mcpRequest(t, MCPMessage{
		Type: "login", RequestID: "r-A", Payload: "creds",
	}, devA))
	bridge.HandleMCPRequest(httptest.NewRecorder(), mcpRequest(t, MCPMessage{
		Type: "login", RequestID: "r-B", Payload: "creds",
	}, devB))
	writeTokenEntry(t, bridge, tokA, idA, devA)
	writeTokenEntry(t, bridge, tokB, idB, devB)

	wA, cancelA, doneA := startSSEConn(t, bridge, devA, tokA)
	wB, cancelB, doneB := startSSEConn(t, bridge, devB, tokB)

	pathA := "world.apps.testapp.users." + idA + ".x"
	pathB := "world.apps.testapp.users." + idB + ".y"
	if err := bridge.SubscribeClientToPath(devA, pathA); err != nil {
		t.Fatalf("subscribe A: %v", err)
	}
	if err := bridge.SubscribeClientToPath(devB, pathB); err != nil {
		t.Fatalf("subscribe B: %v", err)
	}

	// Write only under Alice's subtree.
	tree.Write(strings.Split(pathA, "."), storage.Value{
		TypeTag: types.TypeText,
		Data:    []byte("alice-only"),
	}, 1)

	bridge.ForceProcessDataChanges()
	time.Sleep(80 * time.Millisecond)

	closeSSEConn(t, cancelA, doneA)
	closeSSEConn(t, cancelB, doneB)
	time.Sleep(30 * time.Millisecond)

	if !strings.Contains(wA.Body.String(), pathA) {
		t.Errorf("Alice's SSE missed her own event: body=%q", wA.Body.String())
	}
	// Bob's stream must not see Alice's path or value.
	if strings.Contains(wB.Body.String(), pathA) {
		t.Errorf("Bob's SSE leaked Alice's path: body=%q", wB.Body.String())
	}
	if strings.Contains(wB.Body.String(), "alice-only") {
		t.Errorf("Bob's SSE leaked Alice's data: body=%q", wB.Body.String())
	}
}

// Step A3 supporting test: opening an SSE connection with an invalid token
// returns 401 (and does not promote anything).
func TestPWABridge_SSEInvalidTokenRejected(t *testing.T) {
	bridge := newTestBridge(t)

	deviceID := "dev-bad-tok"
	bridge.HandleMCPRequest(httptest.NewRecorder(), mcpRequest(t, MCPMessage{
		Type: "login", RequestID: "r", Payload: "creds",
	}, deviceID))

	req := httptest.NewRequest(http.MethodGet, "/client-sse/"+deviceID+"?token=no-such", nil)
	w := httptest.NewRecorder()
	bridge.HandleClientSSERequest(w, req, deviceID)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d (body=%s)", w.Code, w.Body.String())
	}

	dev, _ := bridge.GetDeviceInfo(deviceID)
	if dev.Identity != "" {
		t.Errorf("expected Identity to remain empty, got %q", dev.Identity)
	}
}

// Step A3 supporting test: PromoteDeviceToUser preconditions.
func TestPWABridge_PromoteDeviceToUserErrors(t *testing.T) {
	bridge := newTestBridge(t)

	if err := bridge.PromoteDeviceToUser("nope", "alice"); err == nil {
		t.Error("expected error promoting unknown device")
	}

	deviceID := "dev-promote"
	bridge.HandleMCPRequest(httptest.NewRecorder(), mcpRequest(t, MCPMessage{
		Type: "login", RequestID: "r", Payload: "creds",
	}, deviceID))

	if err := bridge.PromoteDeviceToUser(deviceID, ""); err == nil {
		t.Error("expected error promoting to empty identity")
	}

	if err := bridge.PromoteDeviceToUser(deviceID, "u1"); err != nil {
		t.Fatalf("promote: %v", err)
	}
	dev, _ := bridge.GetDeviceInfo(deviceID)
	if dev.Identity != "u1" {
		t.Errorf("expected Identity=u1, got %q", dev.Identity)
	}
}

// writeLoginResult seeds the bytes a login/signup watcher would publish to
// world.apps.testapp.devices.<deviceID>.<msgType>.result. The bridge stores
// it as TypeText and JSON-decodes on read, matching what real watchers will
// produce once Phase C lands.
func writeLoginResult(t *testing.T, bridge *PWABridge, deviceID, msgType, jsonBody string) {
	t.Helper()
	path := []string{"world", "apps", "testapp", "devices", deviceID, msgType, "result"}
	if err := bridge.tree.Write(path, storage.Value{
		TypeTag: types.TypeText,
		Data:    []byte(jsonBody),
	}, 1); err != nil {
		t.Fatalf("seed login result: %v", err)
	}
}

// Step A4 Test 1: a watcher publishes {status:ok, token:..., identity:...}
// at devices.<id>.login.result before the request completes. The bridge
// surfaces the parsed object as response.result so the browser can store
// the token.
func TestPWABridge_LoginResultReturnsToken(t *testing.T) {
	bridge := newTestBridge(t)
	bridge.loginResultTimeout = 200 * time.Millisecond
	tree := bridge.tree

	deviceID := "dev-login-ok"
	identity := "kalevo"
	token := "tok-from-watcher"

	// Pre-stage the watcher's result so it is visible on the bridge's
	// first poll. Real watchers publish after the credentials write; here
	// we just need a value at the path the bridge reads.
	writeLoginResult(t, bridge, deviceID, "login",
		`{"status":"ok","token":"`+token+`","identity":"`+identity+`"}`)

	creds := `{"username":"kalevo","password":"hunter2"}`
	msg := MCPMessage{
		Type:      "login",
		RequestID: "req-login-1",
		Payload:   creds,
		Sent:      time.Now().UnixMilli(),
	}

	w := httptest.NewRecorder()
	bridge.HandleMCPRequest(w, mcpRequest(t, msg, deviceID))

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body=%s)", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	resultRaw, ok := resp["result"]
	if !ok {
		t.Fatalf("expected 'result' field in response, got %v", resp)
	}
	result, ok := resultRaw.(map[string]interface{})
	if !ok {
		t.Fatalf("expected result to be an object, got %T", resultRaw)
	}
	if result["status"] != "ok" {
		t.Errorf("expected result.status=ok, got %v", result["status"])
	}
	if result["token"] != token {
		t.Errorf("expected result.token=%q, got %v", token, result["token"])
	}
	if result["identity"] != identity {
		t.Errorf("expected result.identity=%q, got %v", identity, result["identity"])
	}

	// Bridge must still have routed the credentials verbatim, regardless of
	// the watcher's verdict.
	credPath := []string{"world", "apps", "testapp", "devices", deviceID, "login"}
	val, err := tree.Read(credPath)
	if err != nil {
		t.Fatalf("read credentials: %v", err)
	}
	if string(val.Data) != creds {
		t.Errorf("credentials altered: expected %q, got %q", creds, string(val.Data))
	}
}

// Step A4 Test 2: a watcher publishes {status:error, message:...} at the
// result path; the bridge surfaces the error untouched.
func TestPWABridge_LoginResultReturnsError(t *testing.T) {
	bridge := newTestBridge(t)
	bridge.loginResultTimeout = 200 * time.Millisecond

	deviceID := "dev-login-err"
	errMsg := "Invalid credentials"

	writeLoginResult(t, bridge, deviceID, "login",
		`{"status":"error","message":"`+errMsg+`"}`)

	msg := MCPMessage{
		Type:      "login",
		RequestID: "req-login-err",
		Payload:   `{"username":"kalevo","password":"wrong"}`,
		Sent:      time.Now().UnixMilli(),
	}

	w := httptest.NewRecorder()
	bridge.HandleMCPRequest(w, mcpRequest(t, msg, deviceID))

	// Routing succeeded (200) — the failed credential check is the
	// watcher's verdict, not an MCP-level error.
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for routed login (auth verdict carried in body), got %d (body=%s)",
			w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	result, ok := resp["result"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected result object, got %v", resp["result"])
	}
	if result["status"] != "error" {
		t.Errorf("expected result.status=error, got %v", result["status"])
	}
	if result["message"] != errMsg {
		t.Errorf("expected result.message=%q, got %v", errMsg, result["message"])
	}
	if _, hasToken := result["token"]; hasToken {
		t.Error("error result must not carry a token")
	}
}

// Step A4 Test 3: the bridge does not interpret credentials. Whatever the
// browser submits as the payload is written verbatim to the device login
// path; the bridge does not validate, transform, or short-circuit based on
// content. It only relays the watcher's verdict.
func TestPWABridge_LoginCredentialsRoutedVerbatim(t *testing.T) {
	bridge := newTestBridge(t)
	bridge.loginResultTimeout = 50 * time.Millisecond

	cases := []struct {
		name    string
		payload string
	}{
		{"empty_object", `{}`},
		{"missing_password", `{"username":"alice"}`},
		{"sql_injection_attempt", `{"username":"' OR 1=1 --","password":"x"}`},
		{"non_credential_blob", `{"foo":"bar","note":"not even a credential"}`},
		{"deeply_nested", `{"creds":{"u":"a","p":"b"},"meta":{"ts":1}}`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			deviceID := "dev-routed-" + tc.name
			tree := bridge.tree

			msg := MCPMessage{
				Type:      "login",
				RequestID: "req-routed-" + tc.name,
				Payload:   tc.payload,
				Sent:      time.Now().UnixMilli(),
			}

			w := httptest.NewRecorder()
			bridge.HandleMCPRequest(w, mcpRequest(t, msg, deviceID))

			if w.Code != http.StatusOK {
				t.Fatalf("expected 200 (bridge must not gate on payload shape), got %d (body=%s)",
					w.Code, w.Body.String())
			}

			path := []string{"world", "apps", "testapp", "devices", deviceID, "login"}
			val, err := tree.Read(path)
			if err != nil {
				t.Fatalf("read login: %v", err)
			}
			if string(val.Data) != tc.payload {
				t.Errorf("payload altered: expected %q, got %q", tc.payload, string(val.Data))
			}

			// No watcher fired in this test, so no result should appear.
			var resp map[string]interface{}
			if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if _, hasResult := resp["result"]; hasResult {
				t.Errorf("no watcher result staged, but response carried 'result'=%v",
					resp["result"])
			}
		})
	}
}

// Step A4 Test 4: signup follows the same pattern as login — the watcher's
// result at devices.<id>.signup.result is surfaced verbatim.
func TestPWABridge_SignupResultReturnsToken(t *testing.T) {
	bridge := newTestBridge(t)
	bridge.loginResultTimeout = 200 * time.Millisecond

	deviceID := "dev-signup-ok"
	identity := "newbie"
	token := "tok-fresh-signup"

	writeLoginResult(t, bridge, deviceID, "signup",
		`{"status":"ok","token":"`+token+`","identity":"`+identity+`"}`)

	msg := MCPMessage{
		Type:      "signup",
		RequestID: "req-signup-ok",
		Payload:   `{"username":"newbie","password":"strongpass"}`,
		Sent:      time.Now().UnixMilli(),
	}

	w := httptest.NewRecorder()
	bridge.HandleMCPRequest(w, mcpRequest(t, msg, deviceID))

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body=%s)", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	result, ok := resp["result"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected result object on signup, got %v", resp["result"])
	}
	if result["token"] != token {
		t.Errorf("expected signup result.token=%q, got %v", token, result["token"])
	}
}

// Step A4 Test 5: a watcher result that lands AFTER the bridge starts polling
// is still picked up before the timeout. Exercises the actual poll loop, not
// just the pre-staged read.
func TestPWABridge_LoginResultArrivesDuringPoll(t *testing.T) {
	bridge := newTestBridge(t)
	bridge.loginResultTimeout = 1 * time.Second

	deviceID := "dev-async"
	token := "tok-async"

	// Schedule the watcher's result write a bit after request entry.
	go func() {
		time.Sleep(100 * time.Millisecond)
		writeLoginResult(t, bridge, deviceID, "login",
			`{"status":"ok","token":"`+token+`"}`)
	}()

	msg := MCPMessage{
		Type:      "login",
		RequestID: "req-async",
		Payload:   `{"username":"u","password":"p"}`,
		Sent:      time.Now().UnixMilli(),
	}

	start := time.Now()
	w := httptest.NewRecorder()
	bridge.HandleMCPRequest(w, mcpRequest(t, msg, deviceID))
	elapsed := time.Since(start)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body=%s)", w.Code, w.Body.String())
	}
	if elapsed < 100*time.Millisecond {
		t.Errorf("returned in %v — must have waited for the watcher", elapsed)
	}
	if elapsed >= bridge.loginResultTimeout {
		t.Errorf("hit full timeout (%v) — async result was never observed", bridge.loginResultTimeout)
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	result, ok := resp["result"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected result object, got %v", resp["result"])
	}
	if result["token"] != token {
		t.Errorf("expected async-delivered token %q, got %v", token, result["token"])
	}
}

// Step A5 Test 1: an authenticated user writing to their own subtree with a
// well-formed type name is allowed and counts no boundary violation.
func TestPWABridge_BoundaryAuthenticatedOwnSubtreeAllowed(t *testing.T) {
	bridge := newTestBridge(t)
	tree := bridge.tree

	deviceID := "dev-bound-own"
	token := "tok-bound-own"
	identity := "alice"
	writeTokenEntry(t, bridge, token, identity, deviceID)

	msg := MCPMessage{
		Type:      "submit_order",
		RequestID: "req-bound-own",
		Payload:   `{"item":"book","qty":1}`,
		Sent:      time.Now().UnixMilli(),
	}

	w := httptest.NewRecorder()
	bridge.HandleMCPRequest(w, mcpRequestAuth(t, msg, deviceID, token))

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body=%s)", w.Code, w.Body.String())
	}
	val, err := tree.Read([]string{"world", "apps", "testapp", "users", identity, "submit_order"})
	if err != nil {
		t.Fatalf("read user subtree: %v", err)
	}
	if string(val.Data) != msg.Payload {
		t.Errorf("expected payload at user subtree, got %q", string(val.Data))
	}
	if got := bridge.BoundaryViolations(); got != 0 {
		t.Errorf("expected 0 violations on legal write, got %d", got)
	}
}

// Step A5 Test 2: an authenticated user attempting to escape their subtree
// via a path-separator-bearing type is rejected with 403, no write occurs,
// and the violation counter increments. This is the only client-controlled
// way to attempt cross-user addressing — the bridge always builds the write
// path as ["world","apps",app,"users",identity,Type], so a Type with ".",
// "/", or "\" is the boundary-evasion surface to defend.
func TestPWABridge_BoundaryAuthenticatedCrossUserRejected(t *testing.T) {
	bridge := newTestBridge(t)
	tree := bridge.tree

	deviceID := "dev-bound-cross"
	token := "tok-bound-cross"
	identity := "alice"
	writeTokenEntry(t, bridge, token, identity, deviceID)

	cases := []string{
		"../bob/x",
		"users.bob.profile",
		"users/bob",
		"..",
		".",
		"x.y",
		"x/y",
		`x\y`,
	}

	for _, badType := range cases {
		t.Run(badType, func(t *testing.T) {
			msg := MCPMessage{
				Type:      badType,
				RequestID: "req-cross-" + badType,
				Payload:   "should_not_persist",
				Sent:      time.Now().UnixMilli(),
			}
			before := bridge.BoundaryViolations()
			w := httptest.NewRecorder()
			bridge.HandleMCPRequest(w, mcpRequestAuth(t, msg, deviceID, token))

			if w.Code != http.StatusForbidden {
				t.Fatalf("type=%q: expected 403, got %d (body=%s)", badType, w.Code, w.Body.String())
			}
			if !strings.Contains(w.Body.String(), "path separator") {
				t.Errorf("type=%q: expected error mentioning 'path separator', got %q", badType, w.Body.String())
			}
			if bridge.BoundaryViolations() != before+1 {
				t.Errorf("type=%q: violation count did not increment (before=%d after=%d)",
					badType, before, bridge.BoundaryViolations())
			}
			// Verify nothing landed at the literal joined path.
			if _, err := tree.Read([]string{"world", "apps", "testapp", "users", identity, badType}); err == nil {
				t.Errorf("type=%q: rejected write must not persist", badType)
			}
		})
	}
}

// Step A5 Test 3: an authenticated user attempting to write to any of the
// bridge-managed reserved subtree names (app, assets, auth, tokens, devices,
// users) is rejected with 403. Even though the bridge's routing would in
// practice place the value at users.<identity>.<reserved> rather than the
// global subtree, the spec forbids browsers from naming these directly.
func TestPWABridge_BoundaryAuthenticatedReservedTypeRejected(t *testing.T) {
	bridge := newTestBridge(t)
	tree := bridge.tree

	deviceID := "dev-bound-reserved"
	token := "tok-bound-reserved"
	identity := "alice"
	writeTokenEntry(t, bridge, token, identity, deviceID)

	reserved := []string{"app", "assets", "auth", "tokens", "devices", "users"}
	for _, name := range reserved {
		t.Run(name, func(t *testing.T) {
			msg := MCPMessage{
				Type:      name,
				RequestID: "req-res-" + name,
				Payload:   "should_not_persist",
				Sent:      time.Now().UnixMilli(),
			}
			before := bridge.BoundaryViolations()
			w := httptest.NewRecorder()
			bridge.HandleMCPRequest(w, mcpRequestAuth(t, msg, deviceID, token))

			if w.Code != http.StatusForbidden {
				t.Fatalf("type=%q: expected 403, got %d (body=%s)", name, w.Code, w.Body.String())
			}
			if !strings.Contains(w.Body.String(), "reserved") {
				t.Errorf("type=%q: expected error mentioning 'reserved', got %q", name, w.Body.String())
			}
			if bridge.BoundaryViolations() != before+1 {
				t.Errorf("type=%q: violation count did not increment", name)
			}
			if _, err := tree.Read([]string{"world", "apps", "testapp", "users", identity, name}); err == nil {
				t.Errorf("type=%q: rejected reserved-type write must not persist", name)
			}
		})
	}
}

// Step A5 Test 4: pre-auth writes to login are allowed and do not register
// as boundary violations. Complements TestPWABridge_DeviceLoginWrite by
// asserting the new violation counter does not falsely trip on the happy
// pre-auth path.
func TestPWABridge_BoundaryPreAuthLoginAllowed(t *testing.T) {
	bridge := newTestBridge(t)
	tree := bridge.tree

	deviceID := "dev-bound-pre-login"
	msg := MCPMessage{
		Type:      "login",
		RequestID: "req-pre-login",
		Payload:   `{"username":"u","password":"p"}`,
		Sent:      time.Now().UnixMilli(),
	}

	w := httptest.NewRecorder()
	bridge.HandleMCPRequest(w, mcpRequest(t, msg, deviceID))

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body=%s)", w.Code, w.Body.String())
	}
	if got := bridge.BoundaryViolations(); got != 0 {
		t.Errorf("expected 0 violations on legal pre-auth write, got %d", got)
	}
	if _, err := tree.Read([]string{"world", "apps", "testapp", "devices", deviceID, "login"}); err != nil {
		t.Errorf("expected credentials to land at devices.<id>.login: %v", err)
	}
}

// Step A5 Test 5: a pre-auth device naming a non-login/signup type — including
// the reserved name "users" — is rejected with 401 (so the boilerplate routes
// the user to the login screen, not to a permission error). The violation
// counter still increments even though the response shape mirrors a generic
// auth-required failure.
func TestPWABridge_BoundaryPreAuthUsersRejected(t *testing.T) {
	bridge := newTestBridge(t)
	tree := bridge.tree

	deviceID := "dev-bound-pre-users"

	cases := []struct {
		name    string
		msgType string
	}{
		{"users_reserved", "users"},
		{"app_reserved", "app"},
		{"orders_non_auth", "orders"},
		{"path_separator", "users.alice"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			msg := MCPMessage{
				Type:      tc.msgType,
				RequestID: "req-pre-" + tc.name,
				Payload:   "should_not_persist",
				Sent:      time.Now().UnixMilli(),
			}
			before := bridge.BoundaryViolations()
			w := httptest.NewRecorder()
			bridge.HandleMCPRequest(w, mcpRequest(t, msg, deviceID))

			if w.Code != http.StatusUnauthorized {
				t.Fatalf("type=%q: expected 401, got %d (body=%s)", tc.msgType, w.Code, w.Body.String())
			}
			if !strings.Contains(w.Body.String(), "Authentication") {
				t.Errorf("type=%q: expected body to mention Authentication, got %q", tc.msgType, w.Body.String())
			}
			if bridge.BoundaryViolations() != before+1 {
				t.Errorf("type=%q: violation count did not increment", tc.msgType)
			}
			if _, err := tree.Read([]string{"world", "apps", "testapp", "devices", deviceID, tc.msgType}); err == nil {
				t.Errorf("type=%q: rejected pre-auth write must not persist", tc.msgType)
			}
		})
	}
}

// Step A4 Test 6: when no watcher publishes a result within the wait window,
// the response omits the "result" field rather than blocking indefinitely or
// inventing one.
func TestPWABridge_LoginResultTimeoutOmitsField(t *testing.T) {
	bridge := newTestBridge(t)
	bridge.loginResultTimeout = 80 * time.Millisecond

	deviceID := "dev-timeout"
	msg := MCPMessage{
		Type:      "login",
		RequestID: "req-timeout",
		Payload:   `{"username":"u","password":"p"}`,
		Sent:      time.Now().UnixMilli(),
	}

	start := time.Now()
	w := httptest.NewRecorder()
	bridge.HandleMCPRequest(w, mcpRequest(t, msg, deviceID))
	elapsed := time.Since(start)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body=%s)", w.Code, w.Body.String())
	}
	if elapsed < bridge.loginResultTimeout {
		t.Errorf("returned in %v before timeout %v", elapsed, bridge.loginResultTimeout)
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if _, has := resp["result"]; has {
		t.Errorf("expected no 'result' field on timeout, got %v", resp["result"])
	}
}
