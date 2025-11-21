package containerd

import (
	"context"

	"github.com/containerd/containerd/api/services/tasks/v1"
	versionservice "github.com/containerd/containerd/api/services/version/v1"
	"github.com/containerd/containerd/v2/client"
	"github.com/containerd/containerd/v2/core/containers"
	"github.com/containerd/containerd/v2/core/content"
	"github.com/containerd/containerd/v2/core/events"
	"github.com/containerd/containerd/v2/core/images"
	"github.com/containerd/containerd/v2/core/introspection"
	"github.com/containerd/containerd/v2/core/leases"
	"github.com/containerd/containerd/v2/core/snapshots"
	"github.com/containerd/containerd/v2/pkg/namespaces"
	"github.com/containerd/platforms"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"google.golang.org/grpc/health/grpc_health_v1"
)

type Client interface {
	Close() error
	Conn() any
	Containers(ctx context.Context, filters ...string) ([]client.Container, error)
	ContainerService() containers.Store
	ContentStore() content.Store
	DiffService() client.DiffService
	EventService() client.EventService
	Fetch(ctx context.Context, ref string, opts ...client.RemoteOpt) (images.Image, error)
	GetImage(ctx context.Context, ref string) (client.Image, error)
	GetLabel(ctx context.Context, label string) (string, error)
	GetSnapshotterSupportedPlatforms(ctx context.Context, snapshotterName string) (platforms.MatchComparer, error)
	HealthService() grpc_health_v1.HealthClient
	ImageService() images.Store
	IntrospectionService() introspection.Service
	IsServing(ctx context.Context) (bool, error)
	LeasesService() leases.Manager
	ListImages(ctx context.Context, filters ...string) ([]client.Image, error)
	LoadContainer(ctx context.Context, id string) (client.Container, error)
	NamespaceService() namespaces.Store
	NewContainer(ctx context.Context, id string, opts ...client.NewContainerOpts) (client.Container, error)
	Pull(ctx context.Context, ref string, opts ...client.RemoteOpt) (client.Image, error)
	Push(ctx context.Context, ref string, desc ocispec.Descriptor, opts ...client.RemoteOpt) error
	Reconnect() error
	Restore(
		ctx context.Context,
		id string,
		checkpoint client.Image,
		opts ...client.RestoreOpts,
	) (client.Container, error)
	Runtime() string
	Server(ctx context.Context) (client.ServerInfo, error)
	SnapshotService(snapshotterName string) snapshots.Snapshotter
	Subscribe(ctx context.Context, filters ...string) (<-chan *events.Envelope, <-chan error)
	TaskService() tasks.TasksClient
	Version(ctx context.Context) (client.Version, error)
	VersionService() versionservice.VersionClient
}
