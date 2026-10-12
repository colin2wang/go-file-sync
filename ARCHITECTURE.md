# FileSync 架构决策记录（防上下文压缩）

## 项目
- 路径: /home/z/my-project/filesync
- 技术栈: Go 1.23 + Wails v2.10.1 + Vue3 + TS + Pinia + vue-i18n（无 UI 库，手写 CSS variables 主题）
- 交付: Windows PE32+ exe（交叉编译绕过 wails CLI，坑②）+ ZIP

## 需求映射
- 双向/单向/增量同步/定时/过滤 → internal/engine + scheduler
- 多文件夹=多 yml → config.Store 每任务一个 {id}.yml，UserConfigDir/FileSync/tasks/
- 所见即所得向导 → 前端左表单 watch → debounce 调 app.RenderYAML → 右栏预览；ParseYAML 支持导入回填
- 多语言 zh-CN/en-US、黑白主题 → vue-i18n + data-theme CSS variables；Settings 存后端 json

## YAML 配置格式（config.Render 带中文注释渲染，Parse 用 KnownFields(true) 严格解析）
name/enabled/left/right/mode(one-way|one-way-back|two-way)/incremental/mirror/conflict(newer|left|right|skip)/backup_deleted/filters{exclude[],include[],case_sensitive}/schedule{enabled,type(interval|daily),interval_minutes,daily_time"HH:MM"}/created/updated

## 引擎语义（engine.Diff）
- 增量: size 相同且 |mtime差|<=1s → skip（未变化不进 actions，只计数）
- one-way: left 权威；right 独有 → mirror? delete-right : skip
- two-way 删除传播: prevState(HasL/HasR) 区分「另一侧删除」vs「新增」；mirror 才传播删除，否则 skip 提示
- conflict 策略: newer/left/right/skip(skip=ActConflict 不执行)
- Execute: 覆盖/删除前 backup_deleted → <被写侧>/.filesync-trash/<ts>/；复制后 Chtimes 保留源 mtime（增量判定关键）
- 内置排除: .filesync-trash/ .filesync/

## 状态
- state.Store: UserConfigDir/FileSync/state/{id}.json；Entry{HasL,SizeL,MtimeL,HasR,SizeR,MtimeR}；同步成功后=两侧 post 快照
- 调度: scheduler 每 30s tick；interval=lastRun+N min（首跑 now+1min）；daily=今日HH:MM/明日；NextRun 持久化，重启补跑(nextRun<now)
- 事件: task:run-started/task:progress(50ms节流)/task:log/task:run-finished/sched:changed

## 绑定 API（app.go）
ListTasks/GetTask/SaveTask/DeleteTask/PreviewPlan/RunTask(异步goroutine)/CancelTask/PickFolder/RenderYAML/ParseYAML/GetSettings/SaveSettings/OpenInExplorer

## 前端
- 页面切换用 Pinia state（不引 router）；视图: tasks/wizard/settings
- Mock: window.go 不存在 → MockBackend（内存任务+模拟事件+TS复刻YAML渲染）保证浏览器无头实测
- 组件: TaskCard/RunPanel(抽屉)/PlanDialog(预览表格cap500)/YamlPane(mini高亮)/FilterEditor(chips)

## 环境（10/8 已固化）
export PATH=~/sdk/go:~/sdk/go/bin:~/go/bin:$PATH
fixown: /home/z/fixown.sh <dir>（Write 产生 root 属主，每批写完跑）
构建: GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -tags "desktop,production" -ldflags "-w -s -H windowsgui" -o build/bin/FileSync.exe .
图标: PIL 画 PNG → go-winres simply --icon --manifest gui --arch amd64 → syso 随 build 链入
PowerShell 续行不用 ^（一行写完）
