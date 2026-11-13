package overlay

import (
	"github.com/google/uuid"
	"github.com/opencontainers/go-digest"
)

// ProfileNameErofsV1 is the name of the first base profile.
const ProfileNameErofsV1 = "erofs-v1"

// uuidNamespaceName is the URL the UUID namespace of every base is derived
// from.
const uuidNamespaceName = "https://liquidmetal.dev/flintlock/erofs-base"

// Profile fixes every input of a base build. A base is keyed by the chain
// ID of its image and the name of the profile. A change to any input is a
// new profile name and so a new base identity.
type Profile struct {
	// Name is the profile name. It is part of the base key.
	Name string
	// ErofsUtilsVersion is the version mkfs.erofs must report. The builder
	// refuses any other version.
	ErofsUtilsVersion string
	// BlockSize is the EROFS block size in bytes. It is passed explicitly
	// because the default of mkfs.erofs is the page size of the build host.
	BlockSize int
}

// ErofsV1 returns the profile erofs-v1: erofs-utils 1.9.4, 4096-byte
// blocks, no compression, a zero build time with the file times of the
// layers kept, and the UUIDs of LayerUUID and BaseUUID.
func ErofsV1() Profile {
	return Profile{
		Name:              ProfileNameErofsV1,
		ErofsUtilsVersion: "1.9.4",
		BlockSize:         4096,
	}
}

// uuidNamespace is uuid5(NAMESPACE_URL, uuidNamespaceName).
func uuidNamespace() uuid.UUID {
	return uuid.NewSHA1(uuid.NameSpaceURL, []byte(uuidNamespaceName))
}

// LayerUUID is the filesystem UUID of the EROFS file built from one layer.
// It is a version 5 UUID of the diff ID string, for example
// "sha256:0123...", in the namespace of the package.
func LayerUUID(diffID digest.Digest) uuid.UUID {
	return uuid.NewSHA1(uuidNamespace(), []byte(diffID.String()))
}

// BaseUUID is the filesystem UUID of a merged base. It is a version 5 UUID
// of "<chain-id>/<profile>", for example "sha256:0123.../erofs-v1", in the
// namespace of the package.
func BaseUUID(chainID digest.Digest, profile string) uuid.UUID {
	return uuid.NewSHA1(uuidNamespace(), []byte(baseKey{ChainID: chainID, Profile: profile}.String()))
}
