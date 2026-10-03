package shared_test

import (
	"testing"

	g "github.com/onsi/gomega"

	"github.com/liquidmetal-dev/flintlock/core/errors"
	"github.com/liquidmetal-dev/flintlock/core/models"
	"github.com/liquidmetal-dev/flintlock/infrastructure/microvm/shared"
)

func containerVolume(id string, readOnly bool) models.Volume {
	return models.Volume{
		ID:         id,
		IsReadOnly: readOnly,
		Source: models.VolumeSource{
			Container: &models.ContainerVolumeSource{Image: "image:tag"},
		},
	}
}

func virtioFSVolume(id string) models.Volume {
	return models.Volume{
		ID:     id,
		Source: models.VolumeSource{VirtioFS: &models.VirtioFSVolumeSource{Path: "/shared"}},
	}
}

func vmWithVolumes(root models.Volume, additional ...models.Volume) *models.MicroVM {
	vm := &models.MicroVM{
		Spec: models.MicroVMSpec{
			RootVolume:        root,
			AdditionalVolumes: additional,
		},
		Status: models.MicroVMStatus{
			Volumes: models.VolumeStatuses{
				root.ID: &models.VolumeStatus{Mount: models.Mount{Source: "/" + root.ID + ".img"}},
			},
		},
	}

	for _, vol := range additional {
		vm.Status.Volumes[vol.ID] = &models.VolumeStatus{Mount: models.Mount{Source: "/" + vol.ID + ".img"}}
	}

	return vm
}

func TestBuildDrives_RootThenAdditional(t *testing.T) {
	g.RegisterTestingT(t)

	vm := vmWithVolumes(containerVolume("root", false), containerVolume("data", false), containerVolume("logs", true))

	drives, err := shared.BuildDrives(vm, shared.DriveOptions{})

	g.Expect(err).NotTo(g.HaveOccurred())
	g.Expect(drives).To(g.Equal(models.Drives{
		{ID: "root", Role: models.DriveRoleRoot, VolumeID: "root", GuestDevice: "vda", Path: "/root.img"},
		{ID: "data", Role: models.DriveRoleVolume, VolumeID: "data", GuestDevice: "vdb", Path: "/data.img"},
		{ID: "logs", Role: models.DriveRoleVolume, VolumeID: "logs", GuestDevice: "vdc", ReadOnly: true, Path: "/logs.img"},
	}))
}

func TestBuildDrives_CloudInitImageIsSecond(t *testing.T) {
	g.RegisterTestingT(t)

	vm := vmWithVolumes(containerVolume("root", false), containerVolume("data", false))

	drives, err := shared.BuildDrives(vm, shared.DriveOptions{CloudInitImage: "/state/cloud-init.img"})

	g.Expect(err).NotTo(g.HaveOccurred())
	g.Expect(drives).To(g.Equal(models.Drives{
		{ID: "root", Role: models.DriveRoleRoot, VolumeID: "root", GuestDevice: "vda", Path: "/root.img"},
		{
			ID:          shared.CloudInitDriveID,
			Role:        models.DriveRoleCloudInit,
			GuestDevice: "vdb",
			ReadOnly:    true,
			Path:        "/state/cloud-init.img",
		},
		{ID: "data", Role: models.DriveRoleVolume, VolumeID: "data", GuestDevice: "vdc", Path: "/data.img"},
	}))
}

func TestBuildDrives_VirtioFSIsNotADrive(t *testing.T) {
	g.RegisterTestingT(t)

	vm := vmWithVolumes(containerVolume("root", false), virtioFSVolume("shared"), containerVolume("data", false))

	drives, err := shared.BuildDrives(vm, shared.DriveOptions{})

	g.Expect(err).NotTo(g.HaveOccurred())
	g.Expect(drives).To(g.HaveLen(2))
	g.Expect(drives[1].VolumeID).To(g.Equal("data"))
	g.Expect(drives[1].GuestDevice).To(g.Equal("vdb"))
}

func TestBuildDrives_ReadOnlyRoot(t *testing.T) {
	g.RegisterTestingT(t)

	vm := vmWithVolumes(containerVolume("root", true))

	drives, err := shared.BuildDrives(vm, shared.DriveOptions{})

	g.Expect(err).NotTo(g.HaveOccurred())
	g.Expect(drives[0].ReadOnly).To(g.BeTrue())
}

func TestBuildDrives_VolumeNotMountedHasNoPath(t *testing.T) {
	g.RegisterTestingT(t)

	vm := vmWithVolumes(containerVolume("root", false), containerVolume("data", false))
	delete(vm.Status.Volumes, "data")
	vm.Status.Volumes["root"] = nil

	drives, err := shared.BuildDrives(vm, shared.DriveOptions{})

	g.Expect(err).NotTo(g.HaveOccurred())
	g.Expect(drives).To(g.HaveLen(2))
	g.Expect(drives[0].Path).To(g.BeEmpty())
	g.Expect(drives[1].Path).To(g.BeEmpty())
	g.Expect(drives[1].GuestDevice).To(g.Equal("vdb"))
}

func TestBuildDrives_NoStatusAtAll(t *testing.T) {
	g.RegisterTestingT(t)

	vm := &models.MicroVM{Spec: models.MicroVMSpec{RootVolume: containerVolume("root", false)}}

	drives, err := shared.BuildDrives(vm, shared.DriveOptions{})

	g.Expect(err).NotTo(g.HaveOccurred())
	g.Expect(drives).To(g.HaveLen(1))
	g.Expect(drives[0].Path).To(g.BeEmpty())
}

func TestBuildDrives_NilMicroVM(t *testing.T) {
	g.RegisterTestingT(t)

	_, err := shared.BuildDrives(nil, shared.DriveOptions{})

	g.Expect(err).To(g.MatchError(errors.ErrSpecRequired))
}

func TestGuestDeviceName(t *testing.T) {
	g.RegisterTestingT(t)

	cases := map[int]string{
		0:   "vda",
		1:   "vdb",
		25:  "vdz",
		26:  "vdaa",
		27:  "vdab",
		51:  "vdaz",
		52:  "vdba",
		701: "vdzz",
		702: "vdaaa",
		-1:  "",
	}

	for index, want := range cases {
		g.Expect(shared.GuestDeviceName(index)).To(g.Equal(want), "index %d", index)
	}
}
