// Package core manages the application lifecycle: initialization, orchestration,
// and graceful shutdown of all goroutines (watchers, debouncers, sync workers).
package core

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"

	"go-file-sync/pkg/config"
	"go-file-sync/pkg/history"
	"go-file-sync/pkg/logger"
	"go-file-sync/pkg/stats"
	"go-file-sync/pkg/syncer"
	"go-file-sync/pkg/trigger"
	"go-file-sync/pkg/watcher"
	"go-file-sync/pkg/web"
)

// App represents the main application instance.
type App struct {
	cfg     *config.GeneralConfig
	tasks   []*config.ResolvedConfig
	log     logger.Logger
	ctx     context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup
	pool    *syncer.WorkerPool
	metrics *stats.Manager
	web     *web.Server
	history *history.Writer

	// Pause/resume control
	pauseMu  sync.RWMutex
	paused   bool
	pauseCh  chan struct{}
	resumeCh chan struct{}

	// Config hot-reload
	cfgPath string

	triggerExecutor *trigger.Executor
}

const version = "v0.3.0"

// New creates a new application instance.
func New(cfgPath string) (*App, error) {
	// Load general config
	cfg, err := config.LoadYAML(cfgPath)
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}

	// Initialize logger
	log, err := logger.New(
		cfg.Log.Level,
		cfg.Log.Output,
		cfg.Log.File.Path,
		cfg.Log.File.MaxSize,
		cfg.Log.File.MaxBackups,
		true,
	)
	if err != nil {
		return nil, fmt.Errorf("init logger: %w", err)
	}

	// Load task configurations (supports multiple entries)
	var tasks []*config.ResolvedConfig
	for _, entry := range cfg.Conf.Entry {
		entryTasks, err := config.LoadAllTasks(cfg.Conf.Dir, entry, cfg)
		if err != nil {
			log.Close()
			return nil, fmt.Errorf("load task from entry %s: %w", entry, err)
		}
		tasks = append(tasks, entryTasks...)
	}

	ctx, cancel := context.WithCancel(context.Background())

	app := &App{
		cfg:             cfg,
		tasks:           tasks,
		log:             log,
		ctx:             ctx,
		cancel:          cancel,
		cfgPath:         cfgPath,
		pauseCh:         make(chan struct{}),
		resumeCh:        make(chan struct{}),
		triggerExecutor: trigger.New(),
	}

	// Initialize metrics
	app.metrics = stats.NewManager()

	// Initialize history writer if enabled
	if cfg.History.Enabled && cfg.History.Path != "" {
		h, err := history.New(cfg.History.Path, cfg.History.MaxEntries)
		if err != nil {
			log.Close()
			return nil, fmt.Errorf("init history: %w", err)
		}
		app.history = h
	}

	// Initialize web server if enabled
	if cfg.Web.Enabled {
		app.web = web.New(web.Config{
			Enabled:   cfg.Web.Enabled,
			Listen:    cfg.Web.Listen,
			Dashboard: cfg.Web.Dashboard,
		}, app.metrics)
		app.web.SetPauseFunc(app.Pause)
		app.web.SetResumeFunc(app.Resume)
		app.web.SetReloadFunc(app.Reload)
	}

	return app, nil
}

// Pause pauses all sync operations.
func (a *App) Pause() error {
	a.pauseMu.Lock()
	defer a.pauseMu.Unlock()
	if a.paused {
		return nil
	}
	a.paused = true
	a.pauseCh <- struct{}{}
	a.log.Info("core", "sync paused")
	return nil
}

// Resume resumes all sync operations.
func (a *App) Resume() error {
	a.pauseMu.Lock()
	defer a.pauseMu.Unlock()
	if !a.paused {
		return nil
	}
	a.paused = false
	a.resumeCh <- struct{}{}
	a.log.Info("core", "sync resumed")
	return nil
}

// Reload hot-reloads the configuration.
func (a *App) Reload() error {
	a.log.Info("core", "reloading configuration...")

	// Cancel current context to stop watchers
	a.cancel()

	// Load new config
	cfg, err := config.LoadYAML(a.cfgPath)
	if err != nil {
		return fmt.Errorf("reload config: %w", err)
	}

	// Load tasks
	var tasks []*config.ResolvedConfig
	for _, entry := range cfg.Conf.Entry {
		entryTasks, err := config.LoadAllTasks(cfg.Conf.Dir, entry, cfg)
		if err != nil {
			return fmt.Errorf("reload tasks from %s: %w", entry, err)
		}
		tasks = append(tasks, entryTasks...)
	}

	// Re-create context
	ctx, cancel := context.WithCancel(context.Background())
	a.ctx = ctx
	a.cancel = cancel
	a.cfg = cfg
	a.tasks = tasks

	// Restart watchers
	if a.pool != nil {
		for _, task := range tasks {
			if err := a.startTaskWatcher(task); err != nil {
				a.log.Error("core", "failed to restart watcher", "task", task.TaskName, "error", err)
			}
		}
	}

	a.log.Info("core", "configuration reloaded", "tasks", len(tasks))
	return nil
}

// Run starts the sync engine and blocks until shutdown.
func (a *App) Run() error {
	a.log.Info("core", "starting go-file-sync", "version", version)
	a.log.Info("core", "loaded tasks", "count", len(a.tasks))

	// Create the sync worker pool
	workerCount := a.cfg.Sync.Workers
	if workerCount <= 0 {
		workerCount = 4 // Default
	}
	a.pool = syncer.NewPool(workerCount, a.cfg.Sync.Backup, a.cfg.Sync.Verify)
	a.pool.Syncer().SetDryRun(false)
	a.pool.Syncer().SetPreservePermissions(a.cfg.Sync.PreservePermissions)
	a.pool.Syncer().SetPreserveOwner(a.cfg.Sync.PreserveOwner)
	a.pool.Syncer().SetPreserveTimestamps(a.cfg.Sync.PreserveTimestamps)
	a.pool.Syncer().SetSymlinks(a.cfg.Sync.Symlinks)
	a.pool.Syncer().SetBandwidthLimit(a.cfg.Sync.BandwidthLimit)
	a.pool.Syncer().SetConflictDetection(a.cfg.Sync.ConflictDetection)
	a.pool.Syncer().SetConflictResolution(a.cfg.Sync.ConflictResolution)

	a.pool.Start()
	a.log.Info("core", "started sync worker pool", "workers", workerCount)

	// Start watchers for each task
	for _, task := range a.tasks {
		if err := a.startTaskWatcher(task); err != nil {
			a.log.Error("core", "failed to start watcher", "task", task.TaskName, "error", err)
			continue
		}
	}

	// Perform initial full sync if configured
	if a.cfg.Sync.Mode == "full" || a.shouldInitialSync() {
		a.performInitialSync()
	}

	// Start meta-watcher for hot-reload if enabled
	if a.cfg.Watch.HotReload {
		go a.watchConfigFiles()
	}

	// Start web server if enabled
	if a.web != nil {
		go func() {
			a.log.Info("web", "starting HTTP API", "listen", a.cfg.Web.Listen)
			if err := a.web.Start(a.cfg.Web.Listen); err != nil {
				a.log.Error("web", "HTTP server error", "error", err)
			}
		}()
	}

	a.log.Info("core", "all watchers started. waiting for file changes...")
	a.log.Info("core", "press Ctrl+C to stop")

	// Wait for shutdown signal
	a.waitForShutdown()

	// Graceful shutdown
	a.log.Info("core", "shutting down...")
	a.cancel()
	if a.pool != nil {
		a.pool.Stop()
	}
	if a.history != nil {
		a.history.Close()
	}
	if a.web != nil {
		a.web.Stop()
	}
	a.log.Info("core", "shutdown complete")

	return a.log.Close()
}

// SetDryRun enables or disables dry-run mode.
func (a *App) SetDryRun(v bool) {
	if a.pool != nil {
		a.pool.Syncer().SetDryRun(v)
	}
}

// MetricsReport returns a formatted metrics report.
func (a *App) MetricsReport() string {
	if a.metrics == nil {
		return "No metrics available (not running)\n"
	}
	return a.metrics.ReportAll()
}

// Close releases resources (for use with status command).
func (a *App) Close() {
	if a.log != nil {
		a.log.Close()
	}
}

// RunOnce performs a one-shot full sync and exits (no continuous watching).
func (a *App) RunOnce() error {
	a.log.Info("core", "one-shot sync", "version", version)

	workerCount := a.cfg.Sync.Workers
	if workerCount <= 0 {
		workerCount = 4
	}
	a.pool = syncer.NewPool(workerCount, a.cfg.Sync.Backup, a.cfg.Sync.Verify)
	a.pool.Start()

	a.performInitialSync()

	a.pool.Stop()

	if a.log != nil {
		a.log.Close()
	}
	return nil
}

// startTaskWatcher sets up a watcher, debouncer, and filter pipeline for one task.
func (a *App) startTaskWatcher(task *config.ResolvedConfig) error {
	// Determine source and target (support multi-source/target)
	sources := task.Sources
	targets := task.Targets
	if len(sources) == 0 {
		return fmt.Errorf("task %s has no sources configured", task.TaskName)
	}
	if len(targets) == 0 {
		return fmt.Errorf("task %s has no targets configured", task.TaskName)
	}

	// If single source/target (legacy), wrap into slices
	if len(sources) == 1 && len(targets) == 1 {
		// Set the Source/Target on the task for backward compat
		taskWrapper := *task
		taskWrapper.Sources = sources
		taskWrapper.Targets = targets

		return a.startSingleWatcher(&taskWrapper)
	}

	// Multiple source-target pairs
	if len(sources) == len(targets) || len(targets) == 1 {
		for i, src := range sources {
			tgt := targets[0]
			if len(targets) > 1 {
				tgt = targets[i]
			}

			subTask := *task
			subTask.Sources = []string{src}
			subTask.Targets = []string{tgt}

			if err := a.startSingleWatcher(&subTask); err != nil {
				a.log.Error("watcher", "failed for source-target pair",
					"source", src, "target", tgt, "error", err)
				continue
			}
		}
		return nil
	}

	return fmt.Errorf("source/target count mismatch for task %s: %d sources, %d targets",
		task.TaskName, len(sources), len(targets))
}

func (a *App) startSingleWatcher(task *config.ResolvedConfig) error {
	if len(task.Sources) == 0 || len(task.Targets) == 0 {
		return fmt.Errorf("empty source or target for task %s", task.TaskName)
	}

	source := task.Sources[0]
	target := task.Targets[0]

	// Create a copy with the resolved single source/target
	resolvedTask := *task
	resolvedTask.Source = source
	resolvedTask.Target = target

	// Start the file watcher
	rawEvents, err := watcher.Start(a.ctx, &resolvedTask)
	if err != nil {
		return fmt.Errorf("start watcher for %s: %w", task.TaskName, err)
	}

	// Create debouncer
	debouncer := watcher.NewDebouncer(task.TaskName, task.Debounce, rawEvents)
	debouncedEvents := debouncer.Start(a.ctx)

	// Create event filter
	filter := watcher.NewFilter(&resolvedTask)

	// Start the event processing goroutine
	a.wg.Add(1)
	go func() {
		defer a.wg.Done()
		for {
			select {
			case <-a.ctx.Done():
				return
			case event, ok := <-debouncedEvents:
				if !ok {
					return
				}

				// Check pause state
				a.pauseMu.RLock()
				paused := a.paused
				a.pauseMu.RUnlock()
				if paused {
					// Wait for resume
					select {
					case <-a.ctx.Done():
						return
					case <-a.resumeCh:
					}
				}

				if !filter.ShouldProcess(event) {
					continue
				}

				// Convert watcher event to sync event
				syncEvent := config.SyncEvent{
					Type:     event.Type,
					TaskName: event.TaskName,
					Source:   event.Source,
					Target:   event.Target,
					RelPath:  event.RelPath,
					IsDir:    event.IsDir,
				}

				// Submit to worker pool
				syncTask := syncer.BuildTask(syncEvent, task.Mode)
				a.pool.SubmitForPath(syncTask, event.RelPath)

				// Execute trigger if configured
				if task.TriggerOnSync != "" {
					vars := trigger.Vars{
						RelPath: event.RelPath,
						Src:     event.Source,
						Dst:     event.Target,
						Task:    event.TaskName,
						Event:   string(event.Type),
					}
					timeout := task.TriggerTimeout
					if timeout <= 0 {
						timeout = 30
					}
					if output, err := a.triggerExecutor.Run(task.TriggerOnSync, vars, timeout); err != nil {
						a.log.Error("trigger", "on_sync failed", "error", err, "output", output)
					}
				}

				// Record history if enabled
				if a.history != nil {
					a.history.Write(event.TaskName, string(event.Type), event.RelPath, 0, "ok", "")
				}
			}
		}
	}()

	a.log.Info("watcher", "started", "task", task.TaskName,
		"source", source, "target", target)

	return nil
}

// performInitialSync does a one-time full sync of all sources to targets.
func (a *App) performInitialSync() {
	a.log.Info("core", "performing initial full sync...")
	var wg sync.WaitGroup

	for _, task := range a.tasks {
		sources := task.Sources
		targets := task.Targets

		if len(targets) == 1 {
			tgt := targets[0]
			for _, src := range sources {
				wg.Add(1)
				go func(source, target, taskName string) {
					defer wg.Done()
					a.syncDirectory(source, target, task, taskName)
				}(src, tgt, task.TaskName)
			}
		} else {
			for i, src := range sources {
				tgt := targets[i]
				wg.Add(1)
				go func(source, target, taskName string) {
					defer wg.Done()
					a.syncDirectory(source, target, task, taskName)
				}(src, tgt, task.TaskName)
			}
		}
	}

	wg.Wait()
	a.log.Info("core", "initial full sync complete")
}

// syncDirectory walks a source directory and syncs all files to target.
func (a *App) syncDirectory(source, target string, task *config.ResolvedConfig, taskName string) {
	// Use a semaphore to limit concurrency
	maxConcurrent := a.cfg.Sync.Workers
	if maxConcurrent <= 0 {
		maxConcurrent = 4
	}
	sem := make(chan struct{}, maxConcurrent)
	var innerWg sync.WaitGroup

	filepath.Walk(source, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			a.log.Warn("sync", "walk error", "path", path, "error", err)
			return nil
		}

		if info.IsDir() {
			return nil
		}

		// Check symlink mode
		if info.Mode()&os.ModeSymlink != 0 && a.cfg.Sync.Symlinks == "skip" {
			return nil
		}

		relPath, _ := filepath.Rel(source, path)
		dstPath := filepath.Join(target, relPath)

		syncEvent := config.SyncEvent{
			Type:     config.EventCreate,
			Source:   path,
			Target:   dstPath,
			RelPath:  relPath,
			TaskName: taskName,
			IsDir:    false,
		}

		syncTask := syncer.BuildTask(syncEvent, task.Mode)

		innerWg.Add(1)
		sem <- struct{}{}
		go func() {
			defer func() {
				<-sem
				innerWg.Done()
			}()
			// Submit to pool directly (blocking to ensure completion)
			a.pool.SubmitForPath(syncTask, relPath)
		}()
		return nil
	})

	innerWg.Wait()
}

// shouldInitialSync checks if initial sync is needed (target dirs are empty or missing).
func (a *App) shouldInitialSync() bool {
	for _, task := range a.tasks {
		for _, tgt := range task.Targets {
			if _, err := os.Stat(tgt); os.IsNotExist(err) {
				return true
			}
			entries, _ := os.ReadDir(tgt)
			if len(entries) == 0 {
				return true
			}
		}
	}
	return false
}

// watchConfigFiles monitors config files for changes and triggers hot-reload.
func (a *App) watchConfigFiles() {
	// Create a separate watcher for config files
	cfgWatcher, err := watcher.NewConfigWatcher(a.cfgPath, a.cfg.Conf.Dir)
	if err != nil {
		a.log.Error("core", "failed to start config watcher", "error", err)
		return
	}
	defer cfgWatcher.Close()

	a.log.Info("core", "config file watcher started for hot-reload")

	for {
		select {
		case <-a.ctx.Done():
			return
		case <-cfgWatcher.Changes():
			a.log.Info("core", "config file change detected, reloading...")
			if err := a.Reload(); err != nil {
				a.log.Error("core", "hot-reload failed", "error", err)
			}
		}
	}
}

// waitForShutdown blocks until an OS signal is received.
func (a *App) waitForShutdown() {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	sig := <-sigCh
	a.log.Info("core", "received signal", "signal", sig.String())
}
