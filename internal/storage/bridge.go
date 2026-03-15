// Package storage provides bridge data storage functionality for AmorphDB
package storage

import (
	"fmt"
	"strings"

	"github.com/solifugus/amorphdb/internal/identity"
)

// BridgeStorage handles data access across mesh bridges
type BridgeStorage struct {
	localStorage Tree                             // Local storage tree
	bridges      map[string]*BridgeConnection    // Active bridge connections
	bridgeCache  map[string]map[string]Value     // Cached bridge data
}

// BridgeConnection represents a connection to another mesh for data access
type BridgeConnection struct {
	MeshName      string              // Target mesh name
	Identity      *identity.Identity  // Bridge identity in target mesh
	Status        string              // Connection status
	TargetAddress string              // Target mesh address
}

// NewBridgeStorage creates a new bridge storage manager
func NewBridgeStorage(localStorage Tree) *BridgeStorage {
	return &BridgeStorage{
		localStorage: localStorage,
		bridges:      make(map[string]*BridgeConnection),
		bridgeCache:  make(map[string]map[string]Value),
	}
}

// RegisterBridge registers a bridge connection for data access
func (bs *BridgeStorage) RegisterBridge(meshName string, connection *BridgeConnection) error {
	if meshName == "" {
		return fmt.Errorf("mesh name cannot be empty")
	}

	if connection == nil {
		return fmt.Errorf("bridge connection cannot be nil")
	}

	// Validate bridge identity
	if connection.Identity == nil || connection.Identity.ID == "" {
		return fmt.Errorf("bridge must have valid identity")
	}

	// Register the bridge connection
	bs.bridges[meshName] = connection

	// Initialize cache for this bridge
	if bs.bridgeCache[meshName] == nil {
		bs.bridgeCache[meshName] = make(map[string]Value)
	}

	return nil
}

// RemoveBridge removes a bridge connection
func (bs *BridgeStorage) RemoveBridge(meshName string) error {
	if _, exists := bs.bridges[meshName]; !exists {
		return fmt.Errorf("no bridge connection to mesh '%s'", meshName)
	}

	// Clean up bridge and cache
	delete(bs.bridges, meshName)
	delete(bs.bridgeCache, meshName)

	return nil
}

// ResolveBridgePath converts a local bridge path to the actual storage path
func (bs *BridgeStorage) ResolveBridgePath(path []string) ([]string, string, error) {
	if len(path) < 2 {
		return nil, "", fmt.Errorf("bridge path too short: %v", path)
	}

	// Check if this is a bridge path: my.<meshname>.*
	if path[0] != "my" {
		return nil, "", fmt.Errorf("not a bridge path: %v", path)
	}

	meshName := path[1]

	// Validate bridge exists
	bridge, exists := bs.bridges[meshName]
	if !exists {
		return nil, "", fmt.Errorf("no bridge to mesh '%s'", meshName)
	}

	if bridge.Status != "connected" {
		return nil, "", fmt.Errorf("bridge to mesh '%s' is not connected (status: %s)", meshName, bridge.Status)
	}

	// Convert my.meshname.* -> world.agent.{bridge_identity}.*
	bridgePath := []string{"world", "agent", bridge.Identity.ID}
	if len(path) > 2 {
		bridgePath = append(bridgePath, path[2:]...)
	}

	return bridgePath, meshName, nil
}

// ReadBridgeData reads data from a bridge path
func (bs *BridgeStorage) ReadBridgeData(path []string) (Value, error) {
	// Resolve bridge path
	bridgePath, meshName, err := bs.ResolveBridgePath(path)
	if err != nil {
		return Value{}, err
	}

	// Check cache first
	cacheKey := strings.Join(bridgePath, ".")
	if cachedValue, exists := bs.bridgeCache[meshName][cacheKey]; exists {
		return cachedValue, nil
	}

	// For now, we'll simulate bridge data access by reading from local storage
	// In a full implementation, this would make network calls to the target mesh
	value, err := bs.localStorage.Read(bridgePath)
	if err != nil {
		return Value{}, fmt.Errorf("failed to read bridge data: %w", err)
	}

	// Cache the result
	bs.bridgeCache[meshName][cacheKey] = value

	return value, nil
}

// WriteBridgeData writes data to a bridge path
func (bs *BridgeStorage) WriteBridgeData(path []string, value Value, author uint64) error {
	// Resolve bridge path
	bridgePath, meshName, err := bs.ResolveBridgePath(path)
	if err != nil {
		return err
	}

	// For now, we'll simulate bridge data access by writing to local storage
	// In a full implementation, this would make network calls to the target mesh
	err = bs.localStorage.Write(bridgePath, value, author)
	if err != nil {
		return fmt.Errorf("failed to write bridge data: %w", err)
	}

	// Update cache
	cacheKey := strings.Join(bridgePath, ".")
	bs.bridgeCache[meshName][cacheKey] = value

	return nil
}

// PurgeBridgeData purges data from a bridge path
func (bs *BridgeStorage) PurgeBridgeData(path []string, from, to int64, author uint64) error {
	// Resolve bridge path
	bridgePath, meshName, err := bs.ResolveBridgePath(path)
	if err != nil {
		return err
	}

	// For now, we'll simulate bridge data access by purging from local storage
	// In a full implementation, this would make network calls to the target mesh
	err = bs.localStorage.Purge(bridgePath, from, to, author)
	if err != nil {
		return fmt.Errorf("failed to purge bridge data: %w", err)
	}

	// Clear cache for this path
	cacheKey := strings.Join(bridgePath, ".")
	delete(bs.bridgeCache[meshName], cacheKey)

	return nil
}

// GetBridgeChildren returns children of a bridge path
func (bs *BridgeStorage) GetBridgeChildren(path []string) ([]string, error) {
	// Resolve bridge path
	bridgePath, _, err := bs.ResolveBridgePath(path)
	if err != nil {
		return nil, err
	}

	// For now, we'll simulate bridge data access by reading from local storage
	// In a full implementation, this would make network calls to the target mesh
	attributes, err := bs.localStorage.Children(bridgePath)
	if err != nil {
		return nil, fmt.Errorf("failed to get bridge children: %w", err)
	}

	// For now, return placeholder child names since we don't have access to the value store
	// In a full implementation, we would look up the actual attribute names from the values
	childNames := make([]string, len(attributes))
	for i, attr := range attributes {
		// Use attribute ID as placeholder name since we don't have access to the label value
		childNames[i] = fmt.Sprintf("attr_%d", attr.ID)
	}

	return childNames, nil
}

// IsBridgePath checks if a path is a bridge path (my.<meshname>.*)
func (bs *BridgeStorage) IsBridgePath(path []string) bool {
	if len(path) < 2 {
		return false
	}

	if path[0] != "my" {
		return false
	}

	meshName := path[1]
	_, exists := bs.bridges[meshName]
	return exists
}

// GetRegisteredBridges returns the list of registered bridge mesh names
func (bs *BridgeStorage) GetRegisteredBridges() []string {
	bridges := make([]string, 0, len(bs.bridges))
	for meshName := range bs.bridges {
		bridges = append(bridges, meshName)
	}
	return bridges
}

// GetBridgeStatus returns the status of a specific bridge
func (bs *BridgeStorage) GetBridgeStatus(meshName string) (string, error) {
	bridge, exists := bs.bridges[meshName]
	if !exists {
		return "", fmt.Errorf("no bridge to mesh '%s'", meshName)
	}

	return bridge.Status, nil
}

// ValidateBridgeAccess validates that a bridge connection can be used for data access
func (bs *BridgeStorage) ValidateBridgeAccess(meshName string) error {
	bridge, exists := bs.bridges[meshName]
	if !exists {
		return fmt.Errorf("no bridge connection to mesh '%s' exists", meshName)
	}

	if bridge.Status != "connected" {
		return fmt.Errorf("bridge to mesh '%s' is not connected (status: %s)", meshName, bridge.Status)
	}

	if bridge.Identity == nil || bridge.Identity.ID == "" {
		return fmt.Errorf("bridge to mesh '%s' has invalid identity", meshName)
	}

	return nil
}

// ClearBridgeCache clears the cache for a specific bridge or all bridges
func (bs *BridgeStorage) ClearBridgeCache(meshName string) {
	if meshName == "" {
		// Clear all caches
		for mesh := range bs.bridgeCache {
			bs.bridgeCache[mesh] = make(map[string]Value)
		}
	} else if cache, exists := bs.bridgeCache[meshName]; exists {
		// Clear specific bridge cache
		for key := range cache {
			delete(cache, key)
		}
	}
}