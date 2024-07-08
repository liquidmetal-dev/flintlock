package ports

import (
	"context"
	"errors"

	"github.com/liquidmetal-dev/flintlock/core/ports"
)

type portsCtxKeyType string

const portsKey portsCtxKeyType = "flintlockd.ports"

var ErrPortsMissing = errors.New("ports collection not in the context")

func WithPorts(ctx context.Context, ports *ports.Collection) context.Context {
	return context.WithValue(ctx, portsKey, ports)
}

// GetPorts will get the ports from the context.
func GetPorts(ctx context.Context) (*ports.Collection, bool) {
	ports, ok := ctx.Value(portsKey).(*ports.Collection)

	return ports, ok
}
