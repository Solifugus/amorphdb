// Package identity manages node and mobile agent identities in AmorphDB mesh networks
package identity

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"github.com/solifugus/amorphdb/internal/types"
)

// Identity represents an agent identity in the mesh network
type Identity struct {
	ID       string           `json:"id"`        // Unique identifier for this identity
	Type     types.AgentType  `json:"type"`      // Type of agent (node or mobile)
	MeshName string           `json:"mesh_name"` // Which mesh this identity belongs to
	Created  time.Time        `json:"created"`   // When this identity was created
	NodeID   string           `json:"node_id"`   // Physical node where identity was created
}

// IdentityManager manages identities for a node in the mesh network
type IdentityManager struct {
	mu               sync.RWMutex
	nodeIdentity     *Identity                 // This node's identity
	bridgeIdentities map[string]*Identity      // Bridge identities by mesh name
	meshName         string                    // Primary mesh name (empty = standalone)
	nodeID           string                    // This node's unique identifier
}

// NewIdentityManager creates a new identity manager
func NewIdentityManager(meshName, nodeID string) *IdentityManager {
	return &IdentityManager{
		bridgeIdentities: make(map[string]*Identity),
		meshName:         meshName,
		nodeID:           nodeID,
	}
}

// GenerateNodeIdentity generates a new node identity for the current mesh
func (im *IdentityManager) GenerateNodeIdentity() (*Identity, error) {
	im.mu.Lock()
	defer im.mu.Unlock()

	id, err := generateIdentityID()
	if err != nil {
		return nil, fmt.Errorf("failed to generate identity ID: %w", err)
	}

	identity := &Identity{
		ID:       id,
		Type:     types.NodeAgent,
		MeshName: im.meshName,
		Created:  time.Now().UTC(),
		NodeID:   im.nodeID,
	}

	im.nodeIdentity = identity
	return identity, nil
}

// GetNodeIdentity returns the current node's identity
func (im *IdentityManager) GetNodeIdentity() *Identity {
	im.mu.RLock()
	defer im.mu.RUnlock()
	return im.nodeIdentity
}

// SetNodeIdentity sets the node identity (used when loading from storage)
func (im *IdentityManager) SetNodeIdentity(identity *Identity) error {
	if identity.Type != types.NodeAgent {
		return fmt.Errorf("node identity must be of type NodeAgent, got %s", identity.Type)
	}

	im.mu.Lock()
	defer im.mu.Unlock()
	im.nodeIdentity = identity
	return nil
}

// CreateBridgeIdentity creates a mobile agent identity for bridging to another mesh
func (im *IdentityManager) CreateBridgeIdentity(targetMeshName string) (*Identity, error) {
	im.mu.Lock()
	defer im.mu.Unlock()

	// Check if we already have an identity for this mesh
	if existing, exists := im.bridgeIdentities[targetMeshName]; exists {
		return existing, nil
	}

	id, err := generateIdentityID()
	if err != nil {
		return nil, fmt.Errorf("failed to generate bridge identity ID: %w", err)
	}

	identity := &Identity{
		ID:       id,
		Type:     types.MobileAgent,
		MeshName: targetMeshName,
		Created:  time.Now().UTC(),
		NodeID:   im.nodeID,
	}

	im.bridgeIdentities[targetMeshName] = identity
	return identity, nil
}

// GetBridgeIdentity returns the bridge identity for the specified mesh
func (im *IdentityManager) GetBridgeIdentity(meshName string) *Identity {
	im.mu.RLock()
	defer im.mu.RUnlock()
	return im.bridgeIdentities[meshName]
}

// RemoveBridgeIdentity removes the bridge identity for the specified mesh
func (im *IdentityManager) RemoveBridgeIdentity(meshName string) {
	im.mu.Lock()
	defer im.mu.Unlock()
	delete(im.bridgeIdentities, meshName)
}

// ListBridgeIdentities returns all bridge identities
func (im *IdentityManager) ListBridgeIdentities() map[string]*Identity {
	im.mu.RLock()
	defer im.mu.RUnlock()

	result := make(map[string]*Identity)
	for meshName, identity := range im.bridgeIdentities {
		result[meshName] = identity
	}
	return result
}

// UpdateMeshName updates the primary mesh name and node identity
func (im *IdentityManager) UpdateMeshName(newMeshName string) error {
	im.mu.Lock()
	defer im.mu.Unlock()

	im.meshName = newMeshName

	// Update node identity mesh name if it exists
	if im.nodeIdentity != nil {
		im.nodeIdentity.MeshName = newMeshName
	}

	return nil
}

// GetAgentHomePath returns the storage path for an agent's home directory
func (im *IdentityManager) GetAgentHomePath(identity *Identity) string {
	switch identity.Type {
	case types.NodeAgent:
		// Node agents store data locally under their node ID
		return fmt.Sprintf("world.node.%s", identity.NodeID)
	case types.MobileAgent:
		// Mobile agents store data in the mesh under their identity
		return fmt.Sprintf("world.agent.%s", identity.ID)
	default:
		return ""
	}
}

// ValidateIdentity validates that an identity is well-formed
func ValidateIdentity(identity *Identity) error {
	if identity == nil {
		return fmt.Errorf("identity cannot be nil")
	}

	if identity.ID == "" {
		return fmt.Errorf("identity ID cannot be empty")
	}

	if !identity.Type.IsValid() {
		return fmt.Errorf("invalid agent type: %s", identity.Type)
	}

	if identity.MeshName == "" {
		return fmt.Errorf("mesh name cannot be empty")
	}

	if identity.Created.IsZero() {
		return fmt.Errorf("created timestamp cannot be zero")
	}

	return nil
}

// generateIdentityID generates a cryptographically secure random identity ID
func generateIdentityID() (string, error) {
	// Generate 16 random bytes (128 bits)
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}

	// Convert to hex string
	return hex.EncodeToString(bytes), nil
}

// IsConflict checks if two identities would conflict in the same mesh
func IsConflict(id1, id2 *Identity) bool {
	if id1 == nil || id2 == nil {
		return false
	}

	// Same ID in same mesh is a conflict
	if id1.ID == id2.ID && id1.MeshName == id2.MeshName {
		return true
	}

	// Node agents with same node ID in same mesh conflict
	if id1.Type == types.NodeAgent && id2.Type == types.NodeAgent &&
		id1.NodeID == id2.NodeID && id1.MeshName == id2.MeshName {
		return true
	}

	return false
}

// RemoveIdentity removes an identity from the manager
func (im *IdentityManager) RemoveIdentity(identityID string) error {
	im.mu.Lock()
	defer im.mu.Unlock()

	// Check if it's the node identity
	if im.nodeIdentity != nil && im.nodeIdentity.ID == identityID {
		im.nodeIdentity = nil
		return nil
	}

	// Check bridge identities
	for meshName, identity := range im.bridgeIdentities {
		if identity.ID == identityID {
			delete(im.bridgeIdentities, meshName)
			return nil
		}
	}

	return fmt.Errorf("identity %s not found", identityID)
}