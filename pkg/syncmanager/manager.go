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
	syncer *syncer.FileSyncer
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
	// Create the shared worker pool and route results back to DB logging/stats.
	m.pool = syncer.NewPool(4)
	m.pool.SetResultHandler(m.onSyncResult)
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

func (m *Manager) buildSyncer(task *configdb.SyncTask) *syncer.FileSyncer {
	fs := syncer.New(task.Backup, task.Verify)
	fs.SetPreservePermissions(task.PreservePerms)
	fs.SetPreserveOwner(task.PreserveOwner)
	fs.SetPreserveTimestamps(task.PreserveTimes)
	if task.Symlinks != "" {
		fs.SetSymlinks(task.Symlinks)
	}
	fs.SetBandwidthLimit(task.BandwidthLimit)
	fs.SetConflictDetection(task.ConflictDetection)
	fs.SetConflictResolution(task.ConflictResolution)
	return fs
}

func (m *Manager) startTask(task *configdb.SyncTask) {
	// Stop existing task if running
	if rt, ok := m.tasks[task.ID]; ok {
		rt.cancel()
	}

	taskCtx, taskCancel := context.WithCancel(m.ctx)
	rt := &runningTask{
		task:   task,
		syncer: m.buildSyncer(task),
		cancel: taskCancel,
	}
	m.tasks[task.ID] = rt

	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
		m.runTaskLoop(taskCtx, rt)
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

func (m *Manager) runTaskLoop(ctx context.Context, rt *runningTask) {
	ticker := time.NewTicker(time.Duration(rt.task.MonitorInterval) * time.Second)
	defer ticker.Stop()

	// Perform initial sync
	m.performSync(rt)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.performSync(rt)
		}
	}
}

func (m *Manager) performSync(rt *runningTask) {
	task := rt.task
	source := task.SourcePath
	target := task.TargetPath

	// Ensure source exists
	sourceInfo, err := os.Stat(source)
	if os.IsNotExist(err) {
		m.logSync(task.ID, task.Name, "failed", source, fmt.Sprintf("Source does not exist: %s", source))
		return
	}

	// If source is a file, sync just that file
	if !sourceInfo.IsDir() {
		var wg sync.WaitGroup
		m.syncFile(rt, source, target, "", &wg)
		wg.Wait()
		return
	}

	// If source is a directory, walk and sync all files concurrently via the pool.
	var wg sync.WaitGroup
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
		m.syncFile(rt, path, dstPath, relPath, &wg)
		return nil
	})

	wg.Wait()

	if err != nil {
		m.logSync(task.ID, task.Name, "failed", source, fmt.Sprintf("Walk error: %v", err))
	}
}

// syncFile resolves the actual copy direction, then submits the work to the
// worker pool. The WaitGroup is signalled once the job has been processed.
func (m *Manager) syncFile(rt *runningTask, srcPath, dstPath, relPath string, wg *sync.WaitGroup) {
	// Track monitored files (unique by source path).
	m.statsMu.Lock()
	m.monitoredFiles[srcPath] = true
	m.statsMu.Unlock()

	wg.Add(1)

	from, to, shouldSync, err := m.resolveDirection(rt.task, srcPath, dstPath)
	if err != nil {
		m.logSync(rt.task.ID, rt.task.Name, "failed", srcPath, err.Error())
		wg.Done()
		return
	}
	if !shouldSync {
		wg.Done()
		return
	}

	task := syncer.SyncTask{
		Type:     "copy",
		SrcPath:  from,
		DstPath:  to,
		IsDir:    false,
		TaskID:   rt.task.ID,
		TaskName: rt.task.Name,
		RelPath:  relPath,
	}
	job := syncer.Job{
		Task:   task,
		Syncer: rt.syncer,
		Done:   wg.Done,
	}
	m.pool.SubmitForPath(job, relPath)
}

// resolveDirection determines the actual copy direction based on SyncDirection.
//   - one_way_upload:   source is master -> always copy src -> dst
//   - one_way_download: target is master -> always copy dst -> src
//   - two_way:          copy whichever side is newer
//
// It returns (from, to, shouldSync, error).
func (m *Manager) resolveDirection(task *configdb.SyncTask, srcPath, dstPath string) (string, string, bool, error) {
	switch task.SyncDirection {
	case "one_way_download":
		if _, err := os.Stat(dstPath); err != nil {
			return "", "", false, fmt.Errorf("failed to stat target %s: %v", dstPath, err)
		}
		return dstPath, srcPath, true, nil

	case "two_way":
		srcInfo, err := os.Stat(srcPath)
		if os.IsNotExist(err) {
			if _, e := os.Stat(dstPath); e == nil {
				return dstPath, srcPath, true, nil
			}
			return "", "", false, nil
		} else if err != nil {
			return "", "", false, err
		}
		dstInfo, err := os.Stat(dstPath)
		if os.IsNotExist(err) {
			return srcPath, dstPath, true, nil
		} else if err != nil {
			return "", "", false, err
		}
		if srcInfo.ModTime().After(dstInfo.ModTime()) {
			return srcPath, dstPath, true, nil
		}
		if dstInfo.ModTime().After(srcInfo.ModTime()) {
			return dstPath, srcPath, true, nil
		}
		return "", "", false, nil // identical

	default: // one_way_upload
		if _, err := os.Stat(srcPath); err != nil {
			return "", "", false, fmt.Errorf("failed to stat source %s: %v", srcPath, err)
		}
		return srcPath, dstPath, true, nil
	}
}

// onSyncResult is invoked by the worker pool for every processed job.
func (m *Manager) onSyncResult(task syncer.SyncTask, size int64, err error) {
	if err != nil {
		m.logSync(task.TaskID, task.TaskName, "failed", task.SrcPath,
			fmt.Sprintf("Failed to sync %s: %v", task.RelPath, err))
		return
	}
	m.statsMu.Lock()
	m.syncedFiles[task.SrcPath] = true
	m.statsMu.Unlock()
	m.logSync(task.TaskID, task.TaskName, "synced", task.SrcPath, task.RelPath)
}

func (m *Manager) logSync(taskID int64, taskName, status, filePath, message string) {
	if err := m.db.LogSync(taskID, taskName, "sync", filePath, status, message); err != nil {
		log.Printf("Failed to log sync: %v", err)
	}
}
