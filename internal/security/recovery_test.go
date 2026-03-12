package security

import (
	"bytes"
	"crypto/rand"
	"testing"
)

func TestShamirSecretSharingSplitAndReconstruct(t *testing.T) {
	sss := NewShamirSecretSharing()

	// Test with different secrets and configurations
	testCases := []struct {
		name        string
		secret      []byte
		threshold   int
		totalShares int
	}{
		{"simple", []byte("hello world"), 2, 3},
		{"binary", []byte{0x01, 0x02, 0x03, 0x04, 0x05}, 3, 5},
		{"large", make([]byte, 64), 4, 7},
		{"minimal", []byte("x"), 1, 1},
		{"complex", []byte("complex secret with many characters!@#$%^&*()"), 5, 10},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Fill large test case with random data
			if tc.name == "large" {
				rand.Read(tc.secret)
			}

			// Split secret
			shards, err := sss.SplitSecret(tc.secret, tc.threshold, tc.totalShares)
			if err != nil {
				t.Fatalf("Failed to split secret: %v", err)
			}

			if len(shards) != tc.totalShares {
				t.Errorf("Expected %d shards, got %d", tc.totalShares, len(shards))
			}

			// Verify shard properties
			for i, shard := range shards {
				if shard.ShareID != i+1 {
					t.Errorf("Shard %d has wrong ID: expected %d, got %d", i, i+1, shard.ShareID)
				}
				if shard.Threshold != tc.threshold {
					t.Errorf("Shard %d has wrong threshold: expected %d, got %d", i, tc.threshold, shard.Threshold)
				}
				if shard.TotalShares != tc.totalShares {
					t.Errorf("Shard %d has wrong total shares: expected %d, got %d", i, tc.totalShares, shard.TotalShares)
				}
			}

			// Test reconstruction with exactly threshold shares
			reconstructed, err := sss.ReconstructSecret(shards[:tc.threshold])
			if err != nil {
				t.Fatalf("Failed to reconstruct secret: %v", err)
			}

			if !bytes.Equal(tc.secret, reconstructed) {
				t.Errorf("Reconstructed secret doesn't match original.\nExpected: %x\nGot: %x", tc.secret, reconstructed)
			}

			// Test reconstruction with more than threshold shares
			if tc.totalShares > tc.threshold {
				reconstructed2, err := sss.ReconstructSecret(shards)
				if err != nil {
					t.Fatalf("Failed to reconstruct with all shares: %v", err)
				}

				if !bytes.Equal(tc.secret, reconstructed2) {
					t.Errorf("Reconstruction with all shares failed")
				}
			}

			// Test reconstruction with different subset of threshold shares
			if tc.totalShares > tc.threshold+1 {
				// Use last threshold shares instead of first
				lastShards := shards[tc.totalShares-tc.threshold:]
				reconstructed3, err := sss.ReconstructSecret(lastShards)
				if err != nil {
					t.Fatalf("Failed to reconstruct with last shares: %v", err)
				}

				if !bytes.Equal(tc.secret, reconstructed3) {
					t.Errorf("Reconstruction with different subset failed")
				}
			}
		})
	}
}

func TestShamirSecretSharingInsufficientShares(t *testing.T) {
	sss := NewShamirSecretSharing()
	secret := []byte("test secret")

	shards, err := sss.SplitSecret(secret, 3, 5)
	if err != nil {
		t.Fatalf("Failed to split secret: %v", err)
	}

	// Try to reconstruct with insufficient shares
	_, err = sss.ReconstructSecret(shards[:2]) // Only 2 shards, need 3
	if err == nil {
		t.Error("Expected error with insufficient shards")
	}
}

func TestShamirSecretSharingInvalidParameters(t *testing.T) {
	sss := NewShamirSecretSharing()
	secret := []byte("test")

	testCases := []struct {
		name        string
		threshold   int
		totalShares int
		shouldError bool
	}{
		{"zero threshold", 0, 3, true},
		{"zero total", 3, 0, true},
		{"threshold > total", 5, 3, true},
		{"negative threshold", -1, 3, true},
		{"negative total", 3, -1, true},
		{"valid minimal", 1, 1, false},
		{"valid normal", 2, 3, false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := sss.SplitSecret(secret, tc.threshold, tc.totalShares)
			if tc.shouldError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tc.shouldError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}
}

func TestShamirSecretSharingEmptySecret(t *testing.T) {
	sss := NewShamirSecretSharing()

	_, err := sss.SplitSecret([]byte{}, 2, 3)
	if err == nil {
		t.Error("Expected error for empty secret")
	}
}

func TestRecoveryManagerCreateConfiguration(t *testing.T) {
	rm := NewRecoveryManager()
	agentIdentity := "test-agent"
	trustedAgents := []string{"agent1", "agent2", "agent3"}
	threshold := 2
	recoveryKey := []byte("super-secret-recovery-key")

	config, err := rm.CreateRecoveryConfiguration(agentIdentity, trustedAgents, threshold, recoveryKey)
	if err != nil {
		t.Fatalf("Failed to create recovery configuration: %v", err)
	}

	if config.AgentIdentity != agentIdentity {
		t.Errorf("Wrong agent identity: expected %s, got %s", agentIdentity, config.AgentIdentity)
	}

	if config.Threshold != threshold {
		t.Errorf("Wrong threshold: expected %d, got %d", threshold, config.Threshold)
	}

	if len(config.TrustedAgents) != len(trustedAgents) {
		t.Errorf("Wrong number of trusted agents: expected %d, got %d", len(trustedAgents), len(config.TrustedAgents))
	}

	if len(config.EncryptedShards) != len(trustedAgents) {
		t.Errorf("Wrong number of encrypted shards: expected %d, got %d", len(trustedAgents), len(config.EncryptedShards))
	}

	// Verify each trusted agent has a shard
	for _, agent := range trustedAgents {
		if _, exists := config.EncryptedShards[agent]; !exists {
			t.Errorf("Missing encrypted shard for agent %s", agent)
		}
	}
}

func TestRecoveryManagerInvalidConfiguration(t *testing.T) {
	rm := NewRecoveryManager()

	testCases := []struct {
		name          string
		trustedAgents []string
		threshold     int
		shouldError   bool
	}{
		{"threshold too high", []string{"a1", "a2"}, 3, true},
		{"zero threshold", []string{"a1", "a2", "a3"}, 0, true},
		{"negative threshold", []string{"a1", "a2"}, -1, true},
		{"empty agents", []string{}, 1, true},
		{"valid config", []string{"a1", "a2", "a3"}, 2, false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := rm.CreateRecoveryConfiguration("test", tc.trustedAgents, tc.threshold, []byte("key"))
			if tc.shouldError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tc.shouldError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}
}

func TestRecoveryFlow(t *testing.T) {
	rm := NewRecoveryManager()

	// Setup recovery configuration
	agentIdentity := "alice"
	trustedAgents := []string{"bob", "charlie", "diana"}
	threshold := 2
	recoveryKey := []byte("alice-recovery-key-123")

	config, err := rm.CreateRecoveryConfiguration(agentIdentity, trustedAgents, threshold, recoveryKey)
	if err != nil {
		t.Fatalf("Failed to create recovery configuration: %v", err)
	}

	// Start recovery
	requestorPublicKey := []byte("temporary-public-key")
	session, err := rm.StartRecovery(agentIdentity, requestorPublicKey)
	if err != nil {
		t.Fatalf("Failed to start recovery: %v", err)
	}

	if session.AgentIdentity != agentIdentity {
		t.Errorf("Wrong agent identity in session: expected %s, got %s", agentIdentity, session.AgentIdentity)
	}

	// Submit shards from trusted agents
	for i, agent := range trustedAgents[:threshold] {
		encryptedShard, exists := config.EncryptedShards[agent]
		if !exists {
			t.Fatalf("No encrypted shard for agent %s", agent)
		}

		err := rm.SubmitRecoveryShard(agent, agentIdentity, encryptedShard)
		if err != nil {
			t.Fatalf("Failed to submit shard from %s: %v", agent, err)
		}

		// Check status
		status, exists := rm.GetRecoveryStatus(agentIdentity)
		if !exists {
			t.Fatalf("Recovery session disappeared after submission %d", i+1)
		}

		expectedShards := i + 1
		if len(status.SubmittedShards) != expectedShards {
			t.Errorf("Expected %d submitted shards, got %d", expectedShards, len(status.SubmittedShards))
		}
	}

	// Attempt recovery
	reconstructed, err := rm.AttemptRecovery(agentIdentity)
	if err != nil {
		t.Fatalf("Failed to attempt recovery: %v", err)
	}

	if !bytes.Equal(recoveryKey, reconstructed) {
		t.Errorf("Reconstructed key doesn't match original.\nExpected: %x\nGot: %x", recoveryKey, reconstructed)
	}

	// Verify session is cleaned up
	_, exists := rm.GetRecoveryStatus(agentIdentity)
	if exists {
		t.Error("Recovery session should be cleaned up after successful recovery")
	}
}

func TestRecoveryUntrustedAgent(t *testing.T) {
	rm := NewRecoveryManager()

	// Setup recovery configuration
	agentIdentity := "alice"
	trustedAgents := []string{"bob", "charlie"}
	threshold := 2
	recoveryKey := []byte("recovery-key")

	_, err := rm.CreateRecoveryConfiguration(agentIdentity, trustedAgents, threshold, recoveryKey)
	if err != nil {
		t.Fatalf("Failed to create recovery configuration: %v", err)
	}

	// Start recovery
	_, err = rm.StartRecovery(agentIdentity, []byte("temp-key"))
	if err != nil {
		t.Fatalf("Failed to start recovery: %v", err)
	}

	// Try to submit shard from untrusted agent
	err = rm.SubmitRecoveryShard("eve", agentIdentity, []byte("fake-shard"))
	if err == nil {
		t.Error("Expected error when untrusted agent submits shard")
	}
}

func TestRecoveryInsufficientShards(t *testing.T) {
	rm := NewRecoveryManager()

	// Setup recovery configuration
	agentIdentity := "alice"
	trustedAgents := []string{"bob", "charlie", "diana"}
	threshold := 3
	recoveryKey := []byte("recovery-key")

	config, err := rm.CreateRecoveryConfiguration(agentIdentity, trustedAgents, threshold, recoveryKey)
	if err != nil {
		t.Fatalf("Failed to create recovery configuration: %v", err)
	}

	// Start recovery
	_, err = rm.StartRecovery(agentIdentity, []byte("temp-key"))
	if err != nil {
		t.Fatalf("Failed to start recovery: %v", err)
	}

	// Submit only 2 shards (need 3)
	for i, agent := range trustedAgents[:2] {
		encryptedShard := config.EncryptedShards[agent]
		err := rm.SubmitRecoveryShard(agent, agentIdentity, encryptedShard)
		if err != nil {
			t.Fatalf("Failed to submit shard %d: %v", i+1, err)
		}
	}

	// Try recovery with insufficient shards
	_, err = rm.AttemptRecovery(agentIdentity)
	if err == nil {
		t.Error("Expected error with insufficient shards")
	}
}

func TestRecoveryNoConfiguration(t *testing.T) {
	rm := NewRecoveryManager()

	// Try to start recovery for non-existent configuration
	_, err := rm.StartRecovery("non-existent", []byte("temp-key"))
	if err == nil {
		t.Error("Expected error for non-existent recovery configuration")
	}
}

func TestRecoveryDuplicateStart(t *testing.T) {
	rm := NewRecoveryManager()

	// Setup recovery configuration
	agentIdentity := "alice"
	trustedAgents := []string{"bob", "charlie"}
	threshold := 2
	recoveryKey := []byte("recovery-key")

	_, err := rm.CreateRecoveryConfiguration(agentIdentity, trustedAgents, threshold, recoveryKey)
	if err != nil {
		t.Fatalf("Failed to create recovery configuration: %v", err)
	}

	// Start recovery
	_, err = rm.StartRecovery(agentIdentity, []byte("temp-key1"))
	if err != nil {
		t.Fatalf("Failed to start recovery: %v", err)
	}

	// Try to start another recovery for same agent
	_, err = rm.StartRecovery(agentIdentity, []byte("temp-key2"))
	if err == nil {
		t.Error("Expected error when starting duplicate recovery")
	}
}

func TestRecoveryCancelRecovery(t *testing.T) {
	rm := NewRecoveryManager()

	// Setup and start recovery
	agentIdentity := "alice"
	trustedAgents := []string{"bob", "charlie"}
	threshold := 2
	recoveryKey := []byte("recovery-key")

	_, err := rm.CreateRecoveryConfiguration(agentIdentity, trustedAgents, threshold, recoveryKey)
	if err != nil {
		t.Fatalf("Failed to create recovery configuration: %v", err)
	}

	_, err = rm.StartRecovery(agentIdentity, []byte("temp-key"))
	if err != nil {
		t.Fatalf("Failed to start recovery: %v", err)
	}

	// Verify recovery exists
	_, exists := rm.GetRecoveryStatus(agentIdentity)
	if !exists {
		t.Error("Recovery should exist before cancellation")
	}

	// Cancel recovery
	err = rm.CancelRecovery(agentIdentity)
	if err != nil {
		t.Fatalf("Failed to cancel recovery: %v", err)
	}

	// Verify recovery is gone
	_, exists = rm.GetRecoveryStatus(agentIdentity)
	if exists {
		t.Error("Recovery should not exist after cancellation")
	}

	// Try to cancel non-existent recovery
	err = rm.CancelRecovery("non-existent")
	if err == nil {
		t.Error("Expected error when canceling non-existent recovery")
	}
}

func TestListTrustedAgents(t *testing.T) {
	rm := NewRecoveryManager()

	agentIdentity := "alice"
	trustedAgents := []string{"bob", "charlie", "diana"}
	threshold := 2
	recoveryKey := []byte("recovery-key")

	_, err := rm.CreateRecoveryConfiguration(agentIdentity, trustedAgents, threshold, recoveryKey)
	if err != nil {
		t.Fatalf("Failed to create recovery configuration: %v", err)
	}

	// List trusted agents
	agents, th, err := rm.ListTrustedAgents(agentIdentity)
	if err != nil {
		t.Fatalf("Failed to list trusted agents: %v", err)
	}

	if th != threshold {
		t.Errorf("Wrong threshold: expected %d, got %d", threshold, th)
	}

	if len(agents) != len(trustedAgents) {
		t.Errorf("Wrong number of agents: expected %d, got %d", len(trustedAgents), len(agents))
	}

	// Verify agent list matches (order may differ)
	agentSet := make(map[string]bool)
	for _, agent := range agents {
		agentSet[agent] = true
	}

	for _, expected := range trustedAgents {
		if !agentSet[expected] {
			t.Errorf("Missing trusted agent: %s", expected)
		}
	}

	// Try to list for non-existent agent
	_, _, err = rm.ListTrustedAgents("non-existent")
	if err == nil {
		t.Error("Expected error for non-existent agent")
	}
}

func TestShamirEdgeCases(t *testing.T) {
	sss := NewShamirSecretSharing()

	// Test with single byte secret
	secret := []byte{42}
	shards, err := sss.SplitSecret(secret, 2, 3)
	if err != nil {
		t.Fatalf("Failed to split single byte secret: %v", err)
	}

	reconstructed, err := sss.ReconstructSecret(shards[:2])
	if err != nil {
		t.Fatalf("Failed to reconstruct single byte secret: %v", err)
	}

	if !bytes.Equal(secret, reconstructed) {
		t.Error("Single byte secret reconstruction failed")
	}

	// Test with maximum shares
	secret = []byte("test-max-shares")
	shards, err = sss.SplitSecret(secret, 10, 10)
	if err != nil {
		t.Fatalf("Failed to split with max shares: %v", err)
	}

	reconstructed, err = sss.ReconstructSecret(shards)
	if err != nil {
		t.Fatalf("Failed to reconstruct with max shares: %v", err)
	}

	if !bytes.Equal(secret, reconstructed) {
		t.Error("Max shares reconstruction failed")
	}
}
