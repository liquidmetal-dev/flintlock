package application_test

import (
	"sync"
	"testing"
	"time"

	. "github.com/onsi/gomega"

	"github.com/liquidmetal-dev/flintlock/core/application"
)

func TestVMLocks_TryLock(t *testing.T) {
	RegisterTestingT(t)

	locks := application.NewVMLocks()

	Expect(locks.TryLock("vm1")).To(BeTrue())
	Expect(locks.TryLock("vm1")).To(BeFalse())
	Expect(locks.TryLock("vm2")).To(BeTrue(), "the lock of another microvm is free")

	locks.Unlock("vm1")

	Expect(locks.TryLock("vm1")).To(BeTrue())
}

func TestVMLocks_LockWaitsForUnlock(t *testing.T) {
	RegisterTestingT(t)

	locks := application.NewVMLocks()
	locks.Lock("vm1")

	acquired := make(chan struct{})

	go func() {
		locks.Lock("vm1")
		close(acquired)
	}()

	Consistently(acquired, 50*time.Millisecond).ShouldNot(BeClosed())

	locks.Unlock("vm1")

	Eventually(acquired).Should(BeClosed())
	Expect(locks.TryLock("vm1")).To(BeFalse())
}

func TestVMLocks_Concurrent(t *testing.T) {
	RegisterTestingT(t)

	locks := application.NewVMLocks()
	counter := 0

	var wg sync.WaitGroup

	for range 50 {
		wg.Add(1)

		go func() {
			defer wg.Done()

			locks.Lock("vm1")
			defer locks.Unlock("vm1")

			counter++
		}()
	}

	wg.Wait()

	Expect(counter).To(Equal(50))
	Expect(locks.TryLock("vm1")).To(BeTrue())
}

func TestVMLocks_UnlockOfUnlockedPanics(t *testing.T) {
	RegisterTestingT(t)

	locks := application.NewVMLocks()

	Expect(func() { locks.Unlock("vm1") }).To(Panic())
}
