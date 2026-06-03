package logger

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// ConsoleWriter writes log entries to stdout with optional color.
type ConsoleWriter struct {
	mu       sync.Mutex
	out      *os.File
	useColor bool
	minLevel Level
}

// NewConsoleWriter creates a ConsoleWriter.
func NewConsoleWriter(minLevel Level, useColor bool) *ConsoleWriter {
	return &ConsoleWriter{
		out:      os.Stdout,
		useColor: useColor,
		minLevel: minLevel,
	}
}

// Write writes a log entry to the console.
func (w *ConsoleWriter) Write(entry Entry) {
	if entry.Level < w.minLevel {
		return
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	timestamp := entry.Time.Format("2006-01-02 15:04:05.000")

	if w.useColor {
		color := entry.Level.Color()
		reset := "\033[0m"
		fmt.Fprintf(w.out, "%s %s[%5s]%s [%s] %s\n",
			timestamp, color, entry.Level, reset, entry.Module, entry.Message)
	} else {
		fmt.Fprintf(w.out, "%s [%5s] [%s] %s\n",
			timestamp, entry.Level, entry.Module, entry.Message)
	}
}

// Close is a no-op for console writer.
func (w *ConsoleWriter) Close() error {
	return nil
}

// FileWriter writes log entries to a file with rotation support.
type FileWriter struct {
	mu          sync.Mutex
	file        *os.File
	path        string
	maxSize     int64
	maxBackups  int
	currentSize int64
	minLevel    Level
}

// NewFileWriter creates a FileWriter that writes to the specified path.
func NewFileWriter(path string, maxSizeMB, maxBackups int, minLevel Level) (*FileWriter, error) {
	dir := filepath.Dir(path)
	if dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("create log dir: %w", err)
		}
	}

	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return nil, fmt.Errorf("open log file: %w", err)
	}

	stat, _ := file.Stat()

	return &FileWriter{
		file:        file,
		path:        path,
		maxSize:     int64(maxSizeMB) * 1024 * 1024,
		maxBackups:  maxBackups,
		currentSize: stat.Size(),
		minLevel:    minLevel,
	}, nil
}

// Write writes a log entry to the file.
func (w *FileWriter) Write(entry Entry) {
	if entry.Level < w.minLevel {
		return
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	if w.maxSize > 0 && w.currentSize >= w.maxSize {
		w.rotate()
	}

	timestamp := entry.Time.Format("2006-01-02 15:04:05.000")
	line := fmt.Sprintf("%s [%5s] [%s] %s\n", timestamp, entry.Level, entry.Module, entry.Message)

	n, _ := w.file.WriteString(line)
	w.currentSize += int64(n)
}

// Close closes the underlying file.
func (w *FileWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.file != nil {
		return w.file.Close()
	}
	return nil
}

// rotate renames the current log file and creates a new one.
func (w *FileWriter) rotate() {
	w.file.Close()

	// Rotate existing backups
	for i := w.maxBackups - 1; i >= 1; i-- {
		oldPath := fmt.Sprintf("%s.%d", w.path, i)
		newPath := fmt.Sprintf("%s.%d", w.path, i+1)
		if _, err := os.Stat(oldPath); err == nil {
			os.Rename(oldPath, newPath)
		}
	}

	// Rename current log
	backupPath := fmt.Sprintf("%s.1", w.path)
	os.Rename(w.path, backupPath)

	// Open new file
	file, err := os.OpenFile(w.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to rotate log file: %v\n", err)
		return
	}
	w.file = file
	w.currentSize = 0
}
