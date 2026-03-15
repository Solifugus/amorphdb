package identity

import (
	"testing"

	"github.com/solifugus/amorphdb/internal/types"
)

// TestIdentityIntegration validates that all identity components work together
func TestIdentityIntegration(t *testing.T) {
	// Initialize identity manager for a test mesh
	im := NewIdentityManager("primary-mesh", "node-001")

	// Generate node identity
	nodeIdentity, err := im.GenerateNodeIdentity()
	if err != nil {
		t.Fatalf("Failed to generate node identity: %v", err)
	}

	// Validate node identity
	if err := ValidateIdentity(nodeIdentity); err != nil {
		t.Fatalf("Node identity validation failed: %v", err)
	}

	// Verify node identity properties
	if nodeIdentity.Type != types.NodeAgent {
		t.Errorf("Expected NodeAgent, got %v", nodeIdentity.Type)
	}

	if nodeIdentity.MeshName != "primary-mesh" {
		t.Errorf("Expected mesh name 'primary-mesh', got %q", nodeIdentity.MeshName)
	}

	// Create bridge identity for partner mesh
	bridgeIdentity, err := im.CreateBridgeIdentity("partner-mesh")
	if err != nil {
		t.Fatalf("Failed to create bridge identity: %v", err)
	}

	// Validate bridge identity
	if err := ValidateIdentity(bridgeIdentity); err != nil {
		t.Fatalf("Bridge identity validation failed: %v", err)
	}

	// Verify bridge identity properties
	if bridgeIdentity.Type != types.MobileAgent {
		t.Errorf("Expected MobileAgent, got %v", bridgeIdentity.Type)
	}

	if bridgeIdentity.MeshName != "partner-mesh" {
		t.Errorf("Expected mesh name 'partner-mesh', got %q", bridgeIdentity.MeshName)
	}

	// Verify identities don't conflict
	if IsConflict(nodeIdentity, bridgeIdentity) {
		t.Error("Node and bridge identities should not conflict")
	}

	// Test agent home paths
	nodePath := im.GetAgentHomePath(nodeIdentity)
	expectedNodePath := "world.node.node-001"
	if nodePath != expectedNodePath {
		t.Errorf("Expected node path %q, got %q", expectedNodePath, nodePath)
	}

	bridgePath := im.GetAgentHomePath(bridgeIdentity)
	expectedBridgePath := "world.agent." + bridgeIdentity.ID
	if bridgePath != expectedBridgePath {
		t.Errorf("Expected bridge path %q, got %q", expectedBridgePath, bridgePath)
	}

	// Test bridge identity retrieval
	retrieved := im.GetBridgeIdentity("partner-mesh")
	if retrieved == nil {
		t.Fatal("Should retrieve bridge identity")
	}

	if retrieved.ID != bridgeIdentity.ID {
		t.Error("Retrieved bridge identity should match created identity")
	}

	// Test listing bridge identities
	bridges := im.ListBridgeIdentities()
	if len(bridges) != 1 {
		t.Errorf("Expected 1 bridge identity, got %d", len(bridges))
	}

	// Test mesh name update
	err = im.UpdateMeshName("new-primary-mesh")
	if err != nil {
		t.Fatalf("Failed to update mesh name: %v", err)
	}

	updated := im.GetNodeIdentity()
	if updated.MeshName != "new-primary-mesh" {
		t.Errorf("Expected updated mesh name 'new-primary-mesh', got %q", updated.MeshName)
	}

	// Test removing bridge identity
	im.RemoveBridgeIdentity("partner-mesh")
	if im.GetBridgeIdentity("partner-mesh") != nil {
		t.Error("Bridge identity should be removed")
	}

	bridges = im.ListBridgeIdentities()
	if len(bridges) != 0 {
		t.Errorf("Expected 0 bridge identities after removal, got %d", len(bridges))
	}
}