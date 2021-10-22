package validation

import (
	"fmt"
	"time"

	"github.com/containerd/containerd/reference"
	"github.com/go-playground/validator/v10"
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
