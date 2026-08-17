// Package scanner walks a directory tree to compute size statistics:
// total size, largest files, and largest folders. It is a transient,
// in-memory scan (no persistence) — see CLAUDE.md section 40 — designed to
// stay responsive and bounded in memory on trees with millions of files.
//
// Cycle safety: the walker never follows symlinks or Windows junctions —
// see internal/fswalk.Classify, which is what actually keeps traversal
// cycles structurally impossible (os.DirEntry's own fs.ModeSymlink bit
// turned out not to be reliable for this on Windows; an NTFS junction is
// reported as a plain directory by os.ReadDir, a bug a test caught).
package scanner

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/sonjiwu2/nuvio/internal/fswalk"
	"github.com/sonjiwu2/nuvio/internal/topk"
)

type counters struct {
	files atomic.Int64
	dirs  atomic.Int64
	bytes atomic.Int64
}

type walker struct {
	ctx  context.Context
	pool *fswalk.Pool
	opts Options
	root string

	counters     counters
	topFiles     *topk.Collector[FileEntry]
	topFolders   *topk.Collector[FolderEntry]
	rootChildren *topk.Collector[FolderEntry]
	currentPath  atomic.Pointer[string]

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
		ctx:          ctx,
		pool:         fswalk.NewPool(opts.Workers),
		opts:         opts,
		root:         root,
		topFiles:     topk.New[FileEntry](opts.TopN),
		topFolders:   topk.New[FolderEntry](opts.TopN),
		rootChildren: topk.New[FolderEntry](rootChildrenCap),
	}

	stopProgress := w.startProgressReporter(onProgress)
	w.walkDir(root)
	stopProgress()

	return Result{
		Root:         root,
		TotalSize:    w.counters.bytes.Load(),
		TotalFiles:   w.counters.files.Load(),
		TotalDirs:    w.counters.dirs.Load(),
		TopFiles:     w.topFiles.Sorted(),
		TopFolders:   w.topFolders.Sorted(),
		RootChildren: w.rootChildren.Sorted(),
		Issues:       w.issuesSnapshot(),
		Cancelled:    ctx.Err() != nil,
		Duration:     time.Since(started),
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

		kind, err := fswalk.Classify(childPath, entry)
		if err != nil {
			// Could not determine whether this is a real directory or a
			// reparse point; the safe choice is to not traverse it.
			w.recordIssue(childPath, err)
			continue
		}
		if kind == fswalk.KindDirectory {
			w.recurse(dir, childPath, &wg, &childMu, &childSize)
			continue
		}
		// A file, or a symlink/junction: record it as a leaf entry below
		// instead of recursing, which is what keeps cycles impossible.

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

	if dir == w.root && localSize > 0 {
		// Files sitting directly in the scanned root, not inside any
		// subdirectory, get their own treemap slice instead of silently
		// vanishing from the Storage Overview.
		w.rootChildren.Add(FolderEntry{Path: dir, Name: "Other files", Size: localSize})
	}

	total := localSize + childSize
	if dir != w.root {
		// The root itself is excluded: it would always be the single
		// largest "folder" by definition (100% of the total), which tells
		// the user nothing they don't already see in the summary metrics.
		w.topFolders.Add(FolderEntry{Path: dir, Name: filepath.Base(dir), Size: total})
	}
	return total
}

// recurse walks childPath, running it on a pooled goroutine when the
// worker semaphore has capacity and inline otherwise. parentDir is used
// only to detect direct children of the scanned root for the Storage
// Overview treemap.
func (w *walker) recurse(parentDir, childPath string, wg *sync.WaitGroup, childMu *sync.Mutex, childSize *int64) {
	w.pool.Go(wg, func() {
		size := w.walkDir(childPath)
		childMu.Lock()
		*childSize += size
		childMu.Unlock()
		if parentDir == w.root {
			w.rootChildren.Add(FolderEntry{Path: childPath, Name: filepath.Base(childPath), Size: size})
		}
	})
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
