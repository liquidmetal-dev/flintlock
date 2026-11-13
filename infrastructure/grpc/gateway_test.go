package grpc

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	g "github.com/onsi/gomega"
	"google.golang.org/protobuf/proto"

	mvmv1 "github.com/liquidmetal-dev/flintlock/api/services/microvm/v1alpha1"
	snapshotv1 "github.com/liquidmetal-dev/flintlock/api/services/snapshot/v1alpha1"
	"github.com/liquidmetal-dev/flintlock/api/types"
)

type recordingMicroVMServer struct {
	mvmv1.MicroVMServer

	got *mvmv1.CreateMicroVMRequest
}

func (s *recordingMicroVMServer) CreateMicroVM(
	_ context.Context,
	req *mvmv1.CreateMicroVMRequest,
) (*mvmv1.CreateMicroVMResponse, error) {
	s.got = req

	return &mvmv1.CreateMicroVMResponse{}, nil
}

type recordingSnapshotServer struct {
	snapshotv1.SnapshotServiceServer

	got *snapshotv1.GetSnapshotRequest
}

func (s *recordingSnapshotServer) GetSnapshot(
	_ context.Context,
	req *snapshotv1.GetSnapshotRequest,
) (*snapshotv1.GetSnapshotResponse, error) {
	s.got = req

	return &snapshotv1.GetSnapshotResponse{}, nil
}

func TestGateway_GetSnapshot(t *testing.T) {
	tt := []struct {
		name    string
		path    string
		wantRef *types.SnapshotRef
	}{
		{
			name:    "by uid",
			path:    "/v1alpha1/snapshot/snap-uid",
			wantRef: &types.SnapshotRef{Ref: &types.SnapshotRef_Uid{Uid: "snap-uid"}},
		},
		{
			name: "by namespace and name",
			path: "/v1alpha1/snapshot/ns/nightly",
			wantRef: &types.SnapshotRef{
				Ref: &types.SnapshotRef_Named{Named: &types.NamedRef{Name: "nightly", Namespace: "ns"}},
			},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			g.RegisterTestingT(t)

			srv := &recordingSnapshotServer{}
			mux := runtime.NewServeMux()
			g.Expect(snapshotv1.RegisterSnapshotServiceHandlerServer(context.Background(), mux, srv)).To(g.Succeed())

			req := httptest.NewRequest(http.MethodGet, tc.path, nil)
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)

			g.Expect(rec.Code).To(g.Equal(http.StatusOK), rec.Body.String())
			g.Expect(srv.got).NotTo(g.BeNil())
			g.Expect(proto.Equal(srv.got.GetRef(), tc.wantRef)).To(g.BeTrue(), "got ref %v", srv.got.GetRef())
		})
	}
}

func TestGateway_CreateMicroVM(t *testing.T) {
	tt := []struct {
		name   string
		path   string
		body   string
		expect func(req *mvmv1.CreateMicroVMRequest)
	}{
		{
			name: "spec body",
			path: "/v1alpha1/microvm",
			body: `{"id":"vm","namespace":"ns","vcpu":2}`,
			expect: func(req *mvmv1.CreateMicroVMRequest) {
				g.Expect(req.GetSnapshotSource()).To(g.BeNil())
				g.Expect(req.GetMicrovm().GetId()).To(g.Equal("vm"))
				g.Expect(req.GetMicrovm().GetNamespace()).To(g.Equal("ns"))
				g.Expect(req.GetMicrovm().GetVcpu()).To(g.Equal(int32(2)))
			},
		},
		{
			name: "snapshot source body",
			path: "/v1alpha1/microvm/restore",
			body: `{"snapshot":{"uid":"snap-uid"},"name":"restored","memory_mode":"COPY"}`,
			expect: func(req *mvmv1.CreateMicroVMRequest) {
				g.Expect(req.GetMicrovm()).To(g.BeNil())
				g.Expect(req.GetSnapshotSource().GetSnapshot().GetUid()).To(g.Equal("snap-uid"))
				g.Expect(req.GetSnapshotSource().GetName()).To(g.Equal("restored"))
				g.Expect(req.GetSnapshotSource().GetMemoryMode().String()).To(g.Equal("COPY"))
			},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			g.RegisterTestingT(t)

			srv := &recordingMicroVMServer{}
			mux := runtime.NewServeMux()
			g.Expect(mvmv1.RegisterMicroVMHandlerServer(context.Background(), mux, srv)).To(g.Succeed())

			req := httptest.NewRequest(http.MethodPost, tc.path, strings.NewReader(tc.body))
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)

			g.Expect(rec.Code).To(g.Equal(http.StatusOK), rec.Body.String())
			g.Expect(srv.got).NotTo(g.BeNil())
			tc.expect(srv.got)
		})
	}
}
