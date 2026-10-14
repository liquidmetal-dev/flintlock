package overlayinit

import (
	"bytes"
	"errors"
	"strings"
	"testing"
	"time"

	. "github.com/onsi/gomega"
)

const testCmdline = "console=ttyS0 flintlock.root.contract=1 flintlock.root.base=/dev/vda flintlock.root.rw=/dev/vdb panic=1"

func consoleLines(console *bytes.Buffer) []string {
	return strings.Split(strings.TrimSuffix(console.String(), "\n"), "\n")
}

func TestRun_MountOrder(t *testing.T) {
	g := NewWithT(t)

	sys := newFakeSystem(testCmdline)
	console := &bytes.Buffer{}

	err := Run(sys, console, []string{"single"})
	g.Expect(err).NotTo(HaveOccurred())

	g.Expect(sys.calls).To(Equal([]string{
		"mkdir /dev 755",
		`mount devtmpfs /dev devtmpfs 0x2 "mode=0755"`,
		"mkdir /proc 755",
		`mount proc /proc proc 0xe ""`,
		"readfile /proc/cmdline",
		"stat /dev/vda",
		"stat /dev/vdb",
		"mkdir /base 755",
		`mount /dev/vda /base erofs 0x1 ""`,
		"mkdir /rw 755",
		`mount /dev/vdb /rw ext4 0x0 ""`,
		"mkdir /rw/upper 755",
		"mkdir /rw/work 755",
		"mkdir /newroot 755",
		`mount overlay /newroot overlay 0x0 "lowerdir=/base,upperdir=/rw/upper,workdir=/rw/work"`,
		"mkdir /newroot/dev 755",
		`mount /dev /newroot/dev  0x2000 ""`,
		"mkdir /newroot/proc 755",
		`mount /proc /newroot/proc  0x2000 ""`,
		"chdir /newroot",
		"removecontents /",
		`mount . /  0x2000 ""`,
		"chroot .",
		"chdir /",
		"stat /sbin/init",
		"exec /sbin/init [/sbin/init single] [HOME=/ TERM=linux]",
	}))

	g.Expect(consoleLines(console)).To(Equal([]string{
		"flintlock-overlay-init: contract=1 base=/dev/vda base_fs=erofs rw=/dev/vdb rw_fs=ext4 init=/sbin/init",
	}))
}

func TestRun_ExecsTheInitOfTheImage(t *testing.T) {
	g := NewWithT(t)

	sys := newFakeSystem(testCmdline + " init=/lib/systemd/systemd")
	console := &bytes.Buffer{}

	g.Expect(Run(sys, console, nil)).To(Succeed())
	g.Expect(sys.calls[len(sys.calls)-1]).To(Equal("exec /lib/systemd/systemd [/lib/systemd/systemd] [HOME=/ TERM=linux]"))
	g.Expect(console.String()).To(HaveSuffix(" init=/lib/systemd/systemd\n"))
}

func TestRun_RefusesUnknownContract(t *testing.T) {
	g := NewWithT(t)

	sys := newFakeSystem("flintlock.root.contract=2 flintlock.root.base=/dev/vda flintlock.root.rw=/dev/vdb")
	console := &bytes.Buffer{}

	err := Run(sys, console, nil)
	g.Expect(err).To(MatchError(ErrUnknownContract))

	// Nothing is mounted beyond /dev and /proc, and no device is waited for.
	g.Expect(sys.calls[len(sys.calls)-1]).To(Equal("readfile /proc/cmdline"))
	g.Expect(sys.calls).NotTo(ContainElement(HavePrefix("stat ")))
	g.Expect(sys.calls).NotTo(ContainElement(HavePrefix("exec ")))

	lines := consoleLines(console)
	g.Expect(lines).To(HaveLen(1))
	g.Expect(lines[0]).To(HavePrefix("flintlock-overlay-init: error: unknown contract version"))
	g.Expect(lines[0]).To(ContainSubstring(`flintlock.root.contract="2"`))
}

func TestRun_DeviceTimeout(t *testing.T) {
	g := NewWithT(t)

	sys := newFakeSystem(testCmdline)
	delete(sys.blockDevices, "/dev/vdb")
	console := &bytes.Buffer{}

	start := sys.Now()

	err := Run(sys, console, nil)
	g.Expect(err).To(MatchError(ErrDeviceTimeout))
	g.Expect(err.Error()).To(ContainSubstring("/dev/vdb"))
	g.Expect(err.Error()).NotTo(ContainSubstring("/dev/vda"))

	// The init waited the whole timeout, in DevicePollInterval steps, and gave up.
	g.Expect(sys.Now().Sub(start)).To(Equal(DeviceTimeout))
	g.Expect(sys.calls).To(ContainElement("sleep " + DevicePollInterval.String()))
	g.Expect(sys.calls).NotTo(ContainElement(HavePrefix("mount /dev/vda")))
	g.Expect(sys.calls).NotTo(ContainElement(HavePrefix("exec ")))

	lines := consoleLines(console)
	g.Expect(lines).To(HaveLen(1))
	g.Expect(lines[0]).To(Equal("flintlock-overlay-init: error: timed out waiting for block device: /dev/vdb not present after 10s"))
}

func TestRun_WaitsForLateDevice(t *testing.T) {
	g := NewWithT(t)

	sys := newFakeSystem(testCmdline)
	sys.blockDevices["/dev/vdb"] = sys.Now().Add(3 * time.Second)
	console := &bytes.Buffer{}

	start := sys.Now()

	g.Expect(Run(sys, console, nil)).To(Succeed())
	g.Expect(sys.Now().Sub(start)).To(Equal(3 * time.Second))

	// Mounting starts after the last stat and only once both devices are present.
	g.Expect(sys.calls).To(ContainElement(`mount /dev/vdb /rw ext4 0x0 ""`))
	g.Expect(consoleLines(console)).To(HaveLen(1))
}

func TestRun_RejectsNonBlockDevice(t *testing.T) {
	g := NewWithT(t)

	sys := newFakeSystem(testCmdline)
	delete(sys.blockDevices, "/dev/vda")
	sys.charDevices["/dev/vda"] = true
	console := &bytes.Buffer{}

	err := Run(sys, console, nil)
	g.Expect(err).To(MatchError(ErrNotBlockDevice))
	g.Expect(sys.calls).NotTo(ContainElement(HavePrefix("sleep ")))
	g.Expect(consoleLines(console)).To(HaveLen(1))
}

func TestRun_FailureWritesOneLineAndStops(t *testing.T) {
	tests := []struct {
		name       string
		failCall   string
		wantReason string
		notAfter   string
		// afterContractLine is true when the failure happens after the contract line was written.
		afterContractLine bool
	}{
		{
			name:       "devtmpfs mount",
			failCall:   "mount devtmpfs",
			wantReason: "mounting devtmpfs on /dev as devtmpfs: ",
			notAfter:   "mkdir /proc",
		},
		{
			name:       "cmdline read",
			failCall:   "readfile /proc/cmdline",
			wantReason: "reading /proc/cmdline: ",
			notAfter:   "stat ",
		},
		{
			name:       "base mount",
			failCall:   "mount /dev/vda",
			wantReason: "mounting /dev/vda on /base as erofs: ",
			notAfter:   "mkdir /rw",
		},
		{
			name:       "rw mount",
			failCall:   "mount /dev/vdb",
			wantReason: "mounting /dev/vdb on /rw as ext4: ",
			notAfter:   "mkdir /rw/upper",
		},
		{
			name:       "upper dir",
			failCall:   "mkdir /rw/upper",
			wantReason: "creating /rw/upper: ",
			notAfter:   "mkdir /rw/work",
		},
		{
			name:       "overlay mount",
			failCall:   "mount overlay",
			wantReason: "mounting overlay on /newroot as overlay: ",
			notAfter:   "mkdir /newroot/dev",
		},
		{
			name:       "empty initramfs",
			failCall:   "removecontents /",
			wantReason: "emptying the initramfs: ",
			notAfter:   "mount . /",
		},
		{
			name:       "exec",
			failCall:   "exec /sbin/init",
			wantReason: "exec /sbin/init: ",
			notAfter:   "",

			afterContractLine: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)

			cause := errors.New("boom")
			sys := newFakeSystem(testCmdline)
			sys.fail[tc.failCall] = cause
			console := &bytes.Buffer{}

			err := Run(sys, console, nil)
			g.Expect(err).To(MatchError(cause))

			g.Expect(sys.calls[len(sys.calls)-1]).To(HavePrefix(tc.failCall))

			if tc.notAfter != "" {
				g.Expect(sys.calls).NotTo(ContainElement(HavePrefix(tc.notAfter)))
			}

			// The failure adds exactly one line to the console.
			wantLines := 1
			if tc.afterContractLine {
				wantLines = 2
			}

			lines := consoleLines(console)
			g.Expect(lines).To(HaveLen(wantLines))
			g.Expect(lines[len(lines)-1]).To(Equal("flintlock-overlay-init: error: " + tc.wantReason + "boom"))
		})
	}
}

func TestRun_RefusesMissingInit(t *testing.T) {
	g := NewWithT(t)

	sys := newFakeSystem(testCmdline)
	delete(sys.regularFiles, "/sbin/init")
	console := &bytes.Buffer{}

	err := Run(sys, console, nil)
	g.Expect(err).To(MatchError(ErrInitNotExecutable))
	g.Expect(sys.calls[len(sys.calls)-1]).To(Equal("stat /sbin/init"))
	g.Expect(sys.calls).NotTo(ContainElement(HavePrefix("exec ")))

	lines := consoleLines(console)
	g.Expect(lines).To(HaveLen(1))
	g.Expect(lines[0]).To(HavePrefix("flintlock-overlay-init: error: image init is not an executable file: /sbin/init: "))
	g.Expect(console.String()).NotTo(ContainSubstring("contract="))
}

func TestRun_RefusesNonExecutableInit(t *testing.T) {
	g := NewWithT(t)

	sys := newFakeSystem(testCmdline)
	sys.regularFiles["/sbin/init"] = 0o644
	console := &bytes.Buffer{}

	err := Run(sys, console, nil)
	g.Expect(err).To(MatchError(ErrInitNotExecutable))
	g.Expect(sys.calls).NotTo(ContainElement(HavePrefix("exec ")))

	g.Expect(consoleLines(console)).To(Equal([]string{
		"flintlock-overlay-init: error: image init is not an executable file: /sbin/init has no executable bit",
	}))
}

func TestRun_RefusesInitThatIsNotARegularFile(t *testing.T) {
	g := NewWithT(t)

	sys := newFakeSystem(testCmdline)
	delete(sys.regularFiles, "/sbin/init")
	sys.charDevices["/sbin/init"] = true
	console := &bytes.Buffer{}

	err := Run(sys, console, nil)
	g.Expect(err).To(MatchError(ErrInitNotExecutable))
	g.Expect(sys.calls).NotTo(ContainElement(HavePrefix("exec ")))
	g.Expect(consoleLines(console)).To(HaveLen(1))
}
