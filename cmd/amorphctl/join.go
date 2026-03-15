// Package main provides join command functionality for amorphctl
package main

import (
	"encoding/json"
	"fmt"
	"net"
	"time"
)

// JoinCommand handles mesh join operations
type JoinCommand struct {
	client CommandClient
}

// NewJoinCommand creates a new join command handler
func NewJoinCommand(client CommandClient) *JoinCommand {
	return &JoinCommand{client: client}
}

// JoinMesh executes the join mesh command
func (jc *JoinCommand) JoinMesh(seedAddress string) error {
	// Validate seed address
	if err := jc.validateSeedAddress(seedAddress); err != nil {
		return fmt.Errorf("invalid seed address: %w", err)
	}

	fmt.Printf("Joining mesh at %s...\n", seedAddress)

	// Prepare request
	request := map[string]interface{}{
		"seed_address": seedAddress,
	}

	requestData, err := json.Marshal(request)
	if err != nil {
		return fmt.Errorf("failed to prepare request: %w", err)
	}

	// Send request to service
	responseData, err := jc.client.SendCommand("join-mesh", requestData)
	if err != nil {
		return fmt.Errorf("failed to send join-mesh command: %w", err)
	}

	// Parse response
	var response JoinMeshResponse
	if err := json.Unmarshal(responseData, &response); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	// Display results
	if response.Success {
		fmt.Printf("✅ %s\n", response.Message)

		if response.MeshInfo != nil {
			fmt.Printf("   Mesh: %s\n", response.MeshInfo.Name)
			fmt.Printf("   Status: Member\n")
			if response.MeshInfo.AssignedZone != "" {
				fmt.Printf("   Assigned Zone: %s\n", response.MeshInfo.AssignedZone)
			}
			if response.MeshInfo.MemberCount > 0 {
				fmt.Printf("   Total Members: %d\n", response.MeshInfo.MemberCount)
			}
		}

		if response.NodeIdentity != "" {
			fmt.Printf("   Node Identity: %s\n", response.NodeIdentity)
		}

		if response.DataPreserved {
			fmt.Printf("   ℹ️ Existing data preserved under 'my.standalone.*'\n")
		}
	} else {
		fmt.Printf("❌ %s\n", response.Message)
		return fmt.Errorf("mesh join failed")
	}

	return nil
}

// validateSeedAddress validates the seed address format
func (jc *JoinCommand) validateSeedAddress(seedAddress string) error {
	if seedAddress == "" {
		return fmt.Errorf("seed address cannot be empty")
	}

	// Check if address contains port
	host, port, err := net.SplitHostPort(seedAddress)
	if err != nil {
		// No port specified - validate as hostname/IP
		if net.ParseIP(seedAddress) == nil {
			// Try as hostname
			if _, err := net.LookupHost(seedAddress); err != nil {
				return fmt.Errorf("invalid hostname or IP address: %s", seedAddress)
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

	// Port validation could be enhanced here
	return nil
}

// JoinMeshResponse represents the response from joining a mesh
type JoinMeshResponse struct {
	Success       bool               `json:"success"`
	Message       string             `json:"message"`
	NodeIdentity  string             `json:"node_identity,omitempty"`
	MeshInfo      *JoinedMeshInfo    `json:"mesh_info,omitempty"`
	DataPreserved bool               `json:"data_preserved"`
}

// JoinedMeshInfo represents information about the joined mesh
type JoinedMeshInfo struct {
	Name           string    `json:"name"`
	AssignedZone   string    `json:"assigned_zone,omitempty"`
	MemberCount    int       `json:"member_count"`
	JoinedAt       time.Time `json:"joined_at"`
}