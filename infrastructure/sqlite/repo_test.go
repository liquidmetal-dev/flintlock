package sqlite_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/liquidmetal-dev/flintlock/infrastructure/internal/reposcontract"
	"github.com/liquidmetal-dev/flintlock/infrastructure/sqlite"
)

func TestMicroVMRepo(t *testing.T) {
	repo, err := sqlite.NewMicroVMRepo(&sqlite.Config{
		DatabasePath: filepath.Join(t.TempDir(), "flintlock.db"),
	})
	if err != nil {
		t.Fatalf("creating sqlite microvm repo: %v", err)
	}

	reposcontract.Run(context.Background(), t, repo, "sqlite-repo-test", "sqlite-repo-test-ns")
}

func TestMicroVMRepo_NilConfig(t *testing.T) {
	if _, err := sqlite.NewMicroVMRepo(nil); err == nil {
		t.Fatal("expected an error for a nil config, got nil")
	}
}

func TestMicroVMRepo_MultipleSave(t *testing.T) {
	repo, err := sqlite.NewMicroVMRepo(&sqlite.Config{
		DatabasePath: filepath.Join(t.TempDir(), "flintlock.db"),
	})
	if err != nil {
		t.Fatalf("creating sqlite microvm repo: %v", err)
	}

	reposcontract.RunMultipleSave(context.Background(), t, repo, "sqlite-repo-multisave-test", "sqlite-repo-multisave-test-ns")
}
