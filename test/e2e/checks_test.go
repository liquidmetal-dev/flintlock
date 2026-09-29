//go:build e2e
// +build e2e

package e2e_test

import (
	"fmt"
	"log"
	"testing"
	"time"

	. "github.com/onsi/gomega"

	"github.com/liquidmetal-dev/flintlock/api/services/microvm/v1alpha1"
	u "github.com/liquidmetal-dev/flintlock/test/e2e/utils"
)

const (
	// The guest has to run all of the stages of cloud-init, which takes longer
	// than it takes flintlockd to start the VMM.
	bootTimeout = "300s"
	bootPolling = "2s"

	failureCleanupTimeout = 90 * time.Second
	failureCleanupPolling = time.Second
	// flintlockd can lose a delete which it gets while it creates the microvm
	// (#1266), which is when a test fails at its first check.
	failureCleanupAgain = 15 * time.Second
)

// waitForBoot waits for the guest of a microvm to write its boot marker to the
// console. That the VMM is running and that the microvm is CREATED does not
// show that the guest has booted: a VMM keeps running when its guest has no
// kernel that it can boot, or when the kernel cannot find its root volume.
//
// The vmmPid is the pid of the VMM from when the microvm was CREATED.
func waitForBoot(provider u.Provider, stateDir, name, namespace string, vmmPid int) {
	log.Printf("TEST STEP: waiting for the guest of MicroVM %s/%s to boot", namespace, name)

	start := time.Now()
	marker := ""

	Eventually(func(g Gomega) error {
		// There is no need to wait for the marker if the VMM has stopped.
		if err := verifySameVMM(stateDir, provider, vmmPid); err != nil {
			return StopTrying("the VMM did not keep running while the guest was booting").Wrap(err)
		}

		line, err := u.ConsoleMarker(stateDir, provider, name, namespace)
		g.Expect(err).NotTo(HaveOccurred())

		marker = line

		return nil
	}, bootTimeout, bootPolling).Should(Succeed())

	// The console is appended to when flintlockd starts the VMM again, so the
	// marker could be from a VMM which has stopped since.
	Expect(verifySameVMM(stateDir, provider, vmmPid)).To(Succeed())

	log.Printf("TEST INFO: the guest of MicroVM %s/%s has booted with %s pid %d, it took %s",
		namespace, name, provider.Name, vmmPid, time.Since(start).Round(time.Millisecond))
	log.Printf("TEST INFO: %s/%s has the line: %s", stateDir, provider.ConsoleFile, marker)
}

func verifySameVMM(stateDir string, provider u.Provider, vmmPid int) error {
	pid, err := u.VerifyVMM(stateDir, provider)
	if err != nil {
		return err
	}

	if pid != vmmPid {
		return fmt.Errorf("the VMM was started again, its pid was %d and is now %d", vmmPid, pid)
	}

	return nil
}

// cleanupOnFailure is for a defer. If the test has failed, it logs the files
// which show why, and deletes the microvm. A VMM is not a child of flintlockd
// and keeps running when flintlockd is stopped. It would hold on to the
// devices of the thinpool, which the teardown then cannot remove.
func cleanupOnFailure(
	t *testing.T,
	client v1alpha1.MicroVMClient,
	provider u.Provider,
	stateDir, namespace, uid string,
) {
	t.Helper()

	if !t.Failed() {
		return
	}

	u.DumpDiagnostics(stateDir, provider)

	if params.SkipDelete || params.SkipTeardown {
		return
	}

	log.Printf("TEST INFO: the test has failed, deleting MicroVM %s", uid)

	// The test has failed already, so there are no assertions from here.
	err := u.DeleteAndWait(client, namespace, uid, stateDir,
		failureCleanupTimeout, failureCleanupPolling, failureCleanupAgain)
	if err != nil {
		log.Printf("TEST INFO: %s", err)

		return
	}

	log.Printf("TEST INFO: MicroVM %s is deleted", uid)
}
