package provision

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
)

// FindFreeDisk naively finds a spare block device which is not mounted or
// partitioned. It is not safe to rely on in production - callers should
// prefer an explicit disk name, matching the script's find_free_disk.
func FindFreeDisk(runner *Runner) (string, error) {
	out, err := runner.Output("lsblk", "-o", "NAME,TYPE")
	if err != nil {
		return "", err
	}

	for _, line := range strings.Split(out, "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 || fields[1] != "disk" {
			continue
		}

		name := fields[0]
		if !isMounted(runner, name) && !isPartitioned(runner, name) {
			return name, nil
		}
	}

	return "", errors.New("could not detect a free disk")
}

func isMounted(runner *Runner, deviceName string) bool {
	_, err := runner.Output("findmnt", "-rno", "TARGET", "/dev/"+deviceName)

	return err == nil
}

func isPartitioned(runner *Runner, deviceName string) bool {
	return runner.Run("sfdisk", "-d", "/dev/"+deviceName) == nil
}

// CreatePhysicalVolume creates an LVM physical volume on diskPath, doing
// nothing if one already exists.
func CreatePhysicalVolume(runner *Runner, diskPath string) error {
	if runner.Contains(diskPath, "pvdisplay") {
		return nil
	}

	if err := runner.Run("pvcreate", "-q", diskPath); err != nil {
		return fmt.Errorf("creating physical volume on %s: %w", diskPath, err)
	}

	return nil
}

// CreateVolumeGroup creates an LVM volume group named thinpool on diskPath,
// doing nothing if one already exists.
func CreateVolumeGroup(runner *Runner, diskPath, thinpool string) error {
	if runner.Contains(thinpool, "vgdisplay") {
		return nil
	}

	if err := runner.Run("vgcreate", "-q", thinpool, diskPath); err != nil {
		return fmt.Errorf("creating volume group on %s: %w", diskPath, err)
	}

	return nil
}

// CreateLogicalVolume creates and converts the thinpool data/metadata
// logical volumes into a thin-pool, doing nothing if they already exist.
func CreateLogicalVolume(runner *Runner, volumeGroup string) error {
	if runner.Contains(volumeGroup, "lvdisplay") {
		return nil
	}

	if err := runner.Run(
		"lvcreate", "-q", "--wipesignatures", "y", "-n", "thinpool", volumeGroup, "-l", "95%VG",
	); err != nil {
		return fmt.Errorf("creating logical volume for %s thinpool data: %w", volumeGroup, err)
	}

	if err := runner.Run(
		"lvcreate", "-q", "--wipesignatures", "y", "-n", "thinpoolmeta", volumeGroup, "-l", "1%VG",
	); err != nil {
		return fmt.Errorf("creating logical volume for %s thinpool metadata: %w", volumeGroup, err)
	}

	if err := runner.Run("lvconvert", "-q", "-y",
		"--zero", "n",
		"-c", "512K",
		"--thinpool", volumeGroup+"/thinpool",
		"--poolmetadata", volumeGroup+"/thinpoolmeta",
	); err != nil {
		return fmt.Errorf("converting logical volumes to thinpool storage for %s: %w", volumeGroup, err)
	}

	return nil
}

// LVMProfile is the content written to a thinpool's LVM profile.
const LVMProfile = `activation {
thin_pool_autoextend_threshold=80
thin_pool_autoextend_percent=20
}
`

// ApplyLVMProfile creates (if necessary) and applies the LVM profile for thinpool.
func ApplyLVMProfile(runner *Runner, thinpool string) error {
	profile := filepath.Join(ThinpoolProfilePath, thinpool+"-thinpool.profile")
	profileName := thinpool + "-thinpool"

	if err := writeFileIfMissing(profile, LVMProfile); err != nil {
		return err
	}

	if hasLVMProfile(runner, thinpool, profileName) {
		return nil
	}

	if err := runner.Run("lvchange", "-q", "--metadataprofile", profileName, thinpool+"/thinpool"); err != nil {
		return fmt.Errorf("applying lvm profile %s: %w", profile, err)
	}

	return nil
}

// hasLVMProfile reports whether thinpool's logical volume already has
// profileName assigned as its configuration profile.
func hasLVMProfile(runner *Runner, thinpool, profileName string) bool {
	out, err := runner.Output("lvs", "--noheadings", "-o", "profile", thinpool+"/thinpool")
	if err != nil {
		return false
	}

	return strings.TrimSpace(out) == profileName
}

// MonitorLVMProfile tries (up to 5 times) to ensure the lvm profile for
// thinpool is monitored, matching the script's monitor_lvm_profile.
func MonitorLVMProfile(runner *Runner, thinpool string) error {
	for range 5 {
		if !runner.Contains("not monitored", "lvs", "-o+seg_monitor") {
			return nil
		}

		_ = runner.Run("lvchange", "--monitor", "y", thinpool+"/thinpool")
	}

	return fmt.Errorf("failed to monitor lvm profile for %s", thinpool)
}

// AllDirectLVM sets up a direct-lvm backed thinpool on diskPath.
func AllDirectLVM(runner *Runner, diskPath, thinpool string) error {
	if err := CreatePhysicalVolume(runner, diskPath); err != nil {
		return err
	}

	if err := CreateVolumeGroup(runner, diskPath, thinpool); err != nil {
		return err
	}

	if err := CreateLogicalVolume(runner, thinpool); err != nil {
		return err
	}

	if err := ApplyLVMProfile(runner, thinpool); err != nil {
		return err
	}

	return MonitorLVMProfile(runner, thinpool)
}
