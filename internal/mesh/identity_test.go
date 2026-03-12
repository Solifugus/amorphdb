package mesh

import (
	"strings"
	"testing"
)

func TestGenerateNodeIdentity(t *testing.T) {
	// Test identity generation
	identity, err := GenerateNodeIdentity()
	if err != nil {
		t.Fatalf("GenerateNodeIdentity failed: %v", err)
	}

	if identity == "" {
		t.Error("Expected non-empty identity")
	}

	// Should be valid CV pattern
	if !ValidateIdentity(identity) {
		t.Errorf("Generated identity '%s' is not valid", identity)
	}

	// Should not be blacklisted
	if isBlacklisted(identity) {
		t.Errorf("Generated identity '%s' is blacklisted", identity)
	}

	// Should have syllables separated by hyphens
	parts := strings.Split(identity, "-")
	if len(parts) < 3 || len(parts) > 4 {
		t.Errorf("Expected 3-4 syllables, got %d in '%s'", len(parts), identity)
	}

	t.Logf("Generated identity: %s", identity)
}

func TestGenerateNodeIdentity_Multiple(t *testing.T) {
	// Generate multiple identities to test variety
	identities := make(map[string]bool)

	for i := 0; i < 20; i++ {
		identity, err := GenerateNodeIdentity()
		if err != nil {
			t.Errorf("GenerateNodeIdentity failed on iteration %d: %v", i, err)
			continue
		}

		if identities[identity] {
			t.Logf("Duplicate identity generated (rare but possible): %s", identity)
		}
		identities[identity] = true

		if !ValidateIdentity(identity) {
			t.Errorf("Invalid identity generated: %s", identity)
		}

		if isBlacklisted(identity) {
			t.Errorf("Blacklisted identity generated: %s", identity)
		}
	}

	// Should generate at least some variety
	if len(identities) < 15 {
		t.Errorf("Expected at least 15 unique identities, got %d", len(identities))
	}

	t.Logf("Generated %d unique identities", len(identities))
}

func TestValidateIdentity(t *testing.T) {
	tests := []struct {
		identity string
		expected bool
		reason   string
	}{
		{"ka-ve-lo", true, "valid 3-syllable identity"},
		{"ma-li-no-ka", true, "valid 4-syllable identity"},
		{"be-tu-gan", true, "valid 3-syllable identity with final consonants"},
		{"", false, "empty identity"},
		{"ka", false, "too few syllables"},
		{"ka-ve-lo-ma-no", false, "too many syllables"},
		{"xa-ve-lo", true, "valid identity with 'x' consonant"},
		{"ki-yi-lo", false, "invalid vowel 'y'"},
		{"ka-ve", false, "too few syllables"},
		{"bad-guy", false, "blacklisted content"},
		{"ka_ve_lo", false, "wrong separator (underscore)"},
		{"ka.ve.lo", false, "wrong separator (dot)"},
		{"KA-VE-LO", true, "case should not matter for validation"},
		{"123-456", false, "numeric content"},
		{"ka-ve-lo-", false, "trailing separator"},
		{"-ka-ve-lo", false, "leading separator"},
	}

	for _, test := range tests {
		result := ValidateIdentity(test.identity)
		if result != test.expected {
			t.Errorf("ValidateIdentity('%s') = %v, expected %v (%s)",
				test.identity, result, test.expected, test.reason)
		}
	}
}

func TestGenerateCVSyllable(t *testing.T) {
	for i := 0; i < 50; i++ {
		syllable, err := generateCVSyllable()
		if err != nil {
			t.Fatalf("generateCVSyllable failed: %v", err)
		}

		if len(syllable) < 2 || len(syllable) > 3 {
			t.Errorf("Invalid syllable length: '%s' (length %d)", syllable, len(syllable))
		}

		if !isValidCVSyllable(syllable) {
			t.Errorf("Invalid CV syllable generated: '%s'", syllable)
		}
	}
}

func TestIsValidCVSyllable(t *testing.T) {
	tests := []struct {
		syllable string
		expected bool
		reason   string
	}{
		{"ka", true, "valid CV"},
		{"ben", true, "valid CVC"},
		{"lo", true, "valid CV"},
		{"", false, "empty syllable"},
		{"k", false, "too short"},
		{"kato", false, "too long"},
		{"ak", false, "starts with vowel"},
		{"ko", true, "valid CV"},
		{"kx", false, "invalid vowel"},
		{"xo", true, "valid CV with x consonant"},
		{"kab", true, "valid CVC"},
		{"kay", false, "invalid final consonant"},
	}

	for _, test := range tests {
		result := isValidCVSyllable(test.syllable)
		if result != test.expected {
			t.Errorf("isValidCVSyllable('%s') = %v, expected %v (%s)",
				test.syllable, result, test.expected, test.reason)
		}
	}
}

func TestIsBlacklisted(t *testing.T) {
	tests := []struct {
		identity   string
		expected   bool
		reason     string
	}{
		{"ka-ve-lo", false, "clean identity"},
		{"bad-guy", true, "contains 'bad'"},
		{"my-cat-is-nice", true, "contains 'cat'"},
		{"go-dog-go", true, "contains 'dog'"},
		{"he-said-bad-things", true, "contains 'bad'"},
		{"nice-day", false, "no blacklisted content"},
		{"be-ta-lo", false, "clean identity"},
		{"ra-gun-ka", true, "contains 'gun'"},
		{"pi-got-me", true, "contains 'got'"},
		{"no-war-zone", true, "contains 'war'"},
		{"red-car", true, "contains 'red'"},
		{"li-net-mo", true, "contains 'net'"},
	}

	for _, test := range tests {
		result := isBlacklisted(test.identity)
		if result != test.expected {
			t.Errorf("isBlacklisted('%s') = %v, expected %v (%s)",
				test.identity, result, test.expected, test.reason)
		}
	}
}

func TestGetIdentityParts(t *testing.T) {
	identity := "ka-ve-lo-ma"
	parts := GetIdentityParts(identity)

	expected := []string{"ka", "ve", "lo", "ma"}
	if len(parts) != len(expected) {
		t.Errorf("Expected %d parts, got %d", len(expected), len(parts))
		return
	}

	for i, part := range parts {
		if part != expected[i] {
			t.Errorf("Part %d: expected '%s', got '%s'", i, expected[i], part)
		}
	}
}

func TestGenerateKeyPair(t *testing.T) {
	keyPair, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair failed: %v", err)
	}

	if keyPair == nil {
		t.Fatal("Expected non-nil key pair")
	}

	if len(keyPair.PublicKey) != 32 {
		t.Errorf("Expected public key length 32, got %d", len(keyPair.PublicKey))
	}

	if len(keyPair.PrivateKey) != 32 {
		t.Errorf("Expected private key length 32, got %d", len(keyPair.PrivateKey))
	}

	// Keys should be different
	publicStr := string(keyPair.PublicKey)
	privateStr := string(keyPair.PrivateKey)
	if publicStr == privateStr {
		t.Error("Public and private keys should be different")
	}

	// Generate another pair to ensure randomness
	keyPair2, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("Second GenerateKeyPair failed: %v", err)
	}

	public1Str := string(keyPair.PublicKey)
	public2Str := string(keyPair2.PublicKey)
	if public1Str == public2Str {
		t.Error("Different key pairs should have different public keys")
	}
}

func TestConsonantVowelArrays(t *testing.T) {
	// Verify consonant array has expected entries
	if len(consonants) == 0 {
		t.Error("Consonants array should not be empty")
	}

	// Check for expected consonants
	expectedConsonants := []string{"b", "c", "d", "f", "g", "h", "j", "k", "l", "m", "n", "p", "r", "s", "t", "v", "w", "x", "z"}
	for _, expected := range expectedConsonants {
		if !contains(consonants, expected) {
			t.Errorf("Expected consonant '%s' not found", expected)
		}
	}

	// Verify vowel array has expected entries
	if len(vowels) == 0 {
		t.Error("Vowels array should not be empty")
	}

	expectedVowels := []string{"a", "e", "i", "o", "u"}
	for _, expected := range expectedVowels {
		if !contains(vowels, expected) {
			t.Errorf("Expected vowel '%s' not found", expected)
		}
	}
}

func TestBlacklistCoverage(t *testing.T) {
	// Test that all blacklisted syllables are properly detected
	testCases := []string{
		"bad", "bat", "bit", "cat", "dog", "gun", "war", "wet", "zip",
	}

	for _, blacklisted := range testCases {
		if !blacklistedSyllables[blacklisted] {
			t.Errorf("Expected '%s' to be in blacklist", blacklisted)
		}

		// Test as part of longer identity
		identity := "ka-" + blacklisted + "-lo"
		if !isBlacklisted(identity) {
			t.Errorf("Identity '%s' should be blacklisted", identity)
		}
	}
}

func TestCryptoRandInt(t *testing.T) {
	// Test boundary conditions
	_, err := cryptoRandInt(0)
	if err == nil {
		t.Error("Expected error for max=0")
	}

	_, err = cryptoRandInt(-1)
	if err == nil {
		t.Error("Expected error for negative max")
	}

	// Test valid range
	for i := 0; i < 100; i++ {
		result, err := cryptoRandInt(10)
		if err != nil {
			t.Errorf("cryptoRandInt(10) failed: %v", err)
		}

		if result < 0 || result >= 10 {
			t.Errorf("cryptoRandInt(10) returned %d, expected 0-9", result)
		}
	}

	// Test single value range
	result, err := cryptoRandInt(1)
	if err != nil {
		t.Errorf("cryptoRandInt(1) failed: %v", err)
	}

	if result != 0 {
		t.Errorf("cryptoRandInt(1) returned %d, expected 0", result)
	}
}
