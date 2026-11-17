package firecracker

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"syscall"
	"testing"
	"time"

	g "github.com/onsi/gomega"

	cerrs "github.com/liquidmetal-dev/flintlock/core/errors"
	"github.com/liquidmetal-dev/flintlock/core/models"
	"github.com/liquidmetal-dev/flintlock/core/ports"
)

type apiCall struct {
	method string
	path   string
	body   string
}

// fakeAPI is a Firecracker-like API on a unix socket. It records every call
// in order and fails the calls to failPath.
type fakeAPI struct {
	mu       sync.Mutex
	calls    []apiCall
	failPath string
	// onCall runs for each call that does not fail, before the answer.
	onCall func(call apiCall)
}

func (f *fakeAPI) serve(t *testing.T, sockPath string) {
	t.Helper()

	listener, err := net.Listen("unix", sockPath)
	g.Expect(err).NotTo(g.HaveOccurred())

	srv := &http.Server{ //nolint:gosec // test server
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			body, _ := io.ReadAll(r.Body)
			call := apiCall{method: r.Method, path: r.URL.Path, body: string(body)}

			f.mu.Lock()
			f.calls = append(f.calls, call)
			f.mu.Unlock()

			if r.URL.Path == f.failPath {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusBadRequest)
				fmt.Fprint(w, `{"fault_message":"cannot load snapshot"}`)

				return
			}

			if f.onCall != nil {
				f.onCall(call)
			}

			if r.Method == http.MethodGet && r.URL.Path == "/" {
				w.Header().Set("Content-Type", "application/json")
				fmt.Fprintf(w, `{"id":%q,"state":"Not started"}`, testVMUID)

				return
			}

			w.WriteHeader(http.StatusNoContent)
		}),
	}

	go func() { _ = srv.Serve(listener) }()

	t.Cleanup(func() { _ = srv.Close() })
}

func (f *fakeAPI) recorded() []apiCall {
	f.mu.Lock()
	defer f.mu.Unlock()

	return append([]apiCall(nil), f.calls...)
}

func (f *fakeAPI) call(method, path string) (apiCall, bool) {
	for _, c := range f.recorded() {
		if c.method == method && c.path == path {
			return c, true
		}
	}

	return apiCall{}, false
}

// startedVMM records the command that Restore starts.
type startedVMM struct {
	args []string
	dir  string
	pid  int
}

// fakeStart returns a startVMM function. It records the command, starts api on
// the API socket if api is not nil, and runs sleep in place of firecracker.
func fakeStart(t *testing.T, vmState State, api *fakeAPI, started *startedVMM) func(*exec.Cmd, bool) error {
	t.Helper()

	return func(cmd *exec.Cmd, _ bool) error {
		started.args = append([]string(nil), cmd.Args...)
		started.dir = cmd.Dir

		if api != nil {
			api.serve(t, vmState.APISockPath())
		}

		sleep, err := exec.LookPath("sleep")
		if err != nil {
			return err
		}

		cmd.Path = sleep
		cmd.Args = []string{"sleep", "60"}
		cmd.Err = nil // Set when the firecracker binary is not on the PATH.

		if err := cmd.Start(); err != nil {
			return err
		}

		started.pid = cmd.Process.Pid

		t.Cleanup(func() {
			_ = cmd.Process.Kill()
			_, _ = cmd.Process.Wait()
		})

		return nil
	}
}

func restoreVM(vm *models.MicroVM) *models.MicroVM {
	vm.Spec = models.MicroVMSpec{
		AllowGuestAgent: true,
		Metadata: map[string]string{
			"user-data": base64.StdEncoding.EncodeToString([]byte("#cloud-config")),
			"restore":   "marker",
		},
		NetworkInterfaces: []models.NetworkInterface{
			{GuestDeviceName: "eth0", Type: models.IfaceTypeTap, AllowMetadataRequests: true},
			{GuestDeviceName: "eth1", Type: models.IfaceTypeMacvtap},
		},
	}
	vm.Status = models.MicroVMStatus{
		NetworkInterfaces: models.NetworkInterfaceStatuses{
			"eth0": {HostDeviceName: "fltapnew0"},
			"eth1": {HostDeviceName: "flmacvtapnew1", Index: 42},
		},
	}

	return vm
}

func processGone(pid int) bool {
	return errors.Is(syscall.Kill(pid, 0), syscall.ESRCH)
}

func TestSnapshot(t *testing.T) {
	g.RegisterTestingT(t)

	p, vm, vmState := newTestProvider(t)
	dest := t.TempDir()

	// Firecracker writes the files with its own mode.
	api := &fakeAPI{onCall: func(call apiCall) {
		if call.path != "/snapshot/create" {
			return
		}

		params := map[string]string{}
		_ = json.Unmarshal([]byte(call.body), &params)
		_ = os.WriteFile(params["snapshot_path"], []byte("state"), 0o644)  //nolint:gosec // test
		_ = os.WriteFile(params["mem_file_path"], []byte("memory"), 0o644) //nolint:gosec // test
	}}
	api.serve(t, vmState.APISockPath())

	g.Expect(p.Snapshot(context.Background(), vm, dest)).To(g.Succeed())

	calls := api.recorded()
	g.Expect(calls).To(g.HaveLen(1))
	g.Expect(calls[0].method).To(g.Equal(http.MethodPut))
	g.Expect(calls[0].path).To(g.Equal("/snapshot/create"))
	g.Expect(calls[0].body).To(g.MatchJSON(fmt.Sprintf(
		`{"snapshot_type":"Full","snapshot_path":%q,"mem_file_path":%q}`,
		filepath.Join(dest, "vmstate"), filepath.Join(dest, "memory"),
	)))

	for _, name := range []string{"vmstate", "memory"} {
		info, err := os.Stat(filepath.Join(dest, name))
		g.Expect(err).NotTo(g.HaveOccurred())
		g.Expect(info.Mode().Perm()).To(g.Equal(os.FileMode(0o600)), name)
	}
}

func TestSnapshot_Failures(t *testing.T) {
	t.Run("relative destination", func(t *testing.T) {
		g.RegisterTestingT(t)
		p, vm, _ := newTestProvider(t)

		g.Expect(p.Snapshot(context.Background(), vm, "snapshots/x")).To(g.MatchError(errSnapshotPathNotAbsolute))
	})

	t.Run("api error", func(t *testing.T) {
		g.RegisterTestingT(t)
		p, vm, vmState := newTestProvider(t)
		api := &fakeAPI{failPath: "/snapshot/create"}
		api.serve(t, vmState.APISockPath())

		g.Expect(p.Snapshot(context.Background(), vm, t.TempDir())).To(g.MatchError(g.ContainSubstring("cannot load snapshot")))
	})

	t.Run("files missing after the call", func(t *testing.T) {
		g.RegisterTestingT(t)
		p, vm, vmState := newTestProvider(t)
		api := &fakeAPI{}
		api.serve(t, vmState.APISockPath())

		g.Expect(p.Snapshot(context.Background(), vm, t.TempDir())).NotTo(g.Succeed())
	})
}

func TestRestore(t *testing.T) {
	g.RegisterTestingT(t)

	p, vm, vmState := newTestProvider(t)
	p.config.FirecrackerBin = "/usr/bin/firecracker"
	vm = restoreVM(vm)
	src := t.TempDir()

	api := &fakeAPI{}
	started := &startedVMM{}
	p.startVMM = fakeStart(t, vmState, api, started)

	err := p.Restore(context.Background(), vm, src, ports.RestoreInput{MemoryMode: models.MemoryModeFile})
	g.Expect(err).NotTo(g.HaveOccurred())

	// The command line has no config file and no metadata, and runs in the
	// state directory.
	g.Expect(started.dir).To(g.Equal(vmState.Root()))
	g.Expect(started.args).To(g.Equal([]string{
		"/usr/bin/firecracker",
		"--id", testVMUID,
		"--boot-timer",
		"--api-sock", vmState.APISockPath(),
		"--log-path", vmState.LogPath(),
		"--level", "Debug",
		"--show-level",
		"--show-log-origin",
		"--metrics-path", vmState.MetricsPath(),
	}))

	pid, err := vmState.PID()
	g.Expect(err).NotTo(g.HaveOccurred())
	g.Expect(pid).To(g.Equal(started.pid))

	g.Expect(vm.Status.VSockPath).To(g.Equal(vmState.VSockPath()))

	target, err := os.Readlink(vmState.SocketLinkPath())
	g.Expect(err).NotTo(g.HaveOccurred())
	g.Expect(target).To(g.Equal(vmState.SocketRoot()))

	// The calls come in order: the socket answers, the load, the metadata,
	// then the resume.
	calls := api.recorded()
	order := make([]string, 0, len(calls))

	for _, c := range calls {
		order = append(order, c.method+" "+c.path)
	}

	g.Expect(order).To(g.Equal([]string{
		"GET /",
		"PUT /snapshot/load",
		"PUT /mmds",
		"PATCH /vm",
	}))

	g.Expect(calls[1].body).To(g.MatchJSON(fmt.Sprintf(`{
		"snapshot_path": %q,
		"mem_backend": {"backend_type": "File", "backend_path": %q},
		"network_overrides": [
			{"iface_id": "eth0", "host_dev_name": "fltapnew0"},
			{"iface_id": "eth1", "host_dev_name": "/dev/tap42"}
		],
		"resume_vm": false
	}`, filepath.Join(src, "vmstate"), filepath.Join(src, "memory"))))
	g.Expect(calls[2].body).To(g.MatchJSON(`{"latest":{"user-data":"#cloud-config","restore":"marker"}}`))
	g.Expect(calls[3].body).To(g.MatchJSON(`{"state":"Resumed"}`))
}

func TestRestore_DefaultMemoryMode(t *testing.T) {
	g.RegisterTestingT(t)

	p, vm, vmState := newTestProvider(t)
	vm = restoreVM(vm)
	api := &fakeAPI{}
	p.startVMM = fakeStart(t, vmState, api, &startedVMM{})

	g.Expect(p.Restore(context.Background(), vm, t.TempDir(), ports.RestoreInput{})).To(g.Succeed())

	_, resumed := api.call(http.MethodPatch, "/vm")
	g.Expect(resumed).To(g.BeTrue())
}

func TestRestore_NoMetadataInterface(t *testing.T) {
	g.RegisterTestingT(t)

	p, vm, vmState := newTestProvider(t)
	vm = restoreVM(vm)

	for i := range vm.Spec.NetworkInterfaces {
		vm.Spec.NetworkInterfaces[i].AllowMetadataRequests = false
	}

	api := &fakeAPI{}
	p.startVMM = fakeStart(t, vmState, api, &startedVMM{})

	g.Expect(p.Restore(context.Background(), vm, t.TempDir(), ports.RestoreInput{})).To(g.Succeed())

	// The snapshot has no MMDS data store, so there is no put.
	_, put := api.call(http.MethodPut, "/mmds")
	g.Expect(put).To(g.BeFalse())

	_, resumed := api.call(http.MethodPatch, "/vm")
	g.Expect(resumed).To(g.BeTrue())
}

func TestRestore_CleansUpOnFailure(t *testing.T) {
	cases := []struct {
		name     string
		failPath string
		noAPI    bool
	}{
		{name: "load fails", failPath: "/snapshot/load"},
		{name: "metadata put fails", failPath: "/mmds"},
		{name: "resume fails", failPath: "/vm"},
		{name: "api socket never answers", noAPI: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			g.RegisterTestingT(t)

			p, vm, vmState := newTestProvider(t)
			p.apiReadyTimeout = 200 * time.Millisecond
			vm = restoreVM(vm)

			var api *fakeAPI
			if !tc.noAPI {
				api = &fakeAPI{failPath: tc.failPath}
			}

			started := &startedVMM{}
			p.startVMM = fakeStart(t, vmState, api, started)

			err := p.Restore(context.Background(), vm, t.TempDir(), ports.RestoreInput{})
			g.Expect(err).To(g.HaveOccurred())

			g.Expect(started.pid).NotTo(g.BeZero())
			g.Expect(processGone(started.pid)).To(g.BeTrue())

			for _, path := range []string{vmState.PIDPath(), vmState.APISockPath()} {
				_, statErr := os.Stat(path)
				g.Expect(errors.Is(statErr, os.ErrNotExist)).To(g.BeTrue(), path)
			}

			if api != nil && tc.failPath != "/vm" {
				_, resumed := api.call(http.MethodPatch, "/vm")
				g.Expect(resumed).To(g.BeFalse())
			}
		})
	}
}

func TestRestore_FailsBeforeStart(t *testing.T) {
	cases := []struct {
		name    string
		src     string
		in      ports.RestoreInput
		mutate  func(vm *models.MicroVM)
		matches func(err error) bool
	}{
		{
			name:    "copy memory mode",
			in:      ports.RestoreInput{MemoryMode: models.MemoryModeCopy},
			matches: cerrs.IsNotSupported,
		},
		{
			name:    "relative source",
			src:     "snapshots/x",
			matches: func(err error) bool { return errors.Is(err, errSnapshotPathNotAbsolute) },
		},
		{
			name:   "network interface status missing",
			mutate: func(vm *models.MicroVM) { delete(vm.Status.NetworkInterfaces, "eth1") },
			matches: func(err error) bool {
				var missing cerrs.NetworkInterfaceStatusMissingError

				return errors.As(err, &missing)
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			g.RegisterTestingT(t)

			p, vm, _ := newTestProvider(t)
			vm = restoreVM(vm)

			if tc.mutate != nil {
				tc.mutate(vm)
			}

			src := tc.src
			if src == "" {
				src = t.TempDir()
			}

			p.startVMM = func(*exec.Cmd, bool) error {
				t.Fatal("firecracker must not start")

				return nil
			}

			err := p.Restore(context.Background(), vm, src, tc.in)
			g.Expect(err).To(g.HaveOccurred())
			g.Expect(tc.matches(err)).To(g.BeTrue(), err.Error())
		})
	}
}

func TestCapabilities_HasSnapshot(t *testing.T) {
	g.RegisterTestingT(t)

	p := &fcProvider{config: &Config{}}

	g.Expect(p.Capabilities()).To(g.ContainElement(models.SnapshotCapability))
}

// TestRestore_Twice restores one snapshot into two microvms on one provider.
// Each firecracker process gets its own working directory, sockets and pid
// file, and both load the same state and memory files.
func TestRestore_Twice(t *testing.T) {
	g.RegisterTestingT(t)

	p, first, firstState := newTestProvider(t)
	first = restoreVM(first)

	secondID, err := models.NewVMID(testVMName+"-2", testVMNamespace, "344780b0-6249-11ec-90d6-0242ac120004")
	g.Expect(err).NotTo(g.HaveOccurred())

	second := restoreVM(&models.MicroVM{ID: *secondID})
	second.Status.NetworkInterfaces["eth0"].HostDeviceName = "fltapnew2"
	secondState := NewState(*secondID, p.config.StateRoot, p.config.SocketDir, p.fs)

	src := t.TempDir()
	runs := []struct {
		vm      *models.MicroVM
		state   State
		api     *fakeAPI
		started *startedVMM
	}{
		{vm: first, state: firstState, api: &fakeAPI{}, started: &startedVMM{}},
		{vm: second, state: secondState, api: &fakeAPI{}, started: &startedVMM{}},
	}

	for _, run := range runs {
		p.startVMM = fakeStart(t, run.state, run.api, run.started)

		err := p.Restore(context.Background(), run.vm, src, ports.RestoreInput{MemoryMode: models.MemoryModeFile})
		g.Expect(err).NotTo(g.HaveOccurred())
	}

	a, b := runs[0], runs[1]

	g.Expect(a.started.dir).To(g.Equal(firstState.Root()))
	g.Expect(b.started.dir).To(g.Equal(secondState.Root()))
	g.Expect(a.started.dir).NotTo(g.Equal(b.started.dir))
	g.Expect(a.started.pid).NotTo(g.Equal(b.started.pid))
	g.Expect(firstState.APISockPath()).NotTo(g.Equal(secondState.APISockPath()))
	g.Expect(first.Status.VSockPath).NotTo(g.Equal(second.Status.VSockPath))

	// Each sock link names the socket directory of its own microvm, so the
	// relative vsock path in the snapshot resolves to a different socket.
	for _, run := range runs {
		target, err := os.Readlink(run.state.SocketLinkPath())
		g.Expect(err).NotTo(g.HaveOccurred())
		g.Expect(target).To(g.Equal(run.state.SocketRoot()))

		pid, err := run.state.PID()
		g.Expect(err).NotTo(g.HaveOccurred())
		g.Expect(pid).To(g.Equal(run.started.pid))
	}

	g.Expect(firstState.SocketRoot()).NotTo(g.Equal(secondState.SocketRoot()))

	// Both load the same files, each with its own host devices.
	loadA, ok := a.api.call(http.MethodPut, "/snapshot/load")
	g.Expect(ok).To(g.BeTrue())
	loadB, ok := b.api.call(http.MethodPut, "/snapshot/load")
	g.Expect(ok).To(g.BeTrue())

	type load struct {
		SnapshotPath string `json:"snapshot_path"`
		MemBackend   struct {
			BackendPath string `json:"backend_path"`
		} `json:"mem_backend"`
		NetworkOverrides []struct {
			HostDevName string `json:"host_dev_name"`
		} `json:"network_overrides"`
	}

	var bodyA, bodyB load
	g.Expect(json.Unmarshal([]byte(loadA.body), &bodyA)).To(g.Succeed())
	g.Expect(json.Unmarshal([]byte(loadB.body), &bodyB)).To(g.Succeed())

	g.Expect(bodyA.SnapshotPath).To(g.Equal(filepath.Join(src, models.SnapshotVMStateFile)))
	g.Expect(bodyB.SnapshotPath).To(g.Equal(bodyA.SnapshotPath))
	g.Expect(bodyA.MemBackend.BackendPath).To(g.Equal(filepath.Join(src, models.SnapshotMemoryFile)))
	g.Expect(bodyB.MemBackend.BackendPath).To(g.Equal(bodyA.MemBackend.BackendPath))
	g.Expect(bodyA.NetworkOverrides[0].HostDevName).To(g.Equal("fltapnew0"))
	g.Expect(bodyB.NetworkOverrides[0].HostDevName).To(g.Equal("fltapnew2"))
}
