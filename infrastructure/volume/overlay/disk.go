package overlay

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"

	"github.com/google/uuid"
)

const (
	templatesDir = "templates"
	rwPrefix     = "rw-"
	rwLabel      = "flintlock-rw"
	diskPerm     = 0o600
	megabyte     = 1 << 20
)

// templateUUID is the filesystem UUID of every writable disk template. It
// is fixed so that two templates of one size are the same bytes.
func templateUUID() uuid.UUID {
	return uuid.NewSHA1(uuidNamespace(), []byte("rw-template"))
}

// mkfsExt4 runs mkfs.ext4 with the arguments the design fixes.
type mkfsExt4 struct {
	path string
}

// args is the command line that formats the file at path. Lazy
// initialisation is off so that the guest does not write inode tables
// into every clone.
func (m *mkfsExt4) args(path string) []string {
	return []string{
		"-q",
		"-E", "lazy_itable_init=0,lazy_journal_init=0",
		"-U", templateUUID().String(),
		"-L", rwLabel,
		path,
	}
}

// format runs mkfs.ext4 on the file at path.
func (m *mkfsExt4) format(ctx context.Context, path string) error {
	args := m.args(path)

	cmd := exec.CommandContext(ctx, m.path, args...) //nolint:gosec // the path and args are the package's own
	cmd.Stdout = io.Discard

	var stderr bytes.Buffer

	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			err = errors.Join(ctxErr, err)
		}

		return &Ext4Error{Args: args, Stderr: trimStderr(stderr.Bytes()), Err: err}
	}

	return nil
}

// templates makes and finds the writable disk templates under the state
// root.
type templates struct {
	root string
	mkfs mkfsExt4
}

func newTemplates(root, mkfsPath string) (*templates, error) {
	for _, dir := range []string{
		filepath.Join(root, templatesDir),
		filepath.Join(root, templatesDir, locksDir),
	} {
		if err := os.MkdirAll(dir, dirPerm); err != nil {
			return nil, fmt.Errorf("creating %s: %w", dir, err)
		}
	}

	return &templates{root: root, mkfs: mkfsExt4{path: mkfsPath}}, nil
}

func (t *templates) name(sizeMB int32) string {
	return rwPrefix + strconv.FormatInt(int64(sizeMB), 10)
}

func (t *templates) path(sizeMB int32) string {
	return filepath.Join(t.root, templatesDir, t.name(sizeMB)+baseExt)
}

// get returns the path of the template of sizeMB megabytes. It makes the
// template on first use: a sparse file formatted in scratch/, fsynced and
// renamed into templates/, under a per-size flock. A second caller waits
// on the lock and then finds the finished template.
func (t *templates) get(ctx context.Context, sizeMB int32) (string, error) {
	path := t.path(sizeMB)

	if _, err := os.Stat(path); err == nil {
		return path, nil
	}

	release, err := lockFile(ctx, filepath.Join(t.root, templatesDir, locksDir, t.name(sizeMB)+".lock"))
	if err != nil {
		return "", err
	}

	defer func() { _ = release() }()

	if _, err := os.Stat(path); err == nil {
		return path, nil
	}

	if err := t.make(ctx, sizeMB, path); err != nil {
		return "", fmt.Errorf("making template %s: %w", path, err)
	}

	return path, nil
}

func (t *templates) make(ctx context.Context, sizeMB int32, final string) error {
	tmp, err := os.CreateTemp(filepath.Join(t.root, scratchDir), t.name(sizeMB)+"-")
	if err != nil {
		return fmt.Errorf("creating scratch file: %w", err)
	}

	tmpName := tmp.Name()

	defer os.Remove(tmpName)

	if err := tmp.Truncate(int64(sizeMB) * megabyte); err != nil {
		_ = tmp.Close()

		return fmt.Errorf("sizing %s: %w", tmpName, err)
	}

	if err := tmp.Chmod(diskPerm); err != nil {
		_ = tmp.Close()

		return fmt.Errorf("setting mode of %s: %w", tmpName, err)
	}

	if err := tmp.Close(); err != nil {
		return fmt.Errorf("closing %s: %w", tmpName, err)
	}

	if err := t.mkfs.format(ctx, tmpName); err != nil {
		return err
	}

	if err := syncFile(tmpName); err != nil {
		return err
	}

	if err := os.Rename(tmpName, final); err != nil {
		return fmt.Errorf("moving template to %s: %w", final, err)
	}

	return syncDir(filepath.Dir(final))
}

// syncFile fsyncs the file at path.
func syncFile(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("opening %s: %w", path, err)
	}

	defer f.Close()

	if err := f.Sync(); err != nil {
		return fmt.Errorf("syncing %s: %w", path, err)
	}

	return nil
}
