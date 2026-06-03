package logger

import (
	"fmt"
	"strings"
	"time"
)

// Level represents a log severity level.
type Level int

const (
	// LevelDebug is the lowest log level, for detailed diagnostic information.
	LevelDebug Level = -4
	// LevelInfo is the default log level, for general operational messages.
	LevelInfo Level = 0
	// LevelWarn indicates a warning that does not interrupt operation.
	LevelWarn Level = 4
	// LevelError indicates an error that does not cause a shutdown.
	LevelError Level = 8
)

// String returns the string representation of a log level.
func (l Level) String() string {
	switch l {
	case LevelDebug:
		return "DEBUG"
	case LevelInfo:
		return "INFO"
	case LevelWarn:
		return "WARN"
	case LevelError:
		return "ERROR"
	default:
		return fmt.Sprintf("LEVEL(%d)", int(l))
	}
}

// Color returns the ANSI color code for the level.
func (l Level) Color() string {
	switch l {
	case LevelDebug:
		return "\033[36m" // Cyan
	case LevelInfo:
		return "\033[32m" // Green
	case LevelWarn:
		return "\033[33m" // Yellow
	case LevelError:
		return "\033[31m" // Red
	default:
		return "\033[0m"
	}
}

// ParseLevel parses a level name string into a Level.
func ParseLevel(s string) (Level, error) {
	switch strings.ToLower(s) {
	case "debug":
		return LevelDebug, nil
	case "info":
		return LevelInfo, nil
	case "warn", "warning":
		return LevelWarn, nil
	case "error":
		return LevelError, nil
	default:
		return LevelInfo, fmt.Errorf("unknown log level: %s", s)
	}
}

// Entry represents a single log entry.
type Entry struct {
	Time    time.Time
	Level   Level
	Module  string
	Message string
}

// Logger defines the interface for logging operations.
type Logger interface {
	Debug(module, msg string, args ...any)
	Info(module, msg string, args ...any)
	Warn(module, msg string, args ...any)
	Error(module, msg string, args ...any)
	Log(level Level, module, msg string, args ...any)
	Close() error
}
