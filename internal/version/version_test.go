package version_test

import (
	"testing"

	"github.com/liquidmetal-dev/flintlock/internal/version"
)

func TestIsRelease(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		version  string
		expected bool
	}{
		{version: "0.16.0", expected: true},
		{version: "v0.16.0", expected: true},
		{version: "1.0.0", expected: true},
		{version: "v10.2.33", expected: true},
		{version: "undefined", expected: false},
		{version: "", expected: false},
		{version: "0.16.0-next", expected: false},
		{version: "0.16.1-SNAPSHOT-abc1234", expected: false},
		{version: "v0.16.0-rc.1", expected: false},
		{version: "0.16.0+build.1", expected: false},
		{version: "0.16", expected: false},
		{version: "v0.16.0.1", expected: false},
		{version: "01.2.3", expected: false},
		{version: "main", expected: false},
	}

	for _, tc := range testCases {
		t.Run(tc.version, func(t *testing.T) {
			t.Parallel()

			if got := version.IsRelease(tc.version); got != tc.expected {
				t.Errorf("IsRelease(%q) = %v, want %v", tc.version, got, tc.expected)
			}
		})
	}
}
