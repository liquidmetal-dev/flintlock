package firecracker_test

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"testing"

	"github.com/weaveworks-liquidmetal/flintlock/infrastructure/firecracker"
)

func TestUnmarshallWithFCSample(t *testing.T) {
	file, err := os.Open("testdata/vm_config.json")
	if err != nil {
		t.Fatal(err)
	}

	data, err := ioutil.ReadAll(file)
	if err != nil {
		t.Fatal(err)
	}

	cfg := &firecracker.VmmConfig{}
	err = json.Unmarshal(data, cfg)
	if err != nil {
		t.Fatal(err)
	}
}
