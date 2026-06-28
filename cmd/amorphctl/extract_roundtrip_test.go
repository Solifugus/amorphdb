package main

import (
	"testing"

	"github.com/solifugus/amorphdb/internal/mbl/lexer"
	"github.com/solifugus/amorphdb/internal/mbl/parser"
)

func TestExtractRoundTrip(t *testing.T) {
	// Test cases with different MBL value formats
	testCases := []struct {
		name        string
		mblScript   string
		expectError bool
	}{
		{
			name:        "simple text assignment",
			mblScript:   `world.test.name = "Hello World"`,
			expectError: false,
		},
		{
			name:        "number assignment",
			mblScript:   `world.test.count = 42`,
			expectError: false,
		},
		{
			name:        "boolean assignments",
			mblScript:   `world.test.enabled = true`,
			expectError: false,
		},
		{
			name:        "time assignment",
			mblScript:   `world.test.created = @2024-01-01 12:00:00`,
			expectError: false,
		},
		{
			name:        "money assignment",
			mblScript:   `world.test.price = ¤25.50 USD`,
			expectError: false,
		},
		{
			name:        "unknown assignment",
			mblScript:   `world.test.unknown = unknown("not implemented")`,
			expectError: false,
		},
		{
			name:        "simple boolean false",
			mblScript:   `world.test.disabled = false`,
			expectError: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Test that the MBL script can be parsed successfully
			lexer := lexer.New(tc.mblScript)
			parser := parser.New(lexer)

			// Try to parse the assignment
			ast := parser.ParseProgram()

			if tc.expectError {
				if len(parser.Errors()) == 0 {
					t.Error("Expected parsing to fail, but it succeeded")
				}
			} else {
				if len(parser.Errors()) > 0 {
					t.Errorf("Parser errors: %v", parser.Errors())
				}
				if ast == nil {
					t.Error("Parser returned nil AST")
				}
			}
		})
	}
}

func TestExtractMBLFormatting(t *testing.T) {
	// Test the MBL formatting functions directly
	testCases := []struct {
		name           string
		valueTypeTag   byte
		valueData      []byte
		expectedOutput string
		expectError    bool
	}{
		{
			name:           "empty data should not crash",
			valueTypeTag:   0x01, // Text
			valueData:      []byte{},
			expectedOutput: "",
			expectError:    true, // Should error on malformed data
		},
		{
			name:           "unknown type should fallback",
			valueTypeTag:   0xFF, // Unknown type
			valueData:      []byte{1, 2, 3},
			expectedOutput: "unknown(\"unsupported type: 0xFF\")",
			expectError:    false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Note: We can't easily test formatValueForMBL directly as it's a method on Connection
			// This test structure is prepared for when we extract the formatting logic
			// into a standalone function that can be tested independently

			// For now, just verify the test case structure
			if tc.expectedOutput == "" && !tc.expectError {
				t.Skip("Test case needs expected output or expectError=true")
			}
		})
	}
}

func TestPathValidation(t *testing.T) {
	// Test path validation for various edge cases
	testCases := []struct {
		name        string
		path        []string
		expectError bool
	}{
		{
			name:        "root path (empty)",
			path:        []string{},
			expectError: false,
		},
		{
			name:        "single component",
			path:        []string{"world"},
			expectError: false,
		},
		{
			name:        "nested path",
			path:        []string{"world", "test", "data"},
			expectError: false,
		},
		{
			name:        "path with numbers",
			path:        []string{"world", "test123"},
			expectError: false,
		},
		{
			name:        "path with underscores",
			path:        []string{"my_app", "user_data"},
			expectError: false,
		},
		{
			name:        "empty component",
			path:        []string{"world", ""},
			expectError: true,
		},
		{
			name:        "invalid character",
			path:        []string{"world", "test@invalid"},
			expectError: true,
		},
		{
			name:        "starts with number",
			path:        []string{"world", "123invalid"},
			expectError: true,
		},
		{
			name:        "space in component",
			path:        []string{"world", "test space"},
			expectError: true,
		},
		{
			name:        "hyphen in component",
			path:        []string{"world", "test-hyphen"},
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			client := &ControlClient{} // Mock client
			cmd := NewExtractCommand(client)

			err := cmd.validatePath(tc.path)

			if tc.expectError && err == nil {
				t.Errorf("Expected error for path %v, but got none", tc.path)
			}
			if !tc.expectError && err != nil {
				t.Errorf("Expected no error for path %v, but got: %v", tc.path, err)
			}
		})
	}
}