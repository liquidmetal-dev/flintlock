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

// TestE2EFirecrackerMetadataKernelIP boots a Firecracker microvm with a
// flintlockd that has --firecracker-metadata-kernel-ip. The guest kernel
// configures the metadata interface, so systemd-networkd does not manage it,
// and the guest still reads its cloud-init data from MMDS.
func TestE2EFirecrackerMetadataKernelIP(t *testing.T) {
	RegisterTestingT(t)

	if environmentLeftRunning {
		t.Skip("a previous test left its environment running, use -run to run this test on its own")
	}

	provider := u.Firecracker()
	if !selected(provider) {
		t.Skipf("the %s provider is not one of the providers of -providers", provider.Name)
	}

	environmentLeftRunning = params.SkipTeardown || params.SkipDelete

	r := u.NewRunner(params, "--firecracker-metadata-kernel-ip")
	defer func() {
		log.Println("TEST STEP: cleaning up running processes")
		r.Teardown()
	}()
	log.Println("TEST STEP: performing setup, starting flintlockd server with --firecracker-metadata-kernel-ip")
	flintlockClient := r.Setup()

	runMetadataKernelIP(t, flintlockClient, provider)
}

func runMetadataKernelIP(t *testing.T, flintlockClient v1alpha1.MicroVMClient, provider u.Provider) {
	t.Helper()

	const (
		mvmID     = "mvm-kernel-ip0"
		mvmNS     = "ns-kernel-ip"
		statePath = "/var/lib/flintlock/vm/%s/%s/%s"
	)

	log.Println("TEST STEP: creating MicroVM")
	created := u.CreateMVM(flintlockClient, provider, mvmID, mvmNS, u.MetadataInterfaceStateCommand())

	uid := *created.Microvm.Spec.Uid
	stateDir := fmt.Sprintf(statePath, mvmNS, mvmID, uid)
	defer cleanupOnFailure(t, flintlockClient, provider, stateDir, mvmNS, uid)

	var vmmPid int

	Eventually(func(g Gomega) error {
		pid, err := u.VerifyVMM(stateDir, provider)
		g.Expect(err).NotTo(HaveOccurred())

		vmmPid = pid

		res := u.GetMVM(flintlockClient, uid)
		g.Expect(res.Microvm.Status.State).To(Equal(types.MicroVMStatus_CREATED))

		return nil
	}, "120s").Should(Succeed())

	// The boot marker is in the user-data, which the guest reads from MMDS
	// through the metadata interface.
	waitForBoot(provider, stateDir, mvmID, mvmNS, vmmPid, bootTimeout)

	log.Println("TEST STEP: verifying that each try to read the vendor-data succeeded")
	tries, ok, err := u.VendorDataTries(stateDir, provider)
	Expect(err).NotTo(HaveOccurred())
	Expect(ok).To(BeNumerically(">=", 1), "the guest has not read its vendor-data")
	Expect(tries).To(Equal(ok), "the guest tried to read its vendor-data %d times, %d succeeded", tries, ok)

	log.Println("TEST STEP: verifying that systemd-networkd does not manage the metadata interface")
	setup, address, err := u.MetadataInterfaceState(stateDir, provider)
	Expect(err).NotTo(HaveOccurred())
	Expect(setup).To(Equal("unmanaged"))
	Expect(address).To(Equal("169.254.0.1/16"))

	// flintlockd removes the state directory when the microvm is deleted.
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
