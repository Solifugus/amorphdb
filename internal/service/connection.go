package service

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"sync"

	"github.com/solifugus/amorphdb/internal/mbl/interpreter"
	"github.com/solifugus/amorphdb/internal/protocol"
	"github.com/solifugus/amorphdb/internal/security"
)

// Connection represents a client connection to the service
type Connection struct {
	id          string        // Unique connection identifier
	conn        net.Conn      // Underlying network connection
	isLocal     bool          // Whether this is a local socket connection
	service     *Service      // Reference to parent service
	interpreter *interpreter.Interpreter // Per-connection MBL interpreter
	agentID     uint64        // Current agent identity for this connection
	reader      *bufio.Reader // Buffered reader for the connection
	writer      io.Writer     // Writer for responses
	mu          sync.Mutex    // Protects connection state
	closed      bool          // Whether connection is closed
}

// NewConnection creates a new connection handler
func NewConnection(id string, conn net.Conn, isLocal bool, service *Service) *Connection {
	// Default agent ID for new connections
	const defaultAgentID uint64 = 1000

	// Create per-connection interpreter
	interp := interpreter.New(service.tree, defaultAgentID)

	return &Connection{
		id:          id,
		conn:        conn,
		isLocal:     isLocal,
		service:     service,
		interpreter: interp,
		agentID:     defaultAgentID,
		reader:      bufio.NewReader(conn),
		writer:      conn,
	}
}

// Start begins processing messages for this connection
func (c *Connection) Start() {
	defer c.Close()

	fmt.Printf("Connection %s started (local: %t)\n", c.id, c.isLocal)

	for {
		select {
		case <-c.service.ctx.Done():
			return // Service is shutting down

		default:
			// Read and process next message
			if err := c.processNextMessage(); err != nil {
				if err == io.EOF {
					fmt.Printf("Connection %s closed by client\n", c.id)
				} else {
					fmt.Printf("Connection %s error: %v\n", c.id, err)
				}
				return
			}
		}
	}
}

// Close closes the connection
func (c *Connection) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return nil
	}

	c.closed = true
	return c.conn.Close()
}

// createAgent creates a security.Agent object for the current connection
func (c *Connection) createAgent() *security.Agent {
	return &security.Agent{
		Identity:  fmt.Sprintf("agent-%d", c.agentID),
		Stamp:     make(map[string]interface{}), // Basic stamp for now
		Tree:      c.service.tree, // Now service.tree is already ExtendedTree
		AgentID:   c.agentID,
	}
}

// processNextMessage reads and processes one protocol message
func (c *Connection) processNextMessage() error {
	// Read message header (14 bytes: version + type + sequence + length)
	headerBytes := make([]byte, 10)
	if _, err := io.ReadFull(c.reader, headerBytes); err != nil {
		return err
	}

	// Parse payload length from header
	payloadLength := uint32(headerBytes[6])<<24 | uint32(headerBytes[7])<<16 |
		uint32(headerBytes[8])<<8 | uint32(headerBytes[9])

	// Read payload
	payload := make([]byte, payloadLength)
	if _, err := io.ReadFull(c.reader, payload); err != nil {
		return err
	}

	// Read checksum (4 bytes)
	checksumBytes := make([]byte, 4)
	if _, err := io.ReadFull(c.reader, checksumBytes); err != nil {
		return err
	}

	// Reconstruct full message
	fullMessage := make([]byte, 0, 14+payloadLength)
	fullMessage = append(fullMessage, headerBytes...)
	fullMessage = append(fullMessage, payload...)
	fullMessage = append(fullMessage, checksumBytes...)

	// Decode message
	msg, err := protocol.DecodeMessage(fullMessage)
	if err != nil {
		return fmt.Errorf("message decode error: %w", err)
	}

	// Dispatch message based on type
	response := c.handleMessage(msg)

	// Send response
	return c.sendResponse(response)
}

// handleMessage processes a decoded protocol message
func (c *Connection) handleMessage(msg *protocol.Message) *protocol.Message {
	switch msg.Type {
	case protocol.READ:
		return c.handleRead(msg)
	case protocol.WRITE:
		return c.handleWrite(msg)
	case protocol.PURGE:
		return c.handlePurge(msg)
	case protocol.READ_AT:
		return c.handleReadAt(msg)
	case protocol.CHILDREN:
		return c.handleChildren(msg)
	case protocol.STATUS:
		return c.handleStatus(msg)
	case protocol.STOP:
		return c.handleStop(msg)
	case protocol.COMPACT:
		return c.handleCompact(msg)
	default:
		return c.createErrorResponse(msg.Sequence, 400, fmt.Sprintf("Unknown message type: 0x%02x", msg.Type))
	}
}

// handleRead processes READ messages
func (c *Connection) handleRead(msg *protocol.Message) *protocol.Message {
	readMsg, err := protocol.DecodeReadMessage(msg.Payload)
	if err != nil {
		return c.createErrorResponse(msg.Sequence, 400, fmt.Sprintf("Invalid READ message: %v", err))
	}

	// Create agent object for permission checking
	agent := c.createAgent()

	// Check permissions
	if !c.service.permEvaluator.CanRead(agent, readMsg.Path) {
		return c.createErrorResponse(msg.Sequence, 403, "Permission denied")
	}

	// Read value from storage
	value, err := c.service.tree.Read(readMsg.Path)
	if err != nil {
		return c.createErrorResponse(msg.Sequence, 404, fmt.Sprintf("Read failed: %v", err))
	}

	// Apply output filtering
	// For now, use the raw value since FilterOutput is not the correct method
	// TODO: Implement proper filtering with stamps and FilterVisible
	filteredValue := value

	// Create response
	responseMsg := &protocol.ReadResponseMessage{Value: filteredValue}
	payload, err := protocol.EncodeReadResponseMessage(responseMsg)
	if err != nil {
		return c.createErrorResponse(msg.Sequence, 500, fmt.Sprintf("Response encoding failed: %v", err))
	}

	return protocol.CreateMessage(protocol.READ_RESPONSE, msg.Sequence, payload)
}

// handleWrite processes WRITE messages
func (c *Connection) handleWrite(msg *protocol.Message) *protocol.Message {
	writeMsg, err := protocol.DecodeWriteMessage(msg.Payload)
	if err != nil {
		return c.createErrorResponse(msg.Sequence, 400, fmt.Sprintf("Invalid WRITE message: %v", err))
	}

	// Create agent object for permission checking
	agent := c.createAgent()

	// Check permissions
	if !c.service.permEvaluator.CanWrite(agent, writeMsg.Path) {
		return c.createErrorResponse(msg.Sequence, 403, "Permission denied")
	}

	// For now, use the value as-is since stamp collection and embedding is complex
	// TODO: Implement proper stamp collection and embedding
	// stamped := c.service.stampManager.CollectStamps(writeMsg.Path, c.agentID)
	// embedded := c.service.stampManager.EmbedStamp(instance, stamped)

	// Write to storage
	err = c.service.tree.Write(writeMsg.Path, writeMsg.Value, writeMsg.Author)
	if err != nil {
		return c.createErrorResponse(msg.Sequence, 500, fmt.Sprintf("Write failed: %v", err))
	}

	// TODO: Trigger watchers once we know the correct method name

	// Create acknowledgment
	ackMsg := &protocol.WriteAckMessage{Success: true}
	payload, err := protocol.EncodeWriteAckMessage(ackMsg)
	if err != nil {
		return c.createErrorResponse(msg.Sequence, 500, fmt.Sprintf("Response encoding failed: %v", err))
	}

	return protocol.CreateMessage(protocol.WRITE_ACK, msg.Sequence, payload)
}

// handlePurge processes PURGE messages
func (c *Connection) handlePurge(msg *protocol.Message) *protocol.Message {
	purgeMsg, err := protocol.DecodePurgeMessage(msg.Payload)
	if err != nil {
		return c.createErrorResponse(msg.Sequence, 400, fmt.Sprintf("Invalid PURGE message: %v", err))
	}

	// Create agent object for permission checking
	agent := c.createAgent()

	// Check permissions (purge requires special permission)
	if !c.service.permEvaluator.CanPurge(agent, purgeMsg.Path) {
		return c.createErrorResponse(msg.Sequence, 403, "Permission denied")
	}

	// Perform purge
	err = c.service.tree.Purge(purgeMsg.Path, purgeMsg.From, purgeMsg.To, purgeMsg.Author)
	if err != nil {
		return c.createErrorResponse(msg.Sequence, 500, fmt.Sprintf("Purge failed: %v", err))
	}

	// Create acknowledgment
	ackMsg := &protocol.PurgeAckMessage{Success: true}
	payload, err := protocol.EncodePurgeAckMessage(ackMsg)
	if err != nil {
		return c.createErrorResponse(msg.Sequence, 500, fmt.Sprintf("Response encoding failed: %v", err))
	}

	return protocol.CreateMessage(protocol.PURGE_ACK, msg.Sequence, payload)
}

// handleReadAt processes READ_AT messages
func (c *Connection) handleReadAt(msg *protocol.Message) *protocol.Message {
	// For now, return not implemented
	return c.createErrorResponse(msg.Sequence, 501, "READ_AT not implemented yet")
}

// handleChildren processes CHILDREN messages
func (c *Connection) handleChildren(msg *protocol.Message) *protocol.Message {
	// For now, return not implemented
	return c.createErrorResponse(msg.Sequence, 501, "CHILDREN not implemented yet")
}

// handleStatus processes STATUS messages
func (c *Connection) handleStatus(msg *protocol.Message) *protocol.Message {
	// Only allow status requests from local connections (security)
	if !c.isLocal {
		return c.createErrorResponse(msg.Sequence, 403, "Status only available via local socket")
	}

	status := c.service.GetStatus()
	payload, err := protocol.EncodeStatusResponseMessage(&status)
	if err != nil {
		return c.createErrorResponse(msg.Sequence, 500, fmt.Sprintf("Response encoding failed: %v", err))
	}

	return protocol.CreateMessage(protocol.STATUS_RESPONSE, msg.Sequence, payload)
}

// handleStop processes STOP messages
func (c *Connection) handleStop(msg *protocol.Message) *protocol.Message {
	// Only allow stop requests from local connections (security)
	if !c.isLocal {
		return c.createErrorResponse(msg.Sequence, 403, "Stop only available via local socket")
	}

	// Trigger service shutdown in a separate goroutine
	go func() {
		c.service.Stop()
	}()

	// Create acknowledgment
	ackMsg := &protocol.StopAckMessage{Success: true}
	payload, err := protocol.EncodeStopAckMessage(ackMsg)
	if err != nil {
		return c.createErrorResponse(msg.Sequence, 500, fmt.Sprintf("Response encoding failed: %v", err))
	}

	return protocol.CreateMessage(protocol.STOP_ACK, msg.Sequence, payload)
}

// handleCompact processes COMPACT messages
func (c *Connection) handleCompact(msg *protocol.Message) *protocol.Message {
	// Only allow compact requests from local connections
	if !c.isLocal {
		return c.createErrorResponse(msg.Sequence, 403, "Compact only available via local socket")
	}

	// For now, return not implemented
	return c.createErrorResponse(msg.Sequence, 501, "COMPACT not implemented yet")
}

// createErrorResponse creates an error response message
func (c *Connection) createErrorResponse(sequence uint32, code uint32, message string) *protocol.Message {
	errorMsg := &protocol.ErrorMessage{
		Code:    code,
		Message: message,
	}

	payload, err := protocol.EncodeErrorMessage(errorMsg)
	if err != nil {
		// Fallback error response
		fallbackPayload, _ := protocol.EncodeErrorMessage(&protocol.ErrorMessage{
			Code:    500,
			Message: "Internal error creating error response",
		})
		return protocol.CreateMessage(protocol.ERROR, sequence, fallbackPayload)
	}

	return protocol.CreateMessage(protocol.ERROR, sequence, payload)
}

// sendResponse sends a response message to the client
func (c *Connection) sendResponse(msg *protocol.Message) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return fmt.Errorf("connection closed")
	}

	data, err := protocol.EncodeMessage(msg)
	if err != nil {
		return fmt.Errorf("message encoding failed: %w", err)
	}

	_, err = c.writer.Write(data)
	return err
}