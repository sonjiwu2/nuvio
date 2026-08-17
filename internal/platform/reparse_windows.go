//go:build windows

package platform

import "golang.org/x/sys/windows"

// IsReparsePoint reports whether path is a filesystem reparse point — a
// symlink or an NTFS junction/mount point — without following it.
//
// This exists because Go's os.DirEntry.Type() does not reliably surface
// fs.ModeSymlink for NTFS junctions the way it does for true symlinks
// (verified empirically: a junction created with `mklink /J` is reported
// as a plain directory by os.ReadDir). Checking FILE_ATTRIBUTE_REPARSE_POINT
// directly is the only reliable way to detect either kind of reparse point,
// which the scanner needs so it never traverses into one — see
// internal/scanner's package doc for why that makes cycles structurally
// impossible.
func IsReparsePoint(path string) (bool, error) {
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return false, err
	}

	attrs, err := windows.GetFileAttributes(p)
	if err != nil {
		return false, err
	}

	return attrs&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0, nil
}
