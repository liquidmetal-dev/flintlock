package grpc_test

import (
	"context"
	"errors"
	"testing"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/gomega"
	grpcPkg "google.golang.org/grpc"

	mvm1 "github.com/liquidmetal-dev/flintlock/api/services/microvm/v1alpha1"
	"github.com/liquidmetal-dev/flintlock/api/types"
	"github.com/liquidmetal-dev/flintlock/core/models"
	"github.com/liquidmetal-dev/flintlock/infrastructure/grpc"
	"github.com/liquidmetal-dev/flintlock/infrastructure/mock"
)

func TestServer_CreateMicroVM(t *testing.T) {
	tt := []struct {
		name        string
		createReq   *mvm1.CreateMicroVMRequest
		expectError bool
		expect      func(cm *mock.MockMicroVMCommandUseCasesMockRecorder, qm *mock.MockMicroVMQueryUseCasesMockRecorder)
	}{
		{
			name:        "nil request should fail with error",
			expectError: true,
			expect:      func(cm *mock.MockMicroVMCommandUseCasesMockRecorder, qm *mock.MockMicroVMQueryUseCasesMockRecorder) {},
		},
		{
			name:        "nil spec should fail with error",
			createReq:   &mvm1.CreateMicroVMRequest{},
			expectError: true,
			expect:      func(cm *mock.MockMicroVMCommandUseCasesMockRecorder, qm *mock.MockMicroVMQueryUseCasesMockRecorder) {},
		},
		{
			name:        "missing id should fail with error",
			createReq:   createTestCreateRequest("", ""),
			expectError: true,
			expect:      func(cm *mock.MockMicroVMCommandUseCasesMockRecorder, qm *mock.MockMicroVMQueryUseCasesMockRecorder) {},
		},
		{
			name:        "error from usecase should fail with error",
			createReq:   createTestCreateRequest("mvm1", "default"),
			expectError: true,
			expect: func(cm *mock.MockMicroVMCommandUseCasesMockRecorder, qm *mock.MockMicroVMQueryUseCasesMockRecorder) {
				cm.CreateMicroVM(
					gomock.AssignableToTypeOf(context.Background()),
					gomock.Any(),
				).Return(
					nil,
					errors.New("a random error occurred"),
				)
			},
		},
		{
			name:        "valid spec should not fail",
			createReq:   createTestCreateRequest("mvm1", "default"),
			expectError: false,
			expect: func(cm *mock.MockMicroVMCommandUseCasesMockRecorder, qm *mock.MockMicroVMQueryUseCasesMockRecorder) {
				vmid, _ := models.NewVMID("mvm1", "default", "uid")

				cm.CreateMicroVM(
					gomock.AssignableToTypeOf(context.Background()),
					gomock.Any(),
				).Return(
					&models.MicroVM{
						ID:      *vmid,
						Version: 0,
						Spec:    models.MicroVMSpec{},
						Status: models.MicroVMStatus{
							State: models.CreatedState,
						},
					},
					nil,
				)
			},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			RegisterTestingT(t)

			mockCtrl := gomock.NewController(t)
			cm := mock.NewMockMicroVMCommandUseCases(mockCtrl)
			qm := mock.NewMockMicroVMQueryUseCases(mockCtrl)

			tc.expect(cm.EXPECT(), qm.EXPECT())

			ctx := context.Background()
			svr := grpc.NewServer(cm, qm)
			resp, err := svr.CreateMicroVM(ctx, tc.createReq)

			if tc.expectError {
				Expect(err).To(HaveOccurred())
			} else {
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.Microvm.Spec.Id).To(Equal("mvm1"))
				Expect(resp.Microvm.Spec.Namespace).To(Equal("default"))
			}
		})
	}
}

func TestServer_DeleteMicroVM(t *testing.T) {
	tt := []struct {
		name        string
		deleteReq   *mvm1.DeleteMicroVMRequest
		expectError bool
		expect      func(cm *mock.MockMicroVMCommandUseCasesMockRecorder, qm *mock.MockMicroVMQueryUseCasesMockRecorder)
	}{
		{
			name:        "nil request should fail with error",
			expectError: true,
			expect:      func(cm *mock.MockMicroVMCommandUseCasesMockRecorder, qm *mock.MockMicroVMQueryUseCasesMockRecorder) {},
		},
		{
			name:        "missing id should fail with error",
			deleteReq:   &mvm1.DeleteMicroVMRequest{Uid: ""},
			expectError: true,
			expect:      func(cm *mock.MockMicroVMCommandUseCasesMockRecorder, qm *mock.MockMicroVMQueryUseCasesMockRecorder) {},
		},
		{
			name:        "error from usecase should fail with error",
			deleteReq:   &mvm1.DeleteMicroVMRequest{Uid: "testuid"},
			expectError: true,
			expect: func(cm *mock.MockMicroVMCommandUseCasesMockRecorder, qm *mock.MockMicroVMQueryUseCasesMockRecorder) {
				cm.DeleteMicroVM(
					gomock.AssignableToTypeOf(context.Background()),
					gomock.Not(gomock.Eq("")),
				).Return(
					errors.New("a random error occurred"),
				)
			},
		},
		{
			name:        "valid request and no error from delete microvm usecase should success",
			deleteReq:   &mvm1.DeleteMicroVMRequest{Uid: "testuid"},
			expectError: false,
			expect: func(cm *mock.MockMicroVMCommandUseCasesMockRecorder, qm *mock.MockMicroVMQueryUseCasesMockRecorder) {
				cm.DeleteMicroVM(
					gomock.AssignableToTypeOf(context.Background()),
					gomock.Not(gomock.Eq("")),
				).Return(
					nil,
				)
			},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			RegisterTestingT(t)

			mockCtrl := gomock.NewController(t)
			cm := mock.NewMockMicroVMCommandUseCases(mockCtrl)
			qm := mock.NewMockMicroVMQueryUseCases(mockCtrl)

			tc.expect(cm.EXPECT(), qm.EXPECT())

			ctx := context.Background()
			svr := grpc.NewServer(cm, qm)
			_, err := svr.DeleteMicroVM(ctx, tc.deleteReq)

			if tc.expectError {
				Expect(err).To(HaveOccurred())
			} else {
				Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}

func TestServer_GetMicroVM(t *testing.T) {
	tt := []struct {
		name        string
		getReq      *mvm1.GetMicroVMRequest
		expectError bool
		expect      func(cm *mock.MockMicroVMCommandUseCasesMockRecorder, qm *mock.MockMicroVMQueryUseCasesMockRecorder)
	}{
		{
			name:        "nil request should fail with error",
			expectError: true,
			expect:      func(cm *mock.MockMicroVMCommandUseCasesMockRecorder, qm *mock.MockMicroVMQueryUseCasesMockRecorder) {},
		},
		{
			name:        "missing id should fail with error",
			getReq:      &mvm1.GetMicroVMRequest{Uid: ""},
			expectError: true,
			expect:      func(cm *mock.MockMicroVMCommandUseCasesMockRecorder, qm *mock.MockMicroVMQueryUseCasesMockRecorder) {},
		},
		{
			name:        "error from usecase should fail with error",
			getReq:      &mvm1.GetMicroVMRequest{Uid: "testuid"},
			expectError: true,
			expect: func(cm *mock.MockMicroVMCommandUseCasesMockRecorder, qm *mock.MockMicroVMQueryUseCasesMockRecorder) {
				qm.GetMicroVM(
					gomock.AssignableToTypeOf(context.Background()),
					gomock.Not(gomock.Eq("")),
				).Return(
					nil,
					errors.New("a random error occurred"),
				)
			},
		},
		{
			name:        "valid request with no error should succeed",
			getReq:      &mvm1.GetMicroVMRequest{Uid: "testuid"},
			expectError: false,
			expect: func(cm *mock.MockMicroVMCommandUseCasesMockRecorder, qm *mock.MockMicroVMQueryUseCasesMockRecorder) {
				vmid, _ := models.NewVMID("mvm1", "default", "testuid")

				qm.GetMicroVM(
					gomock.AssignableToTypeOf(context.Background()),
					gomock.Not(gomock.Eq("")),
				).Return(
					&models.MicroVM{
						ID:      *vmid,
						Version: 1,
						Spec:    models.MicroVMSpec{},
						Status: models.MicroVMStatus{
							State: models.CreatedState,
						},
					},
					nil,
				)
			},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			RegisterTestingT(t)

			mockCtrl := gomock.NewController(t)
			cm := mock.NewMockMicroVMCommandUseCases(mockCtrl)
			qm := mock.NewMockMicroVMQueryUseCases(mockCtrl)

			tc.expect(cm.EXPECT(), qm.EXPECT())

			ctx := context.Background()
			svr := grpc.NewServer(cm, qm)
			resp, err := svr.GetMicroVM(ctx, tc.getReq)

			if tc.expectError {
				Expect(err).To(HaveOccurred())
			} else {
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.Microvm.Status.State).To(Equal(types.MicroVMStatus_CREATED))
			}
		})
	}
}

func TestServer_ListMicroVMs(t *testing.T) {
	tt := []struct {
		name        string
		listReq     *mvm1.ListMicroVMsRequest
		expectError bool
		expect      func(cm *mock.MockMicroVMCommandUseCasesMockRecorder, qm *mock.MockMicroVMQueryUseCasesMockRecorder)
	}{
		{
			name:        "nil request should fail with error",
			expectError: true,
			expect:      func(cm *mock.MockMicroVMCommandUseCasesMockRecorder, qm *mock.MockMicroVMQueryUseCasesMockRecorder) {},
		},
		{
			name:        "missing namespace should not fail with error",
			listReq:     &mvm1.ListMicroVMsRequest{Namespace: ""},
			expectError: false,
			expect: func(cm *mock.MockMicroVMCommandUseCasesMockRecorder, qm *mock.MockMicroVMQueryUseCasesMockRecorder) {
				qm.GetAllMicroVM(gomock.AssignableToTypeOf(
					context.Background()),
					gomock.Not(gomock.Eq("")),
				).Return(
					[]*models.MicroVM{
						{
							Version: 1,
							Status: models.MicroVMStatus{
								State: models.CreatedState,
							},
						},
						{
							Version: 1,
							Status: models.MicroVMStatus{
								State: models.CreatedState,
							},
						},
					},
					nil,
				)
			},
		},
		{
			name:        "error from usecase should fail with error",
			listReq:     &mvm1.ListMicroVMsRequest{Namespace: "default"},
			expectError: true,
			expect: func(cm *mock.MockMicroVMCommandUseCasesMockRecorder, qm *mock.MockMicroVMQueryUseCasesMockRecorder) {
				qm.GetAllMicroVM(gomock.AssignableToTypeOf(
					context.Background()),
					gomock.Not(gomock.Eq("")),
				).Return(
					nil,
					errors.New("a random error occurred"),
				)
			},
		},
		{
			name:        "valid request should succeed and return data",
			listReq:     &mvm1.ListMicroVMsRequest{Namespace: "default"},
			expectError: false,
			expect: func(cm *mock.MockMicroVMCommandUseCasesMockRecorder, qm *mock.MockMicroVMQueryUseCasesMockRecorder) {
				qm.GetAllMicroVM(gomock.AssignableToTypeOf(
					context.Background()),
					gomock.Not(gomock.Eq("")),
				).Return(
					[]*models.MicroVM{
						{
							Version: 1,
							Status: models.MicroVMStatus{
								State: models.CreatedState,
							},
						},
						{
							Version: 1,
							Status: models.MicroVMStatus{
								State: models.CreatedState,
							},
						},
					},
					nil,
				)
			},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			RegisterTestingT(t)

			mockCtrl := gomock.NewController(t)
			cm := mock.NewMockMicroVMCommandUseCases(mockCtrl)
			qm := mock.NewMockMicroVMQueryUseCases(mockCtrl)

			tc.expect(cm.EXPECT(), qm.EXPECT())

			ctx := context.Background()
			svr := grpc.NewServer(cm, qm)
			resp, err := svr.ListMicroVMs(ctx, tc.listReq)

			if tc.expectError {
				Expect(err).To(HaveOccurred())
			} else {
				Expect(err).NotTo(HaveOccurred())
				for _, mvm := range resp.Microvm {
					Expect(mvm.Version).To(Equal(int32(1)))
					Expect(mvm.Status.State).To(Equal(types.MicroVMStatus_CREATED))
				}
			}
		})
	}
}

func TestServer_ListMicroVMsStream(t *testing.T) {
	tt := []struct {
		name        string
		listReq     *mvm1.ListMicroVMsRequest
		expectError bool
		expect      func(cm *mock.MockMicroVMCommandUseCasesMockRecorder, qm *mock.MockMicroVMQueryUseCasesMockRecorder)
	}{
		{
			name:        "nil request should fail with error",
			expectError: true,
			expect:      func(cm *mock.MockMicroVMCommandUseCasesMockRecorder, qm *mock.MockMicroVMQueryUseCasesMockRecorder) {},
		},
		{
			name:        "missing namespace should fail with error",
			listReq:     &mvm1.ListMicroVMsRequest{Namespace: ""},
			expectError: true,
			expect:      func(cm *mock.MockMicroVMCommandUseCasesMockRecorder, qm *mock.MockMicroVMQueryUseCasesMockRecorder) {},
		},
		{
			name:        "error from usecase should fail with error",
			listReq:     &mvm1.ListMicroVMsRequest{Namespace: "default"},
			expectError: true,
			expect: func(cm *mock.MockMicroVMCommandUseCasesMockRecorder, qm *mock.MockMicroVMQueryUseCasesMockRecorder) {
				qm.GetAllMicroVM(gomock.AssignableToTypeOf(
					context.Background()),
					gomock.Not(gomock.Eq("")),
				).Return(
					nil,
					errors.New("a random error occurred"),
				)
			},
		},
		{
			name:        "valid request should succeed and return data",
			listReq:     &mvm1.ListMicroVMsRequest{Namespace: "default"},
			expectError: false,
			expect: func(cm *mock.MockMicroVMCommandUseCasesMockRecorder, qm *mock.MockMicroVMQueryUseCasesMockRecorder) {
				qm.GetAllMicroVM(gomock.AssignableToTypeOf(
					context.Background()),
					gomock.Not(gomock.Eq("")),
				).Return(
					[]*models.MicroVM{
						{
							Version: 1,
							Status: models.MicroVMStatus{
								State: models.CreatedState,
							},
						},
						{
							Version: 1,
							Status: models.MicroVMStatus{
								State: models.CreatedState,
							},
						},
					},
					nil,
				)
			},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			RegisterTestingT(t)

			mockCtrl := gomock.NewController(t)
			cm := mock.NewMockMicroVMCommandUseCases(mockCtrl)
			qm := mock.NewMockMicroVMQueryUseCases(mockCtrl)

			ctx := context.Background()
			sendChan := make(chan *mvm1.ListMessage, 10)

			tc.expect(cm.EXPECT(), qm.EXPECT())
			mockStreamServer := makeMockListStream(ctx, sendChan)

			svr := grpc.NewServer(cm, qm)
			err := svr.ListMicroVMsStream(tc.listReq, mockStreamServer)

			close(sendChan)

			if tc.expectError {
				Expect(err).To(HaveOccurred())
			} else {
				Expect(err).NotTo(HaveOccurred())
				for msg := range sendChan {
					Expect(msg.Microvm.Version).To(Equal(int32(1)))
					Expect(msg.Microvm.Status.State).To(Equal(types.MicroVMStatus_CREATED))
				}
			}
		})
	}
}

func createTestCreateRequest(id, namespace string) *mvm1.CreateMicroVMRequest {
	filename := "kernel"
	mac := "AA:FF:00:00:00:01"
	containerSource := "docker.io/library/ubuntu:groovy"
	rootVolSize := int32(20000)

	return &mvm1.CreateMicroVMRequest{
		Microvm: &types.MicroVMSpec{
			Id:         id,
			Namespace:  namespace,
			Vcpu:       2,
			MemoryInMb: 1024,
			Kernel: &types.Kernel{
				Image:    "docker.io/linuxkit/kernel:5.4.129",
				Filename: &filename,
			},
			Interfaces: []*types.NetworkInterface{
				{
					DeviceId: "eth0",
					GuestMac: &mac,
					Type:     types.NetworkInterface_MACVTAP,
				},
				{
					DeviceId: "eth1",
					Type:     types.NetworkInterface_TAP,
				},
			},
			RootVolume: &types.Volume{
				Id:         "root",
				IsReadOnly: false,
				Source: &types.VolumeSource{
					ContainerSource: &containerSource,
				},
				SizeInMb: &rootVolSize,
			},
		},
	}
}

func makeMockListStream(ctx context.Context, sendChan chan *mvm1.ListMessage) *MockListStream {
	return &MockListStream{
		ctx:        ctx,
		serverSend: sendChan,
	}
}

type MockListStream struct {
	grpcPkg.ServerStream
	ctx        context.Context
	serverSend chan *mvm1.ListMessage
}

func (mls *MockListStream) Context() context.Context {
	return mls.ctx
}

func (mls *MockListStream) Send(resp *mvm1.ListMessage) error {
	mls.serverSend <- resp

	return nil
}
