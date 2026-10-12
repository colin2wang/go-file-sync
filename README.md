# FileSync — Desktop File Sync & Backup Tool · User Guide

A Windows desktop file sync & backup tool built with Go + Wails v2 + Vue 3 + TypeScript.

English · [简体中文](README.zh-CN.md)

## Screenshots

| Task List | New Task Wizard (live YAML preview) | Settings |
|---|---|---|
| ![Task list](docs/images/tasks-list.png) | ![New task wizard](docs/images/wizard-yaml-preview.png) | ![Settings](docs/images/settings.png) |

*Task list, five-step wizard with WYSIWYG YAML preview, and the language/theme settings page in dark theme.*

## Features

| Feature | Description |
|---|---|
| Two-way sync | Two folders mirror each other; additions/changes on either side are copied across automatically. With the state manifest it distinguishes "deleted" from "new" and supports deletion propagation |
| One-way sync | Left→right or right→left; the source side is authoritative |
| Incremental sync | Compares file size + modification time and syncs only changed files; unchanged files are skipped automatically (shown as ✓N in previews) |
| Scheduled sync | Fixed interval (every N minutes) or daily at a fixed time (HH:MM); missed tasks are re-run automatically on app start |
| File filters | .gitignore-style glob rules: `*` any chars · `?` single char · `**` across directories · trailing `/` matches directories (e.g. `node_modules/`); supports exclude and include (when include is non-empty, only matching files are synced) and a case-sensitivity toggle |
| Multiple folder pairs | One folder pair = one YAML config file (`{task-id}.yml`); tasks are fully independent |
| YAML WYSIWYG | The wizard shows a step-by-step form on the left and renders the commented YAML live on the right (what you save is what lands on disk); "Import YAML" pastes an existing config back into the form |
| Sync preview | Dry run before executing: lists files to be copied/deleted/conflicted plus statistics, with an unchanged-file count |
| Safe backup | Before deleting or overwriting, old files are moved to `.filesync-trash/<timestamp>/` in the target directory |
| Multi-language | 简体中文 / English, one click at the bottom of the sidebar |
| Light/dark theme | One click at the bottom of the sidebar, persisted with settings |
| Run monitoring | Sync progress bar + live log stream (scrolling with timestamps); can be stopped at any time |

## Quick Start

1. Run `FileSync.exe` (portable single file, no installation needed)
2. Click **＋ New Task** and follow the 5-step wizard:
   - Basics: task name
   - Folders: pick the left/right folders + sync mode (one-way L→R / one-way R→L / two-way)
   - Strategy: incremental sync (recommended), mirror deletions, backup before delete, conflict policy (two-way only)
   - Filters: exclude rules (e.g. `*.tmp`, `node_modules/`) and include rules
   - Schedule: fixed interval or daily time
3. The YAML preview updates live on the right; click **Save** when everything looks right
4. In the task list, click **Preview** to check the planned operations, then **Sync Now**

## Config File Location

Task YAML files and run state are stored in the user config directory (the UI's "Settings → Config directory → Open folder" jumps straight there):

- Windows: `C:\Users\<username>\AppData\Roaming\FileSync\tasks\*.yml`
- State manifest: `...\FileSync\state\{task-id}.json` (the basis for distinguishing deletions from additions in two-way mode)

YAML files can be hand-edited; restart the app to apply, or use "Import YAML" inside the app.

Full example config (a documents-backup task):

- English comments: [`docs/example-config-documents-backup.yml`](docs/example-config-documents-backup.yml)
- 简体中文注释: [`docs/示例配置-文档备份.yml`](docs/示例配置-文档备份.yml)

## Sync Semantics in Detail

### One-way mode (one-way / one-way-back)
- On source, missing on target → copy
- On both sides but different → overwrite with the source version
- On target only → with "mirror deletions" enabled it is deleted on the target, otherwise kept and reported in previews

### Two-way mode (two-way)
- A file unique to one side → copied to the other side
- On both sides and different → resolved by the conflict policy: `newer wins` (default) / `left wins` / `right wins` / `skip & report`
- Deletion propagation: if a file existed on both sides in the last sync manifest and now disappears on one side, it counts as a deletion on that side; with "mirror deletions" enabled the deletion propagates, otherwise the file is kept and continuously reported
- The first sync has no history manifest, so deletion detection is unavailable (independent files on each side count as additions)

### Incremental detection
Same file size **and** modification times within ≤ 1 second → treated as unchanged and skipped. Copying preserves the source file's mtime so future incremental checks stay correct.

### Safety boundaries
- The app's own `.filesync-trash/` and `.filesync/` directories are always excluded to prevent sync loops
- Empty directories are not synced (files only)
- A single failing file does not abort the run; failures are listed in red in the log

## Build from Source

Requirements: Go 1.23+, Node 18+, Wails v2.10.1 (`go install github.com/wailsapp/wails/v2/cmd/wails@v2.10.1`).

One-click PowerShell build (`build.ps1`):

```powershell
.\build.ps1              # frontend build + cross-compile Windows exe
.\build.ps1 -Clean       # clean rebuild
.\build.ps1 -Package     # build + package ZIP
```

Manual commands:

```powershell
cd frontend; pnpm install; pnpm run build; cd ..
$env:GOOS="windows"; $env:GOARCH="amd64"; $env:CGO_ENABLED="0"
go build -tags "desktop,production" -ldflags "-w -s -H windowsgui" -o build/bin/FileSync.exe .
```

## Browser Demo Mode

The frontend ships with a mock backend: running `pnpm run dev` and opening it in a browser gives you demo mode (mock data + simulated syncs), handy for previewing the whole UI without a desktop environment; the desktop exe wires in the real sync engine.

## Known Limitations

- The delivery environment has no Windows GUI runtime; the exe has passed the full Go unit suite (config/filter/engine/scheduler, 27 cases) and a clean vue-tsc + headless browser UI test. Please verify GUI behavior on a real machine
- No symlink following, no permission/ownership sync, no checksum-level comparison (size + mtime is the incremental basis)

---
Version 1.0 · 2026-10-12 · Go 1.23 / Wails v2.10.1 / Vue 3.2 / TypeScript
