package microvm

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	g "github.com/onsi/gomega"

	internalerr "github.com/liquidmetal-dev/flintlock/core/errors"
	"github.com/liquidmetal-dev/flintlock/core/ports"
	"github.com/liquidmetal-dev/flintlock/infrastructure/mock"
)

const waitTestVMID = "ns/vm/uid"

func TestWaitForRunning_ReturnsWhenRunning(t *testing.T) {
	g.RegisterTestingT(t)
	svc := mock.NewMockMicroVMService(gomock.NewController(t))
	ctx := context.Background()

	gomock.InOrder(
		svc.EXPECT().State(ctx, waitTestVMID).Return(ports.MicroVMStateConfigured, nil),
		svc.EXPECT().State(ctx, waitTestVMID).Return(ports.MicroVMStateConfigured, nil),
		svc.EXPECT().State(ctx, waitTestVMID).Return(ports.MicroVMStateRunning, nil),
	)

	state, err := waitForRunning(ctx, svc, waitTestVMID, WaitConfig{Timeout: time.Second, Interval: time.Millisecond})

	g.Expect(err).NotTo(g.HaveOccurred())
	g.Expect(state).To(g.Equal(ports.MicroVMStateRunning))
}

func TestWaitForRunning_TimesOut(t *testing.T) {
	g.RegisterTestingT(t)
	svc := mock.NewMockMicroVMService(gomock.NewController(t))
	ctx := context.Background()

	svc.EXPECT().State(ctx, waitTestVMID).Return(ports.MicroVMStatePending, nil).AnyTimes()

	state, err := waitForRunning(ctx, svc, waitTestVMID, WaitConfig{Timeout: 20 * time.Millisecond, Interval: time.Millisecond})

	g.Expect(err).To(g.MatchError(internalerr.ErrUnableToBoot))
	g.Expect(state).To(g.Equal(ports.MicroVMStatePending))
}

func TestWaitForRunning_StateError(t *testing.T) {
	g.RegisterTestingT(t)
	svc := mock.NewMockMicroVMService(gomock.NewController(t))
	ctx := context.Background()
	stateErr := errors.New("no state")

	svc.EXPECT().State(ctx, waitTestVMID).Return(ports.MicroVMStateUnknown, stateErr).Times(1)

	_, err := waitForRunning(ctx, svc, waitTestVMID, WaitConfig{Timeout: time.Second, Interval: time.Millisecond})

	g.Expect(err).To(g.MatchError(stateErr))
}

func TestWaitForRunning_ContextCancelled(t *testing.T) {
	g.RegisterTestingT(t)
	svc := mock.NewMockMicroVMService(gomock.NewController(t))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	svc.EXPECT().State(ctx, waitTestVMID).Return(ports.MicroVMStateConfigured, nil).AnyTimes()

	start := time.Now()
	_, err := waitForRunning(ctx, svc, waitTestVMID, WaitConfig{Timeout: time.Hour, Interval: 10 * time.Millisecond})

	g.Expect(err).To(g.MatchError(context.Canceled))
	g.Expect(time.Since(start)).To(g.BeNumerically("<", time.Second))
}
