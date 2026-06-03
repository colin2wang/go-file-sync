# go-file-sync

A lightweight file and directory auto-sync tool written in Go. It watches specified files and directories for changes and automatically syncs them to target locations using concurrent goroutines.

## Features

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

## Installation

### Prerequisites

- Go 1.26 or later

### From source

```bash
git clone https://github.com/yourusername/go-file-sync.git
cd go-file-sync
make build
```

### Using go install

```bash
go install github.com/yourusername/go-file-sync@latest
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

## Configuration

### sync.yaml

The global configuration file defines default settings that apply to all tasks.

| Section | Key | Default | Description |
|---------|-----|---------|-------------|
| `log.level` | | `info` | Log level: debug, info, warn, error |
| `log.output` | | `console` | Output mode: console, file, both |
| `watch.debounce` | | `500` | Debounce interval in milliseconds |
| `watch.workers` | | `0` | Watcher goroutines (0 = one per task) |
| `sync.mode` | | `incremental` | Sync mode: full, incremental |
| `sync.workers` | | `4` | Syncer worker goroutines |
| `sync.backup` | | `true` | Backup target files before overwrite |
| `sync.verify` | | `true` | SHA-256 verification after copy |
| `shutdown.timeout` | | `30` | Graceful shutdown timeout in seconds |

### .conf files

Each `.conf` file defines a sync task. Files support inheritance via the `[inherit]` section.

**Inheritance rules:**
- Scalar values (name, source, target, mode): child overrides parent
- Array values (exclude, include): child appends to parent's list
- Max inheritance depth: 10 levels
- Circular inheritance is detected and rejected

## Usage

```
go-file-sync run      Start the sync engine
go-file-sync check    Validate configuration files
go-file-sync version  Print version information
```

### Command-line flags

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--config` | `-c` | `sync.yaml` | Path to the config file |
| `--verbose` | `-v` | `false` | Enable verbose logging |

## Architecture

```
┌─────────────────────────────────────┐
│           CLI Layer (cobra)          │
│   root.go — command parsing          │
├─────────────────────────────────────┤
│           Engine Layer               │
│  ┌─────────┐ ┌─────────┐ ┌────────┐ │
│  │ Watcher │ │ Syncer  │ │ Logger │ │
│  │ Pool    │ │ Pool    │ │        │ │
│  │ (N goro)│ │ (M goro)│ │(thread │ │
│  └────┬────┘ └────┬────┘ │ safe)  │ │
│       │            │      └────────┘ │
│  ┌────▼────┐ ┌────▼────┐            │
│  │Shutdown │ │ Event   │            │
│  │ Signal  │ │ Channel │            │
│  └─────────┘ └─────────┘            │
├─────────────────────────────────────┤
│           Config Layer               │
│  ┌─────────┐ ┌─────────┐ ┌────────┐ │
│  │  YAML   │ │  Conf   │ │ Default│ │
│  │ Config  │ │ Config  │ │ Values │ │
│  └─────────┘ └─────────┘ └────────┘ │
└─────────────────────────────────────┘
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
