package command

import (
	"context"
	"fmt"

	"github.com/spf13/afero"
	"github.com/spf13/cobra"

	reignitev1 "github.com/weaveworks/reignite/api/reignite/v1alpha1"
	"github.com/weaveworks/reignite/pkg/defaults"
	"github.com/weaveworks/reignite/pkg/flags"
	"github.com/weaveworks/reignite/pkg/lifecycle"
	"github.com/weaveworks/reignite/pkg/log"
	fc "github.com/weaveworks/reignite/pkg/microvm/firecracker"
	"github.com/weaveworks/reignite/pkg/state"
)

func newRunCommand(cfg *Config) (*cobra.Command, error) {
	cmd := &cobra.Command{
		Use:   "run",
		Short: "Start running the reignite API",
		PreRunE: func(c *cobra.Command, _ []string) error {
			flags.BindCommandToViper(c)

			return nil
		},
		RunE: func(c *cobra.Command, _ []string) error {
			logger := log.GetLogger(c.Context())
			//TODO: add version info to logger?
			ctx := log.WithLogger(c.Context(), logger)

			//TODO: this is temporary for testing
			logger.Debug("creating lifecycle manager")
			manager, err := createLifecycleManager(c.Context(), cfg)
			if err != nil {
				return fmt.Errorf("creating lifecycle manager: %w", err)
			}

			// TODO: create a actual context

			vm := &reignitev1.MicroVM{
				Spec: reignitev1.MicroVMSpec{
					VCPU:       2,
					MemoryInMb: 2048,
					Kernel: reignitev1.Kernel{
						Image:   "docker.io/linuxkit/kernel:5.4.129",
						CmdLine: "",
					},
					Volumes: []reignitev1.Volume{
						reignitev1.Volume{
							ID:         "root",
							IsRoot:     true,
							IsReadOnly: false,
							MountPoint: "/",
							Source: reignitev1.VolumeSource{
								Container: &reignitev1.ContainerVolumeSource{
									Image: "docker.io/library/ubuntu:bionic",
								},
							},
						},
					},
					NetworkInterfaces: []reignitev1.NetworkInterface{
						reignitev1.NetworkInterface{
							AllowMetadataRequests: true,
							GuestDeviceName:       "eth0",
							GuestMAC:              "AA:FF:00:00:00:01",
							HostDeviceName:        "tap1",
						},
						reignitev1.NetworkInterface{
							AllowMetadataRequests: false,
							GuestDeviceName:       "eth1",
							HostDeviceName:        "/dev/tap10",
						},
					},
				},
			}

			if err := manager.Create(ctx, vm); err != nil {
				return fmt.Errorf("creating microvm: %w", err)
			}

			return nil
		},
	}

	cmd.Flags().IntVarP(&cfg.PortNumber, "port", "p", defaults.API_PORT, "The port number of the API server.")
	fc.AddFlags(cmd.Flags(), cfg.MicroVM.Firecracker)

	return cmd, nil
}

func createLifecycleManager(ctx context.Context, cfg *Config) (lifecycle.Manager, error) {
	vmState := state.NewFilesystem(&cfg.State, afero.NewOsFs())

	vmprovider, err := fc.New(cfg.MicroVM.Firecracker)
	if err != nil {
		return nil, fmt.Errorf("creating firecracker provider: %w", err)
	}

	manager, err := lifecycle.New(vmState, vmprovider)
	if err != nil {
		return nil, fmt.Errorf("creating lifecycle manager: %w", err)
	}

	return manager, nil
}
