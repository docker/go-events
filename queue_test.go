package events

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestQueue(t *testing.T) {
	const nevents = 1000

	ts := newTestSink(t, nevents)
	eq := NewQueue(
		// delayed sync simulates destination slower than channel comms
		&delayedSink{
			Sink:  ts,
			delay: time.Millisecond * 1,
		})
	time.Sleep(10 * time.Millisecond) // let's queue settle to wait condition.

	var wg sync.WaitGroup
	for i := 1; i <= nevents; i++ {
		wg.Add(1)
		go func(event Event) {
			if err := eq.Write(event); err != nil {
				t.Fatalf("error writing event: %v", err)
			}
			wg.Done()
		}("event-" + fmt.Sprint(i))
	}

	wg.Wait()
	checkClose(t, eq)

	ts.mu.Lock()
	defer ts.mu.Unlock()

	if len(ts.events) != nevents {
		t.Fatalf("events did not make it to the sink: %d != %d", len(ts.events), 1000)
	}

	if !ts.closed {
		t.Fatalf("sink should have been closed")
	}
}

func TestListenerQueue(t *testing.T) {
	const nevents = 1000
	ts := newTestSink(t, nevents)
	metrics := newSafeMetrics()
	eq := NewListenerQueue(
		// delayed sync simulates destination slower than channel comms
		&delayedSink{
			Sink:  ts,
			delay: time.Millisecond * 1,
		}, metrics.eventQueueListener())
	time.Sleep(10 * time.Millisecond) // let's queue settle to wait condition.

	var wg sync.WaitGroup
	for i := 1; i <= nevents; i++ {
		wg.Add(1)
		go func(event Event) {
			if err := eq.Write(event); err != nil {
				t.Fatalf("error writing event: %v", err)
			}
			wg.Done()
		}("event-" + fmt.Sprint(i))
	}

	wg.Wait()
	checkClose(t, eq)

	ts.mu.Lock()
	defer ts.mu.Unlock()
	metrics.Lock()
	defer metrics.Unlock()

	if len(ts.events) != nevents {
		t.Fatalf("events did not make it to the sink: %d != %d", len(ts.events), nevents)
	}

	if !ts.closed {
		t.Fatalf("sink should have been closed")
	}

	if metrics.Events != nevents {
		t.Fatalf("unexpected ingress count: %d != %d", metrics.Events, nevents)
	}

	if metrics.Pending != 0 {
		t.Fatalf("unexpected egress count: %d != %d", metrics.Pending, 0)
	}
}

type testMetrics struct {
	Pending   int            // events pending in queue
	Events    int            // total events incoming
	Successes int            // total events written successfully
	Failures  int            // total events failed
	Errors    int            // total events errored
	Statuses  map[string]int // status code histogram, per call event
}

// safeMetrics guards the metrics implementation with a lock and provides a
// safe update function.
type safeMetrics struct {
	testMetrics
	sync.Mutex // protects statuses map
}

// newSafeMetrics returns safeMetrics with map allocated.
func newSafeMetrics() *safeMetrics {
	var sm safeMetrics
	sm.Statuses = make(map[string]int)
	return &sm
}

// eventQueueListener returns a listener that maintains queue related counters.
func (sm *safeMetrics) eventQueueListener() QueueListener {
	return &testMetricsEventQueueListener{
		safeMetrics: sm,
	}
}

// testMetricsEventQueueListener maintains the incoming events counter and
// the queues pending count.
type testMetricsEventQueueListener struct {
	*safeMetrics
}

func (eqc *testMetricsEventQueueListener) Ingress(event Event) {
	eqc.Lock()
	defer eqc.Unlock()
	eqc.Events++
	eqc.Pending++
}

func (eqc *testMetricsEventQueueListener) Egress(event Event) {
	eqc.Lock()
	defer eqc.Unlock()
	eqc.Pending--
}
