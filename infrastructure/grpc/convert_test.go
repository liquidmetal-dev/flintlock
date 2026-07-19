package grpc

import (
	"testing"

	g "github.com/onsi/gomega"

	"github.com/liquidmetal-dev/flintlock/api/types"
	"github.com/liquidmetal-dev/flintlock/core/models"
)

func TestConvert_AllowGuestAgentRoundTrip(t *testing.T) {
	g.RegisterTestingT(t)

	spec := &types.MicroVMSpec{
		Id:              "test",
		Namespace:       "ns",
		AllowGuestAgent: true,
	}

	model, err := convertMicroVMToModel(spec)
	g.Expect(err).NotTo(g.HaveOccurred())
	g.Expect(model.Spec.AllowGuestAgent).To(g.BeTrue())

	back := convertModelToMicroVMSpec(model)
	g.Expect(back.AllowGuestAgent).To(g.BeTrue())
}

func TestConvert_StatusVsockPath(t *testing.T) {
	g.RegisterTestingT(t)

	mvm := &models.MicroVM{
		Status: models.MicroVMStatus{VSockPath: "/var/lib/flintlock/vm/guest-agent.vsock"},
	}

	status := convertModelToMicroVMStatus(mvm)
	g.Expect(status.VsockPath).To(g.Equal("/var/lib/flintlock/vm/guest-agent.vsock"))
}
