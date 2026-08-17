package duplicates

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"
)

func writeContent(t *testing.T, path string, content []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir for %s: %v", path, err)
	}
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func writeFile(t *testing.T, path string, size int64) {
	t.Helper()
	writeContent(t, path, make([]byte, size))
}

func TestFind_GroupsIdenticalFilesTogetherAndExcludesUniqueOnes(t *testing.T) {
	root := t.TempDir()
	content := bytes.Repeat([]byte("duplicate-content-"), 10)
	writeContent(t, filepath.Join(root, "a.txt"), content)
	writeContent(t, filepath.Join(root, "dir1", "b.txt"), content)
	writeContent(t, filepath.Join(root, "unique.txt"), []byte("something else entirely, not duplicated anywhere"))

	result, err := Find(context.Background(), root, Options{MinFileSize: 1}, nil)
	if err != nil {
		t.Fatalf("Find returned error: %v", err)
	}

	if len(result.Groups) != 1 {
		t.Fatalf("len(Groups) = %d, want 1", len(result.Groups))
	}
	group := result.Groups[0]
	if len(group.Files) != 2 {
		t.Errorf("len(group.Files) = %d, want 2", len(group.Files))
	}
	if group.Size != int64(len(content)) {
		t.Errorf("group.Size = %d, want %d", group.Size, len(content))
	}
	if want := int64(len(content)); group.Reclaimable() != want {
		t.Errorf("Reclaimable() = %d, want %d", group.Reclaimable(), want)
	}
}

func TestFind_DoesNotGroupDifferentContentOfTheSameSize(t *testing.T) {
	root := t.TempDir()
	writeContent(t, filepath.Join(root, "a.txt"), bytes.Repeat([]byte("A"), 5000))
	writeContent(t, filepath.Join(root, "b.txt"), bytes.Repeat([]byte("B"), 5000))

	result, err := Find(context.Background(), root, Options{MinFileSize: 1}, nil)
	if err != nil {
		t.Fatalf("Find returned error: %v", err)
	}

	if len(result.Groups) != 0 {
		t.Errorf("Groups = %+v, want none (same size, different content must not be grouped)", result.Groups)
	}
}

func TestFind_SkipsFilesBelowMinFileSize(t *testing.T) {
	root := t.TempDir()
	writeContent(t, filepath.Join(root, "a.txt"), []byte("hi"))
	writeContent(t, filepath.Join(root, "b.txt"), []byte("hi"))

	result, err := Find(context.Background(), root, Options{MinFileSize: 10}, nil)
	if err != nil {
		t.Fatalf("Find returned error: %v", err)
	}

	if len(result.Groups) != 0 {
		t.Errorf("Groups = %+v, want none (both files are below MinFileSize)", result.Groups)
	}
}

func TestFind_TopNLimitsGroupsAndMarksTruncated(t *testing.T) {
	root := t.TempDir()
	sizes := []int{300, 200, 100}
	for _, size := range sizes {
		content := bytes.Repeat([]byte{byte(size)}, size)
		writeContent(t, filepath.Join(root, "a"+string(rune(size)), "1.bin"), content)
		writeContent(t, filepath.Join(root, "b"+string(rune(size)), "2.bin"), content)
	}

	result, err := Find(context.Background(), root, Options{MinFileSize: 1, TopN: 2}, nil)
	if err != nil {
		t.Fatalf("Find returned error: %v", err)
	}

	if len(result.Groups) != 2 {
		t.Fatalf("len(Groups) = %d, want 2", len(result.Groups))
	}
	if !result.Truncated {
		t.Error("Truncated = false, want true (3 groups found, only 2 kept)")
	}
	// Kept groups must be the two largest by reclaimable space: 300 and 200.
	if result.Groups[0].Size != 300 || result.Groups[1].Size != 200 {
		t.Errorf("Groups sizes = [%d, %d], want [300, 200]", result.Groups[0].Size, result.Groups[1].Size)
	}
}

func TestFind_TotalReclaimableSumsAllKeptGroups(t *testing.T) {
	root := t.TempDir()
	contentA := bytes.Repeat([]byte("a"), 1000)
	contentB := bytes.Repeat([]byte("b"), 500)
	writeContent(t, filepath.Join(root, "a1.bin"), contentA)
	writeContent(t, filepath.Join(root, "a2.bin"), contentA)
	writeContent(t, filepath.Join(root, "a3.bin"), contentA)
	writeContent(t, filepath.Join(root, "b1.bin"), contentB)
	writeContent(t, filepath.Join(root, "b2.bin"), contentB)

	result, err := Find(context.Background(), root, Options{MinFileSize: 1}, nil)
	if err != nil {
		t.Fatalf("Find returned error: %v", err)
	}

	// a: 3 copies of 1000 bytes -> reclaim 2*1000 = 2000
	// b: 2 copies of 500 bytes -> reclaim 1*500 = 500
	want := int64(2500)
	if result.TotalReclaimable != want {
		t.Errorf("TotalReclaimable = %d, want %d", result.TotalReclaimable, want)
	}
}

func TestFind_MissingRootReturnsError(t *testing.T) {
	_, err := Find(context.Background(), filepath.Join(t.TempDir(), "missing"), Options{}, nil)
	if err == nil {
		t.Fatal("expected an error for a missing root, got nil")
	}
}

func TestFind_FileRootReturnsError(t *testing.T) {
	root := t.TempDir()
	filePath := filepath.Join(root, "file.txt")
	writeFile(t, filePath, 10)

	_, err := Find(context.Background(), filePath, Options{}, nil)
	if err == nil {
		t.Fatal("expected an error when root is a file, got nil")
	}
}

func TestFind_UnreadableDirectoryIsReportedNotFatal(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("chmod-based permission denial is not reliable on Windows")
	}

	root := t.TempDir()
	content := bytes.Repeat([]byte("x"), 100)
	writeContent(t, filepath.Join(root, "ok", "a.bin"), content)
	writeContent(t, filepath.Join(root, "ok", "b.bin"), content)

	blocked := filepath.Join(root, "blocked")
	if err := os.MkdirAll(blocked, 0o755); err != nil {
		t.Fatalf("mkdir blocked: %v", err)
	}
	writeContent(t, filepath.Join(blocked, "hidden.bin"), content)
	if err := os.Chmod(blocked, 0o000); err != nil {
		t.Fatalf("chmod blocked: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(blocked, 0o755) })

	result, err := Find(context.Background(), root, Options{MinFileSize: 1}, nil)
	if err != nil {
		t.Fatalf("Find returned error: %v", err)
	}

	if len(result.Groups) != 1 {
		t.Errorf("len(Groups) = %d, want 1 (blocked dir excluded)", len(result.Groups))
	}
	if len(result.Issues) != 1 {
		t.Fatalf("len(Issues) = %d, want 1", len(result.Issues))
	}
}

func TestFind_DoesNotFollowJunctions(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("junctions are a Windows-specific reparse point type")
	}

	root := t.TempDir()
	content := bytes.Repeat([]byte("x"), 100)
	writeContent(t, filepath.Join(root, "real", "a.bin"), content)

	target := filepath.Join(root, "real")
	link := filepath.Join(root, "link")

	cmd := exec.Command("cmd", "/c", "mklink", "/J", link, target)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Skipf("junction creation not available in this environment: %v (%s)", err, out)
	}

	result, err := Find(context.Background(), root, Options{MinFileSize: 1}, nil)
	if err != nil {
		t.Fatalf("Find returned error: %v", err)
	}

	// The junction itself is a directory entry Nuvio genuinely visits (and
	// counts as a scanned leaf, same as internal/scanner's equivalent
	// test) — the invariant that matters is that it was never traversed
	// as if it were "real", which would have made a.bin appear twice and
	// be reported as a duplicate of itself.
	if len(result.Groups) != 0 {
		t.Errorf("Groups = %+v, want none (junction must not cause a.bin to be seen twice)", result.Groups)
	}
}

func TestFind_CancellationStopsPromptlyAndMarksResult(t *testing.T) {
	root := t.TempDir()
	for i := 0; i < 50; i++ {
		writeFile(t, filepath.Join(root, "d"+string(rune('a'+i%26)), "f.bin"), 10)
	}

	ctx, cancel := context.WithCancel(context.Background())

	var once sync.Once
	done := make(chan struct{})
	onProgress := func(_ Progress) {
		once.Do(func() {
			cancel()
			close(done)
		})
	}

	result, err := Find(ctx, root, Options{Workers: 1, MinFileSize: 1, ProgressInterval: time.Millisecond}, onProgress)
	if err != nil {
		t.Fatalf("Find returned error: %v", err)
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

func TestFind_NoGoroutineLeakAfterCompletion(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "a.bin"), 10)

	before := runtime.NumGoroutine()

	for i := 0; i < 5; i++ {
		if _, err := Find(context.Background(), root, Options{MinFileSize: 1}, func(Progress) {}); err != nil {
			t.Fatalf("Find returned error: %v", err)
		}
	}

	deadline := time.Now().Add(time.Second)
	for runtime.NumGoroutine() > before && time.Now().Before(deadline) {
		runtime.Gosched()
	}

	if after := runtime.NumGoroutine(); after > before {
		t.Errorf("goroutine count grew from %d to %d after repeated scans", before, after)
	}
}

func TestCoordinator_StartAndCancel(t *testing.T) {
	root := t.TempDir()
	content := bytes.Repeat([]byte("x"), 100)
	for i := 0; i < 20; i++ {
		writeContent(t, filepath.Join(root, "d"+string(rune('a'+i)), "f.bin"), content)
	}

	c := NewCoordinator()
	completed := make(chan Result, 1)
	failed := make(chan error, 1)

	id, err := c.Start(root, Options{Workers: 1, MinFileSize: 1, ProgressInterval: time.Millisecond}, Callbacks{
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
		_ = r
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
