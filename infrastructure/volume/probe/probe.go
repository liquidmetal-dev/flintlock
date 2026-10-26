// Package probe checks at startup which volume modes a host accepts. The
// result is given to every microvm provider, which advertises it as the
// capabilities volume-overlay and volume-full.
package probe

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"

	"github.com/liquidmetal-dev/flintlock/core/models"
	"github.com/liquidmetal-dev/flintlock/infrastructure/containerd"
	"github.com/liquidmetal-dev/flintlock/infrastructure/volume/overlay"
	"github.com/liquidmetal-dev/flintlock/pkg/defaults"
	"github.com/liquidmetal-dev/flintlock/pkg/log"
)

var errNoProbe = errors.New("no probe configured")

// Probes are the checks of one host. Each returns nil when the host passes.
// volume-overlay needs Reflink, MkfsErofs and MkfsExt4. volume-full needs
// Snapshotter.
type Probes struct {
	// Reflink checks that FICLONE works in the state directory.
	Reflink func(ctx context.Context) error
	// MkfsErofs checks that the pinned mkfs.erofs is present and reports
	// the version of the base profile.
	MkfsErofs func(ctx context.Context) error
	// MkfsExt4 checks that mkfs.ext4 is present.
	MkfsExt4 func(ctx context.Context) error
	// Snapshotter checks that containerd loaded the volume snapshotter.
	Snapshotter func(ctx context.Context) error
}

// Host returns the probes of this host. The reflink probe runs in
// stateDir and creates it when needed. The snapshotter probe asks
// containerd through client about the snapshotter named in cfg.
func Host(stateDir, mkfsErofsPath string, cfg *containerd.Config, client containerd.Client) Probes {
	return Probes{
		Reflink: func(context.Context) error {
			if err := os.MkdirAll(stateDir, defaults.DataDirPerm); err != nil {
				return fmt.Errorf("creating state directory %s: %w", stateDir, err)
			}

			return overlay.ProbeReflink(stateDir)
		},
		MkfsErofs: func(ctx context.Context) error {
			return overlay.CheckMkfsErofs(ctx, mkfsErofsPath)
		},
		MkfsExt4: func(context.Context) error {
			if _, err := exec.LookPath(overlay.MkfsExt4); err != nil {
				return fmt.Errorf("finding %s: %w", overlay.MkfsExt4, err)
			}

			return nil
		},
		Snapshotter: func(ctx context.Context) error {
			return containerd.ProbeSnapshotter(ctx, cfg, client, cfg.SnapshotterVolume)
		},
	}
}

// Run runs the probes and returns the volume capabilities the host has.
// Every probe that fails is logged with its reason.
func Run(ctx context.Context, probes Probes) models.Capabilities {
	logger := log.GetLogger(ctx).WithField("component", "host_probe")

	var caps models.Capabilities

	overlayOK := true

	for _, p := range []struct {
		name string
		run  func(ctx context.Context) error
	}{
		{"reflink", probes.Reflink},
		{"mkfs.erofs", probes.MkfsErofs},
		{"mkfs.ext4", probes.MkfsExt4},
	} {
		if err := run(ctx, p.run); err != nil {
			logger.Warnf("%s probe failed, %s is off: %v", p.name, models.VolumeOverlayCapability, err)

			overlayOK = false
		}
	}

	if overlayOK {
		caps = append(caps, models.VolumeOverlayCapability)
	}

	if err := run(ctx, probes.Snapshotter); err != nil {
		logger.Warnf("snapshotter probe failed, %s is off: %v", models.VolumeFullCapability, err)
	} else {
		caps = append(caps, models.VolumeFullCapability)
	}

	logger.Infof("host volume capabilities: %v", caps)

	return caps
}

func run(ctx context.Context, p func(ctx context.Context) error) error {
	if p == nil {
		return errNoProbe
	}

	return p(ctx)
}
