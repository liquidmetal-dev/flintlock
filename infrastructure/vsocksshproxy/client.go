// Package vsocksshproxy opens a raw byte tunnel to a guest-agent's ssh port,
// which itself proxies straight to the guest's local sshd - no framing, no
// agent-level auth.
package vsocksshproxy

import (
	"context"
	"fmt"
	"net"

	"github.com/liquidmetal-dev/guest-agent/pkg/vsockclient"
)

// Session is one raw byte tunnel to a guest-agent's ssh port, reached by
// dialling a vsock UDS multiplexer path.
type Session struct {
	conn net.Conn
}

// Start dials udsPath (a Firecracker/Cloud Hypervisor vsock UDS multiplexer)
// on the guest-agent's ssh port. Callers must call Close when done.
func Start(ctx context.Context, udsPath string, sshPort uint32) (*Session, error) {
	conn, err := vsockclient.Dial(ctx, udsPath, sshPort)
	if err != nil {
		return nil, fmt.Errorf("dialling guest-agent ssh channel: %w", err)
	}

	return &Session{conn: conn}, nil
}

// Write forwards p toward the guest's sshd.
func (s *Session) Write(p []byte) (int, error) {
	n, err := s.conn.Write(p)
	if err != nil {
		return n, fmt.Errorf("writing to guest-agent ssh channel: %w", err)
	}

	return n, nil
}

// Read reads bytes coming from the guest's sshd.
func (s *Session) Read(p []byte) (int, error) {
	n, err := s.conn.Read(p)
	if err != nil {
		return n, fmt.Errorf("reading from guest-agent ssh channel: %w", err)
	}

	return n, nil
}

// CloseWrite half-closes the write side, signalling EOF to the guest-agent
// (and, through it, the guest's sshd) while still allowing reads - used when
// the client is done sending but the guest may still have a response coming.
// Returns an error if the underlying connection doesn't support half-close.
func (s *Session) CloseWrite() error {
	cw, ok := s.conn.(interface{ CloseWrite() error })
	if !ok {
		return fmt.Errorf("%T does not support half-close", s.conn)
	}

	if err := cw.CloseWrite(); err != nil {
		return fmt.Errorf("closing write side of guest-agent ssh channel: %w", err)
	}

	return nil
}

// Close closes the underlying vsock connection.
func (s *Session) Close() error {
	return s.conn.Close()
}
