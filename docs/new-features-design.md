# go-file-sync — New Feature Design

> Version: v0.3.0-v0.5.0 (Proposed)
> Last Updated: 2026-06-04
> Status: Draft

---

## Table of Contents

1. [Overview](#1-overview)
2. [Feature List & Roadmap](#2-feature-list--roadmap)
3. [Feature Details](#3-feature-details)
    - 3.1 Initial Full Sync
    - 3.2 Dry-Run Mode
    - 3.3 Trigger Script / External Command Hooks
    - 3.4 Hot-Reload Configuration
    - 3.5 Sync Status Report / Metrics
    - 3.6 One-Shot Mode (sync once and exit)
    - 3.7 Multiple Conf Entries
    - 3.8 Multi-Source / Multi-Target
    - 3.9 File Permission & Attribute Sync
    - 3.10 Conflict Detection
    - 3.11 Symlink Handling
    - 3.12 Network Bandwidth Limiter
    - 3.13 Web Dashboard / HTTP Status API
    - 3.14 Sync History / Audit Log
    - 3.15 Encrypted Sync (AES-GCM)
    - 3.16 Scheduled Sync (cron-style)
4. [Data Flow Changes](#4-data-flow-changes)
5. [Config Schema Changes](#5-config-schema-changes)
6. [CLI Changes](#6-cli-changes)
7. [Implementation Order](#7-implementation-order)

---

## 1. Overview

This document proposes new features for go-file-sync, organized into three release phases:

| Phase | Version | Focus | Features |
|-------|---------|-------|----------|
| **Phase A** | v0.3.0 | Core improvements | Full sync, dry-run, trigger scripts, hot-reload, status, one-shot |
| **Phase B** | v0.4.0 | Multi-config & routing | Multiple entries, multi-source/target, permissions, symlinks |
| **Phase C** | v0.5.0 | Advanced | Conflict detection, bandwidth limit, web dashboard, history, encryption |

All features below are **optional** — the default configuration produces no breaking changes.

---

## 2. Feature List & Roadmap

| # | Feature | Priority | Phase | Effort | Risk |
|---|---------|----------|-------|--------|------|
| 1 | Initial Full Sync | P0 | A | 2 days | Low |
| 2 | Dry-Run Mode (`--dry-run`) | P0 | A | 1 day | Low |
| 3 | Trigger Script / External Command Hooks | P1 | A | 2 days | Medium |
| 4 | Hot-Reload Configuration | P1 | A | 3 days | Medium |
| 5 | Sync Status Report / Metrics (stdout) | P1 | A | 2 days | Low |
| 6 | One-Shot Mode (`go-file-sync sync`) | P1 | A | 1 day | Low |
| 7 | Multiple Conf Entry Files | P1 | B | 1 day | Low |
| 8 | Multi-Source / Multi-Target per Task | P1 | B | 2 days | Low |
| 9 | File Permission & Attribute Sync | P2 | B | 2 days | Medium |
| 10 | Symlink Handling | P2 | B | 1 day | Low |
| 11 | Conflict Detection | P2 | C | 3 days | Medium |
| 12 | Network Bandwidth Limiter | P2 | C | 2 days | High |
| 13 | Web Dashboard / HTTP Status API | P2 | C | 5 days | Medium |
| 14 | Sync History / Audit Log | P3 | C | 2 days | Low |
| 15 | Encrypted Sync (AES-GCM) | P3 | C | 3 days | High |
| 16 | Scheduled Sync (cron-style) | P3 | C | 2 days | Low |

---

## 3. Feature Details

### 3.1 Initial Full Sync

**Goal:** When the application starts, perform a one-time full copy of all source files to target directories.

**Current state:** `performInitialSync()` in `core/app.go` is a stub with no implementation.

**Implementation:**

```go
// In core/app.go — expand performInitialSync
func (a *App) performInitialSync() {
    for _, task := range a.tasks {
        a.log.Info("core", "initial sync", "task", task.TaskName, "source", task.Source, "target", task.Target)
        a.syncTaskFull(task, pool)
    }
}

func (a *App) syncTaskFull(task *config.ResolvedConfig, pool *syncer.WorkerPool) {
    sem := make(chan struct{}, maxConcurrent)
    filepath.Walk(task.Source, func(path string, info os.FileInfo, err error) error {
        if err != nil { return nil }
        if info.IsDir() { return nil }
        // route to pool via BuildTask
        sem <- struct{}{}
        go func() {
            defer func() { <-sem }()
            relPath, _ := filepath.Rel(task.Source, path)
            syncTask := syncer.BuildTask(config.SyncEvent{
                Type: config.EventCreate, Source: path,
                Target: filepath.Join(task.Target, relPath),
                RelPath: relPath, TaskName: task.TaskName,
            }, task.Mode)
            pool.SubmitForPath(syncTask, relPath)
        }()
    })
    // drain semaphore
    for i := 0; i < cap(sem); i++ { sem <- struct{}{} }
}
```

**Config changes:** `sync.mode: full` triggers full sync on startup (already supported). New: if target directory is empty, auto-default to full sync.

**File changes:**
- `pkg/core/app.go` — implement `performInitialSync` and `syncTaskFull`
- `pkg/syncer/syncer.go` — may need `BulkCopy(src, dst []string)` for efficiency

---

### 3.2 Dry-Run Mode

**Goal:** Show what would be synced without actually performing any file operations.

**CLI:** `go-file-sync run --dry-run` or `go-file-sync check --verbose`

**Implementation:**

Add a `dryRun` bool to `FileSyncer`. When true, `Execute()` prints what it would do but skips all file I/O:

```go
type FileSyncer struct {
    backupEnabled bool
    verifyEnabled bool
    dryRun        bool
}

func (s *FileSyncer) Execute(task SyncTask) error {
    if s.dryRun {
        fmt.Printf("[DRY-RUN] would %s %s -> %s\n", task.Type, task.SrcPath, task.DstPath)
        return nil
    }
    // existing logic...
}
```

**Config changes:** `--dry-run` CLI flag only (no YAML config needed).

**File changes:**
- `pkg/syncer/syncer.go` — add `dryRun` field
- `pkg/syncer/pool.go` — pass dry-run flag through
- `cmd/root.go` — add `--dry-run` flag
- `pkg/core/app.go` — propagate to pool

---

### 3.3 Trigger Script / External Command Hooks

**Goal:** Run an external command or script when a file change event is triggered.

**Config (.conf file):**

```conf
[task]
name = "web-assets"
source = "./src"
target = "./dist"

[trigger]
on_sync = "echo 'File synced: {{relpath}}'"         # after each file sync
on_complete = "deploy.bat"                           # after all files in this batch
on_error = "notify-error.bat {{error}}"              # on sync failure
```

**Trigger section in types.go:**

```go
type TriggerSection struct {
    OnSync     string `ini:"on_sync"`     // Command run after each file sync (optional)
    OnComplete string `ini:"on_complete"` // Command run after batch completes (optional)
    OnError    string `ini:"on_error"`    // Command run on sync failure (optional)
    Timeout    int    `ini:"timeout"`     // Command timeout in seconds (default 30)
}
```

**Implementation:**

Create `pkg/trigger/trigger.go`:

```go
package trigger

type Executor struct {
    config TriggerConfig
}

func (e *Executor) Exec(template string, vars map[string]string) error {
    cmdStr := os.Expand(template, func(key string) string {
        return vars[key]
    })
    ctx, cancel := context.WithTimeout(context.Background(), timeout)
    defer cancel()
    cmd := exec.CommandContext(ctx, "cmd", "/c", cmdStr) // or "sh", "-c" on unix
    return cmd.Run()
}
```

**Template variables available:**
- `{{relpath}}` — relative file path
- `{{src}}` — full source path
- `{{dst}}` — full target path
- `{{task}}` — task name
- `{{event}}` — event type (create/write/remove/rename)
- `{{error}}` — error message (on_error only)

**File changes:**
- `pkg/config/types.go` — add `TriggerSection`
- `pkg/trigger/` — new package
- `pkg/core/app.go` — wire trigger execution after sync

---

### 3.4 Hot-Reload Configuration

**Goal:** When `sync.yaml` or any `.conf` file changes, reload the entire configuration without restarting the process.

**Implementation:**

Add a "meta-watcher" that watches the config files themselves:

```
meta-watcher (goroutine)
    ↓
detects change in sync.yaml or conf/*.conf
    ↓
re-read all configs
    ↓
compare new tasks vs current tasks
    ↓
stop watchers for removed/changed tasks
    ↓
start watchers for new/changed tasks
```

**Key design decisions:**
- Recreate entire app state (watchers, debouncers, event pipeline)
- Drain existing event channels before reload
- Logger and worker pool persist across reload (no restart needed)
- If new config is invalid, log error and keep running with old config

**Config changes:** `watch.hot_reload: true` (default: false)

**File changes:**
- `pkg/config/types.go` — add `HotReload bool` to `WatchConfig`
- `pkg/core/app.go` — add `StartMetaWatcher()`, `Reload()`, `WatchConfigFiles()`

---

### 3.5 Sync Status Report / Metrics

**Goal:** Print a summary report to the console, accessible via `go-file-sync status` or via signal (SIGUSR1 on Unix).

**Metrics tracked:**

| Metric | Type | Description |
|--------|------|-------------|
| `files_synced` | Counter | Total files successfully synced |
| `bytes_transferred` | Counter | Total bytes copied |
| `files_failed` | Counter | Total files that failed sync |
| `files_deleted` | Counter | Total files deleted on target |
| `sync_errors` | Counter | Total sync errors |
| `uptime_seconds` | Gauge | Application uptime |
| `tasks_running` | Gauge | Number of active tasks |
| `queue_depth` | Gauge | Current event queue depth |
| `last_event_time` | Timestamp | Time of last processed event |

**Implementation:**

Create `pkg/stats/stats.go` — thread-safe metrics store using `sync/atomic`:

```go
type Metrics struct {
    FilesSynced     atomic.Int64
    BytesTransferred atomic.Int64
    FilesFailed     atomic.Int64
    FilesDeleted    atomic.Int64
    SyncErrors      atomic.Int64
    StartTime       time.Time
}

func (m *Metrics) Report() string { /* format as table */ }
```

**Per-task metrics** via `metrics map[string]*Metrics`.

**CLI:** `go-file-sync status` reads metrics file or HTTP endpoint.

**File changes:**
- `pkg/stats/` — new package
- `pkg/syncer/syncer.go` — wire metrics updates
- `pkg/core/app.go` — wire metrics and report on signal
- `cmd/root.go` — add `status` subcommand

---

### 3.6 One-Shot Mode

**Goal:** Sync all files once and exit (no continuous watching).

**CLI:** `go-file-sync sync`

Alias for `go-file-sync run --mode=full --exit-after-sync`.

**Implementation:**

```go
// In cmd/root.go
var syncCmd = &cobra.Command{
    Use:   "sync",
    Short: "Sync all files once and exit",
    RunE: func(cmd *cobra.Command, args []string) error {
        app, err := core.New(cfgFile)
        if err != nil { return err }
        app.RunOnce() // full sync, no watchers, then exit
        return nil
    },
}
```

**File changes:**
- `pkg/core/app.go` — add `RunOnce()` method
- `cmd/root.go` — register `sync` command

---

### 3.7 Multiple Conf Entry Files

**Goal:** Load multiple `.conf` entry files instead of just one.

**Config change (sync.yaml):**

```yaml
conf:
  dir: ./conf
  entry: ["sync.conf", "extra.conf", "conf/*.conf"]  # string or list, glob supported
```

**Implementation:**

```go
func ResolveEntries(confDir string, entries interface{}) ([]string, error) {
    // if string, treat as single or glob
    // if []string, expand each with glob
    // return absolute paths to all .conf files
}
```

**File changes:**
- `pkg/config/types.go` — `ConfDirConfig.Entry` changes to `interface{}` or use a custom unmarshaler
- `pkg/config/merge.go` — `LoadAllTasks` iterates over resolved entries

---

### 3.8 Multi-Source / Multi-Target per Task

**Goal:** One task can sync multiple source directories to corresponding or shared target directories.

**Config (.conf file):**

```conf
[task]
name = "aggregate"
source = ["./src/a", "./src/b"]       # multiple sources (new)
target = ["./dst/a", "./dst/b"]       # same count as source, OR
target = "./dst"                       # single target (all sources merged)
mode = "sync"
```

**Or using legacy single values:**

```conf
source = "./src"    # string — backward compatible
target = "./dst"    # string — backward compatible
```

**Implementation:**

```
len(srcs) == len(targets) → direct mapping
len(targets) == 1       → all sources → same target
len(srcs) > 1 && len(targets) > 1 && len(srcs) != len(targets) → error
```

**File changes:**
- `pkg/config/types.go` — `Source` and `Target` become `[]string` with custom unmarshaler
- `pkg/config/merge.go` — merge logic for slices
- `pkg/core/app.go` — one watcher per source-target pair (within one task)

---

### 3.9 File Permission & Attribute Sync

**Goal:** Preserve file permissions, ownership, and timestamps when syncing.

**Config:**

```yaml
sync:
  preserve_permissions: true   # default false
  preserve_owner: false        # default false (requires root/admin)
  preserve_timestamps: true    # default false
```

**Implementation:**

In `syncer.go` `copyFile()`, after copy:

```go
func (s *FileSyncer) preserveAttrs(src, dst string) {
    if s.preservePerms {
        srcInfo, _ := os.Stat(src)
        os.Chmod(dst, srcInfo.Mode())
    }
    if s.preserveTimes {
        srcInfo, _ := os.Stat(src)
        os.Chtimes(dst, srcInfo.ModTime(), srcInfo.ModTime())
    }
    // preserveOwner uses syscall + platform-specific code
}
```

**File changes:**
- `pkg/config/types.go` — add `PreservePermissions`, `PreserveOwner`, `PreserveTimestamps`
- `pkg/syncer/syncer.go` — add fields and `preserveAttrs` method

---

### 3.10 Conflict Detection

**Goal:** Detect when target files have been modified since last sync, and warn/block accordingly.

**Mechanism:** Store a checksum manifest (`sync.manifest`) at the target, tracking `relpath → sha256_hash + timestamp` for each synced file.

**Implementation:**

```go
type Manifest map[string]ManifestEntry
type ManifestEntry struct {
    Hash      string    `json:"hash"`
    SyncedAt  time.Time `json:"synced_at"`
}
```

On each sync event:
1. Read manifest from target (`.go-file-sync-manifest.json`)
2. Check if target file hash differs from manifest → conflict!
3. Apply configured resolution:
   - `conflict_resolution: "warn"` — log warning, proceed with overwrite
   - `conflict_resolution: "skip"` — skip the sync, log warning
   - `conflict_resolution: "backup"` — backup target, then overwrite
4. Update manifest after successful sync

**Config:**

```yaml
sync:
  conflict_detection: false     # enable/disable
  conflict_resolution: "warn"  # warn | skip | backup
```

**File changes:**
- `pkg/config/types.go` — add manifest-related fields
- `pkg/config/manifest.go` — new file
- `pkg/syncer/syncer.go` — add conflict check before write

---

### 3.11 Symlink Handling

**Goal:** Control how symbolic links are treated during sync.

**Config:**

```yaml
sync:
  symlinks: "follow"    # follow | copy | skip
```

- `follow` — resolve symlink and sync the target file (default)
- `copy` — recreate the symlink at target
- `skip` — ignore symlinks entirely

**Implementation:**

In `watcher.addWatchRecursive`, use `filepath.Walk` which follows symlinks by default. In `filter.go`, check for symlinks:

```go
func (f *Filter) shouldHandleSymlink(path string) bool {
    switch f.symlinkMode {
    case "skip":
        return false
    case "copy":
        // preserve symlink
    case "follow":
        // normal file handling
    }
    return true
}
```

**File changes:**
- `pkg/config/types.go` — add `Symlinks string`
- `pkg/watcher/filter.go` — add symlink filtering
- `pkg/syncer/syncer.go` — add `copySymlink` function

---

### 3.12 Network Bandwidth Limiter

**Goal:** Limit bytes-per-second for sync operations to avoid saturating network or disk I/O.

**Implementation:**

Use a token bucket rate limiter (`golang.org/x/time/rate`):

```go
type RateLimitedReader struct {
    reader  io.Reader
    limiter *rate.Limiter
}

func (r *RateLimitedReader) Read(p []byte) (int, error) {
    n, err := r.reader.Read(p)
    if n > 0 {
        r.limiter.WaitN(context.Background(), n)
    }
    return n, err
}
```

**Config:**

```yaml
sync:
  bandwidth_limit: 0         # bytes per second (0 = unlimited)
  bandwidth_limit_per_task: false  # whether limit applies per-task or globally
```

**File changes:**
- `pkg/config/types.go` — add `BandwidthLimit` int
- `pkg/syncer/syncer.go` — wrap reader with rate limiter
- `go.mod` — add `golang.org/x/time` dependency

---

### 3.13 Web Dashboard / HTTP Status API

**Goal:** Provide a lightweight HTTP API and optional web dashboard for monitoring sync status.

**Two modes:**
1. **API only** — returns JSON for integration with external monitoring tools
2. **Dashboard** — optional embedded HTML dashboard (single-page, no extra deps)

**API endpoints:**

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/status` | Overall status: uptime, tasks, metrics |
| GET | `/api/v1/tasks` | List all tasks with their state |
| GET | `/api/v1/tasks/:name` | Single task details |
| GET | `/api/v1/metrics` | Raw metrics |
| POST | `/api/v1/pause` | Pause all tasks |
| POST | `/api/v1/resume` | Resume all tasks |
| POST | `/api/v1/reload` | Hot-reload config |

**Config:**

```yaml
web:
  enabled: false        # default disabled
  listen: ":8080"       # listen address
  dashboard: false      # serve HTML dashboard (requires embedded assets)
```

**Implementation:**

Use Go's `net/http` standard library (no external dependencies):

```go
// pkg/web/server.go
type Server struct {
    mux     *http.ServeMux
    stats   *stats.Metrics
    app     *core.App
}

func (s *Server) Start(addr string) error {
    s.mux.HandleFunc("/api/v1/status", s.handleStatus)
    s.mux.HandleFunc("/api/v1/tasks", s.handleTasks)
    // ...
    return http.ListenAndServe(addr, s.mux)
}
```

**File changes:**
- `pkg/config/types.go` — add `WebConfig`
- `pkg/web/` — new package
- `pkg/core/app.go` — start web server if enabled

---

### 3.14 Sync History / Audit Log

**Goal:** Record a persistent log of every sync operation (file, size, timestamp, status).

**Implementation:**

Write to a JSONL file (one JSON object per line):

```
{"time":"2026-06-04T10:00:00Z","task":"dev","event":"create","file":"src/main.go","size":2048,"status":"ok"}
{"time":"2026-06-04T10:00:01Z","task":"dev","event":"remove","file":"src/old.go","status":"ok"}
{"time":"2026-06-04T10:00:02Z","task":"dev","event":"error","file":"src/big.bin","error":"disk full","status":"failed"}
```

**Config:**

```yaml
history:
  enabled: false
  path: ./sync-history.jsonl
  max_entries: 10000       # auto-prune after this many entries
```

**File changes:**
- `pkg/config/types.go` — add `HistoryConfig`
- `pkg/history/` — new package
- `pkg/core/app.go` — wire history writer

---

### 3.15 Encrypted Sync (AES-GCM)

**Goal:** Encrypt files before writing to target, decrypt on read-back. Useful for syncing to untrusted locations.

**Config:**

```yaml
sync:
  encryption:
    enabled: false
    algorithm: "aes-256-gcm"
    key_file: "./key.bin"      # 32-byte binary key file
    # OR
    key_env: "SYNC_ENCRYPT_KEY" # environment variable name
```

**Implementation:**

```go
// pkg/syncer/encrypt.go
func EncryptFile(src, dst string, key []byte) error {
    plaintext, _ := os.ReadFile(src)
    ciphertext := aesgcm.Seal(nil, nonce, plaintext, nil)
    os.WriteFile(dst, append(nonce, ciphertext...), 0644)
}

func DecryptFile(src, dst string, key []byte) error {
    data, _ := os.ReadFile(src)
    nonce, ciphertext := data[:12], data[12:]
    plaintext, _ := aesgcm.Open(nil, nonce, ciphertext, nil)
    os.WriteFile(dst, plaintext, 0644)
}
```

**Key requirements:**
- Key file: 32 bytes binary
- Each file gets a random 12-byte nonce prefixed to ciphertext
- Encrypted files are NOT human-readable (different from plain copies)
- Decryption only needed for verify or restore operations

**File changes:**
- `pkg/config/types.go` — add `EncryptionConfig`
- `pkg/syncer/encrypt.go` — new file
- `pkg/syncer/syncer.go` — wire encryption in copy path

---

### 3.16 Scheduled Sync (cron-style)

**Goal:** Instead of continuous watching, sync on a schedule (every N minutes, at specific times).

**Config:**

```conf
[task]
name = "daily-backup"
source = "./data"
target = "./backup"

[sync]
schedule = "0 */6 * * *"    # cron expression: every 6 hours
# OR
schedule = "@every 5m"      # Go duration shorthand
```

**Implementation:**

Use `github.com/robfig/cron/v3`:

```go
type Scheduler struct {
    cron *cron.Cron
}

func (s *Scheduler) AddTask(task *config.ResolvedConfig, syncFn func()) error {
    _, err := s.cron.AddFunc(task.Schedule, syncFn)
    return err
}
```

When `schedule` is set:
- Skip file watcher for this task
- On each tick: walk source, diff against last manifest, sync changed files
- Record last-sync time in manifest

**File changes:**
- `pkg/config/types.go` — add `Schedule string` to `TaskSection`
- `pkg/scheduler/` — new package
- `pkg/core/app.go` — use scheduler if schedule is configured
- `go.mod` — add `github.com/robfig/cron/v3` dependency

---

## 4. Data Flow Changes

### Architecture after Phase C

```
                    +-------------------+
                    |   Meta-Watcher    |  (hot-reload)
                    |  (config files)   |
                    +--------+----------+
                             |
                    +--------v----------+
                    |   App Controller  |
                    |  (metrics, web,   |
                    |   history, etc.)  |
                    +--+-------+------+-+
                       |       |      |
              +--------v---+ +-v------v-+ +--------v---+
              | Task Mgmt  | | Stats    | | Web Server |
              | (pause/res) | | (atomic) | | (optional) |
              +--------+---+ +----------+ +------------+
                       |
           +-----------+-----------+
           |                       |
    +------v------+        +------v------+
    | Watcher     |        | Scheduler   |  (if cron mode)
    | (fsnotify)  |        | (cron)      |
    +------+------+        +------+------+
           |                       |
    +------v------+        +------v------+
    | Debouncer   |        | Full Sync   |
    +------+------+        | (walk+diff) |
           |               +------+------+
    +------v------+               |
    | Filter      |               |
    +------+------+               |
           |                      |
    +------v----------------------v--+
    |     Event Channel (buffered)   |
    +------+----------------------+--+
           |                      |
    +------v------+        +------v------+
    | Conflict    |        | Encrypt     |
    | Detection   |        | (optional)  |
    +------+------+        +------+------+
           |                      |
    +------v----------------------v--+
    |   Syncer Worker Pool (M goro)  |
    |   + Rate Limiter               |
    |   + File Permissions           |
    |   + Symlink Handling           |
    +------+-------------------------+
           |
    +------v------+
    | Trigger     |
    | Script Exec |
    +------+------+
           |
    +------v------+
    | History     |
    | Audit Log   |
    +-------------+
```

---

## 5. Config Schema Changes

### sync.yaml (new fields marked with 🔥)

```yaml
log:
  level: info
  output: console
  file:
    path: ./logs/sync.log
    max_size: 100
    max_backups: 7
    compress: true

watch:
  debounce: 500
  interval: 100
  workers: 0
  hot_reload: false                    # 🔥 Phase A — 3.4

sync:
  mode: incremental                    # full | incremental | once (Phase C)
  workers: 4
  backup: true
  verify: true
  preserve_permissions: false          # 🔥 Phase B — 3.9
  preserve_owner: false                # 🔥 Phase B — 3.9
  preserve_timestamps: false           # 🔥 Phase B — 3.9
  symlinks: "follow"                   # 🔥 Phase B — 3.11
  bandwidth_limit: 0                   # 🔥 Phase C — 3.12
  conflict_detection: false            # 🔥 Phase C — 3.10
  conflict_resolution: "warn"          # 🔥 Phase C — 3.10
  exclude:
    - "*.tmp"
    - "*.log"
    - ".git/"
    - "node_modules/"
  include: []
  encryption:                          # 🔥 Phase C — 3.15
    enabled: false
    algorithm: "aes-256-gcm"
    key_file: ""

conf:
  dir: ./conf
  entry: ["sync.conf"]                 # 🔥 Phase B — 3.7 (now supports list)

history:                               # 🔥 Phase C — 3.14
  enabled: false
  path: ./sync-history.jsonl
  max_entries: 10000

web:                                   # 🔥 Phase C — 3.13
  enabled: false
  listen: ":8080"
  dashboard: false

shutdown:
  timeout: 30
```

### .conf file (new fields marked with 🔥)

```conf
[task]
name = "my-task"
source = ["./src/a", "./src/b"]       # 🔥 Phase B — 3.8 (also supports single string)
target = ["./dst/a", "./dst/b"]        # 🔥 Phase B — 3.8
mode = "sync"
exclude = ["*.bak"]
include = []
delete_orphans = false
symlinks = "follow"                    # 🔥 Phase B — 3.11 (per-task override)

[watch]
recursive = true
events = ["create", "write", "remove", "rename"]
debounce = 500

[trigger]                              # 🔥 Phase A — 3.3
on_sync = "echo '{{relpath}} synced'"
on_complete = "deploy.bat"
on_error = "notify-error.bat {{error}}"
timeout = 30

[sync]                                 # 🔥 Phase A — 3.16
schedule = ""                          # cron expression or Go duration

[inherit]
file = "base.conf"
```

---

## 6. CLI Changes

```
go-file-sync run       Start the sync engine (continuous watch)
go-file-sync sync      One-shot sync: sync all files and exit       # NEW
go-file-sync check     Validate configuration files
go-file-sync version   Print version information
go-file-sync status    Print sync status and metrics                # NEW

Flags:
  --config, -c    Path to config file (default: sync.yaml)
  --verbose, -v   Enable verbose logging
  --dry-run       Show what would be synced without doing it        # NEW
```

---

## 7. Implementation Order

### Phase A (v0.3.0) — Core Improvements

| Step | Feature | Files to Create/Modify | Approx. LOC |
|------|---------|----------------------|-------------|
| 1 | Initial Full Sync | `pkg/core/app.go` (+80 lines) | 80 |
| 2 | Sync Status / Metrics | `pkg/stats/` (new, ~150 lines), `pkg/syncer/syncer.go` (+30) | 180 |
| 3 | Dry-Run Mode | `pkg/syncer/syncer.go` (+15), `cmd/root.go` (+5) | 20 |
| 4 | One-Shot Mode (`sync` cmd) | `cmd/root.go` (+30), `pkg/core/app.go` (+20) | 50 |
| 5 | Trigger Script Hooks | `pkg/config/types.go` (+8), `pkg/trigger/` (new, ~120 lines), `pkg/core/app.go` (+40) | 170 |
| 6 | Hot-Reload Config | `pkg/core/app.go` (+150 lines), `pkg/config/types.go` (+3) | 155 |
| **Phase A total** | | | **~655 lines** |

### Phase B (v0.4.0) — Multi-Config & Routing

| Step | Feature | Files to Create/Modify | Approx. LOC |
|------|---------|----------------------|-------------|
| 1 | Multiple Conf Entries | `pkg/config/types.go` (+5), `pkg/config/merge.go` (+30), `pkg/config/yaml.go` (+15) | 50 |
| 2 | Multi-Source / Multi-Target | `pkg/config/types.go` (+15), `pkg/config/merge.go` (+20), `pkg/core/app.go` (+40) | 75 |
| 3 | Symlink Handling | `pkg/config/types.go` (+3), `pkg/watcher/filter.go` (+20), `pkg/syncer/syncer.go` (+30) | 55 |
| 4 | File Permission & Attributes | `pkg/config/types.go` (+3), `pkg/syncer/syncer.go` (+40) | 45 |
| **Phase B total** | | | **~225 lines** |

### Phase C (v0.5.0) — Advanced

| Step | Feature | Files to Create/Modify | Approx. LOC |
|------|---------|----------------------|-------------|
| 1 | Sync History / Audit Log | `pkg/config/types.go` (+5), `pkg/history/` (new, ~100 lines) | 105 |
| 2 | Conflict Detection (Manifest) | `pkg/config/manifest.go` (new, ~120 lines), `pkg/syncer/syncer.go` (+30) | 150 |
| 3 | Scheduled Sync (cron) | `pkg/scheduler/` (new, ~100 lines), `pkg/core/app.go` (+40) | 140 |
| 4 | Network Bandwidth Limiter | `pkg/syncer/syncer.go` (+50), `go.mod` (+1) | 50 |
| 5 | Web Dashboard / HTTP API | `pkg/web/` (new, ~300 lines), `pkg/core/app.go` (+20) | 320 |
| 6 | Encrypted Sync | `pkg/syncer/encrypt.go` (new, ~120 lines), `pkg/syncer/syncer.go` (+20) | 140 |
| **Phase C total** | | | **~905 lines** |

```
Total new code: ~1,785 lines
Total new files: ~7 packages, ~15 files
```

---

## Appendix: Backward Compatibility

All features are **opt-in** with sensible defaults. Existing `sync.yaml` and `.conf` files remain valid without changes:

| Feature | Default (disabled) | Breaks existing config? |
|---------|-------------------|------------------------|
| Initial Full Sync | Only when `mode: full` | No |
| Dry-Run | Requires `--dry-run` flag | No |
| Trigger Scripts | No `[trigger]` section | No |
| Hot-Reload | `hot_reload: false` | No |
| Multiple Entries | Single string entry works as before | No |
| Multi-Source | Single string source works as before | No |
| Permissions | All `false` | No |
| Symlinks | `follow` (current behavior) | No |
| Conflict Detection | `false` | No |
| Web Dashboard | `enabled: false` | No |
| Scheduled Sync | No `schedule` field | No |
