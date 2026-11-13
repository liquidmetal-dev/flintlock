package hostinfo

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	g "github.com/onsi/gomega"

	"github.com/liquidmetal-dev/flintlock/core/models"
	"github.com/liquidmetal-dev/flintlock/infrastructure/microvm/cloudhypervisor"
	"github.com/liquidmetal-dev/flintlock/infrastructure/microvm/firecracker"
)

const testKernel = "6.8.0-45-generic"

func readFixture(t *testing.T, name string) []byte {
	t.Helper()

	content, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("reading fixture %s: %v", name, err)
	}

	return content
}

// fakeRunner returns fixture output for each argument and counts the calls.
type fakeRunner struct {
	outputs map[string][]byte
	calls   map[string]int
}

func newFakeRunner(t *testing.T, fixtures map[string]string) *fakeRunner {
	t.Helper()

	runner := &fakeRunner{outputs: map[string][]byte{}, calls: map[string]int{}}
	for arg, name := range fixtures {
		runner.outputs[arg] = readFixture(t, name)
	}

	return runner
}

func (r *fakeRunner) run(_ context.Context, _ string, args ...string) ([]byte, error) {
	arg := strings.Join(args, " ")
	r.calls[arg]++

	out, ok := r.outputs[arg]
	if !ok {
		return nil, errors.New("unexpected arguments: " + arg)
	}

	return out, nil
}

func fakeUname() (string, error) {
	return testKernel, nil
}

// fakeBinary writes an executable file, so that the service can find it and
// read its mtime.
func fakeBinary(t *testing.T) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "vmm")
	if err := os.WriteFile(path, []byte("#!/bin/false\n"), 0o755); err != nil { //nolint:gosec // must be executable
		t.Fatalf("writing fake binary: %v", err)
	}

	return path
}

func TestVMMTypesMatchProviderNames(t *testing.T) {
	g.RegisterTestingT(t)

	g.Expect(models.VMMTypeFirecracker).To(g.Equal(firecracker.ProviderName))
	g.Expect(models.VMMTypeCloudHypervisor).To(g.Equal(cloudhypervisor.ProviderName))
}

func TestParseCPUInfo_AMD64(t *testing.T) {
	g.RegisterTestingT(t)

	cpu, err := parseCPUInfo(readFixture(t, "cpuinfo_amd64"))
	g.Expect(err).NotTo(g.HaveOccurred())
	g.Expect(cpu).To(g.Equal(cpuInfo{
		vendor:    "GenuineIntel",
		model:     "Intel(R) Xeon(R) Platinum 8488C",
		microcode: "0x2b000603",
	}))
}

func TestParseCPUInfo_ARM64HasNoVendor(t *testing.T) {
	g.RegisterTestingT(t)

	cpu, err := parseCPUInfo(readFixture(t, "cpuinfo_arm64"))
	g.Expect(err).NotTo(g.HaveOccurred())
	g.Expect(cpu).To(g.Equal(cpuInfo{}))
}

func TestParseCPUInfo_Empty(t *testing.T) {
	g.RegisterTestingT(t)

	_, err := parseCPUInfo([]byte("\n\n"))
	g.Expect(err).To(g.MatchError(errNoProcessorInCPUInfo))
}

func TestParseVersion(t *testing.T) {
	testCases := []struct {
		fixture  string
		expected string
	}{
		{fixture: "firecracker_version", expected: "v1.10.1"},
		{fixture: "firecracker_snapshot_version", expected: "v4.0.0"},
		{fixture: "cloudhypervisor_version", expected: "v41.0.0"},
	}

	for _, tc := range testCases {
		t.Run(tc.fixture, func(t *testing.T) {
			g.RegisterTestingT(t)

			version, err := parseVersion(readFixture(t, tc.fixture))
			g.Expect(err).NotTo(g.HaveOccurred())
			g.Expect(version).To(g.Equal(tc.expected))
		})
	}
}

func TestParseVersion_Empty(t *testing.T) {
	g.RegisterTestingT(t)

	_, err := parseVersion([]byte("\n  \n"))
	g.Expect(err).To(g.MatchError(errNoVersionOutput))
}

func TestFacts_Firecracker(t *testing.T) {
	g.RegisterTestingT(t)

	runner := newFakeRunner(t, map[string]string{
		"--version":          "firecracker_version",
		"--snapshot-version": "firecracker_snapshot_version",
	})
	svc := newService(Config{
		FirecrackerBin: fakeBinary(t),
		CPUInfoPath:    filepath.Join("testdata", "cpuinfo_amd64"),
	}, runner.run, fakeUname)

	facts, err := svc.Facts(context.Background(), models.VMMTypeFirecracker)
	g.Expect(err).NotTo(g.HaveOccurred())
	g.Expect(facts).To(g.Equal(&models.HostFacts{
		VMMType:           models.VMMTypeFirecracker,
		VMMVersion:        "v1.10.1",
		FCSnapshotVersion: "v4.0.0",
		Arch:              runtime.GOARCH,
		CPUVendor:         "GenuineIntel",
		CPUModel:          "Intel(R) Xeon(R) Platinum 8488C",
		CPUMicrocode:      "0x2b000603",
		HostKernel:        testKernel,
	}))
}

func TestFacts_CloudHypervisor(t *testing.T) {
	g.RegisterTestingT(t)

	runner := newFakeRunner(t, map[string]string{"--version": "cloudhypervisor_version"})
	svc := newService(Config{
		CloudHypervisorBin: fakeBinary(t),
		CPUInfoPath:        filepath.Join("testdata", "cpuinfo_amd64"),
	}, runner.run, fakeUname)

	facts, err := svc.Facts(context.Background(), models.VMMTypeCloudHypervisor)
	g.Expect(err).NotTo(g.HaveOccurred())
	g.Expect(facts.VMMType).To(g.Equal(models.VMMTypeCloudHypervisor))
	g.Expect(facts.VMMVersion).To(g.Equal("v41.0.0"))
	g.Expect(facts.FCSnapshotVersion).To(g.BeEmpty())
	g.Expect(runner.calls).To(g.Equal(map[string]int{"--version": 1}))
}

func TestFacts_CachesVersionPerPathAndMtime(t *testing.T) {
	g.RegisterTestingT(t)

	bin := fakeBinary(t)
	runner := newFakeRunner(t, map[string]string{
		"--version":          "firecracker_version",
		"--snapshot-version": "firecracker_snapshot_version",
	})
	svc := newService(Config{
		FirecrackerBin: bin,
		CPUInfoPath:    filepath.Join("testdata", "cpuinfo_amd64"),
	}, runner.run, fakeUname)

	for range 3 {
		_, err := svc.Facts(context.Background(), models.VMMTypeFirecracker)
		g.Expect(err).NotTo(g.HaveOccurred())
	}

	g.Expect(runner.calls).To(g.Equal(map[string]int{"--version": 1, "--snapshot-version": 1}))

	// A new mtime means a replaced binary, so the service runs it again.
	newTime := time.Now().Add(time.Hour)
	g.Expect(os.Chtimes(bin, newTime, newTime)).To(g.Succeed())

	_, err := svc.Facts(context.Background(), models.VMMTypeFirecracker)
	g.Expect(err).NotTo(g.HaveOccurred())
	g.Expect(runner.calls).To(g.Equal(map[string]int{"--version": 2, "--snapshot-version": 2}))
}

func TestFacts_ProviderErrors(t *testing.T) {
	g.RegisterTestingT(t)

	runner := newFakeRunner(t, nil)
	svc := newService(Config{FirecrackerBin: fakeBinary(t)}, runner.run, fakeUname)

	_, err := svc.Facts(context.Background(), "qemu")
	g.Expect(err).To(g.MatchError(errUnknownProvider))

	_, err = svc.Facts(context.Background(), models.VMMTypeCloudHypervisor)
	g.Expect(err).To(g.MatchError(errProviderNotEnabled))
}

func TestFacts_MissingCPUInfo(t *testing.T) {
	g.RegisterTestingT(t)

	runner := newFakeRunner(t, map[string]string{"--version": "cloudhypervisor_version"})
	svc := newService(Config{
		CloudHypervisorBin: fakeBinary(t),
		CPUInfoPath:        filepath.Join(t.TempDir(), "missing"),
	}, runner.run, fakeUname)

	_, err := svc.Facts(context.Background(), models.VMMTypeCloudHypervisor)
	g.Expect(err).To(g.MatchError(os.ErrNotExist))
}

func TestNew_DefaultCPUInfoPath(t *testing.T) {
	g.RegisterTestingT(t)

	svc := newService(Config{}, nil, nil)
	g.Expect(svc.cfg.CPUInfoPath).To(g.Equal(DefaultCPUInfoPath))
}
