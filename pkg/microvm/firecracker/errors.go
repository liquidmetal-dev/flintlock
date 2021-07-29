package firecracker

import "errors"

var (
	errNotImplemeted      = errors.New("not implemeted")
	errSocketPathRequired = errors.New("socket path is required")
)
