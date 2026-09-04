package provision

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
)

// Confirm asks the user to confirm msg via in, returning true if they
// answered "y". Unattended callers should skip calling this altogether,
// matching the script's get_user_confirmation.
func Confirm(in io.Reader, out io.Writer, msg string) bool {
	if msg == "" {
		msg = "Continue? (y/n) "
	}

	fmt.Fprint(out, msg)

	scanner := bufio.NewScanner(in)
	if !scanner.Scan() {
		return false
	}

	return strings.TrimSpace(scanner.Text()) == "y"
}

// EnsureKVM checks that /dev/kvm exists, returning an error if it doesn't.
func EnsureKVM() error {
	info, err := os.Stat("/dev/kvm")
	if err != nil {
		return fmt.Errorf("/dev/kvm not found, required for virtualisation: %w", err)
	}

	if info.Mode()&os.ModeCharDevice == 0 {
		return fmt.Errorf("/dev/kvm is not a character device")
	}

	return nil
}
