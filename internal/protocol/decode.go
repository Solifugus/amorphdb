package protocol

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"io"

	"github.com/solifugus/amorphdb/internal/storage"
)

// DecodeMessage deserializes a binary Message
func DecodeMessage(data []byte) (*Message, error) {
	if len(data) < 14 { // Minimum message size: header (10) + checksum (4)
		return nil, fmt.Errorf("message too short: %d bytes", len(data))
	}

	reader := bytes.NewReader(data)
	msg := &Message{}

	// Read fixed header
	if err := binary.Read(reader, binary.BigEndian, &msg.Version); err != nil {
		return nil, fmt.Errorf("failed to decode version: %w", err)
	}
	if err := binary.Read(reader, binary.BigEndian, &msg.Type); err != nil {
		return nil, fmt.Errorf("failed to decode type: %w", err)
	}
	if err := binary.Read(reader, binary.BigEndian, &msg.Sequence); err != nil {
		return nil, fmt.Errorf("failed to decode sequence: %w", err)
	}
	if err := binary.Read(reader, binary.BigEndian, &msg.PayloadLength); err != nil {
		return nil, fmt.Errorf("failed to decode payload length: %w", err)
	}

	// Validate message size
	expectedSize := 14 + int(msg.PayloadLength) // header + payload + checksum
	if len(data) != expectedSize {
		return nil, fmt.Errorf("message size mismatch: expected %d, got %d", expectedSize, len(data))
	}

	// Read payload
	msg.Payload = make([]byte, msg.PayloadLength)
	if n, err := reader.Read(msg.Payload); err != nil || n != int(msg.PayloadLength) {
		return nil, fmt.Errorf("failed to read payload: %w", err)
	}

	// Read checksum
	if err := binary.Read(reader, binary.BigEndian, &msg.Checksum); err != nil {
		return nil, fmt.Errorf("failed to decode checksum: %w", err)
	}

	// Verify checksum
	calculatedChecksum := crc32.ChecksumIEEE(msg.Payload)
	if msg.Checksum != calculatedChecksum {
		return nil, fmt.Errorf("checksum mismatch: expected 0x%08x, got 0x%08x", calculatedChecksum, msg.Checksum)
	}

	// Verify version
	if msg.Version != Version {
		return nil, fmt.Errorf("unsupported protocol version: 0x%02x", msg.Version)
	}

	return msg, nil
}

// DecodeReadMessage deserializes a ReadMessage from payload
func DecodeReadMessage(payload []byte) (*ReadMessage, error) {
	path, err := decodeStringSlice(payload, 0x01)
	if err != nil {
		return nil, fmt.Errorf("failed to decode path: %w", err)
	}

	return &ReadMessage{Path: path}, nil
}

// DecodeReadResponseMessage deserializes a ReadResponseMessage
func DecodeReadResponseMessage(payload []byte) (*ReadResponseMessage, error) {
	value, err := decodeValue(payload, 0x01)
	if err != nil {
		return nil, fmt.Errorf("failed to decode value: %w", err)
	}

	return &ReadResponseMessage{Value: value}, nil
}

// DecodeWriteMessage deserializes a WriteMessage
func DecodeWriteMessage(payload []byte) (*WriteMessage, error) {
	msg := &WriteMessage{}
	reader := bytes.NewReader(payload)

	// Parse tagged fields
	for reader.Len() > 0 {
		tag, err := reader.ReadByte()
		if err != nil {
			return nil, fmt.Errorf("failed to read tag: %w", err)
		}

		switch tag {
		case 0x01: // Path
			path, err := readStringSlice(reader)
			if err != nil {
				return nil, fmt.Errorf("failed to decode path: %w", err)
			}
			msg.Path = path

		case 0x02: // Value
			value, err := readValue(reader)
			if err != nil {
				return nil, fmt.Errorf("failed to decode value: %w", err)
			}
			msg.Value = value

		case 0x03: // Author
			var author uint64
			if err := binary.Read(reader, binary.BigEndian, &author); err != nil {
				return nil, fmt.Errorf("failed to decode author: %w", err)
			}
			msg.Author = author

		default:
			// Skip unknown fields gracefully
			if err := skipField(reader, tag); err != nil {
				return nil, fmt.Errorf("failed to skip unknown field 0x%02x: %w", tag, err)
			}
		}
	}

	return msg, nil
}

// DecodeWriteAckMessage deserializes a WriteAckMessage
func DecodeWriteAckMessage(payload []byte) (*WriteAckMessage, error) {
	msg := &WriteAckMessage{}
	reader := bytes.NewReader(payload)

	for reader.Len() > 0 {
		tag, err := reader.ReadByte()
		if err != nil {
			return nil, fmt.Errorf("failed to read tag: %w", err)
		}

		switch tag {
		case 0x01: // Success
			success, err := reader.ReadByte()
			if err != nil {
				return nil, fmt.Errorf("failed to decode success: %w", err)
			}
			msg.Success = success != 0

		case 0x02: // Error
			errorMsg, err := readString(reader)
			if err != nil {
				return nil, fmt.Errorf("failed to decode error: %w", err)
			}
			msg.Error = errorMsg

		default:
			if err := skipField(reader, tag); err != nil {
				return nil, fmt.Errorf("failed to skip unknown field 0x%02x: %w", tag, err)
			}
		}
	}

	return msg, nil
}

// DecodePurgeMessage deserializes a PurgeMessage
func DecodePurgeMessage(payload []byte) (*PurgeMessage, error) {
	msg := &PurgeMessage{}
	reader := bytes.NewReader(payload)

	for reader.Len() > 0 {
		tag, err := reader.ReadByte()
		if err != nil {
			return nil, fmt.Errorf("failed to read tag: %w", err)
		}

		switch tag {
		case 0x01: // Path
			path, err := readStringSlice(reader)
			if err != nil {
				return nil, fmt.Errorf("failed to decode path: %w", err)
			}
			msg.Path = path

		case 0x02: // From
			var from int64
			if err := binary.Read(reader, binary.BigEndian, &from); err != nil {
				return nil, fmt.Errorf("failed to decode from: %w", err)
			}
			msg.From = from

		case 0x03: // To
			var to int64
			if err := binary.Read(reader, binary.BigEndian, &to); err != nil {
				return nil, fmt.Errorf("failed to decode to: %w", err)
			}
			msg.To = to

		case 0x04: // Author
			var author uint64
			if err := binary.Read(reader, binary.BigEndian, &author); err != nil {
				return nil, fmt.Errorf("failed to decode author: %w", err)
			}
			msg.Author = author

		default:
			if err := skipField(reader, tag); err != nil {
				return nil, fmt.Errorf("failed to skip unknown field 0x%02x: %w", tag, err)
			}
		}
	}

	return msg, nil
}

// DecodeStatusResponseMessage deserializes a StatusResponseMessage
func DecodeStatusResponseMessage(payload []byte) (*StatusResponseMessage, error) {
	msg := &StatusResponseMessage{}
	reader := bytes.NewReader(payload)

	for reader.Len() > 0 {
		tag, err := reader.ReadByte()
		if err != nil {
			return nil, fmt.Errorf("failed to read tag: %w", err)
		}

		switch tag {
		case 0x01: // Uptime
			var uptime int64
			if err := binary.Read(reader, binary.BigEndian, &uptime); err != nil {
				return nil, fmt.Errorf("failed to decode uptime: %w", err)
			}
			msg.Uptime = uptime

		case 0x02: // Node Identity
			identity, err := readString(reader)
			if err != nil {
				return nil, fmt.Errorf("failed to decode node identity: %w", err)
			}
			msg.NodeIdentity = identity

		case 0x03: // Local Socket
			socket, err := readString(reader)
			if err != nil {
				return nil, fmt.Errorf("failed to decode local socket: %w", err)
			}
			msg.LocalSocket = socket

		case 0x04: // Network Port
			var port int32
			if err := binary.Read(reader, binary.BigEndian, &port); err != nil {
				return nil, fmt.Errorf("failed to decode network port: %w", err)
			}
			msg.NetworkPort = int(port)

		case 0x05: // Connections
			var connections int32
			if err := binary.Read(reader, binary.BigEndian, &connections); err != nil {
				return nil, fmt.Errorf("failed to decode connections: %w", err)
			}
			msg.Connections = int(connections)

		case 0x06: // Data Size
			var dataSize int64
			if err := binary.Read(reader, binary.BigEndian, &dataSize); err != nil {
				return nil, fmt.Errorf("failed to decode data size: %w", err)
			}
			msg.DataSize = dataSize

		default:
			if err := skipField(reader, tag); err != nil {
				return nil, fmt.Errorf("failed to skip unknown field 0x%02x: %w", tag, err)
			}
		}
	}

	return msg, nil
}

// DecodeStopAckMessage deserializes a StopAckMessage
func DecodeStopAckMessage(payload []byte) (*StopAckMessage, error) {
	msg := &StopAckMessage{}
	reader := bytes.NewReader(payload)

	for reader.Len() > 0 {
		tag, err := reader.ReadByte()
		if err != nil {
			return nil, fmt.Errorf("failed to read tag: %w", err)
		}

		switch tag {
		case 0x01: // Success
			success, err := reader.ReadByte()
			if err != nil {
				return nil, fmt.Errorf("failed to decode success: %w", err)
			}
			msg.Success = success != 0

		case 0x02: // Error
			errorMsg, err := readString(reader)
			if err != nil {
				return nil, fmt.Errorf("failed to decode error: %w", err)
			}
			msg.Error = errorMsg

		default:
			if err := skipField(reader, tag); err != nil {
				return nil, fmt.Errorf("failed to skip unknown field 0x%02x: %w", tag, err)
			}
		}
	}

	return msg, nil
}

// DecodeCompactAckMessage deserializes a CompactAckMessage
func DecodeCompactAckMessage(payload []byte) (*CompactAckMessage, error) {
	msg := &CompactAckMessage{}
	reader := bytes.NewReader(payload)

	for reader.Len() > 0 {
		tag, err := reader.ReadByte()
		if err != nil {
			return nil, fmt.Errorf("failed to read tag: %w", err)
		}

		switch tag {
		case 0x01: // Success
			success, err := reader.ReadByte()
			if err != nil {
				return nil, fmt.Errorf("failed to decode success: %w", err)
			}
			msg.Success = success != 0

		case 0x02: // Error
			errorMsg, err := readString(reader)
			if err != nil {
				return nil, fmt.Errorf("failed to decode error: %w", err)
			}
			msg.Error = errorMsg

		case 0x03: // Space Reclaimed
			var spaceReclaimed int64
			if err := binary.Read(reader, binary.BigEndian, &spaceReclaimed); err != nil {
				return nil, fmt.Errorf("failed to decode space reclaimed: %w", err)
			}
			msg.SpaceReclaimed = spaceReclaimed

		default:
			if err := skipField(reader, tag); err != nil {
				return nil, fmt.Errorf("failed to skip unknown field 0x%02x: %w", tag, err)
			}
		}
	}

	return msg, nil
}

// DecodePurgeAckMessage deserializes a PurgeAckMessage
func DecodePurgeAckMessage(payload []byte) (*PurgeAckMessage, error) {
	msg := &PurgeAckMessage{}
	reader := bytes.NewReader(payload)

	for reader.Len() > 0 {
		tag, err := reader.ReadByte()
		if err != nil {
			return nil, fmt.Errorf("failed to read tag: %w", err)
		}

		switch tag {
		case 0x01: // Success
			success, err := reader.ReadByte()
			if err != nil {
				return nil, fmt.Errorf("failed to decode success: %w", err)
			}
			msg.Success = success != 0

		default:
			// Skip unknown tag
			if err := skipField(reader, tag); err != nil {
				return nil, fmt.Errorf("failed to skip unknown tag: %w", err)
			}
		}
	}

	return msg, nil
}

// DecodeErrorMessage deserializes an ErrorMessage
func DecodeErrorMessage(payload []byte) (*ErrorMessage, error) {
	msg := &ErrorMessage{}
	reader := bytes.NewReader(payload)

	for reader.Len() > 0 {
		tag, err := reader.ReadByte()
		if err != nil {
			return nil, fmt.Errorf("failed to read tag: %w", err)
		}

		switch tag {
		case 0x01: // Error code
			var code uint32
			if err := binary.Read(reader, binary.BigEndian, &code); err != nil {
				return nil, fmt.Errorf("failed to decode error code: %w", err)
			}
			msg.Code = code

		case 0x02: // Error message
			message, err := readString(reader)
			if err != nil {
				return nil, fmt.Errorf("failed to decode error message: %w", err)
			}
			msg.Message = message

		default:
			if err := skipField(reader, tag); err != nil {
				return nil, fmt.Errorf("failed to skip unknown field 0x%02x: %w", tag, err)
			}
		}
	}

	return msg, nil
}

// Helper functions for decoding tagged fields

func decodeStringSlice(payload []byte, expectedTag uint8) ([]string, error) {
	reader := bytes.NewReader(payload)

	tag, err := reader.ReadByte()
	if err != nil {
		return nil, fmt.Errorf("failed to read tag: %w", err)
	}
	if tag != expectedTag {
		return nil, fmt.Errorf("unexpected tag: expected 0x%02x, got 0x%02x", expectedTag, tag)
	}

	return readStringSlice(reader)
}

func decodeValue(payload []byte, expectedTag uint8) (storage.Value, error) {
	reader := bytes.NewReader(payload)

	tag, err := reader.ReadByte()
	if err != nil {
		return storage.Value{}, fmt.Errorf("failed to read tag: %w", err)
	}
	if tag != expectedTag {
		return storage.Value{}, fmt.Errorf("unexpected tag: expected 0x%02x, got 0x%02x", expectedTag, tag)
	}

	return readValue(reader)
}

func readStringSlice(reader *bytes.Reader) ([]string, error) {
	var length uint32
	if err := binary.Read(reader, binary.BigEndian, &length); err != nil {
		return nil, fmt.Errorf("failed to read slice length: %w", err)
	}

	result := make([]string, length)
	for i := uint32(0); i < length; i++ {
		str, err := readString(reader)
		if err != nil {
			return nil, fmt.Errorf("failed to read string %d: %w", i, err)
		}
		result[i] = str
	}

	return result, nil
}

func readString(reader *bytes.Reader) (string, error) {
	var length uint32
	if err := binary.Read(reader, binary.BigEndian, &length); err != nil {
		return "", fmt.Errorf("failed to read string length: %w", err)
	}

	data := make([]byte, length)
	if n, err := reader.Read(data); err != nil || n != int(length) {
		return "", fmt.Errorf("failed to read string data: %w", err)
	}

	return string(data), nil
}

func readValue(reader *bytes.Reader) (storage.Value, error) {
	// Read type tag
	typeTag, err := reader.ReadByte()
	if err != nil {
		return storage.Value{}, fmt.Errorf("failed to read value type tag: %w", err)
	}

	// Read data length
	var length uint32
	if err := binary.Read(reader, binary.BigEndian, &length); err != nil {
		return storage.Value{}, fmt.Errorf("failed to read value data length: %w", err)
	}

	// Read data
	data := make([]byte, length)
	if n, err := reader.Read(data); err != nil || n != int(length) {
		return storage.Value{}, fmt.Errorf("failed to read value data: %w", err)
	}

	return storage.Value{
		Data:    data,
		TypeTag: typeTag,
	}, nil
}

func skipField(reader *bytes.Reader, tag uint8) error {
	// Simple field skipping - reads one length-prefixed data block
	// This is a simplified implementation; a full implementation would
	// need to know the field format to skip correctly

	switch {
	case tag >= 0x01 && tag <= 0x10: // Assume length-prefixed fields
		var length uint32
		if err := binary.Read(reader, binary.BigEndian, &length); err != nil {
			return fmt.Errorf("failed to read field length: %w", err)
		}

		if length > uint32(reader.Len()) {
			return fmt.Errorf("invalid field length: %d", length)
		}

		// Skip the data
		_, err := reader.Seek(int64(length), io.SeekCurrent)
		return err

	default:
		// For unknown tag formats, skip single byte (conservative approach)
		_, err := reader.ReadByte()
		return err
	}
}