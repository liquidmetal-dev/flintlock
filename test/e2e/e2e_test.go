//go:build e2e
// +build e2e

package e2e_test

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"
	"syscall"
	"testing"

	. "github.com/onsi/gomega"
	"github.com/weaveworks/flintlock/api/services/microvm/v1alpha1"
	"github.com/weaveworks/flintlock/api/types"
	"github.com/weaveworks/flintlock/test/e2e/utils"
)

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

	r := utils.Runner{}
	defer func() {
		log.Println("TEST STEP: cleaning up running processes")
		r.Teardown()
	}()
	log.Println("TEST STEP: performing setup, starting flintlockd server")
	flintlockClient := r.Setup()

	log.Println("TEST STEP: creating MicroVM")
	created := createMVM(flintlockClient, mvmID, mvmNS)
	Expect(created.Microvm.Id).To(Equal(mvmID))

	log.Println("TEST STEP: getting (and verifying) existing MicroVM")
	Eventually(func(g Gomega) error {
		// verify that the socket exists
		g.Expect(fmt.Sprintf(fcPath, mvmNS, mvmID) + "/firecracker.sock").To(BeAnExistingFile())

		// verify that firecracker has started and that a pid has been saved
		// and that there is actually a running process
		mvmPid1 = readPID(fmt.Sprintf(fcPath, mvmNS, mvmID))
		g.Expect(pidRunning(mvmPid1)).To(BeTrue())

		// get the mVM and check the status
		res := getMVM(flintlockClient, mvmID, mvmNS)
		g.Expect(res.Microvm.Spec.Id).To(Equal(mvmID))
		g.Expect(res.Microvm.Status.State).To(Equal(types.MicroVMStatus_CREATED))
		return nil
	}, "120s").Should(Succeed())

	log.Println("TEST STEP: creating a second MicroVM")
	created = createMVM(flintlockClient, secondMvmID, mvmNS)
	Expect(created.Microvm.Id).To(Equal(secondMvmID))

	log.Println("TEST STEP: listing all MicroVMs")
	Eventually(func(g Gomega) error {
		// verify that the new socket exists
		g.Expect(fmt.Sprintf(fcPath, mvmNS, secondMvmID) + "/firecracker.sock").To(BeAnExistingFile())

		// verify that firecracker has started and that a pid has been saved
		// and that there is actually a running process for the new mVM
		mvmPid2 = readPID(fmt.Sprintf(fcPath, mvmNS, secondMvmID))
		g.Expect(pidRunning(mvmPid2)).To(BeTrue())

		// get both the mVMs and check the statuses
		res := listMVMs(flintlockClient, mvmNS)
		g.Expect(res.Microvm).To(HaveLen(2))
		g.Expect(res.Microvm[0].Spec.Id).To(Equal(mvmID))
		g.Expect(res.Microvm[0].Status.State).To(Equal(types.MicroVMStatus_CREATED))
		g.Expect(res.Microvm[1].Spec.Id).To(Equal(secondMvmID))
		g.Expect(res.Microvm[1].Status.State).To(Equal(types.MicroVMStatus_CREATED))
		return nil
	}, "120s").Should(Succeed())

	log.Println("TEST STEP: deleting existing MicroVMs")
	Expect(deleteMVM(flintlockClient, mvmID, mvmNS)).To(Succeed())
	Expect(deleteMVM(flintlockClient, secondMvmID, mvmNS)).To(Succeed())

	Eventually(func(g Gomega) error {
		// verify that the vm state dirs have been removed
		g.Expect(fmt.Sprintf(fcPath, mvmNS, mvmID)).ToNot(BeAnExistingFile())
		g.Expect(fmt.Sprintf(fcPath, mvmNS, secondMvmID)).ToNot(BeAnExistingFile())

		// verify that the firecracker processes are no longer running
		g.Expect(pidRunning(mvmPid1)).To(BeFalse())
		g.Expect(pidRunning(mvmPid2)).To(BeFalse())

		// verify that the mVMs are no longer with us
		res := listMVMs(flintlockClient, mvmNS)
		g.Expect(res.Microvm).To(HaveLen(0))
		return nil
	}, "120s").Should(Succeed())
}

func createMVM(client v1alpha1.MicroVMClient, name, ns string) *v1alpha1.CreateMicroVMResponse {
	createReq := v1alpha1.CreateMicroVMRequest{
		Microvm: defaultTestMicroVM(name, ns),
	}
	created, err := client.CreateMicroVM(context.Background(), &createReq)
	Expect(err).NotTo(HaveOccurred())

	return created
}

func deleteMVM(client v1alpha1.MicroVMClient, name, ns string) error {
	deleteReq := v1alpha1.DeleteMicroVMRequest{
		Id:        name,
		Namespace: ns,
	}
	_, err := client.DeleteMicroVM(context.Background(), &deleteReq)

	return err
}

func getMVM(client v1alpha1.MicroVMClient, name, ns string) *v1alpha1.GetMicroVMResponse {
	getReq := v1alpha1.GetMicroVMRequest{
		Id:        name,
		Namespace: ns,
	}
	res, err := client.GetMicroVM(context.Background(), &getReq)
	Expect(err).NotTo(HaveOccurred())

	return res
}

func listMVMs(client v1alpha1.MicroVMClient, ns string) *v1alpha1.ListMicroVMsResponse {
	listReq := v1alpha1.ListMicroVMsRequest{
		Namespace: ns,
	}
	resp, err := client.ListMicroVMs(context.Background(), &listReq)
	Expect(err).NotTo(HaveOccurred())

	return resp
}

func readPID(path string) int {
	contents, err := os.ReadFile(path + "/firecracker.pid")
	Expect(err).NotTo(HaveOccurred())
	str := string(contents)
	Expect(str).ToNot(BeEmpty())

	pid, err := strconv.Atoi(str)
	Expect(err).NotTo(HaveOccurred())

	return pid
}

func pidRunning(pid int) bool {
	p, err := os.FindProcess(pid)
	Expect(err).NotTo(HaveOccurred())
	if err := p.Signal(syscall.SIGCONT); err != nil {
		return false
	}

	return true
}

func defaultTestMicroVM(name, namespace string) *types.MicroVMSpec {
	var (
		kernelImage = "docker.io/richardcase/ubuntu-bionic-kernel:0.0.11"
		cloudImage  = "docker.io/richardcase/ubuntu-bionic-test:cloudimage_v0.0.1"
	)

	return &types.MicroVMSpec{
		Id:         name,
		Namespace:  namespace,
		Vcpu:       2,
		MemoryInMb: 2048,
		Kernel: &types.Kernel{
			Image:            kernelImage,
			Cmdline:          "console=ttyS0 reboot=k panic=1 pci=off i8042.noaux i8042.nomux i8042.nopnp i8042.dumbkbd ds=nocloud-net;s=http://169.254.169.254/latest/",
			Filename:         pointyString("vmlinux"),
			AddNetworkConfig: true,
		},
		Initrd: &types.Initrd{
			Image:    kernelImage,
			Filename: pointyString("initrd-generic"),
		},
		Volumes: []*types.Volume{{
			Id:         "root",
			IsRoot:     false,
			IsReadOnly: true,
			MountPoint: "/",
			Source: &types.VolumeSource{
				ContainerSource: pointyString(cloudImage),
			}},
		},
		Interfaces: []*types.NetworkInterface{{
			GuestDeviceName:  "eth0",
			Type:             1,
			AllowMetadataReq: true,
			GuestMac:         pointyString("AA:FF:00:00:00:01"),
			Address:          pointyString("169.254.0.1/16"),
		},
			{
				GuestDeviceName:  "eth1",
				Type:             0,
				AllowMetadataReq: false,
			}},
		Metadata: map[string]string{
			"meta-data": "aW5zdGFuY2VfaWQ6IG5zMS9tdm0wCmxvY2FsX2hvc3RuYW1lOiBtdm0wCnBsYXRmb3JtOiBsaXF1aWRfbWV0YWwK",
			"user-data": "I2Nsb3VkLWNvbmZpZwpob3N0bmFtZTogbXZtMApmcWRuOiBtdm0wLmZydWl0Y2FzZQp1c2VyczoKICAgIC0gbmFtZTogcm9vdAogICAgICBzc2hfYXV0aG9yaXplZF9rZXlzOgogICAgICAgIC0gfAogICAgICAgICAgc3NoLWVkMjU1MTkgQUFBQUMzTnphQzFsWkRJMU5URTVBQUFBSUdzbStWSSsyVk5WWFBDRmVmbFhrQTVKY21zMzByajFGUFFjcFNTdDFrdVYgcmljaGFyZEB3ZWF2ZS53b3JrcwpkaXNhYmxlX3Jvb3Q6IGZhbHNlCnBhY2thZ2VfdXBkYXRlOiBmYWxzZQpmaW5hbF9tZXNzYWdlOiBUaGUgcmVpZ25pdGVkIGJvb3RlZCBzeXN0ZW0gaXMgZ29vZCB0byBnbyBhZnRlciAkVVBUSU1FIHNlY29uZHMKcnVuY21kOgogICAgLSBkaGNsaWVudCAtcgogICAgLSBkaGNsaWVudAo=",
		},
	}
}

func pointyString(v string) *string {
	return &v
}
