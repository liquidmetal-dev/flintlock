package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	_ "github.com/mattn/go-sqlite3"

	"github.com/liquidmetal-dev/flintlock/core/errors"
	"github.com/liquidmetal-dev/flintlock/core/models"
	"github.com/liquidmetal-dev/flintlock/core/ports"
	"github.com/liquidmetal-dev/flintlock/pkg/log"
)

type Config struct {
	DatabasePath string
}

type sqliteRepo struct {
	db     *sql.DB
	locks  map[string]*sync.RWMutex
	lockMu sync.Mutex
}

// NewMicroVMRepo creates a new SQLite-backed microvm repository.
func NewMicroVMRepo(cfg *Config) (ports.MicroVMRepository, error) {
	dataPath := filepath.Dir(cfg.DatabasePath)
	if _, err := os.Stat(dataPath); os.IsNotExist(err) {
		if err := os.MkdirAll(dataPath, 0o755); err != nil {
			return nil, fmt.Errorf("creating data directory: %w", err)
		}
	}

	db, err := sql.Open("sqlite3", cfg.DatabasePath)
	if err != nil {
		return nil, fmt.Errorf("opening sqlite database: %w", err)
	}

	// Create tables if they don't exist
	if err := initDatabase(db); err != nil {
		return nil, fmt.Errorf("initializing database: %w", err)
	}

	return &sqliteRepo{
		db:    db,
		locks: make(map[string]*sync.RWMutex),
	}, nil
}

func initDatabase(db *sql.DB) error {
	// Create migrations table first
	if err := createMigrationsTable(db); err != nil {
		return fmt.Errorf("creating migrations table: %w", err)
	}

	// Run migrations
	migrations := []migration{
		{
			version: 1,
			up: `
				CREATE TABLE IF NOT EXISTS microvms (
					uid TEXT NOT NULL,
					name TEXT NOT NULL,
					namespace TEXT NOT NULL,
					version INTEGER NOT NULL,
					spec TEXT NOT NULL,
					created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
					PRIMARY KEY (uid, version)
				)
			`,
		},
		// Add new migrations here with version numbers 2, 3, etc.
	}

	return runMigrations(db, migrations)
}

func (r *sqliteRepo) Save(ctx context.Context, microvm *models.MicroVM) (*models.MicroVM, error) {
	logger := log.GetLogger(ctx).WithField("repo", "sqlite_microvm")
	logger.Debugf("saving microvm spec %s", microvm.ID)

	mu := r.getMutex(microvm.ID.String())
	mu.Lock()
	defer mu.Unlock()

	// Check if exists and compare
	existing, err := r.get(ctx, ports.RepositoryGetOptions{
		Name:      microvm.ID.Name(),
		Namespace: microvm.ID.Namespace(),
		UID:       microvm.ID.UID(),
	})
	if err != nil {
		return nil, fmt.Errorf("checking existing vm: %w", err)
	}

	if existing != nil {
		// If exactly the same, return existing
		specBytes, err := json.Marshal(microvm)
		if err != nil {
			return nil, fmt.Errorf("marshalling new spec: %w", err)
		}

		existingBytes, err := json.Marshal(existing)
		if err != nil {
			return nil, fmt.Errorf("marshalling existing spec: %w", err)
		}

		if string(specBytes) == string(existingBytes) {
			return existing, nil
		}
	}

	// Increment version
	microvm.Version++

	// Marshal the entire microvm to JSON
	specJSON, err := json.Marshal(microvm)
	if err != nil {
		return nil, fmt.Errorf("marshalling microvm: %w", err)
	}

	// Insert new version
	_, err = r.db.ExecContext(ctx, `
		INSERT INTO microvms (uid, name, namespace, version, spec)
		VALUES (?, ?, ?, ?, ?)
	`, microvm.ID.UID(), microvm.ID.Name(), microvm.ID.Namespace(), microvm.Version, string(specJSON))
	if err != nil {
		return nil, fmt.Errorf("inserting microvm: %w", err)
	}

	return microvm, nil
}

func (r *sqliteRepo) Get(ctx context.Context, options ports.RepositoryGetOptions) (*models.MicroVM, error) {
	mu := r.getMutex(options.Name)
	mu.RLock()
	defer mu.RUnlock()

	vm, err := r.get(ctx, options)
	if err != nil {
		return nil, fmt.Errorf("getting vm from db: %w", err)
	}

	if vm == nil {
		return nil, errors.NewSpecNotFound(
			options.Name,
			options.Namespace,
			options.Version,
			options.UID)
	}

	return vm, nil
}

func (r *sqliteRepo) get(ctx context.Context, options ports.RepositoryGetOptions) (*models.MicroVM, error) {
	query := `
		SELECT spec FROM microvms 
		WHERE name = ? AND namespace = ?
	`
	args := []interface{}{options.Name, options.Namespace}

	if options.UID != "" {
		query += " AND uid = ?"
		args = append(args, options.UID)
	}

	if options.Version != "" {
		query += " AND version = ?"
		args = append(args, options.Version)
	} else {
		query += " ORDER BY version DESC LIMIT 1"
	}

	var specJSON string
	err := r.db.QueryRowContext(ctx, query, args...).Scan(&specJSON)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("querying database: %w", err)
	}

	var microvm models.MicroVM
	if err := json.Unmarshal([]byte(specJSON), &microvm); err != nil {
		return nil, fmt.Errorf("unmarshalling spec: %w", err)
	}

	return &microvm, nil
}

func (r *sqliteRepo) GetAll(ctx context.Context, query models.ListMicroVMQuery) ([]*models.MicroVM, error) {
	sqlQuery := `
		SELECT m1.spec 
		FROM microvms m1
		INNER JOIN (
			SELECT uid, MAX(version) as max_version 
			FROM microvms 
			GROUP BY uid
		) m2 
		ON m1.uid = m2.uid AND m1.version = m2.max_version
		WHERE 1=1
	`
	var args []interface{}

	if query["namespace"] != "" {
		sqlQuery += " AND namespace = ?"
		args = append(args, query["namespace"])
	}
	if query["name"] != "" {
		sqlQuery += " AND name = ?"
		args = append(args, query["name"])
	}

	rows, err := r.db.QueryContext(ctx, sqlQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("querying database: %w", err)
	}
	defer rows.Close()

	var results []*models.MicroVM
	for rows.Next() {
		var specJSON string
		if err := rows.Scan(&specJSON); err != nil {
			return nil, fmt.Errorf("scanning row: %w", err)
		}

		var microvm models.MicroVM
		if err := json.Unmarshal([]byte(specJSON), &microvm); err != nil {
			return nil, fmt.Errorf("unmarshalling spec: %w", err)
		}

		results = append(results, &microvm)
	}

	return results, nil
}

func (r *sqliteRepo) Delete(ctx context.Context, microvm *models.MicroVM) error {
	mu := r.getMutex(microvm.ID.String())
	mu.Lock()
	defer mu.Unlock()

	_, err := r.db.ExecContext(ctx, `
		DELETE FROM microvms 
		WHERE uid = ? AND namespace = ? AND name = ?
	`, microvm.ID.UID(), microvm.ID.Namespace(), microvm.ID.Name())
	if err != nil {
		return fmt.Errorf("deleting from database: %w", err)
	}

	return nil
}

func (r *sqliteRepo) Exists(ctx context.Context, vmid models.VMID) (bool, error) {
	mu := r.getMutex(vmid.String())
	mu.RLock()
	defer mu.RUnlock()

	var exists bool
	err := r.db.QueryRowContext(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM microvms 
			WHERE uid = ? AND namespace = ? AND name = ?
		)
	`, vmid.UID(), vmid.Namespace(), vmid.Name()).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("checking existence: %w", err)
	}

	return exists, nil
}

func (r *sqliteRepo) ReleaseLease(ctx context.Context, microvm *models.MicroVM) error {
	// No-op for SQLite as we don't use leases
	return nil
}

func (r *sqliteRepo) getMutex(name string) *sync.RWMutex {
	r.lockMu.Lock()
	defer r.lockMu.Unlock()

	mu, ok := r.locks[name]
	if !ok {
		mu = &sync.RWMutex{}
		r.locks[name] = mu
	}

	return mu
}
