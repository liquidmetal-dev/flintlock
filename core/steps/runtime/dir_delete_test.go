package runtime_test

import (
	"context"
	"os"
	"testing"

	"github.com/onsi/gomega"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/weaveworks/flintlock/core/steps/runtime"
)

func TestDeleteDirectory_NotExists(t *testing.T) {
	gomega.RegisterTestingT(t)

	testDir := "/test/or/not-to-test"

	fs := afero.NewMemMapFs()
	ctx := context.Background()

	step := runtime.NewDeleteDirectory(testDir, fs)
	should, shouldErr := step.ShouldDo(ctx)
	extraSteps, doErr := step.Do(ctx)

	assert.NoError(t, shouldErr)
	assert.False(t, should)
	assert.NoError(t, doErr)
	assert.Empty(t, extraSteps)
}

func TestDeleteDirectory_Exists(t *testing.T) {
	gomega.RegisterTestingT(t)

	testDir := "/test/or/not-to-test"

	fs := afero.NewMemMapFs()
	ctx := context.Background()

	fs.MkdirAll(testDir, os.ModeDir)

	step := runtime.NewDeleteDirectory(testDir, fs)
	should, shouldErr := step.ShouldDo(ctx)
	extraSteps, doErr := step.Do(ctx)

	assert.NoError(t, shouldErr)
	assert.True(t, should)
	assert.NoError(t, doErr)
	assert.Empty(t, extraSteps)
}
