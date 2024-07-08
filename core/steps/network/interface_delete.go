package network

import (
	"context"
	"fmt"

	"github.com/liquidmetal-dev/flintlock/core/errors"
	"github.com/liquidmetal-dev/flintlock/core/models"
	"github.com/liquidmetal-dev/flintlock/core/ports"
	"github.com/liquidmetal-dev/flintlock/pkg/log"
	"github.com/liquidmetal-dev/flintlock/pkg/planner"
	"github.com/sirupsen/logrus"
)

func DeleteNetworkInterface(vmid *models.VMID,
	iface *models.NetworkInterfaceStatus,
	svc ports.NetworkService,
) planner.Procedure {
	return deleteInterface{
		vmid:  vmid,
		iface: iface,
		svc:   svc,
	}
}

type deleteInterface struct {
	vmid  *models.VMID
	iface *models.NetworkInterfaceStatus

	svc ports.NetworkService
}

// Name is the name of the procedure/operation.
func (s deleteInterface) Name() string {
	return "network_iface_delete"
}

// Do will perform the operation/procedure.
func (s deleteInterface) Do(ctx context.Context) ([]planner.Procedure, error) {
	if s.iface == nil {
		return nil, errors.ErrMissingStatusInfo
	}

	logger := log.GetLogger(ctx).WithFields(logrus.Fields{
		"step":  s.Name(),
		"iface": s.iface.HostDeviceName,
		"vm":    s.vmid.String(),
	})
	logger.Debug("running step to delete network interface")

	deviceName := s.iface.HostDeviceName
	if deviceName == "" {
		return nil, errors.ErrMissingStatusInfo
	}

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
	if s.iface == nil {
		return false, nil
	}

	logger := log.GetLogger(ctx).WithFields(logrus.Fields{
		"step":  s.Name(),
		"iface": s.iface.HostDeviceName,
		"vm":    s.vmid.String(),
	})
	logger.Debug("checking if procedure should be run")

	deviceName := s.iface.HostDeviceName
	if deviceName == "" {
		return false, nil
	}

	exists, err := s.svc.IfaceExists(ctx, deviceName)
	if err != nil {
		return false, fmt.Errorf("checking if network interface %s exists: %w", deviceName, err)
	}

	return exists, nil
}

func (s deleteInterface) Verify(ctx context.Context) error {
	return nil
}
