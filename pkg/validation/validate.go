package validation

import (
	"fmt"
	"regexp"
	"time"

	"github.com/containerd/containerd/reference"
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
}
