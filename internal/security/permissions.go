// Package security implements AmorphDB access control and security features
package security

import (
	"fmt"

	"github.com/solifugus/amorphdb/internal/storage"
	"github.com/solifugus/amorphdb/internal/types"
)

// PermissionType represents the five types of permissions in AmorphDB
type PermissionType string

const (
	ReadPermission   PermissionType = "read"   // @read - can see the data
	WritePermission  PermissionType = "write"  // @write - can change existing values
	ExpandPermission PermissionType = "expand" // @expand - can add new attributes
	GrantPermission  PermissionType = "grant"  // @grant - can modify permissions
	PurgePermission  PermissionType = "purge"  // @purge - can permanently erase data
)

// Agent represents an agent with properties for permission evaluation
type Agent struct {
	Identity string                 // Unique agent identifier (CV syllable format)
	Stamp    map[string]interface{} // Agent's stamp attributes (role, clearance, etc.)
	Tree     storage.ExtendedTree   // Access to storage
	AgentID  uint64                 // Internal agent ID
}

// GetProperty returns a property from the agent's stamp for condition evaluation
func (a *Agent) GetProperty(name string) interface{} {
	if name == "identity" {
		return types.Text{Value: a.Identity}
	}

	if value, exists := a.Stamp[name]; exists {
		return value
	}

	return types.Nothing{}
}

// PermissionEvaluator handles permission checking and cascade logic
type PermissionEvaluator struct {
	tree storage.ExtendedTree
}

// NewPermissionEvaluator creates a new permission evaluator
func NewPermissionEvaluator(tree storage.ExtendedTree) *PermissionEvaluator {
	return &PermissionEvaluator{tree: tree}
}

// CheckPermission evaluates if an agent has a specific permission on a path
func (pe *PermissionEvaluator) CheckPermission(agent *Agent, path []string, permType PermissionType) (bool, error) {
	// Get the effective permission condition for this path and permission type
	condition, err := pe.getEffectivePermission(path, permType, agent.AgentID)
	if err != nil {
		return false, fmt.Errorf("failed to get permission condition: %v", err)
	}

	// If no condition found, apply defaults
	if condition == nil {
		return pe.getDefaultPermission(path, permType, agent), nil
	}

	// Evaluate the permission condition against the agent
	result, err := pe.evaluateCondition(condition, agent)
	if err != nil {
		return false, fmt.Errorf("failed to evaluate permission condition (type: %T): %v", condition, err)
	}

	return result, nil
}

// getEffectivePermission walks up the tree to find the applicable permission condition
func (pe *PermissionEvaluator) getEffectivePermission(path []string, permType PermissionType, agentID uint64) (interface{}, error) {
	// Walk from the target path up to root, checking for permission meta-attributes
	for i := len(path); i >= 0; i-- {
		// Create a copy to avoid modifying the original path slice
		currentPath := make([]string, i)
		copy(currentPath, path[:i])

		// Construct the permission meta-attribute name
		permAttr := "@" + string(permType)

		// Look for permission at this level
		fullPath := append(currentPath, permAttr)
		attrHash := storage.HashPath(fullPath)
		instance, err := pe.tree.GetInstance(attrHash, 0) // Query system agent for global permissions
		if err != nil {
			continue // No permission at this level, continue upward
		}

		// Found a permission - return its condition
		value, err := pe.tree.GetValue(instance.ValueHash)
		if err != nil {
			continue
		}

		// Deserialize the condition using the stored TypeTag
		if len(value) < 1 {
			return nil, fmt.Errorf("stored value too short")
		}

		// First byte is the TypeTag, rest is the data
		typeTag := value[0]
		data := value[1:]

		serializedValue := types.SerializedValue{
			TypeTag: typeTag,
			Data:    data,
		}

		condition, err := types.DeserializeValue(serializedValue)
		if err != nil {
			return nil, fmt.Errorf("failed to deserialize condition: %v", err)
		}

		return condition, nil
	}

	// No explicit permission found in the hierarchy
	return nil, nil
}

// getDefaultPermission returns the default permission for a path based on context
func (pe *PermissionEvaluator) getDefaultPermission(path []string, permType PermissionType, agent *Agent) bool {
	// Under an agent's own home (world.agent.{id}.*): all permissions default to
	// owner-only. This is self-relative — only the requesting agent, the owner of
	// that home, holds default permissions on it; any other agent falls through to
	// the closed default below. (Previously keyed off a `~` path prefix, which was
	// self-relative by construction; `~` has since been removed from the language,
	// so the rule now recognizes the resolved home path directly.)
	if agent != nil && len(path) >= 3 && path[0] == "world" && path[1] == "agent" &&
		path[2] == fmt.Sprintf("%d", agent.AgentID) {
		return true
	}

	// Under world (global root): different defaults
	if len(path) > 0 && path[0] == "world" {
		switch permType {
		case ReadPermission:
			return true // @read defaults to (Anything) - open to all
		case WritePermission:
			return false // @write defaults to (Nothing) - no agent can write without explicit grant
		case ExpandPermission, GrantPermission, PurgePermission:
			return false // All other permissions default to (Nothing) - closed
		}
	}

	// For other paths, default to closed
	return false
}

// evaluateCondition evaluates a permission condition against an agent
func (pe *PermissionEvaluator) evaluateCondition(condition interface{}, agent *Agent) (bool, error) {
	switch cond := condition.(type) {
	case types.Boolean:
		return cond.Value, nil

	case types.Text:
		// Handle special values
		switch cond.Value {
		case "(Anything)":
			return true, nil
		case "(Nothing)":
			return false, nil
		default:
			// Check if agent identity matches the condition
			return agent.Identity == cond.Value, nil
		}

	case types.List:
		// Check if agent identity is in the list
		for _, element := range cond.Elements {
			if elementText, ok := element.(types.Text); ok {
				if agent.Identity == elementText.Value {
					return true, nil
				}
			}
		}
		return false, nil

	case types.Nothing:
		return false, nil

	default:
		// TODO: Implement full condition expression evaluation
		// This would include parsing expressions like:
		// (agent.role ?= "employee")
		// (agent.role ?= "manager" or agent.role ?= "admin")
		// (agent.clearance in ["public", "internal"])

		// For now, return false for unknown condition types
		return false, fmt.Errorf("unsupported condition type: %T", condition)
	}
}

// SetPermission sets a permission condition at a specific path
func (pe *PermissionEvaluator) SetPermission(path []string, permType PermissionType, condition interface{}, agentID uint64) error {
	// Construct the permission meta-attribute name
	permAttr := "@" + string(permType)
	fullPath := append(path, permAttr)

	// Determine TypeTag and serialize the condition
	var typeTag byte
	var data []byte

	switch cond := condition.(type) {
	case types.Text:
		typeTag = types.TypeText
		data = cond.Serialize()
	case types.Boolean:
		typeTag = types.TypeBoolean
		data = cond.Serialize()
	case types.List:
		typeTag = types.TypeList
		data = cond.Serialize()
	case types.Nothing:
		typeTag = types.TypeNothing
		data = cond.Serialize()
	default:
		return fmt.Errorf("unsupported permission condition type: %T", condition)
	}

	// Create value with TypeTag as first byte, followed by data
	value := make([]byte, 1+len(data))
	value[0] = typeTag
	copy(value[1:], data)

	// Store in the tree
	attrHash := storage.HashPath(fullPath)
	valueHash := storage.HashValue(value)

	// Create the instance - permissions are stored globally (agent 0)
	instance := storage.SecurityInstance{
		AttributeHash: attrHash,
		ValueHash:     valueHash,
		Agent:         0, // Global storage for permissions
		Timestamp:     storage.GetCurrentTimestamp(),
	}

	// Store value and instance
	err := pe.tree.PutValue(valueHash, value)
	if err != nil {
		return fmt.Errorf("failed to store permission value: %v", err)
	}

	err = pe.tree.PutInstance(attrHash, instance)
	if err != nil {
		return fmt.Errorf("failed to store permission instance: %v", err)
	}

	return nil
}

// HasPermission is a convenience method for checking permissions
func (pe *PermissionEvaluator) HasPermission(agent *Agent, path []string, permType PermissionType) bool {
	result, err := pe.CheckPermission(agent, path, permType)
	if err != nil {
		// Log error in a real implementation
		return false
	}
	return result
}

// CanRead checks if agent can read from the specified path
func (pe *PermissionEvaluator) CanRead(agent *Agent, path []string) bool {
	return pe.HasPermission(agent, path, ReadPermission)
}

// CanWrite checks if agent can write to the specified path
func (pe *PermissionEvaluator) CanWrite(agent *Agent, path []string) bool {
	return pe.HasPermission(agent, path, WritePermission)
}

// CanExpand checks if agent can add new attributes at the specified path
func (pe *PermissionEvaluator) CanExpand(agent *Agent, path []string) bool {
	return pe.HasPermission(agent, path, ExpandPermission)
}

// CanGrant checks if agent can modify permissions at the specified path
func (pe *PermissionEvaluator) CanGrant(agent *Agent, path []string) bool {
	return pe.HasPermission(agent, path, GrantPermission)
}

// CanPurge checks if agent can permanently erase data at the specified path
func (pe *PermissionEvaluator) CanPurge(agent *Agent, path []string) bool {
	return pe.HasPermission(agent, path, PurgePermission)
}
