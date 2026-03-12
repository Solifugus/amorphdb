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
  A number in a global currency with automatic conversion.
  In MBL, money literals begin with the currency character (¤). They resolve against exchange rates at write time.
  Examples:
    - ¤19.99 (in the agent's default currency)
    - ¤USD19.99 (explicitly USD)
    - ¤EUR15.00 (explicitly EUR)
- Picture
  RGBA 16-bit depth (65,536 levels per channel).
  Regardless of source format, pictures are always normalized to 16-bit RGBA upon assignment to allow uniform arithmetic. The type system handles conversion from standard formats (PNG, JPEG, etc.).
  Examples:
    - img = ~/computer/desktop/photos/vacation.jpg
    - blank = Picture(640, 480)
    - pixel_color = img[320, 240]
- Reference
  A path to another attribute in the tree — not the value, but the place.
  In MBL, the `&` prefix on a path creates a reference instead of accessing the value.
  References can point across the mesh — they are not bound to local storage.
  Examples:
    - my_ref = &world.market.stocks.AAPL.price
    - owner = &~.world.properties[address = "123 Main St"].owner
- Procedure
  A sequence of statements that can be invoked with parameters.
  Procedures may return a value or operate through side effects. They are stored like any other value and inherit from the location where they are defined.
- Watcher
  A procedure that executes automatically when specified attributes change.
  Watchers are the reactive programming mechanism — they turn the database into a live computational environment.
- Embed
  A record whose attributes appear in the current namespace.
  The attributes of an embedded record become attributes of the embedding instance.
  Unlike references, the embedded data exists at write time — subsequent changes to the source do not propagate.

#### Time and Money

Time and money handle representation complexity automatically.

Time calculations maintain appropriate precision — adding `@03:00:00` to `@2026-02-01` yields `@2026-02-01 03:00:00`, preserving the precision of both operands.

Money tracks currency and handles conversion. When different currencies are combined, the operation uses exchange rates at evaluation time. Storage maintains the original currency until explicitly converted.

    price_usd = ¤USD19.99
    price_eur = ¤EUR15.00
    total = price_usd + price_eur           # converts at current exchange rate
    total..currency                         # shows the currency of the result
    total..convert("USD")                   # explicit conversion to USD

### Instance Metadata

Every instance carries meta attributes accessible via `@name` notation. The `@` prefix distinguishes meta attributes from the instance's own attributes.

| Meta attribute | Type | Meaning |
|----------------|------|---------|
| `@timestamp` | Time | When the instance was written |
| `@author` | Reference | The agent that wrote the instance |
| `@embed` | Embed | Stamp attributes embedded in the instance |
| `@host` | Text | The node that initially stored the instance |
| `@zone` | Text | The zone the instance belongs to |
| `@previous` | Reference | The previous instance of the same attribute |
| `@next` | Reference | The next instance of the same attribute |
| `@checksum` | Text | Cryptographic hash of the instance content |

Instance metadata is immutable once written. It provides audit trails, temporal navigation, and integrity verification without requiring additional storage structures.

### Indexing

Attributes can be indexed to support efficient queries. MBL provides two indexing strategies:

#### Numerical Indexing

Numerical indexes treat the attribute as an ordered list. New values are appended with the next sequential index.

    my.events..append("first event")       # creates my.events[0]
    my.events..append("second event")      # creates my.events[1]
    my.events[0]                           # returns "first event"
    my.events[1]                           # returns "second event"

Numerical indexes support range queries:

    my.events[0:2]                         # returns events 0 and 1
    my.events[1:]                          # returns events from 1 to the end
    my.events[:3]                          # returns events 0, 1, and 2

#### Textual Indexing

Textual indexes treat the attribute as a key-value map. Values are accessed by their text key.

    my.config.database_host = "localhost"      # creates my.config["database_host"]
    my.config.database_port = 5432              # creates my.config["database_port"]
    my.config["database_host"]                  # returns "localhost"
    my.config[database_host = "localhost"]     # query: returns the instance

Textual indexes support pattern matching and conditional queries:

    my.users[name = "Alice"]                # exact match
    my.users[name ~ "Ali*"]                 # pattern match
    my.users[age > 25, status = "active"]   # compound conditions

#### Dynamic Indexing

An attribute can switch between numerical and textual indexing as data patterns change. The system automatically maintains appropriate indexes based on usage.

### Heritability

When a record serves as a source for instantiation (via the `new` keyword), heritability modifiers determine how each attribute is transferred to the new instance. Modifiers are specified in the source record's attribute definitions.

#### Modifiers

| Modifier | Behavior |
|----------|----------|
| `(copy)` | Value is duplicated; subsequent changes are independent |
| `(link)` | Value remains linked; changes propagate to the instantiated record |
| `(reset)` | Value is computed fresh during instantiation |
| `(exclude)` | Attribute is not transferred to the instantiated record |

If no modifier is specified, `(copy)` is the default behavior.

Example:

    a:
      id:(reset random(100, 999)) 342
      name: "Joe"
      secret:(exclude)
      status:(link) "Good"

    b = new a   # b gets { id: 451, name: "Joe", status: → a.status }
                # secret is excluded, id is freshly generated, status is linked

#### Instantiation

The `new` keyword creates a new record by applying heritability rules from one or more source records. Any record can serve as a source — there is no special template or class type.

    instance = new person

Multiple sources may be listed. They are applied left to right — each successive source overlays the previous, with later sources winning on attribute collision. Heritability modifiers from the last source to define an attribute take effect. Sources may be named records or inline literal records in any position.

    person:
        name: "unnamed"
        age: 0

    employee:
        department: "unassigned"
        role: "staff"

    bob = new person employee { name: "Bob", department: "IT" }

    linked = new person { status:(link) "Active" } employee

Instantiation is recursive. When a source contains sub-records, those sub-records are also instantiated with their heritability rules applied at each level.

A record created with `new` is itself a record and can serve as a source for further instantiation:

    bob = new person employee { name: "Bob" }
    bob_clone = new bob { name: "Bob Jr." }

#### Additional Modifiers

| Modifier | Context | Behavior |
|----------|---------|----------|
| `(protected)` | Record attribute definition | Attribute cannot be overridden by embedded stamp attributes |
| `(cascade)` | Persistent scope | Attribute value is visible to bare references from descendant scopes |
| `(quietly)` | Assignment | Value is written without triggering watchers |


## MBL (Modern Business Language)

### Lexical Structure

#### Indentation

MBL uses indentation to define block structure, similar to Python. Only tabs are permitted for indentation, and they must be leading characters at the start of a line. Spaces in indentation are a syntax error. Within a line, spacing is free-form.

#### Statements

Each line is a statement. Multiple statements may appear on a single line separated by `;`. Expressions within bracket `[]`, brace `{}`, or parenthesis `()` blocks may span multiple lines freely regardless of indentation.

#### Comments

Comments use the `#` symbol. The number of adjacent `#` symbols determines how the comment closes.

A single `#` opens a comment that closes at the next `#` or end of line, whichever comes first. This enables both full-line and sub-line comments:

    x = 5 # set x # + 3          # sub-line: + 3 is live code
    x = 5 # to end of line

Multiple adjacent `#` symbols open a comment that closes only at the next occurrence of the same number of adjacent `#` symbols. These comments may span multiple lines:

    ## This is a block comment
    that spans multiple lines
    and can contain # single hashes # without closing ##

    ### This can contain ## double hashes ## as well ###

The matching-delimiter rule means longer comments can always wrap shorter ones by using more `#` symbols.

#### String Literals

String literals use the same matching-delimiter pattern. One or more adjacent `"` characters open a string, and the same number of adjacent `"` characters close it. Newlines are permitted within string literals. Only visible characters (including whitespace and newlines) are allowed — no escape sequences.

    "Hello World"
    ""a string with "quotes" inside""
    """a string with "" and "quotes" of any kind
    and multiple lines"""

This eliminates the need for backslash escapes entirely.

### Operators and Expressions

#### Comparison

    x ?= y          # equal
    x != y          # not equal
    x > y           # greater than
    x < y           # less than
    x >= y          # greater than or equal
    x <= y          # less than or equal

Equality (`?=`) uses lowest-common-denominator matching for time values. When two time values have different precision, the less precise value represents a range. The comparison rules are:

- `?=` — the more precise value falls within the range of the less precise one
- `>` — the more precise value is after the last moment of the less precise one
- `<` — the more precise value is before the first moment of the less precise one
- `>=` and `<=` — combine equality and inequality accordingly

When both values have the same precision, comparison is straightforward.

    @2026-02-01 ?= @2026-02-01 15:30:00     # true: 15:30 falls within Feb 1st
    @2026-02-01 ?= @2026-02-03              # false: Feb 3rd is not within Feb 1st
    @2026-02 ?= @2026-02-15                  # true: Feb 15th falls within February
    @2026-01 < @2026-02-15                   # true: Feb 15th is after the last moment of January
    @2026-02-01 > @2026-01-15 08:00:00      # false: 8am Jan 15th is not after the last moment of Feb 1st

This avoids the common trap of defaulting omitted parts to midnight, which produces surprising results with inequalities. Partial time literals behave the way humans think about them — "February 1st" means the whole day, not midnight.

#### Logical

    x and y
    x or y
    not x

#### Arithmetic

    x + y           # add
    x - y           # subtract
    x * y           # multiply
    x / y           # divide
    x % y           # modulo

#### Concatenation

    x & y           # concatenate as text

Every type has a text representation. Concatenation should effectively never fail.

#### Assignment

    x = y           # assign a value (record a change)
    x: y            # assign a definition

The `=` symbol serves as both assignment (in statements) and equality comparison (`?=`). There is no ambiguity — `?=` is always comparison, `=` is always assignment.

### Type Coercion

Coercion is determined by the operator, not the operands. The operator declares intent, and operands coerce to fit.

#### Arithmetic coercion (`+`, `-`, `*`, `/`, `%`)

| Expression | Result |
|------------|--------|
| `1 + "2"` | 3 (text parsed as number) |
| `"1" + 2` | 3 (text parsed as number) |
| `¤19.95 + ¤5.00` | ¤24.95 |
| `@2026-02-01 + @03:00:00` | @2026-02-01 03:00:00 |
| `pic * 0.5` | all channels scaled (darken) |
| `picA + picB` | per-pixel channel addition, clamped to max |

#### Concatenation coercion (`&`)

| Expression | Result |
|------------|--------|
| `"hello" & " " & "world"` | "hello world" |
| `"Date: " & @2026-02-01` | "Date: 2026-02-01" |
| `"Total: " & ¤24.95` | "Total: $24.95" |
| `"Value: " & Unknown` | "Value: Unknown" |
| `"Empty: " & empty` | "Empty: " |
| `"None: " & Nothing` | "None: Nothing" |

#### Picture arithmetic

Pictures are always normalized RGBA 16-bit, so arithmetic operations are per-pixel math on identically structured data. When dimensions differ, the result is the size of the larger picture; the smaller one is aligned top-left and treated as transparent beyond its bounds.

| Expression | Effect |
|------------|--------|
| `picA + picB` | additive blend (channel values added, clamped to max) |
| `picA - picB` | difference (channel values subtracted, clamped to zero) |
| `picA * picB` | multiply blend (channels multiplied, normalized) |
| `pic * 0.5` | scale all channels (darken) |
| `pic + 1000` | add to all channels (brighten) |

#### Resilience

MBL is designed for continuous operation. When an operation cannot be reasonably interpreted, it produces an Unknown value with a reason rather than halting execution.

    x = 1 / 0                  # x is Unknown (reason: "division by zero")
    x = "hello" + picture      # x is Unknown (reason: "cannot add Text to Picture")

Unknown values propagate through subsequent operations. Code that does not check for them continues running. Watchers can monitor for Unknowns and perform self-healing or alerting when correctness matters.

### System Operations

System operations are accessed with the `..` syntax — a double dot between a value and the operation name. This distinguishes system operations from attribute access (`.`) and prevents name collisions with user-defined attributes.

Text and number operations return new values — they never modify the original. List and record operations modify in place, reflecting the structural nature of the underlying data.

Operations may be chained left-to-right, forming a natural pipeline:

    result = my.data..sort..reverse..first(5)

#### Text Operations

| Operation | Effect |
|-----------|--------|
| `text..length` | Character count (UTF-8 aware) |
| `text..upper` | Uppercase version |
| `text..lower` | Lowercase version |
| `text..trim` | Remove leading/trailing whitespace |
| `text..split(delimiter)` | Array of substrings |
| `text..replace(old, new)` | Replace occurrences |
| `text..contains(substring)` | True if substring is present |
| `text..starts(prefix)` | True if text begins with prefix |
| `text..ends(suffix)` | True if text ends with suffix |

#### Number Operations

| Operation | Effect |
|-----------|--------|
| `num..abs` | Absolute value |
| `num..floor` | Round down to integer |
| `num..ceil` | Round up to integer |
| `num..round` | Round to nearest integer |
| `num..round(decimals)` | Round to specified decimal places |

#### Picture Operations

| Operation | Effect |
|-----------|--------|
| `pic..width` | Width in pixels |
| `pic..height` | Height in pixels |
| `pic..resize(width, height)` | Scaled version |
| `pic..crop(x, y, width, height)` | Cropped region |
| `pic..rotate(degrees)` | Rotated version |
| `pic..blur(radius)` | Blurred version |
| `pic..grayscale` | Grayscale version |

#### Time Operations

| Operation | Effect |
|-----------|--------|
| `time..year` | Year component |
| `time..month` | Month component (1-12) |
| `time..day` | Day component (1-31) |
| `time..hour` | Hour component (0-23) |
| `time..minute` | Minute component (0-59) |
| `time..second` | Second component (0-59) |
| `time..weekday` | Day of week (1=Sunday, 7=Saturday) |
| `time..format(template)` | Formatted string representation |

#### Collection Operations

| Operation | Effect |
|-----------|--------|
| `collection..length` | Count of elements |
| `collection..empty` | True if no elements |
| `collection..append(value)` | Add value to end |
| `collection..prepend(value)` | Add value to beginning |
| `collection..sort` | Sorted version |
| `collection..reverse` | Reversed version |
| `collection..first(n)` | First n elements |
| `collection..last(n)` | Last n elements |
| `collection..contains(value)` | True if value is present |

### Variables and Scope

Variables exist in three scopes:

1. **Local** — Function or procedure parameters and temporary variables
2. **Session** — Variables that persist for the duration of an agent's connection
3. **Persistent** — Data stored permanently in the mesh

#### Local Variables

Local variables are declared implicitly by assignment within procedures. They exist only during procedure execution.

    procedure calculate(x, y):
        temp = x * 2        # local variable
        result = temp + y   # local variable
        return result

#### Session Variables

Session variables persist across procedure calls within a single agent session but are lost when the session ends. They are useful for maintaining state during interactive work.

    session.current_project = "AmorphDB"
    session.debug_mode = true

#### Persistent Variables

Persistent variables are stored in the mesh and survive across sessions. All paths beginning with `my` or `world` are persistent.

    my.settings.theme = "dark"          # persisted to your home
    world.shared.config = "production"   # persisted to shared space

#### Scope Resolution

The interpreter resolves unqualified names by searching in order:

1. Local scope (procedure parameters and locals)
2. Session scope
3. Persistent scope under `my`

This means local variables shadow session variables, which shadow persistent variables.

### Procedures

Procedures are named sequences of statements that accept parameters and may return values. They are stored as values like any other data and can be passed as parameters or stored in collections.

    procedure greet(name):
        return "Hello, " & name & "!"

    procedure calculate_tax(amount, rate):
        if rate > 1:
            rate = rate / 100       # convert percentage to decimal
        return amount * rate

#### Procedure Storage

Procedures can be stored anywhere in the hierarchy and called by reference:

    my.utils.tax_calculator = procedure(amount, rate):
        return amount * (rate / 100)

    my.utils.validator = procedure(data):
        if data..empty:
            return Unknown("empty data")
        return data

#### Return Values

Procedures may return explicit values or operate through side effects. If no explicit `return` is specified, the procedure returns the value of its last expression.

    procedure log_and_return(message):
        my.logs..append(@now & ": " & message)
        message     # implicit return

#### Error Handling

Procedures handle errors by returning Unknown values rather than throwing exceptions. This maintains the principle of continuous operation.

    procedure safe_divide(a, b):
        if b ?= 0:
            return Unknown("division by zero")
        return a / b

### Watchers

Watchers are procedures that execute automatically when specified attributes change. They form the reactive programming foundation of AmorphDB.

    watch price_monitor(world.market.stocks.AAPL.price):
        if world.market.stocks.AAPL.price > 200:
            my.alerts..append("AAPL exceeded $200")

#### Watcher Syntax

A watcher declaration includes:

1. The `watch` keyword
2. A name for the watcher
3. Parentheses containing the attribute(s) to monitor
4. The procedure body

The monitored attribute(s) may include wildcards or conditional expressions:

    watch user_activity(world.users.*.last_login):
        # triggers when any user's last_login changes

    watch threshold_breach(my.metrics[value > 100]):
        # triggers when any metric exceeds 100

#### Multiple Triggers

Watchers can monitor multiple attributes:

    watch correlation_analysis(world.market.stocks.AAPL.price, world.market.stocks.GOOGL.price):
        correlation = calculate_correlation(AAPL.price, GOOGL.price)
        my.analysis.correlation = correlation

#### Watcher Lifecycle

Watchers run during mesh heartbeat cycles. When an attribute changes, affected watchers are queued for execution in the next heartbeat. This ensures watchers see a consistent view of data and prevents infinite trigger loops.

Watchers execute on the node that owns the zone where they are stored. As zones split and migrate, watchers move with the data they monitor.

### Control Flow

#### Conditional Statements

    if condition:
        # statements
    elif other_condition:
        # statements
    else:
        # statements

#### Loops

    # For loop with range
    for i in 1..10:
        my.numbers[i] = i * i

    # For loop with collection
    for item in my.collection:
        process(item)

    # While loop
    while condition:
        # statements

#### Break and Continue

    for i in 1..100:
        if i % 15 ?= 0:
            continue        # skip multiples of 15
        if i > 50:
            break           # stop after 50

### Built-in Functions

| Function | Purpose |
|----------|---------|
| `output(value)` | Print value to current output stream |
| `input(prompt)` | Read value from current input stream |
| `random(min, max)` | Random number in range |
| `now()` | Current timestamp |
| `uuid()` | Generate unique identifier |
| `hash(value)` | Cryptographic hash of value |
| `encrypt(data, key)` | Encrypt data |
| `decrypt(data, key)` | Decrypt data |
| `type(value)` | Type of value |
| `length(collection)` | Count elements |
| `keys(record)` | Attribute names |
| `values(record)` | Attribute values |

### Import System

MBL supports importing procedures and definitions from other parts of the hierarchy:

    import world.shared.math as math
    import world.shared.utils.*

    result = math.factorial(5)

Imported names are resolved at reference time — they create links to the original definitions rather than copying them. This means updates to shared libraries propagate automatically to all users.


## Storage Engine

The AmorphDB storage engine is designed for append-only temporal data with efficient read access. Every write creates a new instance rather than overwriting existing data. The storage model maintains complete history while optimizing for both current and historical queries.

### File Structure

Each zone's storage consists of four coordinated files:

- **Values file** (`values.dat`) — actual data content
- **Instances file** (`instances.dat`) — timestamped pointers to values with metadata
- **Attributes file** (`attributes.dat`) — hierarchy structure with instance chains
- **Hash index** (`hash.idx`) — fast path lookup for current values

This separation allows different caching and archiving strategies for each data type. Values can be compressed and moved to slower storage while instances and attributes remain accessible on fast storage.

### Values Storage

The values file stores the actual content of all data types using a bucketed approach:

#### Bucket Strategy

| Size Range | Bucket | Strategy |
|------------|--------|----------|
| 0-64 bytes | Small | Inline storage, no compression |
| 65-4KB | Medium | LZ4 compression |
| 4KB-1MB | Large | ZSTD compression |
| 1MB+ | Huge | ZSTD + external file reference |

This bucketed approach optimizes for different data patterns — small values benefit from immediate access, while large values benefit from aggressive compression.

#### Value Addressing

Each value is addressed by a 64-bit offset in the values file. Values are written sequentially with no gaps. When a value is deleted (through compaction), the space is marked as free in a freelist but not immediately reclaimed.

#### Deduplication

Identical values are stored only once. The system computes a SHA-256 hash of each value and maintains a hash-to-offset mapping. When the same value is written again, it references the existing storage.

This is particularly effective for template-based records where many instances share common sub-values.

### Instance Storage

Instances are fixed-size records that link values to timestamps and metadata:

```
Instance Record (64 bytes):
- Value offset (8 bytes)
- Timestamp (8 bytes)
- Author ID (8 bytes)
- Checksum (32 bytes)
- Previous instance offset (8 bytes)
```

#### Instance Chaining

Instances for the same attribute are linked in a doubly-linked list ordered by timestamp. The attribute record points to the most recent instance, and each instance points to its predecessor. This enables efficient temporal navigation in either direction.

#### Integrity Verification

Each instance includes a SHA-256 checksum covering:
- The value content
- The timestamp
- The author ID
- The previous instance checksum (forming a hash chain)

This provides cryptographic verification that the data has not been modified and establishes a tamper-evident audit trail.

### Attribute Hierarchy

The attributes file maintains the hierarchical structure of the data. Each attribute record contains:

```
Attribute Record (variable size):
- Name hash (8 bytes)
- Name length and content (variable)
- Current instance offset (8 bytes)
- Child count (4 bytes)
- Child attribute offsets (variable)
- Indexing strategy (1 byte)
- Metadata (variable)
```

#### Hierarchical Navigation

The attribute structure forms a tree where each node knows its children. This enables efficient traversal of the hierarchy without scanning the entire file. Path resolution is O(log n) in the depth of the hierarchy.

#### Index Strategy

Each attribute declares its indexing strategy:
- **Temporal only** — accessible by timestamp
- **Numerical** — ordered list with integer indices
- **Textual** — key-value map with string keys
- **Hybrid** — supports both numerical and textual access

The chosen strategy affects how child attributes are organized and queried.

### Hash Index

The hash index provides O(1) lookup for current attribute values without scanning the entire hierarchy. It maps path hashes directly to the current instance offset.

#### Index Structure

The index uses a consistent hash table with linear probing:

```
Index Entry (32 bytes):
- Path hash (8 bytes)
- Instance offset (8 bytes)
- Instance timestamp (8 bytes)
- Next entry offset (8 bytes) — for collision handling
```

#### Update Strategy

The index is updated asynchronously during heartbeat cycles. This means the index might temporarily lag behind recent writes, but temporal queries against the attribute files are always consistent.

### Defragmentation

The storage engine performs background defragmentation to reclaim space from deleted values and optimize layout for access patterns.

#### Compaction Process

1. **Analysis phase** — identify fragmented regions and access patterns
2. **Planning phase** — determine optimal value relocation strategy
3. **Migration phase** — move values to new locations and update references
4. **Verification phase** — validate that all references are correct
5. **Commit phase** — atomically switch to the new layout

During defragmentation, the zone enters maintenance mode. Read operations continue using the existing layout while write operations are queued. Once migration completes, queued writes are applied to the new layout.

#### Hot/Cold Separation

The defragmentation process separates frequently accessed (hot) data from rarely accessed (cold) data. Hot data is placed in the beginning of the values file for better cache performance. Cold data can be moved to slower storage or more aggressively compressed.

### Caching Strategy

The storage engine employs a multi-level caching strategy optimized for temporal access patterns:

#### Instance Cache

Recently accessed instances are cached in memory with LRU eviction. This cache is particularly effective for current values and recent history, which see the majority of queries.

#### Value Cache

The value cache stores decompressed content for recently accessed values. Large values (>4KB) are cached in compressed form and decompressed on demand.

#### Path Cache

Resolved paths are cached to avoid repeated hierarchy traversal. This cache includes both successful path resolutions and negative lookups (paths that don't exist).

#### Hash Index Cache

The most frequently accessed portions of the hash index are kept in memory. Given that most queries target current values, a relatively small cache provides high hit rates.

### Consistency Guarantees

#### Write Ordering

All writes within a zone are strictly ordered by timestamp. Concurrent writes to the same zone are serialized at the zone authority. This ensures that the temporal sequence reflects causality.

#### Read Consistency

Reads always see a consistent snapshot of the data. Temporal queries see exactly the state of data as it existed at the specified time, even if newer writes have occurred since then.

#### Cross-Zone Consistency

Writes that span multiple zones use a two-phase commit protocol to ensure atomicity. The coordinator zone prepares all participant zones before committing the transaction.

### Backup and Recovery

#### Continuous Backup

The storage engine supports continuous backup through file-level streaming. Since storage is append-only, backup can stream new writes without interrupting operations.

#### Point-in-Time Recovery

Recovery can target any timestamp in the stored history. The system reconstructs the exact state of data as it existed at the recovery point, including partial transactions that were in progress.

#### Integrity Verification

The backup process verifies integrity of all data during streaming. Any corruption detected during backup triggers automatic repair from replicas before the corrupted data propagates to backup storage.


## Distributed Architecture

AmorphDB is fundamentally a mesh architecture — a collection of peer nodes that collectively maintain a unified global database. There is no central coordinator, no primary/secondary relationship, and no single point of failure.

### Mesh Topology

Each node in the mesh knows about every other node, but this knowledge is maintained through gossip protocols rather than centralized registration. Nodes discover each other through:

1. **Bootstrap** — initial connection to any existing mesh member
2. **Gossip** — periodic exchange of topology information
3. **Referral** — learned about through other nodes' gossip

#### Node Identity

Each node has a cryptographic identity consisting of:
- **Public key** — used for mesh authentication and encrypted communication
- **Node ID** — derived from the public key for consistent identification
- **Capabilities** — storage capacity, computational power, network bandwidth
- **Zone assignments** — which portions of the global tree this node manages

#### Heartbeat Protocol

Nodes exchange heartbeat messages every 5 seconds containing:
- Node health status and resource utilization
- Zone assignments and replica status
- Recently received write operations for cross-zone coordination
- Clock synchronization information

The heartbeat serves multiple purposes:
- **Failure detection** — nodes that miss several heartbeats are considered offline
- **Write coordination** — ensures eventual consistency across zones
- **Load balancing** — provides data for zone splitting decisions
- **Time synchronization** — maintains consistent timestamps across the mesh

### Zone Distribution

The unit of distribution is a **zone** — a subtree of the global hierarchy assigned to a specific node for write authority. Zones are assigned through consistent hashing, which provides automatic load balancing and minimizes data movement when nodes join or leave.

#### Consistent Hashing

The hash ring maps zone identifiers to nodes in a deterministic way:

```
Zone ID: hash(subtree_root + temporal_range) → Ring position → Node assignment
```

Examples:
- `hash("world.agent.kalevo")` → Position 0x1A2B → Node C
- `hash("world.market:2020-2023")` → Position 0x7F81 → Node A
- `hash("world.market:current")` → Position 0xF234 → Node B

#### Replication Strategy

Each zone is replicated to N subsequent nodes on the hash ring (typically N=2 for 3-way replication). The first node is the **authority** for writes, while subsequent nodes are **replicas** for reads and failover.

Replicas maintain synchronized copies through the heartbeat protocol. When the authority receives a write, it forwards the operation to replicas during the next heartbeat cycle.

#### Zone Splitting

Zones autonomously split when they exceed capacity thresholds:

**Spatial splits** divide large subtrees:
- A zone managing `world.users` with 10,000 children might split into `world.users.a-m` and `world.users.n-z`
- The split point is chosen to balance data size and access patterns

**Temporal splits** separate current and historical data:
- Recent data (last 30 days) stays on fast SSD storage
- Historical data moves to slower archival storage
- Queries automatically route to the appropriate temporal zone

After splitting, new zones are re-hashed onto the ring and may land on different nodes, providing automatic load distribution.

### Write Coordination

#### Single-Zone Writes

Writes within a single zone are processed directly by the zone authority:

1. **Validation** — check permissions, schema constraints, and data integrity
2. **Assignment** — apply timestamps, author stamps, and generate checksums
3. **Storage** — append to local storage files
4. **Replication** — queue for replica updates during next heartbeat
5. **Response** — confirm success to client

#### Cross-Zone Writes

Writes that affect multiple zones use distributed transaction coordination:

1. **Prepare phase** — the coordinating zone sends prepare requests to all affected zones
2. **Vote phase** — each zone validates the write and votes commit/abort
3. **Commit phase** — if all zones vote commit, the coordinator sends commit requests
4. **Acknowledge phase** — each zone applies the write and acknowledges completion

This ensures atomicity across zone boundaries while maintaining the mesh's decentralized nature.

### Failure Handling

#### Node Failures

When a node fails, its responsibilities are automatically redistributed:

1. **Detection** — missing heartbeats trigger failure detection (15-30 seconds)
2. **Promotion** — replica nodes become authorities for affected zones
3. **Re-replication** — new replicas are created on available nodes
4. **Re-balancing** — hash ring adjustments may trigger zone migrations

The mesh continues operating with reduced capacity until the failed node recovers or is permanently removed.

#### Partition Tolerance

Network partitions are handled through quorum-based decisions:

- Zones with a majority of replicas remain writable
- Zones with only a minority of replicas become read-only
- When partitions heal, conflicts are resolved through timestamp ordering

#### Data Recovery

Failed nodes that rejoin the mesh automatically resynchronize:

1. **Delta calculation** — compare local state to current replicas
2. **Incremental sync** — download only missing operations since failure
3. **Verification** — validate integrity of recovered data
4. **Resumption** — return to normal operation once fully synchronized

### Performance Optimizations

#### Locality Optimization

The mesh optimizes for data locality through several mechanisms:

- **Agent affinity** — users' data tends to be co-located on nearby nodes
- **Temporal locality** — recent data is cached more aggressively
- **Access patterns** — frequently accessed data is replicated more widely

#### Compression and Archival

Historical data is progressively compressed and archived:

- **Recent data** (< 7 days) — uncompressed on fast storage
- **Medium-term data** (7-90 days) — LZ4 compression on standard storage
- **Long-term data** (> 90 days) — ZSTD compression on archival storage

#### Caching Strategy

Multi-level caching reduces latency for frequent operations:

- **Local cache** — recently accessed data cached in memory
- **Zone cache** — popular data cached at zone replicas
- **Global cache** — widely accessed data cached at multiple nodes

### Security in the Mesh

#### Node Authentication

All nodes authenticate to the mesh using public key cryptography:

- Each node has a Ed25519 keypair
- Node identity is derived from the public key
- All inter-node communication is authenticated and encrypted

#### Data Protection

Data is protected both in transit and at rest:

- **Transport security** — TLS 1.3 for all inter-node communication
- **Storage encryption** — AES-256-GCM for data at rest
- **Key management** — zone-specific keys with automatic rotation

#### Permission Enforcement

Permissions are enforced consistently across all nodes:

- Authority nodes validate permissions before accepting writes
- Replica nodes verify permissions before serving reads
- Permission changes propagate through the gossip protocol

### Monitoring and Observability

#### Mesh Health Metrics

The mesh continuously monitors its own health:

- **Node availability** — track active vs. failed nodes
- **Zone distribution** — ensure balanced load across nodes
- **Replication status** — verify all zones have sufficient replicas
- **Network health** — monitor inter-node communication latency

#### Performance Metrics

Comprehensive performance monitoring covers:

- **Write latency** — time from client request to completion
- **Read latency** — time to serve read requests
- **Throughput** — operations per second across the mesh
- **Resource utilization** — CPU, memory, storage, and network usage

#### Alerting and Self-Healing

The mesh includes automated alerting and self-healing:

- **Anomaly detection** — identify unusual patterns in metrics
- **Automatic recovery** — restart failed services, migrate overloaded zones
- **Capacity planning** — predict when additional nodes are needed
- **Security monitoring** — detect potential security threats or breaches


## Defragmentation

As data ages in AmorphDB, the values file accumulates dead space from old versions that are no longer the current instance. This fragmentation reduces storage efficiency and can impact performance. The defragmentation system compacts storage during maintenance windows while preserving all temporal guarantees.

### Fragmentation Sources

Dead space accumulates from several sources:

1. **Value updates** — when an attribute changes, the old value remains but is no longer current
2. **Attribute deletion** — removed attributes leave their values orphaned
3. **Instance expiration** — when retention policies remove old instances
4. **Failed transactions** — partially written data from aborted operations

Over time, this can result in significant wasted space, particularly in zones with high update rates.

### Compaction Strategy

Defragmentation operates at the zone level and follows a careful process to maintain data integrity:

#### Preparation Phase

1. **Zone enters maintenance mode** — writes are queued, reads continue from current files
2. **Fragmentation analysis** — identify dead space and optimal compaction strategy
3. **Backup verification** — ensure recent backups exist for recovery if needed
4. **Resource reservation** — allocate memory and temporary storage for compaction

#### Compaction Phase

1. **Value relocation** — copy live values to new storage locations
2. **Reference updates** — modify instance records to point to new value offsets
3. **Index reconstruction** — rebuild hash indexes with new offsets
4. **Integrity verification** — validate all references and checksums

#### Commit Phase

1. **Atomic swap** — replace old files with compacted versions
2. **Apply queued writes** — process operations that arrived during maintenance
3. **Resume normal operation** — zone returns to active status
4. **Cleanup** — remove temporary files and release resources

### Scheduling

Defragmentation is scheduled based on several factors:

#### Fragmentation Thresholds

- **Space utilization** — trigger when dead space exceeds 30% of total storage
- **Access patterns** — prioritize zones with high fragmentation and high read load
- **Time since last compaction** — ensure regular maintenance even for low-change zones

#### Maintenance Windows

Compaction is scheduled during periods of low activity:

- **Historical analysis** — identify typical low-activity periods for each zone
- **Advance notification** — warn clients of upcoming maintenance windows
- **Emergency compaction** — immediate compaction when fragmentation severely impacts performance

#### Coordination with Replicas

Since zones have multiple replicas, defragmentation can be staggered:

1. **Replica compaction** — compact replica nodes first
2. **Authority handoff** — temporarily promote a compacted replica to authority
3. **Original authority compaction** — compact the original authority
4. **Role restoration** — return authority to the original node

This approach minimizes downtime while ensuring all replicas remain efficiently organized.

### Data Preservation

Defragmentation must preserve all temporal data while reorganizing storage:

#### Temporal Integrity

- **Timestamp preservation** — all instance timestamps remain unchanged
- **Chain integrity** — previous/next pointers in instance chains are updated correctly
- **Hash verification** — all checksums are recalculated for new storage locations

#### Performance Optimization

During compaction, the system optimizes data layout:

- **Hot data clustering** — place frequently accessed values at the beginning of files
- **Cold data archival** — move infrequently accessed values to compressed storage
- **Temporal ordering** — organize values to improve temporal query performance

#### Backup Integration

Defragmentation coordinates with the backup system:

- **Pre-compaction snapshot** — create recovery point before beginning compaction
- **Incremental backup** — stream only the changes made during compaction
- **Verification** — ensure backup integrity before removing old files

### Error Handling

If defragmentation fails partway through, recovery mechanisms ensure data safety:

#### Rollback Capability

- **Transaction log** — record all changes made during compaction
- **Rollback procedure** — revert to pre-compaction state if errors occur
- **Consistency verification** — validate data integrity after rollback

#### Partial Compaction

If resources are limited, compaction can proceed incrementally:

- **Segment-based compaction** — compact portions of the values file independently
- **Priority-based ordering** — compact the most fragmented segments first
- **Resource monitoring** — pause compaction if system resources become constrained

### Performance Impact

Defragmentation is designed to minimize impact on normal operations:

#### Resource Management

- **Memory usage** — limit compaction memory to avoid affecting other operations
- **I/O throttling** — control disk access to prevent performance degradation
- **CPU priority** — run compaction at lower priority than client operations

#### Client Experience

- **Read availability** — reads continue throughout most of the compaction process
- **Write queuing** — writes are queued briefly during the atomic swap phase
- **Transparent recovery** — clients automatically retry if operations fail during maintenance

#### Post-Compaction Benefits

After compaction, zones experience improved performance:

- **Reduced storage usage** — significant space savings from dead value removal
- **Improved cache efficiency** — better spatial locality for related data
- **Faster temporal queries** — optimized layout accelerates historical data access


## Stamps, Filters, and Permissions

### Stamps

A stamp is a set of attributes automatically embedded into every instance a user writes. Stamps provide context — who wrote it, in what capacity, under what project, at what clearance level — without requiring the author to manually attach this information to every write.

**System-injected identity:** The `@author` meta attribute is mandatory on every instance. The service injects it automatically at write time, recording the identity of the agent that performed the write. This is a system-level guarantee — no user code can suppress or alter it.

**User stamps:** Each agent has a personal stamp at `~.stamp`. Its attributes are embedded into every instance the agent writes.

    ~.stamp:
        role = "engineer"
        timezone = "America/New_York"

**Hierarchical stamps:** Stamps can also be attached at any point in the hierarchy using the `@stamp` meta attribute. As a write occurs, the service collects `@stamp` records from the write point up to `~`, merging them into an effective stamp. Deeper stamps win on collision.

    ~.stamp:                                    # personal
        role = "engineer"

    my.world.employer.@stamp:                   # employer level
        company = "AmeriCU"
        employment_type = "full-time"

    my.world.employer.department.@stamp:         # department level
        department = "IT"
        clearance = "internal"

    my.world.employer.department.task42.@stamp:  # task level
        task_id = 42
        project = "AmorphDB"
        role = "lead"                           # overrides personal "engineer"

A write at `my.world.employer.department.task42.results` produces an effective stamp combining all levels:

    effective stamp:
        role = "lead"                   # task level wins over personal
        timezone = "America/New_York"   # from personal
        company = "AmeriCU"             # from employer
        employment_type = "full-time"   # from employer
        department = "IT"               # from department
        clearance = "internal"          # from department
        task_id = 42                    # from task
        project = "AmorphDB"            # from task

**Stamp snapshots:** When an instance is written, the service creates or reuses a snapshot of the effective stamp and embeds it in the instance. If the stamp has not changed since the last write, the same snapshot is reused. Old instances retain their original stamp regardless of later changes — the stamp is frozen at write time.

The service may cache the effective stamp for each path prefix. The cache invalidates only when a `@stamp` along the path changes, which is rare.

**Collision resolution:** If the instance has an attribute with the same name as an embedded stamp attribute, the instance's own attribute wins. Record owners can mark attributes as `(protected)` to ensure they cannot be removed or overwritten, preventing stamp attributes from surfacing through absence.

### Embed

Embed is the mechanism that makes stamps efficient. The stamp record is stored once and many instances reference the same snapshot.

Embed differs from other reference types:

| Mechanism | Storage | Writes propagate? | Attributes projected? |
|-----------|---------|-------------------|----------------------|
| `(copy)` | Value duplicated | No | No — independent copy |
| `(link)` | Reference to parent attribute | Yes — writes go to parent | No — single value |
| Embed | Reference to a record | No — snapshot is frozen | Yes — all attributes appear as native |

When reading an instance's attributes, the runtime encounters the embed and transparently includes the embedded record's attributes. To the reader, they appear as part of the instance.

### Filters

A filter is a set of conditions that determine what an agent can see. Filters are stored at `~.filter` and apply to all reads the agent performs. Data that does not pass the filter is invisible — it is as if it does not exist.

    ~.filter:
        clearance in ["public", "internal"]
        department ?= "IT" or department ?= "shared"

Filters operate on stamp attributes. Because every instance carries a stamp, filters can select based on who wrote the data, under what department, at what clearance level, or any other stamped context.

This is why the `my` keyword matters as a perspective — `my.world.market` is not raw `world.market` but the filtered view. Different agents with different filters see different slices of the same data.

Filters are user-configured — an agent chooses what to see. Permissions determine what an agent is *allowed* to see. Filters operate within the bounds of permissions, providing personal curation on top of access control.

### Permissions

Permissions are condition expressions stored as meta attributes on any node in the hierarchy. They evaluate against the agent attempting the operation. Permissions cascade downward through the tree until overridden at a deeper point.

#### Permission Types

| Meta attribute | Controls |
|----------------|----------|
| `@read` | Can the agent see the data |
| `@write` | Can the agent change existing values |
| `@expand` | Can the agent add new attributes (grow the structure) |
| `@grant` | Can the agent modify permissions |
| `@purge` | Can the agent permanently erase historical data |

#### Setting Permissions

Permissions are condition expressions that reference properties of the requesting agent:

    my.world.employer.department:
        @read = (agent.role ?= "employee")
        @write = (agent.role ?= "manager" or agent.role ?= "admin")
        @expand = (agent.role ?= "admin")
        @grant = (agent.identity ?= "kalevo")
        @purge = (agent.identity ?= "kalevo" and agent.clearance ?= "admin")

Everything under `department` inherits these permissions. A subtree can override them:

    my.world.employer.department.public:
        @read = (Anything)                      # open to all

    my.world.employer.department.secrets:
        @read = (agent.clearance ?= "admin")    # tighter than parent

#### Defaults

- Under `~` (an agent's home): all permissions default to owner-only.
- Under `world` root: `@read` defaults to Anything; `@write`, `@expand`, `@grant`, and `@purge` default to Nothing.
- Cascade means permissions are set at key points in the tree and flow downward until overridden.

#### Interaction with Filters

Permissions and filters are distinct layers. Permissions determine what an agent is *allowed* to access — they are set by the data owner and enforced by the system. Filters determine what an agent *chooses* to see — they are set by the agent and operate within the bounds of permissions. An agent cannot use a filter to see data they lack permission to read, nor can they remove a filter to bypass permissions.


## Mesh Architecture

AmorphDB is designed as a decentralized mesh network. Virtually, the entire database is one large hierarchy rooted at `world`. Physically, that hierarchy is distributed across nodes, each responsible for a portion of the tree.

### The Virtual Tree

The tree has a single root — `world` — under which all data lives. Agent homes live under `world.agent`, keeping them separate from other top-level structures:

    world
    ├── agent
    │   ├── kalevo              # an agent's home
    │   ├── miratu              # another agent's home
    │   └── ...
    ├── clock                   # system clock
    ├── market                  # shared data
    ├── services                # shared services
    └── ...

Each agent's home is at `world.agent.{identity}`, accessible via `~` universally or `my` from program root scope. The `world` root is reachable as `my.world` from any agent's perspective.

An agent's home (`~`) contains both persistent and virtual elements:

    ~                               # your home (world.agent.{identity})
    ~.stamp                         # your stamp (persistent)
    ~.filter                        # your filter (persistent)
    ~.world                         # the shared world (persistent)
    ~.computer                      # your local machine (virtual)

Most of `~` is persistent and stored in the mesh. The `computer` sub-path is a virtual mount — it is not stored in the mesh but provided by the local node the agent is connected from. Other agents looking at your home from outside (`my.world.agent.kalevo`) see your persistent attributes but not `computer`. If an agent is connected from two different machines simultaneously, each session sees a different `~.computer`.

Local scope (variables within a running procedure) uses the same data structures and types as persistent storage but lives only in memory. Data becomes persistent only when written under `my` or `world`. This means the runtime needs no serialization boundary between working memory and storage — the format is the same throughout.

### Agent Identity

Every agent in the system — whether human, AI, or programmatic — has a unique identity generated from alternating consonant-vowel syllables. This produces identifiers that are pronounceable, easy to spell from hearing, easy to remember, and drawn from a vast non-sequential space.

With approximately 100 CV syllables (20 consonants × 5 vowels), three syllables yield one million possible identities; four syllables yield one hundred million. A blacklist filters out unfortunate combinations that spell unintended words.

Examples: kalevo, miratu, senabo, tokeli, dupano

#### Agent Types

All agents have identities and keys, but their relationship to the mesh differs:

| | Node | External Agent (human, AI, program) |
|---|---|---|
| Home location | Local to the node's own storage | In the mesh, zoned and replicated |
| Keys stored | On local disk | In mesh home, encrypted with agent's derived key |
| Survives hardware loss | No | Yes |
| Accessible from anywhere | No | Yes — agent can connect from any node |

A **node** is a service instance running on hardware. Its home exists only on its own storage — if the hardware is gone, the node is gone. This is appropriate because a node *is* its hardware. Node homes do not consume zone resources or require replication.

An **external agent** (human, AI, or program) is mobile. Its home is stored in the mesh like any other data, replicated and persistent. The agent can connect through any node and reach its home.

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

    zone:
      - subtree_root          # path in the tree (e.g., world.agent.kalevo)
      - instance_range         # optional: temporal range this node is responsible for

Each zone has a single write authority to avoid conflicts. Reads may be served by the authority or any replica.

#### Zone Assignment

Zones are assigned to nodes via consistent hashing. The zone's root path (and temporal range, if present) is hashed onto a ring of nodes. The node where the hash lands becomes the zone authority. The next N nodes on the ring become replicas.

    hash("world.agent.kalevo")          → node_C (authority), node_D, node_E (replicas)
    hash("world.agent.miratu")          → node_A (authority), node_B, node_C (replicas)
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
        world.market.cleaned = process(world.market.raw)

    # watcher on the node owning world.market.cleaned
    watch analyze(world.market.cleaned):
        world.market.signals = analyze(world.market.cleaned)

    # watcher on the node owning world.market.signals
    watch alert(world.market.signals):
        if world.market.signals.anomaly ?= true:
            ~.notifications..append("Market anomaly detected")

Each heartbeat propagates changes through the pipeline. The mesh is the execution engine.

### Locality

Most operations cluster in specific subtrees. A user predominantly accesses `my.*`, a department works within `world.departments.{dept}.*`. Consistent hashing keeps related data together — the zone for a subtree encompasses everything beneath it until a split occurs.

Some subtrees are broadly shared (configuration, shared libraries, common reference data). These require replication across multiple nodes rather than single-node ownership.

### Standalone Mode

Until a node is connected to a mesh, it operates locally with full functionality. All roles are played by the single node. The transition to mesh participation is seamless when a seed address is provided.


## Components

AmorphDB consists of three components:

| Component | Role |
|-----------|------|
| **Service** (`amorphd`) | Daemon. Manages local storage, participates in the mesh, listens on a local socket and a network socket. |
| **Client** (`amorph`) | Interactive shell and batch processor. Connects to a service, executes MBL code. |
| **Control** (`amorphctl`) | Administrative tool. Manages the service, configures mesh participation, monitors health. |

### Service (amorphd)

The service is the core component that manages data storage and mesh participation. It runs as a daemon process on each node.

#### Responsibilities

- **Local storage** — maintains the zone data assigned to this node
- **Mesh participation** — communicates with other nodes via heartbeat and gossip
- **Client connections** — serves requests from local and remote clients
- **Zone management** — handles zone splitting, replication, and migration
- **Security** — enforces permissions, manages encryption, handles authentication

#### Sockets

The service listens on two sockets:

- **Local socket** (Unix domain socket) — for connections from clients on the same machine
- **Network socket** (TCP) — for connections from other mesh nodes and remote clients

The local socket provides better performance and security for clients running on the same machine. The network socket enables remote access and mesh coordination.

#### Configuration

Service configuration is minimal by design:

```yaml
# /etc/amorphd/config.yaml
data_directory: /var/lib/amorphd
local_socket: /var/run/amorphd/socket
network_port: 5000
mesh_seeds:
  - "node1.example.com:5000"
  - "node2.example.com:5000"
```

Most configuration is automatically discovered or negotiated through the mesh.

### Client (amorph)

The client provides both interactive and batch access to AmorphDB. It connects to a service and executes MBL code.

#### Interactive Mode

When run without arguments, the client launches an interactive REPL:

```bash
$ amorph
Connected to amorphd on local socket
Agent: kalevo

amorph> my.test.value = "hello world"
amorph> my.test.value
"hello world"
amorph> my.test.@timestamp
@2026-03-10 14:30:15
```

The REPL provides:
- **Line editing** — history, tab completion, multi-line input
- **Syntax highlighting** — MBL code is colorized for readability
- **Error reporting** — detailed error messages with context
- **Help system** — integrated documentation and examples

#### Batch Mode

The client can execute MBL files non-interactively:

```bash
$ amorph script.mbl
$ amorph --node=production.example.com:5000 --script=batch_process.mbl
```

Batch mode supports:
- **Remote execution** — connect to any mesh node
- **Parameter passing** — command-line arguments become MBL variables
- **Output capture** — redirect results for further processing
- **Error handling** — exit codes reflect script success/failure

#### Agent Authentication

The client handles agent authentication automatically:

1. **Key discovery** — looks for agent keys in standard locations
2. **Automatic login** — authenticates using discovered keys
3. **Agent creation** — helps new users create agent identities
4. **Key management** — assists with key rotation and backup

### Control (amorphctl)

The control tool provides administrative functions for managing AmorphDB deployments.

#### Cluster Management

```bash
# View cluster status
amorphctl status

# Add a new node to the cluster
amorphctl join --node=new-node.example.com:5000

# Remove a node from the cluster
amorphctl leave --node=old-node.example.com:5000

# Rebalance zones across nodes
amorphctl rebalance
```

#### Zone Administration

```bash
# List all zones and their assignments
amorphctl zones list

# Force a zone split
amorphctl zones split world.agent.kalevo

# Migrate a zone to a different node
amorphctl zones migrate world.market --to=node3.example.com:5000

# Check zone health and replica status
amorphctl zones health
```

#### Backup and Recovery

```bash
# Create a cluster-wide backup
amorphctl backup create --target=/backup/path

# Restore from backup
amorphctl restore --source=/backup/path

# Verify backup integrity
amorphctl backup verify /backup/path
```

#### Monitoring and Metrics

```bash
# Real-time performance metrics
amorphctl metrics

# Generate health report
amorphctl health-check

# Export metrics for external monitoring
amorphctl metrics export --format=prometheus
```

### Integration

The three components work together to provide a complete database system:

- **Service** provides the core functionality and data management
- **Client** enables users to interact with their data
- **Control** allows administrators to manage the deployment

Each component can be deployed independently:
- Services run on database nodes in the mesh
- Clients can run anywhere and connect remotely
- Control can be used from administrative workstations

This separation enables flexible deployment architectures, from single-node development setups to large-scale distributed production clusters.