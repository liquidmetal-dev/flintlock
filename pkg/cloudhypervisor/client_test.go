package cloudhypervisor_test

import (
	"context"
	"io"
	"net"
	"net/http"
	"path/filepath"
	"testing"
	"time"

	. "github.com/onsi/gomega"

	"github.com/liquidmetal-dev/flintlock/pkg/cloudhypervisor"
)

// TestClient_Snapshot_SendsDestinationURL guards against a regression where the
// snapshot request was sent with an empty body. Cloud-hypervisor requires the
// destination_url to be present in the body.
func TestClient_Snapshot_SendsDestinationURL(t *testing.T) {
	RegisterTestingT(t)

	var (
		capturedMethod string
		capturedBody   string
	)

	client, closeServer := snapshotTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		capturedMethod = r.Method
		body, _ := io.ReadAll(r.Body)
		capturedBody = string(body)
		w.WriteHeader(http.StatusNoContent)
	})
	defer closeServer()

	destURL := "file:///var/lib/flintlock/snapshots/vm1"
	err := client.Snapshot(context.Background(), &cloudhypervisor.VMSnapshotConfig{DestinationURL: &destURL})

	Expect(err).NotTo(HaveOccurred())
	Expect(capturedMethod).To(Equal(http.MethodPut))
	Expect(capturedBody).To(ContainSubstring("destination_url"))
	Expect(capturedBody).To(ContainSubstring(destURL))
}

func TestClient_Snapshot_RejectsInvalidConfigWithoutRequest(t *testing.T) {
	testCases := []struct {
		name   string
		config *cloudhypervisor.VMSnapshotConfig
	}{
		{
			name:   "nil config",
			config: nil,
		},
		{
			name:   "nil destination url",
			config: &cloudhypervisor.VMSnapshotConfig{},
		},
		{
			name: "empty destination url",
			config: &cloudhypervisor.VMSnapshotConfig{
				DestinationURL: strPtr(""),
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			RegisterTestingT(t)

			requests := 0
			client, closeServer := snapshotTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				requests++
				w.WriteHeader(http.StatusNoContent)
			})
			defer closeServer()

			err := client.Snapshot(context.Background(), tc.config)

			Expect(err).To(HaveOccurred())
			Expect(requests).To(Equal(0))
		})
	}
}

func snapshotTestClient(t *testing.T, handler http.HandlerFunc) (cloudhypervisor.Client, func()) {
	t.Helper()

	sockPath := filepath.Join(t.TempDir(), "ch.sock")

	listener, err := net.Listen("unix", sockPath)
	Expect(err).NotTo(HaveOccurred())

	srv := &http.Server{
		ReadHeaderTimeout: time.Second,
		Handler:           handler,
	}

	go func() { _ = srv.Serve(listener) }()

	return cloudhypervisor.New(sockPath), func() {
		_ = srv.Close()
	}
}

func strPtr(value string) *string {
	return &value
}
