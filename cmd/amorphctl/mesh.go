// Package main provides mesh management commands for amorphctl
package main

import (
	"encoding/json"
	"fmt"

	"github.com/solifugus/amorphdb/internal/config"
)

// CommandClient interface for mesh operations
type CommandClient interface {
	SendCommand(command string, payload []byte) ([]byte, error)
}

// MeshCommand handles mesh-related operations
type MeshCommand struct {
	client CommandClient
}

// NewMeshCommand creates a new mesh command handler
func NewMeshCommand(client CommandClient) *MeshCommand {
	return &MeshCommand{client: client}
}

// CreateMesh executes the create-mesh command
func (mc *MeshCommand) CreateMesh(meshName string) error {
	// Validate mesh name client-side first
	if err := config.ValidateMeshName(meshName); err != nil {
		return fmt.Errorf("invalid mesh name: %w", err)
	}

	fmt.Printf("Creating mesh '%s'...\n", meshName)

	// Prepare request
	request := map[string]interface{}{
		"name": meshName,
	}

	requestData, err := json.Marshal(request)
	if err != nil {
		return fmt.Errorf("failed to prepare request: %w", err)
	}

	// Send request to service
	responseData, err := mc.client.SendCommand("create-mesh", requestData)
	if err != nil {
		return fmt.Errorf("failed to send create-mesh command: %w", err)
	}

	// Parse response
	var response CreateMeshResponse
	if err := json.Unmarshal(responseData, &response); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	// Display results
	if response.Success {
		fmt.Printf("✅ %s\n", response.Message)
		if response.IsFounder {
			fmt.Printf("   Role: Mesh Founder\n")
		}
		if response.NodeID != "" {
			fmt.Printf("   Node Identity: %s\n", response.NodeID)
		}
		fmt.Printf("   Status: Ready for other nodes to join\n")
	} else {
		fmt.Printf("❌ %s\n", response.Message)
		return fmt.Errorf("mesh creation failed")
	}

	return nil
}

// GetMeshStatus retrieves and displays mesh status
func (mc *MeshCommand) GetMeshStatus() error {
	fmt.Println("Retrieving mesh status...")

	// Send status request
	responseData, err := mc.client.SendCommand("mesh-status", []byte("{}"))
	if err != nil {
		return fmt.Errorf("failed to get mesh status: %w", err)
	}

	// Parse response
	var status MeshStatusResponse
	if err := json.Unmarshal(responseData, &status); err != nil {
		return fmt.Errorf("failed to parse status response: %w", err)
	}

	// Display status
	fmt.Printf("Mesh Status:\n")
	if status.MeshName == "" {
		fmt.Printf("  Mode: Standalone\n")
		fmt.Printf("  Status: Not part of any mesh\n")
	} else {
		fmt.Printf("  Mesh Name: %s\n", status.MeshName)
		fmt.Printf("  Status: %s\n", status.Status)
		fmt.Printf("  Role: %s\n", mc.getRoleDescription(status.IsFounder))
		fmt.Printf("  Node Identity: %s\n", status.NodeIdentity)
		fmt.Printf("  Members: %d\n", status.MemberCount)
		fmt.Printf("  Path Authorities: %d\n", status.AuthorityCount)

		if status.FoundedAt != nil {
			fmt.Printf("  Founded: %s\n", formatTimestamp(*status.FoundedAt))
		}

		if len(status.Bridges) > 0 {
			fmt.Printf("  Bridges:\n")
			for meshName, bridge := range status.Bridges {
				fmt.Printf("    %s: %s (%s)\n", meshName, bridge.Status, bridge.Address)
			}
		}
	}

	return nil
}

// getRoleDescription returns a human-readable role description
func (mc *MeshCommand) getRoleDescription(isFounder bool) string {
	if isFounder {
		return "Founder"
	}
	return "Member"
}

// formatTimestamp formats a Unix timestamp for display
func formatTimestamp(timestamp int64) string {
	// Simple timestamp formatting - could be enhanced
	return fmt.Sprintf("%d", timestamp)
}

// Response types matching the service layer

// CreateMeshResponse represents the response from creating a mesh
type CreateMeshResponse struct {
	Success   bool   `json:"success"`
	Message   string `json:"message"`
	MeshName  string `json:"mesh_name,omitempty"`
	NodeID    string `json:"node_id,omitempty"`
	IsFounder bool   `json:"is_founder,omitempty"`
}

// MeshStatusResponse represents the mesh status response
type MeshStatusResponse struct {
	MeshName      string                        `json:"mesh_name"`
	Status        string                        `json:"status"`
	IsFounder     bool                          `json:"is_founder"`
	NodeIdentity  string                        `json:"node_identity"`
	MemberCount   int                           `json:"member_count"`
	AuthorityCount int                          `json:"authority_count"`
	FoundedAt     *int64                        `json:"founded_at,omitempty"`
	Bridges       map[string]BridgeStatusInfo   `json:"bridges"`
}

// BridgeStatusInfo represents bridge connection status information
type BridgeStatusInfo struct {
	Address     string `json:"address"`
	Status      string `json:"status"`
	Identity    string `json:"identity"`
	ConnectedAt *int64 `json:"connected_at,omitempty"`
}