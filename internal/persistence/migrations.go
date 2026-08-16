package persistence

import (
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"sort"
	"strconv"
	"strings"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

// migrate applies every migration in migrations/ that is newer than the
// schema's current version, in filename order (0001_, 0002_, ...), each in
// its own transaction. It is safe to call on every startup.
func migrate(db *sql.DB) error {
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (version INTEGER PRIMARY KEY)`); err != nil {
		return fmt.Errorf("persistence: create schema_migrations table: %w", err)
	}

	current, err := currentVersion(db)
	if err != nil {
		return err
	}

	entries, err := fs.ReadDir(migrationFiles, "migrations")
	if err != nil {
		return fmt.Errorf("persistence: read embedded migrations: %w", err)
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })

	for _, entry := range entries {
		version, err := migrationVersion(entry.Name())
		if err != nil {
			return err
		}
		if version <= current {
			continue
		}

		sqlBytes, err := migrationFiles.ReadFile("migrations/" + entry.Name())
		if err != nil {
			return fmt.Errorf("persistence: read migration %s: %w", entry.Name(), err)
		}

		if err := applyMigration(db, version, string(sqlBytes)); err != nil {
			return fmt.Errorf("persistence: apply migration %s: %w", entry.Name(), err)
		}
	}

	return nil
}

func currentVersion(db *sql.DB) (int, error) {
	var version int
	err := db.QueryRow(`SELECT COALESCE(MAX(version), 0) FROM schema_migrations`).Scan(&version)
	if err != nil {
		return 0, fmt.Errorf("persistence: read schema version: %w", err)
	}
	return version, nil
}

func migrationVersion(filename string) (int, error) {
	prefix, _, ok := strings.Cut(filename, "_")
	if !ok {
		return 0, fmt.Errorf("persistence: migration filename %q missing version prefix", filename)
	}
	version, err := strconv.Atoi(prefix)
	if err != nil {
		return 0, fmt.Errorf("persistence: migration filename %q has non-numeric version: %w", filename, err)
	}
	return version, nil
}

func applyMigration(db *sql.DB, version int, statements string) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck // rollback after successful commit is a no-op

	if _, err := tx.Exec(statements); err != nil {
		return err
	}
	if _, err := tx.Exec(`INSERT INTO schema_migrations (version) VALUES (?)`, version); err != nil {
		return err
	}

	return tx.Commit()
}
