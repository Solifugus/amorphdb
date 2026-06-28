package storage

import (
	"bytes"
	"testing"
)

// newTestAdapter builds a TreeAdapter over a real on-disk StorageTree.
func newTestAdapter(t *testing.T) *TreeAdapter {
	t.Helper()
	tree, err := NewStorageTree(t.TempDir())
	if err != nil {
		t.Fatalf("NewStorageTree failed: %v", err)
	}
	adapter, ok := NewTreeAdapter(tree).(*TreeAdapter)
	if !ok {
		t.Fatalf("NewTreeAdapter did not return *TreeAdapter")
	}
	return adapter
}

// TestTreeAdapterValueRoundTrip verifies a value stored by hash reads back byte-identical.
func TestTreeAdapterValueRoundTrip(t *testing.T) {
	adapter := newTestAdapter(t)

	value := []byte{0x01, 0x02, 0xFF, 0x00, 0xAB}
	valueHash := HashValue(value)

	if err := adapter.PutValue(valueHash, value); err != nil {
		t.Fatalf("PutValue failed: %v", err)
	}

	got, err := adapter.GetValue(valueHash)
	if err != nil {
		t.Fatalf("GetValue failed: %v", err)
	}
	if !bytes.Equal(got, value) {
		t.Fatalf("GetValue returned %v, want %v", got, value)
	}
}

// TestTreeAdapterGetValueMissing verifies an unstored hash reports not-found
// rather than fabricating a dummy value (the old stub returned fake bytes).
func TestTreeAdapterGetValueMissing(t *testing.T) {
	adapter := newTestAdapter(t)

	_, err := adapter.GetValue(HashValue([]byte("never stored")))
	if err == nil {
		t.Fatalf("GetValue on missing hash should error, got nil")
	}
}

// TestTreeAdapterGetValueIsolated verifies stored bytes cannot be mutated
// through the returned slice.
func TestTreeAdapterGetValueIsolated(t *testing.T) {
	adapter := newTestAdapter(t)

	value := []byte{0x10, 0x20, 0x30}
	valueHash := HashValue(value)
	if err := adapter.PutValue(valueHash, value); err != nil {
		t.Fatalf("PutValue failed: %v", err)
	}

	got, err := adapter.GetValue(valueHash)
	if err != nil {
		t.Fatalf("GetValue failed: %v", err)
	}
	got[0] = 0xFF // mutate the caller's copy

	again, err := adapter.GetValue(valueHash)
	if err != nil {
		t.Fatalf("GetValue (second) failed: %v", err)
	}
	if again[0] != 0x10 {
		t.Fatalf("stored value was mutated through returned slice: got %#x", again[0])
	}
}

// TestTreeAdapterInstanceRoundTrip verifies an instance stored for a
// (attribute, agent) pair reads back identically for that same pair.
func TestTreeAdapterInstanceRoundTrip(t *testing.T) {
	adapter := newTestAdapter(t)

	attrHash := HashPath([]string{"world", "projects", "alpha", "@write"})
	valueHash := HashValue([]byte("(link)world.agent.admin"))
	instance := SecurityInstance{
		AttributeHash: attrHash,
		ValueHash:     valueHash,
		Agent:         0, // permissions store globally
		Timestamp:     GetCurrentTimestamp(),
	}

	if err := adapter.PutInstance(attrHash, instance); err != nil {
		t.Fatalf("PutInstance failed: %v", err)
	}

	got, err := adapter.GetInstance(attrHash, 0)
	if err != nil {
		t.Fatalf("GetInstance failed: %v", err)
	}
	if got.AttributeHash != attrHash {
		t.Errorf("AttributeHash mismatch: got %x, want %x", got.AttributeHash, attrHash)
	}
	if got.ValueHash != valueHash {
		t.Errorf("ValueHash mismatch: got %x, want %x", got.ValueHash, valueHash)
	}
	if got.Agent != 0 {
		t.Errorf("Agent mismatch: got %d, want 0", got.Agent)
	}
	if got.Timestamp != instance.Timestamp {
		t.Errorf("Timestamp mismatch: got %d, want %d", got.Timestamp, instance.Timestamp)
	}
}

// TestTreeAdapterInstanceAgentScoped verifies that instances stored under the
// same attribute hash but different agents are kept distinct (personal stamps
// at ~.stamp share an attribute hash across agents and must not collide).
func TestTreeAdapterInstanceAgentScoped(t *testing.T) {
	adapter := newTestAdapter(t)

	attrHash := HashPath([]string{"~", "stamp"})

	instA := SecurityInstance{AttributeHash: attrHash, ValueHash: HashValue([]byte("alice")), Agent: 100}
	instB := SecurityInstance{AttributeHash: attrHash, ValueHash: HashValue([]byte("bob")), Agent: 200}

	if err := adapter.PutInstance(attrHash, instA); err != nil {
		t.Fatalf("PutInstance A failed: %v", err)
	}
	if err := adapter.PutInstance(attrHash, instB); err != nil {
		t.Fatalf("PutInstance B failed: %v", err)
	}

	gotA, err := adapter.GetInstance(attrHash, 100)
	if err != nil {
		t.Fatalf("GetInstance(100) failed: %v", err)
	}
	if gotA.ValueHash != instA.ValueHash {
		t.Errorf("agent 100 got wrong value hash: %x", gotA.ValueHash)
	}

	gotB, err := adapter.GetInstance(attrHash, 200)
	if err != nil {
		t.Fatalf("GetInstance(200) failed: %v", err)
	}
	if gotB.ValueHash != instB.ValueHash {
		t.Errorf("agent 200 got wrong value hash: %x", gotB.ValueHash)
	}
}

// TestTreeAdapterGetInstanceMissing verifies an unstored (attribute, agent)
// pair reports not-found so permission/stamp/filter callers fall back to
// defaults instead of acting on a fabricated instance.
func TestTreeAdapterGetInstanceMissing(t *testing.T) {
	adapter := newTestAdapter(t)

	attrHash := HashPath([]string{"world", "secret", "@write"})
	if _, err := adapter.GetInstance(attrHash, 0); err == nil {
		t.Fatalf("GetInstance on unstored attribute should error, got nil")
	}
}
