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

	"github.com/liquidmetal-dev/flintlock/api/services/microvm/v1alpha1"
	"github.com/liquidmetal-dev/flintlock/api/types"
	"github.com/liquidmetal-dev/flintlock/pkg/ptr"
	u "github.com/liquidmetal-dev/flintlock/test/e2e/utils"
	. "github.com/onsi/gomega"
)

var (
	params *u.Params

	// environmentLeftRunning is set when a test did not tear down what it
	// started, which means that the tests after it cannot do their own setup.
	environmentLeftRunning bool
)

func init() {
	// Call testing.Init() prior to tests.NewParams(), as otherwise custom test flags
	// will not be recognised.
	testing.Init()
	params = u.NewParams()
}

const (
	// Long enough that sockets in the state dir would go over the unix socket path limit.
	longNameLength      = 44
	longNamespaceLength = 43
)

// TestE2E runs the lifecycle of a microvm with each of the providers. The
// providers share one flintlockd, which has all of them enabled.
func TestE2E(t *testing.T) {
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
			// The helpers of the tests assert with the global gomega. A failure
			// has to fail the subtest which is running: if it failed the test of
			// the subtest, the test binary would panic and skip the teardown.
			RegisterTestingT(subT)
			defer RegisterTestingT(t)

			log.Printf("TEST STEP: running the lifecycle of a MicroVM with the %s provider", provider.Name)
			runLifecycle(subT, flintlockClient, provider)
		})
	}
}

// lifecycleNamespaces returns the namespaces for the microvms of a provider
// in the lifecycle test. The second one is a long namespace.
func lifecycleNamespaces(provider u.Provider) (string, string) {
	longPrefix := "ns-" + provider.Name + "-"

	return provider.Name + "-ns0", longPrefix + strings.Repeat("s", longNamespaceLength-len(longPrefix))
}

func runLifecycle(t *testing.T, flintlockClient v1alpha1.MicroVMClient, provider u.Provider) {
	t.Helper()

	var (
		mvmID       = "mvm0"
		secondMvmID = "mvm1"
		statePath   = "/var/lib/flintlock/vm/%s/%s/%s"
		socketDir   = "/run/flintlock"

		longMvmID = "mvm-" + strings.Repeat("n", longNameLength-len("mvm-"))

		mvmPid1 int
		mvmPid2 int
		mvmPid3 int
	)

	mvmNS, longMvmNS := lifecycleNamespaces(provider)

	log.Println("TEST STEP: creating MicroVM")
	created := u.CreateMVM(flintlockClient, provider, mvmID, mvmNS)

	// The cleanup is set up before anything is checked, so that the microvm is
	// also deleted when the first check fails.
	firstMicroVMPath := fmt.Sprintf(statePath, mvmNS, mvmID, *created.Microvm.Spec.Uid)
	defer cleanupOnFailure(t, flintlockClient, provider, firstMicroVMPath, mvmNS, *created.Microvm.Spec.Uid)

	Expect(created.Microvm.Spec.Id).To(Equal(mvmID))

	log.Println("TEST STEP: getting (and verifying) existing MicroVM")
	Eventually(func(g Gomega) error {
		// verify that the VMM of the provider has started, that a pid has been
		// saved and that there is actually a running process
		pid, err := u.VerifyVMM(firstMicroVMPath, provider)
		g.Expect(err).NotTo(HaveOccurred())

		mvmPid1 = pid

		// get the mVM and check the status
		res := u.GetMVM(flintlockClient, *created.Microvm.Spec.Uid)
		g.Expect(res.Microvm.Spec.Id).To(Equal(mvmID))
		g.Expect(res.Microvm.Status.State).To(Equal(types.MicroVMStatus_CREATED))
		return nil
	}, "120s").Should(Succeed())

	waitForBoot(provider, firstMicroVMPath, mvmID, mvmNS, mvmPid1)

	log.Println("TEST STEP: creating a second MicroVM")
	createdSecond := u.CreateMVM(flintlockClient, provider, secondMvmID, mvmNS)

	secondMicroVMPath := fmt.Sprintf(statePath, mvmNS, secondMvmID, *createdSecond.Microvm.Spec.Uid)
	defer cleanupOnFailure(t, flintlockClient, provider, secondMicroVMPath, mvmNS, *createdSecond.Microvm.Spec.Uid)

	Expect(createdSecond.Microvm.Spec.Id).To(Equal(secondMvmID))

	log.Println("TEST STEP: listing all MicroVMs")
	Eventually(func(g Gomega) error {
		// verify that the VMM of the provider has started, that a pid has been
		// saved and that there is actually a running process for the new mVM
		pid, err := u.VerifyVMM(secondMicroVMPath, provider)
		g.Expect(err).NotTo(HaveOccurred())

		mvmPid2 = pid

		// get both the mVMs and check the statuses
		res := u.ListMVMs(flintlockClient, mvmNS, nil)
		g.Expect(res.Microvm).To(HaveLen(2))
		g.Expect(res.Microvm[0].Spec.Id).To(Equal(mvmID))
		g.Expect(res.Microvm[0].Status.State).To(Equal(types.MicroVMStatus_CREATED))
		g.Expect(res.Microvm[1].Spec.Id).To(Equal(secondMvmID))
		g.Expect(res.Microvm[1].Status.State).To(Equal(types.MicroVMStatus_CREATED))

		// get only the second mVM by name and check the statuses
		res = u.ListMVMs(flintlockClient, mvmNS, ptr.String(secondMvmID))
		g.Expect(res.Microvm).To(HaveLen(1))
		g.Expect(res.Microvm[0].Spec.Id).To(Equal(secondMvmID))
		g.Expect(res.Microvm[0].Status.State).To(Equal(types.MicroVMStatus_CREATED))

		return nil
	}, "120s").Should(Succeed())

	waitForBoot(provider, secondMicroVMPath, secondMvmID, mvmNS, mvmPid2)

	log.Println("TEST STEP: creating a MicroVM with a long namespace and name and the guest agent enabled")
	createdLong := u.CreateGuestAgentMVM(flintlockClient, provider, longMvmID, longMvmNS)

	longMicroVMPath := fmt.Sprintf(statePath, longMvmNS, longMvmID, *createdLong.Microvm.Spec.Uid)
	longSocketRoot := filepath.Join(socketDir, *createdLong.Microvm.Spec.Uid)
	defer cleanupOnFailure(t, flintlockClient, provider, longMicroVMPath, longMvmNS, *createdLong.Microvm.Spec.Uid)

	Expect(createdLong.Microvm.Spec.Id).To(Equal(longMvmID))

	Eventually(func(g Gomega) error {
		pid, err := u.VerifyVMM(longMicroVMPath, provider)
		g.Expect(err).NotTo(HaveOccurred())

		mvmPid3 = pid

		// verify that the vsock socket is under the socket dir and the VMM has bound to it
		res := u.GetMVM(flintlockClient, *createdLong.Microvm.Spec.Uid)
		g.Expect(res.Microvm.Status.State).To(Equal(types.MicroVMStatus_CREATED))
		g.Expect(res.Microvm.Status.VsockPath).To(HavePrefix(longSocketRoot + "/"))

		info, err := os.Stat(res.Microvm.Status.VsockPath)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(info.Mode() & os.ModeSocket).NotTo(BeZero())

		return nil
	}, "120s").Should(Succeed())

	waitForBoot(provider, longMicroVMPath, longMvmID, longMvmNS, mvmPid3)

	// flintlockd removes the state directory when the microvm is deleted.
	saveArtefacts(t, firstMicroVMPath)
	saveArtefacts(t, secondMicroVMPath)
	saveArtefacts(t, longMicroVMPath)

	if params.SkipDelete {
		log.Println("TEST STEP: skipping delete")
		return
	}

	log.Println("TEST STEP: deleting existing MicroVMs")
	Expect(u.DeleteMVM(flintlockClient, *created.Microvm.Spec.Uid)).To(Succeed())
	Expect(u.DeleteMVM(flintlockClient, *createdSecond.Microvm.Spec.Uid)).To(Succeed())
	Expect(u.DeleteMVM(flintlockClient, *createdLong.Microvm.Spec.Uid)).To(Succeed())

	Eventually(func(g Gomega) error {
		// verify that the vm state dirs have been removed
		g.Expect(firstMicroVMPath).ToNot(BeAnExistingFile())
		g.Expect(secondMicroVMPath).ToNot(BeAnExistingFile())
		g.Expect(longMicroVMPath).ToNot(BeAnExistingFile())

		// verify that the socket dir has been removed
		g.Expect(longSocketRoot).ToNot(BeAnExistingFile())

		// verify that the VMM processes are no longer running
		g.Expect(u.PidRunning(mvmPid1)).To(BeFalse())
		g.Expect(u.PidRunning(mvmPid2)).To(BeFalse())
		g.Expect(u.PidRunning(mvmPid3)).To(BeFalse())

		// verify that the mVMs are no longer with us
		res := u.ListMVMs(flintlockClient, mvmNS, nil)
		g.Expect(res.Microvm).To(HaveLen(0))
		res = u.ListMVMs(flintlockClient, longMvmNS, nil)
		g.Expect(res.Microvm).To(HaveLen(0))
		return nil
	}, "120s").Should(Succeed())
}
