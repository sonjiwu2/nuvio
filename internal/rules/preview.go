package rules

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/sonjiwu2/nuvio/internal/fswalk"
)

type counters struct {
	files   atomic.Int64
	dirs    atomic.Int64
	matches atomic.Int64
}

type previewWalker struct {
	ctx    context.Context
	cancel context.CancelFunc
	pool   *fswalk.Pool
	opts   PreviewOptions
	byExt  map[string]Rule

	counters    counters
	totalSize   atomic.Int64
	currentPath atomic.Pointer[string]

	entriesMu  sync.Mutex
	entries    []PreviewEntry
	flushedIdx int
	truncated  bool

	issuesMu sync.Mutex
	issues   []PreviewIssue
}

// Preview walks the directory tree rooted at root, reporting — never
// performing — what each of activeRules would do to the files it finds.
// It reports throttled progress and newly matched entries via
// onProgress/onEntries (either may be nil, batched together on the same
// interval) and stops promptly when ctx is cancelled or the entry cap is
// reached, the same way internal/search.Find does.
func Preview(
	ctx context.Context,
	root string,
	activeRules []Rule,
	opts PreviewOptions,
	onProgress func(PreviewProgress),
	onEntries func([]PreviewEntry),
) (PreviewResult, error) {
	started := time.Now()
	opts = opts.withDefaults()

	if len(activeRules) == 0 {
		return PreviewResult{}, fmt.Errorf("rules: no rules to preview")
	}

	info, err := os.Lstat(root)
	if err != nil {
		return PreviewResult{}, fmt.Errorf("rules: stat root %q: %w", root, err)
	}
	if !info.IsDir() {
		return PreviewResult{}, fmt.Errorf("rules: root %q is not a directory", root)
	}

	byExt := make(map[string]Rule, len(activeRules))
	for _, r := range activeRules {
		byExt[r.Extension] = r
	}

	internalCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	w := &previewWalker{
		ctx:    internalCtx,
		cancel: cancel,
		pool:   fswalk.NewPool(opts.Workers),
		opts:   opts,
		byExt:  byExt,
	}

	stopReporter := w.startReporter(onProgress, onEntries)
	w.walkDir(root)
	stopReporter()

	return PreviewResult{
		Root:         root,
		FilesScanned: w.counters.files.Load(),
		DirsScanned:  w.counters.dirs.Load(),
		MatchCount:   w.counters.matches.Load(),
		TotalSize:    w.totalSize.Load(),
		Truncated:    w.isTruncated(),
		Issues:       w.issuesSnapshot(),
		Cancelled:    ctx.Err() != nil,
		Duration:     time.Since(started),
	}, nil
}

func (w *previewWalker) walkDir(dir string) {
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

		kind, err := fswalk.Classify(childPath, entry)
		if err != nil {
			w.recordIssue(childPath, err)
			continue
		}
		if kind == fswalk.KindDirectory {
			w.recurse(childPath, &wg)
			continue
		}

		info, err := entry.Info()
		if err != nil {
			w.recordIssue(childPath, err)
			continue
		}

		w.counters.files.Add(1)

		ext := normalizeExtension(filepath.Ext(entry.Name()))
		if rule, ok := w.byExt[ext]; ok {
			w.addEntry(PreviewEntry{
				SourcePath:      childPath,
				Name:            entry.Name(),
				Size:            info.Size(),
				DestinationPath: filepath.Join(rule.DestinationFolder, entry.Name()),
				RuleID:          rule.ID,
			})
		}
	}

	wg.Wait()
}

func (w *previewWalker) recurse(childPath string, wg *sync.WaitGroup) {
	w.pool.Go(wg, func() {
		w.walkDir(childPath)
	})
}

func (w *previewWalker) addEntry(e PreviewEntry) {
	w.entriesMu.Lock()
	defer w.entriesMu.Unlock()

	if len(w.entries) >= w.opts.MaxEntries {
		if !w.truncated {
			w.truncated = true
			w.cancel()
		}
		return
	}

	w.entries = append(w.entries, e)
	w.counters.matches.Add(1)
	w.totalSize.Add(e.Size)
}

func (w *previewWalker) isTruncated() bool {
	w.entriesMu.Lock()
	defer w.entriesMu.Unlock()
	return w.truncated
}

func (w *previewWalker) drainEntries() []PreviewEntry {
	w.entriesMu.Lock()
	defer w.entriesMu.Unlock()

	if w.flushedIdx >= len(w.entries) {
		return nil
	}
	batch := make([]PreviewEntry, len(w.entries)-w.flushedIdx)
	copy(batch, w.entries[w.flushedIdx:])
	w.flushedIdx = len(w.entries)
	return batch
}

func (w *previewWalker) recordIssue(path string, err error) {
	w.issuesMu.Lock()
	defer w.issuesMu.Unlock()
	w.issues = append(w.issues, PreviewIssue{Path: path, Error: err.Error()})
}

func (w *previewWalker) issuesSnapshot() []PreviewIssue {
	w.issuesMu.Lock()
	defer w.issuesMu.Unlock()
	out := make([]PreviewIssue, len(w.issues))
	copy(out, w.issues)
	return out
}

func (w *previewWalker) startReporter(onProgress func(PreviewProgress), onEntries func([]PreviewEntry)) (stop func()) {
	if onProgress == nil && onEntries == nil {
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
				w.flush(onProgress, onEntries)
			case <-done:
				return
			}
		}
	}()

	return func() {
		close(done)
		<-stopped
		w.flush(onProgress, onEntries)
	}
}

func (w *previewWalker) flush(onProgress func(PreviewProgress), onEntries func([]PreviewEntry)) {
	if onProgress != nil {
		onProgress(w.snapshot())
	}
	if onEntries != nil {
		if batch := w.drainEntries(); len(batch) > 0 {
			onEntries(batch)
		}
	}
}

func (w *previewWalker) snapshot() PreviewProgress {
	var current string
	if p := w.currentPath.Load(); p != nil {
		current = *p
	}
	return PreviewProgress{
		FilesScanned: w.counters.files.Load(),
		DirsScanned:  w.counters.dirs.Load(),
		MatchesFound: w.counters.matches.Load(),
		CurrentPath:  current,
	}
}
