// Package history provides an audit log for sync operations.
// Entries are written as newline-delimited JSON (JSONL) for easy
// programmatic processing.
package history

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"
)

// Entry represents a single sync history record.
type Entry struct {
	Time   string `json:"time"`
	Task   string `json:"task"`
	Event  string `json:"event"`
	File   string `json:"file"`
	Size   int64  `json:"size,omitempty"`
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
}

// Writer manages the history log file with thread-safe writes.
type Writer struct {
	mu         sync.Mutex
	file       *os.File
	encoder    *json.Encoder
	maxEntries int
	count      int
	path       string
}

// New creates a new history writer. If the file already exists, it
// appends to it (or truncates if maxEntries would be exceeded).
func New(path string, maxEntries int) (*Writer, error) {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, fmt.Errorf("open history file %s: %w", path, err)
	}

	return &Writer{
		file:       f,
		encoder:    json.NewEncoder(f),
		maxEntries: maxEntries,
		path:       path,
	}, nil
}

// Write records a sync operation to the history log.
func (w *Writer) Write(task, event, file string, size int64, status, errStr string) {
	entry := Entry{
		Time:   time.Now().UTC().Format(time.RFC3339),
		Task:   task,
		Event:  event,
		File:   file,
		Size:   size,
		Status: status,
		Error:  errStr,
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	w.encoder.Encode(entry)
	w.count++

	if w.maxEntries > 0 && w.count >= w.maxEntries {
		w.rotate()
	}
}

// Close closes the history file.
func (w *Writer) Close() error {
	return w.file.Close()
}

// rotate truncates the file and resets the counter.
func (w *Writer) rotate() {
	w.file.Close()
	// Truncate by reopening
	f, err := os.Create(w.path)
	if err != nil {
		return
	}
	w.file = f
	w.encoder = json.NewEncoder(f)
	w.count = 0
}
