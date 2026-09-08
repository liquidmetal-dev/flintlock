package vsockexec

import (
	"testing"
	"time"

	"github.com/liquidmetal-dev/flintlock/pkg/defaults"
)

func TestIdleTimeoutFor(t *testing.T) {
	tests := []struct {
		name       string
		timeoutSec int
		want       time.Duration
	}{
		{
			name:       "unbounded falls back to the ceiling",
			timeoutSec: 0,
			want:       defaults.ExecSessionUnboundedIdleCeiling,
		},
		{
			name:       "bounded adds grace on top of TimeoutSec",
			timeoutSec: 5,
			want:       5*time.Second + defaults.ExecSessionIdleGrace,
		},
		{
			name:       "longer bounded request still gets the same grace",
			timeoutSec: 120,
			want:       120*time.Second + defaults.ExecSessionIdleGrace,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := idleTimeoutFor(tt.timeoutSec); got != tt.want {
				t.Errorf("idleTimeoutFor(%d) = %v, want %v", tt.timeoutSec, got, tt.want)
			}
		})
	}
}
