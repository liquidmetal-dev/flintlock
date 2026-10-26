package config_test

import (
	"testing"

	"github.com/liquidmetal-dev/flintlock/internal/config"
	"github.com/liquidmetal-dev/flintlock/pkg/defaults"
)

func TestConfig_EffectiveOverlayInitrdImage(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name         string
		flagImage    string
		buildVersion string
		expected     string
	}{
		{
			name:         "release build uses the initrd of its version",
			buildVersion: "0.16.0",
			expected:     defaults.OverlayInitrdImage + ":0.16.0",
		},
		{
			name:         "release build with a v prefix uses the tag as given",
			buildVersion: "v0.16.0",
			expected:     defaults.OverlayInitrdImage + ":v0.16.0",
		},
		{
			name:         "dev build has no default",
			buildVersion: "undefined",
			expected:     "",
		},
		{
			name:         "snapshot build has no default",
			buildVersion: "0.16.1-next",
			expected:     "",
		},
		{
			name:         "the flag wins over the release version",
			flagImage:    "ghcr.io/example/overlay-init:custom",
			buildVersion: "0.16.0",
			expected:     "ghcr.io/example/overlay-init:custom",
		},
		{
			name:         "the flag gives a dev build a default",
			flagImage:    "ghcr.io/example/overlay-init:custom",
			buildVersion: "undefined",
			expected:     "ghcr.io/example/overlay-init:custom",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			cfg := &config.Config{OverlayInitrdImage: tc.flagImage}

			if got := cfg.EffectiveOverlayInitrdImage(tc.buildVersion); got != tc.expected {
				t.Errorf("EffectiveOverlayInitrdImage(%q) = %q, want %q", tc.buildVersion, got, tc.expected)
			}
		})
	}
}
