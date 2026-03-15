// Package types provides path handling functionality for AmorphDB multi-mesh operations
package types

import (
	"fmt"
	"regexp"
	"strings"
)

// PathType represents the type of path being accessed
type PathType int

const (
	LocalPath   PathType = iota // Normal local path
	BridgePath                  // Cross-mesh bridge path (my.meshname.*)
	WorldPath                   // World-level path (world.*)
	InvalidPath                 // Invalid or malformed path
)

// ParsedPath represents a parsed path with type and components
type ParsedPath struct {
	Type         PathType
	OriginalPath []string
	MeshName     string   // For bridge paths
	LocalPath    []string // Translated path components
	Error        error    // Any parsing error
}

// PathValidator provides validation for mesh names and paths
type PathValidator struct {
	meshNameRegex *regexp.Regexp
}

// NewPathValidator creates a new path validator
func NewPathValidator() *PathValidator {
	// Mesh names: alphanumeric, hyphens, underscores only
	meshNameRegex := regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]*[a-zA-Z0-9]$`)
	return &PathValidator{
		meshNameRegex: meshNameRegex,
	}
}

// ParsePath parses a path and determines its type and components
func (pv *PathValidator) ParsePath(path []string) *ParsedPath {
	if len(path) == 0 {
		return &ParsedPath{
			Type:         InvalidPath,
			OriginalPath: path,
			Error:        fmt.Errorf("empty path"),
		}
	}

	// Check path type based on first component
	switch path[0] {
	case "my":
		return pv.parseBridgePath(path)
	case "world":
		return pv.parseWorldPath(path)
	default:
		return pv.parseLocalPath(path)
	}
}

// parseBridgePath handles my.meshname.* paths
func (pv *PathValidator) parseBridgePath(path []string) *ParsedPath {
	if len(path) < 2 {
		return &ParsedPath{
			Type:         InvalidPath,
			OriginalPath: path,
			Error:        fmt.Errorf("bridge path too short: expected my.<meshname>, got %v", path),
		}
	}

	meshName := path[1]

	// Validate mesh name
	if !pv.IsValidMeshName(meshName) {
		return &ParsedPath{
			Type:         InvalidPath,
			OriginalPath: path,
			Error:        fmt.Errorf("invalid mesh name in bridge path: %s", meshName),
		}
	}

	// Extract local path components (everything after my.meshname)
	var localPath []string
	if len(path) > 2 {
		localPath = path[2:]
	}

	return &ParsedPath{
		Type:         BridgePath,
		OriginalPath: path,
		MeshName:     meshName,
		LocalPath:    localPath,
		Error:        nil,
	}
}

// parseWorldPath handles world.* paths
func (pv *PathValidator) parseWorldPath(path []string) *ParsedPath {
	return &ParsedPath{
		Type:         WorldPath,
		OriginalPath: path,
		LocalPath:    path, // World paths are used as-is
		Error:        nil,
	}
}

// parseLocalPath handles normal local paths
func (pv *PathValidator) parseLocalPath(path []string) *ParsedPath {
	// Validate path components don't contain reserved words
	for _, component := range path {
		if pv.isReservedComponent(component) {
			return &ParsedPath{
				Type:         InvalidPath,
				OriginalPath: path,
				Error:        fmt.Errorf("path contains reserved component: %s", component),
			}
		}
	}

	return &ParsedPath{
		Type:         LocalPath,
		OriginalPath: path,
		LocalPath:    path,
		Error:        nil,
	}
}

// IsValidMeshName validates a mesh name format
func (pv *PathValidator) IsValidMeshName(meshName string) bool {
	if meshName == "" {
		return false
	}

	// Length constraints: 2-63 characters
	if len(meshName) < 2 || len(meshName) > 63 {
		return false
	}

	// Reserved names
	if pv.isReservedMeshName(meshName) {
		return false
	}

	// Pattern validation
	return pv.meshNameRegex.MatchString(meshName)
}

// isReservedMeshName checks if a mesh name is reserved
func (pv *PathValidator) isReservedMeshName(meshName string) bool {
	reserved := []string{
		"local", "localhost", "world", "global", "system", "admin",
		"root", "default", "main", "primary", "secondary", "backup",
		"test", "testing", "dev", "development", "prod", "production",
		"staging", "debug", "null", "undefined", "unknown",
		"standalone", // Our special standalone mode
	}

	meshNameLower := strings.ToLower(meshName)
	for _, reservedName := range reserved {
		if meshNameLower == reservedName {
			return true
		}
	}

	return false
}

// isReservedComponent checks if a path component is reserved
func (pv *PathValidator) isReservedComponent(component string) bool {
	reserved := []string{
		"", // Empty components not allowed
		".", "..", // Relative path components
		"~", "my", "world", // Scope prefixes
	}

	for _, reservedComponent := range reserved {
		if component == reservedComponent {
			return true
		}
	}

	return false
}

// ConvertBridgePathToAgentPath converts a bridge path to the corresponding agent path
func (pv *PathValidator) ConvertBridgePathToAgentPath(bridgePath []string, bridgeIdentityID string) ([]string, error) {
	parsed := pv.ParsePath(bridgePath)
	if parsed.Type != BridgePath {
		return nil, fmt.Errorf("not a bridge path: %v", bridgePath)
	}

	if parsed.Error != nil {
		return nil, parsed.Error
	}

	// Convert my.meshname.* -> world.agent.{bridge_identity}.*
	agentPath := []string{"world", "agent", bridgeIdentityID}
	if len(parsed.LocalPath) > 0 {
		agentPath = append(agentPath, parsed.LocalPath...)
	}

	return agentPath, nil
}

// GetMeshNameFromBridgePath extracts the mesh name from a bridge path
func (pv *PathValidator) GetMeshNameFromBridgePath(path []string) (string, error) {
	parsed := pv.ParsePath(path)
	if parsed.Type != BridgePath {
		return "", fmt.Errorf("not a bridge path: %v", path)
	}

	if parsed.Error != nil {
		return "", parsed.Error
	}

	return parsed.MeshName, nil
}

// IsBridgePath checks if a path is a bridge path
func (pv *PathValidator) IsBridgePath(path []string) bool {
	parsed := pv.ParsePath(path)
	return parsed.Type == BridgePath && parsed.Error == nil
}

// IsLocalPath checks if a path is a local path
func (pv *PathValidator) IsLocalPath(path []string) bool {
	parsed := pv.ParsePath(path)
	return parsed.Type == LocalPath && parsed.Error == nil
}

// IsWorldPath checks if a path is a world path
func (pv *PathValidator) IsWorldPath(path []string) bool {
	parsed := pv.ParsePath(path)
	return parsed.Type == WorldPath && parsed.Error == nil
}

// ValidatePath validates a path and returns any errors
func (pv *PathValidator) ValidatePath(path []string) error {
	parsed := pv.ParsePath(path)
	return parsed.Error
}

// NormalizePath normalizes a path by cleaning up components
func (pv *PathValidator) NormalizePath(path []string) []string {
	normalized := make([]string, 0, len(path))

	for _, component := range path {
		// Trim whitespace
		component = strings.TrimSpace(component)

		// Skip empty components
		if component == "" {
			continue
		}

		// Convert to lowercase for consistency (except for identities)
		if !isLikelyIdentity(component) {
			component = strings.ToLower(component)
		}

		normalized = append(normalized, component)
	}

	return normalized
}

// isLikelyIdentity checks if a component looks like an identity ID
func isLikelyIdentity(component string) bool {
	// Identity IDs are typically longer and contain hyphens, mixed case, or hex patterns
	if len(component) < 8 {
		return false
	}

	// Check for patterns common in identity IDs:
	// 1. Contains hyphens and alphanumeric characters
	// 2. Contains mixed case letters
	// 3. Hex-like patterns with hyphens

	hasHyphen := strings.Contains(component, "-")
	hasMixedCase := strings.ToLower(component) != component && strings.ToUpper(component) != component

	// Hex pattern with optional hyphens
	hexPattern := regexp.MustCompile(`^[a-fA-F0-9-]+$`)
	isHexLike := hexPattern.MatchString(component)

	// If it has hyphens and alphanumeric, or is hex-like, or has mixed case, it's likely an identity
	return (hasHyphen && isAlphanumericWithHyphens(component)) || isHexLike || hasMixedCase
}

// isAlphanumericWithHyphens checks if string contains only alphanumeric and hyphens
func isAlphanumericWithHyphens(s string) bool {
	for _, r := range s {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-') {
			return false
		}
	}
	return true
}

// FormatPath formats a path as a string for display
func FormatPath(path []string) string {
	return strings.Join(path, ".")
}

// ParsePathString parses a dot-separated path string into components
func ParsePathString(pathStr string) []string {
	if pathStr == "" {
		return []string{}
	}

	return strings.Split(pathStr, ".")
}