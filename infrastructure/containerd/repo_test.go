package containerd_test

import (
	"testing"

	. "github.com/onsi/gomega"

	"github.com/weaveworks/flintlock/core/models"
	"github.com/weaveworks/flintlock/core/ports"
	"github.com/weaveworks/flintlock/infrastructure/containerd"
)

func TestMicroVMRepo_Integration(t *testing.T) {
	if !runContainerDTests() {
		t.Skip("skipping containerd microvm repo integration test")
	}

	RegisterTestingT(t)

	client, ctx := testCreateClient(t)

	repo := containerd.NewMicroVMRepoWithClient(&containerd.Config{
		SnapshotterKernel: testSnapshotter,
		SnapshotterVolume: testSnapshotter,
		Namespace:         testContainerdNs,
	}, client)
	exists, err := repo.Exists(ctx, testOwnerName, testOwnerNamespace)
	Expect(err).NotTo(HaveOccurred())
	Expect(exists).To(BeFalse())

	testVm := makeSpec(testOwnerName, testOwnerNamespace)
	savedVM, err := repo.Save(ctx, testVm)
	Expect(err).NotTo(HaveOccurred())
	Expect(savedVM).NotTo(BeNil())
	Expect(savedVM.Version).To(Equal(2))

	testVm.Spec.VCPU = 2
	savedVM, err = repo.Save(ctx, testVm)
	Expect(err).NotTo(HaveOccurred())
	Expect(savedVM).NotTo(BeNil())
	Expect(savedVM.Version).To(Equal(3))

	exists, err = repo.Exists(ctx, testOwnerName, testOwnerNamespace)
	Expect(err).NotTo(HaveOccurred())
	Expect(exists).To(BeTrue())

	gotVM, err := repo.Get(ctx, ports.RepositoryGetOptions{
		Name:      testOwnerName,
		Namespace: testOwnerNamespace,
	})
	Expect(err).NotTo(HaveOccurred())
	Expect(gotVM).NotTo(BeNil())
	Expect(gotVM.Version).To(Equal(3))

	olderVM, err := repo.Get(ctx, ports.RepositoryGetOptions{
		Name:      testOwnerName,
		Namespace: testOwnerNamespace,
		Version:   "2",
	})
	Expect(err).NotTo(HaveOccurred())
	Expect(olderVM).NotTo(BeNil())
	Expect(olderVM.Version).To(Equal(2))

	all, err := repo.GetAll(ctx, testOwnerNamespace)
	Expect(err).NotTo(HaveOccurred())
	Expect(len(all)).To(Equal(1))

	err = repo.Delete(ctx, testVm)
	Expect(err).NotTo(HaveOccurred())

	exists, err = repo.Exists(ctx, testOwnerName, testOwnerNamespace)
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

	testVm := makeSpec(testOwnerName, testOwnerNamespace)

	repo := containerd.NewMicroVMRepoWithClient(&containerd.Config{
		SnapshotterKernel: testSnapshotter,
		SnapshotterVolume: testSnapshotter,
		Namespace:         testContainerdNs,
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

func makeSpec(name, ns string) *models.MicroVM {
	vmid, _ := models.NewVMID(name, ns)
	return &models.MicroVM{
		ID:      *vmid,
		Version: 1,
		Spec:    models.MicroVMSpec{},
	}
}
