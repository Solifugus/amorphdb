// Package types provides agent type definitions for AmorphDB mesh networking
package types

import (
	"encoding/json"
	"fmt"
)

// AgentType defines the type of agent in the mesh network
type AgentType int

const (
	// NodeAgent represents a static node that stores data locally only
	// Node agents are tied to specific physical nodes and do not replicate
	// their identity across the mesh
	NodeAgent AgentType = iota

	// MobileAgent represents a mobile identity that can move between nodes
	// Mobile agent data is stored in the mesh and replicated across nodes
	// for availability and mobility
	MobileAgent
)

// String returns the string representation of an AgentType
func (a AgentType) String() string {
	switch a {
	case NodeAgent:
		return "node"
	case MobileAgent:
		return "mobile"
	default:
		return "unknown"
	}
}

// MarshalJSON implements json.Marshaler for AgentType
func (a AgentType) MarshalJSON() ([]byte, error) {
	return json.Marshal(a.String())
}

// UnmarshalJSON implements json.Unmarshaler for AgentType
func (a *AgentType) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}

	switch s {
	case "node":
		*a = NodeAgent
	case "mobile":
		*a = MobileAgent
	default:
		return fmt.Errorf("invalid agent type: %s", s)
	}
	return nil
}

// IsValid returns true if the AgentType is valid
func (a AgentType) IsValid() bool {
	return a == NodeAgent || a == MobileAgent
}

// CanReplicate returns true if this agent type supports replication
func (a AgentType) CanReplicate() bool {
	return a == MobileAgent
}

// RequiresMeshStorage returns true if this agent type requires mesh storage
func (a AgentType) RequiresMeshStorage() bool {
	return a == MobileAgent
}