package provision

import "fmt"

// NormaliseArch maps a uname -m style architecture name to the amd64/arm64
// naming used by flintlock/cloud-hypervisor/containerd release artifacts.
func NormaliseArch(unameArch string) (string, error) {
	switch unameArch {
	case "x86_64", "amd64":
		return "amd64", nil
	case "aarch64", "arm64":
		return "arm64", nil
	default:
		return "", fmt.Errorf("unknown or unsupported architecture: %s", unameArch)
	}
}

// FirecrackerArch maps a normalised amd64/arm64 architecture name to the
// uname -m style naming firecracker's own release artifacts use
// (e.g. firecracker-v1.7.0-x86_64.tgz).
func FirecrackerArch(normalisedArch string) (string, error) {
	switch normalisedArch {
	case "amd64":
		return "x86_64", nil
	case "arm64":
		return "aarch64", nil
	default:
		return "", fmt.Errorf("unknown or unsupported architecture: %s", normalisedArch)
	}
}
