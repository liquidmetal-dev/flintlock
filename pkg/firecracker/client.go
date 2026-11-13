package firecracker

import (
	"context"
	"net"
	"net/http"
	"time"

	"github.com/carlmjohnson/requests"
)

const (
	DefaultServerEndpoint = "http://localhost/"

	PathInstanceInfo   = "/"
	PathVM             = "/vm"
	PathSnapshotCreate = "/snapshot/create"
	PathSnapshotLoad   = "/snapshot/load"
	PathMMDS           = "/mmds"

	DefaultDescribeTimeout = 2 * time.Second
)

// Client represents a client for the Firecracker API.
type Client interface {
	// Describe returns general information about the Firecracker instance.
	Describe(ctx context.Context) (*InstanceInfo, error)
	// Pause pauses a running microvm.
	Pause(ctx context.Context) error
	// Resume resumes a paused microvm.
	Resume(ctx context.Context) error
	// CreateSnapshot writes a snapshot of a paused microvm.
	CreateSnapshot(ctx context.Context, params SnapshotCreateParams) error
	// LoadSnapshot loads a snapshot into a Firecracker process that has no microvm yet.
	LoadSnapshot(ctx context.Context, params SnapshotLoadParams) error
	// PutMMDS replaces the data store of the metadata service with data.
	PutMMDS(ctx context.Context, data any) error
}

type client struct {
	builder         *requests.Builder
	describeTimeout time.Duration
}

// Option configures a Client.
type Option func(*client)

// WithDescribeTimeout sets how long Describe waits for Firecracker to answer.
func WithDescribeTimeout(timeout time.Duration) Option {
	return func(c *client) {
		c.describeTimeout = timeout
	}
}

// New creates a new Firecracker API client that talks to the API socket at socketPath.
func New(socketPath string, opts ...Option) Client {
	t := &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			var d net.Dialer

			return d.DialContext(ctx, "unix", socketPath)
		},
		DisableKeepAlives: true,
	}

	c := &client{
		builder:         requests.URL(DefaultServerEndpoint).Transport(t).AddValidator(faultValidator),
		describeTimeout: DefaultDescribeTimeout,
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

// Describe returns general information about the Firecracker instance. It gives
// up after the describe timeout so that a socket which accepts but never answers
// does not block a state check.
func (c *client) Describe(ctx context.Context) (*InstanceInfo, error) {
	ctx, cancel := context.WithTimeout(ctx, c.describeTimeout)
	defer cancel()

	info := &InstanceInfo{}

	if err := c.builder.Clone().Path(PathInstanceInfo).ToJSON(info).Fetch(ctx); err != nil {
		return nil, err
	}

	return info, nil
}

// Pause pauses a running microvm.
func (c *client) Pause(ctx context.Context) error {
	return c.setVMState(ctx, VMStatePaused)
}

// Resume resumes a paused microvm.
func (c *client) Resume(ctx context.Context) error {
	return c.setVMState(ctx, VMStateResumed)
}

func (c *client) setVMState(ctx context.Context, state VMState) error {
	return c.builder.Clone().Path(PathVM).Patch().BodyJSON(&VM{State: state}).Fetch(ctx)
}

// CreateSnapshot writes a snapshot of a paused microvm.
func (c *client) CreateSnapshot(ctx context.Context, params SnapshotCreateParams) error {
	return c.builder.Clone().Path(PathSnapshotCreate).Put().BodyJSON(&params).Fetch(ctx)
}

// LoadSnapshot loads a snapshot into a Firecracker process that has no microvm yet.
func (c *client) LoadSnapshot(ctx context.Context, params SnapshotLoadParams) error {
	return c.builder.Clone().Path(PathSnapshotLoad).Put().BodyJSON(&params).Fetch(ctx)
}

// PutMMDS replaces the data store of the metadata service with data.
func (c *client) PutMMDS(ctx context.Context, data any) error {
	return c.builder.Clone().Path(PathMMDS).Put().BodyJSON(data).Fetch(ctx)
}
