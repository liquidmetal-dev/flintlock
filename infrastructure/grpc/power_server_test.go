package grpc_test

import (
	"context"
	"errors"
	"testing"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/gomega"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	mvm1 "github.com/liquidmetal-dev/flintlock/api/services/microvm/v1alpha1"
	coreerrs "github.com/liquidmetal-dev/flintlock/core/errors"
	"github.com/liquidmetal-dev/flintlock/infrastructure/grpc"
	"github.com/liquidmetal-dev/flintlock/infrastructure/mock"
)

func TestServer_PauseResumeMicroVM(t *testing.T) {
	type call func(ctx context.Context, svr mvm1.MicroVMServer, uid string) error

	pause := func(ctx context.Context, svr mvm1.MicroVMServer, uid string) error {
		_, err := svr.PauseMicroVM(ctx, &mvm1.PauseMicroVMRequest{Uid: uid})

		return err
	}
	resume := func(ctx context.Context, svr mvm1.MicroVMServer, uid string) error {
		_, err := svr.ResumeMicroVM(ctx, &mvm1.ResumeMicroVMRequest{Uid: uid})

		return err
	}
	expectPause := func(cm *mock.MockMicroVMCommandUseCasesMockRecorder, err error) {
		cm.PauseMicroVM(gomock.Any(), "testuid").Return(err)
	}
	expectResume := func(cm *mock.MockMicroVMCommandUseCasesMockRecorder, err error) {
		cm.ResumeMicroVM(gomock.Any(), "testuid").Return(err)
	}

	tt := []struct {
		name     string
		call     call
		expect   func(cm *mock.MockMicroVMCommandUseCasesMockRecorder, err error)
		uid      string
		ucErr    error
		wantCode codes.Code
	}{
		{name: "pause", call: pause, expect: expectPause, uid: "testuid", wantCode: codes.OK},
		{name: "pause without a uid", call: pause, wantCode: codes.InvalidArgument},
		{
			name: "pause of a microvm that does not exist", call: pause, expect: expectPause, uid: "testuid",
			ucErr: coreerrs.NewSpecNotFound("", "", "", "testuid"), wantCode: codes.NotFound,
		},
		{
			name: "pause of a microvm that does not run", call: pause, expect: expectPause, uid: "testuid",
			ucErr: coreerrs.NewFailedPrecondition("microvm testuid is not running"), wantCode: codes.FailedPrecondition,
		},
		{
			name: "pause that fails", call: pause, expect: expectPause, uid: "testuid",
			ucErr: errors.New("boom"), wantCode: codes.Unknown,
		},
		{name: "resume", call: resume, expect: expectResume, uid: "testuid", wantCode: codes.OK},
		{name: "resume without a uid", call: resume, wantCode: codes.InvalidArgument},
		{
			name: "resume of a microvm that is not paused", call: resume, expect: expectResume, uid: "testuid",
			ucErr: coreerrs.NewFailedPrecondition("microvm testuid is not paused"), wantCode: codes.FailedPrecondition,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			RegisterTestingT(t)

			mockCtrl := gomock.NewController(t)
			cm := mock.NewMockMicroVMCommandUseCases(mockCtrl)
			qm := mock.NewMockMicroVMQueryUseCases(mockCtrl)

			if tc.expect != nil {
				tc.expect(cm.EXPECT(), tc.ucErr)
			}

			svr := grpc.NewServer(cm, qm, grpc.InfoConfig{})
			err := tc.call(context.Background(), svr, tc.uid)

			Expect(status.Code(err)).To(Equal(tc.wantCode), "error: %v", err)
		})
	}
}

func TestServer_PauseResumeMicroVM_NilRequest(t *testing.T) {
	RegisterTestingT(t)

	mockCtrl := gomock.NewController(t)
	svr := grpc.NewServer(mock.NewMockMicroVMCommandUseCases(mockCtrl), mock.NewMockMicroVMQueryUseCases(mockCtrl), grpc.InfoConfig{})

	_, err := svr.PauseMicroVM(context.Background(), nil)
	Expect(status.Code(err)).To(Equal(codes.InvalidArgument))

	_, err = svr.ResumeMicroVM(context.Background(), nil)
	Expect(status.Code(err)).To(Equal(codes.InvalidArgument))
}
