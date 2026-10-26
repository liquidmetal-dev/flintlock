package overlayinit

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"strings"
	"time"
)

const (
	// DeviceTimeout is the longest the init waits for the two block devices.
	DeviceTimeout = 10 * time.Second
	// DevicePollInterval is the pause between two checks for the block devices.
	DevicePollInterval = 100 * time.Millisecond

	// ConsolePrefix starts every line the init writes to the console.
	ConsolePrefix = "flintlock-overlay-init: "

	cmdlinePath = "/proc/cmdline"
	devDir      = "/dev"
	procDir     = "/proc"
	baseDir     = "/base"
	rwDir       = "/rw"
	newRootDir  = "/newroot"
	upperDir    = rwDir + "/upper"
	workDir     = rwDir + "/work"

	dirPerm os.FileMode = 0o755
	// execBits are the executable permission bits of a file mode.
	execBits os.FileMode = 0o111
)

// Run assembles the overlay root and execs the init of the image. args are
// the arguments the kernel gave to /init; they are passed on to the init of
// the image. Run only returns on error, after it wrote one line with the
// reason to the console.
func Run(sys System, console io.Writer, args []string) error {
	err := run(sys, console, args)
	if err != nil {
		fmt.Fprintf(console, "%serror: %v\n", ConsolePrefix, err)
	}

	return err
}

func run(sys System, console io.Writer, args []string) error {
	if err := mountEarly(sys); err != nil {
		return err
	}

	cmdline, err := sys.ReadFile(cmdlinePath)
	if err != nil {
		return fmt.Errorf("reading %s: %w", cmdlinePath, err)
	}

	params, err := ParseCmdline(string(cmdline))
	if err != nil {
		return err
	}

	if err := waitForDevices(sys, params.Base, params.RW); err != nil {
		return err
	}

	if err := mountRoot(sys, params); err != nil {
		return err
	}

	if err := switchRoot(sys); err != nil {
		return err
	}

	return execInit(sys, console, params, args)
}

// mountEarly mounts devtmpfs on /dev and proc on /proc. The init needs /dev
// for the block devices and /proc for the kernel command line.
func mountEarly(sys System) error {
	if err := mountAt(sys, "devtmpfs", devDir, "devtmpfs", msNosuid, "mode=0755"); err != nil {
		return err
	}

	return mountAt(sys, "proc", procDir, "proc", msNosuid|msNodev|msNoexec, "")
}

// waitForDevices polls until every device is a block device, or until
// DeviceTimeout has passed since the first check.
func waitForDevices(sys System, devices ...string) error {
	deadline := sys.Now().Add(DeviceTimeout)

	for {
		missing, err := missingDevices(sys, devices)
		if err != nil {
			return err
		}

		if len(missing) == 0 {
			return nil
		}

		if !sys.Now().Before(deadline) {
			return fmt.Errorf("%w: %s not present after %s", ErrDeviceTimeout, strings.Join(missing, ", "), DeviceTimeout)
		}

		sys.Sleep(DevicePollInterval)
	}
}

// missingDevices returns the devices that do not exist yet. A path that
// exists but is not a block device is an error.
func missingDevices(sys System, devices []string) ([]string, error) {
	var missing []string

	for _, device := range devices {
		info, err := sys.Stat(device)
		if errors.Is(err, fs.ErrNotExist) {
			missing = append(missing, device)

			continue
		}

		if err != nil {
			return nil, fmt.Errorf("stat %s: %w", device, err)
		}

		if !isBlockDevice(info.Mode()) {
			return nil, fmt.Errorf("%w: %s", ErrNotBlockDevice, device)
		}
	}

	return missing, nil
}

func isBlockDevice(mode os.FileMode) bool {
	return mode&os.ModeDevice != 0 && mode&os.ModeCharDevice == 0
}

// mountRoot mounts the base, the writable disk and the overlay on top of
// them, in that order.
func mountRoot(sys System, params Params) error {
	if err := mountAt(sys, params.Base, baseDir, BaseFSType, msRdonly, ""); err != nil {
		return err
	}

	if err := mountAt(sys, params.RW, rwDir, RWFSType, 0, ""); err != nil {
		return err
	}

	for _, dir := range []string{upperDir, workDir} {
		if err := sys.MkdirAll(dir, dirPerm); err != nil {
			return fmt.Errorf("creating %s: %w", dir, err)
		}
	}

	options := fmt.Sprintf("lowerdir=%s,upperdir=%s,workdir=%s", baseDir, upperDir, workDir)

	return mountAt(sys, "overlay", newRootDir, "overlay", 0, options)
}

// mountAt creates target and mounts source on it.
func mountAt(sys System, source, target, fstype string, flags uintptr, data string) error {
	if err := sys.MkdirAll(target, dirPerm); err != nil {
		return fmt.Errorf("creating %s: %w", target, err)
	}

	if err := sys.Mount(source, target, fstype, flags, data); err != nil {
		return fmt.Errorf("mounting %s on %s as %s: %w", source, target, fstype, err)
	}

	return nil
}

// switchRoot does what switch_root(8) does before the exec: it moves /dev
// and /proc into the new root, deletes the contents of the initramfs to give
// the memory back, moves the new root to / and changes the root of the
// process to it.
func switchRoot(sys System) error {
	for _, dir := range []string{devDir, procDir} {
		target := newRootDir + dir

		if err := sys.MkdirAll(target, dirPerm); err != nil {
			return fmt.Errorf("creating %s: %w", target, err)
		}

		if err := sys.Mount(dir, target, "", msMove, ""); err != nil {
			return fmt.Errorf("moving %s to %s: %w", dir, target, err)
		}
	}

	if err := sys.Chdir(newRootDir); err != nil {
		return fmt.Errorf("chdir %s: %w", newRootDir, err)
	}

	if err := sys.RemoveContents("/"); err != nil {
		return fmt.Errorf("emptying the initramfs: %w", err)
	}

	if err := sys.Mount(".", "/", "", msMove, ""); err != nil {
		return fmt.Errorf("moving %s to /: %w", newRootDir, err)
	}

	if err := sys.Chroot("."); err != nil {
		return fmt.Errorf("chroot: %w", err)
	}

	if err := sys.Chdir("/"); err != nil {
		return fmt.Errorf("chdir /: %w", err)
	}

	return nil
}

// execInit checks the init of the image, writes the contract line and execs
// the init. The root of the process is the overlay by now, so the path is
// resolved inside the image. The check keeps the console to one line in the
// usual failure, a missing or non-executable init. Only when exec itself
// fails after the check, which takes a fault the check cannot see such as a
// missing dynamic loader, does the console get the contract line and then
// the error line.
func execInit(sys System, console io.Writer, params Params, args []string) error {
	if err := checkInit(sys, params.Init); err != nil {
		return err
	}

	fmt.Fprintf(console, "%scontract=%s base=%s base_fs=%s rw=%s rw_fs=%s init=%s\n",
		ConsolePrefix, params.Contract, params.Base, BaseFSType, params.RW, RWFSType, params.Init)

	argv := append([]string{params.Init}, args...)

	if err := sys.Exec(params.Init, argv, sys.Environ()); err != nil {
		return fmt.Errorf("exec %s: %w", params.Init, err)
	}

	return nil
}

// checkInit requires that initPath is a regular file with an executable
// bit. Symlinks are followed.
func checkInit(sys System, initPath string) error {
	info, err := sys.Stat(initPath)
	if err != nil {
		return fmt.Errorf("%w: %s: %w", ErrInitNotExecutable, initPath, err)
	}

	if !info.Mode().IsRegular() {
		return fmt.Errorf("%w: %s is not a regular file", ErrInitNotExecutable, initPath)
	}

	if info.Mode().Perm()&execBits == 0 {
		return fmt.Errorf("%w: %s has no executable bit", ErrInitNotExecutable, initPath)
	}

	return nil
}
