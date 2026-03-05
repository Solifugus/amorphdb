# AmorphDB Development Plan

## Preamble

This document is a comprehensive, step-by-step plan for building AmorphDB — a temporal tree-graph database with a decentralized mesh architecture, accessed through MBL (Modern Business Language).

**Language:** Go (Golang)
**Spec:** See `amorphdb_design.md` for the full design specification.
**Project root:** `amorphdb/`

### Project Structure

```
amorphdb/
├── cmd/
│   ├── amorphd/          # service daemon
│   │   └── main.go
│   ├── amorphctl/        # admin tool
│   │   └── main.go
│   └── amorph/           # client (REPL + script runner)
│       └── main.go
├── internal/
│   ├── storage/          # storage engine (attributes, instances, values)
│   ├── types/            # data types (Text, Number, Time, Money, etc.)
│   ├── mbl/              # MBL parser and interpreter
│   │   ├── lexer/
│   │   ├── parser/
│   │   └── interpreter/
│   ├── temporal/         # temporal logic (instance chains, history queries)
│   ├── mesh/             # mesh networking, gossip, heartbeat
│   ├── zone/             # zone management, consistent hashing
│   ├── security/         # encryption, authentication, key management
│   ├── watcher/          # watcher execution engine
│   └── protocol/         # wire protocol encoding/decoding
├── pkg/
│   └── client/           # client library for embedding
├── state/                # runtime state tracking (see Current State below)
│   └── STATUS.md         # living document tracking progress
├── test/
│   ├── storage/
│   ├── mbl/
│   ├── mesh/
│   └── integration/
├── go.mod
├── go.sum
├── amorphdb_design.md           # design specification
├── amorphdb_development_plan.md # design specification
└── README.md
```

### Current State File

The file `state/STATUS.md` must be maintained throughout development. When starting any new Claude Code session, **read this file first** to understand what has been completed, what is in progress, and what issues are known.

Format:

```markdown
# AmorphDB Development Status

## Completed Steps
- Step 1: Storage Engine — completed YYYY-MM-DD
- Step 2: Type System — completed YYYY-MM-DD

## In Progress
- Step 3: MBL Lexer — substep 3.2 (operators) in progress

## Known Issues
- (list any bugs, incomplete features, or design questions)

## Notes
- (any important observations from the development process)
```

### Conventions

- Go standard formatting (`gofmt`)
- Tests use Go's built-in `testing` package
- Errors are returned, not panicked, unless truly unrecoverable
- All public functions and types have doc comments
- File I/O uses 64-bit offsets throughout
- Integer IDs are `uint64`

---

## Step 1: Storage Engine — Core Structures

**Implements:** Spec section "Data Storage and Retrieval System"

**Goal:** Build the foundational three-tier storage model — attributes, instances, and values — with file I/O. This is the bottom of the stack; everything else builds on it.

### 1.1: Value Storage

**What:** Implement the value file and bucketed storage. Values are immutable, content-addressed blobs.

**Files:** `internal/storage/values.go`, `internal/storage/values_test.go`

**Structures:**

```go
// Value record on disk:
//   sentinel (1 byte: 0x1E)
//   type_tag (1 byte)
//   length   (4 bytes, for variable-length types)
//   data     (variable)

type ValueStore struct {
    // manages multiple bucket files and the large value file
}
```

**Bucket tiers:**
- Tier 0: Fixed-size (≤64 bytes — numbers, time, money, references)
- Tier 1: Small (64–512 bytes — short text, small procedures)
- Tier 2: Medium (512–4096 bytes — longer text, procedures)
- Tier 3: Large (>4096 bytes — pictures, large text) — one file, offset-addressed

**Value ID is composite:** upper bits encode tier and bucket file, lower bits encode offset within that file.

**Type tags:**
- 0x01 Text, 0x02 Number, 0x03 Time, 0x04 Money, 0x05 Picture, 0x06 Reference, 0x07 Procedure, 0x08 Watcher, 0x09 Embed
- 0xF0 Nothing, 0xF1 Unknown, 0xF2 Anything

**Success criteria:**
- Write a Text value, read it back, confirm exact match
- Write same value twice, confirm same value_id returned (deduplication)
- Write values of different sizes, confirm correct bucket assignment
- Write large value (>4096 bytes), confirm it goes to the large file
- Tombstone a value (0x1F sentinel), confirm space tracked in free list
- Allocate from free list, confirm reuse of tombstoned space

### 1.2: Instance Storage

**What:** Implement the instance file. Instances are temporal records that point to a value and form a chain.

**Files:** `internal/storage/instances.go`, `internal/storage/instances_test.go`

**Structure (fixed-width on disk):**

```go
// Instance record:
//   value_id          (8 bytes)
//   older_instance_id (8 bytes — 0 if none)
//   timestamp         (8 bytes — UNIX time, microsecond precision)
//   author_id         (8 bytes — value_id of the author identity)
```

**Success criteria:**
- Create instance, read it back, confirm all fields match
- Create second instance for same attribute, confirm older_instance_id links to first
- Query instance chain, walk from newest to oldest, confirm correct order
- Confirm timestamps are UTC and monotonically increasing per attribute

### 1.3: Attribute Storage

**What:** Implement the attribute index. Attributes are named or numbered entries that point to their first (most recent) instance and link to sibling attributes.

**Files:** `internal/storage/attributes.go`, `internal/storage/attributes_test.go`

**Structure (fixed-width on disk):**

```go
// Attribute record:
//   label_value_id     (8 bytes — points to the attribute name/number in values)
//   first_instance_id  (8 bytes)
//   next_attribute_id  (8 bytes — next sibling, 0 if last)
```

**Success criteria:**
- Create attribute with text label, read it back
- Create attribute with numeric label, read it back
- Create multiple sibling attributes, traverse the linked list
- Look up attribute by label

### 1.4: Hash Index for Attributes

**What:** Add a hash table mapping attribute labels to attribute IDs for O(1) lookup.

**Files:** `internal/storage/hashindex.go`, `internal/storage/hashindex_test.go`

**Success criteria:**
- Insert 1000 attributes, look up each by label, confirm O(1) performance
- Handle hash collisions correctly
- Rebuild index from attribute file, confirm identical results

### 1.5: Integrated Tree Operations

**What:** Combine attributes, instances, and values into a tree that supports path traversal, reading, and writing.

**Files:** `internal/storage/tree.go`, `internal/storage/tree_test.go`

**Interface:**

```go
type Tree interface {
    Read(path []string) (Value, error)
    Write(path []string, value Value, author uint64) error
    ReadAt(path []string, timestamp int64) (Value, error)
    Children(path []string) ([]Attribute, error)
    Purge(path []string, from int64, to int64, author uint64) error
}
```

**Success criteria:**
- Write `world.agent.kalevo.name = "Joe"`, read it back
- Write a new value, confirm new instance is created
- Write the same value, confirm no new instance is created
- Read at a past timestamp, confirm historical value returned
- Create nested structure 5 levels deep, traverse it
- Purge instances in a time range, confirm they are tombstoned
- Confirm purge audit record is created

### 1.6: Free List and Defragmentation

**What:** Implement the free list for value storage and a defragmentation process.

**Files:** `internal/storage/freelist.go`, `internal/storage/defrag.go`, `internal/storage/defrag_test.go`

**Success criteria:**
- Purge values, confirm free list entries created
- New writes reuse free list space when available
- Defragmentation compacts a file, updates all references
- Free list can be rebuilt from scanning tombstones

---

## Step 2: Type System

**Implements:** Spec section "Data Types" and "Meta Types"

**Goal:** Implement all MBL data types as Go types with serialization to/from the value store.

### 2.1: Core Types

**Files:** `internal/types/types.go`, `internal/types/types_test.go`

**Types to implement:**

| MBL Type | Go Representation | Serialization |
|----------|-------------------|---------------|
| Text | `string` | UTF-8 bytes with length prefix |
| Number | `float64` | 8 bytes IEEE 754 |
| Time | custom struct | 8 bytes UNIX + 1 byte precision level |
| Money | custom struct | 8 bytes amount + currency code |
| Picture | `[]byte` | RGBA 16-bit, dimensions prefix |
| Reference | `uint64` | 8 bytes (attribute ID) |
| Nothing | singleton | type tag only, no payload |
| Unknown | struct with reason | type tag + reason text |
| Anything | singleton | type tag only, no payload |

**Time precision:** A Time value tracks its precision level (year, month, day, hour, minute, second, subsecond). This affects comparison behavior — see step 2.2.

**Success criteria:**
- Round-trip each type through serialization: create → serialize → deserialize → compare
- Text handles Unicode correctly (multi-byte characters)
- Money preserves currency code through serialization
- Nothing and Unknown serialize to minimal bytes
- Unknown preserves its reason string

### 2.2: Type Comparison and Coercion

**Files:** `internal/types/compare.go`, `internal/types/coerce.go`, `internal/types/compare_test.go`

**Comparison rules for Time:** (Spec: "Comparison operators")
- `?=` (equality): the more precise value falls within the range of the less precise one
- `>`: the more precise value is after the last moment of the less precise one
- `<`: the more precise value is before the first moment of the less precise one
- Same precision: compare normally

**Coercion rules:** (Spec: "Type Coercion")
- Arithmetic operators coerce to Number
- Concatenation (`&`) coerces to Text
- Operations that cannot be coerced produce Unknown with reason

**Success criteria:**
- `@2026-02-01 ?= @2026-02-01 15:30:00` → true
- `@2026-02-01 ?= @2026-02-03` → false
- `@2026-01 < @2026-02-15` → true
- `1 + "2"` → 3 (Number)
- `"hello" & 42` → "hello42" (Text)
- `1 / 0` → Unknown with reason "division by zero"

---

## Step 3: MBL Lexer

**Implements:** Spec section "MBL — Lexical Structure"

**Goal:** Tokenize MBL source code into a stream of tokens.

### 3.1: Token Types and Basic Lexing

**Files:** `internal/mbl/lexer/lexer.go`, `internal/mbl/lexer/tokens.go`, `internal/mbl/lexer/lexer_test.go`

**Token categories:**
- Identifiers (bare words)
- Keywords: `if`, `else`, `while`, `for`, `in`, `consider`, `watch`, `return`, `pass`, `new`, `and`, `or`, `not`, `true`, `false`, `my`, `world`, `quote`, `tab`, `newline`, `empty`, `pi`, `euler`, `Nothing`, `Unknown`, `Anything`
- Literals: text (quoted strings), numbers, time (`@`-prefixed), money (`¤`-prefixed)
- Operators: `+`, `-`, `*`, `/`, `%`, `&`, `?=`, `>`, `<`, `>=`, `<=`, `=`, `..`
- Delimiters: `(`, `)`, `[`, `]`, `{`, `}`, `:`, `,`, `.`, `~`
- Indentation: INDENT, DEDENT tokens (generated from tab counting)
- NEWLINE, EOF
- Modifiers: `(copy)`, `(link)`, `(reset ...)`, `(exclude)`, `(protected)`, `(cascade)`, `(quietly)`
- Comments: `#` through end of line (or multi-char `##...##`)

**Quote handling:** Matching-delimiter strings. Count opening quotes, require same count to close. `"hello"` is one quote, `""she said "hi"""` is two quotes enclosing text that contains one quote.

**Indentation:** Only leading tabs. Convert to INDENT/DEDENT tokens like Python's tokenizer.

**Success criteria:**
- Tokenize `my.contacts.joe.name = "Hello"` → correct token stream
- Tokenize indented blocks with INDENT/DEDENT
- Tokenize time literals: `@2026-02-01`, `@2026-02-01 15:30:00`
- Tokenize money: `¤19.95 USD`
- Tokenize `..` as system operation access
- Tokenize `?=` as equality operator
- Handle multi-line quoted strings correctly
- Handle `#` comments and `##` block comments
- Reject spaces in indentation with a clear error

---

## Step 4: MBL Parser

**Implements:** Spec sections "Operators", "Control Flow", "Procedures", "Watchers"

**Goal:** Parse token stream into an AST.

### 4.1: AST Node Types

**Files:** `internal/mbl/parser/ast.go`

**Node types (minimum):**
- `Program` (root)
- `Assignment`, `Definition` (`:` vs `=`)
- `BinaryOp`, `UnaryOp`
- `PathExpression` (dot-separated with optional bracket filters)
- `BracketFilter` (conditions inside `[]`)
- `TemporalQuery` (using `@` suffix)
- `Literal` (text, number, time, money)
- `If`, `ElseIf`, `Else`
- `Consider`, `ConsiderCase`
- `While`, `For`
- `Procedure`, `ProcedureCall`
- `Watcher`
- `New` (instantiation with source list)
- `SystemOp` (the `..` operations)
- `Record`, `List` (inline literals `{}`)
- `Return`

### 4.2: Expression Parser

**Files:** `internal/mbl/parser/parser.go`, `internal/mbl/parser/parser_test.go`

Pratt parser or recursive descent with operator precedence.

**Operator precedence (low to high):**
1. `or`
2. `and`
3. `not`
4. `?=`, `>`, `<`, `>=`, `<=`
5. `&` (concatenation)
6. `+`, `-`
7. `*`, `/`, `%`
8. Unary `-`, `not`
9. `..` (system operations)
10. `.` (path traversal), `[]` (bracket filter)

**Success criteria:**
- Parse `x = 1 + 2 * 3` with correct precedence (multiply before add)
- Parse `my.contacts[name ?= "Joe"].age`
- Parse `if x > 5: y = 10` (single-line)
- Parse multi-line if/else blocks
- Parse `consider` blocks
- Parse procedure definitions with default parameters
- Parse watcher definitions
- Parse `new person employee { name: "Bob" }`
- Parse `mytext..upper..trim(" ")` as chained system operations
- Parse `@2026-02-01` as time literal in expressions

### 4.3: Block Structure

**Files:** Updates to `internal/mbl/parser/parser.go`

Handle indentation-based block structure using INDENT/DEDENT tokens from the lexer.

**Success criteria:**
- Nested if/else blocks parse correctly
- Procedures with multi-line bodies
- For loops with nested if statements
- `consider` with multiple cases

---

## Step 5: MBL Interpreter — Core

**Implements:** Spec sections "Operators", "Type Coercion", "Resilience", "Control Flow"

**Goal:** Execute an MBL AST against the storage engine.

### 5.1: Expression Evaluator

**Files:** `internal/mbl/interpreter/interpreter.go`, `internal/mbl/interpreter/eval.go`, `internal/mbl/interpreter/interpreter_test.go`

**Environment/scope model:**
```go
type Scope struct {
    local   map[string]Value  // in-memory local variables
    parent  *Scope            // enclosing scope
    tree    *storage.Tree     // persistent storage access
    agent   uint64            // current agent identity
}
```

**What to implement:**
- Arithmetic with coercion
- Concatenation with coercion
- Comparison (including Time precision rules)
- Logical operators
- Unknown propagation: any operation on Unknown produces Unknown
- Resilience: division by zero → Unknown, type mismatch → Unknown

**Success criteria:**
- `1 + 2` → 3
- `"hello" & " " & "world"` → "hello world"
- `1 / 0` → Unknown("division by zero")
- `Unknown + 5` → Unknown (propagation)
- `@2026-02-01 ?= @2026-02-01 15:30:00` → true
- `¤19.95 + ¤5.00` → ¤24.95

### 5.2: Control Flow

**Files:** Updates to `internal/mbl/interpreter/interpreter.go`

**What to implement:**
- `if` / `else if` / `else`
- `consider` with value matching, operator prefixes, `or` in conditions
- `while` loops
- `for` loops over attributes
- `pass` as no-op

**Success criteria:**
- If/else selects correct branch
- Consider matches bare values with `?=`, operator prefixes (`< 13`), and `or` groups
- Consider short-circuits on first match
- While loop terminates correctly
- For loop iterates all children of a path
- For loop variable accesses sub-attributes (`item.quantity`)

### 5.3: Path Resolution and Assignment

**Files:** Updates to `internal/mbl/interpreter/interpreter.go`

**What to implement:**
- Dot-notation path traversal against the tree
- `my` resolves to `world.agent.{identity}`
- `world` resolves to the tree root
- `~` resolves to current agent's home
- Assignment to paths writes to storage (creating instances)
- Bracket filter evaluation within paths
- Line prefix rules (`.` for relative, bare for upstream cascade)

**Success criteria:**
- `my.contacts.joe.name = "Joe"` writes correctly
- Reading `my.contacts.joe.name` returns "Joe"
- `my.contacts[name ?= "Joe"]` returns matching records
- Assignment of same value creates no new instance
- Assignment of different value creates new instance with correct timestamp and author

### 5.4: System Operations

**Files:** `internal/mbl/interpreter/sysops.go`, `internal/mbl/interpreter/sysops_test.go`

**What to implement:** All `..` operations from the spec.

**Text operations:** `length`, `find`, `extract` (both overloads), `replace` (both overloads), `split`, `trim`, `pad`, `upper`, `lower`, `title`

**Number operations:** `round`, `floor`, `ceiling`, `truncate`, `abs`, `sign`, `min`, `max`, `clamp`, `power`, `root`, `log`, `sin`, `cos`, `tan`, `asin`, `acos`, `atan`

**Record/List operations:** `count`, `combine`, `remove` (by index, by value, by range), `reverse`, `sort` (alphanumeric and custom procedure), `append`, `prepend`, `insert`, `inject`

**Universal operations:** `type`

**Text constants:** `quote`, `tab`, `newline`, `empty`

**Numeric constants:** `pi`, `euler`

**Standalone functions:** `symbol(value)`, `random(min, max)`

**Success criteria:**
- `"hello"..upper` → "HELLO"
- `"hello world"..split(" ")` → ["hello", "world"]
- `"a,,b"..split(",")` → ["a", empty, "b"]
- `"x"..pad(9, "-+")` → "x-+-+-+-+"
- `42..round` → 42, `42.567..round(2)` → 42.57
- `(-5)..abs` → 5
- `[3,1,2]..sort("ascending")` → [1,2,3]
- `[3,1,2]..sort("descending")` → [3,2,1]
- `{name: "Joe", age: 34}..inject("Hello {name}, age {age}")` → "Hello Joe, age 34"
- `"hello"..type` → "Text"
- Chaining: `" HELLO "..trim(" ")..lower` → "hello"

---

## Step 6: MBL Interpreter — Procedures and Watchers

**Implements:** Spec sections "Procedures", "Watchers", "Execution Model"

### 6.1: Procedures

**Files:** Updates to `internal/mbl/interpreter/interpreter.go`

**What to implement:**
- Procedure definition (stored as Procedure value type)
- Parameter binding with default values
- Procedure calls with argument evaluation
- `return` statement
- Procedure scope (local variables, access to enclosing scope)
- `@code` meta attribute for persistent procedures

**Success criteria:**
- Define `add(a, b): return a + b`, call `add(3, 4)` → 7
- Default parameters: `greet(name, greeting = "Hello")` works with 1 or 2 args
- Procedures return Nothing if no explicit return
- Procedures stored in the tree can be called by path

### 6.2: Watchers

**Files:** `internal/watcher/engine.go`, `internal/watcher/engine_test.go`

**What to implement:**
- Watcher definition (stored as Watcher value type)
- `watching` list — paths being monitored
- `enabled` attribute — defaults to true
- Watcher triggers when any watched value changes
- `(quietly)` modifier suppresses watcher triggers
- `@code` meta attribute
- Watcher self-triggering (watcher modifies its own watched value without `(quietly)`)

**Success criteria:**
- Watcher fires when watched value changes
- Watcher does not fire when `(quietly)` is used
- Watcher can modify data (writes to storage)
- Setting `enabled = false` prevents firing
- Adding to `watching` list monitors new path
- Removing from `watching` list stops monitoring that path
- Assigning watcher to Nothing removes it
- `(quietly)` inside watcher prevents self-retriggering

### 6.3: Heartbeat Execution Loop

**Files:** `internal/watcher/heartbeat.go`, `internal/watcher/heartbeat_test.go`

**What to implement:**
- ⅓ second tick cycle
- Detect changed values, find matching watchers, execute them
- Atomicity: hold changes until tick completes
- Rollback on unhandled Unknown
- Handled Unknowns (caught in watcher code) commit normally

**Success criteria:**
- Changes within a watcher tick are atomic — all commit or all rollback
- Unhandled Unknown causes full rollback of that tick's changes
- Watcher that catches Unknown and handles it commits successfully
- Multiple watchers firing in same tick execute independently

---

## Step 7: Heritability and Instantiation

**Implements:** Spec sections "Heritability" and "Instantiation"

**Files:** `internal/mbl/interpreter/new.go`, `internal/mbl/interpreter/new_test.go`

**What to implement:**
- `new` keyword creates a child from one or more source records
- Heritability modifiers: `(copy)` (default), `(link)`, `(reset expr)`, `(exclude)`
- Multiple sources applied left to right, later wins on collision
- Inline records `{}` as sources in any position
- Recursive instantiation (sub-records are also instantiated)
- `(reset)` evaluates expression for each new instance

**Success criteria:**
- `new template` creates child with copied values
- `(exclude)` attributes are omitted in child
- `(link)` attributes are references — writing to child writes to parent
- `(reset random(100,999))` produces different values per instantiation
- `new person employee { name: "Bob" }` merges all three sources
- Inline `{}` can appear at any position in source list
- Nested records are recursively instantiated

---

## Step 8: Stamps, Filters, and Permissions

**Implements:** Spec section "Stamps, Filters, and Permissions"

### 8.1: Stamps

**Files:** `internal/mbl/interpreter/stamps.go`, `internal/mbl/interpreter/stamps_test.go`

Stamps project additional meta attributes onto incoming data based on matching rules.

**Success criteria:**
- Stamp with simple condition matches correctly
- Stamp injects attributes into matching records
- `(protected)` attributes resist stamp override
- `@embed` applies stamp attributes to nested records

### 8.2: Filters

**Files:** `internal/mbl/interpreter/filters.go`, `internal/mbl/interpreter/filters_test.go`

Filters control what an agent sees.

**Success criteria:**
- Filter hides attributes that don't match
- Filter operates within permission bounds (can't see what you lack permission to read)

### 8.3: Permissions

**Files:** `internal/security/permissions.go`, `internal/security/permissions_test.go`

**Permission types:** `@read`, `@write`, `@expand`, `@grant`, `@purge`

**Success criteria:**
- Agent without `@read` cannot read the attribute
- Agent without `@write` cannot modify the attribute
- Agent without `@expand` cannot add new attributes
- Permissions inherit down the tree until overridden
- Default under `~`: owner-only. Default under `world`: read-open, write-closed

---

## Step 9: Service and Client

**Implements:** Spec section "Components"

### 9.1: Local Socket Protocol

**Files:** `internal/protocol/encode.go`, `internal/protocol/decode.go`, `internal/protocol/messages.go`, `internal/protocol/protocol_test.go`

**Implement the wire format:**
- Message header: version (1), type (1), sequence (4), payload_length (4), payload (variable), checksum (4)
- Tagged field encoding/decoding
- All message types (connection, data, mesh, recovery)

**Success criteria:**
- Encode a WRITE message, decode it, confirm exact match
- Unknown message types are skipped without error
- Unknown tagged fields are skipped without error
- Checksum detects corruption

### 9.2: Service Daemon

**Files:** `cmd/amorphd/main.go`, `internal/service/service.go`

**What to implement:**
- Listen on local UNIX socket
- Listen on network socket (TCP)
- Accept connections, read messages, dispatch to storage engine
- Handle READ, WRITE, PURGE operations
- Return responses with sequence number correlation
- Standalone mode (single node, full functionality)

**Success criteria:**
- Service starts, listens on both sockets
- Client connects via local socket, writes data, reads it back
- Multiple concurrent clients work correctly
- Service persists data across restart

### 9.3: Client REPL and Script Runner

**Files:** `cmd/amorph/main.go`, `cmd/amorph/repl.go`, `cmd/amorph/script.go`

**What to implement:**
- Interactive REPL with `..` block continuation
- Script execution from `.mbl` files
- Pipe support (`cat script.mbl | amorph run`)
- `my.computer.output()` prints to terminal stdout
- `my.computer.input()` reads from terminal stdin
- `my.computer.error()` prints to terminal stderr
- Connection options (`--node`, `--identity`)

**Success criteria:**
- REPL: type `my.test = "hello"`, then `my.test` shows "hello"
- REPL: multi-line block with indentation works
- Script: `amorph run test.mbl` executes correctly
- Pipe: `echo 'my.test = "hello"' | amorph run` works
- Output from `my.computer.output()` appears on stdout

### 9.4: Control Tool

**Files:** `cmd/amorphctl/main.go`

**What to implement:**
- `amorphctl status` — show service status
- `amorphctl stop` — graceful shutdown
- `amorphctl compact` — trigger defragmentation
- Connects via local socket only

**Success criteria:**
- `amorphctl status` returns service state (running, uptime, zone count)
- `amorphctl stop` triggers graceful shutdown with data flush

---

## Step 10: Mesh Networking

**Implements:** Spec sections "Mesh Architecture" and "Protocol"

### 10.1: Node Identity and Bootstrap

**Files:** `internal/mesh/bootstrap.go`, `internal/mesh/identity.go`, `internal/mesh/bootstrap_test.go`

**What to implement:**
- CV syllable identity generation with blacklist
- Self-genesis for first node
- Connect to seed node, perform key exchange
- Request and receive identity
- Peer exchange (receive list of known nodes)
- Standalone mode until mesh joined

**Success criteria:**
- First node self-generates identity and starts
- Second node connects to first, receives identity
- Both nodes know about each other after bootstrap
- Generated identities are pronounceable CV syllable patterns
- Blacklisted combinations are excluded

### 10.2: Consistent Hashing and Zone Assignment

**Files:** `internal/zone/hashring.go`, `internal/zone/hashring_test.go`

**What to implement:**
- Consistent hash ring with virtual nodes
- Zone-to-node assignment by hashing subtree root path
- Replica assignment (next N nodes on ring)
- Node join: recalculate affected zones
- Node leave: redistribute affected zones

**Success criteria:**
- Hash a path, get deterministic node assignment
- Add a node to the ring, confirm minimal zone redistribution
- Remove a node, confirm zones move to next node on ring
- Two nodes calculate the same assignment for the same path

### 10.3: Heartbeat and Gossip

**Files:** `internal/mesh/heartbeat.go`, `internal/mesh/gossip.go`, `internal/mesh/heartbeat_test.go`

**What to implement:**
- ⅓ second heartbeat tick
- Zone cluster peers: direct heartbeat with replication data
- Gossip peers: broader mesh awareness (membership, zone changes)
- Clock synchronization
- Failure detection (missed heartbeats → node presumed down)

**Success criteria:**
- Two nodes heartbeat to each other at ⅓ second intervals
- Missed heartbeats trigger failure detection after configurable threshold
- Gossip propagates a new node announcement to all nodes within several ticks
- Clock values stay synchronized within acceptable drift

### 10.4: Replication

**Files:** `internal/mesh/replication.go`, `internal/mesh/replication_test.go`

**What to implement:**
- Write authority pushes changes to replicas on heartbeat
- Replicas acknowledge receipt
- Reads can be served by any replica
- Zone transfer for new zone assignment (bulk data)

**Success criteria:**
- Write to authority node, confirm replica receives it within one heartbeat
- Read from replica returns current data
- New node receiving a zone gets full data transfer
- Authority failure: replica can serve reads (writes queued until new authority)

### 10.5: Zone Splitting

**Files:** `internal/zone/split.go`, `internal/zone/split_test.go`

**What to implement:**
- Authority monitors zone load (size, query rate, write rate)
- Autonomous split decision when thresholds exceeded
- Spatial splitting (by subtree children)
- Temporal splitting (by instance age)
- Announce new zone boundaries via gossip
- Data migration to new zone assignments

**Success criteria:**
- Zone exceeding size threshold triggers split
- After split, data is correctly distributed between new zones
- New zone assignments are communicated to all nodes
- Queries route correctly to new zones after split

---

## Step 11: Security

**Implements:** Spec section "Security"

### 11.1: Encryption

**Files:** `internal/security/crypto.go`, `internal/security/crypto_test.go`

**What to implement:**
- Diffie-Hellman key exchange
- Post-quantum encryption upgrade
- Pairwise node-to-node encryption
- Agent-level encryption (secrets encrypted with derived key)

### 11.2: Authentication

**Files:** `internal/security/auth.go`, `internal/security/auth_test.go`

**What to implement:**
- Derived key from passphrase + device secret
- Challenge-response authentication flow
- Public key storage in mesh, private key encrypted with derived key

**Success criteria:**
- Agent authenticates with correct passphrase + device → success
- Wrong passphrase → failure
- Wrong device secret → failure
- Public key retrievable from mesh by any node

### 11.3: Recovery

**Files:** `internal/security/recovery.go`, `internal/security/recovery_test.go`

**What to implement:**
- Shamir's Secret Sharing: split recovery key into N shards
- Threshold reconstruction (M of N shards needed)
- Recovery flow: submit shards, reconstruct key, re-encrypt with new factors
- Single-node degraded mode (node is sole recovery agent)

**Success criteria:**
- Split key into 3 shards with threshold 2
- Any 2 of 3 shards reconstructs the key
- 1 of 3 shards is insufficient
- Full recovery flow: new factors, re-encryption, shard redistribution

---

## Step 12: Integration Testing

**Goal:** End-to-end tests that exercise the full system.

**Files:** `test/integration/`

### 12.1: Single Node

- Start service, connect client, create data, query it, shut down, restart, confirm data persisted
- Watchers fire and modify data correctly
- Procedures stored in tree can be called
- Temporal queries return correct historical values
- Permissions block unauthorized access

### 12.2: Two Node Mesh

- Start two nodes, bootstrap second from first
- Write on node A, read on node B
- Zone assignment via consistent hashing works
- Replication delivers data within one heartbeat
- Node B failure: node A continues serving
- Node B recovery: data syncs correctly

### 12.3: Multi-Node Mesh

- Three+ nodes with zone splitting
- Distributed watcher pipeline across nodes
- Agent connects to any node, reaches their data
- Authentication works from any node
- Recovery with Shamir shards across nodes

### 12.4: Stress and Chaos

- High write volume, confirm no data loss
- Kill a node mid-heartbeat, confirm rollback and recovery
- Network partition simulation, confirm correct behavior on rejoin
- Large data volumes, confirm defragmentation works

---

## Dependency Order Summary

```
Step 1 (Storage)
  └→ Step 2 (Types)
       └→ Step 3 (Lexer)
            └→ Step 4 (Parser)
                 └→ Step 5 (Interpreter Core)
                      ├→ Step 6 (Procedures & Watchers)
                      ├→ Step 7 (Heritability & New)
                      └→ Step 8 (Stamps, Filters, Permissions)
                           └→ Step 9 (Service & Client)
                                └→ Step 10 (Mesh)
                                     └→ Step 11 (Security)
                                          └→ Step 12 (Integration)
```

Steps 6, 7, and 8 can be developed in parallel after Step 5.
Steps 10 and 11 can overlap (security is needed for mesh but can be stubbed initially).

---

## Notes for Claude Code Sessions

1. **Always read `state/STATUS.md` first** to understand current progress.
2. **Always update `state/STATUS.md`** when completing a substep or encountering an issue.
3. **Reference the spec** (`AmorphDB.md`) for design details. Each step notes which spec section it implements.
4. **Run tests** after each substep. Do not proceed to the next substep with failing tests.
5. **Keep files focused.** One concern per file. If a file exceeds ~500 lines, consider splitting.
6. **Use the interfaces** defined in earlier steps. Do not bypass them.
7. **Errors produce Unknown values** in MBL, not panics in Go. Only panic for truly unrecoverable situations (e.g., corrupt storage file header).
