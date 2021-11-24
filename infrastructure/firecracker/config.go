package firecracker

import (
	"encoding/base64"
	"fmt"

	"github.com/firecracker-microvm/firecracker-go-sdk"
	"gopkg.in/yaml.v3"

	"github.com/weaveworks/flintlock/core/errors"
	"github.com/weaveworks/flintlock/core/models"
	"github.com/weaveworks/flintlock/pkg/cloudinit"
	"github.com/weaveworks/flintlock/pkg/ptr"
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

	return cfg, nil
}

func WithMicroVM(vm *models.MicroVM) ConfigOption {
	return func(cfg *VmmConfig) error {
		if vm == nil {
			return errors.ErrSpecRequired
		}

		cfg.MachineConfig = MachineConfig{
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

		rootVolumeStatus, volumeStatusFound := vm.Status.Volumes[vm.Spec.RootVolume.ID]
		if !volumeStatusFound {
			return errors.NewVolumeNotMounted(vm.Spec.RootVolume.ID)
		}

		cfg.BlockDevices = append(cfg.BlockDevices, BlockDeviceConfig{
			ID:           vm.Spec.RootVolume.ID,
			IsReadOnly:   vm.Spec.RootVolume.IsReadOnly,
			IsRootDevice: true,
			PathOnHost:   rootVolumeStatus.Mount.Source,
			CacheType:    CacheTypeUnsafe,
		})

		for _, vol := range vm.Spec.AdditionalVolumes {
			status, ok := vm.Status.Volumes[vol.ID]
			if !ok {
				return errors.NewVolumeNotMounted(vol.ID)
			}

			cfg.BlockDevices = append(cfg.BlockDevices, BlockDeviceConfig{
				ID:           vol.ID,
				IsReadOnly:   vol.IsReadOnly,
				IsRootDevice: false,
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

	for i := range vm.Spec.NetworkInterfaces {
		iface := vm.Spec.NetworkInterfaces[i]

		status, ok := vm.Status.NetworkInterfaces[iface.GuestDeviceName]
		if !ok {
			return "", errors.NewNetworkInterfaceStatusMissing(iface.GuestDeviceName)
		}

		macAdress := getMacAddress(&iface, status)

		eth := &cloudinit.Ethernet{
			Match: cloudinit.Match{},
		}

		if macAdress != "" {
			eth.Match.MACAddress = &macAdress
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

func getMacAddress(iface *models.NetworkInterface, status *models.NetworkInterfaceStatus) string {
	if iface.Type == models.IfaceTypeMacvtap {
		return status.MACAddress
	}

	return iface.GuestMAC
}
