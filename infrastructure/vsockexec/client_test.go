package vsockexec_test

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"path/filepath"
	"strings"
	"testing"

	"github.com/liquidmetal-dev/flintlock/infrastructure/vsockexec"
	"github.com/liquidmetal-dev/guest-agent/pkg/vsockclient"
)

// fakeGuestAgent listens on a UDS, performs the Firecracker/Cloud Hypervisor
// CONNECT handshake exactly once, then hands the connection to handle so
// tests can script whatever control-channel exchange they need.
func fakeGuestAgent(t *testing.T, port uint32, handle func(net.Conn)) string {
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

		r := bufio.NewReader(conn)
		line, err := r.ReadString('\n')
		if err != nil {
			conn.Close()

			return
		}
		if strings.TrimSpace(line) != fmt.Sprintf("CONNECT %d", port) {
			fmt.Fprintf(conn, "ERR unexpected %q\n", line)
			conn.Close()

			return
		}
		fmt.Fprintf(conn, "OK 0\n")

		handle(conn)
	}()

	return udsPath
}

func TestSession_ExecStdoutStderrExit(t *testing.T) {
	const port = 1024

	udsPath := fakeGuestAgent(t, port, func(conn net.Conn) {
		defer conn.Close()

		f, err := vsockclient.ReadFrame(conn)
		if err != nil || f.Type != vsockclient.FrameRequest {
			return
		}
		var req vsockclient.Request
		if err := json.Unmarshal(f.Payload, &req); err != nil || req.Op != vsockclient.OpExec {
			return
		}

		vsockclient.WriteFrame(conn, vsockclient.FrameStdout, []byte("hello stdout"))
		vsockclient.WriteFrame(conn, vsockclient.FrameStderr, []byte("hello stderr"))
		vsockclient.WriteExit(conn, 7)
	})

	session, err := vsockexec.Start(context.Background(), udsPath, port, &vsockclient.Exec{Cmd: "uname"})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer session.Close()

	var gotStdout, gotStderr []byte
	var gotExit int
	var sawExit bool

	for i := 0; i < 10 && !sawExit; i++ {
		event, err := session.Next()
		if err != nil {
			t.Fatalf("Next: %v", err)
		}

		switch event.Type {
		case vsockexec.EventStdout:
			gotStdout = append(gotStdout, event.Data...)
		case vsockexec.EventStderr:
			gotStderr = append(gotStderr, event.Data...)
		case vsockexec.EventExit:
			gotExit = event.ExitCode
			sawExit = true
		}
	}

	if !sawExit {
		t.Fatal("never saw an exit event")
	}
	if string(gotStdout) != "hello stdout" {
		t.Errorf("stdout = %q, want %q", gotStdout, "hello stdout")
	}
	if string(gotStderr) != "hello stderr" {
		t.Errorf("stderr = %q, want %q", gotStderr, "hello stderr")
	}
	if gotExit != 7 {
		t.Errorf("exit code = %d, want 7", gotExit)
	}
}

func TestSession_SendStdinAndCloseStdin(t *testing.T) {
	const port = 1024

	received := make(chan string, 1)
	sawEOF := make(chan struct{}, 1)

	udsPath := fakeGuestAgent(t, port, func(conn net.Conn) {
		defer conn.Close()

		// consume the exec request frame
		if _, err := vsockclient.ReadFrame(conn); err != nil {
			return
		}

		for {
			f, err := vsockclient.ReadFrame(conn)
			if err != nil {
				return
			}
			switch f.Type {
			case vsockclient.FrameStdin:
				received <- string(f.Payload)
			case vsockclient.FrameStdinEOF:
				sawEOF <- struct{}{}
				vsockclient.WriteExit(conn, 0)

				return
			}
		}
	})

	session, err := vsockexec.Start(context.Background(), udsPath, port, &vsockclient.Exec{Cmd: "cat", HasStdin: true})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer session.Close()

	if err := session.SendStdin([]byte("payload")); err != nil {
		t.Fatalf("SendStdin: %v", err)
	}
	if got := <-received; got != "payload" {
		t.Fatalf("agent received %q, want %q", got, "payload")
	}

	if err := session.CloseStdin(); err != nil {
		t.Fatalf("CloseStdin: %v", err)
	}
	<-sawEOF

	event, err := session.Next()
	if err != nil {
		t.Fatalf("Next: %v", err)
	}
	if event.Type != vsockexec.EventExit || event.ExitCode != 0 {
		t.Fatalf("unexpected final event: %+v", event)
	}
}
