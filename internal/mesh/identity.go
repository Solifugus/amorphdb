package mesh

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"strings"
)

// CV syllable patterns for generating pronounceable node identities
var consonants = []string{
	"b", "c", "d", "f", "g", "h", "j", "k", "l", "m",
	"n", "p", "r", "s", "t", "v", "w", "x", "z",
}

var vowels = []string{
	"a", "e", "i", "o", "u",
}

// Blacklisted syllable combinations to avoid offensive or confusing identities
var blacklistedSyllables = map[string]bool{
	"bad":   true,
	"bat":   true,
	"bit":   true,
	"bot":   true,
	"but":   true,
	"cat":   true,
	"cot":   true,
	"cut":   true,
	"dam":   true,
	"dig":   true,
	"dog":   true,
	"dug":   true,
	"fat":   true,
	"fig":   true,
	"fog":   true,
	"got":   true,
	"gun":   true,
	"gut":   true,
	"hat":   true,
	"hog":   true,
	"hot":   true,
	"hut":   true,
	"jag":   true,
	"jog":   true,
	"jut":   true,
	"keg":   true,
	"kid":   true,
	"lag":   true,
	"leg":   true,
	"log":   true,
	"lot":   true,
	"lug":   true,
	"mad":   true,
	"mat":   true,
	"met":   true,
	"mop":   true,
	"mud":   true,
	"mug":   true,
	"net":   true,
	"nit":   true,
	"not":   true,
	"nut":   true,
	"pad":   true,
	"pat":   true,
	"pet":   true,
	"pig":   true,
	"pit":   true,
	"pot":   true,
	"pug":   true,
	"put":   true,
	"rag":   true,
	"rat":   true,
	"red":   true,
	"rig":   true,
	"rot":   true,
	"run":   true,
	"rut":   true,
	"sad":   true,
	"sat":   true,
	"set":   true,
	"sit":   true,
	"sod":   true,
	"sun":   true,
	"tag":   true,
	"tan":   true,
	"ten":   true,
	"tin":   true,
	"top":   true,
	"tug":   true,
	"van":   true,
	"vet":   true,
	"vex":   true,
	"wag":   true,
	"war":   true,
	"wet":   true,
	"wig":   true,
	"win":   true,
	"wit":   true,
	"zap":   true,
	"zen":   true,
	"zip":   true,
}

// GenerateNodeIdentity creates a CV syllable-based node identity
func GenerateNodeIdentity() (string, error) {
	const maxAttempts = 1000

	for attempt := 0; attempt < maxAttempts; attempt++ {
		identity, err := generateCVIdentity()
		if err != nil {
			return "", err
		}

		// Check against blacklist
		if !isBlacklisted(identity) {
			return identity, nil
		}
	}

	return "", fmt.Errorf("failed to generate non-blacklisted identity after %d attempts", maxAttempts)
}

// generateCVIdentity generates a CV syllable pattern identity
func generateCVIdentity() (string, error) {
	// Generate 3-4 CV syllables for a pronounceable identity
	syllableCount, err := cryptoRandInt(2)
	if err != nil {
		return "", err
	}
	syllableCount += 3 // 3 or 4 syllables

	var syllables []string

	for i := 0; i < syllableCount; i++ {
		syllable, err := generateCVSyllable()
		if err != nil {
			return "", err
		}
		syllables = append(syllables, syllable)
	}

	return strings.Join(syllables, "-"), nil
}

// generateCVSyllable generates a single CV (Consonant-Vowel) syllable
func generateCVSyllable() (string, error) {
	// Select random consonant
	consonantIdx, err := cryptoRandInt(len(consonants))
	if err != nil {
		return "", err
	}

	// Select random vowel
	vowelIdx, err := cryptoRandInt(len(vowels))
	if err != nil {
		return "", err
	}

	// 30% chance of adding a final consonant (CVC pattern)
	hasFinalConsonant, err := cryptoRandInt(10)
	if err != nil {
		return "", err
	}

	syllable := consonants[consonantIdx] + vowels[vowelIdx]

	if hasFinalConsonant < 3 { // 30% chance
		finalConsonantIdx, err := cryptoRandInt(len(consonants))
		if err != nil {
			return "", err
		}
		syllable += consonants[finalConsonantIdx]
	}

	return syllable, nil
}

// isBlacklisted checks if the identity contains blacklisted syllables
func isBlacklisted(identity string) bool {
	// Check each part of the identity
	parts := strings.Split(identity, "-")
	for _, part := range parts {
		if blacklistedSyllables[strings.ToLower(part)] {
			return true
		}

		// Also check for substring matches in longer parts
		lowerPart := strings.ToLower(part)
		for blacklisted := range blacklistedSyllables {
			if strings.Contains(lowerPart, blacklisted) {
				return true
			}
		}
	}

	return false
}

// cryptoRandInt generates a cryptographically secure random integer in range [0, max)
func cryptoRandInt(max int) (int, error) {
	if max <= 0 {
		return 0, fmt.Errorf("max must be positive")
	}

	n, err := rand.Int(rand.Reader, big.NewInt(int64(max)))
	if err != nil {
		return 0, err
	}

	return int(n.Int64()), nil
}

// GenerateKeyPair generates a cryptographic key pair for the node
func GenerateKeyPair() (*KeyPair, error) {
	// Generate a simple 32-byte key pair (placeholder for real crypto)
	// In production, this would use proper elliptic curve or RSA keys

	publicKey := make([]byte, 32)
	_, err := rand.Read(publicKey)
	if err != nil {
		return nil, fmt.Errorf("failed to generate public key: %w", err)
	}

	privateKey := make([]byte, 32)
	_, err = rand.Read(privateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to generate private key: %w", err)
	}

	return &KeyPair{
		PublicKey:  publicKey,
		PrivateKey: privateKey,
	}, nil
}

// ValidateIdentity checks if a node identity follows the CV syllable pattern
func ValidateIdentity(identity string) bool {
	if identity == "" {
		return false
	}

	// Normalize to lowercase for validation
	lowerIdentity := strings.ToLower(identity)

	// Check for blacklisted content
	if isBlacklisted(lowerIdentity) {
		return false
	}

	// Split into syllables
	syllables := strings.Split(lowerIdentity, "-")
	if len(syllables) < 3 || len(syllables) > 4 {
		return false
	}

	// Validate each syllable follows CV or CVC pattern
	for _, syllable := range syllables {
		if !isValidCVSyllable(syllable) {
			return false
		}
	}

	return true
}

// isValidCVSyllable checks if a syllable follows CV or CVC pattern
func isValidCVSyllable(syllable string) bool {
	if len(syllable) < 2 || len(syllable) > 3 {
		return false
	}

	// First character should be consonant
	if !contains(consonants, string(syllable[0])) {
		return false
	}

	// Second character should be vowel
	if !contains(vowels, string(syllable[1])) {
		return false
	}

	// If three characters, third should be consonant
	if len(syllable) == 3 {
		if !contains(consonants, string(syllable[2])) {
			return false
		}
	}

	return true
}

// contains checks if a slice contains a string
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// GetIdentityParts returns the syllable components of an identity
func GetIdentityParts(identity string) []string {
	return strings.Split(identity, "-")
}
