package security

import (
	"encoding/binary"
	"fmt"
	"math/big"
)

// Protocol message types for authentication (from design document)
const (
	MsgKeyExchange    = 0x01 // Diffie-Hellman parameters, post-quantum upgrade
	MsgIdentityReq    = 0x02 // New agent requests an identity
	MsgIdentityGrant  = 0x03 // Identity and keys issued to new agent
	MsgAuthChallenge  = 0x04 // Encrypted challenge for authentication
	MsgAuthResponse   = 0x05 // Decrypted challenge proving identity
	MsgPeerExchange   = 0x06 // Share list of known nodes
)

// AuthChallengeMessage represents AUTH_CHALLENGE (0x04) wire protocol message
type AuthChallengeMessage struct {
	AgentIdentity string   // Agent's unique identifier
	ChallengeID   []byte   // Unique challenge identifier (16 bytes)
	EncryptedData []byte   // Challenge encrypted with agent's public key
}

// AuthResponseMessage represents AUTH_RESPONSE (0x05) wire protocol message
type AuthResponseMessage struct {
	AgentIdentity string   // Agent's unique identifier
	ChallengeID   []byte   // Echo of challenge identifier
	DecryptedData []byte   // Decrypted challenge proving private key possession
}

// IdentityRequestMessage represents IDENTITY_REQUEST (0x02) wire protocol message
type IdentityRequestMessage struct {
	RequestedIdentity string // Requested agent identity (may be empty for auto-generation)
}

// IdentityGrantMessage represents IDENTITY_GRANT (0x03) wire protocol message
type IdentityGrantMessage struct {
	GrantedIdentity string   // Granted agent identity
	PublicKey       *big.Int // Agent's public key for future authentication
}

// KeyExchangeMessage represents KEY_EXCHANGE (0x01) wire protocol message
type KeyExchangeMessage struct {
	PublicKey       *big.Int // Diffie-Hellman public key
	PostQuantumData []byte   // Post-quantum upgrade parameters (future use)
}

// SerializeAuthChallenge serializes an AUTH_CHALLENGE message for transmission
func SerializeAuthChallenge(msg *AuthChallengeMessage) []byte {
	// Calculate total length
	identityLen := len(msg.AgentIdentity)
	challengeIDLen := len(msg.ChallengeID)
	encryptedDataLen := len(msg.EncryptedData)

	// Total: 4 + identity + 4 + challengeID + 4 + encryptedData
	totalLen := 4 + identityLen + 4 + challengeIDLen + 4 + encryptedDataLen
	data := make([]byte, totalLen)

	offset := 0

	// Identity length and data
	binary.BigEndian.PutUint32(data[offset:], uint32(identityLen))
	offset += 4
	copy(data[offset:], []byte(msg.AgentIdentity))
	offset += identityLen

	// Challenge ID length and data
	binary.BigEndian.PutUint32(data[offset:], uint32(challengeIDLen))
	offset += 4
	copy(data[offset:], msg.ChallengeID)
	offset += challengeIDLen

	// Encrypted data length and data
	binary.BigEndian.PutUint32(data[offset:], uint32(encryptedDataLen))
	offset += 4
	copy(data[offset:], msg.EncryptedData)

	return data
}

// DeserializeAuthChallenge deserializes an AUTH_CHALLENGE message from wire format
func DeserializeAuthChallenge(data []byte) (*AuthChallengeMessage, error) {
	if len(data) < 12 {
		return nil, fmt.Errorf("auth challenge message too short")
	}

	offset := 0

	// Read identity
	identityLen := binary.BigEndian.Uint32(data[offset:])
	offset += 4
	if offset+int(identityLen) > len(data) {
		return nil, fmt.Errorf("invalid identity length")
	}
	identity := string(data[offset : offset+int(identityLen)])
	offset += int(identityLen)

	// Read challenge ID
	challengeIDLen := binary.BigEndian.Uint32(data[offset:])
	offset += 4
	if offset+int(challengeIDLen) > len(data) {
		return nil, fmt.Errorf("invalid challenge ID length")
	}
	challengeID := make([]byte, challengeIDLen)
	copy(challengeID, data[offset:offset+int(challengeIDLen)])
	offset += int(challengeIDLen)

	// Read encrypted data
	encryptedDataLen := binary.BigEndian.Uint32(data[offset:])
	offset += 4
	if offset+int(encryptedDataLen) != len(data) {
		return nil, fmt.Errorf("invalid encrypted data length")
	}
	encryptedData := make([]byte, encryptedDataLen)
	copy(encryptedData, data[offset:])

	return &AuthChallengeMessage{
		AgentIdentity: identity,
		ChallengeID:   challengeID,
		EncryptedData: encryptedData,
	}, nil
}

// SerializeAuthResponse serializes an AUTH_RESPONSE message for transmission
func SerializeAuthResponse(msg *AuthResponseMessage) []byte {
	// Calculate total length
	identityLen := len(msg.AgentIdentity)
	challengeIDLen := len(msg.ChallengeID)
	decryptedDataLen := len(msg.DecryptedData)

	// Total: 4 + identity + 4 + challengeID + 4 + decryptedData
	totalLen := 4 + identityLen + 4 + challengeIDLen + 4 + decryptedDataLen
	data := make([]byte, totalLen)

	offset := 0

	// Identity length and data
	binary.BigEndian.PutUint32(data[offset:], uint32(identityLen))
	offset += 4
	copy(data[offset:], []byte(msg.AgentIdentity))
	offset += identityLen

	// Challenge ID length and data
	binary.BigEndian.PutUint32(data[offset:], uint32(challengeIDLen))
	offset += 4
	copy(data[offset:], msg.ChallengeID)
	offset += challengeIDLen

	// Decrypted data length and data
	binary.BigEndian.PutUint32(data[offset:], uint32(decryptedDataLen))
	offset += 4
	copy(data[offset:], msg.DecryptedData)

	return data
}

// DeserializeAuthResponse deserializes an AUTH_RESPONSE message from wire format
func DeserializeAuthResponse(data []byte) (*AuthResponseMessage, error) {
	if len(data) < 12 {
		return nil, fmt.Errorf("auth response message too short")
	}

	offset := 0

	// Read identity
	identityLen := binary.BigEndian.Uint32(data[offset:])
	offset += 4
	if offset+int(identityLen) > len(data) {
		return nil, fmt.Errorf("invalid identity length")
	}
	identity := string(data[offset : offset+int(identityLen)])
	offset += int(identityLen)

	// Read challenge ID
	challengeIDLen := binary.BigEndian.Uint32(data[offset:])
	offset += 4
	if offset+int(challengeIDLen) > len(data) {
		return nil, fmt.Errorf("invalid challenge ID length")
	}
	challengeID := make([]byte, challengeIDLen)
	copy(challengeID, data[offset:offset+int(challengeIDLen)])
	offset += int(challengeIDLen)

	// Read decrypted data
	decryptedDataLen := binary.BigEndian.Uint32(data[offset:])
	offset += 4
	if offset+int(decryptedDataLen) != len(data) {
		return nil, fmt.Errorf("invalid decrypted data length")
	}
	decryptedData := make([]byte, decryptedDataLen)
	copy(decryptedData, data[offset:])

	return &AuthResponseMessage{
		AgentIdentity: identity,
		ChallengeID:   challengeID,
		DecryptedData: decryptedData,
	}, nil
}

// AuthenticationSession represents an active authentication session
type AuthenticationSession struct {
	AgentIdentity string                   // Agent being authenticated
	Challenge     *AuthenticationChallenge // Active challenge
	Authenticator *AgentAuthenticator      // Authenticator instance
	PublicKey     *big.Int                 // Agent's public key from mesh
}

// NewAuthenticationSession creates a new authentication session for an agent
func NewAuthenticationSession(identity string, publicKey *big.Int) *AuthenticationSession {
	return &AuthenticationSession{
		AgentIdentity: identity,
		Authenticator: NewAgentAuthenticator(),
		PublicKey:     publicKey,
	}
}

// CreateChallenge creates an authentication challenge for this session
func (session *AuthenticationSession) CreateChallenge() (*AuthChallengeMessage, error) {
	challenge, err := session.Authenticator.CreateChallenge(session.PublicKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create challenge: %w", err)
	}

	session.Challenge = challenge

	return &AuthChallengeMessage{
		AgentIdentity: session.AgentIdentity,
		ChallengeID:   challenge.ChallengeID,
		EncryptedData: challenge.EncryptedData,
	}, nil
}

// VerifyResponse verifies an authentication response for this session
func (session *AuthenticationSession) VerifyResponse(response *AuthResponseMessage) (bool, error) {
	if response.AgentIdentity != session.AgentIdentity {
		return false, fmt.Errorf("identity mismatch: expected %s, got %s",
			session.AgentIdentity, response.AgentIdentity)
	}

	authResponse := &AuthenticationResponse{
		ChallengeID:   response.ChallengeID,
		DecryptedData: response.DecryptedData,
	}

	return session.Authenticator.VerifyResponse(authResponse)
}
