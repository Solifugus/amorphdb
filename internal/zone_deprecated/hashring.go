// Package zone implements consistent hashing and zone assignment for AmorphDB mesh
package zone

import (
	"crypto/sha256"
	"fmt"
	"sort"
	"sync"
)

// HashRing implements consistent hashing with virtual nodes
type HashRing struct {
	mu           sync.RWMutex
	nodes        map[string]*Node        // Physical nodes by identity
	virtualNodes map[uint32]string       // Virtual node hash -> physical node identity
	sortedHashes []uint32               // Sorted virtual node hashes
	replicas     int                    // Number of replicas per zone
	vnodeCount   int                    // Virtual nodes per physical node
}

// Node represents a physical node in the mesh
type Node struct {
	Identity    string   // Node identity (CV syllable pattern)
	Address     string   // Network address
	IsOnline    bool     // Whether node is currently reachable
	ZoneCount   int      // Number of zones this node is responsible for
	LoadMetrics *LoadMetrics // Performance metrics
}

// LoadMetrics tracks node performance for load balancing
type LoadMetrics struct {
	DataSize      int64   // Total data size in bytes
	QueryRate     float64 // Queries per second
	WriteRate     float64 // Writes per second
	CPUUsage      float64 // CPU utilization percentage
	MemoryUsage   float64 // Memory utilization percentage
	LastUpdated   int64   // Timestamp of last metrics update
}

// ZoneAssignment represents a zone assignment
type ZoneAssignment struct {
	ZoneID       string   // Zone identifier (hash of path range)
	PathPrefix   string   // Path prefix for this zone
	Authority    string   // Primary node identity
	Replicas     []string // Replica node identities
	Hash         uint32   // Zone hash for ring position
}

// NewHashRing creates a new consistent hash ring
func NewHashRing(replicas, vnodeCount int) *HashRing {
	return &HashRing{
		nodes:        make(map[string]*Node),
		virtualNodes: make(map[uint32]string),
		sortedHashes: make([]uint32, 0),
		replicas:     replicas,
		vnodeCount:   vnodeCount,
	}
}

// AddNode adds a physical node to the hash ring
func (hr *HashRing) AddNode(identity, address string) error {
	hr.mu.Lock()
	defer hr.mu.Unlock()

	if _, exists := hr.nodes[identity]; exists {
		return fmt.Errorf("node %s already exists in ring", identity)
	}

	// Create physical node
	node := &Node{
		Identity:  identity,
		Address:   address,
		IsOnline:  true,
		ZoneCount: 0,
		LoadMetrics: &LoadMetrics{
			DataSize:    0,
			QueryRate:   0,
			WriteRate:   0,
			CPUUsage:    0,
			MemoryUsage: 0,
			LastUpdated: 0,
		},
	}

	hr.nodes[identity] = node

	// Add virtual nodes
	for i := 0; i < hr.vnodeCount; i++ {
		hash := hr.hashVirtualNode(identity, i)
		hr.virtualNodes[hash] = identity
		hr.sortedHashes = append(hr.sortedHashes, hash)
	}

	// Re-sort the hash array
	sort.Slice(hr.sortedHashes, func(i, j int) bool {
		return hr.sortedHashes[i] < hr.sortedHashes[j]
	})

	return nil
}

// RemoveNode removes a physical node from the hash ring
func (hr *HashRing) RemoveNode(identity string) error {
	hr.mu.Lock()
	defer hr.mu.Unlock()

	if _, exists := hr.nodes[identity]; !exists {
		return fmt.Errorf("node %s does not exist in ring", identity)
	}

	// Remove virtual nodes
	newHashes := make([]uint32, 0)
	for _, hash := range hr.sortedHashes {
		if hr.virtualNodes[hash] != identity {
			newHashes = append(newHashes, hash)
		} else {
			delete(hr.virtualNodes, hash)
		}
	}

	hr.sortedHashes = newHashes
	delete(hr.nodes, identity)

	return nil
}

// GetNodeForZone returns the authority node for a given zone path
func (hr *HashRing) GetNodeForZone(zonePath string) (string, error) {
	hr.mu.RLock()
	defer hr.mu.RUnlock()

	if len(hr.sortedHashes) == 0 {
		return "", fmt.Errorf("no nodes in hash ring")
	}

	hash := hr.hashZonePath(zonePath)

	// Find the first virtual node >= hash
	idx := sort.Search(len(hr.sortedHashes), func(i int) bool {
		return hr.sortedHashes[i] >= hash
	})

	// Wrap around if we went past the end
	if idx == len(hr.sortedHashes) {
		idx = 0
	}

	virtualHash := hr.sortedHashes[idx]
	return hr.virtualNodes[virtualHash], nil
}

// GetReplicaNodes returns the replica nodes for a given zone path
func (hr *HashRing) GetReplicaNodes(zonePath string) ([]string, error) {
	hr.mu.RLock()
	defer hr.mu.RUnlock()

	if len(hr.sortedHashes) == 0 {
		return nil, fmt.Errorf("no nodes in hash ring")
	}

	hash := hr.hashZonePath(zonePath)
	replicas := make([]string, 0, hr.replicas)
	seen := make(map[string]bool)

	// Find starting position
	idx := sort.Search(len(hr.sortedHashes), func(i int) bool {
		return hr.sortedHashes[i] >= hash
	})

	// Collect unique replicas
	for len(replicas) < hr.replicas && len(seen) < len(hr.nodes) {
		// Wrap around if we went past the end
		if idx >= len(hr.sortedHashes) {
			idx = 0
		}

		virtualHash := hr.sortedHashes[idx]
		nodeIdentity := hr.virtualNodes[virtualHash]

		// Only add if we haven't seen this physical node
		if !seen[nodeIdentity] && hr.nodes[nodeIdentity].IsOnline {
			replicas = append(replicas, nodeIdentity)
			seen[nodeIdentity] = true
		}

		idx++
	}

	return replicas, nil
}

// GetZoneAssignment returns complete zone assignment for a path
func (hr *HashRing) GetZoneAssignment(zonePath string) (*ZoneAssignment, error) {
	authority, err := hr.GetNodeForZone(zonePath)
	if err != nil {
		return nil, err
	}

	replicas, err := hr.GetReplicaNodes(zonePath)
	if err != nil {
		return nil, err
	}

	// Remove authority from replicas if present
	filteredReplicas := make([]string, 0)
	for _, replica := range replicas {
		if replica != authority {
			filteredReplicas = append(filteredReplicas, replica)
		}
	}

	// Limit to replicas count - 1 (authority is separate)
	if len(filteredReplicas) > hr.replicas-1 {
		filteredReplicas = filteredReplicas[:hr.replicas-1]
	}

	return &ZoneAssignment{
		ZoneID:     hr.generateZoneID(zonePath),
		PathPrefix: zonePath,
		Authority:  authority,
		Replicas:   filteredReplicas,
		Hash:       hr.hashZonePath(zonePath),
	}, nil
}

// GetAllNodes returns all nodes in the ring
func (hr *HashRing) GetAllNodes() map[string]*Node {
	hr.mu.RLock()
	defer hr.mu.RUnlock()

	result := make(map[string]*Node)
	for identity, node := range hr.nodes {
		// Create a copy to avoid race conditions
		result[identity] = &Node{
			Identity:    node.Identity,
			Address:     node.Address,
			IsOnline:    node.IsOnline,
			ZoneCount:   node.ZoneCount,
			LoadMetrics: node.LoadMetrics, // Note: LoadMetrics is a pointer, so this shares state
		}
	}

	return result
}

// UpdateNodeStatus updates a node's online status
func (hr *HashRing) UpdateNodeStatus(identity string, isOnline bool) error {
	hr.mu.Lock()
	defer hr.mu.Unlock()

	node, exists := hr.nodes[identity]
	if !exists {
		return fmt.Errorf("node %s not found", identity)
	}

	node.IsOnline = isOnline
	return nil
}

// UpdateNodeMetrics updates a node's load metrics
func (hr *HashRing) UpdateNodeMetrics(identity string, metrics *LoadMetrics) error {
	hr.mu.Lock()
	defer hr.mu.Unlock()

	node, exists := hr.nodes[identity]
	if !exists {
		return fmt.Errorf("node %s not found", identity)
	}

	node.LoadMetrics = metrics
	return nil
}

// GetLoadBalancedNode returns the least loaded node from a set of candidates
func (hr *HashRing) GetLoadBalancedNode(candidates []string) (string, error) {
	hr.mu.RLock()
	defer hr.mu.RUnlock()

	if len(candidates) == 0 {
		return "", fmt.Errorf("no candidate nodes provided")
	}

	var bestNode string
	var lowestLoad float64 = -1

	for _, candidate := range candidates {
		node, exists := hr.nodes[candidate]
		if !exists || !node.IsOnline {
			continue
		}

		// Calculate composite load score
		load := hr.calculateLoadScore(node.LoadMetrics)

		if lowestLoad < 0 || load < lowestLoad {
			lowestLoad = load
			bestNode = candidate
		}
	}

	if bestNode == "" {
		return "", fmt.Errorf("no online nodes available from candidates")
	}

	return bestNode, nil
}

// calculateLoadScore computes a composite load score for a node
func (hr *HashRing) calculateLoadScore(metrics *LoadMetrics) float64 {
	// Weighted combination of various metrics
	// Higher scores indicate higher load
	cpuWeight := 0.3
	memoryWeight := 0.3
	queryWeight := 0.2
	writeWeight := 0.2

	return cpuWeight*metrics.CPUUsage +
		memoryWeight*metrics.MemoryUsage +
		queryWeight*(metrics.QueryRate/100) + // Normalize query rate
		writeWeight*(metrics.WriteRate/100)   // Normalize write rate
}

// hashVirtualNode generates a hash for a virtual node
func (hr *HashRing) hashVirtualNode(nodeIdentity string, virtualIndex int) uint32 {
	h := sha256.New()
	h.Write([]byte(fmt.Sprintf("%s:%d", nodeIdentity, virtualIndex)))
	hash := h.Sum(nil)

	// Convert first 4 bytes to uint32
	return uint32(hash[0])<<24 + uint32(hash[1])<<16 + uint32(hash[2])<<8 + uint32(hash[3])
}

// hashZonePath generates a hash for a zone path
func (hr *HashRing) hashZonePath(zonePath string) uint32 {
	h := sha256.New()
	h.Write([]byte(zonePath))
	hash := h.Sum(nil)

	// Convert first 4 bytes to uint32
	return uint32(hash[0])<<24 + uint32(hash[1])<<16 + uint32(hash[2])<<8 + uint32(hash[3])
}

// generateZoneID creates a unique zone identifier
func (hr *HashRing) generateZoneID(zonePath string) string {
	h := sha256.New()
	h.Write([]byte(zonePath))
	hash := h.Sum(nil)

	// Return first 8 bytes as hex string
	return fmt.Sprintf("%x", hash[:8])
}

// GetRingSize returns the number of physical nodes in the ring
func (hr *HashRing) GetRingSize() int {
	hr.mu.RLock()
	defer hr.mu.RUnlock()
	return len(hr.nodes)
}

// GetVirtualNodeCount returns the total number of virtual nodes
func (hr *HashRing) GetVirtualNodeCount() int {
	hr.mu.RLock()
	defer hr.mu.RUnlock()
	return len(hr.virtualNodes)
}

// GetReplicaCount returns the configured replica count
func (hr *HashRing) GetReplicaCount() int {
	return hr.replicas
}
