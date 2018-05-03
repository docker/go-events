package events

import (
	"fmt"
	"sync"

	"github.com/sirupsen/logrus"
)

type eventRequest struct {
	event Event
	ch    chan error
}

// RoundRobin sends events to one of the Sinks in order. The goal of this
// component is to dispatch events to configured endpoints. Reliability can be
// provided by wrapping ingoing sinks.
type RoundRobin struct {
	sinks   []Sink
	cursor  int
	events  chan eventRequest
	adds    chan configureRequest
	removes chan configureRequest

	shutdown chan struct{}
	closed   chan struct{}
	once     sync.Once
}

// NewRoundRobin appends one or more sinks to the list of sinks. The
// round-robin behavior will be affected by the properties of the sink.
// Generally, the sink should accept all messages and deal with reliability on
// its own. Use of EventQueue and RetryingSink should be used here.
func NewRoundRobin(sinks ...Sink) *RoundRobin {
	b := RoundRobin{
		sinks:    sinks,
		events:   make(chan eventRequest),
		adds:     make(chan configureRequest),
		removes:  make(chan configureRequest),
		shutdown: make(chan struct{}),
		closed:   make(chan struct{}),
	}

	// Start the round-robin
	go b.run()

	return &b
}

// Write accepts an event to be dispatched to a sink.
func (rr *RoundRobin) Write(event Event) error {
	respChan := make(chan error, 1)
	request := eventRequest{event: event, ch: respChan}
	
	select {
	case rr.events <- request:
	case <-rr.closed:
		return ErrSinkClosed
	}

	select {
	case err := <-respChan:
		return err
	case <-rr.closed:
		return ErrSinkClosed
	}
}

// Add the sink to the broadcaster.
//
// The provided sink must be comparable with equality. Typically, this just
// works with a regular pointer type.
func (rr *RoundRobin) Add(sink Sink) error {
	return rr.configure(rr.adds, sink)
}

// Remove the provided sink.
func (rr *RoundRobin) Remove(sink Sink) error {
	return rr.configure(rr.removes, sink)
}

func (rr *RoundRobin) configure(ch chan configureRequest, sink Sink) error {
	response := make(chan error, 1)

	for {
		select {
		case ch <- configureRequest{
			sink:     sink,
			response: response}:
			ch = nil
		case err := <-response:
			return err
		case <-rr.closed:
			return ErrSinkClosed
		}
	}
}

// Close the round-robin, ensuring that all messages are flushed to the
// underlying sink before returning.
func (rr *RoundRobin) Close() error {
	rr.once.Do(func() {
		close(rr.shutdown)
	})

	<-rr.closed
	return nil
}

// run is the main round-robin loop, started when the round-robin is created.
// Under normal conditions, it waits for events on the event channel. After
// Close is called, this goroutine will exit.
func (rr *RoundRobin) run() {
	defer close(rr.closed)

	for {
		select {
		case request := <-rr.events:
			if len(rr.sinks) == 0 {
				request.ch <- ErrNoSinks
				break
			}

			rr.cursor++
			rr.cursor %= len(rr.sinks)

			for {
				sink := rr.sinks[rr.cursor]

				if err := sink.Write(request.event); err == ErrSinkClosed {
					// remove closed sinks
					rr.sinks = append(rr.sinks[:rr.cursor],
						rr.sinks[rr.cursor+1:]...)
					// check that it was not the only remaining sink
					if len(rr.sinks) == 0 {
						request.ch <- ErrNoSinks
						break
					}
					// continue from the start if it was the last sink
					rr.cursor %= len(rr.sinks)
				} else {
					request.ch <- err
					break
				}
			}
		case request := <-rr.adds:
			// while we have to iterate for add/remove, common iteration for
			// send is faster against slice.

			var found bool
			for _, sink := range rr.sinks {
				if request.sink == sink {
					found = true
					break
				}
			}

			if !found {
				rr.sinks = append(rr.sinks, request.sink)
			}
			request.response <- nil
		case request := <-rr.removes:
			for i, sink := range rr.sinks {
				if sink == request.sink {
					rr.sinks = append(rr.sinks[:i], rr.sinks[i+1:]...)
					if len(rr.sinks) == 0 {
						rr.cursor = 0
					} else if rr.cursor >= i {
						// decrease the cursor if the remove sink was before
						rr.cursor--
						rr.cursor %= len(rr.sinks)
					}
					break
				}
			}
			request.response <- nil
		case <-rr.shutdown:
			// close all the underlying sinks
			for _, sink := range rr.sinks {
				if err := sink.Close(); err != nil && err != ErrSinkClosed {
					logrus.WithField("events.sink", sink).WithError(err).
						Errorf("round-robin: closing sink failed")
				}
			}
			return
		}
	}
}

func (rr *RoundRobin) String() string {
	// Serialize copy of this round-robin without the sync.Once, to avoid
	// a data race.

	rr2 := map[string]interface{}{
		"sinks":   rr.sinks,
		"cursor":  rr.cursor,
		"events":  rr.events,
		"adds":    rr.adds,
		"removes": rr.removes,

		"shutdown": rr.shutdown,
		"closed":   rr.closed,
	}

	return fmt.Sprint(rr2)
}
