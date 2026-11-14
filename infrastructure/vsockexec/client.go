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
	// idleTimeout is the deadline Next() applies to each read. It starts at
	// the legacy TimeoutSec-derived value (see idleTimeoutFor) and switches
	// to heartbeatIdleTimeout the first time a FrameHeartbeat is observed,
	// proving this session's guest-agent supports heartbeat-based liveness.
	// A guest-agent that never sends one (pre-v0.4.0) keeps the
	// TimeoutSec-derived value for the life of the session.
	idleTimeout time.Duration
	// heartbeatIdleTimeout is the deadline idleTimeout switches to once a
	// FrameHeartbeat has been observed; see defaults.ExecSessionIdleTimeout
	// for how it's sized.
	heartbeatIdleTimeout time.Duration
}

// idleTimeoutFor derives the idle read deadline an exec session starts with,
// from the request's TimeoutSec, before its guest-agent has proven (via a
// FrameHeartbeat) that it supports heartbeat-based liveness:
//
//   - TimeoutSec > 0: the guest-agent enforces it itself and is expected to
//     report an outcome by then, so TimeoutSec-plus-grace bounds how long the
//     guest-agent should legitimately take to respond, not how long the
//     command may stay quiet.
//   - TimeoutSec == 0 ("unbounded" per the exec protocol): there's no
//     declared budget to derive a deadline from, and no protocol-level way
//     to tell a healthy quiet command (e.g. a long sleep) from a wedged
//     connection. Rather than leave the read able to block forever — which
//     is the failure mode being guarded against — fall back to a generous
//     ceiling: a last-resort circuit breaker, not a liveness check, so it's
//     sized to basically never trip a real quiet workload.
func idleTimeoutFor(timeoutSec int) time.Duration {
	if timeoutSec > 0 {
		return time.Duration(timeoutSec)*time.Second + defaults.ExecSessionIdleGrace
	}

	return defaults.ExecSessionUnboundedIdleCeiling
}

// Start dials udsPath (a Firecracker/Cloud Hypervisor vsock UDS multiplexer)
// on the guest-agent's control port and sends an exec request. Callers must
// call Close when done.
func Start(ctx context.Context, udsPath string, controlPort uint32, exec *vsockclient.Exec) (*Session, error) {
	conn, err := dialWithRetry(ctx, udsPath, controlPort)
	if err != nil {
		return nil, fmt.Errorf("dialling guest-agent control channel: %w", err)
	}

	req := &vsockclient.Request{Version: vsockclient.Version, Op: vsockclient.OpExec, Exec: exec}
	if err := vsockclient.WriteRequest(conn, req); err != nil {
		conn.Close()

		return nil, fmt.Errorf("sending exec request: %w", err)
	}

	return &Session{
		conn:                 conn,
		idleTimeout:          idleTimeoutFor(exec.TimeoutSec),
		heartbeatIdleTimeout: defaults.ExecSessionIdleTimeout,
	}, nil
}

// dialRetries and dialRetryDelay back dialWithRetry; they default to the
// package defaults but are overridable by tests via SetDialRetryParams so
// retry-exhaustion cases don't have to sleep through the real delay.
var (
	dialRetries    = defaults.GuestAgentDialRetries
	dialRetryDelay = defaults.GuestAgentDialRetryDelay
)

// SetDialRetryParams overrides the retry count/delay dialWithRetry uses,
// returning a func that restores the previous values. Mainly useful for
// tests that need to exercise retry exhaustion without waiting out the real
// delay.
func SetDialRetryParams(retries int, delay time.Duration) (restore func()) {
	prevRetries, prevDelay := dialRetries, dialRetryDelay
	dialRetries, dialRetryDelay = retries, delay

	return func() { dialRetries, dialRetryDelay = prevRetries, prevDelay }
}

// dialWithRetry dials udsPath/controlPort, retrying up to dialRetries times
// (with dialRetryDelay between attempts) on failure. Rapid repeated dials
// against the same vsock port have been observed to occasionally get EOF
// instead of the CONNECT handshake's OK reply — most likely a transient
// vsock multiplexer hiccup rather than a genuine failure to connect — so a
// few quick retries let the handshake ride that out. See
// defaults.GuestAgentDialRetries.
func dialWithRetry(ctx context.Context, udsPath string, controlPort uint32) (net.Conn, error) {
	var lastErr error

	for attempt := 0; attempt <= dialRetries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, lastErr
			case <-time.After(dialRetryDelay):
			}
		}

		conn, err := vsockclient.Dial(ctx, udsPath, controlPort)
		if err == nil {
			return conn, nil
		}

		lastErr = err
	}

	return nil, lastErr
}

// SetIdleTimeout overrides the idle deadline Next() applies to each read
// before a FrameHeartbeat has been observed. Mainly useful for tests that
// need to exercise the pre-heartbeat timeout path without waiting out the
// real one derived from the exec request's TimeoutSec.
func (s *Session) SetIdleTimeout(d time.Duration) {
	s.idleTimeout = d
}

// SetHeartbeatIdleTimeout overrides the idle deadline Next() switches to
// once a FrameHeartbeat has been observed. Mainly useful for tests that need
// to exercise the post-heartbeat timeout path without waiting out the real
// one.
func (s *Session) SetHeartbeatIdleTimeout(d time.Duration) {
	s.heartbeatIdleTimeout = d
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
// Each read carries an idle deadline so a wedged connection surfaces as an
// error instead of blocking forever — some vsock transports don't reliably
// deliver EOF/RST to the host side when the guest-agent closes its end. That
// deadline starts out derived from the exec request's TimeoutSec (see
// idleTimeoutFor) and switches, the first time a FrameHeartbeat arrives, to
// the much tighter defaults.ExecSessionIdleTimeout — a heartbeat proves this
// session's guest-agent supports that liveness signal (v0.4.0+), whereas an
// older guest-agent that never sends one keeps the TimeoutSec-derived
// deadline for the life of the session. Heartbeats are never surfaced as an
// Event.
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
		case vsockclient.FrameHeartbeat:
			// Proves this session's guest-agent supports heartbeat-based
			// liveness; switch to the tighter post-heartbeat deadline from
			// here on instead of the legacy TimeoutSec-derived one. Never
			// surfaced as an Event.
			s.idleTimeout = s.heartbeatIdleTimeout

			continue
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
