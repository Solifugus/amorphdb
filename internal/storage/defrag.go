package storage

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
)

// DefragmentationStats tracks the results of a defragmentation operation
type DefragmentationStats struct {
	OriginalSize     int64
	CompactedSize    int64
	RecordsProcessed int64
	TombstonesRemoved int64
	BytesReclaimed   int64
}

// Defragmenter handles defragmentation operations for storage files
type Defragmenter struct {
	mu sync.Mutex
}

// NewDefragmenter creates a new defragmentation manager
func NewDefragmenter() *Defragmenter {
	return &Defragmenter{}
}

// DefragmentValueStore performs defragmentation on a ValueStore
func (d *Defragmenter) DefragmentValueStore(vs *ValueStore) (*DefragmentationStats, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	vs.mu.Lock()
	defer vs.mu.Unlock()

	stats := &DefragmentationStats{}

	// Defragment tier 3 (large file) first as it's the most critical
	tier3Stats, err := d.defragmentTier3(vs)
	if err != nil {
		return nil, fmt.Errorf("failed to defragment tier 3: %w", err)
	}
	d.mergeStats(stats, tier3Stats)

	// Defragment bucket-based tiers
	for tier := 0; tier < 3; tier++ {
		tierStats, err := d.defragmentBucketTier(vs, tier)
		if err != nil {
			return nil, fmt.Errorf("failed to defragment tier %d: %w", tier, err)
		}
		d.mergeStats(stats, tierStats)
	}

	// Rebuild free lists after defragmentation
	err = d.rebuildFreeLists(vs)
	if err != nil {
		return nil, fmt.Errorf("failed to rebuild free lists: %w", err)
	}

	return stats, nil
}

// defragmentTier3 defragments the large file (tier 3)
func (d *Defragmenter) defragmentTier3(vs *ValueStore) (*DefragmentationStats, error) {
	if vs.tier3File == nil {
		return &DefragmentationStats{}, nil
	}

	// Get original file size
	info, err := vs.tier3File.Stat()
	if err != nil {
		return nil, fmt.Errorf("failed to stat tier 3 file: %w", err)
	}

	originalSize := info.Size()
	if originalSize == 0 {
		return &DefragmentationStats{OriginalSize: 0, CompactedSize: 0}, nil
	}

	// Create temporary file for compacted data
	tempFile, err := os.CreateTemp(filepath.Dir(vs.tier3File.Name()), "tier3_defrag_*.tmp")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp file: %w", err)
	}
	defer os.Remove(tempFile.Name())
	defer tempFile.Close()

	stats := &DefragmentationStats{OriginalSize: originalSize}

	// Build mapping of old offsets to new offsets
	offsetMapping := make(map[uint64]uint64)
	newOffset := uint64(0)

	// First pass: copy non-tombstone records and build offset mapping
	err = d.scanAndCopyTier3Records(vs.tier3File, tempFile, offsetMapping, &newOffset, stats)
	if err != nil {
		return nil, fmt.Errorf("failed to copy tier 3 records: %w", err)
	}

	stats.CompactedSize = int64(newOffset)
	stats.BytesReclaimed = originalSize - stats.CompactedSize

	// Close temp file before replacement
	tempFile.Close()

	// Replace original file with compacted file
	err = d.replaceFile(vs.tier3File, tempFile.Name())
	if err != nil {
		return nil, fmt.Errorf("failed to replace tier 3 file: %w", err)
	}

	// Update all value IDs that point to tier 3
	err = d.updateTier3ValueIDs(vs, offsetMapping)
	if err != nil {
		return nil, fmt.Errorf("failed to update tier 3 value IDs: %w", err)
	}

	return stats, nil
}

// scanAndCopyTier3Records scans tier 3 file and copies non-tombstone records
func (d *Defragmenter) scanAndCopyTier3Records(sourceFile *os.File, destFile *os.File, offsetMapping map[uint64]uint64, newOffset *uint64, stats *DefragmentationStats) error {
	// Get file size
	info, err := sourceFile.Stat()
	if err != nil {
		return err
	}

	fileSize := info.Size()
	oldOffset := int64(0)

	for oldOffset < fileSize {
		// Seek to current position in source
		_, err := sourceFile.Seek(oldOffset, 0)
		if err != nil {
			return fmt.Errorf("seek failed at offset %d: %w", oldOffset, err)
		}

		// Read sentinel byte
		sentinel := make([]byte, 1)
		n, err := sourceFile.Read(sentinel)
		if err != nil {
			if n == 0 && oldOffset >= fileSize {
				break // End of file
			}
			return fmt.Errorf("failed to read sentinel: %w", err)
		}

		if n == 0 {
			break // End of file
		}

		recordStart := uint64(oldOffset)
		stats.RecordsProcessed++

		if sentinel[0] == SentinelValue {
			// Valid record - determine size and copy
			recordSize, err := d.getTier3RecordSize(sourceFile, oldOffset)
			if err != nil {
				return fmt.Errorf("failed to get record size: %w", err)
			}

			// Copy record to new file
			err = d.copyRecord(sourceFile, destFile, oldOffset, int64(recordSize))
			if err != nil {
				return fmt.Errorf("failed to copy record: %w", err)
			}

			// Record offset mapping
			offsetMapping[recordStart] = *newOffset
			*newOffset += recordSize

			oldOffset += int64(recordSize)
		} else if sentinel[0] == SentinelTombstone {
			// Tombstone - skip it
			recordSize, err := d.getTier3RecordSize(sourceFile, oldOffset)
			if err != nil {
				return fmt.Errorf("failed to get tombstone size: %w", err)
			}

			stats.TombstonesRemoved++
			oldOffset += int64(recordSize)
		} else {
			// Unknown sentinel - assume end of valid data
			break
		}
	}

	return nil
}

// getTier3RecordSize determines the size of a record in tier 3
func (d *Defragmenter) getTier3RecordSize(file *os.File, offset int64) (uint64, error) {
	// This is the same logic as in freelist.go
	fl := NewFreeList()
	return fl.determineValueSize(file, offset)
}

// copyRecord copies a record from source to destination file
func (d *Defragmenter) copyRecord(sourceFile, destFile *os.File, offset int64, size int64) error {
	// Seek to record start in source
	_, err := sourceFile.Seek(offset, 0)
	if err != nil {
		return err
	}

	// Copy the data
	_, err = io.CopyN(destFile, sourceFile, size)
	return err
}

// replaceFile replaces an open file with a new file
func (d *Defragmenter) replaceFile(oldFile *os.File, newFilePath string) error {
	oldPath := oldFile.Name()

	// Close the old file
	err := oldFile.Close()
	if err != nil {
		return fmt.Errorf("failed to close old file: %w", err)
	}

	// Replace with new file
	err = os.Rename(newFilePath, oldPath)
	if err != nil {
		return fmt.Errorf("failed to replace file: %w", err)
	}

	// Reopen the file
	newFile, err := os.OpenFile(oldPath, os.O_RDWR, 0644)
	if err != nil {
		return fmt.Errorf("failed to reopen file: %w", err)
	}

	// Update the file handle (this is tricky - we need the ValueStore to update its reference)
	// For now, we'll return and let the caller handle this
	newFile.Close()

	return nil
}

// updateTier3ValueIDs updates value IDs that reference tier 3 after defragmentation
func (d *Defragmenter) updateTier3ValueIDs(vs *ValueStore, offsetMapping map[uint64]uint64) error {
	// This is a complex operation that would require updating all references
	// in attributes, instances, and the hash map. For now, we'll implement
	// a simplified version that rebuilds the hash map.

	// Clear and rebuild hash map to pick up new offsets
	vs.hashToValueID = make(map[[32]byte]uint64)

	// Reopen tier 3 file
	tier3Path := vs.tier3File.Name()
	newFile, err := os.OpenFile(tier3Path, os.O_RDWR, 0644)
	if err != nil {
		return fmt.Errorf("failed to reopen tier 3 file: %w", err)
	}
	vs.tier3File = newFile

	// Rebuild hash map for tier 3 values
	err = d.rebuildTier3HashEntries(vs, offsetMapping)
	if err != nil {
		return fmt.Errorf("failed to rebuild tier 3 hash entries: %w", err)
	}

	return nil
}

// rebuildTier3HashEntries rebuilds hash map entries for tier 3
func (d *Defragmenter) rebuildTier3HashEntries(vs *ValueStore, offsetMapping map[uint64]uint64) error {
	// Scan the compacted file and rebuild hash entries
	info, err := vs.tier3File.Stat()
	if err != nil {
		return err
	}

	fileSize := info.Size()
	offset := int64(0)

	for offset < fileSize {
		// Seek to current position
		_, err := vs.tier3File.Seek(offset, 0)
		if err != nil {
			return err
		}

		// Read and deserialize the value
		recordSize, err := d.getTier3RecordSize(vs.tier3File, offset)
		if err != nil {
			break // End of valid data
		}

		// Read the entire record
		_, err = vs.tier3File.Seek(offset, 0)
		if err != nil {
			return err
		}

		recordData := make([]byte, recordSize)
		n, err := vs.tier3File.Read(recordData)
		if err != nil || uint64(n) != recordSize {
			break
		}

		// Deserialize to get the value
		value, err := vs.deserializeValue(recordData)
		if err != nil {
			break // Skip invalid records
		}

		// Calculate hash and create new value ID
		hash := vs.hashValue(value)
		newValueID := vs.encodeValueID(3, 0, uint64(offset)) // tier 3, bucket 0, new offset
		vs.hashToValueID[hash] = newValueID

		offset += int64(recordSize)
	}

	return nil
}

// defragmentBucketTier defragments a bucket-based tier (0, 1, 2)
func (d *Defragmenter) defragmentBucketTier(vs *ValueStore, tier int) (*DefragmentationStats, error) {
	stats := &DefragmentationStats{}

	var tierFiles map[uint64]*os.File
	switch tier {
	case 0:
		tierFiles = vs.tier0Files
	case 1:
		tierFiles = vs.tier1Files
	case 2:
		tierFiles = vs.tier2Files
	default:
		return stats, fmt.Errorf("invalid tier: %d", tier)
	}

	// Defragment each bucket file in this tier
	for bucketID, file := range tierFiles {
		bucketStats, err := d.defragmentBucketFile(vs, tier, bucketID, file)
		if err != nil {
			return nil, fmt.Errorf("failed to defragment tier %d bucket %d: %w", tier, bucketID, err)
		}
		d.mergeStats(stats, bucketStats)
	}

	return stats, nil
}

// defragmentBucketFile defragments a single bucket file
func (d *Defragmenter) defragmentBucketFile(vs *ValueStore, tier int, bucketID uint64, file *os.File) (*DefragmentationStats, error) {
	// Similar logic to tier 3, but for bucket files
	// For simplicity, we'll implement a basic version
	info, err := file.Stat()
	if err != nil {
		return nil, err
	}

	stats := &DefragmentationStats{
		OriginalSize: info.Size(),
	}

	// For now, just return the original size as compacted size
	// A full implementation would compact the bucket files
	stats.CompactedSize = info.Size()

	return stats, nil
}

// rebuildFreeLists rebuilds all free lists after defragmentation
func (d *Defragmenter) rebuildFreeLists(vs *ValueStore) error {
	// Clear existing free lists
	for i := 0; i < 4; i++ {
		vs.freeLists[i].Clear()
	}

	// Rebuild tier 3 free list
	if vs.tier3File != nil {
		err := vs.freeLists[3].RebuildFromFile(vs.tier3File)
		if err != nil {
			return fmt.Errorf("failed to rebuild tier 3 free list: %w", err)
		}
	}

	// Rebuild bucket tier free lists
	for tier := 0; tier < 3; tier++ {
		var tierFiles map[uint64]*os.File
		switch tier {
		case 0:
			tierFiles = vs.tier0Files
		case 1:
			tierFiles = vs.tier1Files
		case 2:
			tierFiles = vs.tier2Files
		}

		// For each bucket file, rebuild its portion of the free list
		for _, file := range tierFiles {
			fl := NewFreeList()
			err := fl.RebuildFromFile(file)
			if err != nil {
				return fmt.Errorf("failed to rebuild tier %d free list: %w", tier, err)
			}
			// Merge blocks with existing free list for this tier
			blocks := fl.GetBlocks()
			for _, block := range blocks {
				vs.freeLists[tier].AddBlock(block.Offset, block.Size)
			}
		}
	}

	return nil
}

// mergeStats combines defragmentation stats
func (d *Defragmenter) mergeStats(dest, src *DefragmentationStats) {
	dest.OriginalSize += src.OriginalSize
	dest.CompactedSize += src.CompactedSize
	dest.RecordsProcessed += src.RecordsProcessed
	dest.TombstonesRemoved += src.TombstonesRemoved
	dest.BytesReclaimed += src.BytesReclaimed
}