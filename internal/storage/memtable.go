package storage

import (
	"math/rand"
	"strings"
	"sync"
	"time"
)

const (
	maxLevel     = 32
	probability  = 0.5
)

// SkipListNode represents a node in the skip list
type SkipListNode struct {
	key     string
	value   interface{}
	forward []*SkipListNode
	ttl     *time.Time
}

// Memtable represents an in-memory table using skip list
type Memtable struct {
	header   *SkipListNode
	level    int
	size     int64
	maxSize  int64
	count    int64
	mu       sync.RWMutex
	rand     *rand.Rand
}

// NewMemtable creates a new memtable
func NewMemtable(maxSize int64) *Memtable {
	header := &SkipListNode{
		forward: make([]*SkipListNode, maxLevel),
	}
	
	return &Memtable{
		header:  header,
		level:   0,
		maxSize: maxSize,
		rand:    rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// Put inserts a key-value pair
func (mt *Memtable) Put(key string, value interface{}) {
	mt.mu.Lock()
	defer mt.mu.Unlock()
	
	update := make([]*SkipListNode, maxLevel)
	current := mt.header
	
	// Find position to insert
	for i := mt.level; i >= 0; i-- {
		for current.forward[i] != nil && current.forward[i].key < key {
			current = current.forward[i]
		}
		update[i] = current
	}
	
	current = current.forward[0]
	
	// Update existing key
	if current != nil && current.key == key {
		current.value = value
		return
	}
	
	// Insert new node
	newLevel := mt.randomLevel()
	if newLevel > mt.level {
		for i := mt.level + 1; i <= newLevel; i++ {
			update[i] = mt.header
		}
		mt.level = newLevel
	}
	
	newNode := &SkipListNode{
		key:     key,
		value:   value,
		forward: make([]*SkipListNode, newLevel+1),
	}
	
	for i := 0; i <= newLevel; i++ {
		newNode.forward[i] = update[i].forward[i]
		update[i].forward[i] = newNode
	}
	
	mt.count++
	mt.size += int64(len(key)) + mt.estimateValueSize(value)
}

// Get retrieves a value by key
func (mt *Memtable) Get(key string) (interface{}, bool) {
	mt.mu.RLock()
	defer mt.mu.RUnlock()
	
	current := mt.header
	
	for i := mt.level; i >= 0; i-- {
		for current.forward[i] != nil && current.forward[i].key < key {
			current = current.forward[i]
		}
	}
	
	current = current.forward[0]
	
	if current != nil && current.key == key {
		// Check TTL
		if current.ttl != nil && time.Now().After(*current.ttl) {
			return nil, false
		}
		return current.value, true
	}
	
	return nil, false
}

// Delete removes a key
func (mt *Memtable) Delete(key string) bool {
	mt.mu.Lock()
	defer mt.mu.Unlock()
	
	update := make([]*SkipListNode, maxLevel)
	current := mt.header
	
	for i := mt.level; i >= 0; i-- {
		for current.forward[i] != nil && current.forward[i].key < key {
			current = current.forward[i]
		}
		update[i] = current
	}
	
	current = current.forward[0]
	
	if current != nil && current.key == key {
		for i := 0; i <= mt.level; i++ {
			if update[i].forward[i] != current {
				break
			}
			update[i].forward[i] = current.forward[i]
		}
		
		// Update level
		for mt.level > 0 && mt.header.forward[mt.level] == nil {
			mt.level--
		}
		
		mt.count--
		mt.size -= int64(len(key)) + mt.estimateValueSize(current.value)
		return true
	}
	
	return false
}

// Range iterates over keys with given prefix
func (mt *Memtable) Range(prefix string, fn func(key string, value interface{}) bool) {
	mt.mu.RLock()
	defer mt.mu.RUnlock()
	
	current := mt.header.forward[0]
	
	for current != nil {
		if strings.HasPrefix(current.key, prefix) {
			// Check TTL
			if current.ttl == nil || time.Now().Before(*current.ttl) {
				if !fn(current.key, current.value) {
					break
				}
			}
		}
		current = current.forward[0]
	}
}

// Size returns the current size in bytes
func (mt *Memtable) Size() int64 {
	mt.mu.RLock()
	defer mt.mu.RUnlock()
	return mt.size
}

// Count returns the number of entries
func (mt *Memtable) Count() int64 {
	mt.mu.RLock()
	defer mt.mu.RUnlock()
	return mt.count
}

// IsEmpty returns true if memtable is empty
func (mt *Memtable) IsEmpty() bool {
	mt.mu.RLock()
	defer mt.mu.RUnlock()
	return mt.count == 0
}

// Helper methods

func (mt *Memtable) randomLevel() int {
	level := 0
	for level < maxLevel-1 && mt.rand.Float64() < probability {
		level++
	}
	return level
}

func (mt *Memtable) estimateValueSize(value interface{}) int64 {
	// Simplified size estimation
	switch v := value.(type) {
	case string:
		return int64(len(v))
	case []byte:
		return int64(len(v))
	case int, int32, int64, float32, float64:
		return 8
	case bool:
		return 1
	default:
		return 64 // rough estimate for complex types
	}
}
