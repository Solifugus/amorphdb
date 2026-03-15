// Package mesh provides bridge management functionality for AmorphDB
package mesh

import (
	"fmt"
	"time"

	"github.com/solifugus/amorphdb/internal/identity"
)

// BridgeManager manages bridge connections to other meshes
type BridgeManager struct {
	primaryMesh string                                // Name of the primary mesh
	bridges     map[string]*BridgeConnection         // Active bridge connections by mesh name
	identities  map[string]*identity.Identity        // Bridge identities by mesh name
}

// NewBridgeManager creates a new bridge manager
func NewBridgeManager(primaryMesh string) *BridgeManager {
	return &BridgeManager{
		primaryMesh: primaryMesh,
		bridges:     make(map[string]*BridgeConnection),
		identities:  make(map[string]*identity.Identity),
	}
}

// CreateBridge establishes a bridge connection to another mesh
func (bm *BridgeManager) CreateBridge(targetAddress string, nodeIdentity *identity.Identity) (*BridgeConnection, error) {
	// Step 1: Discover the target mesh
	discoveryClient := NewDiscoveryClient()
	meshInfo, err := discoveryClient.DiscoverMesh(targetAddress)
	if err != nil {
		return nil, fmt.Errorf("failed to discover target mesh: %w", err)
	}

	// Check if we already have a bridge to this mesh
	if _, exists := bm.bridges[meshInfo.Name]; exists {
		return nil, fmt.Errorf("bridge to mesh '%s' already exists", meshInfo.Name)
	}

	// Step 2: Request bridge identity in target mesh
	bridgeIdentity, err := bm.requestBridgeIdentity(meshInfo, nodeIdentity)
	if err != nil {
		return nil, fmt.Errorf("failed to request bridge identity: %w", err)
	}

	// Step 3: Establish the bridge connection
	bridge, err := bm.establishBridge(targetAddress, meshInfo, bridgeIdentity)
	if err != nil {
		return nil, fmt.Errorf("failed to establish bridge: %w", err)
	}

	// Step 4: Store the bridge and identity
	bm.bridges[meshInfo.Name] = bridge
	bm.identities[meshInfo.Name] = bridgeIdentity

	return bridge, nil
}

// requestBridgeIdentity requests an identity assignment in the target mesh
func (bm *BridgeManager) requestBridgeIdentity(meshInfo *MeshInfo, nodeIdentity *identity.Identity) (*identity.Identity, error) {
	// For bridge connections, we act as a mobile agent in the target mesh
	// Generate a bridge-specific identity derived from our node identity
	bridgeID := fmt.Sprintf("bridge-%s-to-%s", nodeIdentity.ID, meshInfo.Name)

	bridgeIdentity := &identity.Identity{
		ID:       bridgeID,
		Type:     nodeIdentity.Type, // Maintain same agent type
		MeshName: meshInfo.Name,      // Target mesh name
		Created:  time.Now().UTC(),
		NodeID:   nodeIdentity.NodeID,
	}

	return bridgeIdentity, nil
}

// establishBridge creates the actual bridge connection
func (bm *BridgeManager) establishBridge(targetAddress string, meshInfo *MeshInfo, bridgeIdentity *identity.Identity) (*BridgeConnection, error) {
	bridge := &BridgeConnection{
		MeshName:    meshInfo.Name,
		Address:     targetAddress,
		Identity:    bridgeIdentity,
		Status:      BridgeStatusConnecting,
		ConnectedAt: time.Time{}, // Will be set when actually connected
	}

	// Attempt to join the target mesh as a mobile agent
	joinClient := NewJoinClient()
	joinResult, err := joinClient.JoinMesh(targetAddress, bridgeIdentity, meshInfo.Name)
	if err != nil {
		bridge.Status = BridgeStatusFailed
		return bridge, fmt.Errorf("failed to join target mesh as bridge: %w", err)
	}

	if !joinResult.Success {
		bridge.Status = BridgeStatusFailed
		return bridge, fmt.Errorf("bridge join rejected: %s", joinResult.Message)
	}

	// Bridge successfully established
	bridge.Status = BridgeStatusConnected
	bridge.ConnectedAt = time.Now().UTC()

	return bridge, nil
}

// GetBridge returns a specific bridge connection
func (bm *BridgeManager) GetBridge(meshName string) (*BridgeConnection, bool) {
	bridge, exists := bm.bridges[meshName]
	if !exists {
		return nil, false
	}

	// Return a copy to prevent external modification
	bridgeCopy := *bridge
	return &bridgeCopy, true
}

// GetAllBridges returns all bridge connections
func (bm *BridgeManager) GetAllBridges() map[string]*BridgeConnection {
	result := make(map[string]*BridgeConnection)
	for name, bridge := range bm.bridges {
		bridgeCopy := *bridge
		result[name] = &bridgeCopy
	}
	return result
}

// GetBridgeIdentity returns the identity for a specific bridge
func (bm *BridgeManager) GetBridgeIdentity(meshName string) (*identity.Identity, bool) {
	identity, exists := bm.identities[meshName]
	if !exists {
		return nil, false
	}

	// Return a copy to prevent external modification
	identityCopy := *identity
	return &identityCopy, true
}

// DisconnectBridge removes a bridge connection
func (bm *BridgeManager) DisconnectBridge(meshName string) error {
	bridge, exists := bm.bridges[meshName]
	if !exists {
		return fmt.Errorf("no bridge to mesh '%s' exists", meshName)
	}

	// Mark as disconnected
	bridge.Status = BridgeStatusDisconnected

	// Clean up the bridge and identity
	delete(bm.bridges, meshName)
	delete(bm.identities, meshName)

	return nil
}

// ValidateBridgeAccess checks if we have access to a specific mesh via bridge
func (bm *BridgeManager) ValidateBridgeAccess(meshName string) error {
	bridge, exists := bm.bridges[meshName]
	if !exists {
		return fmt.Errorf("no bridge to mesh '%s' exists", meshName)
	}

	if bridge.Status != BridgeStatusConnected {
		return fmt.Errorf("bridge to mesh '%s' is not connected (status: %s)", meshName, bridge.Status)
	}

	return nil
}

// GetCrossMeshDataPath converts a local path to a cross-mesh path
func (bm *BridgeManager) GetCrossMeshDataPath(meshName string, localPath []string) ([]string, error) {
	// Validate bridge access
	if err := bm.ValidateBridgeAccess(meshName); err != nil {
		return nil, err
	}

	// Get bridge identity
	bridgeIdentity, exists := bm.identities[meshName]
	if !exists {
		return nil, fmt.Errorf("no bridge identity for mesh '%s'", meshName)
	}

	// Convert to bridge agent home path in the target mesh
	// my.meshname.* -> world.agent.{bridge_identity}.*
	if len(localPath) >= 2 && localPath[0] == "my" && localPath[1] == meshName {
		// Build bridge agent path: world.agent.{bridge_identity}.{rest_of_path}
		bridgePath := []string{"world", "agent", bridgeIdentity.ID}
		if len(localPath) > 2 {
			bridgePath = append(bridgePath, localPath[2:]...)
		}
		return bridgePath, nil
	}

	return nil, fmt.Errorf("invalid cross-mesh path format: expected 'my.%s.*', got '%v'", meshName, localPath)
}


// RemoveBridge removes a bridge connection for a specific mesh
func (bm *BridgeManager) RemoveBridge(meshName string) error {
	_, exists := bm.bridges[meshName]
	if !exists {
		return fmt.Errorf("no bridge to mesh '%s' exists", meshName)
	}

	// Clean up the bridge and identity
	delete(bm.bridges, meshName)
	delete(bm.identities, meshName)

	return nil
}