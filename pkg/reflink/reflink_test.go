package reflink_test

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"syscall"
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

// reflinkDir returns a directory on a filesystem that supports FICLONE, or
// skips the test. It tries the temporary directory of the test first and
// then the package directory, which is on the checkout's filesystem.
func reflinkDir(t *testing.T) string {
	t.Helper()

	if dir := t.TempDir(); reflink.Probe(dir) == nil {
		return dir
	}

	dir, err := os.MkdirTemp(".", ".reflink-test-")
	if err != nil {
		t.Skipf("no reflink filesystem: %v", err)
	}

	t.Cleanup(func() { _ = os.RemoveAll(dir) })

	if err := reflink.Probe(dir); err != nil {
		t.Skipf("no reflink filesystem: %v", err)
	}

	abs, err := filepath.Abs(dir)
	if err != nil {
		t.Fatal(err)
	}

	return abs
}

func inode(t *testing.T, path string) uint64 {
	t.Helper()

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}

	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		t.Fatal("no stat_t")
	}

	return stat.Ino
}

func TestClone(t *testing.T) {
	g := NewWithT(t)
	dir := reflinkDir(t)

	src := filepath.Join(dir, "src")
	g.Expect(os.WriteFile(src, []byte("shared blocks"), 0o644)).To(Succeed())

	dst := filepath.Join(dir, "dst")
	g.Expect(reflink.Clone(src, dst, 0o600)).To(Succeed())

	content, err := os.ReadFile(dst)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(string(content)).To(Equal("shared blocks"))

	info, err := os.Stat(dst)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(info.Mode().Perm()).To(Equal(os.FileMode(0o600)))
	g.Expect(inode(t, dst)).NotTo(Equal(inode(t, src)))

	// A write to the clone does not reach the source.
	g.Expect(os.WriteFile(dst, []byte("changed"), 0o600)).To(Succeed())
	content, err = os.ReadFile(src)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(string(content)).To(Equal("shared blocks"))

	// The destination must not exist.
	err = reflink.Clone(src, dst, 0o600)
	g.Expect(errors.Is(err, os.ErrExist)).To(BeTrue(), "got %v", err)
}

func TestClone_Unsupported(t *testing.T) {
	g := NewWithT(t)

	if runtime.GOOS != "linux" {
		t.Skip("FICLONE is linux only")
	}

	dir := t.TempDir()

	if reflink.Probe(dir) == nil {
		t.Skipf("%s supports reflinks", dir)
	}

	src := filepath.Join(dir, "src")
	g.Expect(os.WriteFile(src, []byte("x"), 0o644)).To(Succeed())

	dst := filepath.Join(dir, "dst")
	err := reflink.Clone(src, dst, 0o600)
	g.Expect(errors.Is(err, reflink.ErrNotSupported)).To(BeTrue(), "got %v", err)
	g.Expect(dst).NotTo(BeAnExistingFile())
}
