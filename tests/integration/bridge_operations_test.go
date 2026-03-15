// Package integration provides comprehensive end-to-end integration tests for AmorphDB multi-mesh bridge operations
package integration

import (
	"testing"

	"github.com/solifugus/amorphdb/internal/auth"
	"github.com/solifugus/amorphdb/internal/crypto"
	"github.com/solifugus/amorphdb/internal/identity"
	"github.com/solifugus/amorphdb/internal/interpreter"
	"github.com/solifugus/amorphdb/internal/storage"
	"github.com/solifugus/amorphdb/internal/types"
)

// TestBridgeOperations_EndToEnd tests complete multi-mesh bridge operations
// This covers Scenario B from the development plan
func TestBridgeOperations_EndToEnd(t *testing.T) {
	// Step 1: Set up storage and bridge components
	t.Log("Step 1: Setting up bridge infrastructure")

	localStorage := storage.NewMemoryTree()
	bridgeStorage := storage.NewBridgeStorage(localStorage)
	bridgeResolver := interpreter.NewBridgeScopeResolver(bridgeStorage)
	bridgeAuth := auth.NewBridgeAuth()

	// Step 2: Create mobile agent identity for partner mesh
	t.Log("Step 2: Creating mobile agent identity")

	identityManager := identity.NewIdentityManager("home-mesh", "bridge-node-001")

	// Create bridge identity for partner mesh
	partnerMeshBridgeIdentity, err := identityManager.CreateBridgeIdentity("partner-mesh")
	if err != nil {
		t.Fatalf("Failed to create partner mesh bridge identity: %v", err)
	}

	if partnerMeshBridgeIdentity.Type != types.MobileAgent {
		t.Errorf("Expected mobile agent type, got %v", partnerMeshBridgeIdentity.Type)
	}

	if partnerMeshBridgeIdentity.MeshName != "partner-mesh" {
		t.Errorf("Expected partner-mesh, got %s", partnerMeshBridgeIdentity.MeshName)
	}

	t.Logf("Created mobile agent identity: %s for partner-mesh", partnerMeshBridgeIdentity.ID)

	// Step 3: Set up mobile agent key derivation
	t.Log("Step 3: Setting up key derivation")

	passphrase := "bridge-test-passphrase"
	deviceSecret := "bridge-device-secret-123"
	keyDerivation := crypto.NewMobileKeyDerivation(passphrase, deviceSecret, "bridge-node-001")

	// Derive keys for partner mesh
	partnerKeys, err := keyDerivation.DeriveKeyPairForMesh("partner-mesh")
	if err != nil {
		t.Fatalf("Failed to derive partner mesh keys: %v", err)
	}

	if partnerKeys.MeshName != "partner-mesh" {
		t.Errorf("Expected keys for partner-mesh, got %s", partnerKeys.MeshName)
	}

	// Validate keys
	err = keyDerivation.ValidateKeyPairForMesh(partnerKeys)
	if err != nil {
		t.Fatalf("Partner mesh key validation failed: %v", err)
	}

	// Step 4: Create mobile identity in bridge authentication
	t.Log("Step 4: Setting up bridge authentication")

	mobileIdentity, err := bridgeAuth.CreateMobileIdentity("partner-mesh", passphrase, deviceSecret, partnerMeshBridgeIdentity)
	if err != nil {
		t.Fatalf("Failed to create mobile identity: %v", err)
	}

	// Validate mobile identity
	err = bridgeAuth.ValidateMobileIdentity("partner-mesh")
	if err != nil {
		t.Fatalf("Mobile identity validation failed: %v", err)
	}

	// Step 5: Register bridge connection
	t.Log("Step 5: Registering bridge connection")

	err = bridgeResolver.RegisterBridge("partner-mesh", partnerMeshBridgeIdentity, "connected", "127.0.0.1:8102")
	if err != nil {
		t.Fatalf("Failed to register bridge: %v", err)
	}

	// Verify bridge registration
	registeredBridges := bridgeResolver.GetRegisteredBridges()
	if len(registeredBridges) != 1 {
		t.Errorf("Expected 1 registered bridge, got %d", len(registeredBridges))
	}

	// Step 6: Test bridge path resolution
	t.Log("Step 6: Testing bridge path resolution")

	bridgeAccessPath := "my.partner-mesh.data.test_file"
	bridgeScope, err := bridgeResolver.ResolveBridgePath(bridgeAccessPath)
	if err != nil {
		t.Fatalf("Failed to resolve bridge path: %v", err)
	}

	expectedAgentHome := "world.agent." + partnerMeshBridgeIdentity.ID
	if bridgeScope.AgentHome != expectedAgentHome {
		t.Errorf("Expected agent home %s, got %s", expectedAgentHome, bridgeScope.AgentHome)
	}

	// Verify bridge path recognition
	if !bridgeResolver.IsBridgePath(bridgeAccessPath) {
		t.Error("Bridge path should be recognized")
	}

	// Step 7: Test bridge authentication session
	t.Log("Step 7: Testing bridge authentication")

	sessionID := "bridge-test-session-001"
	authSession, err := bridgeAuth.AuthenticateToMesh("partner-mesh", sessionID, "mock-connection")
	if err != nil {
		t.Fatalf("Failed to create auth session: %v", err)
	}

	if authSession.Status != "authenticating" {
		t.Errorf("Expected authenticating status, got %s", authSession.Status)
	}

	// Perform challenge-response
	challenge, err := bridgeAuth.CreateChallenge(sessionID)
	if err != nil {
		t.Fatalf("Failed to create challenge: %v", err)
	}

	if challenge.AgentIdentity != mobileIdentity.Identity.ID {
		t.Errorf("Expected challenge for %s, got %s", mobileIdentity.Identity.ID, challenge.AgentIdentity)
	}

	response, err := bridgeAuth.RespondToChallenge(challenge, "partner-mesh")
	if err != nil {
		t.Fatalf("Failed to respond to challenge: %v", err)
	}

	authenticated, err := bridgeAuth.VerifyResponse(sessionID, response)
	if err != nil {
		t.Fatalf("Failed to verify response: %v", err)
	}

	if !authenticated {
		t.Error("Authentication should have succeeded")
	}

	// Step 8: Test cross-mesh data operations
	t.Log("Step 8: Testing cross-mesh data operations")

	// Write data via bridge
	bridgeDataPath := "my.partner-mesh.test.data"
	bridgeDataComponents := types.ParsePathString(bridgeDataPath)
	testData := storage.Value{Data: []byte("Test data via bridge")}

	err = bridgeStorage.WriteBridgeData(bridgeDataComponents, testData, 1)
	if err != nil {
		t.Fatalf("Failed to write bridge data: %v", err)
	}

	// Read data back via bridge
	readData, err := bridgeStorage.ReadBridgeData(bridgeDataComponents)
	if err != nil {
		t.Fatalf("Failed to read bridge data: %v", err)
	}

	if string(readData.Data) != string(testData.Data) {
		t.Errorf("Expected data %q, got %q", testData.Data, readData.Data)
	}

	// Step 9: Test bridge disconnection
	t.Log("Step 9: Testing bridge disconnection")

	err = bridgeResolver.UpdateBridgeStatus("partner-mesh", "disconnected")
	if err != nil {
		t.Fatalf("Failed to disconnect bridge: %v", err)
	}

	// Access should now fail
	err = bridgeScope.ValidateAccess()
	if err == nil {
		t.Error("Expected access to fail after disconnection")
	}

	// Step 10: Test cleanup
	t.Log("Step 10: Testing cleanup")

	err = bridgeAuth.CloseSession(sessionID)
	if err != nil {
		t.Fatalf("Failed to close session: %v", err)
	}

	activeSessions := bridgeAuth.ListActiveSessions()
	if len(activeSessions) != 0 {
		t.Errorf("Expected 0 active sessions, got %d", len(activeSessions))
	}

	err = bridgeResolver.RemoveBridge("partner-mesh")
	if err != nil {
		t.Fatalf("Failed to remove bridge: %v", err)
	}

	if bridgeResolver.IsBridgePath(bridgeAccessPath) {
		t.Error("Bridge path should not be recognized after removal")
	}

	t.Log("Bridge operations test completed successfully")
}

// TestBridgeOperations_MultiMesh tests multiple bridge connections
func TestBridgeOperations_MultiMesh(t *testing.T) {
	t.Log("Testing multiple bridge operations")

	// Set up infrastructure
	localStorage := storage.NewMemoryTree()
	bridgeStorage := storage.NewBridgeStorage(localStorage)
	bridgeResolver := interpreter.NewBridgeScopeResolver(bridgeStorage)
	bridgeAuth := auth.NewBridgeAuth()

	identityManager := identity.NewIdentityManager("hub-mesh", "hub-node-001")

	// Create multiple mesh identities
	meshes := []string{"corporate-mesh", "partner-mesh", "test-mesh"}
	mobileIdentities := make(map[string]*auth.MobileIdentity)

	for _, meshName := range meshes {
		// Create bridge identity
		bridgeIdentity, err := identityManager.CreateBridgeIdentity(meshName)
		if err != nil {
			t.Fatalf("Failed to create bridge identity for %s: %v", meshName, err)
		}

		// Create mobile identity
		mobileIdentity, err := bridgeAuth.CreateMobileIdentity(meshName, "passphrase", "device-secret", bridgeIdentity)
		if err != nil {
			t.Fatalf("Failed to create mobile identity for %s: %v", meshName, err)
		}

		mobileIdentities[meshName] = mobileIdentity

		// Register bridge
		err = bridgeResolver.RegisterBridge(meshName, bridgeIdentity, "connected", "127.0.0.1:9000")
		if err != nil {
			t.Fatalf("Failed to register bridge for %s: %v", meshName, err)
		}

		// Validate bridge registration
		if !bridgeResolver.IsBridgePath("my."+meshName+".test") {
			t.Errorf("Bridge path not recognized for %s", meshName)
		}
	}

	// Verify all bridges are registered
	registeredBridges := bridgeResolver.GetRegisteredBridges()
	if len(registeredBridges) != len(meshes) {
		t.Errorf("Expected %d bridges, got %d", len(meshes), len(registeredBridges))
	}

	// Test concurrent access to multiple meshes
	for _, meshName := range meshes {
		testPath := "my." + meshName + ".concurrent.test"
		pathComponents := types.ParsePathString(testPath)
		testData := storage.Value{Data: []byte("Concurrent test data for " + meshName)}

		err := bridgeStorage.WriteBridgeData(pathComponents, testData, 1)
		if err != nil {
			t.Errorf("Failed to write data to %s: %v", meshName, err)
		}

		readData, err := bridgeStorage.ReadBridgeData(pathComponents)
		if err != nil {
			t.Errorf("Failed to read data from %s: %v", meshName, err)
		} else if string(readData.Data) != string(testData.Data) {
			t.Errorf("Data mismatch for %s: expected %q, got %q", meshName, testData.Data, readData.Data)
		}
	}

	t.Log("Multiple bridge operations test completed")
}

// TestBridgeOperations_ErrorHandling tests error scenarios
func TestBridgeOperations_ErrorHandling(t *testing.T) {
	t.Log("Testing bridge error handling")

	bridgeStorage := storage.NewBridgeStorage(storage.NewMemoryTree())
	bridgeResolver := interpreter.NewBridgeScopeResolver(bridgeStorage)
	bridgeAuth := auth.NewBridgeAuth()

	// Test non-existent bridge access
	nonExistentPath := "my.nonexistent-mesh.data"
	if bridgeResolver.IsBridgePath(nonExistentPath) {
		t.Error("Non-existent bridge should not be recognized")
	}

	pathComponents := types.ParsePathString(nonExistentPath)
	_, err := bridgeStorage.ReadBridgeData(pathComponents)
	if err == nil {
		t.Error("Expected error when reading from non-existent bridge")
	}

	// Test invalid bridge authentication
	_, err = bridgeAuth.AuthenticateToMesh("nonexistent-mesh", "test-session", "mock-connection")
	if err == nil {
		t.Error("Expected error when authenticating to non-existent mesh")
	}

	// Test invalid challenge operations
	_, err = bridgeAuth.CreateChallenge("nonexistent-session")
	if err == nil {
		t.Error("Expected error when creating challenge for non-existent session")
	}

	_, err = bridgeAuth.VerifyResponse("nonexistent-session", nil)
	if err == nil {
		t.Error("Expected error when verifying response for non-existent session")
	}

	t.Log("Bridge error handling test completed")
}