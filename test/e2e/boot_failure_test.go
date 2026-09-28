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
