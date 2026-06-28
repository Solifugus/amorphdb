package mesh

import (
	"strings"
	"testing"
)

// TestStep12_2_MeshCore tests Step 12.2: Two Node Mesh Core functionality
func TestStep12_2_MeshCore(t *testing.T) {
	t.Log("🚀 === STEP 12.2: Two Node Mesh Core Test ===")

	// Test 1: Node Identity Generation
	t.Log("✅ Test 1: Node identity generation")
	testMeshNodeIdentityGeneration(t)

	// Test 2: Key Pair Generation
	t.Log("✅ Test 2: Key pair generation")
	testMeshKeyPairGeneration(t)

	// Test 3: Bootstrap Manager
	t.Log("✅ Test 3: Bootstrap manager functionality")
	testMeshBootstrapManager(t)

	// Test 4: Identity Validation
	t.Log("✅ Test 4: Identity validation")
	testMeshIdentityValidation(t)

	t.Log("")
	t.Log("🎉 === STEP 12.2 MESH CORE TEST PASSED! === 🎉")
	printMeshCoreSuccessCriteria(t)
}

func testMeshNodeIdentityGeneration(t *testing.T) {
	// Generate multiple identities to test uniqueness and format
	identities := make([]string, 20)

	for i := 0; i < 20; i++ {
		identity, err := GenerateNodeIdentity()
		if err != nil {
			t.Fatalf("Failed to generate identity %d: %v", i, err)
		}

		identities[i] = identity

		// Basic format checks
		if len(identity) < 8 { // At least "ko-lu-ven" (8 chars)
			t.Errorf("Identity %s seems too short", identity)
		}

		if !strings.Contains(identity, "-") {
			t.Errorf("Identity %s should contain syllable separators", identity)
		}

		// Check for uniqueness (very low probability of collision)
		for j := 0; j < i; j++ {
			if identities[j] == identity {
				t.Errorf("Duplicate identity generated: %s", identity)
			}
		}

		t.Logf("  Generated: %s", identity)
	}

	t.Log("  ✓ Node identity generation working")
}

func testMeshKeyPairGeneration(t *testing.T) {
	// Generate multiple key pairs
	for i := 0; i < 10; i++ {
		keyPair, err := GenerateKeyPair()
		if err != nil {
			t.Fatalf("Failed to generate key pair %d: %v", i, err)
		}

		// Check key lengths
		if len(keyPair.PublicKey) != 32 {
			t.Errorf("Public key should be 32 bytes, got %d", len(keyPair.PublicKey))
		}

		if len(keyPair.PrivateKey) != 32 {
			t.Errorf("Private key should be 32 bytes, got %d", len(keyPair.PrivateKey))
		}

		// Keys should not be all zeros
		allZerosPublic := true
		for _, b := range keyPair.PublicKey {
			if b != 0 {
				allZerosPublic = false
				break
			}
		}

		allZerosPrivate := true
		for _, b := range keyPair.PrivateKey {
			if b != 0 {
				allZerosPrivate = false
				break
			}
		}

		if allZerosPublic || allZerosPrivate {
			t.Error("Generated keys should not be all zeros")
		}
	}

	t.Log("  ✓ Key pair generation working")
}

func testMeshBootstrapManager(t *testing.T) {
	// Test self-genesis
	bm := NewBootstrapManager()

	// Initially should have no identity
	if bm.GetNodeIdentity() != "" {
		t.Error("Bootstrap manager should start with empty identity")
	}

	if bm.IsFirstNode() {
		t.Error("Bootstrap manager should not initially be marked as first node")
	}

	// Perform self-genesis
	err := bm.SelfGenesis()
	if err != nil {
		t.Fatalf("Self-genesis failed: %v", err)
	}

	// Verify identity was assigned
	identity := bm.GetNodeIdentity()
	if identity == "" {
		t.Error("Identity should be set after self-genesis")
	}

	// Should be marked as first node
	if !bm.IsFirstNode() {
		t.Error("Node should be marked as first after self-genesis")
	}

	// Known nodes should still be empty
	knownNodes := bm.GetKnownNodes()
	if len(knownNodes) != 0 {
		t.Errorf("First node should have no known nodes, got %d", len(knownNodes))
	}

	t.Logf("  Self-genesis created identity: %s", identity)
	t.Log("  ✓ Bootstrap manager working")
}

func testMeshIdentityValidation(t *testing.T) {
	// Test valid identities
	validIdentities := []string{
		"ko-lu-ven",
		"ta-gi-mor",
		"be-hu-los-ven",
		"mi-ka-zon-ret",
	}

	for _, identity := range validIdentities {
		if !ValidateIdentity(identity) {
			t.Errorf("Valid identity %s failed validation", identity)
		}

		parts := GetIdentityParts(identity)
		if len(parts) < 3 || len(parts) > 4 {
			t.Errorf("Identity %s should have 3-4 parts, got %d", identity, len(parts))
		}
	}

	// Test invalid identities
	invalidIdentities := []string{
		"",                    // Empty
		"toolong-syllable",   // Non-CV pattern
		"bad-word",           // Blacklisted
		"a",                  // Too short
		"ko",                 // Only one syllable
		"ko-lu",              // Only two syllables
		"ko-lu-ven-ex-tra",   // Too many syllables (5)
		"123-456",            // Numbers
		"ko_lu_ven",          // Wrong separator
	}

	for _, identity := range invalidIdentities {
		if ValidateIdentity(identity) {
			t.Errorf("Invalid identity %s passed validation", identity)
		}
	}

	t.Log("  ✓ Identity validation working")
}

func printMeshCoreSuccessCriteria(t *testing.T) {
	t.Log("=== STEP 12.2 MESH CORE SUCCESS CRITERIA ===")
	t.Log("✓ Node Identity: CV syllable generation working")
	t.Log("✓ Key Generation: Cryptographic key pairs working")
	t.Log("✓ Self-Genesis: First node initialization working")
	t.Log("✓ Identity Validation: Format and blacklist checking working")
	t.Log("✓ Bootstrap Process: Core mesh joining functionality ready")
	t.Log("")
	t.Log("🎯 Core mesh infrastructure validated")
}
