// Package config 负责 FileSync 任务配置的读写与校验。
// 一个同步任务 = 一个 YAML 文件；多个任务 = 多个 yml。
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"gopkg.in/yaml.v3"
)

// Mode 同步模式
type Mode string

const (
	ModeOneWay     Mode = "one-way"      // 左 → 右
	ModeOneWayBack Mode = "one-way-back" // 右 → 左
	ModeTwoWay     Mode = "two-way"      // 双向保持一致
)

func (m Mode) IsValid() bool {
	switch m {
	case ModeOneWay, ModeOneWayBack, ModeTwoWay:
		return true
	}
	return false
}

// Conflict 双向同步冲突策略
type Conflict string

const (
	ConflictNewer Conflict = "newer" // 较新者胜
	ConflictLeft  Conflict = "left"  // 以左侧为准
	ConflictRight Conflict = "right" // 以右侧为准
	ConflictSkip  Conflict = "skip"  // 跳过并报告
)

func (c Conflict) IsValid() bool {
	switch c {
	case ConflictNewer, ConflictLeft, ConflictRight, ConflictSkip:
		return true
	}
	return false
}

// Filters 文件过滤规则（glob：* 任意、? 单字符、** 跨目录；尾部 / 表示目录）
type Filters struct {
	Exclude       []string `yaml:"exclude" json:"exclude"`
	Include       []string `yaml:"include" json:"include"`
	CaseSensitive bool     `yaml:"case_sensitive" json:"caseSensitive"`
}

// Schedule 定时同步
type Schedule struct {
	Enabled         bool   `yaml:"enabled" json:"enabled"`
	Type            string `yaml:"type" json:"type"` // interval | daily
	IntervalMinutes int    `yaml:"interval_minutes" json:"intervalMinutes"`
	DailyTime       string `yaml:"daily_time" json:"dailyTime"` // HH:MM
}

// TaskConfig 单个同步任务的完整配置
type TaskConfig struct {
	ID            string    `yaml:"id" json:"id"`
	Name          string    `yaml:"name" json:"name"`
	Enabled       bool      `yaml:"enabled" json:"enabled"`
	Left          string    `yaml:"left" json:"left"`
	Right         string    `yaml:"right" json:"right"`
	Mode          Mode      `yaml:"mode" json:"mode"`
	Incremental   bool      `yaml:"incremental" json:"incremental"`
	Mirror        bool      `yaml:"mirror" json:"mirror"`
	Conflict      Conflict  `yaml:"conflict" json:"conflict"`
	BackupDeleted bool      `yaml:"backup_deleted" json:"backupDeleted"`
	Filters       Filters   `yaml:"filters" json:"filters"`
	Schedule      Schedule  `yaml:"schedule" json:"schedule"`
	Created       time.Time `yaml:"created" json:"created"`
	Updated       time.Time `yaml:"updated" json:"updated"`
}

// Default 返回一份合理的默认配置模板
func Default() TaskConfig {
	return TaskConfig{
		Name:          "新建同步任务",
		Enabled:       true,
		Mode:          ModeOneWay,
		Incremental:   true,
		Mirror:        false,
		Conflict:      ConflictNewer,
		BackupDeleted: true,
		Filters: Filters{
			Exclude:       []string{"*.tmp", "~$*", "Thumbs.db", ".DS_Store", "node_modules/"},
			Include:       []string{},
			CaseSensitive: false,
		},
		Schedule: Schedule{
			Enabled:         false,
			Type:            "interval",
			IntervalMinutes: 60,
			DailyTime:       "02:00",
		},
	}
}

// Validate 校验配置合法性

// isAbsPath 绝对路径判定（兼容 Windows 盘符 C:\ 与 POSIX /）
func isAbsPath(p string) bool {
	if filepath.IsAbs(p) {
		return true
	}
	if len(p) >= 3 && p[1] == ':' && (p[2] == '\\' || p[2] == '/') {
		c := p[0] | 0x20 // 转小写
		return c >= 'a' && c <= 'z'
	}
	return false
}

func (c *TaskConfig) Validate() error {
	if strings.TrimSpace(c.Name) == "" {
		return fmt.Errorf("任务名称不能为空 / task name is required")
	}
	if strings.TrimSpace(c.Left) == "" {
		return fmt.Errorf("左侧文件夹不能为空 / left folder is required")
	}
	if strings.TrimSpace(c.Right) == "" {
		return fmt.Errorf("右侧文件夹不能为空 / right folder is required")
	}
	if filepath.Clean(c.Left) == filepath.Clean(c.Right) {
		return fmt.Errorf("左右文件夹不能相同 / left and right folders must differ")
	}
	if !isAbsPath(c.Left) {
		return fmt.Errorf("左侧文件夹必须是绝对路径: %s", c.Left)
	}
	if !isAbsPath(c.Right) {
		return fmt.Errorf("右侧文件夹必须是绝对路径: %s", c.Right)
	}
	if !c.Mode.IsValid() {
		return fmt.Errorf("无效的同步模式: %s", c.Mode)
	}
	if c.Mode == ModeTwoWay && !c.Conflict.IsValid() {
		return fmt.Errorf("无效的冲突策略: %s", c.Conflict)
	}
	if c.Schedule.Enabled {
		switch c.Schedule.Type {
		case "interval":
			if c.Schedule.IntervalMinutes < 1 {
				return fmt.Errorf("定时间隔必须 ≥ 1 分钟")
			}
		case "daily":
			if _, err := time.Parse("15:04", c.Schedule.DailyTime); err != nil {
				return fmt.Errorf("每日时间格式必须为 HH:MM: %s", c.Schedule.DailyTime)
			}
		default:
			return fmt.Errorf("无效的定时类型: %s（可选 interval / daily）", c.Schedule.Type)
		}
	}
	return nil
}

// Store 管理任务目录（UserConfigDir/FileSync/tasks/）
type Store struct {
	Dir string
}

// DefaultDir 返回默认配置目录
func DefaultDir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "FileSync", "tasks"), nil
}

func NewStore(dir string) *Store { return &Store{Dir: dir} }

// EnsureDir 确保目录存在
func (s *Store) EnsureDir() error { return os.MkdirAll(s.Dir, 0o755) }

// List 列出全部任务（按名称排序）
func (s *Store) List() ([]TaskConfig, error) {
	entries, err := os.ReadDir(s.Dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var out []TaskConfig
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || (!strings.HasSuffix(name, ".yml") && !strings.HasSuffix(name, ".yaml")) {
			continue
		}
		var c TaskConfig
		data, err := os.ReadFile(filepath.Join(s.Dir, name))
		if err != nil {
			continue
		}
		if err := yaml.Unmarshal(data, &c); err != nil {
			continue // 跳过坏文件，不让单个损坏配置拖垮列表
		}
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// Load 按 ID 加载任务
func (s *Store) Load(id string) (*TaskConfig, error) {
	if id == "" || strings.ContainsAny(id, `/\`) {
		return nil, fmt.Errorf("非法任务 ID")
	}
	data, err := os.ReadFile(s.PathOf(id))
	if err != nil {
		return nil, err
	}
	var c TaskConfig
	if err := yaml.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("解析任务 %s 失败: %w", id, err)
	}
	return &c, nil
}

// Save 新建或更新任务（原子写入）
func (s *Store) Save(c *TaskConfig) error {
	if c.ID == "" {
		c.ID = uuid.NewString()
	}
	if c.Created.IsZero() {
		c.Created = time.Now()
	}
	c.Updated = time.Now()
	if err := c.Validate(); err != nil {
		return err
	}
	if err := s.EnsureDir(); err != nil {
		return err
	}
	data, err := yaml.Marshal(c)
	if err != nil {
		return err
	}
	dst := s.PathOf(c.ID)
	tmp := dst + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, dst)
}

// Delete 删除任务配置文件
func (s *Store) Delete(id string) error {
	if id == "" || strings.ContainsAny(id, `/\`) {
		return fmt.Errorf("非法任务 ID")
	}
	err := os.Remove(s.PathOf(id))
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

// PathOf 任务配置文件路径
func (s *Store) PathOf(id string) string {
	return filepath.Join(s.Dir, id+".yml")
}
