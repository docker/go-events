package events

import (
	"container/list"
	"sync"

	"github.com/sirupsen/logrus"
)

// Queue accepts all messages into a queue for asynchronous consumption
// by a sink. It is unbounded and thread safe but the sink must be reliable or
// events will be dropped.
type Queue struct {
	dst       Sink
	events    *list.List
	listeners []QueueListener
	cond      *sync.Cond
	mu        sync.Mutex
	closed    bool
}

// QueueListener is called when various events happen on the queue.
type QueueListener interface {
	Ingress(event Event)
	Egress(event Event)
}

// NewQueue returns a queue to the provided Sink dst.
func NewQueue(dst Sink) *Queue {
	eq := Queue{
		dst:    dst,
		events: list.New(),
	}

	eq.cond = sync.NewCond(&eq.mu)
	go eq.run()
	return &eq
}

// NewListenerQueue returns a queue to the provided Sink dst.  If the updater
// is non-nil, it will be called to update pending metrics on ingress and
// egress.
func NewListenerQueue(dst Sink, listeners ...QueueListener) *Queue {
	eq := Queue{
		dst:       dst,
		events:    list.New(),
		listeners: listeners,
	}

	eq.cond = sync.NewCond(&eq.mu)
	go eq.run()
	return &eq
}

// Write accepts the events into the queue, only failing if the queue has
// been closed.
func (eq *Queue) Write(event Event) error {
	eq.mu.Lock()
	defer eq.mu.Unlock()

	if eq.closed {
		return ErrSinkClosed
	}

	for _, listener := range eq.listeners {
		listener.Ingress(event)
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
func (eq *Queue) run() {
	for {
		event := eq.next()

		if event == nil {
			return // nil block means event queue is closed.
		}

		if err := eq.dst.Write(event); err != nil {
			// TODO(aaronl): Dropping events could be bad depending
			// on the application. We should have a way of
			// communicating this condition. However, logging
			// at a log level above debug may not be appropriate.
			// Eventually, go-events should not use logrus at all,
			// and should bubble up conditions like this through
			// error values.
			logrus.WithFields(logrus.Fields{
				"event": event,
				"sink":  eq.dst,
			}).WithError(err).Debug("eventqueue: dropped event")
		}

		for _, listener := range eq.listeners {
			listener.Egress(event)
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
