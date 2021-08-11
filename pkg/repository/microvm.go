package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/containerd/containerd/content"
	"github.com/containerd/containerd/namespaces"
	"github.com/opencontainers/go-digest"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"

	"github.com/weaveworks/reignite/pkg/defaults"
	"github.com/weaveworks/reignite/pkg/models"
)

var (
	// IDLabel is the name of the containerd content store label used for the microvm identifier.
	IDLabel = fmt.Sprintf("%s/vmid", defaults.Domain)
	// NamespaceLabel is the name of the containerd content store label used for the microvm namespace.
	NamespaceLabel = fmt.Sprintf("%s/ns", defaults.Domain)
	// TypeLabel is the name of the containerd content store label used to denote the type of content.
	TypeLabel = fmt.Sprintf("%s/type", defaults.Domain)
)

// MicroVMRepository is the repository definition for a microvm repository.
type MicroVMRepository interface {
	// Save will save the supplied microvm spec.
	Save(ctx context.Context, microvm *models.MicroVM) (*models.MicroVM, error)
	// Delete will delete the supplied microvm.
	Delete(ctx context.Context, microvm *models.MicroVM) error
	// Get will get the microvm spec with the given name/namespace.
	Get(ctx context.Context, name, namespace string) (*models.MicroVM, error)
	// GetAll will get a list of microvm details.
	GetAll(ctx context.Context, namespace string) ([]*models.MicroVM, error)
}

// NewContainerdRepository will create a new containerd backed microvm repository.
func NewContainerdRepository(store content.Store) MicroVMRepository {
	return &containerdRepo{
		store: store,
		locks: map[string]*sync.RWMutex{},
	}
}

type containerdRepo struct {
	store content.Store

	locks   map[string]*sync.RWMutex
	locksMu sync.Mutex
}

// Save will save the supplied microvm spec to the containred content store.
func (r *containerdRepo) Save(ctx context.Context, microvm *models.MicroVM) (*models.MicroVM, error) {
	mu := r.getMutex(microvm.ID)
	mu.Lock()
	defer mu.Unlock()

	namespaceCtx := namespaces.WithNamespace(ctx, defaults.ContainerdNamespace)

	microvm.Version++

	refName := fmt.Sprintf("%s/%s", microvm.Namespace, microvm.ID)
	writer, err := r.store.Writer(namespaceCtx, content.WithRef(refName))
	if err != nil {
		return nil, fmt.Errorf("getting containerd writer: %w", err)
	}

	data, err := json.Marshal(microvm)
	if err != nil {
		return nil, fmt.Errorf("marshalling microvm to yaml: %w", err)
	}

	_, err = writer.Write(data)
	if err != nil {
		return nil, fmt.Errorf("writing data to contentd store: %w", err)
	}

	labels := getVMLabels(microvm)
	err = writer.Commit(namespaceCtx, 0, "", content.WithLabels(labels))
	if err != nil {
		return nil, fmt.Errorf("committing content to store: %w", err)
	}

	return microvm, nil
}

// Get will get the microvm spec with the given name/namespace from the containerd content store.
func (r *containerdRepo) Get(ctx context.Context, name, namespace string) (*models.MicroVM, error) {
	mu := r.getMutex(name)
	mu.RLock()
	defer mu.RUnlock()

	namespaceCtx := namespaces.WithNamespace(ctx, defaults.ContainerdNamespace)

	metadigest, err := r.findDigestForSpec(namespaceCtx, name)
	if err != nil {
		return nil, fmt.Errorf("walking content store: %w", err)
	}
	if metadigest == nil {
		return nil, errSpecNotFound{name: name, namespace: namespace}
	}

	return r.getWithDigest(namespaceCtx, metadigest)
}

// GetAll will get a list of microvm details from the containerd content store.
func (r *containerdRepo) GetAll(ctx context.Context, namespace string) ([]*models.MicroVM, error) {
	namespaceCtx := namespaces.WithNamespace(ctx, defaults.ContainerdNamespace)

	items := []*models.MicroVM{}

	nsLabelFilter := fmt.Sprintf("labels.\"%s\"==\"%s\"", NamespaceLabel, namespace)

	err := r.store.Walk(namespaceCtx, func(i content.Info) error {
		vm, getErr := r.getWithDigest(namespaceCtx, &i.Digest)
		if getErr != nil {
			return fmt.Errorf("getting microvm spec: %w", getErr)
		}

		items = append(items, vm)

		return nil
	}, nsLabelFilter)
	if err != nil {
		return nil, fmt.Errorf("walking content store: %w", err)
	}

	return items, nil
}

// Delete will delete the supplied microvm details from the containerd content store.
func (r *containerdRepo) Delete(ctx context.Context, microvm *models.MicroVM) error {
	mu := r.getMutex(microvm.ID)
	mu.Lock()
	defer mu.Unlock()

	namespaceCtx := namespaces.WithNamespace(ctx, defaults.ContainerdNamespace)

	metadigest, err := r.findDigestForSpec(namespaceCtx, microvm.ID)
	if err != nil {
		return fmt.Errorf("finding digest for %s: %w", microvm.ID, err)
	}
	if metadigest == nil {
		// Ignore not found
		return nil
	}

	if err := r.store.Delete(namespaceCtx, *metadigest); err != nil {
		return fmt.Errorf("deleting content %s from content store: %w", metadigest.String(), err)
	}

	return nil
}

func (r *containerdRepo) getWithDigest(ctx context.Context, metadigest *digest.Digest) (*models.MicroVM, error) {
	readData, err := content.ReadBlob(ctx, r.store, v1.Descriptor{
		Digest: *metadigest,
	})
	if err != nil {
		return nil, fmt.Errorf("reading content %s: %w", metadigest, ErrFailedReadingContent)
	}

	microvm := &models.MicroVM{}
	err = json.Unmarshal(readData, microvm)
	if err != nil {
		return nil, fmt.Errorf("unmarshalling json content to microvm: %w", err)
	}

	return microvm, nil
}

func (r *containerdRepo) findDigestForSpec(ctx context.Context, name string) (*digest.Digest, error) {
	var metaDigest digest.Digest

	idLabelFilter := fmt.Sprintf("labels.\"%s\"==\"%s\"", IDLabel, name)

	err := r.store.Walk(ctx, func(i content.Info) error {
		metaDigest = i.Digest

		return nil
	}, idLabelFilter)
	if err != nil {
		return nil, fmt.Errorf("walking content store for %s: %w", name, err)
	}
	if metaDigest.String() == "" {
		return nil, nil
	}

	return &metaDigest, nil
}

func (r *containerdRepo) getMutex(name string) *sync.RWMutex {
	r.locksMu.Lock()
	defer r.locksMu.Unlock()

	namedMu, ok := r.locks[name]
	if ok {
		return namedMu
	}

	mu := &sync.RWMutex{}
	r.locks[name] = mu

	return mu
}

func getVMLabels(microvm *models.MicroVM) map[string]string {
	labels := map[string]string{
		IDLabel:        microvm.ID,
		NamespaceLabel: microvm.Namespace,
		TypeLabel:      "microvm",
	}

	return labels
}
