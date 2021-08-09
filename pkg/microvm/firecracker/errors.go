package firecracker

import "errors"

var (
	errNotImplemeted      = errors.New("not implemented")
	errSocketPathRequired = errors.New("socket path is required")
)
