package security

import (
	"bytes"
	"testing"
)

func TestRecoveryShardMessageSerialization(t *testing.T) {
	msg := &RecoveryShardMessage{
		RecoveryAgentIdentity: "recovery-agent-bob",
		TargetAgentIdentity:   "target-agent-alice",
		ShareID:               3,
		ShareValue:            []byte("shard-value-data-12345"),
		Threshold:             2,
		TotalShares:           5,
		Signature:             []byte("signature-placeholder"),
	}

	// Serialize
	serialized := SerializeRecoveryShardMessage(msg)

	// Deserialize
	deserialized, err := DeserializeRecoveryShardMessage(serialized)
	if err != nil {
		t.Fatalf("Failed to deserialize recovery shard message: %v", err)
	}

	// Verify all fields match
	if deserialized.RecoveryAgentIdentity != msg.RecoveryAgentIdentity {
		t.Errorf("Recovery agent mismatch: expected %s, got %s", msg.RecoveryAgentIdentity, deserialized.RecoveryAgentIdentity)
	}

	if deserialized.TargetAgentIdentity != msg.TargetAgentIdentity {
		t.Errorf("Target agent mismatch: expected %s, got %s", msg.TargetAgentIdentity, deserialized.TargetAgentIdentity)
	}

	if deserialized.ShareID != msg.ShareID {
		t.Errorf("Share ID mismatch: expected %d, got %d", msg.ShareID, deserialized.ShareID)
	}

	if !bytes.Equal(deserialized.ShareValue, msg.ShareValue) {
		t.Errorf("Share value mismatch: expected %x, got %x", msg.ShareValue, deserialized.ShareValue)
	}

	if deserialized.Threshold != msg.Threshold {
		t.Errorf("Threshold mismatch: expected %d, got %d", msg.Threshold, deserialized.Threshold)
	}

	if deserialized.TotalShares != msg.TotalShares {
		t.Errorf("Total shares mismatch: expected %d, got %d", msg.TotalShares, deserialized.TotalShares)
	}

	if !bytes.Equal(deserialized.Signature, msg.Signature) {
		t.Errorf("Signature mismatch: expected %x, got %x", msg.Signature, deserialized.Signature)
	}
}

func TestRecoveryRequestMessageSerialization(t *testing.T) {
	msg := &RecoveryRequestMessage{
		AgentIdentity:      "requesting-agent-charlie",
		RequestorPublicKey: []byte("temporary-public-key-for-recovery"),
	}

	// Serialize
	serialized := SerializeRecoveryRequestMessage(msg)

	// Deserialize
	deserialized, err := DeserializeRecoveryRequestMessage(serialized)
	if err != nil {
		t.Fatalf("Failed to deserialize recovery request message: %v", err)
	}

	// Verify fields match
	if deserialized.AgentIdentity != msg.AgentIdentity {
		t.Errorf("Agent identity mismatch: expected %s, got %s", msg.AgentIdentity, deserialized.AgentIdentity)
	}

	if !bytes.Equal(deserialized.RequestorPublicKey, msg.RequestorPublicKey) {
		t.Errorf("Public key mismatch: expected %x, got %x", msg.RequestorPublicKey, deserialized.RequestorPublicKey)
	}
}

func TestRecoveryShardMessageDeserializationErrors(t *testing.T) {
	testCases := []struct {
		name string
		data []byte
	}{
		{
			name: "empty data",
			data: []byte{},
		},
		{
			name: "too short",
			data: []byte{0x00, 0x00, 0x00, 0x05},
		},
		{
			name: "invalid recovery agent length",
			data: []byte{0x00, 0x00, 0x00, 0xFF, 0x00, 0x00, 0x00, 0x01},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := DeserializeRecoveryShardMessage(tc.data)
			if err == nil {
				t.Errorf("Expected error for %s, but got none", tc.name)
			}
		})
	}
}

func TestRecoveryRequestMessageDeserializationErrors(t *testing.T) {
	testCases := []struct {
		name string
		data []byte
	}{
		{
			name: "empty data",
			data: []byte{},
		},
		{
			name: "too short",
			data: []byte{0x00, 0x00, 0x00, 0x05},
		},
		{
			name: "invalid identity length",
			data: []byte{0x00, 0x00, 0x00, 0xFF, 0x00, 0x00, 0x00, 0x01},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := DeserializeRecoveryRequestMessage(tc.data)
			if err == nil {
				t.Errorf("Expected error for %s, but got none", tc.name)
			}
		})
	}
}

func TestRecoveryProtocolHandler(t *testing.T) {
	handler := NewRecoveryProtocolHandler()

	// Setup recovery configuration
	agentIdentity := "alice"
	trustedAgents := []string{"bob", "charlie", "diana"}
	threshold := 2
	recoveryKey := []byte("alice-master-recovery-key")

	err := handler.CreateRecoveryConfiguration(agentIdentity, trustedAgents, threshold, recoveryKey)
	if err != nil {
		t.Fatalf("Failed to create recovery configuration: %v", err)
	}

	// Test recovery request
	requestMsg := &RecoveryRequestMessage{
		AgentIdentity:      agentIdentity,
		RequestorPublicKey: []byte("temp-public-key"),
	}

	statusMsg, err := handler.HandleRecoveryRequest(requestMsg)
	if err != nil {
		t.Fatalf("Failed to handle recovery request: %v", err)
	}

	if statusMsg.AgentIdentity != agentIdentity {
		t.Errorf("Wrong agent identity in status: expected %s, got %s", agentIdentity, statusMsg.AgentIdentity)
	}

	if !statusMsg.RecoveryInProgress {
		t.Error("Recovery should be in progress")
	}

	if statusMsg.Threshold != threshold {
		t.Errorf("Wrong threshold in status: expected %d, got %d", threshold, statusMsg.Threshold)
	}

	if len(statusMsg.TrustedAgents) != len(trustedAgents) {
		t.Errorf("Wrong number of trusted agents: expected %d, got %d", len(trustedAgents), len(statusMsg.TrustedAgents))
	}

	// Submit recovery shards
	for i, agent := range trustedAgents[:threshold] {
		shardMsg := &RecoveryShardMessage{
			RecoveryAgentIdentity: agent,
			TargetAgentIdentity:   agentIdentity,
			ShareID:               i + 1,
			ShareValue:            []byte("fake-shard-value"), // This would be the actual shard in production
			Threshold:             threshold,
			TotalShares:           len(trustedAgents),
			Signature:             []byte("signature"),
		}

		statusMsg, err = handler.HandleRecoveryShardSubmission(shardMsg)
		if err != nil {
			t.Fatalf("Failed to handle shard submission from %s: %v", agent, err)
		}

		expectedShards := i + 1
		if statusMsg.SubmittedShards != expectedShards {
			t.Errorf("Wrong number of submitted shards: expected %d, got %d", expectedShards, statusMsg.SubmittedShards)
		}

		if len(statusMsg.ShardsFromAgents) != expectedShards {
			t.Errorf("Wrong number of agents in shard list: expected %d, got %d", expectedShards, len(statusMsg.ShardsFromAgents))
		}
	}

	// Note: We can't test successful recovery reconstruction in this integration test
	// because our test setup uses fake shard values instead of real split secrets.
	// The individual components are tested separately in recovery_test.go
}

func TestRecoveryProtocolHandlerInvalidRequest(t *testing.T) {
	handler := NewRecoveryProtocolHandler()

	// Try to start recovery for non-existent configuration
	requestMsg := &RecoveryRequestMessage{
		AgentIdentity:      "non-existent",
		RequestorPublicKey: []byte("temp-key"),
	}

	_, err := handler.HandleRecoveryRequest(requestMsg)
	if err == nil {
		t.Error("Expected error for non-existent recovery configuration")
	}
}

func TestRecoveryProtocolHandlerUntrustedShard(t *testing.T) {
	handler := NewRecoveryProtocolHandler()

	// Setup recovery configuration
	agentIdentity := "alice"
	trustedAgents := []string{"bob", "charlie"}
	threshold := 2
	recoveryKey := []byte("recovery-key")

	err := handler.CreateRecoveryConfiguration(agentIdentity, trustedAgents, threshold, recoveryKey)
	if err != nil {
		t.Fatalf("Failed to create recovery configuration: %v", err)
	}

	// Start recovery
	requestMsg := &RecoveryRequestMessage{
		AgentIdentity:      agentIdentity,
		RequestorPublicKey: []byte("temp-key"),
	}

	_, err = handler.HandleRecoveryRequest(requestMsg)
	if err != nil {
		t.Fatalf("Failed to handle recovery request: %v", err)
	}

	// Try to submit shard from untrusted agent
	shardMsg := &RecoveryShardMessage{
		RecoveryAgentIdentity: "eve", // Untrusted agent
		TargetAgentIdentity:   agentIdentity,
		ShareID:               1,
		ShareValue:            []byte("fake-shard"),
		Threshold:             threshold,
		TotalShares:           2,
		Signature:             []byte("signature"),
	}

	_, err = handler.HandleRecoveryShardSubmission(shardMsg)
	if err == nil {
		t.Error("Expected error for shard submission from untrusted agent")
	}
}

func TestRecoveryProtocolHandlerGetStatus(t *testing.T) {
	handler := NewRecoveryProtocolHandler()

	agentIdentity := "alice"

	// Get status for non-existent recovery
	status, err := handler.GetRecoveryStatus(agentIdentity)
	if err != nil {
		t.Fatalf("Failed to get status: %v", err)
	}

	if status.RecoveryInProgress {
		t.Error("Recovery should not be in progress for non-existent recovery")
	}

	// Setup recovery configuration and start recovery
	trustedAgents := []string{"bob", "charlie"}
	threshold := 2
	recoveryKey := []byte("recovery-key")

	err = handler.CreateRecoveryConfiguration(agentIdentity, trustedAgents, threshold, recoveryKey)
	if err != nil {
		t.Fatalf("Failed to create recovery configuration: %v", err)
	}

	requestMsg := &RecoveryRequestMessage{
		AgentIdentity:      agentIdentity,
		RequestorPublicKey: []byte("temp-key"),
	}

	_, err = handler.HandleRecoveryRequest(requestMsg)
	if err != nil {
		t.Fatalf("Failed to handle recovery request: %v", err)
	}

	// Get status for active recovery
	status, err = handler.GetRecoveryStatus(agentIdentity)
	if err != nil {
		t.Fatalf("Failed to get status: %v", err)
	}

	if !status.RecoveryInProgress {
		t.Error("Recovery should be in progress")
	}

	if status.SubmittedShards != 0 {
		t.Errorf("Expected 0 submitted shards, got %d", status.SubmittedShards)
	}
}

func TestRecoveryProtocolHandlerCancelRecovery(t *testing.T) {
	handler := NewRecoveryProtocolHandler()

	agentIdentity := "alice"
	trustedAgents := []string{"bob", "charlie"}
	threshold := 2
	recoveryKey := []byte("recovery-key")

	// Setup and start recovery
	err := handler.CreateRecoveryConfiguration(agentIdentity, trustedAgents, threshold, recoveryKey)
	if err != nil {
		t.Fatalf("Failed to create recovery configuration: %v", err)
	}

	requestMsg := &RecoveryRequestMessage{
		AgentIdentity:      agentIdentity,
		RequestorPublicKey: []byte("temp-key"),
	}

	_, err = handler.HandleRecoveryRequest(requestMsg)
	if err != nil {
		t.Fatalf("Failed to handle recovery request: %v", err)
	}

	// Verify recovery is active
	status, err := handler.GetRecoveryStatus(agentIdentity)
	if err != nil {
		t.Fatalf("Failed to get status: %v", err)
	}

	if !status.RecoveryInProgress {
		t.Error("Recovery should be in progress before cancellation")
	}

	// Cancel recovery
	err = handler.CancelRecovery(agentIdentity)
	if err != nil {
		t.Fatalf("Failed to cancel recovery: %v", err)
	}

	// Verify recovery is no longer active
	status, err = handler.GetRecoveryStatus(agentIdentity)
	if err != nil {
		t.Fatalf("Failed to get status after cancellation: %v", err)
	}

	if status.RecoveryInProgress {
		t.Error("Recovery should not be in progress after cancellation")
	}
}

func TestRecoveryShardMessageConstants(t *testing.T) {
	// Verify protocol message constant matches design document
	expectedMsgRecoveryShard := byte(0x30)

	if MsgRecoveryShard != expectedMsgRecoveryShard {
		t.Errorf("MsgRecoveryShard constant mismatch: expected 0x%02x, got 0x%02x", expectedMsgRecoveryShard, MsgRecoveryShard)
	}
}

func TestRecoveryMessageSerializationRoundTrip(t *testing.T) {
	// Test with various message sizes and content
	testCases := []struct {
		name string
		msg  *RecoveryShardMessage
	}{
		{
			"minimal",
			&RecoveryShardMessage{
				RecoveryAgentIdentity: "a",
				TargetAgentIdentity:   "b",
				ShareID:               1,
				ShareValue:            []byte{0x01},
				Threshold:             1,
				TotalShares:           1,
				Signature:             []byte{0x02},
			},
		},
		{
			"large",
			&RecoveryShardMessage{
				RecoveryAgentIdentity: "very-long-recovery-agent-identifier-with-many-characters",
				TargetAgentIdentity:   "equally-long-target-agent-identifier-for-testing",
				ShareID:               999,
				ShareValue:            make([]byte, 1024),
				Threshold:             50,
				TotalShares:           100,
				Signature:             make([]byte, 256),
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Fill large test case with pattern data
			if tc.name == "large" {
				for i := range tc.msg.ShareValue {
					tc.msg.ShareValue[i] = byte(i % 256)
				}
				for i := range tc.msg.Signature {
					tc.msg.Signature[i] = byte((i * 7) % 256)
				}
			}

			serialized := SerializeRecoveryShardMessage(tc.msg)
			deserialized, err := DeserializeRecoveryShardMessage(serialized)
			if err != nil {
				t.Fatalf("Failed round-trip for %s: %v", tc.name, err)
			}

			// Verify all fields match
			if deserialized.RecoveryAgentIdentity != tc.msg.RecoveryAgentIdentity ||
				deserialized.TargetAgentIdentity != tc.msg.TargetAgentIdentity ||
				deserialized.ShareID != tc.msg.ShareID ||
				!bytes.Equal(deserialized.ShareValue, tc.msg.ShareValue) ||
				deserialized.Threshold != tc.msg.Threshold ||
				deserialized.TotalShares != tc.msg.TotalShares ||
				!bytes.Equal(deserialized.Signature, tc.msg.Signature) {
				t.Errorf("Round-trip serialization failed for %s", tc.name)
			}
		})
	}
}
