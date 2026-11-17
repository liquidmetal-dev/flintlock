//go:build e2e
// +build e2e

package utils

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	g "github.com/onsi/gomega"

	"github.com/liquidmetal-dev/flintlock/api/services/microvm/v1alpha1"
	execv1 "github.com/liquidmetal-dev/flintlock/api/services/microvmexec/v1alpha1"
	snapv1 "github.com/liquidmetal-dev/flintlock/api/services/snapshot/v1alpha1"
	"github.com/liquidmetal-dev/flintlock/api/types"
	"github.com/liquidmetal-dev/flintlock/core/models"
)

const (
	// SnapshotsDir is the directory of the snapshots of flintlockd, under its
	// state directory.
	SnapshotsDir = StateRootDir + "/snapshots"
	// SocketRootDir is the directory of the sockets of the microvms, which is
	// the default of flintlockd.
	SocketRootDir = "/run/flintlock"

	// SnapshotBaseLink and SnapshotRWDisk are the files of the captured overlay
	// root in the directory of a snapshot.
	SnapshotBaseLink = overlayBaseLink
	SnapshotRWDisk   = overlayRWDisk
	// SnapshotRefsDir is the directory of a snapshot with a file for each
	// restored microvm which uses the files of the snapshot.
	SnapshotRefsDir = "refs"

	snapshotArtefactExt = ".json"

	// The exec of the guest-agent ends a command which runs for longer.
	execTimeoutSeconds = 20

	// counterFields are the fields of a line of the counter of the guest: the
	// count, the wall clock and the uptime of the guest.
	counterFields = 3
)

// SnapshotFlags returns the flags of flintlockd which the snapshot tests
// need: the exec API, which runs the commands in the guest, and the longest
// time a snapshot keeps its source paused.
func SnapshotFlags(maxPause time.Duration) []string {
	return []string{"--enable-exec-api", "--snapshot-max-pause", maxPause.String()}
}

// SnapshotDir returns the directory of the snapshot with the uid.
func SnapshotDir(uid string) string {
	return filepath.Join(SnapshotsDir, uid)
}

// CreateSnapshot asks flintlockd to take a snapshot of a microvm.
func CreateSnapshot(
	client snapv1.SnapshotServiceClient,
	sourceUID, name string,
	labels map[string]string,
) *types.Snapshot {
	ctx, cancel := requestContext()
	defer cancel()

	res, err := client.CreateSnapshot(ctx, &snapv1.CreateSnapshotRequest{
		SourceUid: sourceUID,
		Name:      &name,
		Labels:    labels,
	})
	g.Expect(err).NotTo(g.HaveOccurred(), "creating snapshot %s of microvm %s", name, sourceUID)

	return res.GetSnapshot()
}

// GetSnapshot returns the snapshot with the uid. It returns the error of
// flintlockd, which is NotFound for a snapshot which is deleted.
func GetSnapshot(client snapv1.SnapshotServiceClient, uid string) (*types.Snapshot, error) {
	ctx, cancel := requestContext()
	defer cancel()

	res, err := client.GetSnapshot(ctx, &snapv1.GetSnapshotRequest{Ref: SnapshotRefOf(uid)})
	if err != nil {
		return nil, err
	}

	return res.GetSnapshot(), nil
}

// ListSnapshots returns the snapshots of the namespace.
func ListSnapshots(client snapv1.SnapshotServiceClient, ns string) []*types.Snapshot {
	ctx, cancel := requestContext()
	defer cancel()

	res, err := client.ListSnapshots(ctx, &snapv1.ListSnapshotsRequest{Namespace: ns})
	g.Expect(err).NotTo(g.HaveOccurred(), "listing the snapshots of namespace %s", ns)

	return res.GetSnapshots()
}

// DeleteSnapshot asks flintlockd to delete the snapshot with the uid.
func DeleteSnapshot(client snapv1.SnapshotServiceClient, uid string) error {
	ctx, cancel := requestContext()
	defer cancel()

	_, err := client.DeleteSnapshot(ctx, &snapv1.DeleteSnapshotRequest{Ref: SnapshotRefOf(uid)})

	return err
}

// SnapshotRefOf returns the reference to the snapshot with the uid.
func SnapshotRefOf(uid string) *types.SnapshotRef {
	return &types.SnapshotRef{Ref: &types.SnapshotRef_Uid{Uid: uid}}
}

// RestoreMVM asks flintlockd to create a microvm from a snapshot. The spec,
// which a restore must not have, is sent when it is not nil. It returns the
// error of flintlockd, as a restore can be refused.
func RestoreMVM(
	client v1alpha1.MicroVMClient,
	source *types.SnapshotSource,
	spec *types.MicroVMSpec,
) (*v1alpha1.CreateMicroVMResponse, error) {
	ctx, cancel := requestContext()
	defer cancel()

	return client.CreateMicroVM(ctx, &v1alpha1.CreateMicroVMRequest{
		Microvm:        spec,
		SnapshotSource: source,
	})
}

// ExecResult is what a command which ran in a guest wrote and its exit code.
type ExecResult struct {
	Stdout   string
	Stderr   string
	ExitCode int32
	// Errors are the errors which flintlockd sent while the command ran.
	Errors []string
}

// Err returns an error with the output of the command, if the command did not
// exit with 0 or flintlockd sent an error.
func (r ExecResult) Err() error {
	if r.ExitCode == 0 && len(r.Errors) == 0 {
		return nil
	}

	return fmt.Errorf("exit code %d, errors %q, stdout %q, stderr %q", r.ExitCode, r.Errors, r.Stdout, r.Stderr)
}

// ExecShell runs a shell script in the guest of a microvm through the
// guest-agent, and returns what it wrote and its exit code. The error is
// the one of the stream, a script which fails is not an error here.
func ExecShell(client execv1.MicroVMExecClient, uid, script string) (ExecResult, error) {
	ctx, cancel := requestContext()
	defer cancel()

	result := ExecResult{}

	stream, err := client.ExecCommand(ctx)
	if err != nil {
		return result, fmt.Errorf("opening the exec stream of microvm %s: %w", uid, err)
	}

	err = stream.Send(&execv1.ExecCommandRequest{Payload: &execv1.ExecCommandRequest_Start{Start: &execv1.ExecStart{
		Uid:            uid,
		Cmd:            script,
		Shell:          true,
		TimeoutSeconds: execTimeoutSeconds,
	}}})
	if err != nil {
		return result, fmt.Errorf("sending the exec start to microvm %s: %w", uid, err)
	}

	if err := stream.CloseSend(); err != nil {
		return result, fmt.Errorf("closing the exec stream of microvm %s: %w", uid, err)
	}

	var stdout, stderr strings.Builder

	for {
		res, err := stream.Recv()
		if err != nil {
			result.Stdout, result.Stderr = stdout.String(), stderr.String()

			if errors.Is(err, io.EOF) {
				return result, fmt.Errorf("the exec stream of microvm %s ended without an exit code (stdout %q, stderr %q)",
					uid, result.Stdout, result.Stderr)
			}

			return result, fmt.Errorf("receiving from the exec stream of microvm %s (stdout %q, stderr %q): %w",
				uid, result.Stdout, result.Stderr, err)
		}

		switch payload := res.GetPayload().(type) {
		case *execv1.ExecCommandResponse_Stdout:
			stdout.Write(payload.Stdout)
		case *execv1.ExecCommandResponse_Stderr:
			stderr.Write(payload.Stderr)
		case *execv1.ExecCommandResponse_Error:
			result.Errors = append(result.Errors, payload.Error)
		case *execv1.ExecCommandResponse_ExitCode:
			result.Stdout, result.Stderr = stdout.String(), stderr.String()
			result.ExitCode = payload.ExitCode

			return result, nil
		}
	}
}

// CounterLine is a line which the counter of a guest appended to its file.
type CounterLine struct {
	// N is the count, the first line has 1.
	N int
	// Realtime is the wall clock of the guest, in seconds.
	Realtime float64
	// Uptime is the uptime of the guest, in seconds.
	Uptime float64
	// Text is the line as it is in the file.
	Text string
}

// CounterScript returns a script which starts a counter in the guest. It
// appends a line to the file once a second, with the count, the wall clock
// and the uptime. The counter runs as a unit of systemd, so that it does not
// end with the exec which starts it.
func CounterScript(unit, file string) string {
	return fmt.Sprintf(`mkdir -p %[1]s && systemd-run --unit %[2]s --collect /bin/sh -c `+
		`'n=0; while :; do n=$((n+1)); echo "$n $(date +%%s.%%N) $(cut -d" " -f1 /proc/uptime)" >> %[3]s; sleep 1; done'`,
		filepath.Dir(file), unit, file)
}

// ParseCounter returns the lines of the file of a counter. A last line
// without a newline is still being written, and is left out. The counts have
// to go up by one from 1.
func ParseCounter(content string) ([]CounterLine, error) {
	lines := []CounterLine{}

	complete := content
	if i := strings.LastIndexByte(complete, '\n'); i >= 0 {
		complete = complete[:i+1]
	} else {
		complete = ""
	}

	scanner := bufio.NewScanner(strings.NewReader(complete))
	for scanner.Scan() {
		text := scanner.Text()

		fields := strings.Fields(text)
		if len(fields) != counterFields {
			return nil, fmt.Errorf("line %d of the counter has %d fields, not %d: %q",
				len(lines)+1, len(fields), counterFields, text)
		}

		n, err := strconv.Atoi(fields[0])
		if err != nil {
			return nil, fmt.Errorf("reading the count of line %q: %w", text, err)
		}

		realtime, err := strconv.ParseFloat(fields[1], 64)
		if err != nil {
			return nil, fmt.Errorf("reading the wall clock of line %q: %w", text, err)
		}

		uptime, err := strconv.ParseFloat(fields[2], 64)
		if err != nil {
			return nil, fmt.Errorf("reading the uptime of line %q: %w", text, err)
		}

		if n != len(lines)+1 {
			return nil, fmt.Errorf("line %d of the counter has the count %d: %q", len(lines)+1, n, text)
		}

		lines = append(lines, CounterLine{N: n, Realtime: realtime, Uptime: uptime, Text: text})
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("reading the counter: %w", err)
	}

	return lines, nil
}

// MaxCounterGap returns the longest time between two lines of the counter by
// the wall clock of the guest, and the index of the line before it. It
// returns -1 when there are fewer than two lines.
func MaxCounterGap(lines []CounterLine) (time.Duration, int) {
	longest := time.Duration(0)
	before := -1

	for i := 1; i < len(lines); i++ {
		gap := time.Duration((lines[i].Realtime - lines[i-1].Realtime) * float64(time.Second))
		if before < 0 || gap > longest {
			longest = gap
			before = i - 1
		}
	}

	return longest, before
}

// CommonCounterLines returns how many lines from the start are the same in
// both of the counters.
func CommonCounterLines(first, second []CounterLine) int {
	n := 0

	for n < len(first) && n < len(second) && first[n].Text == second[n].Text {
		n++
	}

	return n
}

// ParseUptime returns the uptime of a guest from the content of its
// /proc/uptime.
func ParseUptime(content string) (float64, error) {
	fields := strings.Fields(content)
	if len(fields) == 0 {
		return 0, fmt.Errorf("no uptime in %q", content)
	}

	uptime, err := strconv.ParseFloat(fields[0], 64)
	if err != nil {
		return 0, fmt.Errorf("reading the uptime of %q: %w", content, err)
	}

	return uptime, nil
}

// FileModes returns the permissions of the regular files under a directory,
// by their path relative to it. Links and directories are left out.
func FileModes(dir string) (map[string]os.FileMode, error) {
	modes := map[string]os.FileMode{}

	err := filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if !entry.Type().IsRegular() {
			return nil
		}

		info, err := entry.Info()
		if err != nil {
			return err
		}

		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}

		modes[rel] = info.Mode().Perm()

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("reading the files of %s: %w", dir, err)
	}

	return modes, nil
}

// DirEntries returns the sorted names of the entries of a directory.
func DirEntries(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", dir, err)
	}

	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name())
	}

	sort.Strings(names)

	return names, nil
}

// ReadSnapshotDescriptor reads the compatibility descriptor of the snapshot
// in dir. A field which the descriptor of flintlock does not have is an
// error. It also returns the JSON of the file, by the name of its fields.
func ReadSnapshotDescriptor(dir string) (*models.SnapshotDescriptor, map[string]json.RawMessage, error) {
	path := filepath.Join(dir, models.SnapshotDescriptorFilename)

	content, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, fmt.Errorf("reading %s: %w", path, err)
	}

	fields := map[string]json.RawMessage{}
	if err := json.Unmarshal(content, &fields); err != nil {
		return nil, nil, fmt.Errorf("decoding %s: %w", path, err)
	}

	desc := &models.SnapshotDescriptor{}

	decoder := json.NewDecoder(bytes.NewReader(content))
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(desc); err != nil {
		return nil, fields, fmt.Errorf("decoding %s: %w: %s", path, err, content)
	}

	return desc, fields, nil
}

// ReadSnapshotSpec reads the spec of the source microvm in the directory of
// a snapshot.
func ReadSnapshotSpec(dir string) (*models.MicroVMSpec, error) {
	path := filepath.Join(dir, models.SnapshotSpecFilename)

	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}

	spec := &models.MicroVMSpec{}
	if err := json.Unmarshal(content, spec); err != nil {
		return nil, fmt.Errorf("decoding %s: %w", path, err)
	}

	return spec, nil
}

// HostDevices returns the names of the host devices of the network
// interfaces of a microvm, sorted.
func HostDevices(status *types.MicroVMStatus) []string {
	devices := []string{}

	for _, iface := range status.GetNetworkInterfaces() {
		devices = append(devices, iface.GetHostDeviceName())
	}

	sort.Strings(devices)

	return devices
}

// LinkExists reports whether the host has a network device with the name.
func LinkExists(name string) bool {
	_, err := net.InterfaceByName(name)

	return err == nil
}

// IsSocket returns an error if the path is not a unix socket.
func IsSocket(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return fmt.Errorf("reading %s: %w", path, err)
	}

	if info.Mode()&os.ModeSocket == 0 {
		return fmt.Errorf("%s is not a socket, its mode is %s", path, info.Mode())
	}

	return nil
}

// PathsWith returns the paths under root whose path relative to root has one
// of the texts in it.
func PathsWith(root string, texts ...string) ([]string, error) {
	found := []string{}

	err := filepath.WalkDir(root, func(path string, _ fs.DirEntry, err error) error {
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				return nil
			}

			return err
		}

		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}

		for _, text := range texts {
			if strings.Contains(rel, text) {
				found = append(found, path)

				break
			}
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("reading the files of %s: %w", root, err)
	}

	return found, nil
}
