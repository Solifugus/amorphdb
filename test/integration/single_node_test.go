package integration

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/solifugus/amorphdb/internal/protocol"
	"github.com/solifugus/amorphdb/internal/service"
	"github.com/solifugus/amorphdb/internal/types"
)

// TestSingleNodeLifecycle tests the complete service lifecycle
func TestSingleNodeLifecycle(t *testing.T) {
	// Create temporary directory for test
	tmpDir, err := os.MkdirTemp("", "amorphdb_integration_test_")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Phase 1: Start service and perform basic operations
	t.Log("Phase 1: Starting service and testing basic operations")

	service := startTestService(t, tmpDir)
	defer service.Stop()

	client := connectTestClient(t, service)
	defer client.Close()

	// Test basic write/read operations via protocol
	testBasicDataOperations(t, client)

	// Test that data persists in storage
	testDataPersistence(t, client)

	// Phase 2: Stop and restart service
	t.Log("Phase 2: Testing service restart and data persistence")

	client.Close()
	service.Stop()
	time.Sleep(200 * time.Millisecond) // Allow cleanup

	// Restart service with same data directory
	service2 := startTestService(t, tmpDir)
	defer service2.Stop()

	client2 := connectTestClient(t, service2)
	defer client2.Close()

	// Verify data survived restart
	testDataSurvivedRestart(t, client2)

	t.Log("Single node lifecycle test passed!")
}

// TestBasicProtocolOperations tests READ/WRITE protocol messages
func TestBasicProtocolOperations(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "amorphdb_protocol_test_")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	service := startTestService(t, tmpDir)
	defer service.Stop()

	client := connectTestClient(t, service)
	defer client.Close()

	// Test writing text value
	textValue := &types.Text{Value: "Integration Test Value"}
	err = client.Write([]string{"my", "test", "name"}, textValue, 1000)
	if err != nil {
		t.Fatalf("Failed to write text value: %v", err)
	}

	// Test reading text value back
	readValue, err := client.Read([]string{"my", "test", "name"})
	if err != nil {
		t.Fatalf("Failed to read text value: %v", err)
	}

	readText, ok := readValue.(*types.Text)
	if !ok {
		t.Fatalf("Expected text value, got %T", readValue)
	}

	if readText.Value != "Integration Test Value" {
		t.Errorf("Expected 'Integration Test Value', got '%s'", readText.Value)
	}

	// Test writing number value
	numberValue := &types.Number{Value: 42.5}
	err = client.Write([]string{"my", "test", "number"}, numberValue, 1000)
	if err != nil {
		t.Fatalf("Failed to write number value: %v", err)
	}

	// Test reading number value back
	readNumberValue, err := client.Read([]string{"my", "test", "number"})
	if err != nil {
		t.Fatalf("Failed to read number value: %v", err)
	}

	readNumber, ok := readNumberValue.(*types.Number)
	if !ok {
		t.Fatalf("Expected number value, got %T", readNumberValue)
	}

	if readNumber.Value != 42.5 {
		t.Errorf("Expected 42.5, got %f", readNumber.Value)
	}

	t.Log("Basic protocol operations test passed!")
}

// TestServiceStatus tests the STATUS protocol message
func TestServiceStatus(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "amorphdb_status_test_")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	service := startTestService(t, tmpDir)
	defer service.Stop()

	client := connectTestClient(t, service)
	defer client.Close()

	// Request service status
	status, err := client.GetStatus()
	if err != nil {
		t.Fatalf("Failed to get service status: %v", err)
	}

	// Verify status information
	if status.NodeIdentity == "" {
		t.Error("Expected non-empty node identity")
	}

	if status.LocalSocket == "" {
		t.Error("Expected non-empty local socket path")
	}

	if status.Uptime < 0 {
		t.Error("Expected positive uptime")
	}

	t.Logf("Service status: Node=%s, Uptime=%d seconds, Connections=%d",
		status.NodeIdentity, status.Uptime, status.Connections)

	t.Log("Service status test passed!")
}

// TestDirectInterpreterAccess tests the interpreter functionality
func TestDirectInterpreterAccess(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "amorphdb_interpreter_test_")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	service := startTestService(t, tmpDir)
	defer service.Stop()

	// Access the interpreter directly for testing procedures and watchers
	// This is testing at a lower level than the protocol
	interp := service.GetInterpreter(1000) // Agent ID 1000

	// Test basic expression evaluation
	result, err := interp.EvaluateExpression("1 + 2 * 3")
	if err != nil {
		t.Fatalf("Failed to evaluate expression: %v", err)
	}

	if result.(*types.Number).Value != 7.0 {
		t.Errorf("Expected 7, got %v", result)
	}

	// Test procedure definition and execution
	_, err = interp.ExecuteStatement(`
my.test.add(a, b):
	return a + b
`)
	if err != nil {
		t.Fatalf("Failed to define procedure: %v", err)
	}

	// Test calling the procedure
	result, err = interp.EvaluateExpression("my.test.add(5, 3)")
	if err != nil {
		t.Fatalf("Failed to call procedure: %v", err)
	}

	if result.(*types.Number).Value != 8.0 {
		t.Errorf("Expected 8, got %v", result)
	}

	// Test watcher definition
	_, err = interp.ExecuteStatement(`
watch my.test.watcher:
	watching: my.test.counter
	my.test.double = my.test.counter * 2
`)
	if err != nil {
		t.Fatalf("Failed to define watcher: %v", err)
	}

	// Set the watched value
	_, err = interp.ExecuteStatement("my.test.counter = 10")
	if err != nil {
		t.Fatalf("Failed to set counter: %v", err)
	}

	// Allow watcher to fire
	time.Sleep(500 * time.Millisecond)

	// Check if watcher fired
	doubleResult, err := interp.EvaluateExpression("my.test.double")
	if err != nil {
		t.Fatalf("Failed to get double value: %v", err)
	}

	if doubleResult.(*types.Number).Value != 20.0 {
		t.Errorf("Expected watcher to set double to 20, got %v", doubleResult)
	}

	t.Log("Direct interpreter access test passed!")
}

// Helper functions

func startTestService(t *testing.T, dataDir string) *service.Service {
	config := service.Config{
		StorageDir:      dataDir,
		LocalSocketPath: filepath.Join(dataDir, "amorphdb.sock"),
		NetworkPort:     0, // Random port for testing
		NodeIdentity:    "test-node",
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

func connectTestClient(t *testing.T, service *service.Service) *TestProtocolClient {
	client := &TestProtocolClient{
		socketPath: service.GetLocalSocketPath(),
		sequence:   1,
	}

	err := client.Connect()
	if err != nil {
		t.Fatalf("Failed to connect client: %v", err)
	}

	return client
}

func testBasicDataOperations(t *testing.T, client *TestProtocolClient) {
	// Write test data
	textValue := &types.Text{Value: "Integration Test"}
	err := client.Write([]string{"my", "test", "name"}, textValue, 1000)
	if err != nil {
		t.Fatalf("Failed to write test data: %v", err)
	}

	// Read it back
	result, err := client.Read([]string{"my", "test", "name"})
	if err != nil {
		t.Fatalf("Failed to read test data: %v", err)
	}

	resultText, ok := result.(*types.Text)
	if !ok {
		t.Fatalf("Expected text result, got %T", result)
	}

	if resultText.Value != "Integration Test" {
		t.Errorf("Expected 'Integration Test', got '%s'", resultText.Value)
	}

	// Write nested data
	personName := &types.Text{Value: "Alice"}
	err = client.Write([]string{"my", "test", "person", "name"}, personName, 1000)
	if err != nil {
		t.Fatalf("Failed to write person name: %v", err)
	}

	personAge := &types.Number{Value: 30.0}
	err = client.Write([]string{"my", "test", "person", "age"}, personAge, 1000)
	if err != nil {
		t.Fatalf("Failed to write person age: %v", err)
	}

	// Read back nested data
	nameResult, err := client.Read([]string{"my", "test", "person", "name"})
	if err != nil {
		t.Fatalf("Failed to read person name: %v", err)
	}

	if nameResult.(*types.Text).Value != "Alice" {
		t.Errorf("Expected 'Alice', got '%s'", nameResult.(*types.Text).Value)
	}

	ageResult, err := client.Read([]string{"my", "test", "person", "age"})
	if err != nil {
		t.Fatalf("Failed to read person age: %v", err)
	}

	if ageResult.(*types.Number).Value != 30.0 {
		t.Errorf("Expected 30, got %f", ageResult.(*types.Number).Value)
	}
}

func testDataPersistence(t *testing.T, client *TestProtocolClient) {
	// Write data that should survive restart
	persistentData := &types.Text{Value: "This should survive restart"}
	err := client.Write([]string{"my", "persistent", "data"}, persistentData, 1000)
	if err != nil {
		t.Fatalf("Failed to write persistent data: %v", err)
	}

	persistentNumber := &types.Number{Value: 42.0}
	err = client.Write([]string{"my", "persistent", "number"}, persistentNumber, 1000)
	if err != nil {
		t.Fatalf("Failed to write persistent number: %v", err)
	}
}

func testDataSurvivedRestart(t *testing.T, client *TestProtocolClient) {
	// Check that data written before restart is still there
	result, err := client.Read([]string{"my", "persistent", "data"})
	if err != nil {
		t.Fatalf("Failed to read persistent data after restart: %v", err)
	}

	if result.(*types.Text).Value != "This should survive restart" {
		t.Errorf("Persistent data not preserved across restart")
	}

	numberResult, err := client.Read([]string{"my", "persistent", "number"})
	if err != nil {
		t.Fatalf("Failed to read persistent number after restart: %v", err)
	}

	if numberResult.(*types.Number).Value != 42.0 {
		t.Errorf("Expected persistent number 42, got %f", numberResult.(*types.Number).Value)
	}

	// Also check that basic test data is still there
	nameResult, err := client.Read([]string{"my", "test", "person", "name"})
	if err != nil {
		t.Fatalf("Failed to read test data after restart: %v", err)
	}

	if nameResult.(*types.Text).Value != "Alice" {
		t.Errorf("Test data not preserved across restart")
	}
}

// TestProtocolClient provides a simple protocol client for integration testing
type TestProtocolClient struct {
	conn       net.Conn
	socketPath string
	sequence   uint32
}

func (c *TestProtocolClient) Connect() error {
	var err error
	c.conn, err = net.Dial("unix", c.socketPath)
	return err
}

func (c *TestProtocolClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

func (c *TestProtocolClient) Write(path []string, value types.Value, author uint64) error {
	writeMsg := &protocol.WriteMessage{
		Path:   path,
		Value:  value,
		Author: author,
	}

	payload, err := protocol.EncodeWriteMessage(writeMsg)
	if err != nil {
		return fmt.Errorf("failed to encode write message: %w", err)
	}

	msg := protocol.CreateMessage(protocol.WRITE, c.sequence, payload)
	c.sequence++

	data, err := protocol.EncodeMessage(msg)
	if err != nil {
		return fmt.Errorf("failed to encode message: %w", err)
	}

	_, err = c.conn.Write(data)
	if err != nil {
		return fmt.Errorf("failed to send message: %w", err)
	}

	// Read response
	response, err := c.readResponse()
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	if response.Type == protocol.ERROR {
		errorMsg, err := protocol.DecodeErrorMessage(response.Payload)
		if err != nil {
			return fmt.Errorf("write failed with unknown error")
		}
		return fmt.Errorf("write failed: %s", errorMsg.Message)
	}

	return nil
}

func (c *TestProtocolClient) Read(path []string) (types.Value, error) {
	readMsg := &protocol.ReadMessage{Path: path}

	payload, err := protocol.EncodeReadMessage(readMsg)
	if err != nil {
		return nil, fmt.Errorf("failed to encode read message: %w", err)
	}

	msg := protocol.CreateMessage(protocol.READ, c.sequence, payload)
	c.sequence++

	data, err := protocol.EncodeMessage(msg)
	if err != nil {
		return nil, fmt.Errorf("failed to encode message: %w", err)
	}

	_, err = c.conn.Write(data)
	if err != nil {
		return nil, fmt.Errorf("failed to send message: %w", err)
	}

	// Read response
	response, err := c.readResponse()
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if response.Type == protocol.ERROR {
		errorMsg, err := protocol.DecodeErrorMessage(response.Payload)
		if err != nil {
			return nil, fmt.Errorf("read failed with unknown error")
		}
		return nil, fmt.Errorf("read failed: %s", errorMsg.Message)
	}

	if response.Type != protocol.READ_RESPONSE {
		return nil, fmt.Errorf("unexpected response type: 0x%02x", response.Type)
	}

	readResponse, err := protocol.DecodeReadResponseMessage(response.Payload)
	if err != nil {
		return nil, fmt.Errorf("failed to decode read response: %w", err)
	}

	return readResponse.Value, nil
}

func (c *TestProtocolClient) GetStatus() (*protocol.StatusResponseMessage, error) {
	msg := protocol.CreateMessage(protocol.STATUS, c.sequence, []byte{})
	c.sequence++

	data, err := protocol.EncodeMessage(msg)
	if err != nil {
		return nil, fmt.Errorf("failed to encode message: %w", err)
	}

	_, err = c.conn.Write(data)
	if err != nil {
		return nil, fmt.Errorf("failed to send message: %w", err)
	}

	// Read response
	response, err := c.readResponse()
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if response.Type == protocol.ERROR {
		errorMsg, err := protocol.DecodeErrorMessage(response.Payload)
		if err != nil {
			return nil, fmt.Errorf("status request failed with unknown error")
		}
		return nil, fmt.Errorf("status request failed: %s", errorMsg.Message)
	}

	if response.Type != protocol.STATUS_RESPONSE {
		return nil, fmt.Errorf("unexpected response type: 0x%02x", response.Type)
	}

	statusResponse, err := protocol.DecodeStatusResponseMessage(response.Payload)
	if err != nil {
		return nil, fmt.Errorf("failed to decode status response: %w", err)
	}

	return statusResponse, nil
}

func (c *TestProtocolClient) readResponse() (*protocol.Message, error) {
	// Read message header (14 bytes: version + type + sequence + length)
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

	// Decode message
	return protocol.DecodeMessage(fullMessage)
}
