package types

import (
	"encoding/json"
	"testing"
)

func TestAgentType_String(t *testing.T) {
	tests := []struct {
		agentType AgentType
		expected  string
	}{
		{NodeAgent, "node"},
		{MobileAgent, "mobile"},
		{AgentType(999), "unknown"}, // Invalid type
	}

	for _, test := range tests {
		if got := test.agentType.String(); got != test.expected {
			t.Errorf("AgentType(%d).String() = %q, want %q", test.agentType, got, test.expected)
		}
	}
}

func TestAgentType_MarshalJSON(t *testing.T) {
	tests := []struct {
		agentType AgentType
		expected  string
	}{
		{NodeAgent, `"node"`},
		{MobileAgent, `"mobile"`},
		{AgentType(999), `"unknown"`},
	}

	for _, test := range tests {
		data, err := json.Marshal(test.agentType)
		if err != nil {
			t.Errorf("AgentType(%d).MarshalJSON() error = %v", test.agentType, err)
			continue
		}

		if string(data) != test.expected {
			t.Errorf("AgentType(%d).MarshalJSON() = %s, want %s", test.agentType, string(data), test.expected)
		}
	}
}

func TestAgentType_UnmarshalJSON(t *testing.T) {
	tests := []struct {
		json     string
		expected AgentType
		wantErr  bool
	}{
		{`"node"`, NodeAgent, false},
		{`"mobile"`, MobileAgent, false},
		{`"invalid"`, AgentType(0), true},
		{`42`, AgentType(0), true}, // Invalid JSON type
	}

	for _, test := range tests {
		var agentType AgentType
		err := json.Unmarshal([]byte(test.json), &agentType)

		if test.wantErr {
			if err == nil {
				t.Errorf("AgentType.UnmarshalJSON(%s) expected error, got nil", test.json)
			}
			continue
		}

		if err != nil {
			t.Errorf("AgentType.UnmarshalJSON(%s) error = %v", test.json, err)
			continue
		}

		if agentType != test.expected {
			t.Errorf("AgentType.UnmarshalJSON(%s) = %v, want %v", test.json, agentType, test.expected)
		}
	}
}

func TestAgentType_IsValid(t *testing.T) {
	tests := []struct {
		agentType AgentType
		expected  bool
	}{
		{NodeAgent, true},
		{MobileAgent, true},
		{AgentType(-1), false},
		{AgentType(2), false},
		{AgentType(999), false},
	}

	for _, test := range tests {
		if got := test.agentType.IsValid(); got != test.expected {
			t.Errorf("AgentType(%d).IsValid() = %v, want %v", test.agentType, got, test.expected)
		}
	}
}

func TestAgentType_CanReplicate(t *testing.T) {
	tests := []struct {
		agentType AgentType
		expected  bool
	}{
		{NodeAgent, false},
		{MobileAgent, true},
		{AgentType(999), false}, // Invalid types can't replicate
	}

	for _, test := range tests {
		if got := test.agentType.CanReplicate(); got != test.expected {
			t.Errorf("AgentType(%d).CanReplicate() = %v, want %v", test.agentType, got, test.expected)
		}
	}
}

func TestAgentType_RequiresMeshStorage(t *testing.T) {
	tests := []struct {
		agentType AgentType
		expected  bool
	}{
		{NodeAgent, false},
		{MobileAgent, true},
		{AgentType(999), false}, // Invalid types don't require mesh storage
	}

	for _, test := range tests {
		if got := test.agentType.RequiresMeshStorage(); got != test.expected {
			t.Errorf("AgentType(%d).RequiresMeshStorage() = %v, want %v", test.agentType, got, test.expected)
		}
	}
}

func TestAgentType_JSONRoundTrip(t *testing.T) {
	// Test that we can marshal and unmarshal without loss
	original := MobileAgent

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Failed to marshal AgentType: %v", err)
	}

	var unmarshaled AgentType
	if err := json.Unmarshal(data, &unmarshaled); err != nil {
		t.Fatalf("Failed to unmarshal AgentType: %v", err)
	}

	if unmarshaled != original {
		t.Errorf("JSON round trip failed: got %v, want %v", unmarshaled, original)
	}
}