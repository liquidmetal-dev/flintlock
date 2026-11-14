package overlay

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	. "github.com/onsi/gomega"
	"golang.org/x/sys/unix"
)

func lutimes(path string, tv unix.Timeval) error {
	return unix.Lutimes(path, []unix.Timeval{tv, tv})
}

// reflinkDir returns a directory on a filesystem that supports FICLONE, or
// skips the test. It tries the temporary directory of the test first and
// then the package directory, which is on the checkout's filesystem.
func reflinkDir(t *testing.T) string {
	t.Helper()

	if dir := t.TempDir(); ProbeReflink(dir) == nil {
		return dir
	}

	dir, err := os.MkdirTemp(".", ".reflink-test-")
	if err != nil {
		t.Skipf("no reflink filesystem: %v", err)
	}

	t.Cleanup(func() { _ = os.RemoveAll(dir) })

	if err := ProbeReflink(dir); err != nil {
		t.Skipf("no reflink filesystem: %v", err)
	}

	abs, err := filepath.Abs(dir)
	if err != nil {
		t.Fatal(err)
	}

	return abs
}

func TestCloneFile(t *testing.T) {
	g := NewWithT(t)
	dir := reflinkDir(t)

	src := filepath.Join(dir, "src")
	g.Expect(os.WriteFile(src, []byte("shared blocks"), 0o644)).To(Succeed())

	dst := filepath.Join(dir, "dst")
	g.Expect(cloneFile(src, dst, 0o600)).To(Succeed())

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
	err = cloneFile(src, dst, 0o600)
	g.Expect(errors.Is(err, os.ErrExist)).To(BeTrue(), "got %v", err)
}

func TestCloneFile_Unsupported(t *testing.T) {
	g := NewWithT(t)
	dir := t.TempDir()

	if ProbeReflink(dir) == nil {
		t.Skipf("%s supports reflinks", dir)
	}

	src := filepath.Join(dir, "src")
	g.Expect(os.WriteFile(src, []byte("x"), 0o644)).To(Succeed())

	dst := filepath.Join(dir, "dst")
	err := cloneFile(src, dst, 0o600)
	g.Expect(errors.Is(err, ErrReflinkNotSupported)).To(BeTrue(), "got %v", err)
	g.Expect(dst).NotTo(BeAnExistingFile())

	err = ProbeReflink(dir)
	g.Expect(errors.Is(err, ErrReflinkNotSupported)).To(BeTrue(), "got %v", err)

	entries, err := os.ReadDir(dir)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(entries).To(HaveLen(1), "the probe cleans up")
}

func TestService_CreateWithReflink(t *testing.T) {
	g := NewWithT(t)
	env := newTestService(t, reflinkDir(t))
	env.svc.clone = cloneFile
	ctx := context.Background()

	in := createInput("a", "img:1")
	in.SizeMB = 64

	mounts, err := env.svc.Create(ctx, in)
	g.Expect(err).NotTo(HaveOccurred())

	template := filepath.Join(env.root, "templates", "rw-64.img")
	g.Expect(inode(t, mounts.RWMount.Source)).NotTo(Equal(inode(t, template)))

	info, err := os.Stat(mounts.RWMount.Source)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(info.Size()).To(Equal(int64(64 * megabyte)))

	// The second microvm gets its own clone.
	in = createInput("b", "img:1")
	in.SizeMB = 64

	second, err := env.svc.Create(ctx, in)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(inode(t, second.RWMount.Source)).NotTo(Equal(inode(t, mounts.RWMount.Source)))
	g.Expect(env.mkfsRuns(t)).To(HaveLen(1))
}
