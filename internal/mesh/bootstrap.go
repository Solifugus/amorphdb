// Package mesh implements AmorphDB's distributed mesh networking capabilities
package mesh

import (
	"fmt"
	"net"
	"time"

	"github.com/solifugus/amorphdb/internal/protocol"
)

// BootstrapManager handles node identity generation and mesh joining
type BootstrapManager struct {
	nodeIdentity    string
	keyPair        *KeyPair
	knownNodes     map[string]*NodeInfo
	isFirstNode    bool
	bootstrapAddr  string
}

// NodeInfo represents information about a peer node
type NodeInfo struct {
	Identity    string
	Address     string
	PublicKey   []byte
	LastSeen    time.Time
	IsOnline    bool
}

// KeyPair represents node cryptographic keys
type KeyPair struct {
	PublicKey  []byte
	PrivateKey []byte
}

// NewBootstrapManager creates a new bootstrap manager
func NewBootstrapManager() *BootstrapManager {
	return &BootstrapManager{
		knownNodes: make(map[string]*NodeInfo),
	}
}

// SelfGenesis initializes the first node in a new mesh
func (bm *BootstrapManager) SelfGenesis() error {
	// Generate identity for first node
	identity, err := GenerateNodeIdentity()
	if err != nil {
		return fmt.Errorf("failed to generate node identity: %w", err)
	}

	// Generate key pair
	keyPair, err := GenerateKeyPair()
	if err != nil {
		return fmt.Errorf("failed to generate key pair: %w", err)
	}

	bm.nodeIdentity = identity
	bm.keyPair = keyPair
	bm.isFirstNode = true

	fmt.Printf("Self-genesis complete. Node identity: %s\n", identity)
	return nil
}

// JoinMesh connects to an existing mesh via a seed node
func (bm *BootstrapManager) JoinMesh(seedAddr string) error {
	bm.bootstrapAddr = seedAddr

	// Connect to seed node
	conn, err := net.Dial("tcp", seedAddr)
	if err != nil {
		return fmt.Errorf("failed to connect to seed node %s: %w", seedAddr, err)
	}
	defer conn.Close()

	// Perform key exchange
	err = bm.performKeyExchange(conn)
	if err != nil {
		return fmt.Errorf("key exchange failed: %w", err)
	}

	// Request identity from seed node
	identity, err := bm.requestIdentity(conn)
	if err != nil {
		return fmt.Errorf("identity request failed: %w", err)
	}

	bm.nodeIdentity = identity

	// Exchange peer information
	err = bm.exchangePeerInfo(conn)
	if err != nil {
		return fmt.Errorf("peer exchange failed: %w", err)
	}

	fmt.Printf("Successfully joined mesh. Node identity: %s\n", identity)
	return nil
}

// performKeyExchange exchanges public keys with the seed node
func (bm *BootstrapManager) performKeyExchange(conn net.Conn) error {
	// Generate temporary key pair if not already present
	if bm.keyPair == nil {
		keyPair, err := GenerateKeyPair()
		if err != nil {
			return err
		}
		bm.keyPair = keyPair
	}

	// Create key exchange message
	keyExchangeMsg := &protocol.Message{
		Version:  protocol.Version,
		Type:     protocol.KEY_EXCHANGE,
		Sequence: 1,
		Payload:  bm.keyPair.PublicKey,
	}

	// Send key exchange message
	data, err := protocol.EncodeMessage(keyExchangeMsg)
	if err != nil {
		return fmt.Errorf("failed to encode key exchange: %w", err)
	}

	_, err = conn.Write(data)
	if err != nil {
		return fmt.Errorf("failed to send key exchange: %w", err)
	}

	// Read response (seed node's public key)
	response, err := bm.readMessage(conn)
	if err != nil {
		return fmt.Errorf("failed to read key exchange response: %w", err)
	}

	if response.Type == protocol.ERROR {
		return fmt.Errorf("key exchange rejected by seed node")
	}

	if response.Type != protocol.KEY_EXCHANGE_ACK {
		return fmt.Errorf("unexpected response type: %02x", response.Type)
	}

	// Store seed node's public key
	seedPublicKey := response.Payload
	fmt.Printf("Key exchange completed with seed node (key length: %d bytes)\n", len(seedPublicKey))

	return nil
}

// requestIdentity requests a node identity from the seed node
func (bm *BootstrapManager) requestIdentity(conn net.Conn) (string, error) {
	// Create identity request message
	identityMsg := &protocol.Message{
		Version:  protocol.Version,
		Type:     protocol.IDENTITY_REQUEST,
		Sequence: 2,
		Payload:  []byte{}, // Empty payload
	}

	// Send identity request
	data, err := protocol.EncodeMessage(identityMsg)
	if err != nil {
		return "", fmt.Errorf("failed to encode identity request: %w", err)
	}

	_, err = conn.Write(data)
	if err != nil {
		return "", fmt.Errorf("failed to send identity request: %w", err)
	}

	// Read identity response
	response, err := bm.readMessage(conn)
	if err != nil {
		return "", fmt.Errorf("failed to read identity response: %w", err)
	}

	if response.Type == protocol.ERROR {
		return "", fmt.Errorf("identity request rejected by seed node")
	}

	if response.Type != protocol.IDENTITY_RESPONSE {
		return "", fmt.Errorf("unexpected response type: %02x", response.Type)
	}

	identity := string(response.Payload)
	return identity, nil
}

// exchangePeerInfo exchanges known peer information with the seed node
func (bm *BootstrapManager) exchangePeerInfo(conn net.Conn) error {
	// Request peer list from seed node
	peerRequestMsg := &protocol.Message{
		Version:  protocol.Version,
		Type:     protocol.PEER_REQUEST,
		Sequence: 3,
		Payload:  []byte{}, // Empty payload
	}

	// Send peer request
	data, err := protocol.EncodeMessage(peerRequestMsg)
	if err != nil {
		return fmt.Errorf("failed to encode peer request: %w", err)
	}

	_, err = conn.Write(data)
	if err != nil {
		return fmt.Errorf("failed to send peer request: %w", err)
	}

	// Read peer list response
	response, err := bm.readMessage(conn)
	if err != nil {
		return fmt.Errorf("failed to read peer response: %w", err)
	}

	if response.Type == protocol.ERROR {
		return fmt.Errorf("peer request rejected by seed node")
	}

	if response.Type != protocol.PEER_RESPONSE {
		return fmt.Errorf("unexpected response type: %02x", response.Type)
	}

	// Parse peer list
	err = bm.parsePeerList(response.Payload)
	if err != nil {
		return fmt.Errorf("failed to parse peer list: %w", err)
	}

	fmt.Printf("Received information about %d peer nodes\n", len(bm.knownNodes))
	return nil
}

// parsePeerList parses a peer list payload and updates known nodes
func (bm *BootstrapManager) parsePeerList(payload []byte) error {
	// Simple format: each peer is: identity_length(1) + identity + address_length(1) + address
	offset := 0

	for offset < len(payload) {
		if offset >= len(payload) {
			break
		}

		// Read identity length
		identityLen := int(payload[offset])
		offset++

		if offset+identityLen > len(payload) {
			return fmt.Errorf("invalid peer list format: identity overflow")
		}

		// Read identity
		identity := string(payload[offset : offset+identityLen])
		offset += identityLen

		if offset >= len(payload) {
			return fmt.Errorf("invalid peer list format: missing address")
		}

		// Read address length
		addrLen := int(payload[offset])
		offset++

		if offset+addrLen > len(payload) {
			return fmt.Errorf("invalid peer list format: address overflow")
		}

		// Read address
		address := string(payload[offset : offset+addrLen])
		offset += addrLen

		// Add to known nodes
		bm.knownNodes[identity] = &NodeInfo{
			Identity:  identity,
			Address:   address,
			PublicKey: nil, // Will be filled during key exchange
			LastSeen:  time.Now(),
			IsOnline:  true,
		}
	}

	return nil
}

// readMessage reads a complete protocol message from a connection
func (bm *BootstrapManager) readMessage(conn net.Conn) (*protocol.Message, error) {
	// Set read timeout
	conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	defer conn.SetReadDeadline(time.Time{})

	// Read header (10 bytes)
	headerBytes := make([]byte, 10)
	n := 0
	for n < len(headerBytes) {
		read, err := conn.Read(headerBytes[n:])
		if err != nil {
			return nil, fmt.Errorf("failed to read header: %w", err)
		}
		n += read
	}

	// Parse payload length
	payloadLength := uint32(headerBytes[6])<<24 | uint32(headerBytes[7])<<16 |
		uint32(headerBytes[8])<<8 | uint32(headerBytes[9])

	// Read payload
	payload := make([]byte, payloadLength)
	n = 0
	for n < len(payload) {
		read, err := conn.Read(payload[n:])
		if err != nil {
			return nil, fmt.Errorf("failed to read payload: %w", err)
		}
		n += read
	}

	// Read checksum
	checksumBytes := make([]byte, 4)
	n = 0
	for n < len(checksumBytes) {
		read, err := conn.Read(checksumBytes[n:])
		if err != nil {
			return nil, fmt.Errorf("failed to read checksum: %w", err)
		}
		n += read
	}

	// Reconstruct and decode message
	fullMessage := make([]byte, 0, 14+payloadLength)
	fullMessage = append(fullMessage, headerBytes...)
	fullMessage = append(fullMessage, payload...)
	fullMessage = append(fullMessage, checksumBytes...)

	return protocol.DecodeMessage(fullMessage)
}

// GetNodeIdentity returns the current node's identity
func (bm *BootstrapManager) GetNodeIdentity() string {
	return bm.nodeIdentity
}

// GetKnownNodes returns a copy of the known nodes map
func (bm *BootstrapManager) GetKnownNodes() map[string]*NodeInfo {
	nodes := make(map[string]*NodeInfo)
	for id, info := range bm.knownNodes {
		nodes[id] = &NodeInfo{
			Identity:  info.Identity,
			Address:   info.Address,
			PublicKey: info.PublicKey,
			LastSeen:  info.LastSeen,
			IsOnline:  info.IsOnline,
		}
	}
	return nodes
}

// IsFirstNode returns whether this node is the first in the mesh
func (bm *BootstrapManager) IsFirstNode() bool {
	return bm.isFirstNode
}
