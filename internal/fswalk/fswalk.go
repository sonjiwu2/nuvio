// Package fswalk holds the small pieces of directory-walking logic that
// are safety-critical and shared by every feature that recursively walks
// the filesystem (internal/scanner, internal/search, internal/rules):
// classifying an entry as a real directory vs. a reparse point, and
// bounding recursive concurrency. It intentionally does NOT provide a
// generic "Walker" type — each caller's data flow (size rollup, streamed
// matches, rule evaluation) differs enough that forcing them through one
// generic abstraction would cost more in complexity than the remaining
// duplication (a ReadDir loop shaped to each caller's own aggregation)
// actually costs.
package fswalk

import (
	"os"
	"runtime"
	"sync"

	"github.com/sonjiwu2/nuvio/internal/platform"
)

// EntryKind reports how a walker should treat a directory entry.
type EntryKind int

const (
	// KindFile is a regular file, or a directory entry that turned out to
	// be a symlink/junction and must be treated as a leaf, not traversed.
	KindFile EntryKind = iota
	// KindDirectory is a real, safe-to-recurse-into directory.
	KindDirectory
)

// Classify reports how a walker should treat entry at path. This is the
// one place that decides whether an entry gets recursed into — see
// internal/platform.IsReparsePoint's doc comment for why that check, not
// entry.IsDir() alone, is what actually keeps traversal cycles impossible
// (an NTFS junction is reported as a plain directory by os.ReadDir, which
// caused a real infinite-recursion bug before this check existed).
//
// On error, the caller should record it as an issue and not recurse —
// KindFile is returned alongside the error as the safe default.
func Classify(path string, entry os.DirEntry) (EntryKind, error) {
	if !entry.IsDir() {
		return KindFile, nil
	}

	reparse, err := platform.IsReparsePoint(path)
	if err != nil {
		return KindFile, err
	}
	if reparse {
		return KindFile, nil
	}
	return KindDirectory, nil
}

// DefaultWorkerCount returns a sensible default for bounded filesystem
// concurrency: proportional to CPU count, but never less than 2 so a
// single-core CI runner still gets some overlap between I/O waits.
func DefaultWorkerCount() int {
	n := runtime.NumCPU()
	if n < 2 {
		n = 2
	}
	return n
}

// Pool bounds concurrent recursive directory walks. Go runs fn on a
// pooled goroutine when capacity allows, or synchronously inline
// otherwise, so the number of goroutines *spawned* through the pool never
// exceeds its capacity no matter how wide or deep the tree being walked
// is. This is not quite the same as bounding total concurrent execution
// to workers: when the pool is saturated, the calling goroutine keeps
// running inline rather than blocking, so at most one extra thread of
// control (whichever goroutine is currently recursing when saturation
// hits) can be active alongside the full pool — a worst case of
// workers+1, not workers. That single extra goroutine doing useful work
// instead of blocking is the whole point of the inline fallback.
type Pool struct {
	sem chan struct{}
}

// NewPool creates a Pool allowing up to workers concurrent recursive
// calls.
func NewPool(workers int) *Pool {
	return &Pool{sem: make(chan struct{}, workers)}
}

// Go runs fn, either on a pooled goroutine (registered on wg, so the
// caller's wg.Wait() covers it) or inline on the calling goroutine if the
// pool is currently saturated.
func (p *Pool) Go(wg *sync.WaitGroup, fn func()) {
	select {
	case p.sem <- struct{}{}:
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() { <-p.sem }()
			fn()
		}()
	default:
		fn()
	}
}
