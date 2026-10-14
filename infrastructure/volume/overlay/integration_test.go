package overlay_test

import (
	"archive/tar"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"testing"

	"github.com/containerd/containerd/v2/pkg/archive"
	. "github.com/onsi/gomega"
	"golang.org/x/sys/unix"

	"github.com/liquidmetal-dev/flintlock/infrastructure/volume/overlay"
	"github.com/liquidmetal-dev/flintlock/pkg/defaults"
)

// The integration tests run the pinned mkfs.erofs. They find it at
// FLINTLOCK_MKFS_EROFS or at the path flintlock-provision installs it to.
// In CI (CI is set) a missing binary fails the tests. Elsewhere it skips
// them.
const mkfsEnv = "FLINTLOCK_MKFS_EROFS"

func inCI() bool {
	return os.Getenv("CI") != ""
}

// missingPrecondition fails the test in CI and skips it elsewhere.
func missingPrecondition(t *testing.T, format string, args ...any) {
	t.Helper()

	if inCI() {
		t.Fatalf("CI precondition missing: "+format, args...)
	}

	t.Skipf(format, args...)
}

func realMkfs(t *testing.T) string {
	t.Helper()

	path := os.Getenv(mkfsEnv)
	if path == "" {
		path = defaults.MkfsErofsPath
	}

	if _, err := os.Stat(path); err != nil {
		missingPrecondition(t, "mkfs.erofs is not at %s (set %s): %v", path, mkfsEnv, err)
	}

	return path
}

// capNetRaw is a version 2 security.capability value with cap_net_raw in
// the permitted and effective sets, as getcap reports "cap_net_raw=ep".
var capNetRaw = string([]byte{
	0x01, 0x00, 0x00, 0x02, // magic VFS_CAP_REVISION_2 | VFS_CAP_FLAGS_EFFECTIVE
	0x00, 0x20, 0x00, 0x00, // permitted, low word: 1 << 13
	0x00, 0x00, 0x00, 0x00, // inheritable, low word
	0x00, 0x00, 0x00, 0x00, // permitted, high word
	0x00, 0x00, 0x00, 0x00, // inheritable, high word
})

// fixtureLayers reproduce the cases of experiments X-1 and X-2: an
// override, a whited-out file and directory, an opaque directory, a
// directory whited out and then recreated with another mode, a hardlink
// pair within a layer, a setuid file, a non-root owner, a character device,
// a symlink and xattrs.
func fixtureLayers(t *testing.T) [][]byte {
	t.Helper()

	big := strings.Repeat("0123456789abcdef", 32*1024) // 512 KiB

	l1 := tarBytes(t,
		dir("etc", 0o755),
		file("etc/config", 0o644, "v1"),
		file("etc/removed", 0o644, "gone soon"),
		dir("usr", 0o755),
		dir("usr/bin", 0o755),
		file("usr/bin/tool", 0o4755, big),
		hardlink("usr/bin/tool2", "usr/bin/tool", 0o4755),
		entry{
			name: "usr/bin/ping", typeflag: tar.TypeReg, mode: 0o755, content: "ping",
			xattrs: map[string]string{"security.capability": capNetRaw, "user.flintlock": "test"},
		},
		dir("data", 0o755),
		symlink("data/link", "../etc/config"),
		dir("gone", 0o755),
		file("gone/file", 0o644, "x"),
		dir("opaque", 0o755),
		file("opaque/old", 0o644, "old"),
		dir("home", 0o755),
		entry{name: "home/user/", typeflag: tar.TypeDir, mode: 0o750, uid: 1000, gid: 1000},
		entry{name: "home/user/file", typeflag: tar.TypeReg, mode: 0o640, uid: 1000, gid: 1000, content: "mine"},
		dir("dev", 0o755),
		entry{name: "dev/null", typeflag: tar.TypeChar, mode: 0o666, major: 1, minor: 3},
	)

	l2 := tarBytes(t,
		dir("etc", 0o755),
		file("etc/config", 0o644, "v2"),
		file("etc/.wh.removed", 0, ""),
		file(".wh.gone", 0, ""),
		dir("opaque", 0o755),
		file("opaque/.wh..wh..opq", 0, ""),
		file("opaque/new", 0o644, "new"),
	)

	l3 := tarBytes(t,
		dir("gone", 0o700),
		file("gone/again", 0o644, "again"),
		dir("etc", 0o755),
		file("etc/pair-a", 0o644, "pair"),
		hardlink("etc/pair-b", "etc/pair-a", 0o644),
	)

	return [][]byte{l1, l2, l3}
}

func realBuilder(t *testing.T, mkfsPath string, layers ...[]byte) (*overlay.Builder, *memSource) {
	t.Helper()

	source := newMemSource(layers...)

	b, err := overlay.NewBuilder(t.TempDir(), mkfsPath, source)
	if err != nil {
		t.Fatal(err)
	}

	return b, source
}

// superblock holds the fields of an EROFS superblock the tests check.
type superblock struct {
	magic     uint32
	buildTime uint64
	uuid      [16]byte
}

func readSuperblock(t *testing.T, path string) superblock {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	const offset = 1024

	var sb superblock

	sb.magic = binary.LittleEndian.Uint32(data[offset:])
	sb.buildTime = binary.LittleEndian.Uint64(data[offset+24:])
	copy(sb.uuid[:], data[offset+48:offset+64])

	return sb
}

func TestIntegration_BuildIsDeterministic(t *testing.T) {
	g := NewWithT(t)
	mkfsPath := realMkfs(t)
	layers := fixtureLayers(t)
	ctx := context.Background()

	b1, source := realBuilder(t, mkfsPath, layers...)
	b2, _ := realBuilder(t, mkfsPath, layers...)

	first, err := b1.Build(ctx, overlay.LayerInput{ImageRef: "fixture"})
	g.Expect(err).NotTo(HaveOccurred())

	second, err := b2.Build(ctx, overlay.LayerInput{ImageRef: "fixture"})
	g.Expect(err).NotTo(HaveOccurred())

	g.Expect(second.Digest).To(Equal(first.Digest), "two builds of one image give one sha256")
	g.Expect(second.Path).NotTo(Equal(first.Path))

	content, err := os.ReadFile(first.Path)
	g.Expect(err).NotTo(HaveOccurred())
	sum := sha256.Sum256(content)
	g.Expect(first.Digest).To(Equal(hex.EncodeToString(sum[:])))

	sb := readSuperblock(t, first.Path)
	g.Expect(sb.magic).To(Equal(uint32(0xE0F5E1E2)), "EROFS magic")
	g.Expect(sb.buildTime).To(BeZero(), "-T0 --mkfs-time gives a zero build time")

	want := overlay.BaseUUID(source.chainID(), overlay.ProfileNameErofsV1)
	g.Expect(sb.uuid[:]).To(Equal(want[:]), "the UUID is derived from the key")

	info, err := os.Stat(first.Path)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(info.Mode().Perm()).To(Equal(os.FileMode(0o400)))
}

func TestIntegration_HardlinkIntoLowerLayerFails(t *testing.T) {
	g := NewWithT(t)
	mkfsPath := realMkfs(t)
	layers := fixtureLayers(t)

	l4 := tarBytes(t,
		dir("usr", 0o755),
		dir("usr/bin", 0o755),
		hardlink("usr/bin/tool3", "usr/bin/tool", 0o4755),
	)

	b, source := realBuilder(t, mkfsPath, append(layers, l4)...)

	_, err := b.Build(context.Background(), overlay.LayerInput{ImageRef: "fixture"})

	var herr *overlay.HardlinkError
	g.Expect(errors.As(err, &herr)).To(BeTrue(), "got %v", err)
	g.Expect(herr.LayerIndex).To(Equal(3))
	g.Expect(herr.DiffID).To(Equal(source.diffIDs()[3]))
	g.Expect(herr.Entry).To(Equal("usr/bin/tool3"))
	g.Expect(herr.Target).To(Equal("usr/bin/tool"))
}

// TestIntegration_BaseMatchesContainerdUnpack mounts the base through the
// kernel and compares it with the tree containerd's archive.Apply makes
// from the same layers. It needs root for the mount and for the device
// node and owners of the applied tree.
func TestIntegration_BaseMatchesContainerdUnpack(t *testing.T) {
	g := NewWithT(t)
	mkfsPath := realMkfs(t)

	if os.Geteuid() != 0 {
		missingPrecondition(t, "the kernel mount comparison needs root")
	}

	requireErofs(t)

	layers := fixtureLayers(t)
	b, _ := realBuilder(t, mkfsPath, layers...)

	base, err := b.Build(context.Background(), overlay.LayerInput{ImageRef: "fixture"})
	g.Expect(err).NotTo(HaveOccurred())

	mounted := mountErofs(t, base.Path)

	applied := t.TempDir()
	for i, layer := range layers {
		_, err := archive.Apply(context.Background(), applied, bytes.NewReader(layer))
		g.Expect(err).NotTo(HaveOccurred(), "applying layer %d", i)
	}

	// Direct checks of the merge, independent of the apply.
	g.Expect(os.ReadFile(filepath.Join(mounted, "etc/config"))).To(Equal([]byte("v2")))
	g.Expect(filepath.Join(mounted, "etc/removed")).NotTo(BeAnExistingFile())
	g.Expect(filepath.Join(mounted, "opaque/old")).NotTo(BeAnExistingFile())
	g.Expect(os.ReadFile(filepath.Join(mounted, "opaque/new"))).To(Equal([]byte("new")))
	g.Expect(filepath.Join(mounted, "gone/file")).NotTo(BeAnExistingFile())
	g.Expect(os.ReadFile(filepath.Join(mounted, "gone/again"))).To(Equal([]byte("again")))

	goneInfo, err := os.Stat(filepath.Join(mounted, "gone"))
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(goneInfo.Mode().Perm()).To(Equal(os.FileMode(0o700)))

	got := readTree(t, mounted)
	want := readTree(t, applied)

	for _, name := range []string{"etc/.wh.removed", ".wh.gone", "opaque/.wh..wh..opq"} {
		g.Expect(got).NotTo(HaveKey(name))
	}

	g.Expect(got["opaque"].xattrs).NotTo(HaveKey("trusted.overlay.opaque"))
	g.Expect(got["usr/bin/ping"].xattrs).To(HaveKeyWithValue("security.capability", capNetRaw))
	g.Expect(got["usr/bin/tool"].mode & os.ModeSetuid).NotTo(BeZero())
	g.Expect(got["usr/bin/tool"].group).To(Equal("usr/bin/tool,usr/bin/tool2"))
	g.Expect(got["etc/pair-a"].group).To(Equal("etc/pair-a,etc/pair-b"))

	g.Expect(diffTrees(got, want)).To(BeEmpty(), "the mounted base differs from containerd's unpack")
}

// requireErofs loads the erofs module when the kernel does not list the
// filesystem. A kernel without erofs skips the test, in CI too, with a
// message that names the kernel: the runner's kernel is outside this
// repository's control.
func requireErofs(t *testing.T) {
	t.Helper()

	if hasErofs(t) {
		return
	}

	out, err := exec.Command("modprobe", "erofs").CombinedOutput()
	if err != nil || !hasErofs(t) {
		var uts unix.Utsname
		_ = unix.Uname(&uts)

		t.Skipf("kernel %s has no erofs support (modprobe erofs: %v %s); the kernel mount comparison did not run",
			unix.ByteSliceToString(uts.Release[:]), err, strings.TrimSpace(string(out)))
	}
}

func hasErofs(t *testing.T) bool {
	t.Helper()

	data, err := os.ReadFile("/proc/filesystems")
	if err != nil {
		t.Fatal(err)
	}

	for _, line := range strings.Split(string(data), "\n") {
		if strings.TrimSpace(strings.TrimPrefix(line, "nodev")) == "erofs" {
			return true
		}
	}

	return false
}

// mountErofs mounts the image read-only through a loop device and
// unmounts it when the test ends.
func mountErofs(t *testing.T, image string) string {
	t.Helper()

	dir := t.TempDir()

	out, err := exec.Command("mount", "-t", "erofs", "-o", "loop,ro", image, dir).CombinedOutput()
	if err != nil {
		t.Fatalf("mount -t erofs %s: %v: %s", image, err, out)
	}

	t.Cleanup(func() {
		if out, err := exec.Command("umount", dir).CombinedOutput(); err != nil {
			t.Errorf("umount %s: %v: %s", dir, err, out)
		}
	})

	return dir
}

// node is what the comparison records about one path.
type node struct {
	mode   os.FileMode
	uid    uint32
	gid    uint32
	size   int64
	sum    string
	target string
	rdev   uint64
	xattrs map[string]string
	// group is the sorted paths that share the inode, joined by commas.
	group string
}

// readTree records every path below root, without the root itself.
func readTree(t *testing.T, root string) map[string]node {
	t.Helper()

	tree := map[string]node{}
	inodes := map[uint64][]string{}
	inodeOf := map[string]uint64{}

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		rel, err := filepath.Rel(root, path)
		if err != nil || rel == "." {
			return err
		}

		info, err := d.Info()
		if err != nil {
			return err
		}

		st, ok := info.Sys().(*syscall.Stat_t)
		if !ok {
			return fmt.Errorf("%s: no stat", path)
		}

		n := node{mode: info.Mode(), uid: st.Uid, gid: st.Gid, xattrs: listXattrs(t, path)}

		switch {
		case info.Mode().IsRegular():
			n.size = info.Size()
			n.sum = fileSum(t, path)
		case info.Mode()&os.ModeSymlink != 0:
			n.target, err = os.Readlink(path)
			if err != nil {
				return err
			}
		case info.Mode()&os.ModeDevice != 0:
			n.rdev = uint64(st.Rdev) //nolint:unconvert // Rdev is uint32 on some arches
		}

		if !info.IsDir() {
			inodes[st.Ino] = append(inodes[st.Ino], rel)
			inodeOf[rel] = st.Ino
		}

		tree[rel] = n

		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	for rel, ino := range inodeOf {
		paths := inodes[ino]
		sort.Strings(paths)

		n := tree[rel]
		n.group = strings.Join(paths, ",")
		tree[rel] = n
	}

	return tree
}

func fileSum(t *testing.T, path string) string {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	sum := sha256.Sum256(data)

	return hex.EncodeToString(sum[:])
}

func listXattrs(t *testing.T, path string) map[string]string {
	t.Helper()

	buf := make([]byte, 64*1024)

	n, err := unix.Llistxattr(path, buf)
	if err != nil {
		if errors.Is(err, unix.ENOTSUP) {
			return map[string]string{}
		}

		t.Fatalf("listing xattrs of %s: %v", path, err)
	}

	out := map[string]string{}

	for _, name := range bytes.Split(buf[:n], []byte{0}) {
		if len(name) == 0 {
			continue
		}

		value := make([]byte, 64*1024)

		vn, err := unix.Lgetxattr(path, string(name), value)
		if err != nil {
			t.Fatalf("reading xattr %s of %s: %v", name, path, err)
		}

		out[string(name)] = string(value[:vn])
	}

	return out
}

// diffTrees lists every path whose record differs between the trees, with
// both values, so that a failure names the path and the field.
func diffTrees(got, want map[string]node) []string {
	var diffs []string

	for path, w := range want {
		g, ok := got[path]
		if !ok {
			diffs = append(diffs, fmt.Sprintf("%s: missing from the base", path))

			continue
		}

		if fmt.Sprintf("%+v", g) != fmt.Sprintf("%+v", w) {
			diffs = append(diffs, fmt.Sprintf("%s:\n  base:   %+v\n  unpack: %+v", path, g, w))
		}
	}

	for path := range got {
		if _, ok := want[path]; !ok {
			diffs = append(diffs, fmt.Sprintf("%s: extra in the base", path))
		}
	}

	sort.Strings(diffs)

	return diffs
}
