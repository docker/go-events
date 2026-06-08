package events

import (
	"sync"
	"testing"
)

func TestRoundRobin(t *testing.T) {
	const nEvents = 1000
	const nSinks  = 10
	var sinks []Sink
	rr := NewRoundRobin()
	for i := 0; i < nSinks; i++ {
		sinks = append(sinks, newTestSink(t, nEvents/nSinks))
		rr.Add(sinks[i])
		rr.Add(sinks[i]) // noop
	}

	var wg sync.WaitGroup
	for i := 0; i < nEvents; i++ {
		wg.Add(1)
		go func(event Event) {
			if err := rr.Write(event); err != nil {
				t.Fatalf("error writing event %v: %v", event, err)
			}
			wg.Done()
		}("event")
	}

	wg.Wait() // Wait until writes complete

	for _, sink := range sinks {
		rr.Remove(sink)
	}

	// sending one more should trigger no sink failure.
	if err := rr.Write("one-more"); err != ErrNoSinks {
		t.Fatalf("unexpected error: %v != %v", err, ErrNoSinks)
	}

	// add them back to test closing.
	for i := range sinks {
		rr.Add(sinks[i])
	}

	checkClose(t, rr)

	// Iterate through the sinks and check that they all have the expected length.
	for _, sink := range sinks {
		ts := sink.(*testSink)
		ts.mu.Lock()
		defer ts.mu.Unlock()

		if len(ts.events) != nEvents/nSinks {
			t.Fatalf("not all events ended up in testsink: len(testSink) == %d, not %d", len(ts.events), nEvents/nSinks)
		}

		if !ts.closed {
			t.Fatalf("sink should have been closed")
		}
	}
}

func BenchmarkRoundRobin10(b *testing.B) {
	benchmarkRoundRobin(b, 10)
}

func BenchmarkRoundRobin100(b *testing.B) {
	benchmarkRoundRobin(b, 100)
}

func BenchmarkRoundRobin1000(b *testing.B) {
	benchmarkRoundRobin(b, 1000)
}

func BenchmarkRoundRobin10000(b *testing.B) {
	benchmarkRoundRobin(b, 10000)
}

func benchmarkRoundRobin(b *testing.B, nsinks int) {
	b.StopTimer()
	var sinks []Sink
	for i := 0; i < nsinks; i++ {
		sinks = append(sinks, newTestSink(b, b.N/nsinks + 1))
	}
	b.StartTimer()

	benchmarkSink(b, NewRoundRobin(sinks...))
}
