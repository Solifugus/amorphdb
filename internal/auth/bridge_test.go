// Package auth provides bridge authentication tests
package auth

import (
	"testing"
	"time"

	"github.com/solifugus/amorphdb/internal/identity"
	"github.com/solifugus/amorphdb/internal/types"
)

func TestBridgeAuth_CreateMobileIdentity(t *testing.T) {
	bridgeAuth := NewBridgeAuth()

	// Create test base identity
	baseIdentity := &identity.Identity{
		ID:       "test-agent-123",
		Type:     types.MobileAgent,
		MeshName: "source-mesh",
		NodeID:   "node-456",
		Created:  time.Now().UTC(),
	}

	meshName := "target-mesh"
	passphrase := "test-passphrase"
	deviceSecret := "test-device-secret"

	// Test successful identity creation
	mobileIdentity, err := bridgeAuth.CreateMobileIdentity(meshName, passphrase, deviceSecret, baseIdentity)
	if err != nil {
		t.Fatalf("Failed to create mobile identity: %v", err)
	}

	if mobileIdentity == nil {
		t.Fatal("Mobile identity should not be nil")
	}

	if mobileIdentity.MeshName != meshName {
		t.Errorf("Expected mesh name %s, got %s", meshName, mobileIdentity.MeshName)
	}

	if mobileIdentity.Passphrase != passphrase {
		t.Errorf("Expected passphrase %s, got %s", passphrase, mobileIdentity.Passphrase)
	}

	if mobileIdentity.DeviceSecret != deviceSecret {
		t.Errorf("Expected device secret %s, got %s", deviceSecret, mobileIdentity.DeviceSecret)
	}

	// Test duplicate creation returns existing identity
	duplicate, err := bridgeAuth.CreateMobileIdentity(meshName, passphrase, deviceSecret, baseIdentity)
	if err != nil {
		t.Fatalf("Failed to get existing mobile identity: %v", err)
	}

	if duplicate != mobileIdentity {
		t.Error("Expected duplicate creation to return existing identity")
	}

	// Test key pair is properly derived
	if mobileIdentity.KeyPair == nil {
		t.Fatal("Key pair should not be nil")
	}

	if mobileIdentity.KeyPair.Public == nil || mobileIdentity.KeyPair.Private == nil {
		t.Fatal("Public and private keys should not be nil")
	}

	// Test derived key is created
	derivedKey := bridgeAuth.GetDerivedKey(meshName)
	if derivedKey == nil {
		t.Fatal("Derived key should not be nil")
	}

	if derivedKey.PublicKey.Cmp(mobileIdentity.KeyPair.Public) != 0 {
		t.Error("Derived key public key should match mobile identity public key")
	}
}

func TestBridgeAuth_AuthenticateToMesh(t *testing.T) {
	bridgeAuth := NewBridgeAuth()

	// Setup mobile identity
	baseIdentity := &identity.Identity{
		ID:       "test-agent-123",
		Type:     types.MobileAgent,
		MeshName: "source-mesh",
		NodeID:   "node-456",
		Created:  time.Now().UTC(),
	}

	meshName := "target-mesh"
	sessionID := "test-session-789"

	_, err := bridgeAuth.CreateMobileIdentity(meshName, "passphrase", "device-secret", baseIdentity)
	if err != nil {
		t.Fatalf("Failed to create mobile identity: %v", err)
	}

	// Test successful authentication session creation
	session, err := bridgeAuth.AuthenticateToMesh(meshName, sessionID, "mock-connection")
	if err != nil {
		t.Fatalf("Failed to create authentication session: %v", err)
	}

	if session == nil {
		t.Fatal("Session should not be nil")
	}

	if session.SessionID != sessionID {
		t.Errorf("Expected session ID %s, got %s", sessionID, session.SessionID)
	}

	if session.MeshName != meshName {
		t.Errorf("Expected mesh name %s, got %s", meshName, session.MeshName)
	}

	if session.Status != "authenticating" {
		t.Errorf("Expected status 'authenticating', got %s", session.Status)
	}

	// Test duplicate session creation returns existing session
	duplicate, err := bridgeAuth.AuthenticateToMesh(meshName, sessionID, "mock-connection")
	if err != nil {
		t.Fatalf("Failed to get existing session: %v", err)
	}

	if duplicate != session {
		t.Error("Expected duplicate creation to return existing session")
	}

	// Test authentication with non-existent mesh
	_, err = bridgeAuth.AuthenticateToMesh("non-existent-mesh", "test-session-2", "mock-connection")
	if err == nil {
		t.Error("Expected error for non-existent mesh")
	}
}

func TestBridgeAuth_ChallengeResponse(t *testing.T) {
	bridgeAuth := NewBridgeAuth()

	// Setup mobile identity
	baseIdentity := &identity.Identity{
		ID:       "test-agent-123",
		Type:     types.MobileAgent,
		MeshName: "source-mesh",
		NodeID:   "node-456",
		Created:  time.Now().UTC(),
	}

	meshName := "target-mesh"
	sessionID := "test-session-789"

	mobileIdentity, err := bridgeAuth.CreateMobileIdentity(meshName, "passphrase", "device-secret", baseIdentity)
	if err != nil {
		t.Fatalf("Failed to create mobile identity: %v", err)
	}

	// Create authentication session
	_, err = bridgeAuth.AuthenticateToMesh(meshName, sessionID, "mock-connection")
	if err != nil {
		t.Fatalf("Failed to create authentication session: %v", err)
	}

	// Test challenge creation
	challenge, err := bridgeAuth.CreateChallenge(sessionID)
	if err != nil {
		t.Fatalf("Failed to create challenge: %v", err)
	}

	if challenge == nil {
		t.Fatal("Challenge should not be nil")
	}

	if challenge.AgentIdentity != mobileIdentity.Identity.ID {
		t.Errorf("Expected agent identity %s, got %s", mobileIdentity.Identity.ID, challenge.AgentIdentity)
	}

	if len(challenge.ChallengeID) == 0 {
		t.Error("Challenge ID should not be empty")
	}

	if len(challenge.EncryptedData) == 0 {
		t.Error("Encrypted data should not be empty")
	}

	// Test challenge response
	response, err := bridgeAuth.RespondToChallenge(challenge, meshName)
	if err != nil {
		t.Fatalf("Failed to respond to challenge: %v", err)
	}

	if response == nil {
		t.Fatal("Response should not be nil")
	}

	if response.AgentIdentity != challenge.AgentIdentity {
		t.Errorf("Expected response identity %s, got %s", challenge.AgentIdentity, response.AgentIdentity)
	}

	if string(response.ChallengeID) != string(challenge.ChallengeID) {
		t.Error("Challenge ID in response should match challenge")
	}

	// Test response verification
	authenticated, err := bridgeAuth.VerifyResponse(sessionID, response)
	if err != nil {
		t.Fatalf("Failed to verify response: %v", err)
	}

	if !authenticated {
		t.Error("Authentication should have succeeded")
	}

	// Check session status was updated
	updatedSession := bridgeAuth.GetSession(sessionID)
	if updatedSession.Status != "authenticated" {
		t.Errorf("Expected session status 'authenticated', got %s", updatedSession.Status)
	}
}

func TestBridgeAuth_SessionManagement(t *testing.T) {
	bridgeAuth := NewBridgeAuth()

	// Setup mobile identity
	baseIdentity := &identity.Identity{
		ID:       "test-agent-123",
		Type:     types.MobileAgent,
		MeshName: "source-mesh",
		NodeID:   "node-456",
		Created:  time.Now().UTC(),
	}

	meshName := "target-mesh"
	sessionID := "test-session-789"

	_, err := bridgeAuth.CreateMobileIdentity(meshName, "passphrase", "device-secret", baseIdentity)
	if err != nil {
		t.Fatalf("Failed to create mobile identity: %v", err)
	}

	// Create session
	session, err := bridgeAuth.AuthenticateToMesh(meshName, sessionID, "mock-connection")
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}

	// Test session retrieval
	retrievedSession := bridgeAuth.GetSession(sessionID)
	if retrievedSession != session {
		t.Error("Retrieved session should match created session")
	}

	// Test listing active sessions
	activeSessions := bridgeAuth.ListActiveSessions()
	if len(activeSessions) != 1 {
		t.Errorf("Expected 1 active session, got %d", len(activeSessions))
	}

	if activeSessions[sessionID] != session {
		t.Error("Active sessions should contain created session")
	}

	// Test session closure
	err = bridgeAuth.CloseSession(sessionID)
	if err != nil {
		t.Fatalf("Failed to close session: %v", err)
	}

	// Session should no longer be active
	activeSessions = bridgeAuth.ListActiveSessions()
	if len(activeSessions) != 0 {
		t.Errorf("Expected 0 active sessions after closure, got %d", len(activeSessions))
	}

	// Test closing non-existent session
	err = bridgeAuth.CloseSession("non-existent-session")
	if err == nil {
		t.Error("Expected error for closing non-existent session")
	}
}

func TestBridgeAuth_ValidateMobileIdentity(t *testing.T) {
	bridgeAuth := NewBridgeAuth()

	// Test validation with no identity
	err := bridgeAuth.ValidateMobileIdentity("non-existent-mesh")
	if err == nil {
		t.Error("Expected error for non-existent mesh")
	}

	// Create mobile identity
	baseIdentity := &identity.Identity{
		ID:       "test-agent-123",
		Type:     types.MobileAgent,
		MeshName: "source-mesh",
		NodeID:   "node-456",
		Created:  time.Now().UTC(),
	}

	meshName := "target-mesh"
	_, err = bridgeAuth.CreateMobileIdentity(meshName, "passphrase", "device-secret", baseIdentity)
	if err != nil {
		t.Fatalf("Failed to create mobile identity: %v", err)
	}

	// Test successful validation
	err = bridgeAuth.ValidateMobileIdentity(meshName)
	if err != nil {
		t.Fatalf("Mobile identity validation should have succeeded: %v", err)
	}
}

func TestBridgeAuth_RemoveMobileIdentity(t *testing.T) {
	bridgeAuth := NewBridgeAuth()

	// Create mobile identity and session
	baseIdentity := &identity.Identity{
		ID:       "test-agent-123",
		Type:     types.MobileAgent,
		MeshName: "source-mesh",
		NodeID:   "node-456",
	}

	meshName := "target-mesh"
	sessionID := "test-session-789"

	_, err := bridgeAuth.CreateMobileIdentity(meshName, "passphrase", "device-secret", baseIdentity)
	if err != nil {
		t.Fatalf("Failed to create mobile identity: %v", err)
	}

	_, err = bridgeAuth.AuthenticateToMesh(meshName, sessionID, "mock-connection")
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}

	// Verify identity and session exist
	identity := bridgeAuth.GetMobileIdentity(meshName)
	if identity == nil {
		t.Fatal("Mobile identity should exist before removal")
	}

	session := bridgeAuth.GetSession(sessionID)
	if session == nil {
		t.Fatal("Session should exist before removal")
	}

	// Remove mobile identity
	err = bridgeAuth.RemoveMobileIdentity(meshName)
	if err != nil {
		t.Fatalf("Failed to remove mobile identity: %v", err)
	}

	// Verify identity and session are removed
	identity = bridgeAuth.GetMobileIdentity(meshName)
	if identity != nil {
		t.Error("Mobile identity should be removed")
	}

	key := bridgeAuth.GetDerivedKey(meshName)
	if key != nil {
		t.Error("Derived key should be removed")
	}

	session = bridgeAuth.GetSession(sessionID)
	if session != nil {
		t.Error("Session should be removed")
	}
}

func TestMobileIdentity_Validate(t *testing.T) {
	// Test nil identity
	var nilIdentity *MobileIdentity
	err := nilIdentity.Validate()
	if err == nil {
		t.Error("Expected error for nil mobile identity")
	}

	// Create valid mobile identity
	baseIdentity := &identity.Identity{
		ID:       "test-agent-123",
		Type:     types.MobileAgent,
		MeshName: "source-mesh",
		NodeID:   "node-456",
		Created:  time.Now().UTC(),
	}

	bridgeAuth := NewBridgeAuth()
	mobileIdentity, err := bridgeAuth.CreateMobileIdentity("target-mesh", "passphrase", "device-secret", baseIdentity)
	if err != nil {
		t.Fatalf("Failed to create mobile identity: %v", err)
	}

	// Test successful validation
	err = mobileIdentity.Validate()
	if err != nil {
		t.Fatalf("Valid mobile identity should pass validation: %v", err)
	}

	// Test validation with empty passphrase
	mobileIdentity.Passphrase = ""
	err = mobileIdentity.Validate()
	if err == nil {
		t.Error("Expected error for empty passphrase")
	}

	// Test validation with empty device secret
	mobileIdentity.Passphrase = "passphrase"
	mobileIdentity.DeviceSecret = ""
	err = mobileIdentity.Validate()
	if err == nil {
		t.Error("Expected error for empty device secret")
	}

	// Test validation with empty mesh name
	mobileIdentity.DeviceSecret = "device-secret"
	mobileIdentity.MeshName = ""
	err = mobileIdentity.Validate()
	if err == nil {
		t.Error("Expected error for empty mesh name")
	}
}