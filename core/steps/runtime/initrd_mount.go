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

func NewInitrdMount(vm *models.MicroVM, imageService ports.ImageService) planner.Procedure {
	return &initrdMount{
		vm:       vm,
		imageSvc: imageService,
	}
}

type initrdMount struct {
	vm       *models.MicroVM
	imageSvc ports.ImageService
}

// Name is the name of the procedure/operation.
func (s *initrdMount) Name() string {
	return "runtime_initrd_mount"
}

func (s *initrdMount) ShouldDo(ctx context.Context) (bool, error) {
	if s.vm == nil {
		return false, cerrs.ErrSpecRequired
	}

	logger := log.GetLogger(ctx).WithFields(logrus.Fields{
		"step":  s.Name(),
		"image": s.vm.Spec.Kernel.Image,
	})
	logger.Debug("checking if procedure should be run")

	if s.vm.Spec.Initrd == nil {
		return false, nil
	}

	if s.vm.Status.InitrdMount == nil || s.vm.Status.InitrdMount.Source == "" {
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
func (s *initrdMount) Do(ctx context.Context) ([]planner.Procedure, error) {
	if s.vm == nil {
		return nil, cerrs.ErrSpecRequired
	}

	logger := log.GetLogger(ctx).WithFields(logrus.Fields{
		"step":  s.Name(),
		"image": s.vm.Spec.Kernel.Image,
	})
	logger.Debug("running step to mount initrd image")

	input := s.getMountSpec()

	mounts, err := s.imageSvc.PullAndMount(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("mount images %s for initrd use: %w", input.ImageName, err)
	}

	if len(mounts) == 0 {
		return nil, cerrs.ErrNoMount
	}

	s.vm.Status.InitrdMount = &mounts[0]

	return nil, nil
}

func (s *initrdMount) getMountSpec() *ports.ImageMountSpec {
	return &ports.ImageMountSpec{
		ImageName:    string(s.vm.Spec.Initrd.Image),
		Owner:        s.vm.ID.String(),
		OwnerUsageID: "initrd",
		Use:          models.ImageUseInitrd,
	}
}

func (s *initrdMount) Verify(ctx context.Context) error {
	return nil
}
