package shared_test

import (
	"testing"

	. "github.com/onsi/gomega"
	"github.com/spf13/afero"

	"github.com/liquidmetal-dev/flintlock/infrastructure/microvm/shared"
)

const (
	testSocketPath = "/run/flintlock/uid/test.sock"
	testLegacyPath = "/var/lib/flintlock/vm/ns/name/uid/test.sock"
)

func TestResolveSocketPath(t *testing.T) {
	tt := []struct {
		name     string
		existing []string
		expected string
	}{
		{
			name:     "when neither socket exists, the new path is returned",
			expected: testSocketPath,
		},
		{
			name:     "when only the new socket exists, the new path is returned",
			existing: []string{testSocketPath},
			expected: testSocketPath,
		},
		{
			name:     "when only the legacy socket exists, the legacy path is returned",
			existing: []string{testLegacyPath},
			expected: testLegacyPath,
		},
		{
			name:     "when both sockets exist, the new path is returned",
			existing: []string{testSocketPath, testLegacyPath},
			expected: testSocketPath,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)

			fs := afero.NewMemMapFs()
			for _, path := range tc.existing {
				g.Expect(afero.WriteFile(fs, path, nil, 0o644)).To(Succeed())
			}

			g.Expect(shared.ResolveSocketPath(fs, testSocketPath, testLegacyPath)).To(Equal(tc.expected))
		})
	}
}

func TestRemoveStaleSockets(t *testing.T) {
	g := NewWithT(t)

	fs := afero.NewMemMapFs()
	g.Expect(afero.WriteFile(fs, testLegacyPath, nil, 0o644)).To(Succeed())

	g.Expect(shared.RemoveStaleSockets(fs, testSocketPath, testLegacyPath)).To(Succeed())

	g.Expect(afero.Exists(fs, testLegacyPath)).To(BeFalse())
	g.Expect(afero.Exists(fs, testSocketPath)).To(BeFalse())
}
