package operations

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"time"
)

// Journal persists JournalEntry records in SQLite. It is safe for
// concurrent use — all operations go straight through to the database,
// which serializes them.
type Journal struct {
	db *sql.DB
}

// NewJournal wraps db, Nuvio's single shared SQLite connection.
func NewJournal(db *sql.DB) *Journal {
	return &Journal{db: db}
}

// Record persists one successful move.
func (j *Journal) Record(ctx context.Context, batchID, source, destination string) (JournalEntry, error) {
	id, err := newID()
	if err != nil {
		return JournalEntry{}, fmt.Errorf("operations: generate journal entry id: %w", err)
	}
	executedAt := time.Now().UTC()

	_, err = j.db.ExecContext(ctx,
		`INSERT INTO operation_items (id, batch_id, source, destination, executed_at) VALUES (?, ?, ?, ?, ?)`,
		id, batchID, source, destination, executedAt.Format(time.RFC3339Nano),
	)
	if err != nil {
		return JournalEntry{}, fmt.Errorf("operations: record journal entry: %w", err)
	}

	return JournalEntry{
		ID:          id,
		BatchID:     batchID,
		Source:      source,
		Destination: destination,
		ExecutedAt:  executedAt,
	}, nil
}

// ListBatch returns every not-yet-undone entry in a batch, oldest first.
func (j *Journal) ListBatch(ctx context.Context, batchID string) ([]JournalEntry, error) {
	rows, err := j.db.QueryContext(ctx,
		`SELECT id, batch_id, source, destination, executed_at, undone_at
		 FROM operation_items WHERE batch_id = ? AND undone_at IS NULL ORDER BY executed_at`,
		batchID,
	)
	if err != nil {
		return nil, fmt.Errorf("operations: list batch %q: %w", batchID, err)
	}
	defer func() { _ = rows.Close() }()

	var out []JournalEntry
	for rows.Next() {
		var e JournalEntry
		var executedAt string
		var undoneAt sql.NullString
		if err := rows.Scan(&e.ID, &e.BatchID, &e.Source, &e.Destination, &executedAt, &undoneAt); err != nil {
			return nil, fmt.Errorf("operations: scan journal entry: %w", err)
		}
		e.ExecutedAt, err = time.Parse(time.RFC3339Nano, executedAt)
		if err != nil {
			return nil, fmt.Errorf("operations: parse executed_at %q: %w", executedAt, err)
		}
		if undoneAt.Valid {
			t, err := time.Parse(time.RFC3339Nano, undoneAt.String)
			if err != nil {
				return nil, fmt.Errorf("operations: parse undone_at %q: %w", undoneAt.String, err)
			}
			e.UndoneAt = &t
		}
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("operations: list batch %q: %w", batchID, err)
	}
	return out, nil
}

// markUndone records that entry has been reversed, so a second Undo call
// on the same batch won't try to move it again.
func (j *Journal) markUndone(ctx context.Context, entryID string) error {
	_, err := j.db.ExecContext(ctx,
		`UPDATE operation_items SET undone_at = ? WHERE id = ?`,
		time.Now().UTC().Format(time.RFC3339Nano), entryID,
	)
	if err != nil {
		return fmt.Errorf("operations: mark entry %q undone: %w", entryID, err)
	}
	return nil
}

func newID() (string, error) {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}
