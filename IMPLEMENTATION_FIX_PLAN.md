# AmorphDB Implementation Fix Plan

**Goal**: Align implementation with amorphdb_design.md specification in the docs directory.

**Priority**: Critical issues first, then integration issues, then polish

---

## 🔥 **PHASE 1: Critical Type System Fixes** (HIGH PRIORITY)

### **Issue 1.1: Value Interface/Struct Confusion**

**Problem**: Tests expect `types.Value` as interface, implementation uses struct
```go
// Current (struct):
type Value struct { TypeTag byte; Data []byte }

// Tests expect interface pattern like:
func Write(value types.Value) // should accept *types.Text
```

**Fix Strategy**:
1. **Verify Design Intent**: Check design doc for Value pattern
2. **Convert to Interface**: Make `Value` an interface with common methods
3. **Update Implementations**: Make Text, Number, etc. implement Value interface

**Files to Fix**:
- `internal/types/types.go` - Convert Value to interface
- `test/integration/core_integration_test.go` - Update usage patterns

### **Issue 1.2: Missing Protocol Functions**

**Problem**: `protocol.EncodeStatusMessage` undefined
**Fix**: Implement missing protocol encoding functions per design

---

## 🚨 **PHASE 2: Parser Critical Bug** (HIGH PRIORITY)

### **Issue 2.1: Parentheses Parsing Flood**

**Problem**: "no prefix parse function for RPAREN found" errors (1000s of them)
**Root Cause**: Parser is trying to parse RPAREN as standalone expression

**Analysis**:
- `parseGroupedExpression` correctly registered for LPAREN ✅
- RPAREN should be consumed by LPAREN parser, not parsed independently ❌

**Fix Strategy**:
1. **Debug Input**: Identify what input is causing this flood
2. **Token Generation**: Check if lexer is generating unmatched RPAREN tokens
3. **Parser Logic**: Ensure RPAREN is only consumed, never parsed as prefix

**Files to Fix**:
- `internal/mbl/parser/parser.go` - Debug parseGroupedExpression
- `internal/mbl/lexer/lexer.go` - Verify token generation
- Test input causing the flood

---

## 🔧 **PHASE 3: Integration Test Organization** (MEDIUM PRIORITY)

### **Issue 3.1: Multiple Main Functions**

**Problem**: Integration tests structured as standalone programs, not Go tests
```go
// Current problematic structure:
tests/integration/simple_buffer_demo.go:15: main redeclared
tests/integration/commit_buffer_demo.go:14:  main redeclared
```

**Fix Strategy**:
1. **Convert to Test Functions**: Change `func main()` to `func TestXxx(t *testing.T)`
2. **Extract Common Logic**: Move shared code to helper functions
3. **Proper Test Organization**: Follow Go testing conventions

**Files to Fix**:
- `tests/integration/simple_buffer_demo.go`
- `tests/integration/commit_buffer_demo.go`
- `tests/integration/simple_commit_demo.go`

### **Issue 3.2: Missing Imports/Constructors**

**Problem**: Integration tests reference undefined functions
```go
undefined: lexer.Tokenize        // Should be: lexer.New(input).Tokenize()
undefined: parser.NewParser      // Should be: parser.New(lexer)
undefined: interpreter.NewInterpreter  // Check actual constructor signature
```

**Fix Strategy**:
1. **Check Actual APIs**: Verify correct constructor signatures in design
2. **Update Import Paths**: Ensure correct package imports
3. **Constructor Alignment**: Match design specifications

---

## 🛠️ **PHASE 4: Service Interface Alignment** (MEDIUM PRIORITY)

### **Issue 4.1: REPL Constructor Mismatch**

**Problem**: `NewREPL()` signature changed but tests not updated
```go
// Test calls:
repl := NewREPL()  // ❌ missing arguments

// Expected (based on error):
repl := NewREPL(string, string)  // ✅ needs socketPath, storageDir
```

**Fix Strategy**:
1. **Check Design**: Verify intended REPL constructor signature
2. **Update Tests**: Fix test calls to match actual signature
3. **Validate Fields**: Ensure all required fields are accessible

**Files to Fix**:
- `cmd/amorph/repl_test.go`

---

## 🧪 **PHASE 5: Design Compliance Verification** (LOW PRIORITY)

### **Issue 5.1: Type System Alignment**

**Verify Against Design**:
- ✅ 9 data types: Text, Number, Time, Money, Picture, Reference, Procedure, Watcher, Embed
- ✅ Instance metadata: @timestamp, @author, @embed, etc.
- ✅ Temporal semantics: append-only, history preservation
- ❌ **Interface patterns**: Ensure Value interface matches design intent

### **Issue 5.2: Parser Feature Completeness**

**Verify Against Design**:
- ✅ Precedence levels: OR < AND < EQUALS < SUM < PRODUCT < CALL
- ✅ Bracket syntax: [conditions], temporal queries [@2026-01-01]
- ✅ Path resolution: my.path, world.path, ~.path
- ❌ **Parentheses**: Must handle grouped expressions correctly per design

---

## 🎯 **EXECUTION ORDER**

### **Week 1: Critical Fixes**
1. **Day 1-2**: Fix Value interface/struct issue (Phase 1.1)
2. **Day 3-4**: Debug and fix parentheses parser flood (Phase 2.1)
3. **Day 5**: Add missing protocol functions (Phase 1.2)

### **Week 2: Integration**
1. **Day 1-2**: Reorganize integration tests (Phase 3.1)
2. **Day 3**: Fix missing imports/constructors (Phase 3.2)
3. **Day 4**: Fix REPL test signature (Phase 4.1)
4. **Day 5**: Design compliance verification (Phase 5)

---

## 🏁 **SUCCESS CRITERIA**

### **Phase 1 Complete When**:
- [ ] `make test` shows no type interface errors
- [ ] All Value implementations work correctly
- [ ] Protocol functions compile

### **Phase 2 Complete When**:
- [ ] No "RPAREN" parser errors in test output
- [ ] Parentheses expressions parse correctly
- [ ] Parser handles complex expressions without flooding

### **Phase 3 Complete When**:
- [ ] All integration tests structured as proper Go tests
- [ ] No "main redeclared" errors
- [ ] All imports resolve correctly

### **Overall Success**:
- [ ] `make test` runs without build failures
- [ ] Core functionality tests pass
- [ ] Implementation matches amorphdb_design.md specification

---

## 🔍 **IMPLEMENTATION VERIFICATION**

For each fix, verify against design document sections:
- **Types**: Section "Data Types" (lines 76-132)
- **Parser**: Section "Notation and Language" (lines 18-75)
- **Interface**: Section "Temporal Semantics" (lines 12-17)
- **Integration**: Follow Go testing best practices

This plan prioritizes getting the core system working correctly before addressing integration polish.
