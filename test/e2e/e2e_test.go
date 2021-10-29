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
	"github.com/weaveworks/flintlock/core/models"
	ctr "github.com/weaveworks/flintlock/infrastructure/containerd"
	"github.com/weaveworks/flintlock/test/e2e/utils"
)

func TestE2E(t *testing.T) {
	RegisterTestingT(t)

	var (
		mvmID  = "mvm0"
		mvmNS  = "ns0"
		fcPath = "/var/lib/flintlock/vm/%s/%s"
	)

	// TODO rename
	r := utils.Runner{}
	defer func() {
		log.Println("TEST STEP: cleaning up running processes")
		r.Teardown()
	}()
	log.Println("TEST STEP: performing setup, starting flintlockd server")
	flintlockClient := r.Setup()

	log.Println("TEST STEP: creating MicroVM")
	createReq := v1alpha1.CreateMicroVMRequest{
		Microvm: defaultTestMicroVM(mvmID, mvmNS),
	}
	created, err := flintlockClient.CreateMicroVM(context.Background(), &createReq)
	Expect(err).NotTo(HaveOccurred())
	Expect(created.Microvm.Id).To(Equal(mvmID))

	// So all of this is a placeholder, just to verify that we are creating _something_.
	// Once Get has been implemented, all/most of this can be replaced with 'Eventually' calls
	// to check that the state of the mVM is running (which presumably will be somewhere
	// in our recorded instance state returned by that call).
	log.Println("TEST STEP: getting (and verifying) existing MicroVM")
	repo, err := ctr.NewMicroVMRepo(&ctr.Config{
		SocketPath: utils.ContainerdSocket,
		Namespace:  "flintlock",
	})
	Expect(err).NotTo(HaveOccurred())

	var fcPid int
	Eventually(func(g Gomega) error {
		// verify that the socket exists
		g.Expect(fmt.Sprintf(fcPath, mvmNS, mvmID) + "/firecracker.sock").To(BeAnExistingFile())

		// verify that firecracker has started and that a pid has been saved
		contents, err := os.ReadFile(fmt.Sprintf(fcPath, mvmNS, mvmID) + "/firecracker.pid")
		g.Expect(err).NotTo(HaveOccurred())
		str := string(contents)
		g.Expect(str).ToNot(BeEmpty())

		// verify that there is actually a running process
		// the main check here is actually in delete to ensure we have killed the process
		fcPid, err = strconv.Atoi(str)
		g.Expect(err).NotTo(HaveOccurred())
		p, err := os.FindProcess(fcPid)
		g.Expect(err).NotTo(HaveOccurred())
		Expect(p.Signal(syscall.SIGCONT)).To(Succeed())

		// check state according to containerd
		// I would have liked to call the socket to verify the state of the instance from itself, but
		// unfortunately when I try to do that here it casues both things writing to that socket
		// to fail with "broken pipe".
		// I want to ensure the mVM is _actually_ running before we call delete because
		// otherwise if the delete call runs too early (before Save) then it is lost
		// as we don't have requeueing done yet.
		// So it was either this or a Sleep :D
		model, err := repo.Get(context.Background(), mvmID, mvmNS)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(string(model.Status.State)).To(Equal(models.CreatedState))
		return nil
	}, "120s").Should(Succeed())

	log.Println("TEST STEP: deleting existing MicroVM")
	deleteReq := v1alpha1.DeleteMicroVMRequest{
		Id:        mvmID,
		Namespace: mvmNS,
	}
	_, err = flintlockClient.DeleteMicroVM(context.Background(), &deleteReq)
	Expect(err).NotTo(HaveOccurred())

	Eventually(func(g Gomega) error {
		// verify that the vm state dir has been removed
		g.Expect(fmt.Sprintf(fcPath, mvmNS, mvmID)).ToNot(BeAnExistingFile())

		// verify that the firecracker process is no longer running
		p, err := os.FindProcess(fcPid)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(p.Signal(syscall.SIGCONT)).ToNot(Succeed())

		// verify that the lease has been removed from containerd content store
		_, err = repo.Get(context.Background(), mvmID, mvmNS)
		g.Expect(err).To(MatchError(fmt.Sprintf("microvm spec %s/%s not found", mvmNS, mvmID)))
		return nil
	}, "120s").Should(Succeed())
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
