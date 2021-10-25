package validation

import (
	"errors"
	"testing"
	"time"

	"github.com/go-playground/validator/v10"
	. "github.com/onsi/gomega"
	"github.com/weaveworks/flintlock/core/models"
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

	invalidNetworkGuestDeviceName := basicMicroVM
	invalidNetworkGuestDeviceName.Spec.NetworkInterfaces = []models.NetworkInterface{
		{
			GuestDeviceName: "!eth0",
			GuestMAC:        "F5:C3:19:56:01:F4",
			Address:         "192.168.1.1/32",
			Type:            "macvtap",
		},
	}

	invalidVolumes := basicMicroVM
	invalidVolumes.Spec.Volumes = models.Volumes{
		{
			IsRoot:     false,
			MountPoint: "/",
		},
	}

	tt := []struct {
		name      string
		numErrors int
		vmspec    models.MicroVM
	}{
		{
			name:      "nil spec should fail validation with 5 errors",
			numErrors: 7,
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
		{
			name:      "invalid guest device name should fail validation",
			numErrors: 1,
			vmspec:    invalidNetworkGuestDeviceName,
		},
		{
			name:      "should fail validation when no volumes are marked as root",
			numErrors: 1,
			vmspec:    invalidVolumes,
		},
	}

	val := NewValidator()

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			RegisterTestingT(t)

			err := val.ValidateStruct(tc.vmspec)
			Expect(err).To(HaveOccurred())

			var valErrors validator.ValidationErrors

			ok := errors.As(err, &valErrors)
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
				GuestMAC:        "F5:C3:19:56:01:F4",
				Address:         "192.168.1.1/32",
				Type:            "macvtap",
			},
		},
		CreatedAt: time.Now().Add(-100 * time.Second).Unix(),
		Kernel: models.Kernel{
			Image:    "docker.io/richardcase/ubuntu-bionic-kernel:0.0.11",
			Filename: "vmlinux",
		},
		Volumes: models.Volumes{
			{
				IsRoot:     true,
				MountPoint: "/",
			},
		},
	},
}
