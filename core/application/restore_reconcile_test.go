package application_test

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/gomega"
	"github.com/spf13/afero"

	"github.com/liquidmetal-dev/flintlock/api/events"
	"github.com/liquidmetal-dev/flintlock/core/application"
	"github.com/liquidmetal-dev/flintlock/core/models"
	"github.com/liquidmetal-dev/flintlock/core/ports"
	"github.com/liquidmetal-dev/flintlock/infrastructure/mock"
	"github.com/liquidmetal-dev/flintlock/pkg/defaults"
)

const (
	reconcileStateDir = "/state"
	reconcileVSock    = "/run/" + testUID + "/guest-agent.vsock"
)

var errInjected = errors.New("injected failure")

// fakeHost keeps the state that the mocked services of a reconcile change: the
// VMM state and the host network devices.
type fakeHost struct {
	vmm    ports.MicroVMState
	ifaces map[string]bool
}

type reconcileMocks struct {
	repo      *mock.MockMicroVMRepository
	snapshots *mock.MockSnapshotRepository
	events    *mock.MockEventService
	ids       *mock.MockIDService
	provider  *mock.MockMicroVMService
	network   *mock.MockNetworkService
	images    *mock.MockImageService
	volumes   *mock.MockVolumeService
	agent     *mock.MockGuestAgentService
	fs        afero.Fs
	host      *fakeHost
}

func newReconcileTestApp(t *testing.T) (application.App, reconcileMocks) {
	t.Helper()

	mockCtrl := gomock.NewController(t)

	mocks := reconcileMocks{
		repo:      mock.NewMockMicroVMRepository(mockCtrl),
		snapshots: mock.NewMockSnapshotRepository(mockCtrl),
		events:    mock.NewMockEventService(mockCtrl),
		ids:       mock.NewMockIDService(mockCtrl),
		provider:  mock.NewMockMicroVMService(mockCtrl),
		network:   mock.NewMockNetworkService(mockCtrl),
		images:    mock.NewMockImageService(mockCtrl),
		volumes:   mock.NewMockVolumeService(mockCtrl),
		agent:     mock.NewMockGuestAgentService(mockCtrl),
		fs:        afero.NewMemMapFs(),
		host:      &fakeHost{vmm: ports.MicroVMStatePending, ifaces: map[string]bool{}},
	}

	app := application.New(&application.Config{
		RootStateDir: reconcileStateDir,
		SocketDir:    "/run",
		MaximumRetry: 3,
	}, &ports.Collection{
		Repo:              mocks.repo,
		SnapshotRepo:      mocks.snapshots,
		MicrovmProviders:  map[string]ports.MicroVMService{"mock": mocks.provider},
		EventService:      mocks.events,
		IdentifierService: mocks.ids,
		NetworkService:    mocks.network,
		ImageService:      mocks.images,
		VolumeService:     mocks.volumes,
		GuestAgent:        mocks.agent,
		FileSystem:        mocks.fs,
		Clock:             func() time.Time { return snapshotTestTime },
	})

	host := mocks.host

	mocks.ids.EXPECT().GenerateRandom().Return("execution", nil).AnyTimes()
	mocks.provider.EXPECT().Capabilities().Return(models.Capabilities{models.VSockCapability}).AnyTimes()
	mocks.provider.EXPECT().State(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, _ string) (ports.MicroVMState, error) { return host.vmm, nil },
	).AnyTimes()
	mocks.snapshots.EXPECT().Dir(testSnapshotUID).Return(restoreSnapshotDir).AnyTimes()
	mocks.images.EXPECT().IsMounted(gomock.Any(), gomock.Any()).Return(true, nil).AnyTimes()
	mocks.network.EXPECT().IfaceExists(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, name string) (bool, error) { return host.ifaces[name], nil },
	).AnyTimes()

	return app, mocks
}

func (m reconcileMocks) vmDir(vm *models.MicroVM) string {
	return filepath.Join(reconcileStateDir, "vm", vm.ID.String())
}

func (m reconcileMocks) expectNetworkCreate(times int) {
	m.network.EXPECT().IfaceCreate(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, in ports.IfaceCreateInput) (*ports.IfaceDetails, error) {
			m.host.ifaces[in.DeviceName] = true

			return &ports.IfaceDetails{DeviceName: in.DeviceName}, nil
		}).Times(times)
}

func (m reconcileMocks) expectNetworkDelete(times int) {
	m.network.EXPECT().IfaceDelete(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, in ports.DeleteIfaceInput) error {
			delete(m.host.ifaces, in.DeviceName)

			return nil
		}).Times(times)
}

// expectVolumeRestore makes the volume service create the drives of vm, or fail
// after it made the state directory.
func (m reconcileMocks) expectVolumeRestore(vm *models.MicroVM, err error) {
	m.volumes.EXPECT().Restore(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, _ ports.VolumeRestoreInput) (*ports.VolumeMounts, error) {
			dir := filepath.Join(m.vmDir(vm), "volumes")
			Expect(m.fs.MkdirAll(dir, 0o755)).To(Succeed())

			if err != nil {
				return nil, err
			}

			rw := filepath.Join(dir, "root.rw")
			base := filepath.Join(dir, "root.base")

			for _, path := range []string{rw, base} {
				Expect(afero.WriteFile(m.fs, path, nil, 0o600)).To(Succeed())
			}

			return &ports.VolumeMounts{
				RWMount:    models.Mount{Type: models.MountTypeFile, Source: rw},
				BaseMount:  models.Mount{Type: models.MountTypeFile, Source: base},
				BaseDigest: "abc123",
			}, nil
		})
}

func (m reconcileMocks) expectProviderRestore(vm *models.MicroVM, err error) {
	m.provider.EXPECT().
		Restore(gomock.Any(), vm, restoreSnapshotDir, ports.RestoreInput{MemoryMode: models.MemoryModeFile}).
		DoAndReturn(func(_ context.Context, vm *models.MicroVM, _ string, _ ports.RestoreInput) error {
			if err != nil {
				return err
			}

			m.host.vmm = ports.MicroVMStateRunning
			vm.Status.VSockPath = reconcileVSock

			return nil
		})
}

func (m reconcileMocks) expectGet(vm *models.MicroVM) {
	m.repo.EXPECT().Get(gomock.Any(), gomock.Any()).Return(vm, nil)
}

// expectSave records the spec saved at the end of the reconcile.
func (m reconcileMocks) expectSave() *models.MicroVM {
	saved := &models.MicroVM{}

	m.repo.EXPECT().Save(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, vm *models.MicroVM) (*models.MicroVM, error) {
			*saved = *vm

			return vm, nil
		})

	return saved
}

// pendingRestoreVM is a microvm saved by a restore request, not reconciled yet.
func pendingRestoreVM() *models.MicroVM {
	spec := restoreSourceSpec("mock")
	vmid, _ := models.NewVMID("clone1", "ns1", testUID)

	return &models.MicroVM{
		ID: *vmid,
		Spec: func() models.MicroVMSpec {
			s := *spec
			s.CreatedAt = snapshotTestTime.Unix()
			s.SnapshotSource = &models.SnapshotSource{
				Snapshot:   models.SnapshotRef{UID: testSnapshotUID},
				Name:       "clone1",
				Namespace:  "ns1",
				UID:        testUID,
				MemoryMode: models.MemoryModeFile,
				Generation: 4,
			}

			return s
		}(),
		Status: models.MicroVMStatus{State: models.PendingState},
	}
}

func TestApp_ReconcileMicroVM_Restore(t *testing.T) {
	RegisterTestingT(t)

	app, mocks := newReconcileTestApp(t)
	vm := pendingRestoreVM()

	mocks.expectGet(vm)
	mocks.expectNetworkCreate(2)
	mocks.expectVolumeRestore(vm, nil)
	mocks.snapshots.EXPECT().AddRef(gomock.Any(), testSnapshotUID, testUID).Return(nil)
	mocks.expectProviderRestore(vm, nil)
	mocks.agent.EXPECT().Ping(gomock.Any(), reconcileVSock).Return(nil)
	mocks.events.EXPECT().Publish(gomock.Any(), defaults.TopicMicroVMEvents, &events.MicroVMRestored{
		ID: "clone1", Namespace: "ns1", UID: testUID, SnapshotUID: testSnapshotUID, Generation: 4,
	}).Return(nil)

	// Once restored, the plan mounts the kernel and the initrd that a later cold boot needs.
	mocks.images.EXPECT().PullAndMount(gomock.Any(), gomock.Any()).
		Return([]models.Mount{{Type: models.MountTypeHostPath, Source: "/mnt"}}, nil).Times(2)

	saved := mocks.expectSave()

	Expect(app.ReconcileMicroVM(context.Background(), vm.ID)).To(Succeed())
	Expect(saved.Status.State).To(Equal(models.MicroVMState(models.CreatedState)))
	Expect(saved.Spec.SnapshotSource.Restored).To(BeTrue())
}

func TestApp_ReconcileMicroVM_RestoreFailureCleansUp(t *testing.T) {
	testCases := []struct {
		name   string
		expect func(m reconcileMocks, vm *models.MicroVM)
	}{
		{
			name: "volume restore",
			expect: func(m reconcileMocks, vm *models.MicroVM) {
				m.expectVolumeRestore(vm, errInjected)
			},
		},
		{
			name: "add ref",
			expect: func(m reconcileMocks, vm *models.MicroVM) {
				m.expectVolumeRestore(vm, nil)
				m.snapshots.EXPECT().AddRef(gomock.Any(), testSnapshotUID, testUID).Return(errInjected)
			},
		},
		{
			name: "vmm restore",
			expect: func(m reconcileMocks, vm *models.MicroVM) {
				m.expectVolumeRestore(vm, nil)
				m.snapshots.EXPECT().AddRef(gomock.Any(), testSnapshotUID, testUID).Return(nil)
				m.expectProviderRestore(vm, errInjected)
			},
		},
		{
			name: "guest agent ready",
			expect: func(m reconcileMocks, vm *models.MicroVM) {
				m.expectVolumeRestore(vm, nil)
				m.snapshots.EXPECT().AddRef(gomock.Any(), testSnapshotUID, testUID).Return(nil)
				m.expectProviderRestore(vm, nil)
				m.agent.EXPECT().Ping(gomock.Any(), reconcileVSock).Return(errInjected)
				m.provider.EXPECT().Delete(gomock.Any(), vm.ID.String()).DoAndReturn(
					func(_ context.Context, _ string) error {
						m.host.vmm = ports.MicroVMStatePending

						return nil
					})
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			RegisterTestingT(t)

			app, mocks := newReconcileTestApp(t)
			vm := pendingRestoreVM()

			mocks.expectGet(vm)
			mocks.expectNetworkCreate(2)
			tc.expect(mocks, vm)

			// The clean-up removes the network devices, the volumes and the reference.
			mocks.expectNetworkDelete(2)
			mocks.volumes.EXPECT().
				Release(gomock.Any(), ports.VolumeRef{VMID: vm.ID, VolumeID: "root"}).
				Return(nil)
			mocks.snapshots.EXPECT().ReleaseRef(gomock.Any(), testSnapshotUID, testUID).Return(nil)

			saved := mocks.expectSave()

			Expect(app.ReconcileMicroVM(context.Background(), vm.ID)).To(Succeed())
			Expect(saved.Status.State).To(Equal(models.MicroVMState(models.FailedState)))
			Expect(saved.Status.Retry).To(BeZero())
			Expect(saved.Spec.SnapshotSource.Restored).To(BeFalse())
			Expect(mocks.host.vmm).To(Equal(ports.MicroVMStatePending))
			Expect(mocks.host.ifaces).To(BeEmpty())

			exists, err := afero.DirExists(mocks.fs, mocks.vmDir(vm))
			Expect(err).NotTo(HaveOccurred())
			Expect(exists).To(BeFalse())

			// A failed restore is not retried.
			mocks.expectGet(saved)
			Expect(app.ReconcileMicroVM(context.Background(), vm.ID)).To(Succeed())
		})
	}
}

func TestApp_ReconcileMicroVM_RestoredMicroVMColdBoots(t *testing.T) {
	RegisterTestingT(t)

	app, mocks := newReconcileTestApp(t)
	vm := pendingRestoreVM()
	vm.Spec.SnapshotSource.Restored = true
	vm.Status.State = models.CreatedState

	// The restored microvm has its drives and devices; its VMM has died.
	dir := filepath.Join(mocks.vmDir(vm), "volumes")
	rw := filepath.Join(dir, "root.rw")
	base := filepath.Join(dir, "root.base")

	for _, path := range []string{rw, base} {
		Expect(afero.WriteFile(mocks.fs, path, nil, 0o600)).To(Succeed())
	}

	vm.Status.Volumes = models.VolumeStatuses{"root": &models.VolumeStatus{
		Mount:     models.Mount{Type: models.MountTypeFile, Source: rw},
		BaseMount: &models.Mount{Type: models.MountTypeFile, Source: base},
	}}
	vm.Status.NetworkInterfaces = models.NetworkInterfaceStatuses{
		"eth0": {HostDeviceName: "tap0"},
		"eth1": {HostDeviceName: "tap1"},
	}
	mocks.host.ifaces["tap0"] = true
	mocks.host.ifaces["tap1"] = true

	mocks.expectGet(vm)
	mocks.images.EXPECT().PullAndMount(gomock.Any(), gomock.Any()).
		Return([]models.Mount{{Type: models.MountTypeHostPath, Source: "/mnt"}}, nil).Times(2)

	// The normal create plan boots it from its own disk; nothing is restored.
	mocks.provider.EXPECT().Create(gomock.Any(), vm).DoAndReturn(
		func(_ context.Context, vm *models.MicroVM) error {
			mocks.host.vmm = ports.MicroVMStateRunning
			vm.Status.VSockPath = reconcileVSock

			return nil
		})
	mocks.agent.EXPECT().Ping(gomock.Any(), reconcileVSock).Return(nil)

	saved := mocks.expectSave()

	Expect(app.ReconcileMicroVM(context.Background(), vm.ID)).To(Succeed())
	Expect(saved.Status.State).To(Equal(models.MicroVMState(models.CreatedState)))
	Expect(saved.Spec.SnapshotSource.Restored).To(BeTrue())
}
