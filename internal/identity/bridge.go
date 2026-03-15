// Package identity provides bridge identity management for AmorphDB mesh networks
package identity

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"github.com/solifugus/amorphdb/internal/types"
)

// BridgeIdentityManager manages identities for bridge connections across meshes
type BridgeIdentityManager struct {
	mu                sync.RWMutex
	primaryIdentity   *Identity                    // Primary mesh identity
	bridgeIdentities  map[string]*Identity         // Bridge identities by target mesh name
	identityKeys      map[string]*DerivedKey       // Cryptographic keys per mesh
}

// DerivedKey represents cryptographic material for a specific mesh
type DerivedKey struct {
	MeshName    string    // Target mesh name
	KeyMaterial []byte    // Derived key material
	CreatedAt   time.Time // When this key was derived
}

// NewBridgeIdentityManager creates a new bridge identity manager
func NewBridgeIdentityManager(primaryIdentity *Identity) *BridgeIdentityManager {
	return &BridgeIdentityManager{
		primaryIdentity:  primaryIdentity,
		bridgeIdentities: make(map[string]*Identity),
		identityKeys:     make(map[string]*DerivedKey),
	}
}

// CreateBridgeIdentity creates a new identity for bridging to a target mesh
func (bim *BridgeIdentityManager) CreateBridgeIdentity(targetMeshName string) (*Identity, error) {
	bim.mu.Lock()
	defer bim.mu.Unlock()

	// Check if we already have a bridge identity for this mesh
	if existing, exists := bim.bridgeIdentities[targetMeshName]; exists {
		return existing, nil
	}

	// Generate bridge-specific identity
	bridgeID, err := bim.generateBridgeID(targetMeshName)
	if err != nil {
		return nil, fmt.Errorf("failed to generate bridge ID: %w", err)
	}

	// Create bridge identity as a mobile agent
	bridgeIdentity := &Identity{
		ID:       bridgeID,
		Type:     types.MobileAgent, // Bridges are always mobile agents
		MeshName: targetMeshName,
		Created:  time.Now().UTC(),
		NodeID:   bim.primaryIdentity.NodeID, // Same physical node
	}

	// Derive cryptographic key for this bridge
	derivedKey, err := bim.deriveKeyForMesh(targetMeshName, bridgeIdentity)
	if err != nil {
		return nil, fmt.Errorf("failed to derive key for bridge: %w", err)
	}

	// Store the bridge identity and key
	bim.bridgeIdentities[targetMeshName] = bridgeIdentity
	bim.identityKeys[targetMeshName] = derivedKey

	return bridgeIdentity, nil
}

// generateBridgeID generates a unique bridge identity ID
func (bim *BridgeIdentityManager) generateBridgeID(targetMeshName string) (string, error) {
	// Create a bridge ID that's unique and includes mesh context
	// Format: bridge-{primary_id_prefix}-{target_mesh}-{random}

	primaryPrefix := bim.primaryIdentity.ID
	if len(primaryPrefix) > 8 {
		primaryPrefix = primaryPrefix[:8] // Truncate for readability
	}

	// Generate random suffix
	randomBytes := make([]byte, 4)
	if _, err := rand.Read(randomBytes); err != nil {
		return "", fmt.Errorf("failed to generate random bytes: %w", err)
	}
	randomSuffix := hex.EncodeToString(randomBytes)

	bridgeID := fmt.Sprintf("bridge-%s-%s-%s", primaryPrefix, targetMeshName, randomSuffix)
	return bridgeID, nil
}

// deriveKeyForMesh derives cryptographic key material for a specific mesh bridge
func (bim *BridgeIdentityManager) deriveKeyForMesh(meshName string, bridgeIdentity *Identity) (*DerivedKey, error) {
	// In a real implementation, this would use proper key derivation functions
	// For now, we'll create a simple derived key based on identity and mesh

	seed := fmt.Sprintf("%s:%s:%s", bim.primaryIdentity.ID, meshName, bridgeIdentity.ID)
	keyMaterial := make([]byte, 32) // 256-bit key

	// Simple key derivation (in production, use PBKDF2, HKDF, or similar)
	for i, b := range []byte(seed) {
		if i < len(keyMaterial) {
			keyMaterial[i] = b
		}
	}

	derivedKey := &DerivedKey{
		MeshName:    meshName,
		KeyMaterial: keyMaterial,
		CreatedAt:   time.Now().UTC(),
	}

	return derivedKey, nil
}

// GetBridgeIdentity retrieves the bridge identity for a specific mesh
func (bim *BridgeIdentityManager) GetBridgeIdentity(meshName string) (*Identity, bool) {
	bim.mu.RLock()
	defer bim.mu.RUnlock()

	identity, exists := bim.bridgeIdentities[meshName]
	if !exists {
		return nil, false
	}

	// Return a copy to prevent external modification
	identityCopy := *identity
	return &identityCopy, true
}

// GetBridgeKey retrieves the derived key for a specific mesh bridge
func (bim *BridgeIdentityManager) GetBridgeKey(meshName string) (*DerivedKey, bool) {
	bim.mu.RLock()
	defer bim.mu.RUnlock()

	key, exists := bim.identityKeys[meshName]
	if !exists {
		return nil, false
	}

	// Return a copy to prevent external modification
	keyCopy := *key
	return &keyCopy, true
}

// GetAllBridgeIdentities returns all bridge identities
func (bim *BridgeIdentityManager) GetAllBridgeIdentities() map[string]*Identity {
	bim.mu.RLock()
	defer bim.mu.RUnlock()

	result := make(map[string]*Identity)
	for meshName, identity := range bim.bridgeIdentities {
		identityCopy := *identity
		result[meshName] = &identityCopy
	}
	return result
}

// RemoveBridgeIdentity removes a bridge identity and its associated key
func (bim *BridgeIdentityManager) RemoveBridgeIdentity(meshName string) error {
	bim.mu.Lock()
	defer bim.mu.Unlock()

	if _, exists := bim.bridgeIdentities[meshName]; !exists {
		return fmt.Errorf("no bridge identity found for mesh '%s'", meshName)
	}

	delete(bim.bridgeIdentities, meshName)
	delete(bim.identityKeys, meshName)

	return nil
}

// ValidateBridgeIdentity verifies that a bridge identity is valid for the given mesh
func (bim *BridgeIdentityManager) ValidateBridgeIdentity(meshName string, identity *Identity) error {
	if identity == nil {
		return fmt.Errorf("identity cannot be nil")
	}

	if identity.MeshName != meshName {
		return fmt.Errorf("identity mesh name '%s' does not match expected mesh '%s'", identity.MeshName, meshName)
	}

	if identity.Type != types.MobileAgent {
		return fmt.Errorf("bridge identities must be mobile agents, got type %v", identity.Type)
	}

	if identity.NodeID != bim.primaryIdentity.NodeID {
		return fmt.Errorf("bridge identity node ID '%s' does not match primary identity node ID '%s'",
			identity.NodeID, bim.primaryIdentity.NodeID)
	}

	return nil
}

// GetBridgeCount returns the number of active bridge identities
func (bim *BridgeIdentityManager) GetBridgeCount() int {
	bim.mu.RLock()
	defer bim.mu.RUnlock()
	return len(bim.bridgeIdentities)
}

// RefreshBridgeKey regenerates the key for a specific mesh bridge
func (bim *BridgeIdentityManager) RefreshBridgeKey(meshName string) error {
	bim.mu.Lock()
	defer bim.mu.Unlock()

	identity, exists := bim.bridgeIdentities[meshName]
	if !exists {
		return fmt.Errorf("no bridge identity found for mesh '%s'", meshName)
	}

	// Generate new derived key
	newKey, err := bim.deriveKeyForMesh(meshName, identity)
	if err != nil {
		return fmt.Errorf("failed to refresh key for mesh '%s': %w", meshName, err)
	}

	bim.identityKeys[meshName] = newKey
	return nil
}