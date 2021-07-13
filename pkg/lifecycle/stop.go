package lifecycle

import (
	"context"
	"fmt"
)

// Srop will delete the microvm with the supplied id.
func (m *microVMLifecycle) Stop(ctx context.Context, id string) error {
	return fmt.Errorf("not implemented")
}
