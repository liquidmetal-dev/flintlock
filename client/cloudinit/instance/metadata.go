package instance

// New creates a new instance metadata
func New(opts ...MetadataOption) Metadata {
	m := map[string]string{}

	for _, opt := range opts {
		opt(m)
	}

	return m
}

// Metadata represents the cloud-init instance metadata.
// See https://cloudinit.readthedocs.io/en/latest/topics/instancedata.html
type Metadata map[string]string

// HasItem returns true/false if the specific metadata item exists.
func (m Metadata) HasItem(name string) bool {
	if len(m) == 0 {
		return false
	}
	_, ok := m[name]

	return ok
}

// MetadataOption is an option when creating an instance of Metadata
type MetadataOption func(Metadata)

// WithInstanceID will set the instance id metadata.
func WithInstanceID(instanceID string) MetadataOption {
	return func(im Metadata) {
		im[InstanceIDKey] = instanceID
	}
}

// WithCloudName will set the cloud name metadata.
func WithCloudName(name string) MetadataOption {
	return func(im Metadata) {
		im[CloudNameKey] = name
	}
}

// WithLocalHostname will set the local hostname metadata.
func WithLocalHostname(name string) MetadataOption {
	return func(im Metadata) {
		im[LocalHostnameKey] = name
	}
}

// WithPlatform will set the platform metadata.
func WithPlatform(name string) MetadataOption {
	return func(im Metadata) {
		im[PlatformKey] = name
	}
}

// WithClusterName will set the cluster name metadata.
func WithClusterName(name string) MetadataOption {
	return func(im Metadata) {
		im[ClusterNameKey] = name
	}
}

// WithExisting will set the metadata keys/values based on an existing Metadata.
func WithExisting(existing Metadata) MetadataOption {
	return func(im Metadata) {
		for k, v := range existing {
			im[k] = v
		}
	}
}

// WithKeyValue will set the metadata with the specified key and value.
func WithKeyValue(key, value string) MetadataOption {
	return func(im Metadata) {
		im[key] = value
	}
}
