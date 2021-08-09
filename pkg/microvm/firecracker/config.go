package firecracker

import (
	"github.com/firecracker-microvm/firecracker-go-sdk"
	"github.com/firecracker-microvm/firecracker-go-sdk/client/models"

	reignitev1 "github.com/weaveworks/reignite/api/kinds/v1alpha1"
)

func (p *fcProvider) getConfig(machine *reignitev1.MicroVM) (*firecracker.Config, error) {
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
		MachineCfg: models.MachineConfiguration{
			VcpuCount: firecracker.Int64(machine.Spec.VCPU),
			// CPUTemplate: ,
			// HtEnabled: ,
			MemSizeMib: firecracker.Int64(machine.Spec.MemoryInMb),
		},
		// JailerCfg: nil,
		VMID: machine.Name,
	}

	return conf, nil
}
