package main

import (
	"fmt"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/sonjiwu2/nuvio/internal/rules"
)

type organizePreviewStartedEvent struct {
	ID   string `json:"id"`
	Root string `json:"root"`
}

type organizePreviewProgressEvent struct {
	ID string `json:"id"`
	rules.PreviewProgress
}

type organizePreviewEntriesEvent struct {
	ID      string               `json:"id"`
	Entries []rules.PreviewEntry `json:"entries"`
}

type organizePreviewCompletedEvent struct {
	ID string `json:"id"`
	rules.PreviewResult
}

type organizePreviewFailedEvent struct {
	ID    string `json:"id"`
	Error string `json:"error"`
}

const (
	eventOrganizePreviewStarted   = "organize.preview.started"
	eventOrganizePreviewProgress  = "organize.preview.progress"
	eventOrganizePreviewEntries   = "organize.preview.entries"
	eventOrganizePreviewCompleted = "organize.preview.completed"
	eventOrganizePreviewCancelled = "organize.preview.cancelled"
	eventOrganizePreviewFailed    = "organize.preview.failed"
)

// StartOrganizePreview begins a dry-run preview of every saved rule
// against root and returns its preview id immediately. It never moves,
// renames, or deletes a file — see internal/rules' package doc. Matched
// entries stream in as "organize.preview.entries" batches; the final
// outcome is "organize.preview.completed" / "...cancelled" / "...failed".
func (a *App) StartOrganizePreview(root string) (string, error) {
	activeRules, err := a.rulesStore.List(a.ctx)
	if err != nil {
		return "", err
	}
	if len(activeRules) == 0 {
		return "", fmt.Errorf("organize: no rules defined yet")
	}

	id, err := a.previews.Start(root, activeRules, rules.PreviewOptions{}, rules.PreviewCallbacks{
		OnProgress: func(id string, p rules.PreviewProgress) {
			wailsruntime.EventsEmit(a.ctx, eventOrganizePreviewProgress, organizePreviewProgressEvent{ID: id, PreviewProgress: p})
		},
		OnEntries: func(id string, batch []rules.PreviewEntry) {
			wailsruntime.EventsEmit(a.ctx, eventOrganizePreviewEntries, organizePreviewEntriesEvent{ID: id, Entries: batch})
		},
		OnComplete: func(id string, r rules.PreviewResult) {
			event := eventOrganizePreviewCompleted
			if r.Cancelled {
				event = eventOrganizePreviewCancelled
			}
			wailsruntime.EventsEmit(a.ctx, event, organizePreviewCompletedEvent{ID: id, PreviewResult: r})
		},
		OnFailed: func(id string, err error) {
			a.logger.Error("organize preview failed", "preview_id", id, "error", err)
			wailsruntime.EventsEmit(a.ctx, eventOrganizePreviewFailed, organizePreviewFailedEvent{ID: id, Error: err.Error()})
		},
	})
	if err != nil {
		return "", err
	}

	wailsruntime.EventsEmit(a.ctx, eventOrganizePreviewStarted, organizePreviewStartedEvent{ID: id, Root: root})
	return id, nil
}

// CancelOrganizePreview requests that the given preview stop. It is a
// no-op if the preview has already finished or the id is unknown.
func (a *App) CancelOrganizePreview(id string) {
	a.previews.Cancel(id)
}
