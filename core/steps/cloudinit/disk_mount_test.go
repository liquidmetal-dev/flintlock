package cloudinit_test

import (
	"context"
	"encoding/base64"
	"testing"

	g "github.com/onsi/gomega"
	"gopkg.in/yaml.v2"

	"github.com/weaveworks-liquidmetal/flintlock/client/cloudinit/userdata"
	"github.com/weaveworks-liquidmetal/flintlock/core/models"
	"github.com/weaveworks-liquidmetal/flintlock/core/steps/cloudinit"
)

func TestDiskMount_HappyPath(t *testing.T) {
	g.RegisterTestingT(t)

	ctx := context.Background()
	vm := createMicrovm()
	step := cloudinit.NewDiskMountStep(vm, "vdb2", "/opt/data")

	shouldDo, err := step.ShouldDo(ctx)
	g.Expect(err).NotTo(g.HaveOccurred())
	g.Expect(shouldDo).To(g.BeTrue())

	subSteps, err := step.Do(ctx)
	g.Expect(err).NotTo(g.HaveOccurred())
	g.Expect(subSteps).To(g.HaveLen(0))

	g.Expect(vm.Spec.Metadata.Items).To(g.HaveLen(1))
	g.Expect(vm.Spec.Metadata.Items).To(g.HaveKey("vendor-data"))

	metaValue := vm.Spec.Metadata.Items["vendor-data"]
	g.Expect(metaValue).ToNot(g.BeEmpty())

	data, err := base64.StdEncoding.DecodeString(metaValue)
	g.Expect(err).NotTo(g.HaveOccurred())

	vendorData := &userdata.UserData{}
	err = yaml.Unmarshal(data, vendorData)
	g.Expect(err).NotTo(g.HaveOccurred())
	g.Expect(vendorData.Mounts).To(g.HaveLen(1))

	mount := vendorData.Mounts[0]
	g.Expect(mount).To(g.HaveLen(2))
	g.Expect(mount[0]).To(g.Equal("vdb2"))
	g.Expect(mount[1]).To(g.Equal("/opt/data"))
}

func TestDiskMount_ShouldNotDo(t *testing.T) {
	g.RegisterTestingT(t)

	ctx := context.Background()
	vm := createMicrovm()

	step := cloudinit.NewDiskMountStep(vm, "", "/opt/data")
	shouldDo, err := step.ShouldDo(ctx)
	g.Expect(err).NotTo(g.HaveOccurred())
	g.Expect(shouldDo).To(g.BeFalse())

	step = cloudinit.NewDiskMountStep(vm, "vdb2", "")
	shouldDo, err = step.ShouldDo(ctx)
	g.Expect(err).NotTo(g.HaveOccurred())
	g.Expect(shouldDo).To(g.BeFalse())

	step = cloudinit.NewDiskMountStep(vm, "", "")
	shouldDo, err = step.ShouldDo(ctx)
	g.Expect(err).NotTo(g.HaveOccurred())
	g.Expect(shouldDo).To(g.BeFalse())
}

func createMicrovm() *models.MicroVM {
	vmid, _ := models.NewVMID("vm", "ns", "uid")
	return &models.MicroVM{
		ID:      *vmid,
		Version: 1,
		Spec: models.MicroVMSpec{
			Metadata: models.Metadata{
				Items:     map[string]string{},
				AddVolume: false,
			},
		},
	}
}
