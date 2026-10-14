package overlay

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os/exec"
	"strconv"
	"strings"
)

// maxStderr is the number of bytes of mkfs.erofs stderr kept in an error.
const maxStderr = 4096

// mkfs runs the pinned mkfs.erofs with the arguments the profile fixes.
type mkfs struct {
	path    string
	profile Profile
}

// checkVersion runs mkfs.erofs -V and compares the version it reports with
// the profile. The first line of the output is
// "mkfs.erofs (erofs-utils) <version>".
func (m *mkfs) checkVersion(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, m.path, "-V") //nolint:gosec // the path is the configured mkfs.erofs

	var stderr bytes.Buffer

	cmd.Stderr = &stderr

	out, err := cmd.Output()
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			err = errors.Join(ctxErr, err)
		}

		return &MkfsError{Args: []string{"-V"}, Stderr: trimStderr(stderr.Bytes()), Err: err}
	}

	line, _, _ := strings.Cut(string(out), "\n")
	line = strings.TrimSpace(line)

	fields := strings.Fields(line)
	if len(fields) == 0 || fields[len(fields)-1] != m.profile.ErofsUtilsVersion {
		return &VersionError{Path: m.path, Reported: line, Want: m.profile.ErofsUtilsVersion}
	}

	return nil
}

// layerArgs is the command line that converts one tar stream on stdin to
// the EROFS file out.
func (m *mkfs) layerArgs(fsUUID, out string) []string {
	return []string{
		"-b" + strconv.Itoa(m.profile.BlockSize),
		"--tar=f",
		"--aufs",
		"-T0",
		"--mkfs-time",
		"-U", fsUUID,
		out,
		"/dev/stdin",
	}
}

// mergeArgs is the command line that merges the layer files, lowest layer
// first, into the EROFS file out.
func (m *mkfs) mergeArgs(fsUUID, out string, layers []string) []string {
	args := []string{
		"-b" + strconv.Itoa(m.profile.BlockSize),
		"--clean=data",
		"--ovlfs-strip=1",
		"-T0",
		"--mkfs-time",
		"-U", fsUUID,
		out,
	}

	return append(args, layers...)
}

// run executes mkfs.erofs with args. stdin can be nil. A non-zero exit
// gives a MkfsError that carries the standard error of the command.
func (m *mkfs) run(ctx context.Context, args []string, stdin io.Reader) error {
	cmd := exec.CommandContext(ctx, m.path, args...) //nolint:gosec // the path and args are the builder's own
	cmd.Stdin = stdin
	cmd.Stdout = io.Discard

	var stderr bytes.Buffer

	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			err = errors.Join(ctxErr, err)
		}

		return &MkfsError{Args: args, Stderr: trimStderr(stderr.Bytes()), Err: err}
	}

	return nil
}

// trimStderr keeps the last maxStderr bytes of stderr, trimmed.
func trimStderr(b []byte) string {
	if len(b) > maxStderr {
		b = b[len(b)-maxStderr:]
	}

	return strings.TrimSpace(string(b))
}

// CheckMkfsErofs runs the mkfs.erofs at path with -V and checks that it
// reports the erofs-utils version of the erofs-v1 profile. It is the
// startup probe of the base builder.
func CheckMkfsErofs(ctx context.Context, path string) error {
	m := mkfs{path: path, profile: ErofsV1()}

	return m.checkVersion(ctx)
}
