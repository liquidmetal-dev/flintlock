package grpc_test

import (
	"context"
	"errors"
	"testing"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/gomega"
	grpcPkg "google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	snapv1 "github.com/liquidmetal-dev/flintlock/api/services/snapshot/v1alpha1"
	"github.com/liquidmetal-dev/flintlock/api/types"
	coreerrs "github.com/liquidmetal-dev/flintlock/core/errors"
	"github.com/liquidmetal-dev/flintlock/core/models"
	"github.com/liquidmetal-dev/flintlock/core/ports"
	"github.com/liquidmetal-dev/flintlock/infrastructure/grpc"
	"github.com/liquidmetal-dev/flintlock/infrastructure/mock"
	"github.com/liquidmetal-dev/flintlock/pkg/ptr"
)

const (
	testSnapshotUID  = "8f5b2a4e-2b8c-4f4e-9d61-0b6f2f0a7c11"
	testSnapshotVMID = "34b8fc8e-6246-11ec-90d6-0242ac120003"
)

func newSnapshotTestServer(t *testing.T) (
	ports.SnapshotGRPCService,
	*mock.MockSnapshotCommandUseCases,
	*mock.MockSnapshotQueryUseCases,
) {
	t.Helper()

	mockCtrl := gomock.NewController(t)
	commandUC := mock.NewMockSnapshotCommandUseCases(mockCtrl)
	queryUC := mock.NewMockSnapshotQueryUseCases(mockCtrl)

	return grpc.NewSnapshotServer(commandUC, queryUC), commandUC, queryUC
}

func testSnapshot() *models.Snapshot {
	vmid, _ := models.NewVMID("mvm1", "ns1", testSnapshotVMID)

	return &models.Snapshot{
		UID:       testSnapshotUID,
		Name:      "snap1",
		Namespace: "ns1",
		Source:    *vmid,
		Labels:    map[string]string{"env": "test"},
		Status:    models.SnapshotStatus{Phase: models.SnapshotPhasePending},
	}
}

func TestSnapshotServer_CreateSnapshot(t *testing.T) {
	testCases := []struct {
		name       string
		req        *snapv1.CreateSnapshotRequest
		ucErr      error
		callsUC    bool
		expectCode codes.Code
	}{
		{name: "nil request", expectCode: codes.InvalidArgument},
		{name: "no source uid", req: &snapv1.CreateSnapshotRequest{}, expectCode: codes.InvalidArgument},
		{
			name:       "empty name",
			req:        &snapv1.CreateSnapshotRequest{SourceUid: testSnapshotVMID, Name: ptr.String("")},
			expectCode: codes.InvalidArgument,
		},
		{
			name:       "unknown after",
			req:        &snapv1.CreateSnapshotRequest{SourceUid: testSnapshotVMID, After: snapv1.After(7)},
			expectCode: codes.InvalidArgument,
		},
		{
			name:       "source not found",
			req:        &snapv1.CreateSnapshotRequest{SourceUid: testSnapshotVMID},
			ucErr:      coreerrs.NewSpecNotFound("", "", "", testSnapshotVMID),
			callsUC:    true,
			expectCode: codes.NotFound,
		},
		{
			name:       "failed precondition",
			req:        &snapv1.CreateSnapshotRequest{SourceUid: testSnapshotVMID},
			ucErr:      coreerrs.NewFailedPrecondition("microvm is being deleted"),
			callsUC:    true,
			expectCode: codes.FailedPrecondition,
		},
		{
			name:       "name taken",
			req:        &snapv1.CreateSnapshotRequest{SourceUid: testSnapshotVMID, Name: ptr.String("snap1")},
			ucErr:      coreerrs.NewSnapshotAlreadyExists("ns1/snap1"),
			callsUC:    true,
			expectCode: codes.AlreadyExists,
		},
		{
			name:       "other error",
			req:        &snapv1.CreateSnapshotRequest{SourceUid: testSnapshotVMID},
			ucErr:      errors.New("disk full"),
			callsUC:    true,
			expectCode: codes.Unknown,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			RegisterTestingT(t)

			srv, commandUC, _ := newSnapshotTestServer(t)

			if tc.callsUC {
				commandUC.EXPECT().CreateSnapshot(gomock.Any(), gomock.Any()).Return(nil, tc.ucErr)
			}

			resp, err := srv.CreateSnapshot(context.Background(), tc.req)

			Expect(err).To(HaveOccurred())
			Expect(resp).To(BeNil())
			Expect(status.Code(err)).To(Equal(tc.expectCode))
		})
	}
}

func TestSnapshotServer_CreateSnapshot_Valid(t *testing.T) {
	RegisterTestingT(t)

	srv, commandUC, _ := newSnapshotTestServer(t)

	commandUC.EXPECT().
		CreateSnapshot(gomock.Any(), models.CreateSnapshotRequest{
			SourceUID: testSnapshotVMID,
			Name:      "snap1",
			Labels:    map[string]string{"env": "test"},
			After:     models.SnapshotAfterResume,
		}).
		Return(testSnapshot(), nil)

	resp, err := srv.CreateSnapshot(context.Background(), &snapv1.CreateSnapshotRequest{
		SourceUid: testSnapshotVMID,
		Name:      ptr.String("snap1"),
		Labels:    map[string]string{"env": "test"},
	})

	Expect(err).NotTo(HaveOccurred())
	Expect(resp.GetSnapshot().GetUid()).To(Equal(testSnapshotUID))
	Expect(resp.GetSnapshot().GetSource().GetUid()).To(Equal(testSnapshotVMID))
	Expect(resp.GetSnapshot().GetStatus().GetPhase()).To(Equal(types.SnapshotStatus_PENDING))
}

func TestSnapshotServer_CreateSnapshot_AfterStopReachesUseCase(t *testing.T) {
	RegisterTestingT(t)

	srv, commandUC, _ := newSnapshotTestServer(t)

	commandUC.EXPECT().
		CreateSnapshot(gomock.Any(), models.CreateSnapshotRequest{
			SourceUID: testSnapshotVMID,
			After:     models.SnapshotAfterStop,
		}).
		Return(nil, coreerrs.NewFailedPrecondition("after stop is not supported"))

	_, err := srv.CreateSnapshot(context.Background(), &snapv1.CreateSnapshotRequest{
		SourceUid: testSnapshotVMID,
		After:     snapv1.After_STOP,
	})

	Expect(status.Code(err)).To(Equal(codes.FailedPrecondition))
}

func TestSnapshotServer_GetSnapshot(t *testing.T) {
	testCases := []struct {
		name       string
		req        *snapv1.GetSnapshotRequest
		ref        models.SnapshotRef
		ucErr      error
		callsUC    bool
		expectCode codes.Code
	}{
		{name: "nil request", expectCode: codes.InvalidArgument},
		{name: "no ref", req: &snapv1.GetSnapshotRequest{}, expectCode: codes.InvalidArgument},
		{
			name: "named ref without namespace",
			req: &snapv1.GetSnapshotRequest{Ref: &types.SnapshotRef{
				Ref: &types.SnapshotRef_Named{Named: &types.NamedRef{Name: "snap1"}},
			}},
			expectCode: codes.InvalidArgument,
		},
		{
			name: "not found",
			req: &snapv1.GetSnapshotRequest{Ref: &types.SnapshotRef{
				Ref: &types.SnapshotRef_Uid{Uid: testSnapshotUID},
			}},
			ref:        models.SnapshotRef{UID: testSnapshotUID},
			ucErr:      coreerrs.NewSnapshotNotFound(testSnapshotUID),
			callsUC:    true,
			expectCode: codes.NotFound,
		},
		{
			name: "found by name",
			req: &snapv1.GetSnapshotRequest{Ref: &types.SnapshotRef{
				Ref: &types.SnapshotRef_Named{Named: &types.NamedRef{Name: "snap1", Namespace: "ns1"}},
			}},
			ref:        models.SnapshotRef{Name: "snap1", Namespace: "ns1"},
			callsUC:    true,
			expectCode: codes.OK,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			RegisterTestingT(t)

			srv, _, queryUC := newSnapshotTestServer(t)

			if tc.callsUC {
				var found *models.Snapshot
				if tc.ucErr == nil {
					found = testSnapshot()
				}

				queryUC.EXPECT().GetSnapshot(gomock.Any(), tc.ref).Return(found, tc.ucErr)
			}

			resp, err := srv.GetSnapshot(context.Background(), tc.req)

			Expect(status.Code(err)).To(Equal(tc.expectCode))

			if tc.expectCode == codes.OK {
				Expect(resp.GetSnapshot().GetName()).To(Equal("snap1"))
			}
		})
	}
}

func TestSnapshotServer_ListSnapshots(t *testing.T) {
	RegisterTestingT(t)

	srv, _, queryUC := newSnapshotTestServer(t)

	queryUC.EXPECT().
		GetAllSnapshots(gomock.Any(), ports.SnapshotListOptions{
			Namespace: "ns1",
			SourceUID: testSnapshotVMID,
			Labels:    map[string]string{"env": "test"},
		}).
		Return([]*models.Snapshot{testSnapshot()}, nil)

	resp, err := srv.ListSnapshots(context.Background(), &snapv1.ListSnapshotsRequest{
		Namespace: "ns1",
		SourceUid: ptr.String(testSnapshotVMID),
		Labels:    map[string]string{"env": "test"},
	})

	Expect(err).NotTo(HaveOccurred())
	Expect(resp.GetSnapshots()).To(HaveLen(1))
	Expect(resp.GetSnapshots()[0].GetUid()).To(Equal(testSnapshotUID))

	_, err = srv.ListSnapshots(context.Background(), nil)
	Expect(status.Code(err)).To(Equal(codes.InvalidArgument))
}

func TestSnapshotServer_ListSnapshotsStream(t *testing.T) {
	RegisterTestingT(t)

	srv, _, queryUC := newSnapshotTestServer(t)

	queryUC.EXPECT().
		GetAllSnapshots(gomock.Any(), ports.SnapshotListOptions{Namespace: "ns1"}).
		Return([]*models.Snapshot{testSnapshot(), testSnapshot()}, nil)

	stream := &snapshotListStream{ctx: context.Background()}

	Expect(srv.ListSnapshotsStream(&snapv1.ListSnapshotsRequest{Namespace: "ns1"}, stream)).To(Succeed())
	Expect(stream.sent).To(HaveLen(2))
	Expect(stream.sent[0].GetSnapshot().GetUid()).To(Equal(testSnapshotUID))
}

func TestSnapshotServer_DeleteSnapshot(t *testing.T) {
	uidRef := &types.SnapshotRef{Ref: &types.SnapshotRef_Uid{Uid: testSnapshotUID}}

	testCases := []struct {
		name       string
		req        *snapv1.DeleteSnapshotRequest
		ucErr      error
		callsUC    bool
		expectCode codes.Code
	}{
		{name: "nil request", expectCode: codes.InvalidArgument},
		{
			name:       "empty uid",
			req:        &snapv1.DeleteSnapshotRequest{Ref: &types.SnapshotRef{Ref: &types.SnapshotRef_Uid{}}},
			expectCode: codes.InvalidArgument,
		},
		{
			name:       "capturing",
			req:        &snapv1.DeleteSnapshotRequest{Ref: uidRef},
			ucErr:      coreerrs.NewFailedPrecondition("snapshot is capturing"),
			callsUC:    true,
			expectCode: codes.FailedPrecondition,
		},
		{
			name:       "not found",
			req:        &snapv1.DeleteSnapshotRequest{Ref: uidRef},
			ucErr:      coreerrs.NewSnapshotNotFound(testSnapshotUID),
			callsUC:    true,
			expectCode: codes.NotFound,
		},
		{
			name:       "deleted",
			req:        &snapv1.DeleteSnapshotRequest{Ref: uidRef},
			callsUC:    true,
			expectCode: codes.OK,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			RegisterTestingT(t)

			srv, commandUC, _ := newSnapshotTestServer(t)

			if tc.callsUC {
				commandUC.EXPECT().
					DeleteSnapshot(gomock.Any(), models.SnapshotRef{UID: testSnapshotUID}).
					Return(tc.ucErr)
			}

			_, err := srv.DeleteSnapshot(context.Background(), tc.req)

			Expect(status.Code(err)).To(Equal(tc.expectCode))
		})
	}
}

type snapshotListStream struct {
	grpcPkg.ServerStream

	ctx  context.Context
	sent []*snapv1.ListSnapshotsMessage
}

func (s *snapshotListStream) Context() context.Context {
	return s.ctx
}

func (s *snapshotListStream) Send(msg *snapv1.ListSnapshotsMessage) error {
	s.sent = append(s.sent, msg)

	return nil
}
