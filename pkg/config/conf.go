package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// confParser is a lightweight parser for .conf files.
// Format:
//
//	# comment
//	[section]
//	key = "value"
//	key2 = ["item1", "item2"]
//	key3 = true
//	key4 = 123
type confParser struct {
	sections map[string]map[string]string
}

func newConfParser() *confParser {
	return &confParser{
		sections: make(map[string]map[string]string),
	}
}

func (p *confParser) parse(data string) error {
	var currentSection string
	lines := strings.Split(data, "\n")

	for i, line := range lines {
		line = strings.TrimSpace(line)

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}

		// Section header: [section]
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			currentSection = line[1 : len(line)-1]
			if _, exists := p.sections[currentSection]; !exists {
				p.sections[currentSection] = make(map[string]string)
			}
			continue
		}

		if currentSection == "" {
			return fmt.Errorf("line %d: key-value pair outside of section: %s", i+1, line)
		}

		// Key = Value
		eqIdx := strings.Index(line, "=")
		if eqIdx < 0 {
			return fmt.Errorf("line %d: malformed key-value pair: %s", i+1, line)
		}

		key := strings.TrimSpace(line[:eqIdx])
		value := strings.TrimSpace(line[eqIdx+1:])

		p.sections[currentSection][key] = value
	}

	return nil
}

func (p *confParser) getString(section, key string) (string, error) {
	s, ok := p.sections[section]
	if !ok {
		return "", fmt.Errorf("section [%s] not found", section)
	}
	val, ok := s[key]
	if !ok {
		return "", fmt.Errorf("key %q in section [%s] not found", key, section)
	}
	return strings.Trim(val, `"`), nil
}

func (p *confParser) getStringSlice(section, key string) ([]string, error) {
	s, ok := p.sections[section]
	if !ok {
		return nil, fmt.Errorf("section [%s] not found", section)
	}
	val, ok := s[key]
	if !ok {
		return nil, fmt.Errorf("key %q in section [%s] not found", key, section)
	}
	return parseStringSlice(val), nil
}

func (p *confParser) getBool(section, key string) (bool, error) {
	s, ok := p.sections[section]
	if !ok {
		return false, fmt.Errorf("section [%s] not found", section)
	}
	val, ok := s[key]
	if !ok {
		return false, fmt.Errorf("key %q in section [%s] not found", key, section)
	}
	val = strings.Trim(strings.ToLower(val), `"'`)
	switch val {
	case "true", "yes", "1":
		return true, nil
	case "false", "no", "0":
		return false, nil
	default:
		return false, fmt.Errorf("cannot parse %q as bool", val)
	}
}

func (p *confParser) getInt(section, key string) (int, error) {
	s, ok := p.sections[section]
	if !ok {
		return 0, fmt.Errorf("section [%s] not found", section)
	}
	val, ok := s[key]
	if !ok {
		return 0, fmt.Errorf("key %q in section [%s] not found", key, section)
	}
	return strconv.Atoi(strings.TrimSpace(val))
}

// parseStringSlice parses ["item1", "item2"] or "single" into a string slice.
func parseStringSlice(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return []string{}
	}

	// Array format: ["item1", "item2"]
	if strings.HasPrefix(raw, "[") && strings.HasSuffix(raw, "]") {
		inner := raw[1 : len(raw)-1]
		parts := splitCSV(inner)
		result := make([]string, 0, len(parts))
		for _, p := range parts {
			p = strings.TrimSpace(p)
			p = strings.Trim(p, `"'`)
			if p != "" {
				result = append(result, p)
			}
		}
		return result
	}

	// Single value
	return []string{strings.Trim(raw, `"'`)}
}

// splitCSV splits a comma-separated string, respecting quoted values.
func splitCSV(s string) []string {
	var result []string
	var current strings.Builder
	inQuote := false
	quoteChar := byte(0)

	for i := 0; i < len(s); i++ {
		ch := s[i]
		switch {
		case ch == '"' || ch == '\'':
			if inQuote {
				if ch == quoteChar {
					inQuote = false
				}
				current.WriteByte(ch)
			} else {
				inQuote = true
				quoteChar = ch
				current.WriteByte(ch)
			}
		case ch == ',' && !inQuote:
			result = append(result, current.String())
			current.Reset()
		default:
			current.WriteByte(ch)
		}
	}
	if current.Len() > 0 {
		result = append(result, current.String())
	}
	return result
}

// LoadConf reads and parses a single .conf file.
func LoadConf(path string) (*TaskConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read conf %s: %w", path, err)
	}

	p := newConfParser()
	if err := p.parse(string(data)); err != nil {
		return nil, fmt.Errorf("parse conf %s: %w", path, err)
	}

	task := &TaskConfig{}

	// Parse [task] section
	if name, err := p.getString("task", "name"); err == nil {
		task.Task.Name = name
	}
	if src, err := p.getString("task", "source"); err == nil {
		task.Task.Source = src
	}
	if tgt, err := p.getString("task", "target"); err == nil {
		task.Task.Target = tgt
	}
	// Support sources/targets as multi-value lists
	if srcs, err := p.getStringSlice("task", "sources"); err == nil {
		task.Task.Sources = srcs
	}
	if tgts, err := p.getStringSlice("task", "targets"); err == nil {
		task.Task.Targets = tgts
	}
	if mode, err := p.getString("task", "mode"); err == nil {
		task.Task.Mode = mode
	}
	if excl, err := p.getStringSlice("task", "exclude"); err == nil {
		task.Task.Exclude = excl
	}
	if incl, err := p.getStringSlice("task", "include"); err == nil {
		task.Task.Include = incl
	}
	if del, err := p.getBool("task", "delete_orphans"); err == nil {
		task.Task.DeleteOrphans = del
	}
	if sym, err := p.getString("task", "symlinks"); err == nil {
		task.Task.Symlinks = sym
	}
	if sched, err := p.getString("task", "schedule"); err == nil {
		task.Task.Schedule = sched
	}

	// Parse [watch] section
	if rec, err := p.getBool("watch", "recursive"); err == nil {
		task.Watch.Recursive = rec
	}
	if events, err := p.getStringSlice("watch", "events"); err == nil {
		task.Watch.Events = events
	}
	if deb, err := p.getInt("watch", "debounce"); err == nil {
		task.Watch.Debounce = deb
	}

	// Parse [trigger] section
	if onSync, err := p.getString("trigger", "on_sync"); err == nil {
		task.Trigger.OnSync = onSync
	}
	if onComplete, err := p.getString("trigger", "on_complete"); err == nil {
		task.Trigger.OnComplete = onComplete
	}
	if onError, err := p.getString("trigger", "on_error"); err == nil {
		task.Trigger.OnError = onError
	}
	if timeout, err := p.getInt("trigger", "timeout"); err == nil {
		task.Trigger.Timeout = timeout
	}

	// Parse [inherit] section
	if file, err := p.getString("inherit", "file"); err == nil {
		task.Inherit.File = file
	}

	return task, nil
}

// ResolveConfDir resolves the conf directory path.
// If the path is relative, it's resolved relative to the given baseDir.
func ResolveConfDir(confDir, baseDir string) string {
	if filepath.IsAbs(confDir) {
		return confDir
	}
	return filepath.Join(baseDir, confDir)
}

// ListConfFiles returns all .conf files in the given directory (non-recursive).
func ListConfFiles(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read conf dir %s: %w", dir, err)
	}

	var files []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".conf") {
			files = append(files, filepath.Join(dir, entry.Name()))
		}
	}
	return files, nil
}
