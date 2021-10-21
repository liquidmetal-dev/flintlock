package network_test

import (
	"context"
	"testing"

	"github.com/golang/mock/gomock"
	g "github.com/onsi/gomega"
	"github.com/vishvananda/netlink"
	"github.com/weaveworks/flintlock/core/errors"
	"github.com/weaveworks/flintlock/core/models"
	"github.com/weaveworks/flintlock/core/ports"
	"github.com/weaveworks/flintlock/core/steps/network"
	"github.com/weaveworks/flintlock/infrastructure/mock"
)

func TestDeleteNetworkInterface_doesNotExist(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	g.RegisterTestingT(t)

	vmid, _ := models.NewVMID("testvm", "testns")
	iface := &models.NetworkInterface{}
	svc := mock.NewMockNetworkService(mockCtrl)
	ctx := context.Background()

	svc.EXPECT().
		IfaceExists(gomock.Eq(ctx), gomock.Eq("testns_testvm_tap")).
		Return(false, nil).
		Times(1)

	step := network.DeleteNetworkInterface(vmid, iface, svc)
	shouldDo, err := step.ShouldDo(ctx)

	g.Expect(err).To(g.BeNil())
	g.Expect(shouldDo).To(g.BeFalse())

	svc.EXPECT().
		IfaceExists(gomock.Eq(ctx), gomock.Eq("testns_testvm_tap")).
		Return(false, nil).
		Times(1)

	_, err = step.Do(ctx)

	g.Expect(err).To(g.BeNil())
}

func TestDeleteNetworkInterface_exists(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	g.RegisterTestingT(t)

	vmid, _ := models.NewVMID("testvm", "testns")
	iface := &models.NetworkInterface{}
	svc := mock.NewMockNetworkService(mockCtrl)
	ctx := context.Background()

	svc.EXPECT().
		IfaceExists(gomock.Eq(ctx), gomock.Eq("testns_testvm_tap")).
		Return(true, nil).
		Times(1)

	step := network.DeleteNetworkInterface(vmid, iface, svc)
	shouldDo, err := step.ShouldDo(ctx)

	g.Expect(err).To(g.BeNil())
	g.Expect(shouldDo).To(g.BeTrue())

	svc.EXPECT().
		IfaceExists(gomock.Eq(ctx), gomock.Eq("testns_testvm_tap")).
		Return(true, nil).
		Times(1)

	svc.EXPECT().
		IfaceDelete(
			gomock.Eq(ctx),
			gomock.Eq(ports.DeleteIfaceInput{DeviceName: "testns_testvm_tap"}),
		).
		Return(nil).
		Times(1)

	_, err = step.Do(ctx)

	g.Expect(err).To(g.BeNil())
}

func TestDeleteNetworkInterface_exists_errorDeleting(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	g.RegisterTestingT(t)

	vmid, _ := models.NewVMID("testvm", "testns")
	iface := &models.NetworkInterface{}
	svc := mock.NewMockNetworkService(mockCtrl)
	ctx := context.Background()

	svc.EXPECT().
		IfaceExists(gomock.Eq(ctx), gomock.Eq("testns_testvm_tap")).
		Return(true, nil).
		Times(1)

	step := network.DeleteNetworkInterface(vmid, iface, svc)
	shouldDo, err := step.ShouldDo(ctx)

	g.Expect(err).To(g.BeNil())
	g.Expect(shouldDo).To(g.BeTrue())

	svc.EXPECT().
		IfaceExists(gomock.Eq(ctx), gomock.Eq("testns_testvm_tap")).
		Return(true, nil).
		Times(1)

	svc.EXPECT().
		IfaceDelete(
			gomock.Eq(ctx),
			gomock.Eq(ports.DeleteIfaceInput{DeviceName: "testns_testvm_tap"}),
		).
		Return(netlink.LinkNotFoundError{}).
		Times(1)

	_, err = step.Do(ctx)

	g.Expect(err).ToNot(g.BeNil())
}

func TestDeleteNetworkInterface_IfaceExistsError(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	g.RegisterTestingT(t)

	vmid, _ := models.NewVMID("testvm", "testns")
	iface := &models.NetworkInterface{}
	svc := mock.NewMockNetworkService(mockCtrl)
	ctx := context.Background()

	svc.EXPECT().
		IfaceExists(gomock.Eq(ctx), gomock.Eq("testns_testvm_tap")).
		Return(false, errors.ErrParentIfaceRequired).
		Times(2)

	step := network.DeleteNetworkInterface(vmid, iface, svc)
	shouldDo, err := step.ShouldDo(ctx)

	g.Expect(err).ToNot(g.BeNil())
	g.Expect(shouldDo).To(g.BeFalse())

	_, err = step.Do(ctx)

	g.Expect(err).To(g.MatchError(errors.ErrParentIfaceRequired))
}
