package network_test

import (
	"strings"

	"github.com/liquidmetal-dev/flintlock/core/ports"
)

type hostDeviceNameMatcher struct{}

// Matches returns whether name is a match.
func (a *hostDeviceNameMatcher) Matches(value interface{}) bool {
	name, ok := value.(string)
	if !ok {
		return false
	}

	return randomDeviceNameCheck(name)
}

// String describes what the matcher matches.
func (a *hostDeviceNameMatcher) String() string {
	return "Test random generated HostDeviceName"
}

type ifaceCreateInputMatcher struct{}

// Matches returns whether name is a match.
func (a *ifaceCreateInputMatcher) Matches(value interface{}) bool {
	input, ok := value.(ports.IfaceCreateInput)
	if !ok {
		return false
	}

	return randomDeviceNameCheck(input.DeviceName)
}

// String describes what the matcher matches.
func (a *ifaceCreateInputMatcher) String() string {
	return "Test random generated HostDeviceName"
}

func randomDeviceNameCheck(name string) bool {
	rightPrefix := strings.HasPrefix(name, "fltap") || strings.HasPrefix(name, "flvtap")
	length := len(name)

	return rightPrefix && length >= 12 && length <= 13
}
