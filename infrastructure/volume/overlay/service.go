package overlay

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/liquidmetal-dev/flintlock/core/models"
	"github.com/liquidmetal-dev/flintlock/core/ports"
	"github.com/liquidmetal-dev/flintlock/pkg/log"
)

const (
	volumesDir   = "volumes"
	baseLinkName = "root.base"
	rwDiskName   = "root.rw"

	// MkfsExt4 is the default mkfs.ext4, found on PATH.
	MkfsExt4 = "mkfs.ext4"
)

// Config configures the overlay volume service.
type Config struct {
	// StateRoot is the state directory of the daemon. Bases, templates,
	// scratch files and the volumes of each microvm live under it.
	StateRoot string
	// MkfsErofsPath is the pinned mkfs.erofs the base builder runs.
	MkfsErofsPath string
	// MkfsExt4Path is the mkfs.ext4 that formats writable disk templates.
	// An empty value means MkfsExt4 on PATH.
	MkfsExt4Path string
	// DefaultRWSizeMB is the size of a writable disk in megabytes when the
	// spec gives none.
	DefaultRWSizeMB int32
	// BaseRetention is how long a base with no microvm link is kept after
	// its last use.
	BaseRetention time.Duration
}

// baseBuilder is what the service needs from the base builder.
type baseBuilder interface {
	Build(ctx context.Context, in LayerInput) (*Base, error)
}

// cloneFunc makes dst a reflink of src.
type cloneFunc func(src, dst string, perm os.FileMode) error

// Service is the overlay implementation of ports.VolumeService.
type Service struct {
	root          string
	builder       baseBuilder
	index         baseIndex
	templates     *templates
	clone         cloneFunc
	defaultSizeMB int32
	retention     time.Duration
}

var _ ports.VolumeService = (*Service)(nil)

// NewService returns a Service over the state root of cfg that builds
// bases from source. It creates the directories of the on-host layout.
func NewService(cfg Config, source LayerSource) (*Service, error) {
	builder, err := NewBuilder(cfg.StateRoot, cfg.MkfsErofsPath, source)
	if err != nil {
		return nil, err
	}

	mkfsExt4Path := cfg.MkfsExt4Path
	if mkfsExt4Path == "" {
		mkfsExt4Path = MkfsExt4
	}

	tmpl, err := newTemplates(builder.stateRoot, mkfsExt4Path)
	if err != nil {
		return nil, err
	}

	return &Service{
		root:          builder.stateRoot,
		builder:       builder,
		index:         builder.index,
		templates:     tmpl,
		clone:         cloneFile,
		defaultSizeMB: cfg.DefaultRWSizeMB,
		retention:     cfg.BaseRetention,
	}, nil
}

// Create builds or finds the base of the image, links it as root.base in
// the volumes directory of the microvm, and reflinks the writable disk
// template of the requested size to root.rw. A root.rw that exists is kept:
// it can hold guest writes. A root.base that resolves to the base is kept.
func (s *Service) Create(ctx context.Context, in ports.VolumeCreateInput) (*ports.VolumeMounts, error) {
	logger := s.logger(ctx, in.VolumeRef)

	sizeMB := in.SizeMB
	if sizeMB == 0 {
		sizeMB = s.defaultSizeMB
	}

	if sizeMB < models.OverlayMinWritableDiskMB {
		return nil, fmt.Errorf("%d MB: %w (%d MB)", sizeMB, ErrDiskTooSmall, models.OverlayMinWritableDiskMB)
	}

	base, err := s.builder.Build(ctx, LayerInput{ImageRef: in.ImageRef, Owner: in.Owner})
	if err != nil {
		return nil, err
	}

	dir := s.volumesDir(in.VMID)

	if err := os.MkdirAll(dir, dirPerm); err != nil {
		return nil, fmt.Errorf("creating %s: %w", dir, err)
	}

	linkPath := filepath.Join(dir, baseLinkName)

	if err := linkBase(linkPath, base.Path); err != nil {
		return nil, err
	}

	rwPath := filepath.Join(dir, rwDiskName)

	made, err := s.makeWritable(ctx, rwPath, sizeMB)
	if err != nil {
		return nil, err
	}

	if made {
		logger.Infof("created writable disk %s of %d MB on base %s", rwPath, sizeMB, base.Digest)
	} else {
		logger.Debugf("writable disk %s exists, kept", rwPath)
	}

	return &ports.VolumeMounts{
		BaseMount:  models.Mount{Type: models.MountTypeFile, Source: linkPath},
		RWMount:    models.Mount{Type: models.MountTypeFile, Source: rwPath},
		BaseDigest: base.Digest,
	}, nil
}

// Release removes the writable disk and the base link of the microvm. A
// file that is already gone is a success. It then sweeps unused bases.
func (s *Service) Release(ctx context.Context, ref ports.VolumeRef) error {
	logger := s.logger(ctx, ref)
	dir := s.volumesDir(ref.VMID)

	for _, name := range []string{rwDiskName, baseLinkName} {
		path := filepath.Join(dir, name)

		err := os.Remove(path)
		if err == nil {
			logger.Infof("removed %s", path)

			continue
		}

		if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("removing %s: %w", path, err)
		}
	}

	return s.Sweep(ctx)
}

// volumesDir is the volumes directory of a microvm:
// <state root>/vm/<ns>/<name>/<uid>/volumes.
func (s *Service) volumesDir(vmid models.VMID) string {
	return filepath.Join(s.root, vmDir, vmid.String(), volumesDir)
}

func (s *Service) logger(ctx context.Context, ref ports.VolumeRef) *logrus.Entry {
	return log.GetLogger(ctx).WithFields(logrus.Fields{
		"service": "overlay_volume",
		"vmid":    ref.VMID.String(),
		"volume":  ref.VolumeID,
	})
}

// linkBase makes link a relative symlink to the base file at target. A link
// that resolves to target is kept. Any other link is replaced with a rename,
// so a reader never sees a missing link.
func linkBase(link, target string) error {
	dir := filepath.Dir(link)

	current, err := os.Readlink(link)
	if err == nil {
		if !filepath.IsAbs(current) {
			current = filepath.Join(dir, current)
		}

		if current == target {
			return nil
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("reading link %s: %w", link, err)
	}

	rel, err := filepath.Rel(dir, target)
	if err != nil {
		return fmt.Errorf("relative link target for %s: %w", target, err)
	}

	tmp, err := os.CreateTemp(dir, "."+baseLinkName+".tmp-")
	if err != nil {
		return fmt.Errorf("creating temporary link name: %w", err)
	}

	tmpName := tmp.Name()
	_ = tmp.Close()
	_ = os.Remove(tmpName)

	if err := os.Symlink(rel, tmpName); err != nil {
		return fmt.Errorf("creating link %s: %w", tmpName, err)
	}

	if err := os.Rename(tmpName, link); err != nil {
		_ = os.Remove(tmpName)

		return fmt.Errorf("renaming link to %s: %w", link, err)
	}

	return syncDir(dir)
}

// makeWritable reflinks the template of sizeMB to path unless path exists.
// It clones to a temporary name and renames, so a crash never leaves a
// half-made disk that a later Create would keep. It reports whether it
// made the disk.
func (s *Service) makeWritable(ctx context.Context, path string, sizeMB int32) (bool, error) {
	if _, err := os.Lstat(path); err == nil {
		return false, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return false, fmt.Errorf("stat %s: %w", path, err)
	}

	template, err := s.templates.get(ctx, sizeMB)
	if err != nil {
		return false, err
	}

	dir := filepath.Dir(path)

	tmp, err := os.CreateTemp(dir, "."+rwDiskName+".tmp-")
	if err != nil {
		return false, fmt.Errorf("creating temporary disk name: %w", err)
	}

	tmpName := tmp.Name()
	_ = tmp.Close()
	_ = os.Remove(tmpName)

	if err := s.clone(template, tmpName, diskPerm); err != nil {
		return false, err
	}

	if err := os.Rename(tmpName, path); err != nil {
		_ = os.Remove(tmpName)

		return false, fmt.Errorf("renaming disk to %s: %w", path, err)
	}

	return true, syncDir(dir)
}
