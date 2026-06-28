// Package mesh provides agent handshake tests
package mesh

import (
	"net"
	"testing"
	"time"

	"github.com/solifugus/amorphdb/internal/crypto"
	"github.com/solifugus/amorphdb/internal/identity"
	"github.com/solifugus/amorphdb/internal/security"
)

func TestNewAgentHandshake(t *testing.T) {
	meshName := "test-mesh"
	nodeID := "test-node-123"
	identityManager := identity.NewIdentityManager(meshName, nodeID)

	handshake := NewAgentHandshake(meshName, nodeID, identityManager)

	if handshake == nil {
		t.Fatal("Agent handshake should not be nil")
	}

	if handshake.meshName != meshName {
		t.Errorf("Expected mesh name %s, got %s", meshName, handshake.meshName)
	}

	if handshake.nodeID != nodeID {
		t.Errorf("Expected node ID %s, got %s", nodeID, handshake.nodeID)
	}

	if handshake.bridgeAuth == nil {
		t.Fatal("Bridge auth should not be nil")
	}

	if handshake.activeHandshakes == nil {
		t.Fatal("Active handshakes map should not be nil")
	}

	if len(handshake.activeHandshakes) != 0 {
		t.Error("Active handshakes should be empty initially")
	}
}

func TestAgentHandshake_SetKeyDerivation(t *testing.T) {
	meshName := "test-mesh"
	nodeID := "test-node-123"
	identityManager := identity.NewIdentityManager(meshName, nodeID)
	handshake := NewAgentHandshake(meshName, nodeID, identityManager)

	keyDerivation := crypto.NewMobileKeyDerivation("passphrase", "device-secret", nodeID)
	handshake.SetKeyDerivation(keyDerivation)

	if handshake.keyDerivation != keyDerivation {
		t.Error("Key derivation should be set correctly")
	}
}

func TestAgentHandshake_InitiateHandshake(t *testing.T) {
	meshName := "source-mesh"
	nodeID := "test-node-123"
	identityManager := identity.NewIdentityManager(meshName, nodeID)
	handshake := NewAgentHandshake(meshName, nodeID, identityManager)

	// Set up key derivation
	keyDerivation := crypto.NewMobileKeyDerivation("passphrase", "device-secret", nodeID)
	handshake.SetKeyDerivation(keyDerivation)

	targetMesh := "target-mesh"
	targetAddress := "192.168.1.100:5000"

	// Create mock connection
	mockConn := &mockConnection{address: targetAddress}

	// Test successful handshake initiation
	session, err := handshake.InitiateHandshake(targetMesh, targetAddress, mockConn)
	if err != nil {
		t.Fatalf("Failed to initiate handshake: %v", err)
	}

	if session == nil {
		t.Fatal("Session should not be nil")
	}

	if session.RemoteMesh != targetMesh {
		t.Errorf("Expected remote mesh %s, got %s", targetMesh, session.RemoteMesh)
	}

	if session.RemoteAddress != targetAddress {
		t.Errorf("Expected remote address %s, got %s", targetAddress, session.RemoteAddress)
	}

	if session.Status != "waiting_response" {
		t.Errorf("Expected status 'waiting_response', got %s", session.Status)
	}

	if session.Connection.RemoteAddr().String() != mockConn.address {
		t.Error("Connection should match provided connection")
	}

	if session.MobileIdentity == nil {
		t.Fatal("Mobile identity should not be nil")
	}

	// Verify session is tracked
	retrievedSession := handshake.GetHandshakeSession(session.SessionID)
	if retrievedSession != session {
		t.Error("Session should be retrievable by session ID")
	}

	// Test initiation without key derivation
	handshakeNoKeys := NewAgentHandshake(meshName, nodeID, identityManager)
	_, err = handshakeNoKeys.InitiateHandshake(targetMesh, targetAddress, mockConn)
	if err == nil {
		t.Error("Expected error when key derivation is not set")
	}
}

func TestAgentHandshake_HandleIncomingHandshake(t *testing.T) {
	meshName := "target-mesh"
	nodeID := "test-node-123"
	identityManager := identity.NewIdentityManager(meshName, nodeID)
	handshake := NewAgentHandshake(meshName, nodeID, identityManager)

	// Create valid handshake request
	request := &HandshakeRequest{
		SessionID:     "test-session-456",
		AgentIdentity: "remote-agent-789",
		SourceMesh:    "source-mesh",
		TargetMesh:    meshName,
		PublicKey:     "12345678901234567890",
		RequestType:   "bridge",
		Timestamp:     time.Now().Unix(),
	}

	mockConn := &mockConnection{address: "192.168.1.200:5000"}

	// Test successful incoming handshake handling
	response, err := handshake.HandleIncomingHandshake(mockConn, request)
	if err != nil {
		t.Fatalf("Failed to handle incoming handshake: %v", err)
	}

	if response == nil {
		t.Fatal("Response should not be nil")
	}

	if response.SessionID != request.SessionID {
		t.Errorf("Expected session ID %s, got %s", request.SessionID, response.SessionID)
	}

	if response.Status != "challenge" {
		t.Errorf("Expected status 'challenge', got %s", response.Status)
	}

	if response.MeshName != meshName {
		t.Errorf("Expected mesh name %s, got %s", meshName, response.MeshName)
	}

	if response.Challenge == nil {
		t.Fatal("Challenge should not be nil")
	}

	// Verify session was created
	session := handshake.GetHandshakeSession(request.SessionID)
	if session == nil {
		t.Fatal("Handshake session should be created")
	}

	if session.Status != "challenge_sent" {
		t.Errorf("Expected session status 'challenge_sent', got %s", session.Status)
	}

	// Test invalid handshake request
	invalidRequest := &HandshakeRequest{
		SessionID:     "invalid-session",
		AgentIdentity: "",
		SourceMesh:    "",
		TargetMesh:    "wrong-mesh",
		PublicKey:     "",
		RequestType:   "invalid",
		Timestamp:     time.Now().Unix(),
	}

	response, err = handshake.HandleIncomingHandshake(mockConn, invalidRequest)
	if err != nil {
		t.Fatalf("Handler should not return error for invalid request: %v", err)
	}

	if response.Status != "rejected" {
		t.Errorf("Expected status 'rejected' for invalid request, got %s", response.Status)
	}

	if response.ErrorMessage == "" {
		t.Error("Error message should not be empty for rejected request")
	}
}

func TestAgentHandshake_ProcessHandshakeResponse(t *testing.T) {
	meshName := "source-mesh"
	nodeID := "test-node-123"
	identityManager := identity.NewIdentityManager(meshName, nodeID)
	handshake := NewAgentHandshake(meshName, nodeID, identityManager)

	keyDerivation := crypto.NewMobileKeyDerivation("passphrase", "device-secret", nodeID)
	handshake.SetKeyDerivation(keyDerivation)

	// Create a handshake session
	mockConn := &mockConnection{address: "192.168.1.100:5000"}
	session, err := handshake.InitiateHandshake("target-mesh", "192.168.1.100:5000", mockConn)
	if err != nil {
		t.Fatalf("Failed to create handshake session: %v", err)
	}

	// Build a REAL challenge the way a remote mesh would: encrypt against the
	// public key this agent presented for the target mesh. Fabricated ciphertext
	// can't be decrypted by RespondToChallenge, so the challenge must be genuine.
	challenger := security.NewAuthenticationSession(
		session.MobileIdentity.Identity.ID,
		session.MobileIdentity.KeyPair.Public,
	)
	realChallenge, err := challenger.CreateChallenge()
	if err != nil {
		t.Fatalf("Failed to create challenge: %v", err)
	}

	// Test challenge response
	challengeResponse := &HandshakeResponse{
		SessionID: session.SessionID,
		Status:    "challenge",
		MeshName:  "target-mesh",
		Challenge: realChallenge,
		Timestamp: time.Now().Unix(),
	}

	complete, err := handshake.ProcessHandshakeResponse(session.SessionID, challengeResponse)
	if err != nil {
		t.Fatalf("Failed to process challenge response: %v", err)
	}

	if complete == nil {
		t.Fatal("Completion should not be nil")
	}

	if complete.SessionID != session.SessionID {
		t.Errorf("Expected session ID %s, got %s", session.SessionID, complete.SessionID)
	}

	// Verify session status was updated
	updatedSession := handshake.GetHandshakeSession(session.SessionID)
	if updatedSession.Status != "auth_sent" {
		t.Errorf("Expected session status 'auth_sent', got %s", updatedSession.Status)
	}

	// Test rejected response
	rejectedResponse := &HandshakeResponse{
		SessionID:    session.SessionID,
		Status:       "rejected",
		MeshName:     "target-mesh",
		ErrorMessage: "Access denied",
		Timestamp:    time.Now().Unix(),
	}

	complete, err = handshake.ProcessHandshakeResponse(session.SessionID, rejectedResponse)
	if err == nil {
		t.Error("Expected error for rejected response")
	}

	if complete == nil {
		t.Fatal("Completion should not be nil even for rejected response")
	}

	if complete.Authenticated {
		t.Error("Rejected response should not be authenticated")
	}

	// Test non-existent session
	_, err = handshake.ProcessHandshakeResponse("non-existent-session", challengeResponse)
	if err == nil {
		t.Error("Expected error for non-existent session")
	}
}

func TestAgentHandshake_SessionManagement(t *testing.T) {
	meshName := "test-mesh"
	nodeID := "test-node-123"
	identityManager := identity.NewIdentityManager(meshName, nodeID)
	handshake := NewAgentHandshake(meshName, nodeID, identityManager)

	keyDerivation := crypto.NewMobileKeyDerivation("passphrase", "device-secret", nodeID)
	handshake.SetKeyDerivation(keyDerivation)

	// Create multiple handshake sessions
	mockConn1 := &mockConnection{address: "192.168.1.100:5000"}
	mockConn2 := &mockConnection{address: "192.168.1.101:5000"}

	session1, err := handshake.InitiateHandshake("target-mesh-1", "192.168.1.100:5000", mockConn1)
	if err != nil {
		t.Fatalf("Failed to create first session: %v", err)
	}

	session2, err := handshake.InitiateHandshake("target-mesh-2", "192.168.1.101:5000", mockConn2)
	if err != nil {
		t.Fatalf("Failed to create second session: %v", err)
	}

	// Test listing active handshakes
	activeSessions := handshake.ListActiveHandshakes()
	if len(activeSessions) != 2 {
		t.Errorf("Expected 2 active sessions, got %d", len(activeSessions))
	}

	if activeSessions[session1.SessionID] == nil {
		t.Error("First session should be in active list")
	}

	if activeSessions[session2.SessionID] == nil {
		t.Error("Second session should be in active list")
	}

	// Test closing a session
	err = handshake.CloseHandshake(session1.SessionID)
	if err != nil {
		t.Fatalf("Failed to close handshake: %v", err)
	}

	// Verify session was removed
	activeSessions = handshake.ListActiveHandshakes()
	if len(activeSessions) != 1 {
		t.Errorf("Expected 1 active session after closure, got %d", len(activeSessions))
	}

	if activeSessions[session1.SessionID] != nil {
		t.Error("Closed session should not be in active list")
	}

	// Test closing non-existent session
	err = handshake.CloseHandshake("non-existent-session")
	if err == nil {
		t.Error("Expected error for closing non-existent session")
	}
}

func TestAgentHandshake_CleanupExpiredHandshakes(t *testing.T) {
	meshName := "test-mesh"
	nodeID := "test-node-123"
	identityManager := identity.NewIdentityManager(meshName, nodeID)
	handshake := NewAgentHandshake(meshName, nodeID, identityManager)

	keyDerivation := crypto.NewMobileKeyDerivation("passphrase", "device-secret", nodeID)
	handshake.SetKeyDerivation(keyDerivation)

	// Create handshake sessions
	mockConn := &mockConnection{address: "192.168.1.100:5000"}
	session, err := handshake.InitiateHandshake("target-mesh", "192.168.1.100:5000", mockConn)
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}

	// Artificially age the session
	handshake.mu.Lock()
	handshake.activeHandshakes[session.SessionID].LastActivity = time.Now().UTC().Add(-2 * time.Hour)
	handshake.mu.Unlock()

	// Cleanup with 1 hour max age
	handshake.CleanupExpiredHandshakes(1 * time.Hour)

	// Session should be removed
	activeSessions := handshake.ListActiveHandshakes()
	if len(activeSessions) != 0 {
		t.Errorf("Expected 0 active sessions after cleanup, got %d", len(activeSessions))
	}

	// Mock connection should be closed
	if !mockConn.closed {
		t.Error("Expired session connection should be closed")
	}
}

func TestValidateHandshakeRequest(t *testing.T) {
	meshName := "test-mesh"
	nodeID := "test-node-123"
	identityManager := identity.NewIdentityManager(meshName, nodeID)
	handshake := NewAgentHandshake(meshName, nodeID, identityManager)

	// Valid request
	validRequest := &HandshakeRequest{
		SessionID:     "test-session-123",
		AgentIdentity: "agent-456",
		SourceMesh:    "source-mesh",
		TargetMesh:    meshName,
		PublicKey:     "public-key-data",
		RequestType:   "bridge",
		Timestamp:     time.Now().Unix(),
	}

	err := handshake.validateHandshakeRequest(validRequest)
	if err != nil {
		t.Fatalf("Valid request should pass validation: %v", err)
	}

	// Test various invalid requests
	testCases := []struct {
		name    string
		request *HandshakeRequest
	}{
		{
			name: "empty session ID",
			request: &HandshakeRequest{
				SessionID:     "",
				AgentIdentity: "agent-456",
				SourceMesh:    "source-mesh",
				TargetMesh:    meshName,
				PublicKey:     "public-key-data",
				RequestType:   "bridge",
			},
		},
		{
			name: "empty agent identity",
			request: &HandshakeRequest{
				SessionID:     "test-session-123",
				AgentIdentity: "",
				SourceMesh:    "source-mesh",
				TargetMesh:    meshName,
				PublicKey:     "public-key-data",
				RequestType:   "bridge",
			},
		},
		{
			name: "empty source mesh",
			request: &HandshakeRequest{
				SessionID:     "test-session-123",
				AgentIdentity: "agent-456",
				SourceMesh:    "",
				TargetMesh:    meshName,
				PublicKey:     "public-key-data",
				RequestType:   "bridge",
			},
		},
		{
			name: "wrong target mesh",
			request: &HandshakeRequest{
				SessionID:     "test-session-123",
				AgentIdentity: "agent-456",
				SourceMesh:    "source-mesh",
				TargetMesh:    "wrong-mesh",
				PublicKey:     "public-key-data",
				RequestType:   "bridge",
			},
		},
		{
			name: "empty public key",
			request: &HandshakeRequest{
				SessionID:     "test-session-123",
				AgentIdentity: "agent-456",
				SourceMesh:    "source-mesh",
				TargetMesh:    meshName,
				PublicKey:     "",
				RequestType:   "bridge",
			},
		},
		{
			name: "unsupported request type",
			request: &HandshakeRequest{
				SessionID:     "test-session-123",
				AgentIdentity: "agent-456",
				SourceMesh:    "source-mesh",
				TargetMesh:    meshName,
				PublicKey:     "public-key-data",
				RequestType:   "unsupported",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := handshake.validateHandshakeRequest(tc.request)
			if err == nil {
				t.Errorf("Expected error for %s", tc.name)
			}
		})
	}
}

func TestHandshakeSession_IsActive(t *testing.T) {
	session := &HandshakeSession{
		SessionID: "test-session",
		Status:    "initiating",
	}

	if !session.IsActive() {
		t.Error("Session with 'initiating' status should be active")
	}

	session.Status = "completed"
	if session.IsActive() {
		t.Error("Session with 'completed' status should not be active")
	}

	session.Status = "failed"
	if session.IsActive() {
		t.Error("Session with 'failed' status should not be active")
	}

	session.Status = "closed"
	if session.IsActive() {
		t.Error("Session with 'closed' status should not be active")
	}
}

func TestHandshakeSession_IsAuthenticated(t *testing.T) {
	session := &HandshakeSession{
		SessionID: "test-session",
		Status:    "initiating",
	}

	if session.IsAuthenticated() {
		t.Error("Session with 'initiating' status should not be authenticated")
	}

	session.Status = "authenticated"
	if !session.IsAuthenticated() {
		t.Error("Session with 'authenticated' status should be authenticated")
	}

	session.Status = "completed"
	if !session.IsAuthenticated() {
		t.Error("Session with 'completed' status should be authenticated")
	}
}

// Mock connection for testing
type mockConnection struct {
	address string
	closed  bool
}

func (mc *mockConnection) Read(b []byte) (n int, err error) {
	return 0, nil
}

func (mc *mockConnection) Write(b []byte) (n int, err error) {
	return len(b), nil
}

func (mc *mockConnection) Close() error {
	mc.closed = true
	return nil
}

func (mc *mockConnection) LocalAddr() net.Addr {
	return &mockAddr{address: "local-" + mc.address}
}

func (mc *mockConnection) RemoteAddr() net.Addr {
	return &mockAddr{address: mc.address}
}

// mockAddr implements net.Addr interface
type mockAddr struct {
	address string
}

func (ma *mockAddr) Network() string {
	return "tcp"
}

func (ma *mockAddr) String() string {
	return ma.address
}

func (mc *mockConnection) SetDeadline(t time.Time) error {
	return nil
}

func (mc *mockConnection) SetReadDeadline(t time.Time) error {
	return nil
}

func (mc *mockConnection) SetWriteDeadline(t time.Time) error {
	return nil
}