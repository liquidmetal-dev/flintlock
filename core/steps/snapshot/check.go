// Package snapshot holds the steps of the plan that captures a snapshot of a
// microvm, and the rules a microvm must meet to be snapshotted.
package snapshot

import (
	"fmt"

	coreerrs "github.com/liquidmetal-dev/flintlock/core/errors"
	"github.com/liquidmetal-dev/flintlock/core/models"
	"github.com/liquidmetal-dev/flintlock/core/ports"
)

// ErrSnapshotNotSupported tells that the provider of a microvm cannot take a snapshot.
var ErrSnapshotNotSupported = coreerrs.NewFailedPrecondition(
	"snapshot not supported by the microvm provider: it needs the snapshot capability",
)

// CheckSource rejects a source microvm that cannot be snapshotted. Each
// refusal is a failed precondition with the reason.
func CheckSource(vm *models.MicroVM, provider ports.MicroVMService) error {
	if err := CheckState(vm); err != nil {
		return err
	}

	if err := CheckCapability(provider); err != nil {
		return err
	}

	return CheckSpec(vm)
}

// CheckState rejects a microvm that is not created or is being deleted.
func CheckState(vm *models.MicroVM) error {
	if vm.Spec.DeletedAt != 0 || vm.Status.State == models.DeletingState {
		return coreerrs.NewFailedPrecondition(fmt.Sprintf("microvm %s is being deleted", vm.ID.UID()))
	}

	if vm.Status.State != models.CreatedState {
		return coreerrs.NewFailedPrecondition(
			fmt.Sprintf("microvm %s is not running (state: %s)", vm.ID.UID(), vm.Status.State),
		)
	}

	return nil
}

// CheckCapability rejects a provider that cannot take a snapshot.
func CheckCapability(provider ports.MicroVMService) error {
	if !provider.Capabilities().Has(models.SnapshotCapability) {
		return ErrSnapshotNotSupported
	}

	return nil
}

// CheckSpec rejects a microvm whose volumes a snapshot cannot capture, or whose
// guest-agent is not ready.
func CheckSpec(vm *models.MicroVM) error {
	if err := checkVolumes(vm); err != nil {
		return err
	}

	if !vm.Spec.AllowGuestAgent {
		return coreerrs.NewFailedPrecondition(
			fmt.Sprintf("microvm %s does not have allow_guest_agent set", vm.ID.UID()),
		)
	}

	if !guestAgentReady(vm) {
		return coreerrs.NewFailedPrecondition(
			fmt.Sprintf("the guest agent of microvm %s is not ready", vm.ID.UID()),
		)
	}

	return nil
}

// checkVolumes rejects a microvm with a device that a snapshot cannot capture.
// Only an overlay root can be captured.
func checkVolumes(vm *models.MicroVM) error {
	uid := vm.ID.UID()

	if !vm.Spec.RootVolume.IsOverlay() {
		return coreerrs.NewFailedPrecondition(fmt.Sprintf(
			"microvm %s has a root volume that is not an overlay: only an overlay root can be snapshotted", uid,
		))
	}

	for _, volume := range vm.Spec.AdditionalVolumes {
		if volume.Source.VirtioFS != nil {
			return coreerrs.NewFailedPrecondition(fmt.Sprintf(
				"microvm %s has virtiofs volume %s: a virtiofs mount cannot be snapshotted", uid, volume.ID,
			))
		}
	}

	for _, volume := range vm.Spec.AdditionalVolumes {
		if volume.Source.Container != nil && volume.EffectiveMode() == models.VolumeModeFull {
			return coreerrs.NewFailedPrecondition(fmt.Sprintf(
				"microvm %s has full volume %s: a full volume cannot be snapshotted", uid, volume.ID,
			))
		}
	}

	if len(vm.Spec.AdditionalVolumes) > 0 {
		return coreerrs.NewFailedPrecondition(fmt.Sprintf(
			"microvm %s has additional volumes: a microvm with additional volumes cannot be snapshotted", uid,
		))
	}

	return nil
}

func guestAgentReady(vm *models.MicroVM) bool {
	cond, ok := vm.Status.Conditions.Get(models.ConditionGuestAgentReady)

	return ok && cond.Status == models.ConditionTrue
}
