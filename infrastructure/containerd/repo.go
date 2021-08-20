package containerd

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"sync"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/content"
	"github.com/containerd/containerd/namespaces"
	"github.com/opencontainers/go-digest"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"

	"github.com/weaveworks/reignite/core/models"
	"github.com/weaveworks/reignite/core/ports"
	"github.com/weaveworks/reignite/pkg/defaults"
	"github.com/weaveworks/reignite/pkg/log"
)

// NewMicroVMRepo will create a new containerd backed microvm repository with the supplied containerd configuration.
func NewMicroVMRepo(cfg *Config) (ports.MicroVMRepository, error) {
	client, err := containerd.New(cfg.SocketPath)
	if err != nil {
		return nil, fmt.Errorf("creating containerd client: %w", err)
	}

	return NewMicroVMRepoWithClient(client), nil
}

// NewMicroVMRepoWithClient will create a new containerd backed microvm repository with the supplied containerd client.
func NewMicroVMRepoWithClient(client *containerd.Client) ports.MicroVMRepository {
	return &containerdRepo{
		client: client,
		locks:  map[string]*sync.RWMutex{},
	}
}

type containerdRepo struct {
	client *containerd.Client

	locks   map[string]*sync.RWMutex
	locksMu sync.Mutex
}

// Save will save the supplied microvm spec to the containred content store.
func (r *containerdRepo) Save(ctx context.Context, microvm *models.MicroVM) (*models.MicroVM, error) {
	logger := log.GetLogger(ctx).WithField("repo", "containerd_microvm")
	logger.Debugf("saving microvm spec %s/%s", microvm.Namespace, microvm.ID)

	mu := r.getMutex(microvm.ID)
	mu.Lock()
	defer mu.Unlock()

	namespaceCtx := namespaces.WithNamespace(ctx, defaults.ContainerdNamespace)
	store := r.client.ContentStore()

	microvm.Version++

	refName := contentRefName(microvm)
	writer, err := store.Writer(namespaceCtx, content.WithRef(refName))
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

	digest, err := r.findLatestDigestForSpec(namespaceCtx, name, namespace)
	if err != nil {
		return nil, fmt.Errorf("finding content in store: %w", err)
	}
	if digest == nil {
		return nil, errSpecNotFound{name: name, namespace: namespace}
	}

	return r.getWithDigest(namespaceCtx, digest)
}

// GetAll will get a list of microvm details from the containerd content store.
func (r *containerdRepo) GetAll(ctx context.Context, namespace string) ([]*models.MicroVM, error) {
	namespaceCtx := namespaces.WithNamespace(ctx, defaults.ContainerdNamespace)
	store := r.client.ContentStore()

	// NOTE: this seems redundant as we have the namespace based context
	nsLabelFilter := labelFilter(NamespaceLabel, namespace)

	versions := map[string]int{}
	digests := map[string]*digest.Digest{}
	err := store.Walk(namespaceCtx, func(i content.Info) error {
		name := i.Labels[IDLabel]
		version, err := strconv.Atoi(i.Labels[VersionLabel])
		if err != nil {
			return fmt.Errorf("parsing version number: %w", err)
		}

		high, ok := versions[name]
		if !ok {
			high = -1
		}

		if version > high {
			versions[name] = version
			digests[name] = &i.Digest
		}

		return nil
	}, nsLabelFilter)
	if err != nil {
		return nil, fmt.Errorf("walking content store: %w", err)
	}

	items := []*models.MicroVM{}
	for _, d := range digests {
		vm, getErr := r.getWithDigest(namespaceCtx, d)
		if getErr != nil {
			return nil, fmt.Errorf("getting microvm spec: %w", getErr)
		}

		items = append(items, vm)
	}

	return items, nil
}

// Delete will delete the supplied microvm details from the containerd content store.
func (r *containerdRepo) Delete(ctx context.Context, microvm *models.MicroVM) error {
	mu := r.getMutex(microvm.ID)
	mu.Lock()
	defer mu.Unlock()

	namespaceCtx := namespaces.WithNamespace(ctx, defaults.ContainerdNamespace)
	store := r.client.ContentStore()

	digests, err := r.findAllDigestForSpec(namespaceCtx, microvm.ID, microvm.Namespace)
	if err != nil {
		return fmt.Errorf("finding digests for %s: %w", microvm.ID, err)
	}
	if len(digests) == 0 {
		// Ignore not found
		return nil
	}

	for _, d := range digests {
		if err := store.Delete(namespaceCtx, *d); err != nil {
			return fmt.Errorf("deleting content %s from content store: %w", d.String(), err)
		}
	}

	return nil
}

// Exists checks to see if the microvm spec exists in the containerd content store.
func (r *containerdRepo) Exists(ctx context.Context, name, namespace string) (bool, error) {
	mu := r.getMutex(name)
	mu.RLock()
	defer mu.RUnlock()

	namespaceCtx := namespaces.WithNamespace(ctx, defaults.ContainerdNamespace)

	digest, err := r.findLatestDigestForSpec(namespaceCtx, name, namespace)
	if err != nil {
		return false, fmt.Errorf("finding digest for %s/%s: %w", name, namespace, err)
	}
	if digest == nil {
		return false, nil
	}

	return true, nil
}

func (r *containerdRepo) getWithDigest(ctx context.Context, metadigest *digest.Digest) (*models.MicroVM, error) {
	readData, err := content.ReadBlob(ctx, r.client.ContentStore(), v1.Descriptor{
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

func (r *containerdRepo) findLatestDigestForSpec(ctx context.Context, name, namespace string) (*digest.Digest, error) {
	idLabelFilter := labelFilter(IDLabel, name)
	nsFilter := labelFilter(NamespaceLabel, namespace)
	store := r.client.ContentStore()

	var digest *digest.Digest
	highestVersion := 0

	err := store.Walk(ctx, func(i content.Info) error {
		version, err := strconv.Atoi(i.Labels[VersionLabel])
		if err != nil {
			return fmt.Errorf("parsing version number: %w", err)
		}
		if version > highestVersion {
			digest = &i.Digest
			highestVersion = version
		}

		return nil
	}, idLabelFilter, nsFilter)
	if err != nil {
		return nil, fmt.Errorf("walking content store for %s: %w", name, err)
	}

	return digest, nil
}

func (r *containerdRepo) findAllDigestForSpec(ctx context.Context, name, namespace string) ([]*digest.Digest, error) {
	idLabelFilter := labelFilter(IDLabel, name)
	nsLabelFilter := labelFilter(NamespaceLabel, namespace)
	store := r.client.ContentStore()

	digests := []*digest.Digest{}
	err := store.Walk(ctx, func(i content.Info) error {
		digests = append(digests, &i.Digest)

		return nil
	}, idLabelFilter, nsLabelFilter)
	if err != nil {
		return nil, fmt.Errorf("walking content store for %s: %w", name, err)
	}

	return digests, nil
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
		VersionLabel:   strconv.Itoa(microvm.Version),
	}

	return labels
}
