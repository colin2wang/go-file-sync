package logger

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"
)

// MultiWriter writes log entries to multiple writers simultaneously.
type MultiWriter struct {
	writers []io.Closer
	logger  *multiLogger
}

type multiLogger struct {
	entries  chan Entry
	writers  []io.Closer
	minLevel Level
	done     chan struct{}
}

// New creates a logger that writes to the specified outputs.
// Supported output modes: "console", "file", "both".
func New(levelStr, outputMode, filePath string, maxSizeMB, maxBackups int, useColor bool) (Logger, error) {
	minLevel, err := ParseLevel(levelStr)
	if err != nil {
		minLevel = LevelInfo
	}

	var closers []io.Closer
	var entryWriters []entryWriter

	switch outputMode {
	case "console":
		w := NewConsoleWriter(minLevel, useColor)
		entryWriters = append(entryWriters, w)
		closers = append(closers, w)
	case "file":
		w, err := NewFileWriter(filePath, maxSizeMB, maxBackups, minLevel)
		if err != nil {
			return nil, err
		}
		entryWriters = append(entryWriters, w)
		closers = append(closers, w)
	case "both":
		cw := NewConsoleWriter(minLevel, useColor)
		entryWriters = append(entryWriters, cw)
		closers = append(closers, cw)

		fw, err := NewFileWriter(filePath, maxSizeMB, maxBackups, minLevel)
		if err != nil {
			return nil, err
		}
		entryWriters = append(entryWriters, fw)
		closers = append(closers, fw)
	default:
		// Default to console
		w := NewConsoleWriter(minLevel, useColor)
		entryWriters = append(entryWriters, w)
		closers = append(closers, w)
	}

	return newAsyncLogger(entryWriters, closers, minLevel), nil
}

// entryWriter is the interface for writing log entries.
type entryWriter interface {
	Write(Entry)
	Close() error
}

// asyncLogger is an asynchronous logger that processes entries on a background goroutine.
type asyncLogger struct {
	entries  chan Entry
	writers  []entryWriter
	minLevel Level
	done     chan struct{}
}

func newAsyncLogger(writers []entryWriter, closers []io.Closer, minLevel Level) *asyncLogger {
	l := &asyncLogger{
		entries:  make(chan Entry, 1000),
		writers:  writers,
		minLevel: minLevel,
		done:     make(chan struct{}),
	}

	go l.process()

	return l
}

func (l *asyncLogger) process() {
	for entry := range l.entries {
		for _, w := range l.writers {
			w.Write(entry)
		}
	}
	close(l.done)
}

func (l *asyncLogger) Debug(module, msg string, args ...any) {
	l.Log(LevelDebug, module, msg, args...)
}

func (l *asyncLogger) Info(module, msg string, args ...any) {
	l.Log(LevelInfo, module, msg, args...)
}

func (l *asyncLogger) Warn(module, msg string, args ...any) {
	l.Log(LevelWarn, module, msg, args...)
}

func (l *asyncLogger) Error(module, msg string, args ...any) {
	l.Log(LevelError, module, msg, args...)
}

func (l *asyncLogger) Log(level Level, module, msg string, args ...any) {
	if level < l.minLevel {
		return
	}

	entry := Entry{
		Time:    time.Now(),
		Level:   level,
		Module:  module,
		Message: fmtMessage(msg, args...),
	}

	select {
	case l.entries <- entry:
	default:
		// Channel full: drop the entry to avoid blocking
		fmt.Fprintf(os.Stderr, "[LOGGER] dropped log entry: %s [%s] %s\n", entry.Level, entry.Module, entry.Message)
	}
}

func (l *asyncLogger) Close() error {
	close(l.entries)
	<-l.done
	for _, w := range l.writers {
		w.Close()
	}
	return nil
}

// fmtMessage formats a message with optional key-value args.
func fmtMessage(msg string, args ...any) string {
	if len(args) == 0 {
		return msg
	}

	// Format as key=value pairs
	var sb strings.Builder
	sb.WriteString(msg)
	for i := 0; i < len(args); i += 2 {
		sb.WriteString(" ")
		sb.WriteString(fmt.Sprintf("%v", args[i]))
		sb.WriteString("=")
		if i+1 < len(args) {
			sb.WriteString(fmt.Sprintf("%v", args[i+1]))
		} else {
			sb.WriteString("?")
		}
	}
	return sb.String()
}

// compile-time interface check
var _ Logger = (*asyncLogger)(nil)
