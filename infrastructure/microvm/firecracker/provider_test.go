package firecracker

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"testing"

	g "github.com/onsi/gomega"
	"github.com/spf13/afero"

	"github.com/liquidmetal-dev/flintlock/core/models"
	"github.com/liquidmetal-dev/flintlock/core/ports"
	"github.com/liquidmetal-dev/flintlock/pkg/firecracker"
)

const (
	testVMName      = "test"
	testVMNamespace = "ns"
	testVMUID       = "344780b0-6249-11ec-90d6-0242ac120003"
)

func newTestProvider(t *testing.T) (*fcProvider, *models.MicroVM, State) {
	t.Helper()

	fs := afero.NewOsFs()

	// A short root keeps the API socket path inside the unix socket path limit.
	root, err := os.MkdirTemp("", "fc")
	g.Expect(err).NotTo(g.HaveOccurred())
	t.Cleanup(func() { _ = os.RemoveAll(root) })

	p := &fcProvider{
		config: &Config{StateRoot: root, SocketDir: root},
		fs:     fs,
	}

	vmid, err := models.NewVMID(testVMName, testVMNamespace, testVMUID)
	g.Expect(err).NotTo(g.HaveOccurred())

	vmState := NewState(*vmid, root, root, fs)
	g.Expect(fs.MkdirAll(vmState.Root(), 0o755)).To(g.Succeed())
	g.Expect(fs.MkdirAll(vmState.SocketRoot(), 0o755)).To(g.Succeed())

	return p, &models.MicroVM{ID: *vmid}, vmState
}

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

// serveFakeFirecracker serves a Firecracker-like API on sockPath. GET / reports
// fcState, and every PATCH /vm body is sent on the returned channel.
func serveFakeFirecracker(t *testing.T, sockPath string, fcState firecracker.InstanceState) <-chan string {
	t.Helper()

	listener, err := net.Listen("unix", sockPath)
	g.Expect(err).NotTo(g.HaveOccurred())

	patches := make(chan string, 1)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"id":%q,"state":%q}`, testVMUID, fcState)
	})
	mux.HandleFunc("PATCH /vm", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		patches <- string(body)
		w.WriteHeader(http.StatusNoContent)
	})

	srv := &http.Server{Handler: mux} //nolint:gosec // test server
	go func() { _ = srv.Serve(listener) }()

	t.Cleanup(func() { _ = srv.Close() })

	return patches
}

func TestProviderState(t *testing.T) {
	ctx := context.Background()

	t.Run("pid file absent returns pending", func(t *testing.T) {
		g.RegisterTestingT(t)
		p, vm, _ := newTestProvider(t)

		state, err := p.State(ctx, vm.ID.String())
		g.Expect(err).NotTo(g.HaveOccurred())
		g.Expect(state).To(g.Equal(ports.MicroVMStatePending))
	})

	t.Run("process alive and socket not answering returns running", func(t *testing.T) {
		g.RegisterTestingT(t)
		p, vm, vmState := newTestProvider(t)
		g.Expect(vmState.SetPid(startLiveProcess(t))).To(g.Succeed())

		state, err := p.State(ctx, vm.ID.String())
		g.Expect(err).NotTo(g.HaveOccurred())
		g.Expect(state).To(g.Equal(ports.MicroVMStateRunning))
	})

	mapped := []struct {
		fcState firecracker.InstanceState
		want    ports.MicroVMState
	}{
		{fcState: firecracker.InstanceStateRunning, want: ports.MicroVMStateRunning},
		{fcState: firecracker.InstanceStatePaused, want: ports.MicroVMStatePaused},
		{fcState: firecracker.InstanceStateNotStarted, want: ports.MicroVMStateRunning},
	}

	for _, tc := range mapped {
		t.Run(fmt.Sprintf("api state %s returns %s", tc.fcState, tc.want), func(t *testing.T) {
			g.RegisterTestingT(t)
			p, vm, vmState := newTestProvider(t)
			g.Expect(vmState.SetPid(startLiveProcess(t))).To(g.Succeed())
			serveFakeFirecracker(t, vmState.APISockPath(), tc.fcState)

			state, err := p.State(ctx, vm.ID.String())
			g.Expect(err).NotTo(g.HaveOccurred())
			g.Expect(state).To(g.Equal(tc.want))
		})
	}

	t.Run("unknown api state returns unknown with error", func(t *testing.T) {
		g.RegisterTestingT(t)
		p, vm, vmState := newTestProvider(t)
		g.Expect(vmState.SetPid(startLiveProcess(t))).To(g.Succeed())
		serveFakeFirecracker(t, vmState.APISockPath(), "Exploded")

		state, err := p.State(ctx, vm.ID.String())
		g.Expect(err).To(g.HaveOccurred())
		g.Expect(state).To(g.Equal(ports.MicroVMStateUnknown))
	})
}

func TestProviderPauseResume(t *testing.T) {
	ctx := context.Background()

	cases := []struct {
		name string
		call func(p *fcProvider, vm *models.MicroVM) error
		body string
	}{
		{
			name: "pause",
			call: func(p *fcProvider, vm *models.MicroVM) error { return p.Pause(ctx, vm) },
			body: `{"state":"Paused"}`,
		},
		{
			name: "resume",
			call: func(p *fcProvider, vm *models.MicroVM) error { return p.Resume(ctx, vm) },
			body: `{"state":"Resumed"}`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			g.RegisterTestingT(t)
			p, vm, vmState := newTestProvider(t)
			patches := serveFakeFirecracker(t, vmState.APISockPath(), firecracker.InstanceStateRunning)

			g.Expect(tc.call(p, vm)).To(g.Succeed())
			g.Expect(<-patches).To(g.MatchJSON(tc.body))
		})

		t.Run(tc.name+" without api socket fails", func(t *testing.T) {
			g.RegisterTestingT(t)
			p, vm, _ := newTestProvider(t)

			g.Expect(tc.call(p, vm)).NotTo(g.Succeed())
		})
	}
}
