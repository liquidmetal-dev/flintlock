//go:build e2e
// +build e2e

package utils

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	g "github.com/onsi/gomega"
	"gopkg.in/yaml.v2"

	"github.com/liquidmetal-dev/flintlock/api/services/microvm/v1alpha1"
	"github.com/liquidmetal-dev/flintlock/api/types"
	"github.com/liquidmetal-dev/flintlock/client/cloudinit"
	"github.com/liquidmetal-dev/flintlock/client/cloudinit/instance"
	"github.com/liquidmetal-dev/flintlock/client/cloudinit/userdata"
)

const (
	// DefaultRootImage is the image which the test microvms use for their root volume.
	DefaultRootImage = "ghcr.io/liquidmetal-dev/capmvm-k8s-os:1.23.5"

	// cloud-init only reads the user-data as a cloud-config if it starts with this line.
	cloudConfigHeader = "#cloud-config\n"
	metadataPlatform  = "liquid_metal"

	// flintlockd answers the requests of the tests at once, the work is done
	// when it reconciles the microvm.
	requestTimeout = 30 * time.Second
)

// requestContext returns the context for a request to flintlockd. Without a
// deadline, a request to a flintlockd which does not answer would wait until
// the timeout of the test run.
func requestContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), requestTimeout)
}

// CreateMVM creates a microvm with the provider.
func CreateMVM(client v1alpha1.MicroVMClient, provider Provider, name, ns string) *v1alpha1.CreateMicroVMResponse {
	createReq := v1alpha1.CreateMicroVMRequest{
		Microvm: newMicroVMSpec(provider, name, ns),
	}
	ctx, cancel := requestContext()
	defer cancel()

	created, err := client.CreateMicroVM(ctx, &createReq)
	g.Expect(err).NotTo(g.HaveOccurred())

	return created
}

// CreateGuestAgentMVM creates a microvm with the guest-agent vsock device attached.
func CreateGuestAgentMVM(
	client v1alpha1.MicroVMClient,
	provider Provider,
	name, ns string,
) *v1alpha1.CreateMicroVMResponse {
	spec := newMicroVMSpec(provider, name, ns)
	spec.AllowGuestAgent = true

	ctx, cancel := requestContext()
	defer cancel()

	created, err := client.CreateMicroVM(ctx, &v1alpha1.CreateMicroVMRequest{Microvm: spec})
	g.Expect(err).NotTo(g.HaveOccurred())

	return created
}

// CreateMVMWithImages creates a microvm which uses the given images for its
// kernel and its root volume. The kernel image must have the kernel of the
// provider.
func CreateMVMWithImages(
	client v1alpha1.MicroVMClient,
	provider Provider,
	name, ns, kernelImage, rootImage string,
) *v1alpha1.CreateMicroVMResponse {
	spec := newMicroVMSpec(provider, name, ns)
	spec.Kernel.Image = kernelImage
	spec.RootVolume.Source.ContainerSource = pointyString(rootImage)

	ctx, cancel := requestContext()
	defer cancel()

	created, err := client.CreateMicroVM(ctx, &v1alpha1.CreateMicroVMRequest{Microvm: spec})
	g.Expect(err).NotTo(g.HaveOccurred())

	return created
}

func DeleteMVM(client v1alpha1.MicroVMClient, uid string) error {
	deleteReq := v1alpha1.DeleteMicroVMRequest{
		Uid: uid,
	}
	ctx, cancel := requestContext()
	defer cancel()

	_, err := client.DeleteMicroVM(ctx, &deleteReq)

	return err
}

func GetMVM(client v1alpha1.MicroVMClient, uid string) *v1alpha1.GetMicroVMResponse {
	getReq := v1alpha1.GetMicroVMRequest{
		Uid: uid,
	}
	ctx, cancel := requestContext()
	defer cancel()

	res, err := client.GetMicroVM(ctx, &getReq)
	g.Expect(err).NotTo(g.HaveOccurred())

	return res
}

func ListMVMs(client v1alpha1.MicroVMClient, ns string, name *string) *v1alpha1.ListMicroVMsResponse {
	listReq := v1alpha1.ListMicroVMsRequest{
		Namespace: ns,
		Name:      name,
	}
	ctx, cancel := requestContext()
	defer cancel()

	resp, err := client.ListMicroVMs(ctx, &listReq)
	g.Expect(err).NotTo(g.HaveOccurred())

	return resp
}

// ReadPID returns the pid of the VMM of the provider, from the pid file in the
// state directory of a microvm. It returns an error and does not fail the
// test, so that it can be used while the file is being written.
func ReadPID(stateDir string, provider Provider) (int, error) {
	pidFile := filepath.Join(stateDir, provider.PidFile)

	contents, err := os.ReadFile(pidFile)
	if err != nil {
		return 0, fmt.Errorf("reading the pid file: %w", err)
	}

	pid, err := strconv.Atoi(strings.TrimSpace(string(contents)))
	if err != nil {
		return 0, fmt.Errorf("reading the pid from %s: %w", pidFile, err)
	}

	return pid, nil
}

// VMMPidFiles returns the pid files which are in the state directory of a
// microvm, of all of the providers which the tests know about. A pid file
// shows that a VMM was started.
func VMMPidFiles(stateDir string) []string {
	pidFiles := []string{}

	for _, provider := range KnownProviders() {
		pidFile := filepath.Join(stateDir, provider.PidFile)
		if _, err := os.Stat(pidFile); err == nil {
			pidFiles = append(pidFiles, pidFile)
		}
	}

	return pidFiles
}

func PidRunning(pid int) bool {
	p, err := os.FindProcess(pid)
	if err != nil {
		return false
	}

	if err := p.Signal(syscall.SIGCONT); err != nil {
		return false
	}

	return true
}

// MicroVMMetadata returns the cloud-init data for a test microvm, in the
// encoding which the API expects.
func MicroVMMetadata(name, namespace string) (map[string]string, error) {
	instanceData, err := yaml.Marshal(instance.New(
		instance.WithLocalHostname(name),
		instance.WithPlatform(metadataPlatform),
	))
	if err != nil {
		return nil, fmt.Errorf("marshalling the instance data: %w", err)
	}

	// There are no commands which need the network, as there is no DHCP server
	// on the bridge of the tests.
	userData, err := yaml.Marshal(userdata.UserData{
		HostName:      name,
		PackageUpdate: pointyBool(false),
		FinalMessage:  fmt.Sprintf(bootMarkerFormat, namespace, name) + bootMarkerUptime,
	})
	if err != nil {
		return nil, fmt.Errorf("marshalling the user data: %w", err)
	}

	return map[string]string{
		cloudinit.InstanceDataKey: base64.StdEncoding.EncodeToString(instanceData),
		cloudinit.UserdataKey:     base64.StdEncoding.EncodeToString(append([]byte(cloudConfigHeader), userData...)),
	}, nil
}

func newMicroVMSpec(provider Provider, name, namespace string) *types.MicroVMSpec {
	spec, err := NewMicroVMSpec(provider, name, namespace)
	g.Expect(err).NotTo(g.HaveOccurred())

	return spec
}

// NewMicroVMSpec returns the spec of a test microvm which is created with the
// provider.
func NewMicroVMSpec(provider Provider, name, namespace string) (*types.MicroVMSpec, error) {
	metadata, err := MicroVMMetadata(name, namespace)
	if err != nil {
		return nil, err
	}

	guestMAC, guestAddress := guestNetwork(name, namespace)

	return &types.MicroVMSpec{
		Id:         name,
		Namespace:  namespace,
		Provider:   pointyString(provider.Name),
		Vcpu:       2,    //nolint: gomnd
		MemoryInMb: 2048, //nolint: gomnd
		Kernel: &types.Kernel{
			Image:            provider.KernelImage,
			Filename:         pointyString(provider.KernelFilename),
			AddNetworkConfig: true,
		},
		RootVolume: &types.Volume{
			Id:         "root",
			IsReadOnly: false,
			Source: &types.VolumeSource{
				ContainerSource: pointyString(DefaultRootImage),
			},
		},
		Interfaces: []*types.NetworkInterface{
			{
				DeviceId: "eth1",
				Type:     types.NetworkInterface_TAP,
				GuestMac: pointyString(guestMAC),
				Address: &types.StaticAddress{
					Address: guestAddress,
				},
			},
		},
		Metadata: metadata,
	}, nil
}

// guestNetwork returns a MAC address and a static address for the interface
// of a test microvm, which are made from its name and namespace.
//
// The name of the interface in the guest is not the same with all of the
// providers, so the network config has to match it by the MAC address. The
// address is static because the bridge of the tests has no DHCP server, which
// the guest would wait for when it boots.
func guestNetwork(name, namespace string) (string, string) {
	sum := sha256.Sum256([]byte(namespace + "/" + name))

	// 02 is a unicast address which is locally administered.
	mac := net.HardwareAddr{0x02, sum[0], sum[1], sum[2], sum[3], sum[4]}

	// 198.18.0.0/15 is reserved for tests (RFC 2544). The last byte is not 0 or
	// 255, which are not the address of a host in every network.
	address := fmt.Sprintf("198.18.%d.%d/16", sum[5], 1+sum[6]%254)

	return mac.String(), address
}

func pointyString(v string) *string {
	return &v
}

func pointyBool(v bool) *bool {
	return &v
}
