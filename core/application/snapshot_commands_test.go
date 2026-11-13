package application_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/gomega"

	"github.com/liquidmetal-dev/flintlock/api/events"
	"github.com/liquidmetal-dev/flintlock/core/application"
	coreerrs "github.com/liquidmetal-dev/flintlock/core/errors"
	"github.com/liquidmetal-dev/flintlock/core/models"
	"github.com/liquidmetal-dev/flintlock/core/ports"
	"github.com/liquidmetal-dev/flintlock/infrastructure/mock"
	"github.com/liquidmetal-dev/flintlock/pkg/defaults"
)

const (
	testSnapshotUID = "8f5b2a4e-2b8c-4f4e-9d61-0b6f2f0a7c11"
)

var snapshotTestTime = time.Date(2026, 11, 13, 10, 0, 0, 0, time.UTC)

type snapshotMocks struct {
	repo      *mock.MockMicroVMRepository
	snapshots *mock.MockSnapshotRepository
	events    *mock.MockEventService
	provider  *mock.MockMicroVMService
}

func newSnapshotTestApp(t *testing.T) (application.App, snapshotMocks) {
	t.Helper()

	mockCtrl := gomock.NewController(t)

	mocks := snapshotMocks{
		repo:      mock.NewMockMicroVMRepository(mockCtrl),
		snapshots: mock.NewMockSnapshotRepository(mockCtrl),
		events:    mock.NewMockEventService(mockCtrl),
		provider:  mock.NewMockMicroVMService(mockCtrl),
	}

	app := application.New(&application.Config{DefaultProvider: "mock"}, &ports.Collection{
		Repo:             mocks.repo,
		SnapshotRepo:     mocks.snapshots,
		MicrovmProviders: map[string]ports.MicroVMService{"mock": mocks.provider},
		EventService:     mocks.events,
		Clock:            func() time.Time { return snapshotTestTime },
	})

	return app, mocks
}

// snapshotSourceVM returns a microvm that a snapshot can be taken of.
func snapshotSourceVM() *models.MicroVM {
	vmid, _ := models.NewVMID("mvm1", "ns1", testUID)

	return &models.MicroVM{
		ID: *vmid,
		Spec: models.MicroVMSpec{
			Provider: "mock",
			RootVolume: models.Volume{
				ID:   "root",
				Mode: models.VolumeModeOverlay,
				Source: models.VolumeSource{
					Container: &models.ContainerVolumeSource{Image: "docker.io/library/ubuntu:24.04"},
				},
			},
			AllowGuestAgent: true,
		},
		Status: models.MicroVMStatus{
			State: models.CreatedState,
			Conditions: models.Conditions{
				{Type: models.ConditionGuestAgentReady, Status: models.ConditionTrue},
			},
		},
	}
}

var snapshotCapabilities = models.Capabilities{models.SnapshotCapability, models.VolumeOverlayCapability}

func TestApp_CreateSnapshot_Rejections(t *testing.T) {
	testCases := []struct {
		name         string
		req          models.CreateSnapshotRequest
		vm           func(vm *models.MicroVM)
		getErr       error
		capabilities models.Capabilities
		existing     []*models.Snapshot
		expectErr    string
		precondition bool
		notFound     bool
	}{
		{
			name:      "no source uid",
			req:       models.CreateSnapshotRequest{},
			expectErr: "uid is required",
		},
		{
			name:         "after stop",
			req:          models.CreateSnapshotRequest{SourceUID: testUID, After: models.SnapshotAfterStop},
			expectErr:    "after stop is not supported",
			precondition: true,
		},
		{
			name:      "source microvm not found",
			req:       models.CreateSnapshotRequest{SourceUID: testUID},
			getErr:    coreerrs.NewSpecNotFound("", "", "", testUID),
			expectErr: "not found",
			notFound:  true,
		},
		{
			name:         "source microvm being deleted",
			req:          models.CreateSnapshotRequest{SourceUID: testUID},
			vm:           func(vm *models.MicroVM) { vm.Status.State = models.DeletingState },
			expectErr:    "is being deleted",
			precondition: true,
		},
		{
			name: "source microvm marked for deletion",
			req:  models.CreateSnapshotRequest{SourceUID: testUID},
			vm: func(vm *models.MicroVM) {
				vm.Spec.DeletedAt = snapshotTestTime.Unix()
			},
			expectErr:    "is being deleted",
			precondition: true,
		},
		{
			name:         "source microvm not created",
			req:          models.CreateSnapshotRequest{SourceUID: testUID},
			vm:           func(vm *models.MicroVM) { vm.Status.State = models.PendingState },
			expectErr:    "is not running (state: pending)",
			precondition: true,
		},
		{
			name:         "provider not available",
			req:          models.CreateSnapshotRequest{SourceUID: testUID},
			vm:           func(vm *models.MicroVM) { vm.Spec.Provider = "other" },
			expectErr:    "microvm provider other isn't available",
			precondition: true,
		},
		{
			name:         "provider without snapshot capability",
			req:          models.CreateSnapshotRequest{SourceUID: testUID},
			capabilities: models.Capabilities{models.VolumeOverlayCapability},
			expectErr:    "needs the snapshot capability",
			precondition: true,
		},
		{
			name:         "root volume not overlay",
			req:          models.CreateSnapshotRequest{SourceUID: testUID},
			vm:           func(vm *models.MicroVM) { vm.Spec.RootVolume.Mode = models.VolumeModeFull },
			expectErr:    "root volume that is not an overlay",
			precondition: true,
		},
		{
			name: "additional virtiofs volume",
			req:  models.CreateSnapshotRequest{SourceUID: testUID},
			vm: func(vm *models.MicroVM) {
				vm.Spec.AdditionalVolumes = models.Volumes{{
					ID:     "share",
					Source: models.VolumeSource{VirtioFS: &models.VirtioFSVolumeSource{Path: "/srv"}},
				}}
			},
			expectErr:    "a virtiofs mount cannot be snapshotted",
			precondition: true,
		},
		{
			name: "additional full volume",
			req:  models.CreateSnapshotRequest{SourceUID: testUID},
			vm: func(vm *models.MicroVM) {
				vm.Spec.AdditionalVolumes = models.Volumes{{
					ID: "data",
					Source: models.VolumeSource{
						Container: &models.ContainerVolumeSource{Image: "docker.io/library/data:1"},
					},
				}}
			},
			expectErr:    "a full volume cannot be snapshotted",
			precondition: true,
		},
		{
			name: "additional volume",
			req:  models.CreateSnapshotRequest{SourceUID: testUID},
			vm: func(vm *models.MicroVM) {
				vm.Spec.AdditionalVolumes = models.Volumes{{ID: "data"}}
			},
			expectErr:    "additional volumes cannot be snapshotted",
			precondition: true,
		},
		{
			name:         "guest agent not allowed",
			req:          models.CreateSnapshotRequest{SourceUID: testUID},
			vm:           func(vm *models.MicroVM) { vm.Spec.AllowGuestAgent = false },
			expectErr:    "does not have allow_guest_agent set",
			precondition: true,
		},
		{
			name:         "guest agent not yet probed",
			req:          models.CreateSnapshotRequest{SourceUID: testUID},
			vm:           func(vm *models.MicroVM) { vm.Status.Conditions = nil },
			expectErr:    "guest agent of microvm",
			precondition: true,
		},
		{
			name: "guest agent not answering",
			req:  models.CreateSnapshotRequest{SourceUID: testUID},
			vm: func(vm *models.MicroVM) {
				vm.Status.Conditions = models.Conditions{
					{Type: models.ConditionGuestAgentReady, Status: models.ConditionFalse, Reason: models.ReasonPingTimeout},
				}
			},
			expectErr:    "guest agent of microvm",
			precondition: true,
		},
		{
			name: "snapshot pending",
			req:  models.CreateSnapshotRequest{SourceUID: testUID},
			existing: []*models.Snapshot{
				{UID: testSnapshotUID, Status: models.SnapshotStatus{Phase: models.SnapshotPhasePending}},
			},
			expectErr:    "in phase pending: only one snapshot at a time",
			precondition: true,
		},
		{
			name: "snapshot capturing",
			req:  models.CreateSnapshotRequest{SourceUID: testUID},
			existing: []*models.Snapshot{
				{UID: "ready1", Status: models.SnapshotStatus{Phase: models.SnapshotPhaseReady}},
				{UID: testSnapshotUID, Status: models.SnapshotStatus{Phase: models.SnapshotPhaseCapturing}},
			},
			expectErr:    "in phase capturing: only one snapshot at a time",
			precondition: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			RegisterTestingT(t)

			app, mocks := newSnapshotTestApp(t)

			vm := snapshotSourceVM()
			if tc.vm != nil {
				tc.vm(vm)
			}

			caps := snapshotCapabilities
			if tc.capabilities != nil {
				caps = tc.capabilities
			}

			if tc.getErr != nil {
				mocks.repo.EXPECT().Get(gomock.Any(), gomock.Any()).Return(nil, tc.getErr).AnyTimes()
			} else {
				mocks.repo.EXPECT().
					Get(gomock.Any(), ports.RepositoryGetOptions{UID: testUID}).
					Return(vm, nil).
					AnyTimes()
			}

			mocks.provider.EXPECT().Capabilities().Return(caps).AnyTimes()
			mocks.snapshots.EXPECT().
				List(gomock.Any(), ports.SnapshotListOptions{Namespace: "ns1", SourceUID: testUID}).
				Return(tc.existing, nil).
				AnyTimes()
			mocks.snapshots.EXPECT().Create(gomock.Any(), gomock.Any()).Times(0)
			mocks.events.EXPECT().Publish(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)

			snapshot, err := app.CreateSnapshot(context.Background(), tc.req)

			Expect(err).To(MatchError(ContainSubstring(tc.expectErr)))
			Expect(snapshot).To(BeNil())
			Expect(coreerrs.IsFailedPrecondition(err)).To(Equal(tc.precondition))
			Expect(coreerrs.IsSpecNotFound(err)).To(Equal(tc.notFound))
		})
	}
}

func TestApp_CreateSnapshot(t *testing.T) {
	testCases := []struct {
		name     string
		after    models.SnapshotAfter
		existing []*models.Snapshot
	}{
		{name: "no snapshot of the source"},
		{name: "after resume", after: models.SnapshotAfterResume},
		{
			name: "only finished snapshots of the source",
			existing: []*models.Snapshot{
				{UID: "ready1", Status: models.SnapshotStatus{Phase: models.SnapshotPhaseReady}},
				{UID: "failed1", Status: models.SnapshotStatus{Phase: models.SnapshotPhaseFailed}},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			RegisterTestingT(t)

			app, mocks := newSnapshotTestApp(t)
			vm := snapshotSourceVM()
			labels := map[string]string{"env": "test"}

			mocks.repo.EXPECT().Get(gomock.Any(), ports.RepositoryGetOptions{UID: testUID}).Return(vm, nil)
			mocks.provider.EXPECT().Capabilities().Return(snapshotCapabilities).AnyTimes()
			mocks.snapshots.EXPECT().
				List(gomock.Any(), ports.SnapshotListOptions{Namespace: "ns1", SourceUID: testUID}).
				Return(tc.existing, nil)

			var created *models.Snapshot

			mocks.snapshots.EXPECT().
				Create(gomock.Any(), gomock.Any()).
				DoAndReturn(func(_ context.Context, snapshot *models.Snapshot) error {
					created = snapshot
					snapshot.UID = testSnapshotUID
					snapshot.Status = models.SnapshotStatus{Phase: models.SnapshotPhasePending}

					return nil
				}).
				Times(1)
			mocks.events.EXPECT().
				Publish(gomock.Any(), defaults.TopicSnapshotEvents, &events.SnapshotCreated{
					UID:       testSnapshotUID,
					Name:      "snap1",
					Namespace: "ns1",
					SourceUID: testUID,
				}).
				Return(nil).
				Times(1)

			snapshot, err := app.CreateSnapshot(context.Background(), models.CreateSnapshotRequest{
				SourceUID: testUID,
				Name:      "snap1",
				Labels:    labels,
				After:     tc.after,
			})

			Expect(err).NotTo(HaveOccurred())
			Expect(snapshot).To(BeIdenticalTo(created))
			Expect(snapshot.Name).To(Equal("snap1"))
			Expect(snapshot.Namespace).To(Equal("ns1"))
			Expect(snapshot.Source).To(Equal(vm.ID))
			Expect(snapshot.Labels).To(Equal(labels))
			Expect(snapshot.CreatedAt).To(Equal(snapshotTestTime))
			Expect(snapshot.Status.Phase).To(Equal(models.SnapshotPhasePending))
		})
	}
}

func TestApp_DeleteSnapshot(t *testing.T) {
	ref := models.SnapshotRef{Name: "snap1", Namespace: "ns1"}

	testCases := []struct {
		name         string
		phase        models.SnapshotPhase
		getErr       error
		expectErr    string
		precondition bool
	}{
		{
			name:         "pending is refused",
			phase:        models.SnapshotPhasePending,
			expectErr:    "snapshot " + testSnapshotUID + " is pending",
			precondition: true,
		},
		{
			name:         "capturing is refused",
			phase:        models.SnapshotPhaseCapturing,
			expectErr:    "snapshot " + testSnapshotUID + " is capturing",
			precondition: true,
		},
		{
			name:      "not found",
			getErr:    coreerrs.NewSnapshotNotFound("ns1/snap1"),
			expectErr: "snapshot ns1/snap1 not found",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			RegisterTestingT(t)

			app, mocks := newSnapshotTestApp(t)

			if tc.getErr != nil {
				mocks.snapshots.EXPECT().Get(gomock.Any(), ref).Return(nil, tc.getErr)
			} else {
				mocks.snapshots.EXPECT().Get(gomock.Any(), ref).Return(&models.Snapshot{
					UID:    testSnapshotUID,
					Status: models.SnapshotStatus{Phase: tc.phase},
				}, nil)
			}

			mocks.snapshots.EXPECT().Delete(gomock.Any(), gomock.Any()).Times(0)
			mocks.events.EXPECT().Publish(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)

			err := app.DeleteSnapshot(context.Background(), ref)

			Expect(err).To(MatchError(ContainSubstring(tc.expectErr)))
			Expect(coreerrs.IsFailedPrecondition(err)).To(Equal(tc.precondition))
			Expect(coreerrs.IsSnapshotNotFound(err)).To(Equal(tc.getErr != nil))
		})
	}
}

// The store, not the app, keeps the memory file while restored microvms reference the
// snapshot: the app passes the delete to the store and releases no reference itself.
func TestApp_DeleteSnapshot_WithRefsDelegatesToStore(t *testing.T) {
	RegisterTestingT(t)

	app, mocks := newSnapshotTestApp(t)
	vmid, _ := models.NewVMID("mvm1", "ns1", testUID)

	mocks.snapshots.EXPECT().
		Get(gomock.Any(), models.SnapshotRef{UID: testSnapshotUID}).
		Return(&models.Snapshot{
			UID:       testSnapshotUID,
			Name:      "snap1",
			Namespace: "ns1",
			Source:    *vmid,
			Status:    models.SnapshotStatus{Phase: models.SnapshotPhaseReady},
		}, nil)
	mocks.snapshots.EXPECT().Delete(gomock.Any(), testSnapshotUID).Return(nil).Times(1)
	mocks.snapshots.EXPECT().ReleaseRef(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
	mocks.events.EXPECT().
		Publish(gomock.Any(), defaults.TopicSnapshotEvents, &events.SnapshotDeleted{
			UID:       testSnapshotUID,
			Name:      "snap1",
			Namespace: "ns1",
			SourceUID: testUID,
		}).
		Return(nil).
		Times(1)

	Expect(app.DeleteSnapshot(context.Background(), models.SnapshotRef{UID: testSnapshotUID})).To(Succeed())
}

func TestApp_GetAllSnapshots_Empty(t *testing.T) {
	RegisterTestingT(t)

	app, mocks := newSnapshotTestApp(t)
	options := ports.SnapshotListOptions{Namespace: "ns1"}

	mocks.snapshots.EXPECT().List(gomock.Any(), options).Return(nil, nil)

	snapshots, err := app.GetAllSnapshots(context.Background(), options)

	Expect(err).NotTo(HaveOccurred())
	Expect(snapshots).NotTo(BeNil())
	Expect(snapshots).To(BeEmpty())
}

func TestApp_CreateSnapshot_PublishFailureMarksRecordFailed(t *testing.T) {
	RegisterTestingT(t)

	app, mocks := newSnapshotTestApp(t)
	vm := snapshotSourceVM()
	ctx, cancel := context.WithCancel(context.Background())

	mocks.repo.EXPECT().Get(gomock.Any(), ports.RepositoryGetOptions{UID: testUID}).Return(vm, nil)
	mocks.provider.EXPECT().Capabilities().Return(snapshotCapabilities).AnyTimes()
	mocks.snapshots.EXPECT().
		List(gomock.Any(), ports.SnapshotListOptions{Namespace: "ns1", SourceUID: testUID}).
		Return(nil, nil)
	mocks.snapshots.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, snapshot *models.Snapshot) error {
			snapshot.UID = testSnapshotUID
			snapshot.Status = models.SnapshotStatus{Phase: models.SnapshotPhasePending}

			return nil
		})

	publishErr := errors.New("context canceled")
	mocks.events.EXPECT().
		Publish(gomock.Any(), defaults.TopicSnapshotEvents, gomock.Any()).
		DoAndReturn(func(context.Context, string, interface{}) error {
			cancel()

			return publishErr
		})

	var updated *models.Snapshot

	mocks.snapshots.EXPECT().
		Update(gomock.Any(), gomock.Any()).
		DoAndReturn(func(updateCtx context.Context, snapshot *models.Snapshot) error {
			Expect(updateCtx.Err()).NotTo(HaveOccurred(), "the update outlives the canceled request")

			updated = snapshot

			return nil
		}).
		Times(1)

	snapshot, err := app.CreateSnapshot(ctx, models.CreateSnapshotRequest{SourceUID: testUID, Name: "snap1"})

	Expect(snapshot).To(BeNil())
	Expect(err).To(MatchError(publishErr))
	Expect(err.Error()).To(ContainSubstring(testSnapshotUID))
	Expect(updated).NotTo(BeNil())
	Expect(updated.UID).To(Equal(testSnapshotUID))
	Expect(updated.Status.Phase).To(Equal(models.SnapshotPhaseFailed))
	Expect(updated.Status.Error).To(ContainSubstring("context canceled"))
}
