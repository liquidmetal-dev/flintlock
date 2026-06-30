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

	sockPath := filepath.Join(t.TempDir(), "ch.sock")

	listener, err := net.Listen("unix", sockPath)
	Expect(err).NotTo(HaveOccurred())

	var (
		capturedMethod string
		capturedBody   string
	)

	srv := &http.Server{
		ReadHeaderTimeout: time.Second,
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			capturedMethod = r.Method
			body, _ := io.ReadAll(r.Body)
			capturedBody = string(body)
			w.WriteHeader(http.StatusNoContent)
		}),
	}

	go func() { _ = srv.Serve(listener) }()
	defer srv.Close()

	client := cloudhypervisor.New(sockPath)

	destURL := "file:///var/lib/flintlock/snapshots/vm1"
	err = client.Snapshot(context.Background(), &cloudhypervisor.VMSnapshotConfig{DestinationURL: &destURL})

	Expect(err).NotTo(HaveOccurred())
	Expect(capturedMethod).To(Equal(http.MethodPut))
	Expect(capturedBody).To(ContainSubstring("destination_url"))
	Expect(capturedBody).To(ContainSubstring(destURL))
}
