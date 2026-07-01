package protocol

import (
	"reflect"
	"testing"

	"github.com/solifugus/amorphdb/internal/storage"
)

func TestMessageRoundTrip(t *testing.T) {
	// Test basic message encoding/decoding
	payload := []byte("test payload")
	msg := CreateMessage(READ, 12345, payload)

	// Encode
	encoded, err := EncodeMessage(msg)
	if err != nil {
		t.Fatalf("Failed to encode message: %v", err)
	}

	// Decode
	decoded, err := DecodeMessage(encoded)
	if err != nil {
		t.Fatalf("Failed to decode message: %v", err)
	}

	// Verify
	if decoded.Version != msg.Version {
		t.Errorf("Version mismatch: expected %d, got %d", msg.Version, decoded.Version)
	}
	if decoded.Type != msg.Type {
		t.Errorf("Type mismatch: expected %d, got %d", msg.Type, decoded.Type)
	}
	if decoded.Sequence != msg.Sequence {
		t.Errorf("Sequence mismatch: expected %d, got %d", msg.Sequence, decoded.Sequence)
	}
	if decoded.PayloadLength != msg.PayloadLength {
		t.Errorf("PayloadLength mismatch: expected %d, got %d", msg.PayloadLength, decoded.PayloadLength)
	}
	if !reflect.DeepEqual(decoded.Payload, msg.Payload) {
		t.Errorf("Payload mismatch: expected %v, got %v", msg.Payload, decoded.Payload)
	}
	if decoded.Checksum != msg.Checksum {
		t.Errorf("Checksum mismatch: expected %d, got %d", msg.Checksum, decoded.Checksum)
	}
}

func TestReadMessageRoundTrip(t *testing.T) {
	original := &ReadMessage{
		Path: []string{"my", "test", "path"},
	}

	// Encode
	encoded, err := EncodeReadMessage(original)
	if err != nil {
		t.Fatalf("Failed to encode ReadMessage: %v", err)
	}

	// Decode
	decoded, err := DecodeReadMessage(encoded)
	if err != nil {
		t.Fatalf("Failed to decode ReadMessage: %v", err)
	}

	// Verify
	if !reflect.DeepEqual(decoded.Path, original.Path) {
		t.Errorf("Path mismatch: expected %v, got %v", original.Path, decoded.Path)
	}
}

func TestReadResponseMessageRoundTrip(t *testing.T) {
	original := &ReadResponseMessage{
		Value: storage.Value{
			Data:    []byte("test value"),
			TypeTag: storage.TypeText,
		},
	}

	// Encode
	encoded, err := EncodeReadResponseMessage(original)
	if err != nil {
		t.Fatalf("Failed to encode ReadResponseMessage: %v", err)
	}

	// Decode
	decoded, err := DecodeReadResponseMessage(encoded)
	if err != nil {
		t.Fatalf("Failed to decode ReadResponseMessage: %v", err)
	}

	// Verify
	if !reflect.DeepEqual(decoded.Value, original.Value) {
		t.Errorf("Value mismatch: expected %v, got %v", original.Value, decoded.Value)
	}
}

func TestWriteMessageRoundTrip(t *testing.T) {
	original := &WriteMessage{
		Path: []string{"test", "write"},
		Value: storage.Value{
			Data:    []byte("42"),
			TypeTag: storage.TypeNumber,
		},
		Author: 1000,
	}

	// Encode
	encoded, err := EncodeWriteMessage(original)
	if err != nil {
		t.Fatalf("Failed to encode WriteMessage: %v", err)
	}

	// Decode
	decoded, err := DecodeWriteMessage(encoded)
	if err != nil {
		t.Fatalf("Failed to decode WriteMessage: %v", err)
	}

	// Verify
	if !reflect.DeepEqual(decoded.Path, original.Path) {
		t.Errorf("Path mismatch: expected %v, got %v", original.Path, decoded.Path)
	}
	if !reflect.DeepEqual(decoded.Value, original.Value) {
		t.Errorf("Value mismatch: expected %v, got %v", original.Value, decoded.Value)
	}
	if decoded.Author != original.Author {
		t.Errorf("Author mismatch: expected %d, got %d", original.Author, decoded.Author)
	}
}

func TestWriteAckMessageRoundTrip(t *testing.T) {
	tests := []struct {
		name     string
		original *WriteAckMessage
	}{
		{
			name: "success",
			original: &WriteAckMessage{
				Success: true,
				Error:   "",
			},
		},
		{
			name: "error",
			original: &WriteAckMessage{
				Success: false,
				Error:   "test error message",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Encode
			encoded, err := EncodeWriteAckMessage(tt.original)
			if err != nil {
				t.Fatalf("Failed to encode WriteAckMessage: %v", err)
			}

			// Decode
			decoded, err := DecodeWriteAckMessage(encoded)
			if err != nil {
				t.Fatalf("Failed to decode WriteAckMessage: %v", err)
			}

			// Verify
			if decoded.Success != tt.original.Success {
				t.Errorf("Success mismatch: expected %v, got %v", tt.original.Success, decoded.Success)
			}
			if decoded.Error != tt.original.Error {
				t.Errorf("Error mismatch: expected %s, got %s", tt.original.Error, decoded.Error)
			}
		})
	}
}

func TestPurgeMessageRoundTrip(t *testing.T) {
	original := &PurgeMessage{
		Path:   []string{"test", "purge", "path"},
		From:   1000000000,
		To:     2000000000,
		Author: 1001,
	}

	// Encode
	encoded, err := EncodePurgeMessage(original)
	if err != nil {
		t.Fatalf("Failed to encode PurgeMessage: %v", err)
	}

	// Decode
	decoded, err := DecodePurgeMessage(encoded)
	if err != nil {
		t.Fatalf("Failed to decode PurgeMessage: %v", err)
	}

	// Verify
	if !reflect.DeepEqual(decoded.Path, original.Path) {
		t.Errorf("Path mismatch: expected %v, got %v", original.Path, decoded.Path)
	}
	if decoded.From != original.From {
		t.Errorf("From mismatch: expected %d, got %d", original.From, decoded.From)
	}
	if decoded.To != original.To {
		t.Errorf("To mismatch: expected %d, got %d", original.To, decoded.To)
	}
	if decoded.Author != original.Author {
		t.Errorf("Author mismatch: expected %d, got %d", original.Author, decoded.Author)
	}
}

func TestStatusResponseMessageRoundTrip(t *testing.T) {
	original := &StatusResponseMessage{
		Uptime:       3600,
		NodeIdentity: "test-node-id",
		LocalSocket:  "/tmp/test.socket",
		NetworkPort:  5000,
		Connections:  3,
		DataSize:     1048576,
	}

	// Encode
	encoded, err := EncodeStatusResponseMessage(original)
	if err != nil {
		t.Fatalf("Failed to encode StatusResponseMessage: %v", err)
	}

	// Decode
	decoded, err := DecodeStatusResponseMessage(encoded)
	if err != nil {
		t.Fatalf("Failed to decode StatusResponseMessage: %v", err)
	}

	// Verify
	if decoded.Uptime != original.Uptime {
		t.Errorf("Uptime mismatch: expected %d, got %d", original.Uptime, decoded.Uptime)
	}
	if decoded.NodeIdentity != original.NodeIdentity {
		t.Errorf("NodeIdentity mismatch: expected %s, got %s", original.NodeIdentity, decoded.NodeIdentity)
	}
	if decoded.LocalSocket != original.LocalSocket {
		t.Errorf("LocalSocket mismatch: expected %s, got %s", original.LocalSocket, decoded.LocalSocket)
	}
	if decoded.NetworkPort != original.NetworkPort {
		t.Errorf("NetworkPort mismatch: expected %d, got %d", original.NetworkPort, decoded.NetworkPort)
	}
	if decoded.Connections != original.Connections {
		t.Errorf("Connections mismatch: expected %d, got %d", original.Connections, decoded.Connections)
	}
	if decoded.DataSize != original.DataSize {
		t.Errorf("DataSize mismatch: expected %d, got %d", original.DataSize, decoded.DataSize)
	}
}

func TestErrorMessageRoundTrip(t *testing.T) {
	original := &ErrorMessage{
		Code:    404,
		Message: "Path not found",
	}

	// Encode
	encoded, err := EncodeErrorMessage(original)
	if err != nil {
		t.Fatalf("Failed to encode ErrorMessage: %v", err)
	}

	// Decode
	decoded, err := DecodeErrorMessage(encoded)
	if err != nil {
		t.Fatalf("Failed to decode ErrorMessage: %v", err)
	}

	// Verify
	if decoded.Code != original.Code {
		t.Errorf("Code mismatch: expected %d, got %d", original.Code, decoded.Code)
	}
	if decoded.Message != original.Message {
		t.Errorf("Message mismatch: expected %s, got %s", original.Message, decoded.Message)
	}
}

func TestChecksumValidation(t *testing.T) {
	payload := []byte("test payload")
	msg := CreateMessage(READ, 12345, payload)

	// Encode
	encoded, err := EncodeMessage(msg)
	if err != nil {
		t.Fatalf("Failed to encode message: %v", err)
	}

	// Corrupt the checksum
	encoded[len(encoded)-1] = 0xFF

	// Try to decode
	_, err = DecodeMessage(encoded)
	if err == nil {
		t.Error("Expected checksum validation to fail, but it succeeded")
	}
}

func TestVersionValidation(t *testing.T) {
	payload := []byte("test payload")
	msg := CreateMessage(READ, 12345, payload)
	msg.Version = 0xFF // Invalid version

	// Encode
	encoded, err := EncodeMessage(msg)
	if err != nil {
		t.Fatalf("Failed to encode message: %v", err)
	}

	// Try to decode
	_, err = DecodeMessage(encoded)
	if err == nil {
		t.Error("Expected version validation to fail, but it succeeded")
	}
}

func TestEmptyPath(t *testing.T) {
	original := &ReadMessage{
		Path: []string{},
	}

	// Encode
	encoded, err := EncodeReadMessage(original)
	if err != nil {
		t.Fatalf("Failed to encode ReadMessage with empty path: %v", err)
	}

	// Decode
	decoded, err := DecodeReadMessage(encoded)
	if err != nil {
		t.Fatalf("Failed to decode ReadMessage with empty path: %v", err)
	}

	// Verify
	if len(decoded.Path) != 0 {
		t.Errorf("Expected empty path, got %v", decoded.Path)
	}
}

func TestLargePayload(t *testing.T) {
	// Create a large path for stress testing
	largePath := make([]string, 1000)
	for i := 0; i < 1000; i++ {
		largePath[i] = "component"
	}

	original := &ReadMessage{
		Path: largePath,
	}

	// Encode
	encoded, err := EncodeReadMessage(original)
	if err != nil {
		t.Fatalf("Failed to encode large ReadMessage: %v", err)
	}

	// Decode
	decoded, err := DecodeReadMessage(encoded)
	if err != nil {
		t.Fatalf("Failed to decode large ReadMessage: %v", err)
	}

	// Verify
	if !reflect.DeepEqual(decoded.Path, original.Path) {
		t.Errorf("Large path mismatch")
	}
}

func TestExecuteMessageRoundTrip(t *testing.T) {
	original := &ExecuteMessage{
		Code: "my.value = 42\nmy.value",
	}

	// Encode
	encoded, err := EncodeExecuteMessage(original)
	if err != nil {
		t.Fatalf("Failed to encode ExecuteMessage: %v", err)
	}

	// Decode
	decoded, err := DecodeExecuteMessage(encoded)
	if err != nil {
		t.Fatalf("Failed to decode ExecuteMessage: %v", err)
	}

	// Verify
	if decoded.Code != original.Code {
		t.Errorf("Code mismatch: expected %s, got %s", original.Code, decoded.Code)
	}
}

func TestExecuteResponseMessageRoundTrip(t *testing.T) {
	// Test successful response with result
	original := &ExecuteResponseMessage{
		Success: true,
		Result: storage.Value{
			Data:    []byte("test result"),
			TypeTag: 1,
		},
		Error: "",
	}

	// Encode
	encoded, err := EncodeExecuteResponseMessage(original)
	if err != nil {
		t.Fatalf("Failed to encode ExecuteResponseMessage: %v", err)
	}

	// Decode
	decoded, err := DecodeExecuteResponseMessage(encoded)
	if err != nil {
		t.Fatalf("Failed to decode ExecuteResponseMessage: %v", err)
	}

	// Verify
	if decoded.Success != original.Success {
		t.Errorf("Success mismatch: expected %t, got %t", original.Success, decoded.Success)
	}
	if !reflect.DeepEqual(decoded.Result, original.Result) {
		t.Errorf("Result mismatch: expected %v, got %v", original.Result, decoded.Result)
	}
	if decoded.Error != original.Error {
		t.Errorf("Error mismatch: expected %s, got %s", original.Error, decoded.Error)
	}
}

func TestExecuteResponseMessageRoundTripError(t *testing.T) {
	// Test error response
	original := &ExecuteResponseMessage{
		Success: false,
		Result:  storage.Value{}, // Empty value for error case
		Error:   "undefined_function is not a recognized function",
	}

	// Encode
	encoded, err := EncodeExecuteResponseMessage(original)
	if err != nil {
		t.Fatalf("Failed to encode ExecuteResponseMessage: %v", err)
	}

	// Decode
	decoded, err := DecodeExecuteResponseMessage(encoded)
	if err != nil {
		t.Fatalf("Failed to decode ExecuteResponseMessage: %v", err)
	}

	// Verify
	if decoded.Success != original.Success {
		t.Errorf("Success mismatch: expected %t, got %t", original.Success, decoded.Success)
	}
	if len(decoded.Result.Data) != 0 || decoded.Result.TypeTag != 0 {
		t.Errorf("Result should be empty for error response, got %v", decoded.Result)
	}
	if decoded.Error != original.Error {
		t.Errorf("Error mismatch: expected %s, got %s", original.Error, decoded.Error)
	}
}
