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
	time.Sleep(10 * time.Millisecond) // let's queue settle to wait conidition.

	var (
		wg       sync.WaitGroup
		asyncErr error
		once     sync.Once
	)
	for i := 1; i <= nevents; i++ {
		wg.Add(1)
		go func(event Event) {
			defer wg.Done()

			if err := eq.Write(event); err != nil {
				once.Do(func() {
					asyncErr = fmt.Errorf("error writing event(%v): %v", event, err)
				})
			}
		}(fmt.Sprintf("event-%d", i))
	}

	wg.Wait()

	if asyncErr != nil {
		t.Fatalf("expected nil error, got %v", asyncErr)
	}

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
