package provision

import (
	"testing"

	. "github.com/onsi/gomega"

	"github.com/liquidmetal-dev/flintlock/pkg/defaults"
)

func TestAllSkipThinpool(t *testing.T) {
	g := NewWithT(t)

	cmd := newAllCommand()
	g.Expect(cmd.Flags().Lookup("skip-thinpool")).NotTo(BeNil())

	withPool := &allOptions{thinpool: "flintlock"}
	g.Expect(containerdThinpool(withPool)).To(Equal("flintlock"))
	g.Expect(storageStep(withPool)).To(Equal(", direct-lvm"))

	dev := &allOptions{thinpool: "flintlock-dev", dev: true}
	g.Expect(storageStep(dev)).To(Equal(", devpool"))

	skipped := &allOptions{thinpool: "flintlock", skipThinpool: true}
	g.Expect(containerdThinpool(skipped)).To(BeEmpty())
	g.Expect(storageStep(skipped)).To(BeEmpty())
}

func TestStateDirFlags(t *testing.T) {
	g := NewWithT(t)

	cmd := newStateDirCommand()
	g.Expect(cmd.Use).To(Equal("state-dir"))
	g.Expect(cmd.Flags().Lookup("dir").DefValue).To(Equal(defaults.StateRootDir))
	g.Expect(cmd.Flags().Lookup("size").DefValue).To(Equal("20"))

	for _, name := range []string{"device", "loop-file", "force", "unattended"} {
		g.Expect(cmd.Flags().Lookup(name)).NotTo(BeNil(), name)
	}

	root, err := NewRootCommand()
	g.Expect(err).NotTo(HaveOccurred())

	sub, _, err := root.Find([]string{"state-dir"})
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(sub.Use).To(Equal("state-dir"))
}
