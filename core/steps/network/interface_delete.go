package network

import (
	"context"
	"fmt"

	"github.com/sirupsen/logrus"
	"github.com/weaveworks/reignite/core/models"
	"github.com/weaveworks/reignite/core/ports"
	"github.com/weaveworks/reignite/pkg/log"
	"github.com/weaveworks/reignite/pkg/planner"
)

func DeleteNetworkInterface(vmid *models.VMID, iface *models.NetworkInterface, svc ports.NetworkService) planner.Procedure {
	return deleteInterface{
		vmid:  vmid,
		iface: iface,
		svc:   svc,
	}
}

type deleteInterface struct {
	vmid  *models.VMID
	iface *models.NetworkInterface

	svc ports.NetworkService
}

// Name is the name of the procedure/operation.
func (s deleteInterface) Name() string {
	return "network_iface_delete"
}

// Do will perform the operation/procedure.
func (s deleteInterface) Do(ctx context.Context) ([]planner.Procedure, error) {
	logger := log.GetLogger(ctx).WithFields(logrus.Fields{
		"step":  s.Name(),
		"iface": s.iface.GuestDeviceName,
	})
	logger.Debug("running step to delete network interface")

	deviceName := getDeviceName(s.vmid, s.iface)

	exists, err := s.svc.IfaceExists(ctx, deviceName)
	if err != nil {
		return nil, fmt.Errorf("checking if networking interface exists: %w", err)
	}

	if !exists {
		return nil, nil
	}

	deleteErr := s.svc.IfaceDelete(ctx, ports.DeleteIfaceInput{DeviceName: deviceName})
	if deleteErr != nil {
		return nil, fmt.Errorf("deleting networking interface: %w", err)
	}

	return nil, nil
}

// ShouldDo determines if this procedure should be executed.
func (s deleteInterface) ShouldDo(ctx context.Context) (bool, error) {
	logger := log.GetLogger(ctx).WithFields(logrus.Fields{
		"step":  s.Name(),
		"iface": s.iface.GuestDeviceName,
	})
	logger.Debug("checking if procedure should be run")

	deviceName := getDeviceName(s.vmid, s.iface)

	exists, err := s.svc.IfaceExists(ctx, deviceName)
	if err != nil {
		return false, fmt.Errorf("checking if network interface %s exists: %w", deviceName, err)
	}

	return exists, nil
}
