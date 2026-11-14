package provision

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	. "github.com/onsi/gomega"
)

func TestCheckMkfsErofsVersion(t *testing.T) {
	tests := []struct {
		name    string
		output  string
		wantErr bool
	}{
		{name: "pinned version", output: "mkfs.erofs (erofs-utils) 1.9.4\navailable compressors: deflate\n"},
		{name: "older version", output: "mkfs.erofs (erofs-utils) 1.7.1\n", wantErr: true},
		{name: "longer version", output: "mkfs.erofs (erofs-utils) 1.9.41\n", wantErr: true},
		{name: "no output", output: "", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			bin := filepath.Join(t.TempDir(), "mkfs.erofs")
			script := "#!/bin/sh\nprintf '%s' '" + tt.output + "'\n"
			g.Expect(os.WriteFile(bin, []byte(script), 0o755)).To(Succeed())

			runner := NewRunner(io.Discard, io.Discard)

			err := checkMkfsErofsVersion(runner, bin, "1.9.4")
			if tt.wantErr {
				g.Expect(err).To(HaveOccurred())

				return
			}

			g.Expect(err).NotTo(HaveOccurred())
		})
	}
}

func TestCheckMkfsErofsVersion_BinaryFails(t *testing.T) {
	g := NewWithT(t)

	bin := filepath.Join(t.TempDir(), "mkfs.erofs")
	g.Expect(os.WriteFile(bin, []byte("#!/bin/sh\nexit 3\n"), 0o755)).To(Succeed())

	err := checkMkfsErofsVersion(NewRunner(io.Discard, io.Discard), bin, "1.9.4")
	g.Expect(err).To(HaveOccurred())
}
