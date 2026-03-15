// Package main provides bridge command functionality for amorphctl
package main

import (
	"encoding/json"
	"fmt"
	"net"
	"time"
)

// BridgeCommand handles bridge operations
type BridgeCommand struct {
	client CommandClient
}

// NewBridgeCommand creates a new bridge command handler
func NewBridgeCommand(client CommandClient) *BridgeCommand {
	return &BridgeCommand{client: client}
}

// CreateBridge executes the bridge creation command
func (bc *BridgeCommand) CreateBridge(targetAddress string) error {
	// Validate target address
	if err := bc.validateTargetAddress(targetAddress); err != nil {
		return fmt.Errorf("invalid target address: %w", err)
	}

	fmt.Printf("Creating bridge to mesh at %s...\n", targetAddress)

	// Prepare request
	request := map[string]interface{}{
		"target_address": targetAddress,
	}

	requestData, err := json.Marshal(request)
	if err != nil {
		return fmt.Errorf("failed to prepare request: %w", err)
	}

	// Send request to service
	responseData, err := bc.client.SendCommand("create-bridge", requestData)
	if err != nil {
		return fmt.Errorf("failed to send create-bridge command: %w", err)
	}

	// Parse response
	var response CreateBridgeResponse
	if err := json.Unmarshal(responseData, &response); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	// Display results
	if response.Success {
		fmt.Printf("✅ %s\n", response.Message)

		if response.BridgeInfo != nil {
			fmt.Printf("   Target Mesh: %s\n", response.BridgeInfo.TargetMeshName)
			fmt.Printf("   Bridge Identity: %s\n", response.BridgeInfo.BridgeIdentity)
			fmt.Printf("   Status: %s\n", response.BridgeInfo.Status)
			if response.BridgeInfo.ConnectedAt != nil {
				fmt.Printf("   Connected: %s\n", formatBridgeTimestamp(*response.BridgeInfo.ConnectedAt))
			}
		}

		fmt.Printf("   ℹ️ Cross-mesh data now available under 'my.%s.*'\n", response.BridgeInfo.TargetMeshName)
	} else {
		fmt.Printf("❌ %s\n", response.Message)
		return fmt.Errorf("bridge creation failed")
	}

	return nil
}

// validateTargetAddress validates the target address format
func (bc *BridgeCommand) validateTargetAddress(targetAddress string) error {
	if targetAddress == "" {
		return fmt.Errorf("target address cannot be empty")
	}

	// Check if address contains port
	host, port, err := net.SplitHostPort(targetAddress)
	if err != nil {
		// No port specified - validate as hostname/IP
		if net.ParseIP(targetAddress) == nil {
			// Try as hostname
			if _, err := net.LookupHost(targetAddress); err != nil {
				return fmt.Errorf("invalid hostname or IP address: %s", targetAddress)
			}
		}
		return nil
	}

	// Validate host part
	if host == "" {
		return fmt.Errorf("empty host in address")
	}

	if net.ParseIP(host) == nil {
		// Try as hostname
		if _, err := net.LookupHost(host); err != nil {
			return fmt.Errorf("invalid hostname or IP address: %s", host)
		}
	}

	// Validate port part
	if port == "" {
		return fmt.Errorf("empty port in address")
	}

	return nil
}

// CreateBridgeResponse represents the response from creating a bridge
type CreateBridgeResponse struct {
	Success    bool               `json:"success"`
	Message    string             `json:"message"`
	BridgeInfo *BridgeInfo        `json:"bridge_info,omitempty"`
}

// BridgeInfo represents information about the created bridge
type BridgeInfo struct {
	TargetMeshName  string     `json:"target_mesh_name"`
	BridgeIdentity  string     `json:"bridge_identity"`
	Status          string     `json:"status"`
	ConnectedAt     *time.Time `json:"connected_at,omitempty"`
}

// formatBridgeTimestamp formats a timestamp for display (simplified)
func formatBridgeTimestamp(timestamp time.Time) string {
	return timestamp.Format("2006-01-02 15:04:05")
}