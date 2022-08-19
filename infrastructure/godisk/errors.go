package godisk

import "errors"

var (
	errPathRequired = errors.New("path is required to create a disk")
	errSizeRequired = errors.New("size is required to create a disk")
)
