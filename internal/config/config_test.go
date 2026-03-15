package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	// Check that defaults are reasonable
	if config.Mesh.Name != "" {
		t.Errorf("Expected empty mesh name for standalone mode, got: %s", config.Mesh.Name)
	}

	if config.Data.StorageDir == "" {
		t.Error("Expected storage directory to be set")
	}

	if config.Network.Port != 5000 {
		t.Errorf("Expected default port 5000, got: %d", config.Network.Port)
	}

	if config.Network.LocalSocketPath == "" {
		t.Error("Expected socket path to be set")
	}

	if config.Mesh.Bridges == nil {
		t.Error("Expected bridges map to be initialized")
	}
}

func TestValidateMeshName(t *testing.T) {
	testCases := []struct {
		name    string
		wantErr bool
		desc    string
	}{
		{"", true, "empty name"},
		{"a", true, "too short"},
		{"valid-mesh", false, "valid name with hyphen"},
		{"valid_mesh", false, "valid name with underscore"},
		{"ValidMesh123", false, "valid alphanumeric name"},
		{"invalid mesh", true, "contains space"},
		{"invalid@mesh", true, "contains @ symbol"},
		{"invalid.mesh", true, "contains dot"},
		{"invalid!mesh", true, "contains special character"},
		{"x", true, "too short"},
		{string(make([]byte, 65)), true, "too long"},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			err := ValidateMeshName(tc.name)
			if (err != nil) != tc.wantErr {
				t.Errorf("ValidateMeshName(%q) error = %v, wantErr %v", tc.name, err, tc.wantErr)
			}
		})
	}
}

func TestValidateConfig(t *testing.T) {
	t.Run("valid config", func(t *testing.T) {
		config := DefaultConfig()
		config.Mesh.Name = "test-mesh"
		config.Mesh.Bridges["partner"] = "192.168.1.10:5000"

		if err := ValidateConfig(config); err != nil {
			t.Errorf("Expected valid config to pass validation, got: %v", err)
		}
	})

	t.Run("invalid mesh name", func(t *testing.T) {
		config := DefaultConfig()
		config.Mesh.Name = "invalid mesh name"

		if err := ValidateConfig(config); err == nil {
			t.Error("Expected invalid mesh name to fail validation")
		}
	})

	t.Run("invalid bridge name", func(t *testing.T) {
		config := DefaultConfig()
		config.Mesh.Bridges["invalid@bridge"] = "192.168.1.10:5000"

		if err := ValidateConfig(config); err == nil {
			t.Error("Expected invalid bridge name to fail validation")
		}
	})

	t.Run("invalid port", func(t *testing.T) {
		config := DefaultConfig()
		config.Network.Port = 80 // Too low

		if err := ValidateConfig(config); err == nil {
			t.Error("Expected invalid port to fail validation")
		}
	})
}

func TestIsStandalone(t *testing.T) {
	t.Run("standalone mode", func(t *testing.T) {
		config := DefaultConfig()
		if !config.IsStandalone() {
			t.Error("Expected default config to be standalone")
		}
	})

	t.Run("mesh mode", func(t *testing.T) {
		config := DefaultConfig()
		config.Mesh.Name = "test-mesh"
		if config.IsStandalone() {
			t.Error("Expected config with mesh name to not be standalone")
		}
	})
}

func TestLoadSaveConfig(t *testing.T) {
	// Create temporary directory
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test_config.yaml")

	// Create test config
	originalConfig := DefaultConfig()
	originalConfig.Mesh.Name = "test-mesh"
	originalConfig.Mesh.Identity = "test-node-123"
	originalConfig.Mesh.Bridges["partner"] = "192.168.1.10:5000"
	originalConfig.Data.StorageDir = "/tmp/test"
	originalConfig.Network.Port = 6000

	// Save config
	if err := SaveConfig(originalConfig, configPath); err != nil {
		t.Fatalf("Failed to save config: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Fatal("Config file was not created")
	}

	// Load config back
	loadedConfig, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Verify values match
	if loadedConfig.Mesh.Name != originalConfig.Mesh.Name {
		t.Errorf("Mesh name mismatch: got %q, want %q", loadedConfig.Mesh.Name, originalConfig.Mesh.Name)
	}

	if loadedConfig.Mesh.Identity != originalConfig.Mesh.Identity {
		t.Errorf("Identity mismatch: got %q, want %q", loadedConfig.Mesh.Identity, originalConfig.Mesh.Identity)
	}

	if loadedConfig.Data.StorageDir != originalConfig.Data.StorageDir {
		t.Errorf("Storage dir mismatch: got %q, want %q", loadedConfig.Data.StorageDir, originalConfig.Data.StorageDir)
	}

	if loadedConfig.Network.Port != originalConfig.Network.Port {
		t.Errorf("Port mismatch: got %d, want %d", loadedConfig.Network.Port, originalConfig.Network.Port)
	}

	// Check bridge was loaded correctly
	bridgeAddr, exists := loadedConfig.Mesh.Bridges["partner"]
	if !exists {
		t.Error("Bridge 'partner' was not loaded")
	} else if bridgeAddr != "192.168.1.10:5000" {
		t.Errorf("Bridge address mismatch: got %q, want %q", bridgeAddr, "192.168.1.10:5000")
	}
}

func TestLoadConfigNonExistentFile(t *testing.T) {
	// Loading non-existent config should return defaults without error
	config, err := LoadConfig("/nonexistent/path/config.yaml")
	if err != nil {
		t.Fatalf("Expected no error for non-existent config file, got: %v", err)
	}

	// Should be default config
	defaultConfig := DefaultConfig()
	if config.Network.Port != defaultConfig.Network.Port {
		t.Errorf("Expected default port, got: %d", config.Network.Port)
	}
}

func TestSaveConfigInvalidConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "invalid_config.yaml")

	// Create invalid config
	config := DefaultConfig()
	config.Mesh.Name = "invalid mesh name" // Contains space

	// Save should fail due to validation
	if err := SaveConfig(config, configPath); err == nil {
		t.Error("Expected save to fail with invalid config")
	}
}

func TestGetConfigPath(t *testing.T) {
	path := GetConfigPath()
	if path == "" {
		t.Error("Expected non-empty config path")
	}

	// Should end with config.yaml
	if !filepath.IsAbs(path) {
		t.Error("Expected absolute path")
	}
}