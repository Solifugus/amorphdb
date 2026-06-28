# AmorphDB Computer Library

## Overview

The computer library provides every agent with access to the physical machine they are
connected through. It is mounted as a virtual path at `~.computer` (accessible as
`my.computer` from program root scope) and is not stored in the mesh — it is provided
by the local node at connection time.

The library is divided into three sub-libraries:

| Sub-library | Path                  | Purpose                                      |
|-------------|-----------------------|----------------------------------------------|
| Network     | `my.computer.network` | Outbound requests and inbound HTTPS serving  |
| Files       | `my.computer.files`   | Filesystem access and structured file import |
| System      | `my.computer.system`  | System information and process interaction   |

This document covers the **network** and **files** sub-libraries. The system
sub-library is specified separately.

All capabilities are surfaced as MBL procedures or reactive tree nodes. External
libraries and protocols are implementation detail — MBL code works at the level of
intent, not mechanism.

The local node's computer library is private by default. The node owner exposes
capabilities selectively by granting permissions on paths under `my.computer`. This
applies equally to network serving, file access, and system resources. Other agents
in the mesh access these capabilities only when explicitly granted, and only through
the normal AmorphDB permission model.

---

## Network Sub-library

### Overview

`my.computer.network` is the single point of entry for all network activity on the
local node — both reaching out to external services and receiving inbound connections.

The programmer states intent. Protocol is inferred from context and handled by the
system, with sane defaults. An `options.protocol` parameter is available as an escape
hatch for the rare case where the default is not appropriate, but most MBL code will
never use it.

All outbound and inbound communication is encrypted. Plain HTTP is not supported.

---

### Outbound Requests

#### Procedures

```
my.computer.network.get(url)
my.computer.network.get(url, options)

my.computer.network.post(url, body)
my.computer.network.post(url, body, options)

my.computer.network.put(url, body)
my.computer.network.put(url, body, options)

my.computer.network.patch(url, body)
my.computer.network.patch(url, body, options)

my.computer.network.delete(url)
my.computer.network.delete(url, options)
```

Protocol is inferred from the URL scheme. HTTPS is the default for any `https://` URL.
Future schemes (e.g. `mailto:`, `amorphdb://`) will be handled transparently as the
system evolves. The programmer does not need to select a protocol explicitly in normal
use.

#### Options

```
options.protocol          # Text: override inferred protocol (escape hatch)
options.headers           # Record: request headers (Text → Text)
options.timeout           # Number: milliseconds before giving up (default: 30000)
options.follow_redirects  # Boolean: follow redirects (default: true)
```

#### Response Node

Each call returns a response node:

| Attribute | Type | Meaning |
|---|---|---|
| `.status` | Number | HTTP status code; 0 for network-level failures |
| `.body` | Text or Unknown | Response body, or Unknown on failure |
| `.headers` | Record | Response headers (Text → Text) |
| `.time` | Time | When the response was received |
| `.duration` | Number | Round-trip time in milliseconds |

Network failures produce Unknown values in `.body`:

| Failure | Value |
|---|---|
| DNS resolution failure | `#dns_failure` |
| Connection refused | `#connection_refused` |
| Timeout | `#timeout` |
| TLS error | `#tls_error` |
| Redirect loop | `#redirect_loop` |

#### JSON Helpers

REST APIs predominantly exchange JSON. Two helper procedures are provided:

```
my.computer.network.parse_json(text)   # Text → tree node
my.computer.network.to_json(node)      # tree node → Text
```

`parse_json` maps JSON objects to records, arrays to lists, and primitives to their
AmorphDB equivalents. Returns `#invalid_json` for malformed input.

#### Examples

```
# Fetch a price and store it in the mesh
response = my.computer.network.get("https://api.example.com/prices")
if response.status = 200:
    world.market.price = my.computer.network.parse_json(response.body).price
else:
    world.market.price = #http_error

# POST with an auth header
options.headers.Authorization = "Bearer " & my.keys.api_token
options.headers.Content-Type  = "application/json"
response = my.computer.network.post(
    "https://api.example.com/orders",
    my.computer.network.to_json(my.cart),
    options
)

# Handle network failure gracefully
response = my.computer.network.get("https://partner.example.com/feed")
if is_unknown(response.body):
    my.status.partner_feed = #unavailable
else if response.status != 200:
    my.status.partner_feed = #http_error
else:
    my.data.partner_feed = my.computer.network.parse_json(response.body)
```

#### TLS Trust

The node honors its own trust store for all outbound connections. Certificates signed
by a trusted CA — whether public or a private intranet CA — are accepted
transparently. The programmer does not configure TLS trust per call. If an intranet
service uses a private CA, the node operator adds that CA to the node's trust store
once; all outbound calls then work without any code-level changes.

---

### Inbound Requests

The node listens for inbound HTTPS connections on port 443. When a request arrives,
the node records it as a request node appended to:

```
my.computer.network.requests
```

This is a single queue for all inbound requests regardless of which domain was used
to reach the node. The domain name the client used is recorded as a plain attribute
on the request node, available for filtering just like any other attribute.

#### Request Node Attributes

| Attribute | Type | Meaning |
|---|---|---|
| `.domain` | Text | Domain name from the HTTP Host header (e.g. `"api.example.com"`) |
| `.method` | Text | HTTP method: `"GET"`, `"POST"`, `"PUT"`, `"PATCH"`, `"DELETE"` |
| `.path` | Text | URL path (e.g. `"/api/orders/42"`) |
| `.query` | Record | Query string parameters (Text → Text) |
| `.headers` | Record | Request headers (Text → Text) |
| `.body` | Text | Request body (empty Text for GET / DELETE) |
| `.remote` | Text | Client IP address |
| `.@time` | Time | When the request arrived |
| `.@ttl` | Number | Milliseconds until this request expires if unprocessed |
| `.processed` | Boolean | False on arrival; set true by the handling watcher |

The default TTL is 5000 ms. If `.processed` is not set before the TTL expires, the
node automatically returns `503 Service Unavailable` to the client.

#### Responding to Requests

```
request.respond(status, body)
request.respond(status, body, headers)
```

Calling `.respond()` delivers the HTTP response to the client. The watcher then marks
the request processed to prevent a timeout 503:

```
request.respond(200, result)
request.processed = (quietly) true
```

The `(quietly)` modifier prevents assignments from triggering watchers — see the MBL watcher specification for details.

A request may only be responded to once — subsequent calls return `#already_responded`.

#### Route Handlers via Watchers

A route handler watches the request queue filtered by the attributes it cares about,
then iterates all matching unprocessed requests within the tick. This drains the queue
progressively — as many requests as the watcher can complete within 1/3 second are
handled per tick. Any remaining stay in the queue and are picked up on the next tick.
No requests are lost.

Most handlers will filter on `domain`, `method`, and `path`. A handler serving all
domains on this node simply omits the domain filter.

```
# Single-domain route handler
my.services.handlers.balance: watch append(
    my.computer.network.requests[
        domain = "api.example.com",
        method = "GET",
        path = "/api/balance"
    ]
) as new_requests:
    for req in new_requests:
        catch:
            balance = world.agent[req.query.agent].account.balance
            req.respond(200, balance)
        else unknown:
            req.respond(500, "Internal error")
        req.processed = (quietly) true

# Multi-domain handler with different behavior per domain
my.services.handlers.status: watch append(
    my.computer.network.requests[
        method = "GET",
        path = "/status"
    ]
) as new_requests:
    for req in new_requests:
        if req.domain = "api.example.com":
            req.respond(200, my.computer.network.to_json(world.services.status))
        else if req.domain = "admin.corp.internal":
            req.respond(200, my.computer.network.to_json(my.admin.status))
        else:
            req.respond(404, "Not found")
        req.processed = (quietly) true
```

#### Path Parameters

For parameterized paths, watch a prefix and parse the parameter from `.path`:

```
my.services.handlers.order: watch append(
    my.computer.network.requests[
        domain = "api.example.com",
        method = "GET",
        path starts_with "/api/orders/"
    ]
) as new_requests:
    for req in new_requests:
        order_id = substring(req.path, length("/api/orders/"), length(req.path))
        order = world.orders[order_id]
        if is_unknown(order):
            req.respond(404, "Not found")
        else:
            req.respond(200, my.computer.network.to_json(order))
        req.processed = (quietly) true
```

#### Response Helpers

Convenience procedures handle common response patterns and set
`req.processed = (quietly) true` automatically:

```
my.computer.network.ok(request, body)           # 200 with body
my.computer.network.ok_json(request, node)      # 200 with JSON-encoded node
my.computer.network.not_found(request)          # 404 Not Found
my.computer.network.bad_request(request, msg)   # 400 with message
my.computer.network.server_error(request, msg)  # 500 with message
my.computer.network.redirect(request, url)      # 302 redirect
```

---

### Server-Sent Events

SSE streams are published by writing values to paths under `my.computer.network.sse`.
Each distinct path is an independent stream. Connected clients receive an event each
time a new instance is written to the path they are subscribed to.

Events are delivered at heartbeat rhythm — at most one event per path per tick. If
multiple writes occur within a single tick, only the final value is delivered. This
is intentional: SSE carries current state to human-facing clients. The 1/3 second
heartbeat interval matches the pace of human attention and is the correct clock for
keeping UIs synchronized with mesh state.

#### Publishing Events

```
# Direct publish
my.computer.network.sse.balance = my.account.balance

# Reactive publish via watcher — the common pattern
my.services.feeds.balance: watch(my.account.balance):
    my.computer.network.sse.balance = my.account.balance

# Publish structured data as JSON
my.services.feeds.orders: watch(world.orders):
    latest = world.orders[@ = world.orders.@time]
    my.computer.network.sse.orders = my.computer.network.to_json(latest)
```

If no clients are subscribed to a stream, writes to that path are no-ops.

#### Client Subscription

A client subscribes by making a GET request to `/sse/{stream_name}` on the node.
The path segment maps to the sub-attribute name under `my.computer.network.sse`:

```
GET /sse/balance   →   my.computer.network.sse.balance
GET /sse/orders    →   my.computer.network.sse.orders
```

SSE subscription requests are handled entirely by the network adapter — they do not
create request nodes and do not pass through `my.computer.network.requests`.

---

### TLS Certificate Store

The node maintains a certificate store for inbound TLS connections, keyed by domain
name. When a connection arrives the node selects the certificate matching the domain
the client requested — this is standard SNI behavior and requires no configuration
beyond having the right cert in the store.

```
my.computer.network.certs["api.example.com"].cert   # PEM certificate chain (Text)
my.computer.network.certs["api.example.com"].key    # PEM private key (Text)
```

Certificate values are stored as Text (PEM format) directly in the tree. This makes
them subject to normal AmorphDB semantics: permissions, temporal history, and — most
importantly — replication and references across the mesh.

#### Loading Certificates

A node owner imports a certificate from the local filesystem once:

```
my.computer.network.certs["api.example.com"].cert =
    my.computer.files.read("/etc/letsencrypt/live/api.example.com/fullchain.pem")
my.computer.network.certs["api.example.com"].key =
    my.computer.files.read("/etc/letsencrypt/live/api.example.com/privkey.pem")
```

After this the tree holds the live value. Subsequent renewals update the tree and
take effect without a node restart.

#### Shared and Referenced Certificates

A tenant who hosts across multiple nodes does not need to copy certificates to each
node manually. They store their certificate once in their own mesh home:

```
world.agent.kalevo.certs["api.example.com"].cert = ...
world.agent.kalevo.certs["api.example.com"].key  = ...
```

Each hosting node's cert store holds a reference:

```
my.computer.network.certs["api.example.com"].cert =
    ""world.agent.kalevo.certs["api.example.com"].cert""
my.computer.network.certs["api.example.com"].key =
    ""world.agent.kalevo.certs["api.example.com"].key""
```

When the tenant renews their certificate they update one location. Every node holding
a reference picks up the new value within one heartbeat tick. No coordination across
nodes is required.

Where isolation is preferred — for compliance, auditing, or operational independence —
the node owner embeds a copy instead of referencing:

```
my.computer.network.certs["api.example.com"].cert =
    embed(""world.agent.kalevo.certs["api.example.com"].cert"")
```

The embedded copy is independent of future changes to the tenant's cert. Both
arrangements are valid and the choice reflects the trust relationship between the
parties, not a system constraint.

---

### PWA Static Asset Serving

Any domain served by this node can have a PWA associated with it. Static assets are
stored as asset records in the mesh — each a simple record with a `.data` field
holding the raw content and a `.mime_type` field declaring the content type. Because
they live in the tree they are subject to normal AmorphDB semantics: replication,
permissions, temporal history, and deployment from anywhere in the mesh.

#### Asset Storage

Assets for a domain are stored under a conventional path:

```
my.computer.network.pwa["app.example.com"].assets["index.html"]
my.computer.network.pwa["app.example.com"].assets["app.js"]
my.computer.network.pwa["app.example.com"].assets["style.css"]
my.computer.network.pwa["app.example.com"].assets["icons/icon-192.png"]
```

#### Deploying Assets

Individual assets are written using the `asset()` constructor, which pairs raw data
with a declared MIME type:

```
my.computer.network.pwa["app.example.com"].assets["index.html"] =
    asset(my.computer.files.read("/build/index.html"), "text/html")
```

A bulk deployment helper is available for deploying an entire build directory:

```
my.computer.network.deploy_pwa("app.example.com", "/build")
```

This reads all files recursively, wraps each as an asset record with an inferred MIME type, and writes
them in a single staged commit. On the next heartbeat tick the in-memory cache is
updated and the new assets are live.

#### Serving Behavior

Static assets are served from an in-memory cache maintained by the network adapter.
The cache is loaded from the mesh at startup. A watcher on the asset path refreshes
the relevant cache entries within one heartbeat tick when assets change. Asset
responses are served in single-digit milliseconds and never enter the request queue.

PWA serving is enabled per domain by setting:

```
my.computer.network.pwa["app.example.com"].enabled  = true
my.computer.network.pwa["app.example.com"].spa_mode = true
```

The fallback order for each inbound request to a PWA-enabled domain is:

1. Exact asset match in cache → serve asset directly
2. `/sse/*` path → hand off to SSE adapter
3. No asset match → create request node for watcher handling
4. No watcher response before TTL and `spa_mode = true` → return `index.html`
5. No watcher response before TTL and `spa_mode = false` → 503

#### MIME Types

Standard MIME types are inferred from file extension:

| Extension | MIME Type |
|---|---|
| `.html` | `text/html; charset=utf-8` |
| `.js` | `application/javascript` |
| `.css` | `text/css` |
| `.json` | `application/json` |
| `.png` | `image/png` |
| `.jpg`, `.jpeg` | `image/jpeg` |
| `.svg` | `image/svg+xml` |
| `.woff2` | `font/woff2` |
| `.webmanifest` | `application/manifest+json` |
| `.ico` | `image/x-icon` |
| `.wasm` | `application/wasm` |

---

## Files Sub-library

### Overview

`my.computer.files` provides access to the local filesystem and a comprehensive
set of structured file import and export procedures covering the formats most
commonly found in business, financial services, and enterprise integration work.

All operations are subject to OS-level permissions — the daemon runs as a specific
OS user and can only access files that user is permitted to access. AmorphDB does
not grant file access beyond what the OS permits.

All import procedures are one-time reads at call time. They do not watch files for
subsequent changes.

### Implementation Status

| Capability | Status |
|---|---|
| Basic file operations | ✅ Implemented |
| Delimited import/export (CSV, TSV, custom) | ✅ Implemented |
| JSON import/export | ✅ Implemented |
| TOML import/export | ✅ Implemented |
| XML import/export | 🚧 Not yet implemented |
| Fixed-width import via ARI | 🚧 Not yet implemented |
| Fixed-width export | 🔄 Planned (after ARI import) |
| Excel import/export | 🔄 Planned |

---

### Basic File Operations

```
my.computer.files.read(path)            # → Text or Unknown
my.computer.files.write(path, content) # → Time (of write) or Unknown
my.computer.files.exists(path)         # → Boolean
my.computer.files.delete(path)         # → Boolean (true if deleted)
my.computer.files.list(path)           # → List of Text (filenames)
my.computer.files.info(path)           # → Record: .size, .modified, .is_dir
```

Unknown return values carry failure reasons:

| Failure | Value |
|---|---|
| Path not found | `#not_found` |
| Permission denied | `#access_denied` |
| Path is a directory | `#is_directory` |
| Disk full | `#disk_full` |
| I/O error | `#io_error` |

---

### Delimited File Import and Export ✅

Delimited files use a separator character to distinguish fields, with one record
per line. CSV (comma-separated) and TSV (tab-separated) are the most common forms,
but any single-character delimiter is supported.

#### Import

```
my.computer.files.import(path)
my.computer.files.import(path, format)
my.computer.files.import(path, format, options)
```

Format is inferred from file extension (`.csv`, `.tsv`) or supplied explicitly.
Returns a list of records — one per data row.

#### Options

```
options.delimiter    # Text: field delimiter (default "," for CSV, tab for TSV)
options.has_header   # Boolean: first row defines field names (default: true)
options.skip_rows    # Number: rows to skip before header (default: 0)
options.null_value   # Text: treat this value as Unknown (default: "")
options.encoding     # Text: file encoding (default: "utf-8")
options.infer_types  # Boolean: convert to best-fit types (default: true)
options.quote_char   # Text: quote character for field wrapping (default: '"')
```

When `has_header` is false, columns are named `col_0`, `col_1`, `col_2`, etc.

Quoted values containing the delimiter character are handled automatically.
Quote characters within a value are escaped by doubling (standard CSV convention).

#### Type Inference

When `infer_types` is true (the default):

| Pattern | AmorphDB type |
|---|---|
| Integer or decimal numeric | Number |
| ISO 8601 date or datetime | Time |
| Currency symbol + decimal | Money |
| `true` / `false` (case-insensitive) | Number (1 or 0) |
| Empty string or configured null value | Unknown (`#empty`) |
| Anything else | Text |

#### Export

```
my.computer.files.export(node, path)
my.computer.files.export(node, path, format)
my.computer.files.export(node, path, format, options)
```

Each child of the node becomes a row. Export mirrors import options. Values
containing the delimiter or newlines are automatically quoted. Quote characters
within values are escaped by doubling.

#### Examples

```
# Import a pipe-delimited bank transaction file
options.delimiter = "|"
options.has_header = true
my.data.transactions = my.computer.files.import("/data/transactions.txt", "csv", options)

# Import a standard CSV and iterate rows
my.data.sales = my.computer.files.import("/data/q3_sales.csv")
for row in my.data.sales:
    my.computer.output(row.region & ": " & row.revenue)

# Export a filtered query to CSV
pending = world.orders[status = "pending", @time > @2026-01-01]
my.computer.files.export(pending, "/tmp/pending_orders.csv")
```

---

### JSON and TOML Import and Export ✅

JSON and TOML are handled by the same `import` and `export` procedures. Format
is inferred from extension (`.json`, `.toml`) or supplied explicitly.

#### JSON Structure Mapping

| JSON structure | AmorphDB mapping |
|---|---|
| Root object | Single top-level record |
| Root array | List of records |
| Nested objects | Child records (hierarchy preserved) |
| Nested arrays | Child record lists |
| Scalar values | Typed fields (type inference applies) |
| Null values | Unknown (`#null`) |

#### Examples

```
# Import config at startup
config = my.computer.files.import("/etc/myapp/config.toml")
my.config.db.host = config.database.host
my.config.db.port = config.database.port

# Import JSON and store in mesh
my.data.customers = my.computer.files.import("/data/customers.json")

# Backup config to JSON
my.computer.files.export(my.config, "/backup/config.json", "json")

# Pretty-print JSON export
options.pretty = true
my.computer.files.export(my.report, "/output/report.json", "json", options)
```

---

### XML Import and Export 🚧

> **Not yet implemented.** Specified here for planning purposes.

XML is widely used in financial services, healthcare, and enterprise integration.
AmorphDB maps XML structures to the tree hierarchy and back.

#### Import

```
my.computer.files.import(path, "xml")
my.computer.files.import(path, "xml", options)
```

#### XML Structure Mapping

| XML feature | AmorphDB mapping |
|---|---|
| Elements | Record types |
| Attributes | Record fields |
| Text content | Field values |
| Repeating elements | Record lists |
| Namespaces | Supported with prefix aliasing |
| Missing elements or attributes | Unknown values |

#### Options

```
options.namespace_aliases   # Record: prefix → URI mappings
options.field_map           # Record: XPath expression → field name mappings
options.encoding            # Text: file encoding (default: "utf-8")
```

#### Export Options

```
options.pretty              # Boolean: indented output (default: true)
options.namespace_prefix    # Text: default namespace prefix
options.attributes          # List: field names to render as XML attributes
                            #       (others become child elements)
```

#### Example

```xml
<financial_report xmlns:fin="http://example.com/financial">
    <header date="2024-03-15">
        <institution>First National Bank</institution>
    </header>
    <fin:transactions>
        <fin:transaction id="T001" amount="1500.00">
            <description>Wire Transfer</description>
        </fin:transaction>
    </fin:transactions>
</financial_report>
```

Imports to:

```
report:
    header:
        date: @2024-03-15
        institution: "First National Bank"
    transactions[0]:
        id: "T001"
        amount: 1500.00
        description: "Wire Transfer"
```

---

### Fixed-Width Import via ARI 🚧

> **Not yet implemented.** Specified here for planning purposes.

Fixed-width files are common in legacy financial systems from vendors like PSCU,
Jack Henry, and mainframe environments. AmorphDB provides **ARI (Anchor Relative
Identification)**, a notation for describing these file layouts as a text value —
similar in spirit to a regular expression, but operating at the structural level
of a full document rather than a single pattern match.

Unlike traditional fixed-width parsers that rely on rigid column positions, ARI
locates fields through relational anchors — descriptions of what surrounds the
target data within the file. This makes ARI resilient to the inconsistent spacing
and variable-length headers common in real-world legacy formats.

Because an ARI spec is just a text value, it can be stored in the mesh, versioned,
passed as a parameter, and generated programmatically — including by automated
analysis tools.

#### Import Procedure

```
my.computer.files.import_fixed_width(path, ari_spec)
my.computer.files.import_fixed_width(path, ari_spec, options)
```

`ari_spec` is a Text value containing the ARI definition. It is typically stored
as a named value in the mesh and reused across imports:

```
# Store a spec once
my.schemas.bank_report = my.computer.files.read("/schemas/bank_report.ari")

# Reuse it for every file
my.data.report = my.computer.files.import_fixed_width("/data/report.txt", my.schemas.bank_report)
```

#### ARI Spec Language

An ARI spec is a text document using the following syntax.

**Overall structure:**

```
section section_name [starts("pattern")] [ends("pattern")]:
    field field_name: anchor [anchor ...]: pattern [, pattern ...]
    [break [on "pattern" | on /regex/]]
    [section ...]
```

**Anchors** describe the spatial relationship between a field and a known marker
in the file. Each anchor specifies a direction, a distance, and a pattern to
match:

```
direction distance: pattern
```

**Directions:** `left`, `right`, `up`, `down`, `same` (same line as previous anchor)

**Distances:**

| Syntax | Meaning |
|---|---|
| `5` | Exactly 5 characters or lines |
| `2-10` | Between 2 and 10 |
| `3-` | 3 or more |
| `-8` | 8 or fewer |
| `flush` | Edge of the current section |

**Pattern types:**

| Type | Syntax | Example |
|---|---|---|
| Text literal | `"text"` | `"ID"`, `"TOTAL"` |
| Regular expression | `/regex/` | `/\d{4}-\d{2}-\d{2}/` |
| Regex with transform | `/regex/replacement/` | `/(\d{2})\/(\d{2})\/(\d{4})/$3-$1-$2/` |
| Built-in type keyword | `date`, `money`, `integer`, `decimal`, `ssn` | |

Multiple patterns on one anchor are OR logic (any match succeeds). Multiple
anchors on one field are AND logic (all must match).

**Sections** define named regions of the file. A section without `starts`/`ends`
uses the enclosing section's boundaries. Nested sections are supported.

**Break** marks a repeating record structure within a section. The parser starts
a new record each time the first field re-matches:

| Syntax | Trigger |
|---|---|
| `break` | Re-match of first field in section |
| `break on "---"` | Explicit text separator |
| `break on /^\s*$/` | Blank line separator |

**Custom types** can be defined within a spec for reuse:

```
type DATE:
    /(\d{2})\/(\d{2})\/(\d{4})/ -> "$3-$1-$2"
    /(\d{4})-(\d{2})-(\d{2})/  -> "$1-$2-$3"
    output: ISO_DATE

type CURRENCY:
    /\$([\d,]+\.\d{2})/    -> strip(",") as DECIMAL(12,2)
    /([\d,]+\.\d{2})CR/    -> strip(",") negate as DECIMAL(12,2)
    output: DECIMAL(12,2)
```

#### Complete Example

Given this file:

```
BANK REPORT 2024-03-15
ID: 12345
==== transactions ====
TYPE: DEBIT  AMOUNT: $1,500.00  TAX: $45.00
TYPE: CREDIT AMOUNT: $2,000.00  TAX: $0.00
==== end of transactions ====
TOTAL: $500.00
```

This ARI spec parses it:

```
section bank_report:
    field report_date: right 10 up 1: date
    field bank_id: right 5 down 1: "ID"

    section transactions starts("==== transactions ====") ends("==== end of transactions ===="):
        field transaction_type: right 6 same: "TYPE"
        field amount: right 12 same: "AMOUNT"
        field tax: right flush same: "TAX"
        break

    field total: right 10 down 1-5: "TOTAL"
```

Producing this AmorphDB structure:

```
bank_report:
    report_date: @2024-03-15
    bank_id: 12345
    transactions[0]:
        transaction_type: "DEBIT"
        amount: 1500.00
        tax: 45.00
    transactions[1]:
        transaction_type: "CREDIT"
        amount: 2000.00
        tax: 0.00
    total: 500.00
```

Fields not found are recorded as Unknown (`#not_found`).

#### Fixed-Width Export 🔄

> **Planned** after ARI import is complete.

Export will use a position-based mapping spec:

```
my.computer.files.export_fixed_width(node, path, export_spec)
```

The export spec (also a text value) will define column positions, padding rules,
section separators, and type formatting for each field.

---

### Excel Import and Export 🔄

> **Planned.** Specified here for planning purposes.

Excel files (`.xlsx`) are the standard format for business reports and financial
data exchange. Support will be added as a later phase.

```
my.computer.files.import(path, "xlsx")
my.computer.files.import(path, "xlsx", options)
my.computer.files.export(node, path, "xlsx")
my.computer.files.export(node, path, "xlsx", options)
```

Planned options will cover sheet selection, header row handling, cell type
inference, and named range mapping.

---

## Permissions and Access

### Node Owner

All paths under `my.computer` are owned by the local node agent. Default permissions
are owner-only. Nothing is shared until the node owner explicitly grants access.

### Granting Access to Other Agents

The node owner grants other agents access to specific capabilities by setting
permissions on the relevant paths:

```
# Allow a tenant agent to register route handlers
my.computer.network.requests.@write = ""world.agent.kalevo""

# Allow a deploy agent to publish PWA assets
my.computer.network.pwa["app.example.com"].assets.@write = ""world.agent.deploy_bot""

# Allow an agent to publish SSE events on a specific stream
my.computer.network.sse.market_prices.@write = ""world.agent.price_service""
```

Permissions can be revoked at any time by the node owner. The arrangement between
a node owner and a tenant — what is shared, on what terms, for how long — is
expressed entirely through the AmorphDB permission model. No special configuration
is required beyond setting and unsetting permissions.

### File Access

File operations are bounded by OS-level permissions. No MBL code can access files
beyond what the daemon's OS user is permitted to read or write.

---

## Summary of Tree Paths

| Path                                           | Kind       | Purpose                     |
|------------------------------------------------|------------|-----------------------------|
| `my.computer.network.get(url, ...)`            | Procedure  | Outbound GET                |
| `my.computer.network.post(url, body, ...)`     | Procedure  | Outbound POST               |
| `my.computer.network.put(url, body, ...)`      | Procedure  | Outbound PUT                |
| `my.computer.network.patch(url, body, ...)`    | Procedure  | Outbound PATCH              |
| `my.computer.network.delete(url, ...)`         | Procedure  | Outbound DELETE             |
| `my.computer.network.parse_json(text)`         | Procedure  | JSON text → tree node       |
| `my.computer.network.to_json(node)`            | Procedure  | Tree node → JSON text       |
| `my.computer.network.deploy_pwa(domain, path)` | Procedure  | Bulk PWA asset deploy       |
| `my.computer.network.ok(req, body)`            | Procedure  | 200 response helper         |
| `my.computer.network.ok_json(req, node)`       | Procedure  | 200 JSON response helper    |
| `my.computer.network.not_found(req)`           | Procedure  | 404 response helper         |
| `my.computer.network.bad_request(req, msg)`    | Procedure  | 400 response helper         |
| `my.computer.network.server_error(req, msg)`   | Procedure  | 500 response helper         |
| `my.computer.network.redirect(req, url)`       | Procedure  | 302 redirect helper         |
| `my.computer.network.requests`                 | Live list  | All inbound requests        |
| `my.computer.network.sse.*`                    | Live nodes | SSE publish paths           |
| `my.computer.network.certs[domain].cert`       | Text / Ref | TLS certificate (PEM)       |
| `my.computer.network.certs[domain].key`        | Text / Ref | TLS private key (PEM)       |
| `my.computer.network.pwa[domain].enabled`      | Boolean    | Enable PWA for domain       |
| `my.computer.network.pwa[domain].spa_mode`     | Boolean    | SPA fallback to index.html  |
| `my.computer.network.pwa[domain].assets[path]` | Asset record | Static asset store (.data + .mime_type) |
| `asset(data, mime_type)`                       | Constructor  | Build a MIME-typed asset record         |
| `my.computer.files.read(path)`                    | Procedure  | Read file as Text                        |
| `my.computer.files.write(path, content)`          | Procedure  | Write Text to file                       |
| `my.computer.files.exists(path)`                  | Procedure  | Check existence                          |
| `my.computer.files.delete(path)`                  | Procedure  | Delete file                              |
| `my.computer.files.list(path)`                    | Procedure  | List directory                           |
| `my.computer.files.info(path)`                    | Procedure  | File metadata                            |
| `my.computer.files.import(path, ...)`             | Procedure  | Structured file → tree node (JSON, TOML, CSV, TSV, XML ✅/🚧, Excel 🔄) |
| `my.computer.files.export(node, path, ...)`       | Procedure  | Tree node → structured file              |
| `my.computer.files.import_fixed_width(path, ari)` | Procedure  | Fixed-width file → tree node via ARI 🚧  |
| `my.computer.files.export_fixed_width(node, path, spec)` | Procedure | Tree node → fixed-width file 🔄   |
