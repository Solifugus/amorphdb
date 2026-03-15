// Package crypto provides mobile agent cryptographic operations for AmorphDB bridge authentication
package crypto

import (
	"crypto/sha256"
	"fmt"
	"math/big"

	"github.com/solifugus/amorphdb/internal/security"
)

// MobileKeyDerivation handles key derivation for mobile agents across multiple meshes
type MobileKeyDerivation struct {
	authenticator *security.AgentAuthenticator
	baseFactors   *BaseFactors
}

// BaseFactors represents the base authentication factors for a mobile agent
type BaseFactors struct {
	Passphrase   string // User-provided passphrase
	DeviceSecret string // Device-specific secret
	NodeID       string // Node identifier where agent was created
}

// MeshKeyPair represents cryptographic keys derived for a specific mesh
type MeshKeyPair struct {
	MeshName      string            // Target mesh name
	PublicKey     *big.Int          // Public key for this mesh
	PrivateKey    *big.Int          // Private key for this mesh
	AgentKey      []byte            // Agent-level encryption key
	BaseFactors   *BaseFactors      // Base factors used for derivation
	MeshSalt      string            // Mesh-specific salt
}

// NewMobileKeyDerivation creates a new mobile key derivation manager
func NewMobileKeyDerivation(passphrase, deviceSecret, nodeID string) *MobileKeyDerivation {
	return &MobileKeyDerivation{
		authenticator: security.NewAgentAuthenticator(),
		baseFactors: &BaseFactors{
			Passphrase:   passphrase,
			DeviceSecret: deviceSecret,
			NodeID:       nodeID,
		},
	}
}

// DeriveKeyPairForMesh derives a unique key pair for the specified mesh
func (mkd *MobileKeyDerivation) DeriveKeyPairForMesh(meshName string) (*MeshKeyPair, error) {
	if meshName == "" {
		return nil, fmt.Errorf("mesh name cannot be empty")
	}

	// Create mesh-specific salt by combining base factors with mesh name
	meshSalt := mkd.createMeshSalt(meshName)

	// Derive key pair using mesh-specific salt
	keyPair, err := mkd.authenticator.DeriveAgentKeyPair(
		mkd.baseFactors.Passphrase,
		mkd.baseFactors.DeviceSecret+"|"+meshSalt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to derive key pair for mesh %s: %w", meshName, err)
	}

	// Derive agent-level encryption key
	agentKey := security.DeriveAgentKey(
		mkd.baseFactors.Passphrase,
		mkd.baseFactors.DeviceSecret+"|"+meshSalt,
	)

	return &MeshKeyPair{
		MeshName:    meshName,
		PublicKey:   keyPair.Public,
		PrivateKey:  keyPair.Private,
		AgentKey:    agentKey,
		BaseFactors: mkd.baseFactors,
		MeshSalt:    meshSalt,
	}, nil
}

// DeriveMultipleMeshKeys derives key pairs for multiple meshes at once
func (mkd *MobileKeyDerivation) DeriveMultipleMeshKeys(meshNames []string) (map[string]*MeshKeyPair, error) {
	if len(meshNames) == 0 {
		return make(map[string]*MeshKeyPair), nil
	}

	results := make(map[string]*MeshKeyPair)

	for _, meshName := range meshNames {
		keyPair, err := mkd.DeriveKeyPairForMesh(meshName)
		if err != nil {
			return nil, fmt.Errorf("failed to derive key pair for mesh %s: %w", meshName, err)
		}
		results[meshName] = keyPair
	}

	return results, nil
}

// RegenegrateKeyPairForMesh regenerates a key pair for a mesh (useful for key rotation)
func (mkd *MobileKeyDerivation) RegenerateKeyPairForMesh(meshName string, newSalt string) (*MeshKeyPair, error) {
	if meshName == "" {
		return nil, fmt.Errorf("mesh name cannot be empty")
	}

	// Use provided salt or generate a new one
	meshSalt := newSalt
	if meshSalt == "" {
		meshSalt = mkd.createMeshSalt(meshName) + "|rotated"
	}

	// Derive key pair using the new salt
	keyPair, err := mkd.authenticator.DeriveAgentKeyPair(
		mkd.baseFactors.Passphrase,
		mkd.baseFactors.DeviceSecret+"|"+meshSalt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to regenerate key pair for mesh %s: %w", meshName, err)
	}

	// Derive agent-level encryption key
	agentKey := security.DeriveAgentKey(
		mkd.baseFactors.Passphrase,
		mkd.baseFactors.DeviceSecret+"|"+meshSalt,
	)

	return &MeshKeyPair{
		MeshName:    meshName,
		PublicKey:   keyPair.Public,
		PrivateKey:  keyPair.Private,
		AgentKey:    agentKey,
		BaseFactors: mkd.baseFactors,
		MeshSalt:    meshSalt,
	}, nil
}

// ValidateKeyPairForMesh validates that a key pair is correctly derived for the given mesh
func (mkd *MobileKeyDerivation) ValidateKeyPairForMesh(meshKeyPair *MeshKeyPair) error {
	if meshKeyPair == nil {
		return fmt.Errorf("mesh key pair cannot be nil")
	}

	// Re-derive key pair using the same factors and salt
	expectedKeyPair, err := mkd.authenticator.DeriveAgentKeyPair(
		meshKeyPair.BaseFactors.Passphrase,
		meshKeyPair.BaseFactors.DeviceSecret+"|"+meshKeyPair.MeshSalt,
	)
	if err != nil {
		return fmt.Errorf("failed to re-derive key pair for validation: %w", err)
	}

	// Compare public keys
	if meshKeyPair.PublicKey.Cmp(expectedKeyPair.Public) != 0 {
		return fmt.Errorf("public key validation failed for mesh %s", meshKeyPair.MeshName)
	}

	// Compare private keys
	if meshKeyPair.PrivateKey.Cmp(expectedKeyPair.Private) != 0 {
		return fmt.Errorf("private key validation failed for mesh %s", meshKeyPair.MeshName)
	}

	// Validate agent key
	expectedAgentKey := security.DeriveAgentKey(
		meshKeyPair.BaseFactors.Passphrase,
		meshKeyPair.BaseFactors.DeviceSecret+"|"+meshKeyPair.MeshSalt,
	)

	if len(meshKeyPair.AgentKey) != len(expectedAgentKey) {
		return fmt.Errorf("agent key length validation failed for mesh %s", meshKeyPair.MeshName)
	}

	for i := range meshKeyPair.AgentKey {
		if meshKeyPair.AgentKey[i] != expectedAgentKey[i] {
			return fmt.Errorf("agent key validation failed for mesh %s", meshKeyPair.MeshName)
		}
	}

	return nil
}

// GetPublicKeyForMesh returns just the public key for a mesh (for sharing with peers)
func (mkd *MobileKeyDerivation) GetPublicKeyForMesh(meshName string) (*big.Int, error) {
	keyPair, err := mkd.DeriveKeyPairForMesh(meshName)
	if err != nil {
		return nil, err
	}
	return keyPair.PublicKey, nil
}

// CreateSharedSecretWithPeer creates a shared secret with another node's public key
func (mkd *MobileKeyDerivation) CreateSharedSecretWithPeer(meshName string, peerPublicKey *big.Int) ([]byte, error) {
	// Get our private key for this mesh
	keyPair, err := mkd.DeriveKeyPairForMesh(meshName)
	if err != nil {
		return nil, fmt.Errorf("failed to derive key pair: %w", err)
	}

	// Use Diffie-Hellman to compute shared secret
	dh := security.NewDiffieHellman()
	sharedSecret, err := dh.ComputeSharedSecret(keyPair.PrivateKey, peerPublicKey)
	if err != nil {
		return nil, fmt.Errorf("failed to compute shared secret: %w", err)
	}

	// Convert to byte array
	return sharedSecret.Bytes(), nil
}

// UpdateBaseFactors updates the base factors (useful when passphrase changes)
func (mkd *MobileKeyDerivation) UpdateBaseFactors(newPassphrase, newDeviceSecret string) {
	if newPassphrase != "" {
		mkd.baseFactors.Passphrase = newPassphrase
	}
	if newDeviceSecret != "" {
		mkd.baseFactors.DeviceSecret = newDeviceSecret
	}
}

// createMeshSalt creates a deterministic but unique salt for the specified mesh
func (mkd *MobileKeyDerivation) createMeshSalt(meshName string) string {
	// Combine all factors to create mesh-specific salt
	combined := fmt.Sprintf("%s|%s|%s|%s",
		mkd.baseFactors.NodeID,
		mkd.baseFactors.DeviceSecret,
		meshName,
		"mobile-agent-salt",
	)

	// Hash to create deterministic salt
	hash := sha256.Sum256([]byte(combined))
	return fmt.Sprintf("%x", hash[:16]) // Use first 16 bytes as hex string
}

// ExportKeyPairForMesh exports key pair for external storage (with private key)
func (mkp *MeshKeyPair) ExportKeyPair() map[string]interface{} {
	return map[string]interface{}{
		"mesh_name":    mkp.MeshName,
		"public_key":   mkp.PublicKey.String(),
		"private_key":  mkp.PrivateKey.String(),
		"agent_key":    mkp.AgentKey,
		"mesh_salt":    mkp.MeshSalt,
		"passphrase":   mkp.BaseFactors.Passphrase,
		"device_secret": mkp.BaseFactors.DeviceSecret,
		"node_id":      mkp.BaseFactors.NodeID,
	}
}

// ExportPublicKey exports just the public key for sharing (safe for network transmission)
func (mkp *MeshKeyPair) ExportPublicKey() map[string]interface{} {
	return map[string]interface{}{
		"mesh_name":  mkp.MeshName,
		"public_key": mkp.PublicKey.String(),
		"node_id":    mkp.BaseFactors.NodeID,
	}
}

// ImportKeyPair imports a key pair from external storage
func ImportKeyPair(data map[string]interface{}) (*MeshKeyPair, error) {
	// Extract required fields
	meshName, ok := data["mesh_name"].(string)
	if !ok {
		return nil, fmt.Errorf("missing or invalid mesh_name")
	}

	publicKeyStr, ok := data["public_key"].(string)
	if !ok {
		return nil, fmt.Errorf("missing or invalid public_key")
	}

	privateKeyStr, ok := data["private_key"].(string)
	if !ok {
		return nil, fmt.Errorf("missing or invalid private_key")
	}

	// Parse big integers
	publicKey, success := new(big.Int).SetString(publicKeyStr, 10)
	if !success {
		return nil, fmt.Errorf("failed to parse public key")
	}

	privateKey, success := new(big.Int).SetString(privateKeyStr, 10)
	if !success {
		return nil, fmt.Errorf("failed to parse private key")
	}

	// Extract other fields
	agentKey, ok := data["agent_key"].([]byte)
	if !ok {
		return nil, fmt.Errorf("missing or invalid agent_key")
	}

	meshSalt, ok := data["mesh_salt"].(string)
	if !ok {
		return nil, fmt.Errorf("missing or invalid mesh_salt")
	}

	passphrase, ok := data["passphrase"].(string)
	if !ok {
		return nil, fmt.Errorf("missing or invalid passphrase")
	}

	deviceSecret, ok := data["device_secret"].(string)
	if !ok {
		return nil, fmt.Errorf("missing or invalid device_secret")
	}

	nodeID, ok := data["node_id"].(string)
	if !ok {
		return nil, fmt.Errorf("missing or invalid node_id")
	}

	return &MeshKeyPair{
		MeshName:   meshName,
		PublicKey:  publicKey,
		PrivateKey: privateKey,
		AgentKey:   agentKey,
		BaseFactors: &BaseFactors{
			Passphrase:   passphrase,
			DeviceSecret: deviceSecret,
			NodeID:       nodeID,
		},
		MeshSalt: meshSalt,
	}, nil
}

// String returns a string representation of the mesh key pair
func (mkp *MeshKeyPair) String() string {
	return fmt.Sprintf("MeshKeyPair{MeshName=%s, PublicKey=%s...}",
		mkp.MeshName, mkp.PublicKey.String()[:16])
}

// String returns a string representation of the base factors
func (bf *BaseFactors) String() string {
	return fmt.Sprintf("BaseFactors{NodeID=%s, PassphraseLength=%d, DeviceSecretLength=%d}",
		bf.NodeID, len(bf.Passphrase), len(bf.DeviceSecret))
}