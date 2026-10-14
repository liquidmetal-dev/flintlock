package probe_test

import (
	"context"
	"errors"
	"testing"

	. "github.com/onsi/gomega"

	"github.com/liquidmetal-dev/flintlock/core/models"
	"github.com/liquidmetal-dev/flintlock/infrastructure/volume/overlay"
	"github.com/liquidmetal-dev/flintlock/infrastructure/volume/probe"
)

func TestRun(t *testing.T) {
	errFail := errors.New("probe failed")

	pass := func(context.Context) error { return nil }
	fail := func(context.Context) error { return errFail }

	tests := []struct {
		name   string
		probes probe.Probes
		want   models.Capabilities
	}{
		{
			name:   "reflink state directory and thin pool",
			probes: probe.Probes{Reflink: pass, MkfsErofs: pass, MkfsExt4: pass, Snapshotter: pass},
			want:   models.Capabilities{models.VolumeOverlayCapability, models.VolumeFullCapability},
		},
		{
			name:   "thin pool only",
			probes: probe.Probes{Reflink: fail, MkfsErofs: pass, MkfsExt4: pass, Snapshotter: pass},
			want:   models.Capabilities{models.VolumeFullCapability},
		},
		{
			name:   "no mkfs.erofs",
			probes: probe.Probes{Reflink: pass, MkfsErofs: fail, MkfsExt4: pass, Snapshotter: pass},
			want:   models.Capabilities{models.VolumeFullCapability},
		},
		{
			name:   "no mkfs.ext4",
			probes: probe.Probes{Reflink: pass, MkfsErofs: pass, MkfsExt4: fail, Snapshotter: pass},
			want:   models.Capabilities{models.VolumeFullCapability},
		},
		{
			name:   "reflink state directory only",
			probes: probe.Probes{Reflink: pass, MkfsErofs: pass, MkfsExt4: pass, Snapshotter: fail},
			want:   models.Capabilities{models.VolumeOverlayCapability},
		},
		{
			name:   "nothing",
			probes: probe.Probes{Reflink: fail, MkfsErofs: fail, MkfsExt4: fail, Snapshotter: fail},
			want:   nil,
		},
		{
			name:   "missing probes fail",
			probes: probe.Probes{},
			want:   nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)

			got := probe.Run(context.Background(), tc.probes)

			g.Expect(got).To(Equal(tc.want))
		})
	}
}

func TestHost(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()

	probes := probe.Host(t.TempDir()+"/state", "/nonexistent/mkfs.erofs", nil, nil)

	// The reflink probe creates the state directory. Whether it passes
	// depends on the filesystem of the test directory.
	err := probes.Reflink(ctx)
	g.Expect(err == nil || errors.Is(err, overlay.ErrReflinkNotSupported)).To(BeTrue(), "got %v", err)

	g.Expect(probes.MkfsErofs(ctx)).To(MatchError(ContainSubstring("mkfs.erofs -V")))
}
