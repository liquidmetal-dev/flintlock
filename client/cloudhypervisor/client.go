package cloudhypervisor

import (
	"context"
	"net"
	"net/http"

	"github.com/carlmjohnson/requests"
)

type Client interface {
	Ping(ctx context.Context) (*VmmPingResponse, error)
	Info(ctx context.Context) (*VmInfo, error)
	//Counters(ctx context.Context) *VmmCo
	Delete(ctx context.Context) error
	Boot(ctx context.Context) error
	Pause(ctx context.Context) error
	Resume(ctx context.Context) error
	Shutdown(ctx context.Context) error
	Reboot(ctx context.Context) error
	PowerButton(ctx context.Context) error
	Create(ctx context.Context, config *VmConfig) error
	//Resize(ctx context.Context, config *VmResize) error
}

type client struct {
	builder *requests.Builder
}

func New(socketPath string) Client {
	t := &http.Transport{
		DialContext: func(_ context.Context, _, _ string) (net.Conn, error) {
			return net.Dial("unix", socketPath)
		},
	}

	return &client{
		builder: requests.URL("http://localhost/api/v1/").Transport(t),
	}
}

// Ping checks for API server availability
func (c *client) Ping(ctx context.Context) (*VmmPingResponse, error) {
	resp := &VmmPingResponse{}

	if err := c.builder.Clone().Path("vmm.ping").ToJSON(resp).Fetch(ctx); err != nil {
		return nil, err
	}

	return resp, nil
}

// Info returns general information about the cloud-hypervisor Virtual Machine (VM) instance.
func (c *client) Info(ctx context.Context) (*VmInfo, error) {
	data := &VmInfo{}

	if err := c.builder.Clone().Path("vm.info").
		ToJSON(data).
		Fetch(ctx); err != nil {
		return nil, err
	}
	return data, nil
}

// Delete will delete the cloud-hypervisor Virtual Machine (VM) instance.
func (c *client) Delete(ctx context.Context) error {
	return c.builder.Clone().Path("vm.delete").Put().Fetch(ctx)
}

// Boot will boot the previously created VM instance
func (c *client) Boot(ctx context.Context) error {
	return c.
		builder.
		Clone().
		Path("vm.boot").
		AddValidator(CustomErrValidator(map[int]string{
			404: "The VM instance could not boot because it is not created yet",
		})).
		Put().
		Fetch(ctx)
}

// Pause a previously booted VM instance.
func (c *client) Pause(ctx context.Context) error {
	return c.
		builder.
		Clone().
		Path("vm.pause").
		AddValidator(CustomErrValidator(map[int]string{
			404: "The VM instance could not pause because it is not created yet",
			405: "The VM instance could not pause because it is not booted",
		})).
		Put().
		Fetch(ctx)
}

// Resume a previously paused VM instance.
func (c *client) Resume(ctx context.Context) error {
	return c.
		builder.
		Clone().
		Path("vm.resume").
		AddValidator(CustomErrValidator(map[int]string{
			404: "The VM instance could not resume because it is not booted yet",
			405: "The VM instance could not resume because it is not paused",
		})).
		Put().
		Fetch(ctx)
}

// Shutdown will shut the cloud-hypervisor VMM.
func (c *client) Shutdown(ctx context.Context) error {
	return c.
		builder.
		Clone().
		Path("vm.shutdown").
		AddValidator(CustomErrValidator(map[int]string{
			404: "The VM instance could not shut down because is not created",
			405: "The VM instance could not shut down because it is not started",
		})).
		Put().
		Fetch(ctx)
}

// Reboot the VM instance.M.
func (c *client) Reboot(ctx context.Context) error {
	return c.
		builder.
		Clone().
		Path("vm.reboot").
		AddValidator(CustomErrValidator(map[int]string{
			404: "The VM instance could not reboot because it is not created",
			405: "The VM instance could not reboot because it is not booted",
		})).
		Put().
		Fetch(ctx)
}

// PowerButton triggers a power button in the VM.
func (c *client) PowerButton(ctx context.Context) error {
	return c.
		builder.
		Clone().
		Path("vm.power-button").
		AddValidator(CustomErrValidator(map[int]string{
			404: "The button could not be triggered because it is not created yet",
			405: "The button could not be triggered because it is not booted",
		})).
		Put().
		Fetch(ctx)
}

// Create will create the cloud-hypervisor Virtual Machine (VM) instance. The instance is not booted, only created
func (c *client) Create(ctx context.Context, config *VmConfig) error {
	return c.builder.Clone().Path("vm.create").Put().BodyJSON(config).Fetch(ctx)
}
