# AmorphDB — Remaining Work

This document tracks all known remaining work on AmorphDB, organized by
priority. Updated after completing the test plan (Sections 1–12), P0
correctness audit, P1 missing implementations, and P2 feature completeness
(record literals, projections, procedure persistent sub-attributes).

---

## What's Done and Verified

These areas have been tested, audited, and confirmed working correctly:

- **Storage engine:** Values, instances, attributes, deduplication with
  reference counting, children enumeration, concurrent writes, crash recovery
- **Type system:** All 8 core types, 3 meta types (Unknown, Queued, Redacted),
  full type coercion, Unknown/Queued propagation
- **MBL lexer:** All token types, Unicode identifiers, multi-quote strings,
  money literals, time literals, `@` disambiguation, indentation/dedent
  tracking, block comment syntax (`##`, `###`)
- **MBL parser:** Full expression parsing, operator precedence, context-
  sensitive `=`, bracket filters with temporal queries, projections, record
  literals, collection operations (`..count`, `..remove`, `..combine`), all
  control flow (if/else/for/while/break/return/pass), procedures, watchers
  (value-change, append, multi-path, predicate), catch/else, embed, scope
  statements, error recovery with line/column info
- **MBL interpreter:** Expression evaluation, variable scope (local vs
  persistent), control flow, collection operations, 15+ built-in functions,
  bracket queries with AND/OR/temporal, record literal expansion to storage,
  projections, procedure persistent sub-attributes, catch/else exception
  handling
- **Watchers:** Value-change triggers, multi-path with OR semantics and
  deduplication, append watchers with predicate filters, enable/disable,
  watcher attributes (`@enabled`, `@watching`, `@last_run`, etc.)
- **Heartbeat atomicity:** Real staging buffer (CommitBuffer), 10,000 write
  limit, flush on success, discard on unhandled Unknown, watcher cascade
  deferred to next tick, outer run staging
- **shouldRollback():** Correctly triggers only when Unknown escapes watcher
  body unhandled — caught Unknown allows commits
- **Stamps:** Auto-injection of `@agent` and `@time`, custom stamps, stamp
  inheritance, stamp queries
- **Embed:** Basic embed, spread syntax, host-wins collision, later-embed-wins
  collision, multiple embeds, identity preservation
- **Filters:** Hide matching data, client-side isolation, remove/reveal,
  multiple active filters, redacted display
- **Permissions:** All 5 permission types (`@read`, `@write`, `@expand`,
  `@grant`, `@purge`), defaults for `~` and `world`, cascade inheritance,
  agent lists, "Anything"/"Nothing" special values
- **Protocol:** EXECUTE message type for remote MBL execution, full
  encode/decode with checksums
- **Service:** Per-connection MBL interpreter instances, EXECUTE handling
- **Mesh:** Mesh creation, name validation, identity assignment (CV syllables),
  heartbeat (333ms), gossip propagation, failure detection, authority
  management
- **Security:** Diffie-Hellman key exchange, identity generation,
  challenge-response protocol, bridge authentication
- **Purge:** Soft purge with TTL, hard purge, cascade purge, temporal queries
  during soft purge, basic compaction

---

## Remaining Work

### Tier 1 — Required Before Real Use

These block building applications on AmorphDB.

#### 1.1 Projection reading from expanded storage
Record literals expand into individual field paths (`my.person.name`,
`my.person.age`), but projections need to reconstruct records by reading
those expanded paths back. Currently projections parse and partially
evaluate but can't fully reconstruct a record from its expanded fields.

**Where:** `internal/mbl/interpreter/` — projection evaluation logic
**Spec:** "Projections select a subset of sub-attributes from a path"

#### 1.2 ScopeStatement interpreter support
The parser produces `ScopeStatement` AST nodes but the interpreter returns
"unsupported statement type." This means `my.scope.` trailing-dot syntax
for setting scope context doesn't execute.

**Where:** `internal/mbl/interpreter/interpreter.go` — add case for
`*parser.ScopeStatement` in `evalStatement()`
**Spec:** Notation and Language § Suffixes — `.` sets the current scope

#### 1.3 Function call parsing in REPL context
The `amorph` client fails on function call syntax — "unexpected token
RPAREN" errors. Basic expressions work but procedure calls don't parse
correctly through the EXECUTE protocol path.

**Where:** `cmd/amorph/` and `internal/service/connection.go`
**Spec:** Procedures § Calling

#### 1.4 `my.computer.output` and `my.computer.input`
These don't exist yet. Any MBL program that prints output or reads input
will fail. Essential for scripts and REPL interaction.

**Where:** `internal/mbl/interpreter/` — built-in procedure registration
**Spec:** Computer Library § I/O Operations

#### 1.5 Regression failures in existing test packages
12 packages fail on `go test ./...`. None are regressions from the test
plan work — all are pre-existing or build configuration issues. But they
need fixing before the codebase is clean:
- `testutil` — missing `pkg/client` package (empty directory)
- `cmd/amorph` — ScopeStatement and function call issues (see 1.2, 1.3)
- `internal/interpreter` — bridge connectivity logic
- `internal/mbl/interpreter` — ScopeStatement (see 1.2)
- `internal/mbl/lexer` — keyword recognition gaps (`Nothing`, `Anything`)
- `internal/mbl/parser` — watch statement and assignment modifier parsing
- `internal/mesh` — config struct field mismatches, missing zone references
- `internal/security` — filter logic test failures
- `internal/storage` — temporal query and `isVariableLength` test conflicts
- `internal/zone_deprecated` — hash ring replica logic
- `test/integration` — protocol permission issues
- `tests/integration` — config struct and storage API mismatches

#### 1.6 Quiet assignment `(quietly)` integration with watchers
The `(quietly)` modifier is parsed but its integration with the watcher
trigger system needs verification. Watchers should not fire when a path
is written with `(quietly)`.

**Where:** `internal/watcher/` and `internal/mbl/interpreter/`
**Spec:** Watchers § Quiet Assignment

---

### Tier 2 — Important for Feature Completeness

These are specified features not yet implemented. Not strictly blocking
for an initial application but needed for full MBL support.

#### 2.1 Same-line definitions
`my.config.host: "localhost"; my.config.port: 5432`
Multiple statements on the same line separated by semicolons.

**Spec status:** 🚧 explicitly marked not yet implemented

#### 2.2 Recursive assignment
`my.new.deeply.nested.value = 42` should auto-create intermediate nodes.
Currently all intermediate nodes must exist before assignment.

**Spec status:** 🚧 explicitly marked not yet implemented

#### 2.3 Wildcard projections
`my.products{ price_of_* }`, `my.products{ * }`,
`my.products{ *, not internal_code }`

**Spec status:** 🚧 explicitly marked not yet implemented

#### 2.4 Embed keyword in record bodies
```
my.report.data:
    title: "Q3 Report"
    embed my.context.project_stamp
    revenue: $14200
```

**Spec status:** Listed as specified but may not be in interpreter

#### 2.5 Time formatting and parsing functions
`format_time(time, format)` and `parse_time(text, format)`

**Where:** `internal/mbl/interpreter/` — built-in procedure registration

#### 2.6 `convert(value, type)` built-in
Explicit type conversion function.

**Where:** `internal/mbl/interpreter/` — built-in procedure registration

#### 2.7 Collection functions: `sort`, `filter`, `map`, `reduce`
Higher-order collection operations that take procedures as arguments.

**Where:** `internal/mbl/interpreter/` — built-in procedure registration
**Spec:** System Operations § Collection Operations

#### 2.8 `add_time(time, days, hours, minutes)` built-in
Time arithmetic function.

**Where:** `internal/mbl/interpreter/` — built-in procedure registration

---

### Tier 3 — Security Features

Specified in the security section but not yet implemented.

#### 3.1 Agent-level encryption
Agent's private key stored encrypted in mesh with agent's derived key.
Node hosting the data cannot read it.

**Where:** `internal/security/` and `internal/crypto/`
**Spec:** Security § Agent-Level Encryption

#### 3.2 Key rotation
Rotate device secret, re-encrypt stored data, old secret no longer works.

**Spec:** Security § Recovery

#### 3.3 Multiple device secrets
Two devices registered, both can authenticate.

**Spec:** Security § Recovery

#### 3.4 Local socket no-encryption / network socket mandatory encryption
Unix socket connections should skip encryption. TCP connections require
full encryption stack.

**Spec:** Security § Protocol — "The same protocol operates over both
local Unix sockets (no encryption) and network TCP sockets (full
encryption stack)."

---

### Tier 4 — Distributed Systems / Multi-Node

These require the multi-node test harness and in some cases the
subscription-based distribution model that is replacing the zone model.

#### 4.1 Multi-node test harness
Go test harness in `test/multinode/` that uses `exec.Command` to
start/stop `amorphd` instances on different ports.

#### 4.2 Subscription-based data distribution
Replace zone/consistent-hashing with explicit write authority per path,
subscription-based reads, authority promotion on failure.

**Spec status:** Spec describes new model. Code still has old model.
`internal/zone/` is deprecated. `internal/mesh/replication.go` has both
legacy and subscription-based code.

#### 4.3 Write authority routing
Writes route to path authority. Authority splitting when overwhelmed.
Subscription load balancing for fan-out.

#### 4.4 Network partition handling
Split-brain operation during partition, reconciliation on heal.

#### 4.5 Integration tests (Sections 13–14)
End-to-end tests covering single-node lifecycle, multi-node
write/read, distributed watcher pipelines, stress/chaos testing,
and full MBL program tests.

---

### Tier 5 — Future / Design Phase

These are planned but not yet specified in detail.

- MCARS standard interface convention
- WAGLE notation for custom interfaces
- PWA standard component shell
- ARI fixed-width export
- Excel import/export
- XML import/export (partially specified)
- LLM fine-tuning for MBL assistance
- REPL display hints (`:tree`, `:table`, `:list`) and slice pagination
- Auto-inferred table rendering for homogeneous lists

---

## Quick Reference: What Works End-to-End Today

An MBL program can:
- Create and read scalar values at paths (`my.x = 42`)
- Create records from literals (`my.person = { name: "Matt", age: 55 }`)
- Query with bracket filters (`my.users[age > 18]`)
- Use projections (`person{ name, age }`) — partial, needs reconstruction
- Define and call procedures with parameters
- Use persistent procedure sub-attributes
- Set up watchers that fire on data changes
- Use append watchers with predicate filters
- Handle errors with catch/else
- Use all arithmetic, comparison, logical operators
- Use 15+ built-in functions (string, math, type checking)
- Execute remotely via the EXECUTE protocol
- Authenticate with challenge-response
- Operate in a mesh with heartbeat and gossip

An MBL program cannot yet:
- Use `my.computer.output()` or `my.computer.input()`
- Use same-line definitions or recursive assignment
- Use wildcard projections
- Use embed keyword in record bodies
- Use `sort`, `filter`, `map`, `reduce` on collections
- Format or parse times
- Rely on agent-level encryption at rest
- Operate across nodes with subscription-based distribution
