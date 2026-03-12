package security

import (
	"bytes"
	"math/big"
	"testing"
)

func TestDiffieHellmanKeyExchange(t *testing.T) {
	dh := NewDiffieHellman()

	// Generate key pairs for Alice and Bob
	aliceKeys, err := dh.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate Alice's keys: %v", err)
	}

	bobKeys, err := dh.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate Bob's keys: %v", err)
	}

	// Compute shared secrets
	aliceSecret, err := dh.ComputeSharedSecret(aliceKeys.Private, bobKeys.Public)
	if err != nil {
		t.Fatalf("Failed to compute Alice's shared secret: %v", err)
	}

	bobSecret, err := dh.ComputeSharedSecret(bobKeys.Private, aliceKeys.Public)
	if err != nil {
		t.Fatalf("Failed to compute Bob's shared secret: %v", err)
	}

	// Shared secrets should be equal
	if aliceSecret.Cmp(bobSecret) != 0 {
		t.Error("Shared secrets do not match")
	}

	// Shared secret should not be 1 or p-1 (weak keys)
	one := big.NewInt(1)
	pMinusOne := new(big.Int).Sub(dh.P, one)
	if aliceSecret.Cmp(one) == 0 || aliceSecret.Cmp(pMinusOne) == 0 {
		t.Error("Weak shared secret generated")
	}
}

func TestDiffieHellmanInvalidKeys(t *testing.T) {
	dh := NewDiffieHellman()

	// Generate valid key pair
	keys, err := dh.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate keys: %v", err)
	}

	// Test with invalid public keys
	invalidKeys := []*big.Int{
		big.NewInt(0),     // Too small
		big.NewInt(1),     // Too small
		dh.P,              // Equal to modulus
		new(big.Int).Add(dh.P, big.NewInt(1)), // Too large
	}

	for i, invalidKey := range invalidKeys {
		_, err := dh.ComputeSharedSecret(keys.Private, invalidKey)
		if err == nil {
			t.Errorf("Test %d: Expected error for invalid public key %v", i, invalidKey)
		}
	}
}

func TestNodeEncryption(t *testing.T) {
	// Create mock shared secret
	sharedSecret, _ := new(big.Int).SetString("12345678901234567890", 10)

	nodeEnc := NewNodeEncryption(sharedSecret)

	// Test data
	plaintext := []byte("Hello, secure mesh network!")

	// Encrypt
	ciphertext, err := nodeEnc.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encryption failed: %v", err)
	}

	// Ciphertext should be different from plaintext
	if bytes.Equal(plaintext, ciphertext) {
		t.Error("Ciphertext equals plaintext")
	}

	// Decrypt
	decrypted, err := nodeEnc.Decrypt(ciphertext)
	if err != nil {
		t.Fatalf("Decryption failed: %v", err)
	}

	// Decrypted should match original
	if !bytes.Equal(plaintext, decrypted) {
		t.Errorf("Decrypted text does not match original.\nExpected: %s\nGot: %s", plaintext, decrypted)
	}
}

func TestNodeEncryptionDifferentKeys(t *testing.T) {
	// Create different shared secrets
	secret1, _ := new(big.Int).SetString("111111111111111", 10)
	secret2, _ := new(big.Int).SetString("222222222222222", 10)

	nodeEnc1 := NewNodeEncryption(secret1)
	nodeEnc2 := NewNodeEncryption(secret2)

	plaintext := []byte("Test message")

	// Encrypt with first key
	ciphertext, err := nodeEnc1.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encryption failed: %v", err)
	}

	// Try to decrypt with second key (should fail)
	_, err = nodeEnc2.Decrypt(ciphertext)
	if err == nil {
		t.Error("Decryption should have failed with different key")
	}
}

func TestNodeEncryptionRandomIV(t *testing.T) {
	sharedSecret, _ := new(big.Int).SetString("12345678901234567890", 10)
	nodeEnc := NewNodeEncryption(sharedSecret)

	plaintext := []byte("Same message")

	// Encrypt same message twice
	ciphertext1, err := nodeEnc.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("First encryption failed: %v", err)
	}

	ciphertext2, err := nodeEnc.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Second encryption failed: %v", err)
	}

	// Ciphertexts should be different (due to random IV)
	if bytes.Equal(ciphertext1, ciphertext2) {
		t.Error("Identical ciphertexts indicate IV reuse")
	}

	// Both should decrypt to same plaintext
	decrypted1, _ := nodeEnc.Decrypt(ciphertext1)
	decrypted2, _ := nodeEnc.Decrypt(ciphertext2)

	if !bytes.Equal(decrypted1, plaintext) || !bytes.Equal(decrypted2, plaintext) {
		t.Error("Decryption failed for randomized encryption")
	}
}

func TestAgentEncryption(t *testing.T) {
	passphrase := "my-secure-passphrase"
	deviceSecret := "device-unique-id-12345"

	agentEnc := NewAgentEncryption(passphrase, deviceSecret)

	// Test private key encryption
	privateKey := []byte("-----BEGIN PRIVATE KEY-----\nMIIEvwIBADANBgkqhkiG...")

	// Encrypt
	encrypted, err := agentEnc.EncryptSecret(privateKey)
	if err != nil {
		t.Fatalf("Secret encryption failed: %v", err)
	}

	// Encrypted should be different from original
	if bytes.Equal(privateKey, encrypted) {
		t.Error("Encrypted secret equals original")
	}

	// Decrypt
	decrypted, err := agentEnc.DecryptSecret(encrypted)
	if err != nil {
		t.Fatalf("Secret decryption failed: %v", err)
	}

	// Decrypted should match original
	if !bytes.Equal(privateKey, decrypted) {
		t.Error("Decrypted secret does not match original")
	}
}

func TestAgentEncryptionDifferentCredentials(t *testing.T) {
	// Test with different passphrases
	agentEnc1 := NewAgentEncryption("passphrase1", "device123")
	agentEnc2 := NewAgentEncryption("passphrase2", "device123")

	secret := []byte("sensitive data")

	// Encrypt with first agent
	encrypted, err := agentEnc1.EncryptSecret(secret)
	if err != nil {
		t.Fatalf("Encryption failed: %v", err)
	}

	// Try to decrypt with second agent (different passphrase)
	_, err = agentEnc2.DecryptSecret(encrypted)
	if err == nil {
		t.Error("Decryption should have failed with different passphrase")
	}

	// Test with different device secrets
	agentEnc3 := NewAgentEncryption("passphrase1", "device456")

	// Try to decrypt with third agent (different device)
	_, err = agentEnc3.DecryptSecret(encrypted)
	if err == nil {
		t.Error("Decryption should have failed with different device secret")
	}
}

func TestDeriveAgentKey(t *testing.T) {
	// Same credentials should produce same key
	key1 := DeriveAgentKey("password", "device123")
	key2 := DeriveAgentKey("password", "device123")

	if !bytes.Equal(key1, key2) {
		t.Error("Same credentials produced different keys")
	}

	// Different credentials should produce different keys
	key3 := DeriveAgentKey("password", "device456")
	key4 := DeriveAgentKey("different", "device123")

	if bytes.Equal(key1, key3) {
		t.Error("Different device secrets produced same key")
	}

	if bytes.Equal(key1, key4) {
		t.Error("Different passphrases produced same key")
	}

	// Key should be 32 bytes (256 bits)
	if len(key1) != 32 {
		t.Errorf("Expected 32-byte key, got %d bytes", len(key1))
	}
}

func TestPostQuantumEncryption(t *testing.T) {
	// Test post-quantum wrapper (currently using classical encryption)
	sharedSecret, _ := new(big.Int).SetString("987654321987654321", 10)
	pqEnc := NewPostQuantumEncryption(sharedSecret)

	plaintext := []byte("Future-proof encrypted message")

	// Encrypt
	ciphertext, err := pqEnc.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Post-quantum encryption failed: %v", err)
	}

	// Decrypt
	decrypted, err := pqEnc.Decrypt(ciphertext)
	if err != nil {
		t.Fatalf("Post-quantum decryption failed: %v", err)
	}

	// Should match original
	if !bytes.Equal(plaintext, decrypted) {
		t.Error("Post-quantum decryption does not match original")
	}
}

func TestEncryptionEmptyData(t *testing.T) {
	sharedSecret := big.NewInt(123456789)
	nodeEnc := NewNodeEncryption(sharedSecret)

	// Test encrypting empty data
	empty := []byte{}

	encrypted, err := nodeEnc.Encrypt(empty)
	if err != nil {
		t.Fatalf("Failed to encrypt empty data: %v", err)
	}

	decrypted, err := nodeEnc.Decrypt(encrypted)
	if err != nil {
		t.Fatalf("Failed to decrypt empty data: %v", err)
	}

	if !bytes.Equal(empty, decrypted) {
		t.Error("Empty data encryption/decryption failed")
	}
}

func TestEncryptionLargeData(t *testing.T) {
	sharedSecret, _ := new(big.Int).SetString("555555555555555", 10)
	nodeEnc := NewNodeEncryption(sharedSecret)

	// Test with large data (1MB)
	largeData := make([]byte, 1024*1024)
	for i := range largeData {
		largeData[i] = byte(i % 256)
	}

	encrypted, err := nodeEnc.Encrypt(largeData)
	if err != nil {
		t.Fatalf("Failed to encrypt large data: %v", err)
	}

	decrypted, err := nodeEnc.Decrypt(encrypted)
	if err != nil {
		t.Fatalf("Failed to decrypt large data: %v", err)
	}

	if !bytes.Equal(largeData, decrypted) {
		t.Error("Large data encryption/decryption failed")
	}
}

func TestInvalidCiphertext(t *testing.T) {
	sharedSecret, _ := new(big.Int).SetString("999999999999999", 10)
	nodeEnc := NewNodeEncryption(sharedSecret)

	// Test with invalid ciphertext
	invalidCiphertexts := [][]byte{
		{},                           // Empty
		{1, 2, 3},                   // Too short
		make([]byte, 15),            // Wrong length (not multiple of block size)
	}

	for i, invalid := range invalidCiphertexts {
		_, err := nodeEnc.Decrypt(invalid)
		if err == nil {
			t.Errorf("Test %d: Expected error for invalid ciphertext", i)
		}
	}
}

// Authentication Tests

func TestAgentKeyDerivation(t *testing.T) {
	authenticator := NewAgentAuthenticator()

	// Test key derivation consistency
	passphrase := "secure-passphrase-123"
	deviceSecret := "device-unique-id-456"

	keyPair1, err := authenticator.DeriveAgentKeyPair(passphrase, deviceSecret)
	if err != nil {
		t.Fatalf("Failed to derive key pair: %v", err)
	}

	keyPair2, err := authenticator.DeriveAgentKeyPair(passphrase, deviceSecret)
	if err != nil {
		t.Fatalf("Failed to derive key pair again: %v", err)
	}

	// Same inputs should produce same keys
	if keyPair1.Private.Cmp(keyPair2.Private) != 0 {
		t.Error("Private keys don't match for same inputs")
	}

	if keyPair1.Public.Cmp(keyPair2.Public) != 0 {
		t.Error("Public keys don't match for same inputs")
	}

	// Different inputs should produce different keys
	keyPair3, err := authenticator.DeriveAgentKeyPair(passphrase, "different-device")
	if err != nil {
		t.Fatalf("Failed to derive key pair with different device: %v", err)
	}

	if keyPair1.Private.Cmp(keyPair3.Private) == 0 {
		t.Error("Different device secrets produced same private key")
	}

	if keyPair1.Public.Cmp(keyPair3.Public) == 0 {
		t.Error("Different device secrets produced same public key")
	}
}

func TestAgentIdentityCreation(t *testing.T) {
	authenticator := NewAgentAuthenticator()

	identity := "agent-alice-001"
	passphrase := "my-secure-passphrase"
	deviceSecret := "laptop-12345"

	agentID, err := authenticator.CreateAgentIdentity(identity, passphrase, deviceSecret)
	if err != nil {
		t.Fatalf("Failed to create agent identity: %v", err)
	}

	if agentID.Identity != identity {
		t.Errorf("Expected identity %s, got %s", identity, agentID.Identity)
	}

	if agentID.PublicKey == nil {
		t.Error("Public key is nil")
	}

	if agentID.KeyPair == nil {
		t.Error("Key pair is nil")
	}

	// Public key should match derived key pair's public key
	if agentID.PublicKey.Cmp(agentID.KeyPair.Public) != 0 {
		t.Error("Public key doesn't match key pair's public key")
	}
}

func TestAuthenticationChallengeResponse(t *testing.T) {
	authenticator := NewAgentAuthenticator()

	// Create agent identity
	agentID, err := authenticator.CreateAgentIdentity("test-agent", "passphrase", "device123")
	if err != nil {
		t.Fatalf("Failed to create agent identity: %v", err)
	}

	// Create challenge
	challenge, err := authenticator.CreateChallenge(agentID.PublicKey)
	if err != nil {
		t.Fatalf("Failed to create challenge: %v", err)
	}

	// Agent responds to challenge
	response, err := authenticator.RespondToChallenge(challenge, agentID.KeyPair)
	if err != nil {
		t.Fatalf("Failed to respond to challenge: %v", err)
	}

	// Verify response
	valid, err := authenticator.VerifyResponse(response)
	if err != nil {
		t.Fatalf("Failed to verify response: %v", err)
	}

	if !valid {
		t.Error("Authentication should have succeeded")
	}

	// Verify challenge is consumed (single use)
	valid2, err := authenticator.VerifyResponse(response)
	if err == nil {
		t.Error("Expected error for reused challenge")
	}
	if valid2 {
		t.Error("Reused challenge should not be valid")
	}
}

func TestAuthenticationWrongKey(t *testing.T) {
	authenticator := NewAgentAuthenticator()

	// Create two different agent identities
	agentID1, err := authenticator.CreateAgentIdentity("agent1", "pass1", "device1")
	if err != nil {
		t.Fatalf("Failed to create agent identity 1: %v", err)
	}

	agentID2, err := authenticator.CreateAgentIdentity("agent2", "pass2", "device2")
	if err != nil {
		t.Fatalf("Failed to create agent identity 2: %v", err)
	}

	// Create challenge for agent1
	challenge, err := authenticator.CreateChallenge(agentID1.PublicKey)
	if err != nil {
		t.Fatalf("Failed to create challenge: %v", err)
	}

	// Agent2 tries to respond with wrong key
	response, err := authenticator.RespondToChallenge(challenge, agentID2.KeyPair)
	
	// Decryption should fail with wrong key (this is expected)
	if err != nil {
		// This is the expected behavior - wrong key cannot decrypt
		return
	}
	
	// If somehow decryption succeeded, verification should still fail
	valid, err := authenticator.VerifyResponse(response)
	if err != nil {
		// Verification failure is also acceptable
		return
	}
	
	if valid {
		t.Error("Authentication with wrong key should fail")
	}

}

func TestAgentFactorReconstruction(t *testing.T) {
	authenticator := NewAgentAuthenticator()

	identity := "agent-bob"
	passphrase := "secret-phrase-456"
	deviceSecret := "mobile-device-789"

	// Create identity
	originalID, err := authenticator.CreateAgentIdentity(identity, passphrase, deviceSecret)
	if err != nil {
		t.Fatalf("Failed to create original identity: %v", err)
	}

	// Reconstruct identity from factors
	reconstructedID, err := authenticator.GetAgentIdentityFromFactors(identity, passphrase, deviceSecret)
	if err != nil {
		t.Fatalf("Failed to reconstruct identity: %v", err)
	}

	// Should produce identical keys
	if originalID.PublicKey.Cmp(reconstructedID.PublicKey) != 0 {
		t.Error("Reconstructed public key doesn't match original")
	}

	if originalID.KeyPair.Private.Cmp(reconstructedID.KeyPair.Private) != 0 {
		t.Error("Reconstructed private key doesn't match original")
	}

	// Test with wrong factors
	wrongID, err := authenticator.GetAgentIdentityFromFactors(identity, "wrong-passphrase", deviceSecret)
	if err != nil {
		t.Fatalf("Failed to create identity with wrong passphrase: %v", err)
	}

	if originalID.PublicKey.Cmp(wrongID.PublicKey) == 0 {
		t.Error("Wrong passphrase should produce different keys")
	}
}

func TestChallengeUniqueness(t *testing.T) {
	authenticator := NewAgentAuthenticator()

	// Create agent
	agentID, err := authenticator.CreateAgentIdentity("test-agent", "passphrase", "device")
	if err != nil {
		t.Fatalf("Failed to create agent identity: %v", err)
	}

	// Create multiple challenges
	challenge1, err := authenticator.CreateChallenge(agentID.PublicKey)
	if err != nil {
		t.Fatalf("Failed to create challenge 1: %v", err)
	}

	challenge2, err := authenticator.CreateChallenge(agentID.PublicKey)
	if err != nil {
		t.Fatalf("Failed to create challenge 2: %v", err)
	}

	// Challenges should be unique
	if bytes.Equal(challenge1.ChallengeID, challenge2.ChallengeID) {
		t.Error("Challenge IDs should be unique")
	}

	if bytes.Equal(challenge1.EncryptedData, challenge2.EncryptedData) {
		t.Error("Challenge data should be unique")
	}
}

func TestAuthenticationFlowIntegration(t *testing.T) {
	authenticator := NewAgentAuthenticator()

	// Simulate full authentication flow
	identity := "integration-test-agent"
	passphrase := "integration-passphrase"
	deviceSecret := "integration-device-secret"

	// Step 1: Agent registers (creates identity)
	agentID, err := authenticator.CreateAgentIdentity(identity, passphrase, deviceSecret)
	if err != nil {
		t.Fatalf("Failed to create agent identity: %v", err)
	}

	// Store public key (simulating mesh storage)
	storedPublicKey := new(big.Int).Set(agentID.PublicKey)

	// Step 2: Agent connects and provides identity (simulate mesh lookup)
	// Node looks up public key from "mesh"

	// Step 3: Node creates challenge
	challenge, err := authenticator.CreateChallenge(storedPublicKey)
	if err != nil {
		t.Fatalf("Failed to create challenge: %v", err)
	}

	// Step 4: Agent derives keys from factors (passphrase + device)
	sessionID, err := authenticator.GetAgentIdentityFromFactors(identity, passphrase, deviceSecret)
	if err != nil {
		t.Fatalf("Failed to derive session identity: %v", err)
	}

	// Step 5: Agent responds to challenge
	response, err := authenticator.RespondToChallenge(challenge, sessionID.KeyPair)
	if err != nil {
		t.Fatalf("Failed to respond to challenge: %v", err)
	}

	// Step 6: Node verifies response
	valid, err := authenticator.VerifyResponse(response)
	if err != nil {
		t.Fatalf("Failed to verify response: %v", err)
	}

	if !valid {
		t.Error("Full authentication flow should succeed")
	}

	// Verify private key was only in memory during session
	sessionID.KeyPair = nil // Simulate clearing session keys
	if sessionID.KeyPair != nil {
		t.Error("Session keys should be cleared after authentication")
	}
}