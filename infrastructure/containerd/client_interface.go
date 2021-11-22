package containerd

import (
	"context"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/api/services/tasks/v1"
	versionservice "github.com/containerd/containerd/api/services/version/v1"
	"github.com/containerd/containerd/containers"
	"github.com/containerd/containerd/content"
	"github.com/containerd/containerd/events"
	"github.com/containerd/containerd/images"
	"github.com/containerd/containerd/leases"
	"github.com/containerd/containerd/namespaces"
	"github.com/containerd/containerd/platforms"
	"github.com/containerd/containerd/services/introspection"
	"github.com/containerd/containerd/snapshots"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health/grpc_health_v1"
)

type Client interface {
	Close() error
	Conn() *grpc.ClientConn
	Containers(ctx context.Context, filters ...string) ([]containerd.Container, error)
	ContainerService() containers.Store
	ContentStore() content.Store
	DiffService() containerd.DiffService
	EventService() containerd.EventService
	Fetch(ctx context.Context, ref string, opts ...containerd.RemoteOpt) (images.Image, error)
	GetImage(ctx context.Context, ref string) (containerd.Image, error)
	GetLabel(ctx context.Context, label string) (string, error)
	GetSnapshotterSupportedPlatforms(ctx context.Context, snapshotterName string) (platforms.MatchComparer, error)
	HealthService() grpc_health_v1.HealthClient
	ImageService() images.Store
	IntrospectionService() introspection.Service
	IsServing(ctx context.Context) (bool, error)
	LeasesService() leases.Manager
	ListImages(ctx context.Context, filters ...string) ([]containerd.Image, error)
	LoadContainer(ctx context.Context, id string) (containerd.Container, error)
	NamespaceService() namespaces.Store
	NewContainer(ctx context.Context, id string, opts ...containerd.NewContainerOpts) (containerd.Container, error)
	Pull(ctx context.Context, ref string, opts ...containerd.RemoteOpt) (containerd.Image, error)
	Push(ctx context.Context, ref string, desc ocispec.Descriptor, opts ...containerd.RemoteOpt) error
	Reconnect() error
	Restore(
		ctx context.Context,
		id string,
		checkpoint containerd.Image,
		opts ...containerd.RestoreOpts,
	) (containerd.Container, error)
	Runtime() string
	Server(ctx context.Context) (containerd.ServerInfo, error)
	SnapshotService(snapshotterName string) snapshots.Snapshotter
	Subscribe(ctx context.Context, filters ...string) (<-chan *events.Envelope, <-chan error)
	TaskService() tasks.TasksClient
	Version(ctx context.Context) (containerd.Version, error)
	VersionService() versionservice.VersionClient
}
