package models_test

import (
	"testing"

	g "github.com/onsi/gomega"

	"github.com/liquidmetal-dev/flintlock/core/models"
)

func TestDrives_GuestDeviceForVolume(t *testing.T) {
	g.RegisterTestingT(t)

	drives := models.Drives{
		{ID: "root", Role: models.DriveRoleRoot, VolumeID: "root", GuestDevice: "vda"},
		{ID: "cloud-init", Role: models.DriveRoleCloudInit, GuestDevice: "vdb"},
		{ID: "data", Role: models.DriveRoleVolume, VolumeID: "data", GuestDevice: "vdc"},
	}

	device, found := drives.GuestDeviceForVolume("data")
	g.Expect(found).To(g.BeTrue())
	g.Expect(device).To(g.Equal("vdc"))
}

func TestDrives_GuestDeviceForVolume_NotADrive(t *testing.T) {
	g.RegisterTestingT(t)

	drives := models.Drives{
		{ID: "root", Role: models.DriveRoleRoot, VolumeID: "root", GuestDevice: "vda"},
	}

	device, found := drives.GuestDeviceForVolume("shared")
	g.Expect(found).To(g.BeFalse())
	g.Expect(device).To(g.BeEmpty())
}

func TestDrives_GuestDeviceForVolume_IgnoresRoot(t *testing.T) {
	g.RegisterTestingT(t)

	drives := models.Drives{
		{ID: "root", Role: models.DriveRoleRoot, VolumeID: "root", GuestDevice: "vda"},
	}

	_, found := drives.GuestDeviceForVolume("root")
	g.Expect(found).To(g.BeFalse())
}

func TestDrives_ByRole(t *testing.T) {
	g.RegisterTestingT(t)

	drives := models.Drives{
		{ID: "root_base", Role: models.DriveRoleRootBase, VolumeID: "root", GuestDevice: "vda", ReadOnly: true},
		{ID: "root_rw", Role: models.DriveRoleRootRW, VolumeID: "root", GuestDevice: "vdb"},
		{ID: "data", Role: models.DriveRoleVolume, VolumeID: "data", GuestDevice: "vdc"},
	}

	drive, found := drives.ByRole(models.DriveRoleRootRW)
	g.Expect(found).To(g.BeTrue())
	g.Expect(drive.GuestDevice).To(g.Equal("vdb"))

	_, found = drives.ByRole(models.DriveRoleRoot)
	g.Expect(found).To(g.BeFalse())
}
