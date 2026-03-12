package zone

import (
	"fmt"
	"testing"
	"time"
)

func TestNewHashRing(t *testing.T) {
	ring := NewHashRing(3, 100)

	if ring.replicas != 3 {
		t.Errorf("Expected replicas=3, got %d", ring.replicas)
	}

	if ring.vnodeCount != 100 {
		t.Errorf("Expected vnodeCount=100, got %d", ring.vnodeCount)
	}

	if len(ring.nodes) != 0 {
		t.Errorf("Expected empty nodes map, got %d nodes", len(ring.nodes))
	}

	if len(ring.virtualNodes) != 0 {
		t.Errorf("Expected empty virtualNodes map, got %d virtual nodes", len(ring.virtualNodes))
	}
}

func TestHashRing_AddNode(t *testing.T) {
	ring := NewHashRing(3, 10)

	err := ring.AddNode("ka-ve-lo", "192.168.1.100:5000")
	if err != nil {
		t.Fatalf("AddNode failed: %v", err)
	}

	// Verify node was added
	if len(ring.nodes) != 1 {
		t.Errorf("Expected 1 node, got %d", len(ring.nodes))
	}

	node, exists := ring.nodes["ka-ve-lo"]
	if !exists {
		t.Error("Node 'ka-ve-lo' not found")
	}

	if node.Address != "192.168.1.100:5000" {
		t.Errorf("Expected address '192.168.1.100:5000', got '%s'", node.Address)
	}

	if !node.IsOnline {
		t.Error("Expected node to be online")
	}

	// Verify virtual nodes were created
	expectedVNodes := 10
	if len(ring.virtualNodes) != expectedVNodes {
		t.Errorf("Expected %d virtual nodes, got %d", expectedVNodes, len(ring.virtualNodes))
	}

	if len(ring.sortedHashes) != expectedVNodes {
		t.Errorf("Expected %d sorted hashes, got %d", expectedVNodes, len(ring.sortedHashes))
	}

	// Verify hashes are sorted
	for i := 1; i < len(ring.sortedHashes); i++ {
		if ring.sortedHashes[i] < ring.sortedHashes[i-1] {
			t.Error("Sorted hashes are not in ascending order")
			break
		}
	}
}

func TestHashRing_AddNode_Duplicate(t *testing.T) {
	ring := NewHashRing(3, 10)

	err := ring.AddNode("ka-ve-lo", "192.168.1.100:5000")
	if err != nil {
		t.Fatalf("First AddNode failed: %v", err)
	}

	err = ring.AddNode("ka-ve-lo", "192.168.1.101:5000")
	if err == nil {
		t.Error("Expected error for duplicate node, got nil")
	}
}

func TestHashRing_RemoveNode(t *testing.T) {
	ring := NewHashRing(3, 10)

	// Add a node first
	err := ring.AddNode("ka-ve-lo", "192.168.1.100:5000")
	if err != nil {
		t.Fatalf("AddNode failed: %v", err)
	}

	// Verify it was added
	if len(ring.nodes) != 1 {
		t.Errorf("Expected 1 node after add, got %d", len(ring.nodes))
	}

	// Remove the node
	err = ring.RemoveNode("ka-ve-lo")
	if err != nil {
		t.Fatalf("RemoveNode failed: %v", err)
	}

	// Verify it was removed
	if len(ring.nodes) != 0 {
		t.Errorf("Expected 0 nodes after remove, got %d", len(ring.nodes))
	}

	if len(ring.virtualNodes) != 0 {
		t.Errorf("Expected 0 virtual nodes after remove, got %d", len(ring.virtualNodes))
	}

	if len(ring.sortedHashes) != 0 {
		t.Errorf("Expected 0 sorted hashes after remove, got %d", len(ring.sortedHashes))
	}
}

func TestHashRing_RemoveNode_NotExists(t *testing.T) {
	ring := NewHashRing(3, 10)

	err := ring.RemoveNode("nonexistent")
	if err == nil {
		t.Error("Expected error for removing nonexistent node, got nil")
	}
}

func TestHashRing_GetNodeForZone(t *testing.T) {
	ring := NewHashRing(3, 10)

	// Test with empty ring
	_, err := ring.GetNodeForZone("test.path")
	if err == nil {
		t.Error("Expected error for empty ring, got nil")
	}

	// Add some nodes
	nodes := []string{"ka-ve-lo", "ma-li-no", "be-tu-gan"}
	for i, node := range nodes {
		err := ring.AddNode(node, fmt.Sprintf("192.168.1.%d:5000", 100+i))
		if err != nil {
			t.Fatalf("Failed to add node %s: %v", node, err)
		}
	}

	// Test zone assignment
	zonePath := "world.users.alice"
	nodeIdentity, err := ring.GetNodeForZone(zonePath)
	if err != nil {
		t.Fatalf("GetNodeForZone failed: %v", err)
	}

	if nodeIdentity == "" {
		t.Error("Expected non-empty node identity")
	}

	// Verify the node exists in our ring
	_, exists := ring.nodes[nodeIdentity]
	if !exists {
		t.Errorf("Returned node '%s' not found in ring", nodeIdentity)
	}

	// Test determinism - same path should return same node
	nodeIdentity2, err := ring.GetNodeForZone(zonePath)
	if err != nil {
		t.Fatalf("Second GetNodeForZone failed: %v", err)
	}

	if nodeIdentity != nodeIdentity2 {
		t.Errorf("Expected deterministic result: %s != %s", nodeIdentity, nodeIdentity2)
	}

	// Test different paths give different results (usually)
	differentPath := "world.users.bob"
	nodeIdentity3, err := ring.GetNodeForZone(differentPath)
	if err != nil {
		t.Fatalf("Third GetNodeForZone failed: %v", err)
	}

	// Note: This might occasionally fail due to hash collisions, but very unlikely
	if nodeIdentity == nodeIdentity3 {
		t.Logf("Warning: Different paths mapped to same node (hash collision possible)")
	}
}

func TestHashRing_GetReplicaNodes(t *testing.T) {
	ring := NewHashRing(3, 20)

	// Add enough nodes for replicas
	nodes := []string{"ka-ve-lo", "ma-li-no", "be-tu-gan", "ro-si-da", "lu-po-mi"}
	for i, node := range nodes {
		err := ring.AddNode(node, fmt.Sprintf("192.168.1.%d:5000", 100+i))
		if err != nil {
			t.Fatalf("Failed to add node %s: %v", node, err)
		}
	}

	zonePath := "world.users.alice"
	replicas, err := ring.GetReplicaNodes(zonePath)
	if err != nil {
		t.Fatalf("GetReplicaNodes failed: %v", err)
	}

	// Should return up to replica count nodes
	if len(replicas) > ring.replicas {
		t.Errorf("Expected max %d replicas, got %d", ring.replicas, len(replicas))
	}

	// All returned nodes should be unique
	seen := make(map[string]bool)
	for _, replica := range replicas {
		if seen[replica] {
			t.Errorf("Duplicate replica found: %s", replica)
		}
		seen[replica] = true

		// All replicas should exist in ring
		_, exists := ring.nodes[replica]
		if !exists {
			t.Errorf("Replica node '%s' not found in ring", replica)
		}
	}
}

func TestHashRing_GetZoneAssignment(t *testing.T) {
	ring := NewHashRing(3, 20)

	// Add nodes
	nodes := []string{"ka-ve-lo", "ma-li-no", "be-tu-gan", "ro-si-da"}
	for i, node := range nodes {
		err := ring.AddNode(node, fmt.Sprintf("192.168.1.%d:5000", 100+i))
		if err != nil {
			t.Fatalf("Failed to add node %s: %v", node, err)
		}
	}

	zonePath := "world.users.alice"
	assignment, err := ring.GetZoneAssignment(zonePath)
	if err != nil {
		t.Fatalf("GetZoneAssignment failed: %v", err)
	}

	// Verify assignment structure
	if assignment.ZoneID == "" {
		t.Error("Expected non-empty zone ID")
	}

	if assignment.PathPrefix != zonePath {
		t.Errorf("Expected path prefix '%s', got '%s'", zonePath, assignment.PathPrefix)
	}

	if assignment.Authority == "" {
		t.Error("Expected non-empty authority node")
	}

	// Authority should exist in ring
	_, exists := ring.nodes[assignment.Authority]
	if !exists {
		t.Errorf("Authority node '%s' not found in ring", assignment.Authority)
	}

	// Authority should not appear in replicas
	for _, replica := range assignment.Replicas {
		if replica == assignment.Authority {
			t.Errorf("Authority node '%s' should not appear in replicas", assignment.Authority)
		}
	}

	// Replicas should be unique
	seen := make(map[string]bool)
	for _, replica := range assignment.Replicas {
		if seen[replica] {
			t.Errorf("Duplicate replica found: %s", replica)
		}
		seen[replica] = true
	}

	// Total assignment (authority + replicas) should not exceed replica count
	totalNodes := 1 + len(assignment.Replicas)
	if totalNodes > ring.replicas {
		t.Errorf("Expected max %d nodes in assignment, got %d", ring.replicas, totalNodes)
	}
}

func TestHashRing_UpdateNodeStatus(t *testing.T) {
	ring := NewHashRing(3, 10)

	err := ring.AddNode("ka-ve-lo", "192.168.1.100:5000")
	if err != nil {
		t.Fatalf("AddNode failed: %v", err)
	}

	// Verify initially online
	node := ring.nodes["ka-ve-lo"]
	if !node.IsOnline {
		t.Error("Expected node to be online initially")
	}

	// Mark offline
	err = ring.UpdateNodeStatus("ka-ve-lo", false)
	if err != nil {
		t.Fatalf("UpdateNodeStatus failed: %v", err)
	}

	if node.IsOnline {
		t.Error("Expected node to be offline after update")
	}

	// Mark online again
	err = ring.UpdateNodeStatus("ka-ve-lo", true)
	if err != nil {
		t.Fatalf("Second UpdateNodeStatus failed: %v", err)
	}

	if !node.IsOnline {
		t.Error("Expected node to be online after update")
	}
}

func TestHashRing_UpdateNodeStatus_NotExists(t *testing.T) {
	ring := NewHashRing(3, 10)

	err := ring.UpdateNodeStatus("nonexistent", true)
	if err == nil {
		t.Error("Expected error for updating nonexistent node, got nil")
	}
}

func TestHashRing_UpdateNodeMetrics(t *testing.T) {
	ring := NewHashRing(3, 10)

	err := ring.AddNode("ka-ve-lo", "192.168.1.100:5000")
	if err != nil {
		t.Fatalf("AddNode failed: %v", err)
	}

	// Create test metrics
	metrics := &LoadMetrics{
		DataSize:      1024 * 1024 * 100, // 100MB
		QueryRate:     150.5,
		WriteRate:     25.3,
		CPUUsage:      65.2,
		MemoryUsage:   80.1,
		LastUpdated:   time.Now().Unix(),
	}

	err = ring.UpdateNodeMetrics("ka-ve-lo", metrics)
	if err != nil {
		t.Fatalf("UpdateNodeMetrics failed: %v", err)
	}

	// Verify metrics were updated
	node := ring.nodes["ka-ve-lo"]
	if node.LoadMetrics.DataSize != metrics.DataSize {
		t.Errorf("Expected DataSize %d, got %d", metrics.DataSize, node.LoadMetrics.DataSize)
	}

	if node.LoadMetrics.CPUUsage != metrics.CPUUsage {
		t.Errorf("Expected CPUUsage %f, got %f", metrics.CPUUsage, node.LoadMetrics.CPUUsage)
	}
}

func TestHashRing_GetLoadBalancedNode(t *testing.T) {
	ring := NewHashRing(3, 10)

	// Add nodes with different load metrics
	node1 := "ka-ve-lo"
	node2 := "ma-li-no"
	node3 := "be-tu-gan"

	err := ring.AddNode(node1, "192.168.1.100:5000")
	if err != nil {
		t.Fatalf("Failed to add node1: %v", err)
	}

	err = ring.AddNode(node2, "192.168.1.101:5000")
	if err != nil {
		t.Fatalf("Failed to add node2: %v", err)
	}

	err = ring.AddNode(node3, "192.168.1.102:5000")
	if err != nil {
		t.Fatalf("Failed to add node3: %v", err)
	}

	// Set different load metrics
	highLoadMetrics := &LoadMetrics{CPUUsage: 90, MemoryUsage: 85, QueryRate: 200, WriteRate: 50}
	mediumLoadMetrics := &LoadMetrics{CPUUsage: 60, MemoryUsage: 55, QueryRate: 100, WriteRate: 25}
	lowLoadMetrics := &LoadMetrics{CPUUsage: 30, MemoryUsage: 35, QueryRate: 50, WriteRate: 10}

	ring.UpdateNodeMetrics(node1, highLoadMetrics)
	ring.UpdateNodeMetrics(node2, mediumLoadMetrics)
	ring.UpdateNodeMetrics(node3, lowLoadMetrics)

	candidates := []string{node1, node2, node3}
	selectedNode, err := ring.GetLoadBalancedNode(candidates)
	if err != nil {
		t.Fatalf("GetLoadBalancedNode failed: %v", err)
	}

	// Should select the node with lowest load (node3)
	if selectedNode != node3 {
		t.Errorf("Expected lowest load node '%s', got '%s'", node3, selectedNode)
	}
}

func TestHashRing_GetLoadBalancedNode_EmptyCandidates(t *testing.T) {
	ring := NewHashRing(3, 10)

	_, err := ring.GetLoadBalancedNode([]string{})
	if err == nil {
		t.Error("Expected error for empty candidates, got nil")
	}
}

func TestHashRing_GetLoadBalancedNode_OfflineNodes(t *testing.T) {
	ring := NewHashRing(3, 10)

	err := ring.AddNode("ka-ve-lo", "192.168.1.100:5000")
	if err != nil {
		t.Fatalf("AddNode failed: %v", err)
	}

	// Mark node offline
	ring.UpdateNodeStatus("ka-ve-lo", false)

	candidates := []string{"ka-ve-lo"}
	_, err = ring.GetLoadBalancedNode(candidates)
	if err == nil {
		t.Error("Expected error for offline nodes, got nil")
	}
}

func TestHashRing_ConsistentHashing(t *testing.T) {
	ring1 := NewHashRing(3, 100)
	ring2 := NewHashRing(3, 100)

	// Add same nodes to both rings
	nodes := []string{"ka-ve-lo", "ma-li-no", "be-tu-gan"}
	for _, node := range nodes {
		ring1.AddNode(node, "192.168.1.100:5000")
		ring2.AddNode(node, "192.168.1.100:5000")
	}

	// Test paths should map to same nodes in both rings
	testPaths := []string{
		"world.users.alice",
		"world.users.bob",
		"world.products.laptop",
		"world.orders.12345",
		"world.inventory.widgets",
	}

	for _, path := range testPaths {
		node1, err1 := ring1.GetNodeForZone(path)
		node2, err2 := ring2.GetNodeForZone(path)

		if err1 != nil || err2 != nil {
			t.Errorf("GetNodeForZone failed for path '%s': err1=%v, err2=%v", path, err1, err2)
			continue
		}

		if node1 != node2 {
			t.Errorf("Inconsistent mapping for path '%s': ring1=%s, ring2=%s", path, node1, node2)
		}
	}
}

func TestHashRing_MinimalRedistribution(t *testing.T) {
	ring := NewHashRing(3, 50)

	// Add initial nodes
	initialNodes := []string{"ka-ve-lo", "ma-li-no", "be-tu-gan"}
	for _, node := range initialNodes {
		ring.AddNode(node, "192.168.1.100:5000")
	}

	// Record initial mappings
	testPaths := []string{
		"world.users.alice", "world.users.bob", "world.users.charlie",
		"world.products.laptop", "world.products.mouse", "world.products.keyboard",
		"world.orders.1", "world.orders.2", "world.orders.3",
	}

	initialMappings := make(map[string]string)
	for _, path := range testPaths {
		node, _ := ring.GetNodeForZone(path)
		initialMappings[path] = node
	}

	// Add a new node
	ring.AddNode("new-node-do-re-mi", "192.168.1.104:5000")

	// Check how many mappings changed
	changedCount := 0
	for _, path := range testPaths {
		newNode, _ := ring.GetNodeForZone(path)
		if newNode != initialMappings[path] {
			changedCount++
		}
	}

	// Should minimize redistribution (not all paths should change)
	if changedCount == len(testPaths) {
		t.Error("All paths changed nodes - hash ring should minimize redistribution")
	}

	if changedCount == 0 {
		t.Error("No paths changed nodes - new node should get some assignments")
	}

	t.Logf("Redistribution: %d/%d paths changed nodes", changedCount, len(testPaths))
}

func TestHashRing_VirtualNodeDistribution(t *testing.T) {
	ring := NewHashRing(3, 100)

	// Add nodes
	nodes := []string{"ka-ve-lo", "ma-li-no", "be-tu-gan"}
	for _, node := range nodes {
		ring.AddNode(node, "192.168.1.100:5000")
	}

	// Count virtual nodes per physical node
	virtualNodeCounts := make(map[string]int)
	for _, physicalNode := range ring.virtualNodes {
		virtualNodeCounts[physicalNode]++
	}

	// Each physical node should have exactly vnodeCount virtual nodes
	for _, node := range nodes {
		count := virtualNodeCounts[node]
		if count != ring.vnodeCount {
			t.Errorf("Node '%s' has %d virtual nodes, expected %d", node, count, ring.vnodeCount)
		}
	}

	// Total virtual nodes should equal nodes * vnodeCount
	totalVNodes := len(ring.virtualNodes)
	expectedTotal := len(nodes) * ring.vnodeCount
	if totalVNodes != expectedTotal {
		t.Errorf("Total virtual nodes %d, expected %d", totalVNodes, expectedTotal)
	}
}
