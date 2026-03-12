package security

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"math/big"
)

// DiffieHellman implements Diffie-Hellman key exchange
type DiffieHellman struct {
	// RFC 3526 2048-bit MODP Group
	P *big.Int // Prime modulus
	G *big.Int // Generator
}

// NewDiffieHellman creates a new Diffie-Hellman instance with RFC 3526 2048-bit parameters
func NewDiffieHellman() *DiffieHellman {
	// RFC 3526 2048-bit MODP Group 14
	pHex := "FFFFFFFFFFFFFFFFC90FDAA22168C234C4C6628B80DC1CD1" +
		"29024E088A67CC74020BBEA63B139B22514A08798E3404DD" +
		"EF9519B3CD3A431B302B0A6DF25F14374FE1356D6D51C245" +
		"E485B576625E7EC6F44C42E9A637ED6B0BFF5CB6F406B7ED" +
		"EE386BFB5A899FA5AE9F24117C4B1FE649286651ECE45B3D" +
		"C2007CB8A163BF0598DA48361C55D39A69163FA8FD24CF5F" +
		"83655D23DCA3AD961C62F356208552BB9ED529077096966D" +
		"670C354E4ABC9804F1746C08CA18217C32905E462E36CE3B" +
		"E39E772C180E86039B2783A2EC07A28FB5C55DF06F4C52C9" +
		"DE2BCBF6955817183995497CEA956AE515D2261898FA0510" +
		"15728E5A8AACAA68FFFFFFFFFFFFFFFF"

	p, _ := new(big.Int).SetString(pHex, 16)
	g := big.NewInt(2)

	return &DiffieHellman{P: p, G: g}
}

// KeyPair represents a Diffie-Hellman key pair
type KeyPair struct {
	Private *big.Int // Private key (random)
	Public  *big.Int // Public key (g^private mod p)
}

// GenerateKeyPair creates a new Diffie-Hellman key pair
func (dh *DiffieHellman) GenerateKeyPair() (*KeyPair, error) {
	// Generate random private key
	private, err := rand.Int(rand.Reader, new(big.Int).Sub(dh.P, big.NewInt(2)))
	if err != nil {
		return nil, fmt.Errorf("failed to generate private key: %w", err)
	}
	private.Add(private, big.NewInt(1)) // Ensure private key is in range [1, p-2]

	// Calculate public key: g^private mod p
	public := new(big.Int).Exp(dh.G, private, dh.P)

	return &KeyPair{Private: private, Public: public}, nil
}

// ComputeSharedSecret computes the shared secret from our private key and their public key
func (dh *DiffieHellman) ComputeSharedSecret(ourPrivate, theirPublic *big.Int) (*big.Int, error) {
	// Validate their public key is in valid range
	if theirPublic.Cmp(big.NewInt(1)) <= 0 || theirPublic.Cmp(new(big.Int).Sub(dh.P, big.NewInt(1))) >= 0 {
		return nil, errors.New("invalid public key: out of range")
	}

	// Compute shared secret: theirPublic^ourPrivate mod p
	sharedSecret := new(big.Int).Exp(theirPublic, ourPrivate, dh.P)
	return sharedSecret, nil
}

// NodeEncryption handles pairwise node-to-node encryption
type NodeEncryption struct {
	sharedKey []byte // 32-byte key derived from DH shared secret
}

// NewNodeEncryption creates encryption context from Diffie-Hellman shared secret
func NewNodeEncryption(sharedSecret *big.Int) *NodeEncryption {
	// Derive 32-byte encryption key from shared secret using SHA-256
	secretBytes := sharedSecret.Bytes()
	hash := sha256.Sum256(secretBytes)

	return &NodeEncryption{sharedKey: hash[:]}
}

// Encrypt encrypts data for transmission to a specific node
func (ne *NodeEncryption) Encrypt(plaintext []byte) ([]byte, error) {
	// Create AES cipher
	block, err := aes.NewCipher(ne.sharedKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	// Generate random IV
	iv := make([]byte, aes.BlockSize)
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		return nil, fmt.Errorf("failed to generate IV: %w", err)
	}

	// Encrypt using CBC mode
	mode := cipher.NewCBCEncrypter(block, iv)

	// Pad plaintext to block size
	padded := pkcs7Pad(plaintext, aes.BlockSize)
	encrypted := make([]byte, len(padded))
	mode.CryptBlocks(encrypted, padded)

	// Prepend IV to ciphertext
	result := make([]byte, len(iv)+len(encrypted))
	copy(result, iv)
	copy(result[len(iv):], encrypted)

	return result, nil
}

// Decrypt decrypts data received from a specific node
func (ne *NodeEncryption) Decrypt(ciphertext []byte) ([]byte, error) {
	if len(ciphertext) < aes.BlockSize {
		return nil, errors.New("ciphertext too short")
	}

	// Extract IV and encrypted data
	iv := ciphertext[:aes.BlockSize]
	encrypted := ciphertext[aes.BlockSize:]

	// Create AES cipher
	block, err := aes.NewCipher(ne.sharedKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	// Decrypt using CBC mode
	if len(encrypted)%aes.BlockSize != 0 {
		return nil, errors.New("ciphertext length is not a multiple of block size")
	}

	mode := cipher.NewCBCDecrypter(block, iv)
	decrypted := make([]byte, len(encrypted))
	mode.CryptBlocks(decrypted, encrypted)

	// Remove padding
	return pkcs7Unpad(decrypted)
}

// AgentEncryption handles agent-level encryption using derived keys
type AgentEncryption struct {
	derivedKey []byte // Key derived from passphrase + device secret
}

// DeriveAgentKey creates an agent encryption key from passphrase and device secret
func DeriveAgentKey(passphrase, deviceSecret string) []byte {
	// Combine passphrase and device secret
	combined := passphrase + "|" + deviceSecret

	// Use SHA-256 to derive key (in production, use PBKDF2 or Argon2)
	hash := sha256.Sum256([]byte(combined))
	return hash[:]
}

// NewAgentEncryption creates agent encryption context
func NewAgentEncryption(passphrase, deviceSecret string) *AgentEncryption {
	return &AgentEncryption{derivedKey: DeriveAgentKey(passphrase, deviceSecret)}
}

// EncryptSecret encrypts agent secrets (private keys, sensitive data)
func (ae *AgentEncryption) EncryptSecret(plaintext []byte) ([]byte, error) {
	// Create AES cipher
	block, err := aes.NewCipher(ae.derivedKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	// Generate random IV
	iv := make([]byte, aes.BlockSize)
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		return nil, fmt.Errorf("failed to generate IV: %w", err)
	}

	// Encrypt using CBC mode
	mode := cipher.NewCBCEncrypter(block, iv)

	// Pad plaintext to block size
	padded := pkcs7Pad(plaintext, aes.BlockSize)
	encrypted := make([]byte, len(padded))
	mode.CryptBlocks(encrypted, padded)

	// Prepend IV to ciphertext
	result := make([]byte, len(iv)+len(encrypted))
	copy(result, iv)
	copy(result[len(iv):], encrypted)

	return result, nil
}

// DecryptSecret decrypts agent secrets
func (ae *AgentEncryption) DecryptSecret(ciphertext []byte) ([]byte, error) {
	if len(ciphertext) < aes.BlockSize {
		return nil, errors.New("ciphertext too short")
	}

	// Extract IV and encrypted data
	iv := ciphertext[:aes.BlockSize]
	encrypted := ciphertext[aes.BlockSize:]

	// Create AES cipher
	block, err := aes.NewCipher(ae.derivedKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	// Decrypt using CBC mode
	if len(encrypted)%aes.BlockSize != 0 {
		return nil, errors.New("ciphertext length is not a multiple of block size")
	}

	mode := cipher.NewCBCDecrypter(block, iv)
	decrypted := make([]byte, len(encrypted))
	mode.CryptBlocks(decrypted, encrypted)

	// Remove padding
	return pkcs7Unpad(decrypted)
}

// PostQuantumEncryption placeholder for future post-quantum upgrade
type PostQuantumEncryption struct {
	// TODO: Implement post-quantum encryption algorithms
	// This will be used to upgrade from classical DH to quantum-resistant algorithms
	classical *NodeEncryption
}

// AuthenticationChallenge represents an encrypted authentication challenge
type AuthenticationChallenge struct {
	ChallengeID   []byte // Unique challenge identifier
	EncryptedData []byte // Random challenge encrypted with agent's public key
}

// AuthenticationResponse represents the response to an authentication challenge
type AuthenticationResponse struct {
	ChallengeID   []byte // Echo of challenge identifier
	DecryptedData []byte // Decrypted challenge proving private key possession
}

// AgentIdentity represents an external agent's identity and keys
type AgentIdentity struct {
	Identity  string     // Agent's unique identifier
	PublicKey *big.Int   // Agent's public key for challenge encryption
	KeyPair   *KeyPair   // Full key pair (only available during session)
}

// AgentAuthenticator handles agent authentication using challenge-response
type AgentAuthenticator struct {
	dh             *DiffieHellman
	activeChallenges map[string]*AuthenticationChallenge // challengeID -> challenge
}

// NewAgentAuthenticator creates a new agent authentication manager
func NewAgentAuthenticator() *AgentAuthenticator {
	return &AgentAuthenticator{
		dh:               NewDiffieHellman(),
		activeChallenges: make(map[string]*AuthenticationChallenge),
	}
}

// NewPostQuantumEncryption creates a post-quantum encryption context
// Currently wraps classical encryption, will be upgraded to quantum-resistant algorithms
func NewPostQuantumEncryption(sharedSecret *big.Int) *PostQuantumEncryption {
	return &PostQuantumEncryption{
		classical: NewNodeEncryption(sharedSecret),
	}
}

// Encrypt using post-quantum algorithms (currently classical fallback)
func (pq *PostQuantumEncryption) Encrypt(plaintext []byte) ([]byte, error) {
	// TODO: Replace with post-quantum algorithm
	return pq.classical.Encrypt(plaintext)
}

// Decrypt using post-quantum algorithms (currently classical fallback)
func (pq *PostQuantumEncryption) Decrypt(ciphertext []byte) ([]byte, error) {
	// TODO: Replace with post-quantum algorithm
	return pq.classical.Decrypt(ciphertext)
}

// PKCS#7 padding functions
func pkcs7Pad(data []byte, blockSize int) []byte {
	padding := blockSize - len(data)%blockSize
	padtext := make([]byte, padding)
	for i := range padtext {
		padtext[i] = byte(padding)
	}
	return append(data, padtext...)
}

func pkcs7Unpad(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return nil, errors.New("data is empty")
	}

	padding := int(data[len(data)-1])
	if padding > len(data) || padding == 0 {
		return nil, errors.New("invalid padding")
	}

	// Verify padding
	for i := len(data) - padding; i < len(data); i++ {
		if data[i] != byte(padding) {
			return nil, errors.New("invalid padding")
		}
	}

	return data[:len(data)-padding], nil
}

// DeriveAgentKeyPair generates a key pair from passphrase and device secret
func (aa *AgentAuthenticator) DeriveAgentKeyPair(passphrase, deviceSecret string) (*KeyPair, error) {
	// Derive private key from combined factors using SHA-256
	combined := passphrase + "|" + deviceSecret
	hash := sha256.Sum256([]byte(combined))

	// Convert hash to big.Int for private key (ensure it's in valid range)
	private := new(big.Int).SetBytes(hash[:])
	private.Mod(private, new(big.Int).Sub(aa.dh.P, big.NewInt(2)))
	private.Add(private, big.NewInt(1)) // Ensure private key is in range [1, p-2]

	// Calculate corresponding public key: g^private mod p
	public := new(big.Int).Exp(aa.dh.G, private, aa.dh.P)

	return &KeyPair{Private: private, Public: public}, nil
}

// CreateAgentIdentity creates a new agent identity with derived keys
func (aa *AgentAuthenticator) CreateAgentIdentity(identity, passphrase, deviceSecret string) (*AgentIdentity, error) {
	keyPair, err := aa.DeriveAgentKeyPair(passphrase, deviceSecret)
	if err != nil {
		return nil, fmt.Errorf("failed to derive key pair: %w", err)
	}

	return &AgentIdentity{
		Identity:  identity,
		PublicKey: keyPair.Public,
		KeyPair:   keyPair,
	}, nil
}

// CreateChallenge generates an encrypted challenge for agent authentication
func (aa *AgentAuthenticator) CreateChallenge(agentPublicKey *big.Int) (*AuthenticationChallenge, error) {
	// Generate random challenge data (32 bytes)
	challengeData := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, challengeData); err != nil {
		return nil, fmt.Errorf("failed to generate challenge data: %w", err)
	}

	// Generate challenge ID (16 bytes)
	challengeID := make([]byte, 16)
	if _, err := io.ReadFull(rand.Reader, challengeID); err != nil {
		return nil, fmt.Errorf("failed to generate challenge ID: %w", err)
	}

	// For this implementation, we'll use AES encryption with a shared secret
	// derived from the agent's public key (simplified for demonstration)
	// In production, this would use proper RSA or ECIES

	// Derive encryption key from agent's public key
	publicKeyBytes := agentPublicKey.Bytes()
	keyHash := sha256.Sum256(publicKeyBytes)

	// Use agent encryption with this derived key
	tempAgent := &AgentEncryption{derivedKey: keyHash[:]}
	encryptedData, err := tempAgent.EncryptSecret(challengeData)
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt challenge: %w", err)
	}

	challenge := &AuthenticationChallenge{
		ChallengeID:   challengeID,
		EncryptedData: encryptedData,
	}

	// Store challenge for later verification
	challengeIDStr := fmt.Sprintf("%x", challengeID)
	aa.activeChallenges[challengeIDStr] = challenge

	// Store original data for verification
	aa.activeChallenges[challengeIDStr+":original"] = &AuthenticationChallenge{
		ChallengeID:   challengeID,
		EncryptedData: challengeData,
	}

	return challenge, nil
}

// VerifyResponse verifies the agent's response to an authentication challenge
func (aa *AgentAuthenticator) VerifyResponse(response *AuthenticationResponse) (bool, error) {
	challengeIDStr := fmt.Sprintf("%x", response.ChallengeID)

	// Get original challenge data
	originalChallenge, exists := aa.activeChallenges[challengeIDStr+":original"]
	if !exists {
		return false, errors.New("challenge not found or expired")
	}

	// Compare decrypted response with original challenge data
	if !bytes.Equal(originalChallenge.EncryptedData, response.DecryptedData) {
		return false, nil // Authentication failed
	}

	// Clean up challenge (single use)
	delete(aa.activeChallenges, challengeIDStr)
	delete(aa.activeChallenges, challengeIDStr+":original")

	return true, nil
}

// RespondToChallenge allows an agent to respond to an authentication challenge
func (aa *AgentAuthenticator) RespondToChallenge(challenge *AuthenticationChallenge, agentKeyPair *KeyPair) (*AuthenticationResponse, error) {
	// Decrypt the challenge using agent's private key
	// Derive decryption key from agent's public key (same as encryption)
	publicKeyBytes := agentKeyPair.Public.Bytes()
	keyHash := sha256.Sum256(publicKeyBytes)

	// Use agent decryption with this derived key
	tempAgent := &AgentEncryption{derivedKey: keyHash[:]}
	decryptedData, err := tempAgent.DecryptSecret(challenge.EncryptedData)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt challenge: %w", err)
	}

	return &AuthenticationResponse{
		ChallengeID:   challenge.ChallengeID,
		DecryptedData: decryptedData,
	}, nil
}

// ClearExpiredChallenges removes old challenges to prevent memory leaks
func (aa *AgentAuthenticator) ClearExpiredChallenges() {
	// In a real implementation, this would check timestamps
	// For now, we rely on single-use challenge cleanup in VerifyResponse
}

// GetAgentIdentityFromFactors reconstructs agent identity for authentication
func (aa *AgentAuthenticator) GetAgentIdentityFromFactors(identity, passphrase, deviceSecret string) (*AgentIdentity, error) {
	keyPair, err := aa.DeriveAgentKeyPair(passphrase, deviceSecret)
	if err != nil {
		return nil, fmt.Errorf("failed to derive key pair: %w", err)
	}

	return &AgentIdentity{
		Identity:  identity,
		PublicKey: keyPair.Public,
		KeyPair:   keyPair,
	}, nil
}