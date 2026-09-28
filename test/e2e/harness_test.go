//go:build e2e
// +build e2e

package e2e_test

import (
	"context"
	"encoding/base64"
	"errors"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	. "github.com/onsi/gomega"
	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"
	"gopkg.in/yaml.v2"

	"github.com/liquidmetal-dev/flintlock/api/services/microvm/v1alpha1"
	"github.com/liquidmetal-dev/flintlock/api/types"
	"github.com/liquidmetal-dev/flintlock/internal/command/flags"
	"github.com/liquidmetal-dev/flintlock/internal/config"
	"github.com/liquidmetal-dev/flintlock/pkg/ptr"
	u "github.com/liquidmetal-dev/flintlock/test/e2e/utils"
)

func TestParseProviders(t *testing.T) {
	type provider struct {
		name    string
		pidFile string
	}

	var (
		firecracker     = provider{name: "firecracker", pidFile: "firecracker.pid"}
		cloudHypervisor = provider{name: "cloudhypervisor", pidFile: "cloudhypervisor.pid"}
	)

	tt := []struct {
		name      string
		value     string
		expected  []provider
		expectErr bool
	}{
		{
			name:     "one provider",
			value:    "cloudhypervisor",
			expected: []provider{cloudHypervisor},
		},
		{
			name:     "both providers",
			value:    "firecracker,cloudhypervisor",
			expected: []provider{firecracker, cloudHypervisor},
		},
		{
			name:     "the order is kept, the first one is the default provider",
			value:    "cloudhypervisor,firecracker",
			expected: []provider{cloudHypervisor, firecracker},
		},
		{
			name:     "spaces around the names",
			value:    " firecracker , cloudhypervisor ",
			expected: []provider{firecracker, cloudHypervisor},
		},
		{
			name:      "unknown provider",
			value:     "firecracker,qemu",
			expectErr: true,
		},
		{
			name:      "provider given twice",
			value:     "firecracker,firecracker",
			expectErr: true,
		},
		{
			name:      "no providers",
			value:     "",
			expectErr: true,
		},
		{
			name:      "empty name",
			value:     "firecracker,",
			expectErr: true,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)

			providers, err := u.ParseProviders(tc.value)
			if tc.expectErr {
				g.Expect(err).To(HaveOccurred())

				return
			}

			g.Expect(err).NotTo(HaveOccurred())

			parsed := []provider{}
			for _, p := range providers {
				parsed = append(parsed, provider{name: p.Name, pidFile: p.PidFile})
			}

			g.Expect(parsed).To(Equal(tc.expected))
		})
	}
}

func TestNewMicroVMSpecUsesTheProvider(t *testing.T) {
	g := NewWithT(t)

	spec, err := u.NewMicroVMSpec(u.CloudHypervisor(), "mvm0", "ns0")
	g.Expect(err).NotTo(HaveOccurred())

	g.Expect(spec.Id).To(Equal("mvm0"))
	g.Expect(spec.Namespace).To(Equal("ns0"))
	g.Expect(spec.Provider).To(HaveValue(Equal("cloudhypervisor")))
	g.Expect(spec.Kernel.Image).To(Equal("ghcr.io/liquidmetal-dev/cloudhypervisor-kernel-bin:5.12"))
	g.Expect(spec.Kernel.Filename).To(HaveValue(Equal("boot/vmlinux.bin")))

	spec, err = u.NewMicroVMSpec(u.Firecracker(), "mvm0", "ns0")
	g.Expect(err).NotTo(HaveOccurred())

	g.Expect(spec.Provider).To(HaveValue(Equal("firecracker")))
	g.Expect(spec.Kernel.Image).To(Equal("ghcr.io/liquidmetal-dev/firecracker-kernel:6.1"))
	g.Expect(spec.Kernel.Filename).To(HaveValue(Equal("boot/vmlinux")))
}

// The test of a guest which cannot boot needs a kernel which Cloud Hypervisor
// refuses. It must not be the kernel of a provider: the test would stop to
// work when the provider gets a kernel which has a PVH entry point.
func TestKernelWithoutPVHIsNotAKernelOfAProvider(t *testing.T) {
	g := NewWithT(t)

	for _, provider := range u.KnownProviders() {
		g.Expect(provider.KernelImage).NotTo(Equal(u.KernelWithoutPVHImage),
			"the %s provider boots its guests with the kernel without PVH", provider.Name)
	}
}

// The network config of the guest matches the interface by its MAC address,
// and has a static address so that the guest does not wait for DHCP.
func TestNewMicroVMSpecNetworkInterface(t *testing.T) {
	g := NewWithT(t)

	testNetwork := &net.IPNet{IP: net.IPv4(198, 18, 0, 0), Mask: net.CIDRMask(15, 32)}

	newSpec := func(name, namespace string) *types.MicroVMSpec {
		spec, err := u.NewMicroVMSpec(u.Firecracker(), name, namespace)
		g.Expect(err).NotTo(HaveOccurred())

		return spec
	}

	first := newSpec("mvm0", "ns0")
	g.Expect(first.Interfaces).To(HaveLen(1))
	g.Expect(first.Interfaces[0].Type).To(Equal(types.NetworkInterface_TAP))

	mac, err := net.ParseMAC(first.Interfaces[0].GetGuestMac())
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(mac).To(HaveLen(6))
	g.Expect(mac[0]&0x01).To(BeZero(), "%s is not a unicast address", mac)
	g.Expect(mac[0]&0x02).NotTo(BeZero(), "%s is not a locally administered address", mac)

	g.Expect(first.Interfaces[0].Address).NotTo(BeNil())
	g.Expect(first.Interfaces[0].Address.Gateway).To(BeNil())

	ip, network, err := net.ParseCIDR(first.Interfaces[0].Address.Address)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(testNetwork.Contains(ip)).To(BeTrue(), "%s is not in %s", ip, testNetwork)
	g.Expect(ip.Equal(network.IP)).To(BeFalse(), "%s is the address of the network", ip)
	g.Expect(ip.To4()[3]).NotTo(BeElementOf(byte(0), byte(255)))

	again := newSpec("mvm0", "ns0")
	g.Expect(again.Interfaces[0].GetGuestMac()).To(Equal(first.Interfaces[0].GetGuestMac()))
	g.Expect(again.Interfaces[0].Address.Address).To(Equal(first.Interfaces[0].Address.Address))

	for _, other := range []*types.MicroVMSpec{
		newSpec("mvm1", "ns0"),
		newSpec("mvm0", "ns1"),
	} {
		g.Expect(other.Interfaces[0].GetGuestMac()).NotTo(Equal(first.Interfaces[0].GetGuestMac()))
		g.Expect(other.Interfaces[0].Address.Address).NotTo(Equal(first.Interfaces[0].Address.Address))
	}
}

func TestReadPID(t *testing.T) {
	writeFile := func(g Gomega, dir, name, content string) {
		g.Expect(os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600)).To(Succeed())
	}

	tt := []struct {
		name      string
		files     map[string]string
		expected  int
		expectErr bool
	}{
		{
			name:     "pid file of the provider",
			files:    map[string]string{"cloudhypervisor.pid": "4242"},
			expected: 4242,
		},
		{
			name:     "pid with a newline",
			files:    map[string]string{"cloudhypervisor.pid": "4242\n"},
			expected: 4242,
		},
		{
			name:      "no pid file",
			files:     map[string]string{},
			expectErr: true,
		},
		{
			name:      "pid file of another provider",
			files:     map[string]string{"firecracker.pid": "4242"},
			expectErr: true,
		},
		{
			// The file is empty between the moment it is created and the moment
			// the pid is written to it.
			name:      "pid file which is still empty",
			files:     map[string]string{"cloudhypervisor.pid": ""},
			expectErr: true,
		},
		{
			name:      "pid which is not a number",
			files:     map[string]string{"cloudhypervisor.pid": "abc"},
			expectErr: true,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)

			dir := t.TempDir()
			for name, content := range tc.files {
				writeFile(g, dir, name, content)
			}

			pid, err := u.ReadPID(dir, u.CloudHypervisor())
			if tc.expectErr {
				g.Expect(err).To(HaveOccurred())

				return
			}

			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(pid).To(Equal(tc.expected))
		})
	}
}

func TestVMMPidFiles(t *testing.T) {
	g := NewWithT(t)

	dir := t.TempDir()
	g.Expect(u.VMMPidFiles(dir)).To(BeEmpty())
	g.Expect(u.VMMPidFiles(filepath.Join(dir, "missing"))).To(BeEmpty())

	g.Expect(os.WriteFile(filepath.Join(dir, "firecracker.log"), []byte("log"), 0o600)).To(Succeed())
	g.Expect(u.VMMPidFiles(dir)).To(BeEmpty())

	g.Expect(os.WriteFile(filepath.Join(dir, "cloudhypervisor.pid"), []byte("1"), 0o600)).To(Succeed())
	g.Expect(u.VMMPidFiles(dir)).To(Equal([]string{filepath.Join(dir, "cloudhypervisor.pid")}))

	g.Expect(os.WriteFile(filepath.Join(dir, "firecracker.pid"), []byte("2"), 0o600)).To(Succeed())
	g.Expect(u.VMMPidFiles(dir)).To(ConsistOf(
		filepath.Join(dir, "firecracker.pid"),
		filepath.Join(dir, "cloudhypervisor.pid"),
	))
}

func TestMicroVMMetadata(t *testing.T) {
	g := NewWithT(t)

	metadata, err := u.MicroVMMetadata("mvm0", "ns0")
	g.Expect(err).NotTo(HaveOccurred())

	// Cloud Hypervisor cannot build the cloud-init image if a value is not base64.
	userData, err := base64.StdEncoding.DecodeString(metadata["user-data"])
	g.Expect(err).NotTo(HaveOccurred())
	// cloud-init ignores user-data which does not start with this line.
	g.Expect(string(userData)).To(HavePrefix("#cloud-config\n"))

	cloudConfig := map[string]any{}
	g.Expect(yaml.Unmarshal(userData, &cloudConfig)).To(Succeed())
	g.Expect(cloudConfig).To(HaveKeyWithValue("hostname", "mvm0"))
	// cloud-init replaces $UPTIME, so the text on the console of a guest which
	// has booted is not the text which is in the user-data.
	g.Expect(cloudConfig).To(HaveKeyWithValue("final_message", "flintlock-e2e boot ok ns0/mvm0 uptime=$UPTIME"))
	// The bridge of the tests has no DHCP server, dhclient would hold up the boot.
	g.Expect(cloudConfig).NotTo(HaveKey("runcmd"))
	g.Expect(cloudConfig).NotTo(HaveKey("users"))

	metaData, err := base64.StdEncoding.DecodeString(metadata["meta-data"])
	g.Expect(err).NotTo(HaveOccurred())

	// There is no instance_id, so that flintlockd sets it to the uid of the microvm.
	instanceData := map[string]string{}
	g.Expect(yaml.Unmarshal(metaData, &instanceData)).To(Succeed())
	g.Expect(instanceData).To(Equal(map[string]string{
		"local_hostname": "mvm0",
		"platform":       "liquid_metal",
	}))
}

func TestPrivateImageRef(t *testing.T) {
	g := NewWithT(t)

	g.Expect(u.PrivateImageRef("ghcr.io/liquidmetal-dev/flintlock-kernel:5.10.77")).
		To(Equal("127.0.0.1:5050/liquidmetal-dev/flintlock-kernel:5.10.77"))
}

func TestManifestPath(t *testing.T) {
	g := NewWithT(t)

	g.Expect(u.ManifestPath("127.0.0.1:5050/liquidmetal-dev/flintlock-kernel:5.10.77")).
		To(Equal("/v2/liquidmetal-dev/flintlock-kernel/manifests/5.10.77"))
}

// The lines are from the log of the registry in a run of the tests, without
// the fields which come after the headers.
const registryLog = `{"time":"2026-09-27T14:54:53.327480622Z","level":"info","message":"HTTP API","module":"http","component":"session","clientIP":"127.0.0.1:45590","method":"GET","path":"/v2/","statusCode":401,"latency":"0s","bodySize":253,"headers":{"Accept":["application/vnd.oci.image.manifest.v1+json"],"Accept-Encoding":["gzip"],"User-Agent":["Go-http-client/1.1"]}}
{"time":"2026-09-27T14:54:55.736062174Z","level":"info","message":"HTTP API","module":"http","username":"flintlock","component":"session","clientIP":"127.0.0.1:45614","method":"PUT","path":"/v2/liquidmetal-dev/flintlock-kernel/manifests/5.10.77","statusCode":201,"latency":"0s","bodySize":0,"headers":{"Accept-Encoding":["gzip"],"Authorization":["******"],"Content-Length":["563"],"User-Agent":["skopeo/1.13.3"]}}
{"time":"2026-09-27T14:54:58.875543634Z","level":"info","message":"HTTP API","module":"http","component":"session","clientIP":"127.0.0.1:38398","method":"GET","path":"/v2/liquidmetal-dev/flintlock-kernel/manifests/5.10.77","statusCode":401,"latency":"0s","bodySize":253,"headers":{"Accept":["application/vnd.oci.image.manifest.v1+json"],"Accept-Encoding":["gzip"],"User-Agent":["Go-http-client/1.1"]}}
this line is not JSON
{"time":"2026-09-27T14:54:58.9Z","level":"info","message":"some other message","path":"/v2/liquidmetal-dev/flintlock-kernel/manifests/5.10.77","statusCode":200,"headers":{"User-Agent":["containerd/2.2.0+unknown"]}}
{"time":"2026-09-27T14:54:59.020350976Z","level":"info","message":"HTTP API","module":"http","username":"flintlock","component":"session","clientIP":"127.0.0.1:38432","method":"HEAD","path":"/v2/liquidmetal-dev/capmvm-k8s-os/manifests/1.23.5","statusCode":200,"latency":"0s","bodySize":0,"headers":{"Authorization":["******"],"User-Agent":["containerd/2.2.0+unknown"]}}
{"time":"2026-09-27T14:55:18.1Z","level":"info","message":"HTTP API","module":"http","username":"flintlock","component":"session","clientIP":"127.0.0.1:57850","method":"HEAD","path":"/v2/liquidmetal-dev/flintlock-kernel/manifests/5.10.77","statusCode":200,"latency":"0s","bodySize":0,"headers":{"Authorization":["******"],"User-Agent":["containerd/2.2.0+unknown"]}}
{"time":"2026-09-27T14:55:18.224102545Z","level":"info","message":"HTTP API","module":"http","username":"flintlock","component":"session","clientIP":"127.0.0.1:57850","method":"GET","path":"/v2/liquidmetal-dev/flintlock-kernel/blobs/sha256:ac7c335c24f68ce5f1cae03eabedcecd29326badf138a8e0c8457c710364246b","statusCode":200,"latency":"0s","bodySize":698,"headers":{"Accept":["application/vnd.oci.image.config.v1+json, */*"],"Authorization":["******"],"User-Agent":["containerd/2.2.0+unknown"]}}
{"time":"2026-09-27T14:55:19.1Z","level":"info","message":"HTTP API","module":"http","component":"session","clientIP":"127.0.0.1:57850","method":"GET","path":"/v2/liquidmetal-dev/flintlock-kernel-other/manifests/5.10.77","statusCode":404,"latency":"0s","bodySize":0,"headers":{"User-Agent":["containerd/2.2.0+unknown"]}}
`

func TestRequestsFromLog(t *testing.T) {
	const (
		kernelImage  = "127.0.0.1:5050/liquidmetal-dev/flintlock-kernel:5.10.77"
		kernelBlob   = "/v2/liquidmetal-dev/flintlock-kernel/blobs/sha256:ac7c335c24f68ce5f1cae03eabedcecd29326badf138a8e0c8457c710364246b"
		kernelByTag  = "/v2/liquidmetal-dev/flintlock-kernel/manifests/5.10.77"
		rootImage    = "127.0.0.1:5050/liquidmetal-dev/capmvm-k8s-os:1.23.5"
		rootByTag    = "/v2/liquidmetal-dev/capmvm-k8s-os/manifests/1.23.5"
		missingImage = "127.0.0.1:5050/liquidmetal-dev/missing:1.0.0"
	)

	tt := []struct {
		name            string
		image           string
		userAgentPrefix string
		expected        []u.RegistryRequest
	}{
		{
			name:            "requests of containerd for the kernel image",
			image:           kernelImage,
			userAgentPrefix: u.ContainerdUserAgent,
			expected: []u.RegistryRequest{
				{Method: "HEAD", Path: kernelByTag, Status: 200},
				{Method: "GET", Path: kernelBlob, Status: 200},
			},
		},
		{
			name:            "requests of containerd for the root image",
			image:           rootImage,
			userAgentPrefix: u.ContainerdUserAgent,
			expected: []u.RegistryRequest{
				{Method: "HEAD", Path: rootByTag, Status: 200},
			},
		},
		{
			name:            "requests of the test itself are not those of containerd",
			image:           kernelImage,
			userAgentPrefix: "Go-http-client/",
			expected: []u.RegistryRequest{
				{Method: "GET", Path: kernelByTag, Status: 401},
			},
		},
		{
			name:            "image which was not requested",
			image:           missingImage,
			userAgentPrefix: u.ContainerdUserAgent,
			expected:        []u.RegistryRequest{},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)

			g.Expect(u.RequestsFromLog([]byte(registryLog), tc.image, tc.userAgentPrefix)).To(Equal(tc.expected))
		})
	}
}

func TestContainerdMajorVersion(t *testing.T) {
	tt := []struct {
		name      string
		output    string
		expected  int
		expectErr bool
	}{
		{
			name:     "v2 release build",
			output:   "containerd github.com/containerd/containerd/v2 v2.2.9 1294c24a7da8e5a793ed378161673abe94118892\n",
			expected: 2,
		},
		{
			name:     "v1 release build",
			output:   "containerd github.com/containerd/containerd v1.7.28 b98a3aace656320842a23f4a392a33f46af97866\n",
			expected: 1,
		},
		{
			name:     "distro build without v prefix or revision",
			output:   "containerd github.com/containerd/containerd/v2 2.2.1-0ubuntu1~24.04.1 \n",
			expected: 2,
		},
		{
			name:      "too few fields",
			output:    "containerd\n",
			expectErr: true,
		},
		{
			name:      "version is not a number",
			output:    "containerd github.com/containerd/containerd/v2 unknown abc\n",
			expectErr: true,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)

			major, err := u.ContainerdMajorVersion(tc.output)
			if tc.expectErr {
				g.Expect(err).To(HaveOccurred())

				return
			}

			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(major).To(Equal(tc.expected))
		})
	}
}

func TestConsoleMarker(t *testing.T) {
	const booted = "[   14.302118] cloud-init[612]: flintlock-e2e boot ok ns0/mvm0 uptime=14.30"

	tt := []struct {
		name      string
		console   *string
		expected  string
		expectErr bool
	}{
		{
			name: "guest which has booted",
			console: ptr.String("[    0.000000] Linux version 5.12.0\n" +
				"[    1.204911] EXT4-fs (vda): mounted filesystem\n" +
				booted + "\n" +
				"mvm0 login: "),
			expected: booted,
		},
		{
			name:     "console with carriage returns",
			console:  ptr.String("[    0.000000] Linux version 5.12.0\r\n" + booted + "\r\n"),
			expected: booted,
		},
		{
			name:     "uptime without a fraction",
			console:  ptr.String("flintlock-e2e boot ok ns0/mvm0 uptime=14\n"),
			expected: "flintlock-e2e boot ok ns0/mvm0 uptime=14",
		},
		{
			name:      "no console file",
			expectErr: true,
		},
		{
			name:      "guest which has not booted yet",
			console:   ptr.String("[    0.000000] Linux version 5.12.0\n"),
			expectErr: true,
		},
		{
			// The user-data can be on the console without a guest that has booted,
			// cloud-init has not replaced $UPTIME in it.
			name:      "user-data which is printed as it is",
			console:   ptr.String("final_message: flintlock-e2e boot ok ns0/mvm0 uptime=$UPTIME\n"),
			expectErr: true,
		},
		{
			name:      "marker of another microvm",
			console:   ptr.String("flintlock-e2e boot ok ns0/mvm1 uptime=14.30\n"),
			expectErr: true,
		},
		{
			name:      "marker of a microvm with a longer name",
			console:   ptr.String("flintlock-e2e boot ok ns0/mvm01 uptime=14.30\n"),
			expectErr: true,
		},
		{
			name:      "marker of a microvm in a namespace with a longer name",
			console:   ptr.String("flintlock-e2e boot ok other-ns0/mvm0 uptime=14.30\n"),
			expectErr: true,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)

			dir := t.TempDir()
			if tc.console != nil {
				g.Expect(os.WriteFile(filepath.Join(dir, "cloudhypervisor.stdout"), []byte(*tc.console), 0o600)).To(Succeed())
			}
			// The console of another provider must not be read.
			g.Expect(os.WriteFile(filepath.Join(dir, "firecracker.stdout"), []byte(booted+"\n"), 0o600)).To(Succeed())

			line, err := u.ConsoleMarker(dir, u.CloudHypervisor(), "mvm0", "ns0")
			if tc.expectErr {
				g.Expect(err).To(HaveOccurred())

				return
			}

			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(line).To(Equal(tc.expected))
		})
	}
}

// The name of a microvm can have characters which mean something in a
// regular expression.
func TestConsoleMarkerNameIsNotAPattern(t *testing.T) {
	g := NewWithT(t)

	dir := t.TempDir()
	g.Expect(os.WriteFile(filepath.Join(dir, "firecracker.stdout"),
		[]byte("flintlock-e2e boot ok ns0/mvmX0 uptime=1.0\n"), 0o600)).To(Succeed())

	_, err := u.ConsoleMarker(dir, u.Firecracker(), "mvm.0", "ns0")
	g.Expect(err).To(HaveOccurred())
}

func TestVMMExecutable(t *testing.T) {
	g := NewWithT(t)

	testBinary, err := os.Executable()
	g.Expect(err).NotTo(HaveOccurred())

	testBinary, err = filepath.EvalSymlinks(testBinary)
	g.Expect(err).NotTo(HaveOccurred())

	g.Expect(u.VMMExecutable(os.Getpid())).To(Equal(testBinary))

	// Above the highest pid which the kernel can give to a process.
	_, err = u.VMMExecutable(1 << 23)
	g.Expect(err).To(HaveOccurred())
}

func TestVerifyVMM(t *testing.T) {
	testBinary, err := os.Executable()
	NewWithT(t).Expect(err).NotTo(HaveOccurred())

	otherBinary, err := exec.LookPath("sh")
	NewWithT(t).Expect(err).NotTo(HaveOccurred())

	stopped := exec.Command(otherBinary, "-c", "true")
	NewWithT(t).Expect(stopped.Run()).To(Succeed())

	tt := []struct {
		name      string
		binary    string
		pid       string
		expected  int
		expectErr bool
	}{
		{
			name:     "running process of the binary of the provider",
			binary:   testBinary,
			pid:      strconv.Itoa(os.Getpid()),
			expected: os.Getpid(),
		},
		{
			name:      "running process of another binary",
			binary:    otherBinary,
			pid:       strconv.Itoa(os.Getpid()),
			expectErr: true,
		},
		{
			name:      "process which has stopped",
			binary:    otherBinary,
			pid:       strconv.Itoa(stopped.Process.Pid),
			expectErr: true,
		},
		{
			name:      "binary which is not installed",
			binary:    "flintlock-e2e-no-such-vmm",
			pid:       strconv.Itoa(os.Getpid()),
			expectErr: true,
		},
		{
			name:      "empty pid file",
			binary:    testBinary,
			pid:       "",
			expectErr: true,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)

			provider := u.Provider{Name: "test", Binary: tc.binary, PidFile: "test.pid"}

			dir := t.TempDir()
			g.Expect(os.WriteFile(filepath.Join(dir, provider.PidFile), []byte(tc.pid), 0o600)).To(Succeed())

			pid, err := u.VerifyVMM(dir, provider)
			if tc.expectErr {
				g.Expect(err).To(HaveOccurred())

				return
			}

			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(pid).To(Equal(tc.expected))
		})
	}
}

func TestTail(t *testing.T) {
	tt := []struct {
		name     string
		content  string
		lines    int
		expected []string
	}{
		{
			name:     "more lines than asked for",
			content:  "one\ntwo\nthree\nfour\n",
			lines:    2,
			expected: []string{"three", "four"},
		},
		{
			name:     "fewer lines than asked for",
			content:  "one\ntwo\n",
			lines:    5,
			expected: []string{"one", "two"},
		},
		{
			name:     "last line without a newline, and carriage returns",
			content:  "one\r\ntwo\r\nthree",
			lines:    2,
			expected: []string{"two", "three"},
		},
		{
			name:     "empty lines at the end",
			content:  "one\ntwo\n\n\n",
			lines:    1,
			expected: []string{"two"},
		},
		{
			name:     "no content",
			content:  "",
			lines:    3,
			expected: []string{},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)

			g.Expect(u.Tail([]byte(tc.content), tc.lines)).To(Equal(tc.expected))
		})
	}
}

func TestProviderFlags(t *testing.T) {
	g := NewWithT(t)

	testBinary, err := os.Executable()
	g.Expect(err).NotTo(HaveOccurred())

	testBinary, err = filepath.EvalSymlinks(testBinary)
	g.Expect(err).NotTo(HaveOccurred())

	shell, err := exec.LookPath("sh")
	g.Expect(err).NotTo(HaveOccurred())

	shell, err = filepath.EvalSymlinks(shell)
	g.Expect(err).NotTo(HaveOccurred())

	first := u.Provider{Name: "first", Binary: "sh", BinaryFlag: "--first-bin"}
	second := u.Provider{Name: "second", Binary: testBinary, BinaryFlag: "--second-bin"}
	missing := u.Provider{Name: "missing", Binary: "flintlock-e2e-no-such-vmm", BinaryFlag: "--missing-bin"}

	g.Expect(u.ProviderFlags([]u.Provider{first, second})).To(Equal([]string{
		"--default-provider", "first",
		"--first-bin", shell,
		"--second-bin", testBinary,
	}))

	g.Expect(u.ProviderFlags([]u.Provider{second})).To(Equal([]string{
		"--default-provider", "second",
		"--second-bin", testBinary,
	}))

	_, err = u.ProviderFlags([]u.Provider{first, missing})
	g.Expect(err).To(MatchError(ContainSubstring("missing")))

	_, err = u.ProviderFlags([]u.Provider{})
	g.Expect(err).To(HaveOccurred())
}

// The flags are parsed with the flags of flintlockd, so the test fails if
// flintlockd does not have one of them.
func TestProviderFlagsAreFlagsOfFlintlockd(t *testing.T) {
	g := NewWithT(t)

	testBinary, err := os.Executable()
	g.Expect(err).NotTo(HaveOccurred())

	testBinary, err = filepath.EvalSymlinks(testBinary)
	g.Expect(err).NotTo(HaveOccurred())

	shell, err := exec.LookPath("sh")
	g.Expect(err).NotTo(HaveOccurred())

	shell, err = filepath.EvalSymlinks(shell)
	g.Expect(err).NotTo(HaveOccurred())

	cloudHypervisor := u.CloudHypervisor()
	cloudHypervisor.Binary = testBinary

	firecracker := u.Firecracker()
	firecracker.Binary = "sh"

	args, err := u.ProviderFlags([]u.Provider{cloudHypervisor, firecracker})
	g.Expect(err).NotTo(HaveOccurred())

	cfg := &config.Config{}
	cmd := &cobra.Command{}
	flags.AddMicrovmProviderFlagsToCommand(cmd, cfg)

	g.Expect(cmd.Flags().Parse(args)).To(Succeed())
	g.Expect(cfg.DefaultVMProvider).To(Equal("cloudhypervisor"))
	g.Expect(cfg.CloudHypervisorBin).To(Equal(testBinary))
	g.Expect(cfg.FirecrackerBin).To(Equal(shell))
}

// The microvms of a provider are in namespaces of their own, so that a list
// of a namespace only has the microvms of one provider.
func TestLifecycleNamespaces(t *testing.T) {
	g := NewWithT(t)

	namespace, longNamespace := lifecycleNamespaces(u.Firecracker())
	g.Expect(namespace).To(Equal("firecracker-ns0"))
	g.Expect(longNamespace).To(Equal("ns-firecracker-ssssssssssssssssssssssssssss"))

	namespace, longNamespace = lifecycleNamespaces(u.CloudHypervisor())
	g.Expect(namespace).To(Equal("cloudhypervisor-ns0"))
	g.Expect(longNamespace).To(Equal("ns-cloudhypervisor-ssssssssssssssssssssssss"))

	// As long as the namespace was before it had the name of the provider.
	g.Expect(longNamespace).To(HaveLen(len("ns-") + 40))
}

func TestVMMErrorLine(t *testing.T) {
	const bootError = "Error booting VM: VmBoot(KernelMissingPvhHeader)"

	tt := []struct {
		name      string
		stderr    *string
		expected  string
		expectErr bool
	}{
		{
			name:     "VMM which could not boot the kernel",
			stderr:   ptr.String(bootError + "\n"),
			expected: bootError,
		},
		{
			name:     "error after other lines, the case of the text does not matter",
			stderr:   ptr.String("some warning\nerror booting vm: VmBoot(KernelMissingPVHHeader)\n"),
			expected: "error booting vm: VmBoot(KernelMissingPVHHeader)",
		},
		{
			name:      "no stderr file",
			expectErr: true,
		},
		{
			name:      "VMM which has not written an error",
			stderr:    ptr.String(""),
			expectErr: true,
		},
		{
			name:      "another error",
			stderr:    ptr.String("Error booting VM: VmBoot(DeviceManager(CreateVirtioNet))\n"),
			expectErr: true,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)

			dir := t.TempDir()
			if tc.stderr != nil {
				g.Expect(os.WriteFile(filepath.Join(dir, "cloudhypervisor.stderr"), []byte(*tc.stderr), 0o600)).To(Succeed())
			}
			// The stderr of another provider must not be read.
			g.Expect(os.WriteFile(filepath.Join(dir, "firecracker.stderr"), []byte(bootError+"\n"), 0o600)).To(Succeed())

			line, err := u.VMMErrorLine(dir, u.CloudHypervisor(), "error booting vm", "pvh")
			if tc.expectErr {
				g.Expect(err).To(HaveOccurred())

				return
			}

			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(line).To(Equal(tc.expected))
		})
	}
}

// A test which does not tear down its environment leaves containerd,
// flintlockd, the thinpool and the bridge behind. A test which comes after it
// must not do its own setup on top of them.
func TestE2ETestsSkipWhenTheEnvironmentIsLeftRunning(t *testing.T) {
	tt := []struct {
		name string
		test func(*testing.T)
	}{
		{name: "TestE2E", test: TestE2E},
		{name: "TestE2EPrivateRegistry", test: TestE2EPrivateRegistry},
		{name: "TestE2EPrivateRegistryNoCredentials", test: TestE2EPrivateRegistryNoCredentials},
		{name: "TestE2ECloudHypervisorKernelWithoutPVH", test: TestE2ECloudHypervisorKernelWithoutPVH},
		{name: "TestE2EGuestWithoutInit", test: TestE2EGuestWithoutInit},
	}

	leftRunning := environmentLeftRunning

	defer func() { environmentLeftRunning = leftRunning }()

	for _, tc := range tt {
		// A test which is not skipped sets this again.
		environmentLeftRunning = true
		skipped := false

		t.Run(tc.name, func(t *testing.T) {
			defer func() { skipped = t.Skipped() }()

			tc.test(t)
		})

		if !skipped {
			t.Errorf("%s was not skipped", tc.name)
		}
	}
}

// The text of the error is not the same in all of the versions of Cloud
// Hypervisor.
func TestMissingPVHErrorLine(t *testing.T) {
	tt := []struct {
		name      string
		stderr    string
		expected  string
		expectErr bool
	}{
		{
			name:     "Cloud Hypervisor up to v46",
			stderr:   "Error booting VM: VmBoot(KernelMissingPvhHeader)\n",
			expected: "Error booting VM: VmBoot(KernelMissingPvhHeader)",
		},
		{
			name:     "Cloud Hypervisor from v48, the reason is on a line of its own",
			stderr:   "Error booting VM\nKernel lacks PVH header\n",
			expected: "Kernel lacks PVH header",
		},
		{
			name:      "VMM which could not boot for another reason",
			stderr:    "Error booting VM: VmBoot(DeviceManager(CreateVirtioNet))\n",
			expectErr: true,
		},
		{
			name:      "VMM which has not written an error",
			stderr:    "",
			expectErr: true,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)

			dir := t.TempDir()
			g.Expect(os.WriteFile(filepath.Join(dir, "cloudhypervisor.stderr"), []byte(tc.stderr), 0o600)).To(Succeed())

			line, err := u.MissingPVHErrorLine(dir, u.CloudHypervisor())
			if tc.expectErr {
				g.Expect(err).To(HaveOccurred())

				return
			}

			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(line).To(Equal(tc.expected))
		})
	}
}

// fakeMicroVMClient answers the requests of the helpers without a flintlockd.
// A method which a test does not expect to be called is not implemented, and
// panics.
type fakeMicroVMClient struct {
	v1alpha1.MicroVMClient

	// deadlines has the time which was left of each of the requests, or zero
	// for a request without a deadline.
	deadlines []time.Duration
	// listed are the microvms which ListMicroVMs returns, one entry for each
	// of the calls. The last entry is used for the calls after it.
	listed    [][]string
	listErr   error
	listCalls int

	// deletable is a microvm which ListMicroVMs returns until DeleteMicroVM
	// was called deletesNeeded times. With 0 times it is always returned.
	deletable     string
	deletesNeeded int
	deleteErr     error
	deleteCalls   int
}

func (f *fakeMicroVMClient) request(ctx context.Context) {
	left := time.Duration(0)
	if deadline, ok := ctx.Deadline(); ok {
		left = time.Until(deadline)
	}

	f.deadlines = append(f.deadlines, left)
}

func (f *fakeMicroVMClient) CreateMicroVM(
	ctx context.Context, in *v1alpha1.CreateMicroVMRequest, _ ...grpc.CallOption,
) (*v1alpha1.CreateMicroVMResponse, error) {
	f.request(ctx)

	return &v1alpha1.CreateMicroVMResponse{Microvm: &types.MicroVM{Spec: in.Microvm}}, nil
}

func (f *fakeMicroVMClient) DeleteMicroVM(
	ctx context.Context, _ *v1alpha1.DeleteMicroVMRequest, _ ...grpc.CallOption,
) (*emptypb.Empty, error) {
	f.request(ctx)
	f.deleteCalls++

	if f.deleteErr != nil {
		return nil, f.deleteErr
	}

	return &emptypb.Empty{}, nil
}

func (f *fakeMicroVMClient) GetMicroVM(
	ctx context.Context, in *v1alpha1.GetMicroVMRequest, _ ...grpc.CallOption,
) (*v1alpha1.GetMicroVMResponse, error) {
	f.request(ctx)

	return &v1alpha1.GetMicroVMResponse{Microvm: &types.MicroVM{Spec: &types.MicroVMSpec{Uid: &in.Uid}}}, nil
}

func (f *fakeMicroVMClient) ListMicroVMs(
	ctx context.Context, _ *v1alpha1.ListMicroVMsRequest, _ ...grpc.CallOption,
) (*v1alpha1.ListMicroVMsResponse, error) {
	f.request(ctx)

	if f.listErr != nil {
		return nil, f.listErr
	}

	resp := &v1alpha1.ListMicroVMsResponse{}

	if f.deletable != "" && (f.deletesNeeded == 0 || f.deleteCalls < f.deletesNeeded) {
		resp.Microvm = append(resp.Microvm, &types.MicroVM{Spec: &types.MicroVMSpec{Uid: ptr.String(f.deletable)}})
	}

	if len(f.listed) == 0 {
		return resp, nil
	}

	call := min(f.listCalls, len(f.listed)-1)
	f.listCalls++

	for _, uid := range f.listed[call] {
		resp.Microvm = append(resp.Microvm, &types.MicroVM{Spec: &types.MicroVMSpec{Uid: ptr.String(uid)}})
	}

	return resp, nil
}

// A request without a deadline waits for as long as flintlockd does not
// answer, which is until the timeout of the whole test run.
func TestRequestsOfTheHelpersHaveADeadline(t *testing.T) {
	RegisterTestingT(t)

	tt := []struct {
		name    string
		request func(client v1alpha1.MicroVMClient)
	}{
		{
			name: "CreateMVM",
			request: func(client v1alpha1.MicroVMClient) {
				u.CreateMVM(client, u.Firecracker(), "mvm0", "ns0")
			},
		},
		{
			name: "CreateGuestAgentMVM",
			request: func(client v1alpha1.MicroVMClient) {
				u.CreateGuestAgentMVM(client, u.Firecracker(), "mvm0", "ns0")
			},
		},
		{
			name: "CreateMVMWithImages",
			request: func(client v1alpha1.MicroVMClient) {
				u.CreateMVMWithImages(client, u.Firecracker(), "mvm0", "ns0", "kernel:1", "root:1")
			},
		},
		{
			name: "DeleteMVM",
			request: func(client v1alpha1.MicroVMClient) {
				Expect(u.DeleteMVM(client, "uid0")).To(Succeed())
			},
		},
		{
			name: "GetMVM",
			request: func(client v1alpha1.MicroVMClient) {
				u.GetMVM(client, "uid0")
			},
		},
		{
			name: "ListMVMs",
			request: func(client v1alpha1.MicroVMClient) {
				u.ListMVMs(client, "ns0", nil)
			},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			RegisterTestingT(t)

			client := &fakeMicroVMClient{}
			tc.request(client)

			Expect(client.deadlines).To(HaveLen(1))
			Expect(client.deadlines[0]).To(BeNumerically(">", 0), "the request has no deadline")
			Expect(client.deadlines[0]).To(BeNumerically("<=", 30*time.Second))
		})
	}
}

func TestWaitForDeleted(t *testing.T) {
	const (
		timeout = 200 * time.Millisecond
		polling = 10 * time.Millisecond
	)

	tt := []struct {
		name string
		// listed are the microvms in the list of the namespace, for each of the
		// calls.
		listed    [][]string
		listErr   error
		stateDir  bool
		expectErr string
		calls     int
	}{
		{
			name:   "microvm which is deleted after a while",
			listed: [][]string{{"uid0", "uid1"}, {"uid0", "uid1"}, {"uid1"}},
			calls:  3,
		},
		{
			name:   "microvm which is not in the list from the start",
			listed: [][]string{{"uid1"}},
			calls:  1,
		},
		{
			// The state directory does not show that the microvm is deleted, when
			// there never was one.
			name:      "microvm which stays in the list and never had a state directory",
			listed:    [][]string{{"uid0"}},
			expectErr: "is in the list of the namespace",
		},
		{
			name:      "microvm which is not in the list but has its state directory",
			listed:    [][]string{{"uid1"}},
			stateDir:  true,
			expectErr: "exists",
		},
		{
			name:      "flintlockd which does not answer",
			listErr:   errors.New("connection refused"),
			expectErr: "connection refused",
			calls:     1,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)

			stateDir := filepath.Join(t.TempDir(), "uid0")
			if tc.stateDir {
				g.Expect(os.Mkdir(stateDir, 0o700)).To(Succeed())
			}

			client := &fakeMicroVMClient{listed: tc.listed, listErr: tc.listErr}

			err := u.WaitForDeleted(client, "ns0", "uid0", stateDir, timeout, polling)
			if tc.expectErr != "" {
				g.Expect(err).To(MatchError(ContainSubstring(tc.expectErr)))
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}

			if tc.calls > 0 {
				g.Expect(client.deadlines).To(HaveLen(tc.calls))
			}
		})
	}
}

// The setup of the tests stops when the VMM of a provider cannot be used. The
// error has to say which provider and which binary it is.
func TestProviderBinaryVersion(t *testing.T) {
	dir := t.TempDir()

	script := func(name, content string) string {
		file := filepath.Join(dir, name)
		NewWithT(t).Expect(os.WriteFile(file, []byte("#!/bin/sh\n"+content+"\n"), 0o700)).To(Succeed())

		return file
	}

	working := script("working-vmm", `echo "Fake VMM v1.2.3"; echo "a second line"`)
	broken := script("broken-vmm", `echo "cannot start" >&2; exit 3`)

	t.Run("VMM which reports its version", func(t *testing.T) {
		g := NewWithT(t)

		binary, version, err := u.Provider{Name: "fake", Binary: working}.BinaryVersion()
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(binary).To(Equal(working))
		g.Expect(version).To(Equal("Fake VMM v1.2.3"))
	})

	t.Run("VMM which fails", func(t *testing.T) {
		g := NewWithT(t)

		_, _, err := u.Provider{Name: "fake", Binary: broken}.BinaryVersion()
		g.Expect(err).To(MatchError(And(
			ContainSubstring("fake"),
			ContainSubstring(broken),
			ContainSubstring("exit status 3"),
		)))
	})

	t.Run("VMM which is not installed", func(t *testing.T) {
		g := NewWithT(t)

		_, _, err := u.Provider{Name: "fake", Binary: "flintlock-e2e-no-such-vmm"}.BinaryVersion()
		g.Expect(err).To(MatchError(And(
			ContainSubstring("fake"),
			ContainSubstring("flintlock-e2e-no-such-vmm"),
		)))
	})
}

// flintlockd can lose a delete which it gets while it creates the microvm
// (#1266).
func TestDeleteAndWait(t *testing.T) {
	const (
		timeout = 400 * time.Millisecond
		polling = 10 * time.Millisecond
		again   = 50 * time.Millisecond
	)

	tt := []struct {
		name string
		// deletesNeeded is how many times flintlockd has to be asked before it
		// deletes the microvm. With 0 it never does.
		deletesNeeded int
		deleteErr     error
		listErr       error
		expectErr     string
		expectDeletes int
	}{
		{
			name:          "microvm which is deleted when it is asked for",
			deletesNeeded: 1,
			expectDeletes: 1,
		},
		{
			name:          "delete which flintlockd has lost",
			deletesNeeded: 2,
			expectDeletes: 2,
		},
		{
			name:      "microvm which is never deleted",
			expectErr: "is in the list of the namespace",
		},
		{
			// The microvm stays in the list, the delete was not done.
			name:          "flintlockd which refuses the delete",
			deleteErr:     errors.New("permission denied"),
			expectErr:     "permission denied",
			expectDeletes: 1,
		},
		{
			name:          "flintlockd which does not answer",
			deletesNeeded: 1,
			listErr:       errors.New("connection refused"),
			expectErr:     "connection refused",
			expectDeletes: 1,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)

			client := &fakeMicroVMClient{
				deletable:     "uid0",
				deletesNeeded: tc.deletesNeeded,
				deleteErr:     tc.deleteErr,
				listErr:       tc.listErr,
			}
			stateDir := filepath.Join(t.TempDir(), "uid0")

			err := u.DeleteAndWait(client, "ns0", "uid0", stateDir, timeout, polling, again)
			if tc.expectErr != "" {
				g.Expect(err).To(MatchError(ContainSubstring(tc.expectErr)))
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}

			if tc.expectDeletes > 0 {
				g.Expect(client.deleteCalls).To(Equal(tc.expectDeletes))
			} else {
				g.Expect(client.deleteCalls).To(BeNumerically(">", 1), "the delete was not asked for again")
			}
		})
	}
}

// A microvm which was deleted between two requests is deleted, flintlockd
// then refuses the second delete because it does not know the microvm.
func TestDeleteAndWaitMicroVMWhichIsGone(t *testing.T) {
	g := NewWithT(t)

	client := &fakeMicroVMClient{deleteErr: errors.New("microvm spec uid0 not found")}
	stateDir := filepath.Join(t.TempDir(), "uid0")

	g.Expect(u.DeleteAndWait(client, "ns0", "uid0", stateDir, 200*time.Millisecond, 10*time.Millisecond, 50*time.Millisecond)).To(Succeed())
}

func TestConsoleLine(t *testing.T) {
	const panicLine = "[    1.204911] Kernel panic - not syncing: No working init found.  Try passing init= option to kernel."

	tt := []struct {
		name      string
		console   *string
		texts     []string
		expected  string
		expectErr bool
	}{
		{
			name:     "line with all of the texts, in any case",
			console:  ptr.String("[    0.000000] Linux version 5.12.0\r\n" + panicLine + "\r\n"),
			texts:    []string{"kernel panic", "INIT"},
			expected: panicLine,
		},
		{
			name:     "the first of the lines with the texts",
			console:  ptr.String("first Kernel panic: no init\nsecond Kernel panic: no init\n"),
			texts:    []string{"Kernel panic", "init"},
			expected: "first Kernel panic: no init",
		},
		{
			name:      "no console file",
			texts:     []string{"Kernel panic"},
			expectErr: true,
		},
		{
			name:      "texts which are on two lines",
			console:   ptr.String("[    1.0] Kernel panic - not syncing\n[    1.1] No working init found\n"),
			texts:     []string{"Kernel panic", "init"},
			expectErr: true,
		},
		{
			name:      "guest which has not got that far",
			console:   ptr.String("[    0.000000] Linux version 5.12.0\n"),
			texts:     []string{"Kernel panic", "init"},
			expectErr: true,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)

			dir := t.TempDir()
			if tc.console != nil {
				g.Expect(os.WriteFile(filepath.Join(dir, "cloudhypervisor.stdout"), []byte(*tc.console), 0o600)).To(Succeed())
			}
			// The console of another provider and the stderr must not be read.
			g.Expect(os.WriteFile(filepath.Join(dir, "firecracker.stdout"), []byte(panicLine+"\n"), 0o600)).To(Succeed())
			g.Expect(os.WriteFile(filepath.Join(dir, "cloudhypervisor.stderr"), []byte(panicLine+"\n"), 0o600)).To(Succeed())

			line, err := u.ConsoleLine(dir, u.CloudHypervisor(), tc.texts...)
			if tc.expectErr {
				g.Expect(err).To(HaveOccurred())

				return
			}

			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(line).To(Equal(tc.expected))
		})
	}
}
