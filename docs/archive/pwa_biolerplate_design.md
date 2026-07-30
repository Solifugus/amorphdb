# AmorphDB PWA Boilerplate Specification

## Purpose

This document specifies the standard PWA (Progressive Web Application) shell
for building applications on AmorphDB. The boilerplate provides six primitives
that the developer builds on. Everything else is application logic.

The boilerplate is vanilla HTML, CSS, and JavaScript. No frameworks, no build
tools, no npm. Assets are deployed to the mesh via `deploy_pwa` and served by
`amorphd` directly.

---

## Six Primitives

### 1. Transport

The client communicates with the server through two channels:

**Server → Client:** An SSE (Server-Sent Events) connection. The client opens
a persistent HTTP GET to an SSE endpoint. The server pushes events through this
connection whenever data the client cares about changes. If the connection
drops, the browser reconnects automatically and sends a `Last-Event-ID` header
so the server knows where to resume.

**Client → Server:** Standard HTTP POST requests. Each request carries an MCP
message as its body. The server processes it and may respond directly (for
synchronous operations) or push results back via SSE (for async operations).

**REST endpoints** exist only for third-party integrations that require them
(OAuth callbacks, webhooks, payment processor notifications). They are escape
hatches, not the primary protocol.

**Timing:** Every message — inbound and outbound — carries a timestamp of when
it was sent. The reception handler records when it was received. This provides
per-message round-trip timing without any special infrastructure. The developer
can surface this in a debug panel or log it for performance analysis.

```javascript
var app = {
    sse: null,         // SSE connection
    pending: {},       // outgoing requests awaiting response, keyed by request ID
    log: [],           // message log (see Primitive 6)
}

// Establish SSE connection
app.connect = function(url) {
    app.sse = new EventSource(url)
    app.sse.onmessage = function(event) {
        var message = JSON.parse(event.data)
        message._received = Date.now()
        app.reception(message)
    }
}
```

---

### 2. Message Router

All incoming messages flow through `app.reception`. All outgoing messages flow
through `app.request`. These are the only two entry/exit points for
communication.

**`app.reception(message)`**

Handles every incoming message — both SSE events and HTTP POST responses.

1. Log the message (see Primitive 6)
2. If the message has a `request_id` and there is a pending entry for that ID,
   attach the pending data and remove it from `app.pending`
3. Route the message to a handler based on its `type` field

```javascript
app.reception = function(message) {
    // Log
    app.logMessage("in", message)

    // Reattach pending data if this is a response
    if (message.request_id && app.pending[message.request_id]) {
        message._context = app.pending[message.request_id]
        delete app.pending[message.request_id]
    }

    // Route to handler
    if (app.handlers[message.type]) {
        app.handlers[message.type](message)
    }
}
```

**`app.request(type, payload, context)`**

Sends a message to the server. Optionally stores context data that will be
reattached when the response arrives.

1. Generate a request ID
2. If context is provided, store it in `app.pending` under the ID
3. Record send timestamp
4. POST the message to the server

```javascript
app.request = function(type, payload, context) {
    var id = app.generateId()
    var message = {
        type: type,
        request_id: id,
        payload: payload,
        _sent: Date.now()
    }

    if (context) {
        app.pending[id] = context
    }

    app.logMessage("out", message)

    fetch("/mcp", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(message)
    })
}
```

The request ID and context mechanism is optional. Many messages — particularly
data subscriptions and SSE push events — have no corresponding request. The
router handles both correlated (request/response) and uncorrelated (push) 
messages through the same path.

---

### 3. Local Data Cache

The client maintains a local mirror of the mesh data it cares about, keyed by
path. This is a plain JavaScript object.

```javascript
app.data = {}

// Update a path in the local cache
app.setData = function(path, value) {
    app.data[path] = value
    app.onDataChanged(path, value)
}

// Read from local cache
app.getData = function(path) {
    return app.data[path]
}
```

**Population:** When the server pushes data via SSE (because the client has
subscribed to a path), `app.reception` routes it to a handler that calls
`app.setData`. The cache fills progressively as subscriptions are established.

**Subscriptions:** By default, only the currently focused section receives live
SSE updates. When the user navigates away, the cached data remains but no new
updates are pushed. When the user navigates back, the subscription resumes and
the cache is refreshed.

The developer can override this per section:

```javascript
// In the application's section definitions
app.sections["dashboard"].keepLive = true    // always receive updates
app.sections["chat"].keepLive = true         // real-time needed
app.sections["settings"].keepLive = false    // default: only when viewed
```

**Offline resilience:** Because the cache persists in memory, temporary network
disconnections don't blank the screen. The user sees the last known data. When
the SSE connection re-establishes, updates flow again and the cache refreshes.

---

### 4. Data-Driven Renderer

The UI is driven by data. What's visible, what's focused, what's highlighted —
all determined by values in the data cache. The renderer watches for data
changes and updates the DOM accordingly.

```javascript
// When any data changes, check if it affects what's on screen
app.onDataChanged = function(path, value) {
    // Find all DOM elements bound to this path
    var elements = document.querySelectorAll('[data-bind="' + path + '"]')
    for (var i = 0; i < elements.length; i++) {
        app.renderElement(elements[i], value)
    }

    // Check for structural changes (focus, highlight, visibility)
    if (path === app.focusPath) {
        app.renderSection(value)
    }
}
```

**Binding:** HTML elements declare which data path they display:

```html
<span data-bind="my.person.name"></span>
<span data-bind="my.person.age"></span>
```

When `app.setData("my.person.name", "Matthew")` is called (from an SSE event),
the bound element updates automatically.

**Focus:** The currently displayed section is determined by a focus path. 
Changing the focus re-renders the main content area. This is how navigation
works — it's just a data change.

**Highlight:** The AI helper (or the application itself) can highlight elements
by setting a highlight path in the data. The renderer checks for highlight
state and applies a CSS class (pulsing border, background flash, etc.).

**Rendering is the developer's responsibility.** The boilerplate provides the
binding and change-detection infrastructure. The developer writes the actual
rendering functions for their application's content types. A publishing platform
renders articles differently than a banking app renders transactions.

---

### 5. Server-Side Watcher Bridge

The client never writes directly to shared mesh data. All client writes go to
the server, where application-defined watchers mediate access to the mesh.

This is the security boundary. The watcher bridge enforces:

- **Application data boundary:** The web server's AmorphDB agent has
  permissions on a specific subtree of the mesh (e.g., `world.knowledgefoyer.*`).
  The watchers only read and write within this boundary.
- **Web user authorization:** The application defines what each web user can
  do. The watchers check authorization before writing to the mesh.

The web server runs under a configurable AmorphDB agent identity — this can
be the node agent or a dedicated service agent (e.g., `world.agent.knowledgefoyer_web`).
This agent's mesh permissions define the maximum possible data access for the
entire web application.

```
Security layers:

    Web user action
      → Application auth check (what can this user do?)
        → Watcher bridge (within application data boundary)
          → Mesh agent permissions (maximum access ceiling)
            → AmorphDB storage
```

Web users are NOT AmorphDB agents. They authenticate through the application's
own auth system (local accounts, OAuth, etc.). The boilerplate provides a
pluggable auth module that the developer configures — it does not prescribe
a specific authentication method.

**Guest access:** A guest is a web user with limited authorization. The
developer decides what guests can do — read public content, submit a
registration form, post anonymous feedback. The watchers enforce these limits
the same way they enforce any other authorization rule.

**AI helper:** The AI operates through the web user's client instance with
the web user's permissions. It sends MCP messages through `app.request`, which
flow through the same watcher bridge as any user action. The AI cannot access
anything the user cannot access. Every AI action is visible on screen and
logged.

---

### 6. Deep Link Mapping

The URL maps to a focus path. When someone loads a URL, the client reads the
path and sets the initial focus accordingly.

```
https://knowledgefoyer.com/articles/philosophy-of-mind
                          ↓
app.focus = "world.knowledgefoyer.articles.philosophy_of_mind"
```

**For authenticated users:** The SPA shell loads, the focus is set from the
URL, subscriptions fire for that section, data arrives, the section renders.

**For guests:** Same flow but with guest-level authorization. The watchers
only push data the guest is allowed to see.

**For crawlers:** The server detects crawler user agents and returns
pre-rendered HTML with the content for that path, including Open Graph meta
tags for social media previews. Real users get the SPA shell. This is handled
server-side — the boilerplate's client JavaScript is not involved.

**URL updates:** When the user navigates within the SPA (changing focus), the
URL updates via `history.pushState` so the browser's back/forward buttons work
and the URL is always shareable.

```javascript
app.navigate = function(path) {
    app.focus = path
    history.pushState({ path: path }, "", app.pathToUrl(path))
    app.subscribe(path)
    app.render()
}

// Handle browser back/forward
window.onpopstate = function(event) {
    if (event.state && event.state.path) {
        app.navigate(event.state.path)
    }
}
```

---

## Logging

`app.reception` logs every message it processes. The log has two retention
tiers:

**Debug log:** Everything, kept for a short window (configurable, default 5
minutes). For diagnosing issues in real time.

**Action log:** User actions and AI/MCP actions, kept longer (configurable,
default 24 hours). For reviewing what happened, especially what the AI did.

Entries are purged automatically when they exceed their retention window. The
log lives in the client's memory — it is not persisted to the mesh.

```javascript
app.logMessage = function(direction, message) {
    var entry = {
        direction: direction,     // "in" or "out"
        type: message.type,
        timestamp: Date.now(),
        message: message
    }
    app.log.push(entry)
    app.purgeLog()
}
```

---

## File Structure

A minimal PWA built on this boilerplate:

```
build/
├── index.html              # SPA shell
├── manifest.json           # PWA manifest
├── sw.js                   # Service worker (caching, offline)
├── app.js                  # Boilerplate (six primitives)
├── app.css                 # Base styles
├── handlers.js             # Application message handlers
├── sections.js             # Application section definitions and renderers
└── auth.js                 # Application authentication module
```

Deployed with:

```mbl
my.computer.network.web.deploy_pwa("knowledgefoyer.com", "/home/matt/knowledgefoyer/build")
```

The developer writes `handlers.js`, `sections.js`, and `auth.js`. The
boilerplate provides `app.js`. The HTML, CSS, manifest, and service worker
are standard PWA boilerplate with minimal opinions.

---

## What the Boilerplate Does NOT Prescribe

- **Authentication method.** Local accounts, OAuth, SAML — developer's choice.
- **Visual design.** No CSS framework, no component library. Developer's choice.
- **Data model.** The application defines its own mesh subtree structure.
- **Section layout.** The developer writes renderers for their content types.
- **Business logic.** The developer writes watchers for their application rules.
- **AI behavior.** The developer decides what MCP tools the AI can invoke.

The boilerplate is infrastructure, not a framework. It handles communication,
caching, binding, and security boundaries. Everything else is the developer's
domain.
