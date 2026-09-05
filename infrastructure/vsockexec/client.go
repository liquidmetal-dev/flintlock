// Package vsockexec runs one exec request against a guest-agent's control
// channel, translating its framed protocol into a small typed event stream
// for the gRPC layer to relay.
package vsockexec

import (
	"context"
	"encoding/json"
	"fmt"
	"net"

	"github.com/liquidmetal-dev/guest-agent/pkg/vsockclient"
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
// channel, reached by dialing a vsock UDS multiplexer path.
type Session struct {
	conn net.Conn
}

// Start dials udsPath (a Firecracker/Cloud Hypervisor vsock UDS multiplexer)
// on the guest-agent's control port and sends an exec request. Callers must
// call Close when done.
func Start(ctx context.Context, udsPath string, controlPort uint32, exec *vsockclient.Exec) (*Session, error) {
	conn, err := vsockclient.Dial(ctx, udsPath, controlPort)
	if err != nil {
		return nil, fmt.Errorf("dialing guest-agent control channel: %w", err)
	}

	req := &vsockclient.Request{Version: vsockclient.Version, Op: vsockclient.OpExec, Exec: exec}
	if err := vsockclient.WriteRequest(conn, req); err != nil {
		conn.Close()

		return nil, fmt.Errorf("sending exec request: %w", err)
	}

	return &Session{conn: conn}, nil
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
func (s *Session) Next() (Event, error) {
	for {
		f, err := vsockclient.ReadFrame(s.conn)
		if err != nil {
			return Event{}, fmt.Errorf("reading guest-agent frame: %w", err)
		}

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
