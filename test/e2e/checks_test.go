//go:build e2e
// +build e2e

package e2e_test

import (
	"fmt"
	"log"
	"strings"
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
// The vmmPid is the pid of the VMM from when the microvm was CREATED. The
// timeout is how long the guest gets, bootTimeout for a guest which boots.
//
// It fails the test when the guest has not booted, which the tests of a
// guest that cannot boot rely on.
func waitForBoot(provider u.Provider, stateDir, name, namespace string, vmmPid int, timeout string) {
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
	}, timeout, bootPolling).Should(Succeed())

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
	saveArtefacts(t, stateDir)

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

// saveArtefacts copies the files of the state directory of a microvm to the
// directory of the artefacts, if the tests were given one. It has to be called
// before the microvm is deleted: flintlockd removes the state directory then.
//
// What cannot be saved is logged and does not fail the test.
func saveArtefacts(t *testing.T, stateDir string) {
	t.Helper()

	if params.ArtefactsDir == "" {
		return
	}

	dest := u.ArtefactsPath(params.ArtefactsDir, t.Name(), stateDir)

	saved, skipped, err := u.SaveStateFiles(stateDir, dest)
	if err != nil {
		log.Printf("TEST INFO: not all of the files of %s are saved: %s", stateDir, err)
	}

	if len(saved) == 0 {
		log.Printf("TEST INFO: %s has no files to save (skipped: %s)", stateDir, strings.Join(skipped, ", "))

		return
	}

	log.Printf("TEST INFO: saved %d files of %s to %s: %s (skipped: %s)",
		len(saved), stateDir, dest, strings.Join(saved, ", "), strings.Join(skipped, ", "))
}
