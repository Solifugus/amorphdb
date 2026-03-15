// Package storage provides bridge storage tests
package storage

import (
	"testing"

	"github.com/solifugus/amorphdb/internal/identity"
)

func TestBridgeStorage_RegisterBridge(t *testing.T) {
	localStorage := NewMemoryTree()
	bridgeStorage := NewBridgeStorage(localStorage)

	// Create test identity
	testIdentity := &identity.Identity{
		ID:       "bridge-test-123",
		MeshName: "partner-mesh",
	}

	// Create test bridge connection
	connection := &BridgeConnection{
		MeshName:      "partner-mesh",
		Identity:      testIdentity,
		Status:        "connected",
		TargetAddress: "192.168.1.10:5000",
	}

	// Test successful registration
	err := bridgeStorage.RegisterBridge("partner-mesh", connection)
	if err != nil {
		t.Fatalf("Failed to register bridge: %v", err)
	}

	// Test bridge is registered
	bridges := bridgeStorage.GetRegisteredBridges()
	if len(bridges) != 1 || bridges[0] != "partner-mesh" {
		t.Errorf("Expected bridge 'partner-mesh' to be registered, got: %v", bridges)
	}

	// Test duplicate registration
	err = bridgeStorage.RegisterBridge("partner-mesh", connection)
	if err != nil {
		t.Errorf("Expected duplicate registration to succeed (update), got error: %v", err)
	}
}

func TestBridgeStorage_RegisterBridge_InvalidInput(t *testing.T) {
	localStorage := NewMemoryTree()
	bridgeStorage := NewBridgeStorage(localStorage)

	tests := []struct {
		name       string
		meshName   string
		connection *BridgeConnection
		expectErr  bool
	}{
		{
			name:       "empty mesh name",
			meshName:   "",
			connection: &BridgeConnection{Identity: &identity.Identity{ID: "test"}},
			expectErr:  true,
		},
		{
			name:       "nil connection",
			meshName:   "test-mesh",
			connection: nil,
			expectErr:  true,
		},
		{
			name:     "nil identity",
			meshName: "test-mesh",
			connection: &BridgeConnection{
				Identity: nil,
			},
			expectErr: true,
		},
		{
			name:     "empty identity ID",
			meshName: "test-mesh",
			connection: &BridgeConnection{
				Identity: &identity.Identity{ID: ""},
			},
			expectErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := bridgeStorage.RegisterBridge(test.meshName, test.connection)
			if test.expectErr && err == nil {
				t.Errorf("Expected error for %s, but got none", test.name)
			}
			if !test.expectErr && err != nil {
				t.Errorf("Expected no error for %s, but got: %v", test.name, err)
			}
		})
	}
}

func TestBridgeStorage_ResolveBridgePath(t *testing.T) {
	localStorage := NewMemoryTree()
	bridgeStorage := NewBridgeStorage(localStorage)

	// Register a test bridge
	testIdentity := &identity.Identity{
		ID:       "bridge-id-123",
		MeshName: "partner-mesh",
	}

	connection := &BridgeConnection{
		MeshName:      "partner-mesh",
		Identity:      testIdentity,
		Status:        "connected",
		TargetAddress: "192.168.1.10:5000",
	}

	err := bridgeStorage.RegisterBridge("partner-mesh", connection)
	if err != nil {
		t.Fatalf("Failed to register bridge: %v", err)
	}

	tests := []struct {
		name           string
		path           []string
		expectedPath   []string
		expectedMesh   string
		expectErr      bool
	}{
		{
			name:         "valid bridge path",
			path:         []string{"my", "partner-mesh", "data", "users"},
			expectedPath: []string{"world", "agent", "bridge-id-123", "data", "users"},
			expectedMesh: "partner-mesh",
			expectErr:    false,
		},
		{
			name:         "minimal bridge path",
			path:         []string{"my", "partner-mesh"},
			expectedPath: []string{"world", "agent", "bridge-id-123"},
			expectedMesh: "partner-mesh",
			expectErr:    false,
		},
		{
			name:      "path too short",
			path:      []string{"my"},
			expectErr: true,
		},
		{
			name:      "not a bridge path",
			path:      []string{"local", "data"},
			expectErr: true,
		},
		{
			name:      "non-existent bridge",
			path:      []string{"my", "unknown-mesh", "data"},
			expectErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			resolvedPath, meshName, err := bridgeStorage.ResolveBridgePath(test.path)

			if test.expectErr {
				if err == nil {
					t.Errorf("Expected error for %s, but got none", test.name)
				}
				return
			}

			if err != nil {
				t.Errorf("Expected no error for %s, but got: %v", test.name, err)
				return
			}

			if !equalStringSlices(resolvedPath, test.expectedPath) {
				t.Errorf("Expected path %v, got %v", test.expectedPath, resolvedPath)
			}

			if meshName != test.expectedMesh {
				t.Errorf("Expected mesh name %s, got %s", test.expectedMesh, meshName)
			}
		})
	}
}

func TestBridgeStorage_IsBridgePath(t *testing.T) {
	localStorage := NewMemoryTree()
	bridgeStorage := NewBridgeStorage(localStorage)

	// Register a test bridge
	testIdentity := &identity.Identity{
		ID:       "bridge-id-123",
		MeshName: "partner-mesh",
	}

	connection := &BridgeConnection{
		MeshName:      "partner-mesh",
		Identity:      testIdentity,
		Status:        "connected",
		TargetAddress: "192.168.1.10:5000",
	}

	err := bridgeStorage.RegisterBridge("partner-mesh", connection)
	if err != nil {
		t.Fatalf("Failed to register bridge: %v", err)
	}

	tests := []struct {
		name     string
		path     []string
		expected bool
	}{
		{
			name:     "valid bridge path",
			path:     []string{"my", "partner-mesh", "data"},
			expected: true,
		},
		{
			name:     "minimal bridge path",
			path:     []string{"my", "partner-mesh"},
			expected: true,
		},
		{
			name:     "not a bridge path",
			path:     []string{"local", "data"},
			expected: false,
		},
		{
			name:     "wrong prefix",
			path:     []string{"world", "partner-mesh"},
			expected: false,
		},
		{
			name:     "non-existent bridge",
			path:     []string{"my", "unknown-mesh"},
			expected: false,
		},
		{
			name:     "path too short",
			path:     []string{"my"},
			expected: false,
		},
		{
			name:     "empty path",
			path:     []string{},
			expected: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := bridgeStorage.IsBridgePath(test.path)
			if result != test.expected {
				t.Errorf("Expected IsBridgePath to return %v for path %v, got %v",
					test.expected, test.path, result)
			}
		})
	}
}

func TestBridgeStorage_RemoveBridge(t *testing.T) {
	localStorage := NewMemoryTree()
	bridgeStorage := NewBridgeStorage(localStorage)

	// Register a test bridge
	testIdentity := &identity.Identity{
		ID:       "bridge-id-123",
		MeshName: "partner-mesh",
	}

	connection := &BridgeConnection{
		MeshName:      "partner-mesh",
		Identity:      testIdentity,
		Status:        "connected",
		TargetAddress: "192.168.1.10:5000",
	}

	err := bridgeStorage.RegisterBridge("partner-mesh", connection)
	if err != nil {
		t.Fatalf("Failed to register bridge: %v", err)
	}

	// Test successful removal
	err = bridgeStorage.RemoveBridge("partner-mesh")
	if err != nil {
		t.Fatalf("Failed to remove bridge: %v", err)
	}

	// Verify bridge is removed
	bridges := bridgeStorage.GetRegisteredBridges()
	if len(bridges) != 0 {
		t.Errorf("Expected no bridges after removal, got: %v", bridges)
	}

	// Test removing non-existent bridge
	err = bridgeStorage.RemoveBridge("non-existent")
	if err == nil {
		t.Error("Expected error when removing non-existent bridge")
	}
}

func TestBridgeStorage_ValidateBridgeAccess(t *testing.T) {
	localStorage := NewMemoryTree()
	bridgeStorage := NewBridgeStorage(localStorage)

	// Register connected bridge
	connectedIdentity := &identity.Identity{
		ID:       "connected-bridge-id",
		MeshName: "connected-mesh",
	}

	connectedConnection := &BridgeConnection{
		MeshName:      "connected-mesh",
		Identity:      connectedIdentity,
		Status:        "connected",
		TargetAddress: "192.168.1.10:5000",
	}

	err := bridgeStorage.RegisterBridge("connected-mesh", connectedConnection)
	if err != nil {
		t.Fatalf("Failed to register connected bridge: %v", err)
	}

	// Register disconnected bridge
	disconnectedIdentity := &identity.Identity{
		ID:       "disconnected-bridge-id",
		MeshName: "disconnected-mesh",
	}

	disconnectedConnection := &BridgeConnection{
		MeshName:      "disconnected-mesh",
		Identity:      disconnectedIdentity,
		Status:        "disconnected",
		TargetAddress: "192.168.1.11:5000",
	}

	err = bridgeStorage.RegisterBridge("disconnected-mesh", disconnectedConnection)
	if err != nil {
		t.Fatalf("Failed to register disconnected bridge: %v", err)
	}

	tests := []struct {
		name      string
		meshName  string
		expectErr bool
	}{
		{
			name:      "connected bridge",
			meshName:  "connected-mesh",
			expectErr: false,
		},
		{
			name:      "disconnected bridge",
			meshName:  "disconnected-mesh",
			expectErr: true,
		},
		{
			name:      "non-existent bridge",
			meshName:  "non-existent",
			expectErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := bridgeStorage.ValidateBridgeAccess(test.meshName)
			if test.expectErr && err == nil {
				t.Errorf("Expected error for %s, but got none", test.name)
			}
			if !test.expectErr && err != nil {
				t.Errorf("Expected no error for %s, but got: %v", test.name, err)
			}
		})
	}
}

func TestBridgeStorage_DataOperations(t *testing.T) {
	localStorage := NewMemoryTree()
	bridgeStorage := NewBridgeStorage(localStorage)

	// Register a test bridge
	testIdentity := &identity.Identity{
		ID:       "bridge-id-123",
		MeshName: "partner-mesh",
	}

	connection := &BridgeConnection{
		MeshName:      "partner-mesh",
		Identity:      testIdentity,
		Status:        "connected",
		TargetAddress: "192.168.1.10:5000",
	}

	err := bridgeStorage.RegisterBridge("partner-mesh", connection)
	if err != nil {
		t.Fatalf("Failed to register bridge: %v", err)
	}

	// Test write and read
	path := []string{"my", "partner-mesh", "test", "data"}
	value := Value{Data: []byte("test data")}
	author := uint64(1)

	// Write data
	err = bridgeStorage.WriteBridgeData(path, value, author)
	if err != nil {
		t.Fatalf("Failed to write bridge data: %v", err)
	}

	// Read data back
	readValue, err := bridgeStorage.ReadBridgeData(path)
	if err != nil {
		t.Fatalf("Failed to read bridge data: %v", err)
	}

	if string(readValue.Data) != string(value.Data) {
		t.Errorf("Expected data %q, got %q", value.Data, readValue.Data)
	}

	// Test children (MemoryTree doesn't track children, so we just test the call works)
	children, err := bridgeStorage.GetBridgeChildren([]string{"my", "partner-mesh", "test"})
	if err != nil {
		t.Fatalf("Failed to get bridge children: %v", err)
	}

	// MemoryTree implementation returns empty children, so just verify no error
	_ = children

	// Test purge
	err = bridgeStorage.PurgeBridgeData(path, 0, 9999999999, author)
	if err != nil {
		t.Fatalf("Failed to purge bridge data: %v", err)
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