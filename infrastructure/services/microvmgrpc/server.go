package microvmgrpc

import (
	"context"
	"fmt"

	"google.golang.org/protobuf/types/known/emptypb"

	mvmv1 "github.com/weaveworks/reignite/api/services/microvm/v1alpha1"
	"github.com/weaveworks/reignite/api/types"
	"github.com/weaveworks/reignite/core/ports"
	"github.com/weaveworks/reignite/pkg/log"
)

// NewServer creates a new server instance.
// NOTE: this is an unimplemented server at present.
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

func (s *server) CreateMicroVM(ctx context.Context, req *mvmv1.CreateMicroVMRequest) (*mvmv1.CreateMicroVMResponse, error) {
	logger := log.GetLogger(ctx)

	logger.Trace("converting request to model")
	modelSpec := convertMicroVMToModel(req.Microvm)

	logger.Infof("creating microvm %s/%s", modelSpec.ID, modelSpec.Namespace)
	createdModel, err := s.commandUC.CreateMicroVM(ctx, modelSpec)
	if err != nil {
		logger.Errorf("failed to create microvm: %s", err)

		return nil, fmt.Errorf("creating microvm: %w", err)
	}

	logger.Trace("converting model to response")
	resp := &mvmv1.CreateMicroVMResponse{
		Microvm: convertModelToMicroVM(createdModel),
	}

	return resp, nil
}

func (s *server) UpdateMicroVM(ctx context.Context, req *mvmv1.UpdateMicroVMRequest) (*mvmv1.UpdateMicroVMResponse, error) {
	logger := log.GetLogger(ctx)

	logger.Trace("converting request to model")
	modelSpec := convertMicroVMToModel(req.Microvm)

	logger.Infof("updating microvm %s/%s", modelSpec.ID, modelSpec.Namespace)
	updatedModel, err := s.commandUC.UpdateMicroVM(ctx, modelSpec)
	if err != nil {
		logger.Errorf("failed to update microvm: %s", err)

		return nil, fmt.Errorf("updating microvm: %w", err)
	}

	logger.Trace("converting model to response")
	resp := &mvmv1.UpdateMicroVMResponse{
		Microvm: convertModelToMicroVM(updatedModel),
	}

	return resp, nil
}

func (s *server) DeleteMicroVM(ctx context.Context, req *mvmv1.DeleteMicroVMRequest) (*emptypb.Empty, error) {
	logger := log.GetLogger(ctx)

	logger.Infof("deleting microvm %s/%s", req.Id, req.Namespace)
	err := s.commandUC.DeleteMicroVM(ctx, req.Id, req.Namespace)
	if err != nil {
		logger.Errorf("failed to delete microvm: %s", err)

		return nil, fmt.Errorf("deleting microvm: %w", err)
	}

	return &emptypb.Empty{}, nil
}

func (s *server) GetMicroVM(ctx context.Context, req *mvmv1.GetMicroVMRequest) (*mvmv1.GetMicroVMResponse, error) {
	logger := log.GetLogger(ctx)

	logger.Infof("getting microvm %s/%s", req.Namespace, req.Id)
	foundMicrovm, err := s.queryUC.GetMicroVM(ctx, req.Id, req.Namespace)
	if err != nil {
		logger.Errorf("failed to get microvm: %s", err)

		return nil, fmt.Errorf("getting microvm: %w", err)
	}

	logger.Trace("converting model to response")
	resp := &mvmv1.GetMicroVMResponse{
		Microvm: convertModelToMicroVM(foundMicrovm),
	}

	return resp, nil
}

func (s *server) ListMicroVMs(ctx context.Context, req *mvmv1.ListMicroVMsRequest) (*mvmv1.ListMicroVMsResponse, error) {
	logger := log.GetLogger(ctx)

	logger.Infof("getting all microvms in %s", req.Namespace)
	foundMicrovms, err := s.queryUC.GetAllMicroVM(ctx, req.Namespace)
	if err != nil {
		logger.Errorf("failed to getting all microvm: %s", err)

		return nil, fmt.Errorf("getting all microvms: %w", err)
	}

	logger.Trace("converting model to response")
	resp := &mvmv1.ListMicroVMsResponse{
		Microvm: []*types.MicroVMSpec{},
	}

	for _, mvm := range foundMicrovms {
		converted := convertModelToMicroVM(mvm)
		resp.Microvm = append(resp.Microvm, converted)
	}

	return resp, nil
}

func (s *server) ListMicroVMsStream(req *mvmv1.ListMicroVMsRequest, ss mvmv1.MicroVM_ListMicroVMsStreamServer) error {
	ctx := ss.Context()
	logger := log.GetLogger(ctx)

	logger.Infof("getting all microvms in %s", req.Namespace)
	foundMicrovms, err := s.queryUC.GetAllMicroVM(ctx, req.Namespace)
	if err != nil {
		logger.Errorf("failed to getting all microvm: %s", err)

		return fmt.Errorf("getting all microvms: %w", err)
	}

	logger.Info("streaming found microvm results")
	for _, mvm := range foundMicrovms {
		resp := &mvmv1.ListMessage{
			Microvm: convertModelToMicroVM(mvm),
		}

		if err := ss.Send(resp); err != nil {
			logger.Errorf("failed to stream response to client: %s", err)

			return fmt.Errorf("streaming response to client: %w", err)
		}
	}

	return nil
}
