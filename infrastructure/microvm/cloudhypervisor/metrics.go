package cloudhypervisor

import (
	"context"
	"fmt"

	"github.com/liquidmetal-dev/flintlock/core/models"
	"github.com/liquidmetal-dev/flintlock/core/ports"
	"github.com/liquidmetal-dev/flintlock/infrastructure/microvm/shared"
	"github.com/liquidmetal-dev/flintlock/pkg/cloudhypervisor"
)

// Metrics returns with the metrics of a microvm.
func (p *provider) Metrics(ctx context.Context, vmid models.VMID) (ports.MachineMetrics, error) {
	vmState := NewState(vmid, p.config.StateRoot, p.fs)

	chClient := cloudhypervisor.New(vmState.SockPath())
	counters, err := chClient.Counters(ctx)
	if err != nil {
		return nil, fmt.Errorf("getting vm counters: %w", err)
	}

	machineMetrics := shared.MachineMetrics{
		Namespace:   vmid.Namespace(),
		MachineName: vmid.Name(),
		MachineUID:  vmid.UID(),
		Data:        shared.Metrics{},
	}

	for resourceName, resourceCounters := range *counters {
		machineMetrics.Data[resourceName] = resourceCounters
	}

	return machineMetrics, nil
}
