package mesh

import "fmt"
import (
	"testing"
	"time"
)

func TestNewHeartbeatManager(t *testing.T) {
	hm := NewHeartbeatManager("test-node-ka-ve-lo")

	if hm.nodeIdentity != "test-node-ka-ve-lo" {
		t.Errorf("Expected nodeIdentity 'test-node-ka-ve-lo', got '%s'", hm.nodeIdentity)
	}

	if len(hm.peers) != 0 {
		t.Errorf("Expected 0 peers initially, got %d", len(hm.peers))
	}

	if hm.tickInterval != time.Millisecond*333 {
		t.Errorf("Expected tick interval 333ms, got %v", hm.tickInterval)
	}

	if hm.failureTimeout != time.Second*3 {
		t.Errorf("Expected failure timeout 3s, got %v", hm.failureTimeout)
	}

	if hm.isRunning {
		t.Error("Expected heartbeat manager to not be running initially")
	}
}

func TestHeartbeatManager_AddPeer(t *testing.T) {
	hm := NewHeartbeatManager("test-node")

	err := hm.AddPeer("peer1", "192.168.1.100:5000")
	if err != nil {
		t.Fatalf("AddPeer failed: %v", err)
	}

	if len(hm.peers) != 1 {
		t.Errorf("Expected 1 peer, got %d", len(hm.peers))
	}

	peer, exists := hm.peers["peer1"]
	if !exists {
		t.Error("Peer 'peer1' not found")
	}

	if peer.Address != "192.168.1.100:5000" {
		t.Errorf("Expected address '192.168.1.100:5000', got '%s'", peer.Address)
	}

	if peer.IsOnline {
		t.Error("Expected peer to be offline initially")
	}

	if peer.FailureCount != 0 {
		t.Errorf("Expected failure count 0, got %d", peer.FailureCount)
	}

	// Test duplicate peer
	err = hm.AddPeer("peer1", "192.168.1.101:5000")
	if err == nil {
		t.Error("Expected error for duplicate peer, got nil")
	}
}

func TestHeartbeatManager_RemovePeer(t *testing.T) {
	hm := NewHeartbeatManager("test-node")

	// Add a peer first
	err := hm.AddPeer("peer1", "192.168.1.100:5000")
	if err != nil {
		t.Fatalf("AddPeer failed: %v", err)
	}

	// Verify it was added
	if len(hm.peers) != 1 {
		t.Errorf("Expected 1 peer after add, got %d", len(hm.peers))
	}

	// Remove the peer
	hm.RemovePeer("peer1")

	// Verify it was removed
	if len(hm.peers) != 0 {
		t.Errorf("Expected 0 peers after remove, got %d", len(hm.peers))
	}

	// Test removing non-existent peer (should not panic)
	hm.RemovePeer("nonexistent")
}

func TestHeartbeatManager_GetPeerStatus(t *testing.T) {
	hm := NewHeartbeatManager("test-node")

	// Test with no peers
	status := hm.GetPeerStatus()
	if len(status) != 0 {
		t.Errorf("Expected empty status map, got %d entries", len(status))
	}

	// Add some peers
	hm.AddPeer("peer1", "192.168.1.100:5000")
	hm.AddPeer("peer2", "192.168.1.101:5000")

	status = hm.GetPeerStatus()
	if len(status) != 2 {
		t.Errorf("Expected 2 peers in status, got %d", len(status))
	}

	// Verify peer1 status
	peer1Status, exists := status["peer1"]
	if !exists {
		t.Error("Expected peer1 in status")
	} else {
		if peer1Status.Identity != "peer1" {
			t.Errorf("Expected identity 'peer1', got '%s'", peer1Status.Identity)
		}
		if peer1Status.Address != "192.168.1.100:5000" {
			t.Errorf("Expected address '192.168.1.100:5000', got '%s'", peer1Status.Address)
		}
		// Connection should be nil in status (not exposed)
		if peer1Status.Conn != nil {
			t.Error("Expected connection to be nil in status")
		}
	}
}

func TestHeartbeatManager_StartStop(t *testing.T) {
	hm := NewHeartbeatManager("test-node")

	// Should not be running initially
	if hm.isRunning {
		t.Error("Expected heartbeat manager to not be running initially")
	}

	// Start the manager
	hm.Start()

	// Should be running now
	if !hm.isRunning {
		t.Error("Expected heartbeat manager to be running after Start()")
	}

	// Starting again should be safe (no-op)
	hm.Start()

	if !hm.isRunning {
		t.Error("Expected heartbeat manager to still be running after second Start()")
	}

	// Stop the manager
	hm.Stop()

	// Should not be running now
	if hm.isRunning {
		t.Error("Expected heartbeat manager to not be running after Stop()")
	}

	// Stopping again should be safe (no-op)
	hm.Stop()

	if hm.isRunning {
		t.Error("Expected heartbeat manager to still be stopped after second Stop()")
	}
}

func TestHeartbeatManager_GetPeerCount(t *testing.T) {
	hm := NewHeartbeatManager("test-node")

	// Initially no peers
	total, online := hm.GetPeerCount()
	if total != 0 || online != 0 {
		t.Errorf("Expected 0 total and 0 online, got %d total, %d online", total, online)
	}

	// Add some peers
	hm.AddPeer("peer1", "192.168.1.100:5000")
	hm.AddPeer("peer2", "192.168.1.101:5000")

	total, online = hm.GetPeerCount()
	if total != 2 || online != 0 {
		t.Errorf("Expected 2 total and 0 online, got %d total, %d online", total, online)
	}

	// Mark one peer as online
	hm.mu.Lock()
	hm.peers["peer1"].IsOnline = true
	hm.mu.Unlock()

	total, online = hm.GetPeerCount()
	if total != 2 || online != 1 {
		t.Errorf("Expected 2 total and 1 online, got %d total, %d online", total, online)
	}
}

func TestHeartbeatManager_GetOnlinePeers(t *testing.T) {
	hm := NewHeartbeatManager("test-node")

	// No peers initially
	onlinePeers := hm.GetOnlinePeers()
	if len(onlinePeers) != 0 {
		t.Errorf("Expected 0 online peers, got %d", len(onlinePeers))
	}

	// Add peers
	hm.AddPeer("peer1", "192.168.1.100:5000")
	hm.AddPeer("peer2", "192.168.1.101:5000")
	hm.AddPeer("peer3", "192.168.1.102:5000")

	// All should be offline initially
	onlinePeers = hm.GetOnlinePeers()
	if len(onlinePeers) != 0 {
		t.Errorf("Expected 0 online peers initially, got %d", len(onlinePeers))
	}

	// Mark some as online
	hm.mu.Lock()
	hm.peers["peer1"].IsOnline = true
	hm.peers["peer3"].IsOnline = true
	hm.mu.Unlock()

	onlinePeers = hm.GetOnlinePeers()
	if len(onlinePeers) != 2 {
		t.Errorf("Expected 2 online peers, got %d", len(onlinePeers))
	}

	// Check that the correct peers are listed as online
	onlineMap := make(map[string]bool)
	for _, peer := range onlinePeers {
		onlineMap[peer] = true
	}

	if !onlineMap["peer1"] || !onlineMap["peer3"] {
		t.Errorf("Expected peer1 and peer3 to be online, got %v", onlinePeers)
	}

	if onlineMap["peer2"] {
		t.Error("Expected peer2 to be offline")
	}
}

func TestHeartbeatManager_UpdateZoneData(t *testing.T) {
	hm := NewHeartbeatManager("test-node")

	// Add a peer
	hm.AddPeer("peer1", "192.168.1.100:5000")

	testData := []byte("test zone data")
	hm.UpdateZoneData("peer1", testData)

	// Verify zone data was updated
	hm.mu.RLock()
	peer := hm.peers["peer1"]
	hm.mu.RUnlock()

	if string(peer.ZoneData) != string(testData) {
		t.Errorf("Expected zone data '%s', got '%s'", string(testData), string(peer.ZoneData))
	}

	// Test updating non-existent peer (should not panic)
	hm.UpdateZoneData("nonexistent", []byte("test"))
}

func TestHeartbeatManager_SetCallbacks(t *testing.T) {
	hm := NewHeartbeatManager("test-node")

	failedNodes := make([]string, 0)
	recoveredNodes := make([]string, 0)

	failureCallback := func(identity string) {
		failedNodes = append(failedNodes, identity)
	}

	recoveryCallback := func(identity string) {
		recoveredNodes = append(recoveredNodes, identity)
	}

	hm.SetFailureCallback(failureCallback)
	hm.SetRecoveryCallback(recoveryCallback)

	// Verify callbacks were set
	hm.mu.RLock()
	hasFailureCallback := hm.onNodeFailed != nil
	hasRecoveryCallback := hm.onNodeRecovered != nil
	hm.mu.RUnlock()

	if !hasFailureCallback {
		t.Error("Expected failure callback to be set")
	}

	if !hasRecoveryCallback {
		t.Error("Expected recovery callback to be set")
	}
}

func TestHeartbeatManager_ClockSynchronization(t *testing.T) {
	hm := NewHeartbeatManager("test-node")

	// Initially no clock offset
	offset := hm.GetClockOffset()
	if offset != 0 {
		t.Errorf("Expected initial clock offset 0, got %d", offset)
	}

	syncTime := hm.GetSynchronizedTime()
	if syncTime.IsZero() {
		t.Error("Expected non-zero synchronized time")
	}

	// Test that synchronized time is close to current time (within 1 second)
	now := time.Now()
	diff := syncTime.Sub(now)
	if diff > time.Second || diff < -time.Second {
		t.Errorf("Synchronized time differs too much from current time: %v", diff)
	}
}

func TestHeartbeatManager_EncodeHeartbeatPayload(t *testing.T) {
	hm := NewHeartbeatManager("test-node")

	timestamp := time.Now()
	zoneData := []byte("test zone data")

	payload := hm.encodeHeartbeatPayload(timestamp, zoneData)

	// Verify payload structure
	expectedMinSize := 12 + len(zoneData) // 8 bytes timestamp + 4 bytes length + data
	if len(payload) < expectedMinSize {
		t.Errorf("Expected payload size >= %d, got %d", expectedMinSize, len(payload))
	}

	// Verify timestamp encoding (first 8 bytes)
	encodedTimestamp := int64(payload[0])<<56 | int64(payload[1])<<48 |
		int64(payload[2])<<40 | int64(payload[3])<<32 |
		int64(payload[4])<<24 | int64(payload[5])<<16 |
		int64(payload[6])<<8 | int64(payload[7])

	if encodedTimestamp != timestamp.UnixNano() {
		t.Errorf("Expected timestamp %d, got %d", timestamp.UnixNano(), encodedTimestamp)
	}

	// Verify zone data length encoding (bytes 8-11)
	encodedLength := uint32(payload[8])<<24 | uint32(payload[9])<<16 |
		uint32(payload[10])<<8 | uint32(payload[11])

	if encodedLength != uint32(len(zoneData)) {
		t.Errorf("Expected zone data length %d, got %d", len(zoneData), encodedLength)
	}

	// Verify zone data (bytes 12+)
	encodedZoneData := payload[12:]
	if string(encodedZoneData) != string(zoneData) {
		t.Errorf("Expected zone data '%s', got '%s'", string(zoneData), string(encodedZoneData))
	}
}

func TestHeartbeatManager_FailureDetection(t *testing.T) {
	hm := NewHeartbeatManager("test-node")

	// Set a short failure timeout for testing
	hm.failureTimeout = time.Millisecond * 100

	// Add a peer and mark it online
	hm.AddPeer("peer1", "192.168.1.100:5000")
	hm.mu.Lock()
	hm.peers["peer1"].IsOnline = true
	hm.peers["peer1"].LastResponse = time.Now().Add(-time.Millisecond * 50) // Recent response
	hm.mu.Unlock()

	// Check for failures - should still be online
	hm.checkForFailures(time.Now())

	hm.mu.RLock()
	isOnline := hm.peers["peer1"].IsOnline
	hm.mu.RUnlock()

	if !isOnline {
		t.Error("Expected peer to still be online with recent response")
	}

	// Set old response time to trigger failure
	hm.mu.Lock()
	hm.peers["peer1"].LastResponse = time.Now().Add(-time.Millisecond * 200) // Old response
	hm.mu.Unlock()

	// Check for failures - should be marked as failed
	hm.checkForFailures(time.Now())

	hm.mu.RLock()
	isOnline = hm.peers["peer1"].IsOnline
	hm.mu.RUnlock()

	if isOnline {
		t.Error("Expected peer to be marked offline due to timeout")
	}
}

func TestHeartbeatManager_ConcurrentAccess(t *testing.T) {
	hm := NewHeartbeatManager("test-node")

	// Test concurrent access to peer management
	done := make(chan bool)

	// Goroutine 1: Add peers
	go func() {
		for i := 0; i < 10; i++ {
			hm.AddPeer(fmt.Sprintf("peer%d", i), fmt.Sprintf("192.168.1.%d:5000", 100+i))
		}
		done <- true
	}()

	// Goroutine 2: Get status
	go func() {
		for i := 0; i < 10; i++ {
			_ = hm.GetPeerStatus()
			time.Sleep(time.Millisecond)
		}
		done <- true
	}()

	// Goroutine 3: Get peer count
	go func() {
		for i := 0; i < 10; i++ {
			_, _ = hm.GetPeerCount()
			time.Sleep(time.Millisecond)
		}
		done <- true
	}()

	// Wait for all goroutines to complete
	for i := 0; i < 3; i++ {
		<-done
	}

	// Verify final state
	total, _ := hm.GetPeerCount()
	if total != 10 {
		t.Errorf("Expected 10 peers after concurrent access, got %d", total)
	}
}
