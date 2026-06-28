package main

import (
	"testing"
)

func TestExtractCommand_ValidatePath(t *testing.T) {
	client := &ControlClient{} // Mock client for testing
	cmd := NewExtractCommand(client)

	testCases := []struct {
		name        string
		path        []string
		expectError bool
	}{
		{
			name:        "empty path",
			path:        []string{},
			expectError: false,
		},
		{
			name:        "valid single component",
			path:        []string{"world"},
			expectError: false,
		},
		{
			name:        "valid multi-component path",
			path:        []string{"world", "knowledgefoyer", "config"},
			expectError: false,
		},
		{
			name:        "valid path with underscores",
			path:        []string{"my_app", "user_data"},
			expectError: false,
		},
		{
			name:        "invalid empty component",
			path:        []string{"world", "", "config"},
			expectError: true,
		},
		{
			name:        "invalid component starting with number",
			path:        []string{"world", "123invalid"},
			expectError: true,
		},
		{
			name:        "invalid component with special characters",
			path:        []string{"world", "invalid@component"},
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
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

func TestIsValidMBLIdentifier(t *testing.T) {
	testCases := []struct {
		name       string
		identifier string
		valid      bool
	}{
		{
			name:       "valid simple identifier",
			identifier: "world",
			valid:      true,
		},
		{
			name:       "valid with numbers",
			identifier: "config123",
			valid:      true,
		},
		{
			name:       "valid with underscores",
			identifier: "my_app",
			valid:      true,
		},
		{
			name:       "valid starting with underscore",
			identifier: "_private",
			valid:      true,
		},
		{
			name:       "invalid empty string",
			identifier: "",
			valid:      false,
		},
		{
			name:       "invalid starting with number",
			identifier: "123invalid",
			valid:      false,
		},
		{
			name:       "invalid with special characters",
			identifier: "invalid@char",
			valid:      false,
		},
		{
			name:       "invalid with spaces",
			identifier: "invalid space",
			valid:      false,
		},
		{
			name:       "invalid with hyphen",
			identifier: "invalid-hyphen",
			valid:      false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := isValidMBLIdentifier(tc.identifier)
			if result != tc.valid {
				t.Errorf("Expected isValidMBLIdentifier(%q) = %v, got %v", tc.identifier, tc.valid, result)
			}
		})
	}
}

func TestExtractCommand_GenerateHeader(t *testing.T) {
	client := &ControlClient{} // Mock client for testing
	cmd := NewExtractCommand(client)

	pathStr := "world.knowledgefoyer.config"
	header := cmd.generateHeader(pathStr)

	// Check that header contains expected elements
	if header == "" {
		t.Error("Generated header should not be empty")
	}

	// Check for expected content
	expectedParts := []string{
		"AmorphDB Data Extract",
		"Source Path: " + pathStr,
		"amorphctl extract",
	}

	for _, part := range expectedParts {
		if !containsString(header, part) {
			t.Errorf("Header should contain '%s', but got:\n%s", part, header)
		}
	}
}

// Helper function to check if a string contains a substring
func containsString(s, substr string) bool {
	return len(s) >= len(substr) && containsSubstring(s, substr)
}

func containsSubstring(s, substr string) bool {
	if len(substr) == 0 {
		return true
	}
	if len(s) < len(substr) {
		return false
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}