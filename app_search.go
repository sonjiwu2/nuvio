package main

import (
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/sonjiwu2/nuvio/internal/search"
)

type searchStartedEvent struct {
	ID    string `json:"id"`
	Root  string `json:"root"`
	Query string `json:"query"`
}

type searchProgressEvent struct {
	ID string `json:"id"`
	search.Progress
}

type searchMatchesEvent struct {
	ID      string         `json:"id"`
	Matches []search.Match `json:"matches"`
}

type searchCompletedEvent struct {
	ID string `json:"id"`
	search.Result
}

type searchFailedEvent struct {
	ID    string `json:"id"`
	Error string `json:"error"`
}

const (
	eventSearchStarted   = "search.started"
	eventSearchProgress  = "search.progress"
	eventSearchMatches   = "search.matches"
	eventSearchCompleted = "search.completed"
	eventSearchCancelled = "search.cancelled"
	eventSearchFailed    = "search.failed"
)

// StartSearch begins searching root for files whose name contains query
// and returns its search id immediately. Matches stream in as
// "search.matches" batches while the search runs; the final outcome is
// "search.completed" / "search.cancelled" / "search.failed".
func (a *App) StartSearch(root, query string) (string, error) {
	id, err := a.searchC.Start(root, search.Options{Query: query}, search.Callbacks{
		OnProgress: func(id string, p search.Progress) {
			wailsruntime.EventsEmit(a.ctx, eventSearchProgress, searchProgressEvent{ID: id, Progress: p})
		},
		OnMatches: func(id string, batch []search.Match) {
			wailsruntime.EventsEmit(a.ctx, eventSearchMatches, searchMatchesEvent{ID: id, Matches: batch})
		},
		OnComplete: func(id string, r search.Result) {
			event := eventSearchCompleted
			if r.Cancelled {
				event = eventSearchCancelled
			}
			wailsruntime.EventsEmit(a.ctx, event, searchCompletedEvent{ID: id, Result: r})
		},
		OnFailed: func(id string, err error) {
			a.logger.Error("search failed", "search_id", id, "error", err)
			wailsruntime.EventsEmit(a.ctx, eventSearchFailed, searchFailedEvent{ID: id, Error: err.Error()})
		},
	})
	if err != nil {
		return "", err
	}

	wailsruntime.EventsEmit(a.ctx, eventSearchStarted, searchStartedEvent{ID: id, Root: root, Query: query})
	return id, nil
}

// CancelSearch requests that the given search stop. It is a no-op if the
// search has already finished or the id is unknown.
func (a *App) CancelSearch(id string) {
	a.searchC.Cancel(id)
}
