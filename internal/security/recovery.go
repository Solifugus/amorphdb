package security

import (
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
	"math/big"
)

// RecoveryConfiguration represents the recovery settings for an agent
type RecoveryConfiguration struct {
	AgentIdentity   string   // Agent whose recovery this configures
	TrustedAgents   []string // List of trusted recovery agent identities
	Threshold       int      // Minimum number of shards needed for recovery
	EncryptedShards map[string][]byte // AgentIdentity -> encrypted shard
}

// RecoveryShard represents a single shard of a Shamir secret
type RecoveryShard struct {
	ShareID    int    // Unique ID for this share (1-based)
	ShareValue []byte // The shard value
	Threshold  int    // Minimum shares needed for reconstruction
	TotalShares int   // Total number of shares created
	AgentIdentity string // Agent this shard can help recover
}

// RecoveryRequest represents a request to recover an agent's account
type RecoveryRequest struct {
	AgentIdentity      string // Agent requesting recovery
	RequestorPublicKey []byte // Temporary public key for communication
}

// RecoveryShardSubmission represents submission of a recovery shard
type RecoveryShardSubmission struct {
	RecoveryAgentIdentity string        // Agent submitting the shard
	TargetAgentIdentity   string        // Agent being recovered
	Shard                 *RecoveryShard // The recovery shard
	Signature             []byte        // Signature proving submission authenticity
}

// ShamirSecretSharing implements Shamir's Secret Sharing scheme
type ShamirSecretSharing struct {
	prime *big.Int // Large prime for finite field arithmetic
}

// NewShamirSecretSharing creates a new Shamir secret sharing instance
func NewShamirSecretSharing() *ShamirSecretSharing {
	// Use a large prime for finite field operations (2^521 - 1, Mersenne prime)
	prime, _ := new(big.Int).SetString("6864797660130609714981900799081393217269435300143305409394463459185543183397656052122559640661454554977296311391480858037121987999716643812574028291115057151", 10)

	return &ShamirSecretSharing{
		prime: prime,
	}
}

// SplitSecret splits a secret into n shares requiring threshold shares for reconstruction
func (sss *ShamirSecretSharing) SplitSecret(secret []byte, threshold, totalShares int) ([]*RecoveryShard, error) {
	if threshold <= 0 || totalShares <= 0 || threshold > totalShares {
		return nil, fmt.Errorf("invalid threshold (%d) or total shares (%d)", threshold, totalShares)
	}

	if len(secret) == 0 {
		return nil, errors.New("secret cannot be empty")
	}

	// Convert secret to big integer
	secretInt := new(big.Int).SetBytes(secret)
	if secretInt.Cmp(sss.prime) >= 0 {
		return nil, errors.New("secret too large for field size")
	}

	// Generate random coefficients for polynomial of degree (threshold-1)
	coefficients := make([]*big.Int, threshold)
	coefficients[0] = secretInt // a0 = secret

	for i := 1; i < threshold; i++ {
		coeff, err := rand.Int(rand.Reader, sss.prime)
		if err != nil {
			return nil, fmt.Errorf("failed to generate coefficient: %w", err)
		}
		coefficients[i] = coeff
	}

	// Evaluate polynomial at points 1, 2, ..., totalShares
	shards := make([]*RecoveryShard, totalShares)
	for i := 1; i <= totalShares; i++ {
		x := big.NewInt(int64(i))
		y := sss.evaluatePolynomial(coefficients, x)

		shards[i-1] = &RecoveryShard{
			ShareID:     i,
			ShareValue:  y.Bytes(),
			Threshold:   threshold,
			TotalShares: totalShares,
		}
	}

	return shards, nil
}

// ReconstructSecret reconstructs the secret from threshold number of shards
func (sss *ShamirSecretSharing) ReconstructSecret(shards []*RecoveryShard) ([]byte, error) {
	if len(shards) == 0 {
		return nil, errors.New("no shards provided")
	}

	threshold := shards[0].Threshold
	if len(shards) < threshold {
		return nil, fmt.Errorf("insufficient shards: need %d, got %d", threshold, len(shards))
	}

	// Verify all shards have same threshold
	for i, shard := range shards {
		if shard.Threshold != threshold {
			return nil, fmt.Errorf("shard %d has different threshold: %d vs %d", i, shard.Threshold, threshold)
		}
	}

	// Use first 'threshold' shards for reconstruction
	useShards := shards[:threshold]

	// Convert shards to (x, y) points
	points := make([][]*big.Int, threshold)
	for i, shard := range useShards {
		x := big.NewInt(int64(shard.ShareID))
		y := new(big.Int).SetBytes(shard.ShareValue)
		points[i] = []*big.Int{x, y}
	}

	// Use Lagrange interpolation to find polynomial value at x=0
	secret := big.NewInt(0)

	for i := 0; i < threshold; i++ {
		xi := points[i][0]
		yi := points[i][1]

		// Calculate Lagrange basis polynomial l_i(0)
		numerator := big.NewInt(1)
		denominator := big.NewInt(1)

		for j := 0; j < threshold; j++ {
			if i != j {
				xj := points[j][0]

				// numerator *= (0 - xj) = -xj
				numerator.Mul(numerator, new(big.Int).Neg(xj))
				numerator.Mod(numerator, sss.prime)

				// denominator *= (xi - xj)
				diff := new(big.Int).Sub(xi, xj)
				denominator.Mul(denominator, diff)
				denominator.Mod(denominator, sss.prime)
			}
		}

		// Calculate modular inverse of denominator
		denomInv := new(big.Int).ModInverse(denominator, sss.prime)
		if denomInv == nil {
			return nil, fmt.Errorf("failed to calculate modular inverse for shard %d", i)
		}

		// li = numerator * denomInv
		li := new(big.Int).Mul(numerator, denomInv)
		li.Mod(li, sss.prime)

		// secret += yi * li
		term := new(big.Int).Mul(yi, li)
		secret.Add(secret, term)
		secret.Mod(secret, sss.prime)
	}

	return secret.Bytes(), nil
}

// evaluatePolynomial evaluates polynomial at given x
func (sss *ShamirSecretSharing) evaluatePolynomial(coefficients []*big.Int, x *big.Int) *big.Int {
	result := big.NewInt(0)
	xPower := big.NewInt(1)

	for _, coeff := range coefficients {
		// result += coeff * x^i
		term := new(big.Int).Mul(coeff, xPower)
		result.Add(result, term)
		result.Mod(result, sss.prime)

		// xPower *= x for next iteration
		xPower.Mul(xPower, x)
		xPower.Mod(xPower, sss.prime)
	}

	return result
}

// RecoveryManager handles the complete recovery process
type RecoveryManager struct {
	sss         *ShamirSecretSharing
	configs     map[string]*RecoveryConfiguration // AgentIdentity -> config
	activeRecoveries map[string]*RecoverySession  // AgentIdentity -> active recovery
}

// RecoverySession represents an ongoing recovery attempt
type RecoverySession struct {
	AgentIdentity     string                               // Agent being recovered
	RequestorPublicKey []byte                              // Temporary public key
	SubmittedShards   map[string]*RecoveryShardSubmission  // RecoveryAgent -> shard
	Configuration     *RecoveryConfiguration               // Recovery configuration
	Created           int64                                // Timestamp when recovery started
}

// NewRecoveryManager creates a new recovery manager
func NewRecoveryManager() *RecoveryManager {
	return &RecoveryManager{
		sss:              NewShamirSecretSharing(),
		configs:          make(map[string]*RecoveryConfiguration),
		activeRecoveries: make(map[string]*RecoverySession),
	}
}

// CreateRecoveryConfiguration sets up recovery for an agent
func (rm *RecoveryManager) CreateRecoveryConfiguration(agentIdentity string, trustedAgents []string, threshold int, recoveryKey []byte) (*RecoveryConfiguration, error) {
	if threshold <= 0 || len(trustedAgents) < threshold {
		return nil, fmt.Errorf("invalid configuration: threshold %d, trusted agents %d", threshold, len(trustedAgents))
	}

	// Split recovery key into shards
	shards, err := rm.sss.SplitSecret(recoveryKey, threshold, len(trustedAgents))
	if err != nil {
		return nil, fmt.Errorf("failed to split recovery key: %w", err)
	}

	// Create encrypted shards for each trusted agent
	encryptedShards := make(map[string][]byte)
	for i, agentID := range trustedAgents {
		// In production, encrypt shard with agent's public key
		// For this implementation, we'll use a simple hash-based encryption
		agentKey := sha256.Sum256([]byte(agentID + ":recovery"))

		agentEnc := &AgentEncryption{derivedKey: agentKey[:]}
		encryptedShard, err := agentEnc.EncryptSecret(shards[i].ShareValue)
		if err != nil {
			return nil, fmt.Errorf("failed to encrypt shard for agent %s: %w", agentID, err)
		}

		encryptedShards[agentID] = encryptedShard
		shards[i].AgentIdentity = agentIdentity
	}

	config := &RecoveryConfiguration{
		AgentIdentity:   agentIdentity,
		TrustedAgents:   trustedAgents,
		Threshold:       threshold,
		EncryptedShards: encryptedShards,
	}

	rm.configs[agentIdentity] = config
	return config, nil
}

// StartRecovery begins a recovery process for an agent
func (rm *RecoveryManager) StartRecovery(agentIdentity string, requestorPublicKey []byte) (*RecoverySession, error) {
	config, exists := rm.configs[agentIdentity]
	if !exists {
		return nil, fmt.Errorf("no recovery configuration for agent %s", agentIdentity)
	}

	// Check if recovery is already in progress
	if _, inProgress := rm.activeRecoveries[agentIdentity]; inProgress {
		return nil, fmt.Errorf("recovery already in progress for agent %s", agentIdentity)
	}

	session := &RecoverySession{
		AgentIdentity:      agentIdentity,
		RequestorPublicKey: requestorPublicKey,
		SubmittedShards:    make(map[string]*RecoveryShardSubmission),
		Configuration:      config,
		Created:            getCurrentTimestamp(),
	}

	rm.activeRecoveries[agentIdentity] = session
	return session, nil
}

// SubmitRecoveryShard allows a trusted agent to submit their recovery shard
func (rm *RecoveryManager) SubmitRecoveryShard(recoveryAgentIdentity, targetAgentIdentity string, encryptedShard []byte) error {
	session, exists := rm.activeRecoveries[targetAgentIdentity]
	if !exists {
		return fmt.Errorf("no active recovery for agent %s", targetAgentIdentity)
	}

	// Verify the submitting agent is trusted for this recovery
	trustedAgent := false
	for _, trusted := range session.Configuration.TrustedAgents {
		if trusted == recoveryAgentIdentity {
			trustedAgent = true
			break
		}
	}

	if !trustedAgent {
		return fmt.Errorf("agent %s is not trusted for recovery of %s", recoveryAgentIdentity, targetAgentIdentity)
	}

	// Decrypt the shard using agent's recovery key
	agentKey := sha256.Sum256([]byte(recoveryAgentIdentity + ":recovery"))
	agentEnc := &AgentEncryption{derivedKey: agentKey[:]}

	decryptedValue, err := agentEnc.DecryptSecret(encryptedShard)
	if err != nil {
		return fmt.Errorf("failed to decrypt shard from %s: %w", recoveryAgentIdentity, err)
	}

	// Create recovery shard
	// Find the share ID for this agent (based on position in trusted agents list)
	shareID := -1
	for i, agentID := range session.Configuration.TrustedAgents {
		if agentID == recoveryAgentIdentity {
			shareID = i + 1 // 1-based indexing
			break
		}
	}

	if shareID == -1 {
		return fmt.Errorf("could not determine share ID for agent %s", recoveryAgentIdentity)
	}

	shard := &RecoveryShard{
		ShareID:       shareID,
		ShareValue:    decryptedValue,
		Threshold:     session.Configuration.Threshold,
		TotalShares:   len(session.Configuration.TrustedAgents),
		AgentIdentity: targetAgentIdentity,
	}

	submission := &RecoveryShardSubmission{
		RecoveryAgentIdentity: recoveryAgentIdentity,
		TargetAgentIdentity:   targetAgentIdentity,
		Shard:                 shard,
		Signature:             []byte("signature-placeholder"), // In production, would be proper signature
	}

	session.SubmittedShards[recoveryAgentIdentity] = submission
	return nil
}

// AttemptRecovery tries to reconstruct the recovery key if enough shards are available
func (rm *RecoveryManager) AttemptRecovery(agentIdentity string) ([]byte, error) {
	session, exists := rm.activeRecoveries[agentIdentity]
	if !exists {
		return nil, fmt.Errorf("no active recovery for agent %s", agentIdentity)
	}

	// Check if we have enough shards
	if len(session.SubmittedShards) < session.Configuration.Threshold {
		return nil, fmt.Errorf("insufficient shards: need %d, have %d",
			session.Configuration.Threshold, len(session.SubmittedShards))
	}

	// Extract shards for reconstruction
	shards := make([]*RecoveryShard, 0, len(session.SubmittedShards))
	for _, submission := range session.SubmittedShards {
		shards = append(shards, submission.Shard)
	}

	// Reconstruct the recovery key
	recoveryKey, err := rm.sss.ReconstructSecret(shards)
	if err != nil {
		return nil, fmt.Errorf("failed to reconstruct recovery key: %w", err)
	}

	// Clean up the recovery session
	delete(rm.activeRecoveries, agentIdentity)

	return recoveryKey, nil
}

// GetRecoveryStatus returns the current status of a recovery attempt
func (rm *RecoveryManager) GetRecoveryStatus(agentIdentity string) (*RecoverySession, bool) {
	session, exists := rm.activeRecoveries[agentIdentity]
	return session, exists
}

// ListTrustedAgents returns the trusted agents for an agent's recovery
func (rm *RecoveryManager) ListTrustedAgents(agentIdentity string) ([]string, int, error) {
	config, exists := rm.configs[agentIdentity]
	if !exists {
		return nil, 0, fmt.Errorf("no recovery configuration for agent %s", agentIdentity)
	}

	return config.TrustedAgents, config.Threshold, nil
}

// CancelRecovery cancels an ongoing recovery attempt
func (rm *RecoveryManager) CancelRecovery(agentIdentity string) error {
	if _, exists := rm.activeRecoveries[agentIdentity]; !exists {
		return fmt.Errorf("no active recovery for agent %s", agentIdentity)
	}

	delete(rm.activeRecoveries, agentIdentity)
	return nil
}

// Helper function to get current timestamp (placeholder)
func getCurrentTimestamp() int64 {
	return 1672531200 // 2023-01-01 for testing
}
