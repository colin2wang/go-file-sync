package watcher

import (
	"path/filepath"
	"strings"

	"go-file-sync/pkg/config"
)

// Filter filters events based on include/exclude rules.
// It is called synchronously in the event pipeline before the event channel.
type Filter struct {
	includePatterns []string
	excludePatterns []string
	allowedEvents   map[config.EventType]bool
}

// NewFilter creates a filter from the resolved task configuration.
func NewFilter(cfg *config.ResolvedConfig) *Filter {
	return &Filter{
		includePatterns: cfg.Include,
		excludePatterns: cfg.Exclude,
		allowedEvents:   EventTypesFromStrings(cfg.Events),
	}
}

// ShouldProcess checks whether an event should be processed based on filter rules.
// Returns true if the event should be forwarded to the syncer.
func (f *Filter) ShouldProcess(event Event) bool {
	// Check event type
	if len(f.allowedEvents) > 0 {
		if !f.allowedEvents[event.Type] {
			return false
		}
	}

	relPath := event.RelPath
	relPath = strings.ReplaceAll(relPath, "\\", "/")

	// Check include rules (if any, path must match at least one)
	if len(f.includePatterns) > 0 {
		matched := false
		for _, pattern := range f.includePatterns {
			pattern = strings.ReplaceAll(pattern, "\\", "/")
			if m, _ := filepath.Match(pattern, relPath); m {
				matched = true
				break
			}
			// Also check if the pattern matches a parent directory
			if strings.HasPrefix(relPath, strings.TrimSuffix(pattern, "/")+"/") {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}

	// Check exclude rules
	for _, pattern := range f.excludePatterns {
		pattern = strings.ReplaceAll(pattern, "\\", "/")
		if m, _ := filepath.Match(pattern, relPath); m {
			return false
		}
		if strings.HasPrefix(relPath, strings.TrimSuffix(pattern, "/")+"/") {
			return false
		}
		// Check filename (last component)
		if m, _ := filepath.Match(pattern, filepath.Base(relPath)); m {
			return false
		}
	}

	return true
}
