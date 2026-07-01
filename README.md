# AmorphDB

A temporal tree-graph database written in Go. Every value has a history — assignment records a change rather than overwriting what came before.

## What It Is

AmorphDB organizes data as a hierarchy of attributes, each holding a chain of timestamped instances. The full evolution of any piece of data is queryable at any point in time.

The system is accessed through **MBL (Modern Business Language)**, a notation designed around scope resolution and path traversal. Types are deliberately high-level — a number is always the largest float the processor supports, a picture is always full RGBA at 16-bit depth.

AmorphDB operates as a decentralized mesh. Every node is a peer. There is no central coordinator.

### Core Ideas

- **Temporal by default.** `my.x = 5` followed by `my.x = 10` produces two instances, both preserved. Queries can retrieve any historical value.
- **Reactive programming.** Watchers fire automatically when data changes. Multi-path watchers, append watchers with predicates, and heartbeat atomicity with staged writes.
- **Hierarchical data model.** Three storage structures — attributes (the tree), instances (temporal chains), and values (content-addressed, deduplicated).
- **Mesh distribution.** Nodes form a peer-to-peer mesh with heartbeat, gossip, and bridge connections between separate meshes.
- **Capability-based security.** Five permission types (`@read`, `@write`, `@expand`, `@grant`, `@purge`) with cascade inheritance, stamps for metadata provenance, and filters for client-side data hiding.

## Components

| Binary | Purpose |
|--------|---------|
| `amorphd` | Service daemon — storage, mesh networking, MBL execution |
| `amorph` | Client REPL and script runner |
| `amorphctl` | Administrative tool — mesh management, status, compaction |

## Quick Start

```bash
# Build
go build -o bin/amorphd   ./cmd/amorphd
go build -o bin/amorph    ./cmd/amorph
go build -o bin/amorphctl ./cmd/amorphctl

# One-time: establish the node owner (prompts for a passphrase)
amorphd init-owner

# Start a standalone node
amorphd

# Connect with the client (local socket = auto-authenticated as owner)
amorph

# Basic operations in the REPL
amorph> my.name = "Alice"
Alice
amorph> my.name
Alice
```

See [`docs/getting_started.md`](docs/getting_started.md) for the full
walkthrough, including enrolling additional users.

### Mesh Operations

```bash
# Create a named mesh
amorphctl create-mesh "production"

# Join an existing mesh
amorphctl join node2.company.com:5830

# Bridge to a partner mesh
amorphctl bridge partner-db.example.com:5830
```

## MBL Examples

```mbl
# Records and temporal history
my.account.balance = 100
my.account.balance = 250
my.account.balance[@2026-01-01]          # query historical value

# Record literals
my.person = { name: "Matthew", age: 55, job: "Engineer" }
my.person{ name, age }                   # projection: select fields

# Bracket queries
my.employees[active = true]{ name, salary }
my.orders[@ > @2026-03-01, @ < @2026-04-01]   # temporal range

# Procedures with persistent state
my.counters.visits: procedure():
    .count = .count + 1
    return .count

# Reactive watchers
my.automation.balance_check: watch(my.account.balance, my.account.limit):
    if my.account.balance > my.account.limit:
        my.account.status = (quietly) "overlimit"

# Append watchers for event processing
my.automation.new_orders: watch append(my.orders[status ?= "pending"]) as orders:
    for order in orders:
        my.computer.output("New order: " & order.id)

# Exception handling
catch:
    risky_operation()
else unknown:
    my.computer.output("Failed: " & unknown)
```

## Documentation

| Document | Description |
|----------|-------------|
| [`docs/getting_started.md`](docs/getting_started.md) | **Start here** — build, bootstrap, and a hands-on REPL walkthrough |
| [`docs/mbl_reference.md`](docs/mbl_reference.md) | MBL quick reference — types, watchers, procedures, permissions, `my.computer.*` |
| [`docs/amorphdb_design.md`](docs/amorphdb_design.md) | Authoritative specification — language, storage, mesh, security |
| [`docs/mesh_management_guide.md`](docs/mesh_management_guide.md) | Mesh operations |
| [`docs/bridge_architecture.md`](docs/bridge_architecture.md) | Bridge implementation details |
| [`docs/AmorphDB_Remaining_Work.md`](docs/AmorphDB_Remaining_Work.md) | Known gaps and remaining work |

## Development Status

AmorphDB's core systems are implemented and tested. A comprehensive 250-test
plan validated the storage engine, type system, MBL language pipeline (lexer,
parser, interpreter), watcher/procedure system, stamps, filters, permissions,
protocol, mesh networking, and security.

### What Works

- Storage engine with temporal history, deduplication, and concurrent access
- Complete MBL language — lexer, parser, interpreter with 15+ built-in functions
- Reactive watchers (value-change, multi-path, append with predicates)
- Heartbeat atomicity with staged writes and rollback on unhandled Unknown
- Record literals, projections, catch/else exception handling
- Stamps, filters, and capability-based permissions
- Mesh formation, heartbeat, gossip, bridge connections
- Client-server communication via EXECUTE protocol
- Post-quantum encryption and challenge-response authentication

### What's In Progress

See [`docs/AmorphDB_Remaining_Work.md`](docs/AmorphDB_Remaining_Work.md) for
the full prioritized list. Key items:

- Projection reading from expanded storage paths
- `my.computer.output` / `my.computer.input` I/O procedures
- Same-line definitions and recursive assignment (🚧)
- Wildcard projections (🚧)
- Agent-level encryption and key rotation
- Subscription-based data distribution (replacing zone model)
- Multi-node integration tests

### Known Issues

- 12 packages have pre-existing test failures (none from the test plan work).
  See remaining work doc for details.
- Protocol number precision: decimal values may lose precision in encoding.

## Testing

```bash
# Run all tests
go test ./...

# Run specific section tests
go test ./internal/storage/... -v
go test ./internal/mbl/... -v
go test ./internal/watcher/... -v
go test ./internal/security/... -v
```

## Project Structure

```
amorphdb/
├── cmd/
│   ├── amorphd/          # Service daemon
│   ├── amorph/           # Client REPL
│   └── amorphctl/        # Admin tool
├── internal/
│   ├── storage/          # Storage engine
│   ├── types/            # Type system
│   ├── mbl/              # MBL language (lexer, parser, interpreter)
│   ├── watcher/          # Reactive watcher engine
│   ├── mesh/             # Mesh networking
│   ├── security/         # Security and permissions
│   ├── protocol/         # Wire protocol
│   ├── service/          # Daemon service layer
│   └── config/           # Configuration
├── docs/                 # Specifications and guides
├── test/                 # Integration tests
└── examples/             # MBL scripts and mesh setup
```

## License

MIT
