package overlay

import (
	"archive/tar"
	"bytes"
	"io"
	"testing"

	. "github.com/onsi/gomega"
)

func writeTar(t *testing.T, entries ...*tar.Header) []byte {
	t.Helper()

	var buf bytes.Buffer

	tw := tar.NewWriter(&buf)

	for _, hdr := range entries {
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatal(err)
		}

		if hdr.Typeflag == tar.TypeReg {
			if _, err := tw.Write(make([]byte, hdr.Size)); err != nil {
				t.Fatal(err)
			}
		}
	}

	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}

	return buf.Bytes()
}

func reg(name string, size int64) *tar.Header {
	return &tar.Header{Typeflag: tar.TypeReg, Name: name, Mode: 0o644, Size: size}
}

func link(name, target string) *tar.Header {
	return &tar.Header{Typeflag: tar.TypeLink, Name: name, Linkname: target}
}

func TestHardlinkCheck(t *testing.T) {
	tests := []struct {
		name   string
		stream []byte
		entry  string
		target string
		found  bool
	}{
		{
			name:   "no links",
			stream: writeTar(t, reg("a", 10), reg("b", 0)),
		},
		{
			name:   "link to earlier entry",
			stream: writeTar(t, reg("usr/a", 1000), link("usr/b", "usr/a")),
		},
		{
			name:   "link with ./ and / prefixes",
			stream: writeTar(t, reg("./usr/a", 1000), link("/usr/b", "./usr/a")),
		},
		{
			name:   "link to absent entry",
			stream: writeTar(t, reg("usr/a", 1000), link("usr/b", "usr/c")),
			entry:  "usr/b",
			target: "usr/c",
			found:  true,
		},
		{
			name:   "link to a later entry",
			stream: writeTar(t, link("usr/b", "usr/a"), reg("usr/a", 1)),
			entry:  "usr/b",
			target: "usr/a",
			found:  true,
		},
		{
			name:   "first foreign link is reported",
			stream: writeTar(t, reg("a", 1), link("b", "a"), link("c", "x"), link("d", "y")),
			entry:  "c",
			target: "x",
			found:  true,
		},
		{
			name:   "not a tar",
			stream: bytes.Repeat([]byte("junk"), 500),
		},
		{
			name:   "empty",
			stream: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			c := newHardlinkCheck()

			n, err := io.Copy(io.Discard, c.reader(bytes.NewReader(tt.stream)))
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(n).To(Equal(int64(len(tt.stream))), "the tee passes every byte")

			entry, target, found := c.finish()
			g.Expect(found).To(Equal(tt.found))
			g.Expect(entry).To(Equal(tt.entry))
			g.Expect(target).To(Equal(tt.target))
		})
	}
}

func TestHardlinkCheck_DrainAfterShortRead(t *testing.T) {
	g := NewWithT(t)
	stream := writeTar(t, reg("a", 4096), link("b", "zzz"))
	c := newHardlinkCheck()
	r := bytes.NewReader(stream)

	// The consumer stops after one block, as mkfs.erofs does at an error.
	_, err := io.ReadFull(c.reader(r), make([]byte, 512))
	g.Expect(err).NotTo(HaveOccurred())

	c.drain(r)

	entry, target, found := c.finish()
	g.Expect(found).To(BeTrue())
	g.Expect(entry).To(Equal("b"))
	g.Expect(target).To(Equal("zzz"))
}

func TestCleanTarPath(t *testing.T) {
	g := NewWithT(t)

	g.Expect(cleanTarPath("usr/a")).To(Equal("usr/a"))
	g.Expect(cleanTarPath("./usr/a")).To(Equal("usr/a"))
	g.Expect(cleanTarPath("/usr/a")).To(Equal("usr/a"))
	g.Expect(cleanTarPath("usr//a/")).To(Equal("usr/a"))
	g.Expect(cleanTarPath(".")).To(Equal(""))
	g.Expect(cleanTarPath("")).To(Equal(""))
}
