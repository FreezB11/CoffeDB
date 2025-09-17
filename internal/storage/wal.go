package storage

import (
	"bufio"
	"encoding/gob"
	"fmt"
	"os"
	"sync"
	"time"
)

// Add this function
func init() {
    // Register the Document type with gob
    gob.Register(&Document{})
}


// WALEntryType represents the type of WAL entry
type WALEntryType int

const (
	WALPut WALEntryType = iota
	WALDelete
	WALTransaction
)

// WALEntry represents an entry in the write-ahead log
type WALEntry struct {
	Type      WALEntryType  `json:"type"`
	Key       string        `json:"key"`
	Value     interface{}   `json:"value,omitempty"`
	Timestamp time.Time     `json:"timestamp"`
	TxnID     string        `json:"txn_id,omitempty"`
}

// WAL represents the write-ahead log
type WAL struct {
	file   *os.File
	writer *bufio.Writer
	mu     sync.Mutex
}

// NewWAL creates a new write-ahead log
func NewWAL(filename string) (*WAL, error) {
	file, err := os.OpenFile(filename, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open WAL file: %w", err)
	}

	return &WAL{
		file:   file,
		writer: bufio.NewWriter(file),
	}, nil
}

// WriteEntry writes an entry to the WAL
func (w *WAL) WriteEntry(entry WALEntry) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	encoder := gob.NewEncoder(w.writer)
	if err := encoder.Encode(entry); err != nil {
		return fmt.Errorf("failed to encode WAL entry: %w", err)
	}

	// Flush to ensure durability
	if err := w.writer.Flush(); err != nil {
		return fmt.Errorf("failed to flush WAL: %w", err)
	}

	if err := w.file.Sync(); err != nil {
		return fmt.Errorf("failed to sync WAL: %w", err)
	}

	return nil
}

// ReadEntries reads all entries from the WAL for recovery
func (w *WAL) ReadEntries() ([]WALEntry, error) {
	// Open file for reading
	file, err := os.Open(w.file.Name())
	if err != nil {
		return nil, fmt.Errorf("failed to open WAL for reading: %w", err)
	}
	defer file.Close()

	var entries []WALEntry
	decoder := gob.NewDecoder(file)

	for {
		var entry WALEntry
		if err := decoder.Decode(&entry); err != nil {
			break // EOF or error
		}
		entries = append(entries, entry)
	}

	return entries, nil
}

// Close closes the WAL
func (w *WAL) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if err := w.writer.Flush(); err != nil {
		return err
	}

	return w.file.Close()
}
