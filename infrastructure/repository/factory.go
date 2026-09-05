// Package repository provides a runtime factory that selects which
// ports.MicroVMRepository backing store implementation to construct, based
// on configuration.
package repository

import (
	"errors"
	"fmt"

	"github.com/liquidmetal-dev/flintlock/core/ports"
	"github.com/liquidmetal-dev/flintlock/infrastructure/containerd"
	"github.com/liquidmetal-dev/flintlock/infrastructure/sqlite"
)

const (
	// StoreContainerd selects the containerd content-store backed repository.
	StoreContainerd = "containerd"
	// StoreSqlite selects the sqlite backed repository.
	StoreSqlite = "sqlite"
)

// Config holds the configuration needed to construct a ports.MicroVMRepository
// for whichever store is selected.
type Config struct {
	// Store is the name of the backing store to use: "containerd" or "sqlite".
	Store string
	// Containerd holds the configuration for the containerd backed repository.
	Containerd *containerd.Config
	// Sqlite holds the configuration for the sqlite backed repository.
	Sqlite *sqlite.Config
}

// New constructs a ports.MicroVMRepository for the store named in cfg.Store.
func New(cfg *Config) (ports.MicroVMRepository, error) {
	if cfg == nil {
		return nil, errors.New("repository config must not be nil")
	}

	switch cfg.Store {
	case "", StoreContainerd:
		if cfg.Containerd == nil {
			return nil, errors.New("containerd repository config must not be nil")
		}

		return containerd.NewMicroVMRepo(cfg.Containerd)
	case StoreSqlite:
		if cfg.Sqlite == nil {
			return nil, errors.New("sqlite repository config must not be nil")
		}

		return sqlite.NewMicroVMRepo(cfg.Sqlite)
	default:
		return nil, fmt.Errorf("unsupported repository store %q: must be %q or %q", cfg.Store, StoreContainerd, StoreSqlite)
	}
}
