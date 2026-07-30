# MBL Quick Reference

**Audience:** Application developers writing MBL on top of AmorphDB.
**Not for:** Platform implementers — they need the full `amorphdb_design.md`.
**Authority:** When this document conflicts with `amorphdb_design.md`, the full design is authoritative and this doc needs updating. Treat this as a distillation, not a truth.

> Note: **MBL = Modern Business Language.** Designed around scope resolution and path traversal, with deliberately high-level types (numbers are always the processor's largest float, pictures are always full RGBA 16-bit depth). This trades storage efficiency for uniform simplicity.

---

## Orientation

AmorphDB is a temporal tree-graph database. Every value has history — assigning is recording a change, not overwriting. Data lives in a hierarchy of attributes, each of which carries a chain of timestamped instances.

The mental model you need before writing MBL:

- **Everything is a path.** `world.agent.alice.balance`, `my.cache.recent[0].title`. No tables, no rows, no SQL.
- **Everything has history.** `account.balance` means "the current value." `account.balance[@2026-01-01]` means "the value as of that date." You get versioning for free.
- **Writes happen in ticks.** One mesh heartbeat = ~1/3 second. All writes a watcher makes during a tick commit atomically at the tick boundary.
- **Reactivity is primary.** Watchers fire on data changes. This is how you build application logic — not by defining HTTP handlers or event loops.
- **Unknown propagates.** If a computation can't produce a value, the result is `unknown("reason")`. This flows through expressions instead of throwing.

When you find yourself asking "does AmorphDB have X?" — check the full design doc. Don't guess.

---

## Syntax at a Glance

### Line prefixes

| Prefix | Meaning |
|---|---|
| `my` | The current agent's home (`world.agent.{identity}`) |
| `.` | Relative to current scope |
| `world` | The shared root — all shared data lives here |
| (no prefix) | Nearest matching upstream scope; else same as `.` |

### Suffixes

| Suffix | Meaning |
|---|---|
| `:` | Introduces a definition (procedure, watcher, record body) |
| `=` | Records a value change (assignment) |
| `.` | Path traversal |

### Brackets and projections

Brackets filter or query the attributes under a path:

```mbl
person[name = "bob"]                      # Index / equality filter
person[name = "bob", age > 18]            # Comma = AND
person[name = "bob" or age > 18]          # 'or' requires keyword
person[(name = "bob" or age > 18), active = true]   # Mixed groups
```

Temporal filters use `@`:

```mbl
person.name[@2026-01-01]        # As of this date (most recent at or before)
person.name[<@2026-01-01]       # Strictly before
person.name[>=@2026-01-01]      # At or after
person.name[@ >= @2026-01-01, @ < @2026-02-01]   # Range
```

Projections select fields with `{...}`:

```mbl
person{ name, age }                           # Just these two fields
my.users[active = true]{ name, email }        # Filter then project
```

### Comments

```mbl
# Single-line comment
x = 5   # inline comment
## Block comment
spans multiple lines ##
### Nested blocks use more # ###
```

### Assignment vs definition

```mbl
x = 5                           # Assignment — records a new instance
x: procedure(n): return n * 2   # Definition — introduces a callable
```

---

## Types

| Type | Literal | Notes |
|---|---|---|
| Text | `"Hello"` | UTF-8. A simple literal cannot contain a quote. To embed quotes use the extended form `_"…"_`: `_"He said "hi" loudly."_`. Lengthen the quote run on both ends when the text itself contains `"_`: `_""a "_ sequence""_`. To interpolate, use `~"…"~`: `~"Hello {my.user.name}"~` — paths only, `{{` for a literal brace. All forms may span multiple lines. |
| Number | `42`, `3.14`, `1_000_000` | Largest float the processor supports |
| Boolean | `true`, `false` | |
| Time | `@2026-01-15 14:30:22.500` | UNIX time in UTC |
| Money | `$143.68`, `€21.80`, `¤143.68 USD` | Currency is part of the value |
| Picture | (no literal) | RGBA 16-bit; loaded via operation |
| Reference | `(link)world.foo` | A link to a path, not its value |
| Procedure | `procedure(x): return x` | First-class; storable in attributes |
| Watcher | `watch(path): body` | Reactive handler |
| Embed | `embed my.record` | Structural composition (not a value) |

### Unknown (meta type)

Every type supports `Unknown` as its meta type — used for "no value available":

```mbl
x = unknown                         # Bare unknown
x = unknown("not_found")            # With reason
x = unknown("timeout")              # Application-specific
```

Unknown propagates through expressions:

```mbl
y = 5 + unknown("offline")          # y becomes unknown("offline")
```

The **definite-equality** operator `?=` returns a boolean instead of propagating Unknown:

```mbl
if x ?= 5:                          # Safe: false if x is Unknown
    ...

if x ?= unknown:                    # True if x is any Unknown
    ...

if x ?= unknown("timeout"):        # True only if x is Unknown with reason "timeout"
    ...
```

`is_unknown(x)` and `x ?= unknown` are equivalent. Only equality has a
`?`-prefixed form. Ordering operators (`<`, `>`, `<=`, `>=`) propagate
Unknown. For definite non-equality, use `not (x ?= y)`.

---

## Records and Lists

Same underlying structure — both are tree nodes with attributes. Records have named fields; lists are numerically indexed.

```mbl
# Record literal
person = { name: "Alice", age: 30 }

# List literal
numbers = [1, 2, 3]

# Nested
employee = {
    name: "Bob",
    address: { city: "Rome", state: "NY" },
    skills: ["mbl", "go"]
}
```

### Collection operations

```mbl
x..count                            # Number of attributes
x..append(value)                    # Add to end of list
x..prepend(value)                   # Add to beginning
x..remove(index)                    # Remove by position (list re-indexes)
x..remove(value)                    # Remove first match (by value for list, by name for record)
x..remove(from, to)                 # Remove a range by index
x..combine(",")                     # Join values to text (default separator ",")
```

---

## Templates and Heritability

Any record can serve as a template. `new(template)` creates an instance. Each field has a heritability rule controlling what happens at instantiation:

| Modifier | Meaning |
|---|---|
| `(copy)` | Deep copy value — new instance has its own copy (default) |
| `(link)` | Reference template's value — updates propagate |
| `(reset X)` | Use `X` as the default, ignoring template's current value |
| `(exclude)` | Omit from new instances |

```mbl
my.templates.account:
    name:          (copy) ""
    balance:       (reset 0) $1000
    interest_rate: (link) 0.05
    notes:         (exclude) "template maintenance only"

my.customers.alice = new(my.templates.account)
my.customers.alice.name = "Alice"
# alice.balance = 0 (reset)
# alice.interest_rate is linked to template
# alice.notes does not exist
```

### Override at instantiation

```mbl
my.customers.bob = new my.templates.account {
    name: (copy) "Bob",
    balance: (reset 500) 0,
    interest_rate: (copy) 0.05      # Override link to own copy
}
```

### Global modifier

```mbl
my.snapshot = new(my.templates.account, (link)) { name: (copy) "Snapshot" }
# Everything is linked except name, which is copied
```

---

## Temporal Queries

Every attribute carries history. This is AmorphDB's single most distinctive feature — use it.

```mbl
# Current value
person.name

# Value as of a time
person.name[@2026-06-01]

# Value at a specific instant
person.name[@2026-06-01 14:30:00]

# History in a range
person.name[@ >= @2026-01-01, @ < @2026-07-01]

# Instance meta attributes (about the recorded change)
person.name.@time                   # When was this value recorded?
person.name.@agent                  # Who recorded it?
person.name.@previous_instance_id   # Pointer to the previous value
person.name.@size                   # Size in bytes
person.name.@ttl                    # Time-to-live in ms (0 = permanent)
```

Mix temporal and attribute conditions:

```mbl
my.log[@agent = (link)world.agent.alice, @time >= @2026-03-01]
my.cache[@ttl > 0]
my.files[@size > 1000000]
```

**Key idiom:** if you catch yourself wanting a `versions` table, stop. History is already there.

---

## Watchers

The reactive core. A watcher is a procedure that runs when specified paths change.

### Basic form

The primary form places `watch` first. The name binds in the current
persistent scope:

```mbl
watch balance_check(my.account.balance, my.account.limit):
    if my.account.balance > my.account.limit:
        my.account.status = (quietly) "overlimit"
```

The path-first form is equivalent and useful for explicit destinations:

```mbl
my.automation.balance_check: watch(my.account.balance, my.account.limit):
    ...
```

### Multi-path watchers

Comma-separated paths. Fires when **any** of them changes.

```mbl
watch monitor(my.cpu.usage, my.memory.usage, my.disk.usage):
    ...
```

### Append watchers

Fire when items are added to a list — perfect for queue processing:

```mbl
watch new_orders append(my.orders) as orders:
    for order in orders:
        my.computer.output("Order: " & order.id)
```

The `as name` binding is required. `orders` is a list of the items that arrived this tick.

With a predicate filter:

```mbl
watch append(my.orders[status ?= "urgent"]) as urgent:
    for o in urgent:
        my.alerts.send("Urgent: " & o.id)
```

### `(quietly)` — suppressing a trigger

Watcher chains are designed to converge on their own, so `(quietly)` is needed
less often than you might expect. Two rules do most of the work:

- **An unchanged write does not trigger anything.** A watcher that recomputes a
  value and writes back the same answer settles on its second pass.
- **A watcher runs at most once per drain**, seeing the latest state — several
  writes to paths it watches produce one run, not one per write.

Use `(quietly)` when a watcher writes a path it genuinely observes *and* the value
differs every time, so convergence cannot happen by itself — a counter, a
timestamp, a progress marker:

```mbl
# Watcher on my.job.progress that also stamps when it last ran
my.job.last_checked = (quietly) now()   # differs each tick, so suppress it
```

A block form covers several writes at once:

```mbl
quietly:
    my.job.last_checked = now()
    my.job.check_count = my.job.check_count + 1
```

The write still happens and is still recorded — it simply does not announce
itself.

### Watcher attributes

Every watcher exposes:

| Attribute | Meaning |
|---|---|
| `@enabled` | Turn on/off (default `true`) |
| `@watching` | Paths being monitored |
| `@last_run` | When it last executed |
| `@run_count` | Total executions |
| `@last_error` | Error from last failed run |

```mbl
my.automation.monitor.@enabled = false   # Disable
```

### Heartbeat atomicity

All changes a watcher makes in one tick commit **atomically** at the tick boundary. If the watcher produces an unhandled Unknown, all staged changes are discarded. Cross-zone writes are batched into one replication payload per destination.

---

## Procedures

The primary form places `procedure` first, followed by name and parameters.
The name binds in the current persistent scope:

```mbl
procedure double(x):
    return x * 2

# Call
y = double(21)     # 42

# Inline (single-line body)
procedure sum(a, b): return a + b
```

The path-first form is equivalent and useful when the destination differs
from the current scope:

```mbl
my.functions.double: procedure(x): return x * 2
```

Anonymous procedures in expression position:

```mbl
my.double = procedure(x): return x * 2
my.doubled = map(my.numbers, procedure(x): return x * 2)
```

### Local vs persistent

- Parameters and unqualified names are **local** — discarded when the procedure returns.
- `my.*` and `world.*` reach into persistent storage.
- A procedure's own sub-attributes are persistent local storage between calls:

```mbl
procedure page_views():
    .count = .count + 1
    return .count
```

### Control flow

```mbl
if condition:
    ...
else if other_condition:
    ...
else:
    ...

for item in collection:
    ...

for item, index in collection:
    ...

while condition:
    ...

return value
return                              # No value
break
pass                                # No-op; empty blocks need this
```

### Error handling

```mbl
catch:
    risky_operation()
else unknown:
    my.computer.output("Failed: " & unknown)
```

---

## Permissions

Five permission attributes on every piece of data:

| Permission | Controls |
|---|---|
| `@read` | Reading the value and traversing into sub-attributes |
| `@write` | Creating new instances (modifying) |
| `@expand` | Adding new sub-attributes |
| `@grant` | Modifying permissions on this attribute |
| `@purge` | Marking for deletion |

Values:

- Agent reference: `(link)world.agent.alice`
- List: `[(link)world.agent.alice, (link)world.agent.bob]`
- `"Anything"` — unrestricted
- `"Nothing"` — no access

### Defaults

- Under `my` (agent's home, `world.agent.{identity}`): everything defaults to owner-only.
- Under `world`: `@read` defaults to `"Anything"`; others default to `"Nothing"`.
- Permissions cascade down the tree until explicitly overridden.

```mbl
world.announcements.@read = "Anything"
world.announcements.@write = (link)world.agent.admin

my.secrets.@read = (link)world.agent.me    # Private
```

**Placement principle:** put permissions close to the data they protect. Don't rely on far-ancestor cascades for security-critical attributes.

---

## Stamps

Metadata automatically attached to every written value. Travels with the data.

### Built-in stamps

| Stamp | Meaning |
|---|---|
| `@agent` | Who created this instance |
| `@time` | When it was created |
| `@classification` | Security/sensitivity level |
| `@source` | Origin system |
| `@retention` | How long to keep |
| `@geography` | Jurisdiction restrictions |

### Custom stamps

Set stamp attributes once; all subsequent writes by the agent carry them:

```mbl
my.stamp.department = "engineering"
my.stamp.project = "customer-portal"

my.features.new_login = {...}        # Automatically stamped
```

### Querying by stamp

```mbl
world.reports[@stamp.department = "engineering"]
my.datasets[@stamp.source != "internal"]
world.data[@stamp.classification = "confidential", @stamp.retention < now()]
```

Stamps replace most of what "audit columns" do in a traditional schema. You don't need a `created_by` or `created_at` field — they're already there.

---

## The `my.computer` Surface

The platform primitives available to MBL. Categorized for easy lookup.

### Output / input

```mbl
my.computer.output(text)             # Write to stdout
my.computer.input(prompt)            # Read from stdin (returns text)
```

### Files

Basic operations:

```mbl
my.computer.files.read(path)                  # Read file as Text
my.computer.files.write(path, content)        # Write Text to file
my.computer.files.exists(path)                # Returns true/false
my.computer.files.delete(path)
my.computer.files.list(path)                  # List directory → list
my.computer.files.info(path)                  # .size, .modified, .is_directory
```

Structured import/export (format one of `"json"`, `"csv"`, `"tsv"`, `"toml"`, `"xml"`):

```mbl
my.computer.files.import(path, format)
my.computer.files.import(path, format, options)   # options: separator, headers
my.computer.files.export(node, path, format)
my.computer.files.export(node, path, format, options)
```

Import failures return Unknown: `unknown("file_not_found")`, `unknown("unsupported_format")`, `unknown("parse_error")`.

Fixed-width import via ARI spec (see ARI documentation):

```mbl
my.computer.files.import_fixed_width(path, ari_spec)
```

> 🚧 **Current status (per design):** `files.read/write` plus XML import/export and ARI fixed-width import are working. JSON, CSV, TSV, and TOML are stubbed.

### Shell execution

```mbl
result = my.computer.run("ls -la /tmp")   # Runs a shell command
# result.exit_code    Number  (0 = success)
# result.stdout       Text
# result.stderr       Text
```

### Network — outbound HTTP

For calling external APIs:

```mbl
my.computer.network.web.get(url, options)
my.computer.network.web.post(url, body, options)
my.computer.network.web.put(url, body, options)
my.computer.network.web.patch(url, body, options)
my.computer.network.web.delete(url, options)
```

Options (all optional): `headers`, `timeout` (ms, default 30000), `follow_redirects` (default true).

Response record: `.status`, `.body`, `.headers`, `.time`, `.duration`.

Failures return Unknown: `unknown("timeout")`, `unknown("dns_failure")`, `unknown("connection_refused")`, `unknown("tls_error")`.

### Network — JSON

```mbl
my.computer.network.web.parse_json(text)      # JSON text → tree node
my.computer.network.web.to_json(node)         # Tree node → JSON text
```

### Network — inbound request queue

External HTTP requests arrive as nodes in a queue, processed by append watchers:

```mbl
my.api.orders: watch append(
    my.computer.network.web.requests[
        method ?= "GET", path starts "/api/orders/"
    ]
) as reqs:
    for req in reqs:
        order = my.orders[id = req.query.id]
        my.computer.network.web.ok_json(req, order)
```

Request fields: `.domain`, `.method`, `.path`, `.query`, `.headers`, `.body`, `.remote_ip`, `.time`.

Response helpers:

```mbl
my.computer.network.web.ok(req, body)
my.computer.network.web.ok_json(req, node)
my.computer.network.web.not_found(req)
my.computer.network.web.bad_request(req, msg)
my.computer.network.web.server_error(req, msg)
my.computer.network.web.redirect(req, url)
```

Unanswered requests time out (default 5 seconds) and return 503 to the client.

### Network — PWA deployment

```mbl
my.computer.network.web.pwa["host.example.com"].enabled = true
my.computer.network.web.pwa["host.example.com"].spa_mode = true
my.computer.network.web.deploy_pwa("host.example.com", "/path/to/build")
```

Assets are served sub-millisecond from an in-memory cache. Refreshed within one heartbeat tick on change.

### Network — SSE

For PWA clients, SSE is automatic (handled by the Go layer). For external SSE consumers:

```mbl
my.computer.network.web.sse.price_updates = world.market.price
```

External subscribers hit `GET /sse/{name}`.

### Network — Email

`my.computer.network.email` provides IMAP/SMTP client capabilities. AmorphDB
does not act as a mail server — it talks to existing infrastructure (Postfix,
Gmail, Microsoft 365, etc.).

**Two procedures and one configuration trigger:**

```mbl
# Send via SMTP — returns sent message record or Unknown
my.computer.network.email.send(account, message)

# Pull-based IMAP fetch — returns list of new messages
my.computer.network.email.fetch(account, options)
```

Setting `subscription.enabled: true` on an account record activates IMAP IDLE
on the write-authority node. New messages append to `subscription.target`.
Consume them with ordinary append watchers.

**Account record fields** (read by the runtime):

```mbl
my.email.support:
    protocol: "imap"
    host: "outlook.office365.com"
    port: 993
    auth.username: "support@example.com"
    auth.password: "..."
    auth.@read: [(link)world.agent.admin]    # credential safety
    smtp.host: "smtp.office365.com"
    smtp.port: 587
    subscription.enabled: true
    subscription.target: .inbox
```

**Message record fields:** `from`, `to`, `cc`, `bcc`, `subject`, `body`,
`body_html`, `attachments` (list of `{filename, content_type, bytes}`),
`headers`, `time`, `message_id`, `in_reply_to`.

**Subscription example:**

```mbl
watch route_support append(my.email.support.inbox) as msgs:
    for msg in msgs:
        world.support.unclaimed..append(msg)
```

**Send example:**

```mbl
my.computer.network.email.send(my.email.support, {
    to: original.from,
    subject: "Re: " & original.subject,
    in_reply_to: original.message_id,
    body: "Thanks for contacting support."
})
```

Account records can live anywhere — `my.email.*` for personal, `world.email.*`
for shared. Location doesn't affect behavior. Subscription runs on the
write-authority node only; authority transfer carries subscription
responsibility automatically.

> 🚧 **Initial implementation:** IMAP/SMTP with username/password. OAuth 2.0
> for Gmail/Microsoft 365 planned as follow-up.

### Data-driven PWA communication

This is the key pattern for your own PWA clients. You do **not** use the
request queue for PWA traffic. Instead:

1. A user interacts with the PWA. The Go bridge resolves their identity
   from their auth token and writes to
   `world.apps.<app>.users.<identity>.intent.*`.
2. A per-user watcher fires, runs business logic, writes results back to
   that user's data subtree.
3. The Go bridge detects the change and pushes it via SSE to the user's
   browser.

App users are **not** mesh agents. They are managed by the app via a
device/token/identity model. See `amorphdb_design.md` (PWA User Identity
and Authentication) for the full specification.

Your MBL writes per-user watchers on **data**, not on HTTP. Each user gets
their own watchers, registered at signup:

```mbl
# Called at signup to set up this user's watchers
my.app.setup_user_watchers: procedure(username):
    user_path = world.apps.myapp.users[username]

    watch my.app.watchers[username & ".submit"](user_path.intent.submit_order):
        if user_path.intent.submit_order = true:
            world.apps.myapp.shared.orders..append({
                user_identity: username,
                items: user_path.forms.order_data.items,
                submitted: now()
            })
            user_path.intent.submit_order = (quietly) false
            user_path.intent.result = { status: "submitted" }
```

**Data model for PWA apps:**

```
world.apps.<app>
    app/                    # shared UI definition (from app.html)
    assets/                 # static files served by Go layer
    devices/                # pre-auth browser connections (login/signup only)
    users/                  # authenticated per-user data
    tokens/                 # auth tokens mapping device → identity
    auth/                   # app-defined credentials
```

---

## The World Clock

`world.clock` carries the current UTC time with sub-attributes for every common decomposition. Watchers target exactly the granularity they need — the sub-attribute changes only when that granularity ticks:

| Sub-attribute | Example value |
|---|---|
| `.utc` | current UTC time value |
| `.year` | `2026` |
| `.month` | `1`–`12` |
| `.monthname` | `"March"` |
| `.day` | `1`–`31` |
| `.hour` | `0`–`23` |
| `.minute` | `0`–`59` |
| `.second` | `0`–`59` |
| `.weekday` | `1`–`7` (1=Sunday) |
| `.weekdayname` | `"Sunday"` |
| `.yearday` | `1`–`366` |
| `.week` | `1`–`53` |
| `.quarter` | `1`–`4` |
| `.unix` | raw UNIX timestamp |

```mbl
# Hourly
watch hourly_report(world.clock.hour):
    my.reports.generate_hourly

# First of each month
watch monthly_close(world.clock.day):
    if world.clock.day ?= 1:
        my.accounting.close_month
```

All time is stored in UTC. Time-zone conversion is a display concern.

---

## Filters

A **filter** hides data from the agent's own view — client-side only. Filters do not bypass permissions and do not prevent other agents from seeing data.

```mbl
# Define filters
my.filter.hide_test_data: @stamp.environment = "test"
my.filter.production_only: @stamp.classification = "production"

# Activate one filter
my.filter = my.filter.production_only

# Activate multiple
my.filter = [my.filter.hide_test_data, my.filter.production_only]
```

When active, matching data returns `unknown("filtered")`. The agent can remove filters to see the full data, but cannot use a filter to access data they lack permissions for.

---

## Built-in Procedures

A grab bag of primitive procedures available from any scope. These are called as bare names, not under `my.computer`.

**Text:**

```mbl
length(text)                        # character count
substring(text, start, length)
find(text, pattern)
replace(text, old, new)
split(text, separator)              # returns list
trim(text)
upper(text) ; lower(text)
```

**Math:**

```mbl
abs(n) ; round(n) ; floor(n) ; ceiling(n)
min(a, b) ; max(a, b)
sqrt(n) ; log(n) ; exp(n)
random()                            # 0.0 to 1.0
```

**Time:**

```mbl
now()                               # current UTC time
format_time(time, format)
parse_time(text, format)
add_time(time, days, hours, minutes)
```

**Type:**

```mbl
type_of(value)                      # text name of the type
is_unknown(value)                   # safe Unknown check
convert(value, type)                # explicit coercion
```

**Collection (higher-order):**

```mbl
sort(collection, key)
filter(collection, condition)
map(collection, procedure)
reduce(collection, procedure, initial)
```

---

## Common Idioms

### Intent-result pattern (PWA interactions)

The client writes an intent to the user's data subtree; a per-user watcher
processes it; the result goes back to the same user's subtree:

```mbl
# Registered per-user at signup
watch my.app.watchers[username & ".publish"](user_path.intent.publish):
    if user_path.intent.publish = true:
        # ... do the work, compute new_id ...
        user_path.intent.publish = (quietly) false
        user_path.intent.result = { status: "done", id: new_id }
```

### TTL for auto-expiring data

Auth tokens, verification codes, cache entries — anything that should vanish:

```mbl
world.apps.myapp.tokens[token_id].identity = "kalevo"
world.apps.myapp.tokens[token_id].@ttl = 604800    # 7 days in seconds
```

No cleanup watcher needed — AmorphDB purges expired instances.

### Denormalized counts via watchers

Keep an aggregate that updates as data arrives:

```mbl
watch append(world.feedback.rankings) as new_rankings:
    for r in new_rankings:
        if r.positive_utility:
            r.feedback.positive_count = r.feedback.positive_count + 1
```

Because counts are ordinary attributes, subscribed PWA clients see updates automatically via SSE.

### Cross-zone transaction records

Multi-zone atomicity isn't available. Use a transaction record:

```mbl
# 1. Record intent
my.transactions..append({
    status: "pending",
    steps: [...],
    initiated: now()
})

# 2. Watcher executes steps, updates status
# 3. Recovery watcher looks for stuck "pending" transactions
watch my.recovery.sweep(world.clock.minute):
    for t in my.transactions[status = "pending", @time < now() - 60000]:
        # Retry or compensate
```

### Pre-rendered content snapshots (for PWAs)

A watcher regenerates an HTML asset on every content change, and the in-memory asset cache serves it sub-millisecond:

```mbl
watch my.snapshots.article(my.articles.current):
    if my.articles.current.published = true:
        rendered = render_article_html(my.articles.current)
        my.computer.network.web.pwa["app.example.com"].assets["article.html"] =
            my.computer.network.web.asset(rendered, "text/html")
```

---

## Common Pitfalls

**1. A self-writing watcher that never settles.** Writing an *unchanged* value does not trigger anything, so most self-writing watchers converge on their own. A watcher that keeps re-firing is usually writing a value that differs every time — a timestamp, a counter — to a path it watches. Wrap that write in `(quietly)`. A runaway chain does not hang the node: the drain is capped and produces an Unknown naming the change that started it.

**2. Confusing `.content` with `.content[@t]`.** `.content` is the current value. `.content[@t]` is the value at time `t`. If your code accidentally queries "current" when you wanted "historical" (or vice versa), you'll get results that look plausible but are wrong. Be explicit.

**3. Treating Unknown as an error.** Unknown propagates through expressions. If you see an Unknown downstream, don't panic — trace it back. Often the right response is to check for Unknown at a specific point and branch, not to try to prevent Unknown from occurring.

**4. Relying on permissions cascading from too far up.** Permissions cascade, but security-critical attributes should set their own permissions explicitly. Don't assume an ancestor's `@read = "Nothing"` protects everything beneath it forever — later refactoring may break that assumption.

**5. Forgetting heartbeat atomicity limits.** Default 10,000 staged writes per tick. A loop that writes a million attributes will hit `unknown("commit buffer exceeded")`. Break large operations into batches processed across multiple ticks.

**6. Using the request queue for PWA clients.** PWA clients use the data-driven model — the Go bridge routes traffic to `world.apps.<app>.users.<identity>.*` based on auth tokens. Per-user watchers handle intents. The request queue is for **external** traffic only: third-party webhooks, OAuth callbacks, REST consumers. Mixing them up creates unnecessary HTTP plumbing for your own app.

**7. Writing to `world.*` without checking permissions.** `world` root defaults to write-nothing. If your watcher tries to write to `world.some.path` and silently fails, check the permissions — you probably need to set `@write` or use a designated admin agent.

**8. Assuming a primitive exists.** If `my.computer.X.Y` isn't documented in the AmorphDB design doc, it probably doesn't exist. Check before building on it.

---

## When to Read the Full Design Doc

This reference covers what most application developers need. Go to `amorphdb_design.md` when you need to understand:

- **Mesh architecture** — how nodes form meshes, bridge connections, write-authority assignment, subscription-based distribution.
- **Storage model details** — attributes, instances, values as distinct layers; storage file organization; archival; compaction.
- **Exact execution model semantics** — outer vs inner run behavior, commit buffer internals, cross-authority coordination mechanics.
- **Security protocol** — post-quantum key exchange, challenge-response authentication, agent-level encryption, recovery.
- **Data extraction and PWA management tooling** — `amorphctl extract`, `amorphctl init-pwa`, `amorphctl deploy-pwa`.
- **PWA user identity and authentication** — device/token/identity model, login/signup flows, per-user watcher registration, security boundary, reference auth scheme.
- **Implementation status of specific features** — what's working, what's stubbed, what's planned (noted with 🚧 markers in the full doc).
- **Wire protocol or network-level details** — for implementing clients or bridges.

If you're an application developer and you find yourself in these sections regularly, either the thing you're building is platform-level work (move to AmorphDB repo), or this quick reference has a gap (tell Matthew).

---

## Implementation Status

The following features are described in this reference but marked 🚧 in `amorphdb_design.md` as not-yet-implemented. Check `DEVPLAN.md` and the design doc's current markers before building on them:

- **Same-line definitions** (`name: value; other: value`) — 🚧
- **Recursive assignment** (auto-creating intermediate nodes) — 🚧
- **Wildcard projections** (`{ price_of_* }`, `{ * }`, `{ *, not field }`) — 🚧
- **Display hints** (`:tree`, `:table`, `:list`) and **slice pagination** in the REPL — 🚧
- **Network sub-library** — path structure exists with stubbed procedures. The full design for outbound HTTP, inbound request queue, SSE, and PWA serving is not yet implemented in code.
- **PWA device/token/identity model** — the design doc specifies device-based pre-auth routing, token-based identity resolution, and per-user watcher registration. The Go bridge currently uses session-ID routing. Conversion is a dedicated development phase.
- **Email sub-library** (`my.computer.network.email`) — send, fetch, and IMAP IDLE subscription specified in the design doc. Not yet implemented.
- **Files sub-library** — `read`/`write` plus XML import/export and ARI fixed-width import are working. JSON, CSV, TSV, and TOML are stubbed.
- **Subscription-based distribution model** — the spec replaces the earlier zone/consistent-hashing model; code is still transitioning. Do not build new features that depend on zone internals.

Everything else described in this reference (core MBL syntax, watchers, permissions, stamps, temporal queries, record literals, heritability, projections, etc.) is specified as stable in `amorphdb_design.md`. Local test suites or `DEVPLAN.md` are the source of truth for what is currently passing in the code at any given moment.
