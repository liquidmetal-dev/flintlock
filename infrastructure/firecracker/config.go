package firecracker

import (
	"encoding/base64"
	"fmt"

	"github.com/firecracker-microvm/firecracker-go-sdk"
	"gopkg.in/yaml.v3"

	cinetwork "github.com/weaveworks/flintlock/client/cloudinit/network"
	"github.com/weaveworks/flintlock/core/errors"
	"github.com/weaveworks/flintlock/core/models"
	"github.com/weaveworks/flintlock/internal/config"
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

		kernelCmdLine := DefaultKernelCmdLine()

		for key, value := range vm.Spec.Kernel.CmdLine {
			kernelCmdLine.Set(key, value)
		}

		if vm.Spec.Kernel.AddNetworkConfig {
			networkConfig, err := generateNetworkConfig(vm)
			if err != nil {
				return fmt.Errorf("generating kernel network-config: %w", err)
			}

			kernelCmdLine.Set("network-config", networkConfig)
		}

		kernelArgs := kernelCmdLine.String()
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

// DefaultKernelCmdLine is the default recommended kernel parameter list.
//
// console=ttyS0   [KLN] Output console device and options
// reboot=k        [KNL] reboot_type=kbd
// panic=1         [KNL] Kernel behaviour on panic: delay <timeout>
//                         timeout > 0: seconds before rebooting
//                         timeout = 0: wait forever
//                         timeout < 0: reboot immediately
// pci=off         [X86] don't probe for the PCI bus
// i8042.noaux     [HW]  Don't check for auxiliary (== mouse) port
// i8042.nomux     [HW]  Don't check presence of an active multiplexing
//                       controller
// i8042.nopnp     [HW]  Don't use ACPIPnP / PnPBIOS to discover KBD/AUX
//                       controllers
// i8042.dumbkbd   [HW]  Pretend that controller can only read data from
//                       keyboard and cannot control its state
//                       (Don't attempt to blink the leds)
//
// Read more:
// https://www.kernel.org/doc/html/v5.15/admin-guide/kernel-parameters.html
func DefaultKernelCmdLine() config.KernelCmdLine {
	return config.KernelCmdLine{
		"console":       "ttyS0",
		"reboot":        "k",
		"panic":         "1",
		"pci":           "off",
		"i8042.noaux":   "",
		"i8042.nomux":   "",
		"i8042.nopnp":   "",
		"i8042.dumbkbd": "",
		"ds":            "nocloud-net;s=http://169.254.169.254/latest/",
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
	network := &cinetwork.Network{
		Version:  cloudInitNetVersion,
		Ethernet: map[string]cinetwork.Ethernet{},
	}

	for i := range vm.Spec.NetworkInterfaces {
		iface := vm.Spec.NetworkInterfaces[i]

		status, ok := vm.Status.NetworkInterfaces[iface.GuestDeviceName]
		if !ok {
			return "", errors.NewNetworkInterfaceStatusMissing(iface.GuestDeviceName)
		}

		macAddress := getMacAddress(&iface, status)

		eth := &cinetwork.Ethernet{
			Match: cinetwork.Match{},
			DHCP4: firecracker.Bool(true),
			DHCP6: firecracker.Bool(true),
		}

		if macAddress != "" {
			eth.Match.MACAddress = macAddress
		} else {
			eth.Match.Name = iface.GuestDeviceName
		}

		if iface.StaticAddress != nil {
			if err := configureStaticEthernet(&iface, eth); err != nil {
				return "", fmt.Errorf("configuring static ethernet address: %w", err)
			}
		}

		network.Ethernet[iface.GuestDeviceName] = *eth
	}

	nd, err := yaml.Marshal(network)
	if err != nil {
		return "", fmt.Errorf("marshalling network data: %w", err)
	}

	return base64.StdEncoding.EncodeToString(nd), nil
}

func configureStaticEthernet(iface *models.NetworkInterface, eth *cinetwork.Ethernet) error {
	eth.Addresses = []string{string(iface.StaticAddress.Address)}

	if iface.StaticAddress.Gateway != nil {
		isIPv4, err := iface.StaticAddress.Gateway.IsIPv4()
		if err != nil {
			return fmt.Errorf("parsing gateway address: %w", err)
		}

		ipAddr, err := iface.StaticAddress.Gateway.IP()
		if err != nil {
			return fmt.Errorf("parsing gateway address: %w", err)
		}

		if isIPv4 {
			eth.GatewayIPv4 = ipAddr
		} else {
			eth.GatewayIPv6 = ipAddr
		}
	}

	if len(iface.StaticAddress.Nameservers) > 0 {
		eth.Nameservers = cinetwork.Nameservers{
			Addresses: []string{},
		}

		for nsIndex := range iface.StaticAddress.Nameservers {
			ns := iface.StaticAddress.Nameservers[nsIndex]
			eth.Nameservers.Addresses = append(eth.Nameservers.Addresses, ns)
		}
	}

	eth.DHCP4 = firecracker.Bool(false)
	eth.DHCP6 = firecracker.Bool(false)

	return nil
}

func getMacAddress(iface *models.NetworkInterface, status *models.NetworkInterfaceStatus) string {
	if iface.Type == models.IfaceTypeMacvtap {
		return status.MACAddress
	}

	return iface.GuestMAC
}
