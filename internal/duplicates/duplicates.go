package duplicates

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

type fileRef struct {
	path    string
	modTime time.Time
}

type walker struct {
	ctx  context.Context
	pool *fswalk.Pool
	opts Options

	phase        atomic.Pointer[Phase]
	filesScanned atomic.Int64
	filesHashed  atomic.Int64
	currentPath  atomic.Pointer[string]

	sizeGroupsMu sync.Mutex
	sizeGroups   map[int64][]fileRef

	issuesMu sync.Mutex
	issues   []Issue
}

// Find walks the directory tree rooted at root, grouping files with
// identical content: first by size, then a partial-content hash to
// cheaply reject false candidates, then a full hash to confirm — see
// this package's doc comment. It reports throttled progress via
// onProgress (which may be nil) and stops promptly when ctx is
// cancelled, returning a Result with Cancelled set rather than an error.
//
// Unlike internal/search, this cannot stop early once TopN groups are
// found: every file must be seen and hashed before it's known which ones
// are actually duplicated. TopN only bounds how many groups the final
// Result keeps, ranked by reclaimable space.
func Find(ctx context.Context, root string, opts Options, onProgress func(Progress)) (Result, error) {
	started := time.Now()
	opts = opts.withDefaults()

	info, err := os.Lstat(root)
	if err != nil {
		return Result{}, fmt.Errorf("duplicates: stat root %q: %w", root, err)
	}
	if !info.IsDir() {
		return Result{}, fmt.Errorf("duplicates: root %q is not a directory", root)
	}

	w := &walker{
		ctx:        ctx,
		pool:       fswalk.NewPool(opts.Workers),
		opts:       opts,
		sizeGroups: make(map[int64][]fileRef),
	}
	w.setPhase(PhaseScanning)

	stopReporter := w.startReporter(onProgress)

	w.walkDir(root)

	w.setPhase(PhaseHashing)
	groups, totalGroupsFound := w.hashAndGroup()

	stopReporter()

	var totalReclaimable int64
	for _, g := range groups {
		totalReclaimable += g.Reclaimable()
	}

	return Result{
		Root:             root,
		FilesScanned:     w.filesScanned.Load(),
		FilesHashed:      w.filesHashed.Load(),
		Groups:           groups,
		TotalReclaimable: totalReclaimable,
		Truncated:        totalGroupsFound > int64(len(groups)),
		Issues:           w.issuesSnapshot(),
		Cancelled:        ctx.Err() != nil,
		Duration:         time.Since(started),
	}, nil
}

// walkDir is phase 1: collect every file at least MinFileSize into
// sizeGroups, keyed by exact size. Concurrency and cycle safety are the
// same bounded-pool, reparse-aware shape as internal/scanner and
// internal/search.
func (w *walker) walkDir(dir string) {
	if w.ctx.Err() != nil {
		return
	}

	path := dir
	w.currentPath.Store(&path)

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

		kind, err := fswalk.Classify(childPath, entry)
		if err != nil {
			w.recordIssue(childPath, err)
			continue
		}
		if kind == fswalk.KindDirectory {
			w.pool.Go(&wg, func() { w.walkDir(childPath) })
			continue
		}

		info, err := entry.Info()
		if err != nil {
			w.recordIssue(childPath, err)
			continue
		}

		w.filesScanned.Add(1)
		if info.Size() < w.opts.MinFileSize {
			continue
		}

		w.sizeGroupsMu.Lock()
		w.sizeGroups[info.Size()] = append(w.sizeGroups[info.Size()], fileRef{
			path:    childPath,
			modTime: info.ModTime(),
		})
		w.sizeGroupsMu.Unlock()
	}

	wg.Wait()
}

// hashAndGroup is phases 2-4: for each size bucket with more than one
// file, a cheap partial hash narrows candidates before the expensive full
// hash confirms them. totalGroupsFound counts every confirmed duplicate
// group before TopN truncation, so Find can report whether the result is
// incomplete.
func (w *walker) hashAndGroup() (groups []Group, totalGroupsFound int64) {
	collector := topk.New[Group](w.opts.TopN)

	w.sizeGroupsMu.Lock()
	sizeGroups := w.sizeGroups
	w.sizeGroupsMu.Unlock()

	for size, refs := range sizeGroups {
		if w.ctx.Err() != nil {
			break
		}
		if len(refs) < 2 {
			continue
		}

		quickGroups := w.hashRefs(refs, func(path string) (string, error) {
			return quickHash(path, size)
		})

		for _, qrefs := range quickGroups {
			if w.ctx.Err() != nil {
				break
			}
			if len(qrefs) < 2 {
				continue
			}

			fullGroups := w.hashRefs(qrefs, fullHash)

			for hash, frefs := range fullGroups {
				if len(frefs) < 2 {
					continue
				}
				totalGroupsFound++

				files := make([]File, len(frefs))
				for i, r := range frefs {
					files[i] = File{Path: r.path, ModTime: r.modTime}
				}
				collector.Add(Group{Hash: hash, Size: size, Files: files})
			}
		}
	}

	return collector.Sorted(), totalGroupsFound
}

// hashRefs computes hashFn for each ref concurrently (bounded by the same
// pool used for directory traversal) and groups refs by the resulting
// hash.
func (w *walker) hashRefs(refs []fileRef, hashFn func(path string) (string, error)) map[string][]fileRef {
	var mu sync.Mutex
	groups := make(map[string][]fileRef)
	var wg sync.WaitGroup

	for _, ref := range refs {
		if w.ctx.Err() != nil {
			break
		}

		w.pool.Go(&wg, func() {
			if w.ctx.Err() != nil {
				return
			}

			path := ref.path
			w.currentPath.Store(&path)

			hash, err := hashFn(ref.path)
			if err != nil {
				w.recordIssue(ref.path, err)
				return
			}
			w.filesHashed.Add(1)

			mu.Lock()
			groups[hash] = append(groups[hash], ref)
			mu.Unlock()
		})
	}
	wg.Wait()

	return groups
}

func (w *walker) setPhase(p Phase) {
	w.phase.Store(&p)
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
// progress snapshots. The returned stop function blocks until the
// reporter goroutine has exited and performs one final flush, so Find
// never leaves it running after returning.
func (w *walker) startReporter(onProgress func(Progress)) (stop func()) {
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
		onProgress(w.snapshot())
	}
}

func (w *walker) snapshot() Progress {
	var current string
	if p := w.currentPath.Load(); p != nil {
		current = *p
	}
	phase := PhaseScanning
	if p := w.phase.Load(); p != nil {
		phase = *p
	}
	return Progress{
		Phase:        phase,
		FilesScanned: w.filesScanned.Load(),
		FilesHashed:  w.filesHashed.Load(),
		CurrentPath:  current,
	}
}
