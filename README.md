# go-file-sync

A lightweight file and directory auto-sync tool written in Go. It periodically
syncs configured source paths to target locations and ships a web dashboard
(Vue 3) for managing tasks and monitoring progress in real time.

**Version: v0.3.0** — architecture simplified to a single DB-driven engine.

## Architecture

The application is a single Go binary. Tasks and sync logs are persisted in a
local SQLite database (`go-file-sync.db`, created next to the executable). A
sync manager polls each enabled task on its own interval, walks the source, and
copies changed files using a concurrent worker pool backed by `pkg/syncer`.

```
main.go
 ├─ pkg/configdb     SQLite: sync_tasks / sync_logs (schema auto-migrates)
 ├─ pkg/syncmanager  Manager: per-task polling loop + shared WorkerPool
 │     └─ pkg/syncer FileSyncer (copy / backup / verify / symlinks /
 │                    perms / times / owner / bandwidth / conflict)
 └─ pkg/web          HTTP API + embedded Vue3 dashboard
```

### Concurrency model

- Each enabled task runs its own polling goroutine (`monitor_interval` seconds).
- All tasks share a worker pool of goroutines. Every file becomes a `Job`
  routed to a worker by the consistent hash of its relative path, so operations
  on the same file are processed in order.
- Results are reported back to the manager, which writes an audit-log row and
  updates in-memory stats exposed via `/api/sync-stats`.

## Features

- Web dashboard & HTTP API for task management and live stats
- Per-task sync direction:
  - `one_way_upload` — source is master, always copy `src -> dst`
  - `one_way_download` — target is master, always copy `dst -> src`
  - `two_way` — copy whichever side is newer
- Concurrent sync via a worker pool
- Optional SHA-256 verification after copy
- Optional backup of overwritten files
- Symlink handling: `follow` / `copy` / `skip`
- Preserve permissions / timestamps / owner (owner on Unix)
- Bandwidth limiter (bytes/sec)
- Conflict detection with resolution `warn` / `skip` / `overwrite`
- Audit log of every sync operation (stored in the database)
- Cross-platform single binary (Windows / macOS / Linux)

## Quick Start

```bash
make build        # builds the Vue3 frontend, then the binary
# or build the backend only:
go build -o go-file-sync .
./go-file-sync    # opens http://localhost:8080
```

Open the dashboard, create a task (source, target, direction, interval,
options), and it begins syncing immediately.

### Command-line flags

| Flag     | Default | Description                              |
|----------|---------|------------------------------------------|
| `--port` | `8080`  | HTTP listen port for the dashboard / API |

## HTTP API

| Endpoint             | Method        | Description                     |
|----------------------|---------------|---------------------------------|
| `/api/tasks`         | GET / POST    | List / create tasks             |
| `/api/tasks/{id}`    | GET / PUT / DELETE | Get / update / delete a task |
| `/api/logs`          | GET           | Recent sync logs                |
| `/api/logs/{id}`     | GET           | Logs for a specific task        |
| `/api/stats`         | GET           | Aggregate task / log stats      |
| `/api/sync-stats`    | GET           | Live monitored / synced counts |
| `/api/files?path=`   | GET           | Browse a directory              |
| `/api/files/read?path=` | GET       | File info                       |
| `/api/drives`        | GET           | List drives (Windows)           |

## Per-task options

| Field               | Default  | Description                                  |
|---------------------|----------|----------------------------------------------|
| `backup`            | `false`  | Back up target file before overwrite         |
| `verify`            | `false`  | SHA-256 verify after copy                    |
| `preserve_perms`    | `false`  | Preserve file permissions                    |
| `preserve_owner`    | `false`  | Preserve owner (Unix)                        |
| `preserve_times`    | `false`  | Preserve modification time                   |
| `symlinks`          | `follow` | `follow` / `copy` / `skip`                   |
| `bandwidth_limit`   | `0`      | Bytes/sec (0 = unlimited)                    |
| `conflict_detection`| `false`  | Detect out-of-band target changes           |
| `conflict_resolution`| `warn`  | `warn` / `skip` / `overwrite`                |

## Development

```bash
make build      # build frontend + binary
make test       # go test ./...
make lint       # go vet ./...
make verify     # go build ./... (compile check)
```

## Notes — removed components

The earlier YAML + `.conf` + fsnotify event-driven engine
(`pkg/core`, `pkg/config`, `pkg/watcher`, `pkg/trigger`, `pkg/history`,
`pkg/logger`) has been removed. The DB-driven engine described above is now the
only implementation; its sync options are configured per task in the database
rather than via config files.

## License

MIT
