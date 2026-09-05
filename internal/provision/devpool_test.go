package provision

import (
	"testing"

	. "github.com/onsi/gomega"
)

func TestParseSize(t *testing.T) {
	g := NewWithT(t)

	tests := []struct {
		size string
		want int64
	}{
		{size: "100G", want: 100 << 30},
		{size: "10G", want: 10 << 30},
		{size: "512M", want: 512 << 20},
		{size: "1024", want: 1024},
	}

	for _, tt := range tests {
		got, err := parseSize(tt.size)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(got).To(Equal(tt.want))
	}
}

func TestBuildThinPoolTable(t *testing.T) {
	g := NewWithT(t)

	got := BuildThinPoolTable(1000, "/dev/loop1", "/dev/loop0")

	g.Expect(got).To(Equal("0 1000 thin-pool /dev/loop1 /dev/loop0 128 32768 1 skip_block_zeroing"))
}
