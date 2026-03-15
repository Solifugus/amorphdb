// Package types provides path handling tests
package types

import (
	"testing"
)

func TestPathValidator_IsValidMeshName(t *testing.T) {
	validator := NewPathValidator()

	tests := []struct {
		name     string
		meshName string
		expected bool
	}{
		// Valid names
		{"simple name", "testmesh", true},
		{"with hyphen", "test-mesh", true},
		{"with underscore", "test_mesh", true},
		{"alphanumeric", "mesh123", true},
		{"mixed valid", "my-test_mesh2", true},
		{"minimal length", "ab", true},
		{"maximum length", "a23456789012345678901234567890123456789012345678901234567890123", true},

		// Invalid names
		{"empty", "", false},
		{"single char", "a", false},
		{"too long", "a234567890123456789012345678901234567890123456789012345678901234", false},
		{"starts with hyphen", "-testmesh", false},
		{"ends with hyphen", "testmesh-", false},
		{"starts with underscore", "_testmesh", false},
		{"ends with underscore", "testmesh_", false},
		{"special chars", "test@mesh", false},
		{"spaces", "test mesh", false},
		{"dots", "test.mesh", false},
		{"reserved standalone", "standalone", false},
		{"reserved local", "local", false},
		{"reserved world", "world", false},
		{"reserved system", "system", false},
		{"reserved test", "test", false},
		{"reserved case insensitive", "SYSTEM", false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := validator.IsValidMeshName(test.meshName)
			if result != test.expected {
				t.Errorf("IsValidMeshName(%q) = %v, expected %v",
					test.meshName, result, test.expected)
			}
		})
	}
}

func TestPathValidator_ParsePath(t *testing.T) {
	validator := NewPathValidator()

	tests := []struct {
		name         string
		path         []string
		expectedType PathType
		expectedMesh string
		expectErr    bool
	}{
		// Bridge paths
		{"valid bridge path", []string{"my", "testmesh", "data"}, BridgePath, "testmesh", false},
		{"minimal bridge path", []string{"my", "testmesh"}, BridgePath, "testmesh", false},
		{"bridge path invalid mesh", []string{"my", "test@mesh"}, InvalidPath, "", true},
		{"bridge path too short", []string{"my"}, InvalidPath, "", true},

		// World paths
		{"world path", []string{"world", "agent", "123"}, WorldPath, "", false},
		{"minimal world path", []string{"world"}, WorldPath, "", false},

		// Local paths
		{"local path", []string{"users", "john", "data"}, LocalPath, "", false},
		{"single component", []string{"data"}, LocalPath, "", false},

		// Invalid paths
		{"empty path", []string{}, InvalidPath, "", true},
		{"reserved component", []string{"my", "testmesh"}, BridgePath, "testmesh", false}, // my is valid in bridge context
		{"local path with reserved", []string{"world"}, WorldPath, "", false}, // world is valid as world path
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			parsed := validator.ParsePath(test.path)

			if test.expectErr && parsed.Error == nil {
				t.Errorf("Expected error for %s, but got none", test.name)
				return
			}

			if !test.expectErr && parsed.Error != nil {
				t.Errorf("Expected no error for %s, but got: %v", test.name, parsed.Error)
				return
			}

			if parsed.Type != test.expectedType {
				t.Errorf("Expected type %v, got %v", test.expectedType, parsed.Type)
			}

			if test.expectedType == BridgePath && parsed.MeshName != test.expectedMesh {
				t.Errorf("Expected mesh name %q, got %q", test.expectedMesh, parsed.MeshName)
			}
		})
	}
}

func TestPathValidator_ConvertBridgePathToAgentPath(t *testing.T) {
	validator := NewPathValidator()

	tests := []struct {
		name             string
		bridgePath       []string
		bridgeIdentityID string
		expectedPath     []string
		expectErr        bool
	}{
		{
			name:             "full bridge path",
			bridgePath:       []string{"my", "testmesh", "users", "john"},
			bridgeIdentityID: "bridge-id-123",
			expectedPath:     []string{"world", "agent", "bridge-id-123", "users", "john"},
			expectErr:        false,
		},
		{
			name:             "minimal bridge path",
			bridgePath:       []string{"my", "testmesh"},
			bridgeIdentityID: "bridge-id-456",
			expectedPath:     []string{"world", "agent", "bridge-id-456"},
			expectErr:        false,
		},
		{
			name:         "not a bridge path",
			bridgePath:   []string{"local", "data"},
			expectErr:    true,
		},
		{
			name:         "invalid bridge path",
			bridgePath:   []string{"my"},
			expectErr:    true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			agentPath, err := validator.ConvertBridgePathToAgentPath(test.bridgePath, test.bridgeIdentityID)

			if test.expectErr {
				if err == nil {
					t.Errorf("Expected error for %s, but got none", test.name)
				}
				return
			}

			if err != nil {
				t.Errorf("Expected no error for %s, but got: %v", test.name, err)
				return
			}

			if !equalStringSlices(agentPath, test.expectedPath) {
				t.Errorf("Expected agent path %v, got %v", test.expectedPath, agentPath)
			}
		})
	}
}

func TestPathValidator_GetMeshNameFromBridgePath(t *testing.T) {
	validator := NewPathValidator()

	tests := []struct {
		name         string
		path         []string
		expectedMesh string
		expectErr    bool
	}{
		{
			name:         "valid bridge path",
			path:         []string{"my", "testmesh", "data"},
			expectedMesh: "testmesh",
			expectErr:    false,
		},
		{
			name:         "minimal bridge path",
			path:         []string{"my", "anothermesh"},
			expectedMesh: "anothermesh",
			expectErr:    false,
		},
		{
			name:      "not a bridge path",
			path:      []string{"local", "data"},
			expectErr: true,
		},
		{
			name:      "invalid bridge path",
			path:      []string{"my"},
			expectErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			meshName, err := validator.GetMeshNameFromBridgePath(test.path)

			if test.expectErr {
				if err == nil {
					t.Errorf("Expected error for %s, but got none", test.name)
				}
				return
			}

			if err != nil {
				t.Errorf("Expected no error for %s, but got: %v", test.name, err)
				return
			}

			if meshName != test.expectedMesh {
				t.Errorf("Expected mesh name %q, got %q", test.expectedMesh, meshName)
			}
		})
	}
}

func TestPathValidator_PathTypeCheckers(t *testing.T) {
	validator := NewPathValidator()

	tests := []struct {
		path          []string
		isBridge      bool
		isLocal       bool
		isWorld       bool
	}{
		{[]string{"my", "testmesh", "data"}, true, false, false},
		{[]string{"world", "agent", "123"}, false, false, true},
		{[]string{"users", "john"}, false, true, false},
		{[]string{"my"}, false, false, false}, // invalid
		{[]string{}, false, false, false},     // invalid
	}

	for i, test := range tests {
		t.Run(FormatPath(test.path), func(t *testing.T) {
			if validator.IsBridgePath(test.path) != test.isBridge {
				t.Errorf("Test %d: IsBridgePath() = %v, expected %v",
					i, validator.IsBridgePath(test.path), test.isBridge)
			}

			if validator.IsLocalPath(test.path) != test.isLocal {
				t.Errorf("Test %d: IsLocalPath() = %v, expected %v",
					i, validator.IsLocalPath(test.path), test.isLocal)
			}

			if validator.IsWorldPath(test.path) != test.isWorld {
				t.Errorf("Test %d: IsWorldPath() = %v, expected %v",
					i, validator.IsWorldPath(test.path), test.isWorld)
			}
		})
	}
}

func TestPathValidator_NormalizePath(t *testing.T) {
	validator := NewPathValidator()

	tests := []struct {
		name     string
		input    []string
		expected []string
	}{
		{
			name:     "basic normalization",
			input:    []string{"Users", "John", "Data"},
			expected: []string{"users", "john", "data"},
		},
		{
			name:     "with whitespace",
			input:    []string{" my ", " testmesh ", " data "},
			expected: []string{"my", "testmesh", "data"},
		},
		{
			name:     "with empty components",
			input:    []string{"my", "", "testmesh", "data"},
			expected: []string{"my", "testmesh", "data"},
		},
		{
			name:     "identity preservation",
			input:    []string{"world", "agent", "bridge-ID-123ABC"},
			expected: []string{"world", "agent", "bridge-ID-123ABC"}, // Identity should be preserved
		},
		{
			name:     "mixed case with identities",
			input:    []string{"World", "Agent", "a1b2c3d4-e5f6"},
			expected: []string{"world", "agent", "a1b2c3d4-e5f6"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := validator.NormalizePath(test.input)
			if !equalStringSlices(result, test.expected) {
				t.Errorf("Expected %v, got %v", test.expected, result)
			}
		})
	}
}

func TestFormatPath(t *testing.T) {
	tests := []struct {
		path     []string
		expected string
	}{
		{[]string{"my", "testmesh", "data"}, "my.testmesh.data"},
		{[]string{"world", "agent", "123"}, "world.agent.123"},
		{[]string{"single"}, "single"},
		{[]string{}, ""},
	}

	for _, test := range tests {
		t.Run(test.expected, func(t *testing.T) {
			result := FormatPath(test.path)
			if result != test.expected {
				t.Errorf("Expected %q, got %q", test.expected, result)
			}
		})
	}
}

func TestParsePathString(t *testing.T) {
	tests := []struct {
		input    string
		expected []string
	}{
		{"my.testmesh.data", []string{"my", "testmesh", "data"}},
		{"world.agent.123", []string{"world", "agent", "123"}},
		{"single", []string{"single"}},
		{"", []string{}},
		{"a.b.c.d.e", []string{"a", "b", "c", "d", "e"}},
	}

	for _, test := range tests {
		t.Run(test.input, func(t *testing.T) {
			result := ParsePathString(test.input)
			if !equalStringSlices(result, test.expected) {
				t.Errorf("Expected %v, got %v", test.expected, result)
			}
		})
	}
}

// Helper function to compare string slices
func equalStringSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}