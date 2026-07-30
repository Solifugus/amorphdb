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
| `:`    | Introduces a definition (procedures, watchers, record bodies) |
| `=`    | Assignment of a value (i.e., recording a change) |
| `.`    | Sets the current scope |

### Same-Line Definitions

A definition body may be placed on the same line as the colon, as a convenience
for short definitions. Multiple statements on the same line are separated by
semicolons:

    my.functions.double: procedure(x): return x * 2

    my.account.balance: 0; my.account.status: "active"

If the colon is followed by a newline, the body must be indented (the standard
multi-line form). Both forms are always available — same-line is a shorthand, not
a replacement:

    # Same-line (short definitions)
    my.config.host: "localhost"; my.config.port: 5432

    # Multi-line (standard form, always valid)
    my.config:
        host: "localhost"
        port: 5432

> 🚧 **Same-line definitions are not yet implemented.**

### Recursive Assignment

Assigning to a path whose intermediate nodes do not yet exist automatically
creates those nodes rather than producing an error. This applies to both value
assignment and definition:

    my.new.deeply.nested.value = 42      # creates my.new, my.new.deeply, etc.

Each intermediate node is created as an empty record. If an intermediate node
already exists, it is left unchanged.

> 🚧 **Recursive assignment is not yet implemented.** Currently, all intermediate
> nodes must exist before assignment.

### Data Types

Text, Number, Boolean, Time, Money, Picture, Reference, Procedure, Watcher, Embed

> **Note on Embed:** Embed is a structural composition mechanism, not a value type in
> the same sense as the others. It does not hold a value that can be assigned,
> returned, or passed as a parameter. It is listed here for completeness because it
> appears in the type system and is visible in storage, but it is described fully under
> [Embed](#embed) in the Stamps section.

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
- Boolean
  A true or false value. In MBL, the literals are `true` and `false`.
  Truthiness rules for other types: Number 0 is false, any other number is true.
  Empty text is false, any other text is true. Zero time is false, any other time
  is true. Unknown is always false in boolean context.
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
  In MBL, a money literal is specified as either a specific currency symbol or the
  universal currency symbol (¤) followed immediately by a decimal number. When using
  the universal symbol, the ISO 4217 currency code follows the number.
  Examples:
    - $143.68
    - €21.80
    - ¤143.68 USD
    - ¤21.80 EUR
- Picture
  Full RGBA at 16-bits each.
  In MBL, it cannot be typed as a literal but must be loaded via an operation.
- Reference
  A link (by name or unique identifier) to another node elsewhere in the tree.
  In MBL, a literal reference is specified using the `(link)` keyword before a path (e.g., `world.foo` resolves to a value, while `(link)world.foo` is a reference to that location).
- Procedure
  A stored program (see Procedures below).
- Watcher
  A persistent trigger (see Watchers below).
- Embed
  A structural composition mechanism that makes the attributes of one record appear
  as siblings within another record, without copying or merging them. The embedded
  record retains its identity as a distinct unit. Used internally by the stamp system
  and available to programmers via the `embed` keyword. See [Embed](#embed).

### Meta Type

Each type above supports one meta type:

- Unknown
  Indicates that the expected value is not available. Like any value in
  AmorphDB, an Unknown is a node in the tree and may carry sub-attributes.
  In MBL, the `unknown` keyword creates an Unknown value. An optional
  text argument sets the `.reason` attribute as a shorthand:
  Examples:
    - `unknown` — bare Unknown, no reason
    - `unknown("not_found")` — Unknown with reason (shorthand for setting `.reason`)
    - `unknown("timeout")` — application-specific reason
  Additional attributes can be set on an Unknown like any other node:
    - `my.result = unknown("inconclusive")`
    - `my.result.confidence = 0.3`
    - `my.result.options = [5, 3, 2]`
  The system produces Unknown values automatically in these cases:
    - Reading a non-existent path: `unknown("not_found")`
    - Accessing data hidden by permissions or filters: `unknown("filtered")`
    - Division by zero: `unknown("division_by_zero")`
    - Network failure: `unknown("timeout")`, `unknown("connection_refused")`, etc.

### Structures

AmorphDB provides two aggregate structures: Records and Lists. Both are based on the same underlying tree structure.

- Records store data in named fields.
- Lists store data in numerically indexed fields.

Since these are both tree nodes with attributes underneath, either may contain any mix of types including other records, other lists, or any other type. The distinction exists only to indicate the predominant access method.

### Heritability and Instantiation

Any record can serve as a template. The `new` keyword creates an instance from
a template. Each attribute in the template has a heritability rule that controls
what happens to it in the new instance, stored as two meta attributes:

| Meta Attribute | Values | Meaning |
|----------------|--------|---------|
| `@heritability` | `"copy"` (default), `"link"`, `"reset"`, `"exclude"` | How this attribute behaves during instantiation |
| `@default` | any value | The value to use when `@heritability = "reset"` |

Heritability rules:

| Rule | Meaning |
|------|---------|
| `"copy"` | Deep copy the value — the new instance gets its own independent copy (default) |
| `"link"` | Create a reference — the new instance shares the template's value; changes to the template propagate |
| `"reset"` | Override with the `@default` value regardless of the template's current value |
| `"exclude"` | Omit from new instances — the value exists in the template but is not inherited |

When `@heritability` is not set on an attribute, `"copy"` is the default.

**Shorthand syntax:** Value modifiers in parentheses set the meta attributes
inline, consistent with other value modifiers like `(quietly)` and `(link)`:

| Shorthand | Equivalent meta attributes |
|-----------|---------------------------|
| `(copy) value` | `@heritability = "copy"` |
| `(link) value` | `@heritability = "link"` |
| `(reset X) value` | `@heritability = "reset"`, `@default = X` |
| `(exclude) value` | `@heritability = "exclude"` |

**Template definition with shorthand modifiers:**
```
my.templates.account:
    name: (copy) ""
    balance: (reset 0) $1000
    interest_rate: (link) 0.05
    internal_notes: (exclude) "template maintenance log"
```

**Equivalent using meta attributes directly:**
```
my.templates.account:
    name: ""
    balance: $1000
    balance.@heritability = "reset"
    balance.@default = 0
    interest_rate: 0.05
    interest_rate.@heritability = "link"
    internal_notes: "template maintenance log"
    internal_notes.@heritability = "exclude"
```

**Instantiation:**
```
my.customers.alice = new(my.templates.account)
my.customers.alice.name = "Alice"

# alice.balance is 0 (reset to @default, regardless of template's $1000)
# alice.interest_rate is linked to template (shared, updates propagate)
# alice.internal_notes does not exist (excluded)
```

**Per-field overrides at instantiation time:**

Heritability modifiers can be overridden when creating a new instance:

```
my.customers.bob = new my.templates.account {
    name: (copy) "Bob",
    balance: (reset 500) 0,
    interest_rate: (copy) 0.05
}
```

Here `interest_rate` was `"link"` in the template but overridden to `"copy"` for
Bob — he gets his own independent copy of the rate.

**Global modifier:**

A global modifier applies to all attributes that don't have their own explicit
modifier:

```
my.snapshot = new(my.templates.account, (link)) { name: (copy) "Snapshot" }
```

Everything is linked except `name`, which is explicitly copied.

**Heritability is recursive.** When an attribute is a nested record and the
rule is `"copy"`, the entire subtree is deep-copied. When the rule is
`"link"`, the reference points to the template's subtree — changes at any depth
propagate.

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
    my.log[@agent = (link)world.agent.kalevo, @time >= @2026-03-01]

    # Values that are temporary (have TTL)
    my.cache[@ttl > 0]

    # Large files (over 1MB)
    my.files[@size > 1000000]

The `(link)` keyword indicates that `world.agent.kalevo` should be treated as a reference to that path, not the value at that path.

## MBL (Modern Business Language)

### Lexical Structure

MBL source text is UTF-8. A program is a sequence of statements separated by whitespace or newlines.

**Comments** follow the same adjacency principle as text literals. A single `#`
comments to end of line or to a closing `#` on the same line. Two or more
adjacent `#` characters open a block comment that terminates at the next
occurrence of the same number of adjacent `#` characters. Block comments may
span multiple lines and may contain shorter `#` sequences without terminating
early. This allows commenting out code that itself contains comments.

    # This is a single-line comment
    my.account.balance = 100    # Another single-line comment
    my.account.status = "active" # inline comment # my.account.name = "checking"

    ## This is a block comment
    that spans multiple lines ##

    ## This block comment contains # single-line comment syntax
    and still continues until the matching pair ##

    ### This block comment contains ## nested block markers ##
    and still continues until the matching triple ###

**Identifiers** are sequences of letters, numbers, or underscores, beginning with a letter or underscore. Case is significant: `Account` and `account` are different. Identifiers may contain Unicode characters. Reserved words cannot be used as identifiers:

    and, or, not, if, else, while, for, in, return, new, quietly,
    procedure, watch, catch, my, world, true, false, embed,
    break, pass, link, unknown, copy, reset, exclude, append,
    prepend

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

**Operators** include arithmetic, comparison, logical, and concatenation operators:

    +, -, *, /, %, ^               # Arithmetic
    =, !=, <, >, <=, >=            # Comparison
    and, or, not                   # Logical
    &                              # String concatenation

**Paths** are dot-separated sequences of identifiers, with special prefixes:

    world.market.price             # Absolute path
    .local.variable               # Relative to current scope
    ~.account.balance             # Relative to user's home
    my.account.balance            # Same as above (from program root)

**References** use the `(link)` keyword before a path:

    (link)world.market.price         # Reference to a location (not its value)

**Record literals** assign multiple named fields inline using curly braces. Field
names and values are separated by colons, fields separated by commas:

    x = { name: "Matthew C. Tedder", age: 55, job: "Software Engineer" }

This is distinct from a projection (see below) because every field has a value.
Record literals may be nested:

    x = { address: { city: "Rome", state: "NY" }, active: true }

**Projections** select a subset of sub-attributes from a path, using curly braces
containing only names (no values, no colons). The result is a record containing
only the named fields:

    person{ name, age, job }                         # read three fields
    my.employees[active = true]{ name, salary }      # filter then project

Wildcard patterns select all sub-attributes whose names match a prefix, suffix,
or the full set:

    my.products{ price_of_* }     # all attributes starting with "price_of_"
    my.products{ *_total }        # all attributes ending with "_total"
    my.products{ * }              # all sub-attributes (explicit full read)

Exclusions can be combined with wildcards:

    my.products{ *, not internal_code, not cost_basis }   # all except named fields

> 🚧 **Wildcard projections are not yet implemented.** Simple named projections
> (`{ name, age, job }`) are specified and planned. Wildcard and exclusion forms
> are lower-priority and will be added in a later pass.

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

**Assignment:** =
- `=` records a value change (creates a new instance)
- `:` introduces a definition (used for procedures, watchers, record bodies)

**String Concatenation:** &
- Joins values into text: `"Hello" & " " & "World"` yields `"Hello World"`
- Non-text values are coerced to text before joining

**Context-sensitive `=`:** The `=` operator serves as both assignment and
equality comparison. The parser distinguishes them by syntactic context:
- At **statement level** (left side is a path, right side is a value): assignment
- Inside **bracket expressions** `[...]`: equality comparison
- Inside **conditions** (`if`, `while`, `for`): equality comparison
- After **`:` in definitions**: not `=` — uses `:` instead

There is never ambiguity: assignment requires a path target at statement level,
and comparison requires an expression context. The programmer does not need to
use different symbols — the context makes the meaning clear.

The **type-safe** comparison operators (`?=`, `?!=`, etc., described below)
exist for a different purpose: they handle Unknown values safely.
`x = 5` propagates Unknown if `x` is Unknown; `x ?= 5` returns false instead.

**Path Resolution:** ., .., ~
- `.` accesses child attributes
- `..` accesses special operations on collections
- `~` accesses user's home

**Type Checking:** ?=, ?!=, ?<, ?>, ?<=, ?>=
- Comparison operators with "?" prefix check for Unknown values
- Return true/false instead of propagating Unknown up the expression tree
- Example: `if x ?= 5:` succeeds if x is 5, fails safely if x is Unknown

**Collection Operations:**
- `..count` - number of attributes under a node
- `..append(value)` - add item to end of list
- `..prepend(value)` - add item to beginning of list
- `..remove(index)` - remove by position
- `..remove(value)` - remove first match by value
- `..combine(separator)` - join values with separator

**Brackets and Projections:**
- `[expression]` - filter or query child attributes
- Used for array indexing: `my.array[0]`
- Used for conditional queries: `my.users[active = true]`
- `{name, name, ...}` - projection: read only the named sub-attributes
- `{prefix_*}` - wildcard projection (🚧 not yet implemented)
- `{*, not name}` - exclusion projection (🚧 not yet implemented)
- Brackets and projections compose: `my.users[active = true]{ name, email }`

**Operator Precedence** (highest to lowest):
1. Path resolution (.), member access
2. Unary not, unary -, unary +
3. Exponentiation (^)
4. Multiplication (*), Division (/), Modulo (%)
5. Addition (+), Subtraction (-)
6. String concatenation (&)
7. Comparison (=, !=, <, >, <=, >=)
8. Logical and
9. Logical or
10. Assignment (=)

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
- Unknown values propagate through expressions: `5 + unknown("offline")` yields `unknown("offline")`
- Type-safe comparison operators (?=, ?!=, etc.) return false for Unknown instead of propagating

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
is_unknown(value)                         # Check for Unknown meta type
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
    x..append(value)                            # add item to end of list
    x..prepend(value)                           # add item to beginning of list
    x..remove(position)                         # remove by numeric index (list reindexes)
    x..remove(value)                            # remove first match by value (list) or by name (record)
    x..remove(from, to)                         # remove a range by index

    # Examples
    my.shopping_list..count                     # how many items
    my.shopping_list..append("milk")            # add to end
    my.shopping_list..prepend("bread")          # add to beginning
    my.shopping_list..remove(2)                 # remove third item (zero-indexed)
    my.contacts..remove("bob")                  # remove contact named bob
    my.numbers..combine(" + ")                  # "1 + 2 + 3"

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
```

**Flow Control:**
```
return value                                # Exit procedure with value
return                                      # Exit procedure with no value
break                                       # Exit the innermost loop
pass                                        # No-op; placeholder for an empty block
```

`pass` is used where a block is syntactically required but no action is needed.
Because MBL uses indentation for block structure, an empty block body is a
syntax error without `pass`:

```
# Stub procedure — body required, nothing to do yet
my.functions.placeholder: procedure(x):
    pass

# Ignoring a specific condition
if my.account.status = "pending":
    pass
else:
    my.computer.output("Account is active")
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

An alternate form places the `watch` keyword first for convenience:
```
watch my.automation.balance_check(my.account.balance, my.account.limit):
    if my.account.balance > my.account.limit:
        my.account.status = (quietly) "overlimit"
```

Both forms create the same watcher. The first reads as a definition ("this
attribute is a watcher"). The second reads as an action ("watch these paths,
name it this"). Either may be used; they are interchangeable.

**Note:** The syntax `name: watch(...)` is the standard definition form — the watcher name comes first, followed by a colon, then the watch declaration with its body. The alternate `watch name(...)` form is equivalent. Both create a named watcher that can be referenced, enabled/disabled, and inspected like any other value in the hierarchy.

A watcher is a value in the hierarchy like any other. It is operated by the node responsible for the zone where it lives. Its sub-attributes form its persistent local data scope.

#### Watcher Attributes

Every watcher has the following attributes:

| Attribute | Type | Default | Meaning |
|-----------|------|---------|---------|
| `@enabled` | Boolean | true | Whether this watcher is active |
| `@watching` | List of References | (from definition) | Paths being monitored |
| `@code` | Procedure | (from definition) | Code to execute on changes |
| `@last_run` | Time | (none) | When this watcher last executed |
| `@run_count` | Number | 0 | Total number of executions |
| `@last_error` | Text | (none) | Error message from last failed run |

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

#### Multi-Path Watchers

Watchers can monitor multiple paths simultaneously using comma-separated syntax. The watcher triggers when **any** of the watched paths changes (OR semantics):

```
my.automation.account_monitor: watch(my.account.balance, my.account.limit):
    if my.account.balance > my.account.limit:
        my.account.status = (quietly) "overlimit"
        my.alerts.send("Account overlimit: " & my.account.balance)
```

This is equivalent to having separate watchers for each path, but more efficient and allows coordinated logic.

> **Note:** Procedures like `my.alerts.send()` and `my.notifications.send()` in
> the following examples are application-defined — they are not built into
> AmorphDB. They illustrate how developers would build their own logic on top
> of the watcher system.

**Real-World Examples:**

System resource monitoring:
```
my.monitoring.resource_watcher: watch(my.cpu.usage, my.memory.usage, my.disk.usage):
    if my.cpu.usage > 90:
        my.alerts.send("High CPU: " & my.cpu.usage & "%")
    if my.memory.usage > 95:
        my.alerts.send("High memory: " & my.memory.usage & "%")
    if my.disk.usage > 85:
        my.alerts.send("Low disk space: " & (100 - my.disk.usage) & "% remaining")
```

Configuration and time-based automation:
```
my.automation.backup_scheduler: watch(my.config.backup_enabled, world.clock.hour):
    if my.config.backup_enabled and world.clock.hour ?= 2:
        my.computer.run("backup-script.sh")
        my.logs.backup = world.clock.now & ": Backup completed"
```

Multi-account balance tracking:
```
my.automation.balance_monitor: watch(my.checking.balance, my.savings.balance, my.credit.balance):
    total = my.checking.balance + my.savings.balance - my.credit.balance
    my.finance.net_worth = total
    if total < 1000:
        my.alerts.send("Low total balance: $" & total)
```

**Syntax Notes:**
- Use comma-separated paths: `watch(path1, path2, path3)`
- All paths must be valid expressions (no trailing commas)
- Triggers when ANY path changes (not all paths)
- Single-path syntax remains unchanged: `watch(single.path)`
- Append watchers remain single-path only: `watch append(single.list) as items`

#### Append Watchers

A watcher can trigger specifically on items appended to a list during the current tick using the `append` keyword:

```
my.automation.new_orders: watch append(my.orders) as orders:
    for order in orders:
        my.computer.output("New order: " & order.id)
```

**Syntax:**
- The keyword is `append` inside the `watch(...)` call
- The `as name` binding is required — it binds the list of newly arrived items to a local variable for the watcher body
- `name` is chosen by the programmer — it is not a keyword
- The bound variable is an in-memory list of references to the nodes that were appended during this tick, in arrival order
- It follows exactly the same scoping rules as any other local variable — it exists only for the duration of the watcher's execution and is not written to the mesh
- The list always contains everything that arrived in the tick — even if only one item arrived, it is a single-element list. The watcher body can take the last element if it only wants the latest

**Predicate Filter:**
The predicate filter inside `append(...)` is optional. If present, only appended items matching the predicate are included in the bound list, and the watcher only fires if at least one match exists:

```
my.automation.urgent_orders: watch append(my.orders[priority ?= "urgent"]) as urgent:
    for order in urgent:
        my.alerts.send("Urgent order received: " & order.id)
```

This follows the same binding pattern as `for item in list`. The list itself is just a local variable populated by the runtime.

**Real-World Examples:**

External API request processing (for third-party consumers, not PWA clients — see
Network Sub-Library for the data-driven PWA approach):
```
my.services.handlers.balance: watch append(
    my.computer.network.web.requests[
        domain ?= "api.example.com",
        method ?= "GET",
        path ?= "/api/balance"
    ]
) as new_requests:
    for req in new_requests:
        balance = world.agent[req.query.agent].account.balance
        req.respond(200, balance)
        req.processed = (quietly) true
```

Multi-domain external API handling:
```
my.services.handlers.status: watch append(
    my.computer.network.web.requests[
        method ?= "GET",
        path ?= "/status"
    ]
) as new_requests:
    for req in new_requests:
        if req.domain ?= "api.example.com":
            req.respond(200, my.computer.network.web.to_json(world.services.status))
        else if req.domain ?= "admin.corp.internal":
            req.respond(200, my.computer.network.web.to_json(my.admin.status))
        else:
            req.respond(404, "Not found")
        req.processed = (quietly) true
```

Order processing with inventory checking:
```
my.automation.order_processor: watch append(my.orders[status ?= "pending"]) as new_orders:
    for order in new_orders:
        if my.inventory[order.product_id].quantity >= order.quantity:
            order.status = "confirmed"
            my.inventory[order.product_id].quantity = my.inventory[order.product_id].quantity - order.quantity
            my.notifications.send(order.customer_email, "Order confirmed: " & order.id)
        else:
            order.status = "backordered"
            my.notifications.send(order.customer_email, "Order backordered: " & order.id)
```

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

#### The Computer

`~.computer` (accessible as `my.computer` from program root) is a virtual mount
in the agent's home that provides access to the local machine. It is not stored
in the mesh — it is provided by the node the agent is connected from.

```
my.computer.output(value)              # write to stdout
my.computer.input(prompt)              # read from stdin (returns text)
my.computer.run(command)               # execute shell command (returns record)
my.computer.files                      # filesystem access (see below)
my.computer.network                    # network operations (see below)
my.computer.hostname                   # system hostname
my.computer.os                         # operating system name
my.computer.arch                       # processor architecture
```

This allows programs to interact with the physical machine they're running on
while keeping the bulk of their data in the distributed mesh.

##### Shell Execution

`my.computer.run(command)` executes a shell command on the local machine and
returns a record:

```
result = my.computer.run("ls -la /tmp")
my.computer.output(result.stdout)
if result.exit_code != 0:
    my.computer.output("Error: " & result.stderr)
```

| Attribute | Type | Meaning |
|-----------|------|---------|
| `.exit_code` | Number | Process exit code (0 = success) |
| `.stdout` | Text | Standard output |
| `.stderr` | Text | Standard error |

##### Files Sub-Library

`my.computer.files` provides access to the local filesystem. All operations are
subject to OS-level permissions — the daemon runs as a specific OS user and can
only access files that user is permitted to access.

**Basic operations:**
```
my.computer.files.read(path)                    # Read file as Text
my.computer.files.write(path, content)          # Write Text to file
my.computer.files.exists(path)                  # Returns true/false
my.computer.files.delete(path)                  # Delete file
my.computer.files.list(path)                    # List directory (returns list)
my.computer.files.info(path)                    # File metadata (returns record)
```

The `.info()` procedure returns a record with:

| Attribute | Type | Meaning |
|-----------|------|---------|
| `.size` | Number | File size in bytes |
| `.modified` | Time | Last modification time |
| `.is_directory` | Boolean | Whether the path is a directory |

**Structured import:**
```
my.computer.files.import(path, format)
my.computer.files.import(path, format, options)
```

Reads a file and converts its content into an AmorphDB tree node. Supported
formats:

| Format | Description |
|--------|-------------|
| `"json"` | JSON → Record/List. Objects become Records, arrays become Lists. |
| `"csv"` | CSV → List of Records. First row is headers. Type inference on values. |
| `"tsv"` | TSV → List of Records. Same as CSV with tab separator. |
| `"toml"` | TOML → Record. Sections become nested Records. |
| `"xml"` | XML → Record. Elements become Records, text content stored as `_text`. Namespace prefixes preserved. Repeated elements become Lists. |

Options record (all optional, format-dependent):

| Option | Applies to | Meaning |
|--------|------------|---------|
| `separator` | csv | Custom separator character (default: `,`) |
| `headers` | csv, tsv | Boolean — first row contains headers (default: `true`) |

All import failures return Unknown with a reason:
- `unknown("file_not_found")` — path does not exist
- `unknown("unsupported_format")` — format string not recognized
- `unknown("parse_error")` — file content is malformed

**Structured export:**
```
my.computer.files.export(node, path, format)
my.computer.files.export(node, path, format, options)
```

Writes an AmorphDB tree node to a file in the specified format. Supported
formats are the same as import: `"json"`, `"csv"`, `"tsv"`, `"toml"`, `"xml"`.

**Fixed-width import (ARI):**
```
my.computer.files.import_fixed_width(path, ari_spec)
my.computer.files.import_fixed_width(path, ari_spec, options)
```

Imports fixed-width data files using an ARI (Anchor Relative Identification)
specification. ARI is a domain-specific notation for describing the spatial
layout of fixed-width reports and data files:

```
result = my.computer.files.import_fixed_width("/data/report.txt",
    "section bank_report:" &
    "  field report_date: right 12 same: \"BANK REPORT\"" &
    "  field bank_id: right 4 same: \"ID:\""
)
```

> The full ARI specification is documented separately. ARI supports spatial
> anchoring (left/right/up/down), pattern matching, section boundaries
> (starts/ends), break statements for record separation, nested sections,
> and automatic type conversion.

##### Network Sub-Library

`my.computer.network` provides network capabilities. The web-serving features
live under `my.computer.network.web`. All web communication is HTTPS only —
plain HTTP is not supported.

The network sub-library supports three distinct traffic patterns:

| Traffic | Mechanism | MBL Developer Sees |
|---------|-----------|-------------------|
| Own PWA clients | Data-driven — Go layer translates between client and data tree | Just data changes and watchers on application data |
| External API consumption | Outbound HTTP procedures | Procedure calls that return data |
| External API provision / webhooks / auth | Inbound request queue | Request nodes in append watchers |

###### Data-Driven PWA Communication

PWA clients built on the AmorphDB boilerplate do not use the request queue.
Instead, the Go layer translates between MCP messages and the application's
data tree. The MBL developer never sees HTTP requests — they only see data
changing.

**How it works:**

1. A user interacts with the PWA (clicks a button, fills a form)
2. The boilerplate sends the data change to the Go bridge
3. The Go bridge resolves the user's identity from their auth token and
   routes the write to `world.apps.<app>.users.<identity>.*`
4. A per-user watcher fires and executes business logic
5. The watcher writes results back to the user's data subtree
6. The Go bridge detects the change and pushes it to the user's browser via SSE

The MBL developer writes per-user watchers on application data, not on
HTTP requests:

```
# Per-user watcher — registered at signup, fires only for this user
watch my.app.watchers[username & ".submit"](user_path.intent.submit_order):
    if user_path.intent.submit_order = true:
        world.apps.myapp.shared.orders..append({
            user_identity: username,
            items: user_path.forms.order_data.items,
            submitted: now()
        })
        user_path.intent.submit_order = (quietly) false
        user_path.intent.result = { status: "submitted" }
```

This approach means:
- The MBL developer never thinks about HTTP, request objects, or response codes
- Each user has their own watchers — no looping over all users
- Real-time updates flow automatically to the correct user's browser
- Multiple users see shared data changes immediately
- The Go layer handles all transport concerns (SSE connections, MCP parsing,
  identity resolution)

###### PWA User Identity and Authentication

PWAs distinguish between two identity layers:

- **Mesh agents** are AmorphDB's native identities with post-quantum keys
  and mesh permissions.
- **App users** are end users of an application, managed by the app, not by
  AmorphDB. They do not have mesh identities or permissions on the mesh.

The platform provides the data shape, routing, and security boundary. The
application decides what a user is and how they prove identity. **The
developer does not write session logic, identity resolution, or HTTP
plumbing** — the platform handles it.

**Data model:**

```
world.apps.<app>
    app                         # shared UI definition (parsed from app.html)
        _token_ttl: 604800      # login timeout in seconds (from app.html)
    assets                      # static files served by the Go layer
    devices                     # pre-auth browser connections
        d8f2a1b3...             # device ID (generated by boilerplate)
            login               # login form data lands here
            signup              # signup form data lands here
    users                       # authenticated user data
        kalevo
            banner
                user_name = "Kalevo"
            workspace
                orders = [...]
        miratu
            ...
    tokens                      # active auth tokens
        a7f3c1b9...             # opaque token string
            identity: "kalevo"
            device: "d8f2a1b3..."
            @ttl: 604800        # seconds, from app._token_ttl
    auth                        # app-defined auth state
        credentials
            kalevo
                password_hash: "..."
                salt: "..."
```

**Device identity:** The boilerplate generates a device ID on first visit,
stored in localStorage permanently. It identifies the browser, not the person.
Before login, the Go bridge routes writes to
`world.apps.<app>.devices.<deviceId>.login` and `.signup` only. All other
paths are rejected with 401.

**User identity:** The `users.<identity>` subtree is the permanent home of
each user's data. It persists across connections, devices, and disconnections.
The identity is a string chosen by the application — a username, email, UUID,
or anything the app prefers.

**Tokens:** Map a device to a user identity. Each token carries `.identity`,
`.device`, and `@ttl`. Tokens expire automatically via AmorphDB's purge
mechanism. A user can be logged in on multiple devices simultaneously — each
device has its own token pointing to the same identity.

**Token TTL configuration:** Set in `app.html` as a system attribute on the
root section, in seconds:

```html
<section name="app" token_ttl="604800">
    <!-- 604800 seconds = 7 days. 0 = no timeout. -->
    ...
</section>
```

**Go bridge routing:**

| State | Headers | Bridge routes writes to |
|-------|---------|------------------------|
| Pre-auth | `X-AmorphDB-Device` only | `devices.<deviceId>.login` and `.signup` only |
| Post-auth | `X-AmorphDB-Device` + `X-AmorphDB-Token` | `users.<identity>.*` (token must match device) |

A request with an expired, unknown, or device-mismatched token returns 401.

**Authentication flow:**

1. Browser submits credentials to `devices.<deviceId>.login`
2. Login watcher validates credentials, generates token, writes to
   `world.apps.<app>.tokens.<token>` with `.identity`, `.device`, `@ttl`
3. Go bridge returns token to browser; boilerplate stores in localStorage
4. Subsequent requests include both device and token headers
5. Go bridge resolves identity from token and routes to user subtree

**Logout:** Delete the token. The next request returns 401. The boilerplate
clears localStorage and shows the login screen. Passive invalidation happens
automatically when `@ttl` expires.

**Per-user watchers:** Every user gets their own watchers, registered at
signup time via a `setup_user_watchers` procedure. One watcher per user per
action — the watcher fires only when that specific user's data changes.
This is the only pattern; there is no "watch all users" alternative.

```
my.app.setup_user_watchers: procedure(username):
    user_path = world.apps.myapp.users[username]

    watch my.app.watchers[username & ".submit_order"](user_path.intent.submit_order):
        if user_path.intent.submit_order = true:
            world.apps.myapp.shared.orders..append({
                user_identity: username,
                items: user_path.forms.order_data.items,
                submitted: now()
            })
            user_path.intent.submit_order = (quietly) false
            user_path.intent.result = { status: "submitted" }

    watch my.app.watchers[username & ".logout"](user_path.intent.logout):
        if user_path.intent.logout = true:
            for token in world.apps.myapp.tokens:
                if token.identity = username:
                    world.apps.myapp.tokens..remove(token)
            user_path.intent.logout = (quietly) false
```

**Supported user models:**

| Model | How it works |
|-------|-------------|
| Guest only (no login) | All data under `devices.<deviceId>`. No tokens or users. |
| Registered users only | Admin creates users and credentials. PWA shows login only. |
| Self-registration | Signup watcher creates user, credentials, and per-user watchers. |
| External registration | HR import or admin panel writes to `users` and `auth`. |
| Third-party auth (OAuth, SSO) | Login watcher redirects to provider; callback watcher validates and creates token. |

**Security boundary:**

1. **Mesh permissions** — the daemon agent has `@write` on
   `world.apps.<app>.*`. Set once at deployment.
2. **Go bridge enforcement** — routes writes only to the authenticated
   user's subtree. Refuses writes to `app`, `assets`, `auth`, `tokens`.
   Token-to-device matching prevents token theft.
3. **Watcher validation** — app watchers mediate between user data and
   shared data. A user intent does not directly modify shared records.

**Reference auth scheme:** The boilerplate ships with Argon2id password
hashing, CSPRNG token generation, and standard login/signup/logout watchers.
Apps needing OAuth, SAML, SSO, or other schemes replace the login watcher;
the token handoff and security boundary are unchanged.

No MBL interpreter changes are required for the identity model. It is
implemented entirely in the Go bridge layer.

> 🚧 **The current Go bridge routes through session-ID-keyed paths and does
> not yet implement the device/token/identity model. Converting the bridge
> is a dedicated development phase — see DEVPLAN.md.**

###### Outbound HTTP Requests

For calling external APIs and third-party services, outbound HTTP procedures
are available:

```
my.computer.network.web.get(url)
my.computer.network.web.get(url, options)
my.computer.network.web.post(url, body)
my.computer.network.web.post(url, body, options)
my.computer.network.web.put(url, body)
my.computer.network.web.put(url, body, options)
my.computer.network.web.patch(url, body)
my.computer.network.web.patch(url, body, options)
my.computer.network.web.delete(url)
my.computer.network.web.delete(url, options)
```

Options record:

| Option | Type | Default | Meaning |
|--------|------|---------|---------|
| `headers` | Record | (none) | Request headers (Text → Text) |
| `timeout` | Number | 30000 | Milliseconds before giving up |
| `follow_redirects` | Boolean | true | Follow HTTP redirects |

Response record:

| Attribute | Type | Meaning |
|-----------|------|---------|
| `.status` | Number | HTTP status code (200, 404, etc.) |
| `.body` | Text | Response body |
| `.headers` | Record | Response headers |
| `.time` | Time | When the response was received |
| `.duration` | Number | Round-trip time in milliseconds |

Network failures return Unknown: `unknown("dns_failure")`,
`unknown("connection_refused")`, `unknown("timeout")`,
`unknown("tls_error")`.

Example — calling an external shipping API from a watcher:

```
watch my.fulfillment.ship(my.orders[status = "confirmed"]):
    for order in my.orders[status = "confirmed"]:
        result = my.computer.network.web.post(
            "https://api.shipper.com/shipments",
            my.computer.network.web.to_json(order)
        )
        order.tracking = my.computer.network.web.parse_json(result.body)
        order.status = "shipped"
```

**JSON helpers:**
```
my.computer.network.web.parse_json(text)        # JSON text → tree node
my.computer.network.web.to_json(node)           # Tree node → JSON text
```

###### Inbound Request Queue (External Traffic)

For traffic from external systems — third-party webhooks, OAuth callbacks,
REST APIs consumed by external clients — HTTP requests arrive as nodes
appended to the request queue. Watchers process them using the append watcher
pattern.

This is NOT used for PWA client communication. PWA clients use the data-driven
approach described above.

```
my.api.orders: watch append(my.computer.network.web.requests[
    method ?= "GET", path starts "/api/orders/"
]) as reqs:
    for req in reqs:
        order_id = substring(req.path, 12, length(req.path))
        order = my.orders[id = order_id]
        my.computer.network.web.ok_json(req, order)
```

Request node attributes:

| Attribute | Type | Meaning |
|-----------|------|---------|
| `.domain` | Text | Request hostname |
| `.method` | Text | HTTP method (GET, POST, etc.) |
| `.path` | Text | Request path |
| `.query` | Record | Query parameters |
| `.headers` | Record | Request headers |
| `.body` | Text | Request body |
| `.remote_ip` | Text | Client IP address |
| `.time` | Time | Arrival time |

**Response helpers:**
```
my.computer.network.web.ok(req, body)           # 200 response
my.computer.network.web.ok_json(req, node)      # 200 with JSON body
my.computer.network.web.not_found(req)          # 404
my.computer.network.web.bad_request(req, msg)   # 400
my.computer.network.web.server_error(req, msg)  # 500
my.computer.network.web.redirect(req, url)      # 302
```

Example — handling an OAuth callback:

```
watch append(my.computer.network.web.requests[
    path ?= "/auth/google/callback"
]) as reqs:
    for req in reqs:
        token = my.computer.network.web.post(
            "https://oauth2.googleapis.com/token",
            "code=" & req.query.code & "&client_id=..."
        )
        # create or update user identity and token
        my.computer.network.web.redirect(req, "/dashboard")
```

Example — receiving a webhook:

```
watch append(my.computer.network.web.requests[
    method ?= "POST", path ?= "/webhooks/stripe"
]) as reqs:
    for req in reqs:
        event = my.computer.network.web.parse_json(req.body)
        if event.type = "payment_intent.succeeded":
            my.orders[event.data.order_id].payment_status = "paid"
        my.computer.network.web.ok(req, "received")
```

Requests have a configurable TTL (default 5 seconds). If no watcher responds
before the TTL expires, the Go layer returns a 503 to the client.

###### SSE (Server-Sent Events)

For PWA clients, SSE is managed automatically by the Go layer as part of the
data-driven communication model — the developer does not interact with SSE
directly.

For external SSE consumers (third-party systems subscribing to event streams),
publish events by writing to SSE paths:

```
my.computer.network.web.sse.price_updates = world.market.price
```

External clients subscribe via `GET /sse/{stream_name}`. One event per path
per tick. Last value wins within a tick.

###### PWA Static Asset Serving

Static assets are served directly by the Go layer from an in-memory cache.
No MBL code is involved in serving static files — this is handled entirely
at the Go level for maximum performance.

```
my.computer.network.web.pwa["app.example.com"].enabled = true
my.computer.network.web.pwa["app.example.com"].spa_mode = true
my.computer.network.web.deploy_pwa("app.example.com", "/build")
```

`deploy_pwa` reads all files recursively from the directory, wraps each as
an asset with inferred MIME type, and writes them to the mesh in a single
staged commit. The in-memory cache serves assets with sub-millisecond
response times, refreshed within one heartbeat tick when assets change.

Individual assets:
```
my.computer.network.web.pwa["app.example.com"].assets["style.css"] =
    my.computer.network.web.asset(my.computer.files.read("/build/style.css"), "text/css")
```

`my.computer.network.web.asset(data, mime_type)` constructs an asset record
with `.data` and `.mime_type` fields.

SPA fallback order:
1. Exact asset match in cache → serve asset
2. SSE path → hand off to SSE handler
3. No match, `spa_mode = true` → return `index.html`
4. No match, `spa_mode = false` → 503

**TLS configuration** is set in `amorphd.toml`, not in MBL. Certificates are
stored per domain with SNI routing.

> 🚧 **The network sub-library is not yet implemented in code.** The path
> structure exists with stubbed procedures. The files sub-library is partially
> implemented (XML import/export and ARI fixed-width import are working;
> JSON, CSV, TSV, TOML are stubbed).

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

1. The node enters maintenance mode and temporarily transfers write authority
   for its hosted paths to subscriber nodes.
2. Values are relocated to fill gaps, working from the end of the file toward the beginning.
3. Instance records are updated with new value offsets as values move.
4. Free space is consolidated at the end and the file is truncated.
5. The node rejoins the mesh and resyncs — subscriber nodes send all instances
   written during maintenance, which append cleanly to the freshly compacted files.
6. The node resumes write authority for its paths.


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

**Embed** is a structural composition mechanism that makes the attributes of one
record appear as siblings within another record, without copying or merging them.
The embedded record retains its identity as a distinct unit — its attributes are
presented as flat alongside the host record's own attributes, but are stored
separately and remain attributable to the embedded record as a coherent group.

#### How the Stamp System Uses Embed

Every value written by an agent automatically carries the agent's stamp record as
an embed. This means stamp attributes (`@stamp.project`, `@stamp.department`, etc.)
appear alongside the value's own attributes and are queryable like ordinary
attributes — but they are not absorbed into the value itself and can be updated
independently by updating the stamp record.

This is why stamps are ordinary attributes rather than meta attributes: they are
real, agent-defined, queryable data that need to travel with the value as a unit,
while remaining distinct from the value's own structure.

#### User-Facing Syntax

Programmers can use embed directly in record bodies with the `embed` keyword:

```
my.report.data:
    title: "Q3 Report"
    embed my.context.project_stamp
    revenue: $14200
```

`embed` is a statement-level keyword, not an assignment. It may appear anywhere in
a record body. Multiple embeds are permitted.

The spread form is accepted as an equivalent alternative for programmers familiar
with that convention:

```
my.report.data:
    title: "Q3 Report"
    ...my.context.project_stamp
    revenue: $14200
```

#### Collision Rule

When an embedded record's attribute name conflicts with a host record's explicitly
declared attribute, the **host attribute wins**. When two embeds conflict with each
other, the **later embed in declaration order wins**.

#### What Embed Is Not

Embed is not a value. You cannot assign an embed to an attribute, return one from
a procedure, or pass one as a parameter. It is a composition directive that affects
how a record is presented and stored, not a data value in its own right.

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

When a filter is active, any data matching the filter criteria returns `unknown("filtered")`. The agent can remove filters to see the full data, but cannot use filters to access data they lack permissions for.

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
- **Agent reference** (e.g., `(link)world.agent.kalevo`) — specific agent
- **Agent list** (e.g., `[(link)world.agent.alice, (link)world.agent.bob]`) — multiple agents
- **"Anything"** — unrestricted access
- **"Nothing"** — no access (effectively private)

**Permission examples:**
```
# Private data — only the owner can access
my.secrets.password.@read = (link)world.agent.kalevo
my.secrets.password.@write = (link)world.agent.kalevo

# Team collaboration — multiple agents can read and write
world.projects.alpha.@read = "Anything"
world.projects.alpha.@write = [
    (link)world.agent.alice,
    (link)world.agent.bob,
    (link)world.agent.charlie
]

# Public read, restricted write
world.announcements.@read = "Anything"
world.announcements.@write = (link)world.agent.admin
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
- Home: Stored in mesh, replicated across subscriber nodes
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
# Node receives path directory and joins the mesh
```

**Join Process:**
1. Connect to seed node with post-quantum encryption
2. Discover mesh name during handshake
3. Receive assigned identity in target mesh
4. Receive a snapshot of the path directory from the seed node
5. Register as a node in the mesh via gossip
6. Begin receiving write authority assignments and subscription requests

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
- Transferring write authority for hosted paths to subscriber nodes
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

### Data Distribution Model

> 🚧 **The subscription-based distribution model described here replaces the
> previous zone/consistent-hashing model.** The old model assigned each node a
> fixed slice of the keyspace via consistent hashing, with nodes becoming
> authority and replica for whatever zones their hash position covered. The new
> model is subscription-based with explicit write authority per path. This is
> a significant architectural change that requires a dedicated development phase.
> See the development plan for migration details.

#### Write Authority

Every path in the tree has exactly one **write authority** — the node responsible
for accepting and committing writes to that path. All writes to a path are routed
to its authority. The authority processes writes, timestamps them, and pushes
updates to all subscribers.

Write authority is not determined by hashing. It is assigned explicitly and can
be transferred. The mesh maintains a directory of path-to-authority mappings,
distributed across nodes via the gossip protocol.

**Authority splitting:** When a write authority becomes overwhelmed with writes
to a large subtree, it may delegate authority over a sub-path to another node.
For example, an authority for `world.market` that is overloaded may delegate
`world.market.equities` to another node while retaining authority over
`world.market.bonds` and `world.market.fx`. The delegated node becomes the
new authority for that sub-path and its descendants.

**Authority promotion:** When a write authority goes offline, a new authority
is elected from among the current subscribers to that path. Because subscribers
maintain a current copy of the data, promotion is fast — no data migration is
needed, only a role change. The promotion process:

1. Subscribers detect the authority is unresponsive (missed heartbeats)
2. Subscribers elect a new authority among themselves (highest uptime wins as
   tiebreaker)
3. The new authority announces itself via gossip
4. Other nodes update their directory entries
5. Buffered writes that did not reach the old authority are replayed

**Voluntary delegation:** An overwhelmed authority may proactively delegate
write authority to a subscriber rather than waiting for overload to become
critical. The subscriber already has current data and can assume authority
immediately.

#### Subscription Model

Reads do not route to a write authority. Instead, nodes subscribe to the paths
they need and maintain local copies. A node that has subscribed to a path
receives pushed updates from the authority on every heartbeat tick in which
that path changed.

**How subscriptions are established:**

- When a watcher or procedure is stored at a path, the node that hosts it
  automatically subscribes to all paths that code reads. On first execution,
  the runtime observes which paths are accessed and registers subscriptions
  for them. Subsequent executions read from the local subscribed copy.
- A node may also explicitly subscribe to a path for redundancy, caching,
  or local read performance.
- Any node may request that another node subscribe to its data — useful for
  ensuring redundancy when a path has few natural subscribers.

**Subscription load balancing:** When a write authority accumulates more
subscribers than it can efficiently push updates to, it may delegate push
responsibility to a subscriber — that subscriber re-fans updates to a subset
of the original subscriber pool. This keeps per-authority fan-out bounded
regardless of how many nodes subscribe to popular paths.

**Reads from subscribed data** are served locally with no network round-trip.
If a node has not yet subscribed to a path it needs, the first read routes to
the write authority and the subscription is established in the background.

#### Path Directory

The mesh maintains a distributed directory mapping paths to their current write
authority. This directory is propagated via the gossip protocol and cached on
every node. It is eventually consistent — a node may briefly route to a stale
authority after a promotion event, but the receiving node will redirect to the
correct current authority.

#### Temporal Data

Historical instances (older data beyond a configurable recency window) may be
migrated to archival nodes. The `@older_instance_id` pointer on the boundary
instance serves as a redirect to the archival location. Archival nodes are
subscribers that specialize in storing historical data rather than serving
current reads.

### Distributed Computation

Watchers and procedures execute on the node that hosts them. When a node hosts a
watcher, it subscribes to all paths that watcher reads — so the data the watcher
needs is available locally at execution time. Computation moves toward the data
through the subscription mechanism rather than through explicit placement.

A chain of watchers across different nodes becomes a distributed pipeline. Each
watcher executes on its host node, reading from locally subscribed data and
writing through the write authority for its target paths:

    # watcher hosted on node_A, subscribed to world.market.raw
    my.automation.ingest: watch(world.market.raw):
        world.market.processed = transform(world.market.raw)

    # watcher hosted on node_B, subscribed to world.market.processed
    my.automation.analyze: watch(world.market.processed):
        world.reports.daily = analyze(world.market.processed)

    # watcher hosted on node_C, subscribed to world.reports.daily
    my.automation.distribute: watch(world.reports.daily):
        world.notifications.send_report(world.reports.daily)

Each heartbeat propagates changes through the pipeline. The mesh is the execution engine.

### Locality

Because subscriptions are established based on what code actually reads, related
data naturally gravitates to nodes that use it. A node hosting many watchers over
`world.departments.engineering.*` will subscribe to that subtree and maintain a
local copy, effectively co-locating computation and data without requiring
explicit placement decisions.

Broadly shared data (configuration, reference data, shared libraries) will
accumulate many subscribers across the mesh. The subscription load balancing
mechanism handles fan-out automatically, preventing any one authority from
becoming a bottleneck for popular paths.

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

### Data Extract

`amorphctl extract` generates an MBL script that recreates the current state of
a subtree. The output is human-readable MBL code that can be reviewed, edited,
and run on any AmorphDB instance to reproduce the data structure.

```bash
# Extract an entire application subtree
amorphctl extract world.knowledgefoyer -o knowledgefoyer.mbl

# Extract to stdout
amorphctl extract world.knowledgefoyer

# Extract a smaller subtree
amorphctl extract world.knowledgefoyer.config -o config_only.mbl
```

The generated script includes:
- All attribute values with their current data
- Record structure and list contents
- Heritability modifiers (`(copy)`, `(link)`, `(reset)`, `(exclude)`)
- Procedure and watcher definitions
- Permission settings
- Stamp configurations
- Embed directives

The script does NOT include temporal history. History is an immutable audit
trail — extracting and replaying it would undermine its trustworthiness. The
extract captures what exists now, not how it got there.

**Restoring** is just running the extracted MBL script:

```bash
amorph knowledgefoyer.mbl
```

**Common use cases:**
- Back up application state before a software upgrade
- Move a data structure from one instance to another
- Review or edit a data structure as text
- Create a baseline template for new deployments
- Share a data structure definition with another developer

### PWA Management Commands

```bash
# Create a new PWA project
amorphctl init-pwa myapp ~/projects/myapp/

# Deploy project files to AmorphDB
amorphctl deploy-pwa myapp.example.com ~/projects/myapp/

# Enable/disable a PWA
amorphctl enable-pwa myapp.example.com
amorphctl disable-pwa myapp.example.com
```

`init-pwa` creates a starter project directory with `index.html`, `app.html`,
`app.js`, and `style.css`. `deploy-pwa` reads the project files, stores them as
assets in the mesh, and parses `app.html` to build the shared app structure.

The boilerplate JavaScript (`amorphdb-pwa.js`) and default styles are embedded
in the `amorphd` binary and served at `/amorphdb/pwa.js` and `/amorphdb/pwa.css`.
All PWAs on the instance share the same boilerplate version.

> See `AmorphDB_PWA_Boilerplate_Spec.md` for the full PWA specification.

### The Client

The client (`amorph`) is the primary interface for humans and external programs to
interact with AmorphDB. It supports two modes of operation.

**Interactive mode** provides a REPL (Read-Eval-Print Loop) for ad hoc queries,
data exploration, and immediate operations. The display format is inferred
automatically from what is returned — no mode-switching required:

- A **scalar value** (no sub-attributes) prints as a value with its timestamp
  and writing agent shown as context
- A **record** (named sub-attributes) prints as a fielded key-value block; if
  the node also has a scalar value of its own, that value is shown first
- A **list** (numerically indexed, homogeneous sub-attributes) prints as a table
  with automatic column headers

```
AmorphDB> my.account.balance
$1,247.83  (@2026-03-28 14:22:01 by kalevo)

AmorphDB> my.account
value:       "checking"
balance:     $1,247.83
status:      "active"
opened:      @2019-04-12

AmorphDB> my.transactions
  #   date          type      amount      balance
  0   @2026-03-28   debit     $42.50      $1,247.83
  1   @2026-03-27   credit    $2,100.00   $1,290.33
  2   @2026-03-25   debit     $18.99      ...

AmorphDB> world.market.price.@time
@2026-01-15 14:32:18
```

Display hints can override the inferred format:

```
AmorphDB> my.account :tree        # force tree view
AmorphDB> my.transactions :table  # force table view
AmorphDB> my.transactions :list   # force list view
```

Pagination uses slice notation for large collections:

```
AmorphDB> my.transactions[0:9]    # first ten items
AmorphDB> my.transactions[10:19]  # next ten
```

> 🚧 **Display hints (`:tree`, `:table`, `:list`) and slice pagination are not
> yet implemented.** Basic scalar and structured output is implemented. Full
> auto-inferred table rendering for homogeneous lists is planned.

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
    $ amorph --node 192.168.1.50:5000      # connect to remote node
    $ amorph --identity kalevo             # authenticate as specific agent

By default, the client connects to the local node's socket. For remote connections,
an address is specified and authentication proceeds via challenge-response (see
Security). If no identity is specified, the client uses the identity stored in the
local configuration.


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

An agent's secrets — private keys, sensitive data — are stored in the mesh but
encrypted with the agent's own derived key. The node hosting the agent's data
stores this data but cannot read it. Only the authenticated agent can decrypt it.

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
Each node heartbeats to a small set of direct peers — nodes it holds write
authority for, nodes it subscribes to, and a handful of gossip peers for
broader mesh awareness. This keeps per-node connection count bounded regardless
of mesh size.

**Authority heartbeats** are direct and immediate — writes are pushed to
subscribers within one heartbeat tick. This is the hot path for data consistency.

**Gossip heartbeats** propagate mesh-wide information (new nodes, departed nodes,
authority promotions, path directory updates) through neighbors. This information
may take several ticks to reach the entire mesh, which is acceptable for metadata
that changes infrequently.

#### Routing

When a node receives a write request for a path it does not hold authority for,
it consults its local copy of the path directory and forwards the request to the
current authority. The response returns along the same path. The client does not
need to know the mesh topology.

Read requests are served locally if the node has a current subscribed copy of the
data. If not, the read is forwarded to the write authority and a subscription is
established in the background.

**Bridge Routing:** Bridge nodes maintain separate path directories for each mesh
they participate in. Requests are routed within the appropriate mesh based on the
data path being accessed.

#### Extensibility

The protocol is designed to support future extensions:
- New message types can be added without breaking existing nodes
- Version negotiation allows mixed-version meshes during upgrades
- Optional features can be negotiated per connection
- Bridge protocol extensions support multi-mesh scenarios

---

## Future Enhancements

### MCARS — Standard Data Interface Convention

**MCARS** (inspired by the LCARS interface from Star Trek, with improvements) is
AmorphDB's standard visual interface convention. It defines a set of layout,
navigation, and field presentation rules that PWA and client applications built
on AmorphDB can follow to give users a consistent experience across different
applications.

MCARS covers:
- Standard external frame (header with time, current data path, user identity,
  and help access; content area; footer)
- Standard internal frame (message routing via `send`/`reception`, MCP-based
  communication between client and server)
- Generic section layout (hamburger menu, section name, focus radio, field
  presentation rules per value type)
- Field rendering by type: text, time, money, picture, boolean, watcher,
  procedure

**WAGLE notation** is a companion design language for specifying custom
interfaces within the MCARS convention, allowing developers to override the
generic presentation with custom layouts for specific data views.

> 📋 **MCARS and WAGLE are design concepts.** Full specifications will be
> developed as separate documents. No implementation exists yet.

### PWA Standard Component

A standard PWA shell for building applications on AmorphDB. The boilerplate
provides five primitives — proxied data cache, batch sender, SSE receiver,
local watchers, and deep link mapper. Everything else is application logic.

- Vanilla HTML/CSS/JavaScript (no framework dependencies)
- Application structure defined in `app.html` using `<section>` tags
- Data-driven communication — PWA clients interact through data changes, not HTTP requests
- Device-based identity before login, token-based identity after login
- Per-user data at `world.apps.<app>.users.<identity>` with per-user watchers
- Reference password auth scheme (Argon2id) with swappable login watchers
- Surgical DOM updates via `data-bind` attributes
- SPA routing with deep-linkable URLs
- AI helper operates through the same client interface with the same permissions

> 📋 **PWA boilerplate is specified in a separate document.** See
> `AmorphDB_PWA_Boilerplate_Spec.md`.

### Examples Library

A comprehensive set of MBL examples covering every language feature, organized
for use as both a tutorial and a reference. Examples will be cross-referenced
from the language specification sections.

> 📋 **Planned after core language implementation is stable.**

### LLM Fine-Tuning for AmorphDB

A fine-tuned language model trained to understand MBL syntax and semantics,
ARI notation, MCARS/MCP patterns, and AmorphDB architecture. The goal is to
provide AI assistance to developers and users working with AmorphDB — helping
write, debug, and design MBL programs, generate ARI specs from file samples,
and interact with MCARS interfaces.

> 📋 **Planned as a later-stage initiative once the specification and examples
> library are stable enough to generate quality training data.**
