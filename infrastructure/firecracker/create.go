package firecracker

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"

	"github.com/firecracker-microvm/firecracker-go-sdk"
	"github.com/sirupsen/logrus"
	"github.com/spf13/afero"
	"github.com/weaveworks-liquidmetal/flintlock/core/models"
	"github.com/weaveworks-liquidmetal/flintlock/pkg/defaults"
	"github.com/weaveworks-liquidmetal/flintlock/pkg/log"
	"github.com/weaveworks-liquidmetal/flintlock/pkg/process"
)

// Create will create a new microvm.
func (p *fcProvider) Create(ctx context.Context, vm *models.MicroVM) error {
	logger := log.GetLogger(ctx).WithFields(logrus.Fields{
		"service": "firecracker_microvm",
		"vmid":    vm.ID.String(),
	})
	logger.Debug("creating microvm")

	vmState := NewState(vm.ID, vm.Status.RuntimeStateDir, p.fs)

	if err := p.ensureState(vmState); err != nil {
		return fmt.Errorf("ensuring state dir: %w", err)
	}

	opts := []ConfigOption{
		WithMicroVM(vm, p.config.CloudInitFromMMDS),
		WithState(vmState),
	}

	config, err := CreateConfig(opts...)
	if err != nil {
		return fmt.Errorf("creating firecracker config: %w", err)
	}

	if err = vmState.SetConfig(config); err != nil {
		return fmt.Errorf("saving firecracker config: %w", err)
	}

	meta := &Metadata{
		Latest: vm.Spec.Metadata.Items,
	}

	if err = vmState.SetMetadata(meta); err != nil {
		return fmt.Errorf("saving firecracker metadata: %w", err)
	}

	args := []string{"--id", vm.ID.UID(), "--boot-timer", "--no-api"}
	args = append(args, "--config-file", vmState.ConfigPath())
	args = append(args, "--metadata", vmState.MetadataPath())

	cmd := firecracker.VMCommandBuilder{}.
		WithBin(p.config.FirecrackerBin).
		WithArgs(args).
		Build(context.TODO()) //nolint: contextcheck // Intentional.

	proc, err := p.startFirecracker(cmd, vmState, p.config.RunDetached)
	if err != nil {
		return fmt.Errorf("starting firecracker process: %w", err)
	}

	if err = vmState.SetPid(proc.Pid); err != nil {
		return fmt.Errorf("saving pid %d to file: %w", proc.Pid, err)
	}

	return nil
}

func (p *fcProvider) startFirecracker(cmd *exec.Cmd, vmState State, detached bool) (*os.Process, error) {
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
	cmd.Stdin = &bytes.Buffer{}

	var startErr error

	if detached {
		startErr = process.DetachedStart(cmd)
	} else {
		startErr = cmd.Start()
	}

	if startErr != nil {
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
		if err = p.fs.MkdirAll(vmState.Root(), defaults.DataDirPerm); err != nil {
			return fmt.Errorf("creating state directory %s: %w", vmState.Root(), err)
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

// func (p *fcProvider) updateVendorData(vm *models.MicroVM) error {
// 	vendorData := &userdata.UserData{}
// 	vendorDataRaw, ok := vm.Spec.Metadata.Items[cloudinit.VendorDataKey]
// 	if ok {
// 		data, err := base64.RawStdEncoding.DecodeString(vendorDataRaw)
// 		if err != nil {
// 			return fmt.Errorf("deconding vendor data: %w", err)
// 		}
// 		if marshalErr := yaml.Unmarshal(data, vendorData); marshalErr != nil {
// 			return fmt.Errorf("unmarshalling vendordata yaml: %w", err)
// 		}
// 	}

// 	vendorData.Mounts = []userdata.Mount{
// 		userdata.Mount{"vdb2", "/opt/data"},
// 	}
// 	vendorData.MountDefaultFields = userdata.Mount{"None", "None", "auto", "defaults,nofail", "0", "2"}

// 	data, err := yaml.Marshal(vendorData)
// 	if err != nil {
// 		return fmt.Errorf("marshalling vendor data to yaml: %w", err)
// 	}
// 	dataWithHeader := append([]byte("## template: jinja\n#cloud-config\n\n"), data...)
// 	vm.Spec.Metadata.Items[cloudinit.VendorDataKey] = base64.StdEncoding.EncodeToString(dataWithHeader)

// 	return nil
// }
