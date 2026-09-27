//go:build e2e
// +build e2e

package utils

import (
	"bufio"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	ccfg "github.com/containerd/containerd/v2/cmd/containerd/server/config"
	"github.com/containerd/containerd/v2/plugins/snapshots/devmapper/dmsetup"
	gk "github.com/onsi/ginkgo/v2"
	gm "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	"github.com/pelletier/go-toml"
	"google.golang.org/grpc"

	"github.com/liquidmetal-dev/flintlock/api/services/microvm/v1alpha1"
)

const (
	containerdBin      = "containerd"
	flintlockCmdDir    = "github.com/liquidmetal-dev/flintlock/cmd/flintlockd"
	containerdCfgDir   = "/etc/containerd"
	containerdRootDir  = "/var/lib/containerd-e2e"
	containerdStateDir = "/run/containerd-e2e"
	containerdCfg      = containerdCfgDir + "/config-e2e.toml"
	containerdSocket   = containerdStateDir + "/containerd.sock"
	devMapperRoot      = containerdRootDir + "/snapshotter/devmapper"
	grpcDialTarget     = "127.0.0.1:9090"
	loopDeviceTag      = "e2e"
	bridgeName         = "fl-e2e-br0"
	e2eDataDir         = "/var/lib/flintlock-e2e"
	// This is where hack/scripts/devpool.sh creates the files which back the thinpool.
	thinpoolBackingDir = "/var/lib/containerd-dev/snapshotter/devmapper"

	// HostsDir is the certs.d directory flintlockd reads registry hosts.toml
	// files from during the tests.
	HostsDir = e2eDataDir + "/certs.d"

	minContainerdMajor = 2
)

// Runner holds test runner configuration.
type Runner struct {
	params            *Params
	flintlockdBin     string
	containerdSession *gexec.Session
	flintlockdSession *gexec.Session
	flintlockdConn    *grpc.ClientConn
}

// NewRunner creates a new instance of Runner with a set of Params.
func NewRunner(params *Params) Runner {
	return Runner{
		params: params,
	}
}

// Setup is a helper for the e2e tests which:
// - checks that the containerd on the PATH is a supported version
// - sets up up devicemapper thinpools
// - writes containerd config
// - compiles flintlockd
// - starts containerd
// - starts flintlockd
// - opens a connection to the grpc server
// - returns a new MicroVMClient which can then be used in testing
// All opened connections and started processes are saved for later shutdown.
// Teardown should be called before Setup in a defer.
func (r *Runner) Setup() v1alpha1.MicroVMClient {
	checkContainerdVersion()
	makeDirectories()
	r.createThinPools()
	createBridge()
	r.writeContainerdConfig()
	r.buildFLBinary()
	r.startContainerd()
	r.startFlintlockd()
	r.dialGRPCServer()

	return v1alpha1.NewMicroVMClient(r.flintlockdConn)
}

// Teardown will gracefully close and kill all connections and processes which were
// opened as part of Setup.
// It should be called before Setup as part of a defer:
//
// r := utils.Runner{}
// defer r.Teardown()
// r.Setup()
// .
func (r *Runner) Teardown() {
	if r.flintlockdConn != nil {
		r.flintlockdConn.Close()
	}

	// If either of these is true, we should still close the connection held
	// by the runner itself. The other processes can be killed manually after
	// debugging.
	if r.params.SkipTeardown || r.params.SkipDelete {
		log.Println("TEST INFO: skipping teardown, containerd, flintlockd and their resources are left in place")

		return
	}

	if r.flintlockdSession != nil {
		log.Println("TEST INFO: stopping flintlockd")
		r.flintlockdSession.Terminate().Wait()
	}

	if r.containerdSession != nil {
		log.Println("TEST INFO: stopping containerd")
		r.containerdSession.Terminate().Wait()
	}

	deleteBridge()
	r.cleanupThinPools()
	cleanupDirectories()

	log.Println("TEST INFO: removing the flintlockd build artefacts")
	gexec.CleanupBuildArtifacts()
}

func makeDirectories() {
	gm.Expect(os.MkdirAll(devMapperRoot, os.ModePerm)).To(gm.Succeed())
	gm.Expect(os.MkdirAll(containerdStateDir, os.ModePerm)).To(gm.Succeed())
	gm.Expect(os.MkdirAll(containerdCfgDir, os.ModePerm)).To(gm.Succeed())
	gm.Expect(os.MkdirAll(HostsDir, os.ModePerm)).To(gm.Succeed())
}

func cleanupDirectories() {
	log.Printf("TEST INFO: removing directories %s, %s, %s and %s",
		containerdCfgDir, containerdRootDir, containerdStateDir, e2eDataDir)

	gm.Expect(os.RemoveAll(containerdCfgDir)).To(gm.Succeed())
	gm.Expect(os.RemoveAll(containerdRootDir)).To(gm.Succeed())
	gm.Expect(os.RemoveAll(containerdStateDir)).To(gm.Succeed())
	gm.Expect(os.RemoveAll(e2eDataDir)).To(gm.Succeed())
}

// ContainerdMajorVersion returns the major version from the output of
// 'containerd --version', which has the form
// 'containerd <package> <version> <revision>'.
func ContainerdMajorVersion(versionOutput string) (int, error) {
	const versionField = 2

	fields := strings.Fields(versionOutput)
	if len(fields) <= versionField {
		return 0, fmt.Errorf("unexpected containerd version output %q", versionOutput)
	}

	version := strings.TrimPrefix(fields[versionField], "v")
	majorStr, _, _ := strings.Cut(version, ".")

	major, err := strconv.Atoi(majorStr)
	if err != nil {
		return 0, fmt.Errorf("parsing containerd version %q: %w", fields[versionField], err)
	}

	return major, nil
}

func checkContainerdVersion() {
	output, err := exec.Command(containerdBin, "--version").Output()
	gm.Expect(err).NotTo(gm.HaveOccurred())

	log.Printf("TEST INFO: found containerd version: %s", strings.TrimSpace(string(output)))

	major, err := ContainerdMajorVersion(string(output))
	gm.Expect(err).NotTo(gm.HaveOccurred())
	gm.Expect(major).To(gm.BeNumerically(">=", minContainerdMajor),
		"the e2e tests need containerd v%d or later, found: %s", minContainerdMajor, strings.TrimSpace(string(output)))
}

func (r *Runner) createThinPools() {
	if r.params.SkipSetupThinpool {
		log.Printf("TEST INFO: skipping thinpool setup, using existing thinpool %s", r.params.ThinpoolName)

		return
	}

	log.Printf("TEST INFO: creating thinpool %s", r.params.ThinpoolName)

	scriptPath := filepath.Join(baseDir(), "hack", "scripts", "devpool.sh")
	command := exec.Command(scriptPath, r.params.ThinpoolName, loopDeviceTag)
	session, err := gexec.Start(command, gk.GinkgoWriter, gk.GinkgoWriter)

	gm.Expect(err).NotTo(gm.HaveOccurred())
	gm.Eventually(session, "20s").Should(gexec.Exit(0))
}

func (r *Runner) cleanupThinPools() {
	if r.params.SkipSetupThinpool {
		log.Printf("TEST INFO: skipping thinpool cleanup, thinpool %s is left in place", r.params.ThinpoolName)

		return
	}

	log.Printf("TEST INFO: removing thinpool %s, its loop devices and backing files", r.params.ThinpoolName)

	gm.Expect(dmsetup.RemoveDevice(r.params.ThinpoolName, dmsetup.RemoveWithForce)).To(gm.Succeed())

	cmd := exec.Command("losetup")
	loopDevices := grep(cmd, loopDeviceTag, 0)

	for _, dev := range loopDevices {
		command := exec.Command("losetup", "-d", dev)
		session, err := gexec.Start(command, gk.GinkgoWriter, gk.GinkgoWriter)
		gm.Expect(err).NotTo(gm.HaveOccurred())
		gm.Eventually(session).Should(gexec.Exit(0))
	}

	// The backing files hold the thinpool metadata. If they are reused with a
	// new containerd root then the snapshotter will try to create devices with
	// ids that already exist in the pool.
	for _, name := range []string{"data", "metadata"} {
		backingFile := filepath.Join(thinpoolBackingDir, name+"-"+loopDeviceTag)
		gm.Expect(os.RemoveAll(backingFile)).To(gm.Succeed())
	}
}

func createBridge() {
	if bridgeExists() {
		log.Printf("TEST INFO: bridge %s already exists", bridgeName)
	} else {
		log.Printf("TEST INFO: creating bridge %s", bridgeName)

		addCmd := exec.Command("ip", "link", "add", bridgeName, "type", "bridge")
		addSession, err := gexec.Start(addCmd, gk.GinkgoWriter, gk.GinkgoWriter)
		gm.Expect(err).NotTo(gm.HaveOccurred())
		gm.Eventually(addSession).Should(gexec.Exit(0))
	}

	upCmd := exec.Command("ip", "link", "set", bridgeName, "up")
	upSession, err := gexec.Start(upCmd, gk.GinkgoWriter, gk.GinkgoWriter)
	gm.Expect(err).NotTo(gm.HaveOccurred())
	gm.Eventually(upSession).Should(gexec.Exit(0))
}

func bridgeExists() bool {
	cmd := exec.Command("ip", "link", "show", bridgeName)

	return cmd.Run() == nil
}

func deleteBridge() {
	if !bridgeExists() {
		log.Printf("TEST INFO: bridge %s does not exist, nothing to delete", bridgeName)

		return
	}

	log.Printf("TEST INFO: deleting bridge %s", bridgeName)

	command := exec.Command("ip", "link", "delete", bridgeName, "type", "bridge")
	session, err := gexec.Start(command, gk.GinkgoWriter, gk.GinkgoWriter)
	gm.Expect(err).NotTo(gm.HaveOccurred())
	gm.Eventually(session).Should(gexec.Exit(0))
}

func (r *Runner) writeContainerdConfig() {
	dmplug := map[string]interface{}{
		"pool_name":       r.params.ThinpoolName,
		"root_path":       devMapperRoot,
		"base_image_size": "10GB",
		"discard_blocks":  true,
	}
	cfg := ccfg.Config{
		Version: 3,
		Root:    containerdRootDir,
		State:   containerdStateDir,
		GRPC: ccfg.GRPCConfig{
			Address: containerdSocket,
		},
		Metrics: ccfg.MetricsConfig{
			Address: "127.0.0.1:1338",
		},
		Debug: ccfg.Debug{
			Level: r.params.ContainerdLogLevel,
		},
		Plugins: map[string]any{
			"io.containerd.snapshotter.v1.devmapper": dmplug,
		},
	}

	log.Printf("TEST INFO: writing containerd config to %s (root %s, state %s)",
		containerdCfg, containerdRootDir, containerdStateDir)

	f, err := os.Create(containerdCfg)
	gm.Expect(err).NotTo(gm.HaveOccurred())

	defer f.Close()

	gm.Expect(toml.NewEncoder(f).Encode(cfg)).To(gm.Succeed())
}

func (r *Runner) buildFLBinary() {
	log.Println("TEST INFO: building flintlockd")

	flBin, err := gexec.Build(flintlockCmdDir)
	gm.Expect(err).NotTo(gm.HaveOccurred())

	log.Printf("TEST INFO: built flintlockd at %s", flBin)

	r.flintlockdBin = flBin
}

func (r *Runner) startContainerd() {
	log.Printf("TEST INFO: starting containerd with config %s and log level %s",
		containerdCfg, r.params.ContainerdLogLevel)

	ctrdCmd := exec.Command(containerdBin, "--config", containerdCfg)
	ctrdSess, err := gexec.Start(ctrdCmd, gk.GinkgoWriter, gk.GinkgoWriter)
	gm.Expect(err).NotTo(gm.HaveOccurred())

	r.containerdSession = ctrdSess
}

func (r *Runner) startFlintlockd() {
	parentIface, err := getParentInterface()
	gm.Expect(err).NotTo(gm.HaveOccurred())

	//nolint: gosec // We know what we're doing.
	flCmd := exec.Command(r.flintlockdBin, "run",
		"--containerd-socket", containerdSocket,
		"--containerd-hosts-dir", HostsDir,
		"--parent-iface", parentIface,
		"--bridge-name", bridgeName,
		"--verbosity", r.params.FlintlockdLogLevel,
		"--insecure",
	)

	log.Printf("TEST INFO: starting flintlockd with arguments: %s", strings.Join(flCmd.Args[1:], " "))

	flSess, err := gexec.Start(flCmd, gk.GinkgoWriter, gk.GinkgoWriter)
	gm.Expect(err).NotTo(gm.HaveOccurred())

	r.flintlockdSession = flSess
}

func (r *Runner) dialGRPCServer() {
	log.Printf("TEST INFO: connecting to flintlockd at %s", grpcDialTarget)

	conn, err := grpc.Dial(grpcDialTarget, grpc.WithInsecure(), grpc.WithBlock())
	gm.Expect(err).NotTo(gm.HaveOccurred())

	log.Printf("TEST INFO: connected to flintlockd at %s", grpcDialTarget)

	r.flintlockdConn = conn
}

func getParentInterface() (string, error) {
	cmd := exec.Command("ip", "route", "show")
	iface := grep(cmd, "default", 4)

	if len(iface) == 0 {
		return "", errors.New("parent interface not found")
	}

	return iface[0], nil
}

func grep(cmd *exec.Cmd, match string, loc int) []string {
	output, err := cmd.Output()
	gm.Expect(err).NotTo(gm.HaveOccurred())

	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	out := []string{}

	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, match) {
			parts := strings.Split(line, " ")

			out = append(out, parts[loc])
		}
	}

	return out
}

func baseDir() string {
	wd, err := os.Getwd()
	gm.Expect(err).NotTo(gm.HaveOccurred())

	return filepath.Dir(filepath.Dir(wd))
}
