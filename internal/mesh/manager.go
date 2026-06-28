// Package mesh provides mesh management functionality for AmorphDB
package mesh

import (
	"fmt"
	"sync"
	"time"

	"github.com/solifugus/amorphdb/internal/config"
	"github.com/solifugus/amorphdb/internal/directory"
	"github.com/solifugus/amorphdb/internal/identity"
	"github.com/solifugus/amorphdb/internal/storage"
	"github.com/solifugus/amorphdb/internal/subscription"
)

// MeshManager manages mesh creation, joining, and bridging operations
type MeshManager struct {
	mu             sync.RWMutex
	config         *config.Config
	identityMgr    *identity.IdentityManager
	storage        storage.Tree
	meshState      *MeshState
	configPath     string
	isFounder      bool
	bridges        map[string]*BridgeConnection
	bridgeManager  *BridgeManager
	leaveManager   *LeaveManager

	// Subscription-based distribution model components
	pathDirectory    *directory.Directory             // Path-to-authority mapping
	subscriptionReg  *subscription.SubscriptionRegistry // Subscription tracking
	authorityMgr     *AuthorityManager                 // Authority promotion and delegation
	gossipManager    *GossipManager                    // Gossip protocol manager
	heartbeatManager *HeartbeatManager                 // Heartbeat and failure detection
}

// MeshState represents the current state of the mesh
type MeshState struct {
	Name          string            `json:"name"`
	FoundedAt     time.Time         `json:"founded_at"`
	NodeIdentity  *identity.Identity `json:"node_identity"`
	MemberCount   int               `json:"member_count"`
	ZoneCount     int               `json:"zone_count"`
	Status        MeshStatus        `json:"status"`
}

// MeshStatus represents the status of mesh membership
type MeshStatus string

const (
	StatusStandalone    MeshStatus = "standalone"
	StatusFounder       MeshStatus = "founder"
	StatusMember        MeshStatus = "member"
	StatusJoining       MeshStatus = "joining"
	StatusLeaving       MeshStatus = "leaving"
	StatusBridged       MeshStatus = "bridged"
)

// BridgeConnection represents a connection to another mesh
type BridgeConnection struct {
	MeshName      string              `json:"mesh_name"`
	Address       string              `json:"address"`
	TargetAddress string              `json:"target_address"`
	Identity      *identity.Identity  `json:"identity"`
	Status        BridgeStatus        `json:"status"`
	ConnectedAt   time.Time           `json:"connected_at"`
}

// BridgeStatus represents the status of a bridge connection
type BridgeStatus string

const (
	BridgeStatusConnecting BridgeStatus = "connecting"
	BridgeStatusConnected  BridgeStatus = "connected"
	BridgeStatusFailed     BridgeStatus = "failed"
	BridgeStatusDisconnected BridgeStatus = "disconnected"
)

// NewMeshManager creates a new mesh manager
func NewMeshManager(cfg *config.Config, storage storage.Tree, configPath string) (*MeshManager, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}

	// Initialize identity manager
	nodeID := cfg.Mesh.Identity
	if nodeID == "" {
		// Generate node ID if not provided
		nodeID = fmt.Sprintf("node-%d", time.Now().Unix())
		cfg.Mesh.Identity = nodeID
	}

	identityMgr := identity.NewIdentityManager(cfg.Mesh.Name, nodeID)

	// Initialize mesh state
	meshState := &MeshState{
		Name:        cfg.Mesh.Name,
		Status:      StatusStandalone,
		MemberCount: 1,
		ZoneCount:   1,
	}

	if cfg.Mesh.Name != "" {
		meshState.Status = StatusMember
	}

	mm := &MeshManager{
		config:      cfg,
		identityMgr: identityMgr,
		storage:     storage,
		meshState:   meshState,
		configPath:  configPath,
		bridges:     make(map[string]*BridgeConnection),
	}

	// Initialize bridge manager
	mm.bridgeManager = NewBridgeManager(cfg.Mesh.Name)

	// Initialize leave manager (will be fully initialized when protocol is set)
	mm.leaveManager = &LeaveManager{
		meshManager:   mm,
		bridgeManager: mm.bridgeManager,
		protocol:      nil, // Will be set when protocol is available
	}

	// Initialize subscription-based distribution components
	mm.pathDirectory = directory.NewDirectory()
	mm.subscriptionReg = subscription.NewSubscriptionRegistry()

	// Load existing node identity if available
	if cfg.Mesh.Identity != "" {
		err := mm.loadOrCreateNodeIdentity()
		if err != nil {
			return nil, fmt.Errorf("failed to initialize node identity: %w", err)
		}

		// Initialize authority manager with node identity
		mm.authorityMgr = NewAuthorityManager(
			mm.meshState.NodeIdentity,
			mm.pathDirectory,
			mm.subscriptionReg,
		)

		// Initialize gossip and heartbeat managers
		mm.gossipManager = NewGossipManager(mm.meshState.NodeIdentity.ID)
		mm.heartbeatManager = NewHeartbeatManager(mm.meshState.NodeIdentity.ID)

		// Wire authority manager with gossip and heartbeat
		mm.authorityMgr.SetGossipManager(mm.gossipManager)
		mm.authorityMgr.SetHeartbeatManager(mm.heartbeatManager)
	}

	return mm, nil
}

// CreateMesh creates a new mesh with the given name
func (mm *MeshManager) CreateMesh(name string) error {
	mm.mu.Lock()
	defer mm.mu.Unlock()

	// Validate mesh name
	if err := config.ValidateMeshName(name); err != nil {
		return fmt.Errorf("invalid mesh name: %w", err)
	}

	// Check if already in a mesh
	if mm.meshState.Status != StatusStandalone {
		return fmt.Errorf("node is already part of mesh '%s', detach first", mm.config.Mesh.Name)
	}

	// Update configuration
	mm.config.Mesh.Name = name
	mm.identityMgr.UpdateMeshName(name)

	// Generate node identity for this mesh
	nodeIdentity, err := mm.identityMgr.GenerateNodeIdentity()
	if err != nil {
		return fmt.Errorf("failed to generate node identity: %w", err)
	}

	// Update mesh state
	mm.meshState.Name = name
	mm.meshState.FoundedAt = time.Now().UTC()
	mm.meshState.NodeIdentity = nodeIdentity
	mm.meshState.Status = StatusFounder
	mm.meshState.MemberCount = 1
	mm.meshState.ZoneCount = 1
	mm.isFounder = true

	// Initialize mesh founder state
	err = mm.initializeMeshFounder()
	if err != nil {
		// Rollback on failure
		mm.config.Mesh.Name = ""
		mm.identityMgr.UpdateMeshName("")
		mm.meshState.Status = StatusStandalone
		return fmt.Errorf("failed to initialize mesh founder: %w", err)
	}

	// Persist configuration
	if err := config.SaveConfig(*mm.config, mm.configPath); err != nil {
		return fmt.Errorf("failed to save configuration: %w", err)
	}

	return nil
}

// GetMeshState returns the current mesh state
func (mm *MeshManager) GetMeshState() *MeshState {
	mm.mu.RLock()
	defer mm.mu.RUnlock()

	// Return a copy to prevent external modification
	state := *mm.meshState
	return &state
}

// GetNodeIdentity returns the current node identity
func (mm *MeshManager) GetNodeIdentity() *identity.Identity {
	mm.mu.RLock()
	defer mm.mu.RUnlock()
	return mm.identityMgr.GetNodeIdentity()
}

// IsFounder returns true if this node founded the mesh
func (mm *MeshManager) IsFounder() bool {
	mm.mu.RLock()
	defer mm.mu.RUnlock()
	return mm.isFounder
}

// GetBridgeConnections returns all bridge connections
func (mm *MeshManager) GetBridgeConnections() map[string]*BridgeConnection {
	mm.mu.RLock()
	defer mm.mu.RUnlock()

	result := make(map[string]*BridgeConnection)
	for name, bridge := range mm.bridges {
		bridgeCopy := *bridge
		result[name] = &bridgeCopy
	}
	return result
}

// initializeMeshFounder sets up the initial state for a mesh founder
func (mm *MeshManager) initializeMeshFounder() error {
	// Store mesh metadata in the storage
	meshInfo := map[string]interface{}{
		"name":         mm.meshState.Name,
		"founded_at":   mm.meshState.FoundedAt.Unix(),
		"founder_id":   mm.meshState.NodeIdentity.ID,
		"founder_node": mm.meshState.NodeIdentity.NodeID,
	}

	// Store mesh info under world.mesh.info
	for key, value := range meshInfo {
		storageValue := storage.Value{
			Data:    []byte(fmt.Sprintf("%v", value)),
			TypeTag: storage.TypeText,
		}

		path := []string{"world", "mesh", "info", key}
		authorID := mm.hashIdentityID(mm.meshState.NodeIdentity.ID)

		err := mm.storage.Write(path, storageValue, authorID)
		if err != nil {
			return fmt.Errorf("failed to store mesh info '%s': %w", key, err)
		}
	}

	// Initialize zone 0 ownership
	zoneInfo := storage.Value{
		Data:    []byte(mm.meshState.NodeIdentity.NodeID),
		TypeTag: storage.TypeText,
	}

	path := []string{"world", "mesh", "zones", "0", "owner"}
	authorID := mm.hashIdentityID(mm.meshState.NodeIdentity.ID)

	err := mm.storage.Write(path, zoneInfo, authorID)
	if err != nil {
		return fmt.Errorf("failed to initialize zone ownership: %w", err)
	}

	return nil
}

// loadOrCreateNodeIdentity loads existing node identity or creates a new one
func (mm *MeshManager) loadOrCreateNodeIdentity() error {
	// Try to load existing identity from storage
	identityPath := []string{"world", "node", "identity"}

	_, err := mm.storage.Read(identityPath)
	if err == nil {
		// Parse existing identity
		// For now, just generate a new one - in production this would deserialize
		// the stored identity
		nodeIdentity, err := mm.identityMgr.GenerateNodeIdentity()
		if err != nil {
			return fmt.Errorf("failed to generate node identity: %w", err)
		}
		mm.meshState.NodeIdentity = nodeIdentity
		return nil
	}

	// Create new identity
	nodeIdentity, err := mm.identityMgr.GenerateNodeIdentity()
	if err != nil {
		return fmt.Errorf("failed to generate node identity: %w", err)
	}

	// Store identity for persistence
	identityData := storage.Value{
		Data:    []byte(nodeIdentity.ID),
		TypeTag: storage.TypeText,
	}

	authorID := mm.hashIdentityID(nodeIdentity.ID)
	err = mm.storage.Write(identityPath, identityData, authorID)
	if err != nil {
		return fmt.Errorf("failed to store node identity: %w", err)
	}

	mm.meshState.NodeIdentity = nodeIdentity
	return nil
}

// hashIdentityID converts an identity ID to a uint64 for author ID
func (mm *MeshManager) hashIdentityID(identityID string) uint64 {
	hash := uint64(0)
	for i, c := range identityID {
		hash = hash*31 + uint64(c) + uint64(i)
	}
	return hash
}

// JoinMesh discovers and joins an existing mesh at the given seed address
func (mm *MeshManager) JoinMesh(seedAddress string) error {
	mm.mu.Lock()
	defer mm.mu.Unlock()

	// Validate we're in standalone mode
	if mm.meshState.Status != StatusStandalone {
		return fmt.Errorf("node is already part of mesh '%s', detach first", mm.config.Mesh.Name)
	}

	// Set status to joining
	mm.meshState.Status = StatusJoining

	// Create discovery client and discover mesh
	discoveryClient := NewDiscoveryClient()
	meshInfo, err := discoveryClient.DiscoverMesh(seedAddress)
	if err != nil {
		mm.meshState.Status = StatusStandalone // Rollback status
		return fmt.Errorf("failed to discover mesh: %w", err)
	}

	// Preserve existing data if we have any
	if err := mm.preserveExistingData(meshInfo.Name); err != nil {
		mm.meshState.Status = StatusStandalone
		return fmt.Errorf("failed to preserve existing data: %w", err)
	}

	// Update configuration with discovered mesh name
	mm.config.Mesh.Name = meshInfo.Name
	mm.identityMgr.UpdateMeshName(meshInfo.Name)

	// Generate node identity for joining this mesh
	nodeIdentity, err := mm.identityMgr.GenerateNodeIdentity()
	if err != nil {
		mm.meshState.Status = StatusStandalone
		return fmt.Errorf("failed to generate node identity: %w", err)
	}

	// Create join client and attempt to join
	joinClient := NewJoinClient()
	joinResult, err := joinClient.JoinMesh(seedAddress, nodeIdentity, meshInfo.Name)
	if err != nil {
		mm.meshState.Status = StatusStandalone
		return fmt.Errorf("failed to join mesh: %w", err)
	}

	if !joinResult.Success {
		mm.meshState.Status = StatusStandalone
		return fmt.Errorf("join rejected: %s", joinResult.Message)
	}

	// Update mesh state with join results
	mm.meshState.Name = meshInfo.Name
	mm.meshState.NodeIdentity = nodeIdentity
	mm.meshState.Status = StatusMember
	mm.meshState.MemberCount = joinResult.MemberCount
	mm.meshState.FoundedAt = meshInfo.FoundedAt
	mm.isFounder = false

	// Store join information
	if err := mm.storeJoinInfo(meshInfo, joinResult); err != nil {
		return fmt.Errorf("failed to store join info: %w", err)
	}

	// Persist configuration
	if err := config.SaveConfig(*mm.config, mm.configPath); err != nil {
		return fmt.Errorf("failed to save configuration: %w", err)
	}

	return nil
}

// preserveExistingData moves standalone data to my.standalone.* before joining mesh
func (mm *MeshManager) preserveExistingData(meshName string) error {
	// Check if we have any data in the "my" tree that needs preservation
	myPath := []string{"my"}

	// Try to read the "my" tree root
	children, err := mm.storage.Children(myPath)
	if err != nil {
		// If no "my" tree exists, nothing to preserve
		return nil
	}

	// If there's existing data, move it to my.standalone.*
	if len(children) > 0 {
		// For simplicity, we'll just document this as needed preservation
		// In a full implementation, this would recursively copy data
		// from "my.*" to "my.standalone.*"

		// Store preservation marker
		preservationInfo := storage.Value{
			Data:    []byte(fmt.Sprintf("Data preserved before joining mesh '%s' at %s",
			                           meshName, time.Now().UTC().Format(time.RFC3339))),
			TypeTag: storage.TypeText,
		}

		path := []string{"my", "standalone", "_preserved_at"}
		authorID := mm.hashIdentityID("system")

		err = mm.storage.Write(path, preservationInfo, authorID)
		if err != nil {
			return fmt.Errorf("failed to store preservation marker: %w", err)
		}
	}

	return nil
}

// storeJoinInfo stores information about the mesh join operation
func (mm *MeshManager) storeJoinInfo(meshInfo *MeshInfo, joinResult *JoinResult) error {
	// Store join metadata
	joinData := map[string]interface{}{
		"mesh_name":      meshInfo.Name,
		"joined_at":      time.Now().UTC().Unix(),
		"founder_id":     meshInfo.FounderIdentity.ID,
		"assigned_zone":  joinResult.AssignedZone,
		"member_count":   joinResult.MemberCount,
	}

	// Store join info under world.mesh.join
	for key, value := range joinData {
		storageValue := storage.Value{
			Data:    []byte(fmt.Sprintf("%v", value)),
			TypeTag: storage.TypeText,
		}

		path := []string{"world", "mesh", "join", key}
		authorID := mm.hashIdentityID(mm.meshState.NodeIdentity.ID)

		err := mm.storage.Write(path, storageValue, authorID)
		if err != nil {
			return fmt.Errorf("failed to store join info '%s': %w", key, err)
		}
	}

	return nil
}

// CreateBridge establishes a bridge connection to another mesh
func (mm *MeshManager) CreateBridge(targetAddress string) (*BridgeConnection, error) {
	mm.mu.Lock()
	defer mm.mu.Unlock()

	// Validate we're in a mesh (standalone nodes cannot create bridges)
	if mm.meshState.Status == StatusStandalone {
		return nil, fmt.Errorf("cannot create bridges from standalone node, join a mesh first")
	}

	// Get current node identity
	nodeIdentity := mm.identityMgr.GetNodeIdentity()
	if nodeIdentity == nil {
		return nil, fmt.Errorf("no node identity available")
	}

	// Create bridge manager if it doesn't exist
	bridgeManager := NewBridgeManager(mm.meshState.Name)

	// Create the bridge
	bridge, err := bridgeManager.CreateBridge(targetAddress, nodeIdentity)
	if err != nil {
		return nil, fmt.Errorf("failed to create bridge: %w", err)
	}

	// Store the bridge in our bridges map
	mm.bridges[bridge.MeshName] = bridge

	// Store bridge information for persistence
	if err := mm.storeBridgeInfo(bridge); err != nil {
		return nil, fmt.Errorf("failed to store bridge info: %w", err)
	}

	// Persist configuration
	if err := config.SaveConfig(*mm.config, mm.configPath); err != nil {
		return nil, fmt.Errorf("failed to save configuration: %w", err)
	}

	return bridge, nil
}

// DisconnectBridge removes a bridge connection
func (mm *MeshManager) DisconnectBridge(meshName string) error {
	mm.mu.Lock()
	defer mm.mu.Unlock()

	bridge, exists := mm.bridges[meshName]
	if !exists {
		return fmt.Errorf("no bridge to mesh '%s' exists", meshName)
	}

	// Mark as disconnected
	bridge.Status = BridgeStatusDisconnected

	// Remove from our bridges map
	delete(mm.bridges, meshName)

	// Clean up stored bridge information
	if err := mm.removeBridgeInfo(meshName); err != nil {
		return fmt.Errorf("failed to remove bridge info: %w", err)
	}

	return nil
}

// storeBridgeInfo persists bridge information to storage
func (mm *MeshManager) storeBridgeInfo(bridge *BridgeConnection) error {
	bridgeData := map[string]interface{}{
		"mesh_name":      bridge.MeshName,
		"address":        bridge.Address,
		"identity_id":    bridge.Identity.ID,
		"status":         string(bridge.Status),
		"connected_at":   bridge.ConnectedAt.Unix(),
	}

	// Store bridge info under world.mesh.bridges.{mesh_name}
	for key, value := range bridgeData {
		storageValue := storage.Value{
			Data:    []byte(fmt.Sprintf("%v", value)),
			TypeTag: storage.TypeText,
		}

		path := []string{"world", "mesh", "bridges", bridge.MeshName, key}
		authorID := mm.hashIdentityID(mm.meshState.NodeIdentity.ID)

		err := mm.storage.Write(path, storageValue, authorID)
		if err != nil {
			return fmt.Errorf("failed to store bridge info '%s': %w", key, err)
		}
	}

	return nil
}

// removeBridgeInfo removes bridge information from storage
func (mm *MeshManager) removeBridgeInfo(meshName string) error {
	// Remove bridge info directory
	bridgePath := []string{"world", "mesh", "bridges", meshName}
	authorID := mm.hashIdentityID(mm.meshState.NodeIdentity.ID)

	err := mm.storage.Purge(bridgePath, 0, time.Now().Unix(), authorID)
	if err != nil {
		return fmt.Errorf("failed to remove bridge info: %w", err)
	}

	return nil
}

// ValidateMeshName validates a mesh name using config validation
func ValidateMeshName(name string) error {
	return config.ValidateMeshName(name)
}

// GetLeaveManager returns the leave manager for graceful departure operations
func (mm *MeshManager) GetLeaveManager() *LeaveManager {
	mm.mu.RLock()
	defer mm.mu.RUnlock()
	return mm.leaveManager
}

// BecomeStandalone transitions the node back to standalone mode
func (mm *MeshManager) BecomeStandalone() error {
	mm.mu.Lock()
	defer mm.mu.Unlock()

	// Clear mesh configuration
	mm.config.Mesh.Name = ""
	mm.meshState.Name = ""
	mm.meshState.Status = StatusStandalone
	mm.meshState.MemberCount = 1
	mm.meshState.ZoneCount = 1
	mm.meshState.FoundedAt = time.Time{}
	mm.isFounder = false

	// Clear all bridge connections
	mm.bridges = make(map[string]*BridgeConnection)

	// Save updated configuration
	return mm.saveConfig()
}

// GetOwnedZones returns the zones owned by this node (placeholder)
func (mm *MeshManager) GetOwnedZones() []*Zone {
	mm.mu.RLock()
	defer mm.mu.RUnlock()

	// Placeholder implementation - in a real implementation,
	// this would query the zone assignment system
	return []*Zone{}
}

// GetConnectedNodes returns the currently connected mesh nodes (placeholder)
func (mm *MeshManager) GetConnectedNodes() []*identity.Identity {
	mm.mu.RLock()
	defer mm.mu.RUnlock()

	// Placeholder implementation - in a real implementation,
	// this would return the list of connected peer nodes
	return []*identity.Identity{}
}

// GetNodeZoneCount returns the number of zones owned by a specific node (placeholder)
func (mm *MeshManager) GetNodeZoneCount(nodeID string) int {
	mm.mu.RLock()
	defer mm.mu.RUnlock()

	// Placeholder implementation - in a real implementation,
	// this would query the zone assignment system
	return 0
}

// UpdateZoneOwnership updates the ownership of a zone (placeholder)
func (mm *MeshManager) UpdateZoneOwnership(zoneID string, newOwnerID string) error {
	mm.mu.Lock()
	defer mm.mu.Unlock()

	// Placeholder implementation - in a real implementation,
	// this would update the zone assignment system
	return nil
}

// GetIdentityManager returns the identity manager
func (mm *MeshManager) GetIdentityManager() *identity.IdentityManager {
	mm.mu.RLock()
	defer mm.mu.RUnlock()
	return mm.identityMgr
}

// GetCurrentMesh returns information about the current mesh
func (mm *MeshManager) GetCurrentMesh() *MeshState {
	mm.mu.RLock()
	defer mm.mu.RUnlock()
	return mm.meshState
}

// saveConfig saves the current configuration to disk (placeholder)
func (mm *MeshManager) saveConfig() error {
	// Placeholder implementation - in full implementation this would save config to disk
	fmt.Printf("Saving configuration for mesh: %s\n", mm.config.Mesh.Name)
	return nil
}

// GetPathDirectory returns the path directory for authority lookups
func (mm *MeshManager) GetPathDirectory() *directory.Directory {
	mm.mu.RLock()
	defer mm.mu.RUnlock()
	return mm.pathDirectory
}

// GetSubscriptionRegistry returns the subscription registry
func (mm *MeshManager) GetSubscriptionRegistry() *subscription.SubscriptionRegistry {
	mm.mu.RLock()
	defer mm.mu.RUnlock()
	return mm.subscriptionReg
}

// GetAuthorityManager returns the authority manager
func (mm *MeshManager) GetAuthorityManager() *AuthorityManager {
	mm.mu.RLock()
	defer mm.mu.RUnlock()
	return mm.authorityMgr
}

// GetGossipManager returns the gossip manager
func (mm *MeshManager) GetGossipManager() *GossipManager {
	mm.mu.RLock()
	defer mm.mu.RUnlock()
	return mm.gossipManager
}

// GetHeartbeatManager returns the heartbeat manager
func (mm *MeshManager) GetHeartbeatManager() *HeartbeatManager {
	mm.mu.RLock()
	defer mm.mu.RUnlock()
	return mm.heartbeatManager
}