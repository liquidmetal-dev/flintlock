package overlay

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	. "github.com/onsi/gomega"
	"golang.org/x/sys/unix"

	"github.com/liquidmetal-dev/flintlock/pkg/reflink"
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

func TestService_CreateWithReflink(t *testing.T) {
	g := NewWithT(t)
	env := newTestService(t, reflinkDir(t))
	env.svc.clone = reflink.Clone
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

func TestService_CaptureAndRestoreWithReflink(t *testing.T) {
	g := NewWithT(t)
	env := newTestService(t, reflinkDir(t))
	env.svc.clone = reflink.Clone
	ctx := context.Background()

	mounts, err := env.svc.Create(ctx, createInput("a", "img:1"))
	g.Expect(err).NotTo(HaveOccurred())
	writeAt(t, mounts.RWMount.Source, 4096, "guest writes before the capture")

	before := sha256File(t, mounts.RWMount.Source)
	dir := snapshotDir(env, "snap1")

	_, err = env.svc.Capture(ctx, vmRef("a"), dir)
	g.Expect(err).NotTo(HaveOccurred())

	captured := filepath.Join(dir, "root.rw")
	g.Expect(sha256File(t, captured)).To(Equal(before))
	g.Expect(inode(t, captured)).NotTo(Equal(inode(t, mounts.RWMount.Source)))

	// The source writes after the capture without changing it.
	writeAt(t, mounts.RWMount.Source, 4096, "guest writes after the capture")
	g.Expect(sha256File(t, mounts.RWMount.Source)).NotTo(Equal(before))
	g.Expect(sha256File(t, captured)).To(Equal(before))

	b, err := env.svc.Restore(ctx, restoreInput("b", dir))
	g.Expect(err).NotTo(HaveOccurred())
	c, err := env.svc.Restore(ctx, restoreInput("c", dir))
	g.Expect(err).NotTo(HaveOccurred())

	g.Expect(sha256File(t, b.RWMount.Source)).To(Equal(before))
	g.Expect(sha256File(t, c.RWMount.Source)).To(Equal(before))
	g.Expect(b.RWMount.Source).NotTo(Equal(c.RWMount.Source))
	g.Expect(inode(t, b.RWMount.Source)).NotTo(Equal(inode(t, c.RWMount.Source)))
	g.Expect(inode(t, b.RWMount.Source)).NotTo(Equal(inode(t, captured)))
	g.Expect(inode(t, b.BaseMount.Source)).To(Equal(inode(t, c.BaseMount.Source)))
	g.Expect(inode(t, b.BaseMount.Source)).To(Equal(inode(t, mounts.BaseMount.Source)))

	// A write to one restored disk reaches neither the other nor the capture.
	writeAt(t, b.RWMount.Source, 4096, "guest writes of b")
	g.Expect(sha256File(t, c.RWMount.Source)).To(Equal(before))
	g.Expect(sha256File(t, captured)).To(Equal(before))
}
