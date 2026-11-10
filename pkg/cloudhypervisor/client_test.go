package cloudhypervisor

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/carlmjohnson/requests"
	. "github.com/onsi/gomega"
)

// TestRequestBodyIsConfig checks that methods which take a config send it as
// the JSON body of their PUT request, instead of an empty body.
func TestRequestBodyIsConfig(t *testing.T) {
	dest := "file:///tmp/snapshot"
	src := "file:///tmp/snapshot"
	fs := "fs0"
	disk := "/var/lib/flintlock/disk.img"
	sock := "/tmp/vsock.sock"
	tap := "tap0"
	vdpa := "/dev/vhost-vdpa-0"
	vcpus := int32(2)

	cases := []struct {
		name string
		path string
		call func(c *client) (any, error)
		body any
	}{
		{
			name: "Snapshot",
			path: PathSnapshotCreate,
			call: func(c *client) (any, error) {
				return nil, c.Snapshot(context.Background(), &VMSnapshotConfig{DestinationURL: &dest})
			},
			body: &VMSnapshotConfig{DestinationURL: &dest},
		},
		{
			name: "Restore",
			path: PathSnapshotRestore,
			call: func(c *client) (any, error) {
				return nil, c.Restore(context.Background(), &RestoreConfig{SourceURL: src})
			},
			body: &RestoreConfig{SourceURL: src},
		},
		{
			name: "AddFs",
			path: PathVMAddFs,
			call: func(c *client) (any, error) {
				return c.AddFs(context.Background(), &FsConfig{Tag: fs})
			},
			body: &FsConfig{Tag: fs},
		},
		{
			name: "Resize",
			path: PathVMResize,
			call: func(c *client) (any, error) {
				return nil, c.Resize(context.Background(), &VMResize{DesiredVcpus: &vcpus})
			},
			body: &VMResize{DesiredVcpus: &vcpus},
		},
		{
			name: "AddDisk",
			path: PathVMAddDisk,
			call: func(c *client) (any, error) {
				return c.AddDisk(context.Background(), &DiskConfig{Path: disk})
			},
			body: &DiskConfig{Path: disk},
		},
		{
			name: "AddVsockDevice",
			path: PathAddVsockDevice,
			call: func(c *client) (any, error) {
				return c.AddVsockDevice(context.Background(), &VsockConfig{Cid: 3, Socket: sock})
			},
			body: &VsockConfig{Cid: 3, Socket: sock},
		},
		{
			name: "AddNetworkDevice",
			path: PathAddNetworkDevice,
			call: func(c *client) (any, error) {
				return c.AddNetworkDevice(context.Background(), &NetConfig{Tap: &tap})
			},
			body: &NetConfig{Tap: &tap},
		},
		{
			name: "AddVdpaDevice",
			path: PathAddVdpaDevice,
			call: func(c *client) (any, error) {
				return c.AddVdpaDevice(context.Background(), &VdpaConfig{Path: vdpa, NumQueues: 2})
			},
			body: &VdpaConfig{Path: vdpa, NumQueues: 2},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)

			var gotMethod, gotPath string
			var gotBody []byte
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotMethod = r.Method
				gotPath = r.URL.Path
				gotBody, _ = io.ReadAll(r.Body)
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{}`))
			}))
			defer srv.Close()

			c := &client{builder: requests.URL(srv.URL)}
			_, err := tc.call(c)
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(gotMethod).To(Equal(http.MethodPut))
			g.Expect(gotPath).To(HaveSuffix(tc.path))

			want, err := json.Marshal(tc.body)
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(gotBody).To(MatchJSON(want), "request body was %q", string(gotBody))
		})
	}
}

// TestNilConfigSendsEmptyBody checks that passing a nil config keeps the
// previous behaviour of sending a PUT with no body.
func TestNilConfigSendsEmptyBody(t *testing.T) {
	g := NewWithT(t)

	var gotBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	c := &client{builder: requests.URL(srv.URL)}
	g.Expect(c.Snapshot(context.Background(), nil)).To(Succeed())
	g.Expect(gotBody).To(BeEmpty(), "request body was %q", string(gotBody))
}
