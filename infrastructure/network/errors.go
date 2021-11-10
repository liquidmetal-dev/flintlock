package network

import "fmt"

// InterfaceError occurs when something went wrong
// with network interface magic.
type InterfaceError string

func (e InterfaceError) Error() string {
	return string(e)
}

func interfaceErrorf(format string, params ...interface{}) InterfaceError {
	return InterfaceError(fmt.Sprintf(format, params...))
}
