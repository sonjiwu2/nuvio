package rules_test

import (
	"context"
	"testing"

	"github.com/sonjiwu2/nuvio/internal/persistence"
	"github.com/sonjiwu2/nuvio/internal/rules"
)

func newTestStore(t *testing.T) *rules.Store {
	t.Helper()
	db, err := persistence.Open(t.TempDir())
	if err != nil {
		t.Fatalf("persistence.Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return rules.NewStore(db)
}

func TestStore_AddNormalizesExtensionAndPersists(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	rule, err := store.Add(ctx, ".PDF", "Documents/PDFs")
	if err != nil {
		t.Fatalf("Add returned error: %v", err)
	}
	if rule.Extension != "pdf" {
		t.Errorf("Extension = %q, want %q (lowercased, dot stripped)", rule.Extension, "pdf")
	}
	if rule.ID == "" {
		t.Error("ID is empty")
	}
	if rule.CreatedAt.IsZero() {
		t.Error("CreatedAt is zero")
	}

	list, err := store.List(ctx)
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(list) != 1 || list[0].ID != rule.ID {
		t.Errorf("List() = %+v, want a single entry matching Add's result", list)
	}
}

func TestStore_AddRejectsEmptyExtensionOrDestination(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	if _, err := store.Add(ctx, "  ", "Documents"); err == nil {
		t.Error("expected an error for an empty extension, got nil")
	}
	if _, err := store.Add(ctx, "pdf", "  "); err == nil {
		t.Error("expected an error for an empty destination folder, got nil")
	}
}

func TestStore_DeleteRemovesRule(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	rule, err := store.Add(ctx, "png", "Pictures")
	if err != nil {
		t.Fatalf("Add returned error: %v", err)
	}

	if err := store.Delete(ctx, rule.ID); err != nil {
		t.Fatalf("Delete returned error: %v", err)
	}

	list, err := store.List(ctx)
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(list) != 0 {
		t.Errorf("List() = %+v, want empty after Delete", list)
	}
}

func TestStore_DeleteUnknownIDReturnsError(t *testing.T) {
	store := newTestStore(t)
	if err := store.Delete(context.Background(), "does-not-exist"); err == nil {
		t.Error("expected an error deleting an unknown id, got nil")
	}
}

func TestStore_ListOrdersByCreationTime(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	first, err := store.Add(ctx, "pdf", "Documents/PDFs")
	if err != nil {
		t.Fatalf("Add returned error: %v", err)
	}
	second, err := store.Add(ctx, "png", "Pictures")
	if err != nil {
		t.Fatalf("Add returned error: %v", err)
	}

	list, err := store.List(ctx)
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(list) != 2 || list[0].ID != first.ID || list[1].ID != second.ID {
		t.Errorf("List() = %+v, want [%s, %s] in insertion order", list, first.ID, second.ID)
	}
}
