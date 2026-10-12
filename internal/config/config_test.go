package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func sample() TaskConfig {
	return TaskConfig{
		ID: "test-1", Name: "文档备份", Enabled: true,
		Left: "/home/u/docs", Right: "/mnt/bak/docs",
		Mode: ModeTwoWay, Incremental: true, Mirror: false,
		Conflict: ConflictNewer, BackupDeleted: true,
		Filters:  Filters{Exclude: []string{"*.tmp", "Thumbs.db", "node_modules/"}, Include: []string{"*.docx"}, CaseSensitive: true},
		Schedule: Schedule{Enabled: true, Type: "interval", IntervalMinutes: 60, DailyTime: "02:00"},
		Created:  time.Now(),
	}
}

func TestRenderParseRoundTrip(t *testing.T) {
	c := sample()
	text := RenderYAML(c)
	got, err := ParseYAML(text)
	if err != nil {
		t.Fatalf("解析渲染结果失败: %v\n%s", err, text)
	}
	if got.Name != c.Name || got.Left != c.Left || got.Right != c.Right {
		t.Errorf("基础字段不一致: %+v", got)
	}
	if got.Mode != ModeTwoWay || got.Conflict != ConflictNewer {
		t.Errorf("枚举字段不一致: mode=%s conflict=%s", got.Mode, got.Conflict)
	}
	if !got.Incremental || !got.BackupDeleted || got.Mirror {
		t.Errorf("布尔字段不一致: %+v", got)
	}
	if len(got.Filters.Exclude) != 3 || got.Filters.Exclude[2] != "node_modules/" {
		t.Errorf("exclude 不一致: %v", got.Filters.Exclude)
	}
	if !got.Filters.CaseSensitive || got.Filters.Include[0] != "*.docx" {
		t.Errorf("include/case 不一致: %+v", got.Filters)
	}
	if !got.Schedule.Enabled || got.Schedule.IntervalMinutes != 60 || got.Schedule.DailyTime != "02:00" {
		t.Errorf("schedule 不一致: %+v", got.Schedule)
	}
	if !strings.Contains(text, "# mode  : one-way = 左→右") {
		t.Error("渲染应包含中文注释")
	}
}

func TestRenderQuoting(t *testing.T) {
	c := sample()
	c.Name = `包含: 冒号 和 #井号`
	c.Left = ""
	text := RenderYAML(c)
	got, err := ParseYAML(text)
	if err != nil {
		t.Fatalf("特殊字符渲染应可解析: %v", err)
	}
	if got.Name != c.Name {
		t.Errorf("含冒号名称往返失败: %q", got.Name)
	}
}

func TestParseStrict(t *testing.T) {
	if _, err := ParseYAML("name: x\nno_such_field: 1\n"); err == nil {
		t.Error("未知字段应报错")
	}
	if _, err := ParseYAML("name: [1,2]"); err == nil {
		t.Error("类型错误应报错")
	}
}

func TestValidate(t *testing.T) {
	c := sample()
	if err := c.Validate(); err != nil {
		t.Fatalf("合法配置不应报错: %v", err)
	}
	c2 := c
	c2.Mode = "bad"
	if err := c2.Validate(); err == nil {
		t.Error("非法 mode 应报错")
	}
	c3 := c
	c3.Schedule.Type = "daily"
	c3.Schedule.DailyTime = "25:00"
	if err := c3.Validate(); err == nil {
		t.Error("非法 daily_time 应报错")
	}
	c4 := c
	c4.Left = "/same"
	c4.Right = "/same"
	if err := c4.Validate(); err == nil {
		t.Error("两侧路径相同应报错")
	}
	c5 := c
	c5.Schedule.IntervalMinutes = 0
	if err := c5.Validate(); err == nil {
		t.Error("间隔 0 应报错")
	}
}

func TestStore(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "tasks")
	s := NewStore(dir)
	c := sample()
	c.ID = ""
	if err := s.Save(&c); err != nil {
		t.Fatalf("保存失败: %v", err)
	}
	if c.ID == "" {
		t.Fatal("保存后应生成 ID")
	}
	if _, err := os.Stat(s.PathOf(c.ID)); err != nil {
		t.Fatalf("yml 文件应存在: %v", err)
	}
	list, err := s.List()
	if err != nil || len(list) != 1 {
		t.Fatalf("List 应返回 1 个任务: %v %d", err, len(list))
	}
	if list[0].Name != "文档备份" {
		t.Errorf("读取内容不一致: %s", list[0].Name)
	}
	if err := s.Delete(c.ID); err != nil {
		t.Fatalf("删除失败: %v", err)
	}
	if list, _ := s.List(); len(list) != 0 {
		t.Error("删除后列表应为空")
	}
}

func TestDefaultTask(t *testing.T) {
	c := Default()
	if c.Name == "" || !c.Mode.IsValid() || !c.Conflict.IsValid() {
		t.Errorf("默认模板枚举值应合法: %+v", c)
	}
	if c.Schedule.Type != "interval" || c.Schedule.IntervalMinutes < 1 {
		t.Errorf("默认调度应合法: %+v", c.Schedule)
	}
	// Left/Right 留空待用户填写，此处补上占位路径应可过校验
	c.Left, c.Right = "/tmp/l", "/tmp/r"
	if err := c.Validate(); err != nil {
		t.Errorf("补齐路径后应通过校验: %v", err)
	}
}
