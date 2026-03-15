package service

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/solifugus/amorphdb/internal/config"
	"github.com/solifugus/amorphdb/internal/mesh"
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

func createTestMeshManager() (*mesh.MeshManager, error) {
	cfg := &config.Config{
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

	mockTree := NewMockTree()
	return mesh.NewMeshManager(cfg, mockTree, "/tmp/test-config.yaml")
}

func TestNewMeshService(t *testing.T) {
	meshManager, err := createTestMeshManager()
	if err != nil {
		t.Fatalf("Failed to create mesh manager: %v", err)
	}

	meshService := NewMeshService(meshManager)

	if meshService == nil {
		t.Fatal("MeshService should not be nil")
	}

	if meshService.meshManager != meshManager {
		t.Error("MeshService should store the provided mesh manager")
	}
}

func TestMeshService_CreateMesh_ValidRequest(t *testing.T) {
	meshManager, err := createTestMeshManager()
	if err != nil {
		t.Fatalf("Failed to create mesh manager: %v", err)
	}

	meshService := NewMeshService(meshManager)

	request := CreateMeshRequest{
		Name: "test-mesh",
	}

	requestData, err := json.Marshal(request)
	if err != nil {
		t.Fatalf("Failed to marshal request: %v", err)
	}

	responseData, err := meshService.CreateMesh(requestData)
	if err != nil {
		t.Fatalf("CreateMesh failed: %v", err)
	}

	var response CreateMeshResponse
	err = json.Unmarshal(responseData, &response)
	if err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	// Verify response
	if !response.Success {
		t.Errorf("Expected success=true, got false. Message: %s", response.Message)
	}

	if response.MeshName != "test-mesh" {
		t.Errorf("Expected mesh name 'test-mesh', got %q", response.MeshName)
	}

	if !response.IsFounder {
		t.Error("Expected IsFounder=true for mesh creator")
	}

	if response.NodeID == "" {
		t.Error("Expected NodeID to be set")
	}
}

func TestMeshService_CreateMesh_InvalidRequest(t *testing.T) {
	meshManager, err := createTestMeshManager()
	if err != nil {
		t.Fatalf("Failed to create mesh manager: %v", err)
	}

	meshService := NewMeshService(meshManager)

	testCases := []struct {
		name        string
		requestData []byte
		description string
	}{
		{
			name:        "invalid JSON",
			requestData: []byte("{invalid json}"),
			description: "malformed JSON should fail",
		},
		{
			name:        "empty mesh name",
			requestData: []byte(`{"name": ""}`),
			description: "empty mesh name should fail",
		},
		{
			name:        "invalid mesh name",
			requestData: []byte(`{"name": "invalid@mesh"}`),
			description: "invalid characters should fail",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			responseData, err := meshService.CreateMesh(tc.requestData)
			if err != nil {
				t.Fatalf("CreateMesh should not return error for invalid input: %v", err)
			}

			var response CreateMeshResponse
			err = json.Unmarshal(responseData, &response)
			if err != nil {
				t.Fatalf("Failed to unmarshal response: %v", err)
			}

			if response.Success {
				t.Errorf("%s: Expected success=false, got true", tc.description)
			}

			if response.Message == "" {
				t.Errorf("%s: Expected error message to be set", tc.description)
			}
		})
	}
}

func TestMeshService_CreateMesh_AlreadyInMesh(t *testing.T) {
	meshManager, err := createTestMeshManager()
	if err != nil {
		t.Fatalf("Failed to create mesh manager: %v", err)
	}

	meshService := NewMeshService(meshManager)

	// Create first mesh
	request1 := CreateMeshRequest{Name: "first-mesh"}
	requestData1, _ := json.Marshal(request1)

	_, err = meshService.CreateMesh(requestData1)
	if err != nil {
		t.Fatalf("First mesh creation failed: %v", err)
	}

	// Try to create second mesh
	request2 := CreateMeshRequest{Name: "second-mesh"}
	requestData2, _ := json.Marshal(request2)

	responseData, err := meshService.CreateMesh(requestData2)
	if err != nil {
		t.Fatalf("CreateMesh should not return error: %v", err)
	}

	var response CreateMeshResponse
	err = json.Unmarshal(responseData, &response)
	if err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	// Should fail because already in mesh
	if response.Success {
		t.Error("Expected failure when already in mesh")
	}
}

func TestMeshService_GetMeshStatus_Standalone(t *testing.T) {
	meshManager, err := createTestMeshManager()
	if err != nil {
		t.Fatalf("Failed to create mesh manager: %v", err)
	}

	meshService := NewMeshService(meshManager)

	responseData, err := meshService.GetMeshStatus([]byte("{}"))
	if err != nil {
		t.Fatalf("GetMeshStatus failed: %v", err)
	}

	var response MeshStatusResponse
	err = json.Unmarshal(responseData, &response)
	if err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	// Check standalone status
	if response.MeshName != "" {
		t.Errorf("Expected empty mesh name for standalone, got %q", response.MeshName)
	}

	if response.Status != "standalone" {
		t.Errorf("Expected status 'standalone', got %q", response.Status)
	}

	if response.IsFounder {
		t.Error("Expected IsFounder=false for standalone node")
	}

	if response.MemberCount != 1 {
		t.Errorf("Expected member count 1, got %d", response.MemberCount)
	}
}

func TestMeshService_GetMeshStatus_WithMesh(t *testing.T) {
	meshManager, err := createTestMeshManager()
	if err != nil {
		t.Fatalf("Failed to create mesh manager: %v", err)
	}

	meshService := NewMeshService(meshManager)

	// Create mesh first
	request := CreateMeshRequest{Name: "test-mesh"}
	requestData, _ := json.Marshal(request)
	_, err = meshService.CreateMesh(requestData)
	if err != nil {
		t.Fatalf("Failed to create mesh: %v", err)
	}

	// Get status
	responseData, err := meshService.GetMeshStatus([]byte("{}"))
	if err != nil {
		t.Fatalf("GetMeshStatus failed: %v", err)
	}

	var response MeshStatusResponse
	err = json.Unmarshal(responseData, &response)
	if err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	// Check mesh status
	if response.MeshName != "test-mesh" {
		t.Errorf("Expected mesh name 'test-mesh', got %q", response.MeshName)
	}

	if response.Status != "founder" {
		t.Errorf("Expected status 'founder', got %q", response.Status)
	}

	if !response.IsFounder {
		t.Error("Expected IsFounder=true for mesh founder")
	}

	if response.NodeIdentity == "" {
		t.Error("Expected node identity to be set")
	}

	if response.FoundedAt == nil {
		t.Error("Expected FoundedAt to be set for founder")
	}
}

func TestMeshService_IsStandalone(t *testing.T) {
	meshManager, err := createTestMeshManager()
	if err != nil {
		t.Fatalf("Failed to create mesh manager: %v", err)
	}

	meshService := NewMeshService(meshManager)

	// Should be standalone initially
	if !meshService.IsStandalone() {
		t.Error("Expected IsStandalone=true initially")
	}

	// Create mesh
	request := CreateMeshRequest{Name: "test-mesh"}
	requestData, _ := json.Marshal(request)
	_, err = meshService.CreateMesh(requestData)
	if err != nil {
		t.Fatalf("Failed to create mesh: %v", err)
	}

	// Should not be standalone after creating mesh
	if meshService.IsStandalone() {
		t.Error("Expected IsStandalone=false after creating mesh")
	}
}

func TestMeshService_GetMeshName(t *testing.T) {
	meshManager, err := createTestMeshManager()
	if err != nil {
		t.Fatalf("Failed to create mesh manager: %v", err)
	}

	meshService := NewMeshService(meshManager)

	// Should be empty initially
	if meshService.GetMeshName() != "" {
		t.Errorf("Expected empty mesh name initially, got %q", meshService.GetMeshName())
	}

	// Create mesh
	request := CreateMeshRequest{Name: "test-mesh"}
	requestData, _ := json.Marshal(request)
	_, err = meshService.CreateMesh(requestData)
	if err != nil {
		t.Fatalf("Failed to create mesh: %v", err)
	}

	// Should return mesh name after creation
	if meshService.GetMeshName() != "test-mesh" {
		t.Errorf("Expected mesh name 'test-mesh', got %q", meshService.GetMeshName())
	}
}

func TestMeshService_GetNodeIdentity(t *testing.T) {
	meshManager, err := createTestMeshManager()
	if err != nil {
		t.Fatalf("Failed to create mesh manager: %v", err)
	}

	meshService := NewMeshService(meshManager)

	// Should have identity (generated during initialization)
	initialIdentity := meshService.GetNodeIdentity()
	if initialIdentity == "" {
		t.Error("Expected node identity to be generated during initialization")
	}

	// Create mesh
	request := CreateMeshRequest{Name: "test-mesh"}
	requestData, _ := json.Marshal(request)
	_, err = meshService.CreateMesh(requestData)
	if err != nil {
		t.Fatalf("Failed to create mesh: %v", err)
	}

	// Should have identity after mesh creation (may be new or same)
	identity := meshService.GetNodeIdentity()
	if identity == "" {
		t.Error("Expected node identity to be set after mesh creation")
	}
}