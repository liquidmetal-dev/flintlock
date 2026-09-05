package provision

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"github.com/liquidmetal-dev/flintlock/internal/provision"
)

type allOptions struct {
	unattended bool
	skipApt    bool
	thinpool   string
	disk       string
	address    string
	port       string
	iface      string
	bridgeName string
	insecure   bool
	dev        bool
	configFile string
}

func newAllCommand() *cobra.Command {
	opts := &allOptions{}

	cmd := &cobra.Command{
		Use: "all",
		Short: "Complete setup for a production ready host. Component versions can be " +
			"configured by setting the FLINTLOCK, CONTAINERD, FIRECRACKER and CLOUD_HYPERVISOR environment variables.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if !cmd.Flags().Changed("thinpool") && opts.dev {
				opts.thinpool = provision.DefaultDevThinpool
			}

			return runAll(cmd, opts)
		},
	}

	cmd.Flags().BoolVarP(&opts.unattended, "unattended", "y", false, "Autoapprove all prompts (danger)")
	cmd.Flags().BoolVarP(&opts.skipApt, "skip-apt", "s", false, "Skip installation of apt packages")
	cmd.Flags().StringVarP(&opts.thinpool, "thinpool", "t", provision.DefaultThinpool,
		"Name of thinpool to create (default: flintlock or flintlock-dev)")
	cmd.Flags().StringVarP(&opts.disk, "disk", "d", "",
		"Name of a blank, unpartitioned disk to use for the direct-lvm thinpool (ignored if --dev set)")
	cmd.Flags().StringVarP(&opts.address, "grpc-address", "a", "",
		"Address on which to start the Flintlock GRPC server (default: local ipv4 address)")
	cmd.Flags().StringVarP(&opts.port, "grpc-port", "p", "", "Port on which to start the GRPC server (default: 9090)")
	cmd.Flags().StringVarP(&opts.iface, "parent-iface", "i", "", "Interface of the default route of the host")
	cmd.Flags().StringVarP(&opts.bridgeName, "bridge", "b", "",
		"Bridge to use instead of an interface (will override --parent-iface)")
	cmd.Flags().BoolVarP(&opts.insecure, "insecure", "k", false, "Start flintlockd without basic auth or certs")
	cmd.Flags().BoolVar(&opts.dev, "dev", false, "Set up development environment. Loop thinpools will be created.")
	cmd.Flags().StringVarP(&opts.configFile, "flintlock-config-file", "f", "",
		"Path to a valid flintlockd configuration file with overriding config")

	return cmd
}

func runAll(cmd *cobra.Command, opts *allOptions) error {
	arch, err := hostArch()
	if err != nil {
		return err
	}

	out, errOut := cmd.OutOrStdout(), cmd.ErrOrStderr()
	runner := provision.NewRunner(out, errOut)

	fmt.Fprintf(out, "%s: Provisioning host\n", time.Now().UTC().Format("2006-01-02 15:04:05 UTC"))
	fmt.Fprintln(out, "The following will be performed: apt, firecracker, cloudhypervisor, "+
		"containerd, flintlock, direct-lvm|devpool")
	fmt.Fprintf(out, "Will install binaries for architecture: %s\n", arch)

	if err := provision.EnsureKVM(); err != nil {
		return err
	}

	if !opts.skipApt {
		if err := provision.InstallAptPackages(runner); err != nil {
			return err
		}
	}

	if err := setupStorage(cmd, runner, opts); err != nil {
		return err
	}

	fcVersion := provision.VersionFromEnv(provision.FirecrackerVersionEnv)
	if err := provision.InstallFirecracker(cmd.Context(), runner, fcVersion, arch); err != nil {
		return err
	}

	chVersion := provision.VersionFromEnv(provision.CloudHypervisorVersionEnv)
	if err := provision.InstallCloudHypervisor(cmd.Context(), runner, chVersion, arch); err != nil {
		return err
	}

	ctrdVersion := provision.VersionFromEnv(provision.ContainerdVersionEnv)

	containerdPaths, err := provision.AllContainerd(cmd.Context(), runner, ctrdVersion, opts.thinpool, arch, opts.dev)
	if err != nil {
		return err
	}

	if err := provision.AllFlintlock(cmd.Context(), runner, provision.AllFlintlockOptions{
		Version:              provision.VersionFromEnv(provision.FlintlockVersionEnv),
		Address:              opts.address,
		ParentIface:          opts.iface,
		BridgeName:           opts.bridgeName,
		Insecure:             opts.insecure,
		ConfigFile:           opts.configFile,
		Port:                 opts.port,
		Arch:                 arch,
		ContainerdStateDir:   containerdPaths.StateDir,
		ContainerdSystemdSvc: containerdPaths.SystemdSvc,
	}); err != nil {
		return err
	}

	hostname, _ := os.Hostname()
	fmt.Fprintf(out, "%s: Host %s provisioned\n", time.Now().UTC().Format("2006-01-02 15:04:05 UTC"), hostname)

	return nil
}

func setupStorage(cmd *cobra.Command, runner *provision.Runner, opts *allOptions) error {
	if !opts.dev {
		return setupDirectLVM(cmd, runner, opts.unattended, opts.disk, opts.thinpool)
	}

	paths := provision.BuildContainerdPaths(true)
	if err := provision.MakeContainerdDirs(paths); err != nil {
		return err
	}

	return provision.AllDevPool(runner, paths, opts.thinpool)
}

func setupDirectLVM(cmd *cobra.Command, runner *provision.Runner, unattended bool, disk, thinpool string) error {
	diskPath := disk

	if diskPath == "" {
		fmt.Fprintln(cmd.ErrOrStderr(), "WARNING: --disk has not been set. If you continue, "+
			"flintlock-provision will attempt to detect a free disk for formatting. Any data will be lost.")

		if !unattended && !provision.Confirm(cmd.InOrStdin(), cmd.OutOrStdout(), "Are you sure you wish to continue? (y/n) ") {
			return errAborted
		}

		found, err := provision.FindFreeDisk(runner)
		if err != nil {
			return err
		}

		diskPath = found
	}

	diskPath = filepath.Join("/dev", filepath.Base(diskPath))

	fmt.Fprintf(cmd.OutOrStdout(), "Will use %s for direct-lvm thinpool %s\n", diskPath, thinpool)
	fmt.Fprintf(cmd.ErrOrStderr(), "WARNING: all existing data on %s will be overwritten.\n", diskPath)

	if !unattended && !provision.Confirm(cmd.InOrStdin(), cmd.OutOrStdout(), "") {
		return errAborted
	}

	return provision.AllDirectLVM(runner, diskPath, thinpool)
}
