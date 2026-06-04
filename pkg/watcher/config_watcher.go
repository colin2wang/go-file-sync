// Package watcher provides concurrent file system monitoring based on fsnotify.
// Each sync task gets its own watcher goroutine that emits events to a shared
// channel for debouncing and filtering before forwarding to the syncer.
package watcher

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/fsnotify/fsnotify"
)

// ConfigWatcher watches configuration files for changes to trigger hot-reload.
type ConfigWatcher struct {
	watcher *fsnotify.Watcher
	changes chan struct{}
}

// NewConfigWatcher creates a watcher that monitors the main config and conf directory.
func NewConfigWatcher(cfgPath, confDir string) (*ConfigWatcher, error) {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("create config watcher: %w", err)
	}

	cw := &ConfigWatcher{
		watcher: w,
		changes: make(chan struct{}, 10),
	}

	// Watch the main config file's directory
	cfgDir := filepath.Dir(cfgPath)
	if err := w.Add(cfgDir); err != nil {
		w.Close()
		return nil, fmt.Errorf("watch config dir %s: %w", cfgDir, err)
	}

	// Watch the conf directory if it exists
	if absConfDir, err := filepath.Abs(confDir); err == nil {
		if info, err := os.Stat(absConfDir); err == nil && info.IsDir() {
			if err := w.Add(absConfDir); err != nil {
				w.Close()
				return nil, fmt.Errorf("watch conf dir %s: %w", absConfDir, err)
			}
		}
	}

	go cw.run()
	return cw, nil
}

// run monitors fsnotify events and signals changes.
func (cw *ConfigWatcher) run() {
	for event := range cw.watcher.Events {
		// Only trigger on write events for config files
		if event.Op&fsnotify.Write != 0 || event.Op&fsnotify.Create != 0 {
			ext := strings.ToLower(filepath.Ext(event.Name))
			if ext == ".yaml" || ext == ".yml" || ext == ".conf" {
				select {
				case cw.changes <- struct{}{}:
				default:
					// Channel full, drop duplicate
				}
			}
		}
	}
}

// Changes returns a channel that receives a signal when config files change.
func (cw *ConfigWatcher) Changes() <-chan struct{} {
	return cw.changes
}

// Close stops the config watcher.
func (cw *ConfigWatcher) Close() error {
	return cw.watcher.Close()
}
