package grpc

import (
	"context"
	"errors"
	"fmt"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/go-playground/validator/v10"
	mvmv1 "github.com/weaveworks/flintlock/api/services/microvm/v1alpha1"
	"github.com/weaveworks/flintlock/api/types"
	"github.com/weaveworks/flintlock/core/ports"
	"github.com/weaveworks/flintlock/pkg/log"
	"github.com/weaveworks/flintlock/pkg/validation"
)

// NewServer creates a new server instance.
// NOTE: this is an unimplemented server at present.
func NewServer(commandUC ports.MicroVMCommandUseCases, queryUC ports.MicroVMQueryUseCases) ports.MicroVMGRPCService {
	return &server{
		commandUC: commandUC,
		queryUC:   queryUC,
		validator: validation.NewValidator(),
	}
}

type server struct {
	commandUC ports.MicroVMCommandUseCases
	queryUC   ports.MicroVMQueryUseCases
	validator validation.Validator
}

//nolint:dupl
func (s *server) CreateMicroVM(ctx context.Context, req *mvmv1.CreateMicroVMRequest) (*mvmv1.CreateMicroVMResponse, error) {
	logger := log.GetLogger(ctx)

	logger.Trace("converting request to model")
	modelSpec, err := convertMicroVMToModel(req.Microvm)
	if err != nil {
		return nil, fmt.Errorf("converting request: %w", err)
	}

	logger.Trace("validating model")
	err = s.validator.ValidateStruct(modelSpec)
	var valErrors validator.ValidationErrors
	if err != nil {
		if errors.As(err, &valErrors) {
			return nil, status.Errorf(codes.InvalidArgument, "an error occurred when attempting to validate the request: %v", err)
		}

		return nil, status.Errorf(codes.Internal, "an error occurred: %v", err)
	}

	logger.Infof("creating microvm %s", modelSpec.ID)
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

//nolint:dupl
func (s *server) UpdateMicroVM(ctx context.Context, req *mvmv1.UpdateMicroVMRequest) (*mvmv1.UpdateMicroVMResponse, error) {
	logger := log.GetLogger(ctx)

	logger.Trace("converting request to model")
	modelSpec, err := convertMicroVMToModel(req.Microvm)
	if err != nil {
		return nil, fmt.Errorf("converting request: %w", err)
	}

	logger.Trace("validating model")
	err = s.validator.ValidateStruct(modelSpec)
	var valErrors validator.ValidationErrors
	if err != nil {
		if errors.As(err, &valErrors) {
			return nil, status.Errorf(codes.InvalidArgument, "an error occurred when attempting to validate the request: %v", err)
		}

		return nil, status.Errorf(codes.Internal, "an error occurred: %v", err)
	}

	logger.Infof("updating microvm %s", modelSpec.ID)
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
