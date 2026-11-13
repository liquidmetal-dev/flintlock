package local_test

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	. "github.com/onsi/gomega"

	coreerrs "github.com/liquidmetal-dev/flintlock/core/errors"
	"github.com/liquidmetal-dev/flintlock/core/models"
	"github.com/liquidmetal-dev/flintlock/core/ports"
	"github.com/liquidmetal-dev/flintlock/infrastructure/snapshot/local"
)

const (
	testNamespace = "ns1"
	testVMName    = "vm1"
	testVMUID     = "01HZ0000000000000000000000"
	restoredVMUID = "01HZ0000000000000000000001"
	otherVMUID    = "01HZ0000000000000000000002"
)

var testTime = time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)

func newStore(t *testing.T, root string) *local.Store {
	t.Helper()

	store, err := local.New(context.Background(), root)
	if err != nil {
		t.Fatalf("creating store: %v", err)
	}

	return store
}

func newSnapshot(name string) *models.Snapshot {
	return &models.Snapshot{
		Name:      name,
		Source:    *models.NewVMIDForce(testVMName, testNamespace, testVMUID),
		Labels:    map[string]string{"env": "test"},
		CreatedAt: testTime,
	}
}

func mode(g *WithT, path string) os.FileMode {
	info, err := os.Stat(path)
	g.Expect(err).NotTo(HaveOccurred())

	return info.Mode().Perm()
}

func TestCreate_Modes(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()
	root := t.TempDir()
	store := newStore(t, root)

	snapshot := newSnapshot("snap")
	g.Expect(store.Create(ctx, snapshot)).To(Succeed())
	g.Expect(store.AddRef(ctx, snapshot.UID, restoredVMUID)).To(Succeed())

	dir := store.Dir(snapshot.UID)
	g.Expect(dir).To(Equal(filepath.Join(root, "snapshots", snapshot.UID)))

	g.Expect(mode(g, filepath.Join(root, "snapshots"))).To(Equal(os.FileMode(0o700)))
	g.Expect(mode(g, dir)).To(Equal(os.FileMode(0o700)))
	g.Expect(mode(g, filepath.Join(dir, "refs"))).To(Equal(os.FileMode(0o700)))
	g.Expect(mode(g, filepath.Join(dir, "record.json"))).To(Equal(os.FileMode(0o600)))
	g.Expect(mode(g, filepath.Join(dir, ".lock"))).To(Equal(os.FileMode(0o600)))
	g.Expect(mode(g, filepath.Join(dir, "refs", restoredVMUID))).To(Equal(os.FileMode(0o600)))
}

func TestCreate_Defaults(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()
	store := newStore(t, t.TempDir())

	snapshot := newSnapshot("")
	g.Expect(store.Create(ctx, snapshot)).To(Succeed())

	g.Expect(snapshot.UID).NotTo(BeEmpty())
	g.Expect(snapshot.Name).To(Equal("vm1-20260102-030405"))
	g.Expect(snapshot.Namespace).To(Equal(testNamespace))
	g.Expect(snapshot.Status.Phase).To(Equal(models.SnapshotPhasePending))

	got, err := store.Get(ctx, models.SnapshotRef{UID: snapshot.UID})
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(got.Name).To(Equal("vm1-20260102-030405"))
	g.Expect(got.Source.UID()).To(Equal(testVMUID))
	g.Expect(got.Labels).To(Equal(map[string]string{"env": "test"}))
	g.Expect(got.CreatedAt.Equal(testTime)).To(BeTrue())
}

func TestCreate_NameClash(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()
	store := newStore(t, t.TempDir())

	g.Expect(store.Create(ctx, newSnapshot(""))).To(Succeed())

	err := store.Create(ctx, newSnapshot(""))
	g.Expect(coreerrs.IsSnapshotAlreadyExists(err)).To(BeTrue(), "generated name clash: %v", err)

	named := newSnapshot("snap")
	g.Expect(store.Create(ctx, named)).To(Succeed())

	err = store.Create(ctx, newSnapshot("snap"))
	g.Expect(coreerrs.IsSnapshotAlreadyExists(err)).To(BeTrue(), "name clash: %v", err)

	otherNamespace := newSnapshot("snap")
	otherNamespace.Namespace = "ns2"
	g.Expect(store.Create(ctx, otherNamespace)).To(Succeed())

	sameUID := newSnapshot("other")
	sameUID.UID = named.UID
	err = store.Create(ctx, sameUID)
	g.Expect(coreerrs.IsSnapshotAlreadyExists(err)).To(BeTrue(), "uid clash: %v", err)

	g.Expect(store.Delete(ctx, named.UID)).To(Succeed())
	g.Expect(store.Create(ctx, newSnapshot("snap"))).To(Succeed(), "name of a deleted snapshot is free")
}

func TestCreate_InvalidUID(t *testing.T) {
	g := NewWithT(t)
	store := newStore(t, t.TempDir())

	snapshot := newSnapshot("snap")
	snapshot.UID = "../escape"
	g.Expect(store.Create(context.Background(), snapshot)).NotTo(Succeed())
}

func TestGet(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()
	store := newStore(t, t.TempDir())

	snapshot := newSnapshot("snap")
	g.Expect(store.Create(ctx, snapshot)).To(Succeed())

	byUID, err := store.Get(ctx, models.SnapshotRef{UID: snapshot.UID})
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(byUID.Name).To(Equal("snap"))

	byName, err := store.Get(ctx, models.SnapshotRef{Name: "snap", Namespace: testNamespace})
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(byName.UID).To(Equal(snapshot.UID))

	_, err = store.Get(ctx, models.SnapshotRef{Name: "snap", Namespace: "ns2"})
	g.Expect(coreerrs.IsSnapshotNotFound(err)).To(BeTrue())

	_, err = store.Get(ctx, models.SnapshotRef{UID: "1b4e28ba-2fa1-11d2-883f-0016d3cca427"})
	g.Expect(coreerrs.IsSnapshotNotFound(err)).To(BeTrue())

	_, err = store.Get(ctx, models.SnapshotRef{UID: "../.."})
	g.Expect(coreerrs.IsSnapshotNotFound(err)).To(BeTrue())
}

func TestUpdate(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()
	store := newStore(t, t.TempDir())

	snapshot := newSnapshot("snap")
	g.Expect(store.Create(ctx, snapshot)).To(Succeed())

	snapshot.Status.Phase = models.SnapshotPhaseFailed
	snapshot.Status.Error = "boom"
	g.Expect(store.Update(ctx, snapshot)).To(Succeed())

	got, err := store.Get(ctx, models.SnapshotRef{UID: snapshot.UID})
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(got.Status.Phase).To(Equal(models.SnapshotPhaseFailed))
	g.Expect(got.Status.Error).To(Equal("boom"))
	g.Expect(got.Status.LocalPath).To(BeEmpty())

	snapshot.Status = models.SnapshotStatus{Phase: models.SnapshotPhaseReady}
	g.Expect(store.Update(ctx, snapshot)).To(Succeed())

	got, err = store.Get(ctx, models.SnapshotRef{UID: snapshot.UID})
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(got.Status.Phase).To(Equal(models.SnapshotPhaseReady))
	g.Expect(got.Status.LocalPath).To(Equal(store.Dir(snapshot.UID)))
}

func TestList(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()
	store := newStore(t, t.TempDir())

	first := newSnapshot("first")
	g.Expect(store.Create(ctx, first)).To(Succeed())

	second := newSnapshot("second")
	second.Source = *models.NewVMIDForce("vm2", testNamespace, otherVMUID)
	second.Labels = map[string]string{"env": "prod"}
	second.CreatedAt = testTime.Add(time.Second)
	g.Expect(store.Create(ctx, second)).To(Succeed())

	third := newSnapshot("third")
	third.Namespace = "ns2"
	g.Expect(store.Create(ctx, third)).To(Succeed())

	names := func(options ports.SnapshotListOptions) []string {
		snapshots, err := store.List(ctx, options)
		g.Expect(err).NotTo(HaveOccurred())

		out := []string{}
		for _, snapshot := range snapshots {
			out = append(out, snapshot.Name)
		}

		return out
	}

	g.Expect(names(ports.SnapshotListOptions{})).To(ConsistOf("first", "second", "third"))
	g.Expect(names(ports.SnapshotListOptions{Namespace: testNamespace})).To(Equal([]string{"first", "second"}))
	g.Expect(names(ports.SnapshotListOptions{SourceUID: otherVMUID})).To(Equal([]string{"second"}))
	g.Expect(names(ports.SnapshotListOptions{Labels: map[string]string{"env": "test"}})).
		To(ConsistOf("first", "third"))

	g.Expect(store.Delete(ctx, first.UID)).To(Succeed())
	g.Expect(names(ports.SnapshotListOptions{Namespace: testNamespace})).To(Equal([]string{"second"}))
}

func TestNextGeneration_Concurrent(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()
	store := newStore(t, t.TempDir())

	snapshot := newSnapshot("snap")
	g.Expect(store.Create(ctx, snapshot)).To(Succeed())

	const callers = 50

	var (
		wg     sync.WaitGroup
		mu     sync.Mutex
		values = map[int]int{}
		errs   []error
	)

	for range callers {
		wg.Add(1)

		go func() {
			defer wg.Done()

			generation, err := store.NextGeneration(ctx, snapshot.UID)

			mu.Lock()
			defer mu.Unlock()

			if err != nil {
				errs = append(errs, err)

				return
			}

			values[generation]++
		}()
	}

	wg.Wait()

	g.Expect(errs).To(BeEmpty())
	g.Expect(values).To(HaveLen(callers))

	for generation := 1; generation <= callers; generation++ {
		g.Expect(values[generation]).To(Equal(1), "generation %d", generation)
	}
}

func TestDelete_NoRefs(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()
	store := newStore(t, t.TempDir())

	snapshot := newSnapshot("snap")
	g.Expect(store.Create(ctx, snapshot)).To(Succeed())
	g.Expect(store.Delete(ctx, snapshot.UID)).To(Succeed())

	g.Expect(store.Dir(snapshot.UID)).NotTo(BeADirectory())

	_, err := store.Get(ctx, models.SnapshotRef{UID: snapshot.UID})
	g.Expect(coreerrs.IsSnapshotNotFound(err)).To(BeTrue())

	err = store.Delete(ctx, snapshot.UID)
	g.Expect(coreerrs.IsSnapshotNotFound(err)).To(BeTrue())
}

func TestDelete_WithRefs(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()
	store := newStore(t, t.TempDir())

	snapshot := newSnapshot("snap")
	g.Expect(store.Create(ctx, snapshot)).To(Succeed())

	dir := store.Dir(snapshot.UID)
	for _, name := range []string{"vmstate", "memory", "descriptor.json", "spec.json", "root.rw"} {
		g.Expect(os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o600)).To(Succeed())
	}

	g.Expect(os.Symlink("../../bases/abc.img", filepath.Join(dir, "root.base"))).To(Succeed())

	g.Expect(store.AddRef(ctx, snapshot.UID, restoredVMUID)).To(Succeed())
	g.Expect(store.AddRef(ctx, snapshot.UID, otherVMUID)).To(Succeed())
	g.Expect(store.Delete(ctx, snapshot.UID)).To(Succeed())

	entries, err := os.ReadDir(dir)
	g.Expect(err).NotTo(HaveOccurred())

	kept := []string{}
	for _, entry := range entries {
		kept = append(kept, entry.Name())
	}

	g.Expect(kept).To(ConsistOf(".lock", "memory", "record.json", "refs", "vmstate"))

	_, err = store.Get(ctx, models.SnapshotRef{UID: snapshot.UID})
	g.Expect(coreerrs.IsSnapshotNotFound(err)).To(BeTrue())

	_, err = store.Get(ctx, models.SnapshotRef{Name: "snap", Namespace: testNamespace})
	g.Expect(coreerrs.IsSnapshotNotFound(err)).To(BeTrue())

	err = store.AddRef(ctx, snapshot.UID, "01HZ0000000000000000000003")
	g.Expect(coreerrs.IsSnapshotNotFound(err)).To(BeTrue(), "no new reference to a deleted snapshot")

	_, err = store.NextGeneration(ctx, snapshot.UID)
	g.Expect(coreerrs.IsSnapshotNotFound(err)).To(BeTrue())

	g.Expect(store.ReleaseRef(ctx, snapshot.UID, restoredVMUID)).To(Succeed())
	g.Expect(dir).To(BeADirectory())

	g.Expect(store.ReleaseRef(ctx, snapshot.UID, otherVMUID)).To(Succeed())
	g.Expect(dir).NotTo(BeADirectory())
}

func TestReleaseRef_NotDeletedKeepsDir(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()
	store := newStore(t, t.TempDir())

	snapshot := newSnapshot("snap")
	g.Expect(store.Create(ctx, snapshot)).To(Succeed())
	g.Expect(store.AddRef(ctx, snapshot.UID, restoredVMUID)).To(Succeed())
	g.Expect(store.ReleaseRef(ctx, snapshot.UID, restoredVMUID)).To(Succeed())
	g.Expect(store.ReleaseRef(ctx, snapshot.UID, restoredVMUID)).To(Succeed(), "release is idempotent")

	g.Expect(filepath.Join(store.Dir(snapshot.UID), "refs", restoredVMUID)).NotTo(BeAnExistingFile())
	g.Expect(store.Dir(snapshot.UID)).To(BeADirectory())
}

func TestAddRef_InvalidVMUID(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()
	store := newStore(t, t.TempDir())

	snapshot := newSnapshot("snap")
	g.Expect(store.Create(ctx, snapshot)).To(Succeed())

	for _, vmUID := range []string{"", ".", "..", "../x", "a/b"} {
		g.Expect(store.AddRef(ctx, snapshot.UID, vmUID)).NotTo(Succeed(), "vm uid %q", vmUID)
	}
}

func TestRestart_ListsSameRecords(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()
	root := t.TempDir()
	store := newStore(t, root)

	first := newSnapshot("first")
	g.Expect(store.Create(ctx, first)).To(Succeed())

	second := newSnapshot("second")
	g.Expect(store.Create(ctx, second)).To(Succeed())

	second.Status.Phase = models.SnapshotPhaseReady
	g.Expect(store.Update(ctx, second)).To(Succeed())

	generation, err := store.NextGeneration(ctx, second.UID)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(generation).To(Equal(1))

	before, err := store.List(ctx, ports.SnapshotListOptions{})
	g.Expect(err).NotTo(HaveOccurred())

	restarted := newStore(t, root)

	after, err := restarted.List(ctx, ports.SnapshotListOptions{})
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(after).To(HaveLen(2))
	g.Expect(after).To(Equal(before))

	generation, err = restarted.NextGeneration(ctx, second.UID)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(generation).To(Equal(2))
}

func TestRestart_RemovesLeftOvers(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()
	root := t.TempDir()
	store := newStore(t, root)

	kept := newSnapshot("kept")
	g.Expect(store.Create(ctx, kept)).To(Succeed())

	interrupted := filepath.Join(root, "snapshots", "1b4e28ba-2fa1-11d2-883f-0016d3cca427")
	g.Expect(os.MkdirAll(filepath.Join(interrupted, "refs"), 0o700)).To(Succeed())
	g.Expect(os.WriteFile(filepath.Join(interrupted, "vmstate"), []byte("x"), 0o600)).To(Succeed())

	restarted := newStore(t, root)

	g.Expect(interrupted).NotTo(BeADirectory())
	g.Expect(restarted.Dir(kept.UID)).To(BeADirectory())

	snapshots, err := restarted.List(ctx, ports.SnapshotListOptions{})
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(snapshots).To(HaveLen(1))
	g.Expect(snapshots[0].UID).To(Equal(kept.UID))
}
