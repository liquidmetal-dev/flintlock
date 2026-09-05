package provision

import (
	"errors"
	"fmt"
	"os"
)

// EnsureKVM checks that /dev/kvm exists, returning an error if it doesn't.
func EnsureKVM() error {
	info, err := os.Stat("/dev/kvm")
	if err != nil {
		return fmt.Errorf("/dev/kvm not found, required for virtualisation: %w", err)
	}

	if info.Mode()&os.ModeCharDevice == 0 {
		return errors.New("/dev/kvm is not a character device")
	}

	return nil
}
