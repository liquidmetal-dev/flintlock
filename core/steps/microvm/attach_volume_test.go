package microvm_test

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	g "github.com/onsi/gomega"

	"github.com/weaveworks-liquidmetal/flintlock/core/models"
	"github.com/weaveworks-liquidmetal/flintlock/core/steps/microvm"
)

func TestAttachVolume_ShouldDo(t *testing.T) {
	testCases := []struct {
		name            string
		existingVolumes []models.Volume
		volPath         string
		volName         string
		insertFirst     bool
		readOnly        bool
		expectShouldDo  bool
	}{
		{
			name:           "everything filled in, no existing volumes",
			volPath:        "/tmp/vol.img",
			volName:        "data",
			insertFirst:    false,
			readOnly:       false,
			expectShouldDo: true,
		},
		{
			name:        "everything filled in, with existing volume which is different",
			volPath:     "/tmp/vol.img",
			volName:     "data",
			insertFirst: false,
			readOnly:    false,
			existingVolumes: []models.Volume{
				{
					ID:         "uservol",
					IsReadOnly: true,
					Source: models.VolumeSource{
						HostPath: &models.HostPathVolumeSource{
							Path: "/another/user.img",
						},
					},
				},
			},
			expectShouldDo: true,
		},
		{
			name:        "everything filled in, with existing volume same id",
			volPath:     "/tmp/vol.img",
			volName:     "data",
			insertFirst: false,
			readOnly:    false,
			existingVolumes: []models.Volume{
				{
					ID:         "data",
					IsReadOnly: false,
					Source: models.VolumeSource{
						HostPath: &models.HostPathVolumeSource{
							Path: "/tmp/vol.img",
						},
					},
				},
			},
			expectShouldDo: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			g.RegisterTestingT(t)

			ctx := context.Background()
			vm := createMicrovm()
			vm.Spec.AdditionalVolumes = tc.existingVolumes

			step := microvm.NewAttachVolumeStep(vm, tc.volPath, tc.volName, tc.insertFirst, tc.readOnly)

			shouldDo, err := step.ShouldDo(ctx)
			g.Expect(err).NotTo(g.HaveOccurred())
			g.Expect(shouldDo).To(g.Equal(tc.expectShouldDo))
		})
	}

}

func TestAttachVolume_Do(t *testing.T) {
	testCases := []struct {
		name            string
		existingVolumes []models.Volume
		volPath         string
		volName         string
		insertFirst     bool
		readOnly        bool
		expectedVolumes []models.Volume
	}{
		{
			name:        "everything filled in, no existing volumes",
			volPath:     "/tmp/vol.img",
			volName:     "data",
			insertFirst: false,
			readOnly:    false,
			expectedVolumes: []models.Volume{
				{
					ID:         "data",
					IsReadOnly: false,
					Source: models.VolumeSource{
						HostPath: &models.HostPathVolumeSource{
							Path: "/tmp/vol.img",
						},
					},
				},
			},
		},
		{
			name:        "everything filled in, with existing volume which is different",
			volPath:     "/tmp/vol.img",
			volName:     "data",
			insertFirst: false,
			readOnly:    false,
			existingVolumes: []models.Volume{
				{
					ID:         "uservol",
					IsReadOnly: true,
					Source: models.VolumeSource{
						HostPath: &models.HostPathVolumeSource{
							Path: "/another/user.img",
						},
					},
				},
			},
			expectedVolumes: []models.Volume{
				{
					ID:         "uservol",
					IsReadOnly: true,
					Source: models.VolumeSource{
						HostPath: &models.HostPathVolumeSource{
							Path: "/another/user.img",
						},
					},
				},
				{
					ID:         "data",
					IsReadOnly: false,
					Source: models.VolumeSource{
						HostPath: &models.HostPathVolumeSource{
							Path: "/tmp/vol.img",
						},
					},
				},
			},
		},
		{
			name:        "everything filled in, with existing volume which is different, insert first",
			volPath:     "/tmp/vol.img",
			volName:     "data",
			insertFirst: true,
			readOnly:    false,
			existingVolumes: []models.Volume{
				{
					ID:         "uservol",
					IsReadOnly: true,
					Source: models.VolumeSource{
						HostPath: &models.HostPathVolumeSource{
							Path: "/another/user.img",
						},
					},
				},
			},
			expectedVolumes: []models.Volume{
				{
					ID:         "data",
					IsReadOnly: false,
					Source: models.VolumeSource{
						HostPath: &models.HostPathVolumeSource{
							Path: "/tmp/vol.img",
						},
					},
				},
				{
					ID:         "uservol",
					IsReadOnly: true,
					Source: models.VolumeSource{
						HostPath: &models.HostPathVolumeSource{
							Path: "/another/user.img",
						},
					},
				},
			},
		},
		{
			name:        "everything filled in, with existing volume which is different, readonly",
			volPath:     "/tmp/vol.img",
			volName:     "data",
			insertFirst: false,
			readOnly:    true,
			existingVolumes: []models.Volume{
				{
					ID:         "uservol",
					IsReadOnly: true,
					Source: models.VolumeSource{
						HostPath: &models.HostPathVolumeSource{
							Path: "/another/user.img",
						},
					},
				},
			},
			expectedVolumes: []models.Volume{
				{
					ID:         "uservol",
					IsReadOnly: true,
					Source: models.VolumeSource{
						HostPath: &models.HostPathVolumeSource{
							Path: "/another/user.img",
						},
					},
				},
				{
					ID:         "data",
					IsReadOnly: true,
					Source: models.VolumeSource{
						HostPath: &models.HostPathVolumeSource{
							Path: "/tmp/vol.img",
						},
					},
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			g.RegisterTestingT(t)

			ctx := context.Background()
			vm := createMicrovm()
			vm.Spec.AdditionalVolumes = tc.existingVolumes

			step := microvm.NewAttachVolumeStep(vm, tc.volPath, tc.volName, tc.insertFirst, tc.readOnly)

			childSteps, err := step.Do(ctx)
			g.Expect(err).NotTo(g.HaveOccurred())
			g.Expect(childSteps).To(g.HaveLen(0))

			g.Expect(vm.Spec.AdditionalVolumes).To(g.HaveLen(len(tc.expectedVolumes)))
			for i, expected := range tc.expectedVolumes {
				actualVol := vm.Spec.AdditionalVolumes[i]
				volsEqual := cmp.Equal(expected, actualVol)
				g.Expect(volsEqual).To(g.BeTrue())
			}

		})
	}

}

func createMicrovm() *models.MicroVM {
	vmid, _ := models.NewVMID("vm", "ns", "uid")
	return &models.MicroVM{
		ID:      *vmid,
		Version: 1,
		Spec: models.MicroVMSpec{
			Metadata: models.Metadata{
				Items:     map[string]string{},
				AddVolume: false,
			},
		},
		Status: models.MicroVMStatus{},
	}
}
