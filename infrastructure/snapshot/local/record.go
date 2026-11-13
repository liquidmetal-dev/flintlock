package local

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/liquidmetal-dev/flintlock/core/models"
)

// record is the content of record.json. Generation and Deleted are state of
// the store, so they are not part of models.Snapshot.
type record struct {
	UID        string               `json:"uid"`
	Name       string               `json:"name"`
	Namespace  string               `json:"namespace"`
	Source     recordSource         `json:"source"`
	Labels     map[string]string    `json:"labels,omitempty"`
	Phase      models.SnapshotPhase `json:"phase"`
	Error      string               `json:"error,omitempty"`
	CreatedAt  time.Time            `json:"created_at"`
	Generation int                  `json:"generation"`
	Deleted    bool                 `json:"deleted"`
}

type recordSource struct {
	UID       string `json:"uid"`
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
}

func recordFromModel(snapshot *models.Snapshot) *record {
	return &record{
		UID:       snapshot.UID,
		Name:      snapshot.Name,
		Namespace: snapshot.Namespace,
		Source: recordSource{
			UID:       snapshot.Source.UID(),
			Name:      snapshot.Source.Name(),
			Namespace: snapshot.Source.Namespace(),
		},
		Labels:    snapshot.Labels,
		Phase:     snapshot.Status.Phase,
		Error:     snapshot.Status.Error,
		CreatedAt: snapshot.CreatedAt,
	}
}

func (r *record) toModel(dir string) *models.Snapshot {
	snapshot := &models.Snapshot{
		UID:       r.UID,
		Name:      r.Name,
		Namespace: r.Namespace,
		Source:    *models.NewVMIDForce(r.Source.Name, r.Source.Namespace, r.Source.UID),
		Labels:    r.Labels,
		Status: models.SnapshotStatus{
			Phase: r.Phase,
			Error: r.Error,
		},
		CreatedAt: r.CreatedAt,
	}

	if r.Phase == models.SnapshotPhaseReady {
		snapshot.Status.LocalPath = dir
	}

	return snapshot
}

func readRecord(dir string) (*record, error) {
	path := filepath.Join(dir, recordFile)

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}

	rec := &record{}
	if err := json.Unmarshal(data, rec); err != nil {
		return nil, fmt.Errorf("decoding %s: %w", path, err)
	}

	return rec, nil
}

// writeRecord replaces record.json with a temporary file and a rename, so a
// reader never sees a partly written record.
func writeRecord(dir string, rec *record) error {
	data, err := json.MarshalIndent(rec, "", "  ")
	if err != nil {
		return fmt.Errorf("encoding record %s: %w", rec.UID, err)
	}

	tmp, err := os.CreateTemp(dir, "."+recordFile+".tmp-")
	if err != nil {
		return fmt.Errorf("creating temporary record in %s: %w", dir, err)
	}

	tmpName := tmp.Name()

	if err := writeAndClose(tmp, data); err != nil {
		_ = os.Remove(tmpName)

		return err
	}

	path := filepath.Join(dir, recordFile)
	if err := os.Rename(tmpName, path); err != nil {
		_ = os.Remove(tmpName)

		return fmt.Errorf("renaming record to %s: %w", path, err)
	}

	return syncDir(dir)
}

func writeAndClose(f *os.File, data []byte) error {
	if err := f.Chmod(filePerm); err != nil {
		_ = f.Close()

		return fmt.Errorf("setting mode of %s: %w", f.Name(), err)
	}

	if _, err := f.Write(data); err != nil {
		_ = f.Close()

		return fmt.Errorf("writing %s: %w", f.Name(), err)
	}

	if err := f.Sync(); err != nil {
		_ = f.Close()

		return fmt.Errorf("syncing %s: %w", f.Name(), err)
	}

	if err := f.Close(); err != nil {
		return fmt.Errorf("closing %s: %w", f.Name(), err)
	}

	return nil
}

func syncDir(dir string) error {
	d, err := os.Open(dir)
	if err != nil {
		return fmt.Errorf("opening %s: %w", dir, err)
	}

	defer d.Close()

	if err := d.Sync(); err != nil {
		return fmt.Errorf("syncing %s: %w", dir, err)
	}

	return nil
}
