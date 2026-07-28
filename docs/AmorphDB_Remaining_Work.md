# AmorphDB — Remaining Work

Known remaining work on AmorphDB, organized by priority.

**Last verified: 2026-07-28** against a full `go test ./...` run and direct
inspection of the code. Items marked *verified* were checked in that pass;
items marked *inherited* come from earlier status notes and have not been
re-confirmed.

> This document ships in release archives. It is a status report, not a
> specification — `docs/amorphdb_design.md` is the source of truth for
> intended behavior.

---

## Current State

**Build and tests: green.** `go build ./...` is clean and `go test ./...`
passes across all 25 test packages with 0 failures. *(verified)*

The system runs end to end: a daemon starts, serves a REPL over a local
socket, persists to disk, forms a mesh, authenticates network clients, and
serves a PWA over HTTP.

### What works today

- **Storage engine** — values, instances, attributes, deduplication with
  reference counting, children enumeration, concurrent writes, crash
  recovery, leaf-path enumeration
- **Type system** — all core types plus the meta types (Unknown, Queued,
  Redacted), coercion, Unknown/Queued propagation
- **MBL lexer** — all token types, Unicode identifiers, multi-quote strings,
  money and time literals, `@` disambiguation, indentation tracking, block
  comments
- **MBL parser** — full expression parsing, operator precedence,
  context-sensitive `=`, definite equality `?=`, bracket filters with
  temporal queries, projections, record literals, collection operations,
  all control flow, procedures, watchers, catch/else, embed, scope
  statements, error recovery with line/column info
- **MBL interpreter** — expression evaluation, variable scope, control flow
  (including correct early `return`), collection operations, 15+ built-ins,
  bracket queries with AND/OR/temporal, record literal expansion,
  projections over both in-memory records and stored paths, scope
  statements, catch/else, call-depth guard against runaway recursion
- **Bracket key-selectors** — in assignment targets and in terminal read
  position, with static and dynamic keys
- **Watchers** — value-change and append forms, multi-path with OR semantics
  and deduplication, predicate filters, enable/disable, watcher attributes
- **Heartbeat atomicity** — real staging buffer, write limit, flush on
  success, discard on unhandled Unknown, watcher cascade deferred to the
  next tick
- **Stamps, filters, permissions** — auto-injected `@agent`/`@time`, custom
  and hierarchical stamps, hide-on-match filters, all five permission types
  with cascade inheritance, grants round-tripping through the daemon
- **Mesh** — formation, joining, CV-syllable identity, heartbeat, gossip,
  failure detection, path directory, subscription registry, write authority
  with promotion and delegation
- **Distribution** — subscription-based replication is the only model; the
  zone/hash-ring model has been retired to `internal/zone_deprecated/`
- **Security** — ElGamal/DH challenge-response, agent-level encryption,
  bridge authentication, owner bootstrap, grant-gated enrollment
- **Authentication** — owner genesis via `amorphd init-owner`, local-socket
  auto-auth, network challenge-response handshake, invite-based enrollment
  (`amorphctl invite`, `amorph enroll`/`login`) with client-side keygen —
  the private key never reaches the daemon
- **File I/O** — `my.computer.files` (read, write, exists, delete, list,
  info), XML import/export, ARI fixed-width import
- **Web** — outbound HTTP verbs, JSON helpers, inbound request queue, SSE,
  TLS with SNI, PWA asset serving including nested paths
- **PWA** — client boilerplate served from the binary at `/amorphdb/pwa.js`,
  Go bridge with device/token/identity routing, `deploy_pwa()` writer,
  domain discovery, Argon2id password helpers
- **Purge** — soft purge with TTL, hard purge, cascade purge, temporal
  queries during soft purge, basic compaction

---

## Remaining Work

### Tier 1 — Blocks building real applications

#### 1.1 Reference auth watchers do not run end to end
`web/boilerplate/auth/login.mbl` and `signup.mbl` are written but cannot
execute. Three parser gaps stand in the way:

- **Trailing field access after a bracket** — `tokens[token].identity` and
  `users[u].intent.logout` drop the field after the bracket.
  `parseCallExpression` discards the field name following a non-path
  expression.
- **Bracket selectors in watch names and paths** — `watch my.h[user](...)`
  does not resolve.
- **Record reconstruction on read** — a dotted read of a record returns no
  fields, so a whole-record read cannot stand in for the above.

Until these land, PWA authentication must be driven from the Go bridge
rather than from MBL watchers.

**Where:** `internal/mbl/parser/parser.go`, `internal/mbl/interpreter/`

#### 1.2 `(quietly)` does not suppress watcher firing *(verified)*
The modifier lexes and parses (`parser.go:1864`, `parseQuietlyExpression`)
but is never plumbed through to the watcher engine —
`watcher.WatcherEngine.RecordChange(path string)` takes only a path and has
no suppression parameter, and the interpreter's assignment path consults
`node.Modifier` only for `cascade` and the heritability modifiers.

This matters: `(quietly)` is the documented mechanism for preventing
infinite watcher loops, so a watcher that writes to a path it observes will
re-trigger itself.

**Where:** `internal/mbl/interpreter/interpreter.go` (assignment eval),
`internal/watcher/engine.go`
**Spec:** Watchers § Quiet Assignment

#### 1.3 Collection higher-order functions *(verified absent)*
`sort`, `filter`, `map`, and `reduce` are not dispatched by the
interpreter. Working with collections beyond `..count`, `..append`,
`..prepend`, `..remove`, and `..combine` requires explicit loops.

**Where:** `internal/mbl/interpreter/` — built-in dispatch
**Spec:** System Operations § Collection Operations

---

### Tier 2 — Specified but not implemented

#### 2.1 Wildcard projections
`{ price_of_* }`, `{ * }`, `{ *, not field }`. Named-field projections work.

#### 2.2 Embed resolution in record bodies
`embed my.record` and the `...path` spread form parse and produce AST nodes,
but embedded attributes are not made visible during record reads. The
structural resolution layer is unbuilt.

#### 2.3 Time and conversion built-ins *(verified absent)*
`format_time(time, format)`, `parse_time(text, format)`,
`add_time(time, days, hours, minutes)`, and `convert(value, type)` are not
dispatched.

#### 2.4 Import/export formats
JSON, CSV, TSV, and TOML are stubbed in `my.computer.files.import/export`.
XML and ARI fixed-width import work. Fixed-width *export* via ARI is not
built.

#### 2.5 Email library
`my.computer.network.email` — send, fetch, IMAP IDLE subscription.

#### 2.6 REPL display hints and pagination
`:tree`, `:table`, `:list` hints and slice pagination; auto-inferred table
rendering for homogeneous lists.

---

### Tier 3 — Security hardening *(inherited)*

#### 3.1 Key rotation
Rotate a device secret, re-encrypt stored data, invalidate the old secret.

#### 3.2 Multiple device secrets per agent
Register two devices; both authenticate. The spec's §Recovery describes
this; enrollment currently provisions one device secret per agent.

#### 3.3 Transport encryption policy
Local Unix socket connections should skip encryption; TCP connections
should require the full encryption stack. The handshake exists; the
policy split is not enforced.

#### 3.4 Credential handling polish
Passphrase entry is not masked (no `x/term` dependency). Agent homes are
keyed by numeric ID (`world.agent.{number}`); readable CV-string homes are
deferred.

---

### Tier 4 — Distributed systems

#### 4.1 Multi-node test harness
A harness that starts and stops real `amorphd` processes on separate ports.
Current multi-node coverage is in-process.

> ⚠️ **Resource warning.** A past stress run started 10 VMs at ~600 MB each
> and froze the host. Never exceed 2–3 concurrent VMs without checking
> available memory first.

#### 4.2 Network partition handling
Split-brain operation during a partition and reconciliation on heal.

#### 4.3 Authority splitting under load
Write-rate-triggered authority splitting and subscription load balancing
for fan-out. Announcement, promotion, and voluntary delegation are built.

---

### Tier 5 — Planned, not yet specified in detail

- MCARS standard interface convention
- WAGLE notation for custom interfaces
- PWA component system (reusable section templates) and widget library
  (date pickers, data tables, charts)
- Excel import/export
- Email OAuth 2.0 for Gmail / Microsoft 365
- Examples library
- LLM fine-tuning for MBL assistance

---

## Known Rough Edges

- **`internal/temporal/`** is empty or in progress; temporal query support
  lives in the storage and interpreter layers.
- **`cmd` package port binding** — two test-plan tests bind a fixed port
  5000 and a shared socket path, so they can flake under full-suite
  parallel load. They pass in isolation.
- **`go vet`** reports a context-cancel leak on an error path in
  `internal/service/service.go`.
- **Several files are not gofmt-clean** (`coordinator.go` and a handful of
  test files). Left alone to avoid noise in unrelated diffs.
- **Root `DEVPLAN.md` is historical** — all 16 of its steps completed in
  April 2026. Work since then is tracked in commit history rather than in a
  plan document.
