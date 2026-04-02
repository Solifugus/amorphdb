# Multi-Path Watcher Implementation Plan

## Overview

Currently, AmorphDB watchers support only single paths:
```mbl
my_watcher: watch(my.account.balance):
    # responds to balance changes only
```

The design document specifies multi-path syntax:
```mbl
my_watcher: watch(my.account.balance, my.account.limit):
    # should respond to changes in balance OR limit
```

This plan outlines the implementation steps to add multi-path watcher support.

## Current Implementation Analysis

### What Works Now
- ✅ Single-path value watchers: `watch(path)`
- ✅ Append watchers: `watch append(path) as name`
- ✅ Predicate filtering for append watchers
- ✅ Watcher engine with trigger detection and execution

### Gap Analysis
- ❌ Parser doesn't handle comma-separated paths in `watch(path1, path2, ...)`
- ❌ AST `WatchStatement` has single `Path` field, needs multiple paths
- ❌ Watcher engine `RegisterWatcher` accepts `[]string` paths but parser only provides one
- ❌ No tests for multi-path behavior

## Implementation Plan

### Step 1: Update AST Structure
**File**: `internal/mbl/parser/ast.go`

**Current:**
```go
type WatchStatement struct {
    Token       lexer.Token      // The WATCH token
    Name        string           // Watcher identifier
    IsAppend    bool             // Whether this is an append watcher
    Path        Expression       // The path being watched (SINGLE)
    Filters     []Expression     // Optional predicate filters
    BindingName string           // Variable name for 'as name' binding
    Body        *BlockStatement  // The watcher body
}
```

**New:**
```go
type WatchStatement struct {
    Token       lexer.Token      // The WATCH token
    Name        string           // Watcher identifier
    IsAppend    bool             // Whether this is an append watcher
    Paths       []Expression     // The paths being watched (MULTIPLE)
    Filters     []Expression     // Optional predicate filters
    BindingName string           // Variable name for 'as name' binding
    Body        *BlockStatement  // The watcher body
}
```

**Changes:**
- Change `Path Expression` to `Paths []Expression`
- Update `String()` method to handle multiple paths
- Maintain backwards compatibility by supporting single-path case

### Step 2: Update Parser Logic
**File**: `internal/mbl/parser/parser.go`

**Current parseWatchStatement() logic:**
```go
// Parse single path expression
p.nextToken()
stmt.Path = p.parseExpression(LOWEST)
```

**New logic:**
```go
// Parse comma-separated path expressions
p.nextToken()
stmt.Paths = []Expression{}
stmt.Paths = append(stmt.Paths, p.parseExpression(LOWEST))

for p.peekToken.Type == lexer.COMMA {
    p.nextToken() // consume COMMA
    p.nextToken() // move to next expression
    stmt.Paths = append(stmt.Paths, p.parseExpression(LOWEST))
}
```

**Edge Cases to Handle:**
- Single path: `watch(my.value)` → `Paths = [my.value]`
- Multi path: `watch(my.a, my.b)` → `Paths = [my.a, my.b]`
- Append watchers remain single-path: `watch append(my.list) as items`

### Step 3: Update Watcher Engine Integration
**File**: `internal/watcher/engine.go`

**Current Registration:**
- Engine expects `[]string` paths but receives single path from parser
- Need to convert `[]Expression` to `[]string` properly

**Changes Needed:**
- Update watcher registration to handle multiple path strings
- Ensure trigger detection works for any path in the list
- Maintain backwards compatibility with existing single-path watchers

### Step 4: Add Parser Tests
**File**: `internal/mbl/parser/parser_test.go`

**Test Cases:**
```go
// Multi-path regular watcher
my_watcher: watch(my.account.balance, my.account.limit):
    output("balance or limit changed")

// Three paths
monitor: watch(my.cpu.usage, my.memory.usage, my.disk.usage):
    if my.cpu.usage > 90 or my.memory.usage > 95:
        my.alerts.send("High resource usage")

// Mixed path types
complex: watch(my.config.setting, world.clock.minute):
    # responds to config changes OR time tick
```

**Error Cases:**
```go
// Invalid: append watcher with multiple paths (should be rejected)
bad: watch append(my.list1, my.list2) as items:
    pass

// Invalid: trailing comma
bad2: watch(my.path1, my.path2,):
    pass

// Invalid: empty path list
bad3: watch():
    pass
```

### Step 5: Add Integration Tests
**File**: `internal/watcher/engine_test.go`

**Test Scenarios:**
- Multi-path watcher triggers when first path changes
- Multi-path watcher triggers when second path changes
- Multi-path watcher doesn't trigger when unrelated path changes
- Multi-path watcher with multiple simultaneous changes
- Performance with many multi-path watchers

### Step 6: Update Documentation
**File**: `docs/amorphdb_design.md`

**Add Examples:**
```mbl
# Multi-path value watcher
my.automation.balance_monitor: watch(my.account.balance, my.account.limit):
    if my.account.balance > my.account.limit:
        my.account.status = (quietly) "overlimit"
        my.alerts.send("Account overlimit: " & my.account.balance)

# Monitoring multiple resources
my.monitoring.resource_watcher: watch(my.cpu.usage, my.memory.usage, my.disk.usage):
    if my.cpu.usage > 90:
        my.alerts.send("High CPU: " & my.cpu.usage & "%")
    if my.memory.usage > 95:
        my.alerts.send("High memory: " & my.memory.usage & "%")
```

## Technical Considerations

### Trigger Behavior
**Decision**: Multi-path watchers trigger when **ANY** watched path changes (OR semantics)

**Rationale**:
- Matches intuitive expectation
- Consistent with current engine design
- Allows fine-grained reactive programming

### Backwards Compatibility
**Strategy**: Zero-breaking changes
- Single-path syntax continues to work unchanged
- AST change from `Path` to `Paths[]` handled transparently
- Existing watcher engine tests continue to pass

### Performance Impact
**Considerations**:
- Each path in multi-path watcher creates separate trigger registration
- Memory overhead: N paths = N trigger entries
- Acceptable for typical use cases (2-5 paths per watcher)

## Implementation Order

1. **Step 1**: AST changes (safest, affects compilation)
2. **Step 2**: Parser logic (core functionality)
3. **Step 4**: Parser tests (validate parsing works)
4. **Step 3**: Watcher engine integration (full functionality)
5. **Step 5**: Integration tests (validate execution works)
6. **Step 6**: Documentation updates

**Estimated effort**: 2-3 hours of focused development + testing

## Success Criteria

✅ **Parser Acceptance**:
- Single-path syntax: `watch(path)` works unchanged
- Multi-path syntax: `watch(path1, path2)` parses correctly
- Error handling: malformed multi-path syntax rejected properly

✅ **Engine Integration**:
- Multi-path watchers register correctly
- Triggers fire when any watched path changes
- No regressions in existing single-path watchers

✅ **Test Coverage**:
- All parser test cases pass
- All integration test scenarios pass
- Performance acceptable for realistic workloads

This implementation will complete the watcher specification and provide the full reactive programming capabilities described in amorphdb_design.md.