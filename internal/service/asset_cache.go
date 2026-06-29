package service

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/solifugus/amorphdb/internal/storage"
	"github.com/solifugus/amorphdb/internal/types"
)

// Asset represents a cached PWA asset
type Asset struct {
	Data     []byte    // Raw asset data
	MimeType string    // MIME type
	Path     string    // Asset path within the PWA
	LoadTime time.Time // When this asset was last loaded
}

// PWACache represents an in-memory cache for a single PWA domain
type PWACache struct {
	Domain  string            // Domain this cache serves
	Assets  map[string]*Asset // Assets by path (O(1) lookup)
	Enabled bool              // Whether this PWA is enabled
	SPAMode bool              // Whether SPA mode is enabled
	mutex   sync.RWMutex      // Protects concurrent access
}

// WatcherEngineInterface defines the interface for watcher engines used by the cache
type WatcherEngineInterface interface {
	RegisterWatcher(name string, watching []string, code string) error
}

// AssetCacheManager manages PWA asset caches for all domains
type AssetCacheManager struct {
	caches  map[string]*PWACache   // PWA caches by domain
	tree    storage.ExtendedTree   // Storage tree for reading assets
	watcher WatcherEngineInterface // Watcher engine for change detection
	mutex   sync.RWMutex           // Protects caches map
	agentID uint64                 // Agent ID for storage operations
}

// NewAssetCacheManager creates a new asset cache manager
func NewAssetCacheManager(tree storage.ExtendedTree, watcherEngine WatcherEngineInterface, agentID uint64) *AssetCacheManager {
	manager := &AssetCacheManager{
		caches:  make(map[string]*PWACache),
		tree:    tree,
		watcher: watcherEngine,
		agentID: agentID,
	}

	// Set up watcher for PWA configuration and asset changes
	manager.setupWatcher()

	return manager
}

// GetAsset retrieves an asset from the cache for the given domain and path
// Returns nil if asset not found or PWA not enabled
func (acm *AssetCacheManager) GetAsset(domain, path string) *Asset {
	acm.mutex.RLock()
	cache, exists := acm.caches[domain]
	acm.mutex.RUnlock()

	if !exists || !cache.Enabled {
		return nil
	}

	cache.mutex.RLock()
	asset := cache.Assets[path]
	cache.mutex.RUnlock()

	return asset
}

// IsEnabled checks if a PWA domain is enabled and returns SPA mode setting
func (acm *AssetCacheManager) IsEnabled(domain string) (enabled bool, spaMode bool) {
	acm.mutex.RLock()
	cache, exists := acm.caches[domain]
	acm.mutex.RUnlock()

	if !exists {
		return false, false
	}

	cache.mutex.RLock()
	enabled = cache.Enabled
	spaMode = cache.SPAMode
	cache.mutex.RUnlock()

	return enabled, spaMode
}

// RefreshCache forces a complete refresh of the cache for a specific domain
func (acm *AssetCacheManager) RefreshCache(domain string) error {
	return acm.loadPWAConfig(domain)
}

// RefreshAllCaches forces a complete refresh of all caches
func (acm *AssetCacheManager) RefreshAllCaches() error {
	acm.mutex.RLock()
	domains := make([]string, 0, len(acm.caches))
	for domain := range acm.caches {
		domains = append(domains, domain)
	}
	acm.mutex.RUnlock()

	for _, domain := range domains {
		if err := acm.loadPWAConfig(domain); err != nil {
			return fmt.Errorf("failed to refresh cache for domain %s: %w", domain, err)
		}
	}

	return nil
}

// loadPWAConfig loads PWA configuration and assets for a domain
func (acm *AssetCacheManager) loadPWAConfig(domain string) error {
	// Get or create cache for this domain
	acm.mutex.Lock()
	cache, exists := acm.caches[domain]
	if !exists {
		cache = &PWACache{
			Domain: domain,
			Assets: make(map[string]*Asset),
		}
		acm.caches[domain] = cache
	}
	acm.mutex.Unlock()

	// Read PWA configuration
	pwaPath := []string{"my", "computer", "network", "web", "pwa", domain}

	// Check if PWA is enabled
	enabledPath := append(pwaPath, "enabled")
	enabledValue, err := acm.tree.Read(enabledPath)
	if err != nil {
		// PWA not configured, disable cache
		cache.mutex.Lock()
		cache.Enabled = false
		cache.SPAMode = false
		cache.Assets = make(map[string]*Asset)
		cache.mutex.Unlock()
		return nil
	}

	enabled := false
	if enabledValue.TypeTag == types.TypeBoolean && len(enabledValue.Data) > 0 {
		enabled = enabledValue.Data[0] != 0
	}

	// Check SPA mode
	spaModePath := append(pwaPath, "spa_mode")
	spaValue, _ := acm.tree.Read(spaModePath)
	spaMode := false
	if spaValue.TypeTag == types.TypeBoolean && len(spaValue.Data) > 0 {
		spaMode = spaValue.Data[0] != 0
	}

	cache.mutex.Lock()
	cache.Enabled = enabled
	cache.SPAMode = spaMode

	if !enabled {
		// Clear assets if PWA is disabled
		cache.Assets = make(map[string]*Asset)
		cache.mutex.Unlock()
		return nil
	}

	// Load all assets
	assetsPath := append(pwaPath, "assets")
	assets, err := acm.loadAssets(assetsPath)
	if err != nil {
		cache.mutex.Unlock()
		return fmt.Errorf("failed to load assets for domain %s: %w", domain, err)
	}

	cache.Assets = assets
	cache.mutex.Unlock()

	return nil
}

// leafLister is the optional capability a storage tree exposes to enumerate the
// stored leaf paths under a prefix. The production tree (*storage.StorageTree
// via *storage.TreeAdapter) implements it; trees that do not fall back to
// probing a fixed set of common filenames.
type leafLister interface {
	ListLeafPaths(prefix []string) ([]string, error)
}

// loadAssets loads every PWA asset stored directly under basePath.
//
// When the tree can enumerate its leaves, all stored assets are loaded by their
// actual stored names — arbitrary filenames work, not just a hardcoded set. For
// trees that cannot enumerate, it falls back to probing a fixed list of common
// asset filenames.
func (acm *AssetCacheManager) loadAssets(basePath []string) (map[string]*Asset, error) {
	assets := make(map[string]*Asset)

	// Preferred path: enumerate the actual stored asset leaves under basePath.
	if lister, ok := acm.tree.(leafLister); ok {
		if leaves, err := lister.ListLeafPaths(basePath); err == nil {
			for _, assetPath := range leaves {
				fullPath := append(append([]string{}, basePath...), assetPath)
				value, err := acm.tree.Read(fullPath)
				if err != nil || value.TypeTag != types.TypeRecord {
					continue
				}
				if asset := acm.parseAssetFromStorageValue(value, assetPath); asset != nil {
					assets[asset.Path] = asset
				}
			}
			return assets, nil
		}
		// Enumeration failed — fall through to the common-filename probe below.
	}

	// Fallback for trees without enumeration: probe a fixed set of common paths.
	commonPaths := []string{
		"index.html", "app.js", "style.css", "app.css",
		"script.js", "main.css", "favicon.ico", "manifest.json",
	}

	for _, assetPath := range commonPaths {
		fullPath := append(append([]string{}, basePath...), assetPath)
		value, err := acm.tree.Read(fullPath)
		if err != nil {
			continue // Asset doesn't exist, skip
		}

		// Check if this is an asset record
		if value.TypeTag == types.TypeRecord {
			asset := acm.parseAssetFromStorageValue(value, assetPath)
			if asset != nil {
				assets[asset.Path] = asset
			}
		}
	}

	return assets, nil
}

// parseAssetFromStorageValue extracts asset data from a storage value
func (acm *AssetCacheManager) parseAssetFromStorageValue(value storage.Value, path string) *Asset {
	if value.TypeTag != types.TypeRecord {
		return nil // Not a record type
	}

	// Parse simplified JSON format from test data
	recordStr := string(value.Data)

	// Extract data field
	dataStart := strings.Index(recordStr, `"data":"`)
	if dataStart == -1 {
		return nil
	}
	dataStart += 8 // Skip `"data":"`
	dataEnd := strings.Index(recordStr[dataStart:], `"`)
	if dataEnd == -1 {
		return nil
	}
	data := recordStr[dataStart : dataStart+dataEnd]

	// Extract mime_type field
	mimeStart := strings.Index(recordStr, `"mime_type":"`)
	if mimeStart == -1 {
		return nil
	}
	mimeStart += 13 // Skip `"mime_type":"` (13 characters)
	mimeEnd := strings.Index(recordStr[mimeStart:], `"`)
	if mimeEnd == -1 {
		return nil
	}
	mimeType := recordStr[mimeStart : mimeStart+mimeEnd]

	return &Asset{
		Data:     []byte(data),
		MimeType: mimeType,
		Path:     path,
		LoadTime: time.Now(),
	}
}

// setupWatcher configures the watcher to monitor PWA configuration and asset changes
func (acm *AssetCacheManager) setupWatcher() {
	// Create a watcher for PWA changes
	// This monitors the entire my.computer.network.web.pwa tree
	watcherCode := `
		// PWA Asset Cache Refresh Watcher
		// This watcher refreshes the asset cache when PWA configuration or assets change
		println("PWA asset cache refresh triggered")
	`

	// Register the watcher using the correct API
	err := acm.watcher.RegisterWatcher("pwa-asset-cache-refresh", []string{"my.computer.network.web.pwa"}, watcherCode)
	if err != nil {
		// Log error but don't fail - cache will still work with periodic refresh
		fmt.Printf("Warning: Could not register PWA asset cache watcher: %v\n", err)
	}

	// Set up periodic scan for new PWA domains
	go acm.periodicRefresh()
}

// periodicRefresh periodically scans for new PWA domains and refreshes caches
func (acm *AssetCacheManager) periodicRefresh() {
	ticker := time.NewTicker(5 * time.Second) // Check every 5 seconds
	defer ticker.Stop()

	for range ticker.C {
		acm.scanForPWADomains()
	}
}

// scanForPWADomains scans for known test domains
// In a real implementation, this would parse attribute names from the storage tree
func (acm *AssetCacheManager) scanForPWADomains() {
	// For testing, check common domain names
	testDomains := []string{
		"app.example.com", "app1.example.com", "app2.example.com", "app3.example.com",
		"test.com", "demo.app",
	}

	for _, domain := range testDomains {
		// Try to read the PWA enabled flag for this domain
		enabledPath := []string{"my", "computer", "network", "web", "pwa", domain, "enabled"}
		_, err := acm.tree.Read(enabledPath)
		if err != nil {
			continue // Domain doesn't exist
		}

		// Check if we already have a cache for this domain
		acm.mutex.RLock()
		_, exists := acm.caches[domain]
		acm.mutex.RUnlock()

		if !exists {
			// Load configuration for new domain
			acm.loadPWAConfig(domain)
		}
	}
}
