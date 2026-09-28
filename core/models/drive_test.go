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
