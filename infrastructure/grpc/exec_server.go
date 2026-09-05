package grpc

import (
	"errors"
	"fmt"
	"io"

	"github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	mvmexecv1 "github.com/liquidmetal-dev/flintlock/api/services/microvmexec/v1alpha1"
	"github.com/liquidmetal-dev/flintlock/core/models"
	"github.com/liquidmetal-dev/flintlock/core/ports"
	"github.com/liquidmetal-dev/flintlock/infrastructure/vsockexec"
	"github.com/liquidmetal-dev/flintlock/pkg/defaults"
	"github.com/liquidmetal-dev/flintlock/pkg/log"
	"github.com/liquidmetal-dev/guest-agent/pkg/vsockclient"
)

// NewExecServer creates a new gRPC server for the MicroVMExec service.
func NewExecServer(queryUC ports.MicroVMQueryUseCases) ports.MicroVMExecGRPCService {
	return &execServer{queryUC: queryUC}
}

type execServer struct {
	queryUC ports.MicroVMQueryUseCases
}

func (s *execServer) ExecCommand(stream mvmexecv1.MicroVMExec_ExecCommandServer) error {
	ctx := stream.Context()
	logger := log.GetLogger(ctx)

	first, err := stream.Recv()
	if err != nil {
		return fmt.Errorf("receiving exec start message: %w", err)
	}

	start := first.GetStart()
	if start == nil || start.GetUid() == "" || (start.GetCmd() == "" && !start.GetShell()) {
		//nolint:wrapcheck // don't wrap grpc errors when using the status package
		return status.Error(codes.InvalidArgument, "first message must be a start message with uid and cmd set")
	}

	vm, err := s.queryUC.GetMicroVM(ctx, start.GetUid())
	if err != nil {
		return fmt.Errorf("getting microvm: %w", err)
	}

	if err := validateGuestAgentAccess(vm); err != nil {
		return err
	}

	logger.Infof("executing command on microvm %s", start.GetUid())

	execReq := &vsockclient.Exec{
		Cmd:        start.GetCmd(),
		Args:       start.GetArgs(),
		Cwd:        start.GetCwd(),
		Env:        start.GetEnv(),
		Shell:      start.GetShell(),
		User:       start.GetUser(),
		TimeoutSec: int(start.GetTimeoutSeconds()),
		HasStdin:   start.GetHasStdin(),
	}

	session, err := vsockexec.Start(ctx, vm.Status.VSockPath, defaults.GuestAgentControlPort, execReq)
	if err != nil {
		return fmt.Errorf("starting guest-agent exec session: %w", err)
	}
	defer session.Close()

	if start.GetHasStdin() {
		go pumpExecStdin(stream, session, logger)
	}

	for {
		event, err := session.Next()
		if err != nil {
			return fmt.Errorf("reading guest-agent exec session: %w", err)
		}

		resp, done := convertExecEvent(event)
		if err := stream.Send(resp); err != nil {
			return fmt.Errorf("sending exec response: %w", err)
		}

		if done {
			return nil
		}
	}
}

// pumpExecStdin relays client stdin/stdin_eof messages to the guest-agent
// session until the client stream ends or the session is closed (Recv/Send
// then error out, which the caller observes independently).
func pumpExecStdin(stream mvmexecv1.MicroVMExec_ExecCommandServer, session *vsockexec.Session, logger *logrus.Entry) {
	for {
		msg, err := stream.Recv()
		if err != nil {
			if !errors.Is(err, io.EOF) {
				logger.Debugf("exec stdin stream ended: %s", err)
			}

			_ = session.CloseStdin()

			return
		}

		switch p := msg.GetPayload().(type) {
		case *mvmexecv1.ExecCommandRequest_Stdin:
			if err := session.SendStdin(p.Stdin); err != nil {
				logger.Debugf("forwarding stdin: %s", err)

				return
			}
		case *mvmexecv1.ExecCommandRequest_StdinEof:
			if p.StdinEof {
				_ = session.CloseStdin()

				return
			}
		}
	}
}

// convertExecEvent translates a guest-agent session event into a response
// message. done is true once the exchange is over (exit received).
func convertExecEvent(event vsockexec.Event) (*mvmexecv1.ExecCommandResponse, bool) {
	switch event.Type {
	case vsockexec.EventStdout:
		return &mvmexecv1.ExecCommandResponse{
			Payload: &mvmexecv1.ExecCommandResponse_Stdout{Stdout: event.Data},
		}, false
	case vsockexec.EventStderr:
		return &mvmexecv1.ExecCommandResponse{
			Payload: &mvmexecv1.ExecCommandResponse_Stderr{Stderr: event.Data},
		}, false
	case vsockexec.EventError:
		return &mvmexecv1.ExecCommandResponse{
			Payload: &mvmexecv1.ExecCommandResponse_Error{Error: event.Message},
		}, false
	case vsockexec.EventExit:
		return &mvmexecv1.ExecCommandResponse{
			Payload: &mvmexecv1.ExecCommandResponse_ExitCode{ExitCode: int32(event.ExitCode)},
		}, true
	default:
		return &mvmexecv1.ExecCommandResponse{
			Payload: &mvmexecv1.ExecCommandResponse_Error{Error: "unknown guest-agent event"},
		}, true
	}
}

// validateGuestAgentAccess returns a gRPC status error unless vm is in a
// state where dialing its guest-agent vsock device makes sense.
func validateGuestAgentAccess(vm *models.MicroVM) error {
	if !vm.Spec.AllowGuestAgent || vm.Status.VSockPath == "" {
		//nolint:wrapcheck // don't wrap grpc errors when using the status package
		return status.Error(codes.FailedPrecondition, "microvm was not created with guest agent access enabled")
	}

	if vm.Status.State != models.CreatedState {
		//nolint:wrapcheck // don't wrap grpc errors when using the status package
		return status.Errorf(codes.FailedPrecondition, "microvm is not ready (state: %s)", vm.Status.State)
	}

	return nil
}
