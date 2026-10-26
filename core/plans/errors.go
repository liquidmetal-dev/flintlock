package plans

import "errors"

var errVolumeServiceMissing = errors.New("volume service is not available: an overlay root volume needs it")
