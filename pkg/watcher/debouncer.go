package watcher

import (
	"context"
	"sync"
	"time"
)

// Debouncer coalesces rapid events for the same file into a single event.
// Each task gets its own debouncer goroutine.
type Debouncer struct {
	taskName  string
	input     chan Event
	output    chan Event
	debounce  time.Duration
	mu        sync.Mutex
	timers    map[string]*time.Timer
	lastEvent map[string]Event
}

// NewDebouncer creates a debouncer that waits for `wait` duration
// before forwarding the last event for any given file path.
func NewDebouncer(taskName string, waitMs int, input chan Event) *Debouncer {
	if waitMs <= 0 {
		waitMs = 500
	}
	return &Debouncer{
		taskName:  taskName,
		input:     input,
		output:    make(chan Event, 100),
		debounce:  time.Duration(waitMs) * time.Millisecond,
		timers:    make(map[string]*time.Timer),
		lastEvent: make(map[string]Event),
	}
}

// Start begins the debouncer goroutine. It reads events from the input channel,
// resets a per-file timer on each event, and forwards the last event when the timer fires.
func (d *Debouncer) Start(ctx context.Context) chan Event {
	go d.run(ctx)
	return d.output
}

func (d *Debouncer) run(ctx context.Context) {
	defer close(d.output)

	for {
		select {
		case <-ctx.Done():
			// Flush remaining events on shutdown
			d.flushAll()
			return
		case event, ok := <-d.input:
			if !ok {
				d.flushAll()
				return
			}
			d.deferEvent(event)
		}
	}
}

// deferEvent starts or resets the debounce timer for the given event's file.
func (d *Debouncer) deferEvent(event Event) {
	d.mu.Lock()
	defer d.mu.Unlock()

	key := event.RelPath
	d.lastEvent[key] = event

	if timer, exists := d.timers[key]; exists {
		timer.Reset(d.debounce)
		return
	}

	d.timers[key] = time.AfterFunc(d.debounce, func() {
		d.mu.Lock()
		event := d.lastEvent[key]
		delete(d.timers, key)
		delete(d.lastEvent, key)
		d.mu.Unlock()

		select {
		case d.output <- event:
		default:
		}
	})
}

// flushAll sends all pending debounced events immediately.
func (d *Debouncer) flushAll() {
	d.mu.Lock()
	defer d.mu.Unlock()

	for key, event := range d.lastEvent {
		if timer, exists := d.timers[key]; exists {
			timer.Stop()
			delete(d.timers, key)
		}
		select {
		case d.output <- event:
		default:
		}
		delete(d.lastEvent, key)
	}
}
