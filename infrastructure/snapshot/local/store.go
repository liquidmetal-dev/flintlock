// Package local is the store of snapshots on the host: a plain directory per
// snapshot under the state root, with a JSON record that survives a restart.
// Store implements ports.SnapshotRepository.
//
// The layout under the state root is:
//
//	snapshots/.lock                 flock for a create and the startup scan
//	snapshots/<uuid>/               mode 0700
//	snapshots/<uuid>/record.json    mode 0600
//	snapshots/<uuid>/refs/<vm-uid>  one empty file per restored microvm that maps memory
//	snapshots/<uuid>/.lock          flock for every change to record.json or refs/
//
// The other files of a snapshot directory are written by the capture.
package local

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/google/uuid"

	coreerrs "github.com/liquidmetal-dev/flintlock/core/errors"
	"github.com/liquidmetal-dev/flintlock/core/models"
	"github.com/liquidmetal-dev/flintlock/core/ports"
	"github.com/liquidmetal-dev/flintlock/pkg/log"
)

const (
	snapshotsDir = "snapshots"
	recordFile   = "record.json"
	refsDir      = "refs"
	lockName     = ".lock"
	vmstateFile  = models.SnapshotVMStateFile
	memoryFile   = models.SnapshotMemoryFile

	dirPerm  = 0o700
	filePerm = 0o600

	nameTimeFormat = "20060102-150405"
)

var (
	errSnapshotRequired = errors.New("snapshot is required")
	errInvalidUID       = errors.New("snapshot uid is not a uuid")
	errInvalidVMUID     = errors.New("microvm uid is not a file name")
)

// retainedFiles are the files a deleted snapshot keeps while a restored
// microvm references it.
var retainedFiles = map[string]bool{
	recordFile:  true,
	refsDir:     true,
	lockName:    true,
	vmstateFile: true,
	memoryFile:  true,
}

// Store is the snapshot store in the snapshots directory of a state root.
type Store struct {
	root string
}

var _ ports.SnapshotRepository = (*Store)(nil)

// New returns the store under stateRoot. It creates the snapshots directory
// and removes the directories that an interrupted create or delete left.
func New(ctx context.Context, stateRoot string) (*Store, error) {
	root := filepath.Join(stateRoot, snapshotsDir)

	if err := os.MkdirAll(root, dirPerm); err != nil {
		return nil, fmt.Errorf("creating %s: %w", root, err)
	}

	if err := os.Chmod(root, dirPerm); err != nil {
		return nil, fmt.Errorf("setting mode of %s: %w", root, err)
	}

	store := &Store{root: root}

	if err := store.scan(ctx); err != nil {
		return nil, err
	}

	return store, nil
}

// Dir returns the directory of the snapshot with the given uid.
func (s *Store) Dir(uid string) string {
	return filepath.Join(s.root, uid)
}

// Create makes the directory and a pending record for the snapshot.
func (s *Store) Create(ctx context.Context, snapshot *models.Snapshot) (err error) {
	if snapshot == nil {
		return errSnapshotRequired
	}

	if err := setCreateDefaults(snapshot); err != nil {
		return err
	}

	release, err := lockFile(ctx, filepath.Join(s.root, lockName))
	if err != nil {
		return err
	}

	defer unlock(release, &err)

	if _, err := s.find(snapshot.Namespace, snapshot.Name); err == nil {
		return coreerrs.NewSnapshotAlreadyExists(snapshot.Namespace + "/" + snapshot.Name)
	} else if !coreerrs.IsSnapshotNotFound(err) {
		return err
	}

	dir := s.Dir(snapshot.UID)

	if err := os.Mkdir(dir, dirPerm); err != nil {
		if errors.Is(err, os.ErrExist) {
			return coreerrs.NewSnapshotAlreadyExists(snapshot.UID)
		}

		return fmt.Errorf("creating %s: %w", dir, err)
	}

	if err := populate(dir, recordFromModel(snapshot)); err != nil {
		_ = os.RemoveAll(dir)

		return err
	}

	return nil
}

// Get returns the snapshot with the uid of ref, or with its name and namespace.
func (s *Store) Get(_ context.Context, ref models.SnapshotRef) (*models.Snapshot, error) {
	if ref.UID != "" {
		rec, err := s.read(ref.UID)
		if err != nil {
			return nil, err
		}

		if rec.Deleted {
			return nil, coreerrs.NewSnapshotNotFound(ref.UID)
		}

		return rec.toModel(s.Dir(rec.UID)), nil
	}

	if ref.Name == "" {
		return nil, coreerrs.ErrNameRequired
	}

	if ref.Namespace == "" {
		return nil, coreerrs.ErrNamespaceRequired
	}

	rec, err := s.find(ref.Namespace, ref.Name)
	if err != nil {
		return nil, err
	}

	return rec.toModel(s.Dir(rec.UID)), nil
}

// List returns the snapshots that match the options, oldest first.
func (s *Store) List(_ context.Context, options ports.SnapshotListOptions) ([]*models.Snapshot, error) {
	recs, err := s.records()
	if err != nil {
		return nil, err
	}

	snapshots := []*models.Snapshot{}

	for _, rec := range recs {
		if rec.Deleted || !matches(rec, options) {
			continue
		}

		snapshots = append(snapshots, rec.toModel(s.Dir(rec.UID)))
	}

	sort.Slice(snapshots, func(i, j int) bool {
		if snapshots[i].CreatedAt.Equal(snapshots[j].CreatedAt) {
			return snapshots[i].UID < snapshots[j].UID
		}

		return snapshots[i].CreatedAt.Before(snapshots[j].CreatedAt)
	})

	return snapshots, nil
}

// Update saves the phase and error of the snapshot.
func (s *Store) Update(ctx context.Context, snapshot *models.Snapshot) (err error) {
	if snapshot == nil {
		return errSnapshotRequired
	}

	rec, release, err := s.lock(ctx, snapshot.UID)
	if err != nil {
		return err
	}

	defer unlock(release, &err)

	rec.Phase = snapshot.Status.Phase
	rec.Error = snapshot.Status.Error

	return writeRecord(s.Dir(rec.UID), rec)
}

// NextGeneration increments the restore generation of the snapshot and
// returns the new value. The first value is 1.
func (s *Store) NextGeneration(ctx context.Context, uid string) (_ int, err error) {
	rec, release, err := s.lock(ctx, uid)
	if err != nil {
		return 0, err
	}

	defer unlock(release, &err)

	rec.Generation++

	if err := writeRecord(s.Dir(uid), rec); err != nil {
		return 0, err
	}

	return rec.Generation, nil
}

// AddRef creates refs/<vmUID> in the directory of the snapshot.
func (s *Store) AddRef(ctx context.Context, uid, vmUID string) (err error) {
	if err := validateVMUID(vmUID); err != nil {
		return err
	}

	_, release, err := s.lock(ctx, uid)
	if err != nil {
		return err
	}

	defer unlock(release, &err)

	refs := filepath.Join(s.Dir(uid), refsDir)
	path := filepath.Join(refs, vmUID)

	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY, filePerm)
	if err != nil {
		return fmt.Errorf("creating reference %s: %w", path, err)
	}

	if err := f.Close(); err != nil {
		return fmt.Errorf("closing reference %s: %w", path, err)
	}

	return syncDir(refs)
}

// ReleaseRef removes refs/<vmUID>. It removes the directory of a deleted
// snapshot when no reference is left.
func (s *Store) ReleaseRef(ctx context.Context, uid, vmUID string) (err error) {
	if err := validateVMUID(vmUID); err != nil {
		return err
	}

	rec, release, err := s.lockAny(ctx, uid)
	if err != nil {
		return err
	}

	defer unlock(release, &err)

	dir := s.Dir(uid)
	path := filepath.Join(dir, refsDir, vmUID)

	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("removing reference %s: %w", path, err)
	}

	if !rec.Deleted {
		return nil
	}

	count, err := countRefs(dir)
	if err != nil {
		return err
	}

	if count > 0 {
		return nil
	}

	return removePath(dir)
}

// Delete marks the snapshot deleted. With no reference it removes the
// directory. With references it keeps the record, the VM state, the memory
// and the references, and removes the rest.
func (s *Store) Delete(ctx context.Context, uid string) (err error) {
	rec, release, err := s.lock(ctx, uid)
	if err != nil {
		return err
	}

	defer unlock(release, &err)

	dir := s.Dir(uid)

	count, err := countRefs(dir)
	if err != nil {
		return err
	}

	rec.Deleted = true

	if err := writeRecord(dir, rec); err != nil {
		return err
	}

	if count == 0 {
		return removePath(dir)
	}

	return removeUnretained(dir)
}

// scan removes a directory with no record, which an interrupted create
// leaves, and a deleted snapshot with no reference, which an interrupted
// delete leaves.
func (s *Store) scan(ctx context.Context) (err error) {
	logger := log.GetLogger(ctx).WithField("service", "snapshot_store")

	release, err := lockFile(ctx, filepath.Join(s.root, lockName))
	if err != nil {
		return err
	}

	defer unlock(release, &err)

	entries, err := os.ReadDir(s.root)
	if err != nil {
		return fmt.Errorf("reading %s: %w", s.root, err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		dir := s.Dir(entry.Name())

		action, err := recoveryFor(dir)
		if err != nil {
			return err
		}

		switch action {
		case recoveryNone:
		case recoveryRemove:
			if err := removePath(dir); err != nil {
				return err
			}

			logger.Infof("removed left over snapshot directory %s", dir)
		case recoveryPrune:
			if err := s.prune(ctx, dir); err != nil {
				return err
			}
		}
	}

	return nil
}

// prune finishes a Delete that stopped after it marked a referenced snapshot
// deleted, which a retry of Delete cannot do as it no longer finds the snapshot.
func (s *Store) prune(ctx context.Context, dir string) (err error) {
	release, err := lockFile(ctx, filepath.Join(dir, lockName))
	if err != nil {
		return err
	}

	defer unlock(release, &err)

	return removeUnretained(dir)
}

type recovery int

const (
	recoveryNone recovery = iota
	recoveryRemove
	recoveryPrune
)

func recoveryFor(dir string) (recovery, error) {
	rec, err := readRecord(dir)
	if errors.Is(err, os.ErrNotExist) {
		return recoveryRemove, nil
	}

	if err != nil {
		return recoveryNone, err
	}

	if !rec.Deleted {
		return recoveryNone, nil
	}

	count, err := countRefs(dir)
	if err != nil {
		return recoveryNone, err
	}

	if count == 0 {
		return recoveryRemove, nil
	}

	return recoveryPrune, nil
}

// lock takes the lock of a snapshot that is not deleted and reads its record.
func (s *Store) lock(ctx context.Context, uid string) (*record, func() error, error) {
	rec, release, err := s.lockAny(ctx, uid)
	if err != nil {
		return nil, nil, err
	}

	if rec.Deleted {
		_ = release()

		return nil, nil, coreerrs.NewSnapshotNotFound(uid)
	}

	return rec, release, nil
}

// lockAny takes the lock of a snapshot, deleted or not, and reads its record.
func (s *Store) lockAny(ctx context.Context, uid string) (*record, func() error, error) {
	if !validUID(uid) {
		return nil, nil, coreerrs.NewSnapshotNotFound(uid)
	}

	dir := s.Dir(uid)

	release, err := lockFile(ctx, filepath.Join(dir, lockName))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil, coreerrs.NewSnapshotNotFound(uid)
		}

		return nil, nil, err
	}

	// The directory can be removed between opening the lock and taking it.
	rec, err := readRecord(dir)
	if err != nil {
		_ = release()

		if errors.Is(err, os.ErrNotExist) {
			return nil, nil, coreerrs.NewSnapshotNotFound(uid)
		}

		return nil, nil, err
	}

	return rec, release, nil
}

func (s *Store) read(uid string) (*record, error) {
	if !validUID(uid) {
		return nil, coreerrs.NewSnapshotNotFound(uid)
	}

	rec, err := readRecord(s.Dir(uid))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, coreerrs.NewSnapshotNotFound(uid)
		}

		return nil, err
	}

	return rec, nil
}

// find returns the snapshot that is not deleted with the name in the namespace.
func (s *Store) find(namespace, name string) (*record, error) {
	recs, err := s.records()
	if err != nil {
		return nil, err
	}

	for _, rec := range recs {
		if !rec.Deleted && rec.Namespace == namespace && rec.Name == name {
			return rec, nil
		}
	}

	return nil, coreerrs.NewSnapshotNotFound(namespace + "/" + name)
}

// records reads every record. A directory without a record is a create in
// progress, so it is skipped.
func (s *Store) records() ([]*record, error) {
	entries, err := os.ReadDir(s.root)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", s.root, err)
	}

	recs := []*record{}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		rec, err := readRecord(s.Dir(entry.Name()))
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}

			return nil, err
		}

		recs = append(recs, rec)
	}

	return recs, nil
}

func setCreateDefaults(snapshot *models.Snapshot) error {
	if snapshot.UID == "" {
		snapshot.UID = uuid.NewString()
	} else if !validUID(snapshot.UID) {
		return fmt.Errorf("%q: %w", snapshot.UID, errInvalidUID)
	}

	if snapshot.CreatedAt.IsZero() {
		snapshot.CreatedAt = time.Now()
	}

	snapshot.CreatedAt = snapshot.CreatedAt.UTC()

	if snapshot.Namespace == "" {
		snapshot.Namespace = snapshot.Source.Namespace()
	}

	if snapshot.Namespace == "" {
		return coreerrs.ErrNamespaceRequired
	}

	if snapshot.Name == "" {
		if snapshot.Source.Name() == "" {
			return coreerrs.ErrNameRequired
		}

		snapshot.Name = snapshot.Source.Name() + "-" + snapshot.CreatedAt.Format(nameTimeFormat)
	}

	snapshot.Status = models.SnapshotStatus{Phase: models.SnapshotPhasePending}

	return nil
}

// populate writes the record last, so the startup scan removes a directory
// whose populate did not finish.
func populate(dir string, rec *record) error {
	refs := filepath.Join(dir, refsDir)
	if err := os.Mkdir(refs, dirPerm); err != nil {
		return fmt.Errorf("creating %s: %w", refs, err)
	}

	lock := filepath.Join(dir, lockName)
	if err := os.WriteFile(lock, nil, filePerm); err != nil {
		return fmt.Errorf("creating %s: %w", lock, err)
	}

	return writeRecord(dir, rec)
}

func matches(rec *record, options ports.SnapshotListOptions) bool {
	if options.Namespace != "" && rec.Namespace != options.Namespace {
		return false
	}

	if options.SourceUID != "" && rec.Source.UID != options.SourceUID {
		return false
	}

	for key, value := range options.Labels {
		if got, ok := rec.Labels[key]; !ok || got != value {
			return false
		}
	}

	return true
}

func countRefs(dir string) (int, error) {
	refs := filepath.Join(dir, refsDir)

	entries, err := os.ReadDir(refs)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return 0, nil
		}

		return 0, fmt.Errorf("reading %s: %w", refs, err)
	}

	return len(entries), nil
}

func removeUnretained(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("reading %s: %w", dir, err)
	}

	for _, entry := range entries {
		if retainedFiles[entry.Name()] {
			continue
		}

		if err := removePath(filepath.Join(dir, entry.Name())); err != nil {
			return err
		}
	}

	return nil
}

func removePath(path string) error {
	if err := os.RemoveAll(path); err != nil {
		return fmt.Errorf("removing %s: %w", path, err)
	}

	return nil
}

func unlock(release func() error, err *error) {
	if releaseErr := release(); releaseErr != nil && *err == nil {
		*err = releaseErr
	}
}

// validUID accepts only the canonical form of a uuid, so a uid is always
// one directory name below the snapshots directory.
func validUID(uid string) bool {
	parsed, err := uuid.Parse(uid)

	return err == nil && parsed.String() == uid
}

func validateVMUID(vmUID string) error {
	if vmUID == "" || vmUID == "." || vmUID == ".." || filepath.Base(vmUID) != vmUID {
		return fmt.Errorf("%q: %w", vmUID, errInvalidVMUID)
	}

	return nil
}
