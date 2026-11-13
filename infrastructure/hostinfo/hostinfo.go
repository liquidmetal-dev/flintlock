// Package hostinfo collects the facts of the host that decide if it can
// restore a snapshot.
package hostinfo

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"

	"golang.org/x/sys/unix"

	"github.com/liquidmetal-dev/flintlock/core/models"
	"github.com/liquidmetal-dev/flintlock/core/ports"
)

// DefaultCPUInfoPath is the path of the cpuinfo file of the host.
const DefaultCPUInfoPath = "/proc/cpuinfo"

var (
	errUnknownProvider      = errors.New("unknown microvm provider")
	errProviderNotEnabled   = errors.New("microvm provider is not enabled")
	errNoVersionOutput      = errors.New("no version in the output")
	errNoProcessorInCPUInfo = errors.New("no processor in cpuinfo")
)

// Config is the configuration of the host info service.
type Config struct {
	// FirecrackerBin is the Firecracker binary. An empty value means the
	// Firecracker provider is not enabled.
	FirecrackerBin string
	// CloudHypervisorBin is the Cloud Hypervisor binary. An empty value means
	// the Cloud Hypervisor provider is not enabled.
	CloudHypervisorBin string
	// CPUInfoPath is the path of the cpuinfo file. An empty value means
	// DefaultCPUInfoPath.
	CPUInfoPath string
}

// commandRunner runs a command and returns its standard output.
type commandRunner func(ctx context.Context, name string, args ...string) ([]byte, error)

// unameRelease returns the kernel release of the host.
type unameRelease func() (string, error)

// New returns a host info service.
func New(cfg Config) ports.HostInfoService {
	return newService(cfg, runCommand, kernelRelease)
}

func newService(cfg Config, run commandRunner, uname unameRelease) *service {
	if cfg.CPUInfoPath == "" {
		cfg.CPUInfoPath = DefaultCPUInfoPath
	}

	return &service{
		cfg:   cfg,
		run:   run,
		uname: uname,
		cache: map[cacheKey]string{},
	}
}

type service struct {
	cfg   Config
	run   commandRunner
	uname unameRelease

	mu    sync.Mutex
	cache map[cacheKey]string
}

// cacheKey identifies the output of one command on one binary. The mtime is
// part of the key, so a replaced binary is run again.
type cacheKey struct {
	path  string
	mtime time.Time
	arg   string
}

// Facts returns the host facts for the named microvm provider.
func (s *service) Facts(ctx context.Context, provider string) (*models.HostFacts, error) {
	var bin string

	switch provider {
	case models.VMMTypeFirecracker:
		bin = s.cfg.FirecrackerBin
	case models.VMMTypeCloudHypervisor:
		bin = s.cfg.CloudHypervisorBin
	default:
		return nil, fmt.Errorf("%w: %s", errUnknownProvider, provider)
	}

	if bin == "" {
		return nil, fmt.Errorf("%w: %s", errProviderNotEnabled, provider)
	}

	facts := &models.HostFacts{
		VMMType: provider,
		Arch:    runtime.GOARCH,
	}

	var err error

	facts.VMMVersion, err = s.binaryVersion(ctx, bin, "--version")
	if err != nil {
		return nil, err
	}

	if provider == models.VMMTypeFirecracker {
		facts.FCSnapshotVersion, err = s.binaryVersion(ctx, bin, "--snapshot-version")
		if err != nil {
			return nil, err
		}
	}

	facts.HostKernel, err = s.uname()
	if err != nil {
		return nil, fmt.Errorf("reading the kernel release: %w", err)
	}

	cpu, err := s.readCPUInfo()
	if err != nil {
		return nil, err
	}

	facts.CPUVendor = cpu.vendor
	facts.CPUModel = cpu.model
	facts.CPUMicrocode = cpu.microcode

	return facts, nil
}

// binaryVersion runs bin with arg and returns the version it prints. It runs
// the binary once for each path and mtime, and caches the result.
func (s *service) binaryVersion(ctx context.Context, bin, arg string) (string, error) {
	path, err := exec.LookPath(bin)
	if err != nil {
		return "", fmt.Errorf("finding %s: %w", bin, err)
	}

	info, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("reading %s: %w", path, err)
	}

	key := cacheKey{path: path, mtime: info.ModTime(), arg: arg}

	s.mu.Lock()
	defer s.mu.Unlock()

	if version, ok := s.cache[key]; ok {
		return version, nil
	}

	out, err := s.run(ctx, path, arg)
	if err != nil {
		return "", fmt.Errorf("running %s %s: %w", path, arg, err)
	}

	version, err := parseVersion(out)
	if err != nil {
		return "", fmt.Errorf("parsing the output of %s %s: %w", path, arg, err)
	}

	s.cache[key] = version

	return version, nil
}

// parseVersion returns the last word of the first line that is not empty.
// Firecracker prints "Firecracker v1.10.1" for --version and "v4.0.0" for
// --snapshot-version. Cloud Hypervisor prints "cloud-hypervisor v41.0.0".
func parseVersion(out []byte) (string, error) {
	scanner := bufio.NewScanner(bytes.NewReader(out))
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) > 0 {
			return fields[len(fields)-1], nil
		}
	}

	if err := scanner.Err(); err != nil {
		return "", err
	}

	return "", errNoVersionOutput
}

type cpuInfo struct {
	vendor    string
	model     string
	microcode string
}

func (s *service) readCPUInfo() (cpuInfo, error) {
	content, err := os.ReadFile(s.cfg.CPUInfoPath)
	if err != nil {
		return cpuInfo{}, fmt.Errorf("reading %s: %w", s.cfg.CPUInfoPath, err)
	}

	cpu, err := parseCPUInfo(content)
	if err != nil {
		return cpuInfo{}, fmt.Errorf("parsing %s: %w", s.cfg.CPUInfoPath, err)
	}

	return cpu, nil
}

// parseCPUInfo returns the vendor_id, model name and microcode of the first
// processor in the content of /proc/cpuinfo. A blank line ends the block of a
// processor. A field that is not in the block is empty: arm64 hosts, for
// example, have no vendor_id.
func parseCPUInfo(content []byte) (cpuInfo, error) {
	var (
		cpu   cpuInfo
		found bool
	)

	scanner := bufio.NewScanner(bytes.NewReader(content))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			if found {
				break
			}

			continue
		}

		key, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}

		found = true
		value = strings.TrimSpace(value)

		switch strings.TrimSpace(key) {
		case "vendor_id":
			cpu.vendor = value
		case "model name":
			cpu.model = value
		case "microcode":
			cpu.microcode = value
		}
	}

	if err := scanner.Err(); err != nil {
		return cpuInfo{}, err
	}

	if !found {
		return cpuInfo{}, errNoProcessorInCPUInfo
	}

	return cpu, nil
}

func runCommand(ctx context.Context, name string, args ...string) ([]byte, error) {
	return exec.CommandContext(ctx, name, args...).Output()
}

func kernelRelease() (string, error) {
	var uts unix.Utsname
	if err := unix.Uname(&uts); err != nil {
		return "", err
	}

	return unix.ByteSliceToString(uts.Release[:]), nil
}
