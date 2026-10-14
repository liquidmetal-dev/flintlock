package overlay_test

import (
	"archive/tar"
	"bytes"
	"context"
	"io"
	"testing"
	"time"

	"github.com/opencontainers/go-digest"
	"github.com/opencontainers/image-spec/identity"

	"github.com/liquidmetal-dev/flintlock/infrastructure/volume/overlay"
)

// entry is one tar entry of a test layer.
type entry struct {
	name     string
	typeflag byte
	mode     int64
	uid, gid int
	content  string
	link     string
	xattrs   map[string]string
	major    int64
	minor    int64
}

var fixtureTime = time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC)

func dir(name string, mode int64) entry {
	return entry{name: name + "/", typeflag: tar.TypeDir, mode: mode}
}

func file(name string, mode int64, content string) entry {
	return entry{name: name, typeflag: tar.TypeReg, mode: mode, content: content}
}

func hardlink(name, target string, mode int64) entry {
	return entry{name: name, typeflag: tar.TypeLink, mode: mode, link: target}
}

func symlink(name, target string) entry {
	return entry{name: name, typeflag: tar.TypeSymlink, mode: 0o777, link: target}
}

// tarBytes writes the entries as one uncompressed tar stream.
func tarBytes(t *testing.T, entries ...entry) []byte {
	t.Helper()

	var buf bytes.Buffer

	tw := tar.NewWriter(&buf)

	for _, e := range entries {
		hdr := &tar.Header{
			Typeflag: e.typeflag,
			Name:     e.name,
			Mode:     e.mode,
			Uid:      e.uid,
			Gid:      e.gid,
			Linkname: e.link,
			ModTime:  fixtureTime,
			Devmajor: e.major,
			Devminor: e.minor,
			Format:   tar.FormatPAX,
		}

		if e.typeflag == tar.TypeReg {
			hdr.Size = int64(len(e.content))
		}

		for k, v := range e.xattrs {
			if hdr.PAXRecords == nil {
				hdr.PAXRecords = map[string]string{}
			}

			hdr.PAXRecords["SCHILY.xattr."+k] = v
		}

		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatalf("writing tar header %s: %v", e.name, err)
		}

		if e.typeflag == tar.TypeReg {
			if _, err := io.WriteString(tw, e.content); err != nil {
				t.Fatalf("writing tar content %s: %v", e.name, err)
			}
		}
	}

	if err := tw.Close(); err != nil {
		t.Fatalf("closing tar: %v", err)
	}

	return buf.Bytes()
}

// memSource is a LayerSource over in-memory tar streams.
type memSource struct {
	layers [][]byte
	opens  int
}

func newMemSource(layers ...[]byte) *memSource {
	return &memSource{layers: layers}
}

func (s *memSource) diffIDs() []digest.Digest {
	ids := make([]digest.Digest, len(s.layers))
	for i, l := range s.layers {
		ids[i] = digest.FromBytes(l)
	}

	return ids
}

func (s *memSource) chainID() digest.Digest {
	return identity.ChainID(s.diffIDs())
}

func (s *memSource) Layers(_ context.Context, _ overlay.LayerInput) (*overlay.ImageLayers, error) {
	out := &overlay.ImageLayers{ChainID: s.chainID()}

	for i, l := range s.layers {
		out.Layers = append(out.Layers, overlay.Layer{
			DiffID: digest.FromBytes(l),
			Open: func(context.Context) (io.ReadCloser, error) {
				s.opens++

				return io.NopCloser(bytes.NewReader(s.layers[i])), nil
			},
		})
	}

	return out, nil
}

// failSource is a LayerSource that returns an error.
type failSource struct{ err error }

func (s failSource) Layers(context.Context, overlay.LayerInput) (*overlay.ImageLayers, error) {
	return nil, s.err
}
