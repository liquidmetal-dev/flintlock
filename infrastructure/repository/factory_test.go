package repository_test

import (
	"testing"

	"github.com/liquidmetal-dev/flintlock/infrastructure/repository"
)

func TestNew_NilConfig(t *testing.T) {
	if _, err := repository.New(nil); err == nil {
		t.Fatal("expected an error for a nil config, got nil")
	}
}

func TestNew_NilContainerdConfig(t *testing.T) {
	_, err := repository.New(&repository.Config{Store: repository.StoreContainerd})
	if err == nil {
		t.Fatal("expected an error for a nil containerd config, got nil")
	}
}

func TestNew_NilSqliteConfig(t *testing.T) {
	_, err := repository.New(&repository.Config{Store: repository.StoreSqlite})
	if err == nil {
		t.Fatal("expected an error for a nil sqlite config, got nil")
	}
}

func TestNew_UnsupportedStore(t *testing.T) {
	_, err := repository.New(&repository.Config{Store: "bogus"})
	if err == nil {
		t.Fatal("expected an error for an unsupported store, got nil")
	}
}
