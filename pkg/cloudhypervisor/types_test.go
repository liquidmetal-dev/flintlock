package cloudhypervisor_test

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"path/filepath"
	"testing"

	. "github.com/onsi/gomega"

	"github.com/liquidmetal-dev/flintlock/pkg/cloudhypervisor"
)

// The vm.info responses below are written from the OpenAPI schema of the
// named Cloud Hypervisor version. They are not captured from a running VMM.
const (
	// In v48.0 the serial device and the console have the same schema.
	vmInfoV48 = `{
  "config": {
    "payload": {"kernel": "/var/lib/flintlock/vmlinux", "cmdline": "console=hvc0"},
    "serial": {"file": "/var/lib/flintlock/serial.log", "mode": "File", "iommu": false},
    "console": {"mode": "Pty", "iommu": false}
  },
  "state": "Running",
  "memory_actual_size": 1073741824
}`

	// In v53.0 the serial device has its own schema without iommu, and the
	// console has fields which the types do not have.
	vmInfoV53 = `{
  "config": {
    "payload": {"kernel": "/var/lib/flintlock/vmlinux", "cmdline": "console=hvc0"},
    "serial": {"socket": "/var/lib/flintlock/serial.sock", "mode": "Socket"},
    "console": {
      "mode": "Pty",
      "iommu": false,
      "id": "_console",
      "pci_segment": 0,
      "pci_device_id": 3
    }
  },
  "state": "Running",
  "memory_actual_size": 1073741824
}`
)

func TestClientInfo(t *testing.T) {
	testCases := []struct {
		name           string
		response       string
		expectedSerial cloudhypervisor.SerialConfig
	}{
		{
			name:     "v48.0 serial device with the console schema",
			response: vmInfoV48,
			expectedSerial: cloudhypervisor.SerialConfig{
				File:  ptr("/var/lib/flintlock/serial.log"),
				Mode:  cloudhypervisor.ConsoleModeFile,
				Iommu: ptr(false),
			},
		},
		{
			name:     "v53.0 serial device with its own schema",
			response: vmInfoV53,
			expectedSerial: cloudhypervisor.SerialConfig{
				Socket: ptr("/var/lib/flintlock/serial.sock"),
				Mode:   cloudhypervisor.ConsoleModeSocket,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)

			socketPath := serveVMInfo(t, tc.response)

			info, err := cloudhypervisor.New(socketPath).Info(context.Background())
			g.Expect(err).NotTo(HaveOccurred())

			g.Expect(info.State).To(Equal(cloudhypervisor.VMStateRunning))
			g.Expect(info.MemoryActualSize).To(HaveValue(Equal(int64(1073741824))))
			g.Expect(info.Config.Payload.Kernel).To(Equal("/var/lib/flintlock/vmlinux"))

			g.Expect(info.Config.Serial).NotTo(BeNil())
			g.Expect(*info.Config.Serial).To(Equal(tc.expectedSerial))

			g.Expect(info.Config.Console).NotTo(BeNil())
			g.Expect(*info.Config.Console).To(Equal(cloudhypervisor.ConsoleConfig{
				Mode:  cloudhypervisor.ConsoleModePty,
				Iommu: ptr(false),
			}))
		})
	}
}

func TestSerialConfigMarshalSocket(t *testing.T) {
	g := NewWithT(t)

	data, err := json.Marshal(cloudhypervisor.SerialConfig{
		Mode:   cloudhypervisor.ConsoleModeSocket,
		Socket: ptr("/tmp/serial.sock"),
	})
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(data).To(MatchJSON(`{"mode": "Socket", "socket": "/tmp/serial.sock"}`))
}

func TestConsoleConfigMarshalSocket(t *testing.T) {
	g := NewWithT(t)

	data, err := json.Marshal(cloudhypervisor.ConsoleConfig{
		Mode:   cloudhypervisor.ConsoleModeSocket,
		Socket: ptr("/tmp/console.sock"),
	})
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(data).To(MatchJSON(`{"mode": "Socket", "socket": "/tmp/console.sock"}`))
}

// serveVMInfo serves the response as the vm.info endpoint on a unix socket,
// and returns the path of the socket.
func serveVMInfo(t *testing.T, response string) string {
	t.Helper()

	socketPath := filepath.Join(t.TempDir(), "ch.sock")

	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Fatalf("listening on %s: %s", socketPath, err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/"+cloudhypervisor.PathVMInfo, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(response))
	})

	server := &http.Server{Handler: mux}
	go server.Serve(listener)
	t.Cleanup(func() { server.Close() })

	return socketPath
}

func ptr[T any](v T) *T {
	return &v
}
