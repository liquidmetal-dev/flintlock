package provision

import (
	"archive/tar"
	"os"
	"path/filepath"
	"testing"

	. "github.com/onsi/gomega"
)

func TestSafeJoin(t *testing.T) {
	g := NewWithT(t)

	dest, err := safeJoin("/tmp/dest", "bin/firecracker")
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(dest).To(Equal(filepath.Join("/tmp/dest", "bin/firecracker")))

	// filepath.Join already neutralises a leading "/" in name (it's treated
	// as a relative component, not an override), so only ".."-based
	// traversal can actually escape destDir here.
	tests := []string{
		"../../etc/passwd",
		"../escape",
		"a/../../escape",
	}

	for _, name := range tests {
		_, err := safeJoin("/tmp/dest", name)
		g.Expect(err).To(HaveOccurred(), "expected %q to be rejected", name)
	}
}

func TestExtractTarEntry_RejectsPathTraversal(t *testing.T) {
	g := NewWithT(t)

	destDir := t.TempDir()

	header := &tar.Header{
		Name:     "../../etc/passwd",
		Typeflag: tar.TypeReg,
		Mode:     0o644,
	}

	err := extractTarEntry(nil, header, destDir)
	g.Expect(err).To(HaveOccurred())

	// Nothing should have been written outside destDir.
	_, statErr := os.Stat(filepath.Join(filepath.Dir(destDir), "..", "etc", "passwd"))
	g.Expect(os.IsNotExist(statErr)).To(BeTrue())
}
