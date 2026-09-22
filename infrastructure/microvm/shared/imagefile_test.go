package shared_test

import (
	"os"
	"path/filepath"
	"testing"

	g "github.com/onsi/gomega"

	"github.com/liquidmetal-dev/flintlock/core/errors"
	"github.com/liquidmetal-dev/flintlock/infrastructure/microvm/shared"
)

func TestResolveImageFile(t *testing.T) {
	// outside is a host file that must never be reachable from the image root.
	outsideDir := t.TempDir()
	outside := filepath.Join(outsideDir, "secret")
	writeFile(t, outside)

	root := filepath.Join(t.TempDir(), "rootfs")
	writeFile(t, filepath.Join(root, "vmlinux"))
	writeFile(t, filepath.Join(root, "boot", "vmlinux"))
	symlink(t, "vmlinux", filepath.Join(root, "inner-link"))
	symlink(t, outside, filepath.Join(root, "abs-link"))
	symlink(t, "../../../../../../../../"+outside, filepath.Join(root, "rel-link"))
	symlink(t, outsideDir, filepath.Join(root, "dir-link"))

	tt := []struct {
		name     string
		filename string
		expected string
	}{
		{name: "regular file", filename: "vmlinux", expected: filepath.Join(root, "vmlinux")},
		{name: "nested file", filename: "boot/vmlinux", expected: filepath.Join(root, "boot", "vmlinux")},
		{name: "symlink within image", filename: "inner-link", expected: filepath.Join(root, "vmlinux")},
		{name: "empty", filename: ""},
		{name: "absolute path", filename: outside},
		{name: "parent traversal", filename: "../../../../../../../../" + outside},
		{name: "absolute symlink out of image", filename: "abs-link"},
		{name: "relative symlink out of image", filename: "rel-link"},
		{name: "symlinked directory out of image", filename: "dir-link/secret"},
		{name: "directory", filename: "boot"},
		{name: "missing file", filename: "missing"},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			g.RegisterTestingT(t)

			path, err := shared.ResolveImageFile(root, tc.filename)

			if tc.expected == "" {
				g.Expect(err).To(g.MatchError(errors.ErrInvalidImageFilePath))
				g.Expect(path).To(g.BeEmpty())

				return
			}

			g.Expect(err).NotTo(g.HaveOccurred())
			g.Expect(path).To(g.Equal(tc.expected))
		})
	}
}

func TestResolveImageFile_InvalidRoot(t *testing.T) {
	// A file under the working directory that a non-absolute root must never resolve to.
	cwd := t.TempDir()
	writeFile(t, filepath.Join(cwd, "etc", "passwd"))
	writeFile(t, filepath.Join(cwd, "rootfs", "vmlinux"))
	t.Chdir(cwd)

	tt := []struct {
		name     string
		root     string
		filename string
	}{
		{name: "empty root", root: "", filename: "etc/passwd"},
		{name: "dot root", root: ".", filename: "etc/passwd"},
		{name: "relative root", root: "rootfs", filename: "vmlinux"},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			g.RegisterTestingT(t)

			path, err := shared.ResolveImageFile(tc.root, tc.filename)
			g.Expect(err).To(g.MatchError(errors.ErrInvalidImageFilePath))
			g.Expect(path).To(g.BeEmpty())
		})
	}
}

func writeFile(t *testing.T, path string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(path, []byte("data"), 0o600); err != nil {
		t.Fatal(err)
	}
}

func symlink(t *testing.T, target, link string) {
	t.Helper()

	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
}
