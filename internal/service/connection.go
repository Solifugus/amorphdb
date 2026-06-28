package service

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"

	"github.com/solifugus/amorphdb/internal/mbl/interpreter"
	"github.com/solifugus/amorphdb/internal/protocol"
	"github.com/solifugus/amorphdb/internal/security"
	"github.com/solifugus/amorphdb/internal/storage"
	"github.com/solifugus/amorphdb/internal/types"
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
	case protocol.CREATE_MESH:
		return c.handleCreateMesh(msg)
	case protocol.MESH_STATUS:
		return c.handleMeshStatus(msg)
	case protocol.JOIN_MESH:
		return c.handleJoinMesh(msg)
	case protocol.CREATE_BRIDGE:
		return c.handleCreateBridge(msg)
	case protocol.EXECUTE:
		return c.handleExecute(msg)
	case protocol.EXTRACT:
		return c.handleExtract(msg)
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

// handleCreateMesh processes CREATE_MESH messages
func (c *Connection) handleCreateMesh(msg *protocol.Message) *protocol.Message {
	// Only allow mesh operations from local connections
	if !c.isLocal {
		return c.createErrorResponse(msg.Sequence, 403, "Mesh operations only available via local socket")
	}

	responseData, err := c.service.meshService.CreateMesh(msg.Payload)
	if err != nil {
		return c.createErrorResponse(msg.Sequence, 500, fmt.Sprintf("Mesh creation failed: %v", err))
	}

	return protocol.CreateMessage(protocol.CREATE_MESH_ACK, msg.Sequence, responseData)
}

// handleMeshStatus processes MESH_STATUS messages
func (c *Connection) handleMeshStatus(msg *protocol.Message) *protocol.Message {
	// Only allow mesh operations from local connections
	if !c.isLocal {
		return c.createErrorResponse(msg.Sequence, 403, "Mesh operations only available via local socket")
	}

	responseData, err := c.service.meshService.GetMeshStatus(msg.Payload)
	if err != nil {
		return c.createErrorResponse(msg.Sequence, 500, fmt.Sprintf("Mesh status failed: %v", err))
	}

	return protocol.CreateMessage(protocol.MESH_STATUS_RESPONSE, msg.Sequence, responseData)
}

// handleJoinMesh processes JOIN_MESH messages
func (c *Connection) handleJoinMesh(msg *protocol.Message) *protocol.Message {
	// Only allow mesh operations from local connections
	if !c.isLocal {
		return c.createErrorResponse(msg.Sequence, 403, "Mesh operations only available via local socket")
	}

	responseData, err := c.service.meshService.JoinMesh(msg.Payload)
	if err != nil {
		return c.createErrorResponse(msg.Sequence, 500, fmt.Sprintf("Mesh join failed: %v", err))
	}

	return protocol.CreateMessage(protocol.JOIN_MESH_ACK, msg.Sequence, responseData)
}

// handleCreateBridge processes CREATE_BRIDGE messages
func (c *Connection) handleCreateBridge(msg *protocol.Message) *protocol.Message {
	// Only allow bridge operations from local connections
	if !c.isLocal {
		return c.createErrorResponse(msg.Sequence, 403, "Bridge operations only available via local socket")
	}

	responseData, err := c.service.meshService.CreateBridge(msg.Payload)
	if err != nil {
		return c.createErrorResponse(msg.Sequence, 500, fmt.Sprintf("Bridge creation failed: %v", err))
	}

	return protocol.CreateMessage(protocol.CREATE_BRIDGE_ACK, msg.Sequence, responseData)
}

// handleExecute processes EXECUTE messages
func (c *Connection) handleExecute(msg *protocol.Message) *protocol.Message {
	executeMsg, err := protocol.DecodeExecuteMessage(msg.Payload)
	if err != nil {
		return c.createErrorResponse(msg.Sequence, 400, fmt.Sprintf("Invalid EXECUTE message: %v", err))
	}

	// Execute the MBL code using the connection's interpreter
	result, err := c.interpreter.EvaluateExpression(executeMsg.Code)

	var responseMsg *protocol.ExecuteResponseMessage
	if err != nil {
		// Execution failed
		responseMsg = &protocol.ExecuteResponseMessage{
			Success: false,
			Result:  storage.Value{}, // Empty value for error case
			Error:   err.Error(),
		}
	} else {
		// Execution succeeded - convert result to storage.Value
		storageValue, err := c.convertToStorageValue(result)
		if err != nil {
			responseMsg = &protocol.ExecuteResponseMessage{
				Success: false,
				Result:  storage.Value{}, // Empty value for error case
				Error:   fmt.Sprintf("Failed to serialize result: %v", err),
			}
		} else {
			responseMsg = &protocol.ExecuteResponseMessage{
				Success: true,
				Result:  storageValue,
				Error:   "",
			}
		}
	}

	// Encode the response
	payload, err := protocol.EncodeExecuteResponseMessage(responseMsg)
	if err != nil {
		return c.createErrorResponse(msg.Sequence, 500, fmt.Sprintf("Response encoding failed: %v", err))
	}

	return protocol.CreateMessage(protocol.EXECUTE_RESPONSE, msg.Sequence, payload)
}

// convertToStorageValue converts an interpreter result to storage.Value
func (c *Connection) convertToStorageValue(result interface{}) (storage.Value, error) {
	// This conversion depends on the types used by the interpreter and storage
	// For now, we'll use a basic approach - this may need refinement based on actual types
	if result == nil {
		return storage.Value{}, nil
	}

	// The actual implementation will depend on how the interpreter types map to storage.Value
	// This is a placeholder that should be updated based on the types system
	return storage.Value{
		Data:    []byte(fmt.Sprintf("%v", result)),
		TypeTag: 1, // This should be determined based on the actual type
	}, nil
}

// handleExtract processes EXTRACT messages
func (c *Connection) handleExtract(msg *protocol.Message) *protocol.Message {
	extractMsg, err := protocol.DecodeExtractMessage(msg.Payload)
	if err != nil {
		return c.createErrorResponse(msg.Sequence, 400, fmt.Sprintf("Invalid EXTRACT message: %v", err))
	}

	// Generate MBL script by walking the tree from the specified path
	script, err := c.generateMBLScript(extractMsg.Path)

	var responseMsg *protocol.ExtractResponseMessage
	if err != nil {
		// Extraction failed
		responseMsg = &protocol.ExtractResponseMessage{
			Success: false,
			Script:  "",
			Error:   err.Error(),
		}
	} else {
		// Extraction succeeded
		responseMsg = &protocol.ExtractResponseMessage{
			Success: true,
			Script:  script,
			Error:   "",
		}
	}

	// Encode the response
	payload, err := protocol.EncodeExtractResponseMessage(responseMsg)
	if err != nil {
		return c.createErrorResponse(msg.Sequence, 500, fmt.Sprintf("Response encoding failed: %v", err))
	}

	return protocol.CreateMessage(protocol.EXTRACT_RESPONSE, msg.Sequence, payload)
}

// generateMBLScript walks the tree and generates MBL script to recreate the current state
func (c *Connection) generateMBLScript(rootPath []string) (string, error) {
	var script strings.Builder

	// For now, just try to extract the value at the specified path
	// TODO: Add recursive tree walking when storage.Attribute API is enhanced with name resolution
	value, err := c.service.tree.Read(rootPath)
	if err != nil {
		return "", fmt.Errorf("failed to read path %v: %w", rootPath, err)
	}

	// Generate assignment for this value
	assignment, err := c.generateAssignment(rootPath, value)
	if err != nil {
		return "", fmt.Errorf("failed to generate assignment for path %v: %w", rootPath, err)
	}

	script.WriteString(assignment)
	script.WriteString("\n")

	return script.String(), nil
}

// generateAssignment creates an MBL assignment statement for a given path and value
func (c *Connection) generateAssignment(path []string, value storage.Value) (string, error) {
	pathStr := strings.Join(path, ".")

	// Convert storage.Value to MBL literal syntax based on type
	mblValue, err := c.formatValueForMBL(value)
	if err != nil {
		return "", fmt.Errorf("failed to format value: %w", err)
	}

	return fmt.Sprintf("%s = %s", pathStr, mblValue), nil
}

// formatValueForMBL converts a storage.Value to MBL literal syntax
func (c *Connection) formatValueForMBL(value storage.Value) (string, error) {
	// Import the types package functions to deserialize values
	switch value.TypeTag {
	case 0x01: // TypeText
		if text, err := types.DeserializeText(value.Data); err == nil {
			// Escape quotes in text and wrap in quotes
			escaped := strings.ReplaceAll(text.Value, "\"", "\\\"")
			return fmt.Sprintf("\"%s\"", escaped), nil
		}
	case 0x02: // TypeNumber
		if number, err := types.DeserializeNumber(value.Data); err == nil {
			return fmt.Sprintf("%g", number.Value), nil
		}
	case 0x03: // TypeTime
		if timeVal, err := types.DeserializeTime(value.Data); err == nil {
			// Use @ prefix for time literals (no quotes per spec)
			return fmt.Sprintf("@%s", timeVal.Timestamp.Format("2006-01-02 15:04:05")), nil
		}
	case 0x04: // TypeMoney
		if money, err := types.DeserializeMoney(value.Data); err == nil {
			// Use currency symbol prefix (number first, then currency code per spec)
			return fmt.Sprintf("¤%.2f %s", money.Amount, money.CurrencyCode), nil
		}
	case 0x0A: // TypeBoolean
		if boolean, err := types.DeserializeBoolean(value.Data); err == nil {
			if boolean.Value {
				return "true", nil
			}
			return "false", nil
		}
	case 0x06: // TypeReference
		if reference, err := types.DeserializeReference(value.Data); err == nil {
			// Use (link) prefix for references - resolve AttributeID to path per spec
			path, err := c.resolveAttributeIDToPath(reference.AttributeID)
			if err != nil {
				// Fallback to internal ID if resolution fails
				return fmt.Sprintf("(link) &%d", reference.AttributeID), nil
			}
			return fmt.Sprintf("(link)%s", path), nil
		}
	case 0xF0: // TypeNothing
		return "Nothing", nil
	case 0xF1: // TypeUnknown
		if unknown, err := types.DeserializeUnknown(value.Data); err == nil {
			return fmt.Sprintf("unknown(\"%s\")", unknown.Reason), nil
		}
	}

	// Fallback for unsupported or complex types
	return fmt.Sprintf("unknown(\"unsupported type: 0x%02x\")", value.TypeTag), nil
}

// resolveAttributeIDToPath resolves an attribute ID back to its path string
func (c *Connection) resolveAttributeIDToPath(attributeID uint64) (string, error) {
	// Access the storage tree through the service
	tree := c.service.tree

	// Use the ResolveAttributePath method from the ExtendedTree interface
	pathStr, err := tree.ResolveAttributePath(attributeID)
	if err != nil {
		return "", fmt.Errorf("failed to resolve attribute path: %w", err)
	}

	return pathStr, nil
}