package flags_test

import (
	"testing"

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
