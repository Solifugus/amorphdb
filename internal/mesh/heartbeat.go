package mesh

import (
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/solifugus/amorphdb/internal/protocol"
)

// HeartbeatManager manages heartbeat and failure detection for mesh nodes
type HeartbeatManager struct {
	mu               sync.RWMutex
	nodeIdentity     string
	peers           map[string]*PeerConnection // Connected peers
	clockOffset     int64                      // Clock offset for synchronization
	tickInterval    time.Duration              // Heartbeat interval (⅓ second)
	failureTimeout  time.Duration              // Timeout before marking node as failed
	isRunning       bool
	stopChannel     chan struct{}
	onNodeFailed    func(string)               // Callback for node failure
	onNodeRecovered func(string)               // Callback for node recovery
}

// PeerConnection represents a connection to a peer node
type PeerConnection struct {
	Identity        string
	Address         string
	Conn           net.Conn
	LastHeartbeat  time.Time
	LastResponse   time.Time
	IsOnline       bool
	FailureCount   int
	ZoneData       []byte  // Latest zone replication data
	ClockSkew      int64   // Clock difference with this peer
	RTT            time.Duration // Round-trip time
}

// NewHeartbeatManager creates a new heartbeat manager
func NewHeartbeatManager(nodeIdentity string) *HeartbeatManager {
	return &HeartbeatManager{
		nodeIdentity:   nodeIdentity,
		peers:         make(map[string]*PeerConnection),
		clockOffset:   0,
		tickInterval:  time.Millisecond * 333, // ⅓ second
		failureTimeout: time.Second * 3,        // 3 seconds = ~9 missed heartbeats
		stopChannel:   make(chan struct{}),
	}
}

// Start begins the heartbeat process
func (hm *HeartbeatManager) Start() {
	hm.mu.Lock()
	if hm.isRunning {
		hm.mu.Unlock()
		return
	}
	hm.isRunning = true
	hm.mu.Unlock()

	go hm.heartbeatLoop()
}

// Stop stops the heartbeat process
func (hm *HeartbeatManager) Stop() {
	hm.mu.Lock()
	if !hm.isRunning {
		hm.mu.Unlock()
		return
	}
	hm.isRunning = false
	hm.mu.Unlock()

	close(hm.stopChannel)

	// Close all peer connections
	hm.mu.Lock()
	for _, peer := range hm.peers {
		if peer.Conn != nil {
			peer.Conn.Close()
		}
	}
	hm.mu.Unlock()
}

// AddPeer adds a peer node for heartbeat monitoring
func (hm *HeartbeatManager) AddPeer(identity, address string) error {
	hm.mu.Lock()
	defer hm.mu.Unlock()

	if _, exists := hm.peers[identity]; exists {
		return fmt.Errorf("peer %s already exists", identity)
	}

	peer := &PeerConnection{
		Identity:      identity,
		Address:       address,
		Conn:          nil,
		LastHeartbeat: time.Time{},
		LastResponse:  time.Time{},
		IsOnline:      false,
		FailureCount:  0,
		ZoneData:      nil,
		ClockSkew:     0,
		RTT:           0,
	}

	hm.peers[identity] = peer
	return nil
}

// RemovePeer removes a peer from heartbeat monitoring
func (hm *HeartbeatManager) RemovePeer(identity string) {
	hm.mu.Lock()
	defer hm.mu.Unlock()

	if peer, exists := hm.peers[identity]; exists {
		if peer.Conn != nil {
			peer.Conn.Close()
		}
		delete(hm.peers, identity)
	}
}

// GetPeerStatus returns the status of all peers
func (hm *HeartbeatManager) GetPeerStatus() map[string]*PeerConnection {
	hm.mu.RLock()
	defer hm.mu.RUnlock()

	status := make(map[string]*PeerConnection)
	for identity, peer := range hm.peers {
		// Create a copy to avoid concurrent access issues
		status[identity] = &PeerConnection{
			Identity:      peer.Identity,
			Address:       peer.Address,
			Conn:          nil, // Don't expose the connection
			LastHeartbeat: peer.LastHeartbeat,
			LastResponse:  peer.LastResponse,
			IsOnline:      peer.IsOnline,
			FailureCount:  peer.FailureCount,
			ZoneData:      peer.ZoneData,
			ClockSkew:     peer.ClockSkew,
			RTT:           peer.RTT,
		}
	}

	return status
}

// SetFailureCallback sets the callback for node failure events
func (hm *HeartbeatManager) SetFailureCallback(callback func(string)) {
	hm.mu.Lock()
	defer hm.mu.Unlock()
	hm.onNodeFailed = callback
}

// SetRecoveryCallback sets the callback for node recovery events
func (hm *HeartbeatManager) SetRecoveryCallback(callback func(string)) {
	hm.mu.Lock()
	defer hm.mu.Unlock()
	hm.onNodeRecovered = callback
}

// UpdateZoneData updates the zone data to send in heartbeats
func (hm *HeartbeatManager) UpdateZoneData(identity string, data []byte) {
	hm.mu.Lock()
	defer hm.mu.Unlock()

	if peer, exists := hm.peers[identity]; exists {
		peer.ZoneData = data
	}
}

// heartbeatLoop is the main heartbeat processing loop
func (hm *HeartbeatManager) heartbeatLoop() {
	ticker := time.NewTicker(hm.tickInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			hm.processTick()
		case <-hm.stopChannel:
			return
		}
	}
}

// processTick handles one heartbeat tick
func (hm *HeartbeatManager) processTick() {
	now := time.Now()

	hm.mu.RLock()
	peers := make([]*PeerConnection, 0, len(hm.peers))
	for _, peer := range hm.peers {
		peers = append(peers, peer)
	}
	hm.mu.RUnlock()

	// Send heartbeats to all peers
	for _, peer := range peers {
		go hm.sendHeartbeat(peer, now)
	}

	// Check for failures
	hm.checkForFailures(now)
}

// sendHeartbeat sends a heartbeat to a specific peer
func (hm *HeartbeatManager) sendHeartbeat(peer *PeerConnection, timestamp time.Time) {
	// Ensure connection is established
	if peer.Conn == nil {
		err := hm.connectToPeer(peer)
		if err != nil {
			hm.handleConnectionError(peer, err)
			return
		}
	}

	// Create heartbeat message
	heartbeatMsg := &protocol.Message{
		Version:  protocol.Version,
		Type:     protocol.HEARTBEAT,
		Sequence: uint32(timestamp.Unix()),
		Payload:  hm.encodeHeartbeatPayload(timestamp, peer.ZoneData),
	}

	// Send heartbeat
	startTime := time.Now()
	data, err := protocol.EncodeMessage(heartbeatMsg)
	if err != nil {
		hm.handleConnectionError(peer, fmt.Errorf("encode heartbeat: %w", err))
		return
	}

	peer.Conn.SetWriteDeadline(time.Now().Add(time.Second))
	_, err = peer.Conn.Write(data)
	if err != nil {
		hm.handleConnectionError(peer, fmt.Errorf("send heartbeat: %w", err))
		return
	}

	// Read response
	peer.Conn.SetReadDeadline(time.Now().Add(time.Second))
	response, err := hm.readHeartbeatResponse(peer.Conn)
	if err != nil {
		hm.handleConnectionError(peer, fmt.Errorf("read response: %w", err))
		return
	}

	// Calculate RTT
	rtt := time.Since(startTime)

	// Update peer status
	hm.mu.Lock()
	peer.LastHeartbeat = timestamp
	peer.LastResponse = time.Now()
	peer.RTT = rtt
	peer.FailureCount = 0

	// Check if this is a recovery
	wasOffline := !peer.IsOnline
	peer.IsOnline = true

	// Process response data (clock synchronization, etc.)
	hm.processHeartbeatResponse(peer, response)
	hm.mu.Unlock()

	// Notify recovery if node was offline
	if wasOffline && hm.onNodeRecovered != nil {
		go hm.onNodeRecovered(peer.Identity)
	}
}

// connectToPeer establishes a connection to a peer
func (hm *HeartbeatManager) connectToPeer(peer *PeerConnection) error {
	conn, err := net.DialTimeout("tcp", peer.Address, time.Second*2)
	if err != nil {
		return fmt.Errorf("dial %s: %w", peer.Address, err)
	}

	peer.Conn = conn
	return nil
}

// handleConnectionError handles errors during heartbeat communication
func (hm *HeartbeatManager) handleConnectionError(peer *PeerConnection, err error) {
	hm.mu.Lock()
	defer hm.mu.Unlock()

	// Close existing connection
	if peer.Conn != nil {
		peer.Conn.Close()
		peer.Conn = nil
	}

	peer.FailureCount++

	// Check if we should mark as failed
	if peer.FailureCount >= 3 && peer.IsOnline {
		peer.IsOnline = false
		if hm.onNodeFailed != nil {
			go hm.onNodeFailed(peer.Identity)
		}
	}
}

// readHeartbeatResponse reads a heartbeat response message
func (hm *HeartbeatManager) readHeartbeatResponse(conn net.Conn) (*protocol.Message, error) {
	// Read header (10 bytes)
	headerBytes := make([]byte, 10)
	n := 0
	for n < len(headerBytes) {
		read, err := conn.Read(headerBytes[n:])
		if err != nil {
			return nil, fmt.Errorf("read header: %w", err)
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
			return nil, fmt.Errorf("read payload: %w", err)
		}
		n += read
	}

	// Read checksum
	checksumBytes := make([]byte, 4)
	n = 0
	for n < len(checksumBytes) {
		read, err := conn.Read(checksumBytes[n:])
		if err != nil {
			return nil, fmt.Errorf("read checksum: %w", err)
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

// processHeartbeatResponse processes a heartbeat response for clock sync
func (hm *HeartbeatManager) processHeartbeatResponse(peer *PeerConnection, response *protocol.Message) {
	if response.Type != protocol.HEARTBEAT_ACK {
		return
	}

	// Decode response timestamp and calculate clock skew
	if len(response.Payload) >= 8 {
		responseTime := int64(response.Payload[0])<<56 | int64(response.Payload[1])<<48 |
			int64(response.Payload[2])<<40 | int64(response.Payload[3])<<32 |
			int64(response.Payload[4])<<24 | int64(response.Payload[5])<<16 |
			int64(response.Payload[6])<<8 | int64(response.Payload[7])

		now := time.Now().UnixNano()
		peer.ClockSkew = responseTime - now

		// Update global clock offset based on peer skews
		hm.updateClockOffset()
	}
}

// updateClockOffset updates the global clock offset based on peer data
func (hm *HeartbeatManager) updateClockOffset() {
	if len(hm.peers) == 0 {
		return
	}

	// Calculate average clock skew
	var totalSkew int64
	var onlineCount int

	for _, peer := range hm.peers {
		if peer.IsOnline {
			totalSkew += peer.ClockSkew
			onlineCount++
		}
	}

	if onlineCount > 0 {
		hm.clockOffset = totalSkew / int64(onlineCount)
	}
}

// checkForFailures checks all peers for timeout failures
func (hm *HeartbeatManager) checkForFailures(now time.Time) {
	hm.mu.Lock()
	defer hm.mu.Unlock()

	for _, peer := range hm.peers {
		if !peer.IsOnline {
			continue
		}

		// Check if peer has timed out
		timeSinceResponse := now.Sub(peer.LastResponse)
		if timeSinceResponse > hm.failureTimeout {
			peer.IsOnline = false
			if hm.onNodeFailed != nil {
				go hm.onNodeFailed(peer.Identity)
			}
		}
	}
}

// encodeHeartbeatPayload creates the payload for a heartbeat message
func (hm *HeartbeatManager) encodeHeartbeatPayload(timestamp time.Time, zoneData []byte) []byte {
	// Simple encoding: timestamp (8 bytes) + zone data length (4 bytes) + zone data
	payload := make([]byte, 12+len(zoneData))

	// Encode timestamp (nanoseconds since epoch)
	ts := timestamp.UnixNano()
	payload[0] = byte(ts >> 56)
	payload[1] = byte(ts >> 48)
	payload[2] = byte(ts >> 40)
	payload[3] = byte(ts >> 32)
	payload[4] = byte(ts >> 24)
	payload[5] = byte(ts >> 16)
	payload[6] = byte(ts >> 8)
	payload[7] = byte(ts)

	// Encode zone data length
	zoneLen := uint32(len(zoneData))
	payload[8] = byte(zoneLen >> 24)
	payload[9] = byte(zoneLen >> 16)
	payload[10] = byte(zoneLen >> 8)
	payload[11] = byte(zoneLen)

	// Copy zone data
	copy(payload[12:], zoneData)

	return payload
}

// GetSynchronizedTime returns the current time adjusted for clock offset
func (hm *HeartbeatManager) GetSynchronizedTime() time.Time {
	hm.mu.RLock()
	offset := hm.clockOffset
	hm.mu.RUnlock()

	return time.Now().Add(time.Duration(offset) * time.Nanosecond)
}

// GetClockOffset returns the current clock offset in nanoseconds
func (hm *HeartbeatManager) GetClockOffset() int64 {
	hm.mu.RLock()
	defer hm.mu.RUnlock()
	return hm.clockOffset
}

// GetOnlinePeers returns a list of online peer identities
func (hm *HeartbeatManager) GetOnlinePeers() []string {
	hm.mu.RLock()
	defer hm.mu.RUnlock()

	online := make([]string, 0)
	for identity, peer := range hm.peers {
		if peer.IsOnline {
			online = append(online, identity)
		}
	}

	return online
}

// GetPeerCount returns the total number of peers and online peer count
func (hm *HeartbeatManager) GetPeerCount() (total, online int) {
	hm.mu.RLock()
	defer hm.mu.RUnlock()

	total = len(hm.peers)
	for _, peer := range hm.peers {
		if peer.IsOnline {
			online++
		}
	}

	return total, online
}
