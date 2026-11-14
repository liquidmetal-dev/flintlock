package containerd

import (
	"context"
	"errors"
	"fmt"

	containerd "github.com/containerd/containerd/v2/client"
	"github.com/containerd/containerd/v2/pkg/namespaces"
	"github.com/containerd/containerd/v2/plugins"
)

var errSnapshotterNotLoaded = errors.New("snapshotter is not loaded")

// NewClient connects to the containerd socket of cfg.
func NewClient(cfg *Config) (Client, error) {
	client, err := containerd.New(cfg.SocketPath)
	if err != nil {
		return nil, fmt.Errorf("creating containerd client: %w", err)
	}

	return client, nil
}

// ProbeSnapshotter checks that containerd loaded the snapshotter plugin
// named name. It asks the introspection service for the snapshotter
// plugins and looks for one with that ID and no init error. A plugin that
// containerd skipped, for example devmapper with no pool configured, has
// an init error and does not count.
func ProbeSnapshotter(ctx context.Context, cfg *Config, client Client, name string) error {
	nsCtx := namespaces.WithNamespace(ctx, cfg.Namespace)

	resp, err := client.IntrospectionService().Plugins(nsCtx, "type=="+plugins.SnapshotPlugin.String())
	if err != nil {
		return fmt.Errorf("listing containerd snapshotter plugins: %w", err)
	}

	for _, plugin := range resp.Plugins {
		if plugin.ID != name {
			continue
		}

		if initErr := plugin.GetInitErr(); initErr != nil {
			return fmt.Errorf("snapshotter %s: %s: %w", name, initErr.GetMessage(), errSnapshotterNotLoaded)
		}

		return nil
	}

	return fmt.Errorf("snapshotter %s: %w", name, errSnapshotterNotLoaded)
}
