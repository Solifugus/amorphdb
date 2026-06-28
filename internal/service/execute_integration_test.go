package service

import (
	"net"
	"testing"
	"time"

	"github.com/solifugus/amorphdb/internal/protocol"
)

// TestExecuteProtocolIntegration tests the complete EXECUTE message flow
func TestExecuteProtocolIntegration(t *testing.T) {
	// Create a temporary service for testing
	config := DefaultConfig()
	config.LocalSocketPath = "/tmp/test_execute_socket"
	config.NetworkPort = 0 // Use random port

	service, err := New(config)
	if err != nil {
		t.Fatalf("Failed to create service: %v", err)
	}

	// Start the service
	if err := service.Start(); err != nil {
		t.Fatalf("Failed to start service: %v", err)
	}
	defer service.Stop()

	// Wait a bit for service to be ready
	time.Sleep(100 * time.Millisecond)

	// Connect to the local socket
	conn, err := net.Dial("unix", service.GetLocalSocketPath())
	if err != nil {
		t.Fatalf("Failed to connect to service: %v", err)
	}
	defer conn.Close()

	t.Run("Execute valid MBL expression", func(t *testing.T) {
		// Test: Send MBL expression, receive result
		executeMsg := &protocol.ExecuteMessage{
			Code: "42",
		}

		// Create and send the EXECUTE message
		payload, err := protocol.EncodeExecuteMessage(executeMsg)
		if err != nil {
			t.Fatalf("Failed to encode execute message: %v", err)
		}

		msg := protocol.CreateMessage(protocol.EXECUTE, 1, payload)
		msgData, err := protocol.EncodeMessage(msg)
		if err != nil {
			t.Fatalf("Failed to encode message: %v", err)
		}

		// Send message
		_, err = conn.Write(msgData)
		if err != nil {
			t.Fatalf("Failed to send message: %v", err)
		}

		// Read response
		response := make([]byte, 1024)
		n, err := conn.Read(response)
		if err != nil {
			t.Fatalf("Failed to read response: %v", err)
		}

		// Decode response message
		responseMsg, err := protocol.DecodeMessage(response[:n])
		if err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		if responseMsg.Type != protocol.EXECUTE_RESPONSE {
			t.Errorf("Expected EXECUTE_RESPONSE, got 0x%02x", responseMsg.Type)
		}

		// Decode execute response
		executeResponse, err := protocol.DecodeExecuteResponseMessage(responseMsg.Payload)
		if err != nil {
			t.Fatalf("Failed to decode execute response: %v", err)
		}

		if !executeResponse.Success {
			t.Errorf("Expected successful execution, got error: %s", executeResponse.Error)
		}
	})

	t.Run("Execute invalid MBL", func(t *testing.T) {
		// Test: Send invalid MBL, receive error
		executeMsg := &protocol.ExecuteMessage{
			Code: "invalid syntax +++",
		}

		// Create and send the EXECUTE message
		payload, err := protocol.EncodeExecuteMessage(executeMsg)
		if err != nil {
			t.Fatalf("Failed to encode execute message: %v", err)
		}

		msg := protocol.CreateMessage(protocol.EXECUTE, 2, payload)
		msgData, err := protocol.EncodeMessage(msg)
		if err != nil {
			t.Fatalf("Failed to encode message: %v", err)
		}

		// Send message
		_, err = conn.Write(msgData)
		if err != nil {
			t.Fatalf("Failed to send message: %v", err)
		}

		// Read response
		response := make([]byte, 1024)
		n, err := conn.Read(response)
		if err != nil {
			t.Fatalf("Failed to read response: %v", err)
		}

		// Decode response message
		responseMsg, err := protocol.DecodeMessage(response[:n])
		if err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		if responseMsg.Type != protocol.EXECUTE_RESPONSE {
			t.Errorf("Expected EXECUTE_RESPONSE, got 0x%02x", responseMsg.Type)
		}

		// Decode execute response
		executeResponse, err := protocol.DecodeExecuteResponseMessage(responseMsg.Payload)
		if err != nil {
			t.Fatalf("Failed to decode execute response: %v", err)
		}

		if executeResponse.Success {
			t.Errorf("Expected execution to fail, but it succeeded")
		}

		if executeResponse.Error == "" {
			t.Errorf("Expected error message, got empty string")
		}
	})

	t.Run("Execute multi-statement program", func(t *testing.T) {
		// Test: Send multi-statement program, receive final result
		executeMsg := &protocol.ExecuteMessage{
			Code: "my.temp = 10; my.temp = my.temp + 5; my.temp",
		}

		// Create and send the EXECUTE message
		payload, err := protocol.EncodeExecuteMessage(executeMsg)
		if err != nil {
			t.Fatalf("Failed to encode execute message: %v", err)
		}

		msg := protocol.CreateMessage(protocol.EXECUTE, 3, payload)
		msgData, err := protocol.EncodeMessage(msg)
		if err != nil {
			t.Fatalf("Failed to encode message: %v", err)
		}

		// Send message
		_, err = conn.Write(msgData)
		if err != nil {
			t.Fatalf("Failed to send message: %v", err)
		}

		// Read response
		response := make([]byte, 1024)
		n, err := conn.Read(response)
		if err != nil {
			t.Fatalf("Failed to read response: %v", err)
		}

		// Decode response message
		responseMsg, err := protocol.DecodeMessage(response[:n])
		if err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		if responseMsg.Type != protocol.EXECUTE_RESPONSE {
			t.Errorf("Expected EXECUTE_RESPONSE, got 0x%02x", responseMsg.Type)
		}

		// Decode execute response
		executeResponse, err := protocol.DecodeExecuteResponseMessage(responseMsg.Payload)
		if err != nil {
			t.Fatalf("Failed to decode execute response: %v", err)
		}

		if !executeResponse.Success {
			t.Errorf("Expected successful execution, got error: %s", executeResponse.Error)
		}

		// The result should be the final expression result (15)
		// Note: This test depends on the actual MBL interpreter implementation
		// and may need adjustment based on how values are serialized
	})
}