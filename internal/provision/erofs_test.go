package provision_test

import (
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
