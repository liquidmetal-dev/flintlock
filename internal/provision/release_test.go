package provision_test

import (
	"testing"

	. "github.com/onsi/gomega"

	"github.com/liquidmetal-dev/flintlock/internal/provision"
)

func TestDownloadURL(t *testing.T) {
	g := NewWithT(t)

	got := provision.DownloadURL("firecracker-microvm/firecracker", "v1.7.0", "firecracker-v1.7.0-x86_64.tgz")
	want := "https://github.com/firecracker-microvm/firecracker/releases/download/v1.7.0/firecracker-v1.7.0-x86_64.tgz"

	g.Expect(got).To(Equal(want))
}

func TestRawURL(t *testing.T) {
	g := NewWithT(t)

	got := provision.RawURL("liquidmetal-dev/flintlock", "flintlockd.service")
	want := "https://raw.githubusercontent.com/liquidmetal-dev/flintlock/main/flintlockd.service"

	g.Expect(got).To(Equal(want))
}

func TestContainerdReleaseBinName(t *testing.T) {
	g := NewWithT(t)

	tests := []struct {
		tag  string
		arch string
		want string
	}{
		{tag: "v1.7.13", arch: "amd64", want: "containerd-1.7.13-linux-amd64.tar.gz"},
		{tag: "v1.7.13", arch: "arm64", want: "containerd-1.7.13-linux-arm64.tar.gz"},
		{tag: "1.7.13", arch: "amd64", want: "containerd-1.7.13-linux-amd64.tar.gz"},
	}

	for _, tt := range tests {
		got := provision.ContainerdReleaseBinName(tt.tag, tt.arch)
		g.Expect(got).To(Equal(tt.want))
	}
}
