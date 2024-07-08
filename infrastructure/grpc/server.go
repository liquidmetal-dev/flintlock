package grpc

import (
	"context"
	"fmt"

	mvmv1 "github.com/liquidmetal-dev/flintlock/api/services/microvm/v1alpha1"
	"github.com/liquidmetal-dev/flintlock/api/types"
	"github.com/liquidmetal-dev/flintlock/core/models"
	"github.com/liquidmetal-dev/flintlock/core/ports"
	"github.com/liquidmetal-dev/flintlock/pkg/log"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

// NewServer creates a new server instance.
func NewServer(commandUC ports.MicroVMCommandUseCases, queryUC ports.MicroVMQueryUseCases) ports.MicroVMGRPCService {
	return &server{
		commandUC: commandUC,
		queryUC:   queryUC,
	}
}

type server struct {
	commandUC ports.MicroVMCommandUseCases
	queryUC   ports.MicroVMQueryUseCases
}

func (s *server) CreateMicroVM(
	ctx context.Context,
	req *mvmv1.CreateMicroVMRequest,
) (*mvmv1.CreateMicroVMResponse, error) {
	logger := log.GetLogger(ctx)
	logger.Trace("converting request to model")

	if req == nil || req.Microvm == nil {
		logger.Error("invalid create microvm request: MicroVMSpec required")

		//nolint:wrapcheck // don't wrap grpc errors when using the status package
		return nil, status.Error(codes.InvalidArgument, "invalid create microvm request: MicroVMSpec required")
	}

	modelSpec, err := convertMicroVMToModel(req.Microvm)
	if err != nil {
		return nil, fmt.Errorf("converting request: %w", err)
	}

	logger.Infof("creating microvm %s", modelSpec.ID)

	createdModel, err := s.commandUC.CreateMicroVM(ctx, modelSpec)
	if err != nil {
		logger.Errorf("failed to create microvm: %s", err)

		return nil, fmt.Errorf("creating microvm: %w", err)
	}

	logger.Trace("converting model to response")

	resp := &mvmv1.CreateMicroVMResponse{
		Microvm: &types.MicroVM{
			Version: int32(createdModel.Version),
			Spec:    convertModelToMicroVMSpec(createdModel),
			Status:  convertModelToMicroVMStatus(createdModel),
		},
	}

	return resp, nil
}

func (s *server) DeleteMicroVM(ctx context.Context, req *mvmv1.DeleteMicroVMRequest) (*emptypb.Empty, error) {
	logger := log.GetLogger(ctx)

	if req == nil || req.Uid == "" {
		logger.Error("invalid delete microvm request")

		//nolint:wrapcheck // don't wrap grpc errors when using the status package
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}

	logger.Infof("deleting microvm %s", req.Uid)

	if err := s.commandUC.DeleteMicroVM(ctx, req.Uid); err != nil {
		logger.Errorf("failed to delete microvm: %s", err)

		return nil, fmt.Errorf("deleting microvm: %w", err)
	}

	return &emptypb.Empty{}, nil
}

func (s *server) GetMicroVM(ctx context.Context, req *mvmv1.GetMicroVMRequest) (*mvmv1.GetMicroVMResponse, error) {
	logger := log.GetLogger(ctx)

	if req == nil || req.Uid == "" {
		logger.Error("invalid get microvm request")

		//nolint:wrapcheck // don't wrap grpc errors when using the status package
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}

	logger.Infof("getting microvm %s", req.Uid)

	foundMicrovm, err := s.queryUC.GetMicroVM(ctx, req.Uid)
	if err != nil {
		logger.Errorf("failed to get microvm: %s", err)

		return nil, fmt.Errorf("getting microvm: %w", err)
	}

	logger.Trace("converting model to response")

	resp := &mvmv1.GetMicroVMResponse{
		Microvm: &types.MicroVM{
			Version: int32(foundMicrovm.Version),
			Spec:    convertModelToMicroVMSpec(foundMicrovm),
			Status:  convertModelToMicroVMStatus(foundMicrovm),
		},
	}

	return resp, nil
}

func (s *server) ListMicroVMs(ctx context.Context,
	req *mvmv1.ListMicroVMsRequest,
) (*mvmv1.ListMicroVMsResponse, error) {
	logger := log.GetLogger(ctx)

	if req == nil {
		logger.Error("invalid get microvm request")

		//nolint:wrapcheck // don't wrap grpc errors when using the status package
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}

	logger.Infof("getting all microvms in %s", req.Namespace)

	query := models.ListMicroVMQuery{"namespace": req.Namespace}

	if req.Name != nil {
		query["name"] = *req.Name
	}

	foundMicrovms, err := s.queryUC.GetAllMicroVM(ctx, query)
	if err != nil {
		logger.Errorf("failed to getting all microvm: %s", err)

		return nil, fmt.Errorf("getting all microvms: %w", err)
	}

	logger.Trace("converting model to response")

	resp := &mvmv1.ListMicroVMsResponse{
		Microvm: []*types.MicroVM{},
	}

	for _, mvm := range foundMicrovms {
		converted := &types.MicroVM{
			Version: int32(mvm.Version),
			Spec:    convertModelToMicroVMSpec(mvm),
			Status:  convertModelToMicroVMStatus(mvm),
		}
		resp.Microvm = append(resp.Microvm, converted)
	}

	return resp, nil
}

func (s *server) ListMicroVMsStream(
	req *mvmv1.ListMicroVMsRequest,
	streamServer mvmv1.MicroVM_ListMicroVMsStreamServer,
) error {
	ctx := streamServer.Context()
	logger := log.GetLogger(ctx)

	if req == nil || req.Namespace == "" {
		logger.Error("invalid get microvm request")

		//nolint:wrapcheck // don't wrap grpc errors when using the status package
		return status.Error(codes.InvalidArgument, "invalid request")
	}

	logger.Infof("getting all microvms in %s", req.Namespace)

	foundMicrovms, err := s.queryUC.GetAllMicroVM(
		ctx,
		models.ListMicroVMQuery{"namespace": req.Namespace},
	)
	if err != nil {
		logger.Errorf("failed to getting all microvm: %s", err)

		return fmt.Errorf("getting all microvms: %w", err)
	}

	logger.Info("streaming found microvm results")

	for _, mvm := range foundMicrovms {
		resp := &mvmv1.ListMessage{
			Microvm: &types.MicroVM{
				Version: int32(mvm.Version),
				Spec:    convertModelToMicroVMSpec(mvm),
				Status:  convertModelToMicroVMStatus(mvm),
			},
		}

		if err := streamServer.Send(resp); err != nil {
			logger.Errorf("failed to stream response to client: %s", err)

			return fmt.Errorf("streaming response to client: %w", err)
		}
	}

	return nil
}
