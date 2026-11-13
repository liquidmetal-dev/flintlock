//go:build e2e
// +build e2e

package e2e_test

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"testing"
	"time"

	. "github.com/onsi/gomega"
	"github.com/spf13/afero"

	"github.com/liquidmetal-dev/flintlock/api/types"
	"github.com/liquidmetal-dev/flintlock/core/models"
	"github.com/liquidmetal-dev/flintlock/core/ports"
	"github.com/liquidmetal-dev/flintlock/infrastructure/microvm/cloudhypervisor"
	"github.com/liquidmetal-dev/flintlock/infrastructure/microvm/firecracker"
	u "github.com/liquidmetal-dev/flintlock/test/e2e/utils"
)

const (
	// pauseSocketDir is the socket directory which flintlockd is started with.
	pauseSocketDir = "/run/flintlock"
	// pauseStateRoot is the state directory of the microvms of flintlockd.
	pauseStateRoot = u.StateRootDir + "/vm"
)

// TestE2EFirecrackerPauseResume pauses and resumes a microvm with the
// firecracker provider, through the API socket which flintlockd starts
// Firecracker with.
func TestE2EFirecrackerPauseResume(t *testing.T) {
	vmm := firecracker.New(&firecracker.Config{
		StateRoot: pauseStateRoot,
		SocketDir: pauseSocketDir,
	}, nil, afero.NewOsFs())

	testPauseResume(t, u.Firecracker(), vmm, "firecracker.sock", "mvm-pause", "ns-pause")
}

// TestE2ECloudHypervisorPauseResume pauses and resumes a microvm with the
// cloudhypervisor provider, through the API socket which flintlockd starts
// Cloud Hypervisor with.
func TestE2ECloudHypervisorPauseResume(t *testing.T) {
	vmm := cloudhypervisor.New(&cloudhypervisor.Config{
		StateRoot: pauseStateRoot,
		SocketDir: pauseSocketDir,
	}, nil, nil, afero.NewOsFs())

	testPauseResume(t, u.CloudHypervisor(), vmm, "cloudhypervisor.sock", "mvm-pause-ch", "ns-pause-ch")
}

// testPauseResume pauses and resumes a microvm of the provider. The API of
// flintlockd cannot pause a microvm yet, so the test drives vmm, a provider
// of flintlockd on the same state and socket directories.
//
// The guest is paused while it boots. Its console does not grow while it is
// paused, and it finishes its boot after it is resumed, which shows that it
// runs again. The root image of the tests has no guest agent, so the test
// cannot ask the guest to run a command.
func testPauseResume(
	t *testing.T,
	provider u.Provider,
	vmm ports.MicroVMService,
	apiSocketFile, mvmID, mvmNS string,
) {
	RegisterTestingT(t)

	if environmentLeftRunning {
		t.Skip("a previous test left its environment running, use -run to run this test on its own")
	}

	if !selected(provider) {
		t.Skipf("the %s provider is not one of the providers of -providers", provider.Name)
	}

	const (
		statePath = "/var/lib/flintlock/vm/%s/%s/%s"

		// How long the test tries to pause the microvm. The VMM can refuse
		// to pause until it has started the guest.
		pauseTimeout = "10s"
		// How long the guest stays paused, while the test checks that its
		// console does not grow. A guest which boots writes to its console
		// many times a second.
		pausedFor = 5 * time.Second
		// The VMM can still be writing what the guest wrote just before it
		// was paused.
		pauseSettle = time.Second
	)

	environmentLeftRunning = params.SkipTeardown || params.SkipDelete

	r := u.NewRunner(params)
	defer func() {
		log.Println("TEST STEP: cleaning up running processes")
		r.Teardown()
	}()
	log.Println("TEST STEP: performing setup, starting flintlockd server")
	flintlockClient := r.Setup()

	log.Println("TEST STEP: creating a MicroVM")
	created := u.CreateMVM(flintlockClient, provider, mvmID, mvmNS)

	uid := *created.Microvm.Spec.Uid
	microVMPath := fmt.Sprintf(statePath, mvmNS, mvmID, uid)
	socketRoot := filepath.Join(pauseSocketDir, uid)
	apiSocket := filepath.Join(socketRoot, apiSocketFile)

	// The cleanup is set up before anything is checked, so that the microvm is
	// also deleted when the first check fails.
	defer cleanupOnFailure(t, flintlockClient, provider, microVMPath, mvmNS, uid)

	Expect(created.Microvm.Spec.Id).To(Equal(mvmID))

	log.Printf("TEST STEP: waiting for the VMM to run and to listen on %s", apiSocket)
	vmmPid := 0

	Eventually(func(g Gomega) error {
		pid, err := u.VerifyVMM(microVMPath, provider)
		g.Expect(err).NotTo(HaveOccurred())

		vmmPid = pid

		res := u.GetMVM(flintlockClient, uid)
		g.Expect(res.Microvm.Status.State).To(Equal(types.MicroVMStatus_CREATED))

		info, err := os.Stat(apiSocket)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(info.Mode() & os.ModeSocket).NotTo(BeZero())

		return nil
	}, "120s").Should(Succeed())

	vmid, err := models.NewVMID(mvmID, mvmNS, uid)
	Expect(err).NotTo(HaveOccurred())

	vm := &models.MicroVM{ID: *vmid}
	ctx := context.Background()

	_, err = u.ConsoleMarker(microVMPath, provider, mvmID, mvmNS)
	Expect(err).To(HaveOccurred(), "the guest has booted before it was paused, its boot cannot show that it runs again")

	log.Println("TEST STEP: pausing the MicroVM")
	Eventually(func() error {
		return vmm.Pause(ctx, vm)
	}, pauseTimeout, time.Second).Should(Succeed())
	Expect(vmm.State(ctx, vmid.String())).To(Equal(ports.MicroVMStatePaused))

	time.Sleep(pauseSettle)
	consoleSize := fileSize(microVMPath, provider.ConsoleFile)

	// The guest runs while the test retries Pause, so it can finish its boot
	// before Pause succeeds. Check again now that it is paused: a marker
	// written before Pause cannot show that the guest runs after Resume.
	_, err = u.ConsoleMarker(microVMPath, provider, mvmID, mvmNS)
	Expect(err).To(HaveOccurred(), "the guest finished its boot before Pause succeeded, its boot cannot show that it runs again")

	log.Printf("TEST STEP: checking that the console of the paused guest stays at %d bytes for %s", consoleSize, pausedFor)
	Consistently(func() int64 {
		return fileSize(microVMPath, provider.ConsoleFile)
	}, pausedFor, time.Second).Should(Equal(consoleSize))

	Expect(verifySameVMM(microVMPath, provider, vmmPid)).To(Succeed())
	Expect(u.GetMVM(flintlockClient, uid).Microvm.Status.State).To(Equal(types.MicroVMStatus_CREATED))
	Expect(vmm.State(ctx, vmid.String())).To(Equal(ports.MicroVMStatePaused))

	log.Println("TEST STEP: resuming the MicroVM")
	Expect(vmm.Resume(ctx, vm)).To(Succeed())
	Expect(vmm.State(ctx, vmid.String())).To(Equal(ports.MicroVMStateRunning))

	log.Printf("TEST STEP: checking that the console of the resumed guest grows past %d bytes", consoleSize)
	Eventually(func() int64 {
		return fileSize(microVMPath, provider.ConsoleFile)
	}, bootTimeout, time.Second).Should(BeNumerically(">", consoleSize))

	waitForBoot(provider, microVMPath, mvmID, mvmNS, vmmPid, bootTimeout)

	// flintlockd removes the state directory when the microvm is deleted.
	saveArtefacts(t, microVMPath)

	if params.SkipDelete {
		log.Println("TEST STEP: skipping delete")

		return
	}

	log.Println("TEST STEP: deleting the MicroVM")
	Expect(u.DeleteMVM(flintlockClient, uid)).To(Succeed())

	Eventually(func(g Gomega) error {
		g.Expect(microVMPath).ToNot(BeAnExistingFile())
		g.Expect(socketRoot).ToNot(BeAnExistingFile())
		g.Expect(u.PidRunning(vmmPid)).To(BeFalse())

		res := u.ListMVMs(flintlockClient, mvmNS, nil)
		g.Expect(res.Microvm).To(BeEmpty())

		return nil
	}, "120s").Should(Succeed())
}

func fileSize(dir, name string) int64 {
	info, err := os.Stat(filepath.Join(dir, name))
	Expect(err).NotTo(HaveOccurred())

	return info.Size()
}
