package application

import (
	"context"
	"fmt"

	"github.com/weaveworks/flintlock/core/models"
	"github.com/weaveworks/flintlock/core/ports"
	"github.com/weaveworks/flintlock/pkg/log"
)

// GetVMStatus returns the status for a microvm based on the spec and the running state of the microvm.
func (a *app) GetMicroVMStatus(ctx context.Context, id, namespace string) (models.Status, error) {
	logger := log.GetLogger(ctx).WithField("component", "app")

	logger.Debugf("Getting status for %s/%s", namespace, id)

	if id == "" {
		return models.StatusUnknown, errIDRequired
	}

	if namespace == "" {
		return models.StatusUnknown, errNamespaceRequired
	}

	foundMvm, err := a.ports.Repo.Get(ctx, ports.RepositoryGetOptions{Name: id, Namespace: namespace})
	if err != nil {
		return models.StatusUnknown, fmt.Errorf("error attempting to locate microvm with id: %s, in namespace: %s: %w", id, namespace, err)
	}
	if foundMvm == nil {
		return models.StatusUnknown, nil
	}

	microvmState, err := a.ports.Provider.State(ctx, fmt.Sprintf("%s/%s", namespace, id))
	if err != nil {
		return models.StatusUnknown, fmt.Errorf("getting microvm state: %w", err)
	}

	if foundMvm.Spec.DeletedAt > 0 {
		return models.StatusDeleting, nil
	}

	//TODO: handle retry exceeded failed / failure message

	switch microvmState {
	case ports.MicroVMStatePending:
		return models.StatusPending, nil
	case ports.MicroVMStateStopped:
		return models.StatusStopped, nil
	case ports.MicroVMStateRunning:
		return models.StatusRunning, nil
	case ports.MicroVMStateUnknown:
		return models.StatusUnknown, nil
	default:
		logger.Warnf("unknown microvm state: %s", microvmState)
	}

	return models.StatusUnknown, nil
}
