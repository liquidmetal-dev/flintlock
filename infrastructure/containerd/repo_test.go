package containerd_test

import (
	"context"
	"testing"

	ctr "github.com/containerd/containerd"
	. "github.com/onsi/gomega"

	"github.com/liquidmetal-dev/flintlock/core/models"
	"github.com/liquidmetal-dev/flintlock/core/ports"
	"github.com/liquidmetal-dev/flintlock/infrastructure/containerd"
)

const ctrdRepoNS = "flintlock_test_ctr_repo"

func TestMicroVMRepo_Integration(t *testing.T) {
	if !runContainerDTests() {
		t.Skip("skipping containerd microvm repo integration test")
	}

	var (
		repo            ports.MicroVMRepository
		ctx             context.Context
		testVm, testVm2 *models.MicroVM
	)

	t.Cleanup(func() {
		if testVm != nil {
			_ = repo.Delete(ctx, testVm)
		}

		if testVm2 != nil {
			_ = repo.Delete(ctx, testVm2)
		}
	})

	RegisterTestingT(t)

	var client *ctr.Client
	client, ctx = testCreateClient(t)

	repo = containerd.NewMicroVMRepoWithClient(&containerd.Config{
		SnapshotterKernel: testSnapshotter,
		SnapshotterVolume: testSnapshotter,
		Namespace:         ctrdRepoNS,
	}, client)
	exists, err := repo.Exists(ctx, *models.NewVMIDForce(testOwnerName, testOwnerNamespace, testOwnerUID))
	Expect(err).NotTo(HaveOccurred())
	Expect(exists).To(BeFalse())

	testVm = makeSpec(testOwnerName, testOwnerNamespace, "uid")
	savedVM, err := repo.Save(ctx, testVm)
	Expect(err).NotTo(HaveOccurred())
	Expect(savedVM).NotTo(BeNil())
	Expect(savedVM.Version).To(Equal(2))

	testVm.Spec.VCPU = 2
	savedVM, err = repo.Save(ctx, testVm)
	Expect(err).NotTo(HaveOccurred())
	Expect(savedVM).NotTo(BeNil())
	Expect(savedVM.Version).To(Equal(3))

	testVm2 = makeSpec("bar", "foo", "uid2")
	savedVM2, err := repo.Save(ctx, testVm2)
	Expect(err).NotTo(HaveOccurred())
	Expect(savedVM2).NotTo(BeNil())
	Expect(savedVM2.Version).To(Equal(2))

	exists, err = repo.Exists(ctx, *models.NewVMIDForce(testOwnerName, testOwnerNamespace, testOwnerUID))
	Expect(err).NotTo(HaveOccurred())
	Expect(exists).To(BeTrue())

	gotVM, err := repo.Get(ctx, ports.RepositoryGetOptions{
		Name:      testOwnerName,
		Namespace: testOwnerNamespace,
		UID:       "uid",
	})
	Expect(err).NotTo(HaveOccurred())
	Expect(gotVM).NotTo(BeNil())
	Expect(gotVM.Version).To(Equal(3))

	olderVM, err := repo.Get(ctx, ports.RepositoryGetOptions{
		Name:      testOwnerName,
		Namespace: testOwnerNamespace,
		UID:       "uid",
		Version:   "2",
	})
	Expect(err).NotTo(HaveOccurred())
	Expect(olderVM).NotTo(BeNil())
	Expect(olderVM.Version).To(Equal(2))

	all, err := repo.GetAll(ctx, models.ListMicroVMQuery{"namespace": testOwnerNamespace})
	Expect(err).NotTo(HaveOccurred())
	Expect(len(all)).To(Equal(1))

	all2, err := repo.GetAll(ctx, models.ListMicroVMQuery{})
	Expect(err).NotTo(HaveOccurred())
	Expect(len(all2)).To(Equal(2))

	err = repo.Delete(ctx, testVm)
	Expect(err).NotTo(HaveOccurred())

	exists, err = repo.Exists(ctx, *models.NewVMIDForce(testOwnerName, testOwnerNamespace, testOwnerUID))
	Expect(err).NotTo(HaveOccurred())
	Expect(exists).To(BeFalse())

	_, err = repo.Get(ctx, ports.RepositoryGetOptions{
		Name:      testOwnerName,
		Namespace: testOwnerNamespace,
	})
	Expect(err).To(HaveOccurred())
}

func TestMicroVMRepo_Integration_MultipleSave(t *testing.T) {
	if !runContainerDTests() {
		t.Skip("skipping containerd microvm repo integration multipel save test")
	}

	RegisterTestingT(t)

	client, ctx := testCreateClient(t)

	testVm := makeSpec(testOwnerName, testOwnerNamespace, "uid")

	repo := containerd.NewMicroVMRepoWithClient(&containerd.Config{
		SnapshotterKernel: testSnapshotter,
		SnapshotterVolume: testSnapshotter,
		Namespace:         ctrdRepoNS,
	}, client)
	savedVM, err := repo.Save(ctx, testVm)
	Expect(err).NotTo(HaveOccurred())
	Expect(savedVM).NotTo(BeNil())
	Expect(savedVM.Version).To(Equal(2))

	savedVM, err = repo.Save(ctx, testVm)
	Expect(err).NotTo(HaveOccurred())
	Expect(savedVM).NotTo(BeNil())
	Expect(savedVM.Version).To(Equal(2))

	err = repo.Delete(ctx, testVm)
	Expect(err).NotTo(HaveOccurred())
}

func makeSpec(name, ns, uid string) *models.MicroVM {
	vmid, _ := models.NewVMID(name, ns, uid)
	return &models.MicroVM{
		ID:      *vmid,
		Version: 1,
		Spec:    models.MicroVMSpec{},
	}
}
