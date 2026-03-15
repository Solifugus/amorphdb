// Package integration provides integration tests for AmorphDB multi-mesh bridge functionality
package integration

import (
	"testing"

	"github.com/solifugus/amorphdb/internal/identity"
	"github.com/solifugus/amorphdb/internal/interpreter"
	"github.com/solifugus/amorphdb/internal/storage"
	"github.com/solifugus/amorphdb/internal/types"
)

// TestBridgeDataAccess_EndToEnd demonstrates complete bridge data access functionality
func TestBridgeDataAccess_EndToEnd(t *testing.T) {
	// Create local storage and bridge storage
	localStorage := storage.NewMemoryTree()
	bridgeStorage := storage.NewBridgeStorage(localStorage)

	// Create bridge scope resolver
	resolver := interpreter.NewBridgeScopeResolver(bridgeStorage)

	// Create path validator
	validator := types.NewPathValidator()

	// Step 1: Register a bridge connection
	bridgeIdentity := &identity.Identity{
		ID:       "integration-bridge-abc123",
		MeshName: "partner-network",
	}

	err := resolver.RegisterBridge("partner-network", bridgeIdentity, "connected", "192.168.10.100:5000")
	if err != nil {
		t.Fatalf("Failed to register bridge: %v", err)
	}

	// Step 2: Test path validation and parsing
	bridgePath := "my.partner-network.users.alice.profile"

	// Validate the path format
	pathComponents := types.ParsePathString(bridgePath)
	parsed := validator.ParsePath(pathComponents)
	if parsed.Type != types.BridgePath {
		t.Errorf("Expected bridge path type, got %v", parsed.Type)
	}
	if parsed.MeshName != "partner-network" {
		t.Errorf("Expected mesh name 'partner-network', got %s", parsed.MeshName)
	}

	// Step 3: Test bridge scope resolution
	scope, err := resolver.ResolveBridgePath(bridgePath)
	if err != nil {
		t.Fatalf("Failed to resolve bridge path: %v", err)
	}
	if scope == nil {
		t.Fatal("Expected bridge scope, got nil")
	}

	expectedAgentHome := "world.agent.integration-bridge-abc123"
	if scope.AgentHome != expectedAgentHome {
		t.Errorf("Expected agent home %s, got %s", expectedAgentHome, scope.AgentHome)
	}

	// Step 4: Test target path conversion
	targetPath, err := scope.GetTargetPath()
	if err != nil {
		t.Fatalf("Failed to get target path: %v", err)
	}

	expectedTargetPath := []string{"world", "agent", "integration-bridge-abc123", "users", "alice", "profile"}
	if !equalStringSlices(targetPath, expectedTargetPath) {
		t.Errorf("Expected target path %v, got %v", expectedTargetPath, targetPath)
	}

	// Step 5: Test bridge data operations
	testValue := storage.Value{Data: []byte("Alice's Profile Data")}
	author := uint64(1)

	// Write bridge data
	err = bridgeStorage.WriteBridgeData(pathComponents, testValue, author)
	if err != nil {
		t.Fatalf("Failed to write bridge data: %v", err)
	}

	// Read bridge data back
	readValue, err := bridgeStorage.ReadBridgeData(pathComponents)
	if err != nil {
		t.Fatalf("Failed to read bridge data: %v", err)
	}

	if string(readValue.Data) != string(testValue.Data) {
		t.Errorf("Expected data %q, got %q", testValue.Data, readValue.Data)
	}

	// Step 6: Test path type checking
	if !resolver.IsBridgePath(bridgePath) {
		t.Error("Expected bridge path to be recognized")
	}

	if !bridgeStorage.IsBridgePath(pathComponents) {
		t.Error("Expected bridge storage to recognize bridge path")
	}

	// Test non-bridge path
	localPath := "local.users.bob"
	if resolver.IsBridgePath(localPath) {
		t.Error("Expected local path to not be recognized as bridge path")
	}

	// Step 7: Test bridge access validation
	err = scope.ValidateAccess()
	if err != nil {
		t.Errorf("Expected bridge access to be valid, got error: %v", err)
	}

	// Step 8: Test bridge disconnection
	err = resolver.UpdateBridgeStatus("partner-network", "disconnected")
	if err != nil {
		t.Fatalf("Failed to update bridge status: %v", err)
	}

	// Access should now fail
	err = scope.ValidateAccess()
	if err == nil {
		t.Error("Expected bridge access validation to fail after disconnection")
	}

	// Step 9: Test bridge removal
	err = resolver.RemoveBridge("partner-network")
	if err != nil {
		t.Fatalf("Failed to remove bridge: %v", err)
	}

	// Bridge should no longer be recognized
	if resolver.IsBridgePath(bridgePath) {
		t.Error("Expected bridge path to not be recognized after removal")
	}

	bridges := resolver.GetRegisteredBridges()
	if len(bridges) != 0 {
		t.Errorf("Expected no bridges after removal, got: %v", bridges)
	}
}

// TestBridgeDataAccess_MultipleBridges tests multiple bridge connections
func TestBridgeDataAccess_MultipleBridges(t *testing.T) {
	localStorage := storage.NewMemoryTree()
	bridgeStorage := storage.NewBridgeStorage(localStorage)
	resolver := interpreter.NewBridgeScopeResolver(bridgeStorage)

	// Register multiple bridges
	bridges := []struct {
		meshName string
		identity string
	}{
		{"mesh-alpha", "bridge-alpha-xyz789"},
		{"mesh-beta", "bridge-beta-def456"},
		{"mesh-gamma", "bridge-gamma-ghi123"},
	}

	for _, bridge := range bridges {
		identity := &identity.Identity{
			ID:       bridge.identity,
			MeshName: bridge.meshName,
		}

		err := resolver.RegisterBridge(bridge.meshName, identity, "connected", "192.168.1.1:5000")
		if err != nil {
			t.Fatalf("Failed to register bridge %s: %v", bridge.meshName, err)
		}
	}

	// Test all bridges are registered
	registeredBridges := resolver.GetRegisteredBridges()
	if len(registeredBridges) != len(bridges) {
		t.Errorf("Expected %d bridges, got %d", len(bridges), len(registeredBridges))
	}

	// Test path resolution for each bridge
	for _, bridge := range bridges {
		bridgePath := "my." + bridge.meshName + ".data.test"

		scope, err := resolver.ResolveBridgePath(bridgePath)
		if err != nil {
			t.Errorf("Failed to resolve bridge path for %s: %v", bridge.meshName, err)
			continue
		}

		expectedAgentHome := "world.agent." + bridge.identity
		if scope.AgentHome != expectedAgentHome {
			t.Errorf("Expected agent home %s, got %s", expectedAgentHome, scope.AgentHome)
		}

		// Test that bridge paths are correctly identified
		if !resolver.IsBridgePath(bridgePath) {
			t.Errorf("Expected %s to be recognized as bridge path", bridgePath)
		}
	}

	// Test non-existent bridge
	nonExistentPath := "my.mesh-delta.data.test"
	if resolver.IsBridgePath(nonExistentPath) {
		t.Error("Expected non-existent bridge path to not be recognized")
	}

	scope, err := resolver.ResolveBridgePath(nonExistentPath)
	if err == nil {
		t.Error("Expected error for non-existent bridge")
	}
	if scope != nil {
		t.Error("Expected nil scope for non-existent bridge")
	}
}

// TestBridgeDataAccess_PathValidation tests comprehensive path validation
func TestBridgeDataAccess_PathValidation(t *testing.T) {
	validator := types.NewPathValidator()

	testCases := []struct {
		path          string
		expectedType  types.PathType
		expectedMesh  string
		expectError   bool
	}{
		{"my.valid-mesh.data.user", types.BridgePath, "valid-mesh", false},
		{"my.test_network.files", types.BridgePath, "test_network", false},
		{"world.agent.abc123", types.WorldPath, "", false},
		{"users.alice.profile", types.LocalPath, "", false},
		{"my", types.InvalidPath, "", true},
		{"my.invalid@mesh.data", types.InvalidPath, "", true},
		{"my.standalone.data", types.InvalidPath, "", true}, // reserved name
		{"", types.InvalidPath, "", true},
	}

	for _, test := range testCases {
		t.Run(test.path, func(t *testing.T) {
			pathComponents := types.ParsePathString(test.path)
			parsed := validator.ParsePath(pathComponents)

			if test.expectError {
				if parsed.Error == nil {
					t.Errorf("Expected error for path %q, but got none", test.path)
				}
				return
			}

			if parsed.Error != nil {
				t.Errorf("Expected no error for path %q, but got: %v", test.path, parsed.Error)
				return
			}

			if parsed.Type != test.expectedType {
				t.Errorf("Expected type %v for path %q, got %v", test.expectedType, test.path, parsed.Type)
			}

			if test.expectedType == types.BridgePath && parsed.MeshName != test.expectedMesh {
				t.Errorf("Expected mesh name %q for path %q, got %q", test.expectedMesh, test.path, parsed.MeshName)
			}
		})
	}
}

// TestBridgeDataAccess_CrossMeshPatterns demonstrates cross-mesh data access patterns
func TestBridgeDataAccess_CrossMeshPatterns(t *testing.T) {
	localStorage := storage.NewMemoryTree()
	bridgeStorage := storage.NewBridgeStorage(localStorage)
	resolver := interpreter.NewBridgeScopeResolver(bridgeStorage)

	// Register bridge to partner network
	partnerIdentity := &identity.Identity{
		ID:       "partner-bridge-xyz789",
		MeshName: "partner-network",
	}

	err := resolver.RegisterBridge("partner-network", partnerIdentity, "connected", "10.0.1.100:5000")
	if err != nil {
		t.Fatalf("Failed to register partner bridge: %v", err)
	}

	// Test various cross-mesh access patterns
	testCases := []struct {
		description  string
		bridgePath   string
		expectedPath []string
	}{
		{
			description:  "User profile access",
			bridgePath:   "my.partner-network.users.alice.profile",
			expectedPath: []string{"world", "agent", "partner-bridge-xyz789", "users", "alice", "profile"},
		},
		{
			description:  "Shared resource access",
			bridgePath:   "my.partner-network.shared.documents.report",
			expectedPath: []string{"world", "agent", "partner-bridge-xyz789", "shared", "documents", "report"},
		},
		{
			description:  "Configuration access",
			bridgePath:   "my.partner-network.config.settings",
			expectedPath: []string{"world", "agent", "partner-bridge-xyz789", "config", "settings"},
		},
		{
			description:  "Deep nested access",
			bridgePath:   "my.partner-network.org.teams.dev.projects.alpha.status",
			expectedPath: []string{"world", "agent", "partner-bridge-xyz789", "org", "teams", "dev", "projects", "alpha", "status"},
		},
	}

	for _, test := range testCases {
		t.Run(test.description, func(t *testing.T) {
			// Resolve bridge scope
			scope, err := resolver.ResolveBridgePath(test.bridgePath)
			if err != nil {
				t.Fatalf("Failed to resolve bridge path '%s': %v", test.bridgePath, err)
			}

			// Get target path
			targetPath, err := scope.GetTargetPath()
			if err != nil {
				t.Fatalf("Failed to get target path for '%s': %v", test.bridgePath, err)
			}

			// Verify path conversion
			if !equalStringSlices(targetPath, test.expectedPath) {
				t.Errorf("Expected target path %v for '%s', got %v",
					test.expectedPath, test.bridgePath, targetPath)
			}

			// Test data round-trip
			testData := storage.Value{Data: []byte("test data for " + test.description)}
			pathComponents := types.ParsePathString(test.bridgePath)

			err = bridgeStorage.WriteBridgeData(pathComponents, testData, 1)
			if err != nil {
				t.Errorf("Failed to write data for '%s': %v", test.description, err)
			}

			readData, err := bridgeStorage.ReadBridgeData(pathComponents)
			if err != nil {
				t.Errorf("Failed to read data for '%s': %v", test.description, err)
			}

			if string(readData.Data) != string(testData.Data) {
				t.Errorf("Data mismatch for '%s': expected %q, got %q",
					test.description, testData.Data, readData.Data)
			}
		})
	}
}

// Helper function to compare string slices
func equalStringSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}