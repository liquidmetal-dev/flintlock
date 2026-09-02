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
