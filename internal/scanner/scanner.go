// Package scanner walks a directory tree to compute size statistics:
// total size, largest files, and largest folders. It is a transient,
// in-memory scan (no persistence) — see CLAUDE.md section 40 — designed to
// stay responsive and bounded in memory on trees with millions of files.
//
// Cycle safety: the walker never follows symlinks or Windows junctions. A
// directory entry is only recursed into after platform.IsReparsePoint
// confirms it is a real directory, not a reparse point — os.DirEntry's own
// fs.ModeSymlink bit turned out not to be reliable for this on Windows (an
// NTFS junction is reported as a plain directory by os.ReadDir; see
// internal/platform/reparse_windows.go and the test that caught this).
// Reparse points are recorded as a leaf entry instead of being traversed.
// Since regular filesystem directories cannot form cycles on their own (no
// hardlinks to directories, no junctions once excluded), this makes
// recursive cycles structurally impossible rather than something we have
// to detect at runtime.
package scanner

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
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
	files atomic.Int64
	dirs  atomic.Int64
	bytes atomic.Int64
}

type walker struct {
	ctx  context.Context
	sem  chan struct{}
	opts Options

	counters    counters
	topFiles    *topK[FileEntry]
	topFolders  *topK[FolderEntry]
	currentPath atomic.Pointer[string]

	issuesMu sync.Mutex
	issues   []ScanIssue
}

// Walk scans the directory tree rooted at root, aggregating size
// statistics. It reports throttled progress via onProgress (which may be
// nil) and stops promptly when ctx is cancelled, returning a Result with
// Cancelled set rather than an error — cancellation is an expected
// outcome, not a failure, and any partial totals gathered before
// cancellation are still returned.
func Walk(ctx context.Context, root string, opts Options, onProgress func(Progress)) (Result, error) {
	started := time.Now()
	opts = opts.withDefaults()

	info, err := os.Lstat(root)
	if err != nil {
		return Result{}, fmt.Errorf("scanner: stat root %q: %w", root, err)
	}
	if !info.IsDir() {
		return Result{}, fmt.Errorf("scanner: root %q is not a directory", root)
	}

	w := &walker{
		ctx:        ctx,
		sem:        make(chan struct{}, opts.Workers),
		opts:       opts,
		topFiles:   newTopK[FileEntry](opts.TopN),
		topFolders: newTopK[FolderEntry](opts.TopN),
	}

	stopProgress := w.startProgressReporter(onProgress)
	w.walkDir(root)
	stopProgress()

	return Result{
		Root:       root,
		TotalSize:  w.counters.bytes.Load(),
		TotalFiles: w.counters.files.Load(),
		TotalDirs:  w.counters.dirs.Load(),
		TopFiles:   w.topFiles.Sorted(),
		TopFolders: w.topFolders.Sorted(),
		Issues:     w.issuesSnapshot(),
		Cancelled:  ctx.Err() != nil,
		Duration:   time.Since(started),
	}, nil
}

// walkDir processes one directory and returns its recursive total size
// (its own files plus every subdirectory beneath it). Subdirectories are
// processed concurrently up to the worker pool's capacity; once the pool
// is saturated, further subdirectories are processed synchronously in the
// current goroutine so the number of in-flight goroutines stays bounded
// no matter how wide or deep the tree is.
func (w *walker) walkDir(dir string) int64 {
	if w.ctx.Err() != nil {
		return 0
	}

	path := dir
	w.currentPath.Store(&path)
	w.counters.dirs.Add(1)

	entries, err := os.ReadDir(dir)
	if err != nil {
		w.recordIssue(dir, err)
		return 0
	}

	var localSize int64
	var childSize int64
	var childMu sync.Mutex
	var wg sync.WaitGroup

	for _, entry := range entries {
		if w.ctx.Err() != nil {
			break
		}

		childPath := filepath.Join(dir, entry.Name())

		if entry.IsDir() {
			reparse, err := platform.IsReparsePoint(childPath)
			if err != nil {
				// Could not determine whether this is a real directory or
				// a reparse point; the safe choice is to not traverse it.
				w.recordIssue(childPath, err)
				continue
			}
			if !reparse {
				w.recurse(childPath, &wg, &childMu, &childSize)
				continue
			}
			// A symlink or NTFS junction: record it as a leaf entry below
			// instead of recursing, which is what keeps cycles impossible.
		}

		info, err := entry.Info()
		if err != nil {
			// The file may have been deleted or become inaccessible
			// between ReadDir and this call; that is not fatal to the
			// scan as a whole.
			w.recordIssue(childPath, err)
			continue
		}

		size := info.Size()
		localSize += size
		w.counters.files.Add(1)
		w.counters.bytes.Add(size)
		w.topFiles.Add(FileEntry{
			Path:    childPath,
			Name:    entry.Name(),
			Size:    size,
			ModTime: info.ModTime(),
		})
	}

	wg.Wait()

	total := localSize + childSize
	w.topFolders.Add(FolderEntry{Path: dir, Name: filepath.Base(dir), Size: total})
	return total
}

// recurse walks childPath, running it on a pooled goroutine when the
// worker semaphore has capacity and inline otherwise.
func (w *walker) recurse(childPath string, wg *sync.WaitGroup, childMu *sync.Mutex, childSize *int64) {
	select {
	case w.sem <- struct{}{}:
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() { <-w.sem }()
			size := w.walkDir(childPath)
			childMu.Lock()
			*childSize += size
			childMu.Unlock()
		}()
	default:
		size := w.walkDir(childPath)
		childMu.Lock()
		*childSize += size
		childMu.Unlock()
	}
}

func (w *walker) recordIssue(path string, err error) {
	w.issuesMu.Lock()
	defer w.issuesMu.Unlock()
	w.issues = append(w.issues, ScanIssue{Path: path, Error: err.Error()})
}

func (w *walker) issuesSnapshot() []ScanIssue {
	w.issuesMu.Lock()
	defer w.issuesMu.Unlock()
	out := make([]ScanIssue, len(w.issues))
	copy(out, w.issues)
	return out
}

// startProgressReporter starts a background ticker that delivers throttled
// Progress snapshots to onProgress. The returned stop function blocks
// until the reporter goroutine has exited, so Walk never leaves it
// running after returning.
func (w *walker) startProgressReporter(onProgress func(Progress)) (stop func()) {
	if onProgress == nil {
		return func() {}
	}

	done := make(chan struct{})
	stopped := make(chan struct{})

	go func() {
		defer close(stopped)
		ticker := time.NewTicker(w.opts.ProgressInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				onProgress(w.snapshot())
			case <-done:
				return
			}
		}
	}()

	return func() {
		close(done)
		<-stopped
		onProgress(w.snapshot()) // final snapshot so the UI ends on exact totals
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
		BytesScanned: w.counters.bytes.Load(),
		CurrentPath:  current,
	}
}
