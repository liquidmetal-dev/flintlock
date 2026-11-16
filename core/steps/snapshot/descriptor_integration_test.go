package snapshot_test

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/gomega"
	"github.com/spf13/afero"

	"github.com/liquidmetal-dev/flintlock/core/models"
	"github.com/liquidmetal-dev/flintlock/core/ports"
	"github.com/liquidmetal-dev/flintlock/core/steps/snapshot"
	"github.com/liquidmetal-dev/flintlock/infrastructure/mock"
)

const (
	integrationSecret = "c2VjcmV0LXBhc3N3b3Jk"
	staleContent      = `{"stale":true}`
)

type nopLocker struct{}

func (nopLocker) Lock(string)   {}
func (nopLocker) Unlock(string) {}

// TestCaptureSteps_ExistingFilesWithWideMode_Integration runs the capture
// steps on the real filesystem, over a descriptor.json and a spec.json left
// with mode 0644. The new content must land in 0600 files, and the content
// must never be written into the 0644 files: a reader that opened them before
// still reads the stale content.
func TestCaptureSteps_ExistingFilesWithWideMode_Integration(t *testing.T) {
	RegisterTestingT(t)

	dir := t.TempDir()
	vm := integrationSourceVM()
	ctx := context.Background()

	staleReaders := map[string]*os.File{}

	for _, name := range []string{models.SnapshotDescriptorFilename, models.SnapshotSpecFilename} {
		path := filepath.Join(dir, name)
		Expect(os.WriteFile(path, []byte(staleContent), 0o600)).To(Succeed())
		Expect(os.Chmod(path, 0o644)).To(Succeed())

		f, err := os.Open(path)
		Expect(err).NotTo(HaveOccurred())
		t.Cleanup(func() { _ = f.Close() })

		staleReaders[name] = f
	}

	capture := snapshot.NewCapture(integrationInput(t, vm, dir))

	for _, step := range snapshot.Steps(capture) {
		_, err := step.Do(ctx)
		Expect(err).NotTo(HaveOccurred(), "step %s", step.Name())
	}

	for name, stale := range staleReaders {
		info, err := os.Stat(filepath.Join(dir, name))
		Expect(err).NotTo(HaveOccurred())
		Expect(info.Mode().Perm()).To(Equal(os.FileMode(0o600)), name)

		old, err := io.ReadAll(stale)
		Expect(err).NotTo(HaveOccurred())
		Expect(string(old)).To(Equal(staleContent), "the 0644 file %s never holds the new content", name)
	}

	spec := models.MicroVMSpec{}
	data, err := os.ReadFile(filepath.Join(dir, models.SnapshotSpecFilename))
	Expect(err).NotTo(HaveOccurred())
	Expect(json.Unmarshal(data, &spec)).To(Succeed())
	Expect(spec.Metadata).To(HaveKeyWithValue("user-data", integrationSecret))

	desc := models.SnapshotDescriptor{}
	data, err = os.ReadFile(filepath.Join(dir, models.SnapshotDescriptorFilename))
	Expect(err).NotTo(HaveOccurred())
	Expect(json.Unmarshal(data, &desc)).To(Succeed())
	Expect(desc.FormatVersion).To(Equal(models.SnapshotDescriptorFormatVersion))

	entries, err := os.ReadDir(dir)
	Expect(err).NotTo(HaveOccurred())

	names := []string{}
	for _, entry := range entries {
		names = append(names, entry.Name())
	}

	Expect(names).To(ConsistOf(models.SnapshotDescriptorFilename, models.SnapshotSpecFilename),
		"no temporary file is left behind")
}

// TestCaptureSteps_NewFiles_Integration checks the mode of files the capture
// creates on the real filesystem, whatever the umask lets through.
func TestCaptureSteps_NewFiles_Integration(t *testing.T) {
	RegisterTestingT(t)

	dir := t.TempDir()
	capture := snapshot.NewCapture(integrationInput(t, integrationSourceVM(), dir))

	for _, step := range snapshot.Steps(capture) {
		_, err := step.Do(context.Background())
		Expect(err).NotTo(HaveOccurred(), "step %s", step.Name())
	}

	for _, name := range []string{models.SnapshotDescriptorFilename, models.SnapshotSpecFilename} {
		info, err := os.Stat(filepath.Join(dir, name))
		Expect(err).NotTo(HaveOccurred())
		Expect(info.Mode().Perm()).To(Equal(os.FileMode(0o600)), name)
	}
}

func integrationSourceVM() *models.MicroVM {
	vmid, _ := models.NewVMID("mvm1", "ns1", "ae1ce196-6249-11ec-90d6-0242ac120003")

	return &models.MicroVM{
		ID: *vmid,
		Spec: models.MicroVMSpec{
			Provider:   "mock",
			VCPU:       2,
			MemoryInMb: 2048,
			Kernel:     models.Kernel{Image: "docker.io/linuxkit/kernel:5.4.129"},
			RootVolume: models.Volume{
				ID:   "root",
				Mode: models.VolumeModeOverlay,
				Source: models.VolumeSource{
					Container: &models.ContainerVolumeSource{Image: "docker.io/library/ubuntu:24.04"},
				},
			},
			AllowGuestAgent: true,
			Metadata:        map[string]string{"user-data": integrationSecret},
		},
		Status: models.MicroVMStatus{
			State: models.CreatedState,
			Conditions: models.Conditions{
				{Type: models.ConditionGuestAgentReady, Status: models.ConditionTrue},
			},
		},
	}
}

// integrationInput returns the input of a capture whose services are mocks
// that succeed, and whose filesystem is the real one.
func integrationInput(t *testing.T, vm *models.MicroVM, dir string) snapshot.CaptureInput {
	t.Helper()

	ctrl := gomock.NewController(t)
	provider := mock.NewMockMicroVMService(ctrl)
	volumes := mock.NewMockVolumeService(ctrl)
	images := mock.NewMockImageService(ctrl)
	hostInfo := mock.NewMockHostInfoService(ctrl)

	provider.EXPECT().Capabilities().Return(models.Capabilities{models.SnapshotCapability}).AnyTimes()
	provider.EXPECT().State(gomock.Any(), gomock.Any()).Return(ports.MicroVMStateRunning, nil).AnyTimes()
	provider.EXPECT().Pause(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	provider.EXPECT().Snapshot(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	provider.EXPECT().Resume(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	provider.EXPECT().Drives(gomock.Any()).
		Return(models.Drives{{ID: "rw", Role: models.DriveRoleRootRW}}, nil).AnyTimes()
	volumes.EXPECT().Capture(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(&models.CapturedVolume{BaseDigest: "abc"}, nil).AnyTimes()
	images.EXPECT().Digest(gomock.Any(), gomock.Any()).Return("sha256:abc", nil).AnyTimes()
	hostInfo.EXPECT().Facts(gomock.Any(), gomock.Any()).Return(&models.HostFacts{}, nil).AnyTimes()

	return snapshot.CaptureInput{
		Snapshot:    &models.Snapshot{UID: "8f5b2a4e-2b8c-4f4e-9d61-0b6f2f0a7c11", Source: vm.ID},
		VM:          vm,
		Dir:         dir,
		PauseBudget: time.Minute,
		Provider:    provider,
		Volumes:     volumes,
		Images:      images,
		HostInfo:    hostInfo,
		Locks:       nopLocker{},
		FS:          afero.NewOsFs(),
		Clock:       time.Now,
	}
}
