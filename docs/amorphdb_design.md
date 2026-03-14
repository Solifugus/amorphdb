# AmorphDB

## Overview

AmorphDB is a temporal tree-graph database. Every value has a history — assignment records a change rather than overwriting what came before. Data is organized as a hierarchy of attributes, each holding a chain of timestamped instances, making the full evolution of any piece of data queryable at any point in time.

The system is accessed through MBL (Modern Business Language), a notation designed around scope resolution and path traversal. Types are deliberately high-level — a number is always the largest float the processor supports, a picture is always full RGBA at 16-bit depth. This trades storage efficiency for uniform simplicity: code that operates on data never needs to negotiate formats or handle edge cases between representations.

The underlying storage model consists of three structures — attributes, instances, and values — linked together to form a navigable, append-only temporal graph.


## Temporal Semantics

Every indexed attribute — whether numerically or textually indexed — maintains a history of values, not just a current value. Assignment is therefore not the setting of a value, but the recording of a change. If the new value is identical to the current value, no change is recorded.

When a value changes, the question of whether sub-attributes carry forward from the previous value needs to be explicitly defined per use case.


## Notation and Language

### Line Prefixes

The first non-whitespace character on a line determines scope resolution:

| Prefix | Meaning |
|--------|---------|
| `~`    | References the user's home |
| `.`    | Reference is relative to the current scope |
| `+`    | For textually indexed fields: overwrite. For numerically indexed fields: append |
| ``     | Nearest matching upstream scope; otherwise behaves the same as `.` |

Dot notation traverses the hierarchy (e.g., `parent.child.grandchild`).

### Suffixed Brackets

Brackets appended to a path element serve different roles depending on their contents:

- **Single value** — acts as an index lookup
- **Logic expression** — a dynamic query scoped to the attributes under the suffixed point (essentially a mini-lambda)
- **Comma-separated list** — all conditions apply (logical AND)

Within a logic expression, the keywords `and` and `or` are available. Commas are equivalent to `and`. Parentheses may be used for grouping.

    person[name = "bob", age > 18]                      # commas: implicit AND
    person[name = "bob" and age > 18]                    # keyword: same thing
    person[name = "bob" or name = "robert"]              # OR requires keyword
    person[(name = "bob" or name = "robert"), age > 18]  # mixed: OR group AND'd with age

#### Temporal Queries

Because every instance carries a timestamp, temporal filtering is available inside brackets. A bare `@` refers to the instance's timestamp. A bare time literal defaults to "as of" (effectively `<=`).

    person.name[@2025-06-01]                        # as of: most recent value at or before this time
    person.name[<@2025-06-01]                       # before
    person.name[<=@2025-06-01]                      # at or before (same as bare)
    person.name[>@2025-06-01]                       # after
    person.name[>=@2025-06-01]                      # at or after
    person.name[>@2025-05-01, <@2025-06-01]         # range (AND, via comma)

Temporal conditions can be mixed with attribute conditions in the same expression:

    person[name = "bob", age > 18, @ < @2025-04-15]

In the full logic expression form, `@` appears explicitly on the left side of a comparison:

    person.name[@ > @2025-05-01, @ < @2025-06-01]

### Suffixes

| Suffix | Meaning |
|--------|---------|
| `:`    | Assignment of a definition |
| `=`    | Assignment of a value (i.e., recording a change) |
| `.`    | Sets the current scope |

### Data Types

Text, Number, Time, Money, Picture, Reference, Procedure, Watcher, Embed

- Text
  UTF-8 dynamic character string.
  In MBL, a text literal is specified between one or more adjacent quote (") characters.
  The number of adjacent quotes that opens a text literal value are required to end it.
  Only visible characters (including whitespace, newlines, etc) are allowed.
  Examples:
  - "Hello World"
  - ""I said "Hello World", didn't I?""
- Number
  The largest floating point number supported by the processor architecture.
  In MBL, literal numbers may not begin or end with a period (.), may begin with a negation (-), and may include underscore characters (though they are ignored--only for visual clarity purposes).
- Time
  UNIX Time (2038-proofed) kept in UTC.
  In MBL, a time literal begins with the "@" symbol and follows in the order of YYYY-MM-DD hh:mm:ss.n
  The "@" symbol more broadly means "instance-level" — time literals are one use,
  while instance meta attributes (see below) are another. The parser distinguishes
  them: "@" followed by digits is a time literal, "@" followed by a name is a meta
  attribute, and bare "@" in a comparison refers to the instance's timestamp.
  Examples:
    - @2026-02-01 (February 1st, 2026)
    - @2026-02-15 15:30:00.0 (February 15th, 2026 at 3:30 PM)
- Money
  Text/decimal storing monetary values with currency symbol.
  In MBL, A money literal is specified as a currency symbol followed immediately by a decimal number.
  Examples:
    - $143.68
    - €21.80
- Picture
  Full RGBA at 16-bits each.
  In MBL, it cannot be typed as a literal but must be loaded via an operation.
- Reference
  A link (by name or unique identifier) to another node elsewhere in the tree.
  In MBL, literal references are specified by doubling the normal notation for that path (e.g., "world.foo" resolves to a value, while ""world.foo"" is a reference to that location).
- Procedure
  A stored program (see Procedures below).
- Watcher
  A persistent trigger (see Watchers below).
- Embed
  External data marked as content (not metadata).

### Meta Types

Each type above supports meta types:

- Unknown
  Indicates that the expected value is not available, along with a reason why.
  In MBL, this is the "#" character followed by text explaining why.
  Examples:
    - #offline
    - #access_denied
    - #not_found
- Queued
  Indicates that a value will be set but has not been obtained yet, possibly with expected delivery information.
  In MBL, this is the "?" character followed by text explaining when it will be available.
  Examples:
    - ?tomorrow
    - ?in_5_seconds
- Redacted
  Indicates that a value exists but may not be displayed at this level of access or under this filter.
  In MBL, this is displayed as asterisks only and cannot be specified directly (only visible as the result of accessing filtered content).
  Examples:
    - ***, ****, *****, etc. (by character count of original)

### Structures

AmorphDB provides two aggregate structures: Records and Lists. Both are based on the same underlying tree structure.

- Records store data in named fields.
- Lists store data in numerically indexed fields.

Since these are both tree nodes with attributes underneath, either may contain any mix of types including other records, other lists, or any other type. The distinction exists only to indicate the predominant access method.

### Instance Meta Attributes

Every instance — every value ever recorded at every path in the tree — has meta attributes available alongside the value itself. These are queried using the "@" prefix:

| Meta attribute | Type | Meaning |
|-------|------|---------|
| @time | Time | Timestamp when this instance was created |
| @agent | Reference | Agent that created this instance |
| @statement_id | Reference | Statement that created this instance (for audit) |
| @previous_instance_id | Reference | Points to the previous instance of this attribute |
| @next_instance_id | Reference | Points to the next instance of this attribute |
| @older_instance_id | Reference | Points to an older instance that was moved to archival storage |
| @size | Number | Total size of this instance in bytes |
| @ttl | Number | Time-to-live in milliseconds (zero = permanent) |

Examples:

    # When was this value set?
    my.account.balance.@time

    # Who changed my password last?
    my.account.password.@agent

    # What was the previous value of this field?
    my.account.status.@previous_instance_id

    # How large is this record?
    my.documents.report.@size

Instance meta attributes can be used in brackets for queries:

    # All changes made by kalevo today
    my.log[@agent = ""world.agent.kalevo"", @time >= @2026-03-01]

    # Values that are temporary (have TTL)
    my.cache[@ttl > 0]

    # Large files (over 1MB)
    my.files[@size > 1000000]

The double reference `""` syntax indicates that `world.agent.kalevo` should be treated as a reference to that path, not the value at that path.

## MBL (Modern Business Language)

### Lexical Structure

MBL source text is UTF-8. A program is a sequence of statements separated by whitespace or newlines. Comments begin with `#` and extend to end of line:

    # This is a comment
    my.account.balance = 100    # Another comment

**Identifiers** are sequences of letters, numbers, or underscores, beginning with a letter or underscore. Case is significant: `Account` and `account` are different. Identifiers may contain Unicode characters. Reserved words cannot be used as identifiers:

    and, or, not, if, then, else, while, for, in, return, new, quietly,
    procedure, watch, catch, my, world, true, false, null

**Numbers** may be integers or floating-point, and may include underscores for visual grouping:

    42
    3.14159
    1_000_000.50

**Time literals** begin with `@` and specify times in UTC:

    @2026-01-15                    # Date only (midnight UTC)
    @2026-01-15 14:30              # Date and time
    @2026-01-15 14:30:22           # Including seconds
    @2026-01-15 14:30:22.500       # Including milliseconds

**Text literals** are enclosed in quotes. Use multiple quotes to include quotes in the text:

    "Hello"                        # Simple string
    ""He said "Hello" to me""      # Includes quotes
    """Complex "quoted" text"""    # More quotes

**Operators** include arithmetic, comparison, and logical operators:

    +, -, *, /, %, ^               # Arithmetic
    =, !=, <, >, <=, >=            # Comparison
    and, or, not                   # Logical

**Paths** are dot-separated sequences of identifiers, with special prefixes:

    world.market.price             # Absolute path
    .local.variable               # Relative to current scope
    ~.account.balance             # Relative to user's home
    my.account.balance            # Same as above (from program root)

**References** double the path syntax:

    ""world.market.price""         # Reference to a location (not its value)

### Operators and Expressions

**Arithmetic:** +, -, *, /, % (modulo), ^ (exponentiation)

**Comparison:** =, !=, <, >, <=, >=
- Comparison of different types follows type coercion rules
- Text comparison is lexicographic using Unicode code points
- Time comparison uses chronological order

**Logical:** and, or, not
- Short-circuit evaluation: `A and B` does not evaluate B if A is false
- `and` and `or` have equal precedence and are left-associative
- `not` has higher precedence

**Assignment:** =, :=
- `=` records a value change (creates a new instance)
- `:=` records a definition (used for procedures, watchers, types)

**Path Resolution:** ., .., ~
- `.` accesses child attributes
- `..` accesses special operations on collections
- `~` accesses user's home

**Type Checking:** ?=, ?!=, ?<, ?>, ?<=, ?>=
- Comparison operators with "?" prefix check for Unknown and Queued values
- Return true/false instead of propagating Unknown/Queued up the expression tree
- Example: `if x ?= 5:` succeeds if x is 5, fails safely if x is Unknown

**Collection Operations:**
- `..count` - number of attributes under a node
- `..remove(index)` - remove by position
- `..remove(value)` - remove first match by value
- `..combine(separator)` - join values with separator

**Brackets:**
- `[expression]` - filter or query child attributes
- Used for array indexing: `my.array[0]`
- Used for conditional queries: `my.users[active = true]`

**Operator Precedence** (highest to lowest):
1. Path resolution (.), member access
2. Unary not, unary -, unary +
3. Exponentiation (^)
4. Multiplication (*), Division (/), Modulo (%)
5. Addition (+), Subtraction (-)
6. Comparison (=, !=, <, >, <=, >=)
7. Logical and
8. Logical or
9. Assignment (=, :=)

### Type Coercion

MBL performs automatic type conversion in expressions when possible:

**Number conversions:**
- Text to Number: Numeric text is parsed; non-numeric text becomes Unknown
- Time to Number: Converts to Unix timestamp
- Money to Number: Extracts numeric value, discards currency symbol

**Text conversions:**
- Number to Text: Standard numeric formatting
- Time to Text: ISO 8601 format (YYYY-MM-DD HH:MM:SS)
- Money to Text: Preserves currency symbol and formatting
- Any type to Text: All types have text representations

**Boolean conversions:**
- Number: 0 is false, any other number is true
- Text: Empty string is false, any other text is true
- Time: Zero time (@1970-01-01 00:00:00) is false, any other time is true

**Error propagation:**
- Unknown values propagate through expressions: `5 + #offline` yields `#offline`
- Queued values propagate through expressions: `5 + ?tomorrow` yields `?tomorrow`
- Type-safe comparison operators (?=, ?!=, etc.) return false for Unknown/Queued instead of propagating

### System Operations

#### Built-in Procedures

**I/O Operations:**
```
my.computer.output(text)                    # Write to stdout
my.computer.input(prompt)                   # Read from stdin (returns text)
my.computer.files.read(path)                # Read file contents
my.computer.files.write(path, content)     # Write file contents
```

**Text Operations:**
```
length(text)                                # Character count
substring(text, start, length)              # Extract substring
find(text, pattern)                         # Search for pattern
replace(text, old, new)                     # Replace occurrences
split(text, separator)                      # Split into list
trim(text)                                  # Remove whitespace
upper(text), lower(text)                    # Case conversion
```

**Math Operations:**
```
abs(number)                                 # Absolute value
round(number), floor(number), ceiling(number)
min(a, b), max(a, b)                       # Comparison
sqrt(number), log(number), exp(number)      # Mathematical functions
random()                                   # Random number 0.0 to 1.0
```

**Time Operations:**
```
now()                                      # Current time
format_time(time, format)                 # Custom time formatting
parse_time(text, format)                  # Parse time from text
add_time(time, days, hours, minutes)      # Time arithmetic
```

**Type Operations:**
```
type_of(value)                            # Returns type name as text
is_unknown(value), is_queued(value)       # Check for meta types
convert(value, type)                      # Explicit type conversion
```

**Collection Operations:**
```
sort(collection, key)                     # Sort by attribute
filter(collection, condition)            # Filter items
map(collection, procedure)               # Transform items
reduce(collection, procedure, initial)   # Aggregate items
```

#### List and Record Operations

Lists and records are the same underlying structure — a node with attributes. Lists are indexed numerically, records by name. The following operations modify in place.

    x..count                                    # number of attributes under this node
    x..combine(separator = ",")                 # join values into text
    x..remove(position)                         # remove by numeric index (list reindexes)
    x..remove(value)                            # remove first match by value (list) or by name (record)
    x..remove(from, to)                         # remove a range by index

    # Examples
    my.shopping_list..count                     # how many items
    my.shopping_list..remove(2)                 # remove third item (zero-indexed)
    my.contacts..remove("bob")                  # remove contact named bob
    my.numbers..combine(" + ")                  # "1 + 2 + 3"

Assignment with `+` prefix appends to lists:

    +my.shopping_list = "milk"                  # append item to end
    +my.log = now() & ": system started"       # append log entry

### Control Flow

**Conditional Statements:**
```
if condition:
    statements

if condition:
    statements
else:
    statements

if condition1:
    statements
else if condition2:
    statements
else:
    statements
```

**Loops:**
```
# For-each loop over collection
for item in collection:
    statements

# For-each with index
for item, index in collection:
    statements

# While loop
while condition:
    statements
```

**Error Handling:**
```
catch:
    risky_operation()
else unknown:
    # Handle Unknown values
    my.computer.output("Operation failed: " & unknown)
else queued:
    # Handle Queued values
    my.computer.output("Operation delayed: " & queued)
```

**Flow Control:**
```
return value                                # Exit procedure with value
return                                      # Exit procedure with null
```

### Procedures

A **procedure** is a reusable block of code that accepts parameters and returns a value. Procedures are first-class values — they can be stored in attributes, passed as parameters, and returned from other procedures.

**Definition:**
```
my.functions.calculate_tax: procedure(income, rate):
    if income <= 0:
        return 0
    else:
        return income * rate
```

**Calling:**
```
tax_owed = my.functions.calculate_tax(50000, 0.25)
```

**Parameters and Local Scope:**
- Parameters are passed by value
- Procedures have their own local scope
- Local variables exist only during procedure execution
- Access persistent data via `my` and `world` keywords

**Sub-attributes as Local Storage:**
A procedure's sub-attributes serve as persistent local storage between calls:

```
my.counters.page_views: procedure():
    .count = .count + 1                     # Persistent between calls
    return .count
```

**Anonymous Procedures:**
```
# Assign procedure directly
my.double: procedure(x): return x * 2

# Pass as parameter
my.numbers = [1, 2, 3, 4, 5]
my.doubled = map(my.numbers, procedure(x): return x * 2)
```

### Watchers

A **watcher** is a procedure that runs automatically when specified attributes change. Watchers enable reactive programming — code that responds to data changes rather than being explicitly called.

**Definition:**
```
my.automation.balance_check: watch(my.account.balance, my.account.limit):
    if my.account.balance > my.account.limit:
        my.account.status = (quietly) "overlimit"
```

A watcher is a value in the hierarchy like any other. It is operated by the node responsible for the zone where it lives. Its sub-attributes form its persistent local data scope.

#### Watcher Attributes

Every watcher has the following attributes:

| Attribute | Type | Default | Meaning |
|-----------|------|---------|---------|
| `@enabled` | Boolean | true | Whether this watcher is active |
| `@watching` | List of References | (from definition) | Paths being monitored |
| `@code` | Procedure | (from definition) | Code to execute on changes |
| `@last_run` | Time | null | When this watcher last executed |
| `@run_count` | Number | 0 | Total number of executions |
| `@last_error` | Text | null | Error message from last failed run |

**Enabling/Disabling:**
```
my.automation.balance_check.@enabled = false           # Disable
my.automation.balance_check.@enabled = true            # Re-enable

# Conditional disable
for w in my.automation:
    if w.@last_error:
        my.computer.output(w & " is disabled")
```

All changes to `@enabled`, `@watching`, or `@code` take effect on the next mesh heartbeat. Because watchers are data in the hierarchy, permissions apply naturally — you cannot disable or modify another agent's watcher unless you have `@write` permission on it.

#### Quiet Assignment

The `(quietly)` modifier prevents assignments from triggering watchers:

```
my.account.status = (quietly) "maintenance"     # No watchers triggered
```

This is essential for watchers that modify the data they're watching, preventing infinite loops.

### Execution Model

AmorphDB supports two execution models:

#### Outer Run

An outer run is a program sent into the service by a client for execution. The program's local scope exists only in memory — it is not persistent. However, the `my` and `world` keywords reach into persistent storage in the mesh, allowing the program to read, write, inject procedures, set up watchers, or perform ad hoc operations.

When the program ends, its local scope is discarded. Anything it wrote to `my` or `world` persists.

**Examples:**
```
# Interactive session
my.account.balance = my.account.balance + 100
my.computer.output("New balance: " & my.account.balance)

# Script execution
total = 0
for transaction in my.account.transactions:
    total = total + transaction.amount
my.account.computed_balance = total
```

#### Inner Run

An **inner run** is a watcher or procedure stored in the persistent hierarchy and triggered by data changes. Inner runs are the mesh's reactive system — code that responds to changes automatically.

Inner runs are the mesh's nervous system. There is no scheduler, no cron, no event queue — just watchers reacting to changes in the data they monitor, including `world.clock`.

The maximum execution speed of any inner run is one third of a second, matching the mesh heartbeat. This interval is the rate at which updates propagate across the mesh and is derived from the minimum time the human brain can distinguish between events.

#### Heartbeat Atomicity

Within a single heartbeat tick, a watcher's execution is atomic. All changes it makes to persistent data — whether in the local zone or in remote zones — are staged locally until execution completes. At the heartbeat boundary, staged writes are flushed as a single coordinated batch: local zone writes commit directly, and cross-zone writes are dispatched together as one replication payload per destination zone.

This means a loop that writes to many attributes, even across many nodes, does not generate one network message per iteration. The mesh sees one write batch per tick, not one write per assignment. The cost of a loop is bounded by the size of the commit buffer, not by the number of iterations.

If the watcher produces an unhandled Unknown that escapes its body — indicating a failure such as a hardware error, network problem, or unresolvable operation — all staged changes are discarded. None are replicated.

If the watcher handles the Unknown internally (checks for it, takes corrective action), that is normal operation and changes commit as expected. The rollback only occurs when an Unknown propagates out of the watcher unhandled.

This requires no distributed transaction coordination. Changes do not leave the node until the heartbeat boundary, so rollback is purely local — the node discards the staged write buffer.

**Commit buffer limits:** A single execution may stage at most a configurable number of persistent writes (default: 10,000). If the limit is exceeded, an Unknown is produced with reason "commit buffer exceeded." This can be handled like any other Unknown — caught internally for partial work or allowed to propagate for full rollback. The limit exists to prevent unbounded memory use and to keep per-tick replication payloads manageable.

The same staging model applies to outer runs. All persistent writes made during an outer run are staged and flushed when the program exits normally. An unhandled Unknown at the top level rolls back all staged writes from that run.

#### Cross-Zone Operations

Writes within a single zone are atomic within a heartbeat. When an operation spans multiple zones — such as transferring a value between two accounts on different nodes — true atomicity is not available. Instead, the recommended pattern uses a transaction record as the source of truth and watchers for self-healing.

**The pattern:**

1. Write a transaction record capturing the intent, marked as "pending."
2. Execute each step, updating the transaction status as steps complete.
3. Use watchers to detect incomplete transactions and retry or compensate.

This approach provides eventual consistency and automatic recovery without requiring distributed coordination.

### Scope

MBL provides multiple scope contexts for organizing data and controlling access:

#### Program Scope

All variable names without prefixes are **local variables** — they exist only within the running program and are discarded when execution ends. Local scope includes procedure parameters and temporary calculations:

```
# Local variables (temporary)
name = "Alice"
age = 30
temp = calculate_tax(income, rate)
```

#### Persistent Scope

The keywords `my` and `world` access persistent data stored in the mesh:

| Keyword | Destination |
|---------|-------------|
| `my` | The current agent's home in the mesh (`~`, i.e., `world.agent.{identity}`) |
| `world` | The global root of the persistent hierarchy (reachable as `my.world`) |

    program's in-memory tree:
        name = "Alice"                      # local variable
        income = 50000                      # local variable

    persistent storage in mesh:
        my.profile.name = "Alice"           # saved to agent's home
        world.market.price = 142.50         # saved to global hierarchy

The boundary between temporary and persistent is explicit:

    # all work is local — one persistent write at the end
    total = 0
    for item in my.inventory:
        total = total + item.value
    my.portfolio.total_value = total        # only this persists

Local variables can refer to persistent data:

    temp = { name: "Joe", age: 34 }    # local
    my.contacts.joe = temp              # now persistent in the mesh

When a program ends, anything not attached to `my` or `world` is discarded.

[Content continues with unchanged sections: Computer, Cascade, Watcher and Procedure Scope sections...]

#### The Computer

`~.computer` (accessible as `my.computer` from program root) is a virtual mount in the agent's home that provides access to the local machine. It is not stored in the mesh — it is provided by the node the agent is connected from.

    my.computer.output(value)       # write to stdout (procedure)
    my.computer.input(prompt)       # read from stdin (procedure, returns text)
    my.computer.files              # filesystem access
    my.computer.network            # network operations
    my.computer.system             # system information

This allows programs to interact with the physical machine they're running on while keeping the bulk of their data in the distributed mesh.

#### Cascade

In persistent storage, bare references do not cascade by default. Paths must be explicit. An attribute defined with `(cascade)` overrides this — its value becomes visible to bare references from descendant scopes in the persistent hierarchy.

The `~` sigil always resolves to the current agent's home in the mesh, regardless of scope depth. It works inside bracket expressions and any other context where `my` is not in scope.

#### Watcher and Procedure Scope

A watcher's sub-attributes are its persistent local data scope. The `.` prefix accesses these directly. The owning agent is the agent who created it, and `~` resolves to that agent's home. A watcher has access to `my.computer` only when executing on its originating node.

Procedures stored in the hierarchy work the same way — their sub-attributes are persistent local data accessible via `.` prefix.

## Data Storage and Retrieval System

AmorphDB's storage layer is built around three core structures:

**Attribute:** A named relationship in the hierarchy. Each attribute can hold multiple instances over time.

**Instance:** A timestamped value at a specific attribute. Instances form chains representing the complete history of changes.

**Value:** The actual data — text, number, time, etc. Values are stored separately and referenced by instances to enable efficient deduplication.

This structure supports:
- Complete temporal history of every data item
- Efficient storage through value deduplication
- Fast queries across both current and historical data
- Natural hierarchical organization

**Storage files** are organized per zone:
- `attributes.idx` - Attribute names and metadata
- `instances.idx` - Timestamped instance chains
- `values.dat` - Actual data values
- `references.idx` - Cross-references and relationships

The storage engine automatically handles:
- Temporal queries across instance history
- Value deduplication for storage efficiency
- Background compaction and cleanup
- Cross-zone reference resolution

### Storage File Organization

Each zone maintains its own set of storage files:

**attributes.idx:** Maps attribute names to their metadata
- Attribute ID (local to zone)
- Parent attribute ID (for hierarchy)
- Attribute name (text)
- Creation time and agent
- Current instance pointer
- First instance pointer (for temporal queries)

**instances.idx:** Chains of timestamped instances
- Instance ID (globally unique)
- Timestamp and creating agent
- Value reference (points to values.dat)
- Previous instance ID (temporal chain)
- Next instance ID (for reverse queries)
- Attribute ID this instance belongs to

**values.dat:** Actual data storage
- Value ID (for deduplication)
- Value type (Text, Number, Time, etc.)
- Value size and data bytes
- Reference count (for garbage collection)

This organization provides:
- O(1) access to current values via attribute index
- O(log n) temporal queries via instance chains
- Automatic deduplication when identical values are stored
- Efficient space utilization through reference counting

### Storage Optimizations

**Value deduplication:** Identical values are stored once and reference-counted. This is especially effective for:
- Repeated status values ("active", "inactive")
- Common configuration strings
- Frequently used numbers (0, 1, 100, etc.)
- Identical timestamps from batch operations

**Instance chain optimization:** Recent instances are stored together for cache locality. Historical instances may be moved to archival storage with redirect pointers.

**Background compaction:** Storage files are periodically reorganized to:
- Remove unreferenced values
- Compact fragmented space
- Optimize for current access patterns

### Purge

Data marked for purge is not immediately deleted. Instead, instances receive a time-to-live (TTL) value indicating when they should be removed. Background processes handle actual deletion:

**Soft purge:** Instance is marked with TTL but remains accessible until expiration
**Hard purge:** Instance and its value are immediately removed from storage
**Cascade purge:** Removal of an attribute also purges all its sub-attributes

The purge mechanism respects the temporal nature of the database — historical queries can still access purged data until the TTL expires.

**Compaction and Purge Integration:**

During compaction:

1. The node enters maintenance mode and replicas assume zone authority.
2. Values are relocated to fill gaps, working from the end of the file toward the beginning.
3. Instance records are updated with new value offsets as values move.
4. Free space is consolidated at the end and the file is truncated.
5. The node rejoins the mesh and resyncs — the replica sends all instances written during maintenance, which append cleanly to the freshly compacted files.
6. The node resumes authority and replicas return to replica mode.


## Stamps, Filters, and Permissions

### Stamps

A **stamp** is metadata attached to every piece of data an agent writes. Stamps follow data wherever it goes and provide context about origin, classification, and handling requirements.

**Built-in stamp attributes:**

| Attribute | Type | Meaning |
|-----------|------|---------|
| `@agent` | Reference | Agent who created this data |
| `@time` | Time | When the data was created |
| `@classification` | Text | Security/sensitivity level |
| `@source` | Text | Origin system or process |
| `@retention` | Time | How long to keep this data |
| `@geography` | Text | Legal jurisdiction restrictions |

**Custom stamps:** Agents can define additional stamp attributes for their specific needs:

```
my.stamp.department = "engineering"
my.stamp.project = "customer-portal"
my.stamp.cost_center = "R&D"

# All data written by this agent now carries these stamps
my.features.new_login = {...}          # Automatically stamped
world.reports.quarterly = {...}        # Also stamped
```

**Stamp inheritance:** When data is copied or transformed, stamps are inherited unless explicitly modified:

```
# Source data has stamps: {department: "sales", classification: "internal"}
source = world.sales.reports.q4

# Copy inherits all stamps
my.analysis.q4_data = source

# Transform with modified stamps
my.stamp.classification = "public"
my.analysis.public_summary = summarize(source)     # New classification
```

**Stamp queries:** Stamps can be used in bracket expressions to find data by metadata:

```
# All data from engineering department
my.projects[@stamp.department = "engineering"]

# Sensitive data needing attention
world.reports[@stamp.classification = "confidential", @stamp.retention < now()]

# Data from external sources
my.datasets[@stamp.source != "internal"]
```

### Embed

The **embed** meta type marks external data as content rather than metadata. Embedded data preserves its original format and stamps while integrating into the AmorphDB hierarchy.

```
# Embed external JSON data
my.imports.customer_data = embed({
    "customers": [...],
    "format": "json",
    "@source": "external-api"
})

# Embedded data preserves original structure
customer_count = my.imports.customer_data.customers..count
```

Embedded data appears as normal AmorphDB structures but retains stamps indicating its external origin and original format.

### Filters

A **filter** allows an agent to selectively hide data from its own view. Filters operate client-side and only affect what the agent sees — they do not prevent access to data the agent has permissions to read.

**Filter definition:**
```
my.filter.hide_test_data: @stamp.environment = "test"
my.filter.hide_old_logs: @time < (@now - 30 days)
my.filter.production_only: @stamp.classification = "production"
```

**Filter activation:**
```
my.filter = my.filter.production_only           # Apply single filter
my.filter = [                                   # Apply multiple filters
    my.filter.hide_test_data,
    my.filter.production_only
]
```

When a filter is active, any data matching the filter criteria appears as **Redacted** (`***`) in queries and displays. The agent can remove filters to see the full data, but cannot use filters to access data they lack permissions for.

**Use cases:**
- Hide development data in production views
- Focus on specific projects or time periods
- Remove distracting or irrelevant information
- Create role-specific data views

### Permissions

AmorphDB uses a capability-based permission system. Every piece of data has five permission attributes that specify which agents can perform which operations:

| Permission | Meaning |
|------------|---------|
| `@read` | Can read the data and traverse into sub-attributes |
| `@write` | Can modify the data (create new instances) |
| `@expand` | Can create new sub-attributes under this attribute |
| `@grant` | Can modify the permissions on this attribute |
| `@purge` | Can mark this data for deletion |

**Permission values:**
- **Agent reference** (e.g., `""world.agent.kalevo""`) — specific agent
- **Agent list** (e.g., `[""world.agent.alice"", ""world.agent.bob""]`) — multiple agents
- **"Anything"** — unrestricted access
- **"Nothing"** — no access (effectively private)

**Permission examples:**
```
# Private data — only the owner can access
my.secrets.password.@read = ""world.agent.kalevo""
my.secrets.password.@write = ""world.agent.kalevo""

# Team collaboration — multiple agents can read and write
world.projects.alpha.@read = "Anything"
world.projects.alpha.@write = [
    ""world.agent.alice"",
    ""world.agent.bob"",
    ""world.agent.charlie""
]

# Public read, restricted write
world.announcements.@read = "Anything"
world.announcements.@write = ""world.agent.admin""
```

#### Defaults

- Under `~` (an agent's home): all permissions default to owner-only.
- Under `world` root: `@read` defaults to Anything; `@write`, `@expand`, `@grant`, and `@purge` default to Nothing.
- Cascade means permissions are set at key points in the tree and flow downward until overridden.

**Permission inheritance:** Permissions cascade down the hierarchy unless explicitly overridden at a sub-attribute.

#### Interaction with Filters

Permissions and filters are distinct layers. Permissions determine what an agent is *allowed* to access — they are set by the data owner and enforced by the system. Filters determine what an agent *chooses* to see — they are set by the agent and operate within the bounds of permissions. An agent cannot use a filter to see data they lack permission to read, nor can they remove a filter to bypass permissions.


## Mesh Architecture

AmorphDB operates as a decentralized mesh network where every node is a peer. The system supports three distinct operational modes: **standalone nodes**, **unified meshes**, and **bridge connections** between meshes.

### The Virtual Tree

The hierarchy has a single root — `world` — under which all shared data lives. Agent homes live under `world.agent`, keeping them separate from other top-level structures:

    world
    ├── agent
    │   ├── kalevo              # a mobile agent's home
    │   ├── miratu              # another mobile agent's home
    │   └── ...
    ├── clock                   # system clock
    ├── market                  # shared data
    ├── services                # shared services
    └── ...

Each agent's home is at `world.agent.{identity}`, accessible via `~` universally or `my` from program root scope. The `world` root is reachable as `my.world` from any agent's perspective.

### Agent Types and Identity

Every agent in the system has a unique identity generated from alternating consonant-vowel syllables. This produces identifiers that are pronounceable, easy to spell from hearing, easy to remember, and drawn from a vast non-sequential space.

With approximately 100 CV syllables (20 consonants × 5 vowels), three syllables yield one million possible identities; four syllables yield one hundred million. A blacklist filters out unfortunate combinations that spell unintended words.

Examples: kalevo, miratu, senabo, tokeli, dupano

#### Node Agents vs Mobile Agents

All agents have identities and keys, but their relationship to the mesh differs fundamentally:

**Node Agents:**
- Home: Stored locally on the node's hardware only
- Identity/Keys: Stored in local filesystem, not replicated
- Survival: Dies with hardware failure (node IS the hardware)
- Access: Can only be reached through its specific hardware
- Purpose: Provides mesh infrastructure and local services

**Mobile Agents:**
- Home: Stored in mesh, replicated across zones
- Identity/Keys: Stored encrypted in agent's mesh home
- Survival: Persists across hardware failures (data is replicated)
- Access: Can connect through any node in the mesh
- Purpose: Humans, AI programs, external systems, bridge connections

**Mobile Agent Home Structure:**
```
~                               # agent's home (world.agent.{identity})
~.stamp                         # agent's default stamp (persistent)
~.filter                        # agent's active filter (persistent)
~.world                         # the shared world (persistent)
~.computer                      # local machine access (virtual mount)
~.keys.public                   # public key (in mesh)
~.keys.private                  # private key (encrypted with derived key)
```

Most of an agent's home is persistent and stored in the mesh. The `computer` sub-path is a virtual mount provided by the local node the agent connects through.

### Mesh Formation and Management

#### Standalone Mode (Default)

Nodes start in standalone mode with full local functionality but no mesh connectivity:

```bash
amorphd  # Starts standalone
amorphctl status
# Output: Node: quick-moon-7834, Mode: standalone, Mesh: none
```

#### Mesh Creation

To become the founder of a new mesh, a standalone node explicitly creates and names the mesh:

```bash
amorphctl create-mesh "production-alpha"
amorphctl status
# Output: Node: quick-moon-7834, Mode: mesh-founder, Mesh: production-alpha (1 node)
```

**Mesh Name Restrictions:**
- Alphanumeric characters only
- Hyphens and underscores allowed
- No spaces or @ symbols
- Case insensitive

#### Mesh Joining

Standalone nodes can join existing meshes by connecting to any current member:

```bash
amorphctl join node2.company.com:5830
# Handshake discovers mesh name automatically
# Node becomes mesh member with zone assignments
```

**Join Process:**
1. Connect to seed node with post-quantum encryption
2. Discover mesh name during handshake
3. Receive assigned identity in target mesh
4. Calculate position on consistent hash ring
5. Begin zone migration with adjacent nodes
6. Start heartbeat with zone peers and gossip peers

#### Bridge Connections

Nodes can participate in multiple meshes simultaneously through bridge connections. Bridge nodes appear as mobile agents in each mesh they connect to:

```bash
# Already member of production-alpha mesh
amorphctl bridge partner-db.example.com:5830
# Discovers partner mesh name: "partner-net"
# Assigned new identity in partner mesh: "bright-star-1205"
```

**Bridge Architecture:**
```mbl
# Bridge node's local workspace
my.*                            # Node's local storage (node agent)
my.@my_identity = "quick-moon-7834"  # Identity in primary mesh

# Primary mesh access
my.world.*                      # Primary mesh world hierarchy

# Bridge mesh access (appears as mobile agent there)
my.partnernet.*                 # Partner mesh agent home (replicated)
my.partnernet.@my_identity = "bright-star-1205"  # Identity in partner mesh
my.partnernet.world.*           # Partner mesh world hierarchy
my.partnernet.keys.*            # Partner mesh authentication keys
```

**Bridge Identity Management:**
- Each mesh assigns an independent identity to the bridge node
- No @ symbols or identity conflicts
- Bridge node tracks which identity it uses in each mesh
- Each mesh sees bridge as a normal mobile agent

#### Mesh Disconnection

Nodes can leave meshes or disconnect bridges cleanly:

```bash
amorphctl detach              # Leave primary mesh (become standalone)
amorphctl detach partner-net  # Disconnect specific bridge
```

Graceful disconnection includes:
- Announcing departure via gossip protocol
- Migrating owned zones to adjacent nodes
- Clearing local mesh state
- Preserving identity for potential rejoin

### The World Clock

Every node maintains a `world.clock` that provides the current time in UTC. Nodes keep their clocks synchronized via the mesh heartbeat, but each node maintains its own instance — clock updates do not replicate as data changes across the mesh.

`world.clock` carries the current UTC time value along with sub-attributes covering every commonly useful decomposition:

    world.clock:
        utc                 # current UTC time value
        year                # e.g., 2026
        month               # 1-12
        monthname           # e.g., "March"
        day                 # 1-31
        hour                # 0-23
        minute              # 0-59
        second              # 0-59
        weekday             # 1-7 (1=Sunday, 7=Saturday)
        weekdayname         # e.g., "Sunday"
        yearday             # 1-366 (day of year)
        week                # 1-53 (week of year)
        quarter             # 1-4
        unix                # raw UNIX timestamp as number

Time zones are a presentation concern. All data is stored in UTC. Conversion to local time is performed at the point of display.

Because `world.clock` sub-attributes update independently, watchers can target exactly the granularity they need:

    # every hour
    watch hourly_report(world.clock.hour):
        my.reports.generate_hourly

    # first of each month
    watch monthly_close(world.clock.day):
        if world.clock.day ?= 1:
            my.accounting.close_month

    # business hours
    watch office_hours(world.clock.hour):
        if world.clock.hour >= 9 and world.clock.hour < 17:
            my.status = "available"
        else:
            my.status = (quietly) "away"

### Zones

The unit of distribution is a **zone**. A zone is defined by two dimensions: a subtree of the hierarchy and an optional temporal range.

**Spatial zones** encompass a subtree and everything beneath it:
- `world.market` zone contains all market data
- `world.agent.kalevo` zone contains agent kalevo's home
- `world.services.auth` zone contains authentication service data

**Temporal zones** split historical data by time range:
- `world.market:current` contains recent market data
- `world.market:2020-2023` contains historical data from that period
- Old instances automatically link to archival zones via `@older_instance_id`

Each zone has one **authority node** responsible for writes and two **replica nodes** for redundancy. Zone assignments are calculated using consistent hashing:

    hash("world.market:current")        → node_B (authority), node_C, node_A (replicas)
    hash("world.market:2020-2023")      → node_F (authority), node_A, node_B (replicas)

This is deterministic — any node can calculate where a zone lives by hashing the path. No central directory is required for routing, though nodes cache routing information for performance.

When a node joins or leaves the mesh, only zones adjacent on the hash ring need to migrate. The consistent hashing algorithm minimizes data movement during topology changes.

### Zone Splitting

Each zone authority monitors its own load — storage size, query rate, write rate. When thresholds are exceeded, the authority autonomously decides to split. No central coordinator is involved.

**Spatial splitting:** A zone with many sub-attributes splits into smaller subtrees. A zone rooted at `world.agent` might split by key distribution of its children. The tree structure provides natural split points.

**Temporal splitting:** A zone with deep history splits by time range. Recent instances stay on fast-access nodes, historical instances migrate to archival nodes. The `older_instance_id` pointer at the time boundary becomes a redirect to the archival node.

After a split, the new zones are hashed onto the ring independently. They may land on different nodes, automatically distributing the load that caused the split.

### Distributed Computation

Watchers and procedures execute on the node that owns the zone where they live. Computation moves to the data, not the other way around. As zones split and migrate, the code moves with the data.

A chain of watchers across different zones becomes a distributed pipeline, each stage executing on the node closest to its data:

    # watcher on the node owning world.market.raw
    watch ingest(world.market.raw):
        world.market.processed = transform(world.market.raw)

    # watcher on the node owning world.market.processed
    watch analyze(world.market.processed):
        world.reports.daily = analyze(world.market.processed)

    # watcher on the node owning world.reports
    watch distribute(world.reports.daily):
        world.notifications.send_report(world.reports.daily)

Each heartbeat propagates changes through the pipeline. The mesh is the execution engine.

### Locality

Most operations cluster in specific subtrees. A user predominantly accesses `my.*`, a department works within `world.departments.{dept}.*`. Consistent hashing keeps related data together — the zone for a subtree encompasses everything beneath it until a split occurs.

Some subtrees are broadly shared (configuration, shared libraries, common reference data). These require replication across multiple nodes rather than single-node ownership.

### Bridge Communication and Routing

Bridge nodes maintain separate network connections to each mesh they participate in. Each mesh sees the bridge as a normal mobile agent with its own assigned identity.

**Cross-Mesh Data Flow:**
```mbl
# Bridge node can move data between meshes
my.world.shared_reports = filter(my.partnernet.world.data, public_only)

# Or run automated synchronization
watch sync_public_data():
    for item in my.partnernet.world.products:
        if item.public_release:
            my.world.products[item.sku] = sanitize(item)
```

**Security Isolation:**
- Each mesh has independent security boundaries
- Bridge node's identity in one mesh doesn't reveal identity in another
- Data transfer is explicit and programmatic (no automatic federation)
- Permissions and encryption apply independently in each mesh


## Components

AmorphDB consists of three main components:

| Component | Purpose |
|-----------|---------|
| **Service** (`amorphd`) | Core database daemon. Handles storage, mesh networking, and MBL execution. |
| **Control** (`amorphctl`) | Admin tool. Connects via local socket. Start, stop, status, mesh management, compaction. |
| **Client** (`amorph`) | User tool. Connects via local socket or network socket. Interactive REPL or script execution. |

The **service** is the only component that touches disk storage directly. All other components communicate with it through sockets. The local socket serves the control tool, local clients, and local resources such as devices. The network socket serves remote clients and mesh traffic between nodes. The protocol is the same over both, with encryption and authentication layered on for network connections (see Security).

A remote client connects to any node. If the requested data lives in a different node's zone, the connected node proxies the request transparently. The client does not need to know the mesh topology.

### Mesh Management Commands

The control tool provides explicit mesh lifecycle management:

**Mesh Creation:**
```bash
amorphctl create-mesh "production-alpha"  # Start new mesh
amorphctl status                          # View current state
```

**Mesh Joining:**
```bash
amorphctl join node2.company.com:5830     # Join existing mesh
amorphctl status                          # Shows mesh membership
```

**Bridge Creation:**
```bash
amorphctl bridge partner-db.example.com:5830  # Bridge to another mesh
amorphctl status                               # Shows all mesh connections
```

**Disconnection:**
```bash
amorphctl detach                # Leave primary mesh (become standalone)
amorphctl detach partner-net    # Disconnect specific bridge
```

### The Client

The client (`amorph`) is the primary interface for humans and external programs to interact with AmorphDB. It supports two modes of operation.

**Interactive mode** provides a REPL (Read-Eval-Print Loop) for ad hoc queries, data exploration, and immediate operations:

    $ amorph
    AmorphDB> my.account.balance
    $1,247.83

    AmorphDB> my.tasks[priority = "high"]
    [
        {subject: "Client presentation", due: @2026-01-20},
        {subject: "Code review", due: @2026-01-18}
    ]

    AmorphDB> world.market.price.@time
    @2026-01-15 14:32:18

**Script mode** executes MBL programs from files:

    $ amorph script.mbl
    $ amorph --input data.csv --script import.mbl

The client automatically handles:
- Authentication with the local or remote node
- Session management and reconnection
- Multi-line input for complex expressions
- History and readline support
- Output formatting and display

Connection options:

    $ amorph                               # connect to local node (default)
    $ amorph --node 192.168.1.50:5000          # connect to remote node
    $ amorph --identity kalevo                  # authenticate as specific agent

By default, the client connects to the local node's socket. For remote connections, an address is specified and authentication proceeds via challenge-response (see Security). If no identity is specified, the client uses the identity stored in the local configuration.


## Security

### Node Bootstrap

A new node joins the mesh by connecting to at least one known node address.

1. The new node connects to the seed node and performs a Diffie-Hellman key exchange, establishing an ephemeral secure channel.
2. The channel upgrades to post-quantum encryption, protecting the exchange against future quantum attacks.
3. Over this secure channel, the new node requests an identity.
4. The seed node generates a CV syllable identity and a key pair for the new node.
5. The new node stores its identity and keys locally (not in the mesh).
6. The seed node introduces the new node to other nodes it knows about.

The very first node in a mesh self-generates its identity and keys. There is no external authority.

**Bridge Bootstrap:** When a node bridges to a second mesh, it follows the same process but receives a separate identity in the target mesh. The bridge node stores both identities locally and uses the appropriate one for each mesh connection.

### Node-to-Node Encryption

Every pair of nodes that communicates establishes its own encrypted channel. Keys are exchanged lazily — two nodes that have never communicated perform a key exchange the first time they need to, then cache the result. Over time, each node accumulates keys for the peers it actually talks to.

This means each node only holds keys for nodes it has communicated with, and each channel can rotate keys independently.

### Mobile Agent Authentication

Mobile agents (humans, AIs, programs) authenticate using two-factor derived keys. The agent's private key is not stored as a static artifact anywhere — it is derived at authentication time from multiple factors:

- Something the agent knows (a passphrase)
- Something the agent has (a device secret)

The derived key is used for a challenge-response protocol with the node. The node validates the response against the agent's public key stored in the mesh.

**Advantages:**
- No static private keys to compromise
- Device binding prevents password-only attacks
- Forward secrecy if device secrets are rotated
- Works across any node in the mesh

**Bridge Authentication:** When a bridge node acts as a mobile agent in a partner mesh, it uses the same two-factor authentication system with keys specific to that mesh's assigned identity.

### Agent-Level Encryption

An agent's secrets — private keys, sensitive data — are stored in the mesh but encrypted with the agent's own derived key. The node hosting the agent's zone stores this data but cannot read it. Only the authenticated agent can decrypt it.

    ~.keys.public                   # in the mesh, readable by anyone
    ~.keys.private                  # in the mesh, encrypted with agent's derived key

This provides end-to-end encryption: data travels encrypted over the mesh and remains encrypted at rest until the authenticated agent decrypts it.

### Recovery

If an agent loses their device secret, recovery requires:

1. **Identity proof** through an alternative method (backup device, trusted contacts, etc.)
2. **Re-derivation** of keys using the new device secret
3. **Re-encryption** of stored data with the new derived key

The system supports multiple device secrets per agent, allowing for backup devices and key rotation without data loss.

**Bridge Recovery:** Bridge nodes must maintain device secrets for each mesh they participate in. Loss of bridge credentials in one mesh doesn't affect operation in other meshes.

### Protocol

AmorphDB uses a custom protocol over TCP with the following layers:

1. **Transport Security** - Post-quantum encryption for all network communication
2. **Authentication** - Challenge-response for agent identity verification
3. **Message Protocol** - Request/response for queries and updates
4. **Heartbeat Protocol** - Mesh synchronization and failure detection

The same protocol operates over both local Unix sockets (no encryption) and network TCP sockets (full encryption stack).

**Mesh Communication:**
Each node heartbeats to a small set of direct peers — zone cluster peers (the authority and replicas for zones this node participates in) and a handful of gossip peers for broader mesh awareness. This keeps per-node connection count bounded regardless of mesh size.

**Zone cluster heartbeats** are direct and immediate — writes replicate to replicas within one heartbeat tick. This is the hot path for data consistency.

**Gossip heartbeats** propagate mesh-wide information (new nodes, departed nodes, zone splits) through neighbors. This information may take several ticks to reach the entire mesh, which is acceptable for metadata that changes infrequently.

#### Routing

Any node can calculate which node owns a zone by hashing the path onto the consistent hash ring. When a node receives a request for data it does not own, it forwards the request to the calculated authority. The response returns along the same path. The client does not need to know the mesh topology.

For read requests, the node may route to the nearest replica rather than the authority, reducing latency and distributing read load.

**Bridge Routing:** Bridge nodes maintain separate routing tables for each mesh they participate in. Requests are routed within the appropriate mesh based on the data path being accessed.

#### Extensibility

The protocol is designed to support future extensions:
- New message types can be added without breaking existing nodes
- Version negotiation allows mixed-version meshes during upgrades
- Optional features can be negotiated per connection
- Bridge protocol extensions support multi-mesh scenarios

The mesh is designed to be long-lived and evolvable. Nodes can join and leave freely, and the system adapts to changing topology and requirements without central coordination.