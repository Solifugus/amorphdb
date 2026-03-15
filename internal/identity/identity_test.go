package identity

import (
	"testing"
	"time"

	"github.com/solifugus/amorphdb/internal/types"
)

func TestNewIdentityManager(t *testing.T) {
	meshName := "test-mesh"
	nodeID := "node-123"

	im := NewIdentityManager(meshName, nodeID)

	if im == nil {
		t.Fatal("NewIdentityManager returned nil")
	}

	if im.meshName != meshName {
		t.Errorf("Expected mesh name %q, got %q", meshName, im.meshName)
	}

	if im.nodeID != nodeID {
		t.Errorf("Expected node ID %q, got %q", nodeID, im.nodeID)
	}

	if im.bridgeIdentities == nil {
		t.Error("Bridge identities map should be initialized")
	}

	if len(im.bridgeIdentities) != 0 {
		t.Error("Bridge identities should be empty initially")
	}
}

func TestGenerateNodeIdentity(t *testing.T) {
	im := NewIdentityManager("test-mesh", "node-123")

	identity, err := im.GenerateNodeIdentity()
	if err != nil {
		t.Fatalf("Failed to generate node identity: %v", err)
	}

	// Check identity properties
	if identity.ID == "" {
		t.Error("Identity ID should not be empty")
	}

	if identity.Type != types.NodeAgent {
		t.Errorf("Expected NodeAgent type, got %v", identity.Type)
	}

	if identity.MeshName != "test-mesh" {
		t.Errorf("Expected mesh name 'test-mesh', got %q", identity.MeshName)
	}

	if identity.NodeID != "node-123" {
		t.Errorf("Expected node ID 'node-123', got %q", identity.NodeID)
	}

	if identity.Created.IsZero() {
		t.Error("Created timestamp should be set")
	}

	// Check that identity is stored in manager
	stored := im.GetNodeIdentity()
	if stored == nil {
		t.Error("Node identity should be stored in manager")
	} else if stored.ID != identity.ID {
		t.Error("Stored identity should match generated identity")
	}
}

func TestGenerateNodeIdentity_UniqueIDs(t *testing.T) {
	im := NewIdentityManager("test-mesh", "node-123")

	// Generate multiple identities and ensure they're unique
	identity1, err := im.GenerateNodeIdentity()
	if err != nil {
		t.Fatalf("Failed to generate first identity: %v", err)
	}

	identity2, err := im.GenerateNodeIdentity()
	if err != nil {
		t.Fatalf("Failed to generate second identity: %v", err)
	}

	if identity1.ID == identity2.ID {
		t.Error("Generated identities should have unique IDs")
	}
}

func TestSetNodeIdentity(t *testing.T) {
	im := NewIdentityManager("test-mesh", "node-123")

	identity := &Identity{
		ID:       "test-id-123",
		Type:     types.NodeAgent,
		MeshName: "test-mesh",
		Created:  time.Now().UTC(),
		NodeID:   "node-123",
	}

	err := im.SetNodeIdentity(identity)
	if err != nil {
		t.Fatalf("Failed to set node identity: %v", err)
	}

	stored := im.GetNodeIdentity()
	if stored == nil {
		t.Fatal("Node identity should be stored")
	}

	if stored.ID != identity.ID {
		t.Errorf("Expected identity ID %q, got %q", identity.ID, stored.ID)
	}
}

func TestSetNodeIdentity_InvalidType(t *testing.T) {
	im := NewIdentityManager("test-mesh", "node-123")

	identity := &Identity{
		ID:       "test-id-123",
		Type:     types.MobileAgent, // Invalid for node identity
		MeshName: "test-mesh",
		Created:  time.Now().UTC(),
		NodeID:   "node-123",
	}

	err := im.SetNodeIdentity(identity)
	if err == nil {
		t.Error("Expected error when setting mobile agent as node identity")
	}
}

func TestCreateBridgeIdentity(t *testing.T) {
	im := NewIdentityManager("primary-mesh", "node-123")

	identity, err := im.CreateBridgeIdentity("partner-mesh")
	if err != nil {
		t.Fatalf("Failed to create bridge identity: %v", err)
	}

	// Check identity properties
	if identity.ID == "" {
		t.Error("Bridge identity ID should not be empty")
	}

	if identity.Type != types.MobileAgent {
		t.Errorf("Expected MobileAgent type, got %v", identity.Type)
	}

	if identity.MeshName != "partner-mesh" {
		t.Errorf("Expected mesh name 'partner-mesh', got %q", identity.MeshName)
	}

	if identity.NodeID != "node-123" {
		t.Errorf("Expected node ID 'node-123', got %q", identity.NodeID)
	}

	// Check that identity is stored
	stored := im.GetBridgeIdentity("partner-mesh")
	if stored == nil {
		t.Error("Bridge identity should be stored in manager")
	} else if stored.ID != identity.ID {
		t.Error("Stored identity should match created identity")
	}
}

func TestCreateBridgeIdentity_AlreadyExists(t *testing.T) {
	im := NewIdentityManager("primary-mesh", "node-123")

	// Create first identity
	identity1, err := im.CreateBridgeIdentity("partner-mesh")
	if err != nil {
		t.Fatalf("Failed to create first bridge identity: %v", err)
	}

	// Create second identity for same mesh - should return existing
	identity2, err := im.CreateBridgeIdentity("partner-mesh")
	if err != nil {
		t.Fatalf("Failed to create second bridge identity: %v", err)
	}

	if identity1.ID != identity2.ID {
		t.Error("Should return existing identity for same mesh")
	}
}

func TestRemoveBridgeIdentity(t *testing.T) {
	im := NewIdentityManager("primary-mesh", "node-123")

	// Create bridge identity
	_, err := im.CreateBridgeIdentity("partner-mesh")
	if err != nil {
		t.Fatalf("Failed to create bridge identity: %v", err)
	}

	// Verify it exists
	if im.GetBridgeIdentity("partner-mesh") == nil {
		t.Fatal("Bridge identity should exist")
	}

	// Remove it
	im.RemoveBridgeIdentity("partner-mesh")

	// Verify it's gone
	if im.GetBridgeIdentity("partner-mesh") != nil {
		t.Error("Bridge identity should be removed")
	}
}

func TestListBridgeIdentities(t *testing.T) {
	im := NewIdentityManager("primary-mesh", "node-123")

	// Create multiple bridge identities
	_, err := im.CreateBridgeIdentity("mesh1")
	if err != nil {
		t.Fatalf("Failed to create bridge identity for mesh1: %v", err)
	}

	_, err = im.CreateBridgeIdentity("mesh2")
	if err != nil {
		t.Fatalf("Failed to create bridge identity for mesh2: %v", err)
	}

	identities := im.ListBridgeIdentities()

	if len(identities) != 2 {
		t.Errorf("Expected 2 bridge identities, got %d", len(identities))
	}

	if identities["mesh1"] == nil {
		t.Error("Should have identity for mesh1")
	}

	if identities["mesh2"] == nil {
		t.Error("Should have identity for mesh2")
	}
}

func TestUpdateMeshName(t *testing.T) {
	im := NewIdentityManager("old-mesh", "node-123")

	// Generate node identity
	identity, err := im.GenerateNodeIdentity()
	if err != nil {
		t.Fatalf("Failed to generate node identity: %v", err)
	}

	if identity.MeshName != "old-mesh" {
		t.Errorf("Expected mesh name 'old-mesh', got %q", identity.MeshName)
	}

	// Update mesh name
	err = im.UpdateMeshName("new-mesh")
	if err != nil {
		t.Fatalf("Failed to update mesh name: %v", err)
	}

	// Check that node identity was updated
	updated := im.GetNodeIdentity()
	if updated.MeshName != "new-mesh" {
		t.Errorf("Expected updated mesh name 'new-mesh', got %q", updated.MeshName)
	}
}

func TestGetAgentHomePath(t *testing.T) {
	im := NewIdentityManager("test-mesh", "node-123")

	tests := []struct {
		name     string
		identity *Identity
		expected string
	}{
		{
			name: "node agent",
			identity: &Identity{
				ID:     "agent-456",
				Type:   types.NodeAgent,
				NodeID: "node-123",
			},
			expected: "world.node.node-123",
		},
		{
			name: "mobile agent",
			identity: &Identity{
				ID:     "agent-789",
				Type:   types.MobileAgent,
				NodeID: "node-123",
			},
			expected: "world.agent.agent-789",
		},
		{
			name: "invalid agent type",
			identity: &Identity{
				ID:     "agent-999",
				Type:   types.AgentType(999),
				NodeID: "node-123",
			},
			expected: "",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := im.GetAgentHomePath(test.identity)
			if path != test.expected {
				t.Errorf("Expected path %q, got %q", test.expected, path)
			}
		})
	}
}

func TestValidateIdentity(t *testing.T) {
	tests := []struct {
		name     string
		identity *Identity
		wantErr  bool
	}{
		{
			name: "valid node identity",
			identity: &Identity{
				ID:       "valid-id",
				Type:     types.NodeAgent,
				MeshName: "test-mesh",
				Created:  time.Now(),
			},
			wantErr: false,
		},
		{
			name: "valid mobile identity",
			identity: &Identity{
				ID:       "valid-id",
				Type:     types.MobileAgent,
				MeshName: "test-mesh",
				Created:  time.Now(),
			},
			wantErr: false,
		},
		{
			name:     "nil identity",
			identity: nil,
			wantErr:  true,
		},
		{
			name: "empty ID",
			identity: &Identity{
				ID:       "",
				Type:     types.NodeAgent,
				MeshName: "test-mesh",
				Created:  time.Now(),
			},
			wantErr: true,
		},
		{
			name: "invalid type",
			identity: &Identity{
				ID:       "valid-id",
				Type:     types.AgentType(999),
				MeshName: "test-mesh",
				Created:  time.Now(),
			},
			wantErr: true,
		},
		{
			name: "empty mesh name",
			identity: &Identity{
				ID:       "valid-id",
				Type:     types.NodeAgent,
				MeshName: "",
				Created:  time.Now(),
			},
			wantErr: true,
		},
		{
			name: "zero created time",
			identity: &Identity{
				ID:       "valid-id",
				Type:     types.NodeAgent,
				MeshName: "test-mesh",
				Created:  time.Time{},
			},
			wantErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := ValidateIdentity(test.identity)
			if test.wantErr && err == nil {
				t.Error("Expected validation error, got nil")
			} else if !test.wantErr && err != nil {
				t.Errorf("Expected no validation error, got: %v", err)
			}
		})
	}
}

func TestIsConflict(t *testing.T) {
	baseTime := time.Now()

	tests := []struct {
		name      string
		id1       *Identity
		id2       *Identity
		expectErr bool
	}{
		{
			name: "same ID same mesh",
			id1: &Identity{
				ID:       "same-id",
				Type:     types.NodeAgent,
				MeshName: "same-mesh",
				Created:  baseTime,
			},
			id2: &Identity{
				ID:       "same-id",
				Type:     types.MobileAgent,
				MeshName: "same-mesh",
				Created:  baseTime.Add(time.Minute),
			},
			expectErr: true,
		},
		{
			name: "same node ID same mesh",
			id1: &Identity{
				ID:       "id-1",
				Type:     types.NodeAgent,
				MeshName: "same-mesh",
				NodeID:   "same-node",
				Created:  baseTime,
			},
			id2: &Identity{
				ID:       "id-2",
				Type:     types.NodeAgent,
				MeshName: "same-mesh",
				NodeID:   "same-node",
				Created:  baseTime.Add(time.Minute),
			},
			expectErr: true,
		},
		{
			name: "different IDs different mesh",
			id1: &Identity{
				ID:       "id-1",
				Type:     types.NodeAgent,
				MeshName: "mesh-1",
				Created:  baseTime,
			},
			id2: &Identity{
				ID:       "id-2",
				Type:     types.NodeAgent,
				MeshName: "mesh-2",
				Created:  baseTime.Add(time.Minute),
			},
			expectErr: false,
		},
		{
			name:      "nil identities",
			id1:       nil,
			id2:       nil,
			expectErr: false,
		},
		{
			name: "one nil identity",
			id1: &Identity{
				ID:       "id-1",
				Type:     types.NodeAgent,
				MeshName: "mesh-1",
				Created:  baseTime,
			},
			id2:       nil,
			expectErr: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			conflict := IsConflict(test.id1, test.id2)
			if test.expectErr && !conflict {
				t.Error("Expected conflict, got none")
			} else if !test.expectErr && conflict {
				t.Error("Expected no conflict, got conflict")
			}
		})
	}
}

func TestGenerateIdentityID(t *testing.T) {
	// Test that IDs are generated
	id1, err := generateIdentityID()
	if err != nil {
		t.Fatalf("Failed to generate identity ID: %v", err)
	}

	if id1 == "" {
		t.Error("Generated ID should not be empty")
	}

	// Test that IDs are unique
	id2, err := generateIdentityID()
	if err != nil {
		t.Fatalf("Failed to generate second identity ID: %v", err)
	}

	if id1 == id2 {
		t.Error("Generated IDs should be unique")
	}

	// Test that ID is hex string with expected length (32 hex chars = 16 bytes)
	if len(id1) != 32 {
		t.Errorf("Expected ID length 32, got %d", len(id1))
	}
}