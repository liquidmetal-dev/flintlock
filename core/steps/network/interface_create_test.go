package network_test

import (
	"context"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/liquidmetal-dev/flintlock/core/errors"
	"github.com/liquidmetal-dev/flintlock/core/models"
	"github.com/liquidmetal-dev/flintlock/core/ports"
	"github.com/liquidmetal-dev/flintlock/core/steps/network"
	"github.com/liquidmetal-dev/flintlock/infrastructure/mock"
	g "github.com/onsi/gomega"
)

func TestNewNetworkInterface_everythingIsEmpty(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	g.RegisterTestingT(t)

	var status *models.NetworkInterfaceStatus
	vmid, _ := models.NewVMID(vmName, nsName, vmUID)
	iface := &models.NetworkInterface{}
	svc := mock.NewMockNetworkService(mockCtrl)
	ctx := context.Background()

	svc.EXPECT().
		IfaceExists(gomock.Eq(ctx), gomock.Eq(expectedTapDeviceName)).
		Times(0)

	step := network.NewNetworkInterface(vmid, iface, status, svc)

	shouldDo, err := step.ShouldDo(ctx)
	g.Expect(err).To(g.BeNil())
	g.Expect(shouldDo).To(g.BeTrue())

	_, err = step.Do(ctx)
	g.Expect(err).To(g.MatchError(errors.ErrGuestDeviceNameRequired))

	verifyErr := step.Verify(ctx)
	g.Expect(verifyErr).To(g.BeNil())
}

func TestNewNetworkInterface_doesNotExist(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	g.RegisterTestingT(t)

	vmid, _ := models.NewVMID(vmName, nsName, vmUID)
	status := &models.NetworkInterfaceStatus{}
	iface := &models.NetworkInterface{
		GuestDeviceName: defaultEthDevice,
		Type:            models.IfaceTypeTap,
	}
	svc := mock.NewMockNetworkService(mockCtrl)
	ctx := context.Background()

	svc.EXPECT().
		IfaceExists(gomock.Eq(ctx), gomock.Eq(expectedTapDeviceName)).
		Times(0)

	step := network.NewNetworkInterface(vmid, iface, status, svc)
	shouldDo, err := step.ShouldDo(ctx)

	g.Expect(err).To(g.BeNil())
	g.Expect(shouldDo).To(g.BeTrue())

	svc.EXPECT().
		IfaceExists(gomock.Eq(ctx), &hostDeviceNameMatcher{}).
		Return(false, nil).
		Times(1)

	svc.EXPECT().
		IfaceCreate(gomock.Eq(ctx), &ifaceCreateInputMatcher{}).
		Return(&ports.IfaceDetails{
			DeviceName: expectedTapDeviceName,
			Type:       models.IfaceTypeTap,
			MAC:        defaultMACAddress,
			Index:      0,
		}, nil).
		Times(1)

	_, err = step.Do(ctx)
	g.Expect(err).To(g.BeNil())

	verifyErr := step.Verify(ctx)
	g.Expect(verifyErr).To(g.BeNil())
}

func TestNewNetworkInterface_emptyStatus(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	g.RegisterTestingT(t)

	var status *models.NetworkInterfaceStatus
	vmid, _ := models.NewVMID(vmName, nsName, vmUID)
	iface := &models.NetworkInterface{GuestDeviceName: defaultEthDevice}
	svc := mock.NewMockNetworkService(mockCtrl)
	ctx := context.Background()

	svc.EXPECT().
		IfaceExists(gomock.Eq(ctx), gomock.Any()).
		Times(0)

	step := network.NewNetworkInterface(vmid, iface, status, svc)
	shouldDo, err := step.ShouldDo(ctx)

	g.Expect(err).To(g.BeNil())
	g.Expect(shouldDo).To(g.BeTrue())

	svc.EXPECT().
		IfaceExists(gomock.Eq(ctx), gomock.Any()).
		Times(0)

	svc.EXPECT().
		IfaceCreate(gomock.Eq(ctx), gomock.Any()).
		Times(0)

	_, err = step.Do(ctx)
	g.Expect(err).ToNot(g.BeNil())

	verifyErr := step.Verify(ctx)
	g.Expect(verifyErr).To(g.BeNil())
}

func TestNewNetworkInterface_existingInterface(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	g.RegisterTestingT(t)

	vmid, _ := models.NewVMID(vmName, nsName, vmUID)
	iface := &models.NetworkInterface{
		GuestDeviceName:       defaultEthDevice,
		AllowMetadataRequests: false,
		GuestMAC:              defaultMACAddress,
		Type:                  models.IfaceTypeTap,
	}
	status := &models.NetworkInterfaceStatus{
		HostDeviceName: expectedTapDeviceName,
		Index:          0,
		MACAddress:     defaultMACAddress,
	}
	svc := mock.NewMockNetworkService(mockCtrl)
	ctx := context.Background()

	svc.EXPECT().
		IfaceExists(gomock.Eq(ctx), gomock.Eq(expectedTapDeviceName)).
		Return(true, nil).
		Times(1)

	step := network.NewNetworkInterface(vmid, iface, status, svc)
	shouldDo, err := step.ShouldDo(ctx)

	g.Expect(err).To(g.BeNil())
	g.Expect(shouldDo).To(g.BeFalse())

	svc.EXPECT().
		IfaceExists(gomock.Eq(ctx), gomock.Eq(expectedTapDeviceName)).
		Return(true, nil).
		Times(1)

	svc.EXPECT().
		IfaceDetails(gomock.Eq(ctx), gomock.Eq(expectedTapDeviceName)).
		Return(&ports.IfaceDetails{
			DeviceName: expectedTapDeviceName,
			Type:       models.IfaceTypeTap,
			MAC:        defaultMACAddress,
			Index:      0,
		}, nil).
		Times(1)

	_, err = step.Do(ctx)
	g.Expect(err).To(g.BeNil())

	verifyErr := step.Verify(ctx)
	g.Expect(verifyErr).To(g.BeNil())
}

func TestNewNetworkInterface_missingInterface(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	g.RegisterTestingT(t)

	vmid, _ := models.NewVMID(vmName, nsName, vmUID)
	iface, status := fullNetworkInterface()
	svc := mock.NewMockNetworkService(mockCtrl)
	ctx := context.Background()

	svc.EXPECT().
		IfaceExists(gomock.Eq(ctx), gomock.Eq(expectedTapDeviceName)).
		Return(false, nil).
		Times(1)

	step := network.NewNetworkInterface(vmid, iface, status, svc)
	shouldDo, err := step.ShouldDo(ctx)

	g.Expect(err).To(g.BeNil())
	g.Expect(shouldDo).To(g.BeTrue())

	svc.EXPECT().
		IfaceExists(gomock.Eq(ctx), gomock.Eq(expectedTapDeviceName)).
		Return(false, nil).
		Times(1)

	svc.EXPECT().
		IfaceCreate(gomock.Eq(ctx), gomock.Eq(ports.IfaceCreateInput{
			DeviceName: expectedTapDeviceName,
			MAC:        defaultMACAddress,
			Attach:     true,
		})).
		Return(&ports.IfaceDetails{
			DeviceName: expectedTapDeviceName,
			Type:       models.IfaceTypeTap,
			MAC:        reverseMACAddress,
			Index:      0,
		}, nil).
		Times(1)

	_, err = step.Do(ctx)

	g.Expect(err).To(g.BeNil())
	g.Expect(status.MACAddress).To(g.Equal(reverseMACAddress))
}

func TestNewNetworkInterface_svcError(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	g.RegisterTestingT(t)

	vmid, _ := models.NewVMID(vmName, nsName, vmUID)
	iface, status := fullNetworkInterface()
	svc := mock.NewMockNetworkService(mockCtrl)
	ctx := context.Background()

	svc.EXPECT().
		IfaceExists(gomock.Eq(ctx), gomock.Eq(expectedTapDeviceName)).
		Return(false, errors.ErrParentIfaceRequiredForAttachingTap).
		Times(2)

	step := network.NewNetworkInterface(vmid, iface, status, svc)
	shouldDo, err := step.ShouldDo(ctx)

	g.Expect(err).ToNot(g.BeNil())
	g.Expect(shouldDo).To(g.BeFalse())

	_, err = step.Do(ctx)
	g.Expect(err).To(g.MatchError(errors.ErrParentIfaceRequiredForAttachingTap))

	verifyErr := step.Verify(ctx)
	g.Expect(verifyErr).To(g.BeNil())
}

func TestNewNetworkInterface_fillChangedStatus(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	g.RegisterTestingT(t)

	vmid, _ := models.NewVMID(vmName, nsName, vmUID)
	iface, status := fullNetworkInterface()
	iface.Type = models.IfaceTypeMacvtap
	svc := mock.NewMockNetworkService(mockCtrl)
	ctx := context.Background()

	step := network.NewNetworkInterface(vmid, iface, status, svc)

	svc.EXPECT().
		IfaceExists(gomock.Eq(ctx), gomock.Eq(expectedTapDeviceName)).
		Return(true, nil).
		Times(1)

	svc.EXPECT().
		IfaceDetails(gomock.Eq(ctx), gomock.Eq(expectedTapDeviceName)).
		Return(&ports.IfaceDetails{
			DeviceName: expectedTapDeviceName,
			Type:       models.IfaceTypeMacvtap,
			MAC:        reverseMACAddress,
			Index:      0,
		}, nil).
		Times(1)

	_, err := step.Do(ctx)
	g.Expect(err).To(g.BeNil())
	g.Expect(status.MACAddress).To(g.Equal(reverseMACAddress))
	g.Expect(status.HostDeviceName).To(g.Equal(expectedTapDeviceName))

	verifyErr := step.Verify(ctx)
	g.Expect(verifyErr).To(g.BeNil())
}

func TestNewNetworkInterface_createError(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	g.RegisterTestingT(t)

	vmid, _ := models.NewVMID(vmName, nsName, vmUID)
	status := &models.NetworkInterfaceStatus{}
	iface := &models.NetworkInterface{GuestDeviceName: defaultEthDevice, Type: models.IfaceTypeTap}
	svc := mock.NewMockNetworkService(mockCtrl)
	ctx := context.Background()

	svc.EXPECT().
		IfaceExists(gomock.Eq(ctx), gomock.Eq(expectedTapDeviceName)).
		Times(0)

	step := network.NewNetworkInterface(vmid, iface, status, svc)

	shouldDo, err := step.ShouldDo(ctx)
	g.Expect(err).To(g.BeNil())
	g.Expect(shouldDo).To(g.BeTrue())

	svc.EXPECT().
		IfaceExists(gomock.Eq(ctx), &hostDeviceNameMatcher{}).
		Return(false, nil).
		Times(1)

	svc.EXPECT().
		IfaceCreate(gomock.Eq(ctx), &ifaceCreateInputMatcher{}).
		Return(nil, errors.ErrParentIfaceRequiredForAttachingTap).
		Times(1)

	_, err = step.Do(ctx)
	g.Expect(err).To(g.MatchError(errors.ErrParentIfaceRequiredForAttachingTap))

	verifyErr := step.Verify(ctx)
	g.Expect(verifyErr).To(g.BeNil())
}
