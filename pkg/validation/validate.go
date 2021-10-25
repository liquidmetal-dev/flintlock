package validation

import (
	"fmt"
	"regexp"
	"time"

	"github.com/containerd/containerd/reference"
	"github.com/go-playground/validator/v10"
	"github.com/weaveworks/flintlock/core/models"
)

type Validator interface {
	ValidateStruct(interface{}) error
}

type validate struct {
	validator *validator.Validate
}

func NewValidator() Validator {
	v := validator.New()

	// TODO(@jmickey): Do something with this error maybe?
	_ = v.RegisterValidation("imageURI", customImageURIValidator, false)
	_ = v.RegisterValidation("datetimeInPast", customTimestampValidator, false)
	_ = v.RegisterValidation("guestDeviceName", customNetworkGuestDeviceNameValidator, false)
	v.RegisterStructValidation(customMicroVMSpecStructLevelValidation, models.MicroVMSpec{})

	return &validate{
		validator: v,
	}
}

func (v *validate) ValidateStruct(obj interface{}) error {
	if err := v.validator.Struct(obj); err != nil {
		return fmt.Errorf("validation failures found: %w", err)
	}

	return nil
}

func customImageURIValidator(fl validator.FieldLevel) bool {
	uri := fl.Field().String()

	_, err := reference.Parse(uri)

	return err == nil
}

// Ensure that the timestamp is in the past and greater than 0.
func customTimestampValidator(fl validator.FieldLevel) bool {
	tm := fl.Field().Int()

	return tm <= time.Now().Unix() && tm > 0
}

func customNetworkGuestDeviceNameValidator(fl validator.FieldLevel) bool {
	name := fl.Field().String()
	re := regexp.MustCompile("^[a-z][a-z0-9_]*$")

	return re.MatchString(name)
}

func customMicroVMSpecStructLevelValidation(sl validator.StructLevel) {
	spec, _ := sl.Current().Interface().(models.MicroVMSpec)

	if spec.Initrd == nil && len(spec.Volumes) == 0 {
		sl.ReportError(spec.Volumes, "volumes", "Volumes", "volumeOrInitrdRequired", "")

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
				sl.ReportError(spec.Volumes, "volumes", "Volumes", "onlyOneRootVolume", "")
			}

			found = true
		}
	}

	if spec.Initrd == nil && !found {
		sl.ReportError(spec.Volumes, "volumes", "Volumes", "oneVolumeMustBeRoot", "")

		return
	}

	if spec.Initrd != nil && found {
		sl.ReportError(spec.Volumes, "volumes", "Volumes", "noRootVolumeIfInitrdSpecified", "")

		return
	}
}
