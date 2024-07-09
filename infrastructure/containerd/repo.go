package containerd

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/content"
	"github.com/containerd/containerd/namespaces"
	"github.com/google/go-cmp/cmp"
	"github.com/liquidmetal-dev/flintlock/core/errors"
	"github.com/liquidmetal-dev/flintlock/core/models"
	"github.com/liquidmetal-dev/flintlock/core/ports"
	"github.com/liquidmetal-dev/flintlock/pkg/log"
	"github.com/opencontainers/go-digest"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
)

// NewMicroVMRepo will create a new containerd backed microvm repository with the supplied containerd configuration.
func NewMicroVMRepo(cfg *Config) (ports.MicroVMRepository, error) {
	client, err := containerd.New(cfg.SocketPath)
	if err != nil {
		return nil, fmt.Errorf("creating containerd client: %w", err)
	}

	return NewMicroVMRepoWithClient(cfg, client), nil
}

// NewMicroVMRepoWithClient will create a new containerd backed microvm repository with the supplied containerd client.
func NewMicroVMRepoWithClient(cfg *Config, client *containerd.Client) ports.MicroVMRepository {
	return &containerdRepo{
		client: client,
		config: cfg,
		locks:  map[string]*sync.RWMutex{},
	}
}

type containerdRepo struct {
	client *containerd.Client
	config *Config

	locks   map[string]*sync.RWMutex
	locksMu sync.Mutex
}

// Save will save the supplied microvm spec to the containred content store.
func (r *containerdRepo) Save(ctx context.Context, microvm *models.MicroVM) (*models.MicroVM, error) {
	logger := log.GetLogger(ctx).WithField("repo", "containerd_microvm")
	logger.Debugf("saving microvm spec %s", microvm.ID)

	mu := r.getMutex(microvm.ID.String())
	mu.Lock()
	defer mu.Unlock()

	existingSpec, err := r.get(ctx, ports.RepositoryGetOptions{
		Name:      microvm.ID.Name(),
		Namespace: microvm.ID.Namespace(),
		UID:       microvm.ID.UID(),
	})
	if err != nil {
		return nil, fmt.Errorf("getting vm spec from store: %w", err)
	}

	if existingSpec != nil {
		specDiff := cmp.Diff(existingSpec.Spec, microvm.Spec)
		statusDiff := cmp.Diff(existingSpec.Status, microvm.Status)

		if specDiff == "" && statusDiff == "" {
			logger.Debug("microvm specs have no diff, skipping save")

			return existingSpec, nil
		}
	}

	namespaceCtx := namespaces.WithNamespace(ctx, r.config.Namespace)

	leaseCtx, err := withOwnerLease(namespaceCtx, microvm.ID.String(), r.client)
	if err != nil {
		return nil, fmt.Errorf("getting lease for owner: %w", err)
	}

	store := r.client.ContentStore()

	microvm.Version++

	refName := contentRefName(microvm)

	writer, err := store.Writer(leaseCtx, content.WithRef(refName))
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
// If version is not empty, returns with the specified version of the spec.
func (r *containerdRepo) Get(ctx context.Context, options ports.RepositoryGetOptions) (*models.MicroVM, error) {
	mu := r.getMutex(options.Name)
	mu.RLock()
	defer mu.RUnlock()

	spec, err := r.get(ctx, options)
	if err != nil {
		return nil, fmt.Errorf("getting vm spec from store: %w", err)
	}

	if spec == nil {
		return nil, errors.NewSpecNotFound( //nolint: wrapcheck // No need to wrap this error
			options.Name,
			options.Namespace,
			options.Version,
			options.UID)
	}

	return spec, nil
}

// GetAll will get a list of microvm details from the containerd content store.
func (r *containerdRepo) GetAll(ctx context.Context, query models.ListMicroVMQuery) ([]*models.MicroVM, error) {
	namespaceCtx := namespaces.WithNamespace(ctx, r.config.Namespace)
	store := r.client.ContentStore()
	filters := []string{labelFilter(TypeLabel(), MicroVMSpecType)}
	versions := map[string]int{}
	digests := map[string]*digest.Digest{}

	filters = append(filters, convertQueryToFilter(query)...)

	andFilters := strings.Join(filters, ",")

	err := store.Walk(namespaceCtx, func(info content.Info) error {
		key := info.Labels[UIDLabel()]
		version, err := strconv.Atoi(info.Labels[VersionLabel()])
		if err != nil {
			return fmt.Errorf("parsing version number: %w", err)
		}

		high, ok := versions[key]
		if !ok {
			high = -1
		}

		if version > high {
			versions[key] = version
			digests[key] = &info.Digest
		}

		return nil
	}, andFilters)
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

// ReleaseLease will release the supplied lease.
func (r *containerdRepo) ReleaseLease(ctx context.Context, microvm *models.MicroVM) error {
	mu := r.getMutex(microvm.ID.String())

	mu.Lock()
	defer mu.Unlock()

	namespaceCtx := namespaces.WithNamespace(ctx, r.config.Namespace)

	return deleteLease(namespaceCtx, microvm.ID.String(), r.client)
}

// Delete will delete the supplied microvm details from the containerd content store.
func (r *containerdRepo) Delete(ctx context.Context, microvm *models.MicroVM) error {
	mu := r.getMutex(microvm.ID.String())
	mu.Lock()
	defer mu.Unlock()

	namespaceCtx := namespaces.WithNamespace(ctx, r.config.Namespace)
	store := r.client.ContentStore()

	digests, err := r.findAllDigestForSpec(namespaceCtx, microvm.ID.Name(), microvm.ID.Namespace())
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
func (r *containerdRepo) Exists(ctx context.Context, vmid models.VMID) (bool, error) {
	mu := r.getMutex(vmid.Name())
	mu.RLock()
	defer mu.RUnlock()

	namespaceCtx := namespaces.WithNamespace(ctx, r.config.Namespace)

	digest, err := r.findDigestForSpec(
		namespaceCtx,
		ports.RepositoryGetOptions{Name: vmid.Name(), Namespace: vmid.Namespace(), UID: vmid.UID()},
	)
	if err != nil {
		return false, fmt.Errorf(
			"finding digest for %s/%s/%s: %w", vmid.Name(), vmid.Namespace(), vmid.UID(), err)
	}

	if digest == nil {
		return false, nil
	}

	return true, nil
}

func (r *containerdRepo) get(ctx context.Context, options ports.RepositoryGetOptions) (*models.MicroVM, error) {
	namespaceCtx := namespaces.WithNamespace(ctx, r.config.Namespace)

	digest, err := r.findDigestForSpec(namespaceCtx, options)
	if err != nil {
		return nil, fmt.Errorf("finding content in store: %w", err)
	}

	if digest == nil {
		return nil, nil
	}

	return r.getWithDigest(namespaceCtx, digest)
}

func (r *containerdRepo) getWithDigest(ctx context.Context, metadigest *digest.Digest) (*models.MicroVM, error) {
	readData, err := content.ReadBlob(ctx, r.client.ContentStore(), v1.Descriptor{
		Digest: *metadigest,
	})
	if err != nil {
		return nil, fmt.Errorf("reading content %s: %w", metadigest, ErrReadingContent)
	}

	microvm := &models.MicroVM{}

	err = json.Unmarshal(readData, microvm)
	if err != nil {
		return nil, fmt.Errorf("unmarshalling json content to microvm: %w", err)
	}

	return microvm, nil
}

func (r *containerdRepo) findDigestForSpec(ctx context.Context,
	options ports.RepositoryGetOptions,
) (*digest.Digest, error) {
	var digest *digest.Digest

	combinedFilters := []string{}

	if options.Name != "" {
		combinedFilters = append(combinedFilters, labelFilter(NameLabel(), options.Name))
	}

	if options.Namespace != "" {
		combinedFilters = append(combinedFilters, labelFilter(NamespaceLabel(), options.Namespace))
	}

	if options.UID != "" {
		combinedFilters = append(combinedFilters, labelFilter(UIDLabel(), options.UID))
	}

	if options.Version != "" {
		combinedFilters = append(combinedFilters, labelFilter(VersionLabel(), options.Version))
	}

	allFilters := strings.Join(combinedFilters, ",")
	store := r.client.ContentStore()
	highestVersion := 0

	err := store.Walk(
		ctx,
		func(info content.Info) error {
			version, err := strconv.Atoi(info.Labels[VersionLabel()])
			if err != nil {
				return fmt.Errorf("parsing version number: %w", err)
			}

			if version > highestVersion {
				digest = &info.Digest
				highestVersion = version
			}

			return nil
		},
		allFilters,
	)
	if err != nil {
		return nil, fmt.Errorf("walking content store for %s: %w", options.Name, err)
	}

	return digest, nil
}

func (r *containerdRepo) findAllDigestForSpec(ctx context.Context, name, namespace string) ([]*digest.Digest, error) {
	store := r.client.ContentStore()
	idLabelFilter := labelFilter(NameLabel(), name)
	nsLabelFilter := labelFilter(NamespaceLabel(), namespace)
	combinedFilters := []string{idLabelFilter, nsLabelFilter}
	allFilters := strings.Join(combinedFilters, ",")
	digests := []*digest.Digest{}

	err := store.Walk(
		ctx,
		func(i content.Info) error {
			digests = append(digests, &i.Digest)

			return nil
		},
		allFilters,
	)
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
		NameLabel():      microvm.ID.Name(),
		NamespaceLabel(): microvm.ID.Namespace(),
		TypeLabel():      MicroVMSpecType,
		VersionLabel():   strconv.Itoa(microvm.Version),
		UIDLabel():       microvm.ID.UID(),
	}

	return labels
}

func convertQueryToFilter(query models.ListMicroVMQuery) []string {
	filters := []string{}

	for key, value := range query {
		if value == "" {
			continue
		}

		if key == "namespace" {
			filters = append(filters, labelFilter(NamespaceLabel(), value))

			continue
		}

		if key == "name" {
			filters = append(filters, labelFilter(NameLabel(), value))

			continue
		}

		filters = append(filters, labelFilter(key, value))
	}

	return filters
}
