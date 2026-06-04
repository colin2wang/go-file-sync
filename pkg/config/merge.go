package config

import (
	"fmt"
	"path/filepath"
)

const maxInheritDepth = 10

// BuildTaskChain loads a conf file and recursively resolves its inheritance chain.
// Returns the ordered list from root (parent) to leaf (child).
// The first element is the top-most ancestor, the last is the requested conf file.
func BuildTaskChain(entryPath string, confDir string) ([]*TaskConfig, error) {
	chain := make([]*TaskConfig, 0, 4)

	currentPath := entryPath
	for depth := 0; depth < maxInheritDepth; depth++ {
		if !filepath.IsAbs(currentPath) {
			currentPath = filepath.Join(confDir, currentPath)
		}

		task, err := LoadConf(currentPath)
		if err != nil {
			return nil, fmt.Errorf("load conf chain (depth %d): %w", depth, err)
		}

		chain = append(chain, task)

		if task.Inherit.File == "" {
			break // No parent, we're done
		}

		// Detect circular inheritance (check all ancestors)
		for _, prev := range chain[:len(chain)-1] {
			if prev.Inherit.File == task.Inherit.File {
				return nil, fmt.Errorf("circular inheritance detected involving %s", task.Inherit.File)
			}
		}

		currentPath = task.Inherit.File
	}

	if len(chain) >= maxInheritDepth {
		return nil, fmt.Errorf("inheritance chain exceeds max depth (%d)", maxInheritDepth)
	}

	return chain, nil
}

// MergeTaskChain merges a chain of TaskConfigs into a single ResolvedConfig.
// The chain should be ordered from root (index 0) to leaf (last index).
// Merge rules:
//   - Scalar values: child overrides parent
//   - Slice values (exclude, include, sources, targets): child appends to parent's list
func MergeTaskChain(chain []*TaskConfig, global *GeneralConfig) *ResolvedConfig {
	if len(chain) == 0 {
		return nil
	}

	result := &ResolvedConfig{}

	for i, task := range chain {
		ts := task.Task
		ws := task.Watch
		tr := task.Trigger

		if i == 0 {
			// First (root): copy all values
			result.TaskName = ts.Name
			result.Sources = append([]string{}, ts.Sources...)
			result.Targets = append([]string{}, ts.Targets...)

			// Handle legacy single source/target
			if len(result.Sources) == 0 && ts.Source != "" {
				result.Sources = []string{ts.Source}
			}
			if len(result.Targets) == 0 && ts.Target != "" {
				result.Targets = []string{ts.Target}
			}

			// Set primary source/target (first element or empty)
			if len(result.Sources) > 0 {
				result.Source = result.Sources[0]
			}
			if len(result.Targets) > 0 {
				result.Target = result.Targets[0]
			}

			result.Mode = ts.Mode
			if result.Mode == "" {
				result.Mode = global.Sync.Mode
			}
			result.Exclude = append([]string{}, ts.Exclude...)
			result.Include = append([]string{}, ts.Include...)
			result.DeleteOrphans = ts.DeleteOrphans
			result.Recursive = ws.Recursive
			result.Events = append([]string{}, ws.Events...)
			result.Debounce = ws.Debounce
			if result.Debounce <= 0 {
				result.Debounce = global.Watch.Debounce
			}
			result.TriggerOnSync = tr.OnSync
			result.TriggerOnComplete = tr.OnComplete
			result.TriggerOnError = tr.OnError
			result.TriggerTimeout = tr.Timeout
			if result.TriggerTimeout <= 0 {
				result.TriggerTimeout = 30
			}
			result.Schedule = ts.Schedule
			result.Symlinks = ts.Symlinks
			if result.Symlinks == "" {
				result.Symlinks = global.Sync.Symlinks
			}
			result.PreservePermissions = global.Sync.PreservePermissions
			result.PreserveOwner = global.Sync.PreserveOwner
			result.PreserveTimestamps = global.Sync.PreserveTimestamps
			result.ConflictDetection = global.Sync.ConflictDetection
			result.ConflictResolution = global.Sync.ConflictResolution
			if result.ConflictResolution == "" {
				result.ConflictResolution = "warn"
			}
			continue
		}

		// Child: override scalars, append slices
		if ts.Name != "" {
			result.TaskName = ts.Name
		}
		if ts.Source != "" {
			result.Sources = []string{ts.Source}
		}
		if ts.Target != "" {
			result.Targets = []string{ts.Target}
		}
		if len(ts.Sources) > 0 {
			result.Sources = append(result.Sources, ts.Sources...)
		}
		if len(ts.Targets) > 0 {
			result.Targets = append(result.Targets, ts.Targets...)
		}
		if ts.Mode != "" {
			result.Mode = ts.Mode
		}
		if len(ts.Exclude) > 0 {
			result.Exclude = append(result.Exclude, ts.Exclude...)
		}
		if len(ts.Include) > 0 {
			result.Include = append(result.Include, ts.Include...)
		}
		if ts.DeleteOrphans {
			result.DeleteOrphans = ts.DeleteOrphans
		}
		if ts.Symlinks != "" {
			result.Symlinks = ts.Symlinks
		}
		if ts.Schedule != "" {
			result.Schedule = ts.Schedule
		}
		if ws.Recursive {
			result.Recursive = ws.Recursive
		}
		if len(ws.Events) > 0 {
			result.Events = append(result.Events, ws.Events...)
		}
		if ws.Debounce > 0 {
			result.Debounce = ws.Debounce
		}
		if tr.OnSync != "" {
			result.TriggerOnSync = tr.OnSync
		}
		if tr.OnComplete != "" {
			result.TriggerOnComplete = tr.OnComplete
		}
		if tr.OnError != "" {
			result.TriggerOnError = tr.OnError
		}
		if tr.Timeout > 0 {
			result.TriggerTimeout = tr.Timeout
		}
	}

	// Apply global defaults for empty fields
	if result.Events == nil || len(result.Events) == 0 {
		result.Events = []string{"create", "write", "remove", "rename"}
	}
	if result.Debounce <= 0 {
		result.Debounce = global.Watch.Debounce
	}
	if result.Mode == "" {
		result.Mode = global.Sync.Mode
	}
	if result.Symlinks == "" {
		result.Symlinks = "follow"
	}
	if result.ConflictResolution == "" {
		result.ConflictResolution = "warn"
	}
	if result.TriggerTimeout <= 0 {
		result.TriggerTimeout = 30
	}
	// Append global exclude/include
	if len(global.Sync.Exclude) > 0 {
		result.Exclude = append(result.Exclude, global.Sync.Exclude...)
	}
	if len(global.Sync.Include) > 0 {
		result.Include = append(result.Include, global.Sync.Include...)
	}

	return result
}

// LoadAllTasks loads all tasks by reading the entry conf file and resolving inheritance chains.
func LoadAllTasks(confDir, entryFile string, global *GeneralConfig) ([]*ResolvedConfig, error) {
	resolvedDir := ResolveConfDir(confDir, ".")
	entryPath := filepath.Join(resolvedDir, entryFile)

	// Make entryPath absolute to avoid double-prepending in BuildTaskChain
	absEntryPath, err := filepath.Abs(entryPath)
	if err != nil {
		return nil, fmt.Errorf("resolve entry path: %w", err)
	}

	chain, err := BuildTaskChain(absEntryPath, resolvedDir)
	if err != nil {
		return nil, fmt.Errorf("build task chain: %w", err)
	}

	resolved := MergeTaskChain(chain, global)
	if resolved == nil {
		return nil, fmt.Errorf("empty task chain from %s", entryFile)
	}

	return []*ResolvedConfig{resolved}, nil
}
