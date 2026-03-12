package security

import (
	"bytes"
	"testing"
)

func TestAuthChallengeMessageSerialization(t *testing.T) {
	// Create test message
	msg := &AuthChallengeMessage{
		AgentIdentity: "test-agent-123",
		ChallengeID:   []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f, 0x10},
		EncryptedData: []byte("encrypted-challenge-data-for-testing"),
	}

	// Serialize
	serialized := SerializeAuthChallenge(msg)

	// Deserialize
	deserialized, err := DeserializeAuthChallenge(serialized)
	if err != nil {
		t.Fatalf("Failed to deserialize auth challenge: %v", err)
	}

	// Verify fields match
	if deserialized.AgentIdentity != msg.AgentIdentity {
		t.Errorf("Identity mismatch: expected %s, got %s", msg.AgentIdentity, deserialized.AgentIdentity)
	}

	if !bytes.Equal(deserialized.ChallengeID, msg.ChallengeID) {
		t.Errorf("Challenge ID mismatch: expected %x, got %x", msg.ChallengeID, deserialized.ChallengeID)
	}

	if !bytes.Equal(deserialized.EncryptedData, msg.EncryptedData) {
		t.Errorf("Encrypted data mismatch: expected %x, got %x", msg.EncryptedData, deserialized.EncryptedData)
	}
}

func TestAuthResponseMessageSerialization(t *testing.T) {
	// Create test message
	msg := &AuthResponseMessage{
		AgentIdentity: "response-agent-456",
		ChallengeID:   []byte{0x10, 0x0f, 0x0e, 0x0d, 0x0c, 0x0b, 0x0a, 0x09, 0x08, 0x07, 0x06, 0x05, 0x04, 0x03, 0x02, 0x01},
		DecryptedData: []byte("decrypted-challenge-response-data"),
	}

	// Serialize
	serialized := SerializeAuthResponse(msg)

	// Deserialize
	deserialized, err := DeserializeAuthResponse(serialized)
	if err != nil {
		t.Fatalf("Failed to deserialize auth response: %v", err)
	}

	// Verify fields match
	if deserialized.AgentIdentity != msg.AgentIdentity {
		t.Errorf("Identity mismatch: expected %s, got %s", msg.AgentIdentity, deserialized.AgentIdentity)
	}

	if !bytes.Equal(deserialized.ChallengeID, msg.ChallengeID) {
		t.Errorf("Challenge ID mismatch: expected %x, got %x", msg.ChallengeID, deserialized.ChallengeID)
	}

	if !bytes.Equal(deserialized.DecryptedData, msg.DecryptedData) {
		t.Errorf("Decrypted data mismatch: expected %x, got %x", msg.DecryptedData, deserialized.DecryptedData)
	}
}

func TestAuthChallengeDeserializationErrors(t *testing.T) {
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
			_, err := DeserializeAuthChallenge(tc.data)
			if err == nil {
				t.Errorf("Expected error for %s, but got none", tc.name)
			}
		})
	}
}

func TestAuthResponseDeserializationErrors(t *testing.T) {
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
			_, err := DeserializeAuthResponse(tc.data)
			if err == nil {
				t.Errorf("Expected error for %s, but got none", tc.name)
			}
		})
	}
}

func TestAuthenticationSession(t *testing.T) {
	// Create test agent
	authenticator := NewAgentAuthenticator()
	agentID, err := authenticator.CreateAgentIdentity("session-test-agent", "passphrase", "device")
	if err != nil {
		t.Fatalf("Failed to create agent identity: %v", err)
	}

	// Create authentication session
	session := NewAuthenticationSession(agentID.Identity, agentID.PublicKey)

	// Create challenge
	challengeMsg, err := session.CreateChallenge()
	if err != nil {
		t.Fatalf("Failed to create challenge: %v", err)
	}

	if challengeMsg.AgentIdentity != agentID.Identity {
		t.Errorf("Challenge identity mismatch: expected %s, got %s", agentID.Identity, challengeMsg.AgentIdentity)
	}

	// Agent responds to challenge
	authChallenge := &AuthenticationChallenge{
		ChallengeID:   challengeMsg.ChallengeID,
		EncryptedData: challengeMsg.EncryptedData,
	}

	authResponse, err := authenticator.RespondToChallenge(authChallenge, agentID.KeyPair)
	if err != nil {
		t.Fatalf("Failed to respond to challenge: %v", err)
	}

	// Convert to protocol message
	responseMsg := &AuthResponseMessage{
		AgentIdentity: agentID.Identity,
		ChallengeID:   authResponse.ChallengeID,
		DecryptedData: authResponse.DecryptedData,
	}

	// Verify response
	valid, err := session.VerifyResponse(responseMsg)
	if err != nil {
		t.Fatalf("Failed to verify response: %v", err)
	}

	if !valid {
		t.Error("Authentication session should have succeeded")
	}
}

func TestAuthenticationSessionIdentityMismatch(t *testing.T) {
	// Create test agents
	authenticator := NewAgentAuthenticator()
	agentID1, err := authenticator.CreateAgentIdentity("agent1", "pass1", "device1")
	if err != nil {
		t.Fatalf("Failed to create agent identity 1: %v", err)
	}

	agentID2, err := authenticator.CreateAgentIdentity("agent2", "pass2", "device2")
	if err != nil {
		t.Fatalf("Failed to create agent identity 2: %v", err)
	}

	// Create authentication session for agent1
	session := NewAuthenticationSession(agentID1.Identity, agentID1.PublicKey)

	// Create challenge
	challengeMsg, err := session.CreateChallenge()
	if err != nil {
		t.Fatalf("Failed to create challenge: %v", err)
	}

	// Agent2 tries to respond (wrong identity)
	responseMsg := &AuthResponseMessage{
		AgentIdentity: agentID2.Identity, // Wrong identity
		ChallengeID:   challengeMsg.ChallengeID,
		DecryptedData: []byte("fake-response"),
	}

	// Verify response should fail due to identity mismatch
	valid, err := session.VerifyResponse(responseMsg)
	if err == nil {
		t.Error("Expected error for identity mismatch")
	}
	if valid {
		t.Error("Authentication with wrong identity should fail")
	}
}

func TestMessageProtocolConstants(t *testing.T) {
	// Verify protocol message type constants match design document
	expectedTypes := map[string]byte{
		"MsgKeyExchange":   0x01,
		"MsgIdentityReq":   0x02,
		"MsgIdentityGrant": 0x03,
		"MsgAuthChallenge": 0x04,
		"MsgAuthResponse":  0x05,
		"MsgPeerExchange":  0x06,
	}

	actualTypes := map[string]byte{
		"MsgKeyExchange":   MsgKeyExchange,
		"MsgIdentityReq":   MsgIdentityReq,
		"MsgIdentityGrant": MsgIdentityGrant,
		"MsgAuthChallenge": MsgAuthChallenge,
		"MsgAuthResponse":  MsgAuthResponse,
		"MsgPeerExchange":  MsgPeerExchange,
	}

	for name, expected := range expectedTypes {
		if actual, exists := actualTypes[name]; !exists || actual != expected {
			t.Errorf("Protocol constant %s: expected 0x%02x, got 0x%02x", name, expected, actual)
		}
	}
}

func TestSerializationRoundTripStress(t *testing.T) {
	// Test with various message sizes and content
	testCases := []struct {
		name     string
		identity string
		dataSize int
	}{
		{"tiny", "a", 1},
		{"small", "small-agent", 32},
		{"medium", "medium-length-agent-identifier", 256},
		{"large", "very-long-agent-identifier-with-many-characters-to-test-serialization", 4096},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create test data
			challengeID := make([]byte, 16)
			for i := range challengeID {
				challengeID[i] = byte(i)
			}

			encryptedData := make([]byte, tc.dataSize)
			for i := range encryptedData {
				encryptedData[i] = byte(i % 256)
			}

			// Test auth challenge
			challengeMsg := &AuthChallengeMessage{
				AgentIdentity: tc.identity,
				ChallengeID:   challengeID,
				EncryptedData: encryptedData,
			}

			serialized := SerializeAuthChallenge(challengeMsg)
			deserialized, err := DeserializeAuthChallenge(serialized)
			if err != nil {
				t.Fatalf("Failed to deserialize challenge: %v", err)
			}

			if deserialized.AgentIdentity != challengeMsg.AgentIdentity ||
				!bytes.Equal(deserialized.ChallengeID, challengeMsg.ChallengeID) ||
				!bytes.Equal(deserialized.EncryptedData, challengeMsg.EncryptedData) {
				t.Error("Challenge round-trip serialization failed")
			}

			// Test auth response
			responseMsg := &AuthResponseMessage{
				AgentIdentity: tc.identity,
				ChallengeID:   challengeID,
				DecryptedData: encryptedData,
			}

			serialized = SerializeAuthResponse(responseMsg)
			deserializedResp, err := DeserializeAuthResponse(serialized)
			if err != nil {
				t.Fatalf("Failed to deserialize response: %v", err)
			}

			if deserializedResp.AgentIdentity != responseMsg.AgentIdentity ||
				!bytes.Equal(deserializedResp.ChallengeID, responseMsg.ChallengeID) ||
				!bytes.Equal(deserializedResp.DecryptedData, responseMsg.DecryptedData) {
				t.Error("Response round-trip serialization failed")
			}
		})
	}
}
