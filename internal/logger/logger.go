// Package logger 把同步日志写入可执行文件所在目录的 logs 子目录。
// 初始化失败时静默降级为 no-op，不影响同步功能。
package logger

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

var (
	mu  sync.Mutex
	f   *os.File
	day string // 当前日志文件对应的日期，跨天自动切换
)

// Init 打开日志文件（按天分文件），必须在程序启动时调用一次。
func Init() {
	mu.Lock()
	defer mu.Unlock()
	openLocked()
}

// Log 写入一行带时间戳的日志。未初始化或打开失败时静默忽略。
func Log(format string, args ...any) {
	mu.Lock()
	defer mu.Unlock()
	if !ensureOpenLocked() {
		return
	}
	msg := fmt.Sprintf(format, args...)
	fmt.Fprintf(f, "[%s] %s\n", time.Now().Format("2006-01-02 15:04:05"), msg)
}

// Close 关闭当前日志文件（程序退出时调用）。
func Close() {
	mu.Lock()
	defer mu.Unlock()
	if f != nil {
		_ = f.Close()
		f = nil
	}
}

// ensureOpenLocked 确保文件已打开且是今天的；失败返回 false。
func ensureOpenLocked() bool {
	today := time.Now().Format("20060102")
	if f != nil && day == today {
		return true
	}
	if f != nil {
		_ = f.Close()
		f = nil
	}
	return openLocked()
}

// openLocked 打开当天的日志文件。
func openLocked() bool {
	exe, err := os.Executable()
	if err != nil {
		return false
	}
	dir := filepath.Join(filepath.Dir(exe), "logs")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return false
	}
	name := filepath.Join(dir, "filesync-"+time.Now().Format("20060102")+".log")
	file, err := os.OpenFile(name, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return false
	}
	f = file
	day = time.Now().Format("20060102")
	return true
}
