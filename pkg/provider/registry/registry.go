package registry

import (
	"context"
	"fmt"
	"sync"

	"github.com/weaveworks/reignite/pkg/provider"
)

var (
	pluginsLock sync.RWMutex
	plugins     = map[string]provider.Factory{}
)

// RegisterProvider will register a provider plugin with the registry.
func RegisterProvider(name string, factory provider.Factory) error {
	pluginsLock.Lock()
	defer pluginsLock.Unlock()

	if _, ok := plugins[name]; ok {
		return fmt.Errorf("plugin %s has already been registered", name) //nolint:goerr113
	}

	plugins[name] = factory

	return nil
}

// GetPluginInstance will create a new instance of a plugin with the supplied name and runtime
// environment. When creating the instance the factory function for the plugin is called.
func GetPluginInstance(ctx context.Context, name string, runtime *provider.Runtime) (provider.MicrovmProvider, error) {
	pluginsLock.RLock()
	defer pluginsLock.RUnlock()

	if factoryFunc, ok := plugins[name]; ok {
		return factoryFunc(ctx, runtime)
	}

	return nil, fmt.Errorf("plugin %s not found", name) // nolint:goerr113
}

// ListPlugins returns a list of the plugin names that have been registered.
func ListPlugins() []string {
	pluginsLock.RLock()
	defer pluginsLock.RUnlock()

	names := make([]string, 0, len(plugins))
	for name := range plugins {
		names = append(names, name)
	}

	return names
}

// Reset will remove the registered plugins.
func Reset() {
	pluginsLock.Lock()
	defer pluginsLock.Unlock()

	plugins = map[string]provider.Factory{}
}
