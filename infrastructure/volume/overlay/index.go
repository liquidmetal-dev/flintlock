package overlay

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/opencontainers/go-digest"
	"golang.org/x/sys/unix"
)

const (
	basesDir    = "bases"
	bySourceDir = "by-source"
	locksDir    = ".locks"
	scratchDir  = "scratch"
	vmDir       = "vm"
	baseExt     = ".img"

	dirPerm  = 0o755
	basePerm = 0o400
	lockPerm = 0o600
)

// baseKey identifies a base: the rootfs chain ID of its image and the
// profile it was built with.
type baseKey struct {
	ChainID digest.Digest
	Profile string
}

// String is the documented string form of a key, "<chain-id>/<profile>",
// for example "sha256:0123.../erofs-v1". BaseUUID derives the filesystem
// UUID of the merged base from it, and it is the path of the by-source
// link below bases/by-source/.
func (k baseKey) String() string {
	return k.ChainID.String() + "/" + k.Profile
}

// lockName is the file name of the key's lock under bases/.locks/.
func (k baseKey) lockName() string {
	return strings.ReplaceAll(k.ChainID.String(), ":", "-") + "-" + k.Profile + ".lock"
}

// Base describes a built base on the host.
type Base struct {
	// Digest is the sha256 of the base file in hex.
	Digest string
	// Path is the absolute path of the base file, bases/<sha256>.img.
	Path string
	// ChainID is the rootfs chain ID of the image the base was built from.
	ChainID digest.Digest
	// Profile is the name of the profile the base was built with.
	Profile string
}

// Reference is a VM's link to a base.
type Reference struct {
	// Digest is the sha256 of the base the link resolves to.
	Digest string
	// Link is the path of the VM's root.base symlink.
	Link string
}

// baseIndex finds and records bases. The filesystem is its only
// implementation today. The sweep lists its entries and uses References and
// Lock to remove unused bases.
type baseIndex interface {
	// Lock takes the lock of key and returns the function that releases
	// it. It waits until the lock is free or ctx ends.
	Lock(ctx context.Context, key baseKey) (release func() error, err error)
	// Lookup returns the base recorded for key. The error wraps
	// errBaseNotInIndex when there is none.
	Lookup(key baseKey) (*Base, error)
	// Record links key to the base file at path. It replaces an old link.
	Record(key baseKey, path string) error
	// Touch sets the last-use time of key's link to now.
	Touch(key baseKey) error
	// LastUse returns the last-use time of key. The error wraps
	// errBaseNotInIndex when the key has no link.
	LastUse(key baseKey) (time.Time, error)
	// Entries lists every by-source link of the index.
	Entries() ([]Entry, error)
	// Remove deletes the by-source link of key. A missing link is a success.
	Remove(key baseKey) error
	// References lists every VM link to a base under the state root.
	References() ([]Reference, error)
}

// fsIndex is the base index on the filesystem of the state root.
type fsIndex struct {
	root string
}

var _ baseIndex = (*fsIndex)(nil)

func newFSIndex(root string) (*fsIndex, error) {
	for _, dir := range []string{
		filepath.Join(root, basesDir),
		filepath.Join(root, basesDir, bySourceDir),
		filepath.Join(root, basesDir, locksDir),
		filepath.Join(root, scratchDir),
	} {
		if err := os.MkdirAll(dir, dirPerm); err != nil {
			return nil, fmt.Errorf("creating %s: %w", dir, err)
		}
	}

	return &fsIndex{root: root}, nil
}

func (i *fsIndex) basesPath() string {
	return filepath.Join(i.root, basesDir)
}

func (i *fsIndex) linkPath(key baseKey) string {
	return filepath.Join(i.basesPath(), bySourceDir, key.ChainID.String(), key.Profile)
}

// Lock takes an exclusive flock on the key's lock file. It is a file lock
// and not a mutex because the daemon initialises more than one set of
// ports, and the kernel drops the lock when the process dies.
func (i *fsIndex) Lock(ctx context.Context, key baseKey) (func() error, error) {
	return lockFile(ctx, filepath.Join(i.basesPath(), locksDir, key.lockName()))
}

// Lookup follows the by-source link of key to the base file.
func (i *fsIndex) Lookup(key baseKey) (*Base, error) {
	link := i.linkPath(key)

	target, err := os.Readlink(link)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("%s: %w", key, errBaseNotInIndex)
		}

		return nil, fmt.Errorf("reading link %s: %w", link, err)
	}

	if !filepath.IsAbs(target) {
		target = filepath.Join(filepath.Dir(link), target)
	}

	sum, err := digestFromPath(target)
	if err != nil {
		return nil, fmt.Errorf("link %s: %w", link, err)
	}

	if _, err := os.Stat(target); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("%s: link to missing %s: %w", key, target, errBaseNotInIndex)
		}

		return nil, fmt.Errorf("stat %s: %w", target, err)
	}

	return &Base{Digest: sum, Path: target, ChainID: key.ChainID, Profile: key.Profile}, nil
}

// Record creates the by-source link of key as a relative link to the base
// file. A temporary link is renamed over the final name, so a reader never
// sees a half-made link.
func (i *fsIndex) Record(key baseKey, path string) error {
	link := i.linkPath(key)
	dir := filepath.Dir(link)

	if err := os.MkdirAll(dir, dirPerm); err != nil {
		return fmt.Errorf("creating %s: %w", dir, err)
	}

	target, err := filepath.Rel(dir, path)
	if err != nil {
		return fmt.Errorf("relative link target for %s: %w", path, err)
	}

	tmp, err := os.CreateTemp(dir, "."+key.Profile+".tmp-")
	if err != nil {
		return fmt.Errorf("creating temporary link name: %w", err)
	}

	tmpName := tmp.Name()
	_ = tmp.Close()
	_ = os.Remove(tmpName)

	if err := os.Symlink(target, tmpName); err != nil {
		return fmt.Errorf("creating link %s: %w", tmpName, err)
	}

	if err := os.Rename(tmpName, link); err != nil {
		_ = os.Remove(tmpName)

		return fmt.Errorf("renaming link to %s: %w", link, err)
	}

	return syncDir(dir)
}

// Touch sets the times of the by-source link of key to now.
func (i *fsIndex) Touch(key baseKey) error {
	link := i.linkPath(key)

	if err := unix.Lutimes(link, nil); err != nil {
		return fmt.Errorf("touching %s: %w", link, err)
	}

	return nil
}

// LastUse returns the modification time of the by-source link of key.
func (i *fsIndex) LastUse(key baseKey) (time.Time, error) {
	link := i.linkPath(key)

	info, err := os.Lstat(link)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return time.Time{}, fmt.Errorf("%s: %w", key, errBaseNotInIndex)
		}

		return time.Time{}, fmt.Errorf("stat %s: %w", link, err)
	}

	return info.ModTime(), nil
}

// Entry is one by-source link of the index: the key, the base it points
// to and the time of its last use.
type Entry struct {
	// Key is the chain ID and profile of the link.
	Key baseKey
	// Digest is the sha256 the link target names.
	Digest string
	// Path is the absolute path of the base file the link points to. The
	// file can be missing.
	Path string
	// LastUse is the modification time of the link, which Touch refreshes.
	LastUse time.Time
}

// Entries lists every by-source link of the index.
func (i *fsIndex) Entries() ([]Entry, error) {
	pattern := filepath.Join(i.basesPath(), bySourceDir, "*", "*")

	links, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("listing %s: %w", pattern, err)
	}

	entries := make([]Entry, 0, len(links))

	for _, link := range links {
		info, err := os.Lstat(link)
		if err != nil {
			return nil, fmt.Errorf("stat %s: %w", link, err)
		}

		if info.Mode()&os.ModeSymlink == 0 {
			continue
		}

		target, err := os.Readlink(link)
		if err != nil {
			return nil, fmt.Errorf("reading link %s: %w", link, err)
		}

		if !filepath.IsAbs(target) {
			target = filepath.Join(filepath.Dir(link), target)
		}

		sum, err := digestFromPath(target)
		if err != nil {
			return nil, fmt.Errorf("link %s: %w", link, err)
		}

		entries = append(entries, Entry{
			Key: baseKey{
				ChainID: digest.Digest(filepath.Base(filepath.Dir(link))),
				Profile: filepath.Base(link),
			},
			Digest:  sum,
			Path:    target,
			LastUse: info.ModTime(),
		})
	}

	return entries, nil
}

// Remove deletes the by-source link of key. A missing link is a success.
func (i *fsIndex) Remove(key baseKey) error {
	link := i.linkPath(key)

	if err := os.Remove(link); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("removing %s: %w", link, err)
	}

	return nil
}

// References scans vm/*/*/*/volumes/root.base and returns one entry per
// link. A link whose target name is not a base digest is reported with an
// error.
func (i *fsIndex) References() ([]Reference, error) {
	pattern := filepath.Join(i.root, vmDir, "*", "*", "*", "volumes", "root.base")

	links, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("listing %s: %w", pattern, err)
	}

	refs := make([]Reference, 0, len(links))

	for _, link := range links {
		target, err := os.Readlink(link)
		if err != nil {
			return nil, fmt.Errorf("reading link %s: %w", link, err)
		}

		sum, err := digestFromPath(target)
		if err != nil {
			return nil, fmt.Errorf("link %s: %w", link, err)
		}

		refs = append(refs, Reference{Digest: sum, Link: link})
	}

	return refs, nil
}

// digestFromPath returns the sha256 hex a base file name holds.
func digestFromPath(path string) (string, error) {
	name := filepath.Base(path)

	sum, ok := strings.CutSuffix(name, baseExt)
	if !ok || len(sum) != hex.EncodedLen(32) {
		return "", fmt.Errorf("%q is not a base file name", name)
	}

	if _, err := hex.DecodeString(sum); err != nil {
		return "", fmt.Errorf("%q is not a base file name: %w", name, err)
	}

	return sum, nil
}

// syncDir fsyncs a directory so that a rename or link in it is durable.
func syncDir(dir string) error {
	d, err := os.Open(dir)
	if err != nil {
		return fmt.Errorf("opening %s: %w", dir, err)
	}

	defer d.Close()

	if err := d.Sync(); err != nil {
		return fmt.Errorf("syncing %s: %w", dir, err)
	}

	return nil
}
