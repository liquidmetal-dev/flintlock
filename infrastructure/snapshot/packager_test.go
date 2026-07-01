package snapshot_test

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"

	. "github.com/onsi/gomega"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content/oci"

	"github.com/liquidmetal-dev/flintlock/core/models"
	"github.com/liquidmetal-dev/flintlock/core/ports"
	"github.com/liquidmetal-dev/flintlock/infrastructure/snapshot"
)

func TestPackager_Build(t *testing.T) {
	RegisterTestingT(t)

	ctx := context.Background()

	scratch := t.TempDir()
	layoutRoot := t.TempDir()

	statePath := filepath.Join(scratch, "vmstate")
	memPath := filepath.Join(scratch, "memory")
	Expect(os.WriteFile(statePath, []byte("state-bytes"), 0o600)).To(Succeed())
	Expect(os.WriteFile(memPath, []byte("memory-bytes"), 0o600)).To(Succeed())

	vmid, _ := models.NewVMID("vm1", "ns1", "uid1")

	packager := snapshot.New(&snapshot.Config{SnapshotRoot: layoutRoot})

	image, err := packager.Build(ctx, ports.SnapshotPackageInput{
		Reference: "myorg/snap:v1",
		Spec:      &models.MicroVM{ID: *vmid},
		Artifacts: []ports.SnapshotArtifact{
			{Kind: ports.SnapshotState, Path: statePath},
			{Kind: ports.SnapshotMemory, Path: memPath},
		},
	})

	Expect(err).NotTo(HaveOccurred())
	Expect(image.Reference).To(Equal("myorg/snap:v1"))
	Expect(image.Digest).To(HavePrefix("sha256:"))

	// The OCI layout should resolve the tag and the manifest should describe the
	// snapshot: custom artifact type, a config blob, and one layer per artifact.
	store, err := oci.New(layoutRoot)
	Expect(err).NotTo(HaveOccurred())

	desc, err := store.Resolve(ctx, "myorg/snap:v1")
	Expect(err).NotTo(HaveOccurred())
	Expect(desc.Digest.String()).To(Equal(image.Digest))

	manifest := fetchManifest(ctx, store, desc)
	Expect(manifest.ArtifactType).To(Equal(snapshot.ArtifactType))
	Expect(manifest.Config.MediaType).To(Equal(snapshot.ConfigMediaType))
	Expect(manifest.Layers).To(HaveLen(2))

	mediaTypes := []string{manifest.Layers[0].MediaType, manifest.Layers[1].MediaType}
	Expect(mediaTypes).To(ConsistOf(
		"application/vnd.flintlock.microvm.snapshot.state.v1",
		"application/vnd.flintlock.microvm.snapshot.memory.v1",
	))
}

func TestPackager_Build_NoArtifacts(t *testing.T) {
	RegisterTestingT(t)

	packager := snapshot.New(&snapshot.Config{SnapshotRoot: t.TempDir()})

	_, err := packager.Build(context.Background(), ports.SnapshotPackageInput{
		Reference: "myorg/snap:v1",
		Spec:      &models.MicroVM{},
	})

	Expect(err).To(HaveOccurred())
}

func TestPackager_Build_InvalidConfig(t *testing.T) {
	testCases := []struct {
		name     string
		packager ports.SnapshotPackager
	}{
		{
			name:     "nil config",
			packager: snapshot.New(nil),
		},
		{
			name:     "empty snapshot root",
			packager: snapshot.New(&snapshot.Config{}),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			RegisterTestingT(t)

			_, err := tc.packager.Build(context.Background(), ports.SnapshotPackageInput{
				Reference: "myorg/snap:v1",
				Spec:      &models.MicroVM{},
				Artifacts: []ports.SnapshotArtifact{
					{Kind: ports.SnapshotState, Path: filepath.Join(t.TempDir(), "state")},
				},
			})

			Expect(err).To(HaveOccurred())
		})
	}
}

func TestPackager_Build_InvalidInput(t *testing.T) {
	RegisterTestingT(t)

	scratch := t.TempDir()
	artifactPath := filepath.Join(scratch, "vmstate")
	Expect(os.WriteFile(artifactPath, []byte("state-bytes"), 0o600)).To(Succeed())

	testCases := []struct {
		name  string
		input ports.SnapshotPackageInput
	}{
		{
			name: "empty reference",
			input: ports.SnapshotPackageInput{
				Spec: &models.MicroVM{},
				Artifacts: []ports.SnapshotArtifact{
					{Kind: ports.SnapshotState, Path: artifactPath},
				},
			},
		},
		{
			name: "nil spec",
			input: ports.SnapshotPackageInput{
				Reference: "myorg/snap:v1",
				Artifacts: []ports.SnapshotArtifact{
					{Kind: ports.SnapshotState, Path: artifactPath},
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			RegisterTestingT(t)

			packager := snapshot.New(&snapshot.Config{SnapshotRoot: t.TempDir()})

			_, err := packager.Build(context.Background(), tc.input)

			Expect(err).To(HaveOccurred())
			Expect(filepath.Join(scratch, "spec.json")).NotTo(BeAnExistingFile())
		})
	}
}

func fetchManifest(ctx context.Context, store oras.ReadOnlyTarget, desc ocispec.Descriptor) ocispec.Manifest {
	rc, err := store.Fetch(ctx, desc)
	Expect(err).NotTo(HaveOccurred())
	defer rc.Close()

	content, err := io.ReadAll(rc)
	Expect(err).NotTo(HaveOccurred())

	var manifest ocispec.Manifest
	Expect(json.Unmarshal(content, &manifest)).To(Succeed())

	return manifest
}
