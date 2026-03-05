package storage

import (
	"path/filepath"
	"testing"
)

func TestNewAttributeStore(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "attributes.dat")

	as, err := NewAttributeStore(filePath)
	if err != nil {
		t.Fatalf("NewAttributeStore failed: %v", err)
	}
	defer as.Close()

	// Verify initial state
	if as.nextID != 1 {
		t.Errorf("Initial nextID should be 1, got %d", as.nextID)
	}

	count := as.GetAttributeCount()
	if count != 0 {
		t.Errorf("Initial attribute count should be 0, got %d", count)
	}
}

func TestCreateAttributeWithTextLabel(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "attributes.dat")

	as, err := NewAttributeStore(filePath)
	if err != nil {
		t.Fatalf("NewAttributeStore failed: %v", err)
	}
	defer as.Close()

	// Create attribute with text label, read it back
	labelValueID := uint64(12345)    // Points to "name" in value store
	firstInstanceID := uint64(67890) // Points to instance with value "Joe"
	nextAttributeID := uint64(0)     // No siblings yet

	attributeID, err := as.WriteAttribute(labelValueID, firstInstanceID, nextAttributeID)
	if err != nil {
		t.Fatalf("WriteAttribute failed: %v", err)
	}

	if attributeID != 1 {
		t.Errorf("First attribute ID should be 1, got %d", attributeID)
	}

	// Read it back
	attribute, err := as.ReadAttribute(attributeID)
	if err != nil {
		t.Fatalf("ReadAttribute failed: %v", err)
	}

	// Confirm all fields match
	if attribute.ID != attributeID {
		t.Errorf("Attribute ID mismatch: got %d, want %d", attribute.ID, attributeID)
	}

	if attribute.LabelValueID != labelValueID {
		t.Errorf("LabelValueID mismatch: got %d, want %d", attribute.LabelValueID, labelValueID)
	}

	if attribute.FirstInstanceID != firstInstanceID {
		t.Errorf("FirstInstanceID mismatch: got %d, want %d", attribute.FirstInstanceID, firstInstanceID)
	}

	if attribute.NextAttributeID != nextAttributeID {
		t.Errorf("NextAttributeID mismatch: got %d, want %d", attribute.NextAttributeID, nextAttributeID)
	}
}

func TestCreateAttributeWithNumericLabel(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "attributes.dat")

	as, err := NewAttributeStore(filePath)
	if err != nil {
		t.Fatalf("NewAttributeStore failed: %v", err)
	}
	defer as.Close()

	// Create attribute with numeric label, read it back
	labelValueID := uint64(55555)    // Points to numeric value in value store
	firstInstanceID := uint64(77777) // Points to instance
	nextAttributeID := uint64(0)     // No siblings

	attributeID, err := as.WriteAttribute(labelValueID, firstInstanceID, nextAttributeID)
	if err != nil {
		t.Fatalf("WriteAttribute failed: %v", err)
	}

	// Read it back
	attribute, err := as.ReadAttribute(attributeID)
	if err != nil {
		t.Fatalf("ReadAttribute failed: %v", err)
	}

	// Verify numeric label is stored correctly
	if attribute.LabelValueID != labelValueID {
		t.Errorf("Numeric label value ID mismatch: got %d, want %d", attribute.LabelValueID, labelValueID)
	}
}

func TestMultipleSiblingAttributes(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "attributes.dat")

	as, err := NewAttributeStore(filePath)
	if err != nil {
		t.Fatalf("NewAttributeStore failed: %v", err)
	}
	defer as.Close()

	// Create multiple sibling attributes and traverse the linked list
	// Create first attribute (will be head of chain)
	attr1ID, err := as.WriteAttribute(1001, 2001, 0) // No next initially
	if err != nil {
		t.Fatalf("WriteAttribute 1 failed: %v", err)
	}

	// Create second attribute
	attr2ID, err := as.WriteAttribute(1002, 2002, 0) // No next initially
	if err != nil {
		t.Fatalf("WriteAttribute 2 failed: %v", err)
	}

	// Create third attribute
	attr3ID, err := as.WriteAttribute(1003, 2003, 0) // No next initially
	if err != nil {
		t.Fatalf("WriteAttribute 3 failed: %v", err)
	}

	// Now link them: attr1 -> attr2 -> attr3
	err = as.UpdateAttributeNextID(attr1ID, attr2ID)
	if err != nil {
		t.Fatalf("Update attr1 next ID failed: %v", err)
	}

	err = as.UpdateAttributeNextID(attr2ID, attr3ID)
	if err != nil {
		t.Fatalf("Update attr2 next ID failed: %v", err)
	}

	// Traverse the linked list
	siblings, err := as.GetSiblingChain(attr1ID)
	if err != nil {
		t.Fatalf("GetSiblingChain failed: %v", err)
	}

	// Should have 3 siblings
	if len(siblings) != 3 {
		t.Errorf("Expected 3 siblings, got %d", len(siblings))
		return
	}

	// Verify order and content
	expectedLabels := []uint64{1001, 1002, 1003}
	for i, sibling := range siblings {
		if sibling.LabelValueID != expectedLabels[i] {
			t.Errorf("Sibling %d: expected label %d, got %d", i, expectedLabels[i], sibling.LabelValueID)
		}
	}

	// Verify linking
	if siblings[0].NextAttributeID != attr2ID {
		t.Errorf("First sibling should link to attr2 (%d), got %d", attr2ID, siblings[0].NextAttributeID)
	}

	if siblings[1].NextAttributeID != attr3ID {
		t.Errorf("Second sibling should link to attr3 (%d), got %d", attr3ID, siblings[1].NextAttributeID)
	}

	if siblings[2].NextAttributeID != 0 {
		t.Errorf("Third sibling should have no next (0), got %d", siblings[2].NextAttributeID)
	}
}

func TestLookupAttributeByLabel(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "attributes.dat")

	as, err := NewAttributeStore(filePath)
	if err != nil {
		t.Fatalf("NewAttributeStore failed: %v", err)
	}
	defer as.Close()

	// Create several attributes with different labels
	labels := []uint64{100, 200, 300, 400}
	var attributeIDs []uint64

	// Create all attributes
	for _, label := range labels {
		attrID, err := as.WriteAttribute(label, 9999, 0)
		if err != nil {
			t.Fatalf("WriteAttribute failed: %v", err)
		}
		attributeIDs = append(attributeIDs, attrID)
	}

	// Link them into a chain: attr1 -> attr2 -> attr3 -> attr4
	for i := 0; i < len(attributeIDs)-1; i++ {
		err = as.UpdateAttributeNextID(attributeIDs[i], attributeIDs[i+1])
		if err != nil {
			t.Fatalf("Update attribute next ID failed: %v", err)
		}
	}

	// Look up each attribute by label
	for i, expectedLabel := range labels {
		found, err := as.FindAttributeByLabel(attributeIDs[0], expectedLabel)
		if err != nil {
			t.Fatalf("FindAttributeByLabel failed: %v", err)
		}

		if found == nil {
			t.Errorf("Attribute with label %d not found", expectedLabel)
			continue
		}

		if found.ID != attributeIDs[i] {
			t.Errorf("Found wrong attribute ID for label %d: got %d, want %d", expectedLabel, found.ID, attributeIDs[i])
		}

		if found.LabelValueID != expectedLabel {
			t.Errorf("Found attribute has wrong label: got %d, want %d", found.LabelValueID, expectedLabel)
		}
	}

	// Try to find non-existent label
	notFound, err := as.FindAttributeByLabel(attributeIDs[0], 999)
	if err != nil {
		t.Fatalf("FindAttributeByLabel failed: %v", err)
	}

	if notFound != nil {
		t.Errorf("Should not have found attribute with non-existent label 999")
	}
}

func TestSerializeDeserializeAttribute(t *testing.T) {
	as := &AttributeStore{}

	original := Attribute{
		ID:              42,
		LabelValueID:    12345,
		FirstInstanceID: 67890,
		NextAttributeID: 99999,
	}

	// Serialize
	data := as.serializeAttribute(original)

	// Verify correct size
	if len(data) != AttributeRecordSize {
		t.Errorf("Serialized size wrong: got %d, expected %d", len(data), AttributeRecordSize)
	}

	// Deserialize
	deserialized := as.deserializeAttribute(data, original.ID)

	// Compare all fields
	if deserialized.ID != original.ID {
		t.Errorf("ID mismatch: got %d, want %d", deserialized.ID, original.ID)
	}

	if deserialized.LabelValueID != original.LabelValueID {
		t.Errorf("LabelValueID mismatch: got %d, want %d", deserialized.LabelValueID, original.LabelValueID)
	}

	if deserialized.FirstInstanceID != original.FirstInstanceID {
		t.Errorf("FirstInstanceID mismatch: got %d, want %d", deserialized.FirstInstanceID, original.FirstInstanceID)
	}

	if deserialized.NextAttributeID != original.NextAttributeID {
		t.Errorf("NextAttributeID mismatch: got %d, want %d", deserialized.NextAttributeID, original.NextAttributeID)
	}
}

func TestEmptyAttributeChain(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "attributes.dat")

	as, err := NewAttributeStore(filePath)
	if err != nil {
		t.Fatalf("NewAttributeStore failed: %v", err)
	}
	defer as.Close()

	// Query chain with ID 0 (no chain)
	siblings, err := as.GetSiblingChain(0)
	if err != nil {
		t.Fatalf("GetSiblingChain failed: %v", err)
	}

	if len(siblings) != 0 {
		t.Errorf("Empty chain should have length 0, got %d", len(siblings))
	}
}

func TestAttributeStorePersistence(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "attributes.dat")

	// Create store and write an attribute
	{
		as, err := NewAttributeStore(filePath)
		if err != nil {
			t.Fatalf("NewAttributeStore failed: %v", err)
		}

		attributeID, err := as.WriteAttribute(12345, 67890, 0)
		if err != nil {
			t.Fatalf("WriteAttribute failed: %v", err)
		}

		if attributeID != 1 {
			t.Errorf("Expected attribute ID 1, got %d", attributeID)
		}

		as.Close()
	}

	// Reopen store and verify attribute exists and nextID is correct
	{
		as, err := NewAttributeStore(filePath)
		if err != nil {
			t.Fatalf("NewAttributeStore (reopen) failed: %v", err)
		}
		defer as.Close()

		if as.nextID != 2 {
			t.Errorf("After reopen, nextID should be 2, got %d", as.nextID)
		}

		attribute, err := as.ReadAttribute(1)
		if err != nil {
			t.Fatalf("ReadAttribute after reopen failed: %v", err)
		}

		if attribute.LabelValueID != 12345 {
			t.Errorf("LabelValueID after reopen: got %d, want %d", attribute.LabelValueID, 12345)
		}

		if attribute.FirstInstanceID != 67890 {
			t.Errorf("FirstInstanceID after reopen: got %d, want %d", attribute.FirstInstanceID, 67890)
		}
	}
}

func TestInvalidAttributeID(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "attributes.dat")

	as, err := NewAttributeStore(filePath)
	if err != nil {
		t.Fatalf("NewAttributeStore failed: %v", err)
	}
	defer as.Close()

	// Try to read invalid attribute ID 0
	_, err = as.ReadAttribute(0)
	if err == nil {
		t.Errorf("Expected error when reading attribute ID 0")
	}

	expectedError := "invalid attribute ID 0"
	if err.Error() != expectedError {
		t.Errorf("Got error '%v', want '%v'", err.Error(), expectedError)
	}
}

func TestUpdateNextAttributeID(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "attributes.dat")

	as, err := NewAttributeStore(filePath)
	if err != nil {
		t.Fatalf("NewAttributeStore failed: %v", err)
	}
	defer as.Close()

	// Create an attribute
	attributeID, err := as.WriteAttribute(111, 222, 0)
	if err != nil {
		t.Fatalf("WriteAttribute failed: %v", err)
	}

	// Update its NextAttributeID
	newNextID := uint64(999)
	err = as.UpdateAttributeNextID(attributeID, newNextID)
	if err != nil {
		t.Fatalf("UpdateAttributeNextID failed: %v", err)
	}

	// Read back and verify update
	attribute, err := as.ReadAttribute(attributeID)
	if err != nil {
		t.Fatalf("ReadAttribute failed: %v", err)
	}

	if attribute.NextAttributeID != newNextID {
		t.Errorf("NextAttributeID not updated: got %d, want %d", attribute.NextAttributeID, newNextID)
	}

	// Verify other fields unchanged
	if attribute.LabelValueID != 111 {
		t.Errorf("LabelValueID changed unexpectedly: got %d, want %d", attribute.LabelValueID, 111)
	}

	if attribute.FirstInstanceID != 222 {
		t.Errorf("FirstInstanceID changed unexpectedly: got %d, want %d", attribute.FirstInstanceID, 222)
	}
}