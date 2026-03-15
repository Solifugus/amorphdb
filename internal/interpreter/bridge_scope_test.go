// Package interpreter provides bridge scope resolver tests
package interpreter

import (
	"testing"

	"github.com/solifugus/amorphdb/internal/identity"
	"github.com/solifugus/amorphdb/internal/storage"
)

func TestBridgeScopeResolver_RegisterBridge(t *testing.T) {
	localStorage := storage.NewMemoryTree()
	bridgeStorage := storage.NewBridgeStorage(localStorage)
	resolver := NewBridgeScopeResolver(bridgeStorage)

	// Create test identity
	testIdentity := &identity.Identity{
		ID:       "bridge-test-123",
		MeshName: "partner-mesh",
	}

	// Test successful registration
	err := resolver.RegisterBridge("partner-mesh", testIdentity, "connected", "192.168.1.10:5000")
	if err != nil {
		t.Fatalf("Failed to register bridge: %v", err)
	}

	// Test bridge is registered
	bridges := resolver.GetRegisteredBridges()
	if len(bridges) != 1 || bridges[0] != "partner-mesh" {
		t.Errorf("Expected bridge 'partner-mesh' to be registered, got: %v", bridges)
	}

	// Test status
	status, err := resolver.GetBridgeStatus("partner-mesh")
	if err != nil {
		t.Fatalf("Failed to get bridge status: %v", err)
	}
	if status != "connected" {
		t.Errorf("Expected status 'connected', got: %s", status)
	}
}

func TestBridgeScopeResolver_RegisterBridge_InvalidInput(t *testing.T) {
	localStorage := storage.NewMemoryTree()
	bridgeStorage := storage.NewBridgeStorage(localStorage)
	resolver := NewBridgeScopeResolver(bridgeStorage)

	tests := []struct {
		name          string
		meshName      string
		identity      *identity.Identity
		expectErr     bool
	}{
		{
			name:      "empty mesh name",
			meshName:  "",
			identity:  &identity.Identity{ID: "test"},
			expectErr: true,
		},
		{
			name:      "nil identity",
			meshName:  "test-mesh",
			identity:  nil,
			expectErr: true,
		},
		{
			name:      "empty identity ID",
			meshName:  "test-mesh",
			identity:  &identity.Identity{ID: ""},
			expectErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := resolver.RegisterBridge(test.meshName, test.identity, "connected", "address")
			if test.expectErr && err == nil {
				t.Errorf("Expected error for %s, but got none", test.name)
			}
			if !test.expectErr && err != nil {
				t.Errorf("Expected no error for %s, but got: %v", test.name, err)
			}
		})
	}
}

func TestBridgeScopeResolver_ResolveBridgePath(t *testing.T) {
	localStorage := storage.NewMemoryTree()
	bridgeStorage := storage.NewBridgeStorage(localStorage)
	resolver := NewBridgeScopeResolver(bridgeStorage)

	// Register a test bridge
	testIdentity := &identity.Identity{
		ID:       "bridge-id-123",
		MeshName: "partner-mesh",
	}

	err := resolver.RegisterBridge("partner-mesh", testIdentity, "connected", "192.168.1.10:5000")
	if err != nil {
		t.Fatalf("Failed to register bridge: %v", err)
	}

	tests := []struct {
		name              string
		pathString        string
		expectedMeshName  string
		expectedAgentHome string
		expectedBridgeID  string
		expectErr         bool
		expectNil         bool
	}{
		{
			name:              "valid bridge path",
			pathString:        "my.partner-mesh.data.users",
			expectedMeshName:  "partner-mesh",
			expectedAgentHome: "world.agent.bridge-id-123",
			expectedBridgeID:  "bridge-id-123",
			expectErr:         false,
			expectNil:         false,
		},
		{
			name:              "minimal bridge path",
			pathString:        "my.partner-mesh",
			expectedMeshName:  "partner-mesh",
			expectedAgentHome: "world.agent.bridge-id-123",
			expectedBridgeID:  "bridge-id-123",
			expectErr:         false,
			expectNil:         false,
		},
		{
			name:       "not a bridge path",
			pathString: "local.data.users",
			expectNil:  true,
		},
		{
			name:       "world path",
			pathString: "world.agent.123",
			expectNil:  true,
		},
		{
			name:      "non-existent bridge",
			pathString: "my.unknown-mesh.data",
			expectErr: true,
		},
		{
			name:      "invalid bridge path",
			pathString: "my",
			expectErr: true,
		},
		{
			name:      "invalid mesh name in path",
			pathString: "my.invalid@mesh.data",
			expectErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			scope, err := resolver.ResolveBridgePath(test.pathString)

			if test.expectErr {
				if err == nil {
					t.Errorf("Expected error for %s, but got none", test.name)
				}
				return
			}

			if test.expectNil {
				if scope != nil {
					t.Errorf("Expected nil scope for %s, but got: %v", test.name, scope)
				}
				return
			}

			if err != nil {
				t.Errorf("Expected no error for %s, but got: %v", test.name, err)
				return
			}

			if scope == nil {
				t.Errorf("Expected scope for %s, but got nil", test.name)
				return
			}

			if scope.MeshName != test.expectedMeshName {
				t.Errorf("Expected mesh name %s, got %s", test.expectedMeshName, scope.MeshName)
			}

			if scope.AgentHome != test.expectedAgentHome {
				t.Errorf("Expected agent home %s, got %s", test.expectedAgentHome, scope.AgentHome)
			}

			if scope.BridgeID != test.expectedBridgeID {
				t.Errorf("Expected bridge ID %s, got %s", test.expectedBridgeID, scope.BridgeID)
			}

			if scope.LocalPath != test.pathString {
				t.Errorf("Expected local path %s, got %s", test.pathString, scope.LocalPath)
			}
		})
	}
}

func TestBridgeScopeResolver_ResolvePathComponents(t *testing.T) {
	localStorage := storage.NewMemoryTree()
	bridgeStorage := storage.NewBridgeStorage(localStorage)
	resolver := NewBridgeScopeResolver(bridgeStorage)

	// Register a test bridge
	testIdentity := &identity.Identity{
		ID:       "bridge-id-456",
		MeshName: "test-mesh",
	}

	err := resolver.RegisterBridge("test-mesh", testIdentity, "connected", "192.168.1.20:5000")
	if err != nil {
		t.Fatalf("Failed to register bridge: %v", err)
	}

	tests := []struct {
		name              string
		pathComponents    []string
		expectedMeshName  string
		expectedAgentHome string
		expectErr         bool
		expectNil         bool
	}{
		{
			name:              "valid bridge components",
			pathComponents:    []string{"my", "test-mesh", "users", "data"},
			expectedMeshName:  "test-mesh",
			expectedAgentHome: "world.agent.bridge-id-456",
			expectErr:         false,
			expectNil:         false,
		},
		{
			name:           "local components",
			pathComponents: []string{"users", "john"},
			expectNil:      true,
		},
		{
			name:           "world components",
			pathComponents: []string{"world", "global"},
			expectNil:      true,
		},
		{
			name:           "non-existent bridge components",
			pathComponents: []string{"my", "nonexistent"},
			expectErr:      true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			scope, err := resolver.ResolvePathComponents(test.pathComponents)

			if test.expectErr {
				if err == nil {
					t.Errorf("Expected error for %s, but got none", test.name)
				}
				return
			}

			if test.expectNil {
				if scope != nil {
					t.Errorf("Expected nil scope for %s, but got: %v", test.name, scope)
				}
				return
			}

			if err != nil {
				t.Errorf("Expected no error for %s, but got: %v", test.name, err)
				return
			}

			if scope == nil {
				t.Errorf("Expected scope for %s, but got nil", test.name)
				return
			}

			if scope.MeshName != test.expectedMeshName {
				t.Errorf("Expected mesh name %s, got %s", test.expectedMeshName, scope.MeshName)
			}

			if scope.AgentHome != test.expectedAgentHome {
				t.Errorf("Expected agent home %s, got %s", test.expectedAgentHome, scope.AgentHome)
			}
		})
	}
}

func TestBridgeScope_GetTargetPath(t *testing.T) {
	localStorage := storage.NewMemoryTree()
	bridgeStorage := storage.NewBridgeStorage(localStorage)
	resolver := NewBridgeScopeResolver(bridgeStorage)

	// Register a test bridge
	testIdentity := &identity.Identity{
		ID:       "target-bridge-789",
		MeshName: "target-mesh",
	}

	err := resolver.RegisterBridge("target-mesh", testIdentity, "connected", "192.168.1.30:5000")
	if err != nil {
		t.Fatalf("Failed to register bridge: %v", err)
	}

	// Resolve a bridge path
	scope, err := resolver.ResolveBridgePath("my.target-mesh.users.john.profile")
	if err != nil {
		t.Fatalf("Failed to resolve bridge path: %v", err)
	}

	// Get target path
	targetPath, err := scope.GetTargetPath()
	if err != nil {
		t.Fatalf("Failed to get target path: %v", err)
	}

	expectedPath := []string{"world", "agent", "target-bridge-789", "users", "john", "profile"}
	if !equalStringSlices(targetPath, expectedPath) {
		t.Errorf("Expected target path %v, got %v", expectedPath, targetPath)
	}
}

func TestBridgeScope_ValidateAccess(t *testing.T) {
	localStorage := storage.NewMemoryTree()
	bridgeStorage := storage.NewBridgeStorage(localStorage)
	resolver := NewBridgeScopeResolver(bridgeStorage)

	// Register connected bridge
	connectedIdentity := &identity.Identity{
		ID:       "connected-bridge",
		MeshName: "connected-mesh",
	}

	err := resolver.RegisterBridge("connected-mesh", connectedIdentity, "connected", "192.168.1.40:5000")
	if err != nil {
		t.Fatalf("Failed to register connected bridge: %v", err)
	}

	// Register disconnected bridge
	disconnectedIdentity := &identity.Identity{
		ID:       "disconnected-bridge",
		MeshName: "disconnected-mesh",
	}

	err = resolver.RegisterBridge("disconnected-mesh", disconnectedIdentity, "disconnected", "192.168.1.41:5000")
	if err != nil {
		t.Fatalf("Failed to register disconnected bridge: %v", err)
	}

	// Test connected bridge access
	connectedScope, err := resolver.ResolveBridgePath("my.connected-mesh.data")
	if err != nil {
		t.Fatalf("Failed to resolve connected bridge path: %v", err)
	}

	err = connectedScope.ValidateAccess()
	if err != nil {
		t.Errorf("Expected connected bridge to validate successfully, got error: %v", err)
	}

	// Test disconnected bridge access
	disconnectedScope, err := resolver.ResolveBridgePath("my.disconnected-mesh.data")
	if err != nil {
		t.Fatalf("Failed to resolve disconnected bridge path: %v", err)
	}

	err = disconnectedScope.ValidateAccess()
	if err == nil {
		t.Error("Expected disconnected bridge validation to fail, but it succeeded")
	}
}

func TestBridgeScopeResolver_UpdateBridgeStatus(t *testing.T) {
	localStorage := storage.NewMemoryTree()
	bridgeStorage := storage.NewBridgeStorage(localStorage)
	resolver := NewBridgeScopeResolver(bridgeStorage)

	// Register a test bridge
	testIdentity := &identity.Identity{
		ID:       "status-test-bridge",
		MeshName: "status-test-mesh",
	}

	err := resolver.RegisterBridge("status-test-mesh", testIdentity, "connecting", "192.168.1.50:5000")
	if err != nil {
		t.Fatalf("Failed to register bridge: %v", err)
	}

	// Test initial status
	status, err := resolver.GetBridgeStatus("status-test-mesh")
	if err != nil {
		t.Fatalf("Failed to get initial bridge status: %v", err)
	}
	if status != "connecting" {
		t.Errorf("Expected initial status 'connecting', got %s", status)
	}

	// Update status
	err = resolver.UpdateBridgeStatus("status-test-mesh", "connected")
	if err != nil {
		t.Fatalf("Failed to update bridge status: %v", err)
	}

	// Test updated status
	status, err = resolver.GetBridgeStatus("status-test-mesh")
	if err != nil {
		t.Fatalf("Failed to get updated bridge status: %v", err)
	}
	if status != "connected" {
		t.Errorf("Expected updated status 'connected', got %s", status)
	}

	// Test updating non-existent bridge
	err = resolver.UpdateBridgeStatus("nonexistent-mesh", "connected")
	if err == nil {
		t.Error("Expected error when updating status of non-existent bridge")
	}
}

func TestBridgeScopeResolver_PathTypeCheckers(t *testing.T) {
	localStorage := storage.NewMemoryTree()
	bridgeStorage := storage.NewBridgeStorage(localStorage)
	resolver := NewBridgeScopeResolver(bridgeStorage)

	// Register a test bridge
	testIdentity := &identity.Identity{
		ID:       "checker-test-bridge",
		MeshName: "checker-test-mesh",
	}

	err := resolver.RegisterBridge("checker-test-mesh", testIdentity, "connected", "192.168.1.60:5000")
	if err != nil {
		t.Fatalf("Failed to register bridge: %v", err)
	}

	tests := []struct {
		pathString    string
		pathComponents []string
		expectBridge   bool
	}{
		{"my.checker-test-mesh.data", []string{"my", "checker-test-mesh", "data"}, true},
		{"my.nonexistent.data", []string{"my", "nonexistent", "data"}, false},
		{"local.data", []string{"local", "data"}, false},
		{"world.agent.123", []string{"world", "agent", "123"}, false},
		{"my", []string{"my"}, false}, // invalid bridge path
	}

	for _, test := range tests {
		t.Run(test.pathString, func(t *testing.T) {
			// Test string version
			stringResult := resolver.IsBridgePath(test.pathString)
			if stringResult != test.expectBridge {
				t.Errorf("IsBridgePath(%q) = %v, expected %v",
					test.pathString, stringResult, test.expectBridge)
			}

			// Test components version
			componentsResult := resolver.IsBridgePathComponents(test.pathComponents)
			if componentsResult != test.expectBridge {
				t.Errorf("IsBridgePathComponents(%v) = %v, expected %v",
					test.pathComponents, componentsResult, test.expectBridge)
			}
		})
	}
}

func TestBridgeScope_String(t *testing.T) {
	connection := &BridgeConnection{
		Identity:      &identity.Identity{ID: "test-bridge-id"},
		Status:        "connected",
		TargetAddress: "192.168.1.70:5000",
	}

	scope := &BridgeScope{
		MeshName:   "test-mesh",
		AgentHome:  "world.agent.test-bridge-id",
		LocalPath:  "my.test-mesh.data",
		BridgeID:   "test-bridge-id",
		Connection: connection,
	}

	result := scope.String()
	expected := "BridgeScope{mesh=test-mesh, path=my.test-mesh.data, agent=world.agent.test-bridge-id}"
	if result != expected {
		t.Errorf("Expected string %q, got %q", expected, result)
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