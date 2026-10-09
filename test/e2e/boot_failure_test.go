//go:build e2e
// +build e2e

package e2e_test

import (
	"fmt"
	"log"
	"path/filepath"
	"testing"
	"time"

	. "github.com/onsi/gomega"

	"github.com/liquidmetal-dev/flintlock/api/services/microvm/v1alpha1"
	u "github.com/liquidmetal-dev/flintlock/test/e2e/utils"
)

// TestE2ECloudHypervisorKernelWithoutPVH creates a microvm whose guest cannot
// boot. It gives Cloud Hypervisor a kernel which does not have the PVH entry
// point that Cloud Hypervisor boots a kernel from.
//
// It shows that the checks of the other tests do not pass for a microvm like
// this one. They are the only thing which sees it: flintlockd reports the
// microvm as CREATED (#1263), so the test does not look at the state.
func TestE2ECloudHypervisorKernelWithoutPVH(t *testing.T) {
	RegisterTestingT(t)

	if environmentLeftRunning {
		t.Skip("a previous test left its environment running, use -run to run this test on its own")
	}

	provider := u.CloudHypervisor()

	if !selected(provider) {
		t.Skipf("the %s provider is not one of the providers of -providers", provider.Name)
	}

	const (
		mvmID     = "mvm-nopvh"
		mvmNS     = "ns-nopvh"
		statePath = "/var/lib/flintlock/vm/%s/%s/%s"

		// How long the guest gets to boot after the VMM has failed. The guests
		// of the other tests boot in less than 20 seconds.
		noBootDuration = "30s"
		noBootPolling  = "2s"
	)

	provider.KernelImage = u.KernelWithoutPVHImage
	provider.KernelFilename = u.KernelWithoutPVHFilename

	environmentLeftRunning = params.SkipTeardown || params.SkipDelete

	r := u.NewRunner(params)
	defer func() {
		log.Println("TEST STEP: cleaning up running processes")
		r.Teardown()
	}()
	log.Println("TEST STEP: performing setup, starting flintlockd server")
	flintlockClient := r.Setup()

	log.Printf("TEST STEP: creating a MicroVM with the kernel %s, which has no PVH entry point", provider.KernelImage)
	created := u.CreateMVM(flintlockClient, provider, mvmID, mvmNS)

	// The cleanup is set up before anything is checked, so that the microvm is
	// also deleted when the first check fails.
	microVMPath := fmt.Sprintf(statePath, mvmNS, mvmID, *created.Microvm.Spec.Uid)
	defer cleanupOnFailure(t, flintlockClient, provider, microVMPath, mvmNS, *created.Microvm.Spec.Uid)

	Expect(created.Microvm.Spec.Id).To(Equal(mvmID))
	Expect(created.Microvm.Spec.Kernel.Image).To(Equal(provider.KernelImage))

	log.Printf("TEST INFO: MicroVM %s/%s has uid %s and state directory %s",
		mvmNS, mvmID, *created.Microvm.Spec.Uid, microVMPath)

	log.Println("TEST STEP: waiting for the VMM to fail to boot the kernel")
	start := time.Now()
	vmmPid := 0
	vmmError := ""

	Eventually(func(g Gomega) error {
		// The pid shows that flintlockd has started the VMM, and the error shows
		// why the VMM is not running any more.
		pid, err := u.ReadPID(microVMPath, provider)
		g.Expect(err).NotTo(HaveOccurred())

		line, err := u.MissingPVHErrorLine(microVMPath, provider)
		g.Expect(err).NotTo(HaveOccurred())

		vmmPid = pid
		vmmError = line

		return nil
	}, "120s", "1s").Should(Succeed())

	log.Printf("TEST INFO: the VMM with pid %d has failed after %s, %s has the line: %s",
		vmmPid, time.Since(start).Round(time.Millisecond), filepath.Join(microVMPath, provider.StderrFile), vmmError)

	log.Printf("TEST STEP: verifying for %s that the checks of a running MicroVM do not pass", noBootDuration)
	lastSeen := ""

	Consistently(func(g Gomega) {
		// The state is only logged. flintlockd reports the microvm as CREATED,
		// which is what makes the other checks necessary (#1263).
		res := u.GetMVM(flintlockClient, *created.Microvm.Spec.Uid)
		seen := fmt.Sprintf("%s (retry %d)", res.Microvm.Status.State, res.Microvm.Status.Retry)

		if seen != lastSeen {
			log.Printf("TEST INFO: MicroVM %s/%s is %s", mvmNS, mvmID, seen)
			lastSeen = seen
		}

		_, err := u.VerifyVMM(microVMPath, provider)
		g.Expect(err).To(HaveOccurred(), "the VMM is running")

		line, err := u.ConsoleMarker(microVMPath, provider, mvmID, mvmNS)
		g.Expect(err).To(HaveOccurred(), "the guest has booted, the console has the line: %s", line)

		// The VMM of another provider would have its own pid file.
		g.Expect(u.VMMPidFiles(microVMPath)).To(ConsistOf(filepath.Join(microVMPath, provider.PidFile)))
	}, noBootDuration, noBootPolling).Should(Succeed())

	log.Printf("TEST INFO: no VMM is running for MicroVM %s/%s and its guest has not booted", mvmNS, mvmID)

	// The VMM has exited, so waitForBoot does not wait for the marker.
	expectWaitForBootToFail(provider, microVMPath, mvmID, mvmNS, vmmPid, noBootDuration, vmmNotRunningReason)

	// flintlockd removes the state directory when the microvm is deleted.
	saveArtefacts(t, microVMPath)

	if params.SkipDelete {
		log.Println("TEST STEP: skipping delete")

		return
	}

	log.Println("TEST STEP: deleting the MicroVM")
	Expect(u.DeleteMVM(flintlockClient, *created.Microvm.Spec.Uid)).To(Succeed())

	log.Println("TEST STEP: waiting for the MicroVM to be deleted")
	deleteStart := time.Now()

	Eventually(func(g Gomega) error {
		g.Expect(microVMPath).ToNot(BeAnExistingFile())

		res := u.ListMVMs(flintlockClient, mvmNS, nil)
		g.Expect(res.Microvm).To(BeEmpty())

		return nil
	}, "120s").Should(Succeed())

	log.Printf("TEST INFO: MicroVM %s/%s is deleted, it took %s",
		mvmNS, mvmID, time.Since(deleteStart).Round(time.Millisecond))
}

// TestE2EGuestWithoutInit creates a microvm whose guest starts to boot and
// cannot finish. Its root volume has no init, so the kernel boots, mounts the
// root volume, panics and reboots.
//
// What the VMM does then is not the same for all of the providers. One which
// resets the guest keeps running, with a guest that boots and panics again and
// again. Such a microvm is CREATED and has a VMM which runs, the only check
// which does not pass for it is the one of the boot marker.
func TestE2EGuestWithoutInit(t *testing.T) {
	RegisterTestingT(t)

	if environmentLeftRunning {
		t.Skip("a previous test left its environment running, use -run to run this test on its own")
	}

	environmentLeftRunning = params.SkipTeardown || params.SkipDelete

	r := u.NewRunner(params)
	defer func() {
		log.Println("TEST STEP: cleaning up running processes")
		r.Teardown()
	}()
	log.Println("TEST STEP: performing setup, starting flintlockd server")
	flintlockClient := r.Setup()

	for _, provider := range params.Providers {
		t.Run(provider.Name, func(subT *testing.T) {
			RegisterTestingT(subT)
			defer RegisterTestingT(t)

			runGuestWithoutInit(subT, flintlockClient, provider)
		})
	}
}

func runGuestWithoutInit(t *testing.T, flintlockClient v1alpha1.MicroVMClient, provider u.Provider) {
	t.Helper()

	const (
		mvmID     = "mvm-noinit"
		statePath = "/var/lib/flintlock/vm/%s/%s/%s"

		// How long the guest gets to boot after its first panic. The guests of
		// the other tests boot in less than 20 seconds.
		noBootDuration = "30s"
		noBootPolling  = "2s"
	)

	// The image of the kernel has the kernel and nothing else, so it is a root
	// volume without an init. It is in the content store already.
	var (
		mvmNS     = provider.Name + "-noinit"
		rootImage = provider.KernelImage
	)

	log.Printf("TEST STEP: creating a MicroVM with the %s provider and the root volume %s, which has no init",
		provider.Name, rootImage)
	created := u.CreateMVMWithImages(flintlockClient, provider, mvmID, mvmNS, provider.KernelImage, rootImage)

	// The cleanup is set up before anything is checked, so that the microvm is
	// also deleted when the first check fails.
	microVMPath := fmt.Sprintf(statePath, mvmNS, mvmID, *created.Microvm.Spec.Uid)
	defer cleanupOnFailure(t, flintlockClient, provider, microVMPath, mvmNS, *created.Microvm.Spec.Uid)

	Expect(created.Microvm.Spec.Id).To(Equal(mvmID))
	Expect(*created.Microvm.Spec.RootVolume.Source.ContainerSource).To(Equal(rootImage))

	log.Printf("TEST INFO: MicroVM %s/%s has uid %s and state directory %s",
		mvmNS, mvmID, *created.Microvm.Spec.Uid, microVMPath)

	log.Println("TEST STEP: waiting for the kernel of the guest to panic")
	start := time.Now()
	vmmPid := 0
	panicLine := ""

	Eventually(func(g Gomega) error {
		// The pid shows that flintlockd has started the VMM, and the line shows
		// how far the guest got and why it did not get further.
		pid, err := u.ReadPID(microVMPath, provider)
		g.Expect(err).NotTo(HaveOccurred())

		line, err := u.ConsoleLine(microVMPath, provider, "kernel panic", "no working init")
		g.Expect(err).NotTo(HaveOccurred())

		// A VMM which does not reset the guest exits when the guest reboots,
		// which is a moment after the panic.
		if !provider.ResetsGuest {
			g.Expect(u.PidRunning(pid)).To(BeFalse(), "the VMM is still running")
		}

		vmmPid = pid
		panicLine = line

		return nil
	}, "120s", "1s").Should(Succeed())

	log.Printf("TEST INFO: the guest of the VMM with pid %d has panicked after %s, %s has the line: %s",
		vmmPid, time.Since(start).Round(time.Millisecond), filepath.Join(microVMPath, provider.ConsoleFile), panicLine)

	log.Printf("TEST STEP: verifying for %s that the guest does not boot", noBootDuration)
	lastSeen := ""

	Consistently(func(g Gomega) {
		// The state is only logged. flintlockd reports the microvm as CREATED,
		// which is what makes the other checks necessary (#1263).
		res := u.GetMVM(flintlockClient, *created.Microvm.Spec.Uid)
		_, vmmErr := u.VerifyVMM(microVMPath, provider)

		seen := fmt.Sprintf("%s (retry %d), its VMM is running: %t",
			res.Microvm.Status.State, res.Microvm.Status.Retry, vmmErr == nil)
		if seen != lastSeen {
			log.Printf("TEST INFO: MicroVM %s/%s is %s", mvmNS, mvmID, seen)
			lastSeen = seen
		}

		line, err := u.ConsoleMarker(microVMPath, provider, mvmID, mvmNS)
		g.Expect(err).To(HaveOccurred(), "the guest has booted, the console has the line: %s", line)

		if provider.ResetsGuest {
			// This is the microvm which only the boot marker tells from one
			// that runs: the VMM which flintlockd has started is running.
			g.Expect(verifySameVMM(microVMPath, provider, vmmPid)).To(Succeed())
		} else {
			g.Expect(vmmErr).To(HaveOccurred(), "the VMM is running")
		}

		// The VMM of another provider would have its own pid file.
		g.Expect(u.VMMPidFiles(microVMPath)).To(ConsistOf(filepath.Join(microVMPath, provider.PidFile)))
	}, noBootDuration, noBootPolling).Should(Succeed())

	log.Printf("TEST INFO: the guest of MicroVM %s/%s has not booted", mvmNS, mvmID)

	// A VMM which keeps running makes waitForBoot wait for the marker until
	// its timeout. This is the only guest for which it gets that far.
	reason := vmmNotRunningReason
	if provider.ResetsGuest {
		reason = noMarkerReason
	}

	expectWaitForBootToFail(provider, microVMPath, mvmID, mvmNS, vmmPid, noBootDuration, reason)

	// flintlockd removes the state directory when the microvm is deleted.
	saveArtefacts(t, microVMPath)

	if params.SkipDelete {
		log.Println("TEST STEP: skipping delete")

		return
	}

	log.Println("TEST STEP: deleting the MicroVM")
	Expect(u.DeleteMVM(flintlockClient, *created.Microvm.Spec.Uid)).To(Succeed())

	log.Println("TEST STEP: waiting for the MicroVM to be deleted")
	deleteStart := time.Now()

	Eventually(func(g Gomega) error {
		g.Expect(microVMPath).ToNot(BeAnExistingFile())
		g.Expect(u.PidRunning(vmmPid)).To(BeFalse())

		res := u.ListMVMs(flintlockClient, mvmNS, nil)
		g.Expect(res.Microvm).To(BeEmpty())

		return nil
	}, "120s").Should(Succeed())

	log.Printf("TEST INFO: MicroVM %s/%s is deleted, it took %s",
		mvmNS, mvmID, time.Since(deleteStart).Round(time.Millisecond))
}

const (
	// vmmNotRunningReason is in the failure of waitForBoot for a VMM which has
	// exited. It is the message of the StopTrying in waitForBoot.
	vmmNotRunningReason = "did not keep running"
	// noMarkerReason is in the failure of waitForBoot for a VMM which runs
	// without a guest that has booted. It is from the error of ConsoleMarker.
	noMarkerReason = "boot marker"
)

// expectWaitForBootToFail checks that waitForBoot, which the tests of a guest
// that boots rely on, fails for this guest, and fails for the reason given.
// Without this, a waitForBoot which passes for every guest would fail no test.
func expectWaitForBootToFail(provider u.Provider, stateDir, name, namespace string, vmmPid int, timeout, reason string) {
	log.Printf("TEST STEP: verifying that waitForBoot does not pass within %s", timeout)

	// InterceptGomegaFailure stops waitForBoot at its first failed assertion
	// and returns the failure, so the log lines of a guest which has booted
	// are not printed.
	err := InterceptGomegaFailure(func() {
		waitForBoot(provider, stateDir, name, namespace, vmmPid, timeout)
	})
	Expect(err).To(HaveOccurred(), "waitForBoot passed for a guest which has not booted")
	Expect(err).To(MatchError(ContainSubstring(reason)), "waitForBoot failed for another reason")

	log.Printf("TEST INFO: waitForBoot did not pass: %s", err)
}

// selected returns true if the provider is one of the providers which the
// tests run with.
func selected(provider u.Provider) bool {
	for _, p := range params.Providers {
		if p.Name == provider.Name {
			return true
		}
	}

	return false
}
