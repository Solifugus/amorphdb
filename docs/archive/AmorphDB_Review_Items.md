# AmorphDB Post-Test-Plan Review Items

These items were identified during the test plan implementation sessions
(Sections 1–12) and need manual review or follow-up work.

---

## Spec Issues

- **Block comment syntax (`##`, `###`)** — Added to `amorphdb_design.md` but
  not yet implemented in the lexer or covered by tests. Standalone task: add
  block comment token handling to lexer, write tests from spec examples,
  confirm no regressions. Only affects the lexer — no parser/interpreter impact.

- **Procedure definitions** — Claude Code claimed "not yet specified" in
  Section 5 but procedures ARE fully specified in the spec (Procedures section).
  Need to verify the interpreter actually handles full procedure syntax
  including persistent sub-attributes (`.count = .count + 1` between calls).

---

## Suspicious Skips to Audit

Review each skipped test against `amorphdb_design.md`. Only `t.Skip()` is
legitimate for features explicitly marked 🚧 in the spec.

- **Section 5 (5 skipped):** Procedure scope, exception handling, record
  literals, time formatting. Procedures and catch/else are specified in the
  spec, not marked 🚧. May need to be implemented.

- **Section 6 (16 skipped):** That's a large number. Need the full list with
  skip reasons and verification against the spec. The catch/else skip is
  legitimate, but reasons like "parser limitations" and "DefinitionStatement
  syntax not implemented" sound like things that should work.

- **Section 6.3.2 (multi-path watcher double-fire):** Claude Code said it
  "fixed test isolation by creating fresh engine instances per sub-test." This
  might be masking a real bug rather than fixing the actual issue. The spec says
  when multiple watched paths change in the same tick, the watcher fires exactly
  once. Verify the watcher engine actually deduplicates triggers per tick, not
  just that the test avoids the scenario.

- **Section 9 (20 skipped after audit):** The self-audit recovered 11 tests
  from the original 31 skips, bringing mesh to 14/34. The remaining 20 skips
  are likely a mix of legitimate multi-node requirements and features that have
  code but weren't tested. Particular areas to check:
  - Bridge tests (7 skipped) — `internal/mesh/bridge.go` exists with
    CreateBridge(), mobile agent behavior, disconnect. How many of these
    can be unit-tested without a second node?
  - Disconnection/Recovery (4 skipped) — `internal/mesh/leave.go` exists.
    GracefulDetach and IdentityPreservation may be testable in isolation.
  - Data Distribution (8 skipped) — subscription model is the architecture
    change in progress. These skips may be legitimately blocked.

- **Section 10 (2 skipped out of 5):** World clock is a small section. Check
  what was skipped — clock watcher triggering and timezone handling should both
  be testable.

- **Section 11 (8 skipped out of 14):** Security had 6 passing and 8 skipped.
  The passing tests cover DH key exchange, identity generation, and
  challenge-response. Check whether the skips include:
  - Agent-level encryption (specified, not 🚧)
  - Key rotation (specified, not 🚧)
  - Multiple device secrets (specified, not 🚧)
  - Local socket no-encryption vs network socket mandatory encryption

- **Section 12 (4 skipped out of 9):** Purge and compaction had 5 passing.
  Check whether archival migration (`@older_instance_id` redirect) and
  compaction-under-load were skipped — both are specified.

---

## Potential Code-Warping to Verify

These are cases where Claude Code may have adjusted tests or taken shortcuts
instead of fixing the actual implementation.

- **Section 4 — `PassStatement` AST node:** Confirmed legitimate. `pass` is
  now in the spec and reserved words list.

- **Section 4 — Context-sensitive `=`:** The `parseFilterExpression()` approach
  is correct per the spec. Worth reviewing the actual parser code to confirm
  `=` is assignment at statement level and comparison inside `[]` and conditions.

- **Section 5 — `round`, `floor`, `ceil`, `..remove` returning Nothing:** These
  were reported as broken then reported as fixed. Verify the fixes are in the
  interpreter code (not adjusted test expectations). Check that `round(3.7)`
  returns `4`, `floor(3.7)` returns `3`, `ceil(3.2)` returns `4`.

- **Section 6.5.2 — `shouldRollback()` fix:** Claude Code said it "fixed
  shouldRollback() to trigger on any Unknown." But the spec says rollback occurs
  when an Unknown *escapes the watcher body unhandled*. If a watcher catches the
  Unknown internally, changes should commit. Verify the implementation
  distinguishes handled vs unhandled Unknown.

---

## Architecture Questions

These are deeper questions about whether the test plan implementations reflect
real architectural features or were simulated to pass tests.

- **Heartbeat atomicity staging:** Was this implemented as a real staging buffer
  in the watcher engine, or simulated in the tests? The spec requires all writes
  within a watcher execution to be staged locally and flushed as a single batch
  at the heartbeat boundary. Check `internal/watcher/` for actual staging
  buffer code.

- **Watcher cascade across ticks:** Does the engine actually defer triggered
  watchers to the next tick? The spec says Watcher A writing to a path watched
  by Watcher B causes B to fire in the next tick, not the same tick. Verify
  this is enforced in the engine, not just tested in isolation.

- **Commit buffer limit:** The spec says 10,000 writes max per execution.
  Verify this is enforced in the watcher engine with a real counter, not just
  tested with a mock.

- **Outer run staging:** The spec says outer runs (programs sent by clients)
  also stage all writes and flush on exit, with rollback on unhandled Unknown.
  Verify this is implemented in the service layer, not just in the watcher
  engine tests.

---

## Missing Protocol Features

- **MBL EXECUTE protocol message** — The wire protocol in
  `internal/protocol/messages.go` defines READ, WRITE, PURGE, STATUS, STOP,
  and mesh operations but lacks an EXECUTE message type for sending MBL code
  to be executed remotely. This blocks Section 8 tests that require remote MBL
  execution (8.1.4-8.1.6: service MBL integration, and parts of 8.2.1-8.2.8:
  client REPL). The service has per-connection MBL interpreter instances but
  no protocol mechanism to receive and execute MBL code.
  Required implementation: add EXECUTE message type, request/response handling
  in protocol package, and service-side execution delegation to connection
  interpreters.

---

## Multi-Node Test Infrastructure

The following tests require a multi-node test harness that can spin up multiple
`amorphd` instances, manage their lifecycle, and test distributed behavior.
This is a separate test infrastructure project, not something Claude Code can
do in `go test` unit tests.

Tests requiring multi-node harness:
- Section 9: Subscription-based data distribution, write routing, authority
  splitting, subscription load balancing, network partition recovery
- Section 13.2: Two-node write/read, distributed watcher pipeline, agent
  mobility, authority failover, bridge data flow
- Section 13.3: High write throughput, node kill during heartbeat, network
  partition simulation, compaction under load

Suggested approach: Go test harness in `test/multinode/` that uses
`exec.Command` to start/stop `amorphd` instances on different ports, with
helper functions for writing data to one node and reading from another.

---

## Follow-Up Tasks (Priority Order)

### P0 — Correctness (do first)
1. Verify heartbeat atomicity staging is real, not simulated
2. Verify `shouldRollback()` distinguishes handled vs unhandled Unknown
3. Verify watcher cascade defers to next tick in the engine
4. Verify multi-path watcher deduplicates triggers (Section 6.3.2 concern)
5. Verify `round`, `floor`, `ceil`, `..remove` fixes are in interpreter code
6. Full regression run: `go test ./...`

### P1 — Missing implementations (do before real use)
7. Implement MBL EXECUTE protocol message
8. Implement block comment syntax in lexer
9. Audit and fix all suspicious skips (Sections 5, 6, 10, 11, 12)
10. Implement catch/else exception handling (blocks multiple skipped tests)

### P2 — Feature completeness
11. Implement record literal assignment (`x = { name: "...", age: 55 }`)
12. Implement projection syntax (`path{ name, age }`)
13. Implement procedure persistent sub-attributes
14. Build multi-node test harness
15. Run Sections 13–14 integration and MBL program tests
