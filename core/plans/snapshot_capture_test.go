package plans_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/gomega"
	"github.com/spf13/afero"

	"github.com/liquidmetal-dev/flintlock/core/models"
	"github.com/liquidmetal-dev/flintlock/core/plans"
	"github.com/liquidmetal-dev/flintlock/core/ports"
	portsctx "github.com/liquidmetal-dev/flintlock/core/ports/context"
	"github.com/liquidmetal-dev/flintlock/core/steps/snapshot"
	"github.com/liquidmetal-dev/flintlock/infrastructure/mock"
)

const (
	captureSnapshotUID = "8f5b2a4e-2b8c-4f4e-9d61-0b6f2f0a7c11"
	captureDir         = "/var/lib/flintlock/snapshots/" + captureSnapshotUID
	captureSecret      = "I2Nsb3VkLWNvbmZpZwpwYXNzd29yZDogaHVudGVyMgo="
	capturePauseBudget = 10 * time.Second
)

var (
	errCaptureTest   = errors.New("boom")
	errResumeTest    = errors.New("resume refused")
	captureTestStart = time.Date(2026, 11, 14, 9, 0, 0, 0, time.UTC)
)

// recorder records the calls of a capture in order, from the mocks and the
// locker, so a test can check the order across them.
type recorder struct {
	mu    sync.Mutex
	calls []string
}

func (r *recorder) add(call string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.calls = append(r.calls, call)
}

func (r *recorder) list() []string {
	r.mu.Lock()
	defer r.mu.Unlock()

	return append([]string{}, r.calls...)
}

type fakeLocker struct {
	rec  *recorder
	held map[string]bool
}

func (l *fakeLocker) Lock(uid string) {
	l.rec.add("lock " + uid)
	l.held[uid] = true
}

func (l *fakeLocker) Unlock(uid string) {
	l.rec.add("unlock " + uid)
	delete(l.held, uid)
}

type captureMocks struct {
	provider  *mock.MockMicroVMService
	volumes   *mock.MockVolumeService
	hostInfo  *mock.MockHostInfoService
	snapshots *mock.MockSnapshotRepository
	fs        afero.Fs
	rec       *recorder
	locks     *fakeLocker
}

type captureTest struct {
	ctx   context.Context
	m     captureMocks
	vm    *models.MicroVM
	input *plans.SnapshotCapturePlanInput
}

func newCaptureTest(t *testing.T) *captureTest {
	t.Helper()

	ctrl := gomock.NewController(t)
	rec := &recorder{}

	m := captureMocks{
		provider:  mock.NewMockMicroVMService(ctrl),
		volumes:   mock.NewMockVolumeService(ctrl),
		hostInfo:  mock.NewMockHostInfoService(ctrl),
		snapshots: mock.NewMockSnapshotRepository(ctrl),
		fs:        afero.NewMemMapFs(),
		rec:       rec,
		locks:     &fakeLocker{rec: rec, held: map[string]bool{}},
	}

	Expect(m.fs.MkdirAll(captureDir, 0o700)).To(Succeed())

	m.provider.EXPECT().Capabilities().Return(models.Capabilities{models.SnapshotCapability}).AnyTimes()
	m.provider.EXPECT().Drives(gomock.Any()).Return(models.Drives{
		{ID: "root-base", Role: models.DriveRoleRootBase},
		{ID: "root-rw", Role: models.DriveRoleRootRW},
	}, nil).AnyTimes()
	m.snapshots.EXPECT().Dir(captureSnapshotUID).Return(captureDir).AnyTimes()

	clock := captureTestStart
	collection := &ports.Collection{
		MicrovmProviders: map[string]ports.MicroVMService{"mock": m.provider},
		VolumeService:    m.volumes,
		HostInfoService:  m.hostInfo,
		SnapshotRepo:     m.snapshots,
		FileSystem:       m.fs,
		Clock: func() time.Time {
			clock = clock.Add(time.Second)

			return clock
		},
	}

	vm := captureSourceVM()

	return &captureTest{
		ctx: portsctx.WithPorts(context.Background(), collection),
		m:   m,
		vm:  vm,
		input: &plans.SnapshotCapturePlanInput{
			Snapshot: &models.Snapshot{
				UID:       captureSnapshotUID,
				Name:      "snap1",
				Namespace: vm.ID.Namespace(),
				Source:    vm.ID,
				Labels:    map[string]string{"team": "a"},
			},
			VM:               vm,
			PauseBudget:      capturePauseBudget,
			Locks:            m.locks,
			FlintlockVersion: "v0.17.0",
		},
	}
}

func (c *captureTest) run() error {
	return plans.RunSnapshotCapture(c.ctx, c.input, "exec1")
}

func captureSourceVM() *models.MicroVM {
	vm := createTestSpec("mvm1", "ns1")
	vm.Status.State = models.CreatedState
	vm.Status.Conditions = models.Conditions{
		{Type: models.ConditionGuestAgentReady, Status: models.ConditionTrue},
	}
	vm.Spec.AllowGuestAgent = true
	vm.Spec.RootVolume.Mode = models.VolumeModeOverlay
	vm.Spec.Initrd = &models.Initrd{Image: "ghcr.io/liquidmetal-dev/flintlock-overlay-init:0.16.0"}
	vm.Spec.Metadata = map[string]string{"user-data": captureSecret}

	return vm
}

func testCapturedVolume() *models.CapturedVolume {
	return &models.CapturedVolume{BaseDigest: "abc123", SizeBytes: 1 << 30, BlockSize: 4096}
}

func testHostFacts() *models.HostFacts {
	return &models.HostFacts{
		VMMType:           models.VMMTypeFirecracker,
		VMMVersion:        "v1.16.1",
		FCSnapshotVersion: "v4.0.0",
		Arch:              "amd64",
		CPUVendor:         "GenuineIntel",
		CPUModel:          "Intel(R) Xeon(R) Platinum 8488C",
		CPUMicrocode:      "0x2b000603",
		HostKernel:        "6.8.0-45-generic",
	}
}

// expectRunning, expectPause and the others record each call, so a test can
// check the order. Each returns the call for more options.
func (c *captureTest) expectRunning() {
	c.m.provider.EXPECT().State(gomock.Any(), c.vm.ID.String()).DoAndReturn(
		func(_ context.Context, _ string) (ports.MicroVMState, error) {
			c.m.rec.add("state")

			return ports.MicroVMStateRunning, nil
		})
}

func (c *captureTest) expectPause() {
	c.m.provider.EXPECT().Pause(gomock.Any(), c.vm).DoAndReturn(func(_ context.Context, _ *models.MicroVM) error {
		c.m.rec.add("pause")

		return nil
	})
}

func (c *captureTest) expectSnapshot(err error) {
	c.m.provider.EXPECT().Snapshot(gomock.Any(), c.vm, captureDir).DoAndReturn(
		func(_ context.Context, _ *models.MicroVM, _ string) error {
			c.m.rec.add("vmm-snapshot")

			return err
		})
}

func (c *captureTest) expectCapture(err error) {
	ref := ports.VolumeRef{VMID: c.vm.ID, VolumeID: "root"}
	c.m.volumes.EXPECT().Capture(gomock.Any(), ref, captureDir).DoAndReturn(
		func(_ context.Context, _ ports.VolumeRef, _ string) (*models.CapturedVolume, error) {
			c.m.rec.add("disk-capture")

			if err != nil {
				return nil, err
			}

			return testCapturedVolume(), nil
		})
}

func (c *captureTest) expectResume(errs ...error) {
	for _, err := range errs {
		c.m.provider.EXPECT().Resume(gomock.Any(), c.vm).DoAndReturn(func(ctx context.Context, _ *models.MicroVM) error {
			Expect(ctx.Err()).NotTo(HaveOccurred(), "the source is resumed with a live context")
			c.m.rec.add("resume")

			return err
		})
	}
}

func (c *captureTest) expectFacts(err error) {
	c.m.hostInfo.EXPECT().Facts(gomock.Any(), "mock").DoAndReturn(
		func(_ context.Context, _ string) (*models.HostFacts, error) {
			c.m.rec.add("facts")

			if err != nil {
				return nil, err
			}

			return testHostFacts(), nil
		})
}

func (c *captureTest) expectRemoveFiles() {
	c.m.snapshots.EXPECT().RemoveFiles(gomock.Any(), captureSnapshotUID).DoAndReturn(
		func(_ context.Context, _ string) error {
			c.m.rec.add("remove-files")

			return nil
		})
}

func (c *captureTest) readJSON(name string) map[string]any {
	data, err := afero.ReadFile(c.m.fs, filepath.Join(captureDir, name))
	Expect(err).NotTo(HaveOccurred())

	fields := map[string]any{}
	Expect(json.Unmarshal(data, &fields)).To(Succeed())

	return fields
}

func (c *captureTest) fileMode(name string) os.FileMode {
	info, err := c.m.fs.Stat(filepath.Join(captureDir, name))
	Expect(err).NotTo(HaveOccurred())

	return info.Mode().Perm()
}

func lockCall(op string) string {
	return op + " " + testUID
}

func TestSnapshotCapturePlan_Steps(t *testing.T) {
	RegisterTestingT(t)

	c := newCaptureTest(t)
	plan := plans.SnapshotCapturePlan(c.input)

	Expect(plan.Name()).To(Equal(plans.SnapshotCapturePlanName))

	steps, err := plan.Create(c.ctx)
	Expect(err).NotTo(HaveOccurred())
	Expect(stepNames(steps)).To(Equal([]string{
		snapshot.CheckStepName,
		snapshot.PauseStepName,
		snapshot.VMMSnapshotStepName,
		snapshot.DiskCaptureStepName,
		snapshot.ResumeStepName,
		snapshot.DescriptorStepName,
	}))

	steps, err = plan.Create(c.ctx)
	Expect(err).NotTo(HaveOccurred())
	Expect(steps).To(BeEmpty(), "a capture runs once")
}

func TestRunSnapshotCapture_Success(t *testing.T) {
	RegisterTestingT(t)

	c := newCaptureTest(t)
	c.expectRunning()
	c.expectPause()
	c.expectSnapshot(nil)
	c.expectCapture(nil)
	c.expectResume(nil)
	c.expectFacts(nil)

	Expect(c.run()).To(Succeed())

	Expect(c.m.rec.list()).To(Equal([]string{
		"state",
		lockCall("lock"),
		"pause",
		"vmm-snapshot",
		"disk-capture",
		"resume",
		lockCall("unlock"),
		"facts",
	}))
	Expect(c.m.locks.held).To(BeEmpty())
}

func TestRunSnapshotCapture_Descriptor(t *testing.T) {
	RegisterTestingT(t)

	c := newCaptureTest(t)
	c.expectRunning()
	c.expectPause()
	c.expectSnapshot(nil)
	c.expectCapture(nil)
	c.expectResume(nil)
	c.expectFacts(nil)

	Expect(c.run()).To(Succeed())

	raw, err := afero.ReadFile(c.m.fs, filepath.Join(captureDir, models.SnapshotDescriptorFilename))
	Expect(err).NotTo(HaveOccurred())
	Expect(string(raw)).NotTo(ContainSubstring("metadata"))
	Expect(string(raw)).NotTo(ContainSubstring(captureSecret))

	desc := &models.SnapshotDescriptor{}
	Expect(json.Unmarshal(raw, desc)).To(Succeed())
	Expect(desc).To(Equal(&models.SnapshotDescriptor{
		FormatVersion:     models.SnapshotDescriptorFormatVersion,
		VMMType:           models.VMMTypeFirecracker,
		VMMVersion:        "v1.16.1",
		FCSnapshotVersion: "v4.0.0",
		FlintlockVersion:  "v0.17.0",
		Arch:              "amd64",
		CPUVendor:         "GenuineIntel",
		CPUModel:          "Intel(R) Xeon(R) Platinum 8488C",
		CPUMicrocode:      "0x2b000603",
		HostKernel:        "6.8.0-45-generic",
		VCPUs:             2,
		MemoryMB:          2048,
		KernelImage:       models.SnapshotImage{Reference: "docker.io/linuxkit/kernel:5.4.129"},
		InitrdImage:       &models.SnapshotImage{Reference: "ghcr.io/liquidmetal-dev/flintlock-overlay-init:0.16.0"},
		Volumes: []models.SnapshotVolume{{
			DriveID:    "root-rw",
			Order:      1,
			SizeBytes:  1 << 30,
			BlockSize:  4096,
			Filesystem: "ext4",
			BaseDigest: "sha256:abc123",
		}},
		CreatedAt: captureTestStart.Add(time.Second),
		Source:    models.SnapshotOrigin{UID: testUID, Name: "mvm1", Namespace: "ns1"},
		Labels:    map[string]string{"team": "a"},
	}))

	fields := c.readJSON(models.SnapshotDescriptorFilename)
	for _, key := range []string{
		"format_version", "vmm_type", "vmm_version", "fc_snapshot_version", "flintlock_version",
		"arch", "cpu_vendor", "cpu_model", "cpu_microcode", "host_kernel", "vcpus", "memory_mb",
		"kernel_image", "initrd_image", "volumes", "created_at", "source", "labels",
	} {
		Expect(fields).To(HaveKey(key))
	}

	spec := &models.MicroVMSpec{}
	data, err := afero.ReadFile(c.m.fs, filepath.Join(captureDir, models.SnapshotSpecFilename))
	Expect(err).NotTo(HaveOccurred())
	Expect(json.Unmarshal(data, spec)).To(Succeed())
	Expect(spec.Metadata).To(HaveKeyWithValue("user-data", captureSecret))
	Expect(spec.VCPU).To(Equal(int64(2)))

	Expect(c.fileMode(models.SnapshotDescriptorFilename)).To(Equal(os.FileMode(0o600)))
	Expect(c.fileMode(models.SnapshotSpecFilename)).To(Equal(os.FileMode(0o600)))
}

func TestRunSnapshotCapture_FileModeOfExistingFiles(t *testing.T) {
	RegisterTestingT(t)

	c := newCaptureTest(t)

	for _, name := range []string{models.SnapshotDescriptorFilename, models.SnapshotSpecFilename} {
		Expect(afero.WriteFile(c.m.fs, filepath.Join(captureDir, name), []byte("{}"), 0o644)).To(Succeed())
	}

	c.expectRunning()
	c.expectPause()
	c.expectSnapshot(nil)
	c.expectCapture(nil)
	c.expectResume(nil)
	c.expectFacts(nil)

	Expect(c.run()).To(Succeed())

	Expect(c.fileMode(models.SnapshotDescriptorFilename)).To(Equal(os.FileMode(0o600)))
	Expect(c.fileMode(models.SnapshotSpecFilename)).To(Equal(os.FileMode(0o600)))
}

func TestRunSnapshotCapture_FailureAfterPause(t *testing.T) {
	tt := []struct {
		name     string
		expect   func(c *captureTest)
		expected []string
	}{
		{
			name: "vmm snapshot fails",
			expect: func(c *captureTest) {
				c.expectSnapshot(errCaptureTest)
				c.expectResume(nil)
			},
			expected: []string{"vmm-snapshot", "resume", lockCall("unlock"), "remove-files"},
		},
		{
			name: "disk capture fails",
			expect: func(c *captureTest) {
				c.expectSnapshot(nil)
				c.expectCapture(errCaptureTest)
				c.expectResume(nil)
			},
			expected: []string{"vmm-snapshot", "disk-capture", "resume", lockCall("unlock"), "remove-files"},
		},
		{
			name: "resume fails, then the retry resumes",
			expect: func(c *captureTest) {
				c.expectSnapshot(nil)
				c.expectCapture(nil)
				c.expectResume(errCaptureTest, nil)
			},
			expected: []string{"vmm-snapshot", "disk-capture", "resume", "resume", lockCall("unlock"), "remove-files"},
		},
		{
			name: "descriptor fails",
			expect: func(c *captureTest) {
				c.expectSnapshot(nil)
				c.expectCapture(nil)
				c.expectResume(nil)
				c.expectFacts(errCaptureTest)
			},
			expected: []string{"vmm-snapshot", "disk-capture", "resume", lockCall("unlock"), "facts", "remove-files"},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			RegisterTestingT(t)

			c := newCaptureTest(t)
			c.expectRunning()
			c.expectPause()
			tc.expect(c)
			c.expectRemoveFiles()

			err := c.run()

			Expect(err).To(MatchError(errCaptureTest))
			Expect(err.Error()).To(ContainSubstring(captureSnapshotUID))
			Expect(err.Error()).NotTo(ContainSubstring("resuming the source microvm failed"))

			expected := append([]string{"state", lockCall("lock"), "pause"}, tc.expected...)
			Expect(c.m.rec.list()).To(Equal(expected))
			Expect(c.m.locks.held).To(BeEmpty())
		})
	}
}

func TestRunSnapshotCapture_FailedResume(t *testing.T) {
	RegisterTestingT(t)

	c := newCaptureTest(t)
	c.expectRunning()
	c.expectPause()
	c.expectSnapshot(errCaptureTest)
	c.expectResume(errResumeTest)
	c.expectRemoveFiles()

	err := c.run()

	Expect(err).To(MatchError(errCaptureTest))
	Expect(err).To(MatchError(errResumeTest))
	Expect(err.Error()).To(ContainSubstring("resuming the source microvm failed"))
	Expect(c.m.rec.list()).To(Equal([]string{
		"state", lockCall("lock"), "pause", "vmm-snapshot", "resume", lockCall("unlock"), "remove-files",
	}))
	Expect(c.m.locks.held).To(BeEmpty(), "the lock is released so the reconciler can see the paused microvm")
}

func TestRunSnapshotCapture_PauseBudgetAbortsSlowSnapshot(t *testing.T) {
	RegisterTestingT(t)

	c := newCaptureTest(t)
	c.input.PauseBudget = 20 * time.Millisecond

	c.expectRunning()
	c.expectPause()
	c.m.provider.EXPECT().Snapshot(gomock.Any(), c.vm, captureDir).DoAndReturn(
		func(ctx context.Context, _ *models.MicroVM, _ string) error {
			c.m.rec.add("vmm-snapshot")

			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(5 * time.Second):
				return nil
			}
		})
	c.expectResume(nil)
	c.expectRemoveFiles()

	start := time.Now()
	err := c.run()

	Expect(time.Since(start)).To(BeNumerically("<", 2*time.Second))
	Expect(err).To(MatchError(ContainSubstring("pause budget of 20ms exceeded")))

	var budgetErr *snapshot.PauseBudgetExceededError
	Expect(errors.As(err, &budgetErr)).To(BeTrue())
	Expect(budgetErr.Budget).To(Equal(20 * time.Millisecond))

	Expect(c.m.rec.list()).To(Equal([]string{
		"state", lockCall("lock"), "pause", "vmm-snapshot", "resume", lockCall("unlock"), "remove-files",
	}))
}

func TestRunSnapshotCapture_CheckRefuses(t *testing.T) {
	tt := []struct {
		name      string
		mutate    func(vm *models.MicroVM)
		state     ports.MicroVMState
		expectErr string
	}{
		{
			name:      "microvm not created",
			mutate:    func(vm *models.MicroVM) { vm.Status.State = models.PendingState },
			expectErr: "is not running",
		},
		{
			name:      "microvm being deleted",
			mutate:    func(vm *models.MicroVM) { vm.Spec.DeletedAt = 1 },
			expectErr: "is being deleted",
		},
		{
			name:      "root not overlay",
			mutate:    func(vm *models.MicroVM) { vm.Spec.RootVolume.Mode = models.VolumeModeFull },
			expectErr: "not an overlay",
		},
		{
			name: "additional volume",
			mutate: func(vm *models.MicroVM) {
				vm.Spec.AdditionalVolumes = models.Volumes{{ID: "data"}}
			},
			expectErr: "additional volumes",
		},
		{
			name:      "guest agent not ready",
			mutate:    func(vm *models.MicroVM) { vm.Status.Conditions = nil },
			expectErr: "guest agent",
		},
		{
			name:      "vmm paused",
			state:     ports.MicroVMStatePaused,
			expectErr: "vmm of microvm " + testUID + " is not running (state: paused)",
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			RegisterTestingT(t)

			c := newCaptureTest(t)
			if tc.mutate != nil {
				tc.mutate(c.vm)
			}

			if tc.state != "" {
				c.m.provider.EXPECT().State(gomock.Any(), c.vm.ID.String()).Return(tc.state, nil)
			}

			c.expectRemoveFiles()

			err := c.run()

			Expect(err).To(MatchError(ContainSubstring(tc.expectErr)))
			Expect(c.m.rec.list()).To(Equal([]string{"remove-files"}), "nothing is paused or locked")
		})
	}
}

func TestRunSnapshotCapture_NoSnapshotCapability(t *testing.T) {
	RegisterTestingT(t)

	c := newCaptureTest(t)
	other := mock.NewMockMicroVMService(gomock.NewController(t))
	other.EXPECT().Capabilities().Return(models.Capabilities{})

	collection, _ := portsctx.GetPorts(c.ctx)
	collection.MicrovmProviders["mock"] = other

	c.expectRemoveFiles()

	Expect(c.run()).To(MatchError(snapshot.ErrSnapshotNotSupported))
}

func TestRunSnapshotCapture_FailedRemoval(t *testing.T) {
	RegisterTestingT(t)

	c := newCaptureTest(t)
	c.expectRunning()
	c.expectPause()
	c.expectSnapshot(errCaptureTest)
	c.expectResume(nil)
	c.m.snapshots.EXPECT().RemoveFiles(gomock.Any(), captureSnapshotUID).Return(errResumeTest)

	err := c.run()

	Expect(err).To(MatchError(errCaptureTest))
	Expect(err).To(MatchError(errResumeTest))
	Expect(err.Error()).To(ContainSubstring("removing the files of the snapshot failed"))
}
