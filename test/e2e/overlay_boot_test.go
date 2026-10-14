//go:build e2e
// +build e2e

package e2e_test

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	. "github.com/onsi/gomega"

	"github.com/liquidmetal-dev/flintlock/api/services/microvm/v1alpha1"
	"github.com/liquidmetal-dev/flintlock/api/types"
	u "github.com/liquidmetal-dev/flintlock/test/e2e/utils"
)

const (
	// overlayRWSizeMB is the size of the writable disk of the microvms with an
	// overlay root. It names the template which the disks are cloned from.
	overlayRWSizeMB int32 = 2048

	// The first microvm of an image waits for flintlockd to pull the image and
	// to build the base from it.
	overlayCreateTimeout = "300s"

	// The guest writes to its writable disk while it boots. The writes reach
	// the disk when the guest commits its journal, which is a few seconds
	// after the writes.
	overlayWriteTimeout = "60s"
	overlayWritePolling = "2s"
)

// overlayUpperDirectories are the directories of the root which the guest
// writes to when it boots, and which overlayfs copies up to the writable
// disk. One of them is enough to show that the guest writes to the overlay.
var overlayUpperDirectories = []string{"etc", "var", "tmp"}

// TestE2EOverlayBoot boots the ubuntu image from an overlay root with each of
// the kernels of the overlay boot test. The evidence for each check is on the
// host or on the console of the guest. The state which flintlockd reports is
// only logged, it is not evidence (#1263).
//
// The test needs the initrd image of -overlay.initrd.image, a state directory
// on a filesystem with reflinks, and the pinned mkfs.erofs. It is skipped
// when the initrd image is not set.
func TestE2EOverlayBoot(t *testing.T) {
	RegisterTestingT(t)

	if environmentLeftRunning {
		t.Skip("a previous test left its environment running, use -run to run this test on its own")
	}

	if params.OverlayInitrdImage == "" {
		t.Skip("the overlay boot test needs the initrd image of -overlay.initrd.image")
	}

	environmentLeftRunning = params.SkipTeardown || params.SkipDelete

	r := u.NewRunner(params)
	defer func() {
		log.Println("TEST STEP: cleaning up running processes")
		r.Teardown()
	}()
	log.Println("TEST STEP: performing setup, starting flintlockd server")
	flintlockClient := r.Setup()

	for _, kernel := range u.OverlayKernels() {
		t.Run(kernel.Name(), func(subT *testing.T) {
			if !selected(kernel.Provider) {
				subT.Skipf("the %s provider is not one of the providers of -providers", kernel.Provider.Name)
			}

			// The helpers of the tests assert with the global gomega. A failure
			// has to fail the subtest which is running: if it failed the test of
			// the subtest, the test binary would panic and skip the teardown.
			RegisterTestingT(subT)
			defer RegisterTestingT(t)

			log.Printf("TEST STEP: booting an overlay root with the %s provider and the kernel %s",
				kernel.Provider.Name, kernel.Provider.KernelImage)
			runOverlayBoot(subT, flintlockClient, kernel)
		})
	}
}

func runOverlayBoot(t *testing.T, flintlockClient v1alpha1.MicroVMClient, kernel u.OverlayKernel) {
	t.Helper()

	const (
		mvmID       = "mvm-overlay0"
		secondMvmID = "mvm-overlay1"
		statePath   = u.StateRootDir + "/vm/%s/%s/%s"
	)

	var (
		provider = kernel.Provider
		mvmNS    = kernel.Namespace()
		template = u.OverlayTemplatePath(u.StateRootDir, overlayRWSizeMB)
	)

	log.Printf("TEST STEP: creating a MicroVM with the root volume %s in overlay mode", u.OverlayRootImage)
	created := u.CreateOverlayMVM(flintlockClient, provider, mvmID, mvmNS, u.OverlayRootImage, overlayRWSizeMB)

	// The cleanup is set up before anything is checked, so that the microvm is
	// also deleted when the first check fails.
	firstMicroVMPath := fmt.Sprintf(statePath, mvmNS, mvmID, *created.Microvm.Spec.Uid)
	defer cleanupOnFailure(t, flintlockClient, provider, firstMicroVMPath, mvmNS, *created.Microvm.Spec.Uid)

	Expect(created.Microvm.Spec.Id).To(Equal(mvmID))
	Expect(created.Microvm.Spec.RootVolume.Mode).To(Equal(types.VolumeMode_OVERLAY))
	Expect(created.Microvm.Spec.Initrd).NotTo(BeNil(), "flintlockd did not give the MicroVM the default initrd")
	Expect(created.Microvm.Spec.Initrd.Image).To(Equal(params.OverlayInitrdImage))

	log.Printf("TEST INFO: MicroVM %s/%s has uid %s and state directory %s",
		mvmNS, mvmID, *created.Microvm.Spec.Uid, firstMicroVMPath)

	firstPid := waitForVMM(flintlockClient, provider, firstMicroVMPath, mvmID, mvmNS, *created.Microvm.Spec.Uid)

	// The marker of cloud-init shows that cloud-init has run all of its stages
	// on the overlay root.
	waitForBoot(provider, firstMicroVMPath, mvmID, mvmNS, firstPid)
	logResizefs(firstMicroVMPath, provider)

	log.Println("TEST STEP: verifying that the root of the guest is the overlay")
	verifyOverlayContract(firstMicroVMPath, provider)

	firstVolumes := u.OverlayVolumesOf(firstMicroVMPath)
	waitForWrites(firstVolumes.RWDisk, template)

	log.Println("TEST STEP: verifying the base of the MicroVM")
	base, err := u.VerifyOverlayBase(u.StateRootDir, firstVolumes.BaseLink)
	Expect(err).NotTo(HaveOccurred())

	log.Printf("TEST INFO: %s is a link to the base %s, its sha256 is its name and it was modified at %s",
		firstVolumes.BaseLink, base.Path, base.ModTime.Format(time.RFC3339Nano))

	log.Println("TEST STEP: creating a second MicroVM from the same image")
	createdSecond := u.CreateOverlayMVM(flintlockClient, provider, secondMvmID, mvmNS, u.OverlayRootImage, overlayRWSizeMB)

	secondMicroVMPath := fmt.Sprintf(statePath, mvmNS, secondMvmID, *createdSecond.Microvm.Spec.Uid)
	defer cleanupOnFailure(t, flintlockClient, provider, secondMicroVMPath, mvmNS, *createdSecond.Microvm.Spec.Uid)

	Expect(createdSecond.Microvm.Spec.Id).To(Equal(secondMvmID))

	log.Printf("TEST INFO: MicroVM %s/%s has uid %s and state directory %s",
		mvmNS, secondMvmID, *createdSecond.Microvm.Spec.Uid, secondMicroVMPath)

	secondPid := waitForVMM(flintlockClient, provider, secondMicroVMPath, secondMvmID, mvmNS,
		*createdSecond.Microvm.Spec.Uid)

	waitForBoot(provider, secondMicroVMPath, secondMvmID, mvmNS, secondPid)

	log.Println("TEST STEP: verifying that the second MicroVM reuses the base and has its own writable disk")
	verifyOverlayContract(secondMicroVMPath, provider)

	secondVolumes := u.OverlayVolumesOf(secondMicroVMPath)
	waitForWrites(secondVolumes.RWDisk, template)

	sameBase, err := u.SameInode(firstVolumes.BaseLink, secondVolumes.BaseLink)
	Expect(err).NotTo(HaveOccurred())
	Expect(sameBase).To(BeTrue(), "%s and %s do not resolve to the same inode", firstVolumes.BaseLink, secondVolumes.BaseLink)

	sameDisk, err := u.SameInode(firstVolumes.RWDisk, secondVolumes.RWDisk)
	Expect(err).NotTo(HaveOccurred())
	Expect(sameDisk).To(BeFalse(), "%s and %s are the same inode", firstVolumes.RWDisk, secondVolumes.RWDisk)

	Expect(base.Verify()).To(Succeed(), "the base has changed while the MicroVMs ran")

	log.Printf("TEST INFO: %s resolves to the base %s, which is unchanged", secondVolumes.BaseLink, base.Path)

	// flintlockd removes the state directory when the microvm is deleted.
	saveArtefacts(t, firstMicroVMPath)
	saveArtefacts(t, secondMicroVMPath)

	if params.SkipDelete {
		log.Println("TEST STEP: skipping delete")

		return
	}

	log.Println("TEST STEP: deleting the MicroVMs")
	Expect(u.DeleteMVM(flintlockClient, *created.Microvm.Spec.Uid)).To(Succeed())
	Expect(u.DeleteMVM(flintlockClient, *createdSecond.Microvm.Spec.Uid)).To(Succeed())

	log.Println("TEST STEP: waiting for the MicroVMs to be deleted")
	deleteStart := time.Now()

	Eventually(func(g Gomega) error {
		g.Expect(firstMicroVMPath).ToNot(BeAnExistingFile())
		g.Expect(secondMicroVMPath).ToNot(BeAnExistingFile())
		g.Expect(u.PidRunning(firstPid)).To(BeFalse())
		g.Expect(u.PidRunning(secondPid)).To(BeFalse())

		res := u.ListMVMs(flintlockClient, mvmNS, nil)
		g.Expect(res.Microvm).To(BeEmpty())

		return nil
	}, "120s").Should(Succeed())

	log.Printf("TEST INFO: MicroVMs %s/%s and %s/%s are deleted, it took %s",
		mvmNS, mvmID, mvmNS, secondMvmID, time.Since(deleteStart).Round(time.Millisecond))

	log.Println("TEST STEP: verifying that the writable disks and the links are gone, and that the base is kept")

	for _, volumes := range []u.OverlayVolumes{firstVolumes, secondVolumes} {
		Expect(u.Gone(volumes.RWDisk)).To(Succeed())
		Expect(u.Gone(volumes.BaseLink)).To(Succeed())
	}

	Expect(base.Verify()).To(Succeed(), "the base is not kept as it was after the delete")

	log.Printf("TEST INFO: the base %s is kept, its sha256 is its name and it was modified at %s",
		base.Path, base.ModTime.Format(time.RFC3339Nano))
}

// waitForVMM waits for flintlockd to start the VMM of a microvm, and returns
// its pid. The state of the microvm is logged when it changes, which shows
// how far flintlockd has got when the VMM does not start.
func waitForVMM(
	client v1alpha1.MicroVMClient,
	provider u.Provider,
	stateDir, name, namespace, uid string,
) int {
	log.Printf("TEST STEP: waiting for the VMM of MicroVM %s/%s to start", namespace, name)

	start := time.Now()
	lastSeen := ""
	pid := 0

	Eventually(func(g Gomega) error {
		res := u.GetMVM(client, uid)
		seen := fmt.Sprintf("%s (retry %d)", res.Microvm.Status.State, res.Microvm.Status.Retry)
		if seen != lastSeen {
			log.Printf("TEST INFO: MicroVM %s/%s is %s", namespace, name, seen)
			lastSeen = seen
		}

		// verify that the VMM of the provider has started, that a pid has been
		// saved and that there is actually a running process
		found, err := u.VerifyVMM(stateDir, provider)
		g.Expect(err).NotTo(HaveOccurred())

		pid = found

		return nil
	}, overlayCreateTimeout).Should(Succeed())

	log.Printf("TEST INFO: the VMM of MicroVM %s/%s is %s pid %d, it took %s",
		namespace, name, provider.Name, pid, time.Since(start).Round(time.Millisecond))

	return pid
}

// verifyOverlayContract checks the line which the overlay init wrote to the
// console: the base is EROFS and the writable disk is ext4.
func verifyOverlayContract(stateDir string, provider u.Provider) {
	contract, err := u.ConsoleOverlayContract(stateDir, provider)
	Expect(err).NotTo(HaveOccurred())
	Expect(contract.IsOverlay()).To(Succeed())

	log.Printf("TEST INFO: %s/%s has the line: %s", stateDir, provider.ConsoleFile, contract.Line)
}

// waitForWrites waits until the guest has written to its writable disk. The
// disk has more blocks than a fresh clone of the template, and its upper
// directory has the directories which the guest writes to.
//
// The blocks are not compared with the ones of the template: XFS does not map
// the unwritten extents of the template into a clone, so a fresh clone has
// fewer blocks than the template. The test makes a clone of its own, on the
// same filesystem, to compare with.
func waitForWrites(rwDisk, template string) {
	log.Printf("TEST STEP: waiting for the guest to write to %s", rwDisk)

	clone := filepath.Join(filepath.Dir(rwDisk), ".e2e-clone-of-"+filepath.Base(template))
	Expect(u.CloneFile(template, clone)).To(Succeed())

	defer func() { _ = os.Remove(clone) }()

	cloneBlocks, err := u.AllocatedBlocks(clone)
	Expect(err).NotTo(HaveOccurred())

	var (
		diskBlocks int64
		entries    []string
	)

	Eventually(func(g Gomega) error {
		blocks, err := u.AllocatedBlocks(rwDisk)
		g.Expect(err).NotTo(HaveOccurred())

		diskBlocks = blocks

		g.Expect(blocks).To(BeNumerically(">", cloneBlocks),
			"%s has %s, a fresh clone of %s has %s", rwDisk, u.FormatBlocks(blocks), template, u.FormatBlocks(cloneBlocks))

		// The guest writes etc, var and tmp into the upper directory when it
		// boots. The listing is of a snapshot of the disk with the journal
		// replayed, which has what the guest had committed when the snapshot
		// was made. The guest commits every few seconds, so it is tried again.
		names, err := u.UpperEntries(rwDisk)
		g.Expect(err).NotTo(HaveOccurred())

		entries = names

		g.Expect(names).To(ContainElement(BeElementOf(overlayUpperDirectories)),
			"the upper directory of %s has no directory which the guest writes to", rwDisk)

		return nil
	}, overlayWriteTimeout, overlayWritePolling).Should(Succeed())

	log.Printf("TEST INFO: %s has %s, a fresh clone of %s has %s",
		rwDisk, u.FormatBlocks(diskBlocks), template, u.FormatBlocks(cloneBlocks))
	log.Printf("TEST INFO: the upper directory of %s has: %s", rwDisk, strings.Join(entries, ", "))
}

// logResizefs logs what cloud-init said about the size of the root. Its
// resizefs module finds an overlay and is expected to skip it. The marker
// of cloud-init shows that it went on, so the line is only logged.
func logResizefs(stateDir string, provider u.Provider) {
	line, err := u.ConsoleLine(stateDir, provider, "resizefs")
	if err != nil {
		log.Printf("TEST INFO: cloud-init did not write a line about resizefs to the console")

		return
	}

	log.Printf("TEST INFO: %s/%s has the line: %s", stateDir, provider.ConsoleFile, line)
}
