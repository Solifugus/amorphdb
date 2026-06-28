package service

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/solifugus/amorphdb/internal/storage"
	"github.com/solifugus/amorphdb/internal/types"
)

// HTTP headers used by the PWA bridge (see amorphdb_design.md
// §PWA User Identity and Authentication).
const (
	HeaderDevice = "X-AmorphDB-Device"
	HeaderToken  = "X-AmorphDB-Token"
)

// preAuthSubPaths enumerates the sub-attributes a device may write to before
// it has presented a valid auth token. All other types are rejected with 401.
var preAuthSubPaths = map[string]bool{
	"login":  true,
	"signup": true,
}

// reservedTypes are world.apps.<app> subtree names that the bridge owns and
// that browsers must never name as their MCP message type. The bridge always
// places writes under devices.<id> (pre-auth) or users.<identity> (post-auth),
// so a request that names "tokens" as its type would land at
// users.<identity>.tokens, not the global tokens subtree — but per
// amorphdb_design.md §PWA User Identity and Authentication, no browser may
// address those names directly. Catching them here keeps the boundary
// explicit and surfaces accidental or hostile attempts in the violation log.
var reservedTypes = map[string]bool{
	"app":     true,
	"assets":  true,
	"auth":    true,
	"tokens":  true,
	"devices": true,
	"users":   true,
}

// typeSeparators are characters disallowed in an MCP message type. The bridge
// writes to a single tree segment under devices.<id> or users.<identity>; a
// type containing these characters indicates either a confused client or a
// boundary-evasion attempt and is rejected.
const typeSeparators = "./\\"

// defaultLoginResultTimeout is how long HandleMCPRequest waits, by default,
// for a login/signup watcher to publish a result at
// world.apps.<app>.devices.<deviceId>.<type>.result before returning to the
// browser. Watchers fire on heartbeat boundaries, so 2s gives ample headroom
// while still bounding the response. Tests typically lower this.
const defaultLoginResultTimeout = 2 * time.Second

// loginResultPollInterval is how often waitForLoginResult re-reads the
// result path while waiting. Short enough for snappy responses, long enough
// to avoid hammering storage in the no-watcher case.
const loginResultPollInterval = 50 * time.Millisecond

// MCPMessage represents an incoming message from a PWA client.
type MCPMessage struct {
	Type      string      `json:"type"`
	RequestID string      `json:"request_id"`
	Payload   interface{} `json:"payload"`
	Sent      int64       `json:"_sent"`
}

// DeviceConnection tracks an active browser connection identified by its
// device ID. Devices are self-asserted by the client via the
// X-AmorphDB-Device header. Identity is empty pre-auth and set to the
// resolved user identity once a valid token is presented for the device
// (Step A3); subsequent SSE events are routed under the user subtree.
type DeviceConnection struct {
	DeviceID      string
	Identity      string
	LastActive    time.Time
	UserAgent     string
	RemoteAddr    string
	Subscriptions map[string]bool
}

// PWABridge handles data-driven PWA communication — inbound MCP writes and
// outbound SSE events. Routing follows the device/token/identity model:
// pre-auth requests (no token) are routed to world.apps.<app>.devices.<deviceId>.*
// and only the login/signup sub-paths are writable. Authenticated requests
// (valid X-AmorphDB-Token) are routed to world.apps.<app>.users.<identity>.*.
// Tokens are looked up in world.apps.<app>.tokens.<token>; the entry's .device
// must match the presenting device or the request is rejected with 401.
type PWABridge struct {
	tree               storage.ExtendedTree
	sseManager         *SSEManager
	watcher            WatcherEngineInterface
	mutex              sync.RWMutex
	devices            map[string]*DeviceConnection
	agentID            uint64
	appName            string
	deviceTTL          time.Duration
	loginResultTimeout time.Duration
	// violationCount counts boundary violations rejected by HandleMCPRequest.
	// Read via BoundaryViolations(); used by Step A5 tests to verify that
	// reservation/separator rejections do not silently pass.
	violationCount uint64
}

// NewPWABridge creates a new PWA bridge.
func NewPWABridge(tree storage.ExtendedTree, sseManager *SSEManager, watcherEngine WatcherEngineInterface, agentID uint64, appName string) *PWABridge {
	bridge := &PWABridge{
		tree:               tree,
		sseManager:         sseManager,
		watcher:            watcherEngine,
		devices:            make(map[string]*DeviceConnection),
		agentID:            agentID,
		appName:            appName,
		deviceTTL:          24 * time.Hour,
		loginResultTimeout: defaultLoginResultTimeout,
	}

	go bridge.cleanupExpiredDevices()
	bridge.setupDataChangeWatcher()

	return bridge
}

// HandleMCPRequest handles an incoming MCP message from a PWA client.
// Pre-auth (no token): the client must supply X-AmorphDB-Device, and the
// message Type must be "login" or "signup". Anything else is rejected.
func (pb *PWABridge) HandleMCPRequest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	contentType := r.Header.Get("Content-Type")
	if !strings.Contains(contentType, "application/json") {
		http.Error(w, "Content-Type must be application/json", http.StatusBadRequest)
		return
	}

	deviceID := r.Header.Get(HeaderDevice)
	if deviceID == "" {
		http.Error(w, "Missing "+HeaderDevice+" header", http.StatusBadRequest)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	var mcpMsg MCPMessage
	if err := json.Unmarshal(body, &mcpMsg); err != nil {
		http.Error(w, "Invalid JSON message", http.StatusBadRequest)
		return
	}

	if mcpMsg.Type == "" {
		http.Error(w, "Missing 'type' field", http.StatusBadRequest)
		return
	}
	if mcpMsg.RequestID == "" {
		http.Error(w, "Missing 'request_id' field", http.StatusBadRequest)
		return
	}

	// Resolve auth: a valid token elevates the request to authenticated and
	// pins it to a user identity. No token = pre-auth.
	identity, err := pb.resolveAuth(deviceID, r.Header.Get(HeaderToken))
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	pb.touchDevice(deviceID, r)

	response := map[string]interface{}{
		"status":     "success",
		"request_id": mcpMsg.RequestID,
		"device_id":  deviceID,
		"timestamp":  time.Now().UnixMilli(),
	}

	// Step A5: enforce write boundaries before any write occurs. Both pre-auth
	// and post-auth requests are constrained — pre-auth to login/signup only,
	// post-auth to the user's own subtree, and neither may name a bridge-managed
	// reserved subtree or smuggle path separators in the type.
	authenticated := identity != ""
	if vErr := validateWriteBoundary(authenticated, mcpMsg.Type); vErr != nil {
		pb.logBoundaryViolation(deviceID, identity, mcpMsg.Type, vErr)
		// Pre-auth violations stay 401 (the boilerplate interprets that as
		// "show login screen"). Authenticated violations are 403 — the user
		// is known but is reaching outside their permitted subtree.
		status := http.StatusForbidden
		body := vErr.Error()
		if !authenticated {
			status = http.StatusUnauthorized
			body = "Authentication required"
		}
		http.Error(w, body, status)
		return
	}

	if identity == "" {
		// Pre-auth: route to devices.<deviceId>.<type>. validateWriteBoundary
		// has already confirmed type is login or signup.
		if err := pb.writeDeviceMessage(deviceID, mcpMsg); err != nil {
			http.Error(w, fmt.Sprintf("Failed to write message: %v", err), http.StatusInternalServerError)
			return
		}
		// Login/signup follow a request-response pattern: the watcher publishes
		// a result at devices.<deviceId>.<type>.result and the browser uses
		// the token (or error message) on this same MCP response. Surface
		// whatever the watcher writes within the bounded wait.
		if result, ok := pb.waitForLoginResult(deviceID, mcpMsg.Type, pb.loginResultTimeout); ok {
			response["result"] = result
		}
	} else {
		// Authenticated: route to the user's subtree.
		if err := pb.writeUserMessage(identity, mcpMsg); err != nil {
			http.Error(w, fmt.Sprintf("Failed to write message: %v", err), http.StatusInternalServerError)
			return
		}
		response["identity"] = identity
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// resolveAuth returns the authenticated user identity associated with the
// presented device + token pair. An empty token means the request is pre-auth;
// in that case identity == "" and err == nil. A non-empty token must:
//   - Exist at world.apps.<app>.tokens.<token> (entry purged or absent → 401).
//   - Carry an .identity attribute (text).
//   - Carry a .device attribute that exactly matches deviceID (mismatch → 401).
//
// @ttl-driven expiry is enforced by AmorphDB's purge mechanism; from the
// bridge's perspective, an expired token is simply absent from the tokens
// subtree and is indistinguishable from an unknown token.
func (pb *PWABridge) resolveAuth(deviceID, token string) (string, error) {
	if token == "" {
		return "", nil
	}

	identityPath := []string{"world", "apps", pb.appName, "tokens", token, "identity"}
	identityValue, err := pb.tree.Read(identityPath)
	if err != nil {
		return "", fmt.Errorf("token expired or unknown")
	}

	devicePath := []string{"world", "apps", pb.appName, "tokens", token, "device"}
	deviceValue, err := pb.tree.Read(devicePath)
	if err != nil {
		return "", fmt.Errorf("token expired or unknown")
	}

	if string(deviceValue.Data) != deviceID {
		return "", fmt.Errorf("token device mismatch")
	}

	identity := string(identityValue.Data)
	if identity == "" {
		return "", fmt.Errorf("token has empty identity")
	}
	return identity, nil
}

// validateWriteBoundary applies the bridge's write-boundary rules to an MCP
// message type, given whether the request resolved to an authenticated user.
//
// Rules (per amorphdb_design.md §PWA User Identity and Authentication, and
// pwa_auth_development_plan.md Step A5):
//   - The type must be a single tree segment — empty values and any of
//     ".", "/", "\" are rejected so a Type can never escape its parent
//     subtree (devices.<id> or users.<identity>).
//   - The type must not name a bridge-managed reserved subtree
//     (app, assets, auth, tokens, devices, users). Even though such a
//     value would land at users.<identity>.<reserved> rather than the
//     real subtree, the spec explicitly forbids browsers from addressing
//     these names directly.
//   - Pre-auth (no token), the type must be login or signup. All other
//     types are rejected — there is no other writable surface for an
//     unauthenticated device.
//
// A non-nil return value carries a human-readable explanation suitable for
// both the violation log and (for authenticated callers) the HTTP response
// body. Pre-auth callers should still respond 401 with a generic message.
func validateWriteBoundary(authenticated bool, msgType string) error {
	if msgType == "" {
		return fmt.Errorf("message type is empty")
	}
	if strings.ContainsAny(msgType, typeSeparators) {
		return fmt.Errorf("message type %q contains path separator", msgType)
	}
	if reservedTypes[msgType] {
		return fmt.Errorf("write to reserved subtree %q is not allowed", msgType)
	}
	if !authenticated && !preAuthSubPaths[msgType] {
		return fmt.Errorf("pre-auth write to %q is not allowed (only login/signup)", msgType)
	}
	return nil
}

// logBoundaryViolation records a rejected write attempt to stderr and bumps
// the violation counter for tests. The log line includes the device ID,
// resolved identity (empty pre-auth), the offending type, and the reason —
// enough for an operator to spot a misbehaving client without exposing the
// payload.
func (pb *PWABridge) logBoundaryViolation(deviceID, identity, msgType string, reason error) {
	atomic.AddUint64(&pb.violationCount, 1)
	if identity != "" {
		fmt.Printf("[pwa_bridge] boundary violation: device=%s identity=%s type=%q: %v\n",
			deviceID, identity, msgType, reason)
		return
	}
	fmt.Printf("[pwa_bridge] boundary violation: device=%s pre-auth type=%q: %v\n",
		deviceID, msgType, reason)
}

// BoundaryViolations returns the cumulative count of write-boundary
// rejections that HandleMCPRequest has observed since the bridge started.
// Used by tests to assert that a request was rejected as a violation rather
// than passing through some other code path.
func (pb *PWABridge) BoundaryViolations() uint64 {
	return atomic.LoadUint64(&pb.violationCount)
}

// touchDevice records the device's most recent activity, creating a tracking
// entry if this is the first time we've seen it.
func (pb *PWABridge) touchDevice(deviceID string, r *http.Request) {
	pb.mutex.Lock()
	defer pb.mutex.Unlock()

	device, exists := pb.devices[deviceID]
	if !exists {
		pb.devices[deviceID] = &DeviceConnection{
			DeviceID:      deviceID,
			LastActive:    time.Now(),
			UserAgent:     r.Header.Get("User-Agent"),
			RemoteAddr:    r.RemoteAddr,
			Subscriptions: make(map[string]bool),
		}
		return
	}
	device.LastActive = time.Now()
}

// writeDeviceMessage writes the MCP payload to
// world.apps.<app>.devices.<deviceId>.<type>. Caller has already validated
// that <type> is allowed pre-auth.
func (pb *PWABridge) writeDeviceMessage(deviceID string, mcpMsg MCPMessage) error {
	path := []string{"world", "apps", pb.appName, "devices", deviceID, mcpMsg.Type}

	value, err := mcpPayloadToValue(mcpMsg.Payload)
	if err != nil {
		return err
	}

	if err := pb.tree.Write(path, value, pb.agentID); err != nil {
		return fmt.Errorf("failed to write to storage: %v", err)
	}
	return nil
}

// waitForLoginResult polls world.apps.<app>.devices.<deviceID>.<msgType>.result
// until a value appears or the timeout elapses, parsing the value as JSON
// into a generic object. The bridge does not interpret the contents — it
// just relays whatever the login/signup watcher wrote (token, identity,
// error message, status, etc.) so the browser can act on it. Returning
// (nil, false) means no result was available within the wait window;
// callers should respond without a "result" field in that case.
//
// The bridge always attempts at least one read so callers using a zero
// timeout (typical in tests that do not exercise the result flow) can
// short-circuit immediately when no result has been pre-staged.
func (pb *PWABridge) waitForLoginResult(deviceID, msgType string, timeout time.Duration) (map[string]interface{}, bool) {
	path := []string{"world", "apps", pb.appName, "devices", deviceID, msgType, "result"}
	deadline := time.Now().Add(timeout)
	for {
		if value, err := pb.tree.Read(path); err == nil {
			return decodeLoginResult(value)
		}
		if !time.Now().Before(deadline) {
			return nil, false
		}
		time.Sleep(loginResultPollInterval)
	}
}

// decodeLoginResult interprets the storage.Value at .result as a JSON object.
// Anything else (non-JSON text, primitive type tags) is treated as absent
// because the browser-facing payload requires a structured result.
func decodeLoginResult(value storage.Value) (map[string]interface{}, bool) {
	if len(value.Data) == 0 {
		return nil, false
	}
	var result map[string]interface{}
	if err := json.Unmarshal(value.Data, &result); err != nil {
		return nil, false
	}
	return result, true
}

// writeUserMessage writes the MCP payload to
// world.apps.<app>.users.<identity>.<type>. Caller has already verified that
// the presenting token resolves to <identity>.
func (pb *PWABridge) writeUserMessage(identity string, mcpMsg MCPMessage) error {
	path := []string{"world", "apps", pb.appName, "users", identity, mcpMsg.Type}

	value, err := mcpPayloadToValue(mcpMsg.Payload)
	if err != nil {
		return err
	}

	if err := pb.tree.Write(path, value, pb.agentID); err != nil {
		return fmt.Errorf("failed to write to storage: %v", err)
	}
	return nil
}

// mcpPayloadToValue converts a JSON-decoded MCP payload into a storage.Value.
// Complex objects are serialized as JSON text — Step A1 only routes the
// payload; richer structural mapping is handled by downstream watchers.
func mcpPayloadToValue(payload interface{}) (storage.Value, error) {
	switch p := payload.(type) {
	case nil:
		return storage.Value{TypeTag: types.TypeBoolean, Data: []byte{1}}, nil
	case string:
		return storage.Value{TypeTag: types.TypeText, Data: []byte(p)}, nil
	case float64:
		return storage.Value{
			TypeTag: types.TypeNumber,
			Data:    []byte(strconv.FormatFloat(p, 'f', -1, 64)),
		}, nil
	case bool:
		b := byte(0)
		if p {
			b = 1
		}
		return storage.Value{TypeTag: types.TypeBoolean, Data: []byte{b}}, nil
	default:
		jsonData, err := json.Marshal(p)
		if err != nil {
			return storage.Value{}, fmt.Errorf("failed to serialize payload: %v", err)
		}
		return storage.Value{TypeTag: types.TypeText, Data: jsonData}, nil
	}
}

// cleanupExpiredDevices periodically drops device tracking entries that have
// been idle longer than deviceTTL. The device ID itself lives in localStorage
// on the client, so a device "expiring" here just frees server-side state.
func (pb *PWABridge) cleanupExpiredDevices() {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for range ticker.C {
		pb.mutex.Lock()
		cutoff := time.Now().Add(-pb.deviceTTL)
		for id, device := range pb.devices {
			if device.LastActive.Before(cutoff) {
				delete(pb.devices, id)
			}
		}
		pb.mutex.Unlock()
	}
}

// GetActiveDevices returns the number of tracked devices (for testing).
func (pb *PWABridge) GetActiveDevices() int {
	pb.mutex.RLock()
	defer pb.mutex.RUnlock()
	return len(pb.devices)
}

// GetDeviceInfo returns tracking info for a device (for testing).
func (pb *PWABridge) GetDeviceInfo(deviceID string) (*DeviceConnection, bool) {
	pb.mutex.RLock()
	defer pb.mutex.RUnlock()
	device, exists := pb.devices[deviceID]
	return device, exists
}

// IsMCPPath reports whether requestPath is the MCP endpoint.
func (pb *PWABridge) IsMCPPath(requestPath string) bool {
	return requestPath == "mcp" || strings.HasPrefix(requestPath, "mcp/")
}

// HandleClientSSERequest opens an SSE stream for the given device ID. The
// optional ?token=<token> query parameter, if supplied, is resolved against
// world.apps.<app>.tokens.<token> the same way HandleMCPRequest does — a
// valid token promotes the device's connection to the user subtree
// (post-auth) and an invalid one returns 401. Pre-auth connections receive
// events under devices.<deviceId>.*; post-auth connections receive events
// under users.<identity>.*.
func (pb *PWABridge) HandleClientSSERequest(w http.ResponseWriter, r *http.Request, deviceID string) {
	if pb.sseManager == nil {
		http.Error(w, "SSE manager not available", http.StatusServiceUnavailable)
		return
	}

	if deviceID == "" {
		http.Error(w, "Missing device ID", http.StatusBadRequest)
		return
	}

	pb.touchDevice(deviceID, r)

	if token := r.URL.Query().Get("token"); token != "" {
		identity, err := pb.resolveAuth(deviceID, token)
		if err != nil {
			http.Error(w, err.Error(), http.StatusUnauthorized)
			return
		}
		if err := pb.PromoteDeviceToUser(deviceID, identity); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	streamName := fmt.Sprintf("device-%s", deviceID)
	pb.sseManager.HandleSSERequest(w, r, streamName)
}

// PromoteDeviceToUser binds an open device connection to a user identity.
// Subsequent SSE events for the device's connection route under
// world.apps.<app>.users.<identity>.*. The device must already be tracked
// (typically via touchDevice or a prior MCP request). Identity must be
// non-empty.
func (pb *PWABridge) PromoteDeviceToUser(deviceID, identity string) error {
	if identity == "" {
		return fmt.Errorf("identity must be non-empty")
	}

	pb.mutex.Lock()
	defer pb.mutex.Unlock()

	device, exists := pb.devices[deviceID]
	if !exists {
		return fmt.Errorf("device not found: %s", deviceID)
	}

	device.Identity = identity
	device.LastActive = time.Now()
	return nil
}

// IsClientSSEPath detects PWA client SSE paths of the form
// "client-sse/<deviceId>".
func (pb *PWABridge) IsClientSSEPath(requestPath string) (bool, string) {
	if strings.HasPrefix(requestPath, "client-sse/") && len(requestPath) > 11 {
		return true, requestPath[11:]
	}
	return false, ""
}

// SubscribeClientToPath records a subscription for a device.
func (pb *PWABridge) SubscribeClientToPath(deviceID string, dataPath string) error {
	pb.mutex.Lock()
	defer pb.mutex.Unlock()

	device, exists := pb.devices[deviceID]
	if !exists {
		return fmt.Errorf("device not found: %s", deviceID)
	}

	device.Subscriptions[dataPath] = true
	device.LastActive = time.Now()
	return nil
}

// UnsubscribeClientFromPath removes a subscription for a device.
func (pb *PWABridge) UnsubscribeClientFromPath(deviceID string, dataPath string) error {
	pb.mutex.Lock()
	defer pb.mutex.Unlock()

	device, exists := pb.devices[deviceID]
	if !exists {
		return fmt.Errorf("device not found: %s", deviceID)
	}

	delete(device.Subscriptions, dataPath)
	device.LastActive = time.Now()
	return nil
}

// setupDataChangeWatcher configures the watcher that drives outbound SSE
// events when subscribed paths change.
func (pb *PWABridge) setupDataChangeWatcher() {
	if pb.watcher == nil {
		return
	}

	watcherCode := `
		// PWA Client Data Change Watcher
		println("PWA client data change detected")
	`

	clientPath := fmt.Sprintf("world.apps.%s.devices", pb.appName)
	if err := pb.watcher.RegisterWatcher("pwa-client-data-change", []string{clientPath}, watcherCode); err != nil {
		fmt.Printf("Warning: Could not register PWA client data change watcher: %v\n", err)
	}

	go pb.periodicDataChangeCheck()
}

func (pb *PWABridge) periodicDataChangeCheck() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		pb.processSubscribedDataChanges()
	}
}

func (pb *PWABridge) processSubscribedDataChanges() {
	if pb.sseManager == nil {
		return
	}

	pb.mutex.RLock()
	devices := make(map[string]*DeviceConnection)
	for id, device := range pb.devices {
		snapshot := &DeviceConnection{
			DeviceID:      device.DeviceID,
			LastActive:    device.LastActive,
			UserAgent:     device.UserAgent,
			RemoteAddr:    device.RemoteAddr,
			Subscriptions: make(map[string]bool),
		}
		for path, subscribed := range device.Subscriptions {
			snapshot.Subscriptions[path] = subscribed
		}
		devices[id] = snapshot
	}
	pb.mutex.RUnlock()

	currentTime := time.Now()

	for deviceID, device := range devices {
		for dataPath := range device.Subscriptions {
			pathParts := strings.Split(dataPath, ".")

			value, err := pb.tree.Read(pathParts)
			if err != nil {
				continue
			}

			var eventData string
			switch value.TypeTag {
			case types.TypeText:
				eventData = string(value.Data)
			case types.TypeNumber:
				eventData = string(value.Data)
			case types.TypeBoolean:
				if len(value.Data) > 0 && value.Data[0] != 0 {
					eventData = "true"
				} else {
					eventData = "false"
				}
			default:
				eventData = string(value.Data)
			}

			streamName := fmt.Sprintf("device-%s", deviceID)
			event := &SSEEvent{
				StreamName: streamName,
				Data:       fmt.Sprintf(`{"path":"%s","value":%q,"timestamp":%d}`, dataPath, eventData, currentTime.UnixMilli()),
				ID:         fmt.Sprintf("%d-%s", currentTime.UnixNano(), deviceID),
				EventType:  "data_change",
				Timestamp:  currentTime,
			}

			pb.sseManager.broadcastEvent(event)
		}
	}
}

// HandleSubscriptionRequest handles subscribe/unsubscribe requests. The
// device must identify itself with X-AmorphDB-Device.
func (pb *PWABridge) HandleSubscriptionRequest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	deviceID := r.Header.Get(HeaderDevice)
	if deviceID == "" {
		http.Error(w, "Missing "+HeaderDevice+" header", http.StatusBadRequest)
		return
	}

	var reqBody struct {
		Action string `json:"action"`
		Path   string `json:"path"`
	}

	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	pb.touchDevice(deviceID, r)

	var err error
	switch reqBody.Action {
	case "subscribe":
		err = pb.SubscribeClientToPath(deviceID, reqBody.Path)
	case "unsubscribe":
		err = pb.UnsubscribeClientFromPath(deviceID, reqBody.Path)
	default:
		http.Error(w, "Invalid action", http.StatusBadRequest)
		return
	}

	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	response := map[string]interface{}{
		"status":    "success",
		"action":    reqBody.Action,
		"path":      reqBody.Path,
		"device_id": deviceID,
		"timestamp": time.Now().UnixMilli(),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// IsSubscriptionPath reports whether requestPath is the subscription endpoint.
func (pb *PWABridge) IsSubscriptionPath(requestPath string) bool {
	return requestPath == "subscribe" || strings.HasPrefix(requestPath, "subscribe/")
}

// GetClientSubscriptions returns the subscriptions registered for a device
// (for testing).
func (pb *PWABridge) GetClientSubscriptions(deviceID string) (map[string]bool, bool) {
	pb.mutex.RLock()
	defer pb.mutex.RUnlock()

	device, exists := pb.devices[deviceID]
	if !exists {
		return nil, false
	}

	subscriptions := make(map[string]bool)
	for path, subscribed := range device.Subscriptions {
		subscriptions[path] = subscribed
	}
	return subscriptions, true
}

// ForceProcessDataChanges triggers the SSE outbound pump synchronously
// (for testing).
func (pb *PWABridge) ForceProcessDataChanges() {
	pb.processSubscribedDataChanges()
}
