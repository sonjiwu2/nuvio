package operations

import (
	"time"

	"github.com/sonjiwu2/nuvio/internal/fswalk"
)

// ConflictPolicy decides what happens when an item's destination already
// exists. The zero value is ConflictSkip — the safe default — never
// ConflictReplace.
type ConflictPolicy string

const (
	// ConflictSkip leaves both the source and the existing destination
	// untouched.
	ConflictSkip ConflictPolicy = "skip"
	// ConflictKeepBoth renames the incoming file (e.g. "name (1).ext")
	// rather than colliding with what's already there.
	ConflictKeepBoth ConflictPolicy = "keep_both"
	// ConflictReplace deletes the existing destination file and moves the
	// source into its place. Only ever used when the caller — a human, in
	// an explicit choice — opted into it.
	ConflictReplace ConflictPolicy = "replace"
)

// MoveItem is one file Apply is asked to move.
type MoveItem struct {
	Source      string
	Destination string
}

// Outcome is what actually happened to one MoveItem.
type Outcome string

const (
	OutcomeMoved   Outcome = "moved"
	OutcomeSkipped Outcome = "skipped"
	OutcomeFailed  Outcome = "failed"
)

// ItemResult is the outcome of one MoveItem within a batch.
type ItemResult struct {
	Source string `json:"source"`
	// Destination is the actual path the file ended up at, which can
	// differ from the requested one under ConflictKeepBoth. Empty when
	// Outcome is not Moved.
	Destination string  `json:"destination"`
	Outcome     Outcome `json:"outcome"`
	Error       string  `json:"error,omitempty"`
}

// Options configures a single Apply batch.
type Options struct {
	// ConflictPolicy decides what happens when an item's destination
	// already exists. The zero value (empty string) is treated as
	// ConflictSkip.
	ConflictPolicy ConflictPolicy

	// Workers bounds how many files can be moved concurrently. Zero
	// selects a conservative default — lower than scanning's, since a
	// bulk move is a heavier, more consequential operation than reading a
	// directory listing, and easier to reason about with less concurrent
	// filesystem churn.
	Workers int
}

const defaultWorkers = 4

const maxWorkers = 8

func (o Options) withDefaults() Options {
	if o.ConflictPolicy == "" {
		o.ConflictPolicy = ConflictSkip
	}
	if o.Workers <= 0 {
		n := fswalk.DefaultWorkerCount()
		if n > defaultWorkers {
			n = defaultWorkers
		}
		o.Workers = n
	}
	if o.Workers > maxWorkers {
		o.Workers = maxWorkers
	}
	return o
}

// Progress is a snapshot of batch execution, delivered as each item
// finishes.
type Progress struct {
	Completed   int64  `json:"completed"`
	Total       int64  `json:"total"`
	CurrentPath string `json:"currentPath"`
}

// Result is the outcome of a completed or cancelled Apply batch.
type Result struct {
	BatchID   string        `json:"batchId"`
	Items     []ItemResult  `json:"items"`
	Succeeded int64         `json:"succeeded"`
	Skipped   int64         `json:"skipped"`
	Failed    int64         `json:"failed"`
	Cancelled bool          `json:"cancelled"`
	Duration  time.Duration `json:"durationNs"`
}

// JournalEntry records one successfully executed move, durably enough to
// support Undo. Only OutcomeMoved items are journaled — a skipped or
// failed item never touched the filesystem, so there's nothing to
// reverse.
type JournalEntry struct {
	ID          string
	BatchID     string
	Source      string
	Destination string
	ExecutedAt  time.Time
	UndoneAt    *time.Time
}

// UndoItemResult is the outcome of reversing one JournalEntry.
type UndoItemResult struct {
	Source      string  `json:"source"`
	Destination string  `json:"destination"`
	Outcome     Outcome `json:"outcome"`
	Error       string  `json:"error,omitempty"`
}

// UndoResult is the outcome of undoing a whole batch.
type UndoResult struct {
	BatchID  string           `json:"batchId"`
	Items    []UndoItemResult `json:"items"`
	Restored int64            `json:"restored"`
	Skipped  int64            `json:"skipped"`
	Failed   int64            `json:"failed"`
}
