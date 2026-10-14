package reflink_test

import (
	"errors"
	"os"
	"runtime"
	"testing"

	. "github.com/onsi/gomega"

	"github.com/liquidmetal-dev/flintlock/pkg/reflink"
)

func TestProbeTempDir(t *testing.T) {
	g := NewWithT(t)

	if runtime.GOOS != "linux" {
		t.Skip("FICLONE is linux only")
	}

	dir := t.TempDir()

	fsType, err := reflink.FilesystemType(dir)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(fsType).NotTo(BeEmpty())

	// The result depends on the filesystem under the temp dir. The test only
	// checks that a failure is reported as ErrNotSupported and that nothing
	// is left behind.
	err = reflink.Probe(dir)
	if err != nil {
		g.Expect(errors.Is(err, reflink.ErrNotSupported)).To(BeTrue(), err.Error())
	}

	g.Expect(dir).To(BeADirectory())
	g.Expect(dirIsEmpty(dir)).To(BeTrue())
}

func TestProbeMissingDir(t *testing.T) {
	g := NewWithT(t)

	err := reflink.Probe(t.TempDir() + "/missing")
	g.Expect(err).To(HaveOccurred())
	g.Expect(errors.Is(err, reflink.ErrNotSupported)).To(BeFalse())
}

func dirIsEmpty(dir string) bool {
	entries, err := os.ReadDir(dir)

	return err == nil && len(entries) == 0
}
