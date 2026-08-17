package search

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
)

// Coordinator runs and tracks searches by id, mirroring
// internal/scanner.Coordinator so the Wails bridge can start a search,
// receive progress/match/completion callbacks, and cancel a specific
// search without affecting any other search running at the same time.
type Coordinator struct {
	mu     sync.Mutex
	active map[string]context.CancelFunc
}

// NewCoordinator creates an empty Coordinator.
func NewCoordinator() *Coordinator {
	return &Coordinator{active: make(map[string]context.CancelFunc)}
}

// Callbacks receives lifecycle events for one search, each given the
// search's id since a caller may be tracking several concurrently.
type Callbacks struct {
	OnProgress func(id string, p Progress)
	OnMatches  func(id string, batch []Match)
	OnComplete func(id string, r Result)
	OnFailed   func(id string, err error)
}

// Start begins searching root on a new goroutine and returns immediately
// with a search id. Exactly one of OnComplete or OnFailed is called
// exactly once when the search finishes.
func (c *Coordinator) Start(root string, opts Options, cb Callbacks) (string, error) {
	id, err := newSearchID()
	if err != nil {
		return "", fmt.Errorf("search: generate search id: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	c.mu.Lock()
	c.active[id] = cancel
	c.mu.Unlock()

	go func() {
		defer c.forget(id)

		var onProgress func(Progress)
		if cb.OnProgress != nil {
			onProgress = func(p Progress) { cb.OnProgress(id, p) }
		}
		var onMatches func([]Match)
		if cb.OnMatches != nil {
			onMatches = func(batch []Match) { cb.OnMatches(id, batch) }
		}

		result, err := Find(ctx, root, opts, onProgress, onMatches)
		if err != nil {
			if cb.OnFailed != nil {
				cb.OnFailed(id, err)
			}
			return
		}
		if cb.OnComplete != nil {
			cb.OnComplete(id, result)
		}
	}()

	return id, nil
}

// Cancel requests that the given search stop. It returns false if no
// search with that id is currently running.
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

func (c *Coordinator) forget(id string) {
	c.mu.Lock()
	delete(c.active, id)
	c.mu.Unlock()
}

func newSearchID() (string, error) {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}
