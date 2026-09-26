package containerd

const (
	supportedSnapshotters = "overlayfs,native,devmapper"
)

// Config holds the containerd configuration.
type Config struct {
	// SnapshotterKernel is the name of the containerd snapshotter to use for kernel images.
	SnapshotterKernel string
	// SnapshotterVolume is the name of the containerd snapshotter to use for volume (inc initrd) images.
	SnapshotterVolume string
	// SocketPath is the path to the containerd socket.
	SocketPath string
	// Namespace is the default containerd namespace to use
	Namespace string
	// HostsDir is the root of a containerd certs.d layout
	// (<HostsDir>/<registry-host>/hosts.toml) that flintlock's in-process
	// resolver reads to configure auth and mirrors for image pulls. It defaults
	// to the directory the containerd daemon itself reads, so registry config
	// done the containerd way applies to flintlock too. A registry with no
	// hosts.toml, or a missing directory, falls back to containerd's built-in
	// https defaults. An empty value disables per-registry configuration and
	// pulls anonymously using the client library defaults.
	HostsDir string
}
