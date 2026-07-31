# AmorphDB Development Plan

## How To Use This File

This file is read by Claude Code at the start of every session alongside CLAUDE.md.

**Status markers:**
- `TODO` — not started
- `IN PROGRESS (started: YYYY-MM-DD)` — started but not complete
- `DONE (completed: YYYY-MM-DD)` — complete and verified

**Session startup procedure:**
1. Read `CLAUDE.md` (repo root) fully
2. Read `DEVPLAN.md` (this file, at the repository root) fully
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

## What is outstanding

Every step below is `TODO`. Completed work — Steps 1–19a, 25, 28 and 30 — lives
in `docs/archive/DEVPLAN_completed.md`, together with the findings each one
produced. This file is deliberately kept to the work that remains.

| Step | Description | Phase | Blocked on |
|---|---|---|---|
| 20 | Time part access via `.@year` meta-attributes | Date/Time | — |
| 21 | `format_time` / `parse_time` and timezones | Date/Time | — |
| 22 | Duration type and interval syntax | Date/Time | a decision: bare units or a real Duration type |
| 23 | Calendar adjusters | Date/Time | Step 22 |
| 24 | Recurrence rules (RFC 5545 RRULE) | Date/Time | Step 23 |
| 26 | Comparison and range temporal queries | Temporal | a decision: what a range query returns |
| 27 | Instance meta attributes `.@time` / `.@agent` | Temporal | — |
| 29 | Watcher cascade: convergence by design | Watchers | — |
| 32 | Reading a stored container returns silently wrong answers | Interpreter | — |

**Two decisions are waiting**, both recorded in the steps that need them: what a
range temporal query returns (Step 26), and whether time arithmetic gets a real
Duration type (Step 22). Neither blocks the other steps.

**Known defects not yet given a step:** the wire protocol cannot safely skip
unknown non-length-prefixed fields, so adding a field to an existing message is
not backward compatible; `sort`, `filter`, `map`, `reduce` and `convert` are not
implemented.

---

# Phase 6 — Date and Time

Five steps remain (20–24). Their prerequisite, Step 19, is **done** — time
literals now produce a real `types.Time` carrying the precision supplied, so
these steps have something to operate on. See
`docs/archive/DEVPLAN_completed.md` for what it fixed.

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

**A recurring rule across the phase:** when an operation cannot be answered
truthfully, return `unknown` naming the reason — never invent a plausible value.
A minute that was never specified, a comparison between `1 month` and `30 days`,
an unsupported RRULE part: each returns `unknown` rather than a guess. A silently
wrong schedule or timestamp is worse than an absent one.

**Standing constraint:** times are stored as UTC. Only *rendering* converts to a
timezone. Nothing in these steps may change how a timestamp is serialized.

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

---

# Phase 7 — Temporal Queries

The headline claim of the product — "query any historical value". The as-of form
(Step 25) and its wire support (Step 30) are **done**; what remains is the range
and comparison forms, and instance meta attributes.

**The storage engine was already built** — verified 2026-07-29 by direct test,
which is why Step 25 turned out to be plumbing rather than construction:

- `StorageTree.ReadAt(path, timestampMicros)` (`internal/storage/tree.go:187`)
  returns the value as of a timestamp. Confirmed working: two writes of 100 then
  250 return 100 at the earlier timestamp, 250 at now, and a clean
  "no instance found at timestamp" error before the first instance.
- `InstanceStore.FindInstanceAtTime` (`instances.go:202`) and
  `GetInstanceChain` (`instances.go:141`) provide the primitives underneath.
- `types.getTimeRange` (`internal/types/compare.go:330`) already converts a
  precision into a `[start, end]` window, which is exactly what an as-of query on
  a coarse literal like `@2026-01-15` needs.

`evalBracketFilter` (`internal/mbl/interpreter/interpreter.go:3062`) now
recognises a temporal bracket *before* evaluating its base — evaluating it first
would read the current value, the one thing a temporal query is not asking for.
Steps 26 and 27 extend that same entry point.

**Depends on:** Step 19 (done). Steps 20–24 are not required.

---

### Step 26 — Comparison and range temporal queries

**Status:** TODO

**Depends on:** Step 25

**Spec reference:** `docs/mbl_reference.md` — `[<@t]`, `[>=@t]`,
`[@ >= @a, @ < @b]`

**Scope:** `internal/mbl/interpreter/interpreter.go`, `internal/storage/tree.go`
(a read-only chain accessor may be needed)

**Design decision required before starting:** what does a range query *return*?
A range spans several instances, so it cannot return a bare value.
(a) a `types.List` of values — simple, but discards when each was recorded, which
is usually the reason for asking; or (b) a List of Records carrying the value
alongside `@time` and `@agent`. **(b) is the more useful answer and pairs with
Step 27; decide and record it here before writing code.**

**What to do:**
1. Support the bare comparison forms `[<@t]`, `[<=@t]`, `[>@t]`, `[>=@t]`.
2. Support the two-part range form `[@ >= @a, @ < @b]`, where bare `@` denotes
   the instance timestamp. Note the parser already yields a leading-`@`
   `PathExpression` for bare `@` (`parser.go` parseTimeLiteral).
3. Add a chain accessor over `GetInstanceChain` filtered by time window. Reads
   only — do not change how instances are written.
4. **Bound the result.** A long chain must not materialise without limit; apply a
   cap and return `unknown` when exceeded, matching the commit-buffer and
   `MaxCallDepth` guards.
5. Ordering must be documented and stable — newest-first or oldest-first, chosen
   once and asserted in tests.

**New tests to write:**
```go
`x = my.bal[>=@2026-01-01]`                    // instances at or after
`x = my.bal[<@2026-01-01]`                     // strictly before
`x = my.bal[@ >= @2026-01-01, @ < @2026-02-01]`// range
`x = my.bal[@ >= @2030-01-01]`                 // empty list, not unknown
// A chain longer than the cap returns unknown naming the limit
```

---

### Step 27 — Instance meta attributes

**Status:** TODO

**Depends on:** Step 25

**Spec reference:** `docs/mbl_reference.md` — `person.name.@time`, `.@agent`,
`.@previous_instance_id`, `.@size`

**Scope:** `internal/mbl/interpreter/interpreter.go`

⚠️ **Collides with Phase 6 Step 20.** Step 20 adds time *parts* as `.@year`,
`.@month` and so on, applied to a `types.Time`. This step adds instance *meta* as
`.@time`, `.@agent`, applied to any stored value. Both are `.@name` on a path, so
resolution order must be explicit and documented: **instance meta is a property
of the stored instance and takes precedence; time parts apply only when the
receiver is a `types.Time` value.** `my.d.@time` on a stored time therefore means
"when was this recorded", not "the time component". Settle this in whichever step
lands second, and cross-reference it here.

**What to do:**
1. Implement `.@time` (`types.Time` at subsecond precision), `.@agent`,
   `.@previous_instance_id`, `.@size`.
2. Source them from the `Instance` record, not the value.
3. Composes with Step 25: `my.bal[@2026-01-01].@agent` answers "who set the value
   that was in effect then" — add an explicit test.
4. Meta on a path with no instance returns `unknown`.

**New tests to write:**
```go
`my.bal = 100
x = my.bal.@time`                  // types.Time, close to now()
`x = my.bal.@agent`                // the writing agent
`x = my.bal[@2026-01-01].@agent`   // composes with as-of
`x = my.missing.@time`             // unknown
```

**Verification:**
```bash
go test ./internal/mbl/... && go test ./...
```


---

### Step 29 — Watcher cascade: convergence by design

**Status:** PARTIALLY DONE (2026-07-30) — cascade now works and converges.
Remaining: the within-tick drain and the `quietly:` block form (see "What is
left" at the end of this step). Design revised 2026-07-30 after studying the watcher systems in
`~/development/gbasic` and `~/development/HiLow`. Supersedes the earlier
"repair the collection and audit every watcher" framing.

**Backward compatibility:** this changes when and how often watchers fire. That is
a deliberate semantic break, taken before 1.0 while it is cheap.

---

#### The two defects

**1. Cascade collection is dead.** `ExecuteWatcher`
(`internal/watcher/engine.go:389`) runs the watcher body via `interp.Interpret`,
then reads `interp.GetWrittenPaths()` to populate `pendingCascadePaths`. But
`Interpret` has already called `flushBuffer`, which clears the commit buffer, so
the list is always empty. Verified: a watcher whose body writes the path it
watches fires exactly once, identically to one that writes nothing.

**2. Equal-value writes still announce themselves.** `StorageTree.Write`
(`internal/storage/tree.go:158-169`) already compares the value ID and creates no
new instance when the value is unchanged — but it returns `nil` either way, so the
caller cannot tell. The path stays in the commit buffer and would trigger watchers
regardless. **We have the mechanism and discard the one bit that makes it useful.**

---

#### What the sibling languages do

gBASIC (`~/development/gbasic/src/eval.c`) — five mechanisms:

- **Unchanged assignments do not trigger.** `assign_lvalue` returns
  `LVALUE_ASSIGN_UNCHANGED` and the assignment path breaks before calling
  `watcher_trigger_change` (`eval.c:22228`).
- **Pending-dedup queue.** `watcher_enqueue` (`:4013`) no-ops when the watcher is
  already pending, so a burst of writes queues one run, not one per path.
- **One flat drain.** `watcher_drain` (`:4032`) walks a cursor over a queue that
  grows while it iterates; a write inside a watcher body enqueues onto the same
  queue and returns rather than starting a nested drain. Cascade emerges from a
  flat loop — no recursion, no deferral to a later tick.
- **A cap with a structured error** (10,000 per drain, `error.code = 1005`),
  raised against the *originating* statement: the drain saves
  `watcher_drain_origin_line` and restores it before raising, so the blame lands
  on the statement that started the cascade, not on whichever watcher was running
  when the counter tripped.
- **Region suppression** (`without watchers`) rather than a per-assignment modifier.

HiLow (`~/development/HiLow/docs/cell-redesign-brief.md`) — the transferable idea
is the **"deep-watched" bit** set down the parent chain at subscription time, so
propagation is skipped entirely when nobody deep-watches. Its identity-based
subscription solves shadowing, which AmorphDB does not have (paths *are* the
identity), so that part does not apply.

**Deliberately not adopted: synchronous immediate firing.** gBASIC fires before
execution continues past the mutating statement. That contradicts heartbeat
atomicity — which CLAUDE.md marks non-optional — and cannot survive distributed
write authority. Writes stay staged; the drain happens at the tick.

---

#### What to build

1. **Make `Write` report whether it changed anything.** Change the signature to
   return a "changed" indication (or add `WriteChanged`). `tree.go:158-169`
   already knows; it just discards the answer. Keep `Write`'s existing contract
   for other callers, or update them all — decide when reading the call sites.
2. **Do not trigger on unchanged writes.** Mark the `PendingWrite` accordingly, so
   an unchanged write neither creates an instance nor announces itself. This alone
   makes the common self-loop converge: a watcher that recomputes and writes back
   the same value stops on the second pass instead of spinning.
3. **Replace `pendingCascadePaths` with a drain queue in the engine.** Per tick:
   - a set of changed paths, populated as writes commit (not read back after the
     buffer is flushed — that is defect 1);
   - a queue of watchers to run, with a `pending` flag so a watcher is queued at
     most once at a time;
   - a cursor-based loop that keeps going as the queue grows, so a watcher's own
     writes feed the same drain.
4. **Cap the drain** and produce an `unknown` naming the limit, attributed to the
   change that started the drain. Mirror the existing guards (`MaxCallDepth`, the
   commit-buffer limit) in spirit and message style.
5. **Add a `quietly:` block form.** Per-assignment `(quietly)` is noisy when a
   watcher body writes several paths. Reuse the existing keyword with MBL's normal
   block convention rather than importing gBASIC's `without watchers`:
   ```
   quietly:
       my.a = 1
       my.b = 2
   ```
   `(quietly)` on a single assignment stays valid.

After this, `(quietly)` is an escape hatch rather than the primary loop guard —
which also shrinks the audit this step used to require, since equal-value
convergence handles the ordinary case on its own.

**Scope:** `internal/storage/tree.go`, `internal/watcher/engine.go`,
`internal/mbl/interpreter/interpreter.go`, and the lexer/parser for `quietly:`.

**New tests to write:**
```go
// The pair that proves both halves
// - a loud self-write re-triggers (cascade works at all)
// - a (quietly) self-write does not

// Convergence without (quietly): a watcher that writes back the SAME value
// must fire once and stop, not spin.

// Dedup: writes to three watched paths in one tick queue one run, not three.

// Cascade: watcher A writes a path watched by B; both run in the same tick.

// Runaway: two watchers writing each other's watched paths hit the cap and
// produce an unknown that names the originating change, not the watcher.

// quietly: block suppresses every write in the block.
```

**Verification:**
```bash
go test ./internal/watcher/... ./internal/storage/... ./internal/mbl/...
go test ./...
```

---

---

### Step 32 — Reading a stored container returns silently wrong answers

**Status:** TODO

**Discovered:** 2026-07-30, while checking whether the `Children` protocol stub
mattered. It did not — nothing reached it. This is what is actually broken.

The interpreter never enumerates the children of a stored path. Rather than
failing, each affected operation returns a plausible-looking wrong answer:

| MBL | Returns | Should be |
|---|---|---|
| `my.p` (a stored container) | `{}` | the record with its fields |
| `my.p{ name, age }` | `Unknown: not_found` per field | the field values |
| `my.p..count` | `0` | the number of children |

Verified against a live daemon after seeding `my.p.name` and `my.p.age`; the
leaf reads work, so the data is present and only the enumeration is missing.
In-memory records behave correctly — `my.l = [1,2,3]` then `my.l..count` gives 3
— so this is specific to paths whose children live in storage.

**Silent wrong answers are the problem here, not the missing feature.** A `0`
count and an empty record are indistinguishable from a genuinely empty node, so
application code cannot detect the difference. `docs/getting_started.md` already
documents "reading a bare container node" as unsupported, but a projection
answering `not_found` for a field that exists is worse than an unsupported
operation.

`storage.StorageTree.Children` works, and as of Step 31 so does the CHILDREN wire
path, so the primitives are in place for both the embedded and client-side
interpreter.

**Design decisions needed before starting:**
1. Should reading a container return a record of its children, or stay an
   explicit operation? Returning the record is what the spec implies and what
   projection needs.
2. How deep? A single level, or the whole subtree? Whole-subtree reads of a large
   node need a bound, in the spirit of the commit-buffer limit.
3. ~~`Children` returns attribute IDs, not names.~~ **Decided and done
   (2026-07-31):** the CHILDREN response now carries each child's name, so a
   remote client can build a record without a name-resolving round trip per
   child. Changed while nothing depended on the format — the protocol cannot add
   fields compatibly, so that window was going to close permanently.
   `StorageTree.ChildrenWithNames` and `ProtocolClient.ChildrenWithNames` are the
   entry points; `Tree.Children` is unchanged.

**Scope:** `internal/mbl/interpreter/`, possibly `internal/protocol` and
`internal/service` if the response shape changes.

---

### Step 31 — CHILDREN over the wire

**Status:** DONE (completed: 2026-07-30) — Implemented the CHILDREN codecs, a
real server handler (permission check identical to an ordinary read — enumerating
children reveals their names), and `ProtocolClient.Children`. Same three-stub
shape as Step 30: message types and routing existed, codecs did not, and both
ends returned errors.

Two corrections to what prompted this step. `Purge` was **already fully
implemented** on both client and server — the claim that it was a stub was wrong.
And implementing `Children` fixes nothing observable on its own, because the
interpreter never calls it; see Step 32 for what is actually broken.

Kept anyway because it completes the `storage.Tree` interface over the wire and
is a prerequisite for Step 32 on the client side — without it, teaching the
interpreter to enumerate children would fail remotely exactly as `ReadAt` did.

Note the response carries attribute IDs, not names: the label lives in the value
store, which a remote client cannot read. Step 32 has to resolve that.

Tests: `internal/protocol/readat_test.go` (round-trips including an empty list,
large IDs, and a corrupt count that must not cause a large allocation) and
`internal/service/readat_integration_test.go` (over a real socket).

---

### Step 29 — progress note (2026-07-30)

**Landed.** Cascade works for the first time, and converges.

Three defects had to be fixed, not the one the step described. Each was hidden
behind the next, which is why none had been noticed:

1. **Collection was dead.** `ExecuteWatcher` read `interp.GetWrittenPaths()`
   *after* `Interpret` had flushed and cleared the commit buffer. Fixed by
   recording changed paths at flush time — `Interpreter.ChangedPaths()`.
2. **Paths did not match.** Cascade recorded *resolved* paths
   (`world.agent.1000.middle`) while watchers register the form their author
   wrote (`my.middle`). They could never match. The engine now records the
   agent-relative form alongside the resolved one.
3. **Tick ordering destroyed the cascade.** `executeTick` called
   `CommitCascadeChanges()`, then `Tick()`, then `ClearChangeLog()` — so every
   cascade entry was stamped *before* the new watermark `GetTriggeredWatchers`
   compares against, and then wiped by the clear. Even with (1) and (2) fixed,
   nothing fired. The order is now Tick, clear, then commit the cascade.

**Convergence is in place.** `StorageTree.WriteReportingChange` exposes what
storage already knew — an identical value creates no instance — and the
interpreter omits both unchanged and `(quietly)` writes from `ChangedPaths`. A
watcher that recomputes and writes back the same value now settles instead of
spinning.

Deliberately a concrete method rather than a change to the `Tree` interface: the
other three implementations would all have to change, and `ProtocolClient` would
need a new field on `WRITE_ACK`, which the wire protocol cannot add compatibly.
Callers type-assert for the capability and fall back to `Write`.

**Runaway cap.** `MaxCascadeRounds` (1000) counts *consecutive* cascading ticks
and reports the still-changing paths, so a chain that genuinely cannot converge
is stopped and named rather than cascading forever.

Tests: `internal/watcher/cascade_test.go` — a dependent watcher is reached, an
unchanged self-write converges, a `(quietly)` self-write does not cascade, the
cap fires and names paths, and the counter resets on quiet rounds.

**What is left of this step:**
- The **`quietly:` block form**. Per-assignment `(quietly)` works; the block is
  lexer/parser work and independent of everything above.

**Closed, not outstanding: the within-tick drain.** An earlier draft of this step
proposed adopting gBASIC's flat cursor-based drain so a chain settles inside one
tick, and the spec section written alongside it claimed that behaviour. That was
wrong — it was introduced without checking it against the intended design.
One-link-per-tick is deliberate: each link's effect becomes its own instance in
the temporal record, so the chain of causation stays queryable, and each tick's
work stays bounded. The spec has been corrected to describe the intended model,
including that intermediate states between links are observable by design.

**Resolved 2026-07-31: watcher matching is descendant-aware.** A watcher on
`my.orders` now matches a change at `my.orders` or anything beneath it. Matching
was exact string equality, which made watching a container nearly pointless and
was incoherent besides — because a write records the intermediate nodes it
creates, a watcher on a parent fired when a child was *created* but not when an
existing one changed. The comparison requires a dot boundary, so `my.order` does
not match `my.orders.17`.
