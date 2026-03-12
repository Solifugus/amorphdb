package mesh

import (
	"testing"
	"time"
)

func TestBootstrapManager_SelfGenesis(t *testing.T) {
	bm := NewBootstrapManager()

	err := bm.SelfGenesis()
	if err != nil {
		t.Fatalf("SelfGenesis failed: %v", err)
	}

	// Verify identity was generated
	identity := bm.GetNodeIdentity()
	if identity == "" {
		t.Error("Expected non-empty node identity")
	}

	// Verify identity follows CV syllable pattern
	if !ValidateIdentity(identity) {
		t.Errorf("Generated identity '%s' does not follow CV syllable pattern", identity)
	}

	// Verify this is marked as first node
	if !bm.IsFirstNode() {
		t.Error("Expected first node flag to be true")
	}

	// Verify key pair was generated
	if bm.keyPair == nil {
		t.Error("Expected key pair to be generated")
	}

	if len(bm.keyPair.PublicKey) != 32 {
		t.Errorf("Expected public key length 32, got %d", len(bm.keyPair.PublicKey))
	}

	if len(bm.keyPair.PrivateKey) != 32 {
		t.Errorf("Expected private key length 32, got %d", len(bm.keyPair.PrivateKey))
	}
}

func TestBootstrapManager_GetKnownNodes(t *testing.T) {
	bm := NewBootstrapManager()

	// Initially should have no known nodes
	nodes := bm.GetKnownNodes()
	if len(nodes) != 0 {
		t.Errorf("Expected 0 known nodes, got %d", len(nodes))
	}

	// Add a test node
	testNode := &NodeInfo{
		Identity:  "test-node-ka-ve-lo",
		Address:   "192.168.1.100:5000",
		PublicKey: []byte("test-public-key"),
		LastSeen:  time.Now(),
		IsOnline:  true,
	}

	bm.knownNodes["test-node-ka-ve-lo"] = testNode

	// Verify we can retrieve it
	nodes = bm.GetKnownNodes()
	if len(nodes) != 1 {
		t.Errorf("Expected 1 known node, got %d", len(nodes))
	}

	retrievedNode, exists := nodes["test-node-ka-ve-lo"]
	if !exists {
		t.Error("Expected to find test node in known nodes")
	}

	if retrievedNode.Identity != testNode.Identity {
		t.Errorf("Expected identity '%s', got '%s'", testNode.Identity, retrievedNode.Identity)
	}

	if retrievedNode.Address != testNode.Address {
		t.Errorf("Expected address '%s', got '%s'", testNode.Address, retrievedNode.Address)
	}
}

func TestBootstrapManager_ParsePeerList(t *testing.T) {
	bm := NewBootstrapManager()

	// Create test peer list payload
	// Format: identity_length + identity + address_length + address
	payload := []byte{}

	// First peer: "ka-ve-lo" at "192.168.1.100:5000"
	identity1 := "ka-ve-lo"
	address1 := "192.168.1.100:5000"
	payload = append(payload, byte(len(identity1)))
	payload = append(payload, []byte(identity1)...)
	payload = append(payload, byte(len(address1)))
	payload = append(payload, []byte(address1)...)

	// Second peer: "ma-li-no" at "192.168.1.101:5000"
	identity2 := "ma-li-no"
	address2 := "192.168.1.101:5000"
	payload = append(payload, byte(len(identity2)))
	payload = append(payload, []byte(identity2)...)
	payload = append(payload, byte(len(address2)))
	payload = append(payload, []byte(address2)...)

	err := bm.parsePeerList(payload)
	if err != nil {
		t.Fatalf("parsePeerList failed: %v", err)
	}

	// Verify both peers were added
	nodes := bm.GetKnownNodes()
	if len(nodes) != 2 {
		t.Errorf("Expected 2 known nodes, got %d", len(nodes))
	}

	// Check first peer
	node1, exists := nodes[identity1]
	if !exists {
		t.Errorf("Expected to find node '%s'", identity1)
	} else {
		if node1.Address != address1 {
			t.Errorf("Expected address '%s', got '%s'", address1, node1.Address)
		}
		if !node1.IsOnline {
			t.Error("Expected node to be marked as online")
		}
	}

	// Check second peer
	node2, exists := nodes[identity2]
	if !exists {
		t.Errorf("Expected to find node '%s'", identity2)
	} else {
		if node2.Address != address2 {
			t.Errorf("Expected address '%s', got '%s'", address2, node2.Address)
		}
		if !node2.IsOnline {
			t.Error("Expected node to be marked as online")
		}
	}
}

func TestBootstrapManager_ParsePeerList_InvalidFormat(t *testing.T) {
	bm := NewBootstrapManager()

	// Test with truncated payload
	payload := []byte{5, 'h', 'e', 'l', 'l'} // Missing 'o' and address

	err := bm.parsePeerList(payload)
	if err == nil {
		t.Error("Expected error for invalid peer list format")
	}

	// Test with address length overflow
	payload = []byte{5, 'h', 'e', 'l', 'l', 'o', 20} // Address length 20 but no more data

	err = bm.parsePeerList(payload)
	if err == nil {
		t.Error("Expected error for address overflow")
	}
}

func TestNewBootstrapManager(t *testing.T) {
	bm := NewBootstrapManager()

	if bm == nil {
		t.Fatal("Expected non-nil bootstrap manager")
	}

	if bm.knownNodes == nil {
		t.Error("Expected initialized knownNodes map")
	}

	if bm.nodeIdentity != "" {
		t.Error("Expected empty node identity initially")
	}

	if bm.isFirstNode {
		t.Error("Expected first node flag to be false initially")
	}
}

func TestBootstrapManager_NodeIdentityGeneration(t *testing.T) {
	// Test multiple identity generations to ensure uniqueness
	identities := make(map[string]bool)

	for i := 0; i < 10; i++ {
		bm := NewBootstrapManager()
		err := bm.SelfGenesis()
		if err != nil {
			t.Fatalf("SelfGenesis failed on iteration %d: %v", i, err)
		}

		identity := bm.GetNodeIdentity()

		// Check for duplicates (very unlikely but possible)
		if identities[identity] {
			t.Logf("Warning: Duplicate identity generated: %s", identity)
		}
		identities[identity] = true

		// Verify identity format
		if !ValidateIdentity(identity) {
			t.Errorf("Invalid identity generated: %s", identity)
		}
	}

	// Should have generated at least some unique identities
	if len(identities) < 5 {
		t.Errorf("Expected at least 5 unique identities, got %d", len(identities))
	}
}
