package containerd

import (
	"context"
	"fmt"
	"io"

	"github.com/containerd/containerd/v2/core/content"
	"github.com/containerd/containerd/v2/core/images"
	"github.com/containerd/containerd/v2/pkg/archive/compression"
	"github.com/containerd/containerd/v2/pkg/namespaces"
	"github.com/opencontainers/image-spec/identity"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"

	"github.com/liquidmetal-dev/flintlock/infrastructure/volume/overlay"
	"github.com/liquidmetal-dev/flintlock/pkg/log"
)

// NewLayerSource returns an overlay.LayerSource that reads the layers of
// an image from containerd's content store. It pulls the image under the
// lease of the owner, as the image service does, and does not unpack it.
func NewLayerSource(cfg *Config, client Client) overlay.LayerSource {
	return &layerSource{
		images: &imageService{config: cfg, client: client},
	}
}

type layerSource struct {
	images *imageService
}

// Layers pulls the image if needed and returns its rootfs chain ID and one
// opener per layer, lowest first. Each opener returns the layer's tar
// stream decompressed, whatever the registry compressed it with.
func (s *layerSource) Layers(ctx context.Context, in overlay.LayerInput) (*overlay.ImageLayers, error) {
	logger := log.GetLogger(ctx).WithField("service", "containerd_layers")
	logger.Debugf("getting layers of image %s for owner %s", in.ImageRef, in.Owner)

	namespace := s.images.config.Namespace
	nsCtx := namespaces.WithNamespace(ctx, namespace)

	image, err := s.images.pullImage(nsCtx, in.ImageRef, in.Owner)
	if err != nil {
		return nil, fmt.Errorf("getting image %s for owner %s: %w", in.ImageRef, in.Owner, err)
	}

	diffIDs, err := image.RootFS(nsCtx)
	if err != nil {
		return nil, fmt.Errorf("reading rootfs of %s: %w", in.ImageRef, err)
	}

	store := image.ContentStore()

	manifest, err := images.Manifest(nsCtx, store, image.Target(), image.Platform())
	if err != nil {
		return nil, fmt.Errorf("reading manifest of %s: %w", in.ImageRef, err)
	}

	if len(manifest.Layers) != len(diffIDs) {
		return nil, fmt.Errorf("image %s: manifest has %d layers, config has %d diff IDs: %w",
			in.ImageRef, len(manifest.Layers), len(diffIDs), errLayerCount)
	}

	layers := make([]overlay.Layer, len(manifest.Layers))

	for i, desc := range manifest.Layers {
		if !images.IsLayerType(desc.MediaType) {
			return nil, fmt.Errorf("image %s: layer %d has media type %s: %w",
				in.ImageRef, i, desc.MediaType, errNotALayer)
		}

		layers[i] = overlay.Layer{
			DiffID: diffIDs[i],
			Open:   openLayer(namespace, store, desc),
		}
	}

	return &overlay.ImageLayers{
		ChainID: identity.ChainID(diffIDs),
		Layers:  layers,
	}, nil
}

// openLayer returns a function that opens the blob of desc in the content
// store and decompresses it.
func openLayer(
	namespace string, store content.Provider, desc ocispec.Descriptor,
) func(context.Context) (io.ReadCloser, error) {
	return func(ctx context.Context) (io.ReadCloser, error) {
		ra, err := store.ReaderAt(namespaces.WithNamespace(ctx, namespace), desc)
		if err != nil {
			return nil, fmt.Errorf("opening layer %s: %w", desc.Digest, err)
		}

		stream, err := compression.DecompressStream(content.NewReader(ra))
		if err != nil {
			_ = ra.Close()

			return nil, fmt.Errorf("decompressing layer %s: %w", desc.Digest, err)
		}

		return &layerStream{Reader: stream, closers: []io.Closer{stream, ra}}, nil
	}
}

// layerStream is a decompressed layer that closes the blob with the
// decompressor.
type layerStream struct {
	io.Reader
	closers []io.Closer
}

func (s *layerStream) Close() error {
	var errs []error

	for _, c := range s.closers {
		if err := c.Close(); err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) == 0 {
		return nil
	}

	return fmt.Errorf("closing layer stream: %w", errs[0])
}
