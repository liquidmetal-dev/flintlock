//go:build e2e
// +build e2e

package utils

import (
	"bufio"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	ccfg "github.com/containerd/containerd/services/server/config"
	"github.com/containerd/containerd/snapshots/devmapper/dmsetup"
	gk "github.com/onsi/ginkgo"
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
	makeDirectories()
	r.createThinPools()
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
		return
	}

	if r.flintlockdSession != nil {
		r.flintlockdSession.Terminate().Wait()
	}

	if r.containerdSession != nil {
		r.containerdSession.Terminate().Wait()
	}

	r.cleanupThinPools()
	cleanupDirectories()

	gexec.CleanupBuildArtifacts()
}

func makeDirectories() {
	gm.Expect(os.MkdirAll(devMapperRoot, os.ModePerm)).To(gm.Succeed())
	gm.Expect(os.MkdirAll(containerdStateDir, os.ModePerm)).To(gm.Succeed())
	gm.Expect(os.MkdirAll(containerdCfgDir, os.ModePerm)).To(gm.Succeed())
}

func cleanupDirectories() {
	gm.Expect(os.RemoveAll(containerdCfgDir)).To(gm.Succeed())
	gm.Expect(os.RemoveAll(containerdRootDir)).To(gm.Succeed())
	gm.Expect(os.RemoveAll(containerdStateDir)).To(gm.Succeed())
}

func (r *Runner) createThinPools() {
	if r.params.SkipSetupThinpool {
		return
	}

	scriptPath := filepath.Join(baseDir(), "hack", "scripts", "devpool.sh")
	command := exec.Command(scriptPath, r.params.ThinpoolName, loopDeviceTag)
	session, err := gexec.Start(command, gk.GinkgoWriter, gk.GinkgoWriter)

	gm.Expect(err).NotTo(gm.HaveOccurred())
	gm.Eventually(session, "20s").Should(gexec.Exit(0))
}

func (r *Runner) cleanupThinPools() {
	if r.params.SkipSetupThinpool {
		return
	}

	gm.Expect(dmsetup.RemoveDevice(r.params.ThinpoolName, dmsetup.RemoveWithForce)).To(gm.Succeed())

	cmd := exec.Command("losetup")
	loopDevices := grep(cmd, loopDeviceTag, 0)

	for _, dev := range loopDevices {
		command := exec.Command("losetup", "-d", dev)
		session, err := gexec.Start(command, gk.GinkgoWriter, gk.GinkgoWriter)
		gm.Expect(err).NotTo(gm.HaveOccurred())
		gm.Eventually(session).Should(gexec.Exit(0))
	}
}

func (r *Runner) writeContainerdConfig() {
	dmplug := map[string]interface{}{
		"pool_name":       r.params.ThinpoolName,
		"root_path":       devMapperRoot,
		"base_image_size": "10GB",
		"discard_blocks":  true,
	}
	pluginTree, err := toml.TreeFromMap(dmplug)
	gm.Expect(err).NotTo(gm.HaveOccurred())

	cfg := ccfg.Config{
		Version: 2,
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
		Plugins: map[string]toml.Tree{
			"io.containerd.snapshotter.v1.devmapper": *pluginTree,
		},
	}

	f, err := os.Create(containerdCfg)
	gm.Expect(err).NotTo(gm.HaveOccurred())

	defer f.Close()

	gm.Expect(toml.NewEncoder(f).Encode(cfg)).To(gm.Succeed())
}

func (r *Runner) buildFLBinary() {
	flBin, err := gexec.Build(flintlockCmdDir)
	gm.Expect(err).NotTo(gm.HaveOccurred())

	r.flintlockdBin = flBin
}

func (r *Runner) startContainerd() {
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
		"--parent-iface", parentIface,
		"--verbosity", r.params.FlintlockdLogLevel,
		"--insecure",
	)
	flSess, err := gexec.Start(flCmd, gk.GinkgoWriter, gk.GinkgoWriter)
	gm.Expect(err).NotTo(gm.HaveOccurred())

	r.flintlockdSession = flSess
}

func (r *Runner) dialGRPCServer() {
	conn, err := grpc.Dial(grpcDialTarget, grpc.WithInsecure(), grpc.WithBlock())
	gm.Expect(err).NotTo(gm.HaveOccurred())

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
