//go:build e2e
// +build e2e

package e2e_test

import (
	"encoding/base64"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	. "github.com/onsi/gomega"
	"golang.org/x/sys/unix"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/liquidmetal-dev/flintlock/api/services/microvm/v1alpha1"
	execv1 "github.com/liquidmetal-dev/flintlock/api/services/microvmexec/v1alpha1"
	snapv1 "github.com/liquidmetal-dev/flintlock/api/services/snapshot/v1alpha1"
	"github.com/liquidmetal-dev/flintlock/api/types"
	"github.com/liquidmetal-dev/flintlock/core/models"
	"github.com/liquidmetal-dev/flintlock/pkg/ptr"
	u "github.com/liquidmetal-dev/flintlock/test/e2e/utils"
)

const (
	// snapshotMaxPause is the --snapshot-max-pause of flintlockd. The gap in
	// the counter of the source around the snapshot has to be shorter.
	snapshotMaxPause = 10 * time.Second

	// The memory of the guest is written to the snapshot while the source is
	// paused, so it is the smallest which flintlockd accepts.
	snapshotMemoryMB int32 = 1024

	snapshotReadyTimeout = "120s"
	guestAgentTimeout    = "180s"
	snapshotPolling      = "2s"
	snapshotDeleteWait   = 120 * time.Second

	// The files which the test writes in the guest. They are on the root
	// overlay, so they are on the writable disk of the guest.
	guestDir         = "/var/tmp/flintlock-e2e"
	guestMarker      = guestDir + "/marker"
	guestCounter     = guestDir + "/counter"
	guestCounterUnit = "flintlock-e2e-counter"
	guestOwnPrefix   = "own-"

	// The counter has to have this many lines before the snapshot, and grow
	// by this many after it, which shows that it runs.
	counterLinesBefore = 3
	counterGrowth      = 2
)

// snapshotRestoreCase is what the snapshot and restore test needs to know of
// a provider.
type snapshotRestoreCase struct {
	kernel u.OverlayKernel
	// apiSocketFile is the socket in the socket directory of a microvm which
	// the VMM listens on.
	apiSocketFile string
	// retainedFiles are the files of the snapshot directory which a deleted
	// snapshot keeps while a restored microvm uses them. With none, the
	// directory goes when the snapshot is deleted.
	retainedFiles []string
}

// snapshotVM is a microvm of the snapshot and restore test.
type snapshotVM struct {
	name     string
	uid      string
	stateDir string
	pid      int
}

func (v snapshotVM) String() string {
	return v.name + " (" + v.uid + ")"
}

// TestE2EFirecrackerSnapshotRestore takes a snapshot of a running microvm
// with an overlay root and restores it twice, on Firecracker with the 6.1
// kernel of the overlay boot test.
func TestE2EFirecrackerSnapshotRestore(t *testing.T) {
	testSnapshotRestore(t, snapshotRestoreCase{
		kernel:        overlayKernel(t, "firecracker-6.1"),
		apiSocketFile: "firecracker.sock",
		retainedFiles: []string{models.SnapshotMemoryFile, models.SnapshotVMStateFile},
	})
}

func overlayKernel(t *testing.T, name string) u.OverlayKernel {
	t.Helper()

	for _, kernel := range u.OverlayKernels() {
		if kernel.Name() == name {
			return kernel
		}
	}

	t.Fatalf("there is no overlay kernel %s", name)

	return u.OverlayKernel{}
}

// testSnapshotRestore boots a microvm with an overlay root and the guest
// agent, takes a snapshot of it while a counter runs in its guest, and
// restores the snapshot twice. The guests are checked through the exec API,
// and their files on the host.
//
// The test needs what the overlay boot test needs, and is skipped without
// the initrd image of -overlay.initrd.image.
func testSnapshotRestore(t *testing.T, tc snapshotRestoreCase) {
	RegisterTestingT(t)

	provider := tc.kernel.Provider

	if environmentLeftRunning {
		t.Skip("a previous test left its environment running, use -run to run this test on its own")
	}

	if params.OverlayInitrdImage == "" {
		t.Skip("the snapshot and restore test needs the initrd image of -overlay.initrd.image")
	}

	if !selected(provider) {
		t.Skipf("the %s provider is not one of the providers of -providers", provider.Name)
	}

	environmentLeftRunning = params.SkipTeardown || params.SkipDelete

	r := u.NewRunner(params, u.SnapshotFlags(snapshotMaxPause)...)
	defer func() {
		log.Println("TEST STEP: cleaning up running processes")
		r.Teardown()
	}()
	log.Println("TEST STEP: performing setup, starting flintlockd server")

	run := &snapshotRun{
		tc:        tc,
		provider:  provider,
		ns:        provider.Name + "-snapshot",
		microvms:  r.Setup(),
		snapshots: r.SnapshotClient(),
		exec:      r.ExecClient(),
		labels:    map[string]string{"flintlock-e2e": "snapshot-restore"},
	}

	// The cleanups are set up before anything is checked, so that the
	// microvms and the snapshot are also deleted when the first check fails.
	run.createSource()
	defer cleanupOnFailure(t, run.microvms, provider, run.source.stateDir, run.ns, run.source.uid)

	run.prepareSource()

	run.createSnapshot()
	defer cleanupSnapshotOnFailure(t, run.snapshots, run.snapshotDir, run.snapshotUID)

	run.verifySnapshot()

	log.Println("TEST STEP: checking that a restore which changes the vCPU count is refused")
	verifyVCPUChangeRefused(run.microvms, run.snapshotUID, run.ns)

	for generation := 1; generation <= 2; generation++ {
		vm := run.restore(generation)
		defer cleanupOnFailure(t, run.microvms, provider, vm.stateDir, run.ns, vm.uid)

		run.verifyRestored(vm, generation)
	}

	run.verifySeparate()

	// flintlockd removes the state directory when the microvm is deleted, and
	// the JSON files of the snapshot when the snapshot is deleted.
	for _, vm := range run.all() {
		saveArtefacts(t, vm.stateDir)
	}

	saveSnapshotArtefacts(t, run.snapshotDir)

	if params.SkipDelete {
		log.Println("TEST STEP: skipping delete")

		return
	}

	run.deleteAll()
}

// snapshotRun is the state of a run of the snapshot and restore test.
type snapshotRun struct {
	tc        snapshotRestoreCase
	provider  u.Provider
	ns        string
	microvms  v1alpha1.MicroVMClient
	snapshots snapv1.SnapshotServiceClient
	exec      execv1.MicroVMExecClient
	labels    map[string]string

	source     snapshotVM
	sourceSpec *types.MicroVMSpec
	marker     string

	snapshotUID string
	snapshotDir string
	requested   time.Time

	// The lines of the counter of the source when the snapshot was asked for,
	// and when it was ready.
	before     []u.CounterLine
	afterReady []u.CounterLine
	// snapshotLines is how many lines the counter had in the snapshot.
	snapshotLines int

	restored []snapshotVM
	// statuses are the statuses of the source and of the restored microvms,
	// in the order of all.
	statuses []*types.MicroVMStatus
}

func (s *snapshotRun) all() []snapshotVM {
	return append([]snapshotVM{s.source}, s.restored...)
}

func (s *snapshotRun) createSource() {
	log.Printf("TEST STEP: creating the source MicroVM with an overlay root, the guest agent and the kernel %s",
		s.provider.KernelImage)

	spec, err := u.NewOverlayMicroVMSpec(s.provider, "mvm-snap-source", s.ns, u.OverlayRootImage, overlayRWSizeMB)
	Expect(err).NotTo(HaveOccurred())

	spec.AllowGuestAgent = true
	spec.MemoryInMb = snapshotMemoryMB

	created := u.CreateMVMWithSpec(s.microvms, spec)

	s.source = snapshotVM{
		name:     created.Microvm.Spec.Id,
		uid:      *created.Microvm.Spec.Uid,
		stateDir: stateDirOf(s.ns, created.Microvm.Spec.Id, *created.Microvm.Spec.Uid),
	}
}

// prepareSource waits for the source to boot, writes the marker file in its
// guest and starts the counter.
func (s *snapshotRun) prepareSource() {
	s.source.pid = waitForVMM(s.microvms, s.provider, s.source.stateDir, s.source.name, s.ns, s.source.uid)
	waitForBoot(s.provider, s.source.stateDir, s.source.name, s.ns, s.source.pid, bootTimeout)
	verifyOverlayContract(s.source.stateDir, s.provider)

	s.statuses = append(s.statuses, waitForGuestAgent(s.microvms, s.ns, s.source))

	log.Println("TEST STEP: writing the marker file and starting the counter in the source")

	s.marker = "flintlock-e2e source " + s.source.uid
	guestRun(s.exec, s.source, fmt.Sprintf("mkdir -p %s && echo '%s' > %s && sync", guestDir, s.marker, guestMarker))
	guestRun(s.exec, s.source, u.CounterScript(guestCounterUnit, guestCounter))

	waitForCounter(s.exec, s.source, counterLinesBefore)

	// A provider can add to the metadata of a microvm when it creates it, so
	// the spec which the snapshot has is the one of now, not of the request.
	s.sourceSpec = u.GetMVM(s.microvms, s.source.uid).Microvm.Spec
}

func (s *snapshotRun) createSnapshot() {
	const name = "snap0"

	s.before = readCounter(s.exec, s.source)

	log.Printf("TEST STEP: taking the snapshot %s/%s of the source, the counter is at %d", s.ns, name, len(s.before))

	s.requested = time.Now()
	snapshot := u.CreateSnapshot(s.snapshots, s.source.uid, name, s.labels)

	s.snapshotUID = snapshot.GetUid()
	s.snapshotDir = u.SnapshotDir(s.snapshotUID)

	Expect(snapshot.GetName()).To(Equal(name))
	Expect(snapshot.GetNamespace()).To(Equal(s.ns))
	Expect(snapshot.GetSource().GetUid()).To(Equal(s.source.uid))
}

// verifySnapshot waits for the snapshot to be ready, and checks the source
// and the files of the snapshot.
func (s *snapshotRun) verifySnapshot() {
	ready := waitForSnapshotReady(s.snapshots, s.snapshotUID)
	readyAt := time.Now()

	Expect(ready.GetStatus().GetLocalPath()).To(Equal(s.snapshotDir))
	Expect(ready.GetLabels()).To(Equal(s.labels))

	log.Println("TEST STEP: checking that the source runs and that its counter goes on after the snapshot")

	Expect(verifySameVMM(s.source.stateDir, s.provider, s.source.pid)).To(Succeed())
	Expect(u.GetMVM(s.microvms, s.source.uid).Microvm.Status.State).To(Equal(types.MicroVMStatus_CREATED))

	s.afterReady = readCounter(s.exec, s.source)
	lines := waitForCounter(s.exec, s.source, len(s.afterReady)+counterGrowth)

	gap, gapAt := u.MaxCounterGap(lines)
	Expect(gapAt).To(BeNumerically(">=", 0))

	log.Printf("TEST INFO: the longest gap in the counter of the source is %s, between the lines %q and %q",
		gap, lines[gapAt].Text, lines[gapAt+1].Text)

	Expect(gap).To(BeNumerically("<", snapshotMaxPause),
		"the counter of the source stopped for %s, between the lines %q and %q, which is longer than the "+
			"--snapshot-max-pause of %s", gap, lines[gapAt].Text, lines[gapAt+1].Text, snapshotMaxPause)

	log.Printf("TEST STEP: checking the files of the snapshot in %s", s.snapshotDir)
	verifySnapshotFiles(s.snapshotDir, s.source, s.sourceSpec, s.provider, s.requested, readyAt, s.labels)
}

func (s *snapshotRun) restore(generation int) snapshotVM {
	name := fmt.Sprintf("mvm-snap-restore%d", generation)

	log.Printf("TEST STEP: restoring the snapshot as %s/%s, the restore generation %d", s.ns, name, generation)

	source := &types.SnapshotSource{Snapshot: u.SnapshotRefOf(s.snapshotUID), Name: name}

	res, err := u.RestoreMVM(s.microvms, source, nil)
	Expect(err).NotTo(HaveOccurred(), "restoring the snapshot %s as %s", s.snapshotUID, name)

	return snapshotVM{
		name:     res.Microvm.Spec.Id,
		uid:      *res.Microvm.Spec.Uid,
		stateDir: stateDirOf(s.ns, res.Microvm.Spec.Id, *res.Microvm.Spec.Uid),
	}
}

// verifyRestored checks that a restored microvm runs, has the restore
// markers, and that its guest goes on from the snapshot.
func (s *snapshotRun) verifyRestored(vm snapshotVM, generation int) {
	Expect(vm.name).To(Equal(fmt.Sprintf("mvm-snap-restore%d", generation)))
	Expect(vm.uid).NotTo(Equal(s.source.uid))

	vm.pid = waitForVMM(s.microvms, s.provider, vm.stateDir, vm.name, s.ns, vm.uid)

	s.restored = append(s.restored, vm)
	s.statuses = append(s.statuses, waitForGuestAgent(s.microvms, s.ns, vm))

	verifyRestoreMarkers(s.microvms, vm, s.sourceSpec.GetMetadata(), s.snapshotUID, generation)

	log.Printf("TEST STEP: checking the guest of %s", vm)

	marker := guestRun(s.exec, vm, "cat "+guestMarker)
	if marker != s.marker+"\n" {
		logGuestFile(s.exec, vm, guestMarker)
	}

	Expect(marker).To(Equal(s.marker+"\n"),
		"the marker file of the restored guest is not the one which was written in the source")

	lines := verifyCounterResumes(s.exec, s.source, vm)
	if s.snapshotLines == 0 {
		s.snapshotLines = lines
	}

	Expect(lines).To(Equal(s.snapshotLines),
		"the counter of %s resumed after line %d, the counter of the first restore after line %d",
		vm, lines, s.snapshotLines)
	Expect(lines).To(BeNumerically(">=", len(s.before)),
		"the counter of %s resumed after line %d, it was at %d before the snapshot was asked for",
		vm, lines, len(s.before))
	Expect(lines).To(BeNumerically("<=", len(s.afterReady)),
		"the counter of %s resumed after line %d, it was at %d when the snapshot was ready",
		vm, lines, len(s.afterReady))
}

// verifySeparate checks that the microvms have their own disks, TAPs and
// sockets, and that the snapshot has a reference for each restored microvm
// which uses its files.
func (s *snapshotRun) verifySeparate() {
	log.Println("TEST STEP: checking that each of the MicroVMs has a writable disk, a TAP and sockets of its own")
	verifyRestoredAreSeparate(s.exec, s.tc, s.all(), s.statuses, s.snapshotDir)

	refs, err := u.DirEntries(filepath.Join(s.snapshotDir, u.SnapshotRefsDir))
	Expect(err).NotTo(HaveOccurred())

	log.Printf("TEST INFO: %s/%s has the references: %s", s.snapshotDir, u.SnapshotRefsDir, strings.Join(refs, ", "))

	if len(s.tc.retainedFiles) > 0 {
		Expect(refs).To(ConsistOf(s.restored[0].uid, s.restored[1].uid),
			"the snapshot does not have a reference for each of the restored MicroVMs")
	}
}

// deleteAll deletes the snapshot while the restored microvms run, then the
// microvms, and checks that the files of the snapshot go with the last
// restored microvm, and that nothing is left.
func (s *snapshotRun) deleteAll() {
	log.Println("TEST STEP: deleting the snapshot while the restored MicroVMs run")
	Expect(u.DeleteSnapshot(s.snapshots, s.snapshotUID)).To(Succeed())

	verifySnapshotDeleted(s.snapshots, s.ns, s.snapshotUID)
	verifyRetainedFiles(s.snapshotDir, s.tc.retainedFiles)

	for _, vm := range s.restored {
		Expect(verifySameVMM(vm.stateDir, s.provider, vm.pid)).To(Succeed())
		waitForCounter(s.exec, vm, len(readCounter(s.exec, vm))+1)
	}

	first, last := s.restored[0], s.restored[1]

	log.Printf("TEST STEP: deleting %s, the snapshot has to keep its files for %s", first, last)
	deleteSnapshotVM(s.microvms, s.ns, first)

	if len(s.tc.retainedFiles) > 0 {
		verifyRetainedFiles(s.snapshotDir, s.tc.retainedFiles)

		refs, err := u.DirEntries(filepath.Join(s.snapshotDir, u.SnapshotRefsDir))
		Expect(err).NotTo(HaveOccurred())
		Expect(refs).To(Equal([]string{last.uid}), "the references after %s was deleted", first)
	}

	log.Printf("TEST STEP: deleting %s, the last MicroVM which uses the snapshot", last)
	deleteSnapshotVM(s.microvms, s.ns, last)

	Eventually(func() error {
		return u.Gone(s.snapshotDir)
	}, "30s", "1s").Should(Succeed(), "the snapshot directory is still there after the last restored MicroVM was deleted")

	log.Printf("TEST STEP: deleting the source %s", s.source)
	deleteSnapshotVM(s.microvms, s.ns, s.source)

	log.Println("TEST STEP: checking that nothing of the MicroVMs and of the snapshot is left")
	verifyNothingLeft(s.microvms, s.snapshots, s.ns, s.all(), s.statuses, s.snapshotUID)
}

func stateDirOf(ns, name, uid string) string {
	return filepath.Join(u.StateRootDir, "vm", ns, name, uid)
}

func socketDirOf(uid string) string {
	return filepath.Join(u.SocketRootDir, uid)
}

// waitForGuestAgent waits until a microvm is CREATED and its guest agent
// answers, and returns its status.
func waitForGuestAgent(client v1alpha1.MicroVMClient, ns string, vm snapshotVM) *types.MicroVMStatus {
	log.Printf("TEST STEP: waiting for the guest agent of MicroVM %s/%s", ns, vm)

	start := time.Now()

	var current *types.MicroVMStatus

	Eventually(func(g Gomega) {
		current = u.GetMVM(client, vm.uid).Microvm.Status

		g.Expect(current.GetState()).To(Equal(types.MicroVMStatus_CREATED), "conditions: %s", conditions(current))

		ready := u.Condition(current, "GuestAgentReady")
		g.Expect(ready).NotTo(BeNil(), "conditions: %s", conditions(current))
		g.Expect(ready.GetStatus()).To(Equal(types.MicroVMCondition_TRUE), "conditions: %s", conditions(current))
	}, guestAgentTimeout, snapshotPolling).Should(Succeed(), "MicroVM %s/%s", ns, vm)

	log.Printf("TEST INFO: the guest agent of MicroVM %s/%s is ready, it took %s, conditions: %s",
		ns, vm, time.Since(start).Round(time.Millisecond), conditions(current))

	return current
}

func conditions(status *types.MicroVMStatus) string {
	parts := []string{}

	for _, cond := range status.GetConditions() {
		parts = append(parts, fmt.Sprintf("%s=%s (%s: %s)",
			cond.GetType(), cond.GetStatus(), cond.GetReason(), cond.GetMessage()))
	}

	return "[" + strings.Join(parts, ", ") + "]"
}

// guestRun runs a script in the guest of a microvm and returns its stdout.
// The script has to succeed.
func guestRun(client execv1.MicroVMExecClient, vm snapshotVM, script string) string {
	result, err := u.ExecShell(client, vm.uid, script)
	Expect(err).NotTo(HaveOccurred(), "running %q in %s", script, vm)
	Expect(result.Err()).NotTo(HaveOccurred(), "running %q in %s", script, vm)

	return result.Stdout
}

// logGuestFile logs what the guest reports about a file, so that a read that
// differs from the expected content can be told apart from lost exec output.
func logGuestFile(client execv1.MicroVMExecClient, vm snapshotVM, path string) {
	for _, script := range []string{
		"cat " + path,
		"stat " + path,
		"wc -c " + path + " && md5sum " + path,
		"findmnt -T " + path,
		"dmesg | tail -n 40",
	} {
		result, err := u.ExecShell(client, vm.uid, script)
		log.Printf("TEST INFO: %s in %s: stdout %q, stderr %q, err %v, exec err %v",
			script, vm, result.Stdout, result.Stderr, result.Err(), err)
	}
}

func readCounter(client execv1.MicroVMExecClient, vm snapshotVM) []u.CounterLine {
	content := guestRun(client, vm, "cat "+guestCounter)

	lines, err := u.ParseCounter(content)
	Expect(err).NotTo(HaveOccurred(), "the counter of %s", vm)

	return lines
}

// waitForCounter waits until the counter of a guest has at least n lines,
// and returns its lines.
func waitForCounter(client execv1.MicroVMExecClient, vm snapshotVM, n int) []u.CounterLine {
	var lines []u.CounterLine

	Eventually(func(g Gomega) {
		result, err := u.ExecShell(client, vm.uid, "cat "+guestCounter)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(result.Err()).NotTo(HaveOccurred())

		lines, err = u.ParseCounter(result.Stdout)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(len(lines)).To(BeNumerically(">=", n), "the counter of %s", vm)
	}, "30s", "1s").Should(Succeed())

	log.Printf("TEST INFO: the counter of %s has %d lines, the last is %q", vm, len(lines), lines[len(lines)-1].Text)

	return lines
}

// waitForSnapshotReady waits until a snapshot is READY. The phases which it
// goes through are logged.
func waitForSnapshotReady(client snapv1.SnapshotServiceClient, uid string) *types.Snapshot {
	log.Printf("TEST STEP: waiting for the snapshot %s to be READY", uid)

	start := time.Now()
	lastPhase := ""

	var current *types.Snapshot

	Eventually(func(g Gomega) error {
		snapshot, err := u.GetSnapshot(client, uid)
		g.Expect(err).NotTo(HaveOccurred())

		current = snapshot

		phase := snapshot.GetStatus().GetPhase()
		if phase.String() != lastPhase {
			log.Printf("TEST INFO: the snapshot %s is %s after %s", uid, phase, time.Since(start).Round(time.Millisecond))
			lastPhase = phase.String()
		}

		if phase == types.SnapshotStatus_FAILED {
			return StopTrying("the snapshot failed: " + snapshot.GetStatus().GetError())
		}

		g.Expect(phase).To(Equal(types.SnapshotStatus_READY))

		return nil
	}, snapshotReadyTimeout, snapshotPolling).Should(Succeed())

	return current
}

// verifySnapshotFiles checks the files which the snapshot of the source wrote
// to its directory.
func verifySnapshotFiles(
	dir string,
	source snapshotVM,
	sourceSpec *types.MicroVMSpec,
	provider u.Provider,
	requested, readyAt time.Time,
	labels map[string]string,
) {
	names, err := u.DirEntries(dir)
	Expect(err).NotTo(HaveOccurred())

	log.Printf("TEST INFO: %s has: %s", dir, strings.Join(names, ", "))

	modes, err := u.FileModes(dir)
	Expect(err).NotTo(HaveOccurred())

	for _, name := range []string{
		models.SnapshotDescriptorFilename, models.SnapshotSpecFilename,
		models.SnapshotVMStateFile, models.SnapshotMemoryFile, u.SnapshotRWDisk,
	} {
		Expect(modes).To(HaveKey(name), "the snapshot has no file %s, it has: %s", name, strings.Join(names, ", "))
	}

	wrongModes := []string{}

	for name, mode := range modes {
		if mode != 0o600 {
			wrongModes = append(wrongModes, fmt.Sprintf("%s %s", name, mode))
		}
	}

	Expect(wrongModes).To(BeEmpty(), "the files of the snapshot have to be 0600")

	desc, fields, err := u.ReadSnapshotDescriptor(dir)
	Expect(err).NotTo(HaveOccurred())

	log.Printf("TEST INFO: %s has the fields: %s", models.SnapshotDescriptorFilename, strings.Join(keysOf(fields), ", "))

	Expect(fields).NotTo(HaveKey("metadata"), "the descriptor has the metadata of the source")

	content, err := os.ReadFile(filepath.Join(dir, models.SnapshotDescriptorFilename))
	Expect(err).NotTo(HaveOccurred())

	for key, value := range sourceSpec.GetMetadata() {
		if value == "" {
			continue
		}

		Expect(string(content)).NotTo(ContainSubstring(value), "the descriptor has the metadata %s of the source", key)
	}

	verifyDescriptor(desc, source, sourceSpec, provider, requested, readyAt, labels)

	snapshotSpec, err := u.ReadSnapshotSpec(dir)
	Expect(err).NotTo(HaveOccurred())
	Expect(snapshotSpec.Metadata).To(Equal(sourceSpec.GetMetadata()),
		"%s does not have the metadata of the source", models.SnapshotSpecFilename)
	Expect(snapshotSpec.AllowGuestAgent).To(BeTrue())
	Expect(snapshotSpec.VCPU).To(Equal(int64(sourceSpec.GetVcpu())))
	Expect(snapshotSpec.MemoryInMb).To(Equal(int64(sourceSpec.GetMemoryInMb())))

	sameBase, err := u.SameInode(filepath.Join(dir, u.SnapshotBaseLink), u.OverlayVolumesOf(source.stateDir).BaseLink)
	Expect(err).NotTo(HaveOccurred())
	Expect(sameBase).To(BeTrue(), "the base of the snapshot is not the base of the source")
}

func verifyDescriptor(
	desc *models.SnapshotDescriptor,
	source snapshotVM,
	sourceSpec *types.MicroVMSpec,
	provider u.Provider,
	requested, readyAt time.Time,
	labels map[string]string,
) {
	_, vmmVersion, err := provider.BinaryVersion()
	Expect(err).NotTo(HaveOccurred())

	var uts unix.Utsname
	Expect(unix.Uname(&uts)).To(Succeed())

	base, err := filepath.EvalSymlinks(u.OverlayVolumesOf(source.stateDir).BaseLink)
	Expect(err).NotTo(HaveOccurred())

	log.Printf("TEST INFO: the descriptor is %+v", *desc)

	Expect(desc.FormatVersion).To(Equal(models.SnapshotDescriptorFormatVersion))
	Expect(desc.VMMType).To(Equal(provider.Name))
	Expect(strings.Fields(vmmVersion)).To(ContainElement(desc.VMMVersion),
		"the VMM version of the descriptor is not in %q", vmmVersion)
	Expect(desc.FlintlockVersion).NotTo(BeEmpty())
	Expect(desc.Arch).To(Equal(runtime.GOARCH))
	Expect(desc.CPUVendor).NotTo(BeEmpty())
	Expect(desc.CPUModel).NotTo(BeEmpty())
	Expect(desc.HostKernel).To(Equal(unix.ByteSliceToString(uts.Release[:])))
	Expect(desc.VCPUs).To(Equal(int64(sourceSpec.GetVcpu())))
	Expect(desc.MemoryMB).To(Equal(int64(sourceSpec.GetMemoryInMb())))

	if provider.Name == models.VMMTypeFirecracker {
		Expect(desc.FCSnapshotVersion).NotTo(BeEmpty())
	}

	if runtime.GOARCH == "amd64" {
		Expect(desc.CPUMicrocode).NotTo(BeEmpty())
	}

	Expect(desc.KernelImage.Reference).To(Equal(provider.KernelImage))
	Expect(desc.KernelImage.Digest).To(HavePrefix("sha256:"))
	Expect(desc.InitrdImage).NotTo(BeNil(), "the descriptor has no initrd image")
	Expect(desc.InitrdImage.Reference).To(Equal(params.OverlayInitrdImage))
	Expect(desc.InitrdImage.Digest).To(HavePrefix("sha256:"))

	Expect(desc.Volumes).To(HaveLen(1))
	Expect(desc.Volumes[0].SizeBytes).To(Equal(int64(overlayRWSizeMB) * 1024 * 1024))
	Expect(desc.Volumes[0].BlockSize).To(BeNumerically(">", 0))
	Expect(desc.Volumes[0].Filesystem).To(Equal("ext4"))
	Expect(desc.Volumes[0].BaseDigest).To(Equal("sha256:"+strings.TrimSuffix(filepath.Base(base), ".img")),
		"the base of the source is %s", base)

	Expect(desc.CreatedAt).To(BeTemporally(">=", requested.Add(-time.Second)))
	Expect(desc.CreatedAt).To(BeTemporally("<=", readyAt.Add(time.Second)))
	Expect(desc.Source).To(Equal(models.SnapshotOrigin{
		UID:       source.uid,
		Name:      source.name,
		Namespace: sourceSpec.GetNamespace(),
	}))
	Expect(desc.Labels).To(Equal(labels))
}

func keysOf[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}

	return keys
}

// verifyVCPUChangeRefused asks for a restore with a spec which has another
// vCPU count. The API takes either a spec or a snapshot source, so it refuses
// the request before the snapshot is read.
func verifyVCPUChangeRefused(client v1alpha1.MicroVMClient, snapshotUID, ns string) {
	const name = "mvm-snap-vcpu"

	_, err := u.RestoreMVM(client,
		&types.SnapshotSource{Snapshot: u.SnapshotRefOf(snapshotUID), Name: name},
		&types.MicroVMSpec{Vcpu: 4},
	)
	Expect(err).To(HaveOccurred(), "the restore with 4 vCPUs was not refused")
	Expect(status.Code(err)).To(Equal(codes.InvalidArgument), "the restore with 4 vCPUs was refused with: %s", err)

	log.Printf("TEST INFO: the restore with 4 vCPUs was refused with: %s", err)

	Expect(u.ListMVMs(client, ns, ptr.String(name)).Microvm).To(BeEmpty())
}

// verifyRestoreMarkers checks that a restored microvm has the metadata of
// the source and the restore markers.
func verifyRestoreMarkers(
	client v1alpha1.MicroVMClient,
	vm snapshotVM,
	sourceMetadata map[string]string,
	snapshotUID string,
	generation int,
) {
	metadata := u.GetMVM(client, vm.uid).Microvm.Spec.GetMetadata()

	markers := map[string]string{
		models.MetadataSnapshotUID:       snapshotUID,
		models.MetadataRestoreUID:        vm.uid,
		models.MetadataRestoreGeneration: strconv.Itoa(generation),
	}

	for key, expected := range markers {
		Expect(metadata).To(HaveKey(key), "the metadata of %s has the keys %v", vm, keysOf(metadata))

		value, err := base64.StdEncoding.DecodeString(metadata[key])
		Expect(err).NotTo(HaveOccurred(), "the value of %s in the metadata of %s is %q", key, vm, metadata[key])
		Expect(string(value)).To(Equal(expected), "the value of %s in the metadata of %s", key, vm)
	}

	for key, value := range sourceMetadata {
		Expect(metadata).To(HaveKeyWithValue(key, value), "the metadata %s of the source", key)
	}

	Expect(metadata).To(HaveLen(len(sourceMetadata)+len(markers)),
		"the metadata of %s has the keys %v", vm, keysOf(metadata))

	log.Printf("TEST INFO: %s has the restore markers %v", vm, markers)
}

// verifyCounterResumes checks that the counter of a restored guest goes on
// from where the counter of the source was when the snapshot was taken, and
// that the uptime of the guest does too. It returns how many lines the
// counter had then.
func verifyCounterResumes(client execv1.MicroVMExecClient, source, vm snapshotVM) int {
	sourceLines := readCounter(client, source)
	restoredLines := readCounter(client, vm)

	common := u.CommonCounterLines(sourceLines, restoredLines)
	Expect(common).To(BeNumerically(">", 0),
		"the counter of %s has nothing of the counter of the source: its first lines are %v", vm, head(restoredLines))

	snapshotLine := restoredLines[common-1]

	restoredLines = waitForCounter(client, vm, common+counterGrowth)

	log.Printf("TEST INFO: the counter of %s went on after the line %q of the source with %q",
		vm, snapshotLine.Text, restoredLines[common].Text)

	uptime, err := u.ParseUptime(guestRun(client, vm, "cat /proc/uptime"))
	Expect(err).NotTo(HaveOccurred())

	sourceUptime, err := u.ParseUptime(guestRun(client, source, "cat /proc/uptime"))
	Expect(err).NotTo(HaveOccurred())

	log.Printf("TEST INFO: the uptime of %s is %.2fs, it was %.2fs at the snapshot, the uptime of the source is %.2fs",
		vm, uptime, snapshotLine.Uptime, sourceUptime)

	Expect(uptime).To(BeNumerically(">=", snapshotLine.Uptime),
		"the uptime of %s does not go on from the uptime of the source at the snapshot", vm)

	return common
}

func head(lines []u.CounterLine) []string {
	texts := []string{}

	for i := 0; i < len(lines) && i < 3; i++ {
		texts = append(texts, lines[i].Text)
	}

	return texts
}

// verifyRestoredAreSeparate checks that the microvms share the base of their
// root and nothing else: not the writable disk, the TAP or the sockets, and
// that a file which a guest writes is only in its own root.
func verifyRestoredAreSeparate(
	client execv1.MicroVMExecClient,
	tc snapshotRestoreCase,
	vms []snapshotVM,
	statuses []*types.MicroVMStatus,
	snapshotDir string,
) {
	disks := map[string]string{}
	devices := map[string]string{}
	sockets := map[string]string{}

	for i, vm := range vms {
		volumes := u.OverlayVolumesOf(vm.stateDir)

		sameBase, err := u.SameInode(volumes.BaseLink, filepath.Join(snapshotDir, u.SnapshotBaseLink))
		Expect(err).NotTo(HaveOccurred())
		Expect(sameBase).To(BeTrue(), "%s and the snapshot do not resolve to the same base", volumes.BaseLink)

		for other, disk := range disks {
			same, err := u.SameInode(volumes.RWDisk, disk)
			Expect(err).NotTo(HaveOccurred())
			Expect(same).To(BeFalse(), "%s has the writable disk of %s: %s", vm, other, disk)
		}

		disks[vm.String()] = volumes.RWDisk

		hostDevices := u.HostDevices(statuses[i])
		Expect(hostDevices).NotTo(BeEmpty(), "%s has no network interface", vm)

		for _, device := range hostDevices {
			Expect(devices).NotTo(HaveKey(device), "%s has the host device %s of %s", vm, device, devices[device])
			Expect(u.LinkExists(device)).To(BeTrue(), "the host device %s of %s does not exist", device, vm)

			devices[device] = vm.String()
		}

		vmSockets := []string{filepath.Join(socketDirOf(vm.uid), tc.apiSocketFile), statuses[i].GetVsockPath()}

		for _, socket := range vmSockets {
			Expect(sockets).NotTo(HaveKey(socket), "%s has the socket %s of %s", vm, socket, sockets[socket])
			Expect(u.IsSocket(socket)).To(Succeed())

			sockets[socket] = vm.String()
		}

		log.Printf("TEST INFO: %s has the writable disk %s, the host devices %v and the sockets %v",
			vm, volumes.RWDisk, hostDevices, vmSockets)

		guestRun(client, vm, fmt.Sprintf("echo '%s' > %s/%s%s && sync", vm.uid, guestDir, guestOwnPrefix, vm.name))
	}

	for _, vm := range vms {
		listing := guestRun(client, vm, "ls -1 "+guestDir)

		own := []string{}

		for _, name := range strings.Fields(listing) {
			if strings.HasPrefix(name, guestOwnPrefix) {
				own = append(own, name)
			}
		}

		Expect(own).To(Equal([]string{guestOwnPrefix + vm.name}),
			"the guest of %s has the files which the guests wrote: %v", vm, own)
	}
}

// verifySnapshotDeleted checks that flintlockd does not know a snapshot.
func verifySnapshotDeleted(client snapv1.SnapshotServiceClient, ns, uid string) {
	for _, listed := range u.ListSnapshots(client, ns) {
		Expect(listed.GetUid()).NotTo(Equal(uid), "the deleted snapshot is in the list of namespace %s", ns)
	}

	_, err := u.GetSnapshot(client, uid)
	Expect(status.Code(err)).To(Equal(codes.NotFound), "getting the deleted snapshot %s: %v", uid, err)
}

// verifyRetainedFiles checks that the directory of a deleted snapshot still
// has the files which a restored microvm uses.
func verifyRetainedFiles(dir string, retained []string) {
	if len(retained) == 0 {
		Expect(u.Gone(dir)).To(Succeed(), "the snapshot is deleted and no restored MicroVM uses its files")

		return
	}

	names, err := u.DirEntries(dir)
	Expect(err).NotTo(HaveOccurred(), "the snapshot is deleted and a restored MicroVM uses its files")

	log.Printf("TEST INFO: %s has: %s", dir, strings.Join(names, ", "))

	for _, name := range retained {
		Expect(names).To(ContainElement(name), "the deleted snapshot has to keep %s while a restored MicroVM uses it", name)
	}
}

// deleteSnapshotVM deletes a microvm and waits until it is gone.
func deleteSnapshotVM(client v1alpha1.MicroVMClient, ns string, vm snapshotVM) {
	Expect(u.DeleteMVM(client, vm.uid)).To(Succeed())
	Expect(u.WaitForDeleted(client, ns, vm.uid, vm.stateDir, snapshotDeleteWait, time.Second)).To(Succeed())

	Eventually(func() bool {
		return u.PidRunning(vm.pid)
	}, "30s", "1s").Should(BeFalse(), "the VMM of %s pid %d is still running", vm, vm.pid)

	log.Printf("TEST INFO: %s is deleted", vm)
}

// verifyNothingLeft checks that the host has nothing of the microvms and of
// the snapshot: no state directory, socket directory, TAP or file which has
// one of their uids in its path.
func verifyNothingLeft(
	microvms v1alpha1.MicroVMClient,
	snapshots snapv1.SnapshotServiceClient,
	ns string,
	vms []snapshotVM,
	statuses []*types.MicroVMStatus,
	snapshotUID string,
) {
	uids := []string{snapshotUID}

	for i, vm := range vms {
		uids = append(uids, vm.uid)

		Expect(u.Gone(vm.stateDir)).To(Succeed())
		Expect(u.Gone(socketDirOf(vm.uid))).To(Succeed())

		for _, device := range u.HostDevices(statuses[i]) {
			Expect(u.LinkExists(device)).To(BeFalse(), "the host device %s of %s is still there", device, vm)
		}
	}

	Expect(u.Gone(u.SnapshotDir(snapshotUID))).To(Succeed())

	for _, root := range []string{u.StateRootDir, u.SocketRootDir} {
		left, err := u.PathsWith(root, uids...)
		Expect(err).NotTo(HaveOccurred())
		Expect(left).To(BeEmpty(), "%s has files of the deleted MicroVMs or of the deleted snapshot", root)
	}

	Expect(u.ListMVMs(microvms, ns, nil).Microvm).To(BeEmpty())
	Expect(u.ListSnapshots(snapshots, ns)).To(BeEmpty())

	log.Printf("TEST INFO: nothing of %s is left in %s, %s or the network devices of the host",
		strings.Join(uids, ", "), u.StateRootDir, u.SocketRootDir)
}

// cleanupSnapshotOnFailure is for a defer. If the test has failed, it saves
// the JSON files of the snapshot and deletes it.
func cleanupSnapshotOnFailure(t *testing.T, client snapv1.SnapshotServiceClient, dir, uid string) {
	t.Helper()

	if !t.Failed() {
		return
	}

	if names, err := u.DirEntries(dir); err == nil {
		log.Printf("TEST INFO: %s has: %s", dir, strings.Join(names, ", "))
	}

	saveSnapshotArtefacts(t, dir)

	if params.SkipDelete || params.SkipTeardown {
		return
	}

	// The test has failed already, so there are no assertions from here.
	if err := u.DeleteSnapshot(client, uid); err != nil {
		log.Printf("TEST INFO: deleting the snapshot %s: %s", uid, err)

		return
	}

	log.Printf("TEST INFO: the snapshot %s is deleted", uid)
}

// saveSnapshotArtefacts copies the JSON files of a snapshot to the directory
// of the artefacts, if the tests were given one.
func saveSnapshotArtefacts(t *testing.T, dir string) {
	t.Helper()

	if params.ArtefactsDir == "" {
		return
	}

	dest := filepath.Join(params.ArtefactsDir, t.Name(), "snapshots", filepath.Base(dir))

	saved, err := u.SaveSnapshotFiles(dir, dest)
	if err != nil {
		log.Printf("TEST INFO: not all of the files of %s are saved: %s", dir, err)
	}

	log.Printf("TEST INFO: saved %d files of %s to %s: %s", len(saved), dir, dest, strings.Join(saved, ", "))
}
