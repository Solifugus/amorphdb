# AmorphDB

A distributed temporal tree-graph database with reactive programming capabilities.

[![Go Version](https://img.shields.io/badge/go-1.21+-blue.svg)](https://golang.org)
[![License](https://img.shields.io/badge/license-MIT-green.svg)](LICENSE)
[![Tests](https://img.shields.io/badge/tests-45%2F45-brightgreen.svg)](docs/PHASE3_EXECUTION_STATUS.md)

## Overview

AmorphDB is a revolutionary temporal database that preserves complete data history through an append-only architecture. Unlike traditional databases that overwrite data, AmorphDB records every change with full provenance, enabling powerful temporal queries and comprehensive audit trails.

### Key Features

- **Temporal-First Design**: Complete history preservation with point-in-time queries
- **Distributed Mesh Architecture**: Peer-to-peer network with no single points of failure
- **Modern Business Language (MBL)**: Domain-specific language for hierarchical temporal data
- **Reactive Programming**: Automated computation through watchers and procedures
- **Enterprise Security**: Post-quantum encryption, multi-factor authentication
- **Zero-Trust Architecture**: Comprehensive permissions, filters, and audit trails
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