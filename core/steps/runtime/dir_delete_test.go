package runtime_test

import (
	"context"
	"os"
	"testing"

	g "github.com/onsi/gomega"
	"github.com/spf13/afero"

	"github.com/liquidmetal-dev/flintlock/core/steps/runtime"
)

func TestDeleteDirectory_NotExists(t *testing.T) {
	g.RegisterTestingT(t)

	testDir := "/test/or/not-to-test"

	fs := afero.NewMemMapFs()
	ctx := context.Background()

	step := runtime.NewDeleteDirectory(testDir, fs)
	should, shouldErr := step.ShouldDo(ctx)
	extraSteps, doErr := step.Do(ctx)
	verifyErr := step.Verify(ctx)

	g.Expect(should).To(g.BeFalse())
	g.Expect(shouldErr).To(g.BeNil())
	g.Expect(doErr).To(g.BeNil())
	g.Expect(extraSteps).To(g.BeEmpty())
	g.Expect(verifyErr).To(g.BeNil())
}

func TestDeleteDirectory_Exists(t *testing.T) {
	g.RegisterTestingT(t)

	testDir := "/test/or/not-to-test"

	fs := afero.NewMemMapFs()
	ctx := context.Background()

	fs.MkdirAll(testDir, os.ModeDir)

	step := runtime.NewDeleteDirectory(testDir, fs)
	should, shouldErr := step.ShouldDo(ctx)
	extraSteps, doErr := step.Do(ctx)
	verifyErr := step.Verify(ctx)

	g.Expect(should).To(g.BeTrue())
	g.Expect(shouldErr).To(g.BeNil())
	g.Expect(doErr).To(g.BeNil())
	g.Expect(extraSteps).To(g.BeEmpty())
	g.Expect(verifyErr).To(g.BeNil())
}

func TestDeleteDirectoryAndEmptyParents_PrunesOnlyEmptyAncestors(t *testing.T) {
	g.RegisterTestingT(t)

	// Real filesystem: afero's MemMapFs does not model Remove on parents of
	// RemoveAll'd trees reliably.
	stateRoot := t.TempDir()
	vmRoot := stateRoot + "/vm"

	fs := afero.NewOsFs()
	ctx := context.Background()

	// ns/name/uid is deleted; ns/name/other-uid is a live sibling that must
	// keep name alive. ns2/name2/uid2 is the only microvm in its namespace.
	deleted := vmRoot + "/ns/name/uid"
	sibling := vmRoot + "/ns/name/other-uid"
	lone := vmRoot + "/ns2/name2/uid2"

	g.Expect(fs.MkdirAll(deleted, os.ModeDir|0o755)).To(g.Succeed())
	g.Expect(fs.MkdirAll(sibling, os.ModeDir|0o755)).To(g.Succeed())
	g.Expect(fs.MkdirAll(lone, os.ModeDir|0o755)).To(g.Succeed())

	for _, dir := range []string{deleted, lone} {
		step := runtime.NewDeleteDirectoryAndEmptyParents(dir, stateRoot, fs)
		should, shouldErr := step.ShouldDo(ctx)
		g.Expect(shouldErr).To(g.BeNil())
		g.Expect(should).To(g.BeTrue())

		extraSteps, doErr := step.Do(ctx)
		g.Expect(doErr).To(g.BeNil())
		g.Expect(extraSteps).To(g.BeEmpty())
	}

	expectDir := func(dir string, want bool) {
		exists, err := afero.DirExists(fs, dir)
		g.Expect(err).To(g.BeNil())
		g.Expect(exists).To(g.Equal(want), "unexpected existence of %s", dir)
	}

	expectDir(deleted, false)
	expectDir(vmRoot+"/ns/name", true)
	expectDir(sibling, true)
	expectDir(vmRoot+"/ns", true)
	expectDir(lone, false)
	expectDir(vmRoot+"/ns2/name2", false)
	expectDir(vmRoot+"/ns2", false)
	expectDir(vmRoot, true)
	expectDir(stateRoot, true)

	// Deleting the last microvm must prune everything below stateRoot,
	// including vm/ itself, but never stateRoot.
	last := runtime.NewDeleteDirectoryAndEmptyParents(sibling, stateRoot, fs)
	should, shouldErr := last.ShouldDo(ctx)
	g.Expect(shouldErr).To(g.BeNil())
	g.Expect(should).To(g.BeTrue())

	_, doErr := last.Do(ctx)
	g.Expect(doErr).To(g.BeNil())

	expectDir(vmRoot+"/ns/name", false)
	expectDir(vmRoot+"/ns", false)
	expectDir(vmRoot, false)
	expectDir(stateRoot, true)
}

func TestDeleteDirectoryAndEmptyParents_TrailingSlashStopAt(t *testing.T) {
	g.RegisterTestingT(t)

	stateRoot := t.TempDir()
	dir := stateRoot + "/vm/ns/name/uid"

	fs := afero.NewOsFs()
	ctx := context.Background()

	g.Expect(fs.MkdirAll(dir, os.ModeDir|0o755)).To(g.Succeed())

	// A trailing slash on the state root must not disable pruning.
	step := runtime.NewDeleteDirectoryAndEmptyParents(dir, stateRoot+"/", fs)
	extraSteps, doErr := step.Do(ctx)
	g.Expect(doErr).To(g.BeNil())
	g.Expect(extraSteps).To(g.BeEmpty())

	exists, err := afero.DirExists(fs, stateRoot+"/vm")
	g.Expect(err).To(g.BeNil())
	g.Expect(exists).To(g.BeFalse())

	exists, err = afero.DirExists(fs, stateRoot)
	g.Expect(err).To(g.BeNil())
	g.Expect(exists).To(g.BeTrue())
}
