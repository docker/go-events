package events

import "sync"

// Matcher matches events.
type Matcher interface {
	Match(event Event) bool
}

// MatcherFunc implements matcher with just a function.
type MatcherFunc func(event Event) bool

// Match calls the wrapped function.
func (fn MatcherFunc) Match(event Event) bool {
	return fn(event)
}

// Filter provides an event sink that sends only events that are accepted by a
// Matcher. No methods on filter are goroutine safe.
type Filter struct {
	dst     Sink
	matcher Matcher
	closed  chan struct{}
	once    sync.Once
}

// NewFilter returns a new filter that will send to events to dst that return
// true for Matcher.
func NewFilter(dst Sink, matcher Matcher) Sink {
	return &Filter{dst: dst, matcher: matcher, closed: make(chan struct{})}
}

// Write an event to the filter.
func (f *Filter) Write(event Event) error {
	select {
	case <-f.closed:
		return ErrSinkClosed
	default:
	}

	if f.matcher.Match(event) {
		return f.dst.Write(event)
	}

	return nil
}

// Close the filter and allow no more events to pass through.
func (f *Filter) Close() error {
	// TODO(stevvooe): Not all sinks should have Close.
	var ret error
	f.once.Do(func() {
		close(f.closed)
		ret = f.dst.Close()
	})

	return ret
}

// Done returns a channel that will always proceed once the sink is closed.
func (f *Filter) Done() <-chan struct{} {
	return f.closed
}
