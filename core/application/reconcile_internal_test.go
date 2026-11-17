package application

import (
	"testing"

	g "github.com/onsi/gomega"

	"github.com/liquidmetal-dev/flintlock/core/models"
)

func TestReconciledState(t *testing.T) {
	tt := []struct {
		name  string
		power models.PowerState
		want  models.MicroVMState
	}{
		{name: "no power state", want: models.CreatedState},
		{name: "wants running", power: models.PowerStateRunning, want: models.CreatedState},
		{name: "wants paused", power: models.PowerStatePaused, want: models.PausedState},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			g.RegisterTestingT(t)

			vm := &models.MicroVM{Spec: models.MicroVMSpec{PowerState: tc.power}}

			g.Expect(reconciledState(vm)).To(g.Equal(tc.want))
		})
	}
}
