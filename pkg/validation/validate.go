package validation

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/containerd/containerd/v2/pkg/reference"
	playgroundValidator "github.com/go-playground/validator/v10"

	"github.com/liquidmetal-dev/flintlock/core/models"
)

type Validator interface {
	ValidateStruct(interface{}) error
}

type validate struct {
	validator *playgroundValidator.Validate
}

func NewValidator() Validator {
	validator := playgroundValidator.New()

	// TODO(@jmickey): Do something with this error maybe? #236
	_ = validator.RegisterValidation("imageURI", customImageURIValidator, false)
	_ = validator.RegisterValidation("datetimeInPast", customTimestampValidator, false)
	_ = validator.RegisterValidation("guestDeviceName", customNetworkGuestDeviceNameValidator, false)
	_ = validator.RegisterValidation("novirtiofs", customNoVirtioFSValidator, false)
	_ = validator.RegisterValidation("onlyOneVirtioFS", customOnlyOneVirtioFSValidator, false)
	_ = validator.RegisterValidation("multipleVolSources", customMultipleVolSources, false)
	_ = validator.RegisterValidation("noReadOnlyVirtioFS", customNoReadOnlyVirtioFSValidator, false)
	_ = validator.RegisterValidation("kvmCapability", customKVMCapabilityValidator, false)
	_ = validator.RegisterValidation("imageFilePath", customImageFilePathValidator, false)
	_ = validator.RegisterValidation("noReadOnlyOverlay", customNoReadOnlyOverlayValidator, false)
	_ = validator.RegisterValidation("overlayMinSize", customOverlayMinSizeValidator, false)
	_ = validator.RegisterValidation("noOverlayVolumes", customNoOverlayVolumesValidator, false)
	validator.RegisterStructValidation(customMicroVMSpecStructLevelValidation, models.MicroVMSpec{})

	return &validate{
		validator: validator,
	}
}

func (v *validate) ValidateStruct(obj interface{}) error {
	if err := v.validator.Struct(obj); err != nil {
		return fmt.Errorf("validation failures found: %w", err)
	}

	return nil
}

func customImageURIValidator(fl playgroundValidator.FieldLevel) bool {
	uri := fl.Field().String()

	_, err := reference.Parse(uri)

	return err == nil
}

// Ensure that the timestamp is in the past and greater than 0.
func customTimestampValidator(fl playgroundValidator.FieldLevel) bool {
	tm := fl.Field().Int()

	return tm <= time.Now().Unix() && tm > 0
}

func customNetworkGuestDeviceNameValidator(fieldLevel playgroundValidator.FieldLevel) bool {
	name := fieldLevel.Field().String()
	re := regexp.MustCompile("^[a-z][a-z0-9_]*$")

	return re.MatchString(name)
}

func customMicroVMSpecStructLevelValidation(structLevel playgroundValidator.StructLevel) {
	spec, _ := structLevel.Current().Interface().(models.MicroVMSpec)

	// Based on the firecracker documentation:
	// https://github.com/firecracker-microvm/firecracker/blob/main/docs/initrd.md#notes
	//
	// If initramfs is not specified then at least one volume needs to specify is the `is_root_device` flag.
	// Therefore this validation checks if spec.Initrd is `nil`, and if so loops through the volumes to check
	// that a root device has been configured.
	if spec.Initrd == nil && (models.Volume{}) == spec.RootVolume {
		structLevel.ReportError(spec.RootVolume, "root_volume", "RootVolume", "volumeOrInitrdRequired", "")

		return
	}

	// In overlay mode the initramfs /init is the contract between flintlock and the guest.
	// rdinit= replaces that /init, so a spec that sets it in overlay mode is rejected.
	if spec.RootVolume.EffectiveMode() == models.VolumeModeOverlay {
		if _, ok := spec.Kernel.CmdLine["rdinit"]; ok {
			structLevel.ReportError(spec.Kernel.CmdLine, "kernel.cmdline", "CmdLine", "noRdinitInOverlay", "")
		}
	}
}

// customNoReadOnlyOverlayValidator ensures a volume in overlay mode is not read-only.
// The overlay root needs its writable disk, so a read-only root has no meaning there.
func customNoReadOnlyOverlayValidator(fieldLevel playgroundValidator.FieldLevel) bool {
	volume, _ := fieldLevel.Field().Interface().(models.Volume)

	return !(volume.EffectiveMode() == models.VolumeModeOverlay && volume.IsReadOnly)
}

// customOverlayMinSizeValidator ensures the writable disk of a volume in overlay mode is
// at least models.OverlayMinWritableDiskMB. A size of zero means the daemon default.
func customOverlayMinSizeValidator(fieldLevel playgroundValidator.FieldLevel) bool {
	volume, _ := fieldLevel.Field().Interface().(models.Volume)

	if volume.EffectiveMode() != models.VolumeModeOverlay || volume.Size == 0 {
		return true
	}

	return volume.Size >= models.OverlayMinWritableDiskMB
}

// customNoOverlayVolumesValidator ensures no additional volume is in overlay mode.
// Only the root volume can be an overlay.
func customNoOverlayVolumesValidator(fieldLevel playgroundValidator.FieldLevel) bool {
	field, _ := fieldLevel.Field().Interface().(models.Volumes)

	for _, volume := range field {
		if volume.EffectiveMode() == models.VolumeModeOverlay {
			return false
		}
	}

	return true
}

func customNoVirtioFSValidator(fieldLevel playgroundValidator.FieldLevel) bool {
	field, _ := fieldLevel.Field().Interface().(models.Volume)

	return field.Source.VirtioFS == nil
}

func customOnlyOneVirtioFSValidator(fieldLevel playgroundValidator.FieldLevel) bool {
	field, _ := fieldLevel.Field().Interface().(models.Volumes)
	virtioFSCount := 0 // Counter for VirtioFS volumes
	if len(field) > 0 {
		for _, volume := range field {
			// Check if this volume has VirtioFS
			if volume.Source.VirtioFS != nil {
				virtioFSCount++
			}
		}
	}

	return !(virtioFSCount > 1)
}

// customKVMCapabilityValidator ensures a KVM capability ID is a plain non-negative
// integer. The "!" disable prefix is added internally and must not be supplied by the caller.
func customKVMCapabilityValidator(fieldLevel playgroundValidator.FieldLevel) bool {
	value := fieldLevel.Field().String()
	re := regexp.MustCompile(`^\d+$`)

	return re.MatchString(value)
}

func customMultipleVolSources(fieldLevel playgroundValidator.FieldLevel) bool {
	field, _ := fieldLevel.Field().Interface().(models.Volumes)
	if len(field) > 0 {
		for _, volume := range field {
			if volume.Source.Container != nil && volume.Source.VirtioFS != nil {
				return false
			}
		}
	}

	return true
}

// customNoReadOnlyVirtioFSValidator ensures that no virtiofs volume is marked read-only.
// Nothing enforces the mark for a virtiofs volume, so the guest could write to a share
// that the spec says is protected.
func customNoReadOnlyVirtioFSValidator(fieldLevel playgroundValidator.FieldLevel) bool {
	field, _ := fieldLevel.Field().Interface().(models.Volumes)

	for _, volume := range field {
		if volume.Source.VirtioFS != nil && volume.IsReadOnly {
			return false
		}
	}

	return true
}

// customImageFilePathValidator ensures a path to a file inside a container image is
// relative and can't escape the image root (e.g. no absolute paths or ".." traversal).
func customImageFilePathValidator(fieldLevel playgroundValidator.FieldLevel) bool {
	path := fieldLevel.Field().String()

	return filepath.IsLocal(path) && !strings.ContainsRune(path, 0)
}
