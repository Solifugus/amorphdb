// Package main provides bridge command testing for amorphctl
package main

import (
	"encoding/json"
	"testing"
	"time"
)

func TestBridgeCommand_CreateBridge_Success(t *testing.T) {
	mockClient := NewMockControlClient()

	// Set up successful bridge response
	successResponse := CreateBridgeResponse{
		Success: true,
		Message: "Successfully created bridge to mesh 'partner-mesh'",
		BridgeInfo: &BridgeInfo{
			TargetMeshName: "partner-mesh",
			BridgeIdentity: "bridge-node123-partner-mesh-1a2b",
			Status:         "connected",
			ConnectedAt:    &time.Time{},
		},
	}
	responseData, _ := json.Marshal(successResponse)
	mockClient.SetResponse("create-bridge", responseData)

	bridgeCommand := NewBridgeCommand(mockClient)

	err := bridgeCommand.CreateBridge("192.168.1.10:5000")
	if err != nil {
		t.Fatalf("CreateBridge failed: %v", err)
	}

	commands := mockClient.GetCommands()
	if len(commands) != 1 || commands[0] != "create-bridge" {
		t.Errorf("Expected one 'create-bridge' command, got: %v", commands)
	}
}

func TestBridgeCommand_CreateBridge_Failure(t *testing.T) {
	mockClient := NewMockControlClient()

	// Set up failure response
	failureResponse := CreateBridgeResponse{
		Success: false,
		Message: "Target mesh not reachable or bridge already exists",
	}
	responseData, _ := json.Marshal(failureResponse)
	mockClient.SetResponse("create-bridge", responseData)

	bridgeCommand := NewBridgeCommand(mockClient)

	err := bridgeCommand.CreateBridge("192.168.1.10:5000")
	if err == nil {
		t.Error("Expected error for failed bridge creation")
	}

	if err.Error() != "bridge creation failed" {
		t.Errorf("Expected 'bridge creation failed', got: %s", err.Error())
	}
}

func TestBridgeCommand_ValidateTargetAddress(t *testing.T) {
	mockClient := NewMockControlClient()
	bridgeCommand := NewBridgeCommand(mockClient)

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
			err := bridgeCommand.validateTargetAddress(addr)
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
			err := bridgeCommand.validateTargetAddress(addr)
			if err == nil {
				t.Errorf("Expected invalid address %q to fail validation", addr)
			}
		})
	}
}

func TestBridgeCommand_CreateBridge_InvalidAddress(t *testing.T) {
	mockClient := NewMockControlClient()
	bridgeCommand := NewBridgeCommand(mockClient)

	invalidAddresses := []string{
		"",
		"invalid:address:",
		":empty-host",
	}

	for _, addr := range invalidAddresses {
		t.Run("invalid_"+addr, func(t *testing.T) {
			err := bridgeCommand.CreateBridge(addr)
			if err == nil {
				t.Errorf("Expected error for invalid address %q", addr)
			}

			// Should not have sent command to server
			commands := mockClient.GetCommands()
			if len(commands) > 0 {
				t.Error("Should not send command for invalid address")
			}
		})
	}
}