package cloudhypervisor

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"testing"

	g "github.com/onsi/gomega"
	"github.com/spf13/afero"

	"github.com/liquidmetal-dev/flintlock/core/models"
	"github.com/liquidmetal-dev/flintlock/core/ports"
	"github.com/liquidmetal-dev/flintlock/pkg/cloudhypervisor"
)

const (
	testVMName      = "test"
	testVMNamespace = "ns"
	testVMUID       = "344780b0-6249-11ec-90d6-0242ac120003"
)

// newTestProvider returns a provider backed by a real (OS) filesystem rooted at a
// temp dir, along with the vmid string and the initialised vm state.
func newTestProvider(t *testing.T) (*provider, string, State) {
	t.Helper()

	fs := afero.NewOsFs()

	// Use a short root: the unix socket path derived from it must stay under the
	// ~108 char sun_path limit, which t.TempDir() (long test names) would blow.
	stateRoot, err := os.MkdirTemp("", "ch")
	g.Expect(err).NotTo(g.HaveOccurred())
	t.Cleanup(func() { _ = os.RemoveAll(stateRoot) })

	p := &provider{
		config: &Config{StateRoot: stateRoot},
		fs:     fs,
	}

	vmid, err := models.NewVMID(testVMName, testVMNamespace, testVMUID)
	g.Expect(err).NotTo(g.HaveOccurred())

	vmState := NewState(*vmid, stateRoot, fs)
	g.Expect(fs.MkdirAll(vmState.Root(), 0o755)).To(g.Succeed())

	return p, vmid.String(), vmState
}

// startLiveProcess spawns a long-lived process and returns its pid. The process is
// killed on test cleanup.
func startLiveProcess(t *testing.T) int {
	t.Helper()

	cmd := exec.Command("sleep", "60")
	g.Expect(cmd.Start()).To(g.Succeed())

	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
	})

	return cmd.Process.Pid
}

// deadPID spawns then reaps a process, returning a pid that is no longer alive.
func deadPID(t *testing.T) int {
	t.Helper()

	cmd := exec.Command("sleep", "60")
	g.Expect(cmd.Start()).To(g.Succeed())
	g.Expect(cmd.Process.Kill()).To(g.Succeed())
	_, _ = cmd.Process.Wait()

	return cmd.Process.Pid
}

// serveFakeCH stands up a cloud-hypervisor-like API server on the state's socket
// path. If chState is empty the /vm.info endpoint returns 500 to simulate a
// transient error while the process is still coming up.
func serveFakeCH(t *testing.T, sockPath string, chState cloudhypervisor.VMState) {
	t.Helper()

	listener, err := net.Listen("unix", sockPath)
	g.Expect(err).NotTo(g.HaveOccurred())

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/vm.info", func(w http.ResponseWriter, _ *http.Request) {
		if chState == "" {
			w.WriteHeader(http.StatusInternalServerError)

			return
		}

		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"state":%q}`, chState)
	})

	srv := &http.Server{Handler: mux} //nolint:gosec // test server
	go func() { _ = srv.Serve(listener) }()

	t.Cleanup(func() { _ = srv.Close() })
}

func TestProviderState(t *testing.T) {
	g.RegisterTestingT(t)
	ctx := context.Background()

	t.Run("pid file absent returns pending", func(t *testing.T) {
		g.RegisterTestingT(t)
		p, id, _ := newTestProvider(t)

		state, err := p.State(ctx, id)
		g.Expect(err).NotTo(g.HaveOccurred())
		g.Expect(state).To(g.Equal(ports.MicroVMStatePending))
	})

	t.Run("process not alive returns pending", func(t *testing.T) {
		g.RegisterTestingT(t)
		p, id, vmState := newTestProvider(t)
		g.Expect(vmState.SetPid(deadPID(t))).To(g.Succeed())

		state, err := p.State(ctx, id)
		g.Expect(err).NotTo(g.HaveOccurred())
		g.Expect(state).To(g.Equal(ports.MicroVMStatePending))
	})

	t.Run("process alive but socket not yet bound returns running", func(t *testing.T) {
		g.RegisterTestingT(t)
		p, id, vmState := newTestProvider(t)
		g.Expect(vmState.SetPid(startLiveProcess(t))).To(g.Succeed())

		state, err := p.State(ctx, id)
		g.Expect(err).NotTo(g.HaveOccurred())
		g.Expect(state).To(g.Equal(ports.MicroVMStateRunning))
	})

	t.Run("process alive and info errors returns running", func(t *testing.T) {
		g.RegisterTestingT(t)
		p, id, vmState := newTestProvider(t)
		g.Expect(vmState.SetPid(startLiveProcess(t))).To(g.Succeed())
		serveFakeCH(t, vmState.SockPath(), "")

		state, err := p.State(ctx, id)
		g.Expect(err).NotTo(g.HaveOccurred())
		g.Expect(state).To(g.Equal(ports.MicroVMStateRunning))
	})

	t.Run("info created returns running", func(t *testing.T) {
		g.RegisterTestingT(t)
		p, id, vmState := newTestProvider(t)
		g.Expect(vmState.SetPid(startLiveProcess(t))).To(g.Succeed())
		serveFakeCH(t, vmState.SockPath(), cloudhypervisor.VMStateCreated)

		state, err := p.State(ctx, id)
		g.Expect(err).NotTo(g.HaveOccurred())
		g.Expect(state).To(g.Equal(ports.MicroVMStateRunning))
	})

	t.Run("info running returns running", func(t *testing.T) {
		g.RegisterTestingT(t)
		p, id, vmState := newTestProvider(t)
		g.Expect(vmState.SetPid(startLiveProcess(t))).To(g.Succeed())
		serveFakeCH(t, vmState.SockPath(), cloudhypervisor.VMStateRunning)

		state, err := p.State(ctx, id)
		g.Expect(err).NotTo(g.HaveOccurred())
		g.Expect(state).To(g.Equal(ports.MicroVMStateRunning))
	})

	t.Run("info shutdown returns unknown with error", func(t *testing.T) {
		g.RegisterTestingT(t)
		p, id, vmState := newTestProvider(t)
		g.Expect(vmState.SetPid(startLiveProcess(t))).To(g.Succeed())
		serveFakeCH(t, vmState.SockPath(), cloudhypervisor.VMStateShutdown)

		state, err := p.State(ctx, id)
		g.Expect(err).To(g.HaveOccurred())
		g.Expect(state).To(g.Equal(ports.MicroVMStateUnknown))
	})
}
