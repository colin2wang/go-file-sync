package scheduler

import (
	"testing"
	"time"

	"filesync/internal/config"
	"filesync/internal/state"
)

func TestComputeNextInterval(t *testing.T) {
	now := time.Date(2026, 10, 10, 22, 0, 0, 0, time.Local)
	cfg := config.TaskConfig{Schedule: config.Schedule{Type: "interval", IntervalMinutes: 60}}
	// 首次：now + 60min
	st := &state.State{}
	next := ComputeNext(cfg, st, now)
	if diff := next.Sub(now); diff != 60*time.Minute {
		t.Errorf("首次应 now+60m, got %v", diff)
	}
	// 已运行：lastRun + 60m（若在未来）
	st.LastRun = now.Add(-10 * time.Minute)
	next = ComputeNext(cfg, st, now)
	if diff := next.Sub(now); diff != 50*time.Minute {
		t.Errorf("应为 lastRun+60m, got %v", diff)
	}
	// lastRun 久远 → now + 60m
	st.LastRun = now.Add(-5 * time.Hour)
	next = ComputeNext(cfg, st, now)
	if diff := next.Sub(now); diff != 60*time.Minute {
		t.Errorf("过期应 now+60m, got %v", diff)
	}
}

func TestComputeNextDaily(t *testing.T) {
	now := time.Date(2026, 10, 10, 22, 0, 0, 0, time.Local)
	cfg := config.TaskConfig{Schedule: config.Schedule{Type: "daily", DailyTime: "02:00"}}
	next := ComputeNext(cfg, &state.State{}, now)
	// 22:00 之后 → 明天 02:00
	want := time.Date(2026, 10, 11, 2, 0, 0, 0, time.Local)
	if !next.Equal(want) {
		t.Errorf("应次日 02:00, got %v", next)
	}
	// 01:00 → 当天 02:00
	now2 := time.Date(2026, 10, 10, 1, 0, 0, 0, time.Local)
	next2 := ComputeNext(cfg, &state.State{}, now2)
	want2 := time.Date(2026, 10, 10, 2, 0, 0, 0, time.Local)
	if !next2.Equal(want2) {
		t.Errorf("应当天 02:00, got %v", next2)
	}
}

func TestCheckFiresDueTask(t *testing.T) {
	dir := t.TempDir()
	store := config.NewStore(dir)
	states := state.NewStore(dir + "/state")
	fired := []string{}
	s := New(store, states, func(id string, manual bool) {
		fired = append(fired, id)
	}, time.Hour)

	cfg := config.TaskConfig{
		Name: "t", Left: "/a", Right: "/b", Enabled: true, Mode: config.ModeOneWay,
		Schedule: config.Schedule{Enabled: true, Type: "interval", IntervalMinutes: 1},
	}
	if err := store.Save(&cfg); err != nil {
		t.Fatal(err)
	}
	// 强制 NextRun 为过去
	st := states.Load(cfg.ID)
	st.NextRun = time.Now().Add(-time.Minute)
	states.Save(st)

	got := s.Check(time.Now())
	if len(got) != 1 || got[0] != cfg.ID {
		t.Fatalf("到期任务应被触发, got %v", got)
	}
	// 触发后 NextRun 应已滚动到未来
	st2 := states.Load(cfg.ID)
	if !st2.NextRun.After(time.Now()) {
		t.Error("触发后 NextRun 应在未来，防重复触发")
	}
	time.Sleep(50 * time.Millisecond) // 等 goroutine 跑 run()
	if len(fired) != 1 {
		t.Errorf("回调应执行一次, got %d", len(fired))
	}
}

func TestCheckSkipsDisabled(t *testing.T) {
	dir := t.TempDir()
	store := config.NewStore(dir)
	states := state.NewStore(dir + "/state")
	s := New(store, states, func(string, bool) {}, time.Hour)

	cfg := config.TaskConfig{
		Name: "t", Left: "/a", Right: "/b", Enabled: true, Mode: config.ModeOneWay,
		Schedule: config.Schedule{Enabled: false, Type: "interval", IntervalMinutes: 1},
	}
	store.Save(&cfg)
	st := states.Load(cfg.ID)
	st.NextRun = time.Now().Add(-time.Minute)
	states.Save(st)

	if got := s.Check(time.Now()); len(got) != 0 {
		t.Errorf("未启用调度不应触发, got %v", got)
	}
}
