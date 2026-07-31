// Tests for reading a stored container as a record.
//
// DEVPLAN Step 32. Before this, the interpreter never enumerated the children of
// a stored path, and every affected operation returned a plausible-looking wrong
// answer instead of failing: `my.p` gave {}, a projection over it answered
// not_found for fields that existed, and ..count gave 0. A zero count is
// indistinguishable from a genuinely empty node, so application code could not
// detect the difference — which is what made this worse than an unsupported
// operation.
package interpreter

import (
	"fmt"
	"testing"

	"github.com/solifugus/amorphdb/internal/types"
)

func seedPerson(t *testing.T, interp *Interpreter) {
	t.Helper()
	runMBL(t, interp, `my.p.name = "Alice"`)
	runMBL(t, interp, `my.p.age = 30`)
}

func TestStoredContainerReadsAsARecord(t *testing.T) {
	_, interp := freshTreeInterp(t, "stored-record")
	seedPerson(t, interp)

	res := runMBL(t, interp, `my.p`)
	record, ok := res.(types.Record)
	if !ok {
		t.Fatalf("my.p = %T (%v), want types.Record", res, res)
	}

	if got := record.Fields["name"]; got != (types.Text{Value: "Alice"}) {
		t.Errorf("name = %#v, want Text{Alice}", got)
	}
	if got := record.Fields["age"]; got != (types.Number{Value: 30}) {
		t.Errorf("age = %#v, want Number{30}", got)
	}
	if len(record.Fields) != 2 {
		t.Errorf("got %d fields (%v), want 2", len(record.Fields), record.Fields)
	}
}

// TestProjectionOverStoredPath is the case that answered not_found for fields
// that were plainly present.
func TestProjectionOverStoredPath(t *testing.T) {
	_, interp := freshTreeInterp(t, "stored-projection")
	seedPerson(t, interp)
	runMBL(t, interp, `my.p.city = "Rome"`)

	res := runMBL(t, interp, `my.p{ name, age }`)
	record, ok := res.(types.Record)
	if !ok {
		t.Fatalf("projection = %T (%v), want types.Record", res, res)
	}

	if got := record.Fields["name"]; got != (types.Text{Value: "Alice"}) {
		t.Errorf("name = %#v, want Text{Alice}", got)
	}
	if got := record.Fields["age"]; got != (types.Number{Value: 30}) {
		t.Errorf("age = %#v, want Number{30}", got)
	}
	if _, present := record.Fields["city"]; present {
		t.Error("projection included 'city', which was not selected")
	}
}

// TestCountOverStoredPath covers the answer that was silently 0.
func TestCountOverStoredPath(t *testing.T) {
	_, interp := freshTreeInterp(t, "stored-count")
	seedPerson(t, interp)

	res := runMBL(t, interp, `my.p..count`)
	got, ok := res.(types.Number)
	if !ok {
		t.Fatalf("count = %T (%v), want types.Number", res, res)
	}
	if got.Value != 2 {
		t.Errorf("count = %v, want 2", got.Value)
	}
}

// TestNestedContainerComesBackWhole is why the walk recurses. Stopping at one
// level would render a populated child as {}, which is the same misleading
// emptiness this step removes.
func TestNestedContainerComesBackWhole(t *testing.T) {
	_, interp := freshTreeInterp(t, "stored-nested")
	seedPerson(t, interp)
	runMBL(t, interp, `my.p.addr.city = "Rome"`)
	runMBL(t, interp, `my.p.addr.zip = "00184"`)

	res := runMBL(t, interp, `my.p`)
	record, ok := res.(types.Record)
	if !ok {
		t.Fatalf("my.p = %T (%v), want types.Record", res, res)
	}

	addr, ok := record.Fields["addr"].(types.Record)
	if !ok {
		t.Fatalf("addr = %#v, want a nested types.Record", record.Fields["addr"])
	}
	if got := addr.Fields["city"]; got != (types.Text{Value: "Rome"}) {
		t.Errorf("addr.city = %#v, want Text{Rome}", got)
	}
	if len(addr.Fields) != 2 {
		t.Errorf("addr has %d fields (%v), want 2", len(addr.Fields), addr.Fields)
	}

	// Reading the nested path directly must agree with the nested value.
	direct := runMBL(t, interp, `my.p.addr`)
	directRecord, ok := direct.(types.Record)
	if !ok {
		t.Fatalf("my.p.addr = %T, want types.Record", direct)
	}
	if len(directRecord.Fields) != len(addr.Fields) {
		t.Errorf("direct read has %d fields, nested has %d — they disagree",
			len(directRecord.Fields), len(addr.Fields))
	}
}

// TestLeafReadsAreUnaffected guards the regression risk: leaf values must keep
// reading as values, not become records.
func TestLeafReadsAreUnaffected(t *testing.T) {
	_, interp := freshTreeInterp(t, "stored-leaf")
	seedPerson(t, interp)

	if got := runMBL(t, interp, `my.p.name`); got != (types.Text{Value: "Alice"}) {
		t.Errorf("my.p.name = %#v, want Text{Alice}", got)
	}
	if got := runMBL(t, interp, `my.p.age`); got != (types.Number{Value: 30}) {
		t.Errorf("my.p.age = %#v, want Number{30}", got)
	}
}

// TestMissingPathStillUnknown checks a genuinely absent path is still Unknown
// rather than an empty record — the distinction the old behaviour destroyed.
func TestMissingPathStillUnknown(t *testing.T) {
	_, interp := freshTreeInterp(t, "stored-missing")
	seedPerson(t, interp)

	res := runMBL(t, interp, `my.nonexistent`)
	if _, ok := res.(types.Record); ok {
		t.Errorf("a missing path read as a record: %#v", res)
	}
}

// TestLargeRecordHitsTheNodeLimit covers the bound: the walk must report rather
// than silently truncate.
func TestLargeRecordHitsTheNodeLimit(t *testing.T) {
	_, interp := freshTreeInterp(t, "stored-limit")

	// Comfortably more children than the budget allows.
	for n := 0; n < MaxRecordNodes+50; n++ {
		runMBL(t, interp, fmt.Sprintf(`my.big.f%d = %d`, n, n))
	}

	res := runMBL(t, interp, `my.big`)
	unknown, ok := res.(types.Unknown)
	if !ok {
		t.Fatalf("oversized record read as %T, want Unknown naming the limit", res)
	}
	if unknown.Reason == "" {
		t.Error("Unknown carries no reason")
	}
}

// TestBracketFilterOverStoredRecords is the operation that only became reachable
// once stored containers read as records. The test that covered it previously
// passed vacuously — the container was empty, so the predicate never ran, and
// field resolution inside the filter was quietly broken.
func TestBracketFilterOverStoredRecords(t *testing.T) {
	_, interp := freshTreeInterp(t, "stored-filter")
	runMBL(t, interp, `world.staff.alice.age = 25`)
	runMBL(t, interp, `world.staff.bob.age = 40`)
	runMBL(t, interp, `world.staff.carol.age = 35`)

	res := runMBL(t, interp, `world.staff[age > 30]`)
	record, ok := res.(types.Record)
	if !ok {
		t.Fatalf("filter = %T (%v), want types.Record", res, res)
	}

	if _, present := record.Fields["alice"]; present {
		t.Error("alice (25) matched a filter of age > 30")
	}
	for _, want := range []string{"bob", "carol"} {
		if _, present := record.Fields[want]; !present {
			t.Errorf("%s missing from the filter result (got %v)", want, record.Fields)
		}
	}
	if len(record.Fields) != 2 {
		t.Errorf("got %d matches (%v), want 2", len(record.Fields), record.Fields)
	}
}
