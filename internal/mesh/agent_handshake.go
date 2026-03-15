// Package mesh provides agent handshake protocol for AmorphDB bridge authentication
package mesh

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/solifugus/amorphdb/internal/auth"
	"github.com/solifugus/amorphdb/internal/crypto"
	"github.com/solifugus/amorphdb/internal/identity"
	"github.com/solifugus/amorphdb/internal/security"
)

// AgentHandshake manages the handshake protocol for mobile agent bridge authentication
type AgentHandshake struct {
	mu                sync.RWMutex
	bridgeAuth        *auth.BridgeAuth           // Bridge authentication manager
	keyDerivation     *crypto.MobileKeyDerivation // Key derivation for mobile agents
	activeHandshakes  map[string]*HandshakeSession // session_id -> active handshake
	identityManager   *identity.IdentityManager    // Identity management
	meshName          string                       // Primary mesh name
	nodeID            string                       // This node's ID
}

// HandshakeSession represents an active agent handshake session
type HandshakeSession struct {
	SessionID      string                      // Unique session identifier
	RemoteMesh     string                      // Remote mesh name
	RemoteAddress  string                      // Remote node address
	Connection     net.Conn                    // Network connection
	Status         string                      // Session status
	MobileIdentity *auth.MobileIdentity        // Mobile agent identity for this session
	AuthSession    *auth.BridgeSession         // Bridge authentication session
	CreatedAt      time.Time                   // Session creation time
	LastActivity   time.Time                   // Last activity timestamp
}

// HandshakeRequest represents the initial handshake request from a mobile agent
type HandshakeRequest struct {
	SessionID       string `json:"session_id"`       // Unique session identifier
	AgentIdentity   string `json:"agent_identity"`   // Mobile agent identity
	SourceMesh      string `json:"source_mesh"`      // Source mesh name
	TargetMesh      string `json:"target_mesh"`      // Target mesh name
	PublicKey       string `json:"public_key"`       // Agent's public key for this mesh
	RequestType     string `json:"request_type"`     // Type of request (bridge, query, etc.)
	Timestamp       int64  `json:"timestamp"`        // Request timestamp
}

// HandshakeResponse represents the response to a handshake request
type HandshakeResponse struct {
	SessionID       string `json:"session_id"`       // Echo of session identifier
	Status          string `json:"status"`           // Response status (accepted, rejected, challenge)
	MeshName        string `json:"mesh_name"`        // This mesh name
	Challenge       *security.AuthChallengeMessage `json:"challenge,omitempty"` // Auth challenge if required
	ErrorMessage    string `json:"error_message,omitempty"`    // Error description if rejected
	Timestamp       int64  `json:"timestamp"`        // Response timestamp
}

// HandshakeComplete represents the final handshake completion
type HandshakeComplete struct {
	SessionID       string                    `json:"session_id"`       // Session identifier
	AuthResponse    *security.AuthResponseMessage `json:"auth_response"`    // Response to challenge
	Authenticated   bool                      `json:"authenticated"`    // Whether authentication succeeded
	BridgeEstablished bool                    `json:"bridge_established"` // Whether bridge was established
	Timestamp       int64                     `json:"timestamp"`        // Completion timestamp
}

// NewAgentHandshake creates a new agent handshake manager
func NewAgentHandshake(meshName, nodeID string, identityManager *identity.IdentityManager) *AgentHandshake {
	return &AgentHandshake{
		bridgeAuth:       auth.NewBridgeAuth(),
		activeHandshakes: make(map[string]*HandshakeSession),
		identityManager:  identityManager,
		meshName:         meshName,
		nodeID:           nodeID,
	}
}

// SetKeyDerivation sets the mobile key derivation manager
func (ah *AgentHandshake) SetKeyDerivation(keyDerivation *crypto.MobileKeyDerivation) {
	ah.keyDerivation = keyDerivation
}

// InitiateHandshake initiates a handshake with a remote mesh as a mobile agent
func (ah *AgentHandshake) InitiateHandshake(targetMesh, targetAddress string, conn net.Conn) (*HandshakeSession, error) {
	ah.mu.Lock()
	defer ah.mu.Unlock()

	// Generate unique session ID
	sessionID, err := generateSessionID()
	if err != nil {
		return nil, fmt.Errorf("failed to generate session ID: %w", err)
	}

	// Get or create mobile identity for target mesh
	mobileIdentity, err := ah.getOrCreateMobileIdentity(targetMesh)
	if err != nil {
		return nil, fmt.Errorf("failed to get mobile identity for mesh %s: %w", targetMesh, err)
	}

	// Create handshake session
	session := &HandshakeSession{
		SessionID:      sessionID,
		RemoteMesh:     targetMesh,
		RemoteAddress:  targetAddress,
		Connection:     conn,
		Status:         "initiating",
		MobileIdentity: mobileIdentity,
		CreatedAt:      time.Now().UTC(),
		LastActivity:   time.Now().UTC(),
	}

	ah.activeHandshakes[sessionID] = session

	// Create handshake request
	request := &HandshakeRequest{
		SessionID:     sessionID,
		AgentIdentity: mobileIdentity.Identity.ID,
		SourceMesh:    ah.meshName,
		TargetMesh:    targetMesh,
		PublicKey:     mobileIdentity.KeyPair.Public.String(),
		RequestType:   "bridge",
		Timestamp:     time.Now().Unix(),
	}

	// Send handshake request
	if err := ah.sendHandshakeRequest(conn, request); err != nil {
		session.Status = "failed"
		return nil, fmt.Errorf("failed to send handshake request: %w", err)
	}

	session.Status = "waiting_response"
	session.LastActivity = time.Now().UTC()

	return session, nil
}

// HandleIncomingHandshake handles an incoming handshake request from a mobile agent
func (ah *AgentHandshake) HandleIncomingHandshake(conn net.Conn, request *HandshakeRequest) (*HandshakeResponse, error) {
	ah.mu.Lock()
	defer ah.mu.Unlock()

	// Validate handshake request
	if err := ah.validateHandshakeRequest(request); err != nil {
		return &HandshakeResponse{
			SessionID:    request.SessionID,
			Status:       "rejected",
			MeshName:     ah.meshName,
			ErrorMessage: err.Error(),
			Timestamp:    time.Now().Unix(),
		}, nil
	}

	// Create handshake session
	session := &HandshakeSession{
		SessionID:     request.SessionID,
		RemoteMesh:    request.SourceMesh,
		RemoteAddress: conn.RemoteAddr().String(),
		Connection:    conn,
		Status:        "processing",
		CreatedAt:     time.Now().UTC(),
		LastActivity:  time.Now().UTC(),
	}

	ah.activeHandshakes[request.SessionID] = session

	// Create bridge authentication session
	bridgeSession, err := ah.bridgeAuth.AuthenticateToMesh(request.SourceMesh, request.SessionID, conn)
	if err != nil {
		session.Status = "failed"
		return &HandshakeResponse{
			SessionID:    request.SessionID,
			Status:       "rejected",
			MeshName:     ah.meshName,
			ErrorMessage: fmt.Sprintf("Failed to create auth session: %s", err.Error()),
			Timestamp:    time.Now().Unix(),
		}, nil
	}

	session.AuthSession = bridgeSession

	// Create authentication challenge
	challenge, err := ah.bridgeAuth.CreateChallenge(request.SessionID)
	if err != nil {
		session.Status = "failed"
		return &HandshakeResponse{
			SessionID:    request.SessionID,
			Status:       "rejected",
			MeshName:     ah.meshName,
			ErrorMessage: fmt.Sprintf("Failed to create challenge: %s", err.Error()),
			Timestamp:    time.Now().Unix(),
		}, nil
	}

	session.Status = "challenge_sent"
	session.LastActivity = time.Now().UTC()

	return &HandshakeResponse{
		SessionID: request.SessionID,
		Status:    "challenge",
		MeshName:  ah.meshName,
		Challenge: challenge,
		Timestamp: time.Now().Unix(),
	}, nil
}

// ProcessHandshakeResponse processes a handshake response from a remote mesh
func (ah *AgentHandshake) ProcessHandshakeResponse(sessionID string, response *HandshakeResponse) (*HandshakeComplete, error) {
	ah.mu.Lock()
	session, exists := ah.activeHandshakes[sessionID]
	ah.mu.Unlock()

	if !exists {
		return nil, fmt.Errorf("handshake session %s not found", sessionID)
	}

	session.LastActivity = time.Now().UTC()

	switch response.Status {
	case "challenge":
		// Process authentication challenge
		return ah.processAuthenticationChallenge(session, response.Challenge)

	case "rejected":
		session.Status = "failed"
		return &HandshakeComplete{
			SessionID:     sessionID,
			Authenticated: false,
			Timestamp:     time.Now().Unix(),
		}, fmt.Errorf("handshake rejected: %s", response.ErrorMessage)

	case "accepted":
		// Handshake completed without challenge
		session.Status = "completed"
		return &HandshakeComplete{
			SessionID:         sessionID,
			Authenticated:     true,
			BridgeEstablished: true,
			Timestamp:         time.Now().Unix(),
		}, nil

	default:
		session.Status = "failed"
		return nil, fmt.Errorf("unknown handshake response status: %s", response.Status)
	}
}

// ProcessAuthenticationResponse processes the final authentication response
func (ah *AgentHandshake) ProcessAuthenticationResponse(sessionID string, authResponse *security.AuthResponseMessage) (*HandshakeComplete, error) {
	ah.mu.RLock()
	session, exists := ah.activeHandshakes[sessionID]
	ah.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("handshake session %s not found", sessionID)
	}

	if session.AuthSession == nil {
		return nil, fmt.Errorf("no auth session for handshake %s", sessionID)
	}

	// Verify authentication response
	authenticated, err := ah.bridgeAuth.VerifyResponse(sessionID, authResponse)
	if err != nil {
		session.Status = "failed"
		return nil, fmt.Errorf("authentication verification failed: %w", err)
	}

	complete := &HandshakeComplete{
		SessionID:     sessionID,
		AuthResponse:  authResponse,
		Authenticated: authenticated,
		Timestamp:     time.Now().Unix(),
	}

	if authenticated {
		session.Status = "authenticated"
		complete.BridgeEstablished = true
	} else {
		session.Status = "failed"
	}

	session.LastActivity = time.Now().UTC()

	return complete, nil
}

// GetHandshakeSession returns the handshake session for the specified session ID
func (ah *AgentHandshake) GetHandshakeSession(sessionID string) *HandshakeSession {
	ah.mu.RLock()
	defer ah.mu.RUnlock()
	return ah.activeHandshakes[sessionID]
}

// ListActiveHandshakes returns all active handshake sessions
func (ah *AgentHandshake) ListActiveHandshakes() map[string]*HandshakeSession {
	ah.mu.RLock()
	defer ah.mu.RUnlock()

	result := make(map[string]*HandshakeSession)
	for sessionID, session := range ah.activeHandshakes {
		result[sessionID] = session
	}
	return result
}

// CloseHandshake closes and cleans up a handshake session
func (ah *AgentHandshake) CloseHandshake(sessionID string) error {
	ah.mu.Lock()
	defer ah.mu.Unlock()

	session, exists := ah.activeHandshakes[sessionID]
	if !exists {
		return fmt.Errorf("handshake session %s not found", sessionID)
	}

	// Close authentication session if it exists
	if session.AuthSession != nil {
		_ = ah.bridgeAuth.CloseSession(sessionID)
	}

	// Close network connection if it exists
	if session.Connection != nil {
		_ = session.Connection.Close()
	}

	// Mark session as closed
	session.Status = "closed"

	// Remove from active sessions
	delete(ah.activeHandshakes, sessionID)

	return nil
}

// CleanupExpiredHandshakes removes expired handshake sessions
func (ah *AgentHandshake) CleanupExpiredHandshakes(maxAge time.Duration) {
	ah.mu.Lock()
	defer ah.mu.Unlock()

	cutoff := time.Now().UTC().Add(-maxAge)

	for sessionID, session := range ah.activeHandshakes {
		if session.LastActivity.Before(cutoff) {
			// Close connection
			if session.Connection != nil {
				_ = session.Connection.Close()
			}

			// Close auth session
			if session.AuthSession != nil {
				_ = ah.bridgeAuth.CloseSession(sessionID)
			}

			// Remove from active handshakes
			delete(ah.activeHandshakes, sessionID)
		}
	}
}

// processAuthenticationChallenge processes an authentication challenge during handshake
func (ah *AgentHandshake) processAuthenticationChallenge(session *HandshakeSession, challenge *security.AuthChallengeMessage) (*HandshakeComplete, error) {
	if session.MobileIdentity == nil {
		return nil, fmt.Errorf("no mobile identity for session %s", session.SessionID)
	}

	// Respond to challenge using mobile identity
	authResponse, err := ah.bridgeAuth.RespondToChallenge(challenge, session.RemoteMesh)
	if err != nil {
		session.Status = "failed"
		return nil, fmt.Errorf("failed to respond to challenge: %w", err)
	}

	// Send authentication response
	if err := ah.sendAuthenticationResponse(session.Connection, authResponse); err != nil {
		session.Status = "failed"
		return nil, fmt.Errorf("failed to send authentication response: %w", err)
	}

	session.Status = "auth_sent"
	session.LastActivity = time.Now().UTC()

	return &HandshakeComplete{
		SessionID:    session.SessionID,
		AuthResponse: authResponse,
		Timestamp:    time.Now().Unix(),
	}, nil
}

// getOrCreateMobileIdentity gets or creates a mobile identity for the target mesh
func (ah *AgentHandshake) getOrCreateMobileIdentity(targetMesh string) (*auth.MobileIdentity, error) {
	// Check if we already have a mobile identity for this mesh
	if existing := ah.bridgeAuth.GetMobileIdentity(targetMesh); existing != nil {
		return existing, nil
	}

	// Create new bridge identity
	bridgeIdentity, err := ah.identityManager.CreateBridgeIdentity(targetMesh)
	if err != nil {
		return nil, fmt.Errorf("failed to create bridge identity: %w", err)
	}

	// Create mobile identity
	if ah.keyDerivation == nil {
		return nil, fmt.Errorf("key derivation not configured")
	}

	// For now, use placeholder passphrase and device secret
	// In production, these would come from secure user input or device storage
	mobileIdentity, err := ah.bridgeAuth.CreateMobileIdentity(
		targetMesh,
		"mobile-agent-passphrase", // TODO: Get from secure source
		"device-secret-"+ah.nodeID, // TODO: Get from device storage
		bridgeIdentity,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create mobile identity: %w", err)
	}

	return mobileIdentity, nil
}

// validateHandshakeRequest validates an incoming handshake request
func (ah *AgentHandshake) validateHandshakeRequest(request *HandshakeRequest) error {
	if request.SessionID == "" {
		return fmt.Errorf("session ID cannot be empty")
	}

	if request.AgentIdentity == "" {
		return fmt.Errorf("agent identity cannot be empty")
	}

	if request.SourceMesh == "" {
		return fmt.Errorf("source mesh cannot be empty")
	}

	if request.TargetMesh != ah.meshName {
		return fmt.Errorf("target mesh %s does not match our mesh %s", request.TargetMesh, ah.meshName)
	}

	if request.PublicKey == "" {
		return fmt.Errorf("public key cannot be empty")
	}

	if request.RequestType != "bridge" {
		return fmt.Errorf("unsupported request type: %s", request.RequestType)
	}

	return nil
}

// sendHandshakeRequest sends a handshake request over the connection
func (ah *AgentHandshake) sendHandshakeRequest(conn net.Conn, request *HandshakeRequest) error {
	// In a real implementation, this would serialize the request and send it
	// For now, this is a placeholder
	return nil
}

// sendAuthenticationResponse sends an authentication response over the connection
func (ah *AgentHandshake) sendAuthenticationResponse(conn net.Conn, response *security.AuthResponseMessage) error {
	// In a real implementation, this would serialize the response and send it
	// For now, this is a placeholder
	return nil
}

// generateSessionID generates a unique session identifier
func generateSessionID() (string, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

// String returns a string representation of the handshake session
func (hs *HandshakeSession) String() string {
	return fmt.Sprintf("HandshakeSession{ID=%s, RemoteMesh=%s, Status=%s}",
		hs.SessionID, hs.RemoteMesh, hs.Status)
}

// IsActive returns true if the handshake session is still active
func (hs *HandshakeSession) IsActive() bool {
	return hs.Status != "completed" && hs.Status != "failed" && hs.Status != "closed"
}

// IsAuthenticated returns true if the handshake has completed authentication
func (hs *HandshakeSession) IsAuthenticated() bool {
	return hs.Status == "authenticated" || hs.Status == "completed"
}