package mesh

import (
	"fmt"
	"testing"
	"time"

	"github.com/solifugus/amorphdb/internal/config"
	"github.com/solifugus/amorphdb/internal/storage"
)

// MockTree implements the Tree interface for testing
type MockTree struct {
	data map[string]storage.Value
}

func NewMockTree() *MockTree {
	return &MockTree{
		data: make(map[string]storage.Value),
	}
}

func (mt *MockTree) Read(path []string) (storage.Value, error) {
	key := joinPath(path)
	value, exists := mt.data[key]
	if !exists {
		return storage.Value{}, fmt.Errorf("path not found: %v", path)
	}
	return value, nil
}

func (mt *MockTree) Write(path []string, value storage.Value, author uint64) error {
	key := joinPath(path)
	mt.data[key] = value
	return nil
}

func (mt *MockTree) ReadAt(path []string, timestamp int64) (storage.Value, error) {
	return mt.Read(path)
}

func (mt *MockTree) Children(path []string) ([]storage.Attribute, error) {
	return []storage.Attribute{}, nil
}

func (mt *MockTree) Purge(path []string, from int64, to int64, author uint64) error {
	key := joinPath(path)
	delete(mt.data, key)
	return nil
}

func joinPath(path []string) string {
	if len(path) == 0 {
		return ""
	}
	result := path[0]
	for i := 1; i < len(path); i++ {
		result += "." + path[i]
	}
	return result
}

func createTestConfig() *config.Config {
	return &config.Config{
		Mesh: config.MeshConfig{
			Name:     "",
			Bridges:  make(map[string]string),
			Identity: "test-node-123",
		},
		Data: config.DataConfig{
			StorageDir: "/tmp/test",
		},
		Network: config.NetworkConfig{
			LocalSocketPath: "/tmp/test.sock",
			Port:            5000,
		},
	}
}

func TestNewMeshManager(t *testing.T) {
	cfg := createTestConfig()
	mockTree := NewMockTree()
	configPath := "/tmp/test-config.yaml"

	mm, err := NewMeshManager(cfg, mockTree, configPath)
	if err != nil {
		t.Fatalf("Failed to create mesh manager: %v", err)
	}

	if mm == nil {
		t.Fatal("Mesh manager should not be nil")
	}

	// Check initial state
	state := mm.GetMeshState()
	if state.Status != StatusStandalone {
		t.Errorf("Expected status %s, got %s", StatusStandalone, state.Status)
	}

	if state.Name != "" {
		t.Errorf("Expected empty mesh name for standalone, got %q", state.Name)
	}

	if mm.IsFounder() { // Should be false initially
		t.Error("Node should not be founder initially")
	}
}

func TestNewMeshManager_NilConfig(t *testing.T) {
	mockTree := NewMockTree()
	_, err := NewMeshManager(nil, mockTree, "/tmp/test.yaml")
	if err == nil {
		t.Error("Expected error when config is nil")
	}
}

func TestCreateMesh_ValidName(t *testing.T) {
	cfg := createTestConfig()
	mockTree := NewMockTree()
	configPath := "/tmp/test-config.yaml"

	mm, err := NewMeshManager(cfg, mockTree, configPath)
	if err != nil {
		t.Fatalf("Failed to create mesh manager: %v", err)
	}

	meshName := "test-mesh"
	err = mm.CreateMesh(meshName)
	if err != nil {
		t.Fatalf("Failed to create mesh: %v", err)
	}

	// Check state after creation
	state := mm.GetMeshState()
	if state.Status != StatusFounder {
		t.Errorf("Expected status %s, got %s", StatusFounder, state.Status)
	}

	if state.Name != meshName {
		t.Errorf("Expected mesh name %q, got %q", meshName, state.Name)
	}

	if !mm.IsFounder() {
		t.Error("Node should be founder after creating mesh")
	}

	if state.MemberCount != 1 {
		t.Errorf("Expected member count 1, got %d", state.MemberCount)
	}

	if state.ZoneCount != 1 {
		t.Errorf("Expected zone count 1, got %d", state.ZoneCount)
	}

	// Check that node identity was generated
	identity := mm.GetNodeIdentity()
	if identity == nil {
		t.Fatal("Node identity should be generated")
	}

	if identity.MeshName != meshName {
		t.Errorf("Expected identity mesh name %q, got %q", meshName, identity.MeshName)
	}
}

func TestCreateMesh_InvalidName(t *testing.T) {
	cfg := createTestConfig()
	mockTree := NewMockTree()
	configPath := "/tmp/test-config.yaml"

	mm, err := NewMeshManager(cfg, mockTree, configPath)
	if err != nil {
		t.Fatalf("Failed to create mesh manager: %v", err)
	}

	testCases := []string{
		"",                    // Empty name
		"a",                   // Too short
		"invalid mesh name",   // Contains space
		"invalid@mesh",        // Contains special character
		string(make([]byte, 100)), // Too long
	}

	for _, invalidName := range testCases {
		t.Run(fmt.Sprintf("name_%q", invalidName), func(t *testing.T) {
			err := mm.CreateMesh(invalidName)
			if err == nil {
				t.Errorf("Expected error for invalid mesh name %q", invalidName)
			}

			// Ensure state didn't change
			state := mm.GetMeshState()
			if state.Status != StatusStandalone {
				t.Error("State should remain standalone after failed creation")
			}
		})
	}
}

func TestCreateMesh_AlreadyInMesh(t *testing.T) {
	cfg := createTestConfig()
	mockTree := NewMockTree()
	configPath := "/tmp/test-config.yaml"

	mm, err := NewMeshManager(cfg, mockTree, configPath)
	if err != nil {
		t.Fatalf("Failed to create mesh manager: %v", err)
	}

	// Create first mesh
	err = mm.CreateMesh("first-mesh")
	if err != nil {
		t.Fatalf("Failed to create first mesh: %v", err)
	}

	// Try to create second mesh
	err = mm.CreateMesh("second-mesh")
	if err == nil {
		t.Error("Expected error when trying to create mesh while already in one")
	}

	// Ensure original state is preserved
	state := mm.GetMeshState()
	if state.Name != "first-mesh" {
		t.Errorf("Expected mesh name to remain 'first-mesh', got %q", state.Name)
	}
}

func TestGetMeshState(t *testing.T) {
	cfg := createTestConfig()
	mockTree := NewMockTree()
	configPath := "/tmp/test-config.yaml"

	mm, err := NewMeshManager(cfg, mockTree, configPath)
	if err != nil {
		t.Fatalf("Failed to create mesh manager: %v", err)
	}

	// Test getting state returns a copy
	state1 := mm.GetMeshState()
	state2 := mm.GetMeshState()

	// Modify one state
	state1.Name = "modified"

	// Ensure the other state is not affected
	if state2.Name == "modified" {
		t.Error("GetMeshState should return a copy, not reference")
	}
}

func TestGetBridgeConnections(t *testing.T) {
	cfg := createTestConfig()
	mockTree := NewMockTree()
	configPath := "/tmp/test-config.yaml"

	mm, err := NewMeshManager(cfg, mockTree, configPath)
	if err != nil {
		t.Fatalf("Failed to create mesh manager: %v", err)
	}

	// Initially should have no bridges
	bridges := mm.GetBridgeConnections()
	if len(bridges) != 0 {
		t.Errorf("Expected 0 bridges initially, got %d", len(bridges))
	}
}

func TestInitializeMeshFounder(t *testing.T) {
	cfg := createTestConfig()
	mockTree := NewMockTree()
	configPath := "/tmp/test-config.yaml"

	mm, err := NewMeshManager(cfg, mockTree, configPath)
	if err != nil {
		t.Fatalf("Failed to create mesh manager: %v", err)
	}

	// Create mesh to trigger founder initialization
	err = mm.CreateMesh("founder-test")
	if err != nil {
		t.Fatalf("Failed to create mesh: %v", err)
	}

	// Check that founder data was written to storage
	// Check mesh info
	meshInfoKeys := []string{"name", "founded_at", "founder_id", "founder_node"}
	for _, key := range meshInfoKeys {
		path := []string{"world", "mesh", "info", key}
		_, err := mockTree.Read(path)
		if err != nil {
			t.Errorf("Expected mesh info %s to be stored, but got error: %v", key, err)
		}
	}

	// Check zone ownership
	zonePath := []string{"world", "mesh", "zones", "0", "owner"}
	_, err = mockTree.Read(zonePath)
	if err != nil {
		t.Errorf("Expected zone ownership to be stored, but got error: %v", err)
	}
}

func TestValidateMeshName(t *testing.T) {
	validNames := []string{
		"valid-mesh",
		"valid_mesh",
		"ValidMesh123",
		"mesh1",
		"my-test-mesh",
	}

	for _, name := range validNames {
		t.Run(fmt.Sprintf("valid_%s", name), func(t *testing.T) {
			err := ValidateMeshName(name)
			if err != nil {
				t.Errorf("Expected %q to be valid, got error: %v", name, err)
			}
		})
	}

	invalidNames := []string{
		"",                      // Empty
		"a",                     // Too short
		"invalid mesh",          // Space
		"invalid@mesh",          // Special character
		"invalid.mesh",          // Dot
		string(make([]byte, 100)), // Too long
	}

	for _, name := range invalidNames {
		t.Run(fmt.Sprintf("invalid_%s", name), func(t *testing.T) {
			err := ValidateMeshName(name)
			if err == nil {
				t.Errorf("Expected %q to be invalid", name)
			}
		})
	}
}

func TestMeshManager_ThreadSafety(t *testing.T) {
	cfg := createTestConfig()
	mockTree := NewMockTree()
	configPath := "/tmp/test-config.yaml"

	mm, err := NewMeshManager(cfg, mockTree, configPath)
	if err != nil {
		t.Fatalf("Failed to create mesh manager: %v", err)
	}

	// Test concurrent access to mesh state
	done := make(chan bool, 2)

	// Goroutine 1: Read state repeatedly
	go func() {
		for i := 0; i < 100; i++ {
			state := mm.GetMeshState()
			_ = state.Name // Use the state
		}
		done <- true
	}()

	// Goroutine 2: Try to create mesh (should work only once)
	go func() {
		time.Sleep(time.Millisecond) // Small delay to interleave
		_ = mm.CreateMesh("test-concurrent")
		done <- true
	}()

	// Wait for both goroutines to complete
	<-done
	<-done

	// No assertions needed - if we get here without panic, thread safety worked
}