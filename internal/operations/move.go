package operations

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// moveOne resolves any conflict at destination per policy, then performs
// the move. It never overwrites an existing file unless policy is
// explicitly ConflictReplace.
func moveOne(source, destination string, policy ConflictPolicy) (actualDestination string, outcome Outcome, err error) {
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		return "", OutcomeFailed, fmt.Errorf("create destination directory: %w", err)
	}

	finalDest := destination
	if _, statErr := os.Lstat(destination); statErr == nil {
		switch policy {
		case ConflictSkip, "":
			return "", OutcomeSkipped, nil
		case ConflictKeepBoth:
			finalDest = uniquePath(destination)
		case ConflictReplace:
			// Explicit, caller-chosen removal of what's currently there —
			// never done implicitly. We remove rather than lean on
			// os.Rename's overwrite semantics, which differ enough
			// between platforms that relying on them here would be
			// gambling with a user's file.
			if err := os.Remove(destination); err != nil {
				return "", OutcomeFailed, fmt.Errorf("remove existing destination: %w", err)
			}
		default:
			return "", OutcomeSkipped, nil
		}
	} else if !os.IsNotExist(statErr) {
		return "", OutcomeFailed, fmt.Errorf("check destination: %w", statErr)
	}

	if err := renameOrCopy(source, finalDest); err != nil {
		return "", OutcomeFailed, err
	}
	return finalDest, OutcomeMoved, nil
}

// renameOrCopy moves source to dest. os.Rename is tried first (atomic,
// cheap); if it fails — most commonly because source and dest are on
// different volumes, which os.Rename cannot do — it falls back to a
// verified copy-then-remove.
func renameOrCopy(source, dest string) error {
	if err := os.Rename(source, dest); err == nil {
		return nil
	}
	return copyThenRemove(source, dest)
}

// copyThenRemove copies source to dest, verifies the copy matches before
// touching anything else, and only then removes source. The original is
// never deleted until the copy is confirmed intact, and O_EXCL refuses to
// silently overwrite anything that appeared at dest between the caller's
// conflict check and this call.
func copyThenRemove(source, dest string) error {
	in, err := os.Open(source)
	if err != nil {
		return fmt.Errorf("open source: %w", err)
	}

	out, err := os.OpenFile(dest, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		_ = in.Close()
		return fmt.Errorf("create destination: %w", err)
	}

	_, copyErr := io.Copy(out, in)
	// Closed here, not deferred: os.Remove(source) below fails on Windows
	// while this handle is still open — files there can't be deleted
	// while in use, unlike POSIX unlink — so it must happen before we get
	// anywhere near removing the source, on every path out of this
	// function, including the error ones.
	closeErr := in.Close()

	if copyErr != nil {
		_ = out.Close()
		_ = os.Remove(dest)
		return fmt.Errorf("copy contents: %w", copyErr)
	}
	if closeErr != nil {
		_ = out.Close()
		_ = os.Remove(dest)
		return fmt.Errorf("close source: %w", closeErr)
	}
	if err := out.Close(); err != nil {
		_ = os.Remove(dest)
		return fmt.Errorf("finalize destination: %w", err)
	}

	srcInfo, err := os.Stat(source)
	if err != nil {
		return fmt.Errorf("stat source after copy: %w", err)
	}
	dstInfo, err := os.Stat(dest)
	if err != nil {
		return fmt.Errorf("stat destination after copy: %w", err)
	}
	if srcInfo.Size() != dstInfo.Size() {
		_ = os.Remove(dest)
		return fmt.Errorf("copy size mismatch: source %d bytes, destination %d bytes", srcInfo.Size(), dstInfo.Size())
	}

	if err := os.Remove(source); err != nil {
		return fmt.Errorf("remove source after copy: %w", err)
	}
	return nil
}

// uniquePath finds a free "name (1).ext", "name (2).ext", ... sibling of
// path so a ConflictKeepBoth move never collides with anything.
func uniquePath(path string) string {
	ext := filepath.Ext(path)
	base := strings.TrimSuffix(path, ext)

	for i := 1; ; i++ {
		candidate := fmt.Sprintf("%s (%d)%s", base, i, ext)
		if _, err := os.Lstat(candidate); os.IsNotExist(err) {
			return candidate
		}
	}
}
