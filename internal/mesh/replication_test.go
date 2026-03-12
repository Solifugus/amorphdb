package mesh
import "fmt"

import (
	"testing"
	"time"

	"github.com/solifugus/amorphdb/internal/storage"
	"github.com/solifugus/amorphdb/internal/zone"
)

// mockStorageTree implements storage.Tree for testing
type mockStorageTree struct {
	data map[string]storage.Value
}

func newMockStorageTree() *mockStorageTree {
	return &mockStorageTree{
		data: make(map[string]storage.Value),
	}
}

func (m *mockStorageTree) Read(path []string) (storage.Value, error) {
	key := pathToKey(path)
	if value, exists := m.data[key]; exists {
		return value, nil
	}
	return storage.Value{}, fmt.Errorf("path not found: %v", path)
}

func (m *mockStorageTree) Write(path []string, value storage.Value, author uint64) error {
	key := pathToKey(path)
	m.data[key] = value
	return nil
}

func (m *mockStorageTree) ReadAt(path []string, timestamp int64) (storage.Value, error) {
	return m.Read(path) // Simplified for testing
}

func (m *mockStorageTree) Children(path []string) ([]storage.Attribute, error) {
	return []storage.Attribute{}, nil // Simplified for testing
}

func (m *mockStorageTree) Purge(path []string, from int64, to int64, author uint64) error {
	return nil // Simplified for testing
}

func (m *mockStorageTree) Close() error {
	return nil
}

func pathToKey(path []string) string {
	result := ""
	for i, segment := range path {
		if i > 0 {
			result += "."
		}
		result += segment
	}
	return result
}

func TestNewReplicationManager(t *testing.T) {
	hashRing := zone.NewHashRing(3, 10)
	storageTree := newMockStorageTree()

	rm := NewReplicationManager("test-node", hashRing, storageTree)

	if rm.nodeIdentity != "test-node" {
		t.Errorf("Expected nodeIdentity 'test-node', got '%s'", rm.nodeIdentity)
	}

	if rm.hashRing != hashRing {
		t.Error("Expected hash ring to be set")
	}

	if rm.storageTree != storageTree {
		t.Error("Expected storage tree to be set")
	}

	if len(rm.authorityZones) != 0 {
		t.Errorf("Expected 0 authority zones initially, got %d", len(rm.authorityZones))
	}

	if len(rm.replicaZones) != 0 {
		t.Errorf("Expected 0 replica zones initially, got %d", len(rm.replicaZones))
	}

	if rm.replicationRate != time.Second {
		t.Errorf("Expected replication rate 1s, got %v", rm.replicationRate)
	}

	if rm.isRunning {
		t.Error("Expected replication manager to not be running initially")
	}
}

func TestReplicationManager_AddAuthorityZone(t *testing.T) {
	hashRing := zone.NewHashRing(3, 10)
	storageTree := newMockStorageTree()
	rm := NewReplicationManager("test-node", hashRing, storageTree)

	replicas := []string{"replica1", "replica2", "replica3"}
	rm.AddAuthorityZone("zone1", "world.users", replicas)

	// Verify zone was added
	if len(rm.authorityZones) != 1 {
		t.Errorf("Expected 1 authority zone, got %d", len(rm.authorityZones))
	}

	zone, exists := rm.authorityZones["zone1"]
	if !exists {
		t.Error("Authority zone 'zone1' not found")
	}

	if zone.ZoneID != "zone1" {
		t.Errorf("Expected zone ID 'zone1', got '%s'", zone.ZoneID)
	}

	if zone.PathPrefix != "world.users" {
		t.Errorf("Expected path prefix 'world.users', got '%s'", zone.PathPrefix)
	}

	if len(zone.Replicas) != 3 {
		t.Errorf("Expected 3 replicas, got %d", len(zone.Replicas))
	}

	if zone.WriteCount != 0 {
		t.Errorf("Expected write count 0, got %d", zone.WriteCount)
	}

	// Verify sync status for all replicas
	if len(zone.SyncStatus) != 3 {
		t.Errorf("Expected sync status for 3 replicas, got %d", len(zone.SyncStatus))
	}

	for _, replica := range replicas {
		status, exists := zone.SyncStatus[replica]
		if !exists {
			t.Errorf("Expected sync status for replica '%s'", replica)
		} else {
			if status.NodeIdentity != replica {
				t.Errorf("Expected status identity '%s', got '%s'", replica, status.NodeIdentity)
			}
			if !status.IsHealthy {
				t.Errorf("Expected replica '%s' to be healthy initially", replica)
			}
			if status.ConsecutiveFailures != 0 {
				t.Errorf("Expected 0 consecutive failures for replica '%s', got %d", replica, status.ConsecutiveFailures)
			}
		}
	}

	// Verify pending writes list was created
	pendingWrites, exists := rm.pendingWrites["zone1"]
	if !exists {
		t.Error("Expected pending writes list for zone1")
	}

	if len(pendingWrites) != 0 {
		t.Errorf("Expected 0 pending writes initially, got %d", len(pendingWrites))
	}
}

func TestReplicationManager_AddReplicaZone(t *testing.T) {
	hashRing := zone.NewHashRing(3, 10)
	storageTree := newMockStorageTree()
	rm := NewReplicationManager("test-node", hashRing, storageTree)

	rm.AddReplicaZone("zone1", "world.products", "authority-node")

	// Verify zone was added
	if len(rm.replicaZones) != 1 {
		t.Errorf("Expected 1 replica zone, got %d", len(rm.replicaZones))
	}

	zone, exists := rm.replicaZones["zone1"]
	if !exists {
		t.Error("Replica zone 'zone1' not found")
	}

	if zone.ZoneID != "zone1" {
		t.Errorf("Expected zone ID 'zone1', got '%s'", zone.ZoneID)
	}

	if zone.PathPrefix != "world.products" {
		t.Errorf("Expected path prefix 'world.products', got '%s'", zone.PathPrefix)
	}

	if zone.Authority != "authority-node" {
		t.Errorf("Expected authority 'authority-node', got '%s'", zone.Authority)
	}

	if zone.DataSize != 0 {
		t.Errorf("Expected data size 0, got %d", zone.DataSize)
	}

	if zone.IsCurrent {
		t.Error("Expected replica to not be current initially")
	}
}

func TestReplicationManager_RemoveZone(t *testing.T) {
	hashRing := zone.NewHashRing(3, 10)
	storageTree := newMockStorageTree()
	rm := NewReplicationManager("test-node", hashRing, storageTree)

	// Add zones
	rm.AddAuthorityZone("auth-zone", "world.orders", []string{"replica1"})
	rm.AddReplicaZone("replica-zone", "world.inventory", "authority1")

	// Verify zones were added
	if len(rm.authorityZones) != 1 {
		t.Errorf("Expected 1 authority zone, got %d", len(rm.authorityZones))
	}

	if len(rm.replicaZones) != 1 {
		t.Errorf("Expected 1 replica zone, got %d", len(rm.replicaZones))
	}

	// Remove authority zone
	rm.RemoveZone("auth-zone")

	if len(rm.authorityZones) != 0 {
		t.Errorf("Expected 0 authority zones after removal, got %d", len(rm.authorityZones))
	}

	// Verify pending writes for auth-zone was removed
	if _, exists := rm.pendingWrites["auth-zone"]; exists {
		t.Error("Expected pending writes for auth-zone to be removed")
	}

	// Remove replica zone
	rm.RemoveZone("replica-zone")

	if len(rm.replicaZones) != 0 {
		t.Errorf("Expected 0 replica zones after removal, got %d", len(rm.replicaZones))
	}
}

func TestReplicationManager_RecordWrite(t *testing.T) {
	hashRing := zone.NewHashRing(3, 10)
	storageTree := newMockStorageTree()
	rm := NewReplicationManager("test-node", hashRing, storageTree)

	// Add authority zone
	rm.AddAuthorityZone("zone1", "world.users", []string{"replica1", "replica2"})

	// Create test write
	path := []string{"world", "users", "alice", "name"}
	value := storage.Value{
		TypeTag: storage.TypeText,
		Data:    []byte("Alice Smith"),
	}
	author := uint64(1001)

	// Record write
	err := rm.RecordWrite(path, value, author, "zone1")
	if err != nil {
		t.Fatalf("RecordWrite failed: %v", err)
	}

	// Verify pending write was created
	pendingWrites := rm.pendingWrites["zone1"]
	if len(pendingWrites) != 1 {
		t.Errorf("Expected 1 pending write, got %d", len(pendingWrites))
	}

	pendingWrite := pendingWrites[0]
	if len(pendingWrite.Path) != len(path) {
		t.Errorf("Expected path length %d, got %d", len(path), len(pendingWrite.Path))
	}

	for i, segment := range path {
		if pendingWrite.Path[i] != segment {
			t.Errorf("Path segment %d: expected '%s', got '%s'", i, segment, pendingWrite.Path[i])
		}
	}

	if string(pendingWrite.Value.Data) != string(value.Data) {
		t.Errorf("Expected value '%s', got '%s'", string(value.Data), string(pendingWrite.Value.Data))
	}

	if pendingWrite.Author != author {
		t.Errorf("Expected author %d, got %d", author, pendingWrite.Author)
	}

	if pendingWrite.ZoneID != "zone1" {
		t.Errorf("Expected zone ID 'zone1', got '%s'", pendingWrite.ZoneID)
	}

	if pendingWrite.Attempts != 0 {
		t.Errorf("Expected 0 attempts initially, got %d", pendingWrite.Attempts)
	}

	// Verify zone write count was incremented
	zone := rm.authorityZones["zone1"]
	if zone.WriteCount != 1 {
		t.Errorf("Expected write count 1, got %d", zone.WriteCount)
	}
}

func TestReplicationManager_RecordWrite_NotAuthority(t *testing.T) {
	hashRing := zone.NewHashRing(3, 10)
	storageTree := newMockStorageTree()
	rm := NewReplicationManager("test-node", hashRing, storageTree)

	// Try to record write for zone we're not authority for
	path := []string{"world", "users", "alice"}
	value := storage.Value{TypeTag: storage.TypeText, Data: []byte("test")}

	err := rm.RecordWrite(path, value, 1001, "unknown-zone")
	if err == nil {
		t.Error("Expected error for recording write to unknown zone, got nil")
	}
}

func TestReplicationManager_HandleReplicationRequest(t *testing.T) {
	hashRing := zone.NewHashRing(3, 10)
	storageTree := newMockStorageTree()
	rm := NewReplicationManager("test-node", hashRing, storageTree)

	// Add replica zone
	rm.AddReplicaZone("zone1", "world.users", "authority-node")

	// Create test replication data
	replicationData := &ReplicationData{
		ZoneID:     "zone1",
		PathPrefix: "world.users",
		Writes:     []*PendingWrite{},
		DataSize:   1024,
		Timestamp:  time.Now().UnixNano(),
		SourceNode: "authority-node",
	}

	// Encode data
	encodedData := rm.encodeReplicationData(replicationData)

	// Handle replication request
	err := rm.HandleReplicationRequest(encodedData)
	if err != nil {
		t.Fatalf("HandleReplicationRequest failed: %v", err)
	}

	// Verify replica zone was updated
	zone := rm.replicaZones["zone1"]
	if zone.DataSize != replicationData.DataSize {
		t.Errorf("Expected data size %d, got %d", replicationData.DataSize, zone.DataSize)
	}

	if !zone.IsCurrent {
		t.Error("Expected replica to be marked as current")
	}

	// Verify LastReceived was updated (should be recent)
	if time.Since(zone.LastReceived) > time.Second {
		t.Error("Expected LastReceived to be updated recently")
	}
}

func TestReplicationManager_HandleReplicationRequest_NotReplica(t *testing.T) {
	hashRing := zone.NewHashRing(3, 10)
	storageTree := newMockStorageTree()
	rm := NewReplicationManager("test-node", hashRing, storageTree)

	// Create replication data for zone we're not replica for
	replicationData := &ReplicationData{
		ZoneID:     "unknown-zone",
		PathPrefix: "world.unknown",
		Writes:     []*PendingWrite{},
		DataSize:   1024,
		Timestamp:  time.Now().UnixNano(),
		SourceNode: "some-node",
	}

	encodedData := rm.encodeReplicationData(replicationData)

	err := rm.HandleReplicationRequest(encodedData)
	if err == nil {
		t.Error("Expected error for replication request to unknown zone, got nil")
	}
}

func TestReplicationManager_StartStop(t *testing.T) {
	hashRing := zone.NewHashRing(3, 10)
	storageTree := newMockStorageTree()
	rm := NewReplicationManager("test-node", hashRing, storageTree)

	// Should not be running initially
	if rm.isRunning {
		t.Error("Expected replication manager to not be running initially")
	}

	// Start the manager
	rm.Start()

	// Should be running now
	if !rm.isRunning {
		t.Error("Expected replication manager to be running after Start()")
	}

	// Starting again should be safe (no-op)
	rm.Start()

	if !rm.isRunning {
		t.Error("Expected replication manager to still be running after second Start()")
	}

	// Stop the manager
	rm.Stop()

	// Should not be running now
	if rm.isRunning {
		t.Error("Expected replication manager to not be running after Stop()")
	}

	// Stopping again should be safe (no-op)
	rm.Stop()

	if rm.isRunning {
		t.Error("Expected replication manager to still be stopped after second Stop()")
	}
}

func TestReplicationManager_GetAuthorityZones(t *testing.T) {
	hashRing := zone.NewHashRing(3, 10)
	storageTree := newMockStorageTree()
	rm := NewReplicationManager("test-node", hashRing, storageTree)

	// Initially empty
	zones := rm.GetAuthorityZones()
	if len(zones) != 0 {
		t.Errorf("Expected 0 authority zones initially, got %d", len(zones))
	}

	// Add zones
	rm.AddAuthorityZone("zone1", "world.users", []string{"replica1", "replica2"})
	rm.AddAuthorityZone("zone2", "world.products", []string{"replica3"})

	zones = rm.GetAuthorityZones()
	if len(zones) != 2 {
		t.Errorf("Expected 2 authority zones, got %d", len(zones))
	}

	// Verify zone1
	zone1, exists := zones["zone1"]
	if !exists {
		t.Error("Expected zone1 in authority zones")
	} else {
		if zone1.PathPrefix != "world.users" {
			t.Errorf("Expected zone1 path prefix 'world.users', got '%s'", zone1.PathPrefix)
		}

		if len(zone1.Replicas) != 2 {
			t.Errorf("Expected 2 replicas for zone1, got %d", len(zone1.Replicas))
		}

		// Verify SyncStatus is not exposed
		if zone1.SyncStatus != nil {
			t.Error("Expected sync status to not be exposed in returned zones")
		}
	}

	// Verify zone2
	zone2, exists := zones["zone2"]
	if !exists {
		t.Error("Expected zone2 in authority zones")
	} else {
		if zone2.PathPrefix != "world.products" {
			t.Errorf("Expected zone2 path prefix 'world.products', got '%s'", zone2.PathPrefix)
		}

		if len(zone2.Replicas) != 1 {
			t.Errorf("Expected 1 replica for zone2, got %d", len(zone2.Replicas))
		}
	}
}

func TestReplicationManager_GetReplicaZones(t *testing.T) {
	hashRing := zone.NewHashRing(3, 10)
	storageTree := newMockStorageTree()
	rm := NewReplicationManager("test-node", hashRing, storageTree)

	// Initially empty
	zones := rm.GetReplicaZones()
	if len(zones) != 0 {
		t.Errorf("Expected 0 replica zones initially, got %d", len(zones))
	}

	// Add zones
	rm.AddReplicaZone("zone1", "world.orders", "authority1")
	rm.AddReplicaZone("zone2", "world.inventory", "authority2")

	zones = rm.GetReplicaZones()
	if len(zones) != 2 {
		t.Errorf("Expected 2 replica zones, got %d", len(zones))
	}

	// Verify zone1
	zone1, exists := zones["zone1"]
	if !exists {
		t.Error("Expected zone1 in replica zones")
	} else {
		if zone1.PathPrefix != "world.orders" {
			t.Errorf("Expected zone1 path prefix 'world.orders', got '%s'", zone1.PathPrefix)
		}

		if zone1.Authority != "authority1" {
			t.Errorf("Expected zone1 authority 'authority1', got '%s'", zone1.Authority)
		}
	}

	// Verify zone2
	zone2, exists := zones["zone2"]
	if !exists {
		t.Error("Expected zone2 in replica zones")
	} else {
		if zone2.PathPrefix != "world.inventory" {
			t.Errorf("Expected zone2 path prefix 'world.inventory', got '%s'", zone2.PathPrefix)
		}

		if zone2.Authority != "authority2" {
			t.Errorf("Expected zone2 authority 'authority2', got '%s'", zone2.Authority)
		}
	}
}

func TestReplicationManager_EncodeDecodeReplicationData(t *testing.T) {
	hashRing := zone.NewHashRing(3, 10)
	storageTree := newMockStorageTree()
	rm := NewReplicationManager("test-node", hashRing, storageTree)

	// Create test replication data
	originalData := &ReplicationData{
		ZoneID:     "test-zone-123",
		PathPrefix: "world.test.data",
		Writes:     []*PendingWrite{}, // Simplified for basic test
		DataSize:   2048,
		Timestamp:  time.Now().UnixNano(),
		SourceNode: "authority-node-xyz",
	}

	// Encode
	encoded := rm.encodeReplicationData(originalData)
	if len(encoded) == 0 {
		t.Error("Expected non-empty encoded data")
	}

	// Decode
	decoded, err := rm.decodeReplicationData(encoded)
	if err != nil {
		t.Fatalf("Failed to decode replication data: %v", err)
	}

	// Verify decoded data matches original
	if decoded.ZoneID != originalData.ZoneID {
		t.Errorf("Expected zone ID '%s', got '%s'", originalData.ZoneID, decoded.ZoneID)
	}

	if decoded.PathPrefix != originalData.PathPrefix {
		t.Errorf("Expected path prefix '%s', got '%s'", originalData.PathPrefix, decoded.PathPrefix)
	}

	if decoded.DataSize != originalData.DataSize {
		t.Errorf("Expected data size %d, got %d", originalData.DataSize, decoded.DataSize)
	}

	if decoded.Timestamp != originalData.Timestamp {
		t.Errorf("Expected timestamp %d, got %d", originalData.Timestamp, decoded.Timestamp)
	}

	// Note: Writes and SourceNode decoding not fully implemented in basic version
}

func TestReplicationManager_GetReplicationStats(t *testing.T) {
	hashRing := zone.NewHashRing(3, 10)
	storageTree := newMockStorageTree()
	rm := NewReplicationManager("test-node", hashRing, storageTree)

	// Initially empty stats
	stats := rm.GetReplicationStats()

	if stats["authority_zones"] != 0 {
		t.Errorf("Expected 0 authority zones, got %v", stats["authority_zones"])
	}

	if stats["replica_zones"] != 0 {
		t.Errorf("Expected 0 replica zones, got %v", stats["replica_zones"])
	}

	if stats["pending_writes"] != 0 {
		t.Errorf("Expected 0 pending writes, got %v", stats["pending_writes"])
	}

	if stats["healthy_replicas"] != 0 {
		t.Errorf("Expected 0 healthy replicas, got %v", stats["healthy_replicas"])
	}

	if stats["total_replicas"] != 0 {
		t.Errorf("Expected 0 total replicas, got %v", stats["total_replicas"])
	}

	// Add zones and writes
	rm.AddAuthorityZone("zone1", "world.users", []string{"replica1", "replica2"})
	rm.AddReplicaZone("zone2", "world.products", "authority1")

	path := []string{"world", "users", "alice"}
	value := storage.Value{TypeTag: storage.TypeText, Data: []byte("test")}
	rm.RecordWrite(path, value, 1001, "zone1")

	stats = rm.GetReplicationStats()

	if stats["authority_zones"] != 1 {
		t.Errorf("Expected 1 authority zone, got %v", stats["authority_zones"])
	}

	if stats["replica_zones"] != 1 {
		t.Errorf("Expected 1 replica zone, got %v", stats["replica_zones"])
	}

	if stats["pending_writes"] != 1 {
		t.Errorf("Expected 1 pending write, got %v", stats["pending_writes"])
	}

	if stats["healthy_replicas"] != 2 {
		t.Errorf("Expected 2 healthy replicas, got %v", stats["healthy_replicas"])
	}

	if stats["total_replicas"] != 2 {
		t.Errorf("Expected 2 total replicas, got %v", stats["total_replicas"])
	}
}
