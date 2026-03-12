package main

import (
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/solifugus/amorphdb/internal/protocol"
	"github.com/solifugus/amorphdb/internal/storage"
)

// ProtocolClient implements the storage.Tree interface over the wire protocol
type ProtocolClient struct {
	conn     net.Conn // Network connection to service
	sequence uint32   // Message sequence counter
	mu       sync.Mutex // Protects sequence and conn access
}

// NewProtocolClient creates a new protocol client
func NewProtocolClient(address string) (*ProtocolClient, error) {
	var conn net.Conn
	var err error

	// Determine connection type based on address format
	if address[0] == '/' || address[0] == '.' {
		// UNIX socket path
		conn, err = net.Dial("unix", address)
	} else {
		// Network address
		conn, err = net.Dial("tcp", address)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to connect to AmorphDB service: %w", err)
	}

	return &ProtocolClient{
		conn:     conn,
		sequence: 1,
	}, nil
}

// Close closes the client connection
func (c *ProtocolClient) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn != nil {
		err := c.conn.Close()
		c.conn = nil
		return err
	}
	return nil
}

// Read implements storage.Tree.Read via protocol
func (c *ProtocolClient) Read(path []string) (storage.Value, error) {
	readMsg := &protocol.ReadMessage{Path: path}

	payload, err := protocol.EncodeReadMessage(readMsg)
	if err != nil {
		return storage.Value{}, fmt.Errorf("encode request: %w", err)
	}

	response, err := c.sendRequest(protocol.READ, payload)
	if err != nil {
		return storage.Value{}, err
	}

	if response.Type == protocol.ERROR {
		errorMsg, _ := protocol.DecodeErrorMessage(response.Payload)
		return storage.Value{}, fmt.Errorf("server error %d: %s", errorMsg.Code, errorMsg.Message)
	}

	if response.Type != protocol.READ_RESPONSE {
		return storage.Value{}, fmt.Errorf("unexpected response type: 0x%02x", response.Type)
	}

	responseMsg, err := protocol.DecodeReadResponseMessage(response.Payload)
	if err != nil {
		return storage.Value{}, fmt.Errorf("decode response: %w", err)
	}

	return responseMsg.Value, nil
}

// Write implements storage.Tree.Write via protocol
func (c *ProtocolClient) Write(path []string, value storage.Value, author uint64) error {
	writeMsg := &protocol.WriteMessage{
		Path:   path,
		Value:  value,
		Author: author,
	}

	payload, err := protocol.EncodeWriteMessage(writeMsg)
	if err != nil {
		return fmt.Errorf("encode request: %w", err)
	}

	response, err := c.sendRequest(protocol.WRITE, payload)
	if err != nil {
		return err
	}

	if response.Type == protocol.ERROR {
		errorMsg, _ := protocol.DecodeErrorMessage(response.Payload)
		return fmt.Errorf("server error %d: %s", errorMsg.Code, errorMsg.Message)
	}

	if response.Type != protocol.WRITE_ACK {
		return fmt.Errorf("unexpected response type: 0x%02x", response.Type)
	}

	ackMsg, err := protocol.DecodeWriteAckMessage(response.Payload)
	if err != nil {
		return fmt.Errorf("decode response: %w", err)
	}

	if !ackMsg.Success {
		return fmt.Errorf("write failed: %s", ackMsg.Error)
	}

	return nil
}

// ReadAt implements storage.Tree.ReadAt via protocol
func (c *ProtocolClient) ReadAt(path []string, timestamp int64) (storage.Value, error) {
	// For now, not implemented - would use READ_AT message type
	return storage.Value{}, fmt.Errorf("ReadAt not implemented in protocol client")
}

// Children implements storage.Tree.Children via protocol
func (c *ProtocolClient) Children(path []string) ([]storage.Attribute, error) {
	// For now, not implemented - would use CHILDREN message type
	return nil, fmt.Errorf("Children not implemented in protocol client")
}

// Purge implements storage.Tree.Purge via protocol
func (c *ProtocolClient) Purge(path []string, from int64, to int64, author uint64) error {
	purgeMsg := &protocol.PurgeMessage{
		Path:   path,
		From:   from,
		To:     to,
		Author: author,
	}

	payload, err := protocol.EncodePurgeMessage(purgeMsg)
	if err != nil {
		return fmt.Errorf("encode request: %w", err)
	}

	response, err := c.sendRequest(protocol.PURGE, payload)
	if err != nil {
		return err
	}

	if response.Type == protocol.ERROR {
		errorMsg, _ := protocol.DecodeErrorMessage(response.Payload)
		return fmt.Errorf("server error %d: %s", errorMsg.Code, errorMsg.Message)
	}

	if response.Type != protocol.PURGE_ACK {
		return fmt.Errorf("unexpected response type: 0x%02x", response.Type)
	}

	ackMsg, err := protocol.DecodePurgeAckMessage(response.Payload)
	if err != nil {
		return fmt.Errorf("decode response: %w", err)
	}

	if !ackMsg.Success {
		return fmt.Errorf("purge failed: %s", ackMsg.Error)
	}

	return nil
}

// sendRequest sends a protocol message and waits for response
func (c *ProtocolClient) sendRequest(msgType uint8, payload []byte) (*protocol.Message, error) {
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
	c.conn.SetReadDeadline(time.Now().Add(30 * time.Second))
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
func (c *ProtocolClient) readResponse() (*protocol.Message, error) {
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