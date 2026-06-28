# CLAUDE.md — AmorphDB Project Context

This file is read by Claude Code at the start of every session. Read it fully
before touching any code. It is the authoritative source of project conventions,
architectural decisions, and standing rules.

---

## What AmorphDB Is

AmorphDB is a temporal tree-graph database written in Go. Every value has a
history — assignment records a change rather than overwriting. Data is organized
as a hierarchy of attributes, each holding a chain of timestamped instances.

The system is accessed through **MBL (Modern Business Language)**, a notation
designed around scope resolution and path traversal. The design philosophy is
**intent not mechanism** — MBL code expresses what it wants, not how to achieve
it. Types are deliberately high-level (Number is always the largest float the
processor supports; Picture is always full RGBA at 16-bit depth).

AmorphDB operates as a decentralized mesh. Every node is a peer. There is no
central coordinator.

**Three components:**
- `amorphd` — the core daemon (storage, mesh networking, MBL execution)
- `amorphctl` — admin tool (connects via local socket)
- `amorph` — user REPL and script runner (connects via local or network socket)

---

## Authoritative Specification Documents

These documents are the source of truth. If code and spec conflict, **the spec
wins** unless there is a documented reason to deviate. Never alter these files
without explicit instruction.

| Document | Location | Purpose |
|---|---|---|
| `amorphdb_design.md` | `docs/amorphdb_design.md` | Master spec: language, storage, mesh, security, computer library, PWA identity |
| `AmorphDB_PWA_Boilerplate_Spec.md` | `docs/AmorphDB_PWA_Boilerplate_Spec.md` | PWA boilerplate: section-based HTML, proxy, data binding, device/token model |
| `mbl_reference.md` | `docs/mbl_reference.md` | MBL quick reference for application developers and AI coding agents |
| `pwa_identity_auth_draft.md` | `docs/pwa_identity_auth_draft.md` | Detailed PWA auth flows: login, signup, logout, OAuth examples |
| `DEVPLAN.md` | `docs/DEVPLAN.md` | Active development plan with step status |
| `pwa_auth_development_plan.md` | `docs/pwa_auth_development_plan.md` | Development plan for PWA identity/auth implementation |
| `pending_code_changes.md` | `docs/pending_code_changes.md` | Tracked terminology/design changes not yet in code |

Read the relevant spec sections before implementing any feature. Do not
implement from memory or assumption.

---

## Repository Structure

```
amorphdb/
├── cmd/
│   ├── amorph/         # client REPL (client.go, format.go, input.go, repl.go)
│   ├── amorphctl/      # admin tool (join, bridge, detach, mesh management)
│   └── amorphd/        # daemon entry point
├── internal/
│   ├── auth/           # bridge authentication
│   ├── config/         # configuration
│   ├── crypto/         # mobile agent cryptography
│   ├── identity/       # agent identity (CV syllable generation, bridge identity)
│   ├── interpreter/    # bridge scope interpreter
│   ├── mbl/
│   │   ├── interpreter/ # MBL execution engine (coordinator, interpreter)
│   │   ├── lexer/       # MBL lexer (lexer.go, tokens.go)
│   │   └── parser/      # MBL parser (ast.go, parser.go)
│   ├── mesh/           # mesh networking (heartbeat, gossip, bootstrap, replication,
│   │                   #   bridge, discovery, leave, manager)
│   ├── protocol/       # wire protocol (encode, decode, messages)
│   ├── security/       # auth protocol, crypto, permissions, stamps, filters, recovery
│   ├── service/        # daemon service layer (connection, mesh integration)
│   ├── storage/        # storage engine (attributes, instances, values, tree,
│   │                   #   agent, bridge, defrag, freelist, hash, hashindex)
│   ├── temporal/       # temporal query support (in progress)
│   ├── types/          # type system (types, coerce, compare, path, serialize, agent)
│   ├── watcher/        # watcher engine (engine, heartbeat integration)
│   └── zone/           # zone/hash-ring (hashring, split) — BEING REPLACED
│                       #   see Data Distribution Model in amorphdb_design.md
├── pkg/
│   └── client/         # public client library
├── docs/               # specification documents — DO NOT MODIFY WITHOUT INSTRUCTION
│   ├── amorphdb_design.md
│   ├── AmorphDB_PWA_Boilerplate_Spec.md
│   ├── mbl_reference.md
│   ├── pwa_identity_auth_draft.md
│   ├── pwa_auth_development_plan.md
│   ├── pending_code_changes.md
│   ├── AmorphDB_Tutorial.md
│   ├── bridge_architecture.md
│   ├── mesh_management_guide.md
│   └── business/       # business planning docs
├── web/                # PWA boilerplate
│   └── boilerplate/
│       ├── amorphdb-pwa.js    # core boilerplate (embedded in amorphd binary)
│       ├── auth/              # reference auth watchers
│       │   ├── login.mbl      # reference login watcher (Argon2id)
│       │   └── signup.mbl     # reference signup watcher
│       └── test/              # test pages and integration tests
│           ├── index.html     # interactive test page
│           ├── integration.html # automated integration test (17/17)
│           ├── app.html       # sample app structure
│           └── amorphdb-pwa.js # test copy (keep in sync with parent)
├── examples/           # example MBL scripts and mesh setup scripts
├── test/               # integration tests (step-based, core, single node, storage)
├── tests/
│   ├── integration/    # cross-package integration tests
│   ├── phase3/         # MBL script tests for performance, security, temporal
│   └── unit/           # MBL script unit tests
├── scripts/            # deployment, VM testing, build scripts
├── CLAUDE.md           # this file
├── DEVPLAN.md          # development plan and step status
├── Makefile
├── go.mod / go.sum
└── README.md
```

**Key notes on actual structure:**
- MBL lives under `internal/mbl/` with three sub-packages: `lexer`, `parser`, `interpreter`
- There is a separate `internal/interpreter/` package for bridge scope — distinct from `internal/mbl/interpreter/`
- The `my.computer` library is fully specified in `amorphdb_design.md` (The Computer section). There is no separate library spec file.
- `internal/zone/` contains the hash ring and zone split logic — this package is being replaced by the new subscription-based distribution model. Do not add new dependencies on it.
- `internal/temporal/` exists but appears empty or in-progress
- `web/boilerplate/` contains the PWA client-side framework. The test copy at `web/boilerplate/test/amorphdb-pwa.js` must stay in sync with `web/boilerplate/amorphdb-pwa.js`.
- Test files are spread across `test/integration/`, `tests/integration/`, `tests/phase3/`, and `tests/unit/` — be aware of both locations

---

## Implementation Status Summary

Use this as a quick reference. The spec documents have full detail.

### ✅ Implemented
- Core storage engine (attributes, instances, values, temporal chains)
- MBL parser and evaluator — core language features (17/17 spec compliance tests)
- Type system (Text, Number, Boolean, Time, Money, Picture, Reference, Procedure, Watcher, Unknown)
- Definite-equality operator `?=` (single operator replacing old `?=`/`?!=`/`?<`/`?>`/`?<=`/`?>=` family)
- Type-first definition syntax as primary (`procedure name():`, `watch name():`) with path-first equivalent
- Watchers (value-change and append forms, multi-path, predicate filter)
- Quiet assignment `(quietly)`
- Heartbeat atomicity and staged write model
- Record literal assignment (`x = { name: "...", age: 55 }`)
- Projection syntax (`path{ name, age }`) — including projection over a stored path
- Same-line definitions (`name: value; name: value`, `double(x): return x * 2`)
- Heritability and instantiation (`new()` with copy/link/reset/exclude)
- Catch/else unknown error handling
- `(cascade)` scope modifier (parser + basic resolution)
- `(link)` reference syntax
- `unknown` / `unknown("reason")` syntax with `x ?= unknown("reason")` matching
- `..append()` and `..prepend()` collection operations
- `&` string concatenation operator
- `pass` statement, `break` statement, block comments
- `my.computer.run()` shell execution
- `my.computer.output()` / `my.computer.input()`
- `my.computer.files` sub-library (read, write, exists, delete, list, info)
- `my.computer.files.import/export` — XML format working, ARI fixed-width working
- `my.computer.network.web` — outbound HTTP (get/post/put/patch/delete), JSON helpers,
  inbound request queue, SSE manager, PWA asset serving, TLS with SNI
- `my.computer.crypto` — hash_password (Argon2id), verify_password, generate_token (CSPRNG)
- `amorphctl extract` — generates MBL scripts from subtrees
- `amorphctl init-pwa` — scaffolds new PWA projects with auth watchers
- Mesh formation, joining, bridge connections
- Security: post-quantum encryption, challenge-response auth, agent-level encryption
- Stamps and filters
- Permission system
- PWA Go bridge — device/token/identity routing, write boundary enforcement,
  SSE routing by identity, login response handling (17/17 integration tests)
- PWA boilerplate (`amorphdb-pwa.js`) — section parser, data tree generator, proxy
  change tracking, surgical DOM updates, list rendering, batch sender, SSE receiver,
  local watchers, deep linking, list windowing, show/hide, local dev mode, device ID,
  token storage, login/signup UI, logout, 401 handling (17/17 integration tests)
- Reference auth watchers (login.mbl, signup.mbl) with Argon2id password scheme

### 🚧 Specified but Not Yet Implemented
- Recursive assignment (auto-create intermediate nodes)
- Wildcard projections (`{ price_of_* }`, `{ * }`, `{ *, not field }`)
- Embed keyword in record bodies (`embed my.record`)
- `my.computer.files.import/export` — JSON, CSV, TSV, TOML formats (stubbed)
- `my.computer.network.email` — send, fetch, IMAP IDLE subscription
- REPL display hints (`:tree`, `:table`, `:list`) and slice pagination
- Auto-inferred table rendering for homogeneous lists in REPL

### 🔄 Planned (Not Yet Specified in Detail)
- Fixed-width export via ARI
- Excel import/export
- Email OAuth 2.0 for Gmail/Microsoft 365
- Component system for PWA (reusable section templates)
- Widget library (date pickers, data tables, charts)
- MCARS standard interface convention
- Examples library
- LLM fine-tuning

### ⚠️ Architecture Change In Progress
- **Mesh distribution model:** The zone/consistent-hashing model in the current
  code is being replaced by a subscription-based model with explicit write
  authority per path. The spec reflects the new model. The code still reflects
  the old model. **Do not build new features that depend on the zone model.**
  See DEVPLAN.md for the migration plan.

---

## Standing Rules — Read Before Every Task

These rules apply to every session, every step, every file touch. They are not
optional and are not overridden by step instructions.

### 1. Never alter code outside your current step's scope
If a step says "implement projection syntax in the parser," do not modify the
evaluator, storage engine, mesh package, or any other package unless the step
explicitly says to. If you notice something that seems wrong elsewhere, add a
`// TODO(devplan): ...` comment and move on. Do not fix it.

### 2. Run the full test suite before and after every change
```bash
go test ./...
```
Record the baseline pass/fail count before you start. If any tests that were
passing before your change are now failing, stop and investigate before
continuing. Do not mark a step complete if it introduces new test failures
unrelated to the step's scope.

### 3. Read a file fully before modifying it
Never make a targeted edit to a file you haven't read in full during this
session. Partial reads lead to accidental deletions of important code. Use
`view` to read the entire file, then edit.

### 4. Do not remove, simplify, or refactor working code
Unless a step explicitly instructs refactoring, leave working code exactly as
it is. "This looks like it could be cleaner" is not a reason to change it. The
codebase has context you may not have. When in doubt, don't touch it.

### 5. The spec is the source of truth
If existing code implements something differently from the spec, note the
discrepancy in a `// TODO(spec): ...` comment. Do not silently change behavior
to match the spec unless the current step specifically covers that discrepancy.

### 6. Mark step status in DEVPLAN.md
- When starting a step: change its status to `IN PROGRESS` with the current date
- When completing a step: change its status to `DONE` with completion date and
  a one-line note on what was done
- If a step is interrupted: leave it as `IN PROGRESS` — do not revert to `TODO`
  The next session will see `IN PROGRESS` and know to check what was completed

### 7. Each step starts by reading DEVPLAN.md
Find the current `IN PROGRESS` step (if any) or the next `TODO` step. Read its
full instructions. Read the spec sections it references. Then begin.

### 8. Write tests for new features
Every new language feature or library procedure needs at least:
- A unit test covering the happy path
- A unit test covering the primary failure/error case
- An integration test if the feature interacts with storage or mesh

Tests go in the same package as the code under test (`_test.go` files) or in
`tests/` for integration tests.

### 9. Test plan bug fixes must match the spec
When fixing bugs found by tests, always fix the implementation to match amorphdb_design.md. Never weaken a test, change test data formats, or alter expected behavior to make a failing test pass. If the spec and the implementation conflict, the spec wins. If unsure whether a test or the implementation is wrong, re-read the spec section and note the question in STATUS.md.

### 10. Every test must pass. 
Do not declare a section complete at 97% or describe failures as "minor." If a test was worth writing, it's worth passing. Fix all failures before reporting completion.

### 11. Do not stop mid-section. 
When given a section to complete, work through all failures until every test passes. Do not stop to report partial progress. A section is either complete (all tests pass) or in progress (keep working).

### 12. Do not skip tests for implemented features. 
Only use t.Skip() for features explicitly marked 🚧 in the spec. If the feature is specified and the implementation exists, write the test and make it pass.

### 13. No TODO placeholders in tests. 
Every test must contain real assertions that verify the feature works end-to-end. A test that only checks parsing without verifying execution is not a valid test. Never use TODO comments to defer the actual verification.

---

## Key Architectural Concepts

A few concepts that are non-obvious and frequently misunderstood:

**Temporal semantics:** Assignment never overwrites. `my.x = 5` followed by
`my.x = 10` results in two instances chained together, both preserved. Queries
can retrieve any historical value. This is fundamental — do not implement
anything that overwrites instances.

**Embed vs. asset:** `embed` is a structural composition keyword that makes one
record's attributes appear flat alongside another's. It is not a data value and
cannot be assigned. `my.computer.network.web.asset(data, mime_type)` is a
procedure that builds a record with `.data` and `.mime_type` fields — used for
PWA static assets. These are two completely different things.

**Heartbeat atomicity:** All writes within a watcher execution are staged and
flushed as a single batch at the heartbeat boundary. This is not optional
behavior — it is how the system achieves consistency without distributed
transactions. Do not change this model.

**Write authority:** Under the new distribution model (not yet in code), each
path has exactly one write authority node. Reads come from local subscribed
copies. Writes route to the authority. See `amorphdb_design.md` §Data
Distribution Model.

**`(quietly)` modifier:** Prevents an assignment from triggering watchers. Used
inside watchers to prevent infinite loops. It is a modifier on the assignment
statement, not a function call.

**`my` vs `world`:** `my` is the current agent's home (`world.agent.{identity}`).
`world` is the global root. `my.world` and `world` are the same thing from any
agent's perspective.

**PWA identity model:** App users are NOT mesh agents. They are managed by the
application. Before login, the browser has a device ID (stored in localStorage).
After login, a token maps the device to a user identity. The Go bridge routes
writes to `world.apps.<app>.devices.<deviceId>.*` pre-auth and to
`world.apps.<app>.users.<identity>.*` post-auth. Per-user watchers handle
application logic — one watcher per user per action, registered at signup.
See `amorphdb_design.md` §PWA User Identity and Authentication.

**PWA boilerplate architecture:** The client-side boilerplate (`amorphdb-pwa.js`)
uses `<section>` tags in `app.html` to define UI structure. `{fieldname}`
placeholders create the data tree automatically. A JavaScript Proxy tracks all
data changes and batches them to the server via MCP/POST. Server changes arrive
via SSE and update the DOM surgically via `data-bind` attributes. The boilerplate
is embedded in the `amorphd` binary and served at `/amorphdb/pwa.js`.
See `AmorphDB_PWA_Boilerplate_Spec.md`.

---

## Go Conventions for This Project

- Standard Go formatting (`gofmt` / `goimports`) — run before committing
- Error handling: return errors explicitly, do not panic except for truly
  unrecoverable situations (e.g., corrupt storage file at startup)
- Package names: short, lowercase, no underscores
- Test files: `foo_test.go` in the same package, or `foo_integration_test.go`
  in `tests/` for cross-package tests
- Comments on exported symbols follow Go doc convention (`// FuncName does...`)
- Internal implementation comments are encouraged — this is a complex system
  and future maintainers (including future AI sessions) need context

---

## What To Do If You Are Unsure

1. Re-read the relevant spec section
2. Re-read this file
3. Re-read DEVPLAN.md for the current step's instructions
4. If still unsure, add a `// TODO(clarify): ...` comment at the point of
   uncertainty, complete what you can, and note the uncertainty in DEVPLAN.md
   under the current step's entry

Do not guess at architectural decisions. Do not invent behavior not specified.
Do not silently skip parts of a step.
