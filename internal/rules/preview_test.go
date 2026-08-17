package rules_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/sonjiwu2/nuvio/internal/rules"
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

func pdfRule(destination string) rules.Rule {
	return rules.Rule{ID: "rule-pdf", Extension: "pdf", DestinationFolder: destination}
}

func TestPreview_MatchesFilesByExtensionAndComputesDestination(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "invoice.pdf"), 100)
	writeFile(t, filepath.Join(root, "nested", "report.pdf"), 50)
	writeFile(t, filepath.Join(root, "photo.png"), 10)

	dest := filepath.Join(root, "..", "Documents", "PDFs")
	result, err := rules.Preview(context.Background(), root, []rules.Rule{pdfRule(dest)}, rules.PreviewOptions{}, nil, nil)
	if err != nil {
		t.Fatalf("Preview returned error: %v", err)
	}

	if result.MatchCount != 2 {
		t.Errorf("MatchCount = %d, want 2", result.MatchCount)
	}
	if result.FilesScanned != 3 {
		t.Errorf("FilesScanned = %d, want 3", result.FilesScanned)
	}
	if result.TotalSize != 150 {
		t.Errorf("TotalSize = %d, want 150", result.TotalSize)
	}
}

func TestPreview_ComputesDestinationPathUnderRuleFolder(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "invoice.pdf"), 10)
	dest := filepath.Join(root, "Documents", "PDFs")

	var entries []rules.PreviewEntry
	onEntries := func(batch []rules.PreviewEntry) { entries = append(entries, batch...) }

	_, err := rules.Preview(context.Background(), root, []rules.Rule{pdfRule(dest)}, rules.PreviewOptions{BatchInterval: time.Millisecond}, nil, onEntries)
	if err != nil {
		t.Fatalf("Preview returned error: %v", err)
	}

	if len(entries) != 1 {
		t.Fatalf("got %d entries, want 1", len(entries))
	}
	want := filepath.Join(dest, "invoice.pdf")
	if entries[0].DestinationPath != want {
		t.Errorf("DestinationPath = %q, want %q", entries[0].DestinationPath, want)
	}
	if entries[0].RuleID != "rule-pdf" {
		t.Errorf("RuleID = %q, want %q", entries[0].RuleID, "rule-pdf")
	}
}

func TestPreview_NeverModifiesTheFilesystem(t *testing.T) {
	root := t.TempDir()
	sourcePath := filepath.Join(root, "invoice.pdf")
	writeFile(t, sourcePath, 10)
	dest := filepath.Join(root, "Documents", "PDFs")

	if _, err := rules.Preview(context.Background(), root, []rules.Rule{pdfRule(dest)}, rules.PreviewOptions{}, nil, nil); err != nil {
		t.Fatalf("Preview returned error: %v", err)
	}

	if _, err := os.Stat(sourcePath); err != nil {
		t.Errorf("source file %q no longer exists after Preview: %v", sourcePath, err)
	}
	if _, err := os.Stat(dest); err == nil {
		t.Errorf("destination folder %q was created by Preview — it must never touch the filesystem", dest)
	}
}

func TestPreview_RejectsEmptyRuleSet(t *testing.T) {
	root := t.TempDir()
	_, err := rules.Preview(context.Background(), root, nil, rules.PreviewOptions{}, nil, nil)
	if err == nil {
		t.Fatal("expected an error for an empty rule set, got nil")
	}
}

func TestPreview_MissingRootReturnsError(t *testing.T) {
	_, err := rules.Preview(context.Background(), filepath.Join(t.TempDir(), "missing"), []rules.Rule{pdfRule("dest")}, rules.PreviewOptions{}, nil, nil)
	if err == nil {
		t.Fatal("expected an error for a missing root, got nil")
	}
}

func TestPreview_FileRootReturnsError(t *testing.T) {
	root := t.TempDir()
	filePath := filepath.Join(root, "file.txt")
	writeFile(t, filePath, 10)

	_, err := rules.Preview(context.Background(), filePath, []rules.Rule{pdfRule("dest")}, rules.PreviewOptions{}, nil, nil)
	if err == nil {
		t.Fatal("expected an error when root is a file, got nil")
	}
}

func TestPreview_UnreadableDirectoryIsReportedNotFatal(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("chmod-based permission denial is not reliable on Windows")
	}

	root := t.TempDir()
	writeFile(t, filepath.Join(root, "ok", "a.pdf"), 10)

	blocked := filepath.Join(root, "blocked")
	if err := os.MkdirAll(blocked, 0o755); err != nil {
		t.Fatalf("mkdir blocked: %v", err)
	}
	writeFile(t, filepath.Join(blocked, "hidden.pdf"), 10)
	if err := os.Chmod(blocked, 0o000); err != nil {
		t.Fatalf("chmod blocked: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(blocked, 0o755) })

	result, err := rules.Preview(context.Background(), root, []rules.Rule{pdfRule("dest")}, rules.PreviewOptions{}, nil, nil)
	if err != nil {
		t.Fatalf("Preview returned error: %v", err)
	}

	if result.MatchCount != 1 {
		t.Errorf("MatchCount = %d, want 1 (blocked dir excluded)", result.MatchCount)
	}
	if len(result.Issues) != 1 {
		t.Fatalf("len(Issues) = %d, want 1", len(result.Issues))
	}
}

func TestPreview_DoesNotFollowJunctions(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("junctions are a Windows-specific reparse point type")
	}

	root := t.TempDir()
	writeFile(t, filepath.Join(root, "real", "a.pdf"), 10)

	target := filepath.Join(root, "real")
	link := filepath.Join(root, "link")

	cmd := exec.Command("cmd", "/c", "mklink", "/J", link, target)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Skipf("junction creation not available in this environment: %v (%s)", err, out)
	}

	result, err := rules.Preview(context.Background(), root, []rules.Rule{pdfRule("dest")}, rules.PreviewOptions{}, nil, nil)
	if err != nil {
		t.Fatalf("Preview returned error: %v", err)
	}

	if result.MatchCount != 1 {
		t.Errorf("MatchCount = %d, want 1 (junction must not be followed)", result.MatchCount)
	}
}

func TestPreview_TruncatesAtMaxEntriesWithoutReportingUserCancellation(t *testing.T) {
	root := t.TempDir()
	for i := 0; i < 20; i++ {
		writeFile(t, filepath.Join(root, "d"+string(rune('a'+i)), "a.pdf"), 10)
	}

	result, err := rules.Preview(context.Background(), root, []rules.Rule{pdfRule("dest")}, rules.PreviewOptions{MaxEntries: 5, Workers: 1}, nil, nil)
	if err != nil {
		t.Fatalf("Preview returned error: %v", err)
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

func TestPreview_CancellationStopsPromptlyAndMarksResult(t *testing.T) {
	root := t.TempDir()
	for i := 0; i < 50; i++ {
		writeFile(t, filepath.Join(root, "d"+string(rune('a'+i%26)), "a.pdf"), 10)
	}

	ctx, cancel := context.WithCancel(context.Background())

	var once sync.Once
	done := make(chan struct{})
	onProgress := func(_ rules.PreviewProgress) {
		once.Do(func() {
			cancel()
			close(done)
		})
	}

	result, err := rules.Preview(ctx, root, []rules.Rule{pdfRule("dest")}, rules.PreviewOptions{Workers: 1, BatchInterval: time.Millisecond}, onProgress, nil)
	if err != nil {
		t.Fatalf("Preview returned error: %v", err)
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

func TestPreview_NoGoroutineLeakAfterCompletion(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "a.pdf"), 10)

	before := runtime.NumGoroutine()

	for i := 0; i < 5; i++ {
		if _, err := rules.Preview(context.Background(), root, []rules.Rule{pdfRule("dest")}, rules.PreviewOptions{}, func(rules.PreviewProgress) {}, func([]rules.PreviewEntry) {}); err != nil {
			t.Fatalf("Preview returned error: %v", err)
		}
	}

	deadline := time.Now().Add(time.Second)
	for runtime.NumGoroutine() > before && time.Now().Before(deadline) {
		runtime.Gosched()
	}

	if after := runtime.NumGoroutine(); after > before {
		t.Errorf("goroutine count grew from %d to %d after repeated previews", before, after)
	}
}

func TestCoordinator_StartDeliversEntriesAndCompletes(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "invoice.pdf"), 10)

	c := rules.NewCoordinator()
	entries := make(chan []rules.PreviewEntry, 8)
	completed := make(chan rules.PreviewResult, 1)
	failed := make(chan error, 1)

	id, err := c.Start(root, []rules.Rule{pdfRule("dest")}, rules.PreviewOptions{BatchInterval: time.Millisecond}, rules.PreviewCallbacks{
		OnEntries:  func(_ string, batch []rules.PreviewEntry) { entries <- batch },
		OnComplete: func(_ string, r rules.PreviewResult) { completed <- r },
		OnFailed:   func(_ string, e error) { failed <- e },
	})
	if err != nil {
		t.Fatalf("Start returned error: %v", err)
	}
	if id == "" {
		t.Fatal("Start returned empty preview id")
	}

	select {
	case r := <-completed:
		if r.MatchCount != 1 {
			t.Errorf("MatchCount = %d, want 1", r.MatchCount)
		}
	case err := <-failed:
		t.Fatalf("preview failed: %v", err)
	case <-time.After(5 * time.Second):
		t.Fatal("preview did not complete")
	}

	select {
	case batch := <-entries:
		if len(batch) != 1 || batch[0].Name != "invoice.pdf" {
			t.Errorf("unexpected entry batch: %+v", batch)
		}
	case <-time.After(time.Second):
		t.Fatal("OnEntries was never called")
	}
}

func TestCoordinator_UnknownRootReportsFailure(t *testing.T) {
	c := rules.NewCoordinator()
	failed := make(chan error, 1)

	_, err := c.Start(filepath.Join(t.TempDir(), "missing"), []rules.Rule{pdfRule("dest")}, rules.PreviewOptions{}, rules.PreviewCallbacks{
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
