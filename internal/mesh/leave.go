// Package mesh provides graceful departure functionality
package mesh

import (
	"fmt"
	"time"

	"github.com/solifugus/amorphdb/internal/identity"
)

// LeaveManager handles graceful departure from meshes and bridges
type LeaveManager struct {
	meshManager    *MeshManager
	bridgeManager  *BridgeManager
	protocol       interface{} // Placeholder for protocol handler
}

// NewLeaveManager creates a new leave manager
func NewLeaveManager(meshManager *MeshManager, bridgeManager *BridgeManager, protocol interface{}) *LeaveManager {
	return &LeaveManager{
		meshManager:   meshManager,
		bridgeManager: bridgeManager,
		protocol:      protocol,
	}
}

// DetachFromMesh handles detach operations for both primary mesh and bridges
func (lm *LeaveManager) DetachFromMesh(meshName string) (*DetachResult, error) {
	if meshName == "" {
		// Detach from primary mesh
		return lm.leavePrimaryMesh()
	}

	// Detach from specific bridge
	return lm.disconnectBridge(meshName)
}

// DetachResult contains information about the detach operation
type DetachResult struct {
	DetachType      string
	MeshName        string
	PreviousRole    string
	BridgeIdentity  string
	ZonesMigrated   int
	DataPreserved   bool
	DisconnectedAt  int64
}

// leavePrimaryMesh handles leaving the primary mesh and becoming standalone
func (lm *LeaveManager) leavePrimaryMesh() (*DetachResult, error) {
	currentMesh := lm.meshManager.GetCurrentMesh()
	if currentMesh == nil {
		return nil, fmt.Errorf("not currently part of any mesh")
	}

	result := &DetachResult{
		DetachType:     "primary_mesh",
		MeshName:       currentMesh.Name,
		DisconnectedAt: time.Now().Unix(),
	}

	// Determine role before leaving
	if lm.meshManager.IsFounder() {
		result.PreviousRole = "founder"
	} else {
		result.PreviousRole = "member"
	}

	// Step 1: Announce departure intention via gossip
	if err := lm.announceDeparture(); err != nil {
		return nil, fmt.Errorf("failed to announce departure: %w", err)
	}

	// Step 2: Migrate owned zones to adjacent nodes
	zonesMigrated, err := lm.migrateOwnedZones()
	if err != nil {
		return nil, fmt.Errorf("failed to migrate zones: %w", err)
	}
	result.ZonesMigrated = zonesMigrated

	// Step 3: Preserve existing data under 'my.standalone.*'
	if err := lm.preserveDataAsStandalone(); err != nil {
		return nil, fmt.Errorf("failed to preserve data: %w", err)
	}
	result.DataPreserved = true

	// Step 4: Clear mesh state and become standalone
	if err := lm.meshManager.BecomeStandalone(); err != nil {
		return nil, fmt.Errorf("failed to become standalone: %w", err)
	}

	// Step 5: Close mesh connections
	if err := lm.closeMeshConnections(); err != nil {
		// Log warning but don't fail - we're already detached
		fmt.Printf("Warning: Failed to close some mesh connections: %v\n", err)
	}

	return result, nil
}

// disconnectBridge handles disconnection from a specific bridge
func (lm *LeaveManager) disconnectBridge(meshName string) (*DetachResult, error) {
	bridge, exists := lm.bridgeManager.GetBridge(meshName)
	if !exists {
		return nil, fmt.Errorf("no bridge connection to mesh '%s'", meshName)
	}

	result := &DetachResult{
		DetachType:      "bridge",
		MeshName:        meshName,
		BridgeIdentity:  bridge.Identity.ID,
		DisconnectedAt:  time.Now().Unix(),
		ZonesMigrated:   0, // Bridges don't own zones
		DataPreserved:   true, // Bridge data remains accessible until identity expires
	}

	// Step 1: Notify target mesh of bridge closure
	if err := lm.notifyBridgeClosure(bridge); err != nil {
		// Log warning but continue - connection may already be broken
		fmt.Printf("Warning: Failed to notify target mesh of bridge closure: %v\n", err)
	}

	// Step 2: Remove bridge from local state
	if err := lm.bridgeManager.RemoveBridge(meshName); err != nil {
		return nil, fmt.Errorf("failed to remove bridge: %w", err)
	}

	// Step 3: Clear bridge identity (it will eventually expire in target mesh)
	if err := lm.clearBridgeIdentity(bridge.Identity); err != nil {
		// Log warning but don't fail
		fmt.Printf("Warning: Failed to clear bridge identity locally: %v\n", err)
	}

	return result, nil
}

// announceDeparture announces departure intention via gossip
func (lm *LeaveManager) announceDeparture() error {
	// Create departure announcement message
	nodeIdentity := lm.meshManager.GetNodeIdentity()
	if nodeIdentity == nil {
		return fmt.Errorf("no node identity available")
	}

	// Placeholder implementation - in full implementation this would broadcast via gossip
	fmt.Printf("Announcing departure of node %s\n", nodeIdentity.ID)
	return nil
}

// migrateOwnedZones migrates zones owned by this node to adjacent nodes
func (lm *LeaveManager) migrateOwnedZones() (int, error) {
	ownedZones := lm.meshManager.GetOwnedZones()
	if len(ownedZones) == 0 {
		return 0, nil // No zones to migrate
	}

	migratedCount := 0
	for _, zone := range ownedZones {
		// Find best adjacent node for this zone
		targetNode, err := lm.findBestZoneTarget(zone)
		if err != nil {
			return migratedCount, fmt.Errorf("failed to find target for zone %s: %w", zone.ID, err)
		}

		// Transfer zone ownership
		if err := lm.transferZoneOwnership(zone, targetNode); err != nil {
			return migratedCount, fmt.Errorf("failed to transfer zone %s: %w", zone.ID, err)
		}

		migratedCount++
	}

	return migratedCount, nil
}

// findBestZoneTarget finds the best node to take over a zone
func (lm *LeaveManager) findBestZoneTarget(zone *Zone) (*identity.Identity, error) {
	// Simple strategy: find node with lowest zone count
	connectedNodes := lm.meshManager.GetConnectedNodes()
	if len(connectedNodes) == 0 {
		return nil, fmt.Errorf("no connected nodes available for zone transfer")
	}

	var bestNode *identity.Identity
	minZoneCount := int(^uint(0) >> 1) // Max int

	for _, node := range connectedNodes {
		zoneCount := lm.meshManager.GetNodeZoneCount(node.ID)
		if zoneCount < minZoneCount {
			minZoneCount = zoneCount
			bestNode = node
		}
	}

	if bestNode == nil {
		return nil, fmt.Errorf("no suitable node found for zone transfer")
	}

	return bestNode, nil
}

// transferZoneOwnership transfers a zone to another node
func (lm *LeaveManager) transferZoneOwnership(zone *Zone, targetNode *identity.Identity) error {
	// Placeholder implementation - in full implementation this would send zone transfer protocol messages
	fmt.Printf("Transferring zone %s to node %s\n", zone.ID, targetNode.ID)

	// Update local zone registry
	return lm.meshManager.UpdateZoneOwnership(zone.ID, targetNode.ID)
}

// preserveDataAsStandalone preserves current mesh data under standalone scope
func (lm *LeaveManager) preserveDataAsStandalone() error {
	// This would involve moving mesh data to local storage
	// and updating path resolution to work in standalone mode
	// For now, we'll implement a placeholder that logs the operation

	currentMesh := lm.meshManager.GetCurrentMesh()
	if currentMesh == nil {
		return nil // Already standalone
	}

	// In a full implementation, this would:
	// 1. Copy mesh data to local storage under 'my.standalone.*'
	// 2. Update path resolution to work without mesh context
	// 3. Clear mesh-specific caches and references

	fmt.Printf("Preserving mesh data from '%s' under 'my.standalone.*'\n", currentMesh.Name)
	return nil
}

// notifyBridgeClosure notifies the target mesh that the bridge is closing
func (lm *LeaveManager) notifyBridgeClosure(bridge *BridgeConnection) error {
	// Placeholder implementation - in full implementation this would send bridge closure protocol messages
	fmt.Printf("Notifying target mesh of bridge closure: %s -> %s\n", bridge.Identity.ID, bridge.TargetAddress)
	return nil
}

// clearBridgeIdentity clears local bridge identity information
func (lm *LeaveManager) clearBridgeIdentity(bridgeIdentity *identity.Identity) error {
	// Remove from local identity store
	return lm.meshManager.GetIdentityManager().RemoveIdentity(bridgeIdentity.ID)
}

// closeMeshConnections closes all mesh-related network connections
func (lm *LeaveManager) closeMeshConnections() error {
	// This would close active mesh connections
	// For now, we'll implement a placeholder
	fmt.Println("Closing mesh network connections...")
	return nil
}

// Zone represents a mesh zone (placeholder for actual zone structure)
type Zone struct {
	ID    string
	Owner string
}