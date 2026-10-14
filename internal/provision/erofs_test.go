package provision_test

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"

	. "github.com/onsi/gomega"

	"github.com/liquidmetal-dev/flintlock/internal/provision"
	"github.com/liquidmetal-dev/flintlock/pkg/defaults"
)

func TestErofsUtilsAssetName(t *testing.T) {
	g := NewWithT(t)

	g.Expect(provision.ErofsUtilsAssetName("1.9.4", "amd64")).To(Equal("mkfs.erofs-1.9.4-linux-amd64"))
	g.Expect(provision.ErofsUtilsAssetName("1.9.4", "arm64")).To(Equal("mkfs.erofs-1.9.4-linux-arm64"))
}

// The erofs-utils version is pinned in three places: the Go constant, the
// build script's default and the composite action's default. They must agree,
// or the provisioner asks for an asset the release does not carry.
func TestErofsUtilsVersionPinsAgree(t *testing.T) {
	g := NewWithT(t)

	script, err := os.ReadFile(filepath.Join("..", "..", "hack", "scripts", "build_erofs_utils.sh"))
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(string(script)).To(ContainSubstring(`DEFAULT_VERSION="` + defaults.ErofsUtilsVersion + `"`))

	action, err := os.ReadFile(filepath.Join("..", "..", ".github", "actions", "erofs-utils", "action.yml"))
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(string(action)).To(ContainSubstring(`default: "` + defaults.ErofsUtilsVersion + `"`))
}

func TestInstallErofsUtils_RefusesNonAMD64(t *testing.T) {
	g := NewWithT(t)

	// A concrete tag means no release lookup. The refusal must come before
	// any download, so no server is needed.
	err := provision.InstallErofsUtils(context.Background(), provision.NewRunner(io.Discard, io.Discard), "v1.0.0", "arm64")

	g.Expect(err).To(MatchError(provision.ErrErofsUtilsArch))
	g.Expect(err.Error()).To(ContainSubstring("amd64 only"))
	g.Expect(err.Error()).To(ContainSubstring("issues/1317"))
	g.Expect(err.Error()).To(ContainSubstring("host is arm64"))
}
