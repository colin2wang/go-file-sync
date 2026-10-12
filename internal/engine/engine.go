// Package engine 实现文件同步核心：目录快照、差异计划与执行。
package engine

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"filesync/internal/config"
	"filesync/internal/filter"
	"filesync/internal/logger"
	"filesync/internal/state"
)

// TrashDirName 删除/覆盖前的备份目录名（位于被写侧根目录）
const TrashDirName = ".filesync-trash"

// FileInfo 文件快照条目
type FileInfo struct {
	Path    string `json:"path"` // 相对路径，斜杠分隔
	Size    int64  `json:"size"`
	ModTime int64  `json:"modTime"` // unix nano
}

// Snapshot 相对路径 → 文件信息
type Snapshot map[string]FileInfo

// ActionType 操作类型
type ActionType string

const (
	ActCopyL2R  ActionType = "copy-l2r"
	ActCopyR2L  ActionType = "copy-r2l"
	ActDelLeft  ActionType = "delete-left"
	ActDelRight ActionType = "delete-right"
	ActConflict ActionType = "conflict" // 仅报告，不执行
	ActSkip     ActionType = "skip"     // 保留现状（附原因）
)

// Reason 操作原因
type Reason string

const (
	ReasonNew            Reason = "new"              // 一侧新增
	ReasonModified       Reason = "modified"         // 内容已修改
	ReasonDeletedOnLeft  Reason = "deleted-on-left"  // 左侧已删除（按状态推断）
	ReasonDeletedOnRight Reason = "deleted-on-right" // 右侧已删除
	ReasonNewerLeft      Reason = "newer-left"
	ReasonNewerRight     Reason = "newer-right"
	ReasonConflict       Reason = "conflict"
	ReasonExtraRight     Reason = "extra-on-right" // 目标端多出（非镜像，保留）
	ReasonExtraLeft      Reason = "extra-on-left"
	ReasonUnchanged      Reason = "unchanged"
	ReasonPolicy         Reason = "policy" // 策略指定侧为准
)

// Action 单个同步动作
type Action struct {
	Type   ActionType `json:"type"`
	Path   string     `json:"path"`
	Reason Reason     `json:"reason"`
	Size   int64      `json:"size,omitempty"`
	LeftM  int64      `json:"leftM,omitempty"`  // 左侧 mtime（展示用）
	RightM int64      `json:"rightM,omitempty"` // 右侧 mtime
}

// PlanStats 计划统计
type PlanStats struct {
	CopyL2R    int   `json:"copyL2R"`
	CopyR2L    int   `json:"copyR2L"`
	DelLeft    int   `json:"delLeft"`
	DelRight   int   `json:"delRight"`
	Conflict   int   `json:"conflict"`
	Skip       int   `json:"skip"`
	Unchanged  int   `json:"unchanged"`
	TotalBytes int64 `json:"totalBytes"`
}

// Plan 同步计划（干跑结果）
type Plan struct {
	Actions      []Action  `json:"actions"`
	Stats        PlanStats `json:"stats"`
	LeftMissing  bool      `json:"leftMissing"`
	RightMissing bool      `json:"rightMissing"`
	Truncated    bool      `json:"truncated"` // Actions 超过上限被截断
}

// MaxPlanActions 预览列表上限（统计数字始终完整）
const MaxPlanActions = 500

// Snapshot 扫描目录构建快照（应用过滤规则；目录不存在返回空快照）
func SnapshotDir(root string, m *filter.Matcher) (Snapshot, error) {
	snap := Snapshot{}
	root = filepath.Clean(root)
	_, statErr := os.Stat(root)
	if statErr != nil {
		if os.IsNotExist(statErr) {
			return snap, nil // 不存在 → 空快照，由调用方决定语义
		}
		return nil, statErr
	}
	err := filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if p == root {
			return nil
		}
		rel, rerr := filepath.Rel(root, p)
		if rerr != nil {
			return rerr
		}
		rel = filepath.ToSlash(rel)
		if !m.Allowed(rel, d.IsDir()) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			return nil // 仅同步文件；复制时自动 MkdirAll
		}
		info, ierr := d.Info()
		if ierr != nil {
			return ierr
		}
		snap[rel] = FileInfo{Path: rel, Size: info.Size(), ModTime: info.ModTime().UnixNano()}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return snap, nil
}

// mtimeEqual 增量判定：大小一致且修改时间差 ≤ 1s（容忍 FAT/网络盘粒度）
func mtimeEqual(a, b int64) bool {
	d := a - b
	if d < 0 {
		d = -d
	}
	return d <= int64(time.Second)
}

// Diff 生成同步计划。
// prevState 为上次同步完成时的两侧清单（双向模式用于区分「删除」与「新增」）。
func Diff(left, right Snapshot, prevState map[string]state.Entry, cfg config.TaskConfig) Plan {
	plan := Plan{}
	add := func(a Action) {
		switch a.Type {
		case ActCopyL2R:
			plan.Stats.CopyL2R++
			plan.Stats.TotalBytes += a.Size
		case ActCopyR2L:
			plan.Stats.CopyR2L++
			plan.Stats.TotalBytes += a.Size
		case ActDelLeft:
			plan.Stats.DelLeft++
		case ActDelRight:
			plan.Stats.DelRight++
		case ActConflict:
			plan.Stats.Conflict++
		case ActSkip:
			if a.Reason == ReasonUnchanged {
				plan.Stats.Unchanged++
				return // 未变化文件不进预览列表
			}
			plan.Stats.Skip++
		}
		if len(plan.Actions) < MaxPlanActions {
			plan.Actions = append(plan.Actions, a)
		} else {
			plan.Truncated = true
		}
	}

	// 双向：判断一侧删除是否应传播
	propagateDelete := func(existsSide, missingSide string, fi FileInfo, prev state.Entry) (Action, bool) {
		if !cfg.Mirror {
			return Action{}, false
		}
		// prev 中两侧都存在，且存在侧文件与上次一致 → 缺失侧是被删除的
		var same bool
		if existsSide == "L" {
			same = prev.HasL && prev.HasR && prev.SizeL == fi.Size && mtimeEqual(prev.MtimeL, fi.ModTime)
		} else {
			same = prev.HasL && prev.HasR && prev.SizeR == fi.Size && mtimeEqual(prev.MtimeR, fi.ModTime)
		}
		if !same {
			return Action{}, false
		}
		// missingSide 是用户删除文件的那一侧 → 传播删除到存在侧
		if missingSide == "L" {
			return Action{Type: ActDelRight, Path: fi.Path, Reason: ReasonDeletedOnLeft, Size: fi.Size}, true
		}
		return Action{Type: ActDelLeft, Path: fi.Path, Reason: ReasonDeletedOnRight, Size: fi.Size}, true
	}

	for _, path := range mergedPaths(left, right) {
		l, lok := left[path]
		r, rok := right[path]
		prev := prevState[path]

		switch {
		case lok && !rok:
			// 左侧独有
			if cfg.Mode == config.ModeTwoWay && prev.HasL && prev.HasR {
				if a, ok := propagateDelete("L", "R", l, prev); ok {
					add(a)
					continue
				}
				add(Action{Type: ActSkip, Path: path, Reason: ReasonDeletedOnRight, Size: l.Size})
				continue
			}
			if cfg.Mode == config.ModeOneWayBack {
				// 右→左 单向：left 独有 = 目标端多出
				if cfg.Mirror {
					add(Action{Type: ActDelLeft, Path: path, Reason: ReasonExtraLeft, Size: l.Size})
				} else {
					add(Action{Type: ActSkip, Path: path, Reason: ReasonExtraLeft, Size: l.Size})
				}
				continue
			}
			add(Action{Type: ActCopyL2R, Path: path, Reason: ReasonNew, Size: l.Size, LeftM: l.ModTime})

		case rok && !lok:
			// 右侧独有
			if cfg.Mode == config.ModeTwoWay && prev.HasL && prev.HasR {
				if a, ok := propagateDelete("R", "L", r, prev); ok {
					add(a)
					continue
				}
				add(Action{Type: ActSkip, Path: path, Reason: ReasonDeletedOnLeft, Size: r.Size})
				continue
			}
			if cfg.Mode == config.ModeOneWay {
				if cfg.Mirror {
					add(Action{Type: ActDelRight, Path: path, Reason: ReasonExtraRight, Size: r.Size})
				} else {
					add(Action{Type: ActSkip, Path: path, Reason: ReasonExtraRight, Size: r.Size})
				}
				continue
			}
			add(Action{Type: ActCopyR2L, Path: path, Reason: ReasonNew, Size: r.Size, RightM: r.ModTime})

		default:
			// 两侧都有
			if cfg.Incremental && l.Size == r.Size && mtimeEqual(l.ModTime, r.ModTime) {
				add(Action{Type: ActSkip, Path: path, Reason: ReasonUnchanged})
				continue
			}
			switch cfg.Mode {
			case config.ModeOneWay:
				add(Action{Type: ActCopyL2R, Path: path, Reason: ReasonModified, Size: l.Size, LeftM: l.ModTime, RightM: r.ModTime})
			case config.ModeOneWayBack:
				add(Action{Type: ActCopyR2L, Path: path, Reason: ReasonModified, Size: r.Size, LeftM: l.ModTime, RightM: r.ModTime})
			case config.ModeTwoWay:
				lm, rm := time.Unix(0, l.ModTime), time.Unix(0, r.ModTime)
				switch cfg.Conflict {
				case config.ConflictLeft:
					add(Action{Type: ActCopyL2R, Path: path, Reason: ReasonPolicy, Size: l.Size, LeftM: l.ModTime, RightM: r.ModTime})
				case config.ConflictRight:
					add(Action{Type: ActCopyR2L, Path: path, Reason: ReasonPolicy, Size: r.Size, LeftM: l.ModTime, RightM: r.ModTime})
				case config.ConflictSkip:
					add(Action{Type: ActConflict, Path: path, Reason: ReasonConflict, Size: l.Size, LeftM: l.ModTime, RightM: r.ModTime})
				default: // newer
					if !lm.Before(rm) {
						add(Action{Type: ActCopyL2R, Path: path, Reason: ReasonNewerLeft, Size: l.Size, LeftM: l.ModTime, RightM: r.ModTime})
					} else {
						add(Action{Type: ActCopyR2L, Path: path, Reason: ReasonNewerRight, Size: r.Size, LeftM: l.ModTime, RightM: r.ModTime})
					}
				}
			}
		}
	}
	return plan
}

// mergedPaths 返回两侧快照键的有序并集（保证计划顺序稳定）
func mergedPaths(a, b Snapshot) []string {
	seen := map[string]bool{}
	var out []string
	for k := range a {
		if !seen[k] {
			seen[k] = true
			out = append(out, k)
		}
	}
	for k := range b {
		if !seen[k] {
			seen[k] = true
			out = append(out, k)
		}
	}
	// 排序：路径段逐段比较，父目录排在子文件前
	sortPaths(out)
	return out
}

func sortPaths(ps []string) {
	// 简单字典序即可满足稳定性需求
	for i := 1; i < len(ps); i++ {
		for j := i; j > 0 && ps[j] < ps[j-1]; j-- {
			ps[j], ps[j-1] = ps[j-1], ps[j]
		}
	}
}

// Reporter 执行过程回调
type Reporter interface {
	Log(level, format string, args ...any)
	Progress(done, total int, current string)
}

// Result 执行结果
type Result struct {
	CopiedL2R  int       `json:"copiedL2R"`
	CopiedR2L  int       `json:"copiedR2L"`
	Deleted    int       `json:"deleted"`
	Skipped    int       `json:"skipped"`
	Conflicts  int       `json:"conflicts"`
	Failed     int       `json:"failed"`
	Bytes      int64     `json:"bytes"`
	Errors     []string  `json:"errors,omitempty"`
	StartedAt  time.Time `json:"startedAt"`
	FinishedAt time.Time `json:"finishedAt"`
}

// Execute 执行同步计划
func Execute(ctx context.Context, cfg config.TaskConfig, plan Plan, rep Reporter) (*Result, error) {
	res := &Result{StartedAt: time.Now()}
	total := plan.Stats.CopyL2R + plan.Stats.CopyR2L + plan.Stats.DelLeft + plan.Stats.DelRight
	done := 0
	lastEmit := time.Now()

	backup := func(sideRoot, rel string) {
		if !cfg.BackupDeleted {
			return
		}
		src := filepath.Join(sideRoot, filepath.FromSlash(rel))
		if _, err := os.Lstat(src); err != nil {
			return
		}
		ts := time.Now().Format("20060102-150405")
		dst := filepath.Join(sideRoot, TrashDirName, ts, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			res.Failed++
			res.Errors = append(res.Errors, fmt.Sprintf("备份目录创建失败 %s: %v", rel, err))
			logger.Log("失败 备份目录创建 %s: %v", rel, err)
			return
		}
		if err := os.Rename(src, dst); err != nil {
			// 跨盘 rename 失败则复制后删除
			if _, cerr := copyFile(src, dst, nil); cerr == nil {
				os.Remove(src)
				logger.Log("备份 %s → %s", rel, dst)
			} else {
				res.Failed++
				res.Errors = append(res.Errors, fmt.Sprintf("备份失败 %s: %v", rel, err))
				logger.Log("失败 备份 %s: %v", rel, err)
			}
		} else {
			logger.Log("备份 %s → %s", rel, dst)
		}
	}

	for _, a := range plan.Actions {
		select {
		case <-ctx.Done():
			res.FinishedAt = time.Now()
			return res, ctx.Err()
		default:
		}
		switch a.Type {
		case ActCopyL2R:
			src := filepath.Join(cfg.Left, filepath.FromSlash(a.Path))
			dst := filepath.Join(cfg.Right, filepath.FromSlash(a.Path))
			backup(cfg.Right, a.Path)
			n, err := copyPreserveMtime(src, dst)
			if err != nil {
				res.Failed++
				res.Errors = append(res.Errors, fmt.Sprintf("复制失败 %s: %v", a.Path, err))
				logger.Log("失败 复制 L→R %s: %v", a.Path, err)
			} else {
				res.CopiedL2R++
				res.Bytes += n
				logger.Log("复制 L→R %s (%s)", a.Path, formatBytes(n))
			}
		case ActCopyR2L:
			src := filepath.Join(cfg.Right, filepath.FromSlash(a.Path))
			dst := filepath.Join(cfg.Left, filepath.FromSlash(a.Path))
			backup(cfg.Left, a.Path)
			n, err := copyPreserveMtime(src, dst)
			if err != nil {
				res.Failed++
				res.Errors = append(res.Errors, fmt.Sprintf("复制失败 %s: %v", a.Path, err))
				logger.Log("失败 复制 R→L %s: %v", a.Path, err)
			} else {
				res.CopiedR2L++
				res.Bytes += n
				logger.Log("复制 R→L %s (%s)", a.Path, formatBytes(n))
			}
		case ActDelLeft:
			backup(cfg.Left, a.Path)
			if err := os.Remove(filepath.Join(cfg.Left, filepath.FromSlash(a.Path))); err != nil && !os.IsNotExist(err) {
				res.Failed++
				res.Errors = append(res.Errors, fmt.Sprintf("删除失败 %s: %v", a.Path, err))
				logger.Log("失败 删除左侧 %s: %v", a.Path, err)
			} else {
				res.Deleted++
				logger.Log("删除 左侧 %s", a.Path)
			}
		case ActDelRight:
			backup(cfg.Right, a.Path)
			if err := os.Remove(filepath.Join(cfg.Right, filepath.FromSlash(a.Path))); err != nil && !os.IsNotExist(err) {
				res.Failed++
				res.Errors = append(res.Errors, fmt.Sprintf("删除失败 %s: %v", a.Path, err))
				logger.Log("失败 删除右侧 %s: %v", a.Path, err)
			} else {
				res.Deleted++
				logger.Log("删除 右侧 %s", a.Path)
			}
		case ActConflict:
			res.Conflicts++
			rep.Log("warn", "冲突跳过: %s（两侧均有修改）", a.Path)
		default: // ActSkip
			res.Skipped++
		}
		done++
		now := time.Now()
		if now.Sub(lastEmit) >= 50*time.Millisecond || done == total {
			lastEmit = now
			rep.Progress(done, total, a.Path)
		}
	}
	res.FinishedAt = time.Now()
	return res, nil
}

// formatBytes 字节数的人类可读格式（用于文件日志）
func formatBytes(n int64) string {
	switch {
	case n >= 1<<30:
		return fmt.Sprintf("%.1f GiB", float64(n)/(1<<30))
	case n >= 1<<20:
		return fmt.Sprintf("%.1f MiB", float64(n)/(1<<20))
	case n >= 1<<10:
		return fmt.Sprintf("%.1f KiB", float64(n)/(1<<10))
	default:
		return fmt.Sprintf("%d B", n)
	}
}

// copyPreserveMtime 复制单个文件（创建父目录，执行时重新 stat 源文件保留最新 mtime）
func copyPreserveMtime(src, dst string) (int64, error) {
	info, err := os.Stat(src)
	if err != nil {
		return 0, err
	}
	mtime := info.ModTime()
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return 0, err
	}
	n, err := copyFile(src, dst, &mtime)
	return n, err
}

func copyFile(src, dst string, mtime *time.Time) (int64, error) {
	in, err := os.Open(src)
	if err != nil {
		return 0, err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return 0, err
	}
	n, err := io.Copy(out, in)
	cerr := out.Close()
	if err == nil {
		err = cerr
	}
	if err == nil && mtime != nil && !mtime.IsZero() {
		os.Chtimes(dst, *mtime, *mtime)
	}
	return n, err
}

// PostSnapshots 同步完成后的两侧快照（写入 state 供下次判断删除）
func PostSnapshots(cfg config.TaskConfig, m *filter.Matcher) (map[string]state.Entry, error) {
	left, err := SnapshotDir(cfg.Left, m)
	if err != nil {
		return nil, err
	}
	right, err := SnapshotDir(cfg.Right, m)
	if err != nil {
		return nil, err
	}
	entries := map[string]state.Entry{}
	for path, fi := range left {
		entries[path] = state.Entry{HasL: true, SizeL: fi.Size, MtimeL: fi.ModTime}
	}
	for path, fi := range right {
		e := entries[path]
		e.HasR = true
		e.SizeR = fi.Size
		e.MtimeR = fi.ModTime
		entries[path] = e
	}
	return entries, nil
}

// Total 需要执行的操作总数（不含未变化跳过项）
func (s PlanStats) Total() int {
	return s.CopyL2R + s.CopyR2L + s.DelLeft + s.DelRight + s.Conflict + s.Skip
}
