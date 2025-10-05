package storage

/*
definitions
func NewBTree(filename string) (*BTree, error)
func (bt *BTree) Put(key string, value interface{}) error
func (bt *BTree) Get(key string) (interface{}, error)
func (bt *BTree) Delete(key string) error
func (bt *BTree) Range(prefix string) ([]interface{}, error) 
func (bt *BTree) insert(node *BTreeNode, key string, value interface{}) error
func (bt *BTree) insertIntoLeaf(node *BTreeNode, key string, value interface{}) error
func (bt *BTree) search(node *BTreeNode, key string) (interface{}, error)
func (bt *BTree) delete(node *BTreeNode, key string) error
func (bt *BTree) deleteFromInternal(node *BTreeNode, pos int) error
func (bt *BTree) rangeSearch(node *BTreeNode, prefix string, results *[]interface{}) 
func (bt *BTree) findChildIndex(node *BTreeNode, key string) int
func (bt *BTree) splitChild(parent *BTreeNode, childIndex int) error
func (bt *BTree) loadRoot() error
func (bt *BTree) Close() error
func (bt *BTree) flush() error
*/

import (
	"encoding/gob" // for compression tbh
	"fmt"
	"os"
	"sort"
	"sync"
)

const (
	btreeOrder = 256  // B-tree order (max children per node)
	nodeSize   = 4096 // Page size in bytes
)

// BTreeNode represents a node in the B-tree
type BTreeNode struct {
	IsLeaf   bool
	Keys     []string
	Values   []interface{}
	Children []*BTreeNode
	Parent   *BTreeNode
	Modified bool
}

// BTree represents a disk-based B-tree
type BTree struct {
	root     *BTreeNode
	file     *os.File
	mu       sync.RWMutex
	nodePool map[int64]*BTreeNode
}

// NewBTree creates a new B-tree
func NewBTree(filename string) (*BTree, error) {
	file, err := os.OpenFile(filename, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return nil, err
	}

	tree := &BTree{
		file:     file,
		nodePool: make(map[int64]*BTreeNode),
	}

	// Load or create root node
	if err := tree.loadRoot(); err != nil {
		return nil, err
	}

	return tree, nil
}

// Put inserts a key-value pair
func (bt *BTree) Put(key string, value interface{}) error {
	bt.mu.Lock()
	defer bt.mu.Unlock()

	if bt.root == nil {
		bt.root = &BTreeNode{
			IsLeaf: true,
			Keys:   []string{key},
			Values: []interface{}{value},
		}
		return nil
	}

	return bt.insert(bt.root, key, value)
}

// Get retrieves a value by key
func (bt *BTree) Get(key string) (interface{}, error) {
	bt.mu.RLock()
	defer bt.mu.RUnlock()

	if bt.root == nil {
		return nil, fmt.Errorf("key not found")
	}

	return bt.search(bt.root, key)
}

// Delete removes a key-value pair
func (bt *BTree) Delete(key string) error {
	bt.mu.Lock()
	defer bt.mu.Unlock()

	if bt.root == nil {
		return fmt.Errorf("key not found")
	}

	return bt.delete(bt.root, key)
}

// Range returns all values with keys having the given prefix
func (bt *BTree) Range(prefix string) ([]interface{}, error) {
	bt.mu.RLock()
	defer bt.mu.RUnlock()
	var results []interface{}
	bt.rangeSearch(bt.root, prefix, &results)
	return results, nil
}

// Internal methods

func (bt *BTree) insert(node *BTreeNode, key string, value interface{}) error {
	if node.IsLeaf {
		return bt.insertIntoLeaf(node, key, value)
	}

	// Find child to insert into
	childIndex := bt.findChildIndex(node, key)
	if err := bt.insert(node.Children[childIndex], key, value); err != nil {
		return err
	}

	// Check if child needs splitting
	if len(node.Children[childIndex].Keys) > btreeOrder-1 {
		return bt.splitChild(node, childIndex)
	}

	return nil
}

func (bt *BTree) insertIntoLeaf(node *BTreeNode, key string, value interface{}) error {
	// Find position to insert
	pos := sort.SearchStrings(node.Keys, key)
	
	// If key exists, update value
	if pos < len(node.Keys) && node.Keys[pos] == key {
		node.Values[pos] = value
		node.Modified = true
		return nil
	}

	// Insert new key-value pair
	node.Keys = append(node.Keys, "")
	node.Values = append(node.Values, nil)
	
	copy(node.Keys[pos+1:], node.Keys[pos:])
	copy(node.Values[pos+1:], node.Values[pos:])
	
	node.Keys[pos] = key
	node.Values[pos] = value
	node.Modified = true

	return nil
}

func (bt *BTree) search(node *BTreeNode, key string) (interface{}, error) {
	pos := sort.SearchStrings(node.Keys, key)
	
	if pos < len(node.Keys) && node.Keys[pos] == key {
		return node.Values[pos], nil
	}
	
	if node.IsLeaf {
		return nil, fmt.Errorf("key not found")
	}
	
	return bt.search(node.Children[pos], key)
}

func (bt *BTree) delete(node *BTreeNode, key string) error {
	pos := sort.SearchStrings(node.Keys, key)
	
	if pos < len(node.Keys) && node.Keys[pos] == key {
		if node.IsLeaf {
			// Remove from leaf
			copy(node.Keys[pos:], node.Keys[pos+1:])
			copy(node.Values[pos:], node.Values[pos+1:])
			node.Keys = node.Keys[:len(node.Keys)-1]
			node.Values = node.Values[:len(node.Values)-1]
			node.Modified = true
			return nil
		}
		// Handle internal node deletion (more complex)
		return bt.deleteFromInternal(node, pos)
	}
	
	if node.IsLeaf {
		return fmt.Errorf("key not found")
	}
	
	return bt.delete(node.Children[pos], key)
}

func (bt *BTree) deleteFromInternal(node *BTreeNode, pos int) error {
	// Simplified deletion - in production, this would handle merging/rebalancing
	
	// Find predecessor
	pred := node.Children[pos]
	for !pred.IsLeaf {
		pred = pred.Children[len(pred.Children)-1]
	}
	
	// Replace with predecessor
	predKey := pred.Keys[len(pred.Keys)-1]
	predValue := pred.Values[len(pred.Values)-1]
	
	node.Keys[pos] = predKey
	node.Values[pos] = predValue
	node.Modified = true
	
	// Delete predecessor
	return bt.delete(pred, predKey)
}

func (bt *BTree) rangeSearch(node *BTreeNode, prefix string, results *[]interface{}) {
	if node == nil {
		return
	}
	
	if node.IsLeaf {
		for i, key := range node.Keys {
			if len(key) >= len(prefix) && key[:len(prefix)] == prefix {
				*results = append(*results, node.Values[i])
			}
		}
		return
	}
	
	// Search all children that might contain keys with prefix
	for i, key := range node.Keys {
		if key >= prefix {
			bt.rangeSearch(node.Children[i], prefix, results)
		}
	}
	
	// Check last child
	if len(node.Children) > 0 {
		bt.rangeSearch(node.Children[len(node.Children)-1], prefix, results)
	}
}

func (bt *BTree) findChildIndex(node *BTreeNode, key string) int {
	pos := sort.SearchStrings(node.Keys, key)
	return pos
}

func (bt *BTree) splitChild(parent *BTreeNode, childIndex int) error {
	child := parent.Children[childIndex]
	midIndex := len(child.Keys) / 2
	
	// Create new node
	newNode := &BTreeNode{
		IsLeaf: child.IsLeaf,
		Keys:   append([]string(nil), child.Keys[midIndex+1:]...),
		Values: append([]interface{}(nil), child.Values[midIndex+1:]...),
		Parent: parent,
	}
	
	if !child.IsLeaf {
		newNode.Children = append([]*BTreeNode(nil), child.Children[midIndex+1:]...)
	}
	
	// Update old node
	midKey := child.Keys[midIndex]
	midValue := child.Values[midIndex]
	child.Keys = child.Keys[:midIndex]
	child.Values = child.Values[:midIndex]
	
	if !child.IsLeaf {
		child.Children = child.Children[:midIndex+1]
	}
	
	// Insert middle key into parent
	parent.Keys = append(parent.Keys, "")
	parent.Values = append(parent.Values, nil)
	parent.Children = append(parent.Children, nil)
	
	copy(parent.Keys[childIndex+1:], parent.Keys[childIndex:])
	copy(parent.Values[childIndex+1:], parent.Values[childIndex:])
	copy(parent.Children[childIndex+2:], parent.Children[childIndex+1:])
	
	parent.Keys[childIndex] = midKey
	parent.Values[childIndex] = midValue
	parent.Children[childIndex+1] = newNode
	parent.Modified = true
	
	return nil
}

func (bt *BTree) loadRoot() error {
	// Try to read existing root from file
	stat, err := bt.file.Stat()
	if err != nil {
		return err
	}
	
	if stat.Size() == 0 {
		// Empty file, create new root
		bt.root = &BTreeNode{
			IsLeaf: true,
			Keys:   []string{},
			Values: []interface{}{},
		}
		return nil
	}
	
	// Load root from file (simplified - in production would use proper serialization)
	decoder := gob.NewDecoder(bt.file)
	return decoder.Decode(&bt.root)
}

// Close flushes and closes the B-tree
func (bt *BTree) Close() error {
	bt.mu.Lock()
	defer bt.mu.Unlock()
	
	// Flush to disk
	if err := bt.flush(); err != nil {
		return err
	}
	
	return bt.file.Close()
}

func (bt *BTree) flush() error {
	if bt.root == nil {
		return nil
	}
	
	// Seek to beginning
	if _, err := bt.file.Seek(0, 0); err != nil {
		return err
	}
	
	// Truncate file
	if err := bt.file.Truncate(0); err != nil {
		return err
	}
	
	// Write root to file
	encoder := gob.NewEncoder(bt.file)
	return encoder.Encode(bt.root)
}
