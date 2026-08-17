//go:build !windows

package platform

import "os"

// IsReparsePoint reports whether path is a symlink, without following it.
// Non-Windows platforms have no junction equivalent, so this reduces to
// the standard symlink check.
func IsReparsePoint(path string) (bool, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return false, err
	}
	return info.Mode()&os.ModeSymlink != 0, nil
}
