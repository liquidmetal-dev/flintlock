package grpc

import (
	"context"
	"fmt"

	mvmv1 "github.com/weaveworks-liquidmetal/flintlock/api/services/microvm/v1alpha1"
	"github.com/weaveworks-liquidmetal/flintlock/core/ports"
	"github.com/weaveworks-liquidmetal/flintlock/pkg/log"
	"google.golang.org/protobuf/types/known/emptypb"
)

type debugServer struct {
	queryUC ports.MicroVMQueryUseCases
}

// NewDebugServer creates a new debugServer instance.
func NewDebugServer(queryUC ports.MicroVMQueryUseCases) ports.DebugInfoGRPCService {
	return &debugServer{
		queryUC: queryUC,
	}
}

func (s *debugServer) GetDebugInfo(ctx context.Context, _ *emptypb.Empty) (*mvmv1.DebugInfo, error) {
	logger := log.GetLogger(ctx)

	microVMS, err := s.queryUC.GetAllMicroVM(ctx, nil)
	if err != nil {
		logger.Errorf("failed to get all microVMs: %s", err)
		return nil, fmt.Errorf("fetching microVMs: %w", err)
	}

	var mvmCount int32
	var ifaceCount int32
	var totalVCPUCount int32
	var totalMemoryInMB int32
	for _, microVM := range microVMS {
		mvmCount += 1
		ifaceCount += int32(len(microVM.Status.NetworkInterfaces))
		totalVCPUCount += int32(microVM.Spec.VCPU)
		totalMemoryInMB += int32(microVM.Spec.MemoryInMb)
	}

	debugInfo := &mvmv1.DebugInfo{
		MvmCount:        mvmCount,
		IfaceCount:      ifaceCount,
		TotalVcpu:       totalVCPUCount,
		TotalMemoryInMb: totalMemoryInMB,
	}

	return debugInfo, nil
}
