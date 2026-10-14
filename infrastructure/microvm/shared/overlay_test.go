package shared_test

import (
	"testing"

	g "github.com/onsi/gomega"

	"github.com/liquidmetal-dev/flintlock/core/models"
	"github.com/liquidmetal-dev/flintlock/infrastructure/microvm/shared"
	"github.com/liquidmetal-dev/flintlock/internal/config"
	"github.com/liquidmetal-dev/flintlock/internal/overlayinit"
)

func TestSetOverlayKernelParams(t *testing.T) {
	g.RegisterTestingT(t)

	drives := models.Drives{
		{ID: "root_base", Role: models.DriveRoleRootBase, GuestDevice: "vda", ReadOnly: true},
		{ID: "root_rw", Role: models.DriveRoleRootRW, GuestDevice: "vdb"},
		{ID: "data", Role: models.DriveRoleVolume, GuestDevice: "vdc"},
	}
	cmdLine := config.KernelCmdLine{"console": "ttyS0"}

	shared.SetOverlayKernelParams(cmdLine, drives)

	g.Expect(cmdLine).To(g.Equal(config.KernelCmdLine{
		"console":                 "ttyS0",
		"flintlock.root.contract": "1",
		"flintlock.root.base":     "/dev/vda",
		"flintlock.root.rw":       "/dev/vdb",
	}))

	// The values must parse under the contract the init implements.
	params, err := overlayinit.ParseCmdline(cmdLine.String())
	g.Expect(err).NotTo(g.HaveOccurred())
	g.Expect(params.Base).To(g.Equal("/dev/vda"))
	g.Expect(params.RW).To(g.Equal("/dev/vdb"))
}

func TestSetOverlayKernelParams_ReplacesSpecValues(t *testing.T) {
	g.RegisterTestingT(t)

	drives := models.Drives{
		{Role: models.DriveRoleRootBase, GuestDevice: "vda"},
		{Role: models.DriveRoleRootRW, GuestDevice: "vdb"},
	}
	cmdLine := config.KernelCmdLine{
		"flintlock.root.contract": "9",
		"flintlock.root.base":     "/dev/sda",
		"flintlock.root.rw":       "/dev/sdb",
	}

	shared.SetOverlayKernelParams(cmdLine, drives)

	g.Expect(cmdLine).To(g.Equal(config.KernelCmdLine{
		"flintlock.root.contract": "1",
		"flintlock.root.base":     "/dev/vda",
		"flintlock.root.rw":       "/dev/vdb",
	}))
}

func TestSetOverlayKernelParams_FullModeIsUntouched(t *testing.T) {
	g.RegisterTestingT(t)

	drives := models.Drives{
		{Role: models.DriveRoleRoot, GuestDevice: "vda"},
		{Role: models.DriveRoleVolume, GuestDevice: "vdb"},
	}
	cmdLine := config.KernelCmdLine{"root": "/dev/vda"}

	shared.SetOverlayKernelParams(cmdLine, drives)

	g.Expect(cmdLine).To(g.Equal(config.KernelCmdLine{"root": "/dev/vda"}))
}
