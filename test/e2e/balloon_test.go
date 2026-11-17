//go:build e2e
// +build e2e

package e2e_test

import (
	"fmt"
	"log"
	"testing"

	. "github.com/onsi/gomega"

	"github.com/liquidmetal-dev/flintlock/api/services/microvm/v1alpha1"
	"github.com/liquidmetal-dev/flintlock/api/types"
	u "github.com/liquidmetal-dev/flintlock/test/e2e/utils"
)

const (
	// balloonFillMiB is how much memory the guest touches and then frees.
	balloonFillMiB = 1024
	// balloonFillWait is how long the guest holds the memory, in seconds, so
	// that the test can read the VMM's resident set while it is full.
	balloonFillWait = 20
	// balloonMinReturnedMiB is how much of the freed memory the VMM has to
	// give back. Free page reporting returns only blocks of a minimum order,
	// so less than all of it comes back.
	balloonMinReturnedMiB = 512
)

// TestE2EBalloon boots a microvm with a balloon with free page reporting with
// each of the providers. The guest's balloon driver binds the device and
// agrees on free page reporting. Memory that the guest touches and then
// frees goes back to the host: the VMM's resident set shrinks.
func TestE2EBalloon(t *testing.T) {
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
			// As in TestE2E, a failure has to fail the subtest that is running.
			RegisterTestingT(subT)
			defer RegisterTestingT(t)

			runBalloon(subT, flintlockClient, provider)
		})
	}
}

func runBalloon(t *testing.T, flintlockClient v1alpha1.MicroVMClient, provider u.Provider) {
	t.Helper()

	const statePath = "/var/lib/flintlock/vm/%s/%s/%s"

	mvmID := "mvm-balloon0"
	mvmNS := "ns-balloon-" + provider.Name

	log.Println("TEST STEP: creating MicroVM with a balloon with free page reporting")
	created := u.CreateBalloonMVM(flintlockClient, provider, mvmID, mvmNS,
		u.BalloonStateCommand(), u.BalloonFillCommand(balloonFillMiB, balloonFillWait))

	uid := *created.Microvm.Spec.Uid
	stateDir := fmt.Sprintf(statePath, mvmNS, mvmID, uid)
	defer cleanupOnFailure(t, flintlockClient, provider, stateDir, mvmNS, uid)

	Expect(created.Microvm.Spec.Balloon.GetFreePageReporting()).To(BeTrue())

	var vmmPid int

	Eventually(func(g Gomega) error {
		pid, err := u.VerifyVMM(stateDir, provider)
		g.Expect(err).NotTo(HaveOccurred())

		vmmPid = pid

		res := u.GetMVM(flintlockClient, uid)
		g.Expect(res.Microvm.Status.State).To(Equal(types.MicroVMStatus_CREATED))

		return nil
	}, "120s").Should(Succeed())

	log.Println("TEST STEP: waiting for the guest to fill its memory")

	var filledMiB int

	Eventually(func(g Gomega) error {
		g.Expect(verifySameVMM(stateDir, provider, vmmPid)).To(Succeed())

		_, err := u.ConsoleLine(stateDir, provider, u.BalloonMarker+" filled")
		g.Expect(err).NotTo(HaveOccurred())

		filledMiB, err = u.VMMRSSMiB(vmmPid)
		g.Expect(err).NotTo(HaveOccurred())

		return nil
	}, bootTimeout, "1s").Should(Succeed())
	log.Printf("TEST INFO: the %s VMM holds %d MiB with the guest's memory full", provider.Name, filledMiB)

	// The guest frees the memory, then cloud-init writes the boot marker.
	waitForBoot(provider, stateDir, mvmID, mvmNS, vmmPid, bootTimeout)

	log.Println("TEST STEP: verifying that the balloon driver bound the device with free page reporting")
	device, reporting, err := u.BalloonState(stateDir, provider)
	Expect(err).NotTo(HaveOccurred())
	Expect(device).NotTo(Equal("none"), "the guest's balloon driver bound no device")
	Expect(reporting).To(BeTrue(), "the balloon device %s has no free page reporting", device)

	_, err = u.ConsoleLine(stateDir, provider, u.BalloonMarker+" freed")
	Expect(err).NotTo(HaveOccurred())

	log.Println("TEST STEP: verifying that the VMM gave back the memory the guest freed")
	Eventually(func(g Gomega) error {
		rss, err := u.VMMRSSMiB(vmmPid)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(rss).To(BeNumerically("<=", filledMiB-balloonMinReturnedMiB),
			"the %s VMM holds %d MiB, %d MiB with the guest's memory full", provider.Name, rss, filledMiB)

		return nil
	}, "60s", "2s").Should(Succeed())

	saveArtefacts(t, stateDir)

	if params.SkipDelete {
		log.Println("TEST STEP: skipping delete")
		return
	}

	log.Println("TEST STEP: deleting the MicroVM")
	Expect(u.DeleteMVM(flintlockClient, uid)).To(Succeed())

	Eventually(func(g Gomega) error {
		g.Expect(stateDir).NotTo(BeAnExistingFile())
		g.Expect(u.PidRunning(vmmPid)).To(BeFalse())

		return nil
	}, "120s").Should(Succeed())
}
