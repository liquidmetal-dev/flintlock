package overlay

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	. "github.com/onsi/gomega"
	"github.com/opencontainers/go-digest"
	"golang.org/x/sys/unix"

	"github.com/liquidmetal-dev/flintlock/core/models"
	"github.com/liquidmetal-dev/flintlock/core/ports"
)

// The fake mkfs lives in the test binary, see TestMain. These are the
// environment variables it reads.
const (
	testFakeMkfsEnv      = "FLINTLOCK_TEST_FAKE_MKFS"
	testFakeMkfsLogEnv   = "FLINTLOCK_TEST_FAKE_MKFS_LOG"
	testFakeMkfsFailEnv  = "FLINTLOCK_TEST_FAKE_MKFS_FAIL"
	testFakeMkfsSleepEnv = "FLINTLOCK_TEST_FAKE_MKFS_SLEEP"
)

// fakeBaseBuilder writes one base file per image reference and records it
// in the index, as the real builder does, without mkfs.erofs.
type fakeBaseBuilder struct {
	root  string
	index baseIndex
	calls int
}

func (b *fakeBaseBuilder) key(imageRef string) baseKey {
	sum := sha256.Sum256([]byte("chain:" + imageRef))

	return baseKey{ChainID: digest.NewDigestFromBytes(digest.SHA256, sum[:]), Profile: ProfileNameErofsV1}
}

func (b *fakeBaseBuilder) Build(ctx context.Context, in LayerInput) (*Base, error) {
	b.calls++
	key := b.key(in.ImageRef)

	release, err := b.index.Lock(ctx, key)
	if err != nil {
		return nil, err
	}

	defer func() { _ = release() }()

	base, err := b.index.Lookup(key)
	if err == nil {
		return base, b.index.Touch(key)
	}

	content := []byte("base of " + in.ImageRef)
	sum := sha256.Sum256(content)
	path := filepath.Join(b.root, basesDir, hex.EncodeToString(sum[:])+baseExt)

	if err := os.WriteFile(path, content, basePerm); err != nil {
		return nil, err
	}

	if err := b.index.Record(key, path); err != nil {
		return nil, err
	}

	return &Base{Digest: hex.EncodeToString(sum[:]), Path: path, ChainID: key.ChainID, Profile: key.Profile}, nil
}

// copyClone stands in for FICLONE on a filesystem without reflinks.
func copyClone(src, dst string, perm os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}

	defer in.Close()

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_EXCL, perm)
	if err != nil {
		return err
	}

	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()

		return err
	}

	return out.Close()
}

type testEnv struct {
	svc     *Service
	root    string
	builder *fakeBaseBuilder
	mkfsLog string
}

// newTestService returns a Service over a fresh state root with the fake
// builder, the test binary as mkfs.ext4 and a copy in place of FICLONE.
func newTestService(t *testing.T, root string) *testEnv {
	t.Helper()

	self, err := os.Executable()
	if err != nil {
		t.Fatalf("locating test binary: %v", err)
	}

	logPath := filepath.Join(t.TempDir(), "mkfs.log")

	t.Setenv(testFakeMkfsEnv, "1")
	t.Setenv(testFakeMkfsLogEnv, logPath)

	index, err := newFSIndex(root)
	if err != nil {
		t.Fatal(err)
	}

	tmpl, err := newTemplates(root, self)
	if err != nil {
		t.Fatal(err)
	}

	builder := &fakeBaseBuilder{root: root, index: index}

	return &testEnv{
		svc: &Service{
			root:          root,
			builder:       builder,
			index:         index,
			templates:     tmpl,
			clone:         copyClone,
			defaultSizeMB: 128,
			retention:     time.Hour,
		},
		root:    root,
		builder: builder,
		mkfsLog: logPath,
	}
}

func (e *testEnv) mkfsRuns(t *testing.T) []string {
	t.Helper()

	data, err := os.ReadFile(e.mkfsLog)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}

		t.Fatal(err)
	}

	return strings.Split(strings.TrimSpace(string(data)), "\n")
}

func vmRef(name string) ports.VolumeRef {
	return ports.VolumeRef{VMID: *models.NewVMIDForce(name, "ns", "uid-"+name), VolumeID: "root"}
}

func createInput(name, image string) ports.VolumeCreateInput {
	return ports.VolumeCreateInput{VolumeRef: vmRef(name), ImageRef: image, Owner: "owner-" + name}
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

func TestService_Create(t *testing.T) {
	g := NewWithT(t)
	env := newTestService(t, t.TempDir())
	ctx := context.Background()

	mounts, err := env.svc.Create(ctx, createInput("a", "img:1"))
	g.Expect(err).NotTo(HaveOccurred())

	volumes := filepath.Join(env.root, "vm", "ns", "a", "uid-a", "volumes")
	g.Expect(mounts.BaseMount).To(Equal(models.Mount{Type: models.MountTypeFile, Source: filepath.Join(volumes, "root.base")}))
	g.Expect(mounts.RWMount).To(Equal(models.Mount{Type: models.MountTypeFile, Source: filepath.Join(volumes, "root.rw")}))

	target, err := os.Readlink(mounts.BaseMount.Source)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(target).To(Equal("../../../../../bases/" + mounts.BaseDigest + ".img"))

	resolved, err := filepath.EvalSymlinks(mounts.BaseMount.Source)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(resolved).To(Equal(filepath.Join(env.root, "bases", mounts.BaseDigest+".img")))

	info, err := os.Stat(mounts.RWMount.Source)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(info.Size()).To(Equal(int64(128 * megabyte)))
	g.Expect(info.Mode().Perm()).To(Equal(os.FileMode(0o600)))

	head := make([]byte, 64)
	f, err := os.Open(mounts.RWMount.Source)
	g.Expect(err).NotTo(HaveOccurred())
	n, _ := f.Read(head)
	f.Close()
	g.Expect(string(head[:n])).To(HavePrefix("ext4 " + templateUUID().String() + " flintlock-rw\n"))

	runs := env.mkfsRuns(t)
	g.Expect(runs).To(HaveLen(1))
	g.Expect(runs[0]).To(HavePrefix("-q -E lazy_itable_init=0,lazy_journal_init=0 -U " + templateUUID().String() +
		" -L flintlock-rw " + filepath.Join(env.root, "scratch", "rw-128-")))

	g.Expect(filepath.Join(env.root, "templates", "rw-128.img")).To(BeAnExistingFile())

	scratch, err := os.ReadDir(filepath.Join(env.root, "scratch"))
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(scratch).To(BeEmpty())
}

func TestService_CreateSize(t *testing.T) {
	g := NewWithT(t)
	env := newTestService(t, t.TempDir())
	ctx := context.Background()

	in := createInput("a", "img:1")
	in.SizeMB = 64

	mounts, err := env.svc.Create(ctx, in)
	g.Expect(err).NotTo(HaveOccurred())

	info, err := os.Stat(mounts.RWMount.Source)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(info.Size()).To(Equal(int64(64 * megabyte)))
	g.Expect(filepath.Join(env.root, "templates", "rw-64.img")).To(BeAnExistingFile())

	in = createInput("b", "img:1")
	in.SizeMB = 63

	_, err = env.svc.Create(ctx, in)
	g.Expect(errors.Is(err, ErrDiskTooSmall)).To(BeTrue(), "got %v", err)
	g.Expect(env.mkfsRuns(t)).To(HaveLen(1))
}

func TestService_CreateIsIdempotent(t *testing.T) {
	g := NewWithT(t)
	env := newTestService(t, t.TempDir())
	ctx := context.Background()

	first, err := env.svc.Create(ctx, createInput("a", "img:1"))
	g.Expect(err).NotTo(HaveOccurred())

	// The guest wrote to its disk.
	g.Expect(os.WriteFile(first.RWMount.Source, []byte("guest writes"), 0o600)).To(Succeed())
	rwInode := inode(t, first.RWMount.Source)

	second, err := env.svc.Create(ctx, createInput("a", "img:1"))
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(second).To(Equal(first))

	content, err := os.ReadFile(second.RWMount.Source)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(string(content)).To(Equal("guest writes"))
	g.Expect(inode(t, second.RWMount.Source)).To(Equal(rwInode))
	g.Expect(env.mkfsRuns(t)).To(HaveLen(1))
}

func TestService_CreateRelinksAnotherBase(t *testing.T) {
	g := NewWithT(t)
	env := newTestService(t, t.TempDir())
	ctx := context.Background()

	first, err := env.svc.Create(ctx, createInput("a", "img:1"))
	g.Expect(err).NotTo(HaveOccurred())

	// The spec now names another image, so the key differs.
	second, err := env.svc.Create(ctx, createInput("a", "img:2"))
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(second.BaseDigest).NotTo(Equal(first.BaseDigest))
	g.Expect(second.BaseMount).To(Equal(first.BaseMount))

	target, err := os.Readlink(second.BaseMount.Source)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(target).To(Equal("../../../../../bases/" + second.BaseDigest + ".img"))

	// The writable disk is still kept.
	g.Expect(second.RWMount).To(Equal(first.RWMount))
	g.Expect(env.mkfsRuns(t)).To(HaveLen(1))
}

func TestService_SecondVMSharesBase(t *testing.T) {
	g := NewWithT(t)
	env := newTestService(t, t.TempDir())
	ctx := context.Background()

	first, err := env.svc.Create(ctx, createInput("a", "img:1"))
	g.Expect(err).NotTo(HaveOccurred())

	second, err := env.svc.Create(ctx, createInput("b", "img:1"))
	g.Expect(err).NotTo(HaveOccurred())

	g.Expect(second.BaseDigest).To(Equal(first.BaseDigest))
	g.Expect(inode(t, second.BaseMount.Source)).To(Equal(inode(t, first.BaseMount.Source)))
	g.Expect(second.RWMount.Source).NotTo(Equal(first.RWMount.Source))
	g.Expect(inode(t, second.RWMount.Source)).NotTo(Equal(inode(t, first.RWMount.Source)))

	// One template serves both.
	g.Expect(env.mkfsRuns(t)).To(HaveLen(1))

	refs, err := env.svc.index.References()
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(refs).To(HaveLen(2))
}

func TestService_Release(t *testing.T) {
	g := NewWithT(t)
	env := newTestService(t, t.TempDir())
	ctx := context.Background()

	mounts, err := env.svc.Create(ctx, createInput("a", "img:1"))
	g.Expect(err).NotTo(HaveOccurred())

	g.Expect(env.svc.Release(ctx, vmRef("a"))).To(Succeed())

	g.Expect(mounts.RWMount.Source).NotTo(BeAnExistingFile())
	_, err = os.Lstat(mounts.BaseMount.Source)
	g.Expect(errors.Is(err, os.ErrNotExist)).To(BeTrue())

	// The base is kept: its last use is within the retention period.
	g.Expect(filepath.Join(env.root, "bases", mounts.BaseDigest+".img")).To(BeAnExistingFile())

	// Files already gone are a success, and so is a microvm that never had
	// any.
	g.Expect(env.svc.Release(ctx, vmRef("a"))).To(Succeed())
	g.Expect(env.svc.Release(ctx, vmRef("never"))).To(Succeed())
}

func TestService_SweepRetention(t *testing.T) {
	g := NewWithT(t)
	env := newTestService(t, t.TempDir())
	ctx := context.Background()

	mounts, err := env.svc.Create(ctx, createInput("a", "img:1"))
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(env.svc.Release(ctx, vmRef("a"))).To(Succeed())

	base := filepath.Join(env.root, "bases", mounts.BaseDigest+".img")
	link := filepath.Join(env.root, "bases", "by-source", env.builder.key("img:1").ChainID.String(), ProfileNameErofsV1)

	g.Expect(env.svc.Sweep(ctx)).To(Succeed())
	g.Expect(base).To(BeAnExistingFile())

	old := time.Now().Add(-2 * time.Hour)
	g.Expect(setLinkTime(link, old)).To(Succeed())

	g.Expect(env.svc.Sweep(ctx)).To(Succeed())
	g.Expect(base).NotTo(BeAnExistingFile())
	_, err = os.Lstat(link)
	g.Expect(errors.Is(err, os.ErrNotExist)).To(BeTrue())

	// The next create builds the base again.
	again, err := env.svc.Create(ctx, createInput("b", "img:1"))
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(again.BaseDigest).To(Equal(mounts.BaseDigest))
	g.Expect(base).To(BeAnExistingFile())
}

func TestService_SweepKeepsReferencedBase(t *testing.T) {
	g := NewWithT(t)
	env := newTestService(t, t.TempDir())
	ctx := context.Background()

	mounts, err := env.svc.Create(ctx, createInput("a", "img:1"))
	g.Expect(err).NotTo(HaveOccurred())
	_, err = env.svc.Create(ctx, createInput("b", "img:1"))
	g.Expect(err).NotTo(HaveOccurred())

	base := filepath.Join(env.root, "bases", mounts.BaseDigest+".img")
	link := filepath.Join(env.root, "bases", "by-source", env.builder.key("img:1").ChainID.String(), ProfileNameErofsV1)
	g.Expect(setLinkTime(link, time.Now().Add(-2*time.Hour))).To(Succeed())

	// Releasing a sweeps, but b still links the base.
	g.Expect(env.svc.Release(ctx, vmRef("a"))).To(Succeed())
	g.Expect(base).To(BeAnExistingFile())

	g.Expect(env.svc.Release(ctx, vmRef("b"))).To(Succeed())
	g.Expect(base).NotTo(BeAnExistingFile())
}

func TestService_SweepRechecksUnderLock(t *testing.T) {
	g := NewWithT(t)
	env := newTestService(t, t.TempDir())
	ctx := context.Background()

	mounts, err := env.svc.Create(ctx, createInput("a", "img:1"))
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(env.svc.Release(ctx, vmRef("a"))).To(Succeed())

	key := env.builder.key("img:1")
	base := filepath.Join(env.root, "bases", mounts.BaseDigest+".img")
	link := filepath.Join(env.root, "bases", "by-source", key.ChainID.String(), ProfileNameErofsV1)
	g.Expect(setLinkTime(link, time.Now().Add(-2*time.Hour))).To(Succeed())

	// A create holds the key lock while the sweep starts. It touches the
	// link and links the base before it releases the lock, as Build does.
	release, err := env.svc.index.Lock(ctx, key)
	g.Expect(err).NotTo(HaveOccurred())

	done := make(chan error, 1)

	go func() { done <- env.svc.Sweep(ctx) }()

	time.Sleep(3 * lockPoll)
	g.Expect(base).To(BeAnExistingFile())

	g.Expect(env.svc.index.Touch(key)).To(Succeed())
	g.Expect(release()).To(Succeed())

	g.Expect(<-done).To(Succeed())
	g.Expect(base).To(BeAnExistingFile())
	g.Expect(link).To(BeAnExistingFile())
}

func TestService_SweepRemovesOrphanAndDanglingLink(t *testing.T) {
	g := NewWithT(t)
	env := newTestService(t, t.TempDir())
	ctx := context.Background()

	mounts, err := env.svc.Create(ctx, createInput("a", "img:1"))
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(env.svc.Release(ctx, vmRef("a"))).To(Succeed())

	// An old base with no link.
	orphan := filepath.Join(env.root, "bases", strings.Repeat("c", 64)+".img")
	g.Expect(os.WriteFile(orphan, []byte("orphan"), 0o400)).To(Succeed())
	old := time.Now().Add(-2 * time.Hour)
	g.Expect(os.Chtimes(orphan, old, old)).To(Succeed())

	// A fresh base with no link is kept: a build may be recording it.
	fresh := filepath.Join(env.root, "bases", strings.Repeat("d", 64)+".img")
	g.Expect(os.WriteFile(fresh, []byte("fresh"), 0o400)).To(Succeed())

	// An old link whose base is gone.
	base := filepath.Join(env.root, "bases", mounts.BaseDigest+".img")
	link := filepath.Join(env.root, "bases", "by-source", env.builder.key("img:1").ChainID.String(), ProfileNameErofsV1)
	g.Expect(os.Chmod(base, 0o600)).To(Succeed())
	g.Expect(os.Remove(base)).To(Succeed())
	g.Expect(setLinkTime(link, old)).To(Succeed())

	g.Expect(env.svc.Sweep(ctx)).To(Succeed())

	g.Expect(orphan).NotTo(BeAnExistingFile())
	g.Expect(fresh).To(BeAnExistingFile())
	_, err = os.Lstat(link)
	g.Expect(errors.Is(err, os.ErrNotExist)).To(BeTrue())
}

func TestTemplates_LockSerialisesOneSize(t *testing.T) {
	g := NewWithT(t)
	env := newTestService(t, t.TempDir())
	ctx := context.Background()

	t.Setenv(testFakeMkfsSleepEnv, "300ms")

	const callers = 4

	var (
		wg    sync.WaitGroup
		mu    sync.Mutex
		paths []string
		errs  []error
	)

	for range callers {
		wg.Add(1)

		go func() {
			defer wg.Done()

			path, err := env.svc.templates.get(ctx, 64)

			mu.Lock()
			defer mu.Unlock()

			paths = append(paths, path)
			errs = append(errs, err)
		}()
	}

	wg.Wait()

	for _, err := range errs {
		g.Expect(err).NotTo(HaveOccurred())
	}

	for _, path := range paths {
		g.Expect(path).To(Equal(filepath.Join(env.root, "templates", "rw-64.img")))
	}

	g.Expect(env.mkfsRuns(t)).To(HaveLen(1))
}

func TestTemplates_MkfsFailureLeavesNoTemplate(t *testing.T) {
	g := NewWithT(t)
	env := newTestService(t, t.TempDir())
	ctx := context.Background()

	t.Setenv(testFakeMkfsFailEnv, "ext4")

	_, err := env.svc.Create(ctx, createInput("a", "img:1"))

	var ext4Err *Ext4Error

	g.Expect(errors.As(err, &ext4Err)).To(BeTrue(), "got %v", err)
	g.Expect(ext4Err.Stderr).To(ContainSubstring("fake mkfs.ext4: failed"))
	g.Expect(filepath.Join(env.root, "templates", "rw-128.img")).NotTo(BeAnExistingFile())
	g.Expect(filepath.Join(env.root, "vm", "ns", "a", "uid-a", "volumes", "root.rw")).NotTo(BeAnExistingFile())

	scratch, err := os.ReadDir(filepath.Join(env.root, "scratch"))
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(scratch).To(BeEmpty())
}

func TestClearScratch(t *testing.T) {
	g := NewWithT(t)
	root := t.TempDir()

	g.Expect(ClearScratch(root)).To(Succeed(), "no scratch directory is fine")

	scratch := filepath.Join(root, "scratch")
	g.Expect(os.MkdirAll(filepath.Join(scratch, "base-abc"), 0o755)).To(Succeed())
	g.Expect(os.WriteFile(filepath.Join(scratch, "base-abc", "layer-0.erofs"), []byte("x"), 0o644)).To(Succeed())
	g.Expect(os.WriteFile(filepath.Join(scratch, "rw-64-123"), []byte("x"), 0o600)).To(Succeed())

	g.Expect(ClearScratch(root)).To(Succeed())

	entries, err := os.ReadDir(scratch)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(entries).To(BeEmpty())
}

func TestNewService(t *testing.T) {
	g := NewWithT(t)
	root := t.TempDir()

	svc, err := NewService(Config{
		StateRoot:       root,
		MkfsErofsPath:   "/nonexistent/mkfs.erofs",
		DefaultRWSizeMB: 1024,
		BaseRetention:   time.Hour,
	}, nil)
	g.Expect(err).NotTo(HaveOccurred())

	for _, dir := range []string{"bases", "bases/by-source", "templates", "scratch"} {
		info, err := os.Stat(filepath.Join(root, dir))
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(info.IsDir()).To(BeTrue())
	}

	g.Expect(svc.templates.mkfs.path).To(Equal("mkfs.ext4"))
}

// setLinkTime sets the times of a symlink without following it.
func setLinkTime(link string, at time.Time) error {
	ts := unix.NsecToTimeval(at.UnixNano())

	return lutimes(link, ts)
}
