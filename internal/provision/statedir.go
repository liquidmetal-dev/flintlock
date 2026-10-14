package provision

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/liquidmetal-dev/flintlock/pkg/reflink"
)

const (
	// DefaultFstabPath is the file mounts are persisted in.
	DefaultFstabPath = "/etc/fstab"
	// DefaultStateDirLoopSizeGB is the size of the sparse file behind a
	// loop-backed state directory when the caller gives none.
	DefaultStateDirLoopSizeGB = 20
	// StateDirFilesystem is the filesystem type the provisioner creates
	// for the state directory.
	StateDirFilesystem = reflink.FilesystemXFS

	stateDirMode = 0o755
	fstabMode    = 0o644
)

// StateDirOptions describes how to prepare the flintlockd state directory.
// With neither Device nor LoopFile set the directory is only verified.
type StateDirOptions struct {
	// Dir is the state directory. It becomes the mount point when a
	// filesystem is created.
	Dir string
	// Device is a block device or logical volume to format as XFS.
	Device string
	// LoopFile is a sparse file to format as XFS and mount through a loop
	// device. It is meant for development and e2e hosts.
	LoopFile string
	// SizeGB is the size of LoopFile when it is created.
	SizeGB int
	// Force formats a device or loop file that already holds a filesystem.
	Force bool
}

// StateDir prepares the flintlockd state directory so that files on it can
// be cloned with FICLONE. Host commands go through Exec and the filesystem
// checks through Probe and FSType, so tests can replace all three.
type StateDir struct {
	Exec      Executor
	Out       io.Writer
	FstabPath string
	Probe     func(dir string) error
	FSType    func(dir string) (string, error)
}

// NewStateDir returns a StateDir that runs commands through exec, writes
// progress to out and uses the real reflink probe and /etc/fstab.
func NewStateDir(exec Executor, out io.Writer) *StateDir {
	return &StateDir{
		Exec:      exec,
		Out:       out,
		FstabPath: DefaultFstabPath,
		Probe:     reflink.Probe,
		FSType:    reflink.FilesystemType,
	}
}

// Setup verifies or creates the state directory as opts describes. See
// StateDirOptions for the three cases.
func (s *StateDir) Setup(opts StateDirOptions) error {
	if opts.Dir == "" {
		return errors.New("state directory must not be empty")
	}

	switch {
	case opts.Device != "" && opts.LoopFile != "":
		return errors.New("give a device or a loop file, not both")
	case opts.Device != "":
		return s.setupDevice(opts)
	case opts.LoopFile != "":
		return s.setupLoop(opts)
	default:
		return s.verify(opts.Dir)
	}
}

// verify creates dir if needed and checks that it supports FICLONE. It
// changes nothing else on the host.
func (s *StateDir) verify(dir string) error {
	if err := os.MkdirAll(dir, stateDirMode); err != nil {
		return fmt.Errorf("creating state directory %s: %w", dir, err)
	}

	fsType, err := s.FSType(dir)
	if err != nil {
		return err
	}

	if fsType != reflink.FilesystemXFS && fsType != reflink.FilesystemBtrfs {
		return fmt.Errorf("%w: %s is on %s; give --device or --loop-file to create an XFS filesystem",
			ErrNoReflink, dir, fsType)
	}

	if err := s.Probe(dir); err != nil {
		if fsType == reflink.FilesystemXFS {
			return fmt.Errorf("%w: %s is on XFS without reflink; format it with mkfs.xfs -m reflink=1: %w",
				ErrNoReflink, dir, err)
		}

		return fmt.Errorf("%w: %s: %w", ErrNoReflink, dir, err)
	}

	fmt.Fprintf(s.Out, "State directory %s is on %s with reflink support\n", dir, fsType)

	return nil
}

// setupDevice formats opts.Device as XFS, mounts it at opts.Dir and
// persists the mount by UUID.
func (s *StateDir) setupDevice(opts StateDirOptions) error {
	if err := s.ensureNotMounted(opts.Dir); err != nil {
		return err
	}

	if err := s.formatXFS(opts.Device, opts.Force); err != nil {
		return err
	}

	uuid, err := s.Exec.Output("blkid", "-p", "-s", "UUID", "-o", "value", opts.Device)
	if err != nil {
		return fmt.Errorf("reading UUID of %s: %w", opts.Device, err)
	}

	if uuid == "" {
		return fmt.Errorf("blkid returned no UUID for %s", opts.Device)
	}

	if err := s.mountPersistent("UUID="+uuid, opts.Dir, "defaults,nofail"); err != nil {
		return err
	}

	return s.verify(opts.Dir)
}

// setupLoop creates a sparse file, formats it as XFS through a loop device
// and mounts it at opts.Dir with the fstab "loop" option. The loop device
// used for formatting is detached again; mount(8) attaches its own.
func (s *StateDir) setupLoop(opts StateDirOptions) error {
	if err := s.ensureNotMounted(opts.Dir); err != nil {
		return err
	}

	sizeGB := opts.SizeGB
	if sizeGB <= 0 {
		sizeGB = DefaultStateDirLoopSizeGB
	}

	if err := CreateSparseFile(opts.LoopFile, fmt.Sprintf("%dG", sizeGB)); err != nil {
		return err
	}

	device, err := AssociateLoopDevice(s.Exec, opts.LoopFile)
	if err != nil {
		return err
	}

	if err := s.formatLoopDevice(device, opts.LoopFile, opts.Force); err != nil {
		return err
	}

	if err := s.mountPersistent(opts.LoopFile, opts.Dir, "loop,nofail"); err != nil {
		return err
	}

	return s.verify(opts.Dir)
}

// formatLoopDevice formats device, which is attached to file, and detaches
// it again on every path. A failed format must not leave the loop device
// allocated. mount(8) attaches its own loop device from the fstab entry.
func (s *StateDir) formatLoopDevice(device, file string, force bool) error {
	formatErr := s.formatXFS(device, force)

	if err := s.Exec.Run("losetup", "--detach", device); err != nil {
		detachErr := fmt.Errorf("detaching %s from %s: %w", device, file, err)
		if formatErr != nil {
			return errors.Join(formatErr, detachErr)
		}

		return detachErr
	}

	return formatErr
}

// ensureNotMounted fails when dir is already a mount point. The provisioner
// handles a fresh or drained host only and never replaces a live mount.
func (s *StateDir) ensureNotMounted(dir string) error {
	source, err := s.Exec.Output("findmnt", "-rno", "SOURCE", "--mountpoint", dir)
	if err == nil && source != "" {
		return fmt.Errorf("%s is already a mount point for %s; unmount it first", dir, source)
	}

	return nil
}

// formatXFS runs mkfs.xfs with reflink enabled on device. It refuses a
// device that already holds a filesystem or partition table unless force
// is set.
func (s *StateDir) formatXFS(device string, force bool) error {
	existing, err := s.Exec.Output("blkid", "-p", "-s", "TYPE", "-s", "PTTYPE", "-o", "value", device)
	if err == nil && existing != "" {
		if !force {
			return fmt.Errorf("%s already holds a filesystem or partition table (%s); pass --force to format it",
				device, strings.ReplaceAll(existing, "\n", " "))
		}

		fmt.Fprintf(s.Out, "WARNING: formatting %s and destroying its existing %s data\n",
			device, strings.ReplaceAll(existing, "\n", " "))
	}

	args := []string{"-m", "reflink=1", "-q"}
	if force {
		args = append(args, "-f")
	}

	args = append(args, device)

	fmt.Fprintf(s.Out, "Formatting %s as XFS with reflink=1\n", device)

	if err := s.Exec.Run("mkfs.xfs", args...); err != nil {
		return fmt.Errorf("formatting %s as xfs: %w", device, err)
	}

	return nil
}

// mountPersistent writes an fstab entry for spec at dir, reloads systemd
// so it picks up the new mount unit, and mounts dir from that entry.
func (s *StateDir) mountPersistent(spec, dir, options string) error {
	if err := os.MkdirAll(dir, stateDirMode); err != nil {
		return fmt.Errorf("creating state directory %s: %w", dir, err)
	}

	line := BuildFstabLine(spec, dir, StateDirFilesystem, options)

	if err := UpdateFstab(s.FstabPath, dir, line); err != nil {
		return err
	}

	fmt.Fprintf(s.Out, "Persisted mount in %s: %s\n", s.FstabPath, line)

	if err := ReloadSystemd(s.Exec); err != nil {
		return err
	}

	if err := s.Exec.Run("mount", dir); err != nil {
		return fmt.Errorf("mounting %s: %w", dir, err)
	}

	return nil
}

// BuildFstabLine renders one fstab entry. Both fsck fields are 0: xfs is
// checked by its own tools, not by fsck at boot.
func BuildFstabLine(spec, dir, fsType, options string) string {
	return fmt.Sprintf("%s %s %s %s 0 0", spec, dir, fsType, options)
}

// UpdateFstab replaces every entry in the fstab at path whose mount point
// is dir with line, or appends line when there is none. A missing file is
// created.
func UpdateFstab(path, dir, line string) error {
	content, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("reading %s: %w", path, err)
	}

	updated := ReplaceFstabEntry(string(content), dir, line)

	if err := os.WriteFile(path, []byte(updated), fstabMode); err != nil {
		return fmt.Errorf("writing %s: %w", path, err)
	}

	return nil
}

// ReplaceFstabEntry returns fstab with every entry for the mount point dir
// replaced by line. When fstab has no entry for dir, line is appended.
func ReplaceFstabEntry(fstab, dir, line string) string {
	var lines []string
	if strings.TrimSpace(fstab) != "" {
		lines = strings.Split(strings.TrimRight(fstab, "\n"), "\n")
	}

	out := make([]string, 0, len(lines)+1)
	replaced := false

	for _, existing := range lines {
		if fstabMountPoint(existing) != dir {
			out = append(out, existing)

			continue
		}

		if !replaced {
			out = append(out, line)
			replaced = true
		}
	}

	if !replaced {
		out = append(out, line)
	}

	return strings.Join(out, "\n") + "\n"
}

// fstabMountPoint returns the mount point field of an fstab entry, or ""
// for a comment or blank line.
func fstabMountPoint(line string) string {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" || strings.HasPrefix(trimmed, "#") {
		return ""
	}

	fields := strings.Fields(trimmed)
	if len(fields) < 2 {
		return ""
	}

	return fields[1]
}
