// Package interpreter provides commit coordination for batched writes
package interpreter

import (
	"fmt"
	"log"
	"sync"

	"github.com/solifugus/amorphdb/internal/mesh"
	"github.com/solifugus/amorphdb/internal/storage"
	"github.com/solifugus/amorphdb/internal/zone"
)

// CommitCoordinator manages batched writes across zones and execution contexts
type CommitCoordinator struct {
	mu               sync.Mutex
	hashRing         *zone.HashRing
	replicationMgr   *mesh.ReplicationManager
	localStorage     storage.Tree
	localNodeID      string
	pendingWrites    []PendingWrite
	bufferLimit      int
	logger           *log.Logger
}

// NewCommitCoordinator creates a new commit coordination instance
func NewCommitCoordinator(hashRing *zone.HashRing, replicationMgr *mesh.ReplicationManager, localStorage storage.Tree, localNodeID string, logger *log.Logger) *CommitCoordinator {
	if logger == nil {
		logger = log.New(log.Writer(), "[CommitCoordinator] ", log.LstdFlags)
	}

	return &CommitCoordinator{
		hashRing:       hashRing,
		replicationMgr: replicationMgr,
		localStorage:   localStorage,
		localNodeID:    localNodeID,
		pendingWrites:  make([]PendingWrite, 0),
		bufferLimit:    DefaultCommitBufferLimit,
		logger:         logger,
	}
}

// StageWrites adds writes from an interpreter's commit buffer
func (cc *CommitCoordinator) StageWrites(writes []PendingWrite) error {
	cc.mu.Lock()
	defer cc.mu.Unlock()

	// Check buffer limit
	if len(cc.pendingWrites)+len(writes) > cc.bufferLimit {
		return fmt.Errorf("commit buffer exceeded: %d + %d > %d",
			len(cc.pendingWrites), len(writes), cc.bufferLimit)
	}

	// Stage all writes
	cc.pendingWrites = append(cc.pendingWrites, writes...)
	return nil
}

// FlushAll commits all staged writes as coordinated batches
func (cc *CommitCoordinator) FlushAll() error {
	cc.mu.Lock()
	defer cc.mu.Unlock()

	if len(cc.pendingWrites) == 0 {
		return nil // Nothing to flush
	}

	cc.logger.Printf("Flushing %d staged writes", len(cc.pendingWrites))

	// Group writes by zone
	localWrites := make([]PendingWrite, 0)
	crossZoneWrites := make(map[string][]PendingWrite)

	for _, write := range cc.pendingWrites {
		zonePath := cc.pathToZonePath(write.path)
		zoneAssignment, err := cc.hashRing.GetZoneAssignment(zonePath)
		if err != nil {
			// If we can't determine zone, treat as local write
			cc.logger.Printf("Warning: could not determine zone for path %v, treating as local: %v", write.path, err)
			localWrites = append(localWrites, write)
			continue
		}

		if zoneAssignment.Authority == cc.localNodeID {
			// Local zone write
			localWrites = append(localWrites, write)
		} else {
			// Cross-zone write - group by destination zone
			zoneID := zoneAssignment.ZoneID
			if crossZoneWrites[zoneID] == nil {
				crossZoneWrites[zoneID] = make([]PendingWrite, 0)
			}
			crossZoneWrites[zoneID] = append(crossZoneWrites[zoneID], write)
		}
	}

	// Apply local writes directly
	for _, write := range localWrites {
		err := cc.localStorage.Write(write.path, write.value, write.author)
		if err != nil {
			// On failure, discard all pending writes and return error
			cc.pendingWrites = cc.pendingWrites[:0]
			return fmt.Errorf("local write failed for path %v: %w", write.path, err)
		}
	}

	// Send cross-zone writes as batched payloads
	for zoneID, writes := range crossZoneWrites {
		err := cc.sendCrossZoneBatch(zoneID, writes)
		if err != nil {
			// On failure, discard all pending writes and return error
			cc.pendingWrites = cc.pendingWrites[:0]
			return fmt.Errorf("cross-zone batch failed for zone %s: %w", zoneID, err)
		}
	}

	// Clear buffer after successful flush
	cc.pendingWrites = cc.pendingWrites[:0]
	cc.logger.Printf("Successfully flushed %d local writes and %d cross-zone batches",
		len(localWrites), len(crossZoneWrites))

	return nil
}

// DiscardAll clears all staged writes without committing
func (cc *CommitCoordinator) DiscardAll() {
	cc.mu.Lock()
	defer cc.mu.Unlock()

	count := len(cc.pendingWrites)
	cc.pendingWrites = cc.pendingWrites[:0]
	if count > 0 {
		cc.logger.Printf("Discarded %d staged writes due to rollback", count)
	}
}

// GetPendingCount returns the number of staged writes
func (cc *CommitCoordinator) GetPendingCount() int {
	cc.mu.Lock()
	defer cc.mu.Unlock()
	return len(cc.pendingWrites)
}

// pathToZonePath converts a data path to a zone path for hash ring lookup
func (cc *CommitCoordinator) pathToZonePath(path []string) string {
	if len(path) == 0 {
		return "/"
	}

	// Use the first few path elements to determine zone
	// This ensures related data stays in the same zone
	zonePath := "/"
	for i, segment := range path {
		zonePath += segment
		if i < len(path)-1 {
			zonePath += "/"
		}
		// Limit zone path depth to avoid excessive splitting
		if i >= 2 {
			break
		}
	}
	return zonePath
}

// sendCrossZoneBatch sends a batch of writes to a cross-zone authority
func (cc *CommitCoordinator) sendCrossZoneBatch(zoneID string, writes []PendingWrite) error {
	// Use the replication manager to send batched writes
	for _, write := range writes {
		err := cc.replicationMgr.RecordWrite(write.path, write.value, write.author, zoneID)
		if err != nil {
			return fmt.Errorf("failed to record write for replication: %w", err)
		}
	}
	return nil
}