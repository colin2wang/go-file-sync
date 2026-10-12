// Package state 维护每个任务的运行状态与上次同步清单（用于双向模式区分「删除」与「新增」）。
package state

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Entry 单个文件在上次同步完成时的两侧快照
type Entry struct {
	HasL   bool  `json:"hasL"`
	SizeL  int64 `json:"sizeL,omitempty"`
	MtimeL int64 `json:"mtimeL,omitempty"` // unix nano
	HasR   bool  `json:"hasR"`
	SizeR  int64 `json:"sizeR,omitempty"`
	MtimeR int64 `json:"mtimeR,omitempty"`
}

// RunSummary 最近一次运行摘要
type RunSummary struct {
	Status    string    `json:"status"` // ok | error | cancelled
	Copied    int       `json:"copied"`
	Deleted   int       `json:"deleted"`
	Skipped   int       `json:"skipped"`
	Conflicts int       `json:"conflicts"`
	Failed    int       `json:"failed"`
	Bytes     int64     `json:"bytes"`
	Message   string    `json:"message,omitempty"`
	Finished  time.Time `json:"finished"`
}

// State 任务运行状态
type State struct {
	TaskID  string           `json:"taskId"`
	LastRun time.Time        `json:"lastRun"`
	NextRun time.Time        `json:"nextRun"`
	Last    *RunSummary      `json:"last,omitempty"`
	Entries map[string]Entry `json:"entries"`
}

// Store 状态目录（UserConfigDir/FileSync/state/）
type Store struct {
	Dir string
	mu  sync.Mutex
}

func NewStore(dir string) *Store { return &Store{Dir: dir} }

// DefaultDir 返回默认状态目录
func DefaultDir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "FileSync", "state"), nil
}

func (s *Store) taskPath(id string) string { return filepath.Join(s.Dir, id+".json") }

// Load 读取任务状态；不存在返回空状态
func (s *Store) Load(id string) *State {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.load(id)
}

func (s *Store) load(id string) *State {
	st := &State{TaskID: id, Entries: map[string]Entry{}}
	data, err := os.ReadFile(s.taskPath(id))
	if err != nil {
		return st
	}
	_ = json.Unmarshal(data, st)
	if st.Entries == nil {
		st.Entries = map[string]Entry{}
	}
	return st
}

// Save 原子保存状态
func (s *Store) Save(st *State) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := os.MkdirAll(s.Dir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	dst := s.taskPath(st.TaskID)
	tmp := dst + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, dst)
}

// Delete 删除任务状态（任务删除时调用）
func (s *Store) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	err := os.Remove(s.taskPath(id))
	if os.IsNotExist(err) {
		return nil
	}
	return err
}
