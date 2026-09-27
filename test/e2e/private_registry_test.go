//go:build e2e
// +build e2e

package e2e_test

import (
	"fmt"
	"log"
	"net/http"
	"testing"
	"time"

	. "github.com/onsi/gomega"

	"github.com/liquidmetal-dev/flintlock/api/types"
	u "github.com/liquidmetal-dev/flintlock/test/e2e/utils"
)

// TestE2EPrivateRegistry does its own setup so that containerd starts without
// any content. If the images were already in the content store, flintlockd
// would not need to pull them from the registry.
func TestE2EPrivateRegistry(t *testing.T) {
	RegisterTestingT(t)

	if environmentLeftRunning {
		t.Skip("a previous test left its environment running, use -run to run this test on its own")
	}

	var (
		mvmID  = "mvm-private"
		mvmNS  = "ns-private"
		fcPath = "/var/lib/flintlock/vm/%s/%s/%s"

		mvmPid int
	)

	leaveRunning := params.SkipTeardown || params.SkipDelete
	environmentLeftRunning = leaveRunning

	r := u.NewRunner(params)
	defer func() {
		log.Println("TEST STEP: cleaning up running processes")
		r.Teardown()
	}()
	log.Println("TEST STEP: performing setup, starting flintlockd server")
	flintlockClient := r.Setup()

	registry := u.NewRegistry(u.RegistryDir)
	defer func() {
		if leaveRunning {
			return
		}

		log.Println("TEST STEP: stopping the private registry")
		registry.Stop()
	}()
	log.Println("TEST STEP: starting the private registry")
	registry.Start()

	log.Println("TEST STEP: copying the images to the private registry")
	kernelImage := registry.Seed(u.DefaultKernelImage)
	rootImage := registry.Seed(u.DefaultRootImage)

	log.Println("TEST STEP: verifying that the private registry requires authentication")
	for _, image := range []string{kernelImage, rootImage} {
		Expect(registry.ManifestStatus(image, false)).To(Equal(http.StatusUnauthorized))
		Expect(registry.ManifestStatus(image, true)).To(Equal(http.StatusOK))
	}

	log.Println("TEST STEP: configuring the credentials for the private registry")
	registry.WriteHostsConfig(u.HostsDir)

	log.Println("TEST STEP: creating a MicroVM using images from the private registry")
	created := u.CreateMVMWithImages(flintlockClient, mvmID, mvmNS, kernelImage, rootImage)
	Expect(created.Microvm.Spec.Id).To(Equal(mvmID))
	Expect(created.Microvm.Spec.Kernel.Image).To(Equal(kernelImage))
	Expect(*created.Microvm.Spec.RootVolume.Source.ContainerSource).To(Equal(rootImage))

	microVMPath := fmt.Sprintf(fcPath, mvmNS, mvmID, *created.Microvm.Spec.Uid)
	log.Printf("TEST INFO: MicroVM %s/%s has uid %s and state directory %s",
		mvmNS, mvmID, *created.Microvm.Spec.Uid, microVMPath)

	log.Println("TEST STEP: waiting for the MicroVM to be created")
	createStart := time.Now()
	lastSeen := ""

	Eventually(func(g Gomega) error {
		// The function is called many times a second, so the status is only
		// logged when it changes.
		res := u.GetMVM(flintlockClient, *created.Microvm.Spec.Uid)
		seen := fmt.Sprintf("%s (retry %d)", res.Microvm.Status.State, res.Microvm.Status.Retry)
		if seen != lastSeen {
			log.Printf("TEST INFO: MicroVM %s/%s is %s", mvmNS, mvmID, seen)
			lastSeen = seen
		}

		g.Expect(microVMPath + "/firecracker.pid").To(BeAnExistingFile())

		// verify that firecracker has started and that a pid has been saved
		// and that there is actually a running process
		mvmPid = u.ReadPID(microVMPath)
		g.Expect(u.PidRunning(mvmPid)).To(BeTrue())

		// check the status
		g.Expect(res.Microvm.Status.State).To(Equal(types.MicroVMStatus_CREATED))

		return nil
	}, "120s").Should(Succeed())

	log.Printf("TEST INFO: MicroVM %s/%s is running with firecracker pid %d, it took %s",
		mvmNS, mvmID, mvmPid, time.Since(createStart).Round(time.Millisecond))

	log.Println("TEST STEP: verifying that the images were pulled from the private registry")
	for _, image := range []string{kernelImage, rootImage} {
		requests := registry.Requests(image, u.ContainerdUserAgent)

		// The manifest shows that the reference was resolved by the registry, and
		// the blobs show that the content of the image came from it.
		Expect(requests).To(ContainElement(And(
			HaveField("Path", u.ManifestPath(image)),
			HaveField("Status", http.StatusOK),
		)), "the manifest of %s was not pulled from the registry", image)
		Expect(requests).To(ContainElement(And(
			HaveField("Method", http.MethodGet),
			HaveField("Path", ContainSubstring("/blobs/")),
			HaveField("Status", http.StatusOK),
		)), "the blobs of %s were not pulled from the registry", image)
		Expect(requests).NotTo(ContainElement(
			HaveField("Status", BeElementOf(http.StatusUnauthorized, http.StatusForbidden)),
		), "the registry refused a request for %s", image)
	}

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
		g.Expect(u.PidRunning(mvmPid)).To(BeFalse())

		res := u.ListMVMs(flintlockClient, mvmNS, nil)
		g.Expect(res.Microvm).To(BeEmpty())

		return nil
	}, "120s").Should(Succeed())

	log.Printf("TEST INFO: MicroVM %s/%s is deleted, it took %s",
		mvmNS, mvmID, time.Since(deleteStart).Round(time.Millisecond))
}
