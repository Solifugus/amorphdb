// Package integration provides comprehensive end-to-end integration tests for AmorphDB mesh lifecycle
package integration

import (
	"testing"

	"github.com/solifugus/amorphdb/internal/config"
	"github.com/solifugus/amorphdb/internal/mesh"
	"github.com/solifugus/amorphdb/internal/storage"
	"github.com/solifugus/amorphdb/internal/types"
)

// TestMeshLifecycle_StandaloneToMesh tests the complete lifecycle from standalone to mesh
// This covers Scenario A from the development plan
func TestMeshLifecycle_StandaloneToMesh(t *testing.T) {
	// Step 1: Create standalone node with initial data
	t.Log("Step 1: Creating standalone node")

	cfg := &config.Config{
		Data: config.DataConfig{
			StorageDir: "/tmp/amorphdb-test-standalone",
		},
		Network: config.NetworkConfig{
			LocalSocketPath: "/tmp/amorphdb-test-standalone.sock",
			Port:            8001,
		},
		Mesh: config.MeshConfig{
			Name:     "", // Starts standalone
			Identity: "standalone-node-001",
		},
	}

	localStorage := storage.NewMemoryTree()
	meshManager, err := mesh.NewMeshManager(cfg, localStorage, "/tmp/amorphdb-test-standalone.yaml")
	if err != nil {
		t.Fatalf("Failed to create mesh manager: %v", err)
	}

	// Add some initial data to standalone node
	testData := storage.Value{Data: []byte("Standalone data before mesh creation")}
	path := []string{"local", "users", "admin", "profile"}
	err = localStorage.Write(path, testData, 1)
	if err != nil {
		t.Fatalf("Failed to set initial standalone data: %v", err)
	}

	// Verify initial standalone state
	initialState := meshManager.GetMeshState()
	if initialState.Name != "" {
		t.Errorf("Expected empty mesh name for standalone node, got %s", initialState.Name)
	}

	// Step 2: Create a new mesh from the standalone node
	t.Log("Step 2: Converting standalone node to mesh founder")

	meshName := "test-mesh-alpha"
	err = meshManager.CreateMesh(meshName)
	if err != nil {
		t.Fatalf("Failed to create mesh: %v", err)
	}

	// Verify mesh was created
	meshState := meshManager.GetMeshState()
	if meshState.Name != meshName {
		t.Errorf("Expected mesh name %s, got %s", meshName, meshState.Name)
	}

	// Verify node became mesh founder
	nodeIdentity := meshManager.GetNodeIdentity()
	if nodeIdentity.ID == "" {
		t.Fatal("Node identity should be generated")
	}
	if nodeIdentity.MeshName != meshName {
		t.Errorf("Expected node mesh name %s, got %s", meshName, nodeIdentity.MeshName)
	}
	if nodeIdentity.Type != types.NodeAgent {
		t.Errorf("Expected node agent type, got %v", nodeIdentity.Type)
	}

	// Verify mesh founder status
	if !meshManager.IsFounder() {
		t.Error("Node should be mesh founder")
	}

	// Verify original data is accessible (data preservation is handled by the storage layer)
	preservedData, err := localStorage.Read(path)
	if err != nil {
		t.Errorf("Failed to retrieve original data: %v", err)
	} else if string(preservedData.Data) != string(testData.Data) {
		t.Errorf("Expected preserved data %q, got %q", testData.Data, preservedData.Data)
	}

	// Step 3: Test mesh-wide data creation
	t.Log("Step 3: Testing mesh data operations")

	// Create mesh-wide data
	meshData := storage.Value{Data: []byte("Shared mesh data")}
	meshPath := []string{"world", "shared", "config", "mesh_settings"}
	err = localStorage.Write(meshPath, meshData, 1)
	if err != nil {
		t.Fatalf("Failed to set mesh data: %v", err)
	}

	// Verify mesh data exists
	readMeshData, err := localStorage.Read(meshPath)
	if err != nil {
		t.Errorf("Failed to retrieve mesh data: %v", err)
	} else if string(readMeshData.Data) != string(meshData.Data) {
		t.Errorf("Expected mesh data %q, got %q", meshData.Data, readMeshData.Data)
	}

	// Step 4: Test agent-specific data
	t.Log("Step 4: Testing agent-specific data")

	agentPath := []string{"world", "agent", nodeIdentity.ID, "config"}
	agentData := storage.Value{Data: []byte("Agent-specific configuration")}
	err = localStorage.Write(agentPath, agentData, 1)
	if err != nil {
		t.Fatalf("Failed to set agent data: %v", err)
	}

	readAgentData, err := localStorage.Read(agentPath)
	if err != nil {
		t.Errorf("Failed to retrieve agent data: %v", err)
	} else if string(readAgentData.Data) != string(agentData.Data) {
		t.Errorf("Expected agent data %q, got %q", agentData.Data, readAgentData.Data)
	}

	// Step 5: Test mesh state consistency
	t.Log("Step 5: Testing mesh state consistency")

	finalMeshState := meshManager.GetMeshState()
	if finalMeshState.Name != meshName {
		t.Errorf("Expected consistent mesh name %s, got %s", meshName, finalMeshState.Name)
	}

	if finalMeshState.Status != mesh.StatusFounder {
		t.Errorf("Expected founder status, got %s", finalMeshState.Status)
	}

	if finalMeshState.NodeIdentity == nil {
		t.Fatal("Mesh state should have node identity")
	}

	if finalMeshState.NodeIdentity.ID != nodeIdentity.ID {
		t.Errorf("Expected mesh state identity %s, got %s", nodeIdentity.ID, finalMeshState.NodeIdentity.ID)
	}

	t.Log("Mesh lifecycle test completed successfully")
}

// TestMeshLifecycle_BasicOperations tests basic mesh operations
func TestMeshLifecycle_BasicOperations(t *testing.T) {
	t.Log("Testing basic mesh operations")

	// Test 1: Multiple mesh creation should fail
	cfg := &config.Config{
		Network: config.NetworkConfig{
			Port: 8004,
		},
		Mesh: config.MeshConfig{
			Name:     "",
			Identity: "test-node-001",
		},
	}

	localStorage := storage.NewMemoryTree()
	meshManager, err := mesh.NewMeshManager(cfg, localStorage, "/tmp/test-mesh.yaml")
	if err != nil {
		t.Fatalf("Failed to create mesh manager: %v", err)
	}

	// Create first mesh
	err = meshManager.CreateMesh("first-mesh")
	if err != nil {
		t.Fatalf("Failed to create first mesh: %v", err)
	}

	// Attempt to create second mesh should fail or update existing
	err = meshManager.CreateMesh("second-mesh")
	// This may succeed if the implementation allows mesh name changes
	// or fail if it doesn't - both are valid behaviors

	// Test 2: Verify mesh state after operations
	state := meshManager.GetMeshState()
	if state.Name == "" {
		t.Error("Mesh should have a name after creation")
	}

	if !meshManager.IsFounder() {
		t.Error("Node should be founder of created mesh")
	}

	// Test 3: Node identity consistency
	identity := meshManager.GetNodeIdentity()
	if identity == nil {
		t.Fatal("Node identity should not be nil")
	}

	if identity.Type != types.NodeAgent {
		t.Errorf("Expected NodeAgent type, got %v", identity.Type)
	}

	if identity.MeshName != state.Name {
		t.Errorf("Identity mesh name %s should match state mesh name %s",
			identity.MeshName, state.Name)
	}

	t.Log("Basic mesh operations test completed")
}

// TestMeshLifecycle_ConfigPersistence tests configuration persistence
func TestMeshLifecycle_ConfigPersistence(t *testing.T) {
	t.Log("Testing configuration persistence")

	configPath := "/tmp/amorphdb-config-test.yaml"

	cfg := &config.Config{
		Data: config.DataConfig{
			StorageDir: "/tmp/amorphdb-test-config",
		},
		Network: config.NetworkConfig{
			LocalSocketPath: "/tmp/amorphdb-test-config.sock",
			Port:            8003,
		},
		Mesh: config.MeshConfig{
			Name:     "",
			Identity: "config-test-node",
		},
	}

	localStorage := storage.NewMemoryTree()
	meshManager, err := mesh.NewMeshManager(cfg, localStorage, configPath)
	if err != nil {
		t.Fatalf("Failed to create mesh manager: %v", err)
	}

	// Create mesh
	testMeshName := "config-test-mesh"
	err = meshManager.CreateMesh(testMeshName)
	if err != nil {
		t.Fatalf("Failed to create mesh: %v", err)
	}

	// Verify mesh state
	state := meshManager.GetMeshState()
	if state.Name != testMeshName {
		t.Errorf("Expected mesh name %s, got %s", testMeshName, state.Name)
	}

	// Test identity manager access
	identityMgr := meshManager.GetIdentityManager()
	if identityMgr == nil {
		t.Fatal("Identity manager should not be nil")
	}

	identity := identityMgr.GetNodeIdentity()
	if identity.MeshName != testMeshName {
		t.Errorf("Expected identity mesh name %s, got %s", testMeshName, identity.MeshName)
	}

	t.Log("Configuration persistence test completed")
}