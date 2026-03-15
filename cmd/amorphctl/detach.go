// Package main provides detach command functionality for amorphctl
package main

import (
	"encoding/json"
	"fmt"
)

// DetachCommand handles detach operations
type DetachCommand struct {
	client CommandClient
}

// NewDetachCommand creates a new detach command handler
func NewDetachCommand(client CommandClient) *DetachCommand {
	return &DetachCommand{client: client}
}

// DetachFromMesh executes the detach command
func (dc *DetachCommand) DetachFromMesh(meshName string) error {
	if meshName == "" {
		fmt.Println("Detaching from primary mesh...")
	} else {
		fmt.Printf("Detaching from mesh '%s'...\n", meshName)
	}

	// Prepare request
	request := map[string]interface{}{
		"mesh_name": meshName,
	}

	requestData, err := json.Marshal(request)
	if err != nil {
		return fmt.Errorf("failed to prepare request: %w", err)
	}

	// Send request to service
	responseData, err := dc.client.SendCommand("detach", requestData)
	if err != nil {
		return fmt.Errorf("failed to send detach command: %w", err)
	}

	// Parse response
	var response DetachResponse
	if err := json.Unmarshal(responseData, &response); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	// Display results
	if response.Success {
		fmt.Printf("✅ %s\n", response.Message)

		if response.DetachInfo != nil {
			if response.DetachInfo.DetachType == "primary_mesh" {
				fmt.Printf("   Previous Mesh: %s\n", response.DetachInfo.MeshName)
				fmt.Printf("   Role: %s\n", response.DetachInfo.PreviousRole)
				fmt.Printf("   Status: Now standalone\n")

				if response.DetachInfo.ZonesMigrated > 0 {
					fmt.Printf("   Zones Migrated: %d to adjacent nodes\n", response.DetachInfo.ZonesMigrated)
				}

				if response.DetachInfo.DataPreserved {
					fmt.Printf("   Data: Preserved under 'my.standalone.*'\n")
				}
			} else {
				// Bridge disconnection
				fmt.Printf("   Disconnected Bridge: %s\n", response.DetachInfo.MeshName)
				fmt.Printf("   Bridge Identity: %s\n", response.DetachInfo.BridgeIdentity)
				fmt.Printf("   Status: Bridge removed\n")
			}

			if response.DetachInfo.DisconnectedAt != nil {
				fmt.Printf("   Disconnected: %s\n", formatDetachTimestamp(*response.DetachInfo.DisconnectedAt))
			}
		}
	} else {
		fmt.Printf("❌ %s\n", response.Message)
		return fmt.Errorf("detach operation failed")
	}

	return nil
}

// DetachResponse represents the response from detach operation
type DetachResponse struct {
	Success    bool              `json:"success"`
	Message    string            `json:"message"`
	DetachInfo *DetachInfo       `json:"detach_info,omitempty"`
}

// DetachInfo represents information about the detach operation
type DetachInfo struct {
	DetachType      string `json:"detach_type"`       // "primary_mesh" or "bridge"
	MeshName        string `json:"mesh_name"`
	PreviousRole    string `json:"previous_role,omitempty"`      // "founder" or "member"
	BridgeIdentity  string `json:"bridge_identity,omitempty"`   // For bridge disconnections
	ZonesMigrated   int    `json:"zones_migrated"`
	DataPreserved   bool   `json:"data_preserved"`
	DisconnectedAt  *int64 `json:"disconnected_at,omitempty"`
}

// formatDetachTimestamp formats a Unix timestamp for display
func formatDetachTimestamp(timestamp int64) string {
	// Simple timestamp formatting - could be enhanced
	return fmt.Sprintf("%d", timestamp)
}