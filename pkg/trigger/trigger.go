// Package trigger executes external commands when sync events occur.
// Template variables like {{relpath}}, {{src}}, {{dst}} are expanded
// before the command is run.
package trigger

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

// Vars holds template variables available for command expansion.
type Vars struct {
	RelPath string
	Src     string
	Dst     string
	Task    string
	Event   string
	Error   string
}

// Executor runs external commands with template variable substitution.
type Executor struct{}

// New creates a new Executor.
func New() *Executor {
	return &Executor{}
}

// Run executes a command template with the given variables.
// Returns the combined stdout+stderr output.
func (e *Executor) Run(template string, vars Vars, timeoutSec int) (string, error) {
	if template == "" {
		return "", nil
	}

	cmdStr := os.Expand(template, func(key string) string {
		switch key {
		case "relpath":
			return vars.RelPath
		case "src":
			return vars.Src
		case "dst":
			return vars.Dst
		case "task":
			return vars.Task
		case "event":
			return vars.Event
		case "error":
			return vars.Error
		default:
			return "${" + key + "}"
		}
	})

	if timeoutSec <= 0 {
		timeoutSec = 30
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutSec)*time.Second)
	defer cancel()

	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.CommandContext(ctx, "cmd", "/c", cmdStr)
	} else {
		cmd = exec.CommandContext(ctx, "sh", "-c", cmdStr)
	}

	output, err := cmd.CombinedOutput()
	outStr := strings.TrimSpace(string(output))

	if err != nil {
		return outStr, fmt.Errorf("trigger command failed: %w\noutput: %s", err, outStr)
	}
	return outStr, nil
}
