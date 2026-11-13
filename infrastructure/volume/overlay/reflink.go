package overlay

import "github.com/liquidmetal-dev/flintlock/pkg/reflink"

// ProbeReflink checks that FICLONE works in dir. The error wraps
// ErrReflinkNotSupported when the filesystem of dir cannot clone.
func ProbeReflink(dir string) error {
	return reflink.Probe(dir)
}
