package containerd_test

import (
	"testing"

	"github.com/liquidmetal-dev/flintlock/infrastructure/containerd"
	"github.com/liquidmetal-dev/flintlock/infrastructure/internal/reposcontract"
)

const ctrdRepoNS = "flintlock_test_ctr_repo"

func TestMicroVMRepo_Integration(t *testing.T) {
	if !runContainerDTests() {
		t.Skip("skipping containerd microvm repo integration test")
	}

	client, ctx := testCreateClient(t)

	repo := containerd.NewMicroVMRepoWithClient(&containerd.Config{
		SnapshotterKernel: testSnapshotter,
		SnapshotterVolume: testSnapshotter,
		Namespace:         ctrdRepoNS,
	}, client)

	reposcontract.Run(ctx, t, repo, testOwnerName, testOwnerNamespace)
}

func TestMicroVMRepo_Integration_MultipleSave(t *testing.T) {
	if !runContainerDTests() {
		t.Skip("skipping containerd microvm repo integration multipel save test")
	}

	client, ctx := testCreateClient(t)

	repo := containerd.NewMicroVMRepoWithClient(&containerd.Config{
		SnapshotterKernel: testSnapshotter,
		SnapshotterVolume: testSnapshotter,
		Namespace:         ctrdRepoNS,
	}, client)

	reposcontract.RunMultipleSave(ctx, t, repo, testOwnerName, testOwnerNamespace)
}
