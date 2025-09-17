package storage

import (
	"sync"
)

// Index represents a secondary index
type Index struct {
	field   string
	entries map[string][]string // value -> []docIDs
	mu      sync.RWMutex
}

// NewIndex creates a new index
func NewIndex(field string) *Index {
	return &Index{
		field:   field,
		entries: make(map[string][]string),
	}
}

// Put adds a document ID to the index for a given value
func (idx *Index) Put(value, docID string) {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	if _, exists := idx.entries[value]; !exists {
		idx.entries[value] = []string{}
	}
	
	// Check if docID already exists
	for _, existingID := range idx.entries[value] {
		if existingID == docID {
			return
		}
	}
	
	idx.entries[value] = append(idx.entries[value], docID)
}

// Get returns document IDs for a given value
func (idx *Index) Get(value string) []string {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	if docIDs, exists := idx.entries[value]; exists {
		// Return copy to avoid race conditions
		result := make([]string, len(docIDs))
		copy(result, docIDs)
		return result
	}
	
	return []string{}
}

// Delete removes a document ID from all values in the index
func (idx *Index) Delete(docID string) {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	for value, docIDs := range idx.entries {
		for i, id := range docIDs {
			if id == docID {
				// Remove docID from slice
				idx.entries[value] = append(docIDs[:i], docIDs[i+1:]...)
				break
			}
		}
		
		// Remove empty entries
		if len(idx.entries[value]) == 0 {
			delete(idx.entries, value)
		}
	}
}

// Field returns the field name this index is for
func (idx *Index) Field() string {
	return idx.field
}

// Size returns the number of unique values in the index
func (idx *Index) Size() int {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	return len(idx.entries)
}
