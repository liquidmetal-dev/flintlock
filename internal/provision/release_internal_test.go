package provision

import (
	"archive/tar"
	"os"
	"path/filepath"
	"testing"

	. "github.com/onsi/gomega"
)

func TestExtractTarEntry_RejectsPathTraversal(t *testing.T) {
	g := NewWithT(t)

	destDir := t.TempDir()

	names := []string{
		"../../etc/passwd",
		"../escape",
		"a/../../escape",
	}

	for _, name := range names {
		header := &tar.Header{
			Name:     name,
			Typeflag: tar.TypeReg,
			Mode:     0o644,
		}

		err := extractTarEntry(nil, header, destDir)
		g.Expect(err).To(HaveOccurred(), "expected %q to be rejected", name)
	}

	// Nothing should have been written outside destDir.
	_, statErr := os.Stat(filepath.Join(filepath.Dir(destDir), "..", "etc", "passwd"))
	g.Expect(os.IsNotExist(statErr)).To(BeTrue())
}

func TestExtractTarEntry_AllowsWithinDestDir(t *testing.T) {
	g := NewWithT(t)

	destDir := t.TempDir()

	header := &tar.Header{
		Name:     "bin/firecracker",
		Typeflag: tar.TypeDir,
		Mode:     0o755,
	}

	err := extractTarEntry(nil, header, destDir)
	g.Expect(err).NotTo(HaveOccurred())

	info, statErr := os.Stat(filepath.Join(destDir, "bin", "firecracker"))
	g.Expect(statErr).NotTo(HaveOccurred())
	g.Expect(info.IsDir()).To(BeTrue())
}
