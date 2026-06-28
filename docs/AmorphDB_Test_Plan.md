# AmorphDB Comprehensive Test Plan

This document is a test plan for Claude Code to implement against the AmorphDB codebase. Each section describes a test area with specific test cases. Tests should be written using Go's built-in `testing` package and placed in corresponding `_test.go` files alongside the code they test, or under `test/integration/` for cross-package tests.

**Guiding principles:**
- Every test should be independent — no test should depend on another test's side effects
- Use `t.Helper()` for shared setup functions
- Use table-driven tests where there are many similar cases
- Use `t.Parallel()` where safe to do so
- Name tests descriptively: `TestStorageEngine_ValueDedup_IdenticalValuesShareStorage`
- Each section below maps to a package or subsystem; implement all tests for a section before moving on
- Where the spec says "🚧 not yet implemented," skip those tests but leave them as `t.Skip("not yet implemented: <feature>")` placeholders

---

## 1. Storage Engine (`internal/storage/`)

### 1.1 Value Storage

| # | Test Case | What to Verify |
|---|-----------|----------------|
| 1.1.1 | Store and retrieve a text value | Write a text value, read it back, confirm byte-exact match |
| 1.1.2 | Store and retrieve every data type | One value per type: Text, Number, Time, Money, Picture (small RGBA buffer), Reference, Procedure (stored as bytes), Watcher (stored as bytes) |
| 1.1.3 | Value deduplication | Write the same text value twice, confirm only one copy exists on disk and both references resolve correctly |
| 1.1.4 | Dedup with different types | Write Number `42` and Text `"42"` — these are different types and must NOT be deduped |
| 1.1.5 | Reference counting on dedup | Write a value 3 times (3 references), delete 2 references, confirm value still exists; delete last reference, confirm value is eligible for GC |
| 1.1.6 | Large value storage | Store a value >1MB (e.g. a Picture blob), read it back correctly |
| 1.1.7 | Empty value storage | Store empty text `""`, confirm it stores and retrieves distinctly from a null/missing value |
| 1.1.8 | Concurrent writes | 100 goroutines each writing a unique value simultaneously — no corruption, all values retrievable |
| 1.1.9 | Value file recovery after crash | Write values, simulate incomplete write (truncated file), confirm the storage engine detects and recovers gracefully on next open |

### 1.2 Instance Storage

| # | Test Case | What to Verify |
|---|-----------|----------------|
| 1.2.1 | Create instance and retrieve | Create an instance pointing to a value, retrieve it by ID, confirm all fields (timestamp, agent, value ref, attribute ID) |
| 1.2.2 | Instance chain — forward | Create 5 instances for the same attribute, walk the chain via `next_instance_id` from first to last |
| 1.2.3 | Instance chain — backward | Walk the same chain via `previous_instance_id` from last to first |
| 1.2.4 | Instance timestamp ordering | Instances for the same attribute must be in strictly ascending timestamp order |
| 1.2.5 | Temporal query — "as of" | Given instances at T1, T3, T5, query "as of T2" must return T1's value; "as of T3" returns T3; "as of T4" returns T3 |
| 1.2.6 | Temporal query — "before" | Query `< T3` returns T1 only |
| 1.2.7 | Temporal query — "at or before" | Query `<= T3` returns T1 and T3 |
| 1.2.8 | Temporal query — "after" | Query `> T3` returns T5 only |
| 1.2.9 | Temporal query — range | Query `> T1, < T5` returns T3 only |
| 1.2.10 | No-change suppression | Assign the same value twice to an attribute — second assignment must NOT create a new instance |
| 1.2.11 | Instance with TTL | Create an instance with TTL, confirm it reports as expired after the TTL elapses |
| 1.2.12 | Instance meta attributes | Verify `@time`, `@agent`, `@statement_id`, `@previous_instance_id`, `@next_instance_id`, `@size`, `@ttl` are all readable |

### 1.3 Attribute Storage

| # | Test Case | What to Verify |
|---|-----------|----------------|
| 1.3.1 | Create attribute and retrieve | Create a named attribute, retrieve by name and by ID |
| 1.3.2 | Parent-child relationship | Create parent, then child under parent — child's parent ID points to parent |
| 1.3.3 | Multiple children | Create 10 children under one parent, enumerate all children |
| 1.3.4 | Attribute name uniqueness | Creating two attributes with the same name under the same parent should fail or return the existing one (per design) |
| 1.3.5 | Deep hierarchy | Create a 20-level deep path (a.b.c.d...t), traverse from root to leaf and back |
| 1.3.6 | Current instance pointer | After writing 3 instances, the attribute's current instance pointer points to the most recent |
| 1.3.7 | First instance pointer | The attribute's first instance pointer always points to the oldest instance |
| 1.3.8 | Delete attribute | Remove an attribute, confirm it and all its sub-attributes and instances are marked for purge |
| 1.3.9 | Numeric indexing (list) | Create attributes indexed 0, 1, 2, 3 — enumerate in order, confirm count = 4 |
| 1.3.10 | Mixed text and numeric children | An attribute can have both named and indexed children simultaneously |

### 1.4 Storage File Operations

| # | Test Case | What to Verify |
|---|-----------|----------------|
| 1.4.1 | Clean open | Open a fresh data directory, confirm all index files are created |
| 1.4.2 | Reopen persistence | Write data, close, reopen — all data intact |
| 1.4.3 | Compaction | Write 1000 values, delete 500, compact — file shrinks, surviving values intact, instance offsets updated |
| 1.4.4 | Compaction under concurrent reads | Start compaction while reads are ongoing — reads must not fail or return corrupt data |
| 1.4.5 | Free list management | Delete values, confirm free space is tracked and reused by subsequent writes |
| 1.4.6 | Large file handling | Write enough data to exceed 1GB file size, confirm correct behavior with 64-bit offsets |
| 1.4.7 | Corrupted index detection | Manually corrupt an index file header, confirm the engine detects it on open and reports a clear error |

---

## 2. Type System (`internal/types/`)

### 2.1 Core Types

| # | Test Case | What to Verify |
|---|-----------|----------------|
| 2.1.1 | Text — UTF-8 round-trip | Store/retrieve multi-byte UTF-8 (emoji, CJK, Arabic) |
| 2.1.2 | Text — empty string | Empty string is a valid value distinct from null |
| 2.1.3 | Number — float64 precision | Store max float64, min float64, epsilon-adjacent values, NaN, ±Inf — all round-trip or are rejected per spec |
| 2.1.4 | Number — integer representation | Store `42`, read back, confirm no precision loss |
| 2.1.5 | Time — range | Store times at Unix epoch, year 2038 boundary, year 9999, confirm all round-trip (2038-proofed) |
| 2.1.6 | Time — sub-second precision | Store time with millisecond precision, read back, confirm no truncation |
| 2.1.7 | Money — currency preservation | Store `$143.68`, `€21.80`, `¥1000` — currency symbol preserved with value |
| 2.1.8 | Money — precision | Store `$0.01`, `$999999999.99` — no floating-point drift |
| 2.1.9 | Picture — RGBA 16-bit | Store a 2x2 RGBA16 image (16 bytes per pixel), read back, confirm bit-exact |
| 2.1.10 | Reference — path storage | Store reference `""world.agent.kalevo""`, retrieve and confirm path |
| 2.1.11 | Reference — resolution | Dereference a reference, confirm it reaches the correct attribute |

### 2.2 Meta Types

| # | Test Case | What to Verify |
|---|-----------|----------------|
| 2.2.1 | Unknown — creation with reason | Create Unknown `#offline`, confirm reason string is preserved |
| 2.2.2 | Unknown — propagation | `5 + #offline` yields `#offline` (not a panic, not a zero) |
| 2.2.3 | Queued — creation with info | Create Queued `?tomorrow`, confirm delivery info preserved |
| 2.2.4 | Queued — propagation | `5 + ?tomorrow` yields `?tomorrow` |
| 2.2.5 | Redacted — display | A redacted value displays as asterisks matching original character count |
| 2.2.6 | Redacted — no direct creation | Confirm that Redacted cannot be assigned directly — it only appears as a filtered result |

### 2.3 Type Coercion

| # | Test Case | What to Verify |
|---|-----------|----------------|
| 2.3.1 | Text → Number | `"42"` coerces to `42.0`; `"hello"` coerces to Unknown |
| 2.3.2 | Number → Text | `42` coerces to `"42"` (or appropriate formatting) |
| 2.3.3 | Time → Number | Coerces to Unix timestamp |
| 2.3.4 | Money → Number | `$143.68` coerces to `143.68`, discarding currency |
| 2.3.5 | Boolean truthiness — Number | `0` is false, `1` is true, `-1` is true |
| 2.3.6 | Boolean truthiness — Text | `""` is false, `"anything"` is true |
| 2.3.7 | Boolean truthiness — Time | Zero time is false, any other time is true |
| 2.3.8 | Incompatible coercion | Picture → Number yields Unknown |

---

## 3. MBL Lexer (`internal/mbl/lexer/`)

### 3.1 Token Recognition

| # | Test Case | What to Verify |
|---|-----------|----------------|
| 3.1.1 | Identifiers | `account`, `_private`, `Account`, `café` — all recognized as identifiers |
| 3.1.2 | Reserved words | All 30+ reserved words tokenize as their specific token type, not as identifiers |
| 3.1.3 | Numbers — integer | `42`, `1_000_000` — underscore stripped, value correct |
| 3.1.4 | Numbers — float | `3.14159`, `1_000.50` |
| 3.1.5 | Numbers — invalid | `.5` (starts with period), `5.` (ends with period) — must be rejected per spec |
| 3.1.6 | Numbers — negative | `-42`, `-3.14` — negative sign is part of number or unary operator (per grammar) |
| 3.1.7 | Time literals | `@2026-01-15`, `@2026-01-15 14:30`, `@2026-01-15 14:30:22`, `@2026-01-15 14:30:22.500` |
| 3.1.8 | Text — simple | `"Hello"` |
| 3.1.9 | Text — multi-quote | `""He said "Hello"""` — outer double-quotes delimit, inner single-quotes are content |
| 3.1.10 | Text — triple-quote | `"""Complex "quoted" text"""` |
| 3.1.11 | Money literals | `$143.68`, `€21.80` |
| 3.1.12 | Operators — arithmetic | `+`, `-`, `*`, `/`, `%`, `^` |
| 3.1.13 | Operators — comparison | `=`, `!=`, `<`, `>`, `<=`, `>=` |
| 3.1.14 | Operators — logical | `and`, `or`, `not` |
| 3.1.15 | Operators — type-safe | `?=`, `?!=`, `?<`, `?>`, `?<=`, `?>=` |
| 3.1.16 | Operators — assignment | `=`, `:=` (distinguish from comparison `=`) |
| 3.1.17 | Path prefixes | `~`, `.`, `+`, bare |
| 3.1.18 | Brackets | `[`, `]`, `{`, `}`, `(`, `)` |
| 3.1.19 | Comments | `# This is a comment` — everything after `#` to EOL is discarded |
| 3.1.20 | Comment vs Unknown | `#offline` when used as a value (starts with `#` but no space) vs `# comment` |
| 3.1.21 | References | `""world.agent.kalevo""` — double-quoted path tokens |
| 3.1.22 | Dot and double-dot | `.` for path, `..` for collection operations |
| 3.1.23 | `@` disambiguation | `@2026` → time literal; `@time` → meta attribute; bare `@` → instance timestamp reference |
| 3.1.24 | Whitespace handling | Tabs, spaces, newlines — all as whitespace; indentation level tracking for scope |
| 3.1.25 | String concatenation operator | `&` for text join |
| 3.1.26 | Unicode identifiers | Identifiers with Unicode letters |
| 3.1.27 | Empty input | Empty string produces only EOF token |
| 3.1.28 | Unterminated string | `"Hello` with no closing quote — produces error token with clear message |

### 3.2 Indentation / Scope

| # | Test Case | What to Verify |
|---|-----------|----------------|
| 3.2.1 | Indent produces INDENT token | Going from 0 to 4 spaces emits INDENT |
| 3.2.2 | Dedent produces DEDENT token | Going from 4 to 0 spaces emits DEDENT |
| 3.2.3 | Multiple dedent levels | Going from 8 to 0 spaces emits 2 DEDENTs |
| 3.2.4 | Inconsistent indentation | Mixing tabs and spaces produces a clear error |
| 3.2.5 | Blank lines ignored | Blank lines between indented blocks do not emit spurious INDENT/DEDENT |

---

## 4. MBL Parser (`internal/mbl/parser/`)

### 4.1 Expressions

| # | Test Case | What to Verify |
|---|-----------|----------------|
| 4.1.1 | Arithmetic precedence | `2 + 3 * 4` → AST showing `(2 + (3 * 4))` |
| 4.1.2 | Exponentiation right-associativity | `2 ^ 3 ^ 2` → `(2 ^ (3 ^ 2))` |
| 4.1.3 | Parentheses override | `(2 + 3) * 4` → `((2 + 3) * 4)` |
| 4.1.4 | Logical and/or equal precedence | `a and b or c` → `((a and b) or c)` (left-associative, equal precedence) |
| 4.1.5 | Not precedence | `not a and b` → `((not a) and b)` |
| 4.1.6 | Comparison chain | `a = b` is comparison; `x = 5` is assignment (context-dependent) |
| 4.1.7 | Path expression | `my.account.balance` → nested member access |
| 4.1.8 | Path with brackets | `my.users[active = true]` → filter expression |
| 4.1.9 | Path with projection | `my.users{ name, age }` → projection node |
| 4.1.10 | Brackets + projection | `my.users[active = true]{ name, email }` → filter then project |
| 4.1.11 | Temporal in brackets | `person.name[@2025-06-01]` → "as of" temporal query |
| 4.1.12 | Mixed temporal and attribute filter | `person[name = "bob", @ < @2025-04-15]` |
| 4.1.13 | Record literal | `{ name: "Matt", age: 55 }` → record literal AST node |
| 4.1.14 | Nested record literal | `{ address: { city: "Rome", state: "NY" } }` |
| 4.1.15 | Type-safe comparison | `x ?= 5` parses to type-safe equality node |
| 4.1.16 | String concatenation | `"Hello" & " " & "World"` |
| 4.1.17 | Collection operation | `x..count`, `x..combine(",")`, `x..remove(2)` |
| 4.1.18 | Meta attribute access | `my.account.balance.@time` |
| 4.1.19 | Instance meta in brackets | `my.log[@agent = ""world.agent.kalevo""]` |

### 4.2 Statements

| # | Test Case | What to Verify |
|---|-----------|----------------|
| 4.2.1 | Value assignment | `my.x = 42` → assignment AST node |
| 4.2.2 | Definition | `my.func: procedure(x): return x * 2` → definition node |
| 4.2.3 | Append assignment | `+my.list = "item"` → append assignment node |
| 4.2.4 | Quiet assignment | `my.x = (quietly) "value"` → quiet modifier on assignment |
| 4.2.5 | If statement | `if x > 5:` followed by indented block |
| 4.2.6 | If/else | `if ... else ...` |
| 4.2.7 | If/else if/else | Three-branch conditional |
| 4.2.8 | For-each loop | `for item in collection:` |
| 4.2.9 | For-each with index | `for item, index in collection:` |
| 4.2.10 | While loop | `while condition:` |
| 4.2.11 | Catch/else | `catch:` ... `else unknown:` ... `else queued:` |
| 4.2.12 | Return with value | `return x * 2` |
| 4.2.13 | Return void | `return` |
| 4.2.14 | Procedure definition | Full multi-line procedure with params |
| 4.2.15 | Watcher definition | `watch(path):` with body |
| 4.2.16 | Multi-path watcher | `watch(path1, path2, path3):` |
| 4.2.17 | Append watcher | `watch append(my.orders) as orders:` |
| 4.2.18 | Append watcher with predicate | `watch append(my.orders[priority ?= "urgent"]) as urgent:` |
| 4.2.19 | Embed statement | `embed my.context.stamp` |
| 4.2.20 | Spread syntax | `...my.context.stamp` (equivalent to embed) |
| 4.2.21 | Scope set | `my.scope.` — trailing dot sets scope |
| 4.2.22 | Scope-relative reference | `.local.variable` inside a scope block |
| 4.2.23 | Break statement | `break` inside a loop |

### 4.3 Error Recovery

| # | Test Case | What to Verify |
|---|-----------|----------------|
| 4.3.1 | Missing colon after if | `if x > 5` (no colon) — clear error with line number |
| 4.3.2 | Unmatched bracket | `my.list[0` — error identifies missing `]` |
| 4.3.3 | Invalid operator | `my.x == 5` — `==` is not a valid operator |
| 4.3.4 | Unexpected token | `my.x = = 5` — double equals in wrong position |
| 4.3.5 | Empty procedure body | `my.func: procedure():` with no body — error or empty proc |

---

## 5. MBL Interpreter (`internal/mbl/interpreter/`)

### 5.1 Expression Evaluation

| # | Test Case | What to Verify |
|---|-----------|----------------|
| 5.1.1 | Arithmetic | `2 + 3` → `5`, `10 / 3` → `3.333...`, `2 ^ 10` → `1024`, `10 % 3` → `1` |
| 5.1.2 | String concatenation | `"Hello" & " " & "World"` → `"Hello World"` |
| 5.1.3 | Comparison | `5 > 3` → true, `"abc" < "abd"` → true (lexicographic) |
| 5.1.4 | Logical | `true and false` → false, `true or false` → true, `not true` → false |
| 5.1.5 | Short-circuit | `false and error_func()` — error_func must NOT be called |
| 5.1.6 | Type-safe comparison with Unknown | `#offline ?= 5` → false (not Unknown propagation) |
| 5.1.7 | Type-safe comparison with Queued | `?tomorrow ?= 5` → false |
| 5.1.8 | Unknown propagation in arithmetic | `5 + #offline` → `#offline` |
| 5.1.9 | Queued propagation in arithmetic | `5 + ?tomorrow` → `?tomorrow` |
| 5.1.10 | Division by zero | `5 / 0` → Unknown with appropriate reason |
| 5.1.11 | Type coercion in mixed expressions | `"42" + 8` → `50` (text to number coercion) |
| 5.1.12 | Coercion failure | `"hello" + 8` → Unknown |

### 5.2 Variable Scope

| # | Test Case | What to Verify |
|---|-----------|----------------|
| 5.2.1 | Local variable lifecycle | Assign `x = 5` in an outer run, confirm `x` is not persisted after run ends |
| 5.2.2 | `my` persistence | Assign `my.value = 5`, run ends, new run reads `my.value` → `5` |
| 5.2.3 | `world` persistence | Assign `world.shared = "hello"`, confirm another agent can read it |
| 5.2.4 | Procedure local scope | Procedure assigns `x = 5` — caller's `x` is unaffected |
| 5.2.5 | Procedure parameter passing | Modify parameter inside procedure — original variable unaffected (pass by value) |
| 5.2.6 | Procedure persistent sub-attributes | `.count = .count + 1` inside a procedure — persists between calls |
| 5.2.7 | Nested scope resolution | Inner scope references resolve to nearest matching upstream scope |
| 5.2.8 | `~` resolves to agent home | Inside any context, `~.x` and `my.x` reach the same attribute |
| 5.2.9 | Cascade attribute visibility | Attribute with `(cascade)` is visible to bare references in descendant scopes |

### 5.3 Control Flow

| # | Test Case | What to Verify |
|---|-----------|----------------|
| 5.3.1 | If — true branch | `if true: x = 1` → x is 1 |
| 5.3.2 | If — false branch | `if false: x = 1` → x is not set |
| 5.3.3 | If/else | Both branches reachable |
| 5.3.4 | If/else if/else | All three branches reachable |
| 5.3.5 | For-each | Iterate over list `[1, 2, 3]`, sum = 6 |
| 5.3.6 | For-each with index | Confirm index values 0, 1, 2 |
| 5.3.7 | For-each over empty collection | Body never executes |
| 5.3.8 | While loop | Count from 0 to 9, confirm 10 iterations |
| 5.3.9 | While — never enters | `while false:` — body never executes |
| 5.3.10 | Break in for loop | Break after 3 iterations of 10 — confirm only 3 executed |
| 5.3.11 | Break in while loop | Same for while |
| 5.3.12 | Nested loops with break | Break only exits inner loop |
| 5.3.13 | Catch — successful execution | `catch:` body runs without error → no `else` branch taken |
| 5.3.14 | Catch — Unknown handler | `catch:` body produces Unknown → `else unknown:` branch runs, reason accessible |
| 5.3.15 | Catch — Queued handler | Body produces Queued → `else queued:` branch runs |
| 5.3.16 | Return from procedure | Procedure returns value, caller receives it |
| 5.3.17 | Return from nested context | Return inside `if` inside procedure — still exits procedure |
| 5.3.18 | Void return | `return` with no value returns null |

### 5.4 Collection Operations

| # | Test Case | What to Verify |
|---|-----------|----------------|
| 5.4.1 | `..count` | List with 5 items → `..count` = 5 |
| 5.4.2 | `..count` on empty | Empty list → `..count` = 0 |
| 5.4.3 | `..count` on record | Record with 3 fields → `..count` = 3 |
| 5.4.4 | `..combine` | `[1, 2, 3]..combine(", ")` → `"1, 2, 3"` |
| 5.4.5 | `..remove` by index | Remove item at index 1 from `[a, b, c]` → `[a, c]` (reindexes) |
| 5.4.6 | `..remove` by value | Remove `"b"` from `[a, b, c]` → `[a, c]` |
| 5.4.7 | `..remove` range | Remove indices 1-2 from `[a, b, c, d]` → `[a, d]` |
| 5.4.8 | `..remove` by name on record | Remove field `"bob"` from contacts record |
| 5.4.9 | Append with `+` prefix | `+my.list = "milk"` appends to end |
| 5.4.10 | List indexing | `my.list[0]` returns first element |
| 5.4.11 | Negative index | `my.list[-1]` — behavior per spec (error or last item) |
| 5.4.12 | Out-of-bounds index | `my.list[999]` → Unknown |

### 5.5 Built-in Procedures

| # | Test Case | What to Verify |
|---|-----------|----------------|
| 5.5.1 | `length("Hello")` → 5 | Text operations |
| 5.5.2 | `substring("Hello", 1, 3)` → `"ell"` | |
| 5.5.3 | `find("Hello World", "World")` → index or match | |
| 5.5.4 | `replace("Hello", "l", "r")` → `"Herro"` | |
| 5.5.5 | `split("a,b,c", ",")` → list of 3 | |
| 5.5.6 | `trim("  hi  ")` → `"hi"` | |
| 5.5.7 | `upper("hello")` / `lower("HELLO")` | |
| 5.5.8 | `abs(-5)` → `5` | Math operations |
| 5.5.9 | `round(3.7)` → `4`, `floor(3.7)` → `3`, `ceiling(3.2)` → `4` | |
| 5.5.10 | `min(3, 7)` → `3`, `max(3, 7)` → `7` | |
| 5.5.11 | `sqrt(16)` → `4`, `sqrt(-1)` → Unknown | |
| 5.5.12 | `random()` → number in [0.0, 1.0) | |
| 5.5.13 | `now()` → current time (within tolerance) | Time operations |
| 5.5.14 | `type_of(42)` → `"Number"` | Type operations |
| 5.5.15 | `is_unknown(#offline)` → true, `is_unknown(5)` → false | |
| 5.5.16 | `is_queued(?tomorrow)` → true | |
| 5.5.17 | `convert("42", "Number")` → `42` | |
| 5.5.18 | `sort(collection, "name")` | Collection operations |
| 5.5.19 | `filter(collection, condition)` | |
| 5.5.20 | `map(collection, procedure)` | |
| 5.5.21 | `reduce(collection, procedure, initial)` | |

### 5.6 Bracket Queries

| # | Test Case | What to Verify |
|---|-----------|----------------|
| 5.6.1 | Index lookup | `my.list[0]` → first item |
| 5.6.2 | Attribute filter — equality | `person[name = "bob"]` returns matching records |
| 5.6.3 | Attribute filter — comparison | `person[age > 18]` |
| 5.6.4 | AND via comma | `person[name = "bob", age > 18]` — both conditions must match |
| 5.6.5 | AND via keyword | `person[name = "bob" and age > 18]` — same result |
| 5.6.6 | OR via keyword | `person[name = "bob" or name = "robert"]` |
| 5.6.7 | Mixed AND/OR with parens | `person[(name = "bob" or name = "robert"), age > 18]` |
| 5.6.8 | Temporal — as of | `person.name[@2025-06-01]` |
| 5.6.9 | Temporal — range | `person.name[>@2025-05-01, <@2025-06-01]` |
| 5.6.10 | Temporal + attribute | `person[name = "bob", @ < @2025-04-15]` |
| 5.6.11 | No matches | Query that matches nothing returns empty collection, not error |
| 5.6.12 | Meta attribute filter | `my.log[@agent = ""world.agent.kalevo""]` |
| 5.6.13 | Stamp filter | `my.projects[@stamp.department = "engineering"]` |
| 5.6.14 | Projection — named fields | `person{ name, age }` returns only those fields |
| 5.6.15 | Filter + projection | `person[active = true]{ name, email }` |

---

## 6. Procedures & Watchers (`internal/watcher/`)

### 6.1 Procedures

| # | Test Case | What to Verify |
|---|-----------|----------------|
| 6.1.1 | Define and call | Define `double(x): return x * 2`, call with `5`, get `10` |
| 6.1.2 | Multiple parameters | `add(a, b): return a + b` |
| 6.1.3 | Recursive procedure | Factorial function — correct result for small inputs |
| 6.1.4 | Procedure as first-class value | Store procedure in attribute, retrieve and call it |
| 6.1.5 | Procedure passed as parameter | `map(list, double)` — procedure passed as arg |
| 6.1.6 | Anonymous procedure | `procedure(x): return x * 2` used inline |
| 6.1.7 | Persistent sub-attributes | `.count = .count + 1` increments across calls |
| 6.1.8 | Procedure stored in hierarchy | Procedure stored at `my.functions.calc`, callable from another run |
| 6.1.9 | Procedure scope isolation | Procedure cannot accidentally modify caller's local variables |
| 6.1.10 | Procedure `my` access | Procedure can read/write `my.*` persistent paths |

### 6.2 Watchers — Core

| # | Test Case | What to Verify |
|---|-----------|----------------|
| 6.2.1 | Basic trigger | Define watcher on `my.x`, change `my.x` — watcher fires |
| 6.2.2 | No trigger on same value | Set `my.x = 5` when already 5 — no new instance, watcher does not fire |
| 6.2.3 | Watcher body execution | Watcher writes to `my.y` when `my.x` changes — `my.y` updated |
| 6.2.4 | Quiet assignment | `my.x = (quietly) "value"` — watcher on `my.x` does NOT fire |
| 6.2.5 | Watcher attributes | `@enabled`, `@watching`, `@code`, `@last_run`, `@run_count`, `@last_error` all readable |
| 6.2.6 | Disable watcher | Set `@enabled = false`, change watched path — watcher does not fire |
| 6.2.7 | Re-enable watcher | Set `@enabled = true` — watcher fires on next change |
| 6.2.8 | Watcher error handling | Watcher body produces unhandled Unknown — `@last_error` set, staged writes rolled back |
| 6.2.9 | Watcher with catch | Watcher handles Unknown internally — changes commit normally |
| 6.2.10 | Watcher persistent sub-attributes | `.data = .data + 1` inside watcher persists between firings |

### 6.3 Multi-Path Watchers

| # | Test Case | What to Verify |
|---|-----------|----------------|
| 6.3.1 | OR semantics | Watcher on `(path1, path2)` — changing either one triggers it |
| 6.3.2 | Both change in same tick | Both paths change in one heartbeat — watcher fires once, not twice |
| 6.3.3 | Neither changes | No change to watched paths in a tick — watcher does not fire |
| 6.3.4 | Three-path watcher | `watch(a, b, c):` — change only `c`, watcher fires |

### 6.4 Append Watchers

| # | Test Case | What to Verify |
|---|-----------|----------------|
| 6.4.1 | Basic append trigger | Append to `my.orders`, watcher fires with `orders` list containing the new item |
| 6.4.2 | Multiple appends in one tick | Append 3 items in one tick — `orders` list has all 3 in arrival order |
| 6.4.3 | Non-append change | Modify an existing item in the list — append watcher does NOT fire |
| 6.4.4 | Predicate filter | `watch append(my.orders[priority ?= "urgent"]) as urgent:` — only urgent items trigger |
| 6.4.5 | Predicate with no match | Append a non-matching item — watcher does not fire |
| 6.4.6 | Mixed matching/non-matching | Append 3 items where 2 match predicate — bound list has only the 2 matching |

### 6.5 Heartbeat Atomicity

| # | Test Case | What to Verify |
|---|-----------|----------------|
| 6.5.1 | All-or-nothing on success | Watcher writes to 5 paths — all 5 commit at heartbeat boundary |
| 6.5.2 | Rollback on unhandled Unknown | Watcher writes to 3 paths then hits Unknown — all 3 writes discarded |
| 6.5.3 | Caught Unknown allows commit | Watcher catches Unknown, writes other data — those writes commit |
| 6.5.4 | Commit buffer limit | Watcher exceeds 10,000 writes — produces Unknown "commit buffer exceeded" |
| 6.5.5 | Outer run staging | Outer run writes to 5 paths — all staged until program exits, then commit |
| 6.5.6 | Outer run rollback | Outer run hits unhandled Unknown at top level — all writes rolled back |
| 6.5.7 | Watcher cascade does not loop | Watcher A writes to path watched by Watcher B — B fires next tick, not same tick |

---

## 7. Stamps, Filters, Permissions (`internal/` relevant packages)

### 7.1 Stamps

| # | Test Case | What to Verify |
|---|-----------|----------------|
| 7.1.1 | Built-in stamps auto-applied | Write data — `@agent`, `@time` are set automatically |
| 7.1.2 | Custom stamps | Set `my.stamp.department = "engineering"`, write data — data carries `@stamp.department` |
| 7.1.3 | Stamp inheritance on copy | Copy data — stamps carry over |
| 7.1.4 | Stamp modification | Change agent's stamp, write new data — new data has new stamp, old data retains old stamp |
| 7.1.5 | Stamp query | `my.projects[@stamp.department = "engineering"]` returns correctly |
| 7.1.6 | Multiple custom stamps | Set 3 stamp fields, write data — all 3 present |

### 7.2 Embed

| # | Test Case | What to Verify |
|---|-----------|----------------|
| 7.2.1 | Basic embed | `embed my.context.stamp` makes stamp fields appear as siblings in the host record |
| 7.2.2 | Spread syntax | `...my.context.stamp` produces same result as `embed` |
| 7.2.3 | Collision — host wins | Host has field `title`, embed also has `title` — host value is returned |
| 7.2.4 | Collision — later embed wins | Two embeds conflict — second one's field value is used |
| 7.2.5 | Embed is not a value | Attempting to assign an embed to a variable produces an error |
| 7.2.6 | Multiple embeds | Two embeds in one record — all fields from both appear |
| 7.2.7 | Embedded record retains identity | Can still query the embedded record independently |

### 7.3 Filters

| # | Test Case | What to Verify |
|---|-----------|----------------|
| 7.3.1 | Filter hides matching data | Activate `@stamp.environment = "test"` filter — test data shows as Redacted |
| 7.3.2 | Filter is client-side only | Filter does not affect what another agent sees |
| 7.3.3 | Remove filter reveals data | Deactivate filter — data visible again |
| 7.3.4 | Multiple active filters | Stack two filters — both apply simultaneously |
| 7.3.5 | Redacted display | Filtered data appears as `***` (asterisks matching character count) |

### 7.4 Permissions

| # | Test Case | What to Verify |
|---|-----------|----------------|
| 7.4.1 | Default `~` permissions | Under agent home, all permissions default to owner-only — another agent cannot read |
| 7.4.2 | Default `world` permissions | `@read = Anything` by default — any agent can read |
| 7.4.3 | Default `world` write restrictions | `@write = Nothing` by default — no agent can write to `world` root without explicit grant |
| 7.4.4 | Read permission enforced | Set `@read` to specific agent — other agent gets permission error |
| 7.4.5 | Write permission enforced | Set `@write` to specific agent — other agent's write rejected |
| 7.4.6 | Expand permission | Set `@expand = Nothing` — cannot create sub-attributes |
| 7.4.7 | Grant permission | Only agent with `@grant` can modify permissions |
| 7.4.8 | Purge permission | Only agent with `@purge` can delete data |
| 7.4.9 | Permission cascade | Set permissions at parent — child inherits until overridden |
| 7.4.10 | Permission override at child | Set restrictive at parent, permissive at child — child's override takes effect |
| 7.4.11 | Agent list permission | `@write = [agent1, agent2]` — both can write, agent3 cannot |
| 7.4.12 | "Anything" permission | `@read = "Anything"` — all agents can read |
| 7.4.13 | "Nothing" permission | `@write = "Nothing"` — no one can write |
| 7.4.14 | Watcher permission respect | Watcher cannot modify data it doesn't have `@write` on |
| 7.4.15 | Permission + filter independence | Filter cannot bypass permissions; permissions cannot bypass filters |

---

## 8. Service & Client (`cmd/amorphd/`, `cmd/amorph/`, `cmd/amorphctl/`)

### 8.1 Service Lifecycle

| # | Test Case | What to Verify |
|---|-----------|----------------|
| 8.1.1 | Start standalone | `amorphd` starts, creates data directory, listens on socket |
| 8.1.2 | Status check | `amorphctl status` returns node name, mode=standalone, mesh=none |
| 8.1.3 | Clean shutdown | `amorphctl stop` — all data flushed to disk, socket removed |
| 8.1.4 | Restart persistence | Write data, stop, restart — data intact |
| 8.1.5 | Concurrent client connections | 10 simultaneous `amorph` sessions — all work independently |
| 8.1.6 | Client disconnect handling | Client disconnects abruptly — service continues, resources cleaned up |

### 8.2 Client REPL

| # | Test Case | What to Verify |
|---|-----------|----------------|
| 8.2.1 | Scalar display | Query `my.account.balance` → shows value with timestamp and agent |
| 8.2.2 | Record display | Query `my.account` → shows fielded key-value block |
| 8.2.3 | List display | Query `my.transactions` → shows table with column headers |
| 8.2.4 | Multi-line input | Enter a procedure definition spanning multiple lines |
| 8.2.5 | Script execution | `amorph script.mbl` runs the script and exits |
| 8.2.6 | Connection to remote node | `amorph --node host:port` connects over network |
| 8.2.7 | Authentication | `amorph --identity kalevo` authenticates as specific agent |
| 8.2.8 | Error display | Invalid MBL input produces helpful error with line/column |

### 8.3 Computer Virtual Mount

| # | Test Case | What to Verify |
|---|-----------|----------------|
| 8.3.1 | `my.computer.output()` | Writes to stdout — client receives it |
| 8.3.2 | `my.computer.input()` | Reads from stdin — blocks until input received |
| 8.3.3 | `my.computer.files.read()` | Reads a file from the local filesystem |
| 8.3.4 | `my.computer.files.write()` | Writes a file to the local filesystem |
| 8.3.5 | Computer is not persistent | `my.computer` is virtual — not replicated in mesh |
| 8.3.6 | Computer isolated per connection | Two clients on same node have separate `my.computer` contexts |

---

## 9. Mesh (`internal/mesh/`)

### 9.1 Mesh Formation

| # | Test Case | What to Verify |
|---|-----------|----------------|
| 9.1.1 | Create mesh | `amorphctl create-mesh "test-alpha"` — node becomes founder |
| 9.1.2 | Mesh name validation | Reject spaces, `@`, special chars; accept alphanumeric, hyphens, underscores |
| 9.1.3 | Join mesh | Second node joins first — both show mesh membership |
| 9.1.4 | Join discovers mesh name | Joining node learns mesh name from seed during handshake |
| 9.1.5 | Identity assignment | Joining node receives a unique CV-syllable identity |
| 9.1.6 | Blacklist filtering | Identity generator skips blacklisted syllable combinations |
| 9.1.7 | Path directory received | Joining node gets path directory snapshot from seed |
| 9.1.8 | Multi-node join | 5 nodes join sequentially — all discover each other via gossip |

### 9.2 Data Distribution

| # | Test Case | What to Verify |
|---|-----------|----------------|
| 9.2.1 | Write authority assignment | After mesh forms, paths have assigned write authorities |
| 9.2.2 | Write routing | Write to a path routed to its authority, not handled locally if not authority |
| 9.2.3 | Subscription creation | Node hosting a watcher auto-subscribes to paths it reads |
| 9.2.4 | Push updates | Authority pushes changes to subscribers within one heartbeat |
| 9.2.5 | Local read from subscription | Subscribed node serves reads locally (no network round-trip) |
| 9.2.6 | First-read subscription | Read from unsubscribed path — routes to authority, subscription established in background |
| 9.2.7 | Authority splitting | Overwhelmed authority delegates sub-path authority to another node |
| 9.2.8 | Subscription load balancing | Authority with too many subscribers delegates fan-out to a subscriber |

### 9.3 Heartbeat & Gossip

| # | Test Case | What to Verify |
|---|-----------|----------------|
| 9.3.1 | Heartbeat interval | Heartbeat fires at ~333ms intervals |
| 9.3.2 | Authority heartbeat — immediate push | Write at time T, subscriber receives by next heartbeat |
| 9.3.3 | Gossip propagation | New node announcement propagates to all nodes (may take multiple ticks) |
| 9.3.4 | Failure detection | Stop a node — peers detect via missed heartbeats |
| 9.3.5 | Authority promotion | Authority goes offline — subscriber promoted, writes resume |
| 9.3.6 | Write replay after promotion | Buffered writes that didn't reach old authority are replayed to new authority |
| 9.3.7 | Path directory update | After promotion, gossip propagates updated path-to-authority mapping |

### 9.4 Bridge

| # | Test Case | What to Verify |
|---|-----------|----------------|
| 9.4.1 | Bridge creation | `amorphctl bridge remote:port` — node gets identity in both meshes |
| 9.4.2 | Bridge as mobile agent | Bridge appears as regular mobile agent in partner mesh |
| 9.4.3 | Cross-mesh data access | Bridge reads `my.partnernet.world.*` from partner mesh |
| 9.4.4 | Cross-mesh write | Bridge writes data into partner mesh |
| 9.4.5 | Security isolation | Identities in each mesh are independent — no cross-leak |
| 9.4.6 | Bridge disconnect | `amorphctl detach partner-net` — clean departure |
| 9.4.7 | Independent bridge failure | Losing connection to one mesh doesn't affect the other |

### 9.5 Mesh Disconnection & Recovery

| # | Test Case | What to Verify |
|---|-----------|----------------|
| 9.5.1 | Graceful detach | Node announces departure, transfers authority, clears state |
| 9.5.2 | Identity preservation | After detach and rejoin, node can reclaim its identity |
| 9.5.3 | Network partition — split brain | Two halves of mesh operate independently, resolve on rejoin |
| 9.5.4 | Node crash and recovery | Node crashes, restarts, rejoins — syncs missed data from subscribers |

---

## 10. World Clock

| # | Test Case | What to Verify |
|---|-----------|----------------|
| 10.1 | Clock attributes | `world.clock.year`, `.month`, `.day`, `.hour`, `.minute`, `.second`, `.weekday`, `.weekdayname`, `.monthname`, `.yearday`, `.week`, `.quarter`, `.unix` — all correct for current UTC |
| 10.2 | Clock updates per heartbeat | Clock sub-attributes update each heartbeat tick |
| 10.3 | Clock is local per node | Clock updates do not replicate as data changes |
| 10.4 | Watcher on clock | `watch(world.clock.hour):` fires when hour changes |
| 10.5 | Clock time zones | All clock values are UTC regardless of node's local timezone |

---

## 11. Security (`internal/security/`)

| # | Test Case | What to Verify |
|---|-----------|----------------|
| 11.1 | Diffie-Hellman key exchange | Two nodes establish ephemeral secure channel |
| 11.2 | Post-quantum upgrade | Channel upgrades to post-quantum encryption |
| 11.3 | Identity generation | CV-syllable identity is pronounceable and unique |
| 11.4 | Key pair generation | Seed node generates valid key pair for new node |
| 11.5 | Mobile agent — two-factor auth | Passphrase + device secret → derived key → challenge-response |
| 11.6 | Challenge-response protocol | Node validates response against stored public key |
| 11.7 | Agent-level encryption | Private key stored encrypted in mesh — node hosting it cannot read it |
| 11.8 | Invalid credentials rejected | Wrong passphrase → authentication failure |
| 11.9 | Missing device secret rejected | Passphrase alone → authentication failure |
| 11.10 | Key rotation | Rotate device secret — re-encrypt stored data, old secret no longer works |
| 11.11 | Multiple device secrets | Two devices registered — both can authenticate |
| 11.12 | Local socket — no encryption | Unix socket connections skip encryption layer |
| 11.13 | Network socket — mandatory encryption | TCP connections require full encryption stack |
| 11.14 | Bridge authentication | Bridge authenticates independently in each mesh |

---

## 12. Purge & Compaction

| # | Test Case | What to Verify |
|---|-----------|----------------|
| 12.1 | Soft purge with TTL | Instance marked with TTL — accessible until expiration, gone after |
| 12.2 | Hard purge | Instance and value immediately removed |
| 12.3 | Cascade purge | Delete attribute → all sub-attributes and their instances purged |
| 12.4 | Compaction — authority transfer | During compaction, write authority transferred to subscriber node |
| 12.5 | Compaction — value relocation | Values relocated to fill gaps, instances updated |
| 12.6 | Compaction — file truncation | Free space consolidated at end, file truncated |
| 12.7 | Compaction — resync on rejoin | After compaction, node resyncs writes that occurred during maintenance |
| 12.8 | Historical queries during soft purge | Temporal queries still access soft-purged data before TTL expiry |
| 12.9 | Archival migration | Old instances migrated to archival node — `@older_instance_id` redirect works |

---

## 13. Integration Tests (`test/integration/`)

These tests exercise multiple subsystems together end-to-end.

### 13.1 Single-Node End-to-End

| # | Test Case | What to Verify |
|---|-----------|----------------|
| 13.1.1 | Full lifecycle | Start service → connect client → create records → define procedures → set watchers → query data → temporal query historical values → shut down → restart → verify all persisted |
| 13.1.2 | Watcher pipeline | Watcher A writes to path → Watcher B fires → Watcher C fires — full chain executes across ticks |
| 13.1.3 | Permission enforcement E2E | Create two agents, set permissions, verify agent A can read/write, agent B is blocked |
| 13.1.4 | Stamp + filter E2E | Set stamps, activate filter, verify filtered view shows Redacted, unfiltered shows real data |
| 13.1.5 | Error propagation E2E | Outer run encounters network error → Unknown propagates → all staged writes roll back |
| 13.1.6 | Cross-zone watcher | Watcher on zone A writes to zone B — both commit atomically at heartbeat |

### 13.2 Multi-Node End-to-End

| # | Test Case | What to Verify |
|---|-----------|----------------|
| 13.2.1 | Two-node write/read | Write on node A, read on node B — data available within one heartbeat |
| 13.2.2 | Distributed watcher pipeline | Watcher on node A, watcher on node B that depends on A's output — pipeline works across nodes |
| 13.2.3 | Agent mobility | Agent authenticates on node A, writes data, reconnects to node B, data accessible |
| 13.2.4 | Authority failover | Kill authority node — subscriber promotes, writes resume, no data lost |
| 13.2.5 | Three-node consensus | Three nodes, kill one — remaining two continue operating |
| 13.2.6 | Bridge data flow | Bridge node syncs data between two independent meshes |
| 13.2.7 | Clock synchronization | Three nodes — `world.clock` values agree within heartbeat tolerance |

### 13.3 Stress & Chaos

| # | Test Case | What to Verify |
|---|-----------|----------------|
| 13.3.1 | High write throughput | 10,000 writes/second sustained for 60 seconds — no data loss, all queryable |
| 13.3.2 | Many watchers | 1,000 active watchers — all fire within heartbeat when triggered |
| 13.3.3 | Deep hierarchy | Create 100-level deep path, read/write at leaf — performance acceptable |
| 13.3.4 | Wide hierarchy | Create 10,000 children under one parent — `..count` and enumeration work |
| 13.3.5 | Concurrent readers and writers | 50 readers + 50 writers on same paths — no corruption, no deadlocks |
| 13.3.6 | Node kill during heartbeat | Kill node mid-heartbeat — remaining nodes detect and recover |
| 13.3.7 | Network partition simulation | Split a 4-node mesh into two halves — both continue, reconcile on heal |
| 13.3.8 | Compaction under load | Run compaction while writes are ongoing — no data loss or corruption |
| 13.3.9 | Memory pressure | Fill commit buffers to near capacity — system reports errors cleanly, no OOM panic |
| 13.3.10 | Long-running watcher | Watcher that takes >333ms to execute — system handles gracefully (delays next tick, reports latency) |

---

## 14. MBL Program Tests (`test/mbl_programs/`)

These are full MBL programs that test the language end-to-end through the interpreter. Write each as an `.mbl` file with expected output, and a Go test that runs it and compares output.

### 14.1 Example Programs

| # | Program | Expected Behavior |
|---|---------|-------------------|
| 14.1.1 | Counter procedure | Define counter with persistent `.count`, call 5 times, verify count = 5 |
| 14.1.2 | Balance transfer | Set two account balances, transfer between them, verify both updated |
| 14.1.3 | Inventory management | Add items to list, remove by value, verify list state |
| 14.1.4 | Temporal audit trail | Write 5 values to same path over time, query history at various points |
| 14.1.5 | Watcher-driven automation | Set up watcher, trigger it by changing data, verify side effects |
| 14.1.6 | Error handling | Procedure that divides by zero inside catch, handles Unknown gracefully |
| 14.1.7 | Record operations | Create records, set fields, read projections, verify correct subset returned |
| 14.1.8 | Nested procedures | Procedure that calls another procedure that calls a third — all return correctly |
| 14.1.9 | Type coercion chain | `"100" + 50 + $25.00` — verify coercion chain produces correct result |
| 14.1.10 | Permission denied handling | Agent tries to write to restricted path — gets Unknown, catches it, writes audit log |

---

## Test Infrastructure Notes

### Test Helpers Needed

1. **`testutil.StartNode(t)`** — starts a fresh amorphd instance with temp data directory, returns a connected client and cleanup function
2. **`testutil.StartMesh(t, n int)`** — starts n nodes and connects them into a mesh, returns slice of clients
3. **`testutil.WaitForHeartbeat(t)`** — blocks until at least one heartbeat cycle completes
4. **`testutil.CreateAgent(t, client, name)`** — creates a mobile agent with the given name, returns auth credentials
5. **`testutil.RunMBL(t, client, program string)`** — sends an MBL program to the client for execution, returns output and error
6. **`testutil.AssertEventually(t, timeout, fn func() bool, msg)`** — polls fn until true or timeout (for async mesh operations)

### Coverage Goals

- Statement coverage: ≥ 85% across all packages
- Branch coverage: ≥ 75% across all packages
- Every error path should have at least one test
- Every type × operation combination should be tested
- Every permission × operation combination should be tested

### Performance Baselines

Establish baselines in the stress tests and track regressions:
- Single-node write latency (p50, p95, p99)
- Single-node read latency
- Mesh replication latency (write on A → readable on B)
- Watcher trigger latency (change → watcher fires)
- Temporal query latency (as-of query over 1M instances)
