// Package interpreter provides bridge scope resolution for AmorphDB multi-mesh operations
package interpreter

import (
	"fmt"

	"github.com/solifugus/amorphdb/internal/identity"
	"github.com/solifugus/amorphdb/internal/storage"
	"github.com/solifugus/amorphdb/internal/types"
)

// BridgeScope represents a resolved bridge scope for cross-mesh operations
type BridgeScope struct {
	MeshName     string               // Target mesh name
	AgentHome    string               // Agent home path (world.agent.{bridge_identity})
	LocalPath    string               // Original local path (my.partnernet.*)
	BridgeID     string               // Bridge identity ID
	Connection   *BridgeConnection    // Bridge connection info
	PathResolver *types.PathValidator // Path validation and parsing
}

// BridgeConnection represents connection information for a bridge
type BridgeConnection struct {
	Identity      *identity.Identity
	Status        string
	TargetAddress string
}

// BridgeScopeResolver handles resolution of bridge scopes for data access
type BridgeScopeResolver struct {
	bridges       map[string]*BridgeConnection // Active bridge connections by mesh name
	bridgeStorage *storage.BridgeStorage       // Bridge storage manager
	pathValidator *types.PathValidator         // Path validation
}

// NewBridgeScopeResolver creates a new bridge scope resolver
func NewBridgeScopeResolver(bridgeStorage *storage.BridgeStorage) *BridgeScopeResolver {
	return &BridgeScopeResolver{
		bridges:       make(map[string]*BridgeConnection),
		bridgeStorage: bridgeStorage,
		pathValidator: types.NewPathValidator(),
	}
}

// RegisterBridge registers a bridge connection for scope resolution
func (bsr *BridgeScopeResolver) RegisterBridge(meshName string, identity *identity.Identity, status string, targetAddress string) error {
	if meshName == "" {
		return fmt.Errorf("mesh name cannot be empty")
	}

	if identity == nil || identity.ID == "" {
		return fmt.Errorf("bridge identity cannot be nil or empty")
	}

	connection := &BridgeConnection{
		Identity:      identity,
		Status:        status,
		TargetAddress: targetAddress,
	}

	bsr.bridges[meshName] = connection

	// Also register with bridge storage
	bridgeStorageConnection := &storage.BridgeConnection{
		MeshName:      meshName,
		Identity:      identity,
		Status:        status,
		TargetAddress: targetAddress,
	}

	return bsr.bridgeStorage.RegisterBridge(meshName, bridgeStorageConnection)
}

// RemoveBridge removes a bridge connection
func (bsr *BridgeScopeResolver) RemoveBridge(meshName string) error {
	if _, exists := bsr.bridges[meshName]; !exists {
		return fmt.Errorf("no bridge connection to mesh '%s'", meshName)
	}

	delete(bsr.bridges, meshName)

	// Also remove from bridge storage
	return bsr.bridgeStorage.RemoveBridge(meshName)
}

// ResolveBridgePath resolves a path and determines if it's a bridge path
func (bsr *BridgeScopeResolver) ResolveBridgePath(pathString string) (*BridgeScope, error) {
	// Parse path string into components
	pathComponents := types.ParsePathString(pathString)

	// Parse and validate the path
	parsed := bsr.pathValidator.ParsePath(pathComponents)
	if parsed.Error != nil {
		return nil, parsed.Error
	}

	// Only handle bridge paths
	if parsed.Type != types.BridgePath {
		return nil, nil // Not a bridge path, let other resolvers handle it
	}

	meshName := parsed.MeshName

	// Check if we have a bridge to this mesh
	bridge, exists := bsr.bridges[meshName]
	if !exists {
		return nil, fmt.Errorf("no bridge connection to mesh '%s'", meshName)
	}

	// Note: connection status is intentionally NOT checked here. Resolving a
	// bridge path only requires that a bridge to the mesh is registered. Whether
	// the bridge is currently usable (connected) is a separate concern enforced
	// by BridgeScope.ValidateAccess at access time.

	// Create agent home path: world.agent.{bridge_identity}
	agentHome := fmt.Sprintf("world.agent.%s", bridge.Identity.ID)

	// Create the bridge scope
	scope := &BridgeScope{
		MeshName:     meshName,
		AgentHome:    agentHome,
		LocalPath:    pathString,
		BridgeID:     bridge.Identity.ID,
		Connection:   bridge,
		PathResolver: bsr.pathValidator,
	}

	return scope, nil
}

// ResolvePathComponents resolves path components and determines if it's a bridge path
func (bsr *BridgeScopeResolver) ResolvePathComponents(pathComponents []string) (*BridgeScope, error) {
	// Parse and validate the path
	parsed := bsr.pathValidator.ParsePath(pathComponents)
	if parsed.Error != nil {
		return nil, parsed.Error
	}

	// Only handle bridge paths
	if parsed.Type != types.BridgePath {
		return nil, nil // Not a bridge path, let other resolvers handle it
	}

	meshName := parsed.MeshName

	// Check if we have a bridge to this mesh
	bridge, exists := bsr.bridges[meshName]
	if !exists {
		return nil, fmt.Errorf("no bridge connection to mesh '%s'", meshName)
	}

	// Validate bridge status
	if bridge.Status != "connected" {
		return nil, fmt.Errorf("bridge to mesh '%s' is not connected (status: %s)", meshName, bridge.Status)
	}

	// Create agent home path: world.agent.{bridge_identity}
	agentHome := fmt.Sprintf("world.agent.%s", bridge.Identity.ID)

	// Create the bridge scope
	scope := &BridgeScope{
		MeshName:     meshName,
		AgentHome:    agentHome,
		LocalPath:    types.FormatPath(pathComponents),
		BridgeID:     bridge.Identity.ID,
		Connection:   bridge,
		PathResolver: bsr.pathValidator,
	}

	return scope, nil
}

// GetTargetPath returns the target path in the bridge mesh for a given bridge scope
func (bs *BridgeScope) GetTargetPath() ([]string, error) {
	// Parse the original local path
	pathComponents := types.ParsePathString(bs.LocalPath)

	// Convert to agent path
	return bs.PathResolver.ConvertBridgePathToAgentPath(pathComponents, bs.BridgeID)
}

// GetTargetPathForComponents returns the target path for given path components
func (bs *BridgeScope) GetTargetPathForComponents(pathComponents []string) ([]string, error) {
	return bs.PathResolver.ConvertBridgePathToAgentPath(pathComponents, bs.BridgeID)
}

// ValidateAccess validates that the bridge scope can be used for data access
func (bs *BridgeScope) ValidateAccess() error {
	if bs.Connection == nil {
		return fmt.Errorf("bridge scope has no connection information")
	}

	if bs.Connection.Status != "connected" {
		return fmt.Errorf("bridge to mesh '%s' is not connected (status: %s)", bs.MeshName, bs.Connection.Status)
	}

	if bs.Connection.Identity == nil || bs.Connection.Identity.ID == "" {
		return fmt.Errorf("bridge to mesh '%s' has invalid identity", bs.MeshName)
	}

	return nil
}

// GetMeshName returns the target mesh name
func (bs *BridgeScope) GetMeshName() string {
	return bs.MeshName
}

// GetBridgeIdentity returns the bridge identity
func (bs *BridgeScope) GetBridgeIdentity() *identity.Identity {
	if bs.Connection == nil {
		return nil
	}
	return bs.Connection.Identity
}

// GetTargetAddress returns the target mesh address
func (bs *BridgeScope) GetTargetAddress() string {
	if bs.Connection == nil {
		return ""
	}
	return bs.Connection.TargetAddress
}

// IsBridgePath checks if a path string is a bridge path with an existing bridge
func (bsr *BridgeScopeResolver) IsBridgePath(pathString string) bool {
	pathComponents := types.ParsePathString(pathString)
	return bsr.IsBridgePathComponents(pathComponents)
}

// IsBridgePathComponents checks if path components form a bridge path with an existing bridge
func (bsr *BridgeScopeResolver) IsBridgePathComponents(pathComponents []string) bool {
	// First check if it has the correct format
	if !bsr.pathValidator.IsBridgePath(pathComponents) {
		return false
	}

	// Then check if the bridge actually exists
	meshName, err := bsr.pathValidator.GetMeshNameFromBridgePath(pathComponents)
	if err != nil {
		return false
	}

	_, exists := bsr.bridges[meshName]
	return exists
}

// GetRegisteredBridges returns the list of registered bridge mesh names
func (bsr *BridgeScopeResolver) GetRegisteredBridges() []string {
	bridges := make([]string, 0, len(bsr.bridges))
	for meshName := range bsr.bridges {
		bridges = append(bridges, meshName)
	}
	return bridges
}

// GetBridgeStatus returns the status of a specific bridge
func (bsr *BridgeScopeResolver) GetBridgeStatus(meshName string) (string, error) {
	bridge, exists := bsr.bridges[meshName]
	if !exists {
		return "", fmt.Errorf("no bridge connection to mesh '%s'", meshName)
	}
	return bridge.Status, nil
}

// UpdateBridgeStatus updates the status of a bridge connection
func (bsr *BridgeScopeResolver) UpdateBridgeStatus(meshName string, newStatus string) error {
	bridge, exists := bsr.bridges[meshName]
	if !exists {
		return fmt.Errorf("no bridge connection to mesh '%s'", meshName)
	}

	bridge.Status = newStatus
	return nil
}

// String returns a string representation of the bridge scope
func (bs *BridgeScope) String() string {
	return fmt.Sprintf("BridgeScope{mesh=%s, path=%s, agent=%s}",
		bs.MeshName, bs.LocalPath, bs.AgentHome)
}
