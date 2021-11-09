package planner

import (
	"context"
	"fmt"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/weaveworks/flintlock/pkg/log"
)

// Actuator will execute the given plan.
type Actuator interface {
	// Execute the plan.
	Execute(ctx context.Context, p Plan, executionID string) (int, error)
}

// NewActuator creates a new actuator.
func NewActuator() Actuator {
	return &actuatorImpl{}
}

type actuatorImpl struct{}

// Execute will execute the plan.
func (e *actuatorImpl) Execute(ctx context.Context, p Plan, executionID string) (int, error) {
	logger := log.GetLogger(ctx).WithFields(logrus.Fields{
		"execution_id": executionID,
		"plan_name":    p.Name(),
	})

	start := time.Now().UTC()

	logger.Infof("started executing plan")

	numStepsExecuted, err := e.executePlan(ctx, p, logger)
	if err != nil {
		logger.WithFields(logrus.Fields{
			"execution_time": time.Since(start),
			"num_steps":      numStepsExecuted,
		}).Error("failed executing plan")

		return numStepsExecuted, fmt.Errorf("executing plan steps: %w", err)
	}

	logger.WithFields(logrus.Fields{
		"execution_time": time.Since(start),
		"num_steps":      numStepsExecuted,
	}).Info("finished executing plan")

	return numStepsExecuted, nil
}

func (e *actuatorImpl) executePlan(ctx context.Context, p Plan, logger *logrus.Entry) (int, error) {
	numStepsExecuted := 0

	for {
		steps, err := p.Create(ctx)
		if err != nil {
			return numStepsExecuted, fmt.Errorf("creating plan for %s: %w", p.Name(), err)
		}

		if len(steps) == 0 {
			logger.Debug("no more steps to execute")

			return numStepsExecuted, nil
		}

		executed, err := e.react(ctx, steps, logger)
		numStepsExecuted += executed

		if err != nil {
			return numStepsExecuted, fmt.Errorf("executing steps: %w", err)
		}
	}
}

func (e *actuatorImpl) react(ctx context.Context, steps []Procedure, logger *logrus.Entry) (int, error) {
	var childSteps []Procedure

	numStepsExecuted := 0

	for _, step := range steps {
		select {
		case <-ctx.Done():
			logger.WithField("step_name", step.Name()).Info("step not executed due to context done")

			return numStepsExecuted, ctx.Err() //nolint:wrapcheck // It's ok ;)
		default:
			shouldDo, err := step.ShouldDo(ctx)
			if err != nil {
				return numStepsExecuted, fmt.Errorf("checking if step %s should be executed: %w", step.Name(), err)
			}

			if shouldDo {
				logger.WithField("step", step.Name()).Debug("execute step")

				numStepsExecuted++

				childSteps, err = step.Do(ctx)
				if err != nil {
					return numStepsExecuted, fmt.Errorf("executing step %s: %w", step.Name(), err)
				}
			}
		}

		if len(childSteps) > 0 {
			executed, err := e.react(ctx, childSteps, logger)
			numStepsExecuted += executed

			if err != nil {
				return numStepsExecuted, err
			}
		}
	}

	return numStepsExecuted, nil
}
