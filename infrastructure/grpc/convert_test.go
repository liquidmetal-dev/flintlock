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

func TestConvert_CPUConfigRoundTrip(t *testing.T) {
	g.RegisterTestingT(t)

	spec := &types.MicroVMSpec{
		Id:        "test",
		Namespace: "ns",
		CpuConfig: &types.CPUConfig{
			FeaturesToEnable:         []string{"amx"},
			KvmCapabilitiesToDisable: []string{"56"},
		},
	}

	model, err := convertMicroVMToModel(spec)
	g.Expect(err).NotTo(g.HaveOccurred())
	g.Expect(model.Spec.CPUConfig).NotTo(g.BeNil())
	g.Expect(model.Spec.CPUConfig.FeaturesToEnable).To(g.ConsistOf("amx"))
	g.Expect(model.Spec.CPUConfig.KVMCapabilitiesToDisable).To(g.ConsistOf("56"))

	back := convertModelToMicroVMSpec(model)
	g.Expect(back.CpuConfig).NotTo(g.BeNil())
	g.Expect(back.CpuConfig.FeaturesToEnable).To(g.ConsistOf("amx"))
	g.Expect(back.CpuConfig.KvmCapabilitiesToDisable).To(g.ConsistOf("56"))
}

func TestConvert_CPUConfigNil(t *testing.T) {
	g.RegisterTestingT(t)

	spec := &types.MicroVMSpec{Id: "test", Namespace: "ns"}

	model, err := convertMicroVMToModel(spec)
	g.Expect(err).NotTo(g.HaveOccurred())
	g.Expect(model.Spec.CPUConfig).To(g.BeNil())

	back := convertModelToMicroVMSpec(model)
	g.Expect(back.CpuConfig).To(g.BeNil())
}

func TestConvert_StatusVsockPath(t *testing.T) {
	g.RegisterTestingT(t)

	mvm := &models.MicroVM{
		Status: models.MicroVMStatus{VSockPath: "/var/lib/flintlock/vm/guest-agent.vsock"},
	}

	status := convertModelToMicroVMStatus(mvm)
	g.Expect(status.VsockPath).To(g.Equal("/var/lib/flintlock/vm/guest-agent.vsock"))
}
