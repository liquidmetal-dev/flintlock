package grpc

import (
	"testing"
	"time"

	g "github.com/onsi/gomega"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/liquidmetal-dev/flintlock/api/types"
	"github.com/liquidmetal-dev/flintlock/core/models"
	"github.com/liquidmetal-dev/flintlock/pkg/ptr"
)

func TestConvert_SnapshotRoundTrip(t *testing.T) {
	g.RegisterTestingT(t)

	createdAt := time.Date(2026, 11, 13, 10, 0, 0, 0, time.UTC)
	snapshot := &types.Snapshot{
		Uid:       "snap-uid",
		Name:      "nightly",
		Namespace: "ns",
		Source:    &types.SourceMicroVM{Uid: "vm-uid", Name: "vm", Namespace: "ns"},
		Labels:    map[string]string{"env": "test"},
		Status: &types.SnapshotStatus{
			Phase:     types.SnapshotStatus_READY,
			LocalPath: "/var/lib/flintlock/snapshots/snap-uid",
		},
		CreatedAt: timestamppb.New(createdAt),
	}

	model := convertSnapshotToModel(snapshot)
	g.Expect(model.UID).To(g.Equal("snap-uid"))
	g.Expect(model.Name).To(g.Equal("nightly"))
	g.Expect(model.Namespace).To(g.Equal("ns"))
	g.Expect(model.Source.UID()).To(g.Equal("vm-uid"))
	g.Expect(model.Source.Name()).To(g.Equal("vm"))
	g.Expect(model.Source.Namespace()).To(g.Equal("ns"))
	g.Expect(model.Labels).To(g.Equal(map[string]string{"env": "test"}))
	g.Expect(model.Status.Phase).To(g.Equal(models.SnapshotPhaseReady))
	g.Expect(model.Status.LocalPath).To(g.Equal("/var/lib/flintlock/snapshots/snap-uid"))
	g.Expect(model.CreatedAt).To(g.BeTemporally("==", createdAt))

	back := convertModelToSnapshot(model)
	g.Expect(proto.Equal(back, snapshot)).To(g.BeTrue(), "round trip changed the snapshot: %v", back)
}

func TestConvert_SnapshotFailedStatus(t *testing.T) {
	g.RegisterTestingT(t)

	model := &models.Snapshot{
		UID:    "snap-uid",
		Source: *models.NewVMIDForce("vm", "ns", "vm-uid"),
		Status: models.SnapshotStatus{Phase: models.SnapshotPhaseFailed, Error: "capture failed"},
	}

	converted := convertModelToSnapshot(model)
	g.Expect(converted.Status.Phase).To(g.Equal(types.SnapshotStatus_FAILED))
	g.Expect(converted.Status.Error).To(g.Equal("capture failed"))
	g.Expect(converted.CreatedAt).To(g.BeNil())

	back := convertSnapshotToModel(converted)
	g.Expect(back.Status).To(g.Equal(model.Status))
	g.Expect(back.CreatedAt.IsZero()).To(g.BeTrue())
}

func TestConvert_SnapshotPhase(t *testing.T) {
	tt := []struct {
		proto types.SnapshotStatus_Phase
		model models.SnapshotPhase
	}{
		{types.SnapshotStatus_PENDING, models.SnapshotPhasePending},
		{types.SnapshotStatus_QUIESCING, models.SnapshotPhaseQuiescing},
		{types.SnapshotStatus_CAPTURING, models.SnapshotPhaseCapturing},
		{types.SnapshotStatus_PACKAGING, models.SnapshotPhasePackaging},
		{types.SnapshotStatus_PUSHING, models.SnapshotPhasePushing},
		{types.SnapshotStatus_READY, models.SnapshotPhaseReady},
		{types.SnapshotStatus_FAILED, models.SnapshotPhaseFailed},
	}

	g.RegisterTestingT(t)
	g.Expect(tt).To(g.HaveLen(len(types.SnapshotStatus_Phase_name)))

	for _, tc := range tt {
		t.Run(tc.proto.String(), func(t *testing.T) {
			g.RegisterTestingT(t)

			g.Expect(convertSnapshotPhaseToModel(tc.proto)).To(g.Equal(tc.model))
			g.Expect(convertModelToSnapshotPhase(tc.model)).To(g.Equal(tc.proto))
		})
	}
}

func TestConvert_SnapshotRef(t *testing.T) {
	tt := []struct {
		name  string
		proto *types.SnapshotRef
		model models.SnapshotRef
	}{
		{
			name:  "uid",
			proto: &types.SnapshotRef{Ref: &types.SnapshotRef_Uid{Uid: "snap-uid"}},
			model: models.SnapshotRef{UID: "snap-uid"},
		},
		{
			name: "named",
			proto: &types.SnapshotRef{
				Ref: &types.SnapshotRef_Named{Named: &types.NamedRef{Name: "nightly", Namespace: "ns"}},
			},
			model: models.SnapshotRef{Name: "nightly", Namespace: "ns"},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			g.RegisterTestingT(t)

			model := convertSnapshotRefToModel(tc.proto)
			g.Expect(model).To(g.Equal(tc.model))

			back := convertModelToSnapshotRef(&model)
			g.Expect(proto.Equal(back, tc.proto)).To(g.BeTrue(), "round trip changed the ref: %v", back)
		})
	}
}

func TestConvert_SnapshotRefNil(t *testing.T) {
	g.RegisterTestingT(t)

	g.Expect(convertSnapshotRefToModel(nil)).To(g.Equal(models.SnapshotRef{}))
}

func TestConvert_SnapshotSourceRoundTrip(t *testing.T) {
	g.RegisterTestingT(t)

	source := &types.SnapshotSource{
		Snapshot:  &types.SnapshotRef{Ref: &types.SnapshotRef_Uid{Uid: "snap-uid"}},
		Name:      "restored",
		Namespace: ptr.String("other"),
		Uid:       ptr.String("restored-uid"),
		Labels:    map[string]string{"env": "test"},
		Metadata:  map[string]string{"meta-data": "aW5zdGFuY2VfaWQ6IHJlc3RvcmVk"},
		NetworkOverrides: []*types.HostNetworkOverride{
			{GuestDeviceName: "eth1", HostDeviceName: "tap-restored"},
		},
		MemoryMode: types.MemoryMode_COPY,
		Force:      true,
	}

	model := convertSnapshotSourceToModel(source)
	g.Expect(model).To(g.Equal(&models.SnapshotSource{
		Snapshot:  models.SnapshotRef{UID: "snap-uid"},
		Name:      "restored",
		Namespace: "other",
		UID:       "restored-uid",
		Labels:    map[string]string{"env": "test"},
		Metadata:  map[string]string{"meta-data": "aW5zdGFuY2VfaWQ6IHJlc3RvcmVk"},
		NetworkOverrides: []models.HostNetworkOverride{
			{GuestDeviceName: "eth1", HostDeviceName: "tap-restored"},
		},
		MemoryMode: models.MemoryModeCopy,
		Force:      true,
	}))

	back := convertModelToSnapshotSource(model)
	g.Expect(proto.Equal(back, source)).To(g.BeTrue(), "round trip changed the source: %v", back)
}

func TestConvert_SnapshotSourceDefaults(t *testing.T) {
	g.RegisterTestingT(t)

	source := &types.SnapshotSource{
		Snapshot: &types.SnapshotRef{
			Ref: &types.SnapshotRef_Named{Named: &types.NamedRef{Name: "nightly", Namespace: "ns"}},
		},
		Name: "restored",
	}

	model := convertSnapshotSourceToModel(source)
	g.Expect(model.Namespace).To(g.BeEmpty())
	g.Expect(model.UID).To(g.BeEmpty())
	g.Expect(model.Labels).To(g.BeEmpty())
	g.Expect(model.Metadata).To(g.BeEmpty())
	g.Expect(model.MemoryMode).To(g.Equal(models.MemoryModeDefault))

	back := convertModelToSnapshotSource(model)
	g.Expect(back.Namespace).To(g.BeNil())
	g.Expect(back.Uid).To(g.BeNil())
	g.Expect(proto.Equal(back, source)).To(g.BeTrue(), "round trip changed the source: %v", back)
}

func TestConvert_MemoryMode(t *testing.T) {
	tt := []struct {
		proto types.MemoryMode
		model models.MemoryMode
	}{
		{types.MemoryMode_MEMORY_MODE_UNSPECIFIED, models.MemoryModeDefault},
		{types.MemoryMode_FILE, models.MemoryModeFile},
		{types.MemoryMode_COPY, models.MemoryModeCopy},
	}

	g.RegisterTestingT(t)
	g.Expect(tt).To(g.HaveLen(len(types.MemoryMode_name)))

	for _, tc := range tt {
		t.Run(tc.proto.String(), func(t *testing.T) {
			g.RegisterTestingT(t)

			g.Expect(convertMemoryModeToModel(tc.proto)).To(g.Equal(tc.model))
			g.Expect(convertModelToMemoryMode(tc.model)).To(g.Equal(tc.proto))
		})
	}
}
