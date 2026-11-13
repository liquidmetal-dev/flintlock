package firecracker

import (
	"context"
	"net"
	"net/http"

	"github.com/carlmjohnson/requests"
)

const (
	DefaultServerEndpoint = "http://localhost/"

	PathInstanceInfo = "/"
	PathVM           = "/vm"
)

// Client represents a client for the Firecracker API.
type Client interface {
	// Describe returns general information about the Firecracker instance.
	Describe(ctx context.Context) (*InstanceInfo, error)
	// Pause pauses a running microvm.
	Pause(ctx context.Context) error
	// Resume resumes a paused microvm.
	Resume(ctx context.Context) error
}

type client struct {
	builder *requests.Builder
}

// New creates a new Firecracker API client that talks to the API socket at socketPath.
func New(socketPath string) Client {
	t := &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			var d net.Dialer

			return d.DialContext(ctx, "unix", socketPath)
		},
		DisableKeepAlives: true,
	}

	return &client{
		builder: requests.URL(DefaultServerEndpoint).Transport(t).AddValidator(faultValidator),
	}
}

// Describe returns general information about the Firecracker instance.
func (c *client) Describe(ctx context.Context) (*InstanceInfo, error) {
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
