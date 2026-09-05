// Package reposcontract holds a shared behavioural contract test suite for
// ports.MicroVMRepository implementations. Every backing store (containerd,
// sqlite, ...) is expected to satisfy identical semantics, so the test
// bodies live here once and each implementation's test package calls into
// them with its own repository instance. This catches behavioural drift
// between backends automatically.
package reposcontract

import (
	"context"
	"testing"

	. "github.com/onsi/gomega" //nolint:revive,stylecheck // gomega dot-imports are the convention used across this codebase's tests

	"github.com/liquidmetal-dev/flintlock/core/models"
	"github.com/liquidmetal-dev/flintlock/core/ports"
)

// Run exercises the full ports.MicroVMRepository contract (save/version
// increments, get/get-all filtering, delete, exists) against the supplied
// repository. ownerName/ownerNamespace should be unique to the caller so
// concurrent/backend-specific test runs don't collide on shared state.
func Run(ctx context.Context, t *testing.T, repo ports.MicroVMRepository, ownerName, ownerNamespace string) {
	t.Helper()

	var testVM, testVM2 *models.MicroVM

	t.Cleanup(func() {
		if testVM != nil {
			_ = repo.Delete(ctx, testVM)
		}

		if testVM2 != nil {
			_ = repo.Delete(ctx, testVM2)
		}
	})

	RegisterTestingT(t)

	exists, err := repo.Exists(ctx, *models.NewVMIDForce(ownerName, ownerNamespace, "uid"))
	Expect(err).NotTo(HaveOccurred())
	Expect(exists).To(BeFalse())

	testVM = makeSpec(ownerName, ownerNamespace, "uid")
	savedVM, err := repo.Save(ctx, testVM)
	Expect(err).NotTo(HaveOccurred())
	Expect(savedVM).NotTo(BeNil())
	Expect(savedVM.Version).To(Equal(2))

	testVM.Spec.VCPU = 2
	savedVM, err = repo.Save(ctx, testVM)
	Expect(err).NotTo(HaveOccurred())
	Expect(savedVM).NotTo(BeNil())
	Expect(savedVM.Version).To(Equal(3))

	testVM2 = makeSpec("bar-"+ownerName, "foo-"+ownerNamespace, "uid2")
	savedVM2, err := repo.Save(ctx, testVM2)
	Expect(err).NotTo(HaveOccurred())
	Expect(savedVM2).NotTo(BeNil())
	Expect(savedVM2.Version).To(Equal(2))

	exists, err = repo.Exists(ctx, *models.NewVMIDForce(ownerName, ownerNamespace, "uid"))
	Expect(err).NotTo(HaveOccurred())
	Expect(exists).To(BeTrue())

	gotVM, err := repo.Get(ctx, ports.RepositoryGetOptions{
		Name:      ownerName,
		Namespace: ownerNamespace,
		UID:       "uid",
	})
	Expect(err).NotTo(HaveOccurred())
	Expect(gotVM).NotTo(BeNil())
	Expect(gotVM.Version).To(Equal(3))

	olderVM, err := repo.Get(ctx, ports.RepositoryGetOptions{
		Name:      ownerName,
		Namespace: ownerNamespace,
		UID:       "uid",
		Version:   "2",
	})
	Expect(err).NotTo(HaveOccurred())
	Expect(olderVM).NotTo(BeNil())
	Expect(olderVM.Version).To(Equal(2))

	all, err := repo.GetAll(ctx, models.ListMicroVMQuery{"namespace": ownerNamespace})
	Expect(err).NotTo(HaveOccurred())
	Expect(all).To(HaveLen(1))

	all2, err := repo.GetAll(ctx, models.ListMicroVMQuery{})
	Expect(err).NotTo(HaveOccurred())
	Expect(all2).To(HaveLen(2))

	err = repo.Delete(ctx, testVM)
	Expect(err).NotTo(HaveOccurred())

	exists, err = repo.Exists(ctx, *models.NewVMIDForce(ownerName, ownerNamespace, "uid"))
	Expect(err).NotTo(HaveOccurred())
	Expect(exists).To(BeFalse())

	_, err = repo.Get(ctx, ports.RepositoryGetOptions{
		Name:      ownerName,
		Namespace: ownerNamespace,
	})
	Expect(err).To(HaveOccurred())
}

// RunMultipleSave verifies that saving an unchanged spec twice is a no-op
// that does not bump the version.
func RunMultipleSave(
	ctx context.Context, t *testing.T, repo ports.MicroVMRepository, ownerName, ownerNamespace string,
) {
	t.Helper()

	RegisterTestingT(t)

	testVM := makeSpec(ownerName, ownerNamespace, "uid")

	savedVM, err := repo.Save(ctx, testVM)
	Expect(err).NotTo(HaveOccurred())
	Expect(savedVM).NotTo(BeNil())
	Expect(savedVM.Version).To(Equal(2))

	savedVM, err = repo.Save(ctx, testVM)
	Expect(err).NotTo(HaveOccurred())
	Expect(savedVM).NotTo(BeNil())
	Expect(savedVM.Version).To(Equal(2))

	err = repo.Delete(ctx, testVM)
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
