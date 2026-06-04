// Package stats provides thread-safe metrics collection for the sync engine.
// All counters use atomic operations for lock-free concurrent access.
package stats

import (
	"fmt"
	"strings"
	"sync/atomic"
	"time"
)

// Metrics holds all sync counters and gauges.
type Metrics struct {
	FilesSynced      atomic.Int64
	BytesTransferred atomic.Int64
	FilesFailed      atomic.Int64
	FilesDeleted     atomic.Int64
	SyncErrors       atomic.Int64
	StartTime        time.Time
}

// New creates a new Metrics instance with StartTime set to now.
func New() *Metrics {
	return &Metrics{StartTime: time.Now()}
}

// Report returns a formatted summary string of all metrics.
func (m *Metrics) Report() string {
	var b strings.Builder
	uptime := time.Since(m.StartTime).Round(time.Second)
	b.WriteString(fmt.Sprintf("Uptime: %s\n", uptime))
	b.WriteString(fmt.Sprintf("Files synced:  %d\n", m.FilesSynced.Load()))
	b.WriteString(fmt.Sprintf("Files deleted: %d\n", m.FilesDeleted.Load()))
	b.WriteString(fmt.Sprintf("Files failed:  %d\n", m.FilesFailed.Load()))
	b.WriteString(fmt.Sprintf("Sync errors:   %d\n", m.SyncErrors.Load()))
	b.WriteString(fmt.Sprintf("Bytes xfer:    %d\n", m.BytesTransferred.Load()))
	return b.String()
}

// Manager holds per-task metrics and global metrics.
type Manager struct {
	global  *Metrics
	perTask map[string]*Metrics
}

// NewManager creates a metrics manager.
func NewManager() *Manager {
	return &Manager{
		global:  New(),
		perTask: make(map[string]*Metrics),
	}
}

// ForTask returns the metrics for a given task, creating it if needed.
func (m *Manager) ForTask(taskName string) *Metrics {
	if _, ok := m.perTask[taskName]; !ok {
		m.perTask[taskName] = New()
	}
	return m.perTask[taskName]
}

// Global returns the global metrics.
func (m *Manager) Global() *Metrics {
	return m.global
}

// ReportAll returns a formatted report of all metrics.
func (m *Manager) ReportAll() string {
	var b strings.Builder
	b.WriteString("=== Global Metrics ===\n")
	b.WriteString(m.global.Report())
	b.WriteString("\n")

	for taskName, tm := range m.perTask {
		b.WriteString(fmt.Sprintf("=== Task: %s ===\n", taskName))
		b.WriteString(tm.Report())
	}
	return b.String()
}
