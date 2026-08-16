// Package persistence owns Nuvio's single SQLite database: connection
// setup, pragmas, and schema migrations. Domain packages depend on this
// package's *sql.DB, never on database/sql or the driver directly.
package persistence

import (
	"database/sql"
	"fmt"
	"path/filepath"

	_ "modernc.org/sqlite" // registers the "sqlite" driver
)

// Open opens the Nuvio database in dir (creating it if absent), applies
// pending migrations, and configures it for a single-process desktop
// workload: WAL journaling so reads never block writes, foreign keys
// enforced, and a busy timeout so concurrent access waits instead of
// failing immediately.
func Open(dir string) (*sql.DB, error) {
	path := filepath.Join(dir, "nuvio.db")

	db, err := sql.Open("sqlite", path+"?_pragma=busy_timeout(5000)")
	if err != nil {
		return nil, fmt.Errorf("persistence: open %q: %w", path, err)
	}

	for _, pragma := range []string{
		"PRAGMA journal_mode = WAL",
		"PRAGMA foreign_keys = ON",
		"PRAGMA synchronous = NORMAL",
	} {
		if _, err := db.Exec(pragma); err != nil {
			_ = db.Close() // best-effort: we are already returning the real error
			return nil, fmt.Errorf("persistence: apply %q: %w", pragma, err)
		}
	}

	if err := migrate(db); err != nil {
		_ = db.Close() // best-effort: we are already returning the real error
		return nil, err
	}

	return db, nil
}
