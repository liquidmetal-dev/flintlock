package firecracker

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/firecracker-microvm/firecracker-go-sdk"
	"github.com/sirupsen/logrus"
	"github.com/spf13/afero"

	"github.com/weaveworks/reignite/core/models"
	"github.com/weaveworks/reignite/pkg/defaults"
	"github.com/weaveworks/reignite/pkg/log"
	"github.com/weaveworks/reignite/pkg/process"
	"github.com/weaveworks/reignite/pkg/wait"
)

const (
	socketTimeoutInSec = 10
	socketPollInMs     = 500
)

// Create will create a new microvm.
func (p *fcProvider) Create(ctx context.Context, vm *models.MicroVM) error {
	logger := log.GetLogger(ctx).WithFields(logrus.Fields{
		"service": "firecracker_microvm",
		"vmid":    vm.ID.String(),
	})
	logger.Debugf("creating microvm")

	vmState := NewState(vm.ID, p.config.StateRoot, p.fs)
	if err := p.ensureState(vmState); err != nil {
		return fmt.Errorf("ensuring state dir: %w", err)
	}

	config, err := CreateConfig(WithMicroVM(vm), WithState(vmState))
	if err != nil {
		return fmt.Errorf("creating firecracker config: %w", err)
	}
	if err := vmState.SetConfig(config); err != nil {
		return fmt.Errorf("saving firecracker config: %w", err)
	}

	id := strings.ReplaceAll(vm.ID.String(), "/", "-")
	args := []string{"--id", id, "--boot-timer"}
	if !p.config.APIConfig {
		args = append(args, "--config-file", vmState.ConfigPath())
	}
	cmd := firecracker.VMCommandBuilder{}.
		WithBin(p.config.FirecrackerBin).
		WithSocketPath(vmState.SockPath()).
		WithArgs(args).
		Build(ctx)

	var proc *os.Process
	if p.config.RunDetached {
		proc, err = p.startFirecrackerDetached(cmd, vmState)
	} else {
		proc, err = p.startFirecracker(cmd, vmState)
	}
	if err != nil {
		return fmt.Errorf("starting firecracker process: %w", err)
	}

	if err := vmState.SetPid(proc.Pid); err != nil {
		return fmt.Errorf("saving pid %d to file: %w", proc.Pid, err)
	}

	err = wait.ForCondition(wait.FileExistsCondition(vmState.SockPath(), p.fs), socketTimeoutInSec*time.Second, socketPollInMs*time.Millisecond)
	if err != nil {
		return fmt.Errorf("waiting for sock file to exist: %w", err)
	}

	if p.config.APIConfig {
		client := firecracker.NewClient(vmState.SockPath(), logger, true)
		if err := ApplyConfig(ctx, config, client); err != nil {
			return fmt.Errorf("applying firecracker configuration: %w", err)
		}
		if err := ApplyMetadata(ctx, vm.Spec.Metadata, client); err != nil {
			return fmt.Errorf("applying metadata to mmds: %w", err)
		}
	}

	return nil
}

func (p *fcProvider) startFirecrackerDetached(cmd *exec.Cmd, vmState State) (*os.Process, error) {
	stdOutFile, err := os.OpenFile(vmState.StdoutPath(), os.O_WRONLY|os.O_CREATE|os.O_APPEND, defaults.DataFilePerm)
	if err != nil {
		return nil, fmt.Errorf("opening stdout file %s: %w", vmState.StdoutPath(), err)
	}

	stdErrFile, err := os.OpenFile(vmState.StderrPath(), os.O_WRONLY|os.O_CREATE|os.O_APPEND, defaults.DataFilePerm)
	if err != nil {
		return nil, fmt.Errorf("opening sterr file %s: %w", vmState.StderrPath(), err)
	}
	proc, err := process.StartCommandDetached(cmd, stdErrFile, stdOutFile)
	if err != nil {
		return nil, fmt.Errorf("daemonizing command %s: %w", cmd.Path, err)
	}

	return proc, nil
}

func (p *fcProvider) startFirecracker(cmd *exec.Cmd, vmState State) (*os.Process, error) {
	stdOutFile, err := p.fs.OpenFile(vmState.StdoutPath(), os.O_WRONLY|os.O_CREATE|os.O_APPEND, defaults.DataFilePerm)
	if err != nil {
		return nil, fmt.Errorf("opening stdout file %s: %w", vmState.StdoutPath(), err)
	}

	stdErrFile, err := p.fs.OpenFile(vmState.StderrPath(), os.O_WRONLY|os.O_CREATE|os.O_APPEND, defaults.DataFilePerm)
	if err != nil {
		return nil, fmt.Errorf("opening sterr file %s: %w", vmState.StderrPath(), err)
	}
	cmd.Stderr = stdErrFile
	cmd.Stdout = stdOutFile

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("starting firecracker process: %w", err)
	}

	return cmd.Process, nil
}

func (p *fcProvider) ensureState(vmState State) error {
	exists, err := afero.DirExists(p.fs, vmState.Root())
	if err != nil {
		return fmt.Errorf("checking if state dir %s exists: %w", vmState.Root(), err)
	}

	if !exists {
		if err := p.fs.MkdirAll(vmState.Root(), defaults.DataDirPerm); err != nil {
			return fmt.Errorf("creating state directory %s: %w", vmState.Root(), err)
		}
	}

	sockExists, err := afero.Exists(p.fs, vmState.SockPath())
	if err != nil {
		return fmt.Errorf("checking if sock dir exists: %w", err)
	}
	if sockExists {
		if delErr := p.fs.Remove(vmState.SockPath()); delErr != nil {
			return fmt.Errorf("deleting existing sock file: %w", err)
		}
	}

	logFile, err := p.fs.OpenFile(vmState.LogPath(), os.O_WRONLY|os.O_CREATE|os.O_APPEND, defaults.DataFilePerm)
	if err != nil {
		return fmt.Errorf("opening log file %s: %w", vmState.LogPath(), err)
	}
	logFile.Close()

	metricsFile, err := p.fs.OpenFile(vmState.MetricsPath(), os.O_WRONLY|os.O_CREATE|os.O_APPEND, defaults.DataFilePerm)
	if err != nil {
		return fmt.Errorf("opening metrics file %s: %w", vmState.MetricsPath(), err)
	}
	metricsFile.Close()

	return nil
}
