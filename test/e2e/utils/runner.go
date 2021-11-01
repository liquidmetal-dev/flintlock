//go:build e2e
// +build e2e

package utils

import (
	"bufio"
	"errors"
	"os/exec"
	"strings"

	gk "github.com/onsi/ginkgo"
	gm "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	"github.com/weaveworks/flintlock/api/services/microvm/v1alpha1"
	"google.golang.org/grpc"
)

const (
	containerdBin    = "containerd"
	flintlockCmdDir  = "github.com/weaveworks/flintlock/cmd/flintlockd"
	containerdSocket = "/run/containerd-dev/containerd.sock"
	containerdCfg    = "/etc/containerd/config-dev.toml"
	grpcDialTarget   = "127.0.0.1:9090"
)

// Runner is a very poorly named thing and honestly idk what to call it.
// What it does is compile flintlockd and start containerd and flintlockd.
// So 'TestSetterUpper' did not sound as slick, but that is what it is.
// I am happy for literally any suggestions.
type Runner struct {
	flintlockdBin     string
	containerdSession *gexec.Session
	flintlockdSession *gexec.Session
	flintlockdConn    *grpc.ClientConn
}

// Setup is a helper for the e2e tests which:
// - compiles flintlockd
// - starts containerd
// - starts flintlockd
// - opens a connection to the grpc server
// - returns a new MicroVMClient which can then be used in testing
// All opened connections and started processes are saved for later shutdown.
// Teardown should be called before Setup in a defer.
func (r *Runner) Setup() v1alpha1.MicroVMClient {
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

	if r.flintlockdSession != nil {
		r.flintlockdSession.Terminate().Wait()
	}

	if r.containerdSession != nil {
		r.containerdSession.Terminate().Wait()
	}

	gexec.CleanupBuildArtifacts()
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
	flCmd := exec.Command(r.flintlockdBin, "run", "--containerd-socket", containerdSocket, "--parent-iface", parentIface) //nolint:gosec
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
	// If there is a go package which lets me do this without shelling out lmk.
	// I could not find one after a quick search.
	cmd := exec.Command("ip", "route", "show")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, "default") {
			parts := strings.Split(line, " ")

			return parts[4], nil
		}
	}

	return "", errors.New("parent interface not found") //nolint:goerr113
}
