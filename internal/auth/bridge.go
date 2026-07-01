// Package auth provides bridge authentication for AmorphDB multi-mesh operations
package auth

import (
	"fmt"
	"math/big"
	"sync"

	"github.com/solifugus/amorphdb/internal/identity"
	"github.com/solifugus/amorphdb/internal/security"
)

// BridgeAuth manages authentication for bridge connections across meshes
type BridgeAuth struct {
	mu             sync.RWMutex
	meshIdentities map[string]*MobileIdentity // mesh_name -> mobile identity
	derivedKeys    map[string]*DerivedKey     // mesh_name -> derived keys
	authenticator  *security.AgentAuthenticator
	activeSessions map[string]*BridgeSession // session_id -> active session
}

// MobileIdentity represents a mobile agent identity for bridge authentication
type MobileIdentity struct {
	Identity     *identity.Identity // The mobile agent identity
	KeyPair      *security.KeyPair  // Cryptographic key pair for this mesh
	Passphrase   string             // Passphrase for key derivation
	DeviceSecret string             // Device-specific secret
	MeshName     string             // Target mesh name
}

// DerivedKey represents derived cryptographic material for a mesh
type DerivedKey struct {
	PublicKey    *big.Int // Public key for this mesh
	PrivateKey   *big.Int // Private key for this mesh
	SharedSecret []byte   // Derived shared secret if applicable
	AgentKey     []byte   // Agent-level encryption key
}

// BridgeSession represents an active bridge authentication session
type BridgeSession struct {
	SessionID   string                          // Unique session identifier
	MeshName    string                          // Target mesh name
	Identity    *MobileIdentity                 // Mobile agent identity
	AuthSession *security.AuthenticationSession // Underlying auth session
	Status      string                          // Session status (authenticating, authenticated, failed)
	Connection  interface{}                     // Connection handle (mesh-specific)
}

// NewBridgeAuth creates a new bridge authentication manager
func NewBridgeAuth() *BridgeAuth {
	return &BridgeAuth{
		meshIdentities: make(map[string]*MobileIdentity),
		derivedKeys:    make(map[string]*DerivedKey),
		authenticator:  security.NewAgentAuthenticator(),
		activeSessions: make(map[string]*BridgeSession),
	}
}

// CreateMobileIdentity creates a mobile agent identity for bridge authentication to a specific mesh
func (ba *BridgeAuth) CreateMobileIdentity(meshName, passphrase, deviceSecret string, baseIdentity *identity.Identity) (*MobileIdentity, error) {
	ba.mu.Lock()
	defer ba.mu.Unlock()

	// Check if we already have an identity for this mesh
	if existing, exists := ba.meshIdentities[meshName]; exists {
		return existing, nil
	}

	// Derive cryptographic key pair for this mesh
	keyPair, err := ba.authenticator.DeriveAgentKeyPair(passphrase, deviceSecret+"|"+meshName)
	if err != nil {
		return nil, fmt.Errorf("failed to derive key pair for mesh %s: %w", meshName, err)
	}

	// Create mobile identity
	mobileIdentity := &MobileIdentity{
		Identity:     baseIdentity,
		KeyPair:      keyPair,
		Passphrase:   passphrase,
		DeviceSecret: deviceSecret,
		MeshName:     meshName,
	}

	// Store the identity
	ba.meshIdentities[meshName] = mobileIdentity

	// Create derived key
	derivedKey := &DerivedKey{
		PublicKey:  keyPair.Public,
		PrivateKey: keyPair.Private,
		AgentKey:   security.DeriveAgentKey(passphrase, deviceSecret+"|"+meshName),
	}
	ba.derivedKeys[meshName] = derivedKey

	return mobileIdentity, nil
}

// GetMobileIdentity returns the mobile agent identity for the specified mesh
func (ba *BridgeAuth) GetMobileIdentity(meshName string) *MobileIdentity {
	ba.mu.RLock()
	defer ba.mu.RUnlock()
	return ba.meshIdentities[meshName]
}

// GetDerivedKey returns the derived key for the specified mesh
func (ba *BridgeAuth) GetDerivedKey(meshName string) *DerivedKey {
	ba.mu.RLock()
	defer ba.mu.RUnlock()
	return ba.derivedKeys[meshName]
}

// AuthenticateToMesh performs challenge-response authentication to a target mesh
func (ba *BridgeAuth) AuthenticateToMesh(meshName, sessionID string, connection interface{}) (*BridgeSession, error) {
	ba.mu.Lock()
	defer ba.mu.Unlock()

	// Get mobile identity for this mesh
	identity, exists := ba.meshIdentities[meshName]
	if !exists {
		return nil, fmt.Errorf("no mobile identity for mesh %s", meshName)
	}

	// Check if session already exists
	if existing, exists := ba.activeSessions[sessionID]; exists {
		return existing, nil
	}

	// Create authentication session
	authSession := security.NewAuthenticationSession(identity.Identity.ID, identity.KeyPair.Public)

	// Create bridge session
	session := &BridgeSession{
		SessionID:   sessionID,
		MeshName:    meshName,
		Identity:    identity,
		AuthSession: authSession,
		Status:      "authenticating",
		Connection:  connection,
	}

	ba.activeSessions[sessionID] = session
	return session, nil
}

// AuthenticateIncomingAgent creates a bridge authentication session for an
// incoming agent. Unlike AuthenticateToMesh (used by the initiator, which
// authenticates outbound using a locally-held mobile identity), the receiver
// has no local identity for the remote agent. It challenges the agent using the
// public key the agent presented in its handshake request. The resulting
// session's Identity is nil — only the AuthSession (carrying the presented
// public key) is needed to issue and verify the challenge.
func (ba *BridgeAuth) AuthenticateIncomingAgent(sourceMesh, sessionID, agentIdentity string, agentPublicKey *big.Int, connection interface{}) (*BridgeSession, error) {
	ba.mu.Lock()
	defer ba.mu.Unlock()

	// Check if session already exists
	if existing, exists := ba.activeSessions[sessionID]; exists {
		return existing, nil
	}

	// Create authentication session bound to the agent's presented public key.
	authSession := security.NewAuthenticationSession(agentIdentity, agentPublicKey)

	session := &BridgeSession{
		SessionID:   sessionID,
		MeshName:    sourceMesh,
		AuthSession: authSession,
		Status:      "authenticating",
		Connection:  connection,
	}

	ba.activeSessions[sessionID] = session
	return session, nil
}

// CreateChallenge creates an authentication challenge for this bridge session
func (ba *BridgeAuth) CreateChallenge(sessionID string) (*security.AuthChallengeMessage, error) {
	ba.mu.RLock()
	session, exists := ba.activeSessions[sessionID]
	ba.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("session %s not found", sessionID)
	}

	challenge, err := session.AuthSession.CreateChallenge()
	if err != nil {
		ba.updateSessionStatus(sessionID, "failed")
		return nil, fmt.Errorf("failed to create challenge: %w", err)
	}

	return challenge, nil
}

// RespondToChallenge responds to an authentication challenge using mobile agent credentials
func (ba *BridgeAuth) RespondToChallenge(challenge *security.AuthChallengeMessage, meshName string) (*security.AuthResponseMessage, error) {
	ba.mu.RLock()
	identity, exists := ba.meshIdentities[meshName]
	ba.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("no mobile identity for mesh %s", meshName)
	}

	// Create authentication challenge object
	authChallenge := &security.AuthenticationChallenge{
		ChallengeID:        challenge.ChallengeID,
		EncryptedData:      challenge.EncryptedData,
		EphemeralPublicKey: challenge.EphemeralPublicKey,
	}

	// Respond to challenge using mobile agent keys
	authResponse, err := ba.authenticator.RespondToChallenge(authChallenge, identity.KeyPair)
	if err != nil {
		return nil, fmt.Errorf("failed to respond to challenge: %w", err)
	}

	// Create response message
	return &security.AuthResponseMessage{
		AgentIdentity: identity.Identity.ID,
		ChallengeID:   authResponse.ChallengeID,
		DecryptedData: authResponse.DecryptedData,
	}, nil
}

// VerifyResponse verifies an authentication response for a bridge session
func (ba *BridgeAuth) VerifyResponse(sessionID string, response *security.AuthResponseMessage) (bool, error) {
	ba.mu.RLock()
	session, exists := ba.activeSessions[sessionID]
	ba.mu.RUnlock()

	if !exists {
		return false, fmt.Errorf("session %s not found", sessionID)
	}

	authenticated, err := session.AuthSession.VerifyResponse(response)
	if err != nil {
		ba.updateSessionStatus(sessionID, "failed")
		return false, fmt.Errorf("failed to verify response: %w", err)
	}

	if authenticated {
		ba.updateSessionStatus(sessionID, "authenticated")
	} else {
		ba.updateSessionStatus(sessionID, "failed")
	}

	return authenticated, nil
}

// PerformChallengeResponse performs the complete challenge-response authentication flow
func (ba *BridgeAuth) PerformChallengeResponse(sessionID string, challengeHandler func(*security.AuthChallengeMessage) (*security.AuthResponseMessage, error)) error {
	// Create challenge
	challenge, err := ba.CreateChallenge(sessionID)
	if err != nil {
		return fmt.Errorf("failed to create challenge: %w", err)
	}

	// Handle challenge (typically sent over network and processed by remote peer)
	response, err := challengeHandler(challenge)
	if err != nil {
		return fmt.Errorf("challenge handler failed: %w", err)
	}

	// Verify response
	authenticated, err := ba.VerifyResponse(sessionID, response)
	if err != nil {
		return fmt.Errorf("failed to verify response: %w", err)
	}

	if !authenticated {
		return fmt.Errorf("authentication failed")
	}

	return nil
}

// GetSession returns the bridge session for the specified session ID
func (ba *BridgeAuth) GetSession(sessionID string) *BridgeSession {
	ba.mu.RLock()
	defer ba.mu.RUnlock()
	return ba.activeSessions[sessionID]
}

// CloseSession closes and removes a bridge authentication session
func (ba *BridgeAuth) CloseSession(sessionID string) error {
	ba.mu.Lock()
	defer ba.mu.Unlock()

	session, exists := ba.activeSessions[sessionID]
	if !exists {
		return fmt.Errorf("session %s not found", sessionID)
	}

	// Update status to closed
	session.Status = "closed"

	// Remove from active sessions
	delete(ba.activeSessions, sessionID)

	return nil
}

// ListActiveSessions returns all active bridge authentication sessions
func (ba *BridgeAuth) ListActiveSessions() map[string]*BridgeSession {
	ba.mu.RLock()
	defer ba.mu.RUnlock()

	result := make(map[string]*BridgeSession)
	for sessionID, session := range ba.activeSessions {
		result[sessionID] = session
	}
	return result
}

// RemoveMobileIdentity removes the mobile identity for the specified mesh
func (ba *BridgeAuth) RemoveMobileIdentity(meshName string) error {
	ba.mu.Lock()
	defer ba.mu.Unlock()

	// Close any active sessions for this mesh
	for sessionID, session := range ba.activeSessions {
		if session.MeshName == meshName {
			session.Status = "closed"
			delete(ba.activeSessions, sessionID)
		}
	}

	// Remove identity and keys
	delete(ba.meshIdentities, meshName)
	delete(ba.derivedKeys, meshName)

	return nil
}

// ValidateMobileIdentity validates that a mobile identity is properly configured
func (ba *BridgeAuth) ValidateMobileIdentity(meshName string) error {
	ba.mu.RLock()
	defer ba.mu.RUnlock()

	identity, exists := ba.meshIdentities[meshName]
	if !exists {
		return fmt.Errorf("no mobile identity for mesh %s", meshName)
	}

	// Validate identity structure
	if err := identity.Validate(); err != nil {
		return fmt.Errorf("invalid mobile identity for mesh %s: %w", meshName, err)
	}

	// Validate derived key
	key, exists := ba.derivedKeys[meshName]
	if !exists {
		return fmt.Errorf("no derived key for mesh %s", meshName)
	}

	if key.PublicKey == nil || key.PrivateKey == nil {
		return fmt.Errorf("invalid derived key for mesh %s", meshName)
	}

	return nil
}

// updateSessionStatus updates the status of a bridge session (internal helper)
func (ba *BridgeAuth) updateSessionStatus(sessionID, status string) {
	ba.mu.Lock()
	defer ba.mu.Unlock()

	if session, exists := ba.activeSessions[sessionID]; exists {
		session.Status = status
	}
}

// Validate validates that a mobile identity is properly configured
func (mi *MobileIdentity) Validate() error {
	if mi == nil {
		return fmt.Errorf("mobile identity cannot be nil")
	}

	if err := identity.ValidateIdentity(mi.Identity); err != nil {
		return fmt.Errorf("invalid identity: %w", err)
	}

	if mi.KeyPair == nil {
		return fmt.Errorf("key pair cannot be nil")
	}

	if mi.KeyPair.Public == nil || mi.KeyPair.Private == nil {
		return fmt.Errorf("public and private keys cannot be nil")
	}

	if mi.Passphrase == "" {
		return fmt.Errorf("passphrase cannot be empty")
	}

	if mi.DeviceSecret == "" {
		return fmt.Errorf("device secret cannot be empty")
	}

	if mi.MeshName == "" {
		return fmt.Errorf("mesh name cannot be empty")
	}

	return nil
}

// String returns a string representation of the mobile identity
func (mi *MobileIdentity) String() string {
	return fmt.Sprintf("MobileIdentity{ID=%s, MeshName=%s}",
		mi.Identity.ID, mi.MeshName)
}

// String returns a string representation of the bridge session
func (bs *BridgeSession) String() string {
	return fmt.Sprintf("BridgeSession{ID=%s, MeshName=%s, Status=%s}",
		bs.SessionID, bs.MeshName, bs.Status)
}
