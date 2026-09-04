package provision

import (
	"bytes"
	"fmt"
	"io"
	"os/exec"
	"strings"
)

// Runner is the single seam provisioning logic uses to execute host
// commands (systemctl, apt, lvm2 tools, dmsetup, ...). Keeping every
// exec.Command call behind this type means the rest of the package never
// shells out directly.
type Runner struct {
	Stdout io.Writer
	Stderr io.Writer
}

// NewRunner returns a Runner that streams command output to the given writers.
func NewRunner(stdout, stderr io.Writer) *Runner {
	return &Runner{Stdout: stdout, Stderr: stderr}
}

// Run executes name with args, streaming its output to the runner's writers.
func (r *Runner) Run(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = r.Stdout
	cmd.Stderr = r.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("running %s: %w", commandString(name, args), err)
	}

	return nil
}

// Output executes name with args and returns its trimmed stdout.
func (r *Runner) Output(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)

	var stdout bytes.Buffer

	cmd.Stdout = &stdout
	cmd.Stderr = r.Stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("running %s: %w", commandString(name, args), err)
	}

	return strings.TrimSpace(stdout.String()), nil
}

// Contains reports whether the output of name with args contains substr.
// It is a convenience for the "if already exists, do nothing" checks the
// original provisioning script uses around pvdisplay/vgdisplay/lvdisplay/lvs.
func (r *Runner) Contains(substr, name string, args ...string) bool {
	out, err := r.Output(name, args...)
	if err != nil {
		return false
	}

	return strings.Contains(out, substr)
}

func commandString(name string, args []string) string {
	return strings.TrimSpace(name + " " + strings.Join(args, " "))
}
