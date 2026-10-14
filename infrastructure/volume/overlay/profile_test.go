package overlay_test

import (
	"testing"

	. "github.com/onsi/gomega"
	"github.com/opencontainers/go-digest"

	"github.com/liquidmetal-dev/flintlock/infrastructure/volume/overlay"
	"github.com/liquidmetal-dev/flintlock/pkg/defaults"
)

func TestErofsV1(t *testing.T) {
	g := NewWithT(t)
	p := overlay.ErofsV1()

	g.Expect(p.Name).To(Equal("erofs-v1"))
	g.Expect(p.BlockSize).To(Equal(4096))
	g.Expect(p.ErofsUtilsVersion).To(Equal("1.9.4"))
	// The pinned binary must be the one the profile fixes. A bump of the
	// default version is a new profile, not a change to erofs-v1.
	g.Expect(p.ErofsUtilsVersion).To(Equal(defaults.ErofsUtilsVersion))
}

// The UUID values are fixed: they are part of every base that was built.
func TestUUIDs(t *testing.T) {
	g := NewWithT(t)

	l1 := digest.Digest("sha256:0000000000000000000000000000000000000000000000000000000000000001")
	l2 := digest.Digest("sha256:0000000000000000000000000000000000000000000000000000000000000002")
	chain := digest.Digest("sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")

	g.Expect(overlay.LayerUUID(l1).String()).To(Equal("455987de-4968-56c6-b50b-af5abff64a1b"))
	g.Expect(overlay.LayerUUID(l2).String()).To(Equal("ef07d8dc-f273-5762-a37c-a00d5dc0d168"))
	g.Expect(overlay.BaseUUID(chain, "erofs-v1").String()).To(Equal("c87db64b-1de0-57d4-8916-9d83981a6aca"))
	g.Expect(overlay.BaseUUID(chain, "erofs-v2")).NotTo(Equal(overlay.BaseUUID(chain, "erofs-v1")))
}
