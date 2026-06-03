// Package watcher provides concurrent file system monitoring based on fsnotify.
// Each sync task gets its own watcher goroutine that emits events to a shared
// channel for debouncing and filtering before forwarding to the syncer.
package watcher

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/fsnotify/fsnotify"

	"go-file-sync/pkg/config"
)

// Event represents a file system event after processing (debounce + filter).
type Event struct {
	Type     config.EventType
	TaskName string
	Source   string
	Target   string
	RelPath  string
	IsDir    bool
}

// Watcher manages a single fsnotify watcher per task.
type Watcher struct {
	taskName  string
	source    string
	target    string
	recursive bool
	fsWatcher *fsnotify.Watcher
	raw       chan Event
}

// Start creates and starts a file watcher for the given resolved config.
// It returns a channel of raw events (before debounce/filter).
func Start(ctx context.Context, cfg *config.ResolvedConfig) (chan Event, error) {
	fsWatcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("create fsnotify watcher: %w", err)
	}

	w := &Watcher{
		taskName:  cfg.TaskName,
		source:    cfg.Source,
		target:    cfg.Target,
		recursive: cfg.Recursive,
		fsWatcher: fsWatcher,
		raw:       make(chan Event, 100),
	}

	// Add the source directory
	if err := w.addWatchRecursive(cfg.Source); err != nil {
		fsWatcher.Close()
		return nil, fmt.Errorf("add watch for %s: %w", cfg.Source, err)
	}

	// Start the event processing goroutine
	go w.run(ctx)

	return w.raw, nil
}

// run is the main goroutine that reads events from fsnotify and forwards them.
func (w *Watcher) run(ctx context.Context) {
	defer close(w.raw)
	defer w.fsWatcher.Close()

	for {
		select {
		case <-ctx.Done():
			return
		case fsEvent, ok := <-w.fsWatcher.Events:
			if !ok {
				return
			}
			event := w.convertEvent(fsEvent)
			if event != nil {
				select {
				case w.raw <- *event:
				default:
					// Drop event if channel is full
				}
			}
		case err, ok := <-w.fsWatcher.Errors:
			if !ok {
				return
			}
			// Log error but continue watching
			fmt.Printf("[watcher] %s error: %v\n", w.taskName, err)
		}
	}
}

// convertEvent converts a fsnotify event to our internal Event format.
func (w *Watcher) convertEvent(fsEvent fsnotify.Event) *Event {
	relPath, err := filepath.Rel(w.source, fsEvent.Name)
	if err != nil {
		return nil
	}

	// Normalize path separators
	relPath = strings.ReplaceAll(relPath, "\\", "/")

	eventType := w.mapOp(fsEvent.Op)

	return &Event{
		Type:     eventType,
		TaskName: w.taskName,
		Source:   fsEvent.Name,
		Target:   filepath.Join(w.target, relPath),
		RelPath:  relPath,
		IsDir:    false, // Will be checked on sync
	}
}

// mapOp converts fsnotify operation to our EventType.
func (w *Watcher) mapOp(op fsnotify.Op) config.EventType {
	switch {
	case op&fsnotify.Create != 0:
		return config.EventCreate
	case op&fsnotify.Write != 0:
		return config.EventWrite
	case op&fsnotify.Remove != 0:
		return config.EventRemove
	case op&fsnotify.Rename != 0:
		return config.EventRename
	case op&fsnotify.Chmod != 0:
		return config.EventChmod
	default:
		return config.EventWrite
	}
}

// addWatchRecursive adds a watch on the given path, recursively if configured.
func (w *Watcher) addWatchRecursive(root string) error {
	if !w.recursive {
		return w.fsWatcher.Add(root)
	}

	return filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			// Skip paths we can't access
			return nil
		}
		// Use type assertion since we're using fsnotify.Add which only needs a path
		return w.fsWatcher.Add(path)
	})
}

// DefaultEvents returns the default set of events to watch.
func DefaultEvents() []string {
	return []string{"create", "write", "remove", "rename"}
}

// EventTypesFromStrings converts event name strings to EventTypes.
func EventTypesFromStrings(events []string) map[config.EventType]bool {
	result := make(map[config.EventType]bool)
	for _, e := range events {
		switch strings.ToLower(e) {
		case "create":
			result[config.EventCreate] = true
		case "write":
			result[config.EventWrite] = true
		case "remove":
			result[config.EventRemove] = true
		case "rename":
			result[config.EventRename] = true
		case "chmod":
			result[config.EventChmod] = true
		}
	}
	return result
}
