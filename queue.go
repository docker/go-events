package events

import (
	"container/list"
	"context"
	"sync"
)

// Queue accepts all messages into a queue for asynchronous consumption
// by a sink. It is unbounded and thread safe but the sink must be reliable or
// events will be dropped.
type Queue struct {
	dst    Sink
	events *list.List
	cond   *sync.Cond
	mu     sync.Mutex
	errs   chan error
	closed bool
}

// NewQueue returns a queue to the provided Sink dst.
//
// Deprecated: use [events.NewAsyncQueue] instead.
func NewQueue(dst Sink) *Queue {
	eq, _ := NewAsyncQueue(context.TODO(), dst)
	return eq
}

type QueueOpt func(*Queue)

func NewAsyncQueue(ctx context.Context, dst Sink, opts ...QueueOpt) (*Queue, <-chan error) {
	eq := &Queue{
		dst:    dst,
		events: list.New(),
	}

	eq.cond = sync.NewCond(&eq.mu)

	for _, opt := range opts {
		opt(eq)
	}

	go eq.run(ctx)
	return eq, eq.errs
}

func WithBufferedChannel(capacity int) QueueOpt {
	return func(q *Queue) {
		q.errs = make(chan error, capacity)
	}
}

func WithBlockingChannel() QueueOpt {
	return func(q *Queue) {
		q.errs = make(chan error)
	}
}

// Write accepts the events into the queue, only failing if the queue has
// been closed.
func (eq *Queue) Write(event Event) error {
	eq.mu.Lock()
	defer eq.mu.Unlock()

	if eq.closed {
		return ErrSinkClosed
	}

	eq.events.PushBack(event)
	eq.cond.Signal() // signal waiters

	return nil
}

// Close shutsdown the event queue, flushing
func (eq *Queue) Close() error {
	eq.mu.Lock()
	defer eq.mu.Unlock()

	if eq.closed {
		return nil
	}

	// set closed flag
	eq.closed = true
	eq.cond.Signal() // signal flushes queue
	eq.cond.Wait()   // wait for signal from last flush
	return eq.dst.Close()
}

// run is the main goroutine to flush events to the target sink.
func (eq *Queue) run(ctx context.Context) {
	for {
		event := eq.next()

		if event == nil {
			if eq.errs != nil {
				close(eq.errs)
			}
			return // nil block means event queue is closed.
		}

		if err := eq.dst.Write(event); err != nil {
			if eq.errs != nil {
				eq.errs <- err
			}
		}
	}
}

// next encompasses the critical section of the run loop. When the queue is
// empty, it will block on the condition. If new data arrives, it will wake
// and return a block. When closed, a nil slice will be returned.
func (eq *Queue) next() Event {
	eq.mu.Lock()
	defer eq.mu.Unlock()

	for eq.events.Len() < 1 {
		if eq.closed {
			eq.cond.Broadcast()
			return nil
		}

		eq.cond.Wait()
	}

	front := eq.events.Front()
	block := front.Value.(Event)
	eq.events.Remove(front)

	return block
}
