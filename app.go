package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"github.com/google/uuid"
	wails "github.com/wailsapp/wails/v2/pkg/runtime"

	"filesync/internal/config"
	"filesync/internal/engine"
	"filesync/internal/filter"
	"filesync/internal/logger"
	"filesync/internal/scheduler"
	"filesync/internal/state"
)

// App Wails 绑定层
type App struct {
	ctx        context.Context
	store      *config.Store
	states     *state.Store
	sched      *scheduler.Scheduler
	baseDir    string // UserConfigDir/FileSync
	settingsMu sync.Mutex

	mu      sync.Mutex
	running map[string]context.CancelFunc
}

// Settings 应用设置（语言/主题）
type Settings struct {
	Language string `json:"language"` // zh-CN | en-US
	Theme    string `json:"theme"`    // dark | light
}

// TaskSummary 任务 + 运行状态快照
type TaskSummary struct {
	config.TaskConfig
	NextRun    time.Time        `json:"nextRun"`
	LastRun    time.Time        `json:"lastRun"`
	LastResult state.RunSummary `json:"lastResult"`
	Running    bool             `json:"running"`
}

// PreviewResult 干跑结果
type PreviewResult struct {
	Actions      []engine.Action  `json:"actions"`
	Stats        engine.PlanStats `json:"stats"`
	Truncated    bool             `json:"truncated"`
	LeftMissing  bool             `json:"leftMissing"`
	RightMissing bool             `json:"rightMissing"`
	HasPrevState bool             `json:"hasPrevState"`
}

func NewApp() *App {
	return &App{running: map[string]context.CancelFunc{}}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	base, err := os.UserConfigDir()
	if err != nil {
		base = "."
	}
	a.baseDir = filepath.Join(base, "FileSync")
	a.store = config.NewStore(filepath.Join(a.baseDir, "tasks"))
	a.states = state.NewStore(filepath.Join(a.baseDir, "state"))
	_ = os.MkdirAll(a.store.Dir, 0o755)
	_ = os.MkdirAll(a.states.Dir, 0o755)
	a.sched = scheduler.New(a.store, a.states, func(id string, manual bool) {
		_ = a.RunTask(id)
	}, 30*time.Second)
	a.sched.Start()
}

func (a *App) shutdown(ctx context.Context) {
	if a.sched != nil {
		a.sched.Stop()
	}
}

// ---------- 任务 CRUD ----------

// ListTasks 列出全部任务（含运行状态）
func (a *App) ListTasks() ([]TaskSummary, error) {
	cfgs, err := a.store.List()
	if err != nil {
		return nil, err
	}
	out := make([]TaskSummary, 0, len(cfgs))
	for i := range cfgs {
		out = append(out, a.summary(&cfgs[i]))
	}
	return out, nil
}

func (a *App) summary(c *config.TaskConfig) TaskSummary {
	st := a.states.Load(c.ID)
	return TaskSummary{
		TaskConfig: *c,
		NextRun:    st.NextRun,
		LastRun:    st.LastRun,
		LastResult: derefSummary(st.Last),
		Running:    a.IsRunning(c.ID),
	}
}

// GetTask 读取单个任务
func (a *App) GetTask(id string) (*config.TaskConfig, error) {
	return a.store.Load(id)
}

// NewTask 返回向导起始模板
func (a *App) NewTask() config.TaskConfig {
	return config.Default()
}

// SaveTask 新建/更新任务
func (a *App) SaveTask(c *config.TaskConfig) (*config.TaskConfig, error) {
	if err := a.store.Save(c); err != nil {
		return nil, err
	}
	// 保存后重算下次运行时间
	st := a.states.Load(c.ID)
	st.NextRun = scheduler.ComputeNext(*c, st, time.Now())
	_ = a.states.Save(st)
	wails.EventsEmit(a.ctx, "sched:changed", map[string]any{"id": c.ID})
	return c, nil
}

// DeleteTask 删除任务及其状态
func (a *App) DeleteTask(id string) error {
	if a.IsRunning(id) {
		return fmt.Errorf("任务正在运行，无法删除")
	}
	_ = a.states.Delete(id)
	return a.store.Delete(id)
}

// ---------- YAML 所见即所得 ----------

// RenderYAML 配置 → 带注释 YAML（向导右栏预览）
func (a *App) RenderYAML(c config.TaskConfig) string {
	return config.RenderYAML(c)
}

// ParseYAML YAML 文本 → 配置（向导入回填表单）
func (a *App) ParseYAML(text string) (config.TaskConfig, error) {
	return config.ParseYAML(text)
}

// ---------- 同步执行 ----------

// PreviewPlan 干跑：返回将要执行的操作列表
func (a *App) PreviewPlan(id string) (*PreviewResult, error) {
	cfg, err := a.store.Load(id)
	if err != nil {
		return nil, err
	}
	m, err := filter.New(cfg.Filters.Exclude, cfg.Filters.Include, cfg.Filters.CaseSensitive)
	if err != nil {
		return nil, err
	}
	res := &PreviewResult{}
	if _, err := os.Stat(cfg.Left); err != nil {
		res.LeftMissing = true
	}
	if _, err := os.Stat(cfg.Right); err != nil {
		res.RightMissing = true
	}
	left, err := engine.SnapshotDir(cfg.Left, m)
	if err != nil {
		return nil, err
	}
	right, err := engine.SnapshotDir(cfg.Right, m)
	if err != nil {
		return nil, err
	}
	st := a.states.Load(id)
	res.HasPrevState = len(st.Entries) > 0
	plan := engine.Diff(left, right, st.Entries, *cfg)
	res.Actions = plan.Actions
	res.Stats = plan.Stats
	res.Truncated = plan.Truncated
	return res, nil
}

// RunTask 异步执行同步（事件推送进度）
func (a *App) RunTask(id string) error {
	cfg, err := a.store.Load(id)
	if err != nil {
		return err
	}
	if !cfg.Enabled {
		return fmt.Errorf("任务已停用: %s", cfg.Name)
	}
	a.mu.Lock()
	if _, busy := a.running[id]; busy {
		a.mu.Unlock()
		return fmt.Errorf("任务正在运行")
	}
	ctx, cancel := context.WithCancel(a.ctx)
	a.running[id] = cancel
	a.mu.Unlock()

	go a.execTask(ctx, cfg)
	return nil
}

// CancelTask 取消正在运行的任务
func (a *App) CancelTask(id string) error {
	a.mu.Lock()
	cancel, ok := a.running[id]
	a.mu.Unlock()
	if !ok {
		return fmt.Errorf("任务未在运行")
	}
	cancel()
	return nil
}

// IsRunning 任务是否运行中
func (a *App) IsRunning(id string) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	_, busy := a.running[id]
	return busy
}

func (a *App) execTask(ctx context.Context, cfg *config.TaskConfig) {
	id := cfg.ID
	defer func() {
		a.mu.Lock()
		delete(a.running, id)
		a.mu.Unlock()
	}()

	rep := &eventReporter{ctx: a.ctx, id: id}
	rep.Log("info", "任务「%s」开始执行（模式: %s）", cfg.Name, cfg.Mode)
	started := time.Now()
	wails.EventsEmit(a.ctx, "task:run-started", map[string]any{"id": id, "name": cfg.Name})

	st := a.states.Load(id)
	m, err := filter.New(cfg.Filters.Exclude, cfg.Filters.Include, cfg.Filters.CaseSensitive)
	if err != nil {
		a.finish(id, st, "error", started, 0, 0, 0, 0, 0, 0, err.Error())
		return
	}

	left, err := engine.SnapshotDir(cfg.Left, m)
	if err != nil {
		a.finish(id, st, "error", started, 0, 0, 0, 0, 0, 0, "扫描左侧失败: "+err.Error())
		return
	}
	rep.Log("info", "左侧扫描完成: %d 个文件", len(left))
	right, err := engine.SnapshotDir(cfg.Right, m)
	if err != nil {
		a.finish(id, st, "error", started, 0, 0, 0, 0, 0, 0, "扫描右侧失败: "+err.Error())
		return
	}
	rep.Log("info", "右侧扫描完成: %d 个文件", len(right))

	plan := engine.Diff(left, right, st.Entries, *cfg)
	rep.Log("info", "计划: 复制→右 %d、复制→左 %d、删除 %d、冲突 %d、跳过 %d",
		plan.Stats.CopyL2R, plan.Stats.CopyR2L, plan.Stats.DelLeft+plan.Stats.DelRight, plan.Stats.Conflict, plan.Stats.Skip)
	if plan.Stats.Total() == 0 && plan.Stats.Unchanged > 0 {
		rep.Log("info", "全部文件均为最新（%d 个未变化），无需同步", plan.Stats.Unchanged)
	}

	result, err := engine.Execute(ctx, *cfg, plan, rep)
	if err != nil && ctx.Err() != nil {
		a.finish(id, st, "cancelled", started, int64(result.CopiedL2R+result.CopiedR2L), int64(result.Deleted), int64(result.Skipped), int64(result.Conflicts), int64(result.Failed), result.Bytes, "已取消")
		return
	}
	if result.Failed > 0 {
		rep.Log("warn", "完成但有 %d 个失败项", result.Failed)
	} else {
		rep.Log("info", "同步完成: 复制 %d 个（%s）、删除 %d 个、跳过 %d 个",
			result.CopiedL2R+result.CopiedR2L, humanBytes(result.Bytes), result.Deleted, result.Skipped)
	}

	// 更新状态与下次运行时间
	post, perr := engine.PostSnapshots(*cfg, m)
	if perr == nil {
		st.Entries = post
	} else {
		rep.Log("warn", "更新同步清单失败: %v", perr)
	}
	status := "ok"
	msg := ""
	if result.Failed > 0 {
		status = "error"
		msg = fmt.Sprintf("%d 个失败", result.Failed)
	}
	st.LastRun = time.Now()
	st.NextRun = scheduler.ComputeNext(*cfg, st, time.Now())
	st.Last = &state.RunSummary{
		Status: status, Copied: result.CopiedL2R + result.CopiedR2L, Deleted: result.Deleted,
		Skipped: result.Skipped, Conflicts: result.Conflicts, Failed: result.Failed,
		Bytes: result.Bytes, Message: msg, Finished: time.Now(),
	}
	_ = a.states.Save(st)
	wails.EventsEmit(a.ctx, "task:run-finished", map[string]any{"id": id, "result": st.Last})
	wails.EventsEmit(a.ctx, "sched:changed", map[string]any{"id": id})
}

func (a *App) finish(id string, st *state.State, status string, started time.Time, copied, deleted, skipped, conflicts, failed int64, bytes int64, msg string) {
	st.LastRun = time.Now()
	st.Last = &state.RunSummary{Status: status, Copied: int(copied), Deleted: int(deleted), Skipped: int(skipped), Conflicts: int(conflicts), Failed: int(failed), Bytes: bytes, Message: msg, Finished: time.Now()}
	if a.store != nil {
		if cfg, err := a.store.Load(id); err == nil {
			st.NextRun = scheduler.ComputeNext(*cfg, st, time.Now())
		}
	}
	_ = a.states.Save(st)
	wails.EventsEmit(a.ctx, "task:run-finished", map[string]any{"id": id, "result": st.Last})
	wails.EventsEmit(a.ctx, "sched:changed", map[string]any{"id": id})
}

// eventReporter 把引擎日志/进度转发为 Wails 事件（进度 50ms 节流）
type eventReporter struct {
	ctx  context.Context
	id   string
	mu   sync.Mutex
	last time.Time
}

func (r *eventReporter) Log(level, format string, args ...any) {
	text := fmt.Sprintf(format, args...)
	logger.Log("任务 %s [%s] %s", r.id, level, text)
	wails.EventsEmit(r.ctx, "task:log", map[string]any{
		"id": r.id, "level": level, "text": text, "time": time.Now().Format("15:04:05"),
	})
}

func (r *eventReporter) Progress(done, total int, path string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	now := time.Now()
	if done < total && now.Sub(r.last) < 50*time.Millisecond {
		return
	}
	r.last = now
	wails.EventsEmit(r.ctx, "task:progress", map[string]any{"id": r.id, "done": done, "total": total, "path": path})
}

// ---------- 对话框 / 系统集成 ----------

// PickFolder 目录选择对话框
func (a *App) PickFolder(title string) (string, error) {
	return wails.OpenDirectoryDialog(a.ctx, wails.OpenDialogOptions{Title: title})
}

// OpenInExplorer 在文件管理器中打开路径
func (a *App) OpenInExplorer(path string) error {
	if _, err := os.Stat(path); err != nil {
		_ = os.MkdirAll(path, 0o755)
	}
	switch runtime.GOOS {
	case "windows":
		return exec.Command("explorer", path).Start()
	case "darwin":
		return exec.Command("open", path).Start()
	default:
		return exec.Command("xdg-open", path).Start()
	}
}

// ConfigDir 返回任务配置目录（供前端展示）
func (a *App) ConfigDir() string {
	return a.store.Dir
}

// ---------- 设置 ----------

func (a *App) GetSettings() Settings {
	a.settingsMu.Lock()
	defer a.settingsMu.Unlock()
	def := Settings{Language: "zh-CN", Theme: "light"}
	data, err := os.ReadFile(filepath.Join(a.baseDir, "settings.json"))
	if err != nil {
		return def
	}
	s := def
	if err := json.Unmarshal(data, &s); err != nil {
		return def
	}
	if s.Language == "" {
		s.Language = "zh-CN"
	}
	if s.Theme != "dark" && s.Theme != "light" {
		s.Theme = "light"
	}
	return s
}

func (a *App) SaveSettings(s Settings) error {
	a.settingsMu.Lock()
	defer a.settingsMu.Unlock()
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(a.baseDir, "settings.json"), data, 0o644)
}

// NewID 生成任务 ID（备用）
func (a *App) NewID() string {
	return uuid.NewString()
}

func humanBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, e := int64(unit), 0
	for m := n / unit; m >= unit; m /= unit {
		div *= unit
		e++
	}
	return fmt.Sprintf("%.1f %cB", float64(n)/float64(div), "KMGTPE"[e])
}

func derefSummary(s *state.RunSummary) state.RunSummary {
	if s == nil {
		return state.RunSummary{}
	}
	return *s
}
