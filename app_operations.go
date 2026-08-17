package main

import (
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/sonjiwu2/nuvio/internal/operations"
)

// MoveRequestItem is one file the frontend is asking Nuvio to move. It
// mirrors internal/rules.PreviewEntry's source/destination shape — the
// frontend hands back exactly the pairs a prior preview already showed
// the user, rather than Nuvio re-deriving them, so Apply only ever
// touches files the user actually saw and approved.
type MoveRequestItem struct {
	Source      string `json:"source"`
	Destination string `json:"destination"`
}

type applyStartedEvent struct {
	ID    string `json:"id"`
	Total int    `json:"total"`
}

type applyProgressEvent struct {
	ID string `json:"id"`
	operations.Progress
}

type applyCompletedEvent struct {
	ID string `json:"id"`
	operations.Result
}

type applyFailedEvent struct {
	ID    string `json:"id"`
	Error string `json:"error"`
}

const (
	eventApplyStarted   = "operations.apply.started"
	eventApplyProgress  = "operations.apply.progress"
	eventApplyCompleted = "operations.apply.completed"
	eventApplyCancelled = "operations.apply.cancelled"
	eventApplyFailed    = "operations.apply.failed"
)

// StartOrganizeApply moves every item using the given conflict policy
// ("skip", "keep_both", or "replace"; empty defaults to "skip") and
// returns its batch id immediately. Every successful move is journaled,
// so the returned id can later be passed to UndoBatch. Progress and the
// final outcome are delivered as "operations.apply.*" events carrying
// that id.
func (a *App) StartOrganizeApply(items []MoveRequestItem, conflictPolicy string) (string, error) {
	moveItems := make([]operations.MoveItem, len(items))
	for i, it := range items {
		moveItems[i] = operations.MoveItem{Source: it.Source, Destination: it.Destination}
	}

	id, err := a.applies.Start(moveItems, operations.Options{
		ConflictPolicy: operations.ConflictPolicy(conflictPolicy),
	}, operations.Callbacks{
		OnProgress: func(id string, p operations.Progress) {
			wailsruntime.EventsEmit(a.ctx, eventApplyProgress, applyProgressEvent{ID: id, Progress: p})
		},
		OnComplete: func(id string, r operations.Result) {
			event := eventApplyCompleted
			if r.Cancelled {
				event = eventApplyCancelled
			}
			wailsruntime.EventsEmit(a.ctx, event, applyCompletedEvent{ID: id, Result: r})
		},
		OnFailed: func(id string, err error) {
			a.logger.Error("apply batch failed", "batch_id", id, "error", err)
			wailsruntime.EventsEmit(a.ctx, eventApplyFailed, applyFailedEvent{ID: id, Error: err.Error()})
		},
	})
	if err != nil {
		return "", err
	}

	wailsruntime.EventsEmit(a.ctx, eventApplyStarted, applyStartedEvent{ID: id, Total: len(items)})
	return id, nil
}

// CancelOrganizeApply requests that the given batch stop starting new
// moves. It is a no-op if the batch has already finished or the id is
// unknown. Moves already completed remain in place and undoable.
func (a *App) CancelOrganizeApply(id string) {
	a.applies.Cancel(id)
}

// UndoBatch reverses every not-yet-undone item in batchID. See
// internal/operations' package doc for the safety checks Undo performs
// before touching anything.
func (a *App) UndoBatch(batchID string) (operations.UndoResult, error) {
	return operations.Undo(a.ctx, a.journal, batchID)
}
