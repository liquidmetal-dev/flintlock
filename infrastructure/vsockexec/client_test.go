package vsockexec_test

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/liquidmetal-dev/guest-agent/pkg/vsockclient"

	"github.com/liquidmetal-dev/flintlock/infrastructure/vsockexec"
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

// TestSession_Next_TimesOutWhenGuestAgentGoesQuiet reproduces the hang from
// https://github.com/liquidmetal-dev/flintlock/issues/1200: the guest-agent
// responds to the exec request and then never sends another frame (no
// heartbeat, no exit frame) and never closes the connection (as observed
// with some vsock-over-UDS transports after a guest-side RST that never
// reaches the host as EOF). Next() must return an error within the idle
// deadline instead of blocking forever, regardless of the request's
// TimeoutSec.
func TestSession_Next_TimesOutWhenGuestAgentGoesQuiet(t *testing.T) {
	const port = 1024

	stuck := make(chan struct{})

	udsPath := fakeGuestAgent(t, port, func(conn net.Conn) {
		if _, err := vsockclient.ReadFrame(conn); err != nil {
			return
		}

		vsockclient.WriteFrame(conn, vsockclient.FrameStdout, []byte("hello stdout"))

		// Simulate a guest-agent that has gone quiet without closing the
		// connection, sending a heartbeat, or sending an exit frame.
		<-stuck
	})
	t.Cleanup(func() { close(stuck) })

	session, err := vsockexec.Start(context.Background(), udsPath, port, &vsockclient.Exec{Cmd: "uname", TimeoutSec: 1})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer session.Close()

	// Override the real TimeoutSec-derived deadline (1s + 30s grace) so the
	// test doesn't have to wait out the full period.
	session.SetIdleTimeout(100 * time.Millisecond)

	event, err := session.Next()
	if err != nil {
		t.Fatalf("Next: unexpected error on first frame: %v", err)
	}
	if event.Type != vsockexec.EventStdout {
		t.Fatalf("unexpected first event: %+v", event)
	}

	done := make(chan struct{})
	var nextErr error

	go func() {
		defer close(done)
		_, nextErr = session.Next()
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Next() blocked forever instead of timing out")
	}

	if nextErr == nil {
		t.Fatal("Next: expected a timeout error, got nil")
	}
}

// TestSession_Next_HeartbeatsKeepQuietSessionAlive guards against the idle
// deadline killing a healthy command that produces no stdout/stderr for a
// while (e.g. `sleep`) as long as the guest-agent's periodic FrameHeartbeat
// keeps arriving. Run with both a bounded and an unbounded TimeoutSec to
// confirm the deadline no longer depends on it.
func TestSession_Next_HeartbeatsKeepQuietSessionAlive(t *testing.T) {
	for _, timeoutSec := range []int{5, 0} {
		t.Run(fmt.Sprintf("TimeoutSec=%d", timeoutSec), func(t *testing.T) {
			const port = 1024

			udsPath := fakeGuestAgent(t, port, func(conn net.Conn) {
				defer conn.Close()

				if _, err := vsockclient.ReadFrame(conn); err != nil {
					return
				}

				// Quiet interval (no stdout/stderr) longer than the shrunk
				// idle timeout below, bridged entirely by heartbeats.
				for i := 0; i < 5; i++ {
					time.Sleep(30 * time.Millisecond)
					vsockclient.WriteFrame(conn, vsockclient.FrameHeartbeat, nil)
				}
				vsockclient.WriteExit(conn, 0)
			})

			session, err := vsockexec.Start(context.Background(), udsPath, port, &vsockclient.Exec{Cmd: "sleep", TimeoutSec: timeoutSec})
			if err != nil {
				t.Fatalf("Start: %v", err)
			}
			defer session.Close()

			session.SetIdleTimeout(50 * time.Millisecond)

			event, err := session.Next()
			if err != nil {
				t.Fatalf("Next: %v", err)
			}
			if event.Type != vsockexec.EventExit || event.ExitCode != 0 {
				t.Fatalf("unexpected event: %+v", event)
			}
		})
	}
}

// TestSession_Next_HeartbeatNotSurfacedAsEvent confirms FrameHeartbeat is
// consumed internally to re-arm the idle deadline and never returned as an
// Event of its own — callers only ever see stdout/stderr/exit/error events.
func TestSession_Next_HeartbeatNotSurfacedAsEvent(t *testing.T) {
	const port = 1024

	udsPath := fakeGuestAgent(t, port, func(conn net.Conn) {
		defer conn.Close()

		if _, err := vsockclient.ReadFrame(conn); err != nil {
			return
		}

		vsockclient.WriteFrame(conn, vsockclient.FrameHeartbeat, nil)
		vsockclient.WriteFrame(conn, vsockclient.FrameHeartbeat, nil)
		vsockclient.WriteExit(conn, 0)
	})

	session, err := vsockexec.Start(context.Background(), udsPath, port, &vsockclient.Exec{Cmd: "sleep"})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer session.Close()

	// The two heartbeats must be swallowed internally: the very next event
	// observed by the caller is the exit, not something derived from them.
	event, err := session.Next()
	if err != nil {
		t.Fatalf("Next: %v", err)
	}
	if event.Type != vsockexec.EventExit || event.ExitCode != 0 {
		t.Fatalf("unexpected event: %+v", event)
	}
}

// TestSession_Next_LegacyAgentWithoutHeartbeatsUsesTimeoutSecDeadline is a
// regression test for a guest-agent that predates heartbeat support
// (pre-v0.4.0): it never sends FrameHeartbeat, so the session must keep the
// legacy TimeoutSec-derived deadline for its entire life instead of
// switching to (or ever having been on) the much tighter
// defaults.ExecSessionIdleTimeout. Without this, bumping the host's
// guest-agent client library alone — without upgrading the agent binary
// already running inside existing VMs — turns an ordinary quiet command
// into a false timeout.
func TestSession_Next_LegacyAgentWithoutHeartbeatsUsesTimeoutSecDeadline(t *testing.T) {
	const port = 1024

	udsPath := fakeGuestAgent(t, port, func(conn net.Conn) {
		defer conn.Close()

		if _, err := vsockclient.ReadFrame(conn); err != nil {
			return
		}

		// Quiet interval that would blow the flat heartbeat deadline but
		// sits comfortably inside the TimeoutSec-derived one. No heartbeat
		// is ever sent, as a legacy guest-agent never would.
		time.Sleep(250 * time.Millisecond)
		vsockclient.WriteExit(conn, 0)
	})

	session, err := vsockexec.Start(context.Background(), udsPath, port, &vsockclient.Exec{Cmd: "sleep", TimeoutSec: 5})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer session.Close()

	// Shrink only the post-heartbeat deadline; if the session ever switched
	// to it despite no heartbeat arriving, the quiet interval above would
	// trip it and this test would fail.
	session.SetHeartbeatIdleTimeout(50 * time.Millisecond)

	event, err := session.Next()
	if err != nil {
		t.Fatalf("Next: %v", err)
	}
	if event.Type != vsockexec.EventExit || event.ExitCode != 0 {
		t.Fatalf("unexpected event: %+v", event)
	}
}

// TestSession_Next_HeartbeatSwitchesToShortDeadline proves the switch from
// the legacy TimeoutSec-derived deadline to the tight heartbeat deadline
// actually takes effect, rather than being latent: once a heartbeat has
// arrived, a subsequently wedged connection must time out on the short
// post-heartbeat deadline, not the much larger legacy one.
func TestSession_Next_HeartbeatSwitchesToShortDeadline(t *testing.T) {
	const port = 1024

	stuck := make(chan struct{})

	udsPath := fakeGuestAgent(t, port, func(conn net.Conn) {
		if _, err := vsockclient.ReadFrame(conn); err != nil {
			return
		}

		vsockclient.WriteFrame(conn, vsockclient.FrameHeartbeat, nil)

		// Simulate a guest-agent that has gone quiet after proving
		// heartbeat support, without closing the connection or sending an
		// exit frame.
		<-stuck
	})
	t.Cleanup(func() { close(stuck) })

	// A large legacy deadline that would never fire within this test's
	// timeout on its own, isolating the assertion to the post-heartbeat one.
	session, err := vsockexec.Start(context.Background(), udsPath, port, &vsockclient.Exec{Cmd: "uname", TimeoutSec: 3600})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer session.Close()

	session.SetHeartbeatIdleTimeout(100 * time.Millisecond)

	done := make(chan struct{})
	var nextErr error

	go func() {
		defer close(done)
		_, nextErr = session.Next()
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Next() blocked forever instead of timing out on the post-heartbeat deadline")
	}

	if nextErr == nil {
		t.Fatal("Next: expected a timeout error, got nil")
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

// TestSession_Start_RetriesTransientHandshakeFailure reproduces
// https://github.com/liquidmetal-dev/flintlock/issues/1205: a CONNECT
// handshake that occasionally gets EOF instead of the OK reply, well below
// the guest-agent protocol layer. Start() must retry the dial instead of
// failing the whole exec RPC on what's usually a one-off blip.
func TestSession_Start_RetriesTransientHandshakeFailure(t *testing.T) {
	const port = 1024
	const failuresBeforeSuccess = 2

	var attempts int32

	dir := t.TempDir()
	udsPath := filepath.Join(dir, "vm.vsock")

	l, err := net.Listen("unix", udsPath)
	if err != nil {
		t.Fatalf("listen unix: %v", err)
	}
	t.Cleanup(func() { l.Close() })

	go func() {
		for {
			conn, err := l.Accept()
			if err != nil {
				return
			}

			if atomic.AddInt32(&attempts, 1) <= failuresBeforeSuccess {
				// Simulate the reported failure: close before any handshake
				// reply, so the client's read gets EOF.
				conn.Close()

				continue
			}

			go func(conn net.Conn) {
				defer conn.Close()

				r := bufio.NewReader(conn)
				line, err := r.ReadString('\n')
				if err != nil || strings.TrimSpace(line) != fmt.Sprintf("CONNECT %d", port) {
					return
				}
				fmt.Fprintf(conn, "OK 0\n")

				if _, err := vsockclient.ReadFrame(conn); err != nil {
					return
				}
				vsockclient.WriteExit(conn, 0)
			}(conn)
		}
	}()

	restore := vsockexec.SetDialRetryParams(failuresBeforeSuccess+1, 10*time.Millisecond)
	defer restore()

	session, err := vsockexec.Start(context.Background(), udsPath, port, &vsockclient.Exec{Cmd: "true"})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer session.Close()

	event, err := session.Next()
	if err != nil {
		t.Fatalf("Next: %v", err)
	}
	if event.Type != vsockexec.EventExit || event.ExitCode != 0 {
		t.Fatalf("unexpected final event: %+v", event)
	}

	if got := atomic.LoadInt32(&attempts); got != failuresBeforeSuccess+1 {
		t.Fatalf("dial attempts = %d, want %d", got, failuresBeforeSuccess+1)
	}
}

// TestSession_Start_GivesUpAfterRetriesExhausted confirms Start() surfaces
// an error once the handshake keeps failing past the configured retry
// count, rather than retrying forever.
func TestSession_Start_GivesUpAfterRetriesExhausted(t *testing.T) {
	const port = 1024
	const retries = 2

	var attempts int32

	dir := t.TempDir()
	udsPath := filepath.Join(dir, "vm.vsock")

	l, err := net.Listen("unix", udsPath)
	if err != nil {
		t.Fatalf("listen unix: %v", err)
	}
	t.Cleanup(func() { l.Close() })

	go func() {
		for {
			conn, err := l.Accept()
			if err != nil {
				return
			}

			atomic.AddInt32(&attempts, 1)
			conn.Close()
		}
	}()

	restore := vsockexec.SetDialRetryParams(retries, 5*time.Millisecond)
	defer restore()

	if _, err := vsockexec.Start(context.Background(), udsPath, port, &vsockclient.Exec{Cmd: "true"}); err == nil {
		t.Fatal("Start: expected error, got nil")
	}

	if got, want := atomic.LoadInt32(&attempts), int32(retries+1); got != want {
		t.Fatalf("dial attempts = %d, want %d", got, want)
	}
}
