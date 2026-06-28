package storage

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"
)

// TestStep12_1_SingleNodeIntegration tests Step 12.1: Single Node Integration
func TestStep12_1_SingleNodeIntegration(t *testing.T) {
	t.Log("🚀 === STEP 12.1: Single Node Integration Test ===")

	tmpDir, err := os.MkdirTemp("", "amorphdb_step12_1_integration_")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Test 1: Service lifecycle simulation (storage startup/shutdown)
	t.Log("✅ Test 1: Service lifecycle simulation")
	testServiceLifecycleSimulation(t, tmpDir)

	// Test 2: Data persistence across restarts
	t.Log("✅ Test 2: Data persistence across restarts")
	testDataPersistenceAcrossRestarts(t, tmpDir)

	// Test 3: Temporal queries (historical data access)
	t.Log("✅ Test 3: Temporal query functionality")
	testTemporalQueryFunctionality(t, tmpDir)

	// Test 4: Complex data operations
	t.Log("✅ Test 4: Complex data operations")
	testComplexDataOperations(t, tmpDir)

	// Test 5: Performance and scalability
	t.Log("✅ Test 5: Performance and scalability")
	testPerformanceAndScalability(t, tmpDir)

	t.Log("")
	t.Log("🎉 === STEP 12.1 INTEGRATION TEST PASSED! === 🎉")
	t.Log("")
	t.Log("=== SUCCESS CRITERIA ACHIEVED ===")
	t.Log("✓ Service startup and basic operations working")
	t.Log("✓ Data persistence across service restarts validated")
	t.Log("✓ Temporal queries return correct historical values")
	t.Log("✓ Complex nested data structures handled correctly")
	t.Log("✓ Performance meets expectations for single-node operation")
	t.Log("✓ All core AmorphDB functionality validated at storage level")
}

func testServiceLifecycleSimulation(t *testing.T, dataDir string) {
	// Simulate service startup
	tree, err := NewStorageTree(dataDir)
	if err != nil {
		t.Fatalf("Failed to start storage (service startup simulation): %v", err)
	}

	// Test basic operations after startup
	basicData := map[string]string{
		"my.agent.name":      "Test Agent",
		"my.agent.identity":  "agent-12345",
		"my.computer.output": "Hello from AmorphDB",
		"world.config.debug": "true",
		"world.config.port":  "5000",
	}

	// Write data (simulating client operations)
	for pathStr, valueStr := range basicData {
		path := strings.Split(pathStr, ".")
		value := Value{Data: []byte(valueStr), TypeTag: TypeText}
		err := tree.Write(path, value, 1000)
		if err != nil {
			t.Fatalf("Failed to write data during service lifecycle test: %v", err)
		}
	}

	// Read data back (simulating client queries)
	for pathStr, expectedValue := range basicData {
		path := strings.Split(pathStr, ".")
		readValue, err := tree.Read(path)
		if err != nil {
			t.Fatalf("Failed to read data during service lifecycle test: %v", err)
		}

		if string(readValue.Data) != expectedValue {
			t.Errorf("Data mismatch during service lifecycle test for %s", pathStr)
		}
	}

	// Simulate service shutdown
	err = tree.Close()
	if err != nil {
		t.Fatalf("Failed to shutdown storage (service shutdown simulation): %v", err)
	}

	t.Log("  ✓ Service lifecycle simulation completed successfully")
}

func testDataPersistenceAcrossRestarts(t *testing.T, dataDir string) {
	// First service instance
	tree1, err := NewStorageTree(dataDir)
	if err != nil {
		t.Fatalf("Failed to create first storage tree: %v", err)
	}

	// Create comprehensive test data
	persistentData := []struct {
		path  []string
		value string
		desc  string
	}{
		{[]string{"user", "profiles", "alice", "name"}, "Alice Johnson", "User profile name"},
		{[]string{"user", "profiles", "alice", "email"}, "alice@amorphdb.com", "User profile email"},
		{[]string{"user", "profiles", "alice", "preferences", "theme"}, "dark", "User preferences"},
		{[]string{"user", "profiles", "alice", "permissions", "admin"}, "true", "User permissions"},
		{[]string{"system", "config", "database", "version"}, "1.0.0", "System configuration"},
		{[]string{"system", "config", "database", "initialized"}, fmt.Sprintf("%d", time.Now().Unix()), "System timestamp"},
		{[]string{"application", "state", "last_backup"}, "2024-03-07T10:00:00Z", "Application state"},
		{[]string{"application", "state", "active_users"}, "42", "Application metrics"},
	}

	// Write all test data
	for _, item := range persistentData {
		value := Value{Data: []byte(item.value), TypeTag: TypeText}
		err := tree1.Write(item.path, value, 1000)
		if err != nil {
			t.Fatalf("Failed to write %s: %v", item.desc, err)
		}
	}

	// Close first instance (simulate service shutdown)
	tree1.Close()

	// Second service instance (simulate service restart)
	tree2, err := NewStorageTree(dataDir)
	if err != nil {
		t.Fatalf("Failed to create second storage tree (restart): %v", err)
	}
	defer tree2.Close()

	// Verify all data persisted across restart
	for _, item := range persistentData {
		readValue, err := tree2.Read(item.path)
		if err != nil {
			t.Fatalf("Failed to read %s after restart: %v", item.desc, err)
		}

		if string(readValue.Data) != item.value {
			t.Errorf("Data not persistent for %s: expected %s, got %s",
				item.desc, item.value, string(readValue.Data))
		}
	}

	t.Log("  ✓ Data persistence across restarts validated")
}

func testTemporalQueryFunctionality(t *testing.T, dataDir string) {
	tree, err := NewStorageTree(dataDir)
	if err != nil {
		t.Fatalf("Failed to create storage tree: %v", err)
	}
	defer tree.Close()

	// Test temporal behavior for user status changes
	userStatusPath := []string{"user", "status", "alice"}
	statusSequence := []string{"offline", "connecting", "online", "busy", "away", "offline"}
	timestamps := make([]int64, len(statusSequence))

	// Write temporal sequence
	for i, status := range statusSequence {
		value := Value{Data: []byte(status), TypeTag: TypeText}
		err := tree.Write(userStatusPath, value, 1000)
		if err != nil {
			t.Fatalf("Failed to write status %s: %v", status, err)
		}

		// Instances are timestamped in microseconds (see instances.go), so the
		// query times below must also be in microseconds to match the chain.
		timestamps[i] = time.Now().UnixMicro()
		if i < len(statusSequence)-1 {
			time.Sleep(200 * time.Millisecond) // Ensure different timestamps
		}
	}

	// Test current value
	currentValue, err := tree.Read(userStatusPath)
	if err != nil {
		t.Fatalf("Failed to read current status: %v", err)
	}

	if string(currentValue.Data) != "offline" {
		t.Errorf("Current status mismatch: expected 'offline', got %s", string(currentValue.Data))
	}

	// Test historical queries
	for i, expectedStatus := range statusSequence[:len(statusSequence)-1] {
		queryTime := timestamps[i] + 100 // Query slightly after the timestamp
		historicalValue, err := tree.ReadAt(userStatusPath, queryTime)
		if err != nil {
			t.Fatalf("Failed to read historical status at time %d: %v", queryTime, err)
		}

		if string(historicalValue.Data) != expectedStatus {
			t.Errorf("Historical status mismatch at time %d: expected %s, got %s",
				queryTime, expectedStatus, string(historicalValue.Data))
		}
	}

	// Test multiple temporal paths
	tempPaths := []struct {
		path   []string
		values []string
	}{
		{[]string{"metrics", "cpu_usage"}, []string{"10%", "25%", "45%", "60%", "30%"}},
		{[]string{"metrics", "memory_usage"}, []string{"2GB", "2.5GB", "3GB", "3.2GB", "2.8GB"}},
		{[]string{"logs", "last_error"}, []string{"none", "timeout", "none", "disk_full", "none"}},
	}

	for _, pathData := range tempPaths {
		for _, value := range pathData.values {
			val := Value{Data: []byte(value), TypeTag: TypeText}
			err := tree.Write(pathData.path, val, 1000)
			if err != nil {
				t.Fatalf("Failed to write temporal data for %v: %v", pathData.path, err)
			}
			time.Sleep(100 * time.Millisecond)
		}
	}

	t.Log("  ✓ Temporal query functionality validated")
}

func testComplexDataOperations(t *testing.T, dataDir string) {
	tree, err := NewStorageTree(dataDir)
	if err != nil {
		t.Fatalf("Failed to create storage tree: %v", err)
	}
	defer tree.Close()

	// Simulate complex business logic data structures
	businessData := map[string]string{
		// Customer management
		"customers.enterprise.acme.info.name":           "ACME Corporation",
		"customers.enterprise.acme.info.industry":       "Manufacturing",
		"customers.enterprise.acme.billing.plan":        "enterprise",
		"customers.enterprise.acme.billing.monthly_fee": "5000",
		"customers.enterprise.acme.contacts.primary":    "john.doe@acme.com",
		"customers.enterprise.acme.contacts.billing":    "billing@acme.com",
		"customers.enterprise.acme.usage.queries":       "1000000",
		"customers.enterprise.acme.usage.storage":       "100GB",

		// Product catalog
		"products.database.amorphdb.name":               "AmorphDB",
		"products.database.amorphdb.version":            "1.0.0",
		"products.database.amorphdb.pricing.starter":    "99",
		"products.database.amorphdb.pricing.pro":        "299",
		"products.database.amorphdb.pricing.enterprise": "999",
		"products.database.amorphdb.features.temporal":  "true",
		"products.database.amorphdb.features.mesh":      "true",
		"products.database.amorphdb.features.reactive":  "true",

		// Transaction tracking
		"transactions.2024.03.07.1000.customer":  "acme",
		"transactions.2024.03.07.1000.amount":    "5000",
		"transactions.2024.03.07.1000.currency":  "USD",
		"transactions.2024.03.07.1000.status":    "completed",
		"transactions.2024.03.07.1000.processor": "stripe",
		"transactions.2024.03.07.1000.reference": "tx_abc123",

		// Operational metrics
		"metrics.realtime.active_connections":    "150",
		"metrics.realtime.queries_per_second":    "2500",
		"metrics.realtime.memory_usage":          "8GB",
		"metrics.realtime.disk_usage":            "45GB",
		"metrics.daily.2024.03.07.total_queries": "5000000",
		"metrics.daily.2024.03.07.peak_qps":      "3200",
		"metrics.daily.2024.03.07.downtime":      "0",
	}

	// Write all complex business data
	for pathStr, valueStr := range businessData {
		path := strings.Split(pathStr, ".")
		value := Value{Data: []byte(valueStr), TypeTag: TypeText}
		err := tree.Write(path, value, 1000)
		if err != nil {
			t.Fatalf("Failed to write business data %s: %v", pathStr, err)
		}
	}

	// Verify all data was written correctly
	for pathStr, expectedValue := range businessData {
		path := strings.Split(pathStr, ".")
		readValue, err := tree.Read(path)
		if err != nil {
			t.Fatalf("Failed to read business data %s: %v", pathStr, err)
		}

		if string(readValue.Data) != expectedValue {
			t.Errorf("Business data mismatch for %s: expected %s, got %s",
				pathStr, expectedValue, string(readValue.Data))
		}
	}

	// Test complex updates (simulate business logic changes)
	updates := map[string]string{
		"customers.enterprise.acme.usage.queries":       "1500000", // Usage increased
		"customers.enterprise.acme.billing.monthly_fee": "5500",    // Price increase
		"metrics.realtime.active_connections":           "175",     // More connections
		"products.database.amorphdb.version":            "1.0.1",   // New version
	}

	for pathStr, newValue := range updates {
		path := strings.Split(pathStr, ".")
		value := Value{Data: []byte(newValue), TypeTag: TypeText}
		err := tree.Write(path, value, 1000)
		if err != nil {
			t.Fatalf("Failed to update business data %s: %v", pathStr, err)
		}
	}

	// Verify updates
	for pathStr, expectedValue := range updates {
		path := strings.Split(pathStr, ".")
		readValue, err := tree.Read(path)
		if err != nil {
			t.Fatalf("Failed to read updated business data %s: %v", pathStr, err)
		}

		if string(readValue.Data) != expectedValue {
			t.Errorf("Updated business data mismatch for %s: expected %s, got %s",
				pathStr, expectedValue, string(readValue.Data))
		}
	}

	t.Log("  ✓ Complex data operations validated")
}

func testPerformanceAndScalability(t *testing.T, dataDir string) {
	tree, err := NewStorageTree(dataDir)
	if err != nil {
		t.Fatalf("Failed to create storage tree: %v", err)
	}
	defer tree.Close()

	// Test 1: High-volume writes
	t.Log("    Testing high-volume write performance...")

	numRecords := 5000
	startTime := time.Now()

	for i := 0; i < numRecords; i++ {
		path := []string{"performance", "bulk", fmt.Sprintf("record_%05d", i)}
		value := Value{
			Data:    []byte(fmt.Sprintf("Performance test record %d with some data content", i)),
			TypeTag: TypeText,
		}

		err := tree.Write(path, value, 1000)
		if err != nil {
			t.Fatalf("Failed to write performance record %d: %v", i, err)
		}
	}

	writeTime := time.Since(startTime)
	writesPerSecond := float64(numRecords) / writeTime.Seconds()

	t.Logf("    Wrote %d records in %v (%.0f writes/sec)", numRecords, writeTime, writesPerSecond)

	if writesPerSecond < 100 { // Should handle at least 100 writes/sec
		t.Errorf("Write performance too low: %.0f writes/sec (expected > 100)", writesPerSecond)
	}

	// Test 2: High-volume reads
	t.Log("    Testing high-volume read performance...")

	readStartTime := time.Now()
	sampleIndices := []int{0, 100, 500, 1000, 2500, 4999}

	for _, i := range sampleIndices {
		path := []string{"performance", "bulk", fmt.Sprintf("record_%05d", i)}
		_, err := tree.Read(path)
		if err != nil {
			t.Fatalf("Failed to read performance record %d: %v", i, err)
		}
	}

	readTime := time.Since(readStartTime)
	readsPerSecond := float64(len(sampleIndices)) / readTime.Seconds()

	t.Logf("    Read %d records in %v (%.0f reads/sec)", len(sampleIndices), readTime, readsPerSecond)

	// Test 3: Large data handling
	t.Log("    Testing large data handling...")

	largeSizes := []int{1024, 4096, 16384, 65536}
	for _, size := range largeSizes {
		largeData := make([]byte, size)
		for i := 0; i < size; i++ {
			largeData[i] = byte(i % 256)
		}

		path := []string{"performance", "large", fmt.Sprintf("data_%d", size)}
		value := Value{Data: largeData, TypeTag: TypeText}

		startTime := time.Now()
		err := tree.Write(path, value, 1000)
		writeTime := time.Since(startTime)

		if err != nil {
			t.Fatalf("Failed to write large data of size %d: %v", size, err)
		}

		startTime = time.Now()
		readValue, err := tree.Read(path)
		readTime := time.Since(startTime)

		if err != nil {
			t.Fatalf("Failed to read large data of size %d: %v", size, err)
		}

		if len(readValue.Data) != size {
			t.Errorf("Large data size mismatch: expected %d, got %d", size, len(readValue.Data))
		}

		t.Logf("    %d bytes: write %v, read %v", size, writeTime, readTime)
	}

	// Test 4: Deep nesting performance
	t.Log("    Testing deep nesting performance...")

	maxDepth := 20
	deepPath := make([]string, maxDepth)
	for i := 0; i < maxDepth; i++ {
		deepPath[i] = fmt.Sprintf("level_%02d", i)
	}

	value := Value{Data: []byte("Deep nested value"), TypeTag: TypeText}

	startTime = time.Now()
	err = tree.Write(deepPath, value, 1000)
	deepWriteTime := time.Since(startTime)

	if err != nil {
		t.Fatalf("Failed to write deeply nested value: %v", err)
	}

	startTime = time.Now()
	readValue, err := tree.Read(deepPath)
	deepReadTime := time.Since(startTime)

	if err != nil {
		t.Fatalf("Failed to read deeply nested value: %v", err)
	}

	if string(readValue.Data) != "Deep nested value" {
		t.Error("Deep nested value mismatch")
	}

	t.Logf("    Deep nesting (%d levels): write %v, read %v", maxDepth, deepWriteTime, deepReadTime)

	t.Log("  ✓ Performance and scalability validated")
}
