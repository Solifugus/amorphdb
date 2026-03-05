package storage

import (
	"encoding/binary"
	"fmt"
	"os"
	"sort"
	"sync"
)

// FreeBlock represents a tombstoned space available for reuse
type FreeBlock struct {
	Offset uint64
	Size   uint64
}

// FreeList manages free space in a storage file
type FreeList struct {
	mu     sync.RWMutex
	blocks []FreeBlock
}

// NewFreeList creates a new free list
func NewFreeList() *FreeList {
	return &FreeList{
		blocks: make([]FreeBlock, 0),
	}
}

// AddBlock adds a free block to the list
func (fl *FreeList) AddBlock(offset, size uint64) {
	fl.mu.Lock()
	defer fl.mu.Unlock()

	newBlock := FreeBlock{Offset: offset, Size: size}

	// Insert sorted by offset for efficient coalescing
	insertIndex := 0
	for i, block := range fl.blocks {
		if offset < block.Offset {
			insertIndex = i
			break
		}
		insertIndex = i + 1
	}

	// Insert the block
	fl.blocks = append(fl.blocks, FreeBlock{})
	copy(fl.blocks[insertIndex+1:], fl.blocks[insertIndex:])
	fl.blocks[insertIndex] = newBlock

	// Coalesce adjacent blocks
	fl.coalesceBlocks()
}

// AllocateSpace finds and allocates space from the free list
// Returns the allocated offset and whether allocation succeeded
func (fl *FreeList) AllocateSpace(size uint64) (uint64, bool) {
	fl.mu.Lock()
	defer fl.mu.Unlock()

	for i, block := range fl.blocks {
		if block.Size >= size {
			offset := block.Offset

			if block.Size == size {
				// Perfect fit - remove the block
				fl.blocks = append(fl.blocks[:i], fl.blocks[i+1:]...)
			} else {
				// Partial fit - shrink the block
				fl.blocks[i].Offset += size
				fl.blocks[i].Size -= size
			}

			return offset, true
		}
	}

	return 0, false
}

// GetBlocks returns a copy of all free blocks (for testing/debugging)
func (fl *FreeList) GetBlocks() []FreeBlock {
	fl.mu.RLock()
	defer fl.mu.RUnlock()

	result := make([]FreeBlock, len(fl.blocks))
	copy(result, fl.blocks)
	return result
}

// GetTotalFreeSpace returns the total amount of free space
func (fl *FreeList) GetTotalFreeSpace() uint64 {
	fl.mu.RLock()
	defer fl.mu.RUnlock()

	var total uint64
	for _, block := range fl.blocks {
		total += block.Size
	}
	return total
}

// Clear removes all free blocks
func (fl *FreeList) Clear() {
	fl.mu.Lock()
	defer fl.mu.Unlock()
	fl.blocks = fl.blocks[:0]
}

// coalesceBlocks merges adjacent free blocks
// Must be called with lock held
func (fl *FreeList) coalesceBlocks() {
	if len(fl.blocks) <= 1 {
		return
	}

	// Sort by offset to ensure proper ordering
	sort.Slice(fl.blocks, func(i, j int) bool {
		return fl.blocks[i].Offset < fl.blocks[j].Offset
	})

	// Merge adjacent blocks
	writeIndex := 0
	for readIndex := 0; readIndex < len(fl.blocks); readIndex++ {
		if writeIndex > 0 {
			prevBlock := &fl.blocks[writeIndex-1]
			currentBlock := fl.blocks[readIndex]

			// Check if blocks are adjacent
			if prevBlock.Offset+prevBlock.Size == currentBlock.Offset {
				// Merge with previous block
				prevBlock.Size += currentBlock.Size
				continue
			}
		}

		// Copy block to write position
		if writeIndex != readIndex {
			fl.blocks[writeIndex] = fl.blocks[readIndex]
		}
		writeIndex++
	}

	// Truncate to new size
	fl.blocks = fl.blocks[:writeIndex]
}

// RebuildFromFile scans a file for tombstones and rebuilds the free list
func (fl *FreeList) RebuildFromFile(file *os.File) error {
	fl.mu.Lock()
	defer fl.mu.Unlock()

	// Clear existing blocks
	fl.blocks = fl.blocks[:0]

	// Get file size
	info, err := file.Stat()
	if err != nil {
		return fmt.Errorf("failed to stat file: %w", err)
	}

	fileSize := info.Size()
	if fileSize == 0 {
		return nil
	}

	// Scan the file for tombstone records
	offset := int64(0)
	for offset < fileSize {
		// Seek to current position
		_, err := file.Seek(offset, 0)
		if err != nil {
			return fmt.Errorf("seek failed at offset %d: %w", offset, err)
		}

		// Read the sentinel byte
		sentinel := make([]byte, 1)
		n, err := file.Read(sentinel)
		if err != nil {
			if n == 0 && offset >= fileSize {
				break // End of file
			}
			return fmt.Errorf("failed to read sentinel at offset %d: %w", offset, err)
		}

		if n == 0 {
			break // End of file
		}

		recordStart := uint64(offset)

		if sentinel[0] == SentinelTombstone {
			// Found a tombstone - determine its size
			recordSize, err := fl.determineTombstoneSize(file, offset)
			if err != nil {
				return fmt.Errorf("failed to determine tombstone size at offset %d: %w", offset, err)
			}

			// Add to free list
			fl.blocks = append(fl.blocks, FreeBlock{
				Offset: recordStart,
				Size:   recordSize,
			})

			offset += int64(recordSize)
		} else if sentinel[0] == SentinelValue {
			// Valid value record - skip it
			recordSize, err := fl.determineValueSize(file, offset)
			if err != nil {
				return fmt.Errorf("failed to determine value size at offset %d: %w", offset, err)
			}

			offset += int64(recordSize)
		} else {
			// Unknown sentinel - this might indicate corruption or end of valid data
			break
		}
	}

	// Coalesce adjacent blocks
	fl.coalesceBlocks()

	return nil
}

// determineTombstoneSize determines the size of a tombstoned record
func (fl *FreeList) determineTombstoneSize(file *os.File, offset int64) (uint64, error) {
	// Seek to start of record
	_, err := file.Seek(offset, 0)
	if err != nil {
		return 0, err
	}

	// Read header: sentinel (1) + type_tag (1)
	header := make([]byte, 2)
	n, err := file.Read(header)
	if err != nil || n != 2 {
		return 0, fmt.Errorf("failed to read tombstone header")
	}

	typeTag := header[1]
	recordSize := uint64(2) // sentinel + type_tag

	// For variable-length types, read the length field
	if fl.isVariableLength(typeTag) {
		lengthBytes := make([]byte, 4)
		n, err := file.Read(lengthBytes)
		if err != nil || n != 4 {
			return 0, fmt.Errorf("failed to read tombstone length")
		}

		dataLength := binary.LittleEndian.Uint32(lengthBytes)
		recordSize += 4 + uint64(dataLength) // length field + data
	} else {
		// Fixed-length types have known sizes
		dataSize := fl.getFixedTypeSize(typeTag)
		recordSize += uint64(dataSize)
	}

	return recordSize, nil
}

// determineValueSize determines the size of a value record
func (fl *FreeList) determineValueSize(file *os.File, offset int64) (uint64, error) {
	return fl.determineTombstoneSize(file, offset) // Same logic for determining record size
}

// isVariableLength checks if a type tag represents a variable-length type
func (fl *FreeList) isVariableLength(typeTag byte) bool {
	switch typeTag {
	case TypeText, TypePicture, TypeProcedure, TypeWatcher, TypeEmbed:
		return true
	default:
		return false
	}
}

// getFixedTypeSize returns the data size for fixed-length types
func (fl *FreeList) getFixedTypeSize(typeTag byte) int {
	switch typeTag {
	case TypeNumber:
		return 8 // float64
	case TypeTime:
		return 9 // 8 bytes UNIX time + 1 byte precision
	case TypeMoney:
		return 12 // 8 bytes amount + 4 bytes currency code
	case TypeReference:
		return 8 // uint64
	case TypeNothing, TypeAnything:
		return 0 // no data
	case TypeUnknown:
		// Unknown has a reason string, but if we're seeing it as fixed-length
		// it might be stored differently - this is a fallback
		return 0
	default:
		return 0
	}
}