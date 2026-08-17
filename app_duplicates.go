package main

import (
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/sonjiwu2/nuvio/internal/duplicates"
)

type duplicatesStartedEvent struct {
	ID   string `json:"id"`
	Root string `json:"root"`
}

type duplicatesProgressEvent struct {
	ID string `json:"id"`
	duplicates.Progress
}

type duplicatesCompletedEvent struct {
	ID string `json:"id"`
	duplicates.Result
}

type duplicatesFailedEvent struct {
	ID    string `json:"id"`
	Error string `json:"error"`
}

const (
	eventDuplicatesStarted   = "duplicates.started"
	eventDuplicatesProgress  = "duplicates.progress"
	eventDuplicatesCompleted = "duplicates.completed"
	eventDuplicatesCancelled = "duplicates.cancelled"
	eventDuplicatesFailed    = "duplicates.failed"
)

// StartDuplicateScan begins scanning root for files with identical
// content and returns its scan id immediately. It never deletes or
// moves a file — see internal/duplicates' package doc. Progress and the
// final outcome are delivered as "duplicates.progress" /
// "duplicates.completed" / "duplicates.cancelled" / "duplicates.failed"
// events carrying that id.
func (a *App) StartDuplicateScan(root string) (string, error) {
	id, err := a.dupes.Start(root, duplicates.Options{}, duplicates.Callbacks{
		OnProgress: func(id string, p duplicates.Progress) {
			wailsruntime.EventsEmit(a.ctx, eventDuplicatesProgress, duplicatesProgressEvent{ID: id, Progress: p})
		},
		OnComplete: func(id string, r duplicates.Result) {
			event := eventDuplicatesCompleted
			if r.Cancelled {
				event = eventDuplicatesCancelled
			}
			wailsruntime.EventsEmit(a.ctx, event, duplicatesCompletedEvent{ID: id, Result: r})
		},
		OnFailed: func(id string, err error) {
			a.logger.Error("duplicate scan failed", "scan_id", id, "error", err)
			wailsruntime.EventsEmit(a.ctx, eventDuplicatesFailed, duplicatesFailedEvent{ID: id, Error: err.Error()})
		},
	})
	if err != nil {
		return "", err
	}

	wailsruntime.EventsEmit(a.ctx, eventDuplicatesStarted, duplicatesStartedEvent{ID: id, Root: root})
	return id, nil
}

// CancelDuplicateScan requests that the given scan stop. It is a no-op
// if the scan has already finished or the id is unknown.
func (a *App) CancelDuplicateScan(id string) {
	a.dupes.Cancel(id)
}
