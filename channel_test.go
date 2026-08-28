package events

import (
	"fmt"
	"sync"
	"testing"
)

func TestChannel(t *testing.T) {
	const nevents = 100

	errCh := make(chan error)
	sink := NewChannel(0)

	go func() {
		var (
			wg       sync.WaitGroup
			asyncErr error
			once     sync.Once
		)
		for i := 1; i <= nevents; i++ {
			wg.Add(1)
			go func(event Event) {
				defer wg.Done()

				if err := sink.Write(event); err != nil {
					once.Do(func() {
						asyncErr = fmt.Errorf("error writing event(%v): %v", event, err)
					})
				}
			}(fmt.Sprintf("event-%d", i))
		}

		wg.Wait()

		if asyncErr != nil {
			errCh <- asyncErr
			return
		}

		_ = sink.Close()

		// now send another bunch of events and ensure we stay closed
		for i := 1; i <= nevents; i++ {
			wg.Add(1)
			go func(event Event) {
				defer wg.Done()

				if err := sink.Write(event); err != ErrSinkClosed {
					once.Do(func() {
						asyncErr = fmt.Errorf("expected %v, got %v", ErrSinkClosed, err)
					})
				}
			}(fmt.Sprintf("event-%d", i))
		}

		wg.Wait()

		if asyncErr != nil {
			errCh <- asyncErr
		}
	}()

	var received int
loop:
	for {
		select {
		case <-sink.C:
			received++
		case err := <-errCh:
			t.Fatal(err)
		case <-sink.Done():
			break loop
		}
	}

	close(errCh)

	_ = sink.Close()
	_, ok := <-sink.Done() // test will timeout if this hangs
	if ok {
		t.Fatalf("done should be a closed channel")
	}

	if received != nevents {
		t.Fatalf("events did not make it through sink: %v != %v", received, nevents)
	}
}

// TestChannelWriteAfterClose is a regression test for
// https://github.com/docker/go-events/issues/29. Once Close has completed,
// subsequent writes must always return ErrSinkClosed, even if the channel has
// capacity to accept another event.
func TestChannelWriteAfterClose(t *testing.T) {
	const nEvents = 100

	sink := NewChannel(nEvents)
	if err := sink.Close(); err != nil {
		t.Fatal(err)
	}

	for i := range nEvents {
		if err := sink.Write(i); err != ErrSinkClosed {
			t.Fatalf("Write(%d) error = %v, want %v", i, err, ErrSinkClosed)
		}
	}
}

// TestChannelCloseUnblocksWrite verifies that closing a Channel releases a
// Write that is blocked waiting for a receiver, and that the blocked Write
// returns ErrSinkClosed.
func TestChannelCloseUnblocksWrite(t *testing.T) {
	sink := NewChannel(0)

	errCh := make(chan error)
	go func() {
		errCh <- sink.Write("event")
	}()

	if err := sink.Close(); err != nil {
		t.Fatal(err)
	}

	if err := <-errCh; err != ErrSinkClosed {
		t.Fatalf("Write() error = %v, want %v", err, ErrSinkClosed)
	}
}
