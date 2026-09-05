// Package migraterepo implements the "migrate-repository" command, which
// copies existing microvm spec/status definitions from the containerd
// content-store backed repository into the sqlite backed repository. It is
// an explicit, opt-in, idempotent step an operator runs before switching
// --repository-store from "containerd" to "sqlite".
package migraterepo

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/liquidmetal-dev/flintlock/core/models"
	"github.com/liquidmetal-dev/flintlock/infrastructure/containerd"
	"github.com/liquidmetal-dev/flintlock/infrastructure/sqlite"
	cmdflags "github.com/liquidmetal-dev/flintlock/internal/command/flags"
	"github.com/liquidmetal-dev/flintlock/internal/config"
	"github.com/liquidmetal-dev/flintlock/pkg/flags"
	"github.com/liquidmetal-dev/flintlock/pkg/log"
)

// NewCommand creates a new cobra command for migrating microvm specs from
// the containerd backed repository to the sqlite backed repository.
func NewCommand(cfg *config.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "migrate-repository",
		Short: "Copy microvm spec/status definitions from the containerd repository into the sqlite repository",
		PreRunE: func(c *cobra.Command, _ []string) error {
			flags.BindCommandToViper(c)

			return nil
		},
		RunE: func(c *cobra.Command, _ []string) error {
			return migrate(c.Context(), cfg)
		},
	}

	cmdflags.AddStateDirFlagToCommand(cmd, cfg)
	cmdflags.AddContainerDFlagsToCommand(cmd, cfg)
	cmdflags.AddRepositoryFlagsToCommand(cmd, cfg)

	return cmd
}

func migrate(ctx context.Context, cfg *config.Config) error {
	logger := log.GetLogger(ctx).WithField("command", "migrate-repository")

	from, err := containerd.NewMicroVMRepo(&containerd.Config{
		SnapshotterKernel: cfg.CtrSnapshotterKernel,
		SocketPath:        cfg.CtrSocketPath,
		Namespace:         cfg.CtrNamespace,
	})
	if err != nil {
		return fmt.Errorf("creating containerd microvm repository: %w", err)
	}

	to, err := sqlite.NewMicroVMRepo(&sqlite.Config{
		DatabasePath: cfg.ResolvedSqliteDataPath(),
	})
	if err != nil {
		return fmt.Errorf("creating sqlite microvm repository: %w", err)
	}

	microvms, err := from.GetAll(ctx, models.ListMicroVMQuery{})
	if err != nil {
		return fmt.Errorf("listing microvms from containerd repository: %w", err)
	}

	logger.Infof("found %d microvm(s) in the containerd repository", len(microvms))

	migrated, skipped := 0, 0

	for _, microvm := range microvms {
		exists, existsErr := to.Exists(ctx, microvm.ID)
		if existsErr != nil {
			return fmt.Errorf("checking if microvm %s already exists in sqlite repository: %w", microvm.ID, existsErr)
		}

		if exists {
			logger.Infof("microvm %s already exists in the sqlite repository, skipping", microvm.ID)

			skipped++

			continue
		}

		// Reset the version so the destination starts its own version
		// history at 1, the same way a fresh Save would.
		toSave := *microvm
		toSave.Version = 0

		if _, err := to.Save(ctx, &toSave); err != nil {
			return fmt.Errorf("saving microvm %s to sqlite repository: %w", microvm.ID, err)
		}

		logger.Infof("migrated microvm %s", microvm.ID)

		migrated++
	}

	logger.Infof("migration complete: %d migrated, %d already present and skipped", migrated, skipped)

	return nil
}
