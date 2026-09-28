package cloudinit_test

import (
	"context"
	"encoding/base64"
	"errors"
	"testing"

	"github.com/golang/mock/gomock"
	g "github.com/onsi/gomega"
	"gopkg.in/yaml.v2"

	"github.com/liquidmetal-dev/flintlock/client/cloudinit"
	"github.com/liquidmetal-dev/flintlock/client/cloudinit/userdata"
	"github.com/liquidmetal-dev/flintlock/core/models"
	cloudinitstep "github.com/liquidmetal-dev/flintlock/core/steps/cloudinit"
	"github.com/liquidmetal-dev/flintlock/infrastructure/mock"
)

func containerVolume(id, mountPoint string) models.Volume {
	return models.Volume{
		ID:         id,
		MountPoint: mountPoint,
		Source: models.VolumeSource{
			Container: &models.ContainerVolumeSource{Image: "image:tag"},
		},
	}
}

func virtioFSVolume(id, mountPoint string) models.Volume {
	return models.Volume{
		ID:         id,
		MountPoint: mountPoint,
		Source:     models.VolumeSource{VirtioFS: &models.VirtioFSVolumeSource{Path: "/shared"}},
	}
}

func testVM(volumes ...models.Volume) *models.MicroVM {
	vmid, _ := models.NewVMID("vm", "ns", "uid")

	vm := &models.MicroVM{
		ID: *vmid,
		Spec: models.MicroVMSpec{
			RootVolume:        models.Volume{ID: "root"},
			AdditionalVolumes: volumes,
		},
		Status: models.MicroVMStatus{
			Volumes: models.VolumeStatuses{
				"root": &models.VolumeStatus{Mount: models.Mount{Source: "/root.img"}},
			},
		},
	}

	for _, vol := range volumes {
		vm.Status.Volumes[vol.ID] = &models.VolumeStatus{Mount: models.Mount{Source: "/" + vol.ID + ".img"}}
	}

	return vm
}

func setVendorData(t *testing.T, vm *models.MicroVM, vendorData *userdata.UserData) {
	t.Helper()

	data, err := yaml.Marshal(vendorData)
	g.Expect(err).NotTo(g.HaveOccurred())

	vm.Spec.Metadata = map[string]string{
		cloudinit.VendorDataKey: base64.StdEncoding.EncodeToString(data),
	}
}

func mounts(t *testing.T, vm *models.MicroVM) []userdata.Mount {
	t.Helper()

	raw, ok := vm.Spec.Metadata[cloudinit.VendorDataKey]
	g.Expect(ok).To(g.BeTrue(), "vendor data is not set")

	data, err := base64.StdEncoding.DecodeString(raw)
	g.Expect(err).NotTo(g.HaveOccurred())

	vendorData := &userdata.UserData{}
	g.Expect(yaml.Unmarshal(data, vendorData)).To(g.Succeed())

	return vendorData.Mounts
}

func TestDiskMount_UsesDeviceNamesFromProvider(t *testing.T) {
	g.RegisterTestingT(t)

	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	ctx := context.Background()
	vm := testVM(containerVolume("data", "/data"), containerVolume("logs", "/logs"))

	provider := mock.NewMockMicroVMService(mockCtrl)
	provider.EXPECT().Drives(vm).Return(models.Drives{
		{ID: "root", Role: models.DriveRoleRoot, VolumeID: "root", GuestDevice: "vda"},
		{ID: "cloud-init", Role: models.DriveRoleCloudInit, GuestDevice: "vdb"},
		{ID: "data", Role: models.DriveRoleVolume, VolumeID: "data", GuestDevice: "vdc"},
		{ID: "logs", Role: models.DriveRoleVolume, VolumeID: "logs", GuestDevice: "vdd"},
	}, nil)

	step := cloudinitstep.NewDiskMountStep(vm, provider)

	should, err := step.ShouldDo(ctx)
	g.Expect(err).NotTo(g.HaveOccurred())
	g.Expect(should).To(g.BeTrue())

	_, err = step.Do(ctx)
	g.Expect(err).NotTo(g.HaveOccurred())
	g.Expect(mounts(t, vm)).To(g.Equal([]userdata.Mount{
		{"vdc", "/data"},
		{"vdd", "/logs"},
	}))

	should, err = step.ShouldDo(ctx)
	g.Expect(err).NotTo(g.HaveOccurred())
	g.Expect(should).To(g.BeFalse())
}

func TestDiskMount_SkipsVirtioFS(t *testing.T) {
	g.RegisterTestingT(t)

	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	ctx := context.Background()
	vm := testVM(virtioFSVolume("shared", "/shared"), containerVolume("data", "/data"))

	provider := mock.NewMockMicroVMService(mockCtrl)
	provider.EXPECT().Drives(vm).Return(models.Drives{
		{ID: "root", Role: models.DriveRoleRoot, VolumeID: "root", GuestDevice: "vda"},
		{ID: "data", Role: models.DriveRoleVolume, VolumeID: "data", GuestDevice: "vdb"},
	}, nil)

	step := cloudinitstep.NewDiskMountStep(vm, provider)

	_, err := step.Do(ctx)
	g.Expect(err).NotTo(g.HaveOccurred())
	g.Expect(mounts(t, vm)).To(g.Equal([]userdata.Mount{{"vdb", "/data"}}))

	// The virtiofs volume has a mount point and no entry. The step must not
	// ask to run again because of it, or the plan never finishes.
	should, err := step.ShouldDo(ctx)
	g.Expect(err).NotTo(g.HaveOccurred())
	g.Expect(should).To(g.BeFalse())
}

func TestDiskMount_OnlyVirtioFS(t *testing.T) {
	g.RegisterTestingT(t)

	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	vm := testVM(virtioFSVolume("shared", "/shared"))

	// No call is expected on the provider: gomock fails the test if one is made.
	step := cloudinitstep.NewDiskMountStep(vm, mock.NewMockMicroVMService(mockCtrl))

	should, err := step.ShouldDo(context.Background())
	g.Expect(err).NotTo(g.HaveOccurred())
	g.Expect(should).To(g.BeFalse())
}

func TestDiskMount_KeepsExistingMount(t *testing.T) {
	g.RegisterTestingT(t)

	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	ctx := context.Background()
	vm := testVM(containerVolume("data", "/data"))

	// A microvm created before the device names came from the provider.
	setVendorData(t, vm, &userdata.UserData{Mounts: []userdata.Mount{{"vdb", "/data"}}})

	provider := mock.NewMockMicroVMService(mockCtrl)
	provider.EXPECT().Drives(vm).Return(models.Drives{
		{ID: "root", Role: models.DriveRoleRoot, VolumeID: "root", GuestDevice: "vda"},
		{ID: "cloud-init", Role: models.DriveRoleCloudInit, GuestDevice: "vdb"},
		{ID: "data", Role: models.DriveRoleVolume, VolumeID: "data", GuestDevice: "vdc"},
	}, nil)

	step := cloudinitstep.NewDiskMountStep(vm, provider)

	should, err := step.ShouldDo(ctx)
	g.Expect(err).NotTo(g.HaveOccurred())
	g.Expect(should).To(g.BeFalse())

	_, err = step.Do(ctx)
	g.Expect(err).NotTo(g.HaveOccurred())
	g.Expect(mounts(t, vm)).To(g.Equal([]userdata.Mount{{"vdb", "/data"}}))
}

func TestDiskMount_VolumeWithoutMountPoint(t *testing.T) {
	g.RegisterTestingT(t)

	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	ctx := context.Background()
	vm := testVM(containerVolume("scratch", ""), containerVolume("data", "/data"))

	provider := mock.NewMockMicroVMService(mockCtrl)
	provider.EXPECT().Drives(vm).Return(models.Drives{
		{ID: "root", Role: models.DriveRoleRoot, VolumeID: "root", GuestDevice: "vda"},
		{ID: "scratch", Role: models.DriveRoleVolume, VolumeID: "scratch", GuestDevice: "vdb"},
		{ID: "data", Role: models.DriveRoleVolume, VolumeID: "data", GuestDevice: "vdc"},
	}, nil)

	step := cloudinitstep.NewDiskMountStep(vm, provider)

	_, err := step.Do(ctx)
	g.Expect(err).NotTo(g.HaveOccurred())
	g.Expect(mounts(t, vm)).To(g.Equal([]userdata.Mount{{"vdc", "/data"}}))
}

func TestDiskMount_ProviderFails(t *testing.T) {
	g.RegisterTestingT(t)

	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	vm := testVM(containerVolume("data", "/data"))
	errDrives := errors.New("no drives")

	provider := mock.NewMockMicroVMService(mockCtrl)
	provider.EXPECT().Drives(vm).Return(nil, errDrives)

	step := cloudinitstep.NewDiskMountStep(vm, provider)

	_, err := step.Do(context.Background())
	g.Expect(err).To(g.MatchError(errDrives))
	g.Expect(vm.Spec.Metadata).NotTo(g.HaveKey(cloudinit.VendorDataKey))
}

func TestDiskMount_ProviderHasNoDriveForVolume(t *testing.T) {
	g.RegisterTestingT(t)

	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	vm := testVM(containerVolume("data", "/data"))

	provider := mock.NewMockMicroVMService(mockCtrl)
	provider.EXPECT().Drives(vm).Return(models.Drives{
		{ID: "root", Role: models.DriveRoleRoot, VolumeID: "root", GuestDevice: "vda"},
	}, nil)

	step := cloudinitstep.NewDiskMountStep(vm, provider)

	_, err := step.Do(context.Background())
	g.Expect(err).To(g.MatchError(g.ContainSubstring("data")))
}
