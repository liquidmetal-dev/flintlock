package grpc

import (
	"errors"
	"fmt"
	"io"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	mvmsshproxyv1 "github.com/liquidmetal-dev/flintlock/api/services/microvmsshproxy/v1alpha1"
	"github.com/liquidmetal-dev/flintlock/core/ports"
	"github.com/liquidmetal-dev/flintlock/infrastructure/vsocksshproxy"
	"github.com/liquidmetal-dev/flintlock/pkg/defaults"
	"github.com/liquidmetal-dev/flintlock/pkg/log"
)

// NewSSHProxyServer creates a new gRPC server for the MicroVMSSHProxy service.
func NewSSHProxyServer(queryUC ports.MicroVMQueryUseCases) ports.MicroVMSSHProxyGRPCService {
	return &sshProxyServer{queryUC: queryUC}
}

type sshProxyServer struct {
	queryUC ports.MicroVMQueryUseCases
}

func (s *sshProxyServer) SSHProxy(stream mvmsshproxyv1.MicroVMSSHProxy_SSHProxyServer) error {
	ctx := stream.Context()
	logger := log.GetLogger(ctx)

	first, err := stream.Recv()
	if err != nil {
		return fmt.Errorf("receiving ssh proxy start message: %w", err)
	}

	uid, ok := first.GetPayload().(*mvmsshproxyv1.SSHProxyRequest_Uid)
	if !ok || uid.Uid == "" {
		//nolint:wrapcheck // don't wrap grpc errors when using the status package
		return status.Error(codes.InvalidArgument, "first message must set uid")
	}

	vm, err := s.queryUC.GetMicroVM(ctx, uid.Uid)
	if err != nil {
		return fmt.Errorf("getting microvm: %w", err)
	}

	if err := validateGuestAgentAccess(vm); err != nil {
		return err
	}

	logger.Infof("proxying ssh to microvm %s", uid.Uid)

	session, err := vsocksshproxy.Start(ctx, vm.Status.VSockPath, defaults.GuestAgentSSHPort)
	if err != nil {
		return fmt.Errorf("starting guest-agent ssh proxy session: %w", err)
	}
	defer session.Close()

	toGuestErr := make(chan error, 1)
	toClientErr := make(chan error, 1)

	go func() {
		toGuestErr <- pumpSSHProxyToGuest(stream, session)
	}()
	go func() {
		toClientErr <- pumpSSHProxyToClient(stream, session)
	}()

	select {
	case err := <-toGuestErr:
		if err == nil {
			return nil
		}
		if !errors.Is(err, io.EOF) {
			return fmt.Errorf("proxying ssh traffic: %w", err)
		}

		// The client is done sending (a graceful half-close, not a failure):
		// tell the guest so its sshd can see EOF on stdin, then wait for any
		// response still in flight instead of tearing the session down and
		// truncating it. If the transport can't half-close, there is no
		// bounded way to know when the guest will stop, so fall back to the
		// old behaviour (return now) rather than waiting forever.
		if cwErr := session.CloseWrite(); cwErr != nil {
			logger.Debugf("half-closing ssh proxy session: %s", cwErr)

			return nil
		}

		if err := <-toClientErr; err != nil && !errors.Is(err, io.EOF) {
			return fmt.Errorf("proxying ssh traffic: %w", err)
		}

		return nil
	case err := <-toClientErr:
		// The guest side ended (closed or errored): nothing more to relay
		// either way, regardless of what the client is still doing.
		if err != nil && !errors.Is(err, io.EOF) {
			return fmt.Errorf("proxying ssh traffic: %w", err)
		}

		return nil
	}
}

// pumpSSHProxyToGuest relays client->server messages onto the guest-agent
// ssh connection until the client stream ends or a write fails.
func pumpSSHProxyToGuest(stream mvmsshproxyv1.MicroVMSSHProxy_SSHProxyServer, session *vsocksshproxy.Session) error {
	for {
		msg, err := stream.Recv()
		if err != nil {
			return fmt.Errorf("receiving ssh proxy message: %w", err)
		}

		data, ok := msg.GetPayload().(*mvmsshproxyv1.SSHProxyRequest_Data)
		if !ok {
			continue
		}

		if _, err := session.Write(data.Data); err != nil {
			return err
		}
	}
}

// pumpSSHProxyToClient relays guest-agent ssh bytes back to the client until
// the connection closes or a send fails.
func pumpSSHProxyToClient(stream mvmsshproxyv1.MicroVMSSHProxy_SSHProxyServer, session *vsocksshproxy.Session) error {
	buf := make([]byte, 32*1024)

	for {
		n, err := session.Read(buf)
		if n > 0 {
			if sendErr := stream.Send(&mvmsshproxyv1.SSHProxyResponse{Data: append([]byte(nil), buf[:n]...)}); sendErr != nil {
				return fmt.Errorf("sending ssh proxy response: %w", sendErr)
			}
		}

		if err != nil {
			return err
		}
	}
}
