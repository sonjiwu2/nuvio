package search

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"
)

func writeFile(t *testing.T, path string, size int64) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir for %s: %v", path, err)
	}
	if err := os.WriteFile(path, make([]byte, size), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func TestFind_MatchesCaseInsensitiveSubstring(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "Invoice-2024.pdf"), 10)
	writeFile(t, filepath.Join(root, "nested", "invoice-final.PDF"), 10)
	writeFile(t, filepath.Join(root, "photo.png"), 10)

	result, err := Find(context.Background(), root, Options{Query: "invoice"}, nil, nil)
	if err != nil {
		t.Fatalf("Find returned error: %v", err)
	}

	if result.MatchCount != 2 {
		t.Errorf("MatchCount = %d, want 2", result.MatchCount)
	}
	if result.FilesScanned != 3 {
		t.Errorf("FilesScanned = %d, want 3", result.FilesScanned)
	}
}

func TestFind_RejectsEmptyQuery(t *testing.T) {
	root := t.TempDir()
	_, err := Find(context.Background(), root, Options{Query: "   "}, nil, nil)
	if err == nil {
		t.Fatal("expected an error for an empty/blank query, got nil")
	}
}

func TestFind_MissingRootReturnsError(t *testing.T) {
	_, err := Find(context.Background(), filepath.Join(t.TempDir(), "missing"), Options{Query: "x"}, nil, nil)
	if err == nil {
		t.Fatal("expected an error for a missing root, got nil")
	}
}

func TestFind_FileRootReturnsError(t *testing.T) {
	root := t.TempDir()
	filePath := filepath.Join(root, "file.txt")
	writeFile(t, filePath, 10)

	_, err := Find(context.Background(), filePath, Options{Query: "x"}, nil, nil)
	if err == nil {
		t.Fatal("expected an error when root is a file, got nil")
	}
}

func TestFind_UnreadableDirectoryIsReportedNotFatal(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("chmod-based permission denial is not reliable on Windows")
	}

	root := t.TempDir()
	writeFile(t, filepath.Join(root, "ok", "report.txt"), 10)

	blocked := filepath.Join(root, "blocked")
	if err := os.MkdirAll(blocked, 0o755); err != nil {
		t.Fatalf("mkdir blocked: %v", err)
	}
	writeFile(t, filepath.Join(blocked, "report-hidden.txt"), 10)
	if err := os.Chmod(blocked, 0o000); err != nil {
		t.Fatalf("chmod blocked: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(blocked, 0o755) })

	result, err := Find(context.Background(), root, Options{Query: "report"}, nil, nil)
	if err != nil {
		t.Fatalf("Find returned error: %v", err)
	}

	if result.MatchCount != 1 {
		t.Errorf("MatchCount = %d, want 1 (blocked dir excluded)", result.MatchCount)
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
	writeFile(t, filepath.Join(root, "real", "report.txt"), 10)

	target := filepath.Join(root, "real")
	link := filepath.Join(root, "link")

	cmd := exec.Command("cmd", "/c", "mklink", "/J", link, target)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Skipf("junction creation not available in this environment: %v (%s)", err, out)
	}

	result, err := Find(context.Background(), root, Options{Query: "report"}, nil, nil)
	if err != nil {
		t.Fatalf("Find returned error: %v", err)
	}

	if result.MatchCount != 1 {
		t.Errorf("MatchCount = %d, want 1 (junction must not be followed, or report.txt would be found twice)", result.MatchCount)
	}
}

func TestFind_TruncatesAtMaxMatchesWithoutReportingUserCancellation(t *testing.T) {
	root := t.TempDir()
	for i := 0; i < 20; i++ {
		writeFile(t, filepath.Join(root, "d"+string(rune('a'+i)), "match.txt"), 10)
	}

	result, err := Find(context.Background(), root, Options{Query: "match", MaxMatches: 5, Workers: 1}, nil, nil)
	if err != nil {
		t.Fatalf("Find returned error: %v", err)
	}

	if result.MatchCount != 5 {
		t.Errorf("MatchCount = %d, want 5 (capped)", result.MatchCount)
	}
	if !result.Truncated {
		t.Error("Truncated = false, want true")
	}
	if result.Cancelled {
		t.Error("Cancelled = true, want false — hitting the cap is not the same as the user cancelling")
	}
}

func TestFind_CancellationStopsPromptlyAndMarksResult(t *testing.T) {
	root := t.TempDir()
	for i := 0; i < 50; i++ {
		writeFile(t, filepath.Join(root, "d"+string(rune('a'+i%26)), "match.txt"), 10)
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

	result, err := Find(ctx, root, Options{Query: "match", Workers: 1, BatchInterval: time.Millisecond}, onProgress, nil)
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
	writeFile(t, filepath.Join(root, "match.txt"), 10)

	before := runtime.NumGoroutine()

	for i := 0; i < 5; i++ {
		if _, err := Find(context.Background(), root, Options{Query: "match"}, func(Progress) {}, func([]Match) {}); err != nil {
			t.Fatalf("Find returned error: %v", err)
		}
	}

	deadline := time.Now().Add(time.Second)
	for runtime.NumGoroutine() > before && time.Now().Before(deadline) {
		runtime.Gosched()
	}

	if after := runtime.NumGoroutine(); after > before {
		t.Errorf("goroutine count grew from %d to %d after repeated searches", before, after)
	}
}

func TestCoordinator_StartDeliversMatchesAndCompletes(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "invoice.pdf"), 10)

	c := NewCoordinator()
	matches := make(chan []Match, 8)
	completed := make(chan Result, 1)
	failed := make(chan error, 1)

	id, err := c.Start(root, Options{Query: "invoice", BatchInterval: time.Millisecond}, Callbacks{
		OnMatches:  func(_ string, batch []Match) { matches <- batch },
		OnComplete: func(_ string, r Result) { completed <- r },
		OnFailed:   func(_ string, e error) { failed <- e },
	})
	if err != nil {
		t.Fatalf("Start returned error: %v", err)
	}
	if id == "" {
		t.Fatal("Start returned empty search id")
	}

	select {
	case r := <-completed:
		if r.MatchCount != 1 {
			t.Errorf("MatchCount = %d, want 1", r.MatchCount)
		}
	case err := <-failed:
		t.Fatalf("search failed: %v", err)
	case <-time.After(5 * time.Second):
		t.Fatal("search did not complete")
	}

	select {
	case batch := <-matches:
		if len(batch) != 1 || batch[0].Name != "invoice.pdf" {
			t.Errorf("unexpected match batch: %+v", batch)
		}
	case <-time.After(time.Second):
		t.Fatal("OnMatches was never called")
	}
}

func TestCoordinator_UnknownRootReportsFailure(t *testing.T) {
	c := NewCoordinator()
	failed := make(chan error, 1)

	_, err := c.Start(filepath.Join(t.TempDir(), "missing"), Options{Query: "x"}, Callbacks{
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

func TestCoordinator_CancelUnknownIDReturnsFalse(t *testing.T) {
	c := NewCoordinator()
	if c.Cancel("unknown-id") {
		t.Error("Cancel returned true for an unknown search id")
	}
}
