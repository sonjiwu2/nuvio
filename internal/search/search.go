// Package search performs a live, cancellable filename search across a
// directory tree. It deliberately mirrors internal/scanner's approach to
// bounded concurrency and cycle safety (bounded worker pool via a
// semaphore, reparse points treated as leaves via platform.IsReparsePoint)
// rather than sharing code with it — the two walkers' data flow differs
// enough (post-order size rollup vs. streamed match collection) that a
// shared generic abstraction was not worth the risk of destabilizing
// scanner's already-tested traversal. If a third walker-shaped feature
// shows up, that is the point to extract a shared primitive.
package search

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/sonjiwu2/nuvio/internal/platform"
)

func defaultWorkerCount() int {
	n := runtime.NumCPU()
	if n < 2 {
		n = 2
	}
	return n
}

type counters struct {
	files   atomic.Int64
	dirs    atomic.Int64
	matches atomic.Int64
}

type walker struct {
	ctx    context.Context
	cancel context.CancelFunc
	sem    chan struct{}
	opts   Options
	query  string

	counters    counters
	currentPath atomic.Pointer[string]

	matchesMu  sync.Mutex
	matches    []Match
	flushedIdx int
	truncated  bool

	issuesMu sync.Mutex
	issues   []Issue
}

// Find walks the directory tree rooted at root, collecting files whose
// name contains opts.Query (case-insensitive substring match). It reports
// throttled progress and newly found matches via onProgress/onMatches
// (either may be nil, batched together on the same interval) and stops
// promptly when ctx is cancelled or the match cap is reached — both are
// reported as a normal Result rather than an error; only Result.Cancelled
// distinguishes user cancellation from hitting the cap (Result.Truncated).
func Find(
	ctx context.Context,
	root string,
	opts Options,
	onProgress func(Progress),
	onMatches func([]Match),
) (Result, error) {
	started := time.Now()
	opts = opts.withDefaults()

	if strings.TrimSpace(opts.Query) == "" {
		return Result{}, fmt.Errorf("search: query must not be empty")
	}

	info, err := os.Lstat(root)
	if err != nil {
		return Result{}, fmt.Errorf("search: stat root %q: %w", root, err)
	}
	if !info.IsDir() {
		return Result{}, fmt.Errorf("search: root %q is not a directory", root)
	}

	// internalCtx lets the walker stop itself once MaxMatches is reached,
	// without that self-stop being reported as if the user cancelled.
	internalCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	w := &walker{
		ctx:    internalCtx,
		cancel: cancel,
		sem:    make(chan struct{}, opts.Workers),
		opts:   opts,
		query:  strings.ToLower(opts.Query),
	}

	stopReporter := w.startReporter(onProgress, onMatches)
	w.walkDir(root)
	stopReporter()

	return Result{
		Root:         root,
		Query:        opts.Query,
		FilesScanned: w.counters.files.Load(),
		DirsScanned:  w.counters.dirs.Load(),
		MatchCount:   w.counters.matches.Load(),
		Truncated:    w.isTruncated(),
		Issues:       w.issuesSnapshot(),
		Cancelled:    ctx.Err() != nil,
		Duration:     time.Since(started),
	}, nil
}

func (w *walker) walkDir(dir string) {
	if w.ctx.Err() != nil {
		return
	}

	path := dir
	w.currentPath.Store(&path)
	w.counters.dirs.Add(1)

	entries, err := os.ReadDir(dir)
	if err != nil {
		w.recordIssue(dir, err)
		return
	}

	var wg sync.WaitGroup

	for _, entry := range entries {
		if w.ctx.Err() != nil {
			break
		}

		childPath := filepath.Join(dir, entry.Name())

		if entry.IsDir() {
			reparse, err := platform.IsReparsePoint(childPath)
			if err != nil {
				w.recordIssue(childPath, err)
				continue
			}
			if !reparse {
				w.recurse(childPath, &wg)
				continue
			}
			// A symlink or NTFS junction: fall through and treat it as a
			// leaf below, same policy as internal/scanner.
		}

		info, err := entry.Info()
		if err != nil {
			w.recordIssue(childPath, err)
			continue
		}

		w.counters.files.Add(1)
		if strings.Contains(strings.ToLower(entry.Name()), w.query) {
			w.addMatch(Match{
				Path:    childPath,
				Name:    entry.Name(),
				Size:    info.Size(),
				ModTime: info.ModTime(),
			})
		}
	}

	wg.Wait()
}

func (w *walker) recurse(childPath string, wg *sync.WaitGroup) {
	select {
	case w.sem <- struct{}{}:
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() { <-w.sem }()
			w.walkDir(childPath)
		}()
	default:
		w.walkDir(childPath)
	}
}

func (w *walker) addMatch(m Match) {
	w.matchesMu.Lock()
	defer w.matchesMu.Unlock()

	if len(w.matches) >= w.opts.MaxMatches {
		if !w.truncated {
			w.truncated = true
			w.cancel() // stop promptly: scanning further just to prove there are more matches isn't useful
		}
		return
	}

	w.matches = append(w.matches, m)
	w.counters.matches.Add(1)
}

func (w *walker) isTruncated() bool {
	w.matchesMu.Lock()
	defer w.matchesMu.Unlock()
	return w.truncated
}

// drainMatches returns matches collected since the last drain, so the
// periodic reporter only ever sends each match once.
func (w *walker) drainMatches() []Match {
	w.matchesMu.Lock()
	defer w.matchesMu.Unlock()

	if w.flushedIdx >= len(w.matches) {
		return nil
	}
	batch := make([]Match, len(w.matches)-w.flushedIdx)
	copy(batch, w.matches[w.flushedIdx:])
	w.flushedIdx = len(w.matches)
	return batch
}

func (w *walker) recordIssue(path string, err error) {
	w.issuesMu.Lock()
	defer w.issuesMu.Unlock()
	w.issues = append(w.issues, Issue{Path: path, Error: err.Error()})
}

func (w *walker) issuesSnapshot() []Issue {
	w.issuesMu.Lock()
	defer w.issuesMu.Unlock()
	out := make([]Issue, len(w.issues))
	copy(out, w.issues)
	return out
}

// startReporter starts a background ticker that delivers throttled
// progress snapshots and newly found match batches. The returned stop
// function blocks until the reporter goroutine has exited and performs
// one final flush, so Find never leaves it running or drops a trailing
// batch after returning.
func (w *walker) startReporter(onProgress func(Progress), onMatches func([]Match)) (stop func()) {
	if onProgress == nil && onMatches == nil {
		return func() {}
	}

	done := make(chan struct{})
	stopped := make(chan struct{})

	go func() {
		defer close(stopped)
		ticker := time.NewTicker(w.opts.BatchInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				w.flush(onProgress, onMatches)
			case <-done:
				return
			}
		}
	}()

	return func() {
		close(done)
		<-stopped
		w.flush(onProgress, onMatches)
	}
}

func (w *walker) flush(onProgress func(Progress), onMatches func([]Match)) {
	if onProgress != nil {
		onProgress(w.snapshot())
	}
	if onMatches != nil {
		if batch := w.drainMatches(); len(batch) > 0 {
			onMatches(batch)
		}
	}
}

func (w *walker) snapshot() Progress {
	var current string
	if p := w.currentPath.Load(); p != nil {
		current = *p
	}
	return Progress{
		FilesScanned: w.counters.files.Load(),
		DirsScanned:  w.counters.dirs.Load(),
		MatchesFound: w.counters.matches.Load(),
		CurrentPath:  current,
	}
}
