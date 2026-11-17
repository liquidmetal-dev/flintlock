package plans

import "errors"

var (
	errVolumeServiceMissing  = errors.New("volume service is not available: an overlay root volume needs it")
	errSnapshotRepoMissing   = errors.New("snapshot repository is not available: a restore needs it")
	errRestoreRootNotOverlay = errors.New("a restored microvm needs an overlay root volume")
)
