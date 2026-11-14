package queue_test

import (
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/liquidmetal-dev/flintlock/pkg/queue"
)

func TestSimpleSyncQueue_Basic(t *testing.T) {
	q := queue.NewSimpleSyncQueue()

	var countProduced int32
	var countConsumed int32

	numProduces := 10
	numItemsPerProducer := 10
	producersWG := sync.WaitGroup{}
	producersWG.Add(numProduces)
	for i := 0; i < numProduces; i++ {
		go func(i int) {
			defer producersWG.Done()
			for j := 0; j < numItemsPerProducer; j++ {
				offset := i * numItemsPerProducer
				id := offset + j
				vmid := fmt.Sprintf("ns1/vm%d", id)
				q.Enqueue(vmid)
				atomic.AddInt32(&countProduced, 1)
				time.Sleep(time.Millisecond)
			}
		}(i)
	}

	numConsumers := 5
	consumersWG := sync.WaitGroup{}
	consumersWG.Add(numConsumers)
	for i := 0; i < numConsumers; i++ {
		go func(i int) {
			defer consumersWG.Done()
			for {
				item, shutdown := q.Dequeue()
				if shutdown {
					return
				}
				if !strings.HasPrefix(item.(string), "ns1/vm") {
					t.Fatal("received item from queue after shutdown")
				}
				atomic.AddInt32(&countConsumed, 1)
				time.Sleep(3 * time.Millisecond)
			}
		}(i)
	}

	producersWG.Wait()
	t.Log("shutting queue down")
	q.Shutdown()
	t.Log("enqueing message after shutdown")
	q.Enqueue("added after shutdown")
	consumersWG.Wait()

	if countConsumed != countProduced {
		t.Fatalf("number of items enqueued (%d) should equal the number dequeued (%d)", countProduced, countConsumed)
	}
}

func TestSimpleSyncQueue_Duplicate(t *testing.T) {
	q := queue.NewSimpleSyncQueue()

	numItems := 10
	for i := 0; i < numItems; i++ {
		vmid := fmt.Sprintf("ns1/vm%d", i)
		q.Enqueue(vmid)
		q.Enqueue(vmid) // duplicate enqueue with the same id
		time.Sleep(time.Millisecond)
	}

	var countConsumed int32
	numConsumers := 5
	consumersWG := sync.WaitGroup{}
	consumersWG.Add(numConsumers)
	for i := 0; i < numConsumers; i++ {
		go func(i int) {
			defer consumersWG.Done()
			for {
				item, shutdown := q.Dequeue()
				if shutdown {
					return
				}
				if !strings.HasPrefix(item.(string), "ns1/vm") {
					t.Fatal("received item from queue after shutdown")
				}
				atomic.AddInt32(&countConsumed, 1)
				time.Sleep(3 * time.Millisecond)
			}
		}(i)
	}

	t.Log("shutting queue down")
	q.Shutdown()
	t.Log("enqueing message after shutdown")
	q.Enqueue("added after shutdown")
	consumersWG.Wait()

	if countConsumed != int32(numItems) {
		t.Fatalf("number of items enqueued (%d) should equal the number dequeued (%d)", numItems, countConsumed)
	}
}
