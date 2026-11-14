package application

import (
	"fmt"
	"sync"
)

// VMLocks is a mutex per microvm, keyed by the uid of the microvm. Work that
// must not overlap on one microvm, such as a snapshot capture and a reconcile,
// holds its lock. The zero value is not usable: use NewVMLocks.
type VMLocks struct {
	mu    sync.Mutex
	locks map[string]*vmLock
}

type vmLock struct {
	mu sync.Mutex
	// users counts the holder and the waiters, so the entry is removed only
	// when no one needs it.
	users int
}

// NewVMLocks returns a set of microvm locks with every lock free.
func NewVMLocks() *VMLocks {
	return &VMLocks{locks: map[string]*vmLock{}}
}

// Lock takes the lock of the microvm with the uid. It waits until the lock is free.
func (l *VMLocks) Lock(uid string) {
	l.acquire(uid).mu.Lock()
}

// TryLock takes the lock of the microvm with the uid if it is free. It reports
// whether it took the lock.
func (l *VMLocks) TryLock(uid string) bool {
	lock := l.acquire(uid)
	if lock.mu.TryLock() {
		return true
	}

	l.release(uid)

	return false
}

// Unlock releases the lock of the microvm with the uid. It panics when the
// lock is not held, as sync.Mutex does.
func (l *VMLocks) Unlock(uid string) {
	l.mu.Lock()
	lock, ok := l.locks[uid]
	l.mu.Unlock()

	if !ok {
		panic(fmt.Sprintf("unlock of the lock of microvm %s, which is not locked", uid))
	}

	lock.mu.Unlock()
	l.release(uid)
}

func (l *VMLocks) acquire(uid string) *vmLock {
	l.mu.Lock()
	defer l.mu.Unlock()

	lock, ok := l.locks[uid]
	if !ok {
		lock = &vmLock{}
		l.locks[uid] = lock
	}

	lock.users++

	return lock
}

func (l *VMLocks) release(uid string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	lock := l.locks[uid]

	lock.users--
	if lock.users == 0 {
		delete(l.locks, uid)
	}
}
