package storage

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"coffedb/internal/config"
)

// Document represents a JSON document in the database
type Document struct {
	ID        string                 `json:"id"`
	Data      map[string]interface{} `json:"data"`
	CreatedAt time.Time              `json:"created_at"`
	UpdatedAt time.Time              `json:"updated_at"`
	Version   int64                  `json:"version"`
}

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

// Engine is the main storage engine
type Engine struct {
	config    config.StorageConfig
	memtable  *Memtable
	wal       *WAL
	btree     *BTree
	indexes   map[string]*Index
	mu        sync.RWMutex
	compacting bool
}

// NewEngine creates a new storage engine
func NewEngine(cfg config.StorageConfig) (*Engine, error) {
	// Create data directory if it doesn't exist
	if err := os.MkdirAll(cfg.DataDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create data directory: %w", err)
	}

	// Initialize WAL
	wal, err := NewWAL(filepath.Join(cfg.DataDir, "wal.log"))
	if err != nil {
		return nil, fmt.Errorf("failed to initialize WAL: %w", err)
	}

	// Initialize B-tree for persistent storage
	btree, err := NewBTree(filepath.Join(cfg.DataDir, "data.db"))
	if err != nil {
		return nil, fmt.Errorf("failed to initialize B-tree: %w", err)
	}

	// Initialize memtable
	memtable := NewMemtable(cfg.MemtableSize)

	engine := &Engine{
		config:   cfg,
		memtable: memtable,
		wal:      wal,
		btree:    btree,
		indexes:  make(map[string]*Index),
	}

	// Recover from WAL if needed
	if err := engine.recover(); err != nil {
		return nil, fmt.Errorf("failed to recover from WAL: %w", err)
	}

	// Start background compaction
	go engine.backgroundCompaction()

	return engine, nil
}

// Put stores a document in the database
func (e *Engine) Put(collection, id string, data map[string]interface{}) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	key := fmt.Sprintf("%s:%s", collection, id)
	doc := &Document{
		ID:        id,
		Data:      data,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Version:   1,
	}

	// Check if document exists and increment version
	if existing, exists := e.memtable.Get(key); exists {
		if existingDoc, ok := existing.(*Document); ok {
			doc.CreatedAt = existingDoc.CreatedAt
			doc.Version = existingDoc.Version + 1
		}
	}

	// Write to WAL first
	if err := e.wal.WriteEntry(WALEntry{
		Type:      WALPut,
		Key:       key,
		Value:     doc,
		Timestamp: time.Now(),
	}); err != nil {
		return fmt.Errorf("failed to write to WAL: %w", err)
	}

	// Write to memtable
	e.memtable.Put(key, doc)

	// Update indexes
	e.updateIndexes(collection, id, doc)

	// Check if memtable needs flushing
	if e.memtable.Size() >= e.config.MemtableSize {
		go e.flushMemtable()
	}

	return nil
}

// Get retrieves a document from the database
func (e *Engine) Get(collection, id string) (*Document, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	key := fmt.Sprintf("%s:%s", collection, id)

	// Check memtable first
	if value, exists := e.memtable.Get(key); exists {
		if doc, ok := value.(*Document); ok {
			return doc, nil
		}
	}

	// Check disk storage
	value, err := e.btree.Get(key)
	if err != nil {
		return nil, err
	}

	if doc, ok := value.(*Document); ok {
		return doc, nil
	}

	return nil, fmt.Errorf("document not found")
}

// Delete removes a document from the database
func (e *Engine) Delete(collection, id string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	key := fmt.Sprintf("%s:%s", collection, id)

	// Write to WAL first
	if err := e.wal.WriteEntry(WALEntry{
		Type:      WALDelete,
		Key:       key,
		Timestamp: time.Now(),
	}); err != nil {
		return fmt.Errorf("failed to write to WAL: %w", err)
	}

	// Remove from memtable
	e.memtable.Delete(key)

	// Remove from indexes
	e.removeFromIndexes(collection, id)

	return nil
}

// Query performs a query on the collection
func (e *Engine) Query(collection string, filter map[string]interface{}) ([]*Document, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	var results []*Document
	prefix := collection + ":"

	// Query memtable
	e.memtable.Range(prefix, func(key string, value interface{}) bool {
		if doc, ok := value.(*Document); ok {
			if e.matchesFilter(doc, filter) {
				results = append(results, doc)
			}
		}
		return true
	})

	// Query disk storage
	diskResults, err := e.btree.Range(prefix)
	if err != nil {
		return nil, err
	}

	for _, value := range diskResults {
		if doc, ok := value.(*Document); ok {
			if e.matchesFilter(doc, filter) {
				results = append(results, doc)
			}
		}
	}

	return results, nil
}

// CreateIndex creates a secondary index on a field
func (e *Engine) CreateIndex(collection, field string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	indexKey := fmt.Sprintf("%s.%s", collection, field)
	if _, exists := e.indexes[indexKey]; exists {
		return fmt.Errorf("index already exists")
	}

	index := NewIndex(field)
	e.indexes[indexKey] = index

	// Build index from existing data
	return e.buildIndex(collection, field, index)
}

// Helper methods

// FIXED: matchesFilter now handles type conversions properly
func (e *Engine) matchesFilter(doc *Document, filter map[string]interface{}) bool {
	for key, value := range filter {
		docValue, exists := doc.Data[key]
		if !exists {
			return false
		}
		
		// Handle type conversions for numeric comparisons
		if !e.valuesEqual(docValue, value) {
			return false
		}
	}
	return true
}

// NEW: Helper function to handle type conversions
func (e *Engine) valuesEqual(a, b interface{}) bool {
	// Direct equality check first
	if a == b {
		return true
	}
	
	// Handle numeric type conversions
	aFloat, aIsNum := toFloat64(a)
	bFloat, bIsNum := toFloat64(b)
	
	if aIsNum && bIsNum {
		return aFloat == bFloat
	}
	
	// Handle string comparisons
	aStr, aIsStr := a.(string)
	bStr, bIsStr := b.(string)
	
	if aIsStr && bIsStr {
		return aStr == bStr
	}
	
	// Handle boolean comparisons
	aBool, aIsBool := a.(bool)
	bBool, bIsBool := b.(bool)
	
	if aIsBool && bIsBool {
		return aBool == bBool
	}
	
	return false
}

// NEW: Helper function to convert various numeric types to float64
func toFloat64(v interface{}) (float64, bool) {
	switch val := v.(type) {
	case float64:
		return val, true
	case float32:
		return float64(val), true
	case int:
		return float64(val), true
	case int8:
		return float64(val), true
	case int16:
		return float64(val), true
	case int32:
		return float64(val), true
	case int64:
		return float64(val), true
	case uint:
		return float64(val), true
	case uint8:
		return float64(val), true
	case uint16:
		return float64(val), true
	case uint32:
		return float64(val), true
	case uint64:
		return float64(val), true
	default:
		return 0, false
	}
}

func (e *Engine) updateIndexes(collection, id string, doc *Document) {
	for indexKey, index := range e.indexes {
		if len(indexKey) > len(collection) && indexKey[:len(collection)] == collection {
			field := indexKey[len(collection)+1:]
			if value, exists := doc.Data[field]; exists {
				index.Put(fmt.Sprintf("%v", value), id)
			}
		}
	}
}

func (e *Engine) removeFromIndexes(collection, id string) {
	for indexKey, index := range e.indexes {
		if len(indexKey) > len(collection) && indexKey[:len(collection)] == collection {
			index.Delete(id)
		}
	}
}

func (e *Engine) buildIndex(collection, field string, index *Index) error {
	prefix := collection + ":"

	// Build from memtable
	e.memtable.Range(prefix, func(key string, value interface{}) bool {
		if doc, ok := value.(*Document); ok {
			if fieldValue, exists := doc.Data[field]; exists {
				index.Put(fmt.Sprintf("%v", fieldValue), doc.ID)
			}
		}
		return true
	})

	// Build from disk
	diskResults, err := e.btree.Range(prefix)
	if err != nil {
		return err
	}

	for _, value := range diskResults {
		if doc, ok := value.(*Document); ok {
			if fieldValue, exists := doc.Data[field]; exists {
				index.Put(fmt.Sprintf("%v", fieldValue), doc.ID)
			}
		}
	}

	return nil
}

func (e *Engine) flushMemtable() {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.memtable.IsEmpty() {
		return
	}

	// Create new memtable
	oldMemtable := e.memtable
	e.memtable = NewMemtable(e.config.MemtableSize)

	// Write old memtable to disk
	oldMemtable.Range("", func(key string, value interface{}) bool {
		e.btree.Put(key, value)
		return true
	})
}

func (e *Engine) backgroundCompaction() {
	ticker := time.NewTicker(time.Duration(e.config.CompactionInterval) * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		e.compact()
	}
}

func (e *Engine) compact() {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.compacting {
		return
	}

	e.compacting = true
	defer func() {
		e.compacting = false
	}()

	// Perform compaction logic here
}

func (e *Engine) recover() error {
	entries, err := e.wal.ReadEntries()
	if err != nil {
		return err
	}

	for _, entry := range entries {
		switch entry.Type {
		case WALPut:
			if doc, ok := entry.Value.(*Document); ok {
				e.memtable.Put(entry.Key, doc)
			}
		case WALDelete:
			e.memtable.Delete(entry.Key)
		}
	}

	return nil
}

// Close shuts down the storage engine
func (e *Engine) Close() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	// Flush memtable
	e.flushMemtable()

	// Close WAL
	if err := e.wal.Close(); err != nil {
		return err
	}

	// Close B-tree
	if err := e.btree.Close(); err != nil {
		return err
	}

	return nil
}

// Stats returns storage engine statistics
func (e *Engine) Stats() map[string]interface{} {
	e.mu.RLock()
	defer e.mu.RUnlock()

	return map[string]interface{}{
		"memtable_size":    e.memtable.Size(),
		"memtable_count":   e.memtable.Count(),
		"indexes_count":    len(e.indexes),
		"compacting":       e.compacting,
	}
}