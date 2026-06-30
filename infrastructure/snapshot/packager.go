// Package snapshot provides an OCI image packager for microvm snapshots.
package snapshot

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content/file"
	"oras.land/oras-go/v2/content/oci"

	"github.com/liquidmetal-dev/flintlock/core/ports"
)

const (
	// ArtifactType is the artifact type set on the snapshot image manifest.
	ArtifactType = "application/vnd.flintlock.microvm.snapshot.v1"
	// ConfigMediaType is the media type of the microvm spec config blob.
	ConfigMediaType = "application/vnd.flintlock.microvm.snapshot.config.v1+json"

	specFileName = "spec.json"
)

// mediaTypeForKind maps a snapshot artifact kind to its layer media type.
func mediaTypeForKind(kind ports.SnapshotArtifactKind) string {
	return fmt.Sprintf("application/vnd.flintlock.microvm.snapshot.%s.v1", kind)
}

// Config is the configuration for the snapshot packager.
type Config struct {
	// SnapshotRoot is the directory holding the durable OCI image layout.
	SnapshotRoot string
}

// New creates a new snapshot packager.
func New(cfg *Config) ports.SnapshotPackager {
	return &packager{config: cfg}
}

type packager struct {
	config *Config
}

// Build packages the snapshot artifacts and microvm spec into an OCI image
// written to the on-disk layout and returns the resulting image details.
func (p *packager) Build(ctx context.Context, input ports.SnapshotPackageInput) (*ports.SnapshotImage, error) {
	if len(input.Artifacts) == 0 {
		return nil, fmt.Errorf("no snapshot artifacts to package")
	}

	// The artifacts share a scratch directory; use it as the file store root.
	workingDir := filepath.Dir(input.Artifacts[0].Path)

	fileStore, err := file.New(workingDir)
	if err != nil {
		return nil, fmt.Errorf("creating file store: %w", err)
	}
	defer fileStore.Close()

	// Write the microvm spec as a config blob so the image is self-describing.
	specBytes, err := json.Marshal(input.Spec)
	if err != nil {
		return nil, fmt.Errorf("marshalling microvm spec: %w", err)
	}

	specPath := filepath.Join(workingDir, specFileName)
	if err := os.WriteFile(specPath, specBytes, 0o600); err != nil {
		return nil, fmt.Errorf("writing spec blob: %w", err)
	}

	configDesc, err := fileStore.Add(ctx, specFileName, ConfigMediaType, specPath)
	if err != nil {
		return nil, fmt.Errorf("adding spec config blob: %w", err)
	}

	layers := make([]ocispec.Descriptor, 0, len(input.Artifacts))

	for _, artifact := range input.Artifacts {
		name := filepath.Base(artifact.Path)

		desc, addErr := fileStore.Add(ctx, name, mediaTypeForKind(artifact.Kind), artifact.Path)
		if addErr != nil {
			return nil, fmt.Errorf("adding snapshot artifact %s: %w", name, addErr)
		}

		layers = append(layers, desc)
	}

	manifestDesc, err := oras.PackManifest(ctx, fileStore, oras.PackManifestVersion1_1, ArtifactType, oras.PackManifestOptions{
		Layers:           layers,
		ConfigDescriptor: &configDesc,
	})
	if err != nil {
		return nil, fmt.Errorf("packing snapshot manifest: %w", err)
	}

	if err := fileStore.Tag(ctx, manifestDesc, input.Reference); err != nil {
		return nil, fmt.Errorf("tagging snapshot manifest: %w", err)
	}

	if err := os.MkdirAll(p.config.SnapshotRoot, 0o755); err != nil {
		return nil, fmt.Errorf("creating snapshot layout root: %w", err)
	}

	ociStore, err := oci.New(p.config.SnapshotRoot)
	if err != nil {
		return nil, fmt.Errorf("creating oci layout store: %w", err)
	}

	if _, err := oras.Copy(ctx, fileStore, input.Reference, ociStore, input.Reference, oras.DefaultCopyOptions); err != nil {
		return nil, fmt.Errorf("copying snapshot image to layout: %w", err)
	}

	return &ports.SnapshotImage{
		Reference: input.Reference,
		Digest:    manifestDesc.Digest.String(),
	}, nil
}
