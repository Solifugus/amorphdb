package mesh

import "fmt"
import (
	"testing"
	"time"

	"github.com/solifugus/amorphdb/internal/protocol"
)

func TestNewGossipManager(t *testing.T) {
	gm := NewGossipManager("test-node-ka-ve-lo")

	if gm.nodeIdentity != "test-node-ka-ve-lo" {
		t.Errorf("Expected nodeIdentity 'test-node-ka-ve-lo', got '%s'", gm.nodeIdentity)
	}

	if len(gm.allNodes) != 0 {
		t.Errorf("Expected 0 nodes initially, got %d", len(gm.allNodes))
	}

	if gm.gossipInterval != time.Second*2 {
		t.Errorf("Expected gossip interval 2s, got %v", gm.gossipInterval)
	}

	if gm.maxGossipPeers != 3 {
		t.Errorf("Expected max gossip peers 3, got %d", gm.maxGossipPeers)
	}

	if gm.isRunning {
		t.Error("Expected gossip manager to not be running initially")
	}
}

func TestGossipManager_AddNode(t *testing.T) {
	gm := NewGossipManager("test-node")

	zoneInfo := []byte("test zone info")
	gm.AddNode("peer1", "192.168.1.100:5000", zoneInfo)

	// Verify node was added
	if len(gm.allNodes) != 1 {
		t.Errorf("Expected 1 node, got %d", len(gm.allNodes))
	}

	node, exists := gm.allNodes["peer1"]
	if !exists {
		t.Error("Node 'peer1' not found")
	}

	if node.Address != "192.168.1.100:5000" {
		t.Errorf("Expected address '192.168.1.100:5000', got '%s'", node.Address)
	}

	if node.Status != "joined" {
		t.Errorf("Expected status 'joined', got '%s'", node.Status)
	}

	if string(node.ZoneInfo) != string(zoneInfo) {
		t.Errorf("Expected zone info '%s', got '%s'", string(zoneInfo), string(node.ZoneInfo))
	}

	if node.Version != 1 {
		t.Errorf("Expected version 1, got %d", node.Version)
	}

	if node.IsStale {
		t.Error("Expected node to not be stale initially")
	}

	// Verify pending update was created
	if len(gm.pendingUpdates) != 1 {
		t.Errorf("Expected 1 pending update, got %d", len(gm.pendingUpdates))
	}

	update := gm.pendingUpdates[0]
	if update.Identity != "peer1" {
		t.Errorf("Expected update identity 'peer1', got '%s'", update.Identity)
	}

	if update.Status != "joined" {
		t.Errorf("Expected update status 'joined', got '%s'", update.Status)
	}
}

func TestGossipManager_AddNode_Update(t *testing.T) {
	gm := NewGossipManager("test-node")

	// Add node first time
	zoneInfo1 := []byte("zone info 1")
	gm.AddNode("peer1", "192.168.1.100:5000", zoneInfo1)

	// Verify initial state
	node := gm.allNodes["peer1"]
	initialVersion := node.Version
	if initialVersion != 1 {
		t.Errorf("Expected initial version 1, got %d", initialVersion)
	}

	// Add same node again with different info
	time.Sleep(time.Millisecond) // Ensure time difference
	zoneInfo2 := []byte("zone info 2")
	gm.AddNode("peer1", "192.168.1.101:5000", zoneInfo2)

	// Verify node was updated, not duplicated
	if len(gm.allNodes) != 1 {
		t.Errorf("Expected 1 node after update, got %d", len(gm.allNodes))
	}

	updatedNode := gm.allNodes["peer1"]
	if updatedNode.Address != "192.168.1.101:5000" {
		t.Errorf("Expected updated address '192.168.1.101:5000', got '%s'", updatedNode.Address)
	}

	if string(updatedNode.ZoneInfo) != string(zoneInfo2) {
		t.Errorf("Expected updated zone info '%s', got '%s'", string(zoneInfo2), string(updatedNode.ZoneInfo))
	}

	if updatedNode.Version != initialVersion+1 {
		t.Errorf("Expected version %d, got %d", initialVersion+1, updatedNode.Version)
	}

	// Should have 2 pending updates now
	if len(gm.pendingUpdates) != 2 {
		t.Errorf("Expected 2 pending updates, got %d", len(gm.pendingUpdates))
	}
}

func TestGossipManager_RemoveNode(t *testing.T) {
	gm := NewGossipManager("test-node")

	// Add a node first
	gm.AddNode("peer1", "192.168.1.100:5000", []byte("zone info"))

	// Verify it was added
	if len(gm.allNodes) != 1 {
		t.Errorf("Expected 1 node after add, got %d", len(gm.allNodes))
	}

	// Remove the node
	gm.RemoveNode("peer1")

	// Node should still exist but marked as left
	if len(gm.allNodes) != 1 {
		t.Errorf("Expected 1 node after remove (not deleted), got %d", len(gm.allNodes))
	}

	node := gm.allNodes["peer1"]
	if node.Status != "left" {
		t.Errorf("Expected status 'left', got '%s'", node.Status)
	}

	// Version should have incremented
	if node.Version != 2 {
		t.Errorf("Expected version 2 after remove, got %d", node.Version)
	}

	// Should have 2 pending updates (add + remove)
	if len(gm.pendingUpdates) != 2 {
		t.Errorf("Expected 2 pending updates, got %d", len(gm.pendingUpdates))
	}

	// Test removing non-existent node (should not panic)
	gm.RemoveNode("nonexistent")
}

func TestGossipManager_MarkNodeFailed(t *testing.T) {
	gm := NewGossipManager("test-node")

	// Add a node first
	gm.AddNode("peer1", "192.168.1.100:5000", []byte("zone info"))

	// Mark as failed
	gm.MarkNodeFailed("peer1")

	node := gm.allNodes["peer1"]
	if node.Status != "failed" {
		t.Errorf("Expected status 'failed', got '%s'", node.Status)
	}

	// Version should have incremented
	if node.Version != 2 {
		t.Errorf("Expected version 2 after failure, got %d", node.Version)
	}

	// Should have 2 pending updates (add + failed)
	if len(gm.pendingUpdates) != 2 {
		t.Errorf("Expected 2 pending updates, got %d", len(gm.pendingUpdates))
	}

	// Mark as failed again - should not create duplicate update
	originalUpdateCount := len(gm.pendingUpdates)
	gm.MarkNodeFailed("peer1")

	if len(gm.pendingUpdates) != originalUpdateCount {
		t.Errorf("Expected no additional updates for repeated failure, got %d updates", len(gm.pendingUpdates))
	}

	// Test marking non-existent node as failed (should not panic)
	gm.MarkNodeFailed("nonexistent")
}

func TestGossipManager_UpdateZoneInfo(t *testing.T) {
	gm := NewGossipManager("test-node")

	// Add a node first
	originalZoneInfo := []byte("original zone info")
	gm.AddNode("peer1", "192.168.1.100:5000", originalZoneInfo)

	// Update zone info
	newZoneInfo := []byte("new zone info")
	gm.UpdateZoneInfo("peer1", newZoneInfo)

	node := gm.allNodes["peer1"]
	if string(node.ZoneInfo) != string(newZoneInfo) {
		t.Errorf("Expected zone info '%s', got '%s'", string(newZoneInfo), string(node.ZoneInfo))
	}

	// Version should have incremented
	if node.Version != 2 {
		t.Errorf("Expected version 2 after zone update, got %d", node.Version)
	}

	// Should have 2 pending updates (add + zone update)
	if len(gm.pendingUpdates) != 2 {
		t.Errorf("Expected 2 pending updates, got %d", len(gm.pendingUpdates))
	}

	// Test updating non-existent node (should not panic)
	gm.UpdateZoneInfo("nonexistent", []byte("test"))
}

func TestGossipManager_GetAllNodes(t *testing.T) {
	gm := NewGossipManager("test-node")

	// Initially empty
	allNodes := gm.GetAllNodes()
	if len(allNodes) != 0 {
		t.Errorf("Expected 0 nodes initially, got %d", len(allNodes))
	}

	// Add some nodes
	gm.AddNode("peer1", "192.168.1.100:5000", []byte("zone1"))
	gm.AddNode("peer2", "192.168.1.101:5000", []byte("zone2"))
	gm.RemoveNode("peer1") // Mark peer1 as left

	allNodes = gm.GetAllNodes()
	if len(allNodes) != 2 {
		t.Errorf("Expected 2 nodes, got %d", len(allNodes))
	}

	// Verify peer1 is marked as left
	peer1, exists := allNodes["peer1"]
	if !exists {
		t.Error("Expected peer1 to exist in all nodes")
	} else if peer1.Status != "left" {
		t.Errorf("Expected peer1 status 'left', got '%s'", peer1.Status)
	}

	// Verify peer2 is marked as joined
	peer2, exists := allNodes["peer2"]
	if !exists {
		t.Error("Expected peer2 to exist in all nodes")
	} else if peer2.Status != "joined" {
		t.Errorf("Expected peer2 status 'joined', got '%s'", peer2.Status)
	}

	// Verify returned map is a copy (modifications don't affect original)
	peer2.Status = "modified"
	originalPeer2 := gm.allNodes["peer2"]
	if originalPeer2.Status == "modified" {
		t.Error("Expected returned nodes map to be a copy")
	}
}

func TestGossipManager_GetActiveNodes(t *testing.T) {
	gm := NewGossipManager("test-node")

	// Add nodes with different statuses
	gm.AddNode("peer1", "192.168.1.100:5000", []byte("zone1"))
	gm.AddNode("peer2", "192.168.1.101:5000", []byte("zone2"))
	gm.AddNode("peer3", "192.168.1.102:5000", []byte("zone3"))

	gm.RemoveNode("peer1")          // Mark as left
	gm.MarkNodeFailed("peer2")      // Mark as failed

	activeNodes := gm.GetActiveNodes()
	if len(activeNodes) != 1 {
		t.Errorf("Expected 1 active node, got %d", len(activeNodes))
	}

	// Only peer3 should be active
	peer3, exists := activeNodes["peer3"]
	if !exists {
		t.Error("Expected peer3 to be active")
	} else if peer3.Status != "joined" {
		t.Errorf("Expected peer3 status 'joined', got '%s'", peer3.Status)
	}

	// peer1 and peer2 should not be in active nodes
	if _, exists := activeNodes["peer1"]; exists {
		t.Error("Expected peer1 to not be active (left)")
	}

	if _, exists := activeNodes["peer2"]; exists {
		t.Error("Expected peer2 to not be active (failed)")
	}
}

func TestGossipManager_StartStop(t *testing.T) {
	gm := NewGossipManager("test-node")

	// Should not be running initially
	if gm.isRunning {
		t.Error("Expected gossip manager to not be running initially")
	}

	// Start the manager
	gm.Start()

	// Should be running now
	if !gm.isRunning {
		t.Error("Expected gossip manager to be running after Start()")
	}

	// Starting again should be safe (no-op)
	gm.Start()

	if !gm.isRunning {
		t.Error("Expected gossip manager to still be running after second Start()")
	}

	// Stop the manager
	gm.Stop()

	// Should not be running now
	if gm.isRunning {
		t.Error("Expected gossip manager to not be running after Stop()")
	}

	// Stopping again should be safe (no-op)
	gm.Stop()

	if gm.isRunning {
		t.Error("Expected gossip manager to still be stopped after second Stop()")
	}
}

func TestGossipManager_SetCallbacks(t *testing.T) {
	gm := NewGossipManager("test-node")

	joinedNodes := make([]*NodeStatus, 0)
	leftNodes := make([]string, 0)
	zoneChanges := make(map[string][]byte)

	joinCallback := func(node *NodeStatus) {
		joinedNodes = append(joinedNodes, node)
	}

	leftCallback := func(identity string) {
		leftNodes = append(leftNodes, identity)
	}

	zoneCallback := func(identity string, zoneInfo []byte) {
		zoneChanges[identity] = zoneInfo
	}

	gm.SetCallbacks(joinCallback, leftCallback, zoneCallback)

	// Verify callbacks were set
	gm.mu.RLock()
	hasJoinCallback := gm.onNodeJoined != nil
	hasLeftCallback := gm.onNodeLeft != nil
	hasZoneCallback := gm.onZoneChange != nil
	gm.mu.RUnlock()

	if !hasJoinCallback {
		t.Error("Expected join callback to be set")
	}

	if !hasLeftCallback {
		t.Error("Expected left callback to be set")
	}

	if !hasZoneCallback {
		t.Error("Expected zone callback to be set")
	}
}

func TestGossipManager_EncodeDecodeGossipPayload(t *testing.T) {
	gm := NewGossipManager("test-node")

	// Create test gossip message
	now := time.Now().Unix()
	updates := []*protocol.NodeUpdate{
		{
			Identity: "peer1",
			Status:   "joined",
			Address:  "192.168.1.100:5000",
			ZoneInfo: []byte("zone1 info"),
		},
		{
			Identity: "peer2",
			Status:   "left",
			Address:  "192.168.1.101:5000",
			ZoneInfo: []byte("zone2 info"),
		},
	}

	originalGossip := &protocol.GossipMessage{
		FromNode:    "test-node",
		Timestamp:   now,
		NodeUpdates: updates,
	}

	// Encode
	payload := gm.encodeGossipPayload(originalGossip)
	if len(payload) == 0 {
		t.Error("Expected non-empty payload")
	}

	// Decode
	decodedGossip := gm.decodeGossipPayload(payload)
	if decodedGossip == nil {
		t.Error("Expected non-nil decoded gossip")
		return
	}

	// Verify decoded data matches original
	if decodedGossip.FromNode != originalGossip.FromNode {
		t.Errorf("Expected fromNode '%s', got '%s'", originalGossip.FromNode, decodedGossip.FromNode)
	}

	if decodedGossip.Timestamp != originalGossip.Timestamp {
		t.Errorf("Expected timestamp %d, got %d", originalGossip.Timestamp, decodedGossip.Timestamp)
	}

	if len(decodedGossip.NodeUpdates) != len(originalGossip.NodeUpdates) {
		t.Errorf("Expected %d updates, got %d", len(originalGossip.NodeUpdates), len(decodedGossip.NodeUpdates))
		return
	}

	// Verify each update
	for i, original := range originalGossip.NodeUpdates {
		decoded := decodedGossip.NodeUpdates[i]

		if decoded.Identity != original.Identity {
			t.Errorf("Update %d: expected identity '%s', got '%s'", i, original.Identity, decoded.Identity)
		}

		if decoded.Status != original.Status {
			t.Errorf("Update %d: expected status '%s', got '%s'", i, original.Status, decoded.Status)
		}

		if decoded.Address != original.Address {
			t.Errorf("Update %d: expected address '%s', got '%s'", i, original.Address, decoded.Address)
		}

		if string(decoded.ZoneInfo) != string(original.ZoneInfo) {
			t.Errorf("Update %d: expected zone info '%s', got '%s'", i, string(original.ZoneInfo), string(decoded.ZoneInfo))
		}
	}
}

func TestGossipManager_EncodeDecodeGossipPayload_Empty(t *testing.T) {
	gm := NewGossipManager("test-node")

	// Test with empty updates
	emptyGossip := &protocol.GossipMessage{
		FromNode:    "test-node",
		Timestamp:   time.Now().Unix(),
		NodeUpdates: []*protocol.NodeUpdate{},
	}

	payload := gm.encodeGossipPayload(emptyGossip)
	decoded := gm.decodeGossipPayload(payload)

	if decoded == nil {
		t.Error("Expected non-nil decoded gossip for empty updates")
		return
	}

	if len(decoded.NodeUpdates) != 0 {
		t.Errorf("Expected 0 updates, got %d", len(decoded.NodeUpdates))
	}
}

func TestGossipManager_EncodeDecodeGossipPayload_Invalid(t *testing.T) {
	gm := NewGossipManager("test-node")

	// Test with invalid payload
	invalidPayloads := [][]byte{
		{}, // Empty
		{1, 2, 3}, // Too short
		{0, 0, 0, 0, 0, 0, 0, 0, 5, 't', 'e', 's', 't'}, // Missing update count
	}

	for i, payload := range invalidPayloads {
		decoded := gm.decodeGossipPayload(payload)
		if decoded != nil {
			t.Errorf("Invalid payload %d: expected nil decoded gossip, got non-nil", i)
		}
	}
}

func TestGossipManager_UpdateGossipPeers(t *testing.T) {
	gm := NewGossipManager("test-node-main")

	// Add several nodes
	for i := 0; i < 10; i++ {
		gm.AddNode(fmt.Sprintf("peer%d", i), fmt.Sprintf("192.168.1.%d:5000", 100+i), []byte("zone"))
	}

	// Mark some as left to test filtering
	gm.RemoveNode("peer1")
	gm.RemoveNode("peer3")

	// Update gossip peers (this happens automatically in AddNode, but let's test explicitly)
	gm.updateGossipPeers()

	// Verify gossip peers list
	expectedMaxPeers := gm.maxGossipPeers * 2 // 6
	if len(gm.gossipPeers) > expectedMaxPeers {
		t.Errorf("Expected max %d gossip peers, got %d", expectedMaxPeers, len(gm.gossipPeers))
	}

	// Verify only active nodes are in gossip peers
	for _, peerID := range gm.gossipPeers {
		node := gm.allNodes[peerID]
		if node.Status != "joined" {
			t.Errorf("Expected only joined nodes in gossip peers, found '%s' with status '%s'", peerID, node.Status)
		}

		if peerID == gm.nodeIdentity {
			t.Errorf("Node should not include itself in gossip peers: %s", peerID)
		}
	}
}

func TestGossipManager_CleanupStaleNodes(t *testing.T) {
	gm := NewGossipManager("test-node")

	// Add nodes and mark them with different timestamps
	now := time.Now()

	// Recent left node (should not be cleaned up)
	gm.AddNode("peer1", "192.168.1.100:5000", []byte("zone1"))
	gm.RemoveNode("peer1")
	gm.allNodes["peer1"].LastSeen = now.Add(-time.Minute * 5)

	// Old left node (should be cleaned up)
	gm.AddNode("peer2", "192.168.1.101:5000", []byte("zone2"))
	gm.RemoveNode("peer2")
	gm.allNodes["peer2"].LastSeen = now.Add(-time.Minute * 15)

	// Active node (should not be cleaned up)
	gm.AddNode("peer3", "192.168.1.102:5000", []byte("zone3"))
	gm.allNodes["peer3"].LastSeen = now.Add(-time.Minute * 15) // Old but still active

	// Initial count
	initialCount := len(gm.allNodes)
	if initialCount != 3 {
		t.Errorf("Expected 3 nodes before cleanup, got %d", initialCount)
	}

	// Run cleanup
	gm.cleanupStaleNodes()

	// Verify cleanup results
	finalCount := len(gm.allNodes)
	if finalCount != 2 {
		t.Errorf("Expected 2 nodes after cleanup, got %d", finalCount)
	}

	// peer2 should be removed (old and left)
	if _, exists := gm.allNodes["peer2"]; exists {
		t.Error("Expected peer2 to be removed during cleanup")
	}

	// peer1 should remain (recent left)
	if _, exists := gm.allNodes["peer1"]; !exists {
		t.Error("Expected peer1 to remain (recent left)")
	}

	// peer3 should remain (active)
	if _, exists := gm.allNodes["peer3"]; !exists {
		t.Error("Expected peer3 to remain (active)")
	}
}
