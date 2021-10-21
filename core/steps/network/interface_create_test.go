package network_test

import (
	"context"
	"testing"

	"github.com/golang/mock/gomock"
	g "github.com/onsi/gomega"
	"github.com/weaveworks/flintlock/core/errors"
	"github.com/weaveworks/flintlock/core/models"
	"github.com/weaveworks/flintlock/core/ports"
	"github.com/weaveworks/flintlock/core/steps/network"
	"github.com/weaveworks/flintlock/infrastructure/mock"
)

func TestNewNetworkInterface_everythingIsEmpty(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	g.RegisterTestingT(t)

	var status *models.NetworkInterfaceStatus
	vmid, _ := models.NewVMID("testvm", "testns")
	iface := &models.NetworkInterface{}
	svc := mock.NewMockNetworkService(mockCtrl)
	ctx := context.Background()

	svc.EXPECT().
		IfaceExists(gomock.Eq(ctx), gomock.Eq("testns_testvm_tap")).
		Times(0)

	step := network.NewNetworkInterface(vmid, iface, status, svc)
	shouldDo, err := step.ShouldDo(ctx)

	g.Expect(err).To(g.BeNil())
	g.Expect(shouldDo).To(g.BeTrue())

	_, err = step.Do(ctx)

	g.Expect(err).To(g.MatchError(errors.ErrGuestDeviceNameRequired))
}

func TestNewNetworkInterface_doesNotExist(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	g.RegisterTestingT(t)

	var status *models.NetworkInterfaceStatus
	vmid, _ := models.NewVMID("testvm", "testns")
	iface := &models.NetworkInterface{GuestDeviceName: "eth0"}
	svc := mock.NewMockNetworkService(mockCtrl)
	ctx := context.Background()

	svc.EXPECT().
		IfaceExists(gomock.Eq(ctx), gomock.Eq("testns_testvm_tap")).
		Times(0)

	step := network.NewNetworkInterface(vmid, iface, status, svc)
	shouldDo, err := step.ShouldDo(ctx)

	g.Expect(err).To(g.BeNil())
	g.Expect(shouldDo).To(g.BeTrue())

	svc.EXPECT().
		IfaceExists(gomock.Eq(ctx), gomock.Eq("testns_testvm_tap")).
		Return(false, nil).
		Times(1)

	svc.EXPECT().
		IfaceCreate(gomock.Eq(ctx), gomock.Eq(ports.IfaceCreateInput{
			DeviceName: "testns_testvm_tap",
		})).
		Return(&ports.IfaceDetails{
			DeviceName: "testns_testvm_tap",
			Type:       models.IfaceTypeTap,
			MAC:        "AA:BB:CC:DD:EE:FF",
			Index:      0,
		}, nil).
		Times(1)

	_, err = step.Do(ctx)

	g.Expect(err).To(g.BeNil())
}

func TestNewNetworkInterface_existingInterface(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	g.RegisterTestingT(t)

	vmid, _ := models.NewVMID("testvm", "testns")
	iface := &models.NetworkInterface{
		GuestDeviceName:       "eth0",
		AllowMetadataRequests: false,
		GuestMAC:              "AA:BB:CC:DD:EE:FF",
		Type:                  models.IfaceTypeTap,
	}
	status := &models.NetworkInterfaceStatus{
		HostDeviceName: "testns_testvm_tap",
		Index:          0,
		MACAddress:     "AA:BB:CC:DD:EE:FF",
	}
	svc := mock.NewMockNetworkService(mockCtrl)
	ctx := context.Background()

	svc.EXPECT().
		IfaceExists(gomock.Eq(ctx), gomock.Eq("testns_testvm_tap")).
		Return(true, nil).
		Times(1)

	step := network.NewNetworkInterface(vmid, iface, status, svc)
	shouldDo, err := step.ShouldDo(ctx)

	g.Expect(err).To(g.BeNil())
	g.Expect(shouldDo).To(g.BeFalse())

	svc.EXPECT().
		IfaceExists(gomock.Eq(ctx), gomock.Eq("testns_testvm_tap")).
		Return(true, nil).
		Times(1)

	svc.EXPECT().
		IfaceDetails(gomock.Eq(ctx), gomock.Eq("testns_testvm_tap")).
		Return(&ports.IfaceDetails{
			DeviceName: "testns_testvm_tap",
			Type:       models.IfaceTypeTap,
			MAC:        "AA:BB:CC:DD:EE:FF",
			Index:      0,
		}, nil).
		Times(1)

	_, err = step.Do(ctx)

	g.Expect(err).To(g.BeNil())
}

func TestNewNetworkInterface_missingInterface(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	g.RegisterTestingT(t)

	vmid, _ := models.NewVMID("testvm", "testns")
	iface := &models.NetworkInterface{
		GuestDeviceName:       "eth0",
		AllowMetadataRequests: true,
		GuestMAC:              "AA:BB:CC:DD:EE:FF",
	}
	status := &models.NetworkInterfaceStatus{
		HostDeviceName: "testns_testvm_tap",
		Index:          0,
		MACAddress:     "AA:BB:CC:DD:EE:FF",
	}
	svc := mock.NewMockNetworkService(mockCtrl)
	ctx := context.Background()

	svc.EXPECT().
		IfaceExists(gomock.Eq(ctx), gomock.Eq("testns_testvm_tap")).
		Return(false, nil).
		Times(1)

	step := network.NewNetworkInterface(vmid, iface, status, svc)
	shouldDo, err := step.ShouldDo(ctx)

	g.Expect(err).To(g.BeNil())
	g.Expect(shouldDo).To(g.BeTrue())

	svc.EXPECT().
		IfaceExists(gomock.Eq(ctx), gomock.Eq("testns_testvm_tap")).
		Return(false, nil).
		Times(1)

	svc.EXPECT().
		IfaceCreate(gomock.Eq(ctx), gomock.Eq(ports.IfaceCreateInput{
			DeviceName: "testns_testvm_tap",
			MAC:        "AA:BB:CC:DD:EE:FF",
		})).
		Return(&ports.IfaceDetails{
			DeviceName: "testns_testvm_tap",
			Type:       models.IfaceTypeTap,
			MAC:        "FF:EE:DD:CC:BB:AA",
			Index:      0,
		}, nil).
		Times(1)

	_, err = step.Do(ctx)

	g.Expect(err).To(g.BeNil())
	g.Expect(status.MACAddress).To(g.Equal("FF:EE:DD:CC:BB:AA"))
}

func TestNewNetworkInterface_svcError(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	g.RegisterTestingT(t)

	vmid, _ := models.NewVMID("testvm", "testns")
	iface := &models.NetworkInterface{
		GuestDeviceName:       "eth0",
		AllowMetadataRequests: true,
		GuestMAC:              "AA:BB:CC:DD:EE:FF",
	}
	status := &models.NetworkInterfaceStatus{
		HostDeviceName: "testns_testvm_tap",
		Index:          0,
		MACAddress:     "AA:BB:CC:DD:EE:FF",
	}
	svc := mock.NewMockNetworkService(mockCtrl)
	ctx := context.Background()

	svc.EXPECT().
		IfaceExists(gomock.Eq(ctx), gomock.Eq("testns_testvm_tap")).
		Return(false, errors.ErrParentIfaceRequired).
		Times(2)

	step := network.NewNetworkInterface(vmid, iface, status, svc)
	shouldDo, err := step.ShouldDo(ctx)

	g.Expect(err).ToNot(g.BeNil())
	g.Expect(shouldDo).To(g.BeFalse())

	_, err = step.Do(ctx)

	g.Expect(err).To(g.MatchError(errors.ErrParentIfaceRequired))
}

func TestNewNetworkInterface_fillChangedStatus(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	g.RegisterTestingT(t)

	vmid, _ := models.NewVMID("testvm", "testns")
	iface := &models.NetworkInterface{
		GuestDeviceName:       "eth0",
		AllowMetadataRequests: true,
		GuestMAC:              "AA:BB:CC:DD:EE:FF",
		Type:                  models.IfaceTypeMacvtap,
	}
	status := &models.NetworkInterfaceStatus{
		HostDeviceName: "testns_testvm_tap",
		Index:          0,
		MACAddress:     "AA:BB:CC:DD:EE:FF",
	}
	svc := mock.NewMockNetworkService(mockCtrl)
	ctx := context.Background()

	step := network.NewNetworkInterface(vmid, iface, status, svc)

	svc.EXPECT().
		IfaceExists(gomock.Eq(ctx), gomock.Eq("testns_testvm_vtap")).
		Return(true, nil).
		Times(1)

	svc.EXPECT().
		IfaceDetails(gomock.Eq(ctx), gomock.Eq("testns_testvm_vtap")).
		Return(&ports.IfaceDetails{
			DeviceName: "testns_testvm_vtap",
			Type:       models.IfaceTypeTap,
			MAC:        "FF:EE:DD:CC:BB:AA",
			Index:      0,
		}, nil).
		Times(1)

	_, err := step.Do(ctx)

	g.Expect(err).To(g.BeNil())
	g.Expect(status.MACAddress).To(g.Equal("FF:EE:DD:CC:BB:AA"))
}

func TestNewNetworkInterface_createError(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	g.RegisterTestingT(t)

	var status *models.NetworkInterfaceStatus
	vmid, _ := models.NewVMID("testvm", "testns")
	iface := &models.NetworkInterface{GuestDeviceName: "eth0"}
	svc := mock.NewMockNetworkService(mockCtrl)
	ctx := context.Background()

	svc.EXPECT().
		IfaceExists(gomock.Eq(ctx), gomock.Eq("testns_testvm_tap")).
		Times(0)

	step := network.NewNetworkInterface(vmid, iface, status, svc)
	shouldDo, err := step.ShouldDo(ctx)

	g.Expect(err).To(g.BeNil())
	g.Expect(shouldDo).To(g.BeTrue())

	svc.EXPECT().
		IfaceExists(gomock.Eq(ctx), gomock.Eq("testns_testvm_tap")).
		Return(false, nil).
		Times(1)

	svc.EXPECT().
		IfaceCreate(gomock.Eq(ctx), gomock.Eq(ports.IfaceCreateInput{
			DeviceName: "testns_testvm_tap",
		})).
		Return(nil, errors.ErrParentIfaceRequired).
		Times(1)

	_, err = step.Do(ctx)

	g.Expect(err).To(g.MatchError(errors.ErrParentIfaceRequired))
}
