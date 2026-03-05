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
    - @03:00:00.0 (3 hours exactly)
  If latter parts are omitted, the time literal has reduced precision rather than
  defaulting to zeros. `@2026-02-01` represents the entirety of February 1st, not
  midnight. This affects comparison behavior (see Comparison operators).
- Money
  A number with associated currency type.
  In MBL, currencies are prefixed with the "¤" symbol (universal symbol for any currency) followed by a number followed by the currency type.
  For convenience, if the currency is omitted then USD is default.
  Further, if the number is prefixed by a specific currency symbol then the currency suffix is unnecessary.
  Examples:
    - ¤19.95 USD
    - ¤23.45 (USD is assumed)
    - $29.95 (USD is understood)
- Picture
  Normalized 2D image stored as raw RGBA with 16-bit per channel.
  Every picture is the same format — grayscale is represented as equal R, G, B values,
  and fully opaque pixels simply have maximum alpha.
  The value consists of a width, height, and pixel data in row-major order.
- Reference
  Interpretable text to other data.
- Procedure
  A callable block of code with optional parameters and default values.
  The implementation is stored in the `@code` meta attribute.
  Sub-attributes of a procedure serve as its persistent local data scope.
- Watcher
  A reactive block of code that executes when monitored values change.
  The implementation is stored in the `@code` meta attribute.
  Sub-attributes of a watcher serve as its persistent local data scope,
  and include `enabled` (defaults to true) and `watching` (list of monitored paths).
- Embed
  A reference to another record whose attributes are projected into the referencing
  record as if they were native. The embedded record is a frozen snapshot — changes
  to the source after embedding are not reflected.

### Meta Types

- Nothing
  Any reference to data that doesn't exist will return this.
  Assigning this effectively eliminates any value.
  Nothing is the absence of a value, distinct from an empty text string (see `empty` under Text Constants).
- Unknown
  This is a value intended to be used for tertiary operations, similar to NULL in SQL.
  However, as any value may have sub-attributes in AmorphDB, the Unknown may have the following:
  - reason
    A textual indicator of why this is unknown.
  - options
    A numerically indexed list of things it might be.
  Other custom attributes may be provided.
  The purpose is to make dealing with unknowns more intelligent where possible or desirable.
- Anything
  The only value that will not match this is Nothing.
  Anything has uses such as determining if a value exists or not.

### Structures

There is only one underlying structure — a node in the storage system. MBL provides two views of it, differentiating how that structure is treated.

- Record
  A node in the storage system.
  In MBL, a record has attributes indexed by name (textually).
  Created with brace syntax: `myrec = { x: 1, y: 0, name: "coordinates" }`
- List
  A node in the storage system.
  In MBL, a list has attributes indexed numerically.
  Created with bracket syntax: `mylst = ["apple", "orange", "banana"]`

### Instance Meta Attributes

Attributes whose labels begin with `@` are meta attributes — they describe properties of the instance itself rather than user data. Meta attributes are stored in the same attribute chain as normal attributes but are hidden from normal enumeration and cannot collide with user-defined names.

The `@` symbol unifies all instance-level concerns: `@` alone or `@timestamp` refers to when the instance was written, while named meta attributes like `@inherit` describe how the instance behaves.

The system automatically injects the `@author` meta attribute on every instance at write time, recording the identity of the agent that performed the write. This is a system-level guarantee — no user code can suppress or alter it.

#### Heritability

When a new record is instantiated from an existing one, each attribute's inheritance behavior is controlled by the parent. This ensures the parent protects its own contract — children cannot accidentally expose excluded fields or break linked relationships.

Inheritance behavior is declared inline using a parenthetical modifier between the definition operator `:` and the value:

    name:(copy) "Joe"                        # explicit copy
    name: "Joe"                              # same — copy is the default
    quantity:(link) 0                        # linked to parent
    id:(reset random(100, 999)) 342          # reset with expression
    secret:(exclude)                         # omitted from children

The modifier can also appear in standalone assignment: `myvar = (link) yourvar`

| Modifier | Behavior |
|----------|----------|
| *(none)* | Default: child gets a new instance pointing to the same value. Independent going forward. |
| `(copy)` | Explicit form of the default. |
| `(link)` | Child stores a Reference to the parent's attribute. Reads follow the reference; writes propagate to the parent. |
| `(reset <expr>)` | Child gets a new value by evaluating the expression. |
| `(exclude)` | Attribute is not created in the child. |

These modifiers are stored as instance meta attributes (`@inherit` and `@reset`) in the underlying storage. The inline syntax is shorthand for setting them.

The `@reset` meta attribute holds the reset expression. Beyond inheritance, it is also available as a general-purpose default that can be invoked explicitly to restore an attribute to its defined starting state.

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

    result = mytext..trim(" ")..upper..replace("old", "new")

#### Text Operations

    txt..length                                 # number of characters
    txt..find(needle, start = 0)                # position of first match, Nothing if not found
    txt..extract(from_num, thru_num)            # substring by position
    txt..extract(after_txt, until_txt)          # substring by text markers
    txt..replace(old_txt, new_txt, start = 0)   # replace all occurrences of text
    txt..replace(from_num, to_num, new_txt)     # replace by position
    txt..split(separator = ",")                 # split into a list
    txt..trim(chars = " ", side = "both")       # strip characters ("both", "left", or "right")
    txt..pad(total, chars = " ", side = "right") # pad to exact total length ("left" or "right")
    txt..upper                                  # uppercase
    txt..lower                                  # lowercase
    txt..title                                  # titlecase

**`extract` with text markers** returns the text between the first occurrence of the opening and closing markers. If markers are not found, returns Nothing. If multiple matches exist, returns a list of all matches.

**`split`** preserves empty segments. `"a,,b"..split(",")` returns `["a", empty, "b"]`. The `empty` value represents a zero-length text segment, distinct from Nothing.

**`pad`** repeats the `chars` pattern and truncates to hit the exact total specified. `"x"..pad(9, "-+")` produces `"x-+-+-+-+"`.

#### Record and List Operations

Lists and records are the same underlying structure — a node with attributes. Lists are indexed numerically, records by name. The following operations modify in place.

    x..count                                    # number of attributes under this node
    x..combine(separator = ",")                 # join values into text
    x..remove(position)                         # remove by numeric index (list reindexes)
    x..remove(value)                            # remove first match by value (list) or by name (record)
    x..remove(from, to)                         # remove a range by index
    x..reverse                                  # reverse current order
    x..sort(order = "ascending")                # alphanumeric sort ("ascending" or "descending")
    x..sort(procedure)                          # custom sort
    rec..inject(template, opener = "{", closer = "}")  # fill template from record attributes

List-specific operations (numeric indexing):

    lst..append(value)                          # add to end
    lst..prepend(value)                         # add to beginning, shift indexes
    lst..insert(position, value)                # insert at position, shift indexes

**`remove`** by index differs from assigning Nothing — `mylist[3] = Nothing` leaves a gap, `mylist..remove(3)` removes the element and reindexes subsequent items. Range removal `mylist..remove(3, 5)` removes indexes 3 through 5 and reindexes.

**`sort` with a procedure** follows the standard comparator pattern. The procedure receives two arguments and returns a negative number, zero, or a positive number:

    by_quantity(a, b):
        return a.quantity - b.quantity

    my.inventory..sort(by_quantity)

**`inject`** replaces placeholders in the template with matching attribute values from the record. If a placeholder references an attribute that does not exist, it is replaced with Nothing's text representation.

#### Universal Operations

    x..type                 # returns the type name: "Text", "Number", "Time",
                            # "Money", "Picture", "Reference", "Procedure",
                            # "Watcher", "Embed", "Nothing", or "Unknown"

Explicit type conversion is rarely needed — the operator-driven coercion system handles most cases. Arithmetic (`+`) coerces to number, concatenation (`&`) coerces to text. When explicit conversion is required, the operators serve as converters: `mytext + 0` converts text to number, `empty & myvalue` converts any value to text.

#### Text Constants

The following keywords produce text values that cannot be typed as literals:

    quote                   # the " character
    tab                     # tab character
    newline                 # newline character
    empty                   # zero-length text, distinct from Nothing

#### Character Lookup

    symbol(65)              # decimal: produces "A"
    symbol("0x41")          # hexadecimal: produces "A"
    symbol("U+0041")        # unicode notation: produces "A"

`symbol` accepts a numeric value or a text representation of a code point and returns the corresponding character.

#### Number Operations

    num..round(places = 0)          # round to N decimal places
    num..floor                      # round down to integer
    num..ceiling                    # round up to integer
    num..truncate                   # drop decimal, toward zero
    num..abs                        # absolute value
    num..sign                       # returns -1, 0, or 1
    num..min(other)                 # smaller of two values
    num..max(other)                 # larger of two values
    num..clamp(low, high)           # constrain to range
    num..power(exp)                 # exponentiation
    num..root(n = 2)                # nth root (default square root)
    num..log(base = 10)             # logarithm
    num..sin                        # sine (radians)
    num..cos                        # cosine (radians)
    num..tan                        # tangent (radians)
    num..asin                       # arcsine
    num..acos                       # arccosine
    num..atan                       # arctangent

#### Numeric Constants

    pi                      # 3.14159265...
    euler                   # 2.71828182... (Euler's number, e)

#### Random

    random(min, max)        # returns a random number in the range [min, max]

### Control Flow

#### Conditionals

An `if` block evaluates a condition and executes the associated body. An optional `else if` chain and final `else` provide alternatives. If the body is a single statement, it may appear on the same line after the colon. Otherwise, an indented block on the next line is expected. The `pass` keyword serves as an explicit no-op for empty blocks.

    if x ?= 5:
        my.computer.output("five")
    else if x > 10:
        my.computer.output("big")
    else:
        pass

    if z > 1: my.computer.output("positive")

#### Consider

The `consider` block evaluates a value against a series of conditions. It short-circuits on first match — once a condition is satisfied, the remaining conditions are skipped. An `else` clause catches anything that did not match.

Inside a `consider` block, each `if` tests against the considered value. A bare value defaults to equality (`?=`). A comparison operator may be prefixed for other comparisons.

    consider status:
        if "active": my.computer.output("Active")
        if "pending": my.computer.output("Pending")
        else: my.computer.output("Other")

    consider age:
        if < 13: my.computer.output("child")
        if < 20: my.computer.output("teenager")
        if >= 20: my.computer.output("adult")

Multiple values can share a condition using `or`:

    consider code:
        if "a" or "b" or "c": my.computer.output("early alphabet")
        if "x" or "y" or "z": my.computer.output("late alphabet")
        else: my.computer.output("middle")

There is no fallthrough. Each matching branch executes its body and the `consider` block ends.

#### While Loop

A `while` loop repeats its body as long as the condition is true.

    x = 0
    while x < 10:
        my.computer.output(x)
        x = x + 1

#### For Loop

A `for` loop iterates over the attributes of a record or list. The loop variable is bound to each attribute in sequence — it is the attribute itself, which means it is both the key and the entry point to everything beneath it.

    for item in my.inventory:
        my.computer.output(item)                     # the key (name or index)
        my.computer.output(item.quantity)            # sub-attributes are directly accessible
        my.computer.output(item.description)

Because the loop variable *is* the attribute, writes through it modify the actual data:

    for item in my.inventory:
        if item.quantity < 5:
            item.status = "low stock"

This is consistent with the storage model — an attribute is its label and its contents simultaneously. The loop variable behaves exactly like any other path element in the language, with no special accessor syntax or dereferencing required.

The loop variable also works naturally with concatenation and other operators:

    for item in my.inventory:
        my.computer.output(item & ": " & item.quantity)

### Procedures

A procedure is defined with a name, an optional parameter list in parentheses, a colon, and an indented body. If the body is a single statement, it may appear on the same line after the colon.

    # no parameters, no return value
    sayhi:
        my.computer.output("Hi!")

    # no parameters, returns a value
    get_pi:
        return 3.14

    # parameters
    add(a, b):
        return a + b

    # default parameter values
    greet(name, greeting = "Hello"):
        my.computer.output(greeting & " " & name)

Procedures are called by name. Parentheses are required if the procedure was defined with parameters:

    sayhi
    pi = get_pi
    x = add(5, 6)
    greet("Joe")                    # "Hello Joe"
    greet("Joe", "Hey")             # "Hey Joe"

A procedure that does not explicitly `return` a value returns Nothing.

Like watchers, a procedure's implementation is stored in the `@code` meta attribute, and its sub-attributes serve as persistent local data scope. This allows procedures stored in the hierarchy to maintain state across calls:

    my.procedures.counter.@code             # the implementation
    my.procedures.counter.total = 0         # persistent local data

    # the procedure can reference its own sub-attributes
    my.procedures.counter:
        .total = .total + 1
        return .total

### Watchers

A watcher monitors one or more values and executes its body when any of them change. It is defined with the `watch` keyword, a name, a list of watched values in parentheses, a colon, and an indented body.

    watch toobigx(x):
        if x > 10: x = (quietly) 10

    watch balance_check(my.account.balance, my.account.limit):
        if my.account.balance > my.account.limit:
            my.account.status = (quietly) "overlimit"

A watcher is a value in the hierarchy like any other. It is operated by the node responsible for the zone where it lives. Its sub-attributes form its persistent local data scope.

#### Watcher Attributes

Every watcher has the following attributes:

    mywatcher.enabled               # true (default) or false
    mywatcher.watching              # list of monitored paths
    mywatcher.@code                 # the implementation (meta attribute)

The `enabled` attribute controls whether the watcher fires. The `watching` list contains the paths being monitored. The `@code` meta attribute holds the implementation, kept in meta space so it cannot collide with user-defined local data.

Additional sub-attributes may be used freely as persistent local storage for the watcher:

    my.watchers.stock_alert.last_run = world.clock.utc
    my.watchers.stock_alert.run_count = my.watchers.stock_alert.run_count + 1

#### Managing Watchers

Watchers are managed through normal data operations — no special API is required.

    # disable a watcher
    my.watchers.stock_alert.enabled = false

    # re-enable it
    my.watchers.stock_alert.enabled = true

    # add a path to the watch list
    my.watchers.stock_alert.watching[+] = my.warehouse.inventory

    # remove a path from the watch list
    my.watchers.stock_alert.watching[1] = Nothing

    # replace the implementation
    my.watchers.stock_alert.@code = (new implementation)

    # stop and remove the watcher entirely
    my.watchers.stock_alert = Nothing

    # query all disabled watchers
    for w in my.watchers:
        if w.enabled ?= false:
            my.computer.output(w & " is disabled")

All changes to `enabled`, `watching`, or `@code` take effect on the next mesh heartbeat. Because watchers are data in the hierarchy, permissions apply naturally — you cannot disable or modify another agent's watcher unless you have `@write` permission on it.

#### Quiet Assignment

Assigning a value normally triggers any watchers monitoring that value. The `(quietly)` modifier suppresses this — the value is written but no watchers fire.

    x = 10                  # triggers watchers on x
    x = (quietly) 10        # writes x without triggering watchers

This follows the same parenthetical modifier pattern used for heritability (`:(copy)`, `:(link)`, etc.). Spacing around the modifier is free-form: `=(quietly)10`, `= (quietly) 10`, and `=(quietly) 10` are all equivalent.

`(quietly)` is useful both inside and outside watchers. Inside a watcher, it prevents the watcher from retriggering itself when that is not desired. Outside a watcher, it allows bulk updates or corrections without causing a cascade of reactions.

Note that watchers *can* retrigger themselves — a watcher that modifies its own watched value without `(quietly)` will fire again. This is intentional. Some use cases require self-triggering watchers to drive iterative or looping processes. When self-triggering is not desired, `(quietly)` prevents it explicitly.

### Execution Model

#### Outer Run

An outer run is a program sent into the service by a client for execution. The program's local scope exists only in memory — it is not persistent. However, the `my` and `world` keywords reach into persistent storage in the mesh, allowing the program to read, write, inject procedures, set up watchers, or perform ad hoc operations.

When the program ends, its local scope is discarded. Anything it wrote to `my` or `world` persists.

    # outer run: inject a watcher into persistent storage, then exit
    my.watchers.stock_alert: watch stock_check(my.inventory):
        for item in my.inventory:
            if item.quantity < 5:
                item.status = "low stock"

    # outer run: ad hoc query
    for item in my.inventory:
        if item.status ?= "low stock":
            my.computer.output(item & ": " & item.quantity)

#### Inner Run

An inner run is code that executes within the mesh itself, driven by watchers. Watchers that have been persisted under `my` or `world` are alive in the mesh continuously — they fire in response to value changes and execute their bodies as inner runs.

Inner runs are the mesh's nervous system. There is no scheduler, no cron, no event queue — just watchers reacting to changes in the data they monitor, including `world.clock`.

The maximum execution speed of any inner run is one third of a second, matching the mesh heartbeat. This interval is the rate at which updates propagate across the mesh and is derived from the minimum time the human brain can distinguish between events.

#### Heartbeat Atomicity

Within a single heartbeat tick, a watcher's execution is atomic. All changes it makes to persistent data are held locally until the heartbeat completes. If the watcher produces an unhandled Unknown that escapes its body — indicating a failure such as a hardware error, network problem, or unresolvable operation — all changes from that tick are rolled back. None are replicated.

If the watcher handles the Unknown internally (checks for it, takes corrective action), that is normal operation and changes commit as expected. The rollback only occurs when an Unknown propagates out of the watcher unhandled.

This requires no distributed transaction coordination. Changes do not leave the node until the heartbeat boundary, so rollback is purely local — the node discards pending writes.

#### Cross-Zone Operations

Writes within a single zone are atomic within a heartbeat. When an operation spans multiple zones — such as transferring a value between two accounts on different nodes — true atomicity is not available. Instead, the recommended pattern uses a transaction record as the source of truth and watchers for self-healing.

**The pattern:**

1. Write a transaction record capturing the intent, marked as "pending."
2. Execute each step, updating the transaction status as steps complete.
3. A watcher monitors for transactions that remain incomplete beyond an expected time and either retries or reverses them.

Example:

    # record the intent
    my.transfers[+]:
        from = my.world.accounts.checking
        to = my.world.accounts.savings
        amount = 100
        status = "pending"
        created = world.clock.utc

    # a watcher processes pending transfers
    watch transfer_processor(my.transfers):
        for t in my.transfers:
            if t.status ?= "pending":
                t.from.balance = t.from.balance - t.amount
                t.status = "debited"
            if t.status ?= "debited":
                t.to.balance = t.to.balance + t.amount
                t.status = "complete"

    # a separate watcher monitors for stuck transfers
    watch transfer_health(my.transfers):
        for t in my.transfers:
            if t.status ?= "debited" and (world.clock.utc - t.created) > @00:00:10:
                if t.retries ?= Nothing or t.retries < 3:
                    t.retries = (t.retries + 0) + 1
                    t.status = "pending"
                else:
                    t.from.balance = t.from.balance + t.amount
                    t.status = "failed"

The transaction record captures every state change — the temporal model provides a complete audit trail. If a node fails between steps, the health watcher detects the stalled transaction and resolves it. No special syntax is required — this is ordinary MBL using records and watchers.

This mirrors how real financial systems operate: transactions are journaled, steps execute sequentially, and reconciliation processes catch failures.

### Scope

A running program (outer or inner) has its own in-memory tree using the same structures as persistent storage. At the root of this tree, two keywords provide links beyond local scope:

| Keyword | Destination |
|---------|-------------|
| `my` | The current agent's home in the mesh (`~`, i.e., `world.agent.{identity}`) |
| `world` | The global root of the persistent hierarchy (reachable as `my.world`) |

    program's in-memory tree:
        x = 5                       # local, in memory only
        temp = { a: 1, b: 2 }      # local record, in memory only
        my                          # → link to ~ (world.agent.{identity})
        world                       # → link to my.world

Local data uses the same types and structures as persistent data. The runtime needs no serialization boundary between them. Data becomes persistent when written under `my` or `world`:

    temp = { name: "Joe", age: 34 }    # local
    my.contacts.joe = temp              # now persistent in the mesh

When a program ends, anything not attached to `my` or `world` is discarded.

#### The Computer

`~.computer` (accessible as `my.computer` from program root) is a virtual mount in the agent's home that provides access to the local machine. It is not stored in the mesh — it is provided by the node the agent is connected from.

    my.computer.output(value)       # write to stdout (procedure)
    my.computer.input(prompt)       # read from stdin (procedure, returns text)
    my.computer.error(value)        # write to stderr (procedure)
    my.computer.files               # local filesystem
    my.computer.printers            # local printers
    my.computer.display             # screen/UI
    my.computer.network             # network interfaces

The structure under `my.computer` depends on the local machine — different hardware exposes different resources.

**Availability:** An outer run always has access to `my.computer` because it is running on a client connected to a local machine. A watcher (inner run) has access to `my.computer` only if it is executing on the node where it was created. If the watcher migrates to a replica during failover, `my.computer` resolves to Nothing.

**Bridging local resources to the mesh:** Physical resources can be made available to other agents by advertising them in the persistent hierarchy and using a local watcher to bridge between the mesh and the hardware:

    # advertise a printer in a public space
    my.world.services.printers.bldg3_hp:
        node = ~
        description = "HP Office - Building 3"
        available = true

    # local watcher bridges mesh requests to hardware
    watch bldg3_printer(my.world.services.printers.bldg3_hp.queue):
        for job in my.world.services.printers.bldg3_hp.queue:
            my.computer.printers.hp_office.print(job.document)
            job.status = "complete"

Because the bridging watcher runs on the local node, it has access to `my.computer`. If the node goes down for maintenance, the watcher stops and the resource becomes unavailable — reflecting the physical reality. Other agents can monitor service health through watchers of their own.

#### Scope Resolution

Bare references (no prefix) cascade upward through local scopes — inner blocks see outer variables. This follows the line prefix rule: a bare reference resolves to the nearest matching upstream scope.

In persistent storage, bare references do not cascade by default. Paths must be explicit. An attribute defined with `(cascade)` overrides this — its value becomes visible to bare references from descendant scopes in the persistent hierarchy.

The `~` sigil always resolves to the current agent's home in the mesh, regardless of scope depth. It works inside bracket expressions and any other context where `my` is not in scope.

#### Watcher and Procedure Scope

A watcher's sub-attributes are its persistent local data scope. The `.` prefix accesses these directly. The owning agent is the agent who created it, and `~` resolves to that agent's home. A watcher has access to `my.computer` only when executing on its originating node.

Procedures stored in the hierarchy work the same way — their sub-attributes are persistent local data accessible via `.` prefix.


## Data Storage and Retrieval System

The storage layer is built from three interlocking structures: attributes, instances, and values.

An **attribute** is a named or numbered entry in a set. Each attribute points to its first instance and to the next attribute in the same set, forming a linked list of siblings.

    attribute:
      - attribute_value_id    # the attribute's label (points to a value)
      - first_instance_id     # the most recent instance under this attribute
      - next_attribute_id     # the next sibling attribute in this set

An **instance** is a single assignment in time. Instances chain backward to form a temporal history, and each instance may have its own set of sub-attributes. This is the core of AmorphDB's temporal model — nothing is overwritten, every change is preserved.

    instance:
      - timestamp             # UTC UNIX time the value was written
      - value_id              # composite pointer: [4-bit bucket][60-bit offset]
      - older_instance_id     # the previous instance of this attribute
      - first_attribute_id    # first sub-attribute under this instance

The `@author` meta attribute is automatically injected by the system on every instance.

A **value** is the raw stored data. All values share a common header followed by type-specific content.

    value:
      - 0x1E                  # record separator (corruption guard)
      - type                  # determines the layout that follows

      If text:
        - length
        - literal UTF-8 content

      If number:
        - literal value (largest float supported by processor architecture)

      If time:
        - literal UNIX timestamp (2038-proofed)

      If money:
        - literal number value (same as number)
        - currency byte

      If picture:
        - width
        - height
        - pixel data (width × height × 8 bytes, row-major RGBA, 16-bit per channel)

      If reference:
        - length
        - literal UTF-8 content (interpretable path to other data)

      If embed:
        - reference to source record (frozen snapshot)

      If procedure:
        - parameter count
        - for each parameter: name (length + text), has_default flag, default_value_id
        - source length + source text
        - AST length + serialized AST

      If watcher:
        - watch count
        - for each watch: instance_id
        - source length + source text
        - AST length + serialized AST

### Storage File Organization

Each node maintains fixed-width files for attributes and instances, and bucketed files for values.

**Attributes file:** Each entry is a fixed-size record. Purged slots are immediately reusable.

**Instances file:** Each entry is a fixed-size record. Purged slots are immediately reusable.

**Value buckets:** Values are stored in bucketed files organized by size. This minimizes fragmentation by keeping similarly-sized values together, improving reuse of purged space.

The `value_id` in an instance record is a composite 64-bit pointer:

    value_id (64 bits):
        [4 bits: bucket] [60 bits: offset]

The top 4 bits identify the bucket, the remaining 60 bits are the offset within that bucket. This allows up to 16 buckets with no structural change to the instance record.

| Bucket | Contents | Storage |
|--------|----------|---------|
| 0 | Tiny (under 64 bytes — numbers, money, time, references) | Bucket file |
| 1 | Small (64–512 bytes — short text, small procedures) | Bucket file |
| 2 | Medium (512–4096 bytes — longer text, procedures) | Bucket file |
| 3 | Large (4096+ bytes) | Individual files in a folder |
| 4 | Pictures | Individual files in a folder |
| 5–15 | Reserved for future use | — |

For bucket files (0–2), the offset is a byte position within the file. For folder-based buckets (3–4), the offset is a file number within the folder. Folder-based storage has no fragmentation concern — each value is its own file.

Most values in typical use — numbers, short text, money, time, references — fall in the tiny bucket. This concentrates the common case in a single compact file with minimal wasted space and high reuse potential.

### Storage Optimizations

The base storage model prioritizes simplicity, but two low-complexity optimizations are expected:

**Hash indexes for attribute lookup.** The linked list of sibling attributes supports ordered traversal but makes lookup by name O(n). A hash table mapping attribute labels to attribute IDs provides O(1) direct access for the common case. The linked list is retained for enumeration.

**Value deduplication.** Values are content-addressed: on write, the value bytes (type + content) are hashed, and if an identical value already exists, the new instance points to it rather than storing a duplicate. This naturally extends the value-sharing that already occurs during inheritance.

To keep write latency flat, deduplication uses two tiers. Small values (numbers, short text, money, time, references) are deduplicated inline on write — the hash check is cheap relative to the I/O. Large values (long text, pictures, procedures) are written immediately and a background process scans for duplicates during low load, merging them and redirecting instance pointers. Some short-lived duplication is acceptable.

### Purge

Assignment to Nothing records a change — the value is gone but the history of its existence is preserved. Purge is a separate operation that permanently erases instances from history. It is the one operation that breaks the "nothing is overwritten" guarantee and is therefore restricted by the `@purge` permission.

#### Purge Audit

Even though purge erases data, the fact of the purge is preserved. A purge log records:

- What was purged (path and attribute)
- When the purge occurred
- Who authorized it (`@author` of the purge)

The data is gone but accountability for its removal is retained.

#### Storage Reclamation

Purged blocks are tombstoned in place using the `0x1F` byte (ASCII Unit Separator), complementing the `0x1E` (Record Separator) used for live records. The tombstone records the block size so the space can be reused.

A separate free list file provides fast lookup for allocation:

    free_list entry:
        - offset              # position in the file
        - size                # bytes available

On write, the service checks the free list first. If a block of suitable size exists, it is reused. Otherwise, the value is appended to the end of the file. The free list is sorted by block size for best-fit allocation.

The free list can be rebuilt from the main file by scanning for tombstones if it is ever lost or corrupted — the tombstoned blocks make the file self-describing.

#### Defragmentation

Attributes and instances are stored in fixed-width files. Every record is the same size, so purged slots are immediately reusable by new records. These files never need defragmentation.

Values are variable-size and stored in bucketed files. Over time, purging can leave scattered free blocks too small to reuse. A background defragmentation process consolidates free space during low load:

1. The node enters maintenance mode and replicas assume zone authority.
2. Values are relocated to fill gaps, working from the end of the file toward the beginning.
3. Instance records are updated with new value offsets as values move.
4. Free space is consolidated at the end and the file is truncated.
5. The node rejoins the mesh and resyncs — the replica sends all instances written during maintenance, which append cleanly to the freshly compacted files.
6. The node resumes authority and replicas return to replica mode.


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
| **Control** (`amorphctl`) | Admin tool. Connects via local socket. Start, stop, status, zone management, compaction. |
| **Client** (`amorph`) | User tool. Connects via local socket or network socket. Interactive REPL or script execution. |

The **service** is the only component that touches disk storage directly. All other components communicate with it through sockets. The local socket serves the control tool, local clients, and local resources such as devices. The network socket serves remote clients and mesh traffic between nodes. The protocol is the same over both, with encryption and authentication layered on for network connections (see Security).

A remote client connects to any node. If the requested data lives in a different node's zone, the connected node proxies the request transparently. The client does not need to know the mesh topology.

### The Client

The client (`amorph`) is the primary interface for humans and external programs to interact with AmorphDB. It supports two modes of operation.

**Interactive mode (REPL):**

    $ amorph
    amorph> my.contacts.joe.name
    "Joe"
    amorph> my.contacts.joe.age = 35
    amorph> for item in my.inventory:
         ..     if item.quantity < 5:
         ..         my.computer.output(item)
         ..
    Widgets (3)
    Sprockets (1)
    amorph>

The REPL detects indented blocks and continues with `..` until a blank line ends the block. Each entry is sent as an outer run. Output from `my.computer.output` appears in the terminal. Errors from `my.computer.error` appear on stderr.

**Script mode:**

    $ amorph run myscript.mbl
    $ cat myscript.mbl | amorph run

A `.mbl` file contains MBL source code. The file contents are sent as a single outer run. MBL scripts may also be made directly executable on Unix systems with a shebang line:

    #!/usr/bin/env amorph run

**Connection options:**

    $ amorph                                    # local socket (default)
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

### Node-to-Node Encryption

Every pair of nodes that communicates establishes its own encrypted channel. Keys are exchanged lazily — two nodes that have never communicated perform a key exchange the first time they need to, then cache the result. Over time, each node accumulates keys for the peers it actually talks to.

This means each node only holds keys for nodes it has communicated with, and each channel can rotate keys independently.

### External Agent Authentication

External agents (humans, AIs, programs) authenticate using two-factor derived keys. The agent's private key is not stored as a static artifact anywhere — it is derived at authentication time from multiple factors:

- Something the agent knows (a passphrase)
- Something the agent has (a device secret)

Neither factor alone is sufficient. A stolen device without the passphrase is useless. A compromised passphrase without the device is useless.

**Authentication flow:**

1. Agent connects to any node via the client.
2. Agent provides their identity.
3. The node looks up the agent's public key from the mesh (`world.agent.{identity}.keys.public`).
4. The node encrypts a random challenge with the public key.
5. The agent derives their private key from their factors, decrypts the challenge, and returns it.
6. The node confirms the identity.

The derived private key exists only in memory during the session. It is never stored permanently.

### Agent-Level Encryption

An agent's secrets — private keys, sensitive data — are stored in the mesh but encrypted with the agent's own derived key. The node hosting the agent's zone stores this data but cannot read it. Only the authenticated agent can decrypt it.

    ~.keys.public                   # in the mesh, readable by anyone
    ~.keys.private                  # in the mesh, encrypted with agent's derived key
    ~.secrets                       # in the mesh, encrypted with agent's derived key

This creates two layers of encryption: mesh-level encryption protects data in transit between nodes, while agent-level encryption protects secrets at rest from the hosting node itself.

### Recovery

If an agent loses access to their authentication factors (lost device, forgotten passphrase), a recovery mechanism based on Shamir's Secret Sharing allows account restoration without a central authority.

When an agent creates their identity, they designate trusted recovery agents and a threshold:

    ~.recovery:
        agents = [~.world.agent.miratu, ~.world.agent.senabo, ~.world.agent.tokeli]
        threshold = 2       # any 2 of 3 must agree

Each recovery agent receives a shard of a recovery key. No single agent holds enough to recover the account alone.

**Recovery flow:**

1. The agent connects to any node and requests recovery for their identity.
2. The node looks up the recovery configuration from the mesh.
3. The agent contacts their trusted agents through out-of-band means (phone, in person).
4. Each trusted agent who agrees submits their shard.
5. Once the threshold is met, the recovery key is reconstructed.
6. The recovery key decrypts `~.keys.private`.
7. The agent sets up new authentication factors (new passphrase, new device).
8. The old derived key is invalidated, secrets are re-encrypted with the new one.
9. Recovery shards are regenerated and redistributed to trusted agents.

The human element is the verification — when a recovery agent receives a request, they must decide through their own judgment whether it is legitimate. The system cannot and should not automate this.

#### Small Mesh Limitations

Recovery resilience scales with mesh size:

- **One node:** The node is the sole recovery agent. If it is lost and the agent loses access, recovery is not possible. This matches the inherent fragility of a single-node deployment.
- **Two nodes:** Each can serve as recovery agent for the other. Better, but simultaneous loss of both is unrecoverable.
- **Three+ nodes:** Full Shamir's Secret Sharing with meaningful thresholds. The system grows more resilient as the mesh grows.

### Protocol

#### Wire Format

All mesh communication uses a compact binary protocol. Each message has a fixed header followed by a variable-length self-describing payload.

    message:
        version             (1 byte — protocol version)
        type                (1 byte — message type)
        sequence            (4 bytes — request/response correlation)
        payload_length      (4 bytes)
        payload             (variable)
        checksum            (4 bytes)

Total overhead: 14 bytes per message.

The payload consists of tagged fields that can be parsed without knowing every field type. Unknown tags are skipped, allowing older nodes to handle messages from newer protocol versions gracefully.

    tagged field:
        tag                 (2 bytes — field identifier)
        length              (4 bytes)
        data                (variable)

#### Message Types

**Connection establishment:**

| Type | Name | Purpose |
|------|------|---------|
| 0x01 | `KEY_EXCHANGE` | Diffie-Hellman parameters, post-quantum upgrade |
| 0x02 | `IDENTITY_REQUEST` | New agent requests an identity |
| 0x03 | `IDENTITY_GRANT` | Identity and keys issued to new agent |
| 0x04 | `AUTH_CHALLENGE` | Encrypted challenge for authentication |
| 0x05 | `AUTH_RESPONSE` | Decrypted challenge proving identity |
| 0x06 | `PEER_EXCHANGE` | Share list of known nodes |

**Data operations:**

| Type | Name | Purpose |
|------|------|---------|
| 0x10 | `READ` | Request data at a path |
| 0x11 | `READ_RESPONSE` | Return requested data |
| 0x12 | `WRITE` | Write a value to a path |
| 0x13 | `WRITE_ACK` | Acknowledge a write |
| 0x14 | `PURGE` | Permanently erase instances |
| 0x15 | `PURGE_ACK` | Acknowledge a purge |

**Mesh coordination:**

| Type | Name | Purpose |
|------|------|---------|
| 0x20 | `HEARTBEAT` | Alive signal, clock sync, gossip payload |
| 0x21 | `REPLICATE` | Zone authority pushes changes to replicas |
| 0x22 | `ZONE_SPLIT` | Announce new zone boundaries |
| 0x23 | `ZONE_TRANSFER` | Bulk data for zone migration |
| 0x24 | `MAINTENANCE_ENTER` | Node entering maintenance mode |
| 0x25 | `MAINTENANCE_EXIT` | Node rejoining after maintenance |

**Recovery:**

| Type | Name | Purpose |
|------|------|---------|
| 0x30 | `RECOVERY_SHARD` | Submit a Shamir recovery shard |

Types 0x40–0xFF are reserved for future use. Unknown message types are logged and ignored.

#### Heartbeat

The heartbeat is the primary vehicle for mesh synchronization. It fires every third of a second and carries multiple concerns in a single message:

- **Alive signal** — the sender is operational
- **Clock value** — for clock synchronization across the mesh
- **Gossip payload** — membership changes, zone boundary updates, node health

Each node heartbeats to a small set of direct peers — zone cluster peers (the authority and replicas for zones this node participates in) and a handful of gossip peers for broader mesh awareness. This keeps per-node connection count bounded regardless of mesh size.

**Zone cluster heartbeats** are direct and immediate — writes replicate to replicas within one heartbeat tick. This is the hot path for data consistency.

**Gossip heartbeats** propagate mesh-wide information (new nodes, departed nodes, zone splits) through neighbors. This information may take several ticks to reach the entire mesh, which is acceptable for metadata that changes infrequently.

#### Routing

Any node can calculate which node owns a zone by hashing the path onto the consistent hash ring. When a node receives a request for data it does not own, it forwards the request to the calculated authority. The response returns along the same path. The client does not need to know the mesh topology.

For read requests, the node may route to the nearest replica rather than the authority, reducing latency and distributing read load.

#### Extensibility

The protocol is designed for forward compatibility:

- The version byte allows protocol evolution across major changes.
- Unknown message types are ignored gracefully.
- Unknown tagged fields in payloads are skipped.
- New message types and field tags can be added without breaking existing nodes.
