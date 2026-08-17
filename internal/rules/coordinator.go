package rules

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
)

// Coordinator runs and tracks previews by id, mirroring
// internal/search.Coordinator so the Wails bridge can start a preview,
// receive progress/entry/completion callbacks, and cancel a specific
// preview without affecting any other one running at the same time.
type Coordinator struct {
	mu     sync.Mutex
	active map[string]context.CancelFunc
}

// NewCoordinator creates an empty Coordinator.
func NewCoordinator() *Coordinator {
	return &Coordinator{active: make(map[string]context.CancelFunc)}
}

// PreviewCallbacks receives lifecycle events for one preview, each given
// the preview's id since a caller may be tracking several concurrently.
type PreviewCallbacks struct {
	OnProgress func(id string, p PreviewProgress)
	OnEntries  func(id string, batch []PreviewEntry)
	OnComplete func(id string, r PreviewResult)
	OnFailed   func(id string, err error)
}

// Start begins previewing root against activeRules on a new goroutine and
// returns immediately with a preview id. Exactly one of OnComplete or
// OnFailed is called exactly once when the preview finishes.
func (c *Coordinator) Start(root string, activeRules []Rule, opts PreviewOptions, cb PreviewCallbacks) (string, error) {
	id, err := newPreviewID()
	if err != nil {
		return "", fmt.Errorf("rules: generate preview id: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	c.mu.Lock()
	c.active[id] = cancel
	c.mu.Unlock()

	go func() {
		defer c.forget(id)

		var onProgress func(PreviewProgress)
		if cb.OnProgress != nil {
			onProgress = func(p PreviewProgress) { cb.OnProgress(id, p) }
		}
		var onEntries func([]PreviewEntry)
		if cb.OnEntries != nil {
			onEntries = func(batch []PreviewEntry) { cb.OnEntries(id, batch) }
		}

		result, err := Preview(ctx, root, activeRules, opts, onProgress, onEntries)
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

// Cancel requests that the given preview stop. It returns false if no
// preview with that id is currently running.
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

func newPreviewID() (string, error) {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}
