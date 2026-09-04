package provision

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// CreateSparseFile creates an empty file of the given size at path, doing
// nothing if the file already exists, matching create_sparse_file.
func CreateSparseFile(path, size string) error {
	if _, err := os.Stat(path); err == nil {
		return nil
	}

	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("creating sparse file %s: %w", path, err)
	}
	defer f.Close()

	bytes, err := parseSize(size)
	if err != nil {
		return fmt.Errorf("parsing size %s: %w", size, err)
	}

	if err := f.Truncate(bytes); err != nil {
		return fmt.Errorf("truncating sparse file %s to %s: %w", path, size, err)
	}

	return nil
}

// parseSize parses a truncate(1) style size such as "100G" or "10G" into bytes.
func parseSize(size string) (int64, error) {
	if size == "" {
		return 0, fmt.Errorf("empty size")
	}

	unit := size[len(size)-1]

	multiplier := int64(1)
	numeric := size

	switch unit {
	case 'K', 'k':
		multiplier = 1 << 10
	case 'M', 'm':
		multiplier = 1 << 20
	case 'G', 'g':
		multiplier = 1 << 30
	case 'T', 't':
		multiplier = 1 << 40
	default:
		multiplier = 1
	}

	if multiplier != 1 {
		numeric = size[:len(size)-1]
	}

	n, err := strconv.ParseInt(numeric, 10, 64)
	if err != nil {
		return 0, err
	}

	return n * multiplier, nil
}

// AssociateLoopDevice returns the loop device associated with sparseFile,
// creating one if none exists yet, matching associate_loop_device.
func AssociateLoopDevice(runner *Runner, sparseFile string) (string, error) {
	device, err := runner.Output("losetup", "--output", "NAME", "--noheadings", "--associated", sparseFile)
	if err == nil && device != "" {
		return device, nil
	}

	device, err = runner.Output("losetup", "--find", "--show", sparseFile)
	if err != nil {
		return "", fmt.Errorf("associating loop device with %s: %w", sparseFile, err)
	}

	return device, nil
}

// BuildThinPoolTable renders the dmsetup table line for a thin-pool backed
// by metadev/datadev, matching create_dev_thinpool's thinp_table.
func BuildThinPoolTable(lengthSectors int64, metadev, datadev string) string {
	return fmt.Sprintf("0 %d thin-pool %s %s %d %d 1 skip_block_zeroing",
		lengthSectors, metadev, datadev, DataBlockSize, LowWaterMark)
}

// CreateDevThinPool creates (or reloads) the loopback-backed thin-pool
// device named thinpool from metadev/datadev.
func CreateDevThinPool(runner *Runner, thinpool, datadev, metadev string) error {
	sizeOut, err := runner.Output("blockdev", "--getsize64", "-q", datadev)
	if err != nil {
		return fmt.Errorf("getting size of %s: %w", datadev, err)
	}

	dataSize, err := strconv.ParseInt(strings.TrimSpace(sizeOut), 10, 64)
	if err != nil {
		return fmt.Errorf("parsing size of %s: %w", datadev, err)
	}

	lengthSectors := dataSize / SectorSize
	table := BuildThinPoolTable(lengthSectors, metadev, datadev)

	if runner.Run("dmsetup", "reload", thinpool, "--table", table) == nil {
		return nil
	}

	if err := runner.Run("dmsetup", "create", thinpool, "--table", table); err != nil {
		return fmt.Errorf("creating dev thinpool %s: %w", thinpool, err)
	}

	return nil
}

// AllDevPool sets up a loopback-device backed thin-pool named
// thinpool+"-thinpool" using paths.PoolData/PoolMetadata as the backing
// sparse files, matching do_all_devpool.
func AllDevPool(runner *Runner, paths ContainerdPaths, thinpool string) error {
	name := thinpool + "-thinpool"

	if err := CreateSparseFile(paths.PoolData, DataSparseSize); err != nil {
		return err
	}

	if err := CreateSparseFile(paths.PoolMetadata, MetadataSparseSize); err != nil {
		return err
	}

	datadev, err := AssociateLoopDevice(runner, paths.PoolData)
	if err != nil {
		return err
	}

	metadev, err := AssociateLoopDevice(runner, paths.PoolMetadata)
	if err != nil {
		return err
	}

	return CreateDevThinPool(runner, name, datadev, metadev)
}
