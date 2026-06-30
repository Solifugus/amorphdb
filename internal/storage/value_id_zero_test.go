package storage

import (
	"strings"
	"testing"
)

// TestWriteValueNeverReturnsZeroID guards the reserved-sentinel invariant: the
// value store must never hand out value ID 0 for a real value, because 0 is the
// "not found"/null sentinel used throughout the engine (findAttributeForPath,
// FindValueIDForData, attribute/instance chains).
//
// The first value written to tier 0 / bucket 0 would otherwise land at offset 0
// and encode to ID 0. This test writes a small value first (the case that used
// to mint ID 0) and asserts the returned ID is non-zero and round-trips.
func TestWriteValueNeverReturnsZeroID(t *testing.T) {
	vs, err := NewValueStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewValueStore: %v", err)
	}
	defer vs.Close()

	// A small (tier 0) value written first is exactly the case that previously
	// produced value ID 0.
	id, err := vs.WriteValue(Value{TypeTag: TypeText, Data: []byte("first")})
	if err != nil {
		t.Fatalf("WriteValue: %v", err)
	}
	if id == 0 {
		t.Fatal("WriteValue returned reserved value ID 0 for a real value")
	}

	got, err := vs.ReadValue(id)
	if err != nil {
		t.Fatalf("ReadValue(%d): %v", id, err)
	}
	if string(got.Data) != "first" {
		t.Errorf("round-trip data = %q, want %q", string(got.Data), "first")
	}
}

// TestReadAttributeWhenLabelPrecedesData reproduces the end-to-end storage
// symptom that broke PWA deploys: an attribute whose small path label is the
// first tier-0 value written — because its data value is large enough to spill
// into a higher tier — must still be readable.
//
// Before reserving value ID 0, the first such attribute's label took ID 0 and
// findAttributeForPath reported "path not found" even though the attribute and
// its instance existed.
func TestReadAttributeWhenLabelPrecedesData(t *testing.T) {
	st, err := NewStorageTree(t.TempDir())
	if err != nil {
		t.Fatalf("NewStorageTree: %v", err)
	}

	// TypeRecord asset-style value, comfortably larger than the 64-byte tier-0
	// threshold so its data spills to tier 1, leaving the path label as the
	// first tier-0 value.
	bigData := []byte("record-payload-" + strings.Repeat("x", 80))
	path := []string{"my", "computer", "network", "web", "pwa", "app.test", "assets", "app.js"}

	// 0x0C is the MBL Record type tag (defined in the types package); the
	// storage layer only cares that the serialized value exceeds the 64-byte
	// tier-0 threshold so it spills to a higher tier.
	const typeRecord = 0x0C
	if err := st.Write(path, Value{TypeTag: typeRecord, Data: bigData}, 1); err != nil {
		t.Fatalf("Write: %v", err)
	}

	got, err := st.Read(path)
	if err != nil {
		t.Fatalf("Read after write (the first tier-0 value was the path label): %v", err)
	}
	if string(got.Data) != string(bigData) {
		t.Errorf("read data mismatch: got %d bytes, want %d", len(got.Data), len(bigData))
	}
}
