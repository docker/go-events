package events

import (
	"fmt"
	"sync"
	"testing"
)

func TestListener(t *testing.T) {
	const nevents = 100
	tm := &testMetrics{0, 0, 0}

	ts := newTestSink(t, nevents)
	es := NewListenerSink(ts, tm.egress)
	eq := NewQueue(es)
	is := NewListenerSink(eq, tm.ingress)

	var wg sync.WaitGroup
	for i := 1; i <= nevents; i++ {
		wg.Add(1)
		go func(event Event) {
			if err := is.Write(event); err != nil {
				t.Fatalf("error writing event: %v", err)
			}
			wg.Done()
		}("event-" + fmt.Sprint(i))
	}
	wg.Wait()
	checkClose(t, is)

	ts.mu.Lock()
	defer ts.mu.Unlock()

	t.Logf("%#v", tm)

	if len(ts.events) != nevents {
		t.Fatalf("events did not make it to the sink: %d != %d", len(ts.events), 1000)
	}

	if tm.events != nevents && tm.incoming != nevents && tm.outgoing != nevents {
		t.Fatalf("events, incoming, outgoing should all == %d, %#v", nevents, tm)
	}

	if !ts.closed {
		t.Fatalf("sink should have been closed")
	}
	// t.Fatalf("%#v", tm)
}

type endSink struct {
	Sink
}

type testMetrics struct {
	events   int
	incoming int
	outgoing int
}

func (m *testMetrics) ingress(event Event) {
	m.events++
	m.incoming++
}

func (m *testMetrics) egress(event Event) {
	m.outgoing++
}
