package sqlite

import (
	"database/sql"
	"fmt"
)

type migration struct {
	version int
	up      string
}

func createMigrationsTable(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version INTEGER PRIMARY KEY,
			applied_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)
	`)
	return err
}

func runMigrations(db *sql.DB, migrations []migration) error {
	// Get the current schema version
	var currentVersion int
	err := db.QueryRow(`
		SELECT COALESCE(MAX(version), 0) 
		FROM schema_migrations
	`).Scan(&currentVersion)
	if err != nil {
		return fmt.Errorf("getting current schema version: %w", err)
	}

	// Run each migration in a transaction
	for _, m := range migrations {
		if m.version <= currentVersion {
			continue
		}

		tx, err := db.Begin()
		if err != nil {
			return fmt.Errorf("beginning transaction for migration %d: %w", m.version, err)
		}

		// Run the migration
		if _, err := tx.Exec(m.up); err != nil {
			tx.Rollback()
			return fmt.Errorf("running migration %d: %w", m.version, err)
		}

		// Record the migration
		if _, err := tx.Exec(`
			INSERT INTO schema_migrations (version) 
			VALUES (?)
		`, m.version); err != nil {
			tx.Rollback()
			return fmt.Errorf("recording migration %d: %w", m.version, err)
		}

		if err := tx.Commit(); err != nil {
			return fmt.Errorf("committing migration %d: %w", m.version, err)
		}
	}

	return nil
}
