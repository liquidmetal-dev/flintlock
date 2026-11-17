package application_test

import (
	"context"
	"errors"
	"testing"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/gomega"

	"github.com/liquidmetal-dev/flintlock/core/application"
	coreerrs "github.com/liquidmetal-dev/flintlock/core/errors"
	"github.com/liquidmetal-dev/flintlock/core/models"
	"github.com/liquidmetal-dev/flintlock/core/ports"
)

// powerTestVM returns a microvm in state with the given wanted power state.
func powerTestVM(state models.MicroVMState, power models.PowerState) *models.MicroVM {
	vm := snapshotSourceVM()
	vm.Status.State = state
	vm.Spec.PowerState = power

	return vm
}

type powerCall func(app application.App, ctx context.Context, uid string) error

func pauseCall(app application.App, ctx context.Context, uid string) error {
	return app.PauseMicroVM(ctx, uid)
}

func resumeCall(app application.App, ctx context.Context, uid string) error {
	return app.ResumeMicroVM(ctx, uid)
}

func expectSnapshotsOfSource(mocks snapshotMocks, phases ...models.SnapshotPhase) {
	snapshots := make([]*models.Snapshot, 0, len(phases))
	for _, phase := range phases {
		snapshots = append(snapshots, &models.Snapshot{UID: "snap", Status: models.SnapshotStatus{Phase: phase}})
	}

	mocks.snapshots.EXPECT().
		List(gomock.Any(), ports.SnapshotListOptions{Namespace: "ns1", SourceUID: testUID}).
		Return(snapshots, nil)
}

func TestApp_PauseResumeMicroVM_Rejections(t *testing.T) {
	tt := []struct {
		name   string
		call   powerCall
		uid    string
		setup  func(mocks snapshotMocks)
		expect func(err error)
	}{
		{
			name:   "pause without a uid",
			call:   pauseCall,
			setup:  func(snapshotMocks) {},
			expect: func(err error) { Expect(err).To(MatchError(ContainSubstring("uid is required"))) },
		},
		{
			name: "pause of a microvm that does not exist",
			call: pauseCall,
			uid:  testUID,
			setup: func(mocks snapshotMocks) {
				mocks.repo.EXPECT().Get(gomock.Any(), ports.RepositoryGetOptions{UID: testUID}).Return(nil, nil)
			},
			expect: func(err error) { Expect(coreerrs.IsSpecNotFound(err)).To(BeTrue()) },
		},
		{
			name: "pause of a microvm that is being deleted",
			call: pauseCall,
			uid:  testUID,
			setup: func(mocks snapshotMocks) {
				vm := powerTestVM(models.DeletingState, "")
				vm.Spec.DeletedAt = 1
				mocks.repo.EXPECT().Get(gomock.Any(), gomock.Any()).Return(vm, nil)
			},
			expect: func(err error) {
				Expect(coreerrs.IsFailedPrecondition(err)).To(BeTrue())
				Expect(err).To(MatchError(ContainSubstring("being deleted")))
			},
		},
		{
			name: "pause of a pending microvm",
			call: pauseCall,
			uid:  testUID,
			setup: func(mocks snapshotMocks) {
				mocks.repo.EXPECT().Get(gomock.Any(), gomock.Any()).Return(powerTestVM(models.PendingState, ""), nil)
			},
			expect: func(err error) {
				Expect(coreerrs.IsFailedPrecondition(err)).To(BeTrue())
				Expect(err).To(MatchError(ContainSubstring("is not running (state: pending)")))
			},
		},
		{
			name: "pause of a failed microvm",
			call: pauseCall,
			uid:  testUID,
			setup: func(mocks snapshotMocks) {
				mocks.repo.EXPECT().Get(gomock.Any(), gomock.Any()).Return(powerTestVM(models.FailedState, ""), nil)
			},
			expect: func(err error) { Expect(coreerrs.IsFailedPrecondition(err)).To(BeTrue()) },
		},
		{
			name: "pause while a snapshot is taken",
			call: pauseCall,
			uid:  testUID,
			setup: func(mocks snapshotMocks) {
				mocks.repo.EXPECT().Get(gomock.Any(), gomock.Any()).Return(powerTestVM(models.CreatedState, ""), nil)
				expectSnapshotsOfSource(mocks, models.SnapshotPhaseCapturing)
			},
			expect: func(err error) {
				Expect(coreerrs.IsFailedPrecondition(err)).To(BeTrue())
				Expect(err).To(MatchError(ContainSubstring("only one snapshot")))
			},
		},
		{
			name: "pause of a microvm whose vmm does not run",
			call: pauseCall,
			uid:  testUID,
			setup: func(mocks snapshotMocks) {
				mocks.repo.EXPECT().Get(gomock.Any(), gomock.Any()).Return(powerTestVM(models.CreatedState, ""), nil)
				expectSnapshotsOfSource(mocks)
				mocks.provider.EXPECT().State(gomock.Any(), gomock.Any()).Return(ports.MicroVMStatePending, nil)
			},
			expect: func(err error) {
				Expect(coreerrs.IsFailedPrecondition(err)).To(BeTrue())
				Expect(err).To(MatchError(ContainSubstring("vmm state: pending")))
			},
		},
		{
			name: "pause that the provider fails",
			call: pauseCall,
			uid:  testUID,
			setup: func(mocks snapshotMocks) {
				mocks.repo.EXPECT().Get(gomock.Any(), gomock.Any()).Return(powerTestVM(models.CreatedState, ""), nil)
				expectSnapshotsOfSource(mocks)
				mocks.provider.EXPECT().State(gomock.Any(), gomock.Any()).Return(ports.MicroVMStateRunning, nil)
				mocks.provider.EXPECT().Pause(gomock.Any(), gomock.Any()).Return(errors.New("vmm said no"))
			},
			expect: func(err error) {
				Expect(coreerrs.IsFailedPrecondition(err)).To(BeFalse())
				Expect(err).To(MatchError(ContainSubstring("vmm said no")))
			},
		},
		{
			name:   "resume without a uid",
			call:   resumeCall,
			setup:  func(snapshotMocks) {},
			expect: func(err error) { Expect(err).To(MatchError(ContainSubstring("uid is required"))) },
		},
		{
			name: "resume of a pending microvm",
			call: resumeCall,
			uid:  testUID,
			setup: func(mocks snapshotMocks) {
				mocks.repo.EXPECT().Get(gomock.Any(), gomock.Any()).Return(powerTestVM(models.PendingState, ""), nil)
			},
			expect: func(err error) {
				Expect(coreerrs.IsFailedPrecondition(err)).To(BeTrue())
				Expect(err).To(MatchError(ContainSubstring("is not paused (state: pending)")))
			},
		},
		{
			name: "resume while a snapshot is taken",
			call: resumeCall,
			uid:  testUID,
			setup: func(mocks snapshotMocks) {
				mocks.repo.EXPECT().Get(gomock.Any(), gomock.Any()).Return(powerTestVM(models.CreatedState, ""), nil)
				expectSnapshotsOfSource(mocks, models.SnapshotPhaseQuiescing)
			},
			expect: func(err error) { Expect(coreerrs.IsFailedPrecondition(err)).To(BeTrue()) },
		},
		{
			name: "resume that the provider fails",
			call: resumeCall,
			uid:  testUID,
			setup: func(mocks snapshotMocks) {
				vm := powerTestVM(models.PausedState, models.PowerStatePaused)
				mocks.repo.EXPECT().Get(gomock.Any(), gomock.Any()).Return(vm, nil)
				expectSnapshotsOfSource(mocks)
				mocks.provider.EXPECT().State(gomock.Any(), gomock.Any()).Return(ports.MicroVMStatePaused, nil)
				mocks.provider.EXPECT().Resume(gomock.Any(), gomock.Any()).Return(errors.New("vmm said no"))
			},
			expect: func(err error) { Expect(err).To(MatchError(ContainSubstring("vmm said no"))) },
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			RegisterTestingT(t)

			app, mocks := newSnapshotTestApp(t)
			tc.setup(mocks)

			err := tc.call(app, context.Background(), tc.uid)

			Expect(err).To(HaveOccurred())
			tc.expect(err)
		})
	}
}

func TestApp_PauseResumeMicroVM(t *testing.T) {
	tt := []struct {
		name      string
		call      powerCall
		vm        *models.MicroVM
		vmmState  ports.MicroVMState
		expectVMM func(mocks snapshotMocks)
		// saved is the microvm the call saves, or nil when it saves nothing.
		saved *models.MicroVM
	}{
		{
			name:     "pause of a running microvm",
			call:     pauseCall,
			vm:       powerTestVM(models.CreatedState, ""),
			vmmState: ports.MicroVMStateRunning,
			expectVMM: func(mocks snapshotMocks) {
				mocks.provider.EXPECT().Pause(gomock.Any(), gomock.Any()).Return(nil)
			},
			saved: powerTestVM(models.PausedState, models.PowerStatePaused),
		},
		{
			name:      "pause of a paused microvm does nothing",
			call:      pauseCall,
			vm:        powerTestVM(models.PausedState, models.PowerStatePaused),
			vmmState:  ports.MicroVMStatePaused,
			expectVMM: func(snapshotMocks) {},
		},
		{
			name:     "pause of a paused microvm whose vmm runs pauses the vmm",
			call:     pauseCall,
			vm:       powerTestVM(models.PausedState, models.PowerStatePaused),
			vmmState: ports.MicroVMStateRunning,
			expectVMM: func(mocks snapshotMocks) {
				mocks.provider.EXPECT().Pause(gomock.Any(), gomock.Any()).Return(nil)
			},
		},
		{
			name:      "pause of a running microvm whose vmm is paused records the pause",
			call:      pauseCall,
			vm:        powerTestVM(models.CreatedState, ""),
			vmmState:  ports.MicroVMStatePaused,
			expectVMM: func(snapshotMocks) {},
			saved:     powerTestVM(models.PausedState, models.PowerStatePaused),
		},
		{
			name:     "resume of a paused microvm",
			call:     resumeCall,
			vm:       powerTestVM(models.PausedState, models.PowerStatePaused),
			vmmState: ports.MicroVMStatePaused,
			expectVMM: func(mocks snapshotMocks) {
				mocks.provider.EXPECT().Resume(gomock.Any(), gomock.Any()).Return(nil)
			},
			saved: powerTestVM(models.CreatedState, models.PowerStateRunning),
		},
		{
			name:      "resume of a running microvm does nothing",
			call:      resumeCall,
			vm:        powerTestVM(models.CreatedState, ""),
			vmmState:  ports.MicroVMStateRunning,
			expectVMM: func(snapshotMocks) {},
		},
		{
			name:      "resume of a paused microvm whose vmm runs records the resume",
			call:      resumeCall,
			vm:        powerTestVM(models.PausedState, models.PowerStatePaused),
			vmmState:  ports.MicroVMStateRunning,
			expectVMM: func(snapshotMocks) {},
			saved:     powerTestVM(models.CreatedState, models.PowerStateRunning),
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			RegisterTestingT(t)

			app, mocks := newSnapshotTestApp(t)

			mocks.repo.EXPECT().Get(gomock.Any(), ports.RepositoryGetOptions{UID: testUID}).Return(tc.vm, nil)
			expectSnapshotsOfSource(mocks, models.SnapshotPhaseReady, models.SnapshotPhaseFailed)
			mocks.provider.EXPECT().State(gomock.Any(), tc.vm.ID.String()).Return(tc.vmmState, nil)
			tc.expectVMM(mocks)

			if tc.saved != nil {
				mocks.repo.EXPECT().Save(gomock.Any(), gomock.Eq(tc.saved)).Return(tc.saved, nil)
			}

			Expect(tc.call(app, context.Background(), testUID)).To(Succeed())
		})
	}
}

func TestApp_PauseMicroVM_SaveFails(t *testing.T) {
	RegisterTestingT(t)

	app, mocks := newSnapshotTestApp(t)

	mocks.repo.EXPECT().Get(gomock.Any(), gomock.Any()).Return(powerTestVM(models.CreatedState, ""), nil)
	expectSnapshotsOfSource(mocks)
	mocks.provider.EXPECT().State(gomock.Any(), gomock.Any()).Return(ports.MicroVMStateRunning, nil)
	mocks.provider.EXPECT().Pause(gomock.Any(), gomock.Any()).Return(nil)
	mocks.repo.EXPECT().Save(gomock.Any(), gomock.Any()).Return(nil, errors.New("disk full"))

	err := app.PauseMicroVM(context.Background(), testUID)

	Expect(err).To(MatchError(ContainSubstring("disk full")))
}
