package vsocksshproxy_test

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"path/filepath"
	"strings"
	"testing"

	"github.com/liquidmetal-dev/flintlock/infrastructure/vsocksshproxy"
)

// fakeGuestAgentSSH listens on a UDS, performs the CONNECT handshake once,
// then echoes back whatever it receives - standing in for guest-agent's raw
// ssh-proxy port for the purposes of this test.
func fakeGuestAgentSSH(t *testing.T, port uint32) string {
	t.Helper()

	dir := t.TempDir()
	udsPath := filepath.Join(dir, "vm.vsock")

	l, err := net.Listen("unix", udsPath)
	if err != nil {
		t.Fatalf("listen unix: %v", err)
	}
	t.Cleanup(func() { l.Close() })

	go func() {
		conn, err := l.Accept()
		if err != nil {
			return
		}
		defer conn.Close()

		r := bufio.NewReader(conn)
		line, err := r.ReadString('\n')
		if err != nil {
			return
		}
		if strings.TrimSpace(line) != fmt.Sprintf("CONNECT %d", port) {
			fmt.Fprintf(conn, "ERR unexpected %q\n", line)

			return
		}
		fmt.Fprintf(conn, "OK 0\n")

		io.Copy(conn, r) // echo anything sent after the handshake
	}()

	return udsPath
}

func TestSession_WriteReadRoundtrip(t *testing.T) {
	const port = 1025

	udsPath := fakeGuestAgentSSH(t, port)

	session, err := vsocksshproxy.Start(context.Background(), udsPath, port)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer session.Close()

	want := "ssh-client-hello"
	if _, err := session.Write([]byte(want)); err != nil {
		t.Fatalf("Write: %v", err)
	}

	buf := make([]byte, len(want))
	total := 0
	for total < len(buf) {
		n, err := session.Read(buf[total:])
		total += n
		if err != nil {
			t.Fatalf("Read: %v", err)
		}
	}

	if string(buf) != want {
		t.Fatalf("roundtrip = %q, want %q", buf, want)
	}
}
