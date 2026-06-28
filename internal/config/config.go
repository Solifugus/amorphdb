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
	Name                       string            `yaml:"name"`                         // Mesh name (empty = standalone)
	Bridges                    map[string]string `yaml:"bridges"`                      // Bridge connections (mesh_name -> address)
	Identity                   string            `yaml:"identity"`                     // Node identity
	UseSubscriptionReplication bool              `yaml:"use_subscription_replication"` // Use subscription-based replication (default: true)
}

// DataConfig contains data storage configuration
type DataConfig struct {
	StorageDir string `yaml:"storage_dir"` // Data storage directory
}

// TLSConfig contains TLS certificate configuration for a domain
type TLSConfig struct {
	CertFile string `yaml:"cert_file"` // Path to certificate file
	KeyFile  string `yaml:"key_file"`  // Path to private key file
}

// NetworkConfig contains network configuration
type NetworkConfig struct {
	LocalSocketPath string               `yaml:"local_socket_path"` // UNIX socket path
	Port            int                  `yaml:"port"`              // TCP port
	HTTPPort        int                  `yaml:"http_port"`         // HTTP port (redirects to HTTPS)
	HTTPSPort       int                  `yaml:"https_port"`        // HTTPS port
	TLS             map[string]TLSConfig `yaml:"tls"`               // TLS configuration per domain
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
			Name:                       "", // Empty = standalone mode
			Bridges:                    make(map[string]string),
			Identity:                   "", // Will be auto-generated if empty
			UseSubscriptionReplication: true, // Default to new subscription model
		},
		Data: DataConfig{
			StorageDir: filepath.Join(amorphDir, "data"),
		},
		Network: NetworkConfig{
			LocalSocketPath: filepath.Join(amorphDir, "socket"),
			Port:            5000,
			HTTPPort:        8080,  // Default HTTP port (will redirect to HTTPS)
			HTTPSPort:       8443,  // Default HTTPS port
			TLS:             make(map[string]TLSConfig),
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

	// Validate network ports
	if config.Network.Port < 1024 || config.Network.Port > 65535 {
		return fmt.Errorf("invalid network port %d: must be between 1024-65535", config.Network.Port)
	}
	// HTTP/HTTPS serving is optional per node; a zero port means "disabled".
	// Only validate the range when a port is actually configured.
	if config.Network.HTTPPort != 0 && (config.Network.HTTPPort < 1024 || config.Network.HTTPPort > 65535) {
		return fmt.Errorf("invalid HTTP port %d: must be between 1024-65535", config.Network.HTTPPort)
	}
	if config.Network.HTTPSPort != 0 && (config.Network.HTTPSPort < 1024 || config.Network.HTTPSPort > 65535) {
		return fmt.Errorf("invalid HTTPS port %d: must be between 1024-65535", config.Network.HTTPSPort)
	}

	// Validate TLS configuration
	for domain, tlsConfig := range config.Network.TLS {
		if tlsConfig.CertFile == "" {
			return fmt.Errorf("TLS configuration for domain '%s' missing cert_file", domain)
		}
		if tlsConfig.KeyFile == "" {
			return fmt.Errorf("TLS configuration for domain '%s' missing key_file", domain)
		}
		// Check that certificate and key files exist
		if _, err := os.Stat(tlsConfig.CertFile); os.IsNotExist(err) {
			return fmt.Errorf("TLS certificate file for domain '%s' does not exist: %s", domain, tlsConfig.CertFile)
		}
		if _, err := os.Stat(tlsConfig.KeyFile); os.IsNotExist(err) {
			return fmt.Errorf("TLS key file for domain '%s' does not exist: %s", domain, tlsConfig.KeyFile)
		}
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