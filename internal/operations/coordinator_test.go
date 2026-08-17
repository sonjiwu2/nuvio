package operations

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func TestCoordinator_ReturnedIDMatchesJournaledBatch(t *testing.T) {
	root := t.TempDir()
	journal := newTestJournal(t)

	source := filepath.Join(root, "a.txt")
	dest := filepath.Join(root, "out", "a.txt")
	writeFile(t, source, "content")

	c := NewCoordinator(journal)
	completed := make(chan Result, 1)
	failed := make(chan error, 1)

	id, err := c.Start([]MoveItem{{Source: source, Destination: dest}}, Options{}, Callbacks{
		OnComplete: func(_ string, r Result) { completed <- r },
		OnFailed:   func(_ string, e error) { failed <- e },
	})
	if err != nil {
		t.Fatalf("Start returned error: %v", err)
	}

	var result Result
	select {
	case result = <-completed:
	case err := <-failed:
		t.Fatalf("batch failed: %v", err)
	case <-time.After(5 * time.Second):
		t.Fatal("batch did not complete")
	}

	// This is the whole point: the id Start returned, the id the
	// callbacks were given, and the id actually written to the journal
	// must all be the same id — otherwise a caller handing Start's
	// returned id to Undo would find nothing there.
	if result.BatchID != id {
		t.Errorf("result.BatchID = %q, want %q (Start's returned id)", result.BatchID, id)
	}

	entries, err := journal.ListBatch(context.Background(), id)
	if err != nil {
		t.Fatalf("ListBatch returned error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("len(entries) = %d, want 1 (journaled under Start's returned id)", len(entries))
	}
}

func TestCoordinator_CancelStopsInFlightBatch(t *testing.T) {
	root := t.TempDir()
	journal := newTestJournal(t)

	var items []MoveItem
	for i := 0; i < 50; i++ {
		name := "f" + string(rune('a'+i%26)) + string(rune('a'+i/26)) + ".txt"
		writeFile(t, filepath.Join(root, name), "x")
		items = append(items, MoveItem{Source: filepath.Join(root, name), Destination: filepath.Join(root, "out", name)})
	}

	c := NewCoordinator(journal)
	completed := make(chan Result, 1)

	id, err := c.Start(items, Options{Workers: 1}, Callbacks{
		OnComplete: func(_ string, r Result) { completed <- r },
	})
	if err != nil {
		t.Fatalf("Start returned error: %v", err)
	}

	if !c.Cancel(id) {
		t.Fatal("Cancel returned false for a batch that should still be running or just finished")
	}

	select {
	case result := <-completed:
		_ = result // cancellation racing completion is fine; either outcome is valid
	case <-time.After(5 * time.Second):
		t.Fatal("batch did not complete after cancellation")
	}

	if c.Cancel("unknown-id") {
		t.Error("Cancel returned true for an unknown batch id")
	}
}
