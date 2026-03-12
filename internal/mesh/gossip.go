package mesh

import (
	"fmt"
	"math/rand"
	"net"
	"sync"
	"time"

	"github.com/solifugus/amorphdb/internal/protocol"
)

// GossipManager manages mesh-wide information propagation
type GossipManager struct {
	mu              sync.RWMutex
	nodeIdentity    string
	allNodes        map[string]*NodeStatus // All known nodes in the mesh
	gossipPeers     []string               // Random subset for gossip propagation
	gossipInterval  time.Duration          // How often to gossip
	maxGossipPeers  int                    // Maximum peers to gossip with each round
	isRunning       bool
	stopChannel     chan struct{}
	pendingUpdates  []*protocol.NodeUpdate // Updates to propagate
	onNodeJoined    func(*NodeStatus)      // Callback for new nodes
	onNodeLeft      func(string)           // Callback for departed nodes
	onZoneChange    func(string, []byte)   // Callback for zone changes
}

// NodeStatus represents the status of a node in the mesh
type NodeStatus struct {
	Identity     string    // Node identity
	Address      string    // Network address
	Status       string    // "joined", "left", "failed", "recovered"
	LastSeen     time.Time // Last gossip update
	ZoneInfo     []byte    // Zone assignment information
	Version      uint64    // Version counter for updates
	IsStale      bool      // Whether this information might be outdated
}

// NewGossipManager creates a new gossip manager
func NewGossipManager(nodeIdentity string) *GossipManager {
	return &GossipManager{
		nodeIdentity:   nodeIdentity,
		allNodes:       make(map[string]*NodeStatus),
		gossipPeers:    make([]string, 0),
		gossipInterval: time.Second * 2,  // Gossip every 2 seconds
		maxGossipPeers: 3,               // Gossip with up to 3 peers per round
		stopChannel:    make(chan struct{}),
		pendingUpdates: make([]*protocol.NodeUpdate, 0),
	}
}

// Start begins the gossip process
func (gm *GossipManager) Start() {
	gm.mu.Lock()
	if gm.isRunning {
		gm.mu.Unlock()
		return
	}
	gm.isRunning = true
	gm.mu.Unlock()

	go gm.gossipLoop()
}

// Stop stops the gossip process
func (gm *GossipManager) Stop() {
	gm.mu.Lock()
	if !gm.isRunning {
		gm.mu.Unlock()
		return
	}
	gm.isRunning = false
	gm.mu.Unlock()

	close(gm.stopChannel)
}

// AddNode adds a node to the mesh knowledge
func (gm *GossipManager) AddNode(identity, address string, zoneInfo []byte) {
	gm.mu.Lock()
	defer gm.mu.Unlock()

	now := time.Now()
	isNewNode := false

	node, exists := gm.allNodes[identity]
	if !exists {
		node = &NodeStatus{
			Identity: identity,
			Address:  address,
			Status:   "joined",
			LastSeen: now,
			ZoneInfo: zoneInfo,
			Version:  1,
			IsStale:  false,
		}
		gm.allNodes[identity] = node
		isNewNode = true
	} else {
		// Update existing node
		node.Address = address
		node.Status = "joined"
		node.LastSeen = now
		node.ZoneInfo = zoneInfo
		node.Version++
		node.IsStale = false
	}

	// Add to pending updates for gossip
	update := &protocol.NodeUpdate{
		Identity: identity,
		Status:   "joined",
		Address:  address,
		ZoneInfo: zoneInfo,
	}
	gm.pendingUpdates = append(gm.pendingUpdates, update)

	// Update gossip peers list
	gm.updateGossipPeers()

	// Notify callback
	if isNewNode && gm.onNodeJoined != nil {
		go gm.onNodeJoined(node)
	}
}

// RemoveNode marks a node as left
func (gm *GossipManager) RemoveNode(identity string) {
	gm.mu.Lock()
	defer gm.mu.Unlock()

	node, exists := gm.allNodes[identity]
	if !exists {
		return
	}

	node.Status = "left"
	node.LastSeen = time.Now()
	node.Version++

	// Add to pending updates
	update := &protocol.NodeUpdate{
		Identity: identity,
		Status:   "left",
		Address:  node.Address,
		ZoneInfo: node.ZoneInfo,
	}
	gm.pendingUpdates = append(gm.pendingUpdates, update)

	// Update gossip peers list
	gm.updateGossipPeers()

	// Notify callback
	if gm.onNodeLeft != nil {
		go gm.onNodeLeft(identity)
	}
}

// MarkNodeFailed marks a node as failed
func (gm *GossipManager) MarkNodeFailed(identity string) {
	gm.mu.Lock()
	defer gm.mu.Unlock()

	node, exists := gm.allNodes[identity]
	if !exists {
		return
	}

	if node.Status != "failed" {
		node.Status = "failed"
		node.LastSeen = time.Now()
		node.Version++

		// Add to pending updates
		update := &protocol.NodeUpdate{
			Identity: identity,
			Status:   "failed",
			Address:  node.Address,
			ZoneInfo: node.ZoneInfo,
		}
		gm.pendingUpdates = append(gm.pendingUpdates, update)
	}
}

// UpdateZoneInfo updates zone information for a node
func (gm *GossipManager) UpdateZoneInfo(identity string, zoneInfo []byte) {
	gm.mu.Lock()
	defer gm.mu.Unlock()

	node, exists := gm.allNodes[identity]
	if !exists {
		return
	}

	node.ZoneInfo = zoneInfo
	node.LastSeen = time.Now()
	node.Version++

	// Add to pending updates
	update := &protocol.NodeUpdate{
		Identity: identity,
		Status:   node.Status,
		Address:  node.Address,
		ZoneInfo: zoneInfo,
	}
	gm.pendingUpdates = append(gm.pendingUpdates, update)

	// Notify callback
	if gm.onZoneChange != nil {
		go gm.onZoneChange(identity, zoneInfo)
	}
}

// GetAllNodes returns a copy of all known nodes
func (gm *GossipManager) GetAllNodes() map[string]*NodeStatus {
	gm.mu.RLock()
	defer gm.mu.RUnlock()

	result := make(map[string]*NodeStatus)
	for identity, node := range gm.allNodes {
		result[identity] = &NodeStatus{
			Identity: node.Identity,
			Address:  node.Address,
			Status:   node.Status,
			LastSeen: node.LastSeen,
			ZoneInfo: node.ZoneInfo,
			Version:  node.Version,
			IsStale:  node.IsStale,
		}
	}

	return result
}

// GetActiveNodes returns nodes that are currently active (not left or failed)
func (gm *GossipManager) GetActiveNodes() map[string]*NodeStatus {
	gm.mu.RLock()
	defer gm.mu.RUnlock()

	result := make(map[string]*NodeStatus)
	for identity, node := range gm.allNodes {
		if node.Status == "joined" || node.Status == "recovered" {
			result[identity] = &NodeStatus{
				Identity: node.Identity,
				Address:  node.Address,
				Status:   node.Status,
				LastSeen: node.LastSeen,
				ZoneInfo: node.ZoneInfo,
				Version:  node.Version,
				IsStale:  node.IsStale,
			}
		}
	}

	return result
}

// SetCallbacks sets the event callbacks
func (gm *GossipManager) SetCallbacks(
	onNodeJoined func(*NodeStatus),
	onNodeLeft func(string),
	onZoneChange func(string, []byte),
) {
	gm.mu.Lock()
	defer gm.mu.Unlock()

	gm.onNodeJoined = onNodeJoined
	gm.onNodeLeft = onNodeLeft
	gm.onZoneChange = onZoneChange
}

// gossipLoop is the main gossip processing loop
func (gm *GossipManager) gossipLoop() {
	ticker := time.NewTicker(gm.gossipInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			gm.processGossipRound()
		case <-gm.stopChannel:
			return
		}
	}
}

// processGossipRound handles one round of gossip
func (gm *GossipManager) processGossipRound() {
	// Get gossip targets
	targets := gm.selectGossipTargets()
	if len(targets) == 0 {
		return
	}

	// Prepare gossip message
	gossipMsg := gm.prepareGossipMessage()
	if gossipMsg == nil {
		return
	}

	// Send gossip to selected targets
	for _, target := range targets {
		go gm.sendGossip(target, gossipMsg)
	}

	// Mark updates as sent
	gm.mu.Lock()
	gm.pendingUpdates = gm.pendingUpdates[:0] // Clear pending updates
	gm.mu.Unlock()

	// Clean up stale nodes
	gm.cleanupStaleNodes()
}

// selectGossipTargets chooses nodes to gossip with
func (gm *GossipManager) selectGossipTargets() []string {
	gm.mu.RLock()
	defer gm.mu.RUnlock()

	// Select random subset from gossip peers
	available := make([]string, 0)
	for _, peer := range gm.gossipPeers {
		if node, exists := gm.allNodes[peer]; exists && node.Status == "joined" {
			available = append(available, peer)
		}
	}

	if len(available) == 0 {
		return nil
	}

	// Select up to maxGossipPeers random targets
	count := gm.maxGossipPeers
	if count > len(available) {
		count = len(available)
	}

	targets := make([]string, count)
	for i := 0; i < count; i++ {
		idx := rand.Intn(len(available))
		targets[i] = available[idx]

		// Remove selected target to avoid duplicates
		available[idx] = available[len(available)-1]
		available = available[:len(available)-1]
	}

	return targets
}

// prepareGossipMessage creates a gossip message with pending updates
func (gm *GossipManager) prepareGossipMessage() *protocol.Message {
	gm.mu.RLock()
	defer gm.mu.RUnlock()

	if len(gm.pendingUpdates) == 0 {
		return nil
	}

	// Create gossip message
	gossipData := &protocol.GossipMessage{
		FromNode:    gm.nodeIdentity,
		Timestamp:   time.Now().Unix(),
		NodeUpdates: gm.pendingUpdates,
	}

	// Encode payload
	payload := gm.encodeGossipPayload(gossipData)

	return &protocol.Message{
		Version:  protocol.Version,
		Type:     protocol.GOSSIP,
		Sequence: uint32(time.Now().Unix()),
		Payload:  payload,
	}
}

// sendGossip sends a gossip message to a target node
func (gm *GossipManager) sendGossip(targetIdentity string, gossipMsg *protocol.Message) {
	gm.mu.RLock()
	target, exists := gm.allNodes[targetIdentity]
	if !exists {
		gm.mu.RUnlock()
		return
	}
	targetAddr := target.Address
	gm.mu.RUnlock()

	// Connect to target
	conn, err := net.DialTimeout("tcp", targetAddr, time.Second*2)
	if err != nil {
		gm.handleGossipError(targetIdentity, err)
		return
	}
	defer conn.Close()

	// Send gossip message
	data, err := protocol.EncodeMessage(gossipMsg)
	if err != nil {
		gm.handleGossipError(targetIdentity, fmt.Errorf("encode gossip: %w", err))
		return
	}

	conn.SetWriteDeadline(time.Now().Add(time.Second * 3))
	_, err = conn.Write(data)
	if err != nil {
		gm.handleGossipError(targetIdentity, fmt.Errorf("send gossip: %w", err))
		return
	}

	// Read acknowledgment
	conn.SetReadDeadline(time.Now().Add(time.Second * 3))
	response, err := gm.readGossipResponse(conn)
	if err != nil {
		gm.handleGossipError(targetIdentity, fmt.Errorf("read ack: %w", err))
		return
	}

	// Process response if it contains gossip data back
	if response.Type == protocol.GOSSIP {
		gm.processIncomingGossip(response)
	}
}

// handleGossipError handles errors during gossip communication
func (gm *GossipManager) handleGossipError(targetIdentity string, err error) {
	// For now, just mark the node as potentially failed
	// In a production system, we'd track consecutive failures
	gm.mu.Lock()
	if node, exists := gm.allNodes[targetIdentity]; exists {
		node.IsStale = true
	}
	gm.mu.Unlock()
}

// readGossipResponse reads a gossip response message
func (gm *GossipManager) readGossipResponse(conn net.Conn) (*protocol.Message, error) {
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

// processIncomingGossip processes gossip received from other nodes
func (gm *GossipManager) processIncomingGossip(msg *protocol.Message) {
	gossipData := gm.decodeGossipPayload(msg.Payload)
	if gossipData == nil {
		return
	}

	gm.mu.Lock()
	defer gm.mu.Unlock()

	// Process each node update
	for _, update := range gossipData.NodeUpdates {
		existingNode, exists := gm.allNodes[update.Identity]

		if !exists {
			// New node
			node := &NodeStatus{
				Identity: update.Identity,
				Address:  update.Address,
				Status:   update.Status,
				LastSeen: time.Now(),
				ZoneInfo: update.ZoneInfo,
				Version:  1,
				IsStale:  false,
			}
			gm.allNodes[update.Identity] = node

			// Notify if joined
			if update.Status == "joined" && gm.onNodeJoined != nil {
				go gm.onNodeJoined(node)
			}
		} else {
			// Update existing node if this is newer information
			if time.Since(existingNode.LastSeen) > time.Minute {
				existingNode.Address = update.Address
				existingNode.Status = update.Status
				existingNode.LastSeen = time.Now()
				existingNode.ZoneInfo = update.ZoneInfo
				existingNode.Version++
				existingNode.IsStale = false

				// Notify callbacks
				if update.Status == "left" && gm.onNodeLeft != nil {
					go gm.onNodeLeft(update.Identity)
				}
				if len(update.ZoneInfo) > 0 && gm.onZoneChange != nil {
					go gm.onZoneChange(update.Identity, update.ZoneInfo)
				}
			}
		}
	}

	// Update gossip peers
	gm.updateGossipPeers()
}

// updateGossipPeers updates the list of peers to gossip with
func (gm *GossipManager) updateGossipPeers() {
	// Select random subset of active nodes for gossiping
	active := make([]string, 0)
	for identity, node := range gm.allNodes {
		if identity != gm.nodeIdentity && node.Status == "joined" {
			active = append(active, identity)
		}
	}

	// Select up to maxGossipPeers * 2 for the gossip pool
	maxPeers := gm.maxGossipPeers * 2
	if len(active) <= maxPeers {
		gm.gossipPeers = active
	} else {
		// Random selection
		gm.gossipPeers = make([]string, maxPeers)
		for i := 0; i < maxPeers; i++ {
			idx := rand.Intn(len(active))
			gm.gossipPeers[i] = active[idx]

			// Remove to avoid duplicates
			active[idx] = active[len(active)-1]
			active = active[:len(active)-1]
		}
	}
}

// cleanupStaleNodes removes nodes that haven't been seen for a long time
func (gm *GossipManager) cleanupStaleNodes() {
	gm.mu.Lock()
	defer gm.mu.Unlock()

	staleThreshold := time.Minute * 10 // 10 minutes
	now := time.Now()

	for identity, node := range gm.allNodes {
		if identity == gm.nodeIdentity {
			continue // Never remove self
		}

		if now.Sub(node.LastSeen) > staleThreshold && node.Status == "left" {
			delete(gm.allNodes, identity)
		}
	}
}

// encodeGossipPayload encodes gossip data into bytes
func (gm *GossipManager) encodeGossipPayload(gossipData *protocol.GossipMessage) []byte {
	// Simple encoding for gossip payload
	// In production, this would use more efficient serialization

	// Calculate total size needed
	size := 8 + 1 + len(gossipData.FromNode) + 4 // timestamp + fromnode length + fromnode + update count
	for _, update := range gossipData.NodeUpdates {
		size += 1 + len(update.Identity) + 1 + len(update.Status) +
		        1 + len(update.Address) + 4 + len(update.ZoneInfo)
	}

	payload := make([]byte, size)
	offset := 0

	// Encode timestamp
	ts := gossipData.Timestamp
	for i := 0; i < 8; i++ {
		payload[offset] = byte(ts >> (56 - i*8))
		offset++
	}

	// Encode from node
	payload[offset] = byte(len(gossipData.FromNode))
	offset++
	copy(payload[offset:], []byte(gossipData.FromNode))
	offset += len(gossipData.FromNode)

	// Encode update count
	updateCount := len(gossipData.NodeUpdates)
	for i := 0; i < 4; i++ {
		payload[offset] = byte(updateCount >> (24 - i*8))
		offset++
	}

	// Encode updates
	for _, update := range gossipData.NodeUpdates {
		// Identity
		payload[offset] = byte(len(update.Identity))
		offset++
		copy(payload[offset:], []byte(update.Identity))
		offset += len(update.Identity)

		// Status
		payload[offset] = byte(len(update.Status))
		offset++
		copy(payload[offset:], []byte(update.Status))
		offset += len(update.Status)

		// Address
		payload[offset] = byte(len(update.Address))
		offset++
		copy(payload[offset:], []byte(update.Address))
		offset += len(update.Address)

		// Zone info length and data
		zoneLen := len(update.ZoneInfo)
		for i := 0; i < 4; i++ {
			payload[offset] = byte(zoneLen >> (24 - i*8))
			offset++
		}
		copy(payload[offset:], update.ZoneInfo)
		offset += len(update.ZoneInfo)
	}

	return payload
}

// decodeGossipPayload decodes gossip data from bytes
func (gm *GossipManager) decodeGossipPayload(payload []byte) *protocol.GossipMessage {
	if len(payload) < 13 { // Minimum size
		return nil
	}

	offset := 0

	// Decode timestamp
	var timestamp int64
	for i := 0; i < 8; i++ {
		timestamp = (timestamp << 8) | int64(payload[offset])
		offset++
	}

	// Decode from node
	fromNodeLen := int(payload[offset])
	offset++
	if offset+fromNodeLen > len(payload) {
		return nil
	}
	fromNode := string(payload[offset : offset+fromNodeLen])
	offset += fromNodeLen

	// Decode update count
	if offset+4 > len(payload) {
		return nil
	}
	var updateCount int
	for i := 0; i < 4; i++ {
		updateCount = (updateCount << 8) | int(payload[offset])
		offset++
	}

	// Decode updates
	updates := make([]*protocol.NodeUpdate, 0, updateCount)
	for i := 0; i < updateCount; i++ {
		if offset >= len(payload) {
			break
		}

		// Identity
		identityLen := int(payload[offset])
		offset++
		if offset+identityLen > len(payload) {
			break
		}
		identity := string(payload[offset : offset+identityLen])
		offset += identityLen

		// Status
		if offset >= len(payload) {
			break
		}
		statusLen := int(payload[offset])
		offset++
		if offset+statusLen > len(payload) {
			break
		}
		status := string(payload[offset : offset+statusLen])
		offset += statusLen

		// Address
		if offset >= len(payload) {
			break
		}
		addressLen := int(payload[offset])
		offset++
		if offset+addressLen > len(payload) {
			break
		}
		address := string(payload[offset : offset+addressLen])
		offset += addressLen

		// Zone info
		if offset+4 > len(payload) {
			break
		}
		var zoneLen int
		for j := 0; j < 4; j++ {
			zoneLen = (zoneLen << 8) | int(payload[offset])
			offset++
		}
		if offset+zoneLen > len(payload) {
			break
		}
		zoneInfo := make([]byte, zoneLen)
		copy(zoneInfo, payload[offset:offset+zoneLen])
		offset += zoneLen

		updates = append(updates, &protocol.NodeUpdate{
			Identity: identity,
			Status:   status,
			Address:  address,
			ZoneInfo: zoneInfo,
		})
	}

	return &protocol.GossipMessage{
		FromNode:    fromNode,
		Timestamp:   timestamp,
		NodeUpdates: updates,
	}
}
