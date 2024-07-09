package runtime

import (
	"context"
	"fmt"

	cerrs "github.com/liquidmetal-dev/flintlock/core/errors"
	"github.com/liquidmetal-dev/flintlock/core/models"
	"github.com/liquidmetal-dev/flintlock/core/ports"
	"github.com/liquidmetal-dev/flintlock/pkg/log"
	"github.com/liquidmetal-dev/flintlock/pkg/planner"
	"github.com/sirupsen/logrus"
)

func NewKernelMount(vm *models.MicroVM, imageService ports.ImageService) planner.Procedure {
	return &kernelMount{
		vm:       vm,
		imageSvc: imageService,
	}
}

type kernelMount struct {
	vm       *models.MicroVM
	imageSvc ports.ImageService
}

// Name is the name of the procedure/operation.
func (s *kernelMount) Name() string {
	return "runtime_kernel_mount"
}

func (s *kernelMount) ShouldDo(ctx context.Context) (bool, error) {
	if s.vm == nil {
		return false, cerrs.ErrSpecRequired
	}

	logger := log.GetLogger(ctx).WithFields(logrus.Fields{
		"step":  s.Name(),
		"image": s.vm.Spec.Kernel.Image,
	})
	logger.Debug("checking if procedure should be run")

	if s.vm.Status.KernelMount == nil || s.vm.Status.KernelMount.Source == "" {
		return true, nil
	}

	input := s.getMountSpec()

	mounted, err := s.imageSvc.IsMounted(ctx, input)
	if err != nil {
		return false, fmt.Errorf("checking if image %s is mounted: %w", input.ImageName, err)
	}

	return !mounted, nil
}

// Do will perform the operation/procedure.
func (s *kernelMount) Do(ctx context.Context) ([]planner.Procedure, error) {
	if s.vm == nil {
		return nil, cerrs.ErrSpecRequired
	}

	if s.vm.Spec.Kernel.Image == "" {
		return nil, cerrs.ErrKernelImageRequired
	}

	logger := log.GetLogger(ctx).WithFields(logrus.Fields{
		"step":  s.Name(),
		"image": s.vm.Spec.Kernel.Image,
	})
	logger.Debug("running step to mount kernel image")

	input := s.getMountSpec()

	mounts, err := s.imageSvc.PullAndMount(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("mount images %s for kernel use: %w", input.ImageName, err)
	}

	if len(mounts) == 0 {
		return nil, cerrs.ErrNoMount
	}

	s.vm.Status.KernelMount = &mounts[0]

	return nil, nil
}

func (s *kernelMount) getMountSpec() *ports.ImageMountSpec {
	return &ports.ImageMountSpec{
		ImageName:    string(s.vm.Spec.Kernel.Image),
		Owner:        s.vm.ID.String(),
		OwnerUsageID: "kernel",
		Use:          models.ImageUseKernel,
	}
}

func (s *kernelMount) Verify(ctx context.Context) error {
	return nil
}
