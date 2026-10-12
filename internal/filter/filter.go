// Package filter 实现 .gitignore 风格的 glob 过滤规则。
// 语法：* 单段内任意字符、? 单字符、** 跨目录、尾部 / 仅匹配目录及其全部内容。
// 不含斜杠的模式匹配任意层级的文件/目录段（如 *.tmp 命中 a/b/c.tmp）。
package filter

import (
	"fmt"
	"regexp"
	"strings"
)

type compiled struct {
	re  *regexp.Regexp
	raw string
}

// Matcher 过滤器；内置排除同步目录内的 .filesync-trash/ 与 .filesync/
type Matcher struct {
	exclude       []compiled
	include       []compiled
	hasInclude    bool
	caseSensitive bool
}

// BuiltInExcludes 内置排除（备份目录与状态目录，避免回环同步）
var BuiltInExcludes = []string{".filesync-trash/", ".filesync/"}

// New 构建过滤器
func New(exclude, include []string, caseSensitive bool) (*Matcher, error) {
	m := &Matcher{caseSensitive: caseSensitive}
	for _, raw := range append(append([]string{}, BuiltInExcludes...), exclude...) {
		p := strings.TrimSpace(raw)
		if p == "" {
			continue
		}
		c, err := compile(p, caseSensitive)
		if err != nil {
			return nil, fmt.Errorf("无效的排除规则 %q: %w", raw, err)
		}
		m.exclude = append(m.exclude, c)
	}
	for _, raw := range include {
		p := strings.TrimSpace(raw)
		if p == "" {
			continue
		}
		c, err := compile(p, caseSensitive)
		if err != nil {
			return nil, fmt.Errorf("无效的仅同步规则 %q: %w", raw, err)
		}
		m.include = append(m.include, c)
	}
	m.hasInclude = len(m.include) > 0
	return m, nil
}

// NewMust 构建过滤器；规则非法时 panic（仅用于测试）
func NewMust(exclude, include []string, caseSensitive bool) *Matcher {
	m, err := New(exclude, include, caseSensitive)
	if err != nil {
		panic(err)
	}
	return m
}

// Allowed 报告相对路径（斜杠分隔）是否允许同步。
// 目录被排除时调用方应跳过整个子树。
func (m *Matcher) Allowed(relPath string, isDir bool) bool {
	rel := strings.TrimPrefix(strings.ReplaceAll(relPath, "\\", "/"), "./")
	if !m.caseSensitive {
		rel = strings.ToLower(rel)
	}
	for _, p := range m.exclude {
		if p.re.MatchString(rel) {
			return false
		}
	}
	if m.hasInclude && !isDir {
		for _, p := range m.include {
			if p.re.MatchString(rel) {
				return true
			}
		}
		return false
	}
	return true
}

// compile 将 glob 规则编译为正则
func compile(pattern string, caseSensitive bool) (compiled, error) {
	p := pattern
	dirOnly := strings.HasSuffix(p, "/")
	p = strings.TrimSuffix(p, "/")
	if p == "" {
		return compiled{}, fmt.Errorf("空模式")
	}
	segStyle := !strings.Contains(p, "/") // 无斜杠 → 匹配任意层级

	segs := strings.Split(p, "/")
	var sb strings.Builder
	sb.WriteString("^")
	if segStyle {
		sb.WriteString("(?:.*/)?")
	}
	// skipSep：中间 ** 段的 (?:.*/)？已包含与下一段之间的斜杠
	skipSep := false
	for i, seg := range segs {
		if skipSep {
			skipSep = false
		} else if i > 0 {
			sb.WriteString("/")
		}
		if seg == "**" {
			if i == len(segs)-1 {
				sb.WriteString(".*") // 尾部 ** ：其余全部内容
			} else {
				sb.WriteString("(?:.*/)?") // 中间 ** ：零或多段，a/**/b → a/(?:.*/)？b
				skipSep = true
			}
			continue
		}
		for _, r := range seg {
			switch r {
			case '*':
				sb.WriteString("[^/]*")
			case '?':
				sb.WriteString("[^/]")
			default:
				sb.WriteString(regexp.QuoteMeta(string(r)))
			}
		}
	}
	body := sb.String()
	if dirOnly {
		body += "(?:/.*)?" // 目录自身 + 子树
	}
	body += "$"
	if !caseSensitive {
		body = "(?i)" + body
	}
	re, err := regexp.Compile(body)
	if err != nil {
		return compiled{}, err
	}
	return compiled{re: re, raw: pattern}, nil
}
