package containerd_test

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"testing"

	"github.com/containerd/containerd/content"
	"github.com/containerd/containerd/content/local"
	. "github.com/onsi/gomega"
	"github.com/opencontainers/go-digest"

	"github.com/weaveworks/reignite/core/models"
	"github.com/weaveworks/reignite/infrastructure/repositories/microvm/containerd"
)

func TestMicroVMRepo_SaveAndGet(t *testing.T) {
	testCases := []struct {
		name          string
		existingSpecs []*models.MicroVM
		specToGet     string
		expectErr     bool
	}{
		{
			name:          "empty",
			existingSpecs: []*models.MicroVM{},
			specToGet:     "test1",
			expectErr:     true,
		},
		{
			name: "has existing entry",
			existingSpecs: []*models.MicroVM{
				makeSpec("test1", "ns1"),
				makeSpec("test2", "ns1"),
			},
			specToGet: "test1",
			expectErr: false,
		},
		{
			name: "existing entries but no matching spec name",
			existingSpecs: []*models.MicroVM{
				makeSpec("test1", "ns1"),
				makeSpec("test2", "ns1"),
			},
			specToGet: "test3",
			expectErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			RegisterTestingT(t)

			ctx := context.Background()

			store := getLocalContentStore(t)

			repo := containerd.New(store)

			for _, specToAdd := range tc.existingSpecs {
				_, saveErr := repo.Save(ctx, specToAdd)
				Expect(saveErr).NotTo(HaveOccurred())
			}

			mvm, err := repo.Get(ctx, tc.specToGet, "ns1")

			if tc.expectErr {
				Expect(err).To(HaveOccurred())
			} else {
				Expect(err).NotTo(HaveOccurred())
				Expect(mvm).ToNot(BeNil())
				Expect(mvm.ID).To(Equal(tc.specToGet))
			}
		})
	}
}

func TestMicroVMRepo_Delete(t *testing.T) {
	testCases := []struct {
		name          string
		existingSpecs []*models.MicroVM
		specToDelete  string
		expectErr     bool
	}{
		{
			name:          "empty",
			existingSpecs: []*models.MicroVM{},
			specToDelete:  "test1",
			expectErr:     false,
		},
		{
			name: "has existing entry",
			existingSpecs: []*models.MicroVM{
				makeSpec("test1", "ns1"),
				makeSpec("test2", "ns1"),
			},
			specToDelete: "test1",
			expectErr:    false,
		},
		{
			name: "existing entries but no matching spec",
			existingSpecs: []*models.MicroVM{
				makeSpec("test1", "ns1"),
				makeSpec("test2", "ns1"),
			},
			specToDelete: "test3",
			expectErr:    false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			RegisterTestingT(t)

			ctx := context.Background()

			store := getLocalContentStore(t)

			repo := containerd.New(store)

			for _, specToAdd := range tc.existingSpecs {
				_, saveErr := repo.Save(ctx, specToAdd)
				Expect(saveErr).NotTo(HaveOccurred())
			}

			err := repo.Delete(ctx, makeSpec(tc.specToDelete, "ns1"))

			if tc.expectErr {
				Expect(err).To(HaveOccurred())
			} else {
				Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}

func TestMicroVMRepo_GetAll(t *testing.T) {
	testCases := []struct {
		name             string
		existingSpecs    []*models.MicroVM
		nsToGet          string
		expectErr        bool
		expectedNumItems int
	}{
		{
			name:             "empty",
			existingSpecs:    []*models.MicroVM{},
			nsToGet:          "ns1",
			expectErr:        false,
			expectedNumItems: 0,
		},
		{
			name: "has existing entry",
			existingSpecs: []*models.MicroVM{
				makeSpec("test1", "ns1"),
				makeSpec("test2", "ns1"),
			},
			nsToGet:          "ns1",
			expectErr:        false,
			expectedNumItems: 2,
		},
		{
			name: "different ns - has existing entry",
			existingSpecs: []*models.MicroVM{
				makeSpec("test1", "ns1"),
				makeSpec("test2", "ns2"),
			},
			nsToGet:          "ns1",
			expectErr:        false,
			expectedNumItems: 1,
		},
		{
			name: "existing entries but no matching spec",
			existingSpecs: []*models.MicroVM{
				makeSpec("test1", "ns1"),
				makeSpec("test2", "ns1"),
			},
			nsToGet:          "ns2",
			expectErr:        false,
			expectedNumItems: 0,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			RegisterTestingT(t)

			ctx := context.Background()

			store := getLocalContentStore(t)

			repo := containerd.New(store)

			for _, specToAdd := range tc.existingSpecs {
				_, saveErr := repo.Save(ctx, specToAdd)
				Expect(saveErr).NotTo(HaveOccurred())
			}

			items, err := repo.GetAll(ctx, tc.nsToGet)

			if tc.expectErr {
				Expect(err).To(HaveOccurred())
			} else {
				Expect(err).NotTo(HaveOccurred())
				Expect(items).ToNot(BeNil())
				Expect(len(items)).To(Equal(tc.expectedNumItems))
			}
		})
	}
}

func getLocalContentStore(t *testing.T) content.Store {
	contentDir, err := ioutil.TempDir(os.TempDir(), "reignite-store-")
	if err != nil {
		t.Fatal(err)
	}
	blobsDir := fmt.Sprintf("%s/blobs", contentDir)
	err = os.Mkdir(blobsDir, os.ModePerm)
	if err != nil {
		t.Fatal(err)
	}

	store, err := local.NewLabeledStore(contentDir, newInmemoryLabelStore())
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_ = os.RemoveAll(contentDir)
	})

	return store
}

func makeSpec(name, namespace string) *models.MicroVM {
	return &models.MicroVM{
		ID:        name,
		Namespace: namespace,
		Version:   1,
		Spec:      models.MicroVMSpec{},
	}
}

func newInmemoryLabelStore() *inMemoryLabelStore {
	return &inMemoryLabelStore{
		labels: map[string]map[string]string{},
	}
}

type inMemoryLabelStore struct {
	labels map[string]map[string]string
}

func (ls *inMemoryLabelStore) Get(d digest.Digest) (map[string]string, error) {
	labels, ok := ls.labels[d.String()]
	if ok {
		return labels, nil
	}

	return map[string]string{}, nil
}

func (ls *inMemoryLabelStore) Set(d digest.Digest, labelsToSet map[string]string) error {
	ls.labels[d.String()] = labelsToSet

	return nil
}

func (ls *inMemoryLabelStore) Update(d digest.Digest, labelsToUpdate map[string]string) (map[string]string, error) {
	labels, ok := ls.labels[d.String()]
	if !ok {
		ls.labels[d.String()] = labelsToUpdate
		return ls.labels[d.String()], nil
	}

	// Add / update any labels
	for k, v := range labelsToUpdate {
		if v == "" {
			delete(labels, k)
		} else {
			labels[k] = v
		}
	}

	ls.labels[d.String()] = labels

	return labels, nil
}
