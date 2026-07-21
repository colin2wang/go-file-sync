//go:build windows

package syncer

import "os"

// chownFrom is a no-op on Windows, where owner preservation is not applicable.
func chownFrom(srcInfo os.FileInfo, dst string) error {
	return nil
}
