package storage

import (
	"path/filepath"
	"testing"
	"time"
)

func TestNewInstanceStore(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "instances.dat")

	is, err := NewInstanceStore(filePath)
	if err != nil {
		t.Fatalf("NewInstanceStore failed: %v", err)
	}
	defer is.Close()

	// Verify initial state
	if is.nextID != 1 {
		t.Errorf("Initial nextID should be 1, got %d", is.nextID)
	}

	count := is.GetInstanceCount()
	if count != 0 {
		t.Errorf("Initial instance count should be 0, got %d", count)
	}
}

func TestCreateInstanceReadBack(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "instances.dat")

	is, err := NewInstanceStore(filePath)
	if err != nil {
		t.Fatalf("NewInstanceStore failed: %v", err)
	}
	defer is.Close()

	// Create instance, read it back, confirm all fields match
	valueID := uint64(12345)
	olderInstanceID := uint64(0) // First instance
	authorID := uint64(67890)

	beforeWrite := time.Now().UTC().UnixMicro()
	instanceID, err := is.WriteInstance(valueID, olderInstanceID, authorID)
	afterWrite := time.Now().UTC().UnixMicro()

	if err != nil {
		t.Fatalf("WriteInstance failed: %v", err)
	}

	if instanceID != 1 {
		t.Errorf("First instance ID should be 1, got %d", instanceID)
	}

	// Read it back
	instance, err := is.ReadInstance(instanceID)
	if err != nil {
		t.Fatalf("ReadInstance failed: %v", err)
	}

	// Confirm all fields match
	if instance.ID != instanceID {
		t.Errorf("Instance ID mismatch: got %d, want %d", instance.ID, instanceID)
	}

	if instance.ValueID != valueID {
		t.Errorf("ValueID mismatch: got %d, want %d", instance.ValueID, valueID)
	}

	if instance.OlderInstanceID != olderInstanceID {
		t.Errorf("OlderInstanceID mismatch: got %d, want %d", instance.OlderInstanceID, olderInstanceID)
	}

	if instance.AuthorID != authorID {
		t.Errorf("AuthorID mismatch: got %d, want %d", instance.AuthorID, authorID)
	}

	// Verify timestamp is within reasonable bounds (UTC, microsecond precision)
	if instance.Timestamp < beforeWrite || instance.Timestamp > afterWrite {
		t.Errorf("Timestamp %d is not within expected range [%d, %d]",
			instance.Timestamp, beforeWrite, afterWrite)
	}
}

func TestInstanceChainLinking(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "instances.dat")

	is, err := NewInstanceStore(filePath)
	if err != nil {
		t.Fatalf("NewInstanceStore failed: %v", err)
	}
	defer is.Close()

	// Create first instance
	valueID1 := uint64(100)
	authorID := uint64(999)

	// Small delay to ensure different timestamps
	time.Sleep(time.Microsecond * 10)

	instanceID1, err := is.WriteInstance(valueID1, 0, authorID)
	if err != nil {
		t.Fatalf("WriteInstance failed: %v", err)
	}

	// Small delay to ensure different timestamps
	time.Sleep(time.Microsecond * 10)

	// Create second instance for same attribute, confirm older_instance_id links to first
	valueID2 := uint64(200)
	instanceID2, err := is.WriteInstance(valueID2, instanceID1, authorID)
	if err != nil {
		t.Fatalf("WriteInstance failed: %v", err)
	}

	// Read both instances
	instance1, err := is.ReadInstance(instanceID1)
	if err != nil {
		t.Fatalf("ReadInstance 1 failed: %v", err)
	}

	instance2, err := is.ReadInstance(instanceID2)
	if err != nil {
		t.Fatalf("ReadInstance 2 failed: %v", err)
	}

	// Verify linking
	if instance1.OlderInstanceID != 0 {
		t.Errorf("First instance should have OlderInstanceID 0, got %d", instance1.OlderInstanceID)
	}

	if instance2.OlderInstanceID != instanceID1 {
		t.Errorf("Second instance should link to first (ID %d), got %d", instanceID1, instance2.OlderInstanceID)
	}

	// Verify timestamps are ordered correctly (newer should have larger timestamp)
	if instance2.Timestamp <= instance1.Timestamp {
		t.Errorf("Second instance timestamp (%d) should be greater than first (%d)",
			instance2.Timestamp, instance1.Timestamp)
	}
}

func TestInstanceChainWalk(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "instances.dat")

	is, err := NewInstanceStore(filePath)
	if err != nil {
		t.Fatalf("NewInstanceStore failed: %v", err)
	}
	defer is.Close()

	authorID := uint64(999)
	var instanceIDs []uint64

	// Create a chain of 5 instances - first instance has olderInstanceID = 0 (no previous)
	// Each subsequent instance links to the previous one
	for i := 0; i < 5; i++ {
		valueID := uint64(1000 + i)

		// Small delay to ensure different timestamps
		time.Sleep(time.Microsecond * 10)

		var olderInstanceID uint64 = 0
		if i > 0 {
			// Link to previous instance
			olderInstanceID = instanceIDs[i-1]
		}

		instanceID, err := is.WriteInstance(valueID, olderInstanceID, authorID)
		if err != nil {
			t.Fatalf("WriteInstance %d failed: %v", i, err)
		}

		instanceIDs = append(instanceIDs, instanceID)
	}

	// Query instance chain, walk from newest to oldest, confirm correct order
	newestID := instanceIDs[len(instanceIDs)-1] // Last created instance
	chain, err := is.GetInstanceChain(newestID)
	if err != nil {
		t.Fatalf("GetInstanceChain failed: %v", err)
	}

	// Should have 5 instances
	if len(chain) != 5 {
		t.Errorf("Expected 5 instances in chain, got %d", len(chain))
		return
	}

	// Verify order: chain[0] should be newest, chain[4] should be oldest
	for i, instance := range chain {
		expectedID := instanceIDs[len(instanceIDs)-1-i] // Reverse order
		if instance.ID != expectedID {
			t.Errorf("Chain position %d: expected ID %d, got %d", i, expectedID, instance.ID)
		}
	}

	// Validate chain order using the validation function
	err = is.ValidateChainOrder(chain)
	if err != nil {
		t.Errorf("Chain order validation failed: %v", err)
	}
}

func TestTimestampMonotonic(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "instances.dat")

	is, err := NewInstanceStore(filePath)
	if err != nil {
		t.Fatalf("NewInstanceStore failed: %v", err)
	}
	defer is.Close()

	authorID := uint64(999)
	var instances []Instance

	// Create multiple instances with delays
	var previousID uint64 = 0
	for i := 0; i < 3; i++ {
		valueID := uint64(2000 + i)

		// Ensure different timestamps
		time.Sleep(time.Millisecond)

		instanceID, err := is.WriteInstance(valueID, previousID, authorID)
		if err != nil {
			t.Fatalf("WriteInstance %d failed: %v", i, err)
		}

		instance, err := is.ReadInstance(instanceID)
		if err != nil {
			t.Fatalf("ReadInstance %d failed: %v", i, err)
		}

		instances = append(instances, instance)
		previousID = instanceID
	}

	// Confirm timestamps are monotonically increasing
	for i := 1; i < len(instances); i++ {
		current := instances[i]
		previous := instances[i-1]

		if current.Timestamp <= previous.Timestamp {
			t.Errorf("Timestamps not monotonic: instance %d (%d) should be > instance %d (%d)",
				current.ID, current.Timestamp, previous.ID, previous.Timestamp)
		}
	}

	// Verify all timestamps are in UTC (this is implicit in our implementation,
	// but we can check that they're reasonable values)
	now := time.Now().UTC().UnixMicro()
	oneHourAgo := now - (60 * 60 * 1000000) // 1 hour in microseconds

	for _, instance := range instances {
		if instance.Timestamp < oneHourAgo || instance.Timestamp > now {
			t.Errorf("Instance %d timestamp %d seems unreasonable (not recent UTC)",
				instance.ID, instance.Timestamp)
		}
	}
}

func TestSerializeDeserializeInstance(t *testing.T) {
	is := &InstanceStore{}

	original := Instance{
		ID:              42,
		ValueID:         12345,
		OlderInstanceID: 67890,
		Timestamp:       1641024000000000, // Some timestamp in microseconds
		AuthorID:        99999,
	}

	// Serialize
	data := is.serializeInstance(original)

	// Verify correct size
	if len(data) != InstanceRecordSize {
		t.Errorf("Serialized size wrong: got %d, expected %d", len(data), InstanceRecordSize)
	}

	// Deserialize
	deserialized := is.deserializeInstance(data, original.ID)

	// Compare all fields
	if deserialized.ID != original.ID {
		t.Errorf("ID mismatch: got %d, want %d", deserialized.ID, original.ID)
	}

	if deserialized.ValueID != original.ValueID {
		t.Errorf("ValueID mismatch: got %d, want %d", deserialized.ValueID, original.ValueID)
	}

	if deserialized.OlderInstanceID != original.OlderInstanceID {
		t.Errorf("OlderInstanceID mismatch: got %d, want %d", deserialized.OlderInstanceID, original.OlderInstanceID)
	}

	if deserialized.Timestamp != original.Timestamp {
		t.Errorf("Timestamp mismatch: got %d, want %d", deserialized.Timestamp, original.Timestamp)
	}

	if deserialized.AuthorID != original.AuthorID {
		t.Errorf("AuthorID mismatch: got %d, want %d", deserialized.AuthorID, original.AuthorID)
	}
}

func TestEmptyChain(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "instances.dat")

	is, err := NewInstanceStore(filePath)
	if err != nil {
		t.Fatalf("NewInstanceStore failed: %v", err)
	}
	defer is.Close()

	// Query chain with ID 0 (no chain)
	chain, err := is.GetInstanceChain(0)
	if err != nil {
		t.Fatalf("GetInstanceChain failed: %v", err)
	}

	if len(chain) != 0 {
		t.Errorf("Empty chain should have length 0, got %d", len(chain))
	}
}

func TestInstanceStorePersistence(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "instances.dat")

	// Create store and write an instance
	{
		is, err := NewInstanceStore(filePath)
		if err != nil {
			t.Fatalf("NewInstanceStore failed: %v", err)
		}

		instanceID, err := is.WriteInstance(12345, 0, 67890)
		if err != nil {
			t.Fatalf("WriteInstance failed: %v", err)
		}

		if instanceID != 1 {
			t.Errorf("Expected instance ID 1, got %d", instanceID)
		}

		is.Close()
	}

	// Reopen store and verify instance exists and nextID is correct
	{
		is, err := NewInstanceStore(filePath)
		if err != nil {
			t.Fatalf("NewInstanceStore (reopen) failed: %v", err)
		}
		defer is.Close()

		if is.nextID != 2 {
			t.Errorf("After reopen, nextID should be 2, got %d", is.nextID)
		}

		instance, err := is.ReadInstance(1)
		if err != nil {
			t.Fatalf("ReadInstance after reopen failed: %v", err)
		}

		if instance.ValueID != 12345 {
			t.Errorf("ValueID after reopen: got %d, want %d", instance.ValueID, 12345)
		}

		if instance.AuthorID != 67890 {
			t.Errorf("AuthorID after reopen: got %d, want %d", instance.AuthorID, 67890)
		}
	}
}