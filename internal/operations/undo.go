package operations

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
)

// Undo reverses every not-yet-undone item in batchID, most recently
// moved first. It re-checks safety at undo time, not just relying on the
// original journal entry: an item is skipped, never forced, if the moved
// file is no longer where it was left, or if the original source
// location has since been reoccupied by something else — undoing must
// never destroy a file the user created after the original move. Those
// safety skips are reported per item, not treated as failures.
func Undo(ctx context.Context, journal *Journal, batchID string) (UndoResult, error) {
	entries, err := journal.ListBatch(ctx, batchID)
	if err != nil {
		return UndoResult{}, err
	}
	if len(entries) == 0 {
		return UndoResult{}, fmt.Errorf("operations: no undoable items in batch %q", batchID)
	}

	items := make([]UndoItemResult, 0, len(entries))
	var restored, skipped, failed int64

	// Reverse chronological order: undo the most recent move first.
	for i := len(entries) - 1; i >= 0; i-- {
		if ctx.Err() != nil {
			break
		}
		entry := entries[i]

		result := undoOne(entry)
		items = append(items, result)

		switch result.Outcome {
		case OutcomeMoved:
			restored++
			// Best-effort: if this write fails, a repeat Undo call on the
			// same batch would re-attempt this entry, but undoOne's own
			// safety checks make that a safe no-op (the destination is
			// gone, so it will be skipped) rather than a double-move.
			_ = journal.markUndone(ctx, entry.ID)
		case OutcomeSkipped:
			skipped++
		case OutcomeFailed:
			failed++
		}
	}

	return UndoResult{
		BatchID:  batchID,
		Items:    items,
		Restored: restored,
		Skipped:  skipped,
		Failed:   failed,
	}, nil
}

func undoOne(entry JournalEntry) UndoItemResult {
	result := UndoItemResult{Source: entry.Destination, Destination: entry.Source}

	if _, err := os.Lstat(entry.Destination); err != nil {
		result.Outcome = OutcomeSkipped
		result.Error = "file is no longer at its moved-to location"
		return result
	}

	if _, err := os.Lstat(entry.Source); err == nil {
		result.Outcome = OutcomeSkipped
		result.Error = "a different file now exists at the original location"
		return result
	} else if !os.IsNotExist(err) {
		result.Outcome = OutcomeFailed
		result.Error = err.Error()
		return result
	}

	if err := os.MkdirAll(filepath.Dir(entry.Source), 0o755); err != nil {
		result.Outcome = OutcomeFailed
		result.Error = "recreate original directory: " + err.Error()
		return result
	}

	if err := renameOrCopy(entry.Destination, entry.Source); err != nil {
		result.Outcome = OutcomeFailed
		result.Error = err.Error()
		return result
	}

	result.Outcome = OutcomeMoved
	return result
}
