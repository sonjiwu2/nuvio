// Package logging configures Nuvio's structured logger.
//
// Logs are JSON, written to both the log file and stderr, and must never
// contain filenames, absolute paths, file contents, or other user data —
// see CLAUDE.md section 30. Call sites attach contextual fields (operation
// ids, durations, counts) instead of interpolating them into the message.
package logging

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"time"
)

// New opens (or creates) the log file in dir and returns a logger that
// writes structured JSON records to both that file and stderr. The
// returned closer must be called on application shutdown.
func New(dir string) (*slog.Logger, io.Closer, error) {
	path := filepath.Join(dir, "nuvio.log")

	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, nil, err
	}

	handler := slog.NewJSONHandler(io.MultiWriter(file, os.Stderr), &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})

	logger := slog.New(handler).With(
		"app", "nuvio",
		"started_at", time.Now().UTC().Format(time.RFC3339),
	)

	return logger, file, nil
}
