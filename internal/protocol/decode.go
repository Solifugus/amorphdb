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

// DecodeReadAtMessage decodes a temporal read request.
func DecodeReadAtMessage(payload []byte) (*ReadAtMessage, error) {
	msg := &ReadAtMessage{}
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

		case 0x02: // Timestamp
			var ts int64
			if err := binary.Read(reader, binary.BigEndian, &ts); err != nil {
				return nil, fmt.Errorf("failed to decode timestamp: %w", err)
			}
			msg.Timestamp = ts

		default:
			// Skip unknown fields gracefully
			if err := skipField(reader, tag); err != nil {
				return nil, fmt.Errorf("failed to skip unknown field 0x%02x: %w", tag, err)
			}
		}
	}

	return msg, nil
}

// DecodeReadAtResponseMessage decodes the historical value for a temporal read.
func DecodeReadAtResponseMessage(payload []byte) (*ReadAtResponseMessage, error) {
	value, err := decodeValue(payload, 0x01)
	if err != nil {
		return nil, fmt.Errorf("failed to decode value: %w", err)
	}

	return &ReadAtResponseMessage{Value: value}, nil
}

// DecodeChildrenMessage decodes a request for the child attributes of a path.
func DecodeChildrenMessage(payload []byte) (*ChildrenMessage, error) {
	path, err := decodeStringSlice(payload, 0x01)
	if err != nil {
		return nil, fmt.Errorf("failed to decode path: %w", err)
	}

	return &ChildrenMessage{Path: path}, nil
}

// DecodeChildrenResponseMessage decodes a list of child attributes.
func DecodeChildrenResponseMessage(payload []byte) (*ChildrenResponseMessage, error) {
	reader := bytes.NewReader(payload)

	tag, err := reader.ReadByte()
	if err != nil {
		return nil, fmt.Errorf("failed to read tag: %w", err)
	}
	if tag != 0x01 {
		return nil, fmt.Errorf("unexpected tag 0x%02x for children response", tag)
	}

	var count uint32
	if err := binary.Read(reader, binary.BigEndian, &count); err != nil {
		return nil, fmt.Errorf("failed to decode children count: %w", err)
	}

	// Each child is at least four uint64 fields plus a name length; refuse a
	// count the payload cannot possibly satisfy rather than allocating on a
	// corrupt length.
	if int(count)*36 > reader.Len() {
		return nil, fmt.Errorf("children count %d exceeds payload size", count)
	}

	msg := &ChildrenResponseMessage{Children: make([]storage.NamedAttribute, 0, count)}
	for i := uint32(0); i < count; i++ {
		var child storage.NamedAttribute
		for _, field := range []*uint64{&child.ID, &child.LabelValueID, &child.FirstInstanceID, &child.NextAttributeID} {
			if err := binary.Read(reader, binary.BigEndian, field); err != nil {
				return nil, fmt.Errorf("failed to decode attribute field: %w", err)
			}
		}

		var nameLen uint32
		if err := binary.Read(reader, binary.BigEndian, &nameLen); err != nil {
			return nil, fmt.Errorf("failed to decode child name length: %w", err)
		}
		if int(nameLen) > reader.Len() {
			return nil, fmt.Errorf("child name length %d exceeds remaining payload", nameLen)
		}
		name := make([]byte, nameLen)
		if _, err := io.ReadFull(reader, name); err != nil {
			return nil, fmt.Errorf("failed to decode child name: %w", err)
		}
		child.Name = string(name)

		msg.Children = append(msg.Children, child)
	}

	return msg, nil
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

// DecodeWhoAmIResponseMessage deserializes a WhoAmIResponseMessage.
func DecodeWhoAmIResponseMessage(payload []byte) (*WhoAmIResponseMessage, error) {
	msg := &WhoAmIResponseMessage{}
	reader := bytes.NewReader(payload)

	for reader.Len() > 0 {
		tag, err := reader.ReadByte()
		if err != nil {
			return nil, fmt.Errorf("failed to read tag: %w", err)
		}

		switch tag {
		case 0x01: // Agent ID (int64 bits)
			var id int64
			if err := binary.Read(reader, binary.BigEndian, &id); err != nil {
				return nil, fmt.Errorf("failed to decode agent id: %w", err)
			}
			msg.AgentID = uint64(id)

		case 0x02: // Identity label
			identity, err := readString(reader)
			if err != nil {
				return nil, fmt.Errorf("failed to decode identity: %w", err)
			}
			msg.Identity = identity

		default:
			if err := skipField(reader, tag); err != nil {
				return nil, fmt.Errorf("failed to skip unknown field 0x%02x: %w", tag, err)
			}
		}
	}

	return msg, nil
}

// DecodeAuthInitMessage deserializes an AuthInitMessage.
func DecodeAuthInitMessage(payload []byte) (*AuthInitMessage, error) {
	msg := &AuthInitMessage{}
	reader := bytes.NewReader(payload)

	for reader.Len() > 0 {
		tag, err := reader.ReadByte()
		if err != nil {
			return nil, fmt.Errorf("failed to read tag: %w", err)
		}
		switch tag {
		case 0x01:
			identity, err := readString(reader)
			if err != nil {
				return nil, fmt.Errorf("failed to decode identity: %w", err)
			}
			msg.Identity = identity
		default:
			if err := skipField(reader, tag); err != nil {
				return nil, fmt.Errorf("failed to skip unknown field 0x%02x: %w", tag, err)
			}
		}
	}
	return msg, nil
}

// DecodeAuthChallengeMessage deserializes an AuthChallengeMessage.
func DecodeAuthChallengeMessage(payload []byte) (*AuthChallengeMessage, error) {
	msg := &AuthChallengeMessage{}
	reader := bytes.NewReader(payload)

	for reader.Len() > 0 {
		tag, err := reader.ReadByte()
		if err != nil {
			return nil, fmt.Errorf("failed to read tag: %w", err)
		}
		switch tag {
		case 0x01:
			blob, err := readString(reader)
			if err != nil {
				return nil, fmt.Errorf("failed to decode challenge: %w", err)
			}
			msg.Challenge = []byte(blob)
		default:
			if err := skipField(reader, tag); err != nil {
				return nil, fmt.Errorf("failed to skip unknown field 0x%02x: %w", tag, err)
			}
		}
	}
	return msg, nil
}

// DecodeAuthResponseMessage deserializes an AuthResponseMessage.
func DecodeAuthResponseMessage(payload []byte) (*AuthResponseMessage, error) {
	msg := &AuthResponseMessage{}
	reader := bytes.NewReader(payload)

	for reader.Len() > 0 {
		tag, err := reader.ReadByte()
		if err != nil {
			return nil, fmt.Errorf("failed to read tag: %w", err)
		}
		switch tag {
		case 0x01:
			blob, err := readString(reader)
			if err != nil {
				return nil, fmt.Errorf("failed to decode response: %w", err)
			}
			msg.Response = []byte(blob)
		default:
			if err := skipField(reader, tag); err != nil {
				return nil, fmt.Errorf("failed to skip unknown field 0x%02x: %w", tag, err)
			}
		}
	}
	return msg, nil
}

// DecodeAuthResultMessage deserializes an AuthResultMessage.
func DecodeAuthResultMessage(payload []byte) (*AuthResultMessage, error) {
	msg := &AuthResultMessage{}
	reader := bytes.NewReader(payload)

	for reader.Len() > 0 {
		tag, err := reader.ReadByte()
		if err != nil {
			return nil, fmt.Errorf("failed to read tag: %w", err)
		}
		switch tag {
		case 0x01:
			b, err := reader.ReadByte()
			if err != nil {
				return nil, fmt.Errorf("failed to decode success: %w", err)
			}
			msg.Success = b != 0
		case 0x02:
			var id int64
			if err := binary.Read(reader, binary.BigEndian, &id); err != nil {
				return nil, fmt.Errorf("failed to decode agent id: %w", err)
			}
			msg.AgentID = uint64(id)
		case 0x03:
			identity, err := readString(reader)
			if err != nil {
				return nil, fmt.Errorf("failed to decode identity: %w", err)
			}
			msg.Identity = identity
		case 0x04:
			errStr, err := readString(reader)
			if err != nil {
				return nil, fmt.Errorf("failed to decode error: %w", err)
			}
			msg.Error = errStr
		default:
			if err := skipField(reader, tag); err != nil {
				return nil, fmt.Errorf("failed to skip unknown field 0x%02x: %w", tag, err)
			}
		}
	}
	return msg, nil
}

// DecodeRegisterMessage deserializes a RegisterMessage.
func DecodeRegisterMessage(payload []byte) (*RegisterMessage, error) {
	msg := &RegisterMessage{}
	reader := bytes.NewReader(payload)

	for reader.Len() > 0 {
		tag, err := reader.ReadByte()
		if err != nil {
			return nil, fmt.Errorf("failed to read tag: %w", err)
		}
		switch tag {
		case 0x01:
			identity, err := readString(reader)
			if err != nil {
				return nil, fmt.Errorf("failed to decode identity: %w", err)
			}
			msg.Identity = identity
		case 0x02:
			key, err := readString(reader)
			if err != nil {
				return nil, fmt.Errorf("failed to decode public key: %w", err)
			}
			msg.PublicKey = []byte(key)
		case 0x03:
			token, err := readString(reader)
			if err != nil {
				return nil, fmt.Errorf("failed to decode token: %w", err)
			}
			msg.Token = token
		default:
			if err := skipField(reader, tag); err != nil {
				return nil, fmt.Errorf("failed to skip unknown field 0x%02x: %w", tag, err)
			}
		}
	}
	return msg, nil
}

// DecodeRegisterResultMessage deserializes a RegisterResultMessage.
func DecodeRegisterResultMessage(payload []byte) (*RegisterResultMessage, error) {
	msg := &RegisterResultMessage{}
	reader := bytes.NewReader(payload)

	for reader.Len() > 0 {
		tag, err := reader.ReadByte()
		if err != nil {
			return nil, fmt.Errorf("failed to read tag: %w", err)
		}
		switch tag {
		case 0x01:
			b, err := reader.ReadByte()
			if err != nil {
				return nil, fmt.Errorf("failed to decode success: %w", err)
			}
			msg.Success = b != 0
		case 0x02:
			var id int64
			if err := binary.Read(reader, binary.BigEndian, &id); err != nil {
				return nil, fmt.Errorf("failed to decode agent id: %w", err)
			}
			msg.AgentID = uint64(id)
		case 0x03:
			errStr, err := readString(reader)
			if err != nil {
				return nil, fmt.Errorf("failed to decode error: %w", err)
			}
			msg.Error = errStr
		default:
			if err := skipField(reader, tag); err != nil {
				return nil, fmt.Errorf("failed to skip unknown field 0x%02x: %w", tag, err)
			}
		}
	}
	return msg, nil
}

// DecodeInviteCreateResultMessage deserializes an InviteCreateResultMessage.
func DecodeInviteCreateResultMessage(payload []byte) (*InviteCreateResultMessage, error) {
	msg := &InviteCreateResultMessage{}
	reader := bytes.NewReader(payload)

	for reader.Len() > 0 {
		tag, err := reader.ReadByte()
		if err != nil {
			return nil, fmt.Errorf("failed to read tag: %w", err)
		}
		switch tag {
		case 0x01:
			b, err := reader.ReadByte()
			if err != nil {
				return nil, fmt.Errorf("failed to decode success: %w", err)
			}
			msg.Success = b != 0
		case 0x02:
			token, err := readString(reader)
			if err != nil {
				return nil, fmt.Errorf("failed to decode token: %w", err)
			}
			msg.Token = token
		case 0x03:
			errStr, err := readString(reader)
			if err != nil {
				return nil, fmt.Errorf("failed to decode error: %w", err)
			}
			msg.Error = errStr
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

func readBool(reader *bytes.Reader) (bool, error) {
	b, err := reader.ReadByte()
	if err != nil {
		return false, fmt.Errorf("failed to read bool byte: %w", err)
	}
	return b != 0, nil
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

// DecodeCreateMeshMessage deserializes a CreateMeshMessage
func DecodeCreateMeshMessage(payload []byte) (*CreateMeshMessage, error) {
	reader := bytes.NewReader(payload)
	msg := &CreateMeshMessage{}

	for reader.Len() > 0 {
		tag, err := reader.ReadByte()
		if err != nil {
			return nil, fmt.Errorf("failed to read tag: %w", err)
		}

		switch tag {
		case 0x01: // Mesh name
			name, err := readString(reader)
			if err != nil {
				return nil, fmt.Errorf("failed to decode mesh name: %w", err)
			}
			msg.Name = name

		default:
			if err := skipField(reader, tag); err != nil {
				return nil, fmt.Errorf("failed to skip unknown field 0x%02x: %w", tag, err)
			}
		}
	}

	return msg, nil
}

// DecodeCreateMeshAckMessage deserializes a CreateMeshAckMessage
func DecodeCreateMeshAckMessage(payload []byte) (*CreateMeshAckMessage, error) {
	reader := bytes.NewReader(payload)
	msg := &CreateMeshAckMessage{}

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

		case 0x02: // Message
			message, err := readString(reader)
			if err != nil {
				return nil, fmt.Errorf("failed to decode message: %w", err)
			}
			msg.Message = message

		case 0x03: // Mesh name
			meshName, err := readString(reader)
			if err != nil {
				return nil, fmt.Errorf("failed to decode mesh name: %w", err)
			}
			msg.MeshName = meshName

		case 0x04: // Node ID
			nodeID, err := readString(reader)
			if err != nil {
				return nil, fmt.Errorf("failed to decode node ID: %w", err)
			}
			msg.NodeID = nodeID

		case 0x05: // Is Founder
			founder, err := reader.ReadByte()
			if err != nil {
				return nil, fmt.Errorf("failed to decode is founder: %w", err)
			}
			msg.IsFounder = founder != 0

		default:
			if err := skipField(reader, tag); err != nil {
				return nil, fmt.Errorf("failed to skip unknown field 0x%02x: %w", tag, err)
			}
		}
	}

	return msg, nil
}

// DecodeMeshStatusMessage deserializes a MeshStatusMessage (empty request)
func DecodeMeshStatusMessage(payload []byte) (*MeshStatusMessage, error) {
	// MeshStatusMessage has no fields, but we still need to handle any future extensions
	reader := bytes.NewReader(payload)
	msg := &MeshStatusMessage{}

	for reader.Len() > 0 {
		tag, err := reader.ReadByte()
		if err != nil {
			return nil, fmt.Errorf("failed to read tag: %w", err)
		}

		// Skip any unknown fields for forward compatibility
		if err := skipField(reader, tag); err != nil {
			return nil, fmt.Errorf("failed to skip unknown field 0x%02x: %w", tag, err)
		}
	}

	return msg, nil
}

// DecodeMeshStatusResponseMessage deserializes a MeshStatusResponseMessage
func DecodeMeshStatusResponseMessage(payload []byte) (*MeshStatusResponseMessage, error) {
	reader := bytes.NewReader(payload)
	msg := &MeshStatusResponseMessage{
		Bridges: make(map[string]BridgeStatusInfo),
	}

	for reader.Len() > 0 {
		tag, err := reader.ReadByte()
		if err != nil {
			return nil, fmt.Errorf("failed to read tag: %w", err)
		}

		switch tag {
		case 0x01: // Mesh name
			meshName, err := readString(reader)
			if err != nil {
				return nil, fmt.Errorf("failed to decode mesh name: %w", err)
			}
			msg.MeshName = meshName

		case 0x02: // Status
			status, err := readString(reader)
			if err != nil {
				return nil, fmt.Errorf("failed to decode status: %w", err)
			}
			msg.Status = status

		case 0x03: // Is Founder
			founder, err := reader.ReadByte()
			if err != nil {
				return nil, fmt.Errorf("failed to decode is founder: %w", err)
			}
			msg.IsFounder = founder != 0

		case 0x04: // Node Identity
			identity, err := readString(reader)
			if err != nil {
				return nil, fmt.Errorf("failed to decode node identity: %w", err)
			}
			msg.NodeIdentity = identity

		case 0x05: // Member Count
			var memberCount int32
			if err := binary.Read(reader, binary.BigEndian, &memberCount); err != nil {
				return nil, fmt.Errorf("failed to decode member count: %w", err)
			}
			msg.MemberCount = int(memberCount)

		case 0x06: // Zone Count
			var zoneCount int32
			if err := binary.Read(reader, binary.BigEndian, &zoneCount); err != nil {
				return nil, fmt.Errorf("failed to decode zone count: %w", err)
			}
			msg.ZoneCount = int(zoneCount)

		case 0x07: // Founded At
			var foundedAt int64
			if err := binary.Read(reader, binary.BigEndian, &foundedAt); err != nil {
				return nil, fmt.Errorf("failed to decode founded at: %w", err)
			}
			msg.FoundedAt = &foundedAt

		default:
			if err := skipField(reader, tag); err != nil {
				return nil, fmt.Errorf("failed to skip unknown field 0x%02x: %w", tag, err)
			}
		}
	}

	return msg, nil
}

// DecodeDiscoverMeshMessage deserializes a DiscoverMeshMessage (empty request)
func DecodeDiscoverMeshMessage(payload []byte) (*DiscoverMeshMessage, error) {
	// DiscoverMeshMessage has no fields, but we still need to handle any future extensions
	reader := bytes.NewReader(payload)
	msg := &DiscoverMeshMessage{}

	for reader.Len() > 0 {
		tag, err := reader.ReadByte()
		if err != nil {
			return nil, fmt.Errorf("failed to read tag: %w", err)
		}

		// Skip any unknown fields for forward compatibility
		if err := skipField(reader, tag); err != nil {
			return nil, fmt.Errorf("failed to skip unknown field 0x%02x: %w", tag, err)
		}
	}

	return msg, nil
}

// DecodeDiscoverMeshResponseMessage deserializes a DiscoverMeshResponseMessage
func DecodeDiscoverMeshResponseMessage(payload []byte) (*DiscoverMeshResponseMessage, error) {
	reader := bytes.NewReader(payload)
	msg := &DiscoverMeshResponseMessage{}

	for reader.Len() > 0 {
		tag, err := reader.ReadByte()
		if err != nil {
			return nil, fmt.Errorf("failed to read tag: %w", err)
		}

		switch tag {
		case 0x01: // Mesh name
			meshName, err := readString(reader)
			if err != nil {
				return nil, fmt.Errorf("failed to decode mesh name: %w", err)
			}
			msg.MeshName = meshName

		case 0x02: // Founder identity
			founderIdentity, err := readString(reader)
			if err != nil {
				return nil, fmt.Errorf("failed to decode founder identity: %w", err)
			}
			msg.FounderIdentity = founderIdentity

		case 0x03: // Member count
			var memberCount int32
			if err := binary.Read(reader, binary.BigEndian, &memberCount); err != nil {
				return nil, fmt.Errorf("failed to decode member count: %w", err)
			}
			msg.MemberCount = int(memberCount)

		case 0x04: // Zone count
			var zoneCount int32
			if err := binary.Read(reader, binary.BigEndian, &zoneCount); err != nil {
				return nil, fmt.Errorf("failed to decode zone count: %w", err)
			}
			msg.ZoneCount = int(zoneCount)

		case 0x05: // Requires auth
			requiresAuth, err := reader.ReadByte()
			if err != nil {
				return nil, fmt.Errorf("failed to decode requires auth: %w", err)
			}
			msg.RequiresAuth = requiresAuth != 0

		case 0x06: // Founded at
			var foundedAt int64
			if err := binary.Read(reader, binary.BigEndian, &foundedAt); err != nil {
				return nil, fmt.Errorf("failed to decode founded at: %w", err)
			}
			msg.FoundedAt = foundedAt

		default:
			if err := skipField(reader, tag); err != nil {
				return nil, fmt.Errorf("failed to skip unknown field 0x%02x: %w", tag, err)
			}
		}
	}

	return msg, nil
}

// DecodeJoinMeshMessage deserializes a JoinMeshMessage
func DecodeJoinMeshMessage(payload []byte) (*JoinMeshMessage, error) {
	reader := bytes.NewReader(payload)
	msg := &JoinMeshMessage{}

	for reader.Len() > 0 {
		tag, err := reader.ReadByte()
		if err != nil {
			return nil, fmt.Errorf("failed to read tag: %w", err)
		}

		switch tag {
		case 0x01: // Node identity
			nodeIdentity, err := readString(reader)
			if err != nil {
				return nil, fmt.Errorf("failed to decode node identity: %w", err)
			}
			msg.NodeIdentity = nodeIdentity

		case 0x02: // Mesh name
			meshName, err := readString(reader)
			if err != nil {
				return nil, fmt.Errorf("failed to decode mesh name: %w", err)
			}
			msg.MeshName = meshName

		default:
			if err := skipField(reader, tag); err != nil {
				return nil, fmt.Errorf("failed to skip unknown field 0x%02x: %w", tag, err)
			}
		}
	}

	return msg, nil
}

// DecodeJoinMeshAckMessage deserializes a JoinMeshAckMessage
func DecodeJoinMeshAckMessage(payload []byte) (*JoinMeshAckMessage, error) {
	reader := bytes.NewReader(payload)
	msg := &JoinMeshAckMessage{}

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

		case 0x02: // Message
			message, err := readString(reader)
			if err != nil {
				return nil, fmt.Errorf("failed to decode message: %w", err)
			}
			msg.Message = message

		case 0x03: // Assigned zone
			assignedZone, err := readString(reader)
			if err != nil {
				return nil, fmt.Errorf("failed to decode assigned zone: %w", err)
			}
			msg.AssignedZone = assignedZone

		case 0x04: // Member count
			var memberCount int32
			if err := binary.Read(reader, binary.BigEndian, &memberCount); err != nil {
				return nil, fmt.Errorf("failed to decode member count: %w", err)
			}
			msg.MemberCount = int(memberCount)

		default:
			if err := skipField(reader, tag); err != nil {
				return nil, fmt.Errorf("failed to skip unknown field 0x%02x: %w", tag, err)
			}
		}
	}

	return msg, nil
}

// DecodeCreateBridgeMessage deserializes a CreateBridgeMessage
func DecodeCreateBridgeMessage(payload []byte) (*CreateBridgeMessage, error) {
	reader := bytes.NewReader(payload)
	msg := &CreateBridgeMessage{}

	for reader.Len() > 0 {
		tag, err := reader.ReadByte()
		if err != nil {
			return nil, fmt.Errorf("failed to read tag: %w", err)
		}

		switch tag {
		case 0x01: // Target address
			targetAddress, err := readString(reader)
			if err != nil {
				return nil, fmt.Errorf("failed to decode target address: %w", err)
			}
			msg.TargetAddress = targetAddress

		case 0x02: // Node identity
			nodeIdentity, err := readString(reader)
			if err != nil {
				return nil, fmt.Errorf("failed to decode node identity: %w", err)
			}
			msg.NodeIdentity = nodeIdentity

		default:
			if err := skipField(reader, tag); err != nil {
				return nil, fmt.Errorf("failed to skip unknown field 0x%02x: %w", tag, err)
			}
		}
	}

	return msg, nil
}

// DecodeCreateBridgeAckMessage deserializes a CreateBridgeAckMessage
func DecodeCreateBridgeAckMessage(payload []byte) (*CreateBridgeAckMessage, error) {
	reader := bytes.NewReader(payload)
	msg := &CreateBridgeAckMessage{}

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

		case 0x02: // Message
			message, err := readString(reader)
			if err != nil {
				return nil, fmt.Errorf("failed to decode message: %w", err)
			}
			msg.Message = message

		case 0x03: // Target mesh name
			targetMeshName, err := readString(reader)
			if err != nil {
				return nil, fmt.Errorf("failed to decode target mesh name: %w", err)
			}
			msg.TargetMeshName = targetMeshName

		case 0x04: // Bridge identity
			bridgeIdentity, err := readString(reader)
			if err != nil {
				return nil, fmt.Errorf("failed to decode bridge identity: %w", err)
			}
			msg.BridgeIdentity = bridgeIdentity

		case 0x05: // Bridge status
			bridgeStatus, err := readString(reader)
			if err != nil {
				return nil, fmt.Errorf("failed to decode bridge status: %w", err)
			}
			msg.BridgeStatus = bridgeStatus

		default:
			if err := skipField(reader, tag); err != nil {
				return nil, fmt.Errorf("failed to skip unknown field 0x%02x: %w", tag, err)
			}
		}
	}

	return msg, nil
}

// DecodeExecuteMessage decodes an ExecuteMessage from payload
func DecodeExecuteMessage(payload []byte) (*ExecuteMessage, error) {
	msg := &ExecuteMessage{}
	reader := bytes.NewReader(payload)

	for reader.Len() > 0 {
		tag, err := reader.ReadByte()
		if err != nil {
			return nil, fmt.Errorf("failed to read tag: %w", err)
		}

		switch tag {
		case 0x01: // MBL source code
			code, err := readString(reader)
			if err != nil {
				return nil, fmt.Errorf("failed to decode code: %w", err)
			}
			msg.Code = code

		default:
			if err := skipField(reader, tag); err != nil {
				return nil, fmt.Errorf("failed to skip unknown field 0x%02x: %w", tag, err)
			}
		}
	}

	return msg, nil
}

// DecodeExecuteResponseMessage decodes an ExecuteResponseMessage from payload
func DecodeExecuteResponseMessage(payload []byte) (*ExecuteResponseMessage, error) {
	msg := &ExecuteResponseMessage{}
	reader := bytes.NewReader(payload)

	for reader.Len() > 0 {
		tag, err := reader.ReadByte()
		if err != nil {
			return nil, fmt.Errorf("failed to read tag: %w", err)
		}

		switch tag {
		case 0x01: // Success flag
			success, err := readBool(reader)
			if err != nil {
				return nil, fmt.Errorf("failed to decode success: %w", err)
			}
			msg.Success = success

		case 0x02: // Result value
			result, err := readValue(reader)
			if err != nil {
				return nil, fmt.Errorf("failed to decode result: %w", err)
			}
			msg.Result = result

		case 0x03: // Error message
			errMsg, err := readString(reader)
			if err != nil {
				return nil, fmt.Errorf("failed to decode error: %w", err)
			}
			msg.Error = errMsg

		default:
			if err := skipField(reader, tag); err != nil {
				return nil, fmt.Errorf("failed to skip unknown field 0x%02x: %w", tag, err)
			}
		}
	}

	return msg, nil
}

// DecodeExtractMessage decodes an ExtractMessage from payload
func DecodeExtractMessage(payload []byte) (*ExtractMessage, error) {
	msg := &ExtractMessage{}
	reader := bytes.NewReader(payload)

	for reader.Len() > 0 {
		tag, err := reader.ReadByte()
		if err != nil {
			return nil, fmt.Errorf("failed to read tag: %w", err)
		}

		switch tag {
		case 0x01: // Path to extract from
			path, err := readStringSlice(reader)
			if err != nil {
				return nil, fmt.Errorf("failed to decode path: %w", err)
			}
			msg.Path = path

		default:
			if err := skipField(reader, tag); err != nil {
				return nil, fmt.Errorf("failed to skip unknown field 0x%02x: %w", tag, err)
			}
		}
	}

	return msg, nil
}

// DecodeExtractResponseMessage decodes an ExtractResponseMessage from payload
func DecodeExtractResponseMessage(payload []byte) (*ExtractResponseMessage, error) {
	msg := &ExtractResponseMessage{}
	reader := bytes.NewReader(payload)

	for reader.Len() > 0 {
		tag, err := reader.ReadByte()
		if err != nil {
			return nil, fmt.Errorf("failed to read tag: %w", err)
		}

		switch tag {
		case 0x01: // Success flag
			success, err := readBool(reader)
			if err != nil {
				return nil, fmt.Errorf("failed to decode success: %w", err)
			}
			msg.Success = success

		case 0x02: // MBL script
			script, err := readString(reader)
			if err != nil {
				return nil, fmt.Errorf("failed to decode script: %w", err)
			}
			msg.Script = script

		case 0x03: // Error message
			errMsg, err := readString(reader)
			if err != nil {
				return nil, fmt.Errorf("failed to decode error: %w", err)
			}
			msg.Error = errMsg

		default:
			if err := skipField(reader, tag); err != nil {
				return nil, fmt.Errorf("failed to skip unknown field 0x%02x: %w", tag, err)
			}
		}
	}

	return msg, nil
}
