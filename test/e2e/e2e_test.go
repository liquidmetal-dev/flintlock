//go:build e2e
// +build e2e

package e2e_test

import (
	"fmt"
	"log"
	"testing"

	. "github.com/onsi/gomega"
	"github.com/weaveworks/flintlock/api/types"
	u "github.com/weaveworks/flintlock/test/e2e/utils"
)

var params *u.Params

func init() {
	// Call testing.Init() prior to tests.NewParams(), as otherwise custom test flags
	// will not be recognised.
	testing.Init()
	params = u.NewParams()
}

func TestE2E(t *testing.T) {
	RegisterTestingT(t)

	var (
		mvmID       = "mvm0"
		secondMvmID = "mvm1"
		mvmNS       = "ns0"
		fcPath      = "/var/lib/flintlock/vm/%s/%s"

		mvmPid1 int
		mvmPid2 int
	)

	r := u.NewRunner(params)
	defer func() {
		log.Println("TEST STEP: cleaning up running processes")
		r.Teardown()
	}()
	log.Println("TEST STEP: performing setup, starting flintlockd server")
	flintlockClient := r.Setup()

	log.Println("TEST STEP: creating MicroVM")
	created := u.CreateMVM(flintlockClient, mvmID, mvmNS)
	Expect(created.Microvm.Id).To(Equal(mvmID))

	log.Println("TEST STEP: getting (and verifying) existing MicroVM")
	Eventually(func(g Gomega) error {
		g.Expect(fmt.Sprintf(fcPath, mvmNS, mvmID) + "/firecracker.pid").To(BeAnExistingFile())

		// verify that firecracker has started and that a pid has been saved
		// and that there is actually a running process
		mvmPid1 = u.ReadPID(fmt.Sprintf(fcPath, mvmNS, mvmID))
		g.Expect(u.PidRunning(mvmPid1)).To(BeTrue())

		// get the mVM and check the status
		res := u.GetMVM(flintlockClient, mvmID, mvmNS)
		g.Expect(res.Microvm.Spec.Id).To(Equal(mvmID))
		g.Expect(res.Microvm.Status.State).To(Equal(types.MicroVMStatus_CREATED))
		return nil
	}, "120s").Should(Succeed())

	log.Println("TEST STEP: creating a second MicroVM")
	created = u.CreateMVM(flintlockClient, secondMvmID, mvmNS)
	Expect(created.Microvm.Id).To(Equal(secondMvmID))

	log.Println("TEST STEP: listing all MicroVMs")
	Eventually(func(g Gomega) error {
		g.Expect(fmt.Sprintf(fcPath, mvmNS, secondMvmID) + "/firecracker.pid").To(BeAnExistingFile())

		// verify that firecracker has started and that a pid has been saved
		// and that there is actually a running process for the new mVM
		mvmPid2 = u.ReadPID(fmt.Sprintf(fcPath, mvmNS, secondMvmID))
		g.Expect(u.PidRunning(mvmPid2)).To(BeTrue())

		// get both the mVMs and check the statuses
		res := u.ListMVMs(flintlockClient, mvmNS)
		g.Expect(res.Microvm).To(HaveLen(2))
		g.Expect(res.Microvm[0].Spec.Id).To(Equal(mvmID))
		g.Expect(res.Microvm[0].Status.State).To(Equal(types.MicroVMStatus_CREATED))
		g.Expect(res.Microvm[1].Spec.Id).To(Equal(secondMvmID))
		g.Expect(res.Microvm[1].Status.State).To(Equal(types.MicroVMStatus_CREATED))
		return nil
	}, "120s").Should(Succeed())

	if params.SkipDelete {
		log.Println("TEST STEP: skipping delete")
		return
	}

	log.Println("TEST STEP: deleting existing MicroVMs")
	Expect(u.DeleteMVM(flintlockClient, mvmID, mvmNS)).To(Succeed())
	Expect(u.DeleteMVM(flintlockClient, secondMvmID, mvmNS)).To(Succeed())

	Eventually(func(g Gomega) error {
		// verify that the vm state dirs have been removed
		g.Expect(fmt.Sprintf(fcPath, mvmNS, mvmID)).ToNot(BeAnExistingFile())
		g.Expect(fmt.Sprintf(fcPath, mvmNS, secondMvmID)).ToNot(BeAnExistingFile())

		// verify that the firecracker processes are no longer running
		g.Expect(u.PidRunning(mvmPid1)).To(BeFalse())
		g.Expect(u.PidRunning(mvmPid2)).To(BeFalse())

		// verify that the mVMs are no longer with us
		res := u.ListMVMs(flintlockClient, mvmNS)
		g.Expect(res.Microvm).To(HaveLen(0))
		return nil
	}, "120s").Should(Succeed())
}
