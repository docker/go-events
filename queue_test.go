package events

import (
	"context"
	"errors"
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

type errorOnWrite struct {
	numWriteErrs int
	wait         *sync.WaitGroup
}

func (eow *errorOnWrite) Write(event Event) error {
	eow.numWriteErrs++
	eow.wait.Done()
	return errors.New("write failed")
}

func (eow *errorOnWrite) Close() error {
	return nil
}

func TestBufferedChanneledQueue(t *testing.T) {
	const nevents = 100

	var waitWrite sync.WaitGroup
	waitWrite.Add(nevents)
	eow := &errorOnWrite{wait: &waitWrite}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	eq, errCh := NewAsyncQueue(ctx, eow, WithBufferedChannel(nevents))

	var (
		waitQueue sync.WaitGroup
		asyncErr  error
		once      sync.Once
	)
	for i := 1; i <= nevents; i++ {
		waitQueue.Add(1)
		go func(event Event) {
			defer waitQueue.Done()

			if err := eq.Write(event); err != nil {
				once.Do(func() {
					asyncErr = fmt.Errorf("error writing event(%v): %v", event, err)
				})
			}
		}(fmt.Sprintf("event-%d", i))
	}

	// Wait for all writes to be queued.
	waitQueue.Wait()

	// Verify there was no error queuing up events.
	if asyncErr != nil {
		t.Fatalf("expected nil error, got %v", asyncErr)
	}

	waitWrite.Wait()

	if len(errCh) != nevents {
		t.Fatalf("expected %d errors, got %d", nevents, len(errCh))
	}
}
