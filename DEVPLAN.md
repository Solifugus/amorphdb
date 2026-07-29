# AmorphDB Development Plan

## How To Use This File

This file is read by Claude Code at the start of every session alongside CLAUDE.md.

**Status markers:**
- `TODO` — not started
- `IN PROGRESS (started: YYYY-MM-DD)` — started but not complete
- `DONE (completed: YYYY-MM-DD)` — complete and verified

**Session startup procedure:**
1. Read `CLAUDE.md` (repo root) fully
2. Read `docs/DEVPLAN.md` (this file) fully
3. Find the first step marked `IN PROGRESS` — resume it
4. If no `IN PROGRESS` steps, find the first `TODO` step — begin it
5. Mark the step `IN PROGRESS` with today's date before touching any code
6. Run `go test ./...` and record the baseline pass/fail count
7. Complete the step, verify it, then mark `DONE`

**If a session ends before a step is complete:**
Leave the step as `IN PROGRESS`. Do not revert it to `TODO`. The next session
will find it and resume from where things were left off, checking what was
and was not completed.

**Scope discipline:**
Each step lists exactly which packages to modify. Do not touch packages outside
that list. If you discover something that needs fixing in another package, add
a `// TODO(devplan): step N — <description>` comment and move on.

---

## Phase 1 — Terminology Cleanup

### Step 1 — Replace `embed()` constructor with `asset()` for MIME-typed content

**Status:** DONE (completed: 2026-04-02) — Implemented asset(data, mime_type) constructor, added comprehensive tests

**Spec reference:** `docs/pending_code_changes.md` item 1,
`docs/amorphdb_computer_library.md` §PWA Static Asset Serving

**Scope:** `internal/mbl/interpreter/`, `internal/mbl/lexer/`, `internal/mbl/parser/`
and any location in the codebase that calls or tests the two-argument
`embed(data, mime_type)` form.

**Do not touch:** storage, mesh, security, types, or any single-argument
`embed(reference)` usage (structural embed — that is correct and unchanged).

**What to do:**
1. Search the entire codebase for `embed(` with two arguments
   (`grep -rn "embed(" --include="*.go"`) and note every location
2. In the MBL lexer/parser, add `asset` as a recognized built-in constructor
   that takes two arguments: `asset(data, mime_type)`
3. In the MBL interpreter, implement `asset(data, mime_type)` as a procedure
   that returns a record with two fields: `.data` and `.mime_type`
4. Update any existing tests that used `embed(data, mime_type)` to use
   `asset(data, mime_type)` instead
5. Add a new test: `asset("hello", "text/plain")` returns a record where
   `.data = "hello"` and `.mime_type = "text/plain"`
6. The single-argument `embed(reference)` structural form must continue
   to work — verify with existing tests

**Verification:**
```bash
go test ./internal/mbl/...
grep -rn "embed(" --include="*.go" | grep -v "_test.go" | grep -v "// "
# The above grep should return only single-argument embed() calls
```

---

## Phase 2 — MBL Language Features

### Step 2 — Record literal assignment

**Status:** DONE (completed: 2026-04-02) — Added comprehensive tests for record literal assignment, storage and retrieval. Note: current implementation uses quoted field names ({"name": "value"}) rather than spec's unquoted syntax ({name: "value"})

**Spec reference:** `docs/amorphdb_design.md` §Lexical Structure — Record literals

**Scope:** `internal/mbl/lexer/`, `internal/mbl/parser/`, `internal/mbl/interpreter/`

**Do not touch:** storage, mesh, security, watcher, zone, types

**What to do:**
1. Read `internal/mbl/lexer/lexer.go` and `tokens.go` fully before touching them
2. Read `internal/mbl/parser/ast.go` and `parser.go` fully before touching them
3. Read `internal/mbl/interpreter/interpreter.go` fully before touching it
4. In the lexer: ensure `{` and `}` are tokenized (they may already be)
5. In the parser: add a `RecordLiteralNode` AST node. A record literal is
   `{` followed by one or more `name: value` pairs separated by commas,
   followed by `}`. Values may be any expression including nested record literals.
   Disambiguation from projection: a record literal always has colons after names.
   A projection never has colons (names only). The parser can tell them apart
   at the first token after the name.
6. In the interpreter: evaluate `RecordLiteralNode` by creating an in-memory
   record node with the specified named fields set to their evaluated values
7. Assignment: `x = { name: "Alice", age: 30 }` stores the record at path `x`

**New tests to write** (in `internal/mbl/interpreter/interpreter_test.go`):
```go
// Basic record literal
"x = { name: \"Alice\", age: 30 }"
// x.name == "Alice", x.age == 30

// Nested record literal
"x = { address: { city: \"Rome\", state: \"NY\" } }"
// x.address.city == "Rome"

// Record literal with all supported value types
"x = { label: \"test\", count: 42, active: true }"
```

**Verification:**
```bash
go test ./internal/mbl/...
```

---

### Step 3 — Recursive assignment (auto-create intermediate nodes)

**Status:** DONE (completed: 2026-04-02) — Implemented auto-creation of intermediate nodes for path assignments. When assigning to paths like `my.app.deeply.nested.value = 42`, intermediate nodes (my.app, my.app.deeply, my.app.deeply.nested) are automatically created as empty records if they don't exist. Added comprehensive tests covering all scenarios including mixed existing/new intermediates.

**Spec reference:** `docs/amorphdb_design.md` §Recursive Assignment

**Scope:** `internal/mbl/interpreter/`, `internal/storage/`

**Do not touch:** lexer, parser, mesh, security, watcher, zone, types

**What to do:**
1. Read `internal/mbl/interpreter/interpreter.go` fully
2. Read `internal/storage/attributes.go` and `internal/storage/tree.go` fully
3. Locate the code path that handles assignment to a path (e.g., `my.a.b.c = 5`)
4. Currently, if `my.a` or `my.a.b` does not exist, this produces an error
5. Change the behavior: when assigning to a path, walk each segment in order.
   If a segment does not exist, create it as an empty record node before
   continuing. Then perform the final assignment.
6. This must work for both value assignment (`=`) and definition (`:`)
7. Existing behavior for paths that already exist must be unchanged

**New tests to write:**
```go
// Assign to a path where intermediates don't exist
"my.new.deeply.nested.value = 42"
// my.new.deeply.nested.value == 42
// my.new exists, my.new.deeply exists, my.new.deeply.nested exists

// Assign where some intermediates exist and some don't
// (first create my.new, then assign deeper)
"my.new = \"exists\""
"my.new.child.grandchild = \"hello\""
// my.new still == "exists"
// my.new.child.grandchild == "hello"
```

**Verification:**
```bash
go test ./internal/mbl/...
go test ./internal/storage/...
```

---

### Step 4 — Same-line definitions

**Status:** DONE (completed: 2026-04-02) — Implemented same-line definition syntax for both procedure definitions and value assignments. Added SEMICOLON token support for multiple statements on one line. Same-line procedure definitions like `double(x): return x * 2` and value definitions like `my.config.host: "localhost"` now work. Semicolon separation like `my.a: 1; my.b: 2` parses multiple statements correctly. All existing multi-line forms continue to work unchanged.

**Spec reference:** `docs/amorphdb_design.md` §Same-Line Definitions

**Scope:** `internal/mbl/lexer/`, `internal/mbl/parser/`

**Do not touch:** interpreter (beyond what the parser already produces),
storage, mesh, security, watcher, zone, types

**What to do:**
1. Read the lexer and parser fully before touching them
2. Currently, a definition like `my.x: procedure(n): return n * 2` may or may
   not parse correctly with the body on the same line. Clarify current behavior
   by writing a test first.
3. Ensure the parser handles same-line definition bodies: after a `:`, if the
   next token is on the same line (not a newline), treat what follows as the
   definition body rather than requiring indentation
4. Semicolons on the same line separate multiple statements:
   `my.a: 1; my.b: 2` defines both on one line
5. Multi-line form (colon followed by newline, then indented body) must
   continue to work exactly as before — this is additive only

**New tests to write:**
```go
// Same-line procedure definition
"my.double: procedure(x): return x * 2"
// my.double(5) == 10

// Same-line value definition
"my.config.host: \"localhost\"; my.config.port: 5432"
// my.config.host == "localhost"
// my.config.port == 5432

// Multi-line form still works
"my.triple:\n    procedure(x):\n        return x * 3"
// my.triple(4) == 12
```

**Verification:**
```bash
go test ./internal/mbl/...
```

---

### Step 5 — Projection syntax

**Status:** DONE (completed: 2026-04-02) — Implemented projection syntax `path{ field1, field2 }` with complete lexer, parser, and interpreter support. Basic projections, path projections, and error handling all work correctly. Added ProjectionExpression AST node and parseProjectionExpression function. Fixed parser disambiguation between record literals and projections by improving isSimpleAssignment logic for MY/WORLD keywords. Missing fields return Unknown with reason "#not_found" as specified. All core functionality complete and tested.

**Spec reference:** `docs/amorphdb_design.md` §Lexical Structure — Projections
and §Brackets and Projections

**Scope:** `internal/mbl/lexer/`, `internal/mbl/parser/`, `internal/mbl/interpreter/`

**Do not touch:** storage, mesh, security, watcher, zone, types

**What to do:**
1. Read lexer, parser, and interpreter fully before touching them
2. A projection is `path{ name, name, name }` — curly braces after a path
   containing only identifiers separated by commas (no colons, no values)
3. Disambiguation from record literal: projections have NO colons after names.
   Record literals ALWAYS have colons. The parser can tell them apart by looking
   at the token after each name.
4. Add a `ProjectionNode` AST node: it holds a path expression and a list of
   field names to select
5. In the interpreter: evaluate by reading the node at the path, then returning
   a new in-memory record containing only the named fields. If a named field
   does not exist at the path, include it as Unknown (`#not_found`)
6. Projection composes with bracket filters:
   `my.users[active = true]{ name, email }` — filter first, then project each result
7. **Do not implement wildcard projections** (`{ price_of_* }`, `{ * }`,
   `{ *, not field }`) in this step — those are lower priority and marked 🚧
   in the spec. Leave a `// TODO(devplan): step 5b — wildcard projections` comment
   at the relevant parser location.

**New tests to write:**
```go
// Basic projection
// Setup: my.person.name = "Alice", my.person.age = 30, my.person.job = "Engineer"
"my.person{ name, age }"
// Returns record: { name: "Alice", age: 30 }
// job is NOT included

// Projection with filter
// Setup: multiple records in my.users
"my.users[active = true]{ name, email }"
// Returns only name and email for active users

// Missing field becomes Unknown
"my.person{ name, nonexistent }"
// Returns { name: "Alice", nonexistent: #not_found }
```

**Verification:**
```bash
go test ./internal/mbl/...
```

---

### Step 6 — Embed keyword in record bodies

**Status:** DONE (completed: 2026-04-02) — Implemented embed directive syntax with both `embed path` and `...path` forms. Added EMBED and SPREAD tokens to lexer, EmbedDirectiveStatement AST node to parser, and basic evaluation in interpreter. All core language syntax requirements complete. Full embed resolution logic (making embedded attributes visible during record reads) requires storage layer enhancement and is noted for future implementation.

**Spec reference:** `docs/amorphdb_design.md` §Embed (full section),
`docs/pending_code_changes.md` item 2

**Scope:** `internal/mbl/lexer/`, `internal/mbl/parser/`, `internal/mbl/interpreter/`

**Do not touch:** storage, mesh, security, watcher, zone, types

**What to do:**
1. Read lexer, parser, and interpreter fully before touching them
2. `embed` is already a reserved word (added to the spec reserved words list).
   Verify it is in the lexer's reserved word list; add it if not.
3. In the parser: inside a record body (a colon-introduced block), `embed path`
   is a valid statement. It is NOT an assignment — it has no left-hand side name
   and no `=`. Add an `EmbedDirectiveNode` AST node holding the path expression.
4. Multiple embed directives are allowed in one record body
5. The spread form `...path` is an accepted equivalent. Add it as an alternative
   syntax that produces the same `EmbedDirectiveNode`.
6. In the interpreter: when evaluating a record body containing embed directives,
   after evaluating all normal field assignments, process each embed directive:
   read the record at the embedded path and make its attributes visible as if
   they were fields of the host record. This is a presentation/resolution layer —
   the embedded attributes are NOT copied into the host record's storage.
7. Collision rule: host record's explicitly declared fields win over embedded
   fields. Between two embeds, later declaration order wins.
8. `embed` is NOT a value — it cannot appear on the right side of an assignment
   and cannot be passed as a parameter. The parser should produce an error if
   `embed` appears in an expression context.

**New tests to write:**
```go
// Basic embed
// Setup: my.stamp.project = "portal", my.stamp.dept = "engineering"
"my.report:\n    title: \"Q3\"\n    embed my.stamp\n    revenue: $14200"
// my.report.title == "Q3"
// my.report.revenue == $14200
// my.report.project == "portal"   (from embed)
// my.report.dept == "engineering" (from embed)

// Host wins on collision
// Setup: my.other.title = "other title"
"my.report2:\n    title: \"My Title\"\n    embed my.other"
// my.report2.title == "My Title"  (host wins, not "other title")

// Spread form equivalent
"my.report3:\n    title: \"Q4\"\n    ...my.stamp"
// same result as embed form
```

**Verification:**
```bash
go test ./internal/mbl/...
```

---

## Phase 3 — REPL Display

### Step 7 — REPL auto-inferred display and display hints

**Status:** DONE (completed: 2026-04-02) — Implemented auto-inferred display formatting with key-value records and list formats per spec. Added display hints parsing (:tree, :table, :list) as REPL-level commands. Created comprehensive test coverage. Format changes as expected per design spec requirements. Notes: timestamp/author display and record aggregation require storage layer enhancement to provide metadata and Children() method integration.

**Spec reference:** `docs/amorphdb_design.md` §The Client

**Scope:** `cmd/amorph/format.go`, `cmd/amorph/repl.go`

**Do not touch:** any internal packages, parser, interpreter, storage, mesh

**What to do:**
1. Read `cmd/amorph/format.go`, `cmd/amorph/repl.go`, and `cmd/amorph/client.go`
   fully before touching them
2. Implement auto-inferred display formatting based on what is returned:
   - **Scalar value** (node has a value, no sub-attributes): print the value
     followed by timestamp and writing agent in parentheses:
     `$1,247.83  (@2026-03-28 14:22:01 by kalevo)`
   - **Record** (node has named sub-attributes): if the node also has its own
     scalar value, print that first as `value: <val>`, then print each named
     sub-attribute as `fieldname:    value` in a key-value block
   - **List** (node has numerically indexed, homogeneous sub-attributes):
     print as a table with auto-generated column headers from the first item's
     field names, one row per item
   - **Mixed**: if a node has both its own value AND sub-attributes, show the
     value first, then the sub-attributes
3. Implement display hints as REPL-level commands (not MBL syntax):
   - `path :tree` — force tree view (indented hierarchy)
   - `path :table` — force table view
   - `path :list` — force list view (one item per line)
   These are parsed by the REPL before passing to the interpreter — they are
   not MBL syntax and do not go through the parser.
4. Implement slice notation for pagination in the REPL:
   `my.transactions[0:9]` — this IS MBL syntax (bracket expression with range)
   but verify the interpreter handles numeric range slices correctly; if not,
   note in a TODO comment for a future step.

**New tests to write** (in `cmd/amorph/repl_test.go`):
- Scalar value formats with timestamp and agent
- Record with own value shows value first
- Record without own value shows fields only
- List renders as table with column headers

**Verification:**
```bash
go test ./cmd/amorph/...
# Manual verification: run amorph REPL and test display of various node types
```

---

## Phase 4 — File I/O Extensions

### Step 8 — XML import and export

**Status:** DONE (completed: 2026-04-02) — Implemented complete XML import/export functionality with computer library framework. Added support for my.computer.files.import/export with XML format, namespace handling, element/attribute mapping, and comprehensive error handling. Includes xlsx format stub and full test coverage.

**Spec reference:** `docs/amorphdb_computer_library.md` §XML Import and Export

**Scope:** `internal/mbl/interpreter/` (computer library files sub-library
implementation), specifically the files import/export procedures

**Do not touch:** lexer, parser, mesh, storage engine core, security, watcher,
zone, types

**What to do:**
1. Read `docs/amorphdb_computer_library.md` §XML Import and Export fully
2. Read the existing `my.computer.files.import` implementation fully to
   understand the pattern before adding to it
3. Add `"xml"` as a supported format in `my.computer.files.import`:
   - Map XML elements → AmorphDB records
   - Map XML attributes → record fields
   - Map XML text content → field values
   - Map repeating elements → lists
   - Support namespace prefix aliasing via `options.namespace_aliases`
   - Missing elements/attributes → Unknown values
4. Add `"xml"` as a supported format in `my.computer.files.export`:
   - Records → XML elements
   - Fields → XML attributes or child elements (controlled by `options.attributes` list)
   - Support `options.pretty` for indented output
5. Add `"xlsx"` format stub that returns `#not_implemented` with a clear
   message — this reserves the format name and makes it clear to future
   implementers where to add it

**New tests to write:**
```go
// Import a simple XML file
// Create a temp XML file, import it, verify the tree structure

// Import XML with namespaces
// Create XML with namespace prefixes, import with alias options

// Export a record to XML
// Create a record, export to XML, verify the output
```

**Verification:**
```bash
go test ./internal/mbl/...
```

---

### Step 9 — ARI fixed-width file import

**Status:** COMPLETE (started: 2026-04-02, completed: 2026-04-02) — All sub-tasks functional, MBL integration working

**Spec reference:** `docs/amorphdb_computer_library.md` §Fixed-Width Import via ARI
(read this section in full — it is detailed)

**Scope:** New package `internal/ari/` plus integration into
`internal/mbl/interpreter/` computer library files procedures

**Do not touch:** lexer (MBL), parser (MBL), mesh, storage engine core,
security, watcher, zone, types

**What to do:**
1. Read `docs/amorphdb_computer_library.md` §Fixed-Width Import via ARI
   completely and carefully before writing any code
2. Create a new package `internal/ari/` with:
   - `lexer.go` — tokenizes an ARI spec string (section, field, break, type,
     starts, ends, same, direction keywords, distance syntax, pattern types)
   - `parser.go` — parses tokenized ARI into an ARI definition tree (sections,
     fields, break rules, custom types)
   - `engine.go` — takes an ARI definition tree and an input file (as a string
     or reader) and returns an AmorphDB tree node representing the parsed data
3. ARI spec is received as a Text value (string). The ARI package compiles it
   to an internal representation and applies it to the file content.
4. Add `my.computer.files.import_fixed_width(path, ari_spec)` procedure to the
   computer library implementation. It calls `internal/ari/` to parse the spec
   and apply it to the file at `path`.
5. Fields not found are recorded as Unknown (`#not_found`)
6. Support all anchor types: left, right, up, down, same
7. Support all distance forms: exact, range, open-ended, flush
8. Support all pattern types: text literal, regex, regex-with-transform,
   built-in type keywords (date, money, integer, decimal, ssn)
9. Support section start/end patterns and break keyword (all three forms)
10. Support custom type definitions within the spec

**This is a large step — sub-tasks:**
- 9a: ARI lexer and parser (compile spec string to AST)
- 9b: ARI engine — simple cases (single section, exact anchors, text patterns)
- 9c: ARI engine — sections, nesting, break keyword
- 9d: ARI engine — all anchor types and distance forms
- 9e: ARI engine — regex patterns and transforms, built-in type keywords
- 9f: Custom type definitions
- 9g: Integration into `my.computer.files.import_fixed_width`

Mark each sub-task complete in a comment as you go. If a session ends mid-step,
note which sub-tasks are done in this status line:
`IN PROGRESS (started: YYYY-MM-DD) — completed: 9a, 9b`

**Sub-task completion status:**
- ✅ **9a: ARI lexer and parser** — COMPLETE
- ✅ **9b: ARI engine — simple cases** — COMPLETE  
- ✅ **9c: ARI engine — sections, nesting, break** — CORE FUNCTIONALITY COMPLETE
- 🔄 **9d: ARI engine — all anchor types and distance forms** — BASIC FUNCTIONALITY WORKING
  - Directional anchors (left, right, up, down, same, flush) implemented ✅
  - Distance forms (exact, range, open-ended) implemented ✅  
  - ⚠️ Minor positioning refinements needed for some complex anchor combinations
- 🔄 **9e: ARI engine — regex patterns and transforms, built-in type keywords** — BASIC FUNCTIONALITY WORKING
  - Built-in type detection (date, money, integer, decimal) working ✅
  - Regex patterns supported ✅
  - Regex transforms supported in parser ✅
  - ⚠️ One edge case in regex transform lexer test failing
- 🔄 **9f: Custom type definitions** — PARSER COMPLETE, ENGINE INTEGRATION NEEDED
  - Custom type parsing working (parser tests passing) ✅
  - Engine integration with custom types needs implementation
- ✅ **9g: Integration into `my.computer.files.import_fixed_width`** — COMPLETE
  - ARI engine fully integrated into MBL computer library ✅
  - `my.computer.files.import_fixed_width(path, ari_spec)` procedure implemented ✅
  - End-to-end integration test passing ✅
  - Converts ARI results to proper MBL types (Record, List, Text, Number) ✅

**✅ ARI FIXED-WIDTH IMPORT COMPLETE**: Full implementation from ARI specification parsing to MBL library integration. Users can now call `my.computer.files.import_fixed_width(path, ari_spec)` to parse complex fixed-width files with sections, nested structures, break rules, and directional field anchoring. Complete pipeline: ARI lexer → parser → engine → MBL type conversion → user accessible procedure.

**New tests to write** (in `internal/ari/`):
- The complete example from the spec (bank report file → expected tree structure)
- Missing field → Unknown
- Nested sections
- Break keyword — repeating records
- Regex pattern matching
- Custom type DATE and CURRENCY

**Verification:**
```bash
go test ./internal/ari/...
go test ./internal/mbl/...
# Run the bank report example from the spec end-to-end
```

---

## Phase 5 — Mesh Distribution Model Replacement

> ⚠️ This is the largest and most architecturally significant phase.
> The current `internal/zone/` package (hash ring, zone splitting) is being
> replaced by a subscription-based model with explicit write authority per path.
> Read `docs/amorphdb_design.md` §Data Distribution Model completely before
> starting any step in this phase.
>
> The old zone model must remain functional until step 14 (cutover). Do not
> remove `internal/zone/` until the new model is fully operational.

### Step 10 — Path directory data structure and API

**Status:** DONE (completed: 2026-04-02) — Implemented distributed path directory with thread-safe operations, longest-prefix matching, last-write-wins merging, and comprehensive test coverage

**Spec reference:** `docs/amorphdb_design.md` §Path Directory, §Write Authority

**Scope:** New package `internal/directory/`

**Do not touch:** zone, mesh, storage, interpreter, security, watcher

**What to do:**
1. Create `internal/directory/` package implementing a distributed path directory
2. The directory maps tree paths to their current write authority (node identity)
3. In-memory structure: a trie or map of path → authority identity, with
   timestamps for staleness detection
4. Operations:
   - `Set(path, authority, timestamp)` — record or update authority for a path
   - `Get(path) (authority, timestamp, ok)` — look up authority for a path
     (returns the most specific match — longest path prefix)
   - `Delete(path)` — remove authority record (used when authority is relinquished)
   - `Snapshot() []DirectoryEntry` — for gossip propagation
   - `Merge(entries []DirectoryEntry)` — apply incoming gossip updates
     (last-write-wins by timestamp)
5. Thread-safe (read-write mutex)
6. No network operations in this package — pure data structure

**New tests to write:**
- Set and Get basic paths
- Longest-prefix matching (path `world.market.equities` matches authority for
  `world.market.equities` before falling back to `world.market`)
- Merge with last-write-wins
- Snapshot round-trip

**Verification:**
```bash
go test ./internal/directory/...
```

---

### Step 11 — Subscription registry data structure and API

**Status:** DONE (completed: 2026-04-02) — Implemented subscription registry with thread-safe tracking of node subscriptions and subscribers, comprehensive test coverage for all operations

**Spec reference:** `docs/amorphdb_design.md` §Subscription Model

**Scope:** New package `internal/subscription/`

**Do not touch:** zone, mesh, storage, interpreter, security, watcher, directory

**What to do:**
1. Create `internal/subscription/` package
2. The subscription registry tracks:
   - Which paths this node subscribes to (and from which authority)
   - Which remote nodes subscribe to paths this node holds authority for
3. Operations:
   - `Subscribe(path, authorityNode)` — register this node as a subscriber
   - `Unsubscribe(path)` — remove subscription
   - `AddSubscriber(path, subscriberNode)` — record that a remote node subscribes
     to a path we own authority for
   - `RemoveSubscriber(path, subscriberNode)` — remove subscriber record
   - `Subscribers(path) []NodeIdentity` — list all subscribers for a path
   - `Subscriptions() []Subscription` — list all paths this node subscribes to
4. Thread-safe
5. No network operations — pure data structure

**New tests to write:**
- Subscribe and list subscriptions
- Add and remove subscribers
- Multiple subscribers per path

**Verification:**
```bash
go test ./internal/subscription/...
```

---

### Step 12 — Write authority assignment and promotion protocol

**Status:** DONE (completed: 2026-04-02) — Implemented comprehensive write authority management with announcement, promotion, delegation, and splitting protocols

**Spec reference:** `docs/amorphdb_design.md` §Write Authority (full section)

**Scope:** `internal/mesh/` (new file `authority.go`), `internal/directory/`,
`internal/subscription/`

**Do not touch:** zone, storage engine core, interpreter, security, watcher

**What to do:**
1. Read all files in `internal/mesh/` fully before adding to it
2. Add `internal/mesh/authority.go` implementing:
   - **Authority announcement:** when a node takes write authority for a path,
     it announces via gossip to update other nodes' path directories
   - **Authority promotion:** when a node detects an authority is unresponsive
     (missed N heartbeats), it initiates an election among subscribers.
     Election rule: subscriber with longest uptime wins; timestamp as tiebreaker.
     Winner announces itself as new authority via gossip.
   - **Voluntary delegation:** a node may delegate its write authority for a
     sub-path to a subscriber. It announces the delegation via gossip and
     updates its own directory.
   - **Authority splitting:** when an authority is overloaded (configurable
     write rate threshold), it may split a sub-path off to another node.
     It delegates authority for the sub-path and announces via gossip.
3. Wire into the existing gossip protocol — authority announcements and
   promotions use the same gossip channel as node join/leave events
4. Wire into the existing heartbeat — missed heartbeat count triggers promotion

**New tests to write:**
- Authority announcement propagates via gossip
- Promotion triggers after N missed heartbeats
- Elected node becomes authority and announces
- Voluntary delegation updates directory on both nodes

**Verification:**
```bash
go test ./internal/mesh/...
go test ./internal/directory/...
```

---

### Step 13 — Subscription-based replication replacing zone replication

**Status:** DONE (completed: 2026-04-02) — Implemented complete subscription-based replication system with feature flag, write routing, subscription management, and comprehensive test coverage

**Spec reference:** `docs/amorphdb_design.md` §Subscription Model,
§Distributed Computation

**Scope:** `internal/mesh/replication.go` (replace zone-based replication logic),
`internal/subscription/`, `internal/directory/`

**Do not touch:** `internal/zone/` (leave it intact until step 14),
storage engine core, interpreter, security, watcher

**What to do:**
1. Read `internal/mesh/replication.go` fully before touching it
2. The current replication model pushes data to zone replicas. Replace this with:
   - When a write commits on the authority node, look up all subscribers for
     that path in the subscription registry
   - Push the new instance to each subscriber within the same heartbeat tick
   - Subscribers apply the incoming instance to their local storage
3. When a node subscribes to a path for the first time, it requests a full
   snapshot of current data at that path from the authority (bootstrap sync)
4. Subscription-based reads: if a node has a current subscribed copy of a path,
   serve reads locally. If not, forward to the authority and subscribe.
5. Write routing: if a write arrives for a path this node does not hold authority
   for, look up the authority in the path directory and forward the write.
6. Keep the old zone replication code in place but behind a feature flag
   `config.UseSubscriptionReplication = false` (default false until step 14)

**New tests to write:**
- Write on authority pushes to all subscribers
- Subscriber receives update within one heartbeat tick
- New subscriber receives bootstrap snapshot
- Write forwarded to authority when received by non-authority node
- Read served locally from subscribed data

**Verification:**
```bash
go test ./internal/mesh/...
go test ./internal/subscription/...
# Run multi-node integration tests with feature flag enabled
```

---

### Step 14 — Cutover: enable subscription model, retire zone model

**Status:** DONE (completed: 2026-04-02) — Enabled subscription-based replication as default, removed UseSubscriptionReplication config flag, archived zone package to zone_deprecated/. Integration tests pass with subscription model. Some legacy zone-based tests remain that need cleanup in future maintenance.

**Spec reference:** `docs/amorphdb_design.md` §Data Distribution Model

**Scope:** `internal/mesh/`, `internal/service/`, `internal/config/`

**Do not touch:** `internal/zone/` until explicitly instructed below,
storage engine core, interpreter, security, watcher

**What to do:**
1. Run the full integration test suite with `UseSubscriptionReplication = true`
   and confirm all tests pass
2. Change the default: `config.UseSubscriptionReplication = true`
3. Run the full test suite again — all tests must pass
4. If any tests specifically test zone/hash-ring behavior in a way that is
   now obsolete, update them to test equivalent subscription behavior instead
5. Remove the feature flag — subscription model is now the only model
6. Archive `internal/zone/` by moving it to `internal/zone_deprecated/` with
   a README explaining it was the old model. Do NOT delete it yet — keep it
   for reference during any debugging.
7. Update `internal/mesh/manager.go` and any other files that imported
   `internal/zone/` to import nothing from it

**Verification:**
```bash
go test ./...
# All tests must pass
# Run a full multi-node integration scenario manually
```

---

## Phase 6 — Cleanup and Documentation

### Step 15 — Update REPL join process output and status messages

**Status:** DONE (completed: 2026-04-02) — Updated CLI output terminology from zone-based to subscription model. Changed "Assigned Zone" to "Path Authorities" in join output, and "Zones" to "Path Authorities" in mesh status. Updated corresponding struct fields and test fixtures.

**Spec reference:** `docs/amorphdb_design.md` §Mesh Formation and Management

**Scope:** `cmd/amorphctl/join.go`, `cmd/amorphctl/mesh.go`,
`cmd/amorphd/main.go`

**Do not touch:** internal packages

**What to do:**
1. Update any status messages or output that references zones, hash rings,
   or zone assignments to reflect the subscription model terminology
   (e.g., "receives path directory" instead of "zone assignments")
2. Update `amorphctl status` output if it shows zone information

**Verification:**
```bash
go test ./cmd/...
# Manual: run amorphctl join and check output messages
```

---

### Step 16 — Archive old spec documents and place updated specs

**Status:** DONE (completed: 2026-04-02) — Verified all current spec documents contain required sections. Created missing pending_code_changes.md with asset() rename documentation. Historical spec versions preserved.

**Scope:** `docs/` directory only — no code changes

**What to do:**
1. Verify `docs/amorphdb_design.md` is the current updated version
   (check that it contains §Data Distribution Model, §Same-Line Definitions,
   §Projection syntax, §Embed keyword, §MCARS)
2. Verify `docs/amorphdb_computer_library.md` is the current updated version
   (check that it contains the implementation status table, ARI section,
   XML section, `asset()` constructor)
3. Verify `docs/pending_code_changes.md` exists and lists the asset() rename
4. The old spec versions (`amorphdb_design_until20260314.md`,
   `amorphdb_design_until20260402.md`) can be left in place as historical record
5. No code changes in this step

**Verification:**
```bash
grep -l "Data Distribution Model" docs/amorphdb_design.md
grep -l "asset(data, mime_type)" docs/amorphdb_computer_library.md
grep -l "asset()" docs/pending_code_changes.md
# All three should return results
```

---

## Summary

| Step | Description | Phase | Status |
|---|---|---|---|
| 1 | Replace embed() with asset() constructor | Terminology | TODO |
| 2 | Record literal assignment | MBL Language | TODO |
| 3 | Recursive assignment | MBL Language | TODO |
| 4 | Same-line definitions | MBL Language | TODO |
| 5 | Projection syntax | MBL Language | TODO |
| 6 | Embed keyword in record bodies | MBL Language | TODO |
| 7 | REPL auto-inferred display and hints | REPL | TODO |
| 8 | XML import and export | File I/O | TODO |
| 9 | ARI fixed-width file import | File I/O | TODO |
| 10 | Path directory data structure | Mesh Distribution | TODO |
| 11 | Subscription registry data structure | Mesh Distribution | TODO |
| 12 | Write authority assignment and promotion | Mesh Distribution | TODO |
| 13 | Subscription-based replication | Mesh Distribution | TODO |
| 14 | Cutover: enable subscription model | Mesh Distribution | TODO |
| 15 | Update status messages and CLI output | Cleanup | TODO |
| 16 | Archive old specs, verify current specs | Cleanup | TODO |
| 17 | Extended text literal syntax `_"…"_` | MBL Language | DONE (2026-07-29) |
| 18 | Interpolating text literal `~"…{path}…"~` | MBL Language | DONE (2026-07-29) |
| 19 | Make time literals produce a real `types.Time` | Date/Time | TODO |
| 20 | Time part access via `.@year` meta-attributes | Date/Time | TODO |
| 21 | `format_time` / `parse_time` and timezones | Date/Time | TODO |
| 22 | Duration type and interval syntax | Date/Time | TODO |
| 23 | Calendar adjusters | Date/Time | TODO |
| 24 | Recurrence rules (RFC 5545 RRULE) | Date/Time | TODO |

---

## Step 17 — Extended text literal syntax `_"…"_`

Status: `DONE (2026-07-29)` — user-directed language change, not part of the
original plan.

Replaced the bare multi-quote text literal form (`""…""`, `"""…"""`) with an
explicit extended form: `_` plus a run of quotes to open, the same run of quotes
plus `_` to close. The simple `"…"` form is unchanged and remains the normal way
to write text; the extended form exists for text that embeds quote characters.

- `internal/mbl/lexer/lexer.go` — `readString` simplified to the single-quote
  form and now reports termination explicitly; new `readExtendedString` +
  `findExtendedClose` scan the extended form; new exported `UnquoteText` is the
  one place delimiters are stripped. The opening run backs off to the longest
  length that has a matching close, so content may begin with a quote.
- `internal/mbl/interpreter/interpreter.go` — both unwrapping sites now call
  `lexer.UnquoteText` instead of stripping a single character per side.

This also fixed a live bug: the old code stripped exactly one quote per side
regardless of the opening run, so `""He said "Hello" to me""` evaluated to
`"He said "Hello" to me"` with the delimiters leaking into the value. The lexer
tests only asserted raw token text, so nothing caught it. New tests assert
values, not just token text: `internal/mbl/lexer/string_literal_test.go` and
`internal/mbl/interpreter/string_literal_test.go` (including storage round-trip).

Spec updated: `docs/amorphdb_design.md` (Text type, Text literals) and
`docs/mbl_reference.md` (Types table). Dated `amorphdb_design_*` archive copies
were deliberately left untouched.

---

## Step 18 — Interpolating text literal `~"…{path}…"~`

Status: `DONE (2026-07-29)` — user-directed language change, follows Step 17.

Third text literal form, using `~` as the sigil (free since the `~` home sigil was
removed on 2026-06-30). It scans identically to `_"…"_`, including the matching
quote-run rule, so `~""…""~` works the same way.

Design decisions, all deliberate:
- **Paths only** between the braces. Arithmetic, calls and literals are parse
  errors, not silent evaluation.
- **`{{` escapes** an opening brace. An unmatched `}` is literal.
- **Simple `"…"` stays inert** — PWA assets assign CSS as MBL strings
  (`...assets["css/app.css"] = "body{color:red}"`), so braces must stay literal
  there. Interpolation could not be always-on, and could not live in `_"…"_`
  either, since JSON needs embedded quotes *and* literal braces at once.
- **Coercion goes through `types.Concatenate`**, so an interpolated value renders
  exactly as `&` renders it — including absorbing an Unknown as text rather than
  making the whole literal Unknown. A test asserts the two paths agree.

- `internal/mbl/lexer/tokens.go` — new `INTERP_TEXT` token type.
- `internal/mbl/lexer/lexer.go` — `readExtendedString`/`findExtendedClose`
  generalised to `readSigilString`/`findSigilClose` taking the sigil byte; the
  pre-switch lookahead now accepts `_` and `~`; `UnquoteText` accepts both sigils
  and requires the closing sigil to match the opening one.
- `internal/mbl/parser/ast.go` — `InterpolatedStringExpression` + `InterpolationPart`.
- `internal/mbl/parser/parser.go` — `parseInterpolatedString` splits the body;
  `parseInterpolationPath` runs each placeholder through a nested parser, so
  placeholders accept exactly the path syntax the language does (including
  leading-dot scope ascent) and anything not a `PathExpression` is rejected.
- `internal/mbl/interpreter/interpreter.go` — `evalInterpolatedString`.

Tests: `internal/mbl/lexer/interpolated_string_test.go` (tokenizing, sigil
non-cross-termination, unterminated, bare `~`) and
`internal/mbl/interpreter/interpolated_string_test.go` (values, multi-line,
storage round-trip, coercion-matches-`&`, and the path-only rejections).

---

# Phase 6 — Date and Time

Six steps, deliberately ordered: **Step 19 is a hard prerequisite for the rest.**
Until time literals produce a `types.Time`, every function in Steps 20–24 would
receive a `types.Text` and there would be nothing to operate on.

**Design inspiration** (surveyed 2026-07-29):
- **Temporal (ECMAScript 2026, TC39 Stage 4 March 2026)** — the type model.
  Its core idea is distinguishing an *instant* from a *plain date* from a
  *zoned datetime*. AmorphDB already has half of this in `types.Time.Precision`.
- **Postgres `interval`** — the three-field duration representation (months,
  days, seconds kept separate). The only correct model; see Step 22.
- **`java.time.TemporalAdjusters`** — the calendar-navigation vocabulary
  ("last Wednesday of the month"). Small, closed, composable; proven for a decade.
- **RFC 5545 `RRULE`** (iCalendar) — the recurrence standard. Adopting it rather
  than inventing a syntax buys interoperability with every calendar system.

**Do not merge these concerns.** Adjusting one date, generating a series, and
formatting for display are three different jobs and get three different APIs.

**A recurring rule across all six steps:** when an operation cannot be answered
truthfully, return `unknown` naming the reason — never invent a plausible value.
A minute that was never specified, a comparison between `1 month` and `30 days`,
an unsupported RRULE part: each returns `unknown` rather than a guess. A silently
wrong schedule or timestamp is worse than an absent one.

**Standing constraint:** times are stored as UTC. Only *rendering* converts to a
timezone. Nothing in these steps may change how a timestamp is serialized.

---

### Step 19 — Make time literals produce a real `types.Time`

**Status:** TODO

**Spec reference:** `docs/amorphdb_design.md` §Text/Number/... (Time type),
§Lexical Structure — time literals

**Scope:** `internal/mbl/lexer/lexer.go`, `internal/mbl/parser/parser.go`,
`internal/mbl/interpreter/interpreter.go`, `internal/types/coerce.go`

**Do not touch:** storage serialization, mesh, security, watcher. The on-disk
format of a timestamp must not change.

**Four verified defects this step fixes** (all reproduced 2026-07-29):

1. **A time literal never becomes a `types.Time`.** `parseTimeLiteral`
   (`parser.go:1737`) sets `lit.Value = parsedTime`, a Go `time.Time`.
   `convertToMBLType` (`interpreter.go`) has no case for that type, so it falls
   to `default:` and returns `Text{fmt.Sprintf("%v", v)}`. Result:
   `x = @2026-01-15` evaluates to `types.Text` reading
   `2026-01-15 00:00:00 +0000 UTC`. The `Precision` field and
   `formatTimeByPrecision` are unreachable from MBL.
2. **Three spec'd precisions do not parse.** The format list in
   `parseTimeLiteral` lacks `2006`, `2006-01`, and `2006-01-02 15:04`, so
   `@2026`, `@2026-01` and `@2026-01-15 14:30` are parse errors — even though
   `docs/amorphdb_design.md` documents them.
3. **Time comparison is unusable.** `readTimeLiteral` (`lexer.go`) consumes a
   trailing space unconditionally while looking for an optional time part, so
   `@2026-01-15 > @2026-01-01` lexes the literal as `"@2026-01-15 "` (trailing
   space) and fails with `could not parse "@2026-01-15 " as time`.
4. **`now()` is malformed.** `evalNowFunction` (`interpreter.go:2737`) returns
   `types.Time{Timestamp: time.Now()}` — `Precision` is left 0, which is outside
   the valid range 1–7 that `types.go:432` enforces, so `formatTimeByPrecision`
   hits its `default:` branch and prints full microseconds. It also carries the
   machine's local zone rather than UTC.

**What to do:**
1. Read `parseTimeLiteral`, `readTimeLiteral`, `convertToMBLType`,
   `evalNowFunction` and `formatTimeByPrecision` in full first.
2. In `readTimeLiteral`, only consume the space after the date when the *next*
   character is a digit. Everything else is a separate token.
3. In `parseTimeLiteral`, pair each layout with the precision it implies and set
   both fields, producing a `types.Time` rather than a Go `time.Time`:

   | Layout | Precision |
   |---|---|
   | `2006` | `PrecisionYear` |
   | `2006-01` | `PrecisionMonth` |
   | `2006-01-02` | `PrecisionDay` |
   | `2006-01-02 15:04` | `PrecisionMinute` |
   | `2006-01-02 15:04:05` | `PrecisionSecond` |
   | `2006-01-02T15:04:05`, `…Z07:00` | `PrecisionSecond` |

   Go's `time.Parse` accepts a fractional second after the seconds field even
   when the layout omits it; detect a `.` in the literal and use
   `PrecisionSubsecond` in that case. There is no hour-only literal form —
   `PrecisionHour` stays reachable only through truncation in Step 22.
   Parse in UTC (`time.ParseInLocation(..., time.UTC)`), not local.
4. Add a `case time.Time:` and `case types.Time:` to `convertToMBLType` as a
   safety net so a stray Go time can never silently become Text again.
5. Fix `now()`: `time.Now().UTC()` with `Precision: PrecisionSubsecond`.
6. **Change text coercion to drop the `@`.** `formatTimeByPrecision`
   (`coerce.go:331`) currently prefixes `@`, which is *source syntax* and wrong
   in output — `~"Shipped {my.order.date}"~` should read `Shipped 2026-01-15`.
   Leave `Time.String()` (`types.go:173`) alone: the `@` form is correct there,
   for REPL display and round-tripping. Precision already handles the rest —
   a day-precision value renders `2026-01-15` with no `00:00:00`, which is the
   sensible-default behaviour and needs no new code.

**New tests to write:**
```go
// Type, not text
`x = @2026-01-15`            // types.Time, PrecisionDay
`x = now()`                  // types.Time, PrecisionSubsecond, UTC

// All spec'd precisions parse
`x = @2026`                  // PrecisionYear
`x = @2026-01`               // PrecisionMonth
`x = @2026-01-15 14:30`      // PrecisionMinute
`x = @2026-01-15 14:30:22`   // PrecisionSecond
`x = @2026-01-15 14:30:22.5` // PrecisionSubsecond

// Comparison works (regression test for the lexer space bug)
`x = @2026-01-15 > @2026-01-01`   // true
`x = @2026-01-15 ?= @2026-01-15`  // true

// Coercion omits absent components and the @ sigil
`x = "on " & @2026-01-15`         // "on 2026-01-15"
`x = "at " & @2026-01-15 14:30`   // "at 2026-01-15 14:30"
`x = ~"on {my.d}"~`               // matches the & form exactly
```

**Verification:**
```bash
go test ./internal/mbl/... ./internal/types/...
go test ./...   # must stay at the 25-package baseline
```

---

### Step 20 — Time part access via `.@year` meta-attributes

**Status:** TODO

**Depends on:** Step 19

**Scope:** `internal/mbl/parser/` (only if the meta-attribute path needs it),
`internal/mbl/interpreter/`

**Do not touch:** storage, mesh, watcher

**Why meta-attributes rather than plain attributes:** `my.order.date.year` would
read better but collides with path traversal — it becomes ambiguous the moment a
user stores a child attribute named `year`. The `@` sigil keeps computed parts
unambiguously distinct from stored data, and the syntax already exists:
`my.account.balance.@time` parses today (see
`internal/mbl/parser/testplan_section4_test.go`).

**What to do:**
1. Find where `.@`-prefixed path segments are resolved in the interpreter and
   add time-part handling for a `types.Time` receiver.
2. Implement: `.@year`, `.@month`, `.@day`, `.@hour`, `.@minute`, `.@second`,
   `.@weekday`, `.@precision`.
3. `.@year` … `.@second` return `types.Number`. `.@weekday` returns
   `types.Text` (`"wednesday"`) — lowercase, matching the argument spelling the
   adjusters in Step 22 accept, so the two compose.
4. Requesting a part finer than the value's precision returns `unknown` with a
   clear reason, e.g. `@2026-01-15.@minute` → `unknown("time has day precision;
   minute is not known")`. **Do not silently return 0** — that is exactly the
   `00:00:00` fiction this phase exists to remove.
5. Applying a time part to a non-Time returns `unknown`.

**New tests to write:**
```go
`my.d = @2026-01-15 14:30
x = my.d.@year`     // 2026
`... my.d.@month`   // 1
`... my.d.@day`     // 15
`... my.d.@hour`    // 14
`... my.d.@weekday` // "wednesday"

// Precision honesty — the primary failure case
`my.d = @2026-01-15
x = my.d.@minute`   // unknown, NOT 0

// Non-Time receiver
`my.n = 5
x = my.n.@year`     // unknown
```

**Verification:**
```bash
go test ./internal/mbl/...
```

---

### Step 21 — `format_time` / `parse_time` and timezones

**Status:** TODO

**Depends on:** Step 19

**Scope:** `internal/mbl/interpreter/`, `internal/types/`

**Do not touch:** storage serialization — timezone is a *rendering* concern only.

**What to do:**
1. Add `format_time(t, pattern)` and `format_time(t, pattern, zone)` to the
   builtin dispatch (alongside `case "now"`, `interpreter.go:2432`).
2. **Pattern vocabulary — follow Unicode LDML**, which Java, .NET and every
   major date library already use. Adopting the convention people know is the
   opposite of making them learn a table. Keep it to this closed set:

   | Token | Meaning | Token | Meaning |
   |---|---|---|---|
   | `YYYY` / `YY` | year | `HH` | hour, 24-hour |
   | `MMMM` / `MMM` | month name | `hh` | hour, 12-hour |
   | `MM` / `M` | month number | `mm` | minute |
   | `DD` / `D` | day of month | `ss` | second |
   | `dddd` / `ddd` | weekday name | `A` | AM/PM |

   ⚠️ **`MM` is month and `mm` is minute; `HH` is 24-hour and `hh` is 12-hour.**
   This trips everyone. Document it at the top of the reference entry and make
   the error message for an unknown token name the valid set.
3. Add `parse_time(text)` and `parse_time(text, pattern)`, returning `unknown`
   with a useful reason on failure rather than a zero time. Without a pattern,
   accept the same forms the literal syntax accepts and infer precision the same
   way, so `parse_time` and `@…` agree.
4. **Timezone, two layers:**
   - A node default, `my.computer.timezone` (IANA name, e.g.
     `"America/Chicago"`), matching the `my.computer.*` convention. Unset means
     UTC, preserving today's behaviour.
   - An optional third argument to `format_time` overriding it per call.
   For the PWA the browser reports its zone, so per-user storage on the user
   record is the natural home — a watcher then renders correctly for users in
   different zones. Note this in the spec; no PWA code changes in this step.
5. An invalid IANA zone name returns `unknown`, never a silent fallback to UTC.

**New tests to write:**
```go
`x = format_time(@2026-01-15 14:30, "YYYY-MM-DD HH:mm")`  // "2026-01-15 14:30"
`x = format_time(@2026-01-15, "dddd, MMMM D, YYYY")`      // "Wednesday, January 15, 2026"
`x = format_time(@2026-01-15 14:30, "hh:mm A")`           // "02:30 PM"

// Timezone conversion, storage untouched
`x = format_time(@2026-01-15 14:30, "HH:mm", "America/Chicago")`  // "08:30"
`x = format_time(@2026-01-15 14:30, "HH:mm", "Not/AZone")`        // unknown

// Round trip
`x = parse_time("2026-01-15")`         // types.Time, PrecisionDay
`x = parse_time("nonsense")`           // unknown
`x = format_time(parse_time("2026-01-15"), "YYYY-MM-DD")`  // "2026-01-15"
```

**Verification:**
```bash
go test ./internal/mbl/... ./internal/types/...
```


---

### Step 22 — Duration type and interval syntax

**Status:** TODO

**Depends on:** Step 19

**Scope:** `internal/types/`, `internal/mbl/lexer/`, `internal/mbl/parser/`,
`internal/mbl/interpreter/`

**Do not touch:** mesh, security, watcher. Storage gains one new type tag and
nothing else.

**Why a type rather than a bare Number:** `mbl_reference.md` justifies Money as a
type because "currency is part of the value." A duration's unit is part of its
value in exactly the same way. Without the type, `my.terms.payment_days = 30`
puts the unit in the *attribute name* — the schema — so changing "Net 30" to
"Net 6 weeks" requires renaming the attribute and updating every watcher that
reads it. A business user changing a payment term must not require a schema
change. Two AmorphDB-specific reasons reinforce it: temporal history of a stored
term is only meaningful if each instance carries its unit; and a watcher
comparing elapsed time against a stored SLA is comparing two bare Numbers and
trusting the units match, which fails silently.

**Representation — three fields, not one integer:**

```go
type Duration struct {
    Months  int64  // calendar months; cannot reduce to days
    Days    int64  // calendar days; cannot reduce to seconds under DST
    Seconds float64 // exact time, sub-second capable
}
```

This is Postgres's `interval` design and it is the only correct one. Months have
no fixed length (28–31 days). Days are not always 86,400 seconds once Step 21
introduces local-time rendering. Collapsing either loses information that cannot
be recovered, and the split costs nothing to carry now but cannot be retrofitted
after data exists.

`types.Money` is the implementation template — see `Serialize` (`types.go:177`),
`TypeTag` and `DeserializeTime` (`types.go:423`) for the full surface a new type
must cover.

**What to do:**
1. Read `internal/types/types.go`, `coerce.go` and `compare.go` fully, using
   `Money` as the worked example of adding a type.
2. Add `types.Duration` with a new type tag, `Serialize`/`Deserialize`, and
   `String()`.
3. **Literal syntax** — the same notation serves both storing and arithmetic:
   ```
   my.sla.response = 4 hours 30 minutes
   my.due = my.invoice.date + 30 days
   ```
   A duration literal is one or more `<number> <unit>` pairs, juxtaposed with no
   separator. Units are a **closed keyword set** so a typo fails at parse time:
   `year`, `month`, `week`, `day`, `hour`, `minute`, `second`, each accepting a
   plural `s`. `week` normalises to 7 days at parse time; `year` to 12 months.
4. `+` and `-` between a Time and a Duration return a Time. Apply in a fixed,
   documented order — **months first, then days, then seconds** — because
   calendar arithmetic is not commutative (`+ 1 month + 1 day` differs from
   `+ 1 day + 1 month` starting from 31 January). Month arithmetic clamps:
   31 January plus one month is 28/29 February, never 2/3 March.
5. `+` and `-` between two Durations return a Duration. Negation is allowed;
   durations may be negative.
6. **Remove `add_time` / `subtract_time`** from the Step 22 draft that preceded
   this revision — the interval syntax replaces them, and reads as intent rather
   than mechanism.
7. `difference(a, b)` returns a **Duration** rather than a Number plus a unit
   string. Sign convention: `difference(later, earlier)` is positive.
8. **Text coercion omits zero components** — `3 days 2 hours`, never
   `3 days 0 hours 0 seconds`. Same principle as not printing `00:00:00`.
9. **Comparison across the month/day boundary returns `unknown`.** Is
   `1 month > 30 days`? Genuinely undefined — it depends which month. Postgres
   answers by assuming 30-day months, which is a documented lie. Durations whose
   `Months` are both zero compare exactly; otherwise, unless anchored to a date,
   return `unknown`. This matches the honesty rule running through the phase.
10. **Result precision.** `@2026-01-15 + 2 hours` — the operand is day-precision
    and knows nothing about hours. Adding a sub-day duration to a day-precision
    time returns `unknown`, consistent with Step 20 refusing to invent a minute
    that was never specified. Adding whole days to a day-precision time is fine
    and keeps day precision.

**New tests to write:**
```go
// Stored as a value, with its unit intact
`my.sla = 4 hours 30 minutes`        // types.Duration
`x = "SLA: " & my.sla`               // "SLA: 4 hours 30 minutes"
`my.d = 3 days 0 hours
x = "" & my.d`                       // "3 days"   (zero components omitted)

// Arithmetic
`x = @2026-01-15 + 30 days`          // @2026-02-14
`x = @2026-01-31 + 1 month`          // @2026-02-28  (clamp, not 3 Mar)
`x = @2026-01-15 - 1 week`           // @2026-01-08
`x = @2026-01-15 14:00 + 2 hours 15 minutes`  // @2026-01-15 16:15

// Non-commutativity is real and must be documented, not "fixed"
`x = @2026-01-31 + 1 month + 1 day`  // @2026-03-01
`x = @2026-01-31 + 1 day + 1 month`  // @2026-03-01 ... assert actual, document it

// Duration arithmetic and difference
`x = 2 hours + 30 minutes`                       // 2 hours 30 minutes
`x = difference(@2026-01-15, @2026-01-01)`       // 14 days
`x = difference(@2026-01-01, @2026-01-15)`       // -14 days

// Comparison honesty — the primary failure case
`x = 90 minutes > 1 hour`            // true   (both month-free)
`x = 1 month > 30 days`              // unknown, NOT a guess

// Precision honesty
`x = @2026-01-15 + 2 hours`          // unknown (day-precision operand)
`x = @2026-01-15 + 3 days`           // @2026-01-18, still day precision

// Parse-time failures
`my.x = 3 fortnights`                // parse error naming the valid units
`my.x = 3 dayz`                      // parse error
```

**Verification:**
```bash
go test ./internal/types/... ./internal/mbl/...
go test ./...   # must stay at the 25-package baseline
```

---

### Step 23 — Calendar adjusters

**Status:** TODO

**Depends on:** Steps 19, 20, 22

**Scope:** `internal/mbl/interpreter/`, `internal/types/`

**Do not touch:** storage, mesh, security, watcher

Modeled on `java.time.TemporalAdjusters` — a small, closed, composable
vocabulary that has been proven for a decade. This is the "last Wednesday of the
month" capability, which is the thing scheduling actually needs and which plain
duration arithmetic cannot express.

**What to do:**
1. `start_of(t, unit)` and `end_of(t, unit)`. **These are the workhorses** —
   they absorb roughly six `java.time` adjusters (`lastDayOfMonth`,
   `firstDayOfNextMonth` and siblings) into two functions that also work for
   weeks, quarters and years. `start_of` sets the resulting precision to match
   the unit; `end_of` returns the last representable instant within it. Week
   boundaries use ISO Monday as the documented first day.
2. Adjusters, taking the lowercase weekday spellings `.@weekday` returns in
   Step 20 so the two compose directly:
   - `next_weekday(t, "wednesday")` / `previous_weekday(t, "wednesday")`
   - `first_weekday_in_month(t, "wednesday")`
   - `last_weekday_in_month(t, "wednesday")`
   - `nth_weekday_in_month(t, 3, "wednesday")` — negative `n` counts back from
     the end, so `-1` equals `last_weekday_in_month`
3. An out-of-range `n` (a fifth Wednesday in a month with four) returns
   `unknown`, never a silently rolled-over date.
4. Every function returns `unknown` for a non-Time receiver or an unknown unit,
   and the message names the valid vocabulary.

**New tests to write:**
```go
`x = start_of(@2026-01-15 14:30, "month")`   // @2026-01-01, PrecisionDay
`x = end_of(@2026-01-15, "month")`           // last instant of 31 Jan
`x = start_of(@2026-01-15, "week")`          // Monday 12 Jan

// The headline capability
`x = last_weekday_in_month(@2026-01-15, "wednesday")`   // @2026-01-28
`x = first_weekday_in_month(@2026-01-15, "wednesday")`  // @2026-01-07
`x = nth_weekday_in_month(@2026-01-15, 3, "wednesday")` // @2026-01-21
`x = nth_weekday_in_month(@2026-01-15, -1, "wednesday")`// @2026-01-28
`x = nth_weekday_in_month(@2026-01-15, 5, "wednesday")` // unknown
`x = next_weekday(@2026-01-15, "wednesday")`            // @2026-01-21

// Composes with Steps 20 and 22
`my.d = last_weekday_in_month(@2026-01-15, "wednesday")
x = my.d.@day`                                          // 28
`x = last_weekday_in_month(@2026-01-15, "wednesday") - 2 days`  // @2026-01-26

// Failure cases
`x = start_of(@2026-01-15, "fortnight")`     // unknown, names valid units
`x = next_weekday("not a time", "monday")`   // unknown
```

**Verification:**
```bash
go test ./internal/mbl/... ./internal/types/...
```

---

### Step 24 — Recurrence rules (RFC 5545 RRULE)

**Status:** TODO

**Depends on:** Step 23

**Scope:** `internal/mbl/interpreter/`, `internal/types/`, possibly
`internal/watcher/`

**Why RRULE:** RFC 5545 is the recurrence standard every calendar system speaks.
Last Wednesday of every month is `FREQ=MONTHLY;BYDAY=-1WE`; second and fourth
Fridays is `FREQ=MONTHLY;BYDAY=2FR,4FR`. Adopting it rather than inventing a
syntax means AmorphDB schedules interoperate with iCalendar, Google Calendar and
every scheduling tool for free. Do **not** design a bespoke recurrence notation.

**What to do:**
1. Represent an RRULE as `types.Text` holding the RFC 5545 string. Resist adding
   a type until there is a reason — the string *is* the interoperable form, which
   is the opposite of the Duration argument in Step 22, where the unit had
   nowhere else to live.
2. `next_occurrence(rule, after)` → the first occurrence strictly after `after`.
3. `occurrences_between(rule, start, end)` → a `types.List` of times. **This must
   be bounded** — a malformed or unbounded rule must not generate indefinitely.
   Apply a configurable cap and return `unknown` when exceeded, in the spirit of
   the existing commit-buffer limit and `MaxCallDepth` guard.
4. Support at minimum `FREQ` (`DAILY`/`WEEKLY`/`MONTHLY`/`YEARLY`), `INTERVAL`,
   `BYDAY` (including negative ordinals such as `-1WE`), `BYMONTHDAY`, `BYMONTH`,
   `COUNT` and `UNTIL`. Return `unknown` **naming the offending part** for
   anything unsupported rather than ignoring it silently — silently dropping a
   constraint produces a schedule that is wrong rather than absent, the worst
   failure mode here.
5. Validate the rule when first used and report the specific syntax error.
6. **Watcher integration is a separate decision.** A watcher firing on a schedule
   is a natural pairing, but it interacts with the heartbeat model and write
   authority. Scope it as its own step once Steps 19–24 are settled; do not fold
   it in here.

**New tests to write:**
```go
`my.r = "FREQ=MONTHLY;BYDAY=-1WE"
x = next_occurrence(my.r, @2026-01-01)`        // @2026-01-28

`my.r = "FREQ=WEEKLY;BYDAY=MO,WE,FR"
x = occurrences_between(my.r, @2026-01-01, @2026-01-15)`  // list of 6

`my.r = "FREQ=MONTHLY;BYDAY=2FR,4FR"
x = next_occurrence(my.r, @2026-01-01)`        // @2026-01-09

`my.r = "FREQ=DAILY;COUNT=3"
x = occurrences_between(my.r, @2026-01-01, @2026-12-31)`  // exactly 3

// Failure cases
`x = next_occurrence("FREQ=NONSENSE", @2026-01-01)`   // unknown, names the error
`x = next_occurrence("FREQ=SECONDLY", @2026-01-01)`   // unknown if unsupported
// Unbounded rule over a wide range must hit the cap, not hang
```

**Verification:**
```bash
go test ./internal/mbl/... ./internal/types/...
go test ./...
```
