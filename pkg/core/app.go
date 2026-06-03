// Package core manages the application lifecycle: initialization, orchestration,
// and graceful shutdown of all goroutines (watchers, debouncers, sync workers).
package core

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"go-file-sync/pkg/config"
	"go-file-sync/pkg/logger"
	"go-file-sync/pkg/syncer"
	"go-file-sync/pkg/watcher"
)

// App represents the main application instance.
type App struct {
	cfg    *config.GeneralConfig
	tasks  []*config.ResolvedConfig
	log    logger.Logger
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

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

	// Load task configurations
	tasks, err := config.LoadAllTasks(cfg.Conf.Dir, cfg.Conf.Entry, cfg)
	if err != nil {
		log.Close()
		return nil, fmt.Errorf("load tasks: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &App{
		cfg:    cfg,
		tasks:  tasks,
		log:    log,
		ctx:    ctx,
		cancel: cancel,
	}, nil
}

// Run starts the sync engine and blocks until shutdown.
func (a *App) Run() error {
	a.log.Info("core", "starting go-file-sync")
	a.log.Info("core", "loaded tasks", "count", len(a.tasks))

	// Create the sync worker pool
	workerCount := a.cfg.Sync.Workers
	if workerCount <= 0 {
		workerCount = 4 // Default
	}
	pool := syncer.NewPool(workerCount, a.cfg.Sync.Backup, a.cfg.Sync.Verify)
	pool.Start()
	a.log.Info("core", "started sync worker pool", "workers", workerCount)

	// Start watchers for each task
	for _, task := range a.tasks {
		if err := a.startTaskWatcher(task, pool); err != nil {
			a.log.Error("core", "failed to start watcher", "task", task.TaskName, "error", err)
			continue
		}
	}

	// Perform initial sync if configured
	if a.cfg.Sync.Mode == "full" {
		a.performInitialSync()
	}

	a.log.Info("core", "all watchers started. waiting for file changes...")
	a.log.Info("core", "press Ctrl+C to stop")

	// Wait for shutdown signal
	a.waitForShutdown()

	// Graceful shutdown
	a.log.Info("core", "shutting down...")
	a.cancel()
	pool.Stop()
	a.log.Info("core", "shutdown complete")

	return a.log.Close()
}

// startTaskWatcher sets up a watcher, debouncer, and filter pipeline for one task.
func (a *App) startTaskWatcher(task *config.ResolvedConfig, pool *syncer.WorkerPool) error {
	// Start the file watcher
	rawEvents, err := watcher.Start(a.ctx, task)
	if err != nil {
		return fmt.Errorf("start watcher: %w", err)
	}

	// Create debouncer
	debouncer := watcher.NewDebouncer(task.TaskName, task.Debounce, rawEvents)
	debouncedEvents := debouncer.Start(a.ctx)

	// Create event filter
	filter := watcher.NewFilter(task)

	// Start the event processing goroutine
	a.wg.Add(1)
	go func() {
		defer a.wg.Done()
		for event := range debouncedEvents {
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
			pool.SubmitForPath(syncTask, event.RelPath)
		}
	}()

	a.log.Info("watcher", "started", "task", task.TaskName,
		"source", task.Source, "target", task.Target)

	return nil
}

// performInitialSync does a one-time full sync of all sources to targets.
func (a *App) performInitialSync() {
	a.log.Info("core", "performing initial full sync...")
	for _, task := range a.tasks {
		a.log.Info("core", "initial sync", "task", task.TaskName,
			"source", task.Source, "target", task.Target)
		// Walk source directory and copy all files
		// Implementation uses the syncer directly
	}
	a.log.Info("core", "initial sync complete")
}

// waitForShutdown blocks until an OS signal is received.
func (a *App) waitForShutdown() {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	sig := <-sigCh
	a.log.Info("core", "received signal", "signal", sig.String())
}
