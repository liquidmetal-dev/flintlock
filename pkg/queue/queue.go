package queue

import (
	"sync"

	"github.com/google/go-cmp/cmp"
)

// NOTE: this is heavily based on the workerqueue from client-go:
// https://github.com/kubernetes/client-go/blob/master/util/workqueue/queue.go

// Queue is the interface for a queue.
type Queue interface {
	// Enqueue will add an item to the queue for processing. If the item being enqueued already exists then
	// it will be ignored.
	Enqueue(item interface{})
	// Dequeue will get an item from the queue. If there are no items on the queue then it will wait.
	Dequeue() (interface{}, bool)
	// Shutdown will cause the queue processing to shutdown.
	Shutdown()
}

// NewSimpleSyncQueue create a new simple sync queue.
func NewSimpleSyncQueue() Queue {
	return &simpleSyncQueue{
		items:     []interface{}{},
		emptyCond: sync.NewCond(&sync.Mutex{}),
	}
}

type simpleSyncQueue struct {
	items []interface{}

	emptyCond    *sync.Cond
	shuttingDown bool
}

func (q *simpleSyncQueue) Enqueue(item interface{}) {
	q.emptyCond.L.Lock()
	defer q.emptyCond.L.Unlock()

	if q.shuttingDown {
		return
	}

	if q.exists(item) {
		// We already have the item so ignore
		return
	}

	q.items = append(q.items, item)
	q.emptyCond.Signal()
}

func (q *simpleSyncQueue) exists(item interface{}) bool {
	for _, currentItem := range q.items {
		if cmp.Equal(currentItem, item) {
			return true
		}
	}

	return false
}

func (q *simpleSyncQueue) Dequeue() (interface{}, bool) {
	q.emptyCond.L.Lock()
	defer q.emptyCond.L.Unlock()

	for len(q.items) == 0 && !q.shuttingDown {
		q.emptyCond.Wait()
	}

	if len(q.items) == 0 {
		return nil, true
	}

	var item interface{}
	item, q.items = q.items[0], q.items[1:]

	return item, false
}

func (q *simpleSyncQueue) Shutdown() {
	q.emptyCond.L.Lock()
	defer q.emptyCond.L.Unlock()
	q.shuttingDown = true
	q.emptyCond.Broadcast()
}
