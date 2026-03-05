package storage

import (
	"fmt"
	"hash/fnv"
	"sync"
)

// HashBucket represents a single bucket in the hash table
type HashBucket struct {
	LabelValueID uint64 // The label being hashed
	AttributeID  uint64 // The attribute ID for this label
	Next         *HashBucket // For collision resolution via chaining
}

// HashIndex provides O(1) lookup of attributes by label value ID
type HashIndex struct {
	buckets     []*HashBucket
	bucketCount uint64
	mu          sync.RWMutex
	size        uint64 // Number of entries in the index
}

// Default bucket count for the hash table
const DefaultBucketCount = 1024

// NewHashIndex creates a new hash index with the specified bucket count
func NewHashIndex(bucketCount uint64) *HashIndex {
	if bucketCount == 0 {
		bucketCount = DefaultBucketCount
	}

	return &HashIndex{
		buckets:     make([]*HashBucket, bucketCount),
		bucketCount: bucketCount,
	}
}

// hash computes a hash value for a label value ID
func (hi *HashIndex) hash(labelValueID uint64) uint64 {
	h := fnv.New64a()
	// Convert uint64 to bytes for hashing
	bytes := make([]byte, 8)
	for i := 0; i < 8; i++ {
		bytes[i] = byte(labelValueID >> (8 * i))
	}
	h.Write(bytes)
	return h.Sum64() % hi.bucketCount
}

// Insert adds a label-to-attribute mapping to the hash index
func (hi *HashIndex) Insert(labelValueID uint64, attributeID uint64) {
	hi.mu.Lock()
	defer hi.mu.Unlock()

	bucketIndex := hi.hash(labelValueID)

	// Check if this label already exists in the bucket chain
	for bucket := hi.buckets[bucketIndex]; bucket != nil; bucket = bucket.Next {
		if bucket.LabelValueID == labelValueID {
			// Update existing entry
			bucket.AttributeID = attributeID
			return
		}
	}

	// Create new bucket and prepend to chain
	newBucket := &HashBucket{
		LabelValueID: labelValueID,
		AttributeID:  attributeID,
		Next:         hi.buckets[bucketIndex],
	}
	hi.buckets[bucketIndex] = newBucket
	hi.size++
}

// Lookup finds an attribute ID by label value ID
func (hi *HashIndex) Lookup(labelValueID uint64) (uint64, bool) {
	hi.mu.RLock()
	defer hi.mu.RUnlock()

	bucketIndex := hi.hash(labelValueID)

	// Search through the bucket chain
	for bucket := hi.buckets[bucketIndex]; bucket != nil; bucket = bucket.Next {
		if bucket.LabelValueID == labelValueID {
			return bucket.AttributeID, true
		}
	}

	return 0, false
}

// Remove removes a label-to-attribute mapping from the hash index
func (hi *HashIndex) Remove(labelValueID uint64) bool {
	hi.mu.Lock()
	defer hi.mu.Unlock()

	bucketIndex := hi.hash(labelValueID)

	// Handle removal from head of chain
	if hi.buckets[bucketIndex] != nil && hi.buckets[bucketIndex].LabelValueID == labelValueID {
		hi.buckets[bucketIndex] = hi.buckets[bucketIndex].Next
		hi.size--
		return true
	}

	// Search through the chain for the bucket to remove
	for bucket := hi.buckets[bucketIndex]; bucket != nil && bucket.Next != nil; bucket = bucket.Next {
		if bucket.Next.LabelValueID == labelValueID {
			bucket.Next = bucket.Next.Next
			hi.size--
			return true
		}
	}

	return false
}

// Size returns the number of entries in the hash index
func (hi *HashIndex) Size() uint64 {
	hi.mu.RLock()
	defer hi.mu.RUnlock()
	return hi.size
}

// RebuildFromAttributeStore rebuilds the hash index from an attribute store
func (hi *HashIndex) RebuildFromAttributeStore(as *AttributeStore) error {
	hi.mu.Lock()
	defer hi.mu.Unlock()

	// Clear existing index
	hi.buckets = make([]*HashBucket, hi.bucketCount)
	hi.size = 0

	// Read all attributes and rebuild index
	attributeCount := as.GetAttributeCount()
	for attributeID := uint64(1); attributeID <= attributeCount; attributeID++ {
		attribute, err := as.ReadAttribute(attributeID)
		if err != nil {
			return fmt.Errorf("failed to read attribute %d during rebuild: %w", attributeID, err)
		}

		// Insert into index (without acquiring lock since we already have it)
		bucketIndex := hi.hash(attribute.LabelValueID)

		// Check if this label already exists (shouldn't happen in normal operation)
		exists := false
		for bucket := hi.buckets[bucketIndex]; bucket != nil; bucket = bucket.Next {
			if bucket.LabelValueID == attribute.LabelValueID {
				bucket.AttributeID = attributeID
				exists = true
				break
			}
		}

		if !exists {
			// Create new bucket and prepend to chain
			newBucket := &HashBucket{
				LabelValueID: attribute.LabelValueID,
				AttributeID:  attributeID,
				Next:         hi.buckets[bucketIndex],
			}
			hi.buckets[bucketIndex] = newBucket
			hi.size++
		}
	}

	return nil
}

// GetCollisionStats returns statistics about hash collisions for performance analysis
func (hi *HashIndex) GetCollisionStats() (uint64, uint64, uint64) {
	hi.mu.RLock()
	defer hi.mu.RUnlock()

	var usedBuckets uint64
	var totalChainLength uint64
	var maxChainLength uint64

	for _, bucket := range hi.buckets {
		if bucket != nil {
			usedBuckets++
			chainLength := uint64(0)
			for b := bucket; b != nil; b = b.Next {
				chainLength++
			}
			totalChainLength += chainLength
			if chainLength > maxChainLength {
				maxChainLength = chainLength
			}
		}
	}

	return usedBuckets, totalChainLength, maxChainLength
}

// Clear removes all entries from the hash index
func (hi *HashIndex) Clear() {
	hi.mu.Lock()
	defer hi.mu.Unlock()

	hi.buckets = make([]*HashBucket, hi.bucketCount)
	hi.size = 0
}