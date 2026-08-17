package main

import (
	"context"
	"log/slog"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/sonjiwu2/nuvio/internal/scanner"
)

// App is the Wails application bridge. It stays thin: it owns the runtime
// context and exposes bound methods to the frontend, but delegates all real
// work to internal/ domain packages.
type App struct {
	ctx    context.Context
	logger *slog.Logger
	scans  *scanner.Coordinator
}

// NewApp creates a new App application struct.
func NewApp(logger *slog.Logger) *App {
	return &App{logger: logger, scans: scanner.NewCoordinator()}
}

// startup is called when the app starts. The context is saved so we can
// call Wails runtime methods (e.g. dialogs, events) from bound methods.
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

// PickFolder opens a native "choose a folder" dialog and returns the
// chosen absolute path, or "" if the user cancelled the dialog.
func (a *App) PickFolder() (string, error) {
	return wailsruntime.OpenDirectoryDialog(a.ctx, wailsruntime.OpenDialogOptions{
		Title: "Choose a folder to scan",
	})
}

// scanStartedEvent, scanProgressEvent, scanCompletedEvent, and
// scanFailedEvent are the JSON payloads for the "scan.*" events. They are
// documented here rather than generated because Wails only generates TS
// bindings for bound-method signatures, not for event payloads.
type scanStartedEvent struct {
	ID   string `json:"id"`
	Root string `json:"root"`
}

type scanProgressEvent struct {
	ID string `json:"id"`
	scanner.Progress
}

type scanCompletedEvent struct {
	ID string `json:"id"`
	scanner.Result
}

type scanFailedEvent struct {
	ID    string `json:"id"`
	Error string `json:"error"`
}

const (
	eventScanStarted   = "scan.started"
	eventScanProgress  = "scan.progress"
	eventScanCompleted = "scan.completed"
	eventScanCancelled = "scan.cancelled"
	eventScanFailed    = "scan.failed"
)

// StartScan begins scanning root in the background and returns its scan
// id immediately. Progress and the final outcome are delivered as
// "scan.progress" / "scan.completed" / "scan.cancelled" / "scan.failed"
// events carrying that id, so the frontend can start a new scan without
// waiting for a previous one on a different folder to finish.
func (a *App) StartScan(root string) (string, error) {
	id, err := a.scans.Start(root, scanner.Options{}, scanner.Callbacks{
		OnProgress: func(id string, p scanner.Progress) {
			wailsruntime.EventsEmit(a.ctx, eventScanProgress, scanProgressEvent{ID: id, Progress: p})
		},
		OnComplete: func(id string, r scanner.Result) {
			event := eventScanCompleted
			if r.Cancelled {
				event = eventScanCancelled
			}
			wailsruntime.EventsEmit(a.ctx, event, scanCompletedEvent{ID: id, Result: r})
		},
		OnFailed: func(id string, err error) {
			a.logger.Error("scan failed", "scan_id", id, "error", err)
			wailsruntime.EventsEmit(a.ctx, eventScanFailed, scanFailedEvent{ID: id, Error: err.Error()})
		},
	})
	if err != nil {
		return "", err
	}

	wailsruntime.EventsEmit(a.ctx, eventScanStarted, scanStartedEvent{ID: id, Root: root})
	return id, nil
}

// CancelScan requests that the given scan stop. It is a no-op if the scan
// has already finished or the id is unknown.
func (a *App) CancelScan(id string) {
	a.scans.Cancel(id)
}
