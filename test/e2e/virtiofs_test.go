//go:build e2e
// +build e2e

package e2e_test

import (
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	. "github.com/onsi/gomega"

	"github.com/liquidmetal-dev/flintlock/api/types"
	u "github.com/liquidmetal-dev/flintlock/test/e2e/utils"
)

// TestE2ECloudHypervisorVirtioFS creates a microvm with the cloudhypervisor
// provider and a virtiofs volume with a mount point: a directory of the host
// which the guest mounts. The guest writes a marker file into the share when
// it boots, and the test reads the file from the directory of the host. That
// is what shows that the share is mounted: a guest which has not mounted it
// writes the file to its root volume, and the host never sees it (#1305).
//
// The test needs virtiofsd on the host, at the path which flintlockd starts
// it from. It is skipped when cloudhypervisor is not one of the providers:
// Firecracker has no virtiofs and refuses a microvm with a virtiofs volume.
func TestE2ECloudHypervisorVirtioFS(t *testing.T) {
	RegisterTestingT(t)

	if environmentLeftRunning {
		t.Skip("a previous test left its environment running, use -run to run this test on its own")
	}

	provider := u.CloudHypervisor()

	if !selected(provider) {
		t.Skipf("the %s provider is not one of the providers of -providers", provider.Name)
	}

	// The files of virtiofsd show why a share does not work.
	provider.DiagnosticFiles = append(provider.DiagnosticFiles, u.VirtioFSDDiagnosticFiles...)

	const (
		mvmID      = "mvm-virtiofs"
		mvmNS      = "ns-virtiofs"
		mountPoint = "/mnt/share"
		statePath  = "/var/lib/flintlock/vm/%s/%s/%s"
		socketDir  = "/run/flintlock"

		// How long the host gets to see the marker after the guest has
		// booted. cloud-init writes the marker before its final message, so
		// the file is there already when the boot marker is on the console.
		markerTimeout = "60s"
		markerPolling = "2s"
	)

	log.Printf("TEST STEP: checking that virtiofsd is at %s", u.VirtioFSDBin)
	version, err := exec.Command(u.VirtioFSDBin, "--version").Output()
	Expect(err).NotTo(HaveOccurred(), "flintlockd starts virtiofsd from %s, which the test needs", u.VirtioFSDBin)
	log.Printf("TEST INFO: %s is %s", u.VirtioFSDBin, strings.TrimSpace(string(version)))

	environmentLeftRunning = params.SkipTeardown || params.SkipDelete

	r := u.NewRunner(params)
	defer func() {
		log.Println("TEST STEP: cleaning up running processes")
		r.Teardown()
	}()
	log.Println("TEST STEP: performing setup, starting flintlockd server")
	flintlockClient := r.Setup()

	// virtiofsd changes its root to the directory, and does not create it.
	share := filepath.Join(u.VirtioFSDir, mvmNS, mvmID)
	Expect(os.MkdirAll(share, 0o755)).To(Succeed())

	// A marker from an earlier run would pass the check below, so the share
	// starts without one. Only the guest of this run can put it back.
	markerFile := filepath.Join(share, u.ShareMarkerFile)

	err = os.Remove(markerFile)
	if !errors.Is(err, os.ErrNotExist) {
		Expect(err).NotTo(HaveOccurred())
	}

	log.Printf("TEST STEP: creating a MicroVM which has the directory %s of the host mounted at %s", share, mountPoint)
	created := u.CreateVirtioFSMVM(flintlockClient, provider, mvmID, mvmNS, share, mountPoint)

	uid := *created.Microvm.Spec.Uid
	microVMPath := fmt.Sprintf(statePath, mvmNS, mvmID, uid)
	socketRoot := filepath.Join(socketDir, uid)

	// The cleanup is set up before anything is checked, so that the microvm is
	// also deleted when the first check fails.
	defer cleanupOnFailure(t, flintlockClient, provider, microVMPath, mvmNS, uid)

	Expect(created.Microvm.Spec.Id).To(Equal(mvmID))
	Expect(created.Microvm.Spec.AdditionalVolumes).To(HaveLen(1))
	Expect(created.Microvm.Spec.AdditionalVolumes[0].Source.VirtiofsSource).To(HaveValue(Equal(share)))
	Expect(created.Microvm.Spec.AdditionalVolumes[0].MountPoint).To(HaveValue(Equal(mountPoint)))

	log.Printf("TEST INFO: MicroVM %s/%s has uid %s and state directory %s", mvmNS, mvmID, uid, microVMPath)

	log.Println("TEST STEP: waiting for the VMM and virtiofsd to run")
	vmmPid, virtiofsdPid := 0, 0

	Eventually(func(g Gomega) error {
		pid, err := u.VerifyVMM(microVMPath, provider)
		g.Expect(err).NotTo(HaveOccurred())

		vmmPid = pid

		pid, err = u.VerifyVirtioFSD(microVMPath, u.VirtioFSDBin)
		g.Expect(err).NotTo(HaveOccurred())

		virtiofsdPid = pid

		res := u.GetMVM(flintlockClient, uid)
		g.Expect(res.Microvm.Status.State).To(Equal(types.MicroVMStatus_CREATED))

		// The VMM gets the share from the socket of virtiofsd, which is under
		// the socket directory of the microvm.
		status := res.Microvm.Status.Volumes[u.VirtioFSVolumeID]
		g.Expect(status).NotTo(BeNil(), "the volume %s has no status", u.VirtioFSVolumeID)
		g.Expect(status.Mount).NotTo(BeNil())
		g.Expect(status.Mount.Source).To(Equal(filepath.Join(socketRoot, u.VirtioFSDSocketFile)))

		info, err := os.Stat(status.Mount.Source)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(info.Mode() & os.ModeSocket).NotTo(BeZero())

		return nil
	}, "120s").Should(Succeed())

	log.Printf("TEST INFO: the VMM has pid %d and virtiofsd has pid %d", vmmPid, virtiofsdPid)

	waitForBoot(provider, microVMPath, mvmID, mvmNS, vmmPid, bootTimeout)

	// The version of cloud-init and what the guest has at the mount point are
	// logged for when the marker is not in the share. The guest writes the
	// mount state to its console before it writes the marker.
	if line, err := u.ConsoleLine(microVMPath, provider, "cloud-init v."); err == nil {
		log.Printf("TEST INFO: the guest has: %s", line)
	}

	// findmnt prints the source, the type and the options of the mount: "user
	// virtiofs rw,relatime". A line with only "virtiofs" could be one of the
	// SSH keys of the guest, which end with its host name.
	if line, err := u.ConsoleLine(microVMPath, provider, "virtiofs rw"); err == nil {
		log.Printf("TEST INFO: the guest has at %s: %s", mountPoint, line)
	} else if line, err := u.ConsoleLine(microVMPath, provider, "flintlock-e2e nothing is mounted"); err == nil {
		log.Printf("TEST INFO: the guest has: %s", line)
	}

	log.Printf("TEST STEP: waiting for the marker which the guest writes into %s", share)
	marker := ""

	Eventually(func(g Gomega) error {
		content, err := u.ShareMarker(share, mvmID, mvmNS)
		g.Expect(err).NotTo(HaveOccurred())

		marker = content

		return nil
	}, markerTimeout, markerPolling).Should(Succeed())

	// The marker could be from an earlier guest, if the VMM was started again.
	Expect(verifySameVMM(microVMPath, provider, vmmPid)).To(Succeed())

	log.Printf("TEST INFO: %s has the marker: %s", markerFile, strings.TrimSpace(marker))

	// flintlockd removes the state directory when the microvm is deleted.
	saveArtefacts(t, microVMPath)

	if params.SkipDelete {
		log.Println("TEST STEP: skipping delete")

		return
	}

	log.Println("TEST STEP: deleting the MicroVM")
	Expect(u.DeleteMVM(flintlockClient, uid)).To(Succeed())

	log.Println("TEST STEP: waiting for the MicroVM to be deleted")
	deleteStart := time.Now()

	Eventually(func(g Gomega) error {
		g.Expect(microVMPath).ToNot(BeAnExistingFile())
		g.Expect(socketRoot).ToNot(BeAnExistingFile())
		g.Expect(u.PidRunning(vmmPid)).To(BeFalse())
		g.Expect(u.PidRunning(virtiofsdPid)).To(BeFalse())

		res := u.ListMVMs(flintlockClient, mvmNS, nil)
		g.Expect(res.Microvm).To(BeEmpty())

		return nil
	}, "120s").Should(Succeed())

	log.Printf("TEST INFO: MicroVM %s/%s is deleted, it took %s",
		mvmNS, mvmID, time.Since(deleteStart).Round(time.Millisecond))

	// The directory is the user's: the delete leaves it and what the guest wrote.
	_, err = u.ShareMarker(share, mvmID, mvmNS)
	Expect(err).NotTo(HaveOccurred(), "the delete did not leave the share as it was")
}
