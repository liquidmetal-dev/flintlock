package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	stderrors "errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/google/go-cmp/cmp"
	_ "modernc.org/sqlite" // registers the "sqlite" database/sql driver

	"github.com/liquidmetal-dev/flintlock/core/errors"
	"github.com/liquidmetal-dev/flintlock/core/models"
	"github.com/liquidmetal-dev/flintlock/core/ports"
	"github.com/liquidmetal-dev/flintlock/pkg/log"
)

// NewMicroVMRepo will create a new sqlite backed microvm repository with the
// supplied sqlite configuration. It will create the database file (and its
// parent directory) if it doesn't already exist, and apply any pending
// schema migrations.
func NewMicroVMRepo(cfg *Config) (ports.MicroVMRepository, error) {
	if cfg == nil {
		return nil, stderrors.New("sqlite repository config must not be nil")
	}

	if err := os.MkdirAll(filepath.Dir(cfg.DatabasePath), 0o750); err != nil {
		return nil, fmt.Errorf("creating directory for sqlite database: %w", err)
	}

	db, err := sql.Open("sqlite", cfg.DatabasePath)
	if err != nil {
		return nil, fmt.Errorf("opening sqlite database %s: %w", cfg.DatabasePath, err)
	}

	// sqlite only supports a single writer at a time; serialise access at the
	// database/sql level so concurrent callers queue rather than error out.
	db.SetMaxOpenConns(1)

	if err := runMigrations(db, migrations); err != nil {
		_ = db.Close()

		return nil, fmt.Errorf("running sqlite migrations: %w", err)
	}

	return NewMicroVMRepoWithDB(db), nil
}

// NewMicroVMRepoWithDB will create a new sqlite backed microvm repository
// with the supplied database handle. The caller is responsible for ensuring
// the schema has already been migrated.
func NewMicroVMRepoWithDB(db *sql.DB) ports.MicroVMRepository {
	return &sqliteRepo{
		db:    db,
		locks: map[string]*sync.RWMutex{},
	}
}

type sqliteRepo struct {
	db *sql.DB

	locks   map[string]*sync.RWMutex
	locksMu sync.Mutex
}

type microvmRow struct {
	uid       string
	version   int
	name      string
	namespace string
	spec      []byte
}

// Save will save the supplied microvm spec to the sqlite database.
func (r *sqliteRepo) Save(ctx context.Context, microvm *models.MicroVM) (*models.MicroVM, error) {
	logger := log.GetLogger(ctx).WithField("repo", "sqlite_microvm")
	logger.Debugf("saving microvm spec %s", microvm.ID)

	mu := r.getMutex(microvm.ID.Namespace(), microvm.ID.Name())
	mu.Lock()
	defer mu.Unlock()

	existingSpec, err := r.get(ctx, ports.RepositoryGetOptions{
		Name:      microvm.ID.Name(),
		Namespace: microvm.ID.Namespace(),
		UID:       microvm.ID.UID(),
	})
	if err != nil {
		return nil, fmt.Errorf("getting vm spec from store: %w", err)
	}

	if existingSpec != nil {
		specDiff := cmp.Diff(existingSpec.Spec, microvm.Spec)
		statusDiff := cmp.Diff(existingSpec.Status, microvm.Status)

		if specDiff == "" && statusDiff == "" {
			logger.Debug("microvm specs have no diff, skipping save")

			return existingSpec, nil
		}
	}

	microvm.Version++

	data, err := json.Marshal(microvm)
	if err != nil {
		return nil, fmt.Errorf("marshalling microvm to json: %w", err)
	}

	_, err = r.db.ExecContext(ctx,
		`INSERT INTO microvms (uid, version, name, namespace, spec) VALUES (?, ?, ?, ?, ?)`,
		microvm.ID.UID(), microvm.Version, microvm.ID.Name(), microvm.ID.Namespace(), data)
	if err != nil {
		return nil, fmt.Errorf("inserting microvm spec into database: %w", err)
	}

	return microvm, nil
}

// Get will get the microvm spec with the given name/namespace from the sqlite database.
// If version is not empty, returns with the specified version of the spec.
func (r *sqliteRepo) Get(ctx context.Context, options ports.RepositoryGetOptions) (*models.MicroVM, error) {
	mu := r.getMutex(options.Namespace, options.Name)
	mu.RLock()
	defer mu.RUnlock()

	spec, err := r.get(ctx, options)
	if err != nil {
		return nil, fmt.Errorf("getting vm spec from store: %w", err)
	}

	if spec == nil {
		return nil, errors.NewSpecNotFound(
			options.Name,
			options.Namespace,
			options.Version,
			options.UID)
	}

	return spec, nil
}

// GetAll will get a list of microvm details from the sqlite database.
func (r *sqliteRepo) GetAll(ctx context.Context, query models.ListMicroVMQuery) ([]*models.MicroVM, error) {
	sqlQuery := `SELECT m.uid, m.version, m.name, m.namespace, m.spec
		FROM microvms m
		INNER JOIN (SELECT uid, MAX(version) AS version FROM microvms GROUP BY uid) latest
			ON m.uid = latest.uid AND m.version = latest.version`

	where, args := whereClauseForQuery(query)

	rows, err := r.db.QueryContext(ctx, sqlQuery+where, args...)
	if err != nil {
		return nil, fmt.Errorf("querying microvms: %w", err)
	}
	defer rows.Close()

	items := []*models.MicroVM{}

	for rows.Next() {
		row, scanErr := scanRow(rows)
		if scanErr != nil {
			return nil, fmt.Errorf("scanning microvm row: %w", scanErr)
		}

		microvm, unmarshalErr := unmarshalRow(row)
		if unmarshalErr != nil {
			return nil, unmarshalErr
		}

		items = append(items, microvm)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating microvm rows: %w", err)
	}

	return items, nil
}

// ReleaseLease is a no-op for the sqlite repository, as sqlite has no
// equivalent concept to a containerd content-store lease.
func (r *sqliteRepo) ReleaseLease(_ context.Context, _ *models.MicroVM) error {
	return nil
}

// Delete will delete all versions of the supplied microvm from the sqlite database.
func (r *sqliteRepo) Delete(ctx context.Context, microvm *models.MicroVM) error {
	mu := r.getMutex(microvm.ID.Namespace(), microvm.ID.Name())
	mu.Lock()
	defer mu.Unlock()

	_, err := r.db.ExecContext(ctx,
		`DELETE FROM microvms WHERE name = ? AND namespace = ?`,
		microvm.ID.Name(), microvm.ID.Namespace())
	if err != nil {
		return fmt.Errorf("deleting microvm %s from database: %w", microvm.ID, err)
	}

	return nil
}

// Exists checks to see if the microvm spec exists in the sqlite database.
func (r *sqliteRepo) Exists(ctx context.Context, vmid models.VMID) (bool, error) {
	mu := r.getMutex(vmid.Namespace(), vmid.Name())
	mu.RLock()
	defer mu.RUnlock()

	spec, err := r.get(ctx, ports.RepositoryGetOptions{
		Name:      vmid.Name(),
		Namespace: vmid.Namespace(),
		UID:       vmid.UID(),
	})
	if err != nil {
		return false, fmt.Errorf("finding microvm %s: %w", vmid.String(), err)
	}

	return spec != nil, nil
}

func (r *sqliteRepo) get(ctx context.Context, options ports.RepositoryGetOptions) (*models.MicroVM, error) {
	sqlQuery := `SELECT uid, version, name, namespace, spec FROM microvms`

	where, args := whereClauseForOptions(options)

	row := r.db.QueryRowContext(ctx, sqlQuery+where+" ORDER BY version DESC LIMIT 1", args...)

	rowData, err := scanRow(row)
	if err != nil {
		if stderrors.Is(err, sql.ErrNoRows) {
			//nolint: nilnil // absence is (nil, nil), matching the containerd repo's internal get()
			return nil, nil
		}

		return nil, fmt.Errorf("querying microvm: %w", err)
	}

	return unmarshalRow(rowData)
}

func (r *sqliteRepo) getMutex(namespace, name string) *sync.RWMutex {
	r.locksMu.Lock()
	defer r.locksMu.Unlock()

	key := namespace + "/" + name

	namedMu, ok := r.locks[key]
	if ok {
		return namedMu
	}

	mu := &sync.RWMutex{}
	r.locks[key] = mu

	return mu
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanRow(scanner rowScanner) (microvmRow, error) {
	var row microvmRow

	err := scanner.Scan(&row.uid, &row.version, &row.name, &row.namespace, &row.spec)

	return row, err
}

func unmarshalRow(row microvmRow) (*models.MicroVM, error) {
	microvm := &models.MicroVM{}

	if err := json.Unmarshal(row.spec, microvm); err != nil {
		return nil, fmt.Errorf("unmarshalling json content to microvm: %w", err)
	}

	return microvm, nil
}

func whereClauseForOptions(options ports.RepositoryGetOptions) (string, []any) {
	conditions := []string{}
	args := []any{}

	if options.Name != "" {
		conditions = append(conditions, "name = ?")
		args = append(args, options.Name)
	}

	if options.Namespace != "" {
		conditions = append(conditions, "namespace = ?")
		args = append(args, options.Namespace)
	}

	if options.UID != "" {
		conditions = append(conditions, "uid = ?")
		args = append(args, options.UID)
	}

	if options.Version != "" {
		conditions = append(conditions, "version = ?")
		args = append(args, options.Version)
	}

	if len(conditions) == 0 {
		return "", args
	}

	return " WHERE " + strings.Join(conditions, " AND "), args
}

func whereClauseForQuery(query models.ListMicroVMQuery) (string, []any) {
	conditions := []string{}
	args := []any{}

	if namespace, ok := query["namespace"]; ok && namespace != "" {
		conditions = append(conditions, "m.namespace = ?")
		args = append(args, namespace)
	}

	if name, ok := query["name"]; ok && name != "" {
		conditions = append(conditions, "m.name = ?")
		args = append(args, name)
	}

	if len(conditions) == 0 {
		return "", args
	}

	return " WHERE " + strings.Join(conditions, " AND "), args
}
