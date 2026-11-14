package instance

// These constants represent standard instance metadata key names.
const (
	// CloudNameKey is the instance metdata key name representing the cloud name.
	CloudNameKey = "cloud_name"
	// InstanceIDKey is the instance metdata key name representing the unique instance id mof the instance.
	InstanceIDKey = "instance_id"
	// LocalHostnameKey is the instance metdata key name representing the host name of the instance.
	LocalHostnameKey = "local_hostname"
	// PlatformKey is the instance metdata key name representing the hosting platform of the instance.
	PlatformKey = "platform"
)

// These constants represents custom instance metadata names
const (
	// ClusterNameKey is the instance metdata key name representing the cluster name of the instance.
	ClusterNameKey = "cluster_name"
)
