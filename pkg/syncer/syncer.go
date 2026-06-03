// Package syncer provides the core file synchronization engine.
// It uses a worker pool of M goroutines to process sync events concurrently.
// Events for the same file are routed to the same worker via consistent hashing
// to preserve operation ordering.
package syncer

import (
	"fmt"
	"hash/fnv"
	"io"
	"os"
	"path/filepath"
)

// SyncTask represents a single file sync operation.
type SyncTask struct {
	Type    string // "copy", "delete", "rename"
	SrcPath string
	DstPath string
	NewPath string // Used for rename
	IsDir   bool
}

// FileSyncer handles actual file system operations.
type FileSyncer struct {
	backupEnabled bool
	verifyEnabled bool
}

// New creates a FileSyncer.
func New(backup, verify bool) *FileSyncer {
	return &FileSyncer{
		backupEnabled: backup,
		verifyEnabled: verify,
	}
}

// Execute performs a single sync operation.
func (s *FileSyncer) Execute(task SyncTask) error {
	switch task.Type {
	case "copy":
		return s.copyFile(task.SrcPath, task.DstPath)
	case "delete":
		return s.deleteFile(task.DstPath)
	case "rename":
		return s.renameFile(task.SrcPath, task.DstPath, task.NewPath)
	default:
		return fmt.Errorf("unknown sync task type: %s", task.Type)
	}
}

// copyFile copies a file from src to dst, creating intermediate directories.
func (s *FileSyncer) copyFile(src, dst string) error {
	// Ensure destination directory exists
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return fmt.Errorf("create target dir: %w", err)
	}

	// Backup existing file if enabled
	if s.backupEnabled {
		if _, err := os.Stat(dst); err == nil {
			s.backupFile(dst)
		}
	}

	srcFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("open source file: %w", err)
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("create target file: %w", err)
	}
	defer dstFile.Close()

	written, err := io.Copy(dstFile, srcFile)
	if err != nil {
		return fmt.Errorf("copy data: %w", err)
	}

	// Verify if enabled
	if s.verifyEnabled {
		srcHash, err := ComputeHash(src)
		if err != nil {
			return fmt.Errorf("verify source hash: %w", err)
		}
		dstHash, err := ComputeHash(dst)
		if err != nil {
			return fmt.Errorf("verify target hash: %w", err)
		}
		if srcHash != dstHash {
			return fmt.Errorf("hash mismatch after copy: %s != %s", srcHash, dstHash)
		}
	}

	// Preserve file mode
	srcInfo, err := os.Stat(src)
	if err == nil {
		os.Chmod(dst, srcInfo.Mode())
	}

	fmt.Printf("[syncer] copied %s -> %s (%d bytes)\n", src, dst, written)
	return nil
}

// deleteFile removes a file or directory.
func (s *FileSyncer) deleteFile(path string) error {
	if err := os.RemoveAll(path); err != nil {
		return fmt.Errorf("delete %s: %w", path, err)
	}
	fmt.Printf("[syncer] deleted %s\n", path)
	return nil
}

// renameFile handles rename operation (delete old target if exists, then copy new).
func (s *FileSyncer) renameFile(src, oldDst, newDst string) error {
	// Delete the old target (if it exists)
	if _, err := os.Stat(oldDst); err == nil {
		os.RemoveAll(oldDst)
	}

	// Copy the new source to new target
	return s.copyFile(src, newDst)
}

// backupFile creates a backup of an existing file before overwriting.
func (s *FileSyncer) backupFile(path string) {
	backupPath := path + ".bak"
	if err := copyFileSimple(path, backupPath); err != nil {
		fmt.Fprintf(os.Stderr, "[syncer] backup failed for %s: %v\n", path, err)
	}
}

// copyFileSimple is a simple file copy for backup purposes.
func copyFileSimple(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	return err
}

// RouteToWorker returns a worker index for a given file path using consistent hashing.
// This ensures the same file always goes to the same worker for ordered processing.
func RouteToWorker(relPath string, workerCount int) int {
	h := fnv.New32a()
	h.Write([]byte(relPath))
	return int(h.Sum32()) % workerCount
}
