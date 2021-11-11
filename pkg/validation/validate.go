package validation

import (
	"fmt"
	"regexp"
	"time"

	"github.com/containerd/containerd/reference"
	playgroundValidator "github.com/go-playground/validator/v10"

	"github.com/weaveworks/flintlock/core/models"
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

	if spec.Initrd == nil && len(spec.Volumes) == 0 {
		structLevel.ReportError(spec.Volumes, "volumes", "Volumes", "volumeOrInitrdRequired", "")

		return
	}

	// Based on the firecracker documentation:
	// https://github.com/firecracker-microvm/firecracker/blob/main/docs/initrd.md#notes
	//
	// If initramfs is not specified then at least one volume needs to specify is the `is_root_device` flag.
	// Therefore this validation checks if spec.Initrd is `nil`, and if so loops through the volumes to check
	// that a root device has been configured.

	var found bool

	for _, vol := range spec.Volumes {
		if vol.IsRoot {
			if found {
				// Only one volume can be specified as the root volume. If a root volume is found twice then
				// report an error.
				structLevel.ReportError(spec.Volumes, "volumes", "Volumes", "onlyOneRootVolume", "")
			}

			found = true
		}
	}

	if spec.Initrd == nil && !found {
		structLevel.ReportError(spec.Volumes, "volumes", "Volumes", "oneVolumeMustBeRoot", "")

		return
	}
}
