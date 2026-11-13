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
