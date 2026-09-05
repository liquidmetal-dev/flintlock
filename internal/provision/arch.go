package provision

import "fmt"

const (
	archAMD64 = "amd64"
	archARM64 = "arm64"

	unameAMD64 = "x86_64"
	unameARM64 = "aarch64"
)

// NormaliseArch maps a uname -m style architecture name to the amd64/arm64
// naming used by flintlock/cloud-hypervisor/containerd release artefacts.
func NormaliseArch(unameArch string) (string, error) {
	switch unameArch {
	case unameAMD64, archAMD64:
		return archAMD64, nil
	case unameARM64, archARM64:
		return archARM64, nil
	default:
		return "", fmt.Errorf("unknown or unsupported architecture: %s", unameArch)
	}
}

// UnameArch maps a normalised amd64/arm64 architecture name back to the
// uname -m style naming used in firecracker's and cloud-hypervisor's own
// release artefacts (e.g. firecracker-v1.7.0-x86_64.tgz,
// cloud-hypervisor-static-aarch64).
func UnameArch(normalisedArch string) (string, error) {
	switch normalisedArch {
	case archAMD64:
		return unameAMD64, nil
	case archARM64:
		return unameARM64, nil
	default:
		return "", fmt.Errorf("unknown or unsupported architecture: %s", normalisedArch)
	}
}
