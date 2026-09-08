// Package vsockexec runs one exec request against a guest-agent's control
// channel, translating its framed protocol into a small typed event stream
// for the gRPC layer to relay.
package vsockexec

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/liquidmetal-dev/guest-agent/pkg/vsockclient"

	"github.com/liquidmetal-dev/flintlock/pkg/defaults"
)

// EventType identifies what a Session.Next event carries.
type EventType int

const (
	// EventStdout carries a chunk of the command's stdout.
	EventStdout EventType = iota
	// EventStderr carries a chunk of the command's stderr.
	EventStderr
	// EventExit carries the command's exit code and ends the exchange.
	EventExit
	// EventError carries an agent-side error message. The agent follows it
	// with an exit frame, so callers should keep reading after this event.
	EventError
)

// Event is one translated message from the guest-agent control channel.
type Event struct {
	Type     EventType
	Data     []byte
	ExitCode int
	Message  string
}

// Session is one exec request/response exchange with a guest-agent's control
// channel, reached by dialling a vsock UDS multiplexer path.
type Session struct {
	conn net.Conn
	// idleTimeout is the deadline Next() applies to each read. Zero means no
	// deadline: the request's TimeoutSec was unbounded (0), so there's no
	// budget to derive one from and imposing an arbitrary one would risk
	// cutting off a healthy, merely quiet command.
	idleTimeout time.Duration
}

// Start dials udsPath (a Firecracker/Cloud Hypervisor vsock UDS multiplexer)
// on the guest-agent's control port and sends an exec request. Callers must
// call Close when done.
func Start(ctx context.Context, udsPath string, controlPort uint32, exec *vsockclient.Exec) (*Session, error) {
	conn, err := vsockclient.Dial(ctx, udsPath, controlPort)
	if err != nil {
		return nil, fmt.Errorf("dialling guest-agent control channel: %w", err)
	}

	req := &vsockclient.Request{Version: vsockclient.Version, Op: vsockclient.OpExec, Exec: exec}
	if err := vsockclient.WriteRequest(conn, req); err != nil {
		conn.Close()

		return nil, fmt.Errorf("sending exec request: %w", err)
	}

	var idleTimeout time.Duration
	if exec.TimeoutSec > 0 {
		idleTimeout = time.Duration(exec.TimeoutSec)*time.Second + defaults.ExecSessionIdleGrace
	}

	return &Session{conn: conn, idleTimeout: idleTimeout}, nil
}

// SetIdleTimeout overrides the idle deadline Next() applies to each read.
// Mainly useful for tests that need to exercise the timeout path without
// waiting out the real one derived from the exec request's TimeoutSec.
func (s *Session) SetIdleTimeout(d time.Duration) {
	s.idleTimeout = d
}

// SendStdin forwards p to the running command's stdin.
func (s *Session) SendStdin(p []byte) error {
	if err := vsockclient.WriteFrame(s.conn, vsockclient.FrameStdin, p); err != nil {
		return fmt.Errorf("writing stdin frame: %w", err)
	}

	return nil
}

// CloseStdin signals end of stdin to the running command.
func (s *Session) CloseStdin() error {
	if err := vsockclient.WriteFrame(s.conn, vsockclient.FrameStdinEOF, nil); err != nil {
		return fmt.Errorf("writing stdin-eof frame: %w", err)
	}

	return nil
}

// Next blocks for the next frame from the guest-agent and translates it into
// an Event. EventExit always ends the exchange.
//
// When the exec request set a TimeoutSec, each read carries a deadline of
// TimeoutSec-plus-grace: the guest-agent enforces TimeoutSec itself and
// should respond by then, so a read timing out past that means the
// connection is wedged (some vsock transports don't reliably deliver
// EOF/RST to the host side when the guest-agent closes its end) rather than
// the command just being quiet. Requests with no TimeoutSec get no deadline,
// since there's then no way to tell a healthy quiet command from a dead one.
func (s *Session) Next() (Event, error) {
	for {
		if s.idleTimeout > 0 {
			if err := s.conn.SetReadDeadline(time.Now().Add(s.idleTimeout)); err != nil {
				return Event{}, fmt.Errorf("setting exec session read deadline: %w", err)
			}
		}

		f, err := vsockclient.ReadFrame(s.conn)
		if err != nil {
			var netErr net.Error
			if errors.As(err, &netErr) && netErr.Timeout() {
				return Event{}, fmt.Errorf("guest-agent exec session timed out waiting for next frame: %w", err)
			}

			return Event{}, fmt.Errorf("reading guest-agent frame: %w", err)
		}

		//nolint:exhaustive // FrameRequest/FrameStdin/FrameStdinEOF are host->agent-only and never read here
		switch f.Type {
		case vsockclient.FrameStdout:
			return Event{Type: EventStdout, Data: f.Payload}, nil
		case vsockclient.FrameStderr:
			return Event{Type: EventStderr, Data: f.Payload}, nil
		case vsockclient.FrameExit:
			var ex vsockclient.ExitMessage
			if err := json.Unmarshal(f.Payload, &ex); err != nil {
				return Event{}, fmt.Errorf("decoding exit frame: %w", err)
			}

			return Event{Type: EventExit, ExitCode: ex.Code}, nil
		case vsockclient.FrameError:
			var em vsockclient.ErrorMessage
			if err := json.Unmarshal(f.Payload, &em); err != nil {
				return Event{}, fmt.Errorf("decoding error frame: %w", err)
			}

			return Event{Type: EventError, Message: em.Msg}, nil
		default:
			// Unexpected frame type on this channel; skip it rather than fail
			// the whole session over a forward-compatible addition.
			continue
		}
	}
}

// Close closes the underlying vsock connection.
func (s *Session) Close() error {
	return s.conn.Close()
}
