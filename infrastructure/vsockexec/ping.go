package vsockexec

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/liquidmetal-dev/guest-agent/pkg/vsockclient"

	"github.com/liquidmetal-dev/flintlock/core/ports"
	"github.com/liquidmetal-dev/flintlock/pkg/defaults"
)

// NewPinger returns a GuestAgentService that pings the guest-agent control channel.
func NewPinger() ports.GuestAgentService {
	return &pinger{}
}

type pinger struct{}

// Ping runs the attempt in a goroutine because the vsockclient.Dial handshake
// obeys only a deadline, not ctx: a cancelled ctx must not wait for it.
func (p *pinger) Ping(ctx context.Context, vsockPath string) error {
	attemptCtx, cancel := context.WithTimeout(ctx, defaults.GuestAgentPingAttemptTimeout)
	defer cancel()

	result := make(chan error, 1)

	go func() {
		result <- pingOnce(attemptCtx, vsockPath)
	}()

	select {
	case err := <-result:
		return err
	case <-ctx.Done():
		return fmt.Errorf("pinging guest-agent: %w", ctx.Err())
	}
}

func pingOnce(ctx context.Context, vsockPath string) error {
	conn, err := vsockclient.Dial(ctx, vsockPath, defaults.GuestAgentControlPort)
	if err != nil {
		return fmt.Errorf("dialling guest-agent: %w", err)
	}
	defer conn.Close()

	stop := context.AfterFunc(ctx, func() { conn.Close() })
	defer stop()

	if deadline, ok := ctx.Deadline(); ok {
		if err := conn.SetDeadline(deadline); err != nil {
			return fmt.Errorf("setting ping deadline: %w", err)
		}
	}

	req := &vsockclient.Request{Version: vsockclient.Version, Op: vsockclient.OpPing}
	if err := vsockclient.WriteRequest(conn, req); err != nil {
		return fmt.Errorf("writing ping request: %w", err)
	}

	var agentErr error

	for {
		frame, err := vsockclient.ReadFrame(conn)
		if err != nil {
			return fmt.Errorf("reading ping reply: %w", err)
		}

		//nolint:exhaustive // A ping reply ends with an exit frame; stdout, stderr and heartbeat frames are ignored.
		switch frame.Type {
		case vsockclient.FrameError:
			var msg vsockclient.ErrorMessage
			_ = json.Unmarshal(frame.Payload, &msg)
			agentErr = fmt.Errorf("guest-agent error: %s", msg.Msg)
		case vsockclient.FrameExit:
			var exit vsockclient.ExitMessage
			if err := json.Unmarshal(frame.Payload, &exit); err != nil {
				return fmt.Errorf("decoding ping exit: %w", err)
			}

			if agentErr != nil {
				return agentErr
			}

			if exit.Code != 0 {
				return fmt.Errorf("%w: exit code %d", errPingFailed, exit.Code)
			}

			return nil
		}
	}
}

var errPingFailed = errors.New("guest-agent ping failed")
