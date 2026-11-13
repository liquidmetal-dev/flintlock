package grpc

import (
	"testing"
	"time"

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

func TestConvert_VolumeMode(t *testing.T) {
	tt := []struct {
		name      string
		spec      *types.Volume
		wantModel models.VolumeMode
		wantProto types.VolumeMode
	}{
		{
			name:      "unset mode is full",
			spec:      &types.Volume{Id: "root"},
			wantModel: models.VolumeModeFull,
			wantProto: types.VolumeMode_FULL,
		},
		{
			name:      "full mode",
			spec:      &types.Volume{Id: "root", Mode: types.VolumeMode_FULL},
			wantModel: models.VolumeModeFull,
			wantProto: types.VolumeMode_FULL,
		},
		{
			name:      "overlay mode",
			spec:      &types.Volume{Id: "root", Mode: types.VolumeMode_OVERLAY},
			wantModel: models.VolumeModeOverlay,
			wantProto: types.VolumeMode_OVERLAY,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			g.RegisterTestingT(t)

			model := convertVolumeToModel(tc.spec)
			g.Expect(model.Mode).To(g.Equal(tc.wantModel))

			back := convertModelToVolumne(model)
			g.Expect(back.Mode).To(g.Equal(tc.wantProto))
		})
	}
}

func TestConvert_VolumeModeUnsetInModel(t *testing.T) {
	g.RegisterTestingT(t)

	// A spec stored before the mode field existed has no mode. It is a full volume.
	back := convertModelToVolumne(&models.Volume{ID: "root"})
	g.Expect(back.Mode).To(g.Equal(types.VolumeMode_FULL))
}

func TestConvert_VolumeStatusOverlay(t *testing.T) {
	g.RegisterTestingT(t)

	mvm := &models.MicroVM{
		Status: models.MicroVMStatus{
			Volumes: models.VolumeStatuses{
				"root": {
					Mount:      models.Mount{Type: models.MountTypeFile, Source: "volumes/root.rw"},
					BaseMount:  &models.Mount{Type: models.MountTypeFile, Source: "volumes/root.base"},
					BaseDigest: "sha256:abc",
				},
				"data": {
					Mount: models.Mount{Type: models.MountTypeDev, Source: "/dev/mapper/data"},
				},
			},
		},
	}

	status := convertModelToMicroVMStatus(mvm)

	root := status.Volumes["root"]
	g.Expect(root.Mount.Type).To(g.Equal(types.Mount_FILE))
	g.Expect(root.Mount.Source).To(g.Equal("volumes/root.rw"))
	g.Expect(root.BaseMount).NotTo(g.BeNil())
	g.Expect(root.BaseMount.Type).To(g.Equal(types.Mount_FILE))
	g.Expect(root.BaseMount.Source).To(g.Equal("volumes/root.base"))
	g.Expect(root.BaseDigest).To(g.Equal("sha256:abc"))

	data := status.Volumes["data"]
	g.Expect(data.Mount.Type).To(g.Equal(types.Mount_DEV))
	g.Expect(data.BaseMount).To(g.BeNil())
	g.Expect(data.BaseDigest).To(g.BeEmpty())
}

func TestConvert_VolumeRoundTrip(t *testing.T) {
	g.RegisterTestingT(t)

	mountPoint := "/mnt/share"
	sharePath := "/srv/share"
	spec := &types.MicroVMSpec{
		Id:        "vm",
		Namespace: "ns",
		AdditionalVolumes: []*types.Volume{{
			Id:         "share",
			MountPoint: &mountPoint,
			Source:     &types.VolumeSource{VirtiofsSource: &sharePath},
		}},
	}

	model, err := convertMicroVMToModel(spec)
	g.Expect(err).NotTo(g.HaveOccurred())
	g.Expect(model.Spec.AdditionalVolumes).To(g.HaveLen(1))
	g.Expect(model.Spec.AdditionalVolumes[0].MountPoint).To(g.Equal(mountPoint))
	g.Expect(model.Spec.AdditionalVolumes[0].Source.VirtioFS.Path).To(g.Equal(sharePath))

	back := convertModelToMicroVMSpec(model)
	g.Expect(back.AdditionalVolumes).To(g.HaveLen(1))
	g.Expect(back.AdditionalVolumes[0].MountPoint).To(g.HaveValue(g.Equal(mountPoint)))
	g.Expect(back.AdditionalVolumes[0].Source.VirtiofsSource).To(g.HaveValue(g.Equal(sharePath)))
}

func TestConvertModelToMicroVMStatus_Conditions(t *testing.T) {
	g.RegisterTestingT(t)

	t1 := time.Date(2026, 11, 13, 10, 0, 0, 0, time.UTC)
	t2 := t1.Add(time.Minute)
	t3 := t2.Add(time.Minute)

	mvm := &models.MicroVM{
		Status: models.MicroVMStatus{
			Conditions: models.Conditions{
				{Type: models.ConditionVMMRunning, Status: models.ConditionTrue, Reason: models.ReasonRunning, Message: "msg", LastTransition: t1},
				{Type: models.ConditionGuestAgentReady, Status: models.ConditionFalse, Reason: models.ReasonPingTimeout, LastTransition: t2},
				{Type: "Other", Status: models.ConditionUnknown, LastTransition: t3},
			},
		},
	}

	conds := convertModelToMicroVMStatus(mvm).Conditions
	g.Expect(conds).To(g.HaveLen(3))

	g.Expect(conds[0].Type).To(g.Equal("VMMRunning"))
	g.Expect(conds[0].Status).To(g.Equal(types.MicroVMCondition_TRUE))
	g.Expect(conds[0].Reason).To(g.Equal("Running"))
	g.Expect(conds[0].Message).To(g.Equal("msg"))
	g.Expect(conds[0].LastTransition.AsTime().Equal(t1)).To(g.BeTrue())

	g.Expect(conds[1].Type).To(g.Equal("GuestAgentReady"))
	g.Expect(conds[1].Status).To(g.Equal(types.MicroVMCondition_FALSE))
	g.Expect(conds[1].Reason).To(g.Equal("PingTimeout"))
	g.Expect(conds[1].LastTransition.AsTime().Equal(t2)).To(g.BeTrue())

	g.Expect(conds[2].Status).To(g.Equal(types.MicroVMCondition_UNKNOWN))
}
