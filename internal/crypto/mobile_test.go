// Package crypto provides mobile agent key derivation tests
package crypto

import (
	"testing"
)

func TestNewMobileKeyDerivation(t *testing.T) {
	passphrase := "test-passphrase"
	deviceSecret := "test-device-secret"
	nodeID := "test-node-123"

	mkd := NewMobileKeyDerivation(passphrase, deviceSecret, nodeID)

	if mkd == nil {
		t.Fatal("MobileKeyDerivation should not be nil")
	}

	if mkd.baseFactors == nil {
		t.Fatal("Base factors should not be nil")
	}

	if mkd.baseFactors.Passphrase != passphrase {
		t.Errorf("Expected passphrase %s, got %s", passphrase, mkd.baseFactors.Passphrase)
	}

	if mkd.baseFactors.DeviceSecret != deviceSecret {
		t.Errorf("Expected device secret %s, got %s", deviceSecret, mkd.baseFactors.DeviceSecret)
	}

	if mkd.baseFactors.NodeID != nodeID {
		t.Errorf("Expected node ID %s, got %s", nodeID, mkd.baseFactors.NodeID)
	}
}

func TestMobileKeyDerivation_DeriveKeyPairForMesh(t *testing.T) {
	mkd := NewMobileKeyDerivation("test-passphrase", "test-device-secret", "test-node-123")

	meshName := "test-mesh"

	// Test successful key derivation
	keyPair, err := mkd.DeriveKeyPairForMesh(meshName)
	if err != nil {
		t.Fatalf("Failed to derive key pair: %v", err)
	}

	if keyPair == nil {
		t.Fatal("Key pair should not be nil")
	}

	if keyPair.MeshName != meshName {
		t.Errorf("Expected mesh name %s, got %s", meshName, keyPair.MeshName)
	}

	if keyPair.PublicKey == nil {
		t.Fatal("Public key should not be nil")
	}

	if keyPair.PrivateKey == nil {
		t.Fatal("Private key should not be nil")
	}

	if len(keyPair.AgentKey) == 0 {
		t.Fatal("Agent key should not be empty")
	}

	if keyPair.MeshSalt == "" {
		t.Fatal("Mesh salt should not be empty")
	}

	// Test that derived key pairs are deterministic
	keyPair2, err := mkd.DeriveKeyPairForMesh(meshName)
	if err != nil {
		t.Fatalf("Failed to derive second key pair: %v", err)
	}

	if keyPair.PublicKey.Cmp(keyPair2.PublicKey) != 0 {
		t.Error("Public keys should be identical for same mesh")
	}

	if keyPair.PrivateKey.Cmp(keyPair2.PrivateKey) != 0 {
		t.Error("Private keys should be identical for same mesh")
	}

	// Test that different meshes produce different key pairs
	keyPair3, err := mkd.DeriveKeyPairForMesh("different-mesh")
	if err != nil {
		t.Fatalf("Failed to derive key pair for different mesh: %v", err)
	}

	if keyPair.PublicKey.Cmp(keyPair3.PublicKey) == 0 {
		t.Error("Different meshes should produce different public keys")
	}

	if keyPair.PrivateKey.Cmp(keyPair3.PrivateKey) == 0 {
		t.Error("Different meshes should produce different private keys")
	}

	// Test empty mesh name
	_, err = mkd.DeriveKeyPairForMesh("")
	if err == nil {
		t.Error("Expected error for empty mesh name")
	}
}

func TestMobileKeyDerivation_DeriveMultipleMeshKeys(t *testing.T) {
	mkd := NewMobileKeyDerivation("test-passphrase", "test-device-secret", "test-node-123")

	meshNames := []string{"mesh-alpha", "mesh-beta", "mesh-gamma"}

	// Test successful multiple key derivation
	keyPairs, err := mkd.DeriveMultipleMeshKeys(meshNames)
	if err != nil {
		t.Fatalf("Failed to derive multiple key pairs: %v", err)
	}

	if len(keyPairs) != len(meshNames) {
		t.Errorf("Expected %d key pairs, got %d", len(meshNames), len(keyPairs))
	}

	// Verify each mesh has a key pair
	for _, meshName := range meshNames {
		keyPair, exists := keyPairs[meshName]
		if !exists {
			t.Errorf("Key pair not found for mesh %s", meshName)
			continue
		}

		if keyPair.MeshName != meshName {
			t.Errorf("Expected mesh name %s, got %s", meshName, keyPair.MeshName)
		}

		if keyPair.PublicKey == nil || keyPair.PrivateKey == nil {
			t.Errorf("Invalid key pair for mesh %s", meshName)
		}
	}

	// Test all key pairs are different
	keyPairList := make([]*MeshKeyPair, 0, len(keyPairs))
	for _, keyPair := range keyPairs {
		keyPairList = append(keyPairList, keyPair)
	}

	for i := 0; i < len(keyPairList); i++ {
		for j := i + 1; j < len(keyPairList); j++ {
			if keyPairList[i].PublicKey.Cmp(keyPairList[j].PublicKey) == 0 {
				t.Errorf("Key pairs %d and %d have identical public keys", i, j)
			}
		}
	}

	// Test empty mesh list
	emptyResult, err := mkd.DeriveMultipleMeshKeys([]string{})
	if err != nil {
		t.Fatalf("Failed to handle empty mesh list: %v", err)
	}

	if len(emptyResult) != 0 {
		t.Error("Expected empty result for empty mesh list")
	}
}

func TestMobileKeyDerivation_RegenerateKeyPairForMesh(t *testing.T) {
	mkd := NewMobileKeyDerivation("test-passphrase", "test-device-secret", "test-node-123")

	meshName := "test-mesh"

	// Get original key pair
	original, err := mkd.DeriveKeyPairForMesh(meshName)
	if err != nil {
		t.Fatalf("Failed to derive original key pair: %v", err)
	}

	// Regenerate with new salt
	regenerated, err := mkd.RegenerateKeyPairForMesh(meshName, "new-salt")
	if err != nil {
		t.Fatalf("Failed to regenerate key pair: %v", err)
	}

	// Key pairs should be different
	if original.PublicKey.Cmp(regenerated.PublicKey) == 0 {
		t.Error("Regenerated public key should be different from original")
	}

	if original.PrivateKey.Cmp(regenerated.PrivateKey) == 0 {
		t.Error("Regenerated private key should be different from original")
	}

	if original.MeshSalt == regenerated.MeshSalt {
		t.Error("Regenerated mesh salt should be different from original")
	}

	// Test regeneration with empty salt (should auto-generate)
	autoRegenerated, err := mkd.RegenerateKeyPairForMesh(meshName, "")
	if err != nil {
		t.Fatalf("Failed to auto-regenerate key pair: %v", err)
	}

	if autoRegenerated.MeshSalt == original.MeshSalt {
		t.Error("Auto-regenerated salt should be different from original")
	}

	// Test empty mesh name
	_, err = mkd.RegenerateKeyPairForMesh("", "new-salt")
	if err == nil {
		t.Error("Expected error for empty mesh name")
	}
}

func TestMobileKeyDerivation_ValidateKeyPairForMesh(t *testing.T) {
	mkd := NewMobileKeyDerivation("test-passphrase", "test-device-secret", "test-node-123")

	// Create valid key pair
	keyPair, err := mkd.DeriveKeyPairForMesh("test-mesh")
	if err != nil {
		t.Fatalf("Failed to derive key pair: %v", err)
	}

	// Test successful validation
	err = mkd.ValidateKeyPairForMesh(keyPair)
	if err != nil {
		t.Fatalf("Valid key pair should pass validation: %v", err)
	}

	// Test nil key pair
	err = mkd.ValidateKeyPairForMesh(nil)
	if err == nil {
		t.Error("Expected error for nil key pair")
	}

	// Test validation with corrupted public key
	corruptedKeyPair := *keyPair
	corruptedKeyPair.PublicKey = corruptedKeyPair.PublicKey.Add(corruptedKeyPair.PublicKey, corruptedKeyPair.PublicKey)
	err = mkd.ValidateKeyPairForMesh(&corruptedKeyPair)
	if err == nil {
		t.Error("Expected error for corrupted public key")
	}

	// Test validation with corrupted private key
	corruptedKeyPair = *keyPair
	corruptedKeyPair.PrivateKey = corruptedKeyPair.PrivateKey.Add(corruptedKeyPair.PrivateKey, corruptedKeyPair.PrivateKey)
	err = mkd.ValidateKeyPairForMesh(&corruptedKeyPair)
	if err == nil {
		t.Error("Expected error for corrupted private key")
	}

	// Test validation with corrupted agent key
	corruptedKeyPair = *keyPair
	corruptedKeyPair.AgentKey[0] = corruptedKeyPair.AgentKey[0] ^ 0xFF
	err = mkd.ValidateKeyPairForMesh(&corruptedKeyPair)
	if err == nil {
		t.Error("Expected error for corrupted agent key")
	}
}

func TestMobileKeyDerivation_GetPublicKeyForMesh(t *testing.T) {
	mkd := NewMobileKeyDerivation("test-passphrase", "test-device-secret", "test-node-123")

	meshName := "test-mesh"

	// Get public key
	publicKey, err := mkd.GetPublicKeyForMesh(meshName)
	if err != nil {
		t.Fatalf("Failed to get public key: %v", err)
	}

	if publicKey == nil {
		t.Fatal("Public key should not be nil")
	}

	// Verify it matches full key pair derivation
	keyPair, err := mkd.DeriveKeyPairForMesh(meshName)
	if err != nil {
		t.Fatalf("Failed to derive full key pair: %v", err)
	}

	if publicKey.Cmp(keyPair.PublicKey) != 0 {
		t.Error("Standalone public key should match key pair public key")
	}
}

func TestMobileKeyDerivation_UpdateBaseFactors(t *testing.T) {
	mkd := NewMobileKeyDerivation("original-passphrase", "original-device-secret", "test-node-123")

	meshName := "test-mesh"

	// Get key pair with original factors
	original, err := mkd.DeriveKeyPairForMesh(meshName)
	if err != nil {
		t.Fatalf("Failed to derive original key pair: %v", err)
	}

	// Update passphrase
	mkd.UpdateBaseFactors("new-passphrase", "")

	if mkd.baseFactors.Passphrase != "new-passphrase" {
		t.Error("Passphrase should be updated")
	}

	if mkd.baseFactors.DeviceSecret != "original-device-secret" {
		t.Error("Device secret should remain unchanged when empty string passed")
	}

	// Update device secret
	mkd.UpdateBaseFactors("", "new-device-secret")

	if mkd.baseFactors.DeviceSecret != "new-device-secret" {
		t.Error("Device secret should be updated")
	}

	// Key pair should be different with new factors
	updated, err := mkd.DeriveKeyPairForMesh(meshName)
	if err != nil {
		t.Fatalf("Failed to derive updated key pair: %v", err)
	}

	if original.PublicKey.Cmp(updated.PublicKey) == 0 {
		t.Error("Key pair should be different after updating base factors")
	}
}

func TestMeshKeyPair_ExportImport(t *testing.T) {
	mkd := NewMobileKeyDerivation("test-passphrase", "test-device-secret", "test-node-123")

	// Create key pair
	original, err := mkd.DeriveKeyPairForMesh("test-mesh")
	if err != nil {
		t.Fatalf("Failed to derive key pair: %v", err)
	}

	// Test export
	exported := original.ExportKeyPair()
	if exported == nil {
		t.Fatal("Exported data should not be nil")
	}

	// Test import
	imported, err := ImportKeyPair(exported)
	if err != nil {
		t.Fatalf("Failed to import key pair: %v", err)
	}

	// Verify imported key pair matches original
	if imported.MeshName != original.MeshName {
		t.Errorf("Imported mesh name should match original")
	}

	if imported.PublicKey.Cmp(original.PublicKey) != 0 {
		t.Error("Imported public key should match original")
	}

	if imported.PrivateKey.Cmp(original.PrivateKey) != 0 {
		t.Error("Imported private key should match original")
	}

	if len(imported.AgentKey) != len(original.AgentKey) {
		t.Error("Imported agent key length should match original")
	}

	for i := range imported.AgentKey {
		if imported.AgentKey[i] != original.AgentKey[i] {
			t.Error("Imported agent key should match original")
			break
		}
	}

	// Test public key export
	publicExport := original.ExportPublicKey()
	if publicExport["private_key"] != nil {
		t.Error("Public key export should not contain private key")
	}

	if publicExport["public_key"] != original.PublicKey.String() {
		t.Error("Public key export should contain correct public key")
	}
}

func TestMeshKeyPair_String(t *testing.T) {
	mkd := NewMobileKeyDerivation("test-passphrase", "test-device-secret", "test-node-123")

	keyPair, err := mkd.DeriveKeyPairForMesh("test-mesh")
	if err != nil {
		t.Fatalf("Failed to derive key pair: %v", err)
	}

	str := keyPair.String()
	if str == "" {
		t.Error("String representation should not be empty")
	}

	if !contains(str, "test-mesh") {
		t.Error("String representation should contain mesh name")
	}

	if !contains(str, "MeshKeyPair") {
		t.Error("String representation should contain type name")
	}
}

func TestBaseFactors_String(t *testing.T) {
	baseFactors := &BaseFactors{
		Passphrase:   "test-passphrase",
		DeviceSecret: "test-device-secret",
		NodeID:       "test-node-123",
	}

	str := baseFactors.String()
	if str == "" {
		t.Error("String representation should not be empty")
	}

	if !contains(str, "test-node-123") {
		t.Error("String representation should contain node ID")
	}

	if contains(str, "test-passphrase") {
		t.Error("String representation should not contain actual passphrase")
	}
}

// Helper function to check if string contains substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && s[:len(substr)] == substr ||
		   (len(s) > len(substr) && contains(s[1:], substr))
}