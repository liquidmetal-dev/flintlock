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
// branch and writes it to dest, then reloads the systemd daemon.
func FetchServiceFile(ctx context.Context, runner *Runner, repo, service, dest string) error {
	url := RawURL(repo, service)

	if err := DownloadFile(ctx, url, dest, 0o664); err != nil {
		return fmt.Errorf("fetching service file %s: %w", service, err)
	}

	if err := runner.Run("systemctl", "daemon-reload"); err != nil {
		return err
	}

	return nil
}

// StartService enables and starts the given systemd service.
func StartService(runner *Runner, service string) error {
	if err := runner.Run("systemctl", "enable", service); err != nil {
		return fmt.Errorf("enabling %s service: %w", service, err)
	}

	if err := runner.Run("systemctl", "start", service); err != nil {
		return fmt.Errorf("starting %s service: %w", service, err)
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

	if err := os.WriteFile(path, []byte(updated), 0o664); err != nil {
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

	if err := os.WriteFile(path, []byte(updated), 0o664); err != nil {
		return fmt.Errorf("writing %s: %w", path, err)
	}

	return nil
}
