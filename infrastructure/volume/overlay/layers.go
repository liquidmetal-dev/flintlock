package overlay

import (
	"context"
	"io"

	"github.com/opencontainers/go-digest"
)

// LayerInput names the image whose layers a LayerSource returns.
type LayerInput struct {
	// ImageRef is the OCI image reference, for example
	// ghcr.io/liquidmetal-dev/ubuntu:24.04.
	ImageRef string
	// Owner identifies the VM the layers are read for. The containerd
	// implementation holds the image content under this owner's lease.
	Owner string
}

// Layer is one layer of an image.
type Layer struct {
	// DiffID is the sha256 digest of the uncompressed tar stream of the
	// layer. The builder checks the stream against it.
	DiffID digest.Digest
	// Open returns a new uncompressed tar stream of the layer. The caller
	// closes the stream. Open can be called more than once.
	Open func(ctx context.Context) (io.ReadCloser, error)
}

// ImageLayers is the rootfs of an image as a chain of layers.
type ImageLayers struct {
	// ChainID is the OCI chain ID of the diff IDs, lowest layer first.
	ChainID digest.Digest
	// Layers holds the layers, lowest first.
	Layers []Layer
}

// LayerSource gives the builder the layers of an image.
//
// The builder trusts no source for the content of a layer. It hashes each
// stream as it converts it and fails the build when the digest is not the
// layer's DiffID. The chain ID is derived from the diff IDs, so a recorded
// base is keyed by the identity of the bytes that were built, whatever the
// source claimed.
type LayerSource interface {
	// Layers pulls the image if needed and returns its rootfs chain ID and
	// one uncompressed tar stream per layer, lowest first.
	Layers(ctx context.Context, in LayerInput) (*ImageLayers, error)
}
