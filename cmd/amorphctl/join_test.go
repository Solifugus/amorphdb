// Package main provides join command testing for amorphctl
package main

import (
	"encoding/json"
	"testing"
	"time"
)

func TestJoinCommand_JoinMesh_Success(t *testing.T) {
	mockClient := NewMockControlClient()

	// Set up successful join response
	successResponse := JoinMeshResponse{
		Success:      true,
		Message:      "Successfully joined mesh 'test-mesh'",
		NodeIdentity: "node-456",
		MeshInfo: &JoinedMeshInfo{
			Name:            "test-mesh",
			PathAuthorities: "node-123",
			MemberCount:     2,
			JoinedAt:        time.Now(),
		},
		DataPreserved: true,
	}
	responseData, _ := json.Marshal(successResponse)
	mockClient.SetResponse("join-mesh", responseData)

	joinCommand := NewJoinCommand(mockClient)

	err := joinCommand.JoinMesh("192.168.1.10:5000")
	if err != nil {
		t.Fatalf("JoinMesh failed: %v", err)
	}

	commands := mockClient.GetCommands()
	if len(commands) != 1 || commands[0] != "join-mesh" {
		t.Errorf("Expected one 'join-mesh' command, got: %v", commands)
	}
}

func TestJoinCommand_JoinMesh_Failure(t *testing.T) {
	mockClient := NewMockControlClient()

	// Set up failure response
	failureResponse := JoinMeshResponse{
		Success: false,
		Message: "Mesh not found or access denied",
	}
	responseData, _ := json.Marshal(failureResponse)
	mockClient.SetResponse("join-mesh", responseData)

	joinCommand := NewJoinCommand(mockClient)

	err := joinCommand.JoinMesh("192.168.1.10:5000")
	if err == nil {
		t.Error("Expected error for failed mesh join")
	}

	if err.Error() != "mesh join failed" {
		t.Errorf("Expected 'mesh join failed', got: %s", err.Error())
	}
}

func TestJoinCommand_ValidateSeedAddress(t *testing.T) {
	mockClient := NewMockControlClient()
	joinCommand := NewJoinCommand(mockClient)

	validAddresses := []string{
		"192.168.1.10",
		"192.168.1.10:5000",
		"example.com",
		"example.com:8080",
		"localhost",
		"localhost:3000",
	}

	for _, addr := range validAddresses {
		t.Run("valid_"+addr, func(t *testing.T) {
			err := joinCommand.validateSeedAddress(addr)
			if err != nil {
				t.Errorf("Expected valid address %q to pass validation, got error: %v", addr, err)
			}
		})
	}

	invalidAddresses := []string{
		"",
		":5000",
		"192.168.1.10:",
	}

	for _, addr := range invalidAddresses {
		t.Run("invalid_"+addr, func(t *testing.T) {
			err := joinCommand.validateSeedAddress(addr)
			if err == nil {
				t.Errorf("Expected invalid address %q to fail validation", addr)
			}
		})
	}
}