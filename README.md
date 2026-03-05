# AmorphDB

A temporal tree-graph database with decentralized mesh architecture, accessed through MBL (Modern Business Language).

## Features

- **Temporal Storage**: Every value has a history — assignment records changes rather than overwriting
- **Tree-Graph Structure**: Data organized as hierarchy of attributes with timestamped instance chains
- **MBL Language**: High-level notation designed around scope resolution and path traversal
- **Decentralized Mesh**: Distributed architecture with consistent hashing and zone management
- **Type System**: Uniform, high-level data types (Text, Number, Time, Money, Picture, etc.)

## Architecture

AmorphDB consists of three core storage structures:
- **Attributes**: Named or numbered entries that form the tree structure
- **Instances**: Temporal records that form chains of value changes
- **Values**: Immutable, content-addressed data blobs

## Components

- **amorphd**: Service daemon
- **amorph**: Client REPL and script runner
- **amorphctl**: Administrative control tool

## Development Status

This project is currently under development. See `state/STATUS.md` for current progress.

## Design

For complete design specifications, see `amorphdb_design.md`.
For step-by-step development plan, see `amorphdb_development_plan.md`.

## Build

```bash
go build ./cmd/amorphd
go build ./cmd/amorph
go build ./cmd/amorphctl
```

## Test

```bash
go test ./...
```