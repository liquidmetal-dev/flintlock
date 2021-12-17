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
}
