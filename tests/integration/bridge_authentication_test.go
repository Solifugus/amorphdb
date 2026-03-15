// Package integration provides integration tests for AmorphDB bridge authentication
package integration

import (
	"fmt"
	"testing"

	"github.com/solifugus/amorphdb/internal/auth"
	"github.com/solifugus/amorphdb/internal/crypto"
	"github.com/solifugus/amorphdb/internal/identity"
	"github.com/solifugus/amorphdb/internal/mesh"
)

// TestBridgeAuthentication_EndToEnd demonstrates complete bridge authentication workflow
func TestBridgeAuthentication_EndToEnd(t *testing.T) {
	// Setup source mesh (initiating bridge connection)
	sourceMeshName := "source-mesh"
	sourceNodeID := "source-node-123"
	sourceIdentityManager := identity.NewIdentityManager(sourceMeshName, sourceNodeID)

	// Generate source node identity
	_, err := sourceIdentityManager.GenerateNodeIdentity()
	if err != nil {
		t.Fatalf("Failed to generate source node identity: %v", err)
	}

	// Setup target mesh (accepting bridge connection)
	targetMeshName := "target-mesh"
	targetNodeID := "target-node-456"
	targetIdentityManager := identity.NewIdentityManager(targetMeshName, targetNodeID)

	// Generate target node identity
	_, err = targetIdentityManager.GenerateNodeIdentity()
	if err != nil {
		t.Fatalf("Failed to generate target node identity: %v", err)
	}

	// Step 1: Create bridge authentication manager for source mesh
	sourceBridgeAuth := auth.NewBridgeAuth()

	// Create mobile identity for connecting to target mesh
	mobileBridgeIdentity, err := sourceIdentityManager.CreateBridgeIdentity(targetMeshName)
	if err != nil {
		t.Fatalf("Failed to create bridge identity: %v", err)
	}

	// Step 2: Set up mobile agent key derivation
	passphrase := "bridge-auth-passphrase"
	deviceSecret := "device-secret-" + sourceNodeID

	mobileKeyDerivation := crypto.NewMobileKeyDerivation(passphrase, deviceSecret, sourceNodeID)

	// Derive key pair for target mesh
	meshKeyPair, err := mobileKeyDerivation.DeriveKeyPairForMesh(targetMeshName)
	if err != nil {
		t.Fatalf("Failed to derive key pair for target mesh: %v", err)
	}

	// Step 3: Create mobile identity in bridge auth
	mobileIdentity, err := sourceBridgeAuth.CreateMobileIdentity(targetMeshName, passphrase, deviceSecret, mobileBridgeIdentity)
	if err != nil {
		t.Fatalf("Failed to create mobile identity: %v", err)
	}

	// Validate mobile identity
	err = sourceBridgeAuth.ValidateMobileIdentity(targetMeshName)
	if err != nil {
		t.Fatalf("Mobile identity validation failed: %v", err)
	}

	// Step 4: Set up target mesh bridge authentication
	targetBridgeAuth := auth.NewBridgeAuth()

	// Create a reverse mobile identity for potential bidirectional auth
	reverseBridgeIdentity, err := targetIdentityManager.CreateBridgeIdentity(sourceMeshName)
	if err != nil {
		t.Fatalf("Failed to create reverse bridge identity: %v", err)
	}

	_, err = targetBridgeAuth.CreateMobileIdentity(sourceMeshName, passphrase, deviceSecret, reverseBridgeIdentity)
	if err != nil {
		t.Fatalf("Failed to create reverse mobile identity: %v", err)
	}

	// Step 5: Simulate bridge authentication handshake
	sessionID := "bridge-auth-session-789"

	// Source initiates authentication to target mesh
	authSession, err := sourceBridgeAuth.AuthenticateToMesh(targetMeshName, sessionID, "mock-connection")
	if err != nil {
		t.Fatalf("Failed to initiate authentication: %v", err)
	}

	if authSession.Status != "authenticating" {
		t.Errorf("Expected session status 'authenticating', got %s", authSession.Status)
	}

	// Step 6: Create authentication challenge
	challenge, err := sourceBridgeAuth.CreateChallenge(sessionID)
	if err != nil {
		t.Fatalf("Failed to create challenge: %v", err)
	}

	if challenge.AgentIdentity != mobileIdentity.Identity.ID {
		t.Errorf("Expected challenge for agent %s, got %s", mobileIdentity.Identity.ID, challenge.AgentIdentity)
	}

	// Step 7: Respond to challenge using mobile agent credentials
	response, err := sourceBridgeAuth.RespondToChallenge(challenge, targetMeshName)
	if err != nil {
		t.Fatalf("Failed to respond to challenge: %v", err)
	}

	if response.AgentIdentity != challenge.AgentIdentity {
		t.Errorf("Response agent identity should match challenge")
	}

	// Step 8: Verify authentication response
	authenticated, err := sourceBridgeAuth.VerifyResponse(sessionID, response)
	if err != nil {
		t.Fatalf("Failed to verify response: %v", err)
	}

	if !authenticated {
		t.Error("Authentication should have succeeded")
	}

	// Step 9: Verify session is now authenticated
	finalSession := sourceBridgeAuth.GetSession(sessionID)
	if finalSession.Status != "authenticated" {
		t.Errorf("Expected final session status 'authenticated', got %s", finalSession.Status)
	}

	// Step 10: Test key derivation consistency
	// Verify that we can re-derive the same keys
	reDerivedKeyPair, err := mobileKeyDerivation.DeriveKeyPairForMesh(targetMeshName)
	if err != nil {
		t.Fatalf("Failed to re-derive key pair: %v", err)
	}

	if meshKeyPair.PublicKey.Cmp(reDerivedKeyPair.PublicKey) != 0 {
		t.Error("Re-derived public key should match original")
	}

	if meshKeyPair.PrivateKey.Cmp(reDerivedKeyPair.PrivateKey) != 0 {
		t.Error("Re-derived private key should match original")
	}

	// Step 11: Validate derived key against authentication
	err = mobileKeyDerivation.ValidateKeyPairForMesh(meshKeyPair)
	if err != nil {
		t.Fatalf("Key pair validation failed: %v", err)
	}

	// Step 12: Test multiple mesh key derivation
	multipleMeshes := []string{targetMeshName, "additional-mesh-1", "additional-mesh-2"}
	multipleKeys, err := mobileKeyDerivation.DeriveMultipleMeshKeys(multipleMeshes)
	if err != nil {
		t.Fatalf("Failed to derive multiple mesh keys: %v", err)
	}

	if len(multipleKeys) != len(multipleMeshes) {
		t.Errorf("Expected %d key pairs, got %d", len(multipleMeshes), len(multipleKeys))
	}

	for _, meshName := range multipleMeshes {
		if keyPair, exists := multipleKeys[meshName]; !exists {
			t.Errorf("Missing key pair for mesh %s", meshName)
		} else if keyPair.MeshName != meshName {
			t.Errorf("Key pair mesh name mismatch: expected %s, got %s", meshName, keyPair.MeshName)
		}
	}

	// Step 13: Clean up session
	err = sourceBridgeAuth.CloseSession(sessionID)
	if err != nil {
		t.Fatalf("Failed to close session: %v", err)
	}

	// Session should no longer be active
	activeSessions := sourceBridgeAuth.ListActiveSessions()
	if len(activeSessions) != 0 {
		t.Errorf("Expected 0 active sessions after cleanup, got %d", len(activeSessions))
	}
}

// TestBridgeAuthentication_MultipleMeshes tests authentication across multiple meshes
func TestBridgeAuthentication_MultipleMeshes(t *testing.T) {
	// Setup source mesh
	sourceMeshName := "hub-mesh"
	sourceNodeID := "hub-node-001"
	sourceIdentityManager := identity.NewIdentityManager(sourceMeshName, sourceNodeID)

	_, err := sourceIdentityManager.GenerateNodeIdentity()
	if err != nil {
		t.Fatalf("Failed to generate source node identity: %v", err)
	}

	// Setup bridge authentication
	bridgeAuth := auth.NewBridgeAuth()

	// Setup key derivation
	passphrase := "multi-mesh-passphrase"
	deviceSecret := "multi-device-secret"
	keyDerivation := crypto.NewMobileKeyDerivation(passphrase, deviceSecret, sourceNodeID)

	// Define target meshes
	targetMeshes := []string{"mesh-alpha", "mesh-beta", "mesh-gamma"}

	// Create mobile identities for all target meshes
	mobileIdentities := make(map[string]*auth.MobileIdentity)

	for _, targetMesh := range targetMeshes {
		// Create bridge identity
		bridgeIdentity, err := sourceIdentityManager.CreateBridgeIdentity(targetMesh)
		if err != nil {
			t.Fatalf("Failed to create bridge identity for mesh %s: %v", targetMesh, err)
		}

		// Create mobile identity
		mobileIdentity, err := bridgeAuth.CreateMobileIdentity(targetMesh, passphrase, deviceSecret, bridgeIdentity)
		if err != nil {
			t.Fatalf("Failed to create mobile identity for mesh %s: %v", targetMesh, err)
		}

		mobileIdentities[targetMesh] = mobileIdentity

		// Validate mobile identity
		err = bridgeAuth.ValidateMobileIdentity(targetMesh)
		if err != nil {
			t.Fatalf("Mobile identity validation failed for mesh %s: %v", targetMesh, err)
		}
	}

	// Test that each mesh has different keys
	meshKeys := make(map[string]*crypto.MeshKeyPair)

	for _, targetMesh := range targetMeshes {
		keyPair, err := keyDerivation.DeriveKeyPairForMesh(targetMesh)
		if err != nil {
			t.Fatalf("Failed to derive key pair for mesh %s: %v", targetMesh, err)
		}

		meshKeys[targetMesh] = keyPair
	}

	// Verify all keys are different
	keyList := make([]*crypto.MeshKeyPair, 0, len(meshKeys))
	for _, keyPair := range meshKeys {
		keyList = append(keyList, keyPair)
	}

	for i := 0; i < len(keyList); i++ {
		for j := i + 1; j < len(keyList); j++ {
			if keyList[i].PublicKey.Cmp(keyList[j].PublicKey) == 0 {
				t.Errorf("Key pairs for meshes %s and %s have identical public keys",
					keyList[i].MeshName, keyList[j].MeshName)
			}
		}
	}

	// Test concurrent authentication to multiple meshes
	for i, targetMesh := range targetMeshes {
		sessionID := fmt.Sprintf("multi-mesh-session-%d", i)

		// Initiate authentication
		session, err := bridgeAuth.AuthenticateToMesh(targetMesh, sessionID, "mock-connection")
		if err != nil {
			t.Fatalf("Failed to authenticate to mesh %s: %v", targetMesh, err)
		}

		// Create and respond to challenge
		challenge, err := bridgeAuth.CreateChallenge(sessionID)
		if err != nil {
			t.Fatalf("Failed to create challenge for mesh %s: %v", targetMesh, err)
		}

		response, err := bridgeAuth.RespondToChallenge(challenge, targetMesh)
		if err != nil {
			t.Fatalf("Failed to respond to challenge for mesh %s: %v", targetMesh, err)
		}

		authenticated, err := bridgeAuth.VerifyResponse(sessionID, response)
		if err != nil {
			t.Fatalf("Failed to verify response for mesh %s: %v", targetMesh, err)
		}

		if !authenticated {
			t.Errorf("Authentication should have succeeded for mesh %s", targetMesh)
		}

		// Verify session is authenticated
		if session.Status != "authenticated" {
			t.Errorf("Expected authenticated status for mesh %s, got %s", targetMesh, session.Status)
		}
	}

	// Verify all sessions are active
	activeSessions := bridgeAuth.ListActiveSessions()
	if len(activeSessions) != len(targetMeshes) {
		t.Errorf("Expected %d active sessions, got %d", len(targetMeshes), len(activeSessions))
	}

	// Clean up all sessions
	for i := range targetMeshes {
		sessionID := fmt.Sprintf("multi-mesh-session-%d", i)
		err := bridgeAuth.CloseSession(sessionID)
		if err != nil {
			t.Fatalf("Failed to close session %s: %v", sessionID, err)
		}
	}

	// All sessions should be closed
	activeSessions = bridgeAuth.ListActiveSessions()
	if len(activeSessions) != 0 {
		t.Errorf("Expected 0 active sessions after cleanup, got %d", len(activeSessions))
	}
}

// TestBridgeAuthentication_ErrorHandling tests error conditions in bridge authentication
func TestBridgeAuthentication_ErrorHandling(t *testing.T) {
	bridgeAuth := auth.NewBridgeAuth()

	// Test authentication with non-existent mesh
	_, err := bridgeAuth.AuthenticateToMesh("non-existent-mesh", "test-session", "mock-connection")
	if err == nil {
		t.Error("Expected error for non-existent mesh")
	}

	// Test challenge creation with non-existent session
	_, err = bridgeAuth.CreateChallenge("non-existent-session")
	if err == nil {
		t.Error("Expected error for non-existent session")
	}

	// Test response verification with non-existent session
	_, err = bridgeAuth.VerifyResponse("non-existent-session", nil)
	if err == nil {
		t.Error("Expected error for non-existent session")
	}

	// Test challenge response with non-existent mesh
	_, err = bridgeAuth.RespondToChallenge(nil, "non-existent-mesh")
	if err == nil {
		t.Error("Expected error for non-existent mesh")
	}

	// Test mobile identity validation with non-existent mesh
	err = bridgeAuth.ValidateMobileIdentity("non-existent-mesh")
	if err == nil {
		t.Error("Expected error for validating non-existent mesh")
	}
}

// TestBridgeAuthentication_AgentHandshakeIntegration tests integration with agent handshake
func TestBridgeAuthentication_AgentHandshakeIntegration(t *testing.T) {
	// Setup source mesh with handshake manager
	sourceMeshName := "source-mesh"
	sourceNodeID := "source-node-123"
	sourceIdentityManager := identity.NewIdentityManager(sourceMeshName, sourceNodeID)

	_, err := sourceIdentityManager.GenerateNodeIdentity()
	if err != nil {
		t.Fatalf("Failed to generate source node identity: %v", err)
	}

	// Create agent handshake manager
	agentHandshake := mesh.NewAgentHandshake(sourceMeshName, sourceNodeID, sourceIdentityManager)

	// Set up key derivation
	passphrase := "handshake-passphrase"
	deviceSecret := "handshake-device-secret"
	keyDerivation := crypto.NewMobileKeyDerivation(passphrase, deviceSecret, sourceNodeID)

	agentHandshake.SetKeyDerivation(keyDerivation)

	// Verify handshake manager is properly configured
	if agentHandshake == nil {
		t.Fatal("Agent handshake should not be nil")
	}

	// Test that handshake can create authentication session
	activeHandshakes := agentHandshake.ListActiveHandshakes()
	if len(activeHandshakes) != 0 {
		t.Error("Expected no active handshakes initially")
	}

	// The actual handshake initiation would require network connections
	// which are tested in the individual component tests
	// This test verifies the integration setup is correct
}