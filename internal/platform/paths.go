// Package platform resolves OS-specific application directories.
//
// Nuvio is Windows-first: on Windows these paths land under %AppData%,
// matching what Explorer, antivirus exclusion lists, and other desktop
// tooling expect. macOS/Linux fall back to the standard XDG-style locations
// so the domain layer never has to branch on runtime.GOOS itself.
package platform

import (
	"fmt"
	"os"
	"path/filepath"
)

const appDirName = "Nuvio"

// AppDataDir returns the directory Nuvio uses for persistent application
// state (SQLite database, settings). It is created if it does not exist.
func AppDataDir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("platform: resolve user config dir: %w", err)
	}

	dir := filepath.Join(base, appDirName)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("platform: create app data dir %q: %w", dir, err)
	}

	return dir, nil
}

// LogDir returns the directory Nuvio writes structured logs to. It is
// created if it does not exist.
func LogDir() (string, error) {
	dataDir, err := AppDataDir()
	if err != nil {
		return "", err
	}

	dir := filepath.Join(dataDir, "logs")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("platform: create log dir %q: %w", dir, err)
	}

	return dir, nil
}
