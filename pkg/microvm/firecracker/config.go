package firecracker

import (
	"github.com/firecracker-microvm/firecracker-go-sdk"
	fcmodels "github.com/firecracker-microvm/firecracker-go-sdk/client/models"
	"github.com/weaveworks/reignite/pkg/models"
)

func (p *fcProvider) getConfig(machine *models.MicroVM) (*firecracker.Config, error) {
	if p.config.SocketPath == "" {
		return nil, errSocketPathRequired
	}

	// TODO: get the metadata data and populate the rest of firecracker.Config

	conf := &firecracker.Config{
		SocketPath: p.config.SocketPath,
		// LogFifo:         "",
		// LogLevel:        "",
		// MetricsFifo:     "",
		// FifoLogWriter:   nil,
		KernelImagePath: string(machine.Spec.Kernel.Image),
		KernelArgs:      machine.Spec.Kernel.CmdLine,
		// Drives: ,
		// NetworkInterfaces: ,
		// VsockDevices: ,
		MachineCfg: fcmodels.MachineConfiguration{
			VcpuCount: firecracker.Int64(machine.Spec.VCPU),
			// CPUTemplate: ,
			// HtEnabled: ,
			MemSizeMib: firecracker.Int64(machine.Spec.MemoryInMb),
		},
		// JailerCfg: nil,
		VMID: machine.ID,
	}

	return conf, nil
}
