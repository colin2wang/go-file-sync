package logger

// NewConsole creates a simple console-only logger.
func NewConsole(levelStr string) (Logger, error) {
	return New(levelStr, "console", "", 0, 0, true)
}

// NewFile creates a simple file-only logger.
func NewFile(levelStr, path string, maxSizeMB, maxBackups int, compress bool) (Logger, error) {
	return New(levelStr, "file", path, maxSizeMB, maxBackups, false)
}

// NewBoth creates a logger that writes to both console and file.
func NewBoth(levelStr, path string, maxSizeMB, maxBackups int, compress bool) (Logger, error) {
	return New(levelStr, "both", path, maxSizeMB, maxBackups, true)
}

// NopLogger is a logger that discards all log entries.
type NopLogger struct{}

func (n *NopLogger) Debug(module, msg string, args ...any)            {}
func (n *NopLogger) Info(module, msg string, args ...any)             {}
func (n *NopLogger) Warn(module, msg string, args ...any)             {}
func (n *NopLogger) Error(module, msg string, args ...any)            {}
func (n *NopLogger) Log(level Level, module, msg string, args ...any) {}
func (n *NopLogger) Close() error                                     { return nil }

// compile-time interface check
var _ Logger = (*NopLogger)(nil)
var _ entryWriter = (*ConsoleWriter)(nil)
var _ entryWriter = (*FileWriter)(nil)
