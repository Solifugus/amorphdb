package main

import (
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/solifugus/amorphdb/internal/protocol"
)

// ControlClient handles communication with AmorphDB service for administrative commands
type ControlClient struct {
	conn     net.Conn   // UNIX socket connection to service
	sequence uint32     // Message sequence counter
	mu       sync.Mutex // Protects sequence and conn access
}

// NewControlClient creates a new control client connected to the local socket
func NewControlClient(socketPath string) (*ControlClient, error) {
	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to local socket: %w", err)
	}

	return &ControlClient{
		conn:     conn,
		sequence: 1,
	}, nil
}

// Close closes the control client connection
func (c *ControlClient) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn != nil {
		err := c.conn.Close()
		c.conn = nil
		return err
	}
	return nil
}

// GetStatus retrieves service status information
func (c *ControlClient) GetStatus() (*protocol.StatusResponseMessage, error) {
	// Send status request (empty payload)
	response, err := c.sendRequest(protocol.STATUS, []byte{})
	if err != nil {
		return nil, err
	}

	if response.Type == protocol.ERROR {
		errorMsg, _ := protocol.DecodeErrorMessage(response.Payload)
		return nil, fmt.Errorf("server error %d: %s", errorMsg.Code, errorMsg.Message)
	}

	if response.Type != protocol.STATUS_RESPONSE {
		return nil, fmt.Errorf("unexpected response type: 0x%02x", response.Type)
	}

	status, err := protocol.DecodeStatusResponseMessage(response.Payload)
	if err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return status, nil
}

// Stop requests service shutdown
func (c *ControlClient) Stop() error {
	// Create stop message
	stopMsg := &protocol.StopMessage{Force: false} // Graceful shutdown
	payload, err := encodeStopMessage(stopMsg)
	if err != nil {
		return fmt.Errorf("encode request: %w", err)
	}

	response, err := c.sendRequest(protocol.STOP, payload)
	if err != nil {
		return err
	}

	if response.Type == protocol.ERROR {
		errorMsg, _ := protocol.DecodeErrorMessage(response.Payload)
		return fmt.Errorf("server error %d: %s", errorMsg.Code, errorMsg.Message)
	}

	if response.Type != protocol.STOP_ACK {
		return fmt.Errorf("unexpected response type: 0x%02x", response.Type)
	}

	ackMsg, err := protocol.DecodeStopAckMessage(response.Payload)
	if err != nil {
		return fmt.Errorf("decode response: %w", err)
	}

	if !ackMsg.Success {
		return fmt.Errorf("stop failed: %s", ackMsg.Error)
	}

	return nil
}

// CreateInvite asks the daemon to mint a one-time enrollment token. The
// connection's identity must be authorized (@grant on world.agent); over the
// local socket the owner qualifies automatically.
func (c *ControlClient) CreateInvite() (string, error) {
	response, err := c.sendRequest(protocol.INVITE_CREATE, nil)
	if err != nil {
		return "", err
	}

	if response.Type == protocol.ERROR {
		errorMsg, _ := protocol.DecodeErrorMessage(response.Payload)
		return "", fmt.Errorf("server error %d: %s", errorMsg.Code, errorMsg.Message)
	}
	if response.Type != protocol.INVITE_CREATE_RESULT {
		return "", fmt.Errorf("unexpected response type: 0x%02x", response.Type)
	}

	result, err := protocol.DecodeInviteCreateResultMessage(response.Payload)
	if err != nil {
		return "", fmt.Errorf("decode response: %w", err)
	}
	if !result.Success {
		return "", fmt.Errorf("invite creation failed: %s", result.Error)
	}
	return result.Token, nil
}

// Compact triggers data compaction/defragmentation
func (c *ControlClient) Compact() error {
	// Send compact request (empty payload)
	response, err := c.sendRequest(protocol.COMPACT, []byte{})
	if err != nil {
		return err
	}

	if response.Type == protocol.ERROR {
		errorMsg, _ := protocol.DecodeErrorMessage(response.Payload)
		return fmt.Errorf("server error %d: %s", errorMsg.Code, errorMsg.Message)
	}

	if response.Type != protocol.COMPACT_ACK {
		return fmt.Errorf("unexpected response type: 0x%02x", response.Type)
	}

	ackMsg, err := protocol.DecodeCompactAckMessage(response.Payload)
	if err != nil {
		return fmt.Errorf("decode response: %w", err)
	}

	if !ackMsg.Success {
		return fmt.Errorf("compact failed: %s", ackMsg.Error)
	}

	return nil
}

// sendRequest sends a protocol message and waits for response
func (c *ControlClient) sendRequest(msgType uint8, payload []byte) (*protocol.Message, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn == nil {
		return nil, fmt.Errorf("client connection is closed")
	}

	// Create request message
	sequence := c.sequence
	c.sequence++

	request := protocol.CreateMessage(msgType, sequence, payload)

	// Encode and send request
	data, err := protocol.EncodeMessage(request)
	if err != nil {
		return nil, fmt.Errorf("encode message: %w", err)
	}

	_, err = c.conn.Write(data)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}

	// Read response with timeout
	c.conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	defer c.conn.SetReadDeadline(time.Time{})

	response, err := c.readResponse()
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	// Verify sequence matches
	if response.Sequence != sequence {
		return nil, fmt.Errorf("sequence mismatch: expected %d, got %d", sequence, response.Sequence)
	}

	return response, nil
}

// readResponse reads a complete protocol message from the connection
func (c *ControlClient) readResponse() (*protocol.Message, error) {
	// Read header (10 bytes: version + type + sequence + length)
	headerBytes := make([]byte, 10)
	n := 0
	for n < len(headerBytes) {
		read, err := c.conn.Read(headerBytes[n:])
		if err != nil {
			return nil, fmt.Errorf("read header: %w", err)
		}
		n += read
	}

	// Parse payload length from header
	payloadLength := uint32(headerBytes[6])<<24 | uint32(headerBytes[7])<<16 |
		uint32(headerBytes[8])<<8 | uint32(headerBytes[9])

	// Read payload
	payload := make([]byte, payloadLength)
	n = 0
	for n < len(payload) {
		read, err := c.conn.Read(payload[n:])
		if err != nil {
			return nil, fmt.Errorf("read payload: %w", err)
		}
		n += read
	}

	// Read checksum (4 bytes)
	checksumBytes := make([]byte, 4)
	n = 0
	for n < len(checksumBytes) {
		read, err := c.conn.Read(checksumBytes[n:])
		if err != nil {
			return nil, fmt.Errorf("read checksum: %w", err)
		}
		n += read
	}

	// Reconstruct full message
	fullMessage := make([]byte, 0, 14+payloadLength)
	fullMessage = append(fullMessage, headerBytes...)
	fullMessage = append(fullMessage, payload...)
	fullMessage = append(fullMessage, checksumBytes...)

	// Decode message
	return protocol.DecodeMessage(fullMessage)
}

// encodeStopMessage is a minimal encoder for StopMessage (since it's simple)
func encodeStopMessage(msg *protocol.StopMessage) ([]byte, error) {
	// For now, just encode the force flag as a single byte
	if msg.Force {
		return []byte{1}, nil
	}
	return []byte{0}, nil
}

// SendCommand sends a mesh management command to the service
func (c *ControlClient) SendCommand(command string, payload []byte) ([]byte, error) {
	var msgType uint8
	switch command {
	case "create-mesh":
		msgType = protocol.CREATE_MESH
	case "mesh-status":
		msgType = protocol.MESH_STATUS
	case "join-mesh":
		msgType = protocol.JOIN_MESH
	case "create-bridge":
		msgType = protocol.CREATE_BRIDGE
	case "detach":
		msgType = protocol.DETACH
	case "extract":
		msgType = protocol.EXTRACT
	default:
		return nil, fmt.Errorf("unknown command: %s", command)
	}

	response, err := c.sendRequest(msgType, payload)
	if err != nil {
		return nil, err
	}

	if response.Type == protocol.ERROR {
		errorMsg, _ := protocol.DecodeErrorMessage(response.Payload)
		return nil, fmt.Errorf("server error %d: %s", errorMsg.Code, errorMsg.Message)
	}

	// Verify response type matches expected
	expectedResponseType := msgType + 1 // Response types are typically command type + 1
	if response.Type != expectedResponseType {
		return nil, fmt.Errorf("unexpected response type: 0x%02x (expected 0x%02x)", response.Type, expectedResponseType)
	}

	return response.Payload, nil
}
