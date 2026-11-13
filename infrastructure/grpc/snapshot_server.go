package grpc

import (
	"context"
	"fmt"
	"maps"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"

	snapv1 "github.com/liquidmetal-dev/flintlock/api/services/snapshot/v1alpha1"
	"github.com/liquidmetal-dev/flintlock/api/types"
	coreerrs "github.com/liquidmetal-dev/flintlock/core/errors"
	"github.com/liquidmetal-dev/flintlock/core/models"
	"github.com/liquidmetal-dev/flintlock/core/ports"
	"github.com/liquidmetal-dev/flintlock/pkg/log"
	"github.com/liquidmetal-dev/flintlock/pkg/validation"
)

// NewSnapshotServer creates a new gRPC server for the snapshot service.
func NewSnapshotServer(
	commandUC ports.SnapshotCommandUseCases,
	queryUC ports.SnapshotQueryUseCases,
) ports.SnapshotGRPCService {
	return &snapshotServer{
		commandUC: commandUC,
		queryUC:   queryUC,
	}
}

type snapshotServer struct {
	commandUC ports.SnapshotCommandUseCases
	queryUC   ports.SnapshotQueryUseCases
}

func (s *snapshotServer) CreateSnapshot(
	ctx context.Context,
	req *snapv1.CreateSnapshotRequest,
) (*snapv1.CreateSnapshotResponse, error) {
	logger := log.GetLogger(ctx)

	if err := validation.ValidateCreateSnapshotRequest(req); err != nil {
		logger.Errorf("invalid create snapshot request: %s", err)

		//nolint:wrapcheck // don't wrap grpc errors when using the status package
		return nil, status.Errorf(codes.InvalidArgument, "invalid create snapshot request: %s", err)
	}

	logger.Infof("creating snapshot of microvm %s", req.GetSourceUid())

	snapshot, err := s.commandUC.CreateSnapshot(ctx, models.CreateSnapshotRequest{
		SourceUID: req.GetSourceUid(),
		Name:      req.GetName(),
		Labels:    maps.Clone(req.GetLabels()),
		After:     convertAfterToModel(req.GetAfter()),
	})
	if err != nil {
		logger.Errorf("failed to create snapshot: %s", err)

		return nil, snapshotError("creating snapshot", err)
	}

	return &snapv1.CreateSnapshotResponse{Snapshot: convertModelToSnapshot(snapshot)}, nil
}

func (s *snapshotServer) GetSnapshot(
	ctx context.Context,
	req *snapv1.GetSnapshotRequest,
) (*snapv1.GetSnapshotResponse, error) {
	logger := log.GetLogger(ctx)

	if err := validation.ValidateSnapshotRef(req.GetRef()); err != nil {
		logger.Errorf("invalid get snapshot request: %s", err)

		//nolint:wrapcheck // don't wrap grpc errors when using the status package
		return nil, status.Errorf(codes.InvalidArgument, "invalid get snapshot request: %s", err)
	}

	snapshot, err := s.queryUC.GetSnapshot(ctx, convertSnapshotRefToModel(req.GetRef()))
	if err != nil {
		logger.Errorf("failed to get snapshot: %s", err)

		return nil, snapshotError("getting snapshot", err)
	}

	return &snapv1.GetSnapshotResponse{Snapshot: convertModelToSnapshot(snapshot)}, nil
}

func (s *snapshotServer) ListSnapshots(
	ctx context.Context,
	req *snapv1.ListSnapshotsRequest,
) (*snapv1.ListSnapshotsResponse, error) {
	logger := log.GetLogger(ctx)

	if req == nil {
		logger.Error("invalid list snapshots request")

		//nolint:wrapcheck // don't wrap grpc errors when using the status package
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}

	snapshots, err := s.queryUC.GetAllSnapshots(ctx, convertListSnapshotsRequest(req))
	if err != nil {
		logger.Errorf("failed to list snapshots: %s", err)

		return nil, snapshotError("listing snapshots", err)
	}

	resp := &snapv1.ListSnapshotsResponse{Snapshots: []*types.Snapshot{}}

	for _, snapshot := range snapshots {
		resp.Snapshots = append(resp.Snapshots, convertModelToSnapshot(snapshot))
	}

	return resp, nil
}

func (s *snapshotServer) ListSnapshotsStream(
	req *snapv1.ListSnapshotsRequest,
	streamServer snapv1.SnapshotService_ListSnapshotsStreamServer,
) error {
	ctx := streamServer.Context()
	logger := log.GetLogger(ctx)

	if req == nil {
		logger.Error("invalid list snapshots request")

		//nolint:wrapcheck // don't wrap grpc errors when using the status package
		return status.Error(codes.InvalidArgument, "invalid request")
	}

	snapshots, err := s.queryUC.GetAllSnapshots(ctx, convertListSnapshotsRequest(req))
	if err != nil {
		logger.Errorf("failed to list snapshots: %s", err)

		return snapshotError("listing snapshots", err)
	}

	for _, snapshot := range snapshots {
		if err := streamServer.Send(&snapv1.ListSnapshotsMessage{Snapshot: convertModelToSnapshot(snapshot)}); err != nil {
			logger.Errorf("failed to stream response to client: %s", err)

			return fmt.Errorf("streaming response to client: %w", err)
		}
	}

	return nil
}

func (s *snapshotServer) DeleteSnapshot(
	ctx context.Context,
	req *snapv1.DeleteSnapshotRequest,
) (*emptypb.Empty, error) {
	logger := log.GetLogger(ctx)

	if err := validation.ValidateSnapshotRef(req.GetRef()); err != nil {
		logger.Errorf("invalid delete snapshot request: %s", err)

		//nolint:wrapcheck // don't wrap grpc errors when using the status package
		return nil, status.Errorf(codes.InvalidArgument, "invalid delete snapshot request: %s", err)
	}

	if err := s.commandUC.DeleteSnapshot(ctx, convertSnapshotRefToModel(req.GetRef())); err != nil {
		logger.Errorf("failed to delete snapshot: %s", err)

		return nil, snapshotError("deleting snapshot", err)
	}

	return &emptypb.Empty{}, nil
}

// snapshotError gives an error of the snapshot use cases the gRPC code of its kind. Any
// other error is wrapped, as the microvm server wraps the errors of its use cases.
func snapshotError(action string, err error) error {
	var code codes.Code

	switch {
	case coreerrs.IsSnapshotNotFound(err), coreerrs.IsSpecNotFound(err):
		code = codes.NotFound
	case coreerrs.IsSnapshotAlreadyExists(err):
		code = codes.AlreadyExists
	case coreerrs.IsFailedPrecondition(err):
		code = codes.FailedPrecondition
	default:
		return fmt.Errorf("%s: %w", action, err)
	}

	//nolint:wrapcheck // don't wrap grpc errors when using the status package
	return status.Error(code, err.Error())
}

func convertListSnapshotsRequest(req *snapv1.ListSnapshotsRequest) ports.SnapshotListOptions {
	return ports.SnapshotListOptions{
		Namespace: req.GetNamespace(),
		SourceUID: req.GetSourceUid(),
		Labels:    maps.Clone(req.GetLabels()),
	}
}

func convertAfterToModel(after snapv1.After) models.SnapshotAfter {
	switch after {
	case snapv1.After_STOP:
		return models.SnapshotAfterStop
	case snapv1.After_RESUME:
	}

	return models.SnapshotAfterResume
}
