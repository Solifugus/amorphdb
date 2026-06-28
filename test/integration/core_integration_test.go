package integration

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/solifugus/amorphdb/internal/protocol"
	"github.com/solifugus/amorphdb/internal/service"
	"github.com/solifugus/amorphdb/internal/storage"
	"github.com/solifugus/amorphdb/internal/types"
)

// TestCoreSystemIntegration tests the core AmorphDB functionality
func TestCoreSystemIntegration(t *testing.T) {
	// Create temporary directory for test
	tmpDir, err := os.MkdirTemp("", "amorphdb_core_test_")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	t.Log("=== Step 12.1: Core System Integration Test ===")

	// Test 1: Service lifecycle
	t.Log("Test 1: Service startup and configuration")
	service := startCoreService(t, tmpDir)
	defer service.Stop()

	// Verify service is running
	status := service.GetStatus()
	t.Logf("Service started - Node: %s, Uptime: %d seconds", status.NodeIdentity, status.Uptime)

	// Test 2: Storage operations via direct tree access
	t.Log("Test 2: Direct storage operations")
	testDirectStorageOperations(t, service)

	// Test 3: Protocol operations via client
	t.Log("Test 3: Protocol-based operations")
	client := connectCoreClient(t, service)
	defer client.Close()

	testProtocolOperations(t, client)

	// Test 4: Data persistence across restarts
	t.Log("Test 4: Data persistence across service restart")
	testDataPersistenceAcrossRestart(t, tmpDir)

	t.Log("=== Core System Integration Test PASSED ===")
}

// TestStorageEngineIntegration tests the storage engine directly
func TestStorageEngineIntegration(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "amorphdb_storage_test_")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	t.Log("=== Storage Engine Integration Test ===")

	// Create storage tree directly
	tree, err := storage.NewStorageTree(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create storage tree: %v", err)
	}
	defer tree.Close()

	// Test writing and reading different types
	testStorageTypes(t, tree)

	// Test temporal operations
	testTemporalStorage(t, tree)

	// Test complex nested paths
	testNestedPaths(t, tree)

	t.Log("=== Storage Engine Integration Test PASSED ===")
}

// TestProtocolIntegration tests the wire protocol specifically
func TestProtocolIntegration(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "amorphdb_protocol_test_")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	t.Log("=== Protocol Integration Test ===")

	service := startCoreService(t, tmpDir)
	defer service.Stop()

	client := connectCoreClient(t, service)
	defer client.Close()

	// Test all supported protocol messages
	testReadWriteProtocol(t, client)
	testStatusProtocol(t, client)
	testErrorHandling(t, client)

	t.Log("=== Protocol Integration Test PASSED ===")
}

// Helper functions

func startCoreService(t *testing.T, dataDir string) *service.Service {
	config := service.Config{
		StorageDir:      dataDir,
		LocalSocketPath: filepath.Join(dataDir, "amorphdb.sock"),
		NetworkPort:     0, // Random port for testing
		NodeIdentity:    "core-test-node",
	}

	svc, err := service.New(config)
	if err != nil {
		t.Fatalf("Failed to create service: %v", err)
	}

	err = svc.Start()
	if err != nil {
		t.Fatalf("Failed to start service: %v", err)
	}

	// Wait for service to be ready
	time.Sleep(100 * time.Millisecond)

	return svc
}

func connectCoreClient(t *testing.T, service *service.Service) *CoreTestClient {
	client := &CoreTestClient{
		socketPath: service.GetLocalSocketPath(),
		sequence:   1,
	}

	err := client.Connect()
	if err != nil {
		t.Fatalf("Failed to connect client: %v", err)
	}

	return client
}

func testDirectStorageOperations(t *testing.T, service *service.Service) {
	// Get direct access to the tree for testing
	tree := service.GetTree()

	// Write some test data
	textValue := storage.Value{
		Data:    []byte("Direct storage test"),
		TypeTag: storage.TypeText,
	}

	err := tree.Write([]string{"direct", "test", "text"}, textValue, 1000)
	if err != nil {
		t.Fatalf("Failed to write via direct storage: %v", err)
	}

	// Read it back
	readValue, err := tree.Read([]string{"direct", "test", "text"})
	if err != nil {
		t.Fatalf("Failed to read via direct storage: %v", err)
	}

	if string(readValue.Data) != "Direct storage test" {
		t.Errorf("Expected 'Direct storage test', got '%s'", string(readValue.Data))
	}

	t.Log("Direct storage operations working correctly")
}

func testProtocolOperations(t *testing.T, client *CoreTestClient) {
	// Write test data via protocol. Per spec §Defaults, world.* writes default to
	// (Nothing) — an agent may only write to its own home (~) without an explicit
	// grant, so this round-trip uses a ~-rooted path.
	textValue := &types.Text{Value: "Protocol test value"}
	err := client.Write([]string{"~", "protocol", "test", "value"}, textValue, 1000)
	if err != nil {
		t.Fatalf("Failed to write via protocol: %v", err)
	}

	// Read it back via protocol
	readValue, err := client.Read([]string{"~", "protocol", "test", "value"})
	if err != nil {
		t.Fatalf("Failed to read via protocol: %v", err)
	}

	readText, ok := readValue.(*types.Text)
	if !ok {
		t.Fatalf("Expected text value, got %T", readValue)
	}

	if readText.Value != "Protocol test value" {
		t.Errorf("Expected 'Protocol test value', got '%s'", readText.Value)
	}

	t.Log("Protocol operations working correctly")
}

func testDataPersistenceAcrossRestart(t *testing.T, dataDir string) {
	// Write test data with first service instance
	service1 := startCoreService(t, dataDir)
	client1 := connectCoreClient(t, service1)

	persistentValue := &types.Text{Value: "Persistent across restart"}
	err := client1.Write([]string{"~", "persistent", "data"}, persistentValue, 1000)
	if err != nil {
		t.Fatalf("Failed to write persistent data: %v", err)
	}

	client1.Close()
	service1.Stop()
	time.Sleep(100 * time.Millisecond) // Allow cleanup

	// Start new service instance with same data directory
	service2 := startCoreService(t, dataDir)
	defer service2.Stop()

	client2 := connectCoreClient(t, service2)
	defer client2.Close()

	// Read the persistent data
	readValue, err := client2.Read([]string{"~", "persistent", "data"})
	if err != nil {
		t.Fatalf("Failed to read persistent data after restart: %v", err)
	}

	readText := readValue.(*types.Text)
	if readText.Value != "Persistent across restart" {
		t.Errorf("Data not persistent across restart")
	}

	t.Log("Data persistence across restart working correctly")
}

func testStorageTypes(t *testing.T, tree storage.Tree) {
	// Test Text
	textValue := storage.Value{Data: []byte("Test text"), TypeTag: storage.TypeText}
	err := tree.Write([]string{"types", "text"}, textValue, 1000)
	if err != nil {
		t.Fatalf("Failed to write text: %v", err)
	}

	readText, err := tree.Read([]string{"types", "text"})
	if err != nil {
		t.Fatalf("Failed to read text: %v", err)
	}
	if string(readText.Data) != "Test text" {
		t.Errorf("Text mismatch")
	}

	// Test Number
	numberBytes := make([]byte, 8)
	numberBytes[0] = 0x40 // IEEE 754 representation of 2.0
	numberBytes[1] = 0x00
	numberValue := storage.Value{Data: numberBytes, TypeTag: storage.TypeNumber}
	err = tree.Write([]string{"types", "number"}, numberValue, 1000)
	if err != nil {
		t.Fatalf("Failed to write number: %v", err)
	}

	readNumber, err := tree.Read([]string{"types", "number"})
	if err != nil {
		t.Fatalf("Failed to read number: %v", err)
	}
	if len(readNumber.Data) != 8 {
		t.Errorf("Number size mismatch")
	}

	t.Log("Storage type operations working correctly")
}

func testTemporalStorage(t *testing.T, tree storage.Tree) {
	path := []string{"temporal", "value"}

	// Write first value and capture timestamp
	value1 := storage.Value{Data: []byte("First value"), TypeTag: storage.TypeText}
	err := tree.Write(path, value1, 1000) // author ID 1000
	if err != nil {
		t.Fatalf("Failed to write first temporal value: %v", err)
	}
	writeTime1 := time.Now().UTC().UnixMicro()

	// Wait a bit to ensure different timestamps
	time.Sleep(10 * time.Millisecond)

	// Write second value and capture timestamp
	value2 := storage.Value{Data: []byte("Second value"), TypeTag: storage.TypeText}
	err = tree.Write(path, value2, 1000) // author ID 1000
	if err != nil {
		t.Fatalf("Failed to write second temporal value: %v", err)
	}
	writeTime2 := time.Now().UTC().UnixMicro()

	t.Logf("Temporal test: writeTime1=%d, writeTime2=%d", writeTime1, writeTime2)

	// Read current value (should be latest)
	currentValue, err := tree.Read(path)
	if err != nil {
		t.Fatalf("Failed to read current value: %v", err)
	}
	if string(currentValue.Data) != "Second value" {
		t.Errorf("Current value mismatch: expected 'Second value', got '%s'", string(currentValue.Data))
	}

	// Read historical value just after first write (should get first value)
	readTime := writeTime1 + 1000 // 1ms after first write
	t.Logf("Reading at timestamp: %d (after first write)", readTime)
	historicalValue, err := tree.ReadAt(path, readTime)
	if err != nil {
		t.Fatalf("Failed to read historical value: %v", err)
	}
	if string(historicalValue.Data) != "First value" {
		t.Errorf("Historical value mismatch: expected 'First value', got '%s'", string(historicalValue.Data))
	}

	// Read historical value just after second write (should get second value)
	readTime2 := writeTime2 + 1000 // 1ms after second write
	historicalValue2, err := tree.ReadAt(path, readTime2)
	if err != nil {
		t.Fatalf("Failed to read second historical value: %v", err)
	}
	if string(historicalValue2.Data) != "Second value" {
		t.Errorf("Second historical value mismatch: expected 'Second value', got '%s'", string(historicalValue2.Data))
	}

	t.Log("Temporal storage operations working correctly")
}

func testNestedPaths(t *testing.T, tree storage.Tree) {
	// Create deeply nested structure
	deepPath := []string{"level1", "level2", "level3", "level4", "value"}
	deepValue := storage.Value{Data: []byte("Deep nested value"), TypeTag: storage.TypeText}

	err := tree.Write(deepPath, deepValue, 1000)
	if err != nil {
		t.Fatalf("Failed to write deep nested value: %v", err)
	}

	readDeepValue, err := tree.Read(deepPath)
	if err != nil {
		t.Fatalf("Failed to read deep nested value: %v", err)
	}

	if string(readDeepValue.Data) != "Deep nested value" {
		t.Errorf("Deep nested value mismatch")
	}

	t.Log("Nested path operations working correctly")
}

func testReadWriteProtocol(t *testing.T, client *CoreTestClient) {
	// Test writing and reading various types (use ~ path for default permissions)
	textVal := &types.Text{Value: "Protocol text test"}
	err := client.Write([]string{"~", "proto", "text"}, textVal, 1000)
	if err != nil {
		t.Fatalf("Protocol write failed: %v", err)
	}

	readVal, err := client.Read([]string{"~", "proto", "text"})
	if err != nil {
		t.Fatalf("Protocol read failed: %v", err)
	}

	if readVal.(*types.Text).Value != "Protocol text test" {
		t.Errorf("Protocol read/write mismatch")
	}

	// Test number (using integer to avoid protocol precision issues)
	numVal := &types.Number{Value: 42}
	err = client.Write([]string{"~", "proto", "number"}, numVal, 1000)
	if err != nil {
		t.Fatalf("Protocol number write failed: %v", err)
	}

	readNumVal, err := client.Read([]string{"~", "proto", "number"})
	if err != nil {
		t.Fatalf("Protocol number read failed: %v", err)
	}

	if readNumVal.(*types.Number).Value != 42 {
		t.Errorf("Protocol number read/write mismatch: expected 42, got %v", readNumVal.(*types.Number).Value)
	}

	t.Log("Read/write protocol working correctly")
}

func testStatusProtocol(t *testing.T, client *CoreTestClient) {
	status, err := client.GetStatus()
	if err != nil {
		t.Fatalf("Status protocol failed: %v", err)
	}

	if status.NodeIdentity == "" {
		t.Error("Expected non-empty node identity")
	}

	if status.Uptime < 0 {
		t.Error("Expected positive uptime")
	}

	t.Logf("Status protocol working - Node: %s, Uptime: %d", status.NodeIdentity, status.Uptime)
}

func testErrorHandling(t *testing.T, client *CoreTestClient) {
	// Try to read a path that doesn't exist
	_, err := client.Read([]string{"nonexistent", "path"})
	if err == nil {
		t.Error("Expected error when reading nonexistent path")
	}

	t.Log("Error handling working correctly")
}

// CoreTestClient - Simplified version of the protocol client
type CoreTestClient struct {
	conn       net.Conn
	socketPath string
	sequence   uint32
}

func (c *CoreTestClient) Connect() error {
	var err error
	c.conn, err = net.Dial("unix", c.socketPath)
	return err
}

func (c *CoreTestClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

func (c *CoreTestClient) Write(path []string, value types.Value, author uint64) error {
	// Convert types.Value to storage.Value
	storageValue := c.convertToStorageValue(value)

	writeMsg := &protocol.WriteMessage{
		Path:   path,
		Value:  storageValue,
		Author: author,
	}

	return c.sendMessage(protocol.WRITE, writeMsg, func(response *protocol.Message) error {
		if response.Type == protocol.ERROR {
			errorMsg, _ := protocol.DecodeErrorMessage(response.Payload)
			return fmt.Errorf("write failed: %s", errorMsg.Message)
		}
		return nil
	})
}

func (c *CoreTestClient) Read(path []string) (types.Value, error) {
	readMsg := &protocol.ReadMessage{Path: path}

	var result types.Value

	err := c.sendMessage(protocol.READ, readMsg, func(response *protocol.Message) error {
		if response.Type == protocol.ERROR {
			errorMsg, _ := protocol.DecodeErrorMessage(response.Payload)
			return fmt.Errorf("read failed: %s", errorMsg.Message)
		}

		if response.Type != protocol.READ_RESPONSE {
			return fmt.Errorf("unexpected response type: 0x%02x", response.Type)
		}

		readResponse, err := protocol.DecodeReadResponseMessage(response.Payload)
		if err != nil {
			return fmt.Errorf("failed to decode read response: %w", err)
		}

		result = c.convertFromStorageValue(readResponse.Value)
		return nil
	})

	return result, err
}

func (c *CoreTestClient) GetStatus() (*protocol.StatusResponseMessage, error) {
	var result *protocol.StatusResponseMessage

	err := c.sendMessage(protocol.STATUS, &protocol.StatusMessage{}, func(response *protocol.Message) error {
		if response.Type == protocol.ERROR {
			errorMsg, _ := protocol.DecodeErrorMessage(response.Payload)
			return fmt.Errorf("status failed: %s", errorMsg.Message)
		}

		if response.Type != protocol.STATUS_RESPONSE {
			return fmt.Errorf("unexpected response type: 0x%02x", response.Type)
		}

		statusResponse, err := protocol.DecodeStatusResponseMessage(response.Payload)
		if err != nil {
			return fmt.Errorf("failed to decode status response: %w", err)
		}

		result = statusResponse
		return nil
	})

	return result, err
}

func (c *CoreTestClient) sendMessage(msgType uint8, msgData interface{}, handler func(*protocol.Message) error) error {
	var payload []byte
	var err error

	switch msgType {
	case protocol.WRITE:
		payload, err = protocol.EncodeWriteMessage(msgData.(*protocol.WriteMessage))
	case protocol.READ:
		payload, err = protocol.EncodeReadMessage(msgData.(*protocol.ReadMessage))
	case protocol.STATUS:
		payload, err = protocol.EncodeStatusMessage(msgData.(*protocol.StatusMessage))
	default:
		return fmt.Errorf("unsupported message type: 0x%02x", msgType)
	}

	if err != nil {
		return fmt.Errorf("failed to encode message: %w", err)
	}

	msg := protocol.CreateMessage(msgType, c.sequence, payload)
	c.sequence++

	data, err := protocol.EncodeMessage(msg)
	if err != nil {
		return fmt.Errorf("failed to encode message: %w", err)
	}

	_, err = c.conn.Write(data)
	if err != nil {
		return fmt.Errorf("failed to send message: %w", err)
	}

	response, err := c.readResponse()
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	return handler(response)
}

func (c *CoreTestClient) readResponse() (*protocol.Message, error) {
	// Read message header (10 bytes: version + type + sequence + length)
	headerBytes := make([]byte, 10)
	_, err := c.conn.Read(headerBytes)
	if err != nil {
		return nil, err
	}

	// Parse payload length from header
	payloadLength := uint32(headerBytes[6])<<24 | uint32(headerBytes[7])<<16 |
		uint32(headerBytes[8])<<8 | uint32(headerBytes[9])

	// Read payload
	payload := make([]byte, payloadLength)
	_, err = c.conn.Read(payload)
	if err != nil {
		return nil, err
	}

	// Read checksum (4 bytes)
	checksumBytes := make([]byte, 4)
	_, err = c.conn.Read(checksumBytes)
	if err != nil {
		return nil, err
	}

	// Reconstruct full message
	fullMessage := make([]byte, 0, 14+payloadLength)
	fullMessage = append(fullMessage, headerBytes...)
	fullMessage = append(fullMessage, payload...)
	fullMessage = append(fullMessage, checksumBytes...)

	return protocol.DecodeMessage(fullMessage)
}

func (c *CoreTestClient) convertToStorageValue(value types.Value) storage.Value {
	switch v := value.(type) {
	case *types.Text:
		return storage.Value{Data: []byte(v.Value), TypeTag: storage.TypeText}
	case *types.Number:
		// Convert float64 to bytes
		data := make([]byte, 8)
		bits := uint64(v.Value) // Simplified conversion
		for i := 0; i < 8; i++ {
			data[i] = byte(bits >> (8 * (7 - i)))
		}
		return storage.Value{Data: data, TypeTag: storage.TypeNumber}
	default:
		return storage.Value{Data: []byte("unknown"), TypeTag: storage.TypeUnknown}
	}
}

func (c *CoreTestClient) convertFromStorageValue(value storage.Value) types.Value {
	switch value.TypeTag {
	case storage.TypeText:
		return &types.Text{Value: string(value.Data)}
	case storage.TypeNumber:
		// Simplified number conversion
		var bits uint64
		for i := 0; i < 8 && i < len(value.Data); i++ {
			bits |= uint64(value.Data[i]) << (8 * (7 - i))
		}
		return &types.Number{Value: float64(bits)}
	default:
		return &types.Text{Value: string(value.Data)}
	}
}
