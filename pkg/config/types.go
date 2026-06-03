// Package config provides configuration parsing and management for go-file-sync.
// It supports two configuration layers:
//   - sync.yaml: general/global configuration (log, watch, sync defaults)
//   - *.conf: per-task configuration with inheritance support
package config

// GeneralConfig represents the top-level sync.yaml configuration.
type GeneralConfig struct {
	Log      LogConfig      `yaml:"log"`
	Watch    WatchConfig    `yaml:"watch"`
	Sync     SyncConfig     `yaml:"sync"`
	Shutdown ShutdownConfig `yaml:"shutdown"`
	Conf     ConfDirConfig  `yaml:"conf"`
}

// LogConfig holds log-related settings.
type LogConfig struct {
	Level  string        `yaml:"level"`
	Output string        `yaml:"output"`
	File   LogFileConfig `yaml:"file"`
}

// LogFileConfig holds file output settings for the logger.
type LogFileConfig struct {
	Path       string `yaml:"path"`
	MaxSize    int    `yaml:"max_size"`
	MaxBackups int    `yaml:"max_backups"`
	Compress   bool   `yaml:"compress"`
}

// WatchConfig holds file watching settings.
type WatchConfig struct {
	Debounce int `yaml:"debounce"`
	Interval int `yaml:"interval"`
	Workers  int `yaml:"workers"`
}

// SyncConfig holds sync engine settings.
type SyncConfig struct {
	Mode    string   `yaml:"mode"`
	Workers int      `yaml:"workers"`
	Backup  bool     `yaml:"backup"`
	Verify  bool     `yaml:"verify"`
	Exclude []string `yaml:"exclude"`
	Include []string `yaml:"include"`
}

// ShutdownConfig holds graceful shutdown settings.
type ShutdownConfig struct {
	Timeout int `yaml:"timeout"`
}

// ConfDirConfig holds the conf file directory settings.
type ConfDirConfig struct {
	Dir   string `yaml:"dir"`
	Entry string `yaml:"entry"`
}

// TaskConfig represents a single sync task parsed from a .conf file.
type TaskConfig struct {
	Task    TaskSection    `ini:"task"`
	Watch   WatchSection   `ini:"watch"`
	Inherit InheritSection `ini:"inherit"`
}

// TaskSection holds the core task configuration.
type TaskSection struct {
	Name          string   `ini:"name"`
	Source        string   `ini:"source"`
	Target        string   `ini:"target"`
	Mode          string   `ini:"mode"`
	Exclude       []string `ini:"exclude"`
	Include       []string `ini:"include"`
	DeleteOrphans bool     `ini:"delete_orphans"`
}

// WatchSection holds per-task watch settings.
type WatchSection struct {
	Recursive bool     `ini:"recursive"`
	Events    []string `ini:"events"`
	Debounce  int      `ini:"debounce"`
}

// InheritSection holds inheritance reference for parent conf files.
type InheritSection struct {
	File string `ini:"file"`
}

// DefaultConfig returns a GeneralConfig with sensible defaults.
func DefaultConfig() *GeneralConfig {
	return &GeneralConfig{
		Log: LogConfig{
			Level:  "info",
			Output: "console",
			File: LogFileConfig{
				Path:       "./logs/sync.log",
				MaxSize:    100,
				MaxBackups: 7,
				Compress:   true,
			},
		},
		Watch: WatchConfig{
			Debounce: 500,
			Interval: 100,
			Workers:  0, // 0 = one watcher per task
		},
		Sync: SyncConfig{
			Mode:    "incremental",
			Workers: 0, // 0 = auto (CPU count)
			Backup:  true,
			Verify:  true,
			Exclude: []string{"*.tmp", "*.log", ".git/", "node_modules/", ".DS_Store", "*.swp"},
			Include: []string{},
		},
		Shutdown: ShutdownConfig{
			Timeout: 30,
		},
		Conf: ConfDirConfig{
			Dir:   "./conf",
			Entry: "sync.conf",
		},
	}
}

// EventType represents a file system event type.
type EventType string

const (
	EventCreate EventType = "create"
	EventWrite  EventType = "write"
	EventRemove EventType = "remove"
	EventRename EventType = "rename"
	EventChmod  EventType = "chmod"
)

// SyncEvent represents a file system change event to be processed by the syncer.
type SyncEvent struct {
	Type     EventType
	TaskName string
	Source   string // Full source path
	Target   string // Full target path
	RelPath  string // Relative path within the watched directory
	IsDir    bool
}

// ResolvedConfig represents the final merged configuration for a single task.
type ResolvedConfig struct {
	TaskName      string
	Source        string
	Target        string
	Mode          string
	Exclude       []string
	Include       []string
	DeleteOrphans bool
	Recursive     bool
	Events        []string
	Debounce      int
}
