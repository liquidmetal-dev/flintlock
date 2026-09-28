//go:build e2e
// +build e2e

package utils

import (
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

// Provider describes a microvm provider of flintlockd as the tests see it
// from the outside. The names of the files are not taken from the code of the
// providers, so that a test fails if a provider changes one of them.
type Provider struct {
	// Name is the name of the provider in the API.
	Name string
	// Binary is the name of the VMM binary which the provider starts.
	Binary string
	// BinaryFlag is the flag of flintlockd for the path of the VMM binary.
	BinaryFlag string
	// PidFile is the file in the state directory of a microvm which holds the
	// pid of the VMM.
	PidFile string
	// ConsoleFile is the file in the state directory of a microvm which the
	// console of the guest is written to.
	ConsoleFile string
	// DiagnosticFiles are the files in the state directory of a microvm which
	// show why a microvm did not start.
	DiagnosticFiles []string
	// KernelImage is an image with a kernel which the VMM can boot.
	KernelImage string
	// KernelFilename is the path of the kernel in the KernelImage.
	KernelFilename string
}

// BinaryPath returns the path of the VMM binary of the provider, which is
// looked up on the PATH. It is the path which the kernel reports for a
// process of the binary.
func (p Provider) BinaryPath() (string, error) {
	found, err := exec.LookPath(p.Binary)
	if err != nil {
		return "", fmt.Errorf("looking for the VMM of the %s provider: %w", p.Name, err)
	}

	absolute, err := filepath.Abs(found)
	if err != nil {
		return "", fmt.Errorf("getting the path of %s: %w", found, err)
	}

	resolved, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		return "", fmt.Errorf("resolving the links in %s: %w", absolute, err)
	}

	return resolved, nil
}

// BinaryVersion returns the path of the VMM binary of the provider, and the
// first line of what the binary prints as its version.
func (p Provider) BinaryVersion() (string, string, error) {
	binary, err := p.BinaryPath()
	if err != nil {
		return "", "", err
	}

	output, err := exec.Command(binary, "--version").Output()
	if err != nil {
		return "", "", fmt.Errorf("getting the version of the VMM of the %s provider with '%s --version': %w",
			p.Name, binary, err)
	}

	version, _, _ := strings.Cut(strings.TrimSpace(string(output)), "\n")

	return binary, version, nil
}

// Firecracker returns the description of the firecracker provider.
func Firecracker() Provider {
	return Provider{
		Name:            "firecracker",
		Binary:          "firecracker",
		BinaryFlag:      "--firecracker-bin",
		PidFile:         "firecracker.pid",
		ConsoleFile:     "firecracker.stdout",
		DiagnosticFiles: []string{"firecracker.stdout", "firecracker.stderr", "firecracker.log"},
		// Firecracker describes the devices of the guest with ACPI. A kernel
		// needs PCI to use the ACPI tables, which this one has. A kernel with
		// ACPI and without PCI cannot find its devices and panics, see #1262.
		KernelImage:    "ghcr.io/liquidmetal-dev/firecracker-kernel:6.1",
		KernelFilename: "boot/vmlinux",
	}
}

// CloudHypervisor returns the description of the cloudhypervisor provider.
func CloudHypervisor() Provider {
	return Provider{
		Name:            "cloudhypervisor",
		Binary:          "cloud-hypervisor-static",
		BinaryFlag:      "--cloudhypervisor-bin",
		PidFile:         "cloudhypervisor.pid",
		ConsoleFile:     "cloudhypervisor.stdout",
		DiagnosticFiles: []string{"cloudhypervisor.stdout", "cloudhypervisor.stderr", "cloudhypervisor.log"},
		// Cloud Hypervisor boots the kernel from its PVH entry point and attaches
		// the devices with virtio over PCI. The kernel which the firecracker
		// tests use does not have virtio over PCI.
		KernelImage:    "ghcr.io/liquidmetal-dev/cloudhypervisor-kernel-bin:5.12",
		KernelFilename: "boot/vmlinux.bin",
	}
}

// ProviderFlags returns the flags which make flintlockd use the VMM binaries
// of the providers, with the first of the providers as the default provider.
// It is an error if the VMM binary of a provider is not on the PATH.
func ProviderFlags(providers []Provider) ([]string, error) {
	if len(providers) == 0 {
		return nil, errors.New("no providers given")
	}

	flags := []string{"--default-provider", providers[0].Name}

	for _, provider := range providers {
		binary, err := provider.BinaryPath()
		if err != nil {
			return nil, err
		}

		flags = append(flags, provider.BinaryFlag, binary)
	}

	return flags, nil
}

// KnownProviders returns all of the providers which the tests know about.
func KnownProviders() []Provider {
	return []Provider{Firecracker(), CloudHypervisor()}
}

// ParseProviders returns the providers which are named in a comma separated
// list, in the order of the list.
func ParseProviders(value string) ([]Provider, error) {
	if strings.TrimSpace(value) == "" {
		return nil, errors.New("no providers given")
	}

	providers := []Provider{}
	seen := map[string]bool{}

	for _, name := range strings.Split(value, ",") {
		name = strings.TrimSpace(name)

		provider, err := providerByName(name)
		if err != nil {
			return nil, err
		}

		if seen[name] {
			return nil, fmt.Errorf("provider %q is given more than once", name)
		}

		seen[name] = true

		providers = append(providers, provider)
	}

	return providers, nil
}

func providerByName(name string) (Provider, error) {
	known := []string{}

	for _, provider := range KnownProviders() {
		if provider.Name == name {
			return provider, nil
		}

		known = append(known, provider.Name)
	}

	return Provider{}, fmt.Errorf("unknown provider %q, the known providers are: %s", name, strings.Join(known, ", "))
}
