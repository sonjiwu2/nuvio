package scanner

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
)

// Coordinator runs and tracks scans by id so a caller (the Wails bridge)
// can start a scan, receive progress/completion callbacks, and cancel a
// specific scan without affecting any other scan that happens to be
// running at the same time.
type Coordinator struct {
	mu     sync.Mutex
	active map[string]context.CancelFunc
}

// NewCoordinator creates an empty Coordinator.
func NewCoordinator() *Coordinator {
	return &Coordinator{active: make(map[string]context.CancelFunc)}
}

// Callbacks receives lifecycle events for one scan. Every callback is
// given the scan's id so a caller managing several concurrent scans (as
// the Wails bridge does, one per browser-side request) can tell them
// apart without closing over state that doesn't exist yet when Start is
// called.
type Callbacks struct {
	OnProgress func(id string, p Progress)
	OnComplete func(id string, r Result)
	OnFailed   func(id string, err error)
}

// Start begins scanning root on a new goroutine and returns immediately
// with a scan id. Exactly one of OnComplete or OnFailed is called exactly
// once when the scan finishes; OnProgress may be called any number of
// times before that.
func (c *Coordinator) Start(root string, opts Options, cb Callbacks) (string, error) {
	id, err := newScanID()
	if err != nil {
		return "", fmt.Errorf("scanner: generate scan id: %w", err)
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

		result, err := Walk(ctx, root, opts, onProgress)
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

// Cancel requests that the given scan stop. It returns false if no scan
// with that id is currently running (already finished, or unknown id).
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

func newScanID() (string, error) {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}
