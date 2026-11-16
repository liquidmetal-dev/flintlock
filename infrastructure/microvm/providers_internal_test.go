package microvm

import (
	"testing"

	g "github.com/onsi/gomega"

	"github.com/liquidmetal-dev/flintlock/internal/config"
)

func TestFirecrackerConfig_MetadataKernelIP(t *testing.T) {
	g.RegisterTestingT(t)

	g.Expect(firecrackerConfig(&config.Config{FirecrackerMetadataKernelIP: true}, nil).MetadataKernelIP).To(g.BeTrue())
	g.Expect(firecrackerConfig(&config.Config{FirecrackerMetadataKernelIP: false}, nil).MetadataKernelIP).To(g.BeFalse())
}
