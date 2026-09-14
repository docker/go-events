package events

// ErrorMatcher matches errors.
type ErrorMatcher interface {
	Match(err error) bool
}

// ErrorMatcherFunc implements ErrorMatcher with just a function.
type ErrorMatcherFunc func(event Event) bool

// Match calls the wrapped function.
func (fn ErrorMatcherFunc) Match(err error) bool {
	return fn(err)
}

// ErrorFilter provides an event sink that does not return errors accepted by
// an error matcher. No methods on filter are goroutine safe.
type ErrorFilter struct {
	dst     Sink
	matcher ErrorMatcher
	closed  bool
}

// NewErrorFilter returns a new error filter that will send events to dst and
// return errors that return false for matcher.
func NewErrorFilter(dst Sink, matcher ErrorMatcher) Sink {
	return &ErrorFilter{dst: dst, matcher: matcher}
}

// Write an event to the filter.
func (f *ErrorFilter) Write(event Event) error {
	if f.closed {
		return ErrSinkClosed
	}

	err := f.dst.Write(event)

	if f.matcher.Match(err) {
		return nil
	}

	return err
}

// Close the filter and allow no more events to pass through.
func (f *ErrorFilter) Close() error {
	// TODO(stevvooe): Not all sinks should have Close.
	if f.closed {
		return nil
	}

	f.closed = true
	return f.dst.Close()
}

// FatalError provides an event sink that closes itself if destiny returns an
// error accepted by an error matcher. No methods on filter are goroutine safe.
type FatalError struct {
	dst     Sink
	matcher ErrorMatcher
	closed  bool
}

// NewFatalError returns a new fatal error that will close the sink if destiny
// returns an error that returns true for matcher.
func NewFatalError(dst Sink, matcher ErrorMatcher) Sink {
	return &FatalError{dst: dst, matcher: matcher}
}

// Write an event to the fatal error.
func (f *FatalError) Write(event Event) error {
	if f.closed {
		return ErrSinkClosed
	}

	err := f.dst.Write(event)

	if f.matcher.Match(err) {
		f.Close()
	}

	return err
}

// Close the fatal error and allow no more events to pass through.
func (f *FatalError) Close() error {
	// TODO(stevvooe): Not all sinks should have Close.
	if f.closed {
		return nil
	}

	f.closed = true
	return f.dst.Close()
}
