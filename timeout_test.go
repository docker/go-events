package events

import (
	"testing"
	"time"
)

func TestTimeout(t *testing.T) {
	const nevents = 100
	sink := newTestSink(t, nevents*2)

	ts := NewTimeout(
		sink,
		time.Millisecond,
	)
	for i := 0; i < nevents; i++ {
		if err := ts.Write(i); err != nil {
			t.Fatalf("error writting event: %v", err)
		}
	}

	ts = NewTimeout(
		// delayed sink simulates destination slower than timeout
		&delayedSink{
			sink,
			time.Millisecond * 2,
		},
		time.Millisecond,
	)
	for i := 0; i < nevents; i++ {
		if err := ts.Write(i); err != ErrSinkTimeout {
			t.Fatalf("unexpected error: %v != %v", err, ErrSinkTimeout)
		}
	}

	// Wait for all the events
	time.Sleep(time.Millisecond * 5)

	checkClose(t, ts)
}
