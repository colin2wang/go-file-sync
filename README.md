# go-file-sync

A lightweight file and directory auto-sync tool written in Go. It watches specified files and directories for changes and automatically syncs them to target locations using concurrent goroutines.

**Version: v0.3.0**

## Features

### Core
- **Concurrent file watching** — Each sync task runs in its own goroutine, watching for create/write/remove/rename events via [fsnotify](https://github.com/fsnotify/fsnotify)
- **Two-layer configuration** — `sync.yaml` for global settings + `*.conf` files for per-task configuration with inheritance
- **Conf file inheritance** — Child conf files inherit and can override parent properties; array fields (exclude/include) are appended
- **Debouncing** — Rapid changes to the same file are coalesced into a single event to avoid redundant syncs
- **Event filtering** — Include/exclude patterns using glob matching
- **Parallel sync** — Worker pool (M goroutines) processes sync events concurrently; consistent hashing ensures per-file ordering
- **Thread-safe logging** — Configurable output (console/file/both) with log levels and file rotation
- **Graceful shutdown** — Captures SIGINT/SIGTERM, waits for in-flight syncs to complete
- **Hash verification** — Optional SHA-256 verification after file copy
- **Cross-platform** — Compiles to a single binary for Windows, macOS, and Linux

### Phase A (v0.3.0) — New
- **Initial full sync** — On startup, performs a one-time full copy of all source files to target (concurrent, configurable via `sync.mode: full`)
- **Dry-run mode** — `--dry-run` flag shows what would be synced without performing actual file operations
- **Trigger scripts** — Run external commands on sync events (`on_sync`, `on_complete`, `on_error`) with template variable substitution (`{{relpath}}`, `{{src}}`, `{{dst}}`, etc.)
- **Hot-reload configuration** — Enable `watch.hot_reload: true` to automatically reload config when files change (no restart needed)
- **Sync status & metrics** — `go-file-sync status` command prints real-time metrics (files synced, bytes transferred, errors, uptime)
- **One-shot mode** — `go-file-sync sync` performs a full sync and exits (no continuous watching)

### Phase B (v0.4.0) — New
- **Multiple conf entries** — Configure multiple entry files in `conf.entry: ["sync.conf", "extra.conf"]`
- **Multi-source / Multi-target** — One task can sync multiple source directories to one or more targets
- **Symlink handling** — Configurable: `follow` (default, resolve symlink), `copy` (recreate symlink), `skip` (ignore symlinks)
- **File permission & attribute sync** — Preserve permissions, timestamps, and ownership across sync

### Phase C (v0.5.0) — New
- **Conflict detection** — Maintains a SHA-256 manifest on the target to detect out-of-band modifications (`conflict_resolution: warn | skip | backup`)
- **Encrypted sync** — AES-256-GCM encryption for syncing sensitive files to untrusted targets
- **Bandwidth limiter** — Throttle sync speed with `sync.bandwidth_limit` (bytes/sec)
- **Web dashboard** — Optional HTTP API (`/api/v1/*`) and HTML dashboard for real-time monitoring, pause/resume/reload controls
- **Sync history / audit log** — JSONL-formatted persistent log of every sync operation
- **Scheduled sync** — Cron expressions for time-based sync instead of continuous watching (`task.schedule = "0 */6 * * *"`)

## Installation

### Prerequisites

- Go 1.26 or later

### From source

```bash
git clone https://github.com/colin2wang/go-file-sync.git
cd go-file-sync
make build
```

### Using go install

```bash
go install github.com/colin2wang/go-file-sync@latest
```

## Quick Start

### 1. Create configuration

**sync.yaml** (global configuration):

```yaml
log:
  level: info
  output: console

watch:
  debounce: 500

sync:
  mode: incremental
  workers: 4
  exclude:
    - "*.tmp"
    - ".git/"
    - "node_modules/"

conf:
  dir: ./conf
  entry:
    - sync.conf
```

**conf/sync.conf** (task configuration):

```conf
[task]
name = "my-project"
source = "./src"
target = "./dist"
mode = "sync"

[watch]
recursive = true
events = ["create", "write", "remove", "rename"]
debounce = 500
```

### 2. Run

```bash
go-file-sync run
# or
make run
```

## Usage

```
go-file-sync run       Start the sync engine (continuous watch)
go-file-sync sync      One-shot sync: sync all files and exit
go-file-sync check     Validate configuration files
go-file-sync version   Print version information
go-file-sync status    Print sync status and metrics
```

### Command-line flags

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--config` | `-c` | `sync.yaml` | Path to the config file |
| `--verbose` | `-v` | `false` | Enable verbose logging |
| `--dry-run` | `-n` | `false` | Show what would be synced without doing it |

## Configuration

### sync.yaml

The global configuration file defines default settings that apply to all tasks.

| Section | Key | Default | Description |
|---------|-----|---------|-------------|
| `log.level` | | `info` | Log level: debug, info, warn, error |
| `log.output` | | `console` | Output mode: console, file, both |
| `watch.debounce` | | `500` | Debounce interval in milliseconds |
| `watch.workers` | | `0` | Watcher goroutines (0 = one per task) |
| `watch.hot_reload` | | `false` | Enable hot-reload of config files |
| `sync.mode` | | `incremental` | Sync mode: full, incremental |
| `sync.workers` | | `4` | Syncer worker goroutines |
| `sync.backup` | | `true` | Backup target files before overwrite |
| `sync.verify` | | `true` | SHA-256 verification after copy |
| `sync.preserve_permissions` | | `false` | Preserve file permissions |
| `sync.preserve_owner` | | `false` | Preserve file ownership |
| `sync.preserve_timestamps` | | `false` | Preserve file timestamps |
| `sync.symlinks` | | `follow` | Symlink mode: follow, copy, skip |
| `sync.bandwidth_limit` | | `0` | Bandwidth limit (bytes/sec, 0=unlimited) |
| `sync.conflict_detection` | | `false` | Enable conflict detection |
| `sync.conflict_resolution` | | `warn` | Conflict resolution: warn, skip, backup |
| `sync.encryption.enabled` | | `false` | Enable AES-256-GCM encryption |
| `shutdown.timeout` | | `30` | Graceful shutdown timeout in seconds |
| `history.enabled` | | `false` | Enable sync history / audit log |
| `web.enabled` | | `false` | Enable HTTP API and dashboard |
| `web.listen` | | `:8080` | Web server listen address |

### .conf files

Each `.conf` file defines a sync task. Files support inheritance via the `[inherit]` section.

**Inheritance rules:**
- Scalar values (name, source, target, mode): child overrides parent
- Array values (exclude, include): child appends to parent's list
- Max inheritance depth: 10 levels
- Circular inheritance is detected and rejected

**Per-task options:**

| Section | Key | Description |
|---------|-----|-------------|
| `[task]` | `name` | Task name |
| `[task]` | `source` | Source directory (single) |
| `[task]` | `target` | Target directory (single) |
| `[task]` | `sources` | Multiple source directories |
| `[task]` | `targets` | Multiple target directories |
| `[task]` | `mode` | Sync mode: sync, mirror |
| `[task]` | `delete_orphans` | Delete files in target that don't exist in source |
| `[task]` | `symlinks` | Symlink handling (overrides global) |
| `[task]` | `schedule` | Cron expression for scheduled sync |
| `[task]` | `exclude` | Glob patterns to exclude |
| `[task]` | `include` | Glob patterns to include |
| `[watch]` | `recursive` | Watch subdirectories recursively |
| `[watch]` | `events` | Event types: create, write, remove, rename |
| `[watch]` | `debounce` | Per-task debounce override |
| `[trigger]` | `on_sync` | Command to run after each file sync |
| `[trigger]` | `on_complete` | Command to run after batch completes |
| `[trigger]` | `on_error` | Command to run on sync failure |
| `[trigger]` | `timeout` | Command timeout in seconds |
| `[inherit]` | `file` | Parent conf file to inherit from |

### Trigger template variables

| Variable | Description |
|----------|-------------|
| `{{relpath}}` | Relative file path |
| `{{src}}` | Full source path |
| `{{dst}}` | Full target path |
| `{{task}}` | Task name |
| `{{event}}` | Event type (create/write/remove/rename) |
| `{{error}}` | Error message (on_error only) |

## Architecture

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

### Concurrency model

- Each task has one watcher goroutine and one debouncer goroutine
- Syncer uses a configurable worker pool (M goroutines)
- Per-file ordering: files with the same relative path are routed to the same worker via consistent hashing
- Graceful shutdown: signal → cancel context → wait for all goroutines

## Development

```bash
make build      # Build binary
make test       # Run tests
make lint       # Run linters
make clean      # Clean build artifacts
```

### Cross-compilation

```bash
make build-linux     # Linux amd64
make build-darwin    # macOS amd64
make build-windows   # Windows amd64
make build-all       # All platforms
```

## License

MIT
