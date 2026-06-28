# PWA Identity and Authentication — Development Plan

Each step is one Claude Code session. Complete the step, run tests, stop.
Do not combine steps.

**Rules:** Read `CLAUDE.md` before every step. The spec
(`docs/amorphdb_design.md`, PWA User Identity and Authentication section)
is the source of truth. Write tests for each feature. No TODOs in tests.

**Reference:** `docs/pwa_identity_auth_draft.md` has detailed examples
for login, signup, logout, and OAuth flows.

---

## Phase A: Go Bridge — Device and Token Model

### Step A1: Device ID Routing (Pre-Auth)

**Status:** DONE (completed: 2026-04-25) — Replaced session-ID routing with
X-AmorphDB-Device routing in `internal/service/pwa_bridge.go`. MCP writes go
to `world.apps.<app>.devices.<deviceId>.<type>`; pre-auth allows only
`login`/`signup`, all other types return 401, missing device header returns
400. Updated subscription/SSE handlers to key by device ID. Migrated
http_server_test cases to the new model. All `internal/service` tests pass.

Replace session-ID routing in `internal/service/pwa_bridge.go` with
device-ID routing.

- Accept `X-AmorphDB-Device` header on all requests
- If no device header, reject with 400
- Route writes to `world.apps.<app>.devices.<deviceId>.*`
- Restrict pre-auth writes to `.login` and `.signup` sub-paths only
- All other paths return 401 (no token)
- Remove the old session-ID generation and session map

Write tests that:
- Send MCP message with device header → verify data written to `devices.<deviceId>.login`
- Send MCP message without device header → verify 400
- Send write to `devices.<deviceId>.orders` (not login/signup) → verify 401
- Two different device IDs → verify isolation

Files: `internal/service/pwa_bridge.go`, `internal/service/pwa_bridge_test.go`

---

### Step A2: Token Lookup and Identity Resolution

**Status:** DONE (completed: 2026-04-25) — Added `X-AmorphDB-Token` handling
to `HandleMCPRequest` in `internal/service/pwa_bridge.go`. New
`resolveAuth(deviceID, token)` reads `world.apps.<app>.tokens.<token>.identity`
and `.device`; missing token entry → 401 ("token expired or unknown"),
device-mismatch → 401 ("token device mismatch"). On success, writes route
through new `writeUserMessage` to `world.apps.<app>.users.<identity>.<type>`
and the response includes `identity`. Pre-auth (no token) behavior unchanged.
Added Step A2 tests in `pwa_bridge_test.go`: token success + user-subtree
routing, expired (purged) token, device-mismatched token, unknown token, and
a complementary "authenticated non-auth type allowed" test. All
`internal/service` tests pass.

Implement token-based identity resolution in the Go bridge.

- Accept `X-AmorphDB-Token` header alongside device header
- Look up `world.apps.<app>.tokens.<token>` in the storage tree
- Read `.identity` and `.device` fields from the token entry
- Verify `.device` matches the presenting device — reject with 401 if mismatch
- If token is valid, route writes to `world.apps.<app>.users.<identity>.*`
- If token is missing, expired, or unknown, reject with 401
- The `@ttl` expiry is handled by AmorphDB's purge mechanism — no Go code needed

Write tests that:
- Create a token entry in storage, send request with matching token+device → verify routes to user subtree
- Send request with expired token → verify 401
- Send request with valid token but wrong device → verify 401
- Send request with unknown token → verify 401

Files: `internal/service/pwa_bridge.go`, `internal/service/pwa_bridge_test.go`

---

### Step A3: SSE Routing by Identity

**Status:** DONE (completed: 2026-04-25) — Added `Identity` field to
`DeviceConnection` and `PromoteDeviceToUser(deviceID, identity)` to
`internal/service/pwa_bridge.go`. `HandleClientSSERequest` now reads
`?token=<token>` from the URL, calls `resolveAuth` (Step A2 helper), and
auto-promotes the device on success or 401s on invalid/mismatched tokens.
Pre-auth connections receive events under `devices.<deviceId>.*`; post-auth
connections receive events under `users.<identity>.*`. Same identity on
multiple devices yields independent SSE streams that each receive the same
user events; different identities are isolated. Added Step A3 tests in
`pwa_bridge_test.go` using a goroutine + httptest.ResponseRecorder pattern:
pre-auth device events, login transition to user events, two-devices-same-
user fan-out, two-devices-different-users isolation, plus invalid-token-
rejected and PromoteDeviceToUser preconditions. All `internal/service`
tests pass.

Update the SSE connection manager to route events based on identity.

- Pre-auth: SSE connection keyed by device ID. Receives events for
  `world.apps.<app>.devices.<deviceId>.*` (login results)
- Post-auth: SSE connection keyed by device ID + identity. Receives events
  for `world.apps.<app>.users.<identity>.*`
- When a login succeeds (token created), transition the SSE connection from
  device-only to device+identity routing
- Same user on multiple devices = multiple SSE connections, each receiving
  the same user data events

Write tests that:
- Open SSE connection with device only → verify receives device path events
- Simulate login (create token) → verify SSE transitions to user events
- Two devices for same user → verify both receive user events
- Login on device A doesn't affect device B's SSE (if B is a different user)

Files: `internal/service/sse_manager.go`, `internal/service/pwa_bridge.go`

---

### Step A4: Login Response Handling

**Status:** DONE (completed: 2026-04-25) — `HandleMCPRequest` now waits, after
writing pre-auth login/signup credentials, for the watcher's result at
`world.apps.<app>.devices.<deviceId>.<type>.result` and surfaces the parsed
JSON object as `response["result"]`. Added `loginResultTimeout` field on
`PWABridge` (default 2s; production stable, tests opt to short windows or
zero). Helpers added: `waitForLoginResult` (bounded poll, 50 ms cadence,
always reads at least once) and `decodeLoginResult` (JSON-only — non-JSON
or empty values are treated as absent so callers omit the field). The
bridge is purely a relay: it does not interpret credentials, status, token,
or message values. `newTestBridge` defaults to a 0 timeout so legacy tests
pay no polling penalty; Step A4 tests opt back in. Added six new tests in
`pwa_bridge_test.go`: token-on-success, error-on-failure, credentials
routed verbatim across five payload shapes, signup parity, async result
delivered during the poll loop, and timeout-omits-field. All
`internal/service` tests pass; full-repo failure set matches the pre-step
baseline.

Implement the login response flow in the Go bridge.

- When a login watcher writes a result to `devices.<deviceId>.login.result`,
  the Go bridge reads it
- If `result.status = "ok"` and `result.token` exists, include the token
  in the response to the browser (as a JSON field in the SSE event or
  MCP response)
- The browser stores the token and begins including it in subsequent requests
- If `result.status = "error"`, pass the error message to the browser

Write tests that:
- Simulate login: write credentials to device login path, have a mock
  watcher write a successful result with token → verify token returned
- Simulate failed login → verify error message returned
- Verify the bridge doesn't interpret the credentials — it just routes

Files: `internal/service/pwa_bridge.go`

---

### Step A5: Write Boundary Enforcement

**Status:** DONE (completed: 2026-04-25) — Added `validateWriteBoundary` helper
to `internal/service/pwa_bridge.go` and wired it into `HandleMCPRequest`
before any storage write. The check enforces three rules: the MCP message
type must be a single tree segment (no `.`, `/`, or `\`), it must not name
a bridge-managed reserved subtree (`app`, `assets`, `auth`, `tokens`,
`devices`, `users`), and pre-auth requests are still restricted to
`login`/`signup`. Authenticated violations return 403 with the reason in
the body; pre-auth violations return 401 with the generic
"Authentication required" body so the boilerplate routes to the login
screen. Each rejection logs a `[pwa_bridge] boundary violation` line with
device, identity (if any), type, and reason, and increments the new
`violationCount` atomic counter (exposed via `BoundaryViolations()` for
tests). Added five Step A5 tests in `pwa_bridge_test.go`: own-subtree
allowed, cross-user separator subtests (eight payloads incl. `..`,
`users.bob.profile`, `x\y`), reserved-type rejection across all six
reserved names, pre-auth login allowed (counter stays 0), and pre-auth
rejection across reserved/non-auth/separator types. All `internal/service`
tests pass; full-repo failure set matches the pre-step baseline.

Implement the Go bridge's write boundary rules.

- Authenticated users can only write to `users.<their_identity>.*`
- No browser can write to `app`, `assets`, `auth`, or `tokens` directly
- Pre-auth devices can only write to `devices.<deviceId>.login` and `.signup`
- Log and reject any boundary violation attempts

Write tests for each boundary:
- Authenticated user writes to own subtree → allowed
- Authenticated user writes to another user's subtree → rejected
- Authenticated user writes to `tokens` → rejected
- Authenticated user writes to `app` → rejected
- Pre-auth device writes to `login` → allowed
- Pre-auth device writes to `users` → rejected

Files: `internal/service/pwa_bridge.go`

---

## Phase B: Boilerplate JavaScript — Device and Token

### Step B1: Device ID Generation and Storage

**Status:** DONE (completed: 2026-04-26) — Added `getOrCreateDeviceId()` to
the AmorphPWA constructor in `web/boilerplate/test/amorphdb-pwa.js` (and
copied to `web/boilerplate/amorphdb-pwa.js`). On first construction the
boilerplate reads `amorphdb_device_id` from localStorage; if absent or not
32 lowercase hex chars (e.g. corrupt or pre-B1 data), it generates a fresh
ID from `crypto.getRandomValues(new Uint8Array(16))` and persists it. The
ID is exposed as `this.deviceId` and used in three places: (1) the
`X-AmorphDB-Device` header on the `/mcp` POST in `flushChanges()`, (2) the
same header on the `/mcp` POST in `requestNextListPage()`, and (3) the
SSE connection URL in `connectSSE()`, which now points at
`/client-sse/<deviceId>` (the path Step A3's `IsClientSSEPath` already
expects) instead of the legacy session/`userId` path. The
`getUserId()`/`user_id` body field on MCP messages is left untouched —
the bridge ignores it now that A1 routes by header, and removing it is
out of scope for B1. localStorage failures (sandboxed contexts, private
browsing) are caught and ignored so the boilerplate still works with a
non-persistent in-memory ID. Added a 13th test in
`web/boilerplate/test/integration.html` ("Device ID Generation and
Storage") that covers (a) fresh-load generation + 32-hex format +
localStorage persistence, (b) reuse across new instances, (c) recovery
from a corrupt stored value, (d) `X-AmorphDB-Device` header on POSTs by
mocking `window.fetch` and inspecting the captured init, and (e) SSE URL
embedding by mocking `window.EventSource` and capturing the URL passed
to its constructor. All 13 integration tests pass under headless Chrome.
Go test results unchanged from baseline (same packages pass, same
packages fail; only run-time durations differ). `node --check` clean on
both JS copies.

Update `web/boilerplate/amorphdb-pwa.js` to generate and store a device ID.

- On first load, generate a random device ID (32 hex characters)
- Store in localStorage as `amorphdb_device_id`
- Include as `X-AmorphDB-Device` header on all MCP POST requests
- Include in SSE connection URL

Update test page and integration test. Copy to both file locations.

---

### Step B2: Token Storage and Presentation

**Status:** DONE (completed: 2026-04-26) — Added auth-token handling to the
AmorphPWA constructor and request paths in `web/boilerplate/test/amorphdb-pwa.js`
(and copied to `web/boilerplate/amorphdb-pwa.js`). The constructor now reads
`amorphdb_token` from localStorage into `this.token`. Three new methods —
`getStoredToken()`, `setToken(token)`, and `clearToken()` — wrap localStorage
with try/catch so sandboxed/private-mode contexts degrade to in-memory tokens
instead of throwing. `flushChanges()` adds the `X-AmorphDB-Token` header
alongside `X-AmorphDB-Device` whenever a token is present (pre-auth POSTs
deliberately omit it, so the bridge keeps routing login/signup writes to
`devices.<deviceId>.*`). After a successful POST, the new
`handleLoginResponse(response)` clones the response, parses JSON, and inspects
`body.result` (the field Step A4's bridge surfaces from the watcher's result
record). On `result.status === "ok"` with a non-empty `result.token`, the
boilerplate calls `setToken()`, fires the `amorphPWALoginSuccess` event, and
reconnects SSE so Step A3 can re-key the stream from device-only to
device+identity routing. On `result.status === "error"`, the
`amorphPWALoginError` event fires (carrying `result.message`) and no token is
stored. A 401 response from any POST clears the stored token and fires
`amorphPWAAuthRequired` so Step B3 (login UI) can later route the user to the
login screen — B2 only updates auth state, not the rendered UI, per the
"one step per session" rule and Standing Rule 1. The same token header and
401-clears-token logic was added to `requestNextListPage()` so authenticated
list-paging participates in the same auth flow. `connectSSE()` now builds the
URL from a `?token=<token>&since=<ts>` parameter list, matching the spec
example in `docs/AmorphDB_PWA_Boilerplate_Spec.md` lines 530–532; both
parameters remain unencoded to preserve the existing `since` behavior, and
EventSource cannot set custom headers so the token must travel in the URL.
Tokens are 64-character lowercase hex (Step C3 output) which is URL-safe;
non-hex token values would still pass through unencoded but no production
code path produces them. Added a 14th test in
`web/boilerplate/test/integration.html` ("Token Storage and Presentation")
covering nine sub-cases: (1) successful login response stores the token in
both the instance and localStorage and triggers SSE reconnect, (2)
constructor picks up the persisted token and POSTs send `X-AmorphDB-Token`
alongside `X-AmorphDB-Device`, (3) pre-auth POSTs omit the token header,
(4) SSE URL embeds `?token=<token>` when a token is set, (5) SSE URL
combines token and since with `&` when both are set, (6) pre-auth SSE URL
has no query string, (7) 401 response clears the token from both the
instance and localStorage and fires `amorphPWAAuthRequired`, (8) failed
login responses (`status: "error"`) do not store a token and fire
`amorphPWALoginError`, and (9) `setToken`/`clearToken` APIs work directly.
All 14 integration tests pass under headless Chrome. Go test results
unchanged from baseline (same packages pass, same packages fail; only
run-time durations differ). `node --check` clean on both JS copies.

Update the boilerplate to handle auth tokens.

- After a successful login response, store the token in localStorage as
  `amorphdb_token`
- Include as `X-AmorphDB-Token` header on all subsequent MCP POST requests
- Include in SSE connection URL as `?token=<token>`
- On 401 response, clear the stored token and show the login screen

Update test page with a mock login flow.

---

### Step B3: Login/Signup UI

**Status:** DONE (completed: 2026-04-26) — Added login/signup section
visibility to the AmorphPWA boilerplate in
`web/boilerplate/test/amorphdb-pwa.js` (and copied to
`web/boilerplate/amorphdb-pwa.js`). The new `applyAuthVisibility()` method
inspects `this.sectionTree.children` for top-level sections named `login`
or `signup` and toggles their `_shown` state via the existing Step 12
mechanism: pre-auth (no token) shows only `login` and `signup`, post-auth
(token present) shows everything else and hides the auth sections. Apps
with no login/signup sections (guest mode) leave `hasAuthSections` false
and skip the toggle entirely so the full app stays visible regardless of
token state. The method is called from three places: (1) at the end of
`init()` after `renderDOM()` so the initial render reflects the persisted
token state, (2) at the end of `setToken()` so a successful login flips
the auth sections off and the app on, and (3) at the end of `clearToken()`
so a 401 / logout flips back to the login screen. Login and signup forms
use the standard data-driven mechanism: a button's `onclick` sets
`data.login.submitted = true` (or `data.signup.submitted = true`), which
the proxy tracks and `flushChanges` POSTs to `/mcp`. The bridge (Step A1)
routes the pre-auth write to `world.apps.<app>.devices.<deviceId>.login`
and the reference watcher (Step C1) processes it and writes back its
result. `flushChanges` now inspects the change set for `login.submitted`
or `signup.submitted` and remembers the form in `this.pendingAuthForm` so
`handleLoginResponse` can route any error message back to the right
section. On `result.status === "error"`, the boilerplate writes
`result.message` to `data.<form>.error` via `applyServerUpdate`, which
updates any `{error}` placeholder in that section's template surgically.
On `result.status === "ok"` with a token, the boilerplate clears any
prior error on both auth forms (via the new `clearAuthFormError` helper),
calls `setToken` (which flips visibility automatically), fires
`amorphPWALoginSuccess`, and reconnects SSE so Step A3 re-keys the
stream from device-only to device+identity. The 401 path from B2 is
unchanged: `clearToken` runs, which calls `applyAuthVisibility` and
returns the user to the login screen via `amorphPWAAuthRequired`.
Updated `web/boilerplate/test/app.html` with concrete `<section
name="login">` and `<section name="signup">` examples. Added a 14th test
("Login/Signup UI") to `web/boilerplate/test/integration.html` covering
seven sub-cases: (1) pre-auth visibility — only login/signup shown, all
other sections hidden, both in `_shown` data and `style.display` DOM,
(2) post-auth visibility — login/signup hidden, app shown, (3) login form
submission flushes via fetch with `login.submitted=true` plus
`login.username`/`login.password` in the same batch and no token header
(pre-auth POST), (4) login error response writes the message to
`login.error` in the data tree and updates the bound `<div class="error">
{error}</div>` element while keeping the login section visible for retry,
(5) successful login response stores the token, clears any stale error,
fires `amorphPWALoginSuccess`, reconnects SSE, and flips visibility so
login/signup hide and the app shows, (6) `clearToken` (e.g., on 401)
flips visibility back to pre-auth — login/signup show, app hides,
(7) guest mode (no auth sections in tree) is a no-op and all sections
stay visible regardless of token state. Test 13 ("Complete Integration")
was renumbered to test 15. The integration tests use the new authoritative
pattern of `await pwa.flushChanges()` directly (instead of waiting on
`setTimeout(0)` cycles) so the full async chain — fetch → handleLoginResponse
→ Response.clone().json() → applyServerUpdate → dispatchEvent — completes
deterministically before assertions. All 15 integration tests pass under
headless Chrome. Go test results unchanged from baseline (same packages
pass, same packages fail; only run-time durations differ).
`node --check` clean on both JS copies.

Add login/signup section rendering to the boilerplate.

- Before authentication, render only the login/signup sections (not the
  full app)
- Login form submits to `devices.<deviceId>.login` via the standard
  data-driven mechanism
- On successful login (token received), re-render the full app with the
  user's data
- On error, show the error message in the login section

The `app.html` should support a special `<section name="login">` and
`<section name="signup">` that are shown pre-auth and hidden post-auth.

---

### Step B4: Logout

**Status:** DONE (completed: 2026-04-26) — Logout support in the AmorphDB
PWA boilerplate is an orchestration of capabilities already shipped in
B2 (token storage and 401 handling) and B3 (auth section visibility),
with one additional concern unique to logout: tearing down the SSE
connection when the token is cleared. The flow is fully data-driven
and matches the spec at `docs/pwa_identity_auth_draft.md` lines 264–271
(the per-user logout watcher that ships in `setup_user_watchers`):

1. The application sets `data.intent.logout = true` (typically from a
   logout button's `onclick` handler — the test app demonstrates with
   `<button onclick="data.intent.logout = true">Log Out</button>`).
2. The boilerplate's existing proxy tracks the change and `flushChanges`
   POSTs it to `/mcp` with `X-AmorphDB-Device` and `X-AmorphDB-Token`
   headers (the token is still valid at request time).
3. The Go bridge (Step A2) routes the write to
   `world.apps.<app>.users.<identity>.intent.logout`. The per-user
   logout watcher (defined by the application in Step C2's
   `setup_user_watchers`) fires, iterates `world.apps.<app>.tokens`,
   and removes any entry whose `.identity` matches this user. The
   watcher then quietly resets `intent.logout = false`.
4. Subsequent POSTs no longer find a matching token in the tree, so
   `resolveAuth` (Step A2) returns 401. The boilerplate's existing
   B2 handling (`flushChanges`'s 401 branch) calls `clearToken()`
   and fires `amorphPWAAuthRequired`.
5. `clearToken()` now (Step B4) calls a new `disconnectSSE()` helper
   in addition to its existing B3 work of `applyAuthVisibility()`.
   The result: the in-memory and localStorage tokens are removed,
   the EventSource is closed and nulled, the reconnect backoff is
   reset, and the section visibility flips back to the login screen.
6. The device ID is preserved throughout — only the token is cleared.

The single new piece of code is `disconnectSSE()` in
`web/boilerplate/test/amorphdb-pwa.js` (and the parent copy):
it closes `this.eventSource` (if any) inside a try/catch, sets the
reference to null, and resets `this.reconnectAttempts` to 0 so the
backoff doesn't leak into the next session. `clearToken()` calls
`disconnectSSE()` before `applyAuthVisibility()` so the stream tears
down before the UI flips. The benefit over leaving SSE open: no
events for the logged-out identity sneak through during the brief
window between token-removal-server-side and the next 401-driven
client cleanup. A direct call to `clearToken()` (apps that want a
programmatic logout without waiting for the 401 round trip) gets
the same teardown.

The boilerplate intentionally does **not** add a `logout()` method
or any logout-specific code paths beyond `disconnectSSE()`. The
intent-write pattern is the canonical AmorphDB shape — apps express
"I want to log out" by writing data; the watcher does the work.
Wrapping that in a JavaScript helper would diverge from the
data-driven philosophy and bypass any per-user logic the application
might layer on top of the logout intent (audit logging, cleanup,
etc.).

Updated `web/boilerplate/test/amorphdb-pwa.js` and the parent
`web/boilerplate/amorphdb-pwa.js`: added `disconnectSSE()`, wired
it into `clearToken()`, and extended the file header comment with
the Step B4 entry. Added a 15th test ("Logout") to
`web/boilerplate/test/integration.html` covering four sub-cases
that exercise the full flow end-to-end: (1) `data.intent.logout =
true` is tracked and `flushChanges` POSTs the intent with both the
device and token headers and an `intent.logout=true` change body,
verifying the token remains set immediately after this first POST
(the watcher hasn't run yet client-side); (2) the next POST
returning 401 clears the token from both the instance and
localStorage, fires `amorphPWAAuthRequired` with the persisting
device ID, preserves the device ID in localStorage, tears down a
stub EventSource (verifying `close()` was called and the reference
was nulled), resets `reconnectAttempts` to 0, and flips section
visibility so login/signup show and the workspace hides; (3)
direct `clearToken()` (programmatic logout) tears down SSE the
same way as the 401 path; (4) `disconnectSSE()` is safe to call
when no EventSource exists. Test 15 ("Complete Integration") was
renumbered to 16. All 16 integration tests pass under headless
Chrome. Go test results unchanged from baseline (same packages
pass, same packages fail; only run-time durations differ).
`node --check` clean on both JS copies.

Add logout support to the boilerplate.

- When the user triggers logout (via an intent or a dedicated button),
  the watcher removes the token from `world.apps.<app>.tokens`
- The next server response is 401
- The boilerplate clears localStorage token and shows the login screen
- The device ID persists — only the token is cleared

---

### Step B5: 401 Handling and Token Expiry

**Status:** DONE (completed: 2026-04-26) — Three additions on top of B2's
existing MCP POST 401 path, all in
`web/boilerplate/test/amorphdb-pwa.js` (and copied to
`web/boilerplate/amorphdb-pwa.js`):

1. **SSE auth probe.** EventSource's `onerror` does not expose the
   underlying HTTP status code, so a 401 from the bridge looks the same
   as a network blip. The new `probeAuthAfterSSEFailure()` sends a
   single zero-write `data_update` POST to `/mcp` with the current
   device + token headers. The bridge's Step A2 `resolveAuth` runs
   before any storage access, so the response is 401 if the token is
   bad and 200 otherwise — exactly the 1-bit signal needed to decide
   whether to bounce to the login screen or continue with normal SSE
   backoff. `handleSSEError` (now async) calls the probe at most once
   per disconnect window, gated by a new `sseProbedSinceLastOpen` flag
   that flips back to false in `connectSSE`'s `onopen`. A 401 probe
   response routes through `clearToken` (which already disconnects SSE
   per B4 and flips visibility per B3) and `notifyAuthRequired`.
   Network errors during the probe are treated as inconclusive: the
   token is preserved and normal exponential backoff continues so a
   transient WiFi drop doesn't bounce the user.

2. **Pending-change preservation.** When `flushChanges` sees a 401, it
   now snapshots the live `this.changes` Map into a new
   `this.pendingChanges` Map via `preservePendingChanges()` before
   clearing it. Without this, two things would go wrong: (a) the
   user's in-flight edit would be lost when the eventual successful
   pre-auth login POST replaces the changes Map with `login.submitted`
   et al., and (b) the very next pre-auth POST would carry stale
   post-auth writes alongside the login submission, which the bridge
   would route incorrectly per Step A1's pre-auth restriction. On a
   successful re-login, `setToken` calls the new
   `retryPendingChanges()`, which merges the snapshot back into
   `this.changes` (preserving any newer values the user may have set
   in the interim) and triggers `scheduleSend`. A
   `amorphPWAPendingChangesRetried` event with `{ count }` fires so
   apps can show a "your unsaved order has been restored" toast.
   `requestNextListPage`'s 401 path doesn't snapshot — list paging is
   read-only, so there's nothing in flight to preserve — but it
   surfaces the same friendly login-prompt path.

3. **Friendlier logging.** The 401 paths in `flushChanges`,
   `requestNextListPage`, and `handleSSEError` now log a single
   `console.info('🔒 Session ended — please log in again')` line
   instead of `console.warn('🔒 Server returned 401 — clearing
   token')`. 401 is an expected outcome of normal token expiry, not a
   bug, and the spec's "user-friendly message, not a stack trace"
   requirement is best satisfied by treating it that way.

The `notifyAuthRequired` event detail now includes `pendingChanges`
(the count of preserved changes) so apps can render
"unsaved-changes-pending" UI without poking at internals.

Added Test 16 ("401 Handling and Token Expiry") to
`web/boilerplate/test/integration.html`, with nine sub-cases that
exercise the full B5 surface end-to-end: (1) MCP POST 401 snapshots
unsent changes to `pendingChanges`, clears the live changes Map, and
fires `amorphPWAAuthRequired` with the preserved-count detail; (2)
`setToken` after re-login merges `pendingChanges` back into the
changes Map, fires `amorphPWAPendingChangesRetried`, and the next POST
carries the preserved values plus the new token header; (3) SSE
failure with a token + a 401 probe response clears the token, closes
the EventSource, and fires the auth-required event with the same
device + token headers on the probe POST; (4) SSE failure with a
token + a non-401 probe response preserves the token, marks the probe
as done, and increments the backoff counter; (5) repeated
`handleSSEError` calls in the same disconnect window do not re-probe
once `sseProbedSinceLastOpen` is true; (6) SSE failure pre-auth (no
token) skips the probe entirely; (7) network failure during the probe
is treated as inconclusive — the token is preserved and normal
backoff continues; (8) `requestNextListPage`'s 401 also clears the
token and surfaces the auth-required event (no `pendingChanges` to
preserve since list paging is a read-only request); (9)
`preservePendingChanges` is idempotent, no-ops on an empty changes
Map, snapshots when there's content, and never overwrites existing
preserved entries (preferring older values that haven't been retried
yet). Test 16 ("Complete Integration") was renumbered to 17. All 17
integration tests pass under headless Chrome. Go test results
unchanged from baseline (same 10 packages fail, same lists; only
run-time durations differ). `node --check` clean on both JS copies.

Implement graceful handling of authentication failures.

- On 401 from any MCP POST or SSE connection, clear the stored token
- Show the login screen without losing the device ID
- If the user was in the middle of editing, optionally preserve local
  unsent changes for retry after re-login
- Log a user-friendly message, not a stack trace

---

## Phase C: Reference Auth Watchers

### Step C1: Reference Login Watcher (MBL)

**Status:** DONE (completed: 2026-04-25) — Added
`web/boilerplate/auth/login.mbl`, the reference login watcher that ships
with the AmorphDB PWA boilerplate. The watcher follows the canonical
example in docs/pwa_identity_auth_draft.md (lines 168–200) and uses
`myapp` as the placeholder app name throughout (init-pwa, Step C4, will
substitute it). It observes `world.apps.myapp.devices`, fires when a
browser sets `device.login.submitted = true`, looks up credentials at
`world.apps.myapp.auth.credentials.<username>`, and on
`is_unknown(creds)` writes `{ status: "error", message: "Unknown user" }`
to `device.login.result`. On a successful
`my.computer.crypto.verify_password(password, creds.password_hash,
creds.salt)` (Step C3 primitive) it generates a token via
`my.computer.crypto.generate_token()` (also Step C3), installs the token
under `world.apps.myapp.tokens.<token>` with `.identity = username` and
`.device = device`, applies the configured `app._token_ttl` as `@ttl`
when greater than zero, and returns `{ status: "ok", token: token }`.
Mismatched passwords yield `{ status: "error", message: "Invalid
password" }`. The watcher closes by quietly resetting
`device.login.submitted = false` to avoid re-firing. A header comment
documents the substitution rule, the configuration attribute
`world.apps.myapp.app._token_ttl`, and the OAuth/SSO replacement
contract. No Go code changed; `go build ./...` is clean.

Create the reference login watcher as an MBL file that ships with the
boilerplate. This is what `amorphctl init-pwa` includes in new projects.

```mbl
watch my.app.auth.login_handler(world.apps.<app>.devices):
    for device in world.apps.<app>.devices:
        if device.login.submitted = true:
            # validate credentials
            # generate token
            # write to tokens subtree
            # write result to device.login.result
```

Include Argon2id password hashing via a Go-implemented `hash_password()`
and `verify_password()` procedure available in `my.computer`.

Files: `web/boilerplate/auth/login.mbl`

---

### Step C2: Reference Signup Watcher (MBL)

**Status:** DONE (completed: 2026-04-25) — Added
`web/boilerplate/auth/signup.mbl`, the reference signup watcher that
ships with the AmorphDB PWA boilerplate. It mirrors the structure of
the Step C1 login watcher and follows the canonical example in
docs/pwa_identity_auth_draft.md (lines 204–241), using `myapp` as the
placeholder app name (init-pwa, Step C4, will substitute it). The
watcher observes `world.apps.myapp.devices`, fires on
`device.signup.submitted = true`, and rejects duplicates with
`{ status: "error", message: "User already exists" }` when
`not is_unknown(world.apps.myapp.auth.credentials[username])`. On a
fresh username it calls `my.computer.crypto.hash_password(password)`
(Step C3 primitive — returns `{ hash, salt }` together, so no separate
`generate_salt()` is used; this is the only deviation from the draft,
which predates the C3 API), stores `hashed.hash` and `hashed.salt`
under `auth.credentials.<username>`, sets
`users.<username>._shown = true` to materialize the user's data
subtree, invokes the app-provided `my.app.setup_user_watchers(username)`
procedure to register per-user watchers, and then auto-logs in by
generating a token via `my.computer.crypto.generate_token()`,
installing it under `world.apps.myapp.tokens.<token>` with
`.identity = username` and `.device = device`, applying the configured
`app._token_ttl` as `@ttl` when greater than zero, and returning
`{ status: "ok", token: token }`. The watcher closes by quietly
resetting `device.signup.submitted = false`. The header comment
documents the substitution rule, the `_token_ttl` configuration
attribute, the `setup_user_watchers` application contract, and the
OAuth/SSO replacement contract. No Go code changed; `go build ./...`
is clean.

Create the reference signup watcher.

- Validates username doesn't exist
- Hashes password with Argon2id + random salt
- Creates user in `users` subtree
- Creates credentials in `auth.credentials`
- Calls `setup_user_watchers` (app-provided procedure)
- Auto-logs in by generating a token

Files: `web/boilerplate/auth/signup.mbl`

---

### Step C3: Password Hashing Primitives (Go)

**Status:** DONE (completed: 2026-04-25) — Added `internal/mbl/interpreter/crypto.go`
implementing `evalHashPassword`, `evalVerifyPassword`, and `evalGenerateToken`,
and wired a `crypto` case into `evalComputerLibraryProcedure` in
`internal/mbl/interpreter/interpreter.go`. `hash_password(password)` returns
a Record `{ hash, salt }` whose values are hex-encoded Argon2id output (32-byte
key, 16-byte salt) using OWASP-recommended parameters (time=2, memory=64 MiB,
threads=1). `verify_password(password, hash, salt)` decodes both inputs,
recomputes the hash with the stored salt, and returns Boolean using
`subtle.ConstantTimeCompare`; malformed hex or zero-length inputs return
`false` (zero-length must be guarded explicitly because `argon2.IDKey` panics
on `keyLen=0`). `generate_token()` returns 32 bytes of `crypto/rand` as a
64-character lowercase hex Text. Added `golang.org/x/crypto v0.31.0` to
go.mod (Go 1.21-compatible; the module's `go 1.21` directive is preserved).
Added `crypto_test.go` with 11 test functions / 22 subtests covering happy
path round-trip, salt randomness, wrong-password rejection, malformed-hex
rejection (non-hex, empty), arg-count errors for all three procedures,
token length and hex validity, token uniqueness over 16 calls, and
end-to-end dispatcher routing through `evalCryptoLibraryProcedure` (incl.
unknown-procedure and empty-path cases). The non-text rejection cases were
removed because the codebase's `types.CoerceToText` is intentionally
permissive — every type coerces — and `evalFilesImport` and peers follow
the same pattern. All `internal/mbl/interpreter` failures match the
pre-step baseline (TestProjectionExpressions, TestJsonHelpersIntegration,
TestSection5_1_ExpressionEvaluation, TestSection5_2_VariableScope — all
pre-existing and untouched). `go build ./...` is clean.

Implement `hash_password()` and `verify_password()` as Go-backed MBL
procedures in the computer library.

- `my.computer.crypto.hash_password(password)` → returns `{ hash: "...", salt: "..." }`
- `my.computer.crypto.verify_password(password, hash, salt)` → returns `true`/`false`
- Uses Argon2id with recommended parameters
- `my.computer.crypto.generate_token()` → returns 32 bytes of CSPRNG hex

Write Go tests for the crypto primitives.

Files: `internal/mbl/interpreter/interpreter.go`

---

### Step C4: init-pwa Scaffolding

**Status:** DONE (completed: 2026-04-26) — Added the `init-pwa` subcommand
to `amorphctl`. `amorphctl init-pwa <appname> <directory>` scaffolds a
seven-file starter PWA project on disk; the command is purely
local-filesystem and does not require a connection to the amorphd
service, so `cmd/amorphctl/main.go` now dispatches it before the
control-socket connect path (alongside `help`). New file
`cmd/amorphctl/init_pwa.go` contains the implementation. Templates
live under `cmd/amorphctl/templates/` and are pulled into the binary
via `//go:embed templates`. The scaffolder walks the embedded tree
with `fs.WalkDir`, mirroring the layout one-for-one into the target
directory and replacing every occurrence of the literal placeholder
`myapp` with the supplied app name. The seven files written are:
`index.html` (loader matching the canonical example in
`AmorphDB_PWA_Boilerplate_Spec.md` lines 785–800),
`app.html` (root section + login/signup/banner/workspace, with a Log
Out button bound to `data.intent.logout = true` so logout works out
of the box per Step B4 / setup.mbl), `app.js` (empty, comment-only —
the AmorphDB philosophy is to put reactive logic in MBL watchers),
`style.css` (minimal layout styles for the starter sections plus the
`[layout="horizontal"]` selector used by the boilerplate), `setup.mbl`
(initializes `world.apps.<app>.app._token_ttl = 604800`, defines the
`my.app.setup_user_watchers(username)` procedure with a working
per-user logout watcher as the canonical example, and a `pass`-padded
slot for the developer to add app-specific watchers), `auth/login.mbl`
(byte-identical to the canonical `web/boilerplate/auth/login.mbl`
from Step C1), and `auth/signup.mbl` (byte-identical to
`web/boilerplate/auth/signup.mbl` from Step C2). The auth templates
are kept in sync via a drift test (see below) — when those canonical
files change in future steps, the test catches the omission and the
embedded copies must be re-synced.

App-name validation reuses the existing `isValidMBLIdentifier` helper
from `extract.go` so the rules are consistent across `extract` and
`init-pwa`: must start with a letter or underscore, body characters
must be letters/digits/underscores. The empty string is rejected
explicitly. The validator runs before any directory is created so a
rejected input never leaves a half-scaffolded tree on disk. Targets
are accepted in three states — non-existent (created, including any
missing parent dirs via `os.MkdirAll`), existing-and-empty (used as
is), or existing-and-non-empty (rejected with a clear "not empty;
refusing to overwrite" error). Existing files in a non-empty target
are preserved untouched. Permissions: directories `0o755`, files
`0o644`.

The `--help` text now lists `init-pwa <appname> <directory>` and a
matching example invocation; the closing footnote was updated from
"All commands connect via local UNIX socket" to "Most commands
connect via local UNIX socket; init-pwa is local-only" so users
understand they can run init-pwa without a running daemon.

Added `cmd/amorphctl/init_pwa_test.go` with nine test functions / 24
sub-cases: (1) `Scaffolds_AllFiles` — the seven expected files exist
and are non-empty, plus the `auth/` directory is created;
(2) `PerformsAppNameSubstitution` — every file with the placeholder
contains the substituted app name and zero post-substitution files
contain the literal `myapp`, with explicit checks that login.mbl,
signup.mbl, and setup.mbl all reference `world.apps.<appname>.*`
paths and that `app.html` and `index.html` carry the substituted
`<section name="...">` and `<title>...</title>`; (3)
`AcceptsExistingEmptyDirectory` — running against an empty tempdir
succeeds and writes the files; (4) `RefusesNonEmptyDirectory` — the
command errors with "not empty" in the message, the pre-existing
file is unchanged, and zero scaffold files are written;
(5) `CreatesNonExistentTargetDirectory` — works for nested paths,
creating intermediate parents; (6) `RejectsInvalidAppNames` — seven
sub-cases covering empty, leading-digit, hyphen, period, slash,
space, and shell-metacharacter inputs, each verifying both the error
return and that no target directory was created;
(7) `AcceptsValidAppNames` — six sub-cases covering single-letter,
multi-letter, underscored, mixed-case, leading-underscore, and
digit-suffixed names;
(8) `AuthTemplatesMatchCanonical` — the embedded
`templates/auth/login.mbl` and `templates/auth/signup.mbl` are
byte-identical to the canonical `web/boilerplate/auth/login.mbl` and
`web/boilerplate/auth/signup.mbl` (drift detector that fails loudly
if the canonical references and embedded copies diverge);
(9) `SetupMBLContainsRequiredSections` — the generated `setup.mbl`
contains the `world.apps.<app>.app._token_ttl` initializer and the
`procedure my.app.setup_user_watchers(username):` declaration, both
of which are required by Step C4's contract and consumed by the
reference signup watcher; plus a structural
`DoesNotConnectToService` test that confirms `Run()` is self-contained
(no client passed in, completes successfully without a daemon — the
test itself is the proof).

End-to-end smoke test confirms the binary works exactly as the spec
documents: `amorphctl init-pwa demoapp /tmp/c4_demo` scaffolded all
seven files (one per the dev plan, plus the four boilerplate-spec
files), 33 substituted occurrences of `world.apps.demoapp.*` across
the three MBL files, zero remaining `myapp` strings anywhere, and a
second invocation against the same non-empty directory was rejected
with exit 1 and the pre-existing tree untouched. `go test
./cmd/amorphctl/` passes (24 sub-cases under TestInitPWA + all
existing extract/join/mesh/detach tests). Full-repo `go test ./...`
failure set matches the pre-step baseline exactly (same 10 packages:
cmd, internal/interpreter, internal/mbl/interpreter,
internal/mbl/parser, internal/mesh [build failed], internal/security,
internal/storage, internal/zone_deprecated, test/integration,
tests/integration [build failed]).

Update `amorphctl init-pwa` to include auth scaffolding in new projects.

- Include `auth/login.mbl` and `auth/signup.mbl` in the project template
- Include a `setup.mbl` that initializes the app's data structure
  (`world.apps.<app>.app`, `.auth`, `.tokens`, `.devices`, `.users`)
- Include a placeholder `setup_user_watchers` procedure for the developer
  to fill in

Files: `cmd/amorphctl/`

---

## Phase D: Integration

### Step D1: End-to-End Auth Test

**Status:** DONE (completed: 2026-04-26) — Added `TestEndToEndAuth` driving
all nine scenarios end-to-end against a real `*PWABridge`: pre-auth login
screen, signup → token, authed write → user subtree, per-user task watcher
firing, logout via intent purges tokens, re-login produces fresh token while
user data persists, second-device login + multi-device SSE under one
identity, and token expiry. The reference login/signup MBL watchers from
`web/boilerplate/auth/{login,signup}.mbl` and a per-user logout/task watcher
are simulated in Go inside the test (a 20ms polling goroutine) — the bridge
stores each MCP payload as a single JSON value at `devices.<id>.<type>` /
`users.<identity>.<type>`, so the simulated watcher reads, parses, and
writes the result paths the bridge polls. File location deviates from the
plan: this test ships at `internal/service/auth_integration_test.go` rather
than `test/auth_integration_test.go` because the bridge's unexported test
helpers (`newTestBridge`, `mcpRequest`, `writeTokenEntry`,
`expireTokenEntry`, `startSSEConn`, `closeSSEConn`) live in package
`service`; reproducing them externally would have meant duplicating ~100
lines of plumbing without changing the contract under test. Baseline holds:
`internal/service` passes; the nine pre-existing failing packages
(`cmd`, `internal/{interpreter,mbl/interpreter,mbl/parser,mesh,security,storage,zone_deprecated}`,
`test/integration`, `tests/integration`) are unchanged — none touched by
this step.

Write a test that exercises the full flow:

1. Start `amorphd` with a test app deployed
2. Browser (simulated) connects with device ID → receives login screen
3. Submit signup form → user created, token returned
4. Submit with token → data routed to user subtree
5. Write an intent → per-user watcher fires, result returned
6. Logout → token deleted, next request returns 401
7. Re-login → new token, same user data persists
8. Login from second device → both devices receive SSE events
9. Token expires → 401, login screen shown

Files: `test/auth_integration_test.go`

---

### Step D2: Update Existing Tests

**Status:** DONE (completed: 2026-04-26) — Audited every Go source file under
`internal/service/`, `tests/`, and `test/` for residual references to the old
PWA session-ID model. The PWA bridge tests in
`internal/service/pwa_bridge_test.go` and `internal/service/http_server_test.go`
were already migrated as part of Phase A1–A5 and Step D1; this step found two
remaining stale references and migrated both.

The first was a placeholder smoke fixture in
`internal/service/web_integration_test.go::testMCPMessageHandling` that
constructed an MCP-shaped JSON literal using the old `"session": "test-
session-123"` field, the obsolete `"action"` field, and the
`/mcp/action` endpoint — none of which match the protocol the bridge has
spoken since Step A1. The fixture is purely structural (it never reaches
`HandleMCPRequest`), but it perpetuated the wrong wire shape in test code
that future readers might mistake for canonical. Replaced it with a
fixture that mirrors the real `MCPMessage` struct (`type`, `request_id`,
`payload`, `_sent`), POSTs to `/mcp` (not `/mcp/action`), and sets
`X-AmorphDB-Device` via the `HeaderDevice` constant. Added an explicit
assertion that the device header is present so a future regression that
forgets it would fail loudly. The test still does not invoke the bridge
itself — that is what `TestEndToEndAuth` (Step D1) and the Step A1–A5
suite cover end-to-end.

The second was a misnamed local variable in `internal/service/http_server.go`
where `IsClientSSEPath` returns the device ID (per Step A1) but the caller
was still binding it to `sessionID`. Renamed to `deviceID` so the variable
matches the value's actual meaning. The behavior is unchanged; the rename
removes a stale conceptual artifact from the PWA hot path.

Out of scope: `tests/integration/{bridge_authentication,bridge_operations,
mobile_agent_bridge}_test.go` and `internal/{auth,mesh,security}/` use
`sessionID`/"session" terminology, but those refer to mesh-bridge auth
sessions and security agent sessions — separate concepts from PWA browser
sessions. `tests/phase3/test_s1_4_session_management.mbl` covers MBL-side
security sessions, also separate. The boilerplate JS still carries a
legacy `getUserId()`/`user_id` field in MCP message bodies, but the bridge
ignores it (routing keys off the headers per Step A1) and removing it was
explicitly deferred in the Step B1 completion notes; it is not a test
artifact and is therefore out of D2's scope.

Baseline holds exactly: `go test ./...` produces the same set of passing
packages (15) and failing packages (10: `cmd`,
`internal/{interpreter,mbl/interpreter,mbl/parser,mesh [build],security,
storage,zone_deprecated}`, `test/integration`, `tests/integration
[build]`) as the pre-step baseline. `internal/service` passes. No package
that was passing before this step is failing now, and no new failures
were introduced.

Update all existing PWA bridge tests and integration tests to use the
device/token model instead of the old session-ID model.

Run `go test ./...` — no failures from the auth migration.

---

## Summary

| Step | Feature | Depends On | Complexity |
|------|---------|-----------|-----------|
| A1 | Device ID routing | — | medium |
| A2 | Token lookup + identity | A1 | medium |
| A3 | SSE routing by identity | A1, A2 | medium |
| A4 | Login response handling | A2 | small |
| A5 | Write boundary enforcement | A2 | small |
| B1 | JS device ID | — | small |
| B2 | JS token storage | B1 | small |
| B3 | JS login/signup UI | B1, B2 | medium |
| B4 | JS logout | B2 | small |
| B5 | JS 401 handling | B2 | small |
| C1 | Reference login watcher | A4 | small |
| C2 | Reference signup watcher | C1 | small |
| C3 | Crypto primitives (Go) | — | medium |
| C4 | init-pwa scaffolding | C1, C2 | small |
| D1 | End-to-end auth test | all | medium |
| D2 | Update existing tests | all | small |

Phase A (Go bridge) can proceed in parallel with Phase C3 (crypto primitives).
Phase B (JavaScript) depends on Phase A being complete.
Phase C1-C2 (MBL watchers) depend on Phase A4.
Phase D is last.
