package rules

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

// Store persists rules in SQLite. It is safe for concurrent use — all
// operations go straight through to the database, which serializes them.
type Store struct {
	db *sql.DB
}

// NewStore wraps db, Nuvio's single shared SQLite connection.
func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

// List returns every rule, oldest first.
func (s *Store) List(ctx context.Context) ([]Rule, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, extension, destination_folder, created_at FROM rules ORDER BY created_at`)
	if err != nil {
		return nil, fmt.Errorf("rules: list: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []Rule
	for rows.Next() {
		var r Rule
		var createdAt string
		if err := rows.Scan(&r.ID, &r.Extension, &r.DestinationFolder, &createdAt); err != nil {
			return nil, fmt.Errorf("rules: scan: %w", err)
		}
		r.CreatedAt, err = time.Parse(time.RFC3339Nano, createdAt)
		if err != nil {
			return nil, fmt.Errorf("rules: parse created_at %q: %w", createdAt, err)
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rules: list: %w", err)
	}
	return out, nil
}

// Add validates and persists a new rule, returning it with its generated
// id and creation time.
func (s *Store) Add(ctx context.Context, extension, destinationFolder string) (Rule, error) {
	extension = normalizeExtension(extension)
	if extension == "" {
		return Rule{}, fmt.Errorf("rules: extension must not be empty")
	}
	destinationFolder = strings.TrimSpace(destinationFolder)
	if destinationFolder == "" {
		return Rule{}, fmt.Errorf("rules: destination folder must not be empty")
	}

	id, err := newRuleID()
	if err != nil {
		return Rule{}, fmt.Errorf("rules: generate id: %w", err)
	}
	createdAt := time.Now().UTC()

	_, err = s.db.ExecContext(ctx,
		`INSERT INTO rules (id, extension, destination_folder, created_at) VALUES (?, ?, ?, ?)`,
		id, extension, destinationFolder, createdAt.Format(time.RFC3339Nano),
	)
	if err != nil {
		return Rule{}, fmt.Errorf("rules: insert: %w", err)
	}

	return Rule{
		ID:                id,
		Extension:         extension,
		DestinationFolder: destinationFolder,
		CreatedAt:         createdAt,
	}, nil
}

// Delete removes the rule with the given id. It returns an error if no
// such rule exists.
func (s *Store) Delete(ctx context.Context, id string) error {
	result, err := s.db.ExecContext(ctx, `DELETE FROM rules WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("rules: delete: %w", err)
	}
	n, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("rules: delete: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("rules: no rule with id %q", id)
	}
	return nil
}

func normalizeExtension(ext string) string {
	ext = strings.TrimSpace(ext)
	ext = strings.TrimPrefix(ext, ".")
	return strings.ToLower(ext)
}

func newRuleID() (string, error) {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}
