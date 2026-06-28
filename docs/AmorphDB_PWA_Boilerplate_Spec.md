# AmorphDB PWA Boilerplate Specification

## Overview

The AmorphDB PWA boilerplate lets developers build web applications by writing
HTML with `<section>` tags and `{field}` placeholders. The boilerplate
JavaScript handles everything else — data binding, server sync, layout, and
real-time updates.

The developer writes:
- `app.html` — application structure and templates
- `style.css` — styling
- `app.js` — custom behavior only (optional)

No framework. No build step. No REST APIs. No state management.

---

## How It Works

1. The developer defines the UI as nested `<section>` tags in `app.html`
2. The boilerplate parses the section hierarchy and builds the app tree
3. `{fieldname}` placeholders in templates create the data tree automatically
4. A JavaScript Proxy tracks all data changes and batches them to the server
5. Server-side changes arrive via SSE and update individual DOM elements directly
6. Custom watchers handle special logic when needed (optional)

The developer never writes networking code, data synchronization logic, or
manual DOM updates for data-bound fields. Data in, data out, automatic rendering.

---

## Application Structure

### app.html

The application is defined as nested `<section>` tags with four system
attributes:

| Attribute | Values | Default | Meaning |
|-----------|--------|---------|---------|
| `name` | any identifier | (required) | Path name in the data tree |
| `layout` | `"vertical"`, `"horizontal"` | `"vertical"` | How child sections are arranged |
| `type` | `"list"` | (auto) | Marks this section as a repeating list |
| `show_max` | number | (all) | For lists, how many items visible before scrolling |
| `load_max` | number | 200 | For lists, how many items loaded into client data at a time |
| `token_ttl` | number (seconds) | 604800 | On root section only: login token lifetime. 0 = no timeout. |

Every section can contain:
- HTML content with `{fieldname}` placeholders (the template)
- Child `<section>` tags (subsections)
- Both (template content plus subsections)

Any section can contain subsections. A header can have horizontally arranged
subsections. A sidebar item can expand into nested detail sections. The
structure is recursive with no arbitrary limits.

### Example Application

```html
<section name="app" token_ttl="604800">

    <section name="banner" layout="horizontal">
        <h1>{app_name}</h1>
        <span class="user">{user_name}</span>
    </section>

    <section name="workspace" layout="horizontal">

        <section name="sidebar">
            <h3>Menu</h3>
            <section name="menu_items" type="list">
                <a onclick="data.navigation.current = '{target}'">{label}</a>
            </section>
        </section>

        <section name="main">
            <h2>{page_title}</h2>

            <section name="order_form">
                <label>Product</label>
                <input value="{product}">
                <label>Quantity</label>
                <input type="number" value="{quantity}">
                <button onclick="data.workspace.main.order_form.submit = 'down'">
                    Submit Order
                </button>
            </section>

            <section name="orders" type="list" show_max="20" load_max="100">
                <div class="order-row">
                    <span>{product}</span>
                    <span>{quantity}</span>
                    <span class="money">{amount}</span>
                    <span class="status">{status}</span>
                </div>
            </section>
        </section>

    </section>

    <section name="status_bar" layout="horizontal">
        <span>{status_message}</span>
        <span>v{version}</span>
    </section>

</section>
```

This single file defines:
- The entire UI layout (nested sections with horizontal/vertical flow)
- All data fields (extracted from `{fieldname}` placeholders)
- Input bindings (standard HTML event handlers writing to `data.*`)
- Actions (button clicks setting toggle values)

### What the Boilerplate Does With app.html

On load, `amorphdb-pwa.js` parses `app.html` and:

1. **Builds the section tree** — each `<section>` becomes a node with its
   `name`, `layout`, `type`, and `max` attributes
2. **Extracts field names** — every `{fieldname}` in templates is registered
   as a data field at that section's path
3. **Creates the data tree** — a JavaScript object mirroring the section
   hierarchy, with properties for each extracted field
4. **Wraps the data tree in a Proxy** — all assignments are automatically
   tracked for server sync
5. **Renders the initial DOM** — sections are rendered recursively with
   layout rules applied. Each `{fieldname}` placeholder is replaced with a
   DOM element carrying a `data-bind` attribute set to the field's full path
6. **Connects SSE** — begins receiving server updates
7. **Binds updates** — incoming data changes are applied surgically to
   individual DOM elements via their `data-bind` path, not by re-rendering
   sections

---

## Data Tree

The data tree is generated automatically from `app.html`. For the example
above, the data tree would be:

```javascript
{
    app_name: "",
    user_name: "",
    workspace: {
        sidebar: {
            menu_items: []      // type="list" → array
        },
        main: {
            page_title: "",
            order_form: {
                product: "",
                quantity: 0,
                submit: false    // button toggle
            },
            orders: []           // type="list" → array
        }
    },
    status_bar: {
        status_message: "",
        version: ""
    }
}
```

Fields at a section level (referenced in `{fieldname}` placeholders) become
properties of that section's data node. List sections become arrays.

The developer accesses this through the proxied `data` object:

```javascript
data.workspace.main.page_title = "My Orders"
data.workspace.main.order_form.product = "Widget"
```

All assignments are tracked automatically and batched to the server.

---

## Field References

Within a `{fieldname}` placeholder, bare names refer to data at the current
section level. Dot paths reach into other parts of the tree:

```html
<!-- Bare name — local to this section -->
<span>{product}</span>

<!-- Dot path — reaches up to another section -->
<span>Welcome, {banner.user_name}</span>
```

In `onclick` and other event handlers, the full path from root is used:

```javascript
data.workspace.main.order_form.submit = 'down'
```

---

## Rendering

### Initial Render

On first load, the boilerplate renders each section recursively. Template
content is parsed, and each `{fieldname}` placeholder is replaced with a
DOM element carrying a `data-bind` attribute set to the full data path:

Template:
```html
<section name="orders" type="list" show_max="20" load_max="100">
    <div class="order-row">
        <span>{product}</span>
        <span class="money">{amount}</span>
    </div>
</section>
```

Rendered DOM (for list item at index 3):
```html
<div class="order-row">
    <span data-bind="workspace.main.orders.3.product">Widget</span>
    <span class="money" data-bind="workspace.main.orders.3.amount">$24.99</span>
</div>
```

The `data-bind` attribute is the key to surgical updates. Every bound element
in the DOM can be found by its data path.

### Sections

Each section renders its template content, then renders its children in the
specified layout direction. Vertical is default — children stack top to bottom.
Horizontal arranges children side by side.

When children overflow the available space, the section scrolls in the layout
direction.

### Lists

A section with `type="list"` repeats its template for each item in the
corresponding data array. The `{fieldname}` placeholders within a list item
refer to fields of that item, and the rendered `data-bind` includes the index:

```html
<section name="orders" type="list" show_max="20" load_max="100">
    <div class="order-row">
        <span>{product}</span>    <!-- data-bind="...orders.N.product" -->
        <span>{amount}</span>     <!-- data-bind="...orders.N.amount" -->
    </div>
</section>
```

`show_max` limits how many items are visible on screen. If the list has more
items, the section scrolls to reveal them.

`load_max` limits how many items are loaded into the client's data tree at a
time. For large lists (thousands or more), loading everything into the browser
would consume excessive memory and slow the initial connection. Instead, the
client maintains a sliding window of loaded items.

#### List Windowing

The client tracks a window position for each list — `_from` and `_through` —
representing which range of items are currently loaded:

```
Server:  [0 ..................................... 99,999]

Loaded:           [200 ............. 399]
                    ↑                 ↑
                  _from           _through

Visible:               [280 .. 299]
                        (scroll position + show_max)
```

When the user scrolls near the edge of the loaded window, the boilerplate:

1. Requests the next batch from the server (the server sends it via SSE)
2. Appends the new items to the loaded window
3. Optionally evicts items from the far end to keep memory bounded

The server uses its existing slice notation to serve the requested range.
The user experiences smooth scrolling — new items load before they scroll
into the empty region.

`_from`, `_through`, and total list count are available as system data
attributes on the list section, allowing the UI to show indicators like
"showing 200-399 of 12,847".

### Records

A section that is not a list and has `{fieldname}` placeholders renders as a
single record — one set of fields displayed once. This is the default behavior
and requires no `type` attribute.

### Shown/Hidden

Any section can be shown or hidden by setting `_shown` in the data:

```javascript
data.workspace.sidebar._shown = false    // hide sidebar
data.workspace.sidebar._shown = true     // show sidebar
```

The `_shown` property is available on every section automatically.

---

## Data Binding and DOM Updates

### Surgical Updates

When a data value changes, the boilerplate updates only the affected DOM
elements — not the entire section. The `data-bind` attribute maps each element
to its data path:

```javascript
function updateField(path, value) {
    const elements = document.querySelectorAll(`[data-bind="${path}"]`)
    for (const el of elements) {
        if (el.tagName === 'INPUT' || el.tagName === 'SELECT') {
            el.value = value
        } else if (el.tagName === 'INPUT' && el.type === 'checkbox') {
            el.checked = value
        } else {
            el.textContent = value
        }
    }
}
```

This means:
- No re-rendering of sections for single field changes
- Input focus is preserved (no elements are destroyed and recreated)
- Scroll position is preserved
- Animations are not interrupted
- Multiple elements bound to the same path all update

### Update Strategy

| Change | DOM Action |
|--------|-----------|
| Field value changed | `querySelector` by `data-bind`, update content directly |
| List item added (within window) | Clone list item template, set `data-bind` paths with new index, append |
| List item removed | Remove that item's DOM elements, reindex remaining `data-bind` paths |
| List replaced entirely | Re-render the list section (initial load or full refresh only) |
| List window shifted | Append new items at scroll edge, optionally evict items from far end |
| Section `_shown` toggled | Show/hide the section container element |

The only time a full section re-render occurs is when an entire list is replaced
at once (initial load or complete data refresh). Individual field updates,
list item additions, and window shifts are always surgical.

### Multiple Bindings

The same data path can be displayed in multiple places. Both elements update
when the value changes:

```html
<!-- In the banner -->
<span data-bind="banner.user_name">Kalevo</span>

<!-- In the profile section -->
<h2 data-bind="banner.user_name">Kalevo</h2>
```

### Automatic Binding

Simple data display requires no JavaScript. The boilerplate handles the
full cycle automatically:

1. Server pushes a new value for `workspace.main.orders.3.amount` via SSE
2. Boilerplate updates `rawData` at that path
3. Boilerplate calls `updateField("workspace.main.orders.3.amount", newValue)`
4. The `<span data-bind="workspace.main.orders.3.amount">` updates its text

The developer writes zero JavaScript for this. Data changes → DOM updates.

### Input Binding

Any `<input>`, `<select>`, or `<textarea>` with a `{fieldname}` placeholder
is automatically wired for two-way binding by the boilerplate. The developer
just writes:

```html
<input value="{product}">
<select value="{priority}">
    <option value="normal">Normal</option>
    <option value="urgent">Urgent</option>
</select>
<input type="checkbox" checked="{express}">
<textarea>{notes}</textarea>
```

The boilerplate detects these during initial render and automatically:
- Sets the `data-bind` attribute to the full data path
- For text inputs and textareas: attaches a blur handler that writes to
  `data.*` only if the value actually changed
- For selects and checkboxes: attaches a change handler
- For server-side updates: updates the element's value/checked state
  directly via `data-bind`

No `onfocus`, `onblur`, `onchange`, or explicit `data-bind` attributes needed
on input elements. The placeholder `{fieldname}` is sufficient.

If the developer needs custom behavior on an input (e.g., validation on
every keystroke, formatting), they can add their own event handlers. The
automatic binding does not interfere with custom handlers.

Input elements carry `data-bind` (set by the boilerplate) so they can be
updated by server-side changes (e.g., validation corrections) without
re-rendering the form.

### Button Actions

Buttons are toggle values. Clicking sets the value to `"down"`. The server
processes the action and resets it:

```html
<button onclick="data.workspace.main.order_form.submit = 'down'">
    Submit Order
</button>
```

The boilerplate can auto-disable buttons while their value is `"down"`.

---

## Custom Watchers

For behavior beyond simple data display, the developer writes watchers in
`app.js`. These are only needed for custom logic — conditional rendering,
computed values, animations, or side effects:

```javascript
// Show a warning when balance is low
watch('workspace.main.account.balance', (value) => {
    const warning = document.getElementById('low-balance-warning')
    warning.style.display = value < 100 ? 'block' : 'none'
})

// Compute a total from line items
watch('workspace.main.order_form', (form) => {
    data.workspace.main.order_form.total = form.quantity * form.unit_price
})
```

Most applications need very few custom watchers. The automatic data binding
handles the common cases.

---

## Proxy-Based Change Tracking

The data tree is wrapped in a JavaScript Proxy. Assignment to any property
is automatically intercepted, recorded, and scheduled for server sync:

```javascript
const changes = new Map()
let sendScheduled = false

function createProxy(obj, path) {
    return new Proxy(obj, {
        get(target, key) {
            if (key.startsWith('_')) return target[key]  // system attrs
            const value = target[key]
            if (value !== null && typeof value === 'object') {
                return createProxy(value, path ? path + '.' + key : key)
            }
            return value
        },
        set(target, key, value) {
            target[key] = value
            const fullPath = path ? path + '.' + key : key
            changes.set(fullPath, value)
            scheduleSend()
            notifyWatchers(fullPath, value)
            return true
        }
    })
}

function scheduleSend() {
    if (!sendScheduled) {
        sendScheduled = true
        queueMicrotask(flushChanges)
    }
}
```

All changes within a single JavaScript execution frame are collected and
sent as one MCP message when the frame completes. A button click handler
that sets three values sends one message, not three.

### Server Updates Bypass Tracking

When SSE events arrive from the server, the local cache is updated via
`rawData` directly — not through the proxy — so changes don't echo back
to the server. DOM elements are updated surgically via `data-bind`:

```javascript
function applyServerUpdate(path, value) {
    setNestedValue(rawData, path, value)
    updateField(path, value)
    notifyWatchers(path, value)
}
```

---

## SSE Connection

The client connects to a device-specific SSE endpoint on page load. The
device ID is generated by the boilerplate on first visit and stored in
localStorage permanently. It identifies the browser, not the person.

```javascript
const deviceId = localStorage.getItem('amorphdb_device_id') || generateDeviceId()
localStorage.setItem('amorphdb_device_id', deviceId)

function connectSSE() {
    const token = localStorage.getItem('amorphdb_token') || ''
    const headers = { 'X-AmorphDB-Device': deviceId }
    if (token) headers['X-AmorphDB-Token'] = token

    const url = '/client-sse/' + deviceId +
                (token ? '?token=' + token : '') +
                (lastUpdateTime ? '&since=' + lastUpdateTime : '')
    const eventSource = new EventSource(url)

    eventSource.onmessage = function(event) {
        const msg = JSON.parse(event.data)
        lastUpdateTime = msg.timestamp
        localStorage.setItem('last_update', lastUpdateTime)
        for (const [path, value] of Object.entries(msg.changes)) {
            applyServerUpdate(path, value)
        }
    }

    eventSource.onerror = function() {
        eventSource.close()
        setTimeout(connectSSE, 1000)  // reconnect with backoff
    }
}
```

### Pre-Authentication

Before login, the Go bridge accepts the device ID and routes writes only to
`world.apps.<app>.devices.<deviceId>.login` and `.signup`. The SSE connection
receives events for the device path (login results, error messages).

### Post-Authentication

After login, the boilerplate stores the token in localStorage and includes
it in subsequent requests. The Go bridge resolves the user identity from
the token and routes writes to `world.apps.<app>.users.<identity>.*`. The
SSE connection receives events for that user's data.

### Initial State

On first authenticated connection, the server sends the full current state
of the user's data as the first SSE event. This populates the data tree and
triggers the initial render.

### Reconnection and Temporal Resync

The client stores the timestamp of the last received update in localStorage.
On reconnect, it sends this timestamp to the server via the `?since=`
parameter. The server uses its temporal history to send only changes since
that time — no full state transfer needed.

---

## Deep Linking

Navigation is a data change. The browser URL reflects the current view path:

```javascript
// URL → data
window.addEventListener('popstate', () => {
    data.navigation.current = window.location.pathname
})

// Data → URL
watch('navigation.current', (value) => {
    if (window.location.pathname !== value) {
        history.pushState(null, '', value)
    }
})
```

A server-side watcher reacts to the navigation change and populates the
appropriate display data. The client updates automatically.

---

## User Identity

There are no conventional sessions. The boilerplate uses two identifiers:

- **Device ID** — generated on first visit, stored in localStorage permanently.
  Identifies the browser/device, not the person. The same person on two
  browsers has two device IDs. Survives page refreshes and browser restarts.
- **Auth token** — generated at login, stored in localStorage. Maps a device
  to a user identity. Expires after `token_ttl` seconds (set in `app.html`).

### Before Login

The device ID exists immediately. The Go bridge accepts it and routes writes
to `world.apps.<app>.devices.<deviceId>.login` and `.signup` only. This gives
the unauthenticated browser a place to submit credentials without access to
any user data.

### After Login

The login watcher validates credentials and creates a token at
`world.apps.<app>.tokens.<token>` with `.identity` and `.device` fields.
The boilerplate stores the token in localStorage. Subsequent requests include
both headers (`X-AmorphDB-Device` and `X-AmorphDB-Token`). The Go bridge
resolves the identity and routes to `world.apps.<app>.users.<identity>.*`.

### Multiple Devices

A user can be logged in on multiple devices simultaneously. Each device has
its own token pointing to the same identity. The Go bridge pushes SSE events
to each device independently. Token-to-device matching prevents a stolen
token from being used on a different device.

### Disconnection and Reconnection

When the user disconnects and reconnects — whether minutes or days later —
their device ID and token persist in localStorage. The boilerplate reconnects
SSE and resyncs from the last update timestamp. The user can work offline
with their local copy; changes sync when connectivity returns.

### Guest Mode (No Login Required)

Apps that don't need authentication skip tokens entirely. All data lives
under `devices.<deviceId>`. The device ID provides continuity across visits.
The Go bridge routes all traffic to the device path.

```
world.apps.myapp
    app                         # shared UI definition (parsed from app.html)
        _token_ttl: 604800      # from app.html (seconds; 0 = no timeout)
    assets                      # static files for HTTP serving
        index.html = asset(...)
        app.html = asset(...)
        ...
    devices                     # pre-auth browser connections
        d8f2a1b3...
            login/              # login form data
            signup/             # signup form data
    users                       # authenticated per-user data
        kalevo
            banner
                user_name = "Kalevo"
            workspace
                main
                    orders = [...]
        miratu
            banner
                user_name = "Miratu"
            workspace
                main
                    orders = [...]
    tokens                      # active auth tokens
        a7f3c1b9...
            identity: "kalevo"
            device: "d8f2a1b3..."
            @ttl: 604800
    auth                        # app-defined credentials
        credentials
            kalevo
                password_hash: "..."
                salt: "..."
```

The `app` subtree is the shared template — one copy for all users. The
`assets` subtree holds static files served by the Go layer. The `devices`
subtree holds pre-auth connection state. The `users` subtrees hold each
user's data instance mirroring the app structure. The `tokens` subtree maps
active auth tokens to identities and devices. The `auth` subtree holds
app-defined credential data.

Authentication is application-defined — the boilerplate ships a reference
password scheme (Argon2id), but apps can replace the login watcher with
OAuth, SSO, magic links, or any other mechanism. The token handoff and
security boundary are unchanged regardless of auth scheme.

See `amorphdb_design.md` (PWA User Identity and Authentication) for the
full specification including login/signup flows, per-user watcher
registration, supported user models, and the security boundary.

---

## Focused Updates

By default, only sections corresponding to the current view receive live SSE
updates. Sections not currently visible do not receive updates, reducing
bandwidth.

A developer can opt specific paths into persistent updates:

```javascript
keepLive('notifications')
keepLive('unread_count')
```

These paths receive SSE updates regardless of which section is visible.

---

## Security Model

Security operates at three enforcement layers:

1. **Mesh permissions (AmorphDB-native)** — the daemon runs as a mesh agent
   with `@write` on `world.apps.<app>.*`. Set once at deployment time.
2. **Go bridge enforcement (platform-native)** — the Go bridge enforces:
   - Unauthenticated devices can only write to `devices.<deviceId>.login`
     and `devices.<deviceId>.signup`
   - Authenticated users can only write to `users.<their_identity>.*`
   - No browser can write to `app`, `assets`, `auth`, or `tokens` directly
   - Token-to-device matching prevents token theft across devices
3. **Watcher-level validation (application-native)** — app watchers mediate
   between user data and shared data. A user writing to their intent path
   does not directly modify shared records — a watcher reads the intent,
   validates it, and only then writes to shared data.

Read-only fields are enforced server-side via permissions. The boilerplate
respects a `_readonly` data attribute to disable input controls in the UI,
but the server is the authority — a user cannot bypass read-only by
manipulating JavaScript.

### AI Helper

An AI assistant connects through the same interface as a human user. It writes
to the same data paths with the same permissions. No special mechanism needed.

---

## Developer Workflow

### Creating a New PWA

```bash
amorphctl init-pwa myapp ~/projects/myapp/
```

This creates a starter project:

```
~/projects/myapp/
    index.html          — loader
    app.html            — starter template with basic sections
    app.js              — empty (for custom watchers)
    style.css           — minimal default styles
```

### Project Files

The developer works with these files:

| File | Purpose | Required |
|------|---------|----------|
| `index.html` | Loader — references boilerplate and app code | yes |
| `app.html` | Application structure and templates | yes |
| `style.css` | Application styling | yes |
| `app.js` | Custom watchers and functions | optional |
| `manifest.json` | PWA manifest for installability | optional |

### index.html

The loader is minimal. The boilerplate JavaScript (`amorphdb-pwa.js`) is NOT
part of the project — it is served by `amorphd` itself at `/amorphdb/pwa.js`.
This ensures all PWAs on the instance use the same version, and upgrading
AmorphDB upgrades the boilerplate for all apps automatically.

```html
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>My App</title>
    <link rel="stylesheet" href="/style.css">
</head>
<body>
    <div id="app"></div>
    <script src="/amorphdb/pwa.js"></script>
    <script src="/app.js"></script>
</body>
</html>
```

### Boilerplate Serving

The `amorphdb-pwa.js` boilerplate and any standard components are embedded in
the `amorphd` binary using Go's `embed` package. They are served at well-known
paths:

```
/amorphdb/pwa.js                     # core boilerplate
/amorphdb/pwa.css                    # default minimal styling
/amorphdb/components/datepicker.js   # future — standard components
/amorphdb/components/datatable.js    # future — standard components
```

No separate installation, no file paths to configure. The binary carries
everything. Upgrading AmorphDB upgrades the boilerplate for all apps.

### Local Development

The developer opens `index.html` in a browser without a server. The boilerplate
detects no SSE connection and renders the app structure with placeholder data.
The developer can work on layout and styling locally, then deploy to AmorphDB
for real data.

### Deployment

Deployment is explicit — the developer deploys when ready, not on every file
save:

```bash
# Deploy project files to AmorphDB
amorphctl deploy-pwa myapp.example.com ~/projects/myapp/

# Enable the PWA
amorphctl enable-pwa myapp.example.com
```

Or from MBL:

```mbl
my.computer.network.web.deploy_pwa("myapp.example.com", "/home/dev/projects/myapp/")
my.computer.network.web.pwa["myapp.example.com"].enabled = true
my.computer.network.web.pwa["myapp.example.com"].spa_mode = true
```

### What Deployment Does

When `deploy-pwa` runs:

1. Reads all project files from the directory
2. Stores them as assets in the mesh under `world.apps.{name}.assets`
3. Parses `app.html` and builds the shared app structure under
   `world.apps.{name}.app`
4. The in-memory asset cache refreshes within one heartbeat tick

Redeploying overwrites the assets and re-parses `app.html`. User data
under `world.apps.{name}.users` is preserved — only the app definition and
static assets change.

### Data Organization in the Mesh

```
world.apps.myapp
    app                          # shared UI definition (parsed from app.html)
        _token_ttl = 604800
        banner
            _view = "..."
            _layout = "horizontal"
        workspace
            ...
    assets                       # static files for HTTP serving
        index.html = asset(...)
        app.html = asset(...)
        app.js = asset(...)
        style.css = asset(...)
    devices                      # pre-auth browser connections
        d8f2a1b3...
            login/
            signup/
    users                        # authenticated per-user data
        kalevo
            banner
                user_name = "Kalevo"
            workspace
                main
                    orders = [...]
        miratu
            banner
                user_name = "Miratu"
            workspace
                main
                    orders = [...]
    tokens                       # active auth tokens
        a7f3c1b9...
            identity = "kalevo"
            device = "d8f2a1b3..."
            @ttl = 604800
    auth                         # app-defined credentials
        credentials
            kalevo
                password_hash = "..."
                salt = "..."
```

The `app` subtree is the shared template — one copy for all users. The `assets`
subtree holds static files served by the Go layer. The `devices` subtree holds
pre-auth connection state. The `users` subtrees hold each user's data instance
mirroring the app structure. The `tokens` subtree maps auth tokens to identities
and devices. The `auth` subtree holds app-defined credentials.

### Backup and Restore

```bash
# Extract the entire app (structure + all user data)
amorphctl extract world.apps.myapp -o myapp_backup.mbl

# Extract just the app definition (no user data)
amorphctl extract world.apps.myapp.app -o myapp_structure.mbl

# Restore on another instance
amorph myapp_backup.mbl
```

---

## Future Enhancements

These are not part of version 1 but inform design decisions:

- **`_layout` expansions** — `"vertical-tab"`, `"horizontal-tab"`, `"grid"`,
  `"accordion"` for richer layout without custom CSS
- **Component support** — reusable section templates that can be referenced
  by name. AmorphDB would ship a standard library of business components
  (date pickers, data tables, charts, file uploaders), and developers could
  create their own. A component would be a section definition that can be
  used in multiple places with different data bindings:
  ```html
  <!-- Component definition (in a component library or app.html) -->
  <component name="currency-field">
      <span class="currency">{symbol}</span>
      <input type="number" value="{amount}">
  </component>

  <!-- Usage -->
  <section name="order_form">
      <currency-field bind="price"></currency-field>
      <currency-field bind="tax"></currency-field>
  </section>
  ```
- **Table rendering** — lists with column headers, sorting, and filtering
- **Chart rendering** — data visualization bound to list data
- **Offline mode** — full offline operation with conflict resolution on resync
- **Theming** — system-level theme attributes for consistent business styling
