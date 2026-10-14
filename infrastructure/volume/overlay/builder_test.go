package overlay_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	. "github.com/onsi/gomega"

	"github.com/liquidmetal-dev/flintlock/infrastructure/volume/overlay"
)

// fakeBuilder returns a Builder over a fresh state root that runs the test
// binary as mkfs.erofs, and the path of the command log.
func fakeBuilder(t *testing.T, source overlay.LayerSource) (*overlay.Builder, string, string) {
	t.Helper()

	self, err := os.Executable()
	if err != nil {
		t.Fatalf("locating test binary: %v", err)
	}

	stateRoot := t.TempDir()
	logPath := filepath.Join(t.TempDir(), "mkfs.log")

	t.Setenv(fakeMkfsEnv, "1")
	t.Setenv(fakeMkfsLogEnv, logPath)

	b, err := overlay.NewBuilder(stateRoot, self, source)
	if err != nil {
		t.Fatalf("NewBuilder: %v", err)
	}

	return b, stateRoot, logPath
}

func readLog(t *testing.T, path string) [][]string {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}

		t.Fatalf("reading log: %v", err)
	}

	var lines [][]string

	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		if line != "" {
			lines = append(lines, strings.Fields(line))
		}
	}

	return lines
}

func twoLayers(t *testing.T) *memSource {
	t.Helper()

	l1 := tarBytes(t, dir("etc", 0o755), file("etc/a", 0o644, "a"), file("etc/b", 0o644, "b"))
	l2 := tarBytes(t, dir("etc", 0o755), file("etc/a", 0o644, "a2"), file("etc/.wh.b", 0o644, ""))

	return newMemSource(l1, l2)
}

func listBases(t *testing.T, stateRoot string) []string {
	t.Helper()

	matches, err := filepath.Glob(filepath.Join(stateRoot, "bases", "*.img"))
	if err != nil {
		t.Fatal(err)
	}

	return matches
}

func TestBuild_CommandLinesAndLayout(t *testing.T) {
	g := NewWithT(t)
	source := twoLayers(t)
	b, stateRoot, logPath := fakeBuilder(t, source)

	base, err := b.Build(context.Background(), overlay.LayerInput{ImageRef: "img", Owner: "vm"})
	g.Expect(err).NotTo(HaveOccurred())

	lines := readLog(t, logPath)
	g.Expect(lines).To(HaveLen(3))

	ids := source.diffIDs()
	scratch := filepath.Dir(lines[0][7])
	g.Expect(scratch).To(HavePrefix(filepath.Join(stateRoot, "scratch", "base-")))

	for i := range 2 {
		g.Expect(lines[i]).To(Equal([]string{
			"-b4096", "--tar=f", "--aufs", "-T0", "--mkfs-time",
			"-U", overlay.LayerUUID(ids[i]).String(),
			filepath.Join(scratch, "layer-"+string(rune('0'+i))+".erofs"),
			"/dev/stdin",
		}))
	}

	g.Expect(lines[2]).To(Equal([]string{
		"-b4096", "--clean=data", "--ovlfs-strip=1", "-T0", "--mkfs-time",
		"-U", overlay.BaseUUID(source.chainID(), overlay.ProfileNameErofsV1).String(),
		filepath.Join(scratch, "merged.erofs"),
		filepath.Join(scratch, "layer-0.erofs"),
		filepath.Join(scratch, "layer-1.erofs"),
	}))

	// The base is named by its sha256, read-only, and linked by source.
	content, err := os.ReadFile(base.Path)
	g.Expect(err).NotTo(HaveOccurred())
	sum := sha256.Sum256(content)
	g.Expect(base.Digest).To(Equal(hex.EncodeToString(sum[:])))
	g.Expect(base.Path).To(Equal(filepath.Join(stateRoot, "bases", base.Digest+".img")))
	g.Expect(base.ChainID).To(Equal(source.chainID()))
	g.Expect(base.Profile).To(Equal("erofs-v1"))

	info, err := os.Stat(base.Path)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(info.Mode().Perm()).To(Equal(os.FileMode(0o400)))

	link := filepath.Join(stateRoot, "bases", "by-source", source.chainID().String(), "erofs-v1")
	target, err := os.Readlink(link)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(target).To(Equal("../../" + base.Digest + ".img"))

	// The merged content came from both layer files in order.
	g.Expect(string(content)).To(HavePrefix("merge " + overlay.BaseUUID(source.chainID(), "erofs-v1").String() + "\n"))
	g.Expect(string(content)).To(ContainSubstring("layer " + overlay.LayerUUID(ids[0]).String()))
	g.Expect(string(content)).To(ContainSubstring("layer " + overlay.LayerUUID(ids[1]).String()))

	// Scratch is empty afterwards.
	entries, err := os.ReadDir(filepath.Join(stateRoot, "scratch"))
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(entries).To(BeEmpty())
}

func TestBuild_FoundBaseIsNotRebuilt(t *testing.T) {
	g := NewWithT(t)
	source := twoLayers(t)
	b, stateRoot, logPath := fakeBuilder(t, source)
	ctx := context.Background()

	first, err := b.Build(ctx, overlay.LayerInput{ImageRef: "img"})
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(readLog(t, logPath)).To(HaveLen(3))

	link := filepath.Join(stateRoot, "bases", "by-source", source.chainID().String(), "erofs-v1")
	old := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	g.Expect(os.Chtimes(link, old, old)).To(Succeed())

	second, err := b.Build(ctx, overlay.LayerInput{ImageRef: "img"})
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(second).To(Equal(first))
	g.Expect(readLog(t, logPath)).To(HaveLen(3), "a found base runs no mkfs.erofs")

	// The last-use time of the link is refreshed. The base file is not.
	linkInfo, err := os.Lstat(link)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(linkInfo.ModTime()).To(BeTemporally(">", old))
}

func TestBuild_ExistingDigestIsKept(t *testing.T) {
	g := NewWithT(t)
	source := twoLayers(t)
	b, stateRoot, logPath := fakeBuilder(t, source)
	ctx := context.Background()

	first, err := b.Build(ctx, overlay.LayerInput{ImageRef: "img"})
	g.Expect(err).NotTo(HaveOccurred())

	// Drop the link only. The next build runs mkfs.erofs again and finds
	// the file with the same digest in place.
	g.Expect(os.RemoveAll(filepath.Join(stateRoot, "bases", "by-source"))).To(Succeed())
	before, err := os.Stat(first.Path)
	g.Expect(err).NotTo(HaveOccurred())

	second, err := b.Build(ctx, overlay.LayerInput{ImageRef: "img"})
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(second).To(Equal(first))
	g.Expect(readLog(t, logPath)).To(HaveLen(6))

	after, err := os.Stat(first.Path)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(os.SameFile(before, after)).To(BeTrue(), "the existing base file is kept")
	g.Expect(listBases(t, stateRoot)).To(HaveLen(1))

	entries, err := os.ReadDir(filepath.Join(stateRoot, "scratch"))
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(entries).To(BeEmpty())
}

func TestBuild_RefusesOtherVersion(t *testing.T) {
	g := NewWithT(t)
	b, stateRoot, logPath := fakeBuilder(t, twoLayers(t))
	t.Setenv(fakeMkfsVersionEnv, "1.8.10")

	_, err := b.Build(context.Background(), overlay.LayerInput{ImageRef: "img"})

	var verr *overlay.VersionError
	g.Expect(errors.As(err, &verr)).To(BeTrue(), "got %v", err)
	g.Expect(verr.Reported).To(Equal("mkfs.erofs (erofs-utils) 1.8.10"))
	g.Expect(verr.Want).To(Equal("1.9.4"))
	g.Expect(readLog(t, logPath)).To(BeEmpty(), "nothing is built")
	g.Expect(listBases(t, stateRoot)).To(BeEmpty())
}

func TestBuild_MkfsFailureRemovesScratch(t *testing.T) {
	for _, step := range []string{"layer", "merge"} {
		t.Run(step, func(t *testing.T) {
			g := NewWithT(t)
			b, stateRoot, _ := fakeBuilder(t, twoLayers(t))
			t.Setenv(fakeMkfsFailEnv, step)

			_, err := b.Build(context.Background(), overlay.LayerInput{ImageRef: "img"})

			var merr *overlay.MkfsError
			g.Expect(errors.As(err, &merr)).To(BeTrue(), "got %v", err)
			g.Expect(merr.Stderr).To(Equal("fake mkfs.erofs: " + step + " failed"))
			g.Expect(err.Error()).To(ContainSubstring("fake mkfs.erofs: " + step + " failed"))

			entries, err := os.ReadDir(filepath.Join(stateRoot, "scratch"))
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(entries).To(BeEmpty())
			g.Expect(listBases(t, stateRoot)).To(BeEmpty())
		})
	}
}

func TestBuild_HardlinkIntoLowerLayer(t *testing.T) {
	l1 := tarBytes(t, dir("usr", 0o755), dir("usr/bin", 0o755), file("usr/bin/tool", 0o755, "tool"))
	l2 := tarBytes(t,
		dir("usr", 0o755), dir("usr/bin", 0o755),
		file("usr/bin/other", 0o755, "other"),
		hardlink("./usr/bin/tool2", "usr/bin/tool", 0o755),
	)

	// The link comes after 16 KiB of file data, so that a mkfs.erofs that
	// stops at its first block has not read it.
	late := tarBytes(t,
		dir("usr", 0o755),
		file("usr/big", 0o644, strings.Repeat("x", 16*1024)),
		hardlink("usr/bin/tool2", "usr/bin/tool", 0o755),
	)

	tests := []struct {
		name   string
		layers [][]byte
		fail   string
		layer  int
	}{
		{name: "mkfs succeeds", layers: [][]byte{l1, l2}, layer: 1},
		{name: "mkfs stops early", layers: [][]byte{late}, fail: "layer-early", layer: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			source := newMemSource(tt.layers...)
			b, stateRoot, _ := fakeBuilder(t, source)
			t.Setenv(fakeMkfsFailEnv, tt.fail)

			_, err := b.Build(context.Background(), overlay.LayerInput{ImageRef: "img"})

			var herr *overlay.HardlinkError
			g.Expect(errors.As(err, &herr)).To(BeTrue(), "got %v", err)
			g.Expect(herr.LayerIndex).To(Equal(tt.layer))
			g.Expect(herr.DiffID).To(Equal(source.diffIDs()[tt.layer]))
			g.Expect(herr.Entry).To(Equal("usr/bin/tool2"))
			g.Expect(herr.Target).To(Equal("usr/bin/tool"))
			g.Expect(err.Error()).To(ContainSubstring(
				`layer ` + strconv.Itoa(tt.layer) + ` (` + source.diffIDs()[tt.layer].String() +
					`): entry "usr/bin/tool2" is a hardlink to "usr/bin/tool", which is not in the same layer`))
			g.Expect(listBases(t, stateRoot)).To(BeEmpty())
		})
	}
}

func TestBuild_HardlinkWithinLayerIsFine(t *testing.T) {
	g := NewWithT(t)
	l1 := tarBytes(t, dir("usr", 0o755), file("usr/a", 0o644, "a"), hardlink("usr/b", "./usr/a", 0o644))
	b, _, _ := fakeBuilder(t, newMemSource(l1))

	_, err := b.Build(context.Background(), overlay.LayerInput{ImageRef: "img"})
	g.Expect(err).NotTo(HaveOccurred())
}

func TestBuild_SourceErrors(t *testing.T) {
	g := NewWithT(t)
	boom := errors.New("boom")
	b, _, logPath := fakeBuilder(t, failSource{err: boom})

	_, err := b.Build(context.Background(), overlay.LayerInput{ImageRef: "img"})
	g.Expect(errors.Is(err, boom)).To(BeTrue())

	b, _, _ = fakeBuilder(t, newMemSource())
	_, err = b.Build(context.Background(), overlay.LayerInput{ImageRef: "img"})
	g.Expect(err).To(MatchError(ContainSubstring("no layers")))
	g.Expect(readLog(t, logPath)).To(BeEmpty())
}

func TestBuild_ConcurrentBuildsRunOnce(t *testing.T) {
	g := NewWithT(t)
	source := twoLayers(t)
	b1, stateRoot, logPath := fakeBuilder(t, source)
	t.Setenv(fakeMkfsSleepEnv, "300ms")

	self, err := os.Executable()
	g.Expect(err).NotTo(HaveOccurred())

	// A second Builder over the same state root, as a second set of ports
	// in the daemon would have.
	b2, err := overlay.NewBuilder(stateRoot, self, source)
	g.Expect(err).NotTo(HaveOccurred())

	var (
		wg      sync.WaitGroup
		results [2]*overlay.Base
		errs    [2]error
	)

	for i, b := range []*overlay.Builder{b1, b2} {
		wg.Add(1)

		go func() {
			defer wg.Done()

			results[i], errs[i] = b.Build(context.Background(), overlay.LayerInput{ImageRef: "img"})
		}()
	}

	wg.Wait()

	g.Expect(errs[0]).NotTo(HaveOccurred())
	g.Expect(errs[1]).NotTo(HaveOccurred())
	g.Expect(results[0]).To(Equal(results[1]))
	g.Expect(readLog(t, logPath)).To(HaveLen(3), "the second builder found the finished base")
	g.Expect(listBases(t, stateRoot)).To(HaveLen(1))
}

func TestBuild_ContextCancelled(t *testing.T) {
	g := NewWithT(t)
	b, stateRoot, logPath := fakeBuilder(t, twoLayers(t))
	t.Setenv(fakeMkfsSleepEnv, "30s")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan error, 1)

	go func() {
		_, err := b.Build(ctx, overlay.LayerInput{ImageRef: "img"})
		done <- err
	}()

	// Cancel once the merge has started.
	g.Eventually(func() int { return len(readLog(t, logPath)) }, 30*time.Second).Should(Equal(3))
	cancel()

	var err error
	g.Eventually(done, 10*time.Second).Should(Receive(&err))
	g.Expect(errors.Is(err, context.Canceled)).To(BeTrue(), "got %v", err)
	g.Expect(listBases(t, stateRoot)).To(BeEmpty())

	entries, err := os.ReadDir(filepath.Join(stateRoot, "scratch"))
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(entries).To(BeEmpty())
}
