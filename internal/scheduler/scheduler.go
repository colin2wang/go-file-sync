// Package scheduler 实现任务定时调度：interval（每 N 分钟）与 daily（每天固定时刻）。
package scheduler

import (
	"sync"
	"time"

	"filesync/internal/config"
	"filesync/internal/state"
)

// Runner 到期回调（app 层注入：执行同步）
type Runner func(taskID string, manual bool)

// Scheduler 周期扫描任务并触发到期的同步
type Scheduler struct {
	store  *config.Store
	states *state.Store
	run    Runner
	tick   time.Duration

	mu      sync.Mutex
	running map[string]bool
	stop    chan struct{}
}

// New 构建调度器；tick 为扫描周期（默认 30s）
func New(store *config.Store, states *state.Store, run Runner, tick time.Duration) *Scheduler {
	if tick <= 0 {
		tick = 30 * time.Second
	}
	return &Scheduler{
		store: store, states: states, run: run, tick: tick,
		running: map[string]bool{},
		stop:    make(chan struct{}),
	}
}

// Start 启动后台循环
func (s *Scheduler) Start() {
	go func() {
		t := time.NewTicker(s.tick)
		defer t.Stop()
		s.Check(time.Now()) // 启动即检查一次（补跑错过的）
		for {
			select {
			case <-s.stop:
				return
			case now := <-t.C:
				s.Check(now)
			}
		}
	}()
}

// Stop 停止调度
func (s *Scheduler) Stop() { close(s.stop) }

// MarkRunning / IsRunning 运行状态跟踪（防并发重入）
func (s *Scheduler) MarkRunning(id string, v bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if v {
		s.running[id] = true
	} else {
		delete(s.running, id)
	}
}

func (s *Scheduler) IsRunning(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.running[id]
}

// Check 扫描全部任务，触发到期者；返回触发的任务 ID
func (s *Scheduler) Check(now time.Time) []string {
	tasks, err := s.store.List()
	if err != nil {
		return nil
	}
	var fired []string
	for _, cfg := range tasks {
		if !cfg.Enabled || !cfg.Schedule.Enabled {
			continue
		}
		if s.IsRunning(cfg.ID) {
			continue
		}
		st := s.states.Load(cfg.ID)
		next := st.NextRun
		if next.IsZero() {
			next = ComputeNext(cfg, st, now)
		}
		if !next.After(now) {
			// 立即滚动下一次时间，防重复触发
			st.NextRun = ComputeNext(cfg, st, now)
			_ = s.states.Save(st)
			fired = append(fired, cfg.ID)
			id := cfg.ID
			go func() {
				s.MarkRunning(id, true)
				defer s.MarkRunning(id, false)
				s.run(id, false)
			}()
		}
	}
	return fired
}

// ComputeNext 计算下一次运行时间
func ComputeNext(cfg config.TaskConfig, st *state.State, now time.Time) time.Time {
	sc := cfg.Schedule
	if sc.Type == "daily" {
		h, m := parseHM(sc.DailyTime)
		next := time.Date(now.Year(), now.Month(), now.Day(), h, m, 0, 0, now.Location())
		if !next.After(now) {
			next = next.AddDate(0, 0, 1)
		}
		// 若从未运行过，首次可当天/次日按时刻执行（next 已保证未来）
		return next
	}
	// interval
	mins := sc.IntervalMinutes
	if mins < 1 {
		mins = 60
	}
	if st.LastRun.IsZero() {
		return now.Add(time.Duration(mins) * time.Minute)
	}
	next := st.LastRun.Add(time.Duration(mins) * time.Minute)
	if !next.After(now) {
		next = now.Add(time.Duration(mins) * time.Minute)
	}
	return next
}

func parseHM(s string) (int, int) {
	t, err := time.Parse("15:04", s)
	if err != nil {
		return 2, 0 // 配置非法时兜底（Validate 已拦截，此处防御）
	}
	return t.Hour(), t.Minute()
}
