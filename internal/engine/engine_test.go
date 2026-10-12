package engine

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"filesync/internal/config"
	"filesync/internal/filter"
)

type nopReporter struct{}

func (nopReporter) Log(string, string, ...any) {}
func (nopReporter) Progress(int, int, string)  {}

func write(t *testing.T, path, content string, mtime time.Time) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	if !mtime.IsZero() {
		os.Chtimes(path, mtime, mtime)
	}
}

func read(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func cfgOf(mode config.Mode, left, right string, mirror, incremental bool, conflict config.Conflict) config.TaskConfig {
	return config.TaskConfig{
		Name: "t", Left: left, Right: right, Mode: mode,
		Incremental: incremental, Mirror: mirror, Conflict: conflict,
	}
}

var base = time.Date(2026, 10, 10, 12, 0, 0, 0, time.Local)

func TestOneWayNewAndModified(t *testing.T) {
	dir := t.TempDir()
	left := filepath.Join(dir, "L")
	right := filepath.Join(dir, "R")
	write(t, filepath.Join(left, "a.txt"), "hello", base)
	write(t, filepath.Join(left, "sub/b.txt"), "world", base)

	cfg := cfgOf(config.ModeOneWay, left, right, false, true, config.ConflictNewer)
	m := filter.NewMust(nil, nil, false)
	ls, _ := SnapshotDir(left, m)
	rs, _ := SnapshotDir(right, m)
	plan := Diff(ls, rs, nil, cfg)
	if plan.Stats.CopyL2R != 2 {
		t.Fatalf("期望复制2个新文件，got %+v", plan.Stats)
	}
	if _, err := Execute(context.Background(), cfg, plan, nopReporter{}); err != nil {
		t.Fatal(err)
	}
	if read(t, filepath.Join(right, "sub/b.txt")) != "world" {
		t.Fatal("子目录文件未复制")
	}

	// 未变化 → 全部跳过
	ls, _ = SnapshotDir(left, m)
	rs, _ = SnapshotDir(right, m)
	plan = Diff(ls, rs, nil, cfg)
	if plan.Stats.CopyL2R != 0 || plan.Stats.Unchanged != 2 {
		t.Fatalf("增量应跳过未变化文件，got %+v", plan.Stats)
	}

	// 修改左侧 → 重新复制
	write(t, filepath.Join(left, "a.txt"), "hello v2", base.Add(time.Hour))
	ls, _ = SnapshotDir(left, m)
	rs, _ = SnapshotDir(right, m)
	plan = Diff(ls, rs, nil, cfg)
	if plan.Stats.CopyL2R != 1 {
		t.Fatalf("修改应触发复制，got %+v", plan.Stats)
	}
	Execute(context.Background(), cfg, plan, nopReporter{})
	if read(t, filepath.Join(right, "a.txt")) != "hello v2" {
		t.Fatal("修改未同步")
	}
}

func TestOneWayMirrorDelete(t *testing.T) {
	dir := t.TempDir()
	left := filepath.Join(dir, "L")
	right := filepath.Join(dir, "R")
	write(t, filepath.Join(left, "keep.txt"), "k", base)
	write(t, filepath.Join(left, "gone.txt"), "g", base)

	cfg := cfgOf(config.ModeOneWay, left, right, true, true, config.ConflictNewer) // mirror
	m := filter.NewMust(nil, nil, false)
	ls, _ := SnapshotDir(left, m)
	plan := Diff(ls, map[string]FileInfo{}, nil, cfg)
	Execute(context.Background(), cfg, plan, nopReporter{})

	// 删除源端文件，非镜像 → 保留目标端；镜像 → 删除
	os.Remove(filepath.Join(left, "gone.txt"))

	cfgNoMirror := cfgOf(config.ModeOneWay, left, right, false, true, config.ConflictNewer)
	ls, _ = SnapshotDir(left, m)
	rs, _ := SnapshotDir(right, m)
	plan = Diff(ls, rs, nil, cfgNoMirror)
	if plan.Stats.Skip == 0 || plan.Stats.DelRight != 0 {
		t.Fatalf("非镜像应保留多出文件，got %+v", plan.Stats)
	}

	ls, _ = SnapshotDir(left, m)
	plan = Diff(ls, rs, nil, cfg)
	if plan.Stats.DelRight != 1 {
		t.Fatalf("镜像应删除目标端多余文件，got %+v", plan.Stats)
	}
	Execute(context.Background(), cfg, plan, nopReporter{})
	if _, err := os.Stat(filepath.Join(right, "gone.txt")); !os.IsNotExist(err) {
		t.Fatal("镜像删除未生效")
	}
	if _, err := os.Stat(filepath.Join(right, "keep.txt")); err != nil {
		t.Fatal("不应误删保留文件")
	}
	// 备份目录应有备份
	if _, err := os.Stat(filepath.Join(right, TrashDirName)); err != nil {
		t.Log("note: backup_deleted 默认 false 时无备份目录")
	}
}

func TestTwoWayNewAndConflict(t *testing.T) {
	dir := t.TempDir()
	left := filepath.Join(dir, "L")
	right := filepath.Join(dir, "R")
	write(t, filepath.Join(left, "onlyL.txt"), "L", base)
	write(t, filepath.Join(right, "onlyR.txt"), "R", base)
	write(t, filepath.Join(left, "both.txt"), "from-left", base.Add(2*time.Hour))    // 左更新
	write(t, filepath.Join(right, "older.txt"), "from-right", base.Add(3*time.Hour)) // 右更新

	cfg := cfgOf(config.ModeTwoWay, left, right, false, true, config.ConflictNewer)
	m := filter.NewMust(nil, nil, false)
	ls, _ := SnapshotDir(left, m)
	rs, _ := SnapshotDir(right, m)
	plan := Diff(ls, rs, nil, cfg)
	if plan.Stats.CopyL2R != 2 || plan.Stats.CopyR2L != 2 {
		t.Fatalf("双向应各复制2个，got %+v", plan.Stats)
	}
	Execute(context.Background(), cfg, plan, nopReporter{})

	// 同步后两侧一致
	if read(t, filepath.Join(right, "onlyL.txt")) != "L" || read(t, filepath.Join(left, "onlyR.txt")) != "R" {
		t.Fatal("双向新增未互相传播")
	}
	if read(t, filepath.Join(right, "both.txt")) != "from-left" {
		t.Fatal("newer 策略应取左侧较新")
	}
	if read(t, filepath.Join(left, "older.txt")) != "from-right" {
		t.Fatal("newer 策略应取右侧较新")
	}
}

func TestTwoWayConflictSkip(t *testing.T) {
	dir := t.TempDir()
	left := filepath.Join(dir, "L")
	right := filepath.Join(dir, "R")
	write(t, filepath.Join(left, "c.txt"), "left-ver", base)
	write(t, filepath.Join(right, "c.txt"), "right-ver", base.Add(time.Hour))

	cfg := cfgOf(config.ModeTwoWay, left, right, false, true, config.ConflictSkip)
	m := filter.NewMust(nil, nil, false)
	ls, _ := SnapshotDir(left, m)
	rs, _ := SnapshotDir(right, m)
	plan := Diff(ls, rs, nil, cfg)
	if plan.Stats.Conflict != 1 {
		t.Fatalf("skip 策略应报冲突，got %+v", plan.Stats)
	}
	res, _ := Execute(context.Background(), cfg, plan, nopReporter{})
	if res.Conflicts != 1 || res.CopiedL2R+res.CopiedR2L != 0 {
		t.Fatal("冲突不应被执行")
	}
	if read(t, filepath.Join(left, "c.txt")) != "left-ver" {
		t.Fatal("冲突时文件不应被改动")
	}
}

func TestTwoWayDeletionPropagation(t *testing.T) {
	dir := t.TempDir()
	left := filepath.Join(dir, "L")
	right := filepath.Join(dir, "R")
	write(t, filepath.Join(left, "a.txt"), "A", base)
	write(t, filepath.Join(right, "b.txt"), "B", base)

	cfg := cfgOf(config.ModeTwoWay, left, right, true, true, config.ConflictNewer) // mirror=true
	m := filter.NewMust(nil, nil, false)
	ls, _ := SnapshotDir(left, m)
	rs, _ := SnapshotDir(right, m)
	plan := Diff(ls, rs, nil, cfg)
	Execute(context.Background(), cfg, plan, nopReporter{})

	// 两侧都有 a.txt b.txt；构建上次状态
	post, err := PostSnapshots(cfg, m)
	if err != nil {
		t.Fatal(err)
	}

	// 左侧删除 a.txt，右侧未动 → mirror 应传播删除到右侧
	os.Remove(filepath.Join(left, "a.txt"))
	ls, _ = SnapshotDir(left, m)
	rs, _ = SnapshotDir(right, m)
	plan = Diff(ls, rs, post, cfg)
	if plan.Stats.DelRight != 1 {
		t.Fatalf("应传播左侧删除到右侧，got %+v", plan.Stats)
	}
	Execute(context.Background(), cfg, plan, nopReporter{})
	if _, err := os.Stat(filepath.Join(right, "a.txt")); !os.IsNotExist(err) {
		t.Fatal("删除传播未生效")
	}

	// 右侧新增 c.txt（左侧从未有过）→ 应复制到左侧而非删除
	write(t, filepath.Join(right, "c.txt"), "C", base.Add(time.Hour))
	ls, _ = SnapshotDir(left, m)
	rs, _ = SnapshotDir(right, m)
	plan = Diff(ls, rs, post, cfg)
	if plan.Stats.CopyR2L != 1 {
		t.Fatalf("另一侧删除场景不应误伤新增，got %+v", plan.Stats)
	}
}

func TestNoMirrorDeletionKeepsBoth(t *testing.T) {
	dir := t.TempDir()
	left := filepath.Join(dir, "L")
	right := filepath.Join(dir, "R")
	write(t, filepath.Join(left, "a.txt"), "A", base)

	cfg := cfgOf(config.ModeTwoWay, left, right, false, true, config.ConflictNewer) // mirror=false
	m := filter.NewMust(nil, nil, false)
	ls, _ := SnapshotDir(left, m)
	rs, _ := SnapshotDir(right, m)
	plan := Diff(ls, rs, nil, cfg)
	Execute(context.Background(), cfg, plan, nopReporter{})
	post, _ := PostSnapshots(cfg, m)

	os.Remove(filepath.Join(left, "a.txt"))
	ls, _ = SnapshotDir(left, m)
	rs, _ = SnapshotDir(right, m)
	plan = Diff(ls, rs, post, cfg)
	if plan.Stats.DelRight != 0 || plan.Stats.Skip == 0 {
		t.Fatalf("非镜像删除应跳过并提示，got %+v", plan.Stats)
	}
}

func TestFiltersApplied(t *testing.T) {
	dir := t.TempDir()
	left := filepath.Join(dir, "L")
	right := filepath.Join(dir, "R")
	write(t, filepath.Join(left, "good.txt"), "g", base)
	write(t, filepath.Join(left, "skip.tmp"), "s", base)
	write(t, filepath.Join(left, "node_modules/x.js"), "x", base)

	cfg := cfgOf(config.ModeOneWay, left, right, false, true, config.ConflictNewer)
	cfg.Filters.Exclude = []string{"*.tmp", "node_modules/"}
	m := filter.NewMust(cfg.Filters.Exclude, nil, false)
	ls, _ := SnapshotDir(left, m)
	plan := Diff(ls, map[string]FileInfo{}, nil, cfg)
	if plan.Stats.CopyL2R != 1 {
		t.Fatalf("过滤后应只剩 1 个文件，got %+v", plan.Stats)
	}
}

func TestExecuteBackupAndPreserveMtime(t *testing.T) {
	dir := t.TempDir()
	left := filepath.Join(dir, "L")
	right := filepath.Join(dir, "R")
	write(t, filepath.Join(left, "a.txt"), "v1", base)
	cfg := cfgOf(config.ModeOneWay, left, right, false, true, config.ConflictNewer)
	cfg.BackupDeleted = true
	m := filter.NewMust(nil, nil, false)
	ls, _ := SnapshotDir(left, m)
	plan := Diff(ls, map[string]FileInfo{}, nil, cfg)
	Execute(context.Background(), cfg, plan, nopReporter{})

	// 覆盖前备份
	write(t, filepath.Join(left, "a.txt"), "v2-longer", base.Add(time.Hour))
	ls, _ = SnapshotDir(left, m)
	rs, _ := SnapshotDir(right, m)
	plan = Diff(ls, rs, nil, cfg)
	Execute(context.Background(), cfg, plan, nopReporter{})
	if read(t, filepath.Join(right, "a.txt")) != "v2-longer" {
		t.Fatal("覆盖失败")
	}
	entries, _ := os.ReadDir(filepath.Join(right, TrashDirName))
	if len(entries) == 0 {
		t.Fatal("覆盖前应备份原文件到 .filesync-trash")
	}

	// mtime 保留（增量判定依赖）
	dstInfo, _ := os.Stat(filepath.Join(right, "a.txt"))
	srcInfo, _ := os.Stat(filepath.Join(left, "a.txt"))
	if dstInfo.ModTime().Sub(srcInfo.ModTime()).Abs() > time.Second {
		t.Fatalf("mtime 未保留: dst=%v src=%v", dstInfo.ModTime(), srcInfo.ModTime())
	}
}

func TestMissingSideTreatedAsEmpty(t *testing.T) {
	dir := t.TempDir()
	left := filepath.Join(dir, "L")
	write(t, filepath.Join(left, "a.txt"), "A", base)
	right := filepath.Join(dir, "R_no_such")
	cfg := cfgOf(config.ModeOneWay, left, right, false, true, config.ConflictNewer)
	m := filter.NewMust(nil, nil, false)
	ls, _ := SnapshotDir(left, m)
	rs, _ := SnapshotDir(right, m) // 不存在 → 空快照
	plan := Diff(ls, rs, nil, cfg)
	if plan.Stats.CopyL2R != 1 {
		t.Fatalf("目标缺失应视为空，got %+v", plan.Stats)
	}
	Execute(context.Background(), cfg, plan, nopReporter{})
	if read(t, filepath.Join(right, "a.txt")) != "A" {
		t.Fatal("目标不存在时应自动创建")
	}
}
