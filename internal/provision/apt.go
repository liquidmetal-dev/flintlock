package provision

import "fmt"

// AptPackages are the apt packages required to run flintlock.
var AptPackages = []string{"thin-provisioning-tools", "lvm2", "git", "curl", "wget"}

// InstallAptPackages installs AptPackages via apt.
func InstallAptPackages(runner *Runner) error {
	if err := runner.Run("apt", "update"); err != nil {
		return fmt.Errorf("updating apt package lists: %w", err)
	}

	args := append([]string{"install", "-qq", "-y"}, AptPackages...)
	if err := runner.Run("apt", args...); err != nil {
		return fmt.Errorf("installing apt packages: %w", err)
	}

	return nil
}
