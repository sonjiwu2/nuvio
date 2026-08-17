package operations

import (
	"context"
	"fmt"
	"sync"
)

// Coordinator runs and tracks Apply batches by id, mirroring the
// Coordinator types in internal/scanner, internal/search,
// internal/rules, and internal/duplicates.
type Coordinator struct {
	journal *Journal

	mu     sync.Mutex
	active map[string]context.CancelFunc
}

// NewCoordinator creates a Coordinator that journals through journal.
func NewCoordinator(journal *Journal) *Coordinator {
	return &Coordinator{journal: journal, active: make(map[string]context.CancelFunc)}
}

// Callbacks receives lifecycle events for one Apply batch.
type Callbacks struct {
	OnProgress func(id string, p Progress)
	OnComplete func(id string, r Result)
	OnFailed   func(id string, err error)
}

// Start begins applying items on a new goroutine and returns immediately
// with a batch id. That id is the same one the journal records under —
// so it can be handed straight to Undo later — because Coordinator
// generates it up front and passes it into the same execution path Apply
// uses internally, rather than letting two separate calls mint two
// different ids for what's supposed to be one batch. Exactly one of
// OnComplete or OnFailed is called exactly once when the batch finishes.
func (c *Coordinator) Start(items []MoveItem, opts Options, cb Callbacks) (string, error) {
	batchID, err := newID()
	if err != nil {
		return "", fmt.Errorf("operations: generate batch id: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	c.mu.Lock()
	c.active[batchID] = cancel
	c.mu.Unlock()

	go func() {
		defer c.forget(batchID)

		var onProgress func(Progress)
		if cb.OnProgress != nil {
			onProgress = func(p Progress) { cb.OnProgress(batchID, p) }
		}

		result, err := applyBatch(ctx, c.journal, batchID, items, opts, onProgress)
		if err != nil {
			if cb.OnFailed != nil {
				cb.OnFailed(batchID, err)
			}
			return
		}
		if cb.OnComplete != nil {
			cb.OnComplete(batchID, result)
		}
	}()

	return batchID, nil
}

func (c *Coordinator) forget(id string) {
	c.mu.Lock()
	delete(c.active, id)
	c.mu.Unlock()
}

// Cancel requests that the given batch stop starting new moves. It
// returns false if no batch with that id is currently running.
func (c *Coordinator) Cancel(id string) bool {
	c.mu.Lock()
	cancel, ok := c.active[id]
	c.mu.Unlock()

	if !ok {
		return false
	}
	cancel()
	return true
}
