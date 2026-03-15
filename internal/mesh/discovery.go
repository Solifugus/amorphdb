// Package mesh provides mesh discovery functionality for AmorphDB
package mesh

import (
	"fmt"
	"net"
	"time"

	"github.com/solifugus/amorphdb/internal/identity"
	"github.com/solifugus/amorphdb/internal/protocol"
	"github.com/solifugus/amorphdb/internal/types"
)

// MeshInfo represents information about a discovered mesh
type MeshInfo struct {
	Name            string              // Mesh name
	FounderIdentity *identity.Identity  // Founder's identity
	MemberCount     int                 // Current number of members
	ZoneCount       int                 // Number of zones
	RequiresAuth    bool                // Whether authentication is required
	FoundedAt       time.Time           // When the mesh was founded
}

// DiscoveryClient handles mesh discovery operations
type DiscoveryClient struct {
	timeout time.Duration // Connection timeout
}

// NewDiscoveryClient creates a new discovery client
func NewDiscoveryClient() *DiscoveryClient {
	return &DiscoveryClient{
		timeout: 10 * time.Second, // 10 second timeout
	}
}

// DiscoverMesh connects to a seed address and discovers mesh information
func (dc *DiscoveryClient) DiscoverMesh(seedAddress string) (*MeshInfo, error) {
	// Parse and validate the address
	if seedAddress == "" {
		return nil, fmt.Errorf("seed address cannot be empty")
	}

	// Connect to the seed node
	conn, err := dc.connectToSeed(seedAddress)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to seed %s: %w", seedAddress, err)
	}
	defer conn.Close()

	// Send discovery request
	meshInfo, err := dc.sendDiscoveryRequest(conn)
	if err != nil {
		return nil, fmt.Errorf("failed to discover mesh info: %w", err)
	}

	return meshInfo, nil
}

// connectToSeed establishes a TCP connection to the seed address
func (dc *DiscoveryClient) connectToSeed(seedAddress string) (net.Conn, error) {
	// Add default port if not specified
	host, port, err := net.SplitHostPort(seedAddress)
	if err != nil {
		// Assume no port specified, add default
		seedAddress = fmt.Sprintf("%s:5000", seedAddress)
		host = seedAddress[:len(seedAddress)-5] // Remove :5000
		port = "5000"
	}

	// Validate the address format
	if host == "" || port == "" {
		return nil, fmt.Errorf("invalid seed address format: %s", seedAddress)
	}

	// Connect with timeout
	conn, err := net.DialTimeout("tcp", seedAddress, dc.timeout)
	if err != nil {
		return nil, fmt.Errorf("failed to connect: %w", err)
	}

	// Set read/write timeouts
	deadline := time.Now().Add(dc.timeout)
	if err := conn.SetDeadline(deadline); err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to set connection deadline: %w", err)
	}

	return conn, nil
}

// sendDiscoveryRequest sends a discovery message and waits for response
func (dc *DiscoveryClient) sendDiscoveryRequest(conn net.Conn) (*MeshInfo, error) {
	// Create discovery message
	discoveryMsg := &protocol.DiscoverMeshMessage{}
	payload, err := protocol.EncodeDiscoverMeshMessage(discoveryMsg)
	if err != nil {
		return nil, fmt.Errorf("failed to encode discovery message: %w", err)
	}

	// Create protocol message
	message := protocol.CreateMessage(protocol.DISCOVER_MESH, 1, payload)
	messageData, err := protocol.EncodeMessage(message)
	if err != nil {
		return nil, fmt.Errorf("failed to encode protocol message: %w", err)
	}

	// Send the message
	if _, err := conn.Write(messageData); err != nil {
		return nil, fmt.Errorf("failed to send discovery request: %w", err)
	}

	// Read response
	responseData, err := dc.readResponse(conn)
	if err != nil {
		return nil, fmt.Errorf("failed to read discovery response: %w", err)
	}

	// Decode response message
	responseMsg, err := protocol.DecodeMessage(responseData)
	if err != nil {
		return nil, fmt.Errorf("failed to decode response message: %w", err)
	}

	// Verify response type
	if responseMsg.Type != protocol.DISCOVER_MESH_RESPONSE {
		return nil, fmt.Errorf("unexpected response type: expected %d, got %d",
			protocol.DISCOVER_MESH_RESPONSE, responseMsg.Type)
	}

	// Decode discovery response
	discoveryResponse, err := protocol.DecodeDiscoverMeshResponseMessage(responseMsg.Payload)
	if err != nil {
		return nil, fmt.Errorf("failed to decode discovery response: %w", err)
	}

	// Convert to MeshInfo
	meshInfo := &MeshInfo{
		Name:            discoveryResponse.MeshName,
		MemberCount:     discoveryResponse.MemberCount,
		ZoneCount:       discoveryResponse.ZoneCount,
		RequiresAuth:    discoveryResponse.RequiresAuth,
		FoundedAt:       time.Unix(discoveryResponse.FoundedAt, 0),
	}

	// Create founder identity if provided
	if discoveryResponse.FounderIdentity != "" {
		meshInfo.FounderIdentity = &identity.Identity{
			ID:   discoveryResponse.FounderIdentity,
			Type: types.NodeAgent,
		}
	}

	return meshInfo, nil
}

// readResponse reads a complete protocol message from the connection
func (dc *DiscoveryClient) readResponse(conn net.Conn) ([]byte, error) {
	// First, read the header to get message length
	header := make([]byte, 10) // Version(1) + Type(1) + Sequence(4) + PayloadLength(4)
	if _, err := conn.Read(header); err != nil {
		return nil, fmt.Errorf("failed to read message header: %w", err)
	}

	// Extract payload length (bytes 6-9, big endian)
	payloadLength := uint32(header[6])<<24 | uint32(header[7])<<16 |
	                 uint32(header[8])<<8  | uint32(header[9])

	// Calculate total message size: header(10) + payload + checksum(4)
	totalSize := 10 + int(payloadLength) + 4

	// Create buffer for complete message
	message := make([]byte, totalSize)
	copy(message, header)

	// Read remaining data (payload + checksum)
	remaining := totalSize - 10
	if remaining > 0 {
		if _, err := conn.Read(message[10:]); err != nil {
			return nil, fmt.Errorf("failed to read message payload: %w", err)
		}
	}

	return message, nil
}

// JoinClient handles mesh joining operations
type JoinClient struct {
	timeout time.Duration
}

// NewJoinClient creates a new join client
func NewJoinClient() *JoinClient {
	return &JoinClient{
		timeout: 30 * time.Second, // 30 second timeout for join operations
	}
}

// JoinMesh sends a join request to a mesh node
func (jc *JoinClient) JoinMesh(seedAddress string, nodeIdentity *identity.Identity, meshName string) (*JoinResult, error) {
	// Connect to seed node
	conn, err := jc.connectToSeed(seedAddress)
	if err != nil {
		return nil, fmt.Errorf("failed to connect for join: %w", err)
	}
	defer conn.Close()

	// Send join request
	result, err := jc.sendJoinRequest(conn, nodeIdentity, meshName)
	if err != nil {
		return nil, fmt.Errorf("failed to send join request: %w", err)
	}

	return result, nil
}

// JoinResult represents the result of a mesh join operation
type JoinResult struct {
	Success      bool   // Whether join was successful
	Message      string // Status or error message
	AssignedZone string // Zone assigned to the new member
	MemberCount  int    // Updated member count after join
}

// connectToSeed establishes connection for join operations
func (jc *JoinClient) connectToSeed(seedAddress string) (net.Conn, error) {
	// Add default port if not specified
	_, _, err := net.SplitHostPort(seedAddress)
	if err != nil {
		seedAddress = fmt.Sprintf("%s:5000", seedAddress)
	}

	conn, err := net.DialTimeout("tcp", seedAddress, jc.timeout)
	if err != nil {
		return nil, fmt.Errorf("failed to connect: %w", err)
	}

	// Set deadline for join operation
	deadline := time.Now().Add(jc.timeout)
	if err := conn.SetDeadline(deadline); err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to set deadline: %w", err)
	}

	return conn, nil
}

// sendJoinRequest sends a join message and waits for response
func (jc *JoinClient) sendJoinRequest(conn net.Conn, nodeIdentity *identity.Identity, meshName string) (*JoinResult, error) {
	// Create join message
	joinMsg := &protocol.JoinMeshMessage{
		NodeIdentity: nodeIdentity.ID,
		MeshName:     meshName,
	}

	payload, err := protocol.EncodeJoinMeshMessage(joinMsg)
	if err != nil {
		return nil, fmt.Errorf("failed to encode join message: %w", err)
	}

	// Create protocol message
	message := protocol.CreateMessage(protocol.JOIN_MESH, 2, payload)
	messageData, err := protocol.EncodeMessage(message)
	if err != nil {
		return nil, fmt.Errorf("failed to encode protocol message: %w", err)
	}

	// Send the message
	if _, err := conn.Write(messageData); err != nil {
		return nil, fmt.Errorf("failed to send join request: %w", err)
	}

	// Read response
	responseData, err := jc.readResponse(conn)
	if err != nil {
		return nil, fmt.Errorf("failed to read join response: %w", err)
	}

	// Decode response message
	responseMsg, err := protocol.DecodeMessage(responseData)
	if err != nil {
		return nil, fmt.Errorf("failed to decode response message: %w", err)
	}

	// Verify response type
	if responseMsg.Type != protocol.JOIN_MESH_ACK {
		return nil, fmt.Errorf("unexpected response type: expected %d, got %d",
			protocol.JOIN_MESH_ACK, responseMsg.Type)
	}

	// Decode join response
	joinResponse, err := protocol.DecodeJoinMeshAckMessage(responseMsg.Payload)
	if err != nil {
		return nil, fmt.Errorf("failed to decode join response: %w", err)
	}

	// Convert to JoinResult
	result := &JoinResult{
		Success:      joinResponse.Success,
		Message:      joinResponse.Message,
		AssignedZone: joinResponse.AssignedZone,
		MemberCount:  joinResponse.MemberCount,
	}

	return result, nil
}

// readResponse reads a complete protocol message from the connection
func (jc *JoinClient) readResponse(conn net.Conn) ([]byte, error) {
	// Read header first
	header := make([]byte, 10)
	if _, err := conn.Read(header); err != nil {
		return nil, fmt.Errorf("failed to read message header: %w", err)
	}

	// Extract payload length
	payloadLength := uint32(header[6])<<24 | uint32(header[7])<<16 |
	                 uint32(header[8])<<8  | uint32(header[9])

	// Calculate total message size
	totalSize := 10 + int(payloadLength) + 4

	// Create buffer for complete message
	message := make([]byte, totalSize)
	copy(message, header)

	// Read remaining data
	remaining := totalSize - 10
	if remaining > 0 {
		if _, err := conn.Read(message[10:]); err != nil {
			return nil, fmt.Errorf("failed to read message payload: %w", err)
		}
	}

	return message, nil
}