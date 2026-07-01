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
	READ_AT           = 0x16
	READ_AT_RESPONSE  = 0x17
	CHILDREN          = 0x18
	CHILDREN_RESPONSE = 0x19
	EXECUTE           = 0x1A
	EXECUTE_RESPONSE  = 0x1B

	// Control operations
	STATUS           = 0x20
	STATUS_RESPONSE  = 0x21
	STOP             = 0x22
	STOP_ACK         = 0x23
	COMPACT          = 0x24
	COMPACT_ACK      = 0x25
	EXTRACT          = 0x42
	EXTRACT_RESPONSE = 0x43
	WHOAMI           = 0x44
	WHOAMI_RESPONSE  = 0x45

	// Client authentication handshake (challenge-response over the client↔daemon
	// connection). A client presents a claimed identity (AUTH_INIT); the daemon
	// replies with a challenge encrypted to that identity's stored public key
	// (AUTH_CHALLENGE); the client proves possession of the private key by
	// decrypting it (AUTH_RESPONSE); the daemon reports the outcome and the
	// agent ID the connection is now bound to (AUTH_RESULT).
	AUTH_INIT      = 0x46
	AUTH_CHALLENGE = 0x47
	AUTH_RESPONSE  = 0x48
	AUTH_RESULT    = 0x49

	// Agent enrollment: a prospective agent redeems a one-time invite token to
	// register the public key it generated locally (REGISTER); the daemon reports
	// the outcome and the new agent's ID (REGISTER_RESULT).
	REGISTER        = 0x4A
	REGISTER_RESULT = 0x4B

	// Mesh management operations
	CREATE_MESH            = 0x26
	CREATE_MESH_ACK        = 0x27
	MESH_STATUS            = 0x28
	MESH_STATUS_RESPONSE   = 0x29
	DISCOVER_MESH          = 0x2A
	DISCOVER_MESH_RESPONSE = 0x2B
	JOIN_MESH              = 0x2C
	JOIN_MESH_ACK          = 0x2D
	CREATE_BRIDGE          = 0x2E
	CREATE_BRIDGE_ACK      = 0x2F
	DETACH                 = 0x40
	DETACH_ACK             = 0x41

	// Mesh networking operations
	KEY_EXCHANGE      = 0x30
	KEY_EXCHANGE_ACK  = 0x31
	IDENTITY_REQUEST  = 0x32
	IDENTITY_RESPONSE = 0x33
	PEER_REQUEST      = 0x34
	PEER_RESPONSE     = 0x35
	HEARTBEAT         = 0x36
	HEARTBEAT_ACK     = 0x37
	GOSSIP            = 0x38
	GOSSIP_ACK        = 0x39
	ZONE_ASSIGN       = 0x3A
	ZONE_TRANSFER     = 0x3B

	// Authority management operations (subscription model)
	AUTHORITY_ANNOUNCE     = 0x3C
	AUTHORITY_ANNOUNCE_ACK = 0x3D
	AUTHORITY_ELECTION     = 0x3E
	AUTHORITY_VOTE         = 0x3F

	// Error handling
	ERROR = 0xFF
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

// WhoAmIResponseMessage reports the identity the service has assigned to the
// current connection, so the client can resolve `my.*` under that identity.
type WhoAmIResponseMessage struct {
	AgentID  uint64 // Numeric agent ID the connection is authenticated as
	Identity string // Human-readable identity label (e.g. CV syllables)
}

// AuthInitMessage begins a client authentication handshake by presenting the
// identity the client claims to be. The daemon looks up that identity's stored
// public key to build the challenge.
type AuthInitMessage struct {
	Identity string // Claimed agent identity (CV-syllable string)
}

// AuthChallengeMessage carries the daemon's authentication challenge to the
// client. Challenge holds the opaque, security-layer-serialized challenge
// (security.SerializeAuthChallenge) so the protocol layer stays agnostic to the
// crypto encoding.
type AuthChallengeMessage struct {
	Challenge []byte // Serialized security.AuthChallengeMessage
}

// AuthResponseMessage carries the client's answer to the challenge. Response
// holds the opaque, security-layer-serialized response
// (security.SerializeAuthResponse).
type AuthResponseMessage struct {
	Response []byte // Serialized security.AuthResponseMessage
}

// AuthResultMessage reports the outcome of the handshake and, on success, the
// agent ID and identity the connection is now authenticated as.
type AuthResultMessage struct {
	Success  bool   // Whether authentication succeeded
	AgentID  uint64 // Agent ID the connection is now bound to (on success)
	Identity string // Identity label the connection is now bound to (on success)
	Error    string // Human-readable failure reason (on failure)
}

// RegisterMessage carries a new agent's enrollment: the desired identity, the
// public key it generated on its own device (raw big-endian bytes of the DH
// public key), and the one-time invite token authorizing the registration.
type RegisterMessage struct {
	Identity  string // Desired agent identity (CV-syllable string)
	PublicKey []byte // Agent's public key (big.Int bytes)
	Token     string // One-time enrollment invite token
}

// RegisterResultMessage reports the outcome of an enrollment. On success it
// carries the new agent's numeric ID.
type RegisterResultMessage struct {
	Success bool   // Whether registration succeeded
	AgentID uint64 // New agent's numeric ID (on success)
	Error   string // Human-readable failure reason (on failure)
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
	Success        bool   // Whether compaction started
	Error          string // Error message if compact failed
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
	FromNode    string        // Sending node identity
	Timestamp   int64         // Gossip timestamp
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
	ZoneID        string   // Zone identifier
	AuthorityNode string   // New authority node
	ReplicaNodes  []string // Replica nodes
	ZoneRange     []string // Path range for zone
}

// ZoneTransferMessage represents bulk zone data transfer
type ZoneTransferMessage struct {
	ZoneID    string // Zone identifier
	NewOwner  string // Identity of the new zone owner
	Timestamp int64  // When the transfer was initiated
	Data      []byte // Serialized zone data
	IsLast    bool   // Whether this is the final chunk
	ChunkID   uint32 // Chunk sequence number
}

// ErrorMessage represents an error response
type ErrorMessage struct {
	Code    uint32 // Error code
	Message string // Human-readable error message
}

// CreateMeshMessage represents a request to create a new mesh
type CreateMeshMessage struct {
	Name string // Mesh name to create
}

// CreateMeshAckMessage represents acknowledgment of mesh creation
type CreateMeshAckMessage struct {
	Success   bool   // Whether mesh creation succeeded
	Message   string // Human-readable message
	MeshName  string // Created mesh name
	NodeID    string // Node identity ID
	IsFounder bool   // Whether this node is the founder
}

// MeshStatusMessage represents a request for mesh status
type MeshStatusMessage struct {
	// No payload - simple status request
}

// MeshStatusResponseMessage represents mesh status information
type MeshStatusResponseMessage struct {
	MeshName     string                      // Current mesh name
	Status       string                      // Mesh status
	IsFounder    bool                        // Whether this node is founder
	NodeIdentity string                      // Node identity ID
	MemberCount  int                         // Number of mesh members
	ZoneCount    int                         // Number of zones
	FoundedAt    *int64                      // When mesh was founded (founder only)
	Bridges      map[string]BridgeStatusInfo // Bridge connections
}

// BridgeStatusInfo represents bridge connection status
type BridgeStatusInfo struct {
	Address     string // Bridge target address
	Status      string // Bridge status
	Identity    string // Bridge identity ID
	ConnectedAt *int64 // When bridge was connected
}

// DiscoverMeshMessage represents a request to discover mesh information
type DiscoverMeshMessage struct {
	// No payload - simple discovery request
}

// DiscoverMeshResponseMessage represents mesh discovery information
type DiscoverMeshResponseMessage struct {
	MeshName        string // Name of the mesh
	FounderIdentity string // Identity of the founder
	MemberCount     int    // Number of current members
	ZoneCount       int    // Number of zones
	RequiresAuth    bool   // Whether authentication is required
	FoundedAt       int64  // When the mesh was founded
}

// JoinMeshMessage represents a request to join a mesh
type JoinMeshMessage struct {
	NodeIdentity string // Identity of the joining node
	MeshName     string // Name of mesh to join (from discovery)
}

// JoinMeshAckMessage represents the response to a join request
type JoinMeshAckMessage struct {
	Success      bool   // Whether join was successful
	Message      string // Status/error message
	AssignedZone string // Zone assigned to the new member
	MemberCount  int    // Updated member count
}

// CreateBridgeMessage represents a request to create a bridge to another mesh
type CreateBridgeMessage struct {
	TargetAddress string // Address of target mesh to bridge to
	NodeIdentity  string // Identity of the bridge node
}

// CreateBridgeAckMessage represents the response to a bridge creation request
type CreateBridgeAckMessage struct {
	Success        bool   // Whether bridge creation was successful
	Message        string // Status/error message
	TargetMeshName string // Name of the target mesh
	BridgeIdentity string // Identity assigned in the target mesh
	BridgeStatus   string // Current bridge connection status
}

// DetachMessage represents a request to detach from a mesh or bridge
type DetachMessage struct {
	MeshName string // Mesh name to detach from (empty = primary mesh)
}

// DetachAckMessage represents the response to a detach request
type DetachAckMessage struct {
	Success        bool   // Whether detach was successful
	Message        string // Status/error message
	DetachType     string // "primary_mesh" or "bridge"
	MeshName       string // Name of mesh detached from
	PreviousRole   string // Previous role ("founder" or "member")
	BridgeIdentity string // Bridge identity (for bridge detach)
	ZonesMigrated  int    // Number of zones migrated to other nodes
	DataPreserved  bool   // Whether data was preserved
	DisconnectedAt int64  // When detach completed
}

// DepartureMessage represents an announcement of node departure
type DepartureMessage struct {
	NodeID    string // Identity of departing node
	Timestamp int64  // When departure was initiated
	Reason    string // Reason for departure
}

// BridgeClosureMessage represents notification of bridge closure
type BridgeClosureMessage struct {
	BridgeIdentity string // Identity of the closing bridge
	SourceMesh     string // Mesh that owned the bridge
	Timestamp      int64  // When closure was initiated
}

// AuthorityAnnounceMessage represents an authority change announcement
type AuthorityAnnounceMessage struct {
	Path            string // Path for which authority is being announced
	AuthorityID     string // Identity of the new authority node
	Reason          string // Reason for change: "promotion", "delegation", "split", "initial"
	Timestamp       int64  // When the authority change occurred
	FormerAuthority string // Identity of the previous authority (if any)
}

// AuthorityAnnounceAckMessage represents acknowledgment of authority announcement
type AuthorityAnnounceAckMessage struct {
	Path      string // Path that was announced
	Accepted  bool   // Whether the announcement was accepted
	Message   string // Status/error message
	Timestamp int64  // When the acknowledgment was sent
}

// AuthorityElectionMessage represents an authority promotion election
type AuthorityElectionMessage struct {
	Path            string   // Path for which election is being held
	FormerAuthority string   // Identity of the failed/departed authority
	Candidates      []string // List of candidate node identities
	ElectionID      string   // Unique identifier for this election
	StartTime       int64    // When the election started
	Timeout         int64    // When the election expires
}

// AuthorityVoteMessage represents a vote in an authority election
type AuthorityVoteMessage struct {
	ElectionID  string // Unique identifier for the election
	Path        string // Path being elected for
	VoterID     string // Identity of the voting node
	CandidateID string // Identity of the chosen candidate
	Timestamp   int64  // When the vote was cast
	Uptime      int64  // Uptime of the chosen candidate (for tiebreaking)
}

// ExecuteMessage represents a request to execute MBL code
type ExecuteMessage struct {
	Code string // MBL source code to execute
}

// ExecuteResponseMessage represents the response to MBL code execution
type ExecuteResponseMessage struct {
	Success bool          // Whether execution was successful
	Result  storage.Value // Execution result (nil if error occurred)
	Error   string        // Error message if execution failed
}

// ExtractMessage represents a request to extract MBL script from a subtree
type ExtractMessage struct {
	Path []string // Root path to extract from
}

// ExtractResponseMessage represents the response with generated MBL script
type ExtractResponseMessage struct {
	Success bool   // Whether extraction was successful
	Script  string // Generated MBL script
	Error   string // Error message if extraction failed
}
