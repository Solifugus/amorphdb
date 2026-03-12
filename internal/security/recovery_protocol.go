package security

import "crypto/sha256"
import (
	"encoding/binary"
	"fmt"
)

// Protocol message type for recovery (from design document)
const (
	MsgRecoveryShard = 0x30 // Submit a Shamir recovery shard
)

// RecoveryShardMessage represents RECOVERY_SHARD (0x30) wire protocol message
type RecoveryShardMessage struct {
	RecoveryAgentIdentity string // Agent submitting the shard
	TargetAgentIdentity   string // Agent being recovered
	ShareID               int    // Unique ID for this share (1-based)
	ShareValue            []byte // The shard value
	Threshold             int    // Minimum shares needed for reconstruction
	TotalShares           int    // Total number of shares created
	Signature             []byte // Signature proving submission authenticity
}

// RecoveryRequestMessage represents a request to start recovery process
type RecoveryRequestMessage struct {
	AgentIdentity      string // Agent requesting recovery
	RequestorPublicKey []byte // Temporary public key for communication
}

// RecoveryStatusMessage represents the current status of a recovery attempt
type RecoveryStatusMessage struct {
	AgentIdentity        string   // Agent being recovered
	TrustedAgents        []string // List of trusted recovery agents
	Threshold            int      // Required number of shards
	SubmittedShards      int      // Number of shards submitted so far
	RecoveryInProgress   bool     // Whether recovery is active
	ShardsFromAgents     []string // Agents who have submitted shards
}

// SerializeRecoveryShardMessage serializes a RECOVERY_SHARD message for transmission
func SerializeRecoveryShardMessage(msg *RecoveryShardMessage) []byte {
	// Calculate lengths
	recoveryAgentLen := len(msg.RecoveryAgentIdentity)
	targetAgentLen := len(msg.TargetAgentIdentity)
	shareValueLen := len(msg.ShareValue)
	signatureLen := len(msg.Signature)

	// Total: 4 + recoveryAgent + 4 + targetAgent + 4 + shareID + 4 + shareValue + 4 + threshold + 4 + totalShares + 4 + signature
	totalLen := 4 + recoveryAgentLen + 4 + targetAgentLen + 4 + 4 + shareValueLen + 4 + 4 + 4 + signatureLen
	data := make([]byte, totalLen)

	offset := 0

	// Recovery agent identity length and data
	binary.BigEndian.PutUint32(data[offset:], uint32(recoveryAgentLen))
	offset += 4
	copy(data[offset:], []byte(msg.RecoveryAgentIdentity))
	offset += recoveryAgentLen

	// Target agent identity length and data
	binary.BigEndian.PutUint32(data[offset:], uint32(targetAgentLen))
	offset += 4
	copy(data[offset:], []byte(msg.TargetAgentIdentity))
	offset += targetAgentLen

	// Share ID
	binary.BigEndian.PutUint32(data[offset:], uint32(msg.ShareID))
	offset += 4

	// Share value length and data
	binary.BigEndian.PutUint32(data[offset:], uint32(shareValueLen))
	offset += 4
	copy(data[offset:], msg.ShareValue)
	offset += shareValueLen

	// Threshold
	binary.BigEndian.PutUint32(data[offset:], uint32(msg.Threshold))
	offset += 4

	// Total shares
	binary.BigEndian.PutUint32(data[offset:], uint32(msg.TotalShares))
	offset += 4

	// Signature length and data
	binary.BigEndian.PutUint32(data[offset:], uint32(signatureLen))
	offset += 4
	copy(data[offset:], msg.Signature)

	return data
}

// DeserializeRecoveryShardMessage deserializes a RECOVERY_SHARD message from wire format
func DeserializeRecoveryShardMessage(data []byte) (*RecoveryShardMessage, error) {
	if len(data) < 28 { // Minimum: 7 * 4 bytes for lengths/values
		return nil, fmt.Errorf("recovery shard message too short")
	}

	offset := 0

	// Read recovery agent identity
	recoveryAgentLen := binary.BigEndian.Uint32(data[offset:])
	offset += 4
	if offset+int(recoveryAgentLen) > len(data) {
		return nil, fmt.Errorf("invalid recovery agent identity length")
	}
	recoveryAgent := string(data[offset : offset+int(recoveryAgentLen)])
	offset += int(recoveryAgentLen)

	// Read target agent identity
	targetAgentLen := binary.BigEndian.Uint32(data[offset:])
	offset += 4
	if offset+int(targetAgentLen) > len(data) {
		return nil, fmt.Errorf("invalid target agent identity length")
	}
	targetAgent := string(data[offset : offset+int(targetAgentLen)])
	offset += int(targetAgentLen)

	// Read share ID
	if offset+4 > len(data) {
		return nil, fmt.Errorf("insufficient data for share ID")
	}
	shareID := int(binary.BigEndian.Uint32(data[offset:]))
	offset += 4

	// Read share value
	if offset+4 > len(data) {
		return nil, fmt.Errorf("insufficient data for share value length")
	}
	shareValueLen := binary.BigEndian.Uint32(data[offset:])
	offset += 4
	if offset+int(shareValueLen) > len(data) {
		return nil, fmt.Errorf("invalid share value length")
	}
	shareValue := make([]byte, shareValueLen)
	copy(shareValue, data[offset:offset+int(shareValueLen)])
	offset += int(shareValueLen)

	// Read threshold
	if offset+4 > len(data) {
		return nil, fmt.Errorf("insufficient data for threshold")
	}
	threshold := int(binary.BigEndian.Uint32(data[offset:]))
	offset += 4

	// Read total shares
	if offset+4 > len(data) {
		return nil, fmt.Errorf("insufficient data for total shares")
	}
	totalShares := int(binary.BigEndian.Uint32(data[offset:]))
	offset += 4

	// Read signature
	if offset+4 > len(data) {
		return nil, fmt.Errorf("insufficient data for signature length")
	}
	signatureLen := binary.BigEndian.Uint32(data[offset:])
	offset += 4
	if offset+int(signatureLen) != len(data) {
		return nil, fmt.Errorf("invalid signature length")
	}
	signature := make([]byte, signatureLen)
	copy(signature, data[offset:])

	return &RecoveryShardMessage{
		RecoveryAgentIdentity: recoveryAgent,
		TargetAgentIdentity:   targetAgent,
		ShareID:               shareID,
		ShareValue:            shareValue,
		Threshold:             threshold,
		TotalShares:           totalShares,
		Signature:             signature,
	}, nil
}

// SerializeRecoveryRequestMessage serializes a recovery request for transmission
func SerializeRecoveryRequestMessage(msg *RecoveryRequestMessage) []byte {
	identityLen := len(msg.AgentIdentity)
	publicKeyLen := len(msg.RequestorPublicKey)

	// Total: 4 + identity + 4 + publicKey
	totalLen := 4 + identityLen + 4 + publicKeyLen
	data := make([]byte, totalLen)

	offset := 0

	// Agent identity length and data
	binary.BigEndian.PutUint32(data[offset:], uint32(identityLen))
	offset += 4
	copy(data[offset:], []byte(msg.AgentIdentity))
	offset += identityLen

	// Public key length and data
	binary.BigEndian.PutUint32(data[offset:], uint32(publicKeyLen))
	offset += 4
	copy(data[offset:], msg.RequestorPublicKey)

	return data
}

// DeserializeRecoveryRequestMessage deserializes a recovery request from wire format
func DeserializeRecoveryRequestMessage(data []byte) (*RecoveryRequestMessage, error) {
	if len(data) < 8 {
		return nil, fmt.Errorf("recovery request message too short")
	}

	offset := 0

	// Read agent identity
	identityLen := binary.BigEndian.Uint32(data[offset:])
	offset += 4
	if offset+int(identityLen) > len(data) {
		return nil, fmt.Errorf("invalid identity length")
	}
	identity := string(data[offset : offset+int(identityLen)])
	offset += int(identityLen)

	// Read public key
	publicKeyLen := binary.BigEndian.Uint32(data[offset:])
	offset += 4
	if offset+int(publicKeyLen) != len(data) {
		return nil, fmt.Errorf("invalid public key length")
	}
	publicKey := make([]byte, publicKeyLen)
	copy(publicKey, data[offset:])

	return &RecoveryRequestMessage{
		AgentIdentity:      identity,
		RequestorPublicKey: publicKey,
	}, nil
}

// RecoveryProtocolHandler handles recovery-related protocol operations
type RecoveryProtocolHandler struct {
	recoveryManager *RecoveryManager
	authenticator   *AgentAuthenticator
}

// NewRecoveryProtocolHandler creates a new recovery protocol handler
func NewRecoveryProtocolHandler() *RecoveryProtocolHandler {
	return &RecoveryProtocolHandler{
		recoveryManager: NewRecoveryManager(),
		authenticator:   NewAgentAuthenticator(),
	}
}

// HandleRecoveryRequest processes a recovery request and starts recovery session
func (rph *RecoveryProtocolHandler) HandleRecoveryRequest(msg *RecoveryRequestMessage) (*RecoveryStatusMessage, error) {
	_, err := rph.recoveryManager.StartRecovery(msg.AgentIdentity, msg.RequestorPublicKey)
	// Start recovery session
	if err != nil {
		return nil, fmt.Errorf("failed to start recovery: %w", err)
	}

	// Get trusted agents and threshold
	trustedAgents, threshold, err := rph.recoveryManager.ListTrustedAgents(msg.AgentIdentity)
	if err != nil {
		return nil, fmt.Errorf("failed to get trusted agents: %w", err)
	}

	return &RecoveryStatusMessage{
		AgentIdentity:      msg.AgentIdentity,
		TrustedAgents:      trustedAgents,
		Threshold:          threshold,
		SubmittedShards:    0,
		RecoveryInProgress: true,
		ShardsFromAgents:   []string{},
	}, nil
}

// HandleRecoveryShardSubmission processes a recovery shard submission
func (rph *RecoveryProtocolHandler) HandleRecoveryShardSubmission(msg *RecoveryShardMessage) (*RecoveryStatusMessage, error) {
	// Convert protocol message to recovery shard
	shard := &RecoveryShard{
		ShareID:       msg.ShareID,
		ShareValue:    msg.ShareValue,
		Threshold:     msg.Threshold,
		TotalShares:   msg.TotalShares,
		AgentIdentity: msg.TargetAgentIdentity,
	}

	// Create encrypted shard data for submission
	// In production, this would be the actual encrypted shard from the recovery agent
	// For this implementation, we'll simulate by encrypting the share value
	agentKey := sha256.Sum256([]byte(msg.RecoveryAgentIdentity + ":recovery"))
	agentEnc := &AgentEncryption{derivedKey: agentKey[:]}
	encryptedShard, err := agentEnc.EncryptSecret(shard.ShareValue)
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt shard: %w", err)
	}

	// Submit the shard
	err = rph.recoveryManager.SubmitRecoveryShard(msg.RecoveryAgentIdentity, msg.TargetAgentIdentity, encryptedShard)
	if err != nil {
		return nil, fmt.Errorf("failed to submit shard: %w", err)
	}

	// Get current status
	session, exists := rph.recoveryManager.GetRecoveryStatus(msg.TargetAgentIdentity)
	if !exists {
		return nil, fmt.Errorf("recovery session not found")
	}

	// Build list of agents who have submitted shards
	shardsFromAgents := make([]string, 0, len(session.SubmittedShards))
	for agent := range session.SubmittedShards {
		shardsFromAgents = append(shardsFromAgents, agent)
	}

	return &RecoveryStatusMessage{
		AgentIdentity:      msg.TargetAgentIdentity,
		TrustedAgents:      session.Configuration.TrustedAgents,
		Threshold:          session.Configuration.Threshold,
		SubmittedShards:    len(session.SubmittedShards),
		RecoveryInProgress: true,
		ShardsFromAgents:   shardsFromAgents,
	}, nil
}

// AttemptRecoveryReconstruction tries to reconstruct the recovery key
func (rph *RecoveryProtocolHandler) AttemptRecoveryReconstruction(agentIdentity string) ([]byte, *RecoveryStatusMessage, error) {
	// Attempt to reconstruct the recovery key
	recoveryKey, err := rph.recoveryManager.AttemptRecovery(agentIdentity)
	if err != nil {
		// If reconstruction failed, return current status
		session, exists := rph.recoveryManager.GetRecoveryStatus(agentIdentity)
		if !exists {
			return nil, nil, fmt.Errorf("recovery session not found")
		}

		shardsFromAgents := make([]string, 0, len(session.SubmittedShards))
		for agent := range session.SubmittedShards {
			shardsFromAgents = append(shardsFromAgents, agent)
		}

		status := &RecoveryStatusMessage{
			AgentIdentity:      agentIdentity,
			TrustedAgents:      session.Configuration.TrustedAgents,
			Threshold:          session.Configuration.Threshold,
			SubmittedShards:    len(session.SubmittedShards),
			RecoveryInProgress: true,
			ShardsFromAgents:   shardsFromAgents,
		}

		return nil, status, err
	}

	// Recovery successful
	status := &RecoveryStatusMessage{
		AgentIdentity:      agentIdentity,
		RecoveryInProgress: false,
		SubmittedShards:    0, // Session cleaned up
		ShardsFromAgents:   []string{},
	}

	return recoveryKey, status, nil
}

// GetRecoveryStatus returns the current status of a recovery attempt
func (rph *RecoveryProtocolHandler) GetRecoveryStatus(agentIdentity string) (*RecoveryStatusMessage, error) {
	session, exists := rph.recoveryManager.GetRecoveryStatus(agentIdentity)
	if !exists {
		return &RecoveryStatusMessage{
			AgentIdentity:      agentIdentity,
			RecoveryInProgress: false,
		}, nil
	}

	shardsFromAgents := make([]string, 0, len(session.SubmittedShards))
	for agent := range session.SubmittedShards {
		shardsFromAgents = append(shardsFromAgents, agent)
	}

	return &RecoveryStatusMessage{
		AgentIdentity:      agentIdentity,
		TrustedAgents:      session.Configuration.TrustedAgents,
		Threshold:          session.Configuration.Threshold,
		SubmittedShards:    len(session.SubmittedShards),
		RecoveryInProgress: true,
		ShardsFromAgents:   shardsFromAgents,
	}, nil
}

// CreateRecoveryConfiguration creates recovery configuration for an agent
func (rph *RecoveryProtocolHandler) CreateRecoveryConfiguration(agentIdentity string, trustedAgents []string, threshold int, recoveryKey []byte) error {
	_, err := rph.recoveryManager.CreateRecoveryConfiguration(agentIdentity, trustedAgents, threshold, recoveryKey)
	return err
}

// CancelRecovery cancels an ongoing recovery attempt
func (rph *RecoveryProtocolHandler) CancelRecovery(agentIdentity string) error {
	return rph.recoveryManager.CancelRecovery(agentIdentity)
}
