package runtime_test

import (
	"context"
	"os"
	"testing"

	. "github.com/onsi/gomega"
	g "github.com/onsi/gomega"
	"github.com/spf13/afero"

	"github.com/liquidmetal-dev/flintlock/core/steps/runtime"
)

func TestCreateDirectory_NotExists(t *testing.T) {
	RegisterTestingT(t)

	testDir := "/test/dir"
	testMode := os.ModePerm
	dirMode := testMode | os.ModeDir

	fs := afero.NewMemMapFs()
	ctx := context.Background()

	step := runtime.NewCreateDirectory(testDir, testMode, fs)
	childSteps, err := step.Do(ctx)

	Expect(err).NotTo(HaveOccurred())
	Expect(len(childSteps)).To(Equal(0))

	verifyErr := step.Verify(ctx)
	g.Expect(verifyErr).To(g.BeNil())

	testDirExists(t, testDir, dirMode, fs)
}

func TestCreateDirectory_Exists(t *testing.T) {
	RegisterTestingT(t)

	testDir := "/test/dir"
	testMode := os.ModePerm
	dirMode := testMode | os.ModeDir

	fs := afero.NewMemMapFs()

	err := fs.Mkdir(testDir, testMode)
	Expect(err).NotTo(HaveOccurred())

	ctx := context.Background()
	step := runtime.NewCreateDirectory(testDir, testMode, fs)
	shouldDo, shouldErr := step.ShouldDo(ctx)
	childSteps, err := step.Do(ctx)

	Expect(shouldErr).NotTo(HaveOccurred())
	Expect(shouldDo).To(BeFalse())
	Expect(err).NotTo(HaveOccurred())
	Expect(len(childSteps)).To(Equal(0))

	verifyErr := step.Verify(ctx)
	g.Expect(verifyErr).To(g.BeNil())

	testDirExists(t, testDir, dirMode, fs)
}

func TestCreateDirectory_ExistsButChangeMode(t *testing.T) {
	RegisterTestingT(t)

	testDir := "/test/dir"
	createMode := os.FileMode(0o644)
	changeMode := os.FileMode(0o755)
	dirMode := changeMode | os.ModeDir

	fs := afero.NewMemMapFs()

	err := fs.Mkdir(testDir, createMode)
	Expect(err).NotTo(HaveOccurred())

	ctx := context.Background()
	step := runtime.NewCreateDirectory(testDir, changeMode, fs)
	shouldDo, shouldErr := step.ShouldDo(ctx)
	childSteps, err := step.Do(ctx)

	Expect(shouldErr).NotTo(HaveOccurred())
	Expect(shouldDo).To(BeTrue())
	Expect(err).NotTo(HaveOccurred())
	Expect(len(childSteps)).To(Equal(0))

	verifyErr := step.Verify(ctx)
	g.Expect(verifyErr).To(g.BeNil())

	testDirExists(t, testDir, dirMode, fs)
}

func testDirExists(t *testing.T, dir string, mode os.FileMode, fs afero.Fs) {
	info, err := fs.Stat(dir)
	Expect(err).NotTo(HaveOccurred())
	Expect(info.IsDir()).To(BeTrue())
	Expect(info.Mode().String()).To(Equal(mode.String()))
}
