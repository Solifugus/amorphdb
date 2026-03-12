package zone

import (
	"fmt"
	"testing"
)

// TestStep12_2_HashRing tests Step 12.2: Hash Ring functionality
func TestStep12_2_HashRing(t *testing.T) {
	t.Log("🚀 === STEP 12.2: Hash Ring Test ===")

	// Test 1: Hash Ring Creation and Node Management
	t.Log("✅ Test 1: Hash ring creation and node management")
	testHashRingNodeManagement(t)

	// Test 2: Zone Assignment and Load Distribution
	t.Log("✅ Test 2: Zone assignment and load distribution")
	testHashRingZoneAssignment(t)

	// Test 3: Node Failure and Recovery
	t.Log("✅ Test 3: Node failure and recovery")
	testHashRingFailureRecovery(t)

	// Test 4: Load Balancing
	t.Log("✅ Test 4: Load balancing")
	testHashRingLoadBalancing(t)

	t.Log("")
	t.Log("🎉 === STEP 12.2 HASH RING TEST PASSED! === 🎉")
	printHashRingSuccessCriteria(t)
}

func testHashRingNodeManagement(t *testing.T) {
	// Create hash ring with 2 replicas and 100 virtual nodes
	ring := NewHashRing(2, 100)

	// Verify initial state
	if ring.GetRingSize() != 0 {
		t.Errorf("New ring should be empty, got size %d", ring.GetRingSize())
	}

	if ring.GetVirtualNodeCount() != 0 {
		t.Errorf("New ring should have no virtual nodes, got %d", ring.GetVirtualNodeCount())
	}

	if ring.GetReplicaCount() != 2 {
		t.Errorf("Replica count should be 2, got %d", ring.GetReplicaCount())
	}

	// Add first node
	node1 := "ko-lu-ven"
	err := ring.AddNode(node1, "127.0.0.1:5001")
	if err != nil {
		t.Fatalf("Failed to add node1: %v", err)
	}

	if ring.GetRingSize() != 1 {
		t.Errorf("Ring size should be 1, got %d", ring.GetRingSize())
	}

	if ring.GetVirtualNodeCount() != 100 {
		t.Errorf("Virtual node count should be 100, got %d", ring.GetVirtualNodeCount())
	}

	// Add second node
	node2 := "ta-gi-mor"
	err = ring.AddNode(node2, "127.0.0.1:5002")
	if err != nil {
		t.Fatalf("Failed to add node2: %v", err)
	}

	if ring.GetRingSize() != 2 {
		t.Errorf("Ring size should be 2, got %d", ring.GetRingSize())
	}

	if ring.GetVirtualNodeCount() != 200 {
		t.Errorf("Virtual node count should be 200, got %d", ring.GetVirtualNodeCount())
	}

	// Try to add duplicate node (should fail)
	err = ring.AddNode(node1, "127.0.0.1:5003")
	if err == nil {
		t.Error("Adding duplicate node should fail")
	}

	// Remove a node
	err = ring.RemoveNode(node1)
	if err != nil {
		t.Fatalf("Failed to remove node1: %v", err)
	}

	if ring.GetRingSize() != 1 {
		t.Errorf("Ring size should be 1 after removal, got %d", ring.GetRingSize())
	}

	if ring.GetVirtualNodeCount() != 100 {
		t.Errorf("Virtual node count should be 100 after removal, got %d", ring.GetVirtualNodeCount())
	}

	// Try to remove non-existent node
	err = ring.RemoveNode("non-existent")
	if err == nil {
		t.Error("Removing non-existent node should fail")
	}

	t.Log("  ✓ Hash ring node management working")
}

func testHashRingZoneAssignment(t *testing.T) {
	ring := NewHashRing(1, 100) // 1 replica, 100 virtual nodes

	// Add two nodes
	node1 := "mi-ka-zon"
	node2 := "xe-yu-va"

	err := ring.AddNode(node1, "127.0.0.1:6001")
	if err != nil {
		t.Fatalf("Failed to add node1: %v", err)
	}

	err = ring.AddNode(node2, "127.0.0.1:6002")
	if err != nil {
		t.Fatalf("Failed to add node2: %v", err)
	}

	// Test zone assignments for various paths
	testPaths := []string{
		"users.alice.profile",
		"users.bob.settings",
		"company.departments.engineering",
		"company.departments.marketing",
		"system.config.database",
		"products.amorphdb.features",
	}

	assignmentCounts := make(map[string]int)

	for _, path := range testPaths {
		// Test GetNodeForZone
		authority, err := ring.GetNodeForZone(path)
		if err != nil {
			t.Fatalf("Failed to get authority for path %s: %v", path, err)
		}

		assignmentCounts[authority]++

		// Test GetReplicaNodes
		replicas, err := ring.GetReplicaNodes(path)
		if err != nil {
			t.Fatalf("Failed to get replicas for path %s: %v", path, err)
		}

		// Should have exactly 1 replica (our configuration)
		if len(replicas) != 1 {
			t.Errorf("Expected 1 replica for path %s, got %d", path, len(replicas))
		}

		// Replica should be different from authority
		if len(replicas) > 0 && replicas[0] == authority {
			t.Errorf("Authority %s should not be the same as replica for path %s", authority, path)
		}

		// Test GetZoneAssignment (complete assignment)
		assignment, err := ring.GetZoneAssignment(path)
		if err != nil {
			t.Fatalf("Failed to get zone assignment for path %s: %v", path, err)
		}

		if assignment.Authority != authority {
			t.Errorf("Zone assignment authority mismatch for path %s", path)
		}

		if assignment.PathPrefix != path {
			t.Errorf("Zone assignment path prefix mismatch for path %s", path)
		}

		if assignment.ZoneID == "" {
			t.Errorf("Zone assignment should have non-empty zone ID for path %s", path)
		}

		t.Logf("  Path: %s -> Authority: %s, Replicas: %v, ZoneID: %s",
			path, assignment.Authority, assignment.Replicas, assignment.ZoneID)
	}

	// Verify load distribution
	node1Count := assignmentCounts[node1]
	node2Count := assignmentCounts[node2]

	if node1Count == 0 || node2Count == 0 {
		t.Errorf("Zone assignment completely skewed: %s=%d, %s=%d",
			node1, node1Count, node2, node2Count)
	}

	// Test deterministic assignment
	for _, path := range testPaths {
		authority1, _ := ring.GetNodeForZone(path)
		authority2, _ := ring.GetNodeForZone(path)
		if authority1 != authority2 {
			t.Errorf("Zone assignment not deterministic for path %s", path)
		}
	}

	t.Log("  ✓ Zone assignment working correctly")
}

func testHashRingFailureRecovery(t *testing.T) {
	ring := NewHashRing(1, 50)

	// Add three nodes
	nodes := []string{"ko-lu-ven", "ta-gi-mor", "mi-ka-zon"}
	for i, node := range nodes {
		err := ring.AddNode(node, fmt.Sprintf("127.0.0.1:700%d", i+1))
		if err != nil {
			t.Fatalf("Failed to add node %s: %v", node, err)
		}
	}

	testPath := "test.failure.recovery"

	// Get initial assignment
	initialAuthority, err := ring.GetNodeForZone(testPath)
	if err != nil {
		t.Fatalf("Failed to get initial authority: %v", err)
	}

	// Mark the authority node as offline
	err = ring.UpdateNodeStatus(initialAuthority, false)
	if err != nil {
		t.Fatalf("Failed to update node status: %v", err)
	}

	// Verify the node is marked offline
	allNodes := ring.GetAllNodes()
	if allNodes[initialAuthority].IsOnline {
		t.Error("Node should be marked offline after status update")
	}

	// Get new assignment (should be different since original authority is offline)
	newReplicas, err := ring.GetReplicaNodes(testPath)
	if err != nil {
		t.Fatalf("Failed to get replicas after node failure: %v", err)
	}

	// Should not include the offline node
	for _, replica := range newReplicas {
		if replica == initialAuthority {
			t.Errorf("Offline node %s should not appear in replica list", initialAuthority)
		}
	}

	// Bring the node back online
	err = ring.UpdateNodeStatus(initialAuthority, true)
	if err != nil {
		t.Fatalf("Failed to bring node back online: %v", err)
	}

	// Verify the node is back online
	allNodes = ring.GetAllNodes()
	if !allNodes[initialAuthority].IsOnline {
		t.Error("Node should be marked online after recovery")
	}

	t.Log("  ✓ Node failure and recovery working")
}

func testHashRingLoadBalancing(t *testing.T) {
	ring := NewHashRing(1, 100)

	// Add nodes with different load metrics
	node1 := "load-test-1"
	node2 := "load-test-2"
	node3 := "load-test-3"

	nodes := []string{node1, node2, node3}
	for i, node := range nodes {
		err := ring.AddNode(node, fmt.Sprintf("127.0.0.1:800%d", i+1))
		if err != nil {
			t.Fatalf("Failed to add node %s: %v", node, err)
		}
	}

	// Set different load metrics
	metrics1 := &LoadMetrics{
		CPUUsage:    80.0, // High CPU
		MemoryUsage: 60.0,
		QueryRate:   50.0,
		WriteRate:   30.0,
	}

	metrics2 := &LoadMetrics{
		CPUUsage:    30.0, // Low CPU (should be preferred)
		MemoryUsage: 40.0,
		QueryRate:   20.0,
		WriteRate:   10.0,
	}

	metrics3 := &LoadMetrics{
		CPUUsage:    90.0, // Very high CPU
		MemoryUsage: 80.0,
		QueryRate:   100.0,
		WriteRate:   60.0,
	}

	ring.UpdateNodeMetrics(node1, metrics1)
	ring.UpdateNodeMetrics(node2, metrics2)
	ring.UpdateNodeMetrics(node3, metrics3)

	// Test load balanced selection
	candidates := []string{node1, node2, node3}
	selected, err := ring.GetLoadBalancedNode(candidates)
	if err != nil {
		t.Fatalf("Failed to get load balanced node: %v", err)
	}

	// Should select node2 (lowest load)
	if selected != node2 {
		t.Errorf("Load balancer should select %s (lowest load), got %s", node2, selected)
	}

	// Test with empty candidates
	_, err = ring.GetLoadBalancedNode([]string{})
	if err == nil {
		t.Error("Load balancer should fail with empty candidates")
	}

	// Test with offline nodes
	ring.UpdateNodeStatus(node2, false)
	selected, err = ring.GetLoadBalancedNode(candidates)
	if err != nil {
		t.Fatalf("Failed to get load balanced node after offline: %v", err)
	}

	// Should now select node1 (node2 is offline, node1 has lower load than node3)
	if selected != node1 {
		t.Errorf("Load balancer should select %s after %s went offline, got %s", node1, node2, selected)
	}

	t.Log("  ✓ Load balancing working correctly")
}

func printHashRingSuccessCriteria(t *testing.T) {
	t.Log("=== STEP 12.2 HASH RING SUCCESS CRITERIA ===")
	t.Log("✓ Node Management: Add/remove nodes working correctly")
	t.Log("✓ Virtual Nodes: Consistent hashing with virtual nodes functional")
	t.Log("✓ Zone Assignment: Deterministic path-to-node mapping working")
	t.Log("✓ Replica Selection: Proper replica assignment working")
	t.Log("✓ Failure Handling: Node offline/online state management working")
	t.Log("✓ Load Balancing: Metrics-based node selection working")
	t.Log("")
	t.Log("🎯 Hash ring infrastructure ready for two-node mesh operations")
}
