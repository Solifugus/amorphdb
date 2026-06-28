// Package directory manages distributed path-to-authority mappings in AmorphDB mesh networks
package directory

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/solifugus/amorphdb/internal/identity"
)

// DirectoryEntry represents a single path-to-authority mapping
type DirectoryEntry struct {
	Path      string             `json:"path"`      // Tree path (e.g., "world.market.equities")
	Authority *identity.Identity `json:"authority"` // Node holding write authority for this path
	Timestamp time.Time          `json:"timestamp"` // When this authority assignment was made
}

// Directory manages distributed path-to-authority mappings
type Directory struct {
	mu      sync.RWMutex
	entries map[string]*DirectoryEntry // path -> entry mapping
}

// NewDirectory creates a new path directory
func NewDirectory() *Directory {
	return &Directory{
		entries: make(map[string]*DirectoryEntry),
	}
}

// Set records or updates authority for a path
func (d *Directory) Set(path string, authority *identity.Identity, timestamp time.Time) {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.entries[path] = &DirectoryEntry{
		Path:      path,
		Authority: authority,
		Timestamp: timestamp,
	}
}

// Get looks up authority for a path, returning the most specific match (longest path prefix)
func (d *Directory) Get(path string) (authority *identity.Identity, timestamp time.Time, ok bool) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	// First try exact match
	if entry, exists := d.entries[path]; exists {
		return entry.Authority, entry.Timestamp, true
	}

	// Find longest prefix match
	var bestMatch *DirectoryEntry
	var bestLength int

	for entryPath, entry := range d.entries {
		// Check if entryPath is a prefix of the requested path
		if strings.HasPrefix(path, entryPath) {
			// Ensure it's a proper prefix (either exact match or followed by a dot)
			if path == entryPath || strings.HasPrefix(path[len(entryPath):], ".") {
				if len(entryPath) > bestLength {
					bestMatch = entry
					bestLength = len(entryPath)
				}
			}
		}
	}

	if bestMatch != nil {
		return bestMatch.Authority, bestMatch.Timestamp, true
	}

	return nil, time.Time{}, false
}

// Delete removes authority record for a path (used when authority is relinquished)
func (d *Directory) Delete(path string) {
	d.mu.Lock()
	defer d.mu.Unlock()

	delete(d.entries, path)
}

// Snapshot returns all directory entries for gossip propagation
func (d *Directory) Snapshot() []DirectoryEntry {
	d.mu.RLock()
	defer d.mu.RUnlock()

	entries := make([]DirectoryEntry, 0, len(d.entries))
	for _, entry := range d.entries {
		// Create a copy to avoid shared references
		entries = append(entries, DirectoryEntry{
			Path:      entry.Path,
			Authority: entry.Authority,
			Timestamp: entry.Timestamp,
		})
	}

	return entries
}

// Merge applies incoming gossip updates using last-write-wins by timestamp
func (d *Directory) Merge(entries []DirectoryEntry) {
	d.mu.Lock()
	defer d.mu.Unlock()

	for _, incomingEntry := range entries {
		// Check if we have an existing entry for this path
		existingEntry, exists := d.entries[incomingEntry.Path]

		if !exists {
			// No existing entry, add the incoming one
			d.entries[incomingEntry.Path] = &DirectoryEntry{
				Path:      incomingEntry.Path,
				Authority: incomingEntry.Authority,
				Timestamp: incomingEntry.Timestamp,
			}
		} else {
			// Last-write-wins: use the entry with the latest timestamp
			if incomingEntry.Timestamp.After(existingEntry.Timestamp) {
				d.entries[incomingEntry.Path] = &DirectoryEntry{
					Path:      incomingEntry.Path,
					Authority: incomingEntry.Authority,
					Timestamp: incomingEntry.Timestamp,
				}
			}
		}
	}
}

// Size returns the number of path entries in the directory
func (d *Directory) Size() int {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return len(d.entries)
}

// String returns a human-readable representation of the directory
func (d *Directory) String() string {
	d.mu.RLock()
	defer d.mu.RUnlock()

	if len(d.entries) == 0 {
		return "Directory(empty)"
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Directory(%d entries):\n", len(d.entries)))
	for path, entry := range d.entries {
		sb.WriteString(fmt.Sprintf("  %s -> %s (at %s)\n",
			path,
			entry.Authority.ID,
			entry.Timestamp.Format(time.RFC3339)))
	}
	return sb.String()
}