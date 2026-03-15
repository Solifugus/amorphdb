// Package integration provides comprehensive end-to-end integration tests for AmorphDB data preservation and migration
package integration

import (
	"fmt"
	"testing"
	"time"

	"github.com/solifugus/amorphdb/internal/config"
	"github.com/solifugus/amorphdb/internal/identity"
	"github.com/solifugus/amorphdb/internal/mesh"
	"github.com/solifugus/amorphdb/internal/storage"
	"github.com/solifugus/amorphdb/internal/types"
)

// TestDataMigration_StandaloneToMeshToStandalone tests complete data preservation lifecycle
// This covers Scenario C from the development plan
func TestDataMigration_StandaloneToMeshToStandalone(t *testing.T) {
	// Step 1: Create standalone node with comprehensive data
	t.Log("Step 1: Creating standalone node with comprehensive data")

	cfg := &config.Config{
		Data: config.DataConfig{
			Directory: "/tmp/amorphdb-test-migration",
		},
		Network: config.NetworkConfig{
			Address: "127.0.0.1:8201",
		},
		Mesh: config.MeshConfig{
			Name: "", // Starts standalone
		},
	}

	localStorage := storage.NewMemoryTree()
	identityManager := identity.NewIdentityManager("", "migration-test-node")
	meshManager := mesh.NewMeshManager(cfg, localStorage, identityManager)

	// Create various types of test data in standalone mode
	testData := map[string]storage.Value{
		"users/alice/profile": {Data: []byte("Alice's profile data")},
		"users/alice/settings": {Data: []byte("{\"theme\": \"dark\", \"notifications\": true}")},
		"users/bob/profile": {Data: []byte("Bob's profile data")},
		"documents/report1": {Data: []byte("Important business report")},
		"documents/report2": {Data: []byte("Another important document")},
		"config/app_settings": {Data: []byte("{\"version\": \"1.0\", \"features\": [\"a\", \"b\"]}")},
		"config/database": {Data: []byte("database connection configuration")},
		"temp/session_123": {Data: []byte("temporary session data")},
		"logs/2024/03/14": {Data: []byte("log entries for today")},
	}

	// Write all test data
	for pathStr, value := range testData {
		path := types.ParsePathString(pathStr)
		err := localStorage.Set(path, value, 1, time.Now())
		if err != nil {
			t.Fatalf("Failed to set standalone data for %s: %v", pathStr, err)
		}
		t.Logf("Set standalone data: %s", pathStr)
	}

	// Verify all data is accessible in standalone mode
	for pathStr, expectedValue := range testData {
		path := types.ParsePathString(pathStr)
		retrievedValue, err := localStorage.Get(path, time.Now())
		if err != nil {
			t.Errorf("Failed to retrieve standalone data for %s: %v", pathStr, err)
		} else if string(retrievedValue.Data) != string(expectedValue.Data) {
			t.Errorf("Standalone data mismatch for %s: expected %q, got %q",
				pathStr, expectedValue.Data, retrievedValue.Data)
		}
	}

	// Step 2: Convert standalone to mesh founder
	t.Log("Step 2: Converting to mesh founder")

	meshName := "test-migration-mesh"
	err := meshManager.CreateMesh(meshName)
	if err != nil {
		t.Fatalf("Failed to create mesh: %v", err)
	}

	// Verify node is now in mesh
	if meshManager.GetMeshName() != meshName {
		t.Errorf("Expected mesh name %s, got %s", meshName, meshManager.GetMeshName())
	}

	nodeIdentity := identityManager.GetNodeIdentity()
	if nodeIdentity.MeshName != meshName {
		t.Errorf("Expected node mesh name %s, got %s", meshName, nodeIdentity.MeshName)
	}

	// Step 3: Verify data preservation in my.standalone.*
	t.Log("Step 3: Verifying data preservation in my.standalone.*")

	preservationErrors := 0
	for pathStr, expectedValue := range testData {
		// Original path should now be mapped to my.standalone.*
		preservedPathStr := "my.standalone." + pathStr
		preservedPath := types.ParsePathString(preservedPathStr)

		retrievedValue, err := localStorage.Get(preservedPath, time.Now())
		if err != nil {
			t.Errorf("Failed to retrieve preserved data for %s: %v", preservedPathStr, err)
			preservationErrors++
		} else if string(retrievedValue.Data) != string(expectedValue.Data) {
			t.Errorf("Preserved data mismatch for %s: expected %q, got %q",
				preservedPathStr, expectedValue.Data, retrievedValue.Data)
			preservationErrors++
		} else {
			t.Logf("Successfully verified preserved data: %s", preservedPathStr)
		}
	}

	if preservationErrors > 0 {
		t.Errorf("Found %d data preservation errors", preservationErrors)
	}

	// Step 4: Add new mesh-specific data
	t.Log("Step 4: Adding mesh-specific data")

	meshData := map[string]storage.Value{
		"world/shared/mesh_config": {Data: []byte("Mesh-wide configuration")},
		"world/shared/announcements": {Data: []byte("Mesh announcements")},
		"world/agent/" + nodeIdentity.ID + "/agent_data": {Data: []byte("Agent-specific data")},
		"users/charlie/profile": {Data: []byte("Charlie joined after mesh creation")},
	}

	for pathStr, value := range meshData {
		path := types.ParsePathString(pathStr)
		err := localStorage.Set(path, value, 1, time.Now())
		if err != nil {
			t.Fatalf("Failed to set mesh data for %s: %v", pathStr, err)
		}
	}

	// Step 5: Test manual data migration from preserved to mesh paths
	t.Log("Step 5: Testing manual data migration")

	// Migrate user profiles from standalone to mesh structure
	migrationCases := []struct {
		sourcePath string
		targetPath string
	}{
		{"my.standalone.users/alice/profile", "users/alice/profile"},
		{"my.standalone.users/bob/profile", "users/bob/profile"},
		{"my.standalone.config/app_settings", "world/shared/app_settings"},
	}

	for _, migration := range migrationCases {
		// Read from preserved location
		sourcePath := types.ParsePathString(migration.sourcePath)
		sourceValue, err := localStorage.Get(sourcePath, time.Now())
		if err != nil {
			t.Errorf("Failed to read migration source %s: %v", migration.sourcePath, err)
			continue
		}

		// Write to new mesh location
		targetPath := types.ParsePathString(migration.targetPath)
		err = localStorage.Set(targetPath, sourceValue, 1, time.Now())
		if err != nil {
			t.Errorf("Failed to write migration target %s: %v", migration.targetPath, err)
			continue
		}

		// Verify migration
		targetValue, err := localStorage.Get(targetPath, time.Now())
		if err != nil {
			t.Errorf("Failed to verify migration target %s: %v", migration.targetPath, err)
		} else if string(targetValue.Data) != string(sourceValue.Data) {
			t.Errorf("Migration data mismatch for %s->%s: expected %q, got %q",
				migration.sourcePath, migration.targetPath, sourceValue.Data, targetValue.Data)
		} else {
			t.Logf("Successfully migrated: %s -> %s", migration.sourcePath, migration.targetPath)
		}
	}

	// Step 6: Add another node to mesh for data sharing test
	t.Log("Step 6: Adding second node to test mesh data sharing")

	cfg2 := &config.Config{
		Data: config.DataConfig{
			Directory: "/tmp/amorphdb-test-migration-2",
		},
		Network: config.NetworkConfig{
			Address: "127.0.0.1:8202",
		},
		Mesh: config.MeshConfig{
			Name: "",
		},
	}

	localStorage2 := storage.NewMemoryTree()
	identityManager2 := identity.NewIdentityManager("", "migration-test-node-2")
	meshManager2 := mesh.NewMeshManager(cfg2, localStorage2, identityManager2)

	// Add some data to second node before joining
	preJoinData := storage.Value{Data: []byte("Data from second node before joining mesh")}
	preJoinPath := []string{"local", "node2_specific", "data"}
	err = localStorage2.Set(preJoinPath, preJoinData, 1, time.Now())
	if err != nil {
		t.Fatalf("Failed to set pre-join data: %v", err)
	}

	// Join mesh
	discoveredInfo := &mesh.MeshInfo{
		Name:    meshName,
		Founder: nodeIdentity.ID,
	}

	err = meshManager2.JoinDiscoveredMesh(discoveredInfo)
	if err != nil {
		t.Fatalf("Failed to join mesh: %v", err)
	}

	// Verify second node's data preservation
	preservedPathNode2 := []string{"my", "standalone", "local", "node2_specific", "data"}
	preservedValueNode2, err := localStorage2.Get(preservedPathNode2, time.Now())
	if err != nil {
		t.Errorf("Failed to retrieve node2 preserved data: %v", err)
	} else if string(preservedValueNode2.Data) != string(preJoinData.Data) {
		t.Errorf("Node2 data preservation failed: expected %q, got %q",
			preJoinData.Data, preservedValueNode2.Data)
	}

	// Test mesh-wide data sharing (simulate replication)
	sharedDataPath := []string{"world", "shared", "mesh_config"}
	sharedData, err := localStorage.Get(sharedDataPath, time.Now())
	if err != nil {
		t.Errorf("Failed to retrieve shared data from first node: %v", err)
	} else {
		// Simulate replication to second node
		err = localStorage2.Set(sharedDataPath, sharedData, 1, time.Now())
		if err != nil {
			t.Fatalf("Failed to simulate data replication: %v", err)
		}

		// Verify second node can access shared data
		replicatedData, err := localStorage2.Get(sharedDataPath, time.Now())
		if err != nil {
			t.Errorf("Failed to access replicated data on second node: %v", err)
		} else if string(replicatedData.Data) != string(sharedData.Data) {
			t.Errorf("Replicated data mismatch: expected %q, got %q",
				sharedData.Data, replicatedData.Data)
		}
	}

	// Step 7: First node leaves mesh (graceful departure)
	t.Log("Step 7: Testing graceful mesh departure")

	err = meshManager.LeavePrimaryMesh()
	if err != nil {
		t.Fatalf("Failed to leave mesh: %v", err)
	}

	// Verify first node is now standalone
	if meshManager.GetMeshName() != "" {
		t.Error("First node should be standalone after leaving mesh")
	}

	// Step 8: Verify data accessibility after mesh departure
	t.Log("Step 8: Verifying data accessibility after mesh departure")

	// Original preserved data should still be accessible
	for pathStr, expectedValue := range testData {
		preservedPathStr := "my.standalone." + pathStr
		preservedPath := types.ParsePathString(preservedPathStr)

		retrievedValue, err := localStorage.Get(preservedPath, time.Now())
		if err != nil {
			t.Errorf("Failed to retrieve data after mesh departure for %s: %v", preservedPathStr, err)
		} else if string(retrievedValue.Data) != string(expectedValue.Data) {
			t.Errorf("Data corrupted after mesh departure for %s: expected %q, got %q",
				preservedPathStr, expectedValue.Data, retrievedValue.Data)
		}
	}

	// Mesh-specific data should be preserved in a mesh-specific namespace
	meshSpecificPath := []string{"my", meshName, "users", "charlie", "profile"}
	meshSpecificData, err := localStorage.Get(meshSpecificPath, time.Now())
	if err != nil {
		t.Errorf("Failed to access mesh-specific data after departure: %v", err)
	} else {
		expectedData := meshData["users/charlie/profile"]
		if string(meshSpecificData.Data) != string(expectedData.Data) {
			t.Errorf("Mesh-specific data mismatch: expected %q, got %q",
				expectedData.Data, meshSpecificData.Data)
		}
	}

	// Step 9: Test data restoration and cleanup options
	t.Log("Step 9: Testing data restoration options")

	// Option 1: Restore specific data from preserved namespaces
	restorationCases := []struct {
		preservedPath string
		restorePath   string
	}{
		{"my.standalone.users/alice/profile", "users/alice/profile"},
		{"my.standalone.config/app_settings", "config/app_settings"},
	}

	for _, restoration := range restorationCases {
		preservedPath := types.ParsePathString(restoration.preservedPath)
		preservedData, err := localStorage.Get(preservedPath, time.Now())
		if err != nil {
			t.Errorf("Failed to read preserved data for restoration %s: %v", restoration.preservedPath, err)
			continue
		}

		restorePath := types.ParsePathString(restoration.restorePath)
		err = localStorage.Set(restorePath, preservedData, 1, time.Now())
		if err != nil {
			t.Errorf("Failed to restore data to %s: %v", restoration.restorePath, err)
			continue
		}

		// Verify restoration
		restoredData, err := localStorage.Get(restorePath, time.Now())
		if err != nil {
			t.Errorf("Failed to verify restored data %s: %v", restoration.restorePath, err)
		} else if string(restoredData.Data) != string(preservedData.Data) {
			t.Errorf("Data restoration failed for %s: expected %q, got %q",
				restoration.restorePath, preservedData.Data, restoredData.Data)
		} else {
			t.Logf("Successfully restored: %s -> %s", restoration.preservedPath, restoration.restorePath)
		}
	}

	t.Log("Data migration and preservation test completed successfully")
}

// TestDataMigration_MultipleTransitions tests multiple mesh transitions
func TestDataMigration_MultipleTransitions(t *testing.T) {
	t.Log("Testing multiple mesh transitions and data preservation")

	cfg := &config.Config{
		Mesh: config.MeshConfig{Name: ""},
	}
	localStorage := storage.NewMemoryTree()
	identityManager := identity.NewIdentityManager("", "multi-transition-node")
	meshManager := mesh.NewMeshManager(cfg, localStorage, identityManager)

	// Phase 1: Standalone
	originalData := storage.Value{Data: []byte("Original standalone data")}
	originalPath := []string{"data", "original"}
	err := localStorage.Set(originalPath, originalData, 1, time.Now())
	if err != nil {
		t.Fatalf("Failed to set original data: %v", err)
	}

	// Phase 2: Join Mesh A
	err = meshManager.CreateMesh("mesh-a")
	if err != nil {
		t.Fatalf("Failed to create mesh-a: %v", err)
	}

	meshAData := storage.Value{Data: []byte("Data added while in mesh-a")}
	meshAPath := []string{"data", "mesh_a_data"}
	err = localStorage.Set(meshAPath, meshAData, 1, time.Now())
	if err != nil {
		t.Fatalf("Failed to set mesh-a data: %v", err)
	}

	// Leave Mesh A
	err = meshManager.LeavePrimaryMesh()
	if err != nil {
		t.Fatalf("Failed to leave mesh-a: %v", err)
	}

	// Phase 3: Join Mesh B
	err = meshManager.CreateMesh("mesh-b")
	if err != nil {
		t.Fatalf("Failed to create mesh-b: %v", err)
	}

	meshBData := storage.Value{Data: []byte("Data added while in mesh-b")}
	meshBPath := []string{"data", "mesh_b_data"}
	err = localStorage.Set(meshBPath, meshBData, 1, time.Now())
	if err != nil {
		t.Fatalf("Failed to set mesh-b data: %v", err)
	}

	// Leave Mesh B
	err = meshManager.LeavePrimaryMesh()
	if err != nil {
		t.Fatalf("Failed to leave mesh-b: %v", err)
	}

	// Verification: All data should be preserved in appropriate namespaces
	testCases := []struct {
		description  string
		expectedPath string
		expectedData storage.Value
	}{
		{
			description:  "Original standalone data",
			expectedPath: "my.standalone.data.original",
			expectedData: originalData,
		},
		{
			description:  "Mesh-A data",
			expectedPath: "my.mesh-a.data.mesh_a_data",
			expectedData: meshAData,
		},
		{
			description:  "Mesh-B data",
			expectedPath: "my.mesh-b.data.mesh_b_data",
			expectedData: meshBData,
		},
	}

	for _, test := range testCases {
		path := types.ParsePathString(test.expectedPath)
		retrievedData, err := localStorage.Get(path, time.Now())
		if err != nil {
			t.Errorf("Failed to retrieve %s: %v", test.description, err)
		} else if string(retrievedData.Data) != string(test.expectedData.Data) {
			t.Errorf("%s mismatch: expected %q, got %q",
				test.description, test.expectedData.Data, retrievedData.Data)
		} else {
			t.Logf("Successfully verified %s at %s", test.description, test.expectedPath)
		}
	}

	t.Log("Multiple transitions test completed")
}

// TestDataMigration_LargeDatasets tests preservation with large amounts of data
func TestDataMigration_LargeDatasets(t *testing.T) {
	t.Log("Testing data migration with large datasets")

	cfg := &config.Config{
		Mesh: config.MeshConfig{Name: ""},
	}
	localStorage := storage.NewMemoryTree()
	identityManager := identity.NewIdentityManager("", "large-data-node")
	meshManager := mesh.NewMeshManager(cfg, localStorage, identityManager)

	// Create large dataset in standalone mode
	dataCount := 1000
	largeDataset := make(map[string]storage.Value)

	t.Logf("Creating %d data entries", dataCount)
	for i := 0; i < dataCount; i++ {
		pathStr := fmt.Sprintf("dataset/category_%d/item_%d", i/100, i)
		value := storage.Value{Data: []byte(fmt.Sprintf("Large dataset item %d with content", i))}
		largeDataset[pathStr] = value

		path := types.ParsePathString(pathStr)
		err := localStorage.Set(path, value, 1, time.Now())
		if err != nil {
			t.Fatalf("Failed to set large dataset item %d: %v", i, err)
		}

		if i%100 == 0 {
			t.Logf("Created %d/%d data entries", i+1, dataCount)
		}
	}

	// Convert to mesh
	err := meshManager.CreateMesh("large-data-mesh")
	if err != nil {
		t.Fatalf("Failed to create mesh: %v", err)
	}

	// Verify all data is preserved
	t.Log("Verifying data preservation for large dataset")
	verificationErrors := 0
	for pathStr, expectedValue := range largeDataset {
		preservedPathStr := "my.standalone." + pathStr
		preservedPath := types.ParsePathString(preservedPathStr)

		retrievedValue, err := localStorage.Get(preservedPath, time.Now())
		if err != nil {
			t.Errorf("Failed to retrieve preserved large data item %s: %v", preservedPathStr, err)
			verificationErrors++
		} else if string(retrievedValue.Data) != string(expectedValue.Data) {
			t.Errorf("Large data preservation failed for %s", preservedPathStr)
			verificationErrors++
		}

		// Avoid overwhelming test output
		if verificationErrors > 10 {
			t.Errorf("Too many verification errors (%d), stopping verification", verificationErrors)
			break
		}
	}

	if verificationErrors == 0 {
		t.Logf("Successfully verified preservation of all %d large dataset items", dataCount)
	} else {
		t.Errorf("Found %d errors in large dataset preservation", verificationErrors)
	}

	t.Log("Large dataset migration test completed")
}