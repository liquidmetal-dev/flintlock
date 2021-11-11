package wait

import (
	"errors"
	"fmt"
	"time"

	"github.com/spf13/afero"
)

// ErrWaitTimeout is an error when a condition hasn't been met within the supplied max wait duration.
var ErrWaitTimeout = errors.New("timeout waiting for condition")

// ConditionFunc is a function type that is used to implement a wait condition.
type ConditionFunc func() (bool, error)

// ForCondition will wait for the specified condition to be true until the max duration.
func ForCondition(conditionFn ConditionFunc, maxWait time.Duration, checkInternal time.Duration) error {
	timeout := time.NewTimer(maxWait)
	defer timeout.Stop()

	checkTicker := time.NewTicker(checkInternal)
	defer checkTicker.Stop()

	for {
		conditionMet, err := conditionFn()
		if err != nil {
			return fmt.Errorf("checking if condition met: %w", err)
		}

		if conditionMet {
			return nil
		}

		select {
		case <-timeout.C:
			return ErrWaitTimeout
		case <-checkTicker.C:
			continue
		}
	}
}

// FileExistsCondition creates a condition check on the existence of a file.
func FileExistsCondition(filepath string, fs afero.Fs) ConditionFunc {
	return func() (bool, error) {
		return afero.Exists(fs, filepath) //nolint: wrapcheck // It's ok ;)
	}
}
