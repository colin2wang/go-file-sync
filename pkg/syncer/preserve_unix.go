//go:build !windows

package syncer

import (
	"os"
	"syscall"
)

// chownFrom copies the owner (uid/gid) of the source file onto dst.
func chownFrom(srcInfo os.FileInfo, dst string) error {
	if stat, ok := srcInfo.Sys().(*syscall.Stat_t); ok {
		return os.Chown(dst, int(stat.Uid), int(stat.Gid))
	}
	return nil
}
