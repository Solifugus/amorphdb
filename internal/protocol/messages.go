package protocol

import (
	"github.com/solifugus/amorphdb/internal/storage"
)

// Protocol version
const Version = 0x01

// Message types
const (
	// Basic operations
	READ          = 0x10
	READ_RESPONSE = 0x11
	WRITE         = 0x12
	WRITE_ACK     = 0x13
	PURGE         = 0x14
	PURGE_ACK     = 0x15

	// Advanced operations
	READ_AT         = 0x16
	READ_AT_RESPONSE = 0x17
	CHILDREN        = 0x18
	CHILDREN_RESPONSE = 0x19

	// Control operations
	STATUS        = 0x20
	STATUS_RESPONSE = 0x21
	STOP          = 0x22
	STOP_ACK      = 0x23
	COMPACT       = 0x24
	COMPACT_ACK   = 0x25

	// Mesh networking operations
	KEY_EXCHANGE     = 0x30
	KEY_EXCHANGE_ACK = 0x31
	IDENTITY_REQUEST = 0x32
	IDENTITY_RESPONSE = 0x33
	PEER_REQUEST     = 0x34
	PEER_RESPONSE    = 0x35
	HEARTBEAT        = 0x36
	HEARTBEAT_ACK    = 0x37
	GOSSIP           = 0x38
	GOSSIP_ACK       = 0x39
	ZONE_ASSIGN      = 0x3A
	ZONE_TRANSFER    = 0x3B

	// Error handling
	ERROR         = 0xFF
)

// Message represents a protocol message with header and payload
type Message struct {
	Version       uint8  // Protocol version (0x01)
	Type          uint8  // Message type code
	Sequence      uint32 // Correlation sequence number
	PayloadLength uint32 // Variable payload length
	Payload       []byte // Tagged field encoding
	Checksum      uint32 // CRC32 corruption detection
}

// ReadMessage represents a request to read data at a path
type ReadMessage struct {
	Path []string // Path to read
}

// ReadResponseMessage represents a response containing requested data
type ReadResponseMessage struct {
	Value storage.Value // The retrieved value
}

// WriteMessage represents a request to write value to a path
type WriteMessage struct {
	Path   []string      // Path to write to
	Value  storage.Value // Value to write
	Author uint64        // Agent performing the write
}

// WriteAckMessage represents an acknowledgment of a write operation
type WriteAckMessage struct {
	Success bool   // Whether the write succeeded
	Error   string // Error message if write failed
}

// PurgeMessage represents a request to purge instances in a time range
type PurgeMessage struct {
	Path   []string // Path to purge
	From   int64    // Start timestamp
	To     int64    // End timestamp
	Author uint64   // Agent performing the purge
}

// PurgeAckMessage represents an acknowledgment of a purge operation
type PurgeAckMessage struct {
	Success bool   // Whether the purge succeeded
	Error   string // Error message if purge failed
}

// ReadAtMessage represents a request to read data at a specific timestamp
type ReadAtMessage struct {
	Path      []string // Path to read
	Timestamp int64    // Specific timestamp
}

// ReadAtResponseMessage represents a response with historical data
type ReadAtResponseMessage struct {
	Value storage.Value // The retrieved historical value
}

// ChildrenMessage represents a request for child attributes
type ChildrenMessage struct {
	Path []string // Path to get children for
}

// ChildrenResponseMessage represents a response with child attributes
type ChildrenResponseMessage struct {
	Children []storage.Attribute // Child attributes
}

// StatusMessage represents a request for service status
type StatusMessage struct {
	// No payload - simple status request
}

// StatusResponseMessage represents service status information
type StatusResponseMessage struct {
	Uptime       int64  // Service uptime in seconds
	NodeIdentity string // Node identity string
	LocalSocket  string // Local socket path
	NetworkPort  int    // Network port
	Connections  int    // Active connection count
	DataSize     int64  // Total data size in bytes
}

// StopMessage represents a request to stop the service
type StopMessage struct {
	Force bool // Whether to force immediate shutdown
}

// StopAckMessage represents an acknowledgment of stop request
type StopAckMessage struct {
	Success bool   // Whether shutdown initiated
	Error   string // Error message if stop failed
}

// CompactMessage represents a request to compact/defragment data
type CompactMessage struct {
	// No payload - simple compact request
}

// CompactAckMessage represents an acknowledgment of compact request
type CompactAckMessage struct {
	Success       bool   // Whether compaction started
	Error         string // Error message if compact failed
	SpaceReclaimed int64  // Bytes reclaimed by compaction
}

// KeyExchangeMessage represents a public key exchange
type KeyExchangeMessage struct {
	PublicKey []byte // Node's public key
}

// KeyExchangeAckMessage represents acknowledgment of key exchange
type KeyExchangeAckMessage struct {
	PublicKey []byte // Responding node's public key
	Success   bool   // Whether key exchange succeeded
}

// IdentityRequestMessage represents a request for node identity assignment
type IdentityRequestMessage struct {
	// No payload - simple identity request
}

// IdentityResponseMessage represents assignment of node identity
type IdentityResponseMessage struct {
	Identity string // Assigned node identity
}

// PeerRequestMessage represents a request for peer node list
type PeerRequestMessage struct {
	// No payload - simple peer list request
}

// PeerResponseMessage represents a list of known peer nodes
type PeerResponseMessage struct {
	Peers []PeerInfo // List of peer node information
}

// PeerInfo represents information about a peer node
type PeerInfo struct {
	Identity  string // Node identity
	Address   string // Network address
	PublicKey []byte // Node's public key
}

// HeartbeatMessage represents a heartbeat between nodes
type HeartbeatMessage struct {
	FromNode  string // Sending node identity
	Timestamp int64  // Heartbeat timestamp
	ZoneData  []byte // Zone replication data
}

// HeartbeatAckMessage represents acknowledgment of heartbeat
type HeartbeatAckMessage struct {
	ToNode     string // Responding node identity
	Timestamp  int64  // Response timestamp
	Successful bool   // Whether heartbeat was processed
}

// GossipMessage represents mesh membership gossip
type GossipMessage struct {
	FromNode    string      // Sending node identity
	Timestamp   int64       // Gossip timestamp
	NodeUpdates []*NodeUpdate // Node status updates
}

// NodeUpdate represents a change in node status
type NodeUpdate struct {
	Identity string // Node identity
	Status   string // Status: "joined", "left", "failed"
	Address  string // Node address
	ZoneInfo []byte // Zone assignment information
}

// GossipAckMessage represents acknowledgment of gossip
type GossipAckMessage struct {
	ToNode    string // Responding node identity
	Timestamp int64  // Acknowledgment timestamp
}

// ZoneAssignMessage represents zone assignment/transfer
type ZoneAssignMessage struct {
	ZoneID      string   // Zone identifier
	AuthorityNode string // New authority node
	ReplicaNodes  []string // Replica nodes
	ZoneRange     []string // Path range for zone
}

// ZoneTransferMessage represents bulk zone data transfer
type ZoneTransferMessage struct {
	ZoneID   string // Zone identifier
	Data     []byte // Serialized zone data
	IsLast   bool   // Whether this is the final chunk
	ChunkID  uint32 // Chunk sequence number
}

// ErrorMessage represents an error response
type ErrorMessage struct {
	Code    uint32 // Error code
	Message string // Human-readable error message
}