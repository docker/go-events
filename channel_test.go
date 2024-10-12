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

		sink.Close()

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

	sink.Close()
	_, ok := <-sink.Done() // test will timeout if this hangs
	if ok {
		t.Fatalf("done should be a closed channel")
	}

	if received != nevents {
		t.Fatalf("events did not make it through sink: %v != %v", received, nevents)
	}
}
