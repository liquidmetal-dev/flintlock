package overlay

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/liquidmetal-dev/flintlock/pkg/log"
)

// Sweep removes every base that no microvm links to and whose last use is
// older than the retention period. It takes the per-key lock before it
// removes a base and rechecks the references and the last use under the
// lock, so a Create that is linking the base wins. A base file with no
// by-source link is removed when it is older than the retention period
// and unreferenced.
func (s *Service) Sweep(ctx context.Context) error {
	logger := log.GetLogger(ctx).WithField("service", "overlay_sweep")

	entries, err := s.index.Entries()
	if err != nil {
		return err
	}

	referenced, err := s.referenced()
	if err != nil {
		return err
	}

	cutoff := time.Now().Add(-s.retention)
	linked := map[string]bool{}

	var errs []error

	for _, entry := range entries {
		linked[entry.Digest] = true

		if referenced[entry.Digest] || entry.LastUse.After(cutoff) {
			continue
		}

		removed, err := s.sweepEntry(ctx, entry, cutoff)
		if err != nil {
			errs = append(errs, err)

			continue
		}

		if removed {
			logger.Infof("removed unused base %s for %s", entry.Digest, entry.Key)
		}
	}

	orphans, err := s.sweepOrphans(referenced, linked, cutoff)
	if err != nil {
		errs = append(errs, err)
	}

	for _, orphan := range orphans {
		logger.Infof("removed unused base %s with no index entry", orphan)
	}

	return errors.Join(errs...)
}

// referenced returns the digest of every base a microvm links to.
func (s *Service) referenced() (map[string]bool, error) {
	refs, err := s.index.References()
	if err != nil {
		return nil, err
	}

	referenced := make(map[string]bool, len(refs))

	for _, ref := range refs {
		referenced[ref.Digest] = true
	}

	return referenced, nil
}

// sweepEntry removes the base of entry and its by-source link under the
// per-key lock. It keeps the base when, under the lock, a microvm links to
// it or its last use is after cutoff.
func (s *Service) sweepEntry(ctx context.Context, entry Entry, cutoff time.Time) (bool, error) {
	release, err := s.index.Lock(ctx, entry.Key)
	if err != nil {
		return false, err
	}

	defer func() { _ = release() }()

	lastUse, err := s.index.LastUse(entry.Key)
	if err != nil {
		if errors.Is(err, errBaseNotInIndex) {
			return false, nil
		}

		return false, err
	}

	if lastUse.After(cutoff) {
		return false, nil
	}

	referenced, err := s.referenced()
	if err != nil {
		return false, err
	}

	if referenced[entry.Digest] {
		return false, nil
	}

	if err := removeBaseFile(entry.Path); err != nil {
		return false, err
	}

	return true, s.index.Remove(entry.Key)
}

// sweepOrphans removes base files that no index entry and no microvm link
// name, when they are older than cutoff. It returns the digests removed.
func (s *Service) sweepOrphans(referenced, linked map[string]bool, cutoff time.Time) ([]string, error) {
	pattern := filepath.Join(s.root, basesDir, "*"+baseExt)

	files, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("listing %s: %w", pattern, err)
	}

	removed := make([]string, 0)

	for _, file := range files {
		sum, err := digestFromPath(file)
		if err != nil {
			continue
		}

		if referenced[sum] || linked[sum] {
			continue
		}

		info, err := os.Stat(file)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}

			return removed, fmt.Errorf("stat %s: %w", file, err)
		}

		if info.ModTime().After(cutoff) {
			continue
		}

		if err := removeBaseFile(file); err != nil {
			return removed, err
		}

		removed = append(removed, sum)
	}

	return removed, nil
}

// removeBaseFile deletes a base file. A missing file is a success.
func removeBaseFile(path string) error {
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("removing base %s: %w", path, err)
	}

	return nil
}

// ClearScratch removes everything under scratch/ of the state root. The
// daemon calls it once at startup: a build that a crash interrupted left
// its files there, and the kernel dropped its lock.
func ClearScratch(stateRoot string) error {
	dir := filepath.Join(stateRoot, scratchDir)

	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}

		return fmt.Errorf("reading %s: %w", dir, err)
	}

	for _, entry := range entries {
		path := filepath.Join(dir, entry.Name())

		if err := os.RemoveAll(path); err != nil {
			return fmt.Errorf("removing %s: %w", path, err)
		}
	}

	return nil
}
