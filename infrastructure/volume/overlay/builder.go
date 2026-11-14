package overlay

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"

	"github.com/opencontainers/go-digest"

	"github.com/liquidmetal-dev/flintlock/pkg/log"
)

// Builder builds and finds the bases of overlay volumes under a state root.
type Builder struct {
	stateRoot string
	source    LayerSource
	mkfs      mkfs
	profile   Profile
	index     baseIndex
}

// NewBuilder returns a Builder that keeps bases under stateRoot, runs the
// mkfs.erofs at mkfsPath and reads image layers from source. It creates
// the bases and scratch directories. The profile is erofs-v1.
func NewBuilder(stateRoot, mkfsPath string, source LayerSource) (*Builder, error) {
	root, err := filepath.Abs(stateRoot)
	if err != nil {
		return nil, fmt.Errorf("resolving state root %s: %w", stateRoot, err)
	}

	index, err := newFSIndex(root)
	if err != nil {
		return nil, err
	}

	profile := ErofsV1()

	return &Builder{
		stateRoot: root,
		source:    source,
		mkfs:      mkfs{path: mkfsPath, profile: profile},
		profile:   profile,
		index:     index,
	}, nil
}

// Build returns the base of the image. It builds the base when the index
// has none for the image's chain ID and the profile. One flock per key
// serialises builds across processes: a second caller waits and then
// finds the finished base. A found base has its last-use time refreshed.
func (b *Builder) Build(ctx context.Context, in LayerInput) (*Base, error) {
	logger := log.GetLogger(ctx).WithField("service", "overlay_base")

	layers, err := b.source.Layers(ctx, in)
	if err != nil {
		return nil, fmt.Errorf("getting layers of %s: %w", in.ImageRef, err)
	}

	if len(layers.Layers) == 0 {
		return nil, fmt.Errorf("%s: %w", in.ImageRef, errNoLayers)
	}

	key := baseKey{ChainID: layers.ChainID, Profile: b.profile.Name}

	release, err := b.index.Lock(ctx, key)
	if err != nil {
		return nil, err
	}

	defer func() { _ = release() }()

	base, err := b.index.Lookup(key)
	if err == nil {
		logger.Debugf("base %s exists for %s", base.Digest, key)

		return base, b.index.Touch(key)
	}

	if !errors.Is(err, errBaseNotInIndex) {
		return nil, err
	}

	if err := b.mkfs.checkVersion(ctx); err != nil {
		return nil, err
	}

	logger.Infof("building base for %s from %d layers", key, len(layers.Layers))

	base, err = b.build(ctx, key, layers)
	if err != nil {
		return nil, fmt.Errorf("building base for %s: %w", key, err)
	}

	logger.Infof("built base %s for %s", base.Digest, key)

	return base, nil
}

// build runs the two mkfs.erofs steps in a scratch directory, then moves
// the merged file into bases/ under its sha256 and records the key. The
// scratch directory is removed on every return.
func (b *Builder) build(ctx context.Context, key baseKey, layers *ImageLayers) (*Base, error) {
	scratch, err := os.MkdirTemp(filepath.Join(b.stateRoot, scratchDir), "base-")
	if err != nil {
		return nil, fmt.Errorf("creating scratch directory: %w", err)
	}

	defer os.RemoveAll(scratch)

	layerFiles := make([]string, len(layers.Layers))

	for i, layer := range layers.Layers {
		out := filepath.Join(scratch, "layer-"+strconv.Itoa(i)+".erofs")

		if err := b.buildLayer(ctx, i, layer, out); err != nil {
			return nil, err
		}

		layerFiles[i] = out
	}

	merged := filepath.Join(scratch, "merged.erofs")
	fsUUID := BaseUUID(key.ChainID, key.Profile).String()

	if err := b.mkfs.run(ctx, b.mkfs.mergeArgs(fsUUID, merged, layerFiles), nil); err != nil {
		return nil, fmt.Errorf("merging layers: %w", err)
	}

	for _, f := range layerFiles {
		_ = os.Remove(f)
	}

	sum, err := syncAndSum(merged)
	if err != nil {
		return nil, err
	}

	final := filepath.Join(b.stateRoot, basesDir, sum+baseExt)

	if err := installBase(merged, final); err != nil {
		return nil, err
	}

	if err := b.index.Record(key, final); err != nil {
		return nil, err
	}

	return &Base{Digest: sum, Path: final, ChainID: key.ChainID, Profile: key.Profile}, nil
}

// buildLayer converts one layer's tar stream to the EROFS file out. The
// stream passes through a hardlink check. A hardlink whose target is not
// in the layer fails the build with a HardlinkError, whatever mkfs.erofs
// reported.
func (b *Builder) buildLayer(ctx context.Context, i int, layer Layer, out string) error {
	if err := layer.DiffID.Validate(); err != nil {
		return fmt.Errorf("layer %d: diff ID %q: %w", i, layer.DiffID, err)
	}

	if layer.DiffID.Algorithm() != digest.SHA256 {
		return fmt.Errorf("layer %d: diff ID %q: %w", i, layer.DiffID, errDiffIDNotSHA256)
	}

	stream, err := layer.Open(ctx)
	if err != nil {
		return fmt.Errorf("opening layer %d (%s): %w", i, layer.DiffID, err)
	}

	defer stream.Close()

	// Every byte mkfs.erofs reads also goes to the hardlink check and to
	// the digest that is compared with the diff ID.
	check := newHardlinkCheck()
	digester := digest.SHA256.Digester()
	in := io.TeeReader(check.reader(stream), digester.Hash())
	args := b.mkfs.layerArgs(LayerUUID(layer.DiffID).String(), out)

	runErr := b.mkfs.run(ctx, args, in)
	if ctx.Err() == nil {
		// mkfs.erofs stops at the end-of-archive marker, or at the first
		// bad entry. Read the rest so that the digest covers the whole
		// stream and the check reports an entry wherever it is.
		_, _ = io.Copy(io.Discard, in)
	}

	if entry, target, found := check.finish(); found {
		return &HardlinkError{LayerIndex: i, DiffID: layer.DiffID, Entry: entry, Target: target}
	}

	if runErr != nil {
		return fmt.Errorf("converting layer %d (%s): %w", i, layer.DiffID, runErr)
	}

	if got := digester.Digest(); got != layer.DiffID {
		return &DiffIDError{LayerIndex: i, Want: layer.DiffID, Got: got}
	}

	return nil
}

// syncAndSum fsyncs the file and returns its sha256 in hex.
func syncAndSum(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("opening %s: %w", path, err)
	}

	defer f.Close()

	if err := f.Sync(); err != nil {
		return "", fmt.Errorf("syncing %s: %w", path, err)
	}

	h := sha256.New()

	if _, err := io.Copy(h, f); err != nil {
		return "", fmt.Errorf("hashing %s: %w", path, err)
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}

// installBase makes the merged file read-only and renames it to final. A
// base that exists under the same digest holds the same bytes and is kept.
func installBase(merged, final string) error {
	if err := os.Chmod(merged, basePerm); err != nil {
		return fmt.Errorf("setting mode of %s: %w", merged, err)
	}

	_, err := os.Lstat(final)
	if err == nil {
		return nil
	}

	if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("stat %s: %w", final, err)
	}

	if err := os.Rename(merged, final); err != nil {
		return fmt.Errorf("moving base to %s: %w", final, err)
	}

	return syncDir(filepath.Dir(final))
}
