package provision_test

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	. "github.com/onsi/gomega"

	"github.com/liquidmetal-dev/flintlock/internal/provision"
	"github.com/liquidmetal-dev/flintlock/pkg/reflink"
)

// fakeExec records every command it is asked to run. Output returns the
// scripted value for the first prefix in outputs that matches the command;
// a command with no scripted output fails, which is how blkid and findmnt
// report "nothing found".
type fakeExec struct {
	calls   []string
	outputs map[string]string
	fail    map[string]error
}

func (f *fakeExec) Run(name string, args ...string) error {
	cmd := strings.TrimSpace(name + " " + strings.Join(args, " "))
	f.calls = append(f.calls, cmd)

	for prefix, err := range f.fail {
		if strings.HasPrefix(cmd, prefix) {
			return err
		}
	}

	return nil
}

func (f *fakeExec) Output(name string, args ...string) (string, error) {
	cmd := strings.TrimSpace(name + " " + strings.Join(args, " "))
	f.calls = append(f.calls, cmd)

	for prefix, out := range f.outputs {
		if strings.HasPrefix(cmd, prefix) {
			return out, nil
		}
	}

	return "", errors.New("exit status 2")
}

type stateDirFixture struct {
	exec  *fakeExec
	out   *bytes.Buffer
	fstab string
	dir   string
	sd    *provision.StateDir
}

func newStateDirFixture(t *testing.T, fsType string, probeErr error) *stateDirFixture {
	t.Helper()

	root := t.TempDir()
	fx := &stateDirFixture{
		exec:  &fakeExec{outputs: map[string]string{}, fail: map[string]error{}},
		out:   &bytes.Buffer{},
		fstab: filepath.Join(root, "fstab"),
		dir:   filepath.Join(root, "state"),
	}

	fx.sd = provision.NewStateDir(fx.exec, fx.out)
	fx.sd.FstabPath = fx.fstab
	fx.sd.FSType = func(string) (string, error) { return fsType, nil }
	fx.sd.Probe = func(string) error { return probeErr }

	return fx
}

func (fx *stateDirFixture) fstabContent(t *testing.T) string {
	t.Helper()

	content, err := os.ReadFile(fx.fstab)
	if err != nil {
		return ""
	}

	return string(content)
}

func TestStateDirVerify(t *testing.T) {
	tests := []struct {
		name      string
		fsType    string
		probeErr  error
		wantErr   bool
		wantInErr string
	}{
		{name: "xfs with reflink", fsType: reflink.FilesystemXFS},
		{name: "btrfs", fsType: reflink.FilesystemBtrfs},
		{
			name: "ext4", fsType: reflink.FilesystemExt4,
			wantErr: true, wantInErr: "is on ext4; give --device or --loop-file",
		},
		{
			name: "xfs without reflink", fsType: reflink.FilesystemXFS,
			probeErr: reflink.ErrNotSupported,
			wantErr:  true, wantInErr: "XFS without reflink",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			fx := newStateDirFixture(t, tt.fsType, tt.probeErr)

			err := fx.sd.Setup(provision.StateDirOptions{Dir: fx.dir})

			if tt.wantErr {
				g.Expect(err).To(MatchError(provision.ErrNoReflink))
				g.Expect(err.Error()).To(ContainSubstring(tt.wantInErr))
			} else {
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(fx.out.String()).To(ContainSubstring("with reflink support"))
			}

			// Verification never runs a host command or touches fstab.
			g.Expect(fx.exec.calls).To(BeEmpty())
			g.Expect(fx.fstabContent(t)).To(BeEmpty())
			g.Expect(fx.dir).To(BeADirectory())
		})
	}
}

func TestStateDirDevice(t *testing.T) {
	g := NewWithT(t)
	fx := newStateDirFixture(t, reflink.FilesystemXFS, nil)
	fx.exec.outputs["blkid -p -s UUID -o value /dev/sdb"] = "0f1e2d3c-0000-1111-2222-333344445555"

	err := fx.sd.Setup(provision.StateDirOptions{Dir: fx.dir, Device: "/dev/sdb"})
	g.Expect(err).NotTo(HaveOccurred())

	g.Expect(fx.exec.calls).To(Equal([]string{
		"findmnt -rno SOURCE --mountpoint " + fx.dir,
		"blkid -p -s TYPE -s PTTYPE -o value /dev/sdb",
		"mkfs.xfs -m reflink=1 -q /dev/sdb",
		"blkid -p -s UUID -o value /dev/sdb",
		"systemctl daemon-reload",
		"mount " + fx.dir,
	}))

	g.Expect(fx.fstabContent(t)).To(Equal(
		"UUID=0f1e2d3c-0000-1111-2222-333344445555 " + fx.dir + " xfs defaults,nofail 0 0\n",
	))
}

func TestStateDirDeviceAlreadyFormatted(t *testing.T) {
	g := NewWithT(t)
	fx := newStateDirFixture(t, reflink.FilesystemXFS, nil)
	fx.exec.outputs["blkid -p -s TYPE -s PTTYPE -o value /dev/sdb"] = "ext4"

	err := fx.sd.Setup(provision.StateDirOptions{Dir: fx.dir, Device: "/dev/sdb"})
	g.Expect(err).To(MatchError(ContainSubstring("already holds a filesystem or partition table (ext4)")))
	g.Expect(fx.exec.calls).NotTo(ContainElement(HavePrefix("mkfs.xfs")))
	g.Expect(fx.fstabContent(t)).To(BeEmpty())
}

func TestStateDirDeviceForce(t *testing.T) {
	g := NewWithT(t)
	fx := newStateDirFixture(t, reflink.FilesystemXFS, nil)
	fx.exec.outputs["blkid -p -s TYPE -s PTTYPE -o value /dev/sdb"] = "ext4"
	fx.exec.outputs["blkid -p -s UUID -o value /dev/sdb"] = "uuid-1"

	err := fx.sd.Setup(provision.StateDirOptions{Dir: fx.dir, Device: "/dev/sdb", Force: true})
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(fx.exec.calls).To(ContainElement("mkfs.xfs -m reflink=1 -q -f /dev/sdb"))
	g.Expect(fx.out.String()).To(ContainSubstring("WARNING: formatting /dev/sdb"))
}

func TestStateDirAlreadyMounted(t *testing.T) {
	g := NewWithT(t)
	fx := newStateDirFixture(t, reflink.FilesystemXFS, nil)
	fx.exec.outputs["findmnt"] = "/dev/sda1"

	err := fx.sd.Setup(provision.StateDirOptions{Dir: fx.dir, Device: "/dev/sdb"})
	g.Expect(err).To(MatchError(ContainSubstring("already a mount point for /dev/sda1")))
	g.Expect(fx.exec.calls).To(HaveLen(1))
}

func TestStateDirLoopFile(t *testing.T) {
	g := NewWithT(t)
	fx := newStateDirFixture(t, reflink.FilesystemXFS, nil)
	loopFile := filepath.Join(t.TempDir(), "state.img")
	fx.exec.outputs["losetup --find --show "+loopFile] = "/dev/loop7"

	err := fx.sd.Setup(provision.StateDirOptions{Dir: fx.dir, LoopFile: loopFile, SizeGB: 2})
	g.Expect(err).NotTo(HaveOccurred())

	g.Expect(fx.exec.calls).To(Equal([]string{
		"findmnt -rno SOURCE --mountpoint " + fx.dir,
		"losetup --output NAME --noheadings --associated " + loopFile,
		"losetup --find --show " + loopFile,
		"blkid -p -s TYPE -s PTTYPE -o value /dev/loop7",
		"mkfs.xfs -m reflink=1 -q /dev/loop7",
		"losetup --detach /dev/loop7",
		"systemctl daemon-reload",
		"mount " + fx.dir,
	}))

	info, err := os.Stat(loopFile)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(info.Size()).To(Equal(int64(2 << 30)))

	g.Expect(fx.fstabContent(t)).To(Equal(loopFile + " " + fx.dir + " xfs loop,nofail 0 0\n"))
}

func TestStateDirLoopFileAlreadyFormattedDetaches(t *testing.T) {
	g := NewWithT(t)
	fx := newStateDirFixture(t, reflink.FilesystemXFS, nil)
	loopFile := filepath.Join(t.TempDir(), "state.img")
	fx.exec.outputs["losetup --find --show "+loopFile] = "/dev/loop7"
	fx.exec.outputs["blkid -p -s TYPE -s PTTYPE -o value /dev/loop7"] = "xfs"

	err := fx.sd.Setup(provision.StateDirOptions{Dir: fx.dir, LoopFile: loopFile})
	g.Expect(err).To(MatchError(ContainSubstring("/dev/loop7 already holds a filesystem or partition table (xfs)")))

	g.Expect(fx.exec.calls).To(Equal([]string{
		"findmnt -rno SOURCE --mountpoint " + fx.dir,
		"losetup --output NAME --noheadings --associated " + loopFile,
		"losetup --find --show " + loopFile,
		"blkid -p -s TYPE -s PTTYPE -o value /dev/loop7",
		"losetup --detach /dev/loop7",
	}))
	g.Expect(fx.fstabContent(t)).To(BeEmpty())
}

func TestStateDirLoopFileDetachFails(t *testing.T) {
	g := NewWithT(t)
	fx := newStateDirFixture(t, reflink.FilesystemXFS, nil)
	loopFile := filepath.Join(t.TempDir(), "state.img")
	fx.exec.outputs["losetup --find --show "+loopFile] = "/dev/loop7"
	fx.exec.fail["mkfs.xfs"] = errors.New("exit status 1")
	fx.exec.fail["losetup --detach"] = errors.New("exit status 1")

	err := fx.sd.Setup(provision.StateDirOptions{Dir: fx.dir, LoopFile: loopFile})
	g.Expect(err).To(MatchError(ContainSubstring("formatting /dev/loop7 as xfs")))
	g.Expect(err).To(MatchError(ContainSubstring("detaching /dev/loop7 from " + loopFile)))
	g.Expect(fx.exec.calls).To(ContainElement("losetup --detach /dev/loop7"))
	g.Expect(fx.exec.calls).NotTo(ContainElement(HavePrefix("mount")))
}

func TestStateDirLoopFileDefaultSize(t *testing.T) {
	g := NewWithT(t)
	fx := newStateDirFixture(t, reflink.FilesystemXFS, nil)
	loopFile := filepath.Join(t.TempDir(), "state.img")
	fx.exec.outputs["losetup --find --show "+loopFile] = "/dev/loop0"

	g.Expect(fx.sd.Setup(provision.StateDirOptions{Dir: fx.dir, LoopFile: loopFile})).To(Succeed())

	info, err := os.Stat(loopFile)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(info.Size()).To(Equal(int64(provision.DefaultStateDirLoopSizeGB) << 30))
}

func TestStateDirMkfsFails(t *testing.T) {
	g := NewWithT(t)
	fx := newStateDirFixture(t, reflink.FilesystemXFS, nil)
	fx.exec.fail["mkfs.xfs"] = errors.New("exit status 1")

	err := fx.sd.Setup(provision.StateDirOptions{Dir: fx.dir, Device: "/dev/sdb"})
	g.Expect(err).To(MatchError(ContainSubstring("formatting /dev/sdb as xfs")))
	g.Expect(fx.exec.calls).NotTo(ContainElement(HavePrefix("mount")))
	g.Expect(fx.fstabContent(t)).To(BeEmpty())
}

func TestStateDirOptionsValidation(t *testing.T) {
	g := NewWithT(t)
	fx := newStateDirFixture(t, reflink.FilesystemXFS, nil)

	err := fx.sd.Setup(provision.StateDirOptions{Dir: fx.dir, Device: "/dev/sdb", LoopFile: "/x.img"})
	g.Expect(err).To(MatchError(ContainSubstring("not both")))

	err = fx.sd.Setup(provision.StateDirOptions{})
	g.Expect(err).To(MatchError(ContainSubstring("must not be empty")))

	g.Expect(fx.exec.calls).To(BeEmpty())
}

func TestBuildFstabLine(t *testing.T) {
	g := NewWithT(t)

	got := provision.BuildFstabLine("UUID=abc", "/var/lib/flintlock", "xfs", "defaults,nofail")
	g.Expect(got).To(Equal("UUID=abc /var/lib/flintlock xfs defaults,nofail 0 0"))
}

func TestReplaceFstabEntry(t *testing.T) {
	const line = "UUID=new /var/lib/flintlock xfs defaults,nofail 0 0"

	tests := []struct {
		name  string
		fstab string
		want  string
	}{
		{
			name:  "empty file",
			fstab: "",
			want:  line + "\n",
		},
		{
			name:  "append after other entries",
			fstab: "# /etc/fstab\nUUID=root / ext4 defaults 0 1\n",
			want:  "# /etc/fstab\nUUID=root / ext4 defaults 0 1\n" + line + "\n",
		},
		{
			name: "replace entries for the same mount point",
			fstab: "UUID=root / ext4 defaults 0 1\n" +
				"/dev/sdb /var/lib/flintlock xfs defaults 0 0\n" +
				"UUID=old /var/lib/flintlock xfs defaults 0 0\n" +
				"UUID=data /data xfs defaults 0 0\n",
			want: "UUID=root / ext4 defaults 0 1\n" + line + "\nUUID=data /data xfs defaults 0 0\n",
		},
		{
			name:  "comments mentioning the mount point are kept",
			fstab: "# /var/lib/flintlock is the state dir\n",
			want:  "# /var/lib/flintlock is the state dir\n" + line + "\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			got := provision.ReplaceFstabEntry(tt.fstab, "/var/lib/flintlock", line)
			g.Expect(got).To(Equal(tt.want))
		})
	}
}
