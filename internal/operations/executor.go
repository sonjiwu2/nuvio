package operations

import (
	"context"
	"sync"
	"time"

	"github.com/sonjiwu2/nuvio/internal/fswalk"
)

// Apply moves every item, resolving conflicts per opts.ConflictPolicy,
// and journals each successful move so it can later be undone. It stops
// starting new moves as soon as ctx is cancelled — an item already in
// flight is allowed to finish rather than being interrupted mid-write —
// and returns a normal Result with Cancelled set, not an error.
//
// A per-item failure (permission denied, disk full, ...) never aborts
// the rest of the batch; it's recorded in that item's ItemResult and the
// batch continues.
func Apply(
	ctx context.Context,
	journal *Journal,
	items []MoveItem,
	opts Options,
	onProgress func(Progress),
) (Result, error) {
	started := time.Now()
	opts = opts.withDefaults()

	batchID, err := newID()
	if err != nil {
		return Result{}, err
	}

	pool := fswalk.NewPool(opts.Workers)
	var wg sync.WaitGroup
	var mu sync.Mutex
	results := make([]ItemResult, 0, len(items))
	var completed int64

	for _, item := range items {
		if ctx.Err() != nil {
			break
		}

		item := item
		pool.Go(&wg, func() {
			if ctx.Err() != nil {
				return
			}

			actualDest, outcome, moveErr := moveOne(item.Source, item.Destination, opts.ConflictPolicy)
			result := ItemResult{Source: item.Source, Destination: actualDest, Outcome: outcome}
			if moveErr != nil {
				result.Error = moveErr.Error()
			}

			if outcome == OutcomeMoved {
				if _, err := journal.Record(ctx, batchID, item.Source, actualDest); err != nil {
					// The move already happened; we can't undo that just
					// because journaling failed. Surface it via the
					// item's error so the user knows this one might not
					// be undoable, without claiming the move itself
					// failed.
					result.Error = "moved, but could not be recorded for undo: " + err.Error()
				}
			}

			mu.Lock()
			results = append(results, result)
			completed++
			n := completed
			mu.Unlock()

			if onProgress != nil {
				onProgress(Progress{Completed: n, Total: int64(len(items)), CurrentPath: item.Source})
			}
		})
	}
	wg.Wait()

	var succeeded, skipped, failed int64
	for _, r := range results {
		switch r.Outcome {
		case OutcomeMoved:
			succeeded++
		case OutcomeSkipped:
			skipped++
		case OutcomeFailed:
			failed++
		}
	}

	return Result{
		BatchID:   batchID,
		Items:     results,
		Succeeded: succeeded,
		Skipped:   skipped,
		Failed:    failed,
		Cancelled: ctx.Err() != nil,
		Duration:  time.Since(started),
	}, nil
}
