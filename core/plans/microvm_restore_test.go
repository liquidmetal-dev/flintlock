package plans_test

import (
	"context"
	"errors"
	"testing"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/gomega"

	"github.com/liquidmetal-dev/flintlock/api/events"
	"github.com/liquidmetal-dev/flintlock/core/models"
	"github.com/liquidmetal-dev/flintlock/core/plans"
	"github.com/liquidmetal-dev/flintlock/core/ports"
	portsctx "github.com/liquidmetal-dev/flintlock/core/ports/context"
	"github.com/liquidmetal-dev/flintlock/pkg/defaults"
	"github.com/liquidmetal-dev/flintlock/pkg/planner"
)

const (
	testSnapshotUID = "8f5b2a4e-2b8c-4f4e-9d61-0b6f2f0a7c11"
	testSnapshotDir = "/var/lib/flintlock/snapshots/" + testSnapshotUID
	testVSockPath   = "/tmp/run/" + testUID + "/guest-agent.vsock"
)

// restoreTestSpec is a microvm that is still to be restored from a snapshot.
func restoreTestSpec(mode models.MemoryMode) *models.MicroVM {
	vm := createTestSpec("vmid", "namespace")
	vm.Status.NetworkInterfaces = models.NetworkInterfaceStatuses{}
	vm.Spec.RootVolume.Mode = models.VolumeModeOverlay
	vm.Spec.RootVolume.Size = 4096
	vm.Spec.AllowGuestAgent = true
	vm.Spec.Initrd = &models.Initrd{
		Image:    "ghcr.io/liquidmetal-dev/flintlock-overlay-init:0.16.0",
		Filename: "initrd.img",
	}
	vm.Spec.SnapshotSource = &models.SnapshotSource{
		Snapshot:   models.SnapshotRef{UID: testSnapshotUID},
		Name:       "vmid",
		MemoryMode: mode,
		Generation: 2,
	}

	return vm
}

func newRestorePlan(t *testing.T, vm *models.MicroVM) (planner.Plan, context.Context, *mockList) {
	t.Helper()

	mList, mockedPorts := fakePorts(gomock.NewController(t))
	ctx := portsctx.WithPorts(context.Background(), mockedPorts)

	mList.MicroVMService.EXPECT().Capabilities().Return(models.Capabilities{}).AnyTimes()
	mList.SnapshotRepo.EXPECT().Dir(testSnapshotUID).Return(testSnapshotDir).AnyTimes()

	plan := plans.MicroVMCreateOrUpdatePlan(&plans.CreateOrUpdatePlanInput{
		VM:              vm,
		StateDirectory:  testStateDir,
		SocketDirectory: "/tmp/run",
	})

	return plan, ctx, mList
}

// runSteps runs each step the way the actuator does, and stops at the first error.
func runSteps(ctx context.Context, steps []planner.Procedure) error {
	for _, step := range steps {
		should, err := step.ShouldDo(ctx)
		if err != nil {
			return err
		}

		if !should {
			continue
		}

		if _, err := step.Do(ctx); err != nil {
			return err
		}

		if err := step.Verify(ctx); err != nil {
			return err
		}
	}

	return nil
}

// expectVMMState makes the provider report the state that state points to.
func expectVMMState(mList *mockList, state *ports.MicroVMState) {
	mList.MicroVMService.EXPECT().State(gomock.Any(), testVMIDString).DoAndReturn(
		func(_ context.Context, _ string) (ports.MicroVMState, error) {
			return *state, nil
		}).AnyTimes()
}

func expectNetworkCreate(mList *mockList) {
	mList.NetworkService.EXPECT().IfaceExists(gomock.Any(), gomock.Any()).Return(false, nil).AnyTimes()
	mList.NetworkService.EXPECT().IfaceCreate(gomock.Any(), gomock.Any()).Return(&ports.IfaceDetails{}, nil).Times(2)
}

func TestMicroVMCreateOrUpdatePlan_restore(t *testing.T) {
	RegisterTestingT(t)

	vm := restoreTestSpec(models.MemoryModeFile)
	plan, ctx, mList := newRestorePlan(t, vm)

	vmmState := ports.MicroVMStatePending
	expectVMMState(mList, &vmmState)
	expectNetworkCreate(mList)

	gomock.InOrder(
		mList.VolumeService.EXPECT().Restore(gomock.Any(), ports.VolumeRestoreInput{
			VolumeRef:   ports.VolumeRef{VMID: vm.ID, VolumeID: "root"},
			SnapshotDir: testSnapshotDir,
			Owner:       testVMIDString,
		}).Return(testOverlayMounts(), nil),
		mList.SnapshotRepo.EXPECT().AddRef(gomock.Any(), testSnapshotUID, testUID).Return(nil),
		mList.MicroVMService.EXPECT().
			Restore(gomock.Any(), vm, testSnapshotDir, ports.RestoreInput{MemoryMode: models.MemoryModeFile}).
			DoAndReturn(func(_ context.Context, vm *models.MicroVM, _ string, _ ports.RestoreInput) error {
				vmmState = ports.MicroVMStateRunning
				vm.Status.VSockPath = testVSockPath

				return nil
			}),
		mList.GuestAgent.EXPECT().Ping(gomock.Any(), testVSockPath).Return(nil),
		mList.EventService.EXPECT().Publish(gomock.Any(), defaults.TopicMicroVMEvents, &events.MicroVMRestored{
			ID:          "vmid",
			Namespace:   "namespace",
			UID:         testUID,
			SnapshotUID: testSnapshotUID,
			Generation:  2,
		}).Return(nil),
	)

	steps, err := plan.Create(ctx)
	Expect(err).NotTo(HaveOccurred())
	Expect(stepNames(steps)).To(Equal([]string{
		"io_create_dir",
		"io_create_dir",
		"network_iface_create",
		"network_iface_create",
		"runtime_volume_restore",
		"snapshot_add_ref",
		"microvm_restore",
		"microvm_guest_agent_ready",
		"microvm_restored",
	}))

	Expect(runSteps(ctx, steps)).To(Succeed())

	Expect(vm.Spec.SnapshotSource.Restored).To(BeTrue())
	Expect(vm.Status.Volumes["root"].Mount.Source).To(Equal(testRWPath))
	Expect(vm.Status.Volumes["root"].BaseDigest).To(Equal("abc123"))

	cond, ok := vm.Status.Conditions.Get(models.ConditionGuestAgentReady)
	Expect(ok).To(BeTrue())
	Expect(cond.Status).To(Equal(models.ConditionTrue))
}

func TestMicroVMCreateOrUpdatePlan_restoreCopyTakesNoRef(t *testing.T) {
	RegisterTestingT(t)

	vm := restoreTestSpec(models.MemoryModeCopy)
	plan, ctx, mList := newRestorePlan(t, vm)

	vmmState := ports.MicroVMStatePending
	expectVMMState(mList, &vmmState)
	mList.NetworkService.EXPECT().IfaceExists(gomock.Any(), gomock.Any()).Return(false, nil).AnyTimes()

	steps, err := plan.Create(ctx)
	Expect(err).NotTo(HaveOccurred())
	Expect(stepNames(steps)).To(ContainElement("microvm_restore"))
	Expect(stepNames(steps)).NotTo(ContainElement("snapshot_add_ref"))
}

func TestMicroVMCreateOrUpdatePlan_restoreNeedsTheGuestAgent(t *testing.T) {
	RegisterTestingT(t)

	vm := restoreTestSpec(models.MemoryModeFile)
	plan, ctx, mList := newRestorePlan(t, vm)

	vmmState := ports.MicroVMStatePending
	expectVMMState(mList, &vmmState)
	expectNetworkCreate(mList)

	mList.VolumeService.EXPECT().Restore(gomock.Any(), gomock.Any()).Return(testOverlayMounts(), nil)
	mList.SnapshotRepo.EXPECT().AddRef(gomock.Any(), testSnapshotUID, testUID).Return(nil)
	mList.MicroVMService.EXPECT().Restore(gomock.Any(), vm, testSnapshotDir, gomock.Any()).DoAndReturn(
		func(_ context.Context, _ *models.MicroVM, _ string, _ ports.RestoreInput) error {
			vmmState = ports.MicroVMStateRunning

			return nil
		})
	mList.GuestAgent.EXPECT().Ping(gomock.Any(), gomock.Any()).Return(errors.New("connection refused"))

	steps, err := plan.Create(ctx)
	Expect(err).NotTo(HaveOccurred())

	err = runSteps(ctx, steps)
	Expect(err).To(MatchError(ContainSubstring("the guest agent did not answer after the restore")))
	Expect(vm.Spec.SnapshotSource.Restored).To(BeFalse())
}

func TestMicroVMCreateOrUpdatePlan_restoredMicroVMColdBoots(t *testing.T) {
	RegisterTestingT(t)

	vm := restoreTestSpec(models.MemoryModeFile)
	vm.Spec.SnapshotSource.Restored = true
	vm.Status.State = models.CreatedState

	plan, ctx, mList := newRestorePlan(t, vm)

	vmmState := ports.MicroVMStatePending
	expectVMMState(mList, &vmmState)
	mList.NetworkService.EXPECT().IfaceExists(gomock.Any(), gomock.Any()).Return(false, nil).AnyTimes()

	steps, err := plan.Create(ctx)
	Expect(err).NotTo(HaveOccurred())

	names := stepNames(steps)
	Expect(names).To(ContainElements(
		"runtime_overlay_volume_mount", "runtime_kernel_mount", "runtime_initrd_mount", "microvm_create",
	))
	Expect(names).NotTo(ContainElements(
		"runtime_volume_restore", "snapshot_add_ref", "microvm_restore", "microvm_restored",
	))
}
