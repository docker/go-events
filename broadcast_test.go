package events

import (
	"fmt"
	"sync"
	"testing"
)

func TestBroadcaster(t *testing.T) {
	const nEvents = 1000
	var sinks []Sink
	b := NewBroadcaster()
	for i := 0; i < 10; i++ {
		sinks = append(sinks, newTestSink(t, nEvents))
		err := b.Add(sinks[i])
		if err != nil {
			t.Errorf("expected nil error, got %v", err)
		}

		err = b.Add(sinks[i]) // noop
		if err != nil {
			t.Errorf("expected nil error, got %v", err)
		}
	}

	var (
		wg       sync.WaitGroup
		asyncErr error
		once     sync.Once
	)
	for i := 1; i <= nEvents; i++ {
		wg.Add(1)
		go func(event Event) {
			defer wg.Done()

			if err := b.Write(event); err != nil {
				once.Do(func() {
					asyncErr = fmt.Errorf("error writing event(%v): %v", event, err)
				})
			}
		}(fmt.Sprintf("event-%d", i))
	}

	wg.Wait() // Wait until writes complete

	if asyncErr != nil {
		t.Fatalf("expected nil error, got %v", asyncErr)
	}

	for i := range sinks {
		err := b.Remove(sinks[i])
		if err != nil {
			t.Errorf("expected nil error, got %v", err)
		}
	}

	// sending one more should trigger test failure if they weren't removed.
	if err := b.Write("onemore"); err != nil {
		t.Fatalf("unexpected error sending one more: %v", err)
	}

	// add them back to test closing.
	for i := range sinks {
		err := b.Add(sinks[i])
		if err != nil {
			t.Errorf("expected nil error, got %v", err)
		}
	}

	checkClose(t, b)

	// Iterate through the sinks and check that they all have the expected length.
	for _, sink := range sinks {
		ts := sink.(*testSink)
		ts.mu.Lock()
		defer ts.mu.Unlock()

		if len(ts.events) != nEvents {
			t.Fatalf("not all events ended up in testsink: len(testSink) == %d, not %d", len(ts.events), nEvents)
		}

		if !ts.closed {
			t.Fatalf("sink should have been closed")
		}
	}
}

func BenchmarkBroadcast10(b *testing.B) {
	benchmarkBroadcast(b, 10)
}

func BenchmarkBroadcast100(b *testing.B) {
	benchmarkBroadcast(b, 100)
}

func BenchmarkBroadcast1000(b *testing.B) {
	benchmarkBroadcast(b, 1000)
}

func BenchmarkBroadcast10000(b *testing.B) {
	benchmarkBroadcast(b, 10000)
}

func benchmarkBroadcast(b *testing.B, nsinks int) {
	// counter := metrics.NewCounter()
	// metrics.DefaultRegistry.Register(fmt.Sprintf("nsinks: %v", nsinks), counter)
	// go metrics.Log(metrics.DefaultRegistry, 500*time.Millisecond, log.New(os.Stderr, "metrics: ", log.LstdFlags))

	b.StopTimer()
	var sinks []Sink
	for i := 0; i < nsinks; i++ {
		// counter.Inc(1)
		sinks = append(sinks, newTestSink(b, b.N))
		// sinks = append(sinks, NewQueue(&testSink{t: b, expected: b.N}))
	}
	b.StartTimer()

	// meter := metered{}
	// NewQueue(meter.Egress(dst))

	benchmarkSink(b, NewBroadcaster(sinks...))
}
