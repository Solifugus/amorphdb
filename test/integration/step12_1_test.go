package integration

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/solifugus/amorphdb/internal/storage"
)

// TestStep12_1_SingleNodeCore tests Step 12.1: Single Node Integration (Core functionality only)
func TestStep12_1_SingleNodeCore(t *testing.T) {
	t.Log("=== STEP 12.1: Single Node Integration Test (Core) ===")

	tmpDir, err := os.MkdirTemp("", "amorphdb_step12_1_")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Test 1: Storage Engine Lifecycle
	t.Log("Test 1: Storage engine startup and basic operations")
	testStorageEngineLifecycle(t, tmpDir)

	// Test 2: Data persistence across storage restarts
	t.Log("Test 2: Data persistence across storage restarts")
	testDataPersistenceCore(t, tmpDir)

	// Test 3: Temporal queries
	t.Log("Test 3: Temporal query functionality")
	testTemporalQueriesCore(t, tmpDir)

	// Test 4: Complex data structures
	t.Log("Test 4: Complex nested data structures")
	testComplexDataStructures(t, tmpDir)

	t.Log("✅ STEP 12.1 Single Node Integration Test PASSED!")
}

func testStorageEngineLifecycle(t *testing.T, dataDir string) {
	// Create storage tree
	tree, err := storage.NewStorageTree(dataDir)
	if err != nil {
		t.Fatalf("Failed to create storage tree: %v", err)
	}

	// Test basic write operations
	testValue := storage.Value{
		Data:    []byte("Integration test value"),
		TypeTag: storage.TypeText,
	}

	err = tree.Write([]string{"test", "basic", "value"}, testValue, 1000)
	if err != nil {
		t.Fatalf("Failed to write test value: %v", err)
	}

	// Test basic read operations
	readValue, err := tree.Read([]string{"test", "basic", "value"})
	if err != nil {
		t.Fatalf("Failed to read test value: %v", err)
	}

	if string(readValue.Data) != "Integration test value" {
		t.Errorf("Expected 'Integration test value', got '%s'", string(readValue.Data))
	}

	// Test number storage
	numberData := make([]byte, 8)
	// Store float64(42.5) as bytes (IEEE 754)
	binary_representation := uint64(0x4045400000000000) // 42.5 in IEEE 754
	for i := 0; i < 8; i++ {
		numberData[i] = byte(binary_representation >> (8 * (7 - i)))
	}

	numberValue := storage.Value{
		Data:    numberData,
		TypeTag: storage.TypeNumber,
	}

	err = tree.Write([]string{"test", "number"}, numberValue, 1000)
	if err != nil {
		t.Fatalf("Failed to write number value: %v", err)
	}

	readNumber, err := tree.Read([]string{"test", "number"})
	if err != nil {
		t.Fatalf("Failed to read number value: %v", err)
	}

	if len(readNumber.Data) != 8 {
		t.Errorf("Expected 8 bytes for number, got %d", len(readNumber.Data))
	}

	tree.Close()
	t.Log("Storage engine lifecycle test passed")
}

func testDataPersistenceCore(t *testing.T, dataDir string) {
	// First storage session
	tree1, err := storage.NewStorageTree(dataDir)
	if err != nil {
		t.Fatalf("Failed to create first storage tree: %v", err)
	}

	persistentValue := storage.Value{
		Data:    []byte("This should persist across restarts"),
		TypeTag: storage.TypeText,
	}

	err = tree1.Write([]string{"persistent", "data"}, persistentValue, 1000)
	if err != nil {
		t.Fatalf("Failed to write persistent data: %v", err)
	}

	// Write multiple values to test persistence
	for i := 0; i < 5; i++ {
		value := storage.Value{
			Data:    []byte(fmt.Sprintf("Persistent value %d", i)),
			TypeTag: storage.TypeText,
		}
		err = tree1.Write([]string{"persistent", "array", fmt.Sprintf("item_%d", i)}, value, 1000)
		if err != nil {
			t.Fatalf("Failed to write persistent array item %d: %v", i, err)
		}
	}

	tree1.Close()

	// Second storage session (simulating restart)
	tree2, err := storage.NewStorageTree(dataDir)
	if err != nil {
		t.Fatalf("Failed to create second storage tree: %v", err)
	}
	defer tree2.Close()

	// Check if persistent data survived
	readValue, err := tree2.Read([]string{"persistent", "data"})
	if err != nil {
		t.Fatalf("Failed to read persistent data after restart: %v", err)
	}

	if string(readValue.Data) != "This should persist across restarts" {
		t.Errorf("Persistent data was not preserved across restart")
	}

	// Check if array data survived
	for i := 0; i < 5; i++ {
		arrayValue, err := tree2.Read([]string{"persistent", "array", fmt.Sprintf("item_%d", i)})
		if err != nil {
			t.Fatalf("Failed to read persistent array item %d after restart: %v", i, err)
		}

		expected := fmt.Sprintf("Persistent value %d", i)
		if string(arrayValue.Data) != expected {
			t.Errorf("Array item %d mismatch: expected '%s', got '%s'", i, expected, string(arrayValue.Data))
		}
	}

	t.Log("Data persistence test passed")
}

func testTemporalQueriesCore(t *testing.T, dataDir string) {
	tree, err := storage.NewStorageTree(dataDir)
	if err != nil {
		t.Fatalf("Failed to create storage tree: %v", err)
	}
	defer tree.Close()

	path := []string{"temporal", "test", "value"}

	// Write first version
	value1 := storage.Value{
		Data:    []byte("First version"),
		TypeTag: storage.TypeText,
	}
	err = tree.Write(path, value1, 1000)
	if err != nil {
		t.Fatalf("Failed to write first temporal value: %v", err)
	}

	// Capture timestamp after first write (so we can query for this version)
	time.Sleep(10 * time.Millisecond) // Ensure some time passes
	time1 := time.Now().UTC().UnixMicro()

	// Wait and write second version
	time.Sleep(50 * time.Millisecond)
	value2 := storage.Value{
		Data:    []byte("Second version"),
		TypeTag: storage.TypeText,
	}
	err = tree.Write(path, value2, 1000)
	if err != nil {
		t.Fatalf("Failed to write second temporal value: %v", err)
	}

	// Capture timestamp after second write
	time.Sleep(10 * time.Millisecond)
	time2 := time.Now().UTC().UnixMicro()

	// Wait and write third version
	time.Sleep(50 * time.Millisecond)
	value3 := storage.Value{
		Data:    []byte("Third version"),
		TypeTag: storage.TypeText,
	}
	err = tree.Write(path, value3, 1000)
	if err != nil {
		t.Fatalf("Failed to write third temporal value: %v", err)
	}

	// Test current value
	currentValue, err := tree.Read(path)
	if err != nil {
		t.Fatalf("Failed to read current value: %v", err)
	}
	if string(currentValue.Data) != "Third version" {
		t.Errorf("Current value mismatch: expected 'Third version', got '%s'", string(currentValue.Data))
	}

	// Test historical queries (timestamps captured after writes, so should find correct versions)
	historicalValue1, err := tree.ReadAt(path, time1)
	if err != nil {
		t.Fatalf("Failed to read historical value 1: %v", err)
	}
	if string(historicalValue1.Data) != "First version" {
		t.Errorf("Historical value 1 mismatch: expected 'First version', got '%s'", string(historicalValue1.Data))
	}

	historicalValue2, err := tree.ReadAt(path, time2)
	if err != nil {
		t.Fatalf("Failed to read historical value 2: %v", err)
	}
	if string(historicalValue2.Data) != "Second version" {
		t.Errorf("Historical value 2 mismatch: expected 'Second version', got '%s'", string(historicalValue2.Data))
	}

	t.Log("Temporal queries test passed")
}

func testComplexDataStructures(t *testing.T, dataDir string) {
	tree, err := storage.NewStorageTree(dataDir)
	if err != nil {
		t.Fatalf("Failed to create storage tree: %v", err)
	}
	defer tree.Close()

	// Create a complex nested structure representing a user profile
	// user.profile.personal.name
	// user.profile.personal.email
	// user.profile.settings.theme
	// user.profile.settings.notifications.email
	// user.profile.settings.notifications.push

	userProfile := map[string]string{
		"user.profile.personal.name":                   "Alice Johnson",
		"user.profile.personal.email":                  "alice@example.com",
		"user.profile.settings.theme":                  "dark",
		"user.profile.settings.notifications.email":   "true",
		"user.profile.settings.notifications.push":    "false",
		"user.profile.metadata.created":                "2024-01-01",
		"user.profile.metadata.last_login":             "2024-03-07",
		"user.profile.preferences.language":            "en",
		"user.profile.preferences.timezone":            "UTC",
		"user.profile.permissions.admin":               "false",
		"user.profile.permissions.read":                "true",
		"user.profile.permissions.write":               "true",
	}

	// Write all the nested data
	for pathStr, valueStr := range userProfile {
		path := strings.Split(pathStr, ".")
		value := storage.Value{
			Data:    []byte(valueStr),
			TypeTag: storage.TypeText,
		}

		err = tree.Write(path, value, 1000)
		if err != nil {
			t.Fatalf("Failed to write %s: %v", pathStr, err)
		}
	}

	// Read back and verify all the nested data
	for pathStr, expectedValue := range userProfile {
		path := strings.Split(pathStr, ".")
		readValue, err := tree.Read(path)
		if err != nil {
			t.Fatalf("Failed to read %s: %v", pathStr, err)
		}

		if string(readValue.Data) != expectedValue {
			t.Errorf("Value mismatch for %s: expected '%s', got '%s'", pathStr, expectedValue, string(readValue.Data))
		}
	}

	// Test that we can update parts of the structure
	updatedEmail := storage.Value{
		Data:    []byte("alice.johnson@newdomain.com"),
		TypeTag: storage.TypeText,
	}

	err = tree.Write([]string{"user", "profile", "personal", "email"}, updatedEmail, 1000)
	if err != nil {
		t.Fatalf("Failed to update email: %v", err)
	}

	// Verify the update
	readUpdatedEmail, err := tree.Read([]string{"user", "profile", "personal", "email"})
	if err != nil {
		t.Fatalf("Failed to read updated email: %v", err)
	}

	if string(readUpdatedEmail.Data) != "alice.johnson@newdomain.com" {
		t.Errorf("Updated email mismatch")
	}

	// Verify other values are unchanged
	readName, err := tree.Read([]string{"user", "profile", "personal", "name"})
	if err != nil {
		t.Fatalf("Failed to read name after email update: %v", err)
	}

	if string(readName.Data) != "Alice Johnson" {
		t.Errorf("Name was unexpectedly changed during email update")
	}

	t.Log("Complex data structures test passed")
}
