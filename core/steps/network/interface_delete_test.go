package network_test

import (
	"context"
	"testing"

	"github.com/golang/mock/gomock"
	g "github.com/onsi/gomega"
	"github.com/vishvananda/netlink"
	"github.com/weaveworks-liquidmetal/flintlock/core/errors"
	"github.com/weaveworks-liquidmetal/flintlock/core/models"
	"github.com/weaveworks-liquidmetal/flintlock/core/ports"
	"github.com/weaveworks-liquidmetal/flintlock/core/steps/network"
	"github.com/weaveworks-liquidmetal/flintlock/infrastructure/mock"
)

func TestDeleteNetworkInterface_doesNotExist(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	g.RegisterTestingT(t)

	vmid, _ := models.NewVMID(vmName, nsName, vmUID)
	iface := &models.NetworkInterfaceStatus{HostDeviceName: expectedTapDeviceName}
	svc := mock.NewMockNetworkService(mockCtrl)
	ctx := context.Background()

	svc.EXPECT().
		IfaceExists(gomock.Eq(ctx), gomock.Eq(expectedTapDeviceName)).
		Return(false, nil).
		Times(1)

	step := network.DeleteNetworkInterface(vmid, iface, svc)

	shouldDo, err := step.ShouldDo(ctx)
	g.Expect(err).To(g.BeNil())
	g.Expect(shouldDo).To(g.BeFalse())

	svc.EXPECT().
		IfaceExists(gomock.Eq(ctx), gomock.Eq(expectedTapDeviceName)).
		Return(false, nil).
		Times(1)

	_, err = step.Do(ctx)
	g.Expect(err).To(g.BeNil())

	verifyErr := step.Verify(ctx)
	g.Expect(verifyErr).To(g.BeNil())
}

func TestDeleteNetworkInterface_emptyStatus(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	g.RegisterTestingT(t)

	vmid, _ := models.NewVMID(vmName, nsName, vmUID)
	iface := &models.NetworkInterfaceStatus{}
	svc := mock.NewMockNetworkService(mockCtrl)
	ctx := context.Background()

	step := network.DeleteNetworkInterface(vmid, iface, svc)

	shouldDo, err := step.ShouldDo(ctx)
	g.Expect(err).To(g.BeNil())
	g.Expect(shouldDo).To(g.BeFalse())

	_, err = step.Do(ctx)
	g.Expect(err).ToNot(g.BeNil())

	verifyErr := step.Verify(ctx)
	g.Expect(verifyErr).To(g.BeNil())
}

func TestDeleteNetworkInterface_exists(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	g.RegisterTestingT(t)

	vmid, _ := models.NewVMID(vmName, nsName, vmUID)
	iface := &models.NetworkInterfaceStatus{HostDeviceName: expectedTapDeviceName}
	svc := mock.NewMockNetworkService(mockCtrl)
	ctx := context.Background()

	svc.EXPECT().
		IfaceExists(gomock.Eq(ctx), gomock.Eq(expectedTapDeviceName)).
		Return(true, nil).
		Times(1)

	step := network.DeleteNetworkInterface(vmid, iface, svc)

	shouldDo, err := step.ShouldDo(ctx)
	g.Expect(err).To(g.BeNil())
	g.Expect(shouldDo).To(g.BeTrue())

	svc.EXPECT().
		IfaceExists(gomock.Eq(ctx), gomock.Eq(expectedTapDeviceName)).
		Return(true, nil).
		Times(1)

	svc.EXPECT().
		IfaceDelete(
			gomock.Eq(ctx),
			gomock.Eq(ports.DeleteIfaceInput{DeviceName: expectedTapDeviceName}),
		).
		Return(nil).
		Times(1)

	_, err = step.Do(ctx)
	g.Expect(err).To(g.BeNil())

	verifyErr := step.Verify(ctx)
	g.Expect(verifyErr).To(g.BeNil())
}

func TestDeleteNetworkInterface_exists_errorDeleting(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	g.RegisterTestingT(t)

	vmid, _ := models.NewVMID(vmName, nsName, vmUID)
	iface := &models.NetworkInterfaceStatus{HostDeviceName: expectedTapDeviceName}
	svc := mock.NewMockNetworkService(mockCtrl)
	ctx := context.Background()

	svc.EXPECT().
		IfaceExists(gomock.Eq(ctx), gomock.Eq(expectedTapDeviceName)).
		Return(true, nil).
		Times(1)

	step := network.DeleteNetworkInterface(vmid, iface, svc)

	shouldDo, err := step.ShouldDo(ctx)
	g.Expect(err).To(g.BeNil())
	g.Expect(shouldDo).To(g.BeTrue())

	svc.EXPECT().
		IfaceExists(gomock.Eq(ctx), gomock.Eq(expectedTapDeviceName)).
		Return(true, nil).
		Times(1)

	svc.EXPECT().
		IfaceDelete(
			gomock.Eq(ctx),
			gomock.Eq(ports.DeleteIfaceInput{DeviceName: expectedTapDeviceName}),
		).
		Return(netlink.LinkNotFoundError{}).
		Times(1)

	_, err = step.Do(ctx)
	g.Expect(err).ToNot(g.BeNil())

	verifyErr := step.Verify(ctx)
	g.Expect(verifyErr).To(g.BeNil())
}

func TestDeleteNetworkInterface_IfaceExistsError(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	g.RegisterTestingT(t)

	vmid, _ := models.NewVMID(vmName, nsName, vmUID)
	iface := &models.NetworkInterfaceStatus{HostDeviceName: expectedTapDeviceName}
	svc := mock.NewMockNetworkService(mockCtrl)
	ctx := context.Background()

	svc.EXPECT().
		IfaceExists(gomock.Eq(ctx), gomock.Eq(expectedTapDeviceName)).
		Return(false, errors.ErrParentIfaceRequiredForAttachingTap).
		Times(2)

	step := network.DeleteNetworkInterface(vmid, iface, svc)

	shouldDo, err := step.ShouldDo(ctx)
	g.Expect(err).ToNot(g.BeNil())
	g.Expect(shouldDo).To(g.BeFalse())

	_, err = step.Do(ctx)
	g.Expect(err).To(g.MatchError(errors.ErrParentIfaceRequiredForAttachingTap))

	verifyErr := step.Verify(ctx)
	g.Expect(verifyErr).To(g.BeNil())
}
