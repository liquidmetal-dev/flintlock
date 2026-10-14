package overlay

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	. "github.com/onsi/gomega"
	"github.com/opencontainers/go-digest"
)

const (
	testChain = digest.Digest("sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	testSum   = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
)

func testIndex(t *testing.T) *fsIndex {
	t.Helper()

	idx, err := newFSIndex(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	return idx
}

func TestFSIndex_Layout(t *testing.T) {
	g := NewWithT(t)
	idx := testIndex(t)

	for _, dir := range []string{"bases", "bases/by-source", "bases/.locks", "scratch"} {
		info, err := os.Stat(filepath.Join(idx.root, dir))
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(info.IsDir()).To(BeTrue())
	}
}

func TestFSIndex_RecordAndLookup(t *testing.T) {
	g := NewWithT(t)
	idx := testIndex(t)
	key := baseKey{ChainID: testChain, Profile: "erofs-v1"}

	_, err := idx.Lookup(key)
	g.Expect(errors.Is(err, errBaseNotInIndex)).To(BeTrue(), "got %v", err)

	path := filepath.Join(idx.root, "bases", testSum+".img")
	g.Expect(os.WriteFile(path, []byte("base"), 0o400)).To(Succeed())
	g.Expect(idx.Record(key, path)).To(Succeed())

	target, err := os.Readlink(filepath.Join(idx.root, "bases", "by-source", testChain.String(), "erofs-v1"))
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(target).To(Equal("../../" + testSum + ".img"))

	base, err := idx.Lookup(key)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(base).To(Equal(&Base{Digest: testSum, Path: path, ChainID: testChain, Profile: "erofs-v1"}))

	// Record replaces an old link.
	other := filepath.Join(idx.root, "bases", "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc.img")
	g.Expect(os.WriteFile(other, []byte("base2"), 0o400)).To(Succeed())
	g.Expect(idx.Record(key, other)).To(Succeed())

	base, err = idx.Lookup(key)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(base.Path).To(Equal(other))

	// A link to a missing file is not a base.
	g.Expect(os.Remove(other)).To(Succeed())
	_, err = idx.Lookup(key)
	g.Expect(errors.Is(err, errBaseNotInIndex)).To(BeTrue(), "got %v", err)

	// A link to a file that is not named by a digest is an error.
	bad := filepath.Join(idx.root, "bases", "notadigest.img")
	g.Expect(os.WriteFile(bad, []byte("x"), 0o400)).To(Succeed())
	g.Expect(idx.Record(key, bad)).To(Succeed())
	_, err = idx.Lookup(key)
	g.Expect(err).To(MatchError(ContainSubstring("is not a base file name")))
}

func TestFSIndex_Touch(t *testing.T) {
	g := NewWithT(t)
	idx := testIndex(t)
	key := baseKey{ChainID: testChain, Profile: "erofs-v1"}

	path := filepath.Join(idx.root, "bases", testSum+".img")
	g.Expect(os.WriteFile(path, []byte("base"), 0o400)).To(Succeed())
	g.Expect(idx.Record(key, path)).To(Succeed())

	link := filepath.Join(idx.root, "bases", "by-source", testChain.String(), "erofs-v1")
	old := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	g.Expect(os.Chtimes(link, old, old)).To(Succeed())

	g.Expect(idx.Touch(key)).To(Succeed())

	info, err := os.Lstat(link)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(info.ModTime()).To(BeTemporally("~", time.Now(), time.Minute))

	fileInfo, err := os.Stat(path)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(fileInfo.ModTime()).To(BeTemporally("<", info.ModTime()), "the base file is not touched")
}

func TestFSIndex_Lock(t *testing.T) {
	g := NewWithT(t)
	idx := testIndex(t)
	key := baseKey{ChainID: testChain, Profile: "erofs-v1"}
	other := baseKey{ChainID: testChain, Profile: "erofs-v2"}
	ctx := context.Background()

	release, err := idx.Lock(ctx, key)
	g.Expect(err).NotTo(HaveOccurred())

	_, err = os.Stat(filepath.Join(idx.root, "bases", ".locks", "sha256-"+testChain.Encoded()+"-erofs-v1.lock"))
	g.Expect(err).NotTo(HaveOccurred())

	// Another key is independent.
	releaseOther, err := idx.Lock(ctx, other)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(releaseOther()).To(Succeed())

	// The same key waits until the context ends.
	short, cancel := context.WithTimeout(ctx, 150*time.Millisecond)
	defer cancel()

	_, err = idx.Lock(short, key)
	g.Expect(errors.Is(err, context.DeadlineExceeded)).To(BeTrue(), "got %v", err)

	// After the release the key is free.
	g.Expect(release()).To(Succeed())

	release, err = idx.Lock(ctx, key)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(release()).To(Succeed())
}

func TestFSIndex_References(t *testing.T) {
	g := NewWithT(t)
	idx := testIndex(t)

	refs, err := idx.References()
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(refs).To(BeEmpty())

	vol1 := filepath.Join(idx.root, "vm", "ns", "vm1", "uid1", "volumes")
	vol2 := filepath.Join(idx.root, "vm", "ns", "vm2", "uid2", "volumes")
	g.Expect(os.MkdirAll(vol1, 0o755)).To(Succeed())
	g.Expect(os.MkdirAll(vol2, 0o755)).To(Succeed())
	g.Expect(os.Symlink("../../../../../bases/"+testSum+".img", filepath.Join(vol1, "root.base"))).To(Succeed())
	g.Expect(os.Symlink("../../../../../bases/"+testSum+".img", filepath.Join(vol2, "root.base"))).To(Succeed())
	g.Expect(os.WriteFile(filepath.Join(vol2, "root.rw"), nil, 0o644)).To(Succeed())

	refs, err = idx.References()
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(refs).To(ConsistOf(
		Reference{Digest: testSum, Link: filepath.Join(vol1, "root.base")},
		Reference{Digest: testSum, Link: filepath.Join(vol2, "root.base")},
	))

	g.Expect(os.Remove(filepath.Join(vol1, "root.base"))).To(Succeed())
	g.Expect(os.Symlink("../../../../../bases/junk", filepath.Join(vol1, "root.base"))).To(Succeed())

	_, err = idx.References()
	g.Expect(err).To(MatchError(ContainSubstring("is not a base file name")))
}

func TestBaseKey_String(t *testing.T) {
	g := NewWithT(t)
	key := baseKey{ChainID: testChain, Profile: "erofs-v1"}

	g.Expect(key.String()).To(Equal(testChain.String() + "/erofs-v1"))
	g.Expect(key.lockName()).To(Equal("sha256-" + testChain.Encoded() + "-erofs-v1.lock"))
}

func TestFSIndex_EntriesLastUseRemove(t *testing.T) {
	g := NewWithT(t)
	idx := testIndex(t)
	key := baseKey{ChainID: testChain, Profile: "erofs-v1"}

	entries, err := idx.Entries()
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(entries).To(BeEmpty())

	_, err = idx.LastUse(key)
	g.Expect(errors.Is(err, errBaseNotInIndex)).To(BeTrue(), "got %v", err)

	path := filepath.Join(idx.root, "bases", testSum+".img")
	g.Expect(os.WriteFile(path, []byte("base"), 0o400)).To(Succeed())
	g.Expect(idx.Record(key, path)).To(Succeed())

	entries, err = idx.Entries()
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(entries).To(HaveLen(1))
	g.Expect(entries[0].Key).To(Equal(key))
	g.Expect(entries[0].Digest).To(Equal(testSum))
	g.Expect(entries[0].Path).To(Equal(path))
	g.Expect(entries[0].LastUse).To(BeTemporally("~", time.Now(), time.Minute))

	lastUse, err := idx.LastUse(key)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(lastUse).To(Equal(entries[0].LastUse))

	// A dangling link is still listed, so the sweep can remove it.
	g.Expect(os.Chmod(path, 0o600)).To(Succeed())
	g.Expect(os.Remove(path)).To(Succeed())

	entries, err = idx.Entries()
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(entries).To(HaveLen(1))

	g.Expect(idx.Remove(key)).To(Succeed())
	g.Expect(idx.Remove(key)).To(Succeed(), "a missing link is a success")

	entries, err = idx.Entries()
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(entries).To(BeEmpty())
}
