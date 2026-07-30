# MBL Syntax Changes — Development Plan

Two syntax changes from the updated `amorphdb_design.md`:

1. **`?=` operator reduction** — remove `?!=`, `?<`, `?>`, `?<=`, `?>=` from
   the lexer, parser, and interpreter. Keep only `?=`. Add `x ?= unknown` and
   `x ?= unknown("reason")` matching semantics.

2. **Type-first definition syntax** — `procedure name(args):` and
   `watch name(paths):` as primary definition forms, binding the name in the
   current persistent scope. The existing path-first form (`my.path: procedure(...)`)
   remains supported and equivalent.

Each step is one Claude Code session. Complete the step, run tests, stop.

**Rules:** Read `CLAUDE.md` before every step. The spec
(`docs/amorphdb_design.md`) is the source of truth. Write tests for each
feature. No TODOs in tests.

---

## Phase 1: `?=` Operator Reduction

### Step 1.1: Remove `?!=`, `?<`, `?>`, `?<=`, `?>=` from Lexer

**Status:** DONE (completed: 2026-04-26) — Verified the lexer source has
no separate token types for the removed operators (only `?=` → EQUAL is
defined; `?` followed by anything else falls through to ILLEGAL). Updated
`testplan_section3_test.go::TestSection3_1_15_Operators_TypeSafe` to
reflect the new spec: assert `?=` → EQUAL and that `?` followed by a
non-`=` produces an ILLEGAL token. `go test ./internal/mbl/lexer/...`
passes.

Remove the six type-safe comparison tokens from `internal/mbl/lexer/tokens.go`
and `internal/mbl/lexer/lexer.go`. Keep only `?=` (which should already exist
as a token type — verify its name).

If any existing tests reference the removed operators, update them:
- Tests that use `?!=` should be rewritten to use `not (x ?= y)`
- Tests that use `?<`, `?>`, etc. should be rewritten to use standard
  operators with an explicit Unknown check first

Run `go test ./internal/mbl/lexer/...` — no failures.

Files: `internal/mbl/lexer/tokens.go`, `internal/mbl/lexer/lexer.go`,
`internal/mbl/lexer/*_test.go`

---

### Step 1.2: Remove Type-Safe Comparisons from Parser

**Status:** DONE (completed: 2026-04-26) — Verified that the parser source
has no references to the removed operators. `?=` is parsed via
`lexer.EQUAL` into a generic `BinaryExpression` (no special AST node);
the standard `<`, `>`, `<=`, `>=` are parsed via `lexer.LT/GT/LTE/GTE`.
No AST node types or precedence entries exist for `?!=`, `?<`, `?>`,
`?<=`, `?>=`, so nothing needed removal. The only test referencing
"TypeSafe" in this package is `TestSection4_1_15_TypeSafeComparison`,
which already verifies only `?= 5` (kept operator) and passes. Pre-
existing failures in the parser package (TestWatchAppendParsing,
TestMultiPathWatchParsing, TestSameLineDefinitions,
TestSection4_2_23_BreakStatement) are unrelated to this step and
remain at the same baseline count after this step.

Remove parsing of `?!=`, `?<`, `?>`, `?<=`, `?>=` from
`internal/mbl/parser/parser.go`. If the parser has AST node types for these
operators, remove them. Keep `?=` parsing intact.

Update any parser tests that reference the removed operators.

Run `go test ./internal/mbl/parser/...` — no failures.

Files: `internal/mbl/parser/parser.go`, `internal/mbl/parser/ast.go`,
`internal/mbl/parser/*_test.go`

---

### Step 1.3: Remove Type-Safe Comparisons from Interpreter

**Status:** DONE (completed: 2026-04-26) — Verified the interpreter source
has no evaluation logic for the removed operators. `evalBinaryExpression`
(interpreter.go:985) handles only `?=`, `>`, `<`, `>=`, `<=` for
comparison. No interpreter test references `?!=`, `?<`, `?>`, `?<=`, or
`?>=` (the only `?` matches in test files are XML literals like
`<?xml ... ?>`). Since the lexer (Step 1.1) and parser (Step 1.2) already
prevent these tokens from reaching the interpreter, no source change was
needed. Pre-existing interpreter failures (TestProjectionExpressions,
TestJsonHelpersIntegration, TestSection5_1_ExpressionEvaluation/5.1.6,
5.1.9, TestSection5_2_VariableScope/5.2.5) are unrelated to this step
and remain at the same baseline count. Note: 5.1.6 expects
`unknown("offline") ?= 5 → false`, which is Step 1.4 territory
(`?= unknown` matching semantics).

Remove evaluation of `?!=`, `?<`, `?>`, `?<=`, `?>=` from
`internal/mbl/interpreter/interpreter.go`. Keep `?=` evaluation.

Update any interpreter tests that reference the removed operators. Rewrite
them to use the spec-compliant alternatives (`not (x ?= y)` for inequality,
standard operators with Unknown checks for ordering).

Run `go test ./internal/mbl/interpreter/...` — no failures from this change.

Files: `internal/mbl/interpreter/interpreter.go`,
`internal/mbl/interpreter/*_test.go`

---

### Step 1.4: Implement `x ?= unknown` and `x ?= unknown("reason")` Matching

**Status:** DONE (completed: 2026-04-26) — Special-cased `?=` in
`evalBinaryExpression` (interpreter.go) to short-circuit BEFORE the
generic Unknown-propagation block, and added the
`evalDefiniteEquality(left, right)` helper implementing the matching
semantics: bare `unknown` (RHS) matches any Unknown LHS; `unknown("r")`
(RHS) matches only when LHS is Unknown with the same reason; LHS
Unknown with non-Unknown RHS returns false (no propagation); otherwise
standard value equality. Added `TestDefiniteEqualityUnknownMatching`
in `interpreter_test.go` (12 sub-cases, all pass), covering each
matrix in the dev plan: bare/reasoned RHS × {reasoned, bare,
non-Unknown} LHS, plus normal-equality regressions for Number, Text,
and a mismatched-number case.

Cross-package fix required: `parseUnknownLiteral` in
`internal/mbl/parser/parser.go` over-advanced one token after
`unknown("reason")`, so a Pratt-style infix following the literal
(e.g. `?=`) was silently dropped. This made test 5.1.6
(`unknown("offline") ?= 5 → false`) and 5.1.9
(`unknown("offline") + 5 → Unknown`) impossible to satisfy from
inside the interpreter alone. Removed the extra `p.nextToken()` and
documented the Pratt protocol so the prefix function leaves
currentToken at `)`. The dev plan note in Step 1.3 explicitly assigned
5.1.6 to Step 1.4, so the parser fix is in scope.

Variable-storage caveat: assigning an Unknown to a local variable
(`x = unknown("timeout")`) currently returns the Unknown without
storing — so reading `x` back yields `Nothing`, not the Unknown. This
is a pre-existing bug in `evalAssignment` (interpreter.go:~1057,
unrelated to `?=`). Tests therefore use inline `unknown(...)` literals
on the LHS, which exercise the same matching paths.

Test suite status after this step:
- `go test ./internal/mbl/interpreter/...` — pre-existing failures
  reduced from 5 to 3 (5.1.6 and 5.1.9 fixed by this step). Remaining:
  TestProjectionExpressions/projection_from_path,
  TestJsonHelpersIntegration/parse_json_via_MBL_evaluation,
  TestSection5_2_VariableScope/5.2.5_Procedure_parameter_passing — all
  unrelated to `?=` matching.
- `go test ./internal/mbl/parser/...` and `./internal/mbl/lexer/...` —
  baseline unchanged (parser failures all about watch/definition/break
  parsing, no overlap with `parseUnknownLiteral`).

Followup (not in this step's scope):
1. Parser collision: bare `unknown` literal is parsed with
   `Reason: "unknown"` (sentinel), so a user-written `unknown("unknown")`
   is currently indistinguishable from bare `unknown` and matches any
   Unknown. The interpreter follows the parser's convention here.
2. Local-variable assignment of Unknown should store the value rather
   than short-circuiting; this would let the dev plan's variable-based
   examples work directly.
3. `is_unknown(x)` builtin is documented in the spec but not
   implemented; the dev plan's "verify equivalence with `x ?= unknown`"
   test was therefore omitted.

Extend the `?=` evaluation in `internal/mbl/interpreter/interpreter.go`:

- `x ?= unknown` — returns `true` if `x` is any Unknown value, `false`
  otherwise. This makes `x ?= unknown` equivalent to `is_unknown(x)`.
- `x ?= unknown("reason")` — returns `true` only if `x` is Unknown AND
  its reason matches the given string. Returns `false` if `x` is not
  Unknown or if the reason differs.
- `x ?= unknown` (bare, no reason) matches any Unknown regardless of reason.

The right operand `unknown` and `unknown("reason")` are already valid
expressions in MBL (they produce Unknown values). The interpreter needs
to detect when the right side of `?=` is an Unknown value and switch to
"matching" semantics instead of "equality" semantics.

Write tests:
- `x ?= unknown` where x is Unknown (any reason) → true
- `x ?= unknown` where x is a normal value → false
- `x ?= unknown("timeout")` where x is `unknown("timeout")` → true
- `x ?= unknown("timeout")` where x is `unknown("dns_failure")` → false
- `x ?= unknown("timeout")` where x is bare `unknown` → false (reason doesn't match)
- `x ?= unknown("timeout")` where x is a normal value → false
- `x ?= unknown` where x is `unknown("timeout")` → true (bare matches any)
- Verify `is_unknown(x)` and `x ?= unknown` produce identical results

Run `go test ./internal/mbl/interpreter/...` — all new tests pass.

Files: `internal/mbl/interpreter/interpreter.go`,
`internal/mbl/interpreter/*_test.go`

---

### Step 1.5: Update MBL Script Tests

**Status:** DONE (completed: 2026-04-26) — Searched all `.mbl` files
under `tests/unit/`, `tests/phase3/`, and `test/` for the removed
operators (`?!=`, `?<`, `?>`, `?<=`, `?>=`). No occurrences found, so
no rewrites were needed. (`test/` has no `.mbl` files; `tests/unit/`
and `tests/phase3/` use only the kept `?=` form.) Added new MBL
script test at `tests/unit/definite_equality_unknown_test.mbl`
demonstrating the `?= unknown` and `?= unknown("reason")` matching
forms with 11 expression cases covering: bare-`unknown` RHS vs
{reasoned-Unknown LHS, bare-Unknown LHS, Number LHS, Text LHS};
reasoned-`unknown(...)` RHS vs {same-reason Unknown, different-reason
Unknown}; LHS-Unknown vs Number RHS (no propagation); and standard
equality on stored Number and Text values. The script uses inline
`unknown(...)` literals on the LHS where the left value must be
Unknown — this works around the pre-existing
`evalAssignment`-short-circuits-on-Unknown bug noted in Step 1.4
followup #2 (which would otherwise cause stored-variable forms like
`x = unknown("timeout"); result = x ?= unknown` to read back
Nothing). Stored variable assignments are still used for normal
(non-Unknown) values, which round-trip correctly. The file documents
this constraint inline so the script can be revisited once the
`evalAssignment` bug is addressed.

Test suite status after this step: failure set is identical to the
Step 1.4 baseline (3 interpreter, 6 parser, plus pre-existing failures
in cmd, internal/interpreter, internal/security, internal/storage,
internal/zone_deprecated, test/integration, tests/integration that
predate Phase 1). No regressions.

Search `tests/unit/`, `tests/phase3/`, and `test/` for any `.mbl` test
scripts that use the removed operators (`?!=`, `?<`, `?>`, `?<=`, `?>=`).
Rewrite them to use spec-compliant alternatives.

Also add MBL script tests for the new `?= unknown` matching:

```mbl
# Test: x ?= unknown matching
x = unknown("timeout")
result1 = x ?= unknown              # true (any Unknown)
result2 = x ?= unknown("timeout")   # true (reason matches)
result3 = x ?= unknown("dns")       # false (reason differs)
result4 = x ?= 5                    # false (x is Unknown, not 5)

y = 42
result5 = y ?= unknown              # false (y is not Unknown)
result6 = y ?= 42                   # true (normal equality)
```

Run `go test ./...` — verify no regressions across the full suite.

Files: `tests/unit/*.mbl`, `tests/phase3/*.mbl`, `test/**/*.mbl`

---

## Phase 2: Type-First Definition Syntax

### Step 2.1: Parse Type-First Procedure Definitions

**Status:** DONE (completed: 2026-04-26) — Added a `lexer.PROCEDURE`
case to `parseStatement` in `internal/mbl/parser/parser.go`. When the
`procedure` keyword appears at statement position followed by an
identifier, the new helper `parseTypeFirstProcedureStatement` parses
`procedure name(args): body` and returns a `*DefinitionStatement`
whose `Name` is a single-segment `*PathExpression` and whose `Value`
is a `*ProcedureExpression` — i.e., the same AST shape as the
equivalent path-first form `name: procedure(args): body`. This
deliberately avoids introducing a new AST node so that Step 2.2
(evaluation) can flow through the existing `DefinitionStatement`
evaluator. The `procedure(args): body` anonymous form (no name
between `procedure` and `(`) falls through to
`parseExpressionStatement`, which dispatches the existing
`PROCEDURE`-prefix function (`parseProcedureExpression`); anonymous
procedures in expression position (e.g. `map(list, procedure(x): ...)`)
and in `=` assignments are unaffected.

Body parsing mirrors `parseProcedureExpression`: same-line single
statement, or multi-line block via NEWLINE/INDENT. Parameter parsing
is shared in spirit with `parseProcedureExpression` (validates
`IDENT` for each parameter via `expectPeek`), with empty `()` allowed.

Tests: added `TestTypeFirstProcedureDefinition` in
`internal/mbl/parser/parser_test.go` with six sub-cases —
single-line type-first, multi-line type-first with two params,
zero-param type-first, path-first regression, anonymous-`=`-assignment
regression, and anonymous-as-call-argument regression. All six pass.

Test suite status after this step:
- `go test ./internal/mbl/parser/...` — pre-existing failure set
  unchanged (TestWatchAppendParsing, TestWatchAppendWithFilterParsing,
  TestMultiPathWatchParsing×4, TestMultiPathWatchPathExpressions,
  TestSameLineDefinitions×3, TestSection4_2_23_BreakStatement). No
  new failures.
- `go test ./internal/mbl/lexer/...` — pass.
- `go test ./internal/mbl/interpreter/...` — pre-existing failure set
  unchanged (TestProjectionExpressions/projection_from_path,
  TestJsonHelpersIntegration/parse_json_via_MBL_evaluation,
  TestSection5_2_VariableScope/5.2.5_Procedure_parameter_passing).
  No new failures.

Notes for downstream steps:
1. The legacy column-1 `name(args): body` form still produces a
   `*ProcedureStatement` (different AST). Step 2.2 (evaluation) only
   needs to handle `*DefinitionStatement`-with-`*ProcedureExpression`
   for type-first; the legacy form already evaluates via
   `evalProcedureStatement`.
2. The `Token` field on the `*ProcedureExpression` produced by
   `parseTypeFirstProcedureStatement` is the `procedure` keyword
   token, while the path-first form's `*ProcedureExpression` `Token`
   is the same `procedure` keyword token (because the value side is
   parsed via `parseProcedureExpression`). So both forms produce
   structurally identical sub-ASTs for the value.

Add parsing support for `procedure name(args):` as a statement form in
`internal/mbl/parser/parser.go`.

When the parser encounters the `procedure` keyword at statement position
followed by an identifier and `(`, parse it as a procedure definition where
the name binds in the current scope. The resulting AST node should be the
same as the path-first form — the name is just a simple identifier instead
of a dotted path.

**Important:** The existing path-first form (`my.path: procedure(args):`)
must continue to work unchanged. Both forms produce the same AST structure.

**Anonymous procedures** in expression position (`= procedure(x): ...` or
passed as arguments) already work — do not break them.

Write parser tests:
- `procedure double(x): return x * 2` → parses successfully
- `procedure calculate_tax(income, rate):\n    return income * rate` → parses
  with block body
- `my.functions.double: procedure(x): return x * 2` → still parses (path-first)
- `my.double = procedure(x): return x * 2` → still parses (anonymous assignment)
- `map(list, procedure(x): return x * 2)` → still parses (anonymous in expression)

Run `go test ./internal/mbl/parser/...` — no failures.

Files: `internal/mbl/parser/parser.go`, `internal/mbl/parser/ast.go`,
`internal/mbl/parser/*_test.go`

---

### Step 2.2: Evaluate Type-First Procedure Definitions

**Status:** DONE (2026-04-26) — type-first and path-first procedure
definitions both evaluate; 9/9 new sub-tests pass; no new regressions
in interpreter package (3 baseline failures unchanged).

**Implementation summary:**

`internal/mbl/interpreter/interpreter.go`:
- Added `procedures map[string]*Procedure` field to the `Interpreter`
  struct (initialized in `New` and `NewWithCoordinator`). Procedures
  cannot currently flow through `types.CreateValue` → storage, so they
  live in this in-process registry keyed by the joined path.
- Added `case *parser.DefinitionStatement` to `evalStatement`, dispatching
  to a new `evalDefinitionStatement`. Added `case *parser.ProcedureExpression`
  to `evalExpression`, dispatching to a new `evalProcedureExpression`.
- New `evalProcedureExpression` returns a `*Procedure` that captures
  `i.scope` as its closure.
- New `evalDefinitionStatement`: type-asserts `Name` as `*PathExpression`,
  evals `Value`, and routes `*Procedure` values through a shared binding
  helper. Non-procedure values are routed through `evalAssignment` (so
  existing storage-backed path semantics still apply).
- New `bindProcedureAtPath`: stores the procedure in the registry under
  the joined path, AND in `i.scope` for single-segment names so the
  existing `evalCallExpression` lookup keeps working.
- `evalAssignment` now short-circuits when the right-hand side resolves
  to a `*Procedure`, calling `bindProcedureAtPath` instead of attempting
  storage conversion.
- `evalCallExpression` now checks `i.procedures[functionName]` before
  the existing scope lookup, so deep-path procedure calls work.

**Required tests (all passing):**
- `procedure double(x): return x * 2` then `double(21)` → 42
- Procedure accessible at expected path (registry + scope)
- Path-first form still works (`my.functions.triple: procedure(x): ...`)
- Anonymous assignment to deep path still works

**Additional coverage added:**
- Multi-line block body (`procedure calculate(income, rate): ...`)
- Standalone `*Procedure` expression evaluates to a `*Procedure` value
- Zero-parameter procedures
- Calls to unbound deep paths return `Unknown`
- Composing two type-first procedures at a call site

**Out-of-scope discovery (filed under future parser cleanup):**
A pre-existing parser bug splits `f(x)` inside any procedure body into
`f` and `(x)` (it affects type-first, path-first, and legacy column-1
forms equally — Step 2.2 did not introduce it). Composing procedures
at the call site (`g(f(x))`) works fine. Per Rule 1, this is left for
a future parser-fix step.

Files modified: `internal/mbl/interpreter/interpreter.go`,
`internal/mbl/interpreter/interpreter_test.go`

---

### Step 2.3: Parse Type-First Watcher Definitions

**Status:** DONE (completed: 2026-04-26) — Verified that the type-first
watcher form (`watch name(paths):` and `watch name append(path) as
binding:`) already parses through the alternate-style branch of
`parseWatchStatement` (parser.go:967–984). No source change was needed:
when `parseStatement` dispatches `lexer.WATCH`, the alternate branch
detects an identifier (or `my`/`world`-prefixed path) following `watch`
and parses it as the watcher name, then falls through to the existing
shared parsing logic that handles parentheses, optional `append(...) as
<binding>`, predicate filters, and the indented body. The resulting
AST is a top-level `*WatchStatement` — same shape as the alternate form
that already existed.

**New tests (all passing):** Added `TestTypeFirstWatcherDefinition` in
`internal/mbl/parser/parser_test.go` with 8 sub-cases:

1. Single-path type-first watcher.
2. Multi-path type-first watcher (two paths, comma-separated).
3. Type-first watcher with mixed `my`/`world` paths (asserts
   `world.clock.minute` parses correctly inside the path list).
4. Type-first append watcher with `as <binding>`.
5. Type-first append watcher with predicate filter
   (`append(my.orders[priority ?= "urgent"]) as urgent`) — verifies
   `Filters` is populated and `Paths` strips the bracket filter.
6. Type-first watcher accepting a multi-segment dotted name in the name
   position (`watch my.monitors.balance_check(...)`) — covers the
   `MY`/`WORLD` branch in the alternate-style detector.
7. Path-first form regression — accepts either `*WatchStatement` or
   `*DefinitionStatement`-wrapping-`*WatchExpression` (the path-first
   form currently routes through `parseDefinitionStatement` via the
   `isDefinition()` check, producing a DefinitionStatement; this is
   pre-existing behavior unrelated to Step 2.3 and is the cause of the
   baseline `TestWatchAppendParsing` / `TestWatchAppendWithFilterParsing`
   failures noted in earlier steps).
8. Anonymous-watch-expression assignment regression
   (`my.handler = watch(my.x): output("x")`).

**Edge case discovered (pre-existing, out of Step 2.3 scope):**
`parseWatchStatement` requires the body to be on its own indented line
(it expects `INDENT` after the `:`), so a single-line form like
`watch ping(my.x): my.y = 1` fails to parse. The same limitation
already affects the path-first form `name: watch(...): body`, since
both routes share the body-parsing tail. Fixing this would be a
parser-cleanup step parallel to the procedure-body parser bug noted in
Step 2.2's "Out-of-scope discovery." Per Rule 1, leaving this for
future work.

**Test suite status after this step:**
- `go test ./internal/mbl/parser/...` — pre-existing failure set
  unchanged (TestWatchAppendParsing, TestWatchAppendWithFilterParsing,
  TestMultiPathWatchParsing×4, TestMultiPathWatchPathExpressions,
  TestSameLineDefinitions×3, TestSection4_2_23_BreakStatement). No new
  failures.
- `go test ./internal/mbl/lexer/...` — pass.
- `go test ./internal/mbl/interpreter/...` — pre-existing failure set
  unchanged (3 baseline failures: TestProjectionExpressions/projection_from_path,
  TestJsonHelpersIntegration/parse_json_via_MBL_evaluation,
  TestSection5_2_VariableScope/5.2.5_Procedure_parameter_passing).

Files modified: `internal/mbl/parser/parser_test.go` (tests only — no
source change required).

Add parsing support for `watch name(paths):` as a statement form. This may
already partially exist as the "alternate watcher syntax" from earlier work.
Verify it matches the spec exactly:

```
watch balance_check(my.account.balance, my.account.limit):
    body
```

The name is a simple identifier (not a dotted path). The paths in parentheses
are the watched paths. The name binds in the current persistent scope.

**Append form:**

```
watch new_orders append(my.orders) as orders:
    body
```

Verify both value-change and append forms work with type-first syntax.

Write parser tests for both forms.

Run `go test ./internal/mbl/parser/...` — no failures.

Files: `internal/mbl/parser/parser.go`, `internal/mbl/parser/*_test.go`

---

### Step 2.4: Evaluate Type-First Watcher Definitions

**Status:** DONE (2026-04-26) — Added `Watcher` struct with `Fire` method,
`WatcherRegistrar` interface for dependency inversion (avoids circular import
with `internal/watcher`), `evalWatchStatement` and `evalWatchExpression`
handlers, `bindWatcherAtPath` for scope binding, and `bodyToMBLCode`
serialization helper. Path-first form (`name: watch(...)`) routes through
`evalDefinitionStatement`/`evalAssignment` via `*Watcher` value detection.
Added `TestTypeFirstWatcherEvaluation` with 11 sub-tests covering type-first
single/multi-path, path-first, append form, my-scope binding, registrar
forwarding, and error cases — all passing. Baseline test set unchanged.

Add evaluation support for the type-first watcher AST node.

When evaluating `watch name(paths): body`:
- Determine the current persistent scope
- Create the watcher value
- Bind it at `<current_scope>.name`
- Register the watcher with the watcher engine for the specified paths

Write interpreter tests:
- `watch test_watcher(my.data.x): ...` — verify watcher created and fires
- Verify path-first form still works
- Verify append watcher type-first form works

Run `go test ./internal/mbl/interpreter/...` — no failures.

Files: `internal/mbl/interpreter/interpreter.go`,
`internal/mbl/interpreter/*_test.go`

---

### Step 2.5: Scope Resolution for Type-First Definitions

**Status:** DONE (2026-04-26) — Added `currentScopePathRaw` field tracking
the unresolved persistent-scope path (e.g. `["my","utilities"]`)
alongside the existing resolved form. Updated `evalScopeStatement` to
populate both. Added `bindingPathForProcedure` helper and routed
single-segment procedure paths in `evalDefinitionStatement` through it
so type-first procedures bind at `<scope>.<name>` (or `my.<name>` at
REPL). Captured `DefScopePathRaw` and `DefScopePath` on `*Procedure`
at definition time and made `Procedure.Call` save/restore both around
body execution so nested type-first definitions bind in the
procedure's defining scope, not the caller's. Watchers already used
`bindingPathFor` (resolved form) so no impl change was needed for
watcher binding. Updated one existing assertion (Step 2.2 test
checked `procedures["greet"]`; now checks `procedures["my.greet"]`).
Added `TestTypeFirstScopeResolution` with 8 sub-tests covering all
four spec contexts plus call-boundary scope restoration and
scope-clear behavior — all passing. Baseline test set unchanged.

Verify that the "current persistent scope" resolution works correctly for
type-first definitions in different contexts:

- At file root loaded into `my` → `procedure double(x):` creates `my.double`
- At file root loaded into `my.utilities` → creates `my.utilities.double`
- Inside a procedure body → creates in the procedure's persistent scope
- In REPL → creates in `my` (the agent's home)

If scope resolution is not already tracked by the interpreter, implement it.
The interpreter needs to know what scope a file was loaded into (this may
already exist for the `my` keyword resolution).

Write tests for each context.

Run `go test ./internal/mbl/interpreter/...` — no failures.

Files: `internal/mbl/interpreter/interpreter.go`,
`internal/mbl/interpreter/*_test.go`

---

### Step 2.6: Update MBL Script Tests

**Status:** DONE (completed: 2026-04-26) — Audited `tests/unit/`,
`tests/phase3/`, and `test/` for `.mbl` scripts using procedure or
watcher definitions: only three files in `tests/phase3/` mention
the words "procedure" / "watch" and only in comments (not as
definitions); `test/` has no `.mbl` files. No existing scripts
needed updates.

Added two new MBL script tests demonstrating type-first syntax:

1. `tests/unit/type_first_procedure_test.mbl` — exercises the
   `procedure name(args):` form across single-line, multi-line block,
   zero-parameter, and string-concatenation cases; composes two
   type-first procedures at the call site (`plus_one(square(4))`);
   and shows the path-first (`my.functions.triple: procedure(x):`)
   and anonymous-assignment (`my.handlers.negate = procedure(n):`)
   forms remain equivalent. Constraints documented inline: (a) a
   pre-existing parser bug splits `f(x)` inside any procedure body
   into `f` and `(x)`, so calls inside bodies are avoided; (b) MBL
   strings open/close on matched quote counts (no `\"` escape), so
   output labels avoid embedded quotes.

2. `tests/unit/type_first_watcher_test.mbl` — defines watchers in
   single-path, multi-path, append-with-binding, and
   append-with-predicate-filter forms via type-first syntax, plus
   path-first (`my.handlers.legacy: watch(...)`) and
   anonymous-assignment (`my.handlers.assigned = watch(...)`)
   regressions. The script does not exercise firing — the Go unit
   tests in `TestTypeFirstWatcherEvaluation` cover that path; this
   script asserts only that each definition evaluates and binds
   without error.

Added end-to-end Go tests in
`internal/mbl/interpreter/type_first_scripts_test.go` that load each
`.mbl` file, run it through the interpreter, and assert: (a) every
procedure in the procedure script returns its expected value
(`double(21)=42`, `calculate_tax(100,0.25)=25`, `ping()="pong"`,
`greet("World")="hello, World"`, `plus_one(square(4))=17`,
`my.functions.triple(7)=21`, `my.handlers.negate(5)=-5`); (b) each
watcher in the watcher script binds at the expected resolved storage
path (`world.agent.1001.{balance_check, multi_check, new_orders,
urgent_orders, handlers.legacy, handlers.assigned}`). Both Go tests
pass (`TestTypeFirstProcedureScript`, `TestTypeFirstWatcherScript`).

Test suite status: `go test ./...` failure set matches the
pre-existing baseline established in earlier steps — 3 interpreter
failures (TestProjectionExpressions/projection_from_path,
TestJsonHelpersIntegration/parse_json_via_MBL_evaluation,
TestSection5_2_VariableScope/5.2.5_Procedure_parameter_passing), 6
parser failures (TestWatchAppendParsing, TestWatchAppendWithFilterParsing,
TestMultiPathWatchParsing, TestMultiPathWatchPathExpressions,
TestSameLineDefinitions, TestSection4_2_23_BreakStatement), and the
unrelated baseline failures in cmd/, internal/interpreter,
internal/security, internal/storage, internal/zone_deprecated,
internal/mesh (build), test/integration, tests/integration (build) —
all of which predate Step 2.6. No new failures.

Search all `.mbl` test scripts for procedure and watcher definitions. Do NOT
change them to type-first — both forms are valid. Instead, ADD new test
scripts that use type-first syntax to verify it works end-to-end:

```mbl
# Type-first procedure
procedure greet(name):
    return "Hello, " & name

result = greet("World")
# assert result = "Hello, World"
```

```mbl
# Type-first watcher
watch counter(my.test.value):
    my.test.count = my.test.count + 1
```

Run `go test ./...` — verify no regressions.

Files: `tests/unit/*.mbl`, `tests/phase3/*.mbl`

---

## Phase 3: Cleanup

### Step 3.1: Final Verification and Cleanup

**Status:** DONE (completed: 2026-04-26) — Verified the full test suite
failure set is identical to the baseline established before Phase 1
began. Searched the codebase for the removed operators (`?!=`, `?<`,
`?>`, `?<=`, `?>=`) across `internal/`, `cmd/`, `tests/`, `test/` and
confirmed no live code references them. The only matches in source are
intentional negative-test cases:
`internal/mbl/lexer/testplan_section3_test.go:386–390`, where Step 1.1
asserts `?=` → `EQUAL` and `?<`/`?>`/`?!` → `ILLEGAL` (these tests
exist precisely to verify the operators are gone — they must stay).
False-positive matches in `internal/mbl/interpreter/interpreter.go`
and `interpreter_test.go` are XML preamble literals (`<?xml ...?>`),
not the removed operators.

Searched for orphaned identifiers (`TYPE_SAFE`, `TypeSafe`, `TYPE_LT`,
`TYPE_GT`, `TYPE_LTE`, `TYPE_GTE`, `TypeSafeNotEqual`, etc.) — none
defined as token constants or AST nodes. The lexer's only token
constants in this family are `EQUAL // ?=` (kept), `NOT_EQUAL // !=`
(standard `!=`, never type-safe), and the standard ordering tokens
`GT`, `LT`, `GTE`, `LTE` (always non-type-safe). Steps 1.1 and 1.2
already confirmed during their execution that no separate token types
or AST nodes ever existed for the removed operators — `?` followed by
anything other than `=` always fell through to `ILLEGAL` at the lexer,
so there was nothing to remove.

Two test functions retain "TypeSafe" in their names
(`TestSection3_1_15_Operators_TypeSafe` in lexer,
`TestSection4_1_15_TypeSafeComparison` in parser). The names are
historical section labels from `docs/AmorphDB_Test_Plan.md` (kept
stable for cross-reference); their bodies test only the kept `?=`
operator and the lexer test additionally asserts the removed forms
produce `ILLEGAL` tokens. Renaming is out of Step 3.1's scope per
Standing Rule 4.

Documentation references to the removed operators exist in
historical/archived spec snapshots
(`docs/amorphdb_design_until20260402.md`,
`docs/amorphdb_design_until20260426.md`,
`docs/amorphdb_design_preCurrencyFix.md`,
`docs/amorphdb_design_prePWABiolerplate.md`,
`docs/AmorphDB_Test_Plan.md`,
`docs/CLAUDE.md`,
`amorphdb_spec_vs_implementation_report.md` at repo root) and in this
dev plan itself. These are documentation files outside the scope of
"dead code, unused token constants, or orphaned AST node types" — per
Standing Rule 1, they are not modified by this step. The current
`docs/CLAUDE.md` correctly describes the change ("single operator
replacing old `?=`/`?!=`/`?<`/`?>`/`?<=`/`?>=` family"); the
historical snapshots are intentional time-stamped archives.

Test suite status at completion: `go test ./...` failure set matches
the pre-existing baseline — 3 interpreter failures
(TestProjectionExpressions/projection_from_path,
TestJsonHelpersIntegration/parse_json_via_MBL_evaluation,
TestSection5_2_VariableScope/5.2.5_Procedure_parameter_passing), 6
parser failures (TestWatchAppendParsing,
TestWatchAppendWithFilterParsing, TestMultiPathWatchParsing,
TestMultiPathWatchPathExpressions, TestSameLineDefinitions,
TestSection4_2_23_BreakStatement), plus pre-existing failures in
cmd/, internal/interpreter, internal/security, internal/storage,
internal/zone_deprecated, internal/mesh (build), test/integration,
and tests/integration (build) — all of which predate Phase 1. No new
failures introduced by any of Steps 1.1–2.6 or Step 3.1.

Phase 1, Phase 2, and Phase 3 of `syntax_changes_development_plan.md`
are complete.

Run `go test ./...` and verify the failure set matches the pre-existing
baseline (no new failures introduced by any step).

Search the codebase for any remaining references to the removed operators:
```bash
grep -rn '?!=' internal/ cmd/ tests/ test/
grep -rn '?<' internal/ cmd/ tests/ test/
grep -rn '?>' internal/ cmd/ tests/ test/
grep -rn '?<=' internal/ cmd/ tests/ test/
grep -rn '?>=' internal/ cmd/ tests/ test/
```

Remove any dead code, unused token constants, or orphaned AST node types
left over from the removal.

Files: any files with remaining references

---

## Summary

| Step | Feature | Depends On | Complexity |
|------|---------|-----------|-----------|
| 1.1 | Remove old operators from lexer | — | small |
| 1.2 | Remove from parser | 1.1 | small |
| 1.3 | Remove from interpreter | 1.2 | small |
| 1.4 | Implement `?= unknown` matching | 1.3 | medium |
| 1.5 | Update MBL script tests | 1.4 | small |
| 2.1 | Parse type-first procedures | — | medium |
| 2.2 | Evaluate type-first procedures | 2.1 | medium |
| 2.3 | Parse type-first watchers | 2.1 | small-medium |
| 2.4 | Evaluate type-first watchers | 2.3 | medium |
| 2.5 | Scope resolution verification | 2.2, 2.4 | medium |
| 2.6 | MBL script tests | 2.5 | small |
| 3.1 | Final cleanup | all | small |

Phase 1 and Phase 2 are independent — they can be done in either order or
interleaved. Phase 3 is last.
