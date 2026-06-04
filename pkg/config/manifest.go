// Package config provides configuration parsing and management.
// Manifest sub-package provides conflict detection by maintaining a checksum
// manifest at the target location. Each synced file's SHA-256 hash and
// timestamp are recorded to detect out-of-band modifications.
package config

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Entry represents a single file's manifest record.
type Entry struct {
	Hash     string    `json:"hash"`
	SyncedAt time.Time `json:"synced_at"`
}

// Manifest maps relative file paths to their hash entries.
type Manifest struct {
	mu      sync.RWMutex
	entries map[string]Entry
	path    string
}

// New creates or loads a manifest from the given path.
func New(path string) (*Manifest, error) {
	m := &Manifest{
		entries: make(map[string]Entry),
		path:    path,
	}
	data, err := os.ReadFile(path)
	if err == nil {
		if err := json.Unmarshal(data, &m.entries); err != nil {
			return nil, fmt.Errorf("parse manifest %s: %w", path, err)
		}
	}
	return m, nil
}

// Get returns the entry for a given relative path.
func (m *Manifest) Get(relPath string) (Entry, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	e, ok := m.entries[relPath]
	return e, ok
}

// Set records a file's hash and timestamp.
func (m *Manifest) Set(relPath, filePath string) error {
	hash, err := computeHash(filePath)
	if err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.entries[relPath] = Entry{
		Hash:     hash,
		SyncedAt: time.Now(),
	}
	return nil
}

// Remove deletes a file from the manifest.
func (m *Manifest) Remove(relPath string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.entries, relPath)
}

// CheckConflict returns true if the target file has been modified since
// the last sync (hash differs from manifest).
func (m *Manifest) CheckConflict(relPath, targetPath string) (bool, error) {
	entry, ok := m.Get(relPath)
	if !ok {
		return false, nil // No record, no conflict detected
	}
	currentHash, err := computeHash(targetPath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil // Target doesn't exist, no conflict
		}
		return false, err
	}
	return currentHash != entry.Hash, nil
}

// Save writes the manifest to disk.
func (m *Manifest) Save() error {
	m.mu.RLock()
	data, err := json.MarshalIndent(m.entries, "", "  ")
	m.mu.RUnlock()
	if err != nil {
		return fmt.Errorf("marshal manifest: %w", err)
	}
	dir := filepath.Dir(m.path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create manifest dir: %w", err)
	}
	return os.WriteFile(m.path, data, 0644)
}

func computeHash(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}
