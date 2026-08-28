package events

import "testing"

func TestFilter(t *testing.T) {
	const nevents = 100
	ts := newTestSink(t, nevents/2)
	f := NewFilter(ts, MatcherFunc(func(event Event) bool {
		i, ok := event.(int)
		return ok && i%2 == 0
	}))

	for i := range nevents {
		if err := f.Write(i); err != nil {
			t.Fatalf("unexpected error writing event: %v", err)
		}
	}

	checkClose(t, f)
}
