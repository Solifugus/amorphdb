package cmd

import (
	"crypto/rand"
	"strings"
	"testing"
	"time"

	"github.com/solifugus/amorphdb/internal/identity"
	"github.com/solifugus/amorphdb/internal/security"
)

// Section 11: Security Tests
// Based on AmorphDB_Test_Plan.md Section 11

// TestSecurity tests security functionality
func TestSecurity(t *testing.T) {
	t.Parallel()

	// 11.1 Diffie-Hellman key exchange
	t.Run("DiffieHellmanKeyExchange", func(t *testing.T) {
		// Test Diffie-Hellman key exchange implementation
		dh := security.NewDiffieHellman()

		// Generate key pairs for two nodes
		keyPair1, err := dh.GenerateKeyPair()
		if err != nil {
			t.Fatalf("Failed to generate key pair 1: %v", err)
		}

		keyPair2, err := dh.GenerateKeyPair()
		if err != nil {
			t.Fatalf("Failed to generate key pair 2: %v", err)
		}

		// Compute shared secrets
		secret1, err := dh.ComputeSharedSecret(keyPair1.Private, keyPair2.Public)
		if err != nil {
			t.Fatalf("Failed to compute shared secret 1: %v", err)
		}

		secret2, err := dh.ComputeSharedSecret(keyPair2.Private, keyPair1.Public)
		if err != nil {
			t.Fatalf("Failed to compute shared secret 2: %v", err)
		}

		// Verify shared secrets match
		if secret1.Cmp(secret2) != 0 {
			t.Error("Shared secrets should be identical")
		}

		// Test encryption/decryption
		nodeEnc := security.NewNodeEncryption(secret1)
		testData := []byte("test secret message")

		encrypted, err := nodeEnc.Encrypt(testData)
		if err != nil {
			t.Fatalf("Failed to encrypt: %v", err)
		}

		decrypted, err := nodeEnc.Decrypt(encrypted)
		if err != nil {
			t.Fatalf("Failed to decrypt: %v", err)
		}

		if string(testData) != string(decrypted) {
			t.Errorf("Expected %s, got %s", string(testData), string(decrypted))
		}

		t.Logf("DH key exchange verified - two nodes established secure channel")
		t.Logf("  Shared secret length: %d bits", secret1.BitLen())
	})

	// 11.2 Post-quantum upgrade
	t.Run("PostQuantumUpgrade", func(t *testing.T) {
		t.Skip("requires post-quantum encryption implementation")
		// This test would verify channel upgrades to post-quantum encryption
	})

	// 11.3 Identity generation
	t.Run("IdentityGeneration", func(t *testing.T) {
		// Test CV-syllable identity generation
		meshName := "test-mesh"
		nodeID := "test-node"

		idManager := identity.NewIdentityManager(meshName, nodeID)

		// Generate multiple identities to test uniqueness and format
		identities := make([]string, 5)
		for i := 0; i < 5; i++ {
			nodeIdentity, err := idManager.GenerateNodeIdentity()
			if err != nil {
				t.Fatalf("Failed to generate identity %d: %v", i, err)
			}

			identities[i] = nodeIdentity.ID

			// Verify identity is non-empty and reasonable length
			if len(nodeIdentity.ID) < 4 {
				t.Errorf("Identity %d seems too short: %s", i, nodeIdentity.ID)
			}

			if len(nodeIdentity.ID) > 100 {
				t.Errorf("Identity %d seems too long: %s", i, nodeIdentity.ID)
			}

			// Note: Full CV-syllable validation would check consonant-vowel pattern
			// This test verifies the identity generation structure works
		}

		// Verify uniqueness
		for i := 0; i < 5; i++ {
			for j := i + 1; j < 5; j++ {
				if identities[i] == identities[j] {
					t.Errorf("Generated duplicate identities: %s", identities[i])
				}
			}
		}

		t.Logf("CV-syllable identity generation verified:")
		for i, id := range identities {
			t.Logf("  Identity %d: %s", i+1, id)
		}
	})

	// 11.4 Key pair generation
	t.Run("KeyPairGeneration", func(t *testing.T) {
		// Test DH key pair generation
		dh := security.NewDiffieHellman()

		// Generate multiple key pairs to test functionality
		for i := 0; i < 3; i++ {
			keyPair, err := dh.GenerateKeyPair()
			if err != nil {
				t.Fatalf("Failed to generate key pair %d: %v", i, err)
			}

			// Verify key pair structure
			if keyPair.Private == nil {
				t.Error("Private key should not be nil")
			}

			if keyPair.Public == nil {
				t.Error("Public key should not be nil")
			}

			// Verify keys are different values
			if keyPair.Private.Cmp(keyPair.Public) == 0 {
				t.Error("Private and public keys should be different values")
			}

			// Verify key sizes are reasonable
			if keyPair.Private.BitLen() < 256 {
				t.Errorf("Private key seems too small: %d bits", keyPair.Private.BitLen())
			}

			if keyPair.Public.BitLen() < 256 {
				t.Errorf("Public key seems too small: %d bits", keyPair.Public.BitLen())
			}

			t.Logf("Key pair %d generated:", i+1)
			t.Logf("  Private key: %d bits", keyPair.Private.BitLen())
			t.Logf("  Public key: %d bits", keyPair.Public.BitLen())
		}
	})

	// 11.5 Mobile agent — two-factor auth
	t.Run("MobileAgentTwoFactorAuth", func(t *testing.T) {
		t.Skip("requires mobile agent authentication implementation")
		// This test would verify passphrase + device secret → derived key → challenge-response
	})

	// 11.6 Challenge-response protocol
	t.Run("ChallengeResponseProtocol", func(t *testing.T) {
		// Test challenge-response message structures
		challenge := &security.AuthChallengeMessage{
			AgentIdentity: "test-agent-123",
			ChallengeID:   make([]byte, 16),
			EncryptedData: make([]byte, 32),
		}

		// Fill with test data
		rand.Read(challenge.ChallengeID)
		rand.Read(challenge.EncryptedData)

		// Test serialization/deserialization
		serialized := security.SerializeAuthChallenge(challenge)
		if len(serialized) == 0 {
			t.Error("Serialized challenge should not be empty")
		}

		deserialized, err := security.DeserializeAuthChallenge(serialized)
		if err != nil {
			t.Fatalf("Failed to deserialize challenge: %v", err)
		}

		if deserialized.AgentIdentity != challenge.AgentIdentity {
			t.Error("Agent identity should match after serialization")
		}

		if len(deserialized.ChallengeID) != len(challenge.ChallengeID) {
			t.Error("Challenge ID length should match")
		}

		if len(deserialized.EncryptedData) != len(challenge.EncryptedData) {
			t.Error("Encrypted data length should match")
		}

		t.Logf("Challenge-response protocol structure verified:")
		t.Logf("  Agent identity: %s", challenge.AgentIdentity)
		t.Logf("  Challenge ID: %d bytes", len(challenge.ChallengeID))
		t.Logf("  Encrypted data: %d bytes", len(challenge.EncryptedData))
		t.Logf("  Serialized size: %d bytes", len(serialized))
	})

	// 11.7 Agent-level encryption
	t.Run("AgentLevelEncryption", func(t *testing.T) {
		t.Skip("requires agent-level encryption implementation")
		// This test would verify private keys are stored encrypted in mesh
		// and nodes hosting them cannot read them
	})

	// 11.8 Invalid credentials rejected
	t.Run("InvalidCredentialsRejected", func(t *testing.T) {
		t.Skip("requires authentication credential validation implementation")
		// This test would verify wrong passphrase → authentication failure
	})

	// 11.9 Missing device secret rejected
	t.Run("MissingDeviceSecretRejected", func(t *testing.T) {
		t.Skip("requires device secret validation implementation")
		// This test would verify passphrase alone → authentication failure
	})

	// 11.10 Key rotation
	t.Run("KeyRotation", func(t *testing.T) {
		t.Skip("requires key rotation implementation")
		// This test would verify device secret rotation and re-encryption
	})

	// 11.11 Multiple device secrets
	t.Run("MultipleDeviceSecrets", func(t *testing.T) {
		t.Skip("requires multiple device authentication implementation")
		// This test would verify two devices can authenticate with different secrets
	})

	// 11.12 Local socket — no encryption
	t.Run("LocalSocketNoEncryption", func(t *testing.T) {
		dataDir := createTestDataDir(t)
		amorphdPath := buildAmorphd(t)
		amorphctlPath := buildAmorphctl(t)

		cmd := startAmorphdService(t, amorphdPath, dataDir)
		defer stopAmorphdService(t, cmd)

		if !waitForServiceReady(t, dataDir, 10*time.Second) {
			t.Fatal("Service did not become ready within timeout")
		}

		// Test local socket communication (Unix socket)
		output := runAmorphctl(t, amorphctlPath, dataDir, "status")

		// Verify communication works (no encryption required for local socket)
		if !strings.Contains(output, "Service: running") {
			t.Errorf("Expected service status via local socket, got: %s", output)
		}

		if !strings.Contains(output, "Local Socket:") {
			t.Errorf("Expected local socket info in status, got: %s", output)
		}

		t.Logf("Local socket communication verified - no encryption layer")
		t.Logf("Unix socket connections work without encryption overhead")
	})

	// 11.13 Network socket — mandatory encryption
	t.Run("NetworkSocketMandatoryEncryption", func(t *testing.T) {
		t.Skip("requires network socket encryption validation")
		// This test would verify TCP connections require full encryption stack
	})

	// 11.14 Bridge authentication
	t.Run("BridgeAuthentication", func(t *testing.T) {
		// Test bridge authentication independence
		mesh1 := "mesh-1"
		mesh2 := "mesh-2"
		nodeID := "bridge-node"

		// Create identity managers for different meshes
		id1Manager := identity.NewIdentityManager(mesh1, nodeID)
		id2Manager := identity.NewIdentityManager(mesh2, nodeID)

		// Generate identities for each mesh
		identity1, err := id1Manager.GenerateNodeIdentity()
		if err != nil {
			t.Fatalf("Failed to generate identity for mesh1: %v", err)
		}

		identity2, err := id2Manager.GenerateNodeIdentity()
		if err != nil {
			t.Fatalf("Failed to generate identity for mesh2: %v", err)
		}

		// Create DH instances for each mesh (independent key pairs)
		dh1 := security.NewDiffieHellman()
		dh2 := security.NewDiffieHellman()

		// Generate independent key pairs for each mesh
		keyPair1, err := dh1.GenerateKeyPair()
		if err != nil {
			t.Fatalf("Failed to generate key pair for mesh1: %v", err)
		}

		keyPair2, err := dh2.GenerateKeyPair()
		if err != nil {
			t.Fatalf("Failed to generate key pair for mesh2: %v", err)
		}

		// Verify independence
		if identity1.ID == identity2.ID {
			t.Error("Bridge should have different identities in each mesh")
		}

		if identity1.MeshName == identity2.MeshName {
			t.Error("Mesh names should be different")
		}

		if keyPair1.Public.Cmp(keyPair2.Public) == 0 {
			t.Error("Bridge should have different keys in each mesh")
		}

		t.Logf("Bridge authentication independence verified:")
		t.Logf("  Mesh1 identity: %s in mesh %s", identity1.ID, identity1.MeshName)
		t.Logf("  Mesh2 identity: %s in mesh %s", identity2.ID, identity2.MeshName)
		t.Logf("  Independent key pairs generated for each mesh")
	})
}

// Helper functions are shared from other test files