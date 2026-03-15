package storage

import (
	"fmt"
	"testing"
	"time"

	"github.com/solifugus/amorphdb/internal/identity"
	"github.com/solifugus/amorphdb/internal/types"
)

// MockTree implements the Tree interface for testing
type MockTree struct {
	data map[string]Value
}

func NewMockTree() *MockTree {
	return &MockTree{
		data: make(map[string]Value),
	}
}

func (mt *MockTree) Read(path []string) (Value, error) {
	key := joinPath(path)
	value, exists := mt.data[key]
	if !exists {
		return Value{}, fmt.Errorf("path not found: %v", path)
	}
	return value, nil
}

func (mt *MockTree) Write(path []string, value Value, author uint64) error {
	key := joinPath(path)
	mt.data[key] = value
	return nil
}

func (mt *MockTree) ReadAt(path []string, timestamp int64) (Value, error) {
	// Simplified implementation for testing
	return mt.Read(path)
}

func (mt *MockTree) Children(path []string) ([]Attribute, error) {
	// Simplified implementation for testing
	return []Attribute{}, nil
}

func (mt *MockTree) Purge(path []string, from int64, to int64, author uint64) error {
	// Simplified implementation for testing
	key := joinPath(path)
	delete(mt.data, key)
	return nil
}

// joinPath is already defined in hash.go, so we'll use that function

func createTestIdentity(id, meshName, nodeID string, agentType types.AgentType) *identity.Identity {
	return &identity.Identity{
		ID:       id,
		Type:     agentType,
		MeshName: meshName,
		Created:  time.Now(),
		NodeID:   nodeID,
	}
}

func TestNewAgentStorage(t *testing.T) {
	tree := NewMockTree()
	im := identity.NewIdentityManager("test-mesh", "node-123")

	as := NewAgentStorage(tree, im)

	if as == nil {
		t.Fatal("NewAgentStorage returned nil")
	}

	if as.tree != tree {
		t.Error("AgentStorage should store the provided tree")
	}

	if as.im != im {
		t.Error("AgentStorage should store the provided identity manager")
	}
}

func TestAgentStorage_ReadAgentData(t *testing.T) {
	tree := NewMockTree()
	im := identity.NewIdentityManager("test-mesh", "node-123")
	as := NewAgentStorage(tree, im)

	// Create test identity
	testIdentity := createTestIdentity("agent-123", "test-mesh", "node-123", types.MobileAgent)

	// Write test data directly to tree
	testValue := Value{Data: []byte("test data"), TypeTag: TypeText}
	tree.Write([]string{"world", "agent", "agent-123", "test", "path"}, testValue, 1)

	// Read data through agent storage
	value, err := as.ReadAgentData(testIdentity, []string{"test", "path"})
	if err != nil {
		t.Fatalf("Failed to read agent data: %v", err)
	}

	if string(value.Data) != "test data" {
		t.Errorf("Expected 'test data', got %q", string(value.Data))
	}
}

func TestAgentStorage_ReadAgentData_InvalidIdentity(t *testing.T) {
	tree := NewMockTree()
	im := identity.NewIdentityManager("test-mesh", "node-123")
	as := NewAgentStorage(tree, im)

	// Test with nil identity
	_, err := as.ReadAgentData(nil, []string{"test", "path"})
	if err == nil {
		t.Error("Expected error with nil identity")
	}

	// Test with invalid identity
	invalidIdentity := &identity.Identity{} // Empty identity
	_, err = as.ReadAgentData(invalidIdentity, []string{"test", "path"})
	if err == nil {
		t.Error("Expected error with invalid identity")
	}
}

func TestAgentStorage_WriteAgentData(t *testing.T) {
	tree := NewMockTree()
	im := identity.NewIdentityManager("test-mesh", "node-123")
	as := NewAgentStorage(tree, im)

	// Create test identity
	testIdentity := createTestIdentity("agent-123", "test-mesh", "node-123", types.MobileAgent)

	// Write data through agent storage
	testValue := Value{Data: []byte("test data"), TypeTag: TypeText}
	err := as.WriteAgentData(testIdentity, []string{"test", "path"}, testValue)
	if err != nil {
		t.Fatalf("Failed to write agent data: %v", err)
	}

	// Verify data was written to correct path
	value, err := tree.Read([]string{"world", "agent", "agent-123", "test", "path"})
	if err != nil {
		t.Fatalf("Failed to read written data: %v", err)
	}

	if string(value.Data) != "test data" {
		t.Errorf("Expected 'test data', got %q", string(value.Data))
	}
}

func TestAgentStorage_WriteAgentData_NodeAgent(t *testing.T) {
	tree := NewMockTree()
	im := identity.NewIdentityManager("test-mesh", "node-123")
	as := NewAgentStorage(tree, im)

	// Create node agent identity
	nodeIdentity := createTestIdentity("node-456", "test-mesh", "node-123", types.NodeAgent)

	// Write data through agent storage
	testValue := Value{Data: []byte("node data"), TypeTag: TypeText}
	err := as.WriteAgentData(nodeIdentity, []string{"local", "path"}, testValue)
	if err != nil {
		t.Fatalf("Failed to write node agent data: %v", err)
	}

	// Verify data was written to node path
	value, err := tree.Read([]string{"world", "node", "node-123", "local", "path"})
	if err != nil {
		t.Fatalf("Failed to read written node data: %v", err)
	}

	if string(value.Data) != "node data" {
		t.Errorf("Expected 'node data', got %q", string(value.Data))
	}
}

func TestAgentStorage_ResolveAgentPath(t *testing.T) {
	tree := NewMockTree()
	im := identity.NewIdentityManager("primary-mesh", "node-123")
	as := NewAgentStorage(tree, im)

	// Create test identity
	testIdentity := createTestIdentity("agent-123", "primary-mesh", "node-123", types.MobileAgent)

	// Create bridge identity
	bridgeIdentity, err := im.CreateBridgeIdentity("partner-mesh")
	if err != nil {
		t.Fatalf("Failed to create bridge identity: %v", err)
	}

	tests := []struct {
		name             string
		path             []string
		expectedPath     []string
		expectedIdentity *identity.Identity
		wantErr          bool
	}{
		{
			name:             "regular path",
			path:             []string{"data", "file"},
			expectedPath:     []string{"world", "agent", "agent-123", "data", "file"},
			expectedIdentity: testIdentity,
			wantErr:          false,
		},
		{
			name:             "bridge path",
			path:             []string{"my", "partner-mesh", "data"},
			expectedPath:     []string{"world", "agent", bridgeIdentity.ID, "data"},
			expectedIdentity: bridgeIdentity,
			wantErr:          false,
		},
		{
			name:             "standalone path",
			path:             []string{"my", "standalone", "data"},
			expectedPath:     []string{"world", "node", "standalone-node-123", "data"},
			expectedIdentity: nil, // Will be a generated standalone identity
			wantErr:          false,
		},
		{
			name:    "unknown bridge mesh",
			path:    []string{"my", "unknown-mesh", "data"},
			wantErr: true,
		},
		{
			name:    "empty path",
			path:    []string{},
			wantErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			resolvedPath, resolvedIdentity, err := as.ResolveAgentPath(testIdentity, test.path)

			if test.wantErr {
				if err == nil {
					t.Error("Expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			// Check path
			if len(resolvedPath) != len(test.expectedPath) {
				t.Errorf("Expected path length %d, got %d", len(test.expectedPath), len(resolvedPath))
				return
			}

			for i, component := range test.expectedPath {
				if resolvedPath[i] != component {
					t.Errorf("Path component %d: expected %q, got %q", i, component, resolvedPath[i])
				}
			}

			// Check identity (if specified)
			if test.expectedIdentity != nil {
				if resolvedIdentity == nil {
					t.Error("Expected identity, got nil")
				} else if resolvedIdentity.ID != test.expectedIdentity.ID {
					t.Errorf("Expected identity ID %q, got %q", test.expectedIdentity.ID, resolvedIdentity.ID)
				}
			}
		})
	}
}

func TestAgentStorage_GetAgentStorageInfo(t *testing.T) {
	tree := NewMockTree()
	im := identity.NewIdentityManager("test-mesh", "node-123")
	as := NewAgentStorage(tree, im)

	tests := []struct {
		name        string
		identity    *identity.Identity
		expectedType string
		expectedReplicate bool
	}{
		{
			name:              "node agent",
			identity:          createTestIdentity("node-456", "test-mesh", "node-123", types.NodeAgent),
			expectedType:      "local",
			expectedReplicate: false,
		},
		{
			name:              "mobile agent",
			identity:          createTestIdentity("mobile-789", "test-mesh", "node-123", types.MobileAgent),
			expectedType:      "replicated",
			expectedReplicate: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			info, err := as.GetAgentStorageInfo(test.identity)
			if err != nil {
				t.Fatalf("Failed to get storage info: %v", err)
			}

			if info.StorageType != test.expectedType {
				t.Errorf("Expected storage type %q, got %q", test.expectedType, info.StorageType)
			}

			if info.CanReplicate != test.expectedReplicate {
				t.Errorf("Expected CanReplicate %v, got %v", test.expectedReplicate, info.CanReplicate)
			}

			if info.MeshName != test.identity.MeshName {
				t.Errorf("Expected mesh name %q, got %q", test.identity.MeshName, info.MeshName)
			}

			expectedHomePath := im.GetAgentHomePath(test.identity)
			if info.HomePath != expectedHomePath {
				t.Errorf("Expected home path %q, got %q", expectedHomePath, info.HomePath)
			}
		})
	}
}

func TestAgentStorage_GetAgentStorageInfo_InvalidIdentity(t *testing.T) {
	tree := NewMockTree()
	im := identity.NewIdentityManager("test-mesh", "node-123")
	as := NewAgentStorage(tree, im)

	_, err := as.GetAgentStorageInfo(nil)
	if err == nil {
		t.Error("Expected error with nil identity")
	}

	invalidIdentity := &identity.Identity{} // Empty identity
	_, err = as.GetAgentStorageInfo(invalidIdentity)
	if err == nil {
		t.Error("Expected error with invalid identity")
	}
}

func TestAgentStorage_PurgeAgentData(t *testing.T) {
	tree := NewMockTree()
	im := identity.NewIdentityManager("test-mesh", "node-123")
	as := NewAgentStorage(tree, im)

	// Create test identity
	testIdentity := createTestIdentity("agent-123", "test-mesh", "node-123", types.MobileAgent)

	// Write test data
	testValue := Value{Data: []byte("test data"), TypeTag: TypeText}
	tree.Write([]string{"world", "agent", "agent-123", "test", "path"}, testValue, 1)

	// Verify data exists
	_, err := tree.Read([]string{"world", "agent", "agent-123", "test", "path"})
	if err != nil {
		t.Fatalf("Test data should exist: %v", err)
	}

	// Purge data
	err = as.PurgeAgentData(testIdentity, []string{"test", "path"}, 0, time.Now().Unix())
	if err != nil {
		t.Fatalf("Failed to purge agent data: %v", err)
	}

	// Verify data was purged (in our mock, purge deletes the data)
	_, err = tree.Read([]string{"world", "agent", "agent-123", "test", "path"})
	if err == nil {
		t.Error("Data should have been purged")
	}
}

func TestAgentStorage_parseStoragePath(t *testing.T) {
	tree := NewMockTree()
	im := identity.NewIdentityManager("test-mesh", "node-123")
	as := NewAgentStorage(tree, im)

	tests := []struct {
		path     string
		expected []string
	}{
		{"", []string{}},
		{"single", []string{"single"}},
		{"world.agent.123", []string{"world", "agent", "123"}},
		{"world.node.456.data", []string{"world", "node", "456", "data"}},
	}

	for _, test := range tests {
		result := as.parseStoragePath(test.path)

		if len(result) != len(test.expected) {
			t.Errorf("Path %q: expected length %d, got %d", test.path, len(test.expected), len(result))
			continue
		}

		for i, component := range test.expected {
			if result[i] != component {
				t.Errorf("Path %q component %d: expected %q, got %q", test.path, i, component, result[i])
			}
		}
	}
}

func TestAgentStorage_hashIdentityID(t *testing.T) {
	tree := NewMockTree()
	im := identity.NewIdentityManager("test-mesh", "node-123")
	as := NewAgentStorage(tree, im)

	// Test that same ID produces same hash
	id := "test-identity-123"
	hash1 := as.hashIdentityID(id)
	hash2 := as.hashIdentityID(id)

	if hash1 != hash2 {
		t.Error("Same identity ID should produce same hash")
	}

	// Test that different IDs produce different hashes
	hash3 := as.hashIdentityID("different-identity-456")
	if hash1 == hash3 {
		t.Error("Different identity IDs should produce different hashes")
	}

	// Test that hash is non-zero
	if hash1 == 0 {
		t.Error("Hash should not be zero")
	}
}