//go:build e2e
// +build e2e

package e2e_test

import (
	"testing"

	. "github.com/onsi/gomega"

	u "github.com/liquidmetal-dev/flintlock/test/e2e/utils"
)

func TestPrivateImageRef(t *testing.T) {
	g := NewWithT(t)

	g.Expect(u.PrivateImageRef("ghcr.io/liquidmetal-dev/flintlock-kernel:5.10.77")).
		To(Equal("127.0.0.1:5050/liquidmetal-dev/flintlock-kernel:5.10.77"))
}

func TestContainerdMajorVersion(t *testing.T) {
	tt := []struct {
		name      string
		output    string
		expected  int
		expectErr bool
	}{
		{
			name:     "v2 release build",
			output:   "containerd github.com/containerd/containerd/v2 v2.2.9 1294c24a7da8e5a793ed378161673abe94118892\n",
			expected: 2,
		},
		{
			name:     "v1 release build",
			output:   "containerd github.com/containerd/containerd v1.7.28 b98a3aace656320842a23f4a392a33f46af97866\n",
			expected: 1,
		},
		{
			name:     "distro build without v prefix or revision",
			output:   "containerd github.com/containerd/containerd/v2 2.2.1-0ubuntu1~24.04.1 \n",
			expected: 2,
		},
		{
			name:      "too few fields",
			output:    "containerd\n",
			expectErr: true,
		},
		{
			name:      "version is not a number",
			output:    "containerd github.com/containerd/containerd/v2 unknown abc\n",
			expectErr: true,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)

			major, err := u.ContainerdMajorVersion(tc.output)
			if tc.expectErr {
				g.Expect(err).To(HaveOccurred())

				return
			}

			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(major).To(Equal(tc.expected))
		})
	}
}
