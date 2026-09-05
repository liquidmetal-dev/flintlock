package provision

import (
	"context"
	"fmt"
	"os"
	"regexp"
)

var (
	requiresLineRE  = regexp.MustCompile(`(?m)^(Requires=).*$`)
	execStartLineRE = regexp.MustCompile(`(?m)^(ExecStart=.*)$`)
)

// FetchServiceFile downloads the named systemd unit file from repo's default
// branch and writes it to dest. Callers that go on to edit dest (e.g. via
// ReplaceRequires or appendExecStartArg) should call ReloadSystemd
// themselves once all edits are done, rather than reloading here with an
// unedited unit.
func FetchServiceFile(ctx context.Context, repo, service, dest string) error {
	url := RawURL(repo, service)

	if err := DownloadFile(ctx, url, dest, 0o644); err != nil {
		return fmt.Errorf("fetching service file %s: %w", service, err)
	}

	return nil
}

// ReloadSystemd reloads the systemd manager configuration so unit file
// changes on disk take effect.
func ReloadSystemd(runner *Runner) error {
	if err := runner.Run("systemctl", "daemon-reload"); err != nil {
		return fmt.Errorf("reloading systemd: %w", err)
	}

	return nil
}

// StartService enables the given systemd service and starts it, or
// restarts it if it's already active so a changed unit/config is applied.
func StartService(runner *Runner, service string) error {
	if err := runner.Run("systemctl", "enable", service); err != nil {
		return fmt.Errorf("enabling %s service: %w", service, err)
	}

	action := "start"
	if runner.Run("systemctl", "is-active", "--quiet", service) == nil {
		action = "restart"
	}

	if err := runner.Run("systemctl", action, service); err != nil {
		return fmt.Errorf("running systemctl %s for %s service: %w", action, service, err)
	}

	return nil
}

// ReplaceRequires rewrites the "Requires=" line of a systemd unit file at
// path so it requires the given service, matching the script's use of sed
// to point flintlockd.service at the correct (possibly "-dev" tagged)
// containerd service.
func ReplaceRequires(path, service string) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("reading %s: %w", path, err)
	}

	updated := replaceRequiresLine(string(content), service)

	//nolint:gosec // systemd units must be world-readable
	if err := os.WriteFile(path, []byte(updated), 0o644); err != nil {
		return fmt.Errorf("writing %s: %w", path, err)
	}

	return nil
}

func replaceRequiresLine(unit, service string) string {
	return requiresLineRE.ReplaceAllString(unit, "${1}"+service)
}

// appendExecStartArg appends arg to the ExecStart= line of the systemd unit
// file at path, matching the script's `sed -i "s|ExecStart=.*|& $arg|"`.
func appendExecStartArg(path, arg string) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("reading %s: %w", path, err)
	}

	updated := execStartLineRE.ReplaceAllString(string(content), "${1} "+arg)

	//nolint:gosec // systemd units must be world-readable
	if err := os.WriteFile(path, []byte(updated), 0o644); err != nil {
		return fmt.Errorf("writing %s: %w", path, err)
	}

	return nil
}
