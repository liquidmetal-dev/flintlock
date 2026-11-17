package application_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/gomega"
	"github.com/spf13/afero"

	"github.com/liquidmetal-dev/flintlock/api/events"
	"github.com/liquidmetal-dev/flintlock/core/application"
	coreerrs "github.com/liquidmetal-dev/flintlock/core/errors"
	"github.com/liquidmetal-dev/flintlock/core/models"
	"github.com/liquidmetal-dev/flintlock/core/ports"
	"github.com/liquidmetal-dev/flintlock/infrastructure/mock"
	"github.com/liquidmetal-dev/flintlock/pkg/defaults"
)

const (
	restoreSnapshotDir = "/var/lib/flintlock/snapshots/" + testSnapshotUID
	restoreNewUID      = "01HRESTOREDUID00000000000"
)

var restoreCapabilities = models.Capabilities{
	models.SnapshotCapability,
	models.VolumeOverlayCapability,
	models.VSockCapability,
	models.MetadataServiceCapability,
}

type restoreMocks struct {
	repo      *mock.MockMicroVMRepository
	snapshots *mock.MockSnapshotRepository
	events    *mock.MockEventService
	ids       *mock.MockIDService
	hostInfo  *mock.MockHostInfoService
	fc        *mock.MockMicroVMService
	ch        *mock.MockMicroVMService
	fs        afero.Fs
}

func newRestoreTestApp(t *testing.T) (application.App, restoreMocks) {
	t.Helper()

	mockCtrl := gomock.NewController(t)

	mocks := restoreMocks{
		repo:      mock.NewMockMicroVMRepository(mockCtrl),
		snapshots: mock.NewMockSnapshotRepository(mockCtrl),
		events:    mock.NewMockEventService(mockCtrl),
		ids:       mock.NewMockIDService(mockCtrl),
		hostInfo:  mock.NewMockHostInfoService(mockCtrl),
		fc:        mock.NewMockMicroVMService(mockCtrl),
		ch:        mock.NewMockMicroVMService(mockCtrl),
		fs:        afero.NewMemMapFs(),
	}

	app := application.New(&application.Config{DefaultProvider: models.VMMTypeFirecracker}, &ports.Collection{
		Repo:         mocks.repo,
		SnapshotRepo: mocks.snapshots,
		MicrovmProviders: map[string]ports.MicroVMService{
			models.VMMTypeFirecracker:     mocks.fc,
			models.VMMTypeCloudHypervisor: mocks.ch,
		},
		EventService:      mocks.events,
		IdentifierService: mocks.ids,
		HostInfoService:   mocks.hostInfo,
		FileSystem:        mocks.fs,
		Clock:             func() time.Time { return snapshotTestTime },
	})

	return app, mocks
}

func restoreSourceSpec(provider string) *models.MicroVMSpec {
	return &models.MicroVMSpec{
		Provider:   provider,
		VCPU:       2,
		MemoryInMb: 2048,
		Kernel: models.Kernel{
			Image:    "docker.io/linuxkit/kernel:5.4.129",
			Filename: "kernel",
		},
		Initrd: &models.Initrd{
			Image:    "ghcr.io/liquidmetal-dev/flintlock-overlay-init:0.16.0",
			Filename: "initrd.img",
		},
		NetworkInterfaces: []models.NetworkInterface{
			{GuestDeviceName: "eth0", Type: models.IfaceTypeTap, AllowMetadataRequests: true},
			{GuestDeviceName: "eth1", Type: models.IfaceTypeTap},
		},
		RootVolume: models.Volume{
			ID:   "root",
			Mode: models.VolumeModeOverlay,
			Size: 4096,
			Source: models.VolumeSource{
				Container: &models.ContainerVolumeSource{Image: "docker.io/library/ubuntu:24.04"},
			},
		},
		Metadata:        map[string]string{"user-data": "c291cmNl"},
		AllowGuestAgent: true,
		CreatedAt:       snapshotTestTime.Add(-time.Hour).Unix(),
	}
}

func restoreDescriptor(provider string) *models.SnapshotDescriptor {
	return &models.SnapshotDescriptor{
		FormatVersion:     models.SnapshotDescriptorFormatVersion,
		VMMType:           provider,
		VMMVersion:        "v1.16.1",
		FCSnapshotVersion: "8.0.0",
		Arch:              "amd64",
		CPUVendor:         "GenuineIntel",
		CPUModel:          "Xeon",
		CPUMicrocode:      "0x1",
		HostKernel:        "6.8.0",
	}
}

func restoreFacts(desc *models.SnapshotDescriptor) *models.HostFacts {
	return &models.HostFacts{
		VMMType:           desc.VMMType,
		VMMVersion:        desc.VMMVersion,
		FCSnapshotVersion: desc.FCSnapshotVersion,
		Arch:              desc.Arch,
		CPUVendor:         desc.CPUVendor,
		CPUModel:          desc.CPUModel,
		CPUMicrocode:      desc.CPUMicrocode,
		HostKernel:        desc.HostKernel,
	}
}

func readySnapshot() *models.Snapshot {
	source, _ := models.NewVMID("mvm1", "ns1", testUID)

	return &models.Snapshot{
		UID:       testSnapshotUID,
		Name:      "snap1",
		Namespace: "ns1",
		Source:    *source,
		Status:    models.SnapshotStatus{Phase: models.SnapshotPhaseReady, LocalPath: restoreSnapshotDir},
	}
}

func writeSnapshotFiles(t *testing.T, fs afero.Fs, desc *models.SnapshotDescriptor, spec *models.MicroVMSpec) {
	t.Helper()

	for name, value := range map[string]any{
		models.SnapshotDescriptorFilename: desc,
		models.SnapshotSpecFilename:       spec,
	} {
		data, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}

		if err := afero.WriteFile(fs, filepath.Join(restoreSnapshotDir, name), data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

func restoreRequest(source models.SnapshotSource) *models.MicroVM {
	return &models.MicroVM{Spec: models.MicroVMSpec{SnapshotSource: &source}}
}

func defaultSnapshotSource() models.SnapshotSource {
	return models.SnapshotSource{
		Snapshot: models.SnapshotRef{Name: "snap1", Namespace: "ns1"},
		Name:     "clone1",
	}
}

// expectRestoreLookups sets the expectations of a restore request up to the
// compatibility check.
func expectRestoreLookups(mocks restoreMocks, snapshot *models.Snapshot, facts *models.HostFacts, provider string) {
	mocks.snapshots.EXPECT().Get(gomock.Any(), gomock.Any()).Return(snapshot, nil)
	mocks.snapshots.EXPECT().Dir(snapshot.UID).Return(restoreSnapshotDir)
	mocks.hostInfo.EXPECT().Facts(gomock.Any(), provider).Return(facts, nil)
}

func decodedMarker(t *testing.T, vm *models.MicroVM, key string) string {
	t.Helper()

	value, ok := vm.Spec.Metadata[key]
	if !ok {
		t.Fatalf("metadata has no %s", key)
	}

	decoded, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		t.Fatalf("decoding %s: %s", key, err)
	}

	return string(decoded)
}

func TestApp_CreateMicroVM_Restore(t *testing.T) {
	RegisterTestingT(t)

	app, mocks := newRestoreTestApp(t)
	desc := restoreDescriptor(models.VMMTypeFirecracker)
	sourceSpec := restoreSourceSpec(models.VMMTypeFirecracker)
	writeSnapshotFiles(t, mocks.fs, desc, sourceSpec)

	source := defaultSnapshotSource()
	source.NetworkOverrides = []models.HostNetworkOverride{{GuestDeviceName: "eth1", HostDeviceName: "tapclone1"}}

	var saved *models.MicroVM

	gomock.InOrder(
		mocks.snapshots.EXPECT().
			Get(gomock.Any(), models.SnapshotRef{Name: "snap1", Namespace: "ns1"}).
			Return(readySnapshot(), nil),
		mocks.snapshots.EXPECT().Dir(testSnapshotUID).Return(restoreSnapshotDir),
		mocks.hostInfo.EXPECT().Facts(gomock.Any(), models.VMMTypeFirecracker).Return(restoreFacts(desc), nil),
		mocks.ids.EXPECT().GenerateRandom().Return(restoreNewUID, nil),
		mocks.repo.EXPECT().
			Get(gomock.Any(), ports.RepositoryGetOptions{Name: "clone1", Namespace: "ns1", UID: restoreNewUID}).
			Return(nil, nil),
		mocks.snapshots.EXPECT().NextGeneration(gomock.Any(), testSnapshotUID).Return(3, nil),
		mocks.repo.EXPECT().Save(gomock.Any(), gomock.Any()).DoAndReturn(
			func(_ context.Context, vm *models.MicroVM) (*models.MicroVM, error) {
				saved = vm

				return vm, nil
			}),
		mocks.events.EXPECT().Publish(gomock.Any(), defaults.TopicMicroVMEvents, &events.MicroVMSpecCreated{
			ID: "clone1", Namespace: "ns1", UID: restoreNewUID,
		}).Return(nil),
	)
	mocks.fc.EXPECT().Capabilities().Return(restoreCapabilities).AnyTimes()

	created, err := app.CreateMicroVM(context.Background(), restoreRequest(source))
	Expect(err).NotTo(HaveOccurred())
	Expect(created).To(Equal(saved))

	Expect(saved.ID.Name()).To(Equal("clone1"))
	Expect(saved.ID.Namespace()).To(Equal("ns1"))
	Expect(saved.ID.UID()).To(Equal(restoreNewUID))
	Expect(saved.Status.State).To(Equal(models.MicroVMState(models.PendingState)))
	Expect(saved.Spec.CreatedAt).To(Equal(snapshotTestTime.Unix()))

	// The devices, vcpus, memory, kernel and initrd are the source's.
	Expect(saved.Spec.Provider).To(Equal(sourceSpec.Provider))
	Expect(saved.Spec.VCPU).To(Equal(sourceSpec.VCPU))
	Expect(saved.Spec.MemoryInMb).To(Equal(sourceSpec.MemoryInMb))
	Expect(saved.Spec.Kernel).To(Equal(sourceSpec.Kernel))
	Expect(saved.Spec.Initrd).To(Equal(sourceSpec.Initrd))
	Expect(saved.Spec.RootVolume).To(Equal(sourceSpec.RootVolume))
	Expect(saved.Spec.NetworkInterfaces).To(Equal(sourceSpec.NetworkInterfaces))

	// The metadata is the source's, with the markers.
	Expect(saved.Spec.Metadata).To(HaveKeyWithValue("user-data", "c291cmNl"))
	Expect(saved.Spec.Metadata).To(HaveLen(4))
	Expect(decodedMarker(t, saved, models.MetadataSnapshotUID)).To(Equal(testSnapshotUID))
	Expect(decodedMarker(t, saved, models.MetadataRestoreUID)).To(Equal(restoreNewUID))
	Expect(decodedMarker(t, saved, models.MetadataRestoreGeneration)).To(Equal("3"))

	// The reference is resolved to the uid once and recorded with the resolved values.
	Expect(saved.Spec.SnapshotSource).To(Equal(&models.SnapshotSource{
		Snapshot:         models.SnapshotRef{UID: testSnapshotUID},
		Name:             "clone1",
		Namespace:        "ns1",
		UID:              restoreNewUID,
		NetworkOverrides: source.NetworkOverrides,
		MemoryMode:       models.MemoryModeFile,
		Generation:       3,
	}))

	// The override names the host device of eth1. eth0 gets a generated one.
	Expect(saved.Status.NetworkInterfaces).To(HaveLen(1))
	Expect(saved.Status.NetworkInterfaces["eth1"].HostDeviceName).To(Equal("tapclone1"))
}

func TestApp_CreateMicroVM_RestoreOverrides(t *testing.T) {
	RegisterTestingT(t)

	app, mocks := newRestoreTestApp(t)
	desc := restoreDescriptor(models.VMMTypeFirecracker)
	writeSnapshotFiles(t, mocks.fs, desc, restoreSourceSpec(models.VMMTypeFirecracker))

	source := defaultSnapshotSource()
	source.Snapshot = models.SnapshotRef{UID: testSnapshotUID}
	source.Namespace = "ns2"
	source.UID = "given-uid"
	source.Metadata = map[string]string{"meta-data": "cmVwbGFjZWQ="}
	source.MemoryMode = models.MemoryModeFile

	var saved *models.MicroVM

	expectRestoreLookups(mocks, readySnapshot(), restoreFacts(desc), models.VMMTypeFirecracker)
	mocks.fc.EXPECT().Capabilities().Return(restoreCapabilities).AnyTimes()
	mocks.repo.EXPECT().
		Get(gomock.Any(), ports.RepositoryGetOptions{Name: "clone1", Namespace: "ns2", UID: "given-uid"}).
		Return(nil, coreerrs.NewSpecNotFound("clone1", "ns2", "", "given-uid"))
	mocks.snapshots.EXPECT().NextGeneration(gomock.Any(), testSnapshotUID).Return(1, nil)
	mocks.repo.EXPECT().Save(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, vm *models.MicroVM) (*models.MicroVM, error) {
			saved = vm

			return vm, nil
		})
	mocks.events.EXPECT().Publish(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)

	_, err := app.CreateMicroVM(context.Background(), restoreRequest(source))
	Expect(err).NotTo(HaveOccurred())

	Expect(saved.ID.Namespace()).To(Equal("ns2"))
	Expect(saved.ID.UID()).To(Equal("given-uid"))
	Expect(saved.Spec.Metadata).To(HaveKeyWithValue("meta-data", "cmVwbGFjZWQ="))
	Expect(saved.Spec.Metadata).NotTo(HaveKey("user-data"))
	Expect(decodedMarker(t, saved, models.MetadataRestoreUID)).To(Equal("given-uid"))
	Expect(decodedMarker(t, saved, models.MetadataRestoreGeneration)).To(Equal("1"))
}

func TestApp_CreateMicroVM_RestoreRejectsSpecChanges(t *testing.T) {
	testCases := []struct {
		name  string
		field string
		set   func(vm *models.MicroVM)
	}{
		{"vcpu", "vcpu", func(vm *models.MicroVM) { vm.Spec.VCPU = 4 }},
		{"memory", "memory_inmb", func(vm *models.MicroVM) { vm.Spec.MemoryInMb = 4096 }},
		{"kernel", "kernel", func(vm *models.MicroVM) { vm.Spec.Kernel.Image = "docker.io/other/kernel:1" }},
		{"kernel cmdline", "kernel", func(vm *models.MicroVM) {
			vm.Spec.Kernel.CmdLine = map[string]string{"console": "ttyS0"}
		}},
		{"initrd", "initrd", func(vm *models.MicroVM) { vm.Spec.Initrd = &models.Initrd{Image: "docker.io/i:1"} }},
		{"root volume", "root_volume", func(vm *models.MicroVM) { vm.Spec.RootVolume.ID = "root" }},
		{"additional volumes", "additional_volumes", func(vm *models.MicroVM) {
			vm.Spec.AdditionalVolumes = models.Volumes{{ID: "data"}}
		}},
		{"network interfaces", "network_interfaces", func(vm *models.MicroVM) {
			vm.Spec.NetworkInterfaces = []models.NetworkInterface{{GuestDeviceName: "eth0"}}
		}},
		{"metadata", "metadata", func(vm *models.MicroVM) { vm.Spec.Metadata = map[string]string{"a": "b"} }},
		{"guest agent", "allow_guest_agent", func(vm *models.MicroVM) { vm.Spec.AllowGuestAgent = true }},
		{"cpu config", "cpu_config", func(vm *models.MicroVM) { vm.Spec.CPUConfig = &models.CPUConfig{} }},
		{"id", "id", func(vm *models.MicroVM) {
			vmid, _ := models.NewVMID("other", "ns1", "")
			vm.ID = *vmid
		}},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			RegisterTestingT(t)

			app, _ := newRestoreTestApp(t)

			req := restoreRequest(defaultSnapshotSource())
			tc.set(req)

			_, err := app.CreateMicroVM(context.Background(), req)
			Expect(err).To(HaveOccurred())
			Expect(coreerrs.IsInvalidArgument(err)).To(BeTrue(), err.Error())
			Expect(err.Error()).To(HavePrefix(tc.field + " cannot be set for a restore"))
		})
	}
}

func TestApp_CreateMicroVM_RestoreRejections(t *testing.T) {
	testCases := []struct {
		name         string
		source       func(s *models.SnapshotSource)
		provider     string
		capabilities models.Capabilities
		request      func(vm *models.MicroVM)
		descriptor   func(d *models.SnapshotDescriptor)
		expectErr    string
		invalid      bool
		precondition bool
	}{
		{
			name:      "no name",
			source:    func(s *models.SnapshotSource) { s.Name = "" },
			expectErr: "snapshot_source.name is required",
			invalid:   true,
		},
		{
			name:      "no snapshot reference",
			source:    func(s *models.SnapshotSource) { s.Snapshot = models.SnapshotRef{Name: "snap1"} },
			expectErr: "snapshot_source.snapshot needs a uid",
			invalid:   true,
		},
		{
			name:      "another provider",
			request:   func(vm *models.MicroVM) { vm.Spec.Provider = models.VMMTypeCloudHypervisor },
			expectErr: "provider cloudhypervisor is not the provider of the snapshot",
			invalid:   true,
		},
		{
			name: "incompatible host",
			descriptor: func(d *models.SnapshotDescriptor) {
				d.HostKernel = "6.1.0"
			},
			expectErr:    `host_kernel: snapshot "6.1.0", host "6.8.0"`,
			precondition: true,
		},
		{
			name: "incompatible host with force",
			source: func(s *models.SnapshotSource) {
				s.Force = true
			},
			descriptor: func(d *models.SnapshotDescriptor) {
				d.HostKernel = "6.1.0"
				d.CPUVendor = "AuthenticAMD"
			},
			expectErr:    `cpu_vendor: snapshot "AuthenticAMD", host "GenuineIntel"`,
			precondition: true,
		},
		{
			name: "network override of an unknown interface",
			source: func(s *models.SnapshotSource) {
				s.NetworkOverrides = []models.HostNetworkOverride{{GuestDeviceName: "eth9", HostDeviceName: "tap9"}}
			},
			expectErr: "network override for eth9: the snapshot has no interface with that guest device name",
			invalid:   true,
		},
		{
			name: "two network overrides of one interface",
			source: func(s *models.SnapshotSource) {
				s.NetworkOverrides = []models.HostNetworkOverride{
					{GuestDeviceName: "eth1", HostDeviceName: "tap1"},
					{GuestDeviceName: "eth1", HostDeviceName: "tap2"},
				}
			},
			expectErr: "more than one network override for eth1",
			invalid:   true,
		},
		{
			name:         "provider without snapshot capability",
			capabilities: models.Capabilities{models.VSockCapability, models.VolumeOverlayCapability},
			expectErr:    "needs the snapshot capability",
			precondition: true,
		},
		{
			name:      "memory mode copy on firecracker",
			source:    func(s *models.SnapshotSource) { s.MemoryMode = models.MemoryModeCopy },
			expectErr: "memory mode copy is not supported by microvm provider firecracker",
			invalid:   true,
		},
		{
			name:      "memory mode file on cloud hypervisor",
			provider:  models.VMMTypeCloudHypervisor,
			source:    func(s *models.SnapshotSource) { s.MemoryMode = models.MemoryModeFile },
			expectErr: "memory mode file is not supported by microvm provider cloudhypervisor",
			invalid:   true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			RegisterTestingT(t)

			app, mocks := newRestoreTestApp(t)

			provider := tc.provider
			if provider == "" {
				provider = models.VMMTypeFirecracker
			}

			desc := restoreDescriptor(provider)
			facts := restoreFacts(desc)

			if tc.descriptor != nil {
				tc.descriptor(desc)
			}

			writeSnapshotFiles(t, mocks.fs, desc, restoreSourceSpec(provider))

			capabilities := tc.capabilities
			if capabilities == nil {
				capabilities = restoreCapabilities
			}

			mocks.snapshots.EXPECT().Get(gomock.Any(), gomock.Any()).Return(readySnapshot(), nil).MaxTimes(1)
			mocks.snapshots.EXPECT().Dir(testSnapshotUID).Return(restoreSnapshotDir).MaxTimes(1)
			mocks.hostInfo.EXPECT().Facts(gomock.Any(), provider).Return(facts, nil).MaxTimes(1)
			mocks.ids.EXPECT().GenerateRandom().Return(restoreNewUID, nil).MaxTimes(1)
			mocks.fc.EXPECT().Capabilities().Return(capabilities).AnyTimes()
			mocks.ch.EXPECT().Capabilities().Return(capabilities).AnyTimes()

			source := defaultSnapshotSource()
			if tc.source != nil {
				tc.source(&source)
			}

			req := restoreRequest(source)
			if tc.request != nil {
				tc.request(req)
			}

			_, err := app.CreateMicroVM(context.Background(), req)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring(tc.expectErr))
			Expect(coreerrs.IsInvalidArgument(err)).To(Equal(tc.invalid), err.Error())
			Expect(coreerrs.IsFailedPrecondition(err)).To(Equal(tc.precondition), err.Error())
		})
	}
}

func TestApp_CreateMicroVM_RestoreSnapshotState(t *testing.T) {
	testCases := []struct {
		name         string
		snapshot     *models.Snapshot
		getErr       error
		expectErr    string
		notFound     bool
		precondition bool
	}{
		{
			name:      "snapshot not found",
			getErr:    coreerrs.NewSnapshotNotFound("ns1/snap1"),
			expectErr: "snapshot ns1/snap1 not found",
			notFound:  true,
		},
		{
			name: "snapshot not ready",
			snapshot: func() *models.Snapshot {
				s := readySnapshot()
				s.Status.Phase = models.SnapshotPhaseCapturing

				return s
			}(),
			expectErr:    "is capturing: only a ready snapshot can be restored",
			precondition: true,
		},
		{
			name: "snapshot failed",
			snapshot: func() *models.Snapshot {
				s := readySnapshot()
				s.Status.Phase = models.SnapshotPhaseFailed

				return s
			}(),
			expectErr:    "is failed: only a ready snapshot can be restored",
			precondition: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			RegisterTestingT(t)

			app, mocks := newRestoreTestApp(t)
			mocks.snapshots.EXPECT().Get(gomock.Any(), gomock.Any()).Return(tc.snapshot, tc.getErr)

			_, err := app.CreateMicroVM(context.Background(), restoreRequest(defaultSnapshotSource()))
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring(tc.expectErr))
			Expect(coreerrs.IsSnapshotNotFound(err)).To(Equal(tc.notFound))
			Expect(coreerrs.IsFailedPrecondition(err)).To(Equal(tc.precondition))
		})
	}
}

func TestApp_CreateMicroVM_RestoreDefaultMemoryMode(t *testing.T) {
	testCases := []struct {
		provider string
		expected models.MemoryMode
	}{
		{provider: models.VMMTypeFirecracker, expected: models.MemoryModeFile},
		{provider: models.VMMTypeCloudHypervisor, expected: models.MemoryModeCopy},
	}

	for _, tc := range testCases {
		t.Run(tc.provider, func(t *testing.T) {
			RegisterTestingT(t)

			app, mocks := newRestoreTestApp(t)
			desc := restoreDescriptor(tc.provider)
			writeSnapshotFiles(t, mocks.fs, desc, restoreSourceSpec(tc.provider))

			var saved *models.MicroVM

			expectRestoreLookups(mocks, readySnapshot(), restoreFacts(desc), tc.provider)
			capabilities := restoreCapabilities
			mocks.fc.EXPECT().Capabilities().Return(capabilities).AnyTimes()
			mocks.ch.EXPECT().Capabilities().Return(capabilities).AnyTimes()
			mocks.ids.EXPECT().GenerateRandom().Return(restoreNewUID, nil)
			mocks.repo.EXPECT().Get(gomock.Any(), gomock.Any()).Return(nil, nil)
			mocks.snapshots.EXPECT().NextGeneration(gomock.Any(), testSnapshotUID).Return(1, nil)
			mocks.repo.EXPECT().Save(gomock.Any(), gomock.Any()).DoAndReturn(
				func(_ context.Context, vm *models.MicroVM) (*models.MicroVM, error) {
					saved = vm

					return vm, nil
				})
			mocks.events.EXPECT().Publish(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)

			_, err := app.CreateMicroVM(context.Background(), restoreRequest(defaultSnapshotSource()))
			Expect(err).NotTo(HaveOccurred())
			Expect(saved.Spec.SnapshotSource.MemoryMode).To(Equal(tc.expected))
		})
	}
}

func TestApp_CreateMicroVM_RestoreGenerationError(t *testing.T) {
	RegisterTestingT(t)

	app, mocks := newRestoreTestApp(t)
	desc := restoreDescriptor(models.VMMTypeFirecracker)
	writeSnapshotFiles(t, mocks.fs, desc, restoreSourceSpec(models.VMMTypeFirecracker))

	expectRestoreLookups(mocks, readySnapshot(), restoreFacts(desc), models.VMMTypeFirecracker)
	mocks.fc.EXPECT().Capabilities().Return(restoreCapabilities).AnyTimes()
	mocks.ids.EXPECT().GenerateRandom().Return(restoreNewUID, nil)
	mocks.repo.EXPECT().Get(gomock.Any(), gomock.Any()).Return(nil, nil)
	mocks.snapshots.EXPECT().NextGeneration(gomock.Any(), testSnapshotUID).Return(0, errors.New("locked"))

	_, err := app.CreateMicroVM(context.Background(), restoreRequest(defaultSnapshotSource()))
	Expect(err).To(MatchError(ContainSubstring("taking the restore generation")))
}

func macvtapSourceSpec() *models.MicroVMSpec {
	spec := restoreSourceSpec(models.VMMTypeCloudHypervisor)
	spec.NetworkInterfaces[1].Type = models.IfaceTypeMacvtap

	return spec
}

func TestApp_CreateMicroVM_RestoreMacvtapOnce(t *testing.T) {
	testCases := []struct {
		name       string
		generation int
		expectErr  bool
	}{
		{name: "first restore", generation: 1},
		{name: "second restore", generation: 2, expectErr: true},
		{name: "later restore", generation: 5, expectErr: true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			RegisterTestingT(t)

			app, mocks := newRestoreTestApp(t)
			desc := restoreDescriptor(models.VMMTypeCloudHypervisor)
			writeSnapshotFiles(t, mocks.fs, desc, macvtapSourceSpec())

			expectRestoreLookups(mocks, readySnapshot(), restoreFacts(desc), models.VMMTypeCloudHypervisor)
			mocks.ch.EXPECT().Capabilities().Return(append(models.Capabilities{models.MacvtapCapability}, restoreCapabilities...)).AnyTimes()
			mocks.ids.EXPECT().GenerateRandom().Return(restoreNewUID, nil)
			mocks.repo.EXPECT().Get(gomock.Any(), gomock.Any()).Return(nil, nil)
			mocks.snapshots.EXPECT().NextGeneration(gomock.Any(), testSnapshotUID).Return(tc.generation, nil)

			if !tc.expectErr {
				mocks.repo.EXPECT().Save(gomock.Any(), gomock.Any()).DoAndReturn(
					func(_ context.Context, vm *models.MicroVM) (*models.MicroVM, error) { return vm, nil })
				mocks.events.EXPECT().Publish(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
			}

			_, err := app.CreateMicroVM(context.Background(), restoreRequest(defaultSnapshotSource()))

			if !tc.expectErr {
				Expect(err).NotTo(HaveOccurred())

				return
			}

			// The refusal comes before the spec is saved: Save is not expected.
			Expect(coreerrs.IsFailedPrecondition(err)).To(BeTrue(), "got %v", err)
			Expect(err).To(MatchError(ContainSubstring("it can be restored only once")))
		})
	}
}

// TestApp_CreateMicroVM_RestoreTwice restores one snapshot twice. Each
// restored microvm has its own uid and generation, and no host device of
// the source: the network steps create new ones.
func TestApp_CreateMicroVM_RestoreTwice(t *testing.T) {
	RegisterTestingT(t)

	app, mocks := newRestoreTestApp(t)
	desc := restoreDescriptor(models.VMMTypeFirecracker)
	writeSnapshotFiles(t, mocks.fs, desc, restoreSourceSpec(models.VMMTypeFirecracker))

	var saved []*models.MicroVM

	mocks.snapshots.EXPECT().Get(gomock.Any(), gomock.Any()).Return(readySnapshot(), nil).Times(2)
	mocks.snapshots.EXPECT().Dir(testSnapshotUID).Return(restoreSnapshotDir).Times(2)
	mocks.hostInfo.EXPECT().Facts(gomock.Any(), models.VMMTypeFirecracker).Return(restoreFacts(desc), nil).Times(2)
	mocks.fc.EXPECT().Capabilities().Return(restoreCapabilities).AnyTimes()
	gomock.InOrder(
		mocks.ids.EXPECT().GenerateRandom().Return("01HRESTOREDUID00000000001", nil),
		mocks.ids.EXPECT().GenerateRandom().Return("01HRESTOREDUID00000000002", nil),
	)
	mocks.repo.EXPECT().Get(gomock.Any(), gomock.Any()).Return(nil, nil).Times(2)
	gomock.InOrder(
		mocks.snapshots.EXPECT().NextGeneration(gomock.Any(), testSnapshotUID).Return(1, nil),
		mocks.snapshots.EXPECT().NextGeneration(gomock.Any(), testSnapshotUID).Return(2, nil),
	)
	mocks.repo.EXPECT().Save(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, vm *models.MicroVM) (*models.MicroVM, error) {
			saved = append(saved, vm)

			return vm, nil
		}).Times(2)
	mocks.events.EXPECT().Publish(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).Times(2)

	for _, name := range []string{"clone1", "clone2"} {
		source := defaultSnapshotSource()
		source.Name = name

		_, err := app.CreateMicroVM(context.Background(), restoreRequest(source))
		Expect(err).NotTo(HaveOccurred())
	}

	Expect(saved).To(HaveLen(2))
	first, second := saved[0], saved[1]

	Expect(first.ID.UID()).NotTo(Equal(second.ID.UID()))
	Expect(first.Spec.SnapshotSource.Generation).To(Equal(1))
	Expect(second.Spec.SnapshotSource.Generation).To(Equal(2))
	Expect(decodedMarker(t, first, models.MetadataRestoreGeneration)).To(Equal("1"))
	Expect(decodedMarker(t, second, models.MetadataRestoreGeneration)).To(Equal("2"))
	Expect(decodedMarker(t, first, models.MetadataRestoreUID)).To(Equal(first.ID.UID()))
	Expect(decodedMarker(t, second, models.MetadataRestoreUID)).To(Equal(second.ID.UID()))

	// Both restore the same snapshot, and neither has a host device name.
	for _, vm := range saved {
		Expect(vm.Spec.SnapshotSource.Snapshot).To(Equal(models.SnapshotRef{UID: testSnapshotUID}))
		Expect(vm.Status.NetworkInterfaces).To(BeEmpty())
	}
}
