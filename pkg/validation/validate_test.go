package validation

import (
	"testing"
	"time"

	"github.com/go-playground/validator/v10"
	. "github.com/onsi/gomega"
	"github.com/weaveworks/reignite/core/models"
)

func TestValidation_Valid(t *testing.T) {
	RegisterTestingT(t)
	val := NewValidator()

	err := val.ValidateStruct(basicMicroVM)

	Expect(err).NotTo(HaveOccurred())
}

func TestValidation_Invalid(t *testing.T) {
	invalidImageUri := basicMicroVM
	invalidImageUri.Spec.Kernel.Image = "://invalidImage@"

	invalidUnixTimestamp := basicMicroVM
	invalidUnixTimestamp.Spec.CreatedAt = time.Now().Add(1 * time.Hour).Unix()
	invalidUnixTimestamp.Spec.UpdatedAt = time.Now().Add(1 * time.Hour).Unix()
	invalidUnixTimestamp.Spec.DeletedAt = time.Now().Add(1 * time.Hour).Unix()

	tt := []struct {
		name      string
		numErrors int
		vmspec    models.MicroVM
	}{
		{
			name:      "nil spec should fail validation with 5 errors",
			numErrors: 5,
			vmspec:    models.MicroVM{},
		},
		{
			name:      "invalid image URI should fail validation",
			numErrors: 1,
			vmspec:    invalidImageUri,
		},
		{
			name:      "unix timestamps in future should fail validation",
			numErrors: 3,
			vmspec:    invalidUnixTimestamp,
		},
	}

	val := NewValidator()

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			RegisterTestingT(t)

			err := val.ValidateStruct(tc.vmspec)
			Expect(err).To(HaveOccurred())

			valErrors, ok := err.(validator.ValidationErrors)
			Expect(ok).Should(Equal(true))
			// validator.ValidationErrors is an alias for an array of validator.FieldError.
			Expect(len(valErrors)).Should(Equal(tc.numErrors))
		})
	}
}

var basicMicroVM = models.MicroVM{
	Spec: models.MicroVMSpec{
		VCPU:       2,
		MemoryInMb: 2048,
		NetworkInterfaces: []models.NetworkInterface{
			{
				GuestDeviceName: "eth0",
			},
		},
		CreatedAt: time.Now().Add(-100 * time.Second).Unix(),
		Kernel: models.Kernel{
			Image:    "docker.io/richardcase/ubuntu-bionic-kernel:0.0.11",
			Filename: "vmlinux",
		},
	},
}
