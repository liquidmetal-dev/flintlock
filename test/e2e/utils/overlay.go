//go:build e2e
// +build e2e

package utils

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	g "github.com/onsi/gomega"
	"golang.org/x/sys/unix"

	"github.com/liquidmetal-dev/flintlock/api/services/microvm/v1alpha1"
	"github.com/liquidmetal-dev/flintlock/api/types"
)

const (
	// StateRootDir is the state directory which flintlockd is started with.
	// It is the default of flintlockd, the tests do not set it.
	StateRootDir = "/var/lib/flintlock"

	// OverlayRootImage is the image of the root volume of the microvms which
	// boot from an overlay. It is the unmodified ubuntu image, with cloud-init.
	OverlayRootImage = "ghcr.io/liquidmetal-dev/ubuntu:24.04"

	// overlayContractPrefix starts the line which the overlay init writes to
	// the console when it has mounted the root. The names and the values are
	// not taken from the code of the init, so that a test fails if the init
	// changes them.
	overlayContractPrefix = "flintlock-overlay-init: contract="
	overlayBaseFS         = "erofs"
	overlayRWFS           = "ext4"

	// The files of an overlay root in the state directory of a microvm, and
	// the directories of the state root which they point to.
	overlayVolumesDir   = "volumes"
	overlayBaseLink     = "root.base"
	overlayRWDisk       = "root.rw"
	overlayBasesDir     = "bases"
	overlayTemplatesDir = "templates"
	overlayTemplate     = "rw-%d.img"
	overlayBaseExt      = ".img"
	overlayBaseMode     = os.FileMode(0o400)
	overlayCloneMode    = os.FileMode(0o600)

	// overlayUpperDir is the directory of a writable disk which the overlay
	// init gives overlayfs as the upper directory.
	overlayUpperDir = "/upper"
	debugfsBin      = "debugfs"
	e2fsckBin       = "e2fsck"

	sha256HexLength = sha256.Size * 2
)

// OverlayKernel is a kernel which the overlay boot test boots a microvm
// with. The kernel has to have EROFS built in.
type OverlayKernel struct {
	// Version is the version of the kernel, as the tag of its image names it.
	Version string
	// Provider is the provider which boots the kernel, with the image of the
	// kernel in place of the one of the other tests.
	Provider Provider
}

// Name returns the name of the subtest of the kernel.
func (k OverlayKernel) Name() string {
	return k.Provider.Name + "-" + k.Version
}

// Namespace returns the namespace of the microvms of the kernel. A namespace
// cannot have a dot.
func (k OverlayKernel) Namespace() string {
	return k.Provider.Name + "-overlay-" + strings.ReplaceAll(k.Version, ".", "-")
}

// OverlayKernels returns the kernels of the overlay boot test. The tags name
// the builds of mikrolite-images which have EROFS, and are never moved.
func OverlayKernels() []OverlayKernel {
	firecracker510 := Firecracker()
	firecracker510.KernelImage = "ghcr.io/liquidmetal-dev/firecracker-kernel:5.10-20260929-1349"
	firecracker510.KernelFilename = "boot/vmlinux"

	firecracker61 := Firecracker()
	firecracker61.KernelImage = "ghcr.io/liquidmetal-dev/firecracker-kernel:6.1-20260929-1349"
	firecracker61.KernelFilename = "boot/vmlinux"

	cloudHypervisor62 := CloudHypervisor()
	cloudHypervisor62.KernelImage = "ghcr.io/liquidmetal-dev/cloudhypervisor-kernel:6.2-20260929-1411"
	cloudHypervisor62.KernelFilename = "vmlinux.bin"

	return []OverlayKernel{
		{Version: "5.10", Provider: firecracker510},
		{Version: "6.1", Provider: firecracker61},
		{Version: "6.2", Provider: cloudHypervisor62},
	}
}

// OverlayFlags returns the flags which give flintlockd the initrd image which
// boots an overlay root. There are none if the image is empty. The initrd is
// only pulled when a microvm with an overlay root is created, so the flags
// are set for all of the tests.
func OverlayFlags(initrdImage string) []string {
	if initrdImage == "" {
		return []string{}
	}

	return []string{"--overlay-initrd-image", initrdImage}
}

// CreateOverlayMVM creates a microvm whose root volume is an overlay of the
// root image, with a writable disk of sizeMB. The provider has the kernel.
// The spec names no initrd, so flintlockd gives it the default one.
func CreateOverlayMVM(
	client v1alpha1.MicroVMClient,
	provider Provider,
	name, ns, rootImage string,
	sizeMB int32,
) *v1alpha1.CreateMicroVMResponse {
	spec := newMicroVMSpec(provider, name, ns)
	spec.RootVolume.Mode = types.VolumeMode_OVERLAY
	spec.RootVolume.SizeInMb = &sizeMB
	spec.RootVolume.Source.ContainerSource = pointyString(rootImage)

	ctx, cancel := requestContext()
	defer cancel()

	created, err := client.CreateMicroVM(ctx, &v1alpha1.CreateMicroVMRequest{Microvm: spec})
	g.Expect(err).NotTo(g.HaveOccurred())

	return created
}

// OverlayContract is the line which the overlay init writes to the console
// when it has mounted the root, split into its fields.
type OverlayContract struct {
	// Line is the line as it is on the console.
	Line string
	// Contract is the version of the guest contract.
	Contract string
	// Base is the device of the base in the guest.
	Base string
	// BaseFS is the filesystem of the base.
	BaseFS string
	// RW is the device of the writable disk in the guest.
	RW string
	// RWFS is the filesystem of the writable disk.
	RWFS string
	// Init is the init of the image which the init of the overlay has
	// handed over to.
	Init string
}

// IsOverlay returns an error if the fields of the line do not show a root
// which is an overlay of an EROFS base and an ext4 writable disk.
func (c OverlayContract) IsOverlay() error {
	if c.BaseFS != overlayBaseFS {
		return fmt.Errorf("the base is %q, not %s: %s", c.BaseFS, overlayBaseFS, c.Line)
	}

	if c.RWFS != overlayRWFS {
		return fmt.Errorf("the writable disk is %q, not %s: %s", c.RWFS, overlayRWFS, c.Line)
	}

	if c.Base == "" || c.RW == "" || c.Base == c.RW {
		return fmt.Errorf("the base and the writable disk are not two devices: %s", c.Line)
	}

	return nil
}

// ConsoleOverlayContract returns the contract line of the overlay init from
// the console of a microvm. It is an error if there is no such line.
func ConsoleOverlayContract(stateDir string, provider Provider) (OverlayContract, error) {
	consoleFile := filepath.Join(stateDir, provider.ConsoleFile)

	console, err := os.ReadFile(consoleFile)
	if err != nil {
		return OverlayContract{}, fmt.Errorf("reading the console of the guest: %w", err)
	}

	for _, line := range strings.Split(string(console), "\n") {
		if !strings.Contains(line, overlayContractPrefix) {
			continue
		}

		return ParseOverlayContract(line)
	}

	return OverlayContract{}, fmt.Errorf("there is no line of the overlay init in %s (%d bytes)",
		consoleFile, len(console))
}

// ParseOverlayContract splits the contract line of the overlay init into its
// fields. The line can have what the kernel or the VMM put before it.
func ParseOverlayContract(line string) (OverlayContract, error) {
	line = strings.TrimSpace(line)

	_, fields, found := strings.Cut(line, overlayContractPrefix)
	if !found {
		return OverlayContract{}, fmt.Errorf("the line is not the contract line of the overlay init: %s", line)
	}

	contract := OverlayContract{Line: line}
	values := map[string]*string{
		"contract": &contract.Contract,
		"base":     &contract.Base,
		"base_fs":  &contract.BaseFS,
		"rw":       &contract.RW,
		"rw_fs":    &contract.RWFS,
		"init":     &contract.Init,
	}

	for _, field := range strings.Fields("contract=" + fields) {
		key, value, _ := strings.Cut(field, "=")

		if target, known := values[key]; known {
			*target = value
		}
	}

	if contract.Contract == "" {
		return OverlayContract{}, fmt.Errorf("the contract line has no contract version: %s", line)
	}

	return contract, nil
}

// OverlayVolumes are the files of the overlay root in the state directory of
// a microvm.
type OverlayVolumes struct {
	// BaseLink is the link to the base, which the microvm shares.
	BaseLink string
	// RWDisk is the writable disk of the microvm.
	RWDisk string
}

// OverlayVolumesOf returns the files of the overlay root of the microvm with
// the state directory.
func OverlayVolumesOf(stateDir string) OverlayVolumes {
	return OverlayVolumes{
		BaseLink: filepath.Join(stateDir, overlayVolumesDir, overlayBaseLink),
		RWDisk:   filepath.Join(stateDir, overlayVolumesDir, overlayRWDisk),
	}
}

// OverlayTemplatePath returns the template which the writable disks of
// sizeMB are cloned from.
func OverlayTemplatePath(stateRoot string, sizeMB int32) string {
	return filepath.Join(stateRoot, overlayTemplatesDir, fmt.Sprintf(overlayTemplate, sizeMB))
}

// OverlayBase is a base in the bases directory of the state root, as it was
// when it was verified.
type OverlayBase struct {
	// Path is the file of the base.
	Path string
	// Digest is the sha256 of the file, which is its name.
	Digest string
	// ModTime is the modification time of the file.
	ModTime time.Time
}

// VerifyOverlayBase resolves the link to the base of a microvm, and checks
// that it is a symlink to a read-only file in the bases directory of the
// state root whose name is its sha256. It returns the base.
func VerifyOverlayBase(stateRoot, baseLink string) (OverlayBase, error) {
	info, err := os.Lstat(baseLink)
	if err != nil {
		return OverlayBase{}, fmt.Errorf("reading the link to the base: %w", err)
	}

	if info.Mode()&os.ModeSymlink == 0 {
		return OverlayBase{}, fmt.Errorf("%s is not a symlink, its mode is %s", baseLink, info.Mode())
	}

	path, err := filepath.EvalSymlinks(baseLink)
	if err != nil {
		return OverlayBase{}, fmt.Errorf("resolving the link to the base: %w", err)
	}

	basesDir := filepath.Join(stateRoot, overlayBasesDir)
	if filepath.Dir(path) != basesDir {
		return OverlayBase{}, fmt.Errorf("the base %s is not in %s", path, basesDir)
	}

	digest, err := OverlayBaseDigest(path)
	if err != nil {
		return OverlayBase{}, err
	}

	base := OverlayBase{Path: path, Digest: digest}

	return base, base.Verify()
}

// OverlayBaseDigest returns the sha256 which the name of a base file is.
func OverlayBaseDigest(path string) (string, error) {
	digest, found := strings.CutSuffix(filepath.Base(path), overlayBaseExt)
	if !found || len(digest) != sha256HexLength {
		return "", fmt.Errorf("the name of the base %s is not a sha256 with the extension %s", path, overlayBaseExt)
	}

	if _, err := hex.DecodeString(digest); err != nil {
		return "", fmt.Errorf("the name of the base %s is not a sha256: %w", path, err)
	}

	return digest, nil
}

// Verify checks that the file of the base is read-only, that its sha256 is
// still the one in its name, and that its modification time is the one of
// when it was first verified. The first verification records the time.
func (b *OverlayBase) Verify() error {
	info, err := os.Stat(b.Path)
	if err != nil {
		return fmt.Errorf("reading the base: %w", err)
	}

	if !info.Mode().IsRegular() {
		return fmt.Errorf("the base %s is not a regular file, its mode is %s", b.Path, info.Mode())
	}

	if info.Mode().Perm() != overlayBaseMode {
		return fmt.Errorf("the mode of the base %s is %s, not %s", b.Path, info.Mode().Perm(), overlayBaseMode)
	}

	if b.ModTime.IsZero() {
		b.ModTime = info.ModTime()
	} else if !info.ModTime().Equal(b.ModTime) {
		return fmt.Errorf("the base %s was modified at %s, it was modified at %s before",
			b.Path, info.ModTime().Format(time.RFC3339Nano), b.ModTime.Format(time.RFC3339Nano))
	}

	sum, err := FileSHA256(b.Path)
	if err != nil {
		return err
	}

	if sum != b.Digest {
		return fmt.Errorf("the sha256 of the base %s is %s", b.Path, sum)
	}

	return nil
}

// FileSHA256 returns the sha256 of the content of a file, in hex.
func FileSHA256(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("opening %s: %w", path, err)
	}

	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", fmt.Errorf("reading %s: %w", path, err)
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}

// CloneFile makes dst a reflink of src with FICLONE, as flintlockd clones
// the writable disk of a microvm from its template. Both have to be on the
// same filesystem. A file at dst is replaced.
//
// The blocks of a fresh clone are not the ones of the template: XFS keeps the
// extents which mkfs.ext4 allocated and did not write to as unwritten, and
// does not map them into a clone. A clone has fewer blocks than its template
// until it is written to, so the blocks of a disk are compared with the ones
// of a fresh clone and not with the ones of the template.
func CloneFile(src, dst string) error {
	source, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("opening %s: %w", src, err)
	}

	defer source.Close()

	clone, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, overlayCloneMode)
	if err != nil {
		return fmt.Errorf("creating %s: %w", dst, err)
	}

	defer clone.Close()

	if err := unix.IoctlFileClone(int(clone.Fd()), int(source.Fd())); err != nil {
		return fmt.Errorf("cloning %s to %s: %w", src, dst, err)
	}

	return nil
}

// UpperEntries returns the names of the entries of the upper directory of a
// writable disk, which the guest writes the files of the overlay to.
//
// The disk is read while the guest runs. debugfs reads the image as it is
// and does not replay the journal, so an entry which the guest has only
// committed to the journal is not in its listing. The disk is cloned to a
// snapshot on the same filesystem, e2fsck replays the journal into the
// snapshot, and debugfs lists the snapshot. The listing is what the guest
// had committed when the snapshot was made, which can be behind what the
// guest has written since. The snapshot is removed.
func UpperEntries(rwDisk string) ([]string, error) {
	snapshot := filepath.Join(filepath.Dir(rwDisk), ".e2e-snapshot-of-"+filepath.Base(rwDisk))

	if err := CloneFile(rwDisk, snapshot); err != nil {
		return nil, err
	}

	defer os.Remove(snapshot)

	if err := replayJournal(snapshot); err != nil {
		return nil, err
	}

	cmd := exec.Command(debugfsBin, "-R", "ls -l "+overlayUpperDir, snapshot)

	// debugfs writes what it has to say about the image to stderr. Only the
	// listing is read.
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("listing %s of %s with %s: %w", overlayUpperDir, snapshot, debugfsBin, err)
	}

	return ParseDebugfsListing(string(output)), nil
}

// replayJournal runs e2fsck on a snapshot of a live disk, which replays the
// journal and repairs what the snapshot of a disk in use has. e2fsck exits
// with 1 or 2 when it has repaired something, which is expected here, and
// with 4 or more when it could not.
func replayJournal(snapshot string) error {
	const e2fsckCouldNotRepair = 4

	cmd := exec.Command(e2fsckBin, "-f", "-y", snapshot)

	output, err := cmd.CombinedOutput()
	if err == nil {
		return nil
	}

	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && exitErr.ExitCode() < e2fsckCouldNotRepair {
		return nil
	}

	return fmt.Errorf("replaying the journal of %s with %s: %w: %s",
		snapshot, e2fsckBin, err, strings.Join(Tail(output, 1), ""))
}

// ParseDebugfsListing returns the names of the entries in the output of the
// 'ls -l' command of debugfs, without the entries of the directory itself and
// of its parent. Each line of the listing has the inode, the mode with the
// type in parentheses, the owner, the group, the size, the date, the time and
// then the name.
func ParseDebugfsListing(listing string) []string {
	const fieldsBeforeName = 8

	names := []string{}

	for _, line := range strings.Split(listing, "\n") {
		fields := strings.Fields(line)
		if len(fields) <= fieldsBeforeName {
			continue
		}

		name := strings.Join(fields[fieldsBeforeName:], " ")
		if name == "." || name == ".." {
			continue
		}

		names = append(names, name)
	}

	return names
}

// AllocatedBlocks returns the number of 512 byte blocks which are allocated
// to a file. A link is not followed. The blocks of a fresh clone are not the
// ones of the file it was cloned from, see CloneFile.
func AllocatedBlocks(path string) (int64, error) {
	var stat syscall.Stat_t

	if err := syscall.Lstat(path, &stat); err != nil {
		return 0, fmt.Errorf("reading the blocks of %s: %w", path, err)
	}

	return stat.Blocks, nil
}

// SameInode returns true if the two paths are the same file. A link is
// followed, so that the links to a base compare as the base.
func SameInode(first, second string) (bool, error) {
	firstInfo, err := os.Stat(first)
	if err != nil {
		return false, fmt.Errorf("reading %s: %w", first, err)
	}

	secondInfo, err := os.Stat(second)
	if err != nil {
		return false, fmt.Errorf("reading %s: %w", second, err)
	}

	return os.SameFile(firstInfo, secondInfo), nil
}

// Gone returns an error if the path still exists. A link which points to
// nothing has not gone.
func Gone(path string) error {
	_, err := os.Lstat(path)
	if err == nil {
		return fmt.Errorf("%s still exists", path)
	}

	if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("reading %s: %w", path, err)
	}

	return nil
}

// FormatBlocks returns the blocks and their size in bytes, for a log line.
func FormatBlocks(blocks int64) string {
	const blockSize = 512

	return strconv.FormatInt(blocks, 10) + " blocks (" + strconv.FormatInt(blocks*blockSize, 10) + " bytes)"
}
