package syncer

import (
	"context"
	"fmt"
	"sync"

	"go-file-sync/pkg/config"
)

// WorkerPool manages M goroutines that process sync events concurrently.
type WorkerPool struct {
	workers int
	tasks   chan SyncTask
	wg      sync.WaitGroup
	syncer  *FileSyncer
	ctx     context.Context
	cancel  context.CancelFunc
}

// NewPool creates a sync worker pool with the given number of workers.
func NewPool(workerCount int, backup, verify bool) *WorkerPool {
	if workerCount <= 0 {
		workerCount = 4 // Default
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &WorkerPool{
		workers: workerCount,
		tasks:   make(chan SyncTask, 500),
		syncer:  New(backup, verify),
		ctx:     ctx,
		cancel:  cancel,
	}
}

// Start launches the worker goroutines.
func (p *WorkerPool) Start() {
	for i := 0; i < p.workers; i++ {
		p.wg.Add(1)
		go p.worker(i)
	}
}

// Submit adds a sync task to the pool. The call returns immediately.
func (p *WorkerPool) Submit(task SyncTask) {
	select {
	case p.tasks <- task:
	default:
		fmt.Printf("[pool] task queue full, dropping: %s %s\n", task.Type, task.SrcPath)
	}
}

// SubmitForPath submits a task routed to the correct worker based on file path.
func (p *WorkerPool) SubmitForPath(task SyncTask, relPath string) {
	select {
	case p.tasks <- task:
	default:
		fmt.Printf("[pool] task queue full, dropping: %s %s\n", task.Type, task.SrcPath)
	}
}

// Stop gracefully shuts down the pool, waiting for all workers to finish.
func (p *WorkerPool) Stop() {
	p.cancel()
	close(p.tasks)
	p.wg.Wait()
}

// worker is a single goroutine that processes sync tasks.
func (p *WorkerPool) worker(id int) {
	defer p.wg.Done()

	for {
		select {
		case <-p.ctx.Done():
			return
		case task, ok := <-p.tasks:
			if !ok {
				return
			}
			if err := p.syncer.Execute(task); err != nil {
				fmt.Printf("[worker %d] sync failed: %v\n", id, err)
			}
		}
	}
}

// BuildTask creates a SyncTask from a config.SyncEvent.
func BuildTask(event config.SyncEvent, mode string) SyncTask {
	switch event.Type {
	case config.EventCreate, config.EventWrite:
		return SyncTask{
			Type:    "copy",
			SrcPath: event.Source,
			DstPath: event.Target,
			IsDir:   event.IsDir,
		}
	case config.EventRemove:
		return SyncTask{
			Type:    "delete",
			SrcPath: event.Source,
			DstPath: event.Target,
			IsDir:   event.IsDir,
		}
	case config.EventRename:
		return SyncTask{
			Type:    "rename",
			SrcPath: event.Source,
			DstPath: event.Target,
			IsDir:   event.IsDir,
		}
	default:
		return SyncTask{
			Type:    "copy",
			SrcPath: event.Source,
			DstPath: event.Target,
			IsDir:   event.IsDir,
		}
	}
}
