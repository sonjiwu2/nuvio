package operations

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/sonjiwu2/nuvio/internal/persistence"
)

func newTestJournal(t *testing.T) *Journal {
	t.Helper()
	db, err := persistence.Open(t.TempDir())
	if err != nil {
		t.Fatalf("persistence.Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return NewJournal(db)
}

func TestApply_MovesFilesAndReportsCounts(t *testing.T) {
	root := t.TempDir()
	journal := newTestJournal(t)

	writeFile(t, filepath.Join(root, "a.txt"), "a")
	writeFile(t, filepath.Join(root, "b.txt"), "b")

	items := []MoveItem{
		{Source: filepath.Join(root, "a.txt"), Destination: filepath.Join(root, "out", "a.txt")},
		{Source: filepath.Join(root, "b.txt"), Destination: filepath.Join(root, "out", "b.txt")},
	}

	result, err := Apply(context.Background(), journal, items, Options{}, nil)
	if err != nil {
		t.Fatalf("Apply returned error: %v", err)
	}

	if result.Succeeded != 2 {
		t.Errorf("Succeeded = %d, want 2", result.Succeeded)
	}
	if result.Skipped != 0 || result.Failed != 0 {
		t.Errorf("Skipped = %d, Failed = %d, want 0, 0", result.Skipped, result.Failed)
	}
	if !exists(filepath.Join(root, "out", "a.txt")) || !exists(filepath.Join(root, "out", "b.txt")) {
		t.Error("moved files not found at destination")
	}
}

func TestApply_JournalsOnlySuccessfulMoves(t *testing.T) {
	root := t.TempDir()
	journal := newTestJournal(t)

	writeFile(t, filepath.Join(root, "a.txt"), "a")
	writeFile(t, filepath.Join(root, "b.txt"), "b")
	writeFile(t, filepath.Join(root, "b_dest.txt"), "already there")

	items := []MoveItem{
		{Source: filepath.Join(root, "a.txt"), Destination: filepath.Join(root, "a_out.txt")},
		{Source: filepath.Join(root, "b.txt"), Destination: filepath.Join(root, "b_dest.txt")}, // will conflict
	}

	result, err := Apply(context.Background(), journal, items, Options{ConflictPolicy: ConflictSkip}, nil)
	if err != nil {
		t.Fatalf("Apply returned error: %v", err)
	}
	if result.Succeeded != 1 || result.Skipped != 1 {
		t.Fatalf("Succeeded=%d Skipped=%d, want 1, 1", result.Succeeded, result.Skipped)
	}

	entries, err := journal.ListBatch(context.Background(), result.BatchID)
	if err != nil {
		t.Fatalf("ListBatch returned error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("len(entries) = %d, want 1 (only the successful move should be journaled)", len(entries))
	}
	if entries[0].Source != filepath.Join(root, "a.txt") {
		t.Errorf("journaled source = %q, want the successfully moved file", entries[0].Source)
	}
}

func TestApply_PartialFailureDoesNotAbortBatch(t *testing.T) {
	root := t.TempDir()
	journal := newTestJournal(t)

	writeFile(t, filepath.Join(root, "good.txt"), "good")
	writeFile(t, filepath.Join(root, "bad.txt"), "bad")

	blocker := filepath.Join(root, "blocker")
	writeFile(t, blocker, "a file, not a directory")

	items := []MoveItem{
		{Source: filepath.Join(root, "good.txt"), Destination: filepath.Join(root, "out", "good.txt")},
		{Source: filepath.Join(root, "bad.txt"), Destination: filepath.Join(blocker, "sub", "bad.txt")},
	}

	result, err := Apply(context.Background(), journal, items, Options{Workers: 1}, nil)
	if err != nil {
		t.Fatalf("Apply returned error: %v", err)
	}
	if result.Succeeded != 1 {
		t.Errorf("Succeeded = %d, want 1", result.Succeeded)
	}
	if result.Failed != 1 {
		t.Errorf("Failed = %d, want 1", result.Failed)
	}
	if !exists(filepath.Join(root, "out", "good.txt")) {
		t.Error("the good item should still have been moved despite the other item failing")
	}
	if !exists(filepath.Join(root, "bad.txt")) {
		t.Error("the failed item's source must still exist untouched")
	}
}

func TestApply_ReportsProgress(t *testing.T) {
	root := t.TempDir()
	journal := newTestJournal(t)

	for i := 0; i < 5; i++ {
		writeFile(t, filepath.Join(root, "f"+string(rune('a'+i))+".txt"), "x")
	}
	items := []MoveItem{
		{Source: filepath.Join(root, "fa.txt"), Destination: filepath.Join(root, "out", "fa.txt")},
		{Source: filepath.Join(root, "fb.txt"), Destination: filepath.Join(root, "out", "fb.txt")},
		{Source: filepath.Join(root, "fc.txt"), Destination: filepath.Join(root, "out", "fc.txt")},
		{Source: filepath.Join(root, "fd.txt"), Destination: filepath.Join(root, "out", "fd.txt")},
		{Source: filepath.Join(root, "fe.txt"), Destination: filepath.Join(root, "out", "fe.txt")},
	}

	var mu sync.Mutex
	var maxCompleted int64
	onProgress := func(p Progress) {
		mu.Lock()
		defer mu.Unlock()
		if p.Completed > maxCompleted {
			maxCompleted = p.Completed
		}
		if p.Total != int64(len(items)) {
			t.Errorf("Progress.Total = %d, want %d", p.Total, len(items))
		}
	}

	if _, err := Apply(context.Background(), journal, items, Options{Workers: 1}, onProgress); err != nil {
		t.Fatalf("Apply returned error: %v", err)
	}

	if maxCompleted != int64(len(items)) {
		t.Errorf("final Progress.Completed = %d, want %d", maxCompleted, len(items))
	}
}

func TestApply_CancellationStopsStartingNewMoves(t *testing.T) {
	root := t.TempDir()
	journal := newTestJournal(t)

	var items []MoveItem
	for i := 0; i < 50; i++ {
		name := "f" + string(rune('a'+i%26)) + string(rune('a'+i/26)) + ".txt"
		writeFile(t, filepath.Join(root, name), "x")
		items = append(items, MoveItem{Source: filepath.Join(root, name), Destination: filepath.Join(root, "out", name)})
	}

	ctx, cancel := context.WithCancel(context.Background())
	var once sync.Once
	onProgress := func(_ Progress) {
		once.Do(cancel)
	}

	result, err := Apply(ctx, journal, items, Options{Workers: 1}, onProgress)
	if err != nil {
		t.Fatalf("Apply returned error: %v", err)
	}
	if !result.Cancelled {
		t.Error("Cancelled = false, want true")
	}
	if result.Succeeded >= int64(len(items)) {
		t.Error("cancellation did not stop the batch before all items were processed")
	}
}

func TestUndo_RestoresFileToOriginalLocation(t *testing.T) {
	root := t.TempDir()
	journal := newTestJournal(t)

	source := filepath.Join(root, "a.txt")
	dest := filepath.Join(root, "out", "a.txt")
	writeFile(t, source, "original content")

	applyResult, err := Apply(context.Background(), journal, []MoveItem{{Source: source, Destination: dest}}, Options{}, nil)
	if err != nil {
		t.Fatalf("Apply returned error: %v", err)
	}
	if applyResult.Succeeded != 1 {
		t.Fatalf("Apply did not succeed, Succeeded=%d", applyResult.Succeeded)
	}

	undoResult, err := Undo(context.Background(), journal, applyResult.BatchID)
	if err != nil {
		t.Fatalf("Undo returned error: %v", err)
	}
	if undoResult.Restored != 1 {
		t.Fatalf("Restored = %d, want 1", undoResult.Restored)
	}
	if !exists(source) {
		t.Error("source was not restored")
	}
	if exists(dest) {
		t.Error("destination file still exists after undo")
	}
	if got := readFile(t, source); got != "original content" {
		t.Errorf("restored content = %q, want %q", got, "original content")
	}
}

func TestUndo_SkipsWhenDestinationFileIsGone(t *testing.T) {
	root := t.TempDir()
	journal := newTestJournal(t)

	source := filepath.Join(root, "a.txt")
	dest := filepath.Join(root, "out", "a.txt")
	writeFile(t, source, "content")

	applyResult, err := Apply(context.Background(), journal, []MoveItem{{Source: source, Destination: dest}}, Options{}, nil)
	if err != nil {
		t.Fatalf("Apply returned error: %v", err)
	}

	// The user (or something else) removed the moved file before undo ran.
	if err := os.Remove(dest); err != nil {
		t.Fatalf("os.Remove: %v", err)
	}

	undoResult, err := Undo(context.Background(), journal, applyResult.BatchID)
	if err != nil {
		t.Fatalf("Undo returned error: %v", err)
	}
	if undoResult.Skipped != 1 {
		t.Errorf("Skipped = %d, want 1", undoResult.Skipped)
	}
	if undoResult.Restored != 0 {
		t.Errorf("Restored = %d, want 0", undoResult.Restored)
	}
}

func TestUndo_NeverOverwritesANewFileAtTheOriginalLocation(t *testing.T) {
	root := t.TempDir()
	journal := newTestJournal(t)

	source := filepath.Join(root, "a.txt")
	dest := filepath.Join(root, "out", "a.txt")
	writeFile(t, source, "moved content")

	applyResult, err := Apply(context.Background(), journal, []MoveItem{{Source: source, Destination: dest}}, Options{}, nil)
	if err != nil {
		t.Fatalf("Apply returned error: %v", err)
	}

	// The user creates a brand new, unrelated file at the original path
	// before undo runs. This must survive no matter what.
	writeFile(t, source, "the user's new, unrelated file")

	undoResult, err := Undo(context.Background(), journal, applyResult.BatchID)
	if err != nil {
		t.Fatalf("Undo returned error: %v", err)
	}
	if undoResult.Skipped != 1 {
		t.Errorf("Skipped = %d, want 1", undoResult.Skipped)
	}
	if got := readFile(t, source); got != "the user's new, unrelated file" {
		t.Fatalf("the user's new file was destroyed by undo; content = %q", got)
	}
	if !exists(dest) {
		t.Error("the moved file should still be at its destination since undo safely skipped it")
	}
}

func TestUndo_MarksEntriesUndoneSoRepeatUndoDoesNothing(t *testing.T) {
	root := t.TempDir()
	journal := newTestJournal(t)

	source := filepath.Join(root, "a.txt")
	dest := filepath.Join(root, "out", "a.txt")
	writeFile(t, source, "content")

	applyResult, err := Apply(context.Background(), journal, []MoveItem{{Source: source, Destination: dest}}, Options{}, nil)
	if err != nil {
		t.Fatalf("Apply returned error: %v", err)
	}

	if _, err := Undo(context.Background(), journal, applyResult.BatchID); err != nil {
		t.Fatalf("first Undo returned error: %v", err)
	}

	_, err = Undo(context.Background(), journal, applyResult.BatchID)
	if err == nil {
		t.Fatal("expected an error undoing a batch with nothing left to undo, got nil")
	}
}

func TestUndo_ErrorsForUnknownBatch(t *testing.T) {
	journal := newTestJournal(t)
	_, err := Undo(context.Background(), journal, "does-not-exist")
	if err == nil {
		t.Fatal("expected an error for an unknown batch id, got nil")
	}
}

func TestUndo_ReversesMostRecentMoveFirst(t *testing.T) {
	root := t.TempDir()
	journal := newTestJournal(t)

	// Two moves into the same batch via two Apply calls sharing a journal
	// is not how the real flow works (Apply creates its own batch id per
	// call), so instead verify ordering using journal entries directly:
	// two entries in one batch, undone in reverse chronological order.
	batchID := "batch-order-test"
	e1, err := journal.Record(context.Background(), batchID, filepath.Join(root, "s1.txt"), filepath.Join(root, "d1.txt"))
	if err != nil {
		t.Fatalf("Record: %v", err)
	}
	time.Sleep(2 * time.Millisecond) // ensure a distinct executed_at ordering
	e2, err := journal.Record(context.Background(), batchID, filepath.Join(root, "s2.txt"), filepath.Join(root, "d2.txt"))
	if err != nil {
		t.Fatalf("Record: %v", err)
	}
	writeFile(t, e1.Destination, "one")
	writeFile(t, e2.Destination, "two")

	result, err := Undo(context.Background(), journal, batchID)
	if err != nil {
		t.Fatalf("Undo returned error: %v", err)
	}
	if len(result.Items) != 2 {
		t.Fatalf("len(Items) = %d, want 2", len(result.Items))
	}
	if result.Items[0].Source != e2.Destination {
		t.Errorf("first undone item = %q, want the most recently recorded entry %q", result.Items[0].Source, e2.Destination)
	}
}
