package service

// End-to-end auth integration test for the PWA bridge — Step D1 of
// docs/pwa_auth_development_plan.md. Drives the device/token model through
// nine scenarios: pre-auth login screen, signup with token, authenticated
// writes routed to user subtree, per-user intent processing, logout, re-login
// over the same identity, multi-device SSE under one identity, and token
// expiry. The reference login/signup MBL watchers from
// web/boilerplate/auth/{login,signup}.mbl are simulated in Go so the test
// remains package-local; it asserts the bridge's observable behavior, not
// the MBL interpreter.
//
// Note on file location: the development plan named this file
// test/auth_integration_test.go, but the bridge's test helpers
// (newTestBridge, mcpRequest, mcpRequestAuth, writeTokenEntry,
// expireTokenEntry, startSSEConn, closeSSEConn) live in package service and
// are unexported. Reproducing them in an external package would mean
// rebuilding ~100 lines of plumbing, so the test sits next to the helpers it
// reuses. The contract being tested is unchanged.

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/solifugus/amorphdb/internal/storage"
	"github.com/solifugus/amorphdb/internal/types"
)

// credStore is a minimal in-memory user store. Signup adds; verify checks.
type credStore struct {
	mu    sync.Mutex
	users map[string]string // username -> password
}

func newCredStore() *credStore { return &credStore{users: make(map[string]string)} }

func (c *credStore) signup(username, password string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, exists := c.users[username]; exists {
		return false
	}
	c.users[username] = password
	return true
}

func (c *credStore) verify(username, password string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	pw, ok := c.users[username]
	return ok && pw == password
}

// simulatedWatchers stands in for the reference MBL login/signup watchers
// and the per-user logout/task watchers that an app would register at signup.
// It polls the tree every 20ms; on any new login/signup payload it issues a
// token entry and writes a result; on any new task or intent payload under a
// known user's subtree it produces a deterministic side effect (task ->
// task.result, intent.logout=true -> purges that user's tokens).
type simulatedWatchers struct {
	bridge       *PWABridge
	creds        *credStore
	mu           sync.Mutex
	seen         map[string]string // path -> last payload
	tokenCounter int
	knownUsers   map[string]bool
	stopOnce     sync.Once
	cancel       context.CancelFunc
	done         chan struct{}
}

func startSimulatedWatchers(bridge *PWABridge, creds *credStore) *simulatedWatchers {
	ctx, cancel := context.WithCancel(context.Background())
	sw := &simulatedWatchers{
		bridge:     bridge,
		creds:      creds,
		seen:       make(map[string]string),
		knownUsers: make(map[string]bool),
		cancel:     cancel,
		done:       make(chan struct{}),
	}
	go sw.run(ctx)
	return sw
}

func (sw *simulatedWatchers) stop() {
	sw.stopOnce.Do(func() {
		sw.cancel()
		<-sw.done
	})
}

func (sw *simulatedWatchers) nextToken(prefix string) string {
	sw.mu.Lock()
	defer sw.mu.Unlock()
	sw.tokenCounter++
	return fmt.Sprintf("%s-%d", prefix, sw.tokenCounter)
}

func (sw *simulatedWatchers) markUser(username string) {
	sw.mu.Lock()
	sw.knownUsers[username] = true
	sw.mu.Unlock()
}

func (sw *simulatedWatchers) listUsers() []string {
	sw.mu.Lock()
	defer sw.mu.Unlock()
	users := make([]string, 0, len(sw.knownUsers))
	for u := range sw.knownUsers {
		users = append(users, u)
	}
	return users
}

func (sw *simulatedWatchers) run(ctx context.Context) {
	defer close(sw.done)
	tick := time.NewTicker(20 * time.Millisecond)
	defer tick.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		}
		sw.processOnce()
	}
}

// processOnce runs a single pass over the tracked devices and known users,
// reacting to any new payloads. The bridge stores each MCP payload as a
// single JSON-text value at devices.<id>.<type> or users.<identity>.<type>;
// this function reads that value, parses it, and writes the response
// values that a real watcher would produce.
func (sw *simulatedWatchers) processOnce() {
	// Snapshot the current device set under the bridge mutex to avoid
	// racing with HandleMCPRequest.
	sw.bridge.mutex.RLock()
	deviceIDs := make([]string, 0, len(sw.bridge.devices))
	for id := range sw.bridge.devices {
		deviceIDs = append(deviceIDs, id)
	}
	sw.bridge.mutex.RUnlock()

	for _, did := range deviceIDs {
		sw.processDeviceAuth(did, "login")
		sw.processDeviceAuth(did, "signup")
	}
	for _, user := range sw.listUsers() {
		sw.processUserTask(user)
		sw.processUserIntent(user)
	}
}

// processDeviceAuth handles a login or signup submission for a device. On
// signup, a fresh user is recorded in credStore, a stub profile is written,
// and a token is issued; on login, the credentials are checked and a token
// is issued only on match. Either way a JSON result is written under
// devices.<id>.<kind>.result so the bridge's waitForLoginResult delivers it.
func (sw *simulatedWatchers) processDeviceAuth(deviceID, kind string) {
	parent := []string{"world", "apps", "testapp", "devices", deviceID, kind}
	val, err := sw.bridge.tree.Read(parent)
	if err != nil {
		return
	}
	payload := string(val.Data)
	seenKey := strings.Join(parent, "/")
	sw.mu.Lock()
	if sw.seen[seenKey] == payload {
		sw.mu.Unlock()
		return
	}
	sw.seen[seenKey] = payload
	sw.mu.Unlock()

	var body map[string]interface{}
	if err := json.Unmarshal([]byte(payload), &body); err != nil {
		return
	}
	username, _ := body["username"].(string)
	password, _ := body["password"].(string)
	if username == "" {
		return
	}

	var result map[string]interface{}
	switch kind {
	case "signup":
		if sw.creds.signup(username, password) {
			profilePath := []string{"world", "apps", "testapp", "users", username, "profile"}
			seedSig, _ := json.Marshal(map[string]interface{}{
				"username":   username,
				"created_at": time.Now().UnixMilli(),
			})
			_ = sw.bridge.tree.Write(profilePath, storage.Value{
				TypeTag: types.TypeText, Data: seedSig,
			}, 1)
			sw.markUser(username)
			tok := sw.nextToken("signup")
			seedTokenEntry(sw.bridge, tok, username, deviceID)
			result = map[string]interface{}{
				"status": "ok", "token": tok, "identity": username,
			}
		} else {
			result = map[string]interface{}{
				"status": "error", "reason": "username taken",
			}
		}
	case "login":
		if sw.creds.verify(username, password) {
			sw.markUser(username)
			tok := sw.nextToken("login")
			seedTokenEntry(sw.bridge, tok, username, deviceID)
			result = map[string]interface{}{
				"status": "ok", "token": tok, "identity": username,
			}
		} else {
			result = map[string]interface{}{
				"status": "error", "reason": "invalid credentials",
			}
		}
	}
	resultBytes, _ := json.Marshal(result)
	resultPath := []string{"world", "apps", "testapp", "devices", deviceID, kind, "result"}
	_ = sw.bridge.tree.Write(resultPath, storage.Value{
		TypeTag: types.TypeText, Data: resultBytes,
	}, 1)
}

// processUserTask reacts to writes at users.<id>.task by computing a result
// at users.<id>.task.result. The action "ping" produces "pong" — a tiny but
// observable side effect that proves the per-user watcher fired.
func (sw *simulatedWatchers) processUserTask(username string) {
	parent := []string{"world", "apps", "testapp", "users", username, "task"}
	val, err := sw.bridge.tree.Read(parent)
	if err != nil {
		return
	}
	payload := string(val.Data)
	seenKey := strings.Join(parent, "/")
	sw.mu.Lock()
	if sw.seen[seenKey] == payload {
		sw.mu.Unlock()
		return
	}
	sw.seen[seenKey] = payload
	sw.mu.Unlock()

	var body map[string]interface{}
	if err := json.Unmarshal([]byte(payload), &body); err != nil {
		return
	}
	action, _ := body["action"].(string)
	resp := "unknown"
	if action == "ping" {
		resp = "pong"
	}
	resultPath := []string{"world", "apps", "testapp", "users", username, "task", "result"}
	resultBytes, _ := json.Marshal(map[string]interface{}{
		"reply": resp, "for_action": action,
	})
	_ = sw.bridge.tree.Write(resultPath, storage.Value{
		TypeTag: types.TypeText, Data: resultBytes,
	}, 1)
}

// processUserIntent mirrors the per-user logout watcher from setup.mbl: when
// users.<id>.intent has logout=true, every token whose .identity matches
// this user is purged. The intent payload is then marked processed so the
// watcher does not re-fire.
func (sw *simulatedWatchers) processUserIntent(username string) {
	parent := []string{"world", "apps", "testapp", "users", username, "intent"}
	val, err := sw.bridge.tree.Read(parent)
	if err != nil {
		return
	}
	payload := string(val.Data)
	seenKey := strings.Join(parent, "/")
	sw.mu.Lock()
	if sw.seen[seenKey] == payload {
		sw.mu.Unlock()
		return
	}
	sw.seen[seenKey] = payload
	sw.mu.Unlock()

	var body map[string]interface{}
	if err := json.Unmarshal([]byte(payload), &body); err != nil {
		return
	}
	if logout, _ := body["logout"].(bool); !logout {
		return
	}
	purgeUserTokens(sw.bridge, username)
}

// seedTokenEntry writes the two attributes that resolveAuth checks. It
// mirrors writeTokenEntry from pwa_bridge_test.go but is callable from
// non-test goroutines (no *testing.T dependency).
func seedTokenEntry(bridge *PWABridge, token, identity, device string) {
	idPath := []string{"world", "apps", "testapp", "tokens", token, "identity"}
	devPath := []string{"world", "apps", "testapp", "tokens", token, "device"}
	_ = bridge.tree.Write(idPath, storage.Value{
		TypeTag: types.TypeText, Data: []byte(identity),
	}, 1)
	_ = bridge.tree.Write(devPath, storage.Value{
		TypeTag: types.TypeText, Data: []byte(device),
	}, 1)
}

// purgeUserTokens removes every world.apps.testapp.tokens.<token>.* entry
// whose identity attribute equals username. The simulated logout watcher
// uses this to mimic the MBL `for token in world.apps.testapp.tokens: if
// token.identity = username: token = unknown` pattern.
func purgeUserTokens(bridge *PWABridge, username string) {
	mock, ok := bridge.tree.(*AssetMockTree)
	if !ok {
		return
	}
	prefix := "world.apps.testapp.tokens."
	tokensToDelete := make(map[string]bool)
	for k, v := range mock.data {
		if !strings.HasPrefix(k, prefix) {
			continue
		}
		// Identify token attributes that match this user. Token sub-keys
		// are world.apps.testapp.tokens.<token>.identity.
		if strings.HasSuffix(k, ".identity") && string(v.Data) == username {
			rest := strings.TrimPrefix(k, prefix)
			rest = strings.TrimSuffix(rest, ".identity")
			tokensToDelete[rest] = true
		}
	}
	for tok := range tokensToDelete {
		tp := prefix + tok
		for k := range mock.data {
			if k == tp || strings.HasPrefix(k, tp+".") {
				delete(mock.data, k)
			}
		}
	}
}

// authPost issues an MCP message via HandleMCPRequest and returns the
// decoded response body. token may be empty for pre-auth requests. nonce is
// embedded into the payload so repeated calls with otherwise-identical
// content produce distinct stored values (the simulated watcher dedupes on
// payload string and would otherwise skip the second submission).
func authPost(t *testing.T, bridge *PWABridge, deviceID, token, msgType string, payload map[string]interface{}, nonce string) (*httptest.ResponseRecorder, map[string]interface{}) {
	t.Helper()
	if payload == nil {
		payload = map[string]interface{}{}
	}
	if nonce != "" {
		payload["_nonce"] = nonce
	}
	msg := MCPMessage{
		Type:      msgType,
		RequestID: "req-" + nonce,
		Payload:   payload,
		Sent:      time.Now().UnixMilli(),
	}
	body, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("marshal mcp: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(HeaderDevice, deviceID)
	if token != "" {
		req.Header.Set(HeaderToken, token)
	}
	w := httptest.NewRecorder()
	bridge.HandleMCPRequest(w, req)

	var decoded map[string]interface{}
	if w.Body.Len() > 0 && strings.HasPrefix(w.Header().Get("Content-Type"), "application/json") {
		if err := json.NewDecoder(w.Body).Decode(&decoded); err != nil {
			// Fallback: surface raw body for debugging.
			decoded = map[string]interface{}{"_raw": w.Body.String()}
		}
	}
	return w, decoded
}

// resultToken pulls a string token out of a {"result": {"token": ...}}
// response body. Fails the test if missing.
func resultToken(t *testing.T, body map[string]interface{}) string {
	t.Helper()
	res, ok := body["result"].(map[string]interface{})
	if !ok {
		t.Fatalf("response missing result map; got %v", body)
	}
	if status, _ := res["status"].(string); status != "ok" {
		t.Fatalf("watcher result status != ok; got %v", res)
	}
	tok, _ := res["token"].(string)
	if tok == "" {
		t.Fatalf("watcher result missing token; got %v", res)
	}
	return tok
}

// TestEndToEndAuth walks the nine-scenario contract from Step D1.
func TestEndToEndAuth(t *testing.T) {
	bridge := newTestBridge(t)
	// The simulated watchers tick every 20ms; give the bridge enough headroom
	// for at least a few ticks to pick up a payload, write a result, and
	// have waitForLoginResult observe it.
	bridge.loginResultTimeout = 800 * time.Millisecond

	creds := newCredStore()
	sw := startSimulatedWatchers(bridge, creds)
	defer sw.stop()

	deviceA := "dev-A-flow"
	deviceB := "dev-B-flow"
	username := "kalevo"
	password := "s3cret-pw"

	// ---- Phase 1 + 2: pre-auth, login screen ----
	// A pre-auth device that posts a non-login/signup type must be rejected
	// 401. The PWA boilerplate interprets that as "show the login screen".
	// We also touch the device with a (validated) login submission so it is
	// tracked by the bridge and SSE would target the device subtree.
	w, _ := authPost(t, bridge, deviceA, "", "orders", map[string]interface{}{"foo": "bar"}, "p1-bad")
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("Phase 1: expected 401 for pre-auth non-auth type, got %d (body=%s)", w.Code, w.Body.String())
	}
	dev, ok := bridge.GetDeviceInfo(deviceA)
	if ok && dev.Identity != "" {
		t.Errorf("Phase 1: device A should be pre-auth (Identity=\"\"), got %q", dev.Identity)
	}

	// ---- Phase 3: signup submission yields a token ----
	w, body := authPost(t, bridge, deviceA, "", "signup", map[string]interface{}{
		"username": username, "password": password,
	}, "p3-signup")
	if w.Code != http.StatusOK {
		t.Fatalf("Phase 3: signup expected 200, got %d (body=%s)", w.Code, w.Body.String())
	}
	tokenA := resultToken(t, body)

	// ---- Phase 4: post-auth write routes to user subtree ----
	w, body = authPost(t, bridge, deviceA, tokenA, "preferences", map[string]interface{}{
		"theme": "dark",
	}, "p4-prefs")
	if w.Code != http.StatusOK {
		t.Fatalf("Phase 4: authed write expected 200, got %d (body=%s)", w.Code, w.Body.String())
	}
	if body["identity"] != username {
		t.Errorf("Phase 4: response identity %v != %s", body["identity"], username)
	}
	prefsPath := []string{"world", "apps", "testapp", "users", username, "preferences"}
	if v, err := bridge.tree.Read(prefsPath); err != nil {
		t.Errorf("Phase 4: prefs not at user subtree: %v", err)
	} else if !strings.Contains(string(v.Data), `"theme":"dark"`) {
		t.Errorf("Phase 4: prefs content unexpected: %s", string(v.Data))
	}
	// Verify the bridge did NOT route this to devices.<deviceA>.preferences.
	if _, err := bridge.tree.Read([]string{
		"world", "apps", "testapp", "devices", deviceA, "preferences",
	}); err == nil {
		t.Error("Phase 4: authed write leaked into devices subtree")
	}

	// ---- Phase 5: per-user watcher fires on intent, returns result ----
	w, _ = authPost(t, bridge, deviceA, tokenA, "task", map[string]interface{}{
		"action": "ping",
	}, "p5-task")
	if w.Code != http.StatusOK {
		t.Fatalf("Phase 5: task post expected 200, got %d", w.Code)
	}
	taskResultPath := []string{"world", "apps", "testapp", "users", username, "task", "result"}
	deadline := time.Now().Add(time.Second)
	var taskResult string
	for time.Now().Before(deadline) {
		if v, err := bridge.tree.Read(taskResultPath); err == nil {
			taskResult = string(v.Data)
			if strings.Contains(taskResult, "pong") {
				break
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	if !strings.Contains(taskResult, `"reply":"pong"`) {
		t.Errorf("Phase 5: per-user task watcher did not produce pong; got %q", taskResult)
	}

	// ---- Phase 6: logout via intent → token deleted, next request 401 ----
	w, _ = authPost(t, bridge, deviceA, tokenA, "intent", map[string]interface{}{
		"logout": true,
	}, "p6-logout")
	if w.Code != http.StatusOK {
		t.Fatalf("Phase 6: logout intent expected 200, got %d", w.Code)
	}
	// Wait for the per-user logout watcher to run and purge tokens.
	deadline = time.Now().Add(time.Second)
	tokenAlive := true
	for time.Now().Before(deadline) {
		if _, err := bridge.tree.Read([]string{
			"world", "apps", "testapp", "tokens", tokenA, "identity",
		}); err != nil {
			tokenAlive = false
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if tokenAlive {
		t.Fatal("Phase 6: token not purged after logout intent")
	}
	w, _ = authPost(t, bridge, deviceA, tokenA, "preferences", map[string]interface{}{
		"theme": "light",
	}, "p6-after")
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("Phase 6: post-logout request expected 401, got %d (body=%s)", w.Code, w.Body.String())
	}

	// ---- Phase 7: re-login → fresh token, same user data persists ----
	if v, err := bridge.tree.Read(prefsPath); err != nil {
		t.Errorf("Phase 7: user preferences should persist across logout: %v", err)
	} else if !strings.Contains(string(v.Data), `"theme":"dark"`) {
		t.Errorf("Phase 7: preferences mutated; got %s", string(v.Data))
	}
	profilePath := []string{"world", "apps", "testapp", "users", username, "profile"}
	if _, err := bridge.tree.Read(profilePath); err != nil {
		t.Errorf("Phase 7: user profile lost: %v", err)
	}
	w, body = authPost(t, bridge, deviceA, "", "login", map[string]interface{}{
		"username": username, "password": password,
	}, "p7-relogin")
	if w.Code != http.StatusOK {
		t.Fatalf("Phase 7: re-login expected 200, got %d (body=%s)", w.Code, w.Body.String())
	}
	tokenA2 := resultToken(t, body)
	if tokenA2 == tokenA {
		t.Errorf("Phase 7: expected fresh token, got original token back")
	}
	w, body = authPost(t, bridge, deviceA, tokenA2, "preferences", map[string]interface{}{
		"theme": "dark",
	}, "p7-write")
	if w.Code != http.StatusOK {
		t.Fatalf("Phase 7: write with new token expected 200, got %d", w.Code)
	}
	if body["identity"] != username {
		t.Errorf("Phase 7: identity mismatch after relogin: %v", body["identity"])
	}

	// ---- Phase 8: second device login → both devices receive SSE events ----
	w, body = authPost(t, bridge, deviceB, "", "login", map[string]interface{}{
		"username": username, "password": password,
	}, "p8-login-B")
	if w.Code != http.StatusOK {
		t.Fatalf("Phase 8: deviceB login expected 200, got %d", w.Code)
	}
	tokenB := resultToken(t, body)
	if tokenB == tokenA2 {
		t.Errorf("Phase 8: deviceB got same token as deviceA")
	}
	wA, cancelA, doneA := startSSEConn(t, bridge, deviceA, tokenA2)
	wB, cancelB, doneB := startSSEConn(t, bridge, deviceB, tokenB)
	for _, did := range []string{deviceA, deviceB} {
		info, ok := bridge.GetDeviceInfo(did)
		if !ok {
			t.Fatalf("Phase 8: device %s not tracked", did)
		}
		if info.Identity != username {
			t.Errorf("Phase 8: device %s Identity=%q expected %s", did, info.Identity, username)
		}
	}
	notifPath := "world.apps.testapp.users." + username + ".notification"
	if err := bridge.SubscribeClientToPath(deviceA, notifPath); err != nil {
		t.Fatalf("Phase 8: subscribe A: %v", err)
	}
	if err := bridge.SubscribeClientToPath(deviceB, notifPath); err != nil {
		t.Fatalf("Phase 8: subscribe B: %v", err)
	}
	_ = bridge.tree.Write(strings.Split(notifPath, "."), storage.Value{
		TypeTag: types.TypeText, Data: []byte("hello-team"),
	}, 1)
	bridge.ForceProcessDataChanges()
	time.Sleep(80 * time.Millisecond)

	closeSSEConn(t, cancelA, doneA)
	closeSSEConn(t, cancelB, doneB)
	time.Sleep(30 * time.Millisecond)
	if !strings.Contains(wA.Body.String(), notifPath) {
		t.Errorf("Phase 8: deviceA SSE missing notification path; body=%q", wA.Body.String())
	}
	if !strings.Contains(wB.Body.String(), notifPath) {
		t.Errorf("Phase 8: deviceB SSE missing notification path; body=%q", wB.Body.String())
	}

	// ---- Phase 9: token expiry → 401, login screen shown ----
	expireTokenEntry(t, bridge, tokenB)
	w, _ = authPost(t, bridge, deviceB, tokenB, "preferences", map[string]interface{}{
		"theme": "neon",
	}, "p9-expired")
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("Phase 9: expired token expected 401, got %d (body=%s)", w.Code, w.Body.String())
	}
	// The other device's token (tokenA2) is unaffected — it should still work.
	w, _ = authPost(t, bridge, deviceA, tokenA2, "preferences", map[string]interface{}{
		"theme": "midnight",
	}, "p9-other-still-ok")
	if w.Code != http.StatusOK {
		t.Errorf("Phase 9: deviceA's token should still be valid; got %d", w.Code)
	}
}
