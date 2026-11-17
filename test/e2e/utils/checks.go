//go:build e2e
// +build e2e

package utils

import (
	"bytes"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/liquidmetal-dev/flintlock/api/services/microvm/v1alpha1"
)

const (
	// bootMarkerFormat is the final message of cloud-init in a test microvm,
	// with the namespace and the name of the microvm. cloud-init writes it to
	// the console when it has run all of its stages, and replaces $UPTIME with
	// the number of seconds since the boot.
	bootMarkerFormat = "flintlock-e2e boot ok %s/%s uptime="
	bootMarkerUptime = "$UPTIME"

	// ShareMarkerFile is the file which the guest of a microvm with a virtiofs
	// volume writes into the share when it boots, and shareMarkerFormat is
	// its content, with the namespace and the name of the microvm.
	ShareMarkerFile   = "flintlock-e2e-share"
	shareMarkerFormat = "flintlock-e2e share ok %s/%s"

	// VendorDataTriesMarker starts the line which the guest of a microvm that
	// reads the metadata service writes to its console, see
	// vendorDataTriesCommand.
	VendorDataTriesMarker = "flintlock-e2e vendor-data"

	// MetadataInterfaceMarker starts the line which MetadataInterfaceStateCommand
	// writes to the console of the guest.
	MetadataInterfaceMarker = "flintlock-e2e metadata-interface"

	// BalloonMarker starts the lines which BalloonStateCommand and
	// BalloonFillCommand write to the console of the guest.
	BalloonMarker = "flintlock-e2e balloon"

	diagnosticLines = 40
)

// The functions in this file return an error and do not fail the test, so
// that they can be used in a function which is tried again until it succeeds.

// VerifyVMM returns the pid of the VMM of a microvm. It is an error if there
// is no pid, if the process is not running, or if the process is not the VMM
// binary of the provider.
func VerifyVMM(stateDir string, provider Provider) (int, error) {
	binary, err := provider.BinaryPath()
	if err != nil {
		return 0, err
	}

	return verifyProcess(filepath.Join(stateDir, provider.PidFile), binary, "the VMM of the "+provider.Name+" provider")
}

// VerifyVirtioFSD returns the pid of the virtiofsd of a microvm, which the
// VMM gets the virtiofs share from. The binary is the one which flintlockd
// starts, VirtioFSDBin. It is an error if there is no pid, if the process is
// not running, or if the process is not the binary.
func VerifyVirtioFSD(stateDir, binary string) (int, error) {
	resolved, err := binaryPath(binary, "virtiofsd")
	if err != nil {
		return 0, err
	}

	return verifyProcess(filepath.Join(stateDir, VirtioFSDPidFile), resolved, "virtiofsd")
}

// verifyProcess returns the pid in the pid file. It is an error if there is
// no pid, if the process is not running, or if the process is not the binary,
// which is the path that the kernel reports for it. The what is for the
// error, it says whose process it is.
func verifyProcess(pidFile, binary, what string) (int, error) {
	pid, err := readPID(pidFile)
	if err != nil {
		return 0, err
	}

	if !PidRunning(pid) {
		return 0, fmt.Errorf("the process with pid %d from %s is not running", pid, filepath.Base(pidFile))
	}

	executable, err := VMMExecutable(pid)
	if err != nil {
		return 0, err
	}

	if executable != binary {
		return 0, fmt.Errorf("the process with pid %d is %s, %s is %s", pid, executable, what, binary)
	}

	return pid, nil
}

// VMMExecutable returns the path of the binary which the process with the pid
// was started from.
func VMMExecutable(pid int) (string, error) {
	executable, err := os.Readlink(fmt.Sprintf("/proc/%d/exe", pid))
	if err != nil {
		return "", fmt.Errorf("reading the executable of the process with pid %d: %w", pid, err)
	}

	return executable, nil
}

// ShareMarkerContent returns what the guest of a microvm writes into the
// file ShareMarkerFile of its virtiofs share when it boots.
func ShareMarkerContent(namespace, name string) string {
	return fmt.Sprintf(shareMarkerFormat, namespace, name) + "\n"
}

// shareMarkerCommand returns the command with which the guest of a microvm
// writes the share marker. cloud-init runs it in its final stage, after it
// has mounted the share, and before it writes the boot marker.
func shareMarkerCommand(mountPoint, namespace, name string) string {
	return fmt.Sprintf("printf '%%s\\n' '%s' > %s",
		fmt.Sprintf(shareMarkerFormat, namespace, name), filepath.Join(mountPoint, ShareMarkerFile))
}

// mountStateCommand returns the command with which the guest of a microvm
// writes what is mounted at the mount point to its console, so that the
// console shows why the share marker is not in the share.
func mountStateCommand(mountPoint string) string {
	return fmt.Sprintf("findmnt -no SOURCE,FSTYPE,OPTIONS %s > /dev/console"+
		" || echo 'flintlock-e2e nothing is mounted' > /dev/console", mountPoint)
}

// ShareMarker returns the content of the marker file in the directory which
// the host shares with a microvm. It is an error if the file is not there,
// or if it does not have the marker of the microvm.
func ShareMarker(sharePath, name, namespace string) (string, error) {
	file := filepath.Join(sharePath, ShareMarkerFile)

	content, err := os.ReadFile(file)
	if err != nil {
		return "", fmt.Errorf("reading the marker of the share: %w", err)
	}

	expected := ShareMarkerContent(namespace, name)
	if string(content) != expected {
		return "", fmt.Errorf("%s has %q, the marker of %s/%s is %q", file, content, namespace, name, expected)
	}

	return string(content), nil
}

// ConsoleMarker returns the line of the console of a microvm which shows that
// its guest has booted. It is an error if there is no such line.
func ConsoleMarker(stateDir string, provider Provider, name, namespace string) (string, error) {
	consoleFile := filepath.Join(stateDir, provider.ConsoleFile)

	console, err := os.ReadFile(consoleFile)
	if err != nil {
		return "", fmt.Errorf("reading the console of the guest: %w", err)
	}

	// The uptime has to be a number. If it is not, the line is the user-data
	// which something has printed, and not the message of cloud-init.
	//
	// There is nothing before or after the marker in the pattern: an escape
	// sequence can be next to it. A longer namespace or name cannot match, as
	// the marker starts with a fixed text and has the namespace and the name
	// between fixed texts.
	marker := regexp.MustCompile(
		regexp.QuoteMeta(fmt.Sprintf(bootMarkerFormat, namespace, name)) + `[0-9]+(\.[0-9]+)?`)

	for _, line := range bytes.Split(console, []byte("\n")) {
		if marker.Match(line) {
			return strings.TrimSpace(string(line)), nil
		}
	}

	return "", fmt.Errorf("the boot marker of %s/%s is not in %s (%d bytes)", namespace, name, consoleFile, len(console))
}

// MissingPVHErrorLine returns the line of the stderr of the VMM of a microvm
// which says that the kernel has no PVH entry point. It is an error if there
// is no such line.
func MissingPVHErrorLine(stateDir string, provider Provider) (string, error) {
	// Up to v46 Cloud Hypervisor writes "Error booting VM:
	// VmBoot(KernelMissingPvhHeader)". From v48 the reason is on a line of its
	// own, "Kernel lacks PVH header".
	return VMMErrorLine(stateDir, provider, "pvh")
}

// ConsoleLine returns the line of the console of the guest of a microvm which
// has all of the texts, in any case. It is an error if there is no such line.
func ConsoleLine(stateDir string, provider Provider, texts ...string) (string, error) {
	return lineWith(filepath.Join(stateDir, provider.ConsoleFile), "the console of the guest", texts)
}

// VendorDataTries returns how many times the guest of a microvm tried to read
// its vendor-data from the metadata service, and how many of the tries
// succeeded, from the line which vendorDataTriesCommand writes to the console.
func VendorDataTries(stateDir string, provider Provider) (int, int, error) {
	line, err := ConsoleLine(stateDir, provider, VendorDataTriesMarker+" tries=")
	if err != nil {
		return 0, 0, err
	}

	var tries, ok int

	text := line[strings.Index(strings.ToLower(line), VendorDataTriesMarker):]
	if _, err := fmt.Sscanf(text, VendorDataTriesMarker+" tries=%d ok=%d", &tries, &ok); err != nil {
		return 0, 0, fmt.Errorf("parsing the vendor-data line %q: %w", line, err)
	}

	return tries, ok, nil
}

// VMMErrorLine returns the line of the stderr of the VMM of a microvm which
// has all of the texts, in any case. It is an error if there is no such line.
// MetadataInterfaceState returns the systemd-networkd setup state and the IPv4
// address of the metadata interface, from the line that
// MetadataInterfaceStateCommand writes to the console of the guest.
func MetadataInterfaceState(stateDir string, provider Provider) (string, string, error) {
	line, err := ConsoleLine(stateDir, provider, MetadataInterfaceMarker+" setup=")
	if err != nil {
		return "", "", err
	}

	var setup, address string

	_, text, _ := strings.Cut(line, MetadataInterfaceMarker)
	if _, err := fmt.Sscanf(text, " setup=%s address=%s", &setup, &address); err != nil {
		return "", "", fmt.Errorf("parsing the metadata interface line %q: %w", line, err)
	}

	return setup, address, nil
}

func VMMErrorLine(stateDir string, provider Provider, texts ...string) (string, error) {
	return lineWith(filepath.Join(stateDir, provider.StderrFile), "the stderr of the VMM", texts)
}

func lineWith(file, what string, texts []string) (string, error) {
	content, err := os.ReadFile(file)
	if err != nil {
		return "", fmt.Errorf("reading %s: %w", what, err)
	}

	for _, line := range strings.Split(string(content), "\n") {
		if containsAll(strings.ToLower(line), texts) {
			return strings.TrimSpace(line), nil
		}
	}

	return "", fmt.Errorf("there is no line with %q in %s (%d bytes)", texts, file, len(content))
}

func containsAll(line string, texts []string) bool {
	for _, text := range texts {
		if !strings.Contains(line, strings.ToLower(text)) {
			return false
		}
	}

	return true
}

// errNotDeleted is the error of a microvm which is not deleted when the time
// to wait for it is up.
var errNotDeleted = errors.New("the microvm is not deleted")

// DeleteAndWait asks flintlockd to delete a microvm, and waits until it is
// deleted. It returns an error and does not fail the test, so that it can be
// used when the test has failed already.
//
// flintlockd can lose a delete which it gets while it creates the microvm
// (#1266). So the delete is asked for again when the microvm is still there
// after a while.
func DeleteAndWait(
	client v1alpha1.MicroVMClient,
	namespace, uid, stateDir string,
	timeout, polling, again time.Duration,
) error {
	deadline := time.Now().Add(timeout)

	for asked := 1; ; asked++ {
		if err := DeleteMVM(client, uid); err != nil {
			// flintlockd does not know a microvm which is deleted already.
			if reason, listErr := notDeleted(client, namespace, uid, stateDir); listErr == nil && reason == "" {
				return nil
			}

			return fmt.Errorf("asking to delete the microvm %s: %w", uid, err)
		}

		wait := min(again, time.Until(deadline))

		err := WaitForDeleted(client, namespace, uid, stateDir, wait, polling)
		if err == nil || !errors.Is(err, errNotDeleted) {
			return err
		}

		if !time.Now().Before(deadline) {
			return fmt.Errorf("asked %d times in %s: %w", asked, timeout, err)
		}

		log.Printf("TEST INFO: asking to delete the microvm %s again: %s", uid, err)
	}
}

// WaitForDeleted waits until a microvm is not in the list of its namespace
// and has no state directory. It returns an error and does not fail the test,
// so that it can be used when the test has failed already.
//
// The state directory alone does not show that a microvm is deleted: a
// microvm which flintlockd has not started to create does not have one.
func WaitForDeleted(
	client v1alpha1.MicroVMClient,
	namespace, uid, stateDir string,
	timeout, polling time.Duration,
) error {
	deadline := time.Now().Add(timeout)

	for {
		reason, err := notDeleted(client, namespace, uid, stateDir)
		if err != nil {
			return err
		}

		if reason == "" {
			return nil
		}

		if !time.Now().Add(polling).Before(deadline) {
			return fmt.Errorf("%w %s after %s: %s", errNotDeleted, uid, timeout, reason)
		}

		time.Sleep(polling)
	}
}

// notDeleted returns what shows that a microvm is not deleted, or nothing if
// it is deleted.
func notDeleted(client v1alpha1.MicroVMClient, namespace, uid, stateDir string) (string, error) {
	ctx, cancel := requestContext()
	defer cancel()

	listed, err := client.ListMicroVMs(ctx, &v1alpha1.ListMicroVMsRequest{Namespace: namespace})
	if err != nil {
		return "", fmt.Errorf("listing the microvms of the namespace %s: %w", namespace, err)
	}

	for _, microvm := range listed.Microvm {
		if microvm.GetSpec().GetUid() == uid {
			return "it is in the list of the namespace " + namespace, nil
		}
	}

	if _, err := os.Stat(stateDir); !os.IsNotExist(err) {
		return "the state directory " + stateDir + " exists", nil
	}

	return "", nil
}

// DumpDiagnostics logs the end of the files which show why a microvm did not
// start.
func DumpDiagnostics(stateDir string, provider Provider) {
	for _, name := range provider.DiagnosticFiles {
		file := filepath.Join(stateDir, name)

		content, err := os.ReadFile(file)
		if err != nil {
			log.Printf("TEST INFO: cannot read %s: %s", file, err)

			continue
		}

		lines := Tail(content, diagnosticLines)

		log.Printf("TEST INFO: %s has %d bytes, the last %d lines are:", file, len(content), len(lines))

		for _, line := range lines {
			log.Printf("TEST INFO: %s: %s", name, line)
		}
	}
}

// Tail returns the last lines of the content, without the empty lines at its end.
func Tail(content []byte, lines int) []string {
	trimmed := strings.TrimRight(string(content), "\r\n")
	if trimmed == "" {
		return []string{}
	}

	all := strings.Split(trimmed, "\n")
	if len(all) > lines {
		all = all[len(all)-lines:]
	}

	for i := range all {
		all[i] = strings.TrimRight(all[i], "\r")
	}

	return all
}

// BalloonState returns the virtio device that the guest's balloon driver has
// bound, and whether the device and the driver agreed on free page
// reporting, from the line which BalloonStateCommand writes to the console.
// The device is "none" if the driver bound no device.
func BalloonState(stateDir string, provider Provider) (string, bool, error) {
	line, err := ConsoleLine(stateDir, provider, BalloonMarker+" device=")
	if err != nil {
		return "", false, err
	}

	var device, reporting string

	_, text, _ := strings.Cut(line, BalloonMarker)
	if _, err := fmt.Sscanf(text, " device=%s reporting=%s", &device, &reporting); err != nil {
		return "", false, fmt.Errorf("parsing the balloon line %q: %w", line, err)
	}

	return device, reporting == "1", nil
}

// VMMRSSMiB returns the resident set size of the process pid, in MiB, from
// the VmRSS line of /proc/<pid>/status.
func VMMRSSMiB(pid int) (int, error) {
	return rssMiB(fmt.Sprintf("/proc/%d/status", pid))
}

// kibPerMiB is the number of KiB in a MiB.
const kibPerMiB = 1024

func rssMiB(statusFile string) (int, error) {
	content, err := os.ReadFile(statusFile)
	if err != nil {
		return 0, fmt.Errorf("reading %s: %w", statusFile, err)
	}

	for _, line := range strings.Split(string(content), "\n") {
		if !strings.HasPrefix(line, "VmRSS:") {
			continue
		}

		var kib int
		if _, err := fmt.Sscanf(strings.TrimPrefix(line, "VmRSS:"), "%d kB", &kib); err != nil {
			return 0, fmt.Errorf("parsing %q of %s: %w", line, statusFile, err)
		}

		return kib / kibPerMiB, nil
	}

	return 0, fmt.Errorf("%s has no VmRSS line", statusFile)
}
