package grpc_test

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/gomega"

	mvmsshproxyv1 "github.com/liquidmetal-dev/flintlock/api/services/microvmsshproxy/v1alpha1"
	"github.com/liquidmetal-dev/flintlock/core/models"
	"github.com/liquidmetal-dev/flintlock/infrastructure/grpc"
	"github.com/liquidmetal-dev/flintlock/infrastructure/mock"
)

func TestSSHProxyServer_SSHProxy_FirstMessageMustSetUid(t *testing.T) {
	RegisterTestingT(t)

	mockCtrl := gomock.NewController(t)
	qm := mock.NewMockMicroVMQueryUseCases(mockCtrl)

	stream := newFakeBidiStream[mvmsshproxyv1.SSHProxyRequest, mvmsshproxyv1.SSHProxyResponse]()
	stream.pushRecv(&mvmsshproxyv1.SSHProxyRequest{
		Payload: &mvmsshproxyv1.SSHProxyRequest_Data{Data: []byte("too early")},
	})

	svr := grpc.NewSSHProxyServer(qm)
	err := svr.SSHProxy(stream)

	Expect(err).To(HaveOccurred())
}

func TestSSHProxyServer_SSHProxy_GetMicroVMError(t *testing.T) {
	RegisterTestingT(t)

	mockCtrl := gomock.NewController(t)
	qm := mock.NewMockMicroVMQueryUseCases(mockCtrl)
	qm.EXPECT().GetMicroVM(gomock.Any(), "vm1").Return(nil, errors.New("boom"))

	stream := newFakeBidiStream[mvmsshproxyv1.SSHProxyRequest, mvmsshproxyv1.SSHProxyResponse]()
	stream.pushRecv(&mvmsshproxyv1.SSHProxyRequest{
		Payload: &mvmsshproxyv1.SSHProxyRequest_Uid{Uid: "vm1"},
	})

	svr := grpc.NewSSHProxyServer(qm)
	err := svr.SSHProxy(stream)

	Expect(err).To(HaveOccurred())
}

func TestSSHProxyServer_SSHProxy_Preconditions(t *testing.T) {
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

			stream := newFakeBidiStream[mvmsshproxyv1.SSHProxyRequest, mvmsshproxyv1.SSHProxyResponse]()
			stream.pushRecv(&mvmsshproxyv1.SSHProxyRequest{
				Payload: &mvmsshproxyv1.SSHProxyRequest_Uid{Uid: "vm1"},
			})

			svr := grpc.NewSSHProxyServer(qm)
			err := svr.SSHProxy(stream)

			Expect(err).To(HaveOccurred())
		})
	}
}

// fakeGuestAgentSSH listens on a UDS, performs the CONNECT handshake once,
// then echoes back whatever it receives - standing in for guest-agent's raw
// ssh-proxy port.
func fakeGuestAgentSSH(t *testing.T, port uint32) string {
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
		defer conn.Close()

		r := bufio.NewReader(conn)
		line, err := r.ReadString('\n')
		if err != nil {
			return
		}
		if strings.TrimSpace(line) != fmt.Sprintf("CONNECT %d", port) {
			return
		}
		fmt.Fprintf(conn, "OK 0\n")

		io.Copy(conn, r) // echo anything sent after the handshake
	}()

	return udsPath
}

func TestSSHProxyServer_SSHProxy_EchoRoundtripThenClientEOF(t *testing.T) {
	RegisterTestingT(t)

	const sshPort = 1025

	udsPath := fakeGuestAgentSSH(t, sshPort)

	mockCtrl := gomock.NewController(t)
	qm := mock.NewMockMicroVMQueryUseCases(mockCtrl)
	qm.EXPECT().GetMicroVM(gomock.Any(), "vm1").Return(&models.MicroVM{
		Spec:   models.MicroVMSpec{AllowGuestAgent: true},
		Status: models.MicroVMStatus{VSockPath: udsPath, State: models.CreatedState},
	}, nil)

	stream := newFakeBidiStream[mvmsshproxyv1.SSHProxyRequest, mvmsshproxyv1.SSHProxyResponse]()
	stream.pushRecv(&mvmsshproxyv1.SSHProxyRequest{
		Payload: &mvmsshproxyv1.SSHProxyRequest_Uid{Uid: "vm1"},
	})
	stream.pushRecv(&mvmsshproxyv1.SSHProxyRequest{
		Payload: &mvmsshproxyv1.SSHProxyRequest_Data{Data: []byte("ping")},
	})

	svr := grpc.NewSSHProxyServer(qm)

	done := make(chan error, 1)
	go func() { done <- svr.SSHProxy(stream) }()

	// Wait for the echoed "ping" to come back through the stream before
	// signalling client EOF, so the roundtrip is actually observed.
	deadline := time.Now().Add(5 * time.Second)
	for {
		var got []byte
		for _, resp := range stream.Sent() {
			got = append(got, resp.GetData()...)
		}
		if string(got) == "ping" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for echoed data, got %q so far", got)
		}
		time.Sleep(10 * time.Millisecond)
	}

	stream.pushErr(io.EOF)

	select {
	case err := <-done:
		Expect(err).NotTo(HaveOccurred())
	case <-time.After(5 * time.Second):
		t.Fatal("SSHProxy did not return after client EOF")
	}
}
