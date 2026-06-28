# AmorphDB Web Layer Development Plan

Each step is one Claude Code session. Complete the step, run tests, stop.
Do not combine steps. Compact between sessions.

**Rules:** Read `CLAUDE.md` before every step. The spec (`docs/amorphdb_design.md`)
is the source of truth. Write tests for each feature. No TODOs in tests.

---

## Step 1: Outbound HTTP GET

Implement `my.computer.network.web.get(url)` and `my.computer.network.web.get(url, options)`.

Use Go's `net/http` package. Return a response record with `.status`, `.body`,
`.headers`, `.time`, `.duration`. On network failure, return `unknown("dns_failure")`,
`unknown("timeout")`, etc. as specified.

Write tests that:
- GET a known public URL (e.g., httpbin.org or similar) and verify response structure
- GET with timeout option and verify it's respected
- GET a non-existent domain and verify Unknown is returned

Files: `internal/mbl/interpreter/interpreter.go`

---

## Step 2: Outbound HTTP POST/PUT/PATCH/DELETE

Implement the remaining outbound methods using the same pattern as Step 1.
They share the same options and response record structure.

Write tests for POST with a body. The other methods (PUT, PATCH, DELETE) follow
the same code path — one test each confirming they send the correct HTTP method.

Files: `internal/mbl/interpreter/interpreter.go`

---

## Step 3: JSON Helpers

Verify `my.computer.network.web.parse_json(text)` and
`my.computer.network.web.to_json(node)` work correctly. These were implemented
in Phase 5 — verify they handle:

- Nested objects → Records
- Arrays → Lists
- Strings, numbers, booleans, null → MBL types
- Round-trip: `to_json(parse_json(text))` yields equivalent text

If any of these fail, fix them. Write tests for each case.

Files: `internal/mbl/interpreter/interpreter.go`

---

## Step 4: PWA Asset Deployment

Implement `my.computer.network.web.deploy_pwa(domain, directory_path)`.

This reads all files recursively from the directory, infers MIME types from
extensions, wraps each as an asset record (`.data`, `.mime_type`), and writes
them to `my.computer.network.web.pwa[domain].assets[path]` in a single
staged commit.

Also implement `my.computer.network.web.asset(data, mime_type)` for individual
asset creation.

Write tests that:
- Create a temp directory with index.html, app.js, style.css
- Deploy and verify assets are stored with correct MIME types
- Create individual assets and verify structure

Files: `internal/mbl/interpreter/interpreter.go`

---

## Step 5: In-Memory Asset Cache

Implement the Go-level asset cache in the service layer. When a PWA is enabled,
the cache loads all assets from the mesh into memory. A watcher refreshes the
cache within one heartbeat tick when assets change.

Write tests that:
- Store assets, enable PWA, verify cache is populated
- Update an asset, verify cache refreshes
- Verify cache lookup is O(1) by path

Files: `internal/service/` (new file: `asset_cache.go`)

---

## Step 6: HTTP Listener and Static Asset Serving

Implement the Go-level HTTP listener in the service layer. This is the TCP
listener that accepts connections, does TLS, and routes requests.

For this step, only implement static asset serving:
- Request comes in → check asset cache → serve if found
- SPA fallback: if `spa_mode = true` and no asset match, serve index.html
- If no match and no SPA mode, return 503

Write tests using `net/http/httptest` that:
- Serve a known asset and verify correct Content-Type
- Request a non-existent path with SPA mode on → get index.html
- Request a non-existent path with SPA mode off → get 503

Files: `internal/service/` (new file: `http_server.go`)

---

## Step 7: SSE Connection Manager

Implement the Go-level SSE connection manager. This holds open HTTP connections
and writes events when data changes.

For external SSE consumers: writing to `my.computer.network.web.sse.*` paths
publishes events to connected clients.

Write tests that:
- Open an SSE connection to a stream path
- Write a value to the SSE path
- Verify the client receives the event
- Verify one event per tick, last value wins

Files: `internal/service/` (new file: `sse_manager.go`)

---

## Step 8: Data-Driven PWA Bridge — Inbound

Implement the Go layer that translates incoming MCP messages from PWA clients
into data tree writes.

A PWA client sends an HTTP POST with an MCP message body. The Go layer
parses the MCP message and writes the action to the client's session space
in the data tree: `my.app.client.{session}.actions.*`

Write tests that:
- Send an MCP message via HTTP POST
- Verify the corresponding data tree path is written
- Verify session isolation (two clients don't see each other's actions)

Files: `internal/service/` (new file: `pwa_bridge.go`)

---

## Step 9: Data-Driven PWA Bridge — Outbound

Implement the Go layer that detects data changes in client-subscribed paths
and pushes SSE events to the connected PWA client.

When a watcher writes to the application data tree and the path is subscribed
by a client session, the Go layer sends an SSE event to that client.

Write tests that:
- Establish an SSE connection for a client session
- Write data to a subscribed path
- Verify the SSE event is received by the client
- Verify unsubscribed paths do NOT produce events

Files: `internal/service/pwa_bridge.go`

---

## Step 10: Inbound Request Queue

Implement the request queue for external HTTP traffic (APIs, webhooks, OAuth).

When an HTTP request arrives that is NOT a static asset, SSE connection, or
PWA client MCP message, the Go layer creates a request node in the data tree
at `my.computer.network.web.requests` with all specified attributes (domain,
method, path, query, headers, body, remote_ip, time).

The Go layer then waits for a response with a configurable TTL (default 5s).
If no watcher responds in time, return 503.

Write tests that:
- Send an HTTP request, verify request node is created with correct attributes
- Have a watcher respond, verify HTTP response is sent back
- Let TTL expire, verify 503

Files: `internal/service/http_server.go`

---

## Step 11: Response Helpers

Implement the MBL response helper procedures:
- `my.computer.network.web.ok(req, body)` → 200
- `my.computer.network.web.ok_json(req, node)` → 200 with JSON
- `my.computer.network.web.not_found(req)` → 404
- `my.computer.network.web.bad_request(req, msg)` → 400
- `my.computer.network.web.server_error(req, msg)` → 500
- `my.computer.network.web.redirect(req, url)` → 302

Each writes a response record to the request node that the Go layer picks up
and sends as an HTTP response.

Write tests that verify each helper produces the correct status code and body.

Files: `internal/mbl/interpreter/interpreter.go`

---

## Step 12: TLS Configuration

Implement TLS support in the HTTP listener. Read certificate and key paths
from `amorphd.toml`. Support SNI for multiple domains on one listener.

Write tests that:
- Start HTTPS listener with a self-signed cert
- Make a request and verify TLS works
- Verify SNI routing with two domains

Files: `internal/service/http_server.go`, `internal/config/`

---

## Step 13: Integration Test

Write an end-to-end test that:

1. Starts `amorphd`
2. Deploys a PWA with index.html and app.js
3. Requests index.html via HTTP → verify served from cache
4. Sends an MCP message via POST → verify data tree is written
5. Writes data to a subscribed path → verify SSE event received
6. Sends an external API request → verify request node created
7. Has a watcher respond → verify HTTP response sent back

This is the smoke test that all pieces work together.

Files: `test/web_integration_test.go`

---

## Summary

| Step | Feature | Complexity |
|------|---------|-----------|
| 1 | Outbound HTTP GET | small |
| 2 | Outbound POST/PUT/PATCH/DELETE | small |
| 3 | JSON helpers verification | small |
| 4 | PWA asset deployment | medium |
| 5 | In-memory asset cache | medium |
| 6 | HTTP listener + static serving | medium |
| 7 | SSE connection manager | medium |
| 8 | PWA bridge — inbound | medium |
| 9 | PWA bridge — outbound | medium |
| 10 | Inbound request queue | medium |
| 11 | Response helpers | small |
| 12 | TLS configuration | small |
| 13 | Integration test | medium |

Steps 1-3 can be done immediately (pure MBL interpreter work).
Steps 4-12 build the Go service layer progressively.
Step 13 ties everything together.
