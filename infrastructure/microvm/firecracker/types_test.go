package firecracker_test

import (
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/liquidmetal-dev/flintlock/infrastructure/microvm/firecracker"
)

func TestUnmarshallWithFCSample(t *testing.T) {
	file, err := os.Open("testdata/vm_config.json")
	if err != nil {
		t.Fatal(err)
	}

	data, err := io.ReadAll(file)
	if err != nil {
		t.Fatal(err)
	}

	cfg := &firecracker.VmmConfig{}
	err = json.Unmarshal(data, cfg)
	if err != nil {
		t.Fatal(err)
	}

	if cfg.MachineConfig.VcpuCount != 2 {
		t.Fatalf("expected vcpu_count to be 2, got: %d", cfg.MachineConfig.VcpuCount)
	}

	if cfg.MachineConfig.MemSizeMib != 1024 {
		t.Fatalf("expected mem_size_mib to be 1024, got: %d", cfg.MachineConfig.MemSizeMib)
	}

	if cfg.MachineConfig.SMT {
		t.Fatalf("expected smt to be false, got: %t", cfg.MachineConfig.SMT)
	}
}

func TestVmmConfig_CPUConfigMarshalling(t *testing.T) {
	withCPUConfig := &firecracker.VmmConfig{
		CPUConfig: &firecracker.CPUConfig{KvmCapabilities: []string{"171", "!56"}},
	}

	data, err := json.Marshal(withCPUConfig)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(string(data), `"cpu-config":{"kvm_capabilities":["171","!56"]}`) {
		t.Fatalf("expected cpu-config to be marshalled, got: %s", data)
	}

	withoutCPUConfig := &firecracker.VmmConfig{}

	data, err = json.Marshal(withoutCPUConfig)
	if err != nil {
		t.Fatal(err)
	}

	if strings.Contains(string(data), "cpu-config") {
		t.Fatalf("expected cpu-config to be omitted, got: %s", data)
	}
}
