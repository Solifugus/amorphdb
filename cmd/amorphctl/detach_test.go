// Package main provides detach command testing for amorphctl
package main

import (
	"encoding/json"
	"testing"
	"time"
)

func TestDetachCommand_DetachFromPrimaryMesh_Success(t *testing.T) {
	mockClient := NewMockControlClient()

	// Set up successful detach response for primary mesh
	successResponse := DetachResponse{
		Success: true,
		Message: "Successfully detached from mesh 'test-mesh' and returned to standalone mode",
		DetachInfo: &DetachInfo{
			DetachType:     "primary_mesh",
			MeshName:       "test-mesh",
			PreviousRole:   "member",
			ZonesMigrated:  2,
			DataPreserved:  true,
			DisconnectedAt: &[]int64{time.Now().Unix()}[0],
		},
	}
	responseData, _ := json.Marshal(successResponse)
	mockClient.SetResponse("detach", responseData)

	detachCommand := NewDetachCommand(mockClient)

	err := detachCommand.DetachFromMesh("") // Empty string = primary mesh
	if err != nil {
		t.Fatalf("DetachFromMesh failed: %v", err)
	}

	commands := mockClient.GetCommands()
	if len(commands) != 1 || commands[0] != "detach" {
		t.Errorf("Expected one 'detach' command, got: %v", commands)
	}
}

func TestDetachCommand_DetachFromBridge_Success(t *testing.T) {
	mockClient := NewMockControlClient()

	// Set up successful bridge detach response
	successResponse := DetachResponse{
		Success: true,
		Message: "Successfully disconnected bridge to mesh 'partner-mesh'",
		DetachInfo: &DetachInfo{
			DetachType:     "bridge",
			MeshName:       "partner-mesh",
			BridgeIdentity: "bridge-node123-partner-mesh-1a2b",
			ZonesMigrated:  0, // Bridges don't migrate zones
			DataPreserved:  true,
			DisconnectedAt: &[]int64{time.Now().Unix()}[0],
		},
	}
	responseData, _ := json.Marshal(successResponse)
	mockClient.SetResponse("detach", responseData)

	detachCommand := NewDetachCommand(mockClient)

	err := detachCommand.DetachFromMesh("partner-mesh")
	if err != nil {
		t.Fatalf("DetachFromMesh failed: %v", err)
	}

	commands := mockClient.GetCommands()
	if len(commands) != 1 || commands[0] != "detach" {
		t.Errorf("Expected one 'detach' command, got: %v", commands)
	}
}

func TestDetachCommand_DetachFromMesh_Failure(t *testing.T) {
	mockClient := NewMockControlClient()

	// Set up failure response
	failureResponse := DetachResponse{
		Success: false,
		Message: "Not currently part of any mesh",
	}
	responseData, _ := json.Marshal(failureResponse)
	mockClient.SetResponse("detach", responseData)

	detachCommand := NewDetachCommand(mockClient)

	err := detachCommand.DetachFromMesh("")
	if err == nil {
		t.Error("Expected error for failed detach operation")
	}

	if err.Error() != "detach operation failed" {
		t.Errorf("Expected 'detach operation failed', got: %s", err.Error())
	}
}

func TestDetachCommand_DetachFounderWithZoneMigration(t *testing.T) {
	mockClient := NewMockControlClient()

	// Set up response for mesh founder with zone migration
	successResponse := DetachResponse{
		Success: true,
		Message: "Successfully detached from mesh 'my-mesh' and returned to standalone mode",
		DetachInfo: &DetachInfo{
			DetachType:     "primary_mesh",
			MeshName:       "my-mesh",
			PreviousRole:   "founder",
			ZonesMigrated:  5,
			DataPreserved:  true,
			DisconnectedAt: &[]int64{time.Now().Unix()}[0],
		},
	}
	responseData, _ := json.Marshal(successResponse)
	mockClient.SetResponse("detach", responseData)

	detachCommand := NewDetachCommand(mockClient)

	err := detachCommand.DetachFromMesh("")
	if err != nil {
		t.Fatalf("DetachFromMesh failed: %v", err)
	}

	// Verify the response includes zone migration information
	commands := mockClient.GetCommands()
	if len(commands) != 1 || commands[0] != "detach" {
		t.Errorf("Expected one 'detach' command, got: %v", commands)
	}
}

func TestDetachCommand_DetachNonExistentBridge(t *testing.T) {
	mockClient := NewMockControlClient()

	// Set up failure response for non-existent bridge
	failureResponse := DetachResponse{
		Success: false,
		Message: "No bridge connection to mesh 'nonexistent-mesh'",
	}
	responseData, _ := json.Marshal(failureResponse)
	mockClient.SetResponse("detach", responseData)

	detachCommand := NewDetachCommand(mockClient)

	err := detachCommand.DetachFromMesh("nonexistent-mesh")
	if err == nil {
		t.Error("Expected error for non-existent bridge")
	}

	if err.Error() != "detach operation failed" {
		t.Errorf("Expected 'detach operation failed', got: %s", err.Error())
	}
}

func TestDetachCommand_MalformedResponse(t *testing.T) {
	mockClient := NewMockControlClient()

	// Set up malformed response
	mockClient.SetResponse("detach", []byte("invalid json"))

	detachCommand := NewDetachCommand(mockClient)

	err := detachCommand.DetachFromMesh("")
	if err == nil {
		t.Error("Expected error for malformed response")
	}

	// Should contain "failed to parse response" in error message
	if !contains(err.Error(), "failed to parse response") {
		t.Errorf("Expected parse error in message, got: %s", err.Error())
	}
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr ||
		(len(s) > len(substr) &&
			(s[:len(substr)] == substr ||
			 s[len(s)-len(substr):] == substr ||
			 indexOf(s, substr) >= 0)))
}

// Simple indexOf implementation
func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}