package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/solifugus/amorphdb/internal/protocol"
)

func TestExtractIntegration(t *testing.T) {
	// Create a test scenario with various data types
	testCases := []struct {
		name        string
		setupData   map[string]interface{}
		extractPath string
		checkOutput func(t *testing.T, script string)
	}{
		{
			name: "basic text values",
			setupData: map[string]interface{}{
				"world.test.name":        "Hello World",
				"world.test.description": "A test value",
			},
			extractPath: "world.test",
			checkOutput: func(t *testing.T, script string) {
				if !strings.Contains(script, "world.test.name = \"Hello World\"") {
					t.Error("Expected text assignment for name")
				}
				if !strings.Contains(script, "world.test.description = \"A test value\"") {
					t.Error("Expected text assignment for description")
				}
			},
		},
		{
			name: "mixed data types",
			setupData: map[string]interface{}{
				"world.app.version":  "1.2.3",
				"world.app.count":    42.0,
				"world.app.enabled":  true,
				"world.app.disabled": false,
			},
			extractPath: "world.app",
			checkOutput: func(t *testing.T, script string) {
				if !strings.Contains(script, "world.app.version = \"1.2.3\"") {
					t.Error("Expected text assignment for version")
				}
				if !strings.Contains(script, "world.app.count = 42") {
					t.Error("Expected number assignment for count")
				}
				if !strings.Contains(script, "world.app.enabled = true") {
					t.Error("Expected boolean assignment for enabled")
				}
				if !strings.Contains(script, "world.app.disabled = false") {
					t.Error("Expected boolean assignment for disabled")
				}
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Skip("Integration test requires running AmorphDB service - skipping for unit tests")
			// This is a placeholder for future integration testing when service is available
		})
	}
}

func TestExtractCommand_FileOutput(t *testing.T) {
	// Test writing to file functionality
	client := &ControlClient{} // Mock client for testing
	cmd := NewExtractCommand(client)

	// Create a temporary file
	tempDir := os.TempDir()
	outputFile := filepath.Join(tempDir, "test_extract.mbl")

	// Test script content
	script := "# Test script\nworld.test = \"hello\""

	// Test writing to file
	err := cmd.writeToFile(outputFile, script)
	if err != nil {
		t.Fatalf("Failed to write to file: %v", err)
	}

	// Verify file was created and has correct content
	content, err := os.ReadFile(outputFile)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	if string(content) != script {
		t.Errorf("File content mismatch.\nExpected: %s\nGot: %s", script, string(content))
	}

	// Clean up
	os.Remove(outputFile)
}

func TestExtractCommand_HeaderGeneration(t *testing.T) {
	client := &ControlClient{} // Mock client for testing
	cmd := NewExtractCommand(client)

	pathStr := "world.test.data"
	header := cmd.generateHeader(pathStr)

	// Check header format and content
	lines := strings.Split(header, "\n")

	// Should start with comment
	if !strings.HasPrefix(lines[0], "#") {
		t.Error("Header should start with comment")
	}

	// Should contain source path
	found := false
	for _, line := range lines {
		if strings.Contains(line, "Source Path: "+pathStr) {
			found = true
			break
		}
	}
	if !found {
		t.Error("Header should contain source path")
	}

	// Should contain extraction timestamp
	found = false
	for _, line := range lines {
		if strings.Contains(line, "Extracted:") {
			found = true
			break
		}
	}
	if !found {
		t.Error("Header should contain extraction timestamp")
	}

	// Should contain agent information
	found = false
	for _, line := range lines {
		if strings.Contains(line, "Agent:") {
			found = true
			break
		}
	}
	if !found {
		t.Error("Header should contain agent information")
	}
}

func TestProtocolMessages_ExtractRoundTrip(t *testing.T) {
	// Test protocol message encoding/decoding

	// Test ExtractMessage
	originalExtract := &protocol.ExtractMessage{
		Path: []string{"world", "test", "data"},
	}

	// Encode
	payload, err := protocol.EncodeExtractMessage(originalExtract)
	if err != nil {
		t.Fatalf("Failed to encode ExtractMessage: %v", err)
	}

	// Decode
	decodedExtract, err := protocol.DecodeExtractMessage(payload)
	if err != nil {
		t.Fatalf("Failed to decode ExtractMessage: %v", err)
	}

	// Verify path matches
	if len(decodedExtract.Path) != len(originalExtract.Path) {
		t.Fatalf("Path length mismatch: expected %d, got %d", len(originalExtract.Path), len(decodedExtract.Path))
	}

	for i, component := range originalExtract.Path {
		if decodedExtract.Path[i] != component {
			t.Errorf("Path component %d mismatch: expected %s, got %s", i, component, decodedExtract.Path[i])
		}
	}

	// Test ExtractResponseMessage - Success case
	originalResponse := &protocol.ExtractResponseMessage{
		Success: true,
		Script:  "world.test = \"hello\"",
		Error:   "",
	}

	// Encode
	responsePayload, err := protocol.EncodeExtractResponseMessage(originalResponse)
	if err != nil {
		t.Fatalf("Failed to encode ExtractResponseMessage: %v", err)
	}

	// Decode
	decodedResponse, err := protocol.DecodeExtractResponseMessage(responsePayload)
	if err != nil {
		t.Fatalf("Failed to decode ExtractResponseMessage: %v", err)
	}

	// Verify fields match
	if decodedResponse.Success != originalResponse.Success {
		t.Errorf("Success mismatch: expected %t, got %t", originalResponse.Success, decodedResponse.Success)
	}
	if decodedResponse.Script != originalResponse.Script {
		t.Errorf("Script mismatch: expected %s, got %s", originalResponse.Script, decodedResponse.Script)
	}
	if decodedResponse.Error != originalResponse.Error {
		t.Errorf("Error mismatch: expected %s, got %s", originalResponse.Error, decodedResponse.Error)
	}

	// Test ExtractResponseMessage - Error case
	originalError := &protocol.ExtractResponseMessage{
		Success: false,
		Script:  "",
		Error:   "Path not found",
	}

	// Encode
	errorPayload, err := protocol.EncodeExtractResponseMessage(originalError)
	if err != nil {
		t.Fatalf("Failed to encode error ExtractResponseMessage: %v", err)
	}

	// Decode
	decodedError, err := protocol.DecodeExtractResponseMessage(errorPayload)
	if err != nil {
		t.Fatalf("Failed to decode error ExtractResponseMessage: %v", err)
	}

	// Verify fields match
	if decodedError.Success != originalError.Success {
		t.Errorf("Success mismatch: expected %t, got %t", originalError.Success, decodedError.Success)
	}
	if decodedError.Error != originalError.Error {
		t.Errorf("Error mismatch: expected %s, got %s", originalError.Error, decodedError.Error)
	}
}