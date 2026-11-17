package grpc_test

import (
	"context"
	"testing"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/gomega"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	mvm1 "github.com/liquidmetal-dev/flintlock/api/services/microvm/v1alpha1"
	"github.com/liquidmetal-dev/flintlock/api/types"
	coreerrs "github.com/liquidmetal-dev/flintlock/core/errors"
	"github.com/liquidmetal-dev/flintlock/core/models"
	"github.com/liquidmetal-dev/flintlock/infrastructure/grpc"
	"github.com/liquidmetal-dev/flintlock/infrastructure/mock"
	"github.com/liquidmetal-dev/flintlock/pkg/ptr"
)

func restoreCreateRequest() *mvm1.CreateMicroVMRequest {
	return &mvm1.CreateMicroVMRequest{
		SnapshotSource: &types.SnapshotSource{
			Snapshot:   &types.SnapshotRef{Ref: &types.SnapshotRef_Uid{Uid: "snap-uid"}},
			Name:       "clone1",
			Namespace:  ptr.String("ns1"),
			MemoryMode: types.MemoryMode_FILE,
			NetworkOverrides: []*types.HostNetworkOverride{
				{GuestDeviceName: "eth1", HostDeviceName: "tap1"},
			},
		},
	}
}

func TestServer_CreateMicroVM_Restore(t *testing.T) {
	RegisterTestingT(t)

	mockCtrl := gomock.NewController(t)
	cm := mock.NewMockMicroVMCommandUseCases(mockCtrl)
	qm := mock.NewMockMicroVMQueryUseCases(mockCtrl)

	cm.EXPECT().CreateMicroVM(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, mvm *models.MicroVM) (*models.MicroVM, error) {
			Expect(mvm.ID.IsEmpty()).To(BeTrue())
			Expect(mvm.Spec.SnapshotSource).To(Equal(&models.SnapshotSource{
				Snapshot:   models.SnapshotRef{UID: "snap-uid"},
				Name:       "clone1",
				Namespace:  "ns1",
				MemoryMode: models.MemoryModeFile,
				NetworkOverrides: []models.HostNetworkOverride{
					{GuestDeviceName: "eth1", HostDeviceName: "tap1"},
				},
			}))

			vmid, _ := models.NewVMID("clone1", "ns1", "uid1")

			return &models.MicroVM{
				ID:     *vmid,
				Spec:   models.MicroVMSpec{VCPU: 2, MemoryInMb: 2048},
				Status: models.MicroVMStatus{State: models.PendingState},
			}, nil
		})

	svr := grpc.NewServer(cm, qm, grpc.InfoConfig{})

	resp, err := svr.CreateMicroVM(context.Background(), restoreCreateRequest())
	Expect(err).NotTo(HaveOccurred())
	Expect(resp.Microvm.Spec.Id).To(Equal("clone1"))
	Expect(resp.Microvm.Spec.Namespace).To(Equal("ns1"))
	Expect(resp.Microvm.Spec.GetUid()).To(Equal("uid1"))
}

func TestServer_CreateMicroVM_RestoreErrors(t *testing.T) {
	testCases := []struct {
		name       string
		req        func() *mvm1.CreateMicroVMRequest
		ucErr      error
		expectCode codes.Code
	}{
		{
			name: "spec and snapshot source",
			req: func() *mvm1.CreateMicroVMRequest {
				req := restoreCreateRequest()
				req.Microvm = createTestCreateRequest("mvm1", "default").Microvm

				return req
			},
			expectCode: codes.InvalidArgument,
		},
		{
			name: "no name",
			req: func() *mvm1.CreateMicroVMRequest {
				req := restoreCreateRequest()
				req.SnapshotSource.Name = ""

				return req
			},
			expectCode: codes.InvalidArgument,
		},
		{
			name:       "snapshot not found",
			req:        restoreCreateRequest,
			ucErr:      coreerrs.NewSnapshotNotFound("snap-uid"),
			expectCode: codes.NotFound,
		},
		{
			name:       "incompatible host",
			req:        restoreCreateRequest,
			ucErr:      coreerrs.NewFailedPrecondition("snapshot is not compatible with this host: host_kernel"),
			expectCode: codes.FailedPrecondition,
		},
		{
			name:       "unsupported memory mode",
			req:        restoreCreateRequest,
			ucErr:      coreerrs.NewInvalidArgument("memory mode copy is not supported"),
			expectCode: codes.InvalidArgument,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			RegisterTestingT(t)

			mockCtrl := gomock.NewController(t)
			cm := mock.NewMockMicroVMCommandUseCases(mockCtrl)
			qm := mock.NewMockMicroVMQueryUseCases(mockCtrl)

			if tc.ucErr != nil {
				cm.EXPECT().CreateMicroVM(gomock.Any(), gomock.Any()).Return(nil, tc.ucErr)
			}

			svr := grpc.NewServer(cm, qm, grpc.InfoConfig{})

			_, err := svr.CreateMicroVM(context.Background(), tc.req())
			Expect(err).To(HaveOccurred())
			Expect(status.Code(err)).To(Equal(tc.expectCode), err.Error())
		})
	}
}
