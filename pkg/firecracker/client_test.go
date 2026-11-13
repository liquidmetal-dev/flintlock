package firecracker_test

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	. "github.com/onsi/gomega"

	"github.com/liquidmetal-dev/flintlock/pkg/firecracker"
)

type request struct {
	method string
	path   string
	body   string
}

// newUnixServer starts an HTTP server on a unix socket and returns the socket path. Every
// request it gets is sent on the returned channel.
func newUnixServer(t *testing.T, status int, respBody string) (string, <-chan request) {
	t.Helper()

	// A short directory keeps the socket path inside the unix socket path limit.
	dir, err := os.MkdirTemp("", "fc")
	if err != nil {
		t.Fatalf("creating temp dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })

	sockPath := filepath.Join(dir, "firecracker.sock")

	listener, err := net.Listen("unix", sockPath)
	if err != nil {
		t.Fatalf("listening on %s: %v", sockPath, err)
	}

	requests := make(chan request, 1)

	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		requests <- request{method: r.Method, path: r.URL.Path, body: string(body)}

		if respBody != "" {
			w.Header().Set("Content-Type", "application/json")
		}

		w.WriteHeader(status)
		_, _ = w.Write([]byte(respBody))
	}))
	srv.Listener = listener
	srv.Start()
	t.Cleanup(srv.Close)

	return sockPath, requests
}

func TestClientSetsVMState(t *testing.T) {
	cases := []struct {
		name string
		call func(ctx context.Context, c firecracker.Client) error
		body string
	}{
		{
			name: "Pause",
			call: func(ctx context.Context, c firecracker.Client) error { return c.Pause(ctx) },
			body: `{"state":"Paused"}`,
		},
		{
			name: "Resume",
			call: func(ctx context.Context, c firecracker.Client) error { return c.Resume(ctx) },
			body: `{"state":"Resumed"}`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)

			sockPath, requests := newUnixServer(t, http.StatusNoContent, "")

			err := tc.call(context.Background(), firecracker.New(sockPath))
			g.Expect(err).NotTo(HaveOccurred())

			got := <-requests
			g.Expect(got.method).To(Equal(http.MethodPatch))
			g.Expect(got.path).To(Equal("/vm"))
			g.Expect(got.body).To(MatchJSON(tc.body))
		})
	}
}

func TestClientDescribe(t *testing.T) {
	g := NewWithT(t)

	sockPath, requests := newUnixServer(t, http.StatusOK,
		`{"app_name":"Firecracker","id":"uid","state":"Paused","vmm_version":"1.12.0"}`)

	info, err := firecracker.New(sockPath).Describe(context.Background())
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(info).To(Equal(&firecracker.InstanceInfo{
		AppName:    "Firecracker",
		ID:         "uid",
		State:      firecracker.InstanceStatePaused,
		VMMVersion: "1.12.0",
	}))

	got := <-requests
	g.Expect(got.method).To(Equal(http.MethodGet))
	g.Expect(got.path).To(Equal("/"))
	g.Expect(got.body).To(BeEmpty())
}

func TestClientReturnsFaultMessage(t *testing.T) {
	g := NewWithT(t)

	sockPath, _ := newUnixServer(t, http.StatusBadRequest, `{"fault_message":"the microvm is not running"}`)

	err := firecracker.New(sockPath).Pause(context.Background())
	g.Expect(err).To(MatchError(ContainSubstring("the microvm is not running")))
}

func TestClientSocketNotAnswering(t *testing.T) {
	g := NewWithT(t)

	_, err := firecracker.New(filepath.Join(t.TempDir(), "missing.sock")).Describe(context.Background())
	g.Expect(err).To(HaveOccurred())
}

func TestClientDescribeTimesOutWhenSocketHangs(t *testing.T) {
	g := NewWithT(t)

	dir, err := os.MkdirTemp("", "fc")
	g.Expect(err).NotTo(HaveOccurred())
	t.Cleanup(func() { _ = os.RemoveAll(dir) })

	sockPath := filepath.Join(dir, "firecracker.sock")

	listener, err := net.Listen("unix", sockPath)
	g.Expect(err).NotTo(HaveOccurred())
	t.Cleanup(func() { _ = listener.Close() })

	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			t.Cleanup(func() { _ = conn.Close() })
		}
	}()

	start := time.Now()
	_, err = firecracker.New(sockPath, firecracker.WithDescribeTimeout(100*time.Millisecond)).
		Describe(context.Background())
	g.Expect(err).To(MatchError(context.DeadlineExceeded))
	g.Expect(time.Since(start)).To(BeNumerically("<", 5*time.Second))
}

func TestClientPutRequests(t *testing.T) {
	cases := []struct {
		name string
		call func(ctx context.Context, c firecracker.Client) error
		path string
		body string
	}{
		{
			name: "CreateSnapshot",
			call: func(ctx context.Context, c firecracker.Client) error {
				return c.CreateSnapshot(ctx, firecracker.SnapshotCreateParams{
					SnapshotType: firecracker.SnapshotTypeFull,
					SnapshotPath: "/snap/vmstate",
					MemFilePath:  "/snap/memory",
				})
			},
			path: "/snapshot/create",
			body: `{"snapshot_type":"Full","snapshot_path":"/snap/vmstate","mem_file_path":"/snap/memory"}`,
		},
		{
			name: "LoadSnapshot",
			call: func(ctx context.Context, c firecracker.Client) error {
				return c.LoadSnapshot(ctx, firecracker.SnapshotLoadParams{
					SnapshotPath: "/snap/vmstate",
					MemBackend: firecracker.MemoryBackend{
						BackendType: firecracker.MemoryBackendTypeFile,
						BackendPath: "/snap/memory",
					},
					NetworkOverrides: []firecracker.NetworkOverride{
						{IfaceID: "eth0", HostDevName: "fltap123"},
					},
				})
			},
			path: "/snapshot/load",
			body: `{
				"snapshot_path":"/snap/vmstate",
				"mem_backend":{"backend_type":"File","backend_path":"/snap/memory"},
				"network_overrides":[{"iface_id":"eth0","host_dev_name":"fltap123"}],
				"resume_vm":false
			}`,
		},
		{
			name: "LoadSnapshot without network overrides",
			call: func(ctx context.Context, c firecracker.Client) error {
				return c.LoadSnapshot(ctx, firecracker.SnapshotLoadParams{
					SnapshotPath: "/snap/vmstate",
					MemBackend: firecracker.MemoryBackend{
						BackendType: firecracker.MemoryBackendTypeFile,
						BackendPath: "/snap/memory",
					},
				})
			},
			path: "/snapshot/load",
			body: `{
				"snapshot_path":"/snap/vmstate",
				"mem_backend":{"backend_type":"File","backend_path":"/snap/memory"},
				"resume_vm":false
			}`,
		},
		{
			name: "PutMMDS",
			call: func(ctx context.Context, c firecracker.Client) error {
				return c.PutMMDS(ctx, map[string]any{"latest": map[string]string{"user-data": "hello"}})
			},
			path: "/mmds",
			body: `{"latest":{"user-data":"hello"}}`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)

			sockPath, requests := newUnixServer(t, http.StatusNoContent, "")

			err := tc.call(context.Background(), firecracker.New(sockPath))
			g.Expect(err).NotTo(HaveOccurred())

			got := <-requests
			g.Expect(got.method).To(Equal(http.MethodPut))
			g.Expect(got.path).To(Equal(tc.path))
			g.Expect(got.body).To(MatchJSON(tc.body))
		})
	}
}
