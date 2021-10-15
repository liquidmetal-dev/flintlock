package firecracker

import (
	"context"
	"encoding/base64"
	"fmt"

	"github.com/weaveworks/reignite/pkg/ptr"

	"github.com/firecracker-microvm/firecracker-go-sdk"
	fcmodels "github.com/firecracker-microvm/firecracker-go-sdk/client/models"
	"gopkg.in/yaml.v3"

	"github.com/weaveworks/reignite/core/errors"
	"github.com/weaveworks/reignite/core/models"
	"github.com/weaveworks/reignite/pkg/cloudinit"
)

const (
	cloudInitNetVersion = 2
)

type ConfigOption func(cfg *VmmConfig) error

func CreateConfig(opts ...ConfigOption) (*VmmConfig, error) {
	cfg := &VmmConfig{}

	for _, opt := range opts {
		if err := opt(cfg); err != nil {
			return nil, fmt.Errorf("creating firecracker configuration: %w", err)
		}
	}

	// TODO: do we need to add validation?

	return cfg, nil
}

func WithMicroVM(vm *models.MicroVM) ConfigOption {
	return func(cfg *VmmConfig) error {
		if vm == nil {
			return errors.ErrSpecRequired
		}

		cfg.MachineConfig = VMConfig{
			MemSizeMib: vm.Spec.MemoryInMb,
			VcpuCount:  vm.Spec.VCPU,
			HTEnabled:  false,
		}

		cfg.NetDevices = []NetworkInterfaceConfig{}
		for i := range vm.Spec.NetworkInterfaces {
			iface := vm.Spec.NetworkInterfaces[i]

			status, ok := vm.Status.NetworkInterfaces[iface.GuestDeviceName]
			if !ok {
				return errors.NewNetworkInterfaceStatusMissing(iface.GuestDeviceName)
			}

			fcInt := createNetworkIface(&iface, status)
			cfg.NetDevices = append(cfg.NetDevices, *fcInt)
		}

		cfg.BlockDevices = []BlockDeviceConfig{}
		for _, vol := range vm.Spec.Volumes {
			status, ok := vm.Status.Volumes[vol.ID]
			if !ok {
				return errors.NewVolumeNotMounted(vol.ID)
			}

			cfg.BlockDevices = append(cfg.BlockDevices, BlockDeviceConfig{
				ID:           vol.ID,
				IsReadOnly:   vol.IsReadOnly,
				IsRootDevice: vol.IsRoot,
				PathOnHost:   status.Mount.Source,
				// Partuuid: ,
				// RateLimiter: ,
				CacheType: CacheTypeUnsafe,
			})
		}

		kernelArgs := vm.Spec.Kernel.CmdLine
		if vm.Spec.Kernel.AddNetworkConfig {
			networkConfig, err := generateNetworkConfig(vm)
			if err != nil {
				return fmt.Errorf("generating kernel network-config: %w", err)
			}
			kernelArgs = fmt.Sprintf("%s network-config=%s", kernelArgs, networkConfig)
		}

		cfg.BootSource = BootSourceConfig{
			KernelImagePage: fmt.Sprintf("%s/%s", vm.Status.KernelMount.Source, vm.Spec.Kernel.Filename),
			BootArgs:        &kernelArgs,
		}

		if vm.Spec.Initrd != nil {
			initrdPath := fmt.Sprintf("%s/%s", vm.Status.InitrdMount.Source, vm.Spec.Initrd.Filename)
			cfg.BootSource.InitrdPath = &initrdPath
		}

		return nil
	}
}

func WithState(vmState State) ConfigOption {
	return func(cfg *VmmConfig) error {
		cfg.Logger = &LoggerConfig{
			LogPath:       vmState.LogPath(),
			Level:         LogLevelDebug,
			ShowLevel:     true,
			ShowLogOrigin: true,
		}
		cfg.Metrics = &MetricsConfig{
			Path: vmState.MetricsPath(),
		}

		return nil
	}
}

func ApplyConfig(ctx context.Context, cfg *VmmConfig, client *firecracker.Client) error {
	machineConf := &fcmodels.MachineConfiguration{
		VcpuCount:  &cfg.MachineConfig.VcpuCount,
		MemSizeMib: &cfg.MachineConfig.MemSizeMib,
		HtEnabled:  &cfg.MachineConfig.HTEnabled,
	}
	if cfg.MachineConfig.CPUTemplate != nil {
		machineConf.CPUTemplate = fcmodels.CPUTemplate(*cfg.MachineConfig.CPUTemplate)
	}

	_, err := client.PutMachineConfiguration(ctx, machineConf)
	if err != nil {
		return fmt.Errorf("failed to put machine configuration: %w", err)
	}
	for _, drive := range cfg.BlockDevices {
		_, err := client.PutGuestDriveByID(ctx, drive.ID, &fcmodels.Drive{
			DriveID:      &drive.ID,
			IsReadOnly:   &drive.IsReadOnly,
			IsRootDevice: &drive.IsRootDevice,
			Partuuid:     drive.PartUUID,
			PathOnHost:   &drive.PathOnHost,
			// RateLimiter: ,
		})
		if err != nil {
			return fmt.Errorf("putting drive configuration: %w", err)
		}
	}
	for i, netInt := range cfg.NetDevices {
		guestIfaceName := fmt.Sprintf("eth%d", i)
		_, err := client.PutGuestNetworkInterfaceByID(ctx, guestIfaceName, &fcmodels.NetworkInterface{
			IfaceID:           &guestIfaceName,
			GuestMac:          netInt.GuestMAC,
			HostDevName:       &netInt.HostDevName,
			AllowMmdsRequests: netInt.AllowMMDSRequests,
		})
		if err != nil {
			return fmt.Errorf("putting %s network configuration: %w", guestIfaceName, err)
		}
	}
	_, err = client.PutGuestBootSource(ctx, &fcmodels.BootSource{
		KernelImagePath: &cfg.BootSource.KernelImagePage,
		BootArgs:        *cfg.BootSource.BootArgs,
		InitrdPath:      *cfg.BootSource.InitrdPath,
	})
	if err != nil {
		return fmt.Errorf("failed to put machine bootsource: %w", err)
	}
	if cfg.Logger != nil {
		_, err = client.PutLogger(ctx, &fcmodels.Logger{
			LogPath:       &cfg.Logger.LogPath,
			ShowLevel:     ptr.Bool(true),
			ShowLogOrigin: ptr.Bool(true),
			Level:         ptr.String(string(cfg.Logger.Level)),
		})
		if err != nil {
			return fmt.Errorf("failed to put logging configuration: %w", err)
		}
	}
	if cfg.Metrics != nil {
		_, err = client.PutMetrics(ctx, &fcmodels.Metrics{
			MetricsPath: &cfg.Metrics.Path,
		})
		if err != nil {
			return fmt.Errorf("failed to put metrics configuration: %w", err)
		}
	}
	// vsock
	// mmds

	return nil
}

func ApplyMetadata(ctx context.Context, metadata map[string]string, client *firecracker.Client) error {
	if len(metadata) == 0 {
		return nil
	}

	meta := &Metadata{
		Latest: map[string]string{},
	}
	for metadataKey, metadataVal := range metadata {
		encodedVal, err := base64.StdEncoding.DecodeString(metadataVal)
		if err != nil {
			return fmt.Errorf("base64 decoding metadata %s: %w", metadataKey, err)
		}

		meta.Latest[metadataKey] = string(encodedVal)
	}

	if _, err := client.PutMmds(ctx, meta); err != nil {
		return fmt.Errorf("putting %d metadata items into mmds: %w", len(meta.Latest), err)
	}

	return nil
}

func createNetworkIface(iface *models.NetworkInterface, status *models.NetworkInterfaceStatus) *NetworkInterfaceConfig {
	macAddr := iface.GuestMAC
	hostDevName := status.HostDeviceName

	if iface.Type == models.IfaceTypeMacvtap {
		hostDevName = fmt.Sprintf("/dev/tap%d", status.Index)
		if macAddr == "" {
			macAddr = status.MACAddress
		}
	}

	netInt := &NetworkInterfaceConfig{
		IfaceID:           iface.GuestDeviceName,
		HostDevName:       hostDevName,
		GuestMAC:          macAddr,
		AllowMMDSRequests: iface.AllowMetadataRequests,
	}

	return netInt
}

func generateNetworkConfig(vm *models.MicroVM) (string, error) {
	network := &cloudinit.Network{
		Version:  cloudInitNetVersion,
		Ethernet: map[string]*cloudinit.Ethernet{},
	}

	for _, iface := range vm.Spec.NetworkInterfaces {
		eth := &cloudinit.Ethernet{
			Match: cloudinit.Match{},
		}

		if iface.GuestMAC != "" {
			address := iface.GuestMAC
			eth.Match.MACAddress = &address
		} else {
			eth.Match.Name = &iface.GuestDeviceName
		}

		if iface.Address != "" {
			eth.Addresses = []string{iface.Address}
			eth.DHCP4 = ptr.Bool(false)
		} else {
			eth.DHCP4 = firecracker.Bool(true)
		}

		network.Ethernet[iface.GuestDeviceName] = eth
	}

	nd, err := yaml.Marshal(network)
	if err != nil {
		return "", fmt.Errorf("marshalling network data: %w", err)
	}

	return base64.StdEncoding.EncodeToString(nd), nil
}
