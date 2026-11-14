package provision

import (
	"testing"

	. "github.com/onsi/gomega"
)

const sampleUnit = `[Unit]
Description=flintlockd
Requires=containerd.service

[Service]
ExecStart=/usr/local/bin/flintlockd run

[Install]
WantedBy=multi-user.target
`

func TestReplaceRequiresLine(t *testing.T) {
	g := NewWithT(t)

	got := replaceRequiresLine(sampleUnit, "containerd-dev.service")

	g.Expect(got).To(ContainSubstring("Requires=containerd-dev.service"))
	g.Expect(got).NotTo(ContainSubstring("Requires=containerd.service\n"))
}
