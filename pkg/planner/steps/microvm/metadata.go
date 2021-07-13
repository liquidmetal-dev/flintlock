package microvm

import (
	"context"
	"fmt"
	"time"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/sirupsen/logrus"
	reignitev1 "github.com/weaveworks/reignite/api/reignite/v1alpha1"
	"github.com/weaveworks/reignite/pkg/id"
	"github.com/weaveworks/reignite/pkg/planner"
	"github.com/weaveworks/reignite/pkg/state"
)

func NewPopulateMetadataStep(microvm *reignitev1.MicroVM, vmState state.StateProvider, logger *logrus.Entry) planner.Procedure {
	return &populateMetadata{
		microvm: microvm,
		vmState: vmState,
		logger:  logger.WithField("vmid", microvm.Name),
	}
}

type populateMetadata struct {
	microvm *reignitev1.MicroVM
	vmState state.StateProvider
	logger  *logrus.Entry
}

// Name is the name of the procedure/operation.
func (s *populateMetadata) Name() string {
	return "microvm_populate_metadata"
}

// Do will perform the operation/procedure.
func (s *populateMetadata) Do(ctx context.Context) ([]planner.Procedure, error) {
	vmid, err := id.New()
	if err != nil {
		return nil, fmt.Errorf("generating vmid: %w", err)
	}
	s.microvm.Name = vmid

	s.microvm.CreationTimestamp = v1.NewTime(time.Now().UTC())
	s.microvm.Generation = 0

	s.logger.Debug("populated microvm metadata")

	return nil, nil
}
