package plans_test

import (
	"strings"

	"github.com/liquidmetal-dev/flintlock/core/models"
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

type deleteInterfaceMatcher struct{}

// Matches returns whether name is a match.
func (a *deleteInterfaceMatcher) Matches(value interface{}) bool {
	input, ok := value.(ports.DeleteIfaceInput)
	if !ok {
		return false
	}

	return randomDeviceNameCheck(input.DeviceName)
}

// String describes what the matcher matches.
func (a *deleteInterfaceMatcher) String() string {
	return "Test random generated HostDeviceName"
}

type createInterfaceMatcher struct {
	DeviceName string
	MAC        string
	Type       models.IfaceType
}

// Matches returns whether name is a match.
func (a *createInterfaceMatcher) Matches(value interface{}) bool {
	input, ok := value.(ports.IfaceCreateInput)
	if !ok {
		return false
	}

	if a.MAC != "" && a.MAC != input.MAC {
		return false
	}

	if a.Type != input.Type {
		return false
	}

	if a.DeviceName != "" {
		return a.DeviceName != input.DeviceName
	}

	return randomDeviceNameCheck(input.DeviceName)
}

// String describes what the matcher matches.
func (a *createInterfaceMatcher) String() string {
	return "Test random generated HostDeviceName"
}

func randomDeviceNameCheck(name string) bool {
	rightPrefix := strings.HasPrefix(name, "fltap") || strings.HasPrefix(name, "flvtap")
	length := len(name)

	return rightPrefix && length >= 12 && length <= 13
}
