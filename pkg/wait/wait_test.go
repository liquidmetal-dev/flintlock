package wait_test

import (
	"sync"
	"testing"
	"time"

	. "github.com/onsi/gomega"
	"github.com/spf13/afero"

	"github.com/liquidmetal-dev/flintlock/pkg/wait"
)

func TestConditionMet(t *testing.T) {
	RegisterTestingT(t)

	fs := afero.NewMemMapFs()
	maxWait := 300 * time.Millisecond
	pollInterval := 100 * time.Millisecond
	testFile := "/test.txt"

	var conditionErr error
	var createErr error

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		conditionErr = wait.ForCondition(wait.FileExistsCondition(testFile, fs), maxWait, pollInterval)
	}()
	go func() {
		defer wg.Done()
		time.Sleep(50 * time.Millisecond)
		_, createErr = fs.Create(testFile)
	}()

	wg.Wait()
	Expect(conditionErr).NotTo(HaveOccurred())
	Expect(createErr).NotTo(HaveOccurred())
}

func TestConditionTimeout(t *testing.T) {
	RegisterTestingT(t)

	fs := afero.NewMemMapFs()
	maxWait := 50 * time.Millisecond
	pollInterval := 25 * time.Millisecond
	testFile := "/test.txt"

	var conditionErr error

	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		defer wg.Done()
		conditionErr = wait.ForCondition(wait.FileExistsCondition(testFile, fs), maxWait, pollInterval)
	}()

	wg.Wait()
	Expect(conditionErr).To(HaveOccurred())
}
