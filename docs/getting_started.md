# Getting Started with AmorphDB

A hands-on walkthrough: build the binaries, bootstrap a node, open the REPL, and
write your first MBL. Everything shown here is verified against the current
binaries — the examples are what the REPL actually does today.

- **New to the language?** Start here, then read [`mbl_reference.md`](mbl_reference.md).
- **Implementing the platform?** You want [`amorphdb_design.md`](amorphdb_design.md).

> **A note on scope.** MBL (Modern Business Language) is a large language, and
> the interpreter is still catching up to the full specification. This guide
> deliberately sticks to what runs reliably in the REPL *right now*. Where a
> spec'd feature isn't yet wired through the REPL, it says so plainly — see
> [What's not in the REPL yet](#whats-not-in-the-repl-yet).

---

## 1. The three binaries

| Binary | Role |
|--------|------|
| `amorphd` | The daemon — storage, MBL execution, mesh networking |
| `amorph` | The REPL / script runner you talk to |
| `amorphctl` | Admin tool — status, stop, invites (local socket) |

Build them from the repo root:

```bash
go build -o bin/amorphd   ./cmd/amorphd
go build -o bin/amorph    ./cmd/amorph
go build -o bin/amorphctl ./cmd/amorphctl
```

Put `bin/` on your `PATH`, or run the binaries by path. The rest of this guide
writes them bare (`amorphd`, `amorph`, `amorphctl`).

---

## 2. Bootstrap the node owner (one time)

Before first use, establish the **owner** — the genesis identity that owns this
host. This is a one-time step; it generates the owner's key material and records
who owns the node.

```bash
amorphd init-owner
```

You'll be prompted for a passphrase (or set `AMORPH_OWNER_PASSPHRASE`). It prints
the generated identity:

```
Owner initialized.
  Identity: bo-wi-bu-ju
  Home:     world.agent.83328350492948591
  Local clients now authenticate as this owner automatically.
```

Two things worth understanding:

- **The identity** (`bo-wi-bu-ju`) is a pronounceable consonant–vowel name drawn
  from a vast space — every agent gets one.
- **Local clients auto-authenticate as the owner.** A REPL connected over the
  local UNIX socket is trusted by filesystem ownership, so it *is* the owner —
  no login needed. (Remote clients must authenticate; see
  [Adding more users](#6-adding-more-users).)

The passphrase and a host-local *device secret* together derive the owner's
private key on demand. The private key is never stored as a static artifact, and
the device secret never leaves this machine.

---

## 3. Start the daemon

```bash
amorphd
```

```
AmorphDB service started
Node Identity: node-1775393940
Local Socket: /home/you/.amorph/socket
Network Port: 5000
HTTP Port: 8080  HTTPS Port: 8443
Storage: /home/you/.amorph/data
AmorphDB service running. Press Ctrl+C to stop.
```

Defaults: local socket at `~/.amorph/socket`, TCP port `5000`, data in
`~/.amorph/data`. Override with `-socket`, `-port`, and `-data`. Leave this
running and open a second terminal for the REPL.

Check on it any time:

```bash
amorphctl status
```

```
Service: running
Uptime: 12s
Node Identity: ...
Local Socket: /home/you/.amorph/socket
Network Socket: *:5000
Active Connections: 1
Data Size: 1.00 MB
```

---

## 4. Open the REPL

```bash
amorph
```

```
AmorphDB REPL - MBL Interactive Shell
Agent: bo-wi-bu-ju (ID: 83328350492948591)
Connected to service via protocol

Type 'exit' to quit, 'help' for help.

amorph>
```

The `Agent:` line confirms you're acting as the owner (local socket). Type
`exit`, `quit`, or `:q` to leave; `help` lists commands.

---

## 5. Your first MBL

MBL has no tables and no SQL. **Everything is a path**, and `my` is your home
(`world.agent.{your-identity}`). Assigning with `=` records a value; typing a
path reads it back.

```
amorph> my.name = "Alice"
Alice
amorph> my.name
Alice
```

### Values and types

```
amorph> my.age = 30
30
amorph> my.active = true
true
amorph> my.price = $19.99
$19.99
```

Numbers are always the processor's largest float; money carries its currency as
part of the value; booleans are `true`/`false`. Text uses double quotes.

### History is automatic

Assignment never overwrites — each write records a new instance, and reading a
path returns the current value:

```
amorph> my.account.balance = 100
100
amorph> my.account.balance = 250
250
amorph> my.account.balance
250
```

Both `100` and `250` are preserved internally. (Querying historical values by
timestamp is part of the language but not yet wired through the REPL — see
[below](#whats-not-in-the-repl-yet).)

### Building text

`&` concatenates. Non-text values are coerced to text:

```
amorph> greeting = "Hello, " & my.name
Hello, Alice
amorph> "You are " & my.age & " years old"
You are 30 years old
```

### Lists

```
amorph> my.nums = [10, 20, 30]
[10, 20, 30]
amorph> my.nums[0]
10
amorph> my.nums..count
3
```

### Procedures

Define a procedure, then call it. A single-line body is the easiest to enter
interactively:

```
amorph> procedure double(x): return x * 2
amorph> double(21)
42
```

### Control flow

`if` / `for` / `while` use a trailing `:` and an indented body. Press Enter on a
blank line to finish a multi-line block. `output(...)` prints to the daemon's
console:

```
amorph> my.total = 0
0
amorph> for n in [1, 2, 3, 4]:
     |     my.total = my.total + n
     |
10
amorph> my.total
10
```

The `     | ` is the continuation prompt for indented lines.

### Unknown instead of exceptions

When a computation can't produce a value, the result is an `Unknown` that flows
through the expression instead of throwing:

```
amorph> 5 + unknown("offline")
Unknown: offline
```

Guard against it with the definite-equality operator `?=`, which returns a real
boolean (`false`) rather than propagating:

```
amorph> if my.age ?= 30:
     |     output("thirty")
     |
thirty
```

> **Multi-line tip.** A blank line submits an indented block, so single-branch
> `if` and `for` work well interactively. Blocks with `else`/`else if` or nested
> bodies are easier to enter in a script file and run with `-run` (next section) —
> the interactive line-continuation can otherwise submit the block early.

---

## 6. Running scripts

Multi-line procedures and larger programs are best kept in a file and run with
`-run` (which parses the whole file at once, avoiding interactive
line-continuation quirks):

```mbl
# hello.mbl
my.greeting = "Hello from a script"
output(my.greeting)
```

```bash
amorph -run hello.mbl                 # local socket
amorph -node localhost:5000 -run hello.mbl   # remote over TCP
```

---

## 7. Adding more users

Additional users are enrolled with an **invite** — a one-time token an authorized
admin (the owner, by default) mints. The new user generates their own key pair
locally and registers only the public key; **their private key never touches the
daemon.**

**On the owner's machine**, mint a token:

```bash
amorphctl invite
```

```
Enrollment invite created (one-time, valid 24h).

  Token: 3f9a…c7e1

Give this token to the new agent, who enrolls with:
  amorph enroll -node <host:port> -identity <name> -token 3f9a…c7e1
```

**On the new user's machine**, enroll (prompts for a passphrase they choose):

```bash
amorph enroll -node db.example.com:5000 -identity taro -token 3f9a…c7e1
```

```
Enrolled as taro (agent ...).
Log in with:
  amorph login -node db.example.com:5000 -identity taro
```

Then log in — this authenticates via challenge-response and drops you into the
REPL as that identity, so `my.*` resolves under `taro`'s home:

```bash
amorph login -node db.example.com:5000 -identity taro
```

Credentials live under `~/.amorph/agents/{identity}/` on the user's machine (the
device secret, `0600`). The passphrase is never stored — it's prompted each login
and combined with the device secret to re-derive the key.

---

## 8. Stopping the daemon

```bash
amorphctl stop
```

or press `Ctrl+C` in the daemon's terminal.

---

## What's not in the REPL yet

MBL specifies more than the interpreter currently executes end-to-end. So you
don't hit surprises, here is what is **documented but not yet wired through the
REPL**. These are recorded and will light up as the interpreter catches up:

| Feature | Status in the REPL today |
|---|---|
| Historical temporal queries (`path[@2026-01-01]`) | Not yet — reads currently return the current value only; history is still recorded |
| Instance meta-attributes (`path.@time`, `path.@agent`) | Not yet readable over the REPL |
| Reading a bare container/record node (`my.person`) | Reads of a node's *leaves* work (`my.person.name`); reading the container itself does not |
| Field projection over stored paths (`my.person{ name }`) | Not yet |
| Persisting `..append` / `..prepend` back to storage | The operation returns the new collection but does not yet persist it |
| Watchers, templates/`new()`, bracket filters on reads | Specified; exercise via the design doc, not yet reliable in the REPL |

When in doubt about whether something works today, try it in the REPL — an
unsupported read reports a clear error rather than doing the wrong thing.

---

## Where to go next

| Document | For |
|----------|-----|
| [`mbl_reference.md`](mbl_reference.md) | The full MBL language — types, watchers, procedures, permissions, `my.computer.*`. Distillation of the design doc. |
| [`amorphdb_design.md`](amorphdb_design.md) | Authoritative specification — language, storage, mesh, security, PWA identity. The source of truth. |
| [`mesh_management_guide.md`](mesh_management_guide.md) | Forming and managing a multi-node mesh. |

The reference and design docs describe the language as designed; this guide
describes the REPL as it runs today. Where they differ, the difference is the
frontier — and it's shrinking.
