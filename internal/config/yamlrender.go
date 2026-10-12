package config

import (
	"bytes"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// RenderYAML 生成带中文注释的「所见即所得」YAML 文本。
// 这是前端向导右栏预览的唯一真源，也是落盘配置的排版基准。
func RenderYAML(c TaskConfig) string {
	var b bytes.Buffer
	w := func(s string) { b.WriteString(s + "\n") }

	w("# ===================== FileSync 任务配置 =====================")
	w("# name    : 任务名称（显示在任务列表）")
	w("# enabled : false 时该任务不参与手动与定时同步")
	w("")
	w("name: " + yamlStr(c.Name))
	w("enabled: " + yamlBool(c.Enabled))
	w("")
	w("# -----------------------------------------------------------")
	w("# left  : 左侧文件夹（one-way 模式下作为源）")
	w("# right : 右侧文件夹（one-way 模式下作为目标）")
	w("# mode  : one-way = 左→右 | one-way-back = 右→左 | two-way = 双向保持一致")
	w("left: " + yamlStr(c.Left))
	w("right: " + yamlStr(c.Right))
	w("mode: " + string(c.Mode))
	w("")
	w("# -----------------------------------------------------------")
	w("# incremental    : true 时仅同步有变化的文件（大小+修改时间比对），提高效率")
	w("# mirror         : 单向模式下，源端已删除的文件在目标端同步删除")
	w("# conflict       : 双向模式冲突策略：newer=较新者胜 | left | right | skip=跳过并报告")
	w("# backup_deleted : 删除或覆盖前，先把原文件备份到该侧 .filesync-trash/ 目录")
	w("incremental: " + yamlBool(c.Incremental))
	w("mirror: " + yamlBool(c.Mirror))
	w("conflict: " + string(c.Conflict))
	w("backup_deleted: " + yamlBool(c.BackupDeleted))
	w("")
	w("# -----------------------------------------------------------")
	w("# 文件过滤：glob 通配符（* 任意字符、? 单字符、** 跨目录、尾部 / 匹配目录及其内容）")
	w("# exclude  : 排除规则列表，命中即不同步")
	w("# include  : 仅同步规则列表，非空时只同步命中的文件")
	w("filters:")
	if len(c.Filters.Exclude) == 0 {
		w("  exclude: []")
	} else {
		w("  exclude:")
		for _, x := range c.Filters.Exclude {
			w("    - " + yamlStr(x))
		}
	}
	if len(c.Filters.Include) == 0 {
		w("  include: []")
	} else {
		w("  include:")
		for _, x := range c.Filters.Include {
			w("    - " + yamlStr(x))
		}
	}
	w("  case_sensitive: " + yamlBool(c.Filters.CaseSensitive))
	w("")
	w("# -----------------------------------------------------------")
	w("# 定时同步：type = interval（每 N 分钟一次）| daily（每天固定时刻 HH:MM）")
	w("schedule:")
	w("  enabled: " + yamlBool(c.Schedule.Enabled))
	w("  type: " + c.Schedule.Type)
	w("  interval_minutes: " + fmt.Sprintf("%d", c.Schedule.IntervalMinutes))
	w("  daily_time: \"" + c.Schedule.DailyTime + "\"")
	return b.String()
}

// yamlStr 字符串值排版：需要时加双引号
func yamlStr(s string) string {
	if needsQuote(s) {
		return `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(s) + `"`
	}
	return s
}

func needsQuote(s string) bool {
	if s == "" {
		return true
	}
	lower := strings.ToLower(s)
	switch lower {
	case "true", "false", "yes", "no", "on", "off", "null", "~":
		return true
	}
	if strings.TrimSpace(s) != s {
		return true
	}
	if strings.ContainsAny(s, ":#{}[],&*!|>'\"%@`") {
		// 含引号但不含其它特殊字符的仍需引号；简单起见一律加引号
		return true
	}
	if strings.Contains(s, ": ") || strings.Contains(s, " -") || strings.HasPrefix(s, "- ") {
		return true
	}
	// 纯数字样式需要引号避免类型歧义
	allNum := true
	for _, r := range s {
		if (r < '0' || r > '9') && r != '.' && r != '-' && r != '+' {
			allNum = false
			break
		}
	}
	if allNum && s != "" {
		return true
	}
	return false
}

func yamlBool(v bool) string {
	if v {
		return "true"
	}
	return "false"
}

// ParseYAML 解析 YAML 文本为任务配置（严格模式：未知字段报错，帮助用户发现拼写错误）
func ParseYAML(text string) (TaskConfig, error) {
	var c TaskConfig
	dec := yaml.NewDecoder(strings.NewReader(text))
	dec.KnownFields(true)
	if err := dec.Decode(&c); err != nil {
		return c, fmt.Errorf("YAML 解析失败: %w", err)
	}
	if c.Mode == "" {
		c.Mode = ModeOneWay
	}
	if c.Conflict == "" {
		c.Conflict = ConflictNewer
	}
	return c, nil
}
