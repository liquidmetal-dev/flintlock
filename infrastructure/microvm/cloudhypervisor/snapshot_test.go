package cloudhypervisor

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	g "github.com/onsi/gomega"

	cerrs "github.com/liquidmetal-dev/flintlock/core/errors"
	"github.com/liquidmetal-dev/flintlock/core/models"
	"github.com/liquidmetal-dev/flintlock/core/ports"

	virtiofs "github.com/liquidmetal-dev/flintlock/infrastructure/virtiofs"
)

type apiCall struct {
	method string
	path   string
	body   string
}

// fakeAPI is a cloud-hypervisor-like API on a unix socket. It records every
// call in order and fails the calls to failPath.
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
				w.WriteHeader(http.StatusInternalServerError)
				fmt.Fprint(w, "snapshot failed")

				return
			}

			if f.onCall != nil {
				f.onCall(call)
			}

			if r.URL.Path == "/api/v1/vmm.ping" {
				w.Header().Set("Content-Type", "application/json")
				fmt.Fprint(w, `{"version":"v54.1"}`)

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

func (f *fakeAPI) called(method, path string) bool {
	for _, c := range f.recorded() {
		if c.method == method && c.path == path {
			return true
		}
	}

	return false
}

// startedVMM records the command that Restore starts.
type startedVMM struct {
	args []string
	pid  int
	// restoreDir holds the files of the restore directory when the process
	// starts, with the target of each link.
	restoreDir map[string]string
	config     []byte
}

// fakeStart returns a startVMM function. It records the command and the
// restore directory, starts api on the API socket if api is not nil, and runs
// sleep in place of cloud-hypervisor.
func fakeStart(t *testing.T, vmState State, api *fakeAPI, started *startedVMM) func(*exec.Cmd, bool) error {
	t.Helper()

	return func(cmd *exec.Cmd, _ bool) error {
		started.args = append([]string(nil), cmd.Args...)
		started.restoreDir = map[string]string{}

		entries, err := os.ReadDir(vmState.RestoreDir())
		if err != nil {
			return err
		}

		for _, entry := range entries {
			path := filepath.Join(vmState.RestoreDir(), entry.Name())
			target, _ := os.Readlink(path)
			started.restoreDir[entry.Name()] = target
		}

		started.config, _ = os.ReadFile(filepath.Join(vmState.RestoreDir(), "config.json"))

		if api != nil {
			api.serve(t, vmState.SockPath())
		}

		sleep, err := exec.LookPath("sleep")
		if err != nil {
			return err
		}

		cmd.Path = sleep
		cmd.Args = []string{"sleep", "60"}
		cmd.Err = nil

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

// fakeDiskService writes an empty file in place of a disk image.
type fakeDiskService struct{}

func (fakeDiskService) Create(_ context.Context, input ports.DiskCreateInput) error {
	return os.WriteFile(input.Path, nil, 0o600)
}

func testVM(t *testing.T, id string) *models.MicroVM {
	t.Helper()

	vmid, err := models.NewVMIDFromString(id)
	g.Expect(err).NotTo(g.HaveOccurred())

	return &models.MicroVM{ID: *vmid}
}

func restoreVM(t *testing.T, id string) *models.MicroVM {
	t.Helper()

	vm := testVM(t, id)
	vm.Spec = models.MicroVMSpec{
		AllowGuestAgent: true,
		Metadata:        map[string]string{},
		RootVolume:      models.Volume{ID: "root"},
		AdditionalVolumes: models.Volumes{
			{ID: "data"},
			{ID: "share", Source: models.VolumeSource{VirtioFS: &models.VirtioFSVolumeSource{}}},
		},
		NetworkInterfaces: []models.NetworkInterface{
			{GuestDeviceName: "eth0", Type: models.IfaceTypeTap},
			{GuestDeviceName: "eth1", Type: models.IfaceTypeTap},
		},
	}
	vm.Status = models.MicroVMStatus{
		Volumes: models.VolumeStatuses{
			"root":  {Mount: models.Mount{Source: "/dev/mapper/new-root"}},
			"data":  {Mount: models.Mount{Source: "/dev/mapper/new-data"}},
			"share": {},
		},
		NetworkInterfaces: models.NetworkInterfaceStatuses{
			"eth0": {HostDeviceName: "fltapnew0"},
			"eth1": {HostDeviceName: "fltapnew1"},
		},
	}

	return vm
}

// writeSnapshot writes a cloud-hypervisor snapshot of the fixture config to
// src/vmstate.
func writeSnapshot(t *testing.T, src string) {
	t.Helper()

	dir := filepath.Join(src, models.SnapshotVMStateFile)
	g.Expect(os.Mkdir(dir, 0o700)).To(g.Succeed())

	config, err := os.ReadFile(filepath.Join("testdata", "snapshot", "config.json"))
	g.Expect(err).NotTo(g.HaveOccurred())

	g.Expect(os.WriteFile(filepath.Join(dir, "config.json"), config, 0o600)).To(g.Succeed())
	g.Expect(os.WriteFile(filepath.Join(dir, "state.json"), []byte("{}"), 0o600)).To(g.Succeed())
	g.Expect(os.WriteFile(filepath.Join(dir, "memory-ranges"), []byte("memory"), 0o600)).To(g.Succeed())
}

func processGone(pid int) bool {
	return errors.Is(syscall.Kill(pid, 0), syscall.ESRCH)
}

func TestSnapshot(t *testing.T) {
	g.RegisterTestingT(t)

	p, id, vmState := newTestProvider(t)
	dest := t.TempDir()
	stateDir := filepath.Join(dest, "vmstate")

	// Cloud Hypervisor writes the files with its own mode.
	api := &fakeAPI{onCall: func(call apiCall) {
		if call.path != "/api/v1/vm.snapshot" {
			return
		}

		for _, name := range []string{"config.json", "state.json", "memory-ranges"} {
			_ = os.WriteFile(filepath.Join(stateDir, name), []byte(name), 0o644) //nolint:gosec // test
		}
	}}
	api.serve(t, vmState.SockPath())

	g.Expect(p.Snapshot(context.Background(), testVM(t, id), dest)).To(g.Succeed())

	calls := api.recorded()
	g.Expect(calls).To(g.HaveLen(1))
	g.Expect(calls[0].method).To(g.Equal(http.MethodPut))
	g.Expect(calls[0].path).To(g.Equal("/api/v1/vm.snapshot"))
	g.Expect(calls[0].body).To(g.MatchJSON(fmt.Sprintf(`{"destination_url":%q}`, "file://"+stateDir)))

	info, err := os.Stat(stateDir)
	g.Expect(err).NotTo(g.HaveOccurred())
	g.Expect(info.IsDir()).To(g.BeTrue())
	g.Expect(info.Mode().Perm()).To(g.Equal(os.FileMode(0o700)))

	for _, name := range []string{"config.json", "state.json", "memory-ranges"} {
		info, err := os.Stat(filepath.Join(stateDir, name))
		g.Expect(err).NotTo(g.HaveOccurred())
		g.Expect(info.Mode().Perm()).To(g.Equal(os.FileMode(0o600)), name)
	}
}

func TestSnapshot_Failures(t *testing.T) {
	t.Run("relative destination", func(t *testing.T) {
		g.RegisterTestingT(t)
		p, id, _ := newTestProvider(t)

		err := p.Snapshot(context.Background(), testVM(t, id), "snapshots/x")
		g.Expect(err).To(g.MatchError(errSnapshotPathNotAbsolute))
	})

	t.Run("api error", func(t *testing.T) {
		g.RegisterTestingT(t)
		p, id, vmState := newTestProvider(t)
		api := &fakeAPI{failPath: "/api/v1/vm.snapshot"}
		api.serve(t, vmState.SockPath())

		err := p.Snapshot(context.Background(), testVM(t, id), t.TempDir())
		g.Expect(err).To(g.MatchError(g.ContainSubstring("snapshot failed")))
	})

	t.Run("vmstate already exists", func(t *testing.T) {
		g.RegisterTestingT(t)
		p, id, vmState := newTestProvider(t)
		api := &fakeAPI{}
		api.serve(t, vmState.SockPath())

		dest := t.TempDir()
		g.Expect(os.Mkdir(filepath.Join(dest, "vmstate"), 0o700)).To(g.Succeed())

		g.Expect(p.Snapshot(context.Background(), testVM(t, id), dest)).NotTo(g.Succeed())
		g.Expect(api.recorded()).To(g.BeEmpty())
	})
}

func TestRestore(t *testing.T) {
	g.RegisterTestingT(t)

	p, id, vmState := newTestProvider(t)
	p.config.CloudHypervisorBin = "/usr/bin/cloud-hypervisor"
	p.diskSvc = fakeDiskService{}
	vm := restoreVM(t, id)
	src := t.TempDir()
	writeSnapshot(t, src)

	api := &fakeAPI{}
	started := &startedVMM{}
	p.startVMM = fakeStart(t, vmState, api, started)

	err := p.Restore(context.Background(), vm, src, ports.RestoreInput{MemoryMode: models.MemoryModeCopy})
	g.Expect(err).NotTo(g.HaveOccurred())

	g.Expect(started.args).To(g.Equal([]string{
		"/usr/bin/cloud-hypervisor",
		"--api-socket", vmState.SockPath(),
		"--log-file", vmState.LogPath(),
		"-v",
		"--restore", "source_url=file://" + vmState.RestoreDir(),
	}))

	snapshotDir := filepath.Join(src, "vmstate")
	g.Expect(started.restoreDir).To(g.Equal(map[string]string{
		"config.json":   "",
		"state.json":    filepath.Join(snapshotDir, "state.json"),
		"memory-ranges": filepath.Join(snapshotDir, "memory-ranges"),
	}))

	var config jsonObject
	g.Expect(json.Unmarshal(started.config, &config)).To(g.Succeed())
	g.Expect(string(config["disks"])).To(g.ContainSubstring(`"path":"/dev/mapper/new-root"`))
	g.Expect(string(config["disks"])).To(g.ContainSubstring(fmt.Sprintf(`"path":%q`, vmState.CloudInitImage())))
	g.Expect(string(config["disks"])).To(g.ContainSubstring(`"path":"/dev/mapper/new-data"`))
	g.Expect(string(config["net"])).To(g.ContainSubstring(`"tap":"fltapnew0"`))
	g.Expect(string(config["net"])).To(g.ContainSubstring(`"tap":"fltapnew1"`))
	g.Expect(string(config["vsock"])).To(g.ContainSubstring(fmt.Sprintf(`"socket":%q`, vmState.VSockPath())))

	vfsSock := virtiofs.NewState(vm.ID, p.config.StateRoot, p.config.SocketDir, p.fs).ResolveVirtioFSPath()
	g.Expect(string(config["fs"])).To(g.ContainSubstring(fmt.Sprintf(`"socket":%q`, vfsSock)))
	g.Expect(string(config["serial"])).To(g.ContainSubstring(
		fmt.Sprintf(`"file":%q`, filepath.Join(vmState.Root(), "serial.log"))))

	pid, err := vmState.PID()
	g.Expect(err).NotTo(g.HaveOccurred())
	g.Expect(pid).To(g.Equal(started.pid))

	g.Expect(vm.Status.VSockPath).To(g.Equal(vmState.VSockPath()))

	_, err = os.Stat(vmState.CloudInitImage())
	g.Expect(err).NotTo(g.HaveOccurred())

	order := []string{}
	for _, c := range api.recorded() {
		order = append(order, c.method+" "+c.path)
	}

	g.Expect(order).To(g.Equal([]string{
		"GET /api/v1/vmm.ping",
		"PUT /api/v1/vm.resume",
	}))

	_, err = os.Stat(vmState.RestoreDir())
	g.Expect(errors.Is(err, os.ErrNotExist)).To(g.BeTrue())

	// The snapshot is left as it was.
	_, err = os.Stat(filepath.Join(snapshotDir, "memory-ranges"))
	g.Expect(err).NotTo(g.HaveOccurred())
}

func TestRestore_RestoreDirMode(t *testing.T) {
	g.RegisterTestingT(t)

	p, id, vmState := newTestProvider(t)
	p.diskSvc = fakeDiskService{}
	src := t.TempDir()
	writeSnapshot(t, src)

	var dirMode, configMode os.FileMode

	p.startVMM = func(cmd *exec.Cmd, detached bool) error {
		if info, err := os.Stat(vmState.RestoreDir()); err == nil {
			dirMode = info.Mode().Perm()
		}

		if info, err := os.Stat(filepath.Join(vmState.RestoreDir(), "config.json")); err == nil {
			configMode = info.Mode().Perm()
		}

		return fakeStart(t, vmState, &fakeAPI{}, &startedVMM{})(cmd, detached)
	}

	g.Expect(p.Restore(context.Background(), restoreVM(t, id), src, ports.RestoreInput{})).To(g.Succeed())
	g.Expect(dirMode).To(g.Equal(os.FileMode(0o700)))
	g.Expect(configMode).To(g.Equal(os.FileMode(0o600)))
}

func TestRestore_CleansUpOnFailure(t *testing.T) {
	cases := []struct {
		name     string
		failPath string
		noAPI    bool
	}{
		{name: "resume fails", failPath: "/api/v1/vm.resume"},
		{name: "api socket never answers", noAPI: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			g.RegisterTestingT(t)

			p, id, vmState := newTestProvider(t)
			p.diskSvc = fakeDiskService{}
			p.apiReadyTimeout = 200 * time.Millisecond
			src := t.TempDir()
			writeSnapshot(t, src)

			var api *fakeAPI
			if !tc.noAPI {
				api = &fakeAPI{failPath: tc.failPath}
			}

			started := &startedVMM{}
			p.startVMM = fakeStart(t, vmState, api, started)

			err := p.Restore(context.Background(), restoreVM(t, id), src, ports.RestoreInput{})
			g.Expect(err).To(g.HaveOccurred())

			g.Expect(started.pid).NotTo(g.BeZero())
			g.Expect(processGone(started.pid)).To(g.BeTrue())

			for _, path := range []string{vmState.PIDPath(), vmState.SockPath(), vmState.RestoreDir()} {
				_, statErr := os.Stat(path)
				g.Expect(errors.Is(statErr, os.ErrNotExist)).To(g.BeTrue(), path)
			}
		})
	}
}

func TestRestore_FailsBeforeStart(t *testing.T) {
	cases := []struct {
		name     string
		src      string
		in       ports.RestoreInput
		mutate   func(vm *models.MicroVM)
		noConfig bool
		matches  func(err error) bool
	}{
		{
			name:    "file memory mode",
			in:      ports.RestoreInput{MemoryMode: models.MemoryModeFile},
			matches: cerrs.IsNotSupported,
		},
		{
			name:    "relative source",
			src:     "snapshots/x",
			matches: func(err error) bool { return errors.Is(err, errSnapshotPathNotAbsolute) },
		},
		{
			name: "macvtap interface",
			mutate: func(vm *models.MicroVM) {
				vm.Spec.NetworkInterfaces[1].Type = models.IfaceTypeMacvtap
			},
			matches: cerrs.IsNotSupported,
		},
		{
			name:   "network interface status missing",
			mutate: func(vm *models.MicroVM) { delete(vm.Status.NetworkInterfaces, "eth1") },
			matches: func(err error) bool {
				var missing cerrs.NetworkInterfaceStatusMissingError

				return errors.As(err, &missing)
			},
		},
		{
			name:   "volume not mounted",
			mutate: func(vm *models.MicroVM) { delete(vm.Status.Volumes, "data") },
			matches: func(err error) bool {
				return strings.Contains(err.Error(), "data")
			},
		},
		{
			name: "devices differ from the snapshot",
			mutate: func(vm *models.MicroVM) {
				vm.Spec.NetworkInterfaces = vm.Spec.NetworkInterfaces[:1]
			},
			matches: func(err error) bool { return errors.Is(err, errSnapshotDeviceMismatch) },
		},
		{
			name:     "snapshot has no config",
			noConfig: true,
			matches:  func(err error) bool { return errors.Is(err, os.ErrNotExist) },
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			g.RegisterTestingT(t)

			p, id, vmState := newTestProvider(t)
			p.diskSvc = fakeDiskService{}
			vm := restoreVM(t, id)

			if tc.mutate != nil {
				tc.mutate(vm)
			}

			src := tc.src
			if src == "" {
				src = t.TempDir()
				writeSnapshot(t, src)
			}

			if tc.noConfig {
				g.Expect(os.Remove(filepath.Join(src, "vmstate", "config.json"))).To(g.Succeed())
			}

			p.startVMM = func(*exec.Cmd, bool) error {
				t.Fatal("cloud-hypervisor must not start")

				return nil
			}

			err := p.Restore(context.Background(), vm, src, tc.in)
			g.Expect(err).To(g.HaveOccurred())
			g.Expect(tc.matches(err)).To(g.BeTrue(), err.Error())

			_, statErr := os.Stat(vmState.RestoreDir())
			g.Expect(errors.Is(statErr, os.ErrNotExist)).To(g.BeTrue())
		})
	}
}
