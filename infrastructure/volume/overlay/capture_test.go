package overlay

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	. "github.com/onsi/gomega"
	"golang.org/x/sys/unix"

	"github.com/liquidmetal-dev/flintlock/core/models"
	"github.com/liquidmetal-dev/flintlock/core/ports"
	"github.com/liquidmetal-dev/flintlock/pkg/reflink"
)

func sha256File(t *testing.T, path string) string {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	sum := sha256.Sum256(data)

	return hex.EncodeToString(sum[:])
}

func writeAt(t *testing.T, path string, offset int64, data string) {
	t.Helper()

	f, err := os.OpenFile(path, os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}

	defer f.Close()

	if _, err := f.WriteAt([]byte(data), offset); err != nil {
		t.Fatal(err)
	}
}

func snapshotDir(env *testEnv, uid string) string {
	return filepath.Join(env.root, "snapshots", uid)
}

func restoreInput(name, dir string) ports.VolumeRestoreInput {
	return ports.VolumeRestoreInput{VolumeRef: vmRef(name), SnapshotDir: dir, Owner: "owner-" + name}
}

// createAndCapture creates microvm a on img:1, writes to its disk and
// captures it to snapshots/snap1.
func createAndCapture(t *testing.T, env *testEnv) (*ports.VolumeMounts, *models.CapturedVolume, string) {
	t.Helper()

	ctx := context.Background()

	mounts, err := env.svc.Create(ctx, createInput("a", "img:1"))
	if err != nil {
		t.Fatal(err)
	}

	writeAt(t, mounts.RWMount.Source, 4096, "guest writes before the capture")

	dir := snapshotDir(env, "snap1")

	captured, err := env.svc.Capture(ctx, vmRef("a"), dir)
	if err != nil {
		t.Fatal(err)
	}

	return mounts, captured, dir
}

func TestService_Capture(t *testing.T) {
	g := NewWithT(t)
	env := newTestService(t, t.TempDir())
	ctx := context.Background()

	mounts, captured, dir := createAndCapture(t, env)
	rw := filepath.Join(dir, "root.rw")

	info, err := os.Stat(rw)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(info.Mode().Perm()).To(Equal(os.FileMode(0o600)))
	g.Expect(sha256File(t, rw)).To(Equal(sha256File(t, mounts.RWMount.Source)))

	target, err := os.Readlink(filepath.Join(dir, "root.base"))
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(target).To(Equal("../../bases/" + mounts.BaseDigest + ".img"))

	var fs unix.Statfs_t
	g.Expect(unix.Statfs(dir, &fs)).To(Succeed())

	g.Expect(*captured).To(Equal(models.CapturedVolume{
		BaseKey:    env.builder.key("img:1").String(),
		BaseDigest: mounts.BaseDigest,
		SizeBytes:  int64(128 * megabyte),
		BlockSize:  fs.Bsize,
	}))

	// An existing capture is kept, even when the source has changed.
	want := sha256File(t, rw)
	writeAt(t, mounts.RWMount.Source, 4096, "guest writes after the capture")

	again, err := env.svc.Capture(ctx, vmRef("a"), dir)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(again).To(Equal(captured))
	g.Expect(sha256File(t, rw)).To(Equal(want))
}

func TestService_CaptureReflinkNotSupported(t *testing.T) {
	g := NewWithT(t)
	env := newTestService(t, t.TempDir())
	ctx := context.Background()

	_, err := env.svc.Create(ctx, createInput("a", "img:1"))
	g.Expect(err).NotTo(HaveOccurred())

	env.svc.clone = func(src, dst string, _ os.FileMode) error {
		return fmt.Errorf("cloning %s to %s: %w", src, dst, reflink.ErrNotSupported)
	}

	dir := snapshotDir(env, "snap1")

	_, err = env.svc.Capture(ctx, vmRef("a"), dir)
	g.Expect(errors.Is(err, ErrReflinkNotSupported)).To(BeTrue(), "got %v", err)

	entries, err := os.ReadDir(dir)
	g.Expect(err).NotTo(HaveOccurred())

	for _, entry := range entries {
		g.Expect(entry.Name()).NotTo(HavePrefix(".root.rw"))
		g.Expect(entry.Name()).NotTo(Equal("root.rw"))
	}
}

func TestService_CaptureWithoutVolume(t *testing.T) {
	g := NewWithT(t)
	env := newTestService(t, t.TempDir())

	_, err := env.svc.Capture(context.Background(), vmRef("never"), snapshotDir(env, "snap1"))
	g.Expect(errors.Is(err, os.ErrNotExist)).To(BeTrue(), "got %v", err)
}

func TestService_Restore(t *testing.T) {
	g := NewWithT(t)
	env := newTestService(t, t.TempDir())
	ctx := context.Background()

	_, captured, dir := createAndCapture(t, env)
	want := sha256File(t, filepath.Join(dir, "root.rw"))

	link := filepath.Join(env.root, "bases", "by-source", env.builder.key("img:1").ChainID.String(), ProfileNameErofsV1)
	old := time.Now().Add(-2 * time.Hour)
	g.Expect(setLinkTime(link, old)).To(Succeed())

	b, err := env.svc.Restore(ctx, restoreInput("b", dir))
	g.Expect(err).NotTo(HaveOccurred())

	volumes := filepath.Join(env.root, "vm", "ns", "b", "uid-b", "volumes")
	g.Expect(*b).To(Equal(ports.VolumeMounts{
		BaseMount:  models.Mount{Type: models.MountTypeFile, Source: filepath.Join(volumes, "root.base")},
		RWMount:    models.Mount{Type: models.MountTypeFile, Source: filepath.Join(volumes, "root.rw")},
		BaseDigest: captured.BaseDigest,
	}))

	target, err := os.Readlink(b.BaseMount.Source)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(target).To(Equal("../../../../../bases/" + captured.BaseDigest + ".img"))
	g.Expect(sha256File(t, b.RWMount.Source)).To(Equal(want))

	info, err := os.Lstat(link)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(info.ModTime()).To(BeTemporally(">", old.Add(time.Hour)), "restore refreshes the last use")

	c, err := env.svc.Restore(ctx, restoreInput("c", dir))
	g.Expect(err).NotTo(HaveOccurred())

	// Two restores get their own disks and share the base.
	g.Expect(c.RWMount.Source).NotTo(Equal(b.RWMount.Source))
	g.Expect(inode(t, c.RWMount.Source)).NotTo(Equal(inode(t, b.RWMount.Source)))
	g.Expect(inode(t, c.BaseMount.Source)).To(Equal(inode(t, b.BaseMount.Source)))

	// A restore again keeps the disk, which can hold guest writes.
	writeAt(t, b.RWMount.Source, 4096, "guest writes after the restore")
	written := sha256File(t, b.RWMount.Source)

	_, err = env.svc.Restore(ctx, restoreInput("b", dir))
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(sha256File(t, b.RWMount.Source)).To(Equal(written))

	// Release of a restored microvm leaves the snapshot.
	g.Expect(env.svc.Release(ctx, vmRef("b"))).To(Succeed())
	g.Expect(b.RWMount.Source).NotTo(BeAnExistingFile())
	g.Expect(sha256File(t, filepath.Join(dir, "root.rw"))).To(Equal(want))
	g.Expect(filepath.Join(dir, "root.base")).To(BeAnExistingFile())
}

func TestService_RestoreMissingBase(t *testing.T) {
	g := NewWithT(t)
	env := newTestService(t, t.TempDir())
	ctx := context.Background()

	_, captured, dir := createAndCapture(t, env)

	base := filepath.Join(env.root, "bases", captured.BaseDigest+".img")
	g.Expect(os.Chmod(base, 0o600)).To(Succeed())
	g.Expect(os.Remove(base)).To(Succeed())

	_, err := env.svc.Restore(ctx, restoreInput("b", dir))
	g.Expect(err).To(MatchError(fmt.Sprintf("base %s of snapshot snap1 is missing", captured.BaseDigest)))

	var missing *BaseMissingError
	g.Expect(errors.As(err, &missing)).To(BeTrue())

	g.Expect(filepath.Join(env.root, "vm", "ns", "b")).NotTo(BeADirectory())
}

func TestService_SweepKeepsBaseOfSnapshot(t *testing.T) {
	g := NewWithT(t)
	env := newTestService(t, t.TempDir())
	ctx := context.Background()

	_, captured, dir := createAndCapture(t, env)
	g.Expect(env.svc.Release(ctx, vmRef("a"))).To(Succeed())

	base := filepath.Join(env.root, "bases", captured.BaseDigest+".img")
	link := filepath.Join(env.root, "bases", "by-source", env.builder.key("img:1").ChainID.String(), ProfileNameErofsV1)
	old := time.Now().Add(-2 * time.Hour)
	g.Expect(setLinkTime(link, old)).To(Succeed())

	// Only the snapshot links the base, and its last use is past the
	// retention period.
	g.Expect(env.svc.Sweep(ctx)).To(Succeed())
	g.Expect(base).To(BeAnExistingFile())
	g.Expect(link).To(BeAnExistingFile())

	g.Expect(os.RemoveAll(dir)).To(Succeed())

	g.Expect(env.svc.Sweep(ctx)).To(Succeed())
	g.Expect(base).NotTo(BeAnExistingFile())
	_, err := os.Lstat(link)
	g.Expect(errors.Is(err, os.ErrNotExist)).To(BeTrue())
}

func TestService_SweepKeepsOrphanBaseOfSnapshot(t *testing.T) {
	g := NewWithT(t)
	env := newTestService(t, t.TempDir())
	ctx := context.Background()

	_, captured, dir := createAndCapture(t, env)
	g.Expect(env.svc.Release(ctx, vmRef("a"))).To(Succeed())

	// A base with no by-source link, older than the retention period.
	base := filepath.Join(env.root, "bases", captured.BaseDigest+".img")
	link := filepath.Join(env.root, "bases", "by-source", env.builder.key("img:1").ChainID.String(), ProfileNameErofsV1)
	g.Expect(os.Remove(link)).To(Succeed())

	old := time.Now().Add(-2 * time.Hour)
	g.Expect(os.Chtimes(base, old, old)).To(Succeed())

	g.Expect(env.svc.Sweep(ctx)).To(Succeed())
	g.Expect(base).To(BeAnExistingFile())

	// A restore works without the by-source link.
	_, err := env.svc.Restore(ctx, restoreInput("b", dir))
	g.Expect(err).NotTo(HaveOccurred())

	g.Expect(env.svc.Release(ctx, vmRef("b"))).To(Succeed())
	g.Expect(os.RemoveAll(dir)).To(Succeed())

	g.Expect(env.svc.Sweep(ctx)).To(Succeed())
	g.Expect(base).NotTo(BeAnExistingFile())
}

func TestService_CloneDiskNeverReplacesAPublishedDisk(t *testing.T) {
	g := NewWithT(t)
	env := newTestService(t, t.TempDir())
	dir := t.TempDir()
	path := filepath.Join(dir, "root.rw")

	const callers = 16

	srcs := make([]string, callers)
	for i := range srcs {
		srcs[i] = filepath.Join(dir, fmt.Sprintf("src-%d", i))
		g.Expect(os.WriteFile(srcs[i], []byte(fmt.Sprintf("disk %d", i)), 0o600)).To(Succeed())
	}

	var (
		wg    sync.WaitGroup
		start = make(chan struct{})
		made  = make([]bool, callers)
		errs  = make([]error, callers)
	)

	for i := range srcs {
		wg.Add(1)

		go func(i int) {
			defer wg.Done()
			<-start
			made[i], errs[i] = env.svc.cloneDisk(srcs[i], path)
		}(i)
	}

	close(start)
	wg.Wait()

	winners := []int{}
	for i := range srcs {
		g.Expect(errs[i]).NotTo(HaveOccurred())

		if made[i] {
			winners = append(winners, i)
		}
	}

	g.Expect(winners).To(HaveLen(1))

	data, err := os.ReadFile(path)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(string(data)).To(Equal(fmt.Sprintf("disk %d", winners[0])))

	tmps, err := filepath.Glob(filepath.Join(dir, ".root.rw.tmp-*"))
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(tmps).To(BeEmpty())
}
