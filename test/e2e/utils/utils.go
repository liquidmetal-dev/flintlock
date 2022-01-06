//go:build e2e
// +build e2e

package utils

import (
	"context"
	"os"
	"strconv"
	"syscall"

	g "github.com/onsi/gomega"

	"github.com/weaveworks/flintlock/api/services/microvm/v1alpha1"
	"github.com/weaveworks/flintlock/api/types"
)

func CreateMVM(client v1alpha1.MicroVMClient, name, ns string) *v1alpha1.CreateMicroVMResponse {
	createReq := v1alpha1.CreateMicroVMRequest{
		Microvm: defaultTestMicroVM(name, ns),
	}
	created, err := client.CreateMicroVM(context.Background(), &createReq)
	g.Expect(err).NotTo(g.HaveOccurred())

	return created
}

func DeleteMVM(client v1alpha1.MicroVMClient, name, ns string) error {
	deleteReq := v1alpha1.DeleteMicroVMRequest{
		Id:        name,
		Namespace: ns,
	}
	_, err := client.DeleteMicroVM(context.Background(), &deleteReq)

	return err
}

func GetMVM(client v1alpha1.MicroVMClient, name, ns string) *v1alpha1.GetMicroVMResponse {
	getReq := v1alpha1.GetMicroVMRequest{
		Id:        name,
		Namespace: ns,
	}
	res, err := client.GetMicroVM(context.Background(), &getReq)
	g.Expect(err).NotTo(g.HaveOccurred())

	return res
}

func ListMVMs(client v1alpha1.MicroVMClient, ns string) *v1alpha1.ListMicroVMsResponse {
	listReq := v1alpha1.ListMicroVMsRequest{
		Namespace: ns,
	}
	resp, err := client.ListMicroVMs(context.Background(), &listReq)
	g.Expect(err).NotTo(g.HaveOccurred())

	return resp
}

func ReadPID(path string) int {
	contents, err := os.ReadFile(path + "/firecracker.pid")
	g.Expect(err).NotTo(g.HaveOccurred())
	str := string(contents)
	g.Expect(str).ToNot(g.BeEmpty())

	pid, err := strconv.Atoi(str)
	g.Expect(err).NotTo(g.HaveOccurred())

	return pid
}

func PidRunning(pid int) bool {
	p, err := os.FindProcess(pid)
	g.Expect(err).NotTo(g.HaveOccurred())
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
		Vcpu:       2,    //nolint: gomnd
		MemoryInMb: 2048, //nolint: gomnd
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
		RootVolume: &types.Volume{
			Id:         "root",
			IsReadOnly: true,
			MountPoint: "/",
			Source: &types.VolumeSource{
				ContainerSource: pointyString(cloudImage),
			},
		},
		Interfaces: []*types.NetworkInterface{
			{
				DeviceId: "eth0",
				Type:     0,
			},
		},
		Metadata: map[string]string{
			"meta-data": "aW5zdGFuY2VfaWQ6IG5zMS9tdm0wCmxvY2FsX2hvc3RuYW1lOiBtdm0wCnBsYXRmb3JtOiBsaXF1aWRfbWV0YWwK",
			"user-data": "I2Nsb3VkLWNvbmZpZwpob3N0bmFtZTogbXZtMApmcWRuOiBtdm0wLmZydWl0Y2FzZQp1c2VyczoKICAgIC0gbmFtZTogcm9vdAogICAgICBzc2hfYXV0aG9yaXplZF9rZXlzOgogICAgICAgIC0gfAogICAgICAgICAgc3NoLWVkMjU1MTkgQUFBQUMzTnphQzFsWkRJMU5URTVBQUFBSUdzbStWSSsyVk5WWFBDRmVmbFhrQTVKY21zMzByajFGUFFjcFNTdDFrdVYgcmljaGFyZEB3ZWF2ZS53b3JrcwpkaXNhYmxlX3Jvb3Q6IGZhbHNlCnBhY2thZ2VfdXBkYXRlOiBmYWxzZQpmaW5hbF9tZXNzYWdlOiBUaGUgcmVpZ25pdGVkIGJvb3RlZCBzeXN0ZW0gaXMgZ29vZCB0byBnbyBhZnRlciAkVVBUSU1FIHNlY29uZHMKcnVuY21kOgogICAgLSBkaGNsaWVudCAtcgogICAgLSBkaGNsaWVudAo=",
		},
	}
}

func pointyString(v string) *string {
	return &v
}
