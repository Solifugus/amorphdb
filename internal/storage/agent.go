// Package storage provides agent-specific storage functionality for AmorphDB
package storage

import (
	"fmt"
	"strings"

	"github.com/solifugus/amorphdb/internal/identity"
	"github.com/solifugus/amorphdb/internal/types"
)

// AgentStorage provides storage operations specific to agent identities
type AgentStorage struct {
	tree Tree
	im   *identity.IdentityManager
}

// NewAgentStorage creates a new agent storage interface
func NewAgentStorage(tree Tree, identityManager *identity.IdentityManager) *AgentStorage {
	return &AgentStorage{
		tree: tree,
		im:   identityManager,
	}
}

// ReadAgentData reads data from an agent's home directory
func (as *AgentStorage) ReadAgentData(agentIdentity *identity.Identity, path []string) (Value, error) {
	if err := identity.ValidateIdentity(agentIdentity); err != nil {
		return Value{}, fmt.Errorf("invalid agent identity: %w", err)
	}

	homePath := as.im.GetAgentHomePath(agentIdentity)
	fullPath := append(as.parseStoragePath(homePath), path...)

	return as.tree.Read(fullPath)
}

// WriteAgentData writes data to an agent's home directory
func (as *AgentStorage) WriteAgentData(agentIdentity *identity.Identity, path []string, value Value) error {
	if err := identity.ValidateIdentity(agentIdentity); err != nil {
		return fmt.Errorf("invalid agent identity: %w", err)
	}

	// Only mobile agents can write to replicated storage
	// Node agents write to local storage only
	if agentIdentity.Type == types.MobileAgent && !as.canWriteToMesh(agentIdentity) {
		return fmt.Errorf("mobile agent %s not authorized to write to mesh %s",
			agentIdentity.ID, agentIdentity.MeshName)
	}

	homePath := as.im.GetAgentHomePath(agentIdentity)
	fullPath := append(as.parseStoragePath(homePath), path...)

	// Use the agent's identity ID as the author
	authorID := as.hashIdentityID(agentIdentity.ID)

	return as.tree.Write(fullPath, value, authorID)
}

// ReadAgentDataAt reads historical data from an agent's home directory
func (as *AgentStorage) ReadAgentDataAt(agentIdentity *identity.Identity, path []string, timestamp int64) (Value, error) {
	if err := identity.ValidateIdentity(agentIdentity); err != nil {
		return Value{}, fmt.Errorf("invalid agent identity: %w", err)
	}

	homePath := as.im.GetAgentHomePath(agentIdentity)
	fullPath := append(as.parseStoragePath(homePath), path...)

	return as.tree.ReadAt(fullPath, timestamp)
}

// ListAgentData lists all data paths under an agent's home directory
func (as *AgentStorage) ListAgentData(agentIdentity *identity.Identity) ([]Attribute, error) {
	if err := identity.ValidateIdentity(agentIdentity); err != nil {
		return nil, fmt.Errorf("invalid agent identity: %w", err)
	}

	homePath := as.im.GetAgentHomePath(agentIdentity)
	fullPath := as.parseStoragePath(homePath)

	return as.tree.Children(fullPath)
}

// PurgeAgentData purges historical data from an agent's home directory
func (as *AgentStorage) PurgeAgentData(agentIdentity *identity.Identity, path []string, fromTime, toTime int64) error {
	if err := identity.ValidateIdentity(agentIdentity); err != nil {
		return fmt.Errorf("invalid agent identity: %w", err)
	}

	// Only allow purging if the agent owns this data
	if !as.canPurgeData(agentIdentity) {
		return fmt.Errorf("agent %s not authorized to purge data", agentIdentity.ID)
	}

	homePath := as.im.GetAgentHomePath(agentIdentity)
	fullPath := append(as.parseStoragePath(homePath), path...)

	authorID := as.hashIdentityID(agentIdentity.ID)

	return as.tree.Purge(fullPath, fromTime, toTime, authorID)
}

// ResolveAgentPath resolves a path that may contain agent-specific prefixes
// Handles paths like "my.meshname.data" for bridge access
func (as *AgentStorage) ResolveAgentPath(requesterIdentity *identity.Identity, path []string) ([]string, *identity.Identity, error) {
	if len(path) == 0 {
		return nil, nil, fmt.Errorf("empty path")
	}

	// Handle "my.meshname.*" bridge paths
	if path[0] == "my" && len(path) >= 2 {
		meshName := path[1]
		remainingPath := path[2:]

		// Check if this is a bridge mesh
		if bridgeIdentity := as.im.GetBridgeIdentity(meshName); bridgeIdentity != nil {
			homePath := as.im.GetAgentHomePath(bridgeIdentity)
			fullPath := append(as.parseStoragePath(homePath), remainingPath...)
			return fullPath, bridgeIdentity, nil
		}

		// Handle special case for "my.standalone.*" after joining a mesh
		if meshName == "standalone" {
			// Create a special local identity for standalone data access
			standaloneNodeID := "standalone-" + requesterIdentity.NodeID
			standaloneIdentity := &identity.Identity{
				ID:       standaloneNodeID,
				Type:     types.NodeAgent,
				MeshName: "standalone",
				NodeID:   standaloneNodeID,
			}
			homePath := as.im.GetAgentHomePath(standaloneIdentity)
			fullPath := append(as.parseStoragePath(homePath), remainingPath...)
			return fullPath, standaloneIdentity, nil
		}

		return nil, nil, fmt.Errorf("unknown bridge mesh: %s", meshName)
	}

	// Handle regular paths relative to the requester's home
	homePath := as.im.GetAgentHomePath(requesterIdentity)
	fullPath := append(as.parseStoragePath(homePath), path...)
	return fullPath, requesterIdentity, nil
}

// GetAgentStorageInfo returns storage information for an agent
type AgentStorageInfo struct {
	HomePath     string
	StorageType  string // "local" or "replicated"
	CanReplicate bool
	MeshName     string
}

func (as *AgentStorage) GetAgentStorageInfo(agentIdentity *identity.Identity) (*AgentStorageInfo, error) {
	if err := identity.ValidateIdentity(agentIdentity); err != nil {
		return nil, fmt.Errorf("invalid agent identity: %w", err)
	}

	info := &AgentStorageInfo{
		HomePath:     as.im.GetAgentHomePath(agentIdentity),
		MeshName:     agentIdentity.MeshName,
		CanReplicate: agentIdentity.Type.CanReplicate(),
	}

	if agentIdentity.Type == types.NodeAgent {
		info.StorageType = "local"
	} else {
		info.StorageType = "replicated"
	}

	return info, nil
}

// parseStoragePath converts a dot-separated path to a slice of path components
func (as *AgentStorage) parseStoragePath(path string) []string {
	if path == "" {
		return []string{}
	}
	return strings.Split(path, ".")
}

// canWriteToMesh checks if a mobile agent can write to its assigned mesh
func (as *AgentStorage) canWriteToMesh(agentIdentity *identity.Identity) bool {
	// For now, allow all mobile agents to write to their home mesh
	// In a full implementation, this would check mesh membership and permissions
	return agentIdentity.Type == types.MobileAgent
}

// canPurgeData checks if an agent can purge data from storage
func (as *AgentStorage) canPurgeData(agentIdentity *identity.Identity) bool {
	// For now, allow agents to purge their own data
	// In a full implementation, this would check permissions and ownership
	return true
}

// hashIdentityID converts an identity ID string to a uint64 for use as authorID
func (as *AgentStorage) hashIdentityID(identityID string) uint64 {
	// Simple hash function - in production, use a proper hash algorithm
	hash := uint64(0)
	for i, c := range identityID {
		hash = hash*31 + uint64(c) + uint64(i)
	}
	return hash
}

// MigrateAgentData migrates agent data between storage locations
// Used when transitioning between standalone and mesh modes
func (as *AgentStorage) MigrateAgentData(sourceIdentity, targetIdentity *identity.Identity, pathPrefix []string) error {
	if err := identity.ValidateIdentity(sourceIdentity); err != nil {
		return fmt.Errorf("invalid source identity: %w", err)
	}
	if err := identity.ValidateIdentity(targetIdentity); err != nil {
		return fmt.Errorf("invalid target identity: %w", err)
	}

	// List all data under the source path
	sourceHomePath := as.im.GetAgentHomePath(sourceIdentity)
	sourcePath := append(as.parseStoragePath(sourceHomePath), pathPrefix...)

	// Get all attributes under the source path
	attributes, err := as.tree.Children(sourcePath)
	if err != nil {
		return fmt.Errorf("failed to list source data: %w", err)
	}

	// Migrate each attribute
	for range attributes {
		// Read the current value from source path
		// Note: This is a simplified migration that copies the root path value
		// In a full implementation, we would need to recursively migrate all sub-attributes
		value, err := as.tree.Read(sourcePath)
		if err != nil {
			continue // Skip if source path can't be read
		}

		// Write to target location
		targetHomePath := as.im.GetAgentHomePath(targetIdentity)
		targetPath := append(as.parseStoragePath(targetHomePath), pathPrefix...)

		targetAuthorID := as.hashIdentityID(targetIdentity.ID)
		err = as.tree.Write(targetPath, value, targetAuthorID)
		if err != nil {
			return fmt.Errorf("failed to write migrated data: %w", err)
		}
	}

	return nil
}