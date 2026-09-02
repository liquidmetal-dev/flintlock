package sqlite

import (
	"database/sql"
	"fmt"
)

type migration struct {
	id  string
	sql string
}

// migrations is the ordered list of schema migrations to apply to a new or
// existing sqlite database. New migrations must be appended, never edited or
// reordered.
var migrations = []migration{
	{
		id: "0001_create_microvms_table",
		sql: `CREATE TABLE IF NOT EXISTS microvms (
			uid        TEXT NOT NULL,
			version    INTEGER NOT NULL,
			name       TEXT NOT NULL,
			namespace  TEXT NOT NULL,
			spec       BLOB NOT NULL,
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (uid, version)
		);
		CREATE INDEX IF NOT EXISTS idx_microvms_namespace_name ON microvms (namespace, name);`,
	},
}

// runMigrations applies any migrations that have not already been recorded
// as applied in the schema_migrations table.
func runMigrations(db *sql.DB, migrations []migration) error {
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (
		id         TEXT PRIMARY KEY,
		applied_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
	);`); err != nil {
		return fmt.Errorf("creating schema_migrations table: %w", err)
	}

	for _, m := range migrations {
		applied, err := isMigrationApplied(db, m.id)
		if err != nil {
			return fmt.Errorf("checking if migration %s is applied: %w", m.id, err)
		}

		if applied {
			continue
		}

		if err := applyMigration(db, m); err != nil {
			return fmt.Errorf("applying migration %s: %w", m.id, err)
		}
	}

	return nil
}

func isMigrationApplied(db *sql.DB, id string) (bool, error) {
	var count int

	row := db.QueryRow("SELECT COUNT(1) FROM schema_migrations WHERE id = ?", id)
	if err := row.Scan(&count); err != nil {
		return false, fmt.Errorf("querying schema_migrations: %w", err)
	}

	return count > 0, nil
}

func applyMigration(db *sql.DB, m migration) error {
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer tx.Rollback() //nolint: errcheck // rollback is a no-op after a successful commit

	if _, err := tx.Exec(m.sql); err != nil {
		return fmt.Errorf("executing migration sql: %w", err)
	}

	if _, err := tx.Exec("INSERT INTO schema_migrations (id) VALUES (?)", m.id); err != nil {
		return fmt.Errorf("recording migration as applied: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing transaction: %w", err)
	}

	return nil
}
