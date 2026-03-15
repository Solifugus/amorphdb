// Package config handles AmorphDB configuration management
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"gopkg.in/yaml.v3"
)

// MeshConfig contains mesh-specific configuration
type MeshConfig struct {
	Name     string            `yaml:"name"`      // Mesh name (empty = standalone)
	Bridges  map[string]string `yaml:"bridges"`   // Bridge connections (mesh_name -> address)
	Identity string            `yaml:"identity"`  // Node identity
}

// DataConfig contains data storage configuration
type DataConfig struct {
	StorageDir string `yaml:"storage_dir"` // Data storage directory
}

// NetworkConfig contains network configuration
type NetworkConfig struct {
	LocalSocketPath string `yaml:"local_socket_path"` // UNIX socket path
	Port            int    `yaml:"port"`              // TCP port
}

// Config represents the complete AmorphDB configuration
type Config struct {
	Mesh    MeshConfig    `yaml:"mesh"`
	Data    DataConfig    `yaml:"data"`
	Network NetworkConfig `yaml:"network"`
}

// DefaultConfig returns default configuration values
func DefaultConfig() Config {
	homeDir, _ := os.UserHomeDir()
	amorphDir := filepath.Join(homeDir, ".amorph")

	return Config{
		Mesh: MeshConfig{
			Name:     "", // Empty = standalone mode
			Bridges:  make(map[string]string),
			Identity: "", // Will be auto-generated if empty
		},
		Data: DataConfig{
			StorageDir: filepath.Join(amorphDir, "data"),
		},
		Network: NetworkConfig{
			LocalSocketPath: filepath.Join(amorphDir, "socket"),
			Port:            5000,
		},
	}
}

// LoadConfig loads configuration from a YAML file
func LoadConfig(path string) (Config, error) {
	// Start with defaults
	config := DefaultConfig()

	// Check if config file exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		// No config file, return defaults
		return config, nil
	}

	// Read config file
	data, err := os.ReadFile(path)
	if err != nil {
		return config, fmt.Errorf("failed to read config file: %w", err)
	}

	// Parse YAML
	if err := yaml.Unmarshal(data, &config); err != nil {
		return config, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Validate configuration
	if err := ValidateConfig(config); err != nil {
		return config, fmt.Errorf("invalid configuration: %w", err)
	}

	return config, nil
}

// SaveConfig saves configuration to a YAML file
func SaveConfig(config Config, path string) error {
	// Validate configuration before saving
	if err := ValidateConfig(config); err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Convert to YAML
	data, err := yaml.Marshal(&config)
	if err != nil {
		return fmt.Errorf("failed to serialize config: %w", err)
	}

	// Write to file
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// ValidateConfig validates configuration values
func ValidateConfig(config Config) error {
	// Validate mesh name if provided
	if config.Mesh.Name != "" {
		if err := ValidateMeshName(config.Mesh.Name); err != nil {
			return fmt.Errorf("invalid mesh name: %w", err)
		}
	}

	// Validate bridge names
	for meshName := range config.Mesh.Bridges {
		if err := ValidateMeshName(meshName); err != nil {
			return fmt.Errorf("invalid bridge mesh name '%s': %w", meshName, err)
		}
	}

	// Validate network port
	if config.Network.Port < 1024 || config.Network.Port > 65535 {
		return fmt.Errorf("invalid network port %d: must be between 1024-65535", config.Network.Port)
	}

	return nil
}

// ValidateMeshName validates that a mesh name follows naming rules
func ValidateMeshName(name string) error {
	if name == "" {
		return fmt.Errorf("mesh name cannot be empty")
	}

	// Must be alphanumeric with hyphens/underscores only
	// No spaces, @ symbols, or special characters to avoid parsing conflicts
	validName := regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)
	if !validName.MatchString(name) {
		return fmt.Errorf("mesh name '%s' must contain only alphanumeric characters, hyphens, and underscores", name)
	}

	// Reasonable length limits
	if len(name) < 2 || len(name) > 64 {
		return fmt.Errorf("mesh name '%s' must be 2-64 characters long", name)
	}

	return nil
}

// IsStandalone returns true if the node is in standalone mode (no mesh)
func (c Config) IsStandalone() bool {
	return c.Mesh.Name == ""
}

// GetConfigPath returns the default configuration file path
func GetConfigPath() string {
	homeDir, _ := os.UserHomeDir()
	return filepath.Join(homeDir, ".amorph", "config.yaml")
}