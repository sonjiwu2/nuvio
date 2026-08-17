package scanner

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// writeFile creates a file with the given size (in bytes, content is
// irrelevant to the scanner) under a fresh temp-dir fixture.
func writeFile(t *testing.T, path string, size int64) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir for %s: %v", path, err)
	}
	if err := os.WriteFile(path, make([]byte, size), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func TestWalk_AggregatesSizesAcrossNestedDirectories(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "docs", "a.pdf"), 100)
	writeFile(t, filepath.Join(root, "docs", "b.pdf"), 200)
	writeFile(t, filepath.Join(root, "images", "img.png"), 300)
	writeFile(t, filepath.Join(root, "nested", "deep", "file.bin"), 400)

	result, err := Walk(context.Background(), root, Options{}, nil)
	if err != nil {
		t.Fatalf("Walk returned error: %v", err)
	}

	if result.TotalSize != 1000 {
		t.Errorf("TotalSize = %d, want 1000", result.TotalSize)
	}
	if result.TotalFiles != 4 {
		t.Errorf("TotalFiles = %d, want 4", result.TotalFiles)
	}
	// root, docs, images, nested, nested/deep
	if result.TotalDirs != 5 {
		t.Errorf("TotalDirs = %d, want 5", result.TotalDirs)
	}
	if result.Cancelled {
		t.Error("Cancelled = true, want false")
	}
	if len(result.Issues) != 0 {
		t.Errorf("Issues = %v, want none", result.Issues)
	}
}

func TestWalk_TopFilesRanksLargestFirst(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "small.bin"), 10)
	writeFile(t, filepath.Join(root, "medium.bin"), 50)
	writeFile(t, filepath.Join(root, "large.bin"), 100)

	result, err := Walk(context.Background(), root, Options{TopN: 2}, nil)
	if err != nil {
		t.Fatalf("Walk returned error: %v", err)
	}

	if len(result.TopFiles) != 2 {
		t.Fatalf("len(TopFiles) = %d, want 2", len(result.TopFiles))
	}
	if result.TopFiles[0].Name != "large.bin" || result.TopFiles[1].Name != "medium.bin" {
		t.Errorf("TopFiles = %+v, want [large.bin medium.bin]", result.TopFiles)
	}
}

func TestWalk_TopFoldersIncludesRecursiveTotals(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "big", "a.bin"), 100)
	writeFile(t, filepath.Join(root, "big", "sub", "b.bin"), 100)
	writeFile(t, filepath.Join(root, "small", "c.bin"), 10)

	result, err := Walk(context.Background(), root, Options{}, nil)
	if err != nil {
		t.Fatalf("Walk returned error: %v", err)
	}

	var bigTotal, subTotal int64 = -1, -1
	for _, f := range result.TopFolders {
		switch f.Path {
		case filepath.Join(root, "big"):
			bigTotal = f.Size
		case filepath.Join(root, "big", "sub"):
			subTotal = f.Size
		}
	}

	if bigTotal != 200 {
		t.Errorf("big folder size = %d, want 200 (own file + nested sub)", bigTotal)
	}
	if subTotal != 100 {
		t.Errorf("big/sub folder size = %d, want 100", subTotal)
	}
}

func TestWalk_UnreadableDirectoryIsReportedNotFatal(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("chmod-based permission denial is not reliable on Windows")
	}

	root := t.TempDir()
	writeFile(t, filepath.Join(root, "ok", "a.bin"), 10)

	blocked := filepath.Join(root, "blocked")
	if err := os.MkdirAll(blocked, 0o755); err != nil {
		t.Fatalf("mkdir blocked: %v", err)
	}
	writeFile(t, filepath.Join(blocked, "hidden.bin"), 999)
	if err := os.Chmod(blocked, 0o000); err != nil {
		t.Fatalf("chmod blocked: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(blocked, 0o755) })

	result, err := Walk(context.Background(), root, Options{}, nil)
	if err != nil {
		t.Fatalf("Walk returned error: %v", err)
	}

	if result.TotalSize != 10 {
		t.Errorf("TotalSize = %d, want 10 (blocked dir excluded)", result.TotalSize)
	}
	if len(result.Issues) != 1 {
		t.Fatalf("len(Issues) = %d, want 1", len(result.Issues))
	}
	if result.Issues[0].Path != blocked {
		t.Errorf("Issues[0].Path = %q, want %q", result.Issues[0].Path, blocked)
	}
}

func TestWalk_MissingRootReturnsError(t *testing.T) {
	_, err := Walk(context.Background(), filepath.Join(t.TempDir(), "does-not-exist"), Options{}, nil)
	if err == nil {
		t.Fatal("expected an error for a missing root, got nil")
	}
}

func TestWalk_FileRootReturnsError(t *testing.T) {
	root := t.TempDir()
	filePath := filepath.Join(root, "file.txt")
	writeFile(t, filePath, 10)

	_, err := Walk(context.Background(), filePath, Options{}, nil)
	if err == nil {
		t.Fatal("expected an error when root is a file, got nil")
	}
}

func TestWalk_DoesNotFollowSymlinkedDirectories(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "real", "a.bin"), 100)

	target := filepath.Join(root, "real")
	link := filepath.Join(root, "link")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlink creation not permitted in this environment: %v", err)
	}

	result, err := Walk(context.Background(), root, Options{}, nil)
	if err != nil {
		t.Fatalf("Walk returned error: %v", err)
	}

	assertRealFileSeenExactlyOnce(t, result, "a.bin")
	// The symlink itself is a directory entry Nuvio genuinely visited, but
	// it must never have been traversed as if it were "real".
	if result.TotalDirs != 2 {
		t.Errorf("TotalDirs = %d, want 2 (root + real; the symlink must not add a third)", result.TotalDirs)
	}
}

// TestWalk_DoesNotFollowJunctions covers the Windows-specific cycle risk
// directly: NTFS junctions (unlike symlinks) can be created without
// elevated privileges or Developer Mode, so this is the reparse-point
// path most Nuvio users could actually hit.
func TestWalk_DoesNotFollowJunctions(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("junctions are a Windows-specific reparse point type")
	}

	root := t.TempDir()
	writeFile(t, filepath.Join(root, "real", "a.bin"), 100)

	target := filepath.Join(root, "real")
	link := filepath.Join(root, "link")

	cmd := exec.Command("cmd", "/c", "mklink", "/J", link, target)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Skipf("junction creation not available in this environment: %v (%s)", err, out)
	}

	result, err := Walk(context.Background(), root, Options{}, nil)
	if err != nil {
		t.Fatalf("Walk returned error: %v", err)
	}

	assertRealFileSeenExactlyOnce(t, result, "a.bin")
	if result.TotalDirs != 2 {
		t.Errorf("TotalDirs = %d, want 2 (root + real; the junction must not add a third)", result.TotalDirs)
	}
}

// assertRealFileSeenExactlyOnce fails the test if name appears zero or
// more-than-once among the scan's largest files — the signature of a
// reparse point having been traversed and the same file double-counted.
func assertRealFileSeenExactlyOnce(t *testing.T, result Result, name string) {
	t.Helper()
	var matches []FileEntry
	for _, f := range result.TopFiles {
		if f.Name == name {
			matches = append(matches, f)
		}
	}
	if len(matches) != 1 {
		t.Errorf("found %d file(s) named %q in TopFiles, want exactly 1: %+v", len(matches), name, matches)
	}
}

func TestWalk_CancellationStopsPromptlyAndMarksResult(t *testing.T) {
	root := t.TempDir()
	for i := 0; i < 50; i++ {
		writeFile(t, filepath.Join(root, "d"+string(rune('a'+i%26)), "f.bin"), 10)
	}

	ctx, cancel := context.WithCancel(context.Background())

	var progressCount atomic.Int32
	var once sync.Once
	done := make(chan struct{})

	onProgress := func(_ Progress) {
		progressCount.Add(1)
		once.Do(func() {
			cancel()
			close(done)
		})
	}

	result, err := Walk(ctx, root, Options{Workers: 1, ProgressInterval: time.Millisecond}, onProgress)
	if err != nil {
		t.Fatalf("Walk returned error: %v", err)
	}

	select {
	case <-done:
	default:
		t.Fatal("progress callback (and therefore cancel) was never invoked")
	}

	if !result.Cancelled {
		t.Error("Cancelled = false, want true")
	}
}

func TestWalk_NoGoroutineLeakAfterCompletion(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "a.bin"), 10)

	before := runtime.NumGoroutine()

	for i := 0; i < 5; i++ {
		if _, err := Walk(context.Background(), root, Options{}, func(Progress) {}); err != nil {
			t.Fatalf("Walk returned error: %v", err)
		}
	}

	// Give any straggler goroutines a moment to actually exit before we
	// sample again — GC/scheduler timing, not a fixed sleep-based retry.
	deadline := time.Now().Add(time.Second)
	for runtime.NumGoroutine() > before && time.Now().Before(deadline) {
		runtime.Gosched()
	}

	after := runtime.NumGoroutine()
	if after > before {
		t.Errorf("goroutine count grew from %d to %d after repeated scans", before, after)
	}
}

func TestCoordinator_StartAndCancel(t *testing.T) {
	root := t.TempDir()
	for i := 0; i < 20; i++ {
		writeFile(t, filepath.Join(root, "d"+string(rune('a'+i)), "f.bin"), 10)
	}

	c := NewCoordinator()

	var progressCount atomic.Int32
	completed := make(chan Result, 1)
	failed := make(chan error, 1)

	id, err := c.Start(root, Options{Workers: 1, ProgressInterval: time.Millisecond}, Callbacks{
		OnProgress: func(_ string, _ Progress) { progressCount.Add(1) },
		OnComplete: func(_ string, r Result) { completed <- r },
		OnFailed:   func(_ string, e error) { failed <- e },
	})
	if err != nil {
		t.Fatalf("Start returned error: %v", err)
	}
	if id == "" {
		t.Fatal("Start returned empty scan id")
	}

	if !c.Cancel(id) {
		t.Fatal("Cancel returned false for a scan that should still be running or just finished")
	}

	select {
	case r := <-completed:
		_ = r // cancellation racing completion is fine; either outcome is valid
	case err := <-failed:
		t.Fatalf("scan failed: %v", err)
	case <-time.After(5 * time.Second):
		t.Fatal("scan did not complete after cancellation")
	}

	if c.Cancel("unknown-id") {
		t.Error("Cancel returned true for an unknown scan id")
	}
}

func TestCoordinator_UnknownRootReportsFailure(t *testing.T) {
	c := NewCoordinator()
	failed := make(chan error, 1)

	_, err := c.Start(filepath.Join(t.TempDir(), "missing"), Options{}, Callbacks{
		OnFailed: func(_ string, e error) { failed <- e },
	})
	if err != nil {
		t.Fatalf("Start returned error: %v", err)
	}

	select {
	case err := <-failed:
		if err == nil {
			t.Error("OnFailed called with nil error")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("OnFailed was never called for a missing root")
	}
}

func TestTopK_KeepsOnlyLargestUpToCapacity(t *testing.T) {
	k := newTopK[FileEntry](3)
	sizes := []int64{5, 1, 9, 3, 7, 2, 8}
	for i, s := range sizes {
		k.Add(FileEntry{Name: string(rune('a' + i)), Size: s})
	}

	got := k.Sorted()
	if len(got) != 3 {
		t.Fatalf("len(Sorted()) = %d, want 3", len(got))
	}
	want := []int64{9, 8, 7}
	for i, w := range want {
		if got[i].Size != w {
			t.Errorf("Sorted()[%d].Size = %d, want %d", i, got[i].Size, w)
		}
	}
}

func TestTopK_ConcurrentAddIsRaceFree(t *testing.T) {
	k := newTopK[FileEntry](10)
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			k.Add(FileEntry{Name: "f", Size: int64(n)})
		}(i)
	}
	wg.Wait()

	if len(k.Sorted()) != 10 {
		t.Errorf("len(Sorted()) = %d, want 10", len(k.Sorted()))
	}
}
