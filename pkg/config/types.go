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
	Web      WebConfig      `yaml:"web"`
	History  HistoryConfig  `yaml:"history"`
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
	Debounce  int  `yaml:"debounce"`
	Interval  int  `yaml:"interval"`
	Workers   int  `yaml:"workers"`
	HotReload bool `yaml:"hot_reload"`
}

// SyncConfig holds sync engine settings.
type SyncConfig struct {
	Mode                string           `yaml:"mode"`
	Workers             int              `yaml:"workers"`
	Backup              bool             `yaml:"backup"`
	Verify              bool             `yaml:"verify"`
	PreservePermissions bool             `yaml:"preserve_permissions"`
	PreserveOwner       bool             `yaml:"preserve_owner"`
	PreserveTimestamps  bool             `yaml:"preserve_timestamps"`
	Symlinks            string           `yaml:"symlinks"`
	BandwidthLimit      int64            `yaml:"bandwidth_limit"`
	ConflictDetection   bool             `yaml:"conflict_detection"`
	ConflictResolution  string           `yaml:"conflict_resolution"`
	Exclude             []string         `yaml:"exclude"`
	Include             []string         `yaml:"include"`
	Encryption          EncryptionConfig `yaml:"encryption"`
}

// EncryptionConfig holds optional encryption settings for sync.
type EncryptionConfig struct {
	Enabled   bool   `yaml:"enabled"`
	Algorithm string `yaml:"algorithm"`
	KeyFile   string `yaml:"key_file"`
	KeyEnv    string `yaml:"key_env"`
}

// ShutdownConfig holds graceful shutdown settings.
type ShutdownConfig struct {
	Timeout int `yaml:"timeout"`
}

// ConfDirConfig holds the conf file directory settings.
// Entry supports both a single string and a list of strings for multiple entries.
type ConfDirConfig struct {
	Dir   string   `yaml:"dir"`
	Entry []string `yaml:"entry"`
}

// WebConfig holds the optional web dashboard / HTTP API settings.
type WebConfig struct {
	Enabled   bool   `yaml:"enabled"`
	Listen    string `yaml:"listen"`
	Dashboard bool   `yaml:"dashboard"`
}

// HistoryConfig holds sync history / audit log settings.
type HistoryConfig struct {
	Enabled    bool   `yaml:"enabled"`
	Path       string `yaml:"path"`
	MaxEntries int    `yaml:"max_entries"`
}

// TaskConfig represents a single sync task parsed from a .conf file.
type TaskConfig struct {
	Task    TaskSection    `ini:"task"`
	Watch   WatchSection   `ini:"watch"`
	Trigger TriggerSection `ini:"trigger"`
	Inherit InheritSection `ini:"inherit"`
}

// TaskSection holds the core task configuration.
type TaskSection struct {
	Name          string   `ini:"name"`
	Source        string   `ini:"source"`
	Target        string   `ini:"target"`
	Sources       []string `ini:"sources"`
	Targets       []string `ini:"targets"`
	Mode          string   `ini:"mode"`
	Exclude       []string `ini:"exclude"`
	Include       []string `ini:"include"`
	DeleteOrphans bool     `ini:"delete_orphans"`
	Symlinks      string   `ini:"symlinks"`
	Schedule      string   `ini:"schedule"`
}

// WatchSection holds per-task watch settings.
type WatchSection struct {
	Recursive bool     `ini:"recursive"`
	Events    []string `ini:"events"`
	Debounce  int      `ini:"debounce"`
}

// TriggerSection holds optional external command hooks.
type TriggerSection struct {
	OnSync     string `ini:"on_sync"`
	OnComplete string `ini:"on_complete"`
	OnError    string `ini:"on_error"`
	Timeout    int    `ini:"timeout"`
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
			Debounce:  500,
			Interval:  100,
			Workers:   0, // 0 = one watcher per task
			HotReload: false,
		},
		Sync: SyncConfig{
			Mode:                "incremental",
			Workers:             0, // 0 = auto (CPU count)
			Backup:              true,
			Verify:              true,
			PreservePermissions: false,
			PreserveOwner:       false,
			PreserveTimestamps:  false,
			Symlinks:            "follow",
			BandwidthLimit:      0,
			ConflictDetection:   false,
			ConflictResolution:  "warn",
			Exclude:             []string{"*.tmp", "*.log", ".git/", "node_modules/", ".DS_Store", "*.swp"},
			Include:             []string{},
			Encryption: EncryptionConfig{
				Enabled:   false,
				Algorithm: "aes-256-gcm",
				KeyFile:   "",
				KeyEnv:    "",
			},
		},
		Shutdown: ShutdownConfig{
			Timeout: 30,
		},
		Conf: ConfDirConfig{
			Dir:   "./conf",
			Entry: []string{"sync.conf"},
		},
		Web: WebConfig{
			Enabled:   false,
			Listen:    ":8080",
			Dashboard: false,
		},
		History: HistoryConfig{
			Enabled:    false,
			Path:       "./sync-history.jsonl",
			MaxEntries: 10000,
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
	TaskName            string
	Source              string // Primary source (first in Sources, for backward compat)
	Target              string // Primary target (first in Targets, for backward compat)
	Sources             []string
	Targets             []string
	Mode                string
	Exclude             []string
	Include             []string
	DeleteOrphans       bool
	Recursive           bool
	Events              []string
	Debounce            int
	TriggerOnSync       string
	TriggerOnComplete   string
	TriggerOnError      string
	TriggerTimeout      int
	Schedule            string
	Symlinks            string
	PreservePermissions bool
	PreserveOwner       bool
	PreserveTimestamps  bool
	ConflictDetection   bool
	ConflictResolution  string
}
