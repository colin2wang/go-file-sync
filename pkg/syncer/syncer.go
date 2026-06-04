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
	"strings"
	"time"
)

// SyncTask represents a single file sync operation.
type SyncTask struct {
	Type    string // "copy", "delete", "rename"
	SrcPath string
	DstPath string
	NewPath string // Used for rename
	IsDir   bool
	Size    int64
}

// FileSyncer handles actual file system operations.
type FileSyncer struct {
	backupEnabled      bool
	verifyEnabled      bool
	dryRun             bool
	preservePerms      bool
	preserveOwner      bool
	preserveTimes      bool
	symlinks           string
	bandwidthLimit     int64
	conflictDetection  bool
	conflictResolution string
}

// New creates a FileSyncer.
func New(backup, verify bool) *FileSyncer {
	return &FileSyncer{
		backupEnabled:      backup,
		verifyEnabled:      verify,
		symlinks:           "follow",
		conflictResolution: "warn",
	}
}

// SetDryRun enables dry-run mode (no actual file operations).
func (s *FileSyncer) SetDryRun(v bool) { s.dryRun = v }

// SetPreservePermissions enables permission preservation.
func (s *FileSyncer) SetPreservePermissions(v bool) { s.preservePerms = v }

// SetPreserveOwner enables owner preservation.
func (s *FileSyncer) SetPreserveOwner(v bool) { s.preserveOwner = v }

// SetPreserveTimestamps enables timestamp preservation.
func (s *FileSyncer) SetPreserveTimestamps(v bool) { s.preserveTimes = v }

// SetSymlinks sets the symlink handling mode (follow/copy/skip).
func (s *FileSyncer) SetSymlinks(mode string) { s.symlinks = mode }

// SetBandwidthLimit sets the bandwidth limit in bytes/sec (0 = unlimited).
func (s *FileSyncer) SetBandwidthLimit(limit int64) { s.bandwidthLimit = limit }

// SetConflictDetection enables conflict detection.
func (s *FileSyncer) SetConflictDetection(v bool) { s.conflictDetection = v }

// SetConflictResolution sets the conflict resolution mode.
func (s *FileSyncer) SetConflictResolution(mode string) { s.conflictResolution = mode }

// Execute performs a single sync operation.
func (s *FileSyncer) Execute(task SyncTask) (int64, error) {
	if s.dryRun {
		switch task.Type {
		case "copy":
			fmt.Printf("[DRY-RUN] would copy %s -> %s\n", task.SrcPath, task.DstPath)
		case "delete":
			fmt.Printf("[DRY-RUN] would delete %s\n", task.DstPath)
		case "rename":
			fmt.Printf("[DRY-RUN] would rename %s -> %s\n", task.SrcPath, task.NewPath)
		}
		return 0, nil
	}

	switch task.Type {
	case "copy":
		return s.copyFile(task.SrcPath, task.DstPath)
	case "delete":
		return 0, s.deleteFile(task.DstPath)
	case "rename":
		return 0, s.renameFile(task.SrcPath, task.DstPath, task.NewPath)
	default:
		return 0, fmt.Errorf("unknown sync task type: %s", task.Type)
	}
}

// copyFile copies a file from src to dst, creating intermediate directories.
func (s *FileSyncer) copyFile(src, dst string) (int64, error) {
	// Handle symlinks
	srcInfo, err := os.Lstat(src)
	if err != nil {
		return 0, fmt.Errorf("stat source file: %w", err)
	}

	if srcInfo.Mode()&os.ModeSymlink != 0 {
		switch s.symlinks {
		case "skip":
			fmt.Printf("[syncer] skipping symlink %s\n", src)
			return 0, nil
		case "copy":
			return s.copySymlink(src, dst)
		case "follow":
			// Resolve symlink and copy the target
			realPath, err := os.Readlink(src)
			if err != nil {
				return 0, fmt.Errorf("read symlink: %w", err)
			}
			if !filepath.IsAbs(realPath) {
				realPath = filepath.Join(filepath.Dir(src), realPath)
			}
			src = realPath
		}
	}

	// Ensure destination directory exists
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return 0, fmt.Errorf("create target dir: %w", err)
	}

	// Backup existing file if enabled
	if s.backupEnabled {
		if _, err := os.Stat(dst); err == nil {
			s.backupFile(dst)
		}
	}

	srcFile, err := os.Open(src)
	if err != nil {
		return 0, fmt.Errorf("open source file: %w", err)
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return 0, fmt.Errorf("create target file: %w", err)
	}
	defer dstFile.Close()

	var written int64
	if s.bandwidthLimit > 0 {
		written, err = s.copyWithLimit(dstFile, srcFile)
	} else {
		written, err = io.Copy(dstFile, srcFile)
	}
	if err != nil {
		return 0, fmt.Errorf("copy data: %w", err)
	}

	// Preserve attributes
	s.preserveAttrs(src, dst)

	// Verify if enabled
	if s.verifyEnabled {
		if err := s.verifyHash(src, dst); err != nil {
			return written, fmt.Errorf("verify failed: %w", err)
		}
	}

	fmt.Printf("[syncer] copied %s -> %s (%d bytes)\n", src, dst, written)
	return written, nil
}

// copySymlink creates a symlink at dst pointing to the same target as src.
func (s *FileSyncer) copySymlink(src, dst string) (int64, error) {
	target, err := os.Readlink(src)
	if err != nil {
		return 0, fmt.Errorf("read symlink: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return 0, fmt.Errorf("create target dir: %w", err)
	}
	// Remove existing file if any
	os.Remove(dst)
	if err := os.Symlink(target, dst); err != nil {
		return 0, fmt.Errorf("create symlink: %w", err)
	}
	fmt.Printf("[syncer] copied symlink %s -> %s -> %s\n", src, dst, target)
	return 0, nil
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
	_, err := s.copyFile(src, newDst)
	return err
}

// backupFile creates a backup of an existing file before overwriting.
func (s *FileSyncer) backupFile(path string) {
	backupPath := path + "." + time.Now().Format("150405.000") + ".bak"
	if err := copyFileSimple(path, backupPath); err != nil {
		fmt.Fprintf(os.Stderr, "[syncer] backup failed for %s: %v\n", path, err)
	}
}

// preserveAttrs preserves file permissions and timestamps if configured.
func (s *FileSyncer) preserveAttrs(src, dst string) {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return
	}
	if s.preservePerms {
		os.Chmod(dst, srcInfo.Mode())
	}
	if s.preserveTimes {
		os.Chtimes(dst, srcInfo.ModTime(), srcInfo.ModTime())
	}
}

// verifyHash checks that src and dst have the same SHA-256 hash.
func (s *FileSyncer) verifyHash(src, dst string) error {
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
	return nil
}

// copyWithLimit copies data with a bandwidth limit.
func (s *FileSyncer) copyWithLimit(dst io.Writer, src io.Reader) (int64, error) {
	buf := make([]byte, 32*1024)
	var total int64
	limit := s.bandwidthLimit
	start := time.Now()

	for {
		nr, er := src.Read(buf)
		if nr > 0 {
			nw, ew := dst.Write(buf[:nr])
			total += int64(nw)
			if ew != nil {
				return total, ew
			}
			if nr != nw {
				return total, io.ErrShortWrite
			}

			// Rate limiting: ensure we don't exceed bytes/sec
			if limit > 0 {
				elapsed := time.Since(start)
				expected := float64(total) / float64(limit)
				if expected > elapsed.Seconds() {
					time.Sleep(time.Duration(expected-elapsed.Seconds()) * time.Second)
				}
			}
		}
		if er != nil {
			if er == io.EOF {
				break
			}
			return total, er
		}
	}
	return total, nil
}

// ShouldSkipSymlink checks if a symlink should be skipped based on the mode.
func (s *FileSyncer) ShouldSkipSymlink(mode os.FileMode) bool {
	if mode&os.ModeSymlink == 0 {
		return false
	}
	return strings.ToLower(s.symlinks) == "skip"
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
