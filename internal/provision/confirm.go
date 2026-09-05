package provision

import (
	"bufio"
	"fmt"
	"io"
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
