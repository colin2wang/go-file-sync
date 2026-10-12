# AGENTS.md — FileSync 项目指引

## 构建与测试
- 前端：`cd frontend && pnpm install && pnpm run build`（vue-tsc 类型检查 + vite build；lockfile 已存在，用 pnpm 不要用 npm）
- 整体打包：`./build.ps1`（前端构建 → go-winres 生成 .syso 嵌入 `build/windows/icon.ico` → 交叉编译）
- Go 编译：`GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -tags "desktop,production" -ldflags "-w -s -H windowsgui" -o build/bin/FileSync.exe .`
- 测试：`go test ./...`（测试集中在 internal/config 与 internal/engine）
- 绕过 wails CLI 直接 `go build` 是有意为之；图标靠 go-winres 生成的 `rsrc_windows_amd64.syso` 链入，编译后即删除

## 架构（详见 ARCHITECTURE.md）
- Wails v2 桌面应用：`main.go`/`app.go` 是绑定层，业务在 `internal/`：engine（同步语义）、config（每任务一个 `{id}.yml`，Render 带中文注释、Parse 用 KnownFields(true) 严格解析）、scheduler（30s tick 轮询，NextRun 持久化）、state（同步后两侧 post 快照）、filter
- 前端 Vue3 + Pinia（无 router，视图切换靠 store）；`window.go` 不存在时走 MockBackend，可在浏览器无后端调试
- 前端调后端走 `frontend/wailsjs/` 生成绑定（已入库，改动 app.go 后需重新生成）

## 关键约定
- YAML 字段与语义（mode/conflict/mirror/incremental、`.filesync-trash/` 备份机制）改动属 breaking change，会破坏用户既有配置，须同步更新 `示例配置-文档备份.yml` 与 ARCHITECTURE.md
- 增量判定依赖复制后 Chtimes 保留源 mtime（|mtime差|≤1s 视为未变化），勿移除该逻辑
- wails.json 里 frontend:install/build 仍写的是 npm，实际以 pnpm 为准

## 维护规则
项目结构、构建/测试命令、架构边界、开发约定或本文件记录的其他事实发生变化时，必须在同一次改动中同步更新本文件。
