// Package integration provides comprehensive end-to-end integration tests for AmorphDB mobile agent bridge access
package integration

import (
	"fmt"
	"testing"
	"time"

	"github.com/solifugus/amorphdb/internal/auth"
	"github.com/solifugus/amorphdb/internal/config"
	"github.com/solifugus/amorphdb/internal/crypto"
	"github.com/solifugus/amorphdb/internal/identity"
	"github.com/solifugus/amorphdb/internal/interpreter"
	"github.com/solifugus/amorphdb/internal/mesh"
	"github.com/solifugus/amorphdb/internal/storage"
	"github.com/solifugus/amorphdb/internal/types"
)

// TestMobileAgentBridge_EndToEnd tests complete mobile agent bridge functionality
// This covers Scenario D from the development plan
func TestMobileAgentBridge_EndToEnd(t *testing.T) {
	// Step 1: Set up primary mesh (home mesh)
	t.Log("Step 1: Setting up primary mesh")

	homeCfg := &config.Config{
		Data: config.DataConfig{
			Directory: "/tmp/amorphdb-test-home-mesh",
		},
		Network: config.NetworkConfig{
			Address: "127.0.0.1:8301",
		},
		Mesh: config.MeshConfig{
			Name: "home-mesh",
		},
	}

	homeStorage := storage.NewMemoryTree()
	homeIdentity := identity.NewIdentityManager("home-mesh", "home-node-001")
	homeMesh := mesh.NewMeshManager(homeCfg, homeStorage, homeIdentity)

	err := homeMesh.CreateMesh("home-mesh")
	if err != nil {
		t.Fatalf("Failed to create home mesh: %v", err)
	}

	homeNodeIdentity := homeIdentity.GetNodeIdentity()
	if homeNodeIdentity.Type != types.NodeAgent {
		t.Errorf("Expected home node to be NodeAgent, got %v", homeNodeIdentity.Type)
	}

	// Add some data to home mesh
	homeData := storage.Value{Data: []byte("Home mesh private data")}
	homePath := []string{"users", "alice", "home_profile"}
	err = homeStorage.Set(homePath, homeData, 1, time.Now())
	if err != nil {
		t.Fatalf("Failed to set home data: %v", err)
	}

	// Step 2: Set up partner mesh (destination mesh)
	t.Log("Step 2: Setting up partner mesh")

	partnerCfg := &config.Config{
		Data: config.DataConfig{
			Directory: "/tmp/amorphdb-test-partner-mesh",
		},
		Network: config.NetworkConfig{
			Address: "127.0.0.1:8302",
		},
		Mesh: config.MeshConfig{
			Name: "partner-mesh",
		},
	}

	partnerStorage := storage.NewMemoryTree()
	partnerIdentity := identity.NewIdentityManager("partner-mesh", "partner-node-001")
	partnerMesh := mesh.NewMeshManager(partnerCfg, partnerStorage, partnerIdentity)

	err = partnerMesh.CreateMesh("partner-mesh")
	if err != nil {
		t.Fatalf("Failed to create partner mesh: %v", err)
	}

	// Add some data to partner mesh
	partnerData := storage.Value{Data: []byte("Partner mesh shared resources")}
	partnerPath := []string{"world", "shared", "resources", "partner_doc"}
	err = partnerStorage.Set(partnerPath, partnerData, 1, time.Now())
	if err != nil {
		t.Fatalf("Failed to set partner data: %v", err)
	}

	// Step 3: Create mobile agent bridge identity in home mesh
	t.Log("Step 3: Creating mobile agent bridge identity")

	// Create mobile agent identity for accessing partner mesh
	mobileBridgeIdentity, err := homeIdentity.CreateBridgeIdentity("partner-mesh")
	if err != nil {
		t.Fatalf("Failed to create mobile bridge identity: %v", err)
	}

	if mobileBridgeIdentity.Type != types.MobileAgent {
		t.Errorf("Expected mobile agent type, got %v", mobileBridgeIdentity.Type)
	}

	if mobileBridgeIdentity.MeshName != "partner-mesh" {
		t.Errorf("Expected bridge identity for partner-mesh, got %s", mobileBridgeIdentity.MeshName)
	}

	t.Logf("Created mobile agent identity: %s for partner-mesh", mobileBridgeIdentity.ID)

	// Step 4: Set up mobile agent key derivation system
	t.Log("Step 4: Setting up mobile agent key derivation")

	passphrase := "mobile-agent-bridge-passphrase"
	deviceSecret := "device-secret-" + homeNodeIdentity.ID
	homeNodeID := homeNodeIdentity.ID

	keyDerivation := crypto.NewMobileKeyDerivation(passphrase, deviceSecret, homeNodeID)

	// Derive key pair specifically for partner mesh
	partnerMeshKeys, err := keyDerivation.DeriveKeyPairForMesh("partner-mesh")
	if err != nil {
		t.Fatalf("Failed to derive keys for partner mesh: %v", err)
	}

	if partnerMeshKeys.MeshName != "partner-mesh" {
		t.Errorf("Expected keys for partner-mesh, got %s", partnerMeshKeys.MeshName)
	}

	// Validate derived keys
	err = keyDerivation.ValidateKeyPairForMesh(partnerMeshKeys)
	if err != nil {
		t.Fatalf("Partner mesh key validation failed: %v", err)
	}

	t.Logf("Derived and validated keys for partner mesh")

	// Step 5: Set up bridge authentication system
	t.Log("Step 5: Setting up bridge authentication")

	bridgeAuth := auth.NewBridgeAuth()

	// Create mobile identity in bridge authentication system
	mobileIdentity, err := bridgeAuth.CreateMobileIdentity("partner-mesh", passphrase, deviceSecret, mobileBridgeIdentity)
	if err != nil {
		t.Fatalf("Failed to create mobile identity in bridge auth: %v", err)
	}

	// Validate mobile identity
	err = bridgeAuth.ValidateMobileIdentity("partner-mesh")
	if err != nil {
		t.Fatalf("Mobile identity validation failed: %v", err)
	}

	// Verify mobile identity properties
	if mobileIdentity.MeshName != "partner-mesh" {
		t.Errorf("Expected mobile identity for partner-mesh, got %s", mobileIdentity.MeshName)
	}

	if mobileIdentity.Identity.ID != mobileBridgeIdentity.ID {
		t.Errorf("Expected mobile identity ID %s, got %s", mobileBridgeIdentity.ID, mobileIdentity.Identity.ID)
	}

	// Step 6: Request identity assignment in partner mesh
	t.Log("Step 6: Requesting identity assignment in partner mesh")

	// In partner mesh, assign agent home for the mobile agent
	agentHomePath := []string{"world", "agent", mobileBridgeIdentity.ID}
	agentHomeValue := storage.Value{Data: []byte("Mobile agent home directory")}
	err = partnerStorage.Set(agentHomePath, agentHomeValue, 1, time.Now())
	if err != nil {
		t.Fatalf("Failed to create agent home in partner mesh: %v", err)
	}

	// Create agent profile data
	agentProfilePath := []string{"world", "agent", mobileBridgeIdentity.ID, "profile"}
	agentProfileValue := storage.Value{Data: []byte(fmt.Sprintf("Mobile agent profile for %s", mobileBridgeIdentity.ID))}
	err = partnerStorage.Set(agentProfilePath, agentProfileValue, 1, time.Now())
	if err != nil {
		t.Fatalf("Failed to create agent profile in partner mesh: %v", err)
	}

	// Add mobile agent data directory
	agentDataPath := []string{"world", "agent", mobileBridgeIdentity.ID, "data"}
	agentDataValue := storage.Value{Data: []byte("Mobile agent private data")}
	err = partnerStorage.Set(agentDataPath, agentDataValue, 1, time.Now())
	if err != nil {
		t.Fatalf("Failed to create agent data in partner mesh: %v", err)
	}

	t.Logf("Assigned agent home in partner mesh: %s", agentHomePath)

	// Step 7: Establish bridge connection and authentication
	t.Log("Step 7: Establishing bridge connection and authentication")

	// Create authentication session
	sessionID := "mobile-bridge-session-001"
	authSession, err := bridgeAuth.AuthenticateToMesh("partner-mesh", sessionID, "mock-connection")
	if err != nil {
		t.Fatalf("Failed to create authentication session: %v", err)
	}

	if authSession.Status != "authenticating" {
		t.Errorf("Expected session status 'authenticating', got %s", authSession.Status)
	}

	// Perform challenge-response authentication
	challenge, err := bridgeAuth.CreateChallenge(sessionID)
	if err != nil {
		t.Fatalf("Failed to create authentication challenge: %v", err)
	}

	if challenge.AgentIdentity != mobileBridgeIdentity.ID {
		t.Errorf("Expected challenge for agent %s, got %s", mobileBridgeIdentity.ID, challenge.AgentIdentity)
	}

	response, err := bridgeAuth.RespondToChallenge(challenge, "partner-mesh")
	if err != nil {
		t.Fatalf("Failed to respond to authentication challenge: %v", err)
	}

	authenticated, err := bridgeAuth.VerifyResponse(sessionID, response)
	if err != nil {
		t.Fatalf("Failed to verify authentication response: %v", err)
	}

	if !authenticated {
		t.Fatal("Mobile agent authentication should have succeeded")
	}

	// Verify session is now authenticated
	finalSession := bridgeAuth.GetSession(sessionID)
	if finalSession.Status != "authenticated" {
		t.Errorf("Expected final session status 'authenticated', got %s", finalSession.Status)
	}

	t.Log("Mobile agent authentication completed successfully")

	// Step 8: Set up bridge data access from home mesh
	t.Log("Step 8: Setting up bridge data access from home mesh")

	homeBridgeStorage := storage.NewBridgeStorage(homeStorage)
	homeResolver := interpreter.NewBridgeScopeResolver(homeBridgeStorage)

	// Register partner mesh bridge
	err = homeResolver.RegisterBridge("partner-mesh", mobileBridgeIdentity, "connected", "127.0.0.1:8302")
	if err != nil {
		t.Fatalf("Failed to register partner mesh bridge: %v", err)
	}

	// Test bridge path resolution
	bridgeAccessPath := "my.partner-mesh.data.test_file"
	bridgeScope, err := homeResolver.ResolveBridgePath(bridgeAccessPath)
	if err != nil {
		t.Fatalf("Failed to resolve bridge path: %v", err)
	}

	expectedAgentHome := "world.agent." + mobileBridgeIdentity.ID
	if bridgeScope.AgentHome != expectedAgentHome {
		t.Errorf("Expected agent home %s, got %s", expectedAgentHome, bridgeScope.AgentHome)
	}

	// Step 9: Test cross-mesh data operations as mobile agent
	t.Log("Step 9: Testing cross-mesh data operations")

	// Test 1: Write data to partner mesh via bridge
	bridgeDataPath := "my.partner-mesh.mobile_agent.test_data"
	bridgeDataComponents := types.ParsePathString(bridgeDataPath)
	bridgeDataValue := storage.Value{Data: []byte("Data written by mobile agent via bridge")}

	err = homeBridgeStorage.WriteBridgeData(bridgeDataComponents, bridgeDataValue, 1)
	if err != nil {
		t.Fatalf("Failed to write data via bridge: %v", err)
	}

	// Verify data was written to correct agent location in partner mesh
	targetPath := []string{"world", "agent", mobileBridgeIdentity.ID, "mobile_agent", "test_data"}
	targetValue, err := partnerStorage.Get(targetPath, time.Now())
	if err != nil {
		t.Errorf("Failed to verify bridge data in partner mesh: %v", err)
	} else if string(targetValue.Data) != string(bridgeDataValue.Data) {
		t.Errorf("Bridge data mismatch: expected %q, got %q", bridgeDataValue.Data, targetValue.Data)
	}

	// Test 2: Read data back via bridge
	readBridgeData, err := homeBridgeStorage.ReadBridgeData(bridgeDataComponents)
	if err != nil {
		t.Fatalf("Failed to read data via bridge: %v", err)
	}

	if string(readBridgeData.Data) != string(bridgeDataValue.Data) {
		t.Errorf("Bridge read data mismatch: expected %q, got %q", bridgeDataValue.Data, readBridgeData.Data)
	}

	// Test 3: Access agent profile data via bridge
	agentProfileBridgePath := "my.partner-mesh.profile"
	agentProfileComponents := types.ParsePathString(agentProfileBridgePath)
	agentProfileBridgeData, err := homeBridgeStorage.ReadBridgeData(agentProfileComponents)
	if err != nil {
		t.Fatalf("Failed to read agent profile via bridge: %v", err)
	}

	expectedProfileData := agentProfileValue.Data
	if string(agentProfileBridgeData.Data) != string(expectedProfileData) {
		t.Errorf("Agent profile bridge data mismatch: expected %q, got %q",
			expectedProfileData, agentProfileBridgeData.Data)
	}

	t.Log("Cross-mesh data operations completed successfully")

	// Step 10: Test agent home replication pattern
	t.Log("Step 10: Testing agent home replication pattern")

	// Create data in home mesh that should be accessible from partner mesh
	homeAgentDataPath := []string{"world", "agent", homeNodeIdentity.ID, "sync", "home_to_partner"}
	homeAgentDataValue := storage.Value{Data: []byte("Data to sync from home to partner")}
	err = homeStorage.Set(homeAgentDataPath, homeAgentDataValue, 1, time.Now())
	if err != nil {
		t.Fatalf("Failed to set home agent data: %v", err)
	}

	// Simulate agent data sync from home mesh to partner mesh agent home
	partnerAgentSyncPath := []string{"world", "agent", mobileBridgeIdentity.ID, "sync", "home_to_partner"}
	err = partnerStorage.Set(partnerAgentSyncPath, homeAgentDataValue, 1, time.Now())
	if err != nil {
		t.Fatalf("Failed to sync data to partner agent home: %v", err)
	}

	// Verify sync data is accessible via bridge
	syncBridgePath := "my.partner-mesh.sync.home_to_partner"
	syncBridgeComponents := types.ParsePathString(syncBridgePath)
	syncBridgeData, err := homeBridgeStorage.ReadBridgeData(syncBridgeComponents)
	if err != nil {
		t.Fatalf("Failed to read synced data via bridge: %v", err)
	}

	if string(syncBridgeData.Data) != string(homeAgentDataValue.Data) {
		t.Errorf("Synced data mismatch: expected %q, got %q",
			homeAgentDataValue.Data, syncBridgeData.Data)
	}

	t.Log("Agent home replication pattern test completed successfully")

	// Step 11: Test multiple mobile agent scenarios
	t.Log("Step 11: Testing multiple mobile agent scenarios")

	// Create second mobile agent identity for different mesh
	thirdMeshName := "corporate-mesh"
	corporateMobileIdentity, err := homeIdentity.CreateBridgeIdentity(thirdMeshName)
	if err != nil {
		t.Fatalf("Failed to create corporate mobile identity: %v", err)
	}

	corporateKeys, err := keyDerivation.DeriveKeyPairForMesh(thirdMeshName)
	if err != nil {
		t.Fatalf("Failed to derive corporate mesh keys: %v", err)
	}

	corporateIdentity, err := bridgeAuth.CreateMobileIdentity(thirdMeshName, passphrase, deviceSecret, corporateMobileIdentity)
	if err != nil {
		t.Fatalf("Failed to create corporate mobile identity: %v", err)
	}

	// Test multiple mesh key derivation
	multipleMeshes := []string{"partner-mesh", thirdMeshName, "test-mesh-4"}
	multipleKeys, err := keyDerivation.DeriveMultipleMeshKeys(multipleMeshes)
	if err != nil {
		t.Fatalf("Failed to derive multiple mesh keys: %v", err)
	}

	if len(multipleKeys) != len(multipleMeshes) {
		t.Errorf("Expected %d key pairs, got %d", len(multipleMeshes), len(multipleKeys))
	}

	// Verify each mesh has different keys
	keyList := make([]*crypto.MeshKeyPair, 0, len(multipleKeys))
	for _, keyPair := range multipleKeys {
		keyList = append(keyList, keyPair)
	}

	for i := 0; i < len(keyList); i++ {
		for j := i + 1; j < len(keyList); j++ {
			if keyList[i].PublicKey.Cmp(keyList[j].PublicKey) == 0 {
				t.Errorf("Keys for meshes %s and %s are identical",
					keyList[i].MeshName, keyList[j].MeshName)
			}
		}
	}

	t.Log("Multiple mobile agent scenarios test completed")

	// Step 12: Test mobile agent disconnection and cleanup
	t.Log("Step 12: Testing mobile agent disconnection and cleanup")

	// Close authentication session
	err = bridgeAuth.CloseSession(sessionID)
	if err != nil {
		t.Fatalf("Failed to close authentication session: %v", err)
	}

	// Update bridge status to disconnected
	err = homeResolver.UpdateBridgeStatus("partner-mesh", "disconnected")
	if err != nil {
		t.Fatalf("Failed to update bridge status to disconnected: %v", err)
	}

	// Verify bridge access is now denied
	err = bridgeScope.ValidateAccess()
	if err == nil {
		t.Error("Expected bridge access to fail after disconnection")
	}

	// Remove mobile identity
	err = bridgeAuth.RemoveMobileIdentity("partner-mesh")
	if err != nil {
		t.Fatalf("Failed to remove mobile identity: %v", err)
	}

	// Verify mobile identity is cleaned up
	removedIdentity := bridgeAuth.GetMobileIdentity("partner-mesh")
	if removedIdentity != nil {
		t.Error("Mobile identity should be removed")
	}

	// Remove bridge registration
	err = homeResolver.RemoveBridge("partner-mesh")
	if err != nil {
		t.Fatalf("Failed to remove bridge: %v", err)
	}

	// Verify bridge is no longer recognized
	if homeResolver.IsBridgePath(bridgeAccessPath) {
		t.Error("Bridge path should not be recognized after removal")
	}

	t.Log("Mobile agent bridge test completed successfully")
}

// TestMobileAgentBridge_KeyDerivationConsistency tests key derivation consistency
func TestMobileAgentBridge_KeyDerivationConsistency(t *testing.T) {
	t.Log("Testing mobile agent key derivation consistency")

	passphrase := "consistency-test-passphrase"
	deviceSecret := "consistency-device-secret"
	nodeID := "consistency-test-node"

	keyDerivation1 := crypto.NewMobileKeyDerivation(passphrase, deviceSecret, nodeID)
	keyDerivation2 := crypto.NewMobileKeyDerivation(passphrase, deviceSecret, nodeID)

	meshName := "consistency-test-mesh"

	// Derive keys from both instances
	keys1, err := keyDerivation1.DeriveKeyPairForMesh(meshName)
	if err != nil {
		t.Fatalf("Failed to derive keys from first instance: %v", err)
	}

	keys2, err := keyDerivation2.DeriveKeyPairForMesh(meshName)
	if err != nil {
		t.Fatalf("Failed to derive keys from second instance: %v", err)
	}

	// Keys should be identical
	if keys1.PublicKey.Cmp(keys2.PublicKey) != 0 {
		t.Error("Public keys should be consistent across instances")
	}

	if keys1.PrivateKey.Cmp(keys2.PrivateKey) != 0 {
		t.Error("Private keys should be consistent across instances")
	}

	if keys1.MeshSalt != keys2.MeshSalt {
		t.Error("Mesh salts should be consistent across instances")
	}

	// Test key regeneration
	newSalt := "new-consistency-salt"
	regeneratedKeys1, err := keyDerivation1.RegenerateKeyPairForMesh(meshName, newSalt)
	if err != nil {
		t.Fatalf("Failed to regenerate keys: %v", err)
	}

	regeneratedKeys2, err := keyDerivation2.RegenerateKeyPairForMesh(meshName, newSalt)
	if err != nil {
		t.Fatalf("Failed to regenerate keys from second instance: %v", err)
	}

	// Regenerated keys should also be consistent
	if regeneratedKeys1.PublicKey.Cmp(regeneratedKeys2.PublicKey) != 0 {
		t.Error("Regenerated public keys should be consistent")
	}

	// But different from original keys
	if keys1.PublicKey.Cmp(regeneratedKeys1.PublicKey) == 0 {
		t.Error("Regenerated keys should be different from original keys")
	}

	t.Log("Key derivation consistency test completed")
}

// TestMobileAgentBridge_PermissionsAndAccess tests access control for mobile agents
func TestMobileAgentBridge_PermissionsAndAccess(t *testing.T) {
	t.Log("Testing mobile agent permissions and access control")

	// Set up mesh and mobile agent
	localStorage := storage.NewMemoryTree()
	identityManager := identity.NewIdentityManager("test-mesh", "test-node")
	bridgeAuth := auth.NewBridgeAuth()

	// Create mobile agent identity
	mobileIdentity, err := identityManager.CreateBridgeIdentity("partner-mesh")
	if err != nil {
		t.Fatalf("Failed to create mobile identity: %v", err)
	}

	_, err = bridgeAuth.CreateMobileIdentity("partner-mesh", "passphrase", "device-secret", mobileIdentity)
	if err != nil {
		t.Fatalf("Failed to create mobile identity in auth: %v", err)
	}

	// Test access patterns that should work
	allowedPaths := []string{
		"my.partner-mesh.personal.data",
		"my.partner-mesh.sync.files",
		"my.partner-mesh.profile.settings",
	}

	bridgeStorage := storage.NewBridgeStorage(localStorage)
	for _, pathStr := range allowedPaths {
		pathComponents := types.ParsePathString(pathStr)
		testValue := storage.Value{Data: []byte("Test data for " + pathStr)}

		err = bridgeStorage.WriteBridgeData(pathComponents, testValue, 1)
		if err != nil {
			t.Errorf("Should be able to write to %s: %v", pathStr, err)
		}

		_, err = bridgeStorage.ReadBridgeData(pathComponents)
		if err != nil {
			t.Errorf("Should be able to read from %s: %v", pathStr, err)
		}
	}

	// Test access patterns that should be restricted (if implemented)
	restrictedPaths := []string{
		"world.shared.sensitive",
		"world.admin.config",
		"other.agent.private.data",
	}

	for _, pathStr := range restrictedPaths {
		t.Logf("Testing restricted path: %s (implementation-dependent)", pathStr)
		// In a real implementation, these might be blocked at the mesh level
		// For now, this documents the intended access control patterns
	}

	t.Log("Mobile agent permissions test completed")
}