package flags_test

import (
	"testing"
	"time"

	g "github.com/onsi/gomega"
	"github.com/spf13/cobra"

	"github.com/liquidmetal-dev/flintlock/internal/command/flags"
	"github.com/liquidmetal-dev/flintlock/internal/config"
)

func TestFirecrackerMetadataKernelIP_OptIn(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
		want bool
	}{
		{name: "flag not set is off", args: []string{}, want: false},
		{name: "flag set is on", args: []string{"--firecracker-metadata-kernel-ip"}, want: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			g.RegisterTestingT(t)

			cfg := &config.Config{}
			cmd := &cobra.Command{}
			flags.AddMicrovmProviderFlagsToCommand(cmd, cfg)

			g.Expect(cmd.ParseFlags(tc.args)).To(g.Succeed())
			g.Expect(cfg.FirecrackerMetadataKernelIP).To(g.Equal(tc.want))
		})
	}
}

func TestSnapshotFlags(t *testing.T) {
	for _, tc := range []struct {
		name           string
		args           []string
		wantPause      time.Duration
		wantConcurrent int
	}{
		{name: "defaults", args: []string{}, wantPause: 10 * time.Second, wantConcurrent: 1},
		{
			name:           "set",
			args:           []string{"--snapshot-max-pause=3s", "--snapshot-max-concurrent=4"},
			wantPause:      3 * time.Second,
			wantConcurrent: 4,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			g.RegisterTestingT(t)

			cfg := &config.Config{}
			cmd := &cobra.Command{}
			flags.AddSnapshotFlagsToCommand(cmd, cfg)

			g.Expect(cmd.ParseFlags(tc.args)).To(g.Succeed())
			g.Expect(cfg.SnapshotMaxPause).To(g.Equal(tc.wantPause))
			g.Expect(cfg.SnapshotMaxConcurrent).To(g.Equal(tc.wantConcurrent))
		})
	}
}
