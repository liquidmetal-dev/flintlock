package log

import (
	"errors"
	"fmt"
)

// ErrLogOutputRequired is used when no log output is specified.
var ErrLogOutputRequired = errors.New("you must specify a log output")

type errInvalidLogFormat struct {
	format string
}

func (e errInvalidLogFormat) Error() string {
	return fmt.Sprintf("logger format %s is invalid", e.format)
}

// IsInvalidLogFormat tests an error to see if its a invalid log format error.
func IsInvalidLogFormat(err error) bool {
	var e errInvalidLogFormat

	return errors.Is(err, e)
}
