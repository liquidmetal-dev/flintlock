package grpc_test

import (
	"bufio"
	"errors"
	"fmt"
	"net"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/gomega"

	mvmexecv1 "github.com/liquidmetal-dev/flintlock/api/services/microvmexec/v1alpha1"
	"github.com/liquidmetal-dev/flintlock/core/models"
	"github.com/liquidmetal-dev/flintlock/infrastructure/grpc"
	"github.com/liquidmetal-dev/flintlock/infrastructure/mock"
	"github.com/liquidmetal-dev/guest-agent/pkg/vsockclient"
)

func TestExecServer_ExecCommand_FirstMessageMustBeStart(t *testing.T) {
	RegisterTestingT(t)

	mockCtrl := gomock.NewController(t)
	qm := mock.NewMockMicroVMQueryUseCases(mockCtrl)

	stream := newFakeBidiStream[mvmexecv1.ExecCommandRequest, mvmexecv1.ExecCommandResponse]()
	stream.pushRecv(&mvmexecv1.ExecCommandRequest{
		Payload: &mvmexecv1.ExecCommandRequest_Stdin{Stdin: []byte("too early")},
	})

	svr := grpc.NewExecServer(qm)
	err := svr.ExecCommand(stream)

	Expect(err).To(HaveOccurred())
}

func TestExecServer_ExecCommand_InvalidStart(t *testing.T) {
	tt := []struct {
		name  string
		start *mvmexecv1.ExecStart
	}{
		{name: "missing uid", start: &mvmexecv1.ExecStart{Cmd: "uname"}},
		{name: "missing cmd and not shell", start: &mvmexecv1.ExecStart{Uid: "vm1"}},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			RegisterTestingT(t)

			mockCtrl := gomock.NewController(t)
			qm := mock.NewMockMicroVMQueryUseCases(mockCtrl)

			stream := newFakeBidiStream[mvmexecv1.ExecCommandRequest, mvmexecv1.ExecCommandResponse]()
			stream.pushRecv(&mvmexecv1.ExecCommandRequest{
				Payload: &mvmexecv1.ExecCommandRequest_Start{Start: tc.start},
			})

			svr := grpc.NewExecServer(qm)
			err := svr.ExecCommand(stream)

			Expect(err).To(HaveOccurred())
		})
	}
}

func TestExecServer_ExecCommand_GetMicroVMError(t *testing.T) {
	RegisterTestingT(t)

	mockCtrl := gomock.NewController(t)
	qm := mock.NewMockMicroVMQueryUseCases(mockCtrl)
	qm.EXPECT().GetMicroVM(gomock.Any(), "vm1").Return(nil, errors.New("boom"))

	stream := newFakeBidiStream[mvmexecv1.ExecCommandRequest, mvmexecv1.ExecCommandResponse]()
	stream.pushRecv(&mvmexecv1.ExecCommandRequest{
		Payload: &mvmexecv1.ExecCommandRequest_Start{Start: &mvmexecv1.ExecStart{Uid: "vm1", Cmd: "uname"}},
	})

	svr := grpc.NewExecServer(qm)
	err := svr.ExecCommand(stream)

	Expect(err).To(HaveOccurred())
}

func TestExecServer_ExecCommand_Preconditions(t *testing.T) {
	tt := []struct {
		name string
		vm   *models.MicroVM
	}{
		{
			name: "guest agent not allowed",
			vm: &models.MicroVM{
				Spec:   models.MicroVMSpec{AllowGuestAgent: false},
				Status: models.MicroVMStatus{VSockPath: "/tmp/whatever.vsock", State: models.CreatedState},
			},
		},
		{
			name: "no vsock path",
			vm: &models.MicroVM{
				Spec:   models.MicroVMSpec{AllowGuestAgent: true},
				Status: models.MicroVMStatus{VSockPath: "", State: models.CreatedState},
			},
		},
		{
			name: "wrong state",
			vm: &models.MicroVM{
				Spec:   models.MicroVMSpec{AllowGuestAgent: true},
				Status: models.MicroVMStatus{VSockPath: "/tmp/whatever.vsock", State: models.PendingState},
			},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			RegisterTestingT(t)

			mockCtrl := gomock.NewController(t)
			qm := mock.NewMockMicroVMQueryUseCases(mockCtrl)
			qm.EXPECT().GetMicroVM(gomock.Any(), "vm1").Return(tc.vm, nil)

			stream := newFakeBidiStream[mvmexecv1.ExecCommandRequest, mvmexecv1.ExecCommandResponse]()
			stream.pushRecv(&mvmexecv1.ExecCommandRequest{
				Payload: &mvmexecv1.ExecCommandRequest_Start{Start: &mvmexecv1.ExecStart{Uid: "vm1", Cmd: "uname"}},
			})

			svr := grpc.NewExecServer(qm)
			err := svr.ExecCommand(stream)

			Expect(err).To(HaveOccurred())
		})
	}
}

// fakeGuestAgentExec listens on a UDS, performs the CONNECT handshake once,
// reads the exec request frame, then hands the connection to script so the
// test can drive whatever exchange it needs.
func fakeGuestAgentExec(t *testing.T, port uint32, script func(net.Conn)) string {
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
			conn.Close()

			return
		}
		fmt.Fprintf(conn, "OK 0\n")

		if _, err := vsockclient.ReadFrame(conn); err != nil {
			conn.Close()

			return
		}

		script(conn)
	}()

	return udsPath
}

func TestExecServer_ExecCommand_HappyPath(t *testing.T) {
	RegisterTestingT(t)

	const controlPort = 1024

	udsPath := fakeGuestAgentExec(t, controlPort, func(conn net.Conn) {
		defer conn.Close()

		vsockclient.WriteFrame(conn, vsockclient.FrameStdout, []byte("out"))
		vsockclient.WriteFrame(conn, vsockclient.FrameStderr, []byte("err"))
		vsockclient.WriteExit(conn, 3)
	})

	mockCtrl := gomock.NewController(t)
	qm := mock.NewMockMicroVMQueryUseCases(mockCtrl)
	qm.EXPECT().GetMicroVM(gomock.Any(), "vm1").Return(&models.MicroVM{
		Spec:   models.MicroVMSpec{AllowGuestAgent: true},
		Status: models.MicroVMStatus{VSockPath: udsPath, State: models.CreatedState},
	}, nil)

	stream := newFakeBidiStream[mvmexecv1.ExecCommandRequest, mvmexecv1.ExecCommandResponse]()
	stream.pushRecv(&mvmexecv1.ExecCommandRequest{
		Payload: &mvmexecv1.ExecCommandRequest_Start{Start: &mvmexecv1.ExecStart{Uid: "vm1", Cmd: "uname"}},
	})

	svr := grpc.NewExecServer(qm)
	err := svr.ExecCommand(stream)

	Expect(err).NotTo(HaveOccurred())

	sent := stream.Sent()
	Expect(sent).To(HaveLen(3))
	Expect(sent[0].GetStdout()).To(Equal([]byte("out")))
	Expect(sent[1].GetStderr()).To(Equal([]byte("err")))
	Expect(sent[2].GetExitCode()).To(Equal(int32(3)))
}

func TestExecServer_ExecCommand_StdinRoundtrip(t *testing.T) {
	RegisterTestingT(t)

	const controlPort = 1024

	udsPath := fakeGuestAgentExec(t, controlPort, func(conn net.Conn) {
		defer conn.Close()

		for {
			f, err := vsockclient.ReadFrame(conn)
			if err != nil {
				return
			}
			switch f.Type {
			case vsockclient.FrameStdin:
				vsockclient.WriteFrame(conn, vsockclient.FrameStdout, f.Payload)
			case vsockclient.FrameStdinEOF:
				vsockclient.WriteExit(conn, 0)

				return
			}
		}
	})

	mockCtrl := gomock.NewController(t)
	qm := mock.NewMockMicroVMQueryUseCases(mockCtrl)
	qm.EXPECT().GetMicroVM(gomock.Any(), "vm1").Return(&models.MicroVM{
		Spec:   models.MicroVMSpec{AllowGuestAgent: true},
		Status: models.MicroVMStatus{VSockPath: udsPath, State: models.CreatedState},
	}, nil)

	stream := newFakeBidiStream[mvmexecv1.ExecCommandRequest, mvmexecv1.ExecCommandResponse]()
	stream.pushRecv(&mvmexecv1.ExecCommandRequest{
		Payload: &mvmexecv1.ExecCommandRequest_Start{Start: &mvmexecv1.ExecStart{Uid: "vm1", Cmd: "cat", HasStdin: true}},
	})
	stream.pushRecv(&mvmexecv1.ExecCommandRequest{
		Payload: &mvmexecv1.ExecCommandRequest_Stdin{Stdin: []byte("payload")},
	})
	stream.pushRecv(&mvmexecv1.ExecCommandRequest{
		Payload: &mvmexecv1.ExecCommandRequest_StdinEof{StdinEof: true},
	})

	svr := grpc.NewExecServer(qm)

	done := make(chan error, 1)
	go func() { done <- svr.ExecCommand(stream) }()

	select {
	case err := <-done:
		Expect(err).NotTo(HaveOccurred())
	case <-time.After(5 * time.Second):
		t.Fatal("ExecCommand did not return in time")
	}

	sent := stream.Sent()
	Expect(sent).To(HaveLen(2))
	Expect(sent[0].GetStdout()).To(Equal([]byte("payload")))
	Expect(sent[1].GetExitCode()).To(Equal(int32(0)))
}
