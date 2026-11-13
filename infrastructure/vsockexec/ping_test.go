package vsockexec_test

import (
	"context"
	"encoding/json"
	"net"
	"path/filepath"
	"testing"
	"time"

	"github.com/liquidmetal-dev/guest-agent/pkg/vsockclient"

	"github.com/liquidmetal-dev/flintlock/infrastructure/vsockexec"
	"github.com/liquidmetal-dev/flintlock/pkg/defaults"
)

func TestPing_Success(t *testing.T) {
	gotOp := make(chan vsockclient.Op, 1)

	udsPath := fakeGuestAgent(t, defaults.GuestAgentControlPort, func(conn net.Conn) {
		defer conn.Close()

		f, err := vsockclient.ReadFrame(conn)
		if err != nil || f.Type != vsockclient.FrameRequest {
			return
		}

		var req vsockclient.Request
		if err := json.Unmarshal(f.Payload, &req); err != nil {
			return
		}
		gotOp <- req.Op

		vsockclient.WriteExit(conn, 0)
	})

	if err := vsockexec.NewPinger().Ping(context.Background(), udsPath); err != nil {
		t.Fatalf("Ping: %v", err)
	}

	if op := <-gotOp; op != vsockclient.OpPing {
		t.Fatalf("op = %q, want %q", op, vsockclient.OpPing)
	}
}

func TestPing_NonZeroExit(t *testing.T) {
	udsPath := fakeGuestAgent(t, defaults.GuestAgentControlPort, func(conn net.Conn) {
		defer conn.Close()

		if _, err := vsockclient.ReadFrame(conn); err != nil {
			return
		}
		vsockclient.WriteExit(conn, 1)
	})

	if err := vsockexec.NewPinger().Ping(context.Background(), udsPath); err == nil {
		t.Fatal("Ping: want an error for a non-zero exit")
	}
}

func TestPing_ErrorFrame(t *testing.T) {
	udsPath := fakeGuestAgent(t, defaults.GuestAgentControlPort, func(conn net.Conn) {
		defer conn.Close()

		if _, err := vsockclient.ReadFrame(conn); err != nil {
			return
		}
		vsockclient.WriteError(conn, "unknown op")
		vsockclient.WriteExit(conn, 0)
	})

	if err := vsockexec.NewPinger().Ping(context.Background(), udsPath); err == nil {
		t.Fatal("Ping: want an error for an error frame")
	}
}

func TestPing_NoListener(t *testing.T) {
	udsPath := filepath.Join(t.TempDir(), "missing.vsock")

	if err := vsockexec.NewPinger().Ping(context.Background(), udsPath); err == nil {
		t.Fatal("Ping: want an error when nothing listens")
	}
}

func TestPing_CancelDuringRead(t *testing.T) {
	release := make(chan struct{})
	t.Cleanup(func() { close(release) })

	udsPath := fakeGuestAgent(t, defaults.GuestAgentControlPort, func(conn net.Conn) {
		defer conn.Close()

		_, _ = vsockclient.ReadFrame(conn)
		<-release
	})

	assertCancelReturnsPromptly(t, udsPath)
}

func TestPing_CancelDuringHandshake(t *testing.T) {
	udsPath := filepath.Join(t.TempDir(), "hang.vsock")

	l, err := net.Listen("unix", udsPath)
	if err != nil {
		t.Fatalf("listen unix: %v", err)
	}
	t.Cleanup(func() { l.Close() })

	release := make(chan struct{})
	t.Cleanup(func() { close(release) })

	go func() {
		conn, err := l.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		<-release
	}()

	assertCancelReturnsPromptly(t, udsPath)
}

func assertCancelReturnsPromptly(t *testing.T, udsPath string) {
	t.Helper()

	ctx, cancel := context.WithCancel(context.Background())
	time.AfterFunc(20*time.Millisecond, cancel)

	start := time.Now()
	err := vsockexec.NewPinger().Ping(ctx, udsPath)

	if err == nil {
		t.Fatal("Ping: want an error after cancel")
	}
	if ctx.Err() == nil {
		t.Fatalf("Ping returned before cancel: %v", err)
	}
	if elapsed := time.Since(start); elapsed > 200*time.Millisecond {
		t.Fatalf("Ping took %s after cancel, want under 200ms", elapsed)
	}
}
