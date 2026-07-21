package syncer

import (
	"context"
	"fmt"
	"hash/fnv"
	"sync"

	"go-file-sync/pkg/stats"
)

// Job is a unit of work submitted to the WorkerPool. It carries the sync task
// together with the FileSyncer that should execute it, so that different tasks
// can use different sync options while sharing the same pool of workers.
type Job struct {
	Task   SyncTask
	Syncer *FileSyncer
	// Done, if non-nil, is called once the job has been processed (success or error).
	Done func()
}

// WorkerPool manages M goroutines that process sync events concurrently.
// Each worker owns its own job queue; SubmitForPath routes a job to a worker
// based on the file path (consistent hashing) so that operations on the same
// file are processed in order.
type WorkerPool struct {
	workers  int
	queues   []chan Job
	wg       sync.WaitGroup
	ctx      context.Context
	cancel   context.CancelFunc
	metrics  *stats.Manager
	onResult func(task SyncTask, size int64, err error)
	mu       sync.RWMutex
}

// NewPool creates a sync worker pool with the given number of workers.
func NewPool(workerCount int) *WorkerPool {
	if workerCount <= 0 {
		workerCount = 4 // Default
	}

	ctx, cancel := context.WithCancel(context.Background())

	queues := make([]chan Job, workerCount)
	for i := range queues {
		queues[i] = make(chan Job, 256)
	}

	return &WorkerPool{
		workers: workerCount,
		queues:  queues,
		ctx:     ctx,
		cancel:  cancel,
		metrics: stats.NewManager(),
	}
}

// SetResultHandler registers a callback invoked for every processed job.
// It is safe to call before Start.
func (p *WorkerPool) SetResultHandler(fn func(task SyncTask, size int64, err error)) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.onResult = fn
}

// Metrics returns the metrics manager.
func (p *WorkerPool) Metrics() *stats.Manager {
	return p.metrics
}

// Start launches the worker goroutines.
func (p *WorkerPool) Start() {
	for i := 0; i < p.workers; i++ {
		p.wg.Add(1)
		go p.worker(i)
	}
}

// Submit adds a sync job to the pool, routed by the task's source path.
// The call blocks if the destination worker's queue is full (backpressure).
func (p *WorkerPool) Submit(job Job) {
	p.queues[RouteToWorker(job.Task.SrcPath, p.workers)] <- job
}

// SubmitForPath submits a job routed to the correct worker based on file path,
// preserving operation ordering for the same file. Blocks on a full queue.
func (p *WorkerPool) SubmitForPath(job Job, relPath string) {
	p.queues[RouteToWorker(relPath, p.workers)] <- job
}

// Stop gracefully shuts down the pool, waiting for all workers to finish.
func (p *WorkerPool) Stop() {
	p.cancel()
	for _, q := range p.queues {
		close(q)
	}
	p.wg.Wait()
}

// worker is a single goroutine that processes sync tasks from its own queue.
func (p *WorkerPool) worker(id int) {
	defer p.wg.Done()

	for {
		select {
		case <-p.ctx.Done():
			return
		case job, ok := <-p.queues[id]:
			if !ok {
				return
			}
			size, err := job.Syncer.Execute(job.Task)
			taskName := job.Task.TaskName
			m := p.metrics.ForTask(taskName)

			if err != nil {
				m.FilesFailed.Add(1)
				m.SyncErrors.Add(1)
				// Track error in global too
				p.metrics.Global().FilesFailed.Add(1)
				p.metrics.Global().SyncErrors.Add(1)
				fmt.Printf("[worker %d] sync failed: %v\n", id, err)
			} else {
				m.BytesTransferred.Add(size)
				p.metrics.Global().BytesTransferred.Add(size)

				switch job.Task.Type {
				case "copy":
					m.FilesSynced.Add(1)
					p.metrics.Global().FilesSynced.Add(1)
				case "delete":
					m.FilesDeleted.Add(1)
					p.metrics.Global().FilesDeleted.Add(1)
				}
			}

			p.mu.RLock()
			handler := p.onResult
			p.mu.RUnlock()
			if handler != nil {
				handler(job.Task, size, err)
			}
			if job.Done != nil {
				job.Done()
			}
		}
	}
}

// RouteToWorker returns a worker index for a given file path using consistent hashing.
// This ensures the same file always goes to the same worker for ordered processing.
func RouteToWorker(relPath string, workerCount int) int {
	h := fnv.New32a()
	h.Write([]byte(relPath))
	return int(h.Sum32()) % workerCount
}
