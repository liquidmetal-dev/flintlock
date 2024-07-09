package network

import (
	"context"
	"fmt"

	"github.com/liquidmetal-dev/flintlock/core/errors"
	"github.com/liquidmetal-dev/flintlock/core/models"
	"github.com/liquidmetal-dev/flintlock/core/ports"
	"github.com/liquidmetal-dev/flintlock/infrastructure/network"
	"github.com/liquidmetal-dev/flintlock/pkg/log"
	"github.com/liquidmetal-dev/flintlock/pkg/planner"
	"github.com/sirupsen/logrus"
)

func NewNetworkInterface(vmid *models.VMID,
	iface *models.NetworkInterface,
	status *models.NetworkInterfaceStatus,
	svc ports.NetworkService,
) planner.Procedure {
	return &createInterface{
		vmid:   vmid,
		iface:  iface,
		svc:    svc,
		status: status,
	}
}

type createInterface struct {
	vmid   *models.VMID
	iface  *models.NetworkInterface
	status *models.NetworkInterfaceStatus

	svc ports.NetworkService
}

// Name is the name of the procedure/operation.
func (s *createInterface) Name() string {
	return "network_iface_create"
}

func (s *createInterface) ShouldDo(ctx context.Context) (bool, error) {
	logger := log.GetLogger(ctx).WithFields(logrus.Fields{
		"step":  s.Name(),
		"iface": s.iface.GuestDeviceName,
	})
	logger.Debug("checking if procedure should be run")

	if s.status == nil || s.status.HostDeviceName == "" {
		return true, nil
	}

	deviceName := s.status.HostDeviceName

	exists, err := s.svc.IfaceExists(ctx, deviceName)
	if err != nil {
		return false, fmt.Errorf("checking if network interface %s exists: %w", deviceName, err)
	}

	return !exists, nil
}

// Do will perform the operation/procedure.
func (s *createInterface) Do(ctx context.Context) ([]planner.Procedure, error) {
	logger := log.GetLogger(ctx).WithFields(logrus.Fields{
		"step":  s.Name(),
		"iface": s.iface.GuestDeviceName,
	})
	logger.Debug("running step to create network interface")

	if s.iface.GuestDeviceName == "" {
		return nil, errors.ErrGuestDeviceNameRequired
	}

	if s.status == nil {
		return nil, errors.ErrMissingStatusInfo
	}

	if s.status.HostDeviceName == "" {
		ifaceName, err := network.NewIfaceName(s.iface.Type)
		if err != nil {
			return nil, fmt.Errorf("creating network interface name: %w", err)
		}

		s.status.HostDeviceName = ifaceName
	}

	deviceName := s.status.HostDeviceName

	exists, err := s.svc.IfaceExists(ctx, deviceName)
	if err != nil {
		return nil, fmt.Errorf("checking if networking interface exists: %w", err)
	}

	if exists {
		details, detailsErr := s.svc.IfaceDetails(ctx, deviceName)
		if detailsErr != nil {
			return nil, fmt.Errorf("getting interface details: %w", detailsErr)
		}

		s.status.HostDeviceName = deviceName
		s.status.Index = details.Index
		s.status.MACAddress = details.MAC

		return nil, nil
	}

	input := &ports.IfaceCreateInput{
		DeviceName: deviceName,
		Type:       s.iface.Type,
		MAC:        s.iface.GuestMAC,
		Attach:     true,
		BridgeName: s.iface.BridgeName,
	}

	if s.iface.Type == models.IfaceTypeTap && s.iface.AllowMetadataRequests {
		input.Attach = false
	}

	output, err := s.svc.IfaceCreate(ctx, *input)
	if err != nil {
		return nil, fmt.Errorf("creating network interface: %w", err)
	}

	s.status.HostDeviceName = deviceName
	s.status.Index = output.Index
	s.status.MACAddress = output.MAC

	return nil, nil
}

func (s *createInterface) Verify(ctx context.Context) error {
	return nil
}
