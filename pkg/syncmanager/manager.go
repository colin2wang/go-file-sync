package syncmanager

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"go-file-sync/pkg/configdb"
	"go-file-sync/pkg/syncer"
)

// Manager manages sync tasks from the SQLite database.
type Manager struct {
	db     *configdb.ConfigDB
	pool   *syncer.WorkerPool
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
	tasks  map[int64]*runningTask
	mu     sync.RWMutex

	// Statistics - track unique files
	monitoredFiles map[string]bool
	syncedFiles    map[string]bool
	lastReset      time.Time
	statsMu        sync.RWMutex
}

type runningTask struct {
	task   *configdb.SyncTask
	cancel context.CancelFunc
}

// Stats holds sync statistics for the current minute.
type Stats struct {
	MonitoredFiles int64 `json:"monitored_files"`
	SyncedFiles    int64 `json:"synced_files"`
}

// NewManager creates a new sync manager.
func NewManager(db *configdb.ConfigDB) *Manager {
	ctx, cancel := context.WithCancel(context.Background())
	return &Manager{
		db:             db,
		ctx:            ctx,
		cancel:         cancel,
		tasks:          make(map[int64]*runningTask),
		monitoredFiles: make(map[string]bool),
		syncedFiles:    make(map[string]bool),
		lastReset:      time.Now(),
	}
}

// Start starts the sync manager and all enabled tasks.
func (m *Manager) Start() {
	// Create worker pool
	m.pool = syncer.NewPool(4, false, false)
	m.pool.Start()

	// Load and start enabled tasks
	tasks, err := m.db.ListTasks()
	if err != nil {
		log.Printf("Failed to load tasks: %v", err)
		return
	}

	for i := range tasks {
		if tasks[i].Enabled {
			m.startTask(&tasks[i])
		}
	}

	log.Printf("Sync manager started with %d enabled tasks", len(tasks))
}

// Stop stops all running tasks and the worker pool.
func (m *Manager) Stop() {
	m.cancel()
	m.wg.Wait()
	if m.pool != nil {
		m.pool.Stop()
	}
}

// GetStats returns the current sync statistics and resets counters if needed.
func (m *Manager) GetStats() Stats {
	m.statsMu.Lock()
	defer m.statsMu.Unlock()

	// Reset counters if more than 1 minute has passed
	if time.Since(m.lastReset) >= time.Minute {
		monitored := int64(len(m.monitoredFiles))
		synced := int64(len(m.syncedFiles))
		m.monitoredFiles = make(map[string]bool)
		m.syncedFiles = make(map[string]bool)
		m.lastReset = time.Now()
		return Stats{
			MonitoredFiles: monitored,
			SyncedFiles:    synced,
		}
	}

	return Stats{
		MonitoredFiles: int64(len(m.monitoredFiles)),
		SyncedFiles:    int64(len(m.syncedFiles)),
	}
}

// ResetStats resets the statistics counters.
func (m *Manager) ResetStats() {
	m.statsMu.Lock()
	defer m.statsMu.Unlock()
	m.monitoredFiles = make(map[string]bool)
	m.syncedFiles = make(map[string]bool)
	m.lastReset = time.Now()
}

// TaskUpdated is called when a task is created, updated, or deleted.
func (m *Manager) TaskUpdated(taskID int64, enabled bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if enabled {
		task, err := m.db.GetTask(taskID)
		if err != nil {
			log.Printf("Failed to get task %d: %v", taskID, err)
			return
		}
		m.startTask(task)
	} else {
		m.stopTask(taskID)
	}
}

func (m *Manager) startTask(task *configdb.SyncTask) {
	// Stop existing task if running
	if rt, ok := m.tasks[task.ID]; ok {
		rt.cancel()
	}

	taskCtx, taskCancel := context.WithCancel(m.ctx)
	rt := &runningTask{
		task:   task,
		cancel: taskCancel,
	}
	m.tasks[task.ID] = rt

	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
		m.runTaskLoop(taskCtx, task)
	}()

	log.Printf("Started sync task: %s (every %ds)", task.Name, task.MonitorInterval)
}

func (m *Manager) stopTask(taskID int64) {
	if rt, ok := m.tasks[taskID]; ok {
		rt.cancel()
		delete(m.tasks, taskID)
		log.Printf("Stopped sync task: %s", rt.task.Name)
	}
}

func (m *Manager) runTaskLoop(ctx context.Context, task *configdb.SyncTask) {
	ticker := time.NewTicker(time.Duration(task.MonitorInterval) * time.Second)
	defer ticker.Stop()

	// Perform initial sync
	m.performSync(task)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.performSync(task)
		}
	}
}

func (m *Manager) performSync(task *configdb.SyncTask) {
	source := task.SourcePath
	target := task.TargetPath

	// Ensure source exists
	sourceInfo, err := os.Stat(source)
	if os.IsNotExist(err) {
		m.logSync(task, "failed", source, fmt.Sprintf("Source does not exist: %s", source))
		return
	}

	// If source is a file, sync just that file
	if !sourceInfo.IsDir() {
		m.syncFile(task, source, target)
		return
	}

	// If source is a directory, walk and sync all files
	err = filepath.Walk(source, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Calculate relative path
		relPath, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}

		// Skip directories
		if info.IsDir() {
			return nil
		}

		// Determine target path
		dstPath := filepath.Join(target, relPath)

		// Ensure target directory exists
		dstDir := filepath.Dir(dstPath)
		if err := os.MkdirAll(dstDir, 0755); err != nil {
			return err
		}

		// Sync the file
		m.syncFile(task, path, dstPath)
		return nil
	})

	if err != nil {
		m.logSync(task, "failed", source, fmt.Sprintf("Walk error: %v", err))
	}
}

func (m *Manager) syncFile(task *configdb.SyncTask, srcPath, dstPath string) {
	srcInfo, err := os.Stat(srcPath)
	if err != nil {
		m.logSync(task, "failed", srcPath, fmt.Sprintf("Failed to stat source: %v", err))
		return
	}

	// Track monitored files (unique by source path)
	m.statsMu.Lock()
	m.monitoredFiles[srcPath] = true
	m.statsMu.Unlock()

	// Ensure target directory exists
	dstDir := filepath.Dir(dstPath)
	if err := os.MkdirAll(dstDir, 0755); err != nil {
		m.logSync(task, "failed", srcPath, fmt.Sprintf("Failed to create target dir: %v", err))
		return
	}

	// Check if we need to sync based on direction
	shouldSync := false
	switch task.SyncDirection {
	case "one_way_upload":
		shouldSync = true // Always sync from source to target
	case "one_way_download":
		// Check if source is newer than target
		dstInfo, err := os.Stat(dstPath)
		if os.IsNotExist(err) {
			shouldSync = true
		} else if err == nil && srcInfo.ModTime().After(dstInfo.ModTime()) {
			shouldSync = true
		}
	case "two_way":
		// Sync newer file
		dstInfo, err := os.Stat(dstPath)
		if os.IsNotExist(err) {
			shouldSync = true
		} else if err == nil && srcInfo.ModTime().After(dstInfo.ModTime()) {
			shouldSync = true
		}
	}

	if shouldSync {
		relPath, _ := filepath.Rel(filepath.Dir(srcPath), srcPath)
		if err := copyFile(srcPath, dstPath); err != nil {
			m.logSync(task, "failed", srcPath, fmt.Sprintf("Failed to copy %s: %v", relPath, err))
		} else {
			// Track synced files (unique by source path)
			m.statsMu.Lock()
			m.syncedFiles[srcPath] = true
			m.statsMu.Unlock()
			m.logSync(task, "synced", srcPath, relPath)
		}
	}
}

func (m *Manager) logSync(task *configdb.SyncTask, status, filePath, message string) {
	if err := m.db.LogSync(task.ID, task.Name, "sync", filePath, status, message); err != nil {
		log.Printf("Failed to log sync: %v", err)
	}
}

func copyFile(src, dst string) error {
	// Read source file
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	// Get source file info
	srcInfo, err := srcFile.Stat()
	if err != nil {
		return err
	}

	// Create target file
	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	// Copy content
	buf := make([]byte, 64*1024)
	for {
		n, err := srcFile.Read(buf)
		if n > 0 {
			if _, writeErr := dstFile.Write(buf[:n]); writeErr != nil {
				return writeErr
			}
		}
		if err != nil {
			break
		}
	}

	// Preserve timestamps
	return os.Chtimes(dst, srcInfo.ModTime(), srcInfo.ModTime())
}
