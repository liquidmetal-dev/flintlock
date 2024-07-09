package runtime_test

import (
	"context"
	"os"
	"testing"

	"github.com/liquidmetal-dev/flintlock/core/steps/runtime"
	g "github.com/onsi/gomega"
	"github.com/spf13/afero"
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
