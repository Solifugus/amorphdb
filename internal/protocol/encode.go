package protocol

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"hash/crc32"

	"github.com/solifugus/amorphdb/internal/storage"
)

// EncodeMessage serializes a Message to binary format
func EncodeMessage(msg *Message) ([]byte, error) {
	var buf bytes.Buffer

	// Write fixed header (14 bytes total)
	if err := binary.Write(&buf, binary.BigEndian, msg.Version); err != nil {
		return nil, fmt.Errorf("failed to encode version: %w", err)
	}
	if err := binary.Write(&buf, binary.BigEndian, msg.Type); err != nil {
		return nil, fmt.Errorf("failed to encode type: %w", err)
	}
	if err := binary.Write(&buf, binary.BigEndian, msg.Sequence); err != nil {
		return nil, fmt.Errorf("failed to encode sequence: %w", err)
	}
	if err := binary.Write(&buf, binary.BigEndian, msg.PayloadLength); err != nil {
		return nil, fmt.Errorf("failed to encode payload length: %w", err)
	}

	// Write variable payload
	if len(msg.Payload) != int(msg.PayloadLength) {
		return nil, fmt.Errorf("payload length mismatch: expected %d, got %d", msg.PayloadLength, len(msg.Payload))
	}
	buf.Write(msg.Payload)

	// Write checksum
	if err := binary.Write(&buf, binary.BigEndian, msg.Checksum); err != nil {
		return nil, fmt.Errorf("failed to encode checksum: %w", err)
	}

	return buf.Bytes(), nil
}

// CreateMessage creates a Message with payload and checksum
func CreateMessage(msgType uint8, sequence uint32, payload []byte) *Message {
	// Calculate checksum of entire message content (excluding checksum field)
	checksum := crc32.ChecksumIEEE(payload)

	return &Message{
		Version:       Version,
		Type:          msgType,
		Sequence:      sequence,
		PayloadLength: uint32(len(payload)),
		Payload:       payload,
		Checksum:      checksum,
	}
}

// EncodeReadMessage serializes a ReadMessage to tagged field format
func EncodeReadMessage(msg *ReadMessage) ([]byte, error) {
	return encodeStringSlice(0x01, msg.Path)
}

// EncodeReadResponseMessage serializes a ReadResponseMessage
func EncodeReadResponseMessage(msg *ReadResponseMessage) ([]byte, error) {
	return encodeValue(0x01, msg.Value)
}

// EncodeReadAtMessage encodes a temporal read request: a path plus the instant to
// read as of. The timestamp is UNIX microseconds, matching types.Time.Serialize
// and storage.StorageTree.ReadAt.
func EncodeReadAtMessage(msg *ReadAtMessage) ([]byte, error) {
	var buf bytes.Buffer

	// Tag 0x01: Path (string slice)
	pathData, err := encodeStringSlice(0x01, msg.Path)
	if err != nil {
		return nil, fmt.Errorf("failed to encode path: %w", err)
	}
	buf.Write(pathData)

	// Tag 0x02: Timestamp (int64, UNIX microseconds)
	tsData, err := encodeInt64(0x02, msg.Timestamp)
	if err != nil {
		return nil, fmt.Errorf("failed to encode timestamp: %w", err)
	}
	buf.Write(tsData)

	return buf.Bytes(), nil
}

// EncodeReadAtResponseMessage encodes the historical value for a temporal read.
func EncodeReadAtResponseMessage(msg *ReadAtResponseMessage) ([]byte, error) {
	return encodeValue(0x01, msg.Value)
}

// EncodeChildrenMessage encodes a request for the child attributes of a path.
func EncodeChildrenMessage(msg *ChildrenMessage) ([]byte, error) {
	return encodeStringSlice(0x01, msg.Path)
}

// EncodeChildrenResponseMessage encodes a list of child attributes. Each
// attribute is four fixed-width uint64 fields, so the payload is a count
// followed by count * 32 bytes.
func EncodeChildrenResponseMessage(msg *ChildrenResponseMessage) ([]byte, error) {
	var buf bytes.Buffer

	buf.WriteByte(0x01)
	if err := binary.Write(&buf, binary.BigEndian, uint32(len(msg.Children))); err != nil {
		return nil, fmt.Errorf("failed to encode children count: %w", err)
	}
	for _, child := range msg.Children {
		for _, field := range []uint64{child.ID, child.LabelValueID, child.FirstInstanceID, child.NextAttributeID} {
			if err := binary.Write(&buf, binary.BigEndian, field); err != nil {
				return nil, fmt.Errorf("failed to encode attribute field: %w", err)
			}
		}
	}

	return buf.Bytes(), nil
}

// EncodeStatusMessage serializes a StatusMessage (empty request)
func EncodeStatusMessage(msg *StatusMessage) ([]byte, error) {
	// StatusMessage has no fields, return empty payload
	return []byte{}, nil
}

// EncodeWriteMessage serializes a WriteMessage
func EncodeWriteMessage(msg *WriteMessage) ([]byte, error) {
	var buf bytes.Buffer

	// Tag 0x01: Path (string slice)
	pathData, err := encodeStringSlice(0x01, msg.Path)
	if err != nil {
		return nil, fmt.Errorf("failed to encode path: %w", err)
	}
	buf.Write(pathData)

	// Tag 0x02: Value
	valueData, err := encodeValue(0x02, msg.Value)
	if err != nil {
		return nil, fmt.Errorf("failed to encode value: %w", err)
	}
	buf.Write(valueData)

	// Tag 0x03: Author (uint64)
	authorData, err := encodeUint64(0x03, msg.Author)
	if err != nil {
		return nil, fmt.Errorf("failed to encode author: %w", err)
	}
	buf.Write(authorData)

	return buf.Bytes(), nil
}

// EncodeWriteAckMessage serializes a WriteAckMessage
func EncodeWriteAckMessage(msg *WriteAckMessage) ([]byte, error) {
	var buf bytes.Buffer

	// Tag 0x01: Success (bool)
	successData, err := encodeBool(0x01, msg.Success)
	if err != nil {
		return nil, fmt.Errorf("failed to encode success: %w", err)
	}
	buf.Write(successData)

	// Tag 0x02: Error (string, optional)
	if msg.Error != "" {
		errorData, err := encodeString(0x02, msg.Error)
		if err != nil {
			return nil, fmt.Errorf("failed to encode error: %w", err)
		}
		buf.Write(errorData)
	}

	return buf.Bytes(), nil
}

// EncodePurgeMessage serializes a PurgeMessage
func EncodePurgeMessage(msg *PurgeMessage) ([]byte, error) {
	var buf bytes.Buffer

	// Tag 0x01: Path
	pathData, err := encodeStringSlice(0x01, msg.Path)
	if err != nil {
		return nil, fmt.Errorf("failed to encode path: %w", err)
	}
	buf.Write(pathData)

	// Tag 0x02: From timestamp
	fromData, err := encodeInt64(0x02, msg.From)
	if err != nil {
		return nil, fmt.Errorf("failed to encode from: %w", err)
	}
	buf.Write(fromData)

	// Tag 0x03: To timestamp
	toData, err := encodeInt64(0x03, msg.To)
	if err != nil {
		return nil, fmt.Errorf("failed to encode to: %w", err)
	}
	buf.Write(toData)

	// Tag 0x04: Author
	authorData, err := encodeUint64(0x04, msg.Author)
	if err != nil {
		return nil, fmt.Errorf("failed to encode author: %w", err)
	}
	buf.Write(authorData)

	return buf.Bytes(), nil
}

// EncodePurgeAckMessage serializes a PurgeAckMessage
func EncodePurgeAckMessage(msg *PurgeAckMessage) ([]byte, error) {
	var buf bytes.Buffer

	// Tag 0x01: Success
	successData, err := encodeBool(0x01, msg.Success)
	if err != nil {
		return nil, fmt.Errorf("failed to encode success: %w", err)
	}
	buf.Write(successData)

	// Tag 0x02: Error (optional)
	if msg.Error != "" {
		errorData, err := encodeString(0x02, msg.Error)
		if err != nil {
			return nil, fmt.Errorf("failed to encode error: %w", err)
		}
		buf.Write(errorData)
	}

	return buf.Bytes(), nil
}

// EncodeStatusResponseMessage serializes a StatusResponseMessage
func EncodeStatusResponseMessage(msg *StatusResponseMessage) ([]byte, error) {
	var buf bytes.Buffer

	// Tag 0x01: Uptime
	uptimeData, err := encodeInt64(0x01, msg.Uptime)
	if err != nil {
		return nil, fmt.Errorf("failed to encode uptime: %w", err)
	}
	buf.Write(uptimeData)

	// Tag 0x02: Node Identity
	identityData, err := encodeString(0x02, msg.NodeIdentity)
	if err != nil {
		return nil, fmt.Errorf("failed to encode node identity: %w", err)
	}
	buf.Write(identityData)

	// Tag 0x03: Local Socket
	socketData, err := encodeString(0x03, msg.LocalSocket)
	if err != nil {
		return nil, fmt.Errorf("failed to encode local socket: %w", err)
	}
	buf.Write(socketData)

	// Tag 0x04: Network Port
	portData, err := encodeInt32(0x04, int32(msg.NetworkPort))
	if err != nil {
		return nil, fmt.Errorf("failed to encode network port: %w", err)
	}
	buf.Write(portData)

	// Tag 0x05: Connections
	connData, err := encodeInt32(0x05, int32(msg.Connections))
	if err != nil {
		return nil, fmt.Errorf("failed to encode connections: %w", err)
	}
	buf.Write(connData)

	// Tag 0x06: Data Size
	sizeData, err := encodeInt64(0x06, msg.DataSize)
	if err != nil {
		return nil, fmt.Errorf("failed to encode data size: %w", err)
	}
	buf.Write(sizeData)

	return buf.Bytes(), nil
}

// EncodeWhoAmIResponseMessage serializes a WhoAmIResponseMessage. The agent ID
// is carried as int64 bits (bit-preserving) so the full uint64 range round-trips.
func EncodeWhoAmIResponseMessage(msg *WhoAmIResponseMessage) ([]byte, error) {
	var buf bytes.Buffer

	idData, err := encodeInt64(0x01, int64(msg.AgentID))
	if err != nil {
		return nil, fmt.Errorf("failed to encode agent id: %w", err)
	}
	buf.Write(idData)

	identData, err := encodeString(0x02, msg.Identity)
	if err != nil {
		return nil, fmt.Errorf("failed to encode identity: %w", err)
	}
	buf.Write(identData)

	return buf.Bytes(), nil
}

// EncodeAuthInitMessage serializes an AuthInitMessage.
func EncodeAuthInitMessage(msg *AuthInitMessage) ([]byte, error) {
	return encodeString(0x01, msg.Identity)
}

// EncodeAuthChallengeMessage serializes an AuthChallengeMessage. The opaque
// challenge blob is carried length-prefixed (encodeString is binary-safe).
func EncodeAuthChallengeMessage(msg *AuthChallengeMessage) ([]byte, error) {
	return encodeString(0x01, string(msg.Challenge))
}

// EncodeAuthResponseMessage serializes an AuthResponseMessage.
func EncodeAuthResponseMessage(msg *AuthResponseMessage) ([]byte, error) {
	return encodeString(0x01, string(msg.Response))
}

// EncodeAuthResultMessage serializes an AuthResultMessage. The agent ID is
// carried as int64 bits so the full uint64 range round-trips (matching WHOAMI).
func EncodeAuthResultMessage(msg *AuthResultMessage) ([]byte, error) {
	var buf bytes.Buffer

	successData, err := encodeBool(0x01, msg.Success)
	if err != nil {
		return nil, fmt.Errorf("failed to encode success: %w", err)
	}
	buf.Write(successData)

	idData, err := encodeInt64(0x02, int64(msg.AgentID))
	if err != nil {
		return nil, fmt.Errorf("failed to encode agent id: %w", err)
	}
	buf.Write(idData)

	if msg.Identity != "" {
		identData, err := encodeString(0x03, msg.Identity)
		if err != nil {
			return nil, fmt.Errorf("failed to encode identity: %w", err)
		}
		buf.Write(identData)
	}

	if msg.Error != "" {
		errData, err := encodeString(0x04, msg.Error)
		if err != nil {
			return nil, fmt.Errorf("failed to encode error: %w", err)
		}
		buf.Write(errData)
	}

	return buf.Bytes(), nil
}

// EncodeRegisterMessage serializes a RegisterMessage.
func EncodeRegisterMessage(msg *RegisterMessage) ([]byte, error) {
	var buf bytes.Buffer

	identData, err := encodeString(0x01, msg.Identity)
	if err != nil {
		return nil, fmt.Errorf("failed to encode identity: %w", err)
	}
	buf.Write(identData)

	keyData, err := encodeString(0x02, string(msg.PublicKey))
	if err != nil {
		return nil, fmt.Errorf("failed to encode public key: %w", err)
	}
	buf.Write(keyData)

	tokenData, err := encodeString(0x03, msg.Token)
	if err != nil {
		return nil, fmt.Errorf("failed to encode token: %w", err)
	}
	buf.Write(tokenData)

	return buf.Bytes(), nil
}

// EncodeRegisterResultMessage serializes a RegisterResultMessage. The agent ID
// is carried as int64 bits so the full uint64 range round-trips.
func EncodeRegisterResultMessage(msg *RegisterResultMessage) ([]byte, error) {
	var buf bytes.Buffer

	successData, err := encodeBool(0x01, msg.Success)
	if err != nil {
		return nil, fmt.Errorf("failed to encode success: %w", err)
	}
	buf.Write(successData)

	idData, err := encodeInt64(0x02, int64(msg.AgentID))
	if err != nil {
		return nil, fmt.Errorf("failed to encode agent id: %w", err)
	}
	buf.Write(idData)

	if msg.Error != "" {
		errData, err := encodeString(0x03, msg.Error)
		if err != nil {
			return nil, fmt.Errorf("failed to encode error: %w", err)
		}
		buf.Write(errData)
	}

	return buf.Bytes(), nil
}

// EncodeInviteCreateResultMessage serializes an InviteCreateResultMessage.
func EncodeInviteCreateResultMessage(msg *InviteCreateResultMessage) ([]byte, error) {
	var buf bytes.Buffer

	successData, err := encodeBool(0x01, msg.Success)
	if err != nil {
		return nil, fmt.Errorf("failed to encode success: %w", err)
	}
	buf.Write(successData)

	if msg.Token != "" {
		tokenData, err := encodeString(0x02, msg.Token)
		if err != nil {
			return nil, fmt.Errorf("failed to encode token: %w", err)
		}
		buf.Write(tokenData)
	}

	if msg.Error != "" {
		errData, err := encodeString(0x03, msg.Error)
		if err != nil {
			return nil, fmt.Errorf("failed to encode error: %w", err)
		}
		buf.Write(errData)
	}

	return buf.Bytes(), nil
}

// EncodeStopAckMessage serializes a StopAckMessage
func EncodeStopAckMessage(msg *StopAckMessage) ([]byte, error) {
	var buf bytes.Buffer

	// Tag 0x01: Success
	successData, err := encodeBool(0x01, msg.Success)
	if err != nil {
		return nil, fmt.Errorf("failed to encode success: %w", err)
	}
	buf.Write(successData)

	// Tag 0x02: Error (optional)
	if msg.Error != "" {
		errorData, err := encodeString(0x02, msg.Error)
		if err != nil {
			return nil, fmt.Errorf("failed to encode error: %w", err)
		}
		buf.Write(errorData)
	}

	return buf.Bytes(), nil
}

// EncodeCompactAckMessage serializes a CompactAckMessage
func EncodeCompactAckMessage(msg *CompactAckMessage) ([]byte, error) {
	var buf bytes.Buffer

	// Tag 0x01: Success
	successData, err := encodeBool(0x01, msg.Success)
	if err != nil {
		return nil, fmt.Errorf("failed to encode success: %w", err)
	}
	buf.Write(successData)

	// Tag 0x02: Error (optional)
	if msg.Error != "" {
		errorData, err := encodeString(0x02, msg.Error)
		if err != nil {
			return nil, fmt.Errorf("failed to encode error: %w", err)
		}
		buf.Write(errorData)
	}

	// Tag 0x03: Space Reclaimed
	spaceData, err := encodeInt64(0x03, msg.SpaceReclaimed)
	if err != nil {
		return nil, fmt.Errorf("failed to encode space reclaimed: %w", err)
	}
	buf.Write(spaceData)

	return buf.Bytes(), nil
}

// EncodeErrorMessage serializes an ErrorMessage
func EncodeErrorMessage(msg *ErrorMessage) ([]byte, error) {
	var buf bytes.Buffer

	// Tag 0x01: Error code
	codeData, err := encodeUint32(0x01, msg.Code)
	if err != nil {
		return nil, fmt.Errorf("failed to encode error code: %w", err)
	}
	buf.Write(codeData)

	// Tag 0x02: Error message
	msgData, err := encodeString(0x02, msg.Message)
	if err != nil {
		return nil, fmt.Errorf("failed to encode error message: %w", err)
	}
	buf.Write(msgData)

	return buf.Bytes(), nil
}

// Helper functions for encoding tagged fields

func encodeStringSlice(tag uint8, slice []string) ([]byte, error) {
	var buf bytes.Buffer

	// Write tag
	buf.WriteByte(tag)

	// Write length of slice
	if err := binary.Write(&buf, binary.BigEndian, uint32(len(slice))); err != nil {
		return nil, fmt.Errorf("failed to encode slice length: %w", err)
	}

	// Write each string with its length
	for _, s := range slice {
		data := []byte(s)
		if err := binary.Write(&buf, binary.BigEndian, uint32(len(data))); err != nil {
			return nil, fmt.Errorf("failed to encode string length: %w", err)
		}
		buf.Write(data)
	}

	return buf.Bytes(), nil
}

func encodeString(tag uint8, s string) ([]byte, error) {
	var buf bytes.Buffer

	buf.WriteByte(tag)
	data := []byte(s)
	if err := binary.Write(&buf, binary.BigEndian, uint32(len(data))); err != nil {
		return nil, fmt.Errorf("failed to encode string length: %w", err)
	}
	buf.Write(data)

	return buf.Bytes(), nil
}

func encodeValue(tag uint8, value storage.Value) ([]byte, error) {
	var buf bytes.Buffer

	buf.WriteByte(tag)

	// Encode type tag
	buf.WriteByte(value.TypeTag)

	// Encode data length and data
	if err := binary.Write(&buf, binary.BigEndian, uint32(len(value.Data))); err != nil {
		return nil, fmt.Errorf("failed to encode value data length: %w", err)
	}
	buf.Write(value.Data)

	return buf.Bytes(), nil
}

func encodeBool(tag uint8, b bool) ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteByte(tag)
	if b {
		buf.WriteByte(1)
	} else {
		buf.WriteByte(0)
	}
	return buf.Bytes(), nil
}

func encodeUint64(tag uint8, val uint64) ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteByte(tag)
	if err := binary.Write(&buf, binary.BigEndian, val); err != nil {
		return nil, fmt.Errorf("failed to encode uint64: %w", err)
	}
	return buf.Bytes(), nil
}

func encodeInt64(tag uint8, val int64) ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteByte(tag)
	if err := binary.Write(&buf, binary.BigEndian, val); err != nil {
		return nil, fmt.Errorf("failed to encode int64: %w", err)
	}
	return buf.Bytes(), nil
}

func encodeUint32(tag uint8, val uint32) ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteByte(tag)
	if err := binary.Write(&buf, binary.BigEndian, val); err != nil {
		return nil, fmt.Errorf("failed to encode uint32: %w", err)
	}
	return buf.Bytes(), nil
}

func encodeInt32(tag uint8, val int32) ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteByte(tag)
	if err := binary.Write(&buf, binary.BigEndian, val); err != nil {
		return nil, fmt.Errorf("failed to encode int32: %w", err)
	}
	return buf.Bytes(), nil
}

// EncodeCreateMeshMessage serializes a CreateMeshMessage
func EncodeCreateMeshMessage(msg *CreateMeshMessage) ([]byte, error) {
	// Tag 0x01: Mesh name
	return encodeString(0x01, msg.Name)
}

// EncodeCreateMeshAckMessage serializes a CreateMeshAckMessage
func EncodeCreateMeshAckMessage(msg *CreateMeshAckMessage) ([]byte, error) {
	var buf bytes.Buffer

	// Tag 0x01: Success
	successData, err := encodeBool(0x01, msg.Success)
	if err != nil {
		return nil, fmt.Errorf("failed to encode success: %w", err)
	}
	buf.Write(successData)

	// Tag 0x02: Message
	messageData, err := encodeString(0x02, msg.Message)
	if err != nil {
		return nil, fmt.Errorf("failed to encode message: %w", err)
	}
	buf.Write(messageData)

	// Tag 0x03: Mesh name (optional)
	if msg.MeshName != "" {
		meshNameData, err := encodeString(0x03, msg.MeshName)
		if err != nil {
			return nil, fmt.Errorf("failed to encode mesh name: %w", err)
		}
		buf.Write(meshNameData)
	}

	// Tag 0x04: Node ID (optional)
	if msg.NodeID != "" {
		nodeIDData, err := encodeString(0x04, msg.NodeID)
		if err != nil {
			return nil, fmt.Errorf("failed to encode node ID: %w", err)
		}
		buf.Write(nodeIDData)
	}

	// Tag 0x05: Is Founder
	founderData, err := encodeBool(0x05, msg.IsFounder)
	if err != nil {
		return nil, fmt.Errorf("failed to encode is founder: %w", err)
	}
	buf.Write(founderData)

	return buf.Bytes(), nil
}

// EncodeMeshStatusMessage serializes a MeshStatusMessage (empty request)
func EncodeMeshStatusMessage(msg *MeshStatusMessage) ([]byte, error) {
	// MeshStatusMessage has no fields, return empty payload
	return []byte{}, nil
}

// EncodeMeshStatusResponseMessage serializes a MeshStatusResponseMessage
func EncodeMeshStatusResponseMessage(msg *MeshStatusResponseMessage) ([]byte, error) {
	var buf bytes.Buffer

	// Tag 0x01: Mesh name
	meshNameData, err := encodeString(0x01, msg.MeshName)
	if err != nil {
		return nil, fmt.Errorf("failed to encode mesh name: %w", err)
	}
	buf.Write(meshNameData)

	// Tag 0x02: Status
	statusData, err := encodeString(0x02, msg.Status)
	if err != nil {
		return nil, fmt.Errorf("failed to encode status: %w", err)
	}
	buf.Write(statusData)

	// Tag 0x03: Is Founder
	founderData, err := encodeBool(0x03, msg.IsFounder)
	if err != nil {
		return nil, fmt.Errorf("failed to encode is founder: %w", err)
	}
	buf.Write(founderData)

	// Tag 0x04: Node Identity
	identityData, err := encodeString(0x04, msg.NodeIdentity)
	if err != nil {
		return nil, fmt.Errorf("failed to encode node identity: %w", err)
	}
	buf.Write(identityData)

	// Tag 0x05: Member Count
	memberData, err := encodeInt32(0x05, int32(msg.MemberCount))
	if err != nil {
		return nil, fmt.Errorf("failed to encode member count: %w", err)
	}
	buf.Write(memberData)

	// Tag 0x06: Zone Count
	zoneData, err := encodeInt32(0x06, int32(msg.ZoneCount))
	if err != nil {
		return nil, fmt.Errorf("failed to encode zone count: %w", err)
	}
	buf.Write(zoneData)

	// Tag 0x07: Founded At (optional)
	if msg.FoundedAt != nil {
		foundedData, err := encodeInt64(0x07, *msg.FoundedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to encode founded at: %w", err)
		}
		buf.Write(foundedData)
	}

	// Note: Bridges (map) would require more complex encoding
	// For simplicity, we'll skip bridge encoding for now
	// In a full implementation, we'd need a map encoding helper

	return buf.Bytes(), nil
}

// EncodeDiscoverMeshMessage serializes a DiscoverMeshMessage (empty request)
func EncodeDiscoverMeshMessage(msg *DiscoverMeshMessage) ([]byte, error) {
	// DiscoverMeshMessage has no fields, return empty payload
	return []byte{}, nil
}

// EncodeDiscoverMeshResponseMessage serializes a DiscoverMeshResponseMessage
func EncodeDiscoverMeshResponseMessage(msg *DiscoverMeshResponseMessage) ([]byte, error) {
	var buf bytes.Buffer

	// Tag 0x01: Mesh name
	meshNameData, err := encodeString(0x01, msg.MeshName)
	if err != nil {
		return nil, fmt.Errorf("failed to encode mesh name: %w", err)
	}
	buf.Write(meshNameData)

	// Tag 0x02: Founder identity
	founderData, err := encodeString(0x02, msg.FounderIdentity)
	if err != nil {
		return nil, fmt.Errorf("failed to encode founder identity: %w", err)
	}
	buf.Write(founderData)

	// Tag 0x03: Member count
	memberData, err := encodeInt32(0x03, int32(msg.MemberCount))
	if err != nil {
		return nil, fmt.Errorf("failed to encode member count: %w", err)
	}
	buf.Write(memberData)

	// Tag 0x04: Zone count
	zoneData, err := encodeInt32(0x04, int32(msg.ZoneCount))
	if err != nil {
		return nil, fmt.Errorf("failed to encode zone count: %w", err)
	}
	buf.Write(zoneData)

	// Tag 0x05: Requires auth
	authData, err := encodeBool(0x05, msg.RequiresAuth)
	if err != nil {
		return nil, fmt.Errorf("failed to encode requires auth: %w", err)
	}
	buf.Write(authData)

	// Tag 0x06: Founded at
	foundedData, err := encodeInt64(0x06, msg.FoundedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to encode founded at: %w", err)
	}
	buf.Write(foundedData)

	return buf.Bytes(), nil
}

// EncodeJoinMeshMessage serializes a JoinMeshMessage
func EncodeJoinMeshMessage(msg *JoinMeshMessage) ([]byte, error) {
	var buf bytes.Buffer

	// Tag 0x01: Node identity
	identityData, err := encodeString(0x01, msg.NodeIdentity)
	if err != nil {
		return nil, fmt.Errorf("failed to encode node identity: %w", err)
	}
	buf.Write(identityData)

	// Tag 0x02: Mesh name
	meshNameData, err := encodeString(0x02, msg.MeshName)
	if err != nil {
		return nil, fmt.Errorf("failed to encode mesh name: %w", err)
	}
	buf.Write(meshNameData)

	return buf.Bytes(), nil
}

// EncodeJoinMeshAckMessage serializes a JoinMeshAckMessage
func EncodeJoinMeshAckMessage(msg *JoinMeshAckMessage) ([]byte, error) {
	var buf bytes.Buffer

	// Tag 0x01: Success
	successData, err := encodeBool(0x01, msg.Success)
	if err != nil {
		return nil, fmt.Errorf("failed to encode success: %w", err)
	}
	buf.Write(successData)

	// Tag 0x02: Message
	messageData, err := encodeString(0x02, msg.Message)
	if err != nil {
		return nil, fmt.Errorf("failed to encode message: %w", err)
	}
	buf.Write(messageData)

	// Tag 0x03: Assigned zone (optional)
	if msg.AssignedZone != "" {
		zoneData, err := encodeString(0x03, msg.AssignedZone)
		if err != nil {
			return nil, fmt.Errorf("failed to encode assigned zone: %w", err)
		}
		buf.Write(zoneData)
	}

	// Tag 0x04: Member count
	memberData, err := encodeInt32(0x04, int32(msg.MemberCount))
	if err != nil {
		return nil, fmt.Errorf("failed to encode member count: %w", err)
	}
	buf.Write(memberData)

	return buf.Bytes(), nil
}

// EncodeCreateBridgeMessage serializes a CreateBridgeMessage
func EncodeCreateBridgeMessage(msg *CreateBridgeMessage) ([]byte, error) {
	var buf bytes.Buffer

	// Tag 0x01: Target address
	addressData, err := encodeString(0x01, msg.TargetAddress)
	if err != nil {
		return nil, fmt.Errorf("failed to encode target address: %w", err)
	}
	buf.Write(addressData)

	// Tag 0x02: Node identity
	identityData, err := encodeString(0x02, msg.NodeIdentity)
	if err != nil {
		return nil, fmt.Errorf("failed to encode node identity: %w", err)
	}
	buf.Write(identityData)

	return buf.Bytes(), nil
}

// EncodeCreateBridgeAckMessage serializes a CreateBridgeAckMessage
func EncodeCreateBridgeAckMessage(msg *CreateBridgeAckMessage) ([]byte, error) {
	var buf bytes.Buffer

	// Tag 0x01: Success
	successData, err := encodeBool(0x01, msg.Success)
	if err != nil {
		return nil, fmt.Errorf("failed to encode success: %w", err)
	}
	buf.Write(successData)

	// Tag 0x02: Message
	messageData, err := encodeString(0x02, msg.Message)
	if err != nil {
		return nil, fmt.Errorf("failed to encode message: %w", err)
	}
	buf.Write(messageData)

	// Tag 0x03: Target mesh name (optional)
	if msg.TargetMeshName != "" {
		meshNameData, err := encodeString(0x03, msg.TargetMeshName)
		if err != nil {
			return nil, fmt.Errorf("failed to encode target mesh name: %w", err)
		}
		buf.Write(meshNameData)
	}

	// Tag 0x04: Bridge identity (optional)
	if msg.BridgeIdentity != "" {
		identityData, err := encodeString(0x04, msg.BridgeIdentity)
		if err != nil {
			return nil, fmt.Errorf("failed to encode bridge identity: %w", err)
		}
		buf.Write(identityData)
	}

	// Tag 0x05: Bridge status (optional)
	if msg.BridgeStatus != "" {
		statusData, err := encodeString(0x05, msg.BridgeStatus)
		if err != nil {
			return nil, fmt.Errorf("failed to encode bridge status: %w", err)
		}
		buf.Write(statusData)
	}

	return buf.Bytes(), nil
}

// EncodeExecuteMessage serializes an ExecuteMessage
func EncodeExecuteMessage(msg *ExecuteMessage) ([]byte, error) {
	// Tag 0x01: MBL source code
	return encodeString(0x01, msg.Code)
}

// EncodeExecuteResponseMessage serializes an ExecuteResponseMessage
func EncodeExecuteResponseMessage(msg *ExecuteResponseMessage) ([]byte, error) {
	var buf bytes.Buffer

	// Tag 0x01: Success flag
	successData, err := encodeBool(0x01, msg.Success)
	if err != nil {
		return nil, fmt.Errorf("failed to encode success: %w", err)
	}
	buf.Write(successData)

	// Tag 0x02: Result value (only if success is true and result is not empty)
	if msg.Success && (len(msg.Result.Data) > 0 || msg.Result.TypeTag != 0) {
		resultData, err := encodeValue(0x02, msg.Result)
		if err != nil {
			return nil, fmt.Errorf("failed to encode result: %w", err)
		}
		buf.Write(resultData)
	}

	// Tag 0x03: Error message (only if success is false)
	if !msg.Success && msg.Error != "" {
		errorData, err := encodeString(0x03, msg.Error)
		if err != nil {
			return nil, fmt.Errorf("failed to encode error: %w", err)
		}
		buf.Write(errorData)
	}

	return buf.Bytes(), nil
}

// EncodeExtractMessage serializes an ExtractMessage
func EncodeExtractMessage(msg *ExtractMessage) ([]byte, error) {
	// Tag 0x01: Path to extract from
	return encodeStringSlice(0x01, msg.Path)
}

// EncodeExtractResponseMessage serializes an ExtractResponseMessage
func EncodeExtractResponseMessage(msg *ExtractResponseMessage) ([]byte, error) {
	var buf bytes.Buffer

	// Tag 0x01: Success flag
	successData, err := encodeBool(0x01, msg.Success)
	if err != nil {
		return nil, fmt.Errorf("failed to encode success: %w", err)
	}
	buf.Write(successData)

	if msg.Success {
		// Tag 0x02: Generated MBL script
		scriptData, err := encodeString(0x02, msg.Script)
		if err != nil {
			return nil, fmt.Errorf("failed to encode script: %w", err)
		}
		buf.Write(scriptData)
	} else {
		// Tag 0x03: Error message
		errorData, err := encodeString(0x03, msg.Error)
		if err != nil {
			return nil, fmt.Errorf("failed to encode error: %w", err)
		}
		buf.Write(errorData)
	}

	return buf.Bytes(), nil
}
