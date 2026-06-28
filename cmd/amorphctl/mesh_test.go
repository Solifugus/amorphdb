package main

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/solifugus/amorphdb/internal/config"
)

// MockControlClient simulates the control client for testing
type MockControlClient struct {
	responses map[string][]byte
	commands  []string
}

func NewMockControlClient() *MockControlClient {
	return &MockControlClient{
		responses: make(map[string][]byte),
		commands:  make([]string, 0),
	}
}

func (m *MockControlClient) SendCommand(command string, payload []byte) ([]byte, error) {
	m.commands = append(m.commands, command)

	if response, exists := m.responses[command]; exists {
		return response, nil
	}

	return nil, fmt.Errorf("no mock response for command: %s", command)
}

func (m *MockControlClient) SetResponse(command string, response []byte) {
	m.responses[command] = response
}

func (m *MockControlClient) GetCommands() []string {
	return m.commands
}

func TestMeshCommand_CreateMesh_Success(t *testing.T) {
	mockClient := NewMockControlClient()

	// Set up successful response
	successResponse := CreateMeshResponse{
		Success:   true,
		Message:   "Successfully created mesh 'test-mesh'",
		MeshName:  "test-mesh",
		NodeID:    "node-123",
		IsFounder: true,
	}
	responseData, _ := json.Marshal(successResponse)
	mockClient.SetResponse("create-mesh", responseData)

	meshCommand := NewMeshCommand(mockClient)

	err := meshCommand.CreateMesh("test-mesh")
	if err != nil {
		t.Fatalf("CreateMesh failed: %v", err)
	}

	commands := mockClient.GetCommands()
	if len(commands) != 1 || commands[0] != "create-mesh" {
		t.Errorf("Expected one 'create-mesh' command, got: %v", commands)
	}
}

func TestMeshCommand_CreateMesh_Failure(t *testing.T) {
	mockClient := NewMockControlClient()

	// Set up failure response
	failureResponse := CreateMeshResponse{
		Success: false,
		Message: "Mesh name already exists",
	}
	responseData, _ := json.Marshal(failureResponse)
	mockClient.SetResponse("create-mesh", responseData)

	meshCommand := NewMeshCommand(mockClient)

	err := meshCommand.CreateMesh("existing-mesh")
	if err == nil {
		t.Error("Expected error for failed mesh creation")
	}

	if err.Error() != "mesh creation failed" {
		t.Errorf("Expected 'mesh creation failed', got: %s", err.Error())
	}
}

func TestMeshCommand_CreateMesh_InvalidName(t *testing.T) {
	mockClient := NewMockControlClient()
	meshCommand := NewMeshCommand(mockClient)

	invalidNames := []string{
		"",                    // Empty
		"a",                   // Too short
		"invalid mesh name",   // Contains space
		"invalid@mesh",        // Contains special character
	}

	for _, name := range invalidNames {
		t.Run(fmt.Sprintf("name_%q", name), func(t *testing.T) {
			err := meshCommand.CreateMesh(name)
			if err == nil {
				t.Errorf("Expected error for invalid mesh name: %q", name)
			}

			// Should not have sent command to server
			commands := mockClient.GetCommands()
			if len(commands) > 0 {
				t.Error("Should not send command for invalid mesh name")
			}
		})
	}
}

func TestMeshCommand_GetMeshStatus_Standalone(t *testing.T) {
	mockClient := NewMockControlClient()

	// Set up standalone response
	statusResponse := MeshStatusResponse{
		MeshName:     "",
		Status:       "standalone",
		IsFounder:    false,
		NodeIdentity: "node-456",
		MemberCount:  1,
		AuthorityCount:    1,
		Bridges:      make(map[string]BridgeStatusInfo),
	}
	responseData, _ := json.Marshal(statusResponse)
	mockClient.SetResponse("mesh-status", responseData)

	meshCommand := NewMeshCommand(mockClient)

	err := meshCommand.GetMeshStatus()
	if err != nil {
		t.Fatalf("GetMeshStatus failed: %v", err)
	}

	commands := mockClient.GetCommands()
	if len(commands) != 1 || commands[0] != "mesh-status" {
		t.Errorf("Expected one 'mesh-status' command, got: %v", commands)
	}
}

func TestMeshCommand_GetMeshStatus_WithMesh(t *testing.T) {
	mockClient := NewMockControlClient()

	// Set up mesh response
	foundedAt := int64(1647800000)
	statusResponse := MeshStatusResponse{
		MeshName:     "production-mesh",
		Status:       "founder",
		IsFounder:    true,
		NodeIdentity: "node-789",
		MemberCount:  3,
		AuthorityCount:    2,
		FoundedAt:    &foundedAt,
		Bridges:      make(map[string]BridgeStatusInfo),
	}
	responseData, _ := json.Marshal(statusResponse)
	mockClient.SetResponse("mesh-status", responseData)

	meshCommand := NewMeshCommand(mockClient)

	err := meshCommand.GetMeshStatus()
	if err != nil {
		t.Fatalf("GetMeshStatus failed: %v", err)
	}
}

func TestMeshCommand_GetMeshStatus_WithBridges(t *testing.T) {
	mockClient := NewMockControlClient()

	// Set up response with bridges
	connectedAt := int64(1647800100)
	statusResponse := MeshStatusResponse{
		MeshName:     "main-mesh",
		Status:       "member",
		IsFounder:    false,
		NodeIdentity: "node-101",
		MemberCount:  2,
		AuthorityCount:    1,
		Bridges: map[string]BridgeStatusInfo{
			"partner-mesh": {
				Address:     "192.168.1.10:5000",
				Status:      "connected",
				Identity:    "bridge-identity-123",
				ConnectedAt: &connectedAt,
			},
		},
	}
	responseData, _ := json.Marshal(statusResponse)
	mockClient.SetResponse("mesh-status", responseData)

	meshCommand := NewMeshCommand(mockClient)

	err := meshCommand.GetMeshStatus()
	if err != nil {
		t.Fatalf("GetMeshStatus failed: %v", err)
	}
}

func TestMeshCommand_GetRoleDescription(t *testing.T) {
	mockClient := NewMockControlClient()
	meshCommand := NewMeshCommand(mockClient)

	// Test founder role
	if role := meshCommand.getRoleDescription(true); role != "Founder" {
		t.Errorf("Expected 'Founder', got %q", role)
	}

	// Test member role
	if role := meshCommand.getRoleDescription(false); role != "Member" {
		t.Errorf("Expected 'Member', got %q", role)
	}
}

func TestValidateMeshName_Integration(t *testing.T) {
	// Test that our validation matches the config validation
	testCases := []struct {
		name    string
		valid   bool
	}{
		{"valid-mesh", true},
		{"valid_mesh", true},
		{"ValidMesh123", true},
		{"", false},
		{"a", false},
		{"invalid mesh", false},
		{"invalid@mesh", false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := config.ValidateMeshName(tc.name)
			if tc.valid && err != nil {
				t.Errorf("Expected %q to be valid, got error: %v", tc.name, err)
			} else if !tc.valid && err == nil {
				t.Errorf("Expected %q to be invalid", tc.name)
			}
		})
	}
}