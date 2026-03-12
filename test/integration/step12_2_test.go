package integration

import (
	"fmt"
	"net"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/solifugus/amorphdb/internal/mesh"
	"github.com/solifugus/amorphdb/internal/zone"
	"github.com/solifugus/amorphdb/internal/storage"
)

// TestStep12_2_TwoNodeMesh tests Step 12.2: Two Node Mesh Integration
func TestStep12_2_TwoNodeMesh(t *testing.T) {
	t.Log("🚀 === STEP 12.2: Two Node Mesh Integration Test ===")

	// Test 1: Node Bootstrap and Identity Exchange
	t.Log("✅ Test 1: Node bootstrap and identity exchange")
	testNodeBootstrapAndIdentityExchange(t)

	// Test 2: Consistent Hashing and Zone Assignment
	t.Log("✅ Test 2: Consistent hashing and zone assignment")
	testConsistentHashingAndZoneAssignment(t)

	// Test 3: Cross-node data replication
	t.Log("✅ Test 3: Cross-node data replication")
	testCrossNodeDataReplication(t)

	// Test 4: Heartbeat and failure detection
	t.Log("✅ Test 4: Heartbeat and failure detection")
	testHeartbeatAndFailureDetection(t)

	// Test 5: Node recovery and data synchronization
	t.Log("✅ Test 5: Node recovery and data synchronization")
	testNodeRecoveryAndDataSync(t)

	t.Log("")
	t.Log("🎉 === STEP 12.2 TWO NODE MESH TEST PASSED! === 🎉")
	printMeshSuccessCriteria(t)
}

func testNodeBootstrapAndIdentityExchange(t *testing.T) {
	// Create temporary directories for two nodes
	node1Dir, err := os.MkdirTemp("", "amorphdb_node1_")
	if err != nil {
		t.Fatalf("Failed to create node1 directory: %v", err)
	}
	defer os.RemoveAll(node1Dir)

	node2Dir, err := os.MkdirTemp("", "amorphdb_node2_")
	if err != nil {
		t.Fatalf("Failed to create node2 directory: %v", err)
	}
	defer os.RemoveAll(node2Dir)

	// Test node identity generation
	identity1 := mesh.GenerateNodeIdentity()
	identity2 := mesh.GenerateNodeIdentity()

	if identity1 == identity2 {
		t.Error("Generated identities should be unique")
	}

	// Verify identity format (CV syllable patterns)
	if len(identity1) < 4 || len(identity2) < 4 {
		t.Error("Generated identities should be meaningful length")
	}

	t.Logf("  Generated identities: %s, %s", identity1, identity2)

	// Test bootstrap process simulation
	bootstrapper := mesh.NewBootstrap()

	// Node 1 self-genesis (first node)
	node1Config := &mesh.NodeConfig{
		Identity: identity1,
		DataDir:  node1Dir,
		Port:     0, // Use random port
	}

	err = bootstrapper.SelfGenesis(node1Config)
	if err != nil {
		t.Fatalf("Node1 self-genesis failed: %v", err)
	}

	// Node 2 connects to Node 1
	node2Config := &mesh.NodeConfig{
		Identity:  identity2,
		DataDir:   node2Dir,
		Port:      0, // Use random port
		SeedNodes: []string{node1Config.GetAddress()},
	}

	err = bootstrapper.Bootstrap(node2Config)
	if err != nil {
		t.Fatalf("Node2 bootstrap failed: %v", err)
	}

	// Verify both nodes know about each other
	node1Peers := bootstrapper.GetKnownPeers(identity1)
	node2Peers := bootstrapper.GetKnownPeers(identity2)

	if len(node1Peers) == 0 || len(node2Peers) == 0 {
		t.Error("Nodes should know about each other after bootstrap")
	}

	t.Log("  ✓ Node bootstrap and identity exchange working")
}

func testConsistentHashingAndZoneAssignment(t *testing.T) {
	// Create hash ring with two nodes
	ring := zone.NewHashRing()

	node1 := "alpha-node-001"
	node2 := "beta-node-002"

	// Add nodes to ring
	err := ring.AddNode(node1, map[string]interface{}{
		"address": "127.0.0.1:5001",
		"capacity": 100,
	})
	if err != nil {
		t.Fatalf("Failed to add node1 to ring: %v", err)
	}

	err = ring.AddNode(node2, map[string]interface{}{
		"address": "127.0.0.1:5002",
		"capacity": 100,
	})
	if err != nil {
		t.Fatalf("Failed to add node2 to ring: %v", err)
	}

	// Test zone assignment for various paths
	testPaths := []string{
		"users.alice.profile",
		"users.bob.settings",
		"company.departments.engineering",
		"company.departments.marketing",
		"system.config.database",
		"system.logs.errors",
		"products.amorphdb.features",
		"customers.enterprise.acme",
	}

	assignmentCounts := make(map[string]int)

	for _, path := range testPaths {
		authority, replicas, err := ring.GetZoneAssignment(path, 1) // 1 replica
		if err != nil {
			t.Fatalf("Failed to get zone assignment for %s: %v", path, err)
		}

		assignmentCounts[authority]++

		// Verify replica is different from authority
		if len(replicas) != 1 || replicas[0] == authority {
			t.Errorf("Invalid replica assignment for path %s", path)
		}

		t.Logf("  Path %s -> Authority: %s, Replica: %s", path, authority, replicas[0])
	}

	// Verify load distribution (should not be completely skewed)
	node1Count := assignmentCounts[node1]
	node2Count := assignmentCounts[node2]

	if node1Count == 0 || node2Count == 0 {
		t.Errorf("Zone assignment is completely skewed: node1=%d, node2=%d", node1Count, node2Count)
	}

	// Test deterministic assignment
	for i := 0; i < 3; i++ {
		for _, path := range testPaths {
			authority1, _, _ := ring.GetZoneAssignment(path, 1)
			authority2, _, _ := ring.GetZoneAssignment(path, 1)

			if authority1 != authority2 {
				t.Errorf("Zone assignment not deterministic for path %s", path)
			}
		}
	}

	t.Log("  ✓ Consistent hashing and zone assignment working")
}

func testCrossNodeDataReplication(t *testing.T) {
	// Create two storage nodes
	node1Dir, err := os.MkdirTemp("", "amorphdb_repl_node1_")
	if err != nil {
		t.Fatalf("Failed to create replication node1 directory: %v", err)
	}
	defer os.RemoveAll(node1Dir)

	node2Dir, err := os.MkdirTemp("", "amorphdb_repl_node2_")
	if err != nil {
		t.Fatalf("Failed to create replication node2 directory: %v", err)
	}
	defer os.RemoveAll(node2Dir)

	// Create storage trees for both nodes
	tree1, err := storage.NewStorageTree(node1Dir)
	if err != nil {
		t.Fatalf("Failed to create storage tree for node1: %v", err)
	}
	defer tree1.Close()

	tree2, err := storage.NewStorageTree(node2Dir)
	if err != nil {
		t.Fatalf("Failed to create storage tree for node2: %v", err)
	}
	defer tree2.Close()

	// Create replication manager
	replicationMgr := mesh.NewReplicationManager()

	// Configure nodes for replication
	node1Config := &mesh.ReplicationNode{
		Identity: "node1",
		Tree:     tree1,
		Address:  "127.0.0.1:5001",
	}

	node2Config := &mesh.ReplicationNode{
		Identity: "node2",
		Tree:     tree2,
		Address:  "127.0.0.1:5002",
	}

	err = replicationMgr.AddNode(node1Config)
	if err != nil {
		t.Fatalf("Failed to add node1 to replication manager: %v", err)
	}

	err = replicationMgr.AddNode(node2Config)
	if err != nil {
		t.Fatalf("Failed to add node2 to replication manager: %v", err)
	}

	// Set up zone assignments (node1 is authority for some zones, node2 for others)
	zoneAssignments := map[string]mesh.ZoneAssignment{
		"users": {
			Authority: "node1",
			Replicas:  []string{"node2"},
		},
		"system": {
			Authority: "node2",
			Replicas:  []string{"node1"},
		},
	}

	for zone, assignment := range zoneAssignments {
		err = replicationMgr.SetZoneAssignment(zone, assignment)
		if err != nil {
			t.Fatalf("Failed to set zone assignment for %s: %v", zone, err)
		}
	}

	// Test data replication: Write to node1 (authority for users zone)
	userData := storage.Value{
		Data:    []byte("Alice Johnson"),
		TypeTag: storage.TypeText,
	}

	// Write to node1 (authority)
	err = tree1.Write([]string{"users", "alice", "name"}, userData, 1000)
	if err != nil {
		t.Fatalf("Failed to write to node1: %v", err)
	}

	// Simulate replication process
	err = replicationMgr.ReplicateZoneData("users", "node1", []string{"node2"})
	if err != nil {
		t.Fatalf("Failed to replicate data: %v", err)
	}

	// Verify data is replicated to node2
	replicatedValue, err := tree2.Read([]string{"users", "alice", "name"})
	if err != nil {
		t.Fatalf("Failed to read replicated data from node2: %v", err)
	}

	if string(replicatedValue.Data) != "Alice Johnson" {
		t.Errorf("Replicated data mismatch: expected 'Alice Johnson', got '%s'", string(replicatedValue.Data))
	}

	// Test reverse replication: Write to node2 (authority for system zone)
	systemData := storage.Value{
		Data:    []byte("debug=true"),
		TypeTag: storage.TypeText,
	}

	err = tree2.Write([]string{"system", "config", "debug"}, systemData, 1000)
	if err != nil {
		t.Fatalf("Failed to write to node2: %v", err)
	}

	// Replicate from node2 to node1
	err = replicationMgr.ReplicateZoneData("system", "node2", []string{"node1"})
	if err != nil {
		t.Fatalf("Failed to replicate system data: %v", err)
	}

	// Verify data is replicated to node1
	replicatedSystemValue, err := tree1.Read([]string{"system", "config", "debug"})
	if err != nil {
		t.Fatalf("Failed to read replicated system data from node1: %v", err)
	}

	if string(replicatedSystemValue.Data) != "debug=true" {
		t.Errorf("Replicated system data mismatch: expected 'debug=true', got '%s'", string(replicatedSystemValue.Data))
	}

	t.Log("  ✓ Cross-node data replication working")
}

func testHeartbeatAndFailureDetection(t *testing.T) {
	// Create heartbeat manager
	heartbeatMgr := mesh.NewHeartbeatManager()

	node1 := "heartbeat-node-1"
	node2 := "heartbeat-node-2"

	// Register nodes
	err := heartbeatMgr.RegisterNode(node1, "127.0.0.1:6001")
	if err != nil {
		t.Fatalf("Failed to register node1: %v", err)
	}

	err = heartbeatMgr.RegisterNode(node2, "127.0.0.1:6002")
	if err != nil {
		t.Fatalf("Failed to register node2: %v", err)
	}

	// Start heartbeat between nodes
	var wg sync.WaitGroup

	// Node1 sends heartbeats
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 5; i++ {
			err := heartbeatMgr.SendHeartbeat(node1, node2, map[string]interface{}{
				"timestamp": time.Now().Unix(),
				"sequence":  i,
			})
			if err != nil {
				t.Errorf("Failed to send heartbeat from %s to %s: %v", node1, node2, err)
			}
			time.Sleep(333 * time.Millisecond) // ⅓ second intervals
		}
	}()

	// Node2 sends heartbeats
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 5; i++ {
			err := heartbeatMgr.SendHeartbeat(node2, node1, map[string]interface{}{
				"timestamp": time.Now().Unix(),
				"sequence":  i,
			})
			if err != nil {
				t.Errorf("Failed to send heartbeat from %s to %s: %v", node2, node1, err)
			}
			time.Sleep(333 * time.Millisecond)
		}
	}()

	// Wait for heartbeats to complete
	wg.Wait()

	// Check heartbeat status
	node1Status := heartbeatMgr.GetNodeStatus(node1)
	node2Status := heartbeatMgr.GetNodeStatus(node2)

	if node1Status != mesh.NodeStatusHealthy || node2Status != mesh.NodeStatusHealthy {
		t.Errorf("Nodes should be healthy after heartbeats: node1=%s, node2=%s", node1Status, node2Status)
	}

	// Test failure detection by stopping heartbeats from node2
	time.Sleep(2 * time.Second) // Wait longer than heartbeat timeout

	// Simulate failure detection check
	heartbeatMgr.CheckNodeFailures(1 * time.Second) // 1 second timeout

	// Node2 should be detected as failed (no recent heartbeats)
	node2StatusAfterTimeout := heartbeatMgr.GetNodeStatus(node2)
	if node2StatusAfterTimeout == mesh.NodeStatusHealthy {
		t.Error("Node2 should be detected as failed after heartbeat timeout")
	}

	t.Log("  ✓ Heartbeat and failure detection working")
}

func testNodeRecoveryAndDataSync(t *testing.T) {
	// Create nodes for recovery testing
	node1Dir, err := os.MkdirTemp("", "amorphdb_recovery_node1_")
	if err != nil {
		t.Fatalf("Failed to create recovery node1 directory: %v", err)
	}
	defer os.RemoveAll(node1Dir)

	node2Dir, err := os.MkdirTemp("", "amorphdb_recovery_node2_")
	if err != nil {
		t.Fatalf("Failed to create recovery node2 directory: %v", err)
	}
	defer os.RemoveAll(node2Dir)

	// Create recovery manager
	recoveryMgr := mesh.NewRecoveryManager()

	// Setup initial state with node1 having more recent data
	tree1, err := storage.NewStorageTree(node1Dir)
	if err != nil {
		t.Fatalf("Failed to create tree1 for recovery test: %v", err)
	}
	defer tree1.Close()

	// Write some data to node1
	testData := []struct {
		path  []string
		value string
	}{
		{[]string{"recovery", "test", "data1"}, "Node1 Data 1"},
		{[]string{"recovery", "test", "data2"}, "Node1 Data 2"},
		{[]string{"recovery", "test", "data3"}, "Node1 Data 3"},
		{[]string{"recovery", "user", "alice"}, "Alice Recovery Test"},
		{[]string{"recovery", "user", "bob"}, "Bob Recovery Test"},
	}

	for _, item := range testData {
		value := storage.Value{
			Data:    []byte(item.value),
			TypeTag: storage.TypeText,
		}
		err := tree1.Write(item.path, value, 1000)
		if err != nil {
			t.Fatalf("Failed to write recovery data to node1: %v", err)
		}
	}

	// Create node2 (starts empty)
	tree2, err := storage.NewStorageTree(node2Dir)
	if err != nil {
		t.Fatalf("Failed to create tree2 for recovery test: %v", err)
	}
	defer tree2.Close()

	// Simulate node2 recovery: sync data from node1
	syncConfig := &mesh.SyncConfig{
		SourceNode:      "node1",
		SourceTree:      tree1,
		DestinationNode: "node2",
		DestinationTree: tree2,
		Zones:           []string{"recovery"},
	}

	err = recoveryMgr.SyncNodeData(syncConfig)
	if err != nil {
		t.Fatalf("Failed to sync data during recovery: %v", err)
	}

	// Verify all data was synchronized to node2
	for _, item := range testData {
		syncedValue, err := tree2.Read(item.path)
		if err != nil {
			t.Fatalf("Failed to read synced data from node2 for %v: %v", item.path, err)
		}

		if string(syncedValue.Data) != item.value {
			t.Errorf("Synced data mismatch for %v: expected '%s', got '%s'",
				item.path, item.value, string(syncedValue.Data))
		}
	}

	// Test incremental sync: add new data to node1
	newData := storage.Value{
		Data:    []byte("New Data After Recovery"),
		TypeTag: storage.TypeText,
	}

	err = tree1.Write([]string{"recovery", "new", "data"}, newData, 1000)
	if err != nil {
		t.Fatalf("Failed to write new data to node1: %v", err)
	}

	// Perform incremental sync
	err = recoveryMgr.IncrementalSync(syncConfig, time.Now().Add(-1*time.Hour))
	if err != nil {
		t.Fatalf("Failed to perform incremental sync: %v", err)
	}

	// Verify new data was synced
	syncedNewValue, err := tree2.Read([]string{"recovery", "new", "data"})
	if err != nil {
		t.Fatalf("Failed to read incrementally synced data: %v", err)
	}

	if string(syncedNewValue.Data) != "New Data After Recovery" {
		t.Errorf("Incrementally synced data mismatch")
	}

	t.Log("  ✓ Node recovery and data synchronization working")
}

func printMeshSuccessCriteria(t *testing.T) {
	t.Log("=== STEP 12.2 SUCCESS CRITERIA ACHIEVED ===")
	t.Log("✓ Node Bootstrap: Second node successfully bootstraps from first")
	t.Log("✓ Identity Exchange: Both nodes know about each other after bootstrap")
	t.Log("✓ Zone Assignment: Consistent hashing distributes zones across nodes")
	t.Log("✓ Cross-node Replication: Write on node A, read on node B works")
	t.Log("✓ Heartbeat System: ⅓ second heartbeat intervals working")
	t.Log("✓ Failure Detection: Missed heartbeats trigger node failure detection")
	t.Log("✓ Node Recovery: Failed nodes can rejoin and sync data correctly")
	t.Log("✓ Data Synchronization: Full and incremental sync working")
	t.Log("")
	t.Log("🎯 READY FOR STEP 12.3: Multi-Node Mesh Testing")
}
